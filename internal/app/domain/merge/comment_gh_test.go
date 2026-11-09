package merge

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// commentGH is a comment with the given updated time, body and gh elements.
func commentGH(updated, body string, gh ...model.CommentLink) []byte {
	c, err := model.DecodeComment(comment(updated, body))
	if err != nil {
		panic(err)
	}
	if err := c.SetGitHub(gh); err != nil {
		panic(err)
	}
	b, err := c.Encode()
	if err != nil {
		panic(err)
	}
	return b
}

func commentOf(t *testing.T, data []byte) (*model.Comment, []model.CommentLink) {
	t.Helper()
	c, err := model.DecodeComment(data)
	if err != nil {
		t.Fatal(err)
	}
	l, err := c.GitHub()
	if err != nil {
		t.Fatal(err)
	}
	return c, l
}

func link(n int, id int64) model.CommentLink {
	return model.CommentLink{Ref: "github.com/o/r/issues/" + string(rune('0'+n)), ID: id, BodySynced: "x"}
}

func TestCommentGHSurvivesAnEditElsewhere(t *testing.T) {
	t.Parallel()
	base := Side{Data: commentGH("2026-10-31T10:00:00Z", "a\n")}
	pushed := Side{Data: commentGH("2026-10-31T10:00:00Z", "a\n", link(1, 11)), Author: "a"} // gh only: updated stays
	edited := Side{Data: commentGH("2026-10-31T12:00:00Z", "b\n"), Author: "b"}
	// mergeSym checks both orders of ours and theirs.
	r := mergeSym(t, commentPath, base, pushed, edited)
	c, l := commentOf(t, r.Data)
	if c.Body != "b\n" || len(l) != 1 || l[0] != link(1, 11) {
		t.Errorf("r = %s", r.Data)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("a push of gh only was reported as overwritten: %v", r.Overwritten)
	}
}

func TestCommentGHOfBothSides(t *testing.T) {
	t.Parallel()
	base := Side{Data: commentGH("2026-10-31T10:00:00Z", "a\n")}
	lose := Side{Data: commentGH("2026-10-31T10:00:00Z", "a\n", link(1, 11)), Author: "a"}
	// Another ref: the winner's element stays and the loser's is added.
	win := Side{Data: commentGH("2026-10-31T12:00:00Z", "b\n", link(2, 22)), Author: "b"}
	r := mergeSym(t, commentPath, base, lose, win)
	if _, l := commentOf(t, r.Data); len(l) != 2 || l[0] != link(2, 22) || l[1] != link(1, 11) {
		t.Errorf("gh = %+v", l)
	}
	// The same ref: the winner's stays, nothing is added.
	win = Side{Data: commentGH("2026-10-31T12:00:00Z", "b\n", link(1, 99)), Author: "b"}
	r = mergeSym(t, commentPath, base, lose, win)
	if _, l := commentOf(t, r.Data); len(l) != 1 || l[0].ID != 99 {
		t.Errorf("same ref: gh = %+v", l)
	}
}

func TestCommentGHAdoptedFromALoserWhoEditedStillOverwrites(t *testing.T) {
	t.Parallel()
	base := Side{Data: commentGH("2026-10-31T10:00:00Z", "a\n")}
	lose := Side{Data: commentGH("2026-10-31T11:00:00Z", "mine\n", link(1, 11)), Author: "a"} // edited and pushed
	win := Side{Data: commentGH("2026-10-31T12:00:00Z", "b\n"), Author: "b"}
	r := mergeSym(t, commentPath, base, lose, win)
	if c, l := commentOf(t, r.Data); c.Body != "b\n" || len(l) != 1 || l[0] != link(1, 11) {
		t.Errorf("r = %s", r.Data)
	}
	if len(r.Overwritten) != 1 || r.Overwritten[0].Author != "a" {
		t.Errorf("overwritten = %v; want the loser's edit recorded", r.Overwritten)
	}
}
