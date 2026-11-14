package transfers_test

import (
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// withWorkspaceNewcomer は、Web の中身(削除済みのプロジェクトとその下のアイテムを含む)を足し、Epic の担当者を jiro
// (移動先にいない人。移動で足される)にする。
func withWorkspaceNewcomer(t *testing.T, f fixture) {
	t.Helper()
	withWorkspaceContents(t, f)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("assignee", nil, "jiro"))
	})
}

// sourceRows は、移動元(main)の Web のワークスペース・プロジェクト・アイテムのログの行数。
func sourceRows(t *testing.T, f fixture) map[change.Ref]int {
	t.Helper()
	out := map[change.Ref]int{}
	for _, ref := range webRefs() {
		out[ref] = len(logOf(t, f.rec, store1, ref))
	}
	return out
}

// assertWorkspaceRolledBack は、移動先からワークスペース・プロジェクト・アイテム(削除済みを含む)のフォルダと足したメンバーが
// 消え、戻しのアクティビティが書かれ、移動元が元のまま(行数が before と同じで、転送記録がない)であることを確かめる。
func assertWorkspaceRolledBack(t *testing.T, f fixture, before map[change.Ref]int) {
	t.Helper()
	assertWorkspaceUntouched(t, f)
	for _, id := range []project.ID{apptest.SampleProject, wsGoneProject} {
		if _, ok := projectAt(t, f.rec, store2, id); ok {
			t.Errorf("project %s remains in the destination", id)
		}
	}
	if exists(t, f.rec, store2, wsGoneItem) {
		t.Error("the item under the deleted project remains in the destination")
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro"}) {
		t.Errorf("destination members = %v, want [Taro]", got)
	}
	if s := f.rec.summaries(store2); s[len(s)-1] != "ワークスペース「Web」の移動を取り消し" {
		t.Errorf("destination summary = %q", s[len(s)-1])
	}
	if got := sourceRows(t, f); !maps.Equal(got, before) {
		t.Errorf("source rows = %v, want %v", got, before)
	}
}

// B5c-11・02「保存」: 移動元の保存が失敗したら、移動先からフォルダと足したメンバーを消して戻し、移動元は変わらない。
func TestMoveWorkspaceRollsBack(t *testing.T) {
	f := newFixture(t)
	withWorkspaceNewcomer(t, f)
	before := sourceRows(t, f)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1}
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	assertWorkspaceRolledBack(t, f, before)
}

// 移動元の保存の前に、移動先で移したワークスペースの中にアイテムが作られていても、戻しはそれも消す(移動先に、ない
// ワークスペースのアイテムを残さない)。
func TestMoveWorkspaceRollbackRemovesItemsAddedAtTheDestination(t *testing.T) {
	const added item.ID = "01TESTIT000000000000000013"
	f := newFixture(t)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1, before: func() {
		addItem(t, f.rec, store2, item.Item{ID: added, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 9}, Kind: item.Epic, Rank: "X", Title: "移動先で"})
	}}
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if exists(t, f.rec, store2, added) {
		t.Error("the item added at the destination remains")
	}
	assertWorkspaceUntouched(t, f)
}

// 戻しも送れなかった(理由)・失敗した(エラー)ときは *UndoError(Old はワークスペースの名前、NewDisplay は空)。
// 移動先にはワークスペースが残る。
func TestMoveWorkspaceUndoFails(t *testing.T) {
	for _, d := range []undoFails{{reason: syncstatus.WriteDenied}, {err: errUndo}} {
		f := newFixture(t)
		f.svc.Stores = failWrite{Stores: f.rec, store: store1}
		d.inner, d.calls = f.deliver, new(int)
		f.svc.Delivery = d
		_, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2)
		var ue *transfers.UndoError
		if !errors.As(err, &ue) || ue.Old != "Web" || ue.Store != "team" || ue.NewDisplay != "" || !errors.Is(err, errBoom) {
			t.Fatalf("err = %#v", err)
		}
		const want = "ワークスペース「Web」を team へ移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください"
		if ue.Error() != want {
			t.Errorf("文 = %q", ue.Error())
		}
		if _, ok := webAt(t, f.rec, store2); !ok {
			t.Error("the destination workspace was removed")
		}
		if _, ok := webAt(t, f.rec, store1); !ok {
			t.Error("the source workspace was removed")
		}
	}
}

// failMoveWithChangeMeanwhile は、移動先の保存のあと、移動元の保存の前に mutate で移動元を変えてワークスペースを移し、
// 移動が失敗して移動先が戻り、移動元が mutate のあとのままであることを確かめる。
func failMoveWithChangeMeanwhile(t *testing.T, mutate func(fixture)) fixture {
	t.Helper()
	f := newFixture(t)
	withWorkspaceNewcomer(t, f)
	done := false
	f.deliver.inside = func(port.StoreID) {
		if !done {
			done = true
			mutate(f)
		}
	}
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); !errors.Is(err, transfers.ErrSourceChanged) {
		t.Fatalf("err = %v, want the source-changed error", err)
	}
	assertWorkspaceUntouched(t, f)
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro"}) {
		t.Errorf("destination members = %v, want [Taro]", got)
	}
	if s := f.rec.summaries(store2); s[len(s)-1] != "ワークスペース「Web」の移動を取り消し" {
		t.Errorf("destination summary = %q", s[len(s)-1])
	}
	return f
}

