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

// 02「概要の編集」: タイトルは、空だと「タイトルは空にできません」(作成の文言とは違う)。
// 同じワークスペースの別のプロジェクトと同じ名前は断り、自分と同じ名前は断らない。
func TestCheckRename(t *testing.T) {
	siblings := []project.Project{{ID: "A", Title: "ログイン"}, {ID: "B", Title: "決済"}}
	cases := []struct {
		name, title string
		self        project.ID
		want        string
	}{
		{"ok", "請求", "A", ""},
		{"empty", "", "A", "タイトルは空にできません"},
		{"duplicate of another", "決済", "A", "このワークスペースには同じ名前のプロジェクトがあります"},
		{"the same title as itself", "ログイン", "A", ""},
	}
	for _, c := range cases {
		if got := message(project.CheckRename(c.title, c.self, siblings)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// 08「復元」: 同じワークスペースの削除されていないプロジェクトに同じ名前があれば断る。比べ方は作成・名前の変更と同じ(完全一致)。
func TestCheckRevive(t *testing.T) {
	const dup = "同じ名前のプロジェクトがあります。先にそのプロジェクトの名前を変更してください"
	siblings := []project.Project{{ID: "A", Title: "ログイン"}, {ID: "B", Title: "Login"}}
	cases := []struct {
		name     string
		title    string
		siblings []project.Project
		want     string
	}{
		{"same name", "ログイン", siblings, dup},
		{"another same name", "Login", siblings, dup},
		{"different name", "決済", siblings, ""},
		{"no siblings", "ログイン", nil, ""},
		{"empty siblings", "ログイン", []project.Project{}, ""},
		{"case differs", "login", siblings, ""},
		{"extra space differs", "ログイン ", siblings, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := message(project.CheckRevive(c.title, c.siblings)); got != c.want {
				t.Errorf("CheckRevive(%q) = %q, want %q", c.title, got, c.want)
			}
		})
	}
}

// 開始日と終了日が同じ日は認める。どちらかが空なら比べない。
func TestCheckDates(t *testing.T) {
	cases := []struct{ name, start, end, want string }{
		{"same day", "2026-11-01", "2026-11-01", ""},
		{"only start", "2026-11-01", "", ""},
		{"only end", "", "2026-11-01", ""},
		{"end before start", "2026-11-02", "2026-11-01", "終了日は開始日以降にしてください"},
	}
	for _, c := range cases {
		if got := message(project.CheckDates(c.start, c.end)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
