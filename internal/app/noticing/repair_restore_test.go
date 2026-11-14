package noticing_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

var (
	epicRef    = change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}
	taskRef    = change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}
	projectRef = change.Ref{Kind: change.KindProject, ID: apptest.SampleProject}
	wsRef      = change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}
)

// appendRef は、ref のログに drafts を足す保存の中身。
func appendRef(ref change.Ref, drafts ...change.Draft) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		switch ref.Kind {
		case change.KindItem:
			return tx.WriteItems().Append(ref.ID, drafts...)
		case change.KindProject:
			return tx.WriteProjects().Append(ref.ID, drafts...)
		}
		return tx.WriteWorkspaces().Append(ref.ID, drafts...)
	}
}

// restoring は、削除と変更の重なりの場面。prep(taro)のあと、deleter が ref に del を書き、hanako が ref に edit を書く。
type restoring struct {
	ref       change.Ref
	prep      []change.Draft
	del, edit change.Draft
}

// run は、場面を書き、削除の op を、取り消しの修復の保存(change.Undo)と一緒に返す。hanako の表示名は 花子。
func (s restoring) run(f *fixture, deleter string) (port.ReceivedSave, change.ID) {
	f.t.Helper()
	f.save(user("hanako"), func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Rename("hanako", "hanako", "花子"))
	})
	if len(s.prep) > 0 {
		f.save(user("taro"), appendRef(s.ref, s.prep...))
	}
	op := f.save(user(deleter), appendRef(s.ref, s.del)).Key.Op
	f.save(user("hanako"), appendRef(s.ref, s.edit))
	return f.repairSave(func(tx port.Tx) error {
		log, err := tx.Changes().Of(s.ref)
		if err != nil {
			return err
		}
		return appendRef(s.ref, change.Undo(log, op)...)(tx)
	}), op
}

func itemDeleted() restoring {
	return restoring{ref: epicRef, del: change.NewDeleteEntity(true), edit: change.NewSet("title", "認証", "認証2")}
}

func repairsOf(ns []port.Notice) []port.Notice {
	var out []port.Notice
	for _, n := range ns {
		if n.Kind == port.NoticeRepair {
			out = append(out, n)
		}
	}
	return out
}

func TestRestoreItem(t *testing.T) {
	f := setupAs(t, "taro")
	s, op := itemDeleted().run(f, "taro")
	f.judge(s)
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleEpic,
		Text: "HH-1 は 花子 が同時に変更したため、削除を取り消しました", Display: "HH-1", Source: "repair:restore:" + string(op),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestRestoreVariants(t *testing.T) {
	statusName := "statuses/" + string(apptest.SampleStatusPending)
	tests := []struct {
		name    string
		s       restoring
		text    string
		check   func(port.Notice) bool
		section string
	}{
		{
			"project",
			restoring{ref: projectRef, del: change.NewDeleteEntity(true), edit: change.NewSet("title", "ログイン", "ログイン2")},
			"HH-P1 は 花子 が同時に変更したため、削除を取り消しました",
			func(n port.Notice) bool {
				return n.Project == apptest.SampleProject && n.Item == "" && n.Display == "HH-P1"
			}, "",
		},
		{
			"workspace",
			restoring{ref: wsRef, del: change.NewDeleteEntity(true), edit: change.NewSet(statusName+"/name", "Pending", "保留")},
			"ワークスペース「Web」は 花子 が同時に変更したため、削除を取り消しました", nil, "general",
		},
		{
			"status",
			restoring{ref: wsRef, del: change.NewDelete(statusName), edit: change.NewSet(statusName+"/name", "Pending", "Review")},
			"「Review」は 花子 が同時に変更したため、削除を取り消しました", nil, "statuses",
		},
		{
			"sprint",
			restoring{
				ref: wsRef, prep: planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: "SP1", Name: "Sprint\n  1"}),
				del: planning.DeleteSprint("SP1"), edit: change.NewSet("sprints/SP1/goal", "", "目標"),
			},
			"「Sprint 1」は 花子 が同時に変更したため、削除を取り消しました", nil, "sprints",
		},
		{
			"release",
			restoring{
				ref: wsRef, prep: planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: "RL1", Name: "v1"}),
				del: planning.DeleteRelease("RL1"), edit: change.NewSet("releases/RL1/date", "", "2026-12-01"),
			},
			"「v1」は 花子 が同時に変更したため、削除を取り消しました", nil, "releases",
		},
		{
			"rule",
			restoring{
				ref: wsRef, prep: automation.Drafts(automation.Rule{}, automation.Rule{ID: "AU1", Name: "閉じる"}),
				del: automation.DeleteDraft("AU1"), edit: change.NewSet(automation.Field("AU1", "enabled"), false, true),
			},
			"「閉じる」は 花子 が同時に変更したため、削除を取り消しました", nil, "automation",
		},
		{
			"comment",
			restoring{
				ref: epicRef, prep: []change.Draft{change.NewComment("CM1", nil, comment.Value("やあ"))},
				del: change.Draft{T: change.Delete, F: "CM1"}, edit: change.NewComment("CM1", comment.Value("やあ"), comment.Value("やあ!")),
			},
			"HH-1 のコメントは 花子 が同時に変更したため、削除を取り消しました",
			func(n port.Notice) bool { return n.Item == apptest.SampleEpic && n.Display == "HH-1" }, "",
		},
		{
			"worklog",
			restoring{
				ref: epicRef, prep: []change.Draft{change.NewWorklog("WL1", nil, worklog.Value(100, "", "2026-11-14"))},
				del: change.Draft{T: change.Delete, F: "WL1"}, edit: change.NewWorklog("WL1", worklog.Value(100, "", "2026-11-14"), worklog.Value(200, "", "2026-11-14")),
			},
			"HH-1 の作業時間は 花子 が同時に変更したため、削除を取り消しました",
			func(n port.Notice) bool { return n.Item == apptest.SampleEpic }, "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setupAs(t, "taro")
			s, op := tt.s.run(f, "taro")
			f.judge(s)
			ns := repairsOf(f.notices())
			if len(ns) != 1 {
				t.Fatalf("repair notices = %+v, want one", ns)
			}
			n := ns[0]
			if n.Text != tt.text || n.Section != tt.section || n.Workspace != apptest.SampleWorkspace || n.Sender != "" || n.Source != "repair:restore:"+string(op) {
				t.Errorf("notice = %+v, want text %q section %q", n, tt.text, tt.section)
			}
			if tt.check != nil && !tt.check(n) {
				t.Errorf("notice = %+v, wrong target", n)
			}
		})
	}
}

