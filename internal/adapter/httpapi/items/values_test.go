package items_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

const taskPath = "/api/stores/S1/items/" + string(apptest.SampleTask)

func TestSetItemPropertyAPI(t *testing.T) {
	api := newAPI(t)
	res := api.Put(taskPath+"/properties/points", map[string]any{"value": "3"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"points":3`) {
		t.Fatalf("PUT = %d %s", res.Code, res.Body)
	}
	res = api.Put(taskPath+"/properties/points", map[string]any{"value": "-1"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "ポイントは 0 以上の数で入力してください") {
		t.Errorf("PUT -1 = %d %s", res.Code, res.Body)
	}
}

// value は any。真偽・null・配列も届く(決定 12)。
func TestSetItemPropertyValueShapes(t *testing.T) {
	api := newAPI(t)
	if res := api.Put(taskPath+"/properties/assignee", map[string]any{"value": "taro"}); !strings.Contains(res.Body.String(), `"assignee":"taro"`) {
		t.Errorf("string: %d %s", res.Code, res.Body)
	}
	if res := api.Put(taskPath+"/properties/assignee", map[string]any{"value": nil}); !strings.Contains(res.Body.String(), `"assignee":""`) {
		t.Errorf("null: %d %s", res.Code, res.Body)
	}
	if res := api.Put(taskPath+"/properties/start", map[string]any{"value": true}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("bool for a date: %d %s", res.Code, res.Body)
	}
	if res := api.Put(taskPath+"/properties/systems", map[string]any{"value": []string{string(apptest.SampleSystemWeb)}}); res.Code != http.StatusNotFound {
		t.Errorf("array for systems must use the toggle API: %d %s", res.Code, res.Body)
	}
}

func TestToggleAndCheckAPI(t *testing.T) {
	api := newAPI(t)
	path := taskPath + "/properties/systems/values/" + string(apptest.SampleSystemWeb)
	if res := api.Post(path); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), string(apptest.SampleSystemWeb)) {
		t.Errorf("POST = %d %s", res.Code, res.Body)
	}
	if res := api.Delete(path); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"systems":[]`) {
		t.Errorf("DELETE = %d %s", res.Code, res.Body)
	}
	if res := api.Post(taskPath+"/check", map[string]any{"done": true}); res.Code != http.StatusNotFound {
		t.Errorf("check a Task = %d", res.Code)
	}
}

func TestItemHasValues(t *testing.T) {
	api := newAPI(t)
	res := api.Get(taskPath)
	for _, s := range []string{`"status":"` + string(apptest.SampleStatusTodo) + `"`, `"assignee":""`, `"points":null`, `"systems":[]`, `"choices":{}`, `"fields":{}`} {
		if !strings.Contains(res.Body.String(), s) {
			t.Errorf("GET has no %s: %s", s, res.Body)
		}
	}
}

func TestCreateWithValues(t *testing.T) {
	api := newAPI(t)
	body := func(v map[string]any) map[string]any {
		return map[string]any{"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic), "kind": "Task", "title": "値つき", "values": v}
	}
	res := api.Post("/api/stores/S1/items", body(map[string]any{"points": "5", "status": string(apptest.SampleStatusInProgress)}))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"points":5`) || !strings.Contains(res.Body.String(), `"status":"`+string(apptest.SampleStatusInProgress)+`"`) {
		t.Errorf("create with values = %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items", body(map[string]any{"points": "-1"}))
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "ポイントは 0 以上の数で入力してください") {
		t.Errorf("create with bad value = %d %s", res.Code, res.Body)
	}
}
