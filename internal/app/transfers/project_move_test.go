package transfers_test

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const qaStatus change.ID = "VALMBQA"

// projectOf は、main(S1)のプロジェクト(削除済みを含む)。
func projectOf(t *testing.T, s port.Stores, id project.ID) project.Project {
	t.Helper()
	var p project.Project
	if err := s.Read(t.Context(), store1, func(r port.Reader) (err error) { p, err = r.Projects().Get(id); return }); err != nil {
		t.Fatal(err)
	}
	return p
}

// addProject は、main(S1)のワークスペース ws に、タイトル title・番号 n のプロジェクトを作る(番号の行も書く)。
func addProject(t *testing.T, s port.Stores, ws workspace.ID, id project.ID, n int, title string) {
	t.Helper()
	write(t, s, store1, func(tx port.Tx) error {
		p := project.Project{ID: id, Place: project.Place{Workspace: ws, Number: n}, Rank: "X", Title: title, Status: project.Todo}
		return errors.Join(
			tx.WriteWorkspaces().Append(ws, numbering.Draft(numbering.Projects, n)),
			tx.WriteProjects().Append(id, project.Create(p)),
		)
	})
}

// addRule は、main(S1)のワークスペース ws に自動化のルール r を作る(r.Scope があれば専用ルール)。
func addRule(t *testing.T, s port.Stores, ws workspace.ID, r automation.Rule) {
	t.Helper()
	write(t, s, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(ws, automation.Drafts(automation.Rule{}, r)...)
	})
}

// statusRule は、PR がマージされたらステータスを status にする、プロジェクト p の専用ルール。
func statusRule(id change.ID, name string, p project.ID, status change.ID) automation.Rule {
	return automation.Rule{ID: id, Name: name, Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: string(status), Enabled: true, Scope: p}
}

// rulesOf は、main(S1)のワークスペース ws の、プロジェクト p のプロジェクト別のルールと専用ルール。
func rulesOf(t *testing.T, s port.Stores, ws workspace.ID, p project.ID) (projectrule.Rules, []automation.Rule) {
	t.Helper()
	log := logOf(t, s, store1, change.Ref{Kind: change.KindWorkspace, ID: ws})
	own := slices.DeleteFunc(automation.FromLog(log), func(r automation.Rule) bool { return r.Scope != p })
	return projectrule.FromLog(log).Of(p), own
}

// numberRows は、main(S1)のワークスペース ws のログの最後の操作の、番号の行の数(種類ごと)。
func numberRows(t *testing.T, s port.Stores, ws workspace.ID) map[string]int {
	t.Helper()
	log := logOf(t, s, store1, change.Ref{Kind: change.KindWorkspace, ID: ws})
	out := map[string]int{}
	for _, r := range log {
		if r.Op == log[len(log)-1].Op && r.T == numbering.Type {
			out[r.F]++
		}
	}
	return out
}

