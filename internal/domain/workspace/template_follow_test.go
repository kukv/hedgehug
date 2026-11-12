package workspace_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestTemplateValueDrafts(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	high, low := s.Priorities[1].ID, s.Priorities[3].ID
	web, api := s.Systems[0].ID, s.Systems[1].ID
	s.Templates = []workspace.Template{{
		ID: tplA, Name: "A", Kind: "Epic",
		Values: map[change.ID]json.RawMessage{"priority": raw(high), "systems": raw([]change.ID{web, api})},
		Children: []workspace.TemplateChild{{
			ID: child, Kind: "Story", Title: "QA",
			Values: map[change.ID]json.RawMessage{"systems": raw([]change.ID{web})},
		}},
	}}
	got := workspace.TemplateValueDrafts(s, workspace.List{Kind: workspace.ListPriorities}, high, low)
	want := []change.Draft{change.NewSet(tf(tplA, "values", "priority"), raw(high), low)}
	if !sameDrafts(got, want) {
		t.Errorf("priority: %v", got)
	}
	got = workspace.TemplateValueDrafts(s, workspace.List{Kind: workspace.ListSystems}, web, "")
	want = []change.Draft{
		change.NewSet(tf(tplA, "values", "systems"), raw([]change.ID{web, api}), []change.ID{api}),
		change.NewSet(tf(tplA, "children", string(child), "values", "systems"), raw([]change.ID{web}), nil),
	}
	if !sameDrafts(got, want) {
		t.Errorf("systems: %v", got)
	}
	if got := workspace.TemplateValueDrafts(s, workspace.List{Kind: workspace.ListStatuses}, s.Statuses[0].ID, s.Statuses[1].ID); got != nil {
		t.Errorf("statuses: %v", got)
	}
}

func TestTemplatePropertyDrafts(t *testing.T) {
	s := workspace.Settings{Templates: []workspace.Template{
		{ID: tplA, Name: "A", Kind: "Task", Values: map[change.ID]json.RawMessage{envProp: raw("本番")}},
		{ID: tplB, Name: "B", Kind: "Task"},
	}}
	want := []change.Draft{change.NewSet(tf(tplA, "values", string(envProp)), raw("本番"), nil)}
	if got := workspace.TemplatePropertyDrafts(s, envProp); !sameDrafts(got, want) {
		t.Errorf("drafts = %v", got)
	}
}
