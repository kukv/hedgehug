//go:build live

package github

import (
	"context"
	"os"
	"strings"
	"testing"
)

// 本物の GitHub に問い合わせる(github-live)。リポジトリに設定済みの secret HEDGEHUG_LIVE_TOKEN と
// 変数 HEDGEHUG_LIVE_ORG(検証用の組織。リポジトリを読む操作は D3 から確かめる)が要る。
func liveToken(t *testing.T) string {
	t.Helper()
	token, org := os.Getenv("HEDGEHUG_LIVE_TOKEN"), os.Getenv("HEDGEHUG_LIVE_ORG")
	if token == "" || org == "" {
		t.Skip("HEDGEHUG_LIVE_TOKEN and HEDGEHUG_LIVE_ORG are not set")
	}
	return token
}

func TestLiveViewer(t *testing.T) {
	token := liveToken(t)
	// トークンの持ち主のログイン名は設定に持たないので、空でない名前が返ることだけを確かめる。
	got, err := New(nil).Viewer(context.Background(), token)
	if err != nil || got == "" {
		t.Errorf("Viewer = %q, %v", got, err)
	}
}

func TestLiveRejectsBadToken(t *testing.T) {
	liveToken(t)
	_, err := New(nil).Viewer(context.Background(), "not-a-real-token")
	if err == nil || !strings.Contains(err.Error(), "token_rejected") {
		t.Errorf("err = %v", err)
	}
}
