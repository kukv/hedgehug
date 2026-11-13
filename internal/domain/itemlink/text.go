package itemlink

import (
	"fmt"
	"strings"
)

// 07「追加」の断りの文言。
const (
	MsgNumber   = "番号を入力してください"
	MsgAlready  = "もうリンクしています"
	MsgHasIssue = "このアイテムには Issue がリンクされています"
)

// MsgWrongKind は、want の種別でリンクしようとした番号が、別の種別だったときの文言。
func MsgWrongKind(r Ref, want Kind) string {
	if want == PR {
		return r.Display() + " は PR ではなく Issue です"
	}
	return r.Display() + " は Issue ではなく PR です"
}

// MsgIssueTaken は、Issue が別のアイテム id(画面の ID)にリンクされているときの文言。
func MsgIssueTaken(r Ref, id string) string {
	return r.Display() + " は " + id + " にリンクされています"
}

// StateText は、状態の表示(07)。
func StateText(s State) string {
	switch s {
	case Open:
		return "Open"
	case Draft:
		return "Draft"
	case Merged:
		return "Merged"
	case Closed:
		return "Closed"
	}
	return string(s)
}

// ReviewText は、レビューの状態の表示(07)。
func ReviewText(r Review) string {
	switch r {
	case Pending:
		return "レビュー待ち"
	case Approved:
		return "承認済み"
	case ChangesRequested:
		return "修正依頼"
	}
	return "なし"
}

// AddSummary は、手でリンクしたときのアクティビティ(07「アクティビティに残る文」)。
func AddSummary(id string, r Ref, toProject bool) string {
	s := fmt.Sprintf("%s に %s をリンク", id, r.Display())
	if toProject {
		s += "(プロジェクトのリポジトリにも追加)"
	}
	return s
}

// AutoSummary は、GitHub が PR を自動でリンクしたときのアクティビティ(06 の 524 行)。
func AutoSummary(id string, r Ref) string {
	return fmt.Sprintf("%s に %s を自動でリンク", id, r.Display())
}

// RemoveSummary は、解除のアクティビティ。
func RemoveSummary(id string) string { return id + " のリンクを解除" }

// RefreshSummary は、取り直しの保存のアクティビティ(06 の文を、改名・状態・レビュー・タイトルの順に「、」で
// つなぐ。D3-10)。repo は GitHub の名前(変わっていなければ old.Repo)。
func RefreshSummary(id string, old Link, repo string, v Values) string {
	var parts []string
	now := Ref{Kind: old.Kind, Repo: repo, Number: old.Number}
	if repo != old.Repo {
		parts = append(parts, fmt.Sprintf("%s のリンクを %s から %s に更新", id, old.Display(), now.Display()))
	}
	if v.State != old.State {
		parts = append(parts, fmt.Sprintf("%s の %s の状態を %s に変更", id, now.Display(), StateText(v.State)))
	}
	if old.Kind == PR && v.Review != old.Review {
		parts = append(parts, fmt.Sprintf("%s の %s のレビューを %s に変更", id, now.Display(), ReviewText(v.Review)))
	}
	if v.Title != old.Title {
		title := strings.Join(strings.Fields(v.Title), " ")
		parts = append(parts, fmt.Sprintf("%s の %s のタイトルを「%s」に変更", id, now.Display(), title))
	}
	return strings.Join(parts, "、")
}
