package workspace

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// templateBuilder は、テンプレートの項目(B3-4)を集める。
type templateBuilder struct {
	tpls     map[change.ID]*Template
	children map[change.ID]map[change.ID]*TemplateChild // テンプレート → 子
	steps    map[change.ID]map[change.ID]*stepAcc       // テンプレート → Step
}

type stepAcc struct {
	TemplateStep
	parent change.ID // 空ならもとになるアイテム
}

func newTemplateBuilder() *templateBuilder {
	return &templateBuilder{
		tpls: map[change.ID]*Template{}, children: map[change.ID]map[change.ID]*TemplateChild{},
		steps: map[change.ID]map[change.ID]*stepAcc{},
	}
}

// set は、parts(templates/<T>/... を / で分けたもの)の値を入れる。
func (tb *templateBuilder) set(parts []string, raw json.RawMessage) {
	id := change.ID(parts[1])
	t := tb.template(id)
	switch {
	case len(parts) == 3:
		setTemplateField(t, parts[2], raw)
	case len(parts) == 4 && parts[2] == "values":
		putValue(&t.Values, parts[3], raw)
	case len(parts) == 5 && parts[2] == "children":
		setChildField(tb.child(id, parts[3]), parts[4], raw)
	case len(parts) == 6 && parts[2] == "children" && parts[4] == "values":
		putValue(&tb.child(id, parts[3]).Values, parts[5], raw)
	case len(parts) == 5 && parts[2] == "steps":
		setStepField(tb.step(id, parts[3]), parts[4], raw)
	}
}

func (tb *templateBuilder) template(id change.ID) *Template {
	t, ok := tb.tpls[id]
	if !ok {
		t = &Template{ID: id}
		tb.tpls[id] = t
	}
	return t
}

func (tb *templateBuilder) child(t change.ID, id string) *TemplateChild {
	if tb.children[t] == nil {
		tb.children[t] = map[change.ID]*TemplateChild{}
	}
	c, ok := tb.children[t][change.ID(id)]
	if !ok {
		c = &TemplateChild{ID: change.ID(id)}
		tb.children[t][change.ID(id)] = c
	}
	return c
}

func (tb *templateBuilder) step(t change.ID, id string) *stepAcc {
	if tb.steps[t] == nil {
		tb.steps[t] = map[change.ID]*stepAcc{}
	}
	s, ok := tb.steps[t][change.ID(id)]
	if !ok {
		s = &stepAcc{TemplateStep: TemplateStep{ID: change.ID(id)}}
		tb.steps[t][change.ID(id)] = s
	}
	return s
}

func setTemplateField(t *Template, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &t.Name)
	case "kind":
		_ = json.Unmarshal(raw, &t.Kind)
	case "title":
		_ = json.Unmarshal(raw, &t.Title)
	case "description":
		_ = json.Unmarshal(raw, &t.Description)
	}
}

func setChildField(c *TemplateChild, field string, raw json.RawMessage) {
	switch field {
	case "kind":
		_ = json.Unmarshal(raw, &c.Kind)
	case "title":
		_ = json.Unmarshal(raw, &c.Title)
	case "description":
		_ = json.Unmarshal(raw, &c.Description)
	case "rank":
		_ = json.Unmarshal(raw, &c.Rank)
	}
}

func setStepField(s *stepAcc, field string, raw json.RawMessage) {
	switch field {
	case "parent":
		_ = json.Unmarshal(raw, &s.parent)
	case "title":
		_ = json.Unmarshal(raw, &s.Title)
	case "assignee":
		_ = json.Unmarshal(raw, &s.Assignee)
	case "rank":
		_ = json.Unmarshal(raw, &s.Rank)
	}
}

// putValue は、値を入れる。null は「指定しない」なので、入っていた値を外す(B3-5)。
func putValue(m *map[change.ID]json.RawMessage, prop string, raw json.RawMessage) {
	if string(raw) == "null" {
		delete(*m, change.ID(prop))
		return
	}
	if *m == nil {
		*m = map[change.ID]json.RawMessage{}
	}
	(*m)[change.ID(prop)] = raw
}

// templates は、消したもの・名前のないもの・持ち主のないものを除いて並べた一覧。
func (tb *templateBuilder) templates(deleted map[string]bool) []Template {
	var out []Template // 1 つもなければ nil
	for id, t := range tb.tpls {
		if t.Name == "" || deleted[TemplateField(id)] {
			continue
		}
		tt := *t
		tt.Children = tb.liveChildren(id, deleted)
		tt.Steps = tb.liveSteps(id, "", deleted)
		for i := range tt.Children {
			tt.Children[i].Steps = tb.liveSteps(id, tt.Children[i].ID, deleted)
		}
		out = append(out, tt)
	}
	slices.SortFunc(out, func(a, b Template) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return out
}

func (tb *templateBuilder) liveChildren(t change.ID, deleted map[string]bool) []TemplateChild {
	m := maps.Clone(tb.children[t])
	maps.DeleteFunc(m, func(id change.ID, c *TemplateChild) bool {
		return c.Kind == "" || c.Title == "" || deleted[TemplateField(t, "children", string(id))]
	})
	if len(m) == 0 {
		return nil
	}
	return byRank(m, func(c *TemplateChild) (string, change.ID) { return c.Rank, c.ID })
}

func (tb *templateBuilder) liveSteps(t, parent change.ID, deleted map[string]bool) []TemplateStep {
	m := map[change.ID]*TemplateStep{}
	for id, s := range tb.steps[t] {
		if s.parent == parent && s.Title != "" && !deleted[TemplateField(t, "steps", string(id))] {
			m[id] = &s.TemplateStep
		}
	}
	if len(m) == 0 {
		return nil
	}
	return byRank(m, func(s *TemplateStep) (string, change.ID) { return s.Rank, s.ID })
}

// setTemplateDefault は、種類の既定のテンプレートを入れる。null は「なし」なので外す(B3-12)。
func (b *builder) setTemplateDefault(kind string, raw json.RawMessage) {
	if string(raw) == "null" {
		delete(b.acc.TemplateDefaults, kind)
		return
	}
	var id change.ID
	if json.Unmarshal(raw, &id) != nil {
		return
	}
	if b.acc.TemplateDefaults == nil {
		b.acc.TemplateDefaults = map[string]change.ID{}
	}
	b.acc.TemplateDefaults[kind] = id
}
