//go:build live

package github

import (
	"context"
	"os"
	"strings"
	"testing"
)

// 本物の GitHub に問い合わせる(github-live)。リポジトリに設定済みの secret HEDGEHUG_LIVE_TOKEN と
// 変数 HEDGEHUG_LIVE_ORG(検証用の組織。その hh-live リポジトリを読む)が要る。
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

func TestLiveLookup(t *testing.T) {
	token := liveToken(t)
	org := os.Getenv("HEDGEHUG_LIVE_ORG")
	// hh-live の #1 があることだけを前提にする(PR か Issue かは問わない。前回の github-live も hh-live の
	// PR と開いている Issue があることを前提にしていた)。
	f, err := New(nil).Lookup(context.Background(), token, org+"/hh-live", 1)
	if err != nil || f.Number != 1 || f.Title == "" || !strings.EqualFold(f.Repo, org+"/hh-live") {
		t.Errorf("Lookup = %+v, %v", f, err)
	}
	if _, err := New(nil).Lookup(context.Background(), token, org+"/hh-live", 999999); err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Errorf("missing number: %v", err)
	}
}

func TestLiveRejectsBadToken(t *testing.T) {
	liveToken(t)
	_, err := New(nil).Viewer(context.Background(), "not-a-real-token")
	if err == nil || !strings.Contains(err.Error(), "token_rejected") {
		t.Errorf("err = %v", err)
	}
}
