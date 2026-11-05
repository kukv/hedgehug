package integration_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// linksOf is the links of the task id names, in the store ss works on.
func linksOf(t *testing.T, ss *usecase.Session, id string) []model.Link {
	t.Helper()
	s, ix, err := ss.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ss.Resolve(ix, id)
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return x.Links
}

func TestLinkAndUnlink(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	r, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "Org-A/Web#123"})
	if err != nil || !r.Committed {
		t.Fatalf("link = %+v, %v", r, err)
	}
	if op := usecasetest.LastOp(t, ts); op != "link" {
		t.Errorf("op = %s", op)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Ref != "github.com/org-a/web/pull/123" || l[0].Sync != model.SyncMirror || l[0].Type != model.LinkPR {
		t.Fatalf("links = %+v", l)
	}
	head := ts.Head()
	_, err = integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "link.exists")
	if ts.Head() != head {
		t.Error("linking twice committed")
	}
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"}); err != nil {
		t.Fatal(err)
	}
	if op := usecasetest.LastOp(t, ts); op != "unlink" {
		t.Errorf("op = %s", op)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncOff {
		t.Fatalf("after unlink = %+v", l)
	}
	head = ts.Head()
	_, err = integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "link.not_found")
	if ts.Head() != head {
		t.Error("unlinking twice committed")
	}
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#123"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncMirror {
		t.Fatalf("linked again = %+v", l)
	}
}

func TestLinkIssueModes(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#45"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); l[0].Type != model.LinkIssue || l[0].Sync != model.SyncLink || l[0].Ref != "github.com/o/r/issues/45" {
		t.Fatalf("links = %+v", l)
	}
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#45", Sync: "push"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncPush {
		t.Fatalf("mode = %+v", l)
	}
}

func TestLinkIssueDefaultModeIsTheFiles(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	ts.WriteAndCommit(model.AutomationsPath, "issue_fields:\n  default_sync: mirror\n")
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#2", Sync: "push"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); l[0].Sync != model.SyncMirror || l[1].Sync != model.SyncPush {
		t.Fatalf("links = %+v", l)
	}
	// Linking again without a mode is not a change to the default.
	_, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#2"})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "link.exists")
}

// The mode goes into the task, so it is not guessed from a file that cannot
// be read. What does not need the file goes on.
func TestLinkIssueWithAnUnusableFile(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		file string
		code string
	}{
		"broken":        {file: "rules: 3\n", code: "automation.invalid"},
		"unknown value": {file: "issue_fields:\n  default_sync: off\n", code: "automation.invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := usecasetest.New(t)
			ts.AddTask("a", nil)
			ts.WriteAndCommit(model.AutomationsPath, c.file)
			_, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#1"})
			usecasetest.MustCode(t, err, usecase.KindInvalid, c.code)
			if l := linksOf(t, ts.Session, "ME-1"); len(l) != 0 {
				t.Fatalf("links = %+v", l)
			}
			if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#1", Sync: "link"}); err != nil {
				t.Fatal(err)
			}
			if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#2"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// What gh holds of a ref goes when the link comes in or comes back on, and
// stays when only the mode of a live link changes.
func TestLinkClearsWhatGHHoldsOfTheRef(t *testing.T) {
	t.Parallel()
	const ref = "github.com/o/r/issues/1"
	stale := model.GitHubState{Issues: []model.IssueState{{Ref: ref, State: "open"}}}
	for name, c := range map[string]struct {
		links []model.Link
		items int
	}{
		"new":         {nil, 0},
		"off":         {[]model.Link{{Type: model.LinkIssue, Ref: ref, Sync: model.SyncOff}}, 0},
		"mode change": {[]model.Link{{Type: model.LinkIssue, Ref: ref, Sync: model.SyncLink}}, 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := usecasetest.New(t)
			ts.AddTask("a", func(x *model.Task) {
				x.Links = c.links
				if err := x.SetGitHub(stale); err != nil {
					t.Fatal(err)
				}
			})
			if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{Issue: "o/r#1", Sync: "push"}); err != nil {
				t.Fatal(err)
			}
			if g, _ := taskIn(t, ts, "me", "ME-1").GitHub(); len(g.Issues) != c.items {
				t.Errorf("gh = %+v", g)
			}
		})
	}
}

