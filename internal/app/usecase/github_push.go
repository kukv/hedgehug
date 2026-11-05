package usecase

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// IssueResolver knows the names of one repository: it reads each list when it
// is first asked for, and keeps it for one sync.
type IssueResolver struct {
	cl          *github.Client
	repo        string
	owner, name string
	labels      map[string]string // by the lower case, as GitHub spells them
	logins      map[string]string
	milestones  map[string]int
}

// NewIssueResolver makes the resolver of the repository repo, a repo ID.
func NewIssueResolver(cl *github.Client, repo string) *IssueResolver {
	parts := strings.SplitN(repo, "/", 3)
	return &IssueResolver{cl: cl, repo: repo, owner: parts[1], name: parts[2]}
}

// names returns the list of a field of the task, by the lower case.
func (r *IssueResolver) names(field string) (map[string]string, error) {
	spelled := func(list []string) map[string]string {
		m := make(map[string]string, len(list))
		for _, s := range list {
			m[strings.ToLower(s)] = s
		}
		return m
	}
	if field == "labels" {
		if r.labels == nil {
			list, err := r.cl.Labels(r.owner, r.name)
			if err != nil {
				return nil, err
			}
			r.labels = spelled(list)
		}
		return r.labels, nil
	}
	if r.logins == nil {
		list, err := r.cl.Assignees(r.owner, r.name)
		if err != nil {
			return nil, err
		}
		r.logins = spelled(list)
	}
	return r.logins, nil
}

func (r *IssueResolver) milestone(title string) (int, bool, error) {
	if r.milestones == nil {
		list, err := r.cl.Milestones(r.owner, r.name)
		if err != nil {
			return 0, false, err
		}
		r.milestones = make(map[string]int, len(list))
		for _, m := range list {
			r.milestones[m.Title] = m.Number
		}
	}
	n, ok := r.milestones[title]
	return n, ok, nil
}

// Filter takes out of vals, for the fields, the names the repository does not
// know, and returns a warning for each. Nothing is created. A value that is
// the same as one of known came from GitHub or got there, and is not looked
// up. A milestone that is not there leaves gh's.
func (r *IssueResolver) Filter(vals model.IssueValues, fields []string, gh model.IssueValues, known ...model.IssueValues) ([]string, error) {
	var warnings []string
	seen := func(f string) bool {
		return slices.ContainsFunc(append(known, gh), func(k model.IssueValues) bool {
			v, ok := k[f]
			return ok && slices.Equal(v, vals[f])
		})
	}
	for _, f := range fields {
		if !model.IsIssueList(f) && f != "milestone" || seen(f) {
			continue
		}
		if f == "milestone" {
			title := vals[f][0]
			if title == "" {
				continue
			}
			_, ok, err := r.milestone(title)
			if err != nil {
				return nil, err
			}
			if !ok {
				warnings = append(warnings, fmt.Sprintf("%s has no milestone %q: left out", r.repo, title))
				vals[f] = []string{""}
				if v, had := gh[f]; had {
					vals[f] = v
				}
			}
			continue
		}
		names, err := r.names(f)
		if err != nil {
			return nil, err
		}
		kept := make([]string, 0, len(vals[f]))
		for _, s := range vals[f] {
			if _, ok := names[s]; ok {
				kept = append(kept, s)
				continue
			}
			warnings = append(warnings, fmt.Sprintf("%s has no %s %q: left out", r.repo, strings.TrimSuffix(f, "s"), s))
		}
		vals[f] = kept
	}
	return warnings, nil
}

// spell gives the names of a field as GitHub spells them.
func (r *IssueResolver) spell(field string, list []string) ([]string, error) {
	names, err := r.names(field)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, s := range list {
		if n, ok := names[s]; ok {
			out = append(out, n)
			continue
		}
		out = append(out, s) // GitHub answers for it, and the sync tries again
	}
	return out, nil
}

