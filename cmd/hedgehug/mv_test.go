package main

import (
	"strings"
	"testing"
)

func TestMv(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("P", "--kind", "project")
	a := te.addJSON("A", "--parent", p.Short)
	b := te.addJSON("B", "--parent", p.Short)
	c := te.addJSON("C", "--parent", p.Short)
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

	te.ok("mv", c.Short, "--before", a.Short)
	if got := order(); got != "P,C,A,B,Loose" {
		t.Errorf("after --before: %s", got)
	}
	te.ok("mv", c.Short, "--after", b.Short)
	if got := order(); got != "P,A,B,C,Loose" {
		t.Errorf("after --after: %s", got)
	}
	te.ok("mv", loose.Short, "--parent", p.Short, "--after", a.Short)
	if got := order(); got != "P,A,Loose,B,C" {
		t.Errorf("after --parent --after: %s", got)
	}
	if tk := te.task(loose.ID); tk.Parent == nil || tk.Parent.String() != p.ID {
		t.Errorf("parent = %v, want %s", tk.Parent, p.ID)
	}
	te.ok("mv", loose.Short, "--root")
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
	a := te.addJSON("A", "--parent", p.Short)
	loose := te.addJSON("Loose")
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		want     string
	}{
		{"nothing to do", []string{"mv", a.Short}, 2, "--parent"},
		{"--parent and --root", []string{"mv", a.Short, "--parent", p.Short, "--root"}, 2, "--root"},
		{"--before and --after", []string{"mv", a.Short, "--before", p.Short, "--after", p.Short}, 2, "--after"},
		{"not a sibling", []string{"mv", a.Short, "--before", loose.Short}, 1, "not under"},
		{"a cycle", []string{"mv", p.Short, "--parent", a.Short}, 1, "leads back to this task"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := te.run(tc.args...)
			if code != tc.wantCode || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d (want %d), stderr %q (want %q)", code, tc.wantCode, stderr, tc.want)
			}
		})
	}
}
