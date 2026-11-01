package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

// refTo makes the task API-1 of org-a point at the task WEB-1 of org-b
// through line (a line of its front matter; <target> is the ULID of WEB-1),
// as a hand edit or a sync would, and returns the ULID of API-1.
func refTo(t *testing.T, ts *testSession, line string) (from model.ULID) {
	t.Helper()
	ulidIn := func(store, id string) model.ULID {
		r, err := ts.ForStore(store).Show(id)
		if err != nil {
			t.Fatal(err)
		}
		return r.Entry.ULID
	}
	from, target := ulidIn("org-a", "API-1"), ulidIn("org-b", "WEB-1")
	s, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.TaskPath(from)
	p := filepath.Join(s.Dir(), filepath.FromSlash(rel))
	b, err := os.ReadFile(p) //nolint:gosec // a file of the test's own store
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(b), "\nkind: ", "\n"+strings.ReplaceAll(line, "<target>", target.String())+"\nkind: ", 1)
	if body == string(b) {
		t.Fatalf("%s has no kind line", rel)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return from
}

func TestDoctorCoversEveryStoreAndFindsCrossStoreRefs(t *testing.T) {
	ts := newTwoStoreSession(t)
	from := refTo(t, ts, "parent: <target>")
	rs, err := ts.Doctor(false)
	if err != nil || len(rs) != 2 || rs[0].Store != "org-a" || rs[1].Store != "org-b" {
		t.Fatalf("Doctor = %+v, %v; want a report each for org-a and org-b", rs, err)
	}
	var cross []doctor.Finding
	for _, f := range rs[0].Findings {
		if f.Check == "cross_store_ref" {
			cross = append(cross, f)
		}
	}
	want := doctor.Finding{
		Check: "cross_store_ref", Where: "API-1 (" + from.String() + ")",
		Msg: "parent points at org-b's WEB-1; references stay inside a store", Severity: doctor.Warning,
	}
	if len(cross) != 1 || cross[0] != want {
		t.Errorf("org-a cross_store_ref findings = %+v; want [%+v]", cross, want)
	}
	for _, f := range rs[1].Findings {
		if f.Check == "cross_store_ref" {
			t.Errorf("org-b has %+v; it refers to nothing", f)
		}
	}
	// --store names one store
	one, err := ts.ForStore("org-b").Doctor(false)
	if err != nil || len(one) != 1 || one[0].Store != "org-b" {
		t.Errorf("Doctor of org-b = %+v, %v; want org-b only", one, err)
	}
	one, err = ts.ForStore("org-a").Doctor(false)
	if err != nil || len(one) != 1 || one[0].Store != "org-a" || !slices.ContainsFunc(one[0].Findings, func(f doctor.Finding) bool { return f.Check == "cross_store_ref" }) {
		t.Errorf("Doctor of org-a = %+v, %v; want org-a with the cross-store reference", one, err)
	}
	if _, err := ts.ForStore("nope").Doctor(false); KindOf(err) != KindNotFound {
		t.Errorf("Doctor of nope: kind %v (%v); want KindNotFound", KindOf(err), err)
	}
}

func TestDoctorFixSkipsAReadOnlyStoreAndWarns(t *testing.T) {
	ts := newTwoStoreSession(t)
	// an id clash in each store, which --fix repairs
	for _, name := range []string{"org-a", "org-b"} {
		s := ts.ForStore(name)
		if _, err := s.Add(AddInput{Title: "second"}); err != nil {
			t.Fatal(err)
		}
		r, err := s.Show(map[string]string{"org-a": "API-2", "org-b": "WEB-2"}[name])
		if err != nil {
			t.Fatal(err)
		}
		first := map[string]string{"org-a": "API-1", "org-b": "WEB-1"}[name]
		second := map[string]string{"org-a": "API-2", "org-b": "WEB-2"}[name]
		editRaw(t, ts, name, r.Entry.ULID, "id: "+second, "id: "+first)
	}
	makeStoreReadOnly(t, ts, "org-b")
	rs, err := ts.Doctor(true)
	if err != nil || len(rs) != 2 {
		t.Fatalf("Doctor(fix) = %+v, %v", rs, err)
	}
	if len(rs[0].Repaired) != 1 {
		t.Errorf("org-a repaired %+v; want the clash repaired", rs[0].Repaired)
	}
	if len(rs[1].Repaired) != 0 || !slices.ContainsFunc(rs[1].Findings, func(f doctor.Finding) bool { return f.Check == "task" }) {
		t.Errorf("org-b = %+v; want nothing repaired and the clash still found", rs[1])
	}
	if want := "org-b is read-only, so it is not fixed (test)"; !slices.Contains(ts.warns, want) {
		t.Errorf("warnings = %q; want %q", ts.warns, want)
	}
}

