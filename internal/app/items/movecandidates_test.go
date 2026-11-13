package items_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// displays は、候補の画面の ID の並び。
func displays(cs []items.MoveCandidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Display)
	}
	return out
}

func TestMoveCandidatesForAnEpicAreTheOtherLiveProjects(t *testing.T) {
	svc, rec := setupRec(t)
	got, err := svc.MoveCandidates(t.Context(), store, apptest.SampleEpic)
	if err != nil || len(got) != 0 {
		t.Fatalf("プロジェクトが 1 つだけなら候補なし: %+v, %v", got, err)
	}
	addSecondProject(t, rec)  // HH-P2「決済」
	addOtherWorkspace(t, rec) // 別のワークスペースのプロジェクトは出ない
	got, err = svc.MoveCandidates(t.Context(), store, apptest.SampleEpic)
	if err != nil || !slices.Equal(displays(got), []string{"HH-P2"}) || got[0].Project == nil || got[0].Item != nil || got[0].Project.Title != "決済" {
		t.Fatalf("candidates = %+v, %v", got, err)
	}
	if got[0].ProjectID != otherProject || got[0].ProjectTitle != "" {
		t.Errorf("プロジェクトの候補の ProjectID は自分の ID、ProjectTitle は空: %+v", got[0])
	}
	markProjectDeleted(t, rec)
	if got, err = svc.MoveCandidates(t.Context(), store, apptest.SampleEpic); err != nil || len(got) != 0 {
		t.Errorf("削除済みのプロジェクトは候補にしない: %+v, %v", got, err)
	}
}

func TestMoveCandidatesForATaskAreEpicsAcrossProjects(t *testing.T) {
	svc, rec := setupRec(t)
	addSecondProject(t, rec)
	epic3 := mustCreate(t, svc, otherProject, nil, item.Epic, "決済の Epic")              // HH-3(P2)
	epic4 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "もう 1 つの Epic") // HH-4(P1)
	trashed := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "削除した Epic")  // HH-5
	markDeleted(t, rec, trashed.ID, true)
	addOtherWorkspace(t, rec)
	got, err := svc.MoveCandidates(t.Context(), store, apptest.SampleTask)
	// いまの親 HH-1、削除済みの HH-5、別のワークスペースの MB-1 は出ない。プロジェクトの並び(HH-P1 → HH-P2)→ ツリーの並び。
	if err != nil || !slices.Equal(displays(got), []string{"HH-4", "HH-3"}) {
		t.Fatalf("candidates = %v, %v", displays(got), err)
	}
	if got[0].Item.ID != epic4.ID || got[0].ProjectTitle != "ログイン" || got[0].ProjectID != apptest.SampleProject || got[0].Project != nil ||
		got[1].Item.ID != epic3.ID || got[1].ProjectTitle != "決済" || got[1].ProjectID != otherProject {
		t.Errorf("fields = %+v / %+v", got[0], got[1])
	}
}

func TestMoveCandidatesFollowTheTreeOrder(t *testing.T) {
	svc, _ := setupRec(t)
	step := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleTask), item.Step, "下書き") // HH-3
	epic4 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "2 つ目の Epic")             // HH-4
	mustCreate(t, svc, apptest.SampleProject, &epic4.ID, item.Bug, "不具合")                        // HH-5
	story := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Story, "話") // HH-6
	mustCreate(t, svc, apptest.SampleProject, &story.ID, item.Step, "話の Step")                   // HH-7
	got, err := svc.MoveCandidates(t.Context(), store, step.ID)
	// 番号の順ではなくツリーの並び: Epic HH-1 の下の HH-2(いまの親なので出ない)・HH-6、次に Epic HH-4 の下の HH-5。
	// Step・Epic は出ない。
	if err != nil || !slices.Equal(displays(got), []string{"HH-6", "HH-5"}) {
		t.Fatalf("candidates = %v, %v", displays(got), err)
	}
}

func TestMoveCandidatesForAStepAreStoriesTasksAndBugs(t *testing.T) {
	svc, rec := setupRec(t)
	addSecondProject(t, rec)
	step := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleTask), item.Step, "下書き") // HH-3
	bug := mustCreate(t, svc, apptest.SampleProject, ptr(apptest.SampleEpic), item.Bug, "不具合")   // HH-4
	epic5 := mustCreate(t, svc, otherProject, nil, item.Epic, "決済")                              // HH-5
	story := mustCreate(t, svc, otherProject, &epic5.ID, item.Story, "支払い")                      // HH-6
	gone := mustCreate(t, svc, otherProject, &epic5.ID, item.Task, "消した Task")                   // HH-7
	markDeleted(t, rec, gone.ID, true)
	got, err := svc.MoveCandidates(t.Context(), store, step.ID)
	// いまの親 HH-2 と Epic・削除済みは出ない。
	if err != nil || !slices.Equal(displays(got), []string{"HH-4", "HH-6"}) || got[0].Item.ID != bug.ID || got[1].Item.ID != story.ID {
		t.Fatalf("candidates = %v, %v", displays(got), err)
	}
}

func TestMoveCandidatesNotFound(t *testing.T) {
	svc, rec := setupRec(t)
	epic2 := mustCreate(t, svc, apptest.SampleProject, nil, item.Epic, "決済")
	markDeleted(t, rec, epic2.ID, true)
	for _, id := range []item.ID{"01NOPE00000000000000000000", epic2.ID} {
		if _, err := svc.MoveCandidates(t.Context(), store, id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", id, err)
		}
	}
}
