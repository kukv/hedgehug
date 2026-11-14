package query_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const today = "2026-11-15"

func mustParse(t *testing.T, text string) query.Query {
	t.Helper()
	q, errs := query.Parse(text, nil)
	if len(errs) > 0 {
		t.Fatalf("%q: %v", text, errs)
	}
	return q
}

// facts は、HH-1(Story、InProgress、High、api と db のシステム)の材料を作り、mut で変える。
func facts(t *testing.T, mut ...func(*query.Facts)) query.Facts {
	t.Helper()
	f := query.Facts{
		StoreName: "Team",
		Workspace: workspace.Workspace{ID: "w1", Name: "Web", Prefix: "HH"},
		Settings: workspace.Settings{
			Statuses: []workspace.Value{
				{ID: "s1", Name: "InProgress", Category: workspace.CategoryDoing},
				{ID: "s2", Name: "Done", Category: workspace.CategoryDone},
			},
			Priorities: []workspace.Value{{ID: "p1", Name: "High"}},
			Systems:    []workspace.Value{{ID: "y1", Name: "API"}, {ID: "y2", Name: "DB"}},
		},
		Item: item.Item{
			Kind: item.Story, Title: "ガントチャートを作る", Status: "s1", Priority: "p1",
			Systems: []change.ID{"y1", "y2"}, Assignee: "Tanaka", Watchers: []string{"Suzuki"},
		},
		Display:     "HH-1",
		Description: "ガント 表の説明",
		Project:     query.Ref{Display: "HH-P1", Aliases: []string{"OLD-P1"}},
		Parent:      &query.Ref{Display: "HH-2"},
		Ancestors:   []query.Ref{{Display: "HH-2"}, {Display: "HH-9"}},
		Relations: map[string][]query.Ref{
			"blocked_by": {{Display: "HH-3"}},
			"relates_to": {{Display: "HH-3"}},
		},
		Me: "tanaka",
	}
	for _, m := range mut {
		m(&f)
	}
	return f
}

func check(t *testing.T, f query.Facts, cases map[string]bool) {
	t.Helper()
	for in, want := range cases {
		if got := query.Match(mustParse(t, in), f, today).Match; got != want {
			t.Errorf("%q = %v, want %v", in, got, want)
		}
	}
}

func TestStatusAndCategory(t *testing.T) {
	check(t, facts(t), map[string]bool{
		"status:inprogress":      true,
		"status:Done":            false,
		"status:Done,progress":   false,
		"category:doing":         true,
		"category:done,canceled": false,
		"-status:Done":           true,
	})
}

func TestKindSystemPriorityWorkspaceStore(t *testing.T) {
	check(t, facts(t), map[string]bool{
		"kind:Story":     true,
		"kind:bug,story": true,
		"kind:Bug":       false,
		"system:api":     true,
		"system:web":     false,
		"system:none":    false,
		"priority:HIGH":  true,
		"priority:low":   false,
		"workspace:web":  true,
		"workspace:hh":   true,
		"workspace:x":    false,
		"store:Team":     true,
		"store:other":    false,
	})
	check(t, facts(t, func(f *query.Facts) { f.Item.Systems = nil }), map[string]bool{"system:none": true, "system:api": false})
}

func TestAssigneeWatching(t *testing.T) {
	check(t, facts(t), map[string]bool{
		"assignee:@Tanaka": true,
		"assignee:@me":     true,
		"assignee:suzuki":  false,
		"assignee:none":    false,
		"watching:none":    false,
		"watching:suzuki":  true,
		"watching:@me":     false,
	})
	check(t, facts(t, func(f *query.Facts) { f.Me = "" }), map[string]bool{"assignee:@me": false, "-assignee:@me": true})
	check(t, facts(t, func(f *query.Facts) { f.Item.Assignee = ""; f.Item.Watchers = nil }),
		map[string]bool{"assignee:none": true, "watching:none": true, "assignee:@me": false})
}

func TestIDKeys(t *testing.T) {
	check(t, facts(t), map[string]bool{
		"project:hh-p1":     true,
		"project:OLD-P1":    true,
		"project:HH-P2":     false,
		"parent:HH-2":       true,
		"parent:HH-9":       true,
		"parent:HH-7":       false,
		"epic:HH-9":         true,
		"epic:none":         false,
		"blocked_by:HH-3":   true,
		"relates_to:HH-3":   true,
		"blocks:none":       true,
		"blocks:HH-3":       false,
		"blocked_by:none":   false,
		"blocked_by:x,HH-3": true,
	})
	check(t, facts(t, func(f *query.Facts) { f.Parent = nil; f.Ancestors = nil }), map[string]bool{"epic:none": true, "parent:none": true})
}

func TestFreeWord(t *testing.T) {
	check(t, facts(t, func(f *query.Facts) { f.Display = "HH-12" }), map[string]bool{
		"HH-1":           true,
		"hh-12":          true,
		"チャート":           true,
		"説明":             true,
		`"ガント 表"`:        true,
		"ガント 説明":         true,
		"ガント 無い":         false,
		"-ガント":           false,
		"-無い":            true,
		"kind:story ガント": true,
		"kind:bug ガント":   false,
	})
}

func TestFreeWordOldPrefixBothMatch(t *testing.T) {
	q := mustParse(t, "HH-1")
	old := facts(t, func(f *query.Facts) { f.Display = "WEB-1"; f.Aliases = []string{"HH-1"} })
	if got := query.Match(q, old, today); got != (query.Hit{Match: true, OldID: true}) {
		t.Errorf("旧 ID: %+v", got)
	}
	cur := facts(t, func(f *query.Facts) { f.Aliases = []string{"HH-1"} })
	if got := query.Match(q, cur, today); got != (query.Hit{Match: true}) {
		t.Errorf("いまの ID: %+v", got)
	}
	if got := query.Match(mustParse(t, "-HH-1"), facts(t), today); got.OldID {
		t.Errorf("否定は OldID にしない: %+v", got)
	}
}

func TestIsAndHasUnknown(t *testing.T) {
	check(t, facts(t), map[string]bool{"is:foo": false, "-is:foo": true, "has:foo": false})
}

func TestEmptyFreeWordIsNoCondition(t *testing.T) {
	q, _ := query.Parse(`"" " "`, nil)
	if !query.Match(q, facts(t), today).Match {
		t.Error("空の語は条件にしない")
	}
}
