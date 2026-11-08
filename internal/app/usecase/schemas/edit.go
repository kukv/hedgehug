package schemas

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

type EditWrite struct {
	Schema SchemaView `json:"schema"`
	Commit *string    `json:"commit"`
}

type Use struct {
	Target schema.Target
	Prop   string
	Name   string
}

// EditOpts says what Edit checks besides the schema.
type EditOpts struct {
	// Use are the definitions the change removes; one that a task uses
	// refuses the change.
	Use []Use
	// UseAfter, when set, is asked after fn has run for the removals fn found,
	// such as the values of an enum that no edit keeps.
	UseAfter func() []Use
	// Check refuses a schema under which a task breaks a rule it did not
	// break under the old one.
	Check bool
}

// Edit makes the change fn describes to the schema, and the tasks it
// carries, in one commit. fn returns the new schema and the migrations that
// rewrite the tasks for a name that changed; the tasks are rewritten as in
// migrate, those that do not decode left alone.
func Edit(ss *usecase.Session, fn func(*schema.Schema) (*schema.Schema, []schema.Migration, error), opts EditOpts) (EditWrite, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return EditWrite{}, err
	}
	var after *schema.Schema
	// A new name is a task written with a status that the workflow of the
	// old schema does not lead to.
	hash, err := ss.UpdateCommit(s, store.Op{Name: "schema", SkipTransitions: true}, func(tx *store.Tx) error {
		cur := s.Schema()
		next, ms, err := fn(cur)
		if err != nil {
			return editErr(err)
		}
		if ps := next.AddedProblems(cur.Check()); len(ps) > 0 {
			return schemaInvalid(ps)
		}
		uses := opts.Use
		if opts.UseAfter != nil {
			uses = append(slices.Clone(uses), opts.UseAfter()...)
		}
		for _, u := range uses {
			if err := notInUse(s, u); err != nil {
				return err
			}
		}
		if opts.Check {
			if err := noNewViolations(s, ix, cur, next, ms); err != nil {
				return err
			}
		}
		if err := rewriteTasks(s, ix, tx, cur, ms); err != nil {
			return err
		}
		after = next
		return tx.PutSchema(next)
	})
	var se *store.SchemaError
	if errors.As(err, &se) {
		err = schemaInvalid(se.Problems)
	}
	if err != nil {
		return EditWrite{}, err
	}
	return EditWrite{Schema: SchemaViewOf(after), Commit: usecase.CommitPtr(usecase.CommitHex(hash))}, nil
}

