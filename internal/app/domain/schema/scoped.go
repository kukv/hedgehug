package schema

import (
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// ScopedChange is a change to the override of one project. A nil part is
// left as it is; a part that is given but empty is taken off, so the project
// has the store's value again.
type ScopedChange struct {
	Statuses    *[]string
	Transitions *Lists
	Required    *Lists
	// TemplateOnly and TemplateDefault replace the templates the project uses
	// and its default of each kind; empty takes them off.
	TemplateOnly    *[]string
	TemplateDefault *map[string]string
}

// ScopedFor returns the override of project.
func (s *Schema) ScopedFor(project model.ULID) (Scoped, bool) {
	if i := s.scopedIndex(project); i >= 0 {
		return s.Scoped[i], true
	}
	return Scoped{}, false
}

func (s *Schema) scopedIndex(project model.ULID) int {
	return slices.IndexFunc(s.Scoped, func(sc Scoped) bool {
		w, err := parseWhere(sc.Where)
		return err == nil && w == project
	})
}

// WithScoped returns a copy of s whose override of project is changed by ch.
// An override left with no statuses, no workflow and no templates is dropped.
func (s *Schema) WithScoped(project model.ULID, ch ScopedChange) *Schema {
	out := s.clone()
	i := out.scopedIndex(project)
	sc := Scoped{Where: "project:" + project.String()}
	if i >= 0 {
		sc = out.Scoped[i]
	}
	if ch.Statuses != nil {
		sc.Statuses = nil
		if len(*ch.Statuses) > 0 {
			sc.Statuses = slices.Clone(*ch.Statuses)
		}
	}
	if ch.Transitions != nil || ch.Required != nil {
		var w Workflow
		if sc.Workflow != nil {
			w = *sc.Workflow
		}
		if ch.Transitions != nil {
			w.Transitions = nilIfEmpty(ch.Transitions.clone())
		}
		if ch.Required != nil {
			w.Required = nilIfEmpty(ch.Required.clone())
		}
		sc.Workflow = &w
		if w.IsZero() {
			sc.Workflow = nil
		}
	}
	if ch.TemplateOnly != nil || ch.TemplateDefault != nil {
		var tm Templates
		if sc.Templates != nil {
			tm = *sc.Templates
		}
		if ch.TemplateOnly != nil {
			tm.Only = nil
			if len(*ch.TemplateOnly) > 0 {
				tm.Only = slices.Clone(*ch.TemplateOnly)
			}
		}
		if ch.TemplateDefault != nil {
			tm.Default = nil
			if len(*ch.TemplateDefault) > 0 {
				tm.Default = maps.Clone(*ch.TemplateDefault)
			}
		}
		sc.Templates = &tm
		if len(tm.Only) == 0 && len(tm.Default) == 0 {
			sc.Templates = nil
		}
	}
	switch empty := sc.Statuses == nil && sc.Workflow == nil && sc.Templates == nil; {
	case i >= 0 && empty:
		out.Scoped = slices.Delete(out.Scoped, i, i+1)
	case i >= 0:
		out.Scoped[i] = sc
	case !empty:
		out.Scoped = append(out.Scoped, sc)
	}
	return out
}

// WithoutScoped returns a copy of s without the override of project.
func (s *Schema) WithoutScoped(project model.ULID) *Schema {
	out := s.clone()
	if i := out.scopedIndex(project); i >= 0 {
		out.Scoped = slices.Delete(out.Scoped, i, i+1)
	}
	return out
}

func nilIfEmpty(l Lists) Lists {
	if len(l) == 0 {
		return nil
	}
	return l
}

// NewProblems lists the problems of s that are not in before.
func (s *Schema) NewProblems(before []Problem) []Problem {
	var out []Problem
	for _, p := range s.Check() {
		if !slices.ContainsFunc(before, func(b Problem) bool { return b.Path == p.Path && b.Msg == p.Msg }) {
			out = append(out, p)
		}
	}
	return out
}
