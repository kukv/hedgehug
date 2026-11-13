package github_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

var _ port.GitHubTokens = github.Connector{}

func TestRecheckRecovers(t *testing.T) {
	f := newConn(t)
	f.conn.Rejected(work.ID)
	got, err := f.conn.Recheck(context.Background())
	if err != nil || !got || f.state(t, 0).AuthError {
		t.Fatalf("Recheck = %v, %v, state %+v", got, err, f.state(t, 0))
	}
	// 確かめた指紋を覚えるので、次の接続で問い合わせ直さない。
	calls := f.gh.calls
	if _, err := f.conn.Token(context.Background(), work); err != nil || f.gh.calls != calls {
		t.Errorf("after recheck: %v, calls %d → %d", err, calls, f.gh.calls)
	}
}

func TestRecheckStaysOnMismatch(t *testing.T) {
	f := newConn(t)
	f.conn.Rejected(work.ID)
	f.secrets.Tokens["taro"] = "tok-jiro"
	f.gh.Users["tok-jiro"] = "jiro"
	got, err := f.conn.Recheck(context.Background())
	if err != nil || got || !f.state(t, 0).AuthError {
		t.Fatalf("Recheck = %v, %v, state %+v", got, err, f.state(t, 0))
	}
}

func TestRecheckStaysOnRejected(t *testing.T) {
	for _, kind := range []ghcall.Kind{ghcall.TokenRejected, ghcall.Unreachable} {
		f := newConn(t)
		f.conn.Rejected(work.ID)
		f.gh.Err = &ghcall.Failure{Kind: kind}
		got, err := f.conn.Recheck(context.Background())
		if err != nil || got || !f.state(t, 0).AuthError {
			t.Errorf("kind %v: Recheck = %v, %v, state %+v", kind, got, err, f.state(t, 0))
		}
	}
}

func TestRecheckSkipsDeviceFlow(t *testing.T) {
	f := newConn(t)
	f.conn.Rejected(home.ID)
	got, err := f.conn.Recheck(context.Background())
	if err != nil || got || f.gh.calls != 0 || !f.state(t, 1).AuthError {
		t.Fatalf("Recheck = %v, %v, calls %d, state %+v", got, err, f.gh.calls, f.state(t, 1))
	}
}

func TestRecheckIgnoresHealthy(t *testing.T) {
	f := newConn(t)
	got, err := f.conn.Recheck(context.Background())
	if err != nil || got || f.gh.calls != 0 {
		t.Fatalf("Recheck = %v, %v, calls %d", got, err, f.gh.calls)
	}
}
