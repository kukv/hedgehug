package sync

import (
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
)

// infoInterval is how often a sync reads the permission and the
// collaborators from GitHub.
const infoInterval = 5 * time.Minute

// GitHubRepo is the repository on GitHub that origin is. Client gets the
// token only when called.
type GitHubRepo struct {
	Host, Owner, Name string
	// AccountID (<host>/<login>) and AccountName (as in config.yaml) say who
	// asks, for the warnings; sync does not import config.
	AccountID, AccountName string
	Client                 func() (*github.Client, error)
}

// CheckAccess finds out whether the store may be pushed to, by what origin
// is: the permission on GitHub, whether a local bare can be written, or
// nothing to check (a push that is denied tells). It writes access.json, and
// for GitHub the collaborators to members.json. What it could not read is a
// warning, and the access is left as it was, with the time of the check
// moved on so that the next try is infoInterval later.
//
// It writes files of .hedgehug/local that a sync writes under sync.lock: a
// caller outside the sync must hold that lock (Store.WithLocalLock) around it.
func CheckAccess(s *store.Store, opt Options, now time.Time) (Access, []string, error) {
	a, err := ReadAccess(s)
	if err != nil {
		return a, nil, err
	}
	var warns []string
	switch {
	case opt.GitHub != nil:
		gh, err := opt.GitHub.Client()
		var r *github.Repo
		if err == nil {
			r, err = gh.Repo(opt.GitHub.Owner, opt.GitHub.Name)
		}
		if err != nil {
			doing := "read the permission of " + opt.GitHub.Owner + "/" + opt.GitHub.Name
			warn := github.Describe(err, opt.GitHub.AccountID, opt.GitHub.AccountName, doing).Error()
			a.Checked = now // a failure is not tried again before infoInterval
			warns = append(warns, warn)
			// The collaborators are unknown too, not the list of an earlier check.
			if w := forgetCollaborators(s, now); w != "" {
				warns = append(warns, w)
			}
			return a, warns, WriteAccess(s, a)
		}
		a = Access{ReadOnly: !r.Permissions.Push, Checked: now}
		if a.ReadOnly {
			a.Reason = ReasonGitHub
		}
		if w := updateCollaborators(s, gh, opt.GitHub, now); w != "" {
			warns = append(warns, w)
		}
	case len(opt.Remotes) > 0:
		to := opt.Remotes[0]
		if opt.Conn.Rewrite != nil {
			to = opt.Conn.Rewrite(to)
		}
		p, ok := gitrepo.LocalPath(to)
		if !ok {
			return a, nil, nil
		}
		a = Access{ReadOnly: !gitrepo.Writable(p), Checked: now}
		if a.ReadOnly {
			a.Reason = ReasonNotWritable
		}
	default:
		return a, nil, nil
	}
	return a, warns, WriteAccess(s, a)
}

// updateCollaborators writes the collaborators to members.json. When they
// cannot be read, for whatever reason, they are unknown (null), not the list
// of an earlier check. A 403 is how GitHub says the token cannot push, which
// a reader is not told to fix, so it is no warning; any other failure is. It
// returns a warning, or "".
func updateCollaborators(s *store.Store, gh *github.Client, r *GitHubRepo, now time.Time) string {
	m, err := ReadMembers(s)
	if err != nil {
		return "read members.json: " + err.Error()
	}
	warn := ""
	cs, err := gh.Collaborators(r.Owner, r.Name)
	if err == nil {
		m.Collaborators = make([]Collaborator, len(cs))
		for i, c := range cs {
			m.Collaborators[i] = Collaborator{ID: r.Host + "/" + c.Login, Permission: c.RoleName}
		}
	} else {
		m.Collaborators = nil
		if !errors.Is(err, github.ErrForbidden) {
			warn = github.Describe(err, r.AccountID, r.AccountName, "read the collaborators of "+r.Owner+"/"+r.Name).Error()
		}
	}
	m.Checked = now
	if err := WriteMembers(s, m); err != nil {
		return "write members.json: " + err.Error()
	}
	return warn
}

// forgetCollaborators makes the collaborators unknown (null) and moves the
// time of the check on, for a check that could not reach GitHub at all. It
// returns a warning, or "".
func forgetCollaborators(s *store.Store, now time.Time) string {
	m, err := ReadMembers(s)
	if err != nil {
		return "read members.json: " + err.Error()
	}
	m.Collaborators = nil
	m.Checked = now
	if err := WriteMembers(s, m); err != nil {
		return "write members.json: " + err.Error()
	}
	return ""
}

// refreshInfo checks the access and the collaborators when infoInterval has
// passed since the last check, and adds the authors of the new commits. A
// sync runs it under sync.lock.
func refreshInfo(s *store.Store, opt Options, res *Result) {
	now := opt.Now()
	a, err := ReadAccess(s)
	if err == nil && now.Sub(a.Checked) >= infoInterval {
		var warns []string
		_, warns, err = CheckAccess(s, opt, now)
		res.Warnings = append(res.Warnings, warns...)
	}
	if err != nil {
		res.Warnings = append(res.Warnings, "check the access: "+err.Error())
	}
	if err := UpdateAuthors(s); err != nil {
		res.Warnings = append(res.Warnings, "read the authors: "+err.Error())
	}
}
