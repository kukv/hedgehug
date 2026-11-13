package planning_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	itemStory item.ID = "01ASSIGNIT0000000000000003"
	itemY     item.ID = "01ASSIGNIT0000000000000004"
	itemZ     item.ID = "01ASSIGNIT0000000000000005"
	itemStep  item.ID = "01ASSIGNIT0000000000000006"
)

// addItem は、Epic HH-1 の下に種類 kind のアイテムを 1 回の保存で足す。
func addItem(t *testing.T, svc planning.Service, id item.ID, number int, kind item.Kind, r string, sprint change.ID) {
	t.Helper()
	parent := apptest.SampleEpic
	it := item.Item{
		ID:    id,
		Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: number},
		Kind:  kind, Rank: r, Title: string(id), Status: apptest.SampleStatusTodo, Sprint: sprint,
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "アイテムを作成"}
	err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, number)); err != nil {
			return err
		}
		return tx.WriteItems().Append(id, item.Create(it))
	})
	if err != nil {
		t.Fatal(err)
	}
}

// lastOp は、ログの最後の保存の行の項目名。
func lastOp(log []change.Record) []string {
	var out []string
	op := log[len(log)-1].Op
	for _, r := range log {
		if r.Op == op {
			out = append(out, r.F)
		}
	}
	return out
}

func TestSetSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	addItem(t, svc, itemY, 4, item.Task, "Y", s1.ID)
	addItem(t, svc, itemZ, 5, item.Task, "Z", "")
	addItem(t, svc, itemStory, 3, item.Story, "M", "")

	n := len(rec.summaries)
	if err := svc.SetSprint(ctx, store, itemStory, &s1.ID); err != nil {
		t.Fatal(err)
	}
	if len(rec.summaries) != n+1 || lastSummary(rec) != "HH-3 の スプリント を Sprint 1 に変更" {
		t.Errorf("summaries = %v", rec.summaries[n:])
	}
	it, log := getItem(t, svc, itemStory)
	if it.Sprint != s1.ID || it.Rank != rank.After("Y") {
		t.Errorf("S1 へ: %s/%s", it.Sprint, it.Rank)
	}
	if got := lastOp(log); len(got) != 2 || got[0] != item.SprintField || got[1] != "rank" {
		t.Errorf("1 回の保存の行 = %v", got)
	}
	if sprintChanges(log) != 1 {
		t.Errorf("履歴のスプリントの変更 = %d", sprintChanges(log))
	}

	if err := svc.SetSprint(ctx, store, itemStory, nil); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-3 の スプリント を バックログ に変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	if it, _ := getItem(t, svc, itemStory); it.Sprint != "" || it.Rank != rank.After("Z") {
		t.Errorf("バックログへ: %s/%s", it.Sprint, it.Rank)
	}
}

// 移動先の枠が空なら rank を変えない。
func TestSetSprintToEmptyFrame(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s2, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint\n 2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetSprint(ctx, store, apptest.SampleTask, &s2.ID); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-2 の スプリント を Sprint 2 に変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	it, log := getItem(t, svc, apptest.SampleTask)
	if it.Sprint != s2.ID || it.Rank != rank.First {
		t.Errorf("空の枠へ: %s/%s", it.Sprint, it.Rank)
	}
	if got := lastOp(log); len(got) != 1 || got[0] != item.SprintField {
		t.Errorf("1 回の保存の行 = %v", got)
	}
}

// 使えないスプリントに入っているアイテムはバックログの枠にあるが、末尾は自分を除いて求める。
func TestSetSprintFromUnusableExcludesItself(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	addItem(t, svc, itemZ, 5, item.Task, "Z", "")
	addItem(t, svc, itemStory, 3, item.Story, "ZZ", "01NOSUCHSPRINT0000000000")
	if err := svc.SetSprint(ctx, store, itemStory, nil); err != nil {
		t.Fatal(err)
	}
	if it, _ := getItem(t, svc, itemStory); it.Sprint != "" || it.Rank != rank.After("Z") {
		t.Errorf("バックログへ: %s/%s", it.Sprint, it.Rank)
	}
}

const other workspace.ID = "01OTHERWS00000000000000001"

