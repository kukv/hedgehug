package merge

import "github.com/go-git/go-git/v5/plumbing"

// other merges a file that is not read field by field: the side that changed
// it last takes the whole file. The losing file is recorded by its git blob
// hash, which finds it in the history.
func (m *merger) other(ours, theirs Side) (Result, error) {
	win, lose := theirs, ours
	if m.p == OursWins || later(ours.Time, theirs.Time, ours.Author, theirs.Author, ours.Data, theirs.Data) {
		win, lose = ours, theirs
	}
	r := Result{Data: win.Data}
	if lose.Data != nil {
		r.Overwritten = m.blobLost(lose)
	}
	return r, nil
}

// blobLost records the whole file of s as lost, by its git blob hash.
// Nothing is recorded under OursWins.
func (m *merger) blobLost(s Side) []Overwritten {
	if m.p != LWW {
		return nil
	}
	return []Overwritten{{
		Path:   m.path,
		Field:  "*",
		Value:  plumbing.ComputeHash(plumbing.BlobObject, s.Data).String(),
		Author: s.Author,
	}}
}
