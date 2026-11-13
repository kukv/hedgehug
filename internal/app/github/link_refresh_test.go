package github_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func (f *linkFixture) refresh(t *testing.T) []ghapp.LinkRow {
	t.Helper()
	rows, err := f.links.Refresh(t.Context(), linkStore, linkTask)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestRefreshSavesChangedValuesAsGitHub(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	writes := len(f.stores.ops)
	f.refresh(t) // 変わっていない: 保存しない
	if len(f.stores.ops) != writes {
		t.Fatalf("saved without a change: %+v", f.stores.ops[writes:])
	}
	changed := pr41
	changed.State, changed.Review, changed.Checks = itemlink.Merged, itemlink.Approved, itemlink.Success
	f.gh.Put("org/web", changed)
	rows := f.refresh(t)
	op := f.stores.ops[len(f.stores.ops)-1]
	if op.Actor != (change.Actor{Kind: change.ActorGitHub}) ||
		op.Summary != "HH-2 の org/web#41 の状態を Merged に変更、HH-2 の org/web#41 のレビューを 承認済み に変更" {
		t.Errorf("op = %+v", op)
	}
	if rows[0].State != itemlink.Merged || rows[0].Checks != itemlink.Success || rows[0].Error != "" {
		t.Errorf("row = %+v", rows[0])
	}
	if ev := f.events.List(); len(ev) == 0 || ev[len(ev)-1] != (port.Event{Kind: port.EventGitHub, Store: linkStore}) {
		t.Errorf("events = %v", ev)
	}
}

func TestRefreshFollowsRename(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	moved := pr41
	moved.Repo = "org/web2"
	f.gh.Put("org/web", moved)
	rows := f.refresh(t)
	if got := displays(rows); !slices.Equal(got, []string{"pr org/web2#41 Add login form open"}) || rows[0].Error != "" {
		t.Errorf("rows = %q %+v", got, rows)
	}
	if f.lastSummary() != "HH-2 のリンクを org/web#41 から org/web2#41 に更新" {
		t.Errorf("summary = %q", f.lastSummary())
	}
}

func TestRefreshKeepsOldNameWhenRenameCollides(t *testing.T) {
	f := newLinks(t)
	f.gh.Put("org/web2", port.Found{Kind: itemlink.PR, Repo: "org/web2", Number: 41, Values: itemlink.Values{Title: "x", State: itemlink.Open, Review: itemlink.NoReview}})
	for _, repo := range []string{"org/web", "org/web2"} {
		if err := f.add(t, linkStore, linkTask, itemlink.PR, repo, "41", false); err != nil {
			t.Fatal(err)
		}
	}
	moved := pr41
	moved.Repo, moved.State = "org/web2", itemlink.Closed
	f.gh.Put("org/web", moved)
	rows := f.refresh(t)
	if got := displays(rows); !slices.Contains(got, "pr org/web#41 Add login form closed") {
		t.Errorf("rows = %q (the old name stays, values are refreshed)", got)
	}
	for _, r := range rows {
		if r.Error != "" {
			t.Errorf("row %s has an error %q", r.Display(), r.Error)
		}
	}
	if s := f.lastSummary(); s != "HH-2 の org/web#41 の状態を Closed に変更" {
		t.Errorf("summary = %q, want no rename", s)
	}
}

func TestRefreshKeepsOldNameWhenRenamedIssueIsTaken(t *testing.T) {
	f := newLinks(t)
	moved := iss12
	moved.Repo = "org/api2"
	f.gh.Put("org/api2", moved)
	if err := f.add(t, linkStore, linkEpic, itemlink.Issue, "org/api2", "12", false); err != nil {
		t.Fatal(err)
	}
	if err := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Fatal(err)
	}
	moved.State = itemlink.Closed
	f.gh.Put("org/api", moved)
	rows := f.refresh(t)
	if got := displays(rows); !slices.Equal(got, []string{"issue org/api#12 落ちる closed"}) || rows[0].Error != "" {
		t.Errorf("rows = %q %+v (the old name stays, values are refreshed)", got, rows)
	}
	if s := f.lastSummary(); s != "HH-2 の org/api#12 の状態を Closed に変更" {
		t.Errorf("summary = %q, want no rename", s)
	}
}

