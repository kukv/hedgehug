package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/ops"
)

// TestScenario runs the phase 1 done criterion for the CLI: in one store,
// every CRUD operation and the tree work, and each is one commit.
func TestScenario(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.writeAccounts("github.com", "alice")
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
	c := te.addJSON("Login page", "--parent", p.ID)
	g := te.addJSON("Form", "--parent", c.ID)
	o := te.addJSON("Other")

	tree := te.ok("ls", "--tree", "--no-color")
	for _, s := range []string{"Website", "└─ ", "Form", "Other"} {
		if !strings.Contains(tree, s) {
			t.Fatalf("tree lacks %q:\n%s", s, tree)
		}
	}
	step("set", "set", g.ID, "labels+=ui", "due=2026-11-01")
	// The editor writes its contents in turn: the task for edit, then the
	// body for comment edit.
	te.editWith(setLine(te.raw(g.ULID), "title", "Form fields"), "second\n")
	step("edit", "edit", g.ID)
	step("comment", "comment", g.ID, "first")
	cm := te.comments(g.ULID)
	if len(cm) != 1 {
		t.Fatalf("comments = %d, want 1", len(cm))
	}
	step("comment", "comment", "edit", cm[0].ULID.String())
	if cs := te.comments(g.ULID); len(cs) != 1 || cs[0].Body != "second\n" {
		t.Fatalf("comments after edit = %+v", cs)
	}
	out := te.ok("show", g.ID)
	for _, w := range []string{"Form fields", cm[0].ULID.String(), "second", "\nHistory:\n", "hedgehug: edit "} {
		if !strings.Contains(out, w) {
			t.Errorf("show lacks %q:\n%s", w, out)
		}
	}
	var sj struct {
		History []ops.LogView `json:"history"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", g.ID, "--json")), &sj); err != nil {
		t.Fatal(err)
	}
	if len(sj.History) == 0 || !slices.ContainsFunc(sj.History, func(l ops.LogView) bool { return strings.HasPrefix(l.Subject, "hedgehug: edit ") }) {
		t.Errorf("show --json history = %+v, want the edit commit", sj.History)
	}
	step("comment", "comment", "rm", cm[0].ULID.String())
	if out := te.ok("log", g.ID); !strings.Contains(out, "hedgehug: edit ") || !strings.Contains(out, "hedgehug: comment ") || len(strings.Split(strings.TrimRight(out, "\n"), "\n")) < 4 {
		t.Errorf("log for the task:\n%s", out)
	}
	step("status", "status", g.ID, "in_progress")
	step("status", "done", g.ID)
	step("mv", "mv", o.ID, "--parent", p.ID, "--before", c.ID)
	step("dep", "dep", "add", c.ID, "--blocked-by", o.ID)
	step("dep", "dep", "rm", c.ID, "--blocked-by", o.ID)
	step("migrate", "schema", "migrate", "status", "done", "closed")
	if out := te.ok("ls", "status:closed", "--ulids-only"); !strings.Contains(out, g.ULID) {
		t.Errorf("ls status:closed = %q, want %s", out, g.ULID)
	}
	step("rm", "rm", p.ID)
	step("restore", "restore", p.ID)
	step("rm", "rm", p.ID)
	step("purge", "purge", p.ID, "--yes")
	if out := te.ok("ls", "is:deleted", "--ulids-only"); out != "" {
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
	// store init was made before the account was set up; every commit after
	// it is the account's.
	log := te.log("me")
	for i, c := range log {
		want := "alice@users.noreply.github.com"
		if i == len(log)-1 {
			want = "alice@example.com"
		}
		if c.Author.Email != want {
			t.Errorf("commit %s by %s, want %s", c.Hash, c.Author.Email, want)
		}
	}
}
