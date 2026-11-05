package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/usecase/templates"
)

// TemplateParam names a template. It is exported because huma skips an
// embedded struct whose type is not.
type TemplateParam struct {
	StoreParam
	Name string `path:"name" doc:"The name of the template"`
}

type templatePutInput struct {
	TemplateParam
	Body struct {
		Content string `json:"content" doc:"The file: front matter and body"`
	}
}

type templateOutput struct{ Body templates.TemplateWrite }

func (s *Server) registerTemplates() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-templates", Method: http.MethodGet, Path: "/api/templates",
		Summary: "List the templates, as hh template ls", Tags: []string{"templates"},
	}, func(_ context.Context, in *StoreParam) (*struct{ Body []templates.TemplateView }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		vs, err := templates.List(ss)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body []templates.TemplateView }{vs}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "get-template", Method: http.MethodGet, Path: "/api/templates/{name}",
		Summary: "Read the file of a template, or the skeleton of a new one", Tags: []string{"templates"},
	}, func(_ context.Context, in *TemplateParam) (*struct{ Body templates.TemplateFile }, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		f, err := templates.Read(ss, in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return &struct{ Body templates.TemplateFile }{f}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "put-template", Method: http.MethodPut, Path: "/api/templates/{name}",
		Summary: "Write a template, as hh template edit", Tags: []string{"templates"},
	}, func(_ context.Context, in *templatePutInput) (*templateOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := templates.Put(ss, in.Name, in.Body.Content)
		if err != nil {
			return nil, problemOf(err)
		}
		return &templateOutput{w}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "remove-template", Method: http.MethodDelete, Path: "/api/templates/{name}",
		Summary: "Delete a template, as hh template rm", Tags: []string{"templates"},
	}, func(_ context.Context, in *TemplateParam) (*templateOutput, error) {
		ss := s.session(in.Store)
		defer ss.Close()
		w, err := templates.Remove(ss, in.Name)
		if err != nil {
			return nil, problemOf(err)
		}
		return &templateOutput{w}, nil
	})
}
