package automation

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

func pr(ref, state, review string) model.PRState {
	return model.PRState{Ref: ref, State: state, Review: review}
}

func gh(prs []model.PRState, issues ...model.IssueState) model.GitHubState {
	return model.GitHubState{PRs: prs, Issues: issues}
}

func TestEventsOf(t *testing.T) {
	t.Parallel()
	const a, b = "github.com/o/r/pull/1", "github.com/o/r/pull/2"
	issue := func(state string) model.IssueState {
		return model.IssueState{Ref: "github.com/o/r/issues/5", State: state}
	}
	for _, c := range []struct {
		name      string
		old, next model.GitHubState
		want      []string
	}{
		{"nothing", gh(nil), gh(nil), nil},
		{"a new open PR", gh(nil), gh([]model.PRState{pr(a, "open", "none")}), []string{PROpened}},
		{"a new draft PR", gh(nil), gh([]model.PRState{pr(a, "draft", "none")}), []string{PROpened}},
		{"a new closed PR", gh(nil), gh([]model.PRState{pr(a, "closed", "none")}), nil},
		{"a new merged PR", gh(nil), gh([]model.PRState{pr(a, "merged", "approved")}), []string{PRMerged}},
		{"a new PR that is asked for review", gh(nil), gh([]model.PRState{pr(a, "open", "pending")}), []string{PROpened, PRReviewRequested}},
		{"a PR that stays open", gh([]model.PRState{pr(a, "open", "none")}), gh([]model.PRState{pr(a, "open", "none")}), nil},
		{"draft to open", gh([]model.PRState{pr(a, "draft", "none")}), gh([]model.PRState{pr(a, "open", "none")}), nil},
		{"none to pending", gh([]model.PRState{pr(a, "open", "none")}), gh([]model.PRState{pr(a, "open", "pending")}), []string{PRReviewRequested}},
		{"approved to pending", gh([]model.PRState{pr(a, "open", "approved")}), gh([]model.PRState{pr(a, "open", "pending")}), nil},
		{"changes requested to pending", gh([]model.PRState{pr(a, "open", "changes_requested")}), gh([]model.PRState{pr(a, "open", "pending")}), nil},
		{"pending to none", gh([]model.PRState{pr(a, "open", "pending")}), gh([]model.PRState{pr(a, "open", "none")}), nil},
		{"open to merged", gh([]model.PRState{pr(a, "open", "approved")}), gh([]model.PRState{pr(a, "merged", "approved")}), []string{PRMerged}},
		{"merged stays merged", gh([]model.PRState{pr(a, "merged", "approved")}), gh([]model.PRState{pr(a, "merged", "approved")}), nil},
		{"open to closed", gh([]model.PRState{pr(a, "open", "none")}), gh([]model.PRState{pr(a, "closed", "none")}), nil},
		{"the PR is gone", gh([]model.PRState{pr(a, "open", "none")}), gh(nil), nil},
		{"the ref in other letters", gh([]model.PRState{pr("github.com/O/R/pull/1", "open", "none")}), gh([]model.PRState{pr(a, "open", "none")}), nil},
		{"two PRs open", gh(nil), gh([]model.PRState{pr(a, "open", "none"), pr(b, "open", "none")}), []string{PROpened}},
		{"two PRs merge", gh([]model.PRState{pr(a, "open", "none"), pr(b, "open", "none")}), gh([]model.PRState{pr(a, "merged", "none"), pr(b, "merged", "none")}), []string{PRMerged}},
		{"one opens and one merges", gh([]model.PRState{pr(a, "open", "none")}), gh([]model.PRState{pr(a, "merged", "none"), pr(b, "open", "none")}), []string{PROpened, PRMerged}},
		{"a new open issue", gh(nil), gh(nil, issue("open")), nil},
		{"a new closed issue", gh(nil), gh(nil, issue("closed")), []string{IssueClosed}},
		{"an issue closes", gh(nil, issue("open")), gh(nil, issue("closed")), []string{IssueClosed}},
		{"an issue stays closed", gh(nil, issue("closed")), gh(nil, issue("closed")), nil},
		{"all of them", gh(nil), gh([]model.PRState{pr(a, "open", "pending")}, issue("closed")), []string{PROpened, PRReviewRequested, IssueClosed}},
	} {
		if got := EventsOf(c.old, c.next); !slices.Equal(got, c.want) {
			t.Errorf("%s: events = %v, want %v", c.name, got, c.want)
		}
	}
}

