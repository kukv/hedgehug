package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// whenWriting runs fn at the write of the push: after the push has talked to
// GitHub, and before it takes the store. The first write of a sync is the
// fetch's.
func whenWriting(ts *usecasetest.Session, fn func()) {
	calls := 0
	ts.BeforeUpdate = func() {
		if calls++; calls == 2 {
			fn()
		}
	}
}

func TestPushCommentsKeepsAnEditMadeWhileItTalks(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "A")
	refetch(t, ts)
	editComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID), "B")
	ran := false
	whenWriting(ts, func() {
		ran = true
		editComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID), "C")
	})
	if w := refetch(t, ts); len(w) != 0 || !ran {
		t.Fatalf("warnings %v, ran %v", w, ran)
	}
	ts.BeforeUpdate = nil
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].Body != marked(c.ULID, "B") {
		t.Fatalf("comments %+v", got)
	}
	cur := commentOn(t, ts, x.ULID, c.ULID)
	if cur.Body != "C" || oneGH(t, cur).BodySynced != "B" {
		t.Errorf("body %q, gh %+v", cur.Body, ghOf(t, cur))
	}
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].Body != marked(c.ULID, "C") {
		t.Errorf("comments %+v", got)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("a wrong note: %v", notes)
	}
}

func TestPushCommentsDeletedWhileItCreatesIsDeletedNext(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x := ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "other", "text")
	if err != nil {
		t.Fatal(err)
	}
	pushTo(t, ts, x.ULID, web+"/issues/"+itoa(n), "title", "comments")
	c := say(t, ts, x.ULID, mine, "mine")
	whenWriting(ts, func() { deleteComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID)) })
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	ts.BeforeUpdate = nil
	if got := hostComments(t, fake, n); len(got) != 1 {
		t.Fatalf("comments %+v", got)
	}
	cur := commentOn(t, ts, x.ULID, c.ULID)
	if cur.DeletedAt == nil || len(ghOf(t, cur)) != 1 {
		t.Errorf("deleted %v, gh %+v", cur.DeletedAt, ghOf(t, cur))
	}
	if got := syncedOf(t, ts)["title"]; len(got) != 1 || got[0] != "mine" {
		t.Errorf("the issue's gh was lost with it: %v", got)
	}
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 0 {
		t.Errorf("left on GitHub: %+v", got)
	}
}

func TestPushCommentsCreatesAgainWhenTheUpdateFindsItGone(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "A")
	refetch(t, ts)
	was := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
	editComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID), "B")
	fake.CommentGoneOnUpdate()
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	got := hostComments(t, fake, n)
	if len(got) != 1 || got[0].Body != marked(c.ULID, "B") || int64(got[0].ID) == was.ID {
		t.Fatalf("comments %+v", got)
	}
	if l := oneGH(t, commentOn(t, ts, x.ULID, c.ULID)); l.ID != int64(got[0].ID) || l.BodySynced != "B" {
		t.Errorf("gh %+v", l)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("notes %v", notes)
	}
}