// undoAll は、op の行があるログすべてに change.Undo の行を書く修復の保存。
func undoAll(f *fixture, op change.ID, refs ...change.Ref) port.ReceivedSave {
	return f.repairSave(func(tx port.Tx) error {
		for _, ref := range refs {
			log, err := tx.Changes().Of(ref)
			if err != nil {
				return err
			}
			if err := appendRef(ref, change.Undo(log, op)...)(tx); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestRestoreWorkspaceOnePerOp(t *testing.T) {
	f := setupAs(t, "taro")
	op := f.save(user("taro"), func(tx port.Tx) error {
		return errors.Join(
			appendRef(wsRef, change.NewDeleteEntity(true))(tx),
			appendRef(projectRef, change.NewDeleteEntity(false))(tx),
			appendRef(epicRef, change.NewDeleteEntity(false))(tx),
			appendRef(taskRef, change.NewDeleteEntity(false))(tx),
		)
	}).Key.Op
	f.save(user("hanako"), appendRef(wsRef, change.NewSet("name", "Web", "Web2")))
	f.judge(undoAll(f, op, wsRef, projectRef, epicRef, taskRef))
	n := f.wantOne()
	if n.Text != "ワークスペース「Web2」は hanako が同時に変更したため、削除を取り消しました" || n.Item != "" || n.Project != "" || n.Section != "general" {
		t.Errorf("notice = %+v", n)
	}
}

// 1 つの op で 2 つを削除し、片方だけが変更された: 変更された方を対象にし、{変更した人} はその変更の人。
func TestRestoreTwoDeletesInOneOp(t *testing.T) {
	f := setupAs(t, "taro")
	f.save(user("taro"), appendRef(wsRef, append(
		planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: "SP1", Name: "Sprint 1"}),
		planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: "SP2", Name: "Sprint 2"})...)...))
	op := f.save(user("taro"), appendRef(wsRef, planning.DeleteSprint("SP1"), planning.DeleteSprint("SP2"))).Key.Op
	f.save(user("hanako"), appendRef(wsRef, change.NewSet("sprints/SP2/goal", "", "目標")))
	f.judge(undoAll(f, op, wsRef))
	if n := f.wantOne(); n.Text != "「Sprint 2」は hanako が同時に変更したため、削除を取り消しました" {
		t.Errorf("text = %q", n.Text)
	}
}

func TestRestoreUnknownChanger(t *testing.T) {
	f := setupAs(t, "taro")
	s := restoring{ref: epicRef, del: change.NewDeleteEntity(true), edit: change.NewSet("title", "認証", "認証2").ByRule("r1")}
	r, _ := s.run(f, "taro")
	f.judge(r)
	if n := f.wantOne(); n.Text != "HH-1 は 不明なユーザー が同時に変更したため、削除を取り消しました" {
		t.Errorf("text = %q", n.Text)
	}
}

