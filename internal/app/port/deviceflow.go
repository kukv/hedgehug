package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// DeviceCode は、デバイスフローの始まりに GitHub が返すもの。DeviceCode は画面に出さない。
type DeviceCode struct {
	DeviceCode      string
	UserCode        string // XXXX-XXXX
	VerificationURI string
	ExpiresIn       time.Duration
	Interval        time.Duration
}

// DeviceStatus は、許可を待つ問い合わせの答え。
type DeviceStatus string

const (
	DevicePending  DeviceStatus = "pending"
	DeviceSlowDown DeviceStatus = "slow_down" // 間隔を 5 秒延ばす
	DeviceExpired  DeviceStatus = "expired"
	DeviceDenied   DeviceStatus = "denied"
	DeviceGranted  DeviceStatus = "granted"
)

// DeviceGrant は、許可を待つ問い合わせの結果。Token は Granted のときだけ。
type DeviceGrant struct {
	Status DeviceStatus
	Token  account.DeviceToken
}

// DeviceFlow は、GitHub のデバイスフローの口(07「デバイスフロー」)。失敗は *ghcall.Failure。
type DeviceFlow interface {
	StartDevice(ctx context.Context, clientID string) (DeviceCode, error)
	PollDevice(ctx context.Context, clientID, deviceCode string) (DeviceGrant, error)
	// RefreshDevice は、更新用のトークンで取り直す。GitHub が拒否したら TokenRejected。
	RefreshDevice(ctx context.Context, clientID, refresh string) (account.DeviceToken, error)
}
