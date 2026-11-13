//go:build e2e

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// e2e のビルドは、このマシンの中(ループバック)でしか待ち受けない。
func TestE2EAddrMustBeLoopback(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:0":    true,
		"127.0.0.1:5173": true,
		"[::1]:0":        true,
		"localhost:8080": true,
		":8080":          false,
		"0.0.0.0:8080":   false,
		"[::]:8080":      false,
		"192.168.1.2:80": false,
		"example.com:80": false,
		"127.0.0.1":      false,
		"":               false,
	} {
		if err := checkLoopback(addr); (err == nil) != ok {
			t.Errorf("checkLoopback(%q) = %v", addr, err)
		}
	}
}

func TestWireRefusesNonLoopbackAddr(t *testing.T) {
	t.Setenv("HEDGEHUG_TOKEN", "secret")
	t.Setenv("HEDGEHUG_ADDR", "0.0.0.0:8080")
	if _, err := wire(t.Context()); err == nil {
		t.Error("wire accepted a non-loopback address")
	}
}

func e2eDeps(t *testing.T) httpapi.Deps {
	t.Helper()
	t.Setenv("HEDGEHUG_TOKEN", "secret")
	t.Setenv("HEDGEHUG_ADDR", "127.0.0.1:0")
	deps, err := wire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return deps
}

// e2e のビルドの Store は e2e・readonly・team の 3 つ。e2e にはワークスペース Web と Mobile、team には Team がある。
func TestE2EWireStoresAndSample(t *testing.T) {
	deps := e2eDeps(t)
	infos, err := deps.StoreList.List(t.Context())
	if err != nil || len(infos) != 3 || infos[0].Name != "e2e" || infos[1].Name != "readonly" || infos[2].Name != "team" || infos[2].ReadOnly {
		t.Fatalf("Store = %+v, %v", infos, err)
	}
	names := func(id port.StoreID) string {
		var out []string
		if err := deps.Stores.Read(t.Context(), id, func(r port.Reader) error {
			ws, err := r.Workspaces().List()
			for _, w := range ws {
				out = append(out, w.Name+"/"+w.Prefix)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return strings.Join(out, ",")
	}
	if got := names(e2eStore); got != "Web/HH,Mobile/MB" {
		t.Errorf("e2e のワークスペース = %s", got)
	}
	if got := names(teamStore); got != "Team/TM" {
		t.Errorf("team のワークスペース = %s", got)
	}
}

// 同じ Store の移動、別の Store(remote なし)への移動、remote あり(e2e の Store。偽の Replicator)への移動が、
// 本物の Delivery(syncing.Service)で通る。
func TestE2EWireMovesItemsWithinAndAcrossStores(t *testing.T) {
	deps := e2eDeps(t)
	mux := http.NewServeMux()
	httpapi.NewAPI(mux, deps)
	post := func(path string, body map[string]any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b)))
		return rec
	}
	transfer := func(store, id string) string { return "/api/stores/" + store + "/items/" + id + "/transfer" }
	none := map[string]string{}
	// 同じ Store の Mobile へ → team(remote なし)へ → e2e(remote あり。偽の Replicator)へ戻す。
	steps := []struct {
		path string
		body map[string]any
		want string
		left left
	}{
		{transfer(e2eStore, string(apptest.SampleTask)), map[string]any{"toStore": e2eStore, "toWorkspace": apptest.SampleMobileWorkspace, "toProject": apptest.SampleMobileProject, "toParent": apptest.SampleMobileEpic, "statusChoices": none}, `"display":"MB-2"`, left{e2eStore, apptest.SampleProject, 2, false}},
		{transfer(e2eStore, string(apptest.SampleTask)), map[string]any{"toStore": teamStore, "toWorkspace": apptest.SampleTeamWorkspace, "toProject": apptest.SampleTeamProject, "toParent": apptest.SampleTeamEpic, "statusChoices": none}, `"display":"TM-2"`, left{e2eStore, apptest.SampleMobileProject, 2, true}},
		{transfer(teamStore, string(apptest.SampleTask)), map[string]any{"toStore": e2eStore, "toWorkspace": apptest.SampleWorkspace, "toProject": apptest.SampleProject, "toParent": apptest.SampleEpic, "statusChoices": none}, `"display":"HH-3"`, left{teamStore, apptest.SampleTeamProject, 2, true}},
	}
	for i, s := range steps {
		if rec := post(s.path, s.body); rec.Code != 200 || !strings.Contains(rec.Body.String(), s.want) {
			t.Fatalf("手順 %d: %d %s", i+1, rec.Code, rec.Body)
		}
		s.left.check(t, deps.Stores, i+1)
	}
}

// left は、移したあとの移動元。移動元のプロジェクトに旧い番号のアイテムがない。cross(別の Store への移動)なら、
// 移動元の Store にアイテムがない。
type left struct {
	store   port.StoreID
	project project.ID
	number  int
	cross   bool
}

func (l left) check(t *testing.T, s port.Stores, step int) {
	t.Helper()
	if err := s.Read(t.Context(), l.store, func(r port.Reader) error {
		all, err := r.Items().List(l.project)
		if err != nil {
			return err
		}
		for _, it := range all {
			if it.Place.Number == l.number {
				t.Errorf("手順 %d: 移動元に旧い番号 %d のアイテムが残っている: %+v", step, l.number, it)
			}
		}
		if _, err := r.Items().Get(apptest.SampleTask); l.cross && !errors.Is(err, port.ErrNotFound) {
			t.Errorf("手順 %d: 別の Store へ移したのに移動元で読める: %v", step, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
