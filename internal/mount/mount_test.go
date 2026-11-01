package mount

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestOpenKeepsConfigOrderAndULIDs(t *testing.T) {
	es := []config.StoreEntry{newStoreAt(t, "me", "ME"), newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "WEB")}
	set := openSet(t, es...)
	if len(set.Stores) != 3 || len(set.Skipped) != 0 {
		t.Fatalf("stores %d, skipped %v", len(set.Stores), set.Skipped)
	}
	for i, m := range set.Stores {
		if m.Name != es[i].Name || m.Entry.Path != es[i].Path {
			t.Errorf("stores[%d] = %s %s", i, m.Name, m.Entry.Path)
		}
		s, err := store.Open(es[i].Path)
		if err != nil {
			t.Fatal(err)
		}
		if m.ULID != s.Meta().ULID {
			t.Errorf("%s: ulid %v, want %v", m.Name, m.ULID, s.Meta().ULID)
		}
	}
	if m, ok := set.Get("org-a"); !ok || m != set.Stores[1] {
		t.Errorf("Get(org-a) = %v %v", m, ok)
	}
	if _, ok := set.Get("Org-A"); ok {
		t.Error("Get ignores case")
	}
}

func TestOpenSkipsWhatDoesNotOpen(t *testing.T) {
	good := newStoreAt(t, "me", "ME")
	missing := config.StoreEntry{Name: "gone", Path: filepath.Join(t.TempDir(), "nope")}
	broken := newStoreAt(t, "broken", "BRK")
	if err := os.WriteFile(filepath.Join(broken.Path, ".hedgehug", "store.yaml"), []byte(": : not yaml ["), 0o600); err != nil {
		t.Fatal(err)
	}
	last := newStoreAt(t, "org-b", "WEB")
	set := openSet(t, good, missing, broken, last)
	if len(set.Stores) != 2 || set.Stores[0].Name != "me" || set.Stores[1].Name != "org-b" {
		t.Errorf("stores = %v", set.Stores)
	}
	if len(set.Skipped) != 2 || set.Skipped[0].Name != "gone" || set.Skipped[1].Name != "broken" {
		t.Fatalf("skipped = %v", set.Skipped)
	}
	for _, sk := range set.Skipped {
		if sk.Err == nil {
			t.Errorf("%s: no reason", sk.Name)
		}
	}
	if _, ok := set.Get("gone"); ok {
		t.Error("a skipped store is mounted")
	}
}

func TestOpenGivesTheClockToTheStore(t *testing.T) {
	e := newStoreAt(t, "me", "ME")
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	set := Open(&config.Config{Stores: []config.StoreEntry{e}}, Options{Now: func() time.Time { return at }})
	m := set.Stores[0]
	ix, err := m.Index()
	if err != nil {
		t.Fatal(err)
	}
	task := &model.Task{ULID: model.NewULID(), Title: "t", Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID}
	if _, err := m.Store.Update(store.Op{Name: "add", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
		t.Fatal(err)
	}
	if !task.Created.Equal(at) {
		t.Errorf("created %v, want %v", task.Created, at)
	}
}

func TestIndexIsOpenedOnceAndKnowsTheStoreName(t *testing.T) {
	m := openSet(t, newStoreAt(t, "org-a", "API")).Stores[0]
	a, err := m.Index()
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Index()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("Index opened twice")
	}
	if a.StoreName() != "org-a" {
		t.Errorf("StoreName = %q", a.StoreName())
	}
	putTasks(t, m.Entry, 1, nil)
	if err := a.Refresh(); err != nil || a.StoreName() != "org-a" {
		t.Errorf("after Refresh: %q %v", a.StoreName(), err)
	}
}

func TestSplitStore(t *testing.T) {
	set := openSet(t, newStoreAt(t, "org-a", "API"))
	m, rest, ok := set.SplitStore("org-a:API-12")
	if !ok || m.Name != "org-a" || rest != "API-12" {
		t.Errorf("SplitStore = %v %q %v", m, rest, ok)
	}
	for _, in := range []string{"nope:API-12", "Org-A:API-12", "API-12", ""} {
		if _, _, ok := set.SplitStore(in); ok {
			t.Errorf("SplitStore(%q) ok", in)
		}
	}
}

func TestBrokenAccessIsNotSkipped(t *testing.T) {
	e := newStoreAt(t, "me", "ME")
	dir := filepath.Join(e.Path, filepath.FromSlash(store.LocalDir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "access.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	set := openSet(t, e)
	if len(set.Skipped) != 0 || len(set.Stores) != 1 {
		t.Fatalf("stores %d, skipped %v", len(set.Stores), set.Skipped)
	}
}
