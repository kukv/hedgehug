package integration_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

const web = "github.com/org-a/web"

func firstIssue(t *testing.T, fake *fakegh.Server) *fakegh.Issue {
	t.Helper()
	r, ok := fake.Repo("org-a/web")
	if !ok {
		t.Fatal("no repository")
	}
	for _, x := range r.Issues {
		if x.Number == 1 {
			return x
		}
	}
	t.Fatal("no issue 1")
	return nil
}

func taskAt(t *testing.T, ts *usecasetest.Session, id string) *model.Task {
	t.Helper()
	s, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ts.Resolve(ix, id)
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

// reread reads GitHub after the interval, and fails on a warning.
func reread(t *testing.T, ts *usecasetest.Session) {
	t.Helper()
	now := ts.Now().Add(time.Hour)
	ts.Now = func() time.Time { return now }
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	r, err := usecase.FetchGitHub(ts.Session, c, e)
	if err != nil || len(r.Warnings) != 0 {
		t.Fatalf("%+v, %v", r, err)
	}
}

func TestCreateIssue(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	if err := fake.AddLabel("org-a/web", "bug"); err != nil {
		t.Fatal(err)
	}
	x := ts.AddTask("a", func(x *model.Task) {
		x.Repos, x.Body, x.Labels, x.Assignees = []string{web}, "text", []string{"bug"}, []string{"github.com/alice"}
	})
	r, err := integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Ref.Label() != "org-a/web#1" || !r.Committed || len(r.Warnings) != 0 {
		t.Errorf("result %+v", r)
	}
	if op := usecasetest.LastOp(t, ts); op != "issue" {
		t.Errorf("op %s", op)
	}
	got := firstIssue(t, fake)
	if got.Title != "a" || got.Body != "text\n\nhedgehug:"+x.ULID.String() || len(got.Labels) != 0 || !slices.Equal(got.Assignees, []string{"alice"}) {
		t.Errorf("issue %+v", got)
	}
	cur := taskAt(t, ts, "ME-1")
	if len(cur.Links) != 1 || cur.Links[0].Sync != model.SyncPush || cur.Links[0].Ref != web+"/issues/1" {
		t.Errorf("links %+v", cur.Links)
	}
	g, err := cur.GitHub()
	if err != nil || len(g.Issues) != 1 || g.Issues[0].State != "open" {
		t.Fatalf("gh %+v, %v", g, err)
	}
	if v, err := g.Issues[0].Values(); err != nil || !slices.Equal(v["title"], []string{"a"}) {
		t.Errorf("base %v, %v", v, err)
	}
	// The base is what GitHub holds: the next read writes nothing.
	at := got.UpdatedAt
	before := usecasetest.LastOp(t, ts)
	reread(t, ts)
	if op := usecasetest.LastOp(t, ts); op != before {
		t.Errorf("op after the read: %s", op)
	}
	if firstIssue(t, fake).UpdatedAt != at {
		t.Error("the issue was written again")
	}
}

func TestCreateIssueForAClosedTask(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web}; x.Status = "canceled" })
	if _, err := integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{}); err != nil {
		t.Fatal(err)
	}
	if got := firstIssue(t, fake); got.State != "closed" || got.StateReason != "not_planned" {
		t.Errorf("issue %+v", got)
	}
	g, err := taskAt(t, ts, "ME-1").GitHub()
	if err != nil || g.Issues[0].State != "closed" || g.Issues[0].StateReason == nil {
		t.Errorf("gh %+v, %v", g, err)
	}
	reread(t, ts)
}

// The issue is there when it does not close: it is linked, so that a second
// run opens no other, and the next sync closes it.
func TestCreateIssueThatDoesNotClose(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web}; x.Status = "done" })
	fake.RefuseIssueEdits(true)
	r, err := integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "close org-a/web#1") {
		t.Errorf("warnings %v", r.Warnings)
	}
	cur := taskAt(t, ts, "ME-1")
	g, err := cur.GitHub()
	if err != nil || len(cur.Links) != 1 || g.Issues[0].State != "open" {
		t.Fatalf("links %+v, gh %+v, %v", cur.Links, g, err)
	}
	fake.RefuseIssueEdits(false)
	reread(t, ts)
	if got := firstIssue(t, fake); got.State != "closed" {
		t.Errorf("issue %+v", got)
	}
}

