package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

type DefParam struct {
	StoreParam
	Name string `path:"name" doc:"The name of the definition"`
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
		Name     *string `json:"name,omitempty" doc:"The new name; the definition, what refers to it and the tasks that use it are rewritten in the same commit. A name already defined is 409 schema.name_taken"`
		Label    *string `json:"label,omitempty" doc:"Empty takes it off"`
		Category *string `json:"category,omitempty" enum:"todo,doing,done,canceled"`
		Color    *string `json:"color,omitempty" doc:"Empty takes it off"`
		Default  bool    `json:"default,omitempty" doc:"true makes it the default of its category; false is not a change"`
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

// valueEditBody is one value of an enum or multi_enum property after the edit.
type valueEditBody struct {
	Name string   `json:"name"`
	From []string `json:"from,omitempty" doc:"The current values it comes from: none for a new value, one for a value kept or renamed, several for a merge"`
}

type propChangeInput struct {
	DefParam
	Body struct {
		Name      *string          `json:"name,omitempty" doc:"The new name, as for a status"`
		Label     *string          `json:"label,omitempty"`
		Multi     *bool            `json:"multi,omitempty"`
		Inverse   *string          `json:"inverse,omitempty"`
		Symmetric *bool            `json:"symmetric,omitempty"`
		Values    *[]valueEditBody `json:"values,omitempty" doc:"All the values after the edit, in order, of an enum or multi_enum property. A current value that no from names is removed: 422 schema.in_use when a task uses it"`
	}
}

type orderInput struct {
	StoreParam
	Body orderBody
}

type templateDefaultsInput struct {
	StoreParam
	Body struct {
		Default map[string]string `json:"default" nullable:"false" doc:"Kind to template name, all of them after the edit; empty takes templates.default off. A kind that does not exist or a bad template name is 422 schema.invalid"`
	}
}

// plain is a change that carries no migrations.
func plain(fn func(*schema.Schema) (*schema.Schema, error)) func(*schema.Schema) (*schema.Schema, []schema.Migration, error) {
	return func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		next, err := fn(sc)
		return next, nil, err
	}
}

func (s *Server) edit(store string, fn func(*schema.Schema) (*schema.Schema, []schema.Migration, error), opts schemas.EditOpts) (*editOutput, error) {
	ss := s.session(store)
	defer ss.Close()
	w, err := schemas.Edit(ss, fn, opts)
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
			return s.edit(in.Store, plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddStatus(st), nil }), schemas.EditOpts{})
		})
	huma.Register(s.api, reg("change-status", http.MethodPatch, "/api/schema/statuses/{name}", "Change a status"),
		func(_ context.Context, in *statusChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.StatusChange{Name: b.Name, Label: b.Label, Color: b.Color, Default: b.Default}
			if b.Category != nil {
				c := schema.Category(*b.Category)
				ch.Category = &c
			}
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
				return sc.ChangeStatus(in.Name, ch)
			}, schemas.EditOpts{})
		})
	huma.Register(s.api, reg("delete-status", http.MethodDelete, "/api/schema/statuses/{name}", "Remove a status no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveStatus(in.Name) }),
				schemas.EditOpts{Use: []schemas.Use{{Target: schema.TargetStatus, Name: in.Name}}})
		})
	huma.Register(s.api, reg("order-statuses", http.MethodPut, "/api/schema/statuses/order", "Put the statuses in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderStatuses(in.Body.Names) }), schemas.EditOpts{})
		})

	huma.Register(s.api, reg("set-template-defaults", http.MethodPut, "/api/schema/templates/default", "Set the default template of each kind"),
		func(_ context.Context, in *templateDefaultsInput) (*editOutput, error) {
			return s.edit(in.Store, plain(func(sc *schema.Schema) (*schema.Schema, error) {
				return sc.SetTemplateDefaults(in.Body.Default), nil
			}), schemas.EditOpts{})
		})

	huma.Register(s.api, reg("add-prop", http.MethodPost, "/api/schema/props", "Add a property or a relation at the end"),
		func(_ context.Context, in *propAddInput) (*editOutput, error) {
			b := in.Body
			p := schema.Prop{Name: b.Name, Label: b.Label, Type: schema.PropType(b.Type), Values: b.Values, Multi: b.Multi, Inverse: b.Inverse, Symmetric: b.Symmetric}
			return s.edit(in.Store, plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.AddProp(p) }), schemas.EditOpts{})
		})
	huma.Register(s.api, reg("change-prop", http.MethodPatch, "/api/schema/props/{name}", "Change a property or a relation"),
		func(_ context.Context, in *propChangeInput) (*editOutput, error) {
			b := in.Body
			ch := schema.PropChange{Name: b.Name, Label: b.Label, Inverse: b.Inverse, Multi: b.Multi, Symmetric: b.Symmetric}
			if b.Values != nil {
				vs := make([]schema.ValueEdit, len(*b.Values))
				for i, v := range *b.Values {
					vs[i] = schema.ValueEdit{Name: v.Name, From: v.From}
				}
				ch.Values = &vs
			}
			// The values that no edit keeps are known once the change is made,
			// and a task that uses one refuses the change.
			var removed []string
			return s.edit(in.Store, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
				next, ms, r, err := sc.ChangeProp(in.Name, ch)
				removed = r
				return next, ms, err
			}, schemas.EditOpts{Check: b.Multi != nil, UseAfter: func() []schemas.Use {
				us := make([]schemas.Use, len(removed))
				for i, v := range removed {
					us[i] = schemas.Use{Target: schema.TargetValue, Prop: in.Name, Name: v}
				}
				return us
			}})
		})
	huma.Register(s.api, reg("delete-prop", http.MethodDelete, "/api/schema/props/{name}", "Remove a property no task uses"),
		func(_ context.Context, in *DefParam) (*editOutput, error) {
			return s.edit(in.Store, plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.RemoveProp(in.Name) }),
				schemas.EditOpts{Use: []schemas.Use{{Target: schema.TargetProp, Name: in.Name}}})
		})
	huma.Register(s.api, reg("order-props", http.MethodPut, "/api/schema/props/order", "Put the relations, or the other properties, in order"),
		func(_ context.Context, in *orderInput) (*editOutput, error) {
			return s.edit(in.Store, plain(func(sc *schema.Schema) (*schema.Schema, error) { return sc.OrderProps(in.Body.Names) }), schemas.EditOpts{})
		})
}
