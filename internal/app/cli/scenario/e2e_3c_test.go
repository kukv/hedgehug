package scenario

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// addTask adds a task with args and returns it.
func addTask(t *testing.T, u *devUser, title string, args ...string) taskInfo {
	t.Helper()
	var x taskInfo
	out := u.ok(append([]string{"add", title, "--json"}, args...)...)
	if err := unmarshalWritten([]byte(out), &x); err != nil {
		t.Fatalf("add %s: %v\n%s", title, err, out)
	}
	return x
}

// storeLog is the commits of the store name of u, newest first.
func storeLog(t *testing.T, u *devUser, name string) []gitrepotest.Commit {
	t.Helper()
	return gitrepotest.Log(t, filepath.Join(u.home, "stores", name), 0)
}

// taskFilePath is the path, in the work tree of the store name of u, of the
// file that holds the task with the ULID.
func taskFilePath(t *testing.T, u *devUser, name, ulid string) string {
	t.Helper()
	dir := filepath.Join(u.home, "stores", name)
	var found string
	err := filepath.WalkDir(filepath.Join(dir, "tasks"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.Contains(p, ulid) {
			found, err = filepath.Rel(dir, p)
		}
		return err
	})
	if err != nil || found == "" {
		t.Fatalf("no file of %s in %s: %v", ulid, name, err)
	}
	return filepath.ToSlash(found)
}

