package githublink_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

const every = 5 * time.Minute

// readClock は、テストが進める時計(呼んでも進まない)。
type readClock struct{ now time.Time }

func (c *readClock) Now() time.Time { return c.now }

type readFixture struct {
	*linkFixture
	clock *readClock
}

// newRead は、newLinks の組み立てに、テストが進める時計を入れたもの。
func newRead(t *testing.T) *readFixture {
	t.Helper()
	f := newLinks(t)
	c := &readClock{now: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
	f.links.Clock = c
	return &readFixture{linkFixture: f, clock: c}
}

func (f *readFixture) advance(d time.Duration) { f.clock.now = f.clock.now.Add(d) }

func (f *readFixture) read(t *testing.T, every time.Duration, force bool) {
	t.Helper()
	if err := f.links.ReadOnce(t.Context(), every, force); err != nil {
		t.Fatal(err)
	}
}

func readPR(repo string, n int, s itemlink.State) port.Found {
	return port.Found{Kind: itemlink.PR, Repo: repo, Number: n, Values: itemlink.Values{Title: "PR", State: s, Review: itemlink.NoReview}}
}

// linkIn は、store のアイテム id に、f の値で PR をリンクする(取り直しの記録を残さない)。
func (f *readFixture) linkIn(t *testing.T, store port.StoreID, id item.ID, found port.Found) {
	t.Helper()
	ref := itemlink.Ref{Kind: itemlink.PR, Repo: found.Repo, Number: found.Number}
	err := f.stores.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "リンク"}, func(tx port.Tx) error {
		log, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(id, itemlink.FromLog(log).Attach(ref, found.Values)...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// row は、store の HH-2 の、番号 n のリンクの行。
func (f *readFixture) row(t *testing.T, store port.StoreID, n int) githublink.LinkRow {
	t.Helper()
	rows, err := f.links.List(t.Context(), store, linkTask)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Number == n {
			return r
		}
	}
	t.Fatalf("no link #%d in %+v", n, rows)
	return githublink.LinkRow{}
}

// calls は、org/web と org/api に ReadMany を呼んだ回数。
func (f *readFixture) calls() [2]int {
	return [2]int{f.gh.ReadManyCalls("org/web"), f.gh.ReadManyCalls("org/api")}
}

// failed は、取り直しの失敗の文言。
func failed(text string) string { return "状態を更新できませんでした: " + text }

func TestReadOnceRefreshesNonMerged(t *testing.T) {
	f := newRead(t)
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 2, itemlink.Merged))
	f.linkIn(t, linkStore, linkEpic, readPR("org/api", 3, itemlink.Open))
	// #2 は GitHub に置かない: 頼めば「見つかりません」になる
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Merged))
	f.gh.Put("org/api", readPR("org/api", 3, itemlink.Open))
	f.read(t, every, false)
	if got := f.calls(); got != [2]int{1, 1} {
		t.Errorf("calls = %v", got)
	}
	if r := f.row(t, linkStore, 1); r.State != itemlink.Merged || r.Error != "" {
		t.Errorf("#1 = %+v", r)
	}
	if r := f.row(t, linkStore, 2); r.Error != "" {
		t.Errorf("#2 was asked: %+v", r)
	}
	if op := f.stores.ops[len(f.stores.ops)-1]; op.Actor != (change.Actor{Kind: change.ActorGitHub}) {
		t.Errorf("op = %+v", op)
	}
	if it := f.taskItem(t); it.Status != apptest.SampleStatusDone {
		t.Errorf("status = %s", it.Status)
	}
	if ns := f.inbox.Notices(); len(ns) != 1 || ns[0].Rule != ruleMerged {
		t.Errorf("notices = %+v", ns)
	}
}

