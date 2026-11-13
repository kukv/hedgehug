package github_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const (
	linkStore  port.StoreID = "S1"
	otherStore port.StoreID = "S2"
	linkTask                = apptest.SampleTask // HH-2
	linkEpic                = apptest.SampleEpic // HH-1
)

// linkRecorder は、Write に渡した操作を覚える。
type linkRecorder struct {
	*apptest.Stores
	ops []port.Operation
}

func (r *linkRecorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.ops = append(r.ops, op)
	return r.Stores.Write(ctx, id, op, fn)
}

type linkFixture struct {
	links    ghapp.Links
	stores   *linkRecorder
	gh       *apptest.GitHub
	accounts *apptest.Accounts
	infos    *apptest.StoreGitHub
	checks   *apptest.Checks
	events   *apptest.Events
}

var (
	pr41  = port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 41, Values: itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}, Checks: itemlink.Running}
	iss12 = port.Found{Kind: itemlink.Issue, Repo: "org/api", Number: 12, Values: itemlink.Values{Title: "落ちる", State: itemlink.Open, Review: itemlink.NoReview}}
)

// newLinks は、Store S1・S2 にサンプルを入れ、S1 と S2 にアカウント「work」(@taro、環境変数)を割り当てた組み立て。
func newLinks(t *testing.T) *linkFixture {
	t.Helper()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, apptest.NewIDs())
	for _, id := range []port.StoreID{linkStore, otherStore} {
		s.AddStore(id)
		if err := apptest.Sample(t.Context(), s, id); err != nil {
			t.Fatal(err)
		}
	}
	accounts := apptest.NewAccounts()
	work := account.Account{ID: "A1", Name: "work", Login: "taro", Method: account.EnvVar}
	if err := accounts.Add(t.Context(), work); err != nil {
		t.Fatal(err)
	}
	gh := &apptest.GitHub{Users: map[string]string{"tok": "taro"}}
	gh.Put("org/web", pr41)
	gh.Put("org/api", iss12)
	infos := &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{
		{ID: linkStore, Name: "main", Account: "A1"}, {ID: otherStore, Name: "other", Account: "A1"},
	}}
	f := &linkFixture{stores: &linkRecorder{Stores: s}, gh: gh, accounts: accounts, infos: infos, checks: apptest.NewChecks(), events: &apptest.Events{}}
	f.links = ghapp.Links{
		Stores: f.stores, Identity: apptest.Identity{Login: "hanako"},
		Connector: ghapp.Connector{
			Session: ghapp.NewSession(), Accounts: accounts, Secrets: &apptest.Secrets{Tokens: map[string]string{"taro": "tok"}},
			GitHub: gh, Stores: infos, Clock: clock,
		},
		Checks: f.checks, Events: f.events,
	}
	return f
}

func (f *linkFixture) add(t *testing.T, store port.StoreID, id item.ID, kind itemlink.Kind, repo, number string, toProject bool) error {
	t.Helper()
	return f.links.Add(t.Context(), store, id, ghapp.AddLinkInput{Kind: kind, Repo: repo, Number: number, AddToProject: toProject})
}

//nolint:unparam // Task 9 のテストが別の Store でも使う
func (f *linkFixture) list(t *testing.T, store port.StoreID, id item.ID) []ghapp.LinkRow {
	t.Helper()
	rows, err := f.links.List(t.Context(), store, id)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func (f *linkFixture) lastSummary() string {
	if len(f.stores.ops) == 0 {
		return ""
	}
	return f.stores.ops[len(f.stores.ops)-1].Summary
}

// refusalText は、err が断りならその文言、そうでなければ空。
func refusalText(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func displays(rows []ghapp.LinkRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = string(r.Kind) + " " + r.Display() + " " + r.Title + " " + string(r.State)
	}
	return out
}

func TestAddLinkSavesValuesAndChecks(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, " org/web ", "41", false); err != nil {
		t.Fatal(err)
	}
	if got := displays(f.list(t, linkStore, linkTask)); !slices.Equal(got, []string{"pr org/web#41 Add login form open"}) {
		t.Errorf("rows = %q", got)
	}
	if f.lastSummary() != "HH-2 に org/web#41 をリンク" {
		t.Errorf("summary = %q", f.lastSummary())
	}
	if op := f.stores.ops[len(f.stores.ops)-1]; op.Actor != change.User("hanako") {
		t.Errorf("actor = %+v", op.Actor)
	}
	if rows := f.list(t, linkStore, linkTask); rows[0].Checks != itemlink.Running || rows[0].Review != itemlink.Pending {
		t.Errorf("row = %+v", rows[0])
	}
}

