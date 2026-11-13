package transfer_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// settings は、既定の設定で、値の内部 ID を <p>V01〜<p>V16 にしたもの。ステータスは V01〜V05
// (Todo・InProgress・Done・Pending・Cancel)、優先度は V06〜V10(Highest・High・Medium・Low・Lowest)、
// 関連システムは V11〜V16(Web・API・iOS・Android・DB・CI)。
func settings(p string) workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("%sV%02d", p, n)) })
}

func choices(p string, names ...string) []workspace.Value {
	var vs []workspace.Value
	for i, n := range names {
		vs = append(vs, workspace.Value{ID: change.ID(fmt.Sprintf("%s%d", p, i+1)), Name: n})
	}
	return vs
}

// 移動元 F と移動先 T。T は、Pending を Hold に変え、優先度 Lowest と関連システム iOS をなくし、追加プロパティを一部だけ持つ。
func pair() (from, to workspace.Settings) {
	from, to = settings("F"), settings("T")
	from.Properties = append(from.Properties,
		workspace.Property{ID: "FP1", Key: "memo", Name: "メモ", Type: workspace.TypeText},
		workspace.Property{ID: "FP2", Key: "effort", Name: "工数", Type: workspace.TypeNumber},
		workspace.Property{ID: "FP3", Key: "env", Name: "環境", Type: workspace.TypeSingle, Choices: choices("FE", "本番", "検証")},
		workspace.Property{ID: "FP4", Key: "targets", Name: "対象", Type: workspace.TypeMulti, Choices: choices("FT", "A", "B", "C")},
		workspace.Property{ID: "FP5", Key: "reviewer", Name: "レビュアー", Type: workspace.TypeUser},
		workspace.Property{ID: "FP6", Key: "needs", Name: "要確認", Type: workspace.TypeCheck},
	)
	to.Statuses[3].Name = "Hold"
	to.Priorities = to.Priorities[:4]
	to.Systems = slices.Delete(slices.Clone(to.Systems), 2, 3)
	to.Properties = append(to.Properties,
		workspace.Property{ID: "TP1", Key: "memo", Type: workspace.TypeText},
		workspace.Property{ID: "TP2", Key: "effort", Type: workspace.TypeText}, // 型が違う
		workspace.Property{ID: "TP3", Key: "env", Type: workspace.TypeSingle, Choices: choices("TE", "本番")},
		workspace.Property{ID: "TP4", Key: "targets", Type: workspace.TypeMulti, Choices: choices("TT", "A", "B")},
		workspace.Property{ID: "TP5", Key: "reviewer", Type: workspace.TypeUser},
	)
	return from, to
}

func show(r transfer.RemapResult) []string {
	var out []string
	for _, d := range r.Drafts {
		out = append(out, fmt.Sprintf("%s %s %s>%s", d.T, d.F, string(d.From), string(d.To)))
	}
	return out
}

func fullItem() item.Item {
	pts := 3.0
	return item.Item{
		ID: "I1", Status: "FV04", Priority: "FV10", Systems: []change.ID{"FV11", "FV13"}, Points: &pts,
		Assignee: "Taro", Watchers: []string{"Hanako"},
		Choices: map[change.ID][]change.ID{"FP3": {"FE2"}, "FP4": {"FT1", "FT3"}},
		Fields: map[change.ID]json.RawMessage{
			"FP1": json.RawMessage(`"メモ本文"`), "FP2": json.RawMessage(`3`), "FP3": json.RawMessage(`"FE2"`),
			"FP5": json.RawMessage(`"Jiro"`), "FP6": json.RawMessage(`true`),
		},
	}
}

// 同じ Store(people が nil): 担当者・ユーザー型・ウォッチはそのまま。
func TestRemapSameStore(t *testing.T) {
	from, to := pair()
	got := transfer.Remap(fullItem(), from, to, "P1", nil, nil)
	want := []string{
		`set status "FV04">"TV01"`,                                                // Pending は移動先になく、同じ分類(未着手)の既定 Todo
		`set priority "FV10">"TV08"`,                                              // Lowest は移動先になく、初期優先度 Medium
		`remove systems "FV11">`, `remove systems "FV13">`, `add systems >"TV11"`, // Web は一致、iOS はなし
		`set fields/TP1 null>"メモ本文"`, `set fields/FP1 "メモ本文">null`,
		`set fields/FP2 3>null`,                                                         // 型が違うので捨てる
		`set fields/FP3 "FE2">null`,                                                     // 検証は移動先にない
		`remove fields/FP4 "FT1">`, `remove fields/FP4 "FT3">`, `add fields/TP4 >"TT1"`, // A は一致、C はなし
		`set fields/TP5 null>"Jiro"`, `set fields/FP5 "Jiro">null`,
		`set fields/FP6 true>null`, // 移動先にない
	}
	if g := show(got); !slices.Equal(g, want) {
		t.Errorf("rows =\n%q\nwant\n%q", g, want)
	}
	// 値が入っていたのに空になった項目。ステータスと、一部だけ残った複数選択は並べない。優先度は初期優先度にしたので並べる。
	if wantEmptied := []string{"優先度", "工数", "環境", "要確認"}; !slices.Equal(got.Emptied, wantEmptied) {
		t.Errorf("Emptied = %v, want %v", got.Emptied, wantEmptied)
	}
}

