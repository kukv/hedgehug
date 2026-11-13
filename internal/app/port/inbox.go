package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NoticeKind は、通知の種類(06「通知の種類」)。B4 は自動化の 2 つだけを書く。ほかは C4 が足す。
type NoticeKind string

const (
	NoticeAutomation        NoticeKind = "automation"         // 自動化が変えた。ステータスの削除でルールが変わった
	NoticeAutomationStopped NoticeKind = "automation_stopped" // 自動化が制約で止まった
)

// Notice は、あなた宛ての通知 1 件。差出人は種類で決まる(自動化は「hedgehug」)。
type Notice struct {
	Kind      NoticeKind
	Store     StoreID
	Workspace workspace.ID
	Item      item.ID   // 対象のアイテム。ルールが変わった通知では空(開く先は設定の「自動化」)
	Rule      change.ID // 自動化のルール
	Text      string    // 06 の本文
	Reason    string    // 止まった理由(NoticeAutomationStopped だけ)
	Status    change.ID // 止まった実行が変えようとしたステータス(「制約を無視して…」に使う)
}

// Inbox は、受信箱の口(B4-12)。実装は C4(infra/statedb)。既読と一覧は C4 が足す。
type Inbox interface {
	Add(ctx context.Context, n Notice) error
}
