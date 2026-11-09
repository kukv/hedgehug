package tasks

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

	"github.com/kukv/hedgehug/internal/app/domain/merge"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

// Undo takes back the last operation of the writer, keeping what others
// changed after it. It returns an error of Code "undo.nothing" (KindConflict)
// when there is no operation to take back, and writes no commit then.
func Undo(ss *usecase.Session) (UndoResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return UndoResult{}, err
	}
	acct, ok, err := ss.Writer()
	if err != nil {
		return UndoResult{}, err
	}
	if !ok {
		return UndoResult{}, usecase.NewErr(usecase.KindConflict, "account.none", errors.New("no account is set up; hedgehug undo needs one to find your operations; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)"))
	}
	var res UndoResult
	// Old values go back, which is no move of the workflow, and the task was
	// in that status without what it lacks.
	hash, err := ss.UpdateCommit(s, store.Op{Name: "undo", SkipTransitions: true, SkipRequired: true}, func(tx *store.Tx) error {
		var err error
		res, err = undoInTx(tx, s, ix, acct.ID())
		var fe *forwardHitError
		if errors.As(err, &fe) {
			if moved := ss.MovedFor(ix, fe.ulid.String()); moved != nil {
				return moved
			}
			// the forward is there but the index cannot name it (a broken one)
			return usecase.MovedUnknownErr(fe.ulid)
		}
		if err == nil && !res.Found {
			return usecase.NewErr(usecase.KindConflict, "undo.nothing", errors.New("nothing to undo"))
		}
		return err
	})
	if err != nil {
		return UndoResult{}, err
	}
	res.NewCommit = usecase.CommitHex(hash)
	// The commit taken back, named as the index is after the undo.
	if err := ix.Refresh(); err != nil {
		return UndoResult{}, err
	}
	lv := usecase.LogViewOf(ix, res.commit)
	res.Undone = &lv
	return res, nil
}

// undoSkip are the operations undo does not pick: the ones that bring work
// together or set a store up, which are not something a person did. A
// conversion (hh store convert) is one of them: it is not taken back, and the
// undo after it takes back the operation before it.
var undoSkip = []string{"merge", "repair", "setup", "undo", "init", "convert", "gh-fetch", usecase.OpGitHubMirror, usecase.OpGitHubPush}

// undoScanner finds the commit to undo, one entry at a time, newest first,
// along the first parents: the newest that the writer made by an operation
// undo can take back and that no later undo took back.
type undoScanner struct{ undone map[string]bool }

// next reports whether e is the commit to undo, having seen the newer ones.
func (s *undoScanner) next(e gitrepo.LogEntry, me string) (gitrepo.LogEntry, bool) {
	if s.undone == nil {
		s.undone = map[string]bool{}
	}
	for _, h := range e.TrailerValues("Hedgehug-Undo") {
		s.undone[h] = true
	}
	author, _ := e.Trailer("Hedgehug-Author")
	op, _ := e.Trailer("Hedgehug-Op")
	if author == me && !slices.Contains(undoSkip, op) && !s.undone[e.Hash.String()] && len(e.Parents) > 0 {
		return e, true
	}
	return gitrepo.LogEntry{}, false
}

// undoBlocked are the operations undo does not take back, with why.
var undoBlocked = map[string]string{
	"move-in":  "a move between stores cannot be undone; move it back with hedgehug mv --store",
	"move-out": "a move between stores cannot be undone; move it back with hedgehug mv --store",
	"purge":    "a purge cannot be undone",
}

// forwardHitError is a task file that undo would write over a forward: the
// task moved after the operation. Undo turns it into task.moved.
type forwardHitError struct{ ulid model.ULID }

func (e *forwardHitError) Error() string { return e.ulid.String() + " moved to another store" }

// UndoKept is a value undo left as it is, because another writer changed it
// after the commit that is taken back.
type UndoKept struct {
	Path string `json:"path"`
	Task string `json:"task,omitempty" doc:"The ID of the task; empty for other files"`
	// Label is the id as a reader sees it (the ULID when the task has none),
	// which --json leaves out
	Label  string `json:"-"`
	Field  string `json:"field"`
	Author string `json:"author"`
}

// UndoWrite is what undo answers, in the CLI's --json and in the server's
// response.
type UndoWrite struct {
	Undone  *usecase.LogView `json:"undone" doc:"The commit taken back; null only in the CLI when there was nothing to undo (the server answers an undo.nothing problem)"`
	Subject string           `json:"subject" doc:"The subject of the commit taken back, without the hedgehug: prefix"`
	Changed bool             `json:"changed" doc:"false when every field was changed later by others, so nothing was written"`
	Commit  *string          `json:"commit"`
	Kept    []UndoKept       `json:"kept" doc:"What others changed after it, which undo kept"`
}

// UndoResult is what Undo did. Found is false when there was nothing to
// undo (Undo then returns an error too).
type UndoResult struct {
	// Undone is the commit taken back; nil when Found is false.
	Undone  *usecase.LogView
	Found   bool
	Changed bool // a file differs from what HEAD had
	Kept    []UndoKept
	// NewCommit is the hex hash of the commit Undo made; "" when nothing was
	// committed.
	NewCommit string

	commit gitrepo.LogEntry // the commit taken back, which Undone is made of
}

