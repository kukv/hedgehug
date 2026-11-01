package ops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/gitrepo"
)

// Candidate is a store that StoreDiscover offers. Account is the account that
// reaches it (empty when none is known); Permission is "read" or "write".
type Candidate struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Account    string `json:"account,omitempty"`
	Permission string `json:"permission"`
	key        string // what tells it from a mounted store
}

// DiscoverInput is where StoreDiscover looks. From is a directory to look for
// bare repositories under; with none, it looks on GitHub. Account limits the
// look to that account; with From, it is the account of the candidates.
type DiscoverInput struct {
	Account, From string
}

// StoreDiscover lists the stores the accounts can read and that are not set
// up yet, each with the name to add it under.
//
// Without From these are the repositories with the hedgehug-store topic that
// the accounts with a token (or the one named) can read. A repository that
// two accounts see goes with the one that can push, else the one listed
// first. With From they are the bare repositories under it that hold a store.
// With From, no Account and two or more accounts, there is no account to give
// the candidates: KindUsage, "choice.required", with the accounts.
func (ss *Session) StoreDiscover(in DiscoverInput) ([]Candidate, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	var cands []Candidate
	if in.From != "" {
		cands, err = ss.discoverFrom(c, in.From, in.Account)
	} else {
		cands, err = ss.discoverGitHub(c, in.Account)
	}
	if err != nil {
		return nil, err
	}
	cands = dropMounted(c, cands)
	nameCandidates(c, cands)
	return cands, nil
}

// MountResult is what StoreMount did: the names it added and those it could
// not.
type MountResult struct {
	Mounted []string
	Failed  []MountFailure
}

// MountFailure is a candidate that could not be added.
type MountFailure struct {
	Name string
	Err  error
}

// StoreMount adds each candidate as StoreAdd does, with its account, so the
// access to it is judged too. They go one by one and independently: one that
// fails is undone by StoreAdd alone, and the rest go on. When any failed, the
// result comes with an error (KindInvalid, "store.mount_failed").
func (ss *Session) StoreMount(cands []Candidate) (MountResult, error) {
	var res MountResult
	for _, x := range cands {
		if _, err := ss.StoreAdd(x.Name, []string{x.URL}, "", x.Account); err != nil {
			res.Failed = append(res.Failed, MountFailure{Name: x.Name, Err: err})
			continue
		}
		res.Mounted = append(res.Mounted, x.Name)
	}
	if len(res.Failed) == 0 {
		return res, nil
	}
	names := make([]string, len(res.Failed))
	for i, f := range res.Failed {
		names[i] = f.Name
	}
	return res, newErr(KindInvalid, "store.mount_failed",
		fmt.Errorf("%d of %d stores could not be added: %s", len(res.Failed), len(cands), strings.Join(names, ", ")),
		"failed", names)
}

// discoverGitHub lists the repositories with the hedgehug-store topic that
// the accounts can read; only is the one account to look with, when given.
func (ss *Session) discoverGitHub(c *config.Config, only string) ([]Candidate, error) {
	var accts []config.Account
	if only != "" {
		acct, _, err := ss.PickAccount(c, only)
		if err != nil {
			return nil, err
		}
		accts = []config.Account{acct}
	} else {
		for _, x := range c.Accounts {
			if x.Token.Kind != "" {
				accts = append(accts, x)
			}
		}
	}
	if len(accts) == 0 {
		return nil, newErr(KindConflict, "account.none",
			errors.New("no account has a token to look on GitHub with; run hedgehug auth login, or use --from"))
	}
	var out []Candidate
	for _, acct := range accts {
		// The host asked is the account's own, so its token goes nowhere else.
		gh, err := ss.GitHub(c, acct, acct.Host)
		if err != nil {
			return nil, err
		}
		repos, err := gh.UserRepos()
		if err != nil {
			return nil, githubErr(err, acct, "list the repositories")
		}
		for _, r := range repos {
			if !slices.Contains(r.Topics, "hedgehug-store") {
				continue
			}
			perm := "read"
			if r.Permissions.Push {
				perm = "write"
			}
			key, _ := hhctx.RepoID(r.CloneURL)
			x := Candidate{URL: r.CloneURL, Account: acct.Name, Permission: perm, key: key}
			if i := slices.IndexFunc(out, func(y Candidate) bool { return y.key == key }); i >= 0 {
				if out[i].Permission == "read" && perm == "write" {
					out[i] = x
				}
				continue
			}
			out = append(out, x)
		}
	}
	return out, nil
}

