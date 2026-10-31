package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
)

func TestSyncTwoUsers(t *testing.T) {
	alice := newTestEnv(t)
	remote := alice.bare("shared")
	alice.writeAccounts("github.com", "alice")
	alice.ok("store", "init", "org", "--prefix", "ORG", "--remote", remote)
	alice.ok("add", "x")
	alice.ok("sync")
	bob := alice.other()
	bob.writeAccounts("github.com", "bob")
	bob.ok("store", "add", "org", "--remote", remote)
	if out := bob.ok("ls"); !strings.Contains(out, "ORG-1") {
		t.Errorf("bob ls = %s", out)
	}
	alice.ok("set", "ORG-1", "title=from alice")
	bob.ok("status", "ORG-1", "in_progress")
	alice.ok("sync")
	bob.ok("sync")
	alice.ok("sync")
	for _, te := range []*testEnv{alice, bob} {
		out := te.ok("show", "ORG-1", "--json")
		if !strings.Contains(out, `"title": "from alice"`) || !strings.Contains(out, `"status": "in_progress"`) {
			t.Errorf("show = %s", out)
		}
	}
}

func TestSyncShowsRenumbered(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("add", "a")
	bob.ok("add", "b")
	alice.ok("sync")
	out := bob.ok("sync")
	if !regexp.MustCompile(`ORG-2 \([0-9A-Z]{26}\) → ORG-3`).MatchString(out) {
		t.Errorf("sync output = %s", out)
	}
	if !strings.Contains(out, "Synced org: 1 merges, pushed to origin") {
		t.Errorf("sync output = %s", out)
	}
}

func TestSyncShowsOverwritten(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	bob.ok("set", "ORG-1", "title=bob's")
	time.Sleep(1100 * time.Millisecond) // commit times are seconds: alice writes later, so alice's value wins
	alice.ok("set", "ORG-1", "title=alice's")
	alice.ok("sync")
	out := bob.ok("sync")
	if !regexp.MustCompile(`overwritten: ORG-1 title \(github\.com/bob's value was replaced; hedgehug inbox revert [0-9A-Z]{26}\)`).MatchString(out) {
		t.Errorf("sync output = %s", out)
	}
	if !strings.Contains(bob.ok("ls", "is:overwritten"), "ORG-1") {
		t.Error("is:overwritten does not find the task")
	}
	if strings.Contains(alice.ok("ls", "is:overwritten"), "ORG-1") {
		t.Error("is:overwritten finds the task for the one whose value won")
	}
}

func TestSyncJSON(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("add", "a")
	bob.ok("add", "b")
	alice.ok("sync")
	var got []struct {
		Store    string   `json:"store"`
		Merges   int      `json:"merges"`
		Pushed   []string `json:"pushed"`
		Repaired []struct {
			Kind string `json:"kind"`
		} `json:"repaired"`
	}
	out := bob.ok("sync", "--json")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(got) != 1 || got[0].Store != "org" || got[0].Merges != 1 || len(got[0].Repaired) == 0 || got[0].Repaired[0].Kind != "id_collision" {
		t.Errorf("sync --json = %s", out)
	}
}

func TestSyncWritesState(t *testing.T) {
	alice, _, remote := twoUsers(t)
	state := filepath.Join(alice.storeDir("org"), ".hedgehug", "local", "sync.json")
	b, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"last_success"`) || strings.Contains(string(b), `"last_error"`) {
		t.Errorf("sync.json after a sync = %s", b)
	}
	if err := os.RemoveAll(remote); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := alice.run("sync"); code != 1 || errs == "" {
		t.Errorf("sync with no remote: code %d, stderr %q", code, errs)
	}
	b, err = os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"last_error"`) || !strings.Contains(string(b), `"last_attempt"`) {
		t.Errorf("sync.json after a failed sync = %s", b)
	}
}

