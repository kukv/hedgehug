package usecase

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

func TestPeople(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	got, err := ts.People()
	if err != nil || len(got) != 0 {
		t.Fatalf("People of an empty store = %q, %v", got, err)
	}

	ts.asAccount("alice")
	me, err := ts.AccountID()
	if err != nil || me == "" {
		t.Fatalf("AccountID = %q, %v", me, err)
	}
	a, err := ts.Add(AddInput{Title: "a", Assigns: mustAssigns(t, "assignees+=zed", "assignees+=bob", "watchers+=carol")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.CommentAdd(a.ULID.String(), "hi"); err != nil {
		t.Fatal(err)
	}

	got, err = ts.People()
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
	ts := newTwoStoreSession(t)
	if _, err := ts.ForStore("org-a").Add(AddInput{Title: "a", Assigns: mustAssigns(t, "assignees+=in-a")}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.ForStore("org-b").Add(AddInput{Title: "b", Assigns: mustAssigns(t, "assignees+=in-b")}); err != nil {
		t.Fatal(err)
	}
	// org-a's account is alice and org-b's is bob; each one's commits carry
	// the Hedgehug-Author of its own account.
	a, err := ts.ForStore("org-a").People()
	if wantA := []string{"github.com/alice", "in-a"}; err != nil || !slices.Equal(a, wantA) {
		t.Errorf("People of org-a = %q, %v; want %q", a, err, wantA)
	}
	all, err := ts.People()
	if wantAll := []string{"github.com/alice", "github.com/bob", "in-a", "in-b"}; err != nil || !slices.Equal(all, wantAll) {
		t.Errorf("People of every store = %q, %v; want %q", all, err, wantAll)
	}
	_, err = ts.ForStore("nope").People()
	wantErr(t, err, KindNotFound, "")
}

// What @ offers is written into a body as @<name>. The server reads a mention
// as @login or @host/login (index.Mentions) and names an account by it
// (index.MentionNames), so each name People gives must come back from
// Mentions unchanged, and a name that cannot be written that way is left out.
func TestPeopleNamesCanBeMentioned(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.asAccount("alice")
	me, err := ts.AccountID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Add(AddInput{Title: "a", Assigns: mustAssigns(t, "assignees+=bob", "assignees+=has space", "watchers+=github.com/carol")}); err != nil {
		t.Fatal(err)
	}
	got, err := ts.People()
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
	ts := newTestSession(t)
	ts.asAccount("alice")
	a, err := ts.Add(AddInput{Title: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// Only a commit of dave's: nothing he writes names him.
	ts.asAccount("dave")
	if _, err := ts.Set("ME-1", mustAssigns(t, "title=edited")); err != nil {
		t.Fatal(err)
	}
	// A comment by erin, who does not watch the task; the commit that holds
	// it is made as alice, so only the comment names erin.
	ts.asAccount("alice")
	c, err := ts.CommentAdd("ME-1", "hi")
	if err != nil {
		t.Fatal(err)
	}
	ts.editRawFile(model.CommentPath(a.ULID, c.Ref.Comment), "github.com/alice", "github.com/erin")
	// An assignee who does not watch the task: an assignee is made a watcher
	// as set, so the watch is taken off by a second set.
	for _, as := range []string{"assignees+=frank", "watchers-=frank"} {
		if _, err := ts.Set("ME-1", mustAssigns(t, as)); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ts.People()
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
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a", Assigns: mustAssigns(t, "assignees+=alice", "assignees+=Alice", "assignees+=ALICE", "assignees+=bob")}); err != nil {
		t.Fatal(err)
	}
	got, err := ts.People()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"ALICE", "bob", "github.com/alice"}; !slices.Equal(got, want) {
		t.Errorf("People = %q; want %q", got, want)
	}
}
