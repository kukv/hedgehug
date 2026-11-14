package port_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// deleteSampleWorkspace は、サンプルのワークスペース(root: true)と、プロジェクト HH-P1・Epic HH-1(root: false)を
// 同じ操作で削除する(ワークスペースの削除と同じ形)。
func deleteSampleWorkspace(t *testing.T, s port.Stores) {
	t.Helper()
	err := s.Write(t.Context(), reviveStore, user, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDeleteEntity(true)),
			tx.WriteProjects().Append(apptest.SampleProject, change.NewDeleteEntity(false)),
			tx.WriteItems().Append(apptest.SampleEpic, change.NewDeleteEntity(false)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// reviveWorkspace は、1 回の保存の中で ReviveWorkspace を呼び、取り消したかと積んだものを返す。
func reviveWorkspace(t *testing.T, s port.Stores, name, prefix string) (bool, []port.Revival, error) {
	t.Helper()
	ctx := port.WithRevived(t.Context())
	var ok bool
	err := s.Write(ctx, reviveStore, user, func(tx port.Tx) error {
		var err error
		ok, err = port.ReviveWorkspace(ctx, tx, apptest.SampleWorkspace, name, prefix)
		return err
	})
	return ok, port.Revived(ctx), err
}

// ワークスペースの削除を取り消すと、一緒に削除されたプロジェクト・アイテムも戻る。前に自分で削除しておいた
// アイテム(別の操作)は削除済みのまま(決定 R11)。
func TestReviveWorkspaceRestoresContents(t *testing.T) {
	s := reviveStores(t)
	deleteIn(t, s, map[item.ID]bool{apptest.SampleTask: true}, nil)
	deleteSampleWorkspace(t, s)
	assertEditableWorkspace(t, s)
	ok, revived, err := reviveWorkspace(t, s, "", "")
	if err != nil || !ok {
		t.Fatalf("ReviveWorkspace = %v, %v", ok, err)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "Web"}}; !slices.Equal(revived, want) {
		t.Errorf("Revived = %v, want %v", revived, want)
	}
	assertSampleRevived(t, s)
	if ok, _, err := reviveWorkspace(t, s, "", ""); ok || err != nil {
		t.Errorf("ReviveWorkspace of a live workspace = %v, %v, want false", ok, err)
	}
}

// assertEditableWorkspace は、EditableWorkspace が削除済みのワークスペースを返し、ないものは ErrNotFound にすることを確かめる。
func assertEditableWorkspace(t *testing.T, s port.Stores) {
	t.Helper()
	readIn(t, s, func(r port.Reader) {
		if w, err := port.EditableWorkspace(r, apptest.SampleWorkspace); err != nil || w.Deleted == nil {
			t.Errorf("EditableWorkspace of a deleted workspace = %+v, %v", w, err)
		}
		if _, err := port.EditableWorkspace(r, "01NOSUCHWS000000000000001"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("EditableWorkspace of a missing workspace = %v, want ErrNotFound", err)
		}
	})
}

// assertSampleRevived は、ワークスペース・プロジェクト・Epic が戻り、自分で削除した Task は削除済みのままかを確かめる。
func assertSampleRevived(t *testing.T, s port.Stores) {
	t.Helper()
	readIn(t, s, func(r port.Reader) {
		if _, err := port.LiveWorkspace(r, apptest.SampleWorkspace); err != nil {
			t.Errorf("workspace: %v", err)
		}
		if _, err := port.LiveProject(r, apptest.SampleProject); err != nil {
			t.Errorf("project: %v", err)
		}
		if _, err := port.LiveItem(r, apptest.SampleEpic); err != nil {
			t.Errorf("epic: %v", err)
		}
		if _, err := port.LiveItem(r, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("task deleted on its own = %v, want still deleted", err)
		}
	})
}

// readIn は、Store を読んで fn に渡す。
func readIn(t *testing.T, s port.Stores, fn func(r port.Reader)) {
	t.Helper()
	if err := s.Read(t.Context(), reviveStore, func(r port.Reader) error { fn(r); return nil }); err != nil {
		t.Fatal(err)
	}
}

// takeOver は、削除のあと、Mobile の名前・接頭辞を削除したワークスペースのものに変える。
func takeOver(t *testing.T, s port.Stores, d change.Draft) {
	t.Helper()
	if err := apptest.SampleMobile(t.Context(), s, reviveStore); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(t.Context(), reviveStore, user, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleMobileWorkspace, d)
	}); err != nil {
		t.Fatal(err)
	}
}

// 取り消すと名前・接頭辞が削除されていないワークスペースと重なるなら、08「復元」の文言で断る(決定 R11)。
func TestReviveWorkspaceNameTaken(t *testing.T) {
	testReviveWorkspaceTaken(t, change.NewSet("name", "Mobile", "Web"),
		"同じ名前のワークスペースがあります。先にそのワークスペースの名前を変更してください")
}

func TestReviveWorkspacePrefixTaken(t *testing.T) {
	testReviveWorkspaceTaken(t, change.NewSet("prefix", "MB", "HH"),
		"同じ接頭辞のワークスペースがあります。先にそのワークスペースの接頭辞を変更してください")
}

func testReviveWorkspaceTaken(t *testing.T, d change.Draft, msg string) {
	t.Helper()
	s := reviveStores(t)
	deleteSampleWorkspace(t, s)
	takeOver(t, s, d)
	_, revived, err := reviveWorkspace(t, s, "", "")
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != msg {
		t.Errorf("ReviveWorkspace = %v, want %q", err, msg)
	}
	if len(revived) != 0 {
		t.Errorf("Revived = %v, want none", revived)
	}
	readIn(t, s, func(r port.Reader) {
		if _, err := port.LiveWorkspace(r, apptest.SampleWorkspace); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("workspace = %v, want still deleted", err)
		}
	})
	// 変更後の名前・接頭辞で判定するので、重ならない値を渡せば取り消す
	if ok, _, err := reviveWorkspace(t, s, "Web2", "HH2"); !ok || err != nil {
		t.Errorf("ReviveWorkspace with other name and prefix = %v, %v", ok, err)
	}
}
