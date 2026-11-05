package usecase

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

// TaskView is a task as --json prints it.
type TaskView struct {
	ULID      string        `json:"ulid"`
	ID        string        `json:"id"`
	Store     string        `json:"store"`
	FormerIDs []string      `json:"former_ids,omitempty"`
	Workspace *WorkspaceRef `json:"workspace"`
	// Project is the nearest ancestor of kind project (the projects the GUI
	// lists, kind:project); it is left out for a task with none.
	Project *Ref `json:"project,omitempty"`
	// Scope is the ULID of the ancestor (or the task itself, when it is a
	// container) whose schema override applies to the task, chosen as a write
	// is checked (schema.For); left out when none does.
	Scope     string         `json:"scope,omitempty"`
	Kind      string         `json:"kind"`
	Title     string         `json:"title"`
	Parent    *string        `json:"parent"`
	Rank      string         `json:"rank"`
	Status    string         `json:"status"`
	Assignees []string       `json:"assignees"`
	Labels    []string       `json:"labels"`
	BlockedBy []Ref          `json:"blocked_by"`
	Sprint    *PlanRef       `json:"sprint,omitempty"`
	Releases  []PlanRef      `json:"releases,omitempty" nullable:"false"`
	Props     map[string]any `json:"props"`
	Created   time.Time      `json:"created"`
	Updated   time.Time      `json:"updated"`
	DeletedAt *time.Time     `json:"deleted_at"`
	Problems  []Problem      `json:"problems"`
	Broken    string         `json:"broken,omitempty"`
	Links     []LinkView     `json:"links,omitempty" nullable:"false" doc:"The pull requests and issues linked, with what GitHub says of them"`

	// Only in ls --tree --json and show --json.
	Checklist *Count `json:"checklist,omitempty"`
	Progress  *Count `json:"progress,omitempty"`

	// Only in ls --tree --json.
	Context  bool        `json:"context,omitempty"`  // shown only to connect the tasks found
	InCycle  bool        `json:"in_cycle,omitempty"` // in a parent cycle, out of reach of the roots
	Children []*TaskView `json:"children,omitempty"`

	// What the text of show, ls and tree writes and --json does not print.
	DisplayID        string     `json:"-"` // the id as the index shows it (ix.DisplayID)
	ParentRef        *Ref       `json:"-"` // the parent, when the task has one
	FormerDisplayIDs []string   `json:"-"` // FormerIDs as the index shows them (ix.QualifyID)
	PropList         []PropView `json:"-"` // the props in the order the task has them
}

// PropView is a property of a task for the text: its values, or the tasks it
// points at when its type is taskref.
type PropView struct {
	Key    string
	Values []string // when not a taskref
	Refs   []Ref    // when a taskref
	IsRef  bool     // the type in the schema is TypeTaskRef
}

// WorkspaceRef names the workspace of a task.
type WorkspaceRef struct {
	ULID     string `json:"ulid"`
	Name     string `json:"name"`
	IDPrefix string `json:"id_prefix"`
	// Known says the index has the workspace: a workspace may have an empty
	// name, so Name does not tell.
	Known bool `json:"-"`
}

