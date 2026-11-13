package apptest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
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

func TestGitHubLookup(t *testing.T) {
	g := &apptest.GitHub{Users: map[string]string{"tok": "taro"}}
	f := port.Found{
		Kind: itemlink.PR, Repo: "org/web2", Number: 41,
		Values: itemlink.Values{Title: "A", State: itemlink.Open, Review: itemlink.Pending}, Checks: itemlink.Success,
	}
	g.Put("Org/Web", f) // 改名前の名前で引いても、いまの名前で返る
	got, err := g.Lookup(t.Context(), "tok", "org/web", 41)
	if err != nil || got != f {
		t.Errorf("Lookup = %+v, %v", got, err)
	}
	if _, err := g.Lookup(t.Context(), "tok", "org/web", 42); !ghcall.Is(err, ghcall.NotFound) {
		t.Errorf("missing number: %v", err)
	}
	if _, err := g.Lookup(t.Context(), "bad", "org/web", 41); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("unknown token: %v", err)
	}
	if g.Lookups != 3 {
		t.Errorf("Lookups = %d", g.Lookups)
	}
}

func TestGitHubReadPulls(t *testing.T) {
	g := &apptest.GitHub{}
	g.AddUser("tok-taro", "taro")
	t1 := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	g.PutPull("o/web", port.Pull{Number: 3, UpdatedAt: t1})
	g.PutPull("o/web", port.Pull{Number: 4, UpdatedAt: t2})
	got, err := g.ReadPulls(t.Context(), "tok-taro", "O/Web", time.Time{})
	if err != nil || len(got.Pulls) != 2 || got.Pulls[0].Number != 4 || got.Pulls[1].Number != 3 || got.Repo != "O/Web" {
		t.Fatalf("ReadPulls = %+v, %v", got, err)
	}
	got, err = g.ReadPulls(t.Context(), "tok-taro", "o/web", t2)
	if err != nil || len(got.Pulls) != 1 || got.Pulls[0].Number != 4 {
		t.Errorf("since = t2: %+v, %v", got, err)
	}
	if g.ReadPullsCalls("o/web") != 2 {
		t.Errorf("ReadPullsCalls = %d, want 2", g.ReadPullsCalls("o/web"))
	}
	if _, err := g.ReadPulls(t.Context(), "bad", "o/web", time.Time{}); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("unknown token: %v", err)
	}
	g.Fail("o/web", 0, &ghcall.Failure{Kind: ghcall.RepoUnreadable})
	if _, err := g.ReadPulls(t.Context(), "tok-taro", "o/web", time.Time{}); !ghcall.Is(err, ghcall.RepoUnreadable) {
		t.Errorf("Fail: %v", err)
	}
}

func TestChecksContract(t *testing.T) {
	apptest.RunChecksContract(t, func(*testing.T) port.Checks { return apptest.NewChecks() })
}

func TestFakeGitHubReadMany(t *testing.T) {
	g := &apptest.GitHub{}
	g.AddUser("tok-taro", "taro")
	g.Rate = port.Rate{Limit: 5000, Remaining: 4000}
	g.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 1})
	g.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 2})
	b, err := g.ReadMany(t.Context(), "tok-taro", "Org/Web", []int{1, 2, 3})
	if err != nil || len(b.Found) != 2 || len(b.Missing) != 1 || b.Missing[0] != 3 || b.Rate.Remaining != 4000 {
		t.Fatalf("ReadMany = %+v, %v", b, err)
	}
	if g.ReadManyCalls("org/web") != 1 {
		t.Errorf("ReadManyCalls = %d", g.ReadManyCalls("org/web"))
	}
	// 番号ごとの NotFound は、問い合わせ全体を失敗させず、その番号を Missing にする(port.GitHub.ReadMany の約束)
	g.Fail("org/web", 2, &ghcall.Failure{Kind: ghcall.NotFound})
	b, err = g.ReadMany(t.Context(), "tok-taro", "org/web", []int{1, 2})
	if _, found := b.Found[2]; err != nil || found || len(b.Found) != 1 || len(b.Missing) != 1 || b.Missing[0] != 2 {
		t.Errorf("per-number NotFound: %+v, %v", b, err)
	}
	// 番号ごとの NotFound 以外の失敗は、問い合わせ全体の失敗にする
	g.Fail("org/web", 2, &ghcall.Failure{Kind: ghcall.Unreachable})
	var f *ghcall.Failure
	if _, err := g.ReadMany(t.Context(), "tok-taro", "org/web", []int{1, 2}); !errors.As(err, &f) || f.Kind != ghcall.Unreachable {
		t.Errorf("per-number failure: err = %v", err)
	}
	g.Fail("org/web", 2, nil)
	g.Fail("org/web", 0, &ghcall.Failure{Kind: ghcall.RateLimited})
	if _, err := g.ReadMany(t.Context(), "tok-taro", "org/web", []int{1}); !errors.As(err, &f) || f.Kind != ghcall.RateLimited {
		t.Errorf("repo failure: err = %v", err)
	}
	if g.ReadManyCalls("org/web") != 4 {
		t.Errorf("ReadManyCalls = %d, want 4", g.ReadManyCalls("org/web"))
	}
}
