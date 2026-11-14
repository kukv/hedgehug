package transfers_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 移すワークスペースは main(S1)のサンプル「Web」(HH)。移動先は team(S2)。
const (
	wsGoneProject project.ID   = "01TESTPR000000000000000020" // Web の削除済みのプロジェクト
	wsGoneItem    item.ID      = "01TESTIT000000000000000020" // その下のアイテム(自分は削除されていない)
	liveSprint    change.ID    = "01TESTSP000000000000000001"
	goneSprint    change.ID    = "01TESTSP000000000000000002"
	teamDeletedWS workspace.ID = "01TESTWS000000000000000020"

	teamRemote = "https://example.invalid/team.git"
)

// webAt は、Store store のワークスペース「Web」(削除済みを含む)。なければ ok=false。
func webAt(t *testing.T, s port.Stores, store port.StoreID) (workspace.Workspace, bool) {
	t.Helper()
	var w workspace.Workspace
	err := s.Read(t.Context(), store, func(r port.Reader) (err error) { w, err = r.Workspaces().Get(apptest.SampleWorkspace); return })
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		t.Fatal(err)
	}
	return w, err == nil
}

// setRank は、Store store のワークスペース ws の rank を r にする。
func setRank(t *testing.T, s port.Stores, store port.StoreID, ws workspace.ID, r string) {
	t.Helper()
	write(t, s, store, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(ws, change.NewSet("rank", nil, r)) })
}

// withWorkspaceContents は、Web に、削除済みのプロジェクト(下にアイテム)、別に削除した Task HH-2、スプリント 2 つ(1 つは
// 削除済み)、プロジェクト「ログイン」のプロジェクト別のルールを足す。team には Team の rank「X」と、削除済みのワークスペース
// (rank「Y」)を置く(移したワークスペースの rank は、削除済みを含めた最後の後ろの「Z」)。
func withWorkspaceContents(t *testing.T, f fixture) {
	t.Helper()
	ws := apptest.SampleWorkspace
	p := project.Project{ID: wsGoneProject, Place: project.Place{Workspace: ws, Number: 2}, Rank: "W", Title: "古い", Owner: "taro", Status: project.Todo}
	it := item.Item{ID: wsGoneItem, Place: item.Place{Workspace: ws, Project: wsGoneProject, Number: 3}, Kind: item.Epic, Rank: "V", Title: "古いもの"}
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join(tx.WriteProjects().Append(wsGoneProject, project.Create(p)), tx.WriteItems().Append(wsGoneItem, item.Create(it)))
	})
	if err := apptest.DeleteProject(t.Context(), f.rec, store1, wsGoneProject); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	write(t, f.rec, store1, func(tx port.Tx) error {
		ds := planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: liveSprint, Name: "Sprint 1", State: planning.Planned, Start: "2026-11-01", End: "2026-11-14"})
		ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: goneSprint, Name: "Sprint 0", State: planning.Planned})...)
		ds = append(ds, change.NewAdd(projectrule.WorkflowField(apptest.SampleProject, apptest.SampleStatusTodo), apptest.SampleStatusDone))
		return tx.WriteWorkspaces().Append(ws, ds...)
	})
	write(t, f.rec, store1, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(ws, planning.DeleteSprint(goneSprint)) })
	setRank(t, f.rec, store2, teamWS, "X")
	addWorkspace(t, f.rec, store2, workspace.Workspace{ID: teamDeletedWS, Name: "Old", Prefix: "OLD"}, "", "", nil)
	setRank(t, f.rec, store2, teamDeletedWS, "Y")
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store2, teamDeletedWS); err != nil {
		t.Fatal(err)
	}
}

