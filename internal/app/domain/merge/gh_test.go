package merge

import (
	"strings"
	"testing"
)

const (
	refA = "github.com/o/r/pull/1"
	refB = "github.com/o/r/pull/2"
)

func prLines(state1, state2 string) []string {
	return []string{
		"gh:", "  prs:",
		"    - ref: " + refA, "      state: " + state1, "      review: none", "      merged_at: null",
		"    - ref: " + refB, "      state: " + state2, "      review: none", "      merged_at: null",
	}
}

func TestGHMergesPRsByRef(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	theirs := task("2026-10-31T12:00:00Z", prLines("open", "closed")...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	x := decoded(t, r.Data)
	g, err := x.GitHub()
	if err != nil || len(g.PRs) != 2 || g.PRs[0].State != "merged" || g.PRs[1].State != "closed" {
		t.Fatalf("prs = %+v, %v\n%s", g.PRs, err, r.Data)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestGHMergesTheTitleKeyByKey(t *testing.T) {
	t.Parallel()
	const ref = "github.com/o/r/pull/1"
	item := func(title, state string) []string {
		return []string{"gh:", "  prs:", "    - ref: " + ref, "      title: " + title, "      state: " + state, "      review: none", "      merged_at: null"}
	}
	base := task("2026-10-31T10:00:00Z", item("old", "open")...)
	ours := task("2026-10-31T11:00:00Z", item("new", "open")...)
	theirs := task("2026-10-31T12:00:00Z", item("old", "merged")...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	g, err := decoded(t, r.Data).GitHub()
	if err != nil || len(g.PRs) != 1 || g.PRs[0].Title != "new" || g.PRs[0].State != "merged" {
		t.Fatalf("prs = %+v, %v\n%s", g.PRs, err, r.Data)
	}
}

func TestGHConflictTakesTheLaterAndTellsNobody(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	theirs := task("2026-10-31T12:00:00Z", prLines("closed", "open")...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	g, err := decoded(t, r.Data).GitHub()
	if err != nil || g.PRs[0].State != "closed" {
		t.Fatalf("prs = %+v, %v", g.PRs, err)
	}
	if len(r.Overwritten) != 0 {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

func TestGHAddsTheElementsOfBothSides(t *testing.T) {
	t.Parallel()
	one := func(ref string) []string {
		return []string{"gh:", "  prs:", "    - ref: " + ref, "      state: open", "      review: none", "      merged_at: null"}
	}
	r := mergeTask(t,
		side(task("2026-10-31T10:00:00Z"), ""),
		side(task("2026-10-31T11:00:00Z", one(refA)...), "a"),
		side(task("2026-10-31T12:00:00Z", one(refB)...), "b"))
	if s := string(r.Data); !strings.Contains(s, refA) || !strings.Contains(s, refB) {
		t.Errorf("merged:\n%s", s)
	}
}

// The value of a key that is not a list of refs merges as before.
func TestGHOtherKeysAsBefore(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", "gh:", "  prs: [1]")
	ours := task("2026-10-31T11:00:00Z", "gh:", "  prs: [2]")
	theirs := task("2026-10-31T12:00:00Z", "gh:", "  prs: [3]")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	if !strings.Contains(string(r.Data), "prs: [3]") {
		t.Errorf("merged:\n%s", r.Data)
	}
}

// Two people change the same link: the later wins, as links are told apart
// by ref (setField). This fixes unlink against a change of the mode.
func TestLinkOfBothSidesTakesTheLater(t *testing.T) {
	t.Parallel()
	link := func(sync string) []string {
		return []string{"links:", "  - {type: github_issue, ref: github.com/o/r/issues/1, sync: " + sync + "}"}
	}
	r := mergeTask(t,
		side(task("2026-10-31T10:00:00Z", link("link")...), ""),
		side(task("2026-10-31T11:00:00Z", link("off")...), "a"),
		side(task("2026-10-31T12:00:00Z", link("push")...), "b"))
	x := decoded(t, r.Data)
	if len(x.Links) != 1 || x.Links[0].Sync != "push" {
		t.Errorf("links = %+v", x.Links)
	}
	if len(r.Overwritten) != 1 || r.Overwritten[0].Field != "links" {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

// A key of an item that this version does not know merges key by key, and
// stays when only one side has it.
func TestGHKeepsUnknownKeysOfItems(t *testing.T) {
	t.Parallel()
	item := func(state, extra string) []string {
		l := []string{"gh:", "  prs:", "    - ref: " + refA, "      state: " + state, "      review: none", "      merged_at: null"}
		if extra != "" {
			l = append(l, "      "+extra)
		}
		return l
	}
	base := task("2026-10-31T10:00:00Z", item("open", "")...)
	ours := task("2026-10-31T11:00:00Z", item("merged", "")...)
	theirs := task("2026-10-31T12:00:00Z", item("open", "future: 1")...)
	for _, c := range []struct{ o, t []byte }{{ours, theirs}, {theirs, ours}} {
		r := mergeTask(t, side(base, ""), side(c.o, "a"), side(c.t, "b"))
		s := string(r.Data)
		if !strings.Contains(s, "future: 1") || !strings.Contains(s, "state: merged") || len(r.Overwritten) != 0 {
			t.Errorf("merged:\n%s\noverwritten %v", s, r.Overwritten)
		}
	}
}

// A key of gh this version does not know stays beside prs.
func TestGHKeepsUnknownKeys(t *testing.T) {
	t.Parallel()
	pr := func(state string, extra ...string) []string {
		return append([]string{"gh:", "  prs:", "    - ref: " + refA, "      state: " + state, "      review: none", "      merged_at: null"}, extra...)
	}
	base := task("2026-10-31T10:00:00Z", pr("open")...)
	ours := task("2026-10-31T11:00:00Z", pr("merged")...)
	theirs := task("2026-10-31T12:00:00Z", pr("closed", "  future: {a: 1}")...)
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	if s := string(r.Data); !strings.Contains(s, "future:") || !strings.Contains(s, "state: closed") || len(r.Overwritten) != 0 {
		t.Errorf("merged:\n%s\noverwritten %v", s, r.Overwritten)
	}
}

// The result does not depend on which clone is ours.
func TestGHMergeIsSymmetric(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	a := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	b := task("2026-10-31T12:00:00Z", prLines("closed", "closed")...)
	r1 := mergeTask(t, side(base, ""), side(a, "a"), side(b, "b"))
	r2 := mergeTask(t, side(base, ""), side(b, "b"), side(a, "a"))
	if string(r1.Data) != string(r2.Data) {
		t.Errorf("%s\n---\n%s", r1.Data, r2.Data)
	}
}

// A side that only updated gh did not edit the task: the deletion by the
// other side stays, and nobody is told the task came back.
func TestGHOnlyChangeDoesNotReviveADeletedTask(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	del := task("2026-10-31T11:00:00Z", append(prLines("open", "open"), "deleted_at: 2026-10-31T11:00:00Z")...)
	fetched := task("2026-10-31T12:00:00Z", prLines("merged", "open")...)
	for _, c := range []struct{ o, t Side }{{side(del, "a"), side(fetched, "b")}, {side(fetched, "b"), side(del, "a")}} {
		r := mergeTask(t, side(base, ""), c.o, c.t)
		x := decoded(t, r.Data)
		if x.DeletedAt == nil || len(r.Notices) != 0 {
			t.Errorf("deleted_at %v notices %v", x.DeletedAt, r.Notices)
		}
	}
}

// A change of links is a person's edit, so it still revives.
func TestLinksChangeRevivesADeletedTask(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z")
	del := task("2026-10-31T11:00:00Z", "deleted_at: 2026-10-31T11:00:00Z")
	linked := task("2026-10-31T12:00:00Z", "links:", "  - {type: github_pr, ref: "+refA+", sync: link}")
	r := mergeTask(t, side(base, ""), side(del, "a"), side(linked, "b"))
	if x := decoded(t, r.Data); x.DeletedAt != nil || len(r.Notices) != 1 {
		t.Errorf("deleted_at %v notices %v", x.DeletedAt, r.Notices)
	}
}

// Only the title is told: a conflict in gh is not.
func TestGHAndTitleConflictTellsOnlyTheTitle(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", "title: x", "gh:", "  prs: [1]")
	ours := task("2026-10-31T11:00:00Z", "title: o", "gh:", "  prs: [2]")
	theirs := task("2026-10-31T12:00:00Z", "title: t", "gh:", "  prs: [3]")
	r := mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b"))
	want := []Overwritten{{Path: taskPath, Field: "title", Value: `"o"`, Author: "a"}}
	if !sameOverwritten(r.Overwritten, want) {
		t.Errorf("overwritten = %v", r.Overwritten)
	}
}

// One side removed an element and the other changed it, or another one.
func TestGHRemovedAndChangedElement(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", "gh:", "  prs:", "    - ref: "+refA, "      state: open", "      review: none", "      merged_at: null")
	// B was removed by ours and changed by theirs: the change stays.
	theirs := task("2026-10-31T12:00:00Z", prLines("open", "closed")...)
	g, err := decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data).GitHub()
	if err != nil || len(g.PRs) != 2 || g.PRs[1].Ref != refB || g.PRs[1].State != "closed" {
		t.Errorf("prs = %+v, %v", g.PRs, err)
	}
	// B was removed by ours while theirs changed A: B is gone.
	theirs = task("2026-10-31T12:00:00Z", prLines("merged", "open")...)
	g, err = decoded(t, mergeTask(t, side(base, ""), side(ours, "a"), side(theirs, "b")).Data).GitHub()
	if err != nil || len(g.PRs) != 1 || g.PRs[0].Ref != refA || g.PRs[0].State != "merged" {
		t.Errorf("prs = %+v, %v", g.PRs, err)
	}
}

// Items without a ref cannot be told apart: the list merges whole.
func TestGHItemsWithoutRefMergeWhole(t *testing.T) {
	t.Parallel()
	list := func(s string) []string { return []string{"gh:", "  prs:", "    - state: " + s} }
	r := mergeTask(t,
		side(task("2026-10-31T10:00:00Z", list("open")...), ""),
		side(task("2026-10-31T11:00:00Z", list("merged")...), "a"),
		side(task("2026-10-31T12:00:00Z", list("closed")...), "b"))
	if s := string(r.Data); !strings.Contains(s, "state: closed") || strings.Contains(s, "merged") || len(r.Overwritten) != 0 {
		t.Errorf("merged:\n%s\noverwritten %v", s, r.Overwritten)
	}
}

func TestGHUnderOursWins(t *testing.T) {
	t.Parallel()
	base := task("2026-10-31T10:00:00Z", prLines("open", "open")...)
	ours := task("2026-10-31T11:00:00Z", prLines("merged", "open")...)
	theirs := task("2026-10-31T12:00:00Z", prLines("closed", "closed")...)
	r := mergeWith(t, OursWins, side(base, ""), side(ours, "a"), side(theirs, "b"))
	g, err := decoded(t, r.Data).GitHub()
	if err != nil || g.PRs[0].State != "merged" || g.PRs[1].State != "closed" || len(r.Overwritten) != 0 {
		t.Errorf("prs = %+v, %v, overwritten %v", g.PRs, err, r.Overwritten)
	}
}
