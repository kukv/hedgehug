package workspaces_test

import "testing"

func TestList(t *testing.T) {
	s := setup(t)
	a, _ := s.Create(t.Context(), store, "Web", "HH")
	b, _ := s.Create(t.Context(), store, "API", "API")
	list, err := s.List(t.Context(), store)
	if err != nil || len(list) != 2 || list[0] != a || list[1] != b {
		t.Errorf("List = %+v, %v", list, err)
	}
}
