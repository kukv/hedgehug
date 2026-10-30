package main

import (
	"slices"
	"strings"
	"testing"
)

func TestWorkspaceCommands(t *testing.T) {
	te := newStoreEnv(t) // me, prefix ME
	te.ok("workspace", "add", "api", "--prefix", "API")
	if code, _, errs := te.run("add", "x"); code == 0 || !strings.Contains(errs, "--workspace") {
		t.Errorf("add with two workspaces: %d %s", code, errs)
	}
	a := te.addJSON("a", "--workspace", "api")
	if a.Key != "API-1" {
		t.Errorf("key = %s", a.Key)
	}
	child := te.addJSON("c", "--parent", a.Key)
	if child.Key != "API-2" {
		t.Errorf("child key = %s", child.Key)
	}
	if code, _, _ := te.run("add", "y", "--parent", a.Key, "--workspace", "me"); code == 0 {
		t.Error("a parent in another workspace is accepted")
	}
	out := te.ok("workspace", "prefix", "api", "BE")
	if !strings.Contains(out, "API-1 → BE-1") || !strings.Contains(out, "API-2 → BE-2") {
		t.Errorf("prefix =\n%s", out)
	}
	if got := te.task(a.ID); got.Key != "BE-1" || !slices.Equal(got.FormerKeys, []string{"API-1"}) {
		t.Errorf("after prefix: %s %v", got.Key, got.FormerKeys)
	}
	te.ok("show", "API-1") // a former key still resolves
	te.ok("workspace", "rename", "be", "backend")
	if code, _, errs := te.run("workspace", "rm", "backend"); code == 0 || !strings.Contains(errs, "2 tasks") {
		t.Errorf("rm with tasks: %d %s", code, errs)
	}
	te.ok("workspace", "add", "empty", "--prefix", "EM")
	te.ok("workspace", "rm", "EM")
	if code, _, _ := te.run("workspace", "add", "dup", "--prefix", "BE"); code == 0 {
		t.Error("a duplicate prefix is accepted")
	}
}

func TestAddWithoutWorkspaces(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "rm", "me")
	if code, _, errs := te.run("add", "x"); code == 0 || !strings.Contains(errs, "the store has no workspace; add one with hedgehug workspace add") {
		t.Errorf("add with no workspace: %d %s", code, errs)
	}
}

func TestWorkspacePrefixSame(t *testing.T) {
	te := newStoreEnv(t)
	te.addJSON("a")
	before := len(te.log("me"))
	if out := te.ok("workspace", "prefix", "me", "ME"); !strings.HasPrefix(out, "No changes") || strings.Contains(out, "→") {
		t.Errorf("prefix to the same =\n%s", out)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}
