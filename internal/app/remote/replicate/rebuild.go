package replicate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/osfs"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

const (
	// keptPrefix names the copy of the old store left beside the store when
	// the rebuild could not put back all its unpushed changes. Nothing
	// removes it.
	keptPrefix = ".kept-"
	// keptNew is the copy while it is written. It is removed when the next
	// rebuild starts.
	keptNew = ".kept-new"
	// indexCache is the cache of the index, which names trees of the old
	// history.
	indexCache = "index"
)

// ErrDirtyRebuild means the store has changes that were not committed, which
// the rebuild would drop. It is an ErrDirty.
var ErrDirtyRebuild error = dirtyRebuildError{}

type dirtyRebuildError struct{}

func (dirtyRebuildError) Error() string {
	return "the store has uncommitted changes, which the rebuild would drop; commit or restore them, then run hedgehug doctor --fix"
}

func (dirtyRebuildError) Is(target error) bool { return target == ErrDirty }

// Rebuild tells what RebuildStore or FinishRebuild did.
type Rebuild struct {
	Replayed int // the unpushed changes (commits) put back
	// Purged are the ids of the tasks whose unpushed changes were not put
	// back, because the new history lacks the task (the ULID for one the
	// old history has no id of), and PurgedTasks their ULIDs, in the same
	// order.
	Purged      []string
	PurgedTasks []model.ULID
	// Notices are the items put in the inbox.
	Notices []inbox.Item
	// Kept is the copy of the old store; "" when none was made. It is made
	// when Purged is not empty.
	Kept string
	// Warnings are what failed after the store was rebuilt: the removal of
	// the old objects, which the next doctor --fix or sync tries again.
	Warnings []string
}

// rebuildRecord is .hedgehug/local/rebuild.json: what it takes to finish a
// rebuild that stopped, without origin.
type rebuildRecord struct {
	Phase   store.RebuildPhase `json:"phase"`
	OldHead string             `json:"old_head"` // HEAD before the rebuild
	Target  string             `json:"target"`   // where main goes
	Origin  string             `json:"origin"`   // the new head of origin
	Mark    string             `json:"mark"`     // the commit the inbox goes on from
	Items   []inbox.Item       `json:"items"`    // for the inbox; Add leaves out what it has
	Kept    string             `json:"kept,omitempty"`
}

func writeRecord(s *store.Store, rec *rebuildRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.WriteLocal(store.RebuildFile, append(b, '\n'))
}

// readRecord reads rebuild.json; ok is false when there is none.
func readRecord(s *store.Store) (rec *rebuildRecord, ok bool, err error) {
	b, err := s.ReadLocal(store.RebuildFile)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rec = &rebuildRecord{}
	if err := json.Unmarshal(b, rec); err != nil {
		return nil, false, fmt.Errorf("%s: %w", store.RebuildFile, err)
	}
	if rec.Phase != store.RebuildApply && rec.Phase != store.RebuildPrune {
		return nil, false, fmt.Errorf("%s: unknown phase %q", store.RebuildFile, rec.Phase)
	}
	return rec, true, nil
}

// RebuildStore brings the store onto the new history of origin in its own
// directory, for hedgehug doctor --fix after a sync stopped with ErrForcePush.
// The unpushed changes are put back as one commit; a copy of the store is
// kept when some cannot be. From the moment rebuild.json is written the store
// only goes forward: what stops is finished by FinishRebuild. Before that the
// store is as it was. When origin has become a history the store follows
// again, it removes force-push.json and returns nil. The caller holds
// sync.lock; the store lock is held here for the whole rebuild.
func RebuildStore(s *store.Store, opt Options) (*Rebuild, error) {
	held, err := store.LockInOrder(s)
	if err != nil {
		return nil, err
	}
	defer held.Release()
	rec, b, err := prepare(s, opt)
	if err != nil || rec == nil {
		return nil, err
	}
	if err := finish(s, rec, b); err != nil {
		return nil, err
	}
	return b, nil
}

