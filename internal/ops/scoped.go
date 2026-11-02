package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
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
func (ss *Session) scopedProject(ix *index.Index, arg string) (model.ULID, *index.Entry, error) {
	e, err := ss.Resolve(ix, arg)
	if err == nil {
		return e.ULID, e, nil
	}
	if KindOf(err) == KindNotFound {
		if u, perr := model.ParseULID(arg); perr == nil {
			if _, ok := ix.Schema().ScopedFor(u); ok {
				return u, nil, nil
			}
		}
	}
	return model.ULID{}, nil, err
}

// ScopedGet reads the override of the project arg names.
func (ss *Session) ScopedGet(arg string) (ScopedResult, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return ScopedResult{}, err
	}
	ulid, e, err := ss.scopedProject(ix, arg)
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
// container and not deleted (KindInvalid, scoped.not_container; KindConflict,
// task.deleted). A change that brings a problem schema.Check does not find in
// schema.yaml now is KindInvalid, scoped.invalid, with the problems as Args
// "problems"; nothing is written.
func (ss *Session) ScopedSet(arg string, ch schema.ScopedChange) (ScopedResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ScopedResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return ScopedResult{}, err
	}
	if err := NotDeleted(ix, e); err != nil {
		return ScopedResult{}, err
	}
	id := ix.DisplayID(e)
	if k, ok := ix.Schema().Kind(e.Kind); !ok || k.Role != schema.RoleContainer {
		return ScopedResult{}, newErr(KindInvalid, "scoped.not_container",
			fmt.Errorf("%s is a %s, which is not a container", id, e.Kind), "id", id, "kind", e.Kind)
	}
	return ss.scopedWrite(s, ix, e.ULID, e, func(sc *schema.Schema) *schema.Schema { return sc.WithScoped(e.ULID, ch) })
}

// ScopedRemove removes the override of the project arg names, templates
// included, in one commit (op schema). A project with no override is not an
// error; nothing is committed. The project may be deleted, and may be gone
// when arg is its ULID.
func (ss *Session) ScopedRemove(arg string) (ScopedResult, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return ScopedResult{}, err
	}
	ulid, e, err := ss.scopedProject(ix, arg)
	if err != nil {
		return ScopedResult{}, err
	}
	return ss.scopedWrite(s, ix, ulid, e, func(sc *schema.Schema) *schema.Schema { return sc.WithoutScoped(ulid) })
}

// scopedWrite writes the schema that next makes from the one the store has
// under its lock.
func (ss *Session) scopedWrite(s *store.Store, ix *index.Index, ulid model.ULID, e *index.Entry, next func(*schema.Schema) *schema.Schema) (ScopedResult, error) {
	var after *schema.Schema
	hash, err := ss.UpdateCommit(s, store.Op{Name: "schema", Task: ulid}, func(tx *store.Tx) error {
		cur := s.Schema()
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
	r := ScopedResult{Index: ix, Project: ulid, Entry: e, Committed: !hash.IsZero(), Commit: commitHex(hash)}
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
	return newErr(KindInvalid, "scoped.invalid", fmt.Errorf("the override breaks the schema: %s", strings.Join(msgs, "; ")), "problems", msgs)
}
