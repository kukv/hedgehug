package ops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// finishRebuild completes a swap that stopped, or removes what a rebuild
// left, for the store e, before it is opened. Nothing left is nothing done.
// While the store is there, the leftover is touched only under sync.lock: a
// rebuild holds it, so a rebuild in the making is not taken for a leftover.
func (ss *Session) finishRebuild(e config.StoreEntry) error {
	l, err := sync.FindLeftover(e.Path)
	if err != nil || l.None() {
		return err
	}
	s, err := store.Open(e.Path)
	if err != nil {
		// No store (or a broken one) to lock: the swap is finished as it is.
		if _, serr := os.Lstat(e.Path); errors.Is(serr, fs.ErrNotExist) {
			return ss.finishLeftover(e, l)
		}
		return nil // the store's own problem; the sync tells of it
	}
	defer func() { _ = s.Repo().Close() }()
	return s.WithLocalLock("sync.lock", syncLockWait, func() error {
		l, err := sync.FindLeftover(e.Path)
		if err != nil || l.None() {
			return err
		}
		return ss.finishLeftover(e, l)
	})
}

// finishLeftover is sync.FinishLeftover, telling where an old store was kept.
func (ss *Session) finishLeftover(e config.StoreEntry, l sync.Leftover) error {
	kept, err := sync.FinishLeftoverKept(e.Path, l)
	for _, p := range kept {
		ss.warn(fmt.Sprintf("the old store of %s, with changes that were not put back, is kept at %s", e.Name, p))
	}
	return err
}

// Rebuilt is what doctor --fix did to a store a sync found force pushed: it
// cloned the store again from origin.
type Rebuilt struct {
	Replayed int      `json:"replayed"`       // the unpushed commits put back, as one commit
	Dropped  []string `json:"dropped"`        // the ids of the tasks the new history lacks, whose unpushed changes were not put back
	Kept     string   `json:"kept,omitempty"` // where the old store is, when some changes were not put back
}

// rebuildStore clones the store e again from origin and puts the unpushed
// changes back (sync.MakeRebuild), when a sync found its origin force pushed
// (force-push.json). It swaps the new store in after the sync lock is let
// go, and does not push. It returns nil when the store was not force pushed,
// does not open, origin follows it again, or the store changed while it was
// made again (a warning says to run doctor --fix again).
func (ss *Session) rebuildStore(c *config.Config, e config.StoreEntry) (*Rebuilt, error) {
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return nil, nil // the findings tell of a store that does not open
	}
	if _, ok, err := sync.ReadForcePush(s); err != nil || !ok {
		return nil, err
	}
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		return nil, err
	}
	var b *sync.Rebuild
	err = s.WithLocalLock("sync.lock", syncLockWait, func() error {
		var err error
		b, err = sync.MakeRebuild(s, opt)
		return err
	})
	if err != nil || b == nil {
		return nil, err
	}
	dir := s.Dir()
	if ss.BeforeSwap != nil {
		ss.BeforeSwap(dir)
	}
	ss.closeMounted(dir)
	ss.dropMount()
	if err := s.Repo().Close(); err != nil {
		sync.Discard(b)
		return nil, err
	}
	err = sync.Swap(dir, b)
	if errors.Is(err, sync.ErrRebuildStale) {
		ss.warn(fmt.Sprintf("%s changed while it was cloned again, so it was not replaced; run hedgehug doctor --fix again", e.Name))
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, w := range b.Warnings {
		ss.warn(e.Name + ": " + w)
	}
	if left, lerr := sync.FindLeftover(dir); lerr == nil && !left.None() {
		ss.warn(sync.LeftoverMessage(e.Name))
	}
	return &Rebuilt{Replayed: b.Replayed, Dropped: append([]string{}, b.Purged...), Kept: b.Kept}, nil
}
