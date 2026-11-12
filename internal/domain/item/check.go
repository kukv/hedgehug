package item

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
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
