package integration_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
)

// A store with no remote that names a repository is synced by hh sync with
// no arguments, and by the loop of the GUI.
func TestSyncOfAllReadsGitHubForAStoreWithoutRemote(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
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
	if len(taskIn(t, ts, "me", "ME-1").Links) != 1 {
		t.Error("not linked")
	}
}

func TestSyncOfAllSkipsAStoreWithoutRemoteAndRepository(t *testing.T) {
	t.Parallel()
	ts, _ := withFake(t)
	ts.AddTask("a", nil)
	reps, err := replication.Sync(ts.Session, false, true)
	if err != nil || len(reps) != 0 {
		t.Fatalf("reports = %+v, %v", reps, err)
	}
}
