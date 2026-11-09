package github_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/github"
)

func commentFake(t *testing.T) (*fakegh.Server, *github.Client, int) {
	t.Helper()
	fake, c := issueFake(t)
	n, err := c.CreateIssue("alice", "web", github.NewIssue{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	return fake, c, n
}

func TestCommentsCreateUpdateDelete(t *testing.T) {
	t.Parallel()
	fake, c, n := commentFake(t)
	made, err := c.CreateIssueComment("alice", "web", n, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if made.ID == 0 || made.Body != "hello" || made.User == nil || made.User.Login != "alice" || made.CreatedAt.IsZero() || made.UpdatedAt.IsZero() {
		t.Fatalf("created = %+v", made)
	}
	if issueOf(t, fake, n).Comments[0].Body != "hello" {
		t.Errorf("issue = %+v", issueOf(t, fake, n))
	}
	up, err := c.UpdateIssueComment("alice", "web", made.ID, "changed")
	if err != nil || up.ID != made.ID || up.Body != "changed" {
		t.Fatalf("updated = %+v, %v", up, err)
	}
	got, err := c.IssueComments("alice", "web", n)
	if err != nil || len(got) != 1 || got[0].Body != "changed" {
		t.Fatalf("listed = %+v, %v", got, err)
	}
	if err := c.DeleteIssueComment("alice", "web", made.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.IssueComments("alice", "web", n); len(got) != 0 {
		t.Errorf("listed after delete = %+v", got)
	}
	if err := c.DeleteIssueComment("alice", "web", made.ID); !errors.Is(err, github.ErrNotFound) {
		t.Errorf("delete twice = %v, want a 404", err)
	}
	if _, err := c.UpdateIssueComment("alice", "web", made.ID, "x"); !errors.Is(err, github.ErrNotFound) {
		t.Errorf("update of a gone comment = %v, want a 404", err)
	}
	if _, err := c.IssueComments("alice", "web", 99); !errors.Is(err, github.ErrNotFound) {
		t.Errorf("list of a missing issue = %v, want a 404", err)
	}
}

func TestCommentsReadEveryPage(t *testing.T) {
	t.Parallel()
	fake, c, n := commentFake(t)
	for i := range 230 {
		if _, err := fake.AddComment("alice/web", n, "bob", fmt.Sprintf("c%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := c.IssueComments("alice", "web", n)
	if err != nil || len(got) != 230 {
		t.Fatalf("%d comments, %v", len(got), err)
	}
	for i, x := range got {
		if x.Body != fmt.Sprintf("c%d", i) {
			t.Fatalf("comment %d = %q", i, x.Body)
		}
	}
}

func TestCommentsListBreakingOnALaterPageGivesNothing(t *testing.T) {
	t.Parallel()
	fake, c, n := commentFake(t)
	for range 150 {
		if _, err := fake.AddComment("alice/web", n, "bob", "x"); err != nil {
			t.Fatal(err)
		}
	}
	fake.RefuseComments(fakegh.CommentList, true)
	got, err := c.IssueComments("alice", "web", n)
	if err == nil || got != nil {
		t.Errorf("got %d comments, %v: want an error and no part of the list", len(got), err)
	}
}

func TestCommentsAuthors(t *testing.T) {
	t.Parallel()
	fake, c, n := commentFake(t)
	for _, a := range []string{"dependabot[bot]", ""} {
		if _, err := fake.AddComment("alice/web", n, a, "x"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := c.IssueComments("alice", "web", n)
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v, %v", got, err)
	}
	if got[0].User == nil || got[0].User.Login != "dependabot[bot]" {
		t.Errorf("bot = %+v", got[0].User)
	}
	if got[1].User != nil {
		t.Errorf("a user that is gone is null: %+v", got[1].User)
	}
}

func TestCommentWritesThatFail(t *testing.T) {
	t.Parallel()
	fake, c, n := commentFake(t)
	id, err := fake.AddComment("alice/web", n, "alice", "x")
	if err != nil {
		t.Fatal(err)
	}
	for op, call := range map[string]func() error{
		fakegh.CommentCreate: func() error { _, err := c.CreateIssueComment("alice", "web", n, "y"); return err },
		fakegh.CommentUpdate: func() error { _, err := c.UpdateIssueComment("alice", "web", int64(id), "y"); return err },
		fakegh.CommentDelete: func() error { return c.DeleteIssueComment("alice", "web", int64(id)) },
	} {
		fake.RefuseComments(op, true)
		if err := call(); err == nil {
			t.Errorf("%s did not fail", op)
		}
		fake.RefuseComments(op, false)
		if err := call(); err != nil {
			t.Errorf("%s after it is allowed: %v", op, err)
		}
		id, _ = fake.AddComment("alice/web", n, "alice", "x")
	}
}

// Every write to a comment, by the API or by the admin, moves the update time
// of its issue, as GitHub does.
func TestCommentWritesMoveTheIssue(t *testing.T) {
	t.Parallel()
	fake, c, n := commentFake(t)
	clock := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	fake.SetNow(func() time.Time { clock = clock.Add(time.Hour); return clock })
	moved := func(what string, do func()) {
		t.Helper()
		before := issueOf(t, fake, n).UpdatedAt
		do()
		if !issueOf(t, fake, n).UpdatedAt.After(before) {
			t.Errorf("%s did not move the issue", what)
		}
	}
	var id int64
	moved("API create", func() {
		made, err := c.CreateIssueComment("alice", "web", n, "a")
		if err != nil {
			t.Fatal(err)
		}
		id = made.ID
	})
	moved("API update", func() {
		if _, err := c.UpdateIssueComment("alice", "web", id, "b"); err != nil {
			t.Fatal(err)
		}
	})
	moved("API delete", func() {
		if err := c.DeleteIssueComment("alice", "web", id); err != nil {
			t.Fatal(err)
		}
	})
	var aid int
	moved("admin add", func() {
		var err error
		if aid, err = fake.AddComment("alice/web", n, "bob", "x"); err != nil {
			t.Fatal(err)
		}
	})
	moved("admin edit", func() {
		if err := fake.EditComment("alice/web", aid, "y"); err != nil {
			t.Fatal(err)
		}
	})
	moved("admin remove", func() {
		if err := fake.RemoveComment("alice/web", aid); err != nil {
			t.Fatal(err)
		}
	})
}
