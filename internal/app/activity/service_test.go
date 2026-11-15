package activity

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
)

const (
	s2 port.StoreID = "S2"
	s3 port.StoreID = "S3"
)

var s2Info = port.StoreInfo{ID: s2, Name: "team", Color: "rose"}

// moreCommits は、内側の StoreCommits(nil なら空)に、手で作ったコミットを足して返す(行のない bot のコミットや、
// 時刻を決めたコミットを作るため)。
type moreCommits struct {
	inner port.StoreCommits
	extra map[port.StoreID][]port.Commit
}

func (m *moreCommits) Commits(ctx context.Context, id port.StoreID) ([]port.Commit, error) {
	var out []port.Commit
	if m.inner != nil {
		got, err := m.inner.Commits(ctx, id)
		if err != nil {
			return nil, err
		}
		out = got
	}
	out = append(out, m.extra[id]...)
	slices.SortFunc(out, func(a, b port.Commit) int { return cmp.Or(b.At.Compare(a.At), cmp.Compare(a.SHA, b.SHA)) })
	return out, nil
}

func (m *moreCommits) add(id port.StoreID, c port.Commit) {
	if m.extra == nil {
		m.extra = map[port.StoreID][]port.Commit{}
	}
	m.extra[id] = append(m.extra[id], c)
}

type pageFixture struct {
	fixture
	syncs   *apptest.SyncLog
	commits *moreCommits
	svc     Service
}

// newPageFixture は、表示中の S1(Sample の Web と、SampleMobile の Mobile)・S2(SampleTeam)と、非表示の S3
// (SampleTeam)を作る。
func newPageFixture(t *testing.T) pageFixture {
	t.Helper()
	f := pageFixture{fixture: newFixture(t), syncs: &apptest.SyncLog{}}
	if err := apptest.SampleMobile(t.Context(), f.log, s1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []port.StoreID{s2, s3} {
		f.stores.AddStore(id)
		if err := apptest.SampleTeam(t.Context(), f.log, id); err != nil {
			t.Fatal(err)
		}
	}
	f.commits = &moreCommits{inner: f.log}
	f.svc = Service{Stores: f.log, Commits: f.commits, Log: f.syncs, List: apptest.StoreList{Stores: []port.StoreInfo{s1Info, s2Info}}}
	return f
}

func (f pageFixture) page(t *testing.T, q Query) Page {
	t.Helper()
	p, err := f.svc.Page(t.Context(), q)
	if err != nil {
		t.Fatalf("Page(%+v): %v", q, err)
	}
	return p
}

func (f pageFixture) sync(t *testing.T, id port.StoreID, sent, received int) {
	t.Helper()
	if err := f.syncs.Add(t.Context(), id, f.clock.Now(), sent, received); err != nil {
		t.Fatal(err)
	}
}

func texts(rows []Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Text
	}
	return out
}

func countText(rows []Row, text string) int {
	n := 0
	for _, r := range rows {
		if r.Text == text {
			n++
		}
	}
	return n
}

// assertOrdered は、行が時刻の新しい順、同じなら鍵の辞書順の逆に並ぶことを確かめる(C4d-14)。
func assertOrdered(t *testing.T, rows []Row) {
	t.Helper()
	for i := 1; i < len(rows); i++ {
		a, b := rows[i-1], rows[i]
		if a.At.Before(b.At) || a.At.Equal(b.At) && a.Key <= b.Key {
			t.Fatalf("rows %d (%v %q) and %d (%v %q) are out of order", i-1, a.At, a.Key, i, b.At, b.Key)
		}
	}
}

