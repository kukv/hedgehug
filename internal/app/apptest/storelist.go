package apptest

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// StoreList は、Stores に並べた Store を、その順でそのまま返す偽物。
type StoreList struct {
	Stores []port.StoreInfo
}

func (l StoreList) List(context.Context) ([]port.StoreInfo, error) {
	return slices.Clone(l.Stores), nil
}

// RunStoreCommitsContract は、port.StoreCommits が満たすべき振る舞いを確かめる。偽物(CommitLog)と本物の実装
// (infra/gitstore)の両方のテストから呼ぶ。newStores は、空の Store を 1 つ持つ Stores と、その保存を返す口と、
// Store の ID を返す。
func RunStoreCommitsContract(t *testing.T, newStores func(t *testing.T) (port.Stores, port.StoreCommits, port.StoreID)) {
	cases := []struct {
		name string
		fn   func(t *testing.T, s port.Stores, c port.StoreCommits, id port.StoreID)
	}{
		{"commits are newest first", contractCommitsNewestFirst},
		{"an empty write is not recorded", contractCommitsEmptyWrite},
		{"a removed entity is recorded without rows", contractCommitsRemoved},
		{"a bot actor is authored by the bot", contractCommitsBotAuthor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, c, id := newStores(t)
			tc.fn(t, s, c, id)
		})
	}
}

// commitsSetup は、ワークスペースとプロジェクトとメンバーの一覧を 1 回の保存で書く(アイテムはまだない)。
func commitsSetup(t *testing.T, s port.Stores, id port.StoreID) {
	t.Helper()
	d, err := workspace.Create(cWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteMembers().Append(member.Init()),
			tx.WriteWorkspaces().Append(cWorkspace.ID, d),
			tx.WriteProjects().Append(cProject.ID, project.Create(cProject)),
		)
	})
}

func mustCommits(t *testing.T, c port.StoreCommits, id port.StoreID) []port.Commit {
	t.Helper()
	got, err := c.Commits(t.Context(), id)
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	return got
}

func contractCommitsNewestFirst(t *testing.T, s port.Stores, c port.StoreCommits, id port.StoreID) {
	commitsSetup(t, s, id)
	saves := []struct {
		summary string
		fn      func(tx port.Tx) error
	}{
		{"アイテムを作成", func(tx port.Tx) error { return tx.WriteItems().Append(cItem.ID, item.Create(cItem)) }},
		{"タイトルを変更", func(tx port.Tx) error {
			return tx.WriteItems().Append(cItem.ID, change.NewSet("title", cItem.Title, "二段階認証"))
		}},
		{"メンバー hanako を追加", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("hanako")) }},
	}
	for _, sv := range saves {
		mustWrite(t, s, id, port.Operation{Actor: change.User("taro"), Summary: sv.summary}, sv.fn)
	}
	got := mustCommits(t, c, id)
	if len(got) != 4 {
		t.Fatalf("Commits = %d, want 4 (setup and 3 saves)", len(got))
	}
	kinds := []change.Kind{change.KindMembers, change.KindItem, change.KindItem}
	for i, commit := range got[:3] {
		sv := saves[2-i]
		if commit.Summary != sv.summary || len(commit.Changes) != 1 || commit.Changes[0].Ref.Kind != kinds[i] {
			t.Errorf("commit %d = %q, changes %+v; want %q on a %s", i, commit.Summary, commit.Changes, sv.summary, kinds[i])
			continue
		}
		if rows := commit.Changes[0].Rows; len(rows) == 0 || commit.Op != rows[0].Op || commit.Author != "taro" || commit.SHA == "" {
			t.Errorf("commit %d: op %q author %q sha %q rows %+v", i, commit.Op, commit.Author, commit.SHA, rows)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].At.Before(got[i].At) {
			t.Errorf("commit %d is newer than commit %d", i, i-1)
		}
	}
}

func contractCommitsEmptyWrite(t *testing.T, s port.Stores, c port.StoreCommits, id port.StoreID) {
	commitsSetup(t, s, id)
	n := len(mustCommits(t, c, id))
	mustWrite(t, s, id, taro, func(port.Tx) error { return nil })
	if got := len(mustCommits(t, c, id)); got != n {
		t.Errorf("Commits after an empty write = %d, want %d", got, n)
	}
}

func contractCommitsRemoved(t *testing.T, s port.Stores, c port.StoreCommits, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	n := len(mustCommits(t, c, id))
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Remove(cItem.ID),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: cItem.ID, Old: "HH-1", ToStore: "S2"}),
		)
	})
	got := mustCommits(t, c, id)
	if len(got) != n+1 {
		t.Fatalf("Commits = %d, want %d", len(got), n+1)
	}
	if last := got[0]; len(last.Changes) != 0 || !slices.Equal(last.Removed, []change.Ref{{Kind: change.KindItem, ID: cItem.ID}}) || last.StoreLog {
		t.Errorf("last = changes %+v removed %+v storeLog %v", last.Changes, last.Removed, last.StoreLog)
	}
}

func contractCommitsBotAuthor(t *testing.T, s port.Stores, c port.StoreCommits, id port.StoreID) {
	commitsSetup(t, s, id)
	bot := port.Operation{Actor: change.Actor{Kind: change.ActorGitHub}, Summary: "GitHub から作成"}
	mustWrite(t, s, id, bot, func(tx port.Tx) error { return tx.WriteItems().Append(cItem.ID, item.Create(cItem)) })
	got := mustCommits(t, c, id)
	if got[0].Author != change.BotLogin || got[1].Author != "taro" {
		t.Errorf("authors = %q, %q; want %q, taro", got[0].Author, got[1].Author, change.BotLogin)
	}
}
