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

// 足したのに fn が何も書かなかったら、足した分を捨て、包まない fn で内側の SyncSave を呼び直す(送り・Seen・数は、
// 包まない SyncSave と同じ。待っていたローカルの保存も送る)。
func TestJoinOnSyncSaveSkipsEmptyWrite(t *testing.T) {
	c := newJoinStores(t, "hanako")
	want := port.SyncResult{Seen: "seen-after", Sent: 2}
	inner := &apptest.Replicator{Stores: c.Stores.(*apptest.Stores), Results: map[port.StoreID]port.SyncResult{"S1": want}}
	r := members.JoinOnSyncSave(inner, writable())
	ctx := port.WithJoined(t.Context())
	var calls int
	res, err := r.SyncSave(ctx, "S1", joinRemote, taroOp, func(port.Tx) error { calls++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if res != want {
		t.Errorf("result = %+v, want the inner result %+v", res, want)
	}
	// 2 回目は包まない fn なので、メンバーを足さず、何も書かないまま成功する。
	if n := len(inner.Calls()); n != 2 || calls != 2 {
		t.Errorf("inner SyncSave calls = %d, fn calls = %d, want 2 and 2", n, calls)
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
