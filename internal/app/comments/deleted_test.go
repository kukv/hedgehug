package comments_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/comments"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// markDeleted は、持ち主を削除済みにする(行を直接書く。root: true は利用者が選んだ削除)。
func markDeleted(t *testing.T, s port.Stores, tg comments.Target, root bool) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "削除"}
	if err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		if tg.Kind == change.KindProject {
			return tx.WriteProjects().Append(tg.ID, change.NewDeleteEntity(root))
		}
		return tx.WriteItems().Append(tg.ID, change.NewDeleteEntity(root))
	}); err != nil {
		t.Fatal(err)
	}
}

// hasRestore は、持ち主のログに restore の行があるか。
func hasRestore(t *testing.T, s port.Stores, tg comments.Target) bool {
	t.Helper()
	found := false
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: tg.Kind, ID: tg.ID})
		found = slices.ContainsFunc(log, func(rec change.Record) bool { return rec.T == change.Restore })
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// owners は、コメントの持ち主(Task と、プロジェクト)と、その表示 ID。
var owners = []struct {
	name    string
	target  comments.Target
	display string
}{
	{"Task", onTask, "HH-2"},
	{"プロジェクト", onProject, "HH-P1"},
}

// wantRevived は、display の持ち主の削除が取り消され(Revived に積まれ、restore の行がある)たことを確かめる。
func wantRevived(ctx context.Context, t *testing.T, s port.Stores, tg comments.Target, display string) {
	t.Helper()
	if r := port.Revived(ctx); !slices.Equal(r, []port.Revival{{Kind: port.RevivedID, Text: display}}) {
		t.Errorf("Revived = %v", r)
	}
	if !hasRestore(t, s, tg) {
		t.Error("restore の行がない")
	}
}

// 別のタブで削除された(root: true)持ち主へコメントを投稿すると、コメントを残して削除を取り消す。
func TestPostCommentDeletedRevives(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, rec := setup(t, "hanako")
			markDeleted(t, rec, o.target, true)
			ctx := port.WithRevived(t.Context())
			v, err := svc.Post(ctx, store, o.target, "戻して投稿")
			if err != nil || v.Body != "戻して投稿" {
				t.Fatalf("Post = %+v, %v", v, err)
			}
			wantRevived(ctx, t, rec, o.target, o.display)
			if _, err := svc.List(ctx, store, o.target); err != nil {
				t.Errorf("List = %v", err)
			}
		})
	}
}

// 削除済みの持ち主の、自分のコメントを編集すると、削除を取り消す。本文が同じなら取り消さず断る。
func TestEditCommentDeletedRevives(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, rec := setup(t, "hanako")
			posted, err := svc.Post(t.Context(), store, o.target, "前")
			if err != nil {
				t.Fatal(err)
			}
			markDeleted(t, rec, o.target, true)
			same := port.WithRevived(t.Context())
			if _, err := svc.Edit(same, store, o.target, posted.ID, "前"); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("同じ本文の Edit = %v, want port.ErrNotFound", err)
			}
			if hasRestore(t, rec, o.target) {
				t.Error("同じ本文の編集で削除を取り消した")
			}
			ctx := port.WithRevived(t.Context())
			v, err := svc.Edit(ctx, store, o.target, posted.ID, "後")
			if err != nil || v.Body != "後" || !v.Edited {
				t.Fatalf("Edit = %+v, %v", v, err)
			}
			wantRevived(ctx, t, rec, o.target, o.display)
		})
	}
}

// 削除済みの持ち主の、自分のコメントを削除すると、削除を取り消す。
func TestDeleteCommentDeletedRevives(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, rec := setup(t, "hanako")
			posted, err := svc.Post(t.Context(), store, o.target, "消す")
			if err != nil {
				t.Fatal(err)
			}
			markDeleted(t, rec, o.target, true)
			ctx := port.WithRevived(t.Context())
			if err := svc.Delete(ctx, store, o.target, posted.ID); err != nil {
				t.Fatal(err)
			}
			wantRevived(ctx, t, rec, o.target, o.display)
			list, err := svc.List(ctx, store, o.target)
			if err != nil || len(list) != 0 {
				t.Errorf("List = %+v, %v", list, err)
			}
		})
	}
}