func TestCreateIssueLeavesOutWhatGitHubDoesNotKnow(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) {
		x.Repos, x.Assignees = []string{web}, []string{"github.com/nobody"}
	})
	r, err := integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{})
	if err != nil {
		t.Fatal(err)
	}
	if w := strings.Join(r.Warnings, "\n"); !strings.Contains(w, `assignee "nobody"`) {
		t.Errorf("warnings %v", r.Warnings)
	}
	if got := firstIssue(t, fake); len(got.Labels) != 0 || len(got.Assignees) != 0 {
		t.Errorf("issue %+v", got)
	}
}

func TestCreateIssueChoosesTheRepository(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web, "github.com/org-a/api"} })
	_, err := integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{})
	var ue *usecase.Error
	if !errors.As(err, &ue) || ue.Code != "choice.required" {
		t.Fatalf("no choice: %v", err)
	}
	if _, err := integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{Repo: "org-a/web"}); err != nil {
		t.Fatal(err)
	}
	firstIssue(t, fake)
	// A second issue in the repository is a mistake.
	_, err = integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{Repo: "org-a/web"})
	if !errors.As(err, &ue) || ue.Code != "link.exists" {
		t.Fatalf("again: %v", err)
	}
}

func TestCreateIssueInTheRepositoryOfTheCode(t *testing.T) {
	t.Parallel()
	env := newPREnv(t, fakegh.Repo{})
	env.ts.AddTask("a", nil)
	r, err := integration.CreateIssue(env.ts.Session, "ME-1", integration.IssueInput{Dir: env.dir})
	if err != nil {
		t.Fatal(err)
	}
	if r.Ref.String() != web+"/issues/1" {
		t.Errorf("ref %s", r.Ref)
	}
}

func TestCreateIssueRefusals(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := tasks.Remove(ts.Session, "ME-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := integration.CreateIssue(ts.Session, "ME-1", integration.IssueInput{}); err == nil {
		t.Error("a deleted task")
	}
	ts.AddTask("b", func(x *model.Task) { x.Repos = []string{web} })
	if err := fake.SetPerm("org-a/web", "alice", "read"); err != nil {
		t.Fatal(err)
	}
	if _, err := integration.CreateIssue(ts.Session, "ME-2", integration.IssueInput{}); err == nil {
		t.Error("no write access")
	}
	if l := taskAt(t, ts, "ME-2").Links; len(l) != 0 {
		t.Errorf("links %+v", l)
	}
}

func TestChangingTheModeForgetsTheBase(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "a", "")
	if err != nil {
		t.Fatal(err)
	}
	ref := "org-a/web#" + strconv.Itoa(n)
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: ref, Sync: model.SyncMirror}); err != nil {
		t.Fatal(err)
	}
	reread(t, ts)
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: ref, Sync: model.SyncLink}); err != nil {
		t.Fatal(err)
	}
	g, err := taskAt(t, ts, "ME-1").GitHub()
	if err != nil {
		t.Fatal(err)
	}
	if g.Issues[0].Synced != nil || g.Issues[0].State != "open" {
		t.Errorf("gh %+v", g.Issues[0])
	}
}

