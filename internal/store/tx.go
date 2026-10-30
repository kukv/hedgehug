package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

var errTxDone = errors.New("the transaction is used after Update returned")

// Op describes one operation, which becomes one commit.
type Op struct {
	Name   string   // add, update, rm, mv...; the Hedgehug-Op trailer
	Task   model.ID // the main task of the operation; zero if none
	Author gitrepo.Signature
}

// Tx collects the writes of one operation. Its Put methods only queue a
// write; Update writes them all after fn returns.
type Tx struct {
	s    *Store
	done bool

	order   []string // paths in the order first put
	pending map[string]*pending

	tasks     map[model.ID]*model.Task // put tasks; nil for a removed one
	taskOrder []model.ID
	schema    *schema.Schema // set by PutSchema
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
func (tx *Tx) Task(id model.ID) (*model.Task, error) {
	if tx.done {
		return nil, errTxDone
	}
	return tx.task(id)
}

func (tx *Tx) task(id model.ID) (*model.Task, error) {
	if t, ok := tx.tasks[id]; ok {
		if t == nil {
			return nil, fmt.Errorf("%s: %w", model.TaskPath(id), ErrNotFound)
		}
		return t, nil
	}
	return tx.s.Task(id)
}

// PutTask queues a write of t. Update fills in created and updated.
func (tx *Tx) PutTask(t *model.Task) error {
	if t.ID.IsZero() {
		return errors.New("the task has no id")
	}
	if err := tx.put(model.TaskPath(t.ID), &pending{encode: t.Encode, created: &t.Created, updated: &t.Updated}); err != nil {
		return err
	}
	// Not the tasks map: a task removed earlier in the operation is in it.
	if !slices.Contains(tx.taskOrder, t.ID) {
		tx.taskOrder = append(tx.taskOrder, t.ID)
	}
	tx.tasks[t.ID] = t
	return nil
}

func (tx *Tx) PutComment(task model.ID, c *model.Comment) error {
	if task.IsZero() || c.ID.IsZero() {
		return errors.New("the comment has no id or no task")
	}
	return tx.put(model.CommentPath(task, c.ID), &pending{encode: c.Encode, created: &c.Created, updated: &c.Updated})
}

func (tx *Tx) PutWorklog(task model.ID, w *model.Worklog) error {
	if task.IsZero() || w.ID.IsZero() {
		return errors.New("the worklog entry has no id or no task")
	}
	return tx.put(model.WorklogPath(task, w.ID), &pending{encode: w.Encode, created: &w.Created, updated: &w.Updated})
}

func (tx *Tx) PutSprint(sp *model.Sprint) error {
	if sp.ID.IsZero() {
		return errors.New("the sprint has no id")
	}
	return tx.put(model.SprintPath(sp.ID), &pending{encode: sp.Encode, created: &sp.Created, updated: &sp.Updated})
}

func (tx *Tx) PutRelease(r *model.Release) error {
	if r.ID.IsZero() {
		return errors.New("the release has no id")
	}
	return tx.put(model.ReleasePath(r.ID), &pending{encode: r.Encode, created: &r.Created, updated: &r.Updated})
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

// RemoveTask queues the removal of a task file with its comments and
// worklog entries, for purging. Soft deletion is a PutTask with DeletedAt.
func (tx *Tx) RemoveTask(id model.ID) error {
	if tx.done {
		return errTxDone
	}
	if id.IsZero() {
		return errors.New("the task has no id")
	}
	rels := []string{model.TaskPath(id)}
	for _, dir := range []string{path.Dir(model.CommentPath(id, model.ID{})), path.Dir(model.WorklogPath(id, model.ID{}))} {
		entries, err := os.ReadDir(tx.s.path(dir))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				rels = append(rels, dir+"/"+e.Name())
			}
		}
	}
	for _, rel := range rels {
		if err := tx.put(rel, &pending{remove: true}); err != nil {
			return err
		}
	}
	tx.tasks[id] = nil
	return nil
}

// Update runs one operation: under the store lock it runs fn, stamps and
// writes the files fn put, and commits them as one commit. When fn fails
// nothing is written; when writing or committing fails the files are put
// back as they were. An operation that changes nothing makes no commit.
func (s *Store) Update(op Op, fn func(*Tx) error) (err error) {
	if op.Name == "" {
		return errors.New("the operation has no name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := acquire(s.path(lockPath), s.lockWait)
	if err != nil {
		return err
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

	tx := &Tx{s: s, pending: map[string]*pending{}, tasks: map[model.ID]*model.Task{}}
	err = fn(tx)
	tx.done = true
	if err != nil {
		return err
	}
	if tx.schema != nil {
		if ps := tx.schema.Check(); len(ps) > 0 {
			return &SchemaError{Problems: ps}
		}
	}
	sc := s.Schema()
	if tx.schema != nil {
		sc = tx.schema
	}
	if err := tx.validate(sc); err != nil {
		return err
	}

	now := s.now().Truncate(time.Second)
	changes, err := tx.changes(now)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	commit := tx.commitFor(op, changes, now)
	if err := s.apply(changes); err != nil {
		return err
	}
	if err := s.commit(commit); err != nil {
		return errors.Join(err, s.rollback(changes))
	}
	if tx.schema != nil {
		s.schema.Store(tx.schema)
	}
	return nil
}

// changes stamps the queued writes and returns those that alter a file.
// updated is left as it was while comparing, so a write of the same content
// is dropped and keeps its old updated.
func (tx *Tx) changes(now time.Time) ([]change, error) {
	var out []change
	for _, rel := range tx.order {
		p := tx.pending[rel]
		old, err := readFile(tx.s.path(rel))
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

// commitFor builds the commit of op. The subject names the task by its key,
// or by its ID when it has none, and gives its title as op leaves it.
func (tx *Tx) commitFor(op Op, cs []change, now time.Time) gitrepo.Commit {
	subject := "hedgehug: " + op.Name
	trailers := []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: op.Name}}
	if !op.Task.IsZero() {
		label, title := op.Task.String(), ""
		if t, err := tx.task(op.Task); err == nil {
			if t.Key != "" {
				label = t.Key
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
	return gitrepo.Commit{Paths: paths, Subject: subject, Trailers: trailers, Author: op.Author, When: now}
}

// apply writes cs in order. When one fails, the ones before it are rolled
// back.
func (s *Store) apply(cs []change) error {
	for i, c := range cs {
		var err error
		if c.data == nil {
			err = os.Remove(s.path(c.rel))
		} else {
			err = writeFile(s.path(c.rel), c.data)
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
			errs = append(errs, writeFile(s.path(c.rel), c.old))
		} else if err := os.Remove(s.path(c.rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
