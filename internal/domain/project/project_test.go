package project_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

func TestCreateAndFromLog(t *testing.T) {
	p := project.Project{
		ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン",
		Owner: "taro", Status: project.InProgress, Start: "2026-11-01", End: "2026-11-30",
	}
	log := change.Stamp(nil, []change.Draft{project.Create(p)}, change.Meta{Op: "o", At: time.Unix(0, 0), Actor: change.User("taro")}, func() change.ID { return "c" })
	got, ok := project.FromLog(log)
	if !ok || got != p {
		t.Errorf("FromLog = %+v, %v", got, ok)
	}
	if got.DisplayID("HH") != "HH-P1" {
		t.Errorf("DisplayID = %s", got.DisplayID("HH"))
	}
}

func TestValidStatus(t *testing.T) {
	for _, s := range []project.Status{project.Todo, project.InProgress, project.Done, project.Pending, project.Cancel} {
		if !project.ValidStatus(s) {
			t.Errorf("%s is not valid", s)
		}
	}
	if project.ValidStatus("Review") {
		t.Error("Review is valid")
	}
}
