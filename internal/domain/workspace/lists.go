package workspace

import (
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// ListKind は、値の一覧の種類。ログの項目名の頭と同じ。
type ListKind string

const (
	ListStatuses   ListKind = "statuses"
	ListPriorities ListKind = "priorities"
	ListSystems    ListKind = "systems"
	ListChoices    ListKind = "choices" // 追加プロパティの選択肢
)

// List は、値の一覧の 1 つ。Property は選択肢のときだけ持つ。
type List struct {
	Kind     ListKind
	Property change.ID
}

// ParseList は、API のパスの {list} を一覧にする。statuses・priorities・systems 以外は、プロパティの ID。
func ParseList(s string) List {
	switch k := ListKind(s); k {
	case ListStatuses, ListPriorities, ListSystems:
		return List{Kind: k}
	}
	return List{Kind: ListChoices, Property: change.ID(s)}
}

// Ref は、値 v を指す項目名の頭(削除の行の f)。
func (l List) Ref(v change.ID) string {
	if l.Kind == ListChoices {
		return string(ListChoices) + "/" + string(l.Property) + "/" + string(v)
	}
	return string(l.Kind) + "/" + string(v)
}

// Field は、値 v の項目 name(name・color・category・rank)の項目名。
func (l List) Field(v change.ID, name string) string { return l.Ref(v) + "/" + name }

// Property は、id のプロパティ。
func (s Settings) Property(id change.ID) (Property, bool) {
	i := slices.IndexFunc(s.Properties, func(p Property) bool { return p.ID == id })
	if i < 0 {
		return Property{}, false
	}
	return s.Properties[i], true
}

// Values は、一覧 l の値。選択肢は、単一選択・複数選択のプロパティのときだけ。
func (s Settings) Values(l List) ([]Value, bool) {
	switch l.Kind {
	case ListStatuses:
		return s.Statuses, true
	case ListPriorities:
		return s.Priorities, true
	case ListSystems:
		return s.Systems, true
	}
	p, ok := s.Property(l.Property)
	if !ok || (p.Type != TypeSingle && p.Type != TypeMulti) {
		return nil, false
	}
	return p.Choices, true
}

// Label は、アクティビティの文の {一覧}(03 冒頭の表の下)。
func (s Settings) Label(l List) string {
	switch l.Kind {
	case ListStatuses:
		return "ステータス"
	case ListPriorities:
		return "優先度"
	case ListSystems:
		return "関連システム"
	}
	p, _ := s.Property(l.Property)
	return "「" + p.Name + "」の選択肢"
}

// Single は、1 つの値を持つ一覧か(ステータス・優先度・単一選択)。03「削除」の「単一の値」。
func (s Settings) Single(l List) bool {
	switch l.Kind {
	case ListStatuses, ListPriorities:
		return true
	case ListSystems:
		return false
	}
	p, _ := s.Property(l.Property)
	return p.Type == TypeSingle
}
