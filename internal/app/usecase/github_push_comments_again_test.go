package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// Another clone made the comment again while this one was updating it: the
// 404 is followed by a search by the mark, and nothing is made twice.
func TestPushCommentsFindsTheOneMadeAgainWhenTheUpdateFindsItGone(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "A")
	refetch(t, ts)
	was := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
	editComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID), "B")
	fake.CommentGoneOnUpdate(marked(c.ULID, "A"))
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	got := hostComments(t, fake, n)
	if len(got) != 1 || int64(got[0].ID) == was.ID {
		t.Fatalf("made twice, or not found: %+v", got)
	}
	l := oneGH(t, commentOn(t, ts, x.ULID, c.ULID))
	if l.ID != int64(got[0].ID) {
		t.Errorf("gh %+v, comment %d", l, got[0].ID)
	}
	// The text of the task goes to it on the next sync.
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].Body != marked(c.ULID, "B") {
		t.Errorf("comments %+v", got)
	}
}
