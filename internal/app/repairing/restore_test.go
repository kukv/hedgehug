package repairing_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/repairing"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	store    port.StoreID = "S1"
	project2 project.ID   = "01SAMPLEPR0000000000000002"
	review   change.ID    = "01REVIEW000000000000000001"
)

var (
	taro       = change.User("taro")
	statuses   = workspace.ParseList("statuses")
	reviewRef  = statuses.Ref(review)
	byGitHub   = change.Actor{Kind: change.ActorGitHub}
	byRule     = change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "01RULE00000000000000000001"}
	sampleRefs = []change.Ref{
		{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace},
		{Kind: change.KindProject, ID: apptest.SampleProject},
		{Kind: change.KindProject, ID: project2},
		{Kind: change.KindItem, ID: apptest.SampleEpic},
		{Kind: change.KindItem, ID: apptest.SampleTask},
	}
)

type env struct {
	t   *testing.T
	ctx context.Context
	s   *apptest.Stores
}

// newEnv は、サンプルデータ(HH-P1、Epic HH-1、その下の Task HH-2)と、ステータス「Review」と、
// HH-P1 と同じワークスペースのプロジェクト HH-P2 を書いた Store を作る。HH-2 のステータスは Review。
func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, ctx: context.Background(), s: apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())}
	e.s.AddStore(store)
	if err := apptest.Sample(e.ctx, e.s, store); err != nil {
		t.Fatal(err)
	}
	p2 := project.Project{ID: project2, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "管理", Owner: "taro", Status: project.Todo}
	rv := workspace.Value{ID: review, Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(p2.ID, project.Create(p2)),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, workspace.ValueDrafts(statuses, rv)...),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, review)),
		)
	})
	return e
}

// rows は、サンプルのすべてのログの行。
func (e *env) rows() []change.Record {
	e.t.Helper()
	var out []change.Record
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		for _, ref := range sampleRefs {
			log, err := r.Changes().Of(ref)
			if err != nil && !errors.Is(err, port.ErrNotFound) {
				return err
			}
			out = append(out, log...)
		}
		return nil
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

// write は、1 回の保存を書き、足した行の ID を返す。
func (e *env) write(actor change.Actor, fn func(tx port.Tx) error) map[change.ID]bool {
	e.t.Helper()
	before := map[change.ID]bool{}
	for _, r := range e.rows() {
		before[r.ID] = true
	}
	if err := e.s.Write(e.ctx, store, port.Operation{Actor: actor, Summary: "テストの操作"}, fn); err != nil {
		e.t.Fatal(err)
	}
	added := map[change.ID]bool{}
	for _, r := range e.rows() {
		if !before[r.ID] {
			added[r.ID] = true
		}
	}
	return added
}

func (e *env) appendItem(actor change.Actor, id item.ID, ds ...change.Draft) map[change.ID]bool {
	return e.write(actor, func(tx port.Tx) error { return tx.WriteItems().Append(id, ds...) })
}

// run は、ours と theirs を Facts にして Run を 2 回行い、1 回目の保存を返す。2 回目は何も書かないことを確かめる。
func (e *env) run(ours, theirs map[change.ID]bool) []port.Operation {
	e.t.Helper()
	w := &apptest.RepairWriter{Stores: e.s, Store: store, F: repair.Facts{Ours: ours, Theirs: theirs}}
	if err := repairing.Run(e.ctx, w); err != nil {
		e.t.Fatal(err)
	}
	n := len(e.rows())
	again := &apptest.RepairWriter{Stores: e.s, Store: store, F: w.F}
	if err := repairing.Run(e.ctx, again); err != nil {
		e.t.Fatal(err)
	}
	if len(again.Ops) != 0 || len(e.rows()) != n {
		e.t.Errorf("second Run wrote %v (%d rows -> %d)", again.Ops, n, len(e.rows()))
	}
	return w.Ops
}

func (e *env) item(id item.ID) item.Item {
	e.t.Helper()
	var it item.Item
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(id)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return it
}

func (e *env) project(id project.ID) project.Project {
	e.t.Helper()
	var p project.Project
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		p, err = r.Projects().Get(id)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func union(sets ...map[change.ID]bool) map[change.ID]bool {
	out := map[change.ID]bool{}
	for _, s := range sets {
		maps.Copy(out, s)
	}
	return out
}

func wantOps(t *testing.T, got []port.Operation, summaries ...string) {
	t.Helper()
	if len(got) != len(summaries) {
		t.Fatalf("ops = %+v, want %q", got, summaries)
	}
	for i, op := range got {
		if op.Summary != summaries[i] || op.Actor != (change.Actor{Kind: change.ActorSync}) {
			t.Errorf("op %d = %+v, want %q by sync", i, op, summaries[i])
		}
	}
}

func deleteEpic(e *env) map[change.ID]bool {
	return e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleEpic, change.NewDeleteEntity(true)),
			tx.WriteItems().Append(apptest.SampleTask, change.NewDeleteEntity(false)),
		)
	})
}

