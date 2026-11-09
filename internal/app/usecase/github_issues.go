package usecase

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// The ops of the commits that sync issues. Like gh-fetch they are no one's
// work: undoing one, the next sync would write the same again.
const (
	OpGitHubMirror = "gh-mirror"
	OpGitHubPush   = "gh-push"
)

// sortedLower is names in the form they compare in: GitHub does not tell
// logins apart by case. It is never nil, so that no list and an empty one are
// the same.
func sortedLower(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, strings.ToLower(n))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// IssueToken is what the state of an issue is as a status token.
func IssueToken(state, reason string) string {
	switch {
	case state != "closed":
		return automation.IssueOpen
	case reason == automation.IssueNotPlanned:
		return automation.IssueNotPlanned
	}
	return automation.IssueCompleted
}

// GitHubIssueValues is what x holds, as it compares.
func GitHubIssueValues(x github.Issue) model.IssueValues {
	return model.IssueValues{
		"title":     {strings.TrimSpace(x.Title)},
		"body":      {model.StripMark(x.Body)},
		"status":    {IssueToken(x.State, x.StateReason)},
		"assignees": sortedLower(x.Assignees),
		"milestone": {x.Milestone},
	}
}

// repoHost is the host of a repo ID.
func repoHost(repo string) string {
	host, _, _ := strings.Cut(repo, "/")
	return host
}

// loginsOn splits the accounts of a task by the host of a repository: the
// logins there, and the accounts of other hosts, which GitHub there cannot
// know.
func loginsOn(accounts []string, host string) (logins, others []string) {
	for _, a := range accounts {
		h, login, ok := strings.Cut(a, "/")
		if ok && strings.EqualFold(h, host) {
			logins = append(logins, login)
			continue
		}
		others = append(others, a)
	}
	return logins, others
}

// TaskIssueValues is what t holds for an issue of a repository on host, as it
// compares.
func TaskIssueValues(t *model.Task, rules *automation.File, e *schema.Effective, host string) model.IssueValues {
	st, _ := e.Schema().Status(t.Status)
	logins, _ := loginsOn(t.Assignees, host)
	milestone := ""
	if key := rules.IssueFields.Props["milestone"]; key != "" {
		if n := t.Props.Get(key); n != nil && n.Kind == yaml.ScalarNode && n.ShortTag() != "!!null" {
			milestone = n.Value
		}
	}
	return model.IssueValues{
		"title":     {strings.TrimSpace(t.Title)},
		"body":      {model.StripMark(t.Body)},
		"status":    {automation.StatusToken(st.Category)},
		"assignees": sortedLower(logins),
		"milestone": {milestone},
	}
}

// issueNote is a value that a sync of the issue ref wrote over.
type issueNote struct {
	ref string
	automation.Overwrite
}

// mirrorPlan is what mirror writes to one task, decided under the store lock.
type mirrorPlan struct {
	ulid        model.ULID
	set         model.IssueValues // the values in the task's own form
	milestone   string            // the key of props that holds the milestone
	status      string            // the status to move to, or ""
	change      *automation.Change
	rejected    []automation.Rejection
	synced      map[string]model.IssueValues // by ref
	overwritten []issueNote
	comments    *commentMirror // the comments to write, or nil
}

// write puts the plan's values and status on t.
func (p *mirrorPlan) write(t *model.Task) {
	if v, ok := p.set["title"]; ok {
		t.Title = v[0]
	}
	if v, ok := p.set["body"]; ok {
		t.Body = v[0]
	}
	if v, ok := p.set["assignees"]; ok {
		before := t.Assignees
		t.Assignees = slices.Clone(v)
		WatchNewAssignees(t, before)
	}
	if v, ok := p.set["milestone"]; ok {
		if v[0] == "" {
			t.Props.Delete(p.milestone)
		} else {
			t.Props.Set(p.milestone, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v[0]})
		}
	}
	if p.status != "" {
		t.Status = p.status
	}
}

