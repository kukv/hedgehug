package ops

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// ForwardView is a forward file as purge --moved lists it.
type ForwardView struct {
	ULID    model.ULID `json:"ulid"`
	ID      string     `json:"id,omitempty"`       // as ForwardEntry knows it
	ToStore string     `json:"to_store,omitempty"` // the mounted store it moved to; "" when not mounted
	ToULID  model.ULID `json:"-"`                  // the ulid of the store it moved to
	ToID    string     `json:"to_id,omitempty"`    // its id there, when found
}

// forwardViews describes the forwards of ix, the index of the store name.
// A forward file that does not decode is not one to remove: it is counted
// in broken, for doctor.
func (ss *Session) forwardViews(name string, ix *index.Index) (views []ForwardView, broken int, err error) {
	set, err := ss.Mount()
	if err != nil {
		return nil, 0, err
	}
	from, _ := set.Get(name)
	out := []ForwardView{}
	for _, f := range ix.Forwards() {
		if f.Broken != "" {
			broken++
			continue
		}
		v := ForwardView{ULID: f.ULID, ID: f.ID, ToULID: f.MovedTo}
		if m := set.Moved(from, "", f); m.To != nil {
			v.ToStore = m.To.Name
			v.ToID = m.NowID()
		}
		out = append(out, v)
	}
	return out, broken, nil
}

// warnBrokenForwards tells of the forward files that were left because they
// do not decode.
func (ss *Session) warnBrokenForwards(broken int) {
	if broken > 0 {
		ss.warn(fmt.Sprintf("%d broken forward files left; see hedgehug doctor", broken))
	}
}

// PurgeMovedList lists the forwards of the store (a store-unit command) and
// warns of the broken ones, which are not listed.
func (ss *Session) PurgeMovedList() ([]ForwardView, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	_, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	list, broken, err := ss.forwardViews(e.Name, ix)
	if err == nil {
		ss.warnBrokenForwards(broken)
	}
	return list, err
}

// PurgeMoved removes them in one commit (Hedgehug-Op: purge-moved). Without
// yes it is confirm.required. None is no commit and no error. With yes it
// removes the forwards as they are under the lock.
func (ss *Session) PurgeMoved(yes bool) (removed []ForwardView, commit string, err error) {
	return ss.purgeMoved(nil, yes)
}

// PurgeMovedConfirmed is PurgeMoved for the forwards the caller showed to the
// user and the user agreed to (PurgeMovedList lists them): it fails with
// purge.changed when the forwards under the lock are no longer those.
func (ss *Session) PurgeMovedConfirmed(confirmed []model.ULID) (removed []ForwardView, commit string, err error) {
	if len(confirmed) == 0 {
		return nil, "", confirmRequired("purge --moved")
	}
	return ss.purgeMoved(confirmed, true)
}

// purgeMoved is PurgeMoved; a nil confirmed skips the comparison.
func (ss *Session) purgeMoved(confirmed []model.ULID, yes bool) (removed []ForwardView, commit string, err error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, "", err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, "", err
	}
	removed, _, err = ss.forwardViews(e.Name, ix)
	if err != nil {
		return removed, "", err
	}
	if len(removed) == 0 && confirmed == nil {
		return removed, "", nil
	}
	if !yes {
		return nil, "", confirmRequired("purge --moved")
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "purge-moved", Detail: "forwards of " + e.Name}, func(tx *store.Tx) error {
		// Read again under the lock: a forward that came in by a sync since
		// is removed too when the removal was not confirmed one by one; if
		// it was, a change is refused (purge.changed).
		if err := ix.Refresh(); err != nil {
			return err
		}
		if removed, _, err = ss.forwardViews(e.Name, ix); err != nil {
			return err
		}
		if confirmed != nil && !slices.Equal(sortedForwardULIDs(removed), sortedULIDs(confirmed)) {
			return newErr(KindConflict, "purge.changed", errors.New("the forward files changed after the confirmation; run purge --moved again"))
		}
		for _, v := range removed {
			if err := tx.RemoveTask(v.ULID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return removed, commitHex(hash), nil
}

func sortedForwardULIDs(vs []ForwardView) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.ULID.String()
	}
	slices.Sort(out)
	return out
}

func sortedULIDs(us []model.ULID) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.String()
	}
	slices.Sort(out)
	return out
}