func moveProject(t *testing.T, f fixture, p project.ID, in transfers.ProjectMoveInput) transfers.ProjectMoved {
	t.Helper()
	got, err := f.svc.MoveProject(t.Context(), store1, p, in)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// 02 **移動**・A5b-11: 同じ Store の中は 1 回の保存。プロジェクトは移動先の次の番号と末尾の順位、アイテムはワークスペースと
// 番号だけが変わり、移動先に番号の行、転送記録はプロジェクトとアイテムごと。
func TestMoveProjectWithinStoreWritesOneSave(t *testing.T) {
	f := newFixture(t)
	before := len(f.rec.summaries())
	got := moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS))
	if want := (transfers.ProjectMoved{ToStore: store1, Project: apptest.SampleProject, Display: "MB-P2", Destination: "Mobile"}); got != want {
		t.Errorf("moved = %+v, want %+v", got, want)
	}
	if s := f.rec.summaries()[before:]; !slices.Equal(s, []string{"プロジェクト「ログイン」を Web から Mobile へ移動"}) {
		t.Errorf("summaries = %q", s)
	}
	if p := projectOf(t, f.rec, apptest.SampleProject); p.Place != (project.Place{Workspace: mobileWS, Number: 2}) || p.Rank != rank.After("V") {
		t.Errorf("project = %+v", p)
	}
	epic, task := itemOf(t, f.rec, store1, apptest.SampleEpic), itemOf(t, f.rec, store1, apptest.SampleTask)
	if epic.Place != (item.Place{Workspace: mobileWS, Project: apptest.SampleProject, Number: 2}) || epic.Rank != rank.First {
		t.Errorf("epic = %+v", epic)
	}
	if task.Place.Workspace != mobileWS || task.Place.Number != 3 || *task.Place.Parent != apptest.SampleEpic || task.Rank != rank.First {
		t.Errorf("task = %+v", task)
	}
	if rows := numberRows(t, f.rec, mobileWS); !maps.Equal(rows, map[string]int{"projects": 1, "items": 2}) {
		t.Errorf("number rows = %v", rows)
	}
	want := []transfer.Record{
		{Entity: transfer.EntityProject, Target: apptest.SampleProject, Old: "HH-P1", New: "MB-P2", ToStore: string(store1)},
		{Entity: transfer.EntityItem, Target: apptest.SampleEpic, Old: "HH-1", New: "MB-2", ToStore: string(store1)},
		{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "MB-3", ToStore: string(store1)},
	}
	if recs := recordsOf(t, f.rec, store1); !slices.EqualFunc(recs, want, func(a, b transfer.Record) bool {
		return a.Entity == b.Entity && a.Target == b.Target && a.Old == b.Old && a.New == b.New && a.ToStore == b.ToStore
	}) {
		t.Errorf("records = %+v", recs)
	}
}

// remapFixture は、Mobile に Blocked・Review・QA(進行中)を足し、優先度 Low を Minor にしたフィクスチャ。
func remapFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureWith(t, func(st *workspace.Settings) {
		withReviewAndBlocked(st)
		last := st.Statuses[len(st.Statuses)-1].Rank
		st.Statuses = append(st.Statuses, workspace.Value{ID: qaStatus, Name: "QA", Color: "blue", Category: workspace.CategoryDoing, Rank: last + "V"})
		for i := range st.Priorities {
			if st.Priorities[i].Name == "Low" {
				st.Priorities[i].Name = "Minor"
			}
		}
	})
}

// 02「値の引き直し」・A5b-5: 優先度の名前が移動先にないものは初期優先度。ステータスは StatusChoices の値。ただし、一緒に移る
// 使わないステータス(Pending)や移動先にないものを選んだときは、アイテムも専用ルールも同じ分類の既定にする。プロジェクトが
// 使わない Pending をアイテムが持っていても、名前が移動先にあれば置き換えない。
func TestMoveProjectRemapsValues(t *testing.T) {
	f := remapFixture(t)
	pending := hideStatus(t, f.rec, store1, mobileWS, mobileProject, "Pending")
	review := addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000010", Place: item.Place{Workspace: mobileWS, Project: mobileProject, Parent: ptr(mobileEpic), Number: 2}, Kind: item.Task, Rank: "V", Title: "t", Status: reviewStatus, Priority: valueID(t, f.rec, "priority", "Minor")})
	blocked := mobileItem(t, f, "01TESTIT000000000000000011", 3, blockedStatus)
	qa := mobileItem(t, f, "01TESTIT000000000000000012", 4, qaStatus)
	kept := mobileItem(t, f, "01TESTIT000000000000000013", 5, pending)
	addRule(t, f.rec, mobileWS, statusRule("01TESTRULE0000000000000001", "Blocked にする", mobileProject, blockedStatus))
	addRule(t, f.rec, mobileWS, statusRule("01TESTRULE0000000000000002", "QA にする", mobileProject, qaStatus))
	in := projectTo(store1, apptest.SampleWorkspace)
	in.StatusChoices = map[change.ID]change.ID{reviewStatus: apptest.SampleStatusDone, blockedStatus: apptest.SampleStatusPending, qaStatus: "01NOSUCHSTATUS"}
	moveProject(t, f, mobileProject, in)

	for _, c := range []struct {
		it   item.Item
		want change.ID
	}{{review, apptest.SampleStatusDone}, {blocked, apptest.SampleStatusTodo}, {qa, apptest.SampleStatusInProgress}, {kept, apptest.SampleStatusPending}} {
		if got := itemOf(t, f.rec, store1, c.it.ID); got.Status != c.want {
			t.Errorf("%s status = %s, want %s", c.it.ID, got.Status, c.want)
		}
	}
	if got := itemOf(t, f.rec, store1, review.ID); got.Priority != apptest.SamplePriorityMedium {
		t.Errorf("priority = %s, want the initial priority", got.Priority)
	}
	_, own := rulesOf(t, f.rec, apptest.SampleWorkspace, mobileProject)
	values := map[string]string{}
	for _, r := range own {
		values[r.Name] = r.Value
	}
	if want := map[string]string{"Blocked にする": string(apptest.SampleStatusTodo), "QA にする": string(apptest.SampleStatusInProgress)}; !maps.Equal(values, want) {
		t.Errorf("own rules = %v, want %v", values, want)
	}
}

