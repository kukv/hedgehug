package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 候補は、閲覧のみの Store(readonly の Archive)と自分のワークスペース(Web)を除き、Store の並び → ワークスペースの並び。
func TestTargetsExcludeOwnWorkspaceReadOnlyStoresAndEmptyWorkspaces(t *testing.T) {
	f := newFixture(t)
	// プロジェクトのない、プロジェクトが削除済みのワークスペース(team の Empty)も出さない。
	addWorkspace(t, f.rec, store2, workspace.Workspace{ID: "01TESTWS000000000000000009", Name: "Empty", Prefix: "EM"}, "01TESTPR000000000000000009", "01TESTIT000000000000000009", nil)
	if err := apptest.DeleteProject(t.Context(), f.rec, store2, "01TESTPR000000000000000009"); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Targets(t.Context(), store1, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tg := range got {
		names = append(names, tg.Store.Name+"/"+tg.Workspace.Name)
	}
	if want := []string{"main/Mobile", "team/Team"}; !slices.Equal(names, want) {
		t.Fatalf("targets = %v, want %v", names, want)
	}
	mobile := got[0]
	if len(mobile.Projects) != 1 || mobile.Projects[0].ID != mobileProject || len(mobile.Epics[mobileProject]) != 1 || mobile.Epics[mobileProject][0].ID != mobileEpic {
		t.Errorf("Mobile の項目 = %+v", mobile)
	}
}

// Step は単独では移せない。ないもの・削除済みも同じ。
func TestTargetsOfAStepOrADeletedItemIsNotFound(t *testing.T) {
	f := newFixture(t)
	parent := apptest.SampleTask
	step := addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000010", Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3}, Kind: item.Step, Rank: "V", Title: "下書き"})
	if _, err := f.svc.Targets(t.Context(), store1, step.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Step: %v", err)
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask, step.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Targets(t.Context(), store1, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済み: %v", err)
	}
}

// 削除済みのワークスペース(B5b)は、候補に出さず、移動先に選べない(ErrNotFound)。中のプロジェクトがまだ
// 削除済みでなくても同じ(同期の修復がまだ削除済みに入れていないとき)。
func TestTargetsAndMoveSkipADeletedWorkspace(t *testing.T) {
	f := newFixture(t)
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store1, mobileWS); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Targets(t.Context(), store1, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Workspace.ID != teamWS {
		t.Errorf("targets = %+v, want only Team", got)
	}
	in := transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Move into the deleted workspace = %v, want ErrNotFound", err)
	}
}
