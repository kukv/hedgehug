package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/usecase/reports"
)

// A task that moved between stores keeps the time it spent in doing there.
func TestReportCycleFollowsAMove(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	te.ok("store", "init", "other", "--prefix", "OTH")
	te.now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)
	a := te.addJSON("a", "--store", "me")
	te.now = te.now.Add(time.Hour)
	te.ok("set", a.ID, "status=in_progress", "--store", "me")
	te.now = te.now.Add(time.Hour)
	te.ok("mv", a.ULID, "--store", "other", "--yes")
	te.now = te.now.Add(time.Hour)
	te.ok("set", a.ULID, "status=done", "--store", "other")

	var v reports.CycleView
	if err := json.Unmarshal([]byte(te.ok("report", "cycle", "--store", "other", "--json")), &v); err != nil {
		t.Fatal(err)
	}
	if v.Cycle.Count != 1 || v.Cycle.Median != "2h" || v.Lead.Median != "3h" || len(v.Warnings) != 0 {
		t.Fatalf("cycle = %+v", v)
	}

	// With the store it came from gone, the times start at the move.
	te.ok("store", "remove", "me", "--keep")
	out := te.ok("report", "cycle", "--store", "other", "--json")
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Warnings) != 1 || !strings.Contains(v.Warnings[0], "not mounted") {
		t.Fatalf("cycle = %+v", v)
	}
}
