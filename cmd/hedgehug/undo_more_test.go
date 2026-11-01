package main

import (
	"strings"
	"testing"
)

func TestUndoMoveTakesTheSubtree(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.ok("add", "p", "--workspace", "web")
	te.ok("add", "c", "--parent", "WEB-1")
	te.goldenOK("mv", "mv", "WEB-1", "--workspace", "api") // API-1 and API-2
	te.goldenOK("undo", "undo")
	for _, id := range []string{"API-1", "API-2"} {
		got := te.ok("show", id, "--json")
		if !strings.Contains(got, `"name": "web"`) || strings.Contains(got, `"id": "API-`) {
			t.Errorf("%s is not back in web: %s", id, got)
		}
	}
}

func TestUndoComment(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("comment", "ME-1", "hello")
	if got := te.ok("show", "ME-1", "--json"); !strings.Contains(got, "hello") {
		t.Fatalf("no comment: %s", got)
	}
	te.goldenOK("undo", "undo")
	if got := te.ok("show", "ME-1", "--json"); strings.Contains(got, "hello") {
		t.Errorf("the comment is still there: %s", got)
	}
}

func TestUndoAddKeepsTaskOthersChanged(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.withOtherWriter("github.com/bob", func() { te.ok("comment", "ME-1", "from bob") }) // bob becomes a watcher too
	te.goldenOK("undo", "undo")
	if got := te.ok("show", "ME-1", "--json"); !strings.Contains(got, "from bob") {
		t.Errorf("the task or its comment is gone: %s", got)
	}
}

func TestUndoWorkspaceWithTasksIsRefused(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.withOtherWriter("github.com/bob", func() { te.ok("add", "x", "--workspace", "api") })
	before := te.headCommit("me").Hash
	te.goldenRun("undo", 1, "undo")
	if te.headCommit("me").Hash != before {
		t.Error("a refused undo committed")
	}
}
