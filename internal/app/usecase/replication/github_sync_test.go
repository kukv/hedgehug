package replication_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// web is the repository GitHubSession lets alice write.
const web = "github.com/org-a/web"

func TestSyncReadsGitHubAndPushes(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	usecasetest.AddBareRemote(t, ts)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r, err := replication.SyncOne(ts.Session)
	if err != nil || r.Err != nil {
		t.Fatalf("sync: %v, %v", err, r.Err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if n, err := usecase.Unpushed(s); err != nil || n != 0 {
		t.Fatalf("unpushed = %d, %v", n, err)
	}
	if len(ts.AddedTask("ME-1").Links) != 1 {
		t.Error("not linked")
	}
}

func TestSyncOfAStoreWithoutARemoteReadsGitHub(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.SyncOne(ts.Session); err != nil {
		t.Fatal(err)
	}
	if len(ts.AddedTask("ME-1").Links) != 1 {
		t.Error("not linked")
	}
}

func TestSyncOfATaskWithNoRepositoryNeedsNoAccount(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.DropAccounts()
	r, err := replication.SyncOne(ts.Session)
	if err != nil || len(r.Warnings) != 0 {
		t.Fatalf("sync = %+v, %v", r, err)
	}
}

func TestSyncPushesOnceToEachRemote(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	usecasetest.AddBareRemote(t, ts)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r, err := replication.SyncOne(ts.Session)
	if err != nil || r.Err != nil {
		t.Fatalf("sync: %v, %v", err, r.Err)
	}
	if len(r.Pushed) == 0 {
		t.Fatal("nothing pushed")
	}
	sorted := slices.Sorted(slices.Values(r.Pushed))
	if len(slices.Compact(slices.Clone(sorted))) != len(sorted) {
		t.Errorf("pushed = %q", r.Pushed)
	}
}

// The push of what GitHub gave fails: the sync has succeeded, and the commit
// is pushed by the next one.
func TestSyncWithAFailedPushOfWhatGitHubGave(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	remote := usecasetest.AddBareRemote(t, ts)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	away := remote + ".away"
	ts.BeforeUpdate = func() { // the remote goes away after the sync, before the push
		ts.BeforeUpdate = nil
		if err := os.Rename(remote, away); err != nil {
			t.Error(err)
		}
	}
	r, err := replication.SyncOne(ts.Session)
	if err != nil || r.Err != nil {
		t.Fatalf("sync: %v, %v", err, r.Err)
	}
	if !slices.ContainsFunc(r.Warnings, func(w string) bool { return strings.Contains(w, "push the values from GitHub") }) {
		t.Fatalf("warnings = %q", r.Warnings)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if n, err := usecase.Unpushed(s); err != nil || n == 0 {
		t.Fatalf("unpushed = %d, %v", n, err)
	}
	if err := os.Rename(away, remote); err != nil {
		t.Fatal(err)
	}
	if r, err := replication.SyncOne(ts.Session); err != nil || r.Err != nil {
		t.Fatalf("next sync: %v, %v", err, r.Err)
	}
	if n, err := usecase.Unpushed(s); err != nil || n != 0 {
		t.Errorf("unpushed = %d, %v", n, err)
	}
}

// A store with no remote that names a repository is synced by hh sync with
// no arguments, and by the loop of the GUI.
func TestSyncOfAllReadsGitHubForAStoreWithoutRemote(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	reps, err := replication.Sync(ts.Session, false, true)
	if err != nil || len(reps) != 1 {
		t.Fatalf("reports = %+v, %v", reps, err)
	}
	if r := reps[0]; r.Store != "me" || !r.NoRemote || !r.GitHubUpdated {
		t.Errorf("report = %+v", r)
	}
	if len(ts.AddedTask("ME-1").Links) != 1 {
		t.Error("not linked")
	}
}

func TestSyncOfAllSkipsAStoreWithoutRemoteAndRepository(t *testing.T) {
	t.Parallel()
	ts, _ := usecasetest.GitHubSession(t)
	ts.AddTask("a", nil)
	reps, err := replication.Sync(ts.Session, false, true)
	if err != nil || len(reps) != 0 {
		t.Fatalf("reports = %+v, %v", reps, err)
	}
}
