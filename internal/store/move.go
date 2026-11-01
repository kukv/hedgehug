package store

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"path"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Unlink is a reference a move cuts: from Task's Field to Target. Field is
// "blocked_by" or "props.<key>".
type Unlink struct {
	Task   model.ULID
	Field  string
	Target model.ULID
}

// String is the value of a Hedgehug-Unlinked trailer: "<task> <field> <target>".
func (u Unlink) String() string {
	return u.Task.String() + " " + u.Field + " " + u.Target.String()
}

const propsPrefix = "props."

// Unlinks are the references between the tasks moved and the tasks that
// stay (rest), both ways: blocked_by and the properties that sc types
// task_ref. References inside either group are not listed. They come in the
// order of Task, Field, Target.
func Unlinks(sc *schema.Schema, moved, rest []*model.Task) []Unlink {
	in := func(list []*model.Task) map[model.ULID]bool {
		m := make(map[model.ULID]bool, len(list))
		for _, t := range list {
			m[t.ULID] = true
		}
		return m
	}
	var out []Unlink
	collect := func(from []*model.Task, to map[model.ULID]bool) {
		for _, t := range from {
			for _, r := range refsOf(t, sc) {
				if r.field == "parent" || !to[r.ulid] {
					continue
				}
				field := r.field
				if field != "blocked_by" {
					field = propsPrefix + field
				}
				out = append(out, Unlink{Task: t.ULID, Field: field, Target: r.ulid})
			}
		}
	}
	collect(moved, in(rest))
	collect(rest, in(moved))
	slices.SortFunc(out, func(a, b Unlink) int {
		return cmp.Or(
			strings.Compare(a.Task.String(), b.Task.String()),
			strings.Compare(a.Field, b.Field),
			strings.Compare(a.Target.String(), b.Target.String()),
		)
	})
	return slices.Compact(out)
}

// dropRef takes the reference u off t, which must be a task of its own (not
// one shared with a reader).
func dropRef(t *model.Task, u Unlink) {
	if u.Field == "blocked_by" {
		t.BlockedBy = slices.DeleteFunc(slices.Clone(t.BlockedBy), func(x model.ULID) bool { return x == u.Target })
		return
	}
	key := strings.TrimPrefix(u.Field, propsPrefix)
	n := t.Props.Get(key)
	if n == nil {
		return
	}
	is := func(n *yaml.Node) bool {
		x, err := model.ParseULID(n.Value)
		return n.Kind == yaml.ScalarNode && err == nil && x == u.Target
	}
	if n.Kind != yaml.SequenceNode {
		if is(n) {
			t.Props.Delete(key)
		}
		return
	}
	left := slices.DeleteFunc(slices.Clone(n.Content), is)
	if len(left) == 0 {
		t.Props.Delete(key)
		return
	}
	// A new node: the old one may be the one the task's document holds.
	nn := *n
	nn.Content = left
	t.Props.Set(key, &nn)
}

// cloneTask copies t deeply, through its file form.
func cloneTask(t *model.Task) (*model.Task, error) {
	b, err := t.Encode()
	if err != nil {
		return nil, err
	}
	return model.DecodeTask(b)
}

// cloneSchema copies sc deeply, through its file form.
func cloneSchema(sc *schema.Schema) (*schema.Schema, error) {
	b, err := sc.Encode()
	if err != nil {
		return nil, err
	}
	return schema.Decode(b)
}

// heldLock marks a store whose lock LockInOrder holds, and the goroutine
// that holds it.
type heldLock struct{ gid uint64 }

// heldByThisGoroutine reports whether LockInOrder holds the lock of s for
// the calling goroutine, which then does not take it again.
func (s *Store) heldByThisGoroutine() bool {
	h := s.held.Load()
	return h != nil && h.gid == goroutineID()
}

// goroutineID is the number of the calling goroutine. Go has no other way to
// tell the goroutine that holds a lock from the others, and an Update of
// another goroutine must wait for the lock like one of another process.
func goroutineID() uint64 {
	var buf [64]byte
	b := bytes.TrimPrefix(buf[:runtime.Stack(buf[:], false)], []byte("goroutine "))
	if i := bytes.IndexByte(b, ' '); i >= 0 {
		b = b[:i]
	}
	id, _ := strconv.ParseUint(string(b), 10, 64)
	return id
}

// Locked holds the store locks of several stores, taken in the order of the
// stores' ULIDs: the file lock, which keeps other processes out, and the
// mutex of the Store, which keeps the other goroutines out. While it is held,
// Update and UpdateCommit of those stores in the goroutine that took it use
// them and do not take them again; the others wait for Release.
type Locked struct {
	stores []*Store
	locks  []*fileLock
	once   sync.Once
}

// afterLock is called with each store after LockInOrder took its lock;
// tests use it to see the order.
var afterLock = func(*Store) {}

