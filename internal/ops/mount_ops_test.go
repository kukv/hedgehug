package ops

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/sync"
)

// lastAuthor is who made the latest commit of the store name.
func lastAuthor(t *testing.T, ts *testSession, name string) string {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().Log(nil, 1)
	if err != nil || len(log) != 1 {
		t.Fatalf("log of %s: %v %v", name, log, err)
	}
	return log[0].Author.Name
}

// ulidOf is the ULID of the task arg names.
func ulidOf(t *testing.T, ts *testSession, arg string) model.ULID {
	t.Helper()
	r, err := ts.Show(arg)
	if err != nil {
		t.Fatal(err)
	}
	return r.Entry.ULID
}

func wantCode(t *testing.T, what string, err error, kind Kind, code string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: no error; want %v %s", what, kind, code)
		return
	}
	if e := AsError(err); e.Kind != kind || e.Code != code {
		t.Errorf("%s: %v is %v %q; want %v %q", what, err, e.Kind, e.Code, kind, code)
	}
}

func makeStoreReadOnly(t *testing.T, ts *testSession, name string) {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: "test", Checked: ts.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestSetFindsTheTaskInItsStoreAndWritesAsItsAccount(t *testing.T) {
	ts := newTwoStoreSession(t)
	if _, err := ts.Set("WEB-1", []Assign{{Key: "title", Op: "=", Value: "changed"}}); err != nil {
		t.Fatal(err)
	}
	r, err := ts.Show("WEB-1")
	if err != nil || r.Entry.Title != "changed" {
		t.Fatalf("WEB-1 = %v, %v", r.Entry, err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("author in org-b = %q; want bob", got)
	}
	if got := lastAuthor(t, ts, "org-a"); got != "alice" {
		t.Errorf("author in org-a = %q; want alice", got)
	}
}

func TestShowByIDAndByStorePrefix(t *testing.T) {
	ts := newTwoStoreSession(t)
	r, err := ts.Show("API-1")
	if err != nil || r.Entry.Title != "org-a task" {
		t.Errorf("Show(API-1) = %v, %v", r.Entry, err)
	}
	r, err = ts.Show("org-b:WEB-1")
	if err != nil || r.Entry.Title != "org-b task" {
		t.Errorf("Show(org-b:WEB-1) = %v, %v", r.Entry, err)
	}
	if _, _, err := ts.Log([]string{"WEB-1"}, 0); err != nil {
		t.Errorf("Log(WEB-1): %v", err)
	}
}

func TestShowSameIDInTwoStoresIsAmbiguous(t *testing.T) {
	ts := newTwoStoreSessionWith(t, "XX", "XX")
	_, err := ts.Show("XX-1")
	wantCode(t, "Show(XX-1)", err, KindAmbiguous, "task.ambiguous")
	if err != nil {
		for _, want := range []string{"org-a:XX-1", "org-b:XX-1"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message %q lacks %s", err, want)
			}
		}
		if c, ok := AsError(err).Args["candidates"].([]string); !ok || len(c) != 2 {
			t.Errorf("candidates = %v", AsError(err).Args["candidates"])
		}
	}
	if r, err := ts.Show("org-b:XX-1"); err != nil || r.Entry.Title != "org-b task" {
		t.Errorf("Show(org-b:XX-1) = %v, %v", r.Entry, err)
	}
}

func TestStoreNameKeepsTheLookupInTheStore(t *testing.T) {
	ts := newTwoStoreSession(t)
	ts.StoreName = "org-a"
	_, err := ts.Show("WEB-1")
	wantCode(t, "Show(WEB-1) with --store org-a", err, KindNotFound, "task.not_found")
	_, err = ts.Show("org-b:WEB-1")
	wantCode(t, "Show(org-b:WEB-1) with --store org-a", err, KindUsage, "store.conflict")
	if r, err := ts.Show("org-a:API-1"); err != nil || r.Entry.ID != "API-1" {
		t.Errorf("Show(org-a:API-1) = %v, %v", r.Entry, err)
	}
}

func TestReadOnlyRefusalOrder(t *testing.T) {
	ts := newTwoStoreSession(t)
	makeStoreReadOnly(t, ts, "org-b")
	assign := []Assign{{Key: "title", Op: "=", Value: "x"}}
	_, err := ts.Set("WEB-1", assign)
	wantCode(t, "Set(WEB-1)", err, KindConflict, "store.read_only")
	_, err = ts.Set("WEB-999", assign)
	wantCode(t, "Set(WEB-999)", err, KindNotFound, "task.not_found")
	ts.StoreName = "org-b"
	_, err = ts.Set("WEB-999", assign)
	wantCode(t, "Set(WEB-999) with --store org-b", err, KindConflict, "store.read_only")
	ts.StoreName = ""
	_, err = ts.Set("org-b:WEB-999", assign)
	wantCode(t, "Set(org-b:WEB-999)", err, KindConflict, "store.read_only")
}

func TestSecondIDsStayInsideTheStore(t *testing.T) {
	ts := newTwoStoreSession(t)
	const msg = "org-b's WEB-1 cannot be referred to (references stay inside a store)"
	for _, blocker := range []string{"WEB-1", "org-b:WEB-1"} {
		_, err := ts.Dep("API-1", blocker, true)
		wantCode(t, "Dep(API-1, "+blocker+")", err, KindInvalid, "ref.cross_store")
		if err != nil && err.Error() != msg {
			t.Errorf("message = %q; want %q", err, msg)
		}
		if err != nil && (AsError(err).Args["store"] != "org-b" || AsError(err).Args["id"] != "WEB-1") {
			t.Errorf("args = %v", AsError(err).Args)
		}
	}
	if _, err := ts.ForStore("org-a").Add(AddInput{Title: "second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Dep("API-1", "org-a:API-2", true); err != nil {
		t.Errorf("Dep(API-1, org-a:API-2): %v", err)
	}

	_, err := ts.Set("API-1", []Assign{{Key: "blocked_by", Op: "=", Value: "WEB-1"}})
	wantCode(t, "Set blocked_by=WEB-1", err, KindInvalid, "ref.cross_store")
	_, err = ts.Set("API-1", []Assign{{Key: "relates", Op: "=", Value: "WEB-1"}})
	wantCode(t, "Set relates=WEB-1", err, KindInvalid, "ref.cross_store")
	_, err = ts.Move(MoveInput{Arg: "API-1", Parent: "WEB-1"})
	wantCode(t, "Move --parent WEB-1", err, KindInvalid, "ref.cross_store")
	_, err = ts.Move(MoveInput{Arg: "API-1", Before: "WEB-1"})
	wantCode(t, "Move --before WEB-1", err, KindInvalid, "ref.cross_store")
	_, err = ts.Move(MoveInput{Arg: "API-1", After: "org-b:WEB-1"})
	wantCode(t, "Move --after org-b:WEB-1", err, KindInvalid, "ref.cross_store")
	_, err = ts.ForStore("org-a").Add(AddInput{Title: "child", Parent: "WEB-1"})
	wantCode(t, "Add --parent WEB-1", err, KindInvalid, "ref.cross_store")

	es, err := ts.BeginEdit("API-2")
	if err != nil {
		t.Fatal(err)
	}
	web := ulidOf(t, ts, "WEB-1")
	edited := bytes.Replace(es.Bytes, []byte("kind: task\n"), []byte("kind: task\nblocked_by: ["+web.String()+"]\n"), 1)
	nt, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
	if err != nil {
		t.Fatal(err)
	}
	wantCode(t, "Edit blocked_by: [WEB-1]", ts.PrepareEdit(es, nt), KindInvalid, "ref.cross_store")
}

func TestTwoStepOperationsWriteInTheStoreTheyBeganIn(t *testing.T) {
	ts := newTwoStoreSession(t)
	es, err := ts.BeginEdit("WEB-1")
	if err != nil {
		t.Fatal(err)
	}
	nt, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, bytes.Replace(es.Bytes, []byte("title: org-b task"), []byte("title: edited"), 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.PrepareEdit(es, nt); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.WriteEdit(es, nt); err != nil {
		t.Fatal(err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("WriteEdit author = %q; want bob", got)
	}

	cs, err := ts.BeginComment("WEB-1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ts.PrepareComment(cs, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if p.Comment.Author != "github.com/bob" {
		t.Errorf("comment author = %q; want github.com/bob", p.Comment.Author)
	}
	res, err := ts.WriteComment(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("WriteComment author = %q; want bob", got)
	}

	ces, err := ts.OpenComment(res.Comment.ULID.String())
	if err != nil {
		t.Fatal(err)
	}
	pe, err := ts.PrepareCommentEdit(ces, "changed")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.WriteCommentEdit(pe); err != nil {
		t.Fatal(err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("WriteCommentEdit author = %q; want bob", got)
	}
}

func TestCommentOperationsFindTheCommentInAnyStore(t *testing.T) {
	ts := newTwoStoreSession(t)
	r, err := ts.CommentAdd("WEB-1", "first")
	if err != nil {
		t.Fatal(err)
	}
	id := r.Comment.ULID.String()
	if _, err := ts.CommentEdit(id, "second"); err != nil {
		t.Fatalf("CommentEdit without --store: %v", err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("author = %q; want bob", got)
	}
	if _, err := ts.CommentRemove(id); err != nil {
		t.Errorf("CommentRemove without --store: %v", err)
	}
	_, err = ts.CommentEdit("01JZZZZZZZZZZZZZZZZZZZZZZZ", "x")
	wantCode(t, "CommentEdit of no comment", err, KindNotFound, "comment.not_found")
}

func TestCommentULIDInTwoStoresIsAmbiguous(t *testing.T) {
	ts := newTwoStoreSession(t)
	c := &model.Comment{ULID: model.NewULID(), Body: "same\n"}
	for name, id := range map[string]string{"org-a": "API-1", "org-b": "WEB-1"} {
		sub := ts.ForStore(name)
		cs, err := sub.BeginComment(id)
		if err != nil {
			t.Fatal(err)
		}
		p, err := sub.PrepareComment(cs, "same")
		if err != nil {
			t.Fatal(err)
		}
		p.Comment = c
		if _, err := sub.WriteComment(p); err != nil {
			t.Fatal(err)
		}
	}
	_, err := ts.OpenComment(c.ULID.String())
	wantCode(t, "OpenComment of a ULID in two stores", err, KindAmbiguous, "comment.ambiguous")
}
