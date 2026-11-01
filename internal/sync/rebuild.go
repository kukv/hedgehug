package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

const (
	newSuffix    = ".rewrite-new"
	staleMarker  = ".rebuild-stale" // in the rebuild: the old store is the one to keep
	beforePrefix = ".before-rewrite-"
)

// ErrDirtyRebuild means the store has changes that were not committed, which
// the swap of a rebuild would drop. It is an ErrDirty.
var ErrDirtyRebuild error = dirtyRebuildError{}

type dirtyRebuildError struct{}

func (dirtyRebuildError) Error() string {
	return "the store has uncommitted changes, which the rebuild after a history rewrite would drop; commit or restore them, then run hedgehug sync"
}

func (dirtyRebuildError) Is(target error) bool { return target == ErrDirty }

// ErrRebuildStale means the store changed while the rebuild was made, so it
// was not swapped in; the next sync makes it again.
var ErrRebuildStale = errors.New("the store changed during the rebuild")

// Rebuild is a clone made again after a recorded rewrite, ready to take the
// place of the store (spec steps 2 to 4 done).
type Rebuild struct {
	Dir      string        // <path>.rewrite-new
	OldHead  plumbing.Hash // HEAD of the store when it was made
	Record   model.Rewrite
	Replayed int // the unpushed changes (commits) put back
	// Purged are the ids of the purged tasks whose unpushed changes were
	// not taken in (the ULID for one the old history has no id of), and
	// PurgedTasks their ULIDs, in the same order.
	Purged      []string
	PurgedTasks []model.ULID
	// Notices are the items put in the inbox of the rebuild (spec step 7);
	// whoever swaps it in tells of them.
	Notices []inbox.Item

	seen map[string]stamp // the local files copyLocal copied
}

// rebuild makes the clone again (spec steps 2 to 4) and puts it in
// res.Rebuild; the store is as it was. tracked is the origin the clone
// followed before the fetch. The caller holds sync.lock; the store lock is
// held here for the whole rebuild (spec step 1), and a write that lands after
// it is let go is found by Swap.
func rebuild(s *store.Store, tracked plumbing.Hash, opt Options, res *Result) (err error) {
	r := s.Repo()
	held, err := store.LockInOrder(s)
	if err != nil {
		return err
	}
	defer held.Release()
	st, err := r.Status()
	if err != nil {
		return err
	}
	if len(st) > 0 {
		// The swap would drop the changes that were not committed.
		return ErrDirtyRebuild
	}
	oldHead, err := r.Head()
	if err != nil {
		return err
	}
	dir := s.Dir()
	newDir := dir + newSuffix
	// The store is here, so a rebuild left beside it is of a swap that
	// never began.
	if err := os.RemoveAll(newDir); err != nil {
		return err
	}
	nr, err := gitrepo.CloneWith(opt.Remotes[0], newDir, opt.Conn)
	if err != nil {
		return fmt.Errorf("rebuild after a history rewrite: %w", err)
	}
	defer func() {
		_ = nr.Close()
		if err != nil {
			_ = os.RemoveAll(newDir)
		}
	}()
	if err = nr.SetRemotes(opt.Remotes); err != nil {
		return err
	}
	if err = nr.RegisterMergeDriver(); err != nil {
		return err
	}
	head, err := nr.Head()
	if err != nil {
		return err
	}
	recs, err := recordsAt(nr, head)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(recs, func(x model.Rewrite) bool { return x.At.Equal(res.Rewrite.At) }) {
		// Origin was rewritten or reset again since the fetch.
		return errors.New("origin changed while the store was rebuilt after a history rewrite; the next sync tries again")
	}
	// The clone's head came after the fetch, so the old repository may lack
	// its objects; the replay reads both histories there.
	if _, err = r.TreeOf(head); errors.Is(err, plumbing.ErrObjectNotFound) {
		if _, err = r.Fetch("origin"); err == nil {
			err = r.SetTracking("origin", tracked)
		}
	}
	if err != nil {
		return err
	}
	b := &Rebuild{Dir: newDir, OldHead: oldHead, Record: *res.Rewrite}
	purged := map[model.ULID]bool{}
	for _, x := range recs {
		for _, u := range x.ULIDs {
			purged[u] = true
		}
	}
	ns, err := store.Open(newDir)
	if err != nil {
		return err
	}
	defer func() { _ = ns.Repo().Close() }()
	ns.SetNow(opt.Now)
	newHead, err := replay(r, ns, nr, tracked, oldHead, head, purged, opt, b)
	if err != nil {
		return err
	}
	if err = copyLocal(s, ns, b); err != nil {
		return err
	}
	// The inbox goes on from where the new history has the commit it had
	// followed: what came before is what it has told of, what comes after is
	// new to it, and so is the replay commit.
	mark, found, err := r.FindSameCommit(head, tracked)
	if err != nil {
		return err
	}
	if !found {
		mark = head
	}
	if err = inbox.Open(ns).SetMark(mark); err != nil {
		return err
	}
	applied, err := ReadApplied(ns)
	if err != nil {
		return err
	}
	for _, x := range recs {
		if !appliedHas(applied, x.At) {
			applied = append(applied, x.At)
		}
	}
	if err = WriteApplied(ns, applied); err != nil {
		return err
	}
	if b.Notices, err = tellRewrite(ns, newHead, opt, b); err != nil {
		return err
	}
	res.Rebuild = b
	return nil
}

