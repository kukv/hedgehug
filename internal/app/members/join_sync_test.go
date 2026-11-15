package members_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

var joinRemote = port.Remote{URL: "https://github.com/o/r.git", Token: "t", Seen: "seen-before"}

// joinReplicator は、メンバーが hanako だけの S1 に保存する偽の Replicator を、JoinOnSyncSave で包んで返す。
func joinReplicator(t *testing.T) (port.Replicator, *apptest.CommitLog) {
	t.Helper()
	c := newJoinStores(t, "hanako")
	return members.JoinOnSyncSave(&apptest.Replicator{Stores: c.Stores.(*apptest.Stores)}, writable()), c
}

func TestJoinOnSyncSave(t *testing.T) {
	r, c := joinReplicator(t)
	ctx := port.WithJoined(t.Context())
	if _, err := r.SyncSave(ctx, "S1", joinRemote, taroOp, createItem); err != nil {
		t.Fatal(err)
	}
	if got := logins(t, c); !slices.Equal(got, []string{"hanako", "taro"}) {
		t.Errorf("members = %v", got)
	}
	if got := port.Joined(ctx); !slices.Equal(got, []string{"taro"}) {
		t.Errorf("Joined = %v", got)
	}
}

func TestJoinOnSyncSaveSkipsEmptyWrite(t *testing.T) {
	r, c := joinReplicator(t)
	ctx := port.WithJoined(t.Context())
	res, err := r.SyncSave(ctx, "S1", joinRemote, taroOp, func(port.Tx) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	// 受け取った位置は、前の同期の位置のまま(空にすると、次の同期で書き換えを見分けられなくなる)。
	if res.Seen != joinRemote.Seen {
		t.Errorf("Seen = %q, want %q", res.Seen, joinRemote.Seen)
	}
	if got := logins(t, c); !slices.Equal(got, []string{"hanako"}) {
		t.Errorf("members = %v", got)
	}
	if got := port.Joined(ctx); len(got) != 0 {
		t.Errorf("Joined = %v", got)
	}
}

func TestJoinOnSyncSavePassesNonUserAndErrors(t *testing.T) {
	r, c := joinReplicator(t)
	ctx := port.WithJoined(t.Context())
	op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "同期の修復"}
	if _, err := r.SyncSave(ctx, "S1", joinRemote, op, createItem); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("boom")
	_, err := r.SyncSave(ctx, "S1", joinRemote, taroOp, func(tx port.Tx) error { return errors.Join(createItem(tx), failure) })
	if !errors.Is(err, failure) {
		t.Errorf("err = %v, want %v", err, failure)
	}
	if got := logins(t, c); !slices.Equal(got, []string{"hanako"}) {
		t.Errorf("members = %v", got)
	}
	if got := port.Joined(ctx); len(got) != 0 {
		t.Errorf("Joined = %v", got)
	}
}

// retrying は、1 回目の fn を S1(taro はメンバーでない)で試して捨て、2 回目を member(taro がメンバー)に保存する
// Replicator(送信が重なって受け取り直したら、taro がメンバーになっていた)。
type retrying struct {
	port.Replicator
	first, member port.Stores
}

var errDiscard = errors.New("discard")

func (r retrying) SyncSave(ctx context.Context, _ port.StoreID, _ port.Remote, op port.Operation, fn func(tx port.Tx) error) (port.SyncResult, error) {
	err := r.first.Write(ctx, "S1", op, func(tx port.Tx) error { return errors.Join(fn(tx), errDiscard) })
	if !errors.Is(err, errDiscard) {
		return port.SyncResult{}, err
	}
	return port.SyncResult{}, r.member.Write(ctx, "S1", op, fn)
}

func TestJoinOnSyncSaveRetryJudgesAgain(t *testing.T) {
	first, member := newJoinStores(t, "hanako"), newJoinStores(t, "taro")
	r := members.JoinOnSyncSave(retrying{first: first, member: member}, writable())
	ctx := port.WithJoined(t.Context())
	if _, err := r.SyncSave(ctx, "S1", joinRemote, taroOp, createItem); err != nil {
		t.Fatal(err)
	}
	if got := port.Joined(ctx); len(got) != 0 {
		t.Errorf("Joined = %v, want none (taro was a member on the saved try)", got)
	}
	if got := logins(t, member); !slices.Equal(got, []string{"taro"}) {
		t.Errorf("members = %v", got)
	}
}
