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
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const projectSummary = "プロジェクト「ログイン」を main の Web から team の Team へ移動"

func toTeamWS() transfers.ProjectMoveInput { return projectTo(store2, teamWS) }

// projectAt は、Store store のプロジェクト id(削除済みを含む)。なければ ok=false。
func projectAt(t *testing.T, s port.Stores, store port.StoreID, id project.ID) (project.Project, bool) {
	t.Helper()
	var p project.Project
	err := s.Read(t.Context(), store, func(r port.Reader) (err error) { p, err = r.Projects().Get(id); return })
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		t.Fatal(err)
	}
	return p, err == nil
}

// rulesAt は、Store store のワークスペース ws の、プロジェクト「ログイン」のプロジェクト別のルールと専用ルール。
func rulesAt(t *testing.T, s port.Stores, store port.StoreID, ws workspace.ID) (projectrule.Rules, []automation.Rule) {
	t.Helper()
	log := logOf(t, s, store, change.Ref{Kind: change.KindWorkspace, ID: ws})
	own := slices.DeleteFunc(automation.FromLog(log), func(r automation.Rule) bool { return r.Scope != apptest.SampleProject })
	return projectrule.FromLog(log).Of(apptest.SampleProject), own
}

// writeAs は、login の操作として main(S1)に 1 回保存する。
func writeAs(t *testing.T, s port.Stores, login string, fn func(tx port.Tx) error) {
	t.Helper()
	if err := s.Write(t.Context(), store1, port.Operation{Actor: change.User(login), Summary: "テスト"}, fn); err != nil {
		t.Fatal(err)
	}
}

// withRules は、Web のプロジェクト「ログイン」に、使わないステータス Pending、Bug の既定のテンプレート「なし」、
// 専用ルール「マージで完了」を足す。
func withRules(t *testing.T, f fixture) automation.Rule {
	t.Helper()
	hideStatus(t, f.rec, store1, apptest.SampleWorkspace, apptest.SampleProject, "Pending")
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
			change.NewSet(projectrule.TemplateDefaultField(apptest.SampleProject, "Bug"), nil, projectrule.None))
	})
	r := statusRule("01TESTRULE0000000000000001", "マージで完了", apptest.SampleProject, apptest.SampleStatusDone)
	addRule(t, f.rec, apptest.SampleWorkspace, r)
	return r
}

// 02 **移動**・A5b-12: 移動先にプロジェクトとアイテムのフォルダ(行はそのまま + moved_in・set place)、番号の行、ルールの行。
// 移動元はフォルダとルールの行が消え、転送記録がプロジェクトとアイテムごと(日時は moved_in と同じ)。オーナー taro は
// 移動先の綴り Taro になる。アクティビティは両方の Store で同じ文。
func TestMoveProjectAcrossWritesBothStores(t *testing.T) {
	f := newFixture(t)
	old := withRules(t, f)
	srcLog := logOf(t, f.rec, store1, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})
	got, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS())
	if err != nil {
		t.Fatal(err)
	}
	if want := (transfers.ProjectMoved{ToStore: store2, Project: apptest.SampleProject, Display: "TM-P2", Destination: "team の Team"}); got != want {
		t.Errorf("moved = %+v, want %+v", got, want)
	}
	at := assertProjectDestination(t, f, srcLog)
	rules, own := rulesAt(t, f.rec, store2, teamWS)
	if len(rules.Hidden) != 1 || rules.TemplateDefaults["Bug"] != projectrule.None || len(own) != 1 || own[0].ID == old.ID || own[0].Name != old.Name {
		t.Errorf("destination rules = %+v / %+v", rules, own)
	}
	if rules, own := rulesAt(t, f.rec, store1, apptest.SampleWorkspace); !rules.Empty() || len(own) != 0 {
		t.Errorf("source rules = %+v / %+v", rules, own)
	}
	if _, ok := projectAt(t, f.rec, store1, apptest.SampleProject); ok {
		t.Error("the project remains in the source")
	}
	assertProjectRecords(t, f, at)
	for _, s := range []port.StoreID{store1, store2} {
		if got := f.rec.summaries(s); got[len(got)-1] != projectSummary {
			t.Errorf("%s summary = %q", s, got[len(got)-1])
		}
	}
}

