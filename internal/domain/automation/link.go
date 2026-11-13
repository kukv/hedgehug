package automation

import "github.com/kukv/hedgehug/internal/domain/itemlink"

// LinkEvents は、リンクの値が before から after になったときに起きる、PR・Issue のきっかけ(07 の 292〜299 行)。
// before が nil なら、いまリンクした(手でのリンク・自動リンク)。順は Triggers の順。
func LinkEvents(before *itemlink.Link, after itemlink.Link) []Trigger {
	var out []Trigger
	// 「…に変わったとき」は、before の状態がそれでなかったとき。nil はいまリンクしたので、変わったことにする。
	changedTo := func(s itemlink.State) bool {
		return after.State == s && (before == nil || before.State != s)
	}
	if after.Kind == itemlink.PR {
		if changedTo(itemlink.Open) {
			out = append(out, TriggerPROpen)
		}
		// レビュー待ちは before の状態を問わない(Draft のまま依頼されたときも起きる)。
		openOrDraft := after.State == itemlink.Open || after.State == itemlink.Draft
		if openOrDraft && after.Review == itemlink.Pending && (before == nil || before.Review == itemlink.NoReview) {
			out = append(out, TriggerReview)
		}
		if changedTo(itemlink.Merged) {
			out = append(out, TriggerPRMerged)
		}
	}
	if after.Kind == itemlink.Issue && changedTo(itemlink.Closed) {
		out = append(out, TriggerIssueClosed)
	}
	return out
}

// AllMerged は、マージされずに閉じた PR を除くリンク中の PR が 1 件以上あり、すべて Merged か(D4a-13)。Issue は数えない。
func AllMerged(links []itemlink.Link) bool {
	n := 0
	for _, l := range links {
		if l.Kind != itemlink.PR || l.State == itemlink.Closed {
			continue
		}
		if l.State != itemlink.Merged {
			return false
		}
		n++
	}
	return n > 0
}
