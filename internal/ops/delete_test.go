package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
)

func TestRemoveRestorePurge(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "p"}, {Title: "c", Parent: "ME-1"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	r, err := ts.Remove("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Targets) != 2 || r.Entry.ID != "ME-1" || !r.Committed || r.Index == nil {
		t.Fatalf("Remove = %d targets, entry %s, committed %v; want 2 (the child goes with it), ME-1, true", len(r.Targets), r.Entry.ID, r.Committed)
	}
	_, err = ts.Remove("ME-1")
	wantErr(t, err, KindConflict, "task.already_deleted")
	if err != nil && (!strings.HasPrefix(err.Error(), "ME-1 ") || !strings.HasSuffix(err.Error(), " is already deleted")) {
		t.Errorf("message = %q", err)
	}
	r, err = ts.Restore("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Targets) != 2 || r.Targets[0].ID != "ME-1" || r.Targets[1].ID != "ME-2" {
		t.Errorf("Restore targets = %v; want ME-1 and ME-2", r.Targets)
	}
	_, _, _, err = ts.PurgeTargets("ME-1")
	wantErr(t, err, KindConflict, "task.not_deleted")
	if err != nil && !strings.HasSuffix(err.Error(), " is not deleted; delete it with hedgehug rm first") {
		t.Errorf("message = %q", err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	ix, e, targets, err := ts.PurgeTargets("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if ix == nil || e.ID != "ME-1" || len(targets) != 2 {
		t.Fatalf("PurgeTargets = %v %v %d targets; want ME-1 and 2 targets", ix != nil, e.ID, len(targets))
	}
	pr, err := ts.Purge("ME-1", targets)
	if err != nil {
		t.Fatal(err)
	}
	if !pr.Committed || len(pr.Targets) != 2 || pr.Entry.ID != "ME-1" {
		t.Errorf("Purge = committed %v, %d targets, entry %s", pr.Committed, len(pr.Targets), pr.Entry.ID)
	}
	_, ix, _ = ts.Open()
	for _, id := range []string{"ME-1", "ME-2"} {
		_, err := ts.Resolve(ix, id)
		wantErr(t, err, KindNotFound, "task.not_found")
	}
}

func TestRestoreLiveTaskIsConflict(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Restore("ME-1")
	wantErr(t, err, KindConflict, "task.not_deleted")
	if err != nil && !strings.HasSuffix(err.Error(), " a is not deleted") {
		t.Errorf("message = %q", err)
	}
	_, err = ts.Restore("ME-9")
	wantErr(t, err, KindNotFound, "task.not_found")
}

func TestRestoreUnderDeletedParentIsConflict(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "p"}, {Title: "c", Parent: "ME-1"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Restore("ME-2")
	wantErr(t, err, KindConflict, "task.parent_deleted")
}

func TestRestoreLeavesWhatAnotherRemoveDeleted(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "p"}, {Title: "c", Parent: "ME-1"}, {Title: "g", Parent: "ME-2"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ts.Remove("ME-2"); err != nil {
		t.Fatal(err)
	}
	// The clock is fixed, so the second rm must differ in time to be told apart.
	ts.Now = func() time.Time { return time.Date(2026, 10, 30, 13, 0, 0, 0, time.UTC) }
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	r, err := ts.Restore("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Targets) != 1 || r.Targets[0].ID != "ME-1" {
		t.Errorf("Restore targets = %d; want only ME-1 (ME-2 and ME-3 went in an earlier rm)", len(r.Targets))
	}
}

func TestRemoveTruncatesToSecond(t *testing.T) {
	ts := newTestSession(t)
	ts.Now = func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 999, time.UTC) }
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	r, err := ts.Remove("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Index.Refresh(); err != nil {
		t.Fatal(err)
	}
	e, _ := r.Index.Task(r.Entry.ULID)
	if want := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC); e.DeletedAt == nil || !e.DeletedAt.Equal(want) {
		t.Errorf("deleted at = %v; want %v", e.DeletedAt, want)
	}
}

