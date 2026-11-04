package browse_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/comments"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestPeople(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	got, err := browse.People(ts.Session)
	if err != nil || len(got) != 0 {
		t.Fatalf("People of an empty store = %q, %v", got, err)
	}

	ts.AsAccount("alice")
	me, err := ts.AccountID()
	if err != nil || me == "" {
		t.Fatalf("AccountID = %q, %v", me, err)
	}
	a, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Assigns: usecasetest.MustAssigns(t, "assignees+=zed", "assignees+=bob", "watchers+=carol")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := comments.CommentAdd(ts.Session, a.ULID.String(), "hi"); err != nil {
		t.Fatal(err)
	}

	got, err = browse.People(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	// Assignees, watchers, the comment's author and the Hedgehug-Author of the
	// commits; sorted, each once.
	want := []string{"bob", "carol", me, "zed"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("People = %q; want %q", got, want)
	}
}

func TestPeopleOfOneStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	if _, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "a", Assigns: usecasetest.MustAssigns(t, "assignees+=in-a")}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Add(ts.ForStore("org-b"), tasks.AddInput{Title: "b", Assigns: usecasetest.MustAssigns(t, "assignees+=in-b")}); err != nil {
		t.Fatal(err)
	}
	// org-a's account is alice and org-b's is bob; each one's commits carry
	// the Hedgehug-Author of its own account.
	a, err := browse.People(ts.ForStore("org-a"))
	if wantA := []string{"github.com/alice", "in-a"}; err != nil || !slices.Equal(a, wantA) {
		t.Errorf("People of org-a = %q, %v; want %q", a, err, wantA)
	}
	all, err := browse.People(ts.Session)
	if wantAll := []string{"github.com/alice", "github.com/bob", "in-a", "in-b"}; err != nil || !slices.Equal(all, wantAll) {
		t.Errorf("People of every store = %q, %v; want %q", all, err, wantAll)
	}
	_, err = browse.People(ts.ForStore("nope"))
	usecasetest.WantErr(t, err, usecase.KindNotFound, "")
}

// What @ offers is written into a body as @<name>. The server reads a mention
// as @login or @host/login (index.Mentions) and names an account by it
// (index.MentionNames), so each name People gives must come back from
// Mentions unchanged, and a name that cannot be written that way is left out.
func TestPeopleNamesCanBeMentioned(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	me, err := ts.AccountID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Assigns: usecasetest.MustAssigns(t, "assignees+=bob", "assignees+=has space", "watchers+=github.com/carol")}); err != nil {
		t.Fatal(err)
	}
	got, err := browse.People(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(got, "has space") {
		t.Errorf("People = %q; a name with a space cannot be mentioned", got)
	}
	for _, want := range []string{"bob", "github.com/carol", me} {
		if !slices.Contains(got, want) {
			t.Errorf("People = %q; want %q", got, want)
		}
	}
	for _, n := range got {
		ms := index.Mentions("thanks @" + n + " for this")
		if !slices.Equal(ms, []string{n}) {
			t.Errorf("Mentions(@%s) = %q; want [%s]", n, ms, n)
		}
		if !index.MentionNames(n, n) {
			t.Errorf("MentionNames(%q, %q) = false", n, n)
		}
	}
	// An account is named by the mention of its ID and by its login alone.
	if !index.MentionNames("alice", me) || !index.MentionNames(me, me) {
		t.Errorf("a mention of alice or %s does not name the account %s", me, me)
	}
}

// Each source of a name is the only one for some name, so that dropping the
// source shows: the Hedgehug-Author of a commit (an account that only
// committed), the author of a comment who is not a watcher, and an assignee
// who is not a watcher.
func TestPeopleEachSource(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AsAccount("alice")
	a, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// Only a commit of dave's: nothing he writes names him.
	ts.AsAccount("dave")
	if _, err := tasks.Set(ts.Session, "ME-1", usecasetest.MustAssigns(t, "title=edited")); err != nil {
		t.Fatal(err)
	}
	// A comment by erin, who does not watch the task; the commit that holds
	// it is made as alice, so only the comment names erin.
	ts.AsAccount("alice")
	c, err := comments.CommentAdd(ts.Session, "ME-1", "hi")
	if err != nil {
		t.Fatal(err)
	}
	ts.EditRawFile(model.CommentPath(a.ULID, c.Ref.Comment), "github.com/alice", "github.com/erin")
	// An assignee who does not watch the task: an assignee is made a watcher
	// as set, so the watch is taken off by a second set.
	for _, as := range []string{"assignees+=frank", "watchers-=frank"} {
		if _, err := tasks.Set(ts.Session, "ME-1", usecasetest.MustAssigns(t, as)); err != nil {
			t.Fatal(err)
		}
	}

	got, err := browse.People(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"github.com/alice", "github.com/dave", "github.com/erin", "frank"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("People = %q; want %q", got, want)
	}
}

// Account IDs are not case-sensitive (replicate.Members, index.MentionNames), so
// names that differ only in case are one; the one that sorts first stays.
func TestPeopleFoldsCase(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a", Assigns: usecasetest.MustAssigns(t, "assignees+=alice", "assignees+=Alice", "assignees+=ALICE", "assignees+=bob")}); err != nil {
		t.Fatal(err)
	}
	got, err := browse.People(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"ALICE", "bob", "github.com/alice"}; !slices.Equal(got, want) {
		t.Errorf("People = %q; want %q", got, want)
	}
}
