package integration_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// devStore gives the test store the dev schema and the dev rules, after edit
// changed them.
func devStore(t *testing.T, ts *usecasetest.Session, edit func(*schema.Schema, *automation.File)) {
	t.Helper()
	sc, f := schema.Dev(), automation.DevRules()
	if edit != nil {
		edit(sc, f)
	}
	b, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		if err := tx.PutSchema(sc); err != nil {
			return err
		}
		return tx.PutFile(model.AutomationsPath, b)
	})
}

func logOf(t *testing.T, ts *usecasetest.Session) []gitrepo.LogEntry {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	es, err := usecasetest.LogOf(s.Repo(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return es
}

func countOp(es []gitrepo.LogEntry, op string) int {
	n := 0
	for _, e := range es {
		if v, _ := e.Trailer("Hedgehug-Op"); v == op {
			n++
		}
	}
	return n
}

func wantStatus(t *testing.T, ts *usecasetest.Session, id, want string) {
	t.Helper()
	if got := taskIn(t, ts, "me", id).Status; got != want {
		t.Errorf("status of %s = %s, want %s", id, got, want)
	}
}

// wantApplied checks that HEAD is the status commit of the rule.
func wantApplied(t *testing.T, ts *usecasetest.Session, trailer string) {
	t.Helper()
	head := logOf(t, ts)[0]
	if op, _ := head.Trailer("Hedgehug-Op"); op != "set" {
		t.Fatalf("op of HEAD = %s", op)
	}
	if v, _ := head.Trailer(automation.TrailerApplied); v != trailer {
		t.Errorf("trailer = %q, want %q", v, trailer)
	}
}

func TestRulesFollowAPullRequest(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	x := ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if r := fetch(t, ts); !r.Committed || len(r.Warnings) != 0 {
		t.Fatalf("report = %+v", r)
	}
	wantStatus(t, ts, "ME-1", "in_progress")
	wantApplied(t, ts, "pr-opened pr.opened")
	es := logOf(t, ts)
	if !strings.HasPrefix(es[0].Author.Name, "hedgehug-automation") || es[0].Writer() != "github.com/alice" {
		t.Errorf("author = %+v, writer = %s", es[0].Author, es[0].Writer())
	}
	if v, _ := es[0].Trailer("Hedgehug-Task"); v != x.ULID.String() {
		t.Errorf("task = %s", v)
	}
	if op, _ := es[1].Trailer("Hedgehug-Op"); op != "gh-fetch" {
		t.Errorf("op before the status = %s", op)
	}

	later(ts, 5*time.Minute)
	if err := fake.RequestReview("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "review")
	wantApplied(t, ts, "pr-review pr.review_requested")

	later(ts, 5*time.Minute)
	if err := fake.MergePull("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "done")
	wantApplied(t, ts, "pr-merged pr.merged")
}

func TestRulesWaitForEveryLinkedPullRequest(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	for _, b := range []string{"hh/ME-1-a", "hh/ME-1-b"} {
		if _, err := fake.OpenPull("org-a/web", b, "A", "", false); err != nil {
			t.Fatal(err)
		}
	}
	fetch(t, ts)
	later(ts, 5*time.Minute)
	if err := fake.MergePull("org-a/web", 1); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "in_progress")
	later(ts, 5*time.Minute)
	if err := fake.MergePull("org-a/web", 2); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "done")
}

func TestRuleRefusedIsOnTheFetchCommit(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, func(sc *schema.Schema, _ *automation.File) {
		sc.Workflow.Required = schema.Lists{{Key: "in_progress", Values: []string{"estimate"}}}
	})
	x := ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if r := fetch(t, ts); !r.Committed {
		t.Fatalf("report = %+v", r)
	}
	wantStatus(t, ts, "ME-1", "todo")
	head := logOf(t, ts)[0]
	if op, _ := head.Trailer("Hedgehug-Op"); op != "gh-fetch" {
		t.Fatalf("op of HEAD = %s", op)
	}
	v, ok := head.Trailer(automation.TrailerRejected)
	if !ok {
		t.Fatalf("no trailer: %+v", head.Trailers)
	}
	u, rej, ok := automation.ParseRejected(v)
	if !ok || u != x.ULID || rej.Rule != "pr-opened" || !strings.Contains(rej.Reason, "estimate") {
		t.Errorf("trailer = %q", v)
	}
}

// The rule after a refused one still applies: the refusal is on the fetch and
// the status has its own commit.
func TestRuleAfterARefusedOne(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, func(sc *schema.Schema, f *automation.File) {
		sc.Workflow.Required = schema.Lists{{Key: "in_progress", Values: []string{"estimate"}}}
		f.Rules = append(f.Rules, automation.Rule{ID: "hold", On: automation.PROpened, Set: automation.Set{Status: "waiting"}, Enabled: true})
	})
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "waiting")
	wantApplied(t, ts, "hold pr.opened")
	if _, ok := logOf(t, ts)[1].Trailer(automation.TrailerRejected); !ok {
		t.Error("the refusal is not on the fetch")
	}
}