// taskWith makes a todo task that links the PRs, with gh holding their states.
func taskWith(t *testing.T, status string, prs ...model.PRState) *model.Task {
	t.Helper()
	task := &model.Task{ULID: model.NewULID(), Kind: "task", Title: "t", Status: status}
	for _, p := range prs {
		task.Links = append(task.Links, model.Link{Type: model.LinkPR, Ref: p.Ref, Sync: model.SyncMirror})
	}
	if err := task.SetGitHub(model.GitHubState{PRs: prs}); err != nil {
		t.Fatal(err)
	}
	return task
}

const prA, prB = "github.com/o/r/pull/1", "github.com/o/r/pull/2"

func evaluate(t *testing.T, src string, sc *schema.Schema, task *model.Task, ancestors []model.ULID, events ...string) Outcome {
	t.Helper()
	return mustDecode(t, src).Evaluate(Input{Events: events, Task: task, Ancestors: ancestors, Schema: sc})
}

func TestEvaluateBasics(t *testing.T) {
	t.Parallel()
	sc := schema.Dev()
	const rules = `
rules:
- {id: opened, on: pr.opened, set: {status: in_progress}}
- {id: review, on: pr.review_requested, set: {status: review}}
- {id: merged, on: pr.merged, when: all_linked_prs_merged, set: {category: done}}
- {id: closed, on: issue.closed, set: {category: done}}
`
	task := func(status string) *model.Task { return taskWith(t, status, pr(prA, "merged", "approved")) }
	for _, c := range []struct {
		name   string
		status string
		events []string
		want   *Change
	}{
		{"a status", "todo", []string{PROpened}, &Change{"opened", PROpened, "in_progress"}},
		{"a category takes its default", "in_progress", []string{PRMerged}, &Change{"merged", PRMerged, "done"}},
		{"the status is already the target", "in_progress", []string{PROpened}, nil},
		{"the status is already in the category", "done", []string{PRMerged}, nil},
		{"a status of the category that is not the default", "canceled", []string{IssueClosed}, &Change{"closed", IssueClosed, "done"}},
		{"no rule for the event", "todo", nil, nil},
		{"two events, in the order given", "todo", []string{PROpened, PRReviewRequested}, &Change{"review", PRReviewRequested, "review"}},
		{"opened and merged", "todo", []string{PROpened, PRMerged}, &Change{"merged", PRMerged, "done"}},
	} {
		out := evaluate(t, rules, sc, task(c.status), nil, c.events...)
		if len(out.Rejected) != 0 || !changeEqual(out.Change, c.want) {
			t.Errorf("%s: %+v (%v), want %+v", c.name, out.Change, out.Rejected, c.want)
		}
	}
}

func changeEqual(a, b *Change) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestEvaluateCategoryOfAnotherDoneStatus(t *testing.T) {
	t.Parallel()
	sc := schema.Dev()
	sc.Statuses = append(sc.Statuses, schema.Status{Name: "shipped", Category: schema.CategoryDone})
	out := evaluate(t, "rules:\n- {id: r, on: pr.merged, set: {category: done}}\n", sc, taskWith(t, "shipped", pr(prA, "merged", "none")), nil, PRMerged)
	if out.Change != nil || len(out.Rejected) != 0 {
		t.Errorf("out = %+v", out)
	}
}

func TestEvaluateLaterRuleWins(t *testing.T) {
	t.Parallel()
	proj := model.NewULID()
	where := "project:" + proj.String()
	src := `
rules:
- {id: release-review, where: '` + where + `', on: pr.review_requested, set: {status: waiting}}
- {id: review, on: pr.review_requested, set: {status: review}}
`
	task := taskWith(t, "in_progress", pr(prA, "open", "pending"))
	// The rule of the project comes after the rule of the store, wherever it is in the file.
	if out := evaluate(t, src, schema.Dev(), task, []model.ULID{proj}, PRReviewRequested); out.Change == nil || out.Change.Rule != "release-review" || out.Change.Status != "waiting" {
		t.Errorf("in the project: %+v", out)
	}
	// Elsewhere only the store's rule applies.
	if out := evaluate(t, src, schema.Dev(), task, []model.ULID{model.NewULID()}, PRReviewRequested); out.Change == nil || out.Change.Rule != "review" {
		t.Errorf("elsewhere: %+v", out)
	}
	if out := evaluate(t, src, schema.Dev(), task, nil, PRReviewRequested); out.Change == nil || out.Change.Rule != "review" {
		t.Errorf("no project: %+v", out)
	}
}