// 03「移動・削除・復元」・A5b-7: 移動元のワークスペースからプロジェクト別のルールと専用ルールが消え、移動先には合うものだけが
// ある(Blocked は Web にない)。専用ルールは新しい内部 ID で作る。
func TestMoveProjectCarriesRules(t *testing.T) {
	f := newFixtureWith(t, withReviewAndBlocked)
	hideStatus(t, f.rec, store1, mobileWS, mobileProject, "Pending")
	hideStatus(t, f.rec, store1, mobileWS, mobileProject, "Blocked")
	old := statusRule("01TESTRULE0000000000000001", "マージで変える", mobileProject, valueID(t, f.rec, "status", "Done"))
	addRule(t, f.rec, mobileWS, old)
	moveProject(t, f, mobileProject, projectTo(store1, apptest.SampleWorkspace))

	if rules, own := rulesOf(t, f.rec, mobileWS, mobileProject); !rules.Empty() || len(own) != 0 {
		t.Errorf("source still has rules %+v / %+v", rules, own)
	}
	rules, own := rulesOf(t, f.rec, apptest.SampleWorkspace, mobileProject)
	if !slices.Equal(rules.Hidden, []change.ID{apptest.SampleStatusPending}) || len(rules.Workflow)+len(rules.Templates)+len(rules.TemplateDefaults)+len(rules.Disabled) != 0 {
		t.Errorf("destination rules = %+v", rules)
	}
	if len(own) != 1 || own[0].ID == old.ID || own[0].Name != old.Name || own[0].Value != string(apptest.SampleStatusDone) {
		t.Errorf("destination own rules = %+v", own)
	}
}

func wantRefusal(t *testing.T, err error, msg string) {
	t.Helper()
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != msg {
		t.Errorf("err = %v, want %q", err, msg)
	}
}

// A5b-9: 移動先に同じ名前の削除されていないプロジェクトがあれば断る(何も保存しない)。削除済みの同じ名前は断らない。
func TestMoveProjectRefusesSameNameProject(t *testing.T) {
	f := newFixture(t)
	const twin project.ID = "01TESTPR000000000000000005"
	addProject(t, f.rec, mobileWS, twin, 2, "ログイン")
	before := len(f.rec.summaries())
	_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, projectTo(store1, mobileWS))
	wantRefusal(t, err, "Mobile には同じ名前のプロジェクトがあります")
	if len(f.rec.summaries()) != before {
		t.Error("refused move saved something")
	}
	if err := apptest.DeleteProject(t.Context(), f.rec, store1, twin); err != nil {
		t.Fatal(err)
	}
	if got := moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS)); got.Display != "MB-P3" {
		t.Errorf("display = %s, want MB-P3", got.Display)
	}
}

