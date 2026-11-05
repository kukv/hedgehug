package tasks

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

// BulkInput is what a bulk update changes. DryRun checks and writes nothing.
type BulkInput struct {
	Targets []usecase.TaskTarget
	Assigns []usecase.Assign
	DryRun  bool
}

// BulkTask is a task a bulk update is on.
type BulkTask struct {
	ULID  string `json:"ulid"`
	ID    string `json:"id"`
	Store string `json:"store"`
	Title string `json:"title"`
}

// BulkViolation is what a task breaks.
type BulkViolation struct {
	Task    BulkTask `json:"task"`
	Field   string   `json:"field"`
	Message string   `json:"message"`
}

// BulkCommit is the commit of one store.
type BulkCommit struct {
	Store  string `json:"store"`
	Commit string `json:"commit"`
}

// BulkResult is what a bulk update answers. Violations is set only by a dry
// run: a write that breaks anything is refused with bulk.invalid.
type BulkResult struct {
	Tasks      []BulkTask      `json:"tasks"`
	Violations []BulkViolation `json:"violations,omitempty"`
	Commits    []BulkCommit    `json:"commits,omitempty"`
}

// bulkStore is the part of a bulk update in one store.
type bulkStore struct {
	ss    *usecase.Session
	s     *store.Store
	ix    *index.Index
	name  string
	ulids []model.ULID
}

func (b *bulkStore) op(in BulkInput, check bool) store.Op {
	parts := make([]string, len(in.Assigns))
	for i, a := range in.Assigns {
		parts[i] = a.Key + a.Op + a.Value
	}
	return store.Op{Name: usecase.OpBulkSet, Detail: plural.Count(len(b.ulids), "task") + " " + strings.Join(parts, " "), CheckOnly: check}
}

func (b *bulkStore) write(in BulkInput) func(*store.Tx) error {
	return func(tx *store.Tx) error {
		for _, u := range b.ulids {
			t, err := tx.Task(u)
			if err != nil {
				return err
			}
			assignees := slices.Clone(t.Assignees)
			for _, as := range in.Assigns {
				if err := usecase.ApplyTaskAssign(t, as, b.ix, b.ss.ResolveULID(b.ix)); err != nil {
					return err
				}
			}
			usecase.WatchNewAssignees(t, assignees)
			if err := tx.PutTask(t); err != nil {
				return err
			}
			tx.AddTrailer("Hedgehug-Task", u.String())
		}
		return nil
	}
}

func (b *bulkStore) row(u model.ULID) BulkTask {
	r := BulkTask{ULID: u.String(), Store: b.name}
	if e, ok := b.ix.Task(u); ok {
		r.ID, r.Title = b.ix.DisplayID(e), e.Title
	}
	return r
}

// openBulk opens the stores of the targets, in the order they come, and
// refuses a read-only store and a deleted task before anything is written.
func openBulk(ss *usecase.Session, targets []usecase.TaskTarget) ([]*bulkStore, error) {
	var out []*bulkStore
	for _, tg := range targets {
		i := slices.IndexFunc(out, func(b *bulkStore) bool { return b.name == tg.Store })
		if i < 0 {
			ts := ss
			if ss.StoreName != tg.Store {
				ts = ss.ForStore(tg.Store)
			}
			s, ix, err := ts.OpenForWrite()
			if err != nil {
				return nil, err
			}
			out = append(out, &bulkStore{ss: ts, s: s, ix: ix, name: tg.Store})
			i = len(out) - 1
		}
		b := out[i]
		e, ok := b.ix.Task(tg.ULID)
		if !ok {
			return nil, usecase.NewErr(usecase.KindNotFound, "task.not_found", fmt.Errorf("task %s is not in %s", tg.ULID, tg.Store), "id", tg.ULID.String())
		}
		if err := usecase.NotDeleted(b.ix, e); err != nil {
			return nil, err
		}
		b.ulids = append(b.ulids, tg.ULID)
	}
	return out, nil
}

// Bulk applies the assigns to every target: one commit in each store. Every
// store is checked first, so a violation anywhere writes nothing.
func Bulk(ss *usecase.Session, in BulkInput) (BulkResult, error) {
	if len(in.Assigns) == 0 {
		return BulkResult{}, usecase.NewErr(usecase.KindUsage, "bulk.empty", errors.New("nothing to change"))
	}
	stores, err := openBulk(ss, in.Targets)
	if err != nil {
		return BulkResult{}, err
	}
	res := BulkResult{Tasks: []BulkTask{}}
	for _, b := range stores {
		for _, u := range b.ulids {
			res.Tasks = append(res.Tasks, b.row(u))
		}
		_, err := b.ss.UpdateCommit(b.s, b.op(in, true), b.write(in))
		var ves *store.ValidationErrors
		switch {
		case errors.As(err, &ves):
			for _, ve := range ves.Tasks {
				for _, v := range ve.Violations {
					res.Violations = append(res.Violations, BulkViolation{Task: b.row(ve.Task), Field: v.Field, Message: v.Msg})
				}
			}
		case err != nil:
			return BulkResult{}, err
		}
	}
	if in.DryRun {
		return res, nil
	}
	if len(res.Violations) > 0 {
		lines := make([]string, len(res.Violations))
		for i, v := range res.Violations {
			lines[i] = fmt.Sprintf("  %s: %s", v.Task.ID, v.Message)
		}
		return BulkResult{}, usecase.NewErr(usecase.KindInvalid, "bulk.invalid",
			fmt.Errorf("nothing is written; the change breaks the schema:\n%s", strings.Join(lines, "\n")), "violations", len(res.Violations))
	}
	for _, b := range stores {
		hash, err := b.ss.UpdateCommit(b.s, b.op(in, false), b.write(in))
		if err != nil {
			return BulkResult{}, err
		}
		if !hash.IsZero() {
			res.Commits = append(res.Commits, BulkCommit{Store: b.name, Commit: hash.String()})
		}
	}
	return res, nil
}
