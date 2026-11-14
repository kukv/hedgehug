package query

import (
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// GroupKeys は、テーブルのグループの設定と group: に書ける 10 項目(04「まとめ方」)。
var GroupKeys = []string{"status", "assignee", "project", "parent", "kind", "priority", "sprint", "system", "created", "updated"}

const (
	labelNone      = "なし"
	labelRemoved   = "(削除された値)"
	labelUnknownMe = "不明なユーザー"
)

// GroupKey は、使うまとめ方。テーブルの設定が 10 項目のどれかならそれ、なければ group: の値が
// 10 項目のどれかならそれ、どちらも使えなければ ""(E2a-6)。
func GroupKey(setting, fromQuery string) string {
	if slices.Contains(GroupKeys, setting) {
		return setting
	}
	if slices.Contains(GroupKeys, fromQuery) {
		return fromQuery
	}
	return ""
}

// Group は、見出しと、そこに入る行(facts の位置)。
type Group struct {
	Label string
	Rows  []int
}

// cell は、1 行が入るグループ 1 つ。id が同じなら同じグループで、label は最初に現れたものを使う。
type cell struct{ id, label string }

// Groups は、並べ替えたあとの facts を key でグループに分ける。グループは最初に現れた順に並べる
// (E2a-7)。関連システムの複数の値は、定義順に並べてそれぞれのグループに入る(E2a-8)。key が "" なら nil。
func Groups(facts []*Facts, key string, now time.Time) []Group {
	cellsOf := cellFunc(key, now)
	if cellsOf == nil {
		return nil
	}
	var groups []Group
	index := map[string]int{}
	for row, f := range facts {
		for _, c := range cellsOf(f) {
			i, ok := index[c.id]
			if !ok {
				i = len(groups)
				index[c.id] = i
				groups = append(groups, Group{Label: c.label})
			}
			groups[i].Rows = append(groups[i].Rows, row)
		}
	}
	return groups
}

func cellFunc(key string, now time.Time) func(*Facts) []cell {
	switch key {
	case "status":
		return func(f *Facts) []cell {
			return []cell{valueCell(f.Settings.Statuses, f.Item.Status)}
		}
	case "priority":
		return func(f *Facts) []cell {
			return []cell{valueCell(f.Settings.Priorities, f.Item.Priority)}
		}
	case "system":
		return systemCells
	case "assignee":
		return func(f *Facts) []cell { return []cell{assigneeCell(f)} }
	case "project":
		return func(f *Facts) []cell {
			return []cell{stored(f, string(f.Item.Place.Project), f.ProjectTitle, labelNone)}
		}
	case "parent":
		return parentCells
	case "kind":
		return func(f *Facts) []cell {
			if f.Item.Kind == "" {
				return []cell{{"", labelNone}}
			}
			return []cell{{string(f.Item.Kind), string(f.Item.Kind)}}
		}
	case "sprint":
		return func(f *Facts) []cell {
			return []cell{stored(f, string(f.Item.Sprint), f.SprintName, "スプリントなし")}
		}
	case "created":
		return func(f *Facts) []cell { return []cell{dayCell(f.Created, now)} }
	case "updated":
		return func(f *Facts) []cell { return []cell{dayCell(f.Updated, now)} }
	}
	return nil
}

// valueCell は、名前でまとめる値(大文字と小文字を区別しない)。ID がなければ「なし」、一覧になければ「(削除された値)」。
func valueCell(values []workspace.Value, id change.ID) cell {
	if id == "" {
		return cell{"", labelNone}
	}
	for _, v := range values {
		if v.ID == id {
			return cell{"n:" + strings.ToLower(v.Name), v.Name}
		}
	}
	return cell{"removed", labelRemoved}
}

func systemCells(f *Facts) []cell {
	var cells []cell
	for _, v := range f.Settings.Systems {
		if slices.Contains(f.Item.Systems, v.ID) {
			cells = append(cells, cell{"n:" + strings.ToLower(v.Name), v.Name})
		}
	}
	for _, id := range f.Item.Systems {
		if !slices.ContainsFunc(f.Settings.Systems, func(v workspace.Value) bool { return v.ID == id }) {
			cells = append(cells, cell{"removed", labelRemoved})
			break
		}
	}
	if len(cells) == 0 {
		return []cell{{"", "関連システムなし"}}
	}
	return cells
}

func assigneeCell(f *Facts) cell {
	if f.Item.Assignee == "" {
		return cell{"", "未割り当て"}
	}
	login := strings.ToLower(f.Item.Assignee)
	if name, ok := f.Names[login]; ok {
		return cell{login, name}
	}
	return cell{login, labelUnknownMe}
}

// stored は、Store と内部 ID の組でまとめる値(プロジェクト・スプリント)。
func stored(f *Facts, id, label, empty string) cell {
	if id == "" {
		return cell{"", empty}
	}
	return cell{f.StoreName + "\x00" + id, label}
}

func parentCells(f *Facts) []cell {
	if f.Item.Place.Parent == nil || f.Parent == nil {
		return []cell{{"", "親なし"}}
	}
	return []cell{{f.StoreName + "\x00" + string(*f.Item.Place.Parent), f.Parent.Display + " " + f.ParentTitle}}
}

// dayCell は、このマシンの時刻帯の日でまとめる(planning.Today と同じ式)。
func dayCell(t time.Time, now time.Time) cell {
	day := t.Local().Format(time.DateOnly)
	return cell{day, item.DateText(day, now)}
}