func TestPageOrdersByTime(t *testing.T) {
	f := newPageFixture(t)
	f.setItem(t, "HH-1 のタイトルを変更", apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))
	f.sync(t, s1, 2, 1)
	f.write(t, change.User("hanako"), "HH-2 のタイトルを変更", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面", "サインイン画面"))
	})
	future := port.Commit{SHA: strings.Repeat("f", 40), Summary: "未来の保存", Author: "taro", At: f.clock.Now().Add(365 * 24 * time.Hour)}
	f.commits.add(s2, future)
	rows := f.page(t, Query{}).Rows
	assertOrdered(t, rows)
	if rows[0].Text != "未来の保存" || rows[0].Store != s2Info {
		t.Errorf("first row = %+v, want the future save", rows[0])
	}
	if rows[1].Text != "HH-2 のタイトルを変更" || rows[3].Text != "HH-1 のタイトルを変更" {
		t.Errorf("rows = %q", texts(rows[:4]))
	}
	s := rows[2]
	if s.Text != "origin と同期 (送信 2 件・受信 1 件)" || s.Who != "同期" || s.Actor != ActorSync || s.SHA != "" || s.Avatar != "" {
		t.Errorf("sync row = %+v", s)
	}
	if s.Key != "sync:1" || s.Store != s1Info || len(s.Workspaces) != 0 || s.Storewide || s.Item != nil || s.Project != nil {
		t.Errorf("sync row = %+v", s)
	}
}

func TestPageMergesNonHumanDuplicates(t *testing.T) {
	f := newPageFixture(t)
	f.addRule(t)
	f.setStatusByRule(t, "1 回目")
	f.setItem(t, "戻す", apptest.SampleTask, change.NewSet("status", apptest.SampleStatusDone, apptest.SampleStatusTodo))
	f.setStatusByRule(t, "2 回目")
	const reset = "スプリント「Sprint 25」を計画中に戻す"
	at := f.clock.Now()
	f.commits.add(s1, port.Commit{SHA: strings.Repeat("b", 40), Summary: reset, Author: change.BotLogin, At: at.Add(time.Second)})
	f.commits.add(s1, port.Commit{SHA: strings.Repeat("a", 40), Summary: reset, Author: change.BotLogin, At: at})
	// 同じ時刻なら SHA の辞書順で前のものを残す。
	const reset26 = "スプリント「Sprint 26」を計画中に戻す"
	f.commits.add(s1, port.Commit{SHA: strings.Repeat("9", 40), Summary: reset26, Author: change.BotLogin, At: at})
	f.commits.add(s1, port.Commit{SHA: strings.Repeat("8", 40), Summary: reset26, Author: change.BotLogin, At: at})
	f.setItem(t, "HH-1 のタイトルを変更", apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))
	f.setItem(t, "HH-1 のタイトルを変更", apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))

	rows := f.page(t, Query{}).Rows
	auto := "「進行中なら完了」でステータスを Done に変更"
	first := f.commit(t, "1 回目")
	if countText(rows, auto) != 1 {
		t.Errorf("automation rows = %d, want 1", countText(rows, auto))
	}
	for _, r := range rows {
		if r.Text == auto && r.SHA != first.SHA[:7] {
			t.Errorf("automation row sha = %q, want the earliest %q", r.SHA, first.SHA[:7])
		}
		if r.Text == reset && r.SHA != "aaaaaaa" || r.Text == reset26 && r.SHA != "8888888" {
			t.Errorf("%s: sha = %q, want the earliest", r.Text, r.SHA)
		}
	}
	if countText(rows, reset) != 1 || countText(rows, reset26) != 1 || countText(rows, "HH-1 のタイトルを変更") != 2 {
		t.Errorf("reset rows = %d, title rows = %d; want 1 and 2", countText(rows, reset), countText(rows, "HH-1 のタイトルを変更"))
	}
	// まとめてから絞り込む: 「自動化だけ」でも 1 行。
	if got := f.page(t, Query{Kind: KindAutomation}).Rows; countText(got, auto) != 1 {
		t.Errorf("automation rows in the automation filter = %q", texts(got))
	}
}

