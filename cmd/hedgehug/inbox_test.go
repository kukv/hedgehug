package main

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
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

// itemOverwriting returns the ULID of the one unread overwritten item of the field.
func itemOverwriting(t *testing.T, te *testEnv, field string) string {
	t.Helper()
	var found []string
	for _, it := range inboxItems(t, te) {
		if d, _ := it["detail"].(map[string]any); it["kind"] == "overwritten" && d["field"] == field {
			found = append(found, it["ulid"].(string)) //nolint:forcetypeassert // --json prints the ULID as a string
		}
	}
	if len(found) != 1 {
		t.Fatalf("overwritten items of %s = %v", field, found)
	}
	return found[0]
}

func TestInboxOverwrittenAndRevert(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	conflict(t, alice, bob, "title", "bob", "alice")
	bob.goldenOK("inbox", "inbox")
	u := itemOfKind(t, bob, "overwritten")
	bob.goldenOK("revert", "inbox", "revert", u)
	if out := bob.ok("show", "ORG-1", "--json"); !strings.Contains(out, `"title": "bob"`) {
		t.Errorf("after revert: %s", out)
	}
	bob.goldenOK("inbox-after", "inbox")
	if out := bob.ok("inbox", "--all", "--json"); !strings.Contains(out, u) {
		t.Errorf("--all lacks it: %s", out)
	}
	bob.goldenOK("inbox-all-json", "inbox", "--all", "--json")
	if subject := bob.headCommit("org").Message; !strings.HasPrefix(subject, "hedgehug: revert-overwritten ORG-1 bob\n") || !strings.Contains(subject, "Hedgehug-Op: revert-overwritten") {
		t.Errorf("commit = %q", subject)
	}
}

