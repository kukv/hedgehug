// Package automations lists and edits the rules of automations.yaml.
package automations

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// RuleView is a rule as hh automation list prints it with --json. Enabled is
// the rule's own switch; with a project given to List it also takes the
// project's scoped disable into account.
type RuleView struct {
	ID         string        `json:"id"`
	On         string        `json:"on"`
	When       string        `json:"when,omitempty"`
	Status     string        `json:"status,omitempty"`
	Category   string        `json:"category,omitempty"`
	Project    *usecase.Ref  `json:"project,omitempty"`
	Enabled    bool          `json:"enabled"`
	DisabledIn []usecase.Ref `json:"disabled_in" nullable:"false"`
}

// AddInput is a rule to add. Project is a project of the store; empty means
// the rule is the store's. Status or Category, one of them, says what it sets.
type AddInput struct{ ID, On, Status, Category, When, Project string }

// FieldsView is issue_fields as hh automation fields prints it with --json.
type FieldsView struct {
	DefaultSync string            `json:"default_sync"`
	Push        []string          `json:"push" nullable:"false"`
	Mirror      []string          `json:"mirror" nullable:"false"`
	Props       map[string]string `json:"props" nullable:"false"`
}

// FieldsInput is a change to issue_fields; a nil part is left as it is, and a
// key of Props with an empty value is taken off.
type FieldsInput struct {
	DefaultSync *string
	Push        *[]string
	Mirror      *[]string
	Props       map[string]string
}

// List reads the rules in the order of the file. With project, only those
// that apply there are listed (the store's rules and those of the project and
// the projects above it), with the project's scoped disable in Enabled.
func List(ss *usecase.Session, project string) ([]RuleView, error) {
	s, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	f, err := s.Automations()
	if err != nil {
		return nil, invalid(err)
	}
	var ancestors []model.ULID
	if project != "" {
		e, err := projectEntry(ss, ix, project)
		if err != nil {
			return nil, err
		}
		ancestors = ix.Ancestors(e)
	}
	within := func(where string) bool {
		u, ok := whereULID(where)
		return ok && slices.Contains(ancestors, u)
	}
	var disabled []string
	for _, sc := range f.Scoped {
		if within(sc.Where) {
			disabled = append(disabled, sc.Disable...)
		}
	}
	// the rules of the store first, then those of projects, as they apply
	var storeWide, scoped []RuleView
	for _, r := range f.Rules {
		v := RuleView{
			ID: r.ID, On: r.On, When: r.When, Status: r.Set.Status, Category: r.Set.Category,
			Enabled: r.Enabled, DisabledIn: []usecase.Ref{},
		}
		for _, sc := range f.Scoped {
			if u, ok := whereULID(sc.Where); ok && slices.Contains(sc.Disable, r.ID) {
				v.DisabledIn = append(v.DisabledIn, usecase.RefOf(ix, u))
			}
		}
		if r.Where != "" {
			if u, ok := whereULID(r.Where); ok {
				ref := usecase.RefOf(ix, u)
				v.Project = &ref
			}
		}
		if project != "" {
			if r.Where != "" && !within(r.Where) {
				continue
			}
			v.Enabled = v.Enabled && !slices.Contains(disabled, r.ID)
		}
		if r.Where == "" || project == "" {
			storeWide = append(storeWide, v)
		} else {
			scoped = append(scoped, v)
		}
	}
	out := make([]RuleView, 0, len(storeWide)+len(scoped))
	return append(append(out, storeWide...), scoped...), nil
}

// Add appends a rule, in one commit (op automation). An id that is taken is
// automation.exists; a rule with a defect Check finds is automation.invalid.
// Nothing is written then.
func Add(ss *usecase.Session, in AddInput) error {
	return edit(ss, "add rule "+in.ID, func(ix *index.Index, f *automation.File) error {
		r := automation.Rule{
			ID: in.ID, On: in.On, When: in.When, Enabled: true,
			Set: automation.Set{Status: in.Status, Category: in.Category},
		}
		if in.Project != "" {
			e, err := projectEntry(ss, ix, in.Project)
			if err != nil {
				return err
			}
			r.Where = automation.WherePrefix + e.ULID.String()
		}
		return f.AddRule(r)
	})
}

// Remove removes the rule, and its id from every scoped disable. An id that
// is not there is automation.not_found.
func Remove(ss *usecase.Session, id string) error {
	return edit(ss, "rm rule "+id, func(_ *index.Index, f *automation.File) error {
		return f.RemoveRule(id)
	})
}

