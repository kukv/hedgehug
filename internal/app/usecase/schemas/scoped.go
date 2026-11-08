package schemas

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// ScopedProject names the project of an override. ID and Title are empty
// for an override whose project is no longer in the store.
type ScopedProject struct {
	ULID  string `json:"ulid"`
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
}

// ScopedView is the override of a project as hh schema scope prints it with
// --json and PUT and DELETE /api/schema/scoped/{project} return it. Scoped is
// left out when the project has no override; Commit is null when nothing was
// written.
type ScopedView struct {
	Project ScopedProject `json:"project"`
	Scoped  *ScopedDef    `json:"scoped,omitempty"`
	Commit  *string       `json:"commit"`
}

// ScopedResult is what ScopedGet, ScopedSet and ScopedRemove did. Scoped is
// the override as it is after the call, nil when there is none. Entry is nil
// when the project is no longer in the store. Index is the one the call read:
// read the names of tasks from it, not the schema.
type ScopedResult struct {
	Index     *index.Index
	Project   model.ULID
	Entry     *index.Entry
	Scoped    *schema.Scoped
	Committed bool
	Commit    string // hex hash of the commit made; "" when nothing was committed
}

// Name is how the text names the project: its id and title, or its ULID when
// the project is no longer in the store.
func (r ScopedResult) Name() string {
	if r.Entry == nil {
		return r.Project.String()
	}
	return r.Index.DisplayID(r.Entry) + "  " + r.Entry.Title
}

// View is r as ScopedView.
func (r ScopedResult) View() ScopedView {
	v := ScopedView{Project: ScopedProject{ULID: r.Project.String()}}
	if r.Entry != nil {
		v.Project.ID, v.Project.Title = r.Index.DisplayID(r.Entry), r.Entry.Title
	}
	if r.Scoped != nil {
		d := ScopedDefOf(*r.Scoped)
		v.Scoped = &d
	}
	if r.Commit != "" {
		c := r.Commit
		v.Commit = &c
	}
	return v
}

// scopedProject resolves the project arg names. An override whose project is
// gone is still named by its ULID, so that it can be shown and removed.
func scopedProject(ss *usecase.Session, ix *index.Index, arg string) (model.ULID, *index.Entry, error) {
	e, err := ss.Resolve(ix, arg)
	if err == nil {
		return e.ULID, e, nil
	}
	if usecase.KindOf(err) == usecase.KindNotFound {
		if u, perr := model.ParseULID(arg); perr == nil {
			if _, ok := ix.Schema().ScopedFor(u); ok {
				return u, nil, nil
			}
		}
	}
	return model.ULID{}, nil, err
}

// ScopedGet reads the override of the project arg names.
func ScopedGet(ss *usecase.Session, arg string) (ScopedResult, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return ScopedResult{}, err
	}
	ulid, e, err := scopedProject(ss, ix, arg)
	if err != nil {
		return ScopedResult{}, err
	}
	r := ScopedResult{Index: ix, Project: ulid, Entry: e}
	if sc, ok := ix.Schema().ScopedFor(ulid); ok {
		r.Scoped = &sc
	}
	return r, nil
}

// ScopedSet changes the override of the project arg names, in one commit
// (op schema): the parts ch gives are replaced, a part ch gives empty goes
// back to the store's value, and the rest is kept, with the templates and the
// keys schema.yaml has that hedgehug does not know. An override left with
// nothing is removed. The project must be a task of a kind whose role is
// container and not deleted (KindInvalid, scoped.not_group; KindConflict,
// task.deleted). A change that brings a problem schema.Check does not find in
// schema.yaml now is KindInvalid, scoped.invalid, with the problems as Args
// "problems"; nothing is written.
func ScopedSet(ss *usecase.Session, arg string, ch schema.ScopedChange) (ScopedResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ScopedResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return ScopedResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return ScopedResult{}, err
	}
	id := ix.DisplayID(e)
	if err := scopedGroup(id, e.Kind); err != nil {
		return ScopedResult{}, err
	}
	if err := templatesExist(s, ch); err != nil {
		return ScopedResult{}, err
	}
	// the same check again under the lock, on the task and the schema the write sees
	guard := scopedGuard(id, e.ULID)
	return scopedWrite(ss, s, ix, e.ULID, e, guard, func(sc *schema.Schema) *schema.Schema { return sc.WithScoped(e.ULID, ch) })
}

