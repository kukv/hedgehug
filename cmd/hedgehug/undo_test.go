package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestUndoKeepsLaterChangesOfOthers(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	bob.ok("sync")
	alice.ok("set", "ORG-1", "title=alice")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("status", "ORG-1", "in_progress")
	bob.ok("sync")
	alice.ok("sync")
	first := alice.ok("undo")
	if !strings.Contains(first, "Undid") || !strings.Contains(first, "kept status (changed later by github.com/bob)") {
		t.Errorf("undo = %s", first)
	}
	for _, f := range []string{"kept updated", "kept id", "kept former_ids"} {
		if strings.Contains(first, f) {
			t.Errorf("undo lists %q", f)
		}
	}
	got := alice.ok("show", "ORG-1", "--json")
	if !strings.Contains(got, `"title": "x"`) || !strings.Contains(got, `"status": "in_progress"`) { // twoUsers made ORG-1 with the title x
		t.Errorf("after undo: %s", got)
	}
	out := alice.ok("undo") // the add of ORG-1 by alice: bob changed it later, so the file stays
	if !strings.Contains(out, "Undid") || !strings.Contains(out, "nothing to change") {
		t.Errorf("second undo = %s", out)
	}
	out = alice.ok("undo") // the second one is marked undone though it changed no file
	if strings.TrimSpace(out) != "Nothing to undo" {
		t.Errorf("third undo = %s", out)
	}
}

func TestUndoWritesTheCommit(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("set", "ME-1", "title=y")
	target := te.headCommit("me").Hash.String()
	out := te.ok("undo")
	if want := "Undid " + target[:7] + " set ME-1 y"; !strings.Contains(out, want) {
		t.Errorf("undo = %q, want %q", out, want)
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
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("rm", "ME-1")
	te.ok("undo")
	if got := te.ok("show", "ME-1", "--json"); strings.Contains(got, `"deleted_at": "2`) {
		t.Errorf("still deleted: %s", got)
	}
}

func TestUndoJSON(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	var none struct {
		Undone *string `json:"undone"`
	}
	if err := json.Unmarshal([]byte(te.ok("undo", "--json")), &none); err != nil || none.Undone != nil {
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
	if err := json.Unmarshal([]byte(te.ok("undo", "--json")), &done); err != nil {
		t.Fatal(err)
	}
	if done.Undone == "" || !strings.HasPrefix(done.Subject, "add ME-1") || !done.Changed || len(done.Kept) != 0 {
		t.Errorf("undo = %+v", done)
	}
}

func TestUndoWithoutAccounts(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	if code, _, errs := te.run("undo"); code != 1 || !strings.Contains(errs, "accounts are not set up") {
		t.Errorf("code %d stderr %s", code, errs)
	}
}

func TestUndoNothing(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "ME")
	if out := te.ok("undo"); strings.TrimSpace(out) != "Nothing to undo" {
		t.Errorf("undo = %q", out)
	}
}

func TestUndoMoveAcrossWorkspaces(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	te.ok("store", "init", "me", "--prefix", "WEB", "--workspace", "web")
	te.ok("workspace", "add", "api", "--prefix", "API")
	te.ok("add", "p", "--workspace", "web")    // WEB-1
	te.ok("mv", "WEB-1", "--workspace", "api") // API-1 (former WEB-1)
	te.withOtherWriter("github.com/bob", func() {
		te.ok("add", "c", "--parent", "API-1") // API-2, by someone else after the move
	})
	te.ok("undo")                         // alice's last own operation: the mv
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
