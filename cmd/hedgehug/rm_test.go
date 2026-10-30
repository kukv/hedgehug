package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// tree3 adds a project with a child and a grandchild.
func tree3(te *testEnv) (p, c, g taskJSON) {
	p = te.addJSON("P", "--kind", "project")
	c = te.addJSON("C", "--parent", p.Key)
	g = te.addJSON("G", "--parent", c.Key)
	return p, c, g
}

// The commands that change a task refuse a deleted one and point to
// restore.
func TestChangeDeletedTask(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	b := te.addJSON("Blocker")
	te.ok("rm", x.Key)
	before := len(te.log("me"))
	for _, args := range [][]string{
		{"set", x.Key, "title=Renamed"},
		{"status", x.Key, "in_progress"},
		{"done", x.Key},
		{"dep", "add", x.Key, "--blocked-by", b.Key},
	} {
		code, _, stderr := te.run(args...)
		if code != 1 || !strings.Contains(stderr, "is deleted; restore "+x.Key+" first") {
			t.Errorf("%v: exit %d (want 1), stderr %q", args, code, stderr)
		}
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestRmAndRestore(t *testing.T) {
	te := newStoreEnv(t)
	p, c, g := tree3(te)

	// G is deleted on its own first, then P with what is left under it.
	te.now = te.now.Add(time.Minute)
	te.ok("rm", g.Key)
	te.now = te.now.Add(time.Minute)
	before := len(te.log("me"))
	out := te.ok("rm", p.Key)
	if !strings.Contains(out, "P") || !strings.Contains(out, "1 task under it") {
		t.Errorf("stdout = %q", out)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d: rm is one commit", n, before+1)
	}
	pd, cd, gd := te.task(p.ID).DeletedAt, te.task(c.ID).DeletedAt, te.task(g.ID).DeletedAt
	if pd == nil || cd == nil || gd == nil || !pd.Equal(*cd) || pd.Equal(*gd) {
		t.Fatalf("deleted_at: p %v, c %v, g %v", pd, cd, gd)
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: rm") {
		t.Error("not an rm commit")
	}
	if out := te.ok("ls"); strings.Contains(out, "P") {
		t.Errorf("ls shows deleted tasks:\n%s", out)
	}

	// restore brings back what the same rm deleted, not G.
	te.ok("restore", p.Key)
	if te.task(p.ID).DeletedAt != nil || te.task(c.ID).DeletedAt != nil {
		t.Error("P or C is still deleted")
	}
	if te.task(g.ID).DeletedAt == nil {
		t.Error("G came back, but it was deleted by another rm")
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: restore") {
		t.Error("not a restore commit")
	}
}

func TestRmRestoreErrors(t *testing.T) {
	te := newStoreEnv(t)
	p, c, _ := tree3(te)
	if code, _, stderr := te.run("restore", p.Key); code != 1 || !strings.Contains(stderr, "not deleted") {
		t.Errorf("restore of a live task: exit %d, stderr %q", code, stderr)
	}
	te.ok("rm", p.Key)
	if code, _, stderr := te.run("rm", p.Key); code != 1 || !strings.Contains(stderr, "already deleted") {
		t.Errorf("rm twice: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := te.run("restore", c.Key); code != 1 || !strings.Contains(stderr, "restore "+p.Key+" first") {
		t.Errorf("restore under a deleted parent: exit %d, stderr %q", code, stderr)
	}
}

func TestRmJSON(t *testing.T) {
	te := newStoreEnv(t)
	p, _, _ := tree3(te)
	var got []taskJSON
	if err := json.Unmarshal([]byte(te.ok("rm", p.Key, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != p.ID || got[0].DeletedAt == nil {
		t.Errorf("rm --json = %+v", got)
	}
}

func TestPurge(t *testing.T) {
	te := newStoreEnv(t)
	p, c, g := tree3(te)
	if code, _, stderr := te.run("purge", p.Key, "--yes"); code != 1 || !strings.Contains(stderr, "hedgehug rm") {
		t.Errorf("purge of a live task: exit %d, stderr %q", code, stderr)
	}
	te.ok("rm", g.Key)
	if code, _, stderr := te.run("purge", c.Key, "--yes"); code != 1 || !strings.Contains(stderr, "not deleted") {
		t.Errorf("purge with a live descendant: exit %d, stderr %q", code, stderr)
	}
	te.ok("rm", p.Key)
	if code, _, stderr := te.run("purge", p.Key); code != 2 || !strings.Contains(stderr, "--yes") {
		t.Errorf("purge without --yes off a terminal: exit %d, stderr %q", code, stderr)
	}

	// On a terminal it asks; anything but y keeps the files.
	te.terminal, te.stdin = true, "n\n"
	if code, _, stderr := te.run("purge", p.Key); code != 1 || !strings.Contains(stderr, "3 tasks") {
		t.Errorf("purge answered n: exit %d, stderr %q", code, stderr)
	}
	te.stdin = "y\n"
	te.ok("purge", p.Key)
	te.terminal, te.stdin = false, ""
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: purge") {
		t.Error("not a purge commit")
	}
	if out := te.ok("ls", "is:deleted", "--ids-only"); out != "" {
		t.Errorf("purged tasks are still there: %q", out)
	}
}
