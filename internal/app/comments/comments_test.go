package comments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/comments"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

var (
	onTask    = comments.Target{Kind: change.KindItem, ID: apptest.SampleTask}
	onProject = comments.Target{Kind: change.KindProject, ID: apptest.SampleProject}
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T, login string) (comments.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(t.Context(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return comments.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: login}}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func logLen(t *testing.T, s port.Stores, tg comments.Target) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: tg.Kind, ID: tg.ID})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPostThenList(t *testing.T) {
	svc, rec := setup(t, "hanako")
	v, err := svc.Post(t.Context(), store, onTask, "  @taro 見てください\n")
	if err != nil {
		t.Fatal(err)
	}
	if v.Body != "@taro 見てください" || v.By != "hanako" || v.ByName != "hanako" || v.Edited || v.ID == "" {
		t.Errorf("posted %+v", v)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-2 にコメント" {
		t.Errorf("summary = %q", got)
	}
	list, err := svc.List(t.Context(), store, onTask)
	if err != nil || len(list) != 1 || list[0].ID != v.ID {
		t.Errorf("List = %+v, %v", list, err)
	}
	if other, _ := svc.List(t.Context(), store, onProject); len(other) != 0 {
		t.Errorf("a comment on the item showed up on the project: %+v", other)
	}
}

func TestPostOnAProject(t *testing.T) {
	svc, rec := setup(t, "taro")
	if _, err := svc.Post(t.Context(), store, onProject, "概要へ"); err != nil {
		t.Fatal(err)
	}
	if got := rec.summaries[len(rec.summaries)-1]; got != "HH-P1 にコメント" {
		t.Errorf("summary = %q", got)
	}
	if list, _ := svc.List(t.Context(), store, onProject); len(list) != 1 || list[0].Body != "概要へ" {
		t.Errorf("List = %+v", list)
	}
}

func TestPostAnEmptyBodyIsRefusedAndSavesNothing(t *testing.T) {
	svc, rec := setup(t, "hanako")
	before := logLen(t, rec, onTask)
	if _, err := svc.Post(t.Context(), store, onTask, " \n "); message(err) != "コメントを入力してください" {
		t.Errorf("err = %v", err)
	}
	if logLen(t, rec, onTask) != before {
		t.Error("a refused post wrote a row")
	}
}

func TestAuthorMissingFromMembersIsAnUnknownUser(t *testing.T) {
	svc, _ := setup(t, "ghost")
	v, err := svc.Post(t.Context(), store, onTask, "こんにちは")
	if err != nil || v.By != "ghost" || v.ByName != "不明なユーザー" {
		t.Errorf("posted %+v, %v", v, err)
	}
}

func TestEdit(t *testing.T) {
	svc, rec := setup(t, "hanako")
	v, _ := svc.Post(t.Context(), store, onTask, "前")
	before := logLen(t, rec, onTask)
	same, err := svc.Edit(t.Context(), store, onTask, v.ID, " 前 ")
	if err != nil || same.Edited || logLen(t, rec, onTask) != before {
		t.Errorf("an unchanged edit: %+v, %v; log %d → %d", same, err, before, logLen(t, rec, onTask))
	}
	got, err := svc.Edit(t.Context(), store, onTask, v.ID, "後")
	if err != nil || got.Body != "後" || !got.Edited || got.By != "hanako" {
		t.Errorf("edited %+v, %v", got, err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "コメントを編集" {
		t.Errorf("summary = %q", last)
	}
	if _, err := svc.Edit(t.Context(), store, onTask, v.ID, "  "); message(err) != "コメントを入力してください" {
		t.Errorf("an empty edit: %v", err)
	}
	if _, err := svc.Edit(t.Context(), store, onTask, "01NOPE00000000000000000000", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("an unknown comment: %v", err)
	}
}

func TestDelete(t *testing.T) {
	svc, rec := setup(t, "hanako")
	v, _ := svc.Post(t.Context(), store, onTask, "消す")
	if err := svc.Delete(t.Context(), store, onTask, v.ID); err != nil {
		t.Fatal(err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "コメントを削除" {
		t.Errorf("summary = %q", last)
	}
	if list, _ := svc.List(t.Context(), store, onTask); len(list) != 0 {
		t.Errorf("List after delete = %+v", list)
	}
	if err := svc.Delete(t.Context(), store, onTask, v.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("deleting twice: %v", err)
	}
	if _, err := svc.Edit(t.Context(), store, onTask, v.ID, "消す"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("editing a deleted comment to the same body: %v", err)
	}
}

func TestOnlyTheAuthorCanEditOrDelete(t *testing.T) {
	svc, _ := setup(t, "hanako")
	v, err := svc.Post(t.Context(), store, onTask, "自分の")
	if err != nil {
		t.Fatal(err)
	}
	other := svc
	other.Identity = apptest.Identity{Login: "taro"}
	if _, err := other.Edit(t.Context(), store, onTask, v.ID, "乗っ取り"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("another person's edit: %v", err)
	}
	if err := other.Delete(t.Context(), store, onTask, v.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("another person's delete: %v", err)
	}
	if list, _ := svc.List(t.Context(), store, onTask); len(list) != 1 || list[0].Body != "自分の" || list[0].Edited {
		t.Errorf("a refused edit or delete changed the comment: %+v", list)
	}
	// 同じ人なら、ログイン名の大文字と小文字が違っても自分のコメント。
	same := svc
	same.Identity = apptest.Identity{Login: "HANAKO"}
	if got, err := same.Edit(t.Context(), store, onTask, v.ID, "大文字でも自分"); err != nil || got.Body != "大文字でも自分" {
		t.Errorf("own edit with another case: %+v, %v", got, err)
	}
	if err := same.Delete(t.Context(), store, onTask, v.ID); err != nil {
		t.Errorf("own delete with another case: %v", err)
	}
}

func TestUnknownOwner(t *testing.T) {
	svc, _ := setup(t, "hanako")
	gone := comments.Target{Kind: change.KindItem, ID: "01NOPE00000000000000000000"}
	if _, err := svc.List(t.Context(), store, gone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List: %v", err)
	}
	if _, err := svc.Post(t.Context(), store, gone, "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Post: %v", err)
	}
	// 種類と ID が合わない(プロジェクトの ID をアイテムとして指す)ときも、ないものとして扱う。
	wrong := comments.Target{Kind: change.KindItem, ID: apptest.SampleProject}
	if _, err := svc.Post(t.Context(), store, wrong, "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Post with a mismatched kind: %v", err)
	}
}
