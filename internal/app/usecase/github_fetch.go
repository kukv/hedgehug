package usecase

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// GitHubFetchReport is what a fetch did.
type GitHubFetchReport struct {
	Committed bool
	Warnings  []string
}

// target is what a repository is read for.
type target struct{ pulls, issues []int }

// FetchGitHub reads the repositories of the store e that are due, links the pull
// requests that point at a task, and writes what GitHub says to gh, as one
// commit, followed by a commit for each task whose status the rules of
// automations.yaml change. A repository that cannot be read is a warning: the
// others go on. A read-only store is not fetched.
func FetchGitHub(ss *Session, c *config.Config, e config.StoreEntry) (GitHubFetchReport, error) {
	return fetchStore(ss, c, e, "")
}

// FetchGitHubRepo is FetchGitHub of the one repository repo, a repo ID, whether
// it is due or not. The other repositories of the store are left for their
// time.
func FetchGitHubRepo(ss *Session, c *config.Config, e config.StoreEntry, repo string) (GitHubFetchReport, error) {
	return fetchStore(ss, c, e, strings.ToLower(repo))
}

// fetchStore is FetchGitHub, or FetchGitHubRepo when force names a repository.
func fetchStore(ss *Session, c *config.Config, e config.StoreEntry, force string) (GitHubFetchReport, error) {
	ss = ss.ForStore(e.Name)
	ss.BackgroundSync = false // the sync that called this pushes what it writes
	var rep GitHubFetchReport
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return rep, err
	}
	defer func() { _ = s.Repo().Close() }()
	var ro *ReadOnlyError
	if err := ss.CheckWritable(s); errors.As(err, &ro) {
		return rep, nil
	} else if err != nil {
		return rep, err
	}
	err = s.WithLocalLock("github.lock", SyncLockWait, func() error {
		ix, err := ss.IndexOf(e.Name, s)
		if err != nil {
			return err
		}
		got := fetched{pulls: map[string]github.Pull{}, issues: map[string]github.Issue{}}
		if err := fetchLocked(ss, c, e, s, ix, force, got, &rep); err != nil {
			return err
		}
		return pushIssues(ss, c, e, s, ix, got, &rep)
	})
	return rep, err
}

