package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

var errTxDone = errors.New("the transaction is used after Update returned")

// Op describes one operation, which becomes one commit.
type Op struct {
	Name   string     // add, update, rm, mv...; the Hedgehug-Op trailer
	Task   model.ULID // the main task of the operation; zero if none
	Author gitrepo.Signature
	// Account is the ID of the writer's account (host/login), for the
	// Hedgehug-Author trailer; empty if the store has none.
	Account string
	// Detail ends the subject when there is no Task, saying what changed,
	// such as "add web (WEB)".
	Detail string
	// Workspace is the workspace the operation changes, for the
	// Hedgehug-Workspace trailer; zero if none.
	Workspace model.ULID
	// SkipTransitions skips the check of workflow.transitions; a required
	// field of the new status is still checked, but only when the task had
	// it before the write.
	SkipTransitions bool
	// Trailers and AllowEmpty are set from inside fn, by AddTrailer and
	// AllowEmpty, because what they hold is read under the store lock.
	Trailers   []gitrepo.Trailer
	AllowEmpty bool
}

// Tx collects the writes of one operation. Its Put methods only queue a
// write; Update writes them all after fn returns.
type Tx struct {
	s    *Store
	op   Op
	done bool

	order   []string // paths in the order first put
	pending map[string]*pending

	tasks     map[model.ULID]*model.Task // put tasks; nil for a removed one
	removed   map[model.ULID]bool        // tasks given to RemoveTask
	taskOrder []model.ULID
	schema    *schema.Schema // set by PutSchema

	workspaces map[model.ULID]*model.Workspace // put workspaces; nil for a removed one
	numbers    model.Numbers                   // the purged numbers; nil until read
	oldIDs     []oldID                         // what assignIDs changed
	oldRanks   []oldRank                       // what rerank changed
}

// pending is one queued write. created and updated point into the value
// being written, so Update can stamp it; they are nil for files without
// timestamps.
type pending struct {
	encode           func() ([]byte, error)
	created, updated *time.Time
	remove           bool
}

// change is a write that alters a file, with what was there before.
type change struct {
	rel     string
	data    []byte // nil when removing
	old     []byte
	existed bool
}

func (tx *Tx) put(rel string, p *pending) error {
	if tx.done {
		return errTxDone
	}
	if _, ok := tx.pending[rel]; !ok {
		tx.order = append(tx.order, rel)
	}
	tx.pending[rel] = p
	return nil
}

// Task reads a task as it will be after the writes queued so far.
func (tx *Tx) Task(ulid model.ULID) (*model.Task, error) {
	if tx.done {
		return nil, errTxDone
	}
	return tx.task(ulid)
}

func (tx *Tx) task(ulid model.ULID) (*model.Task, error) {
	if t, ok := tx.tasks[ulid]; ok {
		if t == nil {
			return nil, fmt.Errorf("%s: %w", model.TaskPath(ulid), ErrNotFound)
		}
		return t, nil
	}
	return tx.s.Task(ulid)
}

// PutTask queues a write of t. Update fills in created and updated.
func (tx *Tx) PutTask(t *model.Task) error {
	if t.ULID.IsZero() {
		return errors.New("the task has no ulid")
	}
	if err := tx.put(model.TaskPath(t.ULID), &pending{encode: t.Encode, created: &t.Created, updated: &t.Updated}); err != nil {
		return err
	}
	// Not the tasks map: a task removed earlier in the operation is in it.
	if !slices.Contains(tx.taskOrder, t.ULID) {
		tx.taskOrder = append(tx.taskOrder, t.ULID)
	}
	tx.tasks[t.ULID] = t
	return nil
}

func (tx *Tx) PutComment(task model.ULID, c *model.Comment) error {
	if task.IsZero() || c.ULID.IsZero() {
		return errors.New("the comment has no ulid or no task")
	}
	if tx.removed[task] {
		return fmt.Errorf("task %s is removed in this operation", task)
	}
	return tx.put(model.CommentPath(task, c.ULID), &pending{encode: c.Encode, created: &c.Created, updated: &c.Updated})
}

func (tx *Tx) PutWorklog(task model.ULID, w *model.Worklog) error {
	if task.IsZero() || w.ULID.IsZero() {
		return errors.New("the worklog entry has no ulid or no task")
	}
	if tx.removed[task] {
		return fmt.Errorf("task %s is removed in this operation", task)
	}
	return tx.put(model.WorklogPath(task, w.ULID), &pending{encode: w.Encode, created: &w.Created, updated: &w.Updated})
}

