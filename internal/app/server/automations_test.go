package server

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/automations"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestAutomationRules(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	project := ts.addProject("p")
	var rules []automations.RuleView
	ts.call("GET", "/api/automations", nil, 200, &rules)
	if len(rules) != 0 {
		t.Fatalf("empty = %+v", rules)
	}
	add := map[string]any{"id": "merged", "on": "pr.merged", "category": "done", "when": "all_linked_prs_merged"}
	ts.call("POST", "/api/automations", add, 201, &rules)
	if len(rules) != 1 || rules[0].ID != "merged" || rules[0].Category != "done" || !rules[0].Enabled {
		t.Fatalf("add = %+v", rules)
	}
	if p := ts.problem("POST", "/api/automations", add, 409); p.Type != "automation.exists" {
		t.Errorf("twice = %+v", p)
	}
	if p := ts.problem("POST", "/api/automations", map[string]any{"id": "x", "on": "pr.merged"}, 400); p.Type == "" {
		t.Errorf("no set = %+v", p)
	}
	if p := ts.problem("POST", "/api/automations", map[string]any{"id": "x", "on": "nope", "category": "done"}, 422); p.Type != "automation.invalid" {
		t.Errorf("bad event = %+v", p)
	}

	ts.call("POST", "/api/automations", map[string]any{"id": "own", "on": "pr.opened", "status": "in_progress", "project": project.ID}, 201, &rules)
	if len(rules) != 2 || rules[1].Project == nil || rules[1].Project.ID != project.ID {
		t.Fatalf("project rule = %+v", rules)
	}

	ts.call("PUT", "/api/automations/merged/enabled", map[string]any{"enabled": false}, 200, &rules)
	if rules[0].Enabled {
		t.Errorf("disable = %+v", rules[0])
	}
	ts.call("PUT", "/api/automations/merged/enabled", map[string]any{"enabled": true}, 200, &rules)
	ts.call("PUT", "/api/automations/merged/enabled", map[string]any{"enabled": false, "project": project.ID}, 200, &rules)
	if rules[0].Enabled {
		t.Errorf("disable in the project = %+v", rules[0])
	}
	ts.call("GET", "/api/automations", nil, 200, &rules)
	if !rules[0].Enabled || len(rules[0].DisabledIn) != 1 {
		t.Errorf("the store's = %+v", rules[0])
	}
	ts.call("GET", "/api/automations?project="+project.ID, nil, 200, &rules)
	if len(rules) != 2 || rules[0].Enabled {
		t.Errorf("the project's = %+v", rules)
	}
	if p := ts.problem("GET", "/api/automations?project=NOPE-1", nil, 404); p.Type != "task.not_found" {
		t.Errorf("no project = %+v", p)
	}
	if p := ts.problem("PUT", "/api/automations/nope/enabled", map[string]any{"enabled": true}, 404); p.Type != "automation.not_found" {
		t.Errorf("no rule = %+v", p)
	}

	ts.call("DELETE", "/api/automations/merged", nil, 200, &rules)
	if len(rules) != 1 || rules[0].ID != "own" {
		t.Errorf("delete = %+v", rules)
	}
	if p := ts.problem("DELETE", "/api/automations/merged", nil, 404); p.Type != "automation.not_found" {
		t.Errorf("delete twice = %+v", p)
	}
}

func TestAutomationFields(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var v automations.FieldsView
	ts.call("GET", "/api/automations/fields", nil, 200, &v)
	if v.DefaultSync != model.SyncLink {
		t.Fatalf("default = %+v", v)
	}
	body := map[string]any{"default_sync": "mirror", "push": []string{"title"}, "props": map[string]string{"milestone": "target"}}
	ts.call("PATCH", "/api/automations/fields", body, 200, &v)
	if v.DefaultSync != "mirror" || !slices.Equal(v.Push, []string{"title"}) || v.Props["milestone"] != "target" {
		t.Errorf("patch = %+v", v)
	}
	if p := ts.problem("PATCH", "/api/automations/fields", map[string]any{"default_sync": "off"}, 422); p.Type != "automation.invalid" {
		t.Errorf("bad mode = %+v", p)
	}
	for _, field := range []string{"push", "mirror"} {
		if p := ts.problem("PATCH", "/api/automations/fields", map[string]any{field: []string{"title", "labels"}}, 422); p.Type != "automation.invalid" {
			t.Errorf("labels in %s = %+v", field, p)
		}
	}
}

func TestAutomationStatsAndActivity(t *testing.T) {
	t.Parallel()
	ts, fake := newGitHubServer(t)
	sc, f := schema.Dev(), automation.DevRules()
	sc.Workflow.Required = schema.Lists{{Key: "review", Values: []string{"estimate"}}}
	b, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ts.uc.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		if err := tx.PutSchema(sc); err != nil {
			return err
		}
		return tx.PutFile(model.AutomationsPath, b)
	})
	id := ts.add("a", "")
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"repos": []string{webRepo}}, 200, nil)
	if _, err := fake.OpenPull("org-a/web", "hh/"+id+"-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	ts.sync()
	var stats []automations.RuleStat
	ts.call("GET", "/api/automations/stats", nil, 200, &stats)
	if len(stats) != 1 || stats[0].Rule != "pr-opened" || stats[0].Applied != 1 {
		t.Fatalf("stats = %+v", stats)
	}

	var auto, people, syncs []usecase.LogView
	ts.call("GET", "/api/activity?automation=only", nil, 200, &auto)
	var ops []string
	for _, l := range auto {
		ops = append(ops, *l.Op)
	}
	if !slices.Contains(ops, "gh-fetch") || !slices.Contains(ops, "set") {
		t.Errorf("automation = %v", ops)
	}
	ts.call("GET", "/api/activity?sync=exclude&automation=exclude", nil, 200, &people)
	for _, l := range people {
		if usecase.AutomatedCommit(l.Trailer) {
			t.Errorf("people has %s", l.Subject)
		}
	}
	if len(people) == 0 {
		t.Error("what people did is gone")
	}
	ts.call("GET", "/api/activity?sync=only", nil, 200, &syncs)
	if len(syncs) != 0 {
		t.Errorf("sync = %+v", syncs)
	}
	ts.problem("GET", "/api/activity?automation=maybe", nil, 400)
}
