package automation

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Event は、アイテムに起きたきっかけ。
type Event struct {
	Trigger   Trigger
	Status    change.ID // TriggerStatus: 変わったあとのステータス
	AllMerged bool      // TriggerPRMerged: リンク中の PR(マージされずに閉じたものを除く)が 1 件以上あり、すべてマージ済みか
}

// Outcome は、1 つのルールの実行の結果。Draft があれば変えた、Stopped があれば止まった。
type Outcome struct {
	Rule    Rule
	Prop    workspace.Property // ステータスか担当者
	To      json.RawMessage    // 変えたあと(止まったときは変えようとした)の値
	Draft   *change.Draft      // ルールが書く行(Rule 付き)
	Stopped string             // 止まった理由(03「制約で止まったとき」の文言)
	From    change.ID          // 止まったとき(Draft == nil)の、検査に使ったいまのステータス。変えた実行では空
}

// Run は、it に起きた ev で、有効なルールを一覧の順に実行する(03「実行の規則」)。it は利用者の操作のあとの値。
// 1 つのルールがステータスを変えたら、次のルールはそのあとの値から検査する。何もしないルール(同じ値、
// 担当者を使わない種類、なくなった値)は結果に入れない。専用ルールは it のプロジェクトのものだけ、
// it のプロジェクトが「動かさない」と指定したルールは対象にしない(B5a-11)。自動化の変更は、ほかのきっかけを起こさない(呼ぶ側で起こさない)。
func Run(s workspace.Settings, set projectrule.Set, rules []Rule, it item.Item, ev Event) []Outcome {
	return RunAll(s, set, rules, it, []Event{ev})
}

// RunAll は、1 回の変更で it に同時に起きた evs で、有効なルールを一覧の順に 1 回だけ通して実行する
// (03「実行の規則」の「一覧の上から順に」)。evs のどれかに合うルールを動かす。順と値の引き継ぎは Run と同じ。
func RunAll(s workspace.Settings, set projectrule.Set, rules []Rule, it item.Item, evs []Event) []Outcome {
	var out []Outcome
	pr := set.Of(it.Place.Project)
	for _, r := range rules {
		if !r.Enabled || (r.Scope != "" && r.Scope != it.Place.Project) || slices.Contains(pr.Disabled, r.ID) ||
			!slices.ContainsFunc(evs, func(ev Event) bool { return matches(r, ev) }) {
			continue
		}
		if o, ok := apply(s, pr, r, &it); ok {
			out = append(out, o)
		}
	}
	return out
}

func matches(r Rule, ev Event) bool {
	switch {
	case r.Trigger != ev.Trigger:
		return false
	case r.Trigger == TriggerStatus:
		return r.TriggerStatus == ev.Status
	case r.Trigger == TriggerPRMerged:
		return !r.AllMerged || ev.AllMerged
	}
	return true
}

// apply は、r のすることを it に当てる。変えたら it を変えたあとの値にする。
func apply(s workspace.Settings, pr projectrule.Rules, r Rule, it *item.Item) (Outcome, bool) {
	p, ok := s.Property(change.ID(r.Action))
	if !ok || (r.Action == ActionAssignee && !slices.Contains(p.Kinds, string(it.Kind))) {
		return Outcome{}, false
	}
	value := r.Value
	if value == Unassigned {
		value = ""
	}
	to, err := item.ParseInput(p, change.Value(value), s)
	from := item.Current(*it, p)
	if err != nil || bytes.Equal(from, to) {
		return Outcome{}, false
	}
	o := Outcome{Rule: r, Prop: p, To: to}
	if r.Action == ActionStatus {
		// 使わないステータスは、遷移の制約より先に見る(B5a-11)
		err := projectrule.CheckUse(s, pr, change.ID(value))
		if err == nil {
			err = projectrule.CheckTransition(s, pr, it.Status, change.ID(value))
		}
		if err != nil {
			o.Stopped, o.From = err.Error(), it.Status
			return o, true
		}
		it.Status = change.ID(value)
	} else {
		it.Assignee = value
	}
	d := change.NewSet(item.Field(p), from, to).ByRule(r.ID)
	o.Draft = &d
	return o, true
}