func (tx *Tx) PutSprint(sp *model.Sprint) error {
	if sp.ULID.IsZero() {
		return errors.New("the sprint has no ulid")
	}
	return tx.put(model.SprintPath(sp.ULID), &pending{encode: sp.Encode, created: &sp.Created, updated: &sp.Updated})
}

func (tx *Tx) PutWorkspace(w *model.Workspace) error {
	if w.ULID.IsZero() {
		return errors.New("the workspace has no ulid")
	}
	if err := tx.put(model.WorkspacePath(w.ULID), &pending{encode: w.Encode, created: &w.Created, updated: &w.Updated}); err != nil {
		return err
	}
	tx.workspaces[w.ULID] = w
	return nil
}

func (tx *Tx) RemoveWorkspace(ulid model.ULID) error {
	if err := tx.put(model.WorkspacePath(ulid), &pending{remove: true}); err != nil {
		return err
	}
	tx.workspaces[ulid] = nil
	return nil
}

// SetDetail replaces the Detail of the operation, so fn can say what changed
// from what it read under the lock.
func (tx *Tx) SetDetail(s string) { tx.op.Detail = s }

// AddTrailer adds a trailer to the commit of the operation, after the ones
// the store writes.
func (tx *Tx) AddTrailer(key, value string) {
	tx.op.Trailers = append(tx.op.Trailers, gitrepo.Trailer{Key: key, Value: value})
}

// AllowEmpty makes the operation commit even when it changes no file, so
// that the commit's trailers record that it ran.
func (tx *Tx) AllowEmpty() { tx.op.AllowEmpty = true }

// RemoveFile queues the removal of rel, any file of the store that git
// tracks, under the same limits as PutFile. A file that is not there is left
// alone.
func (tx *Tx) RemoveFile(rel string) error {
	if err := trackedPath(rel); err != nil {
		return err
	}
	return tx.put(path.Clean(rel), &pending{remove: true})
}

// Workspace reads a workspace as it will be after the writes queued so far.
func (tx *Tx) Workspace(ulid model.ULID) (*model.Workspace, error) {
	if tx.done {
		return nil, errTxDone
	}
	return tx.workspace(ulid)
}

func (tx *Tx) workspace(ulid model.ULID) (*model.Workspace, error) {
	if w, ok := tx.workspaces[ulid]; ok {
		if w == nil {
			return nil, fmt.Errorf("%s: %w", model.WorkspacePath(ulid), ErrNotFound)
		}
		return w, nil
	}
	return tx.s.Workspace(ulid)
}

func (tx *Tx) PutRelease(r *model.Release) error {
	if r.ULID.IsZero() {
		return errors.New("the release has no ulid")
	}
	return tx.put(model.ReleasePath(r.ULID), &pending{encode: r.Encode, created: &r.Created, updated: &r.Updated})
}

func (tx *Tx) PutTemplate(t *model.Template) error {
	if err := model.ValidTemplateName(t.Name); err != nil {
		return err
	}
	return tx.put(model.TemplatePath(t.Name), &pending{encode: t.Encode, created: &t.Created, updated: &t.Updated})
}

// PutSchema queues a write of schema.yaml. Update refuses a schema that
// fails its own Check, and checks the tasks of the same operation against
// the new schema.
func (tx *Tx) PutSchema(sc *schema.Schema) error {
	if err := tx.put(schemaPath, &pending{encode: sc.Encode}); err != nil {
		return err
	}
	tx.schema = sc
	return nil
}

// PutFile queues a write of data to rel, any file of the store that git
// tracks, such as .gitattributes. Files of .hedgehug/local and .git are
// refused.
func (tx *Tx) PutFile(rel string, data []byte) error {
	if err := trackedPath(rel); err != nil {
		return err
	}
	return tx.put(path.Clean(rel), &pending{encode: func() ([]byte, error) { return data, nil }})
}

// trackedPath refuses rel unless it is a file of the store that git tracks.
func trackedPath(rel string) error {
	clean := path.Clean(rel)
	first, _, _ := strings.Cut(clean, "/")
	lower := strings.ToLower(clean) // case-insensitive file systems
	if !filepath.IsLocal(filepath.FromSlash(rel)) || strings.EqualFold(first, ".git") ||
		lower == LocalDir || strings.HasPrefix(lower, LocalDir+"/") {
		return fmt.Errorf("%q is not a tracked file of the store", rel)
	}
	return nil
}

