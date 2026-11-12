package workspace

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// Ranked は、子と Step の並びを、いまの並びで rank.First から振り直す(B3-10)。
func (t Template) Ranked() Template {
	t.Children = slices.Clone(t.Children)
	r := ""
	for i := range t.Children {
		r = rank.After(r)
		t.Children[i].Rank = r
		t.Children[i].Steps = rankSteps(t.Children[i].Steps)
	}
	t.Steps = rankSteps(t.Steps)
	return t
}

func rankSteps(ss []TemplateStep) []TemplateStep {
	ss = slices.Clone(ss)
	r := ""
	for i := range ss {
		r = rank.After(r)
		ss[i].Rank = r
	}
	return ss
}

// TemplateDrafts は、old から t への行(B3-4・B3-9)。変わった項目だけを書く。old がゼロ値なら作成で、
// 空でない項目をすべて書く。old にあって t にない子と Step は削除の行(消した子の Step は書かない)。
func TemplateDrafts(old, t Template) []change.Draft {
	fresh := old.ID == ""
	f := func(rest ...string) string { return TemplateField(t.ID, rest...) }
	var ds []change.Draft
	ds = setText(ds, f("name"), old.Name, t.Name, fresh)
	ds = setText(ds, f("kind"), old.Kind, t.Kind, fresh)
	ds = setText(ds, f("title"), old.Title, t.Title, fresh)
	ds = setText(ds, f("description"), old.Description, t.Description, fresh)
	ds = setValues(ds, f("values"), old.Values, t.Values)
	ds = childDrafts(ds, t.ID, old.Children, t.Children)
	ds = stepDrafts(ds, t.ID, "", old.Steps, t.Steps)
	for _, c := range t.Children {
		var oldSteps []TemplateStep
		if i := slices.IndexFunc(old.Children, func(o TemplateChild) bool { return o.ID == c.ID }); i >= 0 {
			oldSteps = old.Children[i].Steps
		}
		ds = stepDrafts(ds, t.ID, c.ID, oldSteps, c.Steps)
	}
	return ds
}

// setText は、文字の項目の行。fresh(新しいもの)なら空でないときだけ、from を null にして書く。
func setText(ds []change.Draft, f, from, to string, fresh bool) []change.Draft {
	switch {
	case fresh && to == "":
		return ds
	case fresh:
		return append(ds, change.NewSet(f, nil, to))
	case from == to:
		return ds
	}
	return append(ds, change.NewSet(f, from, to))
}

// setValues は、値の行。足した値・変わった値・なくなった値(null)を、キーの順に書く。
func setValues(ds []change.Draft, prefix string, old, next map[change.ID]json.RawMessage) []change.Draft {
	keys := make([]change.ID, 0, len(old)+len(next))
	for k := range old {
		keys = append(keys, k)
	}
	for k := range next {
		if _, ok := old[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		o, had := old[k]
		n, has := next[k]
		f := prefix + "/" + string(k)
		switch {
		case has && !had:
			ds = append(ds, change.NewSet(f, nil, n))
		case has && !sameJSON(o, n):
			ds = append(ds, change.NewSet(f, o, n))
		case !has:
			ds = append(ds, change.NewSet(f, o, nil))
		}
	}
	return ds
}

func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

func childDrafts(ds []change.Draft, t change.ID, old, next []TemplateChild) []change.Draft {
	for _, c := range next {
		i := slices.IndexFunc(old, func(o TemplateChild) bool { return o.ID == c.ID })
		var o TemplateChild
		if i >= 0 {
			o = old[i]
		}
		f := func(rest ...string) string {
			return TemplateField(t, append([]string{"children", string(c.ID)}, rest...)...)
		}
		ds = setText(ds, f("kind"), o.Kind, c.Kind, i < 0)
		ds = setText(ds, f("title"), o.Title, c.Title, i < 0)
		ds = setText(ds, f("description"), o.Description, c.Description, i < 0)
		ds = setText(ds, f("rank"), o.Rank, c.Rank, i < 0)
		ds = setValues(ds, f("values"), o.Values, c.Values)
	}
	for _, o := range old {
		if !slices.ContainsFunc(next, func(c TemplateChild) bool { return c.ID == o.ID }) {
			ds = append(ds, change.NewDelete(TemplateField(t, "children", string(o.ID))))
		}
	}
	return ds
}

// stepDrafts は、持ち主 parent(空ならもと)の Step の行。新しい Step は parent も書く。
func stepDrafts(ds []change.Draft, t, parent change.ID, old, next []TemplateStep) []change.Draft {
	for _, s := range next {
		i := slices.IndexFunc(old, func(o TemplateStep) bool { return o.ID == s.ID })
		var o TemplateStep
		if i >= 0 {
			o = old[i]
		} else {
			ds = append(ds, change.NewSet(TemplateField(t, "steps", string(s.ID), "parent"), nil, string(parent)))
		}
		f := func(field string) string { return TemplateField(t, "steps", string(s.ID), field) }
		ds = setText(ds, f("title"), o.Title, s.Title, i < 0)
		ds = setText(ds, f("assignee"), o.Assignee, s.Assignee, i < 0)
		ds = setText(ds, f("rank"), o.Rank, s.Rank, i < 0)
	}
	for _, o := range old {
		if !slices.ContainsFunc(next, func(s TemplateStep) bool { return s.ID == o.ID }) {
			ds = append(ds, change.NewDelete(TemplateField(t, "steps", string(o.ID))))
		}
	}
	return ds
}

// TemplateDeleteDrafts は、テンプレート id の削除の行と、id を指していた種類ごとの既定を「なし」にする行(B3-12)。
func TemplateDeleteDrafts(s Settings, id change.ID) []change.Draft {
	return append([]change.Draft{change.NewDelete(TemplateField(id))}, DefaultClearDrafts(s, id, "")...)
}

// DefaultClearDrafts は、id を指す種類ごとの既定のうち、keep の種類以外を「なし」にする行。
// 種類を変えたときは keep に新しい種類を渡す。
func DefaultClearDrafts(s Settings, id change.ID, keep string) []change.Draft {
	var ds []change.Draft
	for _, k := range Kinds {
		if k != keep && s.TemplateDefaults[k] == id {
			ds = append(ds, change.NewSet("template_default/"+k, id, nil))
		}
	}
	return ds
}
