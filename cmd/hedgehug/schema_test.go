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

// writeSchemaFile replaces schema.yaml of the store me with the default
// schema after edit changed it, and commits it past the checks of the store.
func (te *testEnv) writeSchemaFile(edit func(*schema.Schema)) {
	te.t.Helper()
	sc := schema.Default()
	edit(sc)
	b, err := sc.Encode()
	if err != nil {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.storeDir("me"), "schema.yaml"), b, 0o600); err != nil {
		te.t.Fatal(err)
	}
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	author := gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{"schema.yaml"}, Subject: "raw", Author: author, When: te.now}); err != nil {
		te.t.Fatal(err)
	}
}

func TestSchemaCheckNoProblems(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	code, out, stderr := te.run("schema", "check")
	if code != 0 || out != "No problems\n" || stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
	code, out, _ = te.run("schema", "check", "--json")
	if code != 0 || strings.TrimSpace(out) != "[]" {
		t.Errorf("--json: exit %d, stdout %q", code, out)
	}
}

func TestSchemaCheckProblems(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.writeSchemaFile(func(sc *schema.Schema) {
		sc.Kinds[0].Template = "nope"
		sc.Statuses[1].Name = "later" // the status of Alpha is gone
	})
	code, out, stderr := te.run("schema", "check")
	if code != 1 || stderr != "" {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, out)
	}
	want := []string{
		`schema.yaml: kinds[0].template: template "nope" does not exist`,
		"  hint: add templates/nope.md or change the name",
		label(a) + " Alpha: status: unknown status \"todo\"",
		"2 problems",
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < len(want) {
		t.Fatalf("output:\n%s", out)
	}
	if lines[0] != want[0] || lines[1] != want[1] || lines[len(lines)-1] != "2 problems" || !strings.Contains(out, want[2]) {
		t.Errorf("output:\n%s\nwant lines %q", out, want)
	}

	code, out, stderr = te.run("schema", "check", "--json")
	if code != 1 || stderr != "" {
		t.Fatalf("--json: exit %d, stderr %q", code, stderr)
	}
	var fs []map[string]string
	if err := json.Unmarshal([]byte(out), &fs); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(fs) != 2 || fs[0]["check"] != "template" || fs[0]["hint"] == "" || fs[1]["check"] != "task" {
		t.Errorf("findings = %v", fs)
	}
	if fs[0]["severity"] != "error" || fs[1]["severity"] != "error" {
		t.Errorf("severity = %q, %q; want error for every finding of schema check", fs[0]["severity"], fs[1]["severity"])
	}
	if _, ok := fs[1]["hint"]; ok {
		t.Errorf("empty hint is not omitted: %v", fs[1])
	}
}

func TestSchemaCheckBrokenSchemaFile(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("add", "Alpha")
	if err := os.WriteFile(filepath.Join(te.storeDir("me"), "schema.yaml"), []byte("statuses: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := te.run("schema", "check")
	if code != 1 || stderr != "" {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "schema.yaml: ") ||
		lines[1] != "  hint: restore schema.yaml with `git restore schema.yaml`, or fix it by hand and commit" || lines[2] != "1 problems" {
		t.Errorf("output:\n%s", out)
	}
}

func TestSchemaCheckBrokenStoreIsNotAFinding(t *testing.T) {
	te := newStoreEnv(t)
	if err := os.Remove(filepath.Join(te.storeDir("me"), ".hedgehug", "store.yaml")); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := te.run("schema", "check")
	if code != 1 || !strings.HasPrefix(stderr, "hedgehug: ") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestSchemaCheckTakesNoArguments(t *testing.T) {
	te := newStoreEnv(t)
	if code, _, _ := te.run("schema", "check", "x"); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}
