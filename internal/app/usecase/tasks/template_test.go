package tasks_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

const onboarding = `---
kind: epic
title: Onboarding
labels: [people]
assignees: [github.com/alice]
children:
  - title: Account
    assignees: inherit
    children:
      - title: Laptop
  - title: Mentor
    kind: task
    status: backlog
---
Welcome.
`

// defaultTemplateOfTask makes templates.default of schema.yaml name a template for the kind task.
func defaultTemplateOfTask(name string) func(string) string {
	return func(sc string) string {
		return sc + "templates:\n  default:\n    task: " + name + "\n"
	}
}

func addOf(t *testing.T, ts *usecasetest.Session, in tasks.AddInput) *model.Task {
	t.Helper()
	r, err := tasks.Add(ts.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	return taskIn(t, ts, r.ULID)
}

func taskIn(t *testing.T, ts *usecasetest.Session, u model.ULID) *model.Task {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(u)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// children lists the tasks under u, by rank.
func children(t *testing.T, ts *usecasetest.Session, u model.ULID) []*model.Task {
	t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	var out []*model.Task
	for _, e := range ix.Children(u) {
		out = append(out, taskIn(t, ts, e.ULID))
	}
	return out
}

func titles(ts []*model.Task) []string {
	out := make([]string, len(ts))
	for i, x := range ts {
		out[i] = x.Title
	}
	return out
}

func TestAddExpandsATemplate(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.WriteAndCommit("templates/onboarding.md", onboarding)
	p := ts.AddProject("P") // an epic goes under a project
	head := ts.Head()

	r, err := tasks.Add(ts.Session, tasks.AddInput{Template: "onboarding", Parent: p.String()})
	if err != nil {
		t.Fatal(err)
	}
	if ts.Head() == head || r.Children != 3 {
		t.Fatalf("children = %d", r.Children)
	}
	root := taskIn(t, ts, r.ULID)
	if root.Title != "Onboarding" || root.Kind != "epic" || root.Body != "Welcome.\n" || !slices.Equal(root.Labels, []string{"people"}) {
		t.Fatalf("root = %+v", root)
	}
	kids := children(t, ts, root.ULID)
	if got := titles(kids); !slices.Equal(got, []string{"Account", "Mentor"}) {
		t.Fatalf("children = %v", got)
	}
	if !slices.Equal(kids[0].Assignees, root.Assignees) {
		t.Errorf("inherit: %v, want %v", kids[0].Assignees, root.Assignees)
	}
	if kids[1].Status != "backlog" || kids[1].Kind != "task" {
		t.Errorf("Mentor = %+v", kids[1])
	}
	if got := titles(children(t, ts, kids[0].ULID)); !slices.Equal(got, []string{"Laptop"}) {
		t.Errorf("grandchildren = %v", got)
	}

	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	es, err := usecasetest.LogOf(s.Repo(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if es[1].Hash.String() != head {
		t.Errorf("the template took more than one commit")
	}
	if got := es[0].TrailerValues("Hedgehug-Task"); len(got) != 4 {
		t.Errorf("Hedgehug-Task = %v", got)
	}
}

func TestAddArgumentsWinOverTheTemplate(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.WriteAndCommit("templates/onboarding.md", "---\nkind: epic\ntitle: Onboarding\nlabels: [people]\n---\nWelcome.\n") // no children: the template's own would not fit under a task
	p := ts.AddProject("P")
	got := addOf(t, ts, tasks.AddInput{
		Template: "onboarding", Title: "Mine", Kind: "task", Body: "own\n", Parent: p.String(),
		Assigns: []usecase.Assign{{Key: "labels", Op: "=", Value: "x"}},
	})
	if got.Title != "Mine" || got.Kind != "task" || got.Body != "own\n" || !slices.Equal(got.Labels, []string{"x"}) {
		t.Fatalf("task = %+v", got)
	}
}

func TestAddTakesTheDefaultTemplateOfTheKind(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.WriteAndCommit("templates/bug.md", "---\nlabels: [bug]\nchildren:\n  - title: Reproduce\n---\n")
	ts.EditSchema(defaultTemplateOfTask("bug"))
	p := ts.AddProject("P") // a task goes under a project
	got := addOf(t, ts, tasks.AddInput{Title: "crash", Parent: p.String()})
	if !slices.Equal(got.Labels, []string{"bug"}) {
		t.Fatalf("labels = %v", got.Labels)
	}
	if kids := children(t, ts, got.ULID); len(kids) != 1 {
		t.Fatalf("children = %v", titles(kids))
	}
	plain := addOf(t, ts, tasks.AddInput{Title: "plain", NoTemplate: true, Parent: p.String()})
	if len(plain.Labels) != 0 {
		t.Fatalf("--no-template kept %v", plain.Labels)
	}
}

func TestAddMissingTemplate(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Template: "nope"})
	usecasetest.WantErr(t, err, usecase.KindNotFound, "template.not_found")

	ts.EditSchema(defaultTemplateOfTask("gone"))
	p := ts.AddProject("P")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Parent: p.String()}); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(ts.Warns, func(w string) bool { return strings.Contains(w, "gone") }) {
		t.Errorf("warnings = %v", ts.Warns)
	}
}

