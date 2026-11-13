package automation

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
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
}

// Run は、it に起きた ev で、有効なルールを一覧の順に実行する(03「実行の規則」)。it は利用者の操作のあとの値。
// 1 つのルールがステータスを変えたら、次のルールはそのあとの値から検査する。何もしないルール(同じ値、
// 担当者を使わない種類、なくなった値)は結果に入れない。自動化の変更は、ほかのきっかけを起こさない(呼ぶ側で起こさない)。
func Run(s workspace.Settings, rules []Rule, it item.Item, ev Event) []Outcome {
	var out []Outcome
	for _, r := range rules {
		if !r.Enabled || !matches(r, ev) {
			continue
		}
		if o, ok := apply(s, r, &it); ok {
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
func apply(s workspace.Settings, r Rule, it *item.Item) (Outcome, bool) {
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
		if err := workspace.CheckTransition(s, it.Status, change.ID(value)); err != nil {
			o.Stopped = err.Error()
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
