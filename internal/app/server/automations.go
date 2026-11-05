package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase/automations"
)

type automationListInput struct {
	StoreParam
	Project string `query:"project" doc:"Only what applies in this project, with its scoped disable in enabled"`
}

type automationAddInput struct {
	StoreParam
	Body struct {
		ID       string `json:"id" minLength:"1"`
		On       string `json:"on" minLength:"1" doc:"The event, such as pr.merged"`
		Status   string `json:"status,omitempty" doc:"The status to set; not with category"`
		Category string `json:"category,omitempty" doc:"The category whose default to set; not with status"`
		When     string `json:"when,omitempty" doc:"The condition, such as all_linked_prs_merged"`
		Project  string `json:"project,omitempty" doc:"The project the rule is of; without it, the store's"`
	}
}

// AutomationParam is the rule a request is on. It is exported because huma
// skips an embedded struct whose type is not.
type AutomationParam struct {
	StoreParam
	ID string `path:"id" doc:"The id of the rule"`
}

type automationEnabledInput struct {
	AutomationParam
	Body struct {
		Enabled bool   `json:"enabled"`
		Project string `json:"project,omitempty" doc:"Turn it on or off in this project only (scoped disable)"`
	}
}

type automationFieldsInput struct {
	StoreParam
	Body struct {
		DefaultSync *string           `json:"default_sync,omitempty" doc:"The mode of a new issue link"`
		Push        *[]string         `json:"push,omitempty" doc:"Replaces the fields push mode syncs"`
		Mirror      *[]string         `json:"mirror,omitempty" doc:"Replaces the fields mirror mode syncs"`
		Props       map[string]string `json:"props,omitempty" doc:"Field to property; an empty value takes it off"`
	}
}

type automationsOutput struct{ Body []automations.RuleView }

type fieldsOutput struct{ Body automations.FieldsView }

func (s *Server) registerAutomations() {
	list := func(store, project string) (*automationsOutput, error) {
		ss := s.session(store)
		defer ss.Close()
		out, err := automations.List(ss, project)
		if err != nil {
			return nil, problemOf(err)
		}
		if out == nil {
			out = []automations.RuleView{}
		}
		return &automationsOutput{out}, nil
	}

	huma.Register(s.api, huma.Operation{
		OperationID: "list-automations", Method: http.MethodGet, Path: "/api/automations",
		Summary: "List the rules in the order they apply, as hh automation list", Tags: []string{"automations"},
	}, func(_ context.Context, in *automationListInput) (*automationsOutput, error) {
		return list(in.Store, in.Project)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "add-automation", Method: http.MethodPost, Path: "/api/automations", DefaultStatus: http.StatusCreated,
		Summary: "Add a rule, as hh automation add", Tags: []string{"automations"},
		Description: "The answer lists the rules of the store.",
	}, func(_ context.Context, in *automationAddInput) (*automationsOutput, error) {
		b := in.Body
		if (b.Status == "") == (b.Category == "") {
			return nil, badRequest(errors.New("give status or category"))
		}
		ss := s.session(in.Store)
		defer ss.Close()
		if err := automations.Add(ss, automations.AddInput{
			ID: b.ID, On: b.On, Status: b.Status, Category: b.Category, When: b.When, Project: b.Project,
		}); err != nil {
			return nil, problemOf(err)
		}
		return list(in.Store, "")
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "delete-automation", Method: http.MethodDelete, Path: "/api/automations/{id}",
		Summary: "Remove a rule, and its id from every scoped disable, as hh automation rm", Tags: []string{"automations"},
		Description: "The answer lists the rules of the store.",
	}, func(_ context.Context, in *AutomationParam) (*automationsOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		if err := automations.Remove(ss, in.ID); err != nil {
			return nil, problemOf(err)
		}
		return list(in.Store, "")
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "set-automation-enabled", Method: http.MethodPut, Path: "/api/automations/{id}/enabled",
		Summary: "Turn a rule on or off, as hh automation enable and disable", Tags: []string{"automations"},
		Description: "The answer lists the rules of the store, or those of the project given.",
	}, func(_ context.Context, in *automationEnabledInput) (*automationsOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		if err := automations.SetEnabled(ss, in.ID, in.Body.Project, in.Body.Enabled); err != nil {
			return nil, problemOf(err)
		}
		return list(in.Store, in.Body.Project)
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-automation-stats", Method: http.MethodGet, Path: "/api/automations/stats",
		Summary: "Count what each rule did in the last 7 days; a rule that did nothing is not listed", Tags: []string{"automations"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []automations.RuleStat }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		out, err := automations.Stats(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []automations.RuleStat }{out}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-automation-fields", Method: http.MethodGet, Path: "/api/automations/fields",
		Summary: "Read issue_fields, as hh automation fields", Tags: []string{"automations"},
	}, func(_ context.Context, in *StoreParam) (*fieldsOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := automations.Fields(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &fieldsOutput{v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-automation-fields", Method: http.MethodPatch, Path: "/api/automations/fields",
		Summary: "Change issue_fields, as hh automation fields with flags", Tags: []string{"automations"},
	}, func(_ context.Context, in *automationFieldsInput) (*fieldsOutput, error) {
		b := in.Body
		ss := s.session(in.Store)
		defer ss.Close()
		if err := automations.SetFields(ss, automations.FieldsInput{
			DefaultSync: b.DefaultSync, Push: b.Push, Mirror: b.Mirror, Props: b.Props,
		}); err != nil {
			return nil, problemOf(err)
		}
		v, err := automations.Fields(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &fieldsOutput{v}, nil
	})
}
