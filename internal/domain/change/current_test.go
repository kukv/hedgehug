package change_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func note(id string, minute int, by string, from, to any, prev ...change.ID) change.Record {
	return rec(id, at(minute), by, change.NewComment("K1", from, to), prev...)
}

func text(s string) map[string]string { return map[string]string{"body": s} }

func alive(r change.Record) bool { return !change.Absent(r.To) }

func current(log []change.Record) (change.Record, bool) {
	return change.Current(log, change.Comment, "K1", alive)
}

func TestNewCommentWritesNullForAddAndDelete(t *testing.T) {
	post := change.NewComment("K1", nil, text("a"))
	del := change.NewComment("K1", text("a"), nil)
	if post.T != change.Comment || post.F != "K1" || string(post.From) != "null" || string(post.To) != `{"body":"a"}` {
		t.Errorf("post = %+v", post)
	}
	if string(del.To) != "null" || !change.Absent(del.To) || change.Absent(post.To) || !change.Absent(nil) {
		t.Errorf("delete = %+v", del)
	}
}

func TestCurrentFollowsTheEditChain(t *testing.T) {
	log := []change.Record{
		create(map[string]any{"title": "a"}),
		note("p", 1, "hanako", nil, text("a"), "c"),
		note("e", 2, "hanako", text("a"), text("b"), "p"),
	}
	if got, ok := current(log); !ok || got.ID != "e" {
		t.Errorf("Current = %v, %v; want e", got.ID, ok)
	}
	if _, ok := change.Current(log, change.Comment, "K2", alive); ok {
		t.Error("a comment with no rows must have no value")
	}
}

func TestCurrentDeleteThatSawTheEditWins(t *testing.T) {
	log := []change.Record{
		note("p", 1, "hanako", nil, text("a"), "c"),
		note("e", 2, "hanako", text("a"), text("b"), "p"),
		note("d", 3, "taro", text("b"), nil, "e"),
	}
	got, ok := current(log)
	if !ok || got.ID != "d" || !change.Absent(got.To) {
		t.Errorf("Current = %v, %v; want the delete d", got.ID, ok)
	}
}

func TestCurrentKeepsAnEditAgainstAConcurrentDelete(t *testing.T) {
	// 同じ行 p を見て、一方が編集し(a・b)、他方が削除した(d。時計は最もあと)。編集を残し、同時の編集は時計が遅いほう。
	log := []change.Record{
		note("p", 1, "hanako", nil, text("a"), "c"),
		note("a", 2, "hanako", text("a"), text("A"), "p"),
		note("b", 3, "taro", text("a"), text("B"), "p"),
		note("d", 9, "taro", text("a"), nil, "p"),
	}
	want, ok := current(log)
	if !ok || want.ID != "b" {
		t.Fatalf("Current = %v, %v; want b", want.ID, ok)
	}
	rev := slices.Clone(log)
	slices.Reverse(rev)
	if got, _ := current(rev); got.ID != want.ID {
		t.Errorf("Current of the reversed log = %v, want %v", got.ID, want.ID)
	}
}

func TestFirstIsTheRowThatDoesNotFollowAnother(t *testing.T) {
	// 編集の行の時計が投稿より前にずれていても、最初の行は prev で決める。
	log := []change.Record{
		note("e", 1, "taro", text("a"), text("b"), "p"),
		note("p", 5, "hanako", nil, text("a"), "c"),
	}
	got, ok := change.First(log, change.Comment, "K1")
	if !ok || got.ID != "p" || got.By != "hanako" {
		t.Errorf("First = %v, %v; want p by hanako", got.ID, ok)
	}
	if _, ok := change.First(log, change.Comment, "K2"); ok {
		t.Error("First of a comment with no rows")
	}
}
