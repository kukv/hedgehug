package main

import (
	"strings"
	"testing"
)

func TestDep(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.ok("dep", "add", a.Short, "--blocked-by", b.Short)
	if bb := te.task(a.ID).BlockedBy; len(bb) != 1 || bb[0].String() != b.ID {
		t.Errorf("blocked_by = %v", bb)
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: dep") {
		t.Error("dep add is not a dep commit")
	}
	if code, _, stderr := te.run("dep", "add", a.Short, "--blocked-by", b.Short); code != 1 || !strings.Contains(stderr, "already") {
		t.Errorf("a second dep add: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := te.run("dep", "add", b.Short, "--blocked-by", a.Short); code != 1 || !strings.Contains(stderr, "leads back") {
		t.Errorf("a cycle: exit %d, stderr %q", code, stderr)
	}
	te.ok("dep", "rm", a.Short, "--blocked-by", b.Short)
	if bb := te.task(a.ID).BlockedBy; len(bb) != 0 {
		t.Errorf("blocked_by = %v after dep rm", bb)
	}
	if code, _, stderr := te.run("dep", "rm", a.Short, "--blocked-by", b.Short); code != 1 || !strings.Contains(stderr, "not blocked") {
		t.Errorf("dep rm of nothing: exit %d, stderr %q", code, stderr)
	}
	if code, _, _ := te.run("dep", "add", a.Short); code != 2 {
		t.Errorf("dep add with no --blocked-by: exit %d, want 2", code)
	}
}