func TestRestoreChangerNotMember(t *testing.T) {
	f := setupAs(t, "taro")
	s := restoring{ref: epicRef, del: change.NewDeleteEntity(true), edit: change.NewSet("title", "認証", "認証2")}
	f.save(user("hanako"), func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Rename("hanako", "hanako", "花子"))
	})
	op := f.save(user("taro"), appendRef(s.ref, s.del)).Key.Op
	f.save(user("jiro"), appendRef(s.ref, s.edit))
	r := f.repairSave(appendRef(epicRef, change.NewRestore(change.DeletedField, op)))
	f.judge(r)
	if n := f.wantOne(); n.Text != "HH-1 は jiro が同時に変更したため、削除を取り消しました" {
		t.Errorf("text = %q", n.Text)
	}
}

func TestRestoreNotDeleter(t *testing.T) {
	f := setupAs(t, "taro")
	s, _ := itemDeleted().run(f, "hanako")
	f.judge(s)
	f.wantNone()
}

func TestRestoreOnceAcrossMachines(t *testing.T) {
	f := setupAs(t, "taro")
	s, op := itemDeleted().run(f, "taro")
	other := f.repairSave(appendRef(epicRef, change.NewRestore(change.DeletedField, op)))
	f.judge(s, other)
	f.judge(s, other)
	if rows := f.rows(); len(rows) != 1 {
		t.Errorf("rows = %+v, want one", rows)
	}
}

func TestRestoreByPersonSkipped(t *testing.T) {
	f := setupAs(t, "taro")
	op := f.save(user("taro"), appendRef(epicRef, change.NewDeleteEntity(true))).Key.Op
	f.save(user("hanako"), appendRef(epicRef, change.NewSet("title", "認証", "認証2")))
	f.judge(f.save(user("hanako"), appendRef(epicRef, change.NewRestore(change.DeletedField, op))))
	if ns := repairsOf(f.notices()); len(ns) != 0 {
		t.Errorf("repair notices = %+v, want none", ns)
	}
}

// cascading は、deleter が parent を削除し、writer が child を変更したあとの、親の削除の修復の保存。
func cascading(f *fixture, deleter, writer string, parent, child change.Ref, edit change.Draft) (port.ReceivedSave, change.ID) {
	f.t.Helper()
	op := f.save(user(deleter), appendRef(parent, change.NewDeleteEntity(true))).Key.Op
	f.save(user(writer), appendRef(child, edit))
	return f.repairSave(appendRef(child, change.NewDeleteEntity(false).InOp(op))), op
}

func TestCascadeNotice(t *testing.T) {
	f := setupAs(t, "taro")
	s, op := cascading(f, "hanako", "taro", epicRef, taskRef, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	f.judge(s)
	want := port.Notice{
		Kind: port.NoticeRepair, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask,
		Text: "HH-1 が削除されたため、HH-2 も削除済みに入りました", Display: "HH-2",
		Source: "repair:cascade:" + string(apptest.SampleTask) + ":" + string(op),
	}
	if n := f.wantOne(); n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestCascadeWorkspaceParent(t *testing.T) {
	f := setupAs(t, "taro")
	s, op := cascading(f, "hanako", "taro", wsRef, projectRef, change.NewSet("title", "ログイン", "ログイン2"))
	f.judge(s)
	n := f.wantOne()
	if n.Text != "ワークスペース「Web」が削除されたため、HH-P1 も削除済みに入りました" || n.Project != apptest.SampleProject ||
		n.Source != "repair:cascade:"+string(apptest.SampleProject)+":"+string(op) {
		t.Errorf("notice = %+v", n)
	}
}

func TestCascadeNotLastWriter(t *testing.T) {
	f := setupAs(t, "taro")
	s, _ := cascading(f, "hanako", "hanako", epicRef, taskRef, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	f.judge(s)
	f.wantNone()
}

func TestRepairSkipsReadOnly(t *testing.T) {
	f := setupAs(t, "taro")
	s, _ := itemDeleted().run(f, "taro")
	a := f.save(user("hanako"), setTask(assign("taro")))
	if err := f.judgeReceived(port.Received{Saves: []port.ReceivedSave{s, a}, ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	ns := f.notices()
	if len(repairsOf(ns)) != 0 || !slices.ContainsFunc(ns, func(n port.Notice) bool { return n.Kind == port.NoticeAssign }) {
		t.Errorf("notices = %+v, want assign and no repair", ns)
	}
}
