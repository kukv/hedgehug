package main

import (
	"strings"
	"testing"
)

func TestUndoNamesTheAuthorOfEachKeptField(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("set", "ME-1", "title=alice")
	te.withOtherWriter("github.com/bob", func() { te.ok("status", "ME-1", "in_progress") })
	te.withOtherWriter("github.com/carol", func() { te.ok("set", "ME-1", "title=carol") })
	out := te.ok("undo")
	for _, want := range []string{"kept title (changed later by github.com/carol)", "kept status (changed later by github.com/bob)"} {
		if !strings.Contains(out, want) {
			t.Errorf("undo lacks %q:\n%s", want, out)
		}
	}
	// carol's later change to the title does not make her the author of the status.
	if strings.Contains(out, "kept status (changed later by github.com/carol)") {
		t.Errorf("undo = %s", out)
	}
}

func TestUndoKeepsAFieldOthersSetBack(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("set", "ME-1", "title=alice")
	te.withOtherWriter("github.com/bob", func() { te.ok("set", "ME-1", "title=x") }) // what it was before alice
	out := te.ok("undo")
	if !strings.Contains(out, "kept title (changed later by github.com/bob)") {
		t.Errorf("undo = %s", out)
	}
	if got := te.ok("show", "ME-1", "--json"); !strings.Contains(got, `"title": "x"`) {
		t.Errorf("show = %s", got)
	}
}

func TestShowJSONFormerIDs(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.ok("add", "p", "--workspace", "web")
	if got := te.ok("show", "WEB-1", "--json"); strings.Contains(got, "former_ids") {
		t.Errorf("former_ids is not omitted when empty: %s", got)
	}
	te.ok("mv", "WEB-1", "--workspace", "api")
	if got := te.ok("show", "API-1", "--json"); !strings.Contains(got, `"former_ids": [`) || !strings.Contains(got, `"WEB-1"`) {
		t.Errorf("show = %s", got)
	}
}
