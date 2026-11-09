package integration

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// IssueInput says which repository gets the issue, and where the code is.
type IssueInput struct {
	Repo string // [<host>/]owner/repo; else from the task, or from Dir
	Dir  string
}

// IssueResult is what CreateIssue did.
type IssueResult struct {
	usecase.TaskResult
	Ref      model.GitHubRef
	Warnings []string
}

// IssueWrite is the answer of CreateIssue: the write, and the issue.
type IssueWrite struct {
	usecase.TaskWrite
	Ref      string   `json:"ref"`
	Warnings []string `json:"warnings" nullable:"false"`
}

// Write is r as the answer.
func (r IssueResult) Write() (IssueWrite, error) {
	w, err := r.TaskResult.Write()
	if err != nil {
		return IssueWrite{}, err
	}
	warnings := r.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return IssueWrite{TaskWrite: w, Ref: r.Ref.String(), Warnings: warnings}, nil
}

var writtenRepo = regexp.MustCompile(`^(?:([^/#]+)/)?([^/#]+)/([^/#]+)$`)

// ownerName splits a repo ID.
func ownerName(repo string) (owner, name string) {
	parts := strings.SplitN(repo, "/", 3)
	return parts[1], parts[2]
}

// repoID reads [<host>/]owner/repo into a repo ID. host is the writer's, for a
// repository without one.
func repoID(s, host string) (string, error) {
	m := writtenRepo.FindStringSubmatch(s)
	if m == nil {
		return "", usecase.NewErr(usecase.KindUsage, "repo.invalid", fmt.Errorf("%q: want owner/repo", s))
	}
	if m[1] != "" {
		host = m[1]
	}
	if host == "" {
		host = "github.com"
	}
	return strings.ToLower(host + "/" + m[2] + "/" + m[3]), nil
}

// issueRepo is the repository of the new issue of e: the one named, the one
// of the task, or the one of the code.
func issueRepo(ss *usecase.Session, e *index.Entry, in IssueInput) (string, error) {
	switch {
	case in.Repo != "":
		host, err := ss.WriterHost()
		if err != nil {
			return "", err
		}
		return repoID(in.Repo, host)
	case len(e.Repos) == 1:
		return strings.ToLower(e.Repos[0]), nil
	case len(e.Repos) > 1:
		choice := usecase.ChoiceRequired("a repository", e.Repos)
		choice.Err = fmt.Errorf("%s has several repositories: %s; say which with --repo", e.ID, strings.Join(e.Repos, ", "))
		return "", choice
	}
	if in.Dir == "" {
		return "", usecase.NewErr(usecase.KindInvalid, "issue.repo_required", fmt.Errorf("%s has no repository: say which", e.ID))
	}
	code, err := openCode(in.Dir)
	if err != nil {
		return "", err
	}
	defer func() { _ = code.Close() }()
	r, err := originOf(code)
	return r.id, err
}

// valuesFor is vals with only the fields.
func valuesFor(vals model.IssueValues, fields []string) model.IssueValues {
	out := model.IssueValues{}
	for _, f := range fields {
		out[f] = vals[f]
	}
	return out
}

