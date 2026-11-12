package workspaces_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// stored は、保存されているワークスペース id と、そのログの行を返す。
func stored(t *testing.T, s workspaces.Service, id workspace.ID) (workspace.Workspace, []change.Record) {
	t.Helper()
	var w workspace.Workspace
	var log []change.Record
	err := s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		if w, err = r.Workspaces().Get(id); err != nil {
			return err
		}
		log, err = r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: id})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return w, log
}

func TestRename(t *testing.T) {
	s := setup(t)
	w, err := s.Create(t.Context(), store, "Web", "HH")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(t.Context(), store, "API", "API"); err != nil {
		t.Fatal(err)
	}
	rejected(t, s.Rename(t.Context(), store, w.ID, "  "), "名前は空にできません")
	rejected(t, s.Rename(t.Context(), store, w.ID, "API"), "この Store には同じ名前のワークスペースがあります")
	// 同じ名前に変えたときは何もしない(保存しない)。
	if err := s.Rename(t.Context(), store, w.ID, " Web "); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(t.Context(), store, w.ID, "フロント"); err != nil {
		t.Fatal(err)
	}
	got, log := stored(t, s, w.ID)
	// ログは作成と 1 回の変更だけ。
	if got.Name != "フロント" || len(log) != 2 || log[1].F != "name" || log[1].By != "taro" {
		t.Errorf("workspace = %+v, log = %+v", got, log)
	}
}

// 改行の入った名前は 1 行にして保存する(03)。
func TestRenameFoldsNewline(t *testing.T) {
	s := setup(t)
	w, err := s.Create(t.Context(), store, "Web", "HH")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(t.Context(), store, w.ID, " Web\n  App \r\n "); err != nil {
		t.Fatal(err)
	}
	if got, _ := stored(t, s, w.ID); got.Name != "Web App" {
		t.Errorf("name = %q", got.Name)
	}
}

func TestRenameUnknownWorkspace(t *testing.T) {
	s := setup(t)
	if err := s.Rename(t.Context(), store, "01NOPE00000000000000000000", "X"); err == nil {
		t.Error("renamed a workspace that does not exist")
	}
}
