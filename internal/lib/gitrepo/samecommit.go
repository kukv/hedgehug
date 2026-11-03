package gitrepo

import (
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FindSameCommit returns a commit of the history of from that has the same
// author and committer (name, email and time) as the commit like: a history
// rewrite keeps these, so it finds where like went. Both commits must be in
// the repository. Commits of one author and committer in the same second
// cannot be told apart, so it returns the oldest of them that the walk from
// from meets (a line of history is walked from the newest): a caller that
// goes on from it sees the commits again at worst, never misses one.
func (r *Repo) FindSameCommit(from, like plumbing.Hash) (plumbing.Hash, bool, error) {
	want, err := r.commit(like)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	start, err := r.commit(from)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	same := func(a, b object.Signature) bool {
		return a.Name == b.Name && a.Email == b.Email && a.When.Equal(b.When)
	}
	var found plumbing.Hash
	err = object.NewCommitPreorderIter(start, nil, nil).ForEach(func(c *object.Commit) error {
		if same(c.Author, want.Author) && same(c.Committer, want.Committer) {
			found = c.Hash
		}
		return nil
	})
	return found, !found.IsZero(), err
}
