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

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestShow(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("Project", "--kind", "project")
	other := te.addJSON("Other")
	a := te.addJSON("Write the spec", "--parent", p.Short, "-p", "blocked_by+="+other.Short, "-p", "due=2026-11-01", "-p", "labels+=ui")
	waiting := te.addJSON("Waiting", "-p", "blocked_by+="+a.Short)

	// Add a body and a comment through the store; the CLI for them is 1d-2.
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	aid, _ := model.ParseID(a.ID)
	if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, func(tx *store.Tx) error {
		tk, err := tx.Task(aid)
		if err != nil {
			return err
		}
		tk.Body = "The body.\n"
		if err := tx.PutTask(tk); err != nil {
			return err
		}
		return tx.PutComment(aid, &model.Comment{ID: model.NewID(), Author: "github.com/alice", Body: "Looks good.\nShip it.\n"})
	}); err != nil {
		t.Fatal(err)
	}

	out := te.ok("show", a.Short)
	for _, want := range []string{
		a.Short + "  Write the spec",
		"parent      " + p.Short + " Project",
		"blocked_by  " + other.Short + " Other",
		"due         2026-11-01",
		"labels      ui",
		"Related:",
		"blocks      " + waiting.Short + " Waiting",
		"The body.",
		"Comments:",
		"github.com/alice",
		"    Looks good.\n    Ship it.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Problems:") || strings.Contains(out, "deleted ") {
		t.Errorf("show has sections it should not:\n%s", out)
	}

	var got struct {
		taskJSON
		Body     string `json:"body"`
		Comments []struct {
			Author, Body string
		} `json:"comments"`
		Reverse map[string][]struct{ Short, Title string } `json:"reverse"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", a.Short, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Write the spec" || got.Body != "The body.\n" || len(got.Comments) != 1 || len(got.Reverse["blocks"]) != 1 || got.Reverse["blocks"][0].Title != "Waiting" {
		t.Errorf("show --json = %+v", got)
	}
}

func TestShowErrors(t *testing.T) {
	te := newStoreEnv(t)
	if code, _, stderr := te.run("show", "zzzzzzzz"); code != 1 || !strings.Contains(stderr, "not found") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if code, _, _ := te.run("show"); code != 2 {
		t.Errorf("show with no id: exit %d, want 2", code)
	}
}

func TestShowBroken(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("X")
	dir := te.storeDir("me")
	rel := model.TaskPath(mustID(t, x.ID))
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte("not a task\x00\n: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add(rel); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Commit("break a task", &git.CommitOptions{Author: &object.Signature{Name: "T", Email: "t@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	out := te.ok("show", x.ID)
	if !strings.Contains(out, "\nbroken: ") {
		t.Errorf("show of a broken task has no broken line:\n%s", out)
	}
	var got struct {
		Broken   string        `json:"broken"`
		Body     string        `json:"body"`
		Comments []interface{} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(te.ok("show", x.ID, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Broken == "" || got.Body != "" || len(got.Comments) != 0 {
		t.Errorf("show --json of a broken task = %+v", got)
	}
}

func TestShowDeleted(t *testing.T) {
	te := newStoreEnv(t)
	x := te.addJSON("X")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	id := mustID(t, x.ID)
	if _, err := s.Update(store.Op{Name: "rm", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, func(tx *store.Tx) error {
		tk, err := tx.Task(id)
		if err != nil {
			return err
		}
		now := time.Now()
		tk.DeletedAt = &now
		return tx.PutTask(tk)
	}); err != nil {
		t.Fatal(err)
	}
	if out := te.ok("show", x.Short); !strings.Contains(out, "  deleted     ") {
		t.Errorf("show of a deleted task has no deleted line:\n%s", out)
	}
}

func TestShowInverseLabel(t *testing.T) {
	te := newStoreEnv(t)
	other := te.addJSON("Other")
	a := te.addJSON("A", "-p", "duplicates+="+other.Short)
	out := te.ok("show", other.Short)
	if want := "重複されている     " + a.Short + " A"; !strings.Contains(out, want) {
		t.Errorf("show does not contain %q:\n%s", want, out)
	}
}

func mustID(t *testing.T, s string) model.ID {
	t.Helper()
	id, err := model.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
