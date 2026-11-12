package project_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

func TestCreateAndFromLog(t *testing.T) {
	p := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Title: "ログイン"}
	log := change.Stamp(nil, []change.Draft{project.Create(p)}, change.Meta{Op: "o", At: time.Unix(0, 0), Actor: change.User("taro")}, func() change.ID { return "c" })
	got, ok := project.FromLog(log)
	if !ok || got != p {
		t.Errorf("FromLog = %+v, %v", got, ok)
	}
	if got.DisplayID("HH") != "HH-P1" {
		t.Errorf("DisplayID = %s", got.DisplayID("HH"))
	}
}
