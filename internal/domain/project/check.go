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
	if start != "" && end != "" && end < start {
		return refusal.New("終了日は開始日以降にしてください")
	}
	for _, s := range siblings {
		if s.Title == t {
			return refusal.New("このワークスペースには同じ名前のプロジェクトがあります")
		}
	}
	return nil
}