func TestReadOnceWaitsForInterval(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.read(t, every, false)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 1 {
		t.Fatalf("calls = %d, want 1 inside the interval", n)
	}
	f.advance(every)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 2 {
		t.Fatalf("calls = %d, want 2 after the interval", n)
	}
	f.advance(10 * time.Minute)
	f.read(t, 15*time.Minute, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 2 {
		t.Fatalf("calls = %d, want 2 before 15 minutes", n)
	}
	f.advance(5 * time.Minute)
	f.read(t, 15*time.Minute, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 3 {
		t.Errorf("calls = %d, want 3 after 15 minutes", n)
	}
}

func TestReadOnceFailureWaits(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.gh.Fail("org/web", 0, &ghcall.Failure{Kind: ghcall.Unreachable})
	f.read(t, every, false)
	want := failed("GitHub に接続できません。接続を確かめて、もう一度お試しください")
	if r := f.row(t, linkStore, 1); r.Error != want {
		t.Errorf("error = %q", r.Error)
	}
	if ev := f.events.List(); len(ev) == 0 || ev[len(ev)-1] != (port.Event{Kind: port.EventGitHub, Store: linkStore}) {
		t.Errorf("events = %v", ev)
	}
	f.gh.Fail("org/web", 0, nil)
	f.advance(4 * time.Minute)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 1 {
		t.Fatalf("calls = %d, want no retry before the interval", n)
	}
	f.advance(time.Minute)
	f.read(t, every, false)
	if r := f.row(t, linkStore, 1); f.gh.ReadManyCalls("org/web") != 2 || r.Error != "" {
		t.Errorf("calls = %d, row = %+v", f.gh.ReadManyCalls("org/web"), r)
	}
}

func TestReadOnceLowRateTriples(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.gh.Rate = port.Rate{Limit: 5000, Remaining: 400, ResetAt: f.clock.now.Add(time.Hour)}
	f.read(t, every, false)
	f.advance(14 * time.Minute)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 1 {
		t.Fatalf("calls = %d, want 1 before 15 minutes", n)
	}
	f.advance(time.Minute)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 2 {
		t.Errorf("calls = %d, want 2 at 15 minutes", n)
	}
}

// resetText は、ghcall と同じ決め方(時計の時刻帯で分に切り上げ)の「HH:MM」。
func resetText(at, now time.Time) string {
	at = at.In(now.Location())
	if t := at.Truncate(time.Minute); !t.Equal(at) {
		at = t.Add(time.Minute)
	}
	return at.Format("15:04")
}

func TestReadOnceRateLimitedStopsAccount(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkTask, readPR("org/api", 3, itemlink.Open))
	f.linkIn(t, otherStore, linkTask, readPR("org/api", 3, itemlink.Open)) // 同じアカウントの別の Store
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/api", readPR("org/api", 3, itemlink.Open))
	reset := f.clock.now.Add(23 * time.Minute)
	f.gh.Fail("org/web", 0, &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: reset})
	f.read(t, every, false)
	if got := f.calls(); got != [2]int{1, 0} {
		t.Fatalf("calls = %v, want org/api not asked", got)
	}
	want := failed("GitHub の利用上限に達しました。" + resetText(reset, f.clock.now) + " 以降にもう一度お試しください")
	for _, n := range []int{1, 3} {
		if r := f.row(t, linkStore, n); r.Error != want {
			t.Errorf("#%d error = %q, want %q", n, r.Error, want)
		}
	}
	if r := f.row(t, otherStore, 3); r.Error != want {
		t.Errorf("other store error = %q, want %q", r.Error, want)
	}
	f.gh.Fail("org/web", 0, nil)
	f.advance(22 * time.Minute)
	f.read(t, every, false)
	if got := f.calls(); got != [2]int{1, 0} {
		t.Fatalf("calls = %v, want none before the reset", got)
	}
	f.advance(time.Minute)
	f.read(t, every, false)
	if got := f.calls(); got != [2]int{2, 2} {
		t.Errorf("calls = %v, want both at the reset", got)
	}
}

func TestReadOnceRateLimitedWithoutResetWaitsAMinute(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.gh.Fail("org/web", 0, &ghcall.Failure{Kind: ghcall.RateLimited})
	f.read(t, every, false)
	if r := f.row(t, linkStore, 1); r.Error != failed("GitHub の利用上限に達しました。しばらくしてからもう一度お試しください") {
		t.Errorf("error = %q", r.Error)
	}
	f.gh.Fail("org/web", 0, nil)
	f.advance(59 * time.Second)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 1 {
		t.Fatalf("calls = %d, want none before a minute", n)
	}
	f.advance(time.Second)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 2 {
		t.Errorf("calls = %d, want a read after a minute", n)
	}
}

func TestReadOnceRateLimitedPastResetWaitsAMinute(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	// このマシンの時計が GitHub より進んでいて、戻る時刻がもう過ぎている
	f.gh.Fail("org/web", 0, &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: f.clock.now.Add(-time.Second)})
	f.read(t, every, false)
	if r := f.row(t, linkStore, 1); r.Error != failed("GitHub の利用上限に達しました。しばらくしてからもう一度お試しください") {
		t.Errorf("error = %q, want no past time", r.Error)
	}
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 1 {
		t.Fatalf("calls = %d, want no retry right away", n)
	}
	f.advance(time.Minute)
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 2 {
		t.Errorf("calls = %d, want a read after a minute", n)
	}
}

