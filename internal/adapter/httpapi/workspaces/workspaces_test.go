package workspaces_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/workspaces"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appworkspaces "github.com/kukv/hedgehug/internal/app/workspaces"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	_, api := humatest.New(t)
	workspaces.Register(api, appworkspaces.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}})
	return api
}

const base = "/api/stores/S1/workspaces"

func create(t *testing.T, api humatest.TestAPI, name, prefix string) string {
	t.Helper()
	res := api.Post(base, map[string]string{"name": name, "prefix": prefix})
	if res.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	var w struct{ ID string }
	if err := json.Unmarshal(res.Body.Bytes(), &w); err != nil || w.ID == "" {
		t.Fatalf("create body: %v %s", err, res.Body)
	}
	return w.ID
}

func TestCreateListAndGet(t *testing.T) {
	api := newAPI(t)
	res := api.Post(base, map[string]string{"name": "Web", "prefix": "hh"})
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"prefix":"HH"`) {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	var created struct{ ID string }
	_ = json.Unmarshal(res.Body.Bytes(), &created)
	res = api.Get(base)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"Web"`) {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	res = api.Get(base + "/" + created.ID)
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"id":"`+created.ID+`"`) || !strings.Contains(body, `"name":"Web"`) ||
		!strings.Contains(body, `"prefix":"HH"`) || !strings.Contains(body, `"oldPrefixes":[]`) || !strings.Contains(body, `"initialStatus"`) ||
		!strings.Contains(body, `"name":"InProgress"`) {
		t.Errorf("get: %d %s", res.Code, body)
	}
}

func TestRefusalIs422WithTheSpecText(t *testing.T) {
	api := newAPI(t)
	create(t, api, "Web", "HH")
	res := api.Post(base, map[string]string{"name": "Web", "prefix": "WEB"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "この Store には同じ名前のワークスペースがあります") {
		t.Errorf("dup: %d %s", res.Code, res.Body)
	}
}

func TestRenameAndPrefix(t *testing.T) {
	api := newAPI(t)
	id := create(t, api, "Web", "HH")
	if res := api.Put(base+"/"+id+"/name", map[string]string{"name": "フロント"}); res.Code != http.StatusNoContent {
		t.Errorf("rename: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+id+"/prefix", map[string]string{"prefix": "WEB"}); res.Code != http.StatusNoContent {
		t.Errorf("prefix: %d %s", res.Code, res.Body)
	}
	body := api.Get(base + "/" + id).Body.String()
	if !strings.Contains(body, `"name":"フロント"`) || !strings.Contains(body, `"oldPrefixes":["HH"]`) {
		t.Errorf("after: %s", body)
	}
}

func TestNotFound(t *testing.T) {
	api := newAPI(t)
	if res := api.Get(base + "/01NOPE00000000000000000000"); res.Code != http.StatusNotFound {
		t.Errorf("unknown workspace: %d", res.Code)
	}
	if res := api.Put(base+"/01NOPE00000000000000000000/name", map[string]string{"name": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("rename unknown workspace: %d", res.Code)
	}
	if res := api.Get("/api/stores/nope/workspaces"); res.Code != http.StatusNotFound {
		t.Errorf("unknown store: %d", res.Code)
	}
}

// 改行の入った名前は 1 行でないので 422 で断る(プロジェクトのタイトルと同じ)。
func TestNewlineInNameIs422(t *testing.T) {
	api := newAPI(t)
	if res := api.Post(base, map[string]string{"name": "Web\nApp", "prefix": "HH"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("create: %d %s", res.Code, res.Body)
	}
	id := create(t, api, "Web", "HH")
	if res := api.Put(base+"/"+id+"/name", map[string]string{"name": "Web\r\nApp"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("rename: %d %s", res.Code, res.Body)
	}
}

func TestRenameAndPrefixRefusalsAre422WithTheSpecText(t *testing.T) {
	api := newAPI(t)
	id := create(t, api, "Web", "HH")
	create(t, api, "App", "AP")
	cases := []struct {
		name, path string
		body       map[string]string
		want       string
	}{
		{"empty name", "/name", map[string]string{"name": ""}, "名前は空にできません"},
		{"duplicate name", "/name", map[string]string{"name": "App"}, "この Store には同じ名前のワークスペースがあります"},
		{"bad prefix shape", "/prefix", map[string]string{"prefix": "1A"}, "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"},
		{"empty prefix", "/prefix", map[string]string{"prefix": ""}, "接頭辞を入力してください"},
		{"duplicate prefix", "/prefix", map[string]string{"prefix": "ap"}, "この Store には同じ接頭辞のワークスペースがあります"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := api.Put(base+"/"+id+c.path, c.body)
			if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), c.want) {
				t.Errorf("%d %s", res.Code, res.Body)
			}
		})
	}
}
