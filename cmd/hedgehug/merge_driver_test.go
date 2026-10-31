package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

// gitPull runs a real `git pull` in the store of te, with the binary on PATH
// as hedgehug for the merge driver.
func gitPull(t *testing.T, gitBin, bin string, te *testEnv) {
	t.Helper()
	cmd := exec.Command(gitBin, "-c", "user.name=bob", "-c", "user.email=bob@x", "-c", "commit.gpgsign=false", "pull", "--no-rebase", "origin", "main")
	cmd.Dir = te.storeDir("org")
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+filepath.Dir(gitBin), "HEDGEHUG_AUTO_SYNC=0",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1") // not the signing of whoever runs the test
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git pull: %v\n%s", err, out)
	}
}

func TestMergeDriverWithGitPull(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	// The binary is built once, and put on PATH as hedgehug.
	bin := buildHedgehug(t)
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "title=alice")
	alice.ok("sync")
	bob.ok("status", "ORG-1", "in_progress")
	gitPull(t, gitBin, bin, bob)
	out := bob.ok("show", "ORG-1", "--json")
	if !strings.Contains(out, `"title": "alice"`) || !strings.Contains(out, `"status": "in_progress"`) {
		t.Errorf("after git pull: %s", out)
	}
	// The merge by git is a merge of fields, so the next sync has nothing to merge.
	if out := bob.ok("sync"); !strings.Contains(out, "0 merges") {
		t.Errorf("sync after git pull = %s", out)
	}
}

// driverFiles makes the files of a merge of ORG-1 in alice's store as git gives
// them to the driver: both sides changed the title, and theirs is later.
func driverFiles(t *testing.T, alice *testEnv) (dir, rel, o, a, b string) {
	t.Helper()
	var shown struct {
		ULID string `json:"ulid"`
	}
	if err := json.Unmarshal([]byte(alice.ok("show", "ORG-1", "--json")), &shown); err != nil {
		t.Fatal(err)
	}
	ulid, err := model.ParseULID(shown.ULID)
	if err != nil {
		t.Fatal(err)
	}
	dir = alice.storeDir("org")
	base, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.TaskPath(ulid))))
	if err != nil {
		t.Fatal(err)
	}
	task, err := model.DecodeTask(base)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	write := func(name, title string, updated time.Time) string {
		x := *task
		x.Title, x.Updated = title, updated
		b, err := x.Encode()
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	o = write("base", task.Title, task.Updated)
	a = write("ours", "ours", task.Updated.Add(time.Hour))
	b = write("theirs", "theirs", task.Updated.Add(2*time.Hour))
	return dir, model.TaskPath(ulid), o, a, b
}

// Two clones add a task apart and take the same ID. A hand-made `git pull`
// merges the files, which leaves the ID twice (the two tasks are different
// files, so git has nothing to conflict on); the next sync repairs it with a
// commit of its own, and the task that is on origin keeps the ID.
func TestGitPullThenSyncRepairsIDCollision(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	bin := buildHedgehug(t)
	alice, bob, _ := twoUsers(t)
	alice.ok("add", "a")
	bob.ok("add", "b")
	alice.ok("sync") // a reaches origin first, as ORG-2
	gitPull(t, gitBin, bin, bob)
	if out := bob.ok("ls", "--json"); strings.Count(out, `"id": "ORG-2"`) != 2 {
		t.Fatalf("after git pull the ID is not taken twice:\n%s", out)
	}
	before := len(bob.log("org"))
	bob.ok("sync")
	if n := len(bob.log("org")); n != before+1 || !strings.Contains(bob.headCommit("org").Message, "Hedgehug-Op: repair") {
		t.Errorf("sync made %d commits after the pull; head:\n%s", n-before, bob.headCommit("org").Message)
	}
	if a := bob.ok("show", "ORG-2", "--json"); !strings.Contains(a, `"title": "a"`) {
		t.Errorf("ORG-2 is not the task of origin:\n%s", a)
	}
	if b := bob.ok("show", "ORG-3", "--json"); !strings.Contains(b, `"title": "b"`) || !strings.Contains(b, "ORG-2") {
		t.Errorf("b was not renumbered to ORG-3 with ORG-2 in former_ids:\n%s", b)
	}
	if out := bob.ok("ls", "--json"); strings.Count(out, `"id": "ORG-2"`) != 1 {
		t.Errorf("ORG-2 is still taken twice:\n%s", out)
	}
	// What the repair pushed makes alice see the same.
	alice.ok("sync")
	if got := alice.ok("show", "ORG-3", "--json"); !strings.Contains(got, `"title": "b"`) {
		t.Errorf("alice's ORG-3 after the sync:\n%s", got)
	}
}