func TestInboxRevertProp(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	conflict(t, alice, bob, "priority", "low", "high")
	bob.goldenOK("inbox", "inbox")
	bob.goldenOK("revert", "inbox", "revert", itemOfKind(t, bob, "overwritten"))
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
	bob.goldenRun("not-overwritten", 1, "inbox", "revert", u)
	bob.goldenRun("not-found", 1, "inbox", "revert", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
	bob.goldenRun("bad-ulid", 2, "inbox", "revert", "ABCD")
}

func TestInboxRenumbered(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("add", "a")
	bob.ok("add", "b")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("inbox", "inbox")
	bob.goldenOK("inbox-json", "inbox", "--json")
}

func TestInboxAssignedAndMention(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("comment", "ORG-1", "look @bob")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("inbox", "inbox")
	items := inboxItems(t, bob)
	if len(items) != 2 { // bob watches the task by being assigned, which is no third notice
		t.Errorf("items = %v", items)
	}
}

func TestInboxReadAll(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("read", "inbox", "read", "--all")
	bob.goldenOK("inbox", "inbox")
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
	bob.goldenOK("read", "inbox", "read", assigned)
	after := inboxItems(t, bob)
	if len(after) != len(before)-1 {
		t.Errorf("after read: %d items, before %d", len(after), len(before))
	}
	for _, it := range after {
		if it["ulid"] == assigned {
			t.Errorf("the item is still unread: %v", it)
		}
	}
	bob.goldenRun("not-found", 1, "inbox", "read", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
}

func TestInboxJSON(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("inbox", "inbox", "--json")
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
	alice.goldenOK("empty", "inbox", "--json")
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
	te.goldenRun("inbox", 0, "inbox")
}

func TestInboxPrefixDuplicate(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("workspace", "add", "web", "--prefix", "DUP")
	time.Sleep(1100 * time.Millisecond) // the prefix set later is the one told
	bob.ok("workspace", "add", "api", "--prefix", "DUP")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("inbox", "inbox")
	bob.goldenOK("inbox-json", "inbox", "--json")
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
	for _, f := range []string{"*", "id", "former_ids", "workspace", "created", "created_by", "body", "unknown_key"} {
		if err := revertField(task, f, json.RawMessage(`"x"`), sc, resolve); !errors.Is(err, errNotRevertible) || !strings.HasPrefix(err.Error(), f+" ") {
			t.Errorf("%s: err = %v", f, err)
		}
	}
	// A set takes the lost item back, or the whole list.
	task.Labels = []string{"a"}
	if err := revertField(task, "labels", json.RawMessage(`"b"`), sc, resolve); err != nil || !slices.Equal(task.Labels, []string{"a", "b"}) {
		t.Errorf("labels = %v, %v", task.Labels, err)
	}
	if err := revertField(task, "labels", json.RawMessage(`"b"`), sc, resolve); err != nil || !slices.Equal(task.Labels, []string{"a", "b"}) {
		t.Errorf("labels = %v, %v", task.Labels, err)
	}
	if err := revertField(task, "assignees", json.RawMessage(`["github.com/bob"]`), sc, resolve); err != nil || !slices.Equal(task.Assignees, []string{"github.com/bob"}) {
		t.Errorf("assignees = %v, %v", task.Assignees, err)
	}
	u := model.NewULID()
	if err := revertField(task, "blocked_by", json.RawMessage(`"`+u.String()+`"`), sc, resolve); err != nil || !slices.Equal(task.BlockedBy, []model.ULID{u}) {
		t.Errorf("blocked_by = %v, %v", task.BlockedBy, err)
	}
	if err := revertField(task, "sprint", json.RawMessage(`"`+u.String()+`"`), sc, resolve); err != nil || task.Sprint == nil || *task.Sprint != u {
		t.Errorf("sprint = %v, %v", task.Sprint, err)
	}
	if err := revertField(task, "sprint", json.RawMessage(`null`), sc, resolve); err != nil || task.Sprint != nil {
		t.Errorf("sprint = %v, %v", task.Sprint, err)
	}
	if err := revertField(task, "rank", json.RawMessage(`"V"`), sc, resolve); err != nil || task.Rank != "V" {
		t.Errorf("rank = %q, %v", task.Rank, err)
	}
	if err := revertField(task, "links", json.RawMessage(`"x"`), sc, resolve); err == nil {
		t.Errorf("a link that is no link was accepted")
	}
}

func TestInboxRevertParent(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("add", "p")
	alice.ok("add", "q")
	alice.ok("add", "c")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("mv", "ORG-4", "--parent", "ORG-2")
	time.Sleep(1100 * time.Millisecond) // alice writes later, so alice's parent wins
	alice.ok("mv", "ORG-4", "--parent", "ORG-3")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("inbox", "inbox")
	bob.goldenOK("revert", "inbox", "revert", itemOfKind(t, bob, "overwritten"))
	if out := bob.ok("show", "ORG-4"); !regexp.MustCompile(`parent\s+ORG-2 `).MatchString(out) {
		t.Errorf("after revert: %s", out)
	}
}

func TestInboxReadDuplicates(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("set", "ORG-1", "assignees+=github.com/bob")
	alice.ok("sync")
	bob.ok("sync")
	u := itemOfKind(t, bob, "assigned")
	if out := bob.ok("inbox", "read", u, strings.ToLower(u)); !strings.Contains(out, "Marked 1 item read") {
		t.Errorf("read = %s", out)
	}
}

func TestInboxRepairedContent(t *testing.T) {
	alice, _, _ := twoUsers(t)
	var j struct{ ULID string }
	if err := json.Unmarshal([]byte(alice.ok("show", "ORG-1", "--json")), &j); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(alice.storeDir("org"))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(st)
	if err != nil {
		t.Fatal(err)
	}
	item := func(kind, value string) inbox.Item {
		return inbox.Item{Kind: inbox.Repaired, Detail: json.RawMessage(`{"kind":"` + kind + `","value":` + value + `}`)}
	}
	name := "ORG-1 (" + j.ULID + ") x"
	for _, c := range []struct{ kind, value, want string }{
		{"parent_cycle", `{"from":"` + j.ULID + `","to":null}`, "parent_cycle: moved back to root (was under " + name + ")"},
		{"parent_cycle", `{"from":null,"to":"` + j.ULID + `"}`, "parent_cycle: moved back to " + name + " (was under the root)"},
		{"parent_deleted", `{"from":"` + j.ULID + `","to":null}`, "parent_deleted: moved to the root (its parent " + name + " was deleted)"},
	} {
		// The destination is the one the value names, whatever the index has now.
		if got := inboxContent(ix, item(c.kind, c.value), "org", false); got != c.want {
			t.Errorf("%s: content = %q, want %q", c.kind, got, c.want)
		}
	}
	gone := model.NewULID().String()
	if got := inboxContent(nil, item("parent_deleted", `{"from":"`+gone+`","to":null}`), "org", false); got != "parent_deleted: moved to the root (its parent "+gone+" was deleted)" {
		t.Errorf("without an index: %q", got)
	}
	if got := inboxContent(ix, item("id_prefix", `"web"`), "org", false); got != `id_prefix (was "web")` {
		t.Errorf("other kinds: %q", got)
	}
}

func TestInboxRevertWorkspace(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("workspace", "add", "w2", "--prefix", "AA")
	alice.ok("workspace", "add", "w3", "--prefix", "BB")
	alice.ok("sync")
	bob.ok("sync")
	bob.ok("mv", "ORG-1", "--workspace", "w2")
	time.Sleep(1100 * time.Millisecond) // alice writes later, so alice's workspace wins
	alice.ok("mv", "ORG-1", "--workspace", "w3")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("inbox", "inbox")
	bob.goldenOK("revert", "inbox", "revert", itemOverwriting(t, bob, "workspace"))
	var x struct{ ULID string }
	if err := json.Unmarshal([]byte(bob.ok("show", "ORG-1", "--json")), &x); err != nil {
		t.Fatal(err)
	}
	if out := bob.ok("show", x.ULID); !regexp.MustCompile(`workspace\s+w2 \(AA, `).MatchString(out) || !regexp.MustCompile(`id\s+AA-\d+\n`).MatchString(out) {
		t.Errorf("after revert: %s", out)
	}
}

func TestInboxRevertParentInAnotherWorkspace(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	alice.ok("workspace", "add", "w2", "--prefix", "AA")
	alice.ok("workspace", "add", "w3", "--prefix", "BB")
	alice.ok("add", "t2", "--workspace", "w2")
	alice.ok("add", "t3", "--workspace", "w3")
	alice.ok("add", "c", "--workspace", "org")
	alice.ok("sync")
	bob.ok("sync")
	var c struct{ ULID string }
	if err := json.Unmarshal([]byte(bob.ok("show", "ORG-2", "--json")), &c); err != nil {
		t.Fatal(err)
	}
	bob.ok("mv", "ORG-2", "--parent", "AA-1")
	time.Sleep(1100 * time.Millisecond) // alice writes later, so alice's parent wins
	alice.ok("mv", "ORG-2", "--parent", "BB-1")
	alice.ok("sync")
	bob.ok("sync")
	bob.goldenOK("inbox", "inbox")
	bob.goldenOK("revert", "inbox", "revert", itemOverwriting(t, bob, "parent"))
	out := bob.ok("show", c.ULID)
	if !regexp.MustCompile(`parent\s+AA-1 `).MatchString(out) || !regexp.MustCompile(`workspace\s+w2 `).MatchString(out) {
		t.Errorf("after revert: %s", out)
	}
}

func TestInboxRevertDeletedTask(t *testing.T) {
	alice, bob, _ := twoUsers(t)
	conflict(t, alice, bob, "title", "bob", "alice")
	u := itemOfKind(t, bob, "overwritten")
	bob.ok("rm", "ORG-1")
	bob.goldenRun("revert", 1, "inbox", "revert", u)
	if got := itemOfKind(t, bob, "overwritten"); got != u {
		t.Errorf("a refused revert marked the item read")
	}
}

func TestInboxDuplicateNamesEveryMatch(t *testing.T) {
	alice, bob, remote := twoUsers(t)
	carol := alice.other()
	carol.writeAccounts("github.com", "carol")
	carol.ok("store", "add", "org", "--remote", remote)
	alice.ok("workspace", "add", "web", "--prefix", "DUP")
	time.Sleep(1100 * time.Millisecond) // the prefix set later is the one told
	bob.ok("workspace", "add", "api", "--prefix", "DUP")
	time.Sleep(1100 * time.Millisecond)
	carol.ok("workspace", "add", "zed", "--prefix", "DUP")
	for _, te := range []*testEnv{alice, bob, carol, bob, alice} {
		te.ok("sync")
	}
	carol.goldenOK("inbox", "inbox")
}

func TestInboxDuplicateNameMatchesTheName(t *testing.T) {
	alice, _, _ := twoUsers(t)
	alice.ok("workspace", "add", "web", "--prefix", "WEB")
	st, err := store.Open(alice.storeDir("org"))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := index.Open(st)
	if err != nil {
		t.Fatal(err)
	}
	var web *model.Workspace
	for _, w := range ix.Workspaces() {
		if w.Name == "web" {
			web = w
		}
	}
	item := func(kind, value string) inbox.Item {
		return inbox.Item{Kind: inbox.PrefixDuplicate, Detail: json.RawMessage(`{"kind":"` + kind + `","workspace":"` + web.ULID.String() + `","value":` + value + `}`)}
	}
	// The workspace of the first store has a prefix and a name of its own: a name
	// nobody else has, and an empty value, name no other workspace.
	if got := inboxContent(ix, item("name_duplicate", `"Web"`), "org", false); got != "Web is also the name of another workspace; change it with hedgehug workspace rename web <new name>" {
		t.Errorf("name: %q", got)
	}
	if got := inboxContent(ix, item("prefix_duplicate", `"WEB"`), "org", false); got != "WEB is also used by another workspace; change it with hedgehug workspace prefix web <new prefix>" {
		t.Errorf("prefix: %q", got)
	}
	if got := inboxContent(ix, item("name_duplicate", `""`), "org", false); !strings.Contains(got, "another workspace") {
		t.Errorf("empty name: %q", got)
	}
}
