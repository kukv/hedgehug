package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
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
func (te *testEnv) addJSON(args ...string) taskJSON {
	te.t.Helper()
	var got taskJSON
	out := te.ok(append(append([]string{"add"}, args...), "--json")...)
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		te.t.Fatalf("%v: %s", err, out)
	}
	return got
}

// label is how the CLI names task j: its key and full ULID.
func label(j taskJSON) string { return j.Key + " (" + j.ID + ")" }

// task reads the task file of id from the store.
func (te *testEnv) task(id string) *model.Task {
	te.t.Helper()
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	mid, err := model.ParseID(id)
	if err != nil {
		te.t.Fatal(err)
	}
	t, err := s.Task(mid)
	if err != nil {
		te.t.Fatal(err)
	}
	return t
}

func TestAdd(t *testing.T) {
	te := newStoreEnv(t)
	out := te.ok("add", "Write", "the", "spec")
	if !regexp.MustCompile(`^Added ME-1 \([0-9A-Z]{26}\) Write the spec\n$`).MatchString(out) {
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
	if got.Kind != "task" || got.Status != "todo" || got.Parent != nil || got.Key != "ME-2" ||
		got.Workspace == nil || got.Workspace.Name != "me" || got.Workspace.KeyPrefix != "ME" || len(got.Workspace.ID) != 26 {
		t.Errorf("task = %+v", got)
	}
}

func TestAddRanksAtTheEnd(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Parent", "--kind", "project")
	a := te.addJSON("A", "--parent", p.Key)
	b := te.addJSON("B", "--parent", p.Key)
	if a.Parent == nil || *a.Parent != p.ID {
		t.Errorf("a.parent = %v, want %s", a.Parent, p.ID)
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
	te := newStoreEnv(t)
	got := te.addJSON("Bug", "--kind", "bug", "--status", "backlog", "-p", "due=2026-11-01", "-p", "labels+=ui")
	if got.Kind != "bug" || got.Status != "backlog" || got.Props["due"] != "2026-11-01" || len(got.Labels) != 1 || got.Labels[0] != "ui" {
		t.Errorf("task = %+v", got)
	}
	if tk := te.task(got.ID); tk.Title != "Bug" {
		t.Errorf("file title = %q", tk.Title)
	}
}

func TestAddErrors(t *testing.T) {
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
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	te.ok("rm", p.Key)
	code, _, stderr := te.run("add", "E", "--parent", p.Key)
	if code != 1 || !strings.Contains(stderr, "is deleted; restore "+label(p)+" first") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != 3 {
		t.Errorf("commits = %d, want 3 (init, add, rm)", n)
	}
}