// kindFixture は、人(taro・hanako)・自動化・GitHub・同期の修復・同期の行・実行者なしの行を足す。
func kindFixture(t *testing.T) pageFixture {
	t.Helper()
	f := newPageFixture(t)
	f.addRule(t)
	f.setStatusByRule(t, "HH-2 のステータスを変更")
	f.write(t, change.User("hanako"), "HH-1 の説明を変更", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))
	})
	f.write(t, change.Actor{Kind: change.ActorGitHub}, "HH-1 の org/web#1 の状態を Merged に変更", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("title", "二段階認証", "多要素認証"))
	})
	f.write(t, change.Actor{Kind: change.ActorSync}, "HH-2 をバックログに戻す", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面", "サインイン"))
	})
	f.sync(t, s1, 1, 0)
	f.commits.add(s1, port.Commit{SHA: strings.Repeat("c", 40), Summary: "保持日数を過ぎた削除済み 1 件を完全に削除", Author: change.BotLogin, At: f.clock.Now()})
	return f
}

// assertKindRows は、種類 kind で絞った rows の実行者がどれも actors にあり、texts の行が 1 つずつあることを確かめる。
func assertKindRows(t *testing.T, kind Kind, rows []Row, actors []ActorKind, want []string) {
	t.Helper()
	for _, r := range rows {
		if !slices.Contains(actors, r.Actor) {
			t.Errorf("%s: row %q has actor %q", kind, r.Text, r.Actor)
		}
	}
	for _, text := range want {
		if countText(rows, text) != 1 {
			t.Errorf("%s: rows %q lack %q", kind, texts(rows), text)
		}
	}
}

func TestPageKindFilter(t *testing.T) {
	f := kindFixture(t)
	all := f.page(t, Query{Kind: KindAll}).Rows
	if got := f.page(t, Query{}).Rows; len(got) != len(all) {
		t.Errorf("empty kind = %d rows, want all %d", len(got), len(all))
	}
	cases := []struct {
		kind   Kind
		actors []ActorKind
		texts  []string
	}{
		{KindUser, []ActorKind{ActorUser}, []string{"HH-1 の説明を変更", "HH-2 のステータスを変更"}},
		{KindSync, []ActorKind{ActorSync}, []string{"origin と同期 (送信 1 件・受信 0 件)", "HH-2 をバックログに戻す"}},
		{KindAutomation, []ActorKind{ActorAutomation, ActorGitHub}, []string{"「進行中なら完了」でステータスを Done に変更", "HH-1 の org/web#1 の状態を Merged に変更"}},
	}
	seen := 0
	for _, tc := range cases {
		rows := f.page(t, Query{Kind: tc.kind}).Rows
		seen += len(rows)
		assertKindRows(t, tc.kind, rows, tc.actors, tc.texts)
	}
	none := len(slices.DeleteFunc(slices.Clone(all), func(r Row) bool { return r.Actor != ActorNone }))
	if none != 1 || seen+none != len(all) {
		t.Errorf("none rows = %d, filtered rows = %d, all = %d; want 1 and all = filtered + none", none, seen, len(all))
	}
	if _, err := f.svc.Page(t.Context(), Query{Kind: "bot"}); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("unknown kind: err = %v, want ErrInvalidOperation", err)
	}
}

// assertTexts は、rows の文が want をすべて含み、unwanted をどれも含まないことを確かめる。
func assertTexts(t *testing.T, name string, rows []Row, want, unwanted []string) {
	t.Helper()
	got := texts(rows)
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s rows %q lack %q", name, got, w)
		}
	}
	for _, u := range unwanted {
		if slices.Contains(got, u) {
			t.Errorf("%s rows %q have %q", name, got, u)
		}
	}
}

