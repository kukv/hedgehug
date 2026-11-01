package merge

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/model"
)

// forward merges a task file of which a side is a forward (the task moved to
// another store). The forward wins whatever the other side did to the task,
// and the other side's file is recorded whole as lost, a JSON string, or
// null when it removed the file. The task is not here any more, so the
// change is not merged field by field; the inbox tells where to apply it.
//
// Two forwards that name different stores are merged by moved_to alone: the
// side that changed the file last wins, and the other store is recorded.
// Under OursWins ours takes the file.
func (m *merger) forward(ours, theirs Side) (Result, error) {
	of, tf := model.IsForward(ours.Data), model.IsForward(theirs.Data)
	win, lose := ours, theirs
	switch {
	case m.p == OursWins:
	case of && tf:
		if !later(ours.Time, theirs.Time, ours.Author, theirs.Author, ours.Data, theirs.Data) {
			win, lose = theirs, ours
		}
	case tf:
		win, lose = theirs, ours
	}
	r := Result{Data: win.Data}
	if m.p != LWW {
		return r, nil
	}
	if of && tf {
		f, err := model.DecodeForward(lose.Data)
		if err != nil {
			return m.other(ours, theirs) // a forward that does not decode
		}
		// The same destination written in another way loses nothing.
		if w, err := model.DecodeForward(win.Data); err == nil && w.MovedTo == f.MovedTo {
			return r, nil
		}
		r.Overwritten = []Overwritten{{Path: m.path, Field: "moved_to", Value: string(jsonOf(f.MovedTo.String())), Author: lose.Author}}
		return r, nil
	}
	v := []byte("null")
	if lose.Data != nil {
		v, _ = json.Marshal(string(lose.Data)) // a string always encodes
	}
	r.Overwritten = []Overwritten{{Path: m.path, Field: "*", Value: string(v), Author: lose.Author}}
	return r, nil
}
