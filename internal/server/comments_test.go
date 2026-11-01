package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

type commentBody struct {
	Comment struct {
		ULID string `json:"ulid"`
		Body string `json:"body"`
	} `json:"comment"`
	Commit *string `json:"commit"`
}

func TestComments(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("one", "")
	var out commentBody
	ts.call("POST", "/api/tasks/"+id+"/comments", map[string]any{"body": "first"}, 201, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Comment.ULID == "" || out.Comment.Body != "first\n" || out.Commit == nil {
		t.Fatalf("added = %+v", out)
	}
	cid := out.Comment.ULID
	ts.call("PATCH", "/api/tasks/"+id+"/comments/"+cid, map[string]any{"body": "edited"}, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Comment.Body != "edited\n" {
		t.Errorf("edited = %+v", out)
	}
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show) //nolint:bodyclose // raw reads the body into memory
	if len(show.Comments) != 1 || show.Comments[0].Body != "edited\n" {
		t.Errorf("comments = %+v", show.Comments)
	}
	// A comment of another task is not found under this one.
	other := ts.add("two", "")
	if p := ts.problem("PATCH", "/api/tasks/"+other+"/comments/"+cid, map[string]any{"body": "x"}, 404); p.Type != "comment.not_found" {
		t.Errorf("other task: %+v", p)
	}
	ts.problem("DELETE", "/api/tasks/"+other+"/comments/"+cid, nil, 404)
	ts.call("DELETE", "/api/tasks/"+id+"/comments/"+cid, nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	if out.Comment.Body != "edited\n" || out.Commit == nil {
		t.Errorf("removed = %+v", out)
	}
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show) //nolint:bodyclose // raw reads the body into memory
	if len(show.Comments) != 0 {
		t.Errorf("comments after delete = %+v", show.Comments)
	}
	ts.problem("POST", "/api/tasks/"+id+"/comments", map[string]any{"body": ""}, 400)
	ts.problem("POST", "/api/tasks/ME-99/comments", map[string]any{"body": "x"}, 404)
}

func TestWatchMe(t *testing.T) {
	ts := newTestServer(t)
	ts.withAccount()
	id := ts.add("one", "")
	var out taskWriteBody
	ts.call("PUT", "/api/tasks/"+id+"/watchers/me", nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show) //nolint:bodyclose // raw reads the body into memory
	if len(show.Watchers) != 1 {
		t.Fatalf("watchers = %v", show.Watchers)
	}
	ts.call("DELETE", "/api/tasks/"+id+"/watchers/me", nil, 200, &out) //nolint:bodyclose // raw reads the body into memory
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)                  //nolint:bodyclose // raw reads the body into memory
	if len(show.Watchers) != 0 {
		t.Errorf("watchers = %v", show.Watchers)
	}
}
