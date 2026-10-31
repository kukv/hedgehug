package index

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestOpenIndexesTasks(t *testing.T) {
	s, _ := newStore(t)
	epic := newTask("Epic")
	epic.Kind = "epic"
	first, second := newTask("First"), newTask("Second")
	first.Parent, second.Parent = &epic.ULID, &epic.ULID
	first.Rank, second.Rank = "a1", "a0"
	first.Labels = []string{"backend"}
	first.Body = "Some BODY text\n"
	first.Props.Set("priority", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "high"})
	put(t, s, epic, first, second)

	ix := mustOpen(t, s)
	e, ok := ix.Task(first.ULID)
	if !ok {
		t.Fatal("first is not in the index")
	}
	if e.Title != "First" || e.Kind != "task" || e.Parent == nil || *e.Parent != epic.ULID || e.Rank != "a1" || !slices.Equal(e.Labels, []string{"backend"}) {
		t.Errorf("entry = %+v", e)
	}
	if got := e.Prop("priority"); !slices.Equal(got, []string{"high"}) {
		t.Errorf("priority = %v, want [high]", got)
	}
	if !strings.Contains(e.Text, "first") || !strings.Contains(e.Text, "some body text") {
		t.Errorf("text = %q, want the title and body in lower case", e.Text)
	}
	if e.Created.IsZero() || e.Broken != "" || len(e.Violations) != 0 {
		t.Errorf("entry = %+v", e)
	}
	if got := titles(ix.Children(epic.ULID)); !slices.Equal(got, []string{"Second", "First"}) {
		t.Errorf("children = %v, want [Second First] (by rank)", got)
	}
	if got := titles(ix.Roots()); !slices.Equal(got, []string{"Epic"}) {
		t.Errorf("roots = %v, want [Epic]", got)
	}
	if len(ix.Tasks()) != 3 {
		t.Errorf("tasks = %d, want 3", len(ix.Tasks()))
	}
	if ix.Schema() == nil || len(ix.Schema().Statuses) == 0 {
		t.Error("schema not read")
	}
}

func TestOpenBrokenFile(t *testing.T) {
	s, dir := newStore(t)
	ulid := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(ulid), "not a task file\n")
	other := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(other), rawTask(model.NewULID(), "kind: task\ntitle: x\nstatus: todo\n"))

	ix := mustOpen(t, s)
	for _, want := range []model.ULID{ulid, other} {
		e, ok := ix.Task(want)
		if !ok || e.Broken == "" {
			t.Errorf("entry %s = %+v, want a broken entry", want, e)
		}
	}
	if len(ix.Roots()) != 2 {
		t.Errorf("roots = %d, want the 2 broken entries", len(ix.Roots()))
	}
}

func TestOpenViolations(t *testing.T) {
	s, dir := newStore(t)
	parent := newTask("parent")
	put(t, s, parent)
	unknown, misplaced, orphan := model.NewULID(), model.NewULID(), model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(unknown), rawTask(unknown, "kind: nope\ntitle: x\nstatus: todo\n"))
	// A bug cannot sit under a task.
	commitRaw(t, s, dir, model.TaskPath(misplaced), rawTask(misplaced, "kind: bug\ntitle: x\nstatus: todo\nparent: "+parent.ULID.String()+"\n"))
	// The parent is missing, so its kind is unknown: placement is not checked.
	commitRaw(t, s, dir, model.TaskPath(orphan), rawTask(orphan, "kind: epic\ntitle: x\nstatus: todo\nparent: "+model.NewULID().String()+"\n"))

	ix := mustOpen(t, s)
	check := func(ulid model.ULID, want ...string) {
		t.Helper()
		e, _ := ix.Task(ulid)
		var got []string
		for _, v := range e.Violations {
			got = append(got, v.Code+":"+v.Field)
		}
		if !slices.Equal(got, want) {
			t.Errorf("violations of %s = %v, want %v", e.Title, got, want)
		}
	}
	check(unknown, schema.CodeUnknownKind+":kind") // once, though two checks report it
	check(misplaced, schema.CodePlacement+":parent")
	check(orphan)
	check(parent.ULID)
}

func TestSiblingsWithTheSameRankAreOrderedByID(t *testing.T) {
	s, _ := newStore(t)
	// The ULID order (ten first) differs from the id order.
	ten, nine, other := newTask("ten"), newTask("nine"), newTask("other")
	ten.ID, nine.ID, other.ID = "TST-10", "TST-9", "TST-2"
	ten.Rank, nine.Rank, other.Rank = "a5", "a5", "a9"
	put(t, s, ten, nine, other)

	ix := mustOpen(t, s)
	if got := titles(ix.Roots()); !slices.Equal(got, []string{"nine", "ten", "other"}) {
		t.Errorf("roots = %v", got)
	}
}

// A write through a store the index is open on finds the siblings in the
// index.
func TestWriteReranksThroughTheIndex(t *testing.T) {
	s, _ := newStore(t)
	a, b := newTask("a"), newTask("b")
	a.Rank, b.Rank = "a5", "a9"
	put(t, s, a, b)
	ix := mustOpen(t, s)
	calls := 0
	s.SetSiblings(func(parent *model.ULID, ws model.ULID) ([]model.ULID, error) {
		calls++
		return ix.siblingULIDs(parent, ws)
	})

	b.Rank = "a5"
	put(t, s, b)
	if calls == 0 {
		t.Error("the write did not ask the index for the siblings")
	}
	if b.Rank <= "a5" {
		t.Errorf("b.Rank = %s, want it after a5", b.Rank)
	}
}

// The index shows a task whose parent is missing among the roots, and a
// write treats it as one.
func TestSiblingULIDsTreatsAMissingParentAsTheTopLevel(t *testing.T) {
	s, dir := newStore(t)
	root, child := newTask("root"), newTask("child")
	root.Rank, child.Rank = "a7", "a5"
	put(t, s, root)
	child.Parent = &root.ULID
	put(t, s, child)
	orphan := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(orphan), rawTask(orphan, "kind: task\ntitle: x\nstatus: todo\nrank: a5\nworkspace: "+root.Workspace.String()+"\nparent: "+model.NewULID().String()+"\n"))
	ix := mustOpen(t, s)

	ulids, err := ix.siblingULIDs(nil, root.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ulids, orphan) || !slices.Contains(ulids, root.ULID) || slices.Contains(ulids, child.ULID) {
		t.Errorf("siblings of the top level = %v", ulids)
	}

	root.Rank = "a5"
	put(t, s, root)
	if root.Rank == "a5" {
		t.Error("root shares a rank with the orphan, shown at the top level, but was not reranked")
	}
}
