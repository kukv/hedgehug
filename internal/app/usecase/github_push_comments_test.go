package usecase_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// mine is the account of the session: the author of the comments that a push
// sends.
const mine = "github.com/alice"

func marked(u model.ULID, body string) string {
	return body + "\n\n<!-- hedgehug:" + u.String() + " -->"
}

// pushTo links the task u to the issue n of org-a/web in push mode with the
// fields.
func pushTo(t *testing.T, ts *usecasetest.Session, u model.ULID, ref string, fields ...string) {
	t.Helper()
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(u)
		if err != nil {
			return err
		}
		cur.Links = append(cur.Links, model.Link{Type: model.LinkIssue, Ref: ref, Sync: model.SyncPush, Fields: fields})
		return tx.PutTask(cur)
	})
}

// say writes a comment of author on the task u.
func say(t *testing.T, ts *usecasetest.Session, u model.ULID, author, body string) *model.Comment {
	t.Helper()
	c := &model.Comment{ULID: model.NewULID(), Author: author, Body: body}
	ts.MustEdit("me", "comment", usecasetest.TAuthor, func(tx *usecasetest.Tx) error { return tx.PutComment(u, c) })
	return commentOn(t, ts, u, c.ULID)
}

func commentOn(t *testing.T, ts *usecasetest.Session, u, c model.ULID) *model.Comment {
	t.Helper()
	for _, x := range commentsOf(t, ts, u) {
		if x.ULID == c {
			return x
		}
	}
	t.Fatalf("no comment %s", c)
	return nil
}

func commentsOf(t *testing.T, ts *usecasetest.Session, u model.ULID) []*model.Comment {
	t.Helper()
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	var out []*model.Comment
	for c, err := range s.Comments(u) {
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

// editComment changes the body of c in the store.
func editComment(t *testing.T, ts *usecasetest.Session, u model.ULID, c *model.Comment, body string) {
	t.Helper()
	ts.MustEdit("me", "comment", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		c.Body = body
		return tx.PutComment(u, c)
	})
}

func deleteComment(t *testing.T, ts *usecasetest.Session, u model.ULID, c *model.Comment) {
	t.Helper()
	ts.MustEdit("me", "comment", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		at := ts.Now()
		c.DeletedAt = &at
		return tx.PutComment(u, c)
	})
}

func ghOf(t *testing.T, c *model.Comment) []model.CommentLink {
	t.Helper()
	l, err := c.GitHub()
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func oneGH(t *testing.T, c *model.Comment) model.CommentLink {
	t.Helper()
	l := ghOf(t, c)
	if len(l) != 1 {
		t.Fatalf("gh %+v", l)
	}
	return l[0]
}

// commentTask makes ME-1 push the comments to a new issue.
func commentTask(t *testing.T, ts *usecasetest.Session, fake *fakegh.Server) (*model.Task, int) {
	t.Helper()
	x := ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "mine", "text")
	if err != nil {
		t.Fatal(err)
	}
	pushTo(t, ts, x.ULID, web+"/issues/"+itoa(n), "comments")
	return x, n
}

func hostComments(t *testing.T, fake *fakegh.Server, n int) []*fakegh.Comment {
	t.Helper()
	return onGitHub(t, fake, n).Comments
}

