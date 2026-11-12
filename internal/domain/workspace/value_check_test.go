package workspace_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestCheckValueName(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	todo := s.Statuses[0].ID
	cases := []struct {
		id         change.ID
		name, want string
	}{
		{"", "", "名前は空にできません"},
		{"", "todo", "同じ名前の値があります"}, // 大文字と小文字を区別しない
		{"", "Review", ""},
		{todo, "TODO", ""}, // 自分の大文字と小文字だけを変えるのはよい
		{todo, "done", "同じ名前の値があります"},
	}
	for _, c := range cases {
		if got := msg(workspace.CheckValueName(s.Statuses, c.id, c.name)); got != c.want {
			t.Errorf("%q: %q, want %q", c.name, got, c.want)
		}
	}
}

// 作ったばかりのワークスペースで考える。Todo は初期ステータスで未着手の既定、InProgress は進行中の既定、
// Pending はどちらでもない(03「分類ごとの既定のステータス」「初期ステータス」)。
func TestStatusRules(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	todo, doing, pending := s.Statuses[0].ID, s.Statuses[1].ID, s.Statuses[3].ID
	both := "「Todo」は初期ステータスで、未着手の既定のステータスでもあります。先に別のステータスを、初期ステータスと未着手の既定にしてください"
	cases := []struct{ name, got, want string }{
		{"delete todo", msg(workspace.CheckDeleteStatus(s, todo)), both},
		{"delete doing", msg(workspace.CheckDeleteStatus(s, doing)), "「InProgress」は進行中の既定のステータスです。先に別のステータスを既定にしてください"},
		{"delete pending", msg(workspace.CheckDeleteStatus(s, pending)), ""},
		{"todo to done", msg(workspace.CheckStatusCategory(s, todo, workspace.CategoryDone)), both},
		{"todo to doing", msg(workspace.CheckStatusCategory(s, todo, workspace.CategoryDoing)), "「Todo」は未着手の既定のステータスです。先に別のステータスを既定にしてください"},
		{"todo stays", msg(workspace.CheckStatusCategory(s, todo, workspace.CategoryTodo)), ""},
		{"pending to done", msg(workspace.CheckStatusCategory(s, pending, workspace.CategoryDone)), ""},
		{"initial done", msg(workspace.CheckInitialStatus(s, s.Statuses[2].ID)), "完了・中止のステータスは初期ステータスにできません"},
		{"initial pending", msg(workspace.CheckInitialStatus(s, pending)), ""},
	}
	// Pending を初期ステータスにすると、初期ステータスだけに当たる。
	s2 := s
	s2.InitialStatus = pending
	cases = append(cases,
		struct{ name, got, want string }{"initial pending to done", msg(workspace.CheckStatusCategory(s2, pending, workspace.CategoryDone)), "「Pending」は初期ステータスです。完了・中止にするには、先に別のステータスを初期ステータスにしてください"},
		struct{ name, got, want string }{"delete initial pending", msg(workspace.CheckDeleteStatus(s2, pending)), "初期ステータスは削除できません。先に別のステータスを初期ステータスにしてください"},
	)
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: %q, want %q", c.name, c.got, c.want)
		}
	}
	if got := msg(workspace.CheckDeletePriority(s, s.InitialPriority)); got != "初期優先度は削除できません。先に別の優先度を初期優先度にしてください" {
		t.Error(got)
	}
	if err := workspace.CheckDeletePriority(s, s.Priorities[0].ID); err != nil {
		t.Error(err)
	}
}
