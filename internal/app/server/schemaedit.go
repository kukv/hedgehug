package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

type DefParam struct {
	StoreParam
	Name string `path:"name" doc:"The name of the definition"`
}

type ValueParam struct {
	DefParam
	Value string `path:"value" doc:"The value of the enum or multi_enum property"`
}

type editOutput struct{ Body schemas.EditWrite }

type orderBody struct {
	Names []string `json:"names" nullable:"false" doc:"Every name of the list, in the new order; a list that differs from the schema is 409 schema.order_stale"`
}

type statusAddInput struct {
	StoreParam
	Body struct {
		Name     string `json:"name"`
		Label    string `json:"label,omitempty"`
		Category string `json:"category" enum:"todo,doing,done,canceled"`
		Color    string `json:"color,omitempty"`
	}
}

type statusChangeInput struct {
	DefParam
	Body struct {
		Label    *string `json:"label,omitempty" doc:"Empty takes it off"`
		Category *string `json:"category,omitempty" enum:"todo,doing,done,canceled"`
		Color    *string `json:"color,omitempty" doc:"Empty takes it off"`
		Default  bool    `json:"default,omitempty" doc:"true makes it the default of its category; false is not a change"`
	}
}

type kindAddInput struct {
	StoreParam
	Body struct {
		Name     string   `json:"name"`
		Label    string   `json:"label,omitempty"`
		Role     string   `json:"role" enum:"container,work"`
		Parents  []string `json:"parents,omitempty" doc:"The kinds it may sit under, root for the top; leave it out with anywhere"`
		Anywhere bool     `json:"anywhere,omitempty"`
		Icon     string   `json:"icon,omitempty"`
		Template string   `json:"template,omitempty"`
	}
}

type kindChangeInput struct {
	DefParam
	Body struct {
		Label    *string   `json:"label,omitempty"`
		Role     *string   `json:"role,omitempty" enum:"container,work"`
		Parents  *[]string `json:"parents,omitempty" doc:"Not empty; anywhere takes the limit off"`
		Anywhere bool      `json:"anywhere,omitempty"`
		Icon     *string   `json:"icon,omitempty"`
		Template *string   `json:"template,omitempty"`
		Default  bool      `json:"default,omitempty"`
	}
}

type propAddInput struct {
	StoreParam
	Body struct {
		Name      string   `json:"name"`
		Label     string   `json:"label,omitempty"`
		Type      string   `json:"type" enum:"text,number,bool,date,datetime,duration,enum,multi_enum,user,task_ref,url"`
		Values    []string `json:"values,omitempty"`
		Multi     bool     `json:"multi,omitempty"`
		Inverse   string   `json:"inverse,omitempty"`
		Symmetric bool     `json:"symmetric,omitempty"`
	}
}

type propChangeInput struct {
	DefParam
	Body struct {
		Label     *string `json:"label,omitempty"`
		Multi     *bool   `json:"multi,omitempty"`
		Inverse   *string `json:"inverse,omitempty"`
		Symmetric *bool   `json:"symmetric,omitempty"`
	}
}

type valueAddInput struct {
	DefParam
	Body struct {
		Value string `json:"value"`
	}
}

type orderInput struct {
	StoreParam
	Body orderBody
}

type valuesOrderInput struct {
	DefParam
	Body orderBody
}

// parentsOf is the parents a kind is given: nil for anywhere. Both, or an
// empty list, are a bad request.
func parentsOf(parents []string, anywhere bool) ([]string, error) {
	switch {
	case anywhere && parents != nil:
		return nil, errors.New("parents and anywhere cannot be used together")
	case anywhere:
		return nil, nil
	case len(parents) == 0:
		return nil, errors.New("parents is empty; use anywhere to take the limit off")
	}
	return parents, nil
}

func (s *Server) edit(store string, fn func(*schema.Schema) (*schema.Schema, error), use *schemas.Use, check bool) (*editOutput, error) {
	ss := s.session(store)
	defer ss.Close()
	w, err := schemas.Edit(ss, fn, use, check)
	if err != nil {
		return nil, problemOf(err)
	}
	return &editOutput{Body: w}, nil
}