// otherWorkspace は、別のワークスペース other を作る。
func otherWorkspace(t *testing.T, svc planning.Service) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "ワークスペースを作成"}
	err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		n := 0
		d, err := workspace.CreateWithSettings(workspace.Workspace{ID: other, Name: "App", Prefix: "AP"},
			workspace.DefaultSettings(func() change.ID {
				n++
				return change.ID(fmt.Sprintf("01OTHERVAL%016d", n))
			}))
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(other, d)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetSprintRefusals(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s0, _ := activeSprint(t, svc)
	// 完了の Y は、スプリントの完了で移らず S0 に残る。
	addWorks(t, svc, []work{{itemY, 4, "Y", apptest.SampleStatusDone, s0.ID, 1}})
	addItem(t, svc, itemStep, 5, item.Step, "S", "")
	if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, s0.ID, nil); err != nil {
		t.Fatal(err)
	}
	s3, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 3"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s3.ID); err != nil {
		t.Fatal(err)
	}
	otherWorkspace(t, svc)
	elsewhere, err := svc.CreateSprint(ctx, store, other, planning.SprintInput{Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	addItem(t, svc, itemZ, 6, item.Task, "Z", "")
	if err := apptest.DeleteItems(ctx, svc.Stores, store, itemZ); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)

	for _, c := range []struct {
		name string
		id   item.ID
	}{{"Epic", apptest.SampleEpic}, {"Step", itemStep}} {
		if err := svc.SetSprint(ctx, store, c.id, nil); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if err := svc.SetSprint(ctx, store, apptest.SampleTask, &s0.ID); message(err) != "完了したスプリントには入れられません" {
		t.Errorf("完了したスプリント: %v", err)
	}
	if err := svc.SetSprint(ctx, store, itemY, &s0.ID); err != nil {
		t.Errorf("いま入っている完了したスプリント: %v", err)
	}
	if err := svc.SetSprint(ctx, store, apptest.SampleTask, nil); err != nil {
		t.Errorf("同じバックログ: %v", err)
	}
	unknown := change.ID("01NOSUCHSPRINT0000000000")
	for _, c := range []struct {
		name   string
		id     item.ID
		sprint *change.ID
	}{
		{"削除済みのスプリント", apptest.SampleTask, &s3.ID},
		{"ないスプリント", apptest.SampleTask, &unknown},
		{"別のワークスペースのスプリント", apptest.SampleTask, &elsewhere.ID},
		{"削除済みのアイテム", itemZ, nil},
		{"ないアイテム", "01NOSUCHITEM000000000000", nil},
	} {
		if err := svc.SetSprint(ctx, store, c.id, c.sprint); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if len(rec.summaries) != n {
		t.Errorf("断った・同じ値で保存した: %v", rec.summaries[n:])
	}
}

func TestSetRelease(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v2.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleEpic, &r.ID); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-1 の リリース を v2.1.0 に変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	it, log := getItem(t, svc, apptest.SampleEpic)
	if it.Release != r.ID || it.Rank != rank.First {
		t.Errorf("v2.1.0 へ: %s/%s", it.Release, it.Rank)
	}
	if got := lastOp(log); len(got) != 1 || got[0] != item.ReleaseField {
		t.Errorf("1 回の保存の行 = %v", got)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleEpic, nil); err != nil {
		t.Fatal(err)
	}
	if lastSummary(rec) != "HH-1 の リリース を なし に変更" {
		t.Errorf("summary = %q", lastSummary(rec))
	}
	if it, _ := getItem(t, svc, apptest.SampleEpic); it.Release != "" {
		t.Errorf("なしへ: %s", it.Release)
	}
}

func TestSetReleaseRefusals(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	old, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleEpic, &old.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetReleaseState(ctx, store, apptest.SampleWorkspace, old.ID, dp.Archived); err != nil {
		t.Fatal(err)
	}
	gone, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, gone.ID); err != nil {
		t.Fatal(err)
	}
	otherWorkspace(t, svc)
	elsewhere, err := svc.CreateRelease(ctx, store, other, planning.ReleaseInput{Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	addItem(t, svc, itemStep, 3, item.Step, "S", "")
	n := len(rec.summaries)

	if err := svc.SetRelease(ctx, store, itemStep, nil); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Step: %v", err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleTask, &old.ID); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("アーカイブのリリース: %v", err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleEpic, &old.ID); err != nil {
		t.Errorf("いま入っているアーカイブのリリース: %v", err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleTask, nil); err != nil {
		t.Errorf("同じなし: %v", err)
	}
	unknown := change.ID("01NOSUCHRELEASE000000000")
	for _, c := range []struct {
		name string
		id   *change.ID
	}{{"削除済みのリリース", &gone.ID}, {"ないリリース", &unknown}, {"別のワークスペースのリリース", &elsewhere.ID}} {
		if err := svc.SetRelease(ctx, store, apptest.SampleTask, c.id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if len(rec.summaries) != n {
		t.Errorf("断った・同じ値で保存した: %v", rec.summaries[n:])
	}
}