// 別の Store(people が非 nil): 担当者・ユーザー型・ウォッチを移動先のログイン名に置き換える。
func TestRemapOtherStoreReplacesPeople(t *testing.T) {
	from, to := pair()
	people := transfer.PeopleMap{"taro": "taro", "hanako": "hanako2", "jiro": "jiro"}
	got := show(transfer.Remap(fullItem(), from, to, "P1", people, nil))
	for _, w := range []string{
		`set assignee "Taro">"taro"`, `set fields/TP5 null>"jiro"`, `remove watchers "Hanako">`, `add watchers >"hanako2"`,
	} {
		if !slices.Contains(got, w) {
			t.Errorf("rows lack %s: %q", w, got)
		}
	}
	// 担当者はステータスと優先度のあいだ、ウォッチは最後。
	i, j, k := slices.Index(got, `set status "FV04">"TV01"`), slices.Index(got, `set assignee "Taro">"taro"`), slices.Index(got, `set priority "FV10">"TV08"`)
	if i >= j || j >= k {
		t.Errorf("order: status %d, assignee %d, priority %d", i, j, k)
	}
	if got[len(got)-1] != `add watchers >"hanako2"` {
		t.Errorf("last row = %s, want the watchers", got[len(got)-1])
	}
}

// 綴りが変わらない人は行を書かず、people にいない人はそのまま。置き換えたログインがすでにウォッチにいるときは足さない。
func TestRemapOtherStoreKeepsUnchangedAndUnknownPeople(t *testing.T) {
	from, to := pair()
	it := item.Item{ID: "I1", Assignee: "Taro", Watchers: []string{"Hanako", "hanako", "Stranger"}}
	got := show(transfer.Remap(it, from, to, "P1", transfer.PeopleMap{"taro": "Taro", "hanako": "hanako"}, nil))
	if want := []string{`remove watchers "Hanako">`}; !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestRemapStatus(t *testing.T) {
	from, to := pair()
	remap := func(status change.ID, choice map[change.ID]change.ID) []string {
		return show(transfer.Remap(item.Item{ID: "I1", Status: status}, from, to, "P1", nil, choice))
	}
	// 名前が一致して使えるものはそのまま(選択は使わない)。
	if got := remap("FV03", map[change.ID]change.ID{"FV03": "TV02"}); !slices.Equal(got, []string{`set status "FV03">"TV03"`}) {
		t.Errorf("Done = %q", got)
	}
	// 使えないものは、選んだステータス。
	if got := remap("FV04", map[change.ID]change.ID{"FV04": "TV02"}); !slices.Equal(got, []string{`set status "FV04">"TV02"`}) {
		t.Errorf("chosen = %q", got)
	}
	// 選んだものが使えるステータスにないときは、同じ分類の既定。
	if got := remap("FV04", map[change.ID]change.ID{"FV04": "NOWHERE"}); !slices.Equal(got, []string{`set status "FV04">"TV01"`}) {
		t.Errorf("invalid choice = %q", got)
	}
	// 移動元にいない ID は、移動先の初期ステータス。ステータスのないアイテムは何も書かない。
	if got := remap("FGONE", nil); !slices.Equal(got, []string{`set status "FGONE">"TV01"`}) {
		t.Errorf("unknown = %q", got)
	}
	if got := remap("", nil); len(got) != 0 {
		t.Errorf("no status = %q", got)
	}
}

func TestUsableStatusesIsEveryStatusOfTheWorkspace(t *testing.T) {
	_, to := pair()
	want := []change.ID{"TV01", "TV02", "TV03", "TV04", "TV05"}
	if got := transfer.UsableStatuses(to, "P1"); !slices.Equal(got, want) {
		t.Errorf("UsableStatuses = %v, want %v", got, want)
	}
}

// 移動先にない名前(Pending は移動先で Hold)だけを、渡した並びのまま返す。移動元にいない ID は返さない。
func TestUnmatchedReturnsOnlyTheStatusesTheDestinationCannotUse(t *testing.T) {
	from, to := pair()
	got := transfer.Unmatched([]change.ID{"FV05", "FV04", "FV01", "FGONE"}, from, to, "P1")
	if want := []change.ID{"FV04"}; !slices.Equal(got, want) {
		t.Errorf("Unmatched = %v, want %v", got, want)
	}
	if got := transfer.Unmatched(nil, from, to, "P1"); len(got) != 0 {
		t.Errorf("Unmatched(nil) = %v", got)
	}
}

// 予告の置き換えの行(Unmatched)と Remap は、同じ規則で分ける。選んだ置き換え先(Hold = TV04)になるのは、Unmatched にあるものだけ。
func TestUnmatchedAgreesWithRemap(t *testing.T) {
	from, to := pair()
	var all []change.ID
	for _, v := range from.Statuses {
		all = append(all, v.ID)
	}
	unmatched := transfer.Unmatched(all, from, to, "P1")
	for _, id := range all {
		got := show(transfer.Remap(item.Item{ID: "I1", Status: id}, from, to, "P1", nil, map[change.ID]change.ID{id: "TV04"}))
		chosen := slices.Contains(got, `set status "`+string(id)+`">"TV04"`)
		if inUnmatched := slices.Contains(unmatched, id); chosen != inUnmatched {
			t.Errorf("%s: 選んだ置き換え先になった = %v, Unmatched にある = %v", id, chosen, inUnmatched)
		}
	}
}
