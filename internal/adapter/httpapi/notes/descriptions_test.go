package notes_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

const (
	taskDescription    = "/api/stores/S1/items/" + string(apptest.SampleTask) + "/description"
	projectDescription = "/api/stores/S1/projects/" + string(apptest.SampleProject) + "/description"
)

func TestItemDescriptionRoundTripAndConflict(t *testing.T) {
	api := newAPI(t)
	if res := api.Get(taskDescription); res.Code != http.StatusOK || res.Body.String() != `{"text":""}`+"\n" {
		t.Fatalf("empty: %d %s", res.Code, res.Body)
	}
	res := api.Put(taskDescription, map[string]any{"base": "", "text": "a\nb\n"})
	if res.Code != http.StatusOK || res.Body.String() != `{"text":"a\nb\n","conflict":false}`+"\n" {
		t.Fatalf("save: %d %s", res.Code, res.Body)
	}
	if res := api.Get(taskDescription); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"a\nb\n"`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
	// 別の人が 1 行目を変えたあとに、編集を始めたときの説明 "a\nb\n" から 1 行目を変えて保存すると、両方を残す。
	if res := api.Put(taskDescription, map[string]any{"base": "a\nb\n", "text": "X\nb\n"}); res.Code != http.StatusOK {
		t.Fatalf("other: %d %s", res.Code, res.Body)
	}
	res = api.Put(taskDescription, map[string]any{"base": "a\nb\n", "text": "Y\nb\n"})
	if res.Code != http.StatusOK || res.Body.String() != `{"text":"X\n-----\nY\nb\n","conflict":true}`+"\n" {
		t.Errorf("conflict: %d %s", res.Code, res.Body)
	}
}

func TestProjectDescription(t *testing.T) {
	api := newAPI(t)
	res := api.Put(projectDescription, map[string]any{"base": "", "text": "概要"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"概要"`) {
		t.Fatalf("save: %d %s", res.Code, res.Body)
	}
	if res := api.Get(projectDescription); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"text":"概要"`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
}

func TestDescriptionOfUnknownIsNotFound(t *testing.T) {
	api := newAPI(t)
	const (
		item    = "/api/stores/S1/items/01NOPE00000000000000000000/description"
		project = "/api/stores/S1/projects/01NOPE00000000000000000000/description"
	)
	if res := api.Get(item); res.Code != http.StatusNotFound {
		t.Errorf("get item: %d", res.Code)
	}
	if res := api.Put(item, map[string]any{"base": "", "text": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("put item: %d", res.Code)
	}
	if res := api.Get(project); res.Code != http.StatusNotFound {
		t.Errorf("get project: %d", res.Code)
	}
	if res := api.Put(project, map[string]any{"base": "", "text": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("put project: %d", res.Code)
	}
}

func TestDescribeNeedsBaseAndText(t *testing.T) {
	if res := newAPI(t).Put(taskDescription, map[string]any{"text": "x"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("without base: %d %s", res.Code, res.Body)
	}
}
