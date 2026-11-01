package ops

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/store"
	isync "github.com/kukv/hedgehug/internal/sync"
)

func TestSessionConcurrentOperations(t *testing.T) {
	ts := newTestSession(t)
	ts.useAccount()
	if _, err := ts.Add(AddInput{Title: "seed"}); err != nil {
		t.Fatal(err)
	}
	ts.resetWriter() // the first of the goroutines reads the writer again
	ops := []func() error{
		func() error { _, err := ts.Add(AddInput{Title: "a"}); return err },
		func() error { _, err := ts.Add(AddInput{Title: "b"}); return err },
		func() error {
			_, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "seed2"}})
			return err
		},
		func() error { _, err := ts.CommentAdd("ME-1", "hello"); return err },
		func() error {
			q, err := ParseQuery("")
			if err != nil {
				return err
			}
			_, err = ts.List(q)
			return err
		},
		func() error { _, _, err := ts.InboxList(false); return err },
		func() error { _, _, err := ts.Writer(); return err },
		func() error { _, err := ts.AccountID(); return err },
	}
	var wg sync.WaitGroup
	errs := make([]error, len(ops))
	for i, op := range ops {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = op()
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("operation %d: %v", i, err)
		}
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(ix.Roots()); n != 3 {
		t.Errorf("tasks after the concurrent adds = %d; want 3", n)
	}
}

// TestForStoreCopiesEveryExportedField fails when a field is added to
// Session and ForStore does not carry it to the session of another store.
func TestForStoreCopiesEveryExportedField(t *testing.T) {
	src := &Session{}
	v := reflect.ValueOf(src).Elem()
	for i := range v.NumField() {
		f, sf := v.Field(i), v.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		switch f.Kind() {
		case reflect.Func:
			f.Set(reflect.MakeFunc(f.Type(), func([]reflect.Value) []reflect.Value {
				out := make([]reflect.Value, f.Type().NumOut())
				for j := range out {
					out[j] = reflect.Zero(f.Type().Out(j))
				}
				return out
			}))
		case reflect.String:
			f.SetString("x")
		case reflect.Pointer:
			f.Set(reflect.New(f.Type().Elem()))
		case reflect.Interface:
			f.Set(reflect.ValueOf(&fakeKeyring{}))
		default:
			t.Fatalf("Session.%s is a %s; teach this test to set it", sf.Name, f.Kind())
		}
	}
	got := reflect.ValueOf(src.ForStore("other")).Elem()
	for i := range got.NumField() {
		sf := got.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		if sf.Name == "StoreName" {
			if got.Field(i).String() != "other" {
				t.Errorf("StoreName = %q; want other", got.Field(i).String())
			}
			continue
		}
		if got.Field(i).IsZero() {
			t.Errorf("ForStore does not copy Session.%s", sf.Name)
		}
		if got.Field(i).Kind() == reflect.Pointer && got.Field(i).Pointer() != v.Field(i).Pointer() {
			t.Errorf("ForStore does not share Session.%s", sf.Name)
		}
	}
	// The tokens are shared: they are of the accounts, not of a store.
	if sub := src.ForStore("other"); sub.tokens == nil || sub.tokens != src.tokens {
		t.Error("ForStore does not share the token cache")
	}
	// The cache of the writer is not carried over.
	ts := newTestSession(t)
	ts.useAccount()
	if _, ok, err := ts.Writer(); err != nil || !ok {
		t.Fatalf("Writer = %v, %v", ok, err)
	}
	if sub := ts.ForStore("me"); sub.writerDone {
		t.Error("ForStore copied the cached writer")
	}
}

