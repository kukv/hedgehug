package github_test

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestLinksAPI(t *testing.T) {
	_, api := humatest.New(t)
	ghapi.Register(api, linkServices(t))
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"

	res := api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": false})
	if res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	res = api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "x", "addToProject": false})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "番号を入力してください") {
		t.Errorf("refusal: %d %s", res.Code, res.Body)
	}
	res = api.Get(base)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"repo":"org/web","number":41,"title":"Add login form","state":"open","review":"pending","checks":"pending","error":""`) {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	if res = api.Post(base+"/refresh", map[string]any{}); res.Code != http.StatusOK {
		t.Errorf("refresh: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/links")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"display":"HH-2"`) {
		t.Errorf("project links: %d %s", res.Code, res.Body)
	}
	if res = api.Post(base+"/remove", map[string]any{"kind": "pr", "repo": "org/web", "number": 41}); res.Code != http.StatusNoContent {
		t.Errorf("remove: %d %s", res.Code, res.Body)
	}
	if res = api.Get("/api/stores/S1/items/01TESTIT0000000000000000NO/links"); res.Code != http.StatusNotFound {
		t.Errorf("missing item: %d", res.Code)
	}
}

func TestLinksRefreshAnswer(t *testing.T) {
	_, api := humatest.New(t)
	svc := linkServices(t)
	ghapi.Register(api, svc)
	gh, _ := svc.Links.GitHub.(*apptest.GitHub)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"
	if res := api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": false}); res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	gh.Put("org/web", port.Found{
		Kind: itemlink.PR, Repo: "org/web", Number: 41,
		Values: itemlink.Values{Title: "Add login form", State: itemlink.Merged, Review: itemlink.Approved}, Checks: itemlink.Success,
	})
	res := api.Post(base+"/refresh", map[string]any{})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"state":"merged","review":"approved","checks":"success","error":""`) {
		t.Errorf("refresh: %d %s", res.Code, res.Body)
	}
	gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.Unreachable})
	res = api.Post(base+"/refresh", map[string]any{})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"checks":"success","error":"状態を更新できませんでした: GitHub に接続できません。接続を確かめて、もう一度お試しください"`) {
		t.Errorf("refresh failure: %d %s", res.Code, res.Body)
	}
}

// linkServices は、accounts_test.go の newServices に、Store S1(サンプル)と、S1 に割り当てたアカウント A1(@taro)と、
// org/web#41 を持つ偽の GitHub で作った Links を足したもの。
func linkServices(t *testing.T) ghapi.Services {
	t.Helper()
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	gh.Put("org/web", port.Found{
		Kind: itemlink.PR, Repo: "org/web", Number: 41,
		Values: itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}, Checks: itemlink.Running,
	})
	svc := newServices(gh)
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	accounts := apptest.NewAccounts()
	if err := accounts.Add(t.Context(), account.Account{ID: "A1", Name: "work", Login: "taro", Method: account.EnvVar}); err != nil {
		t.Fatal(err)
	}
	infos := &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Account: "A1"}}}
	svc.Links = githublink.Links{
		Stores: s, Identity: apptest.Identity{Login: "taro"},
		Tokens: ghapp.Connector{
			Session: ghapp.NewSession(), Accounts: accounts, Secrets: &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}},
			GitHub: gh, Stores: infos, Clock: clock,
		},
		GitHub: gh, Accounts: accounts, StoreGH: infos,
		Checks: apptest.NewChecks(), Reads: &memFailures{rows: map[string]string{}}, Missing: noMissing{}, Events: &apptest.Events{}, Clock: clock,
	}
	return svc
}

// memFailures は、port.GitHubReads のうち取り直しの失敗の文言だけを持つ偽物(Store は 1 つだけ使う)。
type memFailures struct {
	port.GitHubReads
	rows map[string]string
}

func (m *memFailures) Failures(context.Context, port.StoreID) (map[string]string, error) {
	return maps.Clone(m.rows), nil
}

func (m *memFailures) SetFailure(_ context.Context, _ port.StoreID, key, text string) (bool, error) {
	if m.rows[key] == text {
		return false, nil
	}
	if text == "" {
		delete(m.rows, key)
	} else {
		m.rows[key] = text
	}
	return true, nil
}

// noMissing は、port.AppMissing のうち、消すだけを受ける偽物(この API のテストは App 未導入を使わない)。
type noMissing struct{ port.AppMissing }

func (noMissing) Clear(context.Context, account.ID, string, string) (bool, error) { return false, nil }

// memScans は、port.PRScans のメモリ上の偽物(Store は 1 つだけ使う)。裏の探索から呼ばれるので、ロックで守る。
type memScans struct {
	mu   sync.Mutex
	rows map[string]port.ScanRecord
}

func (m *memScans) Scans(context.Context, port.StoreID) (map[string]port.ScanRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.rows), nil
}

func (m *memScans) SaveScan(_ context.Context, _ port.StoreID, repo string, r port.ScanRecord) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.rows[strings.ToLower(repo)]
	m.rows[strings.ToLower(repo)] = r
	return !ok || old != r, nil
}

