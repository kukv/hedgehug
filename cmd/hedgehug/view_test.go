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

	"github.com/kukv/hedgehug/internal/ops"
)

func TestViewCommands(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	te.addJSON("Alpha", "--parent", p.ID, "--status", "done")
	te.addJSON("Beta", "--parent", p.ID)

	te.goldenOK("save", "view", "save", "今週", "--layout", "table", "--query", "status:todo", "--columns", "id,title,due", "--group-by", "parent")
	te.goldenOK("save-again", "view", "save", "今週", "--query", "status:todo")
	te.goldenOK("save-tree", "view", "save", "tree", "--query", "kind:task")
	te.goldenOK("list", "view", "list")
	te.goldenOK("list-json", "view", "list", "--json")
	te.goldenOK("show-table", "view", "show", "今週")
	te.goldenOK("show-tree", "view", "show", "tree")
	te.goldenOK("show-table-json", "view", "show", "今週", "--json")
	te.goldenOK("show-tree-json", "view", "show", "tree", "--json")
	te.goldenOK("rename", "view", "rename", "tree", "木")
	te.goldenOK("rm", "view", "rm", "木")
	te.goldenRun("not-found", 1, "view", "show", "木")
	te.goldenRun("bad-name", 2, "view", "save", "a/b")
	te.goldenRun("tree-sort", 2, "view", "save", "x", "--query", "sort:due")
	te.ok("view", "save", "r1", "--query", "status:done")
	te.goldenOK("rename-json", "view", "rename", "r1", "r2", "--json")
	te.goldenOK("rm-json", "view", "rm", "r2", "--json")
	te.commitViewFile("broken", "layout: [\n")
	te.goldenOK("rm-unreadable-json", "view", "rm", "broken", "--json")
	te.ok("view", "save", "badq", "--query", "status:todo")
	te.commitViewFile("badq", strings.Replace(te.viewFile("badq"), "status:todo", "'\"abc'", 1))
	te.goldenRun("show-bad-query", 2, "view", "show", "badq")
	te.ok("view", "save", "Release")
	te.goldenRun("case", 1, "view", "save", "release")
	te.goldenRun("no-sub", 2, "view")

	var v ops.SavedView
	if err := json.Unmarshal([]byte(te.ok("view", "save", "k", "--layout", "kanban", "--kanban-columns", "category", "--swimlane", "assignee", "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if v.Name != "k" || v.Kanban.Columns != "category" || v.Kanban.Swimlane != "assignee" || v.Table.GroupBy != "none" {
		t.Errorf("save --json = %+v", v)
	}
	if c := te.headCommit("me"); !strings.HasPrefix(c.Message, "hedgehug: view save k\n") || !strings.Contains(c.Message, "Hedgehug-Op: view") {
		t.Errorf("commit = %q", c.Message)
	}
}

// viewFile returns the file of the view name in the store me.
func (te *testEnv) viewFile(name string) string {
	te.t.Helper()
	b, err := os.ReadFile(filepath.Join(te.storeDir("me"), "views", name+".yaml")) //nolint:gosec // a file of the test's own store
	if err != nil {
		te.t.Fatal(err)
	}
	return string(b)
}

// commitViewFile writes data to the file of the view name and commits it by hand.
func (te *testEnv) commitViewFile(name, data string) {
	te.t.Helper()
	rel := "views/" + name + ".yaml"
	if err := os.MkdirAll(filepath.Join(te.storeDir("me"), "views"), 0o750); err != nil {
		te.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(te.storeDir("me"), rel), []byte(data), 0o600); err != nil {
		te.t.Fatal(err)
	}
	r, err := git.PlainOpen(te.storeDir("me"))
	if err != nil {
		te.t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		te.t.Fatal(err)
	}
	if _, err := w.Add(rel); err != nil {
		te.t.Fatal(err)
	}
	if _, err := w.Commit("by hand", &git.CommitOptions{Author: &object.Signature{Name: "T", Email: "t@example.com", When: time.Now()}}); err != nil {
		te.t.Fatal(err)
	}
}

func TestViewStoreFlag(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("store", "init", "other", "--prefix", "OT")
	te.ok("--store", "other", "view", "save", "only-other", "--query", "status:todo")
	if _, err := os.Stat(filepath.Join(te.storeDir("other"), "views", "only-other.yaml")); err != nil {
		t.Errorf("the view is not in the named store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(te.storeDir("me"), "views", "only-other.yaml")); err == nil {
		t.Error("the view landed in the other store too")
	}
	if out := te.ok("--store", "other", "view", "list"); !strings.Contains(out, "only-other") {
		t.Errorf("list in other = %q", out)
	}
	if out := te.ok("--store", "me", "view", "list"); strings.Contains(out, "only-other") {
		t.Errorf("list in me = %q", out)
	}
	te.ok("--store", "other", "view", "show", "only-other")
	if code, _, _ := te.run("--store", "me", "view", "show", "only-other"); code == 0 {
		t.Error("show in me found a view of the other store")
	}
}
