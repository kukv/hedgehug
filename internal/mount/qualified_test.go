package mount

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

// displayIDs is the display id of the first task of each store of set.
func displayIDs(t *testing.T, set *Set) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, m := range set.Stores {
		ix, err := m.Index()
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ix.Tasks() {
			out[m.Name] = ix.DisplayID(e)
		}
	}
	return out
}

func TestSharedPrefixesShowWithTheStoreName(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	b := newStoreAt(t, "org-b", "API")
	c := newStoreAt(t, "org-c", "WEB")
	for _, e := range []config.StoreEntry{a, b, c} {
		putTasks(t, e, 1, nil)
	}
	got := displayIDs(t, openSet(t, a, b, c))
	want := map[string]string{"org-a": "org-a:API-1", "org-b": "org-b:API-1", "org-c": "WEB-1"}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: DisplayID = %q; want %q", name, got[name], w)
		}
	}
}

func TestSharedPrefixesDoNotCountAStoreThatDoesNotOpen(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	gone := config.StoreEntry{Name: "gone", Path: filepath.Join(t.TempDir(), "nope")}
	putTasks(t, a, 1, nil)
	if got := displayIDs(t, openSet(t, a, gone))["org-a"]; got != "API-1" {
		t.Errorf("DisplayID = %q; want API-1", got)
	}
}

func TestSharedPrefixesOfAConfigIsTheOneOpenGives(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	b := newStoreAt(t, "org-b", "API")
	c := &config.Config{Stores: []config.StoreEntry{a, b}}
	shared := SharedPrefixes(c)
	if !shared("API") || !shared("api") || shared("WEB") {
		t.Errorf("SharedPrefixes: API %v, api %v, WEB %v", shared("API"), shared("api"), shared("WEB"))
	}
	set, err := Open(c, Options{Qualified: func(string) bool { return false }})
	if err != nil {
		t.Fatal(err)
	}
	putTasks(t, a, 1, nil)
	if got := displayIDs(t, set)["org-a"]; got != "API-1" {
		t.Errorf("with Options.Qualified saying no, DisplayID = %q; want API-1", got)
	}
}

func TestQualifiedIsKeptAcrossRefreshAndReopen(t *testing.T) {
	a := newStoreAt(t, "org-a", "API")
	b := newStoreAt(t, "org-b", "API")
	set := openSet(t, a, b)
	m := set.Stores[0]
	ix, err := m.Index()
	if err != nil {
		t.Fatal(err)
	}
	putTasks(t, a, 1, nil)
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := ix.QualifyID("API-7"); got != "org-a:API-7" {
		t.Errorf("after Refresh QualifyID = %q", got)
	}
	if got := ix.QualifyID(""); got != "" {
		t.Errorf("QualifyID(\"\") = %q", got)
	}
	re, err := m.reopen()
	if err != nil {
		t.Fatal(err)
	}
	if got := re.QualifyID("API-7"); got != "org-a:API-7" {
		t.Errorf("after reopen QualifyID = %q", got)
	}
}
