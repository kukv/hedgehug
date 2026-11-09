package automation

import (
	"errors"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// The states of gh that the events read.
const (
	stateOpen   = "open"
	stateDraft  = "draft"
	stateMerged = "merged"
	stateClosed = "closed"

	reviewNone    = "none"
	reviewPending = "pending"
)

// EventsOf lists what happened between the two values of gh, in the order of
// Events, each name once. An item that old lacks has no state and no review
// ("none"), so a PR seen for the first time as merged is a pr.merged.
func EventsOf(old, next model.GitHubState) []string {
	prs := make(map[string]model.PRState, len(old.PRs))
	for _, p := range old.PRs {
		prs[model.CanonRef(p.Ref)] = p
	}
	issues := make(map[string]model.IssueState, len(old.Issues))
	for _, i := range old.Issues {
		issues[model.CanonRef(i.Ref)] = i
	}
	fired := map[string]bool{}
	for _, p := range next.PRs {
		o, had := prs[model.CanonRef(p.Ref)]
		if !had && (p.State == stateOpen || p.State == stateDraft) {
			fired[PROpened] = true
		}
		if p.Review == reviewPending && (o.Review == "" || o.Review == reviewNone) {
			fired[PRReviewRequested] = true
		}
		if p.State == stateMerged && o.State != stateMerged {
			fired[PRMerged] = true
		}
	}
	for _, i := range next.Issues {
		if i.State == stateClosed && issues[model.CanonRef(i.Ref)].State != stateClosed {
			fired[IssueClosed] = true
		}
	}
	var out []string
	for _, e := range Events {
		if fired[e] {
			out = append(out, e)
		}
	}
	return out
}

// Input is what Evaluate needs: the events of one task, the task with the
// gh it has after the fetch, its ancestors (the projects among them select
// the where of the rules), and the schema. Status is where the task is when
// the rules start, when that is not Task.Status: the sync of an issue moved it
// before them.
type Input struct {
	Events    []string
	Task      *model.Task
	Status    string
	Ancestors []model.ULID
	Schema    *schema.Schema
}

// Change is the status that the rules gave a task: the rule that gave it
// last, and the event it was on.
type Change struct{ Rule, Event, Status string }

// Rejection is a rule that could not be applied, and why.
type Rejection struct{ Rule, Reason string }

// Outcome is what the rules did to one task.
type Outcome struct {
	Change   *Change // nil when the status stays
	Rejected []Rejection
}

// Evaluate applies the rules to the events of one task, in the order of the
// events and then of the rules. Each rule is checked against the status the
// rules before it gave, so a path of moves is judged one step at a time. A
// rule that is refused is recorded and skipped.
func (f *File) Evaluate(in Input) Outcome {
	var out Outcome
	if in.Task.DeletedAt != nil {
		return out
	}
	e := in.Schema.For(in.Ancestors)
	start := in.Status
	if start == "" {
		start = in.Task.Status
	}
	cur := start
	for _, event := range in.Events {
		for _, r := range f.rulesFor(event, in.Ancestors) {
			if r.When == AllLinkedPRsMerged && !allMerged(in.Task) {
				continue
			}
			to, skip, err := target(r, cur, e)
			if err != nil {
				out.Rejected = append(out.Rejected, Rejection{r.ID, err.Error()})
				continue
			}
			if skip {
				continue
			}
			if vs := schema.ValidateTransition(cur, to, in.Task, e); len(vs) > 0 {
				out.Rejected = append(out.Rejected, Rejection{r.ID, Joined(vs)})
				continue
			}
			cur = to
			out.Change = &Change{r.ID, event, to}
		}
	}
	if out.Change != nil && out.Change.Status == start {
		out.Change = nil
	}
	return out
}

// rulesFor lists the rules that are on for the event: the rules of the store
// first, then those of projects among ancestors, each in file order. It leaves
// out the rules that are off, and those that a project among ancestors turns
// off.
func (f *File) rulesFor(event string, ancestors []model.ULID) []Rule {
	within := func(where string) bool {
		s, ok := strings.CutPrefix(where, WherePrefix)
		if !ok {
			return false
		}
		u, err := model.ParseULID(s)
		return err == nil && slices.Contains(ancestors, u)
	}
	var disabled []string
	for _, s := range f.Scoped {
		if within(s.Where) {
			disabled = append(disabled, s.Disable...)
		}
	}
	var store, scoped []Rule
	for _, r := range f.Rules {
		if !r.Enabled || r.On != event || slices.Contains(disabled, r.ID) {
			continue
		}
		switch {
		case r.Where == "":
			store = append(store, r)
		case within(r.Where):
			scoped = append(scoped, r)
		}
	}
	return append(store, scoped...)
}

// target is the status that r gives a task now in cur. skip is true when the
// task is there already.
func target(r Rule, cur string, e *schema.Effective) (to string, skip bool, err error) {
	switch {
	case r.Set.Status != "" && r.Set.Category != "":
		return "", false, errors.New("set takes status or category, not both")
	case r.Set.Status != "":
		return r.Set.Status, r.Set.Status == cur, nil
	case r.Set.Category != "":
		c := schema.Category(r.Set.Category)
		if st, ok := e.Schema().Status(cur); ok && st.Category == c {
			return cur, true, nil
		}
		d, ok := e.Schema().DefaultStatus(c)
		if !ok {
			return "", false, errors.New("category " + r.Set.Category + " has no default status")
		}
		return d.Name, d.Name == cur, nil
	}
	return "", false, errors.New("set needs a status or a category")
}

// allMerged reports whether the task links at least one PR that is not off,
// and every such PR is merged in its gh.
func allMerged(t *model.Task) bool {
	g, err := t.GitHub()
	if err != nil {
		return false
	}
	state := make(map[string]string, len(g.PRs))
	for _, p := range g.PRs {
		state[model.CanonRef(p.Ref)] = p.State
	}
	linked := 0
	for _, l := range t.Links {
		if l.Type != model.LinkPR || !l.Live() {
			continue
		}
		linked++
		if state[model.CanonRef(l.Ref)] != stateMerged {
			return false
		}
	}
	return linked > 0
}

// Joined is the reasons of vs on one line.
func Joined(vs []schema.Violation) string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = v.String()
	}
	return strings.Join(s, "; ")
}

