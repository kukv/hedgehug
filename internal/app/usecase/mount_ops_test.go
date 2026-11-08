package usecase_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// lastAuthor is who made the latest commit of the store name.
func lastAuthor(t *testing.T, ts *usecasetest.Session, name string) string {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	log, err := usecasetest.LogOf(s.Repo(), 1)
	if err != nil || len(log) != 1 {
		t.Fatalf("log of %s: %v %v", name, log, err)
	}
	return log[0].Author.Name
}

func TestSetFindsTheTaskInItsStoreAndWritesAsItsAccount(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	if _, err := tasks.Set(ts.Session, "WEB-1", []usecase.Assign{{Key: "title", Op: "=", Value: "changed"}}); err != nil {
		t.Fatal(err)
	}
	r, err := browse.Show(ts.Session, "WEB-1")
	if err != nil || r.Title != "changed" {
		t.Fatalf("WEB-1 = %v, %v", r, err)
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
	ts := usecasetest.NewTwoStore(t)
	r, err := browse.Show(ts.Session, "API-1")
	if err != nil || r.Title != "org-a task" {
		t.Errorf("Show(API-1) = %v, %v", r, err)
	}
	r, err = browse.Show(ts.Session, "org-b:WEB-1")
	if err != nil || r.Title != "org-b task" {
		t.Errorf("Show(org-b:WEB-1) = %v, %v", r, err)
	}
	if _, err := browse.Log(ts.Session, []string{"WEB-1"}, 0); err != nil {
		t.Errorf("Log(WEB-1): %v", err)
	}
}

func TestShowSameIDInTwoStoresIsAmbiguous(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStoreWith(t, "XX", "XX")
	_, err := browse.Show(ts.Session, "XX-1")
	usecasetest.WantCode(t, "Show(XX-1)", err, usecase.KindAmbiguous, "task.ambiguous")
	if err != nil {
		for _, want := range []string{"org-a:XX-1", "org-b:XX-1"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message %q lacks %s", err, want)
			}
		}
		if c, ok := usecase.AsError(err).Args["candidates"].([]string); !ok || len(c) != 2 {
			t.Errorf("candidates = %v", usecase.AsError(err).Args["candidates"])
		}
	}
	if r, err := browse.Show(ts.Session, "org-b:XX-1"); err != nil || r.Title != "org-b task" {
		t.Errorf("Show(org-b:XX-1) = %v, %v", r, err)
	}
}

func TestStoreNameKeepsTheLookupInTheStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	ts.StoreName = "org-a"
	_, err := browse.Show(ts.Session, "WEB-1")
	usecasetest.WantCode(t, "Show(WEB-1) with --store org-a", err, usecase.KindNotFound, "task.not_found")
	_, err = browse.Show(ts.Session, "org-b:WEB-1")
	usecasetest.WantCode(t, "Show(org-b:WEB-1) with --store org-a", err, usecase.KindUsage, "store.conflict")
	if r, err := browse.Show(ts.Session, "org-a:API-1"); err != nil || r.ID != "API-1" {
		t.Errorf("Show(org-a:API-1) = %v, %v", r, err)
	}
}

func TestReadOnlyRefusalOrder(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	usecasetest.MakeStoreReadOnly(t, ts, "org-b")
	assign := []usecase.Assign{{Key: "title", Op: "=", Value: "x"}}
	_, err := tasks.Set(ts.Session, "WEB-1", assign)
	usecasetest.WantCode(t, "Set(WEB-1)", err, usecase.KindConflict, "store.read_only")
	_, err = tasks.Set(ts.Session, "WEB-999", assign)
	usecasetest.WantCode(t, "Set(WEB-999)", err, usecase.KindNotFound, "task.not_found")
	ts.StoreName = "org-b"
	_, err = tasks.Set(ts.Session, "WEB-999", assign)
	usecasetest.WantCode(t, "Set(WEB-999) with --store org-b", err, usecase.KindConflict, "store.read_only")
	ts.StoreName = ""
	_, err = tasks.Set(ts.Session, "org-b:WEB-999", assign)
	usecasetest.WantCode(t, "Set(org-b:WEB-999)", err, usecase.KindConflict, "store.read_only")
}

func TestSecondIDsStayInsideTheStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	const msg = "org-b's WEB-1 cannot be referred to (references stay inside a store)"
	for _, blocker := range []string{"WEB-1", "org-b:WEB-1"} {
		_, err := tasks.Dep(ts.Session, "API-1", blocker, true)
		usecasetest.WantCode(t, "Dep(API-1, "+blocker+")", err, usecase.KindInvalid, "ref.cross_store")
		if err != nil && err.Error() != msg {
			t.Errorf("message = %q; want %q", err, msg)
		}
		if err != nil && (usecase.AsError(err).Args["store"] != "org-b" || usecase.AsError(err).Args["id"] != "WEB-1") {
			t.Errorf("args = %v", usecase.AsError(err).Args)
		}
	}
	if _, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Dep(ts.Session, "API-1", "org-a:API-2", true); err != nil {
		t.Errorf("Dep(API-1, org-a:API-2): %v", err)
	}

	_, err := tasks.Set(ts.Session, "API-1", []usecase.Assign{{Key: "blocked_by", Op: "=", Value: "WEB-1"}})
	usecasetest.WantCode(t, "Set blocked_by=WEB-1", err, usecase.KindInvalid, "ref.cross_store")
	_, err = tasks.Set(ts.Session, "API-1", []usecase.Assign{{Key: "relates", Op: "=", Value: "WEB-1"}})
	usecasetest.WantCode(t, "Set relates=WEB-1", err, usecase.KindInvalid, "ref.cross_store")
	_, err = tasks.Move(ts.Session, tasks.MoveInput{Arg: "API-1", Parent: "WEB-1"})
	usecasetest.WantCode(t, "Move --parent WEB-1", err, usecase.KindInvalid, "ref.cross_store")
	_, err = tasks.Move(ts.Session, tasks.MoveInput{Arg: "API-1", Before: "WEB-1"})
	usecasetest.WantCode(t, "Move --before WEB-1", err, usecase.KindInvalid, "ref.cross_store")
	_, err = tasks.Move(ts.Session, tasks.MoveInput{Arg: "API-1", After: "org-b:WEB-1"})
	usecasetest.WantCode(t, "Move --after org-b:WEB-1", err, usecase.KindInvalid, "ref.cross_store")
	_, err = tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "child", Parent: "WEB-1"})
	usecasetest.WantCode(t, "Add --parent WEB-1", err, usecase.KindInvalid, "ref.cross_store")

	es, err := tasks.BeginEdit(ts.Session, "API-2")
	if err != nil {
		t.Fatal(err)
	}
	web := usecasetest.UlidOf(t, ts, "WEB-1")
	edited := bytes.Replace(es.Bytes, []byte("kind: project\n"), []byte("kind: project\nblocked_by: ["+web.String()+"]\n"), 1)
	nt, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
	if err != nil {
		t.Fatal(err)
	}
	usecasetest.WantCode(t, "Edit blocked_by: [WEB-1]", tasks.PrepareEdit(ts.Session, es, nt), usecase.KindInvalid, "ref.cross_store")
}

