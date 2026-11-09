package server

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

type taskWriteBody struct {
	Task   *usecase.TaskView `json:"task"`
	Commit *string           `json:"commit"`
	Note   string            `json:"note"`
}

type affectedBody struct {
	Tasks  []*usecase.TaskView `json:"tasks"`
	Commit *string             `json:"commit"`
}

func TestCreateTask(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one", "labels": []string{"a", "b"}}, 201, &out)
	if out.Task == nil || out.Task.ID != "ME-1" || out.Commit == nil || len(out.Task.Labels) != 2 {
		t.Fatalf("created = %+v", out)
	}
	ts.call("POST", "/api/tasks", map[string]any{"title": "p", "kind": "project"}, 201, &out)
	if out.Task.Kind != "project" {
		t.Errorf("kind = %q", out.Task.Kind)
	}
	ts.call("POST", "/api/tasks", map[string]any{"title": "c", "parent": "ME-2"}, 201, &out)
	if out.Task.Parent == nil {
		t.Errorf("parent = nil")
	}
	ts.problem("POST", "/api/tasks", map[string]any{}, 400)
}

func TestCreateTaskBeforeAfter(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	a := ts.add("a", "")
	b := ts.add("b", "")
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "after a", "after": a}, 201, &out)
	if out.Commit == nil {
		t.Fatalf("created = %+v", out)
	}
	mid := out.Task.ID
	ts.call("POST", "/api/tasks", map[string]any{"title": "before b", "before": b}, 201, &out)
	last := out.Task.ID
	var list listBody
	ts.call("GET", "/api/tasks", nil, 200, &list)
	var got []string
	for _, x := range list.Tasks {
		got = append(got, x.ID)
	}
	if want := []string{a, mid, last, b}; !slices.Equal(got, want) {
		t.Errorf("order = %v; want %v", got, want)
	}
	p := ts.problem("POST", "/api/tasks", map[string]any{"title": "x", "parent": a, "after": b}, 422)
	if p.Type != "add.position" {
		t.Errorf("type = %q; want add.position", p.Type)
	}
	p = ts.problem("POST", "/api/tasks", map[string]any{"title": "x", "before": a, "after": b}, 400)
	if p.Type != "position.both" {
		t.Errorf("type = %q; want position.both", p.Type)
	}
	p = ts.problem("POST", "/api/tasks/"+b+"/move", map[string]any{"before": a, "after": a}, 400)
	if p.Type != "position.both" {
		t.Errorf("move: type = %q; want position.both", p.Type)
	}

	// under a parent, next to one of its children
	k1 := ts.add("k1", a)
	ts.add("k2", a)
	ts.call("POST", "/api/tasks", map[string]any{"title": "k3", "parent": a, "after": k1}, 201, &out)
	if out.Task.Parent == nil {
		t.Fatalf("created = %+v", out)
	}
	k3Parent := *out.Task.Parent
	ts.call("GET", "/api/tasks", nil, 200, &list)
	got = nil
	for _, x := range list.Tasks {
		if x.Parent != nil {
			got = append(got, x.Title)
		}
	}
	if want := []string{"k1", "k3", "k2"}; !slices.Equal(got, want) {
		t.Errorf("children = %v; want %v", got, want)
	}

	// no parent and no workspace: the sibling's
	ts.call("POST", "/api/tasks", map[string]any{"title": "k4", "after": k1}, 201, &out)
	if out.Task.Parent == nil || out.Task.Workspace == nil {
		t.Fatalf("created = %+v", out)
	}
	if *out.Task.Parent != k3Parent {
		t.Errorf("parent = %v; want %s", *out.Task.Parent, k3Parent)
	}
	ts.call("GET", "/api/tasks", nil, 200, &list)
	got = nil
	for _, x := range list.Tasks {
		if x.Parent != nil {
			got = append(got, x.Title)
		}
	}
	if want := []string{"k1", "k4", "k3", "k2"}; !slices.Equal(got, want) {
		t.Errorf("children = %v; want %v", got, want)
	}
}

func TestCreateTaskWithBodyAndRepos(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{
		"title": "spec", "body": "line one\n\nline two\n", "repos": []string{"github.com/o/r"},
	}, 201, &out)
	if out.Task == nil || out.Commit == nil {
		t.Fatalf("created = %+v", out)
	}
	got := ts.uc.TaskByULID("me", mustULID(t, out.Task.ULID))
	if got.Body != "line one\n\nline two\n" || len(got.Repos) != 1 || got.Repos[0] != "github.com/o/r" {
		t.Errorf("body = %q, repos = %q", got.Body, got.Repos)
	}
}

