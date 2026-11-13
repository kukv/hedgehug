package workspace

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// childKinds は、Epic の子に置ける種類(01「用語と階層」)。
var childKinds = []string{"Story", "Task", "Bug"}

// hasSteps は、もとになるアイテムが Step を持てる種類か。
func hasSteps(kind string) bool { return slices.Contains(childKinds, kind) }

// NormalizeTemplate は、保存する形にする(B3-6・B3-11): 名前の前後の空白を除き(タイトルの書き出しはそのまま)、
// タイトルが空の子と Step を捨て、種類で持てない構成と、持たない・種類で使わないプロパティの値を捨てる。
// Step の担当者が空なら「親と同じ」。
func NormalizeTemplate(s Settings, t Template) Template {
	t.Name = NormalizeName(t.Name)
	t.Values = keepValues(s, t.Kind, t.Values)
	steps, cs := t.Steps, t.Children
	t.Steps, t.Children = nil, nil
	if hasSteps(t.Kind) {
		t.Steps = normalizeSteps(steps)
	}
	if t.Kind == "Epic" {
		t.Children = normalizeChildren(s, cs)
	}
	return t
}

func normalizeChildren(s Settings, cs []TemplateChild) []TemplateChild {
	var out []TemplateChild
	for _, c := range cs {
		c.Title = strings.TrimSpace(c.Title)
		if c.Title == "" {
			continue
		}
		c.Values = keepValues(s, c.Kind, c.Values)
		c.Steps = normalizeSteps(c.Steps)
		out = append(out, c)
	}
	return out
}

func normalizeSteps(ss []TemplateStep) []TemplateStep {
	var out []TemplateStep
	for _, st := range ss {
		st.Title = strings.TrimSpace(st.Title)
		if st.Title == "" {
			continue
		}
		if st.Assignee == "" {
			st.Assignee = StepParent
		}
		out = append(out, st)
	}
	return out
}

// keepValues は、kind の種類で使い、テンプレートに持てるプロパティの値だけを残す。空の値は値なし。
func keepValues(s Settings, kind string, vs map[change.ID]json.RawMessage) map[change.ID]json.RawMessage {
	var out map[change.ID]json.RawMessage
	for id, v := range vs {
		p, ok := s.Property(id)
		if !ok || !HoldsTemplateValue(p) || !slices.Contains(p.Kinds, kind) || emptyValue(v) {
			continue
		}
		if out == nil {
			out = map[change.ID]json.RawMessage{}
		}
		out[id] = v
	}
	return out
}

// TemplateValuesFor は、テンプレートの値のうち、kind の種類で使い、表示中で、値の参照が正しいものだけ
// (A3b-9。03「表示中でなくなったプロパティや、その種類で使わないプロパティは適用しない」)。
func TemplateValuesFor(s Settings, kind string, vs map[change.ID]json.RawMessage) map[change.ID]json.RawMessage {
	out := map[change.ID]json.RawMessage{}
	for id, v := range keepValues(s, kind, vs) {
		if p, _ := s.Property(id); !p.Hidden && validValue(s, p, v) {
			out[id] = v
		}
	}
	return out
}

func emptyValue(v json.RawMessage) bool {
	s := strings.TrimSpace(string(v))
	return s == "" || s == "null" || s == "[]" || s == `""`
}

// CheckTemplate は、保存の検査(03「一覧と編集」)。t は NormalizeTemplate のあと。
func CheckTemplate(s Settings, t Template) error {
	if t.Name == "" {
		return refusal.New("名前を入力してください")
	}
	for _, o := range s.Templates {
		if o.ID != t.ID && o.Name == t.Name {
			return refusal.New("同じ名前のテンプレートがあります")
		}
	}
	return nil
}

// DroppedByKind は、t を kind の種類で保存したときに捨てる子と Step の数(B3-14)。
func DroppedByKind(t Template, kind string) (children, steps int) {
	if kind != "Epic" {
		children = len(t.Children)
		for _, c := range t.Children {
			steps += len(c.Steps)
		}
	}
	if !hasSteps(kind) {
		steps += len(t.Steps)
	}
	return children, steps
}

// ValidTemplate は、種類・子の種類・値の参照が正しいか。画面からは起きないので、false は ErrNotFound にする(B2-12)。
func ValidTemplate(s Settings, t Template) bool {
	if !slices.Contains(Kinds, t.Kind) || !validValues(s, t.Values) {
		return false
	}
	for _, c := range t.Children {
		if !slices.Contains(childKinds, c.Kind) || !validValues(s, c.Values) {
			return false
		}
	}
	return true
}

func validValues(s Settings, vs map[change.ID]json.RawMessage) bool {
	for id, v := range vs {
		p, ok := s.Property(id)
		if !ok || !HoldsTemplateValue(p) || !validValue(s, p, v) {
			return false
		}
	}
	return true
}

// validValue は、値の ID がその一覧にあるか。ほかの型は形を問わない(画面が型どおりに送る)。
func validValue(s Settings, p Property, v json.RawMessage) bool {
	switch {
	case p.Key == "priority" && p.Builtin:
		return validRef(s.Priorities, v)
	case p.Key == "systems" && p.Builtin:
		return validRefs(s.Systems, v)
	case p.Type == TypeSingle:
		return validRef(p.Choices, v)
	case p.Type == TypeMulti:
		return validRefs(p.Choices, v)
	}
	return true
}

func validRef(vs []Value, v json.RawMessage) bool {
	var id change.ID
	return json.Unmarshal(v, &id) == nil && hasValue(vs, id)
}

func validRefs(vs []Value, v json.RawMessage) bool {
	var ids []change.ID
	if json.Unmarshal(v, &ids) != nil {
		return false
	}
	for _, id := range ids {
		if !hasValue(vs, id) {
			return false
		}
	}
	return true
}

func hasValue(vs []Value, id change.ID) bool {
	return slices.ContainsFunc(vs, func(v Value) bool { return v.ID == id })
}
