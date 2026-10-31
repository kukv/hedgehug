package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

func TestSetLater(t *testing.T) {
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	older, newer := &model.Workspace{PrefixSet: t0}, &model.Workspace{PrefixSet: t0.Add(time.Second)}
	if !setLater(newer, older) || setLater(older, newer) {
		t.Error("the prefix set at the later time is not the later one")
	}
	// Set at the same time, the larger ULID is the later one.
	small := &model.Workspace{ULID: model.NewULID(), PrefixSet: t0}
	large := &model.Workspace{ULID: model.NewULID(), PrefixSet: t0}
	if small.ULID.String() > large.ULID.String() {
		small, large = large, small
	}
	if !setLater(large, small) || setLater(small, large) {
		t.Error("the larger ULID is not the later one on a tie")
	}
	if setLater(small, small) {
		t.Error("a workspace is later than itself")
	}
}

// The warning names the workspace whose prefix was set later: here the one
// carol made after the shared store.
func TestStoreAddWarningNamesLater(t *testing.T) {
	alice, _, remote := twoUsers(t)
	time.Sleep(1100 * time.Millisecond) // prefix_set is in seconds
	carol := alice.other()
	carol.ok("store", "init", "mine", "--prefix", "ORG")
	_, _, errs := carol.run("store", "add", "org", "--remote", remote)
	if !strings.Contains(errs, "(workspace mine in store mine)") {
		t.Errorf("stderr = %s", errs)
	}
}

func TestStoreInitRemoteJSON(t *testing.T) {
	te := newTestEnv(t)
	remote, mirror := te.bare("one"), te.bare("two")
	var got struct {
		Name    string   `json:"name"`
		Remotes []string `json:"remotes"`
		Pushed  []string `json:"pushed"`
	}
	out := te.ok("store", "init", "org", "--prefix", "ORG", "--remote", remote, "--remote", mirror, "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got.Name != "org" || len(got.Remotes) != 2 || len(got.Pushed) != 2 || got.Pushed[0] != "origin" || got.Pushed[1] != "mirror1" {
		t.Errorf("init --json = %s", out)
	}
}

func TestStoreAddMirror(t *testing.T) {
	alice, _, remote := twoUsers(t)
	mirror := alice.bare("mirror")
	carol := alice.other()
	carol.ok("store", "add", "org", "--remote", remote, "--remote", mirror)
	cfg, err := os.ReadFile(filepath.Join(carol.storeDir("org"), ".git", "config"))
	if err != nil || !strings.Contains(string(cfg), `[remote "mirror1"]`) || !strings.Contains(string(cfg), strings.ReplaceAll(mirror, `\`, `\\`)) { // .git/config escapes a backslash
		t.Errorf(".git/config = %q, %v", cfg, err)
	}
	if out := carol.ok("store", "list"); !strings.Contains(out, remote+","+mirror) {
		t.Errorf("list = %s", out)
	}
	// A sync fills the mirror.
	carol.ok("sync")
	if out := carol.ok("sync"); !strings.Contains(out, "pushed to origin, mirror1") {
		t.Errorf("sync = %s", out)
	}
}

func TestStoreListAccountIsID(t *testing.T) {
	alice, _, _ := twoUsers(t)
	var list []struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal([]byte(alice.ok("store", "list", "--json")), &list); err != nil || len(list) != 1 || list[0].Account != "github.com/alice" {
		t.Errorf("list = %+v, %v", list, err)
	}
}

func TestStoreRemoveJSON(t *testing.T) {
	alice, _, _ := twoUsers(t)
	var got struct {
		Name   string `json:"name"`
		Purged bool   `json:"purged"`
	}
	out := alice.ok("store", "remove", "org", "--purge", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Name != "org" || !got.Purged {
		t.Errorf("remove --json = %s, %v", out, err)
	}
}

func TestStoreRemovePurgeRefusesDirtyTree(t *testing.T) {
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
	alice, _, _ := twoUsers(t)
	remote2 := alice.bare("second")
	alice.ok("store", "init", "two", "--prefix", "TWO", "--remote", remote2)
	if err := os.WriteFile(filepath.Join(alice.storeDir("two"), ".hedgehug", "local", "sync.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := alice.ok("sync", "--status")
	if !strings.Contains(out, "0 commits not pushed") || !strings.Contains(out, "error:") {
		t.Errorf("status = %s", out)
	}
	var rows []struct {
		Store string `json:"store"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(alice.ok("sync", "--status", "--json")), &rows); err != nil || len(rows) != 2 || rows[0].Error != "" || rows[1].Error == "" {
		t.Errorf("rows = %+v, %v", rows, err)
	}
}
