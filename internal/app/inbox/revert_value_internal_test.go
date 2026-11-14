package inbox

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

var revertNow = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

// revertRows は、drafts を 1 行ずつ別の操作として並べる。
func revertRows(drafts ...change.Draft) []change.Record {
	var log []change.Record
	for i, d := range drafts {
		id := change.ID(fmt.Sprintf("01REC%021d", i))
		meta := change.Meta{Op: id, At: revertNow.Add(time.Duration(i) * time.Second), Actor: change.User("taro")}
		log = append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return id })...)
	}
	return log
}

func revertSettings(t *testing.T, extra ...change.Draft) (workspace.Settings, []change.Record) {
	t.Helper()
	n := 0
	s := workspace.DefaultSettings(func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01VAL%021d", n))
	})
	create, err := workspace.CreateWithSettings(workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}, s)
	if err != nil {
		t.Fatal(err)
	}
	log := revertRows(append([]change.Draft{create}, extra...)...)
	got, ok := workspace.SettingsFromLog(log)
	if !ok {
		t.Fatal("no settings")
	}
	return got, log
}

func propertyOf(st workspace.Settings, typ workspace.PropertyType) workspace.Property {
	for _, p := range st.Properties {
		if p.Type == typ {
			return p
		}
	}
	return workspace.Property{}
}

func TestSameValue(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"null と空文字", `null`, `""`, true},
		{"null と false", `null`, `false`, true},
		{"null と空の配列", `null`, `[]`, true},
		{"nil と null", ``, `null`, true},
		{"配列は順序を見ない", `["b","a"]`, `["a","b"]`, true},
		{"配列の要素が違う", `["a"]`, `["a","b"]`, false},
		{"数は値で比べる", `1`, `1.0`, true},
		{"文字列が違う", `"Todo"`, `"Done"`, false},
		{"空と値", `null`, `"Todo"`, false},
		{"false と true", `false`, `true`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameValue(json.RawMessage(tt.a), json.RawMessage(tt.b)); got != tt.want {
				t.Errorf("sameValue(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestShowPropertyMultiOrder(t *testing.T) {
	st, log := revertSettings(t)
	sys := propertyOf(st, workspace.TypeSystems)
	// 設定の並びと ID の辞書順が逆になる設定にする
	st.Systems = []workspace.Value{{ID: "01SYSB", Name: "Web"}, {ID: "01SYSA", Name: "API"}}
	got := showProperty(sys, json.RawMessage(`["01SYSA","01SYSB"]`), st, log, memberNames(nil), revertNow)
	if got != "Web, API" {
		t.Errorf("show = %q, want %q", got, "Web, API")
	}
	if got := showProperty(sys, json.RawMessage(`[]`), st, log, memberNames(nil), revertNow); got != "なし" {
		t.Errorf("show([]) = %q, want なし", got)
	}
}

func TestShowPropertyDeletedValue(t *testing.T) {
	todo := workspace.Value{ID: "01STATUS00000000000000000R", Name: "Todo", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	list := workspace.ParseList("statuses")
	drafts := workspace.ValueDrafts(list, todo)
	drafts = append(drafts, change.NewDelete(list.Ref(todo.ID)))
	st, log := revertSettings(t, drafts...)
	status := propertyOf(st, workspace.TypeStatus)
	if got := showProperty(status, json.RawMessage(`"`+string(todo.ID)+`"`), st, log, memberNames(nil), revertNow); got != "Todo" {
		t.Errorf("deleted = %q, want Todo", got)
	}
	if got := showProperty(status, json.RawMessage(`"01NONE0000000000000000000A"`), st, log, memberNames(nil), revertNow); got != "(削除された値)" {
		t.Errorf("unknown = %q, want (削除された値)", got)
	}
	live := st.Statuses[0]
	if got := showProperty(status, json.RawMessage(`"`+string(live.ID)+`"`), st, log, memberNames(nil), revertNow); got != live.Name {
		t.Errorf("live = %q, want %q", got, live.Name)
	}
}

func TestShowPlan(t *testing.T) {
	_, log := revertSettings(t,
		change.NewSet("sprints/01SPR0000000000000000000A/name", nil, "Sprint 25"),
		change.NewSet("sprints/01SPR0000000000000000000B/name", nil, "Sprint 24"),
		change.NewDelete("sprints/01SPR0000000000000000000B"),
		change.NewSet("releases/01REL0000000000000000000A/name", nil, "v1.0"),
	)
	tests := []struct{ v, prefix, want string }{
		{`"01SPR0000000000000000000A"`, "sprints/", "Sprint 25"},
		{`"01SPR0000000000000000000B"`, "sprints/", "Sprint 24"},
		{`"01REL0000000000000000000A"`, "releases/", "v1.0"},
		{`null`, "sprints/", "なし"},
		{`""`, "sprints/", "なし"},
	}
	for _, tt := range tests {
		if got := showPlan(json.RawMessage(tt.v), log, tt.prefix); got != tt.want {
			t.Errorf("showPlan(%s) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestShowProject(t *testing.T) {
	names := memberNames([]member.Member{{Login: "hanako", Name: "花子"}})
	tests := []struct{ f, v, want string }{
		{"owner", `"hanako"`, "花子"},
		{"owner", `"HANAKO"`, "花子"},
		{"owner", `"jiro"`, "不明なユーザー"},
		{"owner", `""`, "なし"},
		{"start", `"2026-05-01"`, "5/1"},
		{"end", `""`, "なし"},
		{"status", `"active"`, "active"},
	}
	for _, tt := range tests {
		if got := showProject(tt.f, json.RawMessage(tt.v), names, revertNow); got != tt.want {
			t.Errorf("showProject(%s, %s) = %q, want %q", tt.f, tt.v, got, tt.want)
		}
	}
}