func (s *Server) registerSchemaEdit() {
	reg := func(id, method, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{"schema"}}
	}

	huma.Register(s.api, reg("add-status", http.MethodPost, "/api/schema/statuses", "Add a status at the end"),
		func(_ context.Context, in *statusAddInput) (*editOutput, error) {
			b := in.Body
			st := schema.Status{Name: b.Name, Label: b.Label, Category: schema.Category(b.Category), Color: b.Color}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddStatus(st), nil }, nil, false)
		})
	huma.Register(s.api, reg("change-status", http.MethodPatch, "/api/schema/statuses/{name}", "Change a status"),
		func(_ context.Context, in *statusChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.StatusChange{Label: b.Label, Color: b.Color, Default: b.Default}
			if b.Category != nil {
				c := schema.Category(*b.Category)
				ch.Category = &c
			}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.ChangeStatus(in.Name, ch) }, nil, false)
		})
	huma.Register(s.api, reg("delete-status", http.MethodDelete, "/api/schema/statuses/{name}", "Remove a status no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus(in.Name) },
				&schemas.Use{Target: schema.TargetStatus, Name: in.Name}, false)
		})
	huma.Register(s.api, reg("order-statuses", http.MethodPut, "/api/schema/statuses/order", "Put the statuses in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderStatuses(in.Body.Names) }, nil, false)
		})

	huma.Register(s.api, reg("add-kind", http.MethodPost, "/api/schema/kinds", "Add a kind at the end"),
		func(_ context.Context, in *kindAddInput) (*editOutput, error) {
			b := in.Body
			ps, err := parentsOf(b.Parents, b.Anywhere)
			if err != nil {
				return nil, badRequest(err)
			}
			k := schema.Kind{Name: b.Name, Label: b.Label, Role: schema.Role(b.Role), Parents: ps, Icon: b.Icon, Template: b.Template}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddKind(k), nil }, nil, false)
		})
	huma.Register(s.api, reg("change-kind", http.MethodPatch, "/api/schema/kinds/{name}", "Change a kind"),
		func(_ context.Context, in *kindChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.KindChange{Label: b.Label, Icon: b.Icon, Template: b.Template, Default: b.Default}
			if b.Role != nil {
				r := schema.Role(*b.Role)
				ch.Role = &r
			}
			if b.Parents != nil || b.Anywhere {
				var given []string
				if b.Parents != nil {
					given = *b.Parents
					if given == nil {
						given = []string{}
					}
				}
				ps, err := parentsOf(given, b.Anywhere)
				if err != nil {
					return nil, badRequest(err)
				}
				ch.Parents = &ps
			}
			// role and parents can break the tasks there are
			check := ch.Role != nil || ch.Parents != nil
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.ChangeKind(in.Name, ch) }, nil, check)
		})
	huma.Register(s.api, reg("delete-kind", http.MethodDelete, "/api/schema/kinds/{name}", "Remove a kind no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveKind(in.Name) },
				&schemas.Use{Target: schema.TargetKind, Name: in.Name}, false)
		})
	huma.Register(s.api, reg("order-kinds", http.MethodPut, "/api/schema/kinds/order", "Put the kinds in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderKinds(in.Body.Names) }, nil, false)
		})

	huma.Register(s.api, reg("add-prop", http.MethodPost, "/api/schema/props", "Add a property or a relation at the end"),
		func(_ context.Context, in *propAddInput) (*editOutput, error) {
			b := in.Body
			p := schema.Prop{Name: b.Name, Label: b.Label, Type: schema.PropType(b.Type), Values: b.Values, Multi: b.Multi, Inverse: b.Inverse, Symmetric: b.Symmetric}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddProp(p) }, nil, false)
		})
	huma.Register(s.api, reg("change-prop", http.MethodPatch, "/api/schema/props/{name}", "Change a property or a relation"),
		func(_ context.Context, in *propChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.PropChange{Label: b.Label, Inverse: b.Inverse, Multi: b.Multi, Symmetric: b.Symmetric}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.ChangeProp(in.Name, ch) }, nil, b.Multi != nil)
		})
	huma.Register(s.api, reg("delete-prop", http.MethodDelete, "/api/schema/props/{name}", "Remove a property no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveProp(in.Name) },
				&schemas.Use{Target: schema.TargetProp, Name: in.Name}, false)
		})
	huma.Register(s.api, reg("order-props", http.MethodPut, "/api/schema/props/order", "Put the relations, or the other properties, in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderProps(in.Body.Names) }, nil, false)
		})

	huma.Register(s.api, reg("add-value", http.MethodPost, "/api/schema/props/{name}/values", "Add a value at the end"),
		func(_ context.Context, in *valueAddInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddValue(in.Name, in.Body.Value) }, nil, false)
		})
	huma.Register(s.api, reg("delete-value", http.MethodDelete, "/api/schema/props/{name}/values/{value}", "Remove a value no task uses"),
		func(_ context.Context, in *ValueParam) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveValue(in.Name, in.Value) },
				&schemas.Use{Target: schema.TargetValue, Prop: in.Name, Name: in.Value}, false)
		})
	huma.Register(s.api, reg("order-values", http.MethodPut, "/api/schema/props/{name}/values/order", "Put the values in order"),
		func(_ context.Context, in *valuesOrderInput) (*editOutput, error) {
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderValues(in.Name, in.Body.Names) }, nil, false)
		})
}
