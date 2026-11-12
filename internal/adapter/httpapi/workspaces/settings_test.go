package workspaces_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

type detailBody struct {
	Settings struct {
		InitialStatus   string `json:"initialStatus"`
		InitialPriority string `json:"initialPriority"`
		Priorities      []struct {
			ID string `json:"id"`
		} `json:"priorities"`
	} `json:"settings"`
}

func getDetail(t *testing.T, api humatest.TestAPI, ws string) detailBody {
	t.Helper()
	res := api.Get(base + "/" + ws)
	var d detailBody
	if err := json.Unmarshal(res.Body.Bytes(), &d); err != nil {
		t.Fatalf("detail: %v %s", err, res.Body)
	}
	return d
}

func idOf(t *testing.T, body []byte) string {
	t.Helper()
	var v struct{ ID string }
	if err := json.Unmarshal(body, &v); err != nil || v.ID == "" {
		t.Fatalf("body: %v %s", err, body)
	}
	return v.ID
}

// プロパティを足して選択肢を足し、詳細に出る。断りは 422 と文言。
func TestPropertyAndChoicesAPI(t *testing.T) {
	api := newAPI(t)
	ws := create(t, api, "Web", "HH")
	res := api.Post(base+"/"+ws+"/properties", map[string]any{"name": "顧客", "key": "customer", "type": "single", "kinds": []string{}})
	if res.Code != http.StatusCreated {
		t.Fatalf("add property: %d %s", res.Code, res.Body)
	}
	p := idOf(t, res.Body.Bytes())
	if res := api.Post(base+"/"+ws+"/lists/"+p+"/values", map[string]any{"name": "A社"}); res.Code != http.StatusCreated {
		t.Fatalf("add choice: %d %s", res.Code, res.Body)
	}
	res = api.Post(base+"/"+ws+"/properties", map[string]any{"name": "顧客", "key": "c2", "type": "text", "kinds": []string{}})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "同じ名前のプロパティがあります") {
		t.Errorf("duplicate: %d %s", res.Code, res.Body)
	}
	res = api.Get(base + "/" + ws)
	if !strings.Contains(res.Body.String(), `"choices":[{"id"`) || !strings.Contains(res.Body.String(), `"name":"A社"`) {
		t.Errorf("detail: %s", res.Body)
	}
	if !strings.Contains(res.Body.String(), `"choices":[]`) {
		t.Errorf("choices of other properties must be []: %s", res.Body)
	}
}

// プロパティの名前・使う種類・非表示・並べ替え・削除は 204。
func TestPropertyEditsAPI(t *testing.T) {
	api := newAPI(t)
	ws := create(t, api, "Web", "HH")
	p := idOf(t, api.Post(base+"/"+ws+"/properties", map[string]any{"name": "メモ", "key": "memo", "type": "text", "kinds": []string{}}).Body.Bytes())
	pp := base + "/" + ws + "/properties/" + p
	for name, r := range map[string]int{
		"rename": api.Put(pp+"/name", map[string]any{"name": "備考"}).Code,
		"kinds":  api.Put(pp+"/kinds", map[string]any{"kinds": []string{"Task"}}).Code,
		"hidden": api.Put(base+"/"+ws+"/properties/points/hidden", map[string]any{"hidden": true}).Code, // 非表示は組み込みだけ
		"move":   api.Post(pp+"/move", map[string]any{"direction": "up"}).Code,
	} {
		if r != http.StatusNoContent {
			t.Errorf("%s: %d", name, r)
		}
	}
	if res := api.Post(pp+"/move", map[string]any{"direction": "left"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("direction enum: %d", res.Code)
	}
	if res := api.Delete(pp); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
}

// ステータスを足して消すと、件数と詳細が変わる。初期ステータスは消せない。
func TestValuesAPI(t *testing.T) {
	api := newAPI(t)
	ws := create(t, api, "Web", "HH")
	res := api.Post(base+"/"+ws+"/lists/statuses/values", map[string]any{"name": "Review"})
	if res.Code != http.StatusCreated {
		t.Fatalf("add value: %d %s", res.Code, res.Body)
	}
	v := idOf(t, res.Body.Bytes())
	vv := base + "/" + ws + "/lists/statuses/values/" + v
	for name, r := range map[string]int{
		"rename":   api.Put(vv+"/name", map[string]any{"name": "Reviewing"}).Code,
		"color":    api.Put(vv+"/color", map[string]any{"color": "purple"}).Code,
		"category": api.Put(base+"/"+ws+"/statuses/"+v+"/category", map[string]any{"category": "doing"}).Code,
		"move":     api.Post(vv+"/move", map[string]any{"direction": "up"}).Code,
		"usage":    api.Get(base + "/" + ws + "/usage").Code,
	} {
		if r >= 300 {
			t.Errorf("%s: %d", name, r)
		}
	}
	if res := api.Put(vv+"/color", map[string]any{"color": "black"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("enum: %d", res.Code)
	}
	if res := api.Delete(vv); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	d := getDetail(t, api, ws)
	res = api.Delete(base + "/" + ws + "/lists/statuses/values/" + d.Settings.InitialStatus)
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "初期ステータスで、未着手の既定のステータスでもあります") {
		t.Errorf("initial: %d %s", res.Code, res.Body)
	}
}

// 初期値と分類の既定は 204。ない値は 404。件数は空でも [] と {}。
func TestInitialsAndUsageAPI(t *testing.T) {
	api := newAPI(t)
	ws := create(t, api, "Web", "HH")
	d := getDetail(t, api, ws)
	if len(d.Settings.Priorities) < 2 {
		t.Fatalf("priorities: %+v", d.Settings.Priorities)
	}
	pr := d.Settings.Priorities[1].ID
	if res := api.Put(base+"/"+ws+"/initial-priority", map[string]any{"value": pr}); res.Code != http.StatusNoContent {
		t.Errorf("initial priority: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/initial-status", map[string]any{"value": d.Settings.InitialStatus}); res.Code != http.StatusNoContent {
		t.Errorf("initial status: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/category-defaults/todo", map[string]any{"value": d.Settings.InitialStatus}); res.Code != http.StatusNoContent {
		t.Errorf("category default: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/initial-status", map[string]any{"value": "nope"}); res.Code != http.StatusNotFound {
		t.Errorf("missing value: %d %s", res.Code, res.Body)
	}
	res := api.Get(base + "/" + ws + "/usage")
	for _, want := range []string{`"items":0`, `"firstNumbers":[]`, `"values":{}`, `"properties":{}`} {
		if !strings.Contains(res.Body.String(), want) {
			t.Errorf("usage want %s: %s", want, res.Body)
		}
	}
	if res := api.Post(base+"/"+ws+"/lists/nope/values", map[string]any{"name": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("list of non-select property: %d %s", res.Code, res.Body)
	}
}

// ワークスペースの並べ替えは 204。
func TestMoveWorkspaceAPI(t *testing.T) {
	api := newAPI(t)
	create(t, api, "A", "AA")
	b := create(t, api, "B", "BB")
	if res := api.Post(base+"/"+b+"/move", map[string]any{"direction": "up"}); res.Code != http.StatusNoContent {
		t.Errorf("move: %d %s", res.Code, res.Body)
	}
	res := api.Get(base)
	if strings.Index(res.Body.String(), `"name":"B"`) > strings.Index(res.Body.String(), `"name":"A"`) {
		t.Errorf("order: %s", res.Body)
	}
}
