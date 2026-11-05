package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
	"github.com/kukv/hedgehug/internal/app/usecase/worklogs"
)

// PlanParam is the sprint or release a request is on. It is exported because
// huma skips an embedded struct whose type is not.
type PlanParam struct {
	StoreParam
	ID        string `path:"id" doc:"The ULID or the name"`
	Workspace string `query:"workspace" doc:"The workspace, when the name is in several"`
}

type planListInput struct {
	StoreParam
	Workspace string `query:"workspace" doc:"Only this workspace"`
	State     string `query:"state" doc:"Only this state"`
}

type sprintAddInput struct {
	StoreParam
	Body struct {
		Name      string `json:"name"`
		Start     string `json:"start,omitempty" doc:"YYYY-MM-DD"`
		End       string `json:"end,omitempty" doc:"YYYY-MM-DD"`
		Goal      string `json:"goal,omitempty"`
		Workspace string `json:"workspace,omitempty" doc:"Without it, the only one of the store"`
	}
}

type sprintPatchInput struct {
	PlanParam
	Body struct {
		Name  *string `json:"name,omitempty"`
		Start *string `json:"start,omitempty" doc:"YYYY-MM-DD; empty clears it"`
		End   *string `json:"end,omitempty" doc:"YYYY-MM-DD; empty clears it"`
		Goal  *string `json:"goal,omitempty"`
	}
}

type sprintCloseInput struct {
	PlanParam
	Body struct {
		To string `json:"to,omitempty" doc:"Where the unfinished tasks go: next, backlog or the name of a sprint; may be left out when none is left"`
	}
}

type sprintOutput struct{ Body planning.SprintWrite }

type releaseAddInput struct {
	StoreParam
	Body struct {
		Name      string `json:"name"`
		Date      string `json:"date,omitempty" doc:"YYYY-MM-DD"`
		Workspace string `json:"workspace,omitempty" doc:"Without it, the only one of the store"`
	}
}

type releasePatchInput struct {
	PlanParam
	Body struct {
		Name  *string `json:"name,omitempty"`
		Date  *string `json:"date,omitempty" doc:"YYYY-MM-DD; empty clears it"`
		State *string `json:"state,omitempty" enum:"unreleased,released,archived"`
	}
}

type releaseOutput struct{ Body planning.ReleaseWrite }

type releaseNotesOutput struct {
	Body struct {
		Notes string `json:"notes" doc:"In Markdown"`
	}
}

type worklogAddInput struct {
	TaskParam
	Body struct {
		Duration string `json:"duration" minLength:"1" doc:"Such as 1h30m"`
		Date     string `json:"date,omitempty" doc:"YYYY-MM-DD; without it, today"`
		Note     string `json:"note,omitempty"`
	}
}

// WorklogParam is the worklog entry a request is on. It is exported for the
// reason of PlanParam.
type WorklogParam struct {
	TaskParam
	Log string `path:"log" doc:"The ULID of the entry"`
}

type worklogOutput struct{ Body worklogs.WorklogWrite }

// planField is a field of a PATCH: nil when it is not set.
type planField struct {
	key string
	v   *string
}

// planAssigns turns the fields of a PATCH that are set into assignments.
func planAssigns(fields ...planField) []usecase.Assign {
	var out []usecase.Assign
	for _, f := range fields {
		if f.v != nil {
			out = append(out, usecase.Assign{Key: f.key, Op: "=", Value: *f.v})
		}
	}
	return out
}

func (s *Server) registerPlanning() {
	s.registerSprints()
	s.registerReleases()
	s.registerWorklogs()
}

func (s *Server) registerSprints() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-sprints", Method: http.MethodGet, Path: "/api/sprints",
		Summary: "List the sprints, as hh sprint ls", Tags: []string{"planning"},
	}, func(_ context.Context, in *planListInput) (*struct{ Body []planning.SprintView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		vs, err := planning.SprintList(ss, in.Workspace, in.State)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []planning.SprintView }{vs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "add-sprint", Method: http.MethodPost, Path: "/api/sprints", DefaultStatus: http.StatusCreated,
		Summary: "Add a sprint, as hh sprint add", Tags: []string{"planning"},
	}, func(_ context.Context, in *sprintAddInput) (*sprintOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		b := in.Body
		w, err := planning.SprintAdd(ss, planning.SprintAddInput{Name: b.Name, Start: b.Start, End: b.End, Goal: b.Goal, Workspace: b.Workspace})
		if err != nil {
			return nil, problemOf(err)
		}
		return &sprintOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-sprint", Method: http.MethodPatch, Path: "/api/sprints/{id}",
		Summary: "Change a sprint, as hh sprint set", Tags: []string{"planning"},
	}, func(_ context.Context, in *sprintPatchInput) (*sprintOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		b := in.Body
		w, err := planning.SprintSet(ss, in.ID, in.Workspace, planAssigns(
			planField{"name", b.Name}, planField{"start", b.Start}, planField{"end", b.End}, planField{"goal", b.Goal}))
		if err != nil {
			return nil, problemOf(err)
		}
		return &sprintOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "start-sprint", Method: http.MethodPost, Path: "/api/sprints/{id}/start",
		Summary: "Start a sprint, as hh sprint start", Tags: []string{"planning"},
	}, func(_ context.Context, in *PlanParam) (*sprintOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := planning.SprintStart(ss, in.ID, in.Workspace)
		if err != nil {
			return nil, problemOf(err)
		}
		return &sprintOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "close-sprint", Method: http.MethodPost, Path: "/api/sprints/{id}/close",
		Summary: "Close a sprint, as hh sprint close", Tags: []string{"planning"},
	}, func(_ context.Context, in *sprintCloseInput) (*sprintOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := planning.SprintClose(ss, in.ID, in.Workspace, in.Body.To)
		if err != nil {
			return nil, problemOf(err)
		}
		return &sprintOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-sprint", Method: http.MethodDelete, Path: "/api/sprints/{id}",
		Summary: "Delete a sprint, as hh sprint rm", Tags: []string{"planning"},
	}, func(_ context.Context, in *PlanParam) (*sprintOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := planning.SprintRemove(ss, in.ID, in.Workspace)
		if err != nil {
			return nil, problemOf(err)
		}
		return &sprintOutput{w}, nil
	})
}

