package noticing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// assign は、サンプルのタスク HH-2 の担当者を login にする行。
func assign(login string) change.Draft { return change.NewSet("assignee", nil, login) }

func setTask(drafts ...change.Draft) func(tx port.Tx) error {
	return func(tx port.Tx) error { return tx.WriteItems().Append(apptest.SampleTask, drafts...) }
}

func TestAssignToMe(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("taro"), setTask(assign("hanako"))))
	n := f.wantOne()
	want := port.Notice{
		Kind: port.NoticeAssign, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask,
		Text: "あなたに割り当てました", Display: "HH-2", Sender: "taro",
	}
	if n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestAssignOnlyWhenAfterIsMe(t *testing.T) {
	f := setup(t)
	a := f.save(user("taro"), setTask(assign("hanako")))
	b := f.save(user("taro"), setTask(assign("jiro")))
	f.judge(a, b)
	f.wantNone()

	// 前から hanako のアイテムのタイトルだけを変えた保存
	f.save(user("taro"), setTask(assign("hanako")))
	f.judge(f.save(user("taro"), setTask(change.NewSet("title", "ログイン画面を作る", "ログイン画面"))))
	f.wantNone()
}

func TestAssignByOwnChange(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("Hanako"), setTask(assign("hanako"))))
	f.wantNone()
}

func TestAssignByAutomation(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("taro"), setTask(assign("hanako").ByRule("r1"))))
	n := f.wantOne()
	if n.Kind != port.NoticeAssign || n.Sender != "" {
		t.Errorf("notice = %+v, want assign from hedgehug", n)
	}

	g := setup(t)
	g.judge(g.save(user("hanako"), setTask(assign("hanako").ByRule("r1"))))
	g.wantNone()
}

func TestAssignSkipsGithubAndSync(t *testing.T) {
	for _, kind := range []change.ActorKind{change.ActorGitHub, change.ActorSync} {
		t.Run(string(kind), func(t *testing.T) {
			f := setup(t)
			f.judge(f.save(change.Actor{Kind: kind}, setTask(assign("hanako"))))
			f.wantNone()
		})
	}
}

const newItem item.ID = "01NEWITEM00000000000000001"

// createItem は、サンプルのプロジェクトに番号 3 のタスクを作る。説明が空でなければ本文だけを書く(desc の行は書かない)。
func createItem(assignee, desc string) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		it := item.Item{
			ID: newItem, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3},
			Kind: item.Task, Rank: rank.First, Title: "新しいタスク", Watchers: []string{"taro"},
			Status: apptest.SampleStatusTodo, Assignee: assignee,
		}
		if err := tx.WriteItems().Append(newItem, item.Create(it)); err != nil {
			return err
		}
		if desc == "" {
			return nil
		}
		return tx.WriteItems().Describe(newItem, desc)
	}
}

func TestAssignOnCreate(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("taro"), createItem("hanako", "")))
	n := f.wantOne()
	if n.Kind != port.NoticeAssign || n.Sender != "taro" || n.Item != newItem || n.Display != "HH-3" {
		t.Errorf("notice = %+v", n)
	}
}

func TestOwnerToMe(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("taro"), func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject, change.NewSet("owner", "taro", "hanako"))
	}))
	n := f.wantOne()
	want := port.Notice{
		Kind: port.NoticeAssign, Store: store, Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject,
		Text: "プロジェクト「ログイン」のオーナーになりました", Display: "HH-P1", Sender: "taro",
	}
	if n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestNoIdentity(t *testing.T) {
	f := setup(t)
	f.svc.Identity = noIdentity{}
	s := f.save(user("taro"), setTask(assign("hanako"), change.NewComment("01C1", nil, map[string]string{"body": "@hanako"})))
	if err := f.judgeErr(s); err != nil {
		t.Fatalf("Judge = %v, want nil", err)
	}
	f.wantNone()
}

func TestSkipsMissingEntity(t *testing.T) {
	f := setup(t)
	s := f.save(user("taro"), setTask(assign("hanako"), change.NewComment("01C1", nil, map[string]string{"body": "@hanako"})))
	for i := range s.Entities {
		s.Entities[i].Ref.ID = "01MISSING00000000000000001"
	}
	if err := f.judgeErr(s); err != nil {
		t.Fatalf("Judge = %v, want nil", err)
	}
	f.wantNone()
}
