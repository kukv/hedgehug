package maintenance_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// refTo makes the task API-1 of org-a point at the task WEB-1 of org-b
// through line (a line of its front matter; <target> is the ULID of WEB-1),
// as a hand edit or a sync would, and returns the ULID of API-1.
func refTo(t *testing.T, ts *usecasetest.Session, line string) (from model.ULID) {
	t.Helper()
	ulidIn := func(store, id string) model.ULID {
		r, err := browse.Show(ts.ForStore(store), id)
		if err != nil {
			t.Fatal(err)
		}
		u, err := model.ParseULID(r.ULID)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	bID := "WEB-1"
	if _, err := browse.Show(ts.ForStore("org-b"), bID); err != nil {
		bID = "API-1" // org-b shares the prefix of org-a
	}
	from, target := ulidIn("org-a", "API-1"), ulidIn("org-b", bID)
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
	author := config.CommitAuthor(usecasetest.TestAccount)
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return from
}

func TestDoctorCoversEveryStoreAndFindsCrossStoreRefs(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	from := refTo(t, ts, "parent: <target>")
	rs, err := maintenance.Doctor(ts.Session, false)
	if err != nil || len(rs) != 2 || rs[0].Store != "org-a" || rs[1].Store != "org-b" {
		t.Fatalf("Doctor = %+v, %v; want a report each for org-a and org-b", rs, err)
	}
	var cross []usecase.Finding
	for _, f := range rs[0].Findings {
		if f.Check == "cross_store_ref" {
			cross = append(cross, f)
		}
	}
	want := usecase.Finding{
		Check: "cross_store_ref", Where: "API-1 (" + from.String() + ") org-a task",
		Task: from.String(), Msg: "parent points at org-b's WEB-1; references stay inside a store", Severity: usecase.SeverityWarning,
		Code: "cross_store_ref", Args: []string{"parent", "org-b", "WEB-1"},
	}
	if len(cross) != 1 || !reflect.DeepEqual(cross[0], want) {
		t.Errorf("org-a cross_store_ref findings = %+v; want [%+v]", cross, want)
	}
	for _, f := range rs[1].Findings {
		if f.Check == "cross_store_ref" {
			t.Errorf("org-b has %+v; it refers to nothing", f)
		}
	}
	// --store names one store
	one, err := maintenance.Doctor(ts.ForStore("org-b"), false)
	if err != nil || len(one) != 1 || one[0].Store != "org-b" {
		t.Errorf("Doctor of org-b = %+v, %v; want org-b only", one, err)
	}
	one, err = maintenance.Doctor(ts.ForStore("org-a"), false)
	if err != nil || len(one) != 1 || one[0].Store != "org-a" || !slices.ContainsFunc(one[0].Findings, func(f usecase.Finding) bool { return f.Check == "cross_store_ref" }) {
		t.Errorf("Doctor of org-a = %+v, %v; want org-a with the cross-store reference", one, err)
	}
	if _, err := maintenance.Doctor(ts.ForStore("nope"), false); usecase.KindOf(err) != usecase.KindNotFound {
		t.Errorf("Doctor of nope: kind %v (%v); want KindNotFound", usecase.KindOf(err), err)
	}
}

// With a prefix both stores use, doctor's Where is still the plain id (the
// store has its heading), and the Msg names the other store and its id.
func TestDoctorCrossStoreRefWhereIsThePlainIDWhenThePrefixIsShared(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStoreWith(t, "API", "API")
	from := refTo(t, ts, "parent: <target>")
	rs, err := maintenance.Doctor(ts.Session, false)
	if err != nil || len(rs) != 2 {
		t.Fatalf("Doctor = %+v, %v", rs, err)
	}
	var cross []usecase.Finding
	for _, f := range rs[0].Findings {
		if f.Check == "cross_store_ref" {
			cross = append(cross, f)
		}
	}
	want := usecase.Finding{
		Check: "cross_store_ref", Where: "API-1 (" + from.String() + ") org-a task",
		Task: from.String(), Msg: "parent points at org-b's API-1; references stay inside a store", Severity: usecase.SeverityWarning,
		Code: "cross_store_ref", Args: []string{"parent", "org-b", "API-1"},
	}
	if len(cross) != 1 || !reflect.DeepEqual(cross[0], want) {
		t.Errorf("findings = %+v; want [%+v]", cross, want)
	}
}

func TestDoctorFixSkipsAReadOnlyStoreAndWarns(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	// an id clash in each store, which --fix repairs
	for _, name := range []string{"org-a", "org-b"} {
		s := ts.ForStore(name)
		if _, err := tasks.Add(s, tasks.AddInput{Title: "second"}); err != nil {
			t.Fatal(err)
		}
		r, err := browse.Show(s, map[string]string{"org-a": "API-2", "org-b": "WEB-2"}[name])
		if err != nil {
			t.Fatal(err)
		}
		first := map[string]string{"org-a": "API-1", "org-b": "WEB-1"}[name]
		second := map[string]string{"org-a": "API-2", "org-b": "WEB-2"}[name]
		u, err := model.ParseULID(r.ULID)
		if err != nil {
			t.Fatal(err)
		}
		editRaw(t, ts, name, u, "id: "+second, "id: "+first)
	}
	usecasetest.MakeStoreReadOnly(t, ts, "org-b")
	rs, err := maintenance.Doctor(ts.Session, true)
	if err != nil || len(rs) != 2 {
		t.Fatalf("Doctor(fix) = %+v, %v", rs, err)
	}
	if len(rs[0].Repaired) != 1 {
		t.Errorf("org-a repaired %+v; want the clash repaired", rs[0].Repaired)
	}
	if len(rs[1].Repaired) != 0 || !slices.ContainsFunc(rs[1].Findings, func(f usecase.Finding) bool { return f.Check == "task" }) {
		t.Errorf("org-b = %+v; want nothing repaired and the clash still found", rs[1])
	}
	if want := "org-b is read-only, so it is not fixed (test)"; !slices.Contains(ts.Warns, want) {
		t.Errorf("warnings = %q; want %q", ts.Warns, want)
	}
}

// editRaw replaces old with repl in the file of the task ulid in the store
// name and commits it.
func editRaw(t *testing.T, ts *usecasetest.Session, name string, ulid model.ULID, old, repl string) {
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
	author := config.CommitAuthor(usecasetest.TestAccount)
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorFixOfAReadOnlyStoreWarnsInsteadOfFailing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("one", nil)
	usecasetest.MakeReadOnly(t, ts, "test")
	rep, err := ts.DoctorOne(true)
	if err != nil || len(rep.Repaired) != 0 {
		t.Fatalf("Doctor(fix) = %+v, %v; want no error and no repair", rep, err)
	}
	if want := "me is read-only, so it is not fixed (test)"; !slices.Contains(ts.Warns, want) {
		t.Errorf("warnings = %q; want %q", ts.Warns, want)
	}
}

func TestDoctorKeepsTheOtherStoresWhenOneDoesNotOpen(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	refTo(t, ts, "parent: <target>")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("gone", filepath.Join(t.TempDir(), "gone"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStoreAccount("gone", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.Warns = nil
	rs, err := maintenance.Doctor(ts.Session, false)
	if err != nil || len(rs) != 3 || rs[2].Store != "gone" || len(rs[2].Findings) == 0 {
		t.Fatalf("Doctor = %+v, %v; want three reports, the third with findings about gone", rs, err)
	}
	if !slices.ContainsFunc(rs[0].Findings, func(f usecase.Finding) bool { return f.Check == "cross_store_ref" }) {
		t.Errorf("org-a = %+v; want its cross-store reference still found", rs[0])
	}
	for _, w := range ts.Warns {
		if strings.Contains(w, "cannot open gone") {
			t.Errorf("warning %q repeats what the report of gone says", w)
		}
	}
	// the silence was for Doctor only: a later operation warns, once
	ts.Warns = nil
	if _, err := browse.List(ts.Session, ""); err != nil {
		t.Fatal(err)
	}
	var n int
	for _, w := range ts.Warns {
		if strings.Contains(w, "cannot open gone") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("List after Doctor warned %d times of gone (%q); want once", n, ts.Warns)
	}
	// named alone, it stays what it was: no error, the findings maintenance.Run
	// gives for a directory that is no store
	one, err := maintenance.Doctor(ts.ForStore("gone"), false)
	if err != nil || len(one) != 1 || one[0].Store != "gone" || !slices.ContainsFunc(one[0].Findings, func(f usecase.Finding) bool { return f.Check == "git" }) {
		t.Errorf("Doctor of gone = %+v, %v; want its findings and no error", one, err)
	}
}

func countWarns(ts *usecasetest.Session, mu *gosync.Mutex, sub string) int {
	mu.Lock()
	defer mu.Unlock()
	var n int
	for _, w := range ts.Warns {
		if strings.Contains(w, sub) {
			n++
		}
	}
	return n
}

// The silence of overlapping callers ends with the last of them.
func TestMuteOpenWarningsIsCountedPerCaller(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("gone", filepath.Join(t.TempDir(), "gone"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStoreAccount("gone", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	var mu gosync.Mutex
	ts.Warn = func(m string) { mu.Lock(); ts.Warns = append(ts.Warns, m); mu.Unlock() }
	first, second := ts.MuteOpenWarnings([]string{"gone"}), ts.MuteOpenWarnings([]string{"gone"})
	first()
	if _, err := ts.Mount(); err != nil {
		t.Fatal(err)
	}
	if n := countWarns(ts, &mu, "cannot open gone"); n != 0 {
		t.Errorf("warned %d times while the second caller still mutes", n)
	}
	second()
	if _, err := ts.Mount(); err != nil {
		t.Fatal(err)
	}
	if n := countWarns(ts, &mu, "cannot open gone"); n != 1 {
		t.Errorf("warned %d times after both ended; want once", n)
	}
}

func TestConcurrentDoctorCallsDoNotWarnOfWhatTheyReport(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("gone", filepath.Join(t.TempDir(), "gone"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStoreAccount("gone", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	var mu gosync.Mutex
	ts.Warn = func(m string) { mu.Lock(); ts.Warns = append(ts.Warns, m); mu.Unlock() }
	var wg gosync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := maintenance.Doctor(ts.Session, false); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := countWarns(ts, &mu, "cannot open gone"); n != 0 {
		t.Errorf("warned %d times of gone, which the reports tell of", n)
	}
}

func TestCrossStoreCheckFailureIsAWarning(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	// a session that has not mounted yet, and a config.yaml it cannot read
	ts.Env["HEDGEHUG_HOME"] = t.TempDir()
	if err := os.WriteFile(filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml"), []byte("stores: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts.Warns = nil
	if fs := maintenance.CrossStoreFindings(ts.Session, "org-a"); len(fs) != 0 {
		t.Errorf("findings = %+v; want none", fs)
	}
	ws := ts.CrossStoreWarnings("org-a")
	if len(ts.Warns) != 1 || !strings.HasPrefix(ts.Warns[0], "could not check cross-store references: ") {
		t.Errorf("doctor warned %q; want the could-not-check warning", ts.Warns)
	}
	if len(ws) != 1 || !strings.HasPrefix(ws[0], "could not check cross-store references: ") {
		t.Errorf("sync warnings = %q; want the could-not-check warning", ws)
	}
}

func TestSyncWarnsOfCrossStoreRefs(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
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
	if err := c.SetStoreAccount("org-a", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	e, err = c.Select("org-a")
	if err != nil {
		t.Fatal(err)
	}
	r, err := replication.SyncStore(ts.Session, c, e, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "API-1 blocked_by points at org-b's WEB-1; references stay inside a store"
	if !slices.Contains(r.Warnings, want) {
		t.Errorf("Warnings = %q; want %q", r.Warnings, want)
	}
}
