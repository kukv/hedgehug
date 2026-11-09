package usecase_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func TestTaskViewOf(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.EditSchema(func(s string) string { return s + "props:\n  note: {type: text}\n" })
	blocker := ts.AddTask("blocker", func(x *model.Task) { x.Kind = "project" }) // a project: it needs no parent
	due, _ := model.ParseDate("2026-11-20")
	points := 2.5
	task := ts.AddTask("the task", func(x *model.Task) {
		x.Kind = "project"
		x.Assignees = []string{"alice"}
		x.BlockedBy = []model.ULID{blocker.ULID}
		x.Priority = "high"
		x.Due = &due
		x.Points = &points
		x.Props.Set("note", scalar("hello"))
		x.Relates = []model.ULID{blocker.ULID}
		x.Causes = []model.ULID{blocker.ULID}
	})
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := ix.Task(task.ULID)
	if !ok {
		t.Fatal("task is not in the index")
	}
	v := usecase.TaskViewOf(ix, e)
	if v.ULID != task.ULID.String() || v.ID != task.ID || v.Title != "the task" || v.Kind != "project" || v.Status != "todo" {
		t.Errorf("view = %+v", v)
	}
	if v.Workspace == nil || *v.Workspace != (usecase.WorkspaceRef{ULID: task.Workspace.String(), Name: "me", IDPrefix: "ME", Known: true}) {
		t.Errorf("Workspace = %+v", v.Workspace)
	}
	if !reflect.DeepEqual(v.BlockedBy, []usecase.Ref{{ULID: blocker.ULID.String(), ID: blocker.ID, Title: "blocker", DisplayID: blocker.ID}}) {
		t.Errorf("BlockedBy = %+v", v.BlockedBy)
	}
	if v.Priority == nil || *v.Priority != "high" || v.Due == nil || *v.Due != "2026-11-20" || v.Points == nil || *v.Points != 2.5 || v.Start != nil || v.Estimate != nil {
		t.Errorf("built-ins = %v %v %v %v %v; want high, 2026-11-20, 2.5, no start, no estimate", v.Priority, v.Start, v.Due, v.Estimate, v.Points)
	}
	if v.Props["note"] != "hello" || len(v.Props) != 1 {
		t.Errorf("Props = %v; want the custom note only: the built-ins are not props", v.Props)
	}
	// what the text writes
	if v.DisplayID != task.ID || v.ParentRef != nil || len(v.FormerDisplayIDs) != 0 {
		t.Errorf("DisplayID = %q, ParentRef = %+v, FormerDisplayIDs = %v", v.DisplayID, v.ParentRef, v.FormerDisplayIDs)
	}
	if len(v.PropList) != 1 || v.PropList[0].Key != "note" || !slices.Equal(v.PropList[0].Values, []string{"hello"}) {
		t.Errorf("PropList = %+v; want note only: the relations are not properties", v.PropList)
	}
	wantRef := []usecase.Ref{{ULID: blocker.ULID.String(), ID: blocker.ID, Title: "blocker", DisplayID: blocker.ID}}
	if !reflect.DeepEqual(v.Relates, wantRef) || !reflect.DeepEqual(v.Causes, wantRef) {
		t.Errorf("Relates = %+v, Causes = %+v", v.Relates, v.Causes)
	}
	if v.Duplicates == nil || len(v.Duplicates) != 0 || v.Clones == nil || len(v.Clones) != 0 {
		t.Errorf("Duplicates = %#v, Clones = %#v; want empty lists", v.Duplicates, v.Clones)
	}
	if got, want := v.Label(), usecase.TaskLabel(ix, e); got != want {
		t.Errorf("Label = %q; want %q", got, want)
	}
	if got, want := v.BlockedBy[0].Name(), usecase.RefName(ix, blocker.ULID); got != want {
		t.Errorf("Ref.Name = %q; want %q", got, want)
	}
	if _, ok := v.Props["relates"]; ok {
		t.Errorf("props has relates: %#v", v.Props)
	}
	if !reflect.DeepEqual(v.Assignees, []string{"alice"}) || v.Labels == nil || len(v.Labels) != 0 {
		t.Errorf("Assignees = %v, Labels = %#v", v.Assignees, v.Labels)
	}
	if v.Parent != nil || v.DeletedAt != nil || v.Problems == nil || len(v.Problems) != 0 {
		t.Errorf("Parent %v, DeletedAt %v, Problems %#v", v.Parent, v.DeletedAt, v.Problems)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"workspace":{"ulid":`, `"labels":[]`, `"causes":[{"ulid":`, `"duplicates":[]`, `"clones":[]`, `"problems":[]`, `"parent":null`, `"deleted_at":null`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON lacks %s:\n%s", want, b)
		}
	}
}

// The view has a list for each relation of the table, by its name.
func TestTaskViewOfHasEveryRelation(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	other := ts.AddTask("other", func(x *model.Task) { x.Kind = "project" })
	task := ts.AddTask("the task", func(x *model.Task) {
		x.Kind = "project"
		for _, rel := range schema.Relations {
			rel.Set(x, []model.ULID{other.ULID})
		}
	})
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(task.ULID)
	b, err := json.Marshal(usecase.TaskViewOf(ix, e))
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range schema.Relations {
		var got struct{ Refs []usecase.Ref }
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw[rel.Name], &got.Refs); err != nil || len(got.Refs) != 1 || got.Refs[0].ULID != other.ULID.String() {
			t.Errorf("%s: %s (%v)", rel.Name, raw[rel.Name], err)
		}
	}
}

func TestTaskViewOfProblems(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("first", nil)
	ts.AddTask("second", func(x *model.Task) { x.ID = a.ID })
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(a.ULID)
	v := usecase.TaskViewOf(ix, e)
	if len(v.Problems) != 1 || v.Problems[0].Code != "duplicate_id" || v.Problems[0].Field != "id" || !strings.Contains(v.Problems[0].Message, a.ID) {
		t.Errorf("Problems = %+v", v.Problems)
	}
}

func TestRefOfUnknownTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	u := model.NewULID()
	if got := usecase.RefOf(ix, u); got != (usecase.Ref{ULID: u.String(), Missing: true}) {
		t.Errorf("RefOf = %+v; want only the ULID, and Missing", got)
	}
	if got, want := usecase.RefOf(ix, u).Name(), u.String()+" (not found)"; got != want {
		t.Errorf("Ref.Name = %q; want %q", got, want)
	}
	if got, want := usecase.RefName(ix, u), u.String()+" (not found)"; got != want {
		t.Errorf("RefName = %q; want %q", got, want)
	}
}

func TestNames(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	task := ts.AddTask("Write", nil)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(task.ULID)
	u := task.ULID.String()
	if got, want := usecase.TaskLabel(ix, e), task.ID+" ("+u+")"; got != want {
		t.Errorf("TaskLabel = %q; want %q", got, want)
	}
	if got := usecase.TaskFileName(e); got != task.ID {
		t.Errorf("TaskFileName = %q; want %q", got, task.ID)
	}
	if got, want := usecase.RefName(ix, task.ULID), task.ID+" ("+u+") Write"; got != want {
		t.Errorf("RefName = %q; want %q", got, want)
	}
	noID := *e
	noID.ID = ""
	if got := usecase.TaskLabel(ix, &noID); got != u {
		t.Errorf("TaskLabel without id = %q; want the ULID", got)
	}
	if got := usecase.TaskFileName(&noID); got != u {
		t.Errorf("TaskFileName without id = %q; want the ULID", got)
	}
}

func TestOrEmpty(t *testing.T) {
	t.Parallel()
	if got := usecase.OrEmpty(nil); got == nil || len(got) != 0 {
		t.Errorf("OrEmpty(nil) = %#v", got)
	}
	in := []string{"a"}
	if got := usecase.OrEmpty(in); !reflect.DeepEqual(got, in) {
		t.Errorf("OrEmpty = %v", got)
	}
}