// templatesExist refuses a template of ch that the store does not have.
func templatesExist(s *store.Store, ch schema.ScopedChange) error {
	var names []string
	if ch.TemplateOnly != nil {
		names = append(names, *ch.TemplateOnly...)
	}
	if ch.TemplateDefault != nil {
		for _, n := range *ch.TemplateDefault {
			names = append(names, n)
		}
	}
	for _, n := range names {
		if _, err := s.Template(n); errors.Is(err, store.ErrNotFound) {
			return usecase.NewErr(usecase.KindNotFound, "template.not_found", fmt.Errorf("template %s is not in this store", n), "name", n)
		}
	}
	return nil
}

// scopedGuard checks the project ulid, which id names, under the lock of the
// write: it is there, not deleted, and a container in the schema cur.
func scopedGuard(id string, ulid model.ULID) func(*store.Tx, *schema.Schema) error {
	return func(tx *store.Tx, cur *schema.Schema) error {
		t, err := tx.Task(ulid)
		if errors.Is(err, store.ErrNotFound) {
			return usecase.NewErr(usecase.KindNotFound, "task.not_found", fmt.Errorf("%s: %w", id, err), "input", id)
		}
		if err != nil {
			return err
		}
		if t.DeletedAt != nil {
			return usecase.NewErr(usecase.KindConflict, "task.deleted", fmt.Errorf("%s is deleted; restore %s first", id, id), "id", id)
		}
		return scopedGroup(id, t.Kind)
	}
}

// ScopedRemove removes the override of the project arg names, templates
// included, in one commit (op schema). A project with no override is not an
// error; nothing is committed. The project may be deleted, and may be gone
// when arg is its ULID.
func ScopedRemove(ss *usecase.Session, arg string) (ScopedResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ScopedResult{}, err
	}
	ulid, e, err := scopedProject(ss, ix, arg)
	if err != nil {
		return ScopedResult{}, err
	}
	return scopedWrite(ss, s, ix, ulid, e, nil, func(sc *schema.Schema) *schema.Schema { return sc.WithoutScoped(ulid) })
}

// scopedWrite writes the schema that next makes from the one the store has
// under its lock. guard, when given, checks the project under the same lock.
func scopedWrite(ss *usecase.Session, s *store.Store, ix *index.Index, ulid model.ULID, e *index.Entry, guard func(*store.Tx, *schema.Schema) error, next func(*schema.Schema) *schema.Schema) (ScopedResult, error) {
	var after *schema.Schema
	hash, err := ss.UpdateCommit(s, store.Op{Name: "schema", Task: ulid}, func(tx *store.Tx) error {
		cur := s.Schema()
		if guard != nil {
			if err := guard(tx, cur); err != nil {
				return err
			}
		}
		after = next(cur)
		if ps := after.NewProblems(cur.Check()); len(ps) > 0 {
			return scopedInvalid(ps)
		}
		return tx.PutSchema(after)
	})
	var se *store.SchemaError
	if errors.As(err, &se) {
		err = scopedInvalid(se.Problems)
	}
	if err != nil {
		return ScopedResult{}, err
	}
	r := ScopedResult{Index: ix, Project: ulid, Entry: e, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}
	if sc, ok := after.ScopedFor(ulid); ok {
		r.Scoped = &sc
	}
	return r, nil
}

func scopedInvalid(ps []schema.Problem) error {
	msgs := make([]string, len(ps))
	for i, p := range ps {
		msgs[i] = p.String()
	}
	return usecase.NewErr(usecase.KindInvalid, "scoped.invalid", fmt.Errorf("the override breaks the schema: %s", strings.Join(msgs, "; ")), "problems", msgs)
}

// scopedGroup refuses a kind that is not a group (a project or an epic).
func scopedGroup(id, kind string) error {
	if !schema.IsGroup(kind) {
		return usecase.NewErr(usecase.KindInvalid, "scoped.not_group",
			fmt.Errorf("%s is a %s; an override can be put on a project or an epic only", id, kind), "id", id, "kind", kind)
	}
	return nil
}
