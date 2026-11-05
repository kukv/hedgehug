package report

import (
	"reflect"
	"testing"
	"time"
)

var day0 = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

// at is hours after the midnight of day0.
func at(hours int) time.Time { return day0.Add(time.Duration(hours) * time.Hour) }

func cat(status string) string {
	switch status {
	case "todo", "doing", "done", "canceled":
		return status
	}
	return Unknown
}

const s1 = "S1"

// ver is a version of a task in the sprint.
func ver(hours int, status string, points float64) Version {
	return Version{When: at(hours), Status: status, Sprint: s1, Points: points, HasPoints: points > 0, Created: at(0)}
}

func task(id string, vs ...Version) Task { return Task{ULID: id, Versions: vs} }

func threeDays() []Day { return Days(at(0), at(48), at(72), time.UTC) }

func TestDays(t *testing.T) {
	t.Parallel()
	got := Days(at(5), at(30), at(40), time.UTC)
	if len(got) != 2 || got[0].Date != "2026-11-02" || got[1].Date != "2026-11-03" {
		t.Fatalf("days = %+v", got)
	}
	if !got[1].At.Equal(at(40)) {
		t.Errorf("the cutoff is not the last instant: %v", got[1].At)
	}
	tokyo := time.FixedZone("JST", 9*3600)
	// 20:00 UTC is the next day in Tokyo.
	if got := Days(at(20), at(20), at(72), tokyo); len(got) != 1 || got[0].Date != "2026-11-03" {
		t.Fatalf("days in Tokyo = %+v", got)
	}
}

func TestBurndown(t *testing.T) {
	t.Parallel()
	w := Window{Sprint: s1, Start: at(1), End: at(71), CloseCommit: "close"}
	moved := ver(71, "todo", 2)
	moved.Commit, moved.Sprint = "close", "S2"
	tasks := []Task{
		task("done on the second day", ver(0, "todo", 3), ver(30, "done", 3)),
		task("moved by the close", ver(0, "todo", 2), moved),
		task("left on the second day", ver(0, "todo", 1), Version{When: at(26), Status: "todo", Created: at(0)}),
		task("joined on the third day", Version{When: at(0), Status: "todo", Created: at(0)}, ver(50, "todo", 5)),
		task("canceled on the third day", ver(0, "todo", 4), ver(50, "canceled", 4)),
		task("deleted on the third day", ver(0, "todo", 1), func() Version { v := ver(50, "todo", 1); v.Deleted = true; return v }()),
		task("reopened on the third day", ver(0, "todo", 2), ver(30, "done", 2), ver(55, "doing", 2)),
	}
	got := Burndown(tasks, w, threeDays(), cat, true)
	want := []BurndownDay{
		{Date: "2026-11-02", Scope: 13, Done: 0, Remaining: 13},
		{Date: "2026-11-03", Scope: 12, Done: 5, Remaining: 7},
		{Date: "2026-11-04", Scope: 12, Done: 3, Remaining: 9, Added: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("burndown:\n got %+v\nwant %+v", got, want)
	}
	counted := Burndown(tasks, w, threeDays(), cat, false)
	if counted[0].Scope != 6 || counted[2].Added != 1 {
		t.Fatalf("by count = %+v", counted)
	}
}

func TestVelocity(t *testing.T) {
	t.Parallel()
	w := Window{Sprint: s1, Start: at(1), End: at(71), CloseCommit: "close"}
	moved := ver(71, "todo", 2)
	moved.Commit, moved.Sprint = "close", "S2"
	got := Velocity([]Task{
		task("a", ver(0, "todo", 3), ver(30, "done", 3)),
		task("b", ver(0, "todo", 0), ver(30, "done", 0)),
		task("open", ver(0, "todo", 2), moved),
		task("after the end", ver(0, "todo", 8), ver(80, "done", 8)),
	}, w, cat)
	if want := (Completed{Points: 3, Tasks: 2, Unpointed: 1}); got != want {
		t.Fatalf("velocity = %+v", got)
	}
}

func TestCFD(t *testing.T) {
	t.Parallel()
	ver := func(hours int, status string) Version {
		return Version{When: at(hours), Status: status, Workspace: "w", Created: at(0)}
	}
	gone := ver(30, "todo")
	gone.Gone = true
	other := ver(0, "todo")
	other.Workspace = "x"
	tasks := []Task{
		task("a", ver(0, "todo"), ver(30, "done")),
		task("b", ver(0, "review")),
		task("c", ver(0, "todo"), gone),
		task("d", other),
		task("not yet", ver(50, "todo")),
	}
	got := CFD(tasks, threeDays(), "w", cat)
	want := []CFDDay{
		{"2026-11-02", map[string]int{"todo": 2, Unknown: 1}},
		{"2026-11-03", map[string]int{"done": 1, Unknown: 1}},
		{"2026-11-04", map[string]int{"done": 1, Unknown: 1, "todo": 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cfd:\n got %+v\nwant %+v", got, want)
	}
}

func TestFinished(t *testing.T) {
	t.Parallel()
	ver := func(hours int, status string) Version {
		return Version{When: at(hours), Status: status, Created: at(0)}
	}
	got := Finished([]Task{
		task("a", ver(0, "todo"), ver(10, "doing"), ver(20, "doing"), ver(30, "done")),
		task("reopened", ver(0, "todo"), ver(5, "done"), ver(10, "doing"), ver(40, "done"), ver(41, "done")),
		task("no doing", ver(0, "todo"), ver(30, "done")),
		task("open again", ver(0, "todo"), ver(5, "done"), ver(10, "todo")),
		task("too early", ver(0, "done")),
	}, "", at(1), at(100), cat)
	if len(got) != 3 {
		t.Fatalf("finished = %+v", got)
	}
	if got[0].Cycle != 20*time.Hour || got[0].Lead != 30*time.Hour || !got[0].HasCycle {
		t.Errorf("a = %+v", got[0])
	}
	if got[1].Cycle != 30*time.Hour || got[1].Lead != 40*time.Hour {
		t.Errorf("reopened = %+v", got[1])
	}
	if got[2].HasCycle {
		t.Errorf("no doing = %+v", got[2])
	}
}

func TestPercentile(t *testing.T) {
	t.Parallel()
	ds := []time.Duration{5, 1, 3, 2, 4}
	if got := Percentile(ds, 0.5); got != 3 {
		t.Errorf("median = %v", got)
	}
	if got := Percentile(ds, 0.85); got != 5 {
		t.Errorf("p85 = %v", got)
	}
	if got := Percentile(nil, 0.5); got != 0 {
		t.Errorf("empty = %v", got)
	}
}
