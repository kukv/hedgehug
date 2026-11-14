package reports_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/reports"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appreports "github.com/kukv/hedgehug/internal/app/reports"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/worklog"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const base = "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace)

// selections は、メモリ上のレポートの選択の保存先。
type selections struct {
	rows map[workspace.ID]planning.Selection
}

func (s *selections) Get(_ context.Context, _ port.StoreID, ws workspace.ID) (planning.Selection, bool, error) {
	sel, ok := s.rows[ws]
	return sel, ok, nil
}

func (s *selections) Put(_ context.Context, _ port.StoreID, ws workspace.ID, sel planning.Selection) error {
	s.rows[ws] = sel
	return nil
}

// newAPI は、サンプルを書いた Store S1 のレポートの API。今日は 2026-11-13。
func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	api, _ := newAPIWithStores(t)
	return api
}

func newAPIWithStores(t *testing.T) (humatest.TestAPI, port.Stores) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	reports.Register(api, appreports.Service{Stores: s, Selections: &selections{rows: map[workspace.ID]planning.Selection{}}, Clock: clock})
	return api, s
}

// addActive は、実行中のスプリント(11-10〜11-20)と、そこに入れた作業時間の記録のある作業項目 1 件を足す。
func addActive(t *testing.T, s port.Stores) {
	t.Helper()
	const sp change.ID = "01SAMPLESP0000000000000001"
	const id item.ID = "01WORKITEM00000000000000A1"
	planned := planning.Sprint{ID: sp, Name: "Sprint 1", State: planning.Planned}
	active := planned
	active.State, active.Start, active.End = planning.Active, "2026-11-10", "2026-11-20"
	points, estimate := 3.0, 2.0
	it := item.Item{
		ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3},
		Kind: item.Task, Rank: "a", Title: "A", Status: apptest.SampleStatusInProgress, Sprint: sp, Points: &points, Estimate: &estimate,
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	err := s.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		w := tx.WriteWorkspaces()
		if err := w.Append(apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, planned)...); err != nil {
			return err
		}
		if err := w.Append(apptest.SampleWorkspace, planning.SprintDrafts(planned, active)...); err != nil {
			return err
		}
		if err := w.Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, 3)); err != nil {
			return err
		}
		return tx.WriteItems().Append(id, item.Create(it), change.NewWorklog("L1", nil, worklog.Value(150, "", "2026-11-11")))
	})
	if err != nil {
		t.Fatal(err)
	}
}

// decode は、本文を JSON の map にする。
func decode(t *testing.T, res *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if res.Code != http.StatusOK {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	var m map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// nonEmpty は、m[key] が空でない配列か。
func nonEmpty(t *testing.T, name string, v any) {
	t.Helper()
	if a, ok := v.([]any); !ok || len(a) == 0 {
		t.Errorf("%s が空: %v", name, v)
	}
}

// 実行中のスプリントがあるときの JSON の形。
func TestReportsWithActive(t *testing.T) {
	api, s := newAPIWithStores(t)
	addActive(t, s)

	r := decode(t, api.Get(base+"/reports"))
	for _, k := range []string{"sprints", "velocity", "flow", "layers", "estimates"} {
		nonEmpty(t, k, r[k])
	}
	if burn, ok := r["burn"].(map[string]any); !ok || burn["name"] != "Sprint 1" || burn["state"] != "active" {
		t.Errorf("burn = %v", r["burn"])
	}
	bd, _ := r["burndown"].(map[string]any)
	nonEmpty(t, "burndown.days", bd["days"])
	if bd["shown"] != 4.0 {
		t.Errorf("burndown.shown = %v, want 4", bd["shown"])
	}

	a := decode(t, api.Get(base+"/reports/active"))
	if sp, ok := a["sprint"].(map[string]any); !ok || sp["name"] != "Sprint 1" {
		t.Errorf("sprint = %v", a["sprint"])
	}
	if a["unit"] != "points" || a["done"] == nil || a["total"] == nil {
		t.Errorf("unit = %v, done = %v, total = %v", a["unit"], a["done"], a["total"])
	}
	abd, _ := a["burndown"].(map[string]any)
	nonEmpty(t, "active burndown.days", abd["days"])
}

func check(t *testing.T, name string, res *httptest.ResponseRecorder, code int, contains string) {
	t.Helper()
	body := res.Body.Bytes()
	var buf bytes.Buffer
	if json.Compact(&buf, body) == nil { // 本文は整形されているので、詰めてから探す
		body = buf.Bytes()
	}
	if res.Code != code || !strings.Contains(string(body), contains) {
		t.Errorf("%s: %d %s", name, res.Code, res.Body)
	}
}

func TestGetReport(t *testing.T) {
	api := newAPI(t)
	check(t, "report", api.Get(base+"/reports"), http.StatusOK, `"unit":"points"`)
	check(t, "period", api.Get(base+"/reports"), http.StatusOK, `"period":{"from":"2026-10-24","to":"2026-11-13"}`)
	check(t, "selection", api.Get(base+"/reports"), http.StatusOK, `"selection":{"unit":"points","sprint":"","layers":"category","estimate":""}`)
	check(t, "burn null", api.Get(base+"/reports"), http.StatusOK, `"burn":null`)
	check(t, "bad date", api.Get(base+"/reports?from=2026-13-01&to=2026-11-13"), http.StatusUnprocessableEntity, "not a date: 2026-13-01")
	check(t, "future", api.Get(base+"/reports?from=2026-11-01&to=2026-11-14"), http.StatusUnprocessableEntity, "終了日は今日以前にしてください")
	check(t, "ok period", api.Get(base+"/reports?from=2026-11-01&to=2026-11-13"), http.StatusOK, `"period":{"from":"2026-11-01","to":"2026-11-13"}`)
	check(t, "unknown workspace", api.Get("/api/stores/S1/workspaces/01NOPE0000000000000000000/reports"), http.StatusNotFound, "")
}

func TestPutSelection(t *testing.T) {
	api := newAPI(t)
	check(t, "put", api.Put(base+"/reports/selection", map[string]any{"unit": "count"}), http.StatusNoContent, "")
	check(t, "reflected", api.Get(base+"/reports"), http.StatusOK, `"selection":{"unit":"count","sprint":"","layers":"category","estimate":""}`)
	check(t, "bad unit", api.Put(base+"/reports/selection", map[string]any{"unit": "x"}), http.StatusBadRequest, "")
	check(t, "bad layers", api.Put(base+"/reports/selection", map[string]any{"layers": "x"}), http.StatusBadRequest, "")
	check(t, "empty body", api.Put(base+"/reports/selection", map[string]any{}), http.StatusNoContent, "")
	check(t, "unchanged", api.Get(base+"/reports"), http.StatusOK, `"selection":{"unit":"count"`)
	check(t, "unknown workspace", api.Put("/api/stores/S1/workspaces/01NOPE0000000000000000000/reports/selection", map[string]any{"unit": "count"}), http.StatusNotFound, "")
}

func TestGetActive(t *testing.T) {
	api := newAPI(t)
	check(t, "no active", api.Get(base+"/reports/active"), http.StatusOK, `"sprint":null`)
	check(t, "no active unit", api.Get(base+"/reports/active"), http.StatusOK, `"unit":"points"`)
	check(t, "unknown workspace", api.Get("/api/stores/S1/workspaces/01NOPE0000000000000000000/reports/active"), http.StatusNotFound, "")
}