// A5b-9: 移動先に専用ルールと同じ名前の削除されていないルールがあれば断る。削除済みのルールは断らない。
func TestMoveProjectRefusesSameNameRule(t *testing.T) {
	f := newFixture(t)
	addRule(t, f.rec, apptest.SampleWorkspace, statusRule("01TESTRULE0000000000000001", "レビュー待ち", apptest.SampleProject, apptest.SampleStatusDone))
	other := statusRule("01TESTRULE0000000000000002", "レビュー待ち", "", valueID(t, f.rec, "status", "Done"))
	addRule(t, f.rec, mobileWS, other)
	_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, projectTo(store1, mobileWS))
	wantRefusal(t, err, "Mobile には同じ名前のルールがあります: レビュー待ち。先にルールの名前を変えてください")
	write(t, f.rec, store1, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(mobileWS, automation.DeleteDraft(other.ID)) })
	moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS))
}

// A5b-9 の順: 移動先にプロジェクトとルールの両方で同じ名前があれば、プロジェクトの断りを出す。
func TestMoveProjectRefusesSameNameProjectBeforeRule(t *testing.T) {
	f := newFixture(t)
	addProject(t, f.rec, mobileWS, "01TESTPR000000000000000005", 2, "ログイン")
	addRule(t, f.rec, apptest.SampleWorkspace, statusRule("01TESTRULE0000000000000001", "レビュー待ち", apptest.SampleProject, apptest.SampleStatusDone))
	addRule(t, f.rec, mobileWS, statusRule("01TESTRULE0000000000000002", "レビュー待ち", "", valueID(t, f.rec, "status", "Done")))
	_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, projectTo(store1, mobileWS))
	wantRefusal(t, err, "Mobile には同じ名前のプロジェクトがあります")
}

// Review Focus 4: アイテムが 0 件のプロジェクトも移せる。番号の行はプロジェクトの分だけ。
func TestMoveProjectWithoutItems(t *testing.T) {
	f := newFixture(t)
	const empty project.ID = "01TESTPR000000000000000006"
	addProject(t, f.rec, apptest.SampleWorkspace, empty, 2, "空")
	if got := moveProject(t, f, empty, projectTo(store1, mobileWS)); got.Display != "MB-P2" {
		t.Errorf("display = %s", got.Display)
	}
	if rows := numberRows(t, f.rec, mobileWS); !maps.Equal(rows, map[string]int{"projects": 1}) {
		t.Errorf("number rows = %v", rows)
	}
	if p := projectOf(t, f.rec, empty); p.Place.Workspace != mobileWS {
		t.Errorf("project = %+v", p)
	}
}

// A5b-15: 別の画面で削除したプロジェクト(root: true)を移すと、プロジェクトと、同じ操作で削除された下のアイテムの削除を
// 取り消して移す。前に別の操作で削除したアイテムは削除済みのまま。取り消したのは移動前の ID。番号は、取り消した Epic を
// 削除されていないものとして振る(別の Store の移動と同じ)。
func TestMoveProjectRevivesDeletedElsewhere(t *testing.T) {
	f := newFixture(t)
	deleteSampleProject(t, f)
	ctx := port.WithRevived(t.Context())
	if _, err := f.svc.MoveProject(ctx, store1, apptest.SampleProject, projectTo(store1, mobileWS)); err != nil {
		t.Fatal(err)
	}
	if p := projectOf(t, f.rec, apptest.SampleProject); p.Deleted != nil || p.Place.Workspace != mobileWS {
		t.Errorf("project = %+v", p)
	}
	if e := itemOf(t, f.rec, store1, apptest.SampleEpic); e.Deleted != nil || e.Place.Workspace != mobileWS {
		t.Errorf("epic = %+v", e)
	}
	if task := itemOf(t, f.rec, store1, apptest.SampleTask); task.Deleted == nil {
		t.Error("the task deleted by another operation was revived")
	}
	if got := port.Revived(ctx); !slices.Equal(got, []string{"HH-P1"}) {
		t.Errorf("revived = %v", got)
	}
	for id, n := range map[item.ID]int{apptest.SampleEpic: 2, apptest.SampleTask: 3, earlierEpic: 4} {
		if got := itemOf(t, f.rec, store1, id).Place.Number; got != n {
			t.Errorf("%s number = %d, want %d", id, got, n)
		}
	}
}

