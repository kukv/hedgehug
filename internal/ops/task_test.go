package ops

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// addWorkspace adds a workspace to the test store.
func (ts *testSession) addWorkspace(name, prefix string) {
	ts.t.Helper()
	s, _, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	w := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
	if _, err := ts.Update(s, store.Op{Name: "workspace", Workspace: w.ULID}, func(tx *store.Tx) error { return tx.PutWorkspace(w) }); err != nil {
		ts.t.Fatal(err)
	}
}

// deleteTask sets DeletedAt of the task id.
func (ts *testSession) deleteTask(id string) {
	ts.t.Helper()
	s, ix, err := ts.Open()
	if err != nil {
		ts.t.Fatal(err)
	}
	e, err := ts.Resolve(ix, id)
	if err != nil {
		ts.t.Fatal(err)
	}
	now := ts.Now()
	if _, err := ts.Update(s, store.Op{Name: "rm", Task: e.ULID}, func(tx *store.Tx) error {
		t2, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		t2.DeletedAt = &now
		return tx.PutTask(t2)
	}); err != nil {
		ts.t.Fatal(err)
	}
}

// useAccount makes github.com/alice the account of config.yaml.
func (ts *testSession) useAccount() {
	ts.t.Helper()
	ts.appendConfig("accounts:\n  - name: alice\n    host: github.com\n    login: alice\n")
}

// wantErr checks that err has the kind and the code.
func wantErr(t *testing.T, err error, kind Kind, code string) {
	t.Helper()
	e := AsError(err)
	if err == nil {
		t.Fatalf("no error; want %v %q", kind, code)
	}
	if KindOf(err) != kind {
		t.Errorf("KindOf(%v) = %v; want %v", err, KindOf(err), kind)
	}
	if code != "" && e.Code != code {
		t.Errorf("Code of %v = %+v; want %q", err, e, code)
	}
}

func asError(err error, target **Error) bool { return errors.As(err, target) }

func TestAddSetStatusDep(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	a, err := ts.Add(AddInput{Title: "parent"})
	if err != nil || !a.Committed {
		t.Fatalf("Add = %+v, %v", a, err)
	}
	b, err := ts.Add(AddInput{Title: "child", Parent: "ME-1"})
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := b.Index.Task(b.ULID); ok && e.ID != "" {
		t.Errorf("the index before the write has the new task: %+v", e)
	}
	as, err := ParseAssigns([]string{"title=renamed", "labels+=x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("ME-2", as); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.SetStatus("ME-2", "doing"); KindOf(err) != KindInvalid {
		t.Errorf("SetStatus(unknown) kind = %v; want KindInvalid (%v)", KindOf(err), err)
	}
	if _, err := ts.SetStatus("ME-2", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Dep("ME-1", "ME-2", true); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ts.Resolve(ix, "ME-2")
	if err != nil {
		t.Fatal(err)
	}
	if e.Title != "renamed" || e.Status != "done" {
		t.Errorf("ME-2 = %q %q; want renamed done", e.Title, e.Status)
	}
	p, _ := ts.Resolve(ix, "ME-1")
	if len(p.BlockedBy) != 1 || p.BlockedBy[0] != e.ULID {
		t.Errorf("ME-1 blocked_by = %v", p.BlockedBy)
	}
}

func TestSetDeletedIsConflict(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "x"}})
	wantErr(t, err, KindConflict, "task.deleted")
}

func TestParseAssignBadFormIsUsage(t *testing.T) {
	t.Parallel()
	_, err := ParseAssign("novalue")
	wantErr(t, err, KindUsage, "assign.form")
	_, err = ParseAssign("=x")
	wantErr(t, err, KindUsage, "assign.form")
}

func TestApplyAssignErrorKinds(t *testing.T) {
	t.Parallel()
	resolve := func(string) (model.ULID, error) { return model.ULID{}, nil }
	for _, tc := range []struct {
		in   string
		code string
	}{
		{"parent=x", "assign.readonly"},
		{"rank=x", "assign.readonly"},
		{"ulid=x", "assign.readonly"},
		{"nope=1", "assign.unknown_key"},
		{"title+=x", "assign.single"},
		{"due+=2026-11-01", "assign.single"},
		{"labels+=", "assign.missing_value"},
		{"tags-=", "assign.missing_value"},
		{"due=tomorrow", "assign.bad_value"},
		{"points=many", "assign.bad_value"},
		{"flag=maybe", "assign.bad_value"},
	} {
		a, err := ParseAssign(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		task := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "t", Status: "todo"}
		wantErr(t, ApplyAssign(task, a, testSchema(), resolve), KindInvalid, tc.code)
	}
}

