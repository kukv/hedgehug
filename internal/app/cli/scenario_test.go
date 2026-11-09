package cli

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

// TestScenario runs the phase 1 done criterion for the CLI: in one store,
// every CRUD operation and the tree work, and each is one commit.
//
// 直列: editWith が t.Setenv（EDITOR と HH_TEST_EDITOR_SCRIPT）を使う
func TestScenario(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.writeAccounts("github.com", "alice")
	te.labels("ui")
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
	q := te.addJSON("Elsewhere", "--kind", "project")
	o := te.addJSON("Other", "--parent", q.ID)

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
	step("comment", "comment", "add", g.ID, "first")
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
		History []usecase.LogView `json:"history"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", g.ID, "--json")), &sj); err != nil {
		t.Fatal(err)
	}
	if len(sj.History) == 0 || !slices.ContainsFunc(sj.History, func(l usecase.LogView) bool { return strings.HasPrefix(l.Subject, "hedgehug: edit ") }) {
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
	step("purge", "purge", "--older-than", "0d", "--yes")
	if out := te.ok("ls", "is:deleted", "--ulids-only"); out != "" {
		t.Errorf("deleted tasks remain after purge: %q", out)
	}
	// The store stays consistent after all of it.
	for _, args := range [][]string{{"schema", "check"}, {"doctor"}} {
		if out := te.ok(args...); !strings.Contains(out, "No problems") {
			t.Errorf("%v = %q, want No problems", args, out)
		}
	}
	// store init, the labels, five adds and the fifteen steps.
	if n := len(te.log("me")); n != 22 {
		t.Errorf("commits = %d, want 22", n)
	}
	// Every commit is the account's.
	for _, c := range te.log("me") {
		if c.Author.Email != "alice@users.noreply.github.com" {
			t.Errorf("commit %s by %s, want alice@users.noreply.github.com", c.Hash, c.Author.Email)
		}
	}
}