func TestReadOnceAllMergedAcrossRepos(t *testing.T) {
	f := newRead(t)
	r := statusRule(ruleMerged, "すべてマージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone)
	r.AllMerged = true
	f.withRules(t, r)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkTask, readPR("org/api", 3, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Merged))
	f.gh.Put("org/api", readPR("org/api", 3, itemlink.Merged))
	f.read(t, every, false)
	if rows := ruleRows(f.taskLog(t)); len(rows) != 1 || len(f.inbox.Notices()) != 1 {
		t.Errorf("rule rows = %+v, notices = %+v", rows, f.inbox.Notices())
	}
	if it := f.taskItem(t); it.Status != apptest.SampleStatusDone {
		t.Errorf("status = %s", it.Status)
	}
}

func TestReadOnceMissingNumber(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 4, itemlink.Open))
	changed := readPR("org/web", 4, itemlink.Closed)
	f.gh.Put("org/web", changed)
	f.read(t, every, false)
	if r := f.row(t, linkStore, 1); r.Error != failed("org/web#1 が見つかりません") {
		t.Errorf("#1 error = %q", r.Error)
	}
	if r := f.row(t, linkStore, 4); r.State != itemlink.Closed || r.Error != "" {
		t.Errorf("#4 = %+v", r)
	}
}

const authText = "work(@taro)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください"

func TestReadOnceAuthError(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.tokens.rejected = []account.ID{"A1"}
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 0 {
		t.Errorf("calls = %d, want none", n)
	}
	if r := f.row(t, linkStore, 1); r.Error != failed(authText) {
		t.Errorf("error = %q", r.Error)
	}
}

func TestReadOnceTokenRejected(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.gh.Users = map[string]string{} // トークンを拒否する
	f.read(t, every, false)
	if len(f.tokens.rejected) != 1 || f.tokens.rejected[0] != "A1" {
		t.Errorf("rejected = %v", f.tokens.rejected)
	}
	if r := f.row(t, linkStore, 1); r.Error != failed(authText) {
		t.Errorf("error = %q", r.Error)
	}
}

func TestReadOnceSkipsStores(t *testing.T) {
	f := newRead(t)
	const third port.StoreID = "S3"
	f.stores.AddStore(third)
	if err := apptest.Sample(t.Context(), f.stores.Stores, third); err != nil {
		t.Fatal(err)
	}
	f.infos.Stores = []port.StoreGitHubInfo{
		{ID: linkStore, Name: "main", Account: "A1", ReadOnly: true},
		{ID: otherStore, Name: "other", Account: "A1", Hidden: true},
		{ID: third, Name: "third"},
	}
	// 削除済みのアイテムのリンクは頼まない(D4a-10)。削除の印は A4b が足す
	for _, s := range []port.StoreID{linkStore, otherStore, third} {
		f.linkIn(t, s, linkTask, readPR("org/web", 1, itemlink.Open))
	}
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 0 {
		t.Errorf("calls = %d, want none", n)
	}
	// 一覧は閲覧のみの Store の文言を出さないので、置いた文言そのものを見る
	for _, s := range []port.StoreID{linkStore, otherStore, third} {
		if got := f.reads.failures[s]; len(got) != 0 {
			t.Errorf("%s failures = %v", s, got)
		}
	}
}

func TestReadOnceRenamed(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web2", 1, itemlink.Open))
	f.read(t, every, false)
	if r := f.row(t, linkStore, 1); r.Repo != "org/web2" || r.Error != "" {
		t.Errorf("row = %+v", r)
	}
	recs := f.reads.records[linkStore]
	if _, ok := recs["org/web"]; !ok || len(recs) != 1 {
		t.Errorf("records = %+v, want the old name only", recs)
	}
}

func TestReadOnceForce(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Open))
	f.read(t, every, false)
	f.read(t, every, true)
	if n := f.gh.ReadManyCalls("org/web"); n != 2 {
		t.Errorf("calls = %d, want a forced read", n)
	}
}

// failingStores は、store への保存だけを失敗させる。
type failingStores struct {
	port.Stores
	store port.StoreID
	err   error
}

func (s failingStores) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if id == s.store {
		return s.err
	}
	return s.Stores.Write(ctx, id, op, fn)
}

