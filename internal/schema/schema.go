// Package schema reads schema.yaml and checks tasks against it.
package schema

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/model"
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

// Role says what a kind of task is for.
type Role string

const (
	RoleContainer Role = "container" // groups work, like a project or an epic
	RoleWork      Role = "work"      // a work item
)

// Root stands for "no parent" in Kind.Parents.
const Root = "root"

type Kind struct {
	Name     string   `yaml:"name"`
	Label    string   `yaml:"label,omitempty"`
	Role     Role     `yaml:"role"`
	Parents  []string `yaml:"parents,omitempty"` // nil: anywhere
	Icon     string   `yaml:"icon,omitempty"`
	Template string   `yaml:"template,omitempty"`
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
	TypeTaskRef   PropType = "task_ref"
	TypeURL       PropType = "url"
)

// PropTypes lists every property type.
var PropTypes = []PropType{
	TypeText, TypeNumber, TypeBool, TypeDate, TypeDatetime, TypeDuration,
	TypeEnum, TypeMultiEnum, TypeUser, TypeTaskRef, TypeURL,
}

type Prop struct {
	Name      string   `yaml:"-"`
	Type      PropType `yaml:"type"`
	Values    []string `yaml:"values,omitempty"`
	Multi     bool     `yaml:"multi,omitempty"`
	Label     string   `yaml:"label,omitempty"`
	Inverse   string   `yaml:"inverse,omitempty"`
	Symmetric bool     `yaml:"symmetric,omitempty"`
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
	Statuses []Status
	Kinds    []Kind
	Props    Props
	Workflow Workflow
	Scoped   []Scoped
	Extra    model.Fields

	doc *model.Doc
}

var schemaKeys = []string{"statuses", "kinds", "props", "workflow", "scoped"}

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
		{"kinds", &s.Kinds},
		{"props", &s.Props},
		{"workflow", &s.Workflow},
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
	e.Put("kinds", s.Kinds, len(s.Kinds) == 0)
	e.Put("props", s.Props, len(s.Props) == 0)
	e.Put("workflow", s.Workflow, s.Workflow.IsZero())
	e.Put("scoped", s.Scoped, len(s.Scoped) == 0)
	return e.Bytes(s.Extra)
}

// Default returns the schema of a new store: seven statuses in four
// categories, five kinds and the common properties.
func Default() *Schema {
	return &Schema{
		Statuses: []Status{
			{Name: "backlog", Label: "バックログ", Category: CategoryTodo},
			{Name: "todo", Label: "未着手", Category: CategoryTodo, Default: true},
			{Name: "on_hold", Label: "保留", Category: CategoryTodo},
			{Name: "in_progress", Label: "進行中", Category: CategoryDoing, Default: true},
			{Name: "waiting", Label: "待機中", Category: CategoryDoing},
			{Name: "done", Label: "完了", Category: CategoryDone, Default: true},
			{Name: "canceled", Label: "中止", Category: CategoryCanceled, Default: true},
		},
		Kinds: []Kind{
			{Name: "project", Label: "プロジェクト", Role: RoleContainer},
			{Name: "epic", Label: "エピック", Role: RoleContainer, Parents: []string{Root, "project"}},
			{Name: "story", Label: "ストーリー", Role: RoleWork, Parents: []string{Root, "project", "epic"}},
			{Name: "task", Label: "タスク", Role: RoleWork, Parents: []string{Root, "project", "epic", "story", "task"}, Default: true},
			{Name: "bug", Label: "バグ", Role: RoleWork, Parents: []string{Root, "project", "epic", "story"}},
		},
		Props: Props{
			{Name: "priority", Type: TypeEnum, Values: []string{"low", "mid", "high", "urgent"}},
			{Name: "start", Type: TypeDate},
			{Name: "due", Type: TypeDate},
			{Name: "estimate", Type: TypeDuration},
			{Name: "points", Type: TypeNumber},
			{Name: "duplicates", Type: TypeTaskRef, Multi: true, Label: "重複している", Inverse: "重複されている"},
			{Name: "causes", Type: TypeTaskRef, Multi: true, Label: "引き起こしている", Inverse: "起因している"},
			{Name: "relates", Type: TypeTaskRef, Multi: true, Label: "関連", Symmetric: true},
		},
	}
}

// Dev returns Default plus a review status, for stores that track
// development (hh store init --template dev).
func Dev() *Schema {
	s := Default()
	s.Statuses = append(s.Statuses[:4:4], append([]Status{
		{Name: "review", Label: "レビュー", Category: CategoryDoing},
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

func (s *Schema) Kind(name string) (Kind, bool) {
	for _, k := range s.Kinds {
		if k.Name == name {
			return k, true
		}
	}
	return Kind{}, false
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

// DefaultKind returns the default kind of the work role.
func (s *Schema) DefaultKind() (Kind, bool) {
	for _, k := range s.Kinds {
		if k.Role == RoleWork && k.Default {
			return k, true
		}
	}
	return Kind{}, false
}
