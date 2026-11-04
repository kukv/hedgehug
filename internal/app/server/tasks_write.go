package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

// commitPtr is the commit of a write: nil when nothing was committed.
func commitPtr(hex string) *string {
	if hex == "" {
		return nil
	}
	return &hex
}

// viewsOf opens the index again, after a write, and returns the tasks as
// rows of a list.
func (s *Server) viewsOf(ss *usecase.Session, ulids ...model.ULID) ([]*usecase.TaskView, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return nil, problemOf(err)
	}
	out := make([]*usecase.TaskView, 0, len(ulids))
	for _, u := range ulids {
		e, ok := ix.Task(u)
		if !ok {
			// Another process purged it after the write.
			p := newProblem(http.StatusNotFound, "task.gone", fmt.Sprintf("%s is no longer in the store", u))
			p.Args = map[string]any{"id": u.String()}
			return nil, p
		}
		out = append(out, listView(ix, e))
	}
	return out, nil
}

// assignsOf turns the JSON value v of key into what hedgehug set takes: a
// scalar is key=value, null is key= (clear), a list is clear then key+=item
// for each.
func assignsOf(key string, v any) ([]usecase.Assign, error) {
	scalar := func(x any) (string, error) {
		switch x := x.(type) {
		case string:
			return x, nil
		case bool:
			return strconv.FormatBool(x), nil
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64), nil
		}
		return "", fmt.Errorf("%s: %v is not a string, a number or a boolean", key, x)
	}
	switch x := v.(type) {
	case nil:
		return []usecase.Assign{{Key: key, Op: "=", Value: ""}}, nil
	case []any:
		out := []usecase.Assign{{Key: key, Op: "=", Value: ""}}
		for _, it := range x {
			s, err := scalar(it)
			if err != nil {
				return nil, err
			}
			out = append(out, usecase.Assign{Key: key, Op: "+=", Value: s})
		}
		return out, nil
	}
	s, err := scalar(v)
	if err != nil {
		return nil, err
	}
	return []usecase.Assign{{Key: key, Op: "=", Value: s}}, nil
}

// badRequest is a 400 request.invalid problem for err.
func badRequest(err error) *Problem {
	return newProblem(http.StatusBadRequest, "request.invalid", err.Error())
}

// Fields are the fields hedgehug set changes, as POST and PATCH take them. It
// is exported because huma skips an embedded struct whose type is not.
type Fields struct {
	Title     *string        `json:"title,omitempty"`
	Kind      *string        `json:"kind,omitempty"`
	Status    *string        `json:"status,omitempty"`
	Assignees *[]string      `json:"assignees,omitempty" doc:"Replaces the assignees"`
	Watchers  *[]string      `json:"watchers,omitempty" doc:"Replaces the watchers"`
	Labels    *[]string      `json:"labels,omitempty" doc:"Replaces the labels"`
	Repos     *[]string      `json:"repos,omitempty" doc:"Replaces the repos"`
	BlockedBy *[]string      `json:"blocked_by,omitempty" doc:"Replaces the blockers: IDs or ULIDs"`
	Props     map[string]any `json:"props,omitempty" doc:"Property name to value; null clears it, a list replaces a multiple value"`
}