// 親と一緒に削除された(root: false)持ち主のコメントは、投稿・編集・削除のどれも断り、削除済みのまま(決定 S8)。
func TestEditCommentOnItemDeletedWithParentIsNotFound(t *testing.T) {
	svc, rec := setup(t, "hanako")
	posted, err := svc.Post(t.Context(), store, onTask, "前")
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(t.Context(), rec, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	_, postErr := svc.Post(t.Context(), store, onTask, "新しい")
	_, editErr := svc.Edit(t.Context(), store, onTask, posted.ID, "後")
	deleteErr := svc.Delete(t.Context(), store, onTask, posted.ID)
	for name, err := range map[string]error{"Post": postErr, "Edit": editErr, "Delete": deleteErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
	if hasRestore(t, rec, onTask) {
		t.Error("断ったのに削除を取り消した")
	}
}

// 親と一緒に削除された(root: false)プロジェクトのコメントも、同じ。
func TestCommentsOnProjectDeletedWithParentAreNotFound(t *testing.T) {
	svc, rec := setup(t, "hanako")
	posted, err := svc.Post(t.Context(), store, onProject, "前")
	if err != nil {
		t.Fatal(err)
	}
	markDeleted(t, rec, onProject, false)
	_, postErr := svc.Post(t.Context(), store, onProject, "新しい")
	_, editErr := svc.Edit(t.Context(), store, onProject, posted.ID, "後")
	deleteErr := svc.Delete(t.Context(), store, onProject, posted.ID)
	for name, err := range map[string]error{"Post": postErr, "Edit": editErr, "Delete": deleteErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
	if hasRestore(t, rec, onProject) {
		t.Error("断ったのに削除を取り消した")
	}
}

// wantEditDeleteRefused は、コメント id の編集と削除がどちらも port.ErrNotFound で断られることを確かめる。
func wantEditDeleteRefused(t *testing.T, svc comments.Service, tg comments.Target, id change.ID, name string) {
	t.Helper()
	_, editErr := svc.Edit(t.Context(), store, tg, id, "後")
	deleteErr := svc.Delete(t.Context(), store, tg, id)
	for op, err := range map[string]error{"Edit": editErr, "Delete": deleteErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s の %s = %v, want port.ErrNotFound", name, op, err)
		}
	}
}

// 削除済み(root: true)の持ち主でも、他人のコメントと存在しないコメントの編集・削除は断り、削除済みのまま。
func TestEditDeleteCommentDeletedRefusesOthersAndMissing(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, rec := setup(t, "hanako")
			taro := svc
			taro.Identity = apptest.Identity{Login: "taro"}
			others, err := taro.Post(t.Context(), store, o.target, "taro のコメント")
			if err != nil {
				t.Fatal(err)
			}
			markDeleted(t, rec, o.target, true)
			for name, id := range map[string]change.ID{"他人のコメント": others.ID, "存在しないコメント": "01NOPE00000000000000000000"} {
				wantEditDeleteRefused(t, svc, o.target, id, name)
			}
			if hasRestore(t, rec, o.target) {
				t.Error("断ったのに削除を取り消した")
			}
		})
	}
}

// 削除済みのアイテム・プロジェクトのコメントは、読めない(A4b-4)。
func TestListCommentsOfADeletedOwnerIsNotFound(t *testing.T) {
	svc, rec := setup(t, "hanako")
	markDeleted(t, rec, onTask, true)
	markDeleted(t, rec, onProject, true)
	for _, o := range owners {
		if _, err := svc.List(t.Context(), store, o.target); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("List of a deleted %s = %v, want port.ErrNotFound", o.name, err)
		}
	}
}

