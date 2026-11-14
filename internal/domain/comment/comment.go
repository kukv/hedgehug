// Package comment は、アイテムとプロジェクトのコメントの行の決め方を持つ(保存形式「変更記録」の comment)。
package comment

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Comment は、いまのコメント 1 件。By と At は、そのコメントの最初の行の by と at(投稿者と投稿の時刻)。
// Edited は、投稿のあとに編集の行があるとき真。
type Comment struct {
	ID     change.ID
	By     string
	At     time.Time
	Body   string
	Edited bool
}

type value struct {
	Body string `json:"body"`
}

// Value は、コメントの値(行の from・to に書く形)。
func Value(body string) any { return value{Body: body} }

func alive(r change.Record) bool { return !change.Absent(r.To) }

func bodyOf(raw json.RawMessage) (string, bool) {
	var v struct {
		Body *string `json:"body"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Body == nil {
		return "", false
	}
	return *v.Body, true
}

// FromLog は、ログのいまのコメントを、投稿の古い順(同じなら ID の順)で返す。削除されたものと、
// 本文を読めないものは除く。いまの値は change.Current が決める(先端に削除と編集があれば編集を残す)。
func FromLog(log []change.Record) []Comment {
	ids := map[change.ID]bool{}
	for _, r := range log {
		if r.T == change.Comment {
			ids[change.ID(r.F)] = true
		}
	}
	var out []Comment
	for id := range ids {
		if c, ok := Find(log, id); ok {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b Comment) int {
		return cmp.Or(a.At.Compare(b.At), strings.Compare(string(a.ID), string(b.ID)))
	})
	return out
}

// Find は、いまのコメントを 1 件返す。ないか、削除されているか、本文を読めなければ false。
func Find(log []change.Record, id change.ID) (Comment, bool) {
	f := string(id)
	cur, ok := change.Current(log, change.Comment, f, alive)
	if !ok || change.Absent(cur.To) {
		return Comment{}, false
	}
	body, ok := bodyOf(cur.To)
	if !ok {
		return Comment{}, false
	}
	first, _ := change.First(log, change.Comment, f)
	return Comment{ID: id, By: first.By, At: first.At, Body: body, Edited: cur.ID != first.ID}, true
}

// FindDeleted は、いまの値が削除のコメントを、削除の行の from の本文で返す。By と At は最初の行のもの。
// ないか、削除されていないか、本文を読めなければ false。
func FindDeleted(log []change.Record, id change.ID) (Comment, bool) {
	f := string(id)
	cur, ok := change.Current(log, change.Comment, f, alive)
	if !ok || !change.Absent(cur.To) {
		return Comment{}, false
	}
	body, ok := bodyOf(cur.From)
	if !ok {
		return Comment{}, false
	}
	first, _ := change.First(log, change.Comment, f)
	return Comment{ID: id, By: first.By, At: first.At, Body: body}, true
}
