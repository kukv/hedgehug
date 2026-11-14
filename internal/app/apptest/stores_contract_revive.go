package apptest

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// contractUserRestoreSeenInTx は、利用者の操作の中で書いた restore の行を、同じ Write の中の読み取りが見ることを
// 確かめる。ReviveItem を 2 回呼んでも、restore の行は 1 つだけ書く(C3a-screen)。
func contractUserRestoreSeenInTx(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	if err := DeleteItems(t.Context(), s, id, cItem.ID); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		ctx := port.WithRevived(t.Context())
		if err := port.ReviveItem(ctx, tx, cItem.ID); err != nil {
			return err
		}
		if got, err := tx.Items().Get(cItem.ID); err != nil || got.Deleted != nil {
			t.Errorf("in the same write: Deleted = %+v, %v", got.Deleted, err)
		}
		return port.ReviveItem(ctx, tx, cItem.ID)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		n := 0
		for _, rec := range log {
			if rec.T == change.Restore {
				n++
			}
		}
		if err != nil || n != 1 {
			t.Errorf("restore rows = %d, %v", n, err)
		}
		return nil
	})
}

// contractUserSettingRestoreSeenInTx は、利用者の操作の中で書いた設定の restore の行を、同じ Write の中の
// 設定の読み取りが見ることを確かめる。ReviveSetting を 2 回呼んでも、restore の行は 1 つだけ書く(C3c)。
func contractUserSettingRestoreSeenInTx(t *testing.T, s port.Stores, id port.StoreID) {
	review := workspace.Value{ID: "01CONTRACTRV00000000000001", Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	f := workspace.ParseList("statuses").Ref(review.ID)
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(cWorkspace.ID, workspace.ValueDrafts(workspace.ParseList("statuses"), review)...)
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(cWorkspace.ID, change.NewDelete(f))
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		reviveSettingTwice(t, tx, f, review.ID)
		return nil
	})
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		n := 0
		for _, rec := range log {
			if rec.T == change.Restore {
				n++
			}
		}
		if err != nil || n != 1 {
			t.Errorf("restore rows = %d, %v", n, err)
		}
		return nil
	})
}

// contractUserWorkspaceRestoreSeenInTx は、利用者の操作の中で ReviveWorkspace が書いた restore の行を、同じ Write の中の
// 読み取りが見ること(ワークスペースも一緒に削除されたプロジェクトも削除されていない)を確かめる。
// ワークスペースのログの restore の行は 1 つだけ(C3c)。
func contractUserWorkspaceRestoreSeenInTx(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(cWorkspace.ID, change.NewDeleteEntity(true)),
			tx.WriteProjects().Append(cProject.ID, change.NewDeleteEntity(false)),
		)
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		reviveWorkspaceSeen(t, tx)
		return nil
	})
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		n := 0
		for _, rec := range log {
			if rec.T == change.Restore {
				n++
			}
		}
		if err != nil || n != 1 {
			t.Errorf("restore rows = %d, %v", n, err)
		}
		return nil
	})
}

// reviveWorkspaceSeen は、tx の中で ReviveWorkspace を呼び、そのあとの同じ tx の読み取りで、ワークスペースと
// プロジェクトが削除されていないことを確かめる。
func reviveWorkspaceSeen(t *testing.T, tx port.Tx) {
	t.Helper()
	ok, err := port.ReviveWorkspace(port.WithRevived(t.Context()), tx, cWorkspace.ID, "", "")
	if err != nil || !ok {
		t.Errorf("ReviveWorkspace = %v, %v", ok, err)
	}
	if w, err := tx.Workspaces().Get(cWorkspace.ID); err != nil || w.Deleted != nil {
		t.Errorf("in the same write: workspace Deleted = %+v, %v", w.Deleted, err)
	}
	if p, err := tx.Projects().Get(cProject.ID); err != nil || p.Deleted != nil {
		t.Errorf("in the same write: project Deleted = %+v, %v", p.Deleted, err)
	}
}

// reviveSettingTwice は、同じ tx の中で f に ReviveSetting を 2 回呼ぶ。1 回目は true で、そのあとの設定の読み取りに
// 値 v があり、2 回目は false であることを確かめる。
func reviveSettingTwice(t *testing.T, tx port.Tx, f string, v change.ID) {
	t.Helper()
	ctx := port.WithRevived(t.Context())
	if ok, err := port.ReviveSetting(ctx, tx, cWorkspace.ID, f); err != nil || !ok {
		t.Errorf("first ReviveSetting = %v, %v", ok, err)
	}
	st, err := tx.Workspaces().Settings(cWorkspace.ID)
	if err != nil || !slices.ContainsFunc(st.Statuses, func(x workspace.Value) bool { return x.ID == v }) {
		t.Errorf("in the same write: statuses = %+v, %v", st.Statuses, err)
	}
	if ok, err := port.ReviveSetting(ctx, tx, cWorkspace.ID, f); err != nil || ok {
		t.Errorf("second ReviveSetting = %v, %v", ok, err)
	}
}

// runWorkspaceContract は、ワークスペースのフォルダの Import・Remove の照合を、空の Store で回す。
func runWorkspaceContract(t *testing.T, newStores func(t *testing.T) (s port.Stores, id, other port.StoreID)) {
	t.Run("remove deletes a workspace folder", func(t *testing.T) {
		s, id, _ := newStores(t)
		contractWorkspaceRemove(t, s, id)
	})
	t.Run("import copies a workspace folder between stores", func(t *testing.T) {
		s, a, b := newStores(t)
		contractWorkspaceImport(t, s, a, b)
	})
}

