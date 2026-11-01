package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// breakStore makes the store name stop opening: its repository goes away.
func (te *testEnv) breakStore(name string) {
	te.t.Helper()
	if err := os.RemoveAll(filepath.Join(te.storeDir(name), ".git")); err != nil {
		te.t.Fatal(err)
	}
}

// A sync tells of a reference to another store once, with the store's name
// in front, as the other warnings of a sync have it.
func TestSyncWarnsOfACrossStoreRefOnce(t *testing.T) {
	te := newTestEnv(t)
	remote := te.bare("shared")
	te.ok("store", "init", "alpha", "--prefix", "AA", "--remote", remote)
	te.ok("store", "init", "bravo-long", "--prefix", "BB")
	target := te.addJSON("--store", "bravo-long", "Bravo target")
	te.addJSON("--store", "alpha", "Alpha task")
	te.handEdit("alpha", "AA-1", "\nkind: ", "\nblocked_by: ["+target.ULID+"]\nkind: ")
	code, _, stderr := te.run("sync")
	if code != 0 {
		t.Fatalf("sync: exit %d\nstderr %s", code, stderr)
	}
	want := "warning: alpha: AA-1 blocked_by points at bravo-long's BB-1; references stay inside a store\n"
	if stderr != want {
		t.Errorf("stderr = %q; want %q", stderr, want)
	}
}

// hh use --json gives the id as the file has it, even where the text shows
// it with the store's name.
func TestUseJSONIDIsPlainWhenThePrefixIsShared(t *testing.T) {
	te := newSharedPrefixEnv(t)
	te.addJSON("--store", "org-a", "Alpha project", "--kind", "project")
	te.addJSON("--store", "org-b", "Bravo project", "--kind", "project")
	te.ok("use", "org-b:API-2")
	if got := te.ok("use"); !strings.Contains(got, "org-b:API-2") {
		t.Fatalf("use = %q; want the id with the store's name", got)
	}
	var got struct {
		Store   string `json:"store"`
		Project struct {
			ID, Title string
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(te.ok("use", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Store != "org-b" || got.Project.ID != "API-2" || got.Project.Title != "Bravo project" {
		t.Errorf("use --json = %+v; want org-b's API-2", got)
	}
}

// hh use --store looks in that store only.
func TestUseWithStoreLooksInThatStoreOnly(t *testing.T) {
	te := newSharedPrefixEnv(t)
	te.addJSON("--store", "org-a", "Alpha project", "--kind", "project")
	te.addJSON("--store", "org-b", "Bravo project", "--kind", "project")
	if code, _, _ := te.run("use", "API-2"); code != 2 {
		t.Errorf("use API-2 in two stores: exit %d; want 2", code)
	}
	te.goldenOK("by-flag", "use", "--store", "org-b", "API-2")
	te.goldenRun("conflict", 2, "use", "--store", "org-b", "org-a:API-2")
	te.goldenRun("unknown", 1, "use", "--store", "nope", "API-2")
}

// inbox leaves out a store that does not open, with a warning, and the
// store --store names is the error.
func TestInboxLeavesOutAStoreThatDoesNotOpen(t *testing.T) {
	te := newTwoStoreEnv(t)
	te.breakStore("bravo-long")
	code, out, stderr := te.run("inbox")
	if code != 0 || out != "No unread items\n" || strings.Count(stderr, "warning: cannot open bravo-long: ") != 1 {
		t.Errorf("inbox: exit %d\nstdout %q\nstderr %q", code, out, stderr)
	}
	code, _, stderr = te.run("inbox", "--store", "bravo-long")
	if code == 0 || !strings.Contains(stderr, "bravo-long") {
		t.Errorf("inbox --store bravo-long: exit %d\nstderr %q", code, stderr)
	}
}

// A command about one store does not warn of the others.
func TestStoreFlagKeepsOtherBrokenStoresQuiet(t *testing.T) {
	te := newTwoStoreEnv(t)
	te.breakStore("bravo-long")
	code, _, stderr := te.run("ls", "--store", "alpha")
	if code != 0 || stderr != "" {
		t.Errorf("ls --store alpha: exit %d\nstderr %q", code, stderr)
	}
	code, _, stderr = te.run("ls")
	if code != 0 || !strings.HasPrefix(stderr, "warning: cannot open bravo-long: ") {
		t.Errorf("ls: exit %d\nstderr %q", code, stderr)
	}
}

// A status that only one store's schema has is looked for in that store.
func TestLsStatusOfAnotherSchema(t *testing.T) {
	te := newTwoStoreEnv(t)
	s, err := store.Open(te.storeDir("alpha"))
	if err != nil {
		t.Fatal(err)
	}
	sc := schema.Default()
	sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
	author := gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}
	if _, err := s.Update(store.Op{Name: "update", Author: author}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	te.addJSON("--store", "alpha", "Alpha reviewed", "--status", "review")
	te.goldenOK("status", "ls", "status:review")
	te.goldenOK("not-status", "ls", "--", "-status:review")
	te.goldenRun("unknown", 2, "ls", "status:zzz")
}
