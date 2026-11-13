package transfers_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func withoutPending(st *workspace.Settings) {
	st.Statuses = slices.DeleteFunc(st.Statuses, func(v workspace.Value) bool { return v.Name == "Pending" })
}

// 予告: 下の削除されていない件数 1・削除済み 1、移動先の次の番号の ID、置き換えの行(Pending だけ。生きている Step 1 件と削除済みの Step 1 件で Count は 2)。
func TestPreviewCountsAndStatusRows(t *testing.T) {
	f := newFixtureWith(t, withoutPending)
	parent := apptest.SampleTask
	mk := func(id item.ID, n int, status change.ID) item.Item {
		return addItem(t, f.rec, store1, item.Item{ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: n}, Kind: item.Step, Rank: "V", Title: "s", Status: status})
	}
	mk("01TESTIT000000000000000010", 3, apptest.SampleStatusPending)
	gone := mk("01TESTIT000000000000000011", 4, apptest.SampleStatusPending)
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, gone.ID); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject})
	if err != nil {
		t.Fatal(err)
	}
	// main(S1)には remote があるので、同じ Store の移動でも移動先に remote がある。
	if got.Below != 1 || got.Deleted != 1 || got.NextDisplay != "MB-2" || got.CrossStore || got.RemoteNotice || !got.DestinationHasRemote {
		t.Errorf("preview = %+v", got)
	}
	todo := valueID(t, f.rec, "status", "Todo")
	if len(got.Replacements) != 1 {
		t.Fatalf("rows = %+v", got.Replacements)
	}
	row := got.Replacements[0]
	if row.Status != apptest.SampleStatusPending || row.Name != "Pending" || row.Count != 2 || row.Default != todo || len(row.Choices) != 4 {
		t.Errorf("row = %+v", row)
	}
}

// 別の Store へ: 移動元に remote があり、移動先(team)にない → remote の注意を出す。
func TestPreviewAcrossStoresWarnsWhenOnlyTheSourceHasARemote(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, transfers.MoveInput{ToStore: store2, ToWorkspace: teamWS, ToProject: teamProject})
	if err != nil || !got.CrossStore || !got.RemoteNotice || got.DestinationHasRemote || got.NextDisplay != "TM-2" {
		t.Errorf("preview = %+v, %v", got, err)
	}
}

// 予告の置き換えの行(Unmatched)と、実際の移動(Remap)は、同じ規則で分ける。行にあるステータス(Pending)だけが選んだ置き換え先(Done)になり、
// 行にないステータス(InProgress)は、置き換え先を渡しても名前の一致が使われる。
func TestPreviewRowsAreExactlyTheStatusesMoveReplaces(t *testing.T) {
	f := newFixtureWith(t, withoutPending)
	parent := apptest.SampleTask
	step := addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000010", Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3}, Kind: item.Step, Rank: "V", Title: "s", Status: apptest.SampleStatusInProgress})
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusPending))
	})
	in := toMobile()
	preview, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, in)
	if err != nil || len(preview.Replacements) != 1 || preview.Replacements[0].Status != apptest.SampleStatusPending {
		t.Fatalf("rows = %+v, %v", preview.Replacements, err)
	}
	done := valueID(t, f.rec, "status", "Done")
	in.StatusChoices = map[change.ID]change.ID{apptest.SampleStatusPending: done, apptest.SampleStatusInProgress: done}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in); err != nil {
		t.Fatal(err)
	}
	if got := itemOf(t, f.rec, store1, apptest.SampleTask).Status; got != done {
		t.Errorf("行にある Pending = %q, want 選んだ Done", got)
	}
	if got := itemOf(t, f.rec, store1, step.ID).Status; got != valueID(t, f.rec, "status", "InProgress") {
		t.Errorf("行にない InProgress = %q, want 名前の一致", got)
	}
}

// 移動先に元の分類(Pending は未着手)の既定がないときは、Remap と同じく移動先の初期ステータスを初めの値にする。
func TestPreviewDefaultFallsBackToTheInitialStatus(t *testing.T) {
	f := newFixtureWith(t, func(st *workspace.Settings) {
		withoutPending(st)
		delete(st.CategoryDefaults, workspace.CategoryTodo)
		st.InitialStatus = st.Statuses[1].ID // InProgress
	})
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusPending))
	})
	got, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, toMobile())
	if err != nil || len(got.Replacements) != 1 {
		t.Fatalf("preview = %+v, %v", got, err)
	}
	if want := valueID(t, f.rec, "status", "InProgress"); got.Replacements[0].Default != want {
		t.Errorf("default = %q, want 初期ステータス %q", got.Replacements[0].Default, want)
	}
}
