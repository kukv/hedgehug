// Package descriptions は、アイテムとプロジェクトの説明の読み取りと保存(02「説明」)を持つ。1 ファイル 1 操作。
package descriptions

import "github.com/kukv/hedgehug/internal/app/port"

// Service は、説明の操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
	Inbox    port.Inbox // 同じ箇所が重なって両方を残した保存の通知(C4c1-13)
}

// Result は、説明の保存の結果。Text は保存したあとの説明、Conflict は、同じ箇所が同時に編集されて
// 両方の版を残したとき true(画面がトーストを出す)。
type Result struct {
	Text     string
	Conflict bool
}
