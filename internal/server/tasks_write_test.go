package server

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

type taskWriteBody struct {
	Task   *ops.TaskView `json:"task"`
	Commit *string       `json:"commit"`
	Note   string        `json:"note"`
}

type affectedBody struct {
	Tasks  []*ops.TaskView `json:"tasks"`
	ULIDs  []string        `json:"ulids"`
	Commit *string         `json:"commit"`
}

func TestCreateTask(t *testing.T) {
	ts := newTestServer(t)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one", "labels": []string{"a", "b"}}, 201, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Task == nil || out.Task.ID != "ME-1" || out.Commit == nil || len(out.Task.Labels) != 2 {
		t.Fatalf("created = %+v", out)
	}
	ts.call("POST", "/api/tasks", map[string]any{"title": "p", "kind": "project"}, 201, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Task.Kind != "project" {
		t.Errorf("kind = %q", out.Task.Kind)
	}
	ts.call("POST", "/api/tasks", map[string]any{"title": "c", "parent": "ME-2"}, 201, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Task.Parent == nil {
		t.Errorf("parent = nil")
	}
	ts.problem("POST", "/api/tasks", map[string]any{}, 400)
}

func TestPatchTask(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	blocker := ts.add("blocker", "")
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{ //nolint:bodyclose // raw reads the body into memory
		"title": "two", "labels": []string{"x"}, "blocked_by": []string{blocker}, "body": "hello\n",
	}, 200, &out)
	if out.Task.Title != "two" || len(out.Task.Labels) != 1 || len(out.Task.BlockedBy) != 1 || out.Commit == nil {
		t.Fatalf("patched = %+v", out)
	}
	s, _ := ts.session().OpenStore()
	got, _ := s.Task(mustULID(t, out.Task.ULID))
	if got.Body != "hello\n" {
		t.Errorf("body = %q", got.Body)
	}
	// The same values again: no commit.
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Commit != nil {
		t.Errorf("no change made commit %v", *out.Commit)
	}
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"labels": []string{}}, 200, &out) //nolint:bodyclose // raw reads the body into memory
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
	ts := newTestServer(t)
	a := ts.add("a", "")
	b := ts.add("b", "")
	var out taskWriteBody
	ts.call("POST", "/api/tasks/"+b+"/move", map[string]any{"parent": a}, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Task.Parent == nil || out.Commit == nil {
		t.Fatalf("moved = %+v", out)
	}
	ts.call("POST", "/api/tasks/"+b+"/move", map[string]any{"root": true}, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Task.Parent != nil {
		t.Errorf("parent = %v, want none", *out.Task.Parent)
	}
	ts.problem("POST", "/api/tasks/"+b+"/move", map[string]any{"root": true, "parent": a}, 400)
}

func TestRemoveRestoreTask(t *testing.T) {
	ts := newTestServer(t)
	p := ts.add("p", "")
	ts.add("c", p)
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p, nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if len(out.Tasks) != 2 || out.Tasks[0].DeletedAt == nil || out.Commit == nil {
		t.Fatalf("removed = %+v", out)
	}
	if pr := ts.problem("PATCH", "/api/tasks/"+p, map[string]any{"title": "x"}, 409); pr.Type != "task.deleted" {
		t.Errorf("patch of deleted: %+v", pr)
	}
	ts.call("POST", "/api/tasks/"+p+"/restore", nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if len(out.Tasks) != 2 || out.Tasks[0].DeletedAt != nil {
		t.Errorf("restored = %+v", out)
	}
}

func TestPurgeNeedsConfirm(t *testing.T) {
	ts := newTestServer(t)
	p := ts.add("p", "")
	c := ts.add("c", p)
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p, nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	pr := ts.problem("DELETE", "/api/tasks/"+p+"?purge=true", nil, 409)
	targets, _ := pr.Args["targets"].([]any)
	if pr.Type != "purge.unconfirmed" || len(targets) != 2 {
		t.Fatalf("unconfirmed = %+v", pr)
	}
	var ulids []string
	for _, x := range targets {
		m := x.(map[string]any)
		ulids = append(ulids, m["ulid"].(string))
		if m["id"] == "" || m["title"] == "" {
			t.Errorf("target = %v", m)
		}
	}
	// A wrong list is refused with the targets as they are.
	pr = ts.problem("DELETE", "/api/tasks/"+p+"?purge=true&confirm="+ulids[0], nil, 409)
	if pr.Type != "purge.unconfirmed" {
		t.Errorf("partial confirm: %+v", pr)
	}
	ts.call("DELETE", "/api/tasks/"+p+"?purge=true&confirm="+strings.Join(ulids, ","), nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if len(out.ULIDs) != 2 || out.Commit == nil {
		t.Fatalf("purged = %+v", out)
	}
	ts.problem("GET", "/api/tasks/"+c, nil, 404)
}

func TestPurgeLiveTaskIsConflict(t *testing.T) {
	ts := newTestServer(t)
	p := ts.add("p", "")
	if pr := ts.problem("DELETE", "/api/tasks/"+p+"?purge=true", nil, 409); pr.Type == "purge.unconfirmed" {
		t.Errorf("a live task: %+v, want the refusal of ops", pr)
	}
}

func TestRestoreAtCommit(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	var hist []ops.LogView
	ts.call("GET", "/api/tasks/"+id+"/history", nil, 200, &hist) //nolint:bodyclose // raw reads the body into memory
	first := hist[0].Hash
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &out)  //nolint:bodyclose // raw reads the body into memory
	ts.call("POST", "/api/tasks/"+id+"/history/"+first+"/restore", nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Task.Title != "one" || out.Commit == nil {
		t.Errorf("restored at = %+v", out)
	}
}

func TestAssignsOf(t *testing.T) {
	cases := []struct {
		v    any
		want []ops.Assign
	}{
		{"x", []ops.Assign{{Key: "k", Op: "=", Value: "x"}}},
		{float64(3), []ops.Assign{{Key: "k", Op: "=", Value: "3"}}},
		{true, []ops.Assign{{Key: "k", Op: "=", Value: "true"}}},
		{nil, []ops.Assign{{Key: "k", Op: "=", Value: ""}}},
		{[]any{"a", "b"}, []ops.Assign{{Key: "k", Op: "=", Value: ""}, {Key: "k", Op: "+=", Value: "a"}, {Key: "k", Op: "+=", Value: "b"}}},
		{[]any{}, []ops.Assign{{Key: "k", Op: "=", Value: ""}}},
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
