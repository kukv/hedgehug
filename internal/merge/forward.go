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
// Two forwards that name the same store are merged by moved_at: the later one
// is kept. Under OursWins ours takes the file.
func (m *merger) forward(ours, theirs Side) (Result, error) {
	of, tf := model.IsForward(ours.Data), model.IsForward(theirs.Data)
	theirsWin := false
	// the two forwards as they decode (nil when one does not), read once
	var fo, ft *model.Forward
	if of && tf {
		fo, _ = model.DecodeForward(ours.Data)
		ft, _ = model.DecodeForward(theirs.Data)
	}
	switch {
	case m.p == OursWins:
	case of && tf:
		theirsWin = !later(ours.Time, theirs.Time, ours.Author, theirs.Author, ours.Data, theirs.Data)
		// the same destination: the later move is the one to keep
		if fo != nil && ft != nil && fo.MovedTo == ft.MovedTo && !fo.MovedAt.Equal(ft.MovedAt) {
			theirsWin = ft.MovedAt.After(fo.MovedAt)
		}
	case tf:
		theirsWin = true
	}
	win, lose, fwin, flose := ours, theirs, fo, ft
	if theirsWin {
		win, lose, fwin, flose = theirs, ours, ft, fo
	}
	r := Result{Data: win.Data}
	if m.p != LWW {
		return r, nil
	}
	if of && tf {
		if flose == nil {
			return m.other(ours, theirs) // a forward that does not decode
		}
		// The same destination written in another way loses nothing.
		if fwin != nil && fwin.MovedTo == flose.MovedTo {
			return r, nil
		}
		r.Overwritten = []Overwritten{{Path: m.path, Field: "moved_to", Value: string(jsonOf(flose.MovedTo.String())), Author: lose.Author}}
		return r, nil
	}
	v := []byte("null")
	if lose.Data != nil {
		v, _ = json.Marshal(string(lose.Data)) // a string always encodes
	}
	r.Overwritten = []Overwritten{{Path: m.path, Field: "*", Value: string(v), Author: lose.Author}}
	return r, nil
}
