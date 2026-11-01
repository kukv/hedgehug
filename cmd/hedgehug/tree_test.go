package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

func TestLsTree(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	a := te.addJSON("Alpha", "--parent", p.ID)
	te.addJSON("Alpha child", "--parent", a.ID, "--status", "done")
	te.addJSON("Beta", "--parent", p.ID)
	te.addJSON("Loose")

	out := te.ok("ls", "--tree")
	want := []string{
		p.ID + " " + p.ULID + "  todo  project  Project",
		"├─ " + a.ID + " " + a.ULID + "  todo  task  Alpha",
		"│  └─ ",
		"└─ ",
		"Loose",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("tree does not contain %q:\n%s", w, out)
		}
	}
	if strings.Index(out, "Alpha") > strings.Index(out, "Beta") {
		t.Errorf("siblings are not in rank order:\n%s", out)
	}

	// Ancestors that do not match are shown as context.
	out = te.ok("ls", "--tree", "status:done")
	if !strings.Contains(out, "("+p.ID) || !strings.Contains(out, "("+a.ID) || !strings.Contains(out, "Alpha child") {
		t.Errorf("context ancestors are missing or not marked:\n%s", out)
	}
	if strings.Contains(out, "Beta") || strings.Contains(out, "Loose") {
		t.Errorf("tasks that neither match nor connect are shown:\n%s", out)
	}

	var roots []ops.TaskView
	if err := json.Unmarshal([]byte(te.ok("ls", "--tree", "--json", "status:done")), &roots); err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || !roots[0].Context || len(roots[0].Children) != 1 || len(roots[0].Children[0].Children) != 1 || roots[0].Children[0].Children[0].Context {
		t.Errorf("tree JSON = %+v", roots)
	}
}

func TestLsTreeProgress(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	a := te.addJSON("Alpha", "--parent", p.ID, "--status", "done")
	te.addJSON("Beta", "--parent", p.ID)
	te.addJSON("Gamma", "--parent", p.ID, "--status", "canceled")
	te.editWith(te.raw(a.ULID) + "- [x] one\n- [ ] two\n")
	te.ok("edit", a.ID)
	te.goldenOK("tree", "ls", "--tree")
	te.goldenOK("tree-json", "ls", "--tree", "--json")
	te.goldenOK("show", "show", p.ID)
	te.goldenOK("show-a", "show", a.ID)
	te.goldenOK("show-json", "show", a.ID, "--json")
}

func TestLsTreeULIDsAndFormat(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	a := te.addJSON("Alpha", "--parent", p.ID)
	c := te.addJSON("Alpha child", "--parent", a.ID, "--status", "done")

	// --ulids-only prints the tasks found, not the ancestors drawn for them.
	if got := te.ok("ls", "--tree", "--ulids-only", "status:done"); got != c.ULID+"\n" {
		t.Errorf("--tree --ulids-only = %q, want only %s", got, c.ULID)
	}
	// --format gets every drawn task, marked as context or not.
	got := te.ok("ls", "--tree", "--format", "{{.title}} {{.context}} {{.in_cycle}}", "status:done")
	want := "Project true false\nAlpha true false\nAlpha child false false\n"
	if got != want {
		t.Errorf("--tree --format = %q, want %q", got, want)
	}
}

func TestLsTreeULIDsInCycle(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("X")
	y := te.addJSON("Y", "--parent", x.ID, "--status", "done")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	xULID, _ := model.ParseULID(x.ULID)
	yULID, _ := model.ParseULID(y.ULID)
	writeCycle(t, te, s, xULID, yULID)

	if got := te.ok("ls", "--tree", "--ulids-only", "status:done"); got != y.ULID+"\n" {
		t.Errorf("--tree --ulids-only = %q, want only %s", got, y.ULID)
	}
	got := te.ok("ls", "--tree", "--format", "{{.title}} {{.context}} {{.in_cycle}}", "status:done")
	if !strings.Contains(got, "Y false true\n") || !strings.Contains(got, "X true true\n") {
		t.Errorf("--tree --format = %q", got)
	}
}

func TestLsTreeCycle(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("X")
	y := te.addJSON("Y", "--parent", x.ID)
	// A sync can leave a parent cycle that writes refuse; make one by
	// writing the files behind the store's back and committing them.
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	xULID, _ := model.ParseULID(x.ULID)
	yULID, _ := model.ParseULID(y.ULID)
	writeCycle(t, te, s, xULID, yULID)

	out := te.ok("ls", "--tree")
	i := strings.Index(out, "== in a parent cycle")
	if i < 0 || !strings.Contains(out[i:], x.ID) || !strings.Contains(out[i:], y.ID) {
		t.Errorf("the cycle is not listed at the end:\n%s", out)
	}
}

