package workspaces_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

func setup(t *testing.T) workspaces.Service {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	return workspaces.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}}
}

// rejected は、err が画面に出す文言 msg の断りであることを確かめる。
func rejected(t *testing.T, err error, msg string) {
	t.Helper()
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != msg {
		t.Errorf("error = %v, want %q", err, msg)
	}
}

func TestCreate(t *testing.T) {
	s := setup(t)
	w, err := s.Create(t.Context(), store, "  Web  ", " hh ")
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Web" || w.Prefix != "HH" {
		t.Errorf("workspace = %+v", w)
	}
	err = s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		st, err := r.Workspaces().Settings(w.ID)
		if err != nil || len(st.Statuses) != 5 || len(st.Properties) != 8 {
			t.Errorf("settings = %+v, %v", st, err)
		}
		log, _ := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		if len(log) != 1 || log[0].T != change.Create || log[0].By != "taro" {
			t.Errorf("log = %+v", log)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 名前の途中の空白と改行は 1 つの空白にまとめ、1 行の名前で保存する(03)。
func TestCreateFoldsNewlineInName(t *testing.T) {
	s := setup(t)
	w, err := s.Create(t.Context(), store, " Web\n  App \r\n ", "HH")
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Web App" {
		t.Errorf("name = %q", w.Name)
	}
	// 同じ名前を 1 行で入れても重複になる。
	_, err = s.Create(t.Context(), store, "Web App", "API")
	rejected(t, err, "この Store には同じ名前のワークスペースがあります")
}

func TestCreateRefuses(t *testing.T) {
	s := setup(t)
	if _, err := s.Create(t.Context(), store, "Web", "HH"); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, prefix, msg string }{
		{" ", "API", "名前を入力してください"},
		{"Web", "", "この Store には同じ名前のワークスペースがあります"},
		{"API", "", "接頭辞を入力してください"},
		{"API", "1API", "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"},
		{"API", "ABCDEFGHIJK", "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"},
		{"API", "hh", "この Store には同じ接頭辞のワークスペースがあります"},
	}
	for _, c := range cases {
		_, err := s.Create(t.Context(), store, c.name, c.prefix)
		rejected(t, err, c.msg)
	}
	// 名前の比べ方は完全一致(大文字と小文字は区別する。03「名前の重複」)。
	if _, err := s.Create(t.Context(), store, "web", "WEB"); err != nil {
		t.Errorf("a name differing only in case was refused: %v", err)
	}
}
