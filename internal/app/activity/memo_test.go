package activity

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// countingStores は、Read の回数を数える port.Stores。
type countingStores struct {
	port.Stores
	reads atomic.Int32
}

func (c *countingStores) Read(ctx context.Context, id port.StoreID, fn func(port.Reader) error) error {
	c.reads.Add(1)
	return c.Stores.Read(ctx, id, fn)
}

// countingCommits は、Commits の回数を数える port.StoreCommits。
type countingCommits struct {
	port.StoreCommits
	calls atomic.Int32
}

func (c *countingCommits) Commits(ctx context.Context, id port.StoreID) ([]port.Commit, error) {
	c.calls.Add(1)
	return c.StoreCommits.Commits(ctx, id)
}

// 版の印が変わらなければ、2 回目の Page は Store を読まず、コミットも辿らない。同期の行は毎回読む。保存したあと
// (版が変わる)は読み直して新しい行を出す。Store 名と色は覚えた行に入れず、いまの一覧の値を出す。
func TestPageReusesRowsWhileVersionHolds(t *testing.T) {
	f := newPageFixture(t)
	stores := &countingStores{Stores: f.log}
	commits := &countingCommits{StoreCommits: f.commits}
	svc := f.svc
	svc.Stores, svc.Commits, svc.Versions, svc.Memo = stores, commits, f.stores, &Memo{}
	page := func() Page {
		t.Helper()
		p, err := svc.Page(t.Context(), Query{Store: s1})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := page()
	reads, calls := stores.reads.Load(), commits.calls.Load()
	f.sync(t, s1, 4, 5)
	second := page()
	if stores.reads.Load() != reads || commits.calls.Load() != calls {
		t.Errorf("reads %d→%d, commits %d→%d; want no reads while the version holds", reads, stores.reads.Load(), calls, commits.calls.Load())
	}
	if len(second.Rows) != len(first.Rows)+1 || countText(second.Rows, "origin と同期 (送信 4 件・受信 5 件)") != 1 {
		t.Errorf("second page = %q, want the first rows and the new sync row", texts(second.Rows))
	}
	renamed := s1Info
	renamed.Name, renamed.Color = "renamed", "rose"
	svc.List = apptest.StoreList{Stores: []port.StoreInfo{renamed}}
	for _, r := range page().Rows {
		if r.Store != renamed {
			t.Fatalf("row %q store = %+v, want the current %+v", r.Text, r.Store, renamed)
		}
	}
	f.setItem(t, "HH-1 のタイトルを変更", apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))
	if got := page(); countText(got.Rows, "HH-1 のタイトルを変更") != 1 || stores.reads.Load() == reads {
		t.Errorf("after a save: rows %q, reads %d; want the new row read again", texts(got.Rows), stores.reads.Load())
	}
}

// 表示中の一覧から外した Store の覚えた行は、次の Page で捨てる(Store を絞った要求でも)。
func TestPageForgetsHiddenStores(t *testing.T) {
	f := newPageFixture(t)
	svc := f.svc
	svc.Versions, svc.Memo = f.stores, &Memo{}
	if _, err := svc.Page(t.Context(), Query{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.Memo.byStore[s2]; !ok {
		t.Fatalf("memo = %v, want S2 remembered", svc.Memo.byStore)
	}
	svc.List = apptest.StoreList{Stores: []port.StoreInfo{s1Info}}
	if _, err := svc.Page(t.Context(), Query{Store: s1}); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.Memo.byStore[s2]; ok || len(svc.Memo.byStore) != 1 {
		t.Errorf("memo keeps %v, want only S1", svc.Memo.byStore)
	}
}

// コミットの行の時刻は、続きの印と同じミリ秒にそろえる(秒以下を持つコミットが、印で次のページから抜けない)。
func TestPageCommitRowMillis(t *testing.T) {
	svc, commits, _, base := newManyRows(t, 0)
	commits.add("S4", port.Commit{SHA: strings.Repeat("b", 40), Summary: "あと", Author: "taro", At: base.Add(1_500 * time.Microsecond)})
	commits.add("S4", port.Commit{SHA: strings.Repeat("a", 40), Summary: "さき", Author: "taro", At: base.Add(1_200 * time.Microsecond)})
	first, err := svc.Page(t.Context(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) != 2 || !first.Rows[0].At.Equal(base.Add(time.Millisecond)) {
		t.Fatalf("rows = %+v", first.Rows)
	}
	next, err := svc.Page(t.Context(), Query{Before: Cursor(first.Rows[0])})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Rows) != 1 || next.Rows[0].Key != first.Rows[1].Key {
		t.Errorf("rows after the first = %+v, want the second", next.Rows)
	}
}
