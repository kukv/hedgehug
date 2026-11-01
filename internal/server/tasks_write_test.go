package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
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
	s, _ := ts.session().OpenStore()
	got, _ := s.Task(mustULID(t, out.Task.ULID))
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
	b := ts.add("b", "")
	var out taskWriteBody
	ts.call("POST", "/api/tasks/"+b+"/move", map[string]any{"parent": a}, 200, &out)
	if out.Task.Parent == nil || out.Commit == nil {
		t.Fatalf("moved = %+v", out)
	}
	ts.call("POST", "/api/tasks/"+b+"/move", map[string]any{"root": true}, 200, &out)
	if out.Task.Parent != nil {
		t.Errorf("parent = %v, want none", *out.Task.Parent)
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

func TestPurgeNeedsConfirm(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.add("p", "")
	c := ts.add("c", p)
	var out affectedBody
	ts.call("DELETE", "/api/tasks/"+p, nil, 200, &out)
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
	ts.call("DELETE", "/api/tasks/"+p+"?purge=true&confirm="+strings.Join(ulids, ","), nil, 200, &out)
	if len(out.ULIDs) != 2 || out.Commit == nil {
		t.Fatalf("purged = %+v", out)
	}
	ts.problem("GET", "/api/tasks/"+c, nil, 404)
}

func TestPurgeLiveTaskIsConflict(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.add("p", "")
	if pr := ts.problem("DELETE", "/api/tasks/"+p+"?purge=true", nil, 409); pr.Type == "purge.unconfirmed" {
		t.Errorf("a live task: %+v, want the refusal of ops", pr)
	}
}

func TestRestoreAtCommit(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var hist []ops.LogView
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

func TestPurgeConfirmNotAULID(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.add("p", "")
	ts.call("DELETE", "/api/tasks/"+p, nil, 200, nil)
	pr := ts.problem("DELETE", "/api/tasks/"+p+"?purge=true&confirm=nope", nil, 409)
	if pr.Type != "purge.unconfirmed" || len(pr.Args["targets"].([]any)) != 1 {
		t.Errorf("confirm that is no ULID: %+v", pr)
	}
}

func TestPurgeFailed(t *testing.T) {
	t.Parallel()
	ix := &index.Index{}
	var ts []*index.Entry
	read := func() (*index.Index, []*index.Entry, error) { return ix, ts, nil }
	// purge.changed asks again, over the tasks as they are now.
	changed := &ops.Error{Kind: ops.KindConflict, Code: "purge.changed", Err: errors.New("changed")}
	if p := purgeFailed(changed, read); p.Type != "purge.unconfirmed" || p.Status != 409 {
		t.Errorf("changed: %+v", p)
	}
	// Failing to read them is that error.
	if p := purgeFailed(changed, func() (*index.Index, []*index.Entry, error) {
		return nil, nil, &ops.Error{Kind: ops.KindNotFound, Code: "task.gone", Err: errors.New("gone")}
	}); p.Type != "task.gone" {
		t.Errorf("read failed: %+v", p)
	}
	// Any other error is its own problem.
	if p := purgeFailed(&ops.Error{Kind: ops.KindNotFound, Code: "task.gone", Err: errors.New("gone")}, read); p.Type != "task.gone" {
		t.Errorf("other: %+v", p)
	}
}

func TestViewsOfGone(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	_, err := ts.srv.viewsOf(ts.srv.session(""), model.NewULID())
	var p *Problem
	if !errors.As(err, &p) || p.Status != 404 || p.Type != "task.gone" {
		t.Errorf("viewsOf of a task not in the index = %v", err)
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
