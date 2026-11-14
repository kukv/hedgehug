package planning_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/planning"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	appplanning "github.com/kukv/hedgehug/internal/app/planning"
)

const (
	base     = "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace)
	itemBase = "/api/stores/S1/items/"
)

// newAPI は、サンプルを書いた Store S1 の、計画とアイテムの API。今日は 2026-11-13。
func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	me := apptest.Identity{Login: "taro"}
	planning.Register(api, appplanning.Service{Stores: s, IDs: ids, Identity: me, Clock: clock})
	items.Register(api, appitems.Service{Stores: s, IDs: ids, Identity: me, Clock: clock})
	return api
}

func check(t *testing.T, name string, res *httptest.ResponseRecorder, code int, contains string) {
	t.Helper()
	if res.Code != code || !strings.Contains(res.Body.String(), contains) {
		t.Errorf("%s: %d %s", name, res.Code, res.Body)
	}
}

func idOf(t *testing.T, res *httptest.ResponseRecorder) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil || v.ID == "" {
		t.Fatalf("body: %v %d %s", err, res.Code, res.Body)
	}
	return v.ID
}

func TestEmptyPlanning(t *testing.T) {
	api := newAPI(t)
	check(t, "overview", api.Get(base+"/planning"), http.StatusOK,
		`{"sprints":[],"releases":[],"active":null,"defaults":{"name":"Sprint 1","start":"2026-11-13","end":"2026-11-26"},"retentionDays":30}`)
	check(t, "sprints", api.Get(base+"/sprints"), http.StatusOK, `[]`)
	check(t, "releases", api.Get(base+"/releases"), http.StatusOK, `[]`)
	check(t, "unknown workspace", api.Get("/api/stores/S1/workspaces/01NOPE0000000000000000000/planning"), http.StatusNotFound, "")
}

func TestSprintAPI(t *testing.T) {
	api := newAPI(t)
	in := map[string]any{"name": "Sprint 1", "start": "", "end": "", "goal": "ログイン"}
	res := api.Post(base+"/sprints", in)
	check(t, "create", res, http.StatusCreated, `"state":"planned"`)
	sp := idOf(t, res)
	check(t, "duplicate", api.Post(base+"/sprints", in), http.StatusUnprocessableEntity, "同じ名前のスプリントがあります")
	check(t, "bad date", api.Post(base+"/sprints", map[string]any{"name": "Sprint 2", "start": "11/13", "end": "", "goal": ""}), http.StatusUnprocessableEntity, "")
	// 形だけ合う、暦にない日付も断る。
	check(t, "impossible date", api.Post(base+"/sprints", map[string]any{"name": "Sprint 2", "start": "", "end": "2026-13-45", "goal": ""}), http.StatusUnprocessableEntity, "not a date: 2026-13-45")
	check(t, "update impossible date", api.Put(base+"/sprints/"+sp, map[string]any{"name": "Sprint 1", "start": "2026-02-30", "end": "", "goal": ""}), http.StatusUnprocessableEntity, "not a date: 2026-02-30")
	in["goal"] = "認証"
	check(t, "update", api.Put(base+"/sprints/"+sp, in), http.StatusNoContent, "")
	check(t, "start bad date", api.Post(base+"/sprints/"+sp+"/start", map[string]any{"start": "x", "end": "2026-11-26"}), http.StatusUnprocessableEntity, "")
	check(t, "start impossible date", api.Post(base+"/sprints/"+sp+"/start", map[string]any{"start": "2026-11-13", "end": "2026-11-31"}), http.StatusUnprocessableEntity, "not a date: 2026-11-31")
	check(t, "start", api.Post(base+"/sprints/"+sp+"/start", map[string]any{"start": "2026-11-13", "end": "2026-11-26"}), http.StatusNoContent, "")
	check(t, "card", api.Get(base+"/sprints"), http.StatusOK, `"goal":"認証"`)
	check(t, "active", api.Get(base+"/planning"), http.StatusOK, `"active":{"id":"`+sp+`"`)
	check(t, "close", api.Post(base+"/sprints/"+sp+"/close", map[string]any{"destination": nil}), http.StatusNoContent, "")
	check(t, "closed card", api.Get(base+"/sprints"), http.StatusOK, `"confirmed":{"committed":{"points":0,"count":0},"completed":{"points":0,"count":0}}`)
	check(t, "close again", api.Post(base+"/sprints/"+sp+"/close", map[string]any{"destination": nil}), http.StatusNotFound, "")
	check(t, "delete", api.Delete(base+"/sprints/"+sp), http.StatusNoContent, "")
	check(t, "deleted", api.Get(base+"/sprints"), http.StatusOK, `[]`)
	check(t, "restore", api.Post(base+"/sprints/"+sp+"/restore"), http.StatusNoContent, "")
	check(t, "restored", api.Get(base+"/sprints"), http.StatusOK, `"id":"`+sp+`"`)
	check(t, "delete again", api.Delete(base+"/sprints/"+sp), http.StatusNoContent, "")
	check(t, "same name", api.Post(base+"/sprints", in), http.StatusCreated, "")
	check(t, "restore with same name", api.Post(base+"/sprints/"+sp+"/restore"), http.StatusUnprocessableEntity,
		"同じ名前のスプリントがあります。先にそのスプリントの名前を変えてください")
}

