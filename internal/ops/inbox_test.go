package ops

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// addItem puts it into the inbox of the test store, as a sync would, and
// returns its ULID.
func (ts *testSession) addItem(it inbox.Item) model.ULID {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	added, err := inbox.Open(s).Add([]inbox.Item{it})
	if err != nil || len(added) != 1 {
		ts.t.Fatalf("Add item = %v, %v", added, err)
	}
	return added[0].ULID
}

func overwritten(task *model.Task, commit, field, value string) inbox.Item {
	return inbox.Item{
		Kind: inbox.Overwritten, Task: task.ULID, ID: task.ID, Path: model.TaskPath(task.ULID), Commit: commit,
		Time:   time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
		Detail: json.RawMessage(`{"field":"` + field + `","value":` + value + `}`),
	}
}

func TestInboxEmptyAndUnknownItem(t *testing.T) {
	ts := newTestSession(t)
	_, items, err := ts.InboxList(false)
	if err != nil || len(items) != 0 {
		t.Fatalf("InboxList = %v, %v; want no items", items, err)
	}
	if _, err := ts.InboxRevert("01JZZZZZZZZZZZZZZZZZZZZZZZ"); KindOf(err) != KindNotFound {
		t.Errorf("unknown item kind = %v; want KindNotFound (%v)", KindOf(err), err)
	}
	if _, err := ts.InboxRead([]string{"01JZZZZZZZZZZZZZZZZZZZZZZZ"}, false); KindOf(err) != KindNotFound {
		t.Errorf("read of an unknown item kind = %v; want KindNotFound (%v)", KindOf(err), err)
	}
	var oe *Error
	_, err = ts.InboxRevert("01JZZZZZZZZZZZZZZZZZZZZZZZ")
	if !errors.As(err, &oe) || oe.Code != "inbox.not_found" || oe.Args["input"] != "01JZZZZZZZZZZZZZZZZZZZZZZZ" {
		t.Errorf("unknown item = %+v; want inbox.not_found with the input", oe)
	}
}

func TestInboxStoresWarnsOfMissingAccounts(t *testing.T) {
	ts := newTestSession(t)
	stores, warnings, err := ts.InboxStores()
	if err != nil || len(stores) != 1 || stores[0].Name != "me" {
		t.Fatalf("InboxStores = %v, %v; want the store me", stores, err)
	}
	if !slices.Equal(warnings, []string{"accounts are not set up in config.yaml; mentions and assignments cannot be told"}) {
		t.Errorf("warnings = %q", warnings)
	}
	if _, _, err := ts.InboxList(false); err != nil || !slices.Equal(ts.warns, warnings) {
		t.Errorf("InboxList warned %q, %v; want %q", ts.warns, err, warnings)
	}
	ts.appendConfig(aliceAccount)
	if _, warnings, err := ts.InboxStores(); err != nil || len(warnings) != 0 {
		t.Errorf("with an account: warnings %q, %v; want none", warnings, err)
	}
}

func TestInboxStoresUnknownStore(t *testing.T) {
	ts := newTestSession(t)
	ts.StoreName = "nope"
	if _, _, err := ts.InboxStores(); err == nil {
		t.Fatal("InboxStores of an unknown store succeeded")
	}
}

func TestSyncTargets(t *testing.T) {
	ts := newTestSession(t)
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if es, err := ts.SyncTargets(c, true); err != nil || len(es) != 1 || es[0].Name != "me" {
		t.Errorf("SyncTargets(all) = %v, %v; want the store me", es, err)
	}
	if es, err := ts.SyncTargets(c, false); err != nil || len(es) != 0 {
		t.Errorf("SyncTargets(remotes only) = %v, %v; want none, as me has no remote", es, err)
	}
	ts.StoreName = "me"
	if es, err := ts.SyncTargets(c, false); err != nil || len(es) != 1 {
		t.Errorf("SyncTargets(named) = %v, %v; want the named store", es, err)
	}
	if _, err := (&Session{}).SyncTargets(&config.Config{}, true); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("SyncTargets without stores error = %v; want ErrNoStores", err)
	}
}

