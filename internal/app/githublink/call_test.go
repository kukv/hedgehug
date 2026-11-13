package githublink_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
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
	before := len(f.events.List())
	f.refresh(t) // org/web は読める
	got := f.listMissing(t)
	if !slices.Equal(got["A1"], []port.MissingOrg{{Org: "other-org", Repo: "other-org/x"}}) {
		t.Errorf("missing = %+v, want kept", got)
	}
	if n := countEvents(f.events.List()[before:], port.Event{Kind: port.EventGitHub}); n != 0 {
		t.Errorf("EventGitHub (no store) = %d, want 0", n)
	}
}

// 記録の保存に失敗したら、問い合わせが成功していても失敗として返す(行の文言は「接続できません」の文)。
func TestCallReturnsRecordError(t *testing.T) {
	f := appMissingFixture(t)
	f.missing.err = errors.New("state.db を書けない")
	rows := f.refresh(t)
	if want := "状態を更新できませんでした: GitHub に接続できません。接続を確かめて、もう一度お試しください"; rows[0].Error != want {
		t.Errorf("error = %q, want %q", rows[0].Error, want)
	}
}

// 同じ組織で、記録したリポジトリ(org/web)が読めず、別のリポジトリ(org/api)は読めるとき、記録は消えず、
// 記録を置いたときの 1 回のほかに EventGitHub を流さない(D4b 最終レビュー I-1)。
func TestCallOtherRepoOfSameOrgDoesNotClear(t *testing.T) {
	f := appMissingFixture(t)
	if err := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Fatal(err)
	}
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.AppNotInstalled, Org: "org", Repo: "org/web"})
	before := len(f.events.List())
	f.refresh(t)
	got := f.listMissing(t)
	if !slices.Equal(got["A1"], []port.MissingOrg{{Org: "org", Repo: "org/web"}}) {
		t.Errorf("missing = %+v, want kept", got)
	}
	if n := countEvents(f.events.List()[before:], port.Event{Kind: port.EventGitHub}); n != 1 {
		t.Errorf("EventGitHub (no store) = %d, want 1", n)
	}
	before = len(f.events.List())
	f.refresh(t)
	if got := f.listMissing(t); len(got["A1"]) != 1 {
		t.Errorf("missing after 2nd = %+v, want kept", got)
	}
	if n := countEvents(f.events.List()[before:], port.Event{Kind: port.EventGitHub}); n != 0 {
		t.Errorf("EventGitHub (no store) on 2nd = %d, want 0", n)
	}
}

func countEvents(ev []port.Event, want port.Event) int {
	n := 0
	for _, e := range ev {
		if e == want {
			n++
		}
	}
	return n
}

// 改名・移管したリポジトリでは、記録は答えの名前(org/site)で置かれる。問い合わせた名前(org/web)で読めたら、
// 答えの名前でも当てて消す(D4b 最終レビュー I-1)。
func TestCallClearsRenamedRepo(t *testing.T) {
	f := appMissingFixture(t)
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.AppNotInstalled, Org: "org", Repo: "org/site"})
	f.refresh(t)
	if got := f.listMissing(t); !slices.Equal(got["A1"], []port.MissingOrg{{Org: "org", Repo: "org/site"}}) {
		t.Fatalf("missing = %+v", got)
	}
	f.gh.Fail("org/web", 41, nil)
	moved := pr41
	moved.Repo = "org/site"
	f.gh.Put("org/web", moved)
	f.refresh(t)
	if got := f.listMissing(t); len(got["A1"]) != 0 {
		t.Errorf("missing = %+v, want cleared", got)
	}
}