// LockInOrder takes the store lock of each of ss, smallest ULID first. A
// store given twice is locked once.
func LockInOrder(ss ...*Store) (*Locked, error) {
	var uniq []*Store
	for _, s := range ss {
		if !slices.ContainsFunc(uniq, func(o *Store) bool { return o == s || o.meta.ULID == s.meta.ULID }) {
			uniq = append(uniq, s)
		}
	}
	slices.SortFunc(uniq, func(a, b *Store) int {
		return strings.Compare(a.meta.ULID.String(), b.meta.ULID.String())
	})
	l := &Locked{}
	gid := goroutineID()
	for _, s := range uniq {
		s.mu.Lock()
		fl, err := acquire(s.path(lockPath), s.lockWait)
		if err != nil {
			s.mu.Unlock()
			l.Release()
			return nil, fmt.Errorf("lock %s: %w", s.meta.Name, err)
		}
		l.stores = append(l.stores, s)
		l.locks = append(l.locks, fl)
		s.held.Store(&heldLock{gid: gid})
		afterLock(s)
	}
	return l, nil
}

// Release lets the locks go, the last taken first. It may be called more
// than once.
func (l *Locked) Release() {
	l.once.Do(func() {
		for i := len(l.stores) - 1; i >= 0; i-- {
			l.stores[i].held.Store(nil)
			// The OS drops the lock with the file whatever release says.
			_ = l.locks[i].release()
			l.stores[i].mu.Unlock()
		}
	})
}

// MoveIn is what the destination store writes (spec step 7).
type MoveIn struct {
	From      model.ULID        // the source store's ulid
	Tasks     []*model.Task     // the tasks as the source has them; the root first, every parent before its children
	Workspace model.ULID        // where they go in this store
	Parent    *model.ULID       // the new parent of the root; nil for the workspace's root
	Notes     map[string][]byte // the comment and worklog files of the tasks, by path (same paths here)
	Unlinked  []Unlink
	Scoped    *schema.Scoped // to add to this store's schema.yaml, or nil
}

// MoveIn writes in in tx: each task with its workspace set to in.Workspace,
// the root under in.Parent and last among its new siblings, references to
// tasks outside in.Tasks taken off (in.Unlinked), updated set to now, and its
// id given again by this store (the old one goes to former_ids, which keeps
// what it had). It adds the trailers of spec step 7 and D11: Hedgehug-Task for
// each task but the root, which Op.Task names (the root too when Op.Task is
// not set), Hedgehug-Moved-From and Hedgehug-Unlinked.
//
// It writes copies: in.Tasks are not changed, and the new ids are in the
// store after the commit.
func (tx *Tx) MoveIn(in MoveIn) error {
	if tx.done {
		return errTxDone
	}
	if len(in.Tasks) == 0 || in.From.IsZero() || in.Workspace.IsZero() {
		return errors.New("a move needs tasks, the source store and a workspace")
	}
	inM := make(map[model.ULID]bool, len(in.Tasks))
	for _, t := range in.Tasks {
		inM[t.ULID] = true
	}
	root := in.Tasks[0]
	for _, t := range in.Tasks[1:] {
		if t.Parent == nil || !inM[*t.Parent] {
			return fmt.Errorf("task %s is moved without its parent", t.ULID)
		}
	}
	for rel := range in.Notes {
		if !slices.ContainsFunc(in.Tasks, func(t *model.Task) bool { return isNoteOf(rel, t.ULID) }) {
			return fmt.Errorf("%s is not a comment or worklog entry of a task moved", rel)
		}
	}

	moved := make([]*model.Task, len(in.Tasks))
	for i, src := range in.Tasks {
		t, err := cloneTask(src)
		if err != nil {
			return fmt.Errorf("task %s: %w", src.ULID, err)
		}
		t.Workspace = in.Workspace
		if i == 0 {
			t.Parent = in.Parent
		}
		for _, u := range in.Unlinked {
			if u.Task == t.ULID {
				dropRef(t, u)
			}
		}
		// assignIDs gives the id of this store to a task without one.
		if t.ID != "" && !slices.Contains(t.FormerIDs, t.ID) {
			t.FormerIDs = append(t.FormerIDs, t.ID)
		}
		t.ID = ""
		moved[i] = t
	}

	if in.Scoped != nil {
		base := tx.schema
		if base == nil {
			base = tx.s.Schema()
		}
		sc, err := cloneSchema(base)
		if err != nil {
			return err
		}
		sc.Scoped = slices.DeleteFunc(sc.Scoped, func(s schema.Scoped) bool { return s.Where == in.Scoped.Where })
		sc.Scoped = append(sc.Scoped, *in.Scoped)
		if err := tx.PutSchema(sc); err != nil {
			return err
		}
	}

	sibs, err := tx.siblings(moved[0])
	if err != nil {
		return err
	}
	var last model.Rank
	for _, o := range sibs {
		last = max(last, o.Rank)
	}
	if moved[0].Rank, err = model.Between(last, ""); err != nil {
		return err
	}
	for _, t := range moved {
		if err := tx.PutTask(t); err != nil {
			return err
		}
	}
	for _, rel := range slices.Sorted(func(yield func(string) bool) {
		for rel := range in.Notes {
			if !yield(rel) {
				return
			}
		}
	}) {
		if err := tx.PutFile(rel, in.Notes[rel]); err != nil {
			return err
		}
	}

	if tx.op.Task.IsZero() {
		tx.AddTrailer("Hedgehug-Task", root.ULID.String())
	}
	rest := make([]model.ULID, 0, len(in.Tasks)-1)
	for _, t := range in.Tasks[1:] {
		rest = append(rest, t.ULID)
	}
	tx.addMoveTrailers(rest, "Hedgehug-Moved-From", in.From, in.Unlinked)
	return nil
}

