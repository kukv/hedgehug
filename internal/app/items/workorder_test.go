package items_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workTail は、Epic「決済」の下に Story を 2 件作る(親の子の末尾と作業項目の末尾が食い違う並びにするため)。ワークスペースの作業項目の末尾はその Story の順位で、
// HH-1(Epic)の子の末尾(HH-2 の順位)より後ろにある。
func workTail(t *testing.T, svc items.Service) item.Item {
	t.Helper()
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	mustCreate(t, svc, apptest.SampleProject, &epic2.ID, item.Story, "決済の入力")
	tail := mustCreate(t, svc, apptest.SampleProject, &epic2.ID, item.Story, "決済の画面")
	return tail
}

func TestCreateAppendsToWorkspaceOrder(t *testing.T) {
	svc, _ := setupRec(t)
	tail := workTail(t, svc)
	got := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Story, "ログインの画面")
	if want := rank.After(tail.Rank); got.Rank != want {
		t.Errorf("Story の順位 = %q, want %q(ワークスペースの作業項目の末尾のあと)", got.Rank, want)
	}
	step := mustCreate(t, svc, apptest.SampleProject, &tail.ID, item.Step, "手順")
	if want := rank.After(""); step.Rank != want {
		t.Errorf("Step の順位 = %q, want %q(親の子の末尾のまま)", step.Rank, want)
	}
}

func TestCreateFromATemplateAppendsChildrenToWorkspaceOrder(t *testing.T) {
	svc, rec := setupRec(t)
	tail := workTail(t, svc)
	tp := saveTemplate(t, svc, rec, workspace.Template{
		Name: "リリース", Kind: "Epic", Title: "リリース",
		Children: []workspace.TemplateChild{{Kind: "Story", Title: "A"}, {Kind: "Task", Title: "B"}},
	})
	root, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Kind: "Epic", Title: "11 月のリリース", Template: &tp.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	all := byNumber(t, svc)
	a, b := all[root.Place.Number+1], all[root.Place.Number+2]
	if a.Title != "A" || b.Title != "B" {
		t.Fatalf("children = %q, %q", a.Title, b.Title)
	}
	if want := rank.After(tail.Rank); a.Rank != want || b.Rank != rank.After(want) {
		t.Errorf("子の順位 = %q, %q, want %q, %q", a.Rank, b.Rank, want, rank.After(want))
	}
}

func TestDuplicateAppendsToWorkspaceOrder(t *testing.T) {
	svc, rec := setupRec(t)
	tail := workTail(t, svc)
	id, err := svc.Duplicate(t.Context(), store, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := storedItem(t, rec, id).Rank, rank.After(tail.Rank); got != want {
		t.Errorf("複製の順位 = %q, want %q", got, want)
	}
}

func TestChangeKindPromotionAppendsToWorkspaceOrder(t *testing.T) {
	svc, rec := setupRec(t)
	tail := workTail(t, svc)
	step, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "下書き")
	if err := svc.ChangeKind(t.Context(), store, step.ID, item.Story); err != nil {
		t.Fatal(err)
	}
	if got, want := storedItem(t, rec, step.ID).Rank, rank.After(tail.Rank); got != want {
		t.Errorf("昇格した Story の順位 = %q, want %q", got, want)
	}
}

func TestMoveAppendsToWorkspaceOrder(t *testing.T) {
	svc, rec := setupRec(t)
	tail := workTail(t, svc)
	empty := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "通知")
	if err := svc.Move(t.Context(), store, apptest.SampleTask, items.MoveInput{Parent: &empty.ID}); err != nil {
		t.Fatal(err)
	}
	if got, want := storedItem(t, rec, apptest.SampleTask).Rank, rank.After(tail.Rank); got != want {
		t.Errorf("移した Task の順位 = %q, want %q", got, want)
	}
}

func TestMoveEpicKeepsTheEndOfTheProject(t *testing.T) {
	svc, rec := setupRec(t)
	addSecondProject(t, rec)
	workTail(t, svc)
	other := mustCreate(t, svc, otherProject, nil, item.Epic, "別の Epic")
	if err := svc.Move(t.Context(), store, apptest.SampleEpic, items.MoveInput{Project: pptr(otherProject)}); err != nil {
		t.Fatal(err)
	}
	if got, want := storedItem(t, rec, apptest.SampleEpic).Rank, rank.After(other.Rank); got != want {
		t.Errorf("移した Epic の順位 = %q, want %q(プロジェクトの直下の末尾のあと)", got, want)
	}
}