// assertProjectDestination は、移動先に、プロジェクト(行は移動元の srcLog がそのまま先頭、moved_in、番号 TM-P2、オーナー Taro)と
// アイテム(TM-2・TM-3、moved_in)があり、移動元からアイテムが消え、移動先の次の番号が進んだことを確かめ、moved_in の日時を返す。
func assertProjectDestination(t *testing.T, f fixture, srcLog []change.Record) time.Time {
	t.Helper()
	p, ok := projectAt(t, f.rec, store2, apptest.SampleProject)
	if !ok || p.Place != (project.Place{Workspace: teamWS, Number: 2}) || p.Owner != "Taro" || p.Deleted != nil {
		t.Errorf("destination project = %+v (%v)", p, ok)
	}
	dstLog := logOf(t, f.rec, store2, change.Ref{Kind: change.KindProject, ID: apptest.SampleProject})
	if dstLog[0].ID != srcLog[0].ID || dstLog[len(srcLog)-1].ID != srcLog[len(srcLog)-1].ID {
		t.Error("the project log is not copied")
	}
	mi := transfer.MovedInOf(dstLog)
	if len(mi) != 1 || mi[0].Old != "HH-P1" || mi[0].FromStoreName != "main" || mi[0].FromWorkspaceName != "Web" ||
		mi[0].FromProject != apptest.SampleProject || len(mi[0].Names) != 0 {
		t.Fatalf("project moved_in = %+v", mi)
	}
	for id, n := range map[item.ID]int{apptest.SampleEpic: 2, apptest.SampleTask: 3} {
		it := itemOf(t, f.rec, store2, id)
		log := logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: id})
		if it.Place.Workspace != teamWS || it.Place.Project != apptest.SampleProject || it.Place.Number != n || len(transfer.MovedInOf(log)) != 1 {
			t.Errorf("%s = %+v", id, it.Place)
		}
		if exists(t, f.rec, store1, id) {
			t.Errorf("%s remains in the source", id)
		}
	}
	wsLog := logOf(t, f.rec, store2, change.Ref{Kind: change.KindWorkspace, ID: teamWS})
	if numbering.Next(wsLog, numbering.Projects) != 3 || numbering.Next(wsLog, numbering.Items) != 4 {
		t.Error("the destination numbers did not advance")
	}
	return mi[0].At
}

// assertProjectRecords は、移動元の転送記録が、プロジェクト・Epic・Task の 3 行で、移動先の Store 名と日時 at を持つことを確かめる。
func assertProjectRecords(t *testing.T, f fixture, at time.Time) {
	t.Helper()
	want := []transfer.Record{
		{Entity: transfer.EntityProject, Target: apptest.SampleProject, Old: "HH-P1", New: "TM-P2"},
		{Entity: transfer.EntityItem, Target: apptest.SampleEpic, Old: "HH-1", New: "TM-2"},
		{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "TM-3"},
	}
	recs := recordsOf(t, f.rec, store1)
	if !slices.EqualFunc(recs, want, func(a, b transfer.Record) bool {
		return a.Entity == b.Entity && a.Target == b.Target && a.Old == b.Old && a.New == b.New &&
			a.ToStore == string(store2) && a.ToStoreName == "team" && a.ToRemote == "" && a.At.Equal(at)
	}) {
		t.Errorf("records = %+v", recs)
	}
}

// A5b-3: 移動元のほかのプロジェクトのアイテム(Mobile の Epic)との関連は両方の側から消え、プロジェクトの中の関連は残る。
func TestMoveProjectAcrossCutsOutsideRelations(t *testing.T) {
	f := newFixture(t)
	addRelation(t, f.rec, mobileEpic, apptest.SampleTask, "relates")
	addRelation(t, f.rec, apptest.SampleTask, mobileEpic, "blocks")
	addRelation(t, f.rec, apptest.SampleTask, apptest.SampleEpic, "relates")
	if _, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS()); err != nil {
		t.Fatal(err)
	}
	if rel := itemOf(t, f.rec, store2, apptest.SampleTask).Relations; len(rel) != 1 || rel[0] != (item.Relation{Type: "relates", To: apptest.SampleEpic}) {
		t.Errorf("moved task relations = %+v", rel)
	}
	if rel := itemOf(t, f.rec, store1, mobileEpic).Relations; len(rel) != 0 {
		t.Errorf("outside epic relations = %+v", rel)
	}
}