// 決定 S11: 別の画面で削除したプロジェクトの名前は、移動先で判定する(移動元に同じ名前の新しいプロジェクトがあっても移す)。
// 移動先に同じ名前があれば、移動先の名前で断る(別の Store の移動と同じ)。
func TestMoveProjectRevivesWithSourceSameName(t *testing.T) {
	f := newFixture(t)
	deleteSampleProject(t, f)
	addProject(t, f.rec, apptest.SampleWorkspace, "01TESTPR000000000000000007", 3, "ログイン")
	addProject(t, f.rec, mobileWS, "01TESTPR000000000000000008", 2, "ログイン")
	_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, projectTo(store1, mobileWS))
	wantRefusal(t, err, "Mobile には同じ名前のプロジェクトがあります")
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteProjects().Append("01TESTPR000000000000000008", change.NewDeleteEntity(true))
	})
	moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS))
	if p := projectOf(t, f.rec, apptest.SampleProject); p.Deleted != nil || p.Place.Workspace != mobileWS {
		t.Errorf("project = %+v", p)
	}
}

// 移動元が閲覧のみなら断る。移動先が同じワークスペース・削除済みのワークスペース・閲覧のみの Store なら ErrNotFound、表示中でない
// Store なら ErrStoreNotFound。どれも何も保存しない。
func TestMoveProjectRefusals(t *testing.T) {
	f := newFixture(t)
	const oldWS workspace.ID = "01TESTWS000000000000000005"
	addWorkspace(t, f.rec, store1, workspace.Workspace{ID: oldWS, Name: "Old", Prefix: "OL"}, "", "", nil)
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store1, oldWS); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		in   transfers.ProjectMoveInput
		want error
	}{
		{"同じワークスペース", projectTo(store1, apptest.SampleWorkspace), port.ErrNotFound},
		{"削除済みのワークスペース", projectTo(store1, oldWS), port.ErrNotFound},
		{"ないワークスペース", projectTo(store1, "01TESTWS000000000000000099"), port.ErrNotFound},
		{"閲覧のみの Store", projectTo(store3, archiveWS), port.ErrNotFound},
		{"表示中でない Store", projectTo("S9", mobileWS), port.ErrStoreNotFound},
	}
	before := len(f.rec.summaries())
	for _, c := range cases {
		if _, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	f.list.Stores[0].ReadOnly = true
	_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, projectTo(store1, mobileWS))
	wantRefusal(t, err, "閲覧のみの Store です")
	if len(f.rec.summaries()) != before {
		t.Error("refused moves saved something")
	}
}

// addTemplate は、main(S1)のワークスペース ws にテンプレート「バグ報告」(Bug)を作る。
func addTemplate(t *testing.T, s port.Stores, ws workspace.ID, id change.ID) {
	t.Helper()
	tpl := workspace.Template{ID: id, Name: "バグ報告", Kind: "Bug", Title: "[bug] "}
	write(t, s, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(ws, workspace.TemplateDrafts(workspace.Template{}, tpl)...)
	})
}

