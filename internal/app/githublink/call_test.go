package githublink_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// appMissingFixture は、org/web のリンクを S1 に置いた組み立て。
func appMissingFixture(t *testing.T) *linkFixture {
	t.Helper()
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *linkFixture) listMissing(t *testing.T) map[account.ID][]port.MissingOrg {
	t.Helper()
	m, err := f.missing.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCallRecordsAppMissing(t *testing.T) {
	f := appMissingFixture(t)
	f.gh.MissApp("org", true)
	before := len(f.events.List())
	rows := f.refresh(t)
	want := "状態を更新できませんでした: org に GitHub App が入っていないため、org/web を読めません"
	if rows[0].Error != want {
		t.Errorf("error = %q, want %q", rows[0].Error, want)
	}
	got := f.listMissing(t)
	if !slices.Equal(got["A1"], []port.MissingOrg{{Org: "org", Repo: "org/web"}}) {
		t.Errorf("missing = %+v", got)
	}
	ev := f.events.List()[before:]
	if !slices.Contains(ev, port.Event{Kind: port.EventGitHub}) {
		t.Errorf("events = %v, want EventGitHub with empty Store", ev)
	}
}

func TestCallClearsOnSuccess(t *testing.T) {
	f := appMissingFixture(t)
	f.gh.MissApp("org", true)
	f.refresh(t)
	f.gh.MissApp("org", false)
	before := len(f.events.List())
	rows := f.refresh(t)
	if rows[0].Error != "" {
		t.Errorf("error = %q", rows[0].Error)
	}
	if got := f.listMissing(t); len(got["A1"]) != 0 {
		t.Errorf("missing = %+v, want none", got)
	}
	if !slices.Contains(f.events.List()[before:], port.Event{Kind: port.EventGitHub}) {
		t.Errorf("events = %v", f.events.List()[before:])
	}
}

func TestCallOtherAccountDoesNotClear(t *testing.T) {
	f := appMissingFixture(t)
	if err := f.add(t, otherStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	f.gh.MissApp("org", true)
	f.refresh(t)
	// S2 を別のアカウントに割り当て、そのアカウントでは読めるようにする
	if err := f.accounts.Add(t.Context(), account.Account{ID: "A2", Name: "side", Login: "taro", Method: account.EnvVar}); err != nil {
		t.Fatal(err)
	}
	f.infos.Stores[1].Account = "A2"
	f.gh.MissApp("org", false)
	if _, err := f.links.Refresh(t.Context(), otherStore, linkTask); err != nil {
		t.Fatal(err)
	}
	got := f.listMissing(t)
	if !slices.Equal(got["A1"], []port.MissingOrg{{Org: "org", Repo: "org/web"}}) {
		t.Errorf("A1 missing = %+v, want kept", got["A1"])
	}
}

func TestCallOtherOwnerDoesNotClear(t *testing.T) {
	f := appMissingFixture(t)
	if _, err := f.missing.Put(t.Context(), "A1", port.MissingOrg{Org: "other-org", Repo: "other-org/x"}); err != nil {
		t.Fatal(err)
	}
	f.refresh(t) // org/web は読める
	got := f.listMissing(t)
	if !slices.Equal(got["A1"], []port.MissingOrg{{Org: "other-org", Repo: "other-org/x"}}) {
		t.Errorf("missing = %+v, want kept", got)
	}
}
