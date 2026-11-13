package githublink_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// 削除済みのアイテムのリンクは、読むことも書くこともできない(A4b-4)。
func TestLinksOfADeletedItemAreNotFound(t *testing.T) {
	f := newLinks(t)
	ctx := t.Context()
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, f.stores.Stores, linkStore, linkTask); err != nil {
		t.Fatal(err)
	}
	before, asked := len(f.stores.ops), f.gh.Lookups
	_, listErr := f.links.List(ctx, linkStore, linkTask)
	_, refreshErr := f.links.Refresh(ctx, linkStore, linkTask)
	addErr := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false)
	removeErr := f.links.Remove(ctx, linkStore, linkTask, itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41})
	for name, err := range map[string]error{"List": listErr, "Refresh": refreshErr, "Add": addErr, "Remove": removeErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s of a deleted item = %v, want port.ErrNotFound", name, err)
		}
	}
	if len(f.stores.ops) != before || f.gh.Lookups != asked {
		t.Errorf("wrote %d times, asked GitHub %d times", len(f.stores.ops)-before, f.gh.Lookups-asked)
	}
	// 削除していないアイテムには今までどおりリンクできる。
	if err := f.add(t, linkStore, linkEpic, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Errorf("Add to a live item = %v", err)
	}
}

// プロジェクトの「PR と Issue」には、削除済みのアイテムのリンクを出さない。
func TestProjectLinksSkipDeletedItems(t *testing.T) {
	f := newLinks(t)
	ctx := t.Context()
	f.gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 42, Values: itemlink.Values{Title: "b", State: itemlink.Open, Review: itemlink.NoReview}})
	if err := f.add(t, linkStore, linkEpic, itemlink.PR, "org/web", "42", false); err != nil {
		t.Fatal(err)
	}
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, f.stores.Stores, linkStore, linkTask); err != nil {
		t.Fatal(err)
	}
	rows, err := f.links.ProjectLinks(ctx, linkStore, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Display+" "+r.Link.Display())
	}
	if want := []string{"HH-1 org/web#42"}; !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

// 削除済みのプロジェクトの「PR と Issue」と、その下のアイテムのリンクは、読むことも書くこともできない。
// リポジトリを削除済みのプロジェクトに足すこともない。
func TestLinksOfADeletedProjectAreNotFound(t *testing.T) {
	f := newLinks(t)
	ctx := t.Context()
	if err := apptest.DeleteProject(ctx, f.stores.Stores, linkStore, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	before := len(f.stores.ops)
	_, projectErr := f.links.ProjectLinks(ctx, linkStore, apptest.SampleProject)
	_, listErr := f.links.List(ctx, linkStore, linkTask)
	addErr := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", true)
	for name, err := range map[string]error{"ProjectLinks": projectErr, "List": listErr, "Add": addErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s under a deleted project = %v, want port.ErrNotFound", name, err)
		}
	}
	if len(f.stores.ops) != before {
		t.Errorf("wrote %d times", len(f.stores.ops)-before)
	}
}

// 間隔ごとの取り直しは、削除済みのアイテムのリンクを GitHub に頼まず、書きもしない(D4a-10)。
func TestReadOnceSkipsDeletedItems(t *testing.T) {
	f := newRead(t)
	ctx := t.Context()
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkEpic, readPR("org/web", 2, itemlink.Open))
	f.linkIn(t, linkStore, linkEpic, readPR("org/web", 3, itemlink.Open))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 3, itemlink.Open))
	for _, n := range []int{1, 2, 3} {
		f.gh.Put("org/web", readPR("org/web", n, itemlink.Merged))
	}
	// 偽の GitHub は頼まれた番号を覚えていないので、#1 を頼めば失敗になるようにして、頼んでいないことを確かめる
	f.gh.Fail("org/web", 1, errors.New("boom"))
	if err := apptest.DeleteItems(ctx, f.stores.Stores, linkStore, linkTask); err != nil {
		t.Fatal(err)
	}
	before := len(f.taskLog(t))
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 1 {
		t.Errorf("calls = %d, want 1", n)
	}
	rows, err := f.links.List(ctx, linkStore, linkEpic)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2", rows)
	}
	for _, r := range rows {
		if r.State != itemlink.Merged {
			t.Errorf("#%d state = %v, want Merged", r.Number, r.State)
		}
	}
	if after := len(f.taskLog(t)); after != before {
		t.Errorf("deleted item's log grew by %d", after-before)
	}
	if got := f.reads.failures[linkStore]; len(got) != 0 {
		t.Errorf("failures = %v, want none", got)
	}
}

// 間隔ごとの取り直しは、削除済みのプロジェクトの下のアイテムのリンクを頼まない(D4a-10)。
func TestReadOnceSkipsDeletedProject(t *testing.T) {
	f := newRead(t)
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Merged))
	if err := apptest.DeleteProject(t.Context(), f.stores.Stores, linkStore, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	f.read(t, every, false)
	if n := f.gh.ReadManyCalls("org/web"); n != 0 {
		t.Errorf("calls = %d, want none", n)
	}
	if got := f.reads.failures[linkStore]; len(got) != 0 {
		t.Errorf("failures = %v, want none", got)
	}
}

// 読んだあと書くまでのあいだに削除されたアイテムのリンクは、何も書かずに飛ばし、残りのリンクは書く。
func TestReadOnceItemDeletedMeanwhile(t *testing.T) {
	f := newRead(t)
	ctx := t.Context()
	f.withRules(t, statusRule(ruleMerged, "マージで完了", automation.TriggerPRMerged, apptest.SampleStatusDone))
	f.linkIn(t, linkStore, linkTask, readPR("org/web", 1, itemlink.Open))
	f.linkIn(t, linkStore, linkEpic, readPR("org/web", 2, itemlink.Open))
	f.gh.Put("org/web", readPR("org/web", 1, itemlink.Merged))
	f.gh.Put("org/web", readPR("org/web", 2, itemlink.Merged))
	var before int
	// GitHub から読んだあと、最初の書き込みの前に HH-2 が削除される
	f.stores.before = func() {
		if err := apptest.DeleteItems(ctx, f.stores.Stores, linkStore, linkTask); err != nil {
			t.Error(err)
		}
		before = len(f.taskLog(t))
	}
	f.read(t, every, false)
	rows, err := f.links.List(ctx, linkStore, linkEpic)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].State != itemlink.Merged {
		t.Errorf("rows = %+v, want #2 Merged", rows)
	}
	if after := len(f.taskLog(t)); after != before {
		t.Errorf("deleted item's log grew by %d", after-before)
	}
	if got := f.reads.failures[linkStore]; len(got) != 0 {
		t.Errorf("failures = %v, want none", got)
	}
	if ns := f.inbox.Notices(); len(ns) != 1 {
		t.Errorf("notices = %+v, want 1", ns)
	}
}