// B5c-6・B5c-8・B5c-11・Review Focus 3: 移動先にワークスペース・プロジェクト・アイテム(削除済みを含む)のフォルダ(行はそのまま
// + moved_in)、rank、足した人の join。移動元はフォルダが消え、転送記録が 1 行。アクティビティは Store ごとの文。
func TestMoveWorkspaceWritesBothStores(t *testing.T) {
	f := newFixture(t)
	f.svc.SyncTargets = &apptest.SyncTargets{Targets: []port.SyncTarget{
		{ID: store1, Name: "main"}, {ID: store2, Name: "team", Remote: teamRemote}, {ID: store3, Name: "readonly", ReadOnly: true},
	}}
	withWorkspaceContents(t, f)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("assignee", nil, "shiro"))
	})
	srcLogs := map[change.Ref][]change.Record{}
	for _, ref := range webRefs() {
		srcLogs[ref] = logOf(t, f.rec, store1, ref)
	}
	got, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2)
	if err != nil {
		t.Fatal(err)
	}
	if want := (transfers.WorkspaceMoved{ToStore: store2, ToStoreName: "team", Workspace: apptest.SampleWorkspace, Prefix: "HH"}); got != want {
		t.Errorf("moved = %+v, want %+v", got, want)
	}
	at := assertWorkspaceCopied(t, f, srcLogs)
	assertWorkspaceContents(t, f, srcLogs)
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro", "shiro"}) {
		t.Errorf("destination members = %v", got)
	}
	wsLog := logOf(t, f.rec, store2, change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
	movedIn := slices.IndexFunc(wsLog, func(r change.Record) bool { return r.T == change.MovedIn })
	if members := logOf(t, f.rec, store2, change.Ref{Kind: change.KindMembers}); movedIn < 0 || members[len(members)-1].Op != wsLog[movedIn].Op {
		t.Error("the join of shiro is not in the same operation as the move")
	}
	for _, ref := range webRefs() {
		err := f.rec.Read(t.Context(), store1, func(r port.Reader) error { _, err := r.Changes().Of(ref); return err })
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%v remains in the source (%v)", ref, err)
		}
	}
	want := transfer.Record{Entity: transfer.EntityWorkspace, Target: apptest.SampleWorkspace, Old: "HH", New: "HH", ToStore: string(store2), ToStoreName: "team", ToRemote: teamRemote}
	recs := recordsOf(t, f.rec, store1)
	if len(recs) != 1 || !recs[0].At.Equal(at) {
		t.Fatalf("records = %+v", recs)
	}
	if r := recs[0]; r.Entity != want.Entity || r.Target != want.Target || r.Old != want.Old || r.New != want.New ||
		r.ToStore != want.ToStore || r.ToStoreName != want.ToStoreName || r.ToRemote != want.ToRemote {
		t.Errorf("record = %+v", r)
	}
	if got := f.rec.summaries(store1); got[len(got)-1] != "ワークスペース「Web」を team へ移動" {
		t.Errorf("source summary = %q", got[len(got)-1])
	}
	if got := f.rec.summaries(store2); got[len(got)-1] != "ワークスペース「Web」を main から移動" {
		t.Errorf("destination summary = %q", got[len(got)-1])
	}
}

// webRefs は、Web のワークスペース・プロジェクト・アイテムのログを指す Ref。
func webRefs() []change.Ref {
	refs := []change.Ref{{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}}
	for _, p := range []project.ID{apptest.SampleProject, wsGoneProject} {
		refs = append(refs, change.Ref{Kind: change.KindProject, ID: p})
	}
	for _, it := range []item.ID{apptest.SampleEpic, apptest.SampleTask, wsGoneItem} {
		refs = append(refs, change.Ref{Kind: change.KindItem, ID: it})
	}
	return refs
}

