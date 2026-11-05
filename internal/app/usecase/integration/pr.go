package integration

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/github"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/giturl"
)

// origin is the remote of a code repository that gets the pushes.
const origin = "origin"

// PRInput says where the code is, and whether the pull request is a draft.
type PRInput struct {
	Dir   string
	Draft bool
}

// PRResult is what PullRequest did.
type PRResult struct {
	usecase.TaskResult
	Ref      model.GitHubRef
	Branch   string
	Created  bool     // PullRequest opened the pull request
	Warnings []string // what went wrong after the pull request was there
}

// PRWrite is the answer of PullRequest: the write, and the pull request.
type PRWrite struct {
	usecase.TaskWrite
	Ref      string   `json:"ref"`
	Branch   string   `json:"branch"`
	Created  bool     `json:"created"`
	Warnings []string `json:"warnings" nullable:"false"`
}

// Write is r as the answer.
func (r PRResult) Write() (PRWrite, error) {
	w, err := r.TaskResult.Write()
	if err != nil {
		return PRWrite{}, err
	}
	warnings := r.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return PRWrite{TaskWrite: w, Ref: r.Ref.String(), Branch: r.Branch, Created: r.Created, Warnings: warnings}, nil
}

// codeRepo is a repository on GitHub, as origin of a code repository says.
type codeRepo struct{ id, owner, name string }

// originOf reads origin of code.
func originOf(code *gitrepo.Code) (codeRepo, error) {
	invalid := func(err error) (codeRepo, error) {
		return codeRepo{}, usecase.NewErr(usecase.KindInvalid, "pr.no_origin", err)
	}
	u, ok, err := code.RemoteURL(origin)
	if err != nil {
		return codeRepo{}, err
	}
	if !ok {
		return invalid(errors.New("the repository has no remote origin; add the repository on GitHub as origin"))
	}
	_, owner, name, splits := giturl.Split(u)
	id, known := giturl.RepoID(u)
	if !splits || !known {
		return invalid(fmt.Errorf("origin is %s, which is no repository on a host", u))
	}
	return codeRepo{id: id, owner: owner, name: name}, nil
}

// branchToPush is the branch of the task e: the one the developer is on, or
// else the only one there is.
func branchToPush(code *gitrepo.Code, e *index.Entry) (string, error) {
	match := ofTask(e)
	cur, on, err := code.Branch()
	if err != nil {
		return "", err
	}
	if on && match(cur) {
		return cur, nil
	}
	have, err := code.Branches(match)
	if err != nil {
		return "", err
	}
	switch len(have) {
	case 0:
		return "", usecase.NewErr(usecase.KindInvalid, "pr.no_branch",
			fmt.Errorf("%s has no branch here; make it with hedgehug start %s", e.ID, e.ID), "id", e.ID)
	case 1:
		return have[0], nil
	}
	choice := usecase.ChoiceRequired("a branch", have)
	choice.Err = fmt.Errorf("%s has several branches: %s; git switch to one and run this again", e.ID, strings.Join(have, ", "))
	return "", choice
}

func push(code *gitrepo.Code, branch string, conn gitrepo.Conn) error {
	err := code.PushBranch(origin, branch, conn)
	switch {
	case errors.Is(err, gitrepo.ErrRejected):
		return usecase.NewErr(usecase.KindConflict, "pr.rejected",
			fmt.Errorf("origin has commits that %s does not; bring them in and run this again (hedgehug does not force a push)", branch), "branch", branch)
	case errors.Is(err, gitrepo.ErrPushDenied):
		return usecase.NewErr(usecase.KindInvalid, "pr.denied", err, "branch", branch)
	}
	return err
}

