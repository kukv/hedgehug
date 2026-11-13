package automation

import "fmt"

// 自動化の文言(03「実行の規則」「制約で止まったとき」「削除」、06「通知の種類」。B4-10)。

// ChangeText は「{プロパティ名}を {値} に変更」。
func ChangeText(prop, value string) string { return fmt.Sprintf("%sを %s に変更", prop, value) }

// ActivityText は、アクティビティと最近の実行の文。
func ActivityText(rule, change string) string { return fmt.Sprintf("「%s」で%s", rule, change) }

// AppliedText は、自動化が変えたときの通知の本文。
func AppliedText(rule, change string) string {
	return fmt.Sprintf("「%s」で%sしました", rule, change)
}

// StoppedText は、制約で止まったときの通知の本文。
func StoppedText(rule string) string {
	return fmt.Sprintf("「%s」が制約で止まりました", rule)
}

// IgnoreText は、「制約を無視して…」の保存のアクティビティの文。
func IgnoreText(rule string) string {
	return fmt.Sprintf("「%s」の変更を制約を無視して適用", rule)
}

// DisabledByDeleteText は、ステータスの削除で無効にしたルールの通知の本文。
func DisabledByDeleteText(rule string) string {
	return fmt.Sprintf("自動化「%s」は、ステータスの削除で、きっかけと同じステータスへの変更になったため、無効にしました", rule)
}

// RewrittenText は、ステータスの削除で書き換えたルールの通知の本文。part は「きっかけ」か「すること」。
func RewrittenText(rule, deleted, part, to string) string {
	return fmt.Sprintf("自動化「%s」は、ステータス「%s」の削除で、%sのステータスを「%s」に書き換えました", rule, deleted, part, to)
}
