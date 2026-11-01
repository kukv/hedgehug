package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
	hhsync "github.com/kukv/hedgehug/internal/sync"
)

// run runs hedgehug in dir with its stdin not a terminal and returns the exit
// code, stdout and stderr; unlike ok and fail it accepts either outcome.
func (u *devUser) run(dir string, args ...string) (int, string, string) {
	u.t.Helper()
	cmd := exec.Command(u.bin, args...) //nolint:gosec // the binary the test built
	cmd.Env, cmd.Dir = u.env, dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run() // the exit code is read below
	return cmd.ProcessState.ExitCode(), stdout.String(), stderr.String()
}

// okIn is ok run in dir; it returns stdout and stderr.
func (u *devUser) okIn(dir string, args ...string) (string, string) {
	u.t.Helper()
	code, out, errs := u.run(dir, args...)
	if code != 0 {
		u.t.Fatalf("hedgehug %s: exit %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, out, errs)
	}
	return out, errs
}

// taskInfo is the part of the JSON of a task that the test reads.
type taskInfo struct {
	ULID   string   `json:"ulid"`
	ID     string   `json:"id"`
	Store  string   `json:"store"`
	Title  string   `json:"title"`
	Parent string   `json:"parent"`
	Repos  []string `json:"repos"`
}

// taskOf reads the task ref with show --json.
func taskOf(t *testing.T, u *devUser, ref string) taskInfo {
	t.Helper()
	var x taskInfo
	if err := json.Unmarshal([]byte(u.ok("show", ref, "--json")), &x); err != nil {
		t.Fatalf("show %s: %v", ref, err)
	}
	return x
}

// TestE2EPhase3b is the 3b of the design: the stores of a user mounted as one,
// ids across them, and where a new task goes.
func TestE2EPhase3b(t *testing.T) {
	bin, devenv := buildHedgehug(t), buildDevenv(t)
	dev, fake, _ := initDevenv(t, devenv)
	alice, bob := devUserOf(t, bin, devenv, dev, "alice"), devUserOf(t, bin, devenv, dev, "bob")

	// alice has me (DEV), org-a (ORG) and org-b (OPS). Add the workspaces WEB
	// in org-a and API in org-b, and make org-b's own prefix ORG by hand, as a
	// sync can leave two stores with one prefix; store init refuses it.
	alice.ok("workspace", "add", "web", "--prefix", "WEB", "--store", "org-a")
	alice.ok("workspace", "add", "api", "--prefix", "API", "--store", "org-b")
	orgB, err := store.Open(filepath.Join(alice.home, "stores", "org-b"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orgB.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, func(tx *store.Tx) error {
		for w, err := range orgB.Workspaces() {
			if err != nil {
				return err
			}
			if w.IDPrefix == "OPS" {
				w.IDPrefix = "ORG"
				if err := tx.PutWorkspace(w); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	alice.ok("add", "web task", "--store", "org-a", "--workspace", "web")                                        // WEB-1
	alice.ok("add", "api task", "--store", "org-b", "--workspace", "api")                                        // API-1
	alice.ok("add", "ops task", "--store", "org-b", "--workspace", "org-b")                                      // ORG-1 of org-b
	alice.ok("add", "mine", "--store", "me", "-p", "assignees=github.com/alice")                                 // DEV-1
	alice.ok("add", "theirs", "--store", "org-a", "--workspace", "web", "-p", "assignees=github.com/alice-corp") // WEB-2

	// ls: the STORE column, store:, and assignee:@me over both accounts.
	out := alice.ok("ls")
	if !strings.HasPrefix(out, "STORE") {
		t.Errorf("ls has no STORE column:\n%s", out)
	}
	rows := map[string]string{} // id -> store, from the first two columns
	for _, l := range strings.Split(out, "\n")[1:] {
		if f := strings.Fields(l); len(f) >= 2 {
			rows[f[1]] = f[0]
		}
	}
	for id, st := range map[string]string{"DEV-1": "me", "WEB-1": "org-a", "API-1": "org-b"} {
		if rows[id] != st {
			t.Errorf("ls row of %s has store %q, want %q:\n%s", id, rows[id], st, out)
		}
	}
	// ORG-1 is in two stores, so both rows carry the store-qualified id
	if rows["org-a:ORG-1"] != "org-a" || rows["org-b:ORG-1"] != "org-b" {
		t.Errorf("ls rows of ORG-1:\n%s", out)
	}
	out = alice.ok("ls", "store:org-a")
	if !strings.Contains(out, "WEB-1") || strings.Contains(out, "API-1") || strings.Contains(out, "DEV-1") {
		t.Errorf("ls store:org-a:\n%s", out)
	}
	out = alice.ok("ls", "assignee:@me")
	if !strings.Contains(out, "DEV-1") || !strings.Contains(out, "WEB-2") || strings.Contains(out, "WEB-1") {
		t.Errorf("ls assignee:@me (alice and alice-corp):\n%s", out)
	}

	// ids: a unique id needs no store; a shared one is ambiguous (exit 2)
	// until the store name is given.
	if x := taskOf(t, alice, "WEB-1"); x.Store != "org-a" || x.Title != "web task" {
		t.Errorf("show WEB-1: %+v", x)
	}
	if code, _, errs := alice.run("", "show", "ORG-1"); code != 2 || !strings.Contains(errs, "org-a") || !strings.Contains(errs, "org-b") {
		t.Errorf("show ORG-1: exit %d, %q", code, errs)
	}
	if x := taskOf(t, alice, "org-a:ORG-1"); x.Store != "org-a" || x.Title != "Web release" {
		t.Errorf("show org-a:ORG-1: %+v", x)
	}
	if x := taskOf(t, alice, "org-b:ORG-1"); x.Store != "org-b" || x.Title != "ops task" {
		t.Errorf("show org-b:ORG-1: %+v", x)
	}

	// a reference does not cross stores
	if code, errs := alice.fail("dep", "add", "API-1", "--blocked-by", "WEB-1"); code != 1 || !strings.Contains(errs, "references stay inside a store") {
		t.Errorf("dep across stores: exit %d, %q", code, errs)
	}
	var found []struct {
		Store string `json:"store"`
	}
	if err := json.Unmarshal([]byte(alice.ok("doctor", "--json")), &found); err != nil || len(found) != 3 {
		t.Errorf("doctor --json: %v, %d stores", err, len(found))
	}

	// add in the clone of web: under the project that has web in its repos,
	// with web on the task, and told where it went.
	projectULID := taskOf(t, alice, "org-a:ORG-1").ULID
	web := filepath.Join(alice.home, "src", "web")
	out, _ = alice.okIn(web, "add", "x")
	if !strings.Contains(out, "→ added to org-a / Web release") {
		t.Errorf("add in the clone of web:\n%s", out)
	}
	var added taskInfo
	out, _ = alice.okIn(web, "add", "y", "--json")
	if err := json.Unmarshal([]byte(out), &added); err != nil || added.Store != "org-a" || added.Parent != projectULID {
		t.Errorf("add --json in the clone: %v %q (project %s)", err, out, projectULID)
	}
	if x := taskOf(t, alice, "org-a:ORG-2"); x.Title != "x" || x.Parent != projectULID || !slices.Contains(x.Repos, "github.com/org-a/web") {
		t.Errorf("the task added in the clone: %+v", x)
	}

	// The branch of the clone names WEB-1. The stage that asks to add under
	// it needs a terminal; here stdin is none, so nothing is asked and the
	// task goes where the repository says.
	if err := os.WriteFile(filepath.Join(web, ".git", "HEAD"), []byte("ref: refs/heads/hh/WEB-1-x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	web1 := taskOf(t, alice, "WEB-1").ULID
	code, out, errs := alice.run(web, "add", "on a branch", "--json")
	var onBranch taskInfo
	if code != 0 || json.Unmarshal([]byte(out), &onBranch) != nil || errs != "" || strings.Contains(out+errs, "[Y/n]") {
		t.Fatalf("add on the branch: exit %d\nstdout: %s\nstderr: %s", code, out, errs)
	}
	if onBranch.Parent == web1 || onBranch.Parent != projectULID {
		t.Errorf("the task on the branch hh/WEB-1-x has parent %s, want the project %s (not WEB-1 %s)", onBranch.Parent, projectULID, web1)
	}
	if out, _ := alice.okIn(web, "add", "z", "--no-repo"); !strings.Contains(out, "added to org-a / Web release") {
		t.Errorf("add --no-repo in the clone:\n%s", out)
	}

	// use: the id is shown with its store as ORG is in two (D7); new tasks go
	// under the project, from any directory, until cleared.
	elsewhere := t.TempDir()
	out, _ = alice.okIn(elsewhere, "add", "before use")
	if !strings.Contains(out, "added to me / me") {
		t.Errorf("add before use:\n%s", out)
	}
	if out := alice.ok("use", "org-a:ORG-1"); !strings.Contains(out, "org-a / Web release is the default for new tasks") {
		t.Errorf("use:\n%s", out)
	}
	if out := alice.ok("use"); !strings.Contains(out, "org-a / org-a:ORG-1 Web release") {
		t.Errorf("use with no argument:\n%s", out)
	}
	var viaUse taskInfo
	out, _ = alice.okIn(elsewhere, "add", "after use", "--json")
	if err := json.Unmarshal([]byte(out), &viaUse); err != nil || viaUse.Store != "org-a" || viaUse.Parent != projectULID {
		t.Errorf("add after use: %v %q", err, out)
	}
	alice.ok("use", "--clear")
	if out := alice.ok("use"); strings.TrimSpace(out) != "-" {
		t.Errorf("use after --clear: %q", out)
	}
	var cleared taskInfo
	out, _ = alice.okIn(elsewhere, "add", "after clear", "--json")
	if err := json.Unmarshal([]byte(out), &cleared); err != nil || cleared.Store != "me" || cleared.Parent != "" {
		t.Errorf("add after --clear: %v %q", err, out)
	}

	// comments: only the author edits a comment.
	alice.ok("sync")
	bob.ok("sync")
	var c struct {
		ULID string `json:"ulid"`
	}
	if err := json.Unmarshal([]byte(bob.ok("comment", "WEB-1", "from bob", "--json")), &c); err != nil || c.ULID == "" {
		t.Fatalf("bob's comment: %v", err)
	}
	bob.ok("sync")
	alice.ok("sync")
	// The editor is this test binary, which writes the body of the script; the
	// refusal must come from the author check, not from a missing body.
	script := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(script, []byte("edited by alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	editing := &devUser{t: t, bin: alice.bin, home: alice.home, env: append(slices.Clone(alice.env), "EDITOR="+os.Args[0], "HH_TEST_EDITOR_SCRIPT="+script)}
	if code, errs := editing.fail("comment", "edit", c.ULID); code != 1 || errs != "hedgehug: github.com/bob's comment cannot be edited\n" {
		t.Errorf("alice edits bob's comment: exit %d, %q", code, errs)
	}
	if code, errs := alice.fail("comment", "rm", c.ULID); code != 1 || !strings.Contains(errs, "github.com/bob's comment cannot be removed") {
		t.Errorf("alice removes bob's comment: exit %d, %q", code, errs)
	}

	// mentions: bob is a collaborator of org-a, so there is no warning; once
	// GitHub no longer lists him and a sync has read that, a mention warns.
	if _, errs := alice.okIn("", "comment", "WEB-1", "hello @bob"); strings.Contains(errs, "cannot read") {
		t.Errorf("a mention of a reader warned: %q", errs)
	}
	if err := fake.SetPerm("org-a/hh-shared", "bob", "none"); err != nil {
		t.Fatal(err)
	}
	orgA, err := store.Open(filepath.Join(alice.home, "stores", "org-a"))
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute) // the interval between two checks has passed
	acc, err := hhsync.ReadAccess(orgA)
	if err != nil {
		t.Fatal(err)
	}
	acc.Checked = old
	if err := hhsync.WriteAccess(orgA, acc); err != nil {
		t.Fatal(err)
	}
	mem, err := hhsync.ReadMembers(orgA)
	if err != nil {
		t.Fatal(err)
	}
	mem.Checked = old
	if err := hhsync.WriteMembers(orgA, mem); err != nil {
		t.Fatal(err)
	}
	alice.ok("sync")
	if mem, err := hhsync.ReadMembers(orgA); err != nil || mem.Collaborators == nil || slices.ContainsFunc(mem.Collaborators, func(c hhsync.Collaborator) bool { return strings.HasSuffix(c.ID, "/bob") }) {
		t.Errorf("members.json after the sync: %+v (%v), want collaborators without bob", mem, err)
	}
	if _, errs := alice.okIn("", "comment", "WEB-1", "again @bob"); !strings.Contains(errs, "warning: bob cannot read org-a") {
		t.Errorf("a mention of someone who cannot read: %q", errs)
	}
}