// rewriteTasks puts every task that the migrations, which are given in the
// order they are applied and read cur, the schema before them, change.
func rewriteTasks(s *store.Store, ix *index.Index, tx *store.Tx, cur *schema.Schema, ms []schema.Migration) error {
	if len(ms) == 0 {
		return nil
	}
	var refused []string
	for t, err := range s.Tasks() {
		if err != nil {
			if _, ok := brokenTask(err); ok {
				continue
			}
			return err
		}
		changed := false
		var terr error
		for _, m := range ms {
			ok, err := m.ApplyTask(t, cur)
			if err != nil {
				terr = err
				break
			}
			changed = changed || ok
		}
		switch {
		case terr != nil:
			refused = append(refused, usecase.RefName(ix, t.ULID)+": "+terr.Error())
		case changed:
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
	}
	if len(refused) > 0 {
		slices.Sort(refused)
		return usecase.NewErr(usecase.KindInvalid, "migrate.invalid", fmt.Errorf("cannot rewrite %s:\n  %s", TaskCount(len(refused)), strings.Join(refused, "\n  ")))
	}
	return nil
}

// editErr gives the errors of the schema package their codes.
func editErr(err error) error {
	var pe *schema.ParentsEmptyError
	switch {
	case errors.As(err, &pe):
		return usecase.NewErr(usecase.KindInvalid, "schema.parents_empty", err, "kinds", pe.Kinds)
	case errors.Is(err, schema.ErrNotDefined):
		return usecase.NewErr(usecase.KindNotFound, "schema.not_defined", err)
	case errors.Is(err, schema.ErrOrderStale):
		return usecase.NewErr(usecase.KindConflict, "schema.order_stale", err)
	case errors.Is(err, schema.ErrDefaultContainer):
		return usecase.NewErr(usecase.KindInvalid, "schema.default_container", err)
	case errors.Is(err, schema.ErrNotTaskRef):
		return usecase.NewErr(usecase.KindInvalid, "schema.not_task_ref", err)
	case errors.Is(err, schema.ErrNotEnum):
		return usecase.NewErr(usecase.KindInvalid, "schema.not_enum", err)
	case errors.Is(err, schema.ErrNameTaken):
		return usecase.NewErr(usecase.KindConflict, "schema.name_taken", err)
	case errors.Is(err, schema.ErrValueInvalid):
		return usecase.NewErr(usecase.KindInvalid, "schema.value_invalid", err)
	}
	return err
}

func schemaInvalid(ps []schema.Problem) error {
	msgs := make([]string, len(ps))
	for i, p := range ps {
		msgs[i] = p.String()
	}
	return usecase.NewErr(usecase.KindInvalid, "schema.invalid", fmt.Errorf("the change breaks the schema: %s", strings.Join(msgs, "; ")), "problems", msgs)
}

// notInUse refuses a removal of a definition that a task uses, deleted
// tasks counted: they can be restored.
func notInUse(s *store.Store, u Use) error {
	count, deleted := 0, 0
	for t, err := range s.Tasks() {
		if err != nil {
			if _, ok := brokenTask(err); ok {
				continue
			}
			return err
		}
		if schema.TaskUses(t, u.Target, u.Prop, u.Name) {
			count++
			if t.DeletedAt != nil {
				deleted++
			}
		}
	}
	if count == 0 {
		return nil
	}
	args := []any{"count", count, "deleted", deleted}
	if u.Target == schema.TargetValue {
		args = append(args, "value", u.Name)
	}
	return usecase.NewErr(usecase.KindInvalid, "schema.in_use",
		fmt.Errorf("%s uses %s; merge it into another first", TaskCount(count), u.Name), args...)
}

// migrated is t as the migrations ms, read against cur, write it; ok is false
// when one does not apply (rewriteTasks refuses that).
func migrated(t *model.Task, cur *schema.Schema, ms []schema.Migration) (out *model.Task, ok bool) {
	c := *t
	c.Props = slices.Clone(t.Props)
	for _, m := range ms {
		if _, err := m.ApplyTask(&c, cur); err != nil {
			return nil, false
		}
	}
	return &c, true
}

// noNewViolations refuses a schema under which a task breaks a rule it did
// not break under the old one. The task is looked at as it is written under
// the new schema: the migrations ms, which rename what the tasks carry, are
// applied to it and to the parents it is read against.
func noNewViolations(s *store.Store, ix *index.Index, cur, next *schema.Schema, ms []schema.Migration) error {
	entry := func(u model.ULID) (*model.Task, error) {
		if e, ok := ix.Task(u); ok && e.Broken == "" {
			return &model.Task{ULID: e.ULID, Kind: e.Kind, Parent: e.Parent}, nil
		}
		return nil, store.ErrNotFound
	}
	readNew := func(u model.ULID) (*model.Task, error) {
		p, err := entry(u)
		if err != nil {
			return nil, err
		}
		if m, ok := migrated(p, cur, ms); ok {
			return m, nil
		}
		return p, nil
	}
	var lines []string
	for t, err := range s.Tasks() {
		if err != nil {
			if _, ok := brokenTask(err); ok {
				continue
			}
			return err
		}
		oldKind, oldAnc, err := store.Lineage(t, cur, entry)
		if err != nil {
			return err
		}
		nt, ok := migrated(t, cur, ms)
		if !ok {
			continue
		}
		newKind, newAnc, err := store.Lineage(nt, next, readNew)
		if err != nil {
			return err
		}
		for _, v := range schema.ValidateWrite(t, oldKind, cur.For(oldAnc), nt, newKind, next.For(newAnc), true) {
			lines = append(lines, usecase.RefName(ix, t.ULID)+": "+v.String())
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return usecase.NewErr(usecase.KindInvalid, "schema.tasks_invalid",
		fmt.Errorf("the change breaks %s:\n  %s", TaskCount(len(lines)), strings.Join(lines, "\n  ")), "violations", lines)
}
