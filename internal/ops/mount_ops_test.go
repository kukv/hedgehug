package ops

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"

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
	t.Parallel()
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
	t.Parallel()
	ts := newTwoStoreSession(t)
	r, err := ts.Show("API-1")
	if err != nil || r.Entry.Title != "org-a task" {
		t.Errorf("Show(API-1) = %v, %v", r.Entry, err)
	}
	r, err = ts.Show("org-b:WEB-1")
	if err != nil || r.Entry.Title != "org-b task" {
		t.Errorf("Show(org-b:WEB-1) = %v, %v", r.Entry, err)
	}
	if _, err := ts.Log([]string{"WEB-1"}, 0); err != nil {
		t.Errorf("Log(WEB-1): %v", err)
	}
}

func TestShowSameIDInTwoStoresIsAmbiguous(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// commits is the number of commits of the store name.
func commits(t *testing.T, ts *testSession, name string) int {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return len(log)
}

// handedOver is an operation on a task or comment that takes its id.
type handedOver struct {
	name string
	task bool // takes the id of a task (else a comment)
	run  func(ts *testSession, id string) error
	read bool // reads only
}

func handedOverOps(t *testing.T) []handedOver {
	t.Helper()
	return []handedOver{
		{name: "Show", task: true, read: true, run: func(ts *testSession, id string) error { _, err := ts.Show(id); return err }},
		{name: "Log", task: true, read: true, run: func(ts *testSession, id string) error { _, err := ts.Log([]string{id}, 0); return err }},
		{name: "Set", task: true, run: func(ts *testSession, id string) error {
			_, err := ts.Set(id, []Assign{{Key: "title", Op: "=", Value: "set " + id}})
			return err
		}},
		{name: "SetStatus", task: true, run: func(ts *testSession, id string) error { _, err := ts.SetStatus(id, "in_progress"); return err }},
		{name: "Watch", task: true, run: func(ts *testSession, id string) error { _, err := ts.Watch(id, false); return err }},
		{name: "Dep", task: true, run: func(ts *testSession, id string) error { _, err := ts.Dep(id, "org-b:WEB-2", true); return err }},
		{name: "Move", task: true, run: func(ts *testSession, id string) error {
			_, err := ts.Move(MoveInput{Arg: id, After: "WEB-2"})
			return err
		}},
		{name: "Edit", task: true, run: func(ts *testSession, id string) error {
			es, err := ts.BeginEdit(id)
			if err != nil {
				return err
			}
			_, err = ts.Edit(id, bytes.Replace(es.Bytes, []byte("title: "), []byte("title: edited "), 1))
			return err
		}},
		{name: "Remove", task: true, run: func(ts *testSession, id string) error { _, err := ts.Remove(id); return err }},
		{name: "Restore", task: true, run: func(ts *testSession, id string) error { _, err := ts.Restore(id); return err }},
		{name: "RestoreAt", task: true, run: func(ts *testSession, id string) error {
			_, err := ts.RestoreAt(id, "2026-10-30T12:00:00Z")
			return err
		}},
		{name: "BeginComment", task: true, run: func(ts *testSession, id string) error { _, err := ts.BeginComment(id); return err }},
		{name: "CommentAdd", task: true, run: func(ts *testSession, id string) error { _, err := ts.CommentAdd(id, "hi"); return err }},
	}
}

func TestEveryOperationOnAnIDGoesToTheStoreOfTheTask(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	if _, err := ts.ForStore("org-b").Add(AddInput{Title: "second"}); err != nil {
		t.Fatal(err)
	}
	for _, op := range handedOverOps(t) {
		a, b := commits(t, ts, "org-a"), commits(t, ts, "org-b")
		if err := op.run(ts, "WEB-1"); err != nil {
			// Restore of a live task and RestoreAt of a task that is not older are
			// refusals of the operation itself, which came from org-b
			if op.name == "Restore" || op.name == "RestoreAt" {
				if e := AsError(err); e.Code == "task.not_found" || e.Code == "store.choose" {
					t.Errorf("%s: %v", op.name, err)
				}
				continue
			}
			t.Errorf("%s(WEB-1): %v", op.name, err)
			continue
		}
		if got := commits(t, ts, "org-a"); got != a {
			t.Errorf("%s changed org-a (%d -> %d commits)", op.name, a, got)
		}
		if got := commits(t, ts, "org-b"); !op.read && got <= b && op.name != "BeginComment" && op.name != "RestoreAt" {
			t.Errorf("%s did not write to org-b (%d -> %d commits)", op.name, b, got)
		}
		if !op.read && op.name != "BeginComment" && op.name != "RestoreAt" {
			if got := lastAuthor(t, ts, "org-b"); got != "bob" {
				t.Errorf("%s wrote as %q; want bob", op.name, got)
			}
		}
	}
}

func TestEveryOperationOnAnIDRefusesAReadOnlyStoreAfterTheLookup(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	makeStoreReadOnly(t, ts, "org-b")
	for _, op := range handedOverOps(t) {
		if op.read {
			continue
		}
		wantCode(t, op.name+"(WEB-1)", op.run(ts, "WEB-1"), KindConflict, "store.read_only")
		wantCode(t, op.name+"(WEB-999)", op.run(ts, "WEB-999"), KindNotFound, "task.not_found")
		wantCode(t, op.name+"(org-b:WEB-999)", op.run(ts, "org-b:WEB-999"), KindConflict, "store.read_only")
		ts.StoreName = "org-b"
		wantCode(t, op.name+"(WEB-999) with --store org-b", op.run(ts, "WEB-999"), KindConflict, "store.read_only")
		ts.StoreName = ""
	}
}

func TestSecondIDThatNoStoreHasIsNotFound(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	_, err := ts.Dep("API-1", "org-b:WEB-999", true)
	wantCode(t, "Dep(API-1, org-b:WEB-999)", err, KindNotFound, "task.not_found")
	_, err = ts.Dep("API-1", "WEB-999", true)
	wantCode(t, "Dep(API-1, WEB-999)", err, KindNotFound, "task.not_found")
	_, err = ts.Dep("API-1", "org-b:WEB-1", true)
	wantCode(t, "Dep(API-1, org-b:WEB-1)", err, KindInvalid, "ref.cross_store")
}

func TestNoStoresIsTheErrorOfSelect(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := map[string]string{"HOME": d, "USERPROFILE": d, "HEDGEHUG_HOME": filepath.Join(d, "hh"), "HEDGEHUG_AUTO_SYNC": "0"}
	ss := &Session{Getenv: func(k string) string { return env[k] }, Now: time.Now}
	if _, err := ss.Show("X-1"); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("Show with no store = %v; want ErrNoStores", err)
	}
	if _, err := ss.Set("X-1", nil); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("Set with no store = %v; want ErrNoStores", err)
	}
	if _, err := ss.CommentEdit("01JZZZZZZZZZZZZZZZZZZZZZZZ", "x"); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("CommentEdit with no store = %v; want ErrNoStores", err)
	}
}

func TestOperationsOnTwoStoresRunConcurrently(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	var wg gosync.WaitGroup
	errs := make([]error, 12)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch i % 3 {
			case 0:
				_, errs[i] = ts.Show("WEB-1")
			case 1:
				_, errs[i] = ts.Set("API-1", []Assign{{Key: "title", Op: "=", Value: "t"}})
			default:
				_, err := ts.Dep("API-1", "WEB-1", true)
				if AsError(err) == nil || AsError(err).Code != "ref.cross_store" {
					errs[i] = errors.New("Dep: want ref.cross_store, got " + errString(err))
				}
			}
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("operation %d: %v", i, err)
		}
	}
}

func errString(err error) string {
	if err == nil {
		return "nil"
	}
	return err.Error()
}