// assertWorkspaceCopied は、移動先の各ログが移動元の行をそのまま先頭に持ち、moved_in が 1 行あることを確かめ、
// ワークスペースの moved_in の日時を返す(どの moved_in も同じ日時)。旧 ID と移動元のプロジェクトは、ワークスペース・
// プロジェクト「ログイン」・削除済みのプロジェクトの下のアイテムで確かめる。
func assertWorkspaceCopied(t *testing.T, f fixture, srcLogs map[change.Ref][]change.Record) time.Time {
	t.Helper()
	want := map[change.ID]transfer.MovedInInfo{
		apptest.SampleWorkspace: {Old: "HH"},
		apptest.SampleProject:   {Old: "HH-P1", FromProject: apptest.SampleProject, FromProjectTitle: "ログイン"},
		wsGoneItem:              {Old: "HH-3", FromProject: wsGoneProject, FromProjectTitle: "古い"},
	}
	var ats []time.Time
	for _, ref := range webRefs() {
		mi, ok := copiedMovedIn(t, f, ref, srcLogs[ref])
		if !ok {
			continue
		}
		ats = append(ats, mi.At)
		if w, ok := want[ref.ID]; ok && (mi.Old != w.Old || mi.FromProject != w.FromProject || mi.FromProjectTitle != w.FromProjectTitle) {
			t.Errorf("%v: moved_in = %+v, want %+v", ref, mi, w)
		}
	}
	if len(ats) == 0 || slices.ContainsFunc(ats, func(at time.Time) bool { return !at.Equal(ats[0]) }) {
		t.Fatalf("moved_in at = %v", ats)
	}
	return ats[0]
}

// copiedMovedIn は、移動先の ref のログが移動元の行 src をそのまま先頭に持ち、移動元が main の Web の moved_in を 1 行
// 持つことを確かめ、その moved_in を返す。
func copiedMovedIn(t *testing.T, f fixture, ref change.Ref, src []change.Record) (transfer.MovedInInfo, bool) {
	t.Helper()
	dst := logOf(t, f.rec, store2, ref)
	if len(dst) <= len(src) || dst[0].ID != src[0].ID || dst[len(src)-1].ID != src[len(src)-1].ID {
		t.Errorf("%v: the log is not copied", ref)
		return transfer.MovedInInfo{}, false
	}
	mi := transfer.MovedInOf(dst)
	if len(mi) != 1 || mi[0].FromStoreName != "main" || mi[0].FromWorkspace != apptest.SampleWorkspace || mi[0].FromWorkspaceName != "Web" {
		t.Errorf("%v: moved_in = %+v", ref, mi)
		return transfer.MovedInInfo{}, false
	}
	return mi[0], true
}

// assertWorkspaceContents は、移動先で、名前・接頭辞・rank・スプリント・プロジェクト別のルールが移動元と同じで、削除済みの
// プロジェクト・アイテム・スプリントが元の削除日時で削除済みであることを確かめる。
func assertWorkspaceContents(t *testing.T, f fixture, srcLogs map[change.Ref][]change.Record) {
	t.Helper()
	w, ok := webAt(t, f.rec, store2)
	if !ok || w.Name != "Web" || w.Prefix != "HH" || w.Deleted != nil {
		t.Errorf("destination workspace = %+v (%v)", w, ok)
	}
	wsLog := logOf(t, f.rec, store2, change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
	if r := workspace.RankOf(wsLog); r != "Z" {
		t.Errorf("rank = %q, want Z", r)
	}
	plan := planning.FromLog(wsLog)
	srcPlan := planning.FromLog(srcLogs[change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}])
	if len(plan.Sprints) != 2 || plan.Sprints[0].Deleted != nil || plan.Sprints[1].Deleted == nil ||
		!plan.Sprints[1].Deleted.At.Equal(srcPlan.Sprints[1].Deleted.At) {
		t.Errorf("sprints = %+v", plan.Sprints)
	}
	if rules := projectrule.FromLog(wsLog).Of(apptest.SampleProject); len(rules.Workflow) != 1 {
		t.Errorf("project rules = %+v", rules)
	}
	srcDeletion := func(ref change.Ref) *change.Deletion { return change.DeletionOf(srcLogs[ref]) }
	if p, ok := projectAt(t, f.rec, store2, wsGoneProject); !ok || p.Deleted == nil ||
		!p.Deleted.At.Equal(srcDeletion(change.Ref{Kind: change.KindProject, ID: wsGoneProject}).At) {
		t.Errorf("deleted project = %+v", p)
	}
	if it := itemOf(t, f.rec, store2, apptest.SampleTask); it.Deleted == nil ||
		!it.Deleted.At.Equal(srcDeletion(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}).At) {
		t.Errorf("deleted task = %+v", it.Deleted)
	}
	if it := itemOf(t, f.rec, store2, wsGoneItem); it.Place.Project != wsGoneProject {
		t.Errorf("item under the deleted project = %+v", it.Place)
	}
}

