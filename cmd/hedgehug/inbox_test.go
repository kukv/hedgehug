package main

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// inboxItems returns the items of the inbox of te as --json prints them.
func inboxItems(t *testing.T, te *testEnv, args ...string) []map[string]any {
	t.Helper()
	var items []map[string]any
	if err := json.Unmarshal([]byte(te.ok(append([]string{"inbox", "--json"}, args...)...)), &items); err != nil {
		t.Fatal(err)
	}
	return items
}

// conflict makes bob's value of key lose to alice's, which is later, and
// leaves both synced.
func conflict(t *testing.T, alice, bob *testEnv, key, bobs, alices string) {
	t.Helper()
	bob.ok("set", "ORG-1", key+"="+bobs)
	time.Sleep(1100 * time.Millisecond) // commit times are seconds: alice writes later, so alice's value wins
	alice.ok("set", "ORG-1", key+"="+alices)
	alice.ok("sync")
	bob.ok("sync")
	alice.ok("sync")
}

// itemOfKind returns the ULID of the one unread item of the kind.
func itemOfKind(t *testing.T, te *testEnv, kind string) string {
	t.Helper()
	var found []string
	for _, it := range inboxItems(t, te) {
		if it["kind"] == kind {
			found = append(found, it["ulid"].(string)) //nolint:forcetypeassert // --json prints the ULID as a string
		}
	}
	if len(found) != 1 {
		t.Fatalf("items of kind %s = %v", kind, found)
	}
	return found[0]
}

func TestInboxOverwrittenAndRevert(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	conflict(t, alice, bob, "title", "bob", "alice")
	out := bob.ok("inbox")
	m := regexp.MustCompile(`([0-9A-Z]{26})\s+overwritten\s+ORG-1 \([0-9A-Z]{26}\) alice\s+title: "bob" was replaced\n`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("inbox = %s", out)
	}
	bob.ok("inbox", "revert", m[1])
	if out := bob.ok("show", "ORG-1", "--json"); !strings.Contains(out, `"title": "bob"`) {
		t.Errorf("after revert: %s", out)
	}
	if out := bob.ok("inbox"); strings.Contains(out, m[1]) {
		t.Errorf("reverted item is still unread: %s", out)
	}
	if out := bob.ok("inbox", "--all", "--json"); !strings.Contains(out, m[1]) {
		t.Errorf("--all lacks it: %s", out)
	}
	if subject := bob.headCommit("org").Message; !strings.HasPrefix(subject, "hedgehug: revert-overwritten ORG-1 bob\n") || !strings.Contains(subject, "Hedgehug-Op: revert-overwritten") {
		t.Errorf("commit = %q", subject)
	}
}

func TestInboxRevertProp(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	conflict(t, alice, bob, "priority", "low", "high")
	out := bob.ok("inbox")
	m := regexp.MustCompile(`([0-9A-Z]{26})\s+overwritten\s.*priority: "low" was replaced`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("inbox = %s", out)
	}
	bob.ok("inbox", "revert", m[1])
	if out := bob.ok("show", "ORG-1"); !regexp.MustCompile(`priority\s+low\n`).MatchString(out) {
		t.Errorf("after revert: %s", out)
	}
}

func TestInboxRevertErrors(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("sync")
	bob.ok("sync")
	u := itemOfKind(t, bob, "assigned")
	if code, _, errs := bob.run("inbox", "revert", u); code != 1 || !strings.Contains(errs, "only an overwritten item can be reverted") {
		t.Errorf("exit %d, stderr = %s", code, errs)
	}
	if code, _, errs := bob.run("inbox", "revert", "01ARZ3NDEKTSV4RRFFQ69G5FAV"); code != 1 || !strings.Contains(errs, "not found") {
		t.Errorf("exit %d, stderr = %s", code, errs)
	}
	if code, _, _ := bob.run("inbox", "revert", "ABCD"); code != 2 {
		t.Errorf("a short ULID was accepted: exit %d", code)
	}
}

func TestInboxRenumbered(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("add", "a")
	bob.ok("add", "b")
	alice.ok("sync")
	bob.ok("sync")
	if out := bob.ok("inbox"); !regexp.MustCompile(`renumbered\s+ORG-3 \([0-9A-Z]{26}\) b\s+ORG-2 was renumbered to ORG-3; #ORG-2 in text still points to the task that kept it\n`).MatchString(out) {
		t.Errorf("inbox = %s", out)
	}
}

func TestInboxAssignedAndMention(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("comment", "ORG-1", "look @bob")
	alice.ok("sync")
	bob.ok("sync")
	out := bob.ok("inbox")
	for _, re := range []string{
		`assigned\s+ORG-1 \([0-9A-Z]{26}\) x\s+assigned by github.com/alice\n`,
		`mention\s+ORG-1 \([0-9A-Z]{26}\) x\s+mentioned in a comment by github.com/alice\n`,
	} {
		if !regexp.MustCompile(re).MatchString(out) {
			t.Errorf("inbox lacks %s:\n%s", re, out)
		}
	}
}