func TestPushCommentsCreatesEditsAndDeletes(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "hello\r\n")
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	got := hostComments(t, fake, n)
	if len(got) != 1 || got[0].Body != marked(c.ULID, "hello") || got[0].Author != "alice" {
		t.Fatalf("comments %+v", got)
	}
	if op := usecasetest.LastOp(t, ts); op != "gh-push" {
		t.Errorf("op %s", op)
	}
	after := commentOn(t, ts, x.ULID, c.ULID)
	l := ghOf(t, after)
	if len(l) != 1 || l[0].Ref != web+"/issues/"+itoa(n) || l[0].ID != int64(got[0].ID) || l[0].BodySynced != "hello" {
		t.Errorf("gh %+v", l)
	}
	if !after.Updated.Equal(c.Updated) {
		t.Errorf("updated moved: %v -> %v", c.Updated, after.Updated)
	}
	// Nothing changed: nothing is written.
	n0, at := len(logOf(t, ts)), got[0].UpdatedAt
	refetch(t, ts)
	if k := len(logOf(t, ts)); k != n0 || !hostComments(t, fake, n)[0].UpdatedAt.Equal(at) {
		t.Errorf("a second sync wrote: %d commits", k-n0)
	}

	// An edit goes to the comment.
	editComment(t, ts, x.ULID, after, "hello again")
	refetch(t, ts)
	got = hostComments(t, fake, n)
	if len(got) != 1 || got[0].Body != marked(c.ULID, "hello again") {
		t.Fatalf("after the edit: %+v", got)
	}
	if l := oneGH(t, commentOn(t, ts, x.ULID, c.ULID)); l.BodySynced != "hello again" {
		t.Errorf("gh %+v", l)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("an edit of ours tells nothing: %v", notes)
	}

	// A delete deletes it there, and gh goes.
	deleteComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID))
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 0 {
		t.Errorf("not deleted: %+v", got)
	}
	if l := ghOf(t, commentOn(t, ts, x.ULID, c.ULID)); len(l) != 0 {
		t.Errorf("gh %+v", l)
	}
}

func TestPushCommentsOverwritesAHandEdit(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	refetch(t, ts)
	id := oneGH(t, commentOn(t, ts, x.ULID, c.ULID)).ID
	if err := fake.EditComment("org-a/web", int(id), "by hand"); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].Body != marked(c.ULID, "mine") {
		t.Fatalf("not written over: %+v", got)
	}
	notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten)
	want := fmt.Sprintf("%s comments:%s %s/issues/%d github [\"by hand\"]", x.ULID, c.ULID, web, n)
	if len(notes) != 1 || notes[0] != want {
		t.Errorf("notes %q, want %q", notes, want)
	}
	if got := oneGH(t, commentOn(t, ts, x.ULID, c.ULID)); got.BodySynced != "mine" {
		t.Errorf("gh %+v", got)
	}
	// Told once.
	refetch(t, ts)
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); len(notes) != 1 {
		t.Errorf("notes %q", notes)
	}
}

func TestPushCommentsSendsOnlyMyComments(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	say(t, ts, x.ULID, "github.com/bob", "bob's")
	say(t, ts, x.ULID, "", "nobody's")
	say(t, ts, x.ULID, "github:carol", "from GitHub")
	gone := say(t, ts, x.ULID, mine, "deleted before")
	deleteComment(t, ts, x.ULID, gone)
	c := say(t, ts, x.ULID, mine, "mine")
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	got := hostComments(t, fake, n)
	if len(got) != 1 || got[0].Body != marked(c.ULID, "mine") {
		t.Errorf("comments %+v", got)
	}
	for _, x := range commentsOf(t, ts, x.ULID) {
		if x.ULID != c.ULID && len(ghOf(t, x)) != 0 {
			t.Errorf("gh on %s: %+v", x.Author, ghOf(t, x))
		}
	}
}

func TestPushCommentsNeedsTheField(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x := ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "mine", "text")
	if err != nil {
		t.Fatal(err)
	}
	pushTo(t, ts, x.ULID, web+"/issues/"+itoa(n), "title")
	c := say(t, ts, x.ULID, mine, "mine")
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 0 {
		t.Errorf("sent without the field: %+v", got)
	}
	if l := ghOf(t, commentOn(t, ts, x.ULID, c.ULID)); len(l) != 0 {
		t.Errorf("gh %+v", l)
	}
}

