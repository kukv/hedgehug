package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// rewriteProgressFile is kept in .hedgehug/local from just before the
// history is made again until origin has the new one (D29). While it is
// there a sync is refused and a new run resumes from it.
const rewriteProgressFile = "rewrite-in-progress.json"

// rewriteProgress is the content of rewriteProgressFile.
type rewriteProgress struct {
	Before string       `json:"before"` // the head of origin before the rewrite
	ULIDs  []model.ULID `json:"ulids"`
	At     time.Time    `json:"at"`
	// Live is the tasks the store still had as tasks: step 5 purges them,
	// and a run that stopped before that does it first.
	Live []model.ULID `json:"live,omitempty"`
}

// RewriteInput is what the history rewrite takes.
type RewriteInput struct {
	Arg string
	// Typed is the store name the user typed to confirm (decision 17); ""
	// asks for it.
	Typed string
	// Confirmed is the ULIDs of the tasks the user was shown (PrepareRewrite
	// lists them). When it is not nil, RewriteHistory fails with
	// rewrite.changed if the tasks are no longer those.
	Confirmed []model.ULID
}

// RewritePlan is what a history rewrite would remove.
type RewritePlan struct {
	Store string
	Tasks []RewriteTarget // the tasks to remove from the history
	// Resumed says that an earlier run was left unfinished: the history is
	// made again already, and only the push is left (D29). The tasks are
	// then those that run recorded, without their ids.
	Resumed bool
}

// ULIDs are the ULIDs of the tasks of the plan.
func (p RewritePlan) ULIDs() []model.ULID {
	out := make([]model.ULID, len(p.Tasks))
	for i, t := range p.Tasks {
		out[i] = t.ULID
	}
	return out
}

// RewriteTarget is a task to remove from the history of a store.
type RewriteTarget struct {
	ULID  model.ULID
	ID    string
	Title string
	// Live says that the store still has the task (not a forward): it is
	// purged first (spec step 5).
	Live bool
}

// RewriteResult is what RewriteHistory did.
type RewriteResult struct {
	Plan      RewritePlan
	Rewritten int // commits made again
	Pushed    []string
	Warnings  []string
}

// rewriteRun is a history rewrite located: the session of the store and what
// to remove.
type rewriteRun struct {
	ts     *Session
	plan   RewritePlan
	resume *rewriteProgress
}

// PrepareRewrite does steps 1 to 3 of the history rewrite: it finds the
// store and the tasks, syncs the store, and refuses when it has unpushed or
// uncommitted changes or is read-only. It changes nothing but what the sync
// does.
func (ss *Session) PrepareRewrite(in RewriteInput) (RewritePlan, error) {
	defer ss.dropMount()
	run, err := ss.locateRewrite(in.Arg)
	if err != nil {
		return RewritePlan{}, err
	}
	if run.resume != nil {
		return run.plan, nil
	}
	if err := run.prepare(in.Arg); err != nil {
		return RewritePlan{}, err
	}
	return run.plan, nil
}

// RewriteHistory does steps 1 to 9. An empty Typed is KindUsage
// confirm.required (with the plan); one that is not the store's name is
// Canceled. A run that was left unfinished (rewrite-in-progress.json) goes on
// from the push (D29).
//
// Steps 5 to 8 run holding sync.lock and the store lock: the history is
// made again and main moves to it between the two commits, and a sync or
// another writer in between would be lost or undone.
func (ss *Session) RewriteHistory(in RewriteInput) (RewriteResult, error) {
	defer ss.dropMount()
	run, err := ss.locateRewrite(in.Arg)
	if err != nil {
		return RewriteResult{}, err
	}
	name := run.plan.Store
	if in.Typed == "" {
		return RewriteResult{}, newErr(KindUsage, "confirm.required",
			fmt.Errorf("purge --rewrite-history rewrites the history of %s for good; type the store name to confirm", name),
			"what", "purge --rewrite-history", "store", name, "plan", run.plan)
	}
	if in.Typed != name {
		return RewriteResult{}, newErr(KindInvalid, "rewrite.canceled", errors.New("Canceled"), "store", name)
	}
	if run.resume == nil {
		if err := run.prepare(in.Arg); err != nil {
			return RewriteResult{}, err
		}
	}
	if in.Confirmed != nil && !sameULIDs(run.plan.ULIDs(), in.Confirmed) {
		return RewriteResult{}, newErr(KindConflict, "rewrite.changed", errors.New("the tasks changed after the confirmation; run purge --rewrite-history again"))
	}
	return run.execute()
}

