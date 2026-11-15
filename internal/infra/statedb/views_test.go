package statedb

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/savedview"
)

func TestViewsRoundTrip(t *testing.T) {
	ctx := t.Context()
	v := openTemp(t).Views()

	if _, ok, err := v.Get(ctx, "abc"); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}
	want := savedview.View{ID: "abc", Name: "バグ", Content: savedview.Content{
		Query:  "status:Todo",
		Layout: "table",
		Table:  savedview.Table{Columns: []string{"id", "p:customer", "title"}, Group: "status", Sort: &savedview.Sort{Key: "due", Desc: true}},
	}}
	if err := v.Put(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := v.Get(ctx, "abc")
	if err != nil || !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("get: %+v ok=%v err=%v", got, ok, err)
	}
	if list, err := v.List(ctx); err != nil || len(list) != 1 || !reflect.DeepEqual(list[0], want) {
		t.Fatalf("list: %+v err=%v", list, err)
	}
	want.Name = "変更後"
	want.Content.Query = ""
	if err := v.Put(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := v.Get(ctx, "abc"); got.Name != "変更後" || got.Content.Query != "" {
		t.Errorf("overwrite: %+v", got)
	}
	if list, _ := v.List(ctx); len(list) != 1 {
		t.Errorf("list after overwrite: %+v", list)
	}
}

func TestViewsDeleteDropsScreenState(t *testing.T) {
	ctx := t.Context()
	db := openTemp(t)
	if err := db.ScreenStates().Put(ctx, "view:abc", "{}"); err != nil {
		t.Fatal(err)
	}
	if err := db.ScreenStates().Put(ctx, "search", "{}"); err != nil {
		t.Fatal(err)
	}
	if err := db.Views().Put(ctx, savedview.View{ID: "abc", Name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Views().Delete(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.Views().Get(ctx, "abc"); ok {
		t.Error("ビューが残っている")
	}
	if _, ok, _ := db.ScreenStates().Get(ctx, "view:abc"); ok {
		t.Error("view:abc の画面の状態が残っている")
	}
	if _, ok, _ := db.ScreenStates().Get(ctx, "search"); !ok {
		t.Error("search の画面の状態が消えた")
	}
}