// isNoteOf reports whether rel is a comment or a worklog entry of the task.
func isNoteOf(rel string, task model.ULID) bool {
	for _, p := range []string{model.CommentPath(task, model.ULID{}), model.WorklogPath(task, model.ULID{})} {
		if dir := path.Dir(p) + "/"; strings.HasPrefix(rel, dir) && !strings.Contains(rel[len(dir):], "/") {
			return true
		}
	}
	return false
}

// addMoveTrailers adds a Hedgehug-Task for each of rest in ULID order, then
// the trailer key for store, then a Hedgehug-Unlinked for each unlink.
func (tx *Tx) addMoveTrailers(rest []model.ULID, key string, store model.ULID, unlinked []Unlink) {
	rest = slices.Clone(rest)
	slices.SortFunc(rest, func(a, b model.ULID) int { return strings.Compare(a.String(), b.String()) })
	for _, ulid := range rest {
		tx.AddTrailer("Hedgehug-Task", ulid.String())
	}
	tx.AddTrailer(key, store.String())
	for _, u := range unlinked {
		tx.AddTrailer("Hedgehug-Unlinked", u.String())
	}
}

// MoveOut is what the source store writes (spec step 8).
type MoveOut struct {
	To       model.ULID   // the destination store's ulid
	Tasks    []model.ULID // the tasks moved; the root first
	Unlinked []Unlink
	Scoped   string // the where (project:<ULID>) of the scoped entry to remove, or ""
}

// MoveOut writes out in tx: each task file becomes a forward to out.To, its
// comments and worklog entries go, the tasks that stay lose their references
// to them (out.Unlinked), numbers.yaml keeps their numbers (D13: those of the
// prefixes of this store's workspaces), and the scoped entry goes. It adds
// the trailers of spec step 8 and D11, and when the operation has neither a
// Task nor a Detail, sets the Detail to the root's id and title.
func (tx *Tx) MoveOut(out MoveOut) error {
	if tx.done {
		return errTxDone
	}
	if len(out.Tasks) == 0 || out.To.IsZero() {
		return errors.New("a move needs tasks and the destination store")
	}
	inM := make(map[model.ULID]bool, len(out.Tasks))
	for _, ulid := range out.Tasks {
		inM[ulid] = true
	}
	prefixes := map[string]bool{}
	for w, err := range tx.s.Workspaces() {
		if err == nil {
			prefixes[w.IDPrefix] = true
		}
	}
	for i, ulid := range out.Tasks {
		t, err := tx.task(ulid)
		if err != nil {
			return fmt.Errorf("move out %s: %w", ulid, err)
		}
		if i == 0 && tx.op.Task.IsZero() && tx.op.Detail == "" {
			label := t.ID
			if label == "" {
				label = ulid.String()
			}
			tx.SetDetail(strings.TrimSpace(label + " " + t.Title))
		}
		if err := tx.recordNumbersOf(t, prefixes); err != nil {
			return err
		}
		notes, err := tx.noteFiles(ulid)
		if err != nil {
			return err
		}
		for _, rel := range notes {
			if err := tx.RemoveFile(rel); err != nil {
				return err
			}
		}
		b, err := (&model.Forward{ULID: ulid, MovedTo: out.To}).Encode()
		if err != nil {
			return err
		}
		if err := tx.PutFile(model.TaskPath(ulid), b); err != nil {
			return err
		}
	}
	for _, u := range out.Unlinked {
		if inM[u.Task] {
			continue // the task becomes a forward
		}
		t, err := tx.task(u.Task)
		if err != nil {
			return fmt.Errorf("unlink %s: %w", u, err)
		}
		dropRef(t, u)
		if err := tx.PutTask(t); err != nil {
			return err
		}
	}
	if out.Scoped != "" {
		base := tx.schema
		if base == nil {
			base = tx.s.Schema()
		}
		if slices.ContainsFunc(base.Scoped, func(s schema.Scoped) bool { return s.Where == out.Scoped }) {
			sc, err := cloneSchema(base)
			if err != nil {
				return err
			}
			sc.Scoped = slices.DeleteFunc(sc.Scoped, func(s schema.Scoped) bool { return s.Where == out.Scoped })
			if err := tx.PutSchema(sc); err != nil {
				return err
			}
		}
	}

	tx.AddTrailer("Hedgehug-Task", out.Tasks[0].String())
	tx.addMoveTrailers(out.Tasks[1:], "Hedgehug-Moved-To", out.To, out.Unlinked)
	return nil
}
