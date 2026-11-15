package members_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	joinWorkspace workspace.ID = "01JNWS00000000000000000001"
	joinProject   project.ID   = "01JNPR00000000000000000001"
	joinItem      item.ID      = "01JNIT00000000000000000001"
)

var taroOp = port.Operation{Actor: change.User("taro"), Summary: "アイテムを作成"}

// newJoinStores は、メンバーが hanako だけで、ワークスペースとプロジェクトがある Store S1 を、コミットを数える
// 偽物で返す。first は S1 を最初に書くメンバー(hanako か Taro)。
func newJoinStores(t *testing.T, first string) *apptest.CommitLog {
	t.Helper()
	clock := apptest.NewClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC))
	ids := apptest.NewIDs()
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	ws := workspace.Workspace{ID: joinWorkspace, Name: "Web", Prefix: "HH"}
	pr := project.Project{ID: joinProject, Place: project.Place{Workspace: ws.ID, Number: 1}, Rank: "V", Title: "ログイン", Owner: first, Status: project.Todo}
	op := port.Operation{Actor: change.User(first), Summary: "準備"}
	err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(ids.New))
		if err != nil {
			return err
		}
		return errors.Join(
			tx.WriteMembers().Append(member.Init(), member.Join(first)),
			tx.WriteWorkspaces().Append(ws.ID, d),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	return apptest.NewCommitLog(s, clock)
}

// writable は、S1 を閲覧のみでない Store として持つ一覧。
func writable() *apptest.StoreGitHub {
	return &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "S1"}}}
}

func createItem(tx port.Tx) error {
	it := item.Item{ID: joinItem, Place: item.Place{Workspace: joinWorkspace, Project: joinProject, Number: 1}, Kind: item.Task, Rank: rank.First, Title: "画面を作る"}
	return tx.WriteItems().Append(it.ID, item.Create(it))
}

func logins(t *testing.T, s port.Stores) []string {
	t.Helper()
	var out []string
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		ms, err := r.Members().List()
		for _, m := range ms {
			out = append(out, m.Login)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func commits(t *testing.T, c *apptest.CommitLog) int {
	t.Helper()
	got, err := c.Commits(t.Context(), "S1")
	if err != nil {
		t.Fatal(err)
	}
	return len(got)
}

func TestJoinWithWrite(t *testing.T) {
	c := newJoinStores(t, "hanako")
	ctx := port.WithJoined(t.Context())
	if err := members.JoinOnWrite(c, writable()).Write(ctx, "S1", taroOp, createItem); err != nil {
		t.Fatal(err)
	}
	if n := commits(t, c); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
	if got := logins(t, c); !slices.Equal(got, []string{"hanako", "taro"}) {
		t.Errorf("members = %v", got)
	}
	if got := port.Joined(ctx); !slices.Equal(got, []string{"taro"}) {
		t.Errorf("Joined = %v", got)
	}
	// もうメンバーなので、次の書き込みでは足さない。
	ctx = port.WithJoined(t.Context())
	err := members.JoinOnWrite(c, writable()).Write(ctx, "S1", taroOp, func(tx port.Tx) error {
		return tx.WriteItems().Append(joinItem, change.NewSet("title", "画面を作る", "画面"))
	})
	if err != nil || len(port.Joined(ctx)) != 0 {
		t.Errorf("second write: %v, Joined = %v", err, port.Joined(ctx))
	}
}

func TestJoinBeforeFn(t *testing.T) {
	c := newJoinStores(t, "hanako")
	var seen []string
	err := members.JoinOnWrite(c, writable()).Write(t.Context(), "S1", taroOp, func(tx port.Tx) error {
		ms, err := tx.Members().List()
		for _, m := range ms {
			seen = append(seen, m.Login)
		}
		return errors.Join(err, createItem(tx))
	})
	if err != nil || !slices.Contains(seen, "taro") {
		t.Errorf("members in fn = %v, %v", seen, err)
	}
}

func TestJoinSkipsEmptyWrite(t *testing.T) {
	c := newJoinStores(t, "hanako")
	ctx := port.WithJoined(t.Context())
	empties := map[string]func(port.Tx) error{
		"何も呼ばない":      func(port.Tx) error { return nil },
		"行のない Append": func(tx port.Tx) error { return tx.WriteItems().Append(joinItem) },
	}
	for name, fn := range empties {
		if err := members.JoinOnWrite(c, writable()).Write(ctx, "S1", taroOp, fn); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if n := commits(t, c); n != 0 {
		t.Errorf("commits = %d, want 0", n)
	}
	if got := logins(t, c); !slices.Equal(got, []string{"hanako"}) {
		t.Errorf("members = %v", got)
	}
	if got := port.Joined(ctx); len(got) != 0 {
		t.Errorf("Joined = %v", got)
	}
}

func TestJoinSkipsMemberCaseInsensitive(t *testing.T) {
	c := newJoinStores(t, "Taro")
	ctx := port.WithJoined(t.Context())
	if err := members.JoinOnWrite(c, writable()).Write(ctx, "S1", taroOp, createItem); err != nil {
		t.Fatal(err)
	}
	if got := logins(t, c); !slices.Equal(got, []string{"Taro"}) {
		t.Errorf("members = %v", got)
	}
	if got := port.Joined(ctx); len(got) != 0 {
		t.Errorf("Joined = %v", got)
	}
}

func TestJoinSkipsReadOnlyAndNonUser(t *testing.T) {
	readOnly := &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "S1", ReadOnly: true}}}
	cases := map[string]struct {
		list  port.StoreGitHub
		actor change.Actor
	}{
		"閲覧のみ":   {readOnly, change.User("taro")},
		"同期":     {writable(), change.Actor{Kind: change.ActorSync}},
		"GitHub": {writable(), change.Actor{Kind: change.ActorGitHub}},
		"自動化":    {writable(), change.Actor{Kind: change.ActorAutomation, Rule: "01JNRU00000000000000000001"}},
	}
	for name, tc := range cases {
		c := newJoinStores(t, "hanako")
		ctx := port.WithJoined(t.Context())
		op := port.Operation{Actor: tc.actor, Summary: "アイテムを作成"}
		if err := members.JoinOnWrite(c, tc.list).Write(ctx, "S1", op, createItem); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := logins(t, c); !slices.Equal(got, []string{"hanako"}) {
			t.Errorf("%s: members = %v", name, got)
		}
		if n := commits(t, c); n != 1 {
			t.Errorf("%s: commits = %d, want 1", name, n)
		}
		if got := port.Joined(ctx); len(got) != 0 {
			t.Errorf("%s: Joined = %v", name, got)
		}
	}
}

func TestJoinRolledBackOnError(t *testing.T) {
	c := newJoinStores(t, "hanako")
	ctx := port.WithJoined(t.Context())
	boom := errors.New("boom")
	err := members.JoinOnWrite(c, writable()).Write(ctx, "S1", taroOp, func(tx port.Tx) error {
		return errors.Join(createItem(tx), boom)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if got := logins(t, c); !slices.Equal(got, []string{"hanako"}) {
		t.Errorf("members = %v", got)
	}
	if got := port.Joined(ctx); len(got) != 0 {
		t.Errorf("Joined = %v", got)
	}
}

// Read はそのまま内側へ渡す。
func TestJoinPassesRead(t *testing.T) {
	c := newJoinStores(t, "hanako")
	err := members.JoinOnWrite(c, writable()).Read(t.Context(), "nope", func(port.Reader) error { return nil })
	if !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("Read = %v", err)
	}
}
