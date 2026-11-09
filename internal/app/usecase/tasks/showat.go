package tasks

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// ShowAt is the task arg as it was at at (a time or a commit, as RestoreAt
// takes it), read from the commit and written nowhere. The id and the
// workspace are those of now; the problems, the relations (reverse and
// referenced_by), the comments and the history are left empty, and the
// progress is not filled.
func ShowAt(ss *usecase.Session, arg, at string) (usecase.ShowView, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return usecase.ShowView{}, err
	}
	if ts != ss {
		return ShowAt(ts, arg, at)
	}
	s, ix, err := ss.Open()
	if err != nil {
		return usecase.ShowView{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return usecase.ShowView{}, err
	}
	commit, err := commitAt(ss, s, ix.StoreName(), at)
	if err != nil {
		return usecase.ShowView{}, err
	}
	data, err := readTaskAt(s, commit, e.ULID)
	if errors.Is(err, fs.ErrNotExist) {
		return usecase.ShowView{}, usecase.NewErr(usecase.KindNotFound, "restore_at.not_found", fmt.Errorf("%s did not exist at %s", cmp.Or(ix.DisplayID(e), e.ULID.String()), at), "at", at)
	}
	if err != nil {
		return usecase.ShowView{}, err
	}
	old, err := model.DecodeTask(data)
	if err != nil {
		return usecase.ShowView{}, err
	}
	old.ID, old.FormerIDs, old.Workspace = e.ID, e.FormerIDs, e.Workspace
	entry := index.EntryOf(old)
	v := usecase.ShowView{
		TaskView: usecase.TaskViewOf(ix, entry),
		Body:     old.Body,
		Watchers: usecase.OrEmpty(old.Watchers),
		Repos:    usecase.OrEmpty(old.Repos),
		Comments: []usecase.CommentView{},
		Reverse:  map[string][]usecase.Ref{},
		History:  []usecase.LogView{},
	}
	v.Problems = []usecase.Problem{}
	v.ReferencedBy = []usecase.Ref{}
	v.Checklist = usecase.ChecklistOf(entry)
	return v, nil
}
