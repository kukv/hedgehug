package tasks

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

var errChangedWhileEditing = usecase.NewErr(usecase.KindConflict, "edit.changed", errors.New("the task changed while editing"))

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

	ss *usecase.Session // the session that began it, which writes
}

// BeginEdit resolves arg to a live task and reads its file.
func BeginEdit(ss *usecase.Session, arg string) (*EditSession, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return nil, err
	}
	if ts != ss {
		return BeginEdit(ts, arg)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return nil, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
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

// FileName names the temporary file that edits the task: its id, or its ULID
// when it has no id.
func (es *EditSession) FileName() string { return usecase.TaskFileName(es.Entry) }

// Check decodes the edited file of the task and refuses the changes edit does
// not make, as CheckEdit does.
func (es *EditSession) Check(edited []byte) (*model.Task, error) {
	return CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
}

// Unchanged is the result of an edit that changed nothing: no commit.
func (es *EditSession) Unchanged() usecase.TaskResult {
	return usecase.NewTaskResult(es.Index, es.Entry.ULID, plumbing.ZeroHash)
}

// session is the session that began es, or ss for one made by hand.
func (es *EditSession) session(ss *usecase.Session) *usecase.Session {
	if es.ss != nil {
		return es.ss
	}
	return ss
}

// editInvalid is the error for an edit CheckEdit refuses; field names the
// key that was changed, "" when there is none.
func editInvalid(err error, field string) error {
	if field == "" {
		return usecase.NewErr(usecase.KindInvalid, "edit.invalid", err)
	}
	return usecase.NewErr(usecase.KindInvalid, "edit.invalid", err, "field", field)
}

// CheckEdit decodes the edited file of task ulid and refuses the changes
// edit does not make. old is the task before the edit, nil when it was
// broken; then only the ulid is checked. The workspace can be changed only
// when the old one is missing or not a workspace of ix, to repair it.
func CheckEdit(ix *index.Index, ulid model.ULID, old *model.Task, edited []byte) (*model.Task, error) {
	t, err := model.DecodeTask(edited)
	if errors.Is(err, model.ErrForward) {
		return nil, editInvalid(errors.New("moved_to cannot be set by hand; use hedgehug mv --store"), "moved_to")
	}
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
	case !model.NodeEqual(t.GH, old.GH):
		return nil, fixed("gh")
	case t.Workspace != old.Workspace && hasWorkspace(ix, old.Workspace):
		return nil, editInvalid(fmt.Errorf("workspace cannot be changed here; use hedgehug mv --workspace"), "workspace")
	}
	return t, nil
}

// PrepareEdit adds the watchers the edit calls for to t, the checked edit of
// es. Its error comes before anything is written.
func PrepareEdit(ss *usecase.Session, es *EditSession, t *model.Task) error {
	ss = es.session(ss)
	if err := ss.CheckEditRefs(es.Index, es.Old, t); err != nil {
		return err
	}
	if es.Old == nil {
		return nil
	}
	host, err := ss.WriterHost()
	if err != nil {
		return err
	}
	usecase.AddWatchers(t, usecase.NewMentions(es.Old.Body, t.Body, host)...)
	return nil
}

// WriteEdit writes t, prepared by PrepareEdit, to the store. It refuses when
// the file changed since BeginEdit.
func WriteEdit(ss *usecase.Session, es *EditSession, t *model.Task) (usecase.TaskResult, error) {
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
		return usecase.TaskResult{}, err
	}
	if !hash.IsZero() && es.Old != nil {
		ss.WarnMentions(s, es.Old.Body, t.Body)
	}
	return usecase.NewTaskResult(es.Index, e.ULID, hash), nil
}

// Edit replaces the file of the task arg names with edited, checked as
// CheckEdit does. It is the one call for the server; the CLI opens $EDITOR
// between BeginEdit and CheckEdit.
func Edit(ss *usecase.Session, arg string, edited []byte) (usecase.TaskResult, error) {
	es, err := BeginEdit(ss, arg)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	t, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
	if err != nil {
		return usecase.TaskResult{}, err
	}
	if err := PrepareEdit(ss, es, t); err != nil {
		return usecase.TaskResult{}, err
	}
	return WriteEdit(ss, es, t)
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
