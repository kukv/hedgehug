package views

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
)

func TestRenameOwnCase(t *testing.T) {
	s, _ := newService()
	bugs := mustCreate(t, s, "Bugs")
	mustCreate(t, s, "Tasks")

	v, err := s.Rename(t.Context(), bugs.ID, "BUGS")
	if err != nil || v.Name != "BUGS" {
		t.Fatalf("own case: %+v err=%v", v, err)
	}
	_, err = s.Rename(t.Context(), bugs.ID, " tasks")
	wantRefusal(t, err, "同じ名前のビューがあります")
	_, err = s.Rename(t.Context(), bugs.ID, "  ")
	wantRefusal(t, err, "名前を入力してください")
}

func TestDuplicateNames(t *testing.T) {
	s, _ := newService()
	src, err := s.Create(t.Context(), "Bugs", Content{Query: "q", Layout: "table"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Bugs のコピー", "Bugs のコピー 2"}
	for _, name := range want {
		v, err := s.Duplicate(t.Context(), src.ID)
		if err != nil || v.Name != name {
			t.Fatalf("Duplicate: %+v err=%v, want %q", v, err, name)
		}
		if v.ID == src.ID || !reflect.DeepEqual(v.Content, src.Content) {
			t.Errorf("識別子か内容が違う: %+v", v)
		}
	}
	mustCreate(t, s, "bugs のコピー 3")
	if v, err := s.Duplicate(t.Context(), src.ID); err != nil || v.Name != "Bugs のコピー 4" {
		t.Fatalf("4 番目: %+v err=%v", v, err)
	}
}

func TestWritesPublishViews(t *testing.T) {
	s, rec := newService()
	ctx := t.Context()
	step := func(name string, do func() error) {
		t.Helper()
		before := len(rec.events)
		if err := do(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(rec.events) != before+1 || rec.events[before] != (port.Event{Kind: port.EventViews}) {
			t.Fatalf("%s: events = %v", name, rec.events)
		}
	}
	var v View
	step("作成", func() (err error) { v, err = s.Create(ctx, "a", Content{}); return })
	step("保存", func() (err error) { _, err = s.Save(ctx, v.ID, Content{Query: "x"}); return })
	step("名前の変更", func() (err error) { _, err = s.Rename(ctx, v.ID, "b"); return })
	step("複製", func() (err error) { _, err = s.Duplicate(ctx, v.ID); return })
	step("削除", func() error { return s.Delete(ctx, v.ID) })

	before := len(rec.events)
	if _, err := s.Rename(ctx, "none", " "); err == nil {
		t.Fatal("断られるはず")
	}
	if _, err := s.Create(ctx, "", Content{}); err == nil {
		t.Fatal("断られるはず")
	}
	if len(rec.events) != before {
		t.Errorf("断られた操作で出来事が流れた: %v", rec.events[before:])
	}
}

func TestUnknownViewIsNotFound(t *testing.T) {
	s, _ := newService()
	ctx := t.Context()
	_, errSave := s.Save(ctx, "none", Content{})
	_, errRename := s.Rename(ctx, "none", "x")
	_, errDup := s.Duplicate(ctx, "none")
	errDel := s.Delete(ctx, "none")
	for name, err := range map[string]error{"Save": errSave, "Rename": errRename, "Duplicate": errDup, "Delete": errDel} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
