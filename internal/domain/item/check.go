package item

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NormalizeTitle は、連続する空白と改行を 1 つの空白にまとめ、前後の空白を取り除く(02「入力の検査」)。
// 作成・詳細パネル・テンプレートから作る子・複製のすべての経路で使う。
func NormalizeTitle(s string) string { return strings.Join(strings.Fields(s), " ") }

// PlaceMessage は、置けない場所へ置こうとしたときの文言(02「アイテムの種類と親子」)。
func PlaceMessage(child Kind, parent *Kind) string {
	if parent == nil {
		return "プロジェクトの直下には Epic だけを置けます"
	}
	return fmt.Sprintf("%s は %s の下に置けません", child, *parent)
}

// CheckCreate は、作成の検査を 02「入力の検査」の表の順に行い、最初に合わなかった 1 つを返す。
// parent は選んだ親の種類で、選んでいなければ nil。日付・ポイント・見積もり時間の検査は A3 で足す。
func CheckCreate(kind Kind, title string, projectChosen bool, parent *Kind) error {
	if NormalizeTitle(title) == "" {
		return refusal.New("タイトルを入力してください")
	}
	if !projectChosen {
		return refusal.New("プロジェクトを選んでください")
	}
	if kind != Epic && parent == nil {
		if kind == Step {
			return refusal.New("親になる Story・Task・Bug を選んでください")
		}
		return refusal.New("親になる Epic を選んでください")
	}
	if !CanPlace(kind, parent) {
		return refusal.New(PlaceMessage(kind, parent))
	}
	return nil
}

// CheckInputs は、作成の入力 in を 02「入力の検査」の表の順(日付・ポイント・見積もり時間)に確かめ、
// ほかの値も ParseInput で読んで、ログに書く値にして返す。関連システムと複数選択は ID の配列のまま返す。
// タイトル・プロジェクト・親の検査(CheckCreate)は、呼ぶ側が先に行う。
func CheckInputs(st workspace.Settings, in map[change.ID]json.RawMessage) (map[change.ID]json.RawMessage, error) {
	if err := CheckDates(text(in["start"]), text(in["end"])); err != nil {
		return nil, err
	}
	out := map[change.ID]json.RawMessage{}
	ordered := append([]change.ID{"points", "estimate"}, keysExcept(in, "points", "estimate")...)
	for _, id := range ordered {
		v, ok := in[id]
		if !ok {
			continue
		}
		p, ok := st.Property(id)
		if !ok {
			return nil, ErrNoSuchValue
		}
		parsed, err := parseCreateValue(p, v, st)
		if err != nil {
			return nil, err
		}
		out[id] = parsed
	}
	return out, nil
}

// parseCreateValue は、複数の値(関連システム・複数選択)を ID の配列で、ほかは ParseInput で読む。
func parseCreateValue(p workspace.Property, v json.RawMessage, st workspace.Settings) (json.RawMessage, error) {
	if p.Type != workspace.TypeSystems && p.Type != workspace.TypeMulti {
		return ParseInput(p, v, st)
	}
	var ids []change.ID
	if json.Unmarshal(v, &ids) != nil {
		return nil, ErrBadInput
	}
	list := st.Systems
	if p.Type == workspace.TypeMulti {
		list = p.Choices
	}
	for _, id := range ids {
		if _, err := pickValue(string(id), list, false); err != nil {
			return nil, err
		}
	}
	return change.Value(ids), nil
}

// text は、入力の文字列。なし・null・文字列でない値は空(日付の検査は空を飛ばす)。
func text(v json.RawMessage) string {
	var s string
	_ = json.Unmarshal(v, &s)
	return s
}

// keysExcept は、in のキーのうち skip にないものを辞書順で返す(結果を決まった順にする)。
func keysExcept(in map[change.ID]json.RawMessage, skip ...change.ID) []change.ID {
	var out []change.ID
	for k := range in {
		if !slices.Contains(skip, k) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}