func TestInboxListNewestFirstWithWordsAndJSON(t *testing.T) {
	ts := newTestSession(t)
	task := ts.addTask("title", func(x *model.Task) { x.ID = "ME-1" })
	old := overwritten(task, "c1", "title", `"first"`)
	old.Time = old.Time.Add(-time.Hour)
	oldULID := ts.addItem(old)
	newULID := ts.addItem(overwritten(task, "c2", "status", `"done"`))
	stores, items, err := ts.InboxList(false)
	if err != nil || len(stores) != 1 || len(items) != 2 {
		t.Fatalf("InboxList = %d stores, %d items, %v", len(stores), len(items), err)
	}
	if items[0].ULID != newULID || items[1].ULID != oldULID {
		t.Fatalf("order = %s, %s; want newest first", items[0].ULID, items[1].ULID)
	}
	first := items[0]
	if first.Store != "me" || first.Target != "ME-1 ("+task.ULID.String()+") title" || first.Content != `status: "done" was replaced` {
		t.Errorf("view = store %q target %q content %q", first.Store, first.Target, first.Content)
	}
	if first.Task == nil || first.Task.ID != "ME-1" || first.Task.Title != "title" || first.Task.ULID != task.ULID.String() {
		t.Errorf("task ref = %+v", first.Task)
	}
	b, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["store"] != "me" || m["kind"] != "overwritten" || m["task"] == nil || m["Target"] != nil || m["Content"] != nil || m["target"] != nil || m["content"] != nil {
		t.Errorf("json = %s; want the item's fields with store and task, and no words", b)
	}
	// Read items are listed with all only.
	if _, err := ts.InboxRead([]string{oldULID.String()}, false); err != nil {
		t.Fatal(err)
	}
	if _, items, _ := ts.InboxList(false); len(items) != 1 {
		t.Errorf("unread items = %d; want 1", len(items))
	}
	if _, items, _ := ts.InboxList(true); len(items) != 2 {
		t.Errorf("all items = %d; want 2", len(items))
	}
}

func TestInboxRead(t *testing.T) {
	ts := newTestSession(t)
	task := ts.addTask("t", nil)
	a := ts.addItem(overwritten(task, "c1", "title", `"x"`))
	ts.addItem(overwritten(task, "c2", "title", `"y"`))
	// The same item twice, and in lower case, is one.
	if n, err := ts.InboxRead([]string{a.String(), strings.ToLower(a.String())}, false); err != nil || n != 1 {
		t.Errorf("InboxRead(a, a) = %d, %v; want 1", n, err)
	}
	if n, err := ts.InboxRead(nil, true); err != nil || n != 1 {
		t.Errorf("InboxRead(all) = %d, %v; want the 1 unread item", n, err)
	}
	if n, err := ts.InboxRead(nil, true); err != nil || n != 0 {
		t.Errorf("InboxRead(all) again = %d, %v; want 0", n, err)
	}
}

func TestInboxReadOfSaysWhichItems(t *testing.T) {
	ts := newTestSession(t)
	task := ts.addTask("t", nil)
	u := ts.addItem(overwritten(task, "c1", "title", `"x"`))
	stores, _, err := ts.InboxStores()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.InboxReadOf(stores, []model.ULID{u}, false)
	if err != nil || !slices.Equal(got, []string{u.String()}) {
		t.Errorf("InboxReadOf = %v, %v; want [%s]", got, err, u)
	}
	got, err = ts.InboxReadOf(stores, nil, true)
	if err != nil || len(got) != 0 || got == nil {
		t.Errorf("InboxReadOf(all) after = %v, %v; want an empty list", got, err)
	}
}

