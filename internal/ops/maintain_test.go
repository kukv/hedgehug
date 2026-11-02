package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// commitRawFile writes body to the file rel of the test store and commits it
// past the store's checks, as a hand edit or a sync could.
func (ts *testSession) commitRawFile(rel, body string) {
	ts.t.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		ts.t.Fatal(err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	p := filepath.Join(e.Path, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.Getenv)
	if err != nil {
		ts.t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		ts.t.Fatal(err)
	}
}

// editRawFile replaces old by repl in the file rel of the test store and
// commits it.
func (ts *testSession) editRawFile(rel, old, repl string) {
	ts.t.Helper()
	_, e, _ := ts.Entry()
	b, err := os.ReadFile(filepath.Join(e.Path, filepath.FromSlash(rel))) //nolint:gosec // a file of the test's own store
	if err != nil {
		ts.t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		ts.t.Fatalf("%s has no %q", rel, old)
	}
	ts.commitRawFile(rel, strings.Replace(string(b), old, repl, 1))
}

// newCheckedSession is newTestSession with the merge driver registered, as
// store init does, and a hedgehug on PATH, so that doctor has no finding
// about the driver.
func newCheckedSession(t *testing.T) *testSession {
	t.Helper()
	bin := t.TempDir()
	name := "hedgehug"
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // a stand-in for the binary
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ts := newTestSession(t)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		t.Fatal(err)
	}
	return ts
}

func (ts *testSession) head() string {
	ts.t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	h, err := s.Repo().Head()
	if err != nil {
		ts.t.Fatal(err)
	}
	return h.String()
}

// doctorOne is Doctor of a session with one store: its only report.
func (ts *testSession) doctorOne(fix bool) (DoctorReport, error) {
	rs, err := ts.Doctor(fix)
	if err != nil || len(rs) != 1 {
		return DoctorReport{}, fmt.Errorf("%d reports, %w", len(rs), err)
	}
	return rs[0], nil
}