func TestAddErrors(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	ts.deleteTask("ME-1")
	_, err := ts.Add(AddInput{Title: "c", Parent: "ME-1"})
	wantErr(t, err, KindConflict, "task.parent_deleted")
	if !strings.Contains(err.Error(), "the parent ME-1") {
		t.Errorf("message = %q", err)
	}

	ts.addWorkspace("other", "OT")
	_, err = ts.Add(AddInput{Title: "d"})
	wantErr(t, err, KindInvalid, "workspace.choose")
	var e *Error
	if !asError(err, &e) || !slices.Equal(e.Args["names"].([]string), []string{"me (ME)", "other (OT)"}) {
		t.Errorf("names = %+v", e)
	}

	if _, err := ts.Add(AddInput{Title: "p", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	_, err = ts.Add(AddInput{Title: "q", Parent: "ME-2", Workspace: "other"})
	wantErr(t, err, KindInvalid, "task.workspace_mismatch")
	if !strings.Contains(err.Error(), "the parent is in workspace me") {
		t.Errorf("message = %q", err)
	}
	_, err = ts.Add(AddInput{Title: "q", Workspace: "nope"})
	wantErr(t, err, KindNotFound, "")
}

func TestSetStatusUnknown(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.SetStatus("ME-1", "nope")
	wantErr(t, err, KindInvalid, "status.unknown")
	if !strings.Contains(err.Error(), `"nope" is not a status (statuses: `) {
		t.Errorf("message = %q", err)
	}
}

func TestDepErrors(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	for _, title := range []string{"a", "b"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := ts.Dep("ME-1", "ME-2", false)
	wantErr(t, err, KindInvalid, "dep.invalid")
	if _, err := ts.Dep("ME-1", "ME-2", true); err != nil {
		t.Fatal(err)
	}
	_, err = ts.Dep("ME-1", "ME-2", true)
	wantErr(t, err, KindInvalid, "dep.invalid")
	if r, err := ts.Dep("ME-1", "ME-2", false); err != nil || !r.Committed {
		t.Errorf("Dep remove = %+v, %v", r, err)
	}
}

func TestWatchAddsAndRemovesMe(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	// With no account in config.yaml there is no "me" to watch with.
	_, err := ts.Watch("ME-1", true)
	if err == nil || !strings.Contains(err.Error(), "accounts are not set up") {
		t.Fatalf("Watch without an account: %v", err)
	}
	var ne *Error
	if KindOf(err) != KindConflict || !errors.As(err, &ne) || ne.Code != "account.none" {
		t.Fatalf("Watch without an account: kind %v, err %v; want KindConflict account.none", KindOf(err), err)
	}

	ts.useAccount()
	ts.resetWriter() // the writer is read once per session; read it again
	watchers := func() []string {
		t.Helper()
		_, ix, err := ts.Open()
		if err != nil {
			t.Fatal(err)
		}
		e, err := ts.Resolve(ix, "ME-1")
		if err != nil {
			t.Fatal(err)
		}
		return e.Watchers
	}
	r, err := ts.Watch("ME-1", true)
	if err != nil || !r.Committed {
		t.Fatalf("Watch = %+v, %v", r, err)
	}
	if got := watchers(); !slices.Equal(got, []string{"github.com/alice"}) {
		t.Errorf("watchers = %v", got)
	}
	if r, err := ts.Watch("ME-1", true); err != nil || r.Committed {
		t.Errorf("Watch again = %+v, %v; want no commit", r, err)
	}
	if r, err := ts.Watch("ME-1", false); err != nil || !r.Committed {
		t.Fatalf("Unwatch = %+v, %v", r, err)
	}
	if got := watchers(); len(got) != 0 {
		t.Errorf("watchers after unwatch = %v", got)
	}
}

func TestSiblingsAndEndRank(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	for _, title := range []string{"a", "b", "c"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Add(AddInput{Title: "kid", Parent: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	ts.deleteTask("ME-3")
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0].ULID
	titles := func(es []*index.Entry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.Title)
		}
		return out
	}
	if got := titles(Siblings(ix, nil, ws, model.ULID{})); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("root siblings = %v; want [a b] (kid is a child, c is deleted)", got)
	}
	b, _ := ts.Resolve(ix, "ME-2")
	if got := titles(Siblings(ix, nil, ws, b.ULID)); !slices.Equal(got, []string{"a"}) {
		t.Errorf("siblings without b = %v", got)
	}
	a, _ := ts.Resolve(ix, "ME-1")
	if got := titles(Siblings(ix, &a.ULID, ws, model.ULID{})); !slices.Equal(got, []string{"kid"}) {
		t.Errorf("children of a = %v", got)
	}
	if got := Siblings(ix, nil, model.NewULID(), model.ULID{}); len(got) != 0 {
		t.Errorf("siblings in another workspace = %v", got)
	}
	r, err := EndRank(ix, nil, ws, model.ULID{})
	if err != nil {
		t.Fatal(err)
	}
	if r <= b.Rank {
		t.Errorf("EndRank = %q; want after %q", r, b.Rank)
	}
	if r, err := EndRank(ix, &b.ULID, ws, model.ULID{}); err != nil || r == "" {
		t.Errorf("EndRank under a task without children = %q, %v", r, err)
	}
}

func TestNotDeletedAndLiveParent(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	e, _ := ts.Resolve(ix, "ME-1")
	if err := NotDeleted(ix, e); err != nil {
		t.Errorf("NotDeleted(live) = %v", err)
	}
	if err := LiveParent(ix, e); err != nil {
		t.Errorf("LiveParent(live) = %v", err)
	}
	ts.deleteTask("ME-1")
	_, ix, _ = ts.Open()
	e, _ = ts.Resolve(ix, "ME-1")
	err := NotDeleted(ix, e)
	wantErr(t, err, KindConflict, "task.deleted")
	if !strings.Contains(err.Error(), "ME-1") || !strings.Contains(err.Error(), "is deleted; restore ME-1 first") {
		t.Errorf("message = %q", err)
	}
	err = LiveParent(ix, e)
	wantErr(t, err, KindConflict, "task.parent_deleted")
	if !strings.HasPrefix(err.Error(), "the parent ME-1") {
		t.Errorf("message = %q", err)
	}
}

func TestResolveWorkspace(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addWorkspace("zz", "OT")
	ts.addWorkspace("ot", "XX") // its name is the prefix of the one before
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if w, err := ResolveWorkspace(ix, "xx"); err != nil || w.Name != "ot" {
		t.Errorf("ResolveWorkspace(zz) = %v, %v", w, err)
	}
	_, err = ResolveWorkspace(ix, "ot")
	wantErr(t, err, KindAmbiguous, "workspace.ambiguous")
	if !strings.Contains(err.Error(), `"ot" matches 2 workspaces:`) {
		t.Errorf("message = %q", err)
	}
	_, err = ResolveWorkspace(ix, "nope")
	wantErr(t, err, KindNotFound, "")
}

func TestWatchInTx(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	task := ts.addTask("a", nil)
	s, _, _ := ts.Open()
	run := func(accts ...string) bool {
		t.Helper()
		committed, err := ts.Update(s, store.Op{Name: "comment", Task: task.ULID}, func(tx *store.Tx) error {
			return WatchInTx(tx, task.ULID, accts...)
		})
		if err != nil {
			t.Fatal(err)
		}
		return committed
	}
	if run() {
		t.Error("no accounts committed")
	}
	if !run("github.com/bob") {
		t.Error("a new watcher did not commit")
	}
	if run("GitHub.com/Bob") {
		t.Error("a watcher that differs in case committed")
	}
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Watchers, []string{"github.com/bob"}) {
		t.Errorf("watchers = %v", got.Watchers)
	}
}

func TestNewMentions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		old, now, host string
		want           []string
	}{
		{"", "ping @carol", "github.com", []string{"github.com/carol"}},
		{"", "see @example.com/dave", "github.com", []string{"example.com/dave"}},
		{"hi @carol", "hi @carol and @bob", "github.com", []string{"github.com/bob"}},
		{"hi @Carol", "hi @carol", "github.com", nil},
		{"", "ping @carol", "", nil}, // no writer host: a bare login names no account
		{"", "ping @example.com/dave", "", []string{"example.com/dave"}},
	} {
		if got := NewMentions(tc.old, tc.now, tc.host); !slices.Equal(got, tc.want) {
			t.Errorf("NewMentions(%q, %q, %q) = %v; want %v", tc.old, tc.now, tc.host, got, tc.want)
		}
	}
}

func TestWriterHost(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if h, err := ts.WriterHost(); err != nil || h != "" {
		t.Errorf("WriterHost without an account = %q, %v", h, err)
	}
	ts.useAccount()
	ts.resetWriter()
	if h, err := ts.WriterHost(); err != nil || h != "github.com" {
		t.Errorf("WriterHost = %q, %v; want github.com", h, err)
	}
}