func TestAsError(t *testing.T) {
	if AsError(nil) != nil {
		t.Error("AsError(nil) is not nil")
	}
	// The outermost *Error of the chain.
	inner := newErr(KindNotFound, "task.not_found", errors.New("x"), "input", "ME-9")
	if got := AsError(fmt.Errorf("wrapped: %w", inner)); got != inner {
		t.Errorf("AsError(wrapped) = %+v; want the inner *Error", got)
	}
	// A lower-layer error is classified: Kind and Code from the chain.
	cases := []struct {
		err  error
		kind Kind
		code string
	}{
		{fmt.Errorf("x: %w", store.ErrNotFound), KindNotFound, "not_found"},
		{&store.ValidationError{}, KindInvalid, "schema.violation"},
		{store.ErrLocked, KindConflict, "store.locked"},
		{fmt.Errorf("a: %w", isync.ErrDirty), KindConflict, "sync.dirty"},
		{fmt.Errorf("a: %w", isync.ErrUnrelated), KindConflict, "sync.unrelated"},
		{config.ErrNoStores, KindConflict, "store.none"},
		{errors.New("other"), KindUnknown, ""},
	}
	for _, c := range cases {
		got := AsError(c.err)
		if got == nil || got.Kind != c.kind || got.Code != c.code || got.Error() != c.err.Error() {
			t.Errorf("AsError(%v) = %+v; want %v %q with the same message", c.err, got, c.kind, c.code)
		}
		if KindOf(c.err) != c.kind {
			t.Errorf("KindOf(%v) = %v; want %v", c.err, KindOf(c.err), c.kind)
		}
	}
}