// 03「移動・削除・復元」: Web → Mobile → Web と移すと、使わないステータス・遷移のチェック・使うテンプレートが Web に戻り
// (前の移動の remove のあとの add が効く)、専用ルールも 3 つ目の新しい内部 ID で戻り、削除済みと読まれない。
func TestMoveProjectRoundTripCarriesRulesBack(t *testing.T) {
	f := newFixture(t)
	const webTpl, mobileTpl change.ID = "01TESTTPL00000000000000001", "01TESTTPL00000000000000002"
	addTemplate(t, f.rec, apptest.SampleWorkspace, webTpl)
	addTemplate(t, f.rec, mobileWS, mobileTpl)
	p := apptest.SampleProject
	hideStatus(t, f.rec, store1, apptest.SampleWorkspace, p, "Pending")
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
			change.NewAdd(projectrule.WorkflowField(p, apptest.SampleStatusTodo), apptest.SampleStatusDone),
			change.NewAdd(projectrule.TemplatesField(p), webTpl))
	})
	first := statusRule("01TESTRULE0000000000000001", "マージで完了", p, apptest.SampleStatusDone)
	addRule(t, f.rec, apptest.SampleWorkspace, first)

	moveProject(t, f, p, projectTo(store1, mobileWS))
	_, middle := rulesOf(t, f.rec, mobileWS, p)
	if len(middle) != 1 {
		t.Fatalf("own rules in Mobile = %+v", middle)
	}
	moveProject(t, f, p, projectTo(store1, apptest.SampleWorkspace))

	rules, own := rulesOf(t, f.rec, apptest.SampleWorkspace, p)
	if !slices.Equal(rules.Hidden, []change.ID{apptest.SampleStatusPending}) ||
		!slices.Equal(rules.Workflow[apptest.SampleStatusTodo], []change.ID{apptest.SampleStatusDone}) || len(rules.Workflow) != 1 ||
		!slices.Equal(rules.Templates, []change.ID{webTpl}) {
		t.Errorf("rules back in Web = %+v", rules)
	}
	if len(own) != 1 || own[0].ID == first.ID || own[0].ID == middle[0].ID || own[0].Name != first.Name || own[0].Value != string(apptest.SampleStatusDone) {
		t.Errorf("own rules back in Web = %+v (first %s, middle %s)", own, first.ID, middle[0].ID)
	}
	if rules, own := rulesOf(t, f.rec, mobileWS, p); !rules.Empty() || len(own) != 0 {
		t.Errorf("Mobile still has rules %+v / %+v", rules, own)
	}
}

// Review Focus 4: 削除済みの Epic とその下は、削除済みのまま元の削除の操作のまま移り、番号は削除されていない Epic の下のあと
// (削除済みの Epic の順位が前でも)。
func TestMoveProjectKeepsDeletedItemsDeleted(t *testing.T) {
	f := newFixture(t)
	const goneEpic, goneTask item.ID = "01TESTIT000000000000000020", "01TESTIT000000000000000021"
	addItem(t, f.rec, store1, item.Item{ID: goneEpic, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3}, Kind: item.Epic, Rank: "A", Title: "e"})
	addItem(t, f.rec, store1, item.Item{ID: goneTask, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(goneEpic), Number: 4}, Kind: item.Task, Rank: "V", Title: "t"})
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, goneEpic, goneTask); err != nil {
		t.Fatal(err)
	}
	deleted := map[item.ID]change.Deletion{}
	for _, id := range []item.ID{goneEpic, goneTask} {
		deleted[id] = *itemOf(t, f.rec, store1, id).Deleted
	}
	moveProject(t, f, apptest.SampleProject, projectTo(store1, mobileWS))
	for id, n := range map[item.ID]int{apptest.SampleEpic: 2, apptest.SampleTask: 3, goneEpic: 4, goneTask: 5} {
		it := itemOf(t, f.rec, store1, id)
		if it.Place.Workspace != mobileWS || it.Place.Number != n {
			t.Errorf("%s place = %+v, want number %d", id, it.Place, n)
		}
		if d, ok := deleted[id]; ok && (it.Deleted == nil || it.Deleted.Op != d.Op || it.Deleted.Root != d.Root) {
			t.Errorf("%s deleted = %+v, want %+v", id, it.Deleted, d)
		}
	}
}
