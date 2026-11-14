package search

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Column は、テーブルの列 1 つ。Type は追加プロパティの列だけが持つ(固定の列は "")。Builtin は、組み込みの
// プロパティに当たる列のそのプロパティの鍵(status・systems など)で、ほかは ""。
type Column struct {
	Key      string
	Name     string
	Type     workspace.PropertyType
	Sortable bool
	Builtin  string
}

// fixedColumn は、固定の列。key は並べ替えのキーと同じ綴り、builtin は当たる組み込みのプロパティの鍵(なければ "")。
type fixedColumn struct{ key, name, builtin string }

// fixedColumns は、固定の 17 列の並び(E2a-1)。
var fixedColumns = []fixedColumn{
	{"id", "ID", ""},
	{"kind", "種類", ""},
	{"title", "タイトル", ""},
	{"status", "ステータス", "status"},
	{"assignee", "担当者", "assignee"},
	{"priority", "優先度", "priority"},
	{"start", "開始日", "start"},
	{"end", "終了日", "end"},
	{"points", "ポイント", "points"},
	{"estimate", "見積もり時間", "estimate"},
	{"system", "関連システム", "systems"},
	{"sprint", "スプリント", ""},
	{"release", "リリース", ""},
	{"project", "プロジェクト", ""},
	{"parent", "親", ""},
	{"created", "作成日", ""},
	{"updated", "更新日", ""},
}

// columnsOf は、対象のワークスペース spaces(Store の並び → ワークスペースの並びの順。設定のあるものだけ)の列。
// 固定の 17 列のあとに、追加プロパティの列を、最初に現れた順に並べる(E2a-1〜E2a-4)。
func columnsOf(spaces []*space) []Column {
	cols := make([]Column, 0, len(fixedColumns))
	for _, f := range fixedColumns {
		c := Column{Key: f.key, Name: f.name, Sortable: true, Builtin: f.builtin}
		if f.builtin != "" {
			if name, ok := agreed(namesOf(spaces, f.builtin)); ok {
				c.Name = name
			}
		}
		cols = append(cols, c)
	}
	return append(cols, customColumns(spaces)...)
}

// customColumns は、追加プロパティの列。名前がすべて同じならその名前、違えば最初に現れたキーの綴り。型は最初のもの。
func customColumns(spaces []*space) []Column {
	var cols []Column
	var names [][]string
	index := map[string]int{}
	for _, sp := range spaces {
		for _, p := range sp.settings.Properties {
			if p.Builtin {
				continue
			}
			lower := strings.ToLower(p.Key)
			i, ok := index[lower]
			if !ok {
				i = len(cols)
				index[lower] = i
				cols = append(cols, Column{Key: "p:" + lower, Name: p.Key, Type: p.Type, Sortable: sortableCustom(lower)})
				names = append(names, nil)
			}
			names[i] = append(names[i], p.Name)
		}
	}
	for i := range cols {
		if name, ok := agreed(names[i]); ok {
			cols[i].Name = name
		}
	}
	return cols
}

// sortableCustom は、追加プロパティのキー lower(小文字)の列を並べ替えられるか。組み込みのキーや固定の列の鍵と
// 同じ綴りなら、並べ替えのキーが組み込みの値を指すので並べ替えられない。
func sortableCustom(lower string) bool {
	return !query.Builtin(lower) && !slices.ContainsFunc(fixedColumns, func(f fixedColumn) bool { return f.key == lower })
}

// namesOf は、spaces の組み込みのプロパティ key の名前。
func namesOf(spaces []*space, key string) []string {
	var out []string
	for _, sp := range spaces {
		for _, p := range sp.settings.Properties {
			if p.Builtin && p.Key == key {
				out = append(out, p.Name)
			}
		}
	}
	return out
}

// agreed は、names が 1 つ以上あり、すべて同じならその名前。
func agreed(names []string) (string, bool) {
	if len(names) == 0 {
		return "", false
	}
	for _, n := range names[1:] {
		if n != names[0] {
			return "", false
		}
	}
	return names[0], true
}