// openPull finds the open pull request of branch, or opens one.
func openPull(cl *github.Client, acct config.Account, r codeRepo, branch string, t *model.Task, draft bool) (n int, created bool, err error) {
	n, ok, err := cl.OpenPullFor(r.owner, r.name, branch)
	if err != nil {
		return 0, false, usecase.GithubErr(err, acct, "read the pull requests of "+r.id)
	}
	if ok {
		return n, false, nil
	}
	repo, err := cl.Repo(r.owner, r.name)
	if err != nil {
		return 0, false, usecase.GithubErr(err, acct, "read "+r.id)
	}
	n, err = cl.CreatePull(r.owner, r.name, github.NewPull{
		Title: t.Title, Body: model.MarkedBody(t.ULID, t.Body), Head: branch, Base: repo.DefaultBranch, Draft: draft,
	})
	if errors.Is(err, github.ErrPullExists) {
		// Another run opened it since the look above.
		if n, ok, e := cl.OpenPullFor(r.owner, r.name, branch); e == nil && ok {
			return n, false, nil
		}
	}
	if err != nil {
		return 0, false, usecase.GithubErr(err, acct, "open a pull request in "+r.id)
	}
	return n, true, nil
}

// PullRequest pushes the branch of the task arg names to origin, opens its
// pull request, or finds the one that is open, links it, and reads the
// repository right away. Each step that is done already is passed over, so
// that a run that stopped half way goes on from there. Reading the repository
// is the last step: when it fails, that is a warning.
func PullRequest(ss *usecase.Session, arg string, in PRInput) (PRResult, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return PRResult{}, err
	}
	if ts != ss {
		return PullRequest(ts, arg, in)
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return PRResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return PRResult{}, err
	}
	if err := usecase.NotDeleted(ix, e); err != nil {
		return PRResult{}, err
	}
	// The link needs a writer: find out before anything goes to GitHub.
	if _, ok, err := ss.Writer(); err != nil {
		return PRResult{}, err
	} else if !ok {
		return PRResult{}, usecase.ErrNoAccounts
	}
	code, err := openCode(in.Dir)
	if err != nil {
		return PRResult{}, err
	}
	defer func() { _ = code.Close() }()
	branch, err := branchToPush(code, e)
	if err != nil {
		return PRResult{}, err
	}
	repo, err := originOf(code)
	if err != nil {
		return PRResult{}, err
	}
	c, se, err := ss.Entry()
	if err != nil {
		return PRResult{}, err
	}
	cl, acct, err := ss.GitHubForRepo(c, se, repo.id)
	if err != nil {
		return PRResult{}, err
	}
	conn, _, err := ss.ConnForRepo(c, se, repo.id)
	if err != nil {
		return PRResult{}, err
	}
	if err := push(code, branch, conn); err != nil {
		return PRResult{}, err
	}
	t, err := s.Task(e.ULID)
	if err != nil {
		return PRResult{}, err
	}
	n, created, err := openPull(cl, acct, repo, branch, t, in.Draft)
	if err != nil {
		return PRResult{}, err
	}
	ref := model.GitHubRef{Repo: repo.id, Kind: model.LinkPR, Number: n}
	tr, err := writeLink(ss, s, ix, e, "link", ref, func(s *store.Store, t *model.Task, ref model.GitHubRef) error {
		if slices.ContainsFunc(t.Links, func(l model.Link) bool { return model.CanonRef(l.Ref) == ref.String() && l.Live() }) {
			return nil
		}
		return LinkInput{PR: ref.String()}.link(s, t, ref)
	})
	if err != nil {
		return PRResult{}, err
	}
	out := PRResult{TaskResult: tr, Ref: ref, Branch: branch, Created: created}
	out.Warnings = fetchNow(ss, c, se, s, repo.id)
	return out, nil
}

// fetchNow reads the repository right away and returns what went wrong, which
// is no failure of the command that asked. As in a sync, what another member
// fetched comes first: the fetch reads every task with a link to the
// repository, not only one.
func fetchNow(ss *usecase.Session, c *config.Config, se config.StoreEntry, s *store.Store, repo string) []string {
	if len(se.Remotes) > 0 {
		opt, err := ss.SyncOptions(c, se)
		if err == nil {
			_, err = ss.RunSync(s, opt, true)
		}
		if err != nil {
			return []string{"the sync before reading GitHub failed, so it is not read: " + err.Error()}
		}
	}
	var warnings []string
	rep, err := usecase.FetchGitHubRepo(ss, c, se, repo)
	if err != nil {
		warnings = append(warnings, err.Error())
	}
	warnings = append(warnings, rep.Warnings...)
	if rep.Committed {
		// The fetch does not start a background sync: its other caller, the
		// sync, pushes what it writes.
		ss.MarkWritten(s)
	}
	return warnings
}
