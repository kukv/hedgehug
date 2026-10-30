package main

import (
	"slices"
	"strings"
	"testing"
)

func TestMv(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	a := te.addJSON("A", "--parent", p.Key)
	b := te.addJSON("B", "--parent", p.Key)
	c := te.addJSON("C", "--parent", p.Key)
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

	te.ok("mv", c.Key, "--before", a.Key)
	if got := order(); got != "P,C,A,B,Loose" {
		t.Errorf("after --before: %s", got)
	}
	te.ok("mv", c.Key, "--after", b.Key)
	if got := order(); got != "P,A,B,C,Loose" {
		t.Errorf("after --after: %s", got)
	}
	te.ok("mv", loose.Key, "--parent", p.Key, "--after", a.Key)
	if got := order(); got != "P,A,Loose,B,C" {
		t.Errorf("after --parent --after: %s", got)
	}
	if tk := te.task(loose.ID); tk.Parent == nil || tk.Parent.String() != p.ID {
		t.Errorf("parent = %v, want %s", tk.Parent, p.ID)
	}
	te.ok("mv", loose.Key, "--root")
	if got := order(); got != "P,A,B,C,Loose" {
		t.Errorf("after --root: %s", got)
	}
	if te.task(loose.ID).Parent != nil {
		t.Error("--root left a parent")
	}
	if !strings.Contains(te.log("me")[0].Message, "Hedgehug-Op: mv") {
		t.Error("not an mv commit")
	}
}

func TestMvErrors(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	a := te.addJSON("A", "--parent", p.Key)
	loose := te.addJSON("Loose")
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		want     string
	}{
		{"nothing to do", []string{"mv", a.Key}, 2, "--parent"},
		{"--parent and --root", []string{"mv", a.Key, "--parent", p.Key, "--root"}, 2, "--root"},
		{"--before and --after", []string{"mv", a.Key, "--before", p.Key, "--after", p.Key}, 2, "--after"},
		{"not a sibling", []string{"mv", a.Key, "--before", loose.Key}, 1, "not under"},
		{"a cycle", []string{"mv", p.Key, "--parent", a.Key}, 1, "leads back to this task"},
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
	te.ok("rm", p.Key)
	code, _, stderr := te.run("mv", d.Key, "--parent", p.Key)
	if code != 1 || !strings.Contains(stderr, "is deleted; restore "+label(p)+" first") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if te.task(d.ID).Parent != nil {
		t.Error("the task was moved")
	}
}

func TestMvWorkspace(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	c := te.addJSON("c", "--parent", p.Key)
	out := te.ok("mv", p.Key, "--workspace", "api")
	if !strings.Contains(out, "ME-1 → API-1") || !strings.Contains(out, "ME-2 → API-2") {
		t.Errorf("mv =\n%s", out)
	}
	got := te.task(c.ID)
	if got.Key != "API-2" || got.Parent == nil || got.Parent.String() != p.ID {
		t.Errorf("child after mv: %+v", got)
	}
	other := te.addJSON("o", "--workspace", "me") // ME-1 and ME-2 are not reused
	if other.Key != "ME-3" {
		t.Errorf("key = %s", other.Key)
	}
	te.ok("mv", other.Key, "--parent", p.Key) // into api by its parent
	if got := te.task(other.ID); got.Key != "API-3" {
		t.Errorf("mv --parent across workspaces: %s", got.Key)
	}
	code, _, stderr := te.run("mv", other.Key, "--parent", p.Key, "--workspace", "me")
	if code != 1 || !strings.Contains(stderr, "the parent is in workspace api") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestMvWorkspaceLeafShowsKeyChange(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	x := te.addJSON("x", "--workspace", "me")
	out := te.ok("mv", x.Key, "--workspace", "api")
	if !strings.Contains(out, x.Key+" → API-1") {
		t.Errorf("mv =\n%s", out)
	}
}

func TestMvWorkspaceMovesADeletedDescendant(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	c := te.addJSON("c", "--parent", p.Key)
	te.ok("rm", c.Key)
	te.ok("mv", p.Key, "--workspace", "api")
	got := te.task(c.ID)
	if got.Key != "API-2" || !slices.Equal(got.FormerKeys, []string{c.Key}) {
		t.Errorf("deleted child: %s %v", got.Key, got.FormerKeys)
	}
}
