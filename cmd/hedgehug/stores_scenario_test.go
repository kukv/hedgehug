//go:build !e2e

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
	"github.com/kukv/hedgehug/internal/infra/gitstore"
)

// 本物の組み立て(wire)が、config.yaml の Store を開き、本物の一覧と「あなた」をつなぎ、書き込みと一緒に「あなた」を
// メンバーに足すことを、一時フォルダの config.yaml と Store で確かめる(G1)。アカウントを書かないので、キーチェーンや
// GitHub には触れない。

const (
	wiredStore   port.StoreID = "01WIREDSTORE00000000000001"
	wiredWS      workspace.ID = "01WIREDWS00000000000000001"
	wiredProject project.ID   = "01WIREDPR00000000000000001"
)

// wiredStoreDir は、メンバーが hanako だけで、ワークスペース(WD)とプロジェクト(WD-P1)を持つ Store を作り、そのフォルダを返す。
func wiredStoreDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	if err := gitstore.Create(dir, wiredStore, scenarioStart); err != nil {
		t.Fatal(err)
	}
	s := gitstore.New(apptest.NewClock(scenarioStart), apptest.NewIDs())
	if _, err := s.Attach(dir); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.CreateWithSettings(workspace.Workspace{ID: wiredWS, Name: "配線", Prefix: "WD"}, ruleSettings("01WIREDVALUE", ""))
	if err != nil {
		t.Fatal(err)
	}
	pr := project.Project{ID: wiredProject, Place: project.Place{Workspace: wiredWS, Number: 1}, Rank: "V", Title: "Web", Owner: "hanako", Status: project.Todo}
	err = s.Write(t.Context(), wiredStore, port.Operation{Actor: change.User("hanako"), Summary: "ワークスペースを作成"}, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteMembers().Append(member.Init(), member.Join("hanako")),
			tx.WriteWorkspaces().Append(wiredWS, ws, numbering.Draft(numbering.Projects, 1)),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// wireWith は、config.yaml に login: taro と stores(rows の行)を書き、XDG の場所を一時フォルダにして wire を呼ぶ。
// wire が出した警告を返す。
func wireWith(t *testing.T, rows ...string) (httpapi.Deps, string) {
	t.Helper()
	return wireConfig(t, "login: taro\nstores:\n"+strings.Join(rows, ""))
}

// wireConfig は、config.yaml を body にして wire を呼び、wire が出した警告を返す。テストの終わりに止め、
// wire が動かした goroutine が終わるのを待つ(一時フォルダを消す前に、Store と state.db に触れなくする)。
func wireConfig(t *testing.T, body string) (httpapi.Deps, string) {
	t.Helper()
	cfg, data := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_DATA_HOME", data)
	if err := os.MkdirAll(filepath.Join(cfg, "hedgehug"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "hedgehug", "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	logs := &lockedLog{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	ctx, cancel := context.WithCancel(context.Background())
	var running sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		running.Wait()
	})
	deps, _, err := wireRunning(ctx, &running)
	if err != nil {
		t.Fatal(err)
	}
	return deps, logs.String()
}

// lockedLog は、wire が動かした goroutine も書くログの書き先。読み書きを 1 つずつにする。
type lockedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// openWarnings は、logs のうち、wire が起動のときに config.yaml の Store を開かなかった警告の数。同期などの
// goroutine が出す警告は数えない。
func openWarnings(logs string) int {
	return strings.Count(logs, `level=WARN msg="config.yaml の`)
}

func storeRow(id port.StoreID, name, path string) string {
	return fmt.Sprintf("  - {id: %q, name: %q, path: %q}\n", id, name, path)
}

// oldShapeRow は、前回の版の形の行(id がない)。
const oldShapeRow = "  - {name: old, path: /nowhere, remotes: [u]}\n"

func listedIDs(t *testing.T, deps httpapi.Deps) []port.StoreID {
	t.Helper()
	infos, err := deps.StoreList.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var ids []port.StoreID
	for _, s := range infos {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestWireOpensConfiguredStores(t *testing.T) {
	deps, _ := wireWith(t, storeRow(wiredStore, "main", wiredStoreDir(t)), oldShapeRow)
	if got := listedIDs(t, deps); !slices.Equal(got, []port.StoreID{wiredStore}) {
		t.Errorf("StoreList = %v, want [%s]", got, wiredStore)
	}
	if me, err := deps.Identity.Me(t.Context(), wiredStore); err != nil || me != "taro" {
		t.Errorf("Me = %q, %v, want taro", me, err)
	}
	if targets, err := deps.SyncTargets.List(t.Context()); err != nil || len(targets) != 1 || targets[0].ID != wiredStore {
		t.Errorf("SyncTargets = %+v, %v", targets, err)
	}
}

// createItem は、本物の HTTP の経路で、プロジェクト WD-P1 にアイテムを作る要求を送る。
func createItem(t *testing.T, h http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"project":%q,"kind":"Epic","title":"配線"}`, wiredProject)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/stores/"+string(wiredStore)+"/items", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("create item = %d %s", rec.Code, rec.Body)
	}
	return rec
}

func TestWireJoinsYouOnWrite(t *testing.T) {
	deps, _ := wireWith(t, storeRow(wiredStore, "main", wiredStoreDir(t)))
	token := newToken()
	h := httpapi.New(token, fstest.MapFS{}, deps)
	if got := createItem(t, h, token).Header().Get("Hedgehug-Joined"); got != "taro" {
		t.Errorf("first Hedgehug-Joined = %q, want taro", got)
	}
	var logins []string
	err := deps.Stores.Read(t.Context(), wiredStore, func(r port.Reader) error {
		ms, err := r.Members().List()
		for _, m := range ms {
			logins = append(logins, m.Login)
		}
		return err
	})
	if err != nil || !slices.Equal(logins, []string{"hanako", "taro"}) {
		t.Errorf("members = %v, %v, want [hanako taro]", logins, err)
	}
	if got, ok := createItem(t, h, token).Header()["Hedgehug-Joined"]; ok {
		t.Errorf("second Hedgehug-Joined = %q, want none", got)
	}
}

func TestWireSkipsUnopenable(t *testing.T) {
	good := wiredStoreDir(t)
	gone := filepath.Join(t.TempDir(), "gone")
	deps, logs := wireWith(t,
		storeRow("01WIREDSTORE00000000000002", "gone", gone),
		storeRow("01WIREDSTORE00000000000003", "mismatch", good), // store.yaml の ID は wiredStore
		oldShapeRow,
		storeRow(wiredStore, "main", good),
	)
	if got := listedIDs(t, deps); !slices.Equal(got, []port.StoreID{wiredStore}) {
		t.Errorf("StoreList = %v, want [%s]", got, wiredStore)
	}
	if n := openWarnings(logs); n != 3 {
		t.Errorf("warnings = %d, want 3:\n%s", n, logs)
	}
}

// 同じ id の行が 2 つあると、どちらの path か決められないので、どちらも開かない。
func TestWireSkipsDuplicateIDs(t *testing.T) {
	good := wiredStoreDir(t)
	deps, logs := wireWith(t, storeRow(wiredStore, "main", good), storeRow(wiredStore, "copy", good))
	if got := listedIDs(t, deps); len(got) != 0 {
		t.Errorf("StoreList = %v, want none", got)
	}
	if n := openWarnings(logs); n != 2 {
		t.Errorf("warnings = %d, want 2:\n%s", n, logs)
	}
}

// config.yaml を読めなくても起動は止まらず、Store は 1 つも開かない(G1-2)。一覧は読めない理由を返す。
func TestWireStartsWithBrokenConfig(t *testing.T) {
	deps, logs := wireConfig(t, "stores: [\n")
	if infos, err := deps.StoreList.List(t.Context()); len(infos) != 0 || err == nil {
		t.Errorf("StoreList = %v, %v, want none and the parse error", infos, err)
	}
	if n := openWarnings(logs); n != 1 {
		t.Errorf("warnings = %d, want 1:\n%s", n, logs)
	}
}