// RemoveTask queues the removal of a task file with its comments and
// worklog entries, for purging. Soft deletion is a PutTask with DeletedAt.
func (tx *Tx) RemoveTask(ulid model.ULID) error {
	if tx.done {
		return errTxDone
	}
	if ulid.IsZero() {
		return errors.New("the task has no ulid")
	}
	var de *model.DecodeError
	if t, err := tx.task(ulid); err == nil {
		if err := tx.recordNumbers(t); err != nil {
			return err
		}
	} else if !errors.Is(err, ErrNotFound) && !errors.As(err, &de) {
		return err
	}
	rels := []string{model.TaskPath(ulid)}
	// Files put earlier in this operation are removed too.
	for _, rel := range tx.order {
		for _, root := range []string{model.CommentPath(ulid, model.ULID{}), model.WorklogPath(ulid, model.ULID{})} {
			if strings.HasPrefix(rel, path.Dir(root)+"/") && !slices.Contains(rels, rel) {
				rels = append(rels, rel)
			}
		}
	}
	for _, dir := range []string{path.Dir(model.CommentPath(ulid, model.ULID{})), path.Dir(model.WorklogPath(ulid, model.ULID{}))} {
		entries, err := os.ReadDir(tx.s.path(dir))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if rel := dir + "/" + e.Name(); !e.IsDir() && !slices.Contains(rels, rel) {
				rels = append(rels, rel)
			}
		}
	}
	for _, rel := range rels {
		if err := tx.put(rel, &pending{remove: true}); err != nil {
			return err
		}
	}
	tx.tasks[ulid] = nil
	if tx.removed == nil {
		tx.removed = map[model.ULID]bool{}
	}
	tx.removed[ulid] = true
	return nil
}

