package usecase

import (
	"strings"
	"testing"
)

// A sibling without a Parent decides the parent and the workspace.
func TestAddNextToSiblingTakesItsPlace(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	me := ts.workspaceULID("me")
	api := ts.workspaceULID("api")
	for _, in := range []AddInput{{Title: "p", Workspace: "me"}, {Title: "k1", Parent: "ME-1"}, {Title: "k2", Parent: "ME-1"}, {Title: "x", Workspace: "api"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	// two workspaces: no Workspace needed, the sibling's is taken
	if _, err := ts.Add(AddInput{Title: "mid", After: "ME-2"}); err != nil {
		t.Fatal(err)
	}
	p := ulidOf(t, ts, "ME-1")
	if got, want := strings.Join(ts.order(me, &p), " "), "ME-2 ME-4 ME-3"; got != want {
		t.Errorf("order under the parent = %s; want %s", got, want)
	}
	if _, err := ts.Add(AddInput{Title: "root", Before: "API-1"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ts.order(api, nil), " "), "API-2 API-1"; got != want {
		t.Errorf("order in the sibling's workspace = %s; want %s", got, want)
	}
	// a Workspace or a Parent that does not fit the sibling is refused
	_, err := ts.Add(AddInput{Title: "t", Workspace: "me", After: "API-1"})
	wantErr(t, err, KindInvalid, "add.position")
	_, err = ts.Add(AddInput{Title: "t", Workspace: "api", After: "ME-2"})
	wantErr(t, err, KindInvalid, "add.position")
	_, err = ts.Add(AddInput{Title: "t", Parent: "ME-1", After: "ME-1"})
	wantErr(t, err, KindInvalid, "add.position")
}
