package project_test

import (
	"fmt"
	"reflect"
	"slices"
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
	if !ok || !reflect.DeepEqual(got, p) {
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

// stamp は、drafts を log の続きに、1 秒ずつ進む時刻で書く。
func stamp(log []change.Record, drafts ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: change.ID(fmt.Sprintf("op%d", n)), At: time.Unix(int64(n), 0), Actor: change.User("taro")}
	return append(slices.Clone(log), change.Stamp(log, drafts, meta, func() change.ID {
		n++
		return change.ID(fmt.Sprintf("c%d", n))
	})...)
}

// リポジトリは add・remove の行で持ち、大文字小文字を区別しない辞書順で返す。
func TestFromLogReadsRepos(t *testing.T) {
	p := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	log := stamp(nil, project.Create(p))
	log = stamp(log, change.NewAdd("repos", "example-org/web"), change.NewAdd("repos", "Acme/api"), change.NewAdd("repos", "tmp/x"))
	log = stamp(log, change.NewRemove("repos", "tmp/x"))
	got, ok := project.FromLog(log)
	if want := []string{"Acme/api", "example-org/web"}; !ok || !slices.Equal(got.Repos, want) {
		t.Errorf("Repos = %v, %v; want %v", got.Repos, ok, want)
	}
}

func TestFromLogWithoutReposHasNone(t *testing.T) {
	p := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン", Status: project.Todo}
	got, ok := project.FromLog(stamp(nil, project.Create(p)))
	if !ok || len(got.Repos) != 0 {
		t.Errorf("Repos = %v, %v", got.Repos, ok)
	}
}
