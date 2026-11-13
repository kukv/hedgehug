package apptest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func TestFakeGitHubMissApp(t *testing.T) {
	ctx := context.Background()
	g := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	g.Put("Example-Org/web", port.Found{Repo: "Example-Org/web", Number: 1})
	g.MissApp("example-org", true)
	check := func(name string, err error, want bool) {
		t.Helper()
		var f *ghcall.Failure
		got := errors.As(err, &f) && f.Kind == ghcall.AppNotInstalled && f.Org == "Example-Org" && f.Repo == "Example-Org/web"
		if got != want {
			t.Errorf("%s: err = %v, want AppNotInstalled %v", name, err, want)
		}
	}
	_, err := g.Lookup(ctx, "tok-taro", "Example-Org/web", 1)
	check("Lookup", err, true)
	_, err = g.ReadMany(ctx, "tok-taro", "Example-Org/web", []int{1})
	check("ReadMany", err, true)
	_, err = g.ReadPulls(ctx, "tok-taro", "Example-Org/web", time.Time{})
	check("ReadPulls", err, true)
	check("CheckRepo", g.CheckRepo(ctx, "tok-taro", "Example-Org/web"), true)
	if err := g.CheckRepo(ctx, "tok-taro", "other/web"); err != nil {
		t.Errorf("other owner: %v", err)
	}
	g.MissApp("example-org", false)
	check("CheckRepo after restore", g.CheckRepo(ctx, "tok-taro", "Example-Org/web"), false)
	if _, err := g.Lookup(ctx, "tok-taro", "Example-Org/web", 1); err != nil {
		t.Errorf("Lookup after restore: %v", err)
	}
}
