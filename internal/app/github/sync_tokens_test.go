package github_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

var _ port.SyncTokens = github.Connector{}

func TestStoreTokenUsesTheAssignedAccount(t *testing.T) {
	f := newConn(t)
	tok, err := f.conn.StoreToken(context.Background(), "S1")
	if err != nil || tok != "tok-taro" {
		t.Errorf("StoreToken = %q, %v", tok, err)
	}
	if _, err := f.conn.StoreToken(context.Background(), "S2"); !ghcall.Is(err, ghcall.NoAccount) {
		t.Errorf("no account: %v", err)
	}
}

// 07「状態」: 同期で remote がトークンを拒否したら、そのアカウントを認証エラーにする。
func TestStoreRejectedMarksTheAccountBroken(t *testing.T) {
	f := newConn(t)
	f.conn.StoreRejected(context.Background(), "S1")
	if _, err := f.conn.StoreToken(context.Background(), "S1"); !ghcall.Is(err, ghcall.AuthError) {
		t.Errorf("after rejection: %v", err)
	}
	f.conn.StoreRejected(context.Background(), "S2") // 割り当てがなければ何もしない
	f.conn.StoreRejected(context.Background(), "S9")
}
