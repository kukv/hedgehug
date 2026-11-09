package stores

import (
	"reflect"
	"slices"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// MovePlanView is a MovePlan as the GUI shows it: structured, where MoveView
// is the lines the CLI prints. It holds no content of the tasks (not their
// text, comments or worklogs), and it is what the user agrees to: the same
// view, made again under the locks, must equal it (MoveStoreInput.ConfirmedView).
type MovePlanView struct {
	From      string            `json:"from"`
	To        string            `json:"to"`
	Workspace MoveWorkspaceView `json:"workspace" doc:"The workspace of the destination that the root task goes into"`
	Parent    *MoveTaskView     `json:"parent,omitempty" doc:"The parent in the destination, when one was given; id is as the destination shows it"`
	Tasks     []MoveTaskView    `json:"tasks" nullable:"false" doc:"The root first, every parent before its children; id is as the source shows it"`
	Unlinked  []MoveUnlinkView  `json:"unlinked" nullable:"false" doc:"The references the move cuts"`
	Hidden    []string          `json:"hidden" nullable:"false" doc:"Accounts that can read the source and not the destination; empty when either list is unknown"`
	Scoped    *MoveScopedView   `json:"scoped,omitempty" doc:"Present when a project moved has a scoped schema or automations of its own"`
}

// MoveWorkspaceView names a workspace of the destination.
type MoveWorkspaceView struct {
	ULID   model.ULID `json:"ulid"`
	Name   string     `json:"name"`
	Prefix string     `json:"prefix"`
}

// MoveRefView names a task of the source.
type MoveRefView struct {
	ULID model.ULID `json:"ulid"`
	ID   string     `json:"id"`
}

// MoveTargetView names what a reference points at: a task, a sprint or a release.
type MoveTargetView struct {
	ULID  model.ULID `json:"ulid"`
	Label string     `json:"label" doc:"The ID of a task, the name of a sprint or a release; the ULID when there is none"`
}

// MoveUnlinkView is a reference the move cuts.
type MoveUnlinkView struct {
	Task   MoveRefView    `json:"task"`
	Field  string         `json:"field" doc:"The name of a relation (blocked_by, causes, relates, duplicates or clones), sprint or releases"`
	Target MoveTargetView `json:"target"`
}

// MoveScopedView is the scoped schema and the where rules of automations.yaml
// that the tasks moved bring with them.
type MoveScopedView struct {
	Required bool     `json:"required" doc:"A choice is still needed: scoped was not given"`
	Choice   string   `json:"choice,omitempty" enum:"carry,drop" doc:"The choice that was given"`
	Dropped  []string `json:"dropped" nullable:"false" doc:"What carrying leaves out, as \"<what> <name>\" (status, transitions, required, prop, template, kind, rule, disable); empty unless carried"`
}

// GUIView is the plan as the GUI shows it.
func (p MovePlan) GUIView() MovePlanView {
	v := MovePlanView{
		From: p.From, To: p.To, Workspace: p.Workspace, Parent: p.Parent,
		Tasks:    make([]MoveTaskView, len(p.Tasks)),
		Unlinked: make([]MoveUnlinkView, len(p.Unlinked)),
		Hidden:   slices.Clone(p.Hidden),
	}
	if v.Hidden == nil {
		v.Hidden = []string{}
	}
	for i, e := range p.Tasks {
		v.Tasks[i] = MoveTaskView{ULID: e.ULID, ID: p.Label(e.ULID), Title: e.Title}
	}
	for i, u := range p.Unlinked {
		v.Unlinked[i] = MoveUnlinkView{
			Task:   MoveRefView{ULID: u.Task, ID: p.Label(u.Task)},
			Field:  u.Field,
			Target: MoveTargetView{ULID: u.Target, Label: p.Label(u.Target)},
		}
	}
	if p.Scoped != nil {
		v.Scoped = &MoveScopedView{Required: p.ScopedChoice == "", Choice: p.ScopedChoice, Dropped: []string{}}
		v.Scoped.Dropped = append(v.Scoped.Dropped, p.Scoped.Dropped...)
	}
	return v
}

// canonical gives the lists that are empty the same form, so that a view
// that went through JSON compares equal to the one made here.
func (v MovePlanView) canonical() MovePlanView {
	if v.Tasks == nil {
		v.Tasks = []MoveTaskView{}
	}
	if v.Unlinked == nil {
		v.Unlinked = []MoveUnlinkView{}
	}
	if v.Hidden == nil {
		v.Hidden = []string{}
	}
	if v.Scoped != nil && v.Scoped.Dropped == nil {
		s := *v.Scoped
		s.Dropped = []string{}
		v.Scoped = &s
	}
	return v
}

// samePlanView reports whether a and b are the same plan: the same tasks,
// where they go, the references cut, who loses sight and what the scoped
// schema loses.
func samePlanView(a, b MovePlanView) bool {
	return reflect.DeepEqual(a.canonical(), b.canonical())
}

// MoveStoreResultView is what a move did, as hedgehug mv --store --json
// tells it and as the server answers.
type MoveStoreResultView struct {
	From     string             `json:"from"`
	To       string             `json:"to"`
	Tasks    []MovedTask        `json:"tasks" nullable:"false"`
	Unlinked []MoveUnlinkResult `json:"unlinked,omitempty" nullable:"false" doc:"The references that were cut, by ULID"`
	Scoped   *MoveScopedResult  `json:"scoped,omitempty"`
	Resumed  bool               `json:"resumed,omitempty" doc:"The destination was written by an earlier run that stopped; this one only finished the source"`

	// ScopedLines are what the move leaves out of the scoped schema, for the text.
	ScopedLines []string `json:"-"`
}

// MoveUnlinkResult is a reference that was cut, by ULID. Field is the name of a
// relation (blocked_by, causes, ...), sprint or releases.
type MoveUnlinkResult struct {
	Task   string `json:"task"`
	Field  string `json:"field"`
	Target string `json:"target"`
}

// MoveScopedResult is what became of the scoped schema of the projects moved.
type MoveScopedResult struct {
	Carried bool     `json:"carried"`
	Dropped []string `json:"dropped,omitempty" nullable:"false"`
}

// View is the result as the JSON of the command and of the API.
func (r MoveStoreResult) View() MoveStoreResultView {
	v := MoveStoreResultView{From: r.Plan.From, To: r.Plan.To, Tasks: r.Moved, Resumed: r.Resumed, ScopedLines: r.Plan.Scoped.Lines(r.Plan.To)}
	if v.Tasks == nil {
		v.Tasks = []MovedTask{}
	}
	for _, u := range r.Plan.Unlinked {
		v.Unlinked = append(v.Unlinked, MoveUnlinkResult{Task: u.Task.String(), Field: u.Field, Target: u.Target.String()})
	}
	if r.Plan.Scoped != nil {
		v.Scoped = &MoveScopedResult{Carried: r.Carried, Dropped: r.Plan.Scoped.Dropped}
	}
	return v
}
