package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// SyncTarget は、同期に要る Store の設定(08 の Store の一覧のうち、このマシンだけの値)。
type SyncTarget struct {
	ID       StoreID
	Name     string
	Remote   string     // remote の URL。なければ空
	Mirrors  []string   // ミラーの URL
	Account  account.ID // GitHub に使うアカウント。「なし」なら空
	Hidden   bool
	ReadOnly bool
}

// SyncTargets は、同期に要る Store の設定の口。本物の実装はトラック G が足す。
type SyncTargets interface {
	// List は、このマシンのすべての Store(非表示・閲覧のみ・remote なしを含む)を、サイドバーの並びで返す。
	List(ctx context.Context) ([]SyncTarget, error)
}