func TestAddLinkRefusals(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Fatal(err)
	}
	f.gh.Put("org/api", port.Found{Kind: itemlink.Issue, Repo: "org/api", Number: 13, Values: itemlink.Values{Title: "x", State: itemlink.Open, Review: itemlink.NoReview}})
	cases := []struct {
		name         string
		id           item.ID
		kind         itemlink.Kind
		repo, number string
		want         string
	}{
		{"repo form", linkEpic, itemlink.PR, "org/*", "1", "リポジトリは owner/name の形式で入力してください"},
		{"two slashes", linkEpic, itemlink.PR, "a/b/c", "1", "リポジトリは owner/name の形式で入力してください"},
		{"number", linkEpic, itemlink.PR, "org/web", "0", "番号を入力してください"},
		{"not a number", linkEpic, itemlink.PR, "org/web", "x", "番号を入力してください"},
		{"already (case, kind ignored)", linkTask, itemlink.PR, "ORG/API", "12", "もうリンクしています"},
		{"not found", linkEpic, itemlink.PR, "org/web", "99", "org/web#99 が見つかりません"},
		{"PR but issue", linkEpic, itemlink.PR, "org/api", "12", "org/api#12 は PR ではなく Issue です"},
		{"issue but PR", linkEpic, itemlink.Issue, "org/web", "41", "org/web#41 は Issue ではなく PR です"},
		{"issue taken", linkEpic, itemlink.Issue, "org/api", "12", "org/api#12 は HH-2 にリンクされています"},
		{"item has issue", linkTask, itemlink.Issue, "org/api", "13", "このアイテムには Issue がリンクされています"},
	}
	for _, c := range cases {
		before := len(f.stores.ops)
		err := f.add(t, linkStore, c.id, c.kind, c.repo, c.number, false)
		if got := refusalText(err); got != c.want {
			t.Errorf("%s: %q (%v), want %q", c.name, got, err, c.want)
		}
		if len(f.stores.ops) != before {
			t.Errorf("%s: wrote", c.name)
		}
	}
}

func TestAddIssueTakenInAnotherStore(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, otherStore, linkEpic, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Fatal(err)
	}
	err := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false)
	if got := refusalText(err); got != "org/api#12 は HH-1 にリンクされています" {
		t.Errorf("%q (%v)", got, err)
	}
}

func TestAddIssueTakenBySameInternalIDInAnotherStore(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, otherStore, linkTask, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Fatal(err)
	}
	err := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false)
	if got := refusalText(err); got != "org/api#12 は HH-2 にリンクされています" {
		t.Errorf("%q (%v)", got, err)
	}
}

func TestAddLinkGitHubFailures(t *testing.T) {
	f := newLinks(t)
	f.infos.Stores[0].Account = ""
	if got := refusalText(f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false)); got != "org/web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください" {
		t.Errorf("no account: %q", got)
	}
	f.infos.Stores[0].Account = "A1"
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.Unreachable})
	if got := refusalText(f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false)); got != "GitHub に接続できません。接続を確かめて、もう一度お試しください" {
		t.Errorf("unreachable: %q", got)
	}
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.TokenRejected})
	if got := refusalText(f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false)); got != "work(@taro)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください" {
		t.Errorf("rejected: %q", got)
	}
}

func TestAddLinkToProject(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", true); err != nil {
		t.Fatal(err)
	}
	if f.lastSummary() != "HH-2 に org/web#41 をリンク(プロジェクトのリポジトリにも追加)" {
		t.Errorf("summary = %q", f.lastSummary())
	}
	var p project.Project
	_ = f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		p, err = r.Projects().Get(apptest.SampleProject)
		return err
	})
	if !slices.Equal(p.Repos, []string{"org/web"}) {
		t.Errorf("repos = %v", p.Repos)
	}
	// プロジェクトにもうあるリポジトリは足さない
	f.gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 42, Values: itemlink.Values{Title: "b", State: itemlink.Open, Review: itemlink.NoReview}})
	if err := f.add(t, linkStore, linkEpic, itemlink.PR, "ORG/web", "42", true); err != nil {
		t.Fatal(err)
	}
	if f.lastSummary() != "HH-1 に org/web#42 をリンク" {
		t.Errorf("summary = %q", f.lastSummary())
	}
}

func TestRemoveLink(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	ref := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	if err := f.links.Remove(t.Context(), linkStore, linkTask, ref); err != nil {
		t.Fatal(err)
	}
	if rows := f.list(t, linkStore, linkTask); len(rows) != 0 || f.lastSummary() != "HH-2 のリンクを解除" {
		t.Errorf("rows = %v, summary %q", rows, f.lastSummary())
	}
	// 解除の記録に入り、手でリンクし直すと消える(07「解除」)
	var log []change.Record
	_ = f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: linkTask})
		return err
	})
	if off := itemlink.FromLog(log).Off; len(off) != 1 || off[0] != ref {
		t.Errorf("off = %v", off)
	}
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	_ = f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		log, _ = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: linkTask})
		return nil
	})
	if off := itemlink.FromLog(log).Off; len(off) != 0 {
		t.Errorf("off after relink = %v", off)
	}
	// ないリンクの解除は何も保存しない(行のない Write は何も残さない。port.Stores の約束)
	if err := f.links.Remove(t.Context(), linkStore, linkTask, itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 7}); err != nil {
		t.Fatal(err)
	}
	if rows := f.list(t, linkStore, linkTask); len(rows) != 1 {
		t.Errorf("rows = %v", rows)
	}
}

func TestLinksOfMissingItem(t *testing.T) {
	f := newLinks(t)
	if _, err := f.links.List(t.Context(), linkStore, "01TESTIT0000000000000000NO"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List = %v", err)
	}
	if err := f.add(t, linkStore, "01TESTIT0000000000000000NO", itemlink.PR, "org/web", "41", false); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add = %v", err)
	}
}
