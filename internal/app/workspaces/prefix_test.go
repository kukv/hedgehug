package workspaces_test

import (
	"reflect"
	"testing"
)

func TestChangePrefix(t *testing.T) {
	s := setup(t)
	w, err := s.Create(t.Context(), store, "Web", "HH")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(t.Context(), store, "API", "API"); err != nil {
		t.Fatal(err)
	}
	rejected(t, s.ChangePrefix(t.Context(), store, w.ID, ""), "接頭辞を入力してください")
	rejected(t, s.ChangePrefix(t.Context(), store, w.ID, "W-B"), "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください")
	rejected(t, s.ChangePrefix(t.Context(), store, w.ID, "api"), "この Store には同じ接頭辞のワークスペースがあります")
	// 同じ接頭辞: 何もしない(保存しない)。
	if err := s.ChangePrefix(t.Context(), store, w.ID, "hh"); err != nil {
		t.Fatal(err)
	}
	if _, log := stored(t, s, w.ID); len(log) != 1 {
		t.Errorf("log has %d rows after a no-op", len(log))
	}
	if err := s.ChangePrefix(t.Context(), store, w.ID, "web"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(t.Context(), store, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace.Prefix != "WEB" || !reflect.DeepEqual(got.OldPrefixes, []string{"HH"}) {
		t.Errorf("detail = %+v", got)
	}
}

func TestChangePrefixUnknownWorkspace(t *testing.T) {
	s := setup(t)
	if err := s.ChangePrefix(t.Context(), store, "01NOPE00000000000000000000", "WEB"); err == nil {
		t.Error("changed the prefix of a workspace that does not exist")
	}
}