func TestParseInboxItem(t *testing.T) {
	u := model.NewULID()
	if got, err := ParseInboxItem(u.String()); err != nil || got != u {
		t.Errorf("ParseInboxItem = %v, %v; want %v", got, err, u)
	}
	_, err := ParseInboxItem("ABCD")
	var oe *Error
	if KindOf(err) != KindUsage || !errors.As(err, &oe) || oe.Code != "inbox.ulid" || oe.Args["input"] != "ABCD" || err.Error() != `"ABCD": give the full ULID of the item` {
		t.Errorf("ParseInboxItem(ABCD) = %v; want a usage error inbox.ulid", err)
	}
	if _, err := ParseInboxItems([]string{u.String(), "ABCD"}); KindOf(err) != KindUsage {
		t.Errorf("ParseInboxItems with a short ULID = %v; want KindUsage", err)
	}
	if got, err := ParseInboxItems([]string{u.String(), strings.ToLower(u.String())}); err != nil || len(got) != 1 {
		t.Errorf("ParseInboxItems twice = %v, %v; want once", got, err)
	}
	if _, err := (&Session{}).InboxRead([]string{"ABCD"}, false); KindOf(err) != KindUsage {
		t.Errorf("InboxRead(ABCD) = %v; want KindUsage before any store is opened", err)
	}
}

func TestInboxRevertRefusals(t *testing.T) {
	ts := newTestSession(t)
	task := ts.addTask("t", nil)
	notOverwritten := ts.addItem(inbox.Item{Kind: inbox.Assigned, Task: task.ULID, Commit: "c1", To: "x"})
	ofWorkspace := ts.addItem(inbox.Item{Kind: inbox.Overwritten, Path: "workspaces/x.yaml", Commit: "c2", Detail: json.RawMessage(`{"field":"name","value":"a"}`)})
	bad := overwritten(task, "c3", "title", `"x"`)
	bad.Detail = json.RawMessage(`"not an object"`)
	undecodable := ts.addItem(bad)
	for name, c := range map[string]struct {
		u    model.ULID
		want string
	}{
		"not overwritten": {notOverwritten, "inbox item " + notOverwritten.String() + " is assigned; only an overwritten item can be reverted"},
		"not a task":      {ofWorkspace, "inbox item " + ofWorkspace.String() + " is about workspaces/x.yaml, which is not a task; only the values of tasks can be reverted"},
	} {
		_, err := ts.InboxRevert(c.u.String())
		var oe *Error
		if KindOf(err) != KindInvalid || !errors.As(err, &oe) || oe.Code != "inbox.not_revertable" || err.Error() != c.want {
			t.Errorf("%s: %v (kind %v); want KindInvalid inbox.not_revertable %q", name, err, KindOf(err), c.want)
		}
	}
	// A detail that does not decode is an inconsistency, not a refusal.
	_, err := ts.InboxRevert(undecodable.String())
	if err == nil || KindOf(err) != KindUnknown || !strings.HasPrefix(err.Error(), "inbox item "+undecodable.String()+" does not decode: ") {
		t.Errorf("undecodable detail = %v (kind %v); want an unwrapped decode error", err, KindOf(err))
	}
	// A field that no write restores.
	id := ts.addItem(overwritten(task, "c4", "id", `"ME-9"`))
	_, err = ts.InboxRevert(id.String())
	if KindOf(err) != KindInvalid || !errors.Is(err, errNotRevertible) || !strings.HasPrefix(err.Error(), "id ") {
		t.Errorf("revert of id = %v; want KindInvalid not revertible", err)
	}
	// A refusal does not mark the item read.
	if _, items, _ := ts.InboxList(false); len(items) != 4 {
		t.Errorf("unread items = %d; want all 4", len(items))
	}
}

