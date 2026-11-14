package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
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

// プロジェクトの移動先の候補は、プロジェクトのないワークスペース(main の Empty)も入れる。自分のワークスペース(Mobile)・
// 削除済みのワークスペース(main の Gone)・閲覧のみの Store(readonly)・非表示の Store(S4)は入れない。
// 並びは Store の並び → ワークスペースの並び(rank の昇順。Empty は rank で Web より前)→ 内部 ID。
func TestProjectTargetsIncludeWorkspacesWithoutProjects(t *testing.T) {
	f := newFixture(t)
	empty := workspace.Workspace{ID: "01TESTWS000000000000000009", Name: "Empty", Prefix: "EM"}
	addWorkspace(t, f.rec, store1, empty, "", "", nil)
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("rank", nil, "V")),
			tx.WriteWorkspaces().Append(empty.ID, change.NewSet("rank", nil, "0")),
		)
	})
	addWorkspace(t, f.rec, store1, workspace.Workspace{ID: "01TESTWS000000000000000008", Name: "Gone", Prefix: "GO"}, "", "", nil)
	if err := apptest.DeleteWorkspace(t.Context(), f.rec, store1, "01TESTWS000000000000000008"); err != nil {
		t.Fatal(err)
	}
	const hidden port.StoreID = "S4"
	f.rec.AddStore(hidden)
	addWorkspace(t, f.rec, hidden, workspace.Workspace{ID: "01TESTWS000000000000000007", Name: "Hidden", Prefix: "HI"}, "", "", nil)

	got, err := f.svc.ProjectTargets(t.Context(), store1, mobileProject)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tg := range got {
		names = append(names, tg.Store.Name+"/"+tg.Workspace.Name)
	}
	if want := []string{"main/Empty", "main/Web", "team/Team"}; !slices.Equal(names, want) {
		t.Fatalf("targets = %v, want %v", names, want)
	}
}

// ないプロジェクト・削除済みのプロジェクト(別の画面で利用者が削除したもの = root: true を含む)は、候補を返さない。
func TestProjectTargetsRefuseUnknownProject(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.ProjectTargets(t.Context(), store1, project.ID("01TESTPR000000000000000099")); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ない: %v", err)
	}
	// ワークスペースの削除で一緒に削除済みになったもの(root: false)。
	write(t, f.rec, store2, func(tx port.Tx) error { return tx.WriteProjects().Append(teamProject, change.NewDeleteEntity(false)) })
	if _, err := f.svc.ProjectTargets(t.Context(), store2, teamProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済み(root: false): %v", err)
	}
	if err := apptest.DeleteProject(t.Context(), f.rec, store1, mobileProject); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ProjectTargets(t.Context(), store1, mobileProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済み: %v", err)
	}
}
