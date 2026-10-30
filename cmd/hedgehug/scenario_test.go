package main

import (
	"strings"
	"testing"
	"time"
)

// TestScenario runs the phase 1 done criterion for the CLI: in one store,
// every CRUD operation and the tree work, and each is one commit.
func TestScenario(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me")
	step := func(op string, args ...string) {
		t.Helper()
		before := len(te.log("me"))
		te.now = te.now.Add(time.Second)
		te.ok(args...)
		log := te.log("me")
		if len(log) != before+1 {
			t.Fatalf("%v: commits %d -> %d, want one more", args, before, len(log))
		}
		if !strings.Contains(log[0].Message, "Hedgehug-Op: "+op+"\n") && !strings.HasSuffix(log[0].Message, "Hedgehug-Op: "+op) {
			t.Fatalf("%v: last commit %q, want op %s", args, log[0].Message, op)
		}
	}

	p := te.addJSON("Website", "--kind", "project")
	c := te.addJSON("Login page", "--parent", p.Short)
	g := te.addJSON("Form", "--parent", c.Short)
	o := te.addJSON("Other")

	tree := te.ok("ls", "--tree", "--no-color")
	for _, s := range []string{"Website", "└─ ", "Form", "Other"} {
		if !strings.Contains(tree, s) {
			t.Fatalf("tree lacks %q:\n%s", s, tree)
		}
	}
	step("set", "set", g.Short, "labels+=ui", "due=2026-11-01")
	step("status", "status", g.Short, "in_progress")
	step("status", "done", g.Short)
	step("mv", "mv", o.Short, "--parent", p.Short, "--before", c.Short)
	step("dep", "dep", "add", c.Short, "--blocked-by", o.Short)
	step("dep", "dep", "rm", c.Short, "--blocked-by", o.Short)
	step("rm", "rm", p.Short)
	step("restore", "restore", p.Short)
	step("rm", "rm", p.Short)
	step("purge", "purge", p.Short, "--yes")
	if out := te.ok("ls", "is:deleted", "--ids-only"); out != "" {
		t.Errorf("deleted tasks remain after purge: %q", out)
	}
	// store init, four adds and the ten steps.
	if n := len(te.log("me")); n != 15 {
		t.Errorf("commits = %d, want 15", n)
	}
	for _, c := range te.log("me") {
		if c.Author.Email != "alice@example.com" {
			t.Errorf("commit %s by %s, want alice", c.Hash, c.Author.Email)
		}
	}
}
