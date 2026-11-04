package tasks_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestEditChangesTitleAndRefusesULIDChange(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := tasks.BeginEdit(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(es.Bytes, []byte("title: a"), []byte("title: b"), 1)
	nt, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
	if err != nil {
		t.Fatal(err)
	}
	if err := tasks.PrepareEdit(ts.Session, es, nt); err != nil {
		t.Fatal(err)
	}
	r, err := tasks.WriteEdit(ts.Session, es, nt)
	if err != nil || !r.Committed || r.ULID != es.Entry.ULID {
		t.Fatalf("WriteEdit = %+v, %v", r, err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := ix.Task(es.Entry.ULID); !ok || e.Title != "b" {
		t.Errorf("title after edit = %+v; want b", e)
	}
	bad := bytes.Replace(es.Bytes, []byte(es.Entry.ULID.String()), []byte("01JZZZZZZZZZZZZZZZZZZZZZZZ"), 1)
	if _, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, bad); usecase.KindOf(err) != usecase.KindInvalid {
		t.Errorf("ULID change kind = %v; want KindInvalid (%v)", usecase.KindOf(err), err)
	}
}

func TestBeginEditErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := tasks.BeginEdit(ts.Session, "ME-99")
	usecasetest.WantErr(t, err, usecase.KindNotFound, "task.not_found")
	if _, err := tasks.Remove(ts.Session, "ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err = tasks.BeginEdit(ts.Session, "ME-1")
	usecasetest.WantErr(t, err, usecase.KindConflict, "task.deleted")
}

func TestBeginEditReadsFile(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := tasks.BeginEdit(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if es.Old == nil || es.Old.Title != "a" || es.Old.ID != "ME-1" || es.Old.ULID != es.Entry.ULID {
		t.Errorf("Old = %+v; want task a with id ME-1", es.Old)
	}
	if !strings.HasPrefix(string(es.Bytes), "---\nulid: "+es.Entry.ULID.String()+"\n") {
		t.Errorf("Bytes = %q; want the task file", es.Bytes)
	}
}

func TestCheckEditRefusals(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := tasks.BeginEdit(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	old := string(es.Bytes)
	created := "created: " + es.Old.Created.Format("2006-01-02T15:04:05Z07:00")
	if !strings.Contains(old, created) {
		t.Fatalf("no %q in\n%s", created, old)
	}
	ws := "workspace: " + es.Old.Workspace.String()
	cases := []struct {
		name, from, to, field, msg string
	}{
		{"id", "id: ME-1", "id: ME-2", "id", "id cannot be changed"},
		{"created", created, "created: 2020-01-01T00:00:00Z", "created", "created cannot be changed"},
		{"rank", "rank: a0", "rank: b0", "rank", "rank cannot be changed here; use hedgehug mv"},
		{"workspace", ws, "workspace: " + model.NewULID().String(), "workspace", "workspace cannot be changed here; use hedgehug mv --workspace"},
		{"ulid", "ulid: " + es.Entry.ULID.String(), "ulid: 01JZZZZZZZZZZZZZZZZZZZZZZZ", "ulid", "ulid cannot be changed"},
		{"moved_to", "title: a", "title: a\nmoved_to: 01JC0M3R5W7M2X9V4T6R5N8P1B", "moved_to", "moved_to cannot be set by hand; use hedgehug mv --store"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			edited := strings.Replace(old, c.from, c.to, 1)
			_, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
			usecasetest.WantErr(t, err, usecase.KindInvalid, "edit.invalid")
			var e *usecase.Error
			if !usecasetest.AsError(err, &e) || e.Args["field"] != c.field {
				t.Errorf("Args = %+v; want field %q", e, c.field)
			}
			if err != nil && err.Error() != c.msg {
				t.Errorf("message = %q; want %q", err, c.msg)
			}
		})
	}
	t.Run("updated", func(t *testing.T) {
		edited := strings.Replace(old, "updated: "+es.Old.Updated.Format("2006-01-02T15:04:05Z07:00"), "updated: 2020-01-01T00:00:00Z", 1)
		_, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		usecasetest.WantErr(t, err, usecase.KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "updated cannot be changed" {
			t.Errorf("message = %q", err)
		}
	})
	t.Run("deleted_at", func(t *testing.T) {
		edited := strings.Replace(old, "title: a", "title: a\ndeleted_at: 2020-01-01T00:00:00Z", 1)
		_, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		usecasetest.WantErr(t, err, usecase.KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "deleted_at cannot be changed" {
			t.Errorf("message = %q", err)
		}
	})
	t.Run("parent", func(t *testing.T) {
		edited := strings.Replace(old, "title: a", "title: a\nparent: "+model.NewULID().String(), 1)
		_, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		usecasetest.WantErr(t, err, usecase.KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "parent cannot be changed here; use hedgehug mv" {
			t.Errorf("message = %q", err)
		}
	})
	t.Run("former_ids", func(t *testing.T) {
		edited := strings.Replace(old, "title: a", "title: a\nformer_ids: [ME-7]", 1)
		_, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		usecasetest.WantErr(t, err, usecase.KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "former_ids cannot be changed" {
			t.Errorf("message = %q", err)
		}
	})
}

func TestCheckEditBrokenFrontMatter(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := tasks.BeginEdit(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte("no front matter"))
	usecasetest.WantErr(t, err, usecase.KindInvalid, "edit.invalid")
	var e *usecase.Error
	if usecasetest.AsError(err, &e) && e.Args != nil {
		t.Errorf("Args = %v; want none for a file that does not decode", e.Args)
	}
	if err != nil && !strings.HasPrefix(err.Error(), "the file does not decode: ") {
		t.Errorf("message = %q", err)
	}
}

func TestCheckEditOfBrokenTaskChecksOnlyULID(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := tasks.BeginEdit(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	// With no old task, the id and the workspace may be repaired.
	fixed := strings.Replace(string(es.Bytes), "id: ME-1", "id: ME-5", 1)
	nt, err := tasks.CheckEdit(es.Index, es.Entry.ULID, nil, []byte(fixed))
	if err != nil || nt.ID != "ME-5" {
		t.Fatalf("CheckEdit = %+v, %v; want id ME-5", nt, err)
	}
	other := strings.Replace(string(es.Bytes), es.Entry.ULID.String(), "01JZZZZZZZZZZZZZZZZZZZZZZZ", 1)
	_, err = tasks.CheckEdit(es.Index, es.Entry.ULID, nil, []byte(other))
	usecasetest.WantErr(t, err, usecase.KindInvalid, "edit.invalid")
	if err != nil && err.Error() != "the ulid must stay "+es.Entry.ULID.String() {
		t.Errorf("message = %q", err)
	}
}

func TestWriteEditRefusesWhenFileChanged(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := tasks.BeginEdit(ts.Session, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "c"}}); err != nil {
		t.Fatal(err)
	}
	nt, err := tasks.CheckEdit(es.Index, es.Entry.ULID, es.Old, bytes.Replace(es.Bytes, []byte("title: a"), []byte("title: b"), 1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = tasks.WriteEdit(ts.Session, es, nt)
	usecasetest.WantErr(t, err, usecase.KindConflict, "edit.changed")
	if err != nil && err.Error() != "the task changed while editing" {
		t.Errorf("WriteEdit = %v; want the task changed while editing", err)
	}
}

func TestHelpersSameTimeSameULID(t *testing.T) {
	t.Parallel()
	a, b := model.NewULID(), model.NewULID()
	if !tasks.SameULID(nil, nil) || !tasks.SameULID(&a, &a) || tasks.SameULID(&a, &b) || tasks.SameULID(&a, nil) || tasks.SameULID(nil, &a) {
		t.Error("sameULID is wrong")
	}
	n := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	later := n.Add(1)
	if !tasks.SameTime(nil, nil) || !tasks.SameTime(&n, &n) || tasks.SameTime(&n, &later) || tasks.SameTime(&n, nil) || tasks.SameTime(nil, &n) {
		t.Error("sameTime is wrong")
	}
}