// The four relations and blocked_by are patched as lists of IDs, and come back
// as lists of tasks at the top level of the task.
func TestPatchRelations(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	other := ts.add("other", "")
	third := ts.add("third", "")
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{
		"blocked_by": []string{other}, "causes": []string{other, third}, "relates": []string{third}, "duplicates": []string{other}, "clones": []string{third},
	}, 200, &out)
	ids := func(rs []usecase.Ref) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.ID)
		}
		return out
	}
	t0 := out.Task
	if !slices.Equal(ids(t0.BlockedBy), []string{other}) || !slices.Equal(ids(t0.Causes), []string{other, third}) || !slices.Equal(ids(t0.Relates), []string{third}) ||
		!slices.Equal(ids(t0.Duplicates), []string{other}) || !slices.Equal(ids(t0.Clones), []string{third}) {
		t.Fatalf("patched = %+v", t0)
	}
	if len(t0.Props) != 0 {
		t.Errorf("props = %v; a relation is not a property", t0.Props)
	}
	// An empty list takes them off.
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"causes": []string{}, "clones": []string{}}, 200, &out)
	if len(out.Task.Causes) != 0 || len(out.Task.Clones) != 0 || len(out.Task.Relates) != 1 {
		t.Errorf("after the empty lists = %+v", out.Task)
	}
	// A relation to the task itself is refused, as is a task that is not there.
	p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{"relates": []string{id}}, 422)
	if p.Type != "schema.violation" || !strings.Contains(p.Detail, "自分自身は指せません") {
		t.Errorf("relates to itself: %+v", p)
	}
	ts.problem("PATCH", "/api/tasks/"+id, map[string]any{"duplicates": []string{"ME-999"}}, 404)
}

// A task is created without relations: they are put on it after.
func TestCreateTaskRefusesRelationsInProps(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	for _, rel := range schema.Relations {
		p := ts.problem("POST", "/api/tasks", map[string]any{"title": "t", "props": map[string]any{rel.Name: "ME-1"}}, 400)
		if p.Type != "request.invalid" || !strings.Contains(p.Detail, "関連は作成後に付けます") {
			t.Errorf("props.%s: %+v", rel.Name, p)
		}
	}
}

func TestPatchTask(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	blocker := ts.add("blocker", "")
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{
		"title": "two", "labels": []string{"x"}, "blocked_by": []string{blocker}, "body": "hello\n",
	}, 200, &out)
	if out.Task.Title != "two" || len(out.Task.Labels) != 1 || len(out.Task.BlockedBy) != 1 || out.Commit == nil {
		t.Fatalf("patched = %+v", out)
	}
	got := ts.uc.TaskByULID("me", mustULID(t, out.Task.ULID))
	if got.Body != "hello\n" {
		t.Errorf("body = %q", got.Body)
	}
	// The same values again: no commit.
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &out)
	if out.Commit != nil {
		t.Errorf("no change made commit %v", *out.Commit)
	}
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"labels": []string{}}, 200, &out)
	if len(out.Task.Labels) != 0 {
		t.Errorf("labels = %v, want none", out.Task.Labels)
	}
	if p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{}, 400); p.Type != "patch.empty" {
		t.Errorf("empty patch: %+v", p)
	}
	if p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{"status": "nope"}, 422); p.Type == "" {
		t.Errorf("bad status: %+v", p)
	}
	if p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{"props": map[string]any{"nope": "x"}}, 422); p.Type != "assign.unknown_key" {
		t.Errorf("unknown prop: %+v", p)
	}
}

func TestMoveTask(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	a := ts.add("a", "")
	c := ts.add("c", "")
	b := ts.add("b", a) // a task under the project a
	var out taskWriteBody
	ts.call("POST", "/api/tasks/"+b+"/move", map[string]any{"parent": c}, 200, &out)
	if out.Task.Parent == nil || out.Commit == nil {
		t.Fatalf("moved = %+v", out)
	}
	// only a project sits at the root
	if p := ts.problem("POST", "/api/tasks/"+b+"/move", map[string]any{"root": true}, 422); p.Type != "schema.violation" {
		t.Errorf("a task to the root: %+v", p)
	}
	ts.problem("POST", "/api/tasks/"+b+"/move", map[string]any{"root": true, "parent": a}, 400)
}