// IssueState is the state and the reason of a status token, as GitHub takes
// them.
func IssueState(token string) (state, reason string) {
	if token == automation.IssueOpen {
		return "open", ""
	}
	return "closed", token
}

// Update makes the change of the values to, for the issue of the task u.
func (r *IssueResolver) Update(u model.ULID, to model.IssueValues) (github.IssueUpdate, error) {
	var out github.IssueUpdate
	if v, ok := to["title"]; ok {
		out.Title = &v[0]
	}
	if v, ok := to["body"]; ok {
		body := model.MarkedBody(u, v[0])
		out.Body = &body
	}
	if v, ok := to["status"]; ok {
		state, reason := IssueState(v[0])
		out.State = &state
		if reason != "" {
			out.StateReason = &reason
		}
	}
	for field, dst := range map[string]**[]string{"assignees": &out.Assignees, "labels": &out.Labels} {
		if v, ok := to[field]; ok {
			list, err := r.spell(field, v)
			if err != nil {
				return out, err
			}
			*dst = &list
		}
	}
	if v, ok := to["milestone"]; ok {
		n := 0
		if v[0] != "" {
			var err error
			if n, _, err = r.milestone(v[0]); err != nil {
				return out, err
			}
		}
		out.Milestone = &n
	}
	return out, nil
}

// pushed is what a push of one issue leaves to write to gh.
type pushed struct {
	ref    string
	synced model.IssueValues
	state  string // what GitHub now holds, when this push changed it
	reason string
	notes  []issueNote
}

// pushLink compares one link and writes to GitHub. A failure is a warning, and
// leaves the base: the difference stays, and the next sync tries again.
func pushLink(t *model.Task, l model.Link, x github.Issue, g model.GitHubState, rules *automation.File, vals model.IssueValues, cl *github.Client, r *IssueResolver, acct config.Account, rep *GitHubFetchReport) *pushed {
	ref := model.CanonRef(l.Ref)
	at := slices.IndexFunc(g.Issues, func(i model.IssueState) bool { return model.CanonRef(i.Ref) == ref })
	if at < 0 {
		return nil
	}
	base, err := g.Issues[at].Values()
	if err != nil {
		return nil // told when the fetch read it
	}
	fields := rules.IssueFields.For(l)
	warn := func(err error) {
		rep.Warnings = append(rep.Warnings, github.Describe(err, acct.ID(), acct.Name, "write "+model.GitHubRef{Repo: repoOf(ref), Kind: model.LinkIssue, Number: x.Number}.Label()).Error())
	}
	gh := GitHubIssueValues(x)
	found, err := r.Filter(vals, fields, gh, base)
	if err != nil {
		warn(err)
		return nil
	}
	rep.Warnings = append(rep.Warnings, found...)
	if slices.Contains(fields, "assignees") {
		if _, others := loginsOn(t.Assignees, repoHost(repoOf(ref))); len(others) > 0 {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %s cannot be assigned there: left out", repoOf(ref), strings.Join(others, ", ")))
		}
	}
	re := automation.Reflect(model.SyncPush, fields, vals, gh, base)
	if slices.Contains(fields, "body") && !strings.Contains(x.Body, "hedgehug:"+t.ULID.String()) {
		re.ToGitHub["body"] = vals["body"] // same text, but no way back to the task
	}
	if len(re.ToGitHub) > 0 {
		u, err := r.Update(t.ULID, re.ToGitHub)
		if err == nil {
			parts := strings.SplitN(repoOf(ref), "/", 3)
			err = cl.UpdateIssue(parts[1], parts[2], x.Number, u)
		}
		if err != nil {
			warn(err)
			return nil
		}
	} else if maps.EqualFunc(base, re.Synced, slices.Equal) {
		return nil
	}
	p := &pushed{ref: ref, synced: re.Synced}
	if v, ok := re.ToGitHub["status"]; ok {
		p.state, p.reason = IssueState(v[0])
	}
	for _, o := range re.Overwritten {
		p.notes = append(p.notes, issueNote{ref, o})
	}
	return p
}

