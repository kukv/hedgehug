package itemlink_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// rows は、下書きを「種類 項目 from→to」の文字列にする(比べやすくするため)。
func rows(ds []change.Draft) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = string(d.T) + " " + d.F + " " + string(d.From) + "→" + string(d.To)
	}
	return out
}

func refJSON(r itemlink.Ref) string { b, _ := json.Marshal(r); return string(b) }

func TestAttachWritesAddAndValuesAndClearsOff(t *testing.T) {
	s := itemlink.FromLog(logOf(change.NewAdd("links_off", pr41)))
	v := itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}
	got := rows(s.Attach(pr41, v))
	want := []string{
		"add links →" + refJSON(pr41),
		`set links/pr:org/web#41/title null→"Add login form"`,
		`set links/pr:org/web#41/state null→"open"`,
		`set links/pr:org/web#41/review null→"pending"`,
		"remove links_off " + refJSON(pr41) + "→",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Attach =\n%q\nwant\n%q", got, want)
	}
}

func TestAttachAfterDetachUsesKnownValuesAsFrom(t *testing.T) {
	log := logOf(
		change.NewAdd("links", iss12),
		change.NewSet(itemlink.ValueField(iss12, "title"), nil, "旧"),
		change.NewSet(itemlink.ValueField(iss12, "state"), nil, "open"),
		change.NewRemove("links", iss12),
	)
	got := rows(itemlink.FromLog(log).Attach(iss12, itemlink.Values{Title: "新", State: itemlink.Open, Review: itemlink.NoReview}))
	want := []string{
		"add links →" + refJSON(iss12),
		`set links/issue:org/api#12/title "旧"→"新"`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("Attach =\n%q\nwant\n%q (same values are not written again; an issue has no review row)", got, want)
	}
}

func TestDetachRemembersOnlyPRs(t *testing.T) {
	s := itemlink.FromLog(logOf(change.NewAdd("links", pr41), change.NewAdd("links", iss12)))
	pr, _ := s.Find(pr41)
	if got := rows(s.Detach(pr)); !slices.Equal(got, []string{"remove links " + refJSON(pr41) + "→", "add links_off →" + refJSON(pr41)}) {
		t.Errorf("Detach(PR) = %q", got)
	}
	iss, _ := s.Find(iss12)
	if got := rows(s.Detach(iss)); !slices.Equal(got, []string{"remove links " + refJSON(iss12) + "→"}) {
		t.Errorf("Detach(issue) = %q", got)
	}
}

func TestRefreshWritesOnlyChangedValues(t *testing.T) {
	s := itemlink.FromLog(logOf(
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "A"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
		change.NewSet(itemlink.ValueField(pr41, "review"), nil, "pending"),
	))
	l, _ := s.Find(pr41)
	if got := s.Refresh(l, l.Values); len(got) != 0 {
		t.Errorf("Refresh with the same values = %q, want nothing", rows(got))
	}
	got := rows(s.Refresh(l, itemlink.Values{Title: "A", State: itemlink.Merged, Review: itemlink.Approved}))
	want := []string{`set links/pr:org/web#41/state "open"→"merged"`, `set links/pr:org/web#41/review "pending"→"approved"`}
	if !slices.Equal(got, want) {
		t.Errorf("Refresh =\n%q\nwant\n%q", got, want)
	}
}

func TestRenameMovesTheLink(t *testing.T) {
	s := itemlink.FromLog(logOf(
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "A"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
	))
	l, _ := s.Find(pr41)
	moved := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web2", Number: 41}
	got := rows(s.Rename(l, "org/web2", itemlink.Values{Title: "A", State: itemlink.Merged, Review: itemlink.NoReview}))
	want := []string{
		"remove links " + refJSON(pr41) + "→",
		"add links →" + refJSON(moved),
		`set links/pr:org/web2#41/title "A"→"A"`,
		`set links/pr:org/web2#41/state "open"→"merged"`,
		`set links/pr:org/web2#41/review "none"→"none"`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("Rename =\n%q\nwant\n%q", got, want)
	}
}

func TestAttachWritesNoneForAnUnrecordedReview(t *testing.T) {
	s := itemlink.FromLog(logOf(
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "A"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
		change.NewRemove("links", pr41),
	))
	got := rows(s.Attach(pr41, itemlink.Values{Title: "A", State: itemlink.Open, Review: itemlink.NoReview}))
	want := []string{"add links →" + refJSON(pr41), `set links/pr:org/web#41/review null→"none"`}
	if !slices.Equal(got, want) {
		t.Errorf("Attach =\n%q\nwant\n%q", got, want)
	}
}
