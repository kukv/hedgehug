package query_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

type row struct {
	Name string
	F    query.Facts
}

func rowFacts(r row) *query.Facts { return &r.F }

func mk(t *testing.T, name string, mut ...func(*query.Facts)) row {
	t.Helper()
	return row{Name: name, F: facts(t, mut...)}
}

func order(rows []row, s *query.Sort) string {
	query.Order(rows, rowFacts, s)
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Name
	}
	return strings.Join(names, " ")
}

func sortOf(t *testing.T, text string) *query.Sort { t.Helper(); return mustParse(t, text).Sort }

func TestOrderDefault(t *testing.T) {
	at := func(n int) func(*query.Facts) {
		return func(f *query.Facts) { f.Item.Created.At = time.Unix(int64(n), 0) }
	}
	rows := []row{
		mk(t, "s1w1-late", func(f *query.Facts) { f.StoreOrder = 1; f.Item.Rank = "a" }),
		mk(t, "s0w1-rankb", func(f *query.Facts) { f.Item.Rank = "b" }),
		mk(t, "s0w1-rankA-t2", func(f *query.Facts) { f.Item.Rank = "a"; at(2)(f) }),
		mk(t, "s0w1-rankA-t1", func(f *query.Facts) { f.Item.Rank = "a"; at(1)(f) }),
		mk(t, "s0w0", func(f *query.Facts) { f.WorkspaceOrder = -1; f.Item.Rank = "z" }),
		mk(t, "s0w1-rankA-t1-id", func(f *query.Facts) { f.Item.Rank = "a"; f.Item.ID = "z"; at(1)(f) }),
	}
	want := "s0w0 s0w1-rankA-t1 s0w1-rankA-t1-id s0w1-rankA-t2 s0w1-rankb s1w1-late"
	if got := order(rows, nil); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestOrderKeys(t *testing.T) {
	num := func(n int) func(*query.Facts) { return func(f *query.Facts) { f.Item.Place.Number = n } }
	title := func(s string) func(*query.Facts) { return func(f *query.Facts) { f.Item.Title = s } }
	kind := func(k item.Kind) func(*query.Facts) { return func(f *query.Facts) { f.Item.Kind = k } }
	status := func(id change.ID) func(*query.Facts) { return func(f *query.Facts) { f.Item.Status = id } }
	float := func(v float64) *float64 { return &v }
	prios := func(id change.ID) func(*query.Facts) {
		return func(f *query.Facts) {
			f.Settings.Priorities = []workspace.Value{{ID: "p0", Name: "High"}, {ID: "p1", Name: "Low"}}
			f.Item.Priority = id
		}
	}
	tests := []struct {
		key  string
		rows []row
		want string
	}{
		{"id", []row{mk(t, "b", num(10)), mk(t, "a", num(2))}, "a b"},
		{"title", []row{mk(t, "b", title("b")), mk(t, "a", title("a"))}, "a b"},
		{"kind", []row{mk(t, "step", kind(item.Step)), mk(t, "epic", kind(item.Epic)), mk(t, "bug", kind(item.Bug))}, "epic bug step"},
		{"status", []row{mk(t, "done", status("s2")), mk(t, "doing", status("s1"))}, "doing done"},
		{"priority", []row{mk(t, "low", prios("p1")), mk(t, "high", prios("p0"))}, "high low"},
		{"assignee", []row{
			mk(t, "t", func(f *query.Facts) { f.Item.Assignee = "tanaka"; f.Names = map[string]string{"tanaka": "Zed"} }),
			mk(t, "s", func(f *query.Facts) { f.Item.Assignee = "suzuki"; f.Names = map[string]string{"suzuki": "Abe"} }),
			mk(t, "u", func(f *query.Facts) { f.Item.Assignee = "Mori" }), // 名簿にないものはログイン名で比べる
		}, "s u t"},
		{"project", []row{mk(t, "b", func(f *query.Facts) { f.ProjectTitle = "B" }), mk(t, "a", func(f *query.Facts) { f.ProjectTitle = "A" })}, "a b"},
		{"parent", []row{
			mk(t, "p10", func(f *query.Facts) { f.Parent = &query.Ref{Display: "HH-10"} }),
			mk(t, "p2", func(f *query.Facts) { f.Parent = &query.Ref{Display: "HH-2"} }),
		}, "p2 p10"},
		{"sprint", []row{mk(t, "b", func(f *query.Facts) { f.SprintName = "S2" }), mk(t, "a", func(f *query.Facts) { f.SprintName = "S1" })}, "a b"},
		{"release", []row{mk(t, "b", func(f *query.Facts) { f.ReleaseName = "v2" }), mk(t, "a", func(f *query.Facts) { f.ReleaseName = "v1" })}, "a b"},
		{"start", []row{mk(t, "b", func(f *query.Facts) { f.Item.Start = "2026-02-01" }), mk(t, "a", func(f *query.Facts) { f.Item.Start = "2026-01-01" })}, "a b"},
		{"end", []row{mk(t, "b", func(f *query.Facts) { f.Item.End = "2026-02-01" }), mk(t, "a", func(f *query.Facts) { f.Item.End = "2026-01-01" })}, "a b"},
		{"points", []row{mk(t, "b", func(f *query.Facts) { f.Item.Points = float(8) }), mk(t, "a", func(f *query.Facts) { f.Item.Points = float(3) })}, "a b"},
		{"estimate", []row{mk(t, "b", func(f *query.Facts) { f.Item.Estimate = float(8) }), mk(t, "a", func(f *query.Facts) { f.Item.Estimate = float(3) })}, "a b"},
		{"created", []row{mk(t, "new", func(f *query.Facts) { f.Created = time.Unix(9, 0) }), mk(t, "old", func(f *query.Facts) { f.Created = time.Unix(1, 0) })}, "old new"},
		{"updated", []row{mk(t, "new", func(f *query.Facts) { f.Updated = time.Unix(9, 0) }), mk(t, "old", func(f *query.Facts) { f.Updated = time.Unix(1, 0) })}, "old new"},
	}
	for _, tt := range tests {
		asc := append([]row(nil), tt.rows...)
		if got := order(asc, sortOf(t, "sort:"+tt.key)); got != tt.want {
			t.Errorf("sort:%s = %s, want %s", tt.key, got, tt.want)
		}
		w := strings.Fields(tt.want)
		for i, j := 0, len(w)-1; i < j; i, j = i+1, j-1 {
			w[i], w[j] = w[j], w[i]
		}
		if got := order(tt.rows, sortOf(t, "sort:-"+tt.key)); got != strings.Join(w, " ") {
			t.Errorf("sort:-%s = %s, want %s", tt.key, got, strings.Join(w, " "))
		}
	}
}

func TestOrderCustomProperties(t *testing.T) {
	with := func(p workspace.Property, mut func(*query.Facts)) func(*query.Facts) {
		return func(f *query.Facts) {
			f.Settings.Properties = []workspace.Property{p}
			mut(f)
		}
	}
	single := workspace.Property{
		ID: "c1", Key: "Size", Type: workspace.TypeSingle,
		Choices: []workspace.Value{{ID: "z", Name: "S"}, {ID: "y", Name: "L"}},
	}
	rows := []row{
		mk(t, "L", with(single, func(f *query.Facts) { f.Item.Choices = map[change.ID][]change.ID{"c1": {"y"}} })),
		mk(t, "S", with(single, func(f *query.Facts) { f.Item.Choices = map[change.ID][]change.ID{"c1": {"z"}} })),
	}
	if got := order(rows, sortOf(t, "sort:size")); got != "S L" {
		t.Errorf("single = %s", got)
	}
	num := workspace.Property{ID: "c2", Key: "cost", Type: workspace.TypeNumber}
	rows = []row{
		mk(t, "n10", with(num, func(f *query.Facts) { f.Item.Fields = map[change.ID]json.RawMessage{"c2": json.RawMessage("10")} })),
		mk(t, "n2", with(num, func(f *query.Facts) { f.Item.Fields = map[change.ID]json.RawMessage{"c2": json.RawMessage("2")} })),
		mk(t, "none", with(num, func(f *query.Facts) {})),
	}
	if got := order(rows, sortOf(t, "sort:cost")); got != "n2 n10 none" {
		t.Errorf("number = %s", got)
	}
}

func TestOrderEmptyLast(t *testing.T) {
	mkRows := func() []row {
		return []row{
			mk(t, "none"),
			mk(t, "b", func(f *query.Facts) { f.SprintName = "B" }),
			mk(t, "a", func(f *query.Facts) { f.SprintName = "A" }),
		}
	}
	if got := order(mkRows(), sortOf(t, "sort:sprint")); got != "a b none" {
		t.Errorf("asc = %s", got)
	}
	if got := order(mkRows(), sortOf(t, "sort:-sprint")); got != "b a none" {
		t.Errorf("desc = %s", got)
	}
}

func TestOrderTieByPlace(t *testing.T) {
	rows := []row{
		mk(t, "s1", func(f *query.Facts) { f.StoreOrder = 1; f.SprintName = "A" }),
		mk(t, "w2", func(f *query.Facts) { f.WorkspaceOrder = 2; f.SprintName = "A" }),
		mk(t, "rb", func(f *query.Facts) { f.Item.Rank = "b"; f.SprintName = "A" }),
		mk(t, "ra", func(f *query.Facts) { f.Item.Rank = "a"; f.SprintName = "A" }),
	}
	if got := order(rows, sortOf(t, "sort:-sprint")); got != "ra rb w2 s1" {
		t.Errorf("tie = %s", got)
	}
}

func TestOrderSystemsAcrossWorkspaces(t *testing.T) {
	sys := func(ws change.ID, wsOrder int, defs []string, ids ...change.ID) func(*query.Facts) {
		return func(f *query.Facts) {
			f.Workspace.ID = ws
			f.WorkspaceOrder = wsOrder
			f.Settings.Systems = nil
			for _, d := range defs {
				f.Settings.Systems = append(f.Settings.Systems, workspace.Value{ID: change.ID(strings.ToLower(d)), Name: d})
			}
			f.Item.Systems = ids
		}
	}
	// W1 の定義: API, DB。W2 の定義: Web, db, API。最初に現れた位置: API=0, DB=1, Web=2
	w1 := []string{"API", "DB"}
	w2 := []string{"Web", "db", "API"}
	rows := []row{
		mk(t, "w2-web", sys("w2", 1, w2, "web")),
		mk(t, "w1-db", sys("w1", 0, w1, "db")),
		mk(t, "w2-api-web", sys("w2", 1, w2, "web", "api")), // 並べ直して API, Web
		mk(t, "w1-api", sys("w1", 0, w1, "api")),
		mk(t, "w1-api-db", sys("w1", 0, w1, "db", "api")), // API, DB
		mk(t, "w1-none", sys("w1", 0, w1)),
	}
	want := "w1-api w1-api-db w2-api-web w1-db w2-web w1-none"
	if got := order(rows, sortOf(t, "sort:system")); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestOrderUnknownKey(t *testing.T) {
	rows := []row{
		mk(t, "b", func(f *query.Facts) { f.Item.Rank = "b" }),
		mk(t, "a", func(f *query.Facts) { f.Item.Rank = "a" }),
	}
	if got := order(rows, sortOf(t, "sort:foo")); got != "a b" {
		t.Errorf("got %s", got)
	}
	if got := order(rows, sortOf(t, "sort:-watching")); got != "a b" {
		t.Errorf("builtin without order: got %s", got)
	}
}
