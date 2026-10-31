package ops

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

func TestEditChangesTitleAndRefusesULIDChange(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(es.Bytes, []byte("title: a"), []byte("title: b"), 1)
	nt, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, edited)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ts.ApplyEdit(es, nt)
	if err != nil || !r.Committed || r.ULID != es.Entry.ULID {
		t.Fatalf("ApplyEdit = %+v, %v", r, err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := ix.Task(es.Entry.ULID); !ok || e.Title != "b" {
		t.Errorf("title after edit = %+v; want b", e)
	}
	bad := bytes.Replace(es.Bytes, []byte(es.Entry.ULID.String()), []byte("01JZZZZZZZZZZZZZZZZZZZZZZZ"), 1)
	if _, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, bad); KindOf(err) != KindInvalid {
		t.Errorf("ULID change kind = %v; want KindInvalid (%v)", KindOf(err), err)
	}
}

func TestBeginEditErrors(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.BeginEdit("ME-99")
	wantErr(t, err, KindNotFound, "task.not_found")
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err = ts.BeginEdit("ME-1")
	wantErr(t, err, KindConflict, "task.deleted")
}

func TestBeginEditReadsFile(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
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
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
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
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			edited := strings.Replace(old, c.from, c.to, 1)
			_, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
			wantErr(t, err, KindInvalid, "edit.invalid")
			var e *Error
			if !asError(err, &e) || e.Args["field"] != c.field {
				t.Errorf("Args = %+v; want field %q", e, c.field)
			}
			if err != nil && err.Error() != c.msg {
				t.Errorf("message = %q; want %q", err, c.msg)
			}
		})
	}
	t.Run("updated", func(t *testing.T) {
		edited := strings.Replace(old, "updated: "+es.Old.Updated.Format("2006-01-02T15:04:05Z07:00"), "updated: 2020-01-01T00:00:00Z", 1)
		_, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		wantErr(t, err, KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "updated cannot be changed" {
			t.Errorf("message = %q", err)
		}
	})
	t.Run("deleted_at", func(t *testing.T) {
		edited := strings.Replace(old, "title: a", "title: a\ndeleted_at: 2020-01-01T00:00:00Z", 1)
		_, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		wantErr(t, err, KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "deleted_at cannot be changed" {
			t.Errorf("message = %q", err)
		}
	})
	t.Run("parent", func(t *testing.T) {
		edited := strings.Replace(old, "title: a", "title: a\nparent: "+model.NewULID().String(), 1)
		_, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		wantErr(t, err, KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "parent cannot be changed here; use hedgehug mv" {
			t.Errorf("message = %q", err)
		}
	})
	t.Run("former_ids", func(t *testing.T) {
		edited := strings.Replace(old, "title: a", "title: a\nformer_ids: [ME-7]", 1)
		_, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte(edited))
		wantErr(t, err, KindInvalid, "edit.invalid")
		if err != nil && err.Error() != "former_ids cannot be changed" {
			t.Errorf("message = %q", err)
		}
	})
}

func TestCheckEditBrokenFrontMatter(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = CheckEdit(es.Index, es.Entry.ULID, es.Old, []byte("no front matter"))
	wantErr(t, err, KindInvalid, "edit.invalid")
	var e *Error
	if asError(err, &e) && e.Args != nil {
		t.Errorf("Args = %v; want none for a file that does not decode", e.Args)
	}
	if err != nil && !strings.HasPrefix(err.Error(), "the file does not decode: ") {
		t.Errorf("message = %q", err)
	}
}

func TestCheckEditOfBrokenTaskChecksOnlyULID(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	// With no old task, the id and the workspace may be repaired.
	fixed := strings.Replace(string(es.Bytes), "id: ME-1", "id: ME-5", 1)
	nt, err := CheckEdit(es.Index, es.Entry.ULID, nil, []byte(fixed))
	if err != nil || nt.ID != "ME-5" {
		t.Fatalf("CheckEdit = %+v, %v; want id ME-5", nt, err)
	}
	other := strings.Replace(string(es.Bytes), es.Entry.ULID.String(), "01JZZZZZZZZZZZZZZZZZZZZZZZ", 1)
	_, err = CheckEdit(es.Index, es.Entry.ULID, nil, []byte(other))
	wantErr(t, err, KindInvalid, "edit.invalid")
	if err != nil && err.Error() != "the ulid must stay "+es.Entry.ULID.String() {
		t.Errorf("message = %q", err)
	}
}

func TestApplyEditRefusesWhenFileChanged(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	es, err := ts.BeginEdit("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "c"}}); err != nil {
		t.Fatal(err)
	}
	nt, err := CheckEdit(es.Index, es.Entry.ULID, es.Old, bytes.Replace(es.Bytes, []byte("title: a"), []byte("title: b"), 1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.ApplyEdit(es, nt)
	if err == nil || err.Error() != "the task changed while editing" {
		t.Errorf("ApplyEdit = %v; want the task changed while editing", err)
	}
}

func TestHelpersSameTimeSameULID(t *testing.T) {
	a, b := model.NewULID(), model.NewULID()
	if !sameULID(nil, nil) || !sameULID(&a, &a) || sameULID(&a, &b) || sameULID(&a, nil) || sameULID(nil, &a) {
		t.Error("sameULID is wrong")
	}
	n := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	later := n.Add(1)
	if !sameTime(nil, nil) || !sameTime(&n, &n) || sameTime(&n, &later) || sameTime(&n, nil) || sameTime(nil, &n) {
		t.Error("sameTime is wrong")
	}
}
