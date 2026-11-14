package noticing_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	item4    item.ID   = "01PLACEIT00000000000000004"
	sprintA  change.ID = "01SPRINT000000000000000001"
	sprintB  change.ID = "01SPRINT000000000000000002"
	releaseA change.ID = "01RELEASE00000000000000001"
	gonePlan change.ID = "01GONEPLAN0000000000000001"
)

// writePlan は、by がワークスペース ws に ds の行を書く保存。
func writePlan(f *fixture, by string, ws workspace.ID, ds ...change.Draft) {
	f.t.Helper()
	f.save(user(by), func(tx port.Tx) error { return tx.WriteWorkspaces().Append(ws, ds...) })
}

// addSprint は、taro がワークスペース「Web」にスプリント s を作る。
func addSprint(f *fixture, s planning.Sprint) {
	f.t.Helper()
	writePlan(f, "taro", apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, s)...)
}

// setItem は、by がアイテム id に ds の行を書く保存。
func setItem(f *fixture, by string, id item.ID, ds ...change.Draft) {
	f.t.Helper()
	f.save(user(by), func(tx port.Tx) error { return tx.WriteItems().Append(id, ds...) })
}

// lastRow は、ref のログで、f の項目の最後の行(作成なら作成の行)の ID。
func (f *fixture) lastRow(ref change.Ref, field string) string {
	f.t.Helper()
	var id change.ID
	for _, c := range f.logOfRef(ref) {
		if c.Actor.Kind == change.ActorUser && (c.F == field || c.T == change.Create) {
			id = c.ID
		}
	}
	return string(id)
}

// twoActive は、taro が Sprint 1 を実行中で作り、Sprint 2 を計画中で作り、starter が Sprint 2 を開始する。
func twoActive(f *fixture, starter string) {
	f.t.Helper()
	addSprint(f, planning.Sprint{ID: sprintA, Name: "Sprint 1", State: planning.Active})
	planned := planning.Sprint{ID: sprintB, Name: "Sprint\n 2", State: planning.Planned}
	addSprint(f, planned)
	active := planned
	active.State = planning.Active
	writePlan(f, starter, apptest.SampleWorkspace, planning.SprintDrafts(planned, active)...)
}

// stopRepair は、Sprint 2 を計画中に戻す修復の保存。
func stopRepair(f *fixture) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("sprints/"+string(sprintB)+"/state", planning.Active, planning.Planned))
	})
}

func TestActiveNotice(t *testing.T) {
	f := setup(t)
	twoActive(f, "hanako")
	start := f.lastRow(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}, "sprints/"+string(sprintB)+"/state")
	f.judge(stopRepair(f))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Section: "sprints",
		Text:   "実行中のスプリントが 2 つになったため、「Sprint 2」を計画中に戻しました",
		Source: "repair:active:" + string(sprintB) + ":" + start,
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestActiveNoticeOtherStarter(t *testing.T) {
	f := setup(t)
	twoActive(f, "taro")
	f.judge(stopRepair(f))
	f.wantNone()
}

// 2 台が同じスプリントを計画中に戻した: 2 つ目の修復の保存は、同じ行をもう一度書く。どちらも開始の行が鍵になる。
// このマシンのログに自分の修復の行(other)があるまま、もう 1 台の修復(s)を受け取っても、開始の行を鍵に作る。
func TestActiveOnceAcrossMachines(t *testing.T) {
	f := setup(t)
	twoActive(f, "hanako")
	start := f.lastRow(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}, "sprints/"+string(sprintB)+"/state")
	s, other := stopRepair(f), stopRepair(f)
	f.judge(s)
	if n := f.wantOne(); n.Source != "repair:active:"+string(sprintB)+":"+start {
		t.Errorf("source = %q", n.Source)
	}
	f.judge(s, other)
	f.judge(other)
	if rows := f.rows(); len(rows) != 1 {
		t.Errorf("rows = %+v, want one", rows)
	}
}