// CreateIssue opens an issue for the task arg names, in push mode: it gets
// the fields of issue_fields.push, and it is linked with those values as its
// base, so that the next sync has nothing to do. An issue for a task that is
// closed opens and closes at once.
func CreateIssue(ss *usecase.Session, arg string, in IssueInput) (IssueResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return IssueResult{}, err
	}
	if ts != ss {
		return CreateIssue(ts, arg, in)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return IssueResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return IssueResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return IssueResult{}, err
	}
	if _, ok, err := ss.Writer(); err != nil {
		return IssueResult{}, err
	} else if !ok {
		return IssueResult{}, usecase.ErrNoAccounts
	}
	repo, err := issueRepo(ss, e, in)
	if err != nil {
		return IssueResult{}, err
	}
	t, err := s.Task(e.ULID)
	if err != nil {
		return IssueResult{}, err
	}
	if i := slices.IndexFunc(t.Links, func(l model.Link) bool {
		r, ok := model.ParseGitHubRef(l.Ref)
		return ok && l.Live() && r.Kind == model.LinkIssue && strings.EqualFold(r.Repo, repo)
	}); i >= 0 {
		r, _ := model.ParseGitHubRef(t.Links[i].Ref)
		return IssueResult{}, usecase.NewErr(usecase.KindInvalid, "link.exists", fmt.Errorf("%s is linked already", r.Label()))
	}
	rules, err := s.Automations()
	if err != nil {
		return IssueResult{}, usecase.NewErr(usecase.KindInvalid, "automation.invalid", err)
	}
	c, se, err := ss.Entry()
	if err != nil {
		return IssueResult{}, err
	}
	cl, acct, err := ss.GitHubForRepo(c, se, repo)
	if err != nil {
		return IssueResult{}, err
	}
	host, _, _ := strings.Cut(repo, "/")
	fields := rules.IssueFields.For(model.Link{Sync: model.SyncPush})
	vals := usecase.TaskIssueValues(t, rules, s.Schema().For(ix.Ancestors(e)), host)
	res := usecase.NewIssueResolver(cl, repo)
	warnings, err := res.Filter(vals, fields, nil)
	if err != nil {
		return IssueResult{}, usecase.GithubErr(err, acct, "read the names of "+repo)
	}
	if slices.Contains(fields, "assignees") {
		warnings = append(warnings, usecase.ForeignAssignees(t, repo)...)
	}
	vals = valuesFor(vals, fields)
	n, err := openIssue(cl, res, t, repo, vals)
	if err != nil {
		return IssueResult{}, usecase.GithubErr(err, acct, "open an issue in "+repo)
	}
	ref := model.GitHubRef{Repo: repo, Kind: model.LinkIssue, Number: n}
	// The issue is there: from here it gets linked, or a second run would
	// open another. If it does not close, the base says open, and the next
	// sync closes it.
	if v, ok := vals["status"]; ok && v[0] != automation.IssueOpen {
		if err := closeIssue(cl, repo, n, v[0]); err != nil {
			warnings = append(warnings, usecase.GithubErr(err, acct, "close "+ref.Label()).Error())
			vals["status"] = []string{automation.IssueOpen}
		}
	}
	tr, err := writeLink(ss, s, ix, e, "issue", ref, func(_ *store.Store, t *model.Task, ref model.GitHubRef) error {
		return linkPushed(t, ref, vals)
	})
	if err != nil {
		return IssueResult{}, fmt.Errorf("%s is on GitHub, but the task is not linked to it: %w", ref.Label(), err)
	}
	return IssueResult{TaskResult: tr, Ref: ref, Warnings: warnings}, nil
}

// openIssue opens the issue of t with vals. It opens as open, whatever vals
// say of its status.
func openIssue(cl *github.Client, res *usecase.IssueResolver, t *model.Task, repo string, vals model.IssueValues) (int, error) {
	owner, name := ownerName(repo)
	send := model.IssueValues{}
	for f, v := range vals {
		nothing := len(v) == 0 || f == "milestone" && v[0] == ""
		if f != "status" && f != "title" && !nothing {
			send[f] = v
		}
	}
	u, err := res.Update(t.ULID, send)
	if err != nil {
		return 0, err
	}
	in := github.NewIssue{Title: t.Title, Body: model.MarkedBody(t.ULID, "")}
	if u.Body != nil {
		in.Body = *u.Body
	}
	if u.Assignees != nil {
		in.Assignees = *u.Assignees
	}
	if u.Milestone != nil {
		in.Milestone = *u.Milestone
	}
	return cl.CreateIssue(owner, name, in)
}

// closeIssue closes the issue n as the token says.
func closeIssue(cl *github.Client, repo string, n int, token string) error {
	owner, name := ownerName(repo)
	state, reason := usecase.IssueState(token)
	return cl.UpdateIssue(owner, name, n, github.IssueUpdate{State: &state, StateReason: &reason})
}

// linkPushed links t to the new issue ref, with vals as what the two hold.
func linkPushed(t *model.Task, ref model.GitHubRef, vals model.IssueValues) error {
	t.Links = append(slices.Clone(t.Links), model.Link{Type: model.LinkIssue, Ref: ref.String(), Sync: model.SyncPush})
	g, err := t.GitHub()
	if err != nil {
		return err
	}
	x := model.IssueState{Ref: ref.String(), Title: t.Title, State: "open"}
	if v, ok := vals["status"]; ok {
		state, reason := usecase.IssueState(v[0])
		x.State = state
		if reason != "" {
			x.StateReason = &reason
		}
	}
	if err := x.SetValues(vals); err != nil {
		return err
	}
	g.Issues = append(g.Issues, x)
	return t.SetGitHub(g)
}