// OpGitHubMirror is the Hedgehug-Op of the commits that mirror GitHub into the
// store. It is here because the inbox, which cannot import usecase, tells of
// what these commits take in even to the one who ran them.
const OpGitHubMirror = "gh-mirror"

// The trailers that the commits of the automations carry.
const (
	TrailerApplied  = "Hedgehug-Automation"
	TrailerRejected = "Hedgehug-Automation-Rejected"
)

// AppliedTrailer is the value of TrailerApplied: "<rule> <event>".
func AppliedTrailer(c Change) string { return c.Rule + " " + c.Event }

// ParseApplied reads the value of TrailerApplied.
func ParseApplied(v string) (rule, event string, ok bool) {
	i := strings.LastIndexByte(v, ' ')
	if i <= 0 || i == len(v)-1 {
		return "", "", false
	}
	return v[:i], v[i+1:], true
}

var (
	escapeReason   = strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`)
	unescapeReason = strings.NewReplacer(`\\`, `\`, `\n`, "\n", `\r`, "\r")
)

// RejectedTrailer is the value of TrailerRejected: "<ULID> <rule> <reason>".
// The reason is on one line: its line breaks and backslashes are escaped.
func RejectedTrailer(u model.ULID, r Rejection) string {
	return u.String() + " " + r.Rule + " " + escapeReason.Replace(r.Reason)
}

// ParseRejected reads the value of TrailerRejected.
func ParseRejected(v string) (u model.ULID, r Rejection, ok bool) {
	first, rest, _ := strings.Cut(v, " ")
	u, err := model.ParseULID(first)
	if err != nil {
		return model.ULID{}, Rejection{}, false
	}
	rule, reason, _ := strings.Cut(rest, " ")
	if rule == "" {
		return model.ULID{}, Rejection{}, false
	}
	return u, Rejection{Rule: rule, Reason: unescapeReason.Replace(reason)}, true
}