// hanako が開始 → 修復で計画中 → hanako がもう一度開始 → もう一度修復。2 つ目の通知は、もう一度の開始の行が鍵になる。
func TestActiveNoticeRestarted(t *testing.T) {
	f := setup(t)
	twoActive(f, "hanako")
	ws := change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}
	field := "sprints/" + string(sprintB) + "/state"
	first := f.lastRow(ws, field)
	f.judge(stopRepair(f))
	writePlan(f, "hanako", apptest.SampleWorkspace, change.NewSet(field, planning.Planned, planning.Active))
	again := f.lastRow(ws, field)
	f.judge(stopRepair(f))
	got := f.notices()
	if len(got) != 2 || again == first {
		t.Fatalf("notices = %+v, first = %s, again = %s", got, first, again)
	}
	sources := []string{got[0].Source, got[1].Source}
	for _, start := range []string{first, again} {
		if !slices.Contains(sources, "repair:active:"+string(sprintB)+":"+start) {
			t.Errorf("sources = %v, want one keyed by %s", sources, start)
		}
	}
}

// closedSprint は、taro が Sprint 24 を実行中で作り、adder が id をそこに入れ、taro が完了する。
func closedSprint(f *fixture, adder string, id item.ID, n int) {
	f.t.Helper()
	sp := planning.Sprint{ID: sprintA, Name: "Sprint 24", State: planning.Active}
	addSprint(f, sp)
	addItem(f, adder, webItem(id, n, item.Task, apptest.SampleEpic))
	setItem(f, adder, id, change.NewSet(item.SprintField, nil, sprintA))
	closed := sp
	closed.State = planning.Closed
	writePlan(f, "taro", apptest.SampleWorkspace, append([]change.Draft{planning.CloseDraft(sprintA, planning.Confirmed{})}, planning.SprintDrafts(sp, closed)...)...)
}

// clearRepair は、id の欄 field(いまの値 from)を空にする修復の保存。
func clearRepair(f *fixture, id item.ID, field string, from change.ID) port.ReceivedSave {
	f.t.Helper()
	return f.repairSave(func(tx port.Tx) error { return tx.WriteItems().Append(id, change.NewSet(field, from, nil)) })
}

func TestClosedNotice(t *testing.T) {
	f := setup(t)
	closedSprint(f, "hanako", item9, 9)
	row := f.lastRow(itemRefOf(item9), item.SprintField)
	f.judge(clearRepair(f, item9, item.SprintField, sprintA))
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: item9, Display: "HH-9",
		Text:   "スプリント「Sprint 24」は完了しているため、HH-9 をバックログに戻しました",
		Source: "repair:closed:" + string(item9) + ":" + row,
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

// HH-7 を Sprint 24 に入れたのは hanako、完了と同時に Done から In Progress に戻したのは taro。宛先は入れた人。
func TestClosedNoticeToAdder(t *testing.T) {
	for me, want := range map[string]int{"hanako": 1, "taro": 0} {
		t.Run(me, func(t *testing.T) {
			f := setupAs(t, me)
			closedSprint(f, "hanako", item7, 7)
			setItem(f, "hanako", item7, setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone))
			setItem(f, "taro", item7, setStatus(apptest.SampleStatusDone, apptest.SampleStatusInProgress))
			f.judge(clearRepair(f, item7, item.SprintField, sprintA))
			if got := f.notices(); len(got) != want {
				t.Errorf("notices = %+v, want %d", got, want)
			}
		})
	}
}

// dangling は、hanako が HH-4 を作り、欄 field を target にする。
func dangling(f *fixture, field string, target change.ID) {
	f.t.Helper()
	addItem(f, "hanako", webItem(item4, 4, item.Task, apptest.SampleEpic))
	setItem(f, "hanako", item4, change.NewSet(field, nil, target))
}

