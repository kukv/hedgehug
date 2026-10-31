package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

// watchersOf returns the watchers of the task id in the store of te.
func watchersOf(t *testing.T, te *testEnv, id string) []string {
	t.Helper()
	var got struct{ Watchers []string }
	if err := json.Unmarshal([]byte(te.ok("show", id, "--json")), &got); err != nil {
		t.Fatal(err)
	}
	return got.Watchers
}

func TestAutoWatchers(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "x")                                          // ORG-2: the writer watches
	alice.ok("set", "ORG-2", "assignees+=github.com/bob")         // the assignee watches
	alice.ok("comment", "ORG-2", "ping @carol")                   // the mentioned account, on the writer's host, watches
	alice.ok("comment", "ORG-2", "and @example.com/dave, see me") // a mention with a host is kept as written
	want := []string{"github.com/alice", "github.com/bob", "github.com/carol", "example.com/dave"}
	if got := watchersOf(t, alice, "ORG-2"); !slices.Equal(got, want) {
		t.Errorf("watchers = %v, want %v", got, want)
	}
}

func TestAutoWatchersOnAddWithAssignee(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "y", "-p", "assignees=github.com/bob")
	want := []string{"github.com/alice", "github.com/bob"}
	if got := watchersOf(t, alice, "ORG-2"); !slices.Equal(got, want) {
		t.Errorf("watchers = %v, want %v", got, want)
	}
}

func TestAutoWatchersOnEditMention(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "x")
	var j struct{ ULID string }
	if err := json.Unmarshal([]byte(alice.ok("show", "ORG-2", "--json")), &j); err != nil {
		t.Fatal(err)
	}
	u, err := model.ParseULID(j.ULID)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile(filepath.Join(alice.storeDir("org"), filepath.FromSlash(model.TaskPath(u)))) //nolint:gosec // the store of the test
	if err != nil {
		t.Fatal(err)
	}
	body := string(file) + "\nping @erin\n"
	alice.editWith(body)
	alice.ok("edit", "ORG-2")
	want := []string{"github.com/alice", "github.com/erin"}
	if got := watchersOf(t, alice, "ORG-2"); !slices.Equal(got, want) {
		t.Errorf("watchers = %v, want %v", got, want)
	}
}

func TestAutoWatchersOnCommentEdit(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("comment", "ORG-1", "hello")
	var j struct{ Comments []struct{ ULID string } }
	if err := json.Unmarshal([]byte(alice.ok("show", "ORG-1", "--json")), &j); err != nil || len(j.Comments) != 1 {
		t.Fatalf("comments = %+v, %v", j.Comments, err)
	}
	alice.editWith("hello @frank\n")
	alice.ok("comment", "edit", j.Comments[0].ULID)
	want := []string{"github.com/alice", "github.com/frank"}
	if got := watchersOf(t, alice, "ORG-1"); !slices.Equal(got, want) {
		t.Errorf("watchers = %v, want %v", got, want)
	}
}

func TestNoWatcherWithoutAccounts(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	te.ok("add", "x")
	te.ok("comment", "ME-1", "hi @carol")
	if got := watchersOf(t, te, "ME-1"); len(got) != 0 {
		t.Errorf("watchers = %v, want none", got)
	}
	for _, c := range []string{"watch", "unwatch"} {
		te.goldenRun(c, 1, c, "ME-1")
	}
}

func TestWatchUnwatch(t *testing.T) {
	_, bob, _ := twoUsers(t)
	bob.goldenOK("watch", "watch", "ORG-1")
	if out := bob.ok("ls", "watching:@me"); !strings.Contains(out, "ORG-1") {
		t.Errorf("watching = %s", out)
	}
	bob.goldenOK("watch-twice", "watch", "ORG-1")
	bob.goldenOK("unwatch", "unwatch", "ORG-1")
	if out := bob.ok("ls", "watching:@me"); strings.Contains(out, "ORG-1") {
		t.Errorf("still watching: %s", out)
	}
	bob.goldenOK("unwatch-twice", "unwatch", "ORG-1")
}

func TestShowReferencedBy(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("add", "b")
	if out := alice.ok("show", "ORG-1"); strings.Contains(out, "Referenced by") {
		t.Errorf("a section without references: %s", out)
	}
	alice.ok("comment", "ORG-2", "after #ORG-1")
	out := alice.ok("show", "ORG-1")
	if !regexp.MustCompile(`Referenced by:\n\s+ORG-2 \([0-9A-Z]{26}\) b\n`).MatchString(out) {
		t.Errorf("show = %s", out)
	}
	var j struct {
		ReferencedBy []struct{ ULID, ID, Title string } `json:"referenced_by"`
	}
	if err := json.Unmarshal([]byte(alice.ok("show", "ORG-1", "--json")), &j); err != nil {
		t.Fatal(err)
	}
	if len(j.ReferencedBy) != 1 || j.ReferencedBy[0].ID != "ORG-2" || j.ReferencedBy[0].Title != "b" || len(j.ReferencedBy[0].ULID) != 26 {
		t.Errorf("referenced_by = %+v", j.ReferencedBy)
	}
	j.ReferencedBy = nil
	if err := json.Unmarshal([]byte(alice.ok("show", "ORG-2", "--json")), &j); err != nil || j.ReferencedBy == nil || len(j.ReferencedBy) != 0 {
		t.Errorf("an unreferenced task: %+v, %v", j.ReferencedBy, err)
	}
}

func TestCommentEditOnBrokenTask(t *testing.T) {
	te := newStoreEnv(t)
	te.writeAccounts("github.com", "alice")
	x := te.addJSON("Alpha")
	te.ok("comment", x.ULID, "hello")
	var j struct{ Comments []struct{ ULID string } }
	if err := json.Unmarshal([]byte(te.ok("show", x.ULID, "--json")), &j); err != nil || len(j.Comments) != 1 {
		t.Fatalf("comments = %+v, %v", j.Comments, err)
	}
	te.commitFile(x.ULID, "not a task file\n")
	te.editWith("hello @carol\n")
	before := len(te.log("me"))
	te.ok("comment", "edit", j.Comments[0].ULID)
	if log := te.log("me"); len(log) != before+1 || !strings.Contains(log[0].Message, "Hedgehug-Op: comment") {
		t.Errorf("the edit was not committed: %d commits, want %d", len(log), before+1)
	}
}
