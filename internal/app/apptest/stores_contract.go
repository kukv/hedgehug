package apptest

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// RunStoresContract は、port.Stores が満たすべき振る舞いを確かめる。偽物と、本物の実装(infra/gitstore)の
// 両方のテストから呼ぶ。newStores は、空の Store を 2 つ持つ Stores と、その 2 つの Store の ID を返す。
// ほとんどの照合は 1 つ目だけを使う。
func RunStoresContract(t *testing.T, newStores func(t *testing.T) (s port.Stores, id, other port.StoreID)) {
	cases := []struct {
		name string
		fn   func(t *testing.T, s port.Stores, id port.StoreID)
	}{
		{"write then read", contractWriteRead},
		{"records are stamped", contractStamped},
		{"failed write leaves nothing", contractFailedWrite},
		{"tx sees its own writes", contractTxSeesOwn},
		{"not found", contractNotFound},
		{"writes are serialized", contractSerialized},
		{"workspaces are listed in id order", contractList},
		{"reads never see a write in progress", contractReadIsolated},
		{"summary is one non-empty line", contractSummary},
		{"a write without rows needs no summary", contractEmptyWrite},
		{"actor login must be valid", contractActorLogin},
		{"actor must round-trip", contractActorRoundTrip},
		{"malformed ids are not found and not written", contractMalformedIDs},
		{"members have one log with an empty id", contractRefs},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, id, _ := newStores(t)
			c.fn(t, s, id)
		})
	}
	t.Run("a write to another store inside a write", func(t *testing.T) {
		s, a, b := newStores(t)
		contractNestedStores(t, s, a, b)
	})
}

// 照合テストの準備のデータ。
var (
	cWorkspace = workspace.Workspace{ID: "01CONTRACTWS00000000000001", Name: "Web", Prefix: "HH"}
	cProject   = project.Project{ID: "01CONTRACTPR00000000000001", Place: project.Place{Workspace: cWorkspace.ID, Number: 1}, Title: "ログイン"}
	cItem      = item.Item{ID: "01CONTRACTIT00000000000001", Place: item.Place{Workspace: cWorkspace.ID, Project: cProject.ID, Number: 1}, Kind: item.Epic, Rank: "a", Title: "認証"}
	taro       = port.Operation{Actor: change.User("taro"), Summary: "準備"}
)

func writeFixture(tx port.Tx) error {
	d, err := workspace.Create(cWorkspace)
	if err != nil {
		return err
	}
	return errors.Join(
		tx.WriteMembers().Append(member.Init(), member.Join("taro")),
		tx.WriteWorkspaces().Append(cWorkspace.ID, d),
		tx.WriteProjects().Append(cProject.ID, project.Create(cProject)),
		tx.WriteItems().Append(cItem.ID, item.Create(cItem)),
	)
}

func mustWrite(t *testing.T, s port.Stores, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) {
	t.Helper()
	if err := s.Write(t.Context(), id, op, fn); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func mustRead(t *testing.T, s port.Stores, id port.StoreID, fn func(r port.Reader) error) {
	t.Helper()
	if err := s.Read(t.Context(), id, fn); err != nil {
		t.Fatalf("Read: %v", err)
	}
}

func sameItem(a, b item.Item) bool {
	a.Created, b.Created = item.Stamp{}, item.Stamp{}
	pa, pb := a.Place.Parent, b.Place.Parent
	a.Place.Parent, b.Place.Parent = nil, nil
	return a == b && (pa == nil) == (pb == nil) && (pa == nil || *pa == *pb)
}

func setTitle(id item.ID, from, to string) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewSet("title", from, to))
	}
}
