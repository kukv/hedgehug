package model

import (
	"strings"
	"testing"
	"time"
)

func taskWith(t *testing.T, front string) *Task {
	t.Helper()
	x, err := DecodeTask([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\n" + front + "---\n"))
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestGitHubReadsAndKeepsUnknownKeys(t *testing.T) {
	t.Parallel()
	x := taskWith(t, "gh:\n  number: 1\n  prs:\n    - ref: github.com/o/r/pull/1\n      state: open\n      review: none\n      merged_at: null\n      note: keep\n")
	g, err := x.GitHub()
	if err != nil || len(g.PRs) != 1 || g.PRs[0].State != "open" || g.PRs[0].Review != "none" {
		t.Fatalf("GitHub = %+v, %v", g, err)
	}
	at := time.Date(2026, 11, 5, 1, 2, 3, 0, time.UTC)
	g.PRs[0].State, g.PRs[0].MergedAt = "merged", &at
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	b, err := x.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"number: 1", "state: merged", "note: keep", "merged_at: 2026-11-05T01:02:03Z"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("no %q in\n%s", want, b)
		}
	}
}

func TestGitHubEmptyAndWrongShape(t *testing.T) {
	t.Parallel()
	if g, err := taskWith(t, "").GitHub(); err != nil || len(g.PRs) != 0 {
		t.Errorf("none = %+v, %v", g, err)
	}
	if g, err := taskWith(t, "gh: {}\n").GitHub(); err != nil || len(g.Issues) != 0 {
		t.Errorf("empty = %+v, %v", g, err)
	}
	if _, err := taskWith(t, "gh: text\n").GitHub(); err == nil {
		t.Error("a scalar reads")
	}
	x := taskWith(t, "")
	if err := x.SetGitHub(GitHubState{}); err != nil || x.GH != nil {
		t.Errorf("nothing to keep: %v, %v", x.GH, err)
	}
}

const ghFront = `gh:
  number: 1.0 # keep
  prs:
    - ref: github.com/o/r/pull/1
      extra: [1.0, 2]   # unknown
      state: open
      review: none
      merged_at: null
    - state: open
      ref: github.com/o/r/pull/2
      review: approved
      note: {a: 1.0}
      merged_at: 2026-11-05T01:02:03Z
  issues:
    - ref: github.com/o/r/issues/3
      state: closed
      state_reason: null
      synced:
        title: "x" # comment
        n: 1.0
`

func encoded(t *testing.T, x *Task) string {
	t.Helper()
	b, err := x.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetGitHubUnchangedKeepsBytes(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	before := encoded(t, x)
	g, err := x.GitHub()
	if err != nil {
		t.Fatal(err)
	}
	if g.Issues[0].StateReason != nil || g.Issues[0].Synced == nil || g.PRs[0].MergedAt != nil || g.PRs[1].MergedAt == nil {
		t.Fatalf("issue = %+v", g.Issues[0])
	}
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	if after := encoded(t, x); after != before {
		t.Errorf("round trip changed the file:\n%s\nwant\n%s", after, before)
	}
	for _, want := range []string{"number: 1.0 # keep", "extra: [1.0, 2]", "note: {a: 1.0}", "n: 1.0", "state_reason: null", "merged_at: 2026-11-05T01:02:03Z", "title: \"x\" # comment"} {
		if !strings.Contains(before, want) {
			t.Errorf("no %q in\n%s", want, before)
		}
	}
}

func TestSetGitHubChangeKeepsTheOthers(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	before := encoded(t, x)
	g, _ := x.GitHub()
	g.PRs[0].State = "merged"
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, "state: open\n    review: none", "state: merged\n    review: none", 1)
	if got := encoded(t, x); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	g, _ = x.GitHub()
	r := "completed"
	g.Issues[0].StateReason = &r
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	got := encoded(t, x)
	if !strings.Contains(got, "state_reason: completed") || !strings.Contains(got, "n: 1.0") || !strings.Contains(got, "extra: [1.0, 2]") {
		t.Errorf("got\n%s", got)
	}
}

func TestSetGitHubDropsAnItemAndKeepsKeysOfTheRest(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	g, _ := x.GitHub()
	g.PRs = g.PRs[1:]
	g.PRs[0].State = "closed"
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	got := encoded(t, x)
	if strings.Contains(got, "pull/1\n") || !strings.Contains(got, "note: {a: 1.0}") || strings.Contains(got, "extra: [1.0, 2]") || !strings.Contains(got, "state: closed") {
		t.Errorf("got\n%s", got)
	}
}

func TestSetGitHubEmptiedLeavesGHOut(t *testing.T) {
	t.Parallel()
	x := taskWith(t, ghFront)
	if err := x.SetGitHub(GitHubState{}); err != nil {
		t.Fatal(err)
	}
	if got := encoded(t, x); strings.Contains(got, "prs:") || strings.Contains(got, "issues:") || !strings.Contains(got, "number: 1.0 # keep") {
		t.Errorf("other keys stay:\n%s", got)
	}
	y := taskWith(t, "gh:\n  prs:\n    - ref: github.com/o/r/pull/1\n      state: open\n      review: none\n      merged_at: null\n")
	if err := y.SetGitHub(GitHubState{}); err != nil || y.GH != nil {
		t.Errorf("gh = %v, %v", y.GH, err)
	}
	if strings.Contains(encoded(t, y), "gh:") {
		t.Error("gh is written")
	}
}

func TestSetGitHubReplacesANonMapping(t *testing.T) {
	t.Parallel()
	x := taskWith(t, "gh: text\n")
	g := GitHubState{PRs: []PRState{{Ref: "github.com/o/r/pull/1", State: "open", Review: "none"}}}
	if err := x.SetGitHub(g); err != nil {
		t.Fatal(err)
	}
	if got := encoded(t, x); strings.Contains(got, "text") || !strings.Contains(got, "pull/1") {
		t.Errorf("got\n%s", got)
	}
	y := taskWith(t, "gh: text\n")
	if err := y.SetGitHub(GitHubState{}); err != nil || y.GH != nil {
		t.Errorf("gh = %v, %v", y.GH, err)
	}
}

func TestLinkFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	x := taskWith(t, "links:\n  - {type: github_issue, ref: github.com/o/r/issues/3, sync: mirror, fields: [title, status]}\n")
	if len(x.Links) != 1 || len(x.Links[0].Fields) != 2 || x.Links[0].Fields[1] != "status" {
		t.Fatalf("links = %+v", x.Links)
	}
	before := encoded(t, x)
	x.Links[0].Sync = SyncPush
	got := encoded(t, x)
	if !strings.Contains(before, "fields: [title, status]") || !strings.Contains(got, "sync: push") || !strings.Contains(got, "fields: [title, status]") {
		t.Errorf("before\n%s\nafter\n%s", before, got)
	}
}

