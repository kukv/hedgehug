package repairing_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	mobileWS      workspace.ID = "01SAMPLEWS0000000000000009"
	mobileProject project.ID   = "01SAMPLEPR0000000000000009"
	mobileEpic    item.ID      = "01SAMPLEIT0000000000000009"
	earlyTask     item.ID      = "01AAAAAAAA0000000000000005"
)

// addMobile は、ワークスペース「Mobile」(MB)、プロジェクト MB-P1、Epic MB-1 と、番号の行(項目 1・プロジェクト 1)を足す。
func addMobile(e *env) {
	ws := workspace.Workspace{ID: mobileWS, Name: "Mobile", Prefix: "MB"}
	n := 0
	gen := func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01SAMPLEM9V%015d", n))
	}
	d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(gen))
	if err != nil {
		e.t.Fatal(err)
	}
	pr := project.Project{ID: mobileProject, Place: project.Place{Workspace: mobileWS, Number: 1}, Rank: "V", Title: "アプリ", Owner: "taro", Status: project.Todo}
	epic := item.Item{ID: mobileEpic, Place: item.Place{Workspace: mobileWS, Project: mobileProject, Number: 1}, Kind: item.Epic, Rank: "a", Title: "iOS", Priority: apptest.SamplePriorityMedium}
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(mobileWS, d, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
			tx.WriteItems().Append(epic.ID, item.Create(epic)),
		)
	})
}

// moveToMobile は、id を Mobile の MB-P1 の番号 n へ移す行(set place と Mobile の number n)を書く。親は変えない。
func (e *env) moveToMobile(id item.ID, n int) {
	old := e.item(id).Place
	to := old
	to.Workspace, to.Project, to.Number = mobileWS, mobileProject, n
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(id, change.NewSet("place", old, to)),
			tx.WriteWorkspaces().Append(mobileWS, numbering.Draft(numbering.Items, n)),
		)
	})
}

// moveEpicToMobile は、Epic HH-1 だけを Mobile の MB-P1 の番号 2 へ移す。HH-2 は Web に残る。
func moveEpicToMobile(e *env) { e.moveToMobile(apptest.SampleEpic, 2) }

// logOf は、ref のログ。
func (e *env) logOf(ref change.Ref) []change.Record {
	e.t.Helper()
	var log []change.Record
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(ref)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return log
}

func (e *env) transfers() []transfer.Record {
	e.t.Helper()
	var out []transfer.Record
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		out, err = r.Transfers().List()
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

func (e *env) mobileDoing() change.ID {
	e.t.Helper()
	var id change.ID
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		st, err := r.Workspaces().Settings(mobileWS)
		id = st.CategoryDefaults[workspace.CategoryDoing]
		return err
	})
	if err != nil || id == "" {
		e.t.Fatalf("mobile category default = %q, %v", id, err)
	}
	return id
}

func wantMobilePlace(t *testing.T, got item.Item, n int) {
	t.Helper()
	p := got.Place
	if p.Workspace != mobileWS || p.Project != mobileProject || p.Number != n || p.Parent == nil {
		t.Errorf("%s place = %+v, want mobile %s number %d with parent", got.ID, p, mobileProject, n)
	}
}

func wantParent(t *testing.T, got item.Item, parent item.ID) {
	t.Helper()
	if got.Place.Parent == nil || *got.Place.Parent != parent {
		t.Errorf("%s parent = %v, want %s", got.ID, got.Place.Parent, parent)
	}
}

// hasNumber は、log に項目の番号 n の行があるか。
func hasNumber(log []change.Record, n int) bool {
	want := change.Value(n)
	return slices.ContainsFunc(log, func(r change.Record) bool {
		return r.T == numbering.Type && r.F == string(numbering.Items) && string(r.To) == string(want)
	})
}

// Epic HH-1 を Mobile へ移すと HH-2 が Web に残る。HH-2 を Mobile の次の番号へ、親はそのままで移す。
func TestMatchWorkspacesMovesChild(t *testing.T) {
	e := newEnv(t)
	addMobile(e)
	moveEpicToMobile(e)
	before := map[change.Ref]int{}
	for _, ref := range []change.Ref{{Kind: change.KindItem, ID: apptest.SampleTask}, {Kind: change.KindWorkspace, ID: mobileWS}} {
		before[ref] = len(e.logOf(ref))
	}
	wantOps(t, e.run(nil, nil), "HH-2 を MB-3 に移動(親に合わせる)")
	got := e.item(apptest.SampleTask)
	wantMobilePlace(t, got, 3)
	wantParent(t, got, apptest.SampleEpic)
	if got.Status != e.mobileDoing() {
		t.Errorf("status = %q, want mobile's doing default %q", got.Status, e.mobileDoing())
	}
	want := transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "MB-3", ToStore: "S1"}
	rec := e.transfers()
	if len(rec) != 1 || rec[0].Entity != want.Entity || rec[0].Target != want.Target || rec[0].Old != want.Old || rec[0].New != want.New || rec[0].ToStore != want.ToStore {
		t.Errorf("transfers = %+v, want one like %+v", rec, want)
	}
	if !hasNumber(e.logOf(change.Ref{Kind: change.KindWorkspace, ID: mobileWS}), 3) {
		t.Error("mobile log has no number 3")
	}
	ops := map[change.ID]bool{}
	for ref, n := range before {
		for _, r := range e.logOf(ref)[n:] {
			if r.Actor.Kind != change.ActorSync {
				t.Errorf("repair row %+v is not by sync", r)
			}
			ops[r.Op] = true
		}
	}
	if len(ops) != 1 {
		t.Errorf("sync ops = %v, want one", ops)
	}
}

