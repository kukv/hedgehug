package browse

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Targets lists the tasks a query finds, for a bulk update.
func Targets(ss *usecase.Session, queryText string) ([]usecase.TaskTarget, error) {
	q, err := usecase.ParseQuery(queryText)
	if err != nil {
		return nil, err
	}
	set, err := listSet(ss)
	if err != nil {
		return nil, err
	}
	res, err := queryMounted(ss, set, q)
	if err != nil {
		return nil, err
	}
	out := make([]usecase.TaskTarget, len(res.Entries))
	for i, e := range res.Entries {
		out[i] = usecase.TaskTarget{Store: e.Store.Name, ULID: e.Entry.ULID}
	}
	return out, nil
}

// TargetsOf finds the stores of the tasks ulids among the mounted ones.
func TargetsOf(ss *usecase.Session, ulids []string) ([]usecase.TaskTarget, error) {
	set, err := listSet(ss)
	if err != nil {
		return nil, err
	}
	out := make([]usecase.TaskTarget, 0, len(ulids))
	for _, s := range ulids {
		u, err := model.ParseULID(s)
		if err != nil {
			return nil, usecase.NewErr(usecase.KindUsage, "task.bad_ulid", fmt.Errorf("%q is not a ULID", s), "id", s)
		}
		found := false
		for _, m := range set.Stores {
			if ss.StoreName != "" && m.Name != ss.StoreName {
				continue
			}
			if err := m.With(func(ix *index.Index) {
				if _, ok := ix.Task(u); ok && !found {
					found = true
					out = append(out, usecase.TaskTarget{Store: m.Name, ULID: u})
				}
			}); err != nil {
				return nil, usecase.Classify(err)
			}
		}
		if !found {
			return nil, usecase.NewErr(usecase.KindNotFound, "task.not_found", fmt.Errorf("task %s is not in the stores", s), "id", s)
		}
	}
	return out, nil
}