func TestCloseSprintToDestination(t *testing.T) {
	api := newAPI(t)
	s1 := idOf(t, api.Post(base+"/sprints", map[string]any{"name": "Sprint 1", "start": "", "end": "", "goal": ""}))
	s2 := idOf(t, api.Post(base+"/sprints", map[string]any{"name": "Sprint 2", "start": "", "end": "", "goal": ""}))
	check(t, "start", api.Post(base+"/sprints/"+s1+"/start", map[string]any{"start": "2026-11-13", "end": "2026-11-26"}), http.StatusNoContent, "")
	check(t, "put", api.Put(itemBase+string(apptest.SampleTask)+"/sprint", map[string]any{"sprint": s1}), http.StatusNoContent, "")
	check(t, "close", api.Post(base+"/sprints/"+s1+"/close", map[string]any{"destination": s2}), http.StatusNoContent, "")
	check(t, "moved", api.Get(itemBase+string(apptest.SampleTask)), http.StatusOK, `"sprint":"`+s2+`"`)
}

func TestReleaseAPI(t *testing.T) {
	api := newAPI(t)
	in := map[string]any{"name": "v1.0.0", "date": "2026-12-01", "description": "最初"}
	res := api.Post(base+"/releases", in)
	check(t, "create", res, http.StatusCreated, `"state":"unreleased"`)
	rel := idOf(t, res)
	check(t, "duplicate", api.Post(base+"/releases", in), http.StatusUnprocessableEntity, "同じ名前のリリースがあります")
	check(t, "bad date", api.Post(base+"/releases", map[string]any{"name": "v2", "date": "2026/12/01", "description": ""}), http.StatusUnprocessableEntity, "")
	check(t, "impossible date", api.Post(base+"/releases", map[string]any{"name": "v2", "date": "2026-13-45", "description": ""}), http.StatusUnprocessableEntity, "not a date: 2026-13-45")
	check(t, "update impossible date", api.Put(base+"/releases/"+rel, map[string]any{"name": "v1.0.0", "date": "2026-02-30", "description": ""}), http.StatusUnprocessableEntity, "not a date: 2026-02-30")
	in["description"] = "初版"
	check(t, "update", api.Put(base+"/releases/"+rel, in), http.StatusNoContent, "")
	check(t, "state", api.Put(base+"/releases/"+rel+"/state", map[string]any{"state": "released"}), http.StatusNoContent, "")
	check(t, "bad state", api.Put(base+"/releases/"+rel+"/state", map[string]any{"state": "shipped"}), http.StatusUnprocessableEntity, "")
	check(t, "card", api.Get(base+"/releases"), http.StatusOK, `"description":"初版","state":"released"`)
	check(t, "overview", api.Get(base+"/planning"), http.StatusOK, `"releases":[{"id":"`+rel+`"`)
	check(t, "delete", api.Delete(base+"/releases/"+rel), http.StatusNoContent, "")
	check(t, "deleted", api.Get(base+"/releases"), http.StatusOK, `[]`)
	check(t, "restore", api.Post(base+"/releases/"+rel+"/restore"), http.StatusNoContent, "")
	check(t, "restore again", api.Post(base+"/releases/"+rel+"/restore"), http.StatusNotFound, "")
	check(t, "delete again", api.Delete(base+"/releases/"+rel), http.StatusNoContent, "")
	check(t, "same name", api.Post(base+"/releases", in), http.StatusCreated, "")
	check(t, "restore with same name", api.Post(base+"/releases/"+rel+"/restore"), http.StatusUnprocessableEntity,
		"同じ名前のリリースがあります。先にそのリリースの名前を変えてください")
}

