package ops

import (
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// TaskView is a task as --json prints it.
type TaskView struct {
	ULID      string         `json:"ulid"`
	ID        string         `json:"id"`
	FormerIDs []string       `json:"former_ids,omitempty"`
	Workspace *WorkspaceRef  `json:"workspace"`
	Kind      string         `json:"kind"`
	Title     string         `json:"title"`
	Parent    *string        `json:"parent"`
	Rank      string         `json:"rank"`
	Status    string         `json:"status"`
	Assignees []string       `json:"assignees"`
	Labels    []string       `json:"labels"`
	BlockedBy []Ref          `json:"blocked_by"`
	Props     map[string]any `json:"props"`
	Created   time.Time      `json:"created"`
	Updated   time.Time      `json:"updated"`
	DeletedAt *time.Time     `json:"deleted_at"`
	Problems  []Problem      `json:"problems"`
	Broken    string         `json:"broken,omitempty"`

	// Only in ls --tree --json.
	Context  bool        `json:"context,omitempty"`  // shown only to connect the tasks found
	InCycle  bool        `json:"in_cycle,omitempty"` // in a parent cycle, out of reach of the roots
	Children []*TaskView `json:"children,omitempty"`
}

// WorkspaceRef names the workspace of a task.
type WorkspaceRef struct {
	ULID     string `json:"ulid"`
	Name     string `json:"name"`
	IDPrefix string `json:"id_prefix"`
}

// Ref names a task that another task points at.
type Ref struct {
	ULID  string `json:"ulid"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Problem is a finding of the index about a task.
type Problem struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// TaskViewOf is the view of the task e.
func TaskViewOf(ix *index.Index, e *index.Entry) *TaskView {
	j := &TaskView{
		ULID:      e.ULID.String(),
		ID:        e.ID,
		FormerIDs: e.FormerIDs,
		Kind:      e.Kind,
		Title:     e.Title,
		Rank:      string(e.Rank),
		Status:    e.Status,
		Assignees: OrEmpty(e.Assignees),
		Labels:    OrEmpty(e.Labels),
		BlockedBy: []Ref{},
		Props:     map[string]any{},
		Created:   e.Created,
		Updated:   e.Updated,
		DeletedAt: e.DeletedAt,
		Problems:  []Problem{},
		Broken:    e.Broken,
	}
	if e.Parent != nil {
		p := e.Parent.String()
		j.Parent = &p
	}
	if !e.Workspace.IsZero() {
		j.Workspace = &WorkspaceRef{ULID: e.Workspace.String()}
		if w, ok := ix.Workspace(e.Workspace); ok {
			j.Workspace.Name, j.Workspace.IDPrefix = w.Name, w.IDPrefix
		}
	}
	for _, b := range e.BlockedBy {
		j.BlockedBy = append(j.BlockedBy, RefOf(ix, b))
	}
	sc := ix.Schema()
	for _, p := range e.Props {
		sp, ok := sc.Prop(p.Key)
		multi := (ok && (sp.Multi || sp.Type == schema.TypeMultiEnum)) || (!ok && len(p.Values) != 1)
		switch {
		case multi:
			j.Props[p.Key] = OrEmpty(p.Values)
		case len(p.Values) > 0:
			j.Props[p.Key] = p.Values[0]
		default:
			j.Props[p.Key] = nil
		}
	}
	for _, p := range ix.Problems(e.ULID) {
		j.Problems = append(j.Problems, Problem{p.Code, p.Field, p.Msg})
	}
	return j
}

// RefOf names the task ulid, with an empty id and title when the index does
// not have it.
func RefOf(ix *index.Index, ulid model.ULID) Ref {
	r := Ref{ULID: ulid.String()}
	if e, ok := ix.Task(ulid); ok {
		r.ID, r.Title = e.ID, e.Title
	}
	return r
}

// OrEmpty is s, or an empty slice when s is nil, so JSON prints [].
func OrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// TaskLabel names a task for a reader: its id and full ULID, or the ULID
// alone when the task has no id (a file that does not decode).
func TaskLabel(e *index.Entry) string {
	if e.ID == "" {
		return e.ULID.String()
	}
	return e.ID + " (" + e.ULID.String() + ")"
}

// TaskFileName names the temporary file that edits task e: its id, or its
// ULID when it has no id.
func TaskFileName(e *index.Entry) string {
	if e.ID == "" {
		return e.ULID.String()
	}
	return e.ID
}

// RefName names the task ulid for a reader: its id, ULID and title.
func RefName(ix *index.Index, ulid model.ULID) string {
	if e, ok := ix.Task(ulid); ok {
		return TaskLabel(e) + " " + e.Title
	}
	return ulid.String() + " (not found)"
}