func TestLinkAgainDropsWhatFoundIt(t *testing.T) {
	t.Parallel()
	const ref = "github.com/o/r/pull/1"
	ts := usecasetest.New(t)
	ts.AddTask("a", func(x *model.Task) {
		x.Links = []model.Link{{Type: model.LinkPR, Ref: ref, Sync: model.SyncOff, Via: model.LinkViaBranch}}
	})
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if l := taskIn(t, ts, "me", "ME-1").Links; len(l) != 1 || l[0].Via != "" || !l[0].Live() {
		t.Errorf("links = %+v", l)
	}
}

func TestLinkInputs(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	for name, in := range map[string]integration.LinkInput{
		"nothing":  {},
		"both":     {PR: "o/r#1", Issue: "o/r#2"},
		"bad":      {PR: "web#1"},
		"pr sync":  {PR: "o/r#1", Sync: "push"},
		"off":      {Issue: "o/r#1", Sync: "off"},
		"bad sync": {Issue: "o/r#1", Sync: "both"},
	} {
		_, err := integration.Link(ts.Session, "ME-1", in)
		t.Log(name)
		usecasetest.MustCode(t, err, usecase.KindUsage, "link.invalid")
	}
	// A host may be written.
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "ghe.example/o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); l[0].Ref != "ghe.example/o/r/pull/1" {
		t.Errorf("links = %+v", l)
	}
}

func TestLinkInAnotherStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	// WEB-1 is in org-b, whose account is bob; the host is bob's.
	if _, err := integration.Link(ts.Session, "org-b:WEB-1", integration.LinkInput{PR: "o/r#7"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.ForStore("org-b"), "WEB-1"); len(l) != 1 || l[0].Ref != "github.com/o/r/pull/7" {
		t.Errorf("org-b links = %+v", l)
	}
	if l := linksOf(t, ts.ForStore("org-a"), "API-1"); len(l) != 0 {
		t.Errorf("org-a links = %+v", l)
	}
	// Without the store name the id is looked up in every store.
	if _, err := integration.Unlink(ts.Session, "API-1", integration.LinkInput{PR: "o/r#7"}); err == nil {
		t.Error("unlinked what API-1 never had")
	}
	if _, err := integration.Unlink(ts.Session, "WEB-1", integration.LinkInput{PR: "o/r#7"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.ForStore("org-b"), "WEB-1"); len(l) != 1 || l[0].Sync != model.SyncOff {
		t.Errorf("org-b links = %+v", l)
	}
}

func TestLinkRefusesDeletedTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	ts.DeleteTask("ME-1")
	head := ts.Head()
	_, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"})
	usecasetest.MustCode(t, err, usecase.KindConflict, "task.deleted")
	if ts.Head() != head {
		t.Error("linking a deleted task committed")
	}
}

func TestLinkByFormerID(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", func(x *model.Task) { x.FormerIDs = []string{"OLD-7"} })
	if _, err := integration.Link(ts.Session, "OLD-7", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 {
		t.Errorf("links = %+v", l)
	}
}

func TestUndoTakesBackLink(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddTask("a", nil)
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 0 {
		t.Errorf("links after undo = %+v", l)
	}
	// An unlink is taken back as well: the link is live again.
	if _, err := integration.Link(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := integration.Unlink(ts.Session, "ME-1", integration.LinkInput{PR: "o/r#1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	if l := linksOf(t, ts.Session, "ME-1"); len(l) != 1 || l[0].Sync != model.SyncMirror {
		t.Errorf("links after undoing unlink = %+v", l)
	}
}

func taskIn(t *testing.T, ts *usecasetest.Session, store, id string) *model.Task {
	t.Helper()
	s, ix, err := ts.ForStore(store).Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ts.Resolve(ix, id)
	if err != nil {
		t.Fatal(err)
	}
	x, err := s.Task(e.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