func TestEvaluateOffRules(t *testing.T) {
	t.Parallel()
	proj := model.NewULID()
	where := "project:" + proj.String()
	src := `
rules:
- {id: off, on: pr.opened, set: {status: review}, enabled: false}
- {id: off-here, on: pr.opened, set: {status: waiting}}
- {id: on, on: pr.opened, set: {status: in_progress}}
scoped:
- {where: '` + where + `', disable: [on]}
`
	task := taskWith(t, "todo", pr(prA, "open", "none"))
	if out := evaluate(t, src, schema.Dev(), task, []model.ULID{proj}, PROpened); out.Change == nil || out.Change.Rule != "off-here" {
		t.Errorf("in the project: %+v", out)
	}
	if out := evaluate(t, src, schema.Dev(), task, nil, PROpened); out.Change == nil || out.Change.Rule != "on" {
		t.Errorf("elsewhere: %+v", out)
	}
	deleted := taskWith(t, "todo", pr(prA, "open", "none"))
	at := time.Now()
	deleted.DeletedAt = &at
	if out := evaluate(t, src, schema.Dev(), deleted, nil, PROpened); out.Change != nil || len(out.Rejected) != 0 {
		t.Errorf("deleted: %+v", out)
	}
}

func TestEvaluateWhenAllLinkedPRsMerged(t *testing.T) {
	t.Parallel()
	const src = "rules:\n- {id: m, on: pr.merged, when: all_linked_prs_merged, set: {category: done}}\n"
	ok := func(name string, task *model.Task, want bool) {
		t.Helper()
		out := evaluate(t, src, schema.Dev(), task, nil, PRMerged)
		if (out.Change != nil) != want {
			t.Errorf("%s: %+v", name, out)
		}
	}
	ok("one PR merged", taskWith(t, "review", pr(prA, "merged", "none")), true)
	ok("two PRs merged", taskWith(t, "review", pr(prA, "merged", "none"), pr(prB, "merged", "none")), true)
	ok("one still open", taskWith(t, "review", pr(prA, "merged", "none"), pr(prB, "open", "none")), false)
	ok("one closed", taskWith(t, "review", pr(prA, "merged", "none"), pr(prB, "closed", "none")), false)

	notFetched := taskWith(t, "review", pr(prA, "merged", "none"))
	notFetched.Links = append(notFetched.Links, model.Link{Type: model.LinkPR, Ref: prB, Sync: model.SyncMirror})
	ok("a PR that is not in gh yet", notFetched, false)

	unlinked := taskWith(t, "review", pr(prA, "merged", "none"), pr(prB, "closed", "none"))
	unlinked.Links[1].Sync = model.SyncOff
	ok("a PR that is off", unlinked, true)

	issueOnly := taskWith(t, "review")
	issueOnly.Links = []model.Link{{Type: model.LinkIssue, Ref: "github.com/o/r/issues/5", Sync: model.SyncLink}}
	ok("no PR is linked", issueOnly, false)

	upper := taskWith(t, "review", pr(prA, "merged", "none"))
	upper.Links[0].Ref = "github.com/O/R/pull/1"
	ok("the link in other letters", upper, true)
}

func TestEvaluateRejections(t *testing.T) {
	t.Parallel()
	proj := model.NewULID()
	sc := schema.Dev()
	sc.Scoped = []schema.Scoped{{
		Where:    "project:" + proj.String(),
		Workflow: &schema.Workflow{Transitions: schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}},
	}}
	const src = "rules:\n- {id: m, on: pr.merged, set: {category: done}}\n- {id: o, on: pr.opened, set: {status: in_progress}}\n"
	task := taskWith(t, "todo", pr(prA, "merged", "none"))

	// The project's workflow does not let todo go to done.
	out := evaluate(t, src, sc, task, []model.ULID{proj}, PRMerged)
	if out.Change != nil || len(out.Rejected) != 1 || out.Rejected[0].Rule != "m" || !strings.Contains(out.Rejected[0].Reason, "todo") {
		t.Errorf("out = %+v", out)
	}
	// The store does.
	if out := evaluate(t, src, sc, task, nil, PRMerged); out.Change == nil || len(out.Rejected) != 0 {
		t.Errorf("out = %+v", out)
	}
	// A rejected rule is skipped, and the events after it go on, from the value before.
	out = evaluate(t, src, sc, task, []model.ULID{proj}, PRMerged, PROpened)
	if out.Change == nil || out.Change.Rule != "o" || len(out.Rejected) != 1 || out.Rejected[0].Rule != "m" {
		t.Errorf("out = %+v", out)
	}
	// opened then merged passes through in_progress, which the workflow allows
	// only up to there: in_progress to done is free.
	out = evaluate(t, src, sc, task, []model.ULID{proj}, PROpened, PRMerged)
	if out.Change == nil || out.Change.Rule != "m" || out.Change.Status != "done" || len(out.Rejected) != 0 {
		t.Errorf("out = %+v", out)
	}
}

