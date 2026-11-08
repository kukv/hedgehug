package schema

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// Problem is a defect in a schema. Code and Args say it for a reader in
// another language: Code names the kind of defect, Args are the values the
// message names, in the order it names them.
type Problem struct {
	Path string // where, like "statuses[2]" or "scoped[0].where"
	Msg  string
	Code string
	Args []string
}

// NewProblem is the Problem at path of the kind code, whose message is format
// with args.
func NewProblem(path, code, format string, args ...any) Problem {
	s := make([]string, len(args))
	for i, a := range args {
		s[i] = fmt.Sprint(a)
	}
	return Problem{Path: path, Msg: fmt.Sprintf(format, args...), Code: code, Args: s}
}

func (p Problem) String() string { return p.Path + ": " + p.Msg }

// requirableFields are the built-in task fields that workflow.required may
// name, besides properties.
var requirableFields = []string{
	"parent", "assignees", "watchers", "labels", "repos",
	"blocked_by", "sprint", "releases", "links",
}

// builtinFilled reports whether the built-in field name of t has a value.
// ok is false when name is not a requirable built-in field.
func builtinFilled(t *model.Task, name string) (filled, ok bool) {
	switch name {
	case "parent":
		return t.Parent != nil, true
	case "assignees":
		return len(t.Assignees) > 0, true
	case "watchers":
		return len(t.Watchers) > 0, true
	case "labels":
		return len(t.Labels) > 0, true
	case "repos":
		return len(t.Repos) > 0, true
	case "blocked_by":
		return len(t.BlockedBy) > 0, true
	case "sprint":
		return t.Sprint != nil, true
	case "releases":
		return len(t.Releases) > 0, true
	case "links":
		return len(t.Links) > 0, true
	}
	return false, false
}

// parseWhere reads the target of a scoped override: project:<ULID>.
func parseWhere(w string) (model.ULID, error) {
	s, ok := strings.CutPrefix(w, "project:")
	if !ok {
		return model.ULID{}, errors.New(`where must be "project:<ULID>"`)
	}
	return model.ParseULID(s)
}

// Check lists the defects of s. A schema with defects may still be read, so
// that hh doctor and hh schema check can show them.
func (s *Schema) Check() []Problem {
	var ps []Problem
	add := func(path, code, format string, args ...any) {
		ps = append(ps, NewProblem(path, code, format, args...))
	}

	statuses := map[string]bool{}
	defaults := map[Category]int{}
	for i, st := range s.Statuses {
		p := fmt.Sprintf("statuses[%d]", i)
		switch {
		case st.Name == "":
			add(p, "schema.name_empty", "name is empty")
		case statuses[st.Name]:
			add(p, "schema.status_twice", "status %q is defined twice", st.Name)
		}
		statuses[st.Name] = true
		if !slices.Contains(Categories, st.Category) {
			add(p, "schema.unknown_category", "unknown category %q", st.Category)
		}
		if st.Default {
			defaults[st.Category]++
		}
	}
	for _, c := range Categories {
		if defaults[c] != 1 {
			add("statuses", "schema.default_status", "category %s needs exactly one default status, has %d", c, defaults[c])
		}
	}

	props := map[string]bool{}
	for _, pr := range s.Props {
		p := "props." + pr.Name
		if slices.Contains(model.TaskKeys(), pr.Name) {
			add(p, "schema.prop_builtin", "%q is a built-in field", pr.Name)
		}
		if props[pr.Name] {
			add(p, "schema.prop_twice", "property %q is defined twice", pr.Name)
		}
		props[pr.Name] = true
		if !slices.Contains(PropTypes, pr.Type) {
			add(p, "schema.unknown_type", "unknown type %q", pr.Type)
		}
		if (pr.Type == TypeEnum || pr.Type == TypeMultiEnum) && len(pr.Values) == 0 {
			add(p, "schema.values_empty", "values is empty")
		}
		if pr.Inverse != "" && pr.Symmetric {
			add(p, "schema.inverse_and_symmetric", "inverse and symmetric cannot be used together")
		}
		if (pr.Inverse != "" || pr.Symmetric) && pr.Type != TypeTaskRef {
			add(p, "schema.inverse_not_task_ref", "inverse and symmetric are only for task_ref")
		}
	}

	checkTemplates := func(path string, t *Templates) {
		if t == nil {
			return
		}
		if path == "templates" && len(t.Only) > 0 {
			// Only narrows the templates of a project; on the store it was
			// never in effect.
			add(path+".only", "schema.templates_only_on_store", "only is for the override of a project (scoped[].templates.only); on the store it has no effect, so remove it")
		}
		for _, name := range t.Only {
			if err := model.ValidTemplateName(name); err != nil {
				add(path+".only", "schema.bad_template_name", "%v", err)
			}
		}
		for _, k := range slices.Sorted(maps.Keys(t.Default)) {
			if _, ok := KindOf(k); !ok {
				add(path+".default", "schema.unknown_kind", "unknown kind %q", k)
			}
			if err := model.ValidTemplateName(t.Default[k]); err != nil {
				add(path+".default", "schema.bad_template_name", "%v", err)
			}
		}
	}

	checkWorkflow := func(path string, w Workflow) {
		for _, tr := range w.Transitions {
			if !statuses[tr.Key] {
				add(path+".transitions", "schema.unknown_status", "unknown status %q", tr.Key)
			}
			for _, to := range tr.Values {
				if !statuses[to] {
					add(path+".transitions", "schema.unknown_status", "unknown status %q", to)
				}
			}
		}
		for _, rq := range w.Required {
			if !statuses[rq.Key] {
				add(path+".required", "schema.unknown_status", "unknown status %q", rq.Key)
			}
			for _, f := range rq.Values {
				if !props[f] && !slices.Contains(requirableFields, f) {
					add(path+".required", "schema.not_requirable", "%q is neither a property nor a field that can be required", f)
				}
			}
		}
	}
	checkWorkflow("workflow", s.Workflow)

	wheres := map[model.ULID]bool{}
	for i, sc := range s.Scoped {
		p := fmt.Sprintf("scoped[%d]", i)
		ulid, err := parseWhere(sc.Where)
		switch {
		case err != nil:
			add(p+".where", "schema.bad_where", "%v", err)
		case wheres[ulid]:
			add(p+".where", "schema.where_twice", "%s has two overrides", sc.Where)
		}
		wheres[ulid] = true
		for _, name := range sc.Statuses {
			if !statuses[name] {
				add(p+".statuses", "schema.unknown_status", "unknown status %q", name)
			}
		}
		if sc.Statuses != nil {
			for _, c := range Categories {
				if d, ok := s.DefaultStatus(c); ok && !slices.Contains(sc.Statuses, d.Name) {
					add(p+".statuses", "schema.default_hidden", "%q is the default of %s and cannot be hidden", d.Name, c)
				}
			}
		}
		if sc.Workflow != nil {
			checkWorkflow(p+".workflow", *sc.Workflow)
		}
		checkTemplates(p+".templates", sc.Templates)
	}
	checkTemplates("templates", s.Templates)
	return ps
}