func TestPageWorkspaceFilter(t *testing.T) {
	f := newPageFixture(t)
	f.setItem(t, "HH-1 のタイトルを変更", apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))
	f.write(t, change.User("taro"), "MB-1 のタイトルを変更", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleMobileEpic, change.NewSet("title", "iOS", "iPhone"))
	})
	f.write(t, change.User("taro"), "2 件のアイテムを変更", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("title", "二段階認証", "認証")),
			tx.WriteItems().Append(apptest.SampleMobileEpic, change.NewSet("title", "iPhone", "iOS")),
		)
	})
	// 同期の修復の行は、同期の行と違い、ワークスペースで絞る。
	f.write(t, change.Actor{Kind: change.ActorSync}, "MB-1 をバックログに戻す", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleMobileEpic, change.NewSet("title", "iOS", "iPad"))
	})
	f.write(t, change.User("taro"), "メンバー jiro を追加", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("jiro")) })
	// ワークスペースも Store 全体の印も持たない行(別の Store へ移した記録など)は「すべて」にだけ出る。
	f.commits.add(s1, port.Commit{SHA: strings.Repeat("d", 40), Summary: "どこにも属さない保存", Author: "taro", At: f.clock.Now()})
	f.sync(t, s1, 1, 1)
	f.sync(t, s2, 3, 0)
	webRows := f.page(t, Query{Store: s1, Workspace: apptest.SampleWorkspace}).Rows
	assertTexts(t, "Web", webRows,
		[]string{"HH-1 のタイトルを変更", "2 件のアイテムを変更", "メンバー jiro を追加", "origin と同期 (送信 1 件・受信 1 件)"},
		[]string{"MB-1 のタイトルを変更", "MB-1 をバックログに戻す", "どこにも属さない保存", "origin と同期 (送信 3 件・受信 0 件)"})
	assertTexts(t, "Mobile", f.page(t, Query{Store: s1, Workspace: apptest.SampleMobileWorkspace}).Rows,
		[]string{"2 件のアイテムを変更", "MB-1 のタイトルを変更", "MB-1 をバックログに戻す"},
		[]string{"HH-1 のタイトルを変更", "どこにも属さない保存"})
	all := f.page(t, Query{}).Rows
	assertTexts(t, "all", all, []string{"どこにも属さない保存", "origin と同期 (送信 3 件・受信 0 件)"}, nil)
	if !slices.ContainsFunc(all, func(r Row) bool { return r.Store.ID == s2 && r.SHA != "" }) {
		t.Fatal("all rows lack the commits of S2")
	}
	store := f.page(t, Query{Store: s1}).Rows
	if slices.ContainsFunc(append(webRows, store...), func(r Row) bool { return r.Store.ID != s1 }) {
		t.Errorf("rows of S1 have rows of another store")
	}
	if _, err := f.svc.Page(t.Context(), Query{Workspace: apptest.SampleWorkspace}); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("workspace without store: err = %v, want ErrInvalidOperation", err)
	}
}

func TestPageHiddenStore(t *testing.T) {
	f := newPageFixture(t)
	f.sync(t, s3, 5, 5)
	f.sync(t, s2, 1, 0)
	rows := f.page(t, Query{}).Rows
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	for _, r := range rows {
		if r.Store.ID == s3 {
			t.Errorf("hidden store row %+v", r)
		}
	}
	if got := f.page(t, Query{Store: s3}); len(got.Rows) != 0 || got.More {
		t.Errorf("rows of the hidden store = %+v", got)
	}
}

// 同期の行の時刻は、続きの印と同じミリ秒にそろえる(ナノ秒の時計で書かれても、印で次のページから抜けない)。
func TestPageSyncRowMillis(t *testing.T) {
	f := newPageFixture(t)
	at := time.Date(2030, 1, 1, 0, 0, 0, 1_500_000, time.UTC) // 1.5ms
	if err := f.syncs.Add(t.Context(), s1, at, 1, 0); err != nil {
		t.Fatal(err)
	}
	f.sync(t, s1, 2, 0)
	rows := f.page(t, Query{Kind: KindSync}).Rows
	if len(rows) != 2 || !rows[0].At.Equal(at.Truncate(time.Millisecond)) {
		t.Fatalf("sync rows = %+v", rows)
	}
	next := f.page(t, Query{Kind: KindSync, Before: cursorOf(rows[0])}).Rows
	if len(next) != 1 || next[0].Key != rows[1].Key {
		t.Errorf("rows after the first = %+v, want the second", next)
	}
}

