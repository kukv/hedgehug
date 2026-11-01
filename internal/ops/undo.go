package ops

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// Undo takes back the last operation of the writer, keeping what others
// changed after it. It returns an error of Code "undo.nothing" (KindConflict)
// when there is no operation to take back, and writes no commit then.
func (ss *Session) Undo() (UndoResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return UndoResult{}, err
	}
	acct, ok, err := ss.Writer()
	if err != nil {
		return UndoResult{}, err
	}
	if !ok {
		return UndoResult{}, newErr(KindConflict, "account.none", errors.New("accounts are not set up in config.yaml; hedgehug undo needs them to find your operations"))
	}
	var res UndoResult
	hash, err := ss.UpdateCommit(s, store.Op{Name: "undo"}, func(tx *store.Tx) error {
		var err error
		res, err = undoInTx(tx, s, ix, acct.ID())
		var fe *forwardHitError
		if errors.As(err, &fe) {
			if moved := ss.movedFor(ix, fe.ulid.String()); moved != nil {
				return moved
			}
			// the forward is there but the index cannot name it (a broken one)
			return newErr(KindNotFound, "task.moved", fmt.Errorf("%s moved to another store", fe.ulid),
				"input", fe.ulid.String(), "store", "", "store_ulid", "", "id", "")
		}
		if err == nil && !res.Found {
			return newErr(KindConflict, "undo.nothing", errors.New("nothing to undo"))
		}
		return err
	})
	if err != nil {
		return UndoResult{}, err
	}
	res.NewCommit = commitHex(hash)
	return res, nil
}

// undoSkip are the operations undo does not pick: the ones that bring work
// together or set a store up, which are not something a person did.
var undoSkip = []string{"merge", "repair", "setup", "undo", "init"}

// UndoTarget returns the newest commit of log, which runs newest first along
// the first parents, that the writer me made by an operation undo can take
// back and that no later undo took back.
func UndoTarget(log []gitrepo.LogEntry, me string) (gitrepo.LogEntry, bool) {
	undone := map[string]bool{}
	for _, e := range log {
		for _, h := range e.TrailerValues("Hedgehug-Undo") {
			undone[h] = true
		}
		author, _ := e.Trailer("Hedgehug-Author")
		op, _ := e.Trailer("Hedgehug-Op")
		if author == me && !slices.Contains(undoSkip, op) && !undone[e.Hash.String()] && len(e.Parents) > 0 {
			return e, true
		}
	}
	return gitrepo.LogEntry{}, false
}

// undoBlocked are the operations undo does not take back, with why.
var undoBlocked = map[string]string{
	"move-in":         "a move between stores cannot be undone; move it back with hedgehug mv --store",
	"move-out":        "a move between stores cannot be undone; move it back with hedgehug mv --store",
	"rewrite-history": "a history rewrite cannot be undone",
}

// forwardHitError is a task file that undo would write over a forward: the
// task moved after the operation. Undo turns it into task.moved.
type forwardHitError struct{ ulid model.ULID }

func (e *forwardHitError) Error() string { return e.ulid.String() + " moved to another store" }

// UndoKept is a value undo left as it is, because another writer changed it
// after the commit that is taken back.
type UndoKept struct {
	Path string
	Task string // the id of the task; empty for other files
	// Label is the id as a reader sees it (the ULID when the task has none),
	// which --json leaves out
	Label  string
	Field  string
	Author string
}

// UndoResult is what Undo did. Found is false when there was nothing to
// undo (Undo then returns an error too).
type UndoResult struct {
	Commit  gitrepo.LogEntry
	Found   bool
	Changed bool // a file differs from what HEAD had
	Kept    []UndoKept
	// NewCommit is the hex hash of the commit Undo made; "" when nothing was
	// committed.
	NewCommit string
}

// UndoSubject is the subject of c without the "hedgehug: " every commit of
// the store starts with.
func UndoSubject(c gitrepo.LogEntry) string { return strings.TrimPrefix(c.Subject, "hedgehug: ") }

