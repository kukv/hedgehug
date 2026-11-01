package ops

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
)

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func TestTaskViewOf(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	blocker := ts.addTask("blocker", nil)
	task := ts.addTask("the task", func(x *model.Task) {
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
	v := TaskViewOf(ix, e)
	if v.ULID != task.ULID.String() || v.ID != task.ID || v.Title != "the task" || v.Kind != "task" || v.Status != "todo" {
		t.Errorf("view = %+v", v)
	}
	if v.Workspace == nil || *v.Workspace != (WorkspaceRef{ULID: task.Workspace.String(), Name: "me", IDPrefix: "ME"}) {
		t.Errorf("Workspace = %+v", v.Workspace)
	}
	if !reflect.DeepEqual(v.BlockedBy, []Ref{{ULID: blocker.ULID.String(), ID: blocker.ID, Title: "blocker"}}) {
		t.Errorf("BlockedBy = %+v", v.BlockedBy)
	}
	if v.Props["priority"] != "high" {
		t.Errorf("priority = %v; want the scalar", v.Props["priority"])
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
	ts := newTestSession(t)
	a := ts.addTask("first", nil)
	ts.addTask("second", func(x *model.Task) { x.ID = a.ID })
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(a.ULID)
	v := TaskViewOf(ix, e)
	if len(v.Problems) != 1 || v.Problems[0].Code != "duplicate_id" || v.Problems[0].Field != "id" || !strings.Contains(v.Problems[0].Message, a.ID) {
		t.Errorf("Problems = %+v", v.Problems)
	}
}

func TestRefOfUnknownTask(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	u := model.NewULID()
	if got := RefOf(ix, u); got != (Ref{ULID: u.String()}) {
		t.Errorf("RefOf = %+v; want only the ULID", got)
	}
	if got, want := RefName(ix, u), u.String()+" (not found)"; got != want {
		t.Errorf("RefName = %q; want %q", got, want)
	}
}

func TestNames(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	task := ts.addTask("Write", nil)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(task.ULID)
	u := task.ULID.String()
	if got, want := TaskLabel(ix, e), task.ID+" ("+u+")"; got != want {
		t.Errorf("TaskLabel = %q; want %q", got, want)
	}
	if got := TaskFileName(e); got != task.ID {
		t.Errorf("TaskFileName = %q; want %q", got, task.ID)
	}
	if got, want := RefName(ix, task.ULID), task.ID+" ("+u+") Write"; got != want {
		t.Errorf("RefName = %q; want %q", got, want)
	}
	noID := *e
	noID.ID = ""
	if got := TaskLabel(ix, &noID); got != u {
		t.Errorf("TaskLabel without id = %q; want the ULID", got)
	}
	if got := TaskFileName(&noID); got != u {
		t.Errorf("TaskFileName without id = %q; want the ULID", got)
	}
}

func TestOrEmpty(t *testing.T) {
	t.Parallel()
	if got := OrEmpty(nil); got == nil || len(got) != 0 {
		t.Errorf("OrEmpty(nil) = %#v", got)
	}
	in := []string{"a"}
	if got := OrEmpty(in); !reflect.DeepEqual(got, in) {
		t.Errorf("OrEmpty = %v", got)
	}
}
