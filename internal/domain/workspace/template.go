package workspace

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

const (
	StepParent = "@parent" // Step の担当者「親と同じ」(初期値。B3-8)
	StepNone   = "@none"   // Step の担当者「未割り当て」
)

type TemplateStep struct {
	ID       change.ID
	Title    string
	Assignee string // StepParent・StepNone・メンバーのログイン名
	Rank     string
}

type TemplateChild struct {
	ID          change.ID
	Kind        string // Story・Task・Bug
	Title       string
	Description string
	Values      map[change.ID]json.RawMessage
	Steps       []TemplateStep // Rank の順
	Rank        string
}

type Template struct {
	ID          change.ID
	Name        string
	Kind        string // Epic・Story・Task・Bug・Step
	Title       string // タイトルの書き出し
	Description string
	Values      map[change.ID]json.RawMessage // プロパティの ID → 値(B3-5)。ない項目は「指定しない」
	Children    []TemplateChild               // Epic のときだけ。Rank の順
	Steps       []TemplateStep                // Story・Task・Bug のときだけ。Rank の順
}

// TemplateField は、テンプレート t の項目名 templates/<t>/<rest...>(B3-4)。rest がなければ templates/<t>。
func TemplateField(t change.ID, rest ...string) string {
	return strings.Join(append([]string{"templates", string(t)}, rest...), "/")
}

// Template は、id のテンプレート。
func (s Settings) Template(id change.ID) (Template, bool) {
	i := slices.IndexFunc(s.Templates, func(t Template) bool { return t.ID == id })
	if i < 0 {
		return Template{}, false
	}
	return s.Templates[i], true
}

// HoldsTemplateValue は、テンプレートに値を持てるプロパティか(B3-6)。組み込みは優先度・担当者・関連システムだけ。
func HoldsTemplateValue(p Property) bool {
	if !p.Builtin {
		return true
	}
	return p.Key == "priority" || p.Key == "assignee" || p.Key == "systems"
}
