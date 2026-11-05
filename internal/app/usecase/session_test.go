package usecase_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestUpdateCommitsAndCallsAfterWrite(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	var after int
	ts.AfterWrite = func(string) { after++ }
	s, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0]
	task := &model.Task{ULID: model.NewULID(), Title: "a", Kind: "task", Status: "todo", Workspace: ws.ULID}
	committed, err := ts.Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) })
	if err != nil || !committed {
		t.Fatalf("Update = %v, %v; want true, nil", committed, err)
	}
	if after != 1 {
		t.Errorf("AfterWrite called %d times; want 1", after)
	}
}

func TestResolveNotFoundIsKindNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Resolve(ix, "ME-99")
	if usecase.KindOf(err) != usecase.KindNotFound {
		t.Fatalf("KindOf(%v) = %v; want KindNotFound", err, usecase.KindOf(err))
	}
	if got, want := err.Error(), `"ME-99": not found`; got != want {
		t.Errorf("Error() = %q; want %q", got, want)
	}
}

// A write to a store with a remote starts the background sync once when the
// session asks for it (BackgroundSync), and never when it does not. It does
// not run in parallel: replicate.SetSpawn replaces what Start does for all.
func TestBackgroundSyncStartsOnlyWhenAsked(t *testing.T) {
	for _, tc := range []struct {
		background bool
		want       int
	}{{false, 0}, {true, 1}} {
		ts := usecasetest.New(t)
		usecasetest.AddBareRemote(t, ts)
		ts.Env["HEDGEHUG_AUTO_SYNC"] = "1"
		ts.BackgroundSync = tc.background
		var started int
		restore := replicate.SetSpawn(func(string, []string, string) error { started++; return nil })
		s, ix, err := ts.Open()
		if err != nil {
			t.Fatal(err)
		}
		task := &model.Task{ULID: model.NewULID(), Title: "a", Kind: "task", Status: "todo", Workspace: ix.Workspaces()[0].ULID}
		if _, err := ts.Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
			t.Fatal(err)
		}
		restore()
		if started != tc.want {
			t.Errorf("BackgroundSync=%v: the background sync started %d times; want %d", tc.background, started, tc.want)
		}
	}
}

func TestSessionConcurrentOperations(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.UseAccount()
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "seed"}); err != nil {
		t.Fatal(err)
	}
	ts.Renew() // the first of the goroutines reads the writer again
	ops := []func() error{
		func() error { _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); return err },
		func() error { _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "b"}); return err },
		func() error {
			_, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "seed2"}})
			return err
		},
		func() error { _, err := comments.CommentAdd(ts.Session, "ME-1", "hello"); return err },
		func() error {
			_, err := browse.List(ts.Session, "")
			return err
		},
		func() error { _, _, err := notifications.InboxList(ts.Session, false); return err },
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
	t.Parallel()
	src := &usecase.Session{}
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
		case reflect.Bool:
			f.SetBool(true)
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
	if sub := src.ForStore("other"); !sub.SharesTokensWith(src) {
		t.Error("ForStore does not share the token cache")
	}
	// The cache of the writer is not carried over.
	ts := usecasetest.New(t)
	ts.UseAccount()
	if _, ok, err := ts.Writer(); err != nil || !ok {
		t.Fatalf("Writer = %v, %v", ok, err)
	}
	if sub := ts.ForStore("me"); sub.WriterCached() {
		t.Error("ForStore copied the cached writer")
	}
}

func TestSessionUpdateCommitAndBeforeUpdate(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
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
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if before != 1 {
		t.Errorf("BeforeUpdate called after it was cleared: %d", before)
	}
}

// TestRenewCopiesEveryExportedField fails when a field is added to Session
// and usecasetest's Renew does not carry it to the new session.
func TestRenewCopiesEveryExportedField(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	v := reflect.ValueOf(ts.Session).Elem()
	for i := range v.NumField() {
		f, sf := v.Field(i), v.Type().Field(i)
		if !sf.IsExported() || !f.IsZero() {
			continue // what New set stays as it is
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
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Pointer:
			f.Set(reflect.New(f.Type().Elem()))
		case reflect.Interface:
			f.Set(reflect.ValueOf(&fakeKeyring{}))
		default:
			t.Fatalf("Session.%s is a %s; teach this test to set it", sf.Name, f.Kind())
		}
	}
	old := ts.Session
	ts.Renew()
	if ts.Session == old {
		t.Fatal("Renew kept the old session")
	}
	got := reflect.ValueOf(ts.Session).Elem()
	for i := range got.NumField() {
		sf := got.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		if got.Field(i).IsZero() {
			t.Errorf("Renew does not copy Session.%s", sf.Name)
		}
		if got.Field(i).Kind() == reflect.Pointer && got.Field(i).Pointer() != v.Field(i).Pointer() {
			t.Errorf("Renew does not share Session.%s", sf.Name)
		}
	}
}
