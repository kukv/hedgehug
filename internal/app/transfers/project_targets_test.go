package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

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
	if err := apptest.DeleteProject(t.Context(), f.rec, store1, mobileProject); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ProjectTargets(t.Context(), store1, mobileProject); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済み: %v", err)
	}
}
