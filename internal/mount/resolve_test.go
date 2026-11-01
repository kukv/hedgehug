package mount

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// twelve is the 12th task of a store with the prefix API: API-12.
func twelve(t *testing.T, e config.StoreEntry) *model.Task {
	t.Helper()
	tasks := putTasks(t, e, 12, nil)
	return tasks[11]
}

func TestResolveOneStore(t *testing.T) {
	a, b := newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "WEB")
	task := twelve(t, a)
	putTasks(t, b, 1, nil)
	set := openSet(t, newStoreAt(t, "me", "ME"), a, b)
	for _, in := range []string{"API-12", "api-12", task.ULID.String(), "org-a:API-12"} {
		hit, err := set.Resolve(in)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", in, err)
		}
		if hit.Store.Name != "org-a" || hit.Entry.ULID != task.ULID {
			t.Errorf("Resolve(%q) = %s %v", in, hit.Store.Name, hit.Entry.ULID)
		}
	}
}

func TestResolveSeesATaskWrittenAfterTheIndexWasOpened(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	set := openSet(t, a)
	if _, err := set.Resolve("API-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Resolve before the write = %v; want not found", err)
	}
	putTasks(t, a, 1, nil)
	if hit, err := set.Resolve("API-1"); err != nil || hit.Entry.ID != "API-1" {
		t.Errorf("Resolve after the write = %v, %v", hit, err)
	}
}

func TestResolveNotFound(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	twelve(t, a)
	set := openSet(t, a, newStoreAt(t, "org-b", "WEB"))
	for _, in := range []string{"API-99", "org-b:API-12", "nope:API-12", ""} {
		if _, err := set.Resolve(in); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Resolve(%q) err = %v, want ErrNotFound", in, err)
		}
	}
}

func TestResolveAcrossStoresIsAmbiguous(t *testing.T) {
	a, b := newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "API")
	ta, tb := twelve(t, a), twelve(t, b)
	set := openSet(t, a, b)
	for _, in := range []string{"API-12", "api-12"} {
		_, err := set.Resolve(in)
		var ae *AmbiguousError
		if !errors.As(err, &ae) {
			t.Fatalf("Resolve(%q) err = %v, want AmbiguousError", in, err)
		}
		if len(ae.Candidates) != 2 || ae.Candidates[0].Store.Name != "org-a" || ae.Candidates[1].Store.Name != "org-b" ||
			ae.Candidates[0].Entry.ULID != ta.ULID || ae.Candidates[1].Entry.ULID != tb.ULID {
			t.Errorf("candidates = %+v", ae.Candidates)
		}
		if !strings.Contains(err.Error(), "org-a:API-12") || !strings.Contains(err.Error(), "org-b:API-12") {
			t.Errorf("message = %q", err)
		}
	}
	hit, err := set.Resolve("org-b:API-12")
	if err != nil || hit.Store.Name != "org-b" || hit.Entry.ULID != tb.ULID {
		t.Errorf("org-b:API-12 = %v %v", hit, err)
	}
}

func TestResolveULIDAcrossStores(t *testing.T) {
	a, b := newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "WEB")
	ta := putTasks(t, a, 1, nil)[0]
	tb := putTasks(t, b, 1, nil)[0]
	set := openSet(t, a, b)
	if hit, err := set.Resolve(tb.ULID.String()); err != nil || hit.Store.Name != "org-b" {
		t.Errorf("ULID of org-b = %v %v", hit, err)
	}
	if hit, err := set.Resolve("org-a:" + ta.ULID.String()); err != nil || hit.Store.Name != "org-a" {
		t.Errorf("org-a:ULID = %v %v", hit, err)
	}
}

func TestAmbiguousCandidateWithoutIDUsesULID(t *testing.T) {
	task := model.NewULID()
	ae := &AmbiguousError{Input: "x", Candidates: []Hit{{Store: &Mounted{Name: "s"}, Entry: &index.Entry{ULID: task}}}}
	if !strings.Contains(ae.Error(), "s:"+task.String()) {
		t.Errorf("message = %q", ae)
	}
}

func TestResolveAmbiguousInsideOneStore(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	putTasks(t, a, 2, func(_ int, task *model.Task) { task.FormerIDs = []string{"OLD-3"} })
	// one store alone: the index says it
	alone := openSet(t, a)
	_, err := alone.Resolve("OLD-3")
	var ie *index.AmbiguousError
	if !errors.As(err, &ie) || len(ie.Candidates) != 2 {
		t.Errorf("one store: err = %v, want index.AmbiguousError", err)
	}
	// with two stores the candidates carry the name of the store
	set := openSet(t, a, newStoreAt(t, "org-b", "WEB"))
	for _, in := range []string{"OLD-3", "org-a:OLD-3"} {
		_, err := set.Resolve(in)
		var ae *AmbiguousError
		if !errors.As(err, &ae) || len(ae.Candidates) != 2 {
			t.Fatalf("Resolve(%q) err = %v, want AmbiguousError", in, err)
		}
		if !strings.Contains(err.Error(), "org-a:API-1") || !strings.Contains(err.Error(), "org-a:API-2") || !strings.Contains(err.Error(), "in 1 store:") {
			t.Errorf("message = %q", err)
		}
	}
}

