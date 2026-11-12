package workspace_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestNormalize(t *testing.T) {
	if got := workspace.NormalizeName("  a \n\t b  "); got != "a b" {
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
		err := workspace.CheckCreate(c.w, others)
		var r *refusal.Refusal
		switch {
		case c.msg == "" && err != nil, c.msg != "" && (!errors.As(err, &r) || r.Message != c.msg):
			t.Errorf("CheckCreate(%+v) = %v, want %q", c.w, err, c.msg)
		}
	}
}

func TestTakenIgnoresSelf(t *testing.T) {
	w := workspace.Workspace{ID: "A", Name: "Web", Prefix: "HH"}
	if workspace.NameTaken(w, []workspace.Workspace{w}) || workspace.PrefixTaken(w, []workspace.Workspace{w}) {
		t.Error("a workspace collided with itself")
	}
}