func TestPushCommentsCreatesInOrderAtMost30AcrossRepositories(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.FakeSession(t, fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}},
			{Owner: "org-a", Name: "api", Perms: map[string]string{"alice": "write"}},
		},
	}, "")
	ts.AppendConfig("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	ts.Renew()
	fake.SetNow(func() time.Time { return ts.Now() })
	var tasksOf []*model.Task
	var issues []int
	for _, repo := range []string{"web", "api"} {
		x := ts.AddTask(repo, func(x *model.Task) { x.Repos = []string{"github.com/org-a/" + repo} })
		n, err := fake.OpenIssue("org-a/"+repo, repo, "text")
		if err != nil {
			t.Fatal(err)
		}
		pushTo(t, ts, x.ULID, "github.com/org-a/"+repo+"/issues/"+itoa(n), "comments")
		for i := range 20 {
			say(t, ts, x.ULID, mine, fmt.Sprintf("%s %02d", repo, i))
		}
		tasksOf, issues = append(tasksOf, x), append(issues, n)
	}
	count := func() (total int, per []int) {
		for i, repo := range []string{"org-a/web", "org-a/api"} {
			r, _ := fake.Repo(repo)
			k := 0
			for _, x := range r.Issues {
				if x.Number == issues[i] {
					k = len(x.Comments)
				}
			}
			per = append(per, k)
			total += k
		}
		return
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if total, per := count(); total != 30 {
		t.Fatalf("first sync created %d (%v)", total, per)
	}
	// ULID order inside a repository: the first ones got there.
	for i, x := range tasksOf {
		repo := []string{"org-a/web", "org-a/api"}[i]
		r, _ := fake.Repo(repo)
		var bodies []string
		for _, is := range r.Issues {
			if is.Number == issues[i] {
				for _, c := range is.Comments {
					bodies = append(bodies, strings.SplitN(c.Body, "\n", 2)[0])
				}
			}
		}
		if !slices.IsSorted(bodies) {
			t.Errorf("%s not in order: %v", x.Title, bodies)
		}
	}
	refetch(t, ts)
	if total, _ := count(); total != 40 {
		t.Fatalf("second sync: %d", total)
	}
}

func TestPushCommentsFindsAgainByTheMark(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	refetch(t, ts)
	was := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
	// gh is lost, as when this side lost a merge.
	cur := commentOn(t, ts, x.ULID, c.ULID)
	if err := cur.SetGitHub(nil); err != nil {
		t.Fatal(err)
	}
	ts.MustEdit("me", "comment", usecasetest.TAuthor, func(tx *usecasetest.Tx) error { return tx.PutComment(x.ULID, cur) })
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 1 {
		t.Fatalf("duplicated: %+v", got)
	}
	if got := ghOf(t, commentOn(t, ts, x.ULID, c.ULID)); len(got) != 1 || got[0].ID != was.ID || got[0].BodySynced != "mine" {
		t.Errorf("gh %+v, was %+v", got, was)
	}
}

func TestPushCommentsRewritesOneWithNoMark(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "same")
	id, err := fake.AddComment("org-a/web", n, "alice", "same")
	if err != nil {
		t.Fatal(err)
	}
	cur := commentOn(t, ts, x.ULID, c.ULID)
	if err := cur.SetGitHub([]model.CommentLink{{Ref: web + "/issues/" + itoa(n), ID: int64(id), BodySynced: "same"}}); err != nil {
		t.Fatal(err)
	}
	ts.MustEdit("me", "comment", usecasetest.TAuthor, func(tx *usecasetest.Tx) error { return tx.PutComment(x.ULID, cur) })
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	got := hostComments(t, fake, n)
	if len(got) != 1 || got[0].ID != id || got[0].Body != marked(c.ULID, "same") {
		t.Errorf("comments %+v", got)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("the same text is no loss: %v", notes)
	}
}

func TestPushCommentsCreatesAgainWhenGitHubLostIt(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	refetch(t, ts)
	was := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
	if _, err := fake.AddComment("org-a/web", n, "bob", "other"); err != nil { // so that the fake does not number it again
		t.Fatal(err)
	}
	if err := fake.RemoveComment("org-a/web", int(was.ID)); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	got := hostComments(t, fake, n)
	if len(got) != 2 || got[1].Body != marked(c.ULID, "mine") {
		t.Fatalf("comments %+v", got)
	}
	l := ghOf(t, commentOn(t, ts, x.ULID, c.ULID))
	if len(l) != 1 || l[0].ID != int64(got[1].ID) || l[0].ID == was.ID {
		t.Errorf("gh %+v, was %+v", l, was)
	}
}

