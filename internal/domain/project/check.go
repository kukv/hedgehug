package project

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// CheckCreate は、プロジェクトの作成の検査(02「プロジェクト」の「作成」の表の順)。
// siblings は同じワークスペースのプロジェクト(削除済みを除く)。名前は前後の空白を除いて完全一致で比べる。
// 日付は YYYY-MM-DD の形で来る(API の型で確かめる)ので、文字列の比較で前後が分かる。
func CheckCreate(title, start, end string, siblings []Project) error {
	t := strings.TrimSpace(title)
	if t == "" {
		return refusal.New("タイトルを入力してください")
	}
	if err := CheckDates(start, end); err != nil {
		return err
	}
	return checkSameName(t, "", siblings)
}

// CheckRename は、タイトルの変更の検査(02「概要の編集」)。title は前後の空白を除いたもの。
// 空は断り(作成とは文言が違う)、自分(self)を除く同じワークスペースの別のプロジェクトと同じ名前も断る。
func CheckRename(title string, self ID, siblings []Project) error {
	if title == "" {
		return refusal.New("タイトルは空にできません")
	}
	return checkSameName(title, self, siblings)
}

// CheckDates は、開始日と終了日の組を検査する。どちらかが空なら比べない。同じ日は認める。
func CheckDates(start, end string) error {
	if start != "" && end != "" && end < start {
		return refusal.New("終了日は開始日以降にしてください")
	}
	return nil
}

func checkSameName(title string, self ID, siblings []Project) error {
	for _, s := range siblings {
		if (self == "" || s.ID != self) && s.Title == title { // self が空なら(作成)、誰も除かない
			return refusal.New("このワークスペースには同じ名前のプロジェクトがあります")
		}
	}
	return nil
}
