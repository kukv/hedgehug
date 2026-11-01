package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// newSharedPrefixEnv is an environment where org-a and org-b both use the
// prefix API and org-c uses WEB. `store init` refuses a prefix another store
// has, so org-b is given API by hand, as a sync can leave it.
func newSharedPrefixEnv(t *testing.T) *testEnv {
	t.Helper()
	te := newTestEnv(t)
	te.ok("store", "init", "org-a", "--prefix", "API")
	te.ok("store", "init", "org-b", "--prefix", "TMP")
	te.ok("store", "init", "org-c", "--prefix", "WEB")
	s, err := store.Open(te.storeDir("org-b"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, func(tx *store.Tx) error {
		for w, err := range s.Workspaces() {
			if err != nil {
				return err
			}
			w.IDPrefix = "API"
			if err := tx.PutWorkspace(w); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	te.addJSON("--store", "org-a", "Alpha task")
	te.addJSON("--store", "org-b", "Bravo task")
	te.addJSON("--store", "org-c", "Charlie task")
	return te
}

func TestPrefixAcrossStores(t *testing.T) {
	te := newSharedPrefixEnv(t)
	te.goldenOK("ls", "ls")
	te.goldenOK("tree", "ls", "--tree")
	te.goldenOK("show", "show", "org-a:API-1")
	te.goldenOK("show-web", "show", "WEB-1")
	te.goldenOK("set", "set", "org-a:API-1", "title=Renamed")
	te.goldenOK("done-b", "done", "org-b:API-1")
	// the hashes of a log change with the ULIDs, so it is read, not compared
	log := te.ok("log")
	for _, want := range []string{"hedgehug: add org-a:API-1 (", "hedgehug: add org-b:API-1 (", "hedgehug: set org-a:API-1 (", "hedgehug: add WEB-1 ("} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}

	// an inbox item that names ids
	s, err := store.Open(te.storeDir("org-a"))
	if err != nil {
		t.Fatal(err)
	}
	var task model.ULID
	for e, err := range s.Tasks() {
		if err != nil {
			t.Fatal(err)
		}
		task = e.ULID
	}
	if _, err := inbox.Open(s).Add([]inbox.Item{{
		Kind: inbox.Renumbered, Task: task, ID: "API-1", Path: model.TaskPath(task), Commit: "c",
		Time:   time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		Detail: json.RawMessage(`{"Old":"API-1","New":"API-2"}`),
	}}); err != nil {
		t.Fatal(err)
	}
	te.goldenOK("inbox", "inbox")

	te.goldenOK("rm", "rm", "org-a:API-1")
	te.goldenRun("set-deleted", 1, "set", "org-a:API-1", "status=done")
	te.goldenOK("restore", "restore", "org-a:API-1")

	// --json keeps the id as it is, and says the store
	var got []struct{ ID, Store string }
	if err := json.Unmarshal([]byte(te.ok("ls", "--json", "store:org-a")), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "API-1" || got[0].Store != "org-a" {
		t.Errorf("ls --json = %+v; want id API-1 of org-a", got)
	}
	if out := te.ok("show", "--json", "org-b:API-1"); !strings.Contains(out, `"id": "API-1"`) || strings.Contains(out, `"id": "org-b:`) {
		t.Errorf("show --json = %s", out)
	}
}

func TestPrefixNotSharedIsUnchanged(t *testing.T) {
	te := newTwoStoreEnv(t)
	for _, out := range []string{te.ok("ls"), te.ok("show", "AA-1"), te.ok("set", "AA-1", "title=Renamed"), te.ok("log")} {
		if strings.Contains(out, "alpha:AA") || strings.Contains(out, "bravo-long:BB") {
			t.Errorf("a prefix no other store has shows with the store name:\n%s", out)
		}
	}
}
