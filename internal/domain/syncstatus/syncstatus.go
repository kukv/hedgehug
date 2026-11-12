// Package syncstatus は、同期の結果と状態の文言(06「同期のエラー」「同期の結果」)を持つ。
// 文言は仕様の文字どおりに書く。
package syncstatus

import "fmt"

// Reason は、同期できなかった理由。値は state.db と API に書く。
type Reason string

const (
	NoAccount       Reason = "no_account"
	AuthFailed      Reason = "auth_failed"
	Unreachable     Reason = "unreachable"
	WriteDenied     Reason = "write_denied"
	Unreadable      Reason = "unreadable"
	NotStore        Reason = "not_store"
	NewerFormat     Reason = "newer_format"
	Unrelated       Reason = "unrelated"
	Rewritten       Reason = "rewritten"
	SendConflict    Reason = "send_conflict"
	MirrorUnrelated Reason = "mirror_unrelated"
	MirrorAhead     Reason = "mirror_ahead"
)

// shortText は、06「同期のエラー」の表の「短い理由」と、ミラーの段落の短い理由。
var shortText = map[Reason]string{
	NoAccount:       "アカウントが設定されていません",
	AuthFailed:      "認証に失敗しました",
	Unreachable:     "remote に接続できません",
	WriteDenied:     "remote への書き込みが許可されていません",
	Unreadable:      "remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。",
	NotStore:        "remote が hedgehug の Store ではありません",
	NewerFormat:     "hedgehug を更新してください",
	Unrelated:       "remote の履歴がこの Store と関係がありません",
	Rewritten:       "remote の履歴が書き換えられています",
	SendConflict:    "送信が他の人の更新と重なりました。もう一度同期してください",
	MirrorUnrelated: "ミラーの履歴がこの Store と関係がありません",
	MirrorAhead:     "ミラーの履歴が先に進んでいます",
}

// Short は短い理由。「すべて同期」のトースト、「Store ごとの同期状態」、設定の「Store」で同じ文を使う。
// 知らない理由は空。
func (r Reason) Short() string { return shortText[r] }

// Valid は、知っている理由か。
func (r Reason) Valid() bool { _, ok := shortText[r]; return ok }

// Failed は、手動で 1 つの Store を同期して失敗したときのトースト(エラー)。
func Failed(store string, r Reason) string {
	if r == NoAccount {
		return store + " にアカウントが設定されていないため同期できません"
	}
	return store + " の同期に失敗しました: " + r.Short()
}

// NoRemote は、remote がない Store を手動で同期したときのトースト。Store の状態は変えない。
func NoRemote(store string) string {
	return store + " には remote がないため、同期するものはありません"
}

// Synced は、1 つの Store の同期に成功したときのトースト。
func Synced(store string, sent, received int) string {
	return fmt.Sprintf("%s を同期しました(送信 %d 件・受信 %d 件)", store, sent, received)
}

// MirrorsFailed は、手動で 1 つの Store を同期して、ミラーに送れなかったときの警告のトースト。
func MirrorsFailed(k int) string {
	return fmt.Sprintf("ミラー %d 件に送れませんでした", k)
}

// NothingToSync は、同期する Store がないときのトースト(06「手動で同期する」)。
const NothingToSync = "同期する Store がありません(remote のある、表示中の Store が要ります)"

// Failure は、「すべて同期」で失敗した 1 つの Store。
type Failure struct {
	Store  string
	Reason Reason
}

// All は、「すべて同期」のトーストの行。1 行目が要約で、失敗があれば、続けて失敗した Store ごとに
// 「{Store 名}: {短い理由}」を 1 行ずつ並べる。
func All(ok int, failures []Failure) []string {
	if len(failures) == 0 {
		return []string{fmt.Sprintf("%d 件の Store を同期しました", ok)}
	}
	lines := []string{fmt.Sprintf("成功 %d 件・失敗 %d 件", ok, len(failures))}
	for _, f := range failures {
		lines = append(lines, f.Store+": "+f.Reason.Short())
	}
	return lines
}