func TestInboxRevertOfAGoneOrDeletedTask(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "t"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	e, err := ts.Resolve(ix, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	deleted := ts.addItem(overwritten(&model.Task{ULID: e.ULID, ID: "ME-1"}, "c1", "title", `"x"`))
	_, err = ts.InboxRevert(deleted.String())
	var oe *Error
	if KindOf(err) != KindConflict || !errors.As(err, &oe) || oe.Code != "task.deleted" {
		t.Errorf("revert on a deleted task = %v; want KindConflict task.deleted", err)
	}
	gone := model.NewULID()
	missing := ts.addItem(overwritten(&model.Task{ULID: gone, ID: "ME-7"}, "c2", "title", `"x"`))
	_, err = ts.InboxRevert(missing.String())
	if KindOf(err) != KindNotFound || !errors.As(err, &oe) || oe.Code != "task.gone" || oe.Args["id"] != gone.String() ||
		err.Error() != "the task of inbox item "+missing.String()+" ("+gone.String()+") is gone: not found" {
		t.Errorf("revert on a gone task = %v; want KindNotFound task.gone", err)
	}
}

func TestInboxRevertWritesTheValueBack(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "mine"}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	e, _ := ts.Resolve(ix, "ME-1")
	u := ts.addItem(overwritten(&model.Task{ULID: e.ULID, ID: "ME-1"}, "c1", "title", `"before"`))
	res, err := ts.InboxRevert(u.String())
	if err != nil || !res.Committed || res.ULID != e.ULID || res.Index == nil {
		t.Fatalf("InboxRevert = %+v, %v; want a committed revert of ME-1", res, err)
	}
	if err := res.Index.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got, ok := res.Index.Task(e.ULID); !ok || got.Title != "before" {
		t.Errorf("task after revert = %+v, %v; want the title before", got, ok)
	}
	s, _ := ts.OpenStore()
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Repo().LogFrom(head, 1)
	if err != nil || len(c) != 1 || !strings.HasPrefix(c[0].Subject, "hedgehug: revert-overwritten ME-1 before") {
		t.Errorf("commit = %+v, %v; want a revert-overwritten of ME-1", c, err)
	}
	if _, items, _ := ts.InboxList(false); len(items) != 0 {
		t.Errorf("unread items = %d after the revert; want 0", len(items))
	}
	// Reverting the value that is there writes nothing.
	same := overwritten(&model.Task{ULID: e.ULID, ID: "ME-1"}, "c2", "title", `"before"`)
	same.To = "again" // another item for the same value
	again := ts.addItem(same)
	if res, err := ts.InboxRevert(again.String()); err != nil || res.Committed {
		t.Errorf("revert of the current value = %+v, %v; want no commit", res, err)
	}
}

