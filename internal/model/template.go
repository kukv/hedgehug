package model

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v4"
)

// Assignees are the assignees of a template item: a list of logins, or
// "inherit" to take the parent's assignees when the template is expanded.
type Assignees struct {
	Inherit bool
	Logins  []string
}

func (a Assignees) IsZero() bool { return !a.Inherit && len(a.Logins) == 0 }

func (a Assignees) MarshalYAML() (any, error) {
	if a.Inherit {
		return "inherit", nil
	}
	return a.Logins, nil
}

func (a *Assignees) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Value == "inherit" {
		*a = Assignees{Inherit: true}
		return nil
	}
	logins, err := scalars(n)
	if err != nil {
		return fmt.Errorf("line %d: assignees: %w", n.Line, err)
	}
	*a = Assignees{Logins: logins}
	return nil
}

// TemplateItem is a task to create from a template. The parent and the rank
// are decided when the template is expanded.
type TemplateItem struct {
	Kind      string         `yaml:"kind,omitempty"`
	Title     string         `yaml:"title,omitempty"`
	Status    string         `yaml:"status,omitempty"`
	Assignees Assignees      `yaml:"assignees,omitempty"`
	Watchers  []string       `yaml:"watchers,omitempty"`
	Labels    []string       `yaml:"labels,omitempty"`
	Repos     []string       `yaml:"repos,omitempty"`
	BlockedBy []ID           `yaml:"blocked_by,omitempty"`
	Sprint    *ID            `yaml:"sprint,omitempty"`
	Releases  []ID           `yaml:"releases,omitempty"`
	Links     []Link         `yaml:"links,omitempty"`
	Props     Fields         `yaml:"props,omitempty"`
	Body      string         `yaml:"body,omitempty"`
	Children  []TemplateItem `yaml:"children,omitempty"`
}

// targets returns the decode targets of the keys a template file shares
// with its children. body is not among them: a file's body is its Markdown.
func (it *TemplateItem) targets() map[string]any {
	return map[string]any{
		"kind": &it.Kind, "title": &it.Title, "status": &it.Status,
		"assignees": &it.Assignees, "watchers": &it.Watchers, "labels": &it.Labels,
		"repos": &it.Repos, "blocked_by": &it.BlockedBy, "sprint": &it.Sprint,
		"releases": &it.Releases, "links": &it.Links, "props": &it.Props,
		"children": &it.Children,
	}
}

// UnmarshalYAML reads a child item the way DecodeTemplate reads the file, so
// that a title such as 2026-11-03 stays a string.
func (it *TemplateItem) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: want a mapping", n.Line)
	}
	*it = TemplateItem{}
	targets := it.targets()
	targets["body"] = &it.Body
	d := &Doc{root: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{n}}}
	return decodeAll(d, targets)
}

// Template is one template file. Its name is the file name. Item.Body is the
// body of the file.
type Template struct {
	Name    string
	Item    TemplateItem
	Created time.Time
	Updated time.Time
	Extra   Fields

	doc *Doc
}

var templateKeys = []string{
	"kind", "title", "status", "assignees", "watchers", "labels", "repos",
	"blocked_by", "sprint", "releases", "links", "props", "children", "created", "updated",
}

func DecodeTemplate(name string, data []byte) (*Template, error) {
	if err := ValidTemplateName(name); err != nil {
		return nil, &DecodeError{Field: "name", Err: err}
	}
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	t := &Template{Name: name, doc: d}
	it := &t.Item
	targets := it.targets()
	targets["created"], targets["updated"] = &t.Created, &t.Updated
	if err := decodeAll(d, targets); err != nil {
		return nil, err
	}
	it.Body = body
	t.Extra = d.Extra(templateKeys)
	return t, nil
}

func (t *Template) Encode() ([]byte, error) {
	if t.doc == nil {
		t.doc = NewDoc()
	}
	it := t.Item
	e := t.doc.Encoder()
	e.Put("kind", it.Kind, it.Kind == "")
	e.Put("title", it.Title, it.Title == "")
	e.Put("status", it.Status, it.Status == "")
	e.Put("assignees", it.Assignees, it.Assignees.IsZero())
	e.Put("watchers", it.Watchers, len(it.Watchers) == 0)
	e.Put("labels", it.Labels, len(it.Labels) == 0)
	e.Put("repos", it.Repos, len(it.Repos) == 0)
	e.Put("blocked_by", it.BlockedBy, len(it.BlockedBy) == 0)
	e.Put("sprint", it.Sprint, it.Sprint == nil)
	e.Put("releases", it.Releases, len(it.Releases) == 0)
	e.Put("links", it.Links, len(it.Links) == 0)
	e.Put("props", mappingOf(t.doc.Get("props"), it.Props), len(it.Props) == 0)
	e.Put("children", it.Children, len(it.Children) == 0)
	e.Put("created", t.Created, t.Created.IsZero())
	e.Put("updated", t.Updated, t.Updated.IsZero())
	front, err := e.Bytes(t.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, it.Body), nil
}