// on is t as the plan leaves it, for what is judged after it. t stays.
func (p *mirrorPlan) on(t *model.Task) *model.Task {
	c := *t
	c.Props = slices.Clone(t.Props)
	c.Watchers = slices.Clone(t.Watchers)
	p.write(&c)
	return &c
}

// planMirror compares the issues that t mirrors and got has, one field at a
// time, and returns what to write; nil when t and its base are up to date.
// When two issues give the same field, the first link has it.
func planMirror(t *model.Task, got fetched, rules *automation.File, e *schema.Effective) (*mirrorPlan, error) {
	g, err := t.GitHub()
	if err != nil {
		return nil, err
	}
	p := &mirrorPlan{ulid: t.ULID, set: model.IssueValues{}, synced: map[string]model.IssueValues{}, milestone: rules.IssueFields.Props["milestone"]}
	taken := map[string]bool{}
	dirty := false
	var moved *struct {
		token []string
		news  bool
		note  *issueNote
	}
	for _, l := range t.Links {
		if l.Type != model.LinkIssue || l.Sync != model.SyncMirror {
			continue
		}
		// A link that was not read keeps its fields: the next link does not
		// get them for one fetch.
		var fields []string
		for _, f := range rules.IssueFields.Scalars(l) {
			if !taken[f] {
				taken[f] = true
				fields = append(fields, f)
			}
		}
		ref := model.CanonRef(l.Ref)
		x, read := got.issues[ref]
		at := slices.IndexFunc(g.Issues, func(i model.IssueState) bool { return model.CanonRef(i.Ref) == ref })
		if !read || at < 0 {
			continue
		}
		base, err := g.Issues[at].Values()
		if err != nil {
			return nil, err
		}
		host := repoHost(repoOf(ref))
		gh := GitHubIssueValues(x)
		r := automation.Reflect(model.SyncMirror, fields, TaskIssueValues(t, rules, e, host), gh, base)
		p.synced[ref] = r.Synced
		dirty = dirty || !maps.EqualFunc(base, r.Synced, slices.Equal)
		for _, o := range r.Overwritten {
			if o.Field != "status" {
				p.overwritten = append(p.overwritten, issueNote{ref, o})
			}
		}
		if v, ok := r.ToTask["status"]; ok {
			b, known := base["status"]
			moved = &struct {
				token []string
				news  bool
				note  *issueNote
			}{token: v, news: !known || !slices.Equal(b, v)}
			if i := slices.IndexFunc(r.Overwritten, func(o automation.Overwrite) bool { return o.Field == "status" }); i >= 0 {
				moved.note = &issueNote{ref, r.Overwritten[i]}
			}
			delete(r.ToTask, "status")
		}
		if v, ok := r.ToTask["assignees"]; ok {
			_, others := loginsOn(t.Assignees, host)
			list := slices.Clone(others)
			for _, login := range v {
				list = append(list, host+"/"+login)
			}
			r.ToTask["assignees"] = list
		}
		maps.Copy(p.set, r.ToTask)
	}
	if moved != nil {
		to, why := mirrorStatus(moved.token[0], t.Status, p.on(t), e)
		switch {
		case why == "":
			p.status = to
			p.change = &automation.Change{Rule: automation.MirrorRule, Event: automation.MirrorEvent, Status: to}
			if moved.note != nil {
				p.overwritten = append(p.overwritten, *moved.note)
			}
		case moved.news:
			p.rejected = append(p.rejected, automation.Rejection{Rule: automation.MirrorRule, Reason: why})
		}
	}
	if !dirty && len(p.set) == 0 && p.status == "" {
		return nil, nil //nolint:nilnil // nothing to write is no error
	}
	return p, nil
}

