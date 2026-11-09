package usecase_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

func onGitHub(t *testing.T, fake *fakegh.Server, n int) *fakegh.Issue {
	t.Helper()
	r, ok := fake.Repo("org-a/web")
	if !ok {
		t.Fatal("no repository")
	}
	for _, x := range r.Issues {
		if x.Number == n {
			return x
		}
	}
	t.Fatalf("no issue %d", n)
	return nil
}

func set(t *testing.T, ts *usecasetest.Session, s string) {
	t.Helper()
	if _, err := tasks.Set(ts.Session, "ME-1", assign(t, s)); err != nil {
		t.Fatal(err)
	}
}

func TestPushWritesTheTaskToTheIssue(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	if err := fake.AddLabel("org-a/web", "bug"); err != nil {
		t.Fatal(err)
	}
	x := ts.AddTask("mine", func(x *model.Task) {
		x.Repos, x.Body, x.Labels, x.Assignees = []string{web}, "my text\n", []string{"bug"}, []string{"github.com/alice"}
	})
	n, err := fake.OpenIssue("org-a/web", "theirs", "their text")
	if err != nil {
		t.Fatal(err)
	}
	link(t, ts, n, model.SyncPush)
	got := onGitHub(t, fake, n)
	if got.Title != "mine" || got.Body != "my text\n\nhedgehug:"+x.ULID.String() {
		t.Errorf("issue %+v", got)
	}
	if len(got.Labels) != 0 || !slices.Equal(got.Assignees, []string{"alice"}) {
		t.Errorf("a label is not sent: %+v", got)
	}
	es := logOf(t, ts)
	if op, _ := es[0].Trailer("Hedgehug-Op"); op != "gh-push" {
		t.Errorf("op %s", op)
	}
	if !strings.HasPrefix(es[0].Author.Name, "hedgehug-automation") {
		t.Errorf("author %+v", es[0].Author)
	}
	if notes := trailers(es, automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("a first push tells nothing: %v", notes)
	}
	// The mark is not a difference.
	n0, at := len(es), got.UpdatedAt
	refetch(t, ts)
	if c := len(logOf(t, ts)); c != n0 {
		t.Errorf("%d commits added", c-n0)
	}
	if onGitHub(t, fake, n).UpdatedAt != at {
		t.Error("the issue was written again")
	}
}

// GitHub's web editor answers in CRLF.
func TestPushDoesNotSeeLineEndings(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x := ts.AddTask("a", func(x *model.Task) { x.Repos, x.Body = []string{web}, "one\ntwo" })
	n, err := fake.OpenIssue("org-a/web", "a", "x")
	if err != nil {
		t.Fatal(err)
	}
	link(t, ts, n, model.SyncPush)
	if err := fake.EditIssue("org-a/web", n, "", "one\r\ntwo\r\n\r\nhedgehug:"+x.ULID.String()+"\r\n"); err != nil {
		t.Fatal(err)
	}
	n0 := len(logOf(t, ts))
	refetch(t, ts)
	if c := len(logOf(t, ts)); c != n0 {
		t.Errorf("%d commits added", c-n0)
	}
}

// GitHub trims a title.
func TestPushDoesNotSeeWhiteSpaceAroundTheTitle(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	ts.AddTask("a ", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "a", "")
	if err != nil {
		t.Fatal(err)
	}
	link(t, ts, n, model.SyncPush)
	at := onGitHub(t, fake, n).UpdatedAt
	refetch(t, ts)
	if onGitHub(t, fake, n).UpdatedAt != at {
		t.Error("the issue was written again")
	}
}

func TestPushFollowsTheTask(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncPush)
	set(t, ts, "title=new")
	refetch(t, ts)
	if got := onGitHub(t, fake, n).Title; got != "new" {
		t.Errorf("title %s", got)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("notes %v", notes)
	}
}

func TestPushPutsAHandEditBack(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncPush)
	if err := fake.EditIssue("org-a/web", n, "theirs", ""); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if got := onGitHub(t, fake, n).Title; got != "a" {
		t.Errorf("title %s", got)
	}
	notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten)
	if len(notes) != 1 {
		t.Fatalf("notes %v", notes)
	}
	_, _, o, ok := automation.ParseIssueOverwritten(notes[0])
	if !ok || o.Field != "title" || o.Side != automation.SideGitHub || !slices.Equal(o.Lost, []string{"theirs"}) {
		t.Errorf("note %q", notes[0])
	}
}

func TestPushState(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncPush)
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "done"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if x := onGitHub(t, fake, n); x.State != "closed" || x.StateReason != "completed" {
		t.Errorf("issue %+v", x)
	}
	g, err := taskIn(t, ts, "me", "ME-1").GitHub()
	if err != nil {
		t.Fatal(err)
	}
	if g.Issues[0].State != "closed" {
		t.Errorf("gh %+v", g.Issues[0])
	}
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "canceled"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if x := onGitHub(t, fake, n); x.State != "closed" || x.StateReason != "not_planned" {
		t.Errorf("issue %+v", x)
	}
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "todo"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if x := onGitHub(t, fake, n); x.State != "open" {
		t.Errorf("issue %+v", x)
	}
	// In progress is open too.
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "in_progress"); err != nil {
		t.Fatal(err)
	}
	n0 := len(logOf(t, ts))
	refetch(t, ts)
	if c := len(logOf(t, ts)); c != n0 {
		t.Errorf("%d commits added", c-n0)
	}
}

// What the rules did goes out in the same sync.
func TestPushSendsWhatTheRulesDid(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, nil)
	n := issueTask(t, ts, fake, model.SyncPush)
	p, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false)
	if err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if err := fake.MergePull("org-a/web", p); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	wantStatus(t, ts, "ME-1", "done")
	if x := onGitHub(t, fake, n); x.State != "closed" {
		t.Errorf("issue %+v", x)
	}
}

