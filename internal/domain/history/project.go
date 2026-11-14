package history

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ProjectContext は、プロジェクトの履歴の文を作る材料。WorkspaceName は、この Store のワークスペース(削除済みを含む)の
// 名前(Store にないものは空。nil なら引けないものとする)。
type ProjectContext struct {
	Names         item.Names
	Today         time.Time
	WorkspaceName func(workspace.ID) string
}

// projectFields は、履歴に出すプロジェクトの項目(項目名 → 画面の名前)。
var projectFields = map[string]string{
	"title": "タイトル", "status": "ステータス", "owner": "オーナー", "start": "開始日", "end": "終了日",
}

// ProjectRows は、プロジェクトの履歴を新しい順で返す(02「概要の編集」の履歴)。作成・説明の変更・項目の変更と、
// 別のワークスペースからの移動(A5b-21)だけ。復元の行は出さない。
func ProjectRows(log []change.Record, c ProjectContext) []Row {
	var rows []Row
	for _, r := range log {
		if text, ok := projectText(log, r, c); ok {
			rows = append(rows, Row{Who: Who(r.Actor, c.Names), Text: text, At: r.At})
		}
	}
	return newestFirst(rows)
}

func projectText(log []change.Record, r change.Record, c ProjectContext) (string, bool) {
	switch r.T {
	case change.Create:
		return "作成", true
	case change.Desc:
		return "説明を変更", true
	case change.MovedIn:
		if ms := transfer.MovedInOf([]change.Record{r}); len(ms) == 1 {
			return projectMovedInText(log, r.Op, ms[0]), true
		}
	case change.Set:
		if name, ok := projectFields[r.F]; ok {
			return fmt.Sprintf("%sを %s に変更", name, projectValue(r.F, r.To, c)), true
		}
		return projectMovedText(log, r, c)
	}
	return "", false
}

const projectMovedFormat = "%s から移動"

// projectMovedInText は、別の Store からの moved_in の行の文。ワークスペースと一緒に移った(FromWorkspace が、その操作 op を
// 終えたときのワークスペースと同じ)なら「{移動元の Store 名} から移動」(B5c-12)、それ以外は「{移動元のワークスペース名}
// から移動」(A5b-21)。あとで同じ Store の中で別のワークスペースへ移しても、この行は変わらない。
func projectMovedInText(log []change.Record, op change.ID, m transfer.MovedInInfo) string {
	end := 0
	for i, x := range log {
		if x.Op == op {
			end = i + 1
		}
	}
	if p, ok := project.FromLog(log[:end]); ok && p.Place.Workspace == m.FromWorkspace {
		return fmt.Sprintf(projectMovedFormat, orDeleted(m.FromStoreName))
	}
	return fmt.Sprintf(projectMovedFormat, orDeleted(m.FromWorkspaceName))
}

// projectMovedText は、同じ Store の中でワークスペースが変わる場所の set の行の文(A5b-21)。番号だけが変わるものと、
// 同じ操作に moved_in の行がある(別の Store から。そちらが文を持つ)ものは出さない。前にいた Store の中の移動(写した行)は、
// アイテムと同じく、その Store を出た moved_in の moves の名前で作る(A5a-2)。それ以外はこの Store の WorkspaceName で引く。
func projectMovedText(log []change.Record, r change.Record, c ProjectContext) (string, bool) {
	if !changesWorkspace(r) || slices.ContainsFunc(log, func(x change.Record) bool { return x.Op == r.Op && x.T == change.MovedIn }) {
		return "", false
	}
	for _, m := range transfer.MovedInOf(log) {
		if j := slices.IndexFunc(m.Moves, func(mv transfer.Move) bool { return mv.Op == r.Op }); j >= 0 {
			return fmt.Sprintf(projectMovedFormat, orDeleted(m.Moves[j].FromWorkspaceName)), true
		}
	}
	from, _, _ := places(r)
	name := ""
	if c.WorkspaceName != nil {
		name = c.WorkspaceName(from.Workspace)
	}
	return fmt.Sprintf(projectMovedFormat, orDeleted(name)), true
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
