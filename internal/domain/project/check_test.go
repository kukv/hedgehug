package project_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

// 02「プロジェクト」の「作成」の表。名前は前後の空白を除き、完全に一致するものだけを重複とする。
func TestCheckCreate(t *testing.T) {
	siblings := []project.Project{{Title: "ログイン"}}
	cases := []struct {
		name, title, start, end, want string
	}{
		{"ok", "決済", "", "", ""},
		{"same day", "決済", "2026-11-01", "2026-11-01", ""},
		{"only start", "決済", "2026-11-01", "", ""},
		{"empty", "  ", "", "", "タイトルを入力してください"},
		{"end before start", "決済", "2026-11-02", "2026-11-01", "終了日は開始日以降にしてください"},
		{"duplicate after trim", " ログイン ", "", "", "このワークスペースには同じ名前のプロジェクトがあります"},
		{"case differs", "ログイン画面", "", "", ""},
	}
	for _, c := range cases {
		if got := message(project.CheckCreate(c.title, c.start, c.end, siblings)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if got := message(project.CheckCreate("Login", "", "", []project.Project{{Title: "login"}})); got != "" {
		t.Errorf("upper and lower case must differ: %q", got)
	}
}