// undoInTx takes back the last operation of the writer me, as Store.Update
// holds the lock: for each file the operation changed, the fields nobody
// changed since go back to what they were before it, and the fields others
// changed since stay.
func undoInTx(tx *store.Tx, s *store.Store, ix *index.Index, me string) (UndoResult, error) {
	var res UndoResult
	// Read under the lock, so the tasks under a moved one include those
	// added since the index was read.
	if err := ix.Refresh(); err != nil {
		return res, err
	}
	r := s.Repo()
	head, err := r.Head()
	if err != nil {
		return res, err
	}
	log, err := r.LogFrom(head, 0)
	if err != nil {
		return res, err
	}
	c, ok := UndoTarget(log, me)
	if !ok {
		return res, nil
	}
	op, _ := c.Trailer("Hedgehug-Op")
	if why, blocked := undoBlocked[op]; blocked {
		return res, newErr(KindConflict, "undo.blocked", errors.New(why))
	}
	res.Commit, res.Found = c, true
	tx.SetDetail(UndoSubject(c))
	tx.AddTrailer("Hedgehug-Undo", c.Hash.String())
	// A commit that took back nothing would be picked again by the next undo.
	tx.AllowEmpty()

	cTree, err := r.TreeOf(c.Hash)
	if err != nil {
		return res, err
	}
	pTree, err := r.TreeOf(c.Parents[0])
	if err != nil {
		return res, err
	}
	changes, err := r.DiffTrees(pTree, cTree, "")
	if err != nil {
		return res, err
	}
	// Nothing is written over a task that has moved since: it is told first.
	for _, ch := range changes {
		ulid, err := model.ParseTaskPath(ch.Path)
		if err != nil {
			continue
		}
		if b, err := r.ReadAt(head, ch.Path); err == nil && model.IsForward(b) {
			return res, &forwardHitError{ulid: ulid}
		}
	}
	u := &undoer{tx: tx, r: r, ix: ix, sc: s.Schema(), head: head, c: c, paths: map[string]bool{}, res: &res}
	for _, ch := range changes {
		u.paths[ch.Path] = true
	}
	for _, ch := range changes {
		if err := u.path(ch.Path); err != nil {
			return res, fmt.Errorf("%s: %w", ch.Path, err)
		}
	}
	return res, u.followMoves()
}

// undoer is the state of one undoInTx.
type undoer struct {
	tx    *store.Tx
	r     *gitrepo.Repo
	ix    *index.Index
	sc    *schema.Schema
	head  plumbing.Hash
	c     gitrepo.LogEntry // the commit taken back
	paths map[string]bool  // the files c changed
	res   *UndoResult
	moved []movedTask
}

// movedTask is a task undo put in another workspace.
type movedTask struct {
	ulid model.ULID
	ws   model.ULID
}