func TestEvaluateRequiredFields(t *testing.T) {
	t.Parallel()
	sc := schema.Dev()
	sc.Workflow.Required = schema.Lists{{Key: "review", Values: []string{"assignees"}}}
	const src = "rules:\n- {id: r, on: pr.review_requested, set: {status: review}}\n"
	task := taskWith(t, "in_progress", pr(prA, "open", "pending"))
	out := evaluate(t, src, sc, task, nil, PRReviewRequested)
	if out.Change != nil || len(out.Rejected) != 1 || !strings.Contains(out.Rejected[0].Reason, "assignees") {
		t.Errorf("out = %+v", out)
	}
}

func TestEvaluateBadTargets(t *testing.T) {
	t.Parallel()
	const src = "rules:\n- {id: s, on: pr.opened, set: {status: nope}}\n- {id: c, on: pr.opened, set: {category: nope}}\n- {id: n, on: pr.opened}\n- {id: ok, on: pr.opened, set: {status: in_progress}}\n"
	out := evaluate(t, src, schema.Dev(), taskWith(t, "todo", pr(prA, "open", "none")), nil, PROpened)
	if out.Change == nil || out.Change.Rule != "ok" {
		t.Errorf("change = %+v", out.Change)
	}
	var ids []string
	for _, r := range out.Rejected {
		ids = append(ids, r.Rule)
		if r.Reason == "" || strings.Contains(r.Reason, "\n") {
			t.Errorf("reason = %q", r.Reason)
		}
	}
	if !slices.Equal(ids, []string{"s", "c", "n"}) {
		t.Errorf("rejected = %v", ids)
	}
}

func TestEvaluateBackToWhereItWas(t *testing.T) {
	t.Parallel()
	const src = "rules:\n- {id: a, on: pr.opened, set: {status: in_progress}}\n- {id: b, on: pr.merged, set: {status: todo}}\n"
	out := evaluate(t, src, schema.Dev(), taskWith(t, "todo", pr(prA, "merged", "none")), nil, PROpened, PRMerged)
	if out.Change != nil || len(out.Rejected) != 0 {
		t.Errorf("out = %+v", out)
	}
}

func TestTrailers(t *testing.T) {
	t.Parallel()
	c := Change{Rule: "pr-merged", Event: PRMerged, Status: "done"}
	v := AppliedTrailer(c)
	if v != "pr-merged pr.merged" {
		t.Errorf("applied = %q", v)
	}
	if rule, event, ok := ParseApplied(v); !ok || rule != "pr-merged" || event != PRMerged {
		t.Errorf("parsed = %q %q %v", rule, event, ok)
	}
	for _, bad := range []string{"", "x", " pr.merged", "pr-merged "} {
		if _, _, ok := ParseApplied(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}

	u := model.NewULID()
	for _, reason := range []string{
		"cannot move from todo to done",
		"two  spaces and\ttab",
		"line one\nline two\r\nline three",
		`a back\slash and \n that is not a break`,
		"",
	} {
		v := RejectedTrailer(u, Rejection{Rule: "r1", Reason: reason})
		if strings.ContainsAny(v, "\r\n") {
			t.Errorf("trailer %q is not one line", v)
		}
		gu, r, ok := ParseRejected(v)
		if !ok || gu != u || r.Rule != "r1" || r.Reason != reason {
			t.Errorf("%q: parsed %v %+v %v", reason, gu, r, ok)
		}
	}
	for _, bad := range []string{"", "x", u.String(), u.String() + " ", "nope r1 reason", " r1 reason"} {
		if _, _, ok := ParseRejected(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}
