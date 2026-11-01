package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

func TestLs(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha", "-p", "labels+=ui")
	b := te.addJSON("Beta", "--status", "done")

	out := te.ok("ls")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "ID ") || !strings.Contains(lines[0], " ULID ") {
		t.Fatalf("ls =\n%s", out)
	}
	for _, s := range []string{a.ID, a.ULID, "Alpha", "ui", b.ID, b.ULID, "Beta", "done"} {
		if !strings.Contains(out, s) {
			t.Errorf("ls does not show %q:\n%s", s, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("ls colored output that is not a terminal")
	}

	if out := te.ok("ls", "status:done"); !strings.Contains(out, "Beta") || strings.Contains(out, "Alpha") {
		t.Errorf("ls status:done =\n%s", out)
	}
	if out := te.ok("ls", "status:todo", "label:ui"); !strings.Contains(out, "Alpha") || strings.Contains(out, "Beta") {
		t.Errorf("ls with two args =\n%s", out)
	}
	if out := te.ok("ls", "--ulids-only"); out != a.ULID+"\n"+b.ULID+"\n" && out != b.ULID+"\n"+a.ULID+"\n" {
		t.Errorf("--ulids-only = %q", out)
	}
	if out := te.ok("ls", "--format", "{{.title}}|{{.status}}", "sort:title"); out != "Alpha|todo\nBeta|done\n" {
		t.Errorf("--format = %q", out)
	}
	var got []ops.TaskView
	if err := json.Unmarshal([]byte(te.ok("ls", "--json", "sort:title")), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "Alpha" || got[1].Title != "Beta" || got[0].Labels[0] != "ui" {
		t.Errorf("--json = %+v", got)
	}
	if got[1].Assignees == nil || got[1].Problems == nil || got[1].Props == nil || got[1].BlockedBy == nil {
		t.Error("empty lists in JSON must be [] or {}, not null")
	}

	// blocked_by names each task by its ULID, id and title.
	te.ok("dep", "add", a.ID, "--blocked-by", b.ID)
	if out := te.ok("ls", "--format", "{{range .blocked_by}}{{.ulid}} {{.id}} {{.title}}{{end}}", "sort:title"); out != b.ULID+" "+b.ID+" Beta\n\n" {
		t.Errorf("--format over blocked_by = %q", out)
	}
}

// broken is in every task for --format, empty when the task is not broken,
// so that {{if .broken}} works on a key that must be there.
func TestLsFormatBroken(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("Alpha")
	if out := te.ok("ls", "--format", "{{if .broken}}BROKEN {{end}}{{.title}}"); out != "Alpha\n" {
		t.Errorf("ls --format = %q", out)
	}
	if out := te.ok("ls", "--tree", "--format", "{{if .broken}}BROKEN {{end}}{{.title}}"); out != "Alpha\n" {
		t.Errorf("ls --tree --format = %q", out)
	}
	b := te.addJSON("Beta")
	te.commitFile(b.ULID, "not a task\n")
	if out := te.ok("ls", "--format", "{{if .broken}}BROKEN{{else}}{{.title}}{{end}}", "sort:title"); !strings.Contains(out, "BROKEN\n") || !strings.Contains(out, "Alpha\n") {
		t.Errorf("ls --format with a broken task = %q", out)
	}
}

// A broken task has no readable id, so ls shows ? in the ID column and
// "? <ULID>" in the tree.
func TestLsBrokenID(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.commitFile(b.ULID, "not a task\n")
	var found bool
	for _, line := range strings.Split(te.ok("ls"), "\n") {
		if strings.Contains(line, b.ULID) {
			found = true
			if f := strings.Fields(line); f[0] != "?" {
				t.Errorf("ls line of a broken task = %q", line)
			}
		}
	}
	if !found {
		t.Error("ls does not list the broken task")
	}
	if out := te.ok("ls", "--tree"); !strings.Contains(out, "? "+b.ULID) {
		t.Errorf("ls --tree has no %q:\n%s", "? "+b.ULID, out)
	}
}

func TestLsGroups(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("Alpha")
	te.addJSON("Beta", "--status", "done")
	out := te.ok("ls", "group:status")
	if !strings.Contains(out, "== done") || !strings.Contains(out, "== todo") {
		t.Errorf("ls group:status =\n%s", out)
	}
}

func TestLsColor(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.addJSON("Alpha")
	te.terminal = true
	if out := te.ok("ls"); !strings.Contains(out, "\x1b[") {
		t.Errorf("ls on a terminal is not colored:\n%q", out)
	}
	if out := te.ok("ls", "--no-color"); strings.Contains(out, "\x1b[") {
		t.Error("--no-color still colors")
	}
}

func TestLsErrors(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	// A task, so that the templates are run.
	te.addJSON("Alpha")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"ls", "nope:x"}, "nope"},
		{[]string{"ls", "--json", "--ulids-only"}, "only one of"},
		{[]string{"ls", "--ids-only"}, "unknown flag: --ids-only"},
		{[]string{"ls", "--format", "{{"}, "--format"},
		{[]string{"ls", "--format", "{{.nosuch}}"}, "nosuch"},
		{[]string{"ls", "--tree", "--format", "{{"}, "--format"},
		{[]string{"ls", "--tree", "--format", "{{.nosuch}}"}, "nosuch"},
	} {
		code, stdout, stderr := te.run(tc.args...)
		if code != 2 || !strings.Contains(stderr, tc.want) {
			t.Errorf("%v: exit %d, stderr %q (want 2 and %q)", tc.args, code, stderr, tc.want)
		}
		if stdout != "" {
			t.Errorf("%v: stdout = %q, want nothing", tc.args, stdout)
		}
	}
}

func TestUnknownID(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	// An ambiguous input needs two tasks with one id, which only a raw
	// edit makes, so only the not-found case is run here.
	code, _, stderr := te.run("add", "x", "--parent", "zzzzzzzz")
	if code != 1 || !strings.Contains(stderr, "not found") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.handEdit("me", b.ID, "id: "+b.ID, "id: "+a.ID) // two tasks with one id
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(s)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (&ops.Session{}).Resolve(ix, a.ID)
	if ops.KindOf(err) != ops.KindAmbiguous {
		t.Fatalf("Resolve(%s) = %v; want an ambiguous error", a.ID, err)
	}
	got := describe(err)
	if got != err.Error() || !strings.HasPrefix(got, "\""+a.ID+"\" matches 2 tasks:\n  ") {
		t.Errorf("describe = %q", got)
	}
	for _, j := range []ops.TaskView{a, b} {
		if !strings.Contains(got, "\n  "+a.ID+" ("+j.ULID+") ") {
			t.Errorf("describe lacks the candidate %s:\n%s", j.ULID, got)
		}
	}
}
