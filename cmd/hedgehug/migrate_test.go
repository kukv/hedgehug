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
	te.ok("rm", b.Short)
	c := te.addJSON("Gamma")
	te.ok("status", c.Short, "done")
	d := te.addJSON("Broken")
	te.commitFile(d.ID, "not a task\n")
	te.putSchema(func(sc *schema.Schema) {
		// todo may not move to ready, but a migration is not a move.
		sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	})
	before := len(te.log("me"))

	code, out, stderr := te.run("schema", "migrate", "status", "todo", "ready")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.HasPrefix(out, "Migrated 2 tasks\n") || !strings.Contains(out, a.Short+"  Alpha\n") || !strings.Contains(out, b.Short+"  Beta\n") || strings.Contains(out, "Gamma") {
		t.Errorf("stdout = %q", out)
	}
	if !strings.Contains(stderr, "warning: not migrated, since they do not decode: "+d.Short+"\n") {
		t.Errorf("stderr = %q, want a warning about %s", stderr, d.Short)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: migrate") {
		t.Errorf("commits = %d (want %d), message %q", len(log), before+1, log[0].Message)
	}
	if st := te.task(a.ID).Status; st != "ready" {
		t.Errorf("Alpha status = %q, want ready", st)
	}
	if st := te.task(b.ID).Status; st != "ready" {
		t.Errorf("deleted Beta status = %q, want ready", st)
	}
	if got := te.raw(d.ID); got != "not a task\n" {
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

func TestSchemaMigrateJSON(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("set", a.Short, "priority=mid")
	te.addJSON("Beta")
	d := te.addJSON("Broken")
	te.commitFile(d.ID, "not a task\n")

	var got migrateOut
	out := te.ok("schema", "migrate", "value", "priority", "mid", "medium", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(got.Changed) != 1 || got.Changed[0].Short != a.Short || got.Changed[0].Props["priority"] != "medium" {
		t.Errorf("changed = %+v", got.Changed)
	}
	if len(got.Skipped) != 1 || got.Skipped[0] != d.Short {
		t.Errorf("skipped = %v, want [%s]", got.Skipped, d.Short)
	}
	if !strings.Contains(te.schemaFile(), "values: [low, medium, high, urgent]") {
		t.Errorf("schema.yaml:\n%s", te.schemaFile())
	}
}

func TestSchemaMigrateKindAndProp(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha", "--kind", "bug")
	te.ok("set", a.Short, "start=2026-11-01")
	te.ok("schema", "migrate", "kind", "bug", "task")
	te.ok("schema", "migrate", "prop", "start", "begin")
	x := te.task(a.ID)
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
	te.ok("set", a.Short, "start=2026-11-01", "due=2026-11-30")
	b := te.addJSON("Beta")
	te.ok("set", b.Short, "start=2026-11-01", "due=2026-11-30")
	before := len(te.log("me"))
	code, _, stderr := te.run("schema", "migrate", "prop", "start", "due")
	if code != 1 || !strings.Contains(stderr, a.Short) || !strings.Contains(stderr, b.Short) || !strings.Contains(stderr, "both start and due") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestSchemaMigrateRejectsNewViolation(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.ok("status", a.Short, "backlog")
	te.putSchema(func(sc *schema.Schema) {
		sc.Workflow.Required = schema.Lists{{Key: "todo", Values: []string{"points"}}}
	})
	before := len(te.log("me"))
	sf := te.schemaFile()
	code, _, stderr := te.run("schema", "migrate", "status", "backlog", "todo")
	if code != 1 || !strings.Contains(stderr, "breaks the schema") || !strings.Contains(stderr, "points") || !strings.Contains(stderr, a.Short+" Alpha") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if te.schemaFile() != sf || te.task(a.ID).Status != "backlog" {
		t.Error("the store changed")
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
	if !strings.HasPrefix(out, "Would migrate 1 task\n") || !strings.Contains(out, a.Short+"  Alpha\n") {
		t.Errorf("stdout = %q", out)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if te.schemaFile() != sf || te.task(a.ID).Status != "todo" {
		t.Error("the store changed")
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
