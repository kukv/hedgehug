package store

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestCreateWritesWorkspace(t *testing.T) {
	s := newTestStore(t)
	var got []*model.Workspace
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, w)
	}
	if len(got) != 1 || got[0].KeyPrefix != "TST" || got[0].Name != "test" || got[0].PrefixSet.IsZero() {
		t.Fatalf("workspaces = %+v", got)
	}
}

func TestPutAndRemoveWorkspace(t *testing.T) {
	s := newTestStore(t)
	w := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(w) }); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Workspace(w.ID); err != nil || got.KeyPrefix != "API" {
		t.Fatalf("Workspace = %+v %v", got, err)
	}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveWorkspace(w.ID) }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Workspace(w.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after remove: %v", err)
	}
}

func TestPurgeRecordsNumbers(t *testing.T) {
	s := newTestStore(t)
	task := &model.Task{ID: model.NewID(), Workspace: testWorkspace(t, s).ID, Kind: "task", Title: "x", Status: "todo", Key: "TST-9", FormerKeys: []string{"OLD-12"}}
	if err := put(s, task); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(task.ID) }); err != nil {
		t.Fatal(err)
	}
	n, err := s.Numbers()
	if err != nil || n["TST"] != 9 || n["OLD"] != 12 {
		t.Errorf("Numbers = %v %v", n, err)
	}
}
