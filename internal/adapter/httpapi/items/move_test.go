package items_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
)

const secondProject = "01TESTPR000000000000000002"

// addSecondProject は、サンプルのワークスペースに 2 つ目のプロジェクト HH-P2「決済」を足す。
func addSecondProject(t *testing.T, s port.Stores) {
	t.Helper()
	pr := project.Project{ID: secondProject, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "決済", Status: project.Todo}
	op := port.Operation{Actor: change.User("taro"), Summary: "プロジェクトを作成"}
	if err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Projects, 2)),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
		)
	}); err != nil {
		t.Fatal(err)
	}
}

// createItem は、アイテムを作って内部 ID を返す。
func createItem(t *testing.T, api humatest.TestAPI, parent, kind, title string) string {
	t.Helper()
	body := map[string]any{"project": string(apptest.SampleProject), "kind": kind, "title": title}
	if parent != "" {
		body["parent"] = parent
	}
	res := api.Post("/api/stores/S1/items", body)
	var it struct{ ID string }
	if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &it) != nil {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	return it.ID
}

func TestMoveCandidatesAndMove(t *testing.T) {
	api := newAPI(t)
	epic2 := createItem(t, api, "", "Epic", "決済") // HH-3
	task := string(apptest.SampleTask)
	res := api.Get("/api/stores/S1/items/" + task + "/move-candidates")
	want := `[{"kind":"item","id":"` + epic2 + `","display":"HH-3","title":"決済","projectId":"` + string(apptest.SampleProject) + `","projectTitle":"ログイン"}]`
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("candidates: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleEpic) + "/move-candidates"); res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != "[]" {
		t.Errorf("epic candidates: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/"+task+"/move", map[string]any{"parent": epic2}); res.Code != http.StatusNoContent {
		t.Fatalf("move: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/" + task); !strings.Contains(res.Body.String(), `"parent":"`+epic2+`"`) {
		t.Errorf("moved: %s", res.Body)
	}
}

func TestMoveAnEpicToAnotherProject(t *testing.T) {
	api := newAPI(t, addSecondProject)
	epic := "/api/stores/S1/items/" + string(apptest.SampleEpic)
	res := api.Get(epic + "/move-candidates")
	want := `[{"kind":"project","id":"` + secondProject + `","display":"HH-P2","title":"決済","projectId":"` + secondProject + `","projectTitle":""}]`
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("candidates: %d %s", res.Code, res.Body)
	}
	if res := api.Post(epic+"/move", map[string]any{"project": secondProject}); res.Code != http.StatusNoContent {
		t.Fatalf("move: %d %s", res.Code, res.Body)
	}
	if res := api.Get(epic); !strings.Contains(res.Body.String(), `"project":"`+secondProject+`"`) {
		t.Errorf("moved: %s", res.Body)
	}
}

func TestMoveRefusalsAndNotFound(t *testing.T) {
	api := newAPI(t)
	task := string(apptest.SampleTask)
	res := api.Post("/api/stores/S1/items/"+task+"/move", map[string]any{"parent": task + "x"})
	if res.Code != http.StatusNotFound {
		t.Errorf("unknown parent: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items/"+string(apptest.SampleEpic)+"/move", map[string]any{"parent": task})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "Epic は Task の下に置けません") {
		t.Errorf("refusal: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/"+task+"/move", map[string]any{}); res.Code != http.StatusNotFound {
		t.Errorf("empty body: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/items/01NOPE00000000000000000000/move-candidates"); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
}
