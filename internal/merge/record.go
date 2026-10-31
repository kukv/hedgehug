package merge

import (
	"encoding/json"
	"time"

	"github.com/kukv/hedgehug/internal/model"
)

// record merges a comment or a worklog entry, whose fields belong together:
// the side that updated it last takes the whole file, body and all. The
// losing file is recorded whole, as a JSON string.
func (m *merger) record(ours, theirs Side, updated func([]byte) (time.Time, error)) (Result, error) {
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