func openLabeled(t *testing.T, fake *fakegh.Server, title, body string) int {
	t.Helper()
	n, err := fake.OpenIssue("org-a/web", title, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("org-a/web", n, []string{"alice"}, []string{"hh"}, ""); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestImportIssues(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	parent := ts.AddTask("parent", nil)
	a := openLabeled(t, fake, "A", "their text")
	openLabeled(t, fake, "B", "")
	if _, err := fake.OpenIssue("org-a/web", "not this one", ""); err != nil {
		t.Fatal(err)
	}
	in := integration.ImportInput{Repo: "org-a/web", Label: "hh", Parent: "ME-1", DryRun: true}
	r, err := integration.ImportIssues(ts.Session, in)
	if err != nil {
		t.Fatal(err)
	}
	if r.Count != 2 || len(r.Created) != 0 || !r.DryRun {
		t.Errorf("dry run %+v", r)
	}
	if ids := ts.TaskULIDs("me"); len(ids) != 2 { // the parent and the fixture project
		t.Errorf("a dry run wrote %d tasks", len(ids))
	}
	in.DryRun = false
	if r, err = integration.ImportIssues(ts.Session, in); err != nil {
		t.Fatal(err)
	}
	if r.Count != 2 || !slices.Equal(r.Created, []string{"ME-3", "ME-4"}) || len(r.Warnings) != 0 {
		t.Fatalf("import %+v", r)
	}
	x := taskAt(t, ts, "ME-3")
	if x.Kind != "step" || x.Title != "A" || x.Body != "their text" || x.Parent == nil || *x.Parent != parent.ULID || !slices.Equal(x.Repos, []string{web}) {
		t.Errorf("task %+v", x)
	}
	if len(x.Links) != 1 || x.Links[0].Sync != model.SyncMirror || x.Links[0].Ref != web+"/issues/"+strconv.Itoa(a) {
		t.Errorf("links %+v", x.Links)
	}
	// The fetch at the end brought the rest.
	if !slices.Equal(x.Assignees, []string{"github.com/alice"}) {
		t.Errorf("assignees %v", x.Assignees)
	}
	// The body was copied once.
	if err := fake.EditIssue("org-a/web", a, "", "other"); err != nil {
		t.Fatal(err)
	}
	reread(t, ts)
	if got := taskAt(t, ts, "ME-3").Body; got != "their text" {
		t.Errorf("body %q", got)
	}
	// What is linked is passed over, even when it was unlinked.
	if _, err := integration.Unlink(ts.Session, "ME-3", integration.LinkInput{Issue: "org-a/web#" + strconv.Itoa(a)}); err != nil {
		t.Fatal(err)
	}
	if r, err = integration.ImportIssues(ts.Session, in); err != nil {
		t.Fatal(err)
	}
	if r.Count != 0 || r.Skipped != 2 {
		t.Errorf("again %+v", r)
	}
}

func TestImportWithoutALabelTakesEveryOpenIssue(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("parent", nil)
	openLabeled(t, fake, "A", "")
	if _, err := fake.OpenIssue("org-a/web", "no label", ""); err != nil {
		t.Fatal(err)
	}
	r, err := integration.ImportIssues(ts.Session, integration.ImportInput{Repo: "org-a/web", Parent: "ME-1", DryRun: true})
	if err != nil || r.Count != 2 {
		t.Fatalf("%+v, %v", r, err)
	}
	r, err = integration.ImportIssues(ts.Session, integration.ImportInput{Repo: "org-a/web", Label: "hh", Parent: "ME-1", DryRun: true})
	if err != nil || r.Count != 1 {
		t.Fatalf("with the label: %+v, %v", r, err)
	}
}

func TestImportNeedsAParent(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	openLabeled(t, fake, "A", "")
	in := integration.ImportInput{Repo: "org-a/web", Label: "hh"}
	_, err := integration.ImportIssues(ts.Session, in)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "import.parent_required")
	in.DryRun = true
	_, err = integration.ImportIssues(ts.Session, in)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "import.parent_required")
	if ids := ts.TaskULIDs("me"); len(ids) != 0 {
		t.Errorf("a refused import wrote %d tasks", len(ids))
	}
}

func TestImportGoesUnderTheProjectOfUse(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	openLabeled(t, fake, "A", "")
	id := ts.AddProject("Release")
	if _, err := stores.Use(ts.Session, id.String()); err != nil {
		t.Fatal(err)
	}
	r, err := integration.ImportIssues(ts.Session, integration.ImportInput{Repo: "org-a/web", Label: "hh"})
	if err != nil || len(r.Created) != 1 {
		t.Fatalf("%+v, %v", r, err)
	}
	x := taskAt(t, ts, r.Created[0])
	if x.Kind != "task" || x.Parent == nil || *x.Parent != id {
		t.Errorf("kind %q parent %v; want a task under the project", x.Kind, x.Parent)
	}
}