func TestPurgeChildNotDeleted(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "p"}, {Title: "c", Parent: "ME-1"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	ts.deleteTask("ME-1") // the parent only
	_, _, _, err := ts.PurgeTargets("ME-1")
	wantErr(t, err, KindConflict, "purge.child_not_deleted")
	if err != nil && (!strings.Contains(err.Error(), "ME-2") || !strings.HasSuffix(err.Error(), " under it is not deleted")) {
		t.Errorf("message = %q", err)
	}
	_, err = ts.Purge("ME-1", nil)
	wantErr(t, err, KindConflict, "purge.child_not_deleted")
	_, _, _, err = ts.PurgeTargets("ME-9")
	wantErr(t, err, KindNotFound, "task.not_found")
}

func TestPurgeWithStaleTargetsIsConflict(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, _, targets, err := ts.PurgeTargets("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %d; want 1", len(targets))
	}
	// A task deleted under it after the listing changes the targets.
	if _, err := ts.Restore("ME-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Add(AddInput{Title: "c", Parent: "ME-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err = ts.Purge("ME-1", targets)
	wantErr(t, err, KindConflict, "purge.changed")
	if err != nil && err.Error() != "the tasks under it changed after the confirmation; run purge again" {
		t.Errorf("message = %q", err)
	}
	// Nothing was removed.
	_, ix, _ := ts.Open()
	if _, err := ts.Resolve(ix, "ME-2"); err != nil {
		t.Errorf("ME-2 after the refused purge: %v", err)
	}
	// With the current targets it goes through.
	_, _, targets, err = ts.PurgeTargets("ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Purge("ME-1", targets); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentOfPurgedTaskIsGone(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, ix, _ := ts.Open()
	e, err := ts.Resolve(ix, "ME-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Purge("ME-1", nil); err != nil {
		t.Fatal(err)
	}
	_, err = current(ix, e)
	wantErr(t, err, KindNotFound, "task.gone")
	if err != nil && (!strings.HasPrefix(err.Error(), "ME-1 (") || !strings.HasSuffix(err.Error(), ") is no longer in the store")) {
		t.Errorf("message = %q", err)
	}
	var oe *Error
	if asError(err, &oe); oe == nil || oe.Args["id"] != "ME-1" {
		t.Errorf("args = %v; want id ME-1", oe)
	}
}

func TestDescendants(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{
		{Title: "p"}, {Title: "a", Parent: "ME-1"}, {Title: "b", Parent: "ME-1"}, {Title: "a1", Parent: "ME-2"}, {Title: "other"},
	} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	_, ix, _ := ts.Open()
	p, _ := ts.Resolve(ix, "ME-1")
	ids := func(es []*index.Entry) string {
		var out []string
		for _, e := range es {
			out = append(out, e.ID)
		}
		return strings.Join(out, " ")
	}
	if got, want := ids(Descendants(ix, p.ULID, every)), "ME-2 ME-4 ME-3"; got != want {
		t.Errorf("all = %s; want %s (depth first)", got, want)
	}
	if got, want := ids(Descendants(ix, p.ULID, func(e *index.Entry) bool { return e.Title != "a" })), "ME-3"; got != want {
		t.Errorf("without a = %s; want %s (a and what is under it are skipped)", got, want)
	}
	if got := Descendants(ix, p.ULID, func(*index.Entry) bool { return false }); len(got) != 0 {
		t.Errorf("follow false = %v; want none", got)
	}
	leaf, _ := ts.Resolve(ix, "ME-5")
	if got := Descendants(ix, leaf.ULID, every); len(got) != 0 {
		t.Errorf("leaf = %v; want none", got)
	}
}

func TestRestoreAtByTimeAndCommit(t *testing.T) {
	ts := newTestSession(t)
	// Commit times have a second's precision and come from the real clock.
	time.Sleep(1100 * time.Millisecond)
	if _, err := ts.Add(AddInput{Title: "v1"}); err != nil {
		t.Fatal(err)
	}
	s, _, _ := ts.Open()
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	at := time.Now().UTC().Format(time.RFC3339)
	time.Sleep(1100 * time.Millisecond)
	for _, point := range []string{at, head.String()[:7]} {
		if _, err := ts.Set("ME-1", []Assign{{Key: "title", Op: "=", Value: "later"}}); err != nil {
			t.Fatal(err)
		}
		if _, err := ts.Remove("ME-1"); err != nil {
			t.Fatal(err)
		}
		res, err := ts.RestoreAt("ME-1", point)
		if err != nil {
			t.Fatalf("RestoreAt(%q): %v", point, err)
		}
		if !res.Committed || res.Note != "" || res.Index == nil {
			t.Errorf("RestoreAt(%q) = committed %v note %q", point, res.Committed, res.Note)
		}
		_, ix, _ := ts.Open()
		e, _ := ts.Resolve(ix, "ME-1")
		if e.Title != "v1" || e.DeletedAt != nil || e.ID != "ME-1" {
			t.Errorf("after RestoreAt(%q): title %q deleted %v id %s; want v1, live, ME-1", point, e.Title, e.DeletedAt, e.ID)
		}
		if res.ULID != e.ULID {
			t.Errorf("result ULID = %v; want %v", res.ULID, e.ULID)
		}
	}
}

func TestRestoreAtErrors(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err := ts.RestoreAt("ME-1", "2000-01-01T00:00:00Z")
	wantErr(t, err, KindNotFound, "restore_at.not_found")
	if err != nil && !strings.Contains(err.Error(), "did not exist at 2000-01-01T00:00:00Z") {
		t.Errorf("message = %q", err)
	}
	var oe *Error
	if asError(err, &oe); oe == nil || oe.Args["at"] != "2000-01-01T00:00:00Z" {
		t.Errorf("args = %v; want at", oe)
	}
	_, err = ts.RestoreAt("ME-1", "yesterday-ish")
	wantErr(t, err, KindInvalid, "restore_at.bad_time")
	if err != nil && err.Error() != `"yesterday-ish" is neither a time nor a commit` {
		t.Errorf("message = %q", err)
	}
	_, err = ts.RestoreAt("ME-9", "2000-01-01")
	wantErr(t, err, KindNotFound, "task.not_found")
}

func TestRestoreAtRefusesDeletedParent(t *testing.T) {
	ts := newTestSession(t)
	for _, in := range []AddInput{{Title: "p"}, {Title: "c", Parent: "ME-1"}} {
		if _, err := ts.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	s, _, _ := ts.Open()
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Move(MoveInput{Arg: "ME-2", Root: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Remove("ME-1"); err != nil {
		t.Fatal(err)
	}
	_, err = ts.RestoreAt("ME-2", head.String())
	wantErr(t, err, KindConflict, "task.parent_deleted")
}

func TestRestoreAtKeepsWorkspaceAndSaysSo(t *testing.T) {
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	if _, err := ts.Add(AddInput{Title: "x", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	s, _, _ := ts.Open()
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Move(MoveInput{Arg: "ME-1", Workspace: "api"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Set("API-1", []Assign{{Key: "title", Op: "=", Value: "later"}}); err != nil {
		t.Fatal(err)
	}
	res, err := ts.RestoreAt("API-1", head.String())
	if err != nil {
		t.Fatal(err)
	}
	if want := "the workspace stays api; move it with hedgehug mv API-1 --workspace me"; res.Note != want {
		t.Errorf("note = %q; want %q", res.Note, want)
	}
	_, ix, _ := ts.Open()
	e, _ := ts.Resolve(ix, "API-1")
	if e.Title != "x" || e.ID != "API-1" {
		t.Errorf("after RestoreAt: title %q id %s; want x, API-1", e.Title, e.ID)
	}
}

func TestParseAtTime(t *testing.T) {
	loc := time.FixedZone("x", 9*3600)
	for _, c := range []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"2026-10-30T10:00:00+02:00", time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC), true},
		{"2026-10-30", time.Date(2026, 10, 30, 23, 59, 59, 0, loc), true},
		{"abc1234", time.Time{}, false},
		{"2026-13-45", time.Time{}, false},
	} {
		got, ok := ParseAtTime(c.in, loc)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("ParseAtTime(%q) = %v %v, want %v %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestSameTasks(t *testing.T) {
	a := &index.Entry{ULID: model.NewULID()}
	b := &index.Entry{ULID: model.NewULID()}
	for _, c := range []struct {
		x, y []*index.Entry
		want bool
	}{
		{[]*index.Entry{a, b}, []*index.Entry{b, a}, true},
		{[]*index.Entry{a}, []*index.Entry{a, b}, false},
		{[]*index.Entry{a}, []*index.Entry{b}, false},
		{nil, nil, true},
	} {
		if got := sameTasks(c.x, c.y); got != c.want {
			t.Errorf("sameTasks = %v; want %v", got, c.want)
		}
	}
}
