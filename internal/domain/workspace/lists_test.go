package workspace_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestListRefs(t *testing.T) {
	const p change.ID = "01PROP0000000000000000000A"
	const v change.ID = "01VAL00000000000000000000A"
	cases := []struct {
		in       string
		ref, fld string
	}{
		{"statuses", "statuses/" + string(v), "statuses/" + string(v) + "/name"},
		{"systems", "systems/" + string(v), "systems/" + string(v) + "/name"},
		{string(p), "choices/" + string(p) + "/" + string(v), "choices/" + string(p) + "/" + string(v) + "/name"},
	}
	for _, c := range cases {
		l := workspace.ParseList(c.in)
		if l.Ref(v) != c.ref || l.Field(v, "name") != c.fld {
			t.Errorf("%s: ref %q field %q", c.in, l.Ref(v), l.Field(v, "name"))
		}
	}
}

func TestLabelsAndSingle(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	single := workspace.Property{ID: "01PROP0000000000000000000A", Name: "顧客", Type: workspace.TypeSingle}
	multi := workspace.Property{ID: "01PROP0000000000000000000B", Name: "タグ", Type: workspace.TypeMulti}
	text := workspace.Property{ID: "01PROP0000000000000000000C", Name: "メモ", Type: workspace.TypeText}
	s.Properties = append(s.Properties, single, multi, text)
	cases := []struct {
		list   string
		label  string
		single bool
		ok     bool
	}{
		{"statuses", "ステータス", true, true},
		{"priorities", "優先度", true, true},
		{"systems", "関連システム", false, true},
		{string(single.ID), "「顧客」の選択肢", true, true},
		{string(multi.ID), "「タグ」の選択肢", false, true},
		{string(text.ID), "", false, false},
		{"01NOPE00000000000000000000", "", false, false},
	}
	for _, c := range cases {
		l := workspace.ParseList(c.list)
		_, ok := s.Values(l)
		if ok != c.ok || (ok && (s.Label(l) != c.label || s.Single(l) != c.single)) {
			t.Errorf("%s: ok %v label %q single %v", c.list, ok, s.Label(l), s.Single(l))
		}
	}
}
