package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
)

type ViewParam struct {
	StoreParam
	Name string `path:"name" doc:"The name of the view"`
}

type viewWrite struct {
	View   ops.SavedView `json:"view"`
	Commit *string       `json:"commit"`
}

type viewOutput struct{ Body viewWrite }

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

type viewRenameInput struct {
	ViewParam
	Body struct {
		Name string `json:"name" doc:"The new name"`
	}
}

func viewOut(r ops.ViewResult) *viewOutput {
	out := &viewOutput{Body: viewWrite{Commit: commitPtr(r.Commit)}}
	if r.View != nil {
		out.Body.View = ops.SavedViewOf(r.View)
	}
	return out
}

func (s *Server) registerViews() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-views", Method: http.MethodGet, Path: "/api/views",
		Summary: "List the saved views, as hh view list", Tags: []string{"views"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []ops.SavedView }, error) {
		vs, err := s.session(in.Store).ViewList()
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []ops.SavedView }{vs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-view", Method: http.MethodGet, Path: "/api/views/{name}",
		Summary: "Show a saved view, as hh view show", Tags: []string{"views"},
	}, func(_ context.Context, in *ViewParam) (*struct{ Body ops.SavedView }, error) {
		v, err := s.session(in.Store).ViewGet(in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body ops.SavedView }{ops.SavedViewOf(v)}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "save-view", Method: http.MethodPut, Path: "/api/views/{name}",
		Summary: "Save a view; what is not sent stays as saved, as hh view save", Tags: []string{"views"},
	}, func(_ context.Context, in *viewSaveInput) (*viewOutput, error) {
		b := in.Body
		r, err := s.session(in.Store).ViewSave(in.Name, ops.ViewChange{
			Layout: b.Layout, Query: b.Query, GroupBy: b.GroupBy, KanbanColumns: b.KanbanColumns, Swimlane: b.Swimlane, Columns: b.Columns,
		})
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "rename-view", Method: http.MethodPatch, Path: "/api/views/{name}",
		Summary: "Rename a saved view, as hh view rename", Tags: []string{"views"},
	}, func(_ context.Context, in *viewRenameInput) (*viewOutput, error) {
		r, err := s.session(in.Store).ViewRename(in.Name, in.Body.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-view", Method: http.MethodDelete, Path: "/api/views/{name}",
		Summary: "Remove a saved view, as hh view rm", Tags: []string{"views"},
	}, func(_ context.Context, in *ViewParam) (*viewOutput, error) {
		r, err := s.session(in.Store).ViewRemove(in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return viewOut(r), nil
	})
}
