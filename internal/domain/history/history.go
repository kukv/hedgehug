// Package history は、アイテムとプロジェクトの履歴(02「履歴」)の行を、ログから作る。
// 文は保存形式「アクティビティと履歴」の行の種類ごとに決め、知らない種類の行は出さない。
package history

import (
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Row は、履歴の 1 行。
type Row struct {
	Who  string
	Text string
	At   time.Time
}

// Context は、アイテムの履歴の文を作る材料。
type Context struct {
	Settings workspace.Settings // いまのワークスペースの設定。値の名前をここで引く(A4a-17)
	Names    item.Names
	Hidden   map[change.ID]bool // 出さない操作(HiddenOps)
	Today    time.Time          // 日付の年を決める
	Origin   Origin             // 別のワークスペースから移ってきたときの材料(A5a-17)
}

const unknownUser = "不明なユーザー"

// HiddenOps は、ワークスペースのログの delete の行の操作(op)。値やプロパティの削除と同じ保存で書いた
// アイテムの行は、履歴に出さない(A4a-16。B2-9)。
func HiddenOps(workspaceLog []change.Record) map[change.ID]bool {
	out := map[change.ID]bool{}
	for _, r := range workspaceLog {
		if r.T == change.Delete {
			out[r.Op] = true
		}
	}
	return out
}

// Who は、操作した人の表示(02「履歴」)。人はメンバーの表示名で、メンバーにいなければ「不明なユーザー」。
func Who(a change.Actor, names item.Names) string {
	switch a.Kind {
	case change.ActorAutomation:
		return "自動化"
	case change.ActorGitHub:
		return "GitHub"
	case change.ActorSync:
		return "同期"
	case change.ActorUser:
		return names(a.Login)
	}
	return unknownUser
}

// NamesOf は、メンバーの一覧から、ログイン名(大文字小文字を区別しない)で表示名を引く関数を作る。
func NamesOf(ms []member.Member) item.Names {
	byLogin := make(map[string]string, len(ms))
	for _, m := range ms {
		byLogin[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := byLogin[strings.ToLower(login)]; ok {
			return n
		}
		return unknownUser
	}
}

// newestFirst は、新しい順に並べる。時刻が同じ行(1 回の保存で書いた行)は、ログの後ろにあるものを先にする。
func newestFirst(rows []Row) []Row {
	slices.Reverse(rows)
	slices.SortStableFunc(rows, func(a, b Row) int { return b.At.Compare(a.At) })
	return rows
}