// B5c-3: 移動元のほかのワークスペース(Mobile)のアイテムとの関連は両方の側から消え、ワークスペースの中の関連は残る。
func TestMoveWorkspaceCutsOutsideRelations(t *testing.T) {
	f := newFixture(t)
	addRelation(t, f.rec, mobileEpic, apptest.SampleTask, "relates")
	addRelation(t, f.rec, apptest.SampleTask, mobileEpic, "blocks")
	addRelation(t, f.rec, apptest.SampleTask, apptest.SampleEpic, "relates")
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); err != nil {
		t.Fatal(err)
	}
	if rel := itemOf(t, f.rec, store2, apptest.SampleTask).Relations; len(rel) != 1 || rel[0] != (item.Relation{Type: "relates", To: apptest.SampleEpic}) {
		t.Errorf("moved task relations = %+v", rel)
	}
	if rel := itemOf(t, f.rec, store1, mobileEpic).Relations; len(rel) != 0 {
		t.Errorf("outside epic relations = %+v", rel)
	}
}

// B5c-7: 移動先のメンバーに Taro がいるので、担当者・ウォッチ・ルールの担当者の値・プロジェクトのオーナーの taro は Taro に
// 書き直され、taro は join されない。
func TestMoveWorkspaceRespellsPeople(t *testing.T) {
	f := newFixture(t)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", nil, "taro"))
	})
	rule := automation.Rule{ID: "01TESTRULE0000000000000001", Name: "担当", Trigger: automation.TriggerPRMerged, Action: automation.ActionAssignee, Value: "taro", Enabled: true}
	addRule(t, f.rec, apptest.SampleWorkspace, rule)
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); err != nil {
		t.Fatal(err)
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro"}) {
		t.Errorf("destination members = %v", got)
	}
	if pr, _ := projectAt(t, f.rec, store2, apptest.SampleProject); pr.Owner != "Taro" {
		t.Errorf("owner = %q", pr.Owner)
	}
	task := itemOf(t, f.rec, store2, apptest.SampleTask)
	if task.Assignee != "Taro" || !slices.Equal(task.Watchers, []string{"Taro"}) {
		t.Errorf("task = %q / %v", task.Assignee, task.Watchers)
	}
	rules := automation.FromLog(logOf(t, f.rec, store2, change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace}))
	if len(rules) != 1 || rules[0].Value != "Taro" {
		t.Errorf("rules = %+v", rules)
	}
}

// Review Focus 1: fn が呼び直されたら、断りと rank は呼び直しの tx から決める。
func TestMoveWorkspaceReadsTxOnRetry(t *testing.T) {
	t.Run("refusal", func(t *testing.T) {
		f := newFixture(t)
		f.svc.Delivery = retried{inner: f.deliver, stores: f.rec, done: new(bool), between: func(tx port.Tx) error {
			d, err := workspace.CreateWithSettings(workspace.Workspace{ID: otherWS, Name: "Other", Prefix: "HH"}, workspace.DefaultSettings(f.svc.IDs.New))
			if err != nil {
				return err
			}
			return tx.WriteWorkspaces().Append(otherWS, d)
		}}
		_, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2)
		wantRefusal(t, err, "team: この Store には同じ接頭辞のワークスペースがあります")
		assertWorkspaceUntouched(t, f)
	})
	t.Run("rank", func(t *testing.T) {
		f := newFixture(t)
		f.svc.Delivery = retried{inner: f.deliver, stores: f.rec, done: new(bool), between: func(tx port.Tx) error {
			d, err := workspace.CreateWithRank(workspace.Workspace{ID: otherWS, Name: "Other", Prefix: "OT"}, workspace.DefaultSettings(f.svc.IDs.New), "X")
			if err != nil {
				return err
			}
			return tx.WriteWorkspaces().Append(otherWS, d)
		}}
		if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); err != nil {
			t.Fatal(err)
		}
		if r := workspace.RankOf(logOf(t, f.rec, store2, change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})); r != "Y" {
			t.Errorf("rank = %q, want Y (after the workspace added between the calls)", r)
		}
	})
}

