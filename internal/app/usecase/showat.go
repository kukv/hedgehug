package usecase

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

// ShowAt is the task arg as it was at at (a time or a commit, as RestoreAt
// takes it), read from the commit and written nowhere. The id and the
// workspace are those of now; the problems, the relations (reverse and
// referenced_by), the comments and the history are left empty, and the
// progress is not filled.
func (ss *Session) ShowAt(arg, at string) (ShowView, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return ShowView{}, err
	}
	if ts != ss {
		return ts.ShowAt(arg, at)
	}
	s, ix, err := ss.Open()
	if err != nil {
		return ShowView{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return ShowView{}, err
	}
	commit, err := ss.commitAt(s, ix.StoreName(), at)
	if err != nil {
		return ShowView{}, err
	}
	data, err := s.Repo().ReadAt(commit, model.TaskPath(e.ULID))
	if errors.Is(err, fs.ErrNotExist) {
		return ShowView{}, newErr(KindNotFound, "restore_at.not_found", fmt.Errorf("%s did not exist at %s", cmp.Or(ix.DisplayID(e), e.ULID.String()), at), "at", at)
	}
	if err != nil {
		return ShowView{}, err
	}
	old, err := model.DecodeTask(data)
	if err != nil {
		return ShowView{}, err
	}
	old.ID, old.FormerIDs, old.Workspace = e.ID, e.FormerIDs, e.Workspace
	entry := index.EntryOf(old)
	v := ShowView{
		TaskView: TaskViewOf(ix, entry),
		Body:     old.Body,
		Watchers: OrEmpty(old.Watchers),
		Repos:    OrEmpty(old.Repos),
		Comments: []CommentView{},
		Reverse:  map[string][]Ref{},
		History:  []LogView{},
	}
	v.Problems = []Problem{}
	v.ReferencedBy = []Ref{}
	v.Checklist = ChecklistOf(entry)
	return v, nil
}
