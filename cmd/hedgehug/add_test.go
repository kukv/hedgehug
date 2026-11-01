package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// newStoreEnv returns a test env with one store, named me.
func newStoreEnv(t *testing.T) *testEnv {
	t.Helper()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	return te
}

// addJSON runs add with --json and returns the task it printed.
func (te *testEnv) addJSON(args ...string) ops.TaskView {
	te.t.Helper()
	var got ops.TaskView
	out := te.ok(append(append([]string{"add"}, args...), "--json")...)
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		te.t.Fatalf("%v: %s", err, out)
	}
	return got
}

// label is how the CLI names task j: its id and full ULID.
func label(j ops.TaskView) string { return j.ID + " (" + j.ULID + ")" }

// task reads the task file of ulid from the store.
func (te *testEnv) task(ulid string) *model.Task {
	te.t.Helper()
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	taskULID, err := model.ParseULID(ulid)
	if err != nil {
		te.t.Fatal(err)
	}
	t, err := s.Task(taskULID)
	if err != nil {
		te.t.Fatal(err)
	}
	return t
}

func TestAdd(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	out := te.ok("add", "Write", "the", "spec")
	if !regexp.MustCompile(`^Added ME-1 \([0-9A-Z]{26}\) Write the spec\n→ added to me / me\n$`).MatchString(out) {
		t.Errorf("stdout = %q", out)
	}
	log := te.log("me")
	if len(log) != 2 {
		t.Fatalf("commits = %d, want 2", len(log))
	}
	msg := log[0].Message
	if !strings.Contains(msg, "Hedgehug-Op: add") || !strings.Contains(msg, "Hedgehug-Task: ") {
		t.Errorf("commit message = %q", msg)
	}

	got := te.addJSON("Second")
	if got.Kind != "task" || got.Status != "todo" || got.Parent != nil || got.ID != "ME-2" ||
		got.Workspace == nil || got.Workspace.Name != "me" || got.Workspace.IDPrefix != "ME" || len(got.Workspace.ULID) != 26 {
		t.Errorf("task = %+v", got)
	}
}

func TestAddRanksAtTheEnd(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("Parent", "--kind", "project")
	a := te.addJSON("A", "--parent", p.ID)
	b := te.addJSON("B", "--parent", p.ID)
	if a.Parent == nil || *a.Parent != p.ULID {
		t.Errorf("a.parent = %v, want %s", a.Parent, p.ULID)
	}
	if a.Rank >= b.Rank {
		t.Errorf("ranks: a %q, b %q; want a before b", a.Rank, b.Rank)
	}
	r1 := te.addJSON("R1")
	if p.Rank >= r1.Rank {
		t.Errorf("ranks at the root: %q then %q", p.Rank, r1.Rank)
	}
}

func TestAddProps(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	got := te.addJSON("Bug", "--kind", "bug", "--status", "backlog", "-p", "due=2026-11-01", "-p", "labels+=ui")
	if got.Kind != "bug" || got.Status != "backlog" || got.Props["due"] != "2026-11-01" || len(got.Labels) != 1 || got.Labels[0] != "ui" {
		t.Errorf("task = %+v", got)
	}
	if tk := te.task(got.ULID); tk.Title != "Bug" {
		t.Errorf("file title = %q", tk.Title)
	}
}

func TestAddErrors(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		want     string
	}{
		{"no title", []string{"add"}, 2, "requires at least 1 arg"},
		{"unknown parent", []string{"add", "x", "--parent", "zzzz"}, 1, "not found"},
		{"a kind that cannot be at the root", []string{"add", "x", "--kind", "nope"}, 1, "kind"},
		{"a bad -p", []string{"add", "x", "-p", "due"}, 2, "key=value"},
		{"a bad prop value", []string{"add", "x", "-p", "due=soon"}, 1, "date"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := te.run(tc.args...)
			if code != tc.wantCode || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d (want %d), stderr %q (want %q)", code, tc.wantCode, stderr, tc.want)
			}
		})
	}
	if n := len(te.log("me")); n != 1 {
		t.Errorf("commits = %d, want 1: failed adds must not commit", n)
	}
}

func TestAddRefusesADeletedParent(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	te.ok("rm", p.ID)
	code, _, stderr := te.run("add", "E", "--parent", p.ID)
	if code != 1 || !strings.Contains(stderr, "is deleted; restore "+p.ID+" first") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != 3 {
		t.Errorf("commits = %d, want 3 (init, add, rm)", n)
	}
}

func TestAddWithAccount(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.writeAccounts("github.com", "alice")
	x := te.addJSON("x")
	if got := te.task(x.ULID).CreatedBy; got != "github.com/alice" {
		t.Errorf("created_by = %q", got)
	}
	c := te.headCommit("me")
	if trailer := "Hedgehug-Author: " + te.task(x.ULID).CreatedBy; !strings.Contains(c.Message, trailer) {
		t.Errorf("created_by differs from the trailer: %q", c.Message)
	}
	if c.Author.Name != "alice" || c.Author.Email != "alice@users.noreply.github.com" || !strings.Contains(c.Message, "Hedgehug-Author: github.com/alice") {
		t.Errorf("commit = %+v", c)
	}
	te.ok("comment", x.ID, "hi")
	cs := te.comments(x.ULID)
	if len(cs) != 1 || cs[0].Author != "github.com/alice" {
		t.Errorf("comments = %+v", cs)
	}
}

func TestAddWithoutAccountUsesGitConfig(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("x")
	if c := te.headCommit("me"); c.Author.Email != "alice@example.com" || strings.Contains(c.Message, "Hedgehug-Author") {
		t.Errorf("commit = %+v", c)
	}
	if got := te.task(x.ULID).CreatedBy; got != "" {
		t.Errorf("created_by = %q", got)
	}
}

func TestStoreInitWithAccount(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	log := te.log("me")
	c := log[len(log)-1]
	if c.Author.Name != "alice" || c.Author.Email != "alice@users.noreply.github.com" || !strings.Contains(c.Message, "Hedgehug-Author: github.com/alice") {
		t.Errorf("first commit = %+v", c)
	}
}