func TestDoctorAndSchemaCheckOnACleanStore(t *testing.T) {
	ts := newCheckedSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	fs, err := ts.SchemaCheck()
	if err != nil || len(fs) != 0 {
		t.Fatalf("SchemaCheck = %v, %v; want no findings", fs, err)
	}
	rep, err := ts.doctorOne(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 0 || len(rep.Repaired) != 0 {
		t.Errorf("Doctor = %+v; want nothing found and nothing repaired", rep)
	}
	rep, err = ts.doctorOne(true)
	if err != nil || len(rep.Findings) != 0 || len(rep.Repaired) != 0 {
		t.Errorf("Doctor(fix) = %+v, %v; want nothing to repair", rep, err)
	}
}

func TestSchemaCheckAndDoctorOnABrokenSchemaFile(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	_, e, _ := ts.Entry()
	if err := os.WriteFile(filepath.Join(e.Path, "schema.yaml"), []byte("statuses: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, err := ts.SchemaCheck()
	if err != nil || len(fs) == 0 || fs[0].Check != "schema" || !strings.HasPrefix(fs[0].Where, "schema.yaml") {
		t.Fatalf("SchemaCheck = %+v, %v; want the schema.yaml finding", fs, err)
	}
	rep, err := ts.doctorOne(false)
	if err != nil || len(rep.Findings) == 0 || rep.Findings[0].Check != "schema" {
		t.Fatalf("Doctor = %+v, %v; want the schema finding", rep, err)
	}
	rep, err = ts.doctorOne(true)
	if err != nil || len(rep.Findings) == 0 || len(rep.Repaired) != 0 {
		t.Errorf("Doctor(fix) = %+v, %v; want the finding and no repair of a store that does not open", rep, err)
	}
}

func TestDoctorFindsABrokenTaskFile(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	a, err := ts.Add(AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	ts.commitRawFile(model.TaskPath(a.ULID), "not a task\n")
	rep, err := ts.doctorOne(false)
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
	ts.AfterWrite = func(*store.Store, config.StoreEntry) { written++ }
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	b, err := ts.Add(AddInput{Title: "b"})
	if err != nil {
		t.Fatal(err)
	}
	written = 0
	ts.editRawFile(model.TaskPath(b.ULID), "id: ME-2", "id: ME-1")
	rep, err := ts.doctorOne(false)
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
	before := ts.head()
	rep, err = ts.doctorOne(true)
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
	if ts.head() == before || written != 1 {
		t.Errorf("the repair committed %v and called AfterWrite %d times; want one commit and one call", ts.head() != before, written)
	}
	// Marked for sync, like any write.
	s, _ := ts.OpenStore()
	if st, err := sync.ReadState(s); err != nil || st.PendingSince.IsZero() {
		t.Errorf("sync state = %+v, %v; want the repair pending", st, err)
	}
	if rep, err := ts.doctorOne(true); err != nil || len(rep.Repaired) != 0 || len(rep.Findings) != 0 {
		t.Errorf("second fix = %+v, %v; want nothing", rep, err)
	}
}

func TestDoctorNoStores(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.StoreRemove("me", false); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Doctor(false); KindOf(err) != KindConflict {
		t.Errorf("Doctor without stores: kind %v (%v); want KindConflict", KindOf(err), err)
	}
	if _, err := ts.SchemaCheck(); KindOf(err) != KindConflict {
		t.Errorf("SchemaCheck without stores: kind %v (%v); want KindConflict", KindOf(err), err)
	}
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Add(AddInput{Title: "Beta", Status: "done"}); err != nil {
		t.Fatal(err)
	}
	before := ts.head()
	ix, p, err := ts.Migrate(schema.Migration{Target: schema.TargetStatus, Old: "todo", New: "ready"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if ts.head() != before {
		t.Error("a dry run committed")
	}
	if ix == nil || len(p.Changed) != 1 || p.Changed[0].Title != "Alpha" || p.Changed[0].Status != "ready" || len(p.Skipped) != 0 || p.Schema == nil {
		t.Errorf("plan = %+v", p)
	}
	if e, _ := ts.Resolve(ix, "ME-1"); e.Status != "todo" {
		t.Errorf("the index shows status %q after a dry run; want todo", e.Status)
	}
}

func TestMigrateWritesTasksAndSchemaInOneCommit(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	a, _ := ts.Add(AddInput{Title: "Alpha"})
	b, _ := ts.Add(AddInput{Title: "Beta"})
	ts.deleteTask("ME-2")
	if _, err := ts.Add(AddInput{Title: "Gamma", Status: "done"}); err != nil {
		t.Fatal(err)
	}
	// A task file that does not decode is left alone and named.
	d, _ := ts.Add(AddInput{Title: "Broken"})
	ts.commitRawFile(model.TaskPath(d.ULID), "not a task\n")
	before := ts.head()
	ix, p, err := ts.Migrate(schema.Migration{Target: schema.TargetStatus, Old: "todo", New: "ready"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if ts.head() == before {
		t.Fatal("nothing was committed")
	}
	if len(p.Changed) != 2 || p.Changed[0].ULID.String() > p.Changed[1].ULID.String() {
		t.Fatalf("Changed = %v; want Alpha and Beta by ULID", p.Changed)
	}
	if len(p.Skipped) != 1 || p.Skipped[0] != d.ULID {
		t.Errorf("Skipped = %v; want %s", p.Skipped, d.ULID)
	}
	for _, u := range []model.ULID{a.ULID, b.ULID} {
		if e, ok := ix.Task(u); !ok || e.Status != "ready" {
			t.Errorf("task %s after the migration: %+v, %v; want ready, from the refreshed index", u, e, ok)
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
	ts := newTestSession(t)
	a, _ := ts.Add(AddInput{Title: "Alpha"})
	b, _ := ts.Add(AddInput{Title: "Beta"})
	for _, id := range []string{"ME-1", "ME-2"} {
		if _, err := ts.Set(id, []Assign{{Key: "start", Op: "=", Value: "2026-11-01"}, {Key: "due", Op: "=", Value: "2026-11-30"}}); err != nil {
			t.Fatal(err)
		}
	}
	before := ts.head()
	_, _, err := ts.Migrate(schema.Migration{Target: schema.TargetProp, Old: "start", New: "due"}, false)
	wantErr(t, err, KindInvalid, "migrate.invalid")
	for _, want := range []string{"cannot migrate 2 tasks:\n  ", "ME-1 (" + a.ULID.String() + ")", "ME-2 (" + b.ULID.String() + ")", "both start and due"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message lacks %q:\n%s", want, err)
		}
	}
	if _, _, err := ts.Migrate(schema.Migration{Target: schema.TargetProp, Old: "start", New: "due"}, true); KindOf(err) != KindInvalid {
		t.Errorf("the dry run is refused with kind %v (%v); want KindInvalid", KindOf(err), err)
	}
	if ts.head() != before {
		t.Error("a refused migration committed")
	}
}

func TestMigrateNamesTheTaskOfAViolation(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p, _ := ts.Add(AddInput{Title: "Parent"})
	c, _ := ts.Add(AddInput{Title: "Child", Parent: "ME-1"})
	before := ts.head()
	// A bug may not sit under a bug.
	_, _, err := ts.Migrate(schema.Migration{Target: schema.TargetKind, Old: "task", New: "bug"}, false)
	wantErr(t, err, KindInvalid, "migrate.invalid")
	msg := err.Error()
	if !strings.Contains(msg, "breaks the schema") || !strings.Contains(msg, "cannot be placed") || !strings.HasPrefix(msg, "ME-2 ("+c.ULID.String()+") Child: ") {
		t.Errorf("message = %q", msg)
	}
	_ = p
	if ts.head() != before {
		t.Error("a refused migration committed")
	}
}

func TestMigrateUnknownDefinition(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, _, err := ts.Migrate(schema.Migration{Target: schema.TargetStatus, Old: "nope", New: "x"}, true); err == nil {
		t.Error("a status the schema lacks was migrated")
	}
}

func TestTaskCount(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]string{0: "0 tasks", 1: "1 task", 2: "2 tasks"} {
		if got := TaskCount(n); got != want {
			t.Errorf("TaskCount(%d) = %q; want %q", n, got, want)
		}
	}
}

func TestViolationsText(t *testing.T) {
	t.Parallel()
	ve := &store.ValidationError{Violations: []schema.Violation{{Field: "kind", Msg: "bug is not a kind"}, {Field: "title", Msg: "is empty"}}}
	want := "the change breaks the schema:\n  kind: bug is not a kind\n  title: is empty"
	if got := ViolationsText(ve); got != want {
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
	rep, err := ts.doctorOne(false)
	if err != nil || len(rep.Findings) != 1 {
		t.Fatalf("Doctor = %+v, %v; want one finding", rep, err)
	}
	f := rep.Findings[0]
	if f.Check != "unexpected_force_push" || f.Where != e.Name || !strings.Contains(f.Msg, "without a record of Hedgehug") || f.Hint == "" {
		t.Errorf("finding = %+v", f)
	}
}