// B5c-9・Review Focus 2: 移動先の保存のあと、移動元の保存の前に、ワークスペースの中にアイテムが足された・アイテムが変えられた・
// 設定の行(ステータスの改名)が足されたら、移動元の保存が失敗して移動先を戻す。足したもの・変えたものは移動元に残る。
func TestMoveWorkspaceSourceChanged(t *testing.T) {
	t.Run("item added", func(t *testing.T) {
		const added item.ID = "01TESTIT000000000000000013"
		f := failMoveWithChangeMeanwhile(t, func(f fixture) {
			addItem(t, f.rec, store1, item.Item{ID: added, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Number: 9}, Kind: item.Task, Rank: "X", Title: "あとから"})
		})
		if !exists(t, f.rec, store1, added) || exists(t, f.rec, store2, added) {
			t.Error("the added item moved")
		}
	})
	t.Run("item edited", func(t *testing.T) {
		f := failMoveWithChangeMeanwhile(t, func(f fixture) {
			write(t, f.rec, store1, func(tx port.Tx) error {
				return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("title", "認証", "認証(改)"))
			})
		})
		if got := itemOf(t, f.rec, store1, apptest.SampleEpic).Title; got != "認証(改)" {
			t.Errorf("source title = %q", got)
		}
	})
	t.Run("status renamed", func(t *testing.T) {
		f := failMoveWithChangeMeanwhile(t, func(f fixture) {
			write(t, f.rec, store1, func(tx port.Tx) error {
				field := workspace.List{Kind: workspace.ListStatuses}.Field(apptest.SampleStatusTodo, "name")
				return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet(field, "Todo", "ToDo"))
			})
		})
		var st workspace.Settings
		if err := f.rec.Read(t.Context(), store1, func(r port.Reader) (err error) { st, err = r.Workspaces().Settings(apptest.SampleWorkspace); return }); err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(st.Statuses, func(v workspace.Value) bool { return v.Name == "ToDo" }) {
			t.Errorf("source statuses = %+v", st.Statuses)
		}
	})
}

// B5c-9: 外のアイテムのログは比べない。あいだに外のアイテム(Mobile の Epic)が Web の Task との関連を足しても移動は成功し、
// その関連は読み直した値から移動元で remove される。
func TestMoveWorkspaceIgnoresOutsideChanges(t *testing.T) {
	f := newFixture(t)
	done := false
	f.deliver.inside = func(port.StoreID) {
		if !done {
			done = true
			addRelation(t, f.rec, mobileEpic, apptest.SampleTask, "relates")
		}
	}
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); err != nil {
		t.Fatal(err)
	}
	if rel := itemOf(t, f.rec, store1, mobileEpic).Relations; len(rel) != 0 {
		t.Errorf("outside epic relations = %+v", rel)
	}
}

// 移動先の保存が送れず捨てられもしなかった(保存されたかもしれない)ときは、移動元を保存せずに戻す。保存が実際には
// 捨てられていた(moved_in がない)ときは、戻しはメンバーの行を書かず、ないフォルダは飛ばす。
func TestMoveWorkspaceSaveNotDiscarded(t *testing.T) {
	t.Run("kept", func(t *testing.T) {
		f := newFixture(t)
		withWorkspaceNewcomer(t, f)
		before := sourceRows(t, f)
		f.svc.Delivery = notDiscarded{inner: f.deliver, done: new(bool)}
		if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); !errors.Is(err, port.ErrSaveNotDiscarded) {
			t.Fatalf("err = %v", err)
		}
		assertWorkspaceRolledBack(t, f, before)
		if slices.Contains(f.rec.summaries(store1), "ワークスペース「Web」を team へ移動") {
			t.Error("the source was written")
		}
	})
	t.Run("lost", func(t *testing.T) {
		f := newFixture(t)
		withWorkspaceNewcomer(t, f)
		membersBefore := len(logOf(t, f.rec, store2, change.Ref{Kind: change.KindMembers}))
		f.svc.Delivery = lostSave{inner: f.deliver, stores: f.rec, done: new(bool)}
		if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); !errors.Is(err, port.ErrSaveNotDiscarded) {
			t.Fatalf("err = %v", err)
		}
		if n := len(logOf(t, f.rec, store2, change.Ref{Kind: change.KindMembers})); n != membersBefore {
			t.Errorf("destination member rows %d → %d", membersBefore, n)
		}
		assertWorkspaceUntouched(t, f)
	})
}

// Review Focus 2(A5a): main の Web を team へ、team の Team を main へ同時に移しても止まらない。fn は移動元の Store を
// 読まず、移動元の読み直しは Web の中だけを比べるので、両方とも成功する。
func TestMoveWorkspaceOppositeDirections(t *testing.T) {
	f := newFixture(t)
	f.deliver.gate = newBarrier()
	f.deliver.inside = newBarrier()
	done := make(chan error, 2)
	go func() {
		_, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2)
		done <- err
	}()
	go func() {
		_, err := f.svc.MoveWorkspace(t.Context(), store2, teamWS, store1)
		done <- err
	}()
	timeout := time.After(10 * time.Second)
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("err = %v", err)
			}
		case <-timeout:
			t.Fatal("the two moves stalled")
		}
	}
	if _, ok := webAt(t, f.rec, store2); !ok {
		t.Error("Web is not in team")
	}
	if !exists(t, f.rec, store1, teamEpic) || exists(t, f.rec, store2, teamEpic) {
		t.Error("Team's epic did not move to main")
	}
}
