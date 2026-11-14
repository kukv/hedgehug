package saves

import "github.com/go-git/go-git/v5/plumbing/object"

// baseFunc は、合わせの共通の祖先(2 つの親の MergeBase の 1 つ目。同期の合わせと同じ)のツリー。祖先がなければ nil
// (同期は、共通の祖先のない合わせを作らない)。
type baseFunc func() (*object.Tree, error)

// lazyBase は、最初に呼ばれたときだけ c の共通の祖先を探し、結果を覚える baseFunc。
func lazyBase(c *object.Commit) baseFunc {
	var (
		done bool
		tree *object.Tree
		err  error
	)
	return func() (*object.Tree, error) {
		if done {
			return tree, err
		}
		done = true
		var ours, theirs *object.Commit
		if ours, err = c.Parent(0); err != nil {
			return nil, err
		}
		if theirs, err = c.Parent(1); err != nil {
			return nil, err
		}
		bases, merr := ours.MergeBase(theirs)
		if merr != nil || len(bases) == 0 {
			err = merr
			return nil, err
		}
		tree, err = bases[0].Tree()
		return tree, err
	}
}