func sameULIDs(a, b []model.ULID) bool {
	key := func(us []model.ULID) []string {
		out := make([]string, len(us))
		for i, u := range us {
			out[i] = u.String()
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(key(a), key(b))
}

// locateRewrite does steps 1 and 2 (no sync): the store and the tasks, or
// the unfinished run of a store.
func (ss *Session) locateRewrite(arg string) (*rewriteRun, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	explicit := ss.StoreName
	if name, _, ok := strings.Cut(arg, ":"); ok && storeExists(c, name) {
		if explicit != "" && explicit != name {
			return nil, storeConflict(arg, name, explicit)
		}
		explicit = name
	}
	name, targets, err := ss.rewriteTargets(arg, explicit)
	if err == nil {
		prog, perr := rewriteProgressOf(c, name)
		if perr != nil {
			return nil, perr
		}
		if prog != nil {
			return ss.resumedRewrite(name, prog), nil
		}
		return &rewriteRun{ts: ss.ForStore(name), plan: RewritePlan{Store: name, Tasks: targets}}, nil
	}
	// The history was made again already, so the task is not found any
	// more: the run that was left unfinished is the one to go on with.
	var left []string
	var found *rewriteProgress
	for _, e := range c.Stores {
		if explicit != "" && e.Name != explicit {
			continue
		}
		prog, perr := rewriteProgressOf(c, e.Name)
		if perr != nil {
			return nil, perr
		}
		if prog != nil {
			left = append(left, e.Name)
			found = prog
		}
	}
	if len(left) == 1 {
		return ss.resumedRewrite(left[0], found), nil
	}
	return nil, err
}

func (ss *Session) resumedRewrite(name string, prog *rewriteProgress) *rewriteRun {
	plan := RewritePlan{Store: name, Resumed: true}
	for _, u := range prog.ULIDs {
		plan.Tasks = append(plan.Tasks, RewriteTarget{ULID: u})
	}
	return &rewriteRun{ts: ss.ForStore(name), plan: plan, resume: prog}
}

// rewriteProgressOf reads rewrite-in-progress.json of the store name; nil
// when the store has none. A file that cannot be read or decoded is an
// error naming it, here and in the refusal of a sync alike.
func rewriteProgressOf(c *config.Config, name string) (*rewriteProgress, error) {
	e, err := c.Select(name)
	if err != nil {
		return nil, nil //nolint:nilerr // a store that is not listed has no file
	}
	return readRewriteProgress(filepath.Join(e.Path, store.LocalDir, rewriteProgressFile))
}

func readRewriteProgress(file string) (*rewriteProgress, error) {
	b, err := os.ReadFile(file) //nolint:gosec // a file of the store
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, newErr(KindInvalid, "rewrite.progress_unreadable", fmt.Errorf("%s cannot be read (%w); fix or delete it", file, err), "file", file)
	}
	var p rewriteProgress
	if err := json.Unmarshal(b, &p); err != nil || p.Before == "" || len(p.ULIDs) == 0 || p.At.IsZero() {
		if err == nil {
			err = errors.New("before, ulids and at are needed")
		}
		return nil, newErr(KindInvalid, "rewrite.progress_unreadable", fmt.Errorf("%s is broken (%w); a history rewrite may be unfinished: fix or delete it", file, err), "file", file)
	}
	return &p, nil
}

// rewriteTargets finds the store whose history is rewritten and the tasks to
// remove from it (spec steps 1 and 2). explicit is the store the user named,
// "" for none.
func (ss *Session) rewriteTargets(arg, explicit string) (string, []RewriteTarget, error) {
	set, err := ss.Mount()
	if err != nil {
		return "", nil, err
	}
	if explicit != "" {
		if _, ok := set.Get(explicit); !ok {
			return "", nil, newErr(KindNotFound, "store.not_found", fmt.Errorf("the store %s is not mounted", explicit), "name", explicit)
		}
	}
	// The task is looked for in every store: a moved task is found in the
	// store it moved to, and its forward is in the store to rewrite.
	var root model.ULID
	var dest *mount.Mounted
	var mv *mount.MovedError
	hit, err := set.Resolve(arg)
	switch {
	case err == nil:
		root, dest = hit.Entry.ULID, hit.Store
	case errors.As(err, &mv):
		root, dest = mv.Forward.ULID, mv.To
	default:
		return "", nil, ss.mountResolveError(set, arg, err)
	}
	ulids := []model.ULID{root}
	titles := map[model.ULID]string{}
	if dest != nil {
		if err := dest.With(func(ix *index.Index) {
			e, ok := ix.Task(root)
			if !ok {
				return
			}
			titles[root] = e.Title
			for _, d := range Descendants(ix, root, every) {
				ulids = append(ulids, d.ULID)
				titles[d.ULID] = d.Title
			}
		}); err != nil {
			return "", nil, err
		}
	}
	// D34: the files a store ever had, walked once per store for all the
	// tasks looked for.
	seen := map[string]map[model.ULID]bool{}
	hist := func(m *mount.Mounted) (map[model.ULID]bool, error) {
		if h, ok := seen[m.Name]; ok {
			return h, nil
		}
		h, err := tasksInHistory(m.Store.Repo(), ulids)
		seen[m.Name] = h
		return h, err
	}
	pick := explicit
	if pick == "" {
		var holders []string
		for _, m := range set.Stores {
			if m == dest {
				continue
			}
			var has bool
			var herr error
			if err := m.With(func(ix *index.Index) {
				if _, has = ix.Forward(root); !has {
					// the file may be in the history only, as after purge --moved
					var h map[model.ULID]bool
					h, herr = hist(m)
					has = h[root]
				}
			}); err != nil {
				return "", nil, err
			}
			if herr != nil {
				return "", nil, herr
			}
			if has {
				holders = append(holders, m.Name)
			}
		}
		switch {
		case len(holders) == 1:
			pick = holders[0]
		case len(holders) > 1:
			return "", nil, choiceRequired("--store", holders)
		case dest != nil:
			pick = dest.Name
		default:
			return "", nil, newErr(KindNotFound, "task.not_found", fmt.Errorf("%q: %w", arg, store.ErrNotFound), "input", arg)
		}
	}
	m, _ := set.Get(pick)
	var targets []RewriteTarget
	var herr error
	if err := m.With(func(ix *index.Index) {
		for _, u := range ulids {
			if e, ok := ix.Task(u); ok {
				targets = append(targets, RewriteTarget{ULID: u, ID: ix.DisplayID(e), Title: e.Title, Live: true})
			} else if f, ok := ix.Forward(u); ok {
				targets = append(targets, RewriteTarget{ULID: u, ID: f.ID, Title: titles[u]})
			} else if h, err := hist(m); err != nil {
				herr = err
			} else if h[u] {
				targets = append(targets, RewriteTarget{ULID: u, Title: titles[u]})
			}
		}
	}); err != nil {
		return "", nil, err
	}
	if herr != nil {
		return "", nil, herr
	}
	if len(targets) == 0 {
		return "", nil, newErr(KindNotFound, "rewrite.nothing", fmt.Errorf("%s has no file of %s, so there is nothing to remove from its history", pick, arg), "store", pick, "input", arg)
	}
	return pick, targets, nil
}

// tasksInHistory reports which of want have their task file in some commit of
// the repository (D34): a task whose forward was purged is still there. It
// walks the history once, comparing each commit with its first parent (a
// commit that a merge brought in is walked too), and stops when all are found.
func tasksInHistory(r *gitrepo.Repo, want []model.ULID) (map[model.ULID]bool, error) {
	found := map[model.ULID]bool{}
	if len(want) == 0 {
		return found, nil
	}
	wanted := make(map[model.ULID]bool, len(want))
	for _, u := range want {
		wanted[u] = true
	}
	note := func(p string) {
		parts := strings.Split(p, "/")
		if len(parts) != 4 || parts[0] != "tasks" {
			return
		}
		if u, err := model.ParseULID(strings.TrimSuffix(parts[3], ".md")); err == nil && wanted[u] {
			found[u] = true
		}
	}
	log, err := r.Log(nil, 0)
	if err != nil {
		return nil, err
	}
	for _, e := range log {
		if len(found) == len(wanted) {
			break
		}
		to, err := r.TreeOf(e.Hash)
		if err != nil {
			return nil, err
		}
		if len(e.Parents) == 0 {
			for p, err := range r.Files(to, "tasks/") {
				if err != nil {
					return nil, err
				}
				note(p)
			}
			continue
		}
		from, err := r.TreeOf(e.Parents[0])
		if err != nil {
			return nil, err
		}
		cs, err := r.DiffTrees(from, to, "tasks/")
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			if !c.Deleted {
				note(c.Path)
			}
		}
	}
	return found, nil
}

