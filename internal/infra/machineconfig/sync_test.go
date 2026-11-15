package machineconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestSyncTimingDefaults(t *testing.T) {
	got, err := Open(filepath.Join(t.TempDir(), "config.yaml")).SyncSettings().Timing(context.Background())
	want := port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second, GitHubEvery: 5 * time.Minute}
	if err != nil || got != want {
		t.Errorf("Timing = %+v, %v", got, err)
	}
}

func TestSyncTimingReadsTheSyncSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sync:\n  auto: false\n  interval: 900\n  push_interval: 10\n  github_interval: 900\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Open(path).SyncSettings().Timing(context.Background())
	want := port.SyncTiming{Auto: false, Interval: 15 * time.Minute, AfterWrite: 10 * time.Second, GitHubEvery: 15 * time.Minute}
	if err != nil || got != want {
		t.Errorf("Timing = %+v, %v", got, err)
	}
}

// アカウントの追加などで config.yaml を書き直しても、sync の項目は残る。
func TestSyncSectionSurvivesRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sync:\n  interval: 60\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := Open(path)
	a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.GhCLI}
	if err := f.Accounts().Add(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	got, err := Open(path).SyncSettings().Timing(context.Background())
	if err != nil || got.Interval != time.Minute || !got.Auto {
		t.Errorf("Timing = %+v, %v", got, err)
	}
}

// 選択肢にない値(手で書き換えた)は初期値として扱う。選択肢の値はそのまま(G1-18)。
func TestTimingSnapsToChoices(t *testing.T) {
	ctx := context.Background()
	f := writeConfig(t, "sync:\n  interval: 120\n  push_interval: 45\n  github_interval: 60\n")
	got, err := f.SyncSettings().Timing(ctx)
	want := port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second, GitHubEvery: 5 * time.Minute}
	if err != nil || got != want {
		t.Errorf("Timing = %+v, %v", got, err)
	}
	f = writeConfig(t, "sync:\n  interval: 3600\n  push_interval: 60\n  github_interval: 1800\n")
	got, err = f.SyncSettings().Timing(ctx)
	want = port.SyncTiming{Auto: true, Interval: time.Hour, AfterWrite: time.Minute, GitHubEvery: 30 * time.Minute}
	if err != nil || got != want {
		t.Errorf("Timing = %+v, %v", got, err)
	}
}

func TestSetTimingRoundTrip(t *testing.T) {
	ctx := context.Background()
	f := writeConfig(t, "login: taro\n")
	want := port.SyncTiming{Auto: false, Interval: 15 * time.Minute, AfterWrite: 10 * time.Second, GitHubEvery: 30 * time.Minute}
	if err := f.SyncSettings().SetTiming(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := f.SyncSettings().Timing(ctx)
	if err != nil || got != want {
		t.Errorf("Timing = %+v, %v", got, err)
	}
	c, err := f.read()
	if err != nil || c.Sync.Auto == nil || *c.Sync.Auto || c.Login != "taro" {
		t.Errorf("config = %+v, %v", c, err)
	}
}