func TestRulesOfAProject(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	p := ts.AddTask("p", func(x *model.Task) { x.Kind = "project" })
	ts.AddTask("in", func(x *model.Task) { x.Parent, x.Repos = &p.ULID, []string{web} })
	ts.AddTask("out", func(x *model.Task) { x.Repos = []string{web} })
	devStore(t, ts, func(_ *schema.Schema, f *automation.File) {
		where := automation.WherePrefix + p.ULID.String()
		f.Rules = append(f.Rules, automation.Rule{ID: "hold", Where: where, On: automation.PROpened, Set: automation.Set{Status: "waiting"}, Enabled: true})
	})
	for _, b := range []string{"hh/ME-2-a", "hh/ME-3-a"} {
		if _, err := fake.OpenPull("org-a/web", b, "A", "", false); err != nil {
			t.Fatal(err)
		}
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-2", "waiting")
	wantStatus(t, ts, "ME-3", "in_progress")
	if n := countOp(logOf(t, ts), "set"); n != 2 {
		t.Errorf("status commits = %d", n)
	}
}

func TestUndoTakesBackOneRuleAndItDoesNotFireAgain(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, ts, "ME-1", "todo")
	if g, _ := taskIn(t, ts, "me", "ME-1").GitHub(); len(g.PRs) != 1 {
		t.Errorf("gh after undo = %+v", g)
	}
	later(ts, 5*time.Minute)
	if err := fake.CommentPull("org-a/web", 1, "ping"); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "todo")
	if n := countOp(logOf(t, ts), "set"); n != 1 {
		t.Errorf("status commits = %d", n)
	}
}

func TestRuleDoesNotFireAgainAfterAPersonMovedTheTaskBack(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "todo"); err != nil {
		t.Fatal(err)
	}
	later(ts, 5*time.Minute)
	if err := fake.CommentPull("org-a/web", 1, "ping"); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "todo")
}

func TestRulesLeaveADeletedTask(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	ts.AddDeleted("a", ts.Now(), func(x *model.Task) {
		x.Links = []model.Link{{Type: model.LinkPR, Ref: "github.com/org-a/web/pull/1", Sync: model.SyncMirror}}
	})
	ts.AddTask("b", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if n := countOp(logOf(t, ts), "set"); n != 0 {
		t.Errorf("status commits = %d", n)
	}
}

// A task deleted while GitHub is read is not written: what the fetch writes
// is worked out from the store as the lock finds it.
func TestRulesLeaveATaskDeletedDuringTheRead(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	x := ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	ts.AddTask("b", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.OpenPull("org-a/web", "hh/ME-2-b", "B", "", false); err != nil {
		t.Fatal(err)
	}
	ts.HTTPClient = duringRead(func() {
		ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
			y, err := tx.Task(x.ULID)
			if err != nil {
				return err
			}
			at := ts.Now()
			y.DeletedAt = &at
			return tx.PutTask(y)
		})
	})
	fetch(t, ts)
	y := ts.TaskByULID("me", x.ULID)
	if y.Status != "todo" || len(y.Links) != 0 {
		t.Errorf("deleted task = %s, %+v", y.Status, y.Links)
	}
	wantStatus(t, ts, "ME-2", "in_progress")
}