// prepare does step 3: it syncs the store, refuses a store that is
// read-only, has no remote, or has changes that are not pushed or not
// committed, and finds the tasks again (the sync may have brought some).
func (r *rewriteRun) prepare(arg string) error {
	ts, name := r.ts, r.plan.Store
	if _, _, err := ts.OpenForWrite(); err != nil { // read-only is refused here
		return err
	}
	c, e, err := ts.Entry()
	if err != nil {
		return err
	}
	if len(e.Remotes) == 0 {
		return newErr(KindInvalid, "rewrite.no_remote", fmt.Errorf("%s has no remote, so there is no history to rewrite for others", name), "store", name)
	}
	if _, err := ts.SyncStore(c, e, true); err != nil {
		return fmt.Errorf("sync %s: %w", name, err)
	}
	ts.dropMount()
	s, err := ts.OpenStore()
	if err != nil {
		return err
	}
	if err := rewriteClean(s, name); err != nil {
		return err
	}
	_, targets, err := ts.rewriteTargets(arg, name)
	if err != nil {
		return err
	}
	r.plan.Tasks = targets
	return nil
}

// rewriteClean refuses a store with a change that is not committed or not
// pushed (spec step 3).
func rewriteClean(s *store.Store, name string) error {
	st, err := s.Repo().Status()
	if err != nil {
		return err
	}
	if len(st) > 0 {
		return newErr(KindConflict, "rewrite.dirty", fmt.Errorf("%s has uncommitted changes (%s); commit or undo them first", name, st[0].Path), "store", name)
	}
	n, err := Unpushed(s)
	if err != nil {
		return err
	}
	if n > 0 {
		return newErr(KindConflict, "rewrite.unpushed", fmt.Errorf("%s has %d unpushed commits; run hedgehug sync first", name, n), "store", name, "unpushed", n)
	}
	return nil
}