// contractWorkspaceImport は、別の Store のワークスペースの行を、行を変えずに写すことと、写せない場合の断りを確かめる。
// 写したワークスペースは、Get と Settings で読める。
func contractWorkspaceImport(t *testing.T, s port.Stores, a, b port.StoreID) {
	ref := change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID}
	mustWrite(t, s, a, taro, writeFixture)
	var log []change.Record
	mustRead(t, s, a, func(r port.Reader) (err error) { log, err = r.Changes().Of(ref); return })
	mustWrite(t, s, b, taro, func(tx port.Tx) error { return tx.WriteWorkspaces().Import(cWorkspace.ID, log) })
	mustRead(t, s, b, func(r port.Reader) error {
		got, err := r.Changes().Of(ref)
		if err != nil || len(got) != len(log) {
			t.Fatalf("rows %d (want %d), %v", len(got), len(log), err)
		}
		for i, w := range log {
			if !sameRow(got[i], w) {
				t.Errorf("row %d = %+v, want %+v (行は変えずに写す)", i, got[i], w)
			}
		}
		if w, err := r.Workspaces().Get(cWorkspace.ID); err != nil || w.Name != cWorkspace.Name || w.Prefix != cWorkspace.Prefix {
			t.Errorf("Workspaces().Get = %+v, %v", w, err)
		}
		if _, err := r.Workspaces().Settings(cWorkspace.ID); err != nil {
			t.Errorf("Workspaces().Settings = %v", err)
		}
		return nil
	})
	checkWorkspaceImportRefused(t, s, b, log)
	checkWorkspaceMovedIn(t, s, b)
}

// checkWorkspaceImportRefused は、すでにあるフォルダ・空の行・不正な ID への Import の断りを確かめる。
func checkWorkspaceImportRefused(t *testing.T, s port.Stores, b port.StoreID, log []change.Record) {
	for name, call := range map[string]func(w port.WorkspaceWriter) error{
		"existing folder": func(w port.WorkspaceWriter) error { return w.Import(cWorkspace.ID, log) },
		"empty log":       func(w port.WorkspaceWriter) error { return w.Import("01CONTRACTWS00000000000002", nil) },
		"bad id":          func(w port.WorkspaceWriter) error { return w.Import("bad id", log) },
	} {
		if err := s.Write(t.Context(), b, taro, func(tx port.Tx) error { return call(tx.WriteWorkspaces()) }); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import workspace (%s) = %v, want ErrInvalidOperation", name, err)
		}
	}
}

// contractWorkspaceRemove は、ワークスペースの Remove のあとはどの読み方でももうなく、下のプロジェクトは残ること、
// 同じ Tx で書いた行も捨てること、ないものと不正な ID の断りを確かめる。
func contractWorkspaceRemove(t *testing.T, s port.Stores, id port.StoreID) {
	ref := change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID}
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		w := tx.WriteWorkspaces()
		if err := errors.Join(w.Append(cWorkspace.ID, change.NewSet("name", "Web", "同じ Tx の名前")), w.Remove(cWorkspace.ID)); err != nil {
			return err
		}
		if _, err := tx.Changes().Of(ref); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Changes().Of after Append and Remove = %v, want ErrNotFound", err)
		}
		return nil
	})
	mustRead(t, s, id, func(r port.Reader) error {
		_, getErr := r.Workspaces().Get(cWorkspace.ID)
		_, logErr := r.Changes().Of(ref)
		_, setErr := r.Workspaces().Settings(cWorkspace.ID)
		if !errors.Is(getErr, port.ErrNotFound) || !errors.Is(logErr, port.ErrNotFound) || !errors.Is(setErr, port.ErrNotFound) {
			t.Errorf("Get = %v, Changes().Of = %v, Settings = %v, want ErrNotFound", getErr, logErr, setErr)
		}
		if ws, err := r.Workspaces().List(); err != nil || len(ws) != 0 {
			t.Errorf("List = %+v, %v", ws, err)
		}
		if _, err := r.Projects().Get(cProject.ID); err != nil {
			t.Errorf("the project went too: %v", err)
		}
		return nil
	})
	rm := func(w string) func(tx port.Tx) error {
		return func(tx port.Tx) error { return tx.WriteWorkspaces().Remove(change.ID(w)) }
	}
	if err := s.Write(t.Context(), id, taro, rm(string(cWorkspace.ID))); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Remove of a missing workspace = %v, want ErrNotFound", err)
	}
	if err := s.Write(t.Context(), id, taro, rm("bad id")); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Remove(bad id) = %v, want ErrInvalidOperation", err)
	}
}

// checkWorkspaceMovedIn は、cWorkspace があるときに、そのログに moved_in の行を足しても、Get と Settings が
// 足す前と同じことを確かめる。
func checkWorkspaceMovedIn(t *testing.T, s port.Stores, id port.StoreID) {
	var w workspace.Workspace
	var st workspace.Settings
	read := func() {
		mustRead(t, s, id, func(r port.Reader) (err error) {
			if w, err = r.Workspaces().Get(cWorkspace.ID); err == nil {
				st, err = r.Workspaces().Settings(cWorkspace.ID)
			}
			return
		})
	}
	read()
	before, beforeSettings := w, st
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(cWorkspace.ID, transfer.NewMovedIn(transfer.MovedInInfo{Old: "HH", FromStore: "other", FromWorkspace: cWorkspace.ID, At: time.Unix(0, 0).UTC()}))
	})
	read()
	if !reflect.DeepEqual(w, before) || !reflect.DeepEqual(st, beforeSettings) {
		t.Errorf("after moved_in: workspace %+v (want %+v), settings %+v (want %+v)", w, before, st, beforeSettings)
	}
}
