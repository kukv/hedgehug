package item_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestDisplay(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	names := func(login string) string {
		if login == "taro" {
			return "太郎"
		}
		return login
	}
	today := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	p := func(k string) workspace.Property { return byKey(t, st, k) }
	cases := []struct {
		p    workspace.Property
		v    string
		want string
	}{
		{p("status"), `"` + string(st.Statuses[2].ID) + `"`, "Done"},
		{p("status"), `"01GONE00000000000000000000"`, "(削除された値)"},
		{p("assignee"), `"taro"`, "太郎"},
		{p("assignee"), `null`, "なし"},
		{p("start"), `"2026-11-11"`, "11/11"},
		{p("end"), `"2027-01-05"`, "2027/01/05"},
		{p("estimate"), `1.5`, "1.5 時間"},
		{p("estimate"), `2`, "2 時間"},
		{p("points"), `3`, "3"},
		{p("systems"), `["` + string(st.Systems[0].ID) + `","` + string(st.Systems[1].ID) + `"]`, "Web, API"},
		{p("systems"), `[]`, "なし"},
		{workspace.Property{Type: workspace.TypeCheck}, `true`, "オン"},
		{workspace.Property{Type: workspace.TypeCheck}, `false`, "オフ"},
		{workspace.Property{Type: workspace.TypeText}, `"memo"`, "memo"},
		// 数でない値は panic せず、文字列の形で返す
		{workspace.Property{Type: workspace.TypeDuration}, `"abc"`, "abc"},
		{workspace.Property{Type: workspace.TypeNumber}, `"x"`, "x"},
		{p("estimate"), `"abc"`, "abc"},
		{p("points"), `"abc"`, "abc"},
	}
	for _, c := range cases {
		if got := item.Display(c.p, json.RawMessage(c.v), st, names, today); got != c.want {
			t.Errorf("Display(%s, %s) = %q, want %q", c.p.Key, c.v, got, c.want)
		}
	}
}
