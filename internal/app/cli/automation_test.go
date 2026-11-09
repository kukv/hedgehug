package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/automations"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestAutomationGolden(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Web", "--kind", "project")
	te.ok("add", "Login", "--parent", "ME-1")

	te.goldenRun("list-none", 0, "automation", "list")
	te.goldenRun("add", 0, "automation", "add", "--id", "opened", "--on", "pr.opened", "--status", "in_progress")
	te.goldenRun("add-category", 0, "automation", "add", "--id", "merged", "--on", "pr.merged", "--when", "all_linked_prs_merged", "--category", "done")
	te.goldenRun("add-project", 0, "automation", "add", "--id", "web-review", "--on", "pr.review_requested", "--status", "in_progress", "--project", "ME-1")
	te.goldenRun("list", 0, "automation", "list")
	te.goldenRun("disable", 0, "automation", "disable", "merged")
	te.goldenRun("disable-in-project", 0, "automation", "disable", "opened", "--project", "ME-1")
	te.goldenRun("list-after", 0, "automation", "list")
	te.goldenRun("list-project", 0, "automation", "list", "--project", "ME-1")
	te.goldenRun("list-json", 0, "automation", "list", "--json")
	te.goldenRun("enable-in-project", 0, "automation", "enable", "opened", "--project", "ME-1")
	te.goldenRun("enable", 0, "automation", "enable", "merged")
	te.goldenRun("rm", 0, "automation", "rm", "web-review")

	te.goldenRun("fields", 0, "automation", "fields")
	te.goldenRun("fields-set", 0, "automation", "fields", "--default-sync", "mirror", "--push", "title,status", "--prop", "milestone=target")
	te.goldenRun("fields-json", 0, "automation", "fields", "--json")
	te.goldenRun("fields-unset", 0, "automation", "fields", "--mirror", "", "--prop", "milestone=")

	te.goldenRun("no-subcommand", 2, "automation")
	te.goldenRun("add-no-set", 2, "automation", "add", "--id", "x", "--on", "pr.opened")
	te.goldenRun("add-both", 2, "automation", "add", "--id", "x", "--on", "pr.opened", "--status", "done", "--category", "done")
	te.goldenRun("add-no-id", 2, "automation", "add", "--on", "pr.opened", "--status", "done")
	te.goldenRun("bad-prop", 2, "automation", "fields", "--prop", "milestone")
	te.goldenRun("unknown-status", 1, "automation", "add", "--id", "x", "--on", "pr.opened", "--status", "nope")
	te.goldenRun("unknown-event", 1, "automation", "add", "--id", "x", "--on", "pr.nope", "--status", "done")
	te.goldenRun("duplicate", 1, "automation", "add", "--id", "opened", "--on", "pr.opened", "--status", "done")
	te.goldenRun("rm-unknown", 1, "automation", "rm", "nope")
	te.goldenRun("disable-unknown", 1, "automation", "disable", "nope")
	te.goldenRun("not-a-project", 1, "automation", "add", "--id", "x", "--on", "pr.opened", "--status", "done", "--project", "ME-2")
	te.goldenRun("bad-sync", 1, "automation", "fields", "--default-sync", "nope")
}

func TestAutomationIsOneCommitAndUndoTakesItBack(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	path := filepath.Join(te.storeDir("me"), "automations.yaml")
	before := len(te.log("me"))

	te.ok("automation", "add", "--id", "opened", "--on", "pr.opened", "--status", "in_progress")
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: automation") {
		t.Fatalf("log = %d commits (was %d), newest %q", len(log), before, log[0].Message)
	}
	if b, err := os.ReadFile(path); err != nil || !strings.Contains(string(b), "id: opened") { //nolint:gosec // a file of the test store
		t.Fatalf("automations.yaml = %q, %v", b, err)
	}

	te.ok("undo")
	if b, err := os.ReadFile(path); err == nil && strings.Contains(string(b), "opened") { //nolint:gosec // a file of the test store
		t.Errorf("undo left %q", b)
	}
	if out := te.ok("automation", "list"); !strings.Contains(out, "No rule") {
		t.Errorf("list after undo = %q", out)
	}
}

func TestAutomationListJSON(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("add", "Web", "--kind", "project")
	te.ok("automation", "add", "--id", "a", "--on", "pr.opened", "--status", "in_progress", "--project", "ME-1")
	te.ok("automation", "disable", "a", "--project", "ME-1")
	var rs []automations.RuleView
	strictly(t, te.ok("automation", "list", "--json"), &rs)
	if len(rs) != 1 || rs[0].Project == nil || rs[0].Project.ID == "" || len(rs[0].DisabledIn) != 1 || !rs[0].Enabled {
		t.Errorf("rules = %+v", rs)
	}
	var fv automations.FieldsView
	strictly(t, te.ok("automation", "fields", "--json"), &fv)
	if fv.DefaultSync != "link" {
		t.Errorf("fields = %+v", fv)
	}
	var again []automations.RuleView
	if err := json.Unmarshal([]byte(te.ok("automation", "rm", "a", "--json")), &again); err != nil || len(again) != 0 {
		t.Errorf("rm --json = %v, %v", again, err)
	}
}

func TestAutomationRefusesAReadOnlyStore(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("automation", "add", "--id", "a", "--on", "pr.opened", "--status", "in_progress")
	te.uc().WriteAccess("me", usecasetest.Access{ReadOnly: true, Reason: usecasetest.ReasonGitHub, Checked: te.now})
	for _, args := range [][]string{
		{"automation", "add", "--id", "b", "--on", "pr.opened", "--status", "done"},
		{"automation", "rm", "a"},
		{"automation", "disable", "a"},
		{"automation", "fields", "--default-sync", "push"},
	} {
		if code, _, errs := te.run(args...); code == 0 || !strings.Contains(errs, "read-only") {
			t.Errorf("%v: exit %d, stderr %q", args, code, errs)
		}
	}
	te.ok("automation", "list")
}
