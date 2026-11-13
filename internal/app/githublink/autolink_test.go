package githublink_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/prlink"
)

const autoRepo = "example-org/web"

// newAutoLinks は、サンプルのワークスペースの接頭辞を WEB にした組み立て(HH-1 は WEB-1、HH-2 は WEB-2)。
func newAutoLinks(t *testing.T) *linkFixture {
	t.Helper()
	f := newLinks(t)
	f.writeAs(t, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "WEB"))
	})
	return f
}

// writeAs は、記録を残さずに(linkRecorder を通さずに)テストの準備を保存する。
func (f *linkFixture) writeAs(t *testing.T, fn func(tx port.Tx) error) {
	t.Helper()
	if err := f.stores.Stores.Write(t.Context(), linkStore, port.Operation{Actor: change.User("taro"), Summary: "準備"}, fn); err != nil {
		t.Fatal(err)
	}
}

// autoPull は、example-org/web の番号 n の PR(状態 s、レビュー待ち、チェック実行中)。x はブランチ名・タイトルなど。
func autoPull(n int, s itemlink.State, x prlink.Text) port.Pull {
	return port.Pull{Number: n, Text: x, Values: itemlink.Values{Title: "Add login form", State: s, Review: itemlink.Pending}, Checks: itemlink.Running}
}

func (f *linkFixture) autoLink(t *testing.T, pull port.Pull, first bool, p *githublink.Pass) bool {
	t.Helper()
	ok, err := f.links.AutoLink(t.Context(), linkStore, autoRepo, pull, first, p)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func (f *linkFixture) epicLog(t *testing.T) []change.Record {
	t.Helper()
	var log []change.Record
	err := f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: linkEpic})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func (f *linkFixture) epicItem(t *testing.T) item.Item {
	t.Helper()
	var it item.Item
	err := f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(linkEpic)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return it
}

// setStatus は、id のステータスを from から to に変える(準備)。
func (f *linkFixture) setStatus(t *testing.T, id item.ID, from, to change.ID) {
	t.Helper()
	f.writeAs(t, func(tx port.Tx) error { return tx.WriteItems().Append(id, change.NewSet("status", from, to)) })
}

func TestAutoLinkAdds(t *testing.T) {
	f := newAutoLinks(t)
	if !f.autoLink(t, autoPull(5, itemlink.Open, prlink.Text{Branch: "hh/WEB-1"}), false, nil) {
		t.Fatal("autoLink = false, want a link")
	}
	rows := f.list(t, linkStore, linkEpic)
	if got := displays(rows); !slices.Equal(got, []string{"pr example-org/web#5 Add login form open"}) {
		t.Fatalf("rows = %q", got)
	}
	if rows[0].Review != itemlink.Pending || rows[0].Checks != itemlink.Running {
		t.Errorf("row = %+v", rows[0])
	}
	op := f.stores.ops[len(f.stores.ops)-1]
	if op.Actor != (change.Actor{Kind: change.ActorGitHub}) || op.Summary != "WEB-1 に example-org/web#5 を自動でリンク" {
		t.Errorf("op = %+v", op)
	}
}

func TestAutoLinkSkipsClosed(t *testing.T) {
	f := newAutoLinks(t)
	writes := len(f.stores.ops)
	if f.autoLink(t, autoPull(5, itemlink.Closed, prlink.Text{Branch: "hh/WEB-1"}), false, nil) {
		t.Error("autoLink = true for a closed PR")
	}
	if n := len(f.stores.ops) - writes; n != 0 {
		t.Errorf("writes = %d", n)
	}
	if rows := f.list(t, linkStore, linkEpic); len(rows) != 0 {
		t.Errorf("rows = %q", displays(rows))
	}
}

func TestAutoLinkSkipsExisting(t *testing.T) {
	f := newAutoLinks(t)
	issue := itemlink.Ref{Kind: itemlink.Issue, Repo: "Example-Org/Web", Number: 5}
	f.attachNow(t, linkEpic, issue, itemlink.Values{Title: "x", State: itemlink.Open, Review: itemlink.NoReview})
	if f.autoLink(t, autoPull(5, itemlink.Open, prlink.Text{Branch: "hh/WEB-1"}), false, nil) {
		t.Error("autoLink = true over a link to the same number")
	}
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"issue Example-Org/Web#5 x open"}) {
		t.Errorf("rows = %q", got)
	}
}

