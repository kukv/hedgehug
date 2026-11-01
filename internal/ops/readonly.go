package ops

import (
	"fmt"
	"path/filepath"

	"github.com/kukv/hedgehug/internal/config"
	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// ReadOnlyError refuses a write to a read-only store. Operations return it
// inside an *Error (KindConflict, "store.read_only"); the message is
// "<store> is read-only (<why>)".
type ReadOnlyError struct {
	Store  string
	Access sync.Access
}

func (e *ReadOnlyError) Error() string {
	return fmt.Sprintf("%s is read-only (%s)", e.Store, e.Access.Why())
}

// checkWritable refuses when s is read-only. The store is named as
// config.yaml names it, or as its store.yaml does when config.yaml has no
// store in that directory.
//
// An access.json that cannot be read refuses every write on purpose: a
// store whose state is unknown is not written to. The error says which file
// to delete, or a sync, which judges again.
func (ss *Session) checkWritable(s *store.Store) error {
	acc, err := sync.ReadAccess(s)
	if err != nil {
		return fmt.Errorf("%w; writes are refused until it is fixed: delete %s, or run hedgehug sync, which judges again",
			err, filepath.Join(s.Dir(), store.LocalDir, "access.json"))
	}
	if !acc.ReadOnly {
		return nil
	}
	name := s.Meta().Name
	if c, _, err := ss.Config(); err == nil {
		for _, e := range c.Stores {
			if e.Path == s.Dir() {
				name = e.Name
			}
		}
	}
	return newErr(KindConflict, "store.read_only", &ReadOnlyError{Store: name, Access: acc},
		"store", name, "reason", acc.Reason)
}

// OpenForWrite opens the store and its index for an operation that writes. A
// read-only store is refused here, before the index is read, so that the
// refusal comes before an id is resolved or anything else is checked.
func (ss *Session) OpenForWrite() (*store.Store, *index.Index, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return nil, nil, err
	}
	if err := ss.checkWritable(s); err != nil {
		return nil, nil, err
	}
	ix, err := index.Open(s)
	if err != nil {
		return nil, nil, err
	}
	return s, ix, nil
}

// githubRepoOf is the GitHub repository that origin of e is, or nil: origin
// is not on a GitHub host, e has no remote, or no account asks. It looks at
// the URL as written, before url_rewrites: a store of github.com that the
// development environment keeps in a local bare is still checked on GitHub.
func (ss *Session) githubRepoOf(c *config.Config, e config.StoreEntry) *sync.GitHubRepo {
	if len(e.Remotes) == 0 {
		return nil
	}
	// The host decides, in any form of a URL (https, ssh://, user@host:path).
	host, owner, name, ok := hhctx.Split(e.Remotes[0])
	if !ok || !c.IsGitHubHost(host) {
		return nil
	}
	acct, ok := c.Writer(e)
	if !ok {
		return nil
	}
	return &sync.GitHubRepo{
		Host: host, Owner: owner, Name: name, AccountID: acct.ID(), AccountName: acct.Name,
		Client: func() (*github.Client, error) {
			if !config.SameHost(host, acct.Host) {
				// The token of the account must not go to another host.
				return nil, newErr(KindInvalid, "account.host_mismatch",
					fmt.Errorf("%s is on %s, but account %s is on %s", e.Remotes[0], host, acct.Name, acct.Host), "account", acct.Name)
			}
			return ss.GitHub(c, acct, host)
		},
	}
}