func TestPlanRefNotices(t *testing.T) {
	inMobile := func(ds ...change.Draft) func(f *fixture) {
		return func(f *fixture) {
			if err := apptest.SampleMobile(f.t.Context(), f.stores, store); err != nil {
				f.t.Fatal(err)
			}
			writePlan(f, "taro", apptest.SampleMobileWorkspace, ds...)
		}
	}
	cases := map[string]struct {
		field string
		prep  func(f *fixture)
		want  string
	}{
		"完全に削除したスプリント": {
			field: item.SprintField, prep: func(f *fixture) { f.purge("sprint", gonePlan, "Sprint\n3") },
			want: "スプリント「Sprint 3」は完全に削除されたため、HH-4 をバックログに戻しました",
		},
		"完全に削除したリリース": {
			field: item.ReleaseField, prep: func(f *fixture) { f.purge("release", gonePlan, "v2") },
			want: "リリース「v2」は完全に削除されたため、HH-4 のリリースを外しました",
		},
		"別のワークスペースのスプリント": {
			field: item.SprintField,
			prep:  inMobile(planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: gonePlan, Name: "Sprint 5", State: planning.Planned})...),
			want:  "スプリント「Sprint 5」は HH-4 のワークスペースにないため、HH-4 をバックログに戻しました",
		},
		"別のワークスペースのリリース": {
			field: item.ReleaseField,
			prep:  inMobile(planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: gonePlan, Name: "v3", State: planning.Unreleased})...),
			want:  "リリース「v3」は HH-4 のワークスペースにないため、HH-4 のリリースを外しました",
		},
		"どこにもないスプリント": {
			field: item.SprintField, prep: func(*fixture) {},
			want: "スプリント「削除されたスプリント」は完全に削除されたため、HH-4 をバックログに戻しました",
		},
		"どこにもないリリース": {
			field: item.ReleaseField, prep: func(*fixture) {},
			want: "リリース「削除されたリリース」は完全に削除されたため、HH-4 のリリースを外しました",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			c.prep(f)
			dangling(f, c.field, gonePlan)
			row := f.lastRow(itemRefOf(item4), c.field)
			f.judge(clearRepair(f, item4, c.field, gonePlan))
			want := port.Notice{
				Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: item4, Display: "HH-4",
				Text: c.want, Source: "repair:planref:" + string(item4) + ":" + c.field + ":" + row,
			}
			if n := f.wantOne(); n != want {
				t.Errorf("notice = %+v, want %+v", n, want)
			}
		})
	}
}

// スプリントとリリースが両方ない: 保存は 2 つ、通知も 2 行。
func TestPlanRefBothFields(t *testing.T) {
	f := setup(t)
	dangling(f, item.SprintField, sprintB)
	setItem(f, "hanako", item4, change.NewSet(item.ReleaseField, nil, releaseA))
	f.judge(clearRepair(f, item4, item.SprintField, sprintB), clearRepair(f, item4, item.ReleaseField, releaseA))
	if rows := f.rows(); len(rows) != 2 || rows[0].Source == rows[1].Source {
		t.Errorf("rows = %+v, want two with different keys", rows)
	}
}

// 修復 (5) の別のワークスペースへの移動の保存(set place と set sprint null)からは、(8) の通知を作らない。
func TestPlanRefSkipsPlaceRepair(t *testing.T) {
	f := setup(t)
	if err := apptest.SampleMobile(t.Context(), f.stores, store); err != nil {
		t.Fatal(err)
	}
	addSprint(f, planning.Sprint{ID: sprintA, Name: "Sprint 1", State: planning.Active})
	addItem(f, "taro", webItem(item4, 4, item.Task, ""))
	setItem(f, "hanako", item4, change.NewSet(item.SprintField, nil, sprintA))
	f.judge(f.repairSave(func(tx port.Tx) error {
		it, err := tx.Items().Get(item4)
		if err != nil {
			return err
		}
		to := item.Place{Workspace: apptest.SampleMobileWorkspace, Project: apptest.SampleMobileProject, Number: 2}
		return tx.WriteItems().Append(item4, change.NewSet("place", it.Place, to), change.NewSet(item.SprintField, sprintA, nil))
	}))
	f.wantNone()
}

// スプリントの欄を最後に保存したのが taro(作成の行は hanako)なら、hanako には作らない。
func TestPlanRefLastSaver(t *testing.T) {
	f := setup(t)
	addItem(f, "hanako", webItem(item4, 4, item.Task, apptest.SampleEpic))
	setItem(f, "taro", item4, change.NewSet(item.SprintField, nil, sprintB))
	f.judge(clearRepair(f, item4, item.SprintField, sprintB))
	f.wantNone()
}
