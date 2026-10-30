package model

import (
	"errors"
	"slices"
	"time"

	"go.yaml.in/yaml/v4"
)

// Task is one task file. A project is a task too.
type Task struct {
	ULID      ULID
	ID        string   // human-readable id such as WEB-42, given when the task is written
	FormerIDs []string // ids before renumbering
	Workspace ULID     // zero when not set
	Kind      string
	Title     string
	Parent    *ULID
	Rank      Rank
	Status    string
	Assignees []string
	Watchers  []string
	Labels    []string
	Repos     []string
	BlockedBy []ULID
	Sprint    *ULID
	Releases  []ULID
	Links     []Link
	Props     Fields     // typed by the schema, not here
	GH        *yaml.Node // values from GitHub, read-only; typed in phase 5
	Created   time.Time
	CreatedBy string
	Updated   time.Time
	DeletedAt *time.Time
	Extra     Fields // keys this version does not know, in file order
	Body      string

	doc *Doc
}

// Link ties a task to a pull request or an issue.
type Link struct {
	Type string `yaml:"type"`
	Ref  string `yaml:"ref"`
	Sync string `yaml:"sync,omitempty"`
}

var taskKeys = []string{
	"ulid", "id", "former_ids", "workspace", "kind", "title", "parent", "rank", "status",
	"assignees", "watchers", "labels", "repos", "blocked_by", "sprint", "releases", "links",
	"props", "gh", "created", "created_by", "updated", "deleted_at",
}

// TaskKeys returns the keys of a task file, in the order they are written.
func TaskKeys() []string { return slices.Clone(taskKeys) }

// DecodeTask reads a task file. Only ulid is required; other checks belong to
// the schema.
func DecodeTask(data []byte) (*Task, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(front, 1)
	if err != nil {
		return nil, err
	}
	t := &Task{Body: body, doc: d}
	for _, f := range []struct {
		key string
		out any
	}{
		{"ulid", &t.ULID},
		{"id", &t.ID},
		{"former_ids", &t.FormerIDs},
		{"workspace", &t.Workspace},
		{"kind", &t.Kind},
		{"title", &t.Title},
		{"parent", &t.Parent},
		{"rank", &t.Rank},
		{"status", &t.Status},
		{"assignees", &t.Assignees},
		{"watchers", &t.Watchers},
		{"labels", &t.Labels},
		{"repos", &t.Repos},
		{"blocked_by", &t.BlockedBy},
		{"sprint", &t.Sprint},
		{"releases", &t.Releases},
		{"links", &t.Links},
		{"props", &t.Props},
		{"gh", &t.GH},
		{"created", &t.Created},
		{"created_by", &t.CreatedBy},
		{"updated", &t.Updated},
		{"deleted_at", &t.DeletedAt},
	} {
		if err := d.Decode(f.key, f.out); err != nil {
			return nil, err
		}
	}
	if t.ULID.IsZero() {
		return nil, &DecodeError{Field: "ulid", Err: errors.New("missing")}
	}
	t.Extra = d.Extra(taskKeys)
	return t, nil
}

// Encode writes the task file. Comments, unknown keys and values that did
// not change keep the form they had in the file.
func (t *Task) Encode() ([]byte, error) {
	if t.doc == nil {
		t.doc = NewDoc()
	}
	e := t.doc.Encoder()
	e.Put("ulid", t.ULID, false)
	e.Put("id", t.ID, t.ID == "")
	e.Put("former_ids", t.FormerIDs, len(t.FormerIDs) == 0)
	e.Put("workspace", t.Workspace, t.Workspace.IsZero())
	e.Put("kind", t.Kind, t.Kind == "")
	e.Put("title", t.Title, t.Title == "")
	e.Put("parent", t.Parent, t.Parent == nil)
	e.Put("rank", string(t.Rank), t.Rank == "")
	e.Put("status", t.Status, t.Status == "")
	e.Put("assignees", t.Assignees, len(t.Assignees) == 0)
	e.Put("watchers", t.Watchers, len(t.Watchers) == 0)
	e.Put("labels", t.Labels, len(t.Labels) == 0)
	e.Put("repos", t.Repos, len(t.Repos) == 0)
	e.Put("blocked_by", t.BlockedBy, len(t.BlockedBy) == 0)
	e.Put("sprint", t.Sprint, t.Sprint == nil)
	e.Put("releases", t.Releases, len(t.Releases) == 0)
	e.Put("links", t.Links, len(t.Links) == 0)
	e.Put("props", mappingOf(t.doc.Get("props"), t.Props), len(t.Props) == 0)
	e.Put("gh", t.GH, t.GH == nil)
	e.Put("created", t.Created, t.Created.IsZero())
	e.Put("created_by", t.CreatedBy, t.CreatedBy == "")
	e.Put("updated", t.Updated, t.Updated.IsZero())
	e.Put("deleted_at", t.DeletedAt, t.DeletedAt == nil)
	front, err := e.Bytes(t.Extra)
	if err != nil {
		return nil, err
	}
	return joinFrontmatter(front, t.Body), nil
}
