package history

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ProjectContext は、プロジェクトの履歴の文を作る材料。
type ProjectContext struct {
	Names item.Names
	Today time.Time
}

// projectFields は、履歴に出すプロジェクトの項目(項目名 → 画面の名前)。
var projectFields = map[string]string{
	"title": "タイトル", "status": "ステータス", "owner": "オーナー", "start": "開始日", "end": "終了日",
}

// ProjectRows は、プロジェクトの履歴を新しい順で返す(02「概要の編集」の履歴)。作成・説明の変更・項目の変更だけ。
// 移動と復元の行は出さない。
func ProjectRows(log []change.Record, c ProjectContext) []Row {
	var rows []Row
	for _, r := range log {
		if text, ok := projectText(r, c); ok {
			rows = append(rows, Row{Who: Who(r.Actor, c.Names), Text: text, At: r.At})
		}
	}
	return newestFirst(rows)
}

func projectText(r change.Record, c ProjectContext) (string, bool) {
	switch r.T {
	case change.Create:
		return "作成", true
	case change.Desc:
		return "説明を変更", true
	case change.Set:
		if name, ok := projectFields[r.F]; ok {
			return fmt.Sprintf("%sを %s に変更", name, projectValue(r.F, r.To, c)), true
		}
	}
	return "", false
}

// projectValue は、{値}。ステータスと題名は名前のまま、オーナーは人の名前、日付は 01 の表示。空は「なし」。
func projectValue(f string, to json.RawMessage, c ProjectContext) string {
	var s string
	_ = json.Unmarshal(to, &s)
	switch f {
	case "owner":
		if s == "" {
			return "なし"
		}
		return c.Names(s)
	case "start", "end":
		if s == "" {
			return "なし"
		}
		return item.Display(workspace.Property{Type: workspace.TypeDate}, to, workspace.Settings{}, c.Names, c.Today)
	}
	return s
}
