package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

// tree3 adds a project with a child and a grandchild.
func tree3(te *testEnv) (p, c, g usecase.TaskView) {
	p = te.addJSON("P", "--kind", "project")
	c = te.addJSON("C", "--parent", p.ID)
	g = te.addJSON("G", "--parent", c.ID)
	return p, c, g
}

// The commands that change a task refuse a deleted one and point to
// restore.
func TestChangeDeletedTask(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("Alpha")
	b := te.addJSON("Blocker")
	te.ok("rm", x.ID)
	before := len(te.log("me"))
	for _, args := range [][]string{
		{"set", x.ID, "title=Renamed"},
		{"status", x.ID, "in_progress"},
		{"done", x.ID},
		{"dep", "add", x.ID, "--blocked-by", b.ID},
	} {
		code, _, stderr := te.run(args...)
		if code != 1 || !strings.Contains(stderr, "is deleted; restore "+x.ID+" first") {
			t.Errorf("%v: exit %d (want 1), stderr %q", args, code, stderr)
		}
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestRmAndRestore(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p, c, g := tree3(te)

	// G is deleted on its own first, then P with what is left under it.
	te.now = te.now.Add(time.Minute)
	te.ok("rm", g.ID)
	te.now = te.now.Add(time.Minute)
	before := len(te.log("me"))
	out := te.ok("rm", p.ID)
	if !strings.Contains(out, "P") || !strings.Contains(out, "1 task under it") {
		t.Errorf("stdout = %q", out)
	}
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d: rm is one commit", n, before+1)
	}
	pd, cd, gd := te.task(p.ULID).DeletedAt, te.task(c.ULID).DeletedAt, te.task(g.ULID).DeletedAt
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
	te.ok("restore", p.ID)
	if te.task(p.ULID).DeletedAt != nil || te.task(c.ULID).DeletedAt != nil {
		t.Error("P or C is still deleted")
	}
	if te.task(g.ULID).DeletedAt == nil {
		t.Error("G came back, but it was deleted by another rm")
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: restore") {
		t.Error("not a restore commit")
	}
}

func TestRmRestoreErrors(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p, c, _ := tree3(te)
	if code, _, stderr := te.run("restore", p.ID); code != 1 || !strings.Contains(stderr, "not deleted") {
		t.Errorf("restore of a live task: exit %d, stderr %q", code, stderr)
	}
	te.ok("rm", p.ID)
	if code, _, stderr := te.run("rm", p.ID); code != 1 || !strings.Contains(stderr, "already deleted") {
		t.Errorf("rm twice: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := te.run("restore", c.ID); code != 1 || !strings.Contains(stderr, "restore "+p.ID+" first") {
		t.Errorf("restore under a deleted parent: exit %d, stderr %q", code, stderr)
	}
}

func TestRmJSON(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p, _, _ := tree3(te)
	var got usecase.AffectedWrite
	if err := json.Unmarshal([]byte(te.ok("rm", p.ID, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Tasks) != 3 || got.Tasks[0].ULID != p.ULID || got.Tasks[0].DeletedAt == nil || got.Commit == nil {
		t.Errorf("rm --json = %+v", got)
	}
}