// discoverFrom lists the bare repositories under dir whose HEAD holds a
// store. The URL goes back through url_rewrites to the form before them.
func (ss *Session) discoverFrom(c *config.Config, dir, account string) ([]Candidate, error) {
	acct, ok, err := ss.PickAccount(c, account)
	if err != nil {
		return nil, err
	}
	if !ok && len(c.Accounts) > 1 {
		names := make([]string, len(c.Accounts))
		for i, x := range c.Accounts {
			names[i] = x.Name
		}
		return nil, choiceRequired("--account", names)
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var out []Candidate
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			// One directory that cannot be read does not stop the rest.
			ss.warn(fmt.Sprintf("cannot read %s: %v", p, err))
			if d != nil && !d.IsDir() {
				return nil
			}
			return filepath.SkipDir
		}
		if !d.IsDir() || !isBare(p) {
			return nil
		}
		if has, err := gitrepo.HeadHasFile(p, ".hedgehug/store.yaml"); err == nil && has {
			perm := "read"
			if gitrepo.Writable(p) {
				perm = "write"
			}
			x := Candidate{URL: c.Unrewrite(filepath.ToSlash(p)), Permission: perm, key: filepath.Clean(p)}
			if ok {
				x.Account = acct.Name
			}
			out = append(out, x)
		}
		return filepath.SkipDir
	})
	return out, err
}

// isBare reports whether p has the HEAD file and the objects and refs
// directories of a bare repository.
func isBare(p string) bool {
	if fi, err := os.Stat(filepath.Join(p, "HEAD")); err != nil || fi.IsDir() { //nolint:gosec // G703: p is a directory under the --from the person gave
		return false
	}
	for _, d := range []string{"objects", "refs"} {
		if fi, err := os.Stat(filepath.Join(p, d)); err != nil || !fi.IsDir() { //nolint:gosec // G703: p is a directory under the --from the person gave
			return false
		}
	}
	return true
}

// dropMounted leaves out the candidates that a store set up already uses: by
// repo ID, or by the cleaned path for a local remote.
func dropMounted(c *config.Config, cands []Candidate) []Candidate {
	have := map[string]bool{}
	for _, e := range c.Stores {
		for _, r := range e.Remotes {
			if id, ok := hhctx.RepoID(r); ok {
				have[id] = true
			}
			if p, ok := gitrepo.LocalPath(c.Rewrite(r)); ok {
				have[filepath.Clean(p)] = true
			}
		}
	}
	return slices.DeleteFunc(cands, func(x Candidate) bool {
		if id, ok := hhctx.RepoID(x.URL); ok && have[id] {
			return true
		}
		if p, ok := gitrepo.LocalPath(c.Rewrite(x.URL)); ok && have[filepath.Clean(p)] {
			return true
		}
		return have[x.key]
	})
}

var badNameChars = regexp.MustCompile(`[^a-z0-9_-]+`)

// nameCandidates gives each candidate a store name: the repo name; when that
// is used, <owner>-<repo>; when that is used too, -2, -3 after it. A name the
// rule of store names does not take is made lower case with - for the other
// characters. Where nothing valid can be made of the owner and the repo, the
// name is store-1, store-2... (the first number that is not used, from 1).
func nameCandidates(c *config.Config, cands []Candidate) {
	used := map[string]bool{}
	for _, e := range c.Stores {
		used[e.Name] = true
	}
	fix := func(s string) string {
		s = strings.Trim(badNameChars.ReplaceAllString(strings.ToLower(s), "-"), "-")
		if config.ValidStoreName(s) != nil {
			return ""
		}
		return s
	}
	for i := range cands {
		u := strings.TrimSuffix(filepath.ToSlash(cands[i].URL), "/")
		repo := strings.TrimSuffix(path.Base(u), ".git")
		owner := path.Base(path.Dir(u))
		var name string
		base := fix(owner + "-" + repo)
		for _, n := range []string{fix(repo), base} {
			if n != "" && !used[n] {
				name = n
				break
			}
		}
		switch {
		case name != "":
		case base != "":
			// <owner>-<repo> is used too: -2, -3...
			for n := 2; name == ""; n++ {
				if x := base + "-" + strconv.Itoa(n); !used[x] {
					name = x
				}
			}
		default:
			// Nothing valid can be made of the owner and the repo.
			for n := 1; name == ""; n++ {
				if x := "store-" + strconv.Itoa(n); !used[x] {
					name = x
				}
			}
		}
		used[name] = true
		cands[i].Name = name
	}
}
