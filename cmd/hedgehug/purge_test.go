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

func TestPurgeOlderThan(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	b := te.addJSON("b")
	te.ok("rm", a.ID)
	te.now = te.now.AddDate(0, 0, 50)
	te.ok("rm", b.ID)
	te.now = te.now.AddDate(0, 0, 50) // a was deleted 100 days ago, b 50 days ago
	if out := te.ok("purge", "--older-than", "90d", "--yes"); out != "Purged 1 task and 0 forward files from me\n" {
		t.Errorf("purge --older-than 90d printed %q", out)
	}
	if out := te.ok("ls", "is:deleted", "--ulids-only"); strings.TrimSpace(out) != b.ULID {
		t.Errorf("deleted tasks left = %q; want only b (%s)", out, b.ULID)
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: purge\n") {
		t.Errorf("not a purge commit:\n%s", te.log("me")[0].Message)
	}
	// the default is 90 days: b is 50 days old, and 91 a little later
	if out := te.ok("purge", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("purge printed %q", out)
	}
	te.now = te.now.AddDate(0, 0, 41)
	if out := te.ok("purge", "--yes"); out != "Purged 1 task and 0 forward files from me\n" {
		t.Errorf("purge printed %q", out)
	}
}

func TestPurgeZeroDaysAndNothingToDo(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	if out := te.ok("purge", "--older-than", "1d", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("1d printed %q", out)
	}
	te.ok("purge", "--older-than", "0d", "--yes")
	before := len(te.log("me"))
	if out := te.ok("purge", "--older-than", "0d", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("printed %q", out)
	}
	if got := len(te.log("me")); got != before {
		t.Errorf("nothing to purge wrote %d commits", got-before)
	}
}

func TestPurgeConfirmOnATerminal(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("P")
	te.addJSON("C", "--parent", p.ID)
	te.ok("rm", p.ID)
	prompt := "me:\n  ME-1  P\n  ME-2  C\nPurge 2 tasks and 0 forward files from me for good? [y/N] "
	te.terminal = true

	te.stdin = "n\n"
	code, out, errs := te.run("purge", "--older-than", "0d")
	if code != 1 || out != "" || errs != prompt+"hedgehug: Canceled\n" {
		t.Errorf("declined: exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if got := te.ok("ls", "is:deleted", "--ulids-only"); strings.Count(got, "\n") != 2 {
		t.Errorf("a declined purge removed tasks: %q", got)
	}

	te.stdin = "y\n"
	code, out, errs = te.run("purge", "--older-than", "0d")
	if code != 0 || out != "Purged 2 tasks and 0 forward files from me\n" || errs != prompt {
		t.Errorf("accepted: exit %d, stdout %q, stderr %q", code, out, errs)
	}
}

func TestPurgeWithoutYesOffATerminal(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	before := len(te.log("me"))
	code, _, errs := te.run("purge", "--older-than", "0d")
	if code != 2 || errs != "hedgehug: purge: confirm with --yes\n" {
		t.Errorf("exit %d, stderr %q; want 2 and the refusal", code, errs)
	}
	if got := len(te.log("me")); got != before {
		t.Errorf("a refused purge wrote %d commits", got-before)
	}
}

// The forward files of moved tasks go by their moved_at.
func TestPurgeForwardsAndJSON(t *testing.T) {
	t.Parallel()
	te := newForwardEnv(t) // AA-2 moved to bravo-long at te.now
	if out := te.ok("purge", "--older-than", "1d", "--store", "alpha", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("1d printed %q; the forward is new", out)
	}
	te.now = te.now.AddDate(0, 0, 10)
	if out := te.ok("purge", "--older-than", "11d", "--store", "alpha", "--yes"); out != "Nothing to purge\n" {
		t.Errorf("11d printed %q", out)
	}
	out := te.ok("purge", "--older-than", "10d", "--store", "alpha", "--yes", "--json")
	var got struct {
		Store string `json:"store"`
		Tasks []any  `json:"tasks"`
		Fwds  []struct {
			ULID    string `json:"ulid"`
			MovedTo string `json:"moved_to"`
			MovedAt string `json:"moved_at"`
		} `json:"forwards"`
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got.Store != "alpha" || got.Tasks == nil || len(got.Tasks) != 0 || len(got.Fwds) != 1 || got.Commit == "" ||
		got.Fwds[0].MovedAt != "2026-10-30T12:00:00Z" || got.Fwds[0].ULID == "" || got.Fwds[0].MovedTo == "" {
		t.Errorf("json = %s", out)
	}
	var none map[string]any
	if err := json.Unmarshal([]byte(te.ok("purge", "--older-than", "0d", "--store", "alpha", "--yes", "--json")), &none); err != nil {
		t.Fatal(err)
	}
	if f, ok := none["forwards"].([]any); !ok || len(f) != 0 || none["commit"] != nil {
		t.Errorf("none = %v; want empty lists and no commit", none)
	}
	if tasks, ok := none["tasks"].([]any); !ok || len(tasks) != 0 {
		t.Errorf("none tasks = %v; want []", none["tasks"])
	}
	if code, _, errs := te.run("show", "alpha:AA-2"); code == 0 || strings.Contains(errs, "moved to") {
		t.Errorf("show of the old id after the purge: exit %d, %q", code, errs)
	}
}

func TestPurgeNeedsAStoreWhenThereAreSeveral(t *testing.T) {
	t.Parallel()
	te := newForwardEnv(t)
	code, out, errs := te.run("purge", "--yes")
	if code != 1 || out != "" || errs != "hedgehug: several stores are set up; pick one with --store (stores: alpha, bravo-long)\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
}

// A forward to a store that is not mounted is purged like the others.
func TestPurgeAForwardToAStoreNotMounted(t *testing.T) {
	t.Parallel()
	te := newForwardEnv(t)
	te.ok("store", "remove", "bravo-long", "--keep")
	if out := te.ok("purge", "--older-than", "0d", "--yes"); out != "Purged 0 tasks and 1 forward file from alpha\n" {
		t.Errorf("printed %q", out)
	}
}

func TestPurgeUsageErrors(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"purge", "ME-1"}, "--older-than"},
		{[]string{"purge", "--moved"}, "unknown flag: --moved"},
		{[]string{"purge", "--older-than", "90"}, `"90" is not a period`},
		{[]string{"purge", "--older-than=-5d"}, `"-5d" is not a period`},
		{[]string{"purge", "--older-than", "1w"}, `"1w" is not a period`},
		{[]string{"purge", "--older-than", "d"}, `"d" is not a period`},
	} {
		code, _, errs := te.run(c.args...)
		if code != 2 || !strings.Contains(errs, c.want) {
			t.Errorf("%v: exit %d, stderr %q; want 2 and %q", c.args, code, errs, c.want)
		}
	}
}

