package ops

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var errChangedWhileEditing = newErr(KindConflict, "edit.changed", errors.New("the task changed while editing"))

// EditSession is what edit needs between reading the file and writing the
// edited bytes back: the CLI opens $EDITOR in between, the server takes
// the new body from the request.
type EditSession struct {
	Store *store.Store
	Index *index.Index
	Entry *index.Entry
	// Old is the task as the file holds it; nil for a broken task: its file
	// does not decode, or holds another ULID.
	Old *model.Task
	// Bytes is the file as it was read.
	Bytes []byte

	ss *Session // the session that began it, which writes
}

// BeginEdit resolves arg to a live task and reads its file.
func (ss *Session) BeginEdit(arg string) (*EditSession, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return nil, err
	}
	if ts != ss {
		return ts.BeginEdit(arg)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return nil, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return nil, err
	}
	orig, err := s.TaskBytes(e.ULID)
	if err != nil {
		return nil, err
	}
	old, err := model.DecodeTask(orig)
	if err != nil || old.ULID != e.ULID {
		old = nil
	}
	return &EditSession{Store: s, Index: ix, Entry: e, Old: old, Bytes: orig, ss: ss}, nil
}

// session is the session that began es, or ss for one made by hand.
func (es *EditSession) session(ss *Session) *Session {
	if es.ss != nil {
		return es.ss
	}
	return ss
}

// editInvalid is the error for an edit CheckEdit refuses; field names the
// key that was changed, "" when there is none.
func editInvalid(err error, field string) error {
	if field == "" {
		return newErr(KindInvalid, "edit.invalid", err)
	}
	return newErr(KindInvalid, "edit.invalid", err, "field", field)
}

// CheckEdit decodes the edited file of task ulid and refuses the changes
// edit does not make. old is the task before the edit, nil when it was
// broken; then only the ulid is checked. The workspace can be changed only
// when the old one is missing or not a workspace of ix, to repair it.
func CheckEdit(ix *index.Index, ulid model.ULID, old *model.Task, edited []byte) (*model.Task, error) {
	t, err := model.DecodeTask(edited)
	if err != nil {
		return nil, editInvalid(fmt.Errorf("the file does not decode: %w", err), "")
	}
	if old == nil {
		if t.ULID != ulid {
			return nil, editInvalid(fmt.Errorf("the ulid must stay %s", ulid), "ulid")
		}
		return t, nil
	}
	fixed := func(key string) error { return editInvalid(fmt.Errorf("%s cannot be changed", key), key) }
	moved := func(key string) error {
		return editInvalid(fmt.Errorf("%s cannot be changed here; use hedgehug mv", key), key)
	}
	switch {
	case t.ULID != old.ULID:
		return nil, fixed("ulid")
	case t.ID != old.ID:
		return nil, fixed("id")
	case !t.Created.Equal(old.Created):
		return nil, fixed("created")
	case !t.Updated.Equal(old.Updated):
		return nil, fixed("updated")
	case !sameTime(t.DeletedAt, old.DeletedAt):
		return nil, fixed("deleted_at")
	case !sameULID(t.Parent, old.Parent):
		return nil, moved("parent")
	case t.Rank != old.Rank:
		return nil, moved("rank")
	case !slices.Equal(t.FormerIDs, old.FormerIDs):
		return nil, fixed("former_ids")
	case t.Workspace != old.Workspace && hasWorkspace(ix, old.Workspace):
		return nil, editInvalid(fmt.Errorf("workspace cannot be changed here; use hedgehug mv --workspace"), "workspace")
	}
	return t, nil
}

// PrepareEdit adds the watchers the edit calls for to t, the checked edit of
// es. Its error comes before anything is written.
func (ss *Session) PrepareEdit(es *EditSession, t *model.Task) error {
	ss = es.session(ss)
	if err := ss.checkEditRefs(es.Index, es.Old, t); err != nil {
		return err
	}
	if es.Old == nil {
		return nil
	}
	host, err := ss.WriterHost()
	if err != nil {
		return err
	}
	AddWatchers(t, NewMentions(es.Old.Body, t.Body, host)...)
	return nil
}

// WriteEdit writes t, prepared by PrepareEdit, to the store. It refuses when
// the file changed since BeginEdit.
func (ss *Session) WriteEdit(es *EditSession, t *model.Task) (TaskResult, error) {
	ss = es.session(ss)
	s, e := es.Store, es.Entry
	hash, err := ss.UpdateCommit(s, store.Op{Name: "edit", Task: e.ULID}, func(tx *store.Tx) error {
		cur, err := s.TaskBytes(e.ULID)
		if err != nil {
			return err
		}
		if !bytes.Equal(cur, es.Bytes) {
			return errChangedWhileEditing
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: es.Index, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// Edit replaces the file of the task arg names with edited, checked as
// CheckEdit does. It is the one call for the server; the CLI opens $EDITOR
// between BeginEdit and CheckEdit.
func (ss *Session) Edit(arg string, edited []byte) (TaskResult, error) {
	es, err := ss.BeginEdit(arg)
	if err != nil {
		return TaskResult{}, err
	}
	t, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
	if err != nil {
		return TaskResult{}, err
	}
	if err := ss.PrepareEdit(es, t); err != nil {
		return TaskResult{}, err
	}
	return ss.WriteEdit(es, t)
}

// hasWorkspace reports whether ulid is a workspace of ix.
func hasWorkspace(ix *index.Index, ulid model.ULID) bool {
	_, ok := ix.Workspace(ulid)
	return ok
}

func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func sameULID(a, b *model.ULID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
