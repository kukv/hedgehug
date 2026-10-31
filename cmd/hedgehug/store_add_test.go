package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestStoreAddWarnsPrefixDuplicate(t *testing.T) {
	alice, _, remote := twoUsers(t) // the shared store uses ORG
	carol := alice.other()
	carol.ok("store", "init", "mine", "--prefix", "ORG") // later than the shared ORG
	code, out, errs := carol.run("store", "add", "org", "--remote", remote)
	if code != 0 || !strings.Contains(errs, "ORG is also the id prefix of workspace") || !strings.Contains(errs, "hedgehug workspace prefix") {
		t.Errorf("code %d out %s errs %s", code, out, errs)
	}
	// The store is added all the same.
	if out := carol.ok("store", "list"); !strings.Contains(out, "org") {
		t.Errorf("list = %s", out)
	}
}

func TestStoreAddPath(t *testing.T) {
	alice, _, remote := twoUsers(t)
	carol := alice.other()
	dir := filepath.Join(t.TempDir(), "elsewhere")
	out := carol.ok("store", "add", "org", "--remote", remote, "--path", dir)
	if !strings.Contains(out, dir) {
		t.Errorf("add = %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(model.StoreMetaPath))); err != nil {
		t.Error(err)
	}
	var list []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(carol.ok("store", "list", "--json")), &list); err != nil || len(list) != 1 || list[0].Path != dir {
		t.Errorf("list = %+v, %v", list, err)
	}
}

func TestStoreAddErrors(t *testing.T) {
	alice, bob, remote := twoUsers(t)
	if code, _, errs := bob.run("store", "add", "org", "--remote", remote); code != 1 || !strings.Contains(errs, "already set up") {
		t.Errorf("a name in use: code %d, stderr %s", code, errs)
	}
	if code, _, errs := alice.run("store", "add", "two"); code != 2 || !strings.Contains(errs, "--remote") {
		t.Errorf("no remote: code %d, stderr %s", code, errs)
	}
	empty := alice.bare("empty")
	dir := alice.storeDir("two")
	if code, _, _ := alice.run("store", "add", "two", "--remote", empty); code != 1 {
		t.Errorf("an empty remote: code %d", code)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the directory of the failed add is left: %v", err)
	}
	if out := alice.ok("store", "list"); strings.Contains(out, "two") {
		t.Errorf("list = %s", out)
	}
}

// A store whose .gitattributes lacks a line of the driver is set up when it
// is added, and the setup commit reaches the remote.
func TestStoreAddSetsUp(t *testing.T) {
	te := newTestEnv(t)
	remote := te.bare("old")
	s, err := store.Create(filepath.Join(t.TempDir(), "old"), &model.StoreMeta{ULID: model.NewULID(), Name: "old"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "old", IDPrefix: "OLD"}, gitrepo.Signature{Name: "A", Email: "a@example.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(store.Op{Name: "edit", Author: gitrepo.Signature{Name: "A", Email: "a@example.com"}}, func(tx *store.Tx) error {
		return tx.PutFile(store.AttributesPath, []byte("*.yaml merge=hedgehug\n"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().SetRemotes([]string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().Push("origin"); err != nil {
		t.Fatal(err)
	}

	te.ok("store", "add", "old", "--remote", remote)
	head := te.headCommit("old")
	if !strings.Contains(head.Message, "Hedgehug-Op: setup") {
		t.Errorf("HEAD = %q, want the setup commit", head.Message)
	}
	if out := te.ok("sync", "--status", "--json"); !strings.Contains(out, `"unpushed": 0`) {
		t.Errorf("the setup commit was not pushed: %s", out)
	}
}

func TestStoreListAndRemove(t *testing.T) {
	alice, _, _ := twoUsers(t)
	out := alice.ok("store", "list")
	if !strings.Contains(out, "org") || !strings.Contains(out, "shared.git") {
		t.Errorf("list = %s", out)
	}
	alice.ok("add", "unpushed")
	if code, _, errs := alice.run("store", "remove", "org", "--purge"); code != 1 || !strings.Contains(errs, "hedgehug sync") {
		t.Errorf("purge with unpushed commits: %d %s", code, errs)
	}
	if _, err := os.Stat(alice.storeDir("org")); err != nil {
		t.Errorf("a refused purge removed the directory: %v", err)
	}
	alice.ok("store", "remove", "org")
	if out := alice.ok("store", "list"); strings.Contains(out, "org") {
		t.Errorf("list after remove = %s", out)
	}
	if _, err := os.Stat(alice.storeDir("org")); err != nil {
		t.Errorf("remove without --purge removed the directory: %v", err)
	}
}

func TestStoreRemovePurge(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("store", "remove", "org", "--purge")
	if _, err := os.Stat(alice.storeDir("org")); !os.IsNotExist(err) {
		t.Errorf("the directory is left: %v", err)
	}
	if code, _, errs := alice.run("store", "remove", "org"); code != 1 || errs == "" {
		t.Errorf("remove again: code %d, stderr %s", code, errs)
	}
}

func TestStoreRemovePurgeWithoutRemote(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	if code, _, errs := te.run("store", "remove", "me", "--purge"); code != 1 || !strings.Contains(errs, "no remote") {
		t.Errorf("code %d, stderr %s", code, errs)
	}
}

func TestStoreListJSON(t *testing.T) {
	alice, _, remote := twoUsers(t)
	var list []struct {
		Name        string   `json:"name"`
		Remotes     []string `json:"remotes"`
		Account     string   `json:"account"`
		LastSuccess string   `json:"last_success"`
	}
	if err := json.Unmarshal([]byte(alice.ok("store", "list", "--json")), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "org" || len(list[0].Remotes) != 1 || list[0].Remotes[0] != remote || list[0].Account != "github.com/alice" || list[0].LastSuccess == "" {
		t.Errorf("list = %+v", list)
	}
}