// The rules see what a person wrote while GitHub was read.
func TestRulesSeeAWriteMadeDuringTheRead(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	x := ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	ts.HTTPClient = duringRead(func() {
		ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
			y, err := tx.Task(x.ULID)
			if err != nil {
				return err
			}
			y.Status = "in_progress"
			return tx.PutTask(y)
		})
	})
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "in_progress")
	if n := countOp(logOf(t, ts), "set"); n != 0 {
		t.Errorf("status commits = %d", n)
	}
}

// The store lock is held from the fetch commit to the last status, and not
// while GitHub is read.
func TestFetchHoldsTheStoreLockOnlyToWrite(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	ts.HTTPClient = duringRead(func() {
		if !ts.LocksFree("me", "lock") {
			t.Error("the store lock is held during the read")
		}
	})
	writes := 0
	ts.BeforeUpdate = func() {
		writes++
		if ts.LocksFree("me", "lock") {
			t.Errorf("write %d: the store lock is not held", writes)
		}
	}
	fetch(t, ts)
	if writes != 2 {
		t.Errorf("writes = %d", writes)
	}
	if !ts.LocksFree("me", "lock") {
		t.Error("the store lock is held after the fetch")
	}
}

// When the lock is not to be had, what could not be read is noted all the same
// and what was read is read again.
func TestFetchNotesAFailedReadWhenTheLockIsHeld(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{"github.com/org-b/secret", web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	other, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	ts.HTTPClient = duringRead(func() {
		held, err := store.LockInOrder(other)
		if err != nil {
			t.Error(err)
			return
		}
		t.Cleanup(held.Release)
	})
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := integration.Fetch(ts.Session, c, e); err == nil {
		t.Fatal("the fetch did not fail")
	}
	st, err := ghstate.Read(other)
	if err != nil {
		t.Fatal(err)
	}
	if r := st.Repos["github.com/org-b/secret"]; r.LastTry.IsZero() || r.Warning == "" {
		t.Errorf("state = %+v", r)
	}
	if !st.Repos[web].LastFetch.IsZero() {
		t.Errorf("the repository whose result was lost moved: %+v", st.Repos[web])
	}
}

func TestUnlinkAndLinkAgainFireAgain(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	if _, err := tasks.SetStatus(ts.Session, "ME-1", "todo"); err != nil {
		t.Fatal(err)
	}
	pr := integration.LinkInput{PR: "org-a/web#1"}
	if _, err := integration.Unlink(ts.Session, "ME-1", pr); err != nil {
		t.Fatal(err)
	}
	if g, _ := taskIn(t, ts, "me", "ME-1").GitHub(); len(g.PRs) != 0 {
		t.Fatalf("gh after unlink = %+v", g)
	}
	if _, err := integration.Link(ts.Session, "ME-1", pr); err != nil {
		t.Fatal(err)
	}
	later(ts, 5*time.Minute)
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "in_progress")
	wantApplied(t, ts, "pr-opened pr.opened")
}

func TestABrokenAutomationsFileIsAWarning(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	ts.WriteAndCommit(model.AutomationsPath, "rules: 3\n")
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r := fetch(t, ts)
	if !r.Committed || len(r.Warnings) != 1 || strings.Count(r.Warnings[0], model.AutomationsPath) != 1 {
		t.Fatalf("report = %+v", r)
	}
	wantStatus(t, ts, "ME-1", "todo")
	if op := usecasetest.LastOp(t, ts); op != "gh-fetch" {
		t.Errorf("op = %s", op)
	}
}

func TestFetchRepoReadsOneRepositoryNow(t *testing.T) {
	t.Parallel()
	const api = "github.com/org-a/api"
	ts, fake := withFake(t)
	if err := fake.AddRepo("org-a/api", nil, false); err != nil {
		t.Fatal(err)
	}
	if err := fake.SetPerm("org-a/api", "alice", "write"); err != nil {
		t.Fatal(err)
	}
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web, api} })
	fetch(t, ts)
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.OpenPull("org-a/api", "hh/ME-1-b", "B", "", false); err != nil {
		t.Fatal(err)
	}
	later(ts, time.Minute) // within the interval
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	r, err := integration.FetchRepo(ts.Session, c, e, "github.com/org-a/WEB")
	if err != nil || !r.Committed {
		t.Fatalf("report = %+v, %v", r, err)
	}
	wantStatus(t, ts, "ME-1", "in_progress")
	if g, _ := taskIn(t, ts, "me", "ME-1").GitHub(); len(g.PRs) != 1 || !strings.Contains(g.PRs[0].Ref, "org-a/web") {
		t.Errorf("gh = %+v", g)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := ghstate.Read(s)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Repos[api].LastFetch.Before(st.Repos[web].LastFetch) {
		t.Errorf("api = %+v, web = %+v", st.Repos[api], st.Repos[web])
	}
}

