package history_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestWatchText(t *testing.T) {
	c := ctx()
	c.Settings.Properties = append(c.Settings.Properties, workspace.Property{
		ID: "PM", Name: "ラベル", Type: workspace.TypeMulti,
		Choices: []workspace.Value{{ID: "A", Name: "a"}, {ID: "B", Name: "b"}},
	})
	cases := []struct {
		name string
		p    workspace.Property
		v    string
		want string
	}{
		{"担当者", workspace.Property{Name: "担当者", Type: workspace.TypeUser}, `"taro"`, "担当者を 太郎 に変更"},
		{"空", workspace.Property{Name: "担当者", Type: workspace.TypeUser}, `null`, "担当者を なし に変更"},
		{"チェック", workspace.Property{Name: "対応済み", Type: workspace.TypeCheck}, `true`, "対応済みを オン に変更"},
		{"複数", workspace.Property{ID: "PM", Name: "ラベル", Type: workspace.TypeMulti}, `["A","B"]`, "ラベルを a, b に変更"},
		{"削除された値", workspace.Property{Name: "ステータス", Type: workspace.TypeStatus}, `"GONE"`, "ステータスを (削除された値) に変更"},
	}
	for _, tc := range cases {
		if got := history.WatchText(tc.p, json.RawMessage(tc.v), c); got != tc.want {
			t.Errorf("%s: WatchText = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCommentHead(t *testing.T) {
	if got := history.CommentHead("短い\nコメント"); got != "コメント: 短い コメント" {
		t.Errorf("CommentHead = %q", got)
	}
	want := "コメント: " + strings.Repeat("あ", 40)
	if got := history.CommentHead(strings.Repeat("あ", 41)); got != want {
		t.Errorf("CommentHead(41 文字) = %q, want %q", got, want)
	}
	if got := history.CommentHead(strings.Repeat("あ", 40)); got != want {
		t.Errorf("CommentHead(40 文字) = %q", got)
	}
}
