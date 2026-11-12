package apptest

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

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
