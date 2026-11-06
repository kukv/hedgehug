package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestStoreAddWarnsPrefixDuplicate(t *testing.T) {
	t.Parallel()
	alice, _, remote := twoUsers(t) // the shared store uses ORG
	carol := alice.other()
	carol.ok("store", "init", "mine", "--prefix", "ORG") // later than the shared ORG
	carol.goldenRun("add", 0, "store", "add", "org", "--remote", remote)
	// The store is added all the same.
	carol.goldenOK("list", "store", "list")
}

func TestStoreAddPath(t *testing.T) {
	t.Parallel()
	alice, _, remote := twoUsers(t)
	carol := alice.other()
	dir := filepath.Join(t.TempDir(), "elsewhere")
	out := carol.ok("store", "add", "org", "--remote", remote, "--path", dir)
	if !strings.Contains(out, dir) {
		t.Errorf("add = %s", out)
	}
	carol.golden("add", out)
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
	t.Parallel()
	alice, bob, remote := twoUsers(t)
	bob.goldenRun("name-in-use", 1, "store", "add", "org", "--remote", remote)
	alice.goldenRun("no-remote", 2, "store", "add", "two")
	empty := alice.bare("empty")
	dir := alice.storeDir("two")
	alice.goldenRun("empty-remote", 1, "store", "add", "two", "--remote", empty)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the directory of the failed add is left: %v", err)
	}
	alice.goldenOK("list", "store", "list")
}

// A store whose .gitattributes lacks a line of the driver is set up when it
// is added, and the setup commit reaches the remote.
func TestStoreAddSetsUp(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	remote := te.bare("old")
	usecasetest.NewOldStoreWithAttributes(t, remote)

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
	t.Parallel()
	alice, _, _ := twoUsers(t)
	out := alice.ok("store", "list")
	if !strings.Contains(out, "org") || !strings.Contains(out, "shared.git") {
		t.Errorf("list = %s", out)
	}
	alice.ok("add", "unpushed")
	code, _, errs := alice.run("store", "remove", "org")
	if code != 1 || !strings.Contains(errs, "hedgehug sync --store org") || !strings.Contains(errs, "--keep") {
		t.Errorf("remove with unpushed commits: %d %s", code, errs)
	}
	if _, err := os.Stat(alice.storeDir("org")); err != nil {
		t.Errorf("a refused removal removed the directory: %v", err)
	}
	if out := alice.ok("store", "list"); !strings.Contains(out, "org") {
		t.Errorf("a refused removal took the store out of the config: %s", out)
	}
	if out := alice.ok("store", "remove", "org", "--keep"); out != "Removed store org from the config; "+alice.storeDir("org")+" is left\n" {
		t.Errorf("remove --keep = %q", out)
	}
	if out := alice.ok("store", "list"); strings.Contains(out, "org") {
		t.Errorf("list after remove = %s", out)
	}
	if _, err := os.Stat(alice.storeDir("org")); err != nil {
		t.Errorf("remove --keep removed the directory: %v", err)
	}
}

func TestStoreRemoveDeletes(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	if out := alice.ok("store", "remove", "org"); out != "Removed store org and deleted "+alice.storeDir("org")+"\n" {
		t.Errorf("remove = %q", out)
	}
	if _, err := os.Stat(alice.storeDir("org")); !os.IsNotExist(err) {
		t.Errorf("the directory is left: %v", err)
	}
	if code, _, errs := alice.run("store", "remove", "org"); code != 1 || errs == "" {
		t.Errorf("remove again: code %d, stderr %s", code, errs)
	}
}

// A directory that is already gone is only taken out of the config.
func TestStoreRemoveOfADirectoryThatIsGone(t *testing.T) {
	t.Parallel()
	alice, _, _ := twoUsers(t)
	dir := alice.storeDir("org")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if out := alice.ok("store", "remove", "org"); out != "Removed store org from the config; "+dir+" was not there\n" {
		t.Errorf("remove = %q", out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the directory came back: %v", err)
	}
}

func TestStoreRemoveWithoutRemote(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	code, _, errs := te.run("store", "remove", "me")
	if code != 1 || !strings.Contains(errs, "no remote") || !strings.Contains(errs, "--keep") {
		t.Errorf("code %d, stderr %s", code, errs)
	}
	if _, err := os.Stat(te.storeDir("me")); err != nil {
		t.Errorf("a refused removal removed the directory: %v", err)
	}
	te.ok("store", "remove", "me", "--keep")
}

func TestStoreListJSON(t *testing.T) {
	t.Parallel()
	alice, _, remote := twoUsers(t)
	var list []struct {
		Name        string   `json:"name"`
		Remotes     []string `json:"remotes"`
		Account     string   `json:"account"`
		LastSuccess string   `json:"last_success"`
		Default     bool     `json:"default"`
	}
	if err := json.Unmarshal([]byte(alice.ok("store", "list", "--json")), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "org" || len(list[0].Remotes) != 1 || list[0].Remotes[0] != remote || list[0].Account != "github.com/alice" || list[0].LastSuccess == "" || !list[0].Default {
		t.Errorf("list = %+v", list)
	}
}
