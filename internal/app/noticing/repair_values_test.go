package noticing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	pendingRef  = "statuses/" + string(apptest.SampleStatusPending)
	valueHigh   = change.ID("01SAMPLEVAL000000000000007")
	valueClosed = change.ID("01CLOSED000000000000000001")
	valueShip   = change.ID("01SHIPPED00000000000000001")
)

var (
	statusList   = workspace.ParseList("statuses")
	priorityList = workspace.ParseList("priorities")
)

// nameTaro は、taro の表示名を 太郎 にする。
func nameTaro(f *fixture) {
	f.t.Helper()
	f.save(user("hanako"), func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Rename("taro", "taro", "太郎"))
	})
}

// undoDelete は、op の削除を取り消す、wsRef の修復の保存。
func undoDelete(f *fixture, op change.ID) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(func(tx port.Tx) error {
		log, err := tx.Changes().Of(wsRef)
		if err != nil {
			return err
		}
		return appendRef(wsRef, change.Undo(log, op)...)(tx)
	})
}

// restoredValue は、setter が prep を書いたあと、hanako が wsRef の fdel を削除し、その削除を取り消す修復の保存を
// 判定して、出た通知を返す。
func restoredValue(f *fixture, setter change.Actor, prep []change.Draft, fdel string) port.Notice {
	f.t.Helper()
	nameTaro(f)
	f.save(setter, appendRef(wsRef, prep...))
	op := f.save(user("hanako"), appendRef(wsRef, change.NewDelete(fdel))).Key.Op
	f.judge(undoDelete(f, op))
	n := f.wantOne()
	if n.Source != "repair:restore:"+string(op) {
		f.t.Errorf("source = %q, want repair:restore:%s", n.Source, op)
	}
	return n
}

func reviewDefault() []change.Draft {
	return []change.Draft{
		change.NewSet(pendingRef+"/name", "Pending", "Review"),
		change.NewSet("category_default/doing", apptest.SampleStatusInProgress, apptest.SampleStatusPending),
	}
}

func TestRestoreTextDefault(t *testing.T) {
	f := setup(t)
	n := restoredValue(f, user("taro"), reviewDefault(), pendingRef)
	if want := "「Review」は 太郎 が同時に既定にしたため、削除を取り消しました"; n.Text != want || n.Section != "statuses" {
		t.Errorf("notice = %+v, want text %q in statuses", n, want)
	}
}

func TestRestoreTextEmptyCategory(t *testing.T) {
	f := setup(t)
	// 完了の分類に Closed と Done(valueShip)を足す。
	closed := workspace.Value{ID: valueClosed, Name: "Closed", Color: "gray", Category: workspace.CategoryDone, Rank: "b"}
	done := workspace.Value{ID: valueShip, Name: "Done", Color: "gray", Category: workspace.CategoryDone, Rank: "c"}
	var prep []change.Draft
	prep = append(prep, workspace.ValueDrafts(statusList, closed)...)
	prep = append(prep, workspace.ValueDrafts(statusList, done)...)
	// 既定を Closed にし、元の Done を削除し、Closed を完了から外すと、完了には valueShip だけが残る。
	prep = append(prep,
		change.NewSet("category_default/done", apptest.SampleStatusDone, valueClosed),
		change.NewDelete(statusList.Ref(apptest.SampleStatusDone)),
		change.NewSet(statusList.Field(valueClosed, "category"), workspace.CategoryDone, workspace.CategoryTodo),
	)
	n := restoredValue(f, user("taro"), prep, statusList.Ref(valueShip))
	if want := "「Done」は完了の最後のステータスのため、削除を取り消しました"; n.Text != want || n.Section != "statuses" {
		t.Errorf("notice = %+v, want text %q in statuses", n, want)
	}
}

func TestRestoreTextInitialPriority(t *testing.T) {
	f := setup(t)
	prep := []change.Draft{change.NewSet("initial_priority", apptest.SamplePriorityMedium, valueHigh)}
	n := restoredValue(f, user("taro"), prep, priorityList.Ref(valueHigh))
	if want := "「High」は 太郎 が同時に既定にしたため、削除を取り消しました"; n.Text != want || n.Section != "priorities" {
		t.Errorf("notice = %+v, want text %q in priorities", n, want)
	}
}

// 名前の変更と重なった取り消し(指されていない値)は、これまでの本文のまま。
func TestRestoreTextOverlapUnchanged(t *testing.T) {
	f := setup(t)
	nameTaro(f)
	f.save(user("taro"), appendRef(wsRef, change.NewSet(pendingRef+"/name", "Pending", "Review")))
	op := f.save(user("hanako"), appendRef(wsRef, change.NewDelete(pendingRef))).Key.Op
	f.save(user("taro"), appendRef(wsRef, change.NewSet(pendingRef+"/name", "Review", "Review2")))
	f.judge(undoDelete(f, op))
	if n, want := f.wantOne(), "「Review2」は 太郎 が同時に変更したため、削除を取り消しました"; n.Text != want {
		t.Errorf("text = %q, want %q", n.Text, want)
	}
}

// 既定にした行が人の行でなければ、変更した人は不明なユーザー。
func TestRestoreTextUnknownSetter(t *testing.T) {
	f := setup(t)
	n := restoredValue(f, change.Actor{Kind: change.ActorSync}, reviewDefault(), pendingRef)
	if want := "「Review」は 不明なユーザー が同時に既定にしたため、削除を取り消しました"; n.Text != want {
		t.Errorf("text = %q, want %q", n.Text, want)
	}
}
