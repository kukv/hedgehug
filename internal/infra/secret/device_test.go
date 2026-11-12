package secret

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func TestDeviceTokenRoundTrip(t *testing.T) {
	k := fakeKeyring{}
	s := &Source{Keyring: k}
	want := account.DeviceToken{
		Access: "tok-a", Refresh: "tok-r",
		ExpiresAt:        time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC),
		RefreshExpiresAt: time.Date(2027, 5, 13, 10, 0, 0, 0, time.UTC),
	}
	if err := s.SaveDeviceToken(context.Background(), "taro", want); err != nil {
		t.Fatal(err)
	}
	if v := k["hedgehug|github.com/taro"]; !strings.Contains(v, `"access_token":"tok-a"`) {
		t.Errorf("kept %q", v)
	}
	got, err := s.DeviceToken(context.Background(), "taro")
	if err != nil || !got.ExpiresAt.Equal(want.ExpiresAt) || got.Access != want.Access || got.Refresh != want.Refresh ||
		!got.RefreshExpiresAt.Equal(want.RefreshExpiresAt) {
		t.Errorf("DeviceToken = %+v, %v", got, err)
	}
	if err := s.DeleteDeviceToken(context.Background(), "taro"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDeviceToken(context.Background(), "taro"); err != nil {
		t.Errorf("deleting what is not there: %v", err)
	}
	if _, err := s.DeviceToken(context.Background(), "taro"); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("after delete: %v", err)
	}
}

func TestDeviceTokenPlainValue(t *testing.T) {
	// OS のキーチェーンの認証方法から替えたときの、素のトークン(期限なし)。
	s := &Source{Keyring: fakeKeyring{"hedgehug|github.com/taro": " tok-plain "}}
	got, err := s.DeviceToken(context.Background(), "taro")
	if err != nil || got.Access != "tok-plain" || !got.ExpiresAt.IsZero() {
		t.Errorf("DeviceToken = %+v, %v", got, err)
	}
}

func TestDeviceTokenNoKeychain(t *testing.T) {
	s := &Source{Keyring: fakeKeyring(nil)}
	if _, err := s.DeviceToken(context.Background(), "taro"); !ghcall.Is(err, ghcall.KeychainUnavailable) {
		t.Errorf("read: %v", err)
	}
	if err := s.SaveDeviceToken(context.Background(), "taro", account.DeviceToken{Access: "tok-a"}); !ghcall.Is(err, ghcall.KeychainUnavailable) {
		t.Errorf("save: %v", err)
	}
}
