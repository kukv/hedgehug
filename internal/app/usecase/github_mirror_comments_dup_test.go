package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// loseGH takes the gh off the comment c of the task u (or points it at id when
// id is not 0) and turns the link of the task to mirror.
func loseGH(t *testing.T, ts *usecasetest.Session, u model.ULID, c *model.Comment, ref string, id int64) {
	t.Helper()
	ts.MustEdit("me", "lose", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur := commentOn(t, ts, u, c.ULID)
		var links []model.CommentLink
		if id != 0 {
			links = []model.CommentLink{{Ref: ref, ID: id, BodySynced: "pushed"}}
		}
		if err := cur.SetGitHub(links); err != nil {
			return err
		}
		if err := tx.PutComment(u, cur); err != nil {
			return err
		}
		task, err := tx.Task(u)
		if err != nil {
			return err
		}
		task.Links[0].Sync = model.SyncMirror
		return tx.PutTask(task)
	})
}

// GitHub has two comments with the mark of one comment of the task: the one
// whose ID gh holds is it, and without one the first is; the other is not taken
// in as a comment.
func TestMirrorCommentsTwoWithOneMarkAreOneComment(t *testing.T) {
	t.Parallel()
	for _, held := range []string{"second", "none"} {
		ts, fake := usecasetest.GitHubSession(t)
		x, n := commentTask(t, ts, fake)
		c := say(t, ts, x.ULID, mine, "pushed")
		refetch(t, ts)
		first := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
		second, err := fake.AddComment("org-a/web", n, "alice", marked(c.ULID, "pushed"))
		if err != nil {
			t.Fatal(err)
		}
		want := first.ID
		var id int64
		if held == "second" {
			id, want = int64(second), int64(second)
		}
		loseGH(t, ts, x.ULID, c, web+"/issues/"+itoa(n), id)
		if w := refetch(t, ts); len(w) != 0 {
			t.Fatalf("%s: warnings %v", held, w)
		}
		cs := commentsOf(t, ts, x.ULID)
		if len(cs) != 1 || cs[0].ULID != c.ULID || cs[0].DeletedAt != nil {
			t.Fatalf("%s: comments %+v", held, cs)
		}
		if l := oneGH(t, cs[0]); l.ID != want {
			t.Errorf("%s: gh %+v, want %d", held, l, want)
		}
	}
}

// The body GitHub has is the comment's when gh is given back.
func TestMirrorCommentsGiveBackTheLostGHWithTheBody(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "pushed")
	refetch(t, ts)
	was := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
	if err := fake.EditComment("org-a/web", int(was.ID), marked(c.ULID, "edited there")); err != nil {
		t.Fatal(err)
	}
	loseGH(t, ts, x.ULID, c, web+"/issues/"+itoa(n), 0)
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	cur := commentOn(t, ts, x.ULID, c.ULID)
	if l := oneGH(t, cur); cur.Body != "edited there" || l.BodySynced != "edited there" || l.ID != was.ID {
		t.Errorf("body %q, gh %+v", cur.Body, l)
	}
}
