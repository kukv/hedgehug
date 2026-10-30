package main

import (
	"slices"
	"strings"
	"testing"
)

func TestSet(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	other := te.addJSON("Other")
	before := len(te.log("me"))

	out := te.ok("set", a.Short, "title=Renamed", "labels+=ui", "labels+=api", "labels-=ui", "due=2026-11-01", "blocked_by+="+other.Short)
	if !strings.Contains(out, "Renamed") {
		t.Errorf("stdout = %q", out)
	}
	tk := te.task(a.ID)
	if tk.Title != "Renamed" || !slices.Equal(tk.Labels, []string{"api"}) || len(tk.BlockedBy) != 1 || tk.BlockedBy[0].String() != other.ID {
		t.Errorf("task = %+v", tk)
	}
	if n := tk.Props.Get("due"); n == nil || n.Value != "2026-11-01" {
		t.Errorf("due = %v", n)
	}
	log := te.log("me")
	if len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: set") {
		t.Errorf("commits = %d (want %d), last %q", len(log), before+1, log[0].Message)
	}

	// A set that changes nothing makes no commit.
	te.ok("set", a.Short, "title=Renamed")
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d after a no-op set, want %d", n, before+1)
	}
}

func TestSetErrors(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	before := len(te.log("me"))
	for _, tc := range []struct {
		name     string
		args     []string
		wantCode int
		want     string
	}{
		{"no assignment", []string{"set", a.Short}, 2, "requires at least 2 arg"},
		{"not key=value", []string{"set", a.Short, "title"}, 2, "key=value"},
		{"parent", []string{"set", a.Short, "parent=x"}, 1, "mv"},
		{"unknown key", []string{"set", a.Short, "nope=1"}, 1, "nope"},
		{"a value not in the enum", []string{"set", a.Short, "priority=nope"}, 1, "breaks the schema"},
		{"an unknown status", []string{"set", a.Short, "status=nope"}, 1, "breaks the schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := te.run(tc.args...)
			if code != tc.wantCode || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d (want %d), stderr %q (want %q)", code, tc.wantCode, stderr, tc.want)
			}
		})
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d: refused sets must not commit", n, before)
	}
}