// Review Focus 1: 足す人と綴りの対応も呼び直しの tx から決める。1 回目の呼び出しでは移動先に jiro がいないので足すが、
// あいだに Jiro が足されたら、2 回目は jiro を足さずに担当者を Jiro へ書き直す(fixture の team には Taro が初めからいるので、
// jiro で確かめる)。
func TestMoveWorkspaceReadsTxOnRetryForPeople(t *testing.T) {
	f := newFixture(t)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", nil, "jiro"))
	})
	f.svc.Delivery = retried{inner: f.deliver, stores: f.rec, done: new(bool), between: func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Join("Jiro"))
	}}
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); err != nil {
		t.Fatal(err)
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Jiro", "Taro"}) {
		t.Errorf("destination members = %v", got)
	}
	if a := itemOf(t, f.rec, store2, apptest.SampleTask).Assignee; a != "Jiro" {
		t.Errorf("assignee = %q, want Jiro", a)
	}
}

// 送れない → *SendError で、移動先に何も残らず、移動元もそのまま。
func TestMoveWorkspaceSendFails(t *testing.T) {
	f := newFixture(t)
	f.deliver.reasons = map[port.StoreID]syncstatus.Reason{store2: syncstatus.WriteDenied}
	_, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2)
	var se *transfers.SendError
	if !errors.As(err, &se) || se.Store != "team" {
		t.Fatalf("err = %v", err)
	}
	assertWorkspaceUntouched(t, f)
}

// assertWorkspaceUntouched は、移動元に Web とその中身が元のままあり、移動先にはなく、転送記録がないことを確かめる。
func assertWorkspaceUntouched(t *testing.T, f fixture) {
	t.Helper()
	if w, ok := webAt(t, f.rec, store1); !ok || w.Deleted != nil {
		t.Errorf("source workspace = %+v (%v)", w, ok)
	}
	if _, ok := webAt(t, f.rec, store2); ok {
		t.Error("the workspace was saved in the destination")
	}
	if _, ok := projectAt(t, f.rec, store2, apptest.SampleProject); ok {
		t.Error("the project was saved in the destination")
	}
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if !exists(t, f.rec, store1, id) || exists(t, f.rec, store2, id) {
			t.Errorf("%s moved", id)
		}
	}
	if len(recordsOf(t, f.rec, store1)) != 0 {
		t.Error("transfer records were written")
	}
}

// B5c-10: 別の画面で削除されたワークスペースは、移動先で削除が取り消される(ワークスペースと、その削除で一緒に削除された
// プロジェクトとアイテム)。前に別に削除した Task は削除済みのまま。名前は両方の保存のあとで積まれる。
func TestMoveWorkspaceRevives(t *testing.T) {
	f := newFixture(t)
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDeleteEntity(true)),
			tx.WriteProjects().Append(apptest.SampleProject, change.NewDeleteEntity(false)),
			tx.WriteItems().Append(apptest.SampleEpic, change.NewDeleteEntity(false)),
		)
	})
	failed := port.WithRevived(t.Context())
	f.deliver.reasons = map[port.StoreID]syncstatus.Reason{store2: syncstatus.WriteDenied}
	if _, err := f.svc.MoveWorkspace(failed, store1, apptest.SampleWorkspace, store2); err == nil {
		t.Fatal("moved although the send failed")
	}
	if got := port.Revived(failed); len(got) != 0 {
		t.Errorf("revived before saving = %+v", got)
	}
	f.deliver.reasons = nil
	ctx := port.WithRevived(t.Context())
	if _, err := f.svc.MoveWorkspace(ctx, store1, apptest.SampleWorkspace, store2); err != nil {
		t.Fatal(err)
	}
	if w, ok := webAt(t, f.rec, store2); !ok || w.Deleted != nil {
		t.Errorf("workspace = %+v (%v)", w, ok)
	}
	if p, ok := projectAt(t, f.rec, store2, apptest.SampleProject); !ok || p.Deleted != nil {
		t.Errorf("project = %+v (%v)", p, ok)
	}
	if it := itemOf(t, f.rec, store2, apptest.SampleEpic); it.Deleted != nil {
		t.Errorf("epic = %+v", it.Deleted)
	}
	if it := itemOf(t, f.rec, store2, apptest.SampleTask); it.Deleted == nil {
		t.Error("the task deleted separately was revived")
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "Web"}}; !slices.Equal(port.Revived(ctx), want) {
		t.Errorf("revived = %+v, want %+v", port.Revived(ctx), want)
	}
}

