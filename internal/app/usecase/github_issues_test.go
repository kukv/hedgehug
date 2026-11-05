package usecase_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

func itoa(n int) string { return strconv.Itoa(n) }

func assign(t *testing.T, s string) []usecase.Assign {
	t.Helper()
	a, err := usecase.ParseAssign(s)
	if err != nil {
		t.Fatal(err)
	}
	return []usecase.Assign{a}
}

// refetch reads GitHub again, after the interval.
func refetch(t *testing.T, ts *usecasetest.Session) []string {
	t.Helper()
	later(ts, time.Hour)
	return fetch(t, ts).Warnings
}

// issueTask opens an issue and links it to ME-1 in the mode, and reads it once.
func issueTask(t *testing.T, ts *usecasetest.Session, fake *fakegh.Server, mode string) int {
	t.Helper()
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "a", "text")
	if err != nil {
		t.Fatal(err)
	}
	link(t, ts, n, mode)
	return n
}

func link(t *testing.T, ts *usecasetest.Session, n int, mode string) {
	t.Helper()
	in := integration.LinkInput{Issue: "org-a/web#" + itoa(n), Sync: mode}
	if _, err := integration.Link(ts.Session, "ME-1", in); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings: %v", w)
	}
}

func trailers(es []gitrepo.LogEntry, key string) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.TrailerValues(key)...)
	}
	return out
}

func syncedOf(t *testing.T, ts *usecasetest.Session) model.IssueValues {
	t.Helper()
	g, err := taskIn(t, ts, "me", "ME-1").GitHub()
	if err != nil {
		t.Fatal(err)
	}
	v, err := g.Issues[0].Values()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMirrorTakesTheFirstValuesSilently(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web}; x.Body = "my text" })
	n, err := fake.OpenIssue("org-a/web", "theirs", "their text")
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("org-a/web", n, []string{"alice"}, nil, ""); err != nil {
		t.Fatal(err)
	}
	link(t, ts, n, model.SyncMirror)
	x := taskIn(t, ts, "me", "ME-1")
	if x.Title != "theirs" || x.Body != "my text" || !slices.Equal(x.Assignees, []string{"github.com/alice"}) {
		t.Errorf("task %+v", x)
	}
	if !slices.Contains(x.Watchers, "github.com/alice") {
		t.Errorf("watchers %v", x.Watchers)
	}
	es := logOf(t, ts)
	if op, _ := es[0].Trailer("Hedgehug-Op"); op != "gh-mirror" {
		t.Errorf("op %s", op)
	}
	if !strings.HasPrefix(es[0].Author.Name, "hedgehug-automation") {
		t.Errorf("author %+v", es[0].Author)
	}
	if got := trailers(es, automation.TrailerIssueOverwritten); got != nil {
		t.Errorf("notes %v", got)
	}
	v := syncedOf(t, ts)
	if !slices.Equal(v["title"], []string{"theirs"}) || !slices.Equal(v["status"], []string{"open"}) || !slices.Equal(v["assignees"], []string{"alice"}) {
		t.Errorf("synced %v", v)
	}
	if _, ok := v["body"]; ok {
		t.Errorf("body is not mirrored by default: %v", v)
	}
	// Nothing changed: nothing is written.
	n0 := len(logOf(t, ts))
	refetch(t, ts)
	if got := len(logOf(t, ts)); got != n0 {
		t.Errorf("%d commits added", got-n0)
	}
}

func TestMirrorFollowsGitHub(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncMirror)
	if err := fake.EditIssue("org-a/web", n, "new", ""); err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("org-a/web", n, []string{"alice"}, nil, ""); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	x := taskIn(t, ts, "me", "ME-1")
	if x.Title != "new" || len(x.Assignees) != 1 {
		t.Errorf("task %+v", x)
	}
	if got := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); got != nil {
		t.Errorf("notes %v", got)
	}
}

// An assignee of another host stays: GitHub here says nothing of them.
func TestMirrorKeepsOtherHosts(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncMirror)
	if _, err := tasks.Set(ts.Session, "ME-1", assign(t, "assignees+=ghe.example.com/zoe")); err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("org-a/web", n, []string{"alice"}, nil, ""); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if got := taskIn(t, ts, "me", "ME-1").Assignees; !slices.Equal(got, []string{"ghe.example.com/zoe", "github.com/alice"}) {
		t.Errorf("assignees %v", got)
	}
}