// recordsAt reads the records of rewrites.yaml at the commit.
func recordsAt(r *gitrepo.Repo, commit plumbing.Hash) ([]model.Rewrite, error) {
	data, err := r.ReadAt(commit, model.RewritesPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model.DecodeRewrites(data)
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
// origin, as one commit in ns (spec step 3); it returns the head of ns then.
// The files of the purged tasks are left out of the merge, and a task of
// them that the unpushed commits changed goes to b.Purged.
func replay(r *gitrepo.Repo, ns *store.Store, nr *gitrepo.Repo, tracked, oldHead, head plumbing.Hash, purged map[model.ULID]bool, opt Options, b *Rebuild) (plumbing.Hash, error) {
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
	skip := func(p string) bool {
		u, ok := taskOfPath(p)
		return ok && purged[u]
	}
	if err := findPurgedChanges(r, base, oldHead, skip, b); err != nil {
		return head, err
	}
	// The merge is computed from the new head's side, so that its files are
	// the changes to make to the clone; the rule is symmetric.
	opt.origin, opt.skip = head, skip
	m, err := mergeTrees(r, base, head, oldHead, opt)
	if err != nil {
		return head, err
	}
	// What the new history already has is no change.
	for p, data := range m.files {
		cur, err := nr.ReadAt(head, p)
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
	op := store.Op{Name: "replay", Detail: fmt.Sprintf("%d unpushed commits", len(mine)), Author: opt.Author, Account: opt.Account, AllowEmpty: true}
	return ns.CommitMerge(head, []plumbing.Hash{head}, m.files, op, m.trailers())
}

// findPurgedChanges puts in b the purged tasks the commits from base to
// oldHead changed.
func findPurgedChanges(r *gitrepo.Repo, base, oldHead plumbing.Hash, skip func(string) bool, b *Rebuild) error {
	bt, err := r.TreeOf(base)
	if err != nil {
		return err
	}
	ot, err := r.TreeOf(oldHead)
	if err != nil {
		return err
	}
	changes, err := r.DiffTrees(bt, ot, "")
	if err != nil {
		return err
	}
	for _, c := range changes {
		if !skip(c.Path) {
			continue
		}
		u, _ := taskOfPath(c.Path)
		if slices.Contains(b.PurgedTasks, u) {
			continue
		}
		id := u.String()
		if data, err := r.ReadFile(bt, model.TaskPath(u)); err == nil {
			if t, err := model.DecodeTask(data); err == nil && t.ID != "" {
				id = t.ID
			}
		}
		b.PurgedTasks = append(b.PurgedTasks, u)
		b.Purged = append(b.Purged, id)
	}
	return nil
}

// skipLocal says what a rebuild leaves behind of .hedgehug/local: the
// locks, the index cache and what is about the old history. A name that
// starts with a dot is the temporary file of a write in progress (the
// atomic write names it so; LocalFiles skips it too), not a file of its
// own.
func skipLocal(name string) bool {
	return name == "lock" || strings.HasSuffix(name, ".lock") || name == "index" || name == forcePushFile ||
		name == "rewrite-in-progress.json" || strings.HasPrefix(name, ".")
}

// ownedLocal are the files the rebuild writes in its own way, so that the
// copy made right before the swap leaves them alone.
var ownedLocal = []string{"rewrites-applied.json", "inbox-head"}

// stamp tells whether a file changed: its size and time.
type stamp struct {
	size int64
	mod  time.Time
}

// localFiles lists the files of .hedgehug/local of the store at dir that a
// rebuild carries over, by their names relative to it, with their stamps.
func localFiles(dir string) (map[string]stamp, error) {
	src := filepath.Join(dir, filepath.FromSlash(store.LocalDir))
	out := map[string]stamp{}
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && p == src {
			return nil // a store with nothing local
		}
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() || skipLocal(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil // gone since the walk saw it
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = stamp{info.Size(), info.ModTime()}
		return nil
	})
	return out, err
}

// copyFile copies the local file name of the store at from to the one at to.
func copyFile(from, to, name string) error {
	data, err := os.ReadFile(filepath.Join(from, filepath.FromSlash(store.LocalDir), filepath.FromSlash(name))) //nolint:gosec // a file of the store
	if err != nil {
		return err
	}
	return store.WriteLocalIn(to, name, data)
}

// copyLocal copies .hedgehug/local of from to to (spec step 4), and keeps in
// b the stamps of what it copied, for syncLocal.
func copyLocal(from, to *store.Store, b *Rebuild) error {
	files, err := localFiles(from.Dir())
	if err != nil {
		return err
	}
	for name := range files {
		if err := copyFile(from.Dir(), to.Dir(), name); err != nil {
			return err
		}
	}
	b.seen = files
	return nil
}

// syncLocal brings to the rebuild at b.Dir what changed in .hedgehug/local of
// the store at dir since copyLocal: the sync writes sync.json and the inbox
// after the rebuild was made, and a command may write there until the swap.
// The inbox's items are joined line by line, so the notices of the rebuild
// stay; the files the rebuild writes itself are left as they are.
//
// It copies from the store at from to the one at to, what differs from seen,
// and returns the stamps it saw, for the next pass.
func syncLocal(from, to string, seen map[string]stamp) (map[string]stamp, error) {
	files, err := localFiles(from)
	if err != nil {
		return nil, err
	}
	for name, st := range files {
		if seen[name] == st || slices.Contains(ownedLocal, name) {
			continue
		}
		if name != "inbox.jsonl" {
			if err := copyFile(from, to, name); err != nil {
				return nil, err
			}
			continue
		}
		old, err := os.ReadFile(filepath.Join(from, filepath.FromSlash(store.LocalDir), name)) //nolint:gosec // a file of the store
		if err != nil {
			return nil, err
		}
		cur, err := os.ReadFile(filepath.Join(to, filepath.FromSlash(store.LocalDir), name)) //nolint:gosec // a file of the store
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err := store.WriteLocalIn(to, name, joinLines(cur, old)); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// beforeRename runs between the last look at what changed in
// .hedgehug/local and the first rename of the swap; tests write there.
var beforeRename = func() {}

// joinLines is cur with the lines of more it lacks added.
func joinLines(cur, more []byte) []byte {
	have := map[string]bool{}
	for l := range strings.SplitSeq(string(cur), "\n") {
		have[l] = true
	}
	out := cur
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	for l := range strings.SplitSeq(string(more), "\n") {
		if l != "" && !have[l] {
			out = append(append(out, l...), '\n')
		}
	}
	return out
}

// tellRewrite puts the notices of spec step 7 in the inbox of ns: they are
// written before the swap, with the store, and go for nothing when it is
// not swapped in.
func tellRewrite(ns *store.Store, head plumbing.Hash, opt Options, b *Rebuild) ([]inbox.Item, error) {
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
	return inbox.Open(ns).Add(items)
}

// Swap puts b in the place of the store at dir (spec step 5): the old one
// goes to <dir>.before-rewrite-<time>, the new one to dir, and the old one
// is removed. The caller holds none of the store's locks and has closed
// its git files. It checks first that the old one is as it was when b was
// made (its HEAD is b.OldHead, nothing uncommitted); if not, it removes
// b.Dir and returns ErrRebuildStale (it looks again after the first rename,
// for a commit made in between, and puts the old store back if there is one);
// then it brings over what changed in
// .hedgehug/local since the copy (any other failure to look at the old
// store leaves b.Dir, for FinishLeftover). A rename that fails changes nothing
// or puts the old store back; only when that fails too is a leftover
// (FindLeftover) all there is. The removal of the old store may fail
// without failing Swap; it is a leftover then.
func Swap(dir string, b *Rebuild) error {
	if err := checkUnchanged(dir, b); err != nil {
		if errors.Is(err, ErrRebuildStale) {
			_ = os.RemoveAll(b.Dir)
		}
		return err
	}
	seen, err := syncLocal(dir, b.Dir, b.seen)
	if err != nil {
		return err
	}
	beforeRename()
	before := dir + beforePrefix + time.Now().UTC().Format("20060102T150405.000000000")
	if err := os.Rename(dir, before); err != nil {
		_ = os.RemoveAll(b.Dir)
		return fmt.Errorf("swap the rebuilt store in: %w", err)
	}
	// Nothing new can start in the old store now, but a command that took
	// its lock in the gap since the check may have committed there; that
	// commit would go with the old directory. If so, put it back; the next
	// sync makes the rebuild again.
	if err := checkUnchanged(before, b); err != nil {
		var merr error
		if errors.Is(err, ErrRebuildStale) {
			// If the old store cannot be put back, FinishLeftover must not
			// put the rebuild in its place: it would drop that commit.
			merr = os.WriteFile(filepath.Join(b.Dir, staleMarker), []byte("the old store has a commit this rebuild lacks\n"), 0o600)
		}
		if rerr := os.Rename(before, dir); rerr != nil {
			// merr says that FinishLeftover may not know the rebuild is stale.
			return errors.Join(err, rerr, merr)
		}
		if errors.Is(err, ErrRebuildStale) {
			_ = os.RemoveAll(b.Dir)
		}
		return err
	}
	if err := os.Rename(b.Dir, dir); err != nil {
		err = fmt.Errorf("swap the rebuilt store in: %w", err)
		if rerr := os.Rename(before, dir); rerr != nil {
			return errors.Join(err, rerr)
		}
		_ = os.RemoveAll(b.Dir)
		return err
	}
	// What was written to the old store in the gap since the look above is
	// brought over now, from where it was moved to.
	if _, err := syncLocal(before, dir, seen); err != nil {
		// The rebuilt store is in place; the old one stays for a look.
		return fmt.Errorf("carry over the local files of %s: %w", before, err)
	}
	_ = os.RemoveAll(before)
	return nil
}

// checkUnchanged returns ErrRebuildStale unless the store at dir is as it
// was when b was made.
func checkUnchanged(dir string, b *Rebuild) error {
	r, err := gitrepo.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	head, err := r.Head()
	if err != nil {
		return err
	}
	st, err := r.Status()
	if err != nil {
		return err
	}
	if head != b.OldHead || len(st) > 0 {
		return ErrRebuildStale
	}
	return nil
}

// Leftover is what a rebuild left beside the store at dir.
type Leftover struct {
	New    string   // <dir>.rewrite-new, "" when none
	Before []string // <dir>.before-rewrite-*, oldest first
}

// None reports whether nothing was left.
func (l Leftover) None() bool { return l.New == "" && len(l.Before) == 0 }

// FindLeftover looks beside dir for what a rebuild left.
func FindLeftover(dir string) (Leftover, error) {
	entries, err := ReadParent(dir)
	if err != nil {
		return Leftover{}, err
	}
	return LeftoverIn(dir, entries), nil
}

// ReadParent reads the directory dir is in, for LeftoverIn; a caller that
// looks at several stores reads each directory once. A missing directory
// has no entries.
func ReadParent(dir string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(filepath.Dir(filepath.Clean(dir)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return entries, err
}

// LeftoverIn is FindLeftover over the entries of the directory dir is in
// (ReadParent).
func LeftoverIn(dir string, entries []fs.DirEntry) Leftover {
	var l Leftover
	parent, base := filepath.Split(filepath.Clean(dir))
	if parent == "" {
		parent = "."
	}
	for _, e := range entries { // sorted by name, so the oldest before-rewrite first
		switch name := e.Name(); {
		case name == base+newSuffix:
			l.New = filepath.Join(parent, name)
		case strings.HasPrefix(name, base+beforePrefix):
			l.Before = append(l.Before, filepath.Join(parent, name))
		}
	}
	return l
}

// LeftoverMessage is what Hedgehug tells of an unfinished rebuild of the
// store called name.
func LeftoverMessage(name string) string {
	return fmt.Sprintf("the rebuild of %s after a history rewrite is unfinished (hedgehug doctor --fix finishes it)", name)
}

// LeftoverError says a store has a rebuild left unfinished.
type LeftoverError struct{ Name string }

func (e *LeftoverError) Error() string { return LeftoverMessage(e.Name) }

// FinishLeftover completes a swap that stopped, or removes what a rebuild
// left. With no store at dir, the rebuilt one takes its place; with only an
// old one left, it is put back. Then what remains beside the store goes.
func FinishLeftover(dir string, l Leftover) error {
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		stale := false
		if l.New != "" {
			_, merr := os.Lstat(filepath.Join(l.New, staleMarker))
			stale = merr == nil
		}
		switch {
		case stale && len(l.Before) > 0:
			// The rebuild was found stale and the old store could not be put
			// back: it is put back now, and the rebuild goes with the rest.
			last := l.Before[len(l.Before)-1]
			if err := os.Rename(last, dir); err != nil {
				return fmt.Errorf("finish the rebuild: %w", err)
			}
			l.Before = l.Before[:len(l.Before)-1]
		case l.New != "":
			if err := os.Rename(l.New, dir); err != nil {
				return fmt.Errorf("finish the rebuild: %w", err)
			}
			l.New = ""
			// A marker with no old store to keep has nothing to say.
			if err := os.Remove(filepath.Join(dir, staleMarker)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("finish the rebuild: %w", err)
			}
		case len(l.Before) > 0:
			last := l.Before[len(l.Before)-1]
			if err := os.Rename(last, dir); err != nil {
				return fmt.Errorf("finish the rebuild: %w", err)
			}
			l.Before = l.Before[:len(l.Before)-1]
		}
	} else if err != nil {
		return err
	}
	var errs []error
	if l.New != "" {
		errs = append(errs, os.RemoveAll(l.New))
	}
	for _, p := range l.Before {
		// A swap that stopped may not have carried over the local files of
		// the old store (sync.json, the inbox); what is missing is brought
		// over first, and an old store that could not be read stays.
		if err := carryMissing(p, dir); err != nil {
			errs = append(errs, fmt.Errorf("carry over the local files of %s (it is left): %w", p, err))
			continue
		}
		errs = append(errs, os.RemoveAll(p))
	}
	return errors.Join(errs...)
}

// carryMissing brings the local files of the old store at from that the one
// at to lacks, and the lines of the inbox it lacks.
func carryMissing(from, to string) error {
	files, err := localFiles(from)
	if err != nil {
		return err
	}
	for name := range files {
		if slices.Contains(ownedLocal, name) {
			continue
		}
		if name == "inbox.jsonl" {
			old, err := os.ReadFile(filepath.Join(from, filepath.FromSlash(store.LocalDir), name)) //nolint:gosec // a file of the store
			if err != nil {
				return err
			}
			cur, err := os.ReadFile(filepath.Join(to, filepath.FromSlash(store.LocalDir), name)) //nolint:gosec // a file of the store
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if err := store.WriteLocalIn(to, name, joinLines(cur, old)); err != nil {
				return err
			}
			continue
		}
		_, err := os.Lstat(filepath.Join(to, filepath.FromSlash(store.LocalDir), filepath.FromSlash(name)))
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := copyFile(from, to, name); err != nil {
			return err
		}
	}
	return nil
}

// Discard removes the clone of b, which will not be swapped in.
func Discard(b *Rebuild) { _ = os.RemoveAll(b.Dir) }
