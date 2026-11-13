package apptest

import (
	"errors"
	"slices"
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
		{"retention days default to 30", contractRetentionDays},
		{"writes are serialized", contractSerialized},
		{"workspaces are listed in id order", contractList},
		{"reads never see a write in progress", contractReadIsolated},
		{"summary is one non-empty line", contractSummary},
		{"a write without rows needs no summary", contractEmptyWrite},
		{"actor login must be valid", contractActorLogin},
		{"actor must round-trip", contractActorRoundTrip},
		{"malformed ids are not found and not written", contractMalformedIDs},
		{"members have one log with an empty id", contractRefs},
		{"projects and items are listed", contractLists},
		{"item list follows a move in the same write", contractListFollowsMove},
		{"project list follows a move in the same write", contractProjectListFollowsMove},
		{"workspace settings round trip", contractSettings},
		{"project rules round trip", contractProjectRules},
		{"item values and deletes round trip", contractItemValues},
		{"deleted items and projects carry a mark and are still listed", contractDeleted},
		{"a restore row cancels the deletion it points at", contractRestored},
		{"a person's restore is seen in the same write", contractUserRestoreSeenInTx},
		{"descriptions round trip", contractDescription},
		{"a failed write leaves the description", contractDescriptionRollback},
		{"describing needs an existing entity and a summary", contractDescriptionRules},
		{"automation rows share the operation", contractRuleRows},
		{"automation rows in a GitHub operation are by the bot", contractRuleRowsGitHub},
		{"automation rows need a person or GitHub and a valid rule id", contractRuleRowsRefused},
		{"transfer records are appended and listed", contractTransfers},
		{"a transfer record keeps the time it was given", contractTransferAt},
		{"purged is empty without a purge", contractPurgedEmpty},
		{"remove deletes an item folder", contractRemove},
		{"remove and import refuse in the same write", contractRemoveImportRules},
		{"op rows are for sync operations only", contractOpRows},
		{"remove deletes a project folder", contractProjectRemove},
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
	t.Run("import copies an item folder between stores", func(t *testing.T) {
		s, a, b := newStores(t)
		contractImport(t, s, a, b)
	})
	t.Run("import copies a project folder between stores", func(t *testing.T) {
		s, a, b := newStores(t)
		contractProjectImport(t, s, a, b)
	})
}

// 照合テストの準備のデータ。
var (
	cWorkspace = workspace.Workspace{ID: "01CONTRACTWS00000000000001", Name: "Web", Prefix: "HH"}
	cProject   = project.Project{ID: "01CONTRACTPR00000000000001", Place: project.Place{Workspace: cWorkspace.ID, Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	cItem      = item.Item{ID: "01CONTRACTIT00000000000001", Place: item.Place{Workspace: cWorkspace.ID, Project: cProject.ID, Number: 1}, Kind: item.Epic, Rank: "a", Title: "認証", Watchers: []string{"taro"}}
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

// Remove は、アイテムのフォルダを消す。同じ Tx から読むと、もうない。説明も一緒に消える。
func contractRemove(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error { return tx.WriteItems().Describe(cItem.ID, "説明") })
	boom := errors.New("boom")
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Remove(cItem.ID), boom)
	}); !errors.Is(err, boom) {
		t.Fatalf("failing write = %v", err)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		if d, err := r.Items().Description(cItem.ID); err != nil || d != "説明" {
			t.Errorf("after the failed write: %q, %v", d, err)
		}
		return nil
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Remove(cItem.ID); err != nil {
			return err
		}
		if _, err := tx.Items().Get(cItem.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Get in the tx = %v", err)
		}
		if err := tx.WriteItems().Import(cItem.ID, []change.Record{{ID: "x"}}, ""); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import after Remove = %v, want ErrInvalidOperation", err)
		}
		return nil
	})
	mustRead(t, s, id, func(r port.Reader) error { checkRemoved(t, r); return nil })
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Remove(cItem.ID) }); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Remove of a missing item = %v, want ErrNotFound", err)
	}
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Remove("bad id") }); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Remove of a bad id = %v, want ErrInvalidOperation", err)
	}
}

// checkRemoved は、消したアイテムが、どの読み方でももうなく、プロジェクトは残っていることを確かめる。
func checkRemoved(t *testing.T, r port.Reader) {
	if _, err := r.Items().Get(cItem.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Get = %v", err)
	}
	if _, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Changes().Of = %v", err)
	}
	if _, err := r.Items().Description(cItem.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Description = %v", err)
	}
	if its, err := r.Items().List(cProject.ID); err != nil || len(its) != 0 {
		t.Errorf("List = %+v, %v", its, err)
	}
	if _, err := r.Projects().Get(cProject.ID); err != nil {
		t.Errorf("the project went too: %v", err)
	}
}