// prepare does all of a rebuild that leaves main as it is, up to the writing
// of rebuild.json, under the store lock. rec is nil when origin follows the
// store again.
func prepare(s *store.Store, opt Options) (rec *rebuildRecord, b *Rebuild, err error) {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	r := s.Repo()
	r.SetConn(opt.Conn)
	st, err := r.Status()
	if err != nil {
		return nil, nil, err
	}
	if len(st) > 0 {
		return nil, nil, ErrDirtyRebuild
	}
	oldHead, err := r.Head()
	if err != nil {
		return nil, nil, err
	}
	// What the store followed: the sync that found the force push put the
	// tracking ref back.
	tracked, err := r.RemoteHead("origin")
	if err != nil {
		return nil, nil, err
	}
	dir := s.Dir()
	if err := osfs.RemoveAll(dir + keptNew); err != nil {
		return nil, nil, err
	}
	if err := r.SetRemotes(opt.Remotes); err != nil {
		return nil, nil, err
	}
	head, err := r.Fetch("origin")
	if serr := r.SetTracking("origin", tracked); err == nil {
		err = serr
	}
	if err != nil {
		return nil, nil, fmt.Errorf("rebuild after a force push: %w", err)
	}
	if head.IsZero() {
		return nil, nil, errors.New("rebuild after a force push: origin is empty")
	}
	follows, err := r.IsAncestor(tracked, head)
	if err != nil {
		return nil, nil, err
	}
	if follows {
		// origin went back to a history the store follows: the next sync
		// takes it as usual.
		return nil, nil, s.RemoveLocal(forcePushFile)
	}
	b = &Rebuild{}
	s.SetNow(opt.Now)
	target, err := replay(s, tracked, oldHead, head, opt, b)
	if err != nil {
		return nil, nil, err
	}
	// The inbox goes on from where the new history has the commit it had
	// followed: what came before is what it has told of, what comes after is
	// new to it, and so is the replay commit.
	mark, found, err := r.FindSameCommit(head, tracked)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		mark = head
	}
	items, err := rewriteItems(target, opt, b)
	if err != nil {
		return nil, nil, err
	}
	if len(b.Purged) > 0 {
		if b.Kept, err = keepOldStore(dir); err != nil {
			return nil, nil, err
		}
	}
	rec = &rebuildRecord{
		Phase: store.RebuildApply, OldHead: oldHead.String(), Target: target.String(),
		Origin: head.String(), Mark: mark.String(), Items: items, Kept: b.Kept,
	}
	if err := writeRecord(s, rec); err != nil {
		return nil, nil, err
	}
	return rec, b, nil
}

// FinishRebuild finishes a rebuild that stopped, from rebuild.json; it needs
// no connection. It returns nil when there is none. The caller holds
// sync.lock; the store lock is held here.
func FinishRebuild(s *store.Store) (*Rebuild, error) {
	held, err := store.LockInOrder(s)
	if err != nil {
		return nil, err
	}
	defer held.Release()
	rec, ok, err := readRecord(s)
	if err != nil || !ok {
		return nil, err
	}
	b := &Rebuild{Kept: rec.Kept}
	if err := finish(s, rec, b); err != nil {
		return nil, err
	}
	return b, nil
}

// finish does what rebuild.json says is left. Every step may be run again.
func finish(s *store.Store, rec *rebuildRecord, b *Rebuild) error {
	if rec.Phase == store.RebuildApply {
		if err := apply(s, rec, b); err != nil {
			return fmt.Errorf("finish the rebuild: %w", err)
		}
		rec.Phase = store.RebuildPrune
		if err := writeRecord(s, rec); err != nil {
			return fmt.Errorf("finish the rebuild: %w", err)
		}
	}
	r := s.Repo()
	err := r.KeepMainRefs()
	if err == nil {
		err = r.PruneUnreachable()
	}
	if err == nil {
		err = s.RemoveLocal(store.RebuildFile)
	}
	if err != nil {
		// The store is whole: only what the history no longer has is left.
		b.Warnings = append(b.Warnings, fmt.Sprintf("remove the objects of the old history: %v", err))
	}
	return nil
}

// apply makes the store the rebuilt one: the work tree, main and the
// tracking ref, the inbox, and what was about the old history.
func apply(s *store.Store, rec *rebuildRecord, b *Rebuild) error {
	r := s.Repo()
	from, to := plumbing.NewHash(rec.OldHead), plumbing.NewHash(rec.Target)
	ft, err := r.TreeOf(from)
	if err != nil {
		return err
	}
	tt, err := r.TreeOf(to)
	if err != nil {
		return err
	}
	changes, err := r.DiffTrees(ft, tt, "")
	if err != nil {
		return err
	}
	paths := make([]string, len(changes))
	for i, c := range changes {
		paths[i] = c.Path
	}
	if err := r.MoveMain(from, to, paths); err != nil {
		return err
	}
	if err := r.SetTracking("origin", plumbing.NewHash(rec.Origin)); err != nil {
		return err
	}
	box := inbox.Open(s)
	if err := box.SetMark(plumbing.NewHash(rec.Mark)); err != nil {
		return err
	}
	if b.Notices, err = box.Add(rec.Items); err != nil {
		return err
	}
	if err := s.RemoveLocal(indexCache); err != nil {
		return err
	}
	return s.RemoveLocal(forcePushFile)
}

// skipKept leaves the locks out of the copy of the store: they are not its
// content, and another process may have them open.
func skipKept(rel string) bool {
	name := path.Base(rel)
	return strings.HasPrefix(rel, store.LocalDir+"/") && (name == "lock" || strings.HasSuffix(name, ".lock"))
}

// keepOldStore copies the store at dir to dir + keptPrefix + the time, and
// returns where. It holds the changes the rebuild cannot put back.
func keepOldStore(dir string) (string, error) {
	tmp := dir + keptNew
	if err := osfs.CopyTree(dir, tmp, skipKept); err != nil {
		return "", fmt.Errorf("keep the old store: %w", err)
	}
	kept := dir + keptPrefix + time.Now().UTC().Format("20060102T150405.000000000")
	if err := osfs.Rename(tmp, kept); err != nil {
		return "", fmt.Errorf("keep the old store: %w", err)
	}
	return kept, nil
}

