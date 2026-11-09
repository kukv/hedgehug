package merge

import "testing"

// A push that made the comment again after a 404 changed only gh, with the
// same ref and another id: losing to an edit is no overwrite, and the
// winner's element stays.
func TestCommentGHOfTheSameRefLostWithoutOverwrite(t *testing.T) {
	t.Parallel()
	base := Side{Data: commentGH("2026-10-31T10:00:00Z", "a\n", link(1, 11))}
	pushed := Side{Data: commentGH("2026-10-31T10:00:00Z", "a\n", link(1, 22)), Author: "a"}
	edited := Side{Data: commentGH("2026-10-31T12:00:00Z", "b\n", link(1, 11)), Author: "b"}
	r := mergeSym(t, commentPath, base, pushed, edited)
	c, l := commentOf(t, r.Data)
	if c.Body != "b\n" || len(l) != 1 || l[0] != link(1, 11) {
		t.Errorf("r = %s", r.Data)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v; want none for a change of gh only", r.Overwritten)
	}
}