// 移動元が閲覧のみなら断り、移動先が閲覧のみか自分なら port.ErrNotFound、表示中でなければ port.ErrStoreNotFound。
// 移動先に同じ接頭辞があれば、移動先の保存の前に断る。
func TestMoveWorkspaceRefuses(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.MoveWorkspace(t.Context(), store3, archiveWS, store2); err == nil {
		t.Error("moved from a read-only store")
	} else {
		wantRefusal(t, err, "閲覧のみの Store です")
	}
	for name, to := range map[string]port.StoreID{"read only": store3, "self": store1} {
		if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, to); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, "S9"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("hidden: err = %v", err)
	}
	addWorkspace(t, f.rec, store2, workspace.Workspace{ID: otherWS, Name: "Web", Prefix: "XX"}, "", "", nil)
	_, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2)
	wantRefusal(t, err, "team: この Store には同じ名前のワークスペースがあります")
	if got := f.rec.summaries(store2); slices.Contains(got, "ワークスペース「Web」を main から移動") {
		t.Error("the destination was written")
	}
	assertWorkspaceUntouched(t, f)
}

// B5c-6: 説明はそのまま移り、前に同じ Store の別のワークスペース(Mobile)から移したアイテムの moved_in は、その移動(旧 ID
// MB-1)と、Mobile の値の名前を持つ。
func TestMoveWorkspaceCarriesDescriptionsAndEarlierMoves(t *testing.T) {
	f := newFixture(t)
	todo := valueID(t, f.rec, "status", "Todo")
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(mobileEpic, change.NewSet("status", nil, todo)),
			tx.WriteProjects().Describe(apptest.SampleProject, "プロジェクトの説明"),
			tx.WriteItems().Describe(apptest.SampleEpic, "Epic の説明"),
		)
	})
	in := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject}
	if _, err := f.svc.Move(t.Context(), store1, mobileEpic, in); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.MoveWorkspace(t.Context(), store1, apptest.SampleWorkspace, store2); err != nil {
		t.Fatal(err)
	}
	var pd, id string
	if err := f.rec.Read(t.Context(), store2, func(r port.Reader) (err error) {
		if pd, err = r.Projects().Description(apptest.SampleProject); err != nil {
			return err
		}
		id, err = r.Items().Description(apptest.SampleEpic)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if pd != "プロジェクトの説明" || id != "Epic の説明" {
		t.Errorf("descriptions = %q / %q", pd, id)
	}
	mi := transfer.MovedInOf(logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: mobileEpic}))
	if len(mi) == 0 {
		t.Fatal("no moved_in")
	}
	last := mi[len(mi)-1]
	if len(last.Moves) != 1 || last.Moves[0].Old != "MB-1" || last.Moves[0].FromWorkspaceName != "Mobile" {
		t.Errorf("moves = %+v", last.Moves)
	}
	if last.Names[todo] != "Todo" {
		t.Errorf("names = %+v, want %s: Todo", last.Names, todo)
	}
}
