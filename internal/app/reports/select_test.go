package reports_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/reports"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
)

func ptr[T any](v T) *T { return &v }

func TestSelectPatch(t *testing.T) {
	svc, sel, s := setup(t)
	activeWithWork(t, s)
	ws := apptest.SampleWorkspace
	if err := svc.Select(t.Context(), store, ws, reports.Patch{Unit: ptr(dp.UnitCount)}); err != nil {
		t.Fatal(err)
	}
	if got := sel.rows[ws]; got != (dp.Selection{Unit: dp.UnitCount}) {
		t.Fatalf("保存した行 = %+v", got)
	}

	// 実行中のスプリントを替える。保存した行の Sprint は空のままなので、バーンダウンは新しい実行中に移る。
	active := dp.Sprint{ID: sprint1, Name: "Sprint 1", State: dp.Active, Start: "2026-11-10", End: "2026-11-20"}
	closed := active
	closed.State = dp.Closed
	writeWS(t, s, dp.SprintDrafts(active, closed)...)
	addSprint(t, s, sprint2, "Sprint 2", dp.Active, "2026-11-14", "2026-11-28")
	r, err := svc.Report(t.Context(), store, ws, reports.Period{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Selection.Sprint != sprint2 || sel.rows[ws].Sprint != "" {
		t.Errorf("Sprint = %q, 保存した行 = %+v", r.Selection.Sprint, sel.rows[ws])
	}

	// バーンダウンのスプリントだけを送ると、前の単位は残る。
	if err := svc.Select(t.Context(), store, ws, reports.Patch{Sprint: ptr(sprint1)}); err != nil {
		t.Fatal(err)
	}
	if got := sel.rows[ws]; got != (dp.Selection{Unit: dp.UnitCount, Sprint: sprint1}) {
		t.Errorf("保存した行 = %+v", got)
	}
	if err := svc.Select(t.Context(), store, ws, reports.Patch{Layers: ptr(dp.LayersStatus), Estimate: ptr(sprint2)}); err != nil {
		t.Fatal(err)
	}
	want := dp.Selection{Unit: dp.UnitCount, Sprint: sprint1, Layers: dp.LayersStatus, Estimate: sprint2}
	if got := sel.rows[ws]; got != want {
		t.Errorf("保存した行 = %+v", got)
	}
}

func TestSelect(t *testing.T) {
	svc, sel, s := setup(t)
	ws := apptest.SampleWorkspace
	svc.Stores = readOnly{s}
	if err := svc.Select(t.Context(), store, ws, reports.Patch{Unit: ptr(dp.UnitCount)}); err != nil {
		t.Fatalf("閲覧のみの Store: %v", err)
	}
	if sel.puts != 1 {
		t.Errorf("Put = %d 回", sel.puts)
	}

	for name, p := range map[string]reports.Patch{
		"単位": {Unit: ptr(dp.Unit("hours"))},
		"層":  {Layers: ptr(dp.Layers("weeks"))},
	} {
		if err := svc.Select(t.Context(), store, ws, p); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if sel.puts != 1 {
		t.Errorf("不正な値で Put した: %d 回", sel.puts)
	}

	if err := apptest.DeleteWorkspace(t.Context(), s, store, ws); err != nil {
		t.Fatal(err)
	}
	if err := svc.Select(t.Context(), store, ws, reports.Patch{Unit: ptr(dp.UnitCount)}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済み: err = %v", err)
	}
}
