package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

// TaskParam is the task a request is on: an ID, a former ID or a ULID, as
// the CLI resolves it.
type TaskParam struct {
	StoreParam
	ID string `path:"id" doc:"The ID, a former ID, or the ULID of the task"`
}

type listInput struct {
	StoreParam
	Q    string `query:"q" doc:"The query, as hh ls takes it; empty for every task"`
	Tree bool   `query:"tree" doc:"Return the tasks as a tree, as hh ls --tree"`
}

type listOutput struct{ Body browse.TaskList }

type showOutput struct{ Body *usecase.ShowView }

type historyInput struct {
	TaskParam
	Limit int `query:"limit" default:"50" minimum:"1" maximum:"1000"`
}

type historyOutput struct{ Body []usecase.LogView }

type showAtInput struct {
	TaskParam
	Commit string `path:"commit" doc:"The commit or the time to look at; nothing is changed"`
}

type peopleInput struct {
	Store string `query:"store" doc:"The store; without it, the people of every store"`
}

func (s *Server) registerTaskReads() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-tasks", Method: http.MethodGet, Path: "/api/tasks",
		Summary: "List the tasks a query finds", Tags: []string{"tasks"},
	}, func(_ context.Context, in *listInput) (*listOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		if in.Tree {
			t, err := browse.Tree(ss, in.Q)
			if err != nil {
				return nil, problemOf(err)
			}
			return &listOutput{Body: t.JSON()}, nil
		}
		l, err := browse.List(ss, in.Q)
		if err != nil {
			return nil, problemOf(err)
		}
		return &listOutput{Body: l.JSON()}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-task", Method: http.MethodGet, Path: "/api/tasks/{id}",
		Summary: "Show a task, as hh show", Tags: []string{"tasks"},
	}, func(_ context.Context, in *TaskParam) (*showOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := browse.Show(ss, in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return &showOutput{Body: v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-task-history", Method: http.MethodGet, Path: "/api/tasks/{id}/history",
		Summary: "List the commits of a task, newest first", Tags: []string{"tasks"},
	}, func(_ context.Context, in *historyInput) (*historyOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := browse.Log(ss, []string{in.ID}, in.Limit)
		if err != nil {
			return nil, problemOf(err)
		}
		return &historyOutput{Body: r.Logs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "list-people", Method: http.MethodGet, Path: "/api/people",
		Summary: "List the names to offer after an @: assignees, watchers, comment authors and commit authors", Tags: []string{"tasks"},
	}, func(_ context.Context, in *peopleInput) (*struct{ Body []string }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		names, err := browse.People(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []string }{Body: names}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "show-task-at", Method: http.MethodGet, Path: "/api/tasks/{id}/history/{commit}",
		Summary: "Show a task as it was at a commit, read and written nowhere", Tags: []string{"tasks"},
		Description: "The id and the workspace are those of now. problems, reverse, referenced_by, comments and history are empty, and progress is not filled. A task that did not exist at the commit is restore_at.not_found.",
	}, func(_ context.Context, in *showAtInput) (*showOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := tasks.ShowAt(ss, in.ID, in.Commit)
		if err != nil {
			return nil, problemOf(err)
		}
		return &showOutput{Body: &v}, nil
	})
}
