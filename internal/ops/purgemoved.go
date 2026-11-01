package ops

import (
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
func (ss *Session) forwardViews(name string, ix *index.Index) ([]ForwardView, error) {
	set, err := ss.Mount()
	if err != nil {
		return nil, err
	}
	from, _ := set.Get(name)
	out := []ForwardView{}
	for _, f := range ix.Forwards() {
		v := ForwardView{ULID: f.ULID, ID: f.ID, ToULID: f.MovedTo}
		if m := set.Moved(from, "", f); m.To != nil {
			v.ToStore = m.To.Name
			v.ToID = m.NowID()
		}
		out = append(out, v)
	}
	return out, nil
}

// PurgeMovedList lists the forwards of the store (a store-unit command).
func (ss *Session) PurgeMovedList() ([]ForwardView, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	_, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, err
	}
	return ss.forwardViews(e.Name, ix)
}

// PurgeMoved removes them in one commit (Hedgehug-Op: purge-moved). Without
// yes it is confirm.required. None is no commit and no error.
func (ss *Session) PurgeMoved(yes bool) (removed []ForwardView, commit string, err error) {
	_, e, err := ss.Entry()
	if err != nil {
		return nil, "", err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return nil, "", err
	}
	removed, err = ss.forwardViews(e.Name, ix)
	if err != nil || len(removed) == 0 {
		return removed, "", err
	}
	if !yes {
		return nil, "", confirmRequired("purge --moved")
	}
	hash, err := ss.UpdateCommit(s, store.Op{Name: "purge-moved", Detail: "forwards of " + e.Name}, func(tx *store.Tx) error {
		// Read again under the lock: a forward that came in by a sync since
		// is removed too, so that none is left behind unlisted.
		if err := ix.Refresh(); err != nil {
			return err
		}
		if removed, err = ss.forwardViews(e.Name, ix); err != nil {
			return err
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
