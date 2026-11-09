package merge

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// record merges a comment or a worklog entry, whose fields belong together:
// the side that updated it last takes the whole file, body and all. The
// losing file is recorded whole, as a JSON string. adopt, when given, puts
// into the winning file what must outlive the loss (the gh of a comment);
// what it adopted is not an overwrite.
func (m *merger) record(ours, theirs Side, updated func([]byte) (time.Time, error), adopt func(win, lose []byte) ([]byte, bool)) (Result, error) {
	if ours.Data == nil || theirs.Data == nil {
		return m.revive(ours, theirs), nil
	}
	ot, err1 := updated(ours.Data)
	tt, err2 := updated(theirs.Data)
	if err1 != nil || err2 != nil {
		return m.other(ours, theirs)
	}
	win, lose := theirs, ours
	if m.p == OursWins || later(ot, tt, ours.Author, theirs.Author, ours.Data, theirs.Data) {
		win, lose = ours, theirs
	}
	r := Result{Data: win.Data}
	if adopt != nil {
		data, gone := adopt(win.Data, lose.Data)
		if data != nil {
			r.Data = data
		}
		if gone {
			return r, nil
		}
	}
	if m.p == LWW {
		v, _ := json.Marshal(string(lose.Data)) // a string always encodes
		r.Overwritten = []Overwritten{{Path: m.path, Field: "*", Value: string(v), Author: lose.Author}}
	}
	return r, nil
}

func commentUpdated(data []byte) (time.Time, error) {
	c, err := model.DecodeComment(data)
	if err != nil {
		return time.Time{}, err
	}
	return c.Updated, nil
}

func worklogUpdated(data []byte) (time.Time, error) {
	w, err := model.DecodeWorklog(data)
	if err != nil {
		return time.Time{}, err
	}
	return w.Updated, nil
}

// adoptCommentGH adds to the winning comment the elements of gh of the losing
// one whose ref the winner lacks: the push that recorded them did not edit the
// comment, so an edit elsewhere must not lose them. An element with a ref both
// have stays as the winner has it. data is the winner with them added, or nil
// when there is nothing to add. nothingLost is true when the loser changed
// nothing but gh since the base: gh is a record of the machine, not something
// a person loses, so the loss is no overwrite.
func adoptCommentGH(base, win, lose []byte) (data []byte, nothingLost bool) {
	w, err1 := model.DecodeComment(win)
	l, err2 := model.DecodeComment(lose)
	b, err3 := model.DecodeComment(base) // fails with no base: the loser then overwrote
	if err1 != nil || err2 != nil {
		return nil, false
	}
	rest := func(c *model.Comment) []byte {
		out, _ := (&model.Comment{ULID: c.ULID, Author: c.Author, Created: c.Created, Updated: c.Updated, DeletedAt: c.DeletedAt, Extra: c.Extra, Body: c.Body}).Encode()
		return out
	}
	nothingLost = err3 == nil && bytes.Equal(rest(b), rest(l))
	wl, err1 := w.GitHub()
	ll, err2 := l.GitHub()
	if err1 != nil || err2 != nil {
		return nil, false
	}
	merged := slices.Clone(wl)
	for _, x := range ll {
		if !slices.ContainsFunc(wl, func(y model.CommentLink) bool { return y.Ref == x.Ref }) {
			merged = append(merged, x)
		}
	}
	if len(merged) == len(wl) {
		return nil, nothingLost
	}
	if err := w.SetGitHub(merged); err != nil {
		return nil, false
	}
	out, err := w.Encode()
	if err != nil {
		return nil, false
	}
	return out, nothingLost
}