func TestMirrorPutsAHandEditBack(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	issueTask(t, ts, fake, model.SyncMirror)
	if _, err := tasks.Set(ts.Session, "ME-1", assign(t, "title=mine")); err != nil {
		t.Fatal(err)
	}
	later(ts, time.Hour)
	if r := fetch(t, ts); !r.Committed {
		t.Fatalf("report %+v", r)
	}
	if got := taskIn(t, ts, "me", "ME-1").Title; got != "a" {
		t.Errorf("title %s", got)
	}
	notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten)
	if len(notes) != 1 {
		t.Fatalf("notes %v", notes)
	}
	_, ref, o, ok := automation.ParseIssueOverwritten(notes[0])
	if !ok || !strings.HasSuffix(ref, "/issues/1") || o.Field != "title" || o.Side != automation.SideTask || !slices.Equal(o.Lost, []string{"mine"}) {
		t.Errorf("note %q", notes[0])
	}
}

func TestMirrorState(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncMirror)
	// Open leaves a task that is under way where it is.
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "in_progress"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "in_progress")
	if err := fake.CloseIssue("org-a/web", n, "completed"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "done")
	if v, _ := logOf(t, ts)[0].Trailer(automation.TrailerApplied); v != "mirror issue.mirror" {
		t.Errorf("trailer %q", v)
	}
	if err := fake.ReopenIssue("org-a/web", n); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "todo")
	if err := fake.CloseIssue("org-a/web", n, "not_planned"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "canceled")
}

// A status of the done category that is not the default stays.
func TestMirrorKeepsAnotherDoneStatus(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, func(sc *schema.Schema, f *automation.File) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "shipped", Label: "shipped", Category: schema.CategoryDone})
		f.Rules = nil
	})
	n := issueTask(t, ts, fake, model.SyncMirror)
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "shipped"); err != nil {
		t.Fatal(err)
	}
	if err := fake.CloseIssue("org-a/web", n, "completed"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "shipped")
}

// The rules come after mirror: issue-closed sees the task done already, and
// a rule to a status of its own goes on from there.
func TestRulesAfterMirror(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, func(sc *schema.Schema, f *automation.File) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "shipped", Label: "shipped", Category: schema.CategoryDone})
		f.Rules = append(f.Rules, automation.Rule{ID: "ship", On: automation.IssueClosed, Set: automation.Set{Status: "shipped"}, Enabled: true})
	})
	n := issueTask(t, ts, fake, model.SyncMirror)
	if err := fake.CloseIssue("org-a/web", n, "completed"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "shipped")
	wantApplied(t, ts, "ship issue.closed")
	es := logOf(t, ts)
	if op, _ := es[1].Trailer("Hedgehug-Op"); op != "gh-mirror" {
		t.Errorf("the commit before the rule: %s", op)
	}
	// The status is on the side of the issue: nothing to put back.
	n0 := len(es)
	refetch(t, ts)
	if got := len(logOf(t, ts)); got != n0 {
		t.Errorf("%d commits added", got-n0)
	}
}

// issue-closed says done; an issue closed as not planned stays canceled, and
// is not moved back and forth.
func TestNotPlannedStaysCanceled(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, nil)
	n := issueTask(t, ts, fake, model.SyncMirror)
	if err := fake.CloseIssue("org-a/web", n, "not_planned"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "canceled")
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "canceled")
	if got := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); got != nil {
		t.Errorf("notes %v", got)
	}
}

// With the status not mirrored, closing is for the rules.
func TestNotPlannedFiresWithoutTheStatusField(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, func(_ *schema.Schema, f *automation.File) { f.IssueFields.Mirror = []string{"title"} })
	n := issueTask(t, ts, fake, model.SyncMirror)
	if err := fake.CloseIssue("org-a/web", n, "not_planned"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "done")
}

func TestMirrorRefusedOnce(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, func(sc *schema.Schema, f *automation.File) {
		sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
		f.Rules = nil
	})
	n := issueTask(t, ts, fake, model.SyncMirror)
	if err := fake.CloseIssue("org-a/web", n, "completed"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "todo")
	got := trailers(logOf(t, ts), automation.TrailerRejected)
	if len(got) != 1 || !strings.Contains(got[0], " mirror ") {
		t.Fatalf("rejections %v", got)
	}
	n0 := len(logOf(t, ts))
	refetch(t, ts)
	refetch(t, ts)
	if got := len(logOf(t, ts)); got != n0 {
		t.Errorf("%d commits added", got-n0)
	}
}