// Both sides changed the same field: the driver writes the winner to <ours>
// and the loser to a record for the inbox. With no commit being merged in (a
// rebase, say) the sides carry no history, and the later `updated` wins.
func TestMergeDriverRecordsOverwritten(t *testing.T) {
	alice, _, _ := twoUsers(t)
	dir, rel, o, a, b := driverFiles(t, alice)
	t.Chdir(dir)
	if code, _, errs := alice.run("merge-driver", o, a, b, rel); code != 0 {
		t.Fatalf("merge-driver: %d %s", code, errs)
	}
	merged, err := os.ReadFile(a) //nolint:gosec // the file of the test
	if err != nil {
		t.Fatal(err)
	}
	if got, err := model.DecodeTask(merged); err != nil || got.Title != "theirs" {
		t.Errorf("merged title = %v, %v; want theirs", got, err)
	}
	recs, err := os.ReadDir(filepath.Join(dir, ".hedgehug", "local", "overwritten"))
	if err != nil || len(recs) != 1 {
		t.Fatalf("records = %v, %v", recs, err)
	}
	rec, err := os.ReadFile(filepath.Join(dir, ".hedgehug", "local", "overwritten", recs[0].Name())) //nolint:gosec // the record of the test
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rec), `"field":"title"`) || !strings.Contains(string(rec), `"value":"\"ours\""`) || !strings.Contains(string(rec), rel) {
		t.Errorf("record = %s", rec)
	}
	// The next sync (or hh inbox) takes the record into the inbox.
	alice.ok("sync")
	if _, err := os.Stat(filepath.Join(dir, ".hedgehug", "local", "overwritten", recs[0].Name())); !os.IsNotExist(err) {
		t.Errorf("the sync left the record: %v", err)
	}
}

// git runs the driver while it holds .git/index.lock, so the driver neither
// takes the lock nor removes it.
func TestMergeDriverIgnoresIndexLock(t *testing.T) {
	alice, _, _ := twoUsers(t)
	dir, rel, o, a, b := driverFiles(t, alice)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	// A driver that took the lock would fail with ErrGitBusy, as the lock stays.
	code, _, errs := alice.run("merge-driver", o, a, b, rel)
	if code != 0 || strings.Contains(errs, "index.lock") {
		t.Fatalf("merge-driver with index.lock: %d %s", code, errs)
	}
	merged, err := os.ReadFile(a) //nolint:gosec // the file of the test
	if err != nil {
		t.Fatal(err)
	}
	if got, err := model.DecodeTask(merged); err != nil || got.Title != "theirs" {
		t.Errorf("merged title = %v, %v; want theirs", got, err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the index.lock of git is gone: %v", err)
	}
}

func TestMergeDriverFails(t *testing.T) {
	alice, _, _ := twoUsers(t)
	t.Chdir(alice.storeDir("org"))
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "missing")
	// A file the driver was not given is left to the conflict of git: exit 1.
	if code, _, _ := alice.run("merge-driver", missing, missing, missing, "tasks/00/X.md"); code != 1 {
		t.Errorf("missing files: code %d, want 1", code)
	}
	// So is a merge of a commit the store does not have.
	for _, n := range []string{"base", "ours", "theirs"} {
		if err := os.WriteFile(filepath.Join(tmp, n), []byte(n+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GITHEAD_0123456789abcdef0123456789abcdef01234567", "unknown")
	if code, _, _ := alice.run("merge-driver", filepath.Join(tmp, "base"), filepath.Join(tmp, "ours"), filepath.Join(tmp, "theirs"), "tasks/00/X.md"); code != 1 {
		t.Errorf("unknown commit: code %d, want 1", code)
	}
	if code, _, _ := alice.run("merge-driver", "a", "b"); code != 2 {
		t.Errorf("two arguments: code %d, want 2", code)
	}
}
