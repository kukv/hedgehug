package githublink_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
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