// 親 HH-2 が先に Mobile へ移ったとき、Web に残った Step HH-3 を直す。
func TestMatchWorkspacesMovesStep(t *testing.T) {
	e := newEnv(t)
	addStep(e)
	addMobile(e)
	moveEpicToMobile(e)
	e.moveToMobile(apptest.SampleTask, 3)
	wantOps(t, e.run(nil, nil), "HH-3 を MB-4 に移動(親に合わせる)")
	wantMobilePlace(t, e.item(step), 4)
	wantParent(t, e.item(step), apptest.SampleTask)
}

// 子を直したあと、その下の孫も次の保存で直る。
func TestMatchWorkspacesMovesChildThenGrandchild(t *testing.T) {
	e := newEnv(t)
	addStep(e)
	addMobile(e)
	moveEpicToMobile(e)
	wantOps(t, e.run(nil, nil), "HH-2 を MB-3 に移動(親に合わせる)", "HH-3 を MB-4 に移動(親に合わせる)")
	wantMobilePlace(t, e.item(apptest.SampleTask), 3)
	wantMobilePlace(t, e.item(step), 4)
	wantParent(t, e.item(step), apptest.SampleTask)
}

// 直す子の下の削除済みのものは、同じ保存で続きの番号に移る。削除済みのまま。
func TestMatchWorkspacesMovesDeletedBelow(t *testing.T) {
	e := newEnv(t)
	addStep(e)
	if err := apptest.DeleteItems(e.ctx, e.s, store, step); err != nil {
		t.Fatal(err)
	}
	addMobile(e)
	moveEpicToMobile(e)
	wantOps(t, e.run(nil, nil), "HH-2 を MB-3 に移動(親に合わせる)")
	wantMobilePlace(t, e.item(apptest.SampleTask), 3)
	deleted := e.item(step)
	wantMobilePlace(t, deleted, 4)
	if !strings.HasPrefix(string(deleted.Status), "01SAMPLEM9V") {
		t.Errorf("deleted step status = %q, want a mobile status", deleted.Status)
	}
	if deleted.Deleted == nil {
		t.Error("HH-3 was restored")
	}
	if rec := e.transfers(); len(rec) != 2 || rec[1].Old != "HH-3" || rec[1].New != "MB-4" {
		t.Errorf("transfers = %+v, want two with HH-3 -> MB-4 last", rec)
	}
}

// 親のプロジェクトが削除済みなら、先に削除の連鎖で子も削除済みになる。子は Web に残り、移さない。
func TestMatchWorkspacesAfterCascade(t *testing.T) {
	e := newEnv(t)
	addMobile(e)
	moveEpicToMobile(e)
	e.write(taro, func(tx port.Tx) error {
		return tx.WriteProjects().Append(mobileProject, change.NewDeleteEntity(true))
	})
	ops := e.run(nil, nil)
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if e.item(id).Deleted == nil {
			t.Errorf("%s is not deleted", id)
		}
	}
	if e.item(apptest.SampleTask).Place.Workspace != apptest.SampleWorkspace {
		t.Error("HH-2 was moved")
	}
	for _, op := range ops {
		if strings.HasSuffix(op.Summary, "に移動(親に合わせる)") {
			t.Errorf("unexpected move op %q", op.Summary)
		}
	}
}

// 同じ親の下の子は、旧い番号の順に Mobile の番号を振る(内部 ID の順ではない)。
func TestMatchWorkspacesTwoChildren(t *testing.T) {
	e := newEnv(t)
	epic := apptest.SampleEpic
	early := item.Item{ID: earlyTask, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &epic, Number: 5}, Kind: item.Task, Rank: "b", Title: "先の子", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium}
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(early.ID, item.Create(early)),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, 5)),
		)
	})
	addMobile(e)
	moveEpicToMobile(e)
	wantOps(t, e.run(nil, nil), "HH-2 を MB-3 に移動(親に合わせる)", "HH-5 を MB-4 に移動(親に合わせる)")
	wantMobilePlace(t, e.item(apptest.SampleTask), 3)
	wantMobilePlace(t, e.item(earlyTask), 4)
}

// 移す前のスプリントは、Web のものなので外れる。
func TestMatchWorkspacesRemovesSprint(t *testing.T) {
	e := newEnv(t)
	e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, change.ID("01SAMPLESP0000000000000001")))
	addMobile(e)
	moveEpicToMobile(e)
	e.run(nil, nil)
	got := e.item(apptest.SampleTask)
	wantMobilePlace(t, got, 3)
	if got.Sprint != "" {
		t.Errorf("sprint = %q, want empty", got.Sprint)
	}
}