// A5b-13: オーナー(goro)・専用ルールの担当者(jiro)・プロジェクトのコメントの書き手(saburo)・アイテムの担当者(shiro)の
// うち、移動先にいない人を足す。綴りが同じオーナーは set owner を書かない。
func TestMoveProjectAcrossAddsPeople(t *testing.T) {
	f := newFixture(t)
	p := apptest.SampleProject
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(p, change.NewSet("owner", "taro", "goro")),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", nil, "shiro")),
		)
	})
	writeAs(t, f.rec, "saburo", func(tx port.Tx) error {
		return tx.WriteProjects().Append(p, change.NewComment("01TESTCM000000000000000001", nil, comment.Value("見ました")))
	})
	addRule(t, f.rec, apptest.SampleWorkspace, automation.Rule{
		ID: "01TESTRULE0000000000000001", Name: "担当", Trigger: automation.TriggerPRMerged, Action: automation.ActionAssignee, Value: "jiro", Enabled: true, Scope: p,
	})
	if _, err := f.svc.MoveProject(t.Context(), store1, p, toTeamWS()); err != nil {
		t.Fatal(err)
	}
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro", "goro", "jiro", "saburo", "shiro"}) {
		t.Errorf("destination members = %v", got)
	}
	if pr, _ := projectAt(t, f.rec, store2, p); pr.Owner != "goro" {
		t.Errorf("owner = %q", pr.Owner)
	}
	log := logOf(t, f.rec, store2, change.Ref{Kind: change.KindProject, ID: p})
	if slices.ContainsFunc(log, func(r change.Record) bool { return r.F == "owner" && r.Op == log[len(log)-1].Op }) {
		t.Error("set owner was written although the spelling is the same")
	}
}

// Review Focus 1: fn が呼び直されたら、同じ名前の断りは呼び直しの tx から決める。あいだに移動先へ同じ名前のプロジェクトが
// できたら断り、どちらの Store も変えない。
func TestMoveProjectAcrossReadsTxOnRetryForRefusal(t *testing.T) {
	f := newFixture(t)
	const twin project.ID = "01TESTPR000000000000000005"
	f.svc.Delivery = retried{inner: f.deliver, stores: f.rec, done: new(bool), between: func(tx port.Tx) error {
		pr := project.Project{ID: twin, Place: project.Place{Workspace: teamWS, Number: 2}, Rank: "X", Title: "ログイン", Status: project.Todo}
		return tx.WriteProjects().Append(twin, project.Create(pr))
	}}
	_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS())
	wantRefusal(t, err, "team の Team には同じ名前のプロジェクトがあります")
	if _, ok := projectAt(t, f.rec, store2, apptest.SampleProject); ok {
		t.Error("the project was saved in the destination")
	}
	if _, ok := projectAt(t, f.rec, store1, apptest.SampleProject); !ok || len(recordsOf(t, f.rec, store1)) != 0 {
		t.Error("the source changed")
	}
}

// Review Focus 1: あいだに移動先の番号が進んだら、呼び直しの tx の番号で書く。
func TestMoveProjectAcrossReadsTxOnRetryForNumbers(t *testing.T) {
	f := newFixture(t)
	f.svc.Delivery = retried{inner: f.deliver, stores: f.rec, done: new(bool), between: func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(teamWS, numbering.Draft(numbering.Projects, 2), numbering.Draft(numbering.Items, 3))
	}}
	got, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS())
	if err != nil {
		t.Fatal(err)
	}
	if got.Display != "TM-P3" {
		t.Errorf("display = %s", got.Display)
	}
	if n := itemOf(t, f.rec, store2, apptest.SampleTask).Place.Number; n != 5 {
		t.Errorf("task number = %d, want 5", n)
	}
}

// 送れない → *SendError で、移動先に何も残らず、移動元もそのまま。
func TestMoveProjectAcrossSendFails(t *testing.T) {
	f := newFixture(t)
	withRules(t, f)
	f.deliver.reasons = map[port.StoreID]syncstatus.Reason{store2: syncstatus.WriteDenied}
	_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS())
	var se *transfers.SendError
	if !errors.As(err, &se) || se.Store != "team" {
		t.Fatalf("err = %v", err)
	}
	assertProjectUntouched(t, f)
}

// assertProjectUntouched は、移動元にプロジェクトとアイテムとルールが元のままあり、移動先にはないことを確かめる。
func assertProjectUntouched(t *testing.T, f fixture) {
	t.Helper()
	if p, ok := projectAt(t, f.rec, store1, apptest.SampleProject); !ok || p.Place.Workspace != apptest.SampleWorkspace {
		t.Errorf("source project = %+v (%v)", p, ok)
	}
	if _, ok := projectAt(t, f.rec, store2, apptest.SampleProject); ok {
		t.Error("the project remains in the destination")
	}
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if !exists(t, f.rec, store1, id) || exists(t, f.rec, store2, id) {
			t.Errorf("%s moved", id)
		}
	}
	if len(recordsOf(t, f.rec, store1)) != 0 {
		t.Error("transfer records were written")
	}
	if rules, own := rulesAt(t, f.rec, store1, apptest.SampleWorkspace); rules.Empty() || len(own) == 0 {
		t.Errorf("source rules = %+v / %+v", rules, own)
	}
}