// pushIssues brings the issues that were read up to the tasks that push them.
// It runs after the rules, for what they did to go along, and with the store
// lock free: it talks to GitHub. What it wrote goes in a commit of the task, so
// that a value that could not be written is written by the next sync.
func pushIssues(ss *Session, c *config.Config, e config.StoreEntry, s *store.Store, ix *index.Index, got fetched, rep *GitHubFetchReport) error {
	if len(got.issues) == 0 {
		return nil
	}
	rules, err := s.Automations()
	if err != nil {
		return nil //nolint:nilerr // told when the fetch read it
	}
	if err := ix.Refresh(); err != nil {
		return err
	}
	type conn struct {
		cl   *github.Client
		acct config.Account
		res  *IssueResolver
	}
	conns := map[string]*conn{}
	connOf := func(repo string) *conn {
		if x, ok := conns[repo]; ok {
			return x
		}
		cl, acct, err := ss.GitHubForRepo(c, e, repo)
		if err != nil {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %v", repo, err))
			conns[repo] = nil
			return nil
		}
		x := &conn{cl, acct, NewIssueResolver(cl, repo)}
		conns[repo] = x
		return x
	}
	for _, x := range ix.Tasks() {
		if x.DeletedAt != nil || x.Broken != "" || !slices.ContainsFunc(x.Links, func(l model.Link) bool { return l.Type == model.LinkIssue && l.Sync == model.SyncPush }) {
			continue
		}
		t, err := s.Task(x.ULID)
		if err != nil {
			rep.Warnings = append(rep.Warnings, x.ULID.String()+": "+err.Error())
			continue
		}
		g, err := t.GitHub()
		if err != nil {
			continue // told when the fetch read it
		}
		var done []*pushed
		for _, l := range t.Links {
			issue, read := got.issues[model.CanonRef(l.Ref)]
			if l.Type != model.LinkIssue || l.Sync != model.SyncPush || !read {
				continue
			}
			repo := repoOf(l.Ref)
			cn := connOf(repo)
			if cn == nil {
				continue
			}
			vals := TaskIssueValues(t, rules, s.Schema().For(ix.Ancestors(x)), repoHost(repo))
			if p := pushLink(t, l, issue, g, rules, vals, cn.cl, cn.res, cn.acct, rep); p != nil {
				done = append(done, p)
			}
		}
		if len(done) == 0 {
			continue
		}
		hash, err := ss.UpdateCommit(s, store.Op{Name: OpGitHubPush, Task: t.ULID, Automated: true}, func(tx *store.Tx) error {
			cur, err := tx.Task(t.ULID)
			if err != nil {
				return err
			}
			if err := writePushed(cur, done); err != nil {
				return err
			}
			for _, p := range done {
				if len(p.notes) > 0 {
					tx.AllowEmpty() // GitHub went back, and the base with it: gh is the same
				}
				noteOverwrites(tx, t.ULID, p.notes)
			}
			return tx.PutTask(cur)
		})
		if err != nil {
			// GitHub has the values and the base does not: the next sync finds
			// them the same, or tells what it overwrites.
			rep.Warnings = append(rep.Warnings, t.ULID.String()+": "+err.Error())
			continue
		}
		rep.Committed = rep.Committed || !hash.IsZero()
	}
	return nil
}

// writePushed puts what was pushed in gh of t.
func writePushed(t *model.Task, done []*pushed) error {
	g, err := t.GitHub()
	if err != nil {
		return err
	}
	for i := range g.Issues {
		x := &g.Issues[i]
		for _, p := range done {
			if model.CanonRef(x.Ref) != p.ref {
				continue
			}
			if err := x.SetValues(p.synced); err != nil {
				return err
			}
			if p.state != "" {
				x.State, x.StateReason = p.state, nil
				if p.reason != "" {
					x.StateReason = &p.reason
				}
			}
		}
	}
	return t.SetGitHub(g)
}
