// Package schema reads schema.yaml and checks tasks against it.
package schema

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"go.yaml.in/yaml/v4"
)

// Path is where a store keeps its schema.
const Path = "schema.yaml"

// Category groups statuses so that views across stores line up.
type Category string

const (
	CategoryTodo     Category = "todo"
	CategoryDoing    Category = "doing"
	CategoryDone     Category = "done"
	CategoryCanceled Category = "canceled"
)

// Categories lists every category.
var Categories = []Category{CategoryTodo, CategoryDoing, CategoryDone, CategoryCanceled}

type Status struct {
	Name     string   `yaml:"name"`
	Label    string   `yaml:"label,omitempty"`
	Category Category `yaml:"category"`
	Color    string   `yaml:"color,omitempty"`
	Default  bool     `yaml:"default,omitempty"`
}

type PropType string

const (
	TypeText      PropType = "text"
	TypeNumber    PropType = "number"
	TypeBool      PropType = "bool"
	TypeDate      PropType = "date"
	TypeDatetime  PropType = "datetime"
	TypeDuration  PropType = "duration"
	TypeEnum      PropType = "enum"
	TypeMultiEnum PropType = "multi_enum"
	TypeUser      PropType = "user"
	TypeURL       PropType = "url"
)

// PropTypes lists every property type.
var PropTypes = []PropType{
	TypeText, TypeNumber, TypeBool, TypeDate, TypeDatetime, TypeDuration,
	TypeEnum, TypeMultiEnum, TypeUser, TypeURL,
}

type Prop struct {
	Name   string   `yaml:"-"`
	Type   PropType `yaml:"type"`
	Values []string `yaml:"values,omitempty"`
	Multi  bool     `yaml:"multi,omitempty"`
	Label  string   `yaml:"label,omitempty"`
}

// Props keeps properties in file order. In YAML it is a mapping from name
// to definition.
type Props []Prop

func (p *Props) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: props must be a mapping", n.Line)
	}
	out := make(Props, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		var pr Prop
		if err := n.Content[i+1].Decode(&pr); err != nil {
			return err
		}
		pr.Name = n.Content[i].Value
		out = append(out, pr)
	}
	*p = out
	return nil
}

func (p Props) MarshalYAML() (any, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, pr := range p {
		var v yaml.Node
		if err := v.Encode(pr); err != nil {
			return nil, err
		}
		v.Style = yaml.FlowStyle
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: pr.Name}, &v)
	}
	return m, nil
}

// List is one key of Lists.
type List struct {
	Key    string `yaml:"-"`
	Values []string
}

// Lists is an ordered mapping from a name to a list of names, as in
// workflow.transitions and workflow.required.
type Lists []List

// Get returns the list for key.
func (l Lists) Get(key string) ([]string, bool) {
	for _, x := range l {
		if x.Key == key {
			return x.Values, true
		}
	}
	return nil, false
}

func (l *Lists) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: want a mapping of lists", n.Line)
	}
	out := make(Lists, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		var v []string
		if err := n.Content[i+1].Decode(&v); err != nil {
			return err
		}
		out = append(out, List{n.Content[i].Value, v})
	}
	*l = out
	return nil
}

func (l Lists) MarshalYAML() (any, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, x := range l {
		var v yaml.Node
		if err := v.Encode(x.Values); err != nil {
			return nil, err
		}
		v.Style = yaml.FlowStyle
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x.Key}, &v)
	}
	return m, nil
}

type Workflow struct {
	Transitions Lists `yaml:"transitions,omitempty"` // status → statuses it may move to
	Required    Lists `yaml:"required,omitempty"`    // status → fields needed to enter it
}

func (w Workflow) IsZero() bool { return len(w.Transitions) == 0 && len(w.Required) == 0 }

type Templates struct {
	Only    []string          `yaml:"only,omitempty"`
	Default map[string]string `yaml:"default,omitempty"` // kind → template
}

// Scoped overrides behavior for one project and its descendants.
type Scoped struct {
	Where     string     `yaml:"where"` // project:<ULID>
	Statuses  []string   `yaml:"statuses,omitempty"`
	Workflow  *Workflow  `yaml:"workflow,omitempty"`
	Templates *Templates `yaml:"templates,omitempty"`
}

