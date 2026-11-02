package server

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
)

// ScopedParam names the project of an override. It is exported because huma
// skips an embedded struct whose type is not.
type ScopedParam struct {
	StoreParam
	Project string `path:"project" doc:"The project: its ID or ULID"`
}

type scopedPutInput struct {
	ScopedParam
	Body struct {
		Statuses    *[]string            `json:"statuses,omitempty" doc:"The statuses to show; the default of each category has to be in it. Leave it out to keep what the override has; an empty list takes it off, so the store's statuses show"`
		Transitions *map[string][]string `json:"transitions,omitempty" doc:"Status to the statuses it may move to, on top of the store's. Leave it out to keep what the override has; an empty object takes it off"`
		Required    *map[string][]string `json:"required,omitempty" doc:"Status to the fields needed to enter it, on top of the store's. Leave it out to keep what the override has; an empty object takes it off"`
	}
}

type scopedOutput struct{ Body ops.ScopedView }

// listsOf is m as schema.Lists, in the order of the keys.
func listsOf(m *map[string][]string) *schema.Lists {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(*m))
	for k := range *m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	l := make(schema.Lists, 0, len(keys))
	for _, k := range keys {
		vals := (*m)[k]
		if vals == nil {
			vals = []string{}
		}
		l = append(l, schema.List{Key: k, Values: vals})
	}
	return &l
}

func (s *Server) registerScoped() {
	huma.Register(s.api, huma.Operation{
		OperationID: "put-scoped", Method: http.MethodPut, Path: "/api/schema/scoped/{project}",
		Summary: "Change the override of a project, as hh schema scope set", Tags: []string{"schema"},
	}, func(_ context.Context, in *scopedPutInput) (*scopedOutput, error) {
		b := in.Body
		if b.Statuses == nil && b.Transitions == nil && b.Required == nil {
			return nil, badRequest(errors.New("give statuses, transitions or required"))
		}
		r, err := s.session(in.Store).ScopedSet(in.Project, schema.ScopedChange{
			Statuses: b.Statuses, Transitions: listsOf(b.Transitions), Required: listsOf(b.Required),
		})
		if err != nil {
			return nil, problemOf(err)
		}
		return &scopedOutput{r.View()}, nil
	})

	huma.Register(s.api, huma.Operation{
		OperationID: "delete-scoped", Method: http.MethodDelete, Path: "/api/schema/scoped/{project}",
		Summary: "Remove the override of a project, as hh schema scope rm", Tags: []string{"schema"},
	}, func(_ context.Context, in *ScopedParam) (*scopedOutput, error) {
		r, err := s.session(in.Store).ScopedRemove(in.Project)
		if err != nil {
			return nil, problemOf(err)
		}
		return &scopedOutput{r.View()}, nil
	})
}