// 削除済み(root: true)の Task でも、祖先の Epic が削除済みなら、投稿を断り、削除済みのまま。
func TestPostCommentDeletedUnderDeletedAncestorIsNotFound(t *testing.T) {
	svc, rec := setup(t, "hanako")
	markDeleted(t, rec, comments.Target{Kind: change.KindItem, ID: apptest.SampleEpic}, true)
	markDeleted(t, rec, onTask, true)
	if _, err := svc.Post(t.Context(), store, onTask, "確認"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Post = %v, want port.ErrNotFound", err)
	}
	if hasRestore(t, rec, onTask) {
		t.Error("断ったのに削除を取り消した")
	}
}

func postAndDelete(t *testing.T, svc comments.Service, tg comments.Target, body string) change.ID {
	t.Helper()
	posted, err := svc.Post(t.Context(), store, tg, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(t.Context(), store, tg, posted.ID); err != nil {
		t.Fatal(err)
	}
	return posted.ID
}

// 別のタブで削除された自分のコメントを編集すると、コメントが戻って本文が新しくなる。
func TestEditDeletedCommentRevives(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, _ := setup(t, "hanako")
			id := postAndDelete(t, svc, o.target, "前")
			ctx := port.WithRevived(t.Context())
			v, err := svc.Edit(ctx, store, o.target, id, "後")
			if err != nil || v.Body != "後" {
				t.Fatalf("Edit = %+v, %v", v, err)
			}
			if r := port.Revived(ctx); !slices.Equal(r, []port.Revival{{Kind: port.RevivedComment, Text: o.display}}) {
				t.Errorf("Revived = %v", r)
			}
			list, err := svc.List(t.Context(), store, o.target)
			if err != nil || len(list) != 1 || list[0].Body != "後" {
				t.Errorf("List = %+v, %v", list, err)
			}
		})
	}
}

// 削除の前と同じ本文なら、戻さずに断る。
func TestEditDeletedCommentSameBodyIsNotFound(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, _ := setup(t, "hanako")
			id := postAndDelete(t, svc, o.target, "前")
			ctx := port.WithRevived(t.Context())
			if _, err := svc.Edit(ctx, store, o.target, id, "前"); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("Edit = %v, want port.ErrNotFound", err)
			}
			list, err := svc.List(t.Context(), store, o.target)
			if err != nil || len(list) != 0 {
				t.Errorf("List = %+v, %v", list, err)
			}
		})
	}
}

// 他人の削除済みのコメントは、編集できない。
func TestEditDeletedCommentOfOthersIsNotFound(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, _ := setup(t, "hanako")
			taro := svc
			taro.Identity = apptest.Identity{Login: "taro"}
			id := postAndDelete(t, taro, o.target, "taro のコメント")
			if _, err := svc.Edit(t.Context(), store, o.target, id, "後"); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("Edit = %v, want port.ErrNotFound", err)
			}
		})
	}
}

// 持ち主も削除されていれば、持ち主とコメントの両方が戻る。
func TestEditDeletedCommentOnDeletedOwnerRevivesBoth(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, rec := setup(t, "hanako")
			id := postAndDelete(t, svc, o.target, "前")
			markDeleted(t, rec, o.target, true)
			ctx := port.WithRevived(t.Context())
			if _, err := svc.Edit(ctx, store, o.target, id, "後"); err != nil {
				t.Fatal(err)
			}
			want := []port.Revival{{Kind: port.RevivedID, Text: o.display}, {Kind: port.RevivedComment, Text: o.display}}
			if r := port.Revived(ctx); !slices.Equal(r, want) {
				t.Errorf("Revived = %v, want %v", r, want)
			}
			if !hasRestore(t, rec, o.target) {
				t.Error("restore の行がない")
			}
		})
	}
}

// 削除済みのコメントの削除は、いまのまま断る。
func TestDeleteDeletedCommentIsNotFound(t *testing.T) {
	for _, o := range owners {
		t.Run(o.name, func(t *testing.T) {
			svc, _ := setup(t, "hanako")
			id := postAndDelete(t, svc, o.target, "前")
			if err := svc.Delete(t.Context(), store, o.target, id); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("Delete = %v, want port.ErrNotFound", err)
			}
		})
	}
}