func TestLsTreeCycleNested(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	x := te.addJSON("X")
	y := te.addJSON("Y", "--parent", x.ID)
	c := te.addJSON("C", "--parent", y.ID, "--status", "done")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	writeCycle(t, te, s, mustULID(t, x.ULID), mustULID(t, y.ULID))

	// Text: X and Y at the left under the header, C as Y's child.
	out := te.ok("ls", "--tree")
	_, after, ok := strings.Cut(out, "== in a parent cycle\n")
	if !ok {
		t.Fatalf("no cycle header:\n%s", out)
	}
	lines := strings.Split(strings.TrimRight(after, "\n"), "\n")
	at := map[string]int{}
	for i, l := range lines {
		for _, sh := range []string{x.ID, y.ID, c.ID} {
			if strings.Contains(l, sh) {
				at[sh] = i
			}
		}
	}
	if len(lines) != 3 || len(at) != 3 {
		t.Fatalf("cycle section = %q, want three lines with X, Y and C", lines)
	}
	if !strings.HasPrefix(lines[at[x.ID]], x.ID) || !strings.HasPrefix(lines[at[y.ID]], y.ID) {
		t.Errorf("cycle members are not at the left: %q", lines)
	}
	if at[c.ID] != at[y.ID]+1 || !strings.HasPrefix(lines[at[c.ID]], "└─ "+c.ID) {
		t.Errorf("C is not drawn as Y's child: %q", lines)
	}

	// JSON: the same nesting; in_cycle only on X and Y.
	raw := te.ok("ls", "--tree", "--json")
	var roots []ops.TaskView
	if err := json.Unmarshal([]byte(raw), &roots); err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 {
		t.Fatalf("tree JSON has %d roots, want 2: %+v", len(roots), roots)
	}
	for _, r := range roots {
		if !r.InCycle {
			t.Errorf("%s: in_cycle = false, want true", r.Title)
		}
		switch r.ID {
		case x.ID:
			if len(r.Children) != 0 {
				t.Errorf("X children = %+v, want none", r.Children)
			}
		case y.ID:
			if len(r.Children) != 1 || r.Children[0].ID != c.ID || r.Children[0].InCycle {
				t.Errorf("Y children = %+v, want C out of the cycle", r.Children)
			}
		default:
			t.Errorf("unexpected root %s", r.Title)
		}
	}
	if n := strings.Count(raw, `"in_cycle"`); n != 2 {
		t.Errorf("in_cycle appears %d times in the JSON, want 2", n)
	}

	// --format: in_cycle as in JSON.
	got := te.ok("ls", "--tree", "--format", "{{.title}} {{.context}} {{.in_cycle}}")
	if !strings.Contains(got, "X false true\n") || !strings.Contains(got, "Y false true\nC false false\n") {
		t.Errorf("--tree --format = %q, want X and Y in the cycle and C after Y", got)
	}

	// --ulids-only: only the tasks found, in drawn order.
	if got := te.ok("ls", "--tree", "--ulids-only", "status:done"); got != c.ULID+"\n" {
		t.Errorf("--tree --ulids-only = %q, want only %s", got, c.ULID)
	}
	got = te.ok("ls", "--tree", "--ulids-only")
	if !strings.Contains(got, y.ULID+"\n"+c.ULID+"\n") || strings.Count(got, "\n") != 3 {
		t.Errorf("--tree --ulids-only = %q, want X, Y and C with C after Y", got)
	}
}

func TestLsTreeErrors(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	for _, q := range []string{"group:status", "sort:title"} {
		code, _, stderr := te.run("ls", "--tree", q)
		if code != 2 || !strings.Contains(stderr, "--tree") {
			t.Errorf("ls --tree %s: exit %d, stderr %q", q, code, stderr)
		}
	}
}

// writeCycle makes x the parent of y and y the parent of x, by writing the
// files and committing them with go-git, past the store's checks.
func writeCycle(t *testing.T, te *testEnv, s *store.Store, x, y model.ULID) {
	t.Helper()
	dir := te.storeDir("me")
	for _, pair := range [][2]model.ULID{{x, y}, {y, x}} {
		tk, err := s.Task(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		parent := pair[1]
		tk.Parent = &parent
		b, err := tk.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(model.TaskPath(pair[0]))), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	// Add only the two files: .hedgehug/local/ holds the index cache.
	for _, ulid := range []model.ULID{x, y} {
		if _, err := w.Add(model.TaskPath(ulid)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Commit("make a cycle", &git.CommitOptions{Author: &object.Signature{Name: "T", Email: "t@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
}