func TestInboxRevertMoves(t *testing.T) {
	ts := newTestSession(t)
	for _, title := range []string{"p", "c"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	_, ix, _ := ts.Open()
	p, _ := ts.Resolve(ix, "ME-1")
	c, _ := ts.Resolve(ix, "ME-2")
	child := &model.Task{ULID: c.ULID, ID: "ME-2"}
	u := ts.addItem(overwritten(child, "c1", "parent", `"`+p.ULID.String()+`"`))
	if _, err := ts.InboxRevert(u.String()); err != nil {
		t.Fatal(err)
	}
	_, ix, _ = ts.Open()
	if got, _ := ix.Task(c.ULID); got.Parent == nil || *got.Parent != p.ULID {
		t.Errorf("parent after revert = %v; want %s", got.Parent, p.ULID)
	}
	// null is the root.
	root := ts.addItem(overwritten(child, "c2", "parent", `null`))
	if _, err := ts.InboxRevert(root.String()); err != nil {
		t.Fatal(err)
	}
	_, ix, _ = ts.Open()
	if got, _ := ix.Task(c.ULID); got.Parent != nil {
		t.Errorf("parent after revert to null = %v; want the root", got.Parent)
	}
	// A parent that is not in the store, and a workspace that is empty.
	missing := model.NewULID()
	noParent := ts.addItem(overwritten(child, "c3", "parent", `"`+missing.String()+`"`))
	if _, err := ts.InboxRevert(noParent.String()); KindOf(err) != KindNotFound || !strings.HasPrefix(err.Error(), "parent: "+missing.String()+" is not in this store") {
		t.Errorf("revert to a missing parent = %v; want KindNotFound", err)
	}
	noWs := ts.addItem(overwritten(child, "c4", "workspace", `null`))
	_, err := ts.InboxRevert(noWs.String())
	var oe *Error
	if KindOf(err) != KindInvalid || !errors.As(err, &oe) || oe.Code != "inbox.not_revertable" || err.Error() != "workspace: the value is empty" {
		t.Errorf("revert to no workspace = %v; want KindInvalid inbox.not_revertable", err)
	}
	missingWs := ts.addItem(overwritten(child, "c5", "workspace", `"`+missing.String()+`"`))
	if _, err := ts.InboxRevert(missingWs.String()); KindOf(err) != KindNotFound || !strings.HasPrefix(err.Error(), "workspace: "+missing.String()+" is not in this store") {
		t.Errorf("revert to a missing workspace = %v; want KindNotFound", err)
	}
}

func TestInboxContent(t *testing.T) {
	ts := newTestSession(t)
	ts.addTask("t", func(x *model.Task) { x.ID = "ME-1" })
	_, ix, _ := ts.Open()
	ws := ix.Workspaces()[0]
	detail := func(s string) json.RawMessage { return json.RawMessage(s) }
	for _, c := range []struct {
		name string
		it   inbox.Item
		want string
	}{
		{"overwritten", inbox.Item{Kind: inbox.Overwritten, Detail: detail(`{"field":"title","value": "a b"}`), Author: "github.com/bob"}, `title: "a b" was replaced by github.com/bob`},
		{"overwritten that does not decode", inbox.Item{Kind: inbox.Overwritten, Detail: detail(`x`)}, `:  was replaced`},
		{"renumbered", inbox.Item{Kind: inbox.Renumbered, Detail: detail(`{"old":"ME-1","new":"ME-2"}`)}, "ME-1 was renumbered to ME-2; #ME-1 in text still points to the task that kept it"},
		{"mention in the body", inbox.Item{Kind: inbox.Mention, Author: "github.com/bob"}, "mentioned in the body by github.com/bob"},
		{"mention in a comment", inbox.Item{Kind: inbox.Mention, Detail: detail(`{"comment":"01ARZ3NDEKTSV4RRFFQ69G5FAV"}`)}, "mentioned in a comment"},
		{"assigned", inbox.Item{Kind: inbox.Assigned, Author: "github.com/bob"}, "assigned by github.com/bob"},
		{"watch of fields", inbox.Item{Kind: inbox.Watch, Detail: detail(`{"changed":["title","status"]}`), Author: "github.com/bob"}, "title, status changed by github.com/bob"},
		{"watch of a comment", inbox.Item{Kind: inbox.Watch, Detail: detail(`{"comment":"x"}`)}, "new comment"},
		{"revived", inbox.Item{Kind: inbox.Revived}, "deleted by you, but changed on the other side, so it is back"},
		{"repaired", inbox.Item{Kind: inbox.Repaired, Detail: detail(`{"kind":"id_prefix","value":"web"}`)}, `id_prefix (was "web")`},
		{"unknown kind", inbox.Item{Kind: "other", Detail: detail(`{"a":1}`)}, `{"a":1}`},
		{"duplicate name", inbox.Item{Kind: inbox.PrefixDuplicate, Detail: detail(`{"kind":"name_duplicate","workspace":"` + ws.ULID.String() + `","value":"me"}`)}, "me is also the name of another workspace; change it with hedgehug workspace rename me <new name>"},
	} {
		if got := InboxContent(ix, c.it, "me", false); got != c.want {
			t.Errorf("%s: content = %q; want %q", c.name, got, c.want)
		}
	}
}

func TestRepairedContent(t *testing.T) {
	ts := newTestSession(t)
	task := ts.addTask("t", func(x *model.Task) { x.ID = "ME-1" })
	_, ix, _ := ts.Open()
	u := task.ULID.String()
	name := "ME-1 (" + u + ") t"
	item := func(kind, value string) inbox.Item {
		return inbox.Item{Kind: inbox.Repaired, Detail: json.RawMessage(`{"kind":"` + kind + `","value":` + value + `}`)}
	}
	for _, c := range []struct{ kind, value, want string }{
		{"parent_cycle", `{"from":"` + u + `","to":null}`, "parent_cycle: moved back to root (was under " + name + ")"},
		{"parent_cycle", `{"from":null,"to":"` + u + `"}`, "parent_cycle: moved back to " + name + " (was under the root)"},
		{"parent_deleted", `{"from":"` + u + `","to":null}`, "parent_deleted: moved to the root (its parent " + name + " was deleted)"},
		{"id_prefix", `null`, "id_prefix"},
		{"id_prefix", `"web"`, `id_prefix (was "web")`},
	} {
		if got := RepairedContent(ix, item(c.kind, c.value)); got != c.want {
			t.Errorf("%s %s: content = %q; want %q", c.kind, c.value, got, c.want)
		}
	}
	gone := model.NewULID().String()
	if got := RepairedContent(nil, item("parent_deleted", `{"from":"`+gone+`","to":null}`)); got != "parent_deleted: moved to the root (its parent "+gone+" was deleted)" {
		t.Errorf("without an index: %q", got)
	}
	if got := RepairedContent(ix, inbox.Item{Detail: json.RawMessage(`nope`)}); got != "" {
		t.Errorf("undecodable detail: %q; want empty", got)
	}
}

func TestDuplicateContent(t *testing.T) {
	ts := newTestSession(t)
	_, ix, _ := ts.Open()
	w1 := ix.Workspaces()[0]
	other := model.NewULID()
	item := func(kind, ws, value string) inbox.Item {
		return inbox.Item{Kind: inbox.PrefixDuplicate, Detail: json.RawMessage(`{"kind":"` + kind + `","workspace":"` + ws + `","value":` + value + `}`)}
	}
	w := w1.ULID.String()
	for _, c := range []struct {
		name string
		it   inbox.Item
		many bool
		want string
	}{
		{"prefix of one store", item("prefix_duplicate", w, `"ME"`), false, "ME is also used by another workspace; change it with hedgehug workspace prefix me <new prefix>"},
		{"prefix with many stores", item("prefix_duplicate", w, `"ME"`), true, "ME is also used by another workspace; change it with hedgehug --store s1 workspace prefix me <new prefix>"},
		{"prefix of another store", item("prefix_duplicate", w, `{"store":"other","workspace":"w9"}`), false, "ME is also used by workspace w9 in store other; change it with hedgehug workspace prefix me <new prefix>"},
		{"name of one store", item("name_duplicate", w, `"me"`), false, "me is also the name of another workspace; change it with hedgehug workspace rename me <new name>"},
		{"workspace not in the index", item("prefix_duplicate", other.String(), `"ZZ"`), false, "ZZ is also used by another workspace; change it with hedgehug workspace prefix " + other.String() + " <new prefix>"},
	} {
		if got := duplicateContent(ix, c.it, "s1", c.many); got != c.want {
			t.Errorf("%s: %q; want %q", c.name, got, c.want)
		}
	}
	// Workspaces that share the name or the prefix are named.
	// The store refuses a second workspace with the same prefix, but a
	// merge can bring one in.
	w2 := &model.Workspace{ULID: model.NewULID(), Name: "me2", IDPrefix: "ME"}
	_, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data, err := w2.Encode()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.WorkspacePath(w2.ULID)
	if err := os.WriteFile(filepath.Join(e.Path, filepath.FromSlash(rel)), data, 0o600); err != nil {
		t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ = ts.Open()
	if got, want := duplicateContent(ix, item("prefix_duplicate", w, `"ME"`), "s1", false), "ME is also used by workspace me2; change it with hedgehug workspace prefix me <new prefix>"; got != want {
		t.Errorf("shared prefix: %q; want %q", got, want)
	}
	if got, want := duplicateContent(ix, item("name_duplicate", w, `"me2"`), "s1", false), "me2 is also the name of workspace me2; change it with hedgehug workspace rename me <new name>"; got != want {
		t.Errorf("shared name: %q; want %q", got, want)
	}
}

func TestInboxTargetAndTaskRef(t *testing.T) {
	ts := newTestSession(t)
	task := ts.addTask("title", func(x *model.Task) { x.ID = "ME-1" })
	_, ix, _ := ts.Open()
	ws := ix.Workspaces()[0]
	u := task.ULID
	gone := model.NewULID()
	wsItem := inbox.Item{Kind: inbox.Repaired, Detail: json.RawMessage(`{"kind":"x","workspace":"` + ws.ULID.String() + `","value":null}`)}
	for _, c := range []struct {
		name string
		ix   *index.Index
		it   inbox.Item
		want string
	}{
		{"task in the index", ix, inbox.Item{Task: u, ID: "ME-5"}, "ME-1 (" + u.String() + ") title"},
		{"task gone with its id", ix, inbox.Item{Task: gone, ID: "ME-5"}, "ME-5 (" + gone.String() + ")"},
		{"task gone without an id", ix, inbox.Item{Task: gone}, gone.String()},
		{"no index", nil, inbox.Item{Task: gone, ID: "ME-5"}, "ME-5 (" + gone.String() + ")"},
		{"workspace item", ix, wsItem, "workspace me"},
		{"workspace item without an index", nil, wsItem, "-"},
		{"item of nothing", ix, inbox.Item{}, "-"},
	} {
		if got := inboxTarget(c.ix, c.it); got != c.want {
			t.Errorf("target of %s = %q; want %q", c.name, got, c.want)
		}
	}
	if r := inboxTaskRef(ix, inbox.Item{Task: u, ID: "ME-5"}); r == nil || *r != (Ref{ULID: u.String(), ID: "ME-1", Title: "title"}) {
		t.Errorf("ref of a task in the index = %+v", r)
	}
	if r := inboxTaskRef(ix, inbox.Item{Task: gone, ID: "ME-5"}); r == nil || *r != (Ref{ULID: gone.String(), ID: "ME-5"}) {
		t.Errorf("ref of a gone task = %+v; want the item's id and no title", r)
	}
	if r := inboxTaskRef(nil, inbox.Item{Task: gone, ID: "ME-5"}); r == nil || *r != (Ref{ULID: gone.String(), ID: "ME-5"}) {
		t.Errorf("ref without an index = %+v", r)
	}
	if r := inboxTaskRef(ix, inbox.Item{}); r != nil {
		t.Errorf("ref of no task = %+v; want nil", r)
	}
	if w, ok := detailWorkspace(ix, wsItem); !ok || w.ULID != ws.ULID {
		t.Errorf("detailWorkspace = %v, %v; want the workspace", w, ok)
	}
	if _, ok := detailWorkspace(nil, wsItem); ok {
		t.Errorf("detailWorkspace without an index found a workspace")
	}
	if _, ok := detailWorkspace(ix, inbox.Item{Detail: json.RawMessage(`nope`)}); ok {
		t.Errorf("detailWorkspace of an undecodable detail found a workspace")
	}
	if got := taskName(ix, u); got != "ME-1 ("+u.String()+") title" {
		t.Errorf("taskName = %q", got)
	}
	if got := taskName(ix, gone); got != gone.String() {
		t.Errorf("taskName of a gone task = %q; want its ULID", got)
	}
	if got := taskName(nil, u); got != u.String() {
		t.Errorf("taskName without an index = %q; want the ULID", got)
	}
}

func TestPropText(t *testing.T) {
	sc := schema.Default()
	for _, c := range []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"high", "high"},
		{true, "true"},
		{json.Number("3"), "3"},
	} {
		if got, err := propText(sc, "priority", c.in); err != nil || got != c.want {
			t.Errorf("propText(%v) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	_, err := propText(sc, "priority", []any{"a"})
	var oe *Error
	if KindOf(err) != KindInvalid || !errors.As(err, &oe) || oe.Code != "inbox.not_revertable" || !errors.Is(err, errNotRevertible) {
		t.Errorf("propText of a list = %v; want KindInvalid inbox.not_revertable", err)
	}
}

func TestInboxRevertLeavesTheSessionAlone(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	if _, err := ts.Add(AddInput{Title: "mine"}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	e, _ := ts.Resolve(ix, "ME-1")
	task := &model.Task{ULID: e.ULID, ID: "ME-1"}
	u := ts.addItem(overwritten(task, "c1", "title", `"before"`))
	ts.addItem(overwritten(task, "c2", "status", `"done"`))
	if _, err := ts.InboxRevert(u.String()); err != nil {
		t.Fatal(err)
	}
	if ts.StoreName != "" {
		t.Errorf("StoreName = %q after the revert; want it untouched", ts.StoreName)
	}
	if _, items, err := ts.InboxList(false); err != nil || len(items) != 1 || items[0].Content != `status: "done" was replaced` {
		t.Errorf("InboxList after the revert = %d items, %v; want the one left", len(items), err)
	}
}

func TestCompactJSON(t *testing.T) {
	if got := compactJSON(json.RawMessage("{ \"a\": [1, 2] }")); got != `{"a":[1,2]}` {
		t.Errorf("compactJSON = %q", got)
	}
	if got := compactJSON(json.RawMessage("{ nope")); got != "{ nope" {
		t.Errorf("compactJSON of invalid JSON = %q; want it unchanged", got)
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
