package noticing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/worklog"
)

// サンプルの優先度 High の内部 ID(DefaultSettings が振る 7 番目)。
const samplePriorityHigh change.ID = "01SAMPLEVAL000000000000007"

// watchTask は、hanako がサンプルのタスク HH-2 をウォッチする(判定しない保存)。
func (f *fixture) watchTask() {
	f.t.Helper()
	f.save(user("hanako"), setTask(change.NewAdd("watchers", "hanako")))
}

func setStatus(from, to change.ID) change.Draft { return change.NewSet("status", from, to) }

func texts(ns []port.Notice) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Text)
	}
	return out
}

func TestWatchStatusChanged(t *testing.T) {
	f := setup(t)
	f.watchTask()
	f.judge(f.save(user("taro"), setTask(setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone))))
	n := f.wantOne()
	want := port.Notice{
		Kind: port.NoticeWatch, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask,
		Text: "ステータスを Done に変更", Display: "HH-2", Sender: "taro",
	}
	if n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestWatchSameValue(t *testing.T) {
	f := setup(t)
	f.watchTask()
	a := f.save(user("taro"), setTask(setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone)))
	b := f.save(user("taro"), setTask(setStatus(apptest.SampleStatusDone, apptest.SampleStatusTodo)))
	f.judge(a, b)
	f.wantNone()
}

func TestWatchTitle(t *testing.T) {
	f := setup(t)
	f.watchTask()
	f.judge(f.save(user("taro"), setTask(change.NewSet("title", "ログイン画面を作る", "新しい名前"))))
	if n := f.wantOne(); n.Kind != port.NoticeWatch || n.Text != "タイトルを「新しい名前」に変更" || n.Sender != "taro" {
		t.Errorf("notice = %+v", n)
	}
}

func TestWatchTwoFields(t *testing.T) {
	f := setup(t)
	f.watchTask()
	f.judge(f.save(user("taro"), setTask(
		setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone),
		change.NewSet("priority", apptest.SamplePriorityMedium, samplePriorityHigh),
	)))
	got := texts(f.notices())
	if len(got) != 2 || got[0] != "ステータスを Done に変更" || got[1] != "優先度を High に変更" {
		t.Errorf("texts = %q", got)
	}
}

// 複数の値を持つプロパティ(関連システム)は、add・remove の行が差出人になる。
func TestWatchListProperty(t *testing.T) {
	f := setup(t)
	f.watchTask()
	f.judge(f.save(user("taro"), setTask(change.NewAdd("systems", apptest.SampleSystemWeb))))
	if n := f.wantOne(); n.Text != "関連システムを Web に変更" || n.Sender != "taro" {
		t.Errorf("notice = %+v", n)
	}

	// あなたの別のマシンでの追加は、自分の変更
	g := setup(t)
	g.watchTask()
	g.judge(g.save(user("hanako"), setTask(change.NewAdd("systems", apptest.SampleSystemWeb))))
	g.wantNone()
}

// 取り除いて足し直した複数の値は、並びが変わっても同じ値なので作らない。
func TestWatchListSameSet(t *testing.T) {
	const api change.ID = "01SAMPLEVAL000000000000012"
	f := setup(t)
	f.watchTask()
	f.save(user("taro"), setTask(change.NewAdd("systems", apptest.SampleSystemWeb), change.NewAdd("systems", api)))
	a := f.save(user("taro"), setTask(change.NewRemove("systems", apptest.SampleSystemWeb)))
	b := f.save(user("taro"), setTask(change.NewAdd("systems", apptest.SampleSystemWeb)))
	f.judge(a, b)
	f.wantNone()
}

func TestWatchComment(t *testing.T) {
	f := setup(t)
	f.watchTask()
	f.judge(f.save(user("taro"), setTask(
		change.NewComment("01C1", nil, comment.Value("1 件目")),
		change.NewComment("01C2", nil, comment.Value("2 件目\nの本文")),
	)))
	got := f.notices()
	if len(got) != 2 || got[0].Text != "コメント: 1 件目" || got[1].Text != "コメント: 2 件目 の本文" || got[0].Sender != "taro" {
		t.Errorf("notices = %+v", got)
	}

	g := setup(t)
	g.watchTask()
	g.save(user("taro"), writeComment("", "前からあるコメント"))
	edit := g.save(user("taro"), writeComment("前からあるコメント", "編集した"))
	del := g.save(user("taro"), setTask(change.NewComment(commentID, comment.Value("編集した"), nil)))
	g.judge(edit, del)
	g.wantNone()
}