// What the rules write goes out with the sync that fetched.
func TestSyncPushesWhatTheRulesWrote(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	usecasetest.AddBareRemote(t, ts)
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r, err := replication.SyncOne(ts.Session)
	if err != nil || r.Err != nil || !r.GitHubUpdated {
		t.Fatalf("sync: %+v, %v", r, err)
	}
	wantApplied(t, ts, "pr-opened pr.opened")
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if n, err := usecase.Unpushed(s); err != nil || n != 0 {
		t.Errorf("unpushed = %d, %v", n, err)
	}
}

var hostsRE = regexp.MustCompile(`(?m)^hosts:\n(?:  - .*\n)+`)

// A transition that another member's clone wrote does not fire here: the
// value of gh arrives with the sync, so there is no change to see.
func TestRuleDoesNotFireOnWhatAnotherCloneWrote(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	remote := usecasetest.AddBareRemote(t, ts)
	devStore(t, ts, nil)
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if r, err := replication.SyncOne(ts.Session); err != nil || r.Err != nil {
		t.Fatalf("sync: %+v, %v", r, err)
	}
	other := usecasetest.Clone(t, ts, remote, ts.Now())
	other.DropDefaultAccount()
	other.AppendConfig(hostsRE.FindString(usecasetest.ConfigText(t, ts)) +
		"accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	other.ForgetWriter()

	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	later(ts, 5*time.Minute)
	if r, err := replication.SyncOne(ts.Session); err != nil || r.Err != nil {
		t.Fatalf("sync: %+v, %v", r, err)
	}
	r, err := replication.SyncOne(other.Session)
	if err != nil || r.Err != nil {
		t.Fatalf("sync of the other: %+v, %v", r, err)
	}
	if len(r.Warnings) != 0 {
		t.Fatalf("warnings = %q", r.Warnings)
	}
	es := logOf(t, other)
	if n := countOp(es, "set"); n != 1 {
		t.Errorf("status commits = %d", n)
	}
	if n := countOp(es, "gh-fetch"); n != 1 {
		t.Errorf("fetch commits = %d", n)
	}
	wantStatus(t, other, "ME-1", "in_progress")
}

// A rule moved the task along a limited workflow: undo takes it back all the
// same, and it does not fire again.
func TestUndoOfARuleAgainstTheWorkflow(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, func(sc *schema.Schema, _ *automation.File) {
		sc.Workflow.Transitions = schema.Lists{{Key: "in_progress", Values: []string{"done"}}}
	})
	ts.AddTask("a", func(x *model.Task) { x.Repos = []string{web} })
	if _, err := fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "in_progress")
	if _, err := tasks.Undo(ts.Session); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, ts, "ME-1", "todo")
}

// Undo leaves gh as it is, so the link that undo took away may leave its item
// there: linking again is still a new item, and the rule fires.
func TestLinkAgainAfterUndoFiresAgain(t *testing.T) {
	t.Parallel()
	ts, fake := withFake(t)
	devStore(t, ts, nil)
	ts.AddTask("a", nil)
	if _, err := fake.OpenPull("org-a/web", "other", "A", "", false); err != nil {
		t.Fatal(err)
	}
	pr := integration.LinkInput{PR: "org-a/web#1"}
	if _, err := integration.Link(ts.Session, "ME-1", pr); err != nil {
		t.Fatal(err)
	}
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "in_progress")
	for range 2 { // the rule, then the link
		if _, err := tasks.Undo(ts.Session); err != nil {
			t.Fatal(err)
		}
	}
	wantStatus(t, ts, "ME-1", "todo")
	if _, err := integration.Link(ts.Session, "ME-1", pr); err != nil {
		t.Fatal(err)
	}
	later(ts, 6*time.Minute)
	fetch(t, ts)
	wantStatus(t, ts, "ME-1", "in_progress")
}
