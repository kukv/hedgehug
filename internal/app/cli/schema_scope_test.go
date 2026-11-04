package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

func TestSchemaScopeGolden(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Web", "--kind", "project")
	te.ok("add", "Login")

	te.goldenRun("show-none", 0, "schema", "scope", "show", "ME-1")
	te.goldenRun("set-all", 0, "schema", "scope", "set", "ME-1",
		"--statuses", "todo,in_progress,done,canceled",
		"--transition", "todo=in_progress,done", "--transition", "in_progress=done",
		"--required", "done=points")
	te.goldenRun("show", 0, "schema", "scope", "show", "ME-1")
	te.goldenRun("show-json", 0, "schema", "scope", "show", "ME-1", "--json")
	// only the part that is given is replaced
	te.goldenRun("set-required", 0, "schema", "scope", "set", "ME-1", "--required", "in_progress=points,due")
	te.goldenRun("set-same", 0, "schema", "scope", "set", "ME-1", "--required", "in_progress=points,due")
	te.goldenRun("clear-statuses", 0, "schema", "scope", "set", "ME-1", "--statuses", "")
	te.goldenRun("clear-rest-json", 0, "schema", "scope", "set", "ME-1", "--transition", "", "--required", "", "--json")
	te.goldenRun("show-gone", 0, "schema", "scope", "show", "ME-1")

	te.ok("schema", "scope", "set", "ME-1", "--statuses", "todo,in_progress,done,canceled")
	te.goldenRun("rm", 0, "schema", "scope", "rm", "ME-1")
	te.goldenRun("rm-none", 0, "schema", "scope", "rm", "ME-1")
	te.goldenRun("rm-json", 0, "schema", "scope", "rm", "ME-1", "--json")

	te.goldenRun("no-flags", 2, "schema", "scope", "set", "ME-1")
	te.goldenRun("bad-form", 2, "schema", "scope", "set", "ME-1", "--transition", "todo")
	te.goldenRun("twice", 2, "schema", "scope", "set", "ME-1", "--required", "done=points", "--required", "done=due")
	te.goldenRun("hides-a-default", 1, "schema", "scope", "set", "ME-1", "--statuses", "todo,in_progress,canceled")
	te.goldenRun("unknown-status", 1, "schema", "scope", "set", "ME-1", "--transition", "todo=nope")
	te.goldenRun("not-a-container", 1, "schema", "scope", "set", "ME-2", "--statuses", "")
	te.goldenRun("unknown-project", 1, "schema", "scope", "set", "NOPE-9", "--statuses", "")
}

func TestSchemaScopeIsOneCommit(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("Web", "--kind", "project")
	before := len(te.log("me"))

	te.ok("schema", "scope", "set", p.ID, "--statuses", "todo,in_progress,done,canceled", "--required", "done=points", "--transition", "todo=done")
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: schema") || !strings.Contains(log[0].Message, "Hedgehug-Task: "+p.ULID) {
		t.Fatalf("log = %d commits (was %d), newest %q", len(log), before, log[0].Message)
	}

	var v schemas.ScopedView
	if err := json.Unmarshal([]byte(te.ok("schema", "scope", "show", p.ID, "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if v.Project.ULID != p.ULID || v.Commit != nil || v.Scoped == nil || len(v.Scoped.Statuses) != 4 || v.Scoped.Workflow.Required["done"][0] != "points" {
		t.Errorf("show = %+v", v)
	}

	// the override applies to the descendants: todo may only move to done
	child := te.addJSON("Child", "--parent", p.ID)
	if code, _, _ := te.run("status", child.ID, "canceled"); code != 1 {
		t.Errorf("todo -> canceled: exit %d, want 1", code)
	}
	if code, _, _ := te.run("status", child.ID, "done"); code != 1 {
		t.Errorf("todo -> done without points: exit %d, want 1", code)
	}
	te.ok("set", child.ID, "points=3")
	if code, _, stderr := te.run("status", child.ID, "done"); code != 0 {
		t.Errorf("todo -> done: exit %d, %s", code, stderr)
	}
}
