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