func TestGitHubRef(t *testing.T) {
	t.Parallel()
	r, ok := ParseGitHubRef("github.com/org-a/web/pull/123")
	if !ok || r.Repo != "github.com/org-a/web" || r.Kind != LinkPR || r.Number != 123 {
		t.Fatalf("ParseGitHubRef = %+v, %v", r, ok)
	}
	if r.String() != "github.com/org-a/web/pull/123" || r.Label() != "org-a/web#123" {
		t.Errorf("%s, %s", r, r.Label())
	}
	i, ok := ParseGitHubRef("github.com/org-a/web/issues/45")
	if !ok || i.Kind != LinkIssue || i.String() != "github.com/org-a/web/issues/45" {
		t.Errorf("issue = %+v, %v", i, ok)
	}
	u, ok := ParseGitHubRef("GitHub.com/Org-A/Web/pull/7")
	if !ok || u.Repo != "github.com/org-a/web" || u.String() != "github.com/org-a/web/pull/7" {
		t.Errorf("upper case = %+v, %v", u, ok)
	}
	if got := (GitHubRef{Repo: "GitHub.com/Org-A/Web", Kind: LinkPR, Number: 7}).String(); got != "github.com/org-a/web/pull/7" {
		t.Errorf("String = %s", got)
	}
	if CanonRef("github.com/Org-A/web/issues/3") != "github.com/org-a/web/issues/3" || CanonRef("Odd Text") != "Odd Text" {
		t.Error("CanonRef")
	}
	for _, bad := range []string{"", "github.com/org-a/web", "github.com/org-a/web/pull/0", "org-a/web/pull/1"} {
		if _, ok := ParseGitHubRef(bad); ok {
			t.Errorf("%q parses", bad)
		}
	}
	if !(Link{Sync: SyncMirror}).Live() || (Link{Sync: SyncOff}).Live() {
		t.Error("Live")
	}
}
