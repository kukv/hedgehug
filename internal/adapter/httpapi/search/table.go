package search

import (
	"encoding/json"
	"time"

	appsearch "github.com/kukv/hedgehug/internal/app/search"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/query"
)

// SearchSort は、列見出しの並べ替え。key は固定の列の鍵か、追加プロパティの小文字の鍵(p: を付けない)。
type SearchSort struct {
	_    struct{} `nullable:"true"`
	Key  string   `json:"key"`
	Desc bool     `json:"desc"`
}

// SearchColumn は、テーブルの列 1 つ。type は追加プロパティの列だけが持つ(固定の列は空)。
type SearchColumn struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Type     string `json:"type" doc:"追加プロパティの型。固定の列は空"`
	Sortable bool   `json:"sortable"`
}

// SearchGroup は、グループ 1 つ。rows は応答の rows の位置。
type SearchGroup struct {
	Label string `json:"label"`
	Rows  []int  `json:"rows" nullable:"false"`
}

// SearchProject は、行のプロジェクト。
type SearchProject struct {
	Display string `json:"display"`
	Title   string `json:"title"`
}

// SearchParent は、行の親。
type SearchParent struct {
	_       struct{} `nullable:"true"`
	ItemID  string   `json:"itemId"`
	Display string   `json:"display"`
	Title   string   `json:"title"`
}

// SearchCell は、追加プロパティの列のセル。type は行のワークスペースでのプロパティの型。
type SearchCell struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

// SearchValues は、セルの編集に要る、内部 ID のままの値(表示の名前でなく)。
type SearchValues struct {
	Priority string                     `json:"priority" doc:"優先度の内部 ID。空は値なし"`
	Systems  []string                   `json:"systems" nullable:"false" doc:"関連システムの内部 ID"`
	Choices  map[string][]string        `json:"choices" nullable:"false" doc:"選択式の追加プロパティの値(プロパティの内部 ID → 値の内部 ID)"`
	Fields   map[string]json.RawMessage `json:"fields" nullable:"false" doc:"追加プロパティの値(プロパティの内部 ID → 値)。型は設定で読む"`
	Sprint   string                     `json:"sprint" doc:"スプリントの内部 ID。空はバックログ"`
	Release  string                     `json:"release" doc:"リリースの内部 ID。空は「なし」"`
}

func toSort(s *SearchSort) *query.Sort {
	if s == nil {
		return nil
	}
	return &query.Sort{Key: s.Key, Desc: s.Desc}
}

func toColumns(cols []appsearch.Column) []SearchColumn {
	out := make([]SearchColumn, 0, len(cols))
	for _, c := range cols {
		out = append(out, SearchColumn{Key: c.Key, Name: c.Name, Type: string(c.Type), Sortable: c.Sortable})
	}
	return out
}

func toGroups(gs []query.Group) []SearchGroup {
	out := make([]SearchGroup, 0, len(gs))
	for _, g := range gs {
		out = append(out, SearchGroup{Label: g.Label, Rows: append([]int{}, g.Rows...)})
	}
	return out
}

// fillTable は、行 r の、テーブルの値(E2a-9)を row に入れる。
func fillTable(row *SearchRow, r appsearch.Row) {
	f := r.Facts
	it := f.Item
	row.Start, row.Estimate = it.Start, it.Estimate
	row.Systems = append([]string{}, r.Cells.Systems...)
	row.Sprint, row.Release = f.SprintName, f.ReleaseName
	row.Project = SearchProject{Display: f.Project.Display, Title: f.ProjectTitle}
	if f.Parent != nil && it.Place.Parent != nil {
		row.Parent = &SearchParent{ItemID: string(*it.Place.Parent), Display: f.Parent.Display, Title: f.ParentTitle}
	}
	row.Created, row.Updated = rfc3339(f.Created), rfc3339(f.Updated)
	row.Unused = append([]string{}, r.Cells.Unused...)
	row.Removed = append([]string{}, r.Cells.Removed...)
	fillValues(row, it)
	row.Custom = make(map[string]SearchCell, len(r.Cells.Custom))
	for k, c := range r.Cells.Custom {
		row.Custom[k] = SearchCell{Text: c.Text, Type: string(c.Type)}
	}
}

// fillValues は、アイテムの内部 ID のままの値を row.Values に入れる。
func fillValues(row *SearchRow, it item.Item) {
	v := SearchValues{
		Priority: string(it.Priority), Sprint: string(it.Sprint), Release: string(it.Release),
		Systems: []string{}, Choices: map[string][]string{}, Fields: map[string]json.RawMessage{},
	}
	for _, s := range it.Systems {
		v.Systems = append(v.Systems, string(s))
	}
	for k, vs := range it.Choices {
		ids := make([]string, 0, len(vs))
		for _, x := range vs {
			ids = append(ids, string(x))
		}
		v.Choices[string(k)] = ids
	}
	for k, x := range it.Fields {
		v.Fields[string(k)] = x
	}
	row.Values = v
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
