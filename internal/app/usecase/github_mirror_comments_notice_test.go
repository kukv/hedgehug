package usecase_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// A hand edit of a comment that gh ties to GitHub is written over by the
// mirror, and the text it loses is told in the inbox, as for the fields.
func TestMirrorCommentsTellOfAHandEditItWritesOver(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	refetch(t, ts) // pushed
	// The link goes over to mirror, and the comment is edited by hand.
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(x.ULID)
		if err != nil {
			return err
		}
		cur.Watchers = []string{mine}
		cur.Links[len(cur.Links)-1].Sync = model.SyncMirror
		return tx.PutTask(cur)
	})
	editComment(t, ts, x.ULID, commentOn(t, ts, x.ULID, c.ULID), "by hand")
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := commentOn(t, ts, x.ULID, c.ULID); got.Body != "mine" {
		t.Fatalf("body %q", got.Body)
	}
	notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten)
	want := fmt.Sprintf("%s comments:%s %s/issues/%d task [\"by hand\"]", x.ULID, c.ULID, web, n)
	if len(notes) != 1 || notes[0] != want {
		t.Errorf("notes %q, want %q", notes, want)
	}
	_, items, err := notifications.InboxList(ts.Session, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items %+v", items)
	}
	if want := `the comment starting "mine" of the task was written over by github.com/org-a/web#1; the task had "by hand"`; !strings.HasPrefix(items[0].Content, "the comment starting \"mine\" of the task was written over by") || !strings.HasSuffix(items[0].Content, `the task had "by hand"`) {
		t.Errorf("content %q, want like %q", items[0].Content, want)
	}
	// Told once.
	refetch(t, ts)
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); len(notes) != 1 {
		t.Errorf("notes %q", notes)
	}
}

// A change on GitHub alone is no loss of a hand edit.
func TestMirrorCommentsTellNothingOfAChangeOnGitHub(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := commentTask(t, ts, fake)
	c := say(t, ts, x.ULID, mine, "mine")
	refetch(t, ts)
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(x.ULID)
		if err != nil {
			return err
		}
		cur.Links[len(cur.Links)-1].Sync = model.SyncMirror
		return tx.PutTask(cur)
	})
	if err := fake.EditComment("org-a/web", int(oneGH(t, commentOn(t, ts, x.ULID, c.ULID)).ID), "there"); err != nil {
		t.Fatal(err)
	}
	_ = n
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	if got := commentOn(t, ts, x.ULID, c.ULID); got.Body != "there" {
		t.Fatalf("body %q", got.Body)
	}
	if notes := trailers(logOf(t, ts), automation.TrailerIssueOverwritten); notes != nil {
		t.Errorf("notes %q", notes)
	}
}

// The comments the mirror takes in tell the one who ran the sync as well: the
// commit is theirs, and a new comment and a mention are news to them still.
func TestMirrorCommentsTellTheOneWhoSynced(t *testing.T) {
	t.Parallel()
	ts, fake := usecasetest.GitHubSession(t)
	x, n := mirrorTask(t, ts, fake)
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		cur, err := tx.Task(x.ULID)
		if err != nil {
			return err
		}
		cur.Watchers = []string{mine}
		return tx.PutTask(cur)
	})
	if _, err := fake.AddComment("org-a/web", n, "bob", "look, @alice"); err != nil {
		t.Fatal(err)
	}
	if w := refetch(t, ts); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	_, items, err := notifications.InboxList(ts.Session, false)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, it := range items {
		kinds[it.Kind]++
	}
	if len(items) != 1 || kinds["mention"] != 1 {
		t.Errorf("items %+v", items)
	}
}
