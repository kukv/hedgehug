package workspaces_test

import (
	"reflect"
	"testing"
)

func TestList(t *testing.T) {
	s := setup(t)
	a, _ := s.Create(t.Context(), store, "Web", "HH")
	b, _ := s.Create(t.Context(), store, "API", "API")
	list, err := s.List(t.Context(), store)
	if err != nil || len(list) != 2 || list[0].Workspace != a || list[1].Workspace != b || len(list[0].OldPrefixes) != 0 {
		t.Errorf("List = %+v, %v", list, err)
	}
}

// 画面のアドレスの旧い接頭辞(03「接頭辞」)のため、一覧が旧い接頭辞を返す。
func TestListHasTheOldPrefixes(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	if err := s.ChangePrefix(t.Context(), store, w.ID, "web"); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(t.Context(), store)
	if err != nil || len(list) != 1 || list[0].Prefix != "WEB" || !reflect.DeepEqual(list[0].OldPrefixes, []string{"HH"}) {
		t.Errorf("List = %+v, %v", list, err)
	}
}
