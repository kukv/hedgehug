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

type EditWrite struct {
	Schema SchemaView `json:"schema"`
	Commit *string    `json:"commit"`
}

type Use struct {
	Target schema.Target
	Prop   string
	Name   string
}

func Edit(ss *usecase.Session, fn func(*schema.Schema) (*schema.Schema, error), use *Use, check bool) (EditWrite, error) {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return EditWrite{}, err
	}
	var after *schema.Schema
	hash, err := ss.UpdateCommit(s, store.Op{Name: "schema"}, func(tx *store.Tx) error {
		cur := s.Schema()
		next, err := fn(cur)
		if err != nil {
			return editErr(err)
		}
		if ps := next.AddedProblems(cur.Check()); len(ps) > 0 {
			return schemaInvalid(ps)
		}
		if use != nil {
			if err := notInUse(s, *use); err != nil {
				return err
			}
		}
		if check {
			if err := noNewViolations(s, ix, cur, next); err != nil {
				return err
			}
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
	return usecase.NewErr(usecase.KindInvalid, "schema.in_use",
		fmt.Errorf("%s uses %s; merge it into another first", TaskCount(count), u.Name), "count", count, "deleted", deleted)
}

// noNewViolations refuses a schema under which a task breaks a rule it did
// not break under the old one.
func noNewViolations(s *store.Store, ix *index.Index, cur, next *schema.Schema) error {
	read := func(u model.ULID) (*model.Task, error) {
		if e, ok := ix.Task(u); ok && e.Broken == "" {
			return &model.Task{ULID: e.ULID, Kind: e.Kind, Parent: e.Parent}, nil
		}
		return nil, store.ErrNotFound
	}
	var lines []string
	for t, err := range s.Tasks() {
		if err != nil {
			if _, ok := brokenTask(err); ok {
				continue
			}
			return err
		}
		oldKind, oldAnc, err := store.Lineage(t, cur, read)
		if err != nil {
			return err
		}
		newKind, newAnc, err := store.Lineage(t, next, read)
		if err != nil {
			return err
		}
		for _, v := range schema.ValidateWrite(t, oldKind, cur.For(oldAnc), t, newKind, next.For(newAnc), true) {
			lines = append(lines, usecase.RefName(ix, t.ULID)+": "+v.String())
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return usecase.NewErr(usecase.KindInvalid, "schema.tasks_invalid",
		fmt.Errorf("the change breaks %s:\n  %s", TaskCount(len(lines)), strings.Join(lines, "\n  ")), "violations", lines)
}
