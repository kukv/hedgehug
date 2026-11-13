package items_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// otherProject は、サンプルと同じワークスペースの 2 つ目のプロジェクト(HH-P2)。
const otherProject project.ID = "01TESTPR000000000000000002"

// addSecondProject は、サンプルのワークスペースに 2 つ目のプロジェクト(名前「決済」、サンプルのプロジェクトの次の並び)を足す。
func addSecondProject(t *testing.T, s port.Stores) {
	t.Helper()
	pr := project.Project{ID: otherProject, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "決済", Owner: "taro", Status: project.Todo}
	op := port.Operation{Actor: change.User("taro"), Summary: "プロジェクトを作成"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Projects, 2)); err != nil {
			return err
		}
		return tx.WriteProjects().Append(pr.ID, project.Create(pr))
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 別のワークスペース「Mobile」(MB)と、そのプロジェクトと Epic。
const (
	mobileWorkspace workspace.ID = "01TESTWS000000000000000002"
	mobileProject   project.ID   = "01TESTPR000000000000000003"
	mobileEpic      item.ID      = "01TESTIT000000000000000001"
)

// addOtherWorkspace は、別のワークスペース「Mobile」に、プロジェクト MB-P1 と Epic MB-1 を足す。
func addOtherWorkspace(t *testing.T, s port.Stores) {
	t.Helper()
	n := 0
	d, err := workspace.CreateWithSettings(workspace.Workspace{ID: mobileWorkspace, Name: "Mobile", Prefix: "MB"},
		workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("01TESTVAL%017d", n)) }))
	if err != nil {
		t.Fatal(err)
	}
	pr := project.Project{ID: mobileProject, Place: project.Place{Workspace: mobileWorkspace, Number: 1}, Rank: "V", Title: "アプリ", Status: project.Todo}
	epic := item.Item{ID: mobileEpic, Place: item.Place{Workspace: mobileWorkspace, Project: mobileProject, Number: 1}, Kind: item.Epic, Rank: "V", Title: "iOS"}
	op := port.Operation{Actor: change.User("taro"), Summary: "テスト"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(mobileWorkspace, d),
			tx.WriteProjects().Append(mobileProject, project.Create(pr)),
			tx.WriteItems().Append(mobileEpic, item.Create(epic)),
		)
	}); err != nil {
		t.Fatal(err)
	}
}

// mustCreate は、アイテムを作って返す(作成の応答。失敗したらテストを止める)。
func mustCreate(t *testing.T, svc items.Service, p project.ID, parent *item.ID, kind item.Kind, title string) item.Item {
	t.Helper()
	it, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: p, Parent: parent, Kind: kind, Title: title})
	if err != nil {
		t.Fatal(err)
	}
	return it
}

// markDeleted は、アイテムを削除済みにする(行を直接書く。下のものには書かない)。
func markDeleted(t *testing.T, s port.Stores, id item.ID, root bool) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "削除"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewDeleteEntity(root))
	}); err != nil {
		t.Fatal(err)
	}
}

// storedItem は、保存の口から読んだアイテム(削除済みも返る)。
func storedItem(t *testing.T, s port.Stores, id item.ID) item.Item {
	t.Helper()
	var it item.Item
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return it
}

// markProjectDeleted は、2 つ目のプロジェクトを削除済みにする(行を直接書く)。
func markProjectDeleted(t *testing.T, s port.Stores) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "削除"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteProjects().Append(otherProject, change.NewDeleteEntity(true))
	}); err != nil {
		t.Fatal(err)
	}
}

func pptr(id project.ID) *project.ID { return &id }

// lastOps は、アイテムのログの最後の n 行の op(同じ 1 回の保存かを確かめる)。
func lastOps(t *testing.T, s port.Stores, id item.ID, n int) []change.ID {
	t.Helper()
	log := records(t, s, id)
	var ops []change.ID
	for _, rec := range log[max(0, len(log)-n):] {
		ops = append(ops, rec.Op)
	}
	return ops
}

// samePlace は、場所が同じか(親はポインタなので中身で比べる)。
func samePlace(a, b item.Place) bool {
	return a.Workspace == b.Workspace && a.Project == b.Project && a.Number == b.Number &&
		(a.Parent == nil) == (b.Parent == nil) && (a.Parent == nil || *a.Parent == *b.Parent)
}

// addRelation は、from のログに関連(種類 typ、相手 to)を足す(行を直接書く)。
func addRelation(t *testing.T, s port.Stores, from, to item.ID, typ string) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "関連を追加"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(from, change.NewAdd("relations", item.Relation{Type: typ, To: to}))
	}); err != nil {
		t.Fatal(err)
	}
}

// historyTexts は、アイテムの履歴の文(新しい順)。
func historyTexts(t *testing.T, s port.Stores, id item.ID) []string {
	t.Helper()
	var out []string
	for _, row := range history.ItemRows(records(t, s, id), history.Context{Names: func(string) string { return "" }}) {
		out = append(out, row.Text)
	}
	return out
}
