package port

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// ErrNoLogin は、Store の「あなた」を決められない(割り当てたアカウントも、マシンのログイン名もない)ことを表す。
var ErrNoLogin = errors.New("このマシンのログイン名が未設定です")

// Identity は、Store ごとの「あなた」のログイン名(01「「あなた」の決まり方」)。
type Identity interface {
	Me(ctx context.Context, store StoreID) (string, error)
}

// IDs は内部 ID(ULID)の採番。
type IDs interface {
	New() change.ID
}
