package replicate

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

const forcePushFile = "force-push.json"

// ErrForcePush means the history of origin is not one the store follows: it
// was rewritten or reset. The store is as it was; hedgehug doctor --fix
// clones it again.
var ErrForcePush = errors.New("the history of origin differs from this store's; check origin, then run hedgehug doctor --fix to clone it again")

// ForcePush is force-push.json: the force push the last sync found.
type ForcePush struct {
	Origin  string    `json:"origin"`  // the new head of origin
	Tracked string    `json:"tracked"` // the origin the clone had followed
	Checked time.Time `json:"checked"`
}

// ReadForcePush reads force-push.json; ok is false when there is none.
func ReadForcePush(s *store.Store) (ForcePush, bool, error) {
	var f ForcePush
	b, err := s.ReadLocal(forcePushFile)
	if errors.Is(err, store.ErrNotFound) {
		return f, false, nil
	}
	if err != nil {
		return f, false, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return ForcePush{}, false, fmt.Errorf("%s: %w", forcePushFile, err)
	}
	return f, true, nil
}

func writeForcePush(s *store.Store, f ForcePush) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return s.WriteLocal(forcePushFile, append(b, '\n'))
}

// checkOrigin compares the head of origin after a fetch with tracked, the
// value refs/remotes/origin/main had before it. A head that does not
// descend from tracked is a force push: it puts the tracking ref back, so
// nothing of the clone changes, writes force-push.json and returns
// ErrForcePush. Otherwise it removes force-push.json.
func checkOrigin(s *store.Store, tracked plumbing.Hash, opt Options) error {
	r := s.Repo()
	head, err := r.RemoteHead("origin")
	if err != nil {
		return err
	}
	follows := true
	if !head.IsZero() {
		if follows, err = r.IsAncestor(tracked, head); err != nil {
			return err
		}
	}
	if follows {
		// A missing file is not an error for RemoveLocal, so this writes
		// nothing in the usual sync.
		return s.RemoveLocal(forcePushFile)
	}
	if err := r.SetTracking("origin", tracked); err != nil {
		return err
	}
	if err := writeForcePush(s, ForcePush{Origin: head.String(), Tracked: tracked.String(), Checked: opt.Now()}); err != nil {
		return err
	}
	return ErrForcePush
}