// TestE2EPhase3c is the 3c of the design: a move between stores, what the
// other members see of it.
func TestE2EPhase3c(t *testing.T) {
	t.Parallel()
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, fake, _ := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")

	// 1. A task in each of the three stores; the STORE column shows each on a
	// row of its own store, and each store's commits are by the account of
	// that store.
	alice.ok("add", "mine", "--store", "me")
	alice.ok("add", "ops", "--store", "org-b")
	proj := addTask(t, alice, "project", "--store", "org-a", "--kind", "project")
	child := addTask(t, alice, "child", "--parent", proj.ULID)
	keep := addTask(t, alice, "keep", "--store", "org-a")
	alice.ok("dep", "add", child.ULID, "--blocked-by", keep.ULID)
	alice.ok("sync")
	out := alice.ok("ls")
	if !strings.HasPrefix(out, "STORE") {
		t.Errorf("ls has no STORE column:\n%s", out)
	}
	for store, title := range map[string]string{"me": "mine", "org-b": "ops", "org-a": "project"} {
		found := false
		for _, l := range strings.Split(out, "\n") {
			if f := strings.Fields(l); len(f) > 0 && f[0] == store && strings.HasSuffix(l, title) {
				found = true
			}
		}
		if !found {
			t.Errorf("ls has no row of %q in the store %s:\n%s", title, store, out)
		}
	}
	for name, author := range map[string]string{"me": "github.com/alice", "org-a": "github.com/alice-corp", "org-b": "github.com/alice-corp"} {
		log := storeLog(t, alice, name)
		n := 0
		for _, e := range log {
			if a, ok := e.Trailer("Hedgehug-Author"); ok {
				n++
				if a != author {
					t.Errorf("%s: a commit has Hedgehug-Author %q, want %q (%s)", name, a, author, e.Subject)
				}
			}
		}
		if n == 0 {
			t.Errorf("%s: no commit has a Hedgehug-Author", name)
		}
	}

	// bob gets org-a, and is given write on org-b (the check is made stale as
	// the 5 minutes pass).
	bob.ok("sync")
	if err := fake.SetPerm("org-b/hh-ops", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	acc := bob.uc().ReadAccess("org-b")
	acc.Checked = time.Now().Add(-10 * time.Minute)
	bob.uc().WriteAccess("org-b", acc)
	bob.ok("sync")

	// bob edits and comments on the child before the move, and does not push.
	bob.ok("set", child.ID, "title=child by bob", "--store", "org-a")
	bob.ok("comment", "add", child.ID, "bob was here", "--store", "org-a")
	// Commit times are seconds: the move must be a second later than bob's
	// edit, or the two would tie.
	sleepToNextSecond()

	// 2. alice moves the project, with the child, to org-b.
	var mv struct {
		Tasks []struct {
			ULID  string `json:"ulid"`
			OldID string `json:"old_id"`
			NewID string `json:"new_id"`
		} `json:"tasks"`
		Unlinked []struct {
			Task   string `json:"task"`
			Field  string `json:"field"`
			Target string `json:"target"`
		} `json:"unlinked"`
	}
	mvOut := alice.ok("mv", "org-a:"+proj.ID, "--store", "org-b", "--yes", "--json")
	if err := json.Unmarshal([]byte(mvOut), &mv); err != nil {
		t.Fatalf("mv --json: %v\n%s", err, mvOut)
	}
	if len(mv.Tasks) != 2 || mv.Tasks[0].ULID != proj.ULID || mv.Tasks[0].OldID != proj.ID || mv.Tasks[1].ULID != child.ULID {
		t.Fatalf("mv moved %+v, want the project and the child", mv.Tasks)
	}
	if len(mv.Unlinked) != 1 || mv.Unlinked[0].Task != child.ULID || mv.Unlinked[0].Field != "blocked_by" || mv.Unlinked[0].Target != keep.ULID {
		t.Errorf("mv unlinked %+v, want the child's blocked_by on keep", mv.Unlinked)
	}
	moved, movedChild := taskOf(t, alice, proj.ULID), taskOf(t, alice, child.ULID)
	if moved.Store != "org-b" || moved.ID != mv.Tasks[0].NewID || !strings.HasPrefix(moved.ID, "OPS-") {
		t.Errorf("the project is %+v, want a new id of org-b", moved)
	}
	if movedChild.Store != "org-b" || movedChild.Parent != proj.ULID || movedChild.ID != mv.Tasks[1].NewID {
		t.Errorf("the child is %+v, want it in org-b under the project", movedChild)
	}
	// The history keeps the Hedgehug-Unlinked trailer, which names keep, so
	// only the fields of the task are looked at.
	var childView struct {
		BlockedBy []struct {
			ULID string `json:"ulid"`
		} `json:"blocked_by"`
	}
	if out := alice.ok("show", child.ULID, "--json"); json.Unmarshal([]byte(out), &childView) != nil || len(childView.BlockedBy) != 0 {
		t.Errorf("the child still refers to keep:\n%s", out)
	}
	// The forward is a file of org-a at the path of the task, with none of
	// the task's content.
	forwardRel := taskFilePath(t, alice, "org-a", proj.ULID)
	forward, err := os.ReadFile(filepath.Join(alice.home, "stores", "org-a", filepath.FromSlash(forwardRel)))
	if err != nil || len(forward) == 0 || strings.Contains(string(forward), "title: project") {
		t.Errorf("the forward file %s = %q, %v; want a forward, not the task", forwardRel, forward, err)
	}
	code, _, errs := alice.run("", "show", "org-a:"+proj.ID)
	want := proj.ID + " moved to org-b (org-b:" + moved.ID + ")"
	if code != 1 || !strings.Contains(errs, want) {
		t.Errorf("show of the old id: exit %d, %q; want %q", code, errs, want)
	}
	if x := taskOf(t, alice, proj.ID); x.Store != "org-b" || x.Title != "project" {
		t.Errorf("show %s = %+v, want the task of org-b", proj.ID, x)
	}
	alice.ok("sync")

	// 3. bob's edit of the child lost to the move: he is told, and where. The
	// child in org-b keeps the title it had when it moved.
	bob.ok("sync")
	if x := taskOf(t, bob, "org-b:"+movedChild.ID); x.Title != "child" {
		t.Errorf("bob's edit reached the child in org-b: %+v", x)
	}
	inbox := bob.ok("inbox")
	if !strings.Contains(inbox, "overwritten") || !strings.Contains(inbox, "→ apply it to org-b's "+movedChild.ID) {
		t.Errorf("bob's inbox lacks the notice with the new place of the child (%s):\n%s", movedChild.ID, inbox)
	}
	if n := len(overwrittenNaming(t, bob, child.ULID)); n == 0 {
		t.Error("no commit of bob's org-a has Hedgehug-Overwritten naming the child")
	}
	// 4. purge takes the forward files away.
	pm := alice.ok("purge", "--older-than", "0d", "--store", "org-a", "--yes")
	if !strings.Contains(pm, "2 forward files") {
		t.Errorf("purge printed %q, want the two forwards removed", pm)
	}
	if _, err := os.Stat(filepath.Join(alice.home, "stores", "org-a", filepath.FromSlash(forwardRel))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the forward file is still there: %v", err)
	}
	if code, _, errs := alice.run("", "show", "org-a:"+proj.ID); code == 0 || strings.Contains(errs, "moved to") {
		t.Errorf("show of the old id after purge: exit %d, %q", code, errs)
	}
	alice.ok("sync")
	bob.ok("sync")
}

// overwrittenNaming lists the Hedgehug-Overwritten and Hedgehug-Repaired
// lines, in the commits of the store org-a of u, that name the ULID.
func overwrittenNaming(t *testing.T, u *devUser, ulid string) []string {
	t.Helper()
	log := storeLog(t, u, "org-a")
	var out []string
	for _, e := range log {
		for _, key := range []string{"Hedgehug-Overwritten", "Hedgehug-Repaired"} {
			for _, v := range e.TrailerValues(key) {
				if strings.Contains(v, ulid) {
					out = append(out, key+": "+v)
				}
			}
		}
	}
	return out
}
