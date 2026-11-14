package comment_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
)

func at(minute int) time.Time { return time.Date(2026, 11, 12, 10, minute, 0, 0, time.UTC) }

func row(id, f string, minute int, by string, from, to any, prev ...change.ID) change.Record {
	d := change.NewComment(change.ID(f), from, to)
	return change.Record{ID: change.ID(id), Op: change.ID(id), At: at(minute), By: by, Actor: change.User(by), T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev}
}

func TestFromLogOrdersByPostAndMarksEdited(t *testing.T) {
	log := []change.Record{
		row("k1p", "K1", 1, "hanako", nil, comment.Value("最初"), "c"),
		row("k2p", "K2", 3, "taro", nil, comment.Value("二番目"), "c"),
		row("k1e", "K1", 5, "hanako", comment.Value("最初"), comment.Value("直した"), "k1p"),
	}
	got := comment.FromLog(log)
	want := []comment.Comment{
		{ID: "K1", By: "hanako", At: at(1), Body: "直した", Edited: true},
		{ID: "K2", By: "taro", At: at(3), Body: "二番目"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FromLog = %+v\nwant %+v", got, want)
	}
}

func TestFromLogLeavesOutDeletedAndUnreadable(t *testing.T) {
	log := []change.Record{
		row("k1p", "K1", 1, "hanako", nil, comment.Value("消す"), "c"),
		row("k1d", "K1", 2, "hanako", comment.Value("消す"), nil, "k1p"),
		row("k2p", "K2", 3, "taro", nil, "本文の形が違う", "c"),
		{ID: "t", Op: "t", At: at(4), By: "taro", Actor: change.User("taro"), T: change.Set, F: "title", To: json.RawMessage(`"x"`)},
	}
	if got := comment.FromLog(log); len(got) != 0 {
		t.Errorf("FromLog = %+v, want none", got)
	}
}

func TestFromLogKeepsAnEditAgainstAConcurrentDeleteInAnyOrder(t *testing.T) {
	log := []change.Record{
		row("p", "K1", 1, "hanako", nil, comment.Value("a"), "c"),
		row("e", "K1", 2, "taro", comment.Value("a"), comment.Value("b"), "p"),
		row("d", "K1", 9, "hanako", comment.Value("a"), nil, "p"),
	}
	got := comment.FromLog(log)
	if len(got) != 1 || got[0].Body != "b" || !got[0].Edited || got[0].By != "hanako" {
		t.Fatalf("FromLog = %+v", got)
	}
	rev := slices.Clone(log)
	slices.Reverse(rev)
	if !reflect.DeepEqual(comment.FromLog(rev), got) {
		t.Errorf("the reversed log gave %+v", comment.FromLog(rev))
	}
}

func TestFind(t *testing.T) {
	log := []change.Record{row("p", "K1", 1, "hanako", nil, comment.Value("a"), "c")}
	if c, ok := comment.Find(log, "K1"); !ok || c.Body != "a" {
		t.Errorf("Find K1 = %+v, %v", c, ok)
	}
	if _, ok := comment.Find(log, "K9"); ok {
		t.Error("Find of an unknown comment")
	}
}

func TestFindDeleted(t *testing.T) {
	posted := row("k1p", "K1", 1, "hanako", nil, comment.Value("消す前"), "c")
	deleted := row("k1d", "K1", 2, "hanako", comment.Value("消す前"), nil, "k1p")
	got, ok := comment.FindDeleted([]change.Record{posted, deleted}, "K1")
	want := comment.Comment{ID: "K1", By: "hanako", At: at(1), Body: "消す前"}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("FindDeleted = %+v, %v\nwant %+v", got, ok, want)
	}
	if _, ok := comment.FindDeleted([]change.Record{posted}, "K1"); ok {
		t.Error("削除されていないコメントが見つかった")
	}
	if _, ok := comment.FindDeleted([]change.Record{posted, deleted}, "K9"); ok {
		t.Error("ないコメントが見つかった")
	}
	edited := row("k1e", "K1", 3, "taro", comment.Value("消す前"), comment.Value("編集"), "k1p")
	log := []change.Record{posted, deleted, edited}
	if _, ok := comment.Find(log, "K1"); !ok {
		t.Error("削除と並んだ編集の行があるのに Find で見つからない")
	}
	if _, ok := comment.FindDeleted(log, "K1"); ok {
		t.Error("削除と編集が並んでいるのに FindDeleted が見つけた")
	}
}