func TestAddTemplateNeedsTitles(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.WriteAndCommit("templates/plain.md", "---\nlabels: [x]\n---\n")
	ts.WriteAndCommit("templates/child.md", "---\ntitle: root\nchildren:\n  - labels: [x]\n---\n")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Template: "plain"}); err == nil {
		t.Error("a task with no title was added")
	}
	_, err := tasks.Add(ts.Session, tasks.AddInput{Template: "child"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "template.no_title")
}

func TestAddDropsSprintOfAnotherWorkspace(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("docs", "DOC")
	sp, rel := ts.PutSprintRelease(ts.WorkspaceULID("docs"), "S3", "v9")
	ts.WriteAndCommit("templates/t.md", "---\ntitle: root\nsprint: "+sp.String()+"\nreleases: ["+rel.String()+"]\nchildren:\n  - title: kid\n    sprint: "+sp.String()+"\n---\n")

	r, err := tasks.Add(ts.Session, tasks.AddInput{Template: "t", Workspace: "me"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Write()
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Detached) != 2 || w.Detached[0].Sprint != "S3" || !slices.Equal(w.Detached[0].Releases, []string{"v9"}) || w.Detached[1].Sprint != "S3" {
		t.Fatalf("detached = %+v", w.Detached)
	}
	if w.Detached[0].DisplayID == "" {
		t.Errorf("no id: %+v", w.Detached[0])
	}
	if got := taskIn(t, ts, r.ULID); got.Sprint != nil || len(got.Releases) != 0 {
		t.Fatalf("task = %+v", got)
	}

	// In the workspace of the sprint it stays.
	kept := addOf(t, ts, tasks.AddInput{Template: "t", Workspace: "docs"})
	if kept.Sprint == nil || *kept.Sprint != sp {
		t.Fatalf("sprint = %v", kept.Sprint)
	}
}

func TestAddTemplateInAProjectThatLimitsThem(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("p")
	ts.WriteAndCommit("templates/a.md", "---\nlabels: [a]\n---\n")
	ts.WriteAndCommit("templates/b.md", "---\nlabels: [b]\n---\n")
	ts.EditSchema(func(sc string) string {
		return sc + "scoped:\n  - where: project:" + p.String() + "\n    templates:\n      only: [a]\n      default:\n        task: a\n"
	})
	_, err := tasks.Add(ts.Session, tasks.AddInput{Title: "x", Parent: p.String(), Template: "b"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "template.not_allowed")
	got := addOf(t, ts, tasks.AddInput{Title: "x", Parent: p.String()})
	if !slices.Equal(got.Labels, []string{"a"}) {
		t.Fatalf("labels = %v", got.Labels)
	}
	if outside := addOf(t, ts, tasks.AddInput{Title: "y", Template: "b"}); !slices.Equal(outside.Labels, []string{"b"}) {
		t.Fatalf("outside the project: %v", outside.Labels)
	}
}

func TestUndoTakesTheExpansionBack(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.WriteAndCommit("templates/onboarding.md", onboarding)
	p := ts.AddProject("P") // an epic goes under a project
	before := len(ts.TaskULIDs("me"))
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Template: "onboarding", Parent: p.String()}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if got := len(ts.TaskULIDs("me")); got != before {
		t.Fatalf("%d tasks, want %d", got, before)
	}
}

// A template's four relations and blocked_by go to the task made, and to its children.
func TestAddTemplateGivesTheRelations(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	other := ts.AddProject("Other")
	u := other.String()
	ts.WriteAndCommit("templates/rel.md", "---\nkind: epic\ntitle: Rel\nblocked_by: ["+u+"]\ncauses: ["+u+"]\nrelates: ["+u+"]\nduplicates: ["+u+"]\nclones: ["+u+"]\n"+
		"children:\n  - {title: Child, causes: ["+u+"], relates: ["+u+"], duplicates: ["+u+"], clones: ["+u+"]}\n---\n")
	p := ts.AddProject("P")
	r, err := tasks.Add(ts.Session, tasks.AddInput{Template: "rel", Parent: p.String()})
	if err != nil {
		t.Fatal(err)
	}
	root := taskIn(t, ts, r.ULID)
	kids := children(t, ts, root.ULID)
	if len(kids) != 1 {
		t.Fatalf("children = %v", titles(kids))
	}
	for name, x := range map[string]*model.Task{"root": root, "child": kids[0]} {
		for _, rel := range schema.Relations {
			if name == "child" && rel.Name == "blocked_by" {
				continue
			}
			if got := rel.Get(x); !slices.Equal(got, []model.ULID{other}) {
				t.Errorf("%s %s = %v, want [%s]", name, rel.Name, got, u)
			}
		}
	}
}