// SetEnabled turns a rule on or off. With project it is the project's scoped
// disable that changes, not the rule's own switch.
func SetEnabled(ss *usecase.Session, id, project string, on bool) error {
	verb := "disable"
	if on {
		verb = "enable"
	}
	return edit(ss, verb+" rule "+id, func(ix *index.Index, f *automation.File) error {
		if _, ok := f.Rule(id); !ok {
			return fmt.Errorf("%q: %w", id, automation.ErrNotFound)
		}
		if project == "" {
			return f.SetEnabled(id, on)
		}
		e, err := projectEntry(ss, ix, project)
		if err != nil {
			return err
		}
		return f.SetDisabledIn(automation.WherePrefix+e.ULID.String(), id, !on)
	})
}

// Fields reads issue_fields.
func Fields(ss *usecase.Session) (FieldsView, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return FieldsView{}, err
	}
	f, err := s.Automations()
	if err != nil {
		return FieldsView{}, invalid(err)
	}
	i := f.IssueFields
	v := FieldsView{DefaultSync: i.DefaultSync, Push: usecase.OrEmpty(i.Push), Mirror: usecase.OrEmpty(i.Mirror), Props: i.Props}
	if v.Props == nil {
		v.Props = map[string]string{}
	}
	return v, nil
}

// SetFields changes issue_fields, in one commit. A value Check would report
// is automation.invalid.
func SetFields(ss *usecase.Session, in FieldsInput) error {
	return edit(ss, "fields", func(_ *index.Index, f *automation.File) error {
		return f.SetIssueFields(automation.IssueFieldsChange(in))
	})
}

// edit applies change to automations.yaml as the store has it under its
// lock, and writes the result in one commit of op automation. Only the
// problems that the change brings are refused.
func edit(ss *usecase.Session, detail string, change func(*index.Index, *automation.File) error) error {
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return err
	}
	_, err = ss.UpdateCommit(s, store.Op{Name: "automation", Detail: detail}, func(tx *store.Tx) error {
		cur, err := s.Automations()
		if err != nil {
			return invalid(err)
		}
		before, err := cur.Encode()
		if err != nil {
			return err
		}
		sc := s.Schema()
		known := problems(cur, sc, ix)
		next, err := s.Automations()
		if err != nil {
			return invalid(err)
		}
		if err := change(ix, next); err != nil {
			return mapErr(err)
		}
		var fresh []string
		for _, p := range automation.Fresh(known, problems(next, sc, ix)) {
			fresh = append(fresh, p.String())
		}
		if len(fresh) > 0 {
			return usecase.NewErr(usecase.KindInvalid, "automation.invalid",
				fmt.Errorf("the change breaks %s: %s", model.AutomationsPath, strings.Join(fresh, "; ")), "problems", fresh)
		}
		after, err := next.Encode()
		if err != nil {
			return err
		}
		if bytes.Equal(before, after) {
			return nil
		}
		return tx.PutFile(model.AutomationsPath, after)
	})
	return err
}

// problems are the defects Check finds in f.
func problems(f *automation.File, sc *schema.Schema, ix *index.Index) []schema.Problem {
	isProject := func(u model.ULID) bool {
		e, ok := ix.Task(u)
		return ok && e.DeletedAt == nil && e.Kind == usecase.ProjectKind
	}
	return f.Check(sc, isProject)
}

// projectEntry resolves the project arg names. The task has to be a project
// and not deleted.
func projectEntry(ss *usecase.Session, ix *index.Index, arg string) (*index.Entry, error) {
	e, err := ss.ResolveRef(ix, arg)
	if err != nil {
		return nil, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return nil, err
	}
	if e.Kind != usecase.ProjectKind {
		id := ix.DisplayID(e)
		return nil, usecase.NewErr(usecase.KindInvalid, "automation.not_project",
			fmt.Errorf("%s is a %s, not a project", id, e.Kind), "id", id, "kind", e.Kind)
	}
	return e, nil
}

func whereULID(where string) (model.ULID, bool) {
	s, ok := strings.CutPrefix(where, automation.WherePrefix)
	if !ok {
		return model.ULID{}, false
	}
	u, err := model.ParseULID(s)
	return u, err == nil
}

// mapErr gives the errors of a File their codes. An error that has a code
// already is kept.
func mapErr(err error) error {
	var ue *usecase.Error
	switch {
	case errors.As(err, &ue):
		return err
	case errors.Is(err, automation.ErrExists):
		return usecase.NewErr(usecase.KindConflict, "automation.exists", err)
	case errors.Is(err, automation.ErrNotFound):
		return usecase.NewErr(usecase.KindNotFound, "automation.not_found", err)
	}
	return usecase.NewErr(usecase.KindInvalid, "automation.invalid", err)
}

// invalid is the error of an automations.yaml that does not decode.
func invalid(err error) error {
	return usecase.NewErr(usecase.KindInvalid, "automation.invalid", err)
}
