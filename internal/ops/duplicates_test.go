package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

// duplicateFx moves API-2 (and what is under it) to org-b, then puts the old
// file of API-2 back in org-a over its forward: the task lives in both, as
// after two moves that crossed.
func duplicateFx(t *testing.T) *moveFx {
	t.Helper()
	f := newMoveFx(t)
	s, err := f.ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Dir(), filepath.FromSlash(model.TaskPath(f.p)))
	old, err := os.ReadFile(p) //nolint:gosec // a file of the test's own store
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, old, 0o600); err != nil {
		t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, f.ts.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{model.TaskPath(f.p)}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	f.ts.dropMount()
	return f
}

// idIn is the id the root task of the fixture has in the store name.
func idIn(t *testing.T, f *moveFx, name string) string {
	t.Helper()
	e, ok := mustIndex(t, f.ts, name).Task(f.p)
	if !ok {
		t.Fatalf("%s lacks the task", name)
	}
	return e.ID
}

func findingsOf(t *testing.T, ts *testSession, store, check string) []doctor.Finding {
	t.Helper()
	rs, err := ts.ForStore(store).Doctor(false)
	if err != nil {
		t.Fatal(err)
	}
	var out []doctor.Finding
	for _, f := range rs[0].Findings {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

func TestDoctorDuplicateAcrossStores(t *testing.T) {
	f := duplicateFx(t)
	bID := idIn(t, f, "org-b")
	want := "in org-a (API-2) and org-b (" + bID + "); remove the one not needed with hedgehug rm and hedgehug purge"
	for _, store := range []string{"org-a", "org-b"} {
		got := findingsOf(t, f.ts, store, "duplicate_across_stores")
		if len(got) != 1 || got[0].Where != f.p.String() || got[0].Msg != want || got[0].Severity != doctor.Warning {
			t.Errorf("%s: findings = %+v; want one warning for %s: %s", store, got, f.p, want)
		}
	}
	// a sync of both stores tells of the duplicate once, in the first report
	reports := []SyncReport{{Store: "org-a"}, {Store: "org-b"}}
	f.ts.AddDuplicateWarnings(reports)
	if w := reports[0].Warnings; len(w) != 1 || w[0] != f.p.String()+" is "+want {
		t.Errorf("warnings of org-a = %q", w)
	}
	if w := reports[1].Warnings; len(w) != 0 {
		t.Errorf("warnings of org-b = %q; the duplicate is told once", w)
	}
	// a store that has no remote or failed is not told of
	reports = []SyncReport{{Store: "org-a", NoRemote: true}, {Store: "org-b"}}
	f.ts.AddDuplicateWarnings(reports)
	if len(reports[0].Warnings) != 0 || len(reports[1].Warnings) != 1 {
		t.Errorf("warnings = %q, %q", reports[0].Warnings, reports[1].Warnings)
	}
}

func TestDuplicateIgnoresADeletedTask(t *testing.T) {
	f := duplicateFx(t)
	// the copy in org-b is deleted: nothing lives in both
	if _, err := f.ts.ForStore("org-b").Remove(idIn(t, f, "org-b")); err != nil {
		t.Fatal(err)
	}
	f.ts.dropMount()
	if got := findingsOf(t, f.ts, "org-a", "duplicate_across_stores"); len(got) != 0 {
		t.Errorf("findings = %+v; want none for a deleted task", got)
	}
}

func TestDuplicateTextOfThree(t *testing.T) {
	d := duplicate{Places: []dupPlace{{"org-a", "API-1"}, {"org-b", "WEB-2"}, {"org-c", "OPS-3"}}}
	if got, want := d.text(), "in org-a (API-1), org-b (WEB-2) and org-c (OPS-3); remove"; !strings.HasPrefix(got, want) {
		t.Errorf("text = %q; want it to start %q", got, want)
	}
}

func TestDoctorForwardTarget(t *testing.T) {
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	if got := findingsOf(t, f.ts, "org-a", "forward_target"); len(got) != 0 {
		t.Fatalf("findings = %+v; want none while org-b is mounted", got)
	}
	bs, err := f.ts.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.ts.StoreRemove("org-b", false); err != nil {
		t.Fatal(err)
	}
	f.ts.dropMount()
	got := findingsOf(t, f.ts, "org-a", "forward_target")
	if len(got) != 3 {
		t.Fatalf("findings = %+v; want one for each forward", got)
	}
	for _, x := range got {
		if x.Severity != doctor.Info || x.Msg != "moved to a store that is not mounted (ULID "+bs.Meta().ULID.String()+")" || !strings.HasPrefix(x.Where, "API-") {
			t.Errorf("finding = %+v", x)
		}
	}
	if doctor.Failing(got) {
		t.Error("an info finding fails doctor")
	}
}