// recordNumbers raises the record of purged numbers to the numbers of t's
// ids, so numbering never gives them again.
func (tx *Tx) recordNumbers(t *model.Task) error {
	if tx.numbers == nil {
		n, err := tx.s.Numbers()
		if err != nil {
			return err
		}
		tx.numbers = n
	}
	changed := false
	for _, k := range append([]string{t.ID}, t.FormerIDs...) {
		if p, n, ok := model.ParseTaskID(k); ok && n > tx.numbers[p] {
			tx.numbers[p] = n
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return tx.put(model.NumbersPath, &pending{encode: tx.numbers.Encode})
}

// Update runs one operation: under the store lock it runs fn, stamps and
// writes the files fn put, and commits them as one commit. When fn fails
// nothing is written; when writing or committing fails the files are put
// back as they were. An operation that changes nothing makes no commit, and
// committed is false.
func (s *Store) Update(op Op, fn func(*Tx) error) (committed bool, err error) {
	h, err := s.UpdateCommit(op, fn)
	return !h.IsZero(), err
}

// UpdateCommit is Update that returns the hash of the commit it made; the
// zero hash says nothing was committed.
func (s *Store) UpdateCommit(op Op, fn func(*Tx) error) (hash plumbing.Hash, err error) {
	if op.Name == "" {
		return plumbing.ZeroHash, errors.New("the operation has no name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := acquire(s.path(lockPath), s.lockWait)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	// A release error is dropped when the operation succeeded: release
	// closes the file, which drops the OS lock on Unix and Windows anyway,
	// and the caller must not think a committed operation failed. When the
	// operation failed, the release error is joined to its error.
	defer func() {
		if rerr := l.release(); rerr != nil && err != nil {
			err = errors.Join(err, rerr)
		}
	}()

	// Another Store or a sync may have changed schema.yaml since it was read.
	cached, err := readDecode(s.dir, schemaPath, schema.Decode)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	s.schema.Store(cached)

	tx := &Tx{s: s, op: op, pending: map[string]*pending{}, tasks: map[model.ULID]*model.Task{}, workspaces: map[model.ULID]*model.Workspace{}}
	// assignIDs and rerank change the caller's tasks; a failed operation
	// leaves them as they were given.
	defer func() {
		if err != nil {
			tx.restoreIDs()
		}
	}()
	err = fn(tx)
	if err == nil {
		err = tx.assignIDs()
		if err == nil {
			err = tx.rerank()
		}
	}
	tx.done = true
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if tx.schema != nil {
		if ps := tx.schema.Check(); len(ps) > 0 {
			return plumbing.ZeroHash, &SchemaError{Problems: ps}
		}
	}
	sc := cached
	if tx.schema != nil {
		sc = tx.schema
	}
	if err := tx.validate(sc); err != nil {
		return plumbing.ZeroHash, err
	}

	now := s.now().Truncate(time.Second)
	changes, err := tx.changes(now)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if len(changes) == 0 && !tx.op.AllowEmpty {
		return plumbing.ZeroHash, nil
	}
	commit := tx.commitFor(tx.op, changes, now)
	if err := s.apply(changes); err != nil {
		return plumbing.ZeroHash, err
	}
	if err := s.commit(commit); err != nil {
		// The files written equal HEAD's, so there is nothing to undo.
		if !errors.Is(err, gitrepo.ErrNothingToCommit) {
			return plumbing.ZeroHash, errors.Join(err, s.rollback(changes))
		}
	} else {
		h, err := s.repo.Head()
		if err != nil {
			return plumbing.ZeroHash, err
		}
		hash = h
	}
	if tx.schema != nil {
		s.schema.Store(tx.schema)
	}
	return hash, nil
}

// changes stamps the queued writes and returns those that alter a file.
// updated is left as it was while comparing, so a write of the same content
// is dropped and keeps its old updated.
func (tx *Tx) changes(now time.Time) ([]change, error) {
	var out []change
	for _, rel := range tx.order {
		p := tx.pending[rel]
		old, err := atomicfile.ReadFile(tx.s.path(rel))
		existed := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if p.remove {
			if existed {
				out = append(out, change{rel: rel, old: old, existed: true})
			}
			continue
		}
		if p.created != nil && p.created.IsZero() {
			*p.created = now
		}
		data, err := p.encode()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if existed && bytes.Equal(data, old) {
			continue
		}
		if p.updated != nil {
			*p.updated = now
			if data, err = p.encode(); err != nil {
				return nil, fmt.Errorf("%s: %w", rel, err)
			}
		}
		out = append(out, change{rel: rel, data: data, old: old, existed: existed})
	}
	return out, nil
}

// commitFor builds the commit of op. The subject names the task by its id,
// or by its ULID when it has none, and gives its title as op leaves it; with
// no task it ends with op.Detail.
func (tx *Tx) commitFor(op Op, cs []change, now time.Time) gitrepo.Commit {
	subject := "hedgehug: " + op.Name
	trailers := []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: op.Name}}
	if op.Task.IsZero() && op.Detail != "" {
		subject += " " + op.Detail
	}
	if op.Account != "" {
		trailers = append(trailers, gitrepo.Trailer{Key: "Hedgehug-Author", Value: op.Account})
	}
	if !op.Workspace.IsZero() {
		trailers = append(trailers, gitrepo.Trailer{Key: "Hedgehug-Workspace", Value: op.Workspace.String()})
	}
	if !op.Task.IsZero() {
		label, title := op.Task.String(), ""
		t, err := tx.task(op.Task)
		if errors.Is(err, ErrNotFound) {
			// Removed in this operation (purge); the file is still on disk.
			t, err = tx.s.Task(op.Task)
		}
		if err == nil {
			if t.ID != "" {
				label = t.ID
			}
			title = t.Title
		}
		subject += " " + label
		if title != "" {
			subject += " " + title
		}
		trailers = append(trailers, gitrepo.Trailer{Key: "Hedgehug-Task", Value: op.Task.String()})
	}
	paths := make([]string, len(cs))
	for i, c := range cs {
		paths[i] = c.rel
	}
	return gitrepo.Commit{
		Paths: paths, Subject: subject, Trailers: append(trailers, op.Trailers...),
		Author: op.Author, When: now, AllowEmpty: op.AllowEmpty,
	}
}

// apply writes cs in order. When one fails, the ones before it are rolled
// back.
func (s *Store) apply(cs []change) error {
	for i, c := range cs {
		var err error
		if c.data == nil {
			err = os.Remove(s.path(c.rel))
		} else {
			err = atomicfile.WriteFile(s.path(c.rel), c.data)
		}
		if err != nil {
			return errors.Join(err, s.rollback(cs[:i]))
		}
	}
	return nil
}

// rollback puts the files of cs back as they were, last first.
func (s *Store) rollback(cs []change) error {
	var errs []error
	for _, c := range slices.Backward(cs) {
		if c.existed {
			errs = append(errs, atomicfile.WriteFile(s.path(c.rel), c.old))
		} else if err := os.Remove(s.path(c.rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
