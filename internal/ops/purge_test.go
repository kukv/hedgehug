package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
)

// addDeleted writes a task that was deleted at.
func (ts *testSession) addDeleted(title string, at time.Time, edit func(*model.Task)) *model.Task {
	ts.t.Helper()
	return ts.addTask(title, func(t *model.Task) {
		t.DeletedAt = &at
		if edit != nil {
			edit(t)
		}
	})
}

func daysAgo(ts *testSession, n int) time.Time { return ts.Now().AddDate(0, 0, -n) }

func purgedULIDs(p PurgePlan) []model.ULID {
	var out []model.ULID
	for _, t := range p.Tasks {
		out = append(out, t.ULID)
	}
	return out
}

func lastOp(t *testing.T, ts *testSession) string {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().LogFrom(head, 1)
	if err != nil || len(log) == 0 {
		t.Fatalf("log = %v, %v", log, err)
	}
	op, _ := log[0].Trailer("Hedgehug-Op")
	return op
}

func TestParseOlderThan(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]int{"90d": 90, "0d": 0, "1d": 1, "007d": 7, "36500d": 36500} {
		if got, err := ParseOlderThan(in); err != nil || got != want {
			t.Errorf("ParseOlderThan(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "d", "90", "-5d", "+5d", "1w", "9d0", "1.5d", "36501d", "99999999999999999999d", " 5d"} {
		_, err := ParseOlderThan(in)
		wantErr(t, err, KindUsage, "purge.bad_period")
	}
}

func TestPurgeRemovesWhatIsOlder(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	old := ts.addDeleted("old", daysAgo(ts, 100), nil)
	recent := ts.addDeleted("recent", daysAgo(ts, 10), nil)
	live := ts.addTask("live", nil)

	plan, err := ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	if got := purgedULIDs(plan); len(got) != 1 || got[0] != old.ULID || plan.Store != "me" || plan.Tasks[0].Title != "old" || plan.Tasks[0].ID != old.ID {
		t.Fatalf("plan = %+v; want only the old one", plan)
	}
	_, err = ts.Purge(90, false)
	wantErr(t, err, KindUsage, "confirm.required")
	if _, ok := mustIndex2(t, ts).Task(old.ULID); !ok {
		t.Fatal("a refused purge removed the task")
	}

	res, err := ts.Purge(90, true)
	if err != nil || res.Commit == "" || len(res.Tasks) != 1 {
		t.Fatalf("Purge = %+v, %v", res, err)
	}
	if op := lastOp(t, ts); op != "purge" {
		t.Errorf("Hedgehug-Op = %q; want purge", op)
	}
	ix := mustIndex2(t, ts)
	for _, c := range []struct {
		u    model.ULID
		want bool
	}{{old.ULID, false}, {recent.ULID, true}, {live.ULID, true}} {
		if _, ok := ix.Task(c.u); ok != c.want {
			t.Errorf("task %s present = %v; want %v", c.u, ok, c.want)
		}
	}
}

// The boundary: what was deleted exactly N days ago is older than N days, so
// 0d takes what was deleted just now.
func TestPurgeBoundary(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addDeleted("just now", ts.Now(), nil)
	ts.addDeleted("a day ago", daysAgo(ts, 1), nil)
	for days, want := range map[int]int{0: 2, 1: 1, 2: 0} {
		plan, err := ts.PurgeList(days)
		if err != nil || len(plan.Tasks) != want {
			t.Errorf("PurgeList(%d) = %d tasks, %v; want %d", days, len(plan.Tasks), err, want)
		}
	}
}

func TestPurgeNothingIsNoCommit(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addTask("live", nil)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before := headOf(t, s)
	for _, yes := range []bool{false, true} {
		res, err := ts.Purge(0, yes)
		if err != nil || !res.Empty() || res.Commit != "" || res.Tasks == nil || res.Forwards == nil {
			t.Errorf("yes=%v: %+v, %v; want an empty plan with empty lists and no commit", yes, res, err)
		}
	}
	if headOf(t, s) != before {
		t.Error("head moved")
	}
}

// A deleted task with a task under it that is not deleted stays, and the
// warning names it. Its deleted children go.
func TestPurgeKeepsATaskWithALiveChild(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	at := daysAgo(ts, 100)
	parent := ts.addDeleted("parent", at, nil)
	live := ts.addTask("live child", func(t *model.Task) { t.Parent = &parent.ULID })
	gone := ts.addDeleted("gone child", at, func(t *model.Task) { t.Parent = &parent.ULID })

	plan, err := ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	if got := purgedULIDs(plan); len(got) != 1 || got[0] != gone.ULID {
		t.Fatalf("plan = %+v; want only the deleted child", plan)
	}
	if len(ts.warns) != 1 || !strings.Contains(ts.warns[0], parent.ID) || !strings.Contains(ts.warns[0], "is not deleted") {
		t.Errorf("warnings = %q; want one naming %s", ts.warns, parent.ID)
	}
	if _, err := ts.Purge(90, true); err != nil {
		t.Fatal(err)
	}
	ix := mustIndex2(t, ts)
	if _, ok := ix.Task(parent.ULID); !ok {
		t.Error("the parent was purged")
	}
	if _, ok := ix.Task(live.ULID); !ok {
		t.Error("the live child was purged")
	}
}

// A task whose child was deleted later than the limit waits for the child.
func TestPurgeWaitsForARecentlyDeletedChild(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	parent := ts.addDeleted("parent", daysAgo(ts, 100), nil)
	ts.addDeleted("child", daysAgo(ts, 10), func(t *model.Task) { t.Parent = &parent.ULID })
	plan, err := ts.PurgeList(90)
	if err != nil || !plan.Empty() || len(ts.warns) != 0 {
		t.Errorf("plan = %+v, %v, warnings %q; want nothing and no warning", plan, err, ts.warns)
	}
}

func TestPurgeRecordsTheNumbers(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	for _, title := range []string{"a", "b"} {
		if _, err := ts.Add(AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"ME-1", "ME-2"} {
		if _, err := ts.Remove(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Purge(0, true); err != nil {
		t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.Numbers()
	if err != nil || n["ME"] != 2 {
		t.Errorf("numbers = %v, %v; want ME: 2", n, err)
	}
}

func TestPurgeConfirmedRefusesChanges(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addDeleted("first", daysAgo(ts, 100), nil)
	plan, err := ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	// a task that is old enough turns up after the list
	later := ts.addDeleted("later", daysAgo(ts, 100), nil)
	_, err = ts.PurgeConfirmed(90, plan)
	wantErr(t, err, KindConflict, "purge.changed")
	if _, ok := mustIndex2(t, ts).Task(later.ULID); !ok {
		t.Error("a refused purge removed a task")
	}
	// nothing confirmed is no confirmation
	_, err = ts.PurgeConfirmed(90, PurgePlan{})
	wantErr(t, err, KindUsage, "confirm.required")

	plan, err = ts.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ts.PurgeConfirmed(90, plan)
	if err != nil || len(res.Tasks) != 2 || res.Commit == "" {
		t.Errorf("PurgeConfirmed = %+v, %v; want both tasks", res, err)
	}
}

func mustIndex2(t *testing.T, ts *testSession) *index.Index {
	t.Helper()
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	return ix
}