// assigns is f as hedgehug set's assignments, the properties by name.
func (f *Fields) assigns() ([]usecase.Assign, error) {
	var out []usecase.Assign
	add := func(key string, v any) error {
		as, err := assignsOf(key, v)
		out = append(out, as...)
		return err
	}
	for _, sc := range []struct {
		key string
		v   *string
	}{{"title", f.Title}, {"kind", f.Kind}, {"status", f.Status}} {
		if sc.v != nil {
			if err := add(sc.key, *sc.v); err != nil {
				return nil, err
			}
		}
	}
	for _, l := range []struct {
		key string
		v   *[]string
	}{{"assignees", f.Assignees}, {"watchers", f.Watchers}, {"labels", f.Labels}, {"repos", f.Repos}, {"blocked_by", f.BlockedBy}} {
		if l.v != nil {
			items := make([]any, len(*l.v))
			for i, s := range *l.v {
				items[i] = s
			}
			if err := add(l.key, items); err != nil {
				return nil, err
			}
		}
	}
	names := make([]string, 0, len(f.Props))
	for n := range f.Props {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		if slices.Contains(model.TaskKeys(), n) {
			return nil, fmt.Errorf("props: %s is a field of the task, not a property", n)
		}
		if err := add(n, f.Props[n]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type taskWrite struct {
	Task   *usecase.TaskView `json:"task"`
	Commit *string           `json:"commit" doc:"The commit made; null when nothing changed"`
	Note   string            `json:"note,omitempty" doc:"What restore at a commit has to say, as hh restore --at prints it"`
	// Detached is for a move to another workspace.
	Detached []usecase.Detached `json:"detached,omitempty" doc:"What a move to another workspace took off the tasks: the sprint and the releases that are not of the new workspace, as hh mv tells it"`
}

type taskWriteOutput struct{ Body taskWrite }

type affectedWrite struct {
	Tasks  []*usecase.TaskView `json:"tasks,omitempty" doc:"The task and the tasks under it, as they are now"`
	Commit *string             `json:"commit"`
}

type affectedOutput struct{ Body affectedWrite }

// writeOut is the output of a write to one task.
func (s *Server) writeOut(ss *usecase.Session, u model.ULID, commit string) (*taskWriteOutput, error) {
	vs, err := s.viewsOf(ss, u)
	if err != nil {
		return nil, err
	}
	return &taskWriteOutput{Body: taskWrite{Task: vs[0], Commit: commitPtr(commit)}}, nil
}

// affectedOut is the output of rm and restore: the task, then the tasks
// under it.
func (s *Server) affectedOut(ss *usecase.Session, r tasks.AffectedResult) (*affectedOutput, error) {
	ulids := []model.ULID{r.Entry.ULID}
	for _, e := range r.Targets {
		if e.ULID != r.Entry.ULID {
			ulids = append(ulids, e.ULID)
		}
	}
	vs, err := s.viewsOf(ss, ulids...)
	if err != nil {
		return nil, err
	}
	return &affectedOutput{Body: affectedWrite{Tasks: vs, Commit: commitPtr(r.Commit)}}, nil
}

type createInput struct {
	StoreParam
	Body struct {
		Title     string         `json:"title" minLength:"1"`
		Parent    string         `json:"parent,omitempty" doc:"The parent: an ID or a ULID"`
		Workspace string         `json:"workspace,omitempty" doc:"The workspace, by name or prefix"`
		Kind      string         `json:"kind,omitempty" doc:"project for hh add --kind project"`
		Status    string         `json:"status,omitempty"`
		Body      string         `json:"body,omitempty" doc:"The body in Markdown"`
		Assignees []string       `json:"assignees,omitempty"`
		Labels    []string       `json:"labels,omitempty"`
		Repos     []string       `json:"repos,omitempty" doc:"The repo IDs to put on the task"`
		Props     map[string]any `json:"props,omitempty" doc:"Property name to value"`
		Before    string         `json:"before,omitempty" doc:"Put it before this sibling; without a parent, it goes under the sibling's parent and in its workspace"`
		After     string         `json:"after,omitempty" doc:"Put it after this sibling; without a parent, it goes under the sibling's parent and in its workspace"`
	}
}

type patchInput struct {
	TaskParam
	Body struct {
		Fields
		Body *string `json:"body,omitempty" doc:"The body in Markdown"`
	}
}

type moveInput struct {
	TaskParam
	Body struct {
		Parent    string `json:"parent,omitempty" doc:"The new parent: an ID or a ULID"`
		Root      bool   `json:"root,omitempty" doc:"Move to the top, with no parent"`
		Workspace string `json:"workspace,omitempty" doc:"Move to another workspace"`
		Before    string `json:"before,omitempty" doc:"Put it before this sibling"`
		After     string `json:"after,omitempty" doc:"Put it after this sibling"`
	}
}

type restoreAtInput struct {
	TaskParam
	Commit string `path:"commit" doc:"The commit to bring the task back to"`
}

func (s *Server) registerTaskWrites() {
	huma.Register(s.api, huma.Operation{
		OperationID: "create-task", Method: http.MethodPost, Path: "/api/tasks", DefaultStatus: http.StatusCreated,
		Summary: "Add a task, as hh add", Tags: []string{"tasks"},
	}, func(_ context.Context, in *createInput) (*taskWriteOutput, error) {
		b := &in.Body
		f := Fields{Props: b.Props}
		if b.Assignees != nil {
			f.Assignees = &b.Assignees
		}
		if b.Labels != nil {
			f.Labels = &b.Labels
		}
		if b.Repos != nil {
			f.Repos = &b.Repos
		}
		assigns, err := f.assigns()
		if err != nil {
			return nil, badRequest(err)
		}
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := tasks.Add(ss, tasks.AddInput{Title: b.Title, Parent: b.Parent, Workspace: b.Workspace, Kind: b.Kind, Status: b.Status, Body: b.Body, Assigns: assigns, Before: b.Before, After: b.After})
		if err != nil {
			return nil, problemOf(err)
		}
		return s.writeOut(ss, res.ULID, res.Commit)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-task", Method: http.MethodPatch, Path: "/api/tasks/{id}",
		Summary: "Change the fields sent, in one commit, as hh set and hh edit", Tags: []string{"tasks"},
	}, func(_ context.Context, in *patchInput) (*taskWriteOutput, error) {
		assigns, err := in.Body.assigns()
		if err != nil {
			return nil, badRequest(err)
		}
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := tasks.Patch(ss, in.ID, tasks.PatchInput{Assigns: assigns, Body: in.Body.Body})
		if err != nil {
			return nil, problemOf(err)
		}
		return s.writeOut(ss, res.ULID, res.Commit)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "move-task", Method: http.MethodPost, Path: "/api/tasks/{id}/move",
		Summary: "Move a task, as hh mv", Tags: []string{"tasks"},
	}, func(_ context.Context, in *moveInput) (*taskWriteOutput, error) {
		b := in.Body
		if b.Root && b.Parent != "" {
			return nil, badRequest(errors.New("root and parent cannot both be given"))
		}
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := tasks.Move(ss, tasks.MoveInput{Arg: in.ID, Workspace: b.Workspace, Parent: b.Parent, Root: b.Root, Before: b.Before, After: b.After})
		if err != nil {
			return nil, problemOf(err)
		}
		out, err := s.writeOut(ss, res.ULID, res.Commit)
		if err != nil {
			return nil, err
		}
		out.Body.Detached = res.Detached
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "restore-task", Method: http.MethodPost, Path: "/api/tasks/{id}/restore",
		Summary: "Restore a deleted task and the tasks under it, as hh restore", Tags: []string{"tasks"},
	}, func(_ context.Context, in *TaskParam) (*affectedOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := tasks.Restore(ss, in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return s.affectedOut(ss, res)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "delete-task", Method: http.MethodDelete, Path: "/api/tasks/{id}",
		Summary: "Delete a task and the tasks under it (hh rm)", Tags: []string{"tasks"},
	}, func(_ context.Context, in *TaskParam) (*affectedOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := tasks.Remove(ss, in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return s.affectedOut(ss, res)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "restore-task-at", Method: http.MethodPost, Path: "/api/tasks/{id}/history/{commit}/restore",
		Summary: "Bring a task back to how it was at a commit, as hh restore --at", Tags: []string{"tasks"},
	}, func(_ context.Context, in *restoreAtInput) (*taskWriteOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		res, err := tasks.RestoreAt(ss, in.ID, in.Commit)
		if err != nil {
			return nil, problemOf(err)
		}
		out, err := s.writeOut(ss, res.ULID, res.Commit)
		if err != nil {
			return nil, err
		}
		out.Body.Note = res.Note
		return out, nil
	})
}