// newManyRows は、空の表示中の Store S4 に、行のないコミットと同期の行を合わせて n 行作る。2 行ずつ同じ時刻にし、
// 300 行目と 301 行目が同じ時刻になるようにする(区切りが同じ時刻の行のあいだに来る)。
func newManyRows(t *testing.T, n int) (Service, *moreCommits, *apptest.SyncLog, time.Time) {
	t.Helper()
	const s4 port.StoreID = "S4"
	stores := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	stores.AddStore(s4)
	commits, syncs := &moreCommits{}, &apptest.SyncLog{}
	base := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for i := range n {
		at := base.Add(-time.Duration((i+1)/2) * time.Millisecond)
		if i%3 == 0 {
			if err := syncs.Add(t.Context(), s4, at, i+1, 0); err != nil {
				t.Fatal(err)
			}
			continue
		}
		commits.add(s4, port.Commit{SHA: fmt.Sprintf("%040x", i+1), Summary: fmt.Sprintf("保存 %d", i), Author: "taro", At: at})
	}
	info := port.StoreInfo{ID: s4, Name: "many", Color: "sky"}
	return Service{Stores: stores, Commits: commits, Log: syncs, List: apptest.StoreList{Stores: []port.StoreInfo{info}}}, commits, syncs, base
}

func cursorOf(r Row) string { return r.At.UTC().Format("2006-01-02T15:04:05.000Z07:00") + "|" + r.Key }

func TestPageLimitAndMore(t *testing.T) {
	svc, _, _, _ := newManyRows(t, 305)
	first, err := svc.Page(t.Context(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) != 300 || !first.More {
		t.Fatalf("first page = %d rows, more %v; want 300 and true", len(first.Rows), first.More)
	}
	second, err := svc.Page(t.Context(), Query{Before: cursorOf(first.Rows[299])})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Rows) != 5 || second.More {
		t.Fatalf("second page = %d rows, more %v; want 5 and false", len(second.Rows), second.More)
	}
	if !first.Rows[299].At.Equal(second.Rows[0].At) {
		t.Fatalf("the boundary rows %v and %v should share a time", first.Rows[299].At, second.Rows[0].At)
	}
	all := append(slices.Clone(first.Rows), second.Rows...)
	assertOrdered(t, all)
	if _, err := svc.Page(t.Context(), Query{Before: "yesterday"}); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("bad cursor: err = %v, want ErrInvalidOperation", err)
	}
}

// 1 ページ目を読んだあとに新しい保存と同期の行が足されても、2 ページ目は 1 ページ目の続きで、重なりも抜けもない。
func TestPageCursorStable(t *testing.T) {
	svc, commits, syncs, base := newManyRows(t, 305)
	first, err := svc.Page(t.Context(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	commits.add("S4", port.Commit{SHA: strings.Repeat("e", 40), Summary: "新しい保存", Author: "taro", At: base.Add(time.Minute)})
	if err := syncs.Add(t.Context(), "S4", base.Add(time.Minute), 1, 1); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Page(t.Context(), Query{Before: cursorOf(first.Rows[len(first.Rows)-1])})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, r := range append(slices.Clone(first.Rows), second.Rows...) {
		if keys[r.Key] {
			t.Errorf("row %q appears twice", r.Key)
		}
		keys[r.Key] = true
	}
	if len(keys) != 305 || second.More {
		t.Errorf("rows over two pages = %d, more %v; want 305 and false", len(keys), second.More)
	}
	assertOrdered(t, append(slices.Clone(first.Rows), second.Rows...))
}
