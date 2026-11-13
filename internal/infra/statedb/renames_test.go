package statedb

import (
	"maps"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// 改名の対応は小文字で持ち、Store ごとに分ける。改名先から先の対応は消す(戻す改名で輪にしない。D4b 最終レビュー R-1)。
func TestRenames(t *testing.T) {
	s := openTemp(t).PRScans()
	ctx := t.Context()
	save := func(from, to string) {
		t.Helper()
		if err := s.SaveRename(ctx, "S1", from, to); err != nil {
			t.Fatal(err)
		}
	}
	renames := func(store port.StoreID) map[string]string {
		t.Helper()
		got, err := s.Renames(ctx, store)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	save("Org/Web", "Org/Site")
	save("org/site", "org/app") // 鎖
	if got := renames("S1"); !maps.Equal(got, map[string]string{"org/web": "org/site", "org/site": "org/app"}) {
		t.Fatalf("Renames = %v", got)
	}
	if got := renames("S2"); len(got) != 0 {
		t.Errorf("other store: %v", got)
	}
	save("org/app", "org/web") // 元の名前に戻す: org/web から先の対応を消す
	if got := renames("S1"); !maps.Equal(got, map[string]string{"org/site": "org/app", "org/app": "org/web"}) {
		t.Errorf("after rename back: %v", got)
	}
	save("org/web", "ORG/WEB") // 大文字小文字だけの違いは記録しない
	if got := renames("S1"); len(got) != 2 {
		t.Errorf("case only: %v", got)
	}
}
