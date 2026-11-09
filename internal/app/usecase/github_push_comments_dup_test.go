package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// twice makes the state that two clones of one account leave when they make a
// comment at the same time: two comments on the issue with the mark of c.
func twice(t *testing.T, fake *fakegh.Server, n int, c *model.Comment) (first, second int) {
	t.Helper()
	first, err := fake.AddComment("org-a/web", n, "alice", marked(c.ULID, "mine"))
	if err != nil {
		t.Fatal(err)
	}
	second, err = fake.AddComment("org-a/web", n, "alice", marked(c.ULID, "mine"))
	if err != nil {
		t.Fatal(err)
	}
	return first, second
}

func setGH(t *testing.T, ts *usecasetest.Session, u model.ULID, c *model.Comment, n, id int) {
	t.Helper()
	cur := commentOn(t, ts, u, c.ULID)
	if err := cur.SetGitHub([]model.CommentLink{{Ref: web + "/issues/" + itoa(n), ID: int64(id), BodySynced: "mine"}}); err != nil {
		t.Fatal(err)
	}
	ts.MustEdit("me", "comment", usecasetest.TAuthor, func(tx *usecasetest.Tx) error { return tx.PutComment(u, cur) })
}

// The one gh points to stays, and the other with the mark is deleted.
func TestPushCommentsDeleteTheExtraWithTheMark(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	_, second := twice(t, fake, n, c)
	setGH(t, ts, x.ULID, c, n, second)
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].ID != second {
		t.Errorf("comments %+v, want only %d", got, second)
	}
	if l := oneGH(t, commentOn(t, ts, x.ULID, c.ULID)); l.ID != int64(second) {
		t.Errorf("gh %+v", l)
	}
}

// Without a gh, the first on the list is the one, as when the mirror takes the
// mark back.
func TestPushCommentsKeepTheFirstWithTheMarkWhenGHIsLost(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	first, _ := twice(t, fake, n, c)
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].ID != first {
		t.Errorf("comments %+v, want only %d", got, first)
	}
	if l := oneGH(t, commentOn(t, ts, x.ULID, c.ULID)); l.ID != int64(first) {
		t.Errorf("gh %+v", l)
	}
}

// A comment that is deleted takes the extra with it.
func TestPushCommentsDeleteTheExtraOfADeletedComment(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	first, _ := twice(t, fake, n, c)
	setGH(t, ts, x.ULID, c, n, first)
	deleteComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID))
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 0 {
		t.Errorf("comments %+v", got)
	}
}

// The account of a comment is compared without case, as for who may edit it.
func TestPushCommentsSendsACommentOfMeInAnotherCase(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, "github.com/ALICE", "mine")
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].Body != marked(c.ULID, "mine") {
		t.Errorf("comments %+v", got)
	}
}
