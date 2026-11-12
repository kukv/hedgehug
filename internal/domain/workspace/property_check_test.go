package workspace_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// msg は、断り(*refusal.Refusal)の文言を返す。断りでなければ err の文字、nil なら空。
func msg(err error) string {
	if err == nil {
		return ""
	}
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return err.Error()
}

func TestCheckNewProperty(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	s.Properties = append(s.Properties, workspace.Property{ID: "01PROP0000000000000000000A", Key: "customer", Name: "顧客", Type: workspace.TypeSingle})
	cases := []struct{ name, key, want string }{
		{"", "x", "名前を入力してください"},
		{"顧客", "x", "同じ名前のプロパティがあります"},
		{"ステータス", "x", "同じ名前のプロパティがあります"}, // 組み込みとも比べる
		{"環境", "Env", "キーは英小文字で始まる英小文字・数字・_ にしてください"},
		{"環境", "1env", "キーは英小文字で始まる英小文字・数字・_ にしてください"},
		{"環境", "", "キーは英小文字で始まる英小文字・数字・_ にしてください"},
		{"環境", "customer", "このキーは使用できません(もうあるか、予約されています)"},
		{"環境", "systems", "このキーは使用できません(もうあるか、予約されています)"},    // 組み込みのキー
		{"環境", "blocked_by", "このキーは使用できません(もうあるか、予約されています)"}, // 予約語
		{"環境", "env_2", ""},
		{"顧客 ", "x", ""}, // 呼ぶ側が NormalizeName を通す前なので、ここでは別の名前
	}
	for _, c := range cases {
		if got := msg(workspace.CheckNewProperty(s, c.name, c.key)); got != c.want {
			t.Errorf("%q %q: %q, want %q", c.name, c.key, got, c.want)
		}
	}
	if len(workspace.ReservedKeys) != 34 {
		t.Errorf("reserved = %d", len(workspace.ReservedKeys))
	}
}

func TestCheckPropertyNameAndKinds(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	if got := msg(workspace.CheckPropertyName(s, "assignee", "")); got != "名前は空にできません" {
		t.Error(got)
	}
	if got := msg(workspace.CheckPropertyName(s, "assignee", "優先度")); got != "同じ名前のプロパティがあります" {
		t.Error(got)
	}
	if err := workspace.CheckPropertyName(s, "assignee", "担当者"); err != nil { // 自分と同じ名前はよい
		t.Error(err)
	}
	if got := msg(workspace.CheckKinds(nil)); got != "少なくとも 1 つの種類で使ってください" {
		t.Error(got)
	}
	if got := workspace.NormalizeKinds([]string{"Task", "Epic", "Task", "Nope"}); !slices.Equal(got, []string{"Epic", "Task"}) {
		t.Errorf("kinds = %v", got)
	}
}