// editRaw replaces old with repl in the file of the task ulid in the store
// name and commits it.
func editRaw(t *testing.T, ts *testSession, name string, ulid model.ULID, old, repl string) {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	rel := model.TaskPath(ulid)
	p := filepath.Join(s.Dir(), filepath.FromSlash(rel))
	b, err := os.ReadFile(p) //nolint:gosec // a file of the test's own store
	if err != nil || !strings.Contains(string(b), old) {
		t.Fatalf("%s has no %q (%v)", rel, old, err)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(b), old, repl, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFixOfAReadOnlyStoreWarnsInsteadOfFailing(t *testing.T) {
	ts := newTestSession(t)
	ts.addTask("one", nil)
	makeReadOnly(t, ts, "test")
	rep, err := ts.doctorOne(true)
	if err != nil || len(rep.Repaired) != 0 {
		t.Fatalf("Doctor(fix) = %+v, %v; want no error and no repair", rep, err)
	}
	if want := "me is read-only, so it is not fixed (test)"; !slices.Contains(ts.warns, want) {
		t.Errorf("warnings = %q; want %q", ts.warns, want)
	}
}

func TestDoctorKeepsTheOtherStoresWhenOneDoesNotOpen(t *testing.T) {
	ts := newTwoStoreSession(t)
	refTo(t, ts, "parent: <target>")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("gone", filepath.Join(t.TempDir(), "gone"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.warns = nil
	rs, err := ts.Doctor(false)
	if err != nil || len(rs) != 3 || rs[2].Store != "gone" || len(rs[2].Findings) == 0 {
		t.Fatalf("Doctor = %+v, %v; want three reports, the third with findings about gone", rs, err)
	}
	if !slices.ContainsFunc(rs[0].Findings, func(f doctor.Finding) bool { return f.Check == "cross_store_ref" }) {
		t.Errorf("org-a = %+v; want its cross-store reference still found", rs[0])
	}
	for _, w := range ts.warns {
		if strings.Contains(w, "cannot open gone") {
			t.Errorf("warning %q repeats what the report of gone says", w)
		}
	}
	// named alone, it is still the error of today or its findings
	if one, err := ts.ForStore("gone").Doctor(false); err == nil && (len(one) != 1 || len(one[0].Findings) == 0) {
		t.Errorf("Doctor of gone = %+v; want the findings or the error", one)
	}
}

func TestCrossStoreCheckFailureIsAWarning(t *testing.T) {
	ts := newTwoStoreSession(t)
	// a session that has not mounted yet, and a config.yaml it cannot read
	ts.env["HEDGEHUG_HOME"] = t.TempDir()
	if err := os.WriteFile(filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml"), []byte("stores: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts.warns = nil
	if fs := ts.crossStoreFindings("org-a"); len(fs) != 0 {
		t.Errorf("findings = %+v; want none", fs)
	}
	ws := ts.crossStoreWarnings("org-a")
	if len(ts.warns) != 1 || !strings.HasPrefix(ts.warns[0], "could not check cross-store references: ") {
		t.Errorf("doctor warned %q; want the could-not-check warning", ts.warns)
	}
	if len(ws) != 1 || !strings.HasPrefix(ws[0], "could not check cross-store references: ") {
		t.Errorf("sync warnings = %q; want the could-not-check warning", ws)
	}
}

func TestSyncWarnsOfCrossStoreRefs(t *testing.T) {
	ts := newTwoStoreSession(t)
	refTo(t, ts, "blocked_by: [<target>]")
	remote := filepath.Join(t.TempDir(), "r.git")
	if err := gitrepo.InitBare(remote); err != nil {
		t.Fatal(err)
	}
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Select("org-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("org-a"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("org-a", e.Path, []string{remote}); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	e, err = c.Select("org-a")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ts.SyncStore(c, e, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "org-a: API-1 blocked_by points at org-b's WEB-1; references stay inside a store"
	if !slices.Contains(r.Warnings, want) {
		t.Errorf("Warnings = %q; want %q", r.Warnings, want)
	}
}