// taskOfPath is the task a file of the store belongs to: its own file, or a
// comment or worklog of it.
func taskOfPath(p string) (model.ULID, bool) {
	if u, err := model.ParseTaskPath(p); err == nil {
		return u, true
	}
	if u, _, err := model.ParseCommentPath(p); err == nil {
		return u, true
	}
	if u, _, err := model.ParseWorklogPath(p); err == nil {
		return u, true
	}
	return model.ULID{}, false
}

// replay puts the unpushed changes of oldHead back on head, the new head of
// origin, as one commit off main; it returns that commit, or head when there
// is nothing to put back. The files of
// the tasks the new history lacks are left out of the merge, and a task of
// them that the unpushed commits changed goes to b.Purged.
func replay(s *store.Store, tracked, oldHead, head plumbing.Hash, opt Options, b *Rebuild) (plumbing.Hash, error) {
	r := s.Repo()
	mine, err := r.Between(oldHead, tracked)
	if err != nil {
		return head, err
	}
	if len(mine) == 0 {
		return head, nil
	}
	b.Replayed = len(mine)
	base, err := r.MergeBase(oldHead, tracked)
	if err != nil {
		return head, err
	}
	if base.IsZero() {
		return head, fmt.Errorf("replay: %w", ErrUnrelated)
	}
	dropped, err := findDropped(r, base, oldHead, head, b)
	if err != nil {
		return head, err
	}
	skip := func(p string) bool {
		u, ok := taskOfPath(p)
		return ok && dropped[u]
	}
	// The merge is computed from the new head's side, so that its files are
	// the changes to make to the clone; the rule is symmetric.
	opt.origin, opt.skip = head, skip
	m, err := mergeTrees(r, nil, base, head, oldHead, opt) // head is new on origin: no index has it
	if err != nil {
		return head, err
	}
	// What the new history already has is no change.
	for p, data := range m.files {
		cur, err := r.ReadAt(head, p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return head, err
		}
		if (data == nil && err != nil) || (data != nil && err == nil && string(cur) == string(data)) {
			delete(m.files, p)
		}
	}
	// A commit without a file records what lost, as a merge commit does.
	if len(m.files) == 0 && len(m.lost) == 0 && len(m.notices) == 0 && len(m.repaired) == 0 {
		return head, nil
	}
	op := store.Op{Name: "replay", Detail: plural.Count(len(mine), "unpushed commit"), Author: opt.Author, Account: opt.Account, AllowEmpty: true}
	return s.CommitOn(head, m.files, op, m.trailers())
}

// findDropped puts in b the tasks that the commits from base to oldHead
// changed (the task, a comment or a worklog entry) and that the new history
// lacks: the base has the task's file and the new head does not. A task
// that only the unpushed commits have is not one of them. It returns them as
// a set.
func findDropped(r *gitrepo.Repo, base, oldHead, head plumbing.Hash, b *Rebuild) (map[model.ULID]bool, error) {
	bt, err := r.TreeOf(base)
	if err != nil {
		return nil, err
	}
	ot, err := r.TreeOf(oldHead)
	if err != nil {
		return nil, err
	}
	changes, err := r.DiffTrees(bt, ot, "")
	if err != nil {
		return nil, err
	}
	dropped := map[model.ULID]bool{}
	seen := map[model.ULID]bool{}
	for _, c := range changes {
		u, ok := taskOfPath(c.Path)
		if !ok || seen[u] {
			continue
		}
		seen[u] = true
		data, err := r.ReadFile(bt, model.TaskPath(u))
		if errors.Is(err, fs.ErrNotExist) {
			continue // made by the unpushed commits
		}
		if err != nil {
			return nil, err
		}
		if _, err := r.ReadAt(head, model.TaskPath(u)); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		id := u.String()
		if t, err := model.DecodeTask(data); err == nil && t.ID != "" {
			id = t.ID
		}
		dropped[u] = true
		b.PurgedTasks = append(b.PurgedTasks, u)
		b.Purged = append(b.Purged, id)
	}
	return dropped, nil
}

// rewriteItems are the notices of spec step 7 for the rebuild to the commit
// head.
func rewriteItems(head plumbing.Hash, opt Options, b *Rebuild) ([]inbox.Item, error) {
	to := opt.Account
	if to == "" && len(opt.Me) > 0 {
		to = opt.Me[0]
	}
	now := opt.Now()
	detail, err := json.Marshal(struct {
		Store    string `json:"store"`
		Replayed int    `json:"replayed"`
	}{opt.Name, b.Replayed})
	if err != nil {
		return nil, err
	}
	items := []inbox.Item{{Kind: inbox.RewriteApplied, Commit: head.String(), To: to, Time: now, Detail: detail}}
	for i, u := range b.PurgedTasks {
		items = append(items, inbox.Item{Kind: inbox.PurgedChange, Task: u, ID: b.Purged[i], Commit: head.String(), To: to, Time: now})
	}
	return items, nil
}
