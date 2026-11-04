package usecase_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// duplicateFx moves API-2 (and what is under it) to org-b, then puts the old
// file of API-2 back in org-a over its forward: the task lives in both, as
// after two moves that crossed.
func duplicateFx(t *testing.T) *usecasetest.MoveFx {
	t.Helper()
	f := usecasetest.NewMoveFx(t)
	s, err := f.TS.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Dir(), filepath.FromSlash(model.TaskPath(f.P)))
	old, err := os.ReadFile(p) //nolint:gosec // a file of the test's own store
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, old, 0o600); err != nil {
		t.Fatal(err)
	}
	author := config.CommitAuthor(usecasetest.TestAccount)
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{model.TaskPath(f.P)}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	f.TS.DropMount()
	return f
}

// idIn is the id the root task of the fixture has in the store name.
func idIn(t *testing.T, f *usecasetest.MoveFx, name string) string {
	t.Helper()
	e, ok := usecasetest.MustIndex(t, f.TS, name).Task(f.P)
	if !ok {
		t.Fatalf("%s lacks the task", name)
	}
	return e.ID
}

func findingsOf(t *testing.T, ts *usecasetest.Session, store, check string) []maintenance.Finding {
	t.Helper()
	rs, err := maintenance.Doctor(ts.ForStore(store), false)
	if err != nil {
		t.Fatal(err)
	}
	var out []maintenance.Finding
	for _, f := range rs[0].Findings {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

func TestDoctorDuplicateAcrossStores(t *testing.T) {
	t.Parallel()
	f := duplicateFx(t)
	bID := idIn(t, f, "org-b")
	want := "in org-a (API-2) and org-b (" + bID + "); remove the one not needed with hedgehug rm and hedgehug purge --older-than 0d --store <store>"
	for _, store := range []string{"org-a", "org-b"} {
		got := findingsOf(t, f.TS, store, "duplicate_across_stores")
		if len(got) != 1 || got[0].Where != f.P.String() || got[0].Task != f.P.String() || got[0].Msg != want || got[0].Severity != maintenance.Warning {
			t.Errorf("%s: findings = %+v; want one warning for %s: %s", store, got, f.P, want)
		}
	}
	// a sync of both stores tells of the duplicate once, in the first report
	reports := []usecase.SyncReport{{Store: "org-a"}, {Store: "org-b"}}
	f.TS.AddDuplicateWarnings(reports)
	if w := reports[0].Warnings; len(w) != 1 || w[0] != f.P.String()+" is "+want {
		t.Errorf("warnings of org-a = %q", w)
	}
	if w := reports[1].Warnings; len(w) != 0 {
		t.Errorf("warnings of org-b = %q; the duplicate is told once", w)
	}
	// a store that has no remote or failed is not told of
	reports = []usecase.SyncReport{{Store: "org-a", NoRemote: true}, {Store: "org-b"}}
	f.TS.AddDuplicateWarnings(reports)
	if len(reports[0].Warnings) != 0 || len(reports[1].Warnings) != 1 {
		t.Errorf("warnings = %q, %q", reports[0].Warnings, reports[1].Warnings)
	}
}

func TestDuplicateIgnoresADeletedTask(t *testing.T) {
	t.Parallel()
	f := duplicateFx(t)
	// the copy in org-b is deleted: nothing lives in both
	if _, err := tasks.Remove(f.TS.ForStore("org-b"), idIn(t, f, "org-b")); err != nil {
		t.Fatal(err)
	}
	f.TS.DropMount()
	if got := findingsOf(t, f.TS, "org-a", "duplicate_across_stores"); len(got) != 0 {
		t.Errorf("findings = %+v; want none for a deleted task", got)
	}
}

func TestDuplicateTextOfThree(t *testing.T) {
	t.Parallel()
	d := usecase.Duplicate{Places: []usecase.DupPlace{{"org-a", "API-1"}, {"org-b", "WEB-2"}, {"org-c", "OPS-3"}}}
	if got, want := d.Text(), "in org-a (API-1), org-b (WEB-2) and org-c (OPS-3); remove"; !strings.HasPrefix(got, want) {
		t.Errorf("text = %q; want it to start %q", got, want)
	}
}

func TestDoctorForwardTarget(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	if got := findingsOf(t, f.TS, "org-a", "forward_target"); len(got) != 0 {
		t.Fatalf("findings = %+v; want none while org-b is mounted", got)
	}
	bs, err := f.TS.ForStore("org-b").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stores.StoreRemove(f.TS.Session, "org-b", true); err != nil {
		t.Fatal(err)
	}
	f.TS.DropMount()
	got := findingsOf(t, f.TS, "org-a", "forward_target")
	if len(got) != 3 {
		t.Fatalf("findings = %+v; want one for each forward", got)
	}
	for _, x := range got {
		if x.Severity != maintenance.Info || x.Msg != "moved to a store that is not mounted (ULID "+bs.Meta().ULID.String()+")" || !strings.HasPrefix(x.Where, "API-") {
			t.Errorf("finding = %+v", x)
		}
	}
	if maintenance.Failing(got) {
		t.Error("an info finding fails doctor")
	}
}
