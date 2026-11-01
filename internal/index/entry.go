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
	ULID      model.ULID
	ID        string
	FormerIDs []string
	Workspace model.ULID
	Kind      string
	Title     string
	Parent    *model.ULID
	Rank      model.Rank
	Status    string
	Assignees []string
	Watchers  []string
	Labels    []string
	BlockedBy []model.ULID
	Props     []Prop // in file order; queries read them by the schema's types
	Created   time.Time
	Updated   time.Time
	DeletedAt *time.Time
	Text      string   // title and body in lower case, for free-text search
	Mentions  []string // accounts mentioned in the body and the comments not deleted (login or host/login, as written), each once, in order
	Refs      []string // ids referred to as #<ID> there, each once, in order
	// The checkbox items of the body, and those checked; total is 0 when it has none.
	ChecklistDone  int
	ChecklistTotal int
	Comments       []CommentEntry     // in ULID order, deleted ones included
	Worklogs       []WorklogEntry     // in ULID order, deleted ones included
	Violations     []schema.Violation // against the schema where the task sits
	Broken         string             // why the file did not decode; empty when it did
	// BrokenNumbers is, for a file that did not decode, the largest number
	// of each prefix among its words of the form of an id; numbering counts
	// them, since the file's ids cannot be read.
	BrokenNumbers map[string]int
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

// newEntry decodes the task file data, found at the path of ulid. A file that
// does not decode, or whose ULID is not the one of its path, gives a broken
// entry and no task.
func newEntry(ulid model.ULID, data []byte) (*Entry, *model.Task) {
	t, err := model.DecodeTask(data)
	if err == nil && t.ULID != ulid {
		err = fmt.Errorf("ulid %s does not match the file name", t.ULID)
	}
	if err != nil {
		return &Entry{ULID: ulid, Broken: err.Error(), BrokenNumbers: model.IDNumbersIn(data)}, nil
	}
	return EntryOf(t), t
}

// EntryOf returns what the index keeps of t, as read from its file: its
// mentions and refs are those of the body, as it knows of no comments. A
// command uses it for a task that is not yet written.
func EntryOf(t *model.Task) *Entry {
	e := &Entry{
		ULID:      t.ULID,
		ID:        t.ID,
		FormerIDs: t.FormerIDs,
		Workspace: t.Workspace,
		Kind:      t.Kind,
		Title:     t.Title,
		Parent:    t.Parent,
		Rank:      t.Rank,
		Status:    t.Status,
		Assignees: t.Assignees,
		Watchers:  t.Watchers,
		Labels:    t.Labels,
		BlockedBy: t.BlockedBy,
		Created:   t.Created,
		Updated:   t.Updated,
		DeletedAt: t.DeletedAt,
		Text:      strings.ToLower(t.Title + "\n" + t.Body),
		Mentions:  Mentions(t.Body),
		Refs:      TaskRefs(t.Body),
	}
	e.ChecklistDone, e.ChecklistTotal = Checklist(t.Body)
	for _, f := range t.Props {
		e.Props = append(e.Props, Prop{Key: f.Key, Values: scalars(f.Value)})
	}
	return e
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

// Targets returns the tasks e points at through rel: blocked_by, or a
// task_ref property. Property values that are not ULIDs are left out.
func (e *Entry) Targets(rel string) []model.ULID {
	if rel == "blocked_by" {
		return e.BlockedBy
	}
	var out []model.ULID
	for _, v := range e.Prop(rel) {
		if ulid, err := model.ParseULID(v); err == nil {
			out = append(out, ulid)
		}
	}
	return out
}
