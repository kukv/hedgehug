package noticing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// planRows は、by が ds をワークスペース「Web」に書いた保存の、ワークスペースの行。
func planRows(f *fixture, by string, ds []change.Draft) []change.Record {
	f.t.Helper()
	s := f.save(user(by), func(tx port.Tx) error { return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...) })
	for _, e := range s.Entities {
		if e.Ref.Kind == change.KindWorkspace {
			return e.Rows
		}
	}
	f.t.Fatal("no workspace rows")
	return nil
}

// droppedPlan は、合わせで target の行が取り除かれた合わせ。log はその側の行、newRows は共通の祖先にない行。
func droppedPlan(target change.ID, log, newRows []change.Record) port.ReceivedSave {
	return port.ReceivedSave{
		Key: port.SaveKey{Version: "merge"}, Merge: true,
		DroppedPlan: []port.DroppedPlan{{Workspace: apptest.SampleWorkspace, Target: target, Log: log, New: newRows}},
	}
}

func sprintDrafts(name string) []change.Draft {
	return planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: sprintB, Name: name, State: planning.Planned})
}

func TestDroppedPlanNotice(t *testing.T) {
	cases := []struct {
		name, section, text, purgedKind, display string
		target                                   change.ID
		drafts                                   []change.Draft
	}{
		{"sprint", "sprints", "「Sprint 3」は完全に削除されたため、あなたの変更は反映されませんでした", "sprint", "Sprint 3", sprintB, sprintDrafts("Sprint 3")},
		{
			"release", "releases", "「v3」は完全に削除されたため、あなたの変更は反映されませんでした", "release", "v3", releaseA,
			planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: releaseA, Name: "v3", State: planning.Unreleased}),
		},
		{"purged display empty", "sprints", "「Sprint 3」は完全に削除されたため、あなたの変更は反映されませんでした", "sprint", "", sprintB, sprintDrafts("Sprint 3")},
		{"log name", "sprints", "「Sprint x」は完全に削除されたため、あなたの変更は反映されませんでした", "", "", sprintB, sprintDrafts("Sprint\n x")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setup(t)
			rows := planRows(f, "hanako", c.drafts)
			if c.purgedKind != "" {
				f.purge(c.purgedKind, c.target, c.display)
			}
			f.judge(droppedPlan(c.target, rows, rows))
			want := port.Notice{
				Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Section: c.section,
				Text: c.text, Source: "repair:purged:" + string(c.target),
			}
			if n := f.wantOne(); n != want {
				t.Errorf("notice = %+v, want %+v", n, want)
			}
		})
	}
}

func TestDroppedPlanNoName(t *testing.T) {
	f := setup(t)
	f.judge(droppedPlan(sprintB, nil, planRows(f, "hanako", []change.Draft{change.NewSet("sprints/"+string(sprintB)+"/goal", "", "x")})))
	f.wantNone()
}

func TestDroppedPlanOnlyNewRows(t *testing.T) {
	f := setup(t)
	old := planRows(f, "hanako", sprintDrafts("Sprint 3"))
	f.purge("sprint", sprintB, "Sprint 3")
	fresh := planRows(f, "jiro", []change.Draft{change.NewSet("sprints/"+string(sprintB)+"/name", "Sprint 3", "Sprint 4")})
	f.judge(droppedPlan(sprintB, append(old, fresh...), fresh))
	f.wantNone()
}

func TestDroppedPlanNotHuman(t *testing.T) {
	f := setup(t)
	rows := f.save(syncActor, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, sprintDrafts("Sprint 3")...)
	}).Entities
	var log []change.Record
	for _, e := range rows {
		if e.Ref.Kind == change.KindWorkspace {
			log = e.Rows
		}
	}
	f.purge("sprint", sprintB, "Sprint 3")
	f.judge(droppedPlan(sprintB, log, log))
	f.wantNone()
}

func TestDroppedPlanOnce(t *testing.T) {
	f := setup(t)
	rows := planRows(f, "hanako", sprintDrafts("Sprint 3"))
	f.purge("sprint", sprintB, "Sprint 3")
	s := droppedPlan(sprintB, rows, rows)
	f.judge(s)
	f.judge(s)
	if got := f.rows(); len(got) != 1 {
		t.Fatalf("rows = %+v, want one", got)
	}
	if err := f.inbox.Prune(t.Context(), store, []change.ID{apptest.SampleWorkspace}); err != nil {
		t.Fatal(err)
	}
	if got := f.rows(); len(got) != 1 {
		t.Errorf("rows after prune = %+v, want one", got)
	}
}
