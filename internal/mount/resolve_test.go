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
	set := openSet(t, a, newStoreAt(t, "org-b", "WEB"))
	for _, in := range []string{"OLD-3", "org-a:OLD-3"} {
		_, err := set.Resolve(in)
		var ie *index.AmbiguousError
		if !errors.As(err, &ie) || len(ie.Candidates) != 2 {
			t.Errorf("Resolve(%q) err = %v, want index.AmbiguousError", in, err)
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
	var warns []string
	set, err := Open(&config.Config{Stores: []config.StoreEntry{a, b}}, Options{Warn: func(m string) { warns = append(warns, m) }})
	if err != nil {
		t.Fatal(err)
	}
	breakIndex(t, set.Stores[0])
	hit, err := set.Resolve(tb.ULID.String())
	if err != nil || hit.Store.Name != "org-b" {
		t.Fatalf("Resolve = %v %v", hit, err)
	}
	if len(warns) != 1 || !strings.HasPrefix(warns[0], "cannot open org-a: ") {
		t.Errorf("warnings = %q", warns)
	}
}

func TestResolveNamedStoreWhoseIndexDoesNotOpenIsAnError(t *testing.T) {
	a, b := newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "WEB")
	putTasks(t, a, 1, nil)
	var warns []string
	set, err := Open(&config.Config{Stores: []config.StoreEntry{a, b}}, Options{Warn: func(m string) { warns = append(warns, m) }})
	if err != nil {
		t.Fatal(err)
	}
	breakIndex(t, set.Stores[0])
	_, err = set.Resolve("org-a:API-1")
	if err == nil || !strings.HasPrefix(err.Error(), "cannot open org-a: ") || errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}
