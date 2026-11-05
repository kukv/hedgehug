package usecase_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// A link written by hand in other letters is still the link: GitHub's answer
// reaches gh, linking again finds it, and the view shows its state.
func TestFetchTakesALinkWrittenInOtherLetters(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) {
		x.Repos = []string{web}
		x.Links = []model.Link{{Type: model.LinkPR, Ref: "github.com/Org-A/Web/pull/1", Sync: model.SyncMirror}}
	})
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	x := taskIn(t, ts, "me", "ME-1")
	g, _ := x.GitHub()
	if len(g.PRs) != 1 || g.PRs[0].State != "open" {
		t.Fatalf("gh = %+v", g)
	}
	_, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#1"})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "link.exists")
	if v := usecase.LinkViewsOf(x); len(v) != 1 || v[0].State != "open" {
		t.Errorf("view = %+v", v)
	}
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#1"}); err != nil {
		t.Fatal(err)
	}
	if l := taskIn(t, ts, "me", "ME-1").Links; len(l) != 1 || l[0].Sync != model.SyncOff {
		t.Errorf("links after unlink = %+v", l)
	}
}

// The view carries when a pull request was merged and why an issue closed.
func TestLinkViewCarriesMergedAtAndStateReason(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) {
		x.Repos = []string{web}
		x.Links = []model.Link{
			{Type: model.LinkPR, Ref: "github.com/org-a/web/pull/1", Sync: model.SyncMirror},
			{Type: model.LinkIssue, Ref: "github.com/org-a/web/issues/2", Sync: model.SyncLink},
		}
	})
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if err := fake.MergePull("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	if n, err := fake.OpenIssue("org-a/web", "I", ""); err != nil || n != 2 {
		t.Fatalf("issue %d, %v", n, err)
	}
	if err := fake.CloseIssue("org-a/web", 2, "not_planned"); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	v := usecase.LinkViewsOf(taskIn(t, ts, "me", "ME-1"))
	if len(v) != 2 || v[0].MergedAt == nil || v[1].StateReason != "not_planned" {
		t.Fatalf("view = %+v", v)
	}
	if v[1].MergedAt != nil || v[0].StateReason != "" {
		t.Errorf("view = %+v", v)
	}
}

// When the commit fails, the repository that could not be read is still noted,
// so that it is not asked again at once.
func TestFetchNotesAFailedReadWhenTheCommitFails(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x := ts.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-b/secret", web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	ts.BeforeUpdate = func() {
		ts.BeforeUpdate = nil
		rel := model.TaskPath(x.ULID)
		if err := os.WriteFile(filepath.Join(e.Path, rel), []byte("not a task"), 0o600); err != nil {
			t.Error(err)
		}
		gitrepotest.CommitFiles(t, e.Path, "break", "Alice", "alice@example.com", ts.Now(), filepath.ToSlash(rel))
	}
	if _, err := usecase.FetchGitHub(ts.Session, c, e); err == nil {
		t.Fatal("the commit did not fail")
	}
	s, err := ts.ForStore("me").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := ghstate.Read(s)
	if err != nil {
		t.Fatal(err)
	}
	if r := st.Repos["github.com/org-b/secret"]; r.LastTry.IsZero() || r.Warning == "" {
		t.Errorf("state = %+v", r)
	}
	if !st.Repos[web].LastFetch.IsZero() {
		t.Errorf("the repository whose result was lost moved: %+v", st.Repos[web])
	}
}
