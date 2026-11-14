package noticing_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

const commentID change.ID = "01COMMENT00000000000000001"

// writeComment は、サンプルのタスク HH-2 のコメント commentID を from から to にする(from が空なら投稿)。
func writeComment(from, to string) func(tx port.Tx) error {
	var old any
	if from != "" {
		old = comment.Value(from)
	}
	return setTask(change.NewComment(commentID, old, comment.Value(to)))
}

func TestMentionInComment(t *testing.T) {
	f := setup(t)
	s := f.save(user("taro"), writeComment("", "@hanako 見てください"))
	f.judge(s)
	n := f.wantOne()
	want := port.Notice{
		Kind: port.NoticeMention, Store: store, Workspace: apptest.SampleWorkspace, Item: apptest.SampleTask,
		Text: "@hanako 見てください", Display: "HH-2", Sender: "taro", Source: "comment:" + string(commentID), At: s.Key.At,
	}
	if n != want {
		t.Errorf("notice = %+v, want %+v", n, want)
	}
}

func TestMentionInDescription(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("taro"), func(tx port.Tx) error {
		if err := tx.WriteItems().Describe(apptest.SampleTask, "説明 @hanako"); err != nil {
			return err
		}
		return tx.WriteItems().Append(apptest.SampleTask, change.NewDesc())
	}))
	n := f.wantOne()
	if n.Kind != port.NoticeMention || n.Source != "item:"+string(apptest.SampleTask)+":desc" || n.Item != apptest.SampleTask ||
		n.Text != "説明 @hanako" || n.Sender != "taro" {
		t.Errorf("notice = %+v", n)
	}

	g := setup(t)
	g.judge(g.save(user("taro"), func(tx port.Tx) error {
		if err := tx.WriteProjects().Describe(apptest.SampleProject, "@hanako プロジェクトの説明"); err != nil {
			return err
		}
		return tx.WriteProjects().Append(apptest.SampleProject, change.NewDesc())
	}))
	n = g.wantOne()
	if n.Kind != port.NoticeMention || n.Source != "project:"+string(apptest.SampleProject)+":desc" ||
		n.Project != apptest.SampleProject || n.Item != "" || n.Display != "HH-P1" || n.Sender != "taro" {
		t.Errorf("notice = %+v", n)
	}
}

func TestMentionRewrite(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("taro"), writeComment("", "@hanako 見てください")))
	first := f.rows()
	if len(first) != 1 {
		t.Fatalf("rows = %+v, want one", first)
	}
	if err := f.inbox.SetRead(t.Context(), first[0].ID, true); err != nil {
		t.Fatal(err)
	}
	second := f.save(user("jiro"), writeComment("@hanako 見てください", "@hanako 見てください(追記)"))
	f.judge(second)
	rows := f.rows()
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one", rows)
	}
	r := rows[0]
	if r.Text != "@hanako 見てください(追記)" || r.Sender != "jiro" || !r.At.Equal(second.Key.At) || r.Read {
		t.Errorf("row = %+v, want rewritten and unread at %v", r, second.Key.At)
	}

	// 本文に @hanako がなくなった編集では書き換えない
	f.judge(f.save(user("taro"), writeComment("@hanako 見てください(追記)", "解決しました")))
	if rows := f.rows(); len(rows) != 1 || rows[0].Text != "@hanako 見てください(追記)" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestMentionInDescriptionOnCreate(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("taro"), createItem("", "作成時の説明 @hanako")))
	n := f.wantOne()
	if n.Kind != port.NoticeMention || n.Sender != "taro" || n.Source != "item:"+string(newItem)+":desc" || n.Display != "HH-3" {
		t.Errorf("notice = %+v", n)
	}
}

func TestMovedInSkipped(t *testing.T) {
	f := setup(t)
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	it := item.Item{
		ID: newItem, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3},
		Kind: item.Task, Rank: rank.First, Title: "移ってきたタスク", Watchers: []string{"hanako"}, Assignee: "hanako",
	}
	create := item.Create(it)
	com := change.NewComment(commentID, nil, comment.Value("@hanako 移動元のコメント"))
	log := []change.Record{
		{ID: "01OLDROW000000000000000001", Op: "01OLDOP0000000000000000001", At: at, By: "taro", Actor: user("taro"), T: create.T, To: create.To},
		{ID: "01OLDROW000000000000000002", Op: "01OLDOP0000000000000000002", At: at.Add(time.Minute), By: "taro", Actor: user("taro"), T: com.T, F: com.F, From: com.From, To: com.To, Prev: []change.ID{"01OLDROW000000000000000001"}},
	}
	f.judge(f.save(user("taro"), func(tx port.Tx) error {
		if err := tx.WriteItems().Import(newItem, log, "移動元の説明 @hanako"); err != nil {
			return err
		}
		return tx.WriteItems().Append(newItem, transfer.NewMovedIn(transfer.MovedInInfo{
			Old: "MB-9", FromStore: "S2", FromStoreName: "mobile", FromWorkspace: apptest.SampleMobileWorkspace, At: at,
		}))
	}))
	f.wantNone()
}

func TestMentionOwnOrNonMember(t *testing.T) {
	f := setup(t)
	f.judge(f.save(user("hanako"), writeComment("", "@hanako 自分へのメモ")))
	f.wantNone()

	g := setup(t)
	g.save(user("taro"), func(tx port.Tx) error {
		return tx.WriteMembers().Append(change.NewRemove("members", "hanako"))
	})
	g.judge(g.save(user("taro"), writeComment("", "@hanako 見てください")))
	g.wantNone()
}
