package secret

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// deviceJSON は、デバイスフローのトークンの組をキーチェーンに置く形(前回の実装と同じ。この小計画の決定 D2-5)。
type deviceJSON struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	ExpiresAt        time.Time `json:"expires_at,omitzero"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitzero"`
}

// parseDevice は、キーチェーンの値を読む。JSON でなければ、値そのものを期限のないトークンとして読む。
func parseDevice(v string) account.DeviceToken {
	var d deviceJSON
	if err := json.Unmarshal([]byte(v), &d); err != nil || d.AccessToken == "" {
		return account.DeviceToken{Access: strings.TrimSpace(v)}
	}
	return account.DeviceToken{
		Access: strings.TrimSpace(d.AccessToken), Refresh: d.RefreshToken,
		ExpiresAt: d.ExpiresAt, RefreshExpiresAt: d.RefreshExpiresAt,
	}
}

// accessOf は、キーチェーンの値のうち、問い合わせに使うトークン。
func accessOf(v string) string { return parseDevice(v).Access }

func (s *Source) DeviceToken(_ context.Context, login string) (account.DeviceToken, error) {
	v, err := s.Keyring.Get(Service, KeychainKey(login))
	switch {
	case errors.Is(err, ErrNotFound):
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	case err != nil:
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	t := parseDevice(v)
	if t.Access == "" {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return t, nil
}

func (s *Source) SaveDeviceToken(_ context.Context, login string, t account.DeviceToken) error {
	b, err := json.Marshal(deviceJSON{ //nolint:gosec // G117: JSON はキーチェーンにだけ置く
		AccessToken: t.Access, RefreshToken: t.Refresh, ExpiresAt: t.ExpiresAt, RefreshExpiresAt: t.RefreshExpiresAt,
	})
	if err != nil {
		return err
	}
	if err := s.Keyring.Set(Service, KeychainKey(login), string(b)); err != nil {
		return &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	return nil
}

func (s *Source) DeleteDeviceToken(_ context.Context, login string) error {
	err := s.Keyring.Delete(Service, KeychainKey(login))
	if err == nil || errors.Is(err, ErrNotFound) {
		return nil
	}
	return &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
}
