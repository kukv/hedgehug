package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

func TestMv(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	a := te.addJSON("A", "--parent", p.ID)
	b := te.addJSON("B", "--parent", p.ID)
	c := te.addJSON("C", "--parent", p.ID)
	loose := te.addJSON("Loose")

	order := func() string {
		var titles []string
		for _, l := range strings.Split(te.ok("ls", "--tree", "--no-color"), "\n") {
			if i := strings.LastIndex(l, "  "); i >= 0 {
				titles = append(titles, l[i+2:])
			}
		}
		return strings.Join(titles, ",")
	}

	te.ok("mv", c.ID, "--before", a.ID)
	if got := order(); got != "P,C,A,B,Loose" {
		t.Errorf("after --before: %s", got)
	}
	te.ok("mv", c.ID, "--after", b.ID)
	if got := order(); got != "P,A,B,C,Loose" {
		t.Errorf("after --after: %s", got)
	}
	te.ok("mv", loose.ID, "--parent", p.ID, "--after", a.ID)
	if got := order(); got != "P,A,Loose,B,C" {
		t.Errorf("after --parent --after: %s", got)
	}
	if tk := te.task(loose.ULID); tk.Parent == nil || tk.Parent.String() != p.ULID {
		t.Errorf("parent = %v, want %s", tk.Parent, p.ULID)
	}
	te.ok("mv", loose.ID, "--root")
	if got := order(); got != "P,A,B,C,Loose" {
		t.Errorf("after --root: %s", got)
	}
	if te.task(loose.ULID).Parent != nil {
		t.Error("--root left a parent")
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: mv") {
		t.Error("not an mv commit")
	}
}

// A merge can leave siblings with one rank; they are ordered by id and no
// rank lies between them, so mv puts the task past the group on the side it
// was asked for, whatever its own id.
func TestMvNextToSiblingsWithTheSameRank(t *testing.T) {
	for _, tc := range []struct {
		name       string
		moverFirst bool // the mover has the lowest id
		flag, to   string
		want       string
	}{
		{"after A, highest id", false, "--after", "A", "P,A,B,M,D"},
		{"after A, lowest id", true, "--after", "A", "P,A,B,M,D"},
		{"before B, highest id", false, "--before", "B", "P,M,A,B,D"},
		{"before B, lowest id", true, "--before", "B", "P,M,A,B,D"},
		{"after B, lowest id", true, "--after", "B", "P,A,B,M,D"},
		{"before A, highest id", false, "--before", "A", "P,M,A,B,D"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			te := newStoreEnv(t)
			var m ops.TaskView
			if tc.moverFirst {
				m = te.addJSON("M")
			}
			p := te.addJSON("P", "--kind", "project")
			a := te.addJSON("A", "--parent", p.ID)
			b := te.addJSON("B", "--parent", p.ID)
			d := te.addJSON("D", "--parent", p.ID)
			if !tc.moverFirst {
				m = te.addJSON("M")
			}
			te.commitFile(b.ULID, strings.Replace(te.raw(b.ULID), "rank: "+string(te.task(b.ULID).Rank), "rank: "+string(te.task(a.ULID).Rank), 1))
			if te.task(a.ULID).Rank != te.task(b.ULID).Rank {
				t.Fatal("A and B do not share a rank")
			}

			aRank, bRank := te.task(a.ULID).Rank, te.task(b.ULID).Rank
			ref := map[string]string{"A": a.ID, "B": b.ID}[tc.to]
			te.ok("mv", m.ID, "--parent", p.ID, tc.flag, ref)
			var titles []string
			for _, l := range strings.Split(te.ok("ls", "--tree", "--no-color"), "\n") {
				if i := strings.LastIndex(l, "  "); i >= 0 {
					titles = append(titles, l[i+2:])
				}
			}
			if got := strings.Join(titles, ","); got != tc.want {
				t.Errorf("order = %s, want %s", got, tc.want)
			}
			if te.task(a.ULID).Rank != aRank || te.task(b.ULID).Rank != bRank {
				t.Error("mv changed the rank of A or B")
			}
			mr, ar, dr := te.task(m.ULID).Rank, te.task(a.ULID).Rank, te.task(d.ULID).Rank
			if mr == ar || mr == dr {
				t.Errorf("M shares a rank: %s", mr)
			}
		})
	}
}

func TestMvErrors(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	a := te.addJSON("A", "--parent", p.ID)
	loose := te.addJSON("Loose")
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		want     string
	}{
		{"nothing to do", []string{"mv", a.ID}, 2, "--parent"},
		{"--parent and --root", []string{"mv", a.ID, "--parent", p.ID, "--root"}, 2, "--root"},
		{"--before and --after", []string{"mv", a.ID, "--before", p.ID, "--after", p.ID}, 2, "--after"},
		{"not a sibling", []string{"mv", a.ID, "--before", loose.ID}, 1, "not under"},
		{"a cycle", []string{"mv", p.ID, "--parent", a.ID}, 1, "leads back to this task"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := te.run(tc.args...)
			if code != tc.wantCode || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d (want %d), stderr %q (want %q)", code, tc.wantCode, stderr, tc.want)
			}
		})
	}
}

func TestMvRefusesADeletedParent(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	d := te.addJSON("D")
	te.ok("rm", p.ID)
	code, _, stderr := te.run("mv", d.ID, "--parent", p.ID)
	if code != 1 || !strings.Contains(stderr, "is deleted; restore "+p.ID+" first") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if te.task(d.ULID).Parent != nil {
		t.Error("the task was moved")
	}
}

func TestMvWorkspace(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	c := te.addJSON("c", "--parent", p.ID)
	out := te.ok("mv", p.ID, "--workspace", "api")
	if !strings.Contains(out, "ME-1 → API-1") || !strings.Contains(out, "ME-2 → API-2") {
		t.Errorf("mv =\n%s", out)
	}
	got := te.task(c.ULID)
	if got.ID != "API-2" || got.Parent == nil || got.Parent.String() != p.ULID {
		t.Errorf("child after mv: %+v", got)
	}
	other := te.addJSON("o", "--workspace", "me") // ME-1 and ME-2 are not reused
	if other.ID != "ME-3" {
		t.Errorf("id = %s", other.ID)
	}
	te.ok("mv", other.ID, "--parent", p.ID) // into api by its parent
	if got := te.task(other.ULID); got.ID != "API-3" {
		t.Errorf("mv --parent across workspaces: %s", got.ID)
	}
	code, _, stderr := te.run("mv", other.ID, "--parent", p.ID, "--workspace", "me")
	if code != 1 || !strings.Contains(stderr, "the parent is in workspace api") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestMvWorkspaceLeafShowsIDChange(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	x := te.addJSON("x", "--workspace", "me")
	out := te.ok("mv", x.ID, "--workspace", "api")
	if !strings.Contains(out, x.ID+" → API-1") {
		t.Errorf("mv =\n%s", out)
	}
}

func TestMvWorkspaceMovesADeletedDescendant(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	c := te.addJSON("c", "--parent", p.ID)
	te.ok("rm", c.ID)
	te.ok("mv", p.ID, "--workspace", "api")
	got := te.task(c.ULID)
	if got.ID != "API-2" || !slices.Equal(got.FormerIDs, []string{c.ID}) {
		t.Errorf("deleted child: %s %v", got.ID, got.FormerIDs)
	}
}