func TestAutoLinkSkipsUnlinked(t *testing.T) {
	f := newAutoLinks(t)
	pull := autoPull(5, itemlink.Open, prlink.Text{Branch: "hh/WEB-1"})
	if !f.autoLink(t, pull, false, nil) {
		t.Fatal("first autoLink = false")
	}
	ref := itemlink.Ref{Kind: itemlink.PR, Repo: autoRepo, Number: 5}
	if err := f.links.Remove(t.Context(), linkStore, linkEpic, ref); err != nil {
		t.Fatal(err)
	}
	if f.autoLink(t, pull, false, nil) {
		t.Error("autoLink = true after the user unlinked it")
	}
	if rows := f.list(t, linkStore, linkEpic); len(rows) != 0 {
		t.Errorf("rows after unlink = %q", displays(rows))
	}
	// 手で「PR をリンク」し直したあとは、自動のリンクを重ねない
	f.gh.Put(autoRepo, port.Found{Kind: itemlink.PR, Repo: autoRepo, Number: 5, Values: pull.Values, Checks: itemlink.Running})
	if err := f.add(t, linkStore, linkEpic, itemlink.PR, autoRepo, "5", false); err != nil {
		t.Fatal(err)
	}
	if f.autoLink(t, pull, false, nil) {
		t.Error("autoLink = true over the manual link")
	}
	if got := displays(f.list(t, linkStore, linkEpic)); !slices.Equal(got, []string{"pr example-org/web#5 Add login form open"}) {
		t.Errorf("rows = %q", got)
	}
}

func TestAutoLinkFiresAfterFirst(t *testing.T) {
	rule := statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone)
	merged := autoPull(5, itemlink.Merged, prlink.Text{Branch: "hh/WEB-1"})

	f := newAutoLinks(t)
	f.withRules(t, rule)
	if !f.autoLink(t, merged, true, nil) {
		t.Fatal("first scan: autoLink = false")
	}
	if it := f.epicItem(t); it.Status != apptest.SampleStatusTodo {
		t.Errorf("first scan: status = %s, want unchanged", it.Status)
	}
	if rows := ruleRows(f.epicLog(t)); len(rows) != 0 || len(f.inbox.Notices()) != 0 {
		t.Errorf("first scan: rule rows = %+v, notices = %+v", rows, f.inbox.Notices())
	}

	f = newAutoLinks(t)
	f.withRules(t, rule)
	writes := len(f.stores.ops)
	if !f.autoLink(t, merged, false, nil) {
		t.Fatal("later scan: autoLink = false")
	}
	if n := len(f.stores.ops) - writes; n != 1 {
		t.Errorf("writes = %d, want the rule inside the same save", n)
	}
	if it := f.epicItem(t); it.Status != apptest.SampleStatusDone {
		t.Errorf("later scan: status = %s", it.Status)
	}
	rows := ruleRows(f.epicLog(t))
	if len(rows) != 1 || rows[0].Actor.Rule != ruleMerged || rows[0].By != change.BotLogin {
		t.Errorf("rule rows = %+v", rows)
	}
	if ns := f.inbox.Notices(); len(ns) != 1 || ns[0].Item != linkEpic || ns[0].Text != "「マージで完了」でステータスを Done に変更しました" {
		t.Errorf("notices = %+v", ns)
	}
}

func TestAutoLinkOncePerPass(t *testing.T) {
	f := newAutoLinks(t)
	f.withRules(t, statusRule(ruleOpen, "PR が Open で In Progress", automation.TriggerPROpen, apptest.SampleStatusInProgress))
	p := githublink.NewPass()
	f.autoLink(t, autoPull(5, itemlink.Open, prlink.Text{Branch: "hh/WEB-1"}), false, p)
	f.setStatus(t, linkEpic, apptest.SampleStatusInProgress, apptest.SampleStatusTodo)
	f.autoLink(t, autoPull(6, itemlink.Open, prlink.Text{Title: "WEB-1"}), false, p)
	if it := f.epicItem(t); it.Status != apptest.SampleStatusTodo {
		t.Errorf("same pass: status = %s, want the rule once", it.Status)
	}
	if rows := ruleRows(f.epicLog(t)); len(rows) != 1 {
		t.Errorf("same pass: rule rows = %+v", rows)
	}
	// 別の読み取りなら、もう一度動く
	f.autoLink(t, autoPull(7, itemlink.Open, prlink.Text{Title: "WEB-1"}), false, githublink.NewPass())
	if it := f.epicItem(t); it.Status != apptest.SampleStatusInProgress {
		t.Errorf("next pass: status = %s", it.Status)
	}
}

