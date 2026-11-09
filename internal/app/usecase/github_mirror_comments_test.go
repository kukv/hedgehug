package usecase_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

// mirrorTo links the task u to the issue ref in mirror mode with the fields.
func mirrorTo(t *testing.T, ts *usecasetest.Session, u model.ULID, ref string, fields ...string) {
	t.Helper()
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(u)
		if err != nil {
			return err
		}
		cur.Links = append(cur.Links, model.Link{Type: model.LinkIssue, Ref: ref, Sync: model.SyncMirror, Fields: fields})
		return tx.PutTask(cur)
	})
}

// mirrorTask makes ME-1 mirror the comments of a new issue.
func mirrorTask(t *testing.T, ts *usecasetest.Session, fake *fakegh.Server) (*model.Task, int) {
	t.Helper()
	x := ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "mine", "text")
	if err != nil {
		t.Fatal(err)
	}
	mirrorTo(t, ts, x.ULID, web+"/issues/"+itoa(n), "comments")
	return x, n
}

// inOrder is the comments of the task u in the order of their ULIDs.
func inOrder(t *testing.T, ts *usecasetest.Session, u model.ULID) []*model.Comment {
	t.Helper()
	cs := commentsOf(t, ts, u)
	slices.SortFunc(cs, func(a, b *model.Comment) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	return cs
}

func TestMirrorCommentsTakesInEditsAndDeletes(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	id, err := fake.AddComment("org-a/web", n, "bob", "hello\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	cs := inOrder(t, ts, x.ULID)
	if len(cs) != 1 {
		t.Fatalf("comments %+v", cs)
	}
	c, there := cs[0], hostComments(t, fake, n)[0]
	if c.Author != "github:bob" || c.Body != "hello" || !c.Created.Equal(there.CreatedAt) || c.DeletedAt != nil {
		t.Errorf("comment %+v, on GitHub %+v", c, there)
	}
	if want := model.DerivedULID(there.CreatedAt, web+"/issues/"+itoa(n)+"#"+itoa(id)); c.ULID != want {
		t.Errorf("ulid %s, want %s", c.ULID, want)
	}
	if l := oneGH(t, c); l.ID != int64(id) || l.Ref != web+"/issues/"+itoa(n) || l.BodySynced != "hello" {
		t.Errorf("gh %+v", l)
	}
	if op := usecasetest.LastOp(t, ts); op != "gh-mirror" {
		t.Errorf("op %s", op)
	}
	n0 := len(logOf(t, ts))
	refetch(t, ts)
	if c := len(logOf(t, ts)); c != n0 {
		t.Errorf("%d commits added by a read of the same", c-n0)
	}

	if err := fake.EditComment("org-a/web", id, "changed"); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if c := commentOn(t, ts, x.ULID, c.ULID); c.Body != "changed" || oneGH(t, c).BodySynced != "changed" {
		t.Errorf("edited %q, gh %+v", c.Body, ghOf(t, c))
	}
	if notes := trailers(logOf(t, ts), "Hedgehug-Issue-Overwritten"); notes != nil {
		t.Errorf("notes %v", notes)
	}

	if err := fake.RemoveComment("org-a/web", id); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if c := commentOn(t, ts, x.ULID, c.ULID); c.DeletedAt == nil {
		t.Errorf("not deleted: %+v", c)
	}
}

func TestMirrorCommentsAuthorsAndOrder(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	for _, c := range []struct{ author, body string }{{"bob", "one"}, {"dependabot[bot]", "two"}, {"", "three"}} {
		later(ts, time.Minute)
		if _, err := fake.AddComment("org-a/web", n, c.author, c.body); err != nil {
			t.Fatal(err)
		}
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	var got []string
	for _, c := range inOrder(t, ts, x.ULID) {
		got = append(got, c.Author+" "+c.Body)
	}
	if want := []string{"github:bob one", "github:dependabot[bot] two", "github:ghost three"}; !slices.Equal(got, want) {
		t.Errorf("comments %q, want %q", got, want)
	}
}

func TestMirrorCommentsTakesTheMarkOff(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	if _, err := fake.AddComment("org-a/web", n, "bob", marked(model.NewULID(), "from another clone")); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if cs := inOrder(t, ts, x.ULID); len(cs) != 1 || cs[0].Body != "from another clone" || oneGH(t, cs[0]).BodySynced != "from another clone" {
		t.Errorf("comments %+v", cs)
	}
}

// Without comments among the fields nothing is taken in, and the comments are
// not read: 101 of them need a second page, which the fake refuses, so a read
// would be a warning.
func TestMirrorCommentsNotReadWithoutTheField(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x := ts.AddTask("mine", func(x *model.Task) { x.Repos = []string{web} })
	n, err := fake.OpenIssue("org-a/web", "mine", "text")
	if err != nil {
		t.Fatal(err)
	}
	mirrorTo(t, ts, x.ULID, web+"/issues/"+itoa(n), "title", "body")
	for range 101 {
		if _, err := fake.AddComment("org-a/web", n, "bob", "hello"); err != nil {
			t.Fatal(err)
		}
	}
	fake.RefuseComments(fakegh.CommentList, true)
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if cs := commentsOf(t, ts, x.ULID); len(cs) != 0 {
		t.Errorf("comments %+v", cs)
	}
}

func TestMirrorCommentsDoNotWatch(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	was := slices.Clone(taskIn(t, ts, "me", "ME-1").Watchers)
	if _, err := fake.AddComment("org-a/web", n, "bob", "ping @github.com/carol please"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if cs := commentsOf(t, ts, x.ULID); len(cs) != 1 {
		t.Fatalf("comments %+v", cs)
	}
	if got := taskIn(t, ts, "me", "ME-1").Watchers; !slices.Equal(got, was) {
		t.Errorf("watchers %v, were %v", got, was)
	}
}

func TestMirrorCommentsLeaveTheLocalOnes(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	mine1 := say(t, ts, x.ULID, mine, "only here")
	if _, err := fake.AddComment("org-a/web", n, "bob", "there"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	if c := commentOn(t, ts, x.ULID, mine1.ULID); c.DeletedAt != nil || c.Body != "only here" || len(ghOf(t, c)) != 0 {
		t.Errorf("comment %+v", c)
	}
	if len(commentsOf(t, ts, x.ULID)) != 2 {
		t.Errorf("comments %+v", commentsOf(t, ts, x.ULID))
	}
	if got := hostComments(t, fake, n); len(got) != 1 {
		t.Errorf("on GitHub %+v", got)
	}
}

func TestMirrorCommentsKeepWhatAnUnfinishedListCannotSay(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	var ids []int
	for range 102 { // still a second page after one is gone
		id, err := fake.AddComment("org-a/web", n, "bob", "c")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if cs := commentsOf(t, ts, x.ULID); len(cs) != 102 {
		t.Fatalf("%d comments", len(cs))
	}
	if err := fake.RemoveComment("org-a/web", ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := fake.EditComment("org-a/web", ids[1], "edited"); err != nil {
		t.Fatal(err)
	}
	fake.RefuseComments(fakegh.CommentList, true)
	w := refetch(t, ts)
	if len(w) != 1 || !strings.Contains(w[0], "comments") {
		t.Fatalf("warnings %v", w)
	}
	for _, c := range commentsOf(t, ts, x.ULID) {
		if c.DeletedAt != nil || c.Body == "edited" {
			t.Fatalf("changed by a list that broke: %+v", c)
		}
	}
	fake.RefuseComments(fakegh.CommentList, false)
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	gone := 0
	for _, c := range commentsOf(t, ts, x.ULID) {
		if c.DeletedAt != nil {
			gone++
		}
	}
	if gone != 1 {
		t.Errorf("%d deleted", gone)
	}
}

func TestMirrorCommentsTakeTheFirstLinkOnly(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	m, err := fake.OpenIssue("org-a/web", "second", "text")
	if err != nil {
		t.Fatal(err)
	}
	mirrorTo(t, ts, x.ULID, web+"/issues/"+itoa(m), "comments")
	if _, err := fake.AddComment("org-a/web", n, "bob", "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.AddComment("org-a/web", m, "bob", "second"); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	cs := inOrder(t, ts, x.ULID)
	if len(cs) != 1 || cs[0].Body != "first" {
		t.Errorf("comments %+v", cs)
	}
}

func TestMirrorCommentsDoNotTakeWhatWasPushed(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "pushed")
	refetch(t, ts)
	if got := hostComments(t, fake, n); len(got) != 1 {
		t.Fatalf("on GitHub %+v", got)
	}
	ts.MustEdit("me", "mode", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(x.ULID)
		if err != nil {
			return err
		}
		cur.Links[0].Sync = model.SyncMirror
		return tx.PutTask(cur)
	})
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	cs := commentsOf(t, ts, x.ULID)
	if len(cs) != 1 || cs[0].ULID != c.ULID || cs[0].Author != mine {
		t.Errorf("comments %+v", cs)
	}
}

func TestMirroredCommentsAreNotPushedAfterTheModeChanges(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	if _, err := fake.AddComment("org-a/web", n, "bob", "theirs"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	ts.MustEdit("me", "mode", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(x.ULID)
		if err != nil {
			return err
		}
		cur.Links[0].Sync = model.SyncPush
		return tx.PutTask(cur)
	})
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := hostComments(t, fake, n); len(got) != 1 || got[0].Body != "theirs" {
		t.Errorf("on GitHub %+v", got)
	}
}

// Unlinking and linking again start gh over, but not the comments' gh.
func TestUnlinkLeavesTheGHOfTheComments(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	if _, err := fake.AddComment("org-a/web", n, "bob", "theirs"); err != nil {
		t.Fatal(err)
	}
	refetch(t, ts)
	was := ghOf(t, inOrder(t, ts, x.ULID)[0])
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{Issue: "org-a/web#" + itoa(n)}); err != nil {
		t.Fatal(err)
	}
	if got := ghOf(t, inOrder(t, ts, x.ULID)[0]); !slices.Equal(got, was) {
		t.Errorf("gh %+v, was %+v", got, was)
	}
}

// Two clones that take in the same comment make one comment, and no one is told
// of an overwrite.
func TestMirrorCommentsOfTwoClonesMakeOne(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	remote := usecasetest.AddBareRemote(t, ts)
	devStore(t, ts, nil)
	x, n := mirrorTask(t, ts, fake)
	if r, err := replication.SyncOne(ts.Session); err != nil || r.Err != nil {
		t.Fatalf("sync: %+v, %v", r, err)
	}
	other := usecasetest.Clone(t, ts, remote, ts.Now())
	other.DropDefaultAccount()
	other.AppendConfig(hostsRE.FindString(usecasetest.ConfigText(t, ts)) +
		"accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	other.Renew()

	if _, err := fake.AddComment("org-a/web", n, "bob", "hello"); err != nil {
		t.Fatal(err)
	}
	later(ts, 5*time.Minute)
	later(other, 5*time.Minute)
	for _, s := range []*usecasetest.Session{ts, other, ts, other} {
		if r, err := replication.SyncOne(s.Session); err != nil || r.Err != nil || len(r.Warnings) != 0 {
			t.Fatalf("sync: %+v, %v", r, err)
		}
	}
	for _, s := range []*usecasetest.Session{ts, other} {
		cs := commentsOf(t, s, x.ULID)
		if len(cs) != 1 || cs[0].Body != "hello" || len(ghOf(t, cs[0])) != 1 {
			t.Errorf("comments %+v", cs)
		}
		_, items, err := notifications.InboxList(s.Session, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			t.Errorf("inbox item %+v", it)
		}
	}
}
