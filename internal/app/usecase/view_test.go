package usecase_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func TestTaskViewOf(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	blocker := ts.AddTask("blocker", func(x *model.Task) { x.Kind = "project" }) // a project: it needs no parent
	task := ts.AddTask("the task", func(x *model.Task) {
		x.Kind = "project"
		x.Assignees = []string{"alice"}
		x.BlockedBy = []model.ULID{blocker.ULID}
		x.Props.Set("priority", scalar("high"))
		x.Props.Set("relates", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{scalar(blocker.ULID.String())}})
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
	if v.Props["priority"] != "high" {
		t.Errorf("priority = %v; want the scalar", v.Props["priority"])
	}
	// what the text writes
	if v.DisplayID != task.ID || v.ParentRef != nil || len(v.FormerDisplayIDs) != 0 {
		t.Errorf("DisplayID = %q, ParentRef = %+v, FormerDisplayIDs = %v", v.DisplayID, v.ParentRef, v.FormerDisplayIDs)
	}
	if len(v.PropList) != 2 || v.PropList[0].Key != "priority" || v.PropList[0].IsRef || !slices.Equal(v.PropList[0].Values, []string{"high"}) {
		t.Errorf("PropList[0] = %+v", v.PropList)
	}
	if r := v.PropList[1]; r.Key != "relates" || !r.IsRef || len(r.Refs) != 1 || r.Refs[0].ID != blocker.ID {
		t.Errorf("PropList[1] = %+v; want the relates taskref", r)
	}
	if got, want := v.Label(), usecase.TaskLabel(ix, e); got != want {
		t.Errorf("Label = %q; want %q", got, want)
	}
	if got, want := v.BlockedBy[0].Name(), usecase.RefName(ix, blocker.ULID); got != want {
		t.Errorf("Ref.Name = %q; want %q", got, want)
	}
	if !reflect.DeepEqual(v.Props["relates"], []string{blocker.ULID.String()}) {
		t.Errorf("relates = %#v; want a list", v.Props["relates"])
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
	for _, want := range []string{`"workspace":{"ulid":`, `"labels":[]`, `"problems":[]`, `"parent":null`, `"deleted_at":null`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON lacks %s:\n%s", want, b)
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