// The option that rewrote the history is gone: an unknown flag is a usage error.
func TestPurgeRewriteHistoryIsGone(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("x")
	code, _, errs := te.run("purge", "--rewrite-history", x.ID)
	if code != 2 || !strings.Contains(errs, "unknown flag: --rewrite-history") {
		t.Errorf("exit %d, stderr %q; want a usage error", code, errs)
	}
}

// A deleted task with a task under it that is not deleted stays; the other
// deleted tasks go, and purge tells of the one that stays.
func TestPurgeKeepsATaskWithALiveChild(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("p")
	c := te.addJSON("c", "--parent", p.ID)
	g := te.addJSON("g")
	te.ok("rm", p.ID)
	te.ok("rm", g.ID)
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	// the child is alive again under a deleted parent (restore would refuse it)
	if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}},
		changeTask(c.ULID, func(t *model.Task) { t.DeletedAt = nil })); err != nil {
		t.Fatal(err)
	}
	code, out, errs := te.run("purge", "--older-than", "0d", "--yes")
	if code != 0 || out != "Purged 1 task and 0 forward files from me\n" || !strings.Contains(errs, "warning: kept (a task under each is not deleted): "+p.ID) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if got := te.task(p.ULID); got.DeletedAt == nil {
		t.Error("the deleted parent lost its deleted_at")
	}
}

func TestPurgeRecordsTheNumbers(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("a")
	b := te.addJSON("b")
	te.ok("rm", a.ID)
	te.ok("rm", b.ID)
	te.ok("purge", "--older-than", "0d", "--yes")
	data, err := os.ReadFile(filepath.Join(te.storeDir("me"), filepath.FromSlash(model.NumbersPath)))
	if err != nil || !strings.Contains(string(data), "ME: 2\n") {
		t.Errorf("numbers.yaml = %q, %v; want ME: 2", data, err)
	}
	if next := te.addJSON("c"); next.ID != "ME-3" {
		t.Errorf("the next id = %s; want ME-3, not a number that was purged", next.ID)
	}
}

func TestUndoDoesNotTakeBackAPurge(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.writeAccounts("github.com", "alice") // undo needs an account
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	te.ok("purge", "--older-than", "0d", "--yes")
	before := len(te.log("me"))
	code, _, errs := te.run("undo")
	if code != 1 || !strings.Contains(errs, "a purge cannot be undone") {
		t.Errorf("undo: exit %d, stderr %q", code, errs)
	}
	if got := len(te.log("me")); got != before {
		t.Errorf("undo wrote %d commits", got-before)
	}
}

// workspace rm counts the deleted tasks too, and says how to get rid of them.
func TestWorkspaceRmAfterRmNeedsAPurge(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	x := te.addJSON("x", "--workspace", "api")
	te.ok("rm", x.ID)
	code, _, errs := te.run("workspace", "rm", "api")
	want := "move it with hedgehug mv --workspace, or delete it with hedgehug rm and remove it with hedgehug purge --older-than 0d --store <store>"
	if code != 1 || !strings.Contains(errs, want) {
		t.Errorf("workspace rm: exit %d, stderr %q; want it to say %q", code, errs, want)
	}
	te.ok("purge", "--older-than", "0d", "--yes")
	te.ok("workspace", "rm", "api")
}