func TestItemSprintAPI(t *testing.T) {
	api := newAPI(t)
	task := itemBase + string(apptest.SampleTask)
	check(t, "empty", api.Get(task), http.StatusOK, `"sprint":""`)
	sp := idOf(t, api.Post(base+"/sprints", map[string]any{"name": "Sprint 1", "start": "", "end": "", "goal": ""}))
	check(t, "put", api.Put(task+"/sprint", map[string]any{"sprint": sp}), http.StatusNoContent, "")
	check(t, "get", api.Get(task), http.StatusOK, `"sprint":"`+sp+`"`)
	check(t, "card row", api.Get(base+"/sprints"), http.StatusOK, `"display":"HH-2","kind":"Task","title":"ログイン画面を作る"`)
	check(t, "epic", api.Put(itemBase+string(apptest.SampleEpic)+"/sprint", map[string]any{"sprint": sp}), http.StatusBadRequest, "")
	check(t, "unknown", api.Put(task+"/sprint", map[string]any{"sprint": "01NOPE0000000000000000000"}), http.StatusNotFound, "")
	check(t, "start", api.Post(base+"/sprints/"+sp+"/start", map[string]any{"start": "2026-11-13", "end": "2026-11-26"}), http.StatusNoContent, "")
	check(t, "backlog", api.Put(task+"/sprint", map[string]any{"sprint": nil}), http.StatusNoContent, "")
	check(t, "close", api.Post(base+"/sprints/"+sp+"/close", map[string]any{"destination": nil}), http.StatusNoContent, "")
	check(t, "closed", api.Put(task+"/sprint", map[string]any{"sprint": sp}), http.StatusUnprocessableEntity, "完了したスプリントには入れられません")
	check(t, "still backlog", api.Get(task), http.StatusOK, `"sprint":""`)
}

func TestItemReleaseAPI(t *testing.T) {
	api := newAPI(t)
	epic := itemBase + string(apptest.SampleEpic)
	check(t, "empty", api.Get(epic), http.StatusOK, `"release":""`)
	rel := idOf(t, api.Post(base+"/releases", map[string]any{"name": "v1.0.0", "date": "", "description": ""}))
	check(t, "put", api.Put(epic+"/release", map[string]any{"release": rel}), http.StatusNoContent, "")
	check(t, "get", api.Get(epic), http.StatusOK, `"release":"`+rel+`"`)
	check(t, "card", api.Get(base+"/releases"), http.StatusOK, `"kind":"Epic"`)
	check(t, "none", api.Put(epic+"/release", map[string]any{"release": nil}), http.StatusNoContent, "")
	check(t, "cleared", api.Get(epic), http.StatusOK, `"release":""`)
	check(t, "unknown item", api.Put(itemBase+"01NOPE0000000000000000000/release", map[string]any{"release": nil}), http.StatusNotFound, "")
}

func TestCreateItemInSprint(t *testing.T) {
	api := newAPI(t)
	sp := idOf(t, api.Post(base+"/sprints", map[string]any{"name": "Sprint 1", "start": "", "end": "", "goal": ""}))
	create := func(extra map[string]any) *httptest.ResponseRecorder {
		body := map[string]any{"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic), "kind": "Task", "title": "API"}
		for k, v := range extra {
			body[k] = v
		}
		return api.Post("/api/stores/S1/items", body)
	}
	check(t, "sprint", create(map[string]any{"sprint": sp}), http.StatusOK, `"sprint":"`+sp+`"`)
	check(t, "null", create(map[string]any{"sprint": nil}), http.StatusOK, `"sprint":""`)
	check(t, "absent", create(nil), http.StatusOK, `"sprint":""`)
	check(t, "unknown", create(map[string]any{"sprint": "01NOPE0000000000000000000"}), http.StatusNotFound, "")
	check(t, "epic", create(map[string]any{"kind": "Epic", "parent": "", "sprint": sp}), http.StatusBadRequest, "")
	check(t, "start", api.Post(base+"/sprints/"+sp+"/start", map[string]any{"start": "2026-11-13", "end": "2026-11-26"}), http.StatusNoContent, "")
	check(t, "close", api.Post(base+"/sprints/"+sp+"/close", map[string]any{"destination": nil}), http.StatusNoContent, "")
	check(t, "closed", create(map[string]any{"kind": "Story", "sprint": sp}), http.StatusUnprocessableEntity, "完了したスプリントには入れられません")
}
