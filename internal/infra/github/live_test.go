//go:build live

package github

import (
	"context"
	"os"
	"strings"
	"testing"
)

// 本物の GitHub に問い合わせる(github-live)。HEDGEHUG_LIVE_TOKEN と HEDGEHUG_LIVE_LOGIN が要る。
func liveEnv(t *testing.T) (token, login string) {
	t.Helper()
	token, login = os.Getenv("HEDGEHUG_LIVE_TOKEN"), os.Getenv("HEDGEHUG_LIVE_LOGIN")
	if token == "" || login == "" {
		t.Skip("HEDGEHUG_LIVE_TOKEN and HEDGEHUG_LIVE_LOGIN are not set")
	}
	return token, login
}

func TestLiveViewer(t *testing.T) {
	token, login := liveEnv(t)
	got, err := New(nil).Viewer(context.Background(), token)
	if err != nil || !strings.EqualFold(got, login) {
		t.Errorf("Viewer = %q, %v; want %q", got, err, login)
	}
}

func TestLiveRejectsBadToken(t *testing.T) {
	liveEnv(t)
	_, err := New(nil).Viewer(context.Background(), "not-a-real-token")
	if err == nil || !strings.Contains(err.Error(), "token_rejected") {
		t.Errorf("err = %v", err)
	}
}
