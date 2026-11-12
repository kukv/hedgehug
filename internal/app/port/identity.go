package port

import "context"

// Identity は、Store ごとの「あなた」のログイン名(01「「あなた」の決まり方」)。
// 本物の実装はトラック G が足す。
type Identity interface {
	Me(ctx context.Context, store StoreID) (string, error)
}