func TestTwoStepOperationsWriteInTheStoreTheyBeganIn(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	es, err := tasks.BeginEdit(ts.Session, "WEB-1")
	if err != nil {
		t.Fatal(err)
	}
	nt, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, bytes.Replace(es.Bytes, []byte("title: org-b task"), []byte("title: edited"), 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := tasks.PrepareEdit(ts.Session, es, nt); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.WriteEdit(ts.Session, es, nt); err != nil {
		t.Fatal(err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("WriteEdit author = %q; want bob", got)
	}

	cs, err := comments.BeginComment(ts.Session, "WEB-1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := comments.PrepareComment(ts.Session, cs, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if p.Comment.Author != "github.com/bob" {
		t.Errorf("comment author = %q; want github.com/bob", p.Comment.Author)
	}
	res, err := comments.WriteComment(ts.Session, p)
	if err != nil {
		t.Fatal(err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("WriteComment author = %q; want bob", got)
	}

	ces, err := comments.OpenComment(ts.Session, res.Comment.ULID.String())
	if err != nil {
		t.Fatal(err)
	}
	pe, err := comments.PrepareCommentEdit(ts.Session, ces, "changed")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comments.WriteCommentEdit(ts.Session, pe); err != nil {
		t.Fatal(err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("WriteCommentEdit author = %q; want bob", got)
	}
}

func TestCommentOperationsFindTheCommentInAnyStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	r, err := comments.CommentAdd(ts.Session, "WEB-1", "first")
	if err != nil {
		t.Fatal(err)
	}
	id := r.Comment.ULID.String()
	if _, err := comments.CommentEdit(ts.Session, id, "second"); err != nil {
		t.Fatalf("CommentEdit without --store: %v", err)
	}
	if got := lastAuthor(t, ts, "org-b"); got != "bob" {
		t.Errorf("author = %q; want bob", got)
	}
	if _, err := comments.CommentRemove(ts.Session, id); err != nil {
		t.Errorf("CommentRemove without --store: %v", err)
	}
	_, err = comments.CommentEdit(ts.Session, "01JZZZZZZZZZZZZZZZZZZZZZZZ", "x")
	usecasetest.WantCode(t, "CommentEdit of no comment", err, usecase.KindNotFound, "comment.not_found")
}

func TestCommentULIDInTwoStoresIsAmbiguous(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	c := &model.Comment{ULID: model.NewULID(), Body: "same\n"}
	for name, id := range map[string]string{"org-a": "API-1", "org-b": "WEB-1"} {
		sub := ts.ForStore(name)
		cs, err := comments.BeginComment(sub, id)
		if err != nil {
			t.Fatal(err)
		}
		p, err := comments.PrepareComment(sub, cs, "same")
		if err != nil {
			t.Fatal(err)
		}
		p.Comment = c
		if _, err := comments.WriteComment(sub, p); err != nil {
			t.Fatal(err)
		}
	}
	_, err := comments.OpenComment(ts.Session, c.ULID.String())
	usecasetest.WantCode(t, "OpenComment of a ULID in two stores", err, usecase.KindAmbiguous, "comment.ambiguous")
}

// commits is the number of commits of the store name.
func commits(t *testing.T, ts *usecasetest.Session, name string) int {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	log, err := usecasetest.LogOf(s.Repo(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return len(log)
}

// handedOver is an operation on a task or comment that takes its id.
type handedOver struct {
	name string
	task bool // takes the id of a task (else a comment)
	run  func(ts *usecasetest.Session, id string) error
	read bool // reads only
}

func handedOverOps(t *testing.T) []handedOver {
	t.Helper()
	return []handedOver{
		{name: "Show", task: true, read: true, run: func(ts *usecasetest.Session, id string) error { _, err := browse.Show(ts.Session, id); return err }},
		{name: "Log", task: true, read: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := browse.Log(ts.Session, []string{id}, 0)
			return err
		}},
		{name: "Set", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := tasks.Set(ts.Session, id, []usecase.Assign{{Key: "title", Op: "=", Value: "set " + id}})
			return err
		}},
		{name: "SetStatus", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := tasks.SetStatus(ts.Session, id, "in_progress")
			return err
		}},
		{name: "Watch", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := tasks.Watch(ts.Session, id, false)
			return err
		}},
		{name: "Dep", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := tasks.Dep(ts.Session, id, "org-b:WEB-2", true)
			return err
		}},
		{name: "Move", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := tasks.Move(ts.Session, tasks.MoveInput{Arg: id, After: "WEB-2"})
			return err
		}},
		{name: "Edit", task: true, run: func(ts *usecasetest.Session, id string) error {
			es, err := tasks.BeginEdit(ts.Session, id)
			if err != nil {
				return err
			}
			_, err = tasks.Edit(ts.Session, id, bytes.Replace(es.Bytes, []byte("title: "), []byte("title: edited "), 1))
			return err
		}},
		{name: "Remove", task: true, run: func(ts *usecasetest.Session, id string) error { _, err := tasks.Remove(ts.Session, id); return err }},
		{name: "Restore", task: true, run: func(ts *usecasetest.Session, id string) error { _, err := tasks.Restore(ts.Session, id); return err }},
		{name: "RestoreAt", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := tasks.RestoreAt(ts.Session, id, "2026-10-30T12:00:00Z")
			return err
		}},
		{name: "BeginComment", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := comments.BeginComment(ts.Session, id)
			return err
		}},
		{name: "CommentAdd", task: true, run: func(ts *usecasetest.Session, id string) error {
			_, err := comments.CommentAdd(ts.Session, id, "hi")
			return err
		}},
	}
}

