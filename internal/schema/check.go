package schema

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
)

// Problem is a defect in a schema.
type Problem struct {
	Path string // where, like "statuses[2]" or "scoped[0].where"
	Msg  string
}

func (p Problem) String() string { return p.Path + ": " + p.Msg }

// requirableFields are the built-in task fields that workflow.required may
// name, besides properties.
var requirableFields = []string{
	"key_prefix", "parent", "assignees", "watchers", "labels", "repos",
	"blocked_by", "sprint", "releases", "links",
}

// builtinFilled reports whether the built-in field name of t has a value.
// ok is false when name is not a requirable built-in field.
func builtinFilled(t *model.Task, name string) (filled, ok bool) {
	switch name {
	case "key_prefix":
		return t.KeyPrefix != "", true
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
func parseWhere(w string) (model.ID, error) {
	s, ok := strings.CutPrefix(w, "project:")
	if !ok {
		return model.ID{}, errors.New(`where must be "project:<ULID>"`)
	}
	return model.ParseID(s)
}

// Check lists the defects of s. A schema with defects may still be read, so
// that hh doctor and hh schema check can show them.
func (s *Schema) Check() []Problem {
	var ps []Problem
	add := func(path, format string, args ...any) {
		ps = append(ps, Problem{path, fmt.Sprintf(format, args...)})
	}

	statuses := map[string]bool{}
	defaults := map[Category]int{}
	for i, st := range s.Statuses {
		p := fmt.Sprintf("statuses[%d]", i)
		switch {
		case st.Name == "":
			add(p, "name is empty")
		case statuses[st.Name]:
			add(p, "status %q is defined twice", st.Name)
		}
		statuses[st.Name] = true
		if !slices.Contains(Categories, st.Category) {
			add(p, "unknown category %q", st.Category)
		}
		if st.Default {
			defaults[st.Category]++
		}
	}
	for _, c := range Categories {
		if defaults[c] != 1 {
			add("statuses", "category %s needs exactly one default status, has %d", c, defaults[c])
		}
	}

	kinds := map[string]bool{}
	workDefaults := 0
	for i, k := range s.Kinds {
		p := fmt.Sprintf("kinds[%d]", i)
		switch {
		case k.Name == "":
			add(p, "name is empty")
		case kinds[k.Name]:
			add(p, "kind %q is defined twice", k.Name)
		}
		kinds[k.Name] = true
		if k.Role != RoleContainer && k.Role != RoleWork {
			add(p, "unknown role %q", k.Role)
		}
		if k.Default && k.Role == RoleWork {
			workDefaults++
		}
		if k.Template != "" {
			if err := model.ValidTemplateName(k.Template); err != nil {
				add(p+".template", "%v", err)
			}
		}
	}
	for i, k := range s.Kinds {
		for _, parent := range k.Parents {
			if parent != Root && !kinds[parent] {
				add(fmt.Sprintf("kinds[%d].parents", i), "unknown kind %q", parent)
			}
		}
	}
	if workDefaults != 1 {
		add("kinds", "the work role needs exactly one default kind, has %d", workDefaults)
	}

	props := map[string]bool{}
	for _, pr := range s.Props {
		p := "props." + pr.Name
		if slices.Contains(model.TaskKeys(), pr.Name) {
			add(p, "%q is a built-in field", pr.Name)
		}
		if props[pr.Name] {
			add(p, "property %q is defined twice", pr.Name)
		}
		props[pr.Name] = true
		if !slices.Contains(PropTypes, pr.Type) {
			add(p, "unknown type %q", pr.Type)
		}
		if (pr.Type == TypeEnum || pr.Type == TypeMultiEnum) && len(pr.Values) == 0 {
			add(p, "values is empty")
		}
		if pr.Inverse != "" && pr.Symmetric {
			add(p, "inverse and symmetric cannot be used together")
		}
		if (pr.Inverse != "" || pr.Symmetric) && pr.Type != TypeTaskRef {
			add(p, "inverse and symmetric are only for task_ref")
		}
	}

	checkWorkflow := func(path string, w Workflow) {
		for _, tr := range w.Transitions {
			if !statuses[tr.Key] {
				add(path+".transitions", "unknown status %q", tr.Key)
			}
			for _, to := range tr.Values {
				if !statuses[to] {
					add(path+".transitions", "unknown status %q", to)
				}
			}
		}
		for _, rq := range w.Required {
			if !statuses[rq.Key] {
				add(path+".required", "unknown status %q", rq.Key)
			}
			for _, f := range rq.Values {
				if !props[f] && !slices.Contains(requirableFields, f) {
					add(path+".required", "%q is neither a property nor a field that can be required", f)
				}
			}
		}
	}
	checkWorkflow("workflow", s.Workflow)

	wheres := map[model.ID]bool{}
	for i, sc := range s.Scoped {
		p := fmt.Sprintf("scoped[%d]", i)
		id, err := parseWhere(sc.Where)
		switch {
		case err != nil:
			add(p+".where", "%v", err)
		case wheres[id]:
			add(p+".where", "%s has two overrides", sc.Where)
		}
		wheres[id] = true
		for _, name := range sc.Statuses {
			if !statuses[name] {
				add(p+".statuses", "unknown status %q", name)
			}
		}
		if sc.Statuses != nil {
			for _, c := range Categories {
				if d, ok := s.DefaultStatus(c); ok && !slices.Contains(sc.Statuses, d.Name) {
					add(p+".statuses", "%q is the default of %s and cannot be hidden", d.Name, c)
				}
			}
		}
		if sc.Workflow != nil {
			checkWorkflow(p+".workflow", *sc.Workflow)
		}
		if sc.Templates != nil {
			for _, name := range sc.Templates.Only {
				if err := model.ValidTemplateName(name); err != nil {
					add(p+".templates.only", "%v", err)
				}
			}
			keys := make([]string, 0, len(sc.Templates.Default))
			for k := range sc.Templates.Default {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if !kinds[k] {
					add(p+".templates.default", "unknown kind %q", k)
				}
				if err := model.ValidTemplateName(sc.Templates.Default[k]); err != nil {
					add(p+".templates.default", "%v", err)
				}
			}
		}
	}
	return ps
}