// アイテムの削除と、そのアイテムの変更が重なったら、変更を残して削除を取り消す。
func TestRestoreItem(t *testing.T) {
	e := newEnv(t)
	del := e.appendItem(taro, apptest.SampleTask, change.NewDeleteEntity(true))
	title := e.appendItem(taro, apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	ops := e.run(del, title)
	wantOps(t, ops, "HH-2 の削除を取り消し")
	if it := e.item(apptest.SampleTask); it.Deleted != nil || it.Title != "ログイン画面" {
		t.Errorf("HH-2 = deleted %v, title %q", it.Deleted, it.Title)
	}
}

// 親の削除の取り消しは、一緒に削除した子も戻す。
func TestRestoreItemWithChildren(t *testing.T) {
	e := newEnv(t)
	del := deleteEpic(e)
	title := e.appendItem(taro, apptest.SampleEpic, change.NewSet("title", "認証", "ログイン"))
	wantOps(t, e.run(del, title), "HH-1 の削除を取り消し")
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if it := e.item(id); it.Deleted != nil {
			t.Errorf("%s is still deleted", id)
		}
	}
}

// 親の削除と子の変更は、子のログだけに変更があるので戻さない(保存形式「テスト」の「親と一緒の削除」)。
func TestNoRestoreForChildChange(t *testing.T) {
	e := newEnv(t)
	del := deleteEpic(e)
	title := e.appendItem(taro, apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	wantOps(t, e.run(del, title))
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if it := e.item(id); it.Deleted == nil {
			t.Errorf("%s was restored", id)
		}
	}
}

// 人の操作でない変更と、ほかのものへの操作で一緒に書いた行では、削除を取り消さない(C3a-9)。
func TestNoRestoreForIncidentalChanges(t *testing.T) {
	tests := []struct {
		name   string
		theirs func(e *env) map[change.ID]bool
	}{
		{"github and automation", func(e *env) map[change.ID]bool {
			return union(
				e.appendItem(byGitHub, apptest.SampleTask, change.NewSet("links/1/state", "open", "closed")),
				e.appendItem(byRule, apptest.SampleTask, change.NewSet("status", review, apptest.SampleStatusDone)),
			)
		}},
		{"parent moved to another project", func(e *env) map[change.ID]bool {
			return e.write(taro, func(tx port.Tx) error {
				return errors.Join(
					movePlace(tx, apptest.SampleEpic, project2),
					movePlace(tx, apptest.SampleTask, project2),
				)
			})
		}},
		{"replaced by a status delete", func(e *env) map[change.ID]bool {
			return e.write(taro, func(tx port.Tx) error {
				return errors.Join(
					tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(reviewRef)),
					tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", review, apptest.SampleStatusTodo)),
				)
			})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			del := e.appendItem(taro, apptest.SampleTask, change.NewDeleteEntity(true))
			wantOps(t, e.run(del, tt.theirs(e)))
			if it := e.item(apptest.SampleTask); it.Deleted == nil {
				t.Error("HH-2 was restored")
			}
		})
	}
}

// movePlace は、アイテムのプロジェクトを p にする set place の行を書く。
func movePlace(tx port.Tx, id item.ID, p project.ID) error {
	it, err := tx.Items().Get(id)
	if err != nil {
		return err
	}
	next := it.Place
	next.Project = p
	return tx.WriteItems().Append(id, change.NewSet("place", it.Place, next))
}

// プロジェクトの削除の取り消しは、一緒に削除した下のアイテムも戻す。
func TestRestoreProject(t *testing.T) {
	e := newEnv(t)
	del := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(apptest.SampleProject, change.NewDeleteEntity(true)),
			tx.WriteItems().Append(apptest.SampleEpic, change.NewDeleteEntity(false)),
			tx.WriteItems().Append(apptest.SampleTask, change.NewDeleteEntity(false)),
		)
	})
	title := e.write(taro, func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject, change.NewSet("title", "ログイン", "認証基盤"))
	})
	wantOps(t, e.run(del, title), "HH-P1 の削除を取り消し")
	if p := e.project(apptest.SampleProject); p.Deleted != nil || p.Title != "認証基盤" {
		t.Errorf("HH-P1 = deleted %v, title %q", p.Deleted, p.Title)
	}
	for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask} {
		if it := e.item(id); it.Deleted != nil {
			t.Errorf("%s is still deleted", id)
		}
	}
}

// 値の削除の取り消しは、同じ操作で置き換えたアイテムの値も戻す。
func TestRestoreStatus(t *testing.T) {
	e := newEnv(t)
	del := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(reviewRef)),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", review, apptest.SampleStatusTodo)),
		)
	})
	color := e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet(statuses.Field(review, "color"), "blue", "green"))
	})
	wantOps(t, e.run(del, color), "「Review」の削除を取り消し")
	var st workspace.Settings
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		st, err = r.Workspaces().Settings(apptest.SampleWorkspace)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if last := st.Statuses[len(st.Statuses)-1]; last.ID != review || last.Color != "green" {
		t.Errorf("statuses = %+v, want Review in green last", st.Statuses)
	}
	if it := e.item(apptest.SampleTask); it.Status != review {
		t.Errorf("HH-2 status = %s, want Review", it.Status)
	}
}

// 名前に改行や続いた空白があっても、アクティビティの文は 1 行にする(保存の Summary は 1 行だけ書ける)。
func TestRestoreSummaryOnOneLine(t *testing.T) {
	e := newEnv(t)
	e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet(statuses.Field(review, "name"), "Review", "Re\nview  中"))
	})
	del := e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(reviewRef))
	})
	color := e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet(statuses.Field(review, "color"), "blue", "green"))
	})
	wantOps(t, e.run(del, color), "「Re view 中」の削除を取り消し")
}