// mirrorStatus is the status that an issue in the state token gives a task in
// from, or why it cannot go there. view is the task as it will be then.
func mirrorStatus(token, from string, view *model.Task, e *schema.Effective) (to, why string) {
	c := automation.TokenCategory(token)
	d, ok := e.Schema().DefaultStatus(c)
	if !ok {
		return "", fmt.Sprintf("category %s has no default status", c)
	}
	if vs := schema.ValidateTransition(from, d.Name, view, e); len(vs) > 0 {
		return "", automation.Joined(vs)
	}
	return d.Name, ""
}

// noteOverwrites puts the values a commit writes over on it.
func noteOverwrites(tx *store.Tx, u model.ULID, notes []issueNote) {
	for _, n := range notes {
		tx.AddTrailer(automation.TrailerIssueOverwritten, automation.IssueOverwrittenTrailer(u, n.ref, n.Overwrite))
	}
}

// putSynced writes the values to gh of t, by ref.
func putSynced(t *model.Task, synced map[string]model.IssueValues) error {
	g, err := t.GitHub()
	if err != nil {
		return err
	}
	for i := range g.Issues {
		if v, ok := synced[model.CanonRef(g.Issues[i].Ref)]; ok {
			if err := g.Issues[i].SetValues(v); err != nil {
				return err
			}
		}
	}
	return t.SetGitHub(g)
}

// applyMirror writes each plan as a commit of its own, under the hold that it
// was planned under. The values and the base go together: a task that cannot
// take them is a warning, and is compared again by the next fetch. It returns
// the tasks that it could not write.
func applyMirror(ss *Session, s *store.Store, held *store.Locked, plans []*mirrorPlan, rep *GitHubFetchReport) map[model.ULID]bool {
	failed := map[model.ULID]bool{}
	for _, p := range plans {
		op := store.Op{Name: OpGitHubMirror, Task: p.ulid, Automated: true, SkipTransitions: true, Locked: held}
		hash, err := ss.UpdateCommit(s, op, func(tx *store.Tx) error {
			t, err := tx.Task(p.ulid)
			if err != nil {
				return err
			}
			p.write(t)
			if err := putSynced(t, p.synced); err != nil {
				return err
			}
			if p.change != nil {
				tx.AddTrailer(automation.TrailerApplied, automation.AppliedTrailer(*p.change))
			}
			for _, r := range p.rejected {
				tx.AddTrailer(automation.TrailerRejected, automation.RejectedTrailer(p.ulid, r))
			}
			noteOverwrites(tx, p.ulid, p.overwritten)
			if p.comments != nil {
				noteOverwrites(tx, p.ulid, p.comments.notes)
				for _, c := range p.comments.puts {
					if err := tx.PutCommentAt(p.ulid, c); err != nil {
						return err
					}
				}
				for _, c := range p.comments.ghOnly {
					if err := tx.PutCommentGH(p.ulid, c); err != nil {
						return err
					}
				}
			}
			return tx.PutTask(t)
		})
		if err != nil {
			failed[p.ulid] = true
			rep.Warnings = append(rep.Warnings, p.ulid.String()+": "+err.Error())
			continue
		}
		rep.Committed = rep.Committed || !hash.IsZero()
	}
	return failed
}

// closedHere drops from next the issues that close by mirroring the status of
// a task as not planned: the task is canceled, and the rules of issue.closed,
// which say what closing means, are not for it. It would be moved on, and be
// moved back by the next mirror as if a person had edited it.
func closedHere(t *model.Task, rules *automation.File, next model.GitHubState) model.GitHubState {
	mirrors := func(ref string) bool {
		for _, l := range t.Links {
			if l.Type == model.LinkIssue && l.Sync == model.SyncMirror && slices.Contains(rules.IssueFields.For(l), "status") {
				return model.CanonRef(l.Ref) == ref // the first such link has the status
			}
		}
		return false
	}
	out := next
	out.Issues = slices.DeleteFunc(slices.Clone(next.Issues), func(x model.IssueState) bool {
		return x.State == "closed" && x.StateReason != nil && *x.StateReason == automation.IssueNotPlanned && mirrors(model.CanonRef(x.Ref))
	})
	return out
}