func TestPushCommentsDeleteOfAGoneCommentIsDone(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	refetch(t, ts)
	id := oneGH(t, commentOn(t, ts, x.ULID, c.ULID)).ID
	if err := fake.RemoveComment("org-a/web", int(id)); err != nil {
		t.Fatal(err)
	}
	deleteComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID))
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if l := ghOf(t, commentOn(t, ts, x.ULID, c.ULID)); len(l) != 0 {
		t.Errorf("gh %+v", l)
	}
	if got := hostComments(t, fake, n); len(got) != 0 {
		t.Errorf("comments %+v", got)
	}
}

func TestPushCommentsToEachLink(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x := ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web} })
	var issues []int
	for range 2 {
		n, err := fake.OpenIssue("org-a/web", "i", "text")
		if err != nil {
			t.Fatal(err)
		}
		pushTo(t, ts, x.ULID, web+"/issues/"+itoa(n), "comments")
		issues = append(issues, n)
	}
	c := say(t, ts, x.ULID, mine, "mine")
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	l := ghOf(t, commentOn(t, ts, x.ULID, c.ULID))
	if len(l) != 2 {
		t.Fatalf("gh %+v", l)
	}
	for i, n := range issues {
		got := hostComments(t, fake, n)
		if len(got) != 1 || l[i].Ref != web+"/issues/"+itoa(n) || l[i].ID != int64(got[0].ID) {
			t.Errorf("issue %d: %+v, gh %+v", n, got, l)
		}
	}
	// A comment that has the ID of the issue is the same comment: nothing is
	// created again.
	refetch(t, ts)
	for _, n := range issues {
		if got := hostComments(t, fake, n); len(got) != 1 {
			t.Errorf("issue %d: %+v", n, got)
		}
	}
}

func TestPushCommentsFailureIsAWarningAndTriedAgain(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	fake.RefuseComments(fakegh.CommentCreate, true)
	w := refetch(t, ts)
	if len(w) != 1 || !strings.Contains(w[0], "alice") {
		t.Fatalf("warnings %v", w)
	}
	if l := ghOf(t, commentOn(t, ts, x.ULID, c.ULID)); len(l) != 0 {
		t.Errorf("gh %+v", l)
	}
	fake.RefuseComments(fakegh.CommentCreate, false)
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 1 {
		t.Errorf("comments %+v", got)
	}

	// An update that fails leaves the difference for the next sync.
	editComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID), "second")
	fake.RefuseComments(fakegh.CommentUpdate, true)
	if w := refetch(t, ts); len(w) != 1 {
		t.Fatalf("warnings %v", w)
	}
	fake.RefuseComments(fakegh.CommentUpdate, false)
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].Body != marked(c.ULID, "second") {
		t.Errorf("comments %+v", got)
	}

	// So does a delete.
	deleteComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID))
	fake.RefuseComments(fakegh.CommentDelete, true)
	if w := refetch(t, ts); len(w) != 1 {
		t.Fatalf("warnings %v", w)
	}
	if l := ghOf(t, commentOn(t, ts, x.ULID, c.ULID)); len(l) != 1 {
		t.Errorf("gh went with a delete that failed: %+v", l)
	}
	fake.RefuseComments(fakegh.CommentDelete, false)
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 0 {
		t.Errorf("comments %+v", got)
	}
}

func TestPushCommentsWaitsWhenTheListBreaks(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	for i := range 101 { // a second page
		if _, err := fake.AddComment("org-a/web", n, "bob", fmt.Sprint("c", i)); err != nil {
			t.Fatal(err)
		}
	}
	c := say(t, ts, x.ULID, mine, "mine")
	fake.RefuseComments(fakegh.CommentList, true)
	if w := refetch(t, ts); len(w) != 1 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 101 {
		t.Errorf("created with no list: %d", len(got))
	}
	fake.RefuseComments(fakegh.CommentList, false)
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 102 || got[101].Body != marked(c.ULID, "mine") {
		t.Errorf("comments: %d", len(got))
	}
}