// read returns p as commit has it, or nil when it is not there.
func (u *undoer) read(commit plumbing.Hash, p string) ([]byte, error) {
	b, err := u.r.ReadAt(commit, p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// path takes back c's change to the file p.
func (u *undoer) path(p string) error {
	var files [3][]byte // c's version, HEAD's, and the version before c
	for i, commit := range []plumbing.Hash{u.c.Hash, u.head, u.c.Parents[0]} {
		b, err := u.read(commit, p)
		if err != nil {
			return err
		}
		files[i] = b
	}
	base, ours, theirs := files[0], files[1], files[2]
	if ours == nil && base != nil {
		return nil // removed since, and stays removed
	}
	m, err := merge.Merge(p, u.sc, base, merge.Side{Data: ours}, merge.Side{Data: theirs}, merge.OursWins)
	if err != nil {
		return err
	}
	if ulid, err := model.ParseTaskPath(p); err == nil {
		return u.task(ulid, p, files, m.Data)
	}
	if ulid, err := model.ParseWorkspacePath(p); err == nil {
		return u.workspace(ulid, ours, m.Data)
	}
	if p == schema.Path && m.Data != nil {
		sc, err := schema.Decode(m.Data)
		if err != nil {
			return err
		}
		u.res.Changed = u.res.Changed || !bytes.Equal(m.Data, ours)
		return u.tx.PutSchema(sc)
	}
	u.res.Changed = u.res.Changed || !bytes.Equal(m.Data, ours)
	if m.Data == nil {
		return u.tx.RemoveFile(p)
	}
	return u.tx.PutFile(p, m.Data)
}

// task writes the task file p, which merged is the merge of: with the id and
// former ids HEAD has, since taking back the ids would let a number be given
// twice. A task that goes to another workspace is noted, for its subtree to
// follow.
func (u *undoer) task(ulid model.ULID, p string, files [3][]byte, merged []byte) error {
	ours := files[1]
	var oursT *model.Task // nil when HEAD lacks the file: c purged the task
	if ours != nil {
		var err error
		if oursT, err = model.DecodeTask(ours); err != nil {
			return err
		}
		if err := u.keep(p, files, oursT); err != nil {
			return err
		}
	}
	if merged == nil {
		if oursT == nil {
			return nil
		}
		u.res.Changed = true
		return u.tx.RemoveTask(ulid)
	}
	t, err := model.DecodeTask(merged)
	if err != nil {
		return err
	}
	if oursT != nil {
		t.ID, t.FormerIDs = oursT.ID, oursT.FormerIDs
		if t.Workspace != oursT.Workspace {
			u.moved = append(u.moved, movedTask{ulid: ulid, ws: t.Workspace})
		}
	}
	enc, err := t.Encode()
	if err != nil {
		return err
	}
	u.res.Changed = u.res.Changed || !bytes.Equal(enc, ours)
	return u.tx.PutTask(t)
}

// keep notes the fields of the task that HEAD has otherwise than c had them:
// others changed them after c, so the merge left them. files are c's
// version, HEAD's and the one before c.
func (u *undoer) keep(p string, files [3][]byte, oursT *model.Task) error {
	if files[0] == nil {
		return nil // c removed the file: nothing to compare with
	}
	baseT, err := model.DecodeTask(files[0])
	if err != nil {
		return fmt.Errorf("the version c wrote does not read: %w", err)
	}
	changed := merge.TaskChanges(baseT, oursT)
	if len(changed) == 0 {
		return nil
	}
	authors, err := u.fieldAuthors(p)
	if err != nil {
		return err
	}
	for _, f := range changed {
		u.res.Kept = append(u.res.Kept, UndoKept{Path: p, Task: cmp.Or(oursT.ID, oursT.ULID.String()), Label: cmp.Or(u.ix.QualifyID(oursT.ID), oursT.ULID.String()), Field: f, Author: cmp.Or(authors[f], "someone")})
	}
	return nil
}

// fieldAuthors tells, for each field of the task file p, who changed it
// last after c: of the commits between c and HEAD, the newest by author time
// (the later in history among equals) whose version of the file differs in
// that field from each of its parents'.
func (u *undoer) fieldAuthors(p string) (map[string]string, error) {
	log, err := u.r.Between(u.head, u.c.Hash)
	if err != nil {
		return nil, err
	}
	// Each version is decoded once: a commit is read as itself and as the
	// parent of the next.
	type version struct {
		t   *model.Task
		err error
	}
	seen := map[plumbing.Hash]version{}
	task := func(commit plumbing.Hash) (*model.Task, error) {
		if v, ok := seen[commit]; ok {
			return v.t, v.err
		}
		var v version
		if b, err := u.read(commit, p); err != nil || b == nil {
			v.err = err
		} else {
			v.t, v.err = model.DecodeTask(b)
		}
		seen[commit] = v
		return v.t, v.err
	}
	out := map[string]string{}
	when := map[string]time.Time{}
	for _, e := range log {
		t, err := task(e.Hash)
		if err != nil || t == nil {
			continue // a version that does not read changed no field
		}
		var fields []string
		for i, ph := range e.Parents {
			pt, err := task(ph)
			if err != nil || pt == nil {
				fields = nil
				break
			}
			diff := merge.TaskChanges(pt, t)
			if i == 0 {
				fields = diff
			} else {
				fields = slices.DeleteFunc(fields, func(f string) bool { return !slices.Contains(diff, f) })
			}
		}
		for _, f := range fields {
			if w, ok := when[f]; !ok || !e.When.Before(w) {
				out[f], when[f] = e.Writer(), e.When
			}
		}
	}
	return out, nil
}

// workspace writes the workspace file ulid, which merged is the merge of. A
// workspace that still has tasks is not removed.
func (u *undoer) workspace(ulid model.ULID, ours, merged []byte) error {
	if merged == nil {
		if ours == nil {
			return nil
		}
		if n := CountTasks(u.ix, ulid, true); n > 0 {
			return newErr(KindConflict, "undo.blocked", fmt.Errorf("the workspace still has %d tasks; move them with hedgehug mv --workspace or purge them", n))
		}
		u.res.Changed = true
		return u.tx.RemoveWorkspace(ulid)
	}
	w, err := model.DecodeWorkspace(merged)
	if err != nil {
		return err
	}
	enc, err := w.Encode()
	if err != nil {
		return err
	}
	u.res.Changed = u.res.Changed || !bytes.Equal(enc, ours)
	return u.tx.PutWorkspace(w)
}

// followMoves puts the tasks under each task that went to another workspace
// in that workspace too, those added since c included: a task and its
// subtree are in one workspace, as hedgehug mv --workspace keeps them.
func (u *undoer) followMoves() error {
	for _, m := range u.moved {
		for _, d := range Descendants(u.ix, m.ulid, func(*index.Entry) bool { return true }) {
			t, err := u.tx.Task(d.ULID)
			if errors.Is(err, store.ErrNotFound) {
				continue // removed in this undo
			}
			if err != nil {
				return err
			}
			if t.Workspace == m.ws {
				continue
			}
			t.Workspace = m.ws
			u.res.Changed = true
			if err := u.tx.PutTask(t); err != nil {
				return err
			}
		}
	}
	return nil
}

// CountTasks counts the tasks of a workspace: the ones not deleted, or all
// of them when withDeleted.
func CountTasks(ix *index.Index, ws model.ULID, withDeleted bool) int {
	n := 0
	for _, e := range ix.Tasks() {
		if e.Workspace == ws && (withDeleted || e.DeletedAt == nil) {
			n++
		}
	}
	return n
}
