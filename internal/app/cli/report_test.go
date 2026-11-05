package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase/reports"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// sprintRun runs one sprint day by day, in the local time zone of the test
// (the clock of the env, which also dates the commits).
func sprintRun(te *testEnv) {
	te.now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)
	te.ok("sprint", "add", "s1", "--start", "2026-11-02", "--end", "2026-11-04")
	te.ok("sprint", "add", "s2", "--start", "2026-11-05", "--end", "2026-11-06")
	done := te.addJSON("done", "--sprint", "s1", "-p", "points=3").ID
	te.addJSON("moved", "--sprint", "s1", "-p", "points=2")
	te.now = te.now.Add(time.Hour)
	te.ok("sprint", "start", "s1")

	te.now = te.now.Add(24 * time.Hour) // the 3rd
	te.ok("set", done, "status=done")
	late := te.addJSON("late", "--sprint", "s1", "-p", "points=5").ID

	te.now = te.now.Add(24 * time.Hour) // the 4th
	te.ok("set", late, "status=canceled")
	te.ok("sprint", "close", "s1", "--to", "next")
}

func burndownOf(t *testing.T, te *testEnv, args ...string) reports.BurndownView {
	t.Helper()
	var v reports.BurndownView
	if err := json.Unmarshal([]byte(te.ok(append([]string{"report", "burndown", "--json"}, args...)...)), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestReportBurndownOfAClosedSprint(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	sprintRun(te)

	v := burndownOf(t, te, "s1")
	if v.By != "points" || len(v.Days) != 3 {
		t.Fatalf("burndown = %+v", v)
	}
	want := [][4]float64{ // scope, done, remaining, added
		{5, 0, 5, 0},
		{10, 3, 7, 5},
		{5, 3, 2, 0},
	}
	for i, w := range want {
		d := v.Days[i]
		if d.Scope != w[0] || d.Done != w[1] || d.Remaining != w[2] || d.Added != w[3] {
			t.Errorf("day %d = %+v, want %v", i, d, w)
		}
	}
	if out := te.ok("report", "burndown", "s1"); !strings.Contains(out, "s1 (closed), by points") || !strings.Contains(out, "█") {
		t.Errorf("text:\n%s", out)
	}
	if v := burndownOf(t, te, "s1", "--by", "count"); v.Days[0].Scope != 2 {
		t.Errorf("by count = %+v", v.Days)
	}
}

func TestReportVelocity(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	sprintRun(te)
	var vs []reports.VelocityRow
	if err := json.Unmarshal([]byte(te.ok("report", "velocity", "--json")), &vs); err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Sprint.Name != "s1" || vs[0].Points != 3 || vs[0].Tasks != 1 {
		t.Fatalf("velocity = %+v", vs)
	}
	if out := te.ok("report", "velocity"); !strings.Contains(out, "s1") {
		t.Errorf("text:\n%s", out)
	}
}

func TestReportBurndownPicksTheActiveSprint(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	if code, _, stderr := te.run("report", "burndown"); code != 2 || !strings.Contains(stderr, "0 sprints are active") {
		t.Fatalf("code %d, stderr %s", code, stderr)
	}
	te.now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)
	addStartedSprint(te)
	te.addJSON("a", "--sprint", "s1")
	v := burndownOf(t, te)
	if v.Sprint.Name != "s1" || v.By != "count" || v.Note == "" {
		t.Fatalf("burndown = %+v", v)
	}
	te.ok("sprint", "add", "planned")
	if code, _, stderr := te.run("report", "burndown", "planned"); code == 0 || !strings.Contains(stderr, "has not started") {
		t.Fatalf("code %d, stderr %s", code, stderr)
	}
}

func TestReportOfASprintClosedByHand(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)
	a := te.addJSON("a", "-p", "points=2")
	te.ok("sprint", "add", "s1", "--start", "2026-11-01", "--end", "2026-11-03")
	te.ok("set", a.ID, "sprint=s1", "status=done")
	dir := te.storeDir("me")
	files, err := filepath.Glob(filepath.Join(dir, "sprints", "*.md"))
	if err != nil || len(files) != 1 {
		t.Fatalf("sprint files = %v, %v", files, err)
	}
	b, err := os.ReadFile(files[0]) //nolint:gosec // a file of the test's own store
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(files[0], []byte(strings.Replace(string(b), "state: planned", "state: closed", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(dir, files[0])
	if err != nil {
		t.Fatal(err)
	}
	gitrepotest.CommitFiles(t, dir, "hand edit", "Alice", "alice@example.com", te.now, filepath.ToSlash(rel))
	v := burndownOf(t, te, "s1")
	if len(v.Days) != 3 || v.Days[2].Done != 2 {
		t.Fatalf("burndown = %+v", v)
	}
}

func TestReportCFD(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	sprintRun(te)
	var v reports.CFDView
	if err := json.Unmarshal([]byte(te.ok("report", "cfd", "--since", "2026-11-02", "--until", "2026-11-03", "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Days) != 2 || v.Days[0].Counts["todo"] != 2 || v.Days[1].Counts["done"] != 1 || v.Days[1].Counts["todo"] != 2 {
		t.Fatalf("cfd = %+v", v)
	}
	if out := te.ok("report", "cfd", "--since", "2026-11-02", "--by", "status"); !strings.Contains(out, "IN_PROGRESS") {
		t.Errorf("by status:\n%s", out)
	}
	if code, _, _ := te.run("report", "cfd", "--since", "soon"); code != 2 {
		t.Errorf("code = %d", code)
	}
	if code, _, _ := te.run("report", "cfd", "--by", "kind"); code != 2 {
		t.Errorf("code = %d", code)
	}
}

func TestReportCycle(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)
	a := te.addJSON("a")
	te.now = te.now.Add(time.Hour)
	te.ok("set", a.ID, "status=in_progress")
	te.now = te.now.Add(2 * time.Hour)
	te.ok("set", a.ID, "status=done")
	var v reports.CycleView
	if err := json.Unmarshal([]byte(te.ok("report", "cycle", "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if v.Cycle.Count != 1 || v.Cycle.Median != "2h" || v.Lead.Median != "3h" {
		t.Fatalf("cycle = %+v", v)
	}
	if out := te.ok("report", "cycle"); !strings.Contains(out, "cycle") {
		t.Errorf("text:\n%s", out)
	}
}

func TestReportEstimate(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("sprint", "add", "s1")
	a := te.addJSON("a", "--sprint", "s1", "-p", "estimate=3h")
	b := te.addJSON("b")
	te.addJSON("neither")
	te.ok("worklog", a.ID, "1h")
	te.ok("worklog", a.ID, "1h30m")
	te.ok("worklog", b.ID, "45m")
	var v reports.EstimateView
	if err := json.Unmarshal([]byte(te.ok("report", "estimate", "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Tasks) != 2 || v.Estimate != "3h" || v.Actual != "3h15m" {
		t.Fatalf("estimate = %+v", v)
	}
	if err := json.Unmarshal([]byte(te.ok("report", "estimate", "--sprint", "s1", "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Tasks) != 1 || v.Actual != "2h30m" {
		t.Fatalf("estimate of s1 = %+v", v)
	}
	if out := te.ok("report", "estimate"); !strings.Contains(out, "total") {
		t.Errorf("text:\n%s", out)
	}
}
