package apptest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
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
		if err != nil || len(ps) != 2 || !reflect.DeepEqual(ps[0], early) || !reflect.DeepEqual(ps[1], cProject) {
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

// contractDeleted は、削除の行(delete・f が deleted)が、アイテム・プロジェクト・ワークスペースの Deleted の印として読め、
// 削除済みも Get と List が返すことを確かめる(A4b-2・A4b-3)。root と op は行のとおりで、下のものは root が false。
func contractDeleted(t *testing.T, s port.Stores, id port.StoreID) {
	parent := cItem.ID
	child := item.Item{ID: "01CONTRACTIT00000000000002", Place: item.Place{Workspace: cWorkspace.ID, Project: cProject.ID, Parent: &parent, Number: 2}, Kind: item.Task, Rank: "a", Title: "下"}
	other := item.Item{ID: "01CONTRACTIT00000000000003", Place: item.Place{Workspace: cWorkspace.ID, Project: cProject.ID, Number: 3}, Kind: item.Epic, Rank: "b", Title: "別"}
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Append(child.ID, item.Create(child)), tx.WriteItems().Append(other.ID, item.Create(other)))
	})
	mustWrite(t, s, id, port.Operation{Actor: change.User("taro"), Summary: "削除"}, func(tx port.Tx) error {
		err := errors.Join(
			tx.WriteItems().Append(cItem.ID, change.NewDeleteEntity(true)),
			tx.WriteItems().Append(child.ID, change.NewDeleteEntity(false)),
		)
		if got, gerr := tx.Items().Get(cItem.ID); gerr != nil || got.Deleted == nil {
			t.Errorf("a tx must see its own delete: %+v, %v", got.Deleted, gerr)
		}
		return err
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteProjects().Append(cProject.ID, change.NewDeleteEntity(true)), tx.WriteWorkspaces().Append(cWorkspace.ID, change.NewDeleteEntity(true)))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		wantDeletedItems(t, r, child.ID)
		wantDeletedProject(t, r)
		wantDeletedWorkspace(t, r)
		return nil
	})
}

// wantDeletedItems は、削除済みのアイテムも List が返し、親(root: true)と子(root: false)が同じ op を持つことを確かめる。
func wantDeletedItems(t *testing.T, r port.Reader, child item.ID) {
	t.Helper()
	its, err := r.Items().List(cProject.ID)
	if err != nil || len(its) != 3 {
		t.Fatalf("Items().List must return deleted items too: %+v, %v", its, err)
	}
	log, _ := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
	del := log[len(log)-1]
	d0, d1, d2 := its[0].Deleted, its[1].Deleted, its[2].Deleted
	if d0 == nil || !d0.Root || d0.Op != del.Op || !d0.At.Equal(del.At) {
		t.Errorf("deleted parent = %+v, want op %s at %s root", d0, del.Op, del.At)
	}
	if d1 == nil || d1.Root || d1.Op != del.Op {
		t.Errorf("deleted child = %+v, want the parent's op and root false", d1)
	}
	if d2 != nil {
		t.Errorf("a live item has Deleted = %+v", d2)
	}
	if got, err := r.Items().Get(child); err != nil || got.Deleted == nil || got.Title != "下" {
		t.Errorf("Items().Get(deleted) = %+v, %v", got, err)
	}
}

func wantDeletedProject(t *testing.T, r port.Reader) {
	t.Helper()
	ps, err := r.Projects().List(cWorkspace.ID)
	if err != nil || len(ps) != 1 || ps[0].Deleted == nil || !ps[0].Deleted.Root {
		t.Errorf("Projects().List must return the deleted project: %+v, %v", ps, err)
	}
	if p, err := r.Projects().Get(cProject.ID); err != nil || p.Deleted == nil || p.Title != "ログイン" {
		t.Errorf("Projects().Get(deleted) = %+v, %v", p, err)
	}
}

// contractProjectRules は、プロジェクト別のルールの行(add・remove・set)が、書いたとおりに読めることを確かめる。
func contractProjectRules(t *testing.T, s port.Stores, id port.StoreID) {
	const a, b change.ID = "01CONTRACTST00000000000001", "01CONTRACTST00000000000002"
	hidden, def := projectrule.HiddenField(cProject.ID), projectrule.TemplateDefaultField(cProject.ID, "Task")
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(cWorkspace.ID, change.NewAdd(hidden, a), change.NewAdd(hidden, b),
			change.NewRemove(hidden, a), change.NewSet(def, nil, projectrule.None))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		want := projectrule.Set{cProject.ID: {Hidden: []change.ID{b}, TemplateDefaults: map[string]string{"Task": projectrule.None}}}
		if got := projectrule.FromLog(log); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("project rules = %+v, %v, want %+v", got, err, want)
		}
		return nil
	})
}