// fetchLocked reads the repositories into got and writes what they say. The
// store lock it takes is free again when it returns.
func fetchLocked(ss *Session, c *config.Config, e config.StoreEntry, s *store.Store, ix *index.Index, force string, got fetched, rep *GitHubFetchReport) error {
	state, err := ghstate.Read(s)
	if err != nil {
		return err
	}
	targets := targetsOf(ix)
	if len(targets) == 0 {
		return nil // no repository: nothing needs an account
	}
	now := ss.Now()
	auto := map[model.ULID][]autoLink{}
	// read is what each repository that was read now knows. It goes into the
	// state only when what was found is written: a repository whose result
	// was lost must be read again from its old time.
	read := map[string]ghstate.Repo{}
	for _, repo := range slices.Sorted(maps.Keys(targets)) {
		if force != "" && repo != force {
			continue
		}
		st := state.Repos[repo]
		if force == "" && !st.Due(now) {
			// Not asked again, and what the last read said is said again.
			if st.Warning != "" {
				rep.Warnings = append(rep.Warnings, strings.Split(st.Warning, "\n")...)
			}
			continue
		}
		// fail notes a read that did not happen: it is not tried again within
		// the interval, and its warning is told meanwhile.
		fail := func(msg string) {
			st.LastTry, st.Warning = now, msg
			state.Repos[repo] = st
			rep.Warnings = append(rep.Warnings, msg)
		}
		t := targets[repo]
		cl, acct, err := ss.GitHubForRepo(c, e, repo)
		if err != nil {
			fail(fmt.Sprintf("%s: %v", repo, err))
			continue
		}
		parts := strings.SplitN(repo, "/", 3)
		owner, name := parts[1], parts[2]
		scanned, rl, err := cl.ScanPulls(owner, name, st.Since)
		var linked github.Linked
		if err == nil {
			var rl2 github.RateLimit
			linked, rl2, err = cl.LinkedItems(owner, name, t.pulls, t.issues)
			if rl2.Limit > 0 {
				rl = rl2
			}
		}
		if err != nil {
			var limited *github.RateLimitError
			if errors.As(err, &limited) {
				st.NotBefore = limited.Reset
			}
			fail(github.Describe(err, acct.ID(), acct.Name, "read "+repo).Error())
			continue
		}
		st.LastFetch, st.Limit, st.Remaining, st.ResetAt = now, rl.Limit, rl.Remaining, rl.ResetAt
		st.LastTry, st.Warning = time.Time{}, ""
		for _, p := range scanned {
			if p.UpdatedAt.After(st.Since) {
				st.Since = p.UpdatedAt // GitHub's own time: the clocks may differ
			}
		}
		var found []string
		for _, p := range slices.Concat(scanned, linked.Pulls) {
			got.pulls[model.GitHubRef{Repo: repo, Kind: model.LinkPR, Number: p.Number}.String()] = p
		}
		for _, x := range linked.Issues {
			got.issues[model.GitHubRef{Repo: repo, Kind: model.LinkIssue, Number: x.Number}.String()] = x
		}
		for _, n := range slices.Concat(linked.MissingPulls, linked.MissingIssues) {
			kind := model.LinkPR
			if slices.Contains(linked.MissingIssues, n) {
				kind = model.LinkIssue
			}
			found = append(found, model.GitHubRef{Repo: repo, Kind: kind, Number: n}.Label()+" is not on GitHub")
		}
		rep.Warnings = append(rep.Warnings, found...)
		st.Warning = strings.Join(found, "\n")
		read[repo] = st
		for u, links := range autoLinks(ix, repo, scanned) {
			auto[u] = append(auto[u], links...)
		}
	}
	if len(got.pulls)+len(got.issues) == 0 {
		maps.Copy(state.Repos, read) // nothing to write; a rate limit may be noted
		return ghstate.Write(s, state)
	}
	// The store lock is held from here to the last status, and not during the
	// reads above: what the rules decide is not overtaken by another writer
	// before it is written.
	held, err := store.LockInOrder(s)
	if err == nil {
		defer held.Release()
		err = ix.Refresh()
	}
	if err != nil {
		return errors.Join(err, ghstate.Write(s, state)) // as for a commit that failed, below
	}
	rules, err := s.Automations()
	usable := err == nil // which fields sync is not known without it: nothing does
	if err != nil {
		rep.Warnings = append(rep.Warnings, err.Error())
		rules = automation.Default() // the fetch goes on, without rules
	}
	// Only a task with a link to something that was read, or one to link, has
	// anything to write.
	ulids := make([]model.ULID, 0)
	reads := func(x *index.Entry) bool {
		return len(auto[x.ULID]) > 0 || slices.ContainsFunc(x.Links, func(l model.Link) bool {
			_, ok := read[repoOf(l.Ref)]
			return ok
		})
	}
	for _, x := range ix.Tasks() {
		if x.DeletedAt == nil && x.Broken == "" && reads(x) {
			ulids = append(ulids, x.ULID)
		}
	}
	slices.SortFunc(ulids, func(a, b model.ULID) int { return cmp.Compare(a.String(), b.String()) })
	failed := map[string][]string{} // repositories whose result a task could not take, with why
	var changes []taskChange
	var plans []*mirrorPlan
	// A store with no remote is never synced, so nothing else starts its
	// inbox: the commits below must not fall before where it starts.
	if head, err := s.Repo().Head(); err != nil {
		return errors.Join(err, ghstate.Write(s, state))
	} else if err := inbox.Open(s).Start(head); err != nil {
		return errors.Join(err, ghstate.Write(s, state))
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "gh-fetch", Automated: true, Locked: held}, func(tx *store.Tx) error {
		for _, u := range ulids {
			t, err := tx.Task(u)
			if err != nil {
				return err
			}
			a, err := apply(t, got, auto[u])
			if err != nil {
				rep.Warnings = append(rep.Warnings, u.String()+": "+err.Error())
				for _, l := range auto[u] {
					failed[repoOf(l.ref)] = append(failed[repoOf(l.ref)], u.String()+": "+err.Error())
				}
				for _, l := range t.Links {
					failed[repoOf(l.Ref)] = append(failed[repoOf(l.Ref)], u.String()+": "+err.Error())
				}
				continue
			}
			var plan *mirrorPlan
			if usable {
				x, _ := ix.Task(u) // chosen from ix above
				if plan, err = planMirror(t, got, rules, s.Schema().For(ix.Ancestors(x))); err != nil {
					rep.Warnings = append(rep.Warnings, u.String()+": "+err.Error())
				}
			}
			if plan != nil {
				plans = append(plans, plan)
			}
			if !a.Changed {
				continue
			}
			if err := tx.PutTask(t); err != nil {
				return err
			}
			if c := runRules(tx, rules, ix, s.Schema(), t, a, plan); c != nil {
				changes = append(changes, *c)
			}
		}
		return nil
	})
	if err != nil {
		// The reads that failed are noted all the same, or each try would ask
		// them again. What was read stays unmarked: it must be read again.
		return errors.Join(err, ghstate.Write(s, state))
	}
	rep.Committed = !hash.IsZero()
	skip := applyMirror(ss, s, held, plans, rep)
	applyChanges(ss, s, held, changes, skip, rep)
	for repo, st := range read {
		if why, bad := failed[repo]; bad {
			// Read again from the old time. The warning was told already.
			old := state.Repos[repo]
			old.LastTry, old.Warning = now, strings.Join(slices.Compact(slices.Sorted(slices.Values(why))), "\n")
			state.Repos[repo] = old
			continue
		}
		state.Repos[repo] = st
	}
	return ghstate.Write(s, state)
}

