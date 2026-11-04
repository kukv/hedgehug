package maintenance

import (
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// Rebuilt is what doctor --fix did to a store a sync found force pushed: it
// cloned the store again from origin.
type Rebuilt struct {
	Replayed int      `json:"replayed"`       // the unpushed commits put back, as one commit
	Dropped  []string `json:"dropped"`        // the ids of the tasks the new history lacks, whose unpushed changes were not put back
	Kept     string   `json:"kept,omitempty"` // where the old store is, when some changes were not put back
}

// rebuildStore clones the store e again from origin and puts the unpushed
// changes back (replicate.MakeRebuild), when a sync found its origin force pushed
// (force-push.json). It swaps the new store in after the sync lock is let
// go, and does not push. It returns nil when the store was not force pushed,
// does not open, origin follows it again, or the store changed while it was
// made again (a warning says to run doctor --fix again).
func rebuildStore(ss *usecase.Session, c *config.Config, e config.StoreEntry) (*Rebuilt, error) {
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return nil, nil // the findings tell of a store that does not open
	}
	defer func() { _ = s.Repo().Close() }() // a second close, after the explicit one below, does nothing
	if _, ok, err := replicate.ReadForcePush(s); err != nil || !ok {
		return nil, err
	}
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		return nil, err
	}
	var b *replicate.Rebuild
	err = s.WithLocalLock("sync.lock", usecase.SyncLockWait, func() error {
		var err error
		b, err = replicate.MakeRebuild(s, opt)
		return err
	})
	if err != nil || b == nil {
		return nil, err
	}
	dir := s.Dir()
	if ss.BeforeSwap != nil {
		ss.BeforeSwap(dir)
	}
	ss.CloseMounted(dir)
	ss.DropMount()
	if err := s.Repo().Close(); err != nil {
		replicate.Discard(b)
		return nil, err
	}
	err = replicate.Swap(dir, b)
	if errors.Is(err, replicate.ErrRebuildStale) {
		ss.AddWarning(fmt.Sprintf("%s changed while it was cloned again, so it was not replaced; run hedgehug doctor --fix again", e.Name))
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, w := range b.Warnings {
		ss.AddWarning(e.Name + ": " + w)
	}
	// The index is a cache: the swapped-in store works without it.
	if ns, err := ss.OpenStoreAt(dir); err == nil {
		if _, err := ss.OpenHistory(ns, e.Name); err != nil {
			ss.AddWarning(fmt.Sprintf("%s: could not index its history: %v", e.Name, err))
		}
		_ = ns.Repo().Close()
	}
	if left, lerr := store.FindLeftover(dir); lerr == nil && !left.None() {
		ss.AddWarning(store.LeftoverMessage(e.Name))
	}
	return &Rebuilt{Replayed: b.Replayed, Dropped: append([]string{}, b.Purged...), Kept: b.Kept}, nil
}
