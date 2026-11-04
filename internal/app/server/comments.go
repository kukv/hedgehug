package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

// CommentParam is the comment a request is on. It is exported because huma
// skips an embedded struct whose type is not.
type CommentParam struct {
	TaskParam
	CID string `path:"cid" doc:"The ULID of the comment"`
}

type commentInput struct {
	TaskParam
	Body struct {
		Body string `json:"body" minLength:"1" doc:"The comment in Markdown"`
	}
}

type commentEditInput struct {
	CommentParam
	Body struct {
		Body string `json:"body" minLength:"1" doc:"The comment in Markdown"`
	}
}

type commentOutput struct{ Body comments.CommentWrite }

// commentOut is the output of a write to a comment.
func commentOut(r comments.CommentResult) *commentOutput {
	return &commentOutput{Body: r.Write()}
}

// commentOf refuses a comment that is not of the task of the path.
func commentOf(ss *usecase.Session, id, cid string) error {
	u, err := ss.TaskULID(id)
	if err != nil {
		return problemOf(err)
	}
	ces, err := comments.OpenComment(ss, cid)
	if err != nil {
		return problemOf(err)
	}
	if ces.Ref.Task != u {
		return newProblem(http.StatusNotFound, "comment.not_found", "comment "+cid+" is not a comment of "+id)
	}
	return nil
}

func (s *Server) registerComments() {
	huma.Register(s.api, huma.Operation{
		OperationID: "add-comment", Method: http.MethodPost, Path: "/api/tasks/{id}/comments", DefaultStatus: http.StatusCreated,
		Summary: "Add a comment, as hh comment add", Tags: []string{"comments"},
	}, func(_ context.Context, in *commentInput) (*commentOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := comments.CommentAdd(ss, in.ID, in.Body.Body)
		if err != nil {
			return nil, problemOf(err)
		}
		return commentOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "edit-comment", Method: http.MethodPatch, Path: "/api/tasks/{id}/comments/{cid}",
		Summary: "Change a comment, as hh comment edit", Tags: []string{"comments"},
	}, func(_ context.Context, in *commentEditInput) (*commentOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		if err := commentOf(ss, in.ID, in.CID); err != nil {
			return nil, err
		}
		r, err := comments.CommentEdit(ss, in.CID, in.Body.Body)
		if err != nil {
			return nil, problemOf(err)
		}
		return commentOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-comment", Method: http.MethodDelete, Path: "/api/tasks/{id}/comments/{cid}",
		Summary: "Remove a comment, as hh comment rm", Tags: []string{"comments"},
	}, func(_ context.Context, in *CommentParam) (*commentOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		if err := commentOf(ss, in.ID, in.CID); err != nil {
			return nil, err
		}
		r, err := comments.CommentRemove(ss, in.CID)
		if err != nil {
			return nil, problemOf(err)
		}
		return commentOut(r), nil
	})

	for _, w := range []struct {
		method, id, summary string
		on                  bool
	}{
		{http.MethodPut, "watch-task", "Watch a task, as hh watch", true},
		{http.MethodDelete, "unwatch-task", "Stop watching a task, as hh unwatch", false},
	} {
		huma.Register(s.api, huma.Operation{
			OperationID: w.id, Method: w.method, Path: "/api/tasks/{id}/watchers/me",
			Summary: w.summary, Tags: []string{"tasks"},
		}, func(_ context.Context, in *TaskParam) (*taskWriteOutput, error) {
			ss := s.session(in.Store)
			defer ss.Close()
			r, err := tasks.Watch(ss, in.ID, w.on)
			if err != nil {
				return nil, problemOf(err)
			}
			return writeOut(r)
		})
	}
}