// Schema is schema.yaml.
type Schema struct {
	Statuses  []Status
	Props     Props
	Workflow  Workflow
	Templates *Templates // default template of each kind, for the store
	Scoped    []Scoped
	Extra     model.Fields

	doc *model.Doc
}

var schemaKeys = []string{"statuses", "props", "workflow", "templates", "scoped"}

// Decode reads schema.yaml. It does not check the schema; see Check.
func Decode(data []byte) (*Schema, error) {
	d, err := model.ParseDoc(data, 0)
	if err != nil {
		return nil, err
	}
	s := &Schema{doc: d}
	for _, f := range []struct {
		key string
		out any
	}{
		{"statuses", &s.Statuses},
		{"props", &s.Props},
		{"workflow", &s.Workflow},
		{"templates", &s.Templates},
		{"scoped", &s.Scoped},
	} {
		if err := d.Decode(f.key, f.out); err != nil {
			return nil, err
		}
	}
	s.Extra = d.Extra(schemaKeys)
	return s, nil
}

// Encode writes schema.yaml, keeping comments and unchanged parts as written.
func (s *Schema) Encode() ([]byte, error) {
	if s.doc == nil {
		s.doc = model.NewDoc()
	}
	e := s.doc.Encoder()
	e.Put("statuses", s.Statuses, len(s.Statuses) == 0)
	e.Put("props", s.Props, len(s.Props) == 0)
	if !s.Workflow.IsZero() { // a key not put is left out, so an emptied workflow is not written as {}
		e.Put("workflow", s.Workflow, false)
	}
	e.Put("templates", s.Templates, s.Templates == nil || (len(s.Templates.Only) == 0 && len(s.Templates.Default) == 0))
	e.Put("scoped", s.Scoped, len(s.Scoped) == 0)
	return e.Bytes(s.Extra)
}

// Default returns the schema of a new store: seven statuses in four
// categories and the common properties.
func Default() *Schema {
	return &Schema{
		Statuses: []Status{
			{Name: "backlog", Label: "Backlog", Category: CategoryTodo},
			{Name: "todo", Label: "Todo", Category: CategoryTodo, Default: true},
			{Name: "on_hold", Label: "On hold", Category: CategoryTodo},
			{Name: "in_progress", Label: "In progress", Category: CategoryDoing, Default: true},
			{Name: "waiting", Label: "Waiting", Category: CategoryDoing},
			{Name: "done", Label: "Done", Category: CategoryDone, Default: true},
			{Name: "canceled", Label: "Canceled", Category: CategoryCanceled, Default: true},
		},
		Props: Props{
			{Name: "priority", Type: TypeEnum, Values: []string{"low", "mid", "high", "urgent"}, Label: "優先度"},
			{Name: "start", Type: TypeDate, Label: "開始日"},
			{Name: "due", Type: TypeDate, Label: "期限"},
			{Name: "estimate", Type: TypeDuration, Label: "見積もり"},
			{Name: "points", Type: TypeNumber, Label: "ポイント"},
		},
	}
}

// Dev returns Default plus a review status, for stores that track
// development (hh store init --template dev).
func Dev() *Schema {
	s := Default()
	s.Statuses = append(s.Statuses[:4:4], append([]Status{
		{Name: "review", Label: "Review", Category: CategoryDoing},
	}, s.Statuses[4:]...)...)
	return s
}

func (s *Schema) Status(name string) (Status, bool) {
	for _, st := range s.Statuses {
		if st.Name == name {
			return st, true
		}
	}
	return Status{}, false
}

func (s *Schema) Prop(name string) (Prop, bool) {
	for _, p := range s.Props {
		if p.Name == name {
			return p, true
		}
	}
	return Prop{}, false
}

// DefaultStatus returns the default status of category c. hh start,
// automations and moves between stores use it.
func (s *Schema) DefaultStatus(c Category) (Status, bool) {
	for _, st := range s.Statuses {
		if st.Category == c && st.Default {
			return st, true
		}
	}
	return Status{}, false
}
