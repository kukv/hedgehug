package query_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

var groupNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.Local)

func ptrs(fs ...query.Facts) []*query.Facts {
	out := make([]*query.Facts, len(fs))
	for i := range fs {
		out[i] = &fs[i]
	}
	return out
}

func withStatus(id change.ID) func(*query.Facts) {
	return func(f *query.Facts) { f.Item.Status = id }
}

func TestGroupKey(t *testing.T) {
	cases := []struct{ setting, q, want string }{
		{"assignee", "kind", "assignee"},
		{"none", "kind", "kind"},
		{"", "foo", ""},
		{"none", "", ""},
		{"bogus", "status", "status"},
	}
	for _, c := range cases {
		if got := query.GroupKey(c.setting, c.q); got != c.want {
			t.Errorf("GroupKey(%q, %q) = %q, want %q", c.setting, c.q, got, c.want)
		}
	}
}

func TestGroupsOrderOfFirstAppearance(t *testing.T) {
	got := query.Groups(ptrs(facts(t, withStatus("s2")), facts(t), facts(t, withStatus("s2"))), "status", groupNow)
	want := []query.Group{{Label: "Done", Rows: []int{0, 2}}, {Label: "InProgress", Rows: []int{1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupsStatusByNameAcrossWorkspaces(t *testing.T) {
	a := facts(t, func(f *query.Facts) {
		f.Settings.Statuses = []workspace.Value{{ID: "a1", Name: "Review"}}
		f.Item.Status = "a1"
	})
	b := facts(t, func(f *query.Facts) {
		f.Settings.Statuses = []workspace.Value{{ID: "b1", Name: "review"}}
		f.Item.Status = "b1"
	})
	got := query.Groups(ptrs(a, b), "status", groupNow)
	want := []query.Group{{Label: "Review", Rows: []int{0, 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupsEmptyLabels(t *testing.T) {
	empty := facts(t, func(f *query.Facts) {
		f.Item = item.Item{}
		f.Parent = nil
		f.SprintName = ""
	})
	for key, label := range map[string]string{
		"assignee": "未割り当て", "project": "なし", "status": "なし", "parent": "親なし",
		"sprint": "スプリントなし", "system": "関連システムなし",
	} {
		got := query.Groups(ptrs(empty), key, groupNow)
		if len(got) != 1 || got[0].Label != label {
			t.Errorf("%s: got %v, want label %q", key, got, label)
		}
	}
}

func TestGroupsAssigneeUnknown(t *testing.T) {
	names := map[string]string{"tanaka": "田中"}
	as := func(login string) func(*query.Facts) {
		return func(f *query.Facts) { f.Names = names; f.Item.Assignee = login }
	}
	got := query.Groups(ptrs(facts(t, as("Tanaka")), facts(t, as("tanaka")), facts(t, as("Ghost"))), "assignee", groupNow)
	want := []query.Group{{Label: "田中", Rows: []int{0, 1}}, {Label: "不明なユーザー", Rows: []int{2}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupsParentLabel(t *testing.T) {
	f := facts(t, func(f *query.Facts) {
		id := item.ID("i3")
		f.Item.Place.Parent = &id
		f.Parent = &query.Ref{Display: "HH-3"}
		f.ParentTitle = "ログイン"
	})
	got := query.Groups(ptrs(f), "parent", groupNow)
	if len(got) != 1 || got[0].Label != "HH-3 ログイン" {
		t.Errorf("got %v", got)
	}
}

func TestGroupsStoreAndIDIdentity(t *testing.T) {
	proj := func(store string, id project.ID) func(*query.Facts) {
		return func(f *query.Facts) { f.StoreName = store; f.Item.Place.Project = id; f.ProjectTitle = "Web" }
	}
	got := query.Groups(ptrs(facts(t, proj("A", "p1")), facts(t, proj("B", "p1")), facts(t, proj("A", "p1"))), "project", groupNow)
	want := []query.Group{{Label: "Web", Rows: []int{0, 2}}, {Label: "Web", Rows: []int{1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupsSystemsMultiple(t *testing.T) {
	set := func(ids ...change.ID) func(*query.Facts) {
		return func(f *query.Facts) {
			f.Settings.Systems = []workspace.Value{{ID: "w", Name: "Web"}, {ID: "a", Name: "API"}, {ID: "i", Name: "iOS"}}
			f.Item.Systems = ids
		}
	}
	got := query.Groups(ptrs(facts(t, set("a", "w")), facts(t, set("a"))), "system", groupNow)
	want := []query.Group{{Label: "Web", Rows: []int{0}}, {Label: "API", Rows: []int{0, 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupsRemovedValue(t *testing.T) {
	got := query.Groups(ptrs(facts(t, withStatus("gone"))), "status", groupNow)
	if len(got) != 1 || got[0].Label != "(削除された値)" {
		t.Errorf("got %v", got)
	}
}

func TestGroupsCreatedByLocalDay(t *testing.T) {
	at := func(d, h, m int) func(*query.Facts) {
		return func(f *query.Facts) { f.Created = time.Date(2026, 11, d, h, m, 0, 0, time.Local) }
	}
	got := query.Groups(ptrs(facts(t, at(15, 23, 30)), facts(t, at(16, 0, 10))), "created", groupNow)
	want := []query.Group{{Label: "11/15", Rows: []int{0}}, {Label: "11/16", Rows: []int{1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGroupsNoKey(t *testing.T) {
	if got := query.Groups(ptrs(facts(t)), "", groupNow); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
