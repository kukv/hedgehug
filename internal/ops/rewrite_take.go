package ops

import (
	"errors"
	"io/fs"
	"os"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// finishRebuild completes a swap after a history rewrite that stopped, or
// removes what a rebuild left, for the store e, before it is opened (spec:
// 作り直しと未 push の変更の取り込み, the part on a failed step 5). Nothing
// left is nothing done. While the store is there, the leftover is touched
// only under sync.lock: a sync that makes a rebuild holds it, so a rebuild
// in the making is not taken for a leftover.
func (ss *Session) finishRebuild(e config.StoreEntry) error {
	l, err := sync.FindLeftover(e.Path)
	if err != nil || l.None() {
		return err
	}
	s, err := store.Open(e.Path)
	if err != nil {
		// No store (or a broken one) to lock: the swap is finished as it is.
		if _, serr := os.Lstat(e.Path); errors.Is(serr, fs.ErrNotExist) {
			return sync.FinishLeftover(e.Path, l)
		}
		return nil // the store's own problem; the sync tells of it
	}
	defer func() { _ = s.Repo().Close() }()
	return s.WithLocalLock("sync.lock", syncLockWait, func() error {
		l, err := sync.FindLeftover(e.Path)
		if err != nil || l.None() {
			return err
		}
		return sync.FinishLeftover(e.Path, l)
	})
}

// rewriteLeftoverFinding is rewrite_leftover (warning): what a rebuild after
// a history rewrite left beside the store e.
func rewriteLeftoverFinding(e config.StoreEntry) ([]doctor.Finding, error) {
	l, err := sync.FindLeftover(e.Path)
	if err != nil || l.None() {
		return nil, err
	}
	var rest []string
	if l.New != "" {
		rest = append(rest, l.New)
	}
	rest = append(rest, l.Before...)
	out := make([]doctor.Finding, len(rest))
	for i, p := range rest {
		out[i] = doctor.Finding{Check: "rewrite_leftover", Where: e.Path, Msg: p + " is left from a rebuild after a history rewrite", Hint: "run hedgehug doctor --fix", Severity: doctor.Warning}
	}
	return out, nil
}
