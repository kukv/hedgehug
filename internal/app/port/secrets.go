package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Secrets はトークンの口。トークンは、この口の外では、問い合わせに渡すあいだだけ持つ。
type Secrets interface {
	// Token は、アカウントの認証方法でトークンを読む(デバイスフローは除く)。読めなければ *ghcall.Failure
	// (GhTokenMissing・GhTimeout・EnvMissing・KeychainMissing・KeychainUnavailable)を返す。
	Token(ctx context.Context, a account.Account) (string, error)
	// DeviceToken は、デバイスフローで得たトークンの組を読む。項目がなければ TokenRejected、
	// キーチェーンがなければ KeychainUnavailable の *ghcall.Failure。
	DeviceToken(ctx context.Context, login string) (account.DeviceToken, error)
	// SaveDeviceToken は、デバイスフローで得たトークンの組を置く。キーチェーンがなければ KeychainUnavailable。
	SaveDeviceToken(ctx context.Context, login string, t account.DeviceToken) error
	// DeleteDeviceToken は、デバイスフローで得たトークンの組を消す。項目がなくてもエラーにしない。
	DeleteDeviceToken(ctx context.Context, login string) error
}