// execute does steps 5 to 9, holding sync.lock and the store lock.
func (r *rewriteRun) execute() (RewriteResult, error) {
	ts := r.ts
	s, ix, err := ts.OpenForWrite()
	if err != nil {
		return RewriteResult{}, err
	}
	c, e, err := ts.Entry()
	if err != nil {
		return RewriteResult{}, err
	}
	opt, err := ts.SyncOptions(c, e)
	if err != nil {
		return RewriteResult{}, err
	}
	res := RewriteResult{Plan: r.plan}
	err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
		l, err := store.LockInOrder(s)
		if err != nil {
			return err
		}
		defer l.Release()
		return r.run(s, l, opt.Conn, &res)
	})
	if err != nil {
		return res, err
	}
	// The head moved to a commit that is not a descendant of the old one;
	// the old trees are still in the object storage, so the index catches up
	// by the difference.
	if err := ix.Refresh(); err != nil {
		res.Warnings = append(res.Warnings, "read the index again: "+err.Error())
	}
	return res, nil
}

// run is steps 5 to 8 under the locks.
func (r *rewriteRun) run(s *store.Store, l *store.Locked, conn gitrepo.Conn, res *RewriteResult) error {
	ts, name := r.ts, r.plan.Store
	repo := s.Repo()
	repo.SetConn(conn)
	prog := r.resume
	if prog == nil {
		// Read again under the locks: a write or a sync since step 3 would
		// be lost, and origin may have moved on.
		if err := rewriteClean(s, name); err != nil {
			return err
		}
		tracked, err := repo.RemoteHead("origin")
		if err != nil {
			return err
		}
		fetched, err := repo.Fetch("origin")
		if err != nil {
			return err
		}
		head, err := repo.Head()
		if err != nil {
			return err
		}
		if fetched != tracked || head != fetched {
			return newErr(KindConflict, "rewrite.origin_moved", fmt.Errorf("origin of %s changed since the sync; run purge --rewrite-history again", name), "store", name)
		}
		// The file goes first, so that a run that stops after any step
		// below can be run again (D29, from before step 5).
		prog = &rewriteProgress{Before: tracked.String(), ULIDs: r.plan.ULIDs(), At: ts.Now().UTC().Truncate(time.Second)}
		for _, t := range r.plan.Tasks {
			if t.Live {
				prog.Live = append(prog.Live, t.ULID)
			}
		}
		b, err := json.Marshal(prog)
		if err != nil {
			return err
		}
		if err := s.WriteLocal(rewriteProgressFile, b); err != nil {
			return err
		}
	} else if _, err := r.originStill(repo, prog); err != nil {
		return err
	}

	recorded, err := rewriteRecorded(repo, prog.At)
	if err != nil {
		return err
	}
	if !recorded {
		// Step 5: the tasks the store still has go first, with their numbers.
		// A run that stopped after it finds them gone.
		var live []model.ULID
		for _, u := range prog.Live {
			_, err := s.Task(u)
			if err == nil {
				live = append(live, u)
			} else if !errors.Is(err, store.ErrNotFound) && !errors.Is(err, model.ErrForward) {
				return err
			}
		}
		if len(live) > 0 {
			if _, err := ts.UpdateCommit(s, store.Op{Name: "purge", Task: live[0], Locked: l}, func(tx *store.Tx) error {
				for _, u := range live {
					if err := tx.RemoveTask(u); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	if !recorded {
		// Steps 6 and 7. A run that stopped between them finds the history
		// made again already: making it again changes nothing (the commits
		// are the same), so it goes on from here.
		head, err := repo.Head()
		if err != nil {
			return err
		}
		set := map[model.ULID]bool{}
		for _, u := range prog.ULIDs {
			set[u] = true
		}
		drop := rewriteDrop(set)
		newHead, n, err := repo.RewriteHistory(head, gitrepo.Rewrite{Drop: drop, Message: rewriteMessage(set, drop)})
		if err != nil {
			return err
		}
		res.Rewritten = n
		if err := repo.ResetMain(newHead); err != nil {
			return err
		}
		if _, err := ts.UpdateCommit(s, store.Op{Name: "rewrite-history", Detail: fmt.Sprintf("%d tasks", len(prog.ULIDs)), Locked: l}, func(tx *store.Tx) error {
			old, err := repo.ReadAt(newHead, model.RewritesPath)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			rs, err := model.DecodeRewrites(old)
			if err != nil {
				return err
			}
			rs = append(rs, model.Rewrite{At: prog.At, Before: prog.Before, ULIDs: prog.ULIDs})
			data, err := model.EncodeRewrites(rs)
			if err != nil {
				return err
			}
			return tx.PutFile(model.RewritesPath, data)
		}); err != nil {
			return err
		}
	}

	// Step 8: origin first; its failure keeps rewrite-in-progress.json, so
	// that the next run pushes again and a sync is refused until then.
	expect, err := r.originStill(repo, prog)
	if err != nil {
		return err
	}
	remotes := repo.Remotes()
	if len(remotes) == 0 {
		return newErr(KindInvalid, "rewrite.no_remote", fmt.Errorf("%s has no remote", name), "store", name)
	}
	if err := repo.ForcePush(remotes[0], expect); err != nil {
		return fmt.Errorf("%w; the history of %s is rewritten here, run purge --rewrite-history again to push it", err, name)
	}
	res.Pushed = append(res.Pushed, remotes[0])
	if err := s.RemoveLocal(rewriteProgressFile); err != nil {
		return err
	}
	for _, m := range remotes[1:] {
		if err := repo.ForcePush(m, plumbing.ZeroHash); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("push %s: %v", m, err))
			continue
		}
		res.Pushed = append(res.Pushed, m)
	}
	// Nothing is left to push; the mark the commits made above set is stale.
	if n, err := Unpushed(s); err == nil && n == 0 {
		if _, err := sync.UpdateState(s, func(st *sync.State) { st.PendingSince = time.Time{} }); err != nil {
			res.Warnings = append(res.Warnings, "clear the pending mark: "+err.Error())
		}
	}
	return nil
}

// originStill fetches origin and refuses unless it is still where the
// rewrite began (prog.Before: the push is due) or already at HEAD (the push
// was made). A commit pushed by someone else meanwhile would be lost by the
// force push.
func (r *rewriteRun) originStill(repo *gitrepo.Repo, prog *rewriteProgress) (plumbing.Hash, error) {
	fetched, err := repo.Fetch("origin")
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("%w; the history of %s is rewritten here, run purge --rewrite-history again to push it", err, r.plan.Store)
	}
	head, err := repo.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if fetched.String() != prog.Before && fetched != head {
		name := r.plan.Store
		return plumbing.ZeroHash, newErr(KindConflict, "rewrite.origin_moved",
			fmt.Errorf("origin of %s changed since the history rewrite began; nothing is pushed, and %s is kept: fix it by hand or delete it to give the rewrite up", name, rewriteProgressFile), "store", name)
	}
	return fetched, nil
}

// rewriteRecorded reports whether HEAD already has the record of the
// rewrite made at at in rewrites.yaml.
func rewriteRecorded(r *gitrepo.Repo, at time.Time) (bool, error) {
	head, err := r.Head()
	if err != nil {
		return false, err
	}
	data, err := r.ReadAt(head, model.RewritesPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	rs, err := model.DecodeRewrites(data)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(rs, func(x model.Rewrite) bool { return x.At.Equal(at) }), nil
}

// rewriteDrop reports whether a path is a file of one of the tasks in set:
// the task file, or a comment or a worklog entry under it.
func rewriteDrop(set map[model.ULID]bool) func(string) bool {
	return func(p string) bool {
		parts := strings.Split(p, "/")
		var file string
		switch {
		case len(parts) == 4 && parts[0] == "tasks":
			file = strings.TrimSuffix(parts[3], ".md")
		case len(parts) == 5 && (parts[0] == "comments" || parts[0] == "worklogs"):
			file = parts[3]
		default:
			return false
		}
		u, err := model.ParseULID(file)
		return err == nil && set[u]
	}
}

// rewriteMessage gives the message of a commit made again (spec step 6): the
// title of the subject of a commit about one of the tasks becomes (purged),
// and the lines that tell of their files in Hedgehug-Overwritten and
// Hedgehug-Repaired go.
func rewriteMessage(set map[model.ULID]bool, drop func(string) bool) func(string) string {
	return func(msg string) string {
		lines := strings.Split(msg, "\n")
		about := false
		kept := lines[:1:1]
		for _, l := range lines[1:] {
			if v, ok := strings.CutPrefix(l, "Hedgehug-Task: "); ok {
				if u, err := model.ParseULID(strings.TrimSpace(v)); err == nil && set[u] {
					about = true
				}
			}
			if v, ok := strings.CutPrefix(l, "Hedgehug-Overwritten: "); ok {
				first, _, _ := strings.Cut(v, " ")
				if p, err := url.PathUnescape(first); err == nil && drop(p) {
					continue
				}
			}
			if v, ok := strings.CutPrefix(l, "Hedgehug-Repaired: "); ok {
				first, _, _ := strings.Cut(v, " ")
				if u, err := model.ParseULID(first); err == nil && set[u] {
					continue
				}
			}
			kept = append(kept, l)
		}
		if about {
			// "hedgehug: <operation> <id> <title>"
			if parts := strings.SplitN(kept[0], " ", 4); len(parts) == 4 && parts[0] == "hedgehug:" {
				kept[0] = strings.Join(parts[:3], " ") + " (purged)"
			}
		}
		return strings.Join(kept, "\n")
	}
}

// errRewriteInProgress is the refusal of a sync of a store with an unfinished
// history rewrite (D29).
func errRewriteInProgress(name string) error {
	return newErr(KindConflict, "rewrite.in_progress",
		fmt.Errorf("a history rewrite of %s is unfinished; run hedgehug purge --rewrite-history again", name), "store", name)
}

// rewriteUnfinished reports whether the store s has an unfinished history
// rewrite.
func rewriteUnfinished(s *store.Store) (bool, error) {
	p, err := readRewriteProgress(s.LocalPath(rewriteProgressFile))
	return p != nil, err
}