func TestPushLeavesOutWhatGitHubDoesNotKnow(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncPush)
	set(t, ts, "assignees+=github.com/nobody")
	set(t, ts, "assignees+=ghe.example.com/zoe")
	w := strings.Join(refetch(t, ts), "\n")
	for _, s := range []string{`assignee "nobody"`, "ghe.example.com/zoe"} {
		if !strings.Contains(w, s) {
			t.Errorf("no warning of %s: %s", s, w)
		}
	}
	if x := onGitHub(t, fake, n); len(x.Assignees) != 0 {
		t.Errorf("issue %+v", x)
	}
	// It is told again, and is not an edit of GitHub.
	n0 := len(logOf(t, ts))
	if w := refetch(t, ts); len(w) == 0 {
		t.Error("no warning the second time")
	}
	if c := len(logOf(t, ts)); c != n0 {
		t.Errorf("%d commits added", c-n0)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("notes %v", notes)
	}
}

// A link that still names labels in its fields sends nothing of them.
func TestPushDoesNotSendTheLabelsOfALink(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	if err := fake.AddLabel("org-a/web", "bug"); err != nil {
		t.Fatal(err)
	}
	n := issueTask(t, ts, fake, model.SyncPush)
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(taskIn(t, ts, "me", "ME-1").ULID)
		if err != nil {
			return err
		}
		cur.Labels = []string{"bug"}
		cur.Links[0].Fields = []string{"labels", "title"}
		return tx.PutTask(cur)
	})
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings: %v", w)
	}
	if x := onGitHub(t, fake, n); x.Title != "a" || len(x.Labels) != 0 {
		t.Errorf("issue %+v", x)
	}
}

func TestPushFailureIsTriedAgain(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncPush)
	set(t, ts, "title=new")
	if err := fake.SetPerm("org-a/web", "alice", "read"); err != nil {
		t.Fatal(err)
	}
	w := refetch(t, ts)
	if len(w) != 1 || !strings.Contains(w[0], "org-a/web#") {
		t.Fatalf("warnings %v", w)
	}
	if v := syncedOf(t, ts); !slices.Equal(v["title"], []string{"a"}) {
		t.Errorf("base %v", v)
	}
	if err := fake.SetPerm("org-a/web", "alice", "write"); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := onGitHub(t, fake, n).Title; got != "new" {
		t.Errorf("title %s", got)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("notes %v", notes)
	}
}

func TestPushOnlyTheFieldsOfTheLink(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x := ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web}; x.Body = "mine" })
	n, err := fake.OpenIssue("org-a/web", "theirs", "their text")
	if err != nil {
		t.Fatal(err)
	}
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(x.ULID)
		if err != nil {
			return err
		}
		cur.Links = []model.Link{{Type: model.LinkIssue, Ref: web + "/issues/" + itoa(n), Sync: model.SyncPush, Fields: []string{"title"}}}
		return tx.PutTask(cur)
	})
	refetch(t, ts)
	if got := onGitHub(t, fake, n); got.Title != "mine" || got.Body != "their text" {
		t.Errorf("issue %+v", got)
	}
}

func TestPushMilestone(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, func(sc *schema.Schema, f *automation.File) {
		sc.Props = append(sc.Props, schema.Prop{Name: "target", Type: schema.TypeText})
		f.IssueFields.Push = append(f.IssueFields.Push, "milestone")
		f.IssueFields.Props = map[string]string{"milestone": "target"}
		f.Rules = nil
	})
	if err := fake.AddMilestone("org-a/web", "v1"); err != nil {
		t.Fatal(err)
	}
	n := issueTask(t, ts, fake, model.SyncPush)
	set(t, ts, "target=v1")
	refetch(t, ts)
	if got := onGitHub(t, fake, n).Milestone; got != "v1" {
		t.Errorf("milestone %q", got)
	}
	set(t, ts, "target=v9")
	if w := refetch(t, ts); len(w) != 1 || !strings.Contains(w[0], `milestone "v9"`) {
		t.Fatalf("warnings %v", w)
	}
	if got := onGitHub(t, fake, n).Milestone; got != "v1" {
		t.Errorf("milestone %q", got)
	}
}

// Off by default: a milestone in issue_fields.push does nothing without props.
func TestMilestoneIsOffByDefault(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	devStore(t, ts, func(_ *schema.Schema, f *automation.File) {
		f.IssueFields.Push = append(f.IssueFields.Push, "milestone")
		f.Rules = nil
	})
	n := issueTask(t, ts, fake, model.SyncPush)
	if err := fake.TagIssue("org-a/web", n, nil, nil, "v1"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if got := onGitHub(t, fake, n).Milestone; got != "v1" {
		t.Errorf("milestone %q", got)
	}
}

func TestPushLeavesTheIssueOfADeletedTask(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncPush)
	set(t, ts, "title=new")
	if _, err := tasks.Remove(ts.Session, "ME-1"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if got := onGitHub(t, fake, n).Title; got != "a" {
		t.Errorf("title %s", got)
	}
}

func TestAnIssueThatIsGoneIsAWarning(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	n := issueTask(t, ts, fake, model.SyncPush)
	st := fake.DropIssue("org-a/web", n)
	if st != nil {
		t.Fatal(st)
	}
	w := refetch(t, ts)
	if len(w) != 1 || !strings.Contains(w[0], "is not on GitHub") {
		t.Fatalf("warnings %v", w)
	}
	if l := taskIn(t, ts, "me", "ME-1").Links; len(l) != 1 || !l[0].Live() {
		t.Errorf("links %+v", l)
	}
}
