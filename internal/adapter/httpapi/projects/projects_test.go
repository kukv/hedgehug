package projects_test

import (
	"context"
	"maps"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// newAPI は、リポジトリを足しても探さない(探索の関数が nil の)組み立て。
func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	api, _ := build(t, false)
	return api
}

// newAPIWithGitHub は、リポジトリを足したら ghapi.ScanLater で探す組み立てと、探索が問い合わせる偽の GitHub。
// Store S1 にはアカウント A1 を割り当てる。
func newAPIWithGitHub(t *testing.T) (humatest.TestAPI, *apptest.GitHub) {
	t.Helper()
	return build(t, true)
}

func build(t *testing.T, withScan bool) (humatest.TestAPI, *apptest.GitHub) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	gh := &apptest.GitHub{Users: map[string]string{"tok": "taro"}}
	links := githublink.Links{
		Stores: s, Identity: apptest.Identity{Login: "taro"}, Tokens: tokens{}, GitHub: gh,
		StoreGH: &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Account: "A1"}}},
		Checks:  apptest.NewChecks(), Scans: &scans{rows: map[string]port.ScanRecord{}}, Missing: noMissing{},
		Events: &apptest.Events{}, Clock: clock,
	}
	var scan projects.ScanFunc
	if withScan {
		scan = ghapi.ScanLater(links)
	}
	_, api := humatest.New(t)
	projects.Register(api, appprojects.Service{
		Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"},
		Clock: apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)),
	}, scan)
	return api, gh
}

// tokens は、どの Store にもアカウント A1 のトークン「tok」を返す偽物。
type tokens struct{}

func (tokens) TokenFor(context.Context, port.StoreID, string) (string, account.Account, error) {
	return "tok", account.Account{ID: "A1", Name: "work", Login: "taro"}, nil
}

func (tokens) Rejected(account.ID) {}

func (tokens) Recheck(context.Context) (bool, error) { return false, nil }

// noMissing は、port.AppMissing のうち、消すだけを受ける偽物。
type noMissing struct{ port.AppMissing }

func (noMissing) Clear(context.Context, account.ID, string, string) (bool, error) { return false, nil }

// scans は、port.PRScans のメモリ上の偽物(Store は 1 つだけ使う)。裏の探索から呼ばれるので、ロックで守る。
type scans struct {
	mu   sync.Mutex
	rows map[string]port.ScanRecord
}

func (m *scans) Scans(context.Context, port.StoreID) (map[string]port.ScanRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.rows), nil
}

func (m *scans) SaveScan(_ context.Context, _ port.StoreID, repo string, r port.ScanRecord) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(repo)
	old, ok := m.rows[key]
	if ok { // いまの記録と合わせる(本物の SaveScan と同じ: Read は論理和、Since は大きい方、Failure は書く側)
		r.Read = r.Read || old.Read
		if old.Since.After(r.Since) {
			r.Since = old.Since
		}
	}
	m.rows[key] = r
	return !ok || old != r, nil
}

// Renames と SaveRename は、改名の対応を持たない(このパッケージのテストは改名を使わない)。
func (m *scans) Renames(context.Context, port.StoreID) (map[string]string, error) { return nil, nil }

func (m *scans) SaveRename(context.Context, port.StoreID, string, string) error { return nil }

const base = "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace) + "/projects"

func TestCreateAndList(t *testing.T) {
	api := newAPI(t)
	res := api.Post(base, map[string]any{"title": "決済", "owner": "taro", "status": "InProgress", "start": "2026-11-01", "end": ""})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"number":2`) || !strings.Contains(res.Body.String(), `"status":"InProgress"`) {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	res = api.Get(base)
	if res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 2 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"title":"ログイン"`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
}

func TestRefusalIs422WithTheMessage(t *testing.T) {
	res := newAPI(t).Post(base, map[string]any{"title": "ログイン"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "このワークスペースには同じ名前のプロジェクトがあります") {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

func TestBadInputIsRefusedByTheSchema(t *testing.T) {
	api := newAPI(t)
	for _, body := range []map[string]any{
		{"title": "x", "status": "Review"}, {"title": "x", "start": "11/01"}, {"title": "x", "end": "2026-02-30"}, {"title": "a\nb"},
	} {
		if res := api.Post(base, body); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%v: %d", body, res.Code)
		}
	}
}

func TestUnknownStoreIs404(t *testing.T) {
	if res := newAPI(t).Get("/api/stores/nope/projects/x"); res.Code != http.StatusNotFound {
		t.Errorf("%d", res.Code)
	}
}
