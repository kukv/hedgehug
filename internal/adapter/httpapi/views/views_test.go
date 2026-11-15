package views_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	apiviews "github.com/kukv/hedgehug/internal/adapter/httpapi/views"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appviews "github.com/kukv/hedgehug/internal/app/views"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

type memStore struct {
	mu sync.Mutex
	m  map[string]appviews.View
}

func (s *memStore) List(context.Context) ([]appviews.View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []appviews.View
	for _, v := range s.m {
		out = append(out, v)
	}
	return out, nil
}

func (s *memStore) Get(_ context.Context, id string) (appviews.View, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[id]
	return v, ok, nil
}

func (s *memStore) Put(_ context.Context, v appviews.View) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[v.ID] = v
	return nil
}

func (s *memStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type noEvents struct{}

func (noEvents) Publish(port.Event) {}

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	apiviews.Register(api, &appviews.Service{
		Store: &memStore{m: map[string]appviews.View{}},
		IDs:   apptest.NewIDs(),
		Keys:  func(context.Context) (map[string]bool, error) { return map[string]bool{"customer": true}, nil },
		HeldElsewhere: func(_ context.Context, _ port.StoreID, _ workspace.ID, key string) (bool, error) {
			return key == "held", nil
		},
		Events: noEvents{},
	})
	return api
}

type viewBody struct {
	ID      string
	Name    string
	Content struct {
		Query  string
		Layout string
		Table  struct {
			Columns []string
			Group   string
			Sort    *struct {
				Key  string
				Desc bool
			}
		}
	}
}

func content(query string) map[string]any {
	return map[string]any{"query": query, "layout": "table", "table": map[string]any{"columns": []string{"id", "p:customer", "p:gone"}, "group": "", "sort": nil}}
}

func create(t *testing.T, api humatest.TestAPI, name string) viewBody {
	t.Helper()
	res := api.Post("/api/views", map[string]any{"name": name, "content": content("status:Todo")})
	var v viewBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &v) != nil {
		t.Fatalf("create %q: %d %s", name, res.Code, res.Body)
	}
	return v
}

func TestCreateAndList(t *testing.T) {
	api := newAPI(t)
	v := create(t, api, "Bugs")
	if len(v.ID) != 26 || v.Name != "Bugs" || v.Content.Query != "status:Todo" || v.Content.Layout != "table" {
		t.Errorf("view = %+v", v)
	}
	if got := strings.Join(v.Content.Table.Columns, ","); got != "id,p:customer" {
		t.Errorf("存在しないキーの列は外す: %q", got)
	}
	res := api.Get("/api/views")
	var list struct{ Views []viewBody }
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &list) != nil || len(list.Views) != 1 || list.Views[0].ID != v.ID {
		t.Fatalf("list: %d %s", res.Code, res.Body)
	}
}

func TestEmptyListIsArray(t *testing.T) {
	if res := newAPI(t).Get("/api/views"); strings.TrimSpace(res.Body.String()) != `{"views":[]}` {
		t.Errorf("body = %s", res.Body)
	}
}

func TestCreateSameNameIsRefused(t *testing.T) {
	api := newAPI(t)
	create(t, api, "Bugs")
	res := api.Post("/api/views", map[string]any{"name": "bugs", "content": content("")})
	if res.Code != 422 || !strings.Contains(res.Body.String(), "同じ名前のビューがあります") {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

func TestSaveRenameDuplicateDelete(t *testing.T) {
	api := newAPI(t)
	v := create(t, api, "Bugs")

	res := api.Put("/api/views/"+v.ID, map[string]any{"content": content("kind:bug")})
	var saved viewBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &saved) != nil || saved.Content.Query != "kind:bug" {
		t.Fatalf("save: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/views/"+v.ID+"/rename", map[string]any{"name": "Tasks"})
	var renamed viewBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &renamed) != nil || renamed.Name != "Tasks" {
		t.Fatalf("rename: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/views/"+v.ID+"/duplicate", map[string]any{})
	var dup viewBody
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &dup) != nil || dup.Name != "Tasks のコピー" || dup.ID == v.ID {
		t.Fatalf("duplicate: %d %s", res.Code, res.Body)
	}
	if res = api.Delete("/api/views/" + v.ID); res.Code != 204 {
		t.Fatalf("delete: %d %s", res.Code, res.Body)
	}
	if res = api.Delete("/api/views/" + v.ID); res.Code != 404 {
		t.Errorf("もう一度削除: %d", res.Code)
	}
}

func TestUnknownIDIs404(t *testing.T) {
	api := newAPI(t)
	id := strings.Repeat("a", 26)
	for name, code := range map[string]int{
		"save":      api.Put("/api/views/"+id, map[string]any{"content": content("")}).Code,
		"rename":    api.Post("/api/views/"+id+"/rename", map[string]any{"name": "x"}).Code,
		"duplicate": api.Post("/api/views/"+id+"/duplicate", map[string]any{}).Code,
		"delete":    api.Delete("/api/views/" + id).Code,
	} {
		if code != 404 {
			t.Errorf("%s: %d", name, code)
		}
	}
}

func TestInvalidInputIs422(t *testing.T) {
	api := newAPI(t)
	if res := api.Put("/api/views/ABC", map[string]any{"content": content("")}); res.Code != 422 {
		t.Errorf("id ABC: %d", res.Code)
	}
	bad := content("")
	bad["layout"] = "kanban"
	if res := api.Post("/api/views", map[string]any{"name": "x", "content": bad}); res.Code != 422 {
		t.Errorf("layout kanban: %d", res.Code)
	}
}

func TestTooLargeContentIs422(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/views", map[string]any{"name": "x", "content": content(strings.Repeat("a", 64*1024+1))})
	if res.Code != 422 || !strings.Contains(res.Body.String(), "ビューが大きすぎます(64 KiB まで)") {
		t.Errorf("%d %s", res.Code, res.Body)
	}
	v := create(t, api, "y")
	if res = api.Put("/api/views/"+v.ID, map[string]any{"content": content(strings.Repeat("a", 64*1024+1))}); res.Code != 422 {
		t.Errorf("save: %d", res.Code)
	}
}

func TestKeyImpact(t *testing.T) {
	api := newAPI(t)
	if res := api.Post("/api/views", map[string]any{"name": "c", "content": content("customer:A")}); res.Code != 200 {
		t.Fatal(res.Body)
	}
	res := api.Get("/api/views/key-impact?storeId=S1&workspaceId=W1&key=customer")
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"affected":true}` {
		t.Errorf("使っている: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/views/key-impact?storeId=S1&workspaceId=W1&key=other")
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != `{"affected":false}` {
		t.Errorf("使っていない: %d %s", res.Code, res.Body)
	}
}
