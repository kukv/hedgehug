package project_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

func TestFromLogReadsTheDeletionMark(t *testing.T) {
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	p := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	log := change.Stamp(nil, []change.Draft{project.Create(p)}, change.Meta{Op: "o1", At: at, Actor: change.User("taro")}, func() change.ID { return "r1" })
	if got, ok := project.FromLog(log); !ok || got.Deleted != nil {
		t.Fatalf("live project = %+v, %v", got, ok)
	}
	del := change.Stamp(log, []change.Draft{change.NewDeleteEntity(true)}, change.Meta{Op: "o2", At: at.Add(time.Hour), Actor: change.User("taro")}, func() change.ID { return "r2" })
	got, ok := project.FromLog(append(log, del...))
	if !ok || got.Deleted == nil || got.Deleted.Op != "o2" || !got.Deleted.Root || got.Title != "ログイン" {
		t.Errorf("deleted project = %+v, %v", got, ok)
	}
}

func TestLive(t *testing.T) {
	ps := []project.Project{{ID: "P1"}, {ID: "P2", Deleted: &change.Deletion{Op: "o"}}, {ID: "P3"}}
	got := project.Live(ps)
	if len(got) != 2 || got[0].ID != "P1" || got[1].ID != "P3" || len(ps) != 3 {
		t.Errorf("Live = %+v", got)
	}
}
