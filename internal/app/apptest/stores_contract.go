package apptest

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
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
	for _, k := range entityKinds {
		t.Run("import copies a "+k.name+" folder between stores", func(t *testing.T) {
			s, a, b := newStores(t)
			k.contractImport(t, s, a, b)
		})
	}
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

func sameItem(a, b item.Item) bool {
	a.Created, b.Created = item.Stamp{}, item.Stamp{}
	return reflect.DeepEqual(a, b)
}

func setTitle(id item.ID, from, to string) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewSet("title", from, to))
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

// entityWriter は、ItemWriter と ProjectWriter に共通の口(Import・Remove を同じ照合で確かめる)。
type entityWriter interface {
	Append(id change.ID, drafts ...change.Draft) error
	Describe(id change.ID, text string) error
	Import(id change.ID, log []change.Record, description string) error
	Remove(id change.ID) error
}

// entityKind は、アイテムとプロジェクトの違い(Import・Remove の照合を、両方で回す)。
type entityKind struct {
	name      string
	kind      change.Kind
	id, newID change.ID // 準備のデータにある ID と、まだない ID
	write     func(tx port.Tx) entityWriter
	create    func(id change.ID) change.Draft
	desc      func(r port.Reader) (string, error)
	count     func(r port.Reader) (int, error) // 準備のデータの親の下にある数
	then      []change.Draft                   // 写したあとに同じ Tx で足す行
	applied   func(r port.Reader) bool         // then の行が読み取りに効いている
}

var (
	movedPlace  = project.Place{Workspace: cWorkspace.ID, Number: 2}
	movedIn     = transfer.NewMovedIn(transfer.MovedInInfo{Old: "HH-1", FromStore: "other", At: time.Unix(0, 0).UTC()})
	entityKinds = []entityKind{{
		name: "item", kind: change.KindItem, id: cItem.ID, newID: "01CONTRACTIT00000000000002",
		write:  func(tx port.Tx) entityWriter { return tx.WriteItems() },
		create: func(id change.ID) change.Draft { c := cItem; c.ID = id; return item.Create(c) },
		desc:   func(r port.Reader) (string, error) { return r.Items().Description(cItem.ID) },
		count:  func(r port.Reader) (int, error) { l, err := r.Items().List(cProject.ID); return len(l), err },
		then:   []change.Draft{change.NewSet("title", "認証", "移動先で変えた")},
		applied: func(r port.Reader) bool {
			it, err := r.Items().Get(cItem.ID)
			return err == nil && it.Title == "移動先で変えた"
		},
	}, {
		name: "project", kind: change.KindProject, id: cProject.ID, newID: "01CONTRACTPR00000000000002",
		write:  func(tx port.Tx) entityWriter { return tx.WriteProjects() },
		create: func(id change.ID) change.Draft { c := cProject; c.ID = id; return project.Create(c) },
		desc:   func(r port.Reader) (string, error) { return r.Projects().Description(cProject.ID) },
		count:  func(r port.Reader) (int, error) { l, err := r.Projects().List(cWorkspace.ID); return len(l), err },
		then:   []change.Draft{movedIn, change.NewSet("place", cProject.Place, movedPlace)},
		applied: func(r port.Reader) bool {
			p, err := r.Projects().Get(cProject.ID)
			return err == nil && p.Place == movedPlace
		},
	}}
)