func TestWatchSkips(t *testing.T) {
	other := apptest.SampleTask
	cases := map[string]struct {
		actor  change.Actor
		watch  bool
		drafts []change.Draft
	}{
		"説明":    {user("taro"), true, []change.Draft{change.NewDesc()}},
		"関連":    {user("taro"), true, []change.Draft{change.NewAdd("relations", item.Relation{Type: "relates", To: apptest.SampleEpic})}},
		"作業時間":  {user("taro"), true, []change.Draft{change.NewWorklog("01W1", nil, worklog.Value(100, "", "2026-11-14"))}},
		"スプリント": {user("taro"), true, []change.Draft{change.NewSet(item.SprintField, nil, "01SPRINT0000000000000001")}},
		"移動": {user("taro"), true, []change.Draft{
			change.NewSet("place",
				item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Number: 2},
				item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 2}),
			change.NewSet("rank", rank.First, rank.After(rank.First)),
		}},
		"種類":         {user("taro"), true, []change.Draft{change.NewSet("kind", "Task", "Bug")}},
		"ウォッチしていない":  {user("taro"), false, []change.Draft{setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone)}},
		"hanako の変更": {user("Hanako"), true, []change.Draft{setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone)}},
		"github":     {change.Actor{Kind: change.ActorGitHub}, true, []change.Draft{setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone)}},
		"sync":       {change.Actor{Kind: change.ActorSync}, true, []change.Draft{setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone)}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			if tc.watch {
				f.watchTask()
			}
			f.judge(f.save(tc.actor, func(tx port.Tx) error {
				if name == "説明" {
					if err := tx.WriteItems().Describe(other, "新しい説明"); err != nil {
						return err
					}
				}
				return tx.WriteItems().Append(other, tc.drafts...)
			}))
			f.wantNone()
		})
	}
}

// 判定の中で作られたアイテムは、前の値がないので作らない。
func TestWatchSkipsCreatedItem(t *testing.T) {
	f := setup(t)
	create := f.save(user("taro"), func(tx port.Tx) error {
		it := item.Item{
			ID: newItem, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3},
			Kind: item.Task, Rank: rank.First, Title: "新しいタスク", Watchers: []string{"taro", "hanako"}, Status: apptest.SampleStatusTodo,
		}
		return tx.WriteItems().Append(newItem, item.Create(it))
	})
	edit := f.save(user("taro"), func(tx port.Tx) error {
		return tx.WriteItems().Append(newItem, setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone))
	})
	f.judge(create, edit)
	f.wantNone()
}

// 別のワークスペースへの移動は、値の定義が変わるので、プロパティの通知を作らない。
func TestWatchSkipsWorkspaceMove(t *testing.T) {
	f := setup(t)
	if err := apptest.SampleMobile(t.Context(), f.stores, store); err != nil {
		t.Fatal(err)
	}
	f.watchTask()
	f.judge(f.save(user("taro"), setTask(
		change.NewSet("place",
			item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Number: 2},
			item.Place{Workspace: apptest.SampleMobileWorkspace, Project: apptest.SampleMobileProject, Parent: ptr(apptest.SampleMobileEpic), Number: 2}),
		setStatus(apptest.SampleStatusTodo, "01SAMPLEMBV000000000000001"),
	)))
	f.wantNone()
}

func TestWatchByAutomation(t *testing.T) {
	f := setup(t)
	f.watchTask()
	f.judge(f.save(user("taro"), setTask(setStatus(apptest.SampleStatusTodo, apptest.SampleStatusDone).ByRule("r1"))))
	if n := f.wantOne(); n.Kind != port.NoticeWatch || n.Text != "ステータスを Done に変更" || n.Sender != "" {
		t.Errorf("notice = %+v, want watch from hedgehug", n)
	}
}

func ptr(id item.ID) *item.ID { return &id }
