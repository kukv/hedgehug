package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The warning names the workspace whose prefix was set later: here the one
// carol made after the shared store.
func TestStoreAddWarningNamesLater(t *testing.T) {
	t.Parallel()
	alice, _, remote := twoUsers(t)
	carol := alice.other()
	carol.now = carol.now.Add(time.Second) // prefix_set is in seconds
	carol.ok("store", "init", "mine", "--prefix", "ORG")
	carol.goldenRun("add", 0, "store", "add", "org", "--remote", remote)
}

func TestStoreInitRemoteJSON(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	remote, mirror := te.bare("one"), te.bare("two")
	var got struct {
		Name    string   `json:"name"`
		Remotes []string `json:"remotes"`
		Pushed  []string `json:"pushed"`
	}
	out := te.goldenOK("init", "store", "init", "org", "--prefix", "ORG", "--remote", remote, "--remote", mirror, "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got.Name != "org" || len(got.Remotes) != 2 || len(got.Pushed) != 2 || got.Pushed[0] != "origin" || got.Pushed[1] != "mirror1" {
		t.Errorf("init --json = %s", out)
	}
}

func TestStoreAddMirror(t *testing.T) {
	t.Parallel()
	alice, _, remote := twoUsers(t)
	mirror := alice.bare("mirror")
	carol := alice.other()
	carol.goldenOK("add", "store", "add", "org", "--remote", remote, "--remote", mirror)
	cfg, err := os.ReadFile(filepath.Join(carol.storeDir("org"), ".git", "config"))
	if err != nil || !strings.Contains(string(cfg), `[remote "mirror1"]`) || !strings.Contains(string(cfg), strings.ReplaceAll(mirror, `\`, `\\`)) { // .git/config escapes a backslash
		t.Errorf(".git/config = %q, %v", cfg, err)
	}
	carol.goldenOK("list", "store", "list")
	// A sync fills the mirror.
	carol.goldenOK("sync-first", "sync")
	carol.goldenOK("sync-second", "sync")
}

func TestStoreListAccountIsID(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	var list []struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal([]byte(alice.ok("store", "list", "--json")), &list); err != nil || len(list) != 1 || list[0].Account != "github.com/alice" {
		t.Errorf("list = %+v, %v", list, err)
	}
}

func TestStoreRemoveJSON(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	var got struct {
		Name   string `json:"name"`
		Purged bool   `json:"purged"`
	}
	out := alice.goldenOK("remove", "store", "remove", "org", "--purge", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Name != "org" || !got.Purged {
		t.Errorf("remove --json = %s, %v", out, err)
	}
}

func TestStoreRemovePurgeRefusesDirtyTree(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	dir := alice.storeDir("org")
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("hand edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errs := alice.run("store", "remove", "org", "--purge")
	if code != 1 || !strings.Contains(errs, "not committed") || !strings.Contains(errs, "notes.md") {
		t.Errorf("code %d, stderr %s", code, errs)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("a refused purge removed the directory: %v", err)
	}
	if out := alice.ok("store", "list"); !strings.Contains(out, "org") {
		t.Errorf("a refused purge took the store out of the config: %s", out)
	}
}

// One store that cannot be read does not hide the status of the others.
func TestSyncStatusSkipsBrokenStore(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	remote2 := alice.bare("second")
	alice.ok("store", "init", "two", "--prefix", "TWO", "--remote", remote2)
	if err := os.WriteFile(filepath.Join(alice.storeDir("two"), ".hedgehug", "local", "sync.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	alice.goldenOK("status", "sync", "--status")
	var rows []struct {
		Store string `json:"store"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(alice.goldenOK("status-json", "sync", "--status", "--json")), &rows); err != nil || len(rows) != 2 || rows[0].Error != "" || rows[1].Error == "" {
		t.Errorf("rows = %+v, %v", rows, err)
	}
}