func (s *Server) registerReleases() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-releases", Method: http.MethodGet, Path: "/api/releases",
		Summary: "List the releases, as hh release ls", Tags: []string{"planning"},
	}, func(_ context.Context, in *planListInput) (*struct{ Body []planning.ReleaseView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		vs, err := planning.ReleaseList(ss, in.Workspace, in.State)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []planning.ReleaseView }{vs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "add-release", Method: http.MethodPost, Path: "/api/releases", DefaultStatus: http.StatusCreated,
		Summary: "Add a release, as hh release add", Tags: []string{"planning"},
	}, func(_ context.Context, in *releaseAddInput) (*releaseOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := planning.ReleaseAdd(ss, in.Body.Name, in.Body.Date, in.Body.Workspace)
		if err != nil {
			return nil, problemOf(err)
		}
		return &releaseOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-release", Method: http.MethodPatch, Path: "/api/releases/{id}",
		Summary: "Change a release, as hh release set", Tags: []string{"planning"},
	}, func(_ context.Context, in *releasePatchInput) (*releaseOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		b := in.Body
		w, err := planning.ReleaseSet(ss, in.ID, in.Workspace, planAssigns(
			planField{"name", b.Name}, planField{"date", b.Date}, planField{"state", b.State}))
		if err != nil {
			return nil, problemOf(err)
		}
		return &releaseOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-release-notes", Method: http.MethodGet, Path: "/api/releases/{id}/notes",
		Summary: "Read the release notes, as hh release notes", Tags: []string{"planning"},
	}, func(_ context.Context, in *PlanParam) (*releaseNotesOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		notes, err := planning.ReleaseNotes(ss, in.ID, in.Workspace)
		if err != nil {
			return nil, problemOf(err)
		}
		out := &releaseNotesOutput{}
		out.Body.Notes = notes
		return out, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-release", Method: http.MethodDelete, Path: "/api/releases/{id}",
		Summary: "Delete a release, as hh release rm", Tags: []string{"planning"},
	}, func(_ context.Context, in *PlanParam) (*releaseOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := planning.ReleaseRemove(ss, in.ID, in.Workspace)
		if err != nil {
			return nil, problemOf(err)
		}
		return &releaseOutput{w}, nil
	})
}

func (s *Server) registerWorklogs() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-worklogs", Method: http.MethodGet, Path: "/api/tasks/{id}/worklogs",
		Summary: "List the time spent on a task, as hh worklog ls", Tags: []string{"planning"},
	}, func(_ context.Context, in *TaskParam) (*struct{ Body []worklogs.WorklogView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		vs, err := worklogs.List(ss, in.ID)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []worklogs.WorklogView }{vs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "add-worklog", Method: http.MethodPost, Path: "/api/tasks/{id}/worklogs", DefaultStatus: http.StatusCreated,
		Summary: "Record time spent on a task, as hh worklog", Tags: []string{"planning"},
	}, func(_ context.Context, in *worklogAddInput) (*worklogOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := worklogs.Add(ss, in.ID, in.Body.Duration, in.Body.Date, in.Body.Note)
		if err != nil {
			return nil, problemOf(err)
		}
		return &worklogOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-worklog", Method: http.MethodDelete, Path: "/api/tasks/{id}/worklogs/{log}",
		Summary: "Delete a worklog entry, as hh worklog rm", Tags: []string{"planning"},
	}, func(_ context.Context, in *WorklogParam) (*worklogOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := worklogs.Remove(ss, in.ID, in.Log)
		if err != nil {
			return nil, problemOf(err)
		}
		return &worklogOutput{w}, nil
	})
}
