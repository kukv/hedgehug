package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Identity は、Store ごとの「あなた」のログイン名(01「「あなた」の決まり方」)。
type Identity interface {
	Me(ctx context.Context, store StoreID) (string, error)
}

// IDs は内部 ID(ULID)の採番。
type IDs interface {
	New() change.ID
}
