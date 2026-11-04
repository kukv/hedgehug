package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase/views"
)

type ViewParam struct {
	StoreParam
	Name string `path:"name" doc:"The name of the view"`
}

type viewOutput struct{ Body views.ViewWrite }

type viewSaveInput struct {
	ViewParam
	Body struct {
		Layout        *string   `json:"layout,omitempty" enum:"tree,table,kanban"`
		Query         *string   `json:"query,omitempty"`
		GroupBy       *string   `json:"group_by,omitempty"`
		KanbanColumns *string   `json:"kanban_columns,omitempty"`
		Swimlane      *string   `json:"swimlane,omitempty"`
		Columns       *[]string `json:"columns,omitempty" doc:"The columns of the table"`
	}
}

type viewPatchInput struct {
	ViewParam
	Body struct {
		Name          *string   `json:"name,omitempty" doc:"The new name; a rename cannot come with settings"`
		Layout        *string   `json:"layout,omitempty" enum:"tree,table,kanban"`
		Query         *string   `json:"query,omitempty"`
		GroupBy       *string   `json:"group_by,omitempty"`
		KanbanColumns *string   `json:"kanban_columns,omitempty"`
		Swimlane      *string   `json:"swimlane,omitempty"`
		Columns       *[]string `json:"columns,omitempty" doc:"The columns of the table"`
	}
}

func viewOut(r views.ViewResult) *viewOutput { return &viewOutput{Body: r.Write()} }

func (s *Server) registerViews() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-views", Method: http.MethodGet, Path: "/api/views",
		Summary: "List the saved views, as hh view list", Tags: []string{"views"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []views.SavedView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		vs, err := views.ViewList(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []views.SavedView }{vs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-view", Method: http.MethodGet, Path: "/api/views/{name}",
		Summary: "Show a saved view, as hh view show", Tags: []string{"views"},
	}, func(_ context.Context, in *ViewParam) (*struct{ Body views.SavedView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		v, err := views.ViewGet(ss, in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body views.SavedView }{v}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "save-view", Method: http.MethodPut, Path: "/api/views/{name}",
		Summary: "Save a view; what is not sent stays as saved, as hh view save", Tags: []string{"views"},
	}, func(_ context.Context, in *viewSaveInput) (*viewOutput, error) {
		b := in.Body
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := views.ViewSave(ss, in.Name, views.ViewChange{
			Layout: b.Layout, Query: b.Query, GroupBy: b.GroupBy, KanbanColumns: b.KanbanColumns, Swimlane: b.Swimlane, Columns: b.Columns,
		})
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "update-view", Method: http.MethodPatch, Path: "/api/views/{name}",
		Summary: "Rename a saved view (as hh view rename), or change the settings that are given of a view that exists (404 if it does not, unlike PUT; a rename cannot come with settings)",
		Tags:    []string{"views"},
	}, func(_ context.Context, in *viewPatchInput) (*viewOutput, error) {
		b := in.Body
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := views.ViewPatch(ss, in.Name, b.Name, views.ViewChange{
			Layout: b.Layout, Query: b.Query, GroupBy: b.GroupBy, KanbanColumns: b.KanbanColumns, Swimlane: b.Swimlane, Columns: b.Columns,
		})
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-view", Method: http.MethodDelete, Path: "/api/views/{name}",
		Summary: "Remove a saved view, as hh view rm", Tags: []string{"views"},
	}, func(_ context.Context, in *ViewParam) (*viewOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		r, err := views.ViewRemove(ss, in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})
}
