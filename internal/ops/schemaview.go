package ops

import "github.com/kukv/hedgehug/internal/schema"

// SchemaView is schema.yaml as GET /api/schema returns it: the keys of the
// file, with props as a mapping of name to definition.
type SchemaView struct {
	Statuses []StatusDef        `json:"statuses"`
	Kinds    []KindDef          `json:"kinds"`
	Props    map[string]PropDef `json:"props"`
	Workflow WorkflowDef        `json:"workflow"`
	Scoped   []ScopedDef        `json:"scoped"`
}

// StatusDef is a status of the schema.
type StatusDef struct {
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	Category string `json:"category"`
	Color    string `json:"color,omitempty"`
	Default  bool   `json:"default,omitempty"`
}

// KindDef is a kind of the schema; Parents nil means anywhere.
type KindDef struct {
	Name     string   `json:"name"`
	Label    string   `json:"label,omitempty"`
	Role     string   `json:"role"`
	Parents  []string `json:"parents"`
	Icon     string   `json:"icon,omitempty"`
	Template string   `json:"template,omitempty"`
	Default  bool     `json:"default,omitempty"`
}

// PropDef is a property of the schema.
type PropDef struct {
	Type      string   `json:"type"`
	Values    []string `json:"values,omitempty"`
	Multi     bool     `json:"multi,omitempty"`
	Label     string   `json:"label,omitempty"`
	Inverse   string   `json:"inverse,omitempty"`
	Symmetric bool     `json:"symmetric,omitempty"`
}

// WorkflowDef is the workflow: status → statuses it may move to, and
// status → fields needed to enter it.
type WorkflowDef struct {
	Transitions map[string][]string `json:"transitions"`
	Required    map[string][]string `json:"required"`
}

// TemplatesDef is the templates of a scoped override, as the keys of
// schema.yaml name them.
type TemplatesDef struct {
	Only    []string          `json:"only,omitempty"`
	Default map[string]string `json:"default,omitempty"`
}

// ScopedDef is an override of a project.
type ScopedDef struct {
	Where     string        `json:"where"`
	Statuses  []string      `json:"statuses,omitempty"`
	Workflow  *WorkflowDef  `json:"workflow,omitempty"`
	Templates *TemplatesDef `json:"templates,omitempty"`
}

// SchemaViewOf is sc as SchemaView.
func SchemaViewOf(sc *schema.Schema) SchemaView {
	v := SchemaView{
		Statuses: []StatusDef{}, Kinds: []KindDef{}, Props: map[string]PropDef{},
		Workflow: workflowDefOf(sc.Workflow), Scoped: []ScopedDef{},
	}
	for _, st := range sc.Statuses {
		v.Statuses = append(v.Statuses, StatusDef{st.Name, st.Label, string(st.Category), st.Color, st.Default})
	}
	for _, k := range sc.Kinds {
		v.Kinds = append(v.Kinds, KindDef{k.Name, k.Label, string(k.Role), k.Parents, k.Icon, k.Template, k.Default})
	}
	for _, p := range sc.Props {
		v.Props[p.Name] = PropDef{string(p.Type), p.Values, p.Multi, p.Label, p.Inverse, p.Symmetric}
	}
	for _, s := range sc.Scoped {
		d := ScopedDef{Where: s.Where, Statuses: s.Statuses}
		if s.Workflow != nil {
			w := workflowDefOf(*s.Workflow)
			d.Workflow = &w
		}
		if s.Templates != nil {
			d.Templates = &TemplatesDef{Only: s.Templates.Only, Default: s.Templates.Default}
		}
		v.Scoped = append(v.Scoped, d)
	}
	return v
}

func workflowDefOf(w schema.Workflow) WorkflowDef {
	return WorkflowDef{Transitions: listsOf(w.Transitions), Required: listsOf(w.Required)}
}

// listsOf is l as a mapping; a key with no values keeps an empty list.
func listsOf(l schema.Lists) map[string][]string {
	m := make(map[string][]string, len(l))
	for _, x := range l {
		vals := x.Values
		if vals == nil {
			vals = []string{}
		}
		m[x.Key] = vals
	}
	return m
}
