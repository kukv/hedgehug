package apptest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
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

func contractLists(t *testing.T, s port.Stores, id port.StoreID) {
	early := project.Project{ID: "01CONTRACTPR00000000000000", Place: project.Place{Workspace: cWorkspace.ID, Number: 2}, Rank: "A", Title: "先頭", Status: project.Todo}
	task := item.Item{ID: "01CONTRACTIT00000000000002", Place: item.Place{Workspace: cWorkspace.ID, Project: cProject.ID, Number: 3}, Kind: item.Epic, Rank: "W", Title: "あと"}
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteProjects().Append(early.ID, project.Create(early)); err != nil {
			return err
		}
		return tx.WriteItems().Append(task.ID, item.Create(task))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		ps, err := r.Projects().List(cWorkspace.ID)
		if err != nil || len(ps) != 2 || ps[0] != early || ps[1] != cProject {
			t.Errorf("Projects().List = %+v, %v", ps, err)
		}
		its, err := r.Items().List(cProject.ID)
		if err != nil || len(its) != 2 || !sameItem(its[0], cItem) || !sameItem(its[1], task) {
			t.Errorf("Items().List = %+v, %v", its, err)
		}
		if none, err := r.Items().List("01NOPE00000000000000000000"); err != nil || len(none) != 0 {
			t.Errorf("Items().List(unknown) = %+v, %v", none, err)
		}
		return nil
	})
}