func TestRemoveRestoreTask(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.add("p", "")
	ts.add("c", p)
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p, nil, 200, &out)
	if len(out.Tasks) != 2 || out.Tasks[0].DeletedAt == nil || out.Commit == nil {
		t.Fatalf("removed = %+v", out)
	}
	if pr := ts.problem("PATCH", "/api/tasks/"+p, map[string]any{"title": "x"}, 409); pr.Type != "task.deleted" {
		t.Errorf("patch of deleted: %+v", pr)
	}
	ts.call("POST", "/api/tasks/"+p+"/restore", nil, 200, &out)
	if len(out.Tasks) != 2 || out.Tasks[0].DeletedAt != nil {
		t.Errorf("restored = %+v", out)
	}
}

// The API has no purge: ?purge=true is not a parameter, so the call is the
// plain delete of hh rm and the task stays, deleted.
func TestDeleteTaskHasNoPurge(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.add("p", "")
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p+"?purge=true", nil, 200, &out)
	if len(out.Tasks) != 1 || out.Tasks[0].DeletedAt == nil || out.Commit == nil {
		t.Fatalf("delete = %+v, want the task deleted", out)
	}
	var got usecase.TaskView
	ts.call("GET", "/api/tasks/"+p, nil, 200, &got)
	if got.DeletedAt == nil {
		t.Errorf("the task is %+v, want it still there, deleted", got)
	}
}

func TestRestoreAtCommit(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var hist []usecase.LogView
	ts.call("GET", "/api/tasks/"+id+"/history", nil, 200, &hist)
	first := hist[0].Hash
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &out)
	ts.call("POST", "/api/tasks/"+id+"/history/"+first+"/restore", nil, 200, &out)
	if out.Task.Title != "one" || out.Commit == nil {
		t.Errorf("restored at = %+v", out)
	}
}

func TestAssignsOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    any
		want []usecase.Assign
	}{
		{"x", []usecase.Assign{{Key: "k", Op: "=", Value: "x"}}},
		{float64(3), []usecase.Assign{{Key: "k", Op: "=", Value: "3"}}},
		{true, []usecase.Assign{{Key: "k", Op: "=", Value: "true"}}},
		{nil, []usecase.Assign{{Key: "k", Op: "=", Value: ""}}},
		{[]any{"a", "b"}, []usecase.Assign{{Key: "k", Op: "=", Value: ""}, {Key: "k", Op: "+=", Value: "a"}, {Key: "k", Op: "+=", Value: "b"}}},
		{[]any{}, []usecase.Assign{{Key: "k", Op: "=", Value: ""}}},
	}
	for _, c := range cases {
		got, err := assignsOf("k", c.v)
		if err != nil || !slicesEqual(got, c.want) {
			t.Errorf("assignsOf(%v) = %v, %v; want %v", c.v, got, err, c.want)
		}
	}
	if _, err := assignsOf("k", map[string]any{}); err == nil {
		t.Error("an object is refused")
	}
}

func TestWriteOutGone(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	_, err := writeOut(ts.uc.GoneTaskResult())
	var p *Problem
	if !errors.As(err, &p) || p.Status != 404 || p.Type != "task.gone" {
		t.Errorf("the answer to a write to a task not in the index = %v", err)
	}
}

func TestPropsRefuseTaskKeys(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	for _, k := range append(model.TaskKeys(), "blocked_by") {
		if p := ts.problem("PATCH", "/api/tasks/"+id, map[string]any{"props": map[string]any{k: "x"}}, 400); p.Type != "request.invalid" {
			t.Errorf("props.%s: %+v", k, p)
		}
	}
	ts.problem("POST", "/api/tasks", map[string]any{"title": "t", "props": map[string]any{"title": "x"}}, 400)
}

// POST reads repos as PATCH does: a repo given twice is kept once, and an
// empty one is refused.
func TestCreateTaskReposAsPatchReadsThem(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "a", "repos": []string{"github.com/o/r", "github.com/o/r", "github.com/o/s"}}, 201, &out)
	got := ts.uc.TaskByULID("me", mustULID(t, out.Task.ULID))
	if want := []string{"github.com/o/r", "github.com/o/s"}; !slices.Equal(got.Repos, want) {
		t.Errorf("repos = %q; want %q", got.Repos, want)
	}
	patched := ts.problem("PATCH", "/api/tasks/"+out.Task.ID, map[string]any{"repos": []string{"github.com/o/r", ""}}, 422)
	created := ts.problem("POST", "/api/tasks", map[string]any{"title": "b", "repos": []string{"github.com/o/r", ""}}, 422)
	if created.Type != patched.Type {
		t.Errorf("an empty repo: POST %q, PATCH %q; want the same", created.Type, patched.Type)
	}
}
