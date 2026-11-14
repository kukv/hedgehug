package workspace_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestNormalize(t *testing.T) {
	if got := workspace.NormalizeName("  a \n\t b  "); got != "a \n\t b" {
		t.Errorf("NormalizeName = %q", got)
	}
	if got := workspace.NormalizePrefix(" hh "); got != "HH" {
		t.Errorf("NormalizePrefix = %q", got)
	}
}

func TestCheckCreate(t *testing.T) {
	others := []workspace.Workspace{{ID: "A", Name: "Web", Prefix: "HH"}}
	cases := []struct {
		w   workspace.Workspace
		msg string
	}{
		{workspace.Workspace{ID: "N", Name: "", Prefix: "API"}, "名前を入力してください"},
		{workspace.Workspace{ID: "N", Name: "Web", Prefix: ""}, "この Store には同じ名前のワークスペースがあります"},
		{workspace.Workspace{ID: "N", Name: "API", Prefix: ""}, "接頭辞を入力してください"},
		{workspace.Workspace{ID: "N", Name: "API", Prefix: "1API"}, "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"},
		{workspace.Workspace{ID: "N", Name: "API", Prefix: "HH"}, "この Store には同じ接頭辞のワークスペースがあります"},
		{workspace.Workspace{ID: "N", Name: "web", Prefix: "WEB"}, ""},
	}
	for _, c := range cases {
		t.Run(c.w.Name+"/"+c.w.Prefix, func(t *testing.T) {
			if got := messageOf(workspace.CheckCreate(c.w, others)); got != c.msg {
				t.Errorf("message = %q, want %q", got, c.msg)
			}
		})
	}
}

func TestCheckRename(t *testing.T) {
	others := []workspace.Workspace{{ID: "A", Name: "Web", Prefix: "HH"}, {ID: "B", Name: "API", Prefix: "API"}}
	cases := []struct {
		name, msg string
	}{
		{"", "名前は空にできません"},
		{"API", "この Store には同じ名前のワークスペースがあります"},
		{"Web", ""}, // 自分と同じ名前は重複ではない
		{"web", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := workspace.Workspace{ID: "A", Name: c.name, Prefix: "HH"}
			if got := messageOf(workspace.CheckRename(w, others)); got != c.msg {
				t.Errorf("message = %q, want %q", got, c.msg)
			}
		})
	}
}

func TestCheckPrefix(t *testing.T) {
	others := []workspace.Workspace{{ID: "A", Name: "Web", Prefix: "HH"}, {ID: "B", Name: "API", Prefix: "API"}}
	cases := []struct {
		prefix, msg string
	}{
		{"", "接頭辞を入力してください"},
		{"W-B", "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"},
		{"API", "この Store には同じ接頭辞のワークスペースがあります"},
		{"HH", ""}, // 自分と同じ接頭辞は重複ではない
		{"WEB", ""},
	}
	for _, c := range cases {
		t.Run(c.prefix, func(t *testing.T) {
			w := workspace.Workspace{ID: "A", Name: "Web", Prefix: c.prefix}
			if got := messageOf(workspace.CheckPrefix(w, others)); got != c.msg {
				t.Errorf("message = %q, want %q", got, c.msg)
			}
		})
	}
}

// messageOf は、断りの文言を返す。断りでなければ err の文言、nil なら空文字。
func messageOf(err error) string {
	var r *refusal.Refusal
	switch {
	case err == nil:
		return ""
	case errors.As(err, &r):
		return r.Message
	}
	return err.Error()
}

func TestTakenIgnoresSelf(t *testing.T) {
	w := workspace.Workspace{ID: "A", Name: "Web", Prefix: "HH"}
	if workspace.NameTaken(w, []workspace.Workspace{w}) || workspace.PrefixTaken(w, []workspace.Workspace{w}) {
		t.Error("a workspace collided with itself")
	}
}

func TestCheckRevive(t *testing.T) {
	others := []workspace.Workspace{{ID: "A", Name: "Web", Prefix: "HH"}}
	cases := []struct {
		w   workspace.Workspace
		msg string
	}{
		{workspace.Workspace{ID: "N", Name: "Web", Prefix: "WB"}, "同じ名前のワークスペースがあります。先にそのワークスペースの名前を変更してください"},
		{workspace.Workspace{ID: "N", Name: "API", Prefix: "HH"}, "同じ接頭辞のワークスペースがあります。先にそのワークスペースの接頭辞を変更してください"},
		{workspace.Workspace{ID: "N", Name: "web", Prefix: "WB"}, ""},
		{workspace.Workspace{ID: "A", Name: "Web", Prefix: "HH"}, ""}, // 自分自身とは比べない
	}
	for _, c := range cases {
		t.Run(c.w.Name+"/"+c.w.Prefix, func(t *testing.T) {
			if got := messageOf(workspace.CheckRevive(c.w, others)); got != c.msg {
				t.Errorf("message = %q, want %q", got, c.msg)
			}
		})
	}
}