// targetsOf collects the repositories the tasks of ix name, by repo ID: those
// in repos and those of the links in use, with the numbers of the links.
func targetsOf(ix *index.Index) map[string]*target {
	out := map[string]*target{}
	at := func(repo string) *target {
		repo = strings.ToLower(repo)
		if out[repo] == nil {
			out[repo] = &target{}
		}
		return out[repo]
	}
	for _, e := range ix.Tasks() {
		if e.DeletedAt != nil || e.Broken != "" {
			continue
		}
		for _, r := range e.Repos {
			at(r)
		}
		for _, l := range e.Links {
			r, ok := model.ParseGitHubRef(l.Ref)
			if !ok || !l.Live() {
				continue
			}
			t := at(r.Repo)
			switch {
			case r.Kind == model.LinkPR && !slices.Contains(t.pulls, r.Number):
				t.pulls = append(t.pulls, r.Number)
			case r.Kind == model.LinkIssue && !slices.Contains(t.issues, r.Number):
				t.issues = append(t.issues, r.Number)
			}
		}
	}
	for _, t := range out {
		slices.Sort(t.pulls)
		slices.Sort(t.issues)
	}
	return out
}

// repoOf is the repo ID of a ref, as targetsOf keys it; "" for a ref that is
// not a GitHub one.
func repoOf(ref string) string {
	r, ok := model.ParseGitHubRef(ref)
	if !ok {
		return ""
	}
	return strings.ToLower(r.Repo)
}

// HasGitHubTargets reports whether the tasks of the store e name a repository to
// read: one in repos, or one a link in use points at. A store that does not
// open has none.
func HasGitHubTargets(ss *Session, e config.StoreEntry) bool {
	ss = ss.ForStore(e.Name)
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return false
	}
	defer func() { _ = s.Repo().Close() }()
	ix, err := ss.IndexOf(e.Name, s)
	return err == nil && len(targetsOf(ix)) > 0
}
