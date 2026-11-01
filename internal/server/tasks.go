package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/ops"
)

// TaskParam is the task a request is on: an ID, a former ID or a ULID, as
// the CLI resolves it.
type TaskParam struct {
	StoreParam
	ID string `path:"id" doc:"The ID, a former ID, or the ULID of the task"`
}

// listView is a row of a list: the task with its checklist and progress.
func listView(ix *index.Index, e *index.Entry) *ops.TaskView {
	v := ops.TaskViewOf(ix, e)
	v.Checklist = ops.ChecklistOf(e)
	v.Progress = ops.ProgressOf(ix, e)
	return v
}

type groupView struct {
	Key   string   `json:"key" doc:"The value of the group; empty for the tasks with none"`
	ULIDs []string `json:"ulids"`
}

type listInput struct {
	StoreParam
	Q    string `query:"q" doc:"The query, as hh ls takes it; empty for every task"`
	Tree bool   `query:"tree" doc:"Return the tasks as a tree, as hh ls --tree"`
}

type listOutput struct {
	Body struct {
		Tasks  *[]*ops.TaskView `json:"tasks,omitempty" doc:"The tasks found, without tree"`
		Groups []groupView      `json:"groups,omitempty" doc:"The groups, when the query has group:"`
		Roots  *[]*ops.TaskView `json:"roots,omitempty" doc:"The roots of the tree, with tree"`
		Cycle  []*ops.TaskView  `json:"cycle,omitempty" doc:"The tasks in a parent cycle, with tree"`
	}
}

type showOutput struct{ Body *ops.ShowView }

type historyInput struct {
	TaskParam
	Limit int `query:"limit" default:"50" minimum:"1" maximum:"1000"`
}

type historyOutput struct{ Body []ops.LogView }

func (s *Server) registerTaskReads() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-tasks", Method: http.MethodGet, Path: "/api/tasks",
		Summary: "List the tasks a query finds", Tags: []string{"tasks"},
	}, func(_ context.Context, in *listInput) (*listOutput, error) {
		q, err := ops.ParseQuery(in.Q)
		if err != nil {
			return nil, problemOf(err)
		}
		ss := s.session(in.Store)
		out := &listOutput{}
		if in.Tree {
			r, err := ss.Tree(q)
			if err != nil {
				return nil, problemOf(err)
			}
			roots := []*ops.TaskView{}
			for _, ts := range r.Stores {
				for _, n := range ts.Roots {
					roots = append(roots, ops.TreeViewOf(ts.Index, n))
				}
				for _, n := range ts.Cycle {
					out.Body.Cycle = append(out.Body.Cycle, ops.TreeViewOf(ts.Index, n))
				}
			}
			out.Body.Roots = &roots
			return out, nil
		}
		r, err := ss.List(q)
		if err != nil {
			return nil, problemOf(err)
		}
		tasks := make([]*ops.TaskView, 0, len(r.Entries))
		for _, e := range r.Entries {
			ix, err := e.Store.Index()
			if err != nil {
				return nil, problemOf(err)
			}
			tasks = append(tasks, listView(ix, e.Entry))
		}
		out.Body.Tasks = &tasks
		if ops.HasTerm(q, "group") {
			for _, g := range r.Groups {
				gv := groupView{Key: g.Key, ULIDs: make([]string, len(g.Entries))}
				for i, e := range g.Entries {
					gv.ULIDs[i] = e.Entry.ULID.String()
				}
				out.Body.Groups = append(out.Body.Groups, gv)
			}
		}
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-task", Method: http.MethodGet, Path: "/api/tasks/{id}",
		Summary: "Show a task, as hh show", Tags: []string{"tasks"},
	}, func(_ context.Context, in *TaskParam) (*showOutput, error) {
		r, err := s.session(in.Store).Show(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return &showOutput{Body: ops.ShowViewOf(r)}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-task-history", Method: http.MethodGet, Path: "/api/tasks/{id}/history",
		Summary: "List the commits of a task, newest first", Tags: []string{"tasks"},
	}, func(_ context.Context, in *historyInput) (*historyOutput, error) {
		r, err := s.session(in.Store).Log([]string{in.ID}, in.Limit)
		if err != nil {
			return nil, problemOf(err)
		}
		return &historyOutput{Body: ops.StoreLogViews(r.Logs)}, nil
	})
}
