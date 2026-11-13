package history_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	mobileWorkspace workspace.ID = "01TESTWS000000000000000002"
	mobileProject   project.ID   = "01TESTPR000000000000000002"
)

// addMobile は、S1 にワークスペース「Mobile」(MB。値の内部 ID は VALMB001〜)とプロジェクト「アプリ」を足す。
func addMobile(t *testing.T, s port.Stores) {
	t.Helper()
	n := 0
	st := workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("VALMB%03d", n)) })
	d, err := workspace.CreateWithSettings(workspace.Workspace{ID: mobileWorkspace, Name: "Mobile", Prefix: "MB"}, st)
	if err != nil {
		t.Fatal(err)
	}
	p := project.Project{ID: mobileProject, Place: project.Place{Workspace: mobileWorkspace, Number: 1}, Rank: "V", Title: "アプリ", Status: project.Todo}
	write(t, s, "taro", func(tx port.Tx) error {
		return errors.Join(tx.WriteWorkspaces().Append(mobileWorkspace, d), tx.WriteProjects().Append(mobileProject, project.Create(p)))
	})
}

// 同じ Store の別のワークスペースへ移したアイテムの履歴。移動の行は、転送記録の旧 ID と、移動元の Store・ワークスペース・
// プロジェクトの名前で作り、移る前の値は移動元のワークスペース(Web)の定義で引く。
func TestItemHistoryOfAnItemMovedWithinTheStore(t *testing.T) {
	svc, s := setup(t)
	addMobile(t, s)
	write(t, s, "taro", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusInProgress))
	})
	write(t, s, "hanako", func(tx port.Tx) error {
		task, err := tx.Items().Get(apptest.SampleTask)
		if err != nil {
			return err
		}
		to := task.Place
		to.Workspace, to.Project, to.Parent, to.Number = mobileWorkspace, mobileProject, nil, 2
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("place", task.Place, to)),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "MB-2", ToStore: string(store)}),
		)
	})
	rows, err := svc.Item(t.Context(), store, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"hanako: HH-2 から移動(main / Web / ログイン)", "taro: ステータスを Todo → InProgress", "taro: 作成"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}