// An input that is ambiguous in one store and found in another is one
// AmbiguousError with all the candidates, whichever store comes first.
func TestResolveAmbiguousInOneStoreAndFoundInAnother(t *testing.T) {
	for _, order := range [][2]string{{"org-a", "org-b"}, {"org-b", "org-a"}} {
		two := newStoreAt(t, "org-a", "API")
		one := newStoreAt(t, "org-b", "WEB")
		putTasks(t, two, 2, func(_ int, task *model.Task) { task.FormerIDs = []string{"OLD-3"} })
		putTasks(t, one, 1, func(_ int, task *model.Task) { task.FormerIDs = []string{"OLD-3"} })
		entries := map[string]config.StoreEntry{"org-a": two, "org-b": one}
		set := openSet(t, entries[order[0]], entries[order[1]])
		_, err := set.Resolve("OLD-3")
		var ae *AmbiguousError
		if !errors.As(err, &ae) || len(ae.Candidates) != 3 {
			t.Fatalf("order %v: err = %v, want AmbiguousError of 3", order, err)
		}
		for _, want := range []string{"org-a:API-1", "org-a:API-2", "org-b:WEB-1", "in 2 stores"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("order %v: message %q lacks %s", order, err, want)
			}
		}
	}
}

// breakIndex makes the index of m fail to open: the repository goes away
// after the store was opened.
func breakIndex(t *testing.T, m *Mounted) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(m.Entry.Path, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Index(); err == nil {
		t.Fatal("the index of the broken store opens")
	}
}

func TestResolveSkipsAStoreWhoseIndexDoesNotOpen(t *testing.T) {
	a, b := newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "WEB")
	tb := putTasks(t, b, 1, nil)[0]
	var told []Skipped
	set := Open(&config.Config{Stores: []config.StoreEntry{a, b}}, Options{Unopened: func(sk Skipped) { told = append(told, sk) }})
	breakIndex(t, set.Stores[0])
	for range 2 {
		hit, err := set.Resolve(tb.ULID.String())
		if err != nil || hit.Store.Name != "org-b" {
			t.Fatalf("Resolve = %v %v", hit, err)
		}
	}
	// every time: how often to say it is for the caller to decide
	if len(told) != 2 || told[0].Name != "org-a" || told[0].Err == nil {
		t.Errorf("told of %+v", told)
	}
}

func TestResolveNamedStoreWhoseIndexDoesNotOpenIsAnError(t *testing.T) {
	a, b := newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "WEB")
	putTasks(t, a, 1, nil)
	var told []Skipped
	set := Open(&config.Config{Stores: []config.StoreEntry{a, b}}, Options{Unopened: func(sk Skipped) { told = append(told, sk) }})
	breakIndex(t, set.Stores[0])
	_, err := set.Resolve("org-a:API-1")
	if err == nil || !strings.HasPrefix(err.Error(), "cannot open org-a: ") || errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	if len(told) != 0 {
		t.Errorf("told of %+v", told)
	}
}

func TestResolveAfterAnotherHandleRepackedTheStore(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	putTasks(t, a, 1, nil)
	set := openSet(t, a)
	if _, err := set.Resolve("API-1"); err != nil {
		t.Fatal(err)
	}
	// another handle writes, repacks (the loose objects the first handle
	// knew are gone) and writes again
	putTasks(t, a, 1, nil)
	s, err := store.Open(a.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().Repack(); err != nil {
		t.Fatal(err)
	}
	putTasks(t, a, 1, nil)
	if hit, err := set.Resolve("API-3"); err != nil || hit.Entry.ID != "API-3" {
		t.Errorf("Resolve(API-3) = %v, %v", hit, err)
	}
}

// brokenMounted is a Mounted on the store a whose store and index are those
// of another store b, which is then removed: its Refresh fails.
func brokenMounted(t *testing.T, a config.StoreEntry) *Mounted {
	t.Helper()
	b := newStoreAt(t, "org-b", "WEB")
	sb, err := store.Open(b.Path)
	if err != nil {
		t.Fatal(err)
	}
	m := &Mounted{Name: a.Name, Entry: a, Store: sb}
	if _, err := m.Index(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(b.Path); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWithOpensTheStoreAgainWhenTheIndexCannotRefresh(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	m := brokenMounted(t, a)
	old := m.Store
	ran := false
	if err := m.With(func(ix *index.Index) {
		ran = true
		if len(ix.Workspaces()) != 1 || ix.Workspaces()[0].IDPrefix != "API" {
			t.Errorf("the index is not that of org-a: %v", ix.Workspaces())
		}
	}); err != nil || !ran {
		t.Fatalf("With = %v, ran %v", err, ran)
	}
	if m.Store == old {
		t.Error("the store was not opened again")
	}
}

func TestWithFailsWhenTheStoreCannotBeOpenedAgain(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	m := brokenMounted(t, a)
	if err := os.RemoveAll(a.Path); err != nil {
		t.Fatal(err)
	}
	ran := false
	if err := m.With(func(*index.Index) { ran = true }); err == nil || ran {
		t.Errorf("With = %v, ran %v; want an error and no run", err, ran)
	}
}
