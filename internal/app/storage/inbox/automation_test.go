package inbox

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// ruleSet makes the commit of a rule: a set by c that carries the applied
// trailer, as the fetch writes it.
func ruleSet(t *testing.T, c *clone, u model.ULID, status string) {
	t.Helper()
	op := c.op("set", u)
	op.Automated = true
	if _, err := c.s.Update(op, func(tx *store.Tx) error {
		x, err := tx.Task(u)
		if err != nil {
			return err
		}
		x.Status = status
		tx.AddTrailer(automation.TrailerApplied, automation.AppliedTrailer(automation.Change{Rule: "merged", Event: "pr.merged"}))
		return tx.PutTask(x)
	}); err != nil {
		t.Fatal(err)
	}
}

// rejection makes the empty commit of a fetch that carries rejections.
func rejection(t *testing.T, c *clone, u model.ULID, rules ...string) {
	t.Helper()
	if _, err := c.s.Update(c.op("gh-fetch", model.ULID{}), func(tx *store.Tx) error {
		tx.AllowEmpty()
		for _, r := range rules {
			tx.AddTrailer(automation.TrailerRejected, automation.RejectedTrailer(u, automation.Rejection{Rule: r, Reason: "points is required"}))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// collectAfter collects what fn wrote in c for me.
func collectAfter(t *testing.T, c *clone, fn func(), me ...string) []Item {
	t.Helper()
	h := head(t, c)
	fn()
	items, err := Collect(c.s, h, head(t, c), me)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func TestCollectAutomationToAssigneeAndWatcher(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	x := addTask(t, a, "x", func(x *model.Task) {
		x.Assignees = []string{"github.com/b"}
		x.Watchers = []string{"github.com/b", "github.com/a"}
	})
	syncBoth(t, a, b)
	before := head(t, b)
	ruleSet(t, a, x.ULID, "done")
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	// The watch the status change would make is covered by the automation item.
	if len(items) != 1 || items[0].Kind != Automation || items[0].Task != x.ULID || items[0].To != "github.com/b" || items[0].ID != x.ID {
		t.Fatalf("items = %+v", items)
	}
	if string(items[0].Detail) != `{"rule":"merged","event":"pr.merged","status":"done"}` {
		t.Errorf("detail = %s", items[0].Detail)
	}
}

func TestCollectAutomationOwnCommit(t *testing.T) {
	t.Parallel()
	a, _ := twoClones(t)
	x := addTask(t, a, "x", func(x *model.Task) { x.Watchers = []string{"github.com/a"} })
	y := addTask(t, a, "y", nil)
	for _, c := range []struct {
		name string
		u    model.ULID
		want int
	}{{"watcher", x.ULID, 1}, {"neither", y.ULID, 0}} {
		items := collectAfter(t, a, func() { ruleSet(t, a, c.u, "done") }, "github.com/a")
		if len(items) != c.want {
			t.Errorf("%s: items = %+v; want %d", c.name, items, c.want)
		}
		for _, it := range items {
			if it.Kind != Automation {
				t.Errorf("%s: kind = %s", c.name, it.Kind)
			}
		}
	}
}

func TestCollectAutomationAssignedToo(t *testing.T) {
	t.Parallel()
	a, _ := twoClones(t)
	x := addTask(t, a, "x", func(x *model.Task) {
		x.Assignees = []string{"github.com/a"}
		x.Watchers = []string{"github.com/a"}
	})
	items := collectAfter(t, a, func() { ruleSet(t, a, x.ULID, "done") }, "github.com/a")
	if len(items) != 1 || items[0].Kind != Automation {
		t.Errorf("items = %+v; want one automation", items)
	}
}

func TestCollectIssueOverwritten(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	x := addTask(t, a, "x", func(x *model.Task) { x.Assignees = []string{"github.com/b"} })
	other := addTask(t, a, "other", nil)
	syncBoth(t, a, b)
	note := func(u model.ULID, fields ...string) {
		t.Helper()
		if _, err := a.s.Update(a.op("gh-push", u), func(tx *store.Tx) error {
			tx.AllowEmpty()
			for _, f := range fields {
				tx.AddTrailer(automation.TrailerIssueOverwritten, automation.IssueOverwrittenTrailer(u, "github.com/o/r/issues/1", automation.Overwrite{Field: f, Side: automation.SideGitHub, Lost: []string{"old\nline"}}))
			}
			tx.AddTrailer(automation.TrailerIssueOverwritten, "not one of ours")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	items := collectAfter(t, a, func() { note(x.ULID, "title", "body") }, "github.com/b")
	if len(items) != 2 || items[0].Kind != IssueOverwritten || items[0].Task != x.ULID || items[0].To != "github.com/b" || items[0].ID != x.ID {
		t.Fatalf("items = %+v", items)
	}
	if want := `{"ref":"github.com/o/r/issues/1","field":"body","side":"github","lost":["old\nline"]}`; string(items[1].Detail) != want {
		t.Errorf("detail = %s", items[1].Detail)
	}
	if key(items[0]) == key(items[1]) {
		t.Error("two values of a commit are one item")
	}
	if items := collectAfter(t, a, func() { note(other.ULID, "title") }, "github.com/b"); len(items) != 0 {
		t.Errorf("a task b has no part in: items = %+v", items)
	}
}

func TestCollectIssueOverwrittenComment(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	x := addTask(t, a, "x", func(x *model.Task) { x.Assignees = []string{"github.com/b"} })
	syncBoth(t, a, b)
	cm := addComment(t, a, x.ULID, "手元の本文を\nここに書く。あとは長い続き\n")
	field := "comments:" + cm.ULID.String()
	items := collectAfter(t, a, func() {
		if _, err := a.s.Update(a.op("gh-push", x.ULID), func(tx *store.Tx) error {
			tx.AllowEmpty()
			tx.AddTrailer(automation.TrailerIssueOverwritten, automation.IssueOverwrittenTrailer(x.ULID, "github.com/o/r/issues/1", automation.Overwrite{Field: field, Side: automation.SideGitHub, Lost: []string{"GitHub 側"}}))
			// a comment the commit does not have names nothing
			tx.AddTrailer(automation.TrailerIssueOverwritten, automation.IssueOverwrittenTrailer(x.ULID, "github.com/o/r/issues/1", automation.Overwrite{Field: "comments:" + model.NewULID().String(), Side: automation.SideGitHub, Lost: []string{"y"}}))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}, "github.com/b")
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	want := `{"ref":"github.com/o/r/issues/1","field":"` + field + `","side":"github","lost":["GitHub 側"],"comment":"手元の本文を ここに書く"}`
	if string(items[0].Detail) != want {
		t.Errorf("detail = %s\nwant %s", items[0].Detail, want)
	}
	if strings.Contains(string(items[1].Detail), `"comment":`) {
		t.Errorf("detail = %s; want no comment text", items[1].Detail)
	}
}

func TestCollectAutomationRejected(t *testing.T) {
	t.Parallel()
	a, b := twoClones(t)
	x := addTask(t, a, "x", func(x *model.Task) { x.Assignees = []string{"github.com/b"} })
	other := addTask(t, a, "other", nil)
	syncBoth(t, a, b)
	items := collectAfter(t, a, func() { rejection(t, a, x.ULID, "r1", "r2") }, "github.com/b")
	if len(items) != 2 || items[0].Kind != AutomationRejected || items[0].Task != x.ULID || items[0].To != "github.com/b" || items[0].ID != x.ID {
		t.Fatalf("items = %+v", items)
	}
	if string(items[1].Detail) != `{"rule":"r2","reason":"points is required"}` {
		t.Errorf("detail = %s", items[1].Detail)
	}
	if items := collectAfter(t, a, func() { rejection(t, a, other.ULID, "r1") }, "github.com/b"); len(items) != 0 {
		t.Errorf("a task b has no part in: items = %+v", items)
	}
	// A rejection tells the assignee even when they ran the fetch.
	syncBoth(t, a, b)
	items = collectAfter(t, b, func() { rejection(t, b, x.ULID, "r1") }, "github.com/b")
	if len(items) != 1 || items[0].Kind != AutomationRejected {
		t.Errorf("own fetch: items = %+v", items)
	}
}

func TestCollectAutomationBadTrailers(t *testing.T) {
	t.Parallel()
	a, _ := twoClones(t)
	x := addTask(t, a, "x", func(x *model.Task) { x.Assignees = []string{"github.com/a"} })
	items := collectAfter(t, a, func() {
		if _, err := a.s.Update(a.op("set", x.ULID), func(tx *store.Tx) error {
			tx.AllowEmpty()
			tx.AddTrailer(automation.TrailerApplied, "nospace")
			tx.AddTrailer(automation.TrailerRejected, "not-a-ulid rule reason")
			tx.AddTrailer(automation.TrailerRejected, model.NewULID().String()+" rule reason")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}, "github.com/a")
	if len(items) != 0 {
		t.Errorf("items = %+v; want none", items)
	}
}
