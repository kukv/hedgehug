package server

import (
	"slices"
	"testing"
)

func TestPeopleAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one", "assignees": []string{"zed", "bob"}}, 201, &out)
	ts.call("PATCH", "/api/tasks/"+out.Task.ID, map[string]any{"watchers": []string{"carol", "bob"}}, 200, &out)

	var people []string
	ts.call("GET", "/api/people", nil, 200, &people)
	want := []string{"bob", "carol", "github.com/alice", "zed"}
	if !slices.Equal(people, want) {
		t.Errorf("people = %q; want %q", people, want)
	}
	ts.call("GET", "/api/people?store=me", nil, 200, &people)
	if !slices.Equal(people, want) {
		t.Errorf("people of me = %q; want %q", people, want)
	}
	if p := ts.problem("GET", "/api/people?store=nope", nil, 404); p.Type == "" {
		t.Errorf("unknown store: %+v", p)
	}
}
