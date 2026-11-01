package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
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
func (s *Server) viewsOf(ss *ops.Session, ulids ...model.ULID) ([]*ops.TaskView, error) {
	_, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	out := make([]*ops.TaskView, 0, len(ulids))
	for _, u := range ulids {
		e, ok := ix.Task(u)
		if !ok {
			return nil, fmt.Errorf("task %s is not in the index after the write", u)
		}
		out = append(out, listView(ix, e))
	}
	return out, nil
}

// assignsOf turns the JSON value v of key into what hedgehug set takes: a
// scalar is key=value, null is key= (clear), a list is clear then key+=item
// for each.
func assignsOf(key string, v any) ([]ops.Assign, error) {
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
		return []ops.Assign{{Key: key, Op: "=", Value: ""}}, nil
	case []any:
		out := []ops.Assign{{Key: key, Op: "=", Value: ""}}
		for _, it := range x {
			s, err := scalar(it)
			if err != nil {
				return nil, err
			}
			out = append(out, ops.Assign{Key: key, Op: "+=", Value: s})
		}
		return out, nil
	}
	s, err := scalar(v)
	if err != nil {
		return nil, err
	}
	return []ops.Assign{{Key: key, Op: "=", Value: s}}, nil
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
func (f *Fields) assigns() ([]ops.Assign, error) {
	var out []ops.Assign
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
		if err := add(n, f.Props[n]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type taskWrite struct {
	Task   *ops.TaskView `json:"task"`
	Commit *string       `json:"commit" doc:"The commit made; null when nothing changed"`
	Note   string        `json:"note,omitempty" doc:"What restore at a commit has to say, as hh restore --at prints it"`
}

type taskWriteOutput struct{ Body taskWrite }

type affectedWrite struct {
	Tasks  []*ops.TaskView `json:"tasks,omitempty" doc:"The task and the tasks under it, as they are now"`
	ULIDs  []string        `json:"ulids,omitempty" doc:"The tasks purged"`
	Commit *string         `json:"commit"`
}

type affectedOutput struct{ Body affectedWrite }

// writeOut is the output of a write to one task.
func (s *Server) writeOut(ss *ops.Session, u model.ULID, commit string) (*taskWriteOutput, error) {
	vs, err := s.viewsOf(ss, u)
	if err != nil {
		return nil, problemOf(err)
	}
	return &taskWriteOutput{Body: taskWrite{Task: vs[0], Commit: commitPtr(commit)}}, nil
}

// affectedOut is the output of rm and restore: the task, then the tasks
// under it.
func (s *Server) affectedOut(ss *ops.Session, r ops.AffectedResult) (*affectedOutput, error) {
	ulids := []model.ULID{r.Entry.ULID}
	for _, e := range r.Targets {
		if e.ULID != r.Entry.ULID {
			ulids = append(ulids, e.ULID)
		}
	}
	vs, err := s.viewsOf(ss, ulids...)
	if err != nil {
		return nil, problemOf(err)
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
		Assignees []string       `json:"assignees,omitempty"`
		Labels    []string       `json:"labels,omitempty"`
		Props     map[string]any `json:"props,omitempty" doc:"Property name to value"`
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

type deleteInput struct {
	TaskParam
	Purge   bool   `query:"purge" doc:"Remove the files for good; the task must be deleted already"`
	Confirm string `query:"confirm" doc:"With purge: the ULIDs the user agreed to purge, comma-separated, as purge.unconfirmed listed them"`
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
		assigns, err := f.assigns()
		if err != nil {
			return nil, badRequest(err)
		}
		ss := s.session(in.Store)
		res, err := ss.Add(ops.AddInput{Title: b.Title, Parent: b.Parent, Workspace: b.Workspace, Kind: b.Kind, Status: b.Status, Assigns: assigns})
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
		res, err := ss.Patch(in.ID, ops.PatchInput{Assigns: assigns, Body: in.Body.Body})
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
		res, err := ss.Move(ops.MoveInput{Arg: in.ID, Workspace: b.Workspace, Parent: b.Parent, Root: b.Root, Before: b.Before, After: b.After})
		if err != nil {
			return nil, problemOf(err)
		}
		return s.writeOut(ss, res.ULID, res.Commit)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "restore-task", Method: http.MethodPost, Path: "/api/tasks/{id}/restore",
		Summary: "Restore a deleted task and the tasks under it, as hh restore", Tags: []string{"tasks"},
	}, func(_ context.Context, in *TaskParam) (*affectedOutput, error) {
		ss := s.session(in.Store)
		res, err := ss.Restore(in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return s.affectedOut(ss, res)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "delete-task", Method: http.MethodDelete, Path: "/api/tasks/{id}",
		Summary: "Delete a task and the tasks under it (hh rm), or purge them (hh purge)", Tags: []string{"tasks"},
	}, func(_ context.Context, in *deleteInput) (*affectedOutput, error) {
		ss := s.session(in.Store)
		if !in.Purge {
			res, err := ss.Remove(in.ID)
			if err != nil {
				return nil, problemOf(err)
			}
			return s.affectedOut(ss, res)
		}
		return s.purge(ss, in.ID, in.Confirm)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "restore-task-at", Method: http.MethodPost, Path: "/api/tasks/{id}/history/{commit}/restore",
		Summary: "Bring a task back to how it was at a commit, as hh restore --at", Tags: []string{"tasks"},
	}, func(_ context.Context, in *restoreAtInput) (*taskWriteOutput, error) {
		ss := s.session(in.Store)
		res, err := ss.RestoreAt(in.ID, in.Commit)
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

// purge purges the task id after the confirmation: without confirm, or
// with a list that is not the tasks to purge, it is refused with
// purge.unconfirmed and those tasks, for the GUI to ask the user.
func (s *Server) purge(ss *ops.Session, id, confirm string) (*affectedOutput, error) {
	ix, _, targets, err := ss.PurgeTargets(id)
	if err != nil {
		return nil, problemOf(err)
	}
	var confirmed []model.ULID
	for _, c := range strings.Split(confirm, ",") {
		if c == "" {
			continue
		}
		u, err := model.ParseULID(c)
		if err != nil {
			return nil, badRequest(fmt.Errorf("confirm: %w", err))
		}
		confirmed = append(confirmed, u)
	}
	if !sameULIDs(targets, confirmed) {
		return nil, unconfirmed(ix, targets)
	}
	res, err := ss.Purge(id, confirmed)
	if ops.AsError(err) != nil && ops.AsError(err).Code == "purge.changed" {
		ix, _, targets, err2 := ss.PurgeTargets(id)
		if err2 != nil {
			return nil, problemOf(err2)
		}
		return nil, unconfirmed(ix, targets)
	}
	if err != nil {
		return nil, problemOf(err)
	}
	out := &affectedOutput{Body: affectedWrite{Commit: commitPtr(res.Commit)}}
	for _, e := range res.Targets {
		out.Body.ULIDs = append(out.Body.ULIDs, e.ULID.String())
	}
	return out, nil
}

// unconfirmed is the purge.unconfirmed problem with the tasks to purge.
func unconfirmed(ix *index.Index, targets []*index.Entry) *Problem {
	ts := make([]map[string]string, len(targets))
	for i, e := range targets {
		v := ops.TaskViewOf(ix, e)
		ts[i] = map[string]string{"ulid": v.ULID, "id": v.ID, "title": v.Title}
	}
	p := newProblem(http.StatusConflict, "purge.unconfirmed", fmt.Sprintf("purging %d tasks for good needs a confirmation", len(targets)))
	p.Args = map[string]any{"targets": ts}
	return p
}

// sameULIDs reports whether confirmed holds the ULIDs of targets, in any
// order.
func sameULIDs(targets []*index.Entry, confirmed []model.ULID) bool {
	if len(targets) != len(confirmed) {
		return false
	}
	for _, e := range targets {
		if !slices.Contains(confirmed, e.ULID) {
			return false
		}
	}
	return true
}