func TestEveryOperationOnAnIDGoesToTheStoreOfTheTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	if _, err := tasks.Add(ts.ForStore("org-b"), tasks.AddInput{Title: "second"}); err != nil {
		t.Fatal(err)
	}
	for _, op := range handedOverOps(t) {
		a, b := commits(t, ts, "org-a"), commits(t, ts, "org-b")
		if err := op.run(ts, "WEB-1"); err != nil {
			// Restore of a live task and RestoreAt of a task that is not older are
			// refusals of the operation itself, which came from org-b
			if op.name == "Restore" || op.name == "RestoreAt" {
				if e := usecase.AsError(err); e.Code == "task.not_found" || e.Code == "store.choose" {
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
	ts := usecasetest.NewTwoStore(t)
	usecasetest.MakeStoreReadOnly(t, ts, "org-b")
	for _, op := range handedOverOps(t) {
		if op.read {
			continue
		}
		usecasetest.WantCode(t, op.name+"(WEB-1)", op.run(ts, "WEB-1"), usecase.KindConflict, "store.read_only")
		usecasetest.WantCode(t, op.name+"(WEB-999)", op.run(ts, "WEB-999"), usecase.KindNotFound, "task.not_found")
		usecasetest.WantCode(t, op.name+"(org-b:WEB-999)", op.run(ts, "org-b:WEB-999"), usecase.KindConflict, "store.read_only")
		ts.StoreName = "org-b"
		usecasetest.WantCode(t, op.name+"(WEB-999) with --store org-b", op.run(ts, "WEB-999"), usecase.KindConflict, "store.read_only")
		ts.StoreName = ""
	}
}

func TestSecondIDThatNoStoreHasIsNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	_, err := tasks.Dep(ts.Session, "API-1", "org-b:WEB-999", true)
	usecasetest.WantCode(t, "Dep(API-1, org-b:WEB-999)", err, usecase.KindNotFound, "task.not_found")
	_, err = tasks.Dep(ts.Session, "API-1", "WEB-999", true)
	usecasetest.WantCode(t, "Dep(API-1, WEB-999)", err, usecase.KindNotFound, "task.not_found")
	_, err = tasks.Dep(ts.Session, "API-1", "org-b:WEB-1", true)
	usecasetest.WantCode(t, "Dep(API-1, org-b:WEB-1)", err, usecase.KindInvalid, "ref.cross_store")
}

func TestNoStoresIsTheErrorOfSelect(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	env := map[string]string{"HOME": d, "HEDGEHUG_HOME": filepath.Join(d, "hh"), "HEDGEHUG_AUTO_SYNC": "0"}
	ss := &usecase.Session{Getenv: func(k string) string { return env[k] }, Now: time.Now}
	if _, err := browse.Show(ss, "X-1"); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("Show with no store = %v; want ErrNoStores", err)
	}
	if _, err := tasks.Set(ss, "X-1", nil); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("Set with no store = %v; want ErrNoStores", err)
	}
	if _, err := comments.CommentEdit(ss, "01JZZZZZZZZZZZZZZZZZZZZZZZ", "x"); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("CommentEdit with no store = %v; want ErrNoStores", err)
	}
}

func TestOperationsOnTwoStoresRunConcurrently(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	var wg gosync.WaitGroup
	errs := make([]error, 12)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch i % 3 {
			case 0:
				_, errs[i] = browse.Show(ts.Session, "WEB-1")
			case 1:
				_, errs[i] = tasks.Set(ts.Session, "API-1", []usecase.Assign{{Key: "title", Op: "=", Value: "t"}})
			default:
				_, err := tasks.Dep(ts.Session, "API-1", "WEB-1", true)
				if usecase.AsError(err) == nil || usecase.AsError(err).Code != "ref.cross_store" {
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
