package notes_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/notes"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appcomments "github.com/kukv/hedgehug/internal/app/comments"
)

func newCommentsAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	notes.Register(api, notes.Services{
		Comments: appcomments.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"}},
	})
	return api
}

func TestItemCommentLifecycle(t *testing.T) {
	api := newCommentsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/comments"
	res := api.Post(base, map[string]any{"body": " @taro 見てください "})
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"body":"@taro 見てください"`) ||
		!strings.Contains(body, `"byName":"hanako"`) || !strings.Contains(body, `"edited":false`) {
		t.Fatalf("post: %d %s", res.Code, body)
	}
	var posted notes.Comment
	if err := json.Unmarshal(res.Body.Bytes(), &posted); err != nil || posted.ID == "" || posted.At == "" {
		t.Fatalf("posted = %+v, %v", posted, err)
	}
	one := base + "/" + posted.ID
	if res := api.Put(one, map[string]any{"body": "直した"}); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"edited":true`) {
		t.Errorf("edit: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 1 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	if res := api.Delete(one); res.Code != http.StatusNoContent {
		t.Errorf("delete: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || res.Body.String() != "[]\n" {
		t.Errorf("list after delete: %d %q", res.Code, res.Body)
	}
	if res := api.Delete(one); res.Code != http.StatusNotFound {
		t.Errorf("delete twice: %d", res.Code)
	}
}

func TestProjectComments(t *testing.T) {
	api := newCommentsAPI(t)
	base := "/api/stores/S1/projects/" + string(apptest.SampleProject) + "/comments"
	if res := api.Post(base, map[string]any{"body": "概要へ"}); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"body":"概要へ"`) {
		t.Fatalf("post: %d %s", res.Code, res.Body)
	}
	if res := api.Get(base); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 1 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	// アイテムのコメントには出ない。
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/comments"); res.Body.String() != "[]\n" {
		t.Errorf("item list: %s", res.Body)
	}
}

func TestCommentRefusalsAndUnknowns(t *testing.T) {
	api := newCommentsAPI(t)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/comments"
	res := api.Post(base, map[string]any{"body": "  "})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "コメントを入力してください") {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/01NOPE00000000000000000000/comments", map[string]any{"body": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown item: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/projects/01NOPE00000000000000000000/comments"); res.Code != http.StatusNotFound {
		t.Errorf("unknown project: %d", res.Code)
	}
	if res := api.Put(base+"/01NOPE00000000000000000000", map[string]any{"body": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown comment: %d", res.Code)
	}
}
