package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

// task1 reads the task ORG-1 of the store org as the driver sees a file.
func task1(t *testing.T, te *testEnv) *model.Task {
	t.Helper()
	var ulid string
	for _, l := range strings.Split(te.ok("ls", "--ulids-only"), "\n") {
		if l != "" {
			ulid = l
		}
	}
	u, err := model.ParseULID(ulid)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(te.storeDir("org"), filepath.FromSlash(model.TaskPath(u)))) //nolint:gosec // the store of the test
	if err != nil {
		t.Fatal(err)
	}
	task, err := model.DecodeTask(b)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func writeTaskFile(t *testing.T, p string, task model.Task, title string, updated time.Time) {
	t.Helper()
	task.Title, task.Updated = title, updated
	b, err := task.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Both sides added the file: git gives an empty base file, which is no base.
func TestMergeDriverAddAdd(t *testing.T) {
	alice, _, _ := twoUsers(t)
	task := *task1(t, alice)
	task.ULID = model.NewULID()
	task.ID = "ORG-9"
	tmp := t.TempDir()
	base, ours, theirs := filepath.Join(tmp, "base"), filepath.Join(tmp, "ours"), filepath.Join(tmp, "theirs")
	if err := os.WriteFile(base, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := task.Updated
	writeTaskFile(t, ours, task, "ours", now.Add(time.Hour))
	writeTaskFile(t, theirs, task, "theirs", now.Add(2*time.Hour))
	t.Chdir(alice.storeDir("org"))
	if code, _, errs := alice.run("merge-driver", base, ours, theirs, model.TaskPath(task.ULID)); code != 0 {
		t.Fatalf("merge-driver: %d %s", code, errs)
	}
	merged, err := os.ReadFile(ours) //nolint:gosec // the file of the test
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.DecodeTask(merged)
	if err != nil || got.Title != "theirs" || got.ID != "ORG-9" {
		t.Errorf("merged = %+v, %v", got, err)
	}
	recs, err := os.ReadDir(filepath.Join(alice.storeDir("org"), ".hedgehug", "local", "overwritten"))
	if err != nil || len(recs) != 1 {
		t.Fatalf("records = %v, %v", recs, err)
	}
}

// The records of a merge git is in the middle of carry the commit merged in,
// so that the inbox can tell the merge was aborted.
func TestMergeDriverRecordCarriesMergeHead(t *testing.T) {
	t.Parallel()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	bin := buildHedgehug(t)
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "title=alice")
	alice.ok("sync")
	bob.ok("set", "ORG-1", "title=bob")
	dir := bob.storeDir("org")
	run := func(args ...string) {
		cmd := exec.Command(gitBin, append([]string{"-c", "user.name=bob", "-c", "user.email=bob@x", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"), "HEDGEHUG_AUTO_SYNC=0",
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("pull", "--no-rebase", "--no-commit", "origin", "main")
	mh, err := os.ReadFile(filepath.Join(dir, ".git", "MERGE_HEAD")) //nolint:gosec // the store of the test
	if err != nil {
		t.Fatal(err)
	}
	recs, err := os.ReadDir(filepath.Join(dir, ".hedgehug", "local", "overwritten"))
	if err != nil || len(recs) == 0 {
		t.Fatalf("records = %v, %v", recs, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".hedgehug", "local", "overwritten", recs[0].Name())) //nolint:gosec // the record of the test
	if err != nil {
		t.Fatal(err)
	}
	if want := `"commit":"` + strings.TrimSpace(string(mh)) + `"`; !strings.Contains(string(b), want) {
		t.Errorf("record = %s, want %s", b, want)
	}
	run("merge", "--abort")
}

// With a commit being merged in the times and authors come from the history,
// and the base is still git's: empty for a file both sides added.
func TestMergeDriverAddAddWithMergedInCommit(t *testing.T) {
	alice, _, _ := twoUsers(t)
	task := *task1(t, alice)
	task.ULID = model.NewULID()
	task.ID = "ORG-9"
	tmp := t.TempDir()
	base, ours, theirs := filepath.Join(tmp, "base"), filepath.Join(tmp, "ours"), filepath.Join(tmp, "theirs")
	if err := os.WriteFile(base, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeTaskFile(t, ours, task, "ours", task.Updated.Add(time.Hour))
	writeTaskFile(t, theirs, task, "theirs", task.Updated.Add(2*time.Hour))
	dir := alice.storeDir("org")
	t.Chdir(dir)
	r, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHEAD_"+head.String(), "theirs")
	if code, _, errs := alice.run("merge-driver", base, ours, theirs, model.TaskPath(task.ULID)); code != 0 {
		t.Fatalf("merge-driver: %d %s", code, errs)
	}
	merged, err := os.ReadFile(ours) //nolint:gosec // the file of the test
	if err != nil {
		t.Fatal(err)
	}
	if got, err := model.DecodeTask(merged); err != nil || got.ID != "ORG-9" {
		t.Errorf("merged = %+v, %v", got, err)
	}
	recs, err := os.ReadDir(filepath.Join(dir, ".hedgehug", "local", "overwritten"))
	if err != nil || len(recs) != 1 {
		t.Fatalf("records = %v, %v", recs, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".hedgehug", "local", "overwritten", recs[0].Name())) //nolint:gosec // the record of the test
	if err != nil || !strings.Contains(string(b), `"commit":"`+head.String()+`"`) {
		t.Errorf("record = %s, %v", b, err)
	}
}
