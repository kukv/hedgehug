package apptest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// contractItemValues は、アイテムの値(ステータス・優先度・関連システム・選択肢)と、ワークスペースの
// 削除の行が、書いたとおりに読めることを確かめる。
func contractItemValues(t *testing.T, s port.Stores, id port.StoreID) {
	const (
		st   change.ID = "01CONTRACTST00000000000001"
		pr   change.ID = "01CONTRACTPI00000000000001"
		sy   change.ID = "01CONTRACTSY00000000000001"
		prop change.ID = "01CONTRACTPP00000000000001"
		ch   change.ID = "01CONTRACTCH00000000000001"
		note change.ID = "01CONTRACTNT00000000000001"
	)
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(cWorkspace.ID, change.NewDelete("statuses/"+string(st))); err != nil {
			return err
		}
		return tx.WriteItems().Append(cItem.ID,
			change.NewSet("status", nil, st), change.NewSet("priority", nil, pr),
			change.NewAdd("systems", sy), change.NewAdd(item.FieldKey(prop), ch),
			change.NewSet("assignee", nil, "taro"), change.NewSet("end", nil, "2026-11-15"),
			change.NewSet("estimate", nil, 1.5), change.NewSet(item.FieldKey(note), nil, "memo"))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		it, err := r.Items().Get(cItem.ID)
		if err != nil || it.Status != st || it.Priority != pr ||
			!reflect.DeepEqual(it.Systems, []change.ID{sy}) || !reflect.DeepEqual(it.Choices[prop], []change.ID{ch}) {
			t.Errorf("item = %+v, %v", it, err)
		}
		if err != nil || it.Assignee != "taro" || it.End != "2026-11-15" || it.Estimate == nil || *it.Estimate != 1.5 ||
			string(it.Fields[note]) != `"memo"` {
			t.Errorf("item values = %+v, %v", it, err)
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		if err != nil || log[len(log)-1].T != change.Delete || log[len(log)-1].F != "statuses/"+string(st) {
			t.Errorf("workspace log = %+v, %v", log, err)
		}
		return nil
	})
}

// contractSettings は、設定を含めて作ったワークスペースの設定が、そのまま読めることを確かめる。
func contractSettings(t *testing.T, s port.Stores, id port.StoreID) {
	n := 0
	settings := workspace.DefaultSettings(func() change.ID {
		n++
		return change.ID("01CONTRACTVAL" + string(rune('A'+n)) + "000000000000")
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(cWorkspace, settings)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(cWorkspace.ID, d)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Workspaces().Settings(cWorkspace.ID)
		if err != nil || !reflect.DeepEqual(got, settings) {
			t.Errorf("Settings = %+v, %v", got, err)
		}
		if _, err := r.Workspaces().Settings("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("unknown workspace: %v", err)
		}
		return nil
	})
}

// wantDescription は、説明の読み取りの結果が、エラーなしで want であることを確かめる。
func wantDescription(t *testing.T, label, got string, err error, want string) {
	t.Helper()
	if err != nil || got != want {
		t.Errorf("%s = %q, %v; want %q", label, got, err, want)
	}
}

// contractDescription は、説明(アイテムとプロジェクト)が、書いたとおりに読め、空にすると消えることを確かめる。
func contractDescription(t *testing.T, s port.Stores, id port.StoreID) {
	const body = "## 手順\n1. 確認\n"
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "a new item's description", got, err, "")
		return nil
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Describe(cItem.ID, body); err != nil {
			return err
		}
		got, err := tx.Items().Description(cItem.ID)
		wantDescription(t, "a tx must see its own description", got, err, body)
		return errors.Join(
			tx.WriteItems().Append(cItem.ID, change.NewDesc()),
			tx.WriteProjects().Describe(cProject.ID, "概要"),
			tx.WriteProjects().Append(cProject.ID, change.NewDesc()),
		)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "item description", got, err, body)
		got, err = r.Projects().Description(cProject.ID)
		wantDescription(t, "project description", got, err, "概要")
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || log[len(log)-1].T != change.Desc {
			t.Errorf("item log = %+v, %v", log, err)
		}
		return nil
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(cItem.ID, ""), tx.WriteItems().Append(cItem.ID, change.NewDesc()))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "an emptied description", got, err, "")
		got, err = r.Projects().Description(cProject.ID)
		wantDescription(t, "the other description must stay", got, err, "概要")
		return nil
	})
}

// contractDescriptionRollback は、失敗した Write が説明を残さず、同じ Write の中で書いて戻した説明も残さないことを確かめる。
func contractDescriptionRollback(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	boom := errors.New("boom")
	err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(cItem.ID, "消える"), boom)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write = %v", err)
	}
	// 書いてから同じ説明(空)に戻した Write は、何も残さず、Summary も確かめない。
	mustWrite(t, s, id, port.Operation{Actor: taro.Actor}, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Describe(cItem.ID, "一時"), tx.WriteItems().Describe(cItem.ID, ""))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "description", got, err, "")
		return nil
	})
}

// contractDescriptionRules は、ないアイテムと使えない ID、Summary のない説明の書き換えを断ることを確かめる。
func contractDescriptionRules(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	const missing = "01CONTRACTNO00000000000001"
	mustRead(t, s, id, func(r port.Reader) error {
		if _, err := r.Items().Description(missing); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("unknown item: %v", err)
		}
		if _, err := r.Projects().Description("../x"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("malformed id: %v", err)
		}
		return nil
	})
	err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Describe(missing, "x") })
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Describe unknown = %v", err)
	}
	err = s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Describe("../x", "x") })
	if !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Describe malformed = %v", err)
	}
	err = s.Write(t.Context(), id, port.Operation{Actor: taro.Actor}, func(tx port.Tx) error {
		return tx.WriteItems().Describe(cItem.ID, "要約なし")
	})
	if !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Describe without a summary = %v", err)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Items().Description(cItem.ID)
		wantDescription(t, "a refused write must leave the description", got, err, "")
		return nil
	})
}