// Ref names a task that another task points at.
type Ref struct {
	ULID  string `json:"ulid"`
	ID    string `json:"id"`
	Title string `json:"title"`
	// DisplayID is the id as the index shows it; Missing says the index does
	// not have the task. Both are for the text.
	DisplayID string `json:"-"`
	Missing   bool   `json:"-"`
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
		Store:     ix.StoreName(),
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
		Links:     linkViews(e.Links, e.GitHub),

		DisplayID: ix.DisplayID(e),
	}
	if e.Parent != nil {
		p := e.Parent.String()
		j.Parent = &p
		r := RefOf(ix, *e.Parent)
		j.ParentRef = &r
	}
	for _, id := range e.FormerIDs {
		j.FormerDisplayIDs = append(j.FormerDisplayIDs, ix.QualifyID(id))
	}
	if !e.Workspace.IsZero() {
		j.Workspace = &WorkspaceRef{ULID: e.Workspace.String()}
		if w, ok := ix.Workspace(e.Workspace); ok {
			j.Workspace.Name, j.Workspace.IDPrefix, j.Workspace.Known = w.Name, w.IDPrefix, true
		}
	}
	j.Project = projectOf(ix, e)
	j.Scope = scopeOf(ix, e)
	for _, b := range e.BlockedBy {
		j.BlockedBy = append(j.BlockedBy, RefOf(ix, b))
	}
	if e.Sprint != nil {
		r := sprintRef(ix, *e.Sprint)
		j.Sprint = &r
	}
	for _, u := range e.Releases {
		j.Releases = append(j.Releases, releaseRef(ix, u))
	}
	sc := ix.Schema()
	for _, p := range e.Props {
		sp, ok := sc.Prop(p.Key)
		pv := PropView{Key: p.Key, Values: p.Values}
		if ok && sp.Type == schema.TypeTaskRef {
			pv.IsRef, pv.Values = true, nil
			for _, u := range e.Targets(p.Key) {
				pv.Refs = append(pv.Refs, RefOf(ix, u))
			}
		}
		j.PropList = append(j.PropList, pv)
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

// ListViewOf is the view of the task e as a row of a list: the task with its
// checklist and progress.
func ListViewOf(ix *index.Index, e *index.Entry) *TaskView {
	v := TaskViewOf(ix, e)
	v.Checklist = ChecklistOf(e)
	v.Progress = ProgressOf(ix, e)
	return v
}

// projectOf is the nearest ancestor of e of kind project, or nil. e itself is
// not looked at. An ancestor the index does not have, or a cycle of parents,
// ends the walk with nil.
func projectOf(ix *index.Index, e *index.Entry) *Ref {
	seen := map[model.ULID]bool{e.ULID: true}
	for p := e.Parent; p != nil; {
		if seen[*p] {
			return nil
		}
		seen[*p] = true
		a, ok := ix.Task(*p)
		if !ok {
			return nil
		}
		if a.Kind == ProjectKind {
			r := RefOf(ix, a.ULID)
			return &r
		}
		p = a.Parent
	}
	return nil
}

// scopeOf is the ULID of the ancestor whose override applies to e, or "". The
// ancestors are the ones the index checks e against (Index.Ancestors): e first
// when it is a container, then its parents nearest first, up to a task the
// index lacks or cannot read.
func scopeOf(ix *index.Index, e *index.Entry) string {
	sc := ix.Schema()
	if len(sc.Scoped) == 0 {
		return ""
	}
	if u, ok := sc.ScopeOf(ix.Ancestors(e)); ok {
		return u.String()
	}
	return ""
}

// RefOf names the task ulid, with an empty id and title when the index does
// not have it.
func RefOf(ix *index.Index, ulid model.ULID) Ref {
	r := Ref{ULID: ulid.String()}
	if e, ok := ix.Task(ulid); ok {
		r.ID, r.Title, r.DisplayID = e.ID, e.Title, ix.DisplayID(e)
	} else {
		r.Missing = true
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

// TaskLabel names a task for a reader: its id (as DisplayID of ix has it)
// and full ULID, or the ULID alone when the task has no id (a file that does
// not decode).
func TaskLabel(ix *index.Index, e *index.Entry) string {
	return FormatTaskLabel(e.ID, ix.DisplayID(e), e.ULID)
}

// FormatTaskLabel is TaskLabel for a task read out of its index: its id as the
// file has it, as the index shows it, and its ULID.
func FormatTaskLabel(id, shown string, ulid model.ULID) string {
	return labelOf(id, shown, ulid.String())
}

func labelOf(id, shown, ulid string) string {
	if id == "" {
		return ulid
	}
	return shown + " (" + ulid + ")"
}

// Label is TaskLabel of the task v is the view of.
func (v *TaskView) Label() string { return labelOf(v.ID, v.DisplayID, v.ULID) }

// Name is RefName of the task r names.
func (r Ref) Name() string {
	if r.Missing {
		return r.ULID + " (not found)"
	}
	return labelOf(r.ID, r.DisplayID, r.ULID) + " " + r.Title
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
		return TaskLabel(ix, e) + " " + e.Title
	}
	return ulid.String() + " (not found)"
}