// contractImport は、別の Store のフォルダの行と説明を、行を変えずに写す。続けて書く行は、写した行の先端につながる。
func (k entityKind) contractImport(t *testing.T, s port.Stores, a, b port.StoreID) {
	ref := change.Ref{Kind: k.kind, ID: k.id}
	mustWrite(t, s, a, taro, writeFixture)
	mustWrite(t, s, a, taro, func(tx port.Tx) error { return k.write(tx).Describe(k.id, "説明") })
	var log []change.Record
	var text string
	mustRead(t, s, a, func(r port.Reader) (err error) {
		if log, err = r.Changes().Of(ref); err == nil {
			text, err = k.desc(r)
		}
		return
	})
	mustWrite(t, s, b, taro, func(tx port.Tx) error {
		w := k.write(tx)
		if err := w.Import(k.id, log, text); err != nil {
			return err
		}
		return w.Append(k.id, k.then...)
	})
	mustRead(t, s, b, func(r port.Reader) error {
		got, err := r.Changes().Of(ref)
		d, _ := k.desc(r)
		if err != nil || len(got) != len(log)+len(k.then) || d != "説明" || !k.applied(r) || !slices.Equal(got[len(log)].Prev, []change.ID{log[len(log)-1].ID}) {
			t.Fatalf("%s: rows %d (want %d), %v, description %q, applied %v", k.name, len(got), len(log)+len(k.then), err, d, k.applied(r))
		}
		for i, w := range log {
			if !sameRow(got[i], w) {
				t.Errorf("%s: row %d = %+v, want %+v (行は変えずに写す)", k.name, i, got[i], w)
			}
		}
		return nil
	})
	for name, call := range map[string]func(w entityWriter) error{
		"existing folder": func(w entityWriter) error { return w.Import(k.id, log, "") },
		"empty log":       func(w entityWriter) error { return w.Import(k.newID, nil, "") },
		"bad id":          func(w entityWriter) error { return w.Import("bad id", log, "") },
	} {
		if err := s.Write(t.Context(), b, taro, func(tx port.Tx) error { return call(k.write(tx)) }); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import %s (%s) = %v, want ErrInvalidOperation", k.name, name, err)
		}
	}
}

// Remove だけの Write も Summary が要る。同じ Tx で行を足してから消しても、もうない。消したものにも、同じ Tx で作ったものにも
// Import できない。アイテムとプロジェクトの両方で確かめる。
func contractRemoveImportRules(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	bad := func(k entityKind, what string, err error) {
		if !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("%s: %s = %v, want ErrInvalidOperation", k.name, what, err)
		}
	}
	for _, k := range entityKinds {
		noSummary := port.Operation{Actor: change.User("taro")}
		bad(k, "Remove without a summary", s.Write(t.Context(), id, noSummary, func(tx port.Tx) error { return k.write(tx).Remove(k.id) }))
		mustWrite(t, s, id, taro, func(tx port.Tx) error {
			w := k.write(tx)
			if err := errors.Join(w.Append(k.id, k.then[len(k.then)-1]), w.Remove(k.id)); err != nil {
				return err
			}
			if _, err := tx.Changes().Of(change.Ref{Kind: k.kind, ID: k.id}); !errors.Is(err, port.ErrNotFound) {
				t.Errorf("%s: Changes().Of after Append and Remove = %v, want ErrNotFound", k.name, err)
			}
			if n, err := k.count(tx); err != nil || n != 0 {
				t.Errorf("%s: List after Append and Remove = %d, %v", k.name, n, err)
			}
			bad(k, "Import after Remove", w.Import(k.id, []change.Record{{ID: "x"}}, ""))
			return nil
		})
		bad(k, "Import after Append in the same write", s.Write(t.Context(), id, taro, func(tx port.Tx) error {
			w := k.write(tx)
			return errors.Join(w.Append(k.newID, k.create(k.newID)), w.Import(k.newID, []change.Record{{ID: "x"}}, ""))
		}))
	}
}

// sameRow は、写した行が元の行と、全部の項目で同じことを確かめる。
func sameRow(g, w change.Record) bool {
	return g.ID == w.ID && g.Op == w.Op && g.At.Equal(w.At) && g.By == w.By && g.Actor == w.Actor && g.T == w.T && g.F == w.F &&
		string(g.From) == string(w.From) && string(g.To) == string(w.To) && slices.Equal(g.Prev, w.Prev)
}
