package ops

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/index"
)

func mustAssigns(t *testing.T, args ...string) []Assign {
	t.Helper()
	as, err := ParseAssigns(args)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

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
	a, err := ts.ForStore("org-a").People()
	if err != nil || !slices.Contains(a, "in-a") || slices.Contains(a, "in-b") || !slices.IsSorted(a) {
		t.Errorf("People of org-a = %q, %v; want in-a and not in-b", a, err)
	}
	all, err := ts.People()
	if err != nil || !slices.Contains(all, "in-a") || !slices.Contains(all, "in-b") || !slices.IsSorted(all) {
		t.Errorf("People of every store = %q, %v; want both in-a and in-b", all, err)
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