// Write is the answer to the undo. Of the result of an undo that found
// nothing it is {undone: null, subject: "", changed: false, commit: null,
// kept: []}.
func (r UndoResult) Write() UndoWrite {
	w := UndoWrite{Undone: r.Undone, Changed: r.Changed, Commit: usecase.CommitPtr(r.NewCommit), Kept: r.Kept}
	if r.Undone != nil {
		w.Subject = UndoSubject(r.Undone.Subject)
	}
	if w.Kept == nil {
		w.Kept = []UndoKept{}
	}
	return w
}

// UndoSubject is subject without the "hedgehug: " every commit of the store
// starts with.
func UndoSubject(subject string) string { return strings.TrimPrefix(subject, "hedgehug: ") }

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
	// Read the first parents only until the commit to undo is found.
	var sc undoScanner
	var c gitrepo.LogEntry
	var ok bool
	err = r.WalkFirstParents(head, func(e gitrepo.LogEntry) bool {
		c, ok = sc.next(e, me)
		return !ok
	})
	if err != nil {
		return res, err
	}
	if !ok {
		return res, nil
	}
	op, _ := c.Trailer("Hedgehug-Op")
	if why, blocked := undoBlocked[op]; blocked {
		return res, usecase.NewErr(usecase.KindConflict, "undo.blocked", errors.New(why), "op", op)
	}
	res.commit, res.Found = c, true
	tx.SetDetail(UndoSubject(c.Subject))
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
	u := &undoer{tx: tx, r: r, ix: ix, sc: s.Schema(), head: head, c: c, paths: map[string]bool{}, commits: map[plumbing.Hash]*upgrade.Commit{}, res: &res}
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
	// commits read a commit's task files in the newest format, by commit
	commits map[plumbing.Hash]*upgrade.Commit
	res     *UndoResult
	moved   []movedTask
}

// movedTask is a task undo put in another workspace.
type movedTask struct {
	ulid model.ULID
	ws   model.ULID
}

// read returns p as commit has it, or nil when it is not there.
// A task or template file comes back in the newest store format, whatever the
// format of the commit (domain/upgrade), so that the versions of a file are
// compared and merged in one form.
func (u *undoer) read(commit plumbing.Hash, p string) ([]byte, error) {
	b, err := u.r.ReadAt(commit, p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tpl, isTpl := model.TemplateNameOf(p)
	if _, perr := model.ParseTaskPath(p); perr != nil && !isTpl {
		return b, nil //nolint:nilerr // not a task or template file
	}
	up := u.upgraded(commit)
	if isTpl {
		return up.Template(tpl, b)
	}
	return up.Task(b)
}

// upgraded reads the files of commit in the newest format.
func (u *undoer) upgraded(commit plumbing.Hash) *upgrade.Commit {
	up, ok := u.commits[commit]
	if !ok {
		up = upgrade.At(func(p string) ([]byte, error) { return u.r.ReadAt(commit, p) }, u.r.ReadHead)
		u.commits[commit] = up
	}
	return up
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
	if err := checkCommentNotPushed(p, base, ours, theirs); err != nil {
		return err
	}
	if _, isView := model.IsViewPath(p); (p == schema.Path || isView) && theirs != nil {
		// The version to go back to is in the form of an older format: the
		// schema.yaml that schema.Decode does not read, or a view whose query
		// and columns use names that are gone. No step upgrades them.
		f, err := u.upgraded(u.c.Parents[0]).Format()
		if err != nil {
			return err
		}
		if f < model.StoreFormat {
			return usecase.NewErr(usecase.KindConflict, "undo.unsupported", fmt.Errorf("%s from before the store was converted cannot be restored; change it with hedgehug schema or hedgehug view", p))
		}
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

// checkCommentNotPushed refuses to take back the adding of a comment that has
// been pushed to GitHub (its file has gh): the comment would stay there. p is
// the file, and base, ours and theirs its versions as c wrote it, as HEAD has
// it, and as it was before c (nil when absent).
func checkCommentNotPushed(p string, base, ours, theirs []byte) error {
	if _, _, err := model.ParseCommentPath(p); err != nil || base == nil || ours == nil || theirs != nil {
		return nil //nolint:nilerr // not a comment that c added
	}
	c, err := model.DecodeComment(ours)
	if err != nil {
		return err
	}
	links, err := c.GitHub()
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	return usecase.NewErr(usecase.KindInvalid, "undo.comment_pushed", errors.New("the comment was sent to GitHub, so adding it cannot be undone; remove it with hedgehug comment rm, which removes it from GitHub too"))
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
	// gh is a machine's value (gh-fetch writes it): no one kept it by hand.
	changed := slices.DeleteFunc(merge.TaskChanges(baseT, oursT), func(f string) bool { return f == "gh" || strings.HasPrefix(f, "gh.") })
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
		if n := usecase.CountTasks(u.ix, ulid, true); n > 0 {
			return usecase.NewErr(usecase.KindConflict, "undo.blocked", fmt.Errorf("the workspace still has %[1]s; move %[2]s with hedgehug mv --workspace, or delete %[2]s with hedgehug rm and remove %[2]s with hedgehug purge --older-than 0d --store <store>", plural.Count(n, "task"), plural.Pick(n, "it", "them")))
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
		for _, d := range usecase.Descendants(u.ix, m.ulid, func(*index.Entry) bool { return true }) {
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
