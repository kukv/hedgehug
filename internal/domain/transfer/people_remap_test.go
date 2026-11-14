package transfer_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func rawStr(v string) json.RawMessage { b, _ := json.Marshal(v); return b }

func userSettings() workspace.Settings {
	s := settings("F")
	s.Properties = append(s.Properties, workspace.Property{ID: "FP5", Key: "reviewer", Name: "レビュアー", Type: workspace.TypeUser})
	return s
}

func TestSettingsPeople(t *testing.T) {
	s := userSettings()
	s.Templates = []workspace.Template{{
		ID: "T1", Values: map[change.ID]json.RawMessage{"assignee": rawStr("taro"), "FP5": rawStr("saburo")},
		Steps: []workspace.TemplateStep{{ID: "S1", Assignee: workspace.StepParent}, {ID: "S2", Assignee: workspace.StepNone}},
		Children: []workspace.TemplateChild{{
			ID: "C1", Values: map[change.ID]json.RawMessage{"assignee": rawStr("Hanako")},
			Steps: []workspace.TemplateStep{{ID: "S3", Assignee: workspace.StepParent}, {ID: "S4", Assignee: "jiro"}},
		}},
	}}
	rules := []automation.Rule{
		{ID: "R1", Action: automation.ActionAssignee, Value: "shiro"},
		{ID: "R2", Action: automation.ActionAssignee, Value: automation.Unassigned},
		{ID: "R3", Action: automation.ActionStatus, Value: "FV01"},
	}
	got := transfer.SettingsPeople(s, rules)
	slices.Sort(got)
	if want := "Hanako,jiro,saburo,shiro,taro"; strings.Join(got, ",") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
}

func showDrafts(ds []change.Draft) []string {
	var out []string
	for _, d := range ds {
		out = append(out, string(d.T)+" "+d.F+" "+string(d.From)+">"+string(d.To))
	}
	return out
}

func TestRemapPeopleOnlySpelling(t *testing.T) {
	s := userSettings()
	it := item.Item{
		ID: "I1", Status: "FV01", Assignee: "taro", Watchers: []string{"taro", "hanako"},
		Fields: map[change.ID]json.RawMessage{"FP5": rawStr("taro")},
	}
	got := showDrafts(transfer.RemapPeople(it, s, transfer.PeopleMap{"taro": "Taro", "hanako": "hanako"}))
	want := []string{
		`set assignee "taro">"Taro"`,
		`set fields/FP5 "taro">"Taro"`,
		`remove watchers "taro">`,
		`add watchers >"Taro"`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if transfer.RemapPeople(it, s, nil) != nil {
		t.Fatal("同じ Store は行を出さない")
	}
}

func TestRemapSettingsPeople(t *testing.T) {
	s := userSettings()
	s.Templates = []workspace.Template{{
		ID: "T1", Values: map[change.ID]json.RawMessage{"assignee": rawStr("taro"), "FP5": rawStr("jiro")},
		Steps: []workspace.TemplateStep{{ID: "S1", Assignee: "taro"}, {ID: "S2", Assignee: workspace.StepParent}},
	}}
	rules := []automation.Rule{
		{ID: "R1", Action: automation.ActionAssignee, Value: "taro"},
		{ID: "R2", Action: automation.ActionAssignee, Value: "jiro"},
	}
	got := showDrafts(transfer.RemapSettingsPeople(s, rules, transfer.PeopleMap{"taro": "Taro", "jiro": "jiro"}))
	want := []string{
		`set templates/T1/values/assignee "taro">"Taro"`,
		`set templates/T1/steps/S1/assignee "taro">"Taro"`,
		`set rules/R1/value "taro">"Taro"`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
