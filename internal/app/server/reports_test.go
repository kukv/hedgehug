package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/reports"
)

func TestReportAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("a", "")
	ts.call("POST", "/api/sprints", map[string]any{"name": "s1", "start": "2026-10-28", "end": "2026-10-30"}, 201, nil)
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"sprint": "s1", "props": map[string]any{"points": 3, "estimate": "2h"}}, 200, nil)
	ts.call("POST", "/api/sprints/s1/start", nil, 200, nil)
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"status": "done"}, 200, nil)
	ts.call("POST", "/api/tasks/"+id+"/worklogs", map[string]any{"duration": "1h"}, 201, nil)

	var b reports.BurndownView
	ts.call("GET", "/api/reports/burndown", nil, 200, &b)
	if b.Sprint.Name != "s1" || b.By != "points" || len(b.Days) == 0 || b.Days[len(b.Days)-1].Done != 3 {
		t.Fatalf("burndown = %+v", b)
	}
	ts.problem("GET", "/api/reports/burndown?sprint=nope", nil, 404)
	ts.problem("GET", "/api/reports/burndown?by=hours", nil, 400)

	ts.call("POST", "/api/sprints/s1/close", nil, 200, nil)
	var vs []reports.VelocityRow
	ts.call("GET", "/api/reports/velocity", nil, 200, &vs)
	if len(vs) != 1 || vs[0].Points != 3 {
		t.Fatalf("velocity = %+v", vs)
	}

	var c reports.CFDView
	ts.call("GET", "/api/reports/cfd", nil, 200, &c)
	if len(c.Columns) != 4 || len(c.Days) == 0 || c.Days[len(c.Days)-1].Counts["done"] != 1 {
		t.Fatalf("cfd = %+v", c)
	}
	var cy reports.CycleView
	ts.call("GET", "/api/reports/cycle", nil, 200, &cy)
	if cy.Lead.Count != 1 {
		t.Fatalf("cycle = %+v", cy)
	}
	var e reports.EstimateView
	ts.call("GET", "/api/reports/estimate", nil, 200, &e)
	if e.Estimate != "2h" || e.Actual != "1h" {
		t.Fatalf("estimate = %+v", e)
	}
}