// A write that the schema refuses is a warning. The values and the base stay
// together, so that the next fetch compares them again, and the rest of the
// fetch is done.
func TestMirrorThatCannotBeWrittenIsTriedAgain(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, func(sc *schema.Schema, f *automation.File) {
		sc.Props = append(sc.Props, schema.Prop{Name: "target", Type: schema.TypeNumber})
		f.IssueFields.Mirror = []string{"title", "milestone"}
		f.IssueFields.Props = map[string]string{"milestone": "target"}
		f.Rules = nil
	})
	n := issueTask(t, ts, fake, model.SyncMirror)
	if err := fake.EditIssue("org-a/web", n, "new", ""); err != nil {
		t.Fatal(err)
	}
	if err := fake.TagIssue("org-a/web", n, nil, nil, "v1"); err != nil {
		t.Fatal(err)
	}
	w := refetch(t, ts)
	if len(w) != 1 || !strings.Contains(w[0], "target") {
		t.Fatalf("warnings %v", w)
	}
	if got := taskIn(t, ts, "me", "ME-1").Title; got != "a" {
		t.Errorf("title %s", got)
	}
	if v := syncedOf(t, ts); !slices.Equal(v["title"], []string{"a"}) {
		t.Errorf("base %v", v)
	}
	if err := fake.TagIssue("org-a/web", n, nil, nil, "12"); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	x := taskIn(t, ts, "me", "ME-1")
	if x.Title != "new" || x.Props.Get("target") == nil {
		t.Errorf("task %+v", x)
	}
}

func TestTwoMirrorsTheFirstHasTheField(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	issueTask(t, ts, fake, model.SyncMirror)
	m, err := fake.OpenIssue("org-a/web", "other", "")
	if err != nil {
		t.Fatal(err)
	}
	link(t, ts, m, model.SyncMirror)
	n0 := len(logOf(t, ts))
	refetch(t, ts)
	if got := taskIn(t, ts, "me", "ME-1").Title; got != "a" {
		t.Errorf("title %s", got)
	}
	if got := len(logOf(t, ts)); got != n0 {
		t.Errorf("%d commits added", got-n0)
	}
}

// The first link keeps its fields while it cannot be read.
func TestAnIssueThatIsGoneKeepsItsFields(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	first := issueTask(t, ts, fake, model.SyncMirror)
	m, err := fake.OpenIssue("org-a/web", "other", "")
	if err != nil {
		t.Fatal(err)
	}
	link(t, ts, m, model.SyncMirror)
	if err := fake.DropIssue("org-a/web", first); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if got := taskIn(t, ts, "me", "ME-1").Title; got != "a" {
		t.Errorf("title %s", got)
	}
}

// Without automations.yaml it is not known which fields sync: neither mode
// writes, and the fetch is done.
func TestAnUnusableFileStopsTheSyncOfIssues(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	m := issueTask(t, ts, fake, model.SyncMirror)
	ts.AddTask("b", func(x *model.Task) { x.Repos = []string{web} })
	p, err := fake.OpenIssue("org-a/web", "b", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := integration.Link(ts.Session, "ME-2", integration.LinkInput{Issue: "org-a/web#" + itoa(p), Sync: model.SyncPush}); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if err := fake.EditIssue("org-a/web", m, "theirs", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, "ME-2", assign(t, "title=mine")); err != nil {
		t.Fatal(err)
	}
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		return tx.PutFile(model.AutomationsPath, []byte("rules: [\n"))
	})
	if w := refetch(t, ts); len(w) == 0 {
		t.Error("no warning")
	}
	if got := taskIn(t, ts, "me", "ME-1").Title; got != "a" {
		t.Errorf("mirrored: %s", got)
	}
	if got := onGitHub(t, fake, p).Title; got != "b" {
		t.Errorf("pushed: %s", got)
	}
}

func TestLinkModeTouchesNothing(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncLink)
	if err := fake.EditIssue("org-a/web", n, "new", ""); err != nil {
		t.Fatal(err)
	}
	if err := fake.CloseIssue("org-a/web", n, "completed"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	x := taskIn(t, ts, "me", "ME-1")
	g, err := x.GitHub()
	if err != nil {
		t.Fatal(err)
	}
	if x.Title != "a" || x.Status != "todo" || g.Issues[0].State != "closed" || g.Issues[0].Synced != nil {
		t.Errorf("task %+v, gh %+v", x, g)
	}
}
