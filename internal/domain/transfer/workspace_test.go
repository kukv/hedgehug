package transfer_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

func wsProject(id, ws string, deleted bool) project.Project {
	p := project.Project{ID: project.ID(id), Place: project.Place{Workspace: change.ID(ws)}}
	if deleted {
		p.Deleted = &change.Deletion{At: at}
	}
	return p
}

func wsItem(id, proj, ws string, deleted bool) item.Item {
	it := node(id, "", "a", deleted)
	it.Place.Workspace = change.ID(ws)
	it.Place.Project = project.ID(proj)
	return it
}

// workspaceFixture は、W にプロジェクト P1・削除済みの P2、P1 の下に E と削除済みの T、P2 の下に U。
// 別のワークスペース X にプロジェクト PX とアイテム V。
func workspaceFixture() ([]project.Project, []item.Item) {
	projects := []project.Project{wsProject("P1", "W", false), wsProject("P2", "W", true), wsProject("PX", "X", false)}
	items := []item.Item{
		wsItem("E", "P1", "W", false), wsItem("T", "P1", "W", true), wsItem("U", "P2", "W", false), wsItem("V", "PX", "X", false),
	}
	return projects, items
}

func TestCollectWorkspaceTakesAllIncludingDeleted(t *testing.T) {
	projects, items := workspaceFixture()
	s := transfer.CollectWorkspace("W", projects, items)
	var pids []project.ID
	for _, p := range s.Projects {
		pids = append(pids, p.ID)
	}
	if want := []project.ID{"P1", "P2"}; !reflect.DeepEqual(pids, want) {
		t.Errorf("Projects = %v, want %v", pids, want)
	}
	if want := []item.ID{"E", "T", "U"}; !reflect.DeepEqual(ids(s.Items), want) {
		t.Errorf("Items = %v, want %v", ids(s.Items), want)
	}
}

func TestCollectWorkspaceCutsOutside(t *testing.T) {
	projects, items := workspaceFixture()
	items[0] = related(related(items[0], "blocks", "V"), "relates", "U") // E→V(外)、E→U(中どうし)
	items[3] = related(items[3], "blocks", "E")                          // V→E(外から中へ)
	s := transfer.CollectWorkspace("W", projects, items)
	want := []transfer.Cut{{From: "E", To: "V", Type: "blocks"}, {From: "V", To: "E", Type: "blocks"}}
	if !reflect.DeepEqual(s.Outside, want) {
		t.Errorf("Outside = %v, want %v", s.Outside, want)
	}
}

func TestWorkspaceCounts(t *testing.T) {
	projects, items := workspaceFixture()
	got := transfer.CollectWorkspace("W", projects, items).Counts()
	if want := (transfer.Counts{Projects: 1, Items: 1, Deleted: 3}); got != want {
		t.Errorf("Counts = %+v, want %+v", got, want)
	}
}
