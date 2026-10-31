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
	if m.p == LWW && lose.Data != nil {
		r.Overwritten = []Overwritten{{
			Path:   m.path,
			Field:  "*",
			Value:  plumbing.ComputeHash(plumbing.BlobObject, lose.Data).String(),
			Author: lose.Author,
		}}
	}
	return r, nil
}
