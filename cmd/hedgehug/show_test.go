package main

import (
	"encoding/json"
	"strings"
	"testing"

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
	if err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, func(tx *store.Tx) error {
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
	if got.Title != "Write the spec" || got.Body != "The body.\n" || len(got.Comments) != 1 || got.Reverse["blocks"][0].Title != "Waiting" {
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