func TestSyncClearsPendingSince(t *testing.T) {
	alice, _, _ := twoUsers(t)
	dir := alice.storeDir("org")
	state := filepath.Join(dir, ".hedgehug", "local", "sync.json")
	// A write before the sync began is pushed by it; one after is not.
	for name, pending := range map[string]string{"before": "2026-10-30T11:00:00Z", "after": "2026-10-30T13:00:00Z"} {
		if err := os.WriteFile(state, []byte(`{"pending_since":"`+pending+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		alice.ok("sync")
		b, _ := os.ReadFile(state)
		has := strings.Contains(string(b), "pending_since")
		if name == "before" && has || name == "after" && !has {
			t.Errorf("%s: sync.json = %s", name, b)
		}
	}
}

// A write made after the push, while pending_since was already set by an
// earlier one, finds it set and leaves it: the sync must not clear it.
func TestSyncKeepsPendingForAWriteAfterThePush(t *testing.T) {
	alice, _, _ := twoUsers(t)
	dir := alice.storeDir("org")
	state := filepath.Join(dir, ".hedgehug", "local", "sync.json")
	if err := os.WriteFile(state, []byte(`{"pending_since":"2026-10-30T11:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	alice.ok("add", "pushed")
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	wrote := false
	// The clock is read after the push too; the first such read writes,
	// as another command would.
	now := func() time.Time {
		if n, err := unpushed(s); !wrote && err == nil && n == 0 {
			wrote = true
			if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "alice", Email: "alice@x"}}, func(tx *store.Tx) error {
				return tx.PutFile("late.txt", []byte("late\n"))
			}); err != nil {
				t.Fatal(err)
			}
		}
		return alice.now
	}
	var out, errb bytes.Buffer
	code := run([]string{"sync"}, &env{stdin: strings.NewReader(""), stdout: &out, stderr: &errb, getenv: alice.getenv, isTerminal: func(any) bool { return false }, now: now})
	if code != 0 || !wrote {
		t.Fatalf("sync: exit %d, wrote %v\n%s", code, wrote, errb.String())
	}
	if b, _ := os.ReadFile(state); !strings.Contains(string(b), "pending_since") {
		t.Errorf("a write the sync did not push is not pending: %s", b)
	}
}

func TestSyncStatus(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "a")
	out := alice.ok("sync", "--status")
	if !strings.Contains(out, "1 commit not pushed") {
		t.Errorf("status = %s", out)
	}
	alice.ok("sync")
	if out := alice.ok("sync", "--status", "--json"); !strings.Contains(out, `"unpushed": 0`) {
		t.Errorf("status json = %s", out)
	}
}

func TestSyncStatusNoRemote(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	if out := te.ok("sync", "--status"); !strings.Contains(out, "no remote") {
		t.Errorf("status = %s", out)
	}
	if out := te.ok("sync"); !strings.Contains(out, "no store has a remote") {
		t.Errorf("sync = %s", out)
	}
	if out := te.ok("sync", "--store", "me"); !strings.Contains(out, "me has no remote") {
		t.Errorf("sync --store me = %s", out)
	}
}

func TestSyncAllStores(t *testing.T) {
	alice, _, _ := twoUsers(t)
	remote2 := alice.bare("second")
	alice.ok("store", "init", "two", "--prefix", "TWO", "--remote", remote2)
	alice.ok("--store", "two", "add", "y")
	out := alice.ok("sync")
	if !strings.Contains(out, "Synced org:") || !strings.Contains(out, "Synced two:") {
		t.Errorf("sync = %s", out)
	}
	if out := alice.ok("sync", "--store", "two"); strings.Contains(out, "Synced org:") {
		t.Errorf("sync --store two = %s", out)
	}
}

func TestStoreInitRemoteNotEmpty(t *testing.T) {
	alice, _, remote := twoUsers(t)
	code, _, errs := alice.run("store", "init", "again", "--prefix", "AG", "--remote", remote)
	if code != 1 || !strings.Contains(errs, "hedgehug store add again --remote "+remote) {
		t.Errorf("code %d, stderr %s", code, errs)
	}
	// Nothing is left: the next try starts clean.
	if _, err := os.Stat(alice.storeDir("again")); !os.IsNotExist(err) {
		t.Errorf("the directory of the failed store is left: %v", err)
	}
	if out := alice.ok("store", "list"); strings.Contains(out, "again") {
		t.Errorf("list = %s", out)
	}
}

func TestStoreInitRemoteFirstCommit(t *testing.T) {
	te := newTestEnv(t)
	remote := te.bare("shared")
	te.ok("store", "init", "org", "--prefix", "ORG", "--remote", remote)
	log := te.log("org")
	if len(log) != 1 || !strings.Contains(log[0].Message, "Hedgehug-Op: init") {
		t.Fatalf("log = %d commits", len(log))
	}
	b, err := os.ReadFile(filepath.Join(te.storeDir("org"), ".gitattributes"))
	if err != nil || !strings.Contains(string(b), "tasks/**/*.md merge=hedgehug") {
		t.Errorf(".gitattributes = %q, %v", b, err)
	}
	cfg, err := os.ReadFile(filepath.Join(te.storeDir("org"), ".git", "config"))
	if err != nil || !strings.Contains(string(cfg), "driver = hedgehug merge-driver %O %A %B %P") {
		t.Errorf("driver not registered: %q, %v", cfg, err)
	}
	if out := te.ok("sync", "--status", "--json"); !strings.Contains(out, `"unpushed": 0`) {
		t.Errorf("the first push did not happen: %s", out)
	}
}