// A task that is not found says task.not_found; a store error for something
// else does not.
func TestNotFoundCodes(t *testing.T) {
	ts := newTestSession(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Resolve(ix, "ME-9")
	wantErr(t, err, KindNotFound, "task.not_found")
	if e := AsError(err); e.Args["input"] != "ME-9" {
		t.Errorf("Args = %v; want input ME-9", e.Args)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.TaskBytes(ix.Workspaces()[0].ULID) // no task has this ULID
	wantErr(t, err, KindNotFound, "not_found")
	_, err = ts.OpenComment("01JZZZZZZZZZZZZZZZZZZZZZZZ")
	wantErr(t, err, KindNotFound, "comment.not_found")
	_, err = ResolveWorkspace(ix, "nope")
	wantErr(t, err, KindNotFound, "workspace.not_found")
	_, err = ts.InboxRead([]string{"01JZZZZZZZZZZZZZZZZZZZZZZZ"}, false)
	wantErr(t, err, KindNotFound, "inbox.not_found")
}

func TestEditIsOneCall(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ts.Edit("ME-1", bytes.Replace(es.Bytes, []byte("title: a"), []byte("title: b"), 1))
	if err != nil || !r.Committed || len(r.Commit) != 40 {
		t.Fatalf("Edit = %+v, %v", r, err)
	}
	v, err := r.View()
	if err != nil || v.Title != "b" {
		t.Errorf("View after Edit = %+v, %v; want title b", v, err)
	}
	// The same bytes again change nothing.
	es, err = ts.BeginEdit("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	r, err = ts.Edit("ME-1", es.Bytes)
	if err != nil || r.Committed || r.Commit != "" {
		t.Errorf("Edit of the same bytes = %+v, %v; want no commit", r, err)
	}
	// What CheckEdit refuses, Edit refuses, before anything is written.
	_, err = ts.Edit("ME-1", bytes.Replace(es.Bytes, []byte("id: ME-1"), []byte("id: ME-7"), 1))
	wantErr(t, err, KindInvalid, "edit.invalid")
	_, err = ts.Edit("ME-1", []byte("not a task"))
	wantErr(t, err, KindInvalid, "edit.invalid")
	_, err = ts.Edit("ME-9", es.Bytes)
	wantErr(t, err, KindNotFound, "task.not_found")
}

func TestCommentAddCleansTheBody(t *testing.T) {
	ts := newTestSession(t)
	ts.asAccount("alice")
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.CommentAdd("ME-1", " \t\r\n")
	wantErr(t, err, KindInvalid, "comment.empty")
	r, err := ts.CommentAdd("ME-1", "raw  \n\n")
	if err != nil || r.Comment.Body != "raw\n" || len(r.Commit) != 40 {
		t.Errorf("CommentAdd = %+v, %v; want the body cleaned and a commit hash", r, err)
	}
	// An edit takes a raw body too, and stores it cleaned.
	e, err := ts.CommentEdit(r.Ref.Comment.String(), "again \n\n")
	if err != nil || e.Comment.Body != "again\n" || len(e.Commit) != 40 {
		t.Errorf("CommentEdit = %+v, %v", e, err)
	}
	same, err := ts.CommentEdit(r.Ref.Comment.String(), "again")
	if err != nil || same.Committed || same.Commit != "" {
		t.Errorf("CommentEdit of the same body = %+v, %v; want no commit", same, err)
	}
}

func TestResultsCarryTheCommitAndAFreshView(t *testing.T) {
	ts := newTestSession(t)
	ts.asAccount("alice")
	head := func() string {
		t.Helper()
		s, err := ts.OpenStore()
		if err != nil {
			t.Fatal(err)
		}
		h, err := s.Repo().Head()
		if err != nil {
			t.Fatal(err)
		}
		return h.String()
	}
	add, err := ts.Add(AddInput{Title: "a"})
	if err != nil || add.Commit == "" || add.Commit != head() {
		t.Fatalf("Add = %+v, %v; want Commit = HEAD %s", add, err, head())
	}
	// The index of the result was read before the write; View reads again.
	if e, ok := add.Index.Task(add.ULID); ok {
		t.Fatalf("the index of Add already holds the task: %+v", e)
	}
	if v, err := add.View(); err != nil || v.ID != "ME-1" || v.Title != "a" {
		t.Errorf("Add View = %+v, %v", v, err)
	}
	set, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "b"}})
	if err != nil || set.Commit != head() || set.Commit == add.Commit {
		t.Fatalf("Set = %+v, %v; want a new Commit = HEAD", set, err)
	}
	if v, err := set.View(); err != nil || v.Title != "b" {
		t.Errorf("Set View = %+v, %v; want title b", v, err)
	}
	// Nothing changed: no commit, and HEAD stays.
	before := head()
	noop, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "b"}})
	if err != nil || noop.Committed || noop.Commit != "" || head() != before {
		t.Errorf("no-op Set = %+v, %v", noop, err)
	}
	if v, err := noop.View(); err != nil || v.Title != "b" {
		t.Errorf("no-op View = %+v, %v", v, err)
	}

	if _, err := ts.Add(AddInput{Title: "c", Parent: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	rm, err := ts.Remove("ME-1")
	if err != nil || rm.Commit != head() {
		t.Fatalf("Remove = %+v, %v", rm, err)
	}
	vs, err := rm.Views()
	if err != nil || len(vs) != 2 {
		t.Fatalf("Remove Views = %d, %v; want 2", len(vs), err)
	}
	for _, v := range vs {
		if v.DeletedAt == nil {
			t.Errorf("Views of Remove: %s is not deleted", v.ID)
		}
	}
	rs, err := ts.Restore("ME-1")
	if err != nil || rs.Commit != head() {
		t.Fatalf("Restore = %+v, %v", rs, err)
	}
	if vs, err := rs.Views(); err != nil || len(vs) != 2 || vs[0].DeletedAt != nil || vs[1].DeletedAt != nil {
		t.Errorf("Restore Views = %+v, %v; want both live", vs, err)
	}

	cm, err := ts.CommentAdd("ME-1", "x")
	if err != nil || cm.Commit != head() {
		t.Errorf("CommentAdd = %+v, %v; want Commit = HEAD", cm, err)
	}
	rc, err := ts.CommentRemove(cm.Ref.Comment.String())
	if err != nil || rc.Commit != head() {
		t.Errorf("CommentRemove = %+v, %v; want Commit = HEAD", rc, err)
	}
	mv, err := ts.Move(MoveInput{Arg: "ME-2", Root: true})
	if err != nil || !mv.Committed || mv.Commit != head() {
		t.Errorf("Move = %+v, %v; want Commit = HEAD", mv, err)
	}
	ws, err := ts.WorkspaceAdd("other", "OT")
	if err != nil || ws.Commit != head() {
		t.Errorf("WorkspaceAdd = %+v, %v; want Commit = HEAD", ws, err)
	}
	ts.useAccount()
	ts.resetWriter()
	if _, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "z"}}); err != nil {
		t.Fatal(err)
	}
	un, err := ts.Undo()
	if err != nil || un.NewCommit != head() {
		t.Errorf("Undo = %+v, %v; want NewCommit = HEAD", un, err)
	}
}

func TestSessionUpdateCommitAndBeforeUpdate(t *testing.T) {
	ts := newTestSession(t)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	var before int
	ts.BeforeUpdate = func() { before++ }
	h, err := ts.UpdateCommit(s, store.Op{Name: "noop"}, func(*store.Tx) error { return nil })
	if err != nil || !h.IsZero() {
		t.Errorf("UpdateCommit of nothing = %v, %v; want the zero hash", h, err)
	}
	if before != 1 {
		t.Errorf("BeforeUpdate called %d times; want 1", before)
	}
	ts.BeforeUpdate = nil
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if before != 1 {
		t.Errorf("BeforeUpdate called after it was cleared: %d", before)
	}
}
