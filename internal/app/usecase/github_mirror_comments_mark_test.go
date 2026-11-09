package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// A comment of GitHub that carries the mark of a comment of the task is that
// comment, whose gh was lost: it gets its gh back and is not taken in again.
func TestMirrorCommentsGiveBackTheLostGH(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "pushed")
	refetch(t, ts)
	ref := web + "/issues/" + itoa(n)
	was := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
	// gh is lost, and the link turns to mirror.
	ts.MustEdit("me", "lose", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur := commentOn(t, ts, x.ULID, c.ULID)
		if err := cur.SetGitHub(nil); err != nil {
			return err
		}
		if err := tx.PutComment(x.ULID, cur); err != nil {
			return err
		}
		task, err := tx.Task(x.ULID)
		if err != nil {
			return err
		}
		task.Links[0].Sync = model.SyncMirror
		return tx.PutTask(task)
	})
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	cs := commentsOf(t, ts, x.ULID)
	if len(cs) != 1 || cs[0].ULID != c.ULID || cs[0].Author != mine || cs[0].DeletedAt != nil {
		t.Fatalf("comments %+v", cs)
	}
	if l := oneGH(t, cs[0]); l.Ref != ref || l.ID != was.ID || l.BodySynced != "pushed" {
		t.Errorf("gh %+v, was %+v", l, was)
	}
	if got := hostComments(t, fake, n); len(got) != 1 {
		t.Errorf("on GitHub %+v", got)
	}
}

// A mark of a comment that this store does not have is a comment of someone
// else's store: it is taken in, without the mark.
func TestMirrorCommentsTakeInAForeignMark(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	if _, err := fake.AddComment("org-a/web", n, "bob", marked(model.NewULID(), "foreign")); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if cs := commentsOf(t, ts, x.ULID); len(cs) != 1 || cs[0].Author != "github:bob" || cs[0].Body != "foreign" {
		t.Errorf("comments %+v", cs)
	}
}