// waitPullsCalls は、gh が repo の ReadPulls を n 回呼ぶまで待つ(裏の探索)。
func waitPullsCalls(t *testing.T, gh *apptest.GitHub, repo string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for gh.ReadPullsCalls(repo) < n {
		if time.Now().After(deadline) {
			t.Fatalf("ReadPulls calls = %d, want %d", gh.ReadPullsCalls(repo), n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// 「PR をリンク」で「プロジェクトのリポジトリにも追加する」なら、そのリポジトリをすぐ探す(D4b-9)。
func TestAddLinkToProjectStartsScan(t *testing.T) {
	_, api := humatest.New(t)
	svc := linkServices(t)
	svc.Links.Scans = &memScans{rows: map[string]port.ScanRecord{}}
	ghapi.Register(api, svc)
	gh, _ := svc.Links.GitHub.(*apptest.GitHub)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"
	if res := api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": true}); res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	waitPullsCalls(t, gh, "org/web", 1)
}

// failingChecks は、チェックの結果を置けない port.Checks。
type failingChecks struct{ port.Checks }

func (failingChecks) Save(context.Context, port.StoreID, string, itemlink.Checks) (bool, error) {
	return false, errors.New("state.db を書けない")
}

// リンクとリポジトリを保存したあとで失敗しても(チェックの結果を置けない)、足したリポジトリは探す
// (D4b 最終レビュー M-1)。
func TestAddLinkToProjectScansEvenAfterALaterError(t *testing.T) {
	_, api := humatest.New(t)
	svc := linkServices(t)
	svc.Links.Scans = &memScans{rows: map[string]port.ScanRecord{}}
	svc.Links.Checks = failingChecks{svc.Links.Checks}
	ghapi.Register(api, svc)
	gh, _ := svc.Links.GitHub.(*apptest.GitHub)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"
	if res := api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": true}); res.Code < 500 {
		t.Fatalf("add: %d %s, want the error", res.Code, res.Body)
	}
	waitPullsCalls(t, gh, "org/web", 1)
}

// プロジェクトに足さなかったとき(addToProject が偽・断られた・もうプロジェクトにある)は探さない。
// 最後に別のリポジトリを足して探すのを待ち、それまでに探していないことを確かめる。
func TestAddLinkWithoutProjectNoScan(t *testing.T) {
	_, api := humatest.New(t)
	svc := linkServices(t)
	svc.Links.Scans = &memScans{rows: map[string]port.ScanRecord{}}
	ghapi.Register(api, svc)
	gh, _ := svc.Links.GitHub.(*apptest.GitHub)
	gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 42, Values: itemlink.Values{Title: "b", State: itemlink.Open, Review: itemlink.NoReview}})
	gh.Put("org/api", port.Found{Kind: itemlink.PR, Repo: "org/api", Number: 7, Values: itemlink.Values{Title: "c", State: itemlink.Open, Review: itemlink.NoReview}})
	task := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"
	epic := "/api/stores/S1/items/" + string(apptest.SampleEpic) + "/links"
	if res := api.Post(task, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": false}); res.Code != http.StatusNoContent {
		t.Fatalf("add without project: %d %s", res.Code, res.Body)
	}
	if res := api.Post(task, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": true}); res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("refused add: %d %s", res.Code, res.Body)
	}
	err := svc.Links.Stores.Write(t.Context(), "S1", port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return tx.WriteProjects().Append(apptest.SampleProject, change.NewAdd("repos", "org/web"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if res := api.Post(epic, map[string]any{"kind": "pr", "repo": "org/web", "number": "42", "addToProject": true}); res.Code != http.StatusNoContent {
		t.Fatalf("add with the repo in the project: %d %s", res.Code, res.Body)
	}
	if res := api.Post(epic, map[string]any{"kind": "pr", "repo": "org/api", "number": "7", "addToProject": true}); res.Code != http.StatusNoContent {
		t.Fatalf("add org/api: %d %s", res.Code, res.Body)
	}
	waitPullsCalls(t, gh, "org/api", 1)
	if n := gh.ReadPullsCalls("org/web"); n != 0 {
		t.Errorf("ReadPulls calls for org/web = %d, want none", n)
	}
}

func TestListRepoFailures(t *testing.T) {
	_, api := humatest.New(t)
	svc := linkServices(t)
	svc.Links.Scans = &memScans{rows: map[string]port.ScanRecord{"org/web": {Failure: "org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"}}}
	ghapi.Register(api, svc)
	for _, repo := range []string{"org/web", "org/api"} {
		err := svc.Links.Stores.Write(t.Context(), "S1", port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
			return tx.WriteProjects().Append(apptest.SampleProject, change.NewAdd("repos", repo))
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	res := api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/repo-failures")
	want := `{"failures":[{"repo":"org/web","text":"org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"}]}`
	if res.Code != http.StatusOK || strings.TrimSpace(res.Body.String()) != want {
		t.Errorf("failures: %d %s", res.Code, res.Body)
	}
	if res = api.Get("/api/stores/S1/projects/01NOPE00000000000000000000/repo-failures"); res.Code != http.StatusNotFound {
		t.Errorf("missing project: %d %s", res.Code, res.Body)
	}
}