func TestRefreshDoesNotRenameOntoALinkAddedMeanwhile(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	moved := pr41
	moved.Repo, moved.State = "org/web2", itemlink.Closed
	f.gh.Put("org/web", moved)
	web2 := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web2", Number: 41}
	f.stores.before = func() { f.attachNow(t, linkTask, web2, moved.Values) } // 取り直すあいだに誰かが付けた
	rows := f.refresh(t)
	if got := displays(rows); !slices.Equal(got, []string{"pr org/web#41 Add login form closed", "pr org/web2#41 Add login form closed"}) {
		t.Errorf("rows = %q (no rename onto the new link; values are refreshed)", got)
	}
	if s := f.lastSummary(); s != "HH-2 の org/web#41 の状態を Closed に変更" {
		t.Errorf("summary = %q, want no rename", s)
	}
}

func TestRefreshRenameKeepsChecksOfAnotherItem(t *testing.T) {
	f := newLinks(t)
	for _, id := range []item.ID{linkTask, linkEpic} {
		if err := f.add(t, linkStore, id, itemlink.PR, "org/web", "41", false); err != nil {
			t.Fatal(err)
		}
	}
	moved := pr41
	moved.Repo, moved.Checks = "org/web2", itemlink.Success
	f.gh.Put("org/web", moved)
	f.refresh(t) // HH-2 だけ改名を追う
	got, _ := f.checks.Read(t.Context(), linkStore)
	if got["pr:org/web2#41"] != itemlink.Success || got["pr:org/web#41"] != itemlink.Running {
		t.Errorf("checks = %v, want the new name and HH-1's old name both kept", got)
	}
	if rows := f.list(t, linkStore, linkEpic); len(rows) != 1 || rows[0].Checks != itemlink.Running {
		t.Errorf("HH-1 rows = %+v", rows)
	}
}

func TestRefreshStopsAtTheRateLimit(t *testing.T) {
	f := newLinks(t)
	f.gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 42, Values: itemlink.Values{Title: "b", State: itemlink.Open, Review: itemlink.NoReview}})
	for _, n := range []string{"41", "42"} {
		if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", n, false); err != nil {
			t.Fatal(err)
		}
	}
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.RateLimited})
	asked := f.gh.Lookups
	rows := f.refresh(t)
	if f.gh.Lookups-asked != 1 {
		t.Errorf("lookups = %d, want 1 (stop at the rate limit)", f.gh.Lookups-asked)
	}
	if len(rows) != 2 || rows[0].Error == "" || rows[1].Error != rows[0].Error {
		t.Errorf("rows = %+v, want the same error on both", rows)
	}
}

func TestRefreshFailureShowsOnTheRow(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: "org/web"})
	rows := f.refresh(t)
	if rows[0].Error != "状態を更新できませんでした: org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください" || rows[0].State != itemlink.Open {
		t.Errorf("row = %+v", rows[0])
	}
	f.gh.Fail("org/web", 41, nil)
	if rows := f.refresh(t); rows[0].Error != "" {
		t.Errorf("the error stays after a success: %+v", rows[0])
	}
}

func TestRefreshSkips(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	asked := f.gh.Lookups
	f.infos.Stores[0].ReadOnly = true
	f.refresh(t)
	if f.gh.Lookups != asked {
		t.Error("a read-only store was refreshed")
	}
	f.infos.Stores[0].ReadOnly = false
	f.infos.Stores[0].Account = ""
	rows := f.refresh(t)
	if f.gh.Lookups != asked || rows[0].Error != "状態を更新できませんでした: org/web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください" {
		t.Errorf("no account assigned: lookups %d, row %+v", f.gh.Lookups-asked, rows[0])
	}
	if err := f.accounts.Remove(t.Context(), "A1"); err != nil {
		t.Fatal(err)
	}
	if rows := f.refresh(t); rows[0].Error != "" || f.gh.Lookups != asked {
		t.Errorf("no accounts on this machine: row %+v", rows[0])
	}
}

func TestProjectLinks(t *testing.T) {
	f := newLinks(t)
	f.gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 42, Values: itemlink.Values{Title: "b", State: itemlink.Draft, Review: itemlink.NoReview}})
	for _, a := range []struct {
		id     item.ID
		kind   itemlink.Kind
		repo   string
		number string
	}{{linkTask, itemlink.PR, "org/web", "41"}, {linkEpic, itemlink.PR, "org/web", "42"}, {linkTask, itemlink.Issue, "org/api", "12"}} {
		if err := f.add(t, linkStore, a.id, a.kind, a.repo, a.number, false); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := f.links.ProjectLinks(t.Context(), linkStore, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Display+" "+r.Link.Display())
	}
	if want := []string{"HH-1 org/web#42", "HH-2 org/web#41", "HH-2 org/api#12"}; !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q (item number, then link order)", got, want)
	}
}
