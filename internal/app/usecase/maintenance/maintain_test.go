package maintenance_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// newCheckedSession is usecasetest.New with the merge driver registered, as
// store init does, and a hedgehug on PATH, so that doctor has no finding
// about the driver.
func newCheckedSession(t *testing.T) *usecasetest.Session {
	t.Helper()
	bin := t.TempDir()
	name := "hedgehug"
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // a stand-in for the binary
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ts := usecasetest.New(t)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestDoctorAndSchemaCheckOnACleanStore(t *testing.T) {
	ts := newCheckedSession(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	fs, err := schemas.SchemaCheck(ts.Session)
	if err != nil || len(fs) != 0 {
		t.Fatalf("SchemaCheck = %v, %v; want no findings", fs, err)
	}
	rep, err := ts.DoctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 0 || len(rep.Repaired) != 0 {
		t.Errorf("Doctor = %+v; want nothing found and nothing repaired", rep)
	}
	rep, err = ts.DoctorOne(true)
	if err != nil || len(rep.Findings) != 0 || len(rep.Repaired) != 0 {
		t.Errorf("Doctor(fix) = %+v, %v; want nothing to repair", rep, err)
	}
}

func TestSchemaCheckAndDoctorOnABrokenSchemaFile(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, e, _ := ts.Entry()
	if err := os.WriteFile(filepath.Join(e.Path, "schema.yaml"), []byte("statuses: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, err := schemas.SchemaCheck(ts.Session)
	if err != nil || len(fs) == 0 || fs[0].Check != "schema" || !strings.HasPrefix(fs[0].Where, "schema.yaml") {
		t.Fatalf("SchemaCheck = %+v, %v; want the schema.yaml finding", fs, err)
	}
	rep, err := ts.DoctorOne(false)
	if err != nil || len(rep.Findings) == 0 || rep.Findings[0].Check != "schema" {
		t.Fatalf("Doctor = %+v, %v; want the schema finding", rep, err)
	}
	rep, err = ts.DoctorOne(true)
	if err != nil || len(rep.Findings) == 0 || len(rep.Repaired) != 0 {
		t.Errorf("Doctor(fix) = %+v, %v; want the finding and no repair of a store that does not open", rep, err)
	}
}

func TestDoctorFindsABrokenTaskFile(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	ts.CommitRawFile(model.TaskPath(a.ULID), "not a task\n")
	rep, err := ts.DoctorOne(false)
	if err != nil || len(rep.Findings) == 0 {
		t.Fatalf("Doctor = %+v, %v; want findings", rep, err)
	}
	var found bool
	for _, f := range rep.Findings {
		found = found || strings.Contains(f.Where, model.TaskPath(a.ULID))
	}
	if !found {
		t.Errorf("no finding names %s: %+v", model.TaskPath(a.ULID), rep.Findings)
	}
}

func TestDoctorFixRepairsAnIDClash(t *testing.T) {
	ts := newCheckedSession(t)
	var written int
	ts.AfterWrite = func(string) { written++ }
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	b, err := tasks.Add(ts.Session, tasks.AddInput{Title: "b"})
	if err != nil {
		t.Fatal(err)
	}
	written = 0
	ts.EditRawFile(model.TaskPath(b.ULID), "id: ME-2", "id: ME-1")
	rep, err := ts.DoctorOne(false)
	if err != nil || len(rep.Findings) == 0 || len(rep.Repaired) != 0 {
		t.Fatalf("Doctor = %+v, %v; want the clash found and nothing repaired", rep, err)
	}
	var hinted bool
	for _, f := range rep.Findings {
		hinted = hinted || strings.Contains(f.Hint, "hedgehug doctor --fix fixes it")
	}
	if !hinted {
		t.Errorf("no finding says --fix fixes it: %+v", rep.Findings)
	}
	before := ts.Head()
	rep, err = ts.DoctorOne(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 0 || len(rep.Repaired) != 1 {
		t.Fatalf("Doctor(fix) = %+v; want one repair and nothing left", rep)
	}
	r := rep.Repaired[0]
	if r.Kind != "id_collision" || r.ID != "ME-2" || r.Title != "b" || r.ULID != b.ULID.String() || !strings.Contains(string(r.Value), `"old":"ME-1"`) {
		t.Errorf("repaired = %+v", r)
	}
	if !regexp.MustCompile(`^repaired: ME-2 \(` + b.ULID.String() + `\) b: id_collision ME-1 → ME-2$`).MatchString(r.Line) {
		t.Errorf("line = %q", r.Line)
	}
	if ts.Head() == before || written != 1 {
		t.Errorf("the repair committed %v and called AfterWrite %d times; want one commit and one call", ts.Head() != before, written)
	}
	// Marked for sync, like any write.
	s, _ := ts.OpenStore()
	if st, err := replicate.ReadState(s); err != nil || st.PendingSince.IsZero() {
		t.Errorf("sync state = %+v, %v; want the repair pending", st, err)
	}
	if rep, err := ts.DoctorOne(true); err != nil || len(rep.Repaired) != 0 || len(rep.Findings) != 0 {
		t.Errorf("second fix = %+v, %v; want nothing", rep, err)
	}
}

func TestDoctorNoStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := stores.StoreRemove(ts.Session, "me", true); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenance.Doctor(ts.Session, false); usecase.KindOf(err) != usecase.KindConflict {
		t.Errorf("Doctor without stores: kind %v (%v); want KindConflict", usecase.KindOf(err), err)
	}
	if _, err := schemas.SchemaCheck(ts.Session); usecase.KindOf(err) != usecase.KindConflict {
		t.Errorf("SchemaCheck without stores: kind %v (%v); want KindConflict", usecase.KindOf(err), err)
	}
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "Beta", Status: "done"}); err != nil {
		t.Fatal(err)
	}
	before := ts.Head()
	w, err := schemas.Migrate(ts.Session, schema.Migration{Target: schema.TargetStatus, Old: "todo", New: "ready"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Head() != before {
		t.Error("a dry run committed")
	}
	if len(w.Changed) != 1 || w.Changed[0].Title != "Alpha" || w.Changed[0].Status != "ready" || len(w.Skipped) != 0 || w.Skipped == nil || w.Commit != nil {
		t.Errorf("plan = %+v", w)
	}
	_, ix, _ := ts.Open()
	if e, _ := ts.Resolve(ix, "ME-1"); e.Status != "todo" {
		t.Errorf("the index shows status %q after a dry run; want todo", e.Status)
	}
}

func TestMigrateWritesTasksAndSchemaInOneCommit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a, _ := tasks.Add(ts.Session, tasks.AddInput{Title: "Alpha"})
	b, _ := tasks.Add(ts.Session, tasks.AddInput{Title: "Beta"})
	ts.DeleteTask("ME-2")
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "Gamma", Status: "done"}); err != nil {
		t.Fatal(err)
	}
	// A task file that does not decode is left alone and named.
	d, _ := tasks.Add(ts.Session, tasks.AddInput{Title: "Broken"})
	ts.CommitRawFile(model.TaskPath(d.ULID), "not a task\n")
	before := ts.Head()
	w, err := schemas.Migrate(ts.Session, schema.Migration{Target: schema.TargetStatus, Old: "todo", New: "ready"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Head() == before {
		t.Fatal("nothing was committed")
	}
	if len(w.Changed) != 2 || w.Changed[0].ULID > w.Changed[1].ULID || w.Commit == nil || *w.Commit != ts.Head() {
		t.Fatalf("Changed = %v, commit %v; want Alpha and Beta by ULID, and HEAD", w.Changed, w.Commit)
	}
	if len(w.Skipped) != 1 || w.Skipped[0] != d.ULID.String() {
		t.Errorf("Skipped = %v; want %s", w.Skipped, d.ULID)
	}
	for _, v := range w.Changed {
		if v.Status != "ready" || (v.ULID != a.ULID.String() && v.ULID != b.ULID.String()) {
			t.Errorf("task %s after the migration: status %q; want ready, read from the refreshed index", v.ULID, v.Status)
		}
	}
	s, _ := ts.OpenStore()
	if _, ok := s.Schema().Status("ready"); !ok {
		t.Error("the schema has no status ready")
	}
	if _, ok := s.Schema().Status("todo"); ok {
		t.Error("the schema still has todo")
	}
	if g, _ := s.Task(a.ULID); g.Status != "ready" {
		t.Errorf("Alpha = %q", g.Status)
	}
}

func TestMigrateRefusals(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a, _ := tasks.Add(ts.Session, tasks.AddInput{Title: "Alpha"})
	b, _ := tasks.Add(ts.Session, tasks.AddInput{Title: "Beta"})
	for _, id := range []string{"ME-1", "ME-2"} {
		if _, err := tasks.Set(ts.Session, id, []usecase.Assign{{Key: "start", Op: "=", Value: "2026-11-01"}, {Key: "due", Op: "=", Value: "2026-11-30"}}); err != nil {
			t.Fatal(err)
		}
	}
	before := ts.Head()
	_, err := schemas.Migrate(ts.Session, schema.Migration{Target: schema.TargetProp, Old: "start", New: "due"}, false)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "migrate.invalid")
	for _, want := range []string{"cannot migrate 2 tasks:\n  ", "ME-1 (" + a.ULID.String() + ")", "ME-2 (" + b.ULID.String() + ")", "both start and due"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message lacks %q:\n%s", want, err)
		}
	}
	if _, err := schemas.Migrate(ts.Session, schema.Migration{Target: schema.TargetProp, Old: "start", New: "due"}, true); usecase.KindOf(err) != usecase.KindInvalid {
		t.Errorf("the dry run is refused with kind %v (%v); want KindInvalid", usecase.KindOf(err), err)
	}
	if ts.Head() != before {
		t.Error("a refused migration committed")
	}
}

func TestMigrateNamesTheTaskOfAViolation(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p, _ := tasks.Add(ts.Session, tasks.AddInput{Title: "Parent"})
	c, _ := tasks.Add(ts.Session, tasks.AddInput{Title: "Child", Parent: "ME-1"})
	before := ts.Head()
	// A bug may not sit under a bug.
	_, err := schemas.Migrate(ts.Session, schema.Migration{Target: schema.TargetKind, Old: "task", New: "bug"}, false)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "migrate.invalid")
	msg := err.Error()
	if !strings.Contains(msg, "breaks the schema") || !strings.Contains(msg, "cannot be placed") || !strings.HasPrefix(msg, "ME-2 ("+c.ULID.String()+") Child: ") {
		t.Errorf("message = %q", msg)
	}
	_ = p
	if ts.Head() != before {
		t.Error("a refused migration committed")
	}
}