func TestAutoLinkManyItems(t *testing.T) {
	f := newAutoLinks(t)
	writes := len(f.stores.ops)
	if !f.autoLink(t, autoPull(5, itemlink.Open, prlink.Text{Title: "WEB-1 と WEB-2"}), false, nil) {
		t.Fatal("autoLink = false")
	}
	if n := len(f.stores.ops) - writes; n != 2 {
		t.Errorf("writes = %d, want one save per item", n)
	}
	sums := []string{f.stores.ops[writes].Summary, f.stores.ops[writes+1].Summary}
	if !slices.Equal(sums, []string{"WEB-1 に example-org/web#5 を自動でリンク", "WEB-2 に example-org/web#5 を自動でリンク"}) {
		t.Errorf("summaries = %q", sums)
	}
	for _, id := range []item.ID{linkEpic, linkTask} {
		if got := displays(f.list(t, linkStore, id)); !slices.Equal(got, []string{"pr example-org/web#5 Add login form open"}) {
			t.Errorf("%s: rows = %q", id, got)
		}
	}
}

// 2 つ目のアイテムの保存に失敗しても、1 つ目に書いたリンクのチェックの結果は置く(D4b 最終レビュー M-2)。
func TestAutoLinkSavesChecksWhenALaterSaveFails(t *testing.T) {
	f := newAutoLinks(t)
	n := 0
	f.stores.failOn = func(port.Operation) error {
		if n++; n == 2 {
			return errors.New("保存できない")
		}
		return nil
	}
	ok, err := f.links.AutoLink(t.Context(), linkStore, autoRepo, autoPull(5, itemlink.Open, prlink.Text{Title: "WEB-1 と WEB-2"}), false, nil)
	if !ok || err == nil {
		t.Fatalf("AutoLink = %v, %v; want linked and the error", ok, err)
	}
	got, err := f.checks.Read(t.Context(), linkStore)
	if err != nil {
		t.Fatal(err)
	}
	if key := (itemlink.Ref{Kind: itemlink.PR, Repo: autoRepo, Number: 5}).Key(); got[key] != itemlink.Running {
		t.Errorf("checks = %v, want %s running", got, key)
	}
}

func TestAutoLinkDeletedMeanwhile(t *testing.T) {
	f := newAutoLinks(t)
	f.stores.before = func() {
		if err := apptest.DeleteItems(t.Context(), f.stores.Stores, linkStore, linkEpic); err != nil {
			t.Fatal(err)
		}
	}
	if f.autoLink(t, autoPull(5, itemlink.Open, prlink.Text{Branch: "hh/WEB-1"}), false, nil) {
		t.Error("autoLink = true for an item deleted before the save")
	}
	for _, r := range f.epicLog(t) {
		if r.Actor.Kind == change.ActorGitHub {
			t.Errorf("wrote %+v to a deleted item", r)
		}
	}
	if c, err := f.checks.Read(t.Context(), linkStore); err != nil || len(c) != 0 {
		t.Errorf("checks = %v, %v", c, err)
	}
}

func TestAutoLinkNoWatchNotice(t *testing.T) {
	f := newAutoLinks(t)
	f.writeAs(t, func(tx port.Tx) error { return tx.WriteItems().Append(linkEpic, change.NewAdd("watchers", "hanako")) })
	if !f.autoLink(t, autoPull(5, itemlink.Open, prlink.Text{Branch: "hh/WEB-1"}), false, nil) {
		t.Fatal("autoLink = false")
	}
	if ns := f.inbox.Notices(); len(ns) != 0 {
		t.Errorf("notices = %+v, want no watch notice", ns)
	}
}
