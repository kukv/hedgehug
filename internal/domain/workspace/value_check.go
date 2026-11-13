package workspace

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

var categoryNames = map[Category]string{
	CategoryTodo: "未着手", CategoryDoing: "進行中", CategoryDone: "完了", CategoryCanceled: "中止",
}

// CategoryName は、分類の画面の名前。
func CategoryName(c Category) string { return categoryNames[c] }

// CheckValueName は、値の名前の検査(03「名前」)。同じ一覧の中で、大文字と小文字を区別せずに比べる。
// id が空なら追加。name は NormalizeName を通した形。
func CheckValueName(vs []Value, id change.ID, name string) error {
	switch {
	case name == "":
		return refusal.New("名前は空にできません")
	case slices.ContainsFunc(vs, func(v Value) bool { return v.ID != id && strings.EqualFold(v.Name, name) }):
		return refusal.New("同じ名前の値があります")
	}
	return nil
}

// DefaultCategory は、id を既定のステータスにしている分類。
func (s Settings) DefaultCategory(id change.ID) (Category, bool) {
	for _, c := range Categories {
		if s.CategoryDefaults[c] == id {
			return c, true
		}
	}
	return "", false
}

// StatusName は、ステータス id の名前。なければ空。
func StatusName(s Settings, id change.ID) string {
	for _, v := range s.Statuses {
		if v.ID == id {
			return v.Name
		}
	}
	return ""
}

func bothText(name string, c Category) error {
	return refusal.New(fmt.Sprintf("「%s」は初期ステータスで、%sの既定のステータスでもあります。先に別のステータスを、初期ステータスと%sの既定にしてください",
		name, CategoryName(c), CategoryName(c)))
}

func defaultText(name string, c Category) error {
	return refusal.New(fmt.Sprintf("「%s」は%sの既定のステータスです。先に別のステータスを既定にしてください", name, CategoryName(c)))
}

// CheckStatusCategory は、ステータスの分類の変更の検査(03「分類ごとの既定のステータス」)。
func CheckStatusCategory(s Settings, id change.ID, to Category) error {
	i := slices.IndexFunc(s.Statuses, func(v Value) bool { return v.ID == id })
	if i < 0 || s.Statuses[i].Category == to {
		return nil
	}
	name := s.Statuses[i].Name
	def, isDefault := s.DefaultCategory(id)
	initial := id == s.InitialStatus && (to == CategoryDone || to == CategoryCanceled)
	switch {
	case isDefault && initial:
		return bothText(name, def)
	case isDefault:
		return defaultText(name, def)
	case initial:
		return refusal.New(fmt.Sprintf("「%s」は初期ステータスです。完了・中止にするには、先に別のステータスを初期ステータスにしてください", name))
	}
	return nil
}

// CheckDeleteStatus は、ステータスの削除の検査(03「初期ステータス」「分類ごとの既定のステータス」)。
func CheckDeleteStatus(s Settings, id change.ID) error {
	def, isDefault := s.DefaultCategory(id)
	initial := id == s.InitialStatus
	switch {
	case isDefault && initial:
		return bothText(StatusName(s, id), def)
	case initial:
		return refusal.New("初期ステータスは削除できません。先に別のステータスを初期ステータスにしてください")
	case isDefault:
		return defaultText(StatusName(s, id), def)
	}
	return nil
}

// CheckDeletePriority は、優先度の削除の検査(03「初期優先度」)。
func CheckDeletePriority(s Settings, id change.ID) error {
	if id == s.InitialPriority {
		return refusal.New("初期優先度は削除できません。先に別の優先度を初期優先度にしてください")
	}
	return nil
}

// CheckInitialStatus は、初期ステータスにできるか(03「初期ステータス」。未着手か進行中だけ)。
func CheckInitialStatus(s Settings, id change.ID) error {
	for _, v := range s.Statuses {
		if v.ID == id && (v.Category == CategoryDone || v.Category == CategoryCanceled) {
			return refusal.New("完了・中止のステータスは初期ステータスにできません")
		}
	}
	return nil
}
