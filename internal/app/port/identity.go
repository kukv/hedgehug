package port

import (
	"context"
	"errors"
	"slices"

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

type joinedKey struct{}

// joined は、書き込みと一緒にメンバーへ足したログイン名を、足した順に持つ。
type joined struct{ logins []string }

// WithJoined は、メンバーへ足したログイン名を積む入れ物を置いた context を返す(決定 G1-6)。
func WithJoined(ctx context.Context) context.Context {
	return context.WithValue(ctx, joinedKey{}, &joined{})
}

// Joined は、入れ物に積んだログイン名を、積んだ順に返す(同じものは 1 回)。入れ物がなければ nil。
func Joined(ctx context.Context) []string {
	v, _ := ctx.Value(joinedKey{}).(*joined)
	if v == nil {
		return nil
	}
	return slices.Clone(v.logins)
}

// NoteJoined は、入れ物に login を積む(同じものは 1 つ)。入れ物がなければ捨てる。保存できたあとに呼ぶ。
func NoteJoined(ctx context.Context, login string) {
	if v, _ := ctx.Value(joinedKey{}).(*joined); v != nil && !slices.Contains(v.logins, login) {
		v.logins = append(v.logins, login)
	}
}
