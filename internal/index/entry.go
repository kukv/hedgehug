package index

import (
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Entry is what the index keeps of one task.
type Entry struct {
	ID         model.ID
	Key        string
	FormerKeys []string
	Kind       string
	Title      string
	Parent     *model.ID
	Rank       model.Rank
	Status     string
	Assignees  []string
	Labels     []string
	BlockedBy  []model.ID
	Props      []Prop // in file order; queries read them by the schema's types
	Created    time.Time
	Updated    time.Time
	DeletedAt  *time.Time
	Text       string             // title and body in lower case, for free-text search
	Violations []schema.Violation // against the schema where the task sits
	Broken     string             // why the file did not decode; empty when it did
}

// Prop is one property of a task.
type Prop struct {
	Key    string
	Values []string // one for a scalar, one per item for a list of scalars, none otherwise
}

// Prop returns the values of the property key; nil when the task has none.
func (e *Entry) Prop(key string) []string {
	for _, p := range e.Props {
		if p.Key == key {
			return p.Values
		}
	}
	return nil
}

// newEntry decodes the task file data, found at the path of id. A file that
// does not decode, or whose id is not the one of its path, gives a broken
// entry and no task.
func newEntry(id model.ID, data []byte) (*Entry, *model.Task) {
	t, err := model.DecodeTask(data)
	if err == nil && t.ID != id {
		err = fmt.Errorf("id %s does not match the file name", t.ID)
	}
	if err != nil {
		return &Entry{ID: id, Broken: err.Error()}, nil
	}
	e := &Entry{
		ID:         t.ID,
		Key:        t.Key,
		FormerKeys: t.FormerKeys,
		Kind:       t.Kind,
		Title:      t.Title,
		Parent:     t.Parent,
		Rank:       t.Rank,
		Status:     t.Status,
		Assignees:  t.Assignees,
		Labels:     t.Labels,
		BlockedBy:  t.BlockedBy,
		Created:    t.Created,
		Updated:    t.Updated,
		DeletedAt:  t.DeletedAt,
		Text:       strings.ToLower(t.Title + "\n" + t.Body),
	}
	for _, f := range t.Props {
		e.Props = append(e.Props, Prop{Key: f.Key, Values: scalars(f.Value)})
	}
	return e, t
}

// scalars returns the values of a property node: one for a scalar, one per
// item for a list of scalars, none for null or any other shape.
func scalars(n *yaml.Node) []string {
	switch {
	case n == nil || n.ShortTag() == "!!null":
		return nil
	case n.Kind == yaml.ScalarNode:
		return []string{n.Value}
	case n.Kind == yaml.SequenceNode:
		var out []string
		for _, it := range n.Content {
			if it.Kind != yaml.ScalarNode {
				return nil
			}
			out = append(out, it.Value)
		}
		return out
	}
	return nil
}