// Import は、別の Store のアイテムの行と説明を、行を変えずに写す。続けて書く行は、写した行の先端につながる。
func contractImport(t *testing.T, s port.Stores, a, b port.StoreID) {
	ref := change.Ref{Kind: change.KindItem, ID: cItem.ID}
	mustWrite(t, s, a, taro, writeFixture)
	mustWrite(t, s, a, taro, func(tx port.Tx) error { return tx.WriteItems().Describe(cItem.ID, "説明") })
	var log []change.Record
	var text string
	mustRead(t, s, a, func(r port.Reader) error {
		var err error
		if log, err = r.Changes().Of(ref); err != nil {
			return err
		}
		text, err = r.Items().Description(cItem.ID)
		return err
	})
	mustWrite(t, s, b, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Import(cItem.ID, log, text); err != nil {
			return err
		}
		return tx.WriteItems().Append(cItem.ID, change.NewSet("title", "認証", "移動先で変えた"))
	})
	mustRead(t, s, b, func(r port.Reader) error { checkImported(t, r, log); return nil })
	for name, call := range map[string]func(tx port.Tx) error{
		"existing folder": func(tx port.Tx) error { return tx.WriteItems().Import(cItem.ID, log, "") },
		"empty log":       func(tx port.Tx) error { return tx.WriteItems().Import("01CONTRACTIT00000000000009", nil, "") },
		"bad id":          func(tx port.Tx) error { return tx.WriteItems().Import("bad id", log, "") },
	} {
		if err := s.Write(t.Context(), b, taro, call); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import (%s) = %v, want ErrInvalidOperation", name, err)
		}
	}
}

// Import は、プロジェクトの行と説明を変えずに写す。写したプロジェクトに、同じ Tx で行を足せる。
func contractProjectImport(t *testing.T, s port.Stores, a, b port.StoreID) {
	ref := change.Ref{Kind: change.KindProject, ID: cProject.ID}
	mustWrite(t, s, a, taro, writeFixture)
	mustWrite(t, s, a, taro, func(tx port.Tx) error { return tx.WriteProjects().Describe(cProject.ID, "説明") })
	var log []change.Record
	mustRead(t, s, a, func(r port.Reader) (err error) { log, err = r.Changes().Of(ref); return })
	moved := cProject.Place
	moved.Number = 2
	mustWrite(t, s, b, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteProjects().Import(cProject.ID, log, "説明"),
			tx.WriteProjects().Append(cProject.ID, change.NewSet("place", cProject.Place, moved)))
	})
	mustRead(t, s, b, func(r port.Reader) error {
		got, err := r.Changes().Of(ref)
		p, perr := r.Projects().Get(cProject.ID)
		d, _ := r.Projects().Description(cProject.ID)
		if err != nil || perr != nil || len(got) != len(log)+1 || got[0].ID != log[0].ID || p.Place != moved || d != "説明" ||
			!slices.Equal(got[len(log)].Prev, []change.ID{log[len(log)-1].ID}) {
			t.Errorf("rows %d (want %d), %v, project %+v (%v), description %q", len(got), len(log)+1, err, p, perr, d)
		}
		return nil
	})
	for name, call := range map[string]func(tx port.Tx) error{
		"existing folder": func(tx port.Tx) error { return tx.WriteProjects().Import(cProject.ID, log, "") },
		"empty log":       func(tx port.Tx) error { return tx.WriteProjects().Import("01CONTRACTPR00000000000009", nil, "") },
		"bad id":          func(tx port.Tx) error { return tx.WriteProjects().Import("bad id", log, "") },
	} {
		if err := s.Write(t.Context(), b, taro, call); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import project (%s) = %v, want ErrInvalidOperation", name, err)
		}
	}
}

// checkImported は、写した行が元の行と同じで、続けて書いた 1 行が写した行の先端につながっていることを確かめる。
func checkImported(t *testing.T, r port.Reader, log []change.Record) {
	got, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
	if err != nil || len(got) != len(log)+1 {
		t.Fatalf("rows = %d, %v, want %d", len(got), err, len(log)+1)
	}
	for i, w := range log {
		g := got[i]
		if g.ID != w.ID || g.Op != w.Op || !g.At.Equal(w.At) || g.By != w.By || g.Actor != w.Actor || g.T != w.T || g.F != w.F ||
			string(g.From) != string(w.From) || string(g.To) != string(w.To) || !slices.Equal(g.Prev, w.Prev) {
			t.Errorf("row %d = %+v, want %+v (行は変えずに写す)", i, g, w)
		}
	}
	if last := got[len(got)-1]; !slices.Equal(last.Prev, []change.ID{log[0].ID}) {
		t.Errorf("the row after the import has prev %v, want [%s]", last.Prev, log[0].ID)
	}
	it, err := r.Items().Get(cItem.ID)
	if d, _ := r.Items().Description(cItem.ID); err != nil || it.Title != "移動先で変えた" || d != "説明" {
		t.Errorf("item = %+v (%v), description %q", it, err, d)
	}
}

// Remove だけの Write も Summary が要る。同じ Tx で行を足してから消しても、もうない。同じ Tx で作ったものには Import できない。
func contractRemoveImportRules(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	noSummary := port.Operation{Actor: change.User("taro")}
	if err := s.Write(t.Context(), id, noSummary, func(tx port.Tx) error { return tx.WriteItems().Remove(cItem.ID) }); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Remove without a summary = %v, want ErrInvalidOperation", err)
	}
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Append(cItem.ID, change.NewSet("title", "認証", "消す前")); err != nil {
			return err
		}
		if err := tx.WriteItems().Remove(cItem.ID); err != nil {
			return err
		}
		if _, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID}); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Changes().Of after Append and Remove = %v, want ErrNotFound", err)
		}
		if its, err := tx.Items().List(cProject.ID); err != nil || len(its) != 0 {
			t.Errorf("List after Append and Remove = %+v, %v", its, err)
		}
		return nil
	})
	created := cItem
	created.ID = "01CONTRACTIT00000000000002"
	err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Append(created.ID, item.Create(created)); err != nil {
			return err
		}
		return tx.WriteItems().Import(created.ID, []change.Record{{ID: "x"}}, "")
	})
	if !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Import after Append in the same write = %v, want ErrInvalidOperation", err)
	}
}
