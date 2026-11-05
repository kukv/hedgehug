package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

func TestSetQuery(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a", "-p", "labels=x")
	b := te.addJSON("b", "-p", "labels=x")
	c := te.addJSON("c")
	before := len(te.log("me"))

	code, _, stderr := te.run("set", "--query", "label:x", "status=done")
	if code != 2 || !strings.Contains(stderr, "--yes") {
		t.Fatalf("without --yes: code %d, stderr %s", code, stderr)
	}

	out := te.ok("set", "--query", "label:x", "status=done", "--dry-run")
	if !strings.Contains(out, a.ID) || !strings.Contains(out, b.ID) || strings.Contains(out, c.ID) {
		t.Fatalf("dry run:\n%s", out)
	}
	if got := len(te.log("me")); got != before {
		t.Fatalf("dry run made %d commits", got-before)
	}

	out = te.ok("set", "--query", "label:x", "status=done", "--yes")
	if !strings.Contains(out, "Updated 2 tasks") {
		t.Fatalf("set:\n%s", out)
	}
	if got := len(te.log("me")); got != before+1 {
		t.Fatalf("%d commits", got-before)
	}
	if got := te.task(a.ULID); got.Status != "done" {
		t.Errorf("status = %s", got.Status)
	}
	if got := te.ok("log"); !strings.Contains(got, "bulk-set 2 tasks status=done") {
		t.Errorf("log:\n%s", got)
	}
}

func TestSetQueryConfirmsOnATerminal(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a", "-p", "labels=x")
	te.terminal = true
	te.stdin = "n\n"
	if out := te.ok("set", "--query", "label:x", "status=done"); !strings.Contains(out, a.ID) {
		t.Fatalf("no list:\n%s", out)
	}
	if got := te.task(a.ULID); got.Status == "done" {
		t.Fatal("no was taken for yes")
	}
	te.stdin = "y\n"
	te.ok("set", "--query", "label:x", "status=done")
	if got := te.task(a.ULID); got.Status != "done" {
		t.Fatal("yes did not write")
	}
}

func TestSetQueryNothingMatches(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	if out := te.ok("set", "--query", "label:none-of-these", "status=done", "--yes"); !strings.Contains(out, "No tasks match") {
		t.Fatalf("out:\n%s", out)
	}
}

func TestSetQueryViolation(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a", "-p", "labels=x")
	code, _, stderr := te.run("set", "--query", "label:x", "status=nope", "--yes")
	if code == 0 || !strings.Contains(stderr, a.ID) {
		t.Fatalf("code %d, stderr %s", code, stderr)
	}
	var r tasks.BulkResult
	if err := json.Unmarshal([]byte(te.ok("set", "--query", "label:x", "status=nope", "--dry-run", "--json")), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Violations) != 1 {
		t.Fatalf("result = %+v", r)
	}
}

func TestAddTemplateCommand(t *testing.T) {
	te := newStoreEnv(t) // not parallel: editWith sets the environment
	te.editWith("---\nkind: task\ntitle: Bug\nchildren:\n  - title: Reproduce\n---\nSteps\n")
	if got := te.ok("template", "edit", "bug"); !strings.Contains(got, "Saved template bug") {
		t.Fatalf("edit:\n%s", got)
	}
	if got := te.ok("template", "ls"); !strings.Contains(got, "bug") || !strings.Contains(got, "1") {
		t.Fatalf("ls:\n%s", got)
	}
	if got := te.ok("template", "show", "bug"); !strings.Contains(got, "Steps") {
		t.Fatalf("show:\n%s", got)
	}
	if got := te.ok("add", "--template", "bug"); !strings.Contains(got, "Bug") || !strings.Contains(got, "with 1 task under it") {
		t.Fatalf("add:\n%s", got)
	}
	te.ok("template", "rm", "bug")
	if code, _, _ := te.run("template", "show", "bug"); code == 0 {
		t.Fatal("the template is still there")
	}
}

func TestTemplateEditLeavesANewNameUnwritten(t *testing.T) {
	te := newStoreEnv(t) // not parallel: editWith sets the environment
	te.editWith("---\nkind: task\n---\n")
	te.ok("template", "edit", "bug")
	if out := te.ok("template", "ls"); strings.Contains(out, "bug") {
		t.Fatalf("ls:\n%s", out)
	}
}

func TestAddNeedsATitleWithoutATemplate(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	if code, _, _ := te.run("add"); code != 2 {
		t.Fatalf("code = %d", code)
	}
}

func TestSchemaScopeTemplates(t *testing.T) {
	te := newStoreEnv(t) // not parallel: editWith sets the environment
	te.editWith("---\nlabels: [a]\n---\n")
	te.ok("template", "edit", "a")
	p := te.addJSON("p", "--kind", "project")

	if code, _, stderr := te.run("schema", "scope", "set", p.ID, "--templates", "nope"); code == 0 || !strings.Contains(stderr, "nope") {
		t.Fatalf("code %d, stderr %s", code, stderr)
	}
	te.ok("schema", "scope", "set", p.ID, "--templates", "a", "--template-default", "task=a")
	if out := te.ok("schema", "scope", "show", p.ID); !strings.Contains(out, "a") {
		t.Fatalf("show:\n%s", out)
	}
	c := te.addJSON("c", "--parent", p.ID)
	if got := te.task(c.ULID); len(got.Labels) != 1 || got.Labels[0] != "a" {
		t.Fatalf("labels = %v", got.Labels)
	}
	te.ok("schema", "scope", "set", p.ID, "--templates", "", "--template-default", "")
	d := te.addJSON("d", "--parent", p.ID)
	if got := te.task(d.ULID); len(got.Labels) != 0 {
		t.Fatalf("labels = %v", got.Labels)
	}
}
