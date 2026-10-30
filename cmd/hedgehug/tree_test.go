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
	"github.com/kukv/hedgehug/internal/store"
)

func TestLsTree(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	a := te.addJSON("Alpha", "--parent", p.Short)
	te.addJSON("Alpha child", "--parent", a.Short, "--status", "done")
	te.addJSON("Beta", "--parent", p.Short)
	te.addJSON("Loose")

	out := te.ok("ls", "--tree")
	want := []string{
		p.Short + "  todo  project  Project",
		"├─ " + a.Short + "  todo  task  Alpha",
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
	if !strings.Contains(out, "("+p.Short) || !strings.Contains(out, "("+a.Short) || !strings.Contains(out, "Alpha child") {
		t.Errorf("context ancestors are missing or not marked:\n%s", out)
	}
	if strings.Contains(out, "Beta") || strings.Contains(out, "Loose") {
		t.Errorf("tasks that neither match nor connect are shown:\n%s", out)
	}

	var roots []taskJSON
	if err := json.Unmarshal([]byte(te.ok("ls", "--tree", "--json", "status:done")), &roots); err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || !roots[0].Context || len(roots[0].Children) != 1 || len(roots[0].Children[0].Children) != 1 || roots[0].Children[0].Children[0].Context {
		t.Errorf("tree JSON = %+v", roots)
	}
}

func TestLsTreeCycle(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("X")
	y := te.addJSON("Y", "--parent", x.Short)
	// A sync can leave a parent cycle that writes refuse; make one by
	// writing the files behind the store's back and committing them.
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	xid, _ := model.ParseID(x.ID)
	yid, _ := model.ParseID(y.ID)
	writeCycle(t, te, s, xid, yid)

	out := te.ok("ls", "--tree")
	i := strings.Index(out, "== in a parent cycle")
	if i < 0 || !strings.Contains(out[i:], x.Short) || !strings.Contains(out[i:], y.Short) {
		t.Errorf("the cycle is not listed at the end:\n%s", out)
	}
}

func TestLsTreeErrors(t *testing.T) {
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
func writeCycle(t *testing.T, te *testEnv, s *store.Store, x, y model.ID) {
	t.Helper()
	dir := te.storeDir("me")
	for _, pair := range [][2]model.ID{{x, y}, {y, x}} {
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
	for _, id := range []model.ID{x, y} {
		if _, err := w.Add(model.TaskPath(id)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Commit("make a cycle", &git.CommitOptions{Author: &object.Signature{Name: "T", Email: "t@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
}