func TestReadOnceSaveFailureKeepsOtherStores(t *testing.T) {
	f := newRead(t)
	for _, s := range []port.StoreID{linkStore, otherStore} {
		f.linkIn(t, s, linkTask, readPR("org/web", 1, itemlink.Open))
	}
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Closed))
	boom := errors.New("disk full")
	f.links.Stores = failingStores{Stores: f.stores, store: linkStore, err: boom}
	if err := f.links.ReadOnce(t.Context(), every, false); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the save failure", err)
	}
	if r := f.row(t, otherStore, 1); r.State != itemlink.Closed {
		t.Errorf("other store row = %+v", r)
	}
}

func TestReadOnceValuesChangedMeanwhile(t *testing.T) {
	f := newRead(t)
	f.withRules(t, statusRule(ruleOpen, "PR が Open で In Progress", automation.TriggerPROpen, apptest.SampleStatusInProgress))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	// GitHub から Open(題が変わった)を読んだあと、書く前に、別のところが Merged を書いた
	retitled := readPR("org/web", 1, itemlink.Open)
	retitled.Title = "PR v2"
	f.gh.Put("org/web", retitled)
	f.storeMeanwhile(t, linkStore, 1, readPR("org/web", 1, itemlink.Merged).Values)
	since := len(f.taskLog(t))
	f.read(t, every, false)
	f.noGitHubWrites(t, since)
	if r := f.row(t, linkStore, 1); r.State != itemlink.Merged || r.Title != "PR" {
		t.Errorf("row = %+v, want the newer value kept", r)
	}
}

func TestReadOnceAllMergedCountsValueStoredMeanwhile(t *testing.T) {
	f := newRead(t)
	r := statusRule(ruleMerged, "すべてマージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone)
	r.AllMerged = true
	f.withRules(t, r)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 2, itemlink.Open))
	retitled := readPR("org/web", 1, itemlink.Open)
	retitled.Title = "PR v2"
	f.gh.Put("org/web", retitled)
	f.gh.Put("org/web", readPR("org/web", 2, itemlink.Merged))
	// #1 は読んだあとでマージされ、別のところが Merged を書いた。#1 の古い値(Open)では数えない
	f.storeMeanwhile(t, linkStore, 1, readPR("org/web", 1, itemlink.Merged).Values)
	f.read(t, every, false)
	if rows := ruleRows(f.taskLog(t)); len(rows) != 1 || len(f.inbox.Notices()) != 1 {
		t.Errorf("rule rows = %+v, notices = %+v", rows, f.inbox.Notices())
	}
	if it := f.taskItem(t); it.Status != apptest.SampleStatusDone {
		t.Errorf("status = %s", it.Status)
	}
}

// ghcall の失敗でないエラー(キーチェーンの読み出しなど)は、行には「接続できません」の文言を置き、ReadOnce の
// エラーにも入れる(Reader がログに出す)。
func TestReadOnceReturnsNonGitHubErrors(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	boom := errors.New("keychain: read failed")
	f.gh.Err = boom
	if err := f.links.ReadOnce(t.Context(), every, false); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the underlying error", err)
	}
	if r := f.row(t, linkStore, 1); r.Error != failed("GitHub に接続できません。接続を確かめて、もう一度お試しください") {
		t.Errorf("error = %q", r.Error)
	}
}

// EventGitHub は、変わったものがあった Store ごとに 1 回だけ流し、何も変わらなければ流さない。
func TestReadOnceGitHubEventPerStore(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 2, itemlink.Open))
	f.linkIn(t, linkStore, linkEpic, readPR("org/api", 3, itemlink.Open))
	f.linkIn(t, otherStore, linkTask, readPR("org/web", 1, itemlink.Open))
	for _, n := range []int{1, 2} {
		pr := readPR("org/web", n, itemlink.Open)
		pr.Checks = itemlink.Success
		f.gh.Put("org/web", pr)
	}
	f.gh.Put("org/api", readPR("org/api", 3, itemlink.Closed))
	f.read(t, every, false)
	if s1, s2 := f.githubEvents(linkStore), f.githubEvents(otherStore); s1 != 1 || s2 != 1 {
		t.Fatalf("events = %d, %d, want one per store", s1, s2)
	}
	f.read(t, every, true) // 値もチェックの結果も文言も変わらない
	if s1, s2 := f.githubEvents(linkStore), f.githubEvents(otherStore); s1 != 1 || s2 != 1 {
		t.Errorf("events = %d, %d, want none when nothing changed", s1, s2)
	}
}
