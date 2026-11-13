package projects_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

const one = "/api/stores/S1/projects/" + string(apptest.SampleProject)

func check(t *testing.T, name string, res *httptest.ResponseRecorder, code int, contains string) {
	t.Helper()
	if res.Code != code || !strings.Contains(res.Body.String(), contains) {
		t.Errorf("%s: %d %s, want %d containing %q", name, res.Code, res.Body, code, contains)
	}
}

func TestEditEndpoints(t *testing.T) {
	api := newAPI(t)
	check(t, "get has no repos", api.Get(one), http.StatusOK, `"repos":[]`)

	check(t, "rename", api.Put(one+"/title", map[string]any{"title": " 決済 "}), http.StatusOK, `"title":"決済"`)
	check(t, "rename to empty", api.Put(one+"/title", map[string]any{"title": " "}), http.StatusUnprocessableEntity, "タイトルは空にできません")
	check(t, "rename with a newline", api.Put(one+"/title", map[string]any{"title": "a\nb"}), http.StatusUnprocessableEntity, "")
	api.Post(base, map[string]any{"title": "請求"})
	check(t, "rename to a duplicate", api.Put(one+"/title", map[string]any{"title": "請求"}), http.StatusUnprocessableEntity, "このワークスペースには同じ名前のプロジェクトがあります")

	check(t, "status", api.Put(one+"/status", map[string]any{"status": "InProgress"}), http.StatusOK, `"status":"InProgress"`)
	check(t, "unknown status", api.Put(one+"/status", map[string]any{"status": "Review"}), http.StatusUnprocessableEntity, "")

	check(t, "owner", api.Put(one+"/owner", map[string]any{"owner": "hanako"}), http.StatusOK, `"owner":"hanako"`)
	check(t, "no owner", api.Put(one+"/owner", map[string]any{"owner": ""}), http.StatusOK, `"owner":""`)
	check(t, "a non-member owner", api.Put(one+"/owner", map[string]any{"owner": "nobody"}), http.StatusNotFound, "")

	check(t, "start", api.Put(one+"/date", map[string]any{"field": "start", "value": "2026-11-20"}), http.StatusOK, `"start":"2026-11-20"`)
	check(t, "end before start", api.Put(one+"/date", map[string]any{"field": "end", "value": "2026-11-19"}), http.StatusUnprocessableEntity, "終了日は開始日以降にしてください")
	check(t, "not a date", api.Put(one+"/date", map[string]any{"field": "end", "value": "2026-02-30"}), http.StatusUnprocessableEntity, "not a date")
	check(t, "unknown field", api.Put(one+"/date", map[string]any{"field": "middle", "value": "2026-11-20"}), http.StatusUnprocessableEntity, "")
	check(t, "clear start", api.Put(one+"/date", map[string]any{"field": "start", "value": ""}), http.StatusOK, `"start":""`)

	check(t, "add repo", api.Post(one+"/repos", map[string]any{"repo": " example-org/web "}), http.StatusOK, `"repos":["example-org/web"]`)
	check(t, "add the same repo", api.Post(one+"/repos", map[string]any{"repo": "Example-Org/WEB"}), http.StatusUnprocessableEntity, "もう追加されています")
	check(t, "add a bad repo", api.Post(one+"/repos", map[string]any{"repo": "web"}), http.StatusUnprocessableEntity, "リポジトリは owner/name の形式で入力してください")
	check(t, "remove repo", api.Post(one+"/repos/remove", map[string]any{"repo": "example-org/web"}), http.StatusOK, `"repos":[]`)
}

// リポジトリを追加したら、そのリポジトリをすぐ探す(D4b-9)。断られたら探さない。
func TestAddRepoStartsScan(t *testing.T) {
	api, gh := newAPIWithGitHub(t)
	check(t, "add a bad repo", api.Post(one+"/repos", map[string]any{"repo": "web"}), http.StatusUnprocessableEntity, "")
	check(t, "add repo", api.Post(one+"/repos", map[string]any{"repo": " example-org/web "}), http.StatusOK, `"repos":["example-org/web"]`)
	deadline := time.Now().Add(5 * time.Second)
	for gh.ReadPullsCalls("example-org/web") < 1 {
		if time.Now().After(deadline) {
			t.Fatalf("ReadPulls calls = %d, want 1", gh.ReadPullsCalls("example-org/web"))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := gh.ReadPullsCalls("web"); n != 0 {
		t.Errorf("refused repo was scanned: %d", n)
	}
}

func TestEditMissingProjectIs404(t *testing.T) {
	missing := "/api/stores/S1/projects/01NOPE00000000000000000000"
	check(t, "rename", newAPI(t).Put(missing+"/title", map[string]any{"title": "請求"}), http.StatusNotFound, "")
}
