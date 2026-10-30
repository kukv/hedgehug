package main

import (
	"encoding/json"
	"slices"
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
	// The editor writes its contents in turn: the task for edit, then the
	// body for comment edit.
	te.editWith(setLine(te.raw(g.ID), "title", "Form fields"), "second\n")
	step("edit", "edit", g.Short)
	step("comment", "comment", g.Short, "first")
	cm := te.comments(g.ID)
	if len(cm) != 1 {
		t.Fatalf("comments = %d, want 1", len(cm))
	}
	cid := cm[0].ID.String()[len(cm[0].ID.String())-6:]
	step("comment", "comment", "edit", cid)
	if cs := te.comments(g.ID); len(cs) != 1 || cs[0].Body != "second\n" {
		t.Fatalf("comments after edit = %+v", cs)
	}
	out := te.ok("show", g.Short)
	for _, w := range []string{"Form fields", cid[len(cid)-4:], "second", "\nHistory:\n", "hedgehug: edit "} {
		if !strings.Contains(out, w) {
			t.Errorf("show lacks %q:\n%s", w, out)
		}
	}
	var sj struct {
		History []logJSON `json:"history"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", g.Short, "--json")), &sj); err != nil {
		t.Fatal(err)
	}
	if len(sj.History) == 0 || !slices.ContainsFunc(sj.History, func(l logJSON) bool { return strings.HasPrefix(l.Subject, "hedgehug: edit ") }) {
		t.Errorf("show --json history = %+v, want the edit commit", sj.History)
	}
	step("comment", "comment", "rm", cid)
	if out := te.ok("log", g.Short); !strings.Contains(out, "hedgehug: edit ") || !strings.Contains(out, "hedgehug: comment ") || len(strings.Split(strings.TrimRight(out, "\n"), "\n")) < 4 {
		t.Errorf("log for the task:\n%s", out)
	}
	step("status", "status", g.Short, "in_progress")
	step("status", "done", g.Short)
	step("mv", "mv", o.Short, "--parent", p.Short, "--before", c.Short)
	step("dep", "dep", "add", c.Short, "--blocked-by", o.Short)
	step("dep", "dep", "rm", c.Short, "--blocked-by", o.Short)
	step("migrate", "schema", "migrate", "status", "done", "closed")
	if out := te.ok("ls", "status:closed", "--ids-only"); !strings.Contains(out, g.Short) {
		t.Errorf("ls status:closed = %q, want %s", out, g.Short)
	}
	step("rm", "rm", p.Short)
	step("restore", "restore", p.Short)
	step("rm", "rm", p.Short)
	step("purge", "purge", p.Short, "--yes")
	if out := te.ok("ls", "is:deleted", "--ids-only"); out != "" {
		t.Errorf("deleted tasks remain after purge: %q", out)
	}
	// The store stays consistent after all of it.
	for _, args := range [][]string{{"schema", "check"}, {"doctor"}} {
		if out := te.ok(args...); !strings.Contains(out, "No problems") {
			t.Errorf("%v = %q, want No problems", args, out)
		}
	}
	// store init, four adds and the fifteen steps.
	if n := len(te.log("me")); n != 20 {
		t.Errorf("commits = %d, want 20", n)
	}
	for _, c := range te.log("me") {
		if c.Author.Email != "alice@example.com" {
			t.Errorf("commit %s by %s, want alice", c.Hash, c.Author.Email)
		}
	}
}
