package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestUndoKeepsLaterChangesOfOthers(t *testing.T) {
	t.Parallel()
	alice, bob, _ := twoUsers(t)
	bob.ok("sync")
	alice.ok("set", "ORG-1", "title=alice")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("status", "ORG-1", "in_progress")
	bob.ok("sync")
	alice.ok("sync")
	alice.goldenOK("first", "undo")
	got := alice.ok("show", "ORG-1", "--json")
	if !strings.Contains(got, `"title": "x"`) || !strings.Contains(got, `"status": "in_progress"`) { // twoUsers made ORG-1 with the title x
		t.Errorf("after undo: %s", got)
	}
	alice.goldenOK("second", "undo") // the add of ORG-1 by alice: bob changed it later, so the file stays
	alice.goldenOK("third", "undo")  // the second one is marked undone though it changed no file
}

func TestUndoWritesTheCommit(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("set", "ME-1", "title=y")
	target := te.headCommit("me").Hash.String()
	if out := te.goldenOK("undo", "undo"); !strings.Contains(out, "Undid "+target[:7]+" set ME-1 y") {
		t.Errorf("undo = %q does not name %s", out, target[:7])
	}
	c := te.headCommit("me")
	for _, want := range []string{"hedgehug: undo set ME-1 y", "Hedgehug-Op: undo", "Hedgehug-Undo: " + target, "Hedgehug-Author: github.com/alice"} {
		if !strings.Contains(c.Message, want) {
			t.Errorf("message lacks %q:\n%s", want, c.Message)
		}
	}
	if got := te.ok("show", "ME-1", "--json"); !strings.Contains(got, `"title": "x"`) {
		t.Errorf("show = %s", got)
	}
}

func TestUndoRm(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("rm", "ME-1")
	te.goldenOK("undo", "undo")
	if got := te.ok("show", "ME-1", "--json"); strings.Contains(got, `"deleted_at": "2`) {
		t.Errorf("still deleted: %s", got)
	}
}

func TestUndoJSON(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	var none struct {
		Undone *string `json:"undone"`
	}
	if err := json.Unmarshal([]byte(te.goldenOK("none", "undo", "--json")), &none); err != nil || none.Undone != nil {
		t.Errorf("nothing to undo: %+v %v", none, err)
	}
	te.ok("add", "x")
	var done struct {
		Undone  string `json:"undone"`
		Subject string `json:"subject"`
		Changed bool   `json:"changed"`
		Kept    []struct {
			Field  string `json:"field"`
			Author string `json:"author"`
		} `json:"kept"`
	}
	if err := json.Unmarshal([]byte(te.goldenOK("done", "undo", "--json")), &done); err != nil {
		t.Fatal(err)
	}
	if done.Undone == "" || !strings.HasPrefix(done.Subject, "add ME-1") || !done.Changed || len(done.Kept) != 0 {
		t.Errorf("undo = %+v", done)
	}
}

func TestUndoWithoutAccounts(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.goldenRun("undo", 1, "undo")
}

func TestUndoNothing(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.goldenOK("undo", "undo")
}

func TestUndoMoveAcrossWorkspaces(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.ok("add", "p", "--workspace", "web")                // WEB-1
	te.goldenOK("mv", "mv", "WEB-1", "--workspace", "api") // API-1 (former WEB-1)
	te.withOtherWriter("github.com/bob", func() {
		te.ok("add", "c", "--parent", "API-1") // API-2, by someone else after the move
	})
	te.goldenOK("undo", "undo")           // alice's last own operation: the mv
	p := te.ok("show", "WEB-1", "--json") // found by its former id
	// p is back in web with a new number; WEB-1 and API-1 stay in former_ids
	if !regexp.MustCompile(`"id": "WEB-2"`).MatchString(p) {
		t.Errorf("p = %s", p)
	}
	if !regexp.MustCompile(`"former_ids": \[\s*"WEB-1",\s*"API-1"\s*\]`).MatchString(p) {
		t.Errorf("former_ids missing from %s", p)
	}
	c := te.ok("show", "API-2", "--json")
	if !strings.Contains(c, `"id": "WEB-3"`) {
		t.Errorf("the child did not follow its parent: %s", c)
	}
}
