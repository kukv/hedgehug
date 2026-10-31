package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// putSchema writes the schema of the store me after edit changes the
// default one.
func (te *testEnv) putSchema(edit func(*schema.Schema)) {
	te.t.Helper()
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	sc := schema.Default()
	edit(sc)
	author := gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}
	if _, err := s.Update(store.Op{Name: "update", Author: author}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		te.t.Fatal(err)
	}
}

func (te *testEnv) schemaFile() string {
	te.t.Helper()
	b, err := os.ReadFile(filepath.Join(te.storeDir("me"), "schema.yaml"))
	if err != nil {
		te.t.Fatal(err)
	}
	return string(b)
}

type migrateOut struct {
	Changed []taskJSON `json:"changed"`
	Skipped []string   `json:"skipped"`
}

func TestSchemaMigrateStatus(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.ok("rm", b.ID)
	c := te.addJSON("Gamma")
	te.ok("status", c.ID, "done")
	d := te.addJSON("Broken")
	te.commitFile(d.ULID, "not a task\n")
	te.putSchema(func(sc *schema.Schema) {
		// todo may not move to ready, but a migration is not a move.
		sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	})
	before := len(te.log("me"))

	code, out, stderr := te.run("schema", "migrate", "status", "todo", "ready")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(out, "Migrated 2 tasks\n") || !strings.Contains(out, label(a)+"  Alpha\n") || !strings.Contains(out, label(b)+"  Beta\n") || strings.Contains(out, "Gamma") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(stderr, "warning: not migrated, since they do not decode: "+d.ULID+"\n") {
		t.Errorf("stderr = %q, want a warning about %s", stderr, d.ULID)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: migrate") {
		t.Errorf("commits = %d (want %d), message %q", len(log), before+1, log[0].Message)
	}
	if st := te.task(a.ULID).Status; st != "ready" {
		t.Errorf("Alpha status = %q, want ready", st)
	}
	if st := te.task(b.ULID).Status; st != "ready" {
		t.Errorf("deleted Beta status = %q, want ready", st)
	}
	if got := te.raw(d.ULID); got != "not a task\n" {
		t.Errorf("broken task = %q", got)
	}
	sf := te.schemaFile()
	if strings.Contains(sf, "name: todo") || !strings.Contains(sf, "{name: ready, label: 未着手, category: todo, default: true}") || !strings.Contains(sf, "ready: [in_progress]") {
		t.Errorf("schema.yaml:\n%s", sf)
	}
	if out := te.ok("ls", "status:ready"); !strings.Contains(out, "Alpha") {
		t.Errorf("ls status:ready = %q", out)
	}
}

// A file at the path of one task that holds another, as a stale copy, is
// left alone: writing it would overwrite the real file of the other task.
func TestSchemaMigrateSkipsMismatchedULID(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	d := te.addJSON("Delta")
	stale := te.raw(a.ULID)
	te.commitFile(d.ULID, stale)
	te.ok("set", a.ID, "title=Alpha2")

	code, out, stderr := te.run("schema", "migrate", "status", "todo", "ready")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(out, "Migrated 1 task\n") || !strings.Contains(out, label(a)+"  Alpha2\n") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(stderr, "warning: not migrated, since they do not decode: "+d.ULID+"\n") {
		t.Errorf("stderr = %q, want a warning about %s only", stderr, d.ULID)
	}
	if x := te.task(a.ULID); x.Title != "Alpha2" || x.Status != "ready" {
		t.Errorf("Alpha = title %q, status %q; want Alpha2, ready", x.Title, x.Status)
	}
	if got := te.raw(d.ULID); got != stale {
		t.Errorf("the file at Delta's path = %q, want it unchanged", got)
	}
}