func TestMigrateUnknownDefinition(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := schemas.Migrate(ts.Session, schema.Migration{Target: schema.TargetStatus, Old: "nope", New: "x"}, true); err == nil {
		t.Error("a status the schema lacks was migrated")
	}
}

func TestTaskCount(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]string{0: "0 tasks", 1: "1 task", 2: "2 tasks"} {
		if got := schemas.TaskCount(n); got != want {
			t.Errorf("TaskCount(%d) = %q; want %q", n, got, want)
		}
	}
}

func TestViolationsText(t *testing.T) {
	t.Parallel()
	ve := &store.ValidationError{Violations: []schema.Violation{{Field: "kind", Msg: "bug is not a kind"}, {Field: "title", Msg: "is empty"}}}
	want := "the change breaks the schema:\n  kind: bug is not a kind\n  title: is empty"
	if got := usecase.ViolationsText(ve.Violations); got != want {
		t.Errorf("ViolationsText = %q; want %q", got, want)
	}
}

// force-push.json is told of as unexpected_force_push (D23).
func TestDoctorFindsAnUnexpectedForcePush(t *testing.T) {
	// 直列: t.Setenv
	ts := newCheckedSession(t)
	_, e, _ := ts.Entry()
	file := filepath.Join(e.Path, ".hedgehug", "local", "force-push.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(`{"origin":"a","tracked":"b","checked":"2026-11-02T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := ts.DoctorOne(false)
	if err != nil || len(rep.Findings) != 1 {
		t.Fatalf("Doctor = %+v, %v; want one finding", rep, err)
	}
	f := rep.Findings[0]
	if f.Check != "unexpected_force_push" || f.Where != e.Name || !strings.Contains(f.Msg, "differs from this store's") || !strings.Contains(f.Hint, "doctor --fix") {
		t.Errorf("finding = %+v", f)
	}
}