func TestInboxReadAll(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("inbox", "read", "--all")
	if out := bob.ok("inbox"); strings.TrimSpace(out) != "No unread items" {
		t.Errorf("inbox = %q", out)
	}
	if items := inboxItems(t, bob, "--all"); len(items) == 0 {
		t.Errorf("--all lists nothing after the items were read")
	}
}

func TestInboxRead(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("comment", "ORG-1", "look @bob")
	alice.ok("sync")
	bob.ok("sync")
	before := inboxItems(t, bob)
	assigned := itemOfKind(t, bob, "assigned")
	bob.ok("inbox", "read", assigned)
	after := inboxItems(t, bob)
	if len(after) != len(before)-1 {
		t.Errorf("after read: %d items, before %d", len(after), len(before))
	}
	for _, it := range after {
		if it["ulid"] == assigned {
			t.Errorf("the item is still unread: %v", it)
		}
	}
	if code, _, errs := bob.run("inbox", "read", "01ARZ3NDEKTSV4RRFFQ69G5FAV"); code != 1 || !strings.Contains(errs, "not found") {
		t.Errorf("exit %d, stderr = %s", code, errs)
	}
}

func TestInboxJSON(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("sync")
	bob.ok("sync")
	var it map[string]any
	for _, x := range inboxItems(t, bob) {
		if x["kind"] == "assigned" {
			it = x
		}
	}
	task, _ := it["task"].(map[string]any)
	if it["store"] != "org" || it["author"] != "github.com/alice" || it["to"] != "github.com/bob" || task["id"] != "ORG-1" || task["title"] != "x" || len(task["ulid"].(string)) != 26 { //nolint:forcetypeassert // the ULID of --json is a string
		t.Errorf("item = %v", it)
	}
	if out := alice.ok("inbox", "--json"); strings.TrimSpace(out) != "[]" {
		t.Errorf("an empty inbox as JSON = %q", out)
	}
}

func TestInboxRejectsBadArguments(t *testing.T) {
	_, bob, _ := twoUsers(t)
	for _, args := range [][]string{
		{"inbox", "read", "ABCD"},
		{"inbox", "read"},
		{"inbox", "read", "--all", "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{"inbox", "revert"},
		{"inbox", "extra"},
	} {
		if code, _, _ := bob.run(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}

func TestInboxWithoutAccounts(t *testing.T) {
	te := newTestEnv(t)
	te.ok("store", "init", "me", "--prefix", "ME")
	code, out, errs := te.run("inbox")
	if code != 0 || strings.TrimSpace(out) != "No unread items" || !strings.Contains(errs, "accounts are not set up") {
		t.Errorf("exit %d, stdout = %q, stderr = %s", code, out, errs)
	}
}

func TestInboxPrefixDuplicate(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("workspace", "add", "web", "--prefix", "DUP")
	time.Sleep(1100 * time.Millisecond) // the prefix set later is the one told
	bob.ok("workspace", "add", "api", "--prefix", "DUP")
	alice.ok("sync")
	bob.ok("sync")
	out := bob.ok("inbox")
	if !regexp.MustCompile(`prefix_duplicate\s+workspace api\s+DUP is also used by another workspace; change it with hedgehug workspace prefix api <new prefix>\n`).MatchString(out) {
		t.Errorf("inbox = %s", out)
	}
}

func TestRevertField(t *testing.T) {
	sc := schema.Default()
	resolve := func(string) (model.ULID, error) { return model.ULID{}, errors.New("no tasks here") }
	task := &model.Task{Title: "alice", Links: []model.Link{{Type: "pr", Ref: "o/r#1", Sync: "auto"}}}
	if err := revertField(task, "title", json.RawMessage(`"bob"`), sc, resolve); err != nil || task.Title != "bob" {
		t.Errorf("title = %q, %v", task.Title, err)
	}
	if err := revertField(task, "links", json.RawMessage(`{"type":"pr","ref":"o/r#1","sync":"manual"}`), sc, resolve); err != nil || len(task.Links) != 1 || task.Links[0].Sync != "manual" {
		t.Errorf("links = %+v, %v", task.Links, err)
	}
	if err := revertField(task, "links", json.RawMessage(`{"type":"issue","ref":"o/r#2"}`), sc, resolve); err != nil || len(task.Links) != 2 {
		t.Errorf("links = %+v, %v", task.Links, err)
	}
	if err := revertField(task, "props.priority", json.RawMessage(`"high"`), sc, resolve); err != nil || task.Props.Get("priority") == nil || task.Props.Get("priority").Value != "high" {
		t.Errorf("props = %+v, %v", task.Props, err)
	}
	if err := revertField(task, "props.priority", json.RawMessage(`null`), sc, resolve); err != nil || task.Props.Get("priority") != nil {
		t.Errorf("props = %+v, %v", task.Props, err)
	}
	for _, f := range []string{"*", "rank", "parent", "id", "body"} {
		if err := revertField(task, f, json.RawMessage(`"x"`), sc, resolve); !errors.Is(err, errNotRevertible) {
			t.Errorf("%s: err = %v", f, err)
		}
	}
	if err := revertField(task, "links", json.RawMessage(`"x"`), sc, resolve); err == nil {
		t.Errorf("a link that is no link was accepted")
	}
}
