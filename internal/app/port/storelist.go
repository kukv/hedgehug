package port

import "context"

// StoreInfo は、このマシンに取り付けた Store の 1 つ。名前・色・閲覧のみの印は、マシンごとの設定(01)。
type StoreInfo struct {
	ID       StoreID
	Name     string // Store 名(英数字・ハイフン・アンダースコア。08)
	Color    string // 画面のトークン --t-<色> の名前(例 "sky")
	ReadOnly bool
}

// StoreList は、表示中の Store の一覧の口。
type StoreList interface {
	// List は、表示中の Store を、サイドバーの並びで返す。非表示の Store は返さない。
	List(ctx context.Context) ([]StoreInfo, error)
}
