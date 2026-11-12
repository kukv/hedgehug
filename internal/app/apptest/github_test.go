package apptest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func TestFakeSecretsDeviceToken(t *testing.T) {
	ctx := context.Background()
	s := &apptest.Secrets{}
	if err := s.SaveDeviceToken(ctx, "taro", account.DeviceToken{Access: "tok-taro"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.DeviceToken(ctx, "taro")
	if err != nil || got.Access != "tok-taro" {
		t.Fatalf("DeviceToken = %+v, %v", got, err)
	}
	if err := s.DeleteDeviceToken(ctx, "taro"); err != nil {
		t.Fatal(err)
	}
	var f *ghcall.Failure
	if _, err := s.DeviceToken(ctx, "taro"); !errors.As(err, &f) || f.Kind != ghcall.TokenRejected {
		t.Errorf("after delete: err = %v", err)
	}
}

func TestFakeSecretsDeleteFailure(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	s := &apptest.Secrets{Device: map[string]account.DeviceToken{"taro": {Access: "tok-taro"}}, DeleteErr: boom}
	if err := s.DeleteDeviceToken(ctx, "taro"); !errors.Is(err, boom) {
		t.Errorf("DeleteDeviceToken = %v, want DeleteErr", err)
	}
	if _, ok := s.Device["taro"]; !ok {
		t.Error("a failed delete must keep the token")
	}
}

func TestFakeSecretsNoKeychainStillDeletes(t *testing.T) {
	s := &apptest.Secrets{NoKeychain: true, Device: map[string]account.DeviceToken{"taro": {Access: "tok-taro"}}}
	if err := s.DeleteDeviceToken(context.Background(), "taro"); err != nil {
		t.Errorf("DeleteDeviceToken with NoKeychain = %v", err)
	}
}
