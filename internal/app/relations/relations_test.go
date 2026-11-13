package relations_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/relations"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const store port.StoreID = "S1"

// サンプルのほかに足すもの: HH-3(Task、ステータス Done)、HH-4(HH-2 の下の Step)、
// ワークスペース「Mobile」(MB)の MB-1(Epic)。
const (
	wsMobile workspace.ID = "01TESTWS00000000000000002"
	prMobile project.ID   = "01TESTPR00000000000000002"
	itDone   item.ID      = "01TESTIT00000000000000003"
	itStep   item.ID      = "01TESTIT00000000000000004"
	itMobile item.ID      = "01TESTIT00000000000000005"
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func (r *recorder) last() string {
	if len(r.summaries) == 0 {
		return ""
	}
	return r.summaries[len(r.summaries)-1]
}

func setup(t *testing.T) (relations.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	seed(t, s)
	rec := &recorder{Stores: s}
	return relations.Service{Stores: rec, Identity: apptest.Identity{Login: "hanako"}}, rec
}

func seed(t *testing.T, s port.Stores) {
	t.Helper()
	n := 0
	valueIDs := func() change.ID { n++; return change.ID(fmt.Sprintf("01TESTVAL%016d", n)) }
	epic, task := apptest.SampleEpic, apptest.SampleTask
	mk := func(id item.ID, number int, kind item.Kind, parent *item.ID, title string, status change.ID) item.Item {
		return item.Item{
			ID: id, Kind: kind, Rank: "W", Title: title, Status: status,
			Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: parent, Number: number},
		}
	}
	done := mk(itDone, 3, item.Task, &epic, "決済", apptest.SampleStatusDone)
	step := mk(itStep, 4, item.Step, &task, "設計", apptest.SampleStatusTodo)
	mobile := item.Item{ID: itMobile, Kind: item.Epic, Rank: "V", Title: "iOS ログイン", Place: item.Place{Workspace: wsMobile, Project: prMobile, Number: 1}}
	ws := workspace.Workspace{ID: wsMobile, Name: "Mobile", Prefix: "MB"}
	pr := project.Project{ID: prMobile, Place: project.Place{Workspace: wsMobile, Number: 1}, Rank: "V", Title: "アプリ", Status: project.Todo}
	d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(valueIDs))
	if err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "テストデータを作成"}
	err = s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(wsMobile, d),
			tx.WriteProjects().Append(prMobile, project.Create(pr)),
			tx.WriteItems().Append(itDone, item.Create(done)),
			tx.WriteItems().Append(itStep, item.Create(step)),
			tx.WriteItems().Append(itMobile, item.Create(mobile)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// addRaw は、操作を通さずに、owner のログに関連を足す(読むテストの前提を作る)。
func addRaw(t *testing.T, s port.Stores, owner item.ID, typ string, to item.ID) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "関連を足す"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(owner, change.NewAdd("relations", item.Relation{Type: typ, To: to}))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func relationsOf(t *testing.T, s port.Stores, id item.ID) []item.Relation {
	t.Helper()
	var rels []item.Relation
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		rels = it.Relations
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return rels
}

func logLen(t *testing.T, s port.Stores, id item.ID) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}