func TestSchemaMigrateJSON(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "priority=mid")
	te.addJSON("Beta")
	d := te.addJSON("Broken")
	te.commitFile(d.ULID, "not a task\n")

	var got migrateOut
	out := te.ok("schema", "migrate", "value", "priority", "mid", "medium", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(got.Changed) != 1 || got.Changed[0].ID != a.ID || got.Changed[0].Props["priority"] != "medium" {
		t.Errorf("changed = %+v", got.Changed)
	}
	if len(got.Skipped) != 1 || got.Skipped[0] != d.ULID {
		t.Errorf("skipped = %v, want [%s]", got.Skipped, d.ULID)
	}
	if !strings.Contains(te.schemaFile(), "values: [low, medium, high, urgent]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}
}

func TestSchemaMigrateKindAndProp(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha", "--kind", "bug")
	te.ok("set", a.ID, "start=2026-11-01")
	te.ok("schema", "migrate", "kind", "bug", "task")
	te.ok("schema", "migrate", "prop", "start", "begin")
	x := te.task(a.ULID)
	if x.Kind != "task" || x.Props.Get("begin") == nil || x.Props.Get("start") != nil {
		t.Errorf("task = kind %q, props %v", x.Kind, x.Props)
	}
	if sf := te.schemaFile(); strings.Contains(sf, "name: bug") || !strings.Contains(sf, "begin: {type: date}") {
		t.Errorf("schema.yaml:\n%s", sf)
	}
}

func TestSchemaMigratePropOnBoth(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("set", a.ID, "start=2026-11-01", "due=2026-11-30")
	b := te.addJSON("Beta")
	te.ok("set", b.ID, "start=2026-11-01", "due=2026-11-30")
	before := len(te.log("me"))
	code, _, stderr := te.run("schema", "migrate", "prop", "start", "due")
	if code != 1 || !strings.Contains(stderr, label(a)) || !strings.Contains(stderr, label(b)) || !strings.Contains(stderr, "both start and due") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestSchemaMigrateRejectsNewViolation(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Parent")
	c := te.addJSON("Child", "--parent", p.ID)
	before := len(te.log("me"))
	sf := te.schemaFile()
	// A bug may not sit under a bug.
	code, _, stderr := te.run("schema", "migrate", "kind", "task", "bug")
	if code != 1 || !strings.Contains(stderr, "breaks the schema") || !strings.Contains(stderr, "cannot be placed") || !strings.Contains(stderr, label(c)+" Child") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if te.schemaFile() != sf || te.task(c.ULID).Kind != "task" {
		t.Error("the store changed")
	}
}

// Merging into a status with required fields keeps the tasks that lack
// them: a migration is not a transition, and they did not have them before.
func TestSchemaMigrateMergeIntoRequired(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("status", a.ID, "backlog")
	te.putSchema(func(sc *schema.Schema) {
		sc.Workflow.Required = schema.Lists{{Key: "todo", Values: []string{"points"}}}
	})
	te.ok("schema", "migrate", "status", "backlog", "todo")
	if st := te.task(a.ULID).Status; st != "todo" {
		t.Errorf("status = %q, want todo", st)
	}
}

// A rename keeps a task that already lacked a required field of the status.
func TestSchemaMigrateKeepsMissingRequired(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("status", a.ID, "in_progress")
	te.putSchema(func(sc *schema.Schema) {
		sc.Workflow.Required = schema.Lists{{Key: "in_progress", Values: []string{"assignees"}}}
	})
	code, out, stderr := te.run("schema", "migrate", "status", "in_progress", "doing")
	if code != 0 || !strings.HasPrefix(out, "Migrated 1 task\n") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	if st := te.task(a.ULID).Status; st != "doing" {
		t.Errorf("status = %q, want doing", st)
	}
	if sf := te.schemaFile(); !strings.Contains(sf, "doing: [assignees]") {
		t.Errorf("schema.yaml:\n%s", sf)
	}
}

func TestSchemaMigrateRejectsBadSchema(t *testing.T) {
	te := newStoreEnv(t)
	te.addJSON("Alpha")
	before := len(te.log("me"))
	code, _, stderr := te.run("schema", "migrate", "status", "todo", "in_progress")
	if code != 1 || !strings.Contains(stderr, "needs exactly one default status") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestSchemaMigrateDryRun(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	before := len(te.log("me"))
	sf := te.schemaFile()
	out := te.ok("schema", "migrate", "status", "todo", "ready", "--dry-run")
	if !strings.HasPrefix(out, "Would migrate 1 task\n") || !strings.Contains(out, label(a)+"  Alpha\n") {
		t.Errorf("stdout = %q", out)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if te.schemaFile() != sf || te.task(a.ULID).Status != "todo" {
		t.Error("the store changed")
	}

	var got migrateOut
	out = te.ok("schema", "migrate", "status", "todo", "ready", "--dry-run", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(got.Changed) != 1 || got.Changed[0].ID != a.ID || got.Changed[0].Status != "ready" {
		t.Errorf("changed = %+v, want Alpha as migrated", got.Changed)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestSchemaMigrateArgs(t *testing.T) {
	te := newStoreEnv(t)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"schema"}, 2},
		{[]string{"schema", "migrate"}, 2},
		{[]string{"schema", "migrate", "nope", "a", "b"}, 2},
		{[]string{"schema", "migrate", "status", "todo"}, 2},
		{[]string{"schema", "migrate", "status", "todo", "a", "b"}, 2},
		{[]string{"schema", "migrate", "value", "priority", "low"}, 2},
		{[]string{"schema", "migrate", "status", "nope", "x"}, 1},
		{[]string{"schema", "migrate", "status", "todo", "todo"}, 1},
		{[]string{"schema", "migrate", "status", "-h"}, 0},
	} {
		if code, _, stderr := te.run(tc.args...); code != tc.code {
			t.Errorf("%v: exit %d, want %d (stderr %q)", tc.args, code, tc.code, stderr)
		}
	}
}
