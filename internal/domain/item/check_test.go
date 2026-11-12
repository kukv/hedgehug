package item_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	if err != nil {
		return "unexpected: " + err.Error()
	}
	return ""
}

// 02「入力の検査」: 連続する空白と改行を 1 つの空白にまとめ、前後の空白を取り除く。
func TestNormalizeTitle(t *testing.T) {
	for in, want := range map[string]string{
		"ログイン画面": "ログイン画面", "  ログイン\n\n 画面  ": "ログイン 画面", " \t\n ": "", "a　　b": "a b",
	} {
		if got := item.NormalizeTitle(in); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// 02「入力の検査」の表を上から順に検査し、最初の 1 つだけを返す。
func TestCheckCreate(t *testing.T) {
	epic, task := item.Epic, item.Task
	cases := []struct {
		name    string
		kind    item.Kind
		title   string
		project bool
		parent  *item.Kind
		want    string
	}{
		{"ok epic", item.Epic, "認証", true, nil, ""},
		{"ok task", item.Task, "画面", true, &epic, ""},
		{"ok step", item.Step, "下書き", true, &task, ""},
		{"empty title comes first", item.Task, "  \n ", false, nil, "タイトルを入力してください"},
		{"no project", item.Task, "画面", false, nil, "プロジェクトを選んでください"},
		{"step without parent", item.Step, "下書き", true, nil, "親になる Story・Task・Bug を選んでください"},
		{"task without parent", item.Task, "画面", true, nil, "親になる Epic を選んでください"},
		{"step under epic", item.Step, "下書き", true, &epic, "Step は Epic の下に置けません"},
		{"epic under task", item.Epic, "認証", true, &task, "Epic は Task の下に置けません"},
	}
	for _, c := range cases {
		if got := message(item.CheckCreate(c.kind, c.title, c.project, c.parent)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPlaceMessageAtProjectRoot(t *testing.T) {
	if got := item.PlaceMessage(item.Task, nil); got != "プロジェクトの直下には Epic だけを置けます" {
		t.Errorf("PlaceMessage = %q", got)
	}
}
