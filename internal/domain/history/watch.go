package history

import (
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/workspace"
)

var newlines = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

// WatchText は、ウォッチの通知の文「{プロパティ名}を {値} に変更」。値は履歴と同じ表示。
func WatchText(p workspace.Property, v json.RawMessage, c Context) string {
	return p.Name + "を " + vocab{settings: c.Settings}.display(p, v, c) + " に変更"
}

// CommentHead は、コメントの通知の文「コメント: {先頭 40 字}」。改行は空白 1 つにする。
func CommentHead(body string) string {
	s := newlines.Replace(body)
	if r := []rune(s); len(r) > 40 {
		s = string(r[:40])
	}
	return "コメント: " + s
}
