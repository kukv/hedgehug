package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// addTask adds a task with args and returns it.
func addTask(t *testing.T, u *devUser, title string, args ...string) taskInfo {
	t.Helper()
	var x taskInfo
	out := u.ok(append([]string{"add", title, "--json"}, args...)...)
	if err := json.Unmarshal([]byte(out), &x); err != nil {
		t.Fatalf("add %s: %v\n%s", title, err, out)
	}
	return x
}

// storeLog is the commits of the store name of u, newest first.
func storeLog(t *testing.T, u *devUser, name string) (*gitrepo.Repo, []gitrepo.LogEntry) {
	t.Helper()
	s, err := store.Open(filepath.Join(u.home, "stores", name))
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return s.Repo(), log
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

// inProcess runs hedgehug in this process as u, with a terminal and the lines
// as its standard input (D27).
func inProcess(t *testing.T, u *devUser, input string, args ...string) (int, string, string) {
	t.Helper()
	vars := map[string]string{}
	for _, kv := range u.env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	var out, errb bytes.Buffer
	e := &env{
		stdin:      strings.NewReader(input),
		stdout:     &out,
		stderr:     &errb,
		getenv:     func(k string) string { return vars[k] },
		isTerminal: func(any) bool { return true },
		now:        time.Now,
	}
	code := run(args, e)
	return code, out.String(), errb.String()
}

// TestE2EPhase3c is the 3c of the design: a move between stores, what the
// other members see of it, and the rewrite of the history.
func TestE2EPhase3c(t *testing.T) {
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, fake, _ := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")

	// 1. A task in each of the three stores; the STORE column shows them, and
	// each store's commits are by the account of that store.
	alice.ok("add", "mine", "--store", "me")
	alice.ok("add", "ops", "--store", "org-b")
	proj := addTask(t, alice, "project", "--store", "org-a", "--kind", "project")
	child := addTask(t, alice, "child", "--parent", proj.ULID)
	keep := addTask(t, alice, "keep", "--store", "org-a")
	alice.ok("dep", "add", child.ULID, "--blocked-by", keep.ULID)
	alice.ok("sync")
	out := alice.ok("ls")
	for _, want := range []string{"STORE", "me ", "org-a", "org-b"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls lacks %q:\n%s", want, out)
		}
	}
	for name, author := range map[string]string{"me": "github.com/alice", "org-a": "github.com/alice-corp", "org-b": "github.com/alice-corp"} {
		_, log := storeLog(t, alice, name)
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
	s, err := store.Open(filepath.Join(bob.home, "stores", "org-b"))
	if err != nil {
		t.Fatal(err)
	}
	acc, err := hhsync.ReadAccess(s)
	if err != nil {
		t.Fatal(err)
	}
	acc.Checked = time.Now().Add(-10 * time.Minute)
	if err := hhsync.WriteAccess(s, acc); err != nil {
		t.Fatal(err)
	}
	bob.ok("sync")

	// bob edits and comments on the child before the move, and does not push.
	bob.ok("set", child.ID, "title=child by bob", "--store", "org-a")
	bob.ok("comment", child.ID, "bob was here", "--store", "org-a")
	time.Sleep(1100 * time.Millisecond)

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
	if out := alice.ok("show", child.ULID, "--json"); strings.Contains(out, keep.ULID) {
		t.Errorf("the child still refers to keep:\n%s", out)
	}
	if got := taskFilePath(t, alice, "org-a", proj.ULID); got == "" {
		t.Error("org-a has no forward file")
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

	// 3. bob's edit of the child lost to the move: he is told, and where.
	bob.ok("sync")
	inbox := bob.ok("inbox")
	if !strings.Contains(inbox, "overwritten") || !strings.Contains(inbox, "→ apply it to org-b's "+movedChild.ID) {
		t.Errorf("bob's inbox lacks the notice with the new place of the child (%s):\n%s", movedChild.ID, inbox)
	}
	_, blog := storeLog(t, bob, "org-a")
	overwritten := false
	for _, e := range blog {
		if len(e.TrailerValues("Hedgehug-Overwritten")) > 0 {
			overwritten = true
		}
	}
	if !overwritten {
		t.Error("no commit of bob's org-a has Hedgehug-Overwritten")
	}

	bobPath := taskFilePath(t, bob, "org-a", proj.ULID) // the forward is at the path of the task

	// 4. purge --moved takes the forward files away.
	alice.ok("purge", "--moved", "--store", "org-a", "--yes")
	if code, _, errs := alice.run("", "show", "org-a:"+proj.ID); code == 0 || strings.Contains(errs, "moved to") {
		t.Errorf("show of the old id after purge --moved: exit %d, %q", code, errs)
	}
	alice.ok("sync")
	bob.ok("sync")

	// 5. bob has an edit not pushed; alice rewrites the history of org-a to be
	// rid of the moved project (D34: its forward is gone, so the task is named
	// by its place in org-b).
	bob.ok("set", keep.ID, "title=keep by bob", "--store", "org-a")
	code, rout, rerr := inProcess(t, alice, "org-a\n", "purge", "--rewrite-history", moved.ID)
	if code != 0 {
		t.Fatalf("purge --rewrite-history: exit %d\nstdout: %s\nstderr: %s", code, rout, rerr)
	}
	inHistory := func() bool {
		repo, log := storeLog(t, bob, "org-a")
		for _, e := range log {
			tree, err := repo.TreeOf(e.Hash)
			if err != nil {
				t.Fatal(err)
			}
			for p, err := range repo.Files(tree, "tasks/") {
				if err != nil {
					t.Fatal(err)
				}
				if p == bobPath {
					return true
				}
			}
		}
		return false
	}
	if !inHistory() {
		t.Fatalf("%s is in no commit of bob's org-a before the rewrite; the check below would prove nothing", bobPath)
	}
	bob.ok("sync")

	// bob's org-a was made again: his edit is back, and the task is in no
	// commit of it.
	if x := taskOf(t, bob, "org-a:"+keep.ID); x.Title != "keep by bob" {
		t.Errorf("bob's edit did not come back: %+v", x)
	}
	if inHistory() {
		t.Errorf("%s is still in a commit of bob's org-a", bobPath)
	}
	inbox = bob.ok("inbox")
	if !strings.Contains(inbox, "rewrite_applied") {
		t.Errorf("bob's inbox lacks rewrite_applied:\n%s", inbox)
	}
}
