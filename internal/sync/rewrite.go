package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

const (
	appliedFile   = "rewrites-applied.json"
	forcePushFile = "force-push.json"
)

// ErrForcePush means the history of origin was rewritten and Hedgehug has no
// record of it (D23). The store is as it was.
var ErrForcePush = errors.New("the history of origin was rewritten without a record of Hedgehug; check it with hedgehug doctor")

// RewriteState is what a fetch of origin found.
type RewriteState int

// The states.
const (
	NoRewrite  RewriteState = iota
	Recorded                // a rewrite with a record not yet taken in (spec)
	Unexpected              // a force push without a record (D23)
)

// ForcePush is force-push.json: the unexpected force push the last sync
// found (D23).
type ForcePush struct {
	Origin  string    `json:"origin"`  // the new head of origin
	Tracked string    `json:"tracked"` // the origin the clone had followed
	Checked time.Time `json:"checked"`
}

// Detect compares the head of origin after a fetch with tracked, the value
// refs/remotes/origin/main had before it, as spec says. The record returned
// with Recorded is the newest one not in applied whose before is tracked or
// a descendant of it. A before the clone never fetched cannot be compared,
// so the record counts when the new history holds a commit with the same
// author and committer as tracked (D33).
func Detect(r *gitrepo.Repo, tracked, head plumbing.Hash, applied []time.Time) (RewriteState, *model.Rewrite, error) {
	if head.IsZero() {
		return NoRewrite, nil, nil
	}
	ok, err := r.IsAncestor(tracked, head)
	if err != nil || ok {
		return NoRewrite, nil, err
	}
	data, err := r.ReadAt(head, model.RewritesPath)
	if errors.Is(err, fs.ErrNotExist) {
		return Unexpected, nil, nil
	}
	if err != nil {
		return Unexpected, nil, err
	}
	recs, err := model.DecodeRewrites(data)
	if err != nil {
		return Unexpected, nil, nil //nolint:nilerr // a record that does not decode is no record
	}
	for i := len(recs) - 1; i >= 0; i-- {
		rec := recs[i]
		if appliedHas(applied, rec.At) {
			continue
		}
		before := plumbing.NewHash(rec.Before)
		if before.IsZero() {
			continue
		}
		ok, err := r.IsAncestor(tracked, before)
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			// The clone never fetched before, which the new history does
			// not reach. The rewrite keeps the author and committer of
			// every commit, so the commit the clone followed is found in
			// the new history, if the record is the rewrite of that
			// history (D33).
			ok, err = r.HasSameCommit(head, tracked)
		}
		if err != nil {
			return Unexpected, nil, err
		}
		if ok {
			return Recorded, &rec, nil
		}
	}
	return Unexpected, nil, nil
}

func appliedHas(applied []time.Time, at time.Time) bool {
	for _, a := range applied {
		if a.Equal(at) {
			return true
		}
	}
	return false
}

// ancestorOrMissing is IsAncestor, false when a commit is not in the store.
func ancestorOrMissing(r *gitrepo.Repo, a, b plumbing.Hash) (bool, error) {
	ok, err := r.IsAncestor(a, b)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return false, nil
	}
	return ok, err
}

// staleMirror tells whether the mirror whose main is at mirror still holds
// a history that HEAD's rewrites.yaml says was rewritten: it has a before
// that HEAD does not. Such a mirror is not merged but force pushed to.
func staleMirror(r *gitrepo.Repo, head, mirror plumbing.Hash) (bool, error) {
	if head.IsZero() || mirror.IsZero() {
		return false, nil
	}
	data, err := r.ReadAt(head, model.RewritesPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	recs, err := model.DecodeRewrites(data)
	if err != nil {
		return false, nil //nolint:nilerr // a broken record says nothing
	}
	for _, rec := range recs {
		before := plumbing.NewHash(rec.Before)
		if before.IsZero() {
			continue
		}
		in, err := ancestorOrMissing(r, before, mirror)
		if err != nil {
			return false, err
		}
		if !in {
			continue
		}
		mine, err := ancestorOrMissing(r, before, head)
		if err != nil {
			return false, err
		}
		if !mine {
			return true, nil
		}
	}
	return false, nil
}

// ReadApplied reads rewrites-applied.json: the at of the rewrites taken in.
// A missing file has none.
func ReadApplied(s *store.Store) ([]time.Time, error) {
	b, err := s.ReadLocal(appliedFile)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var at []time.Time
	if err := json.Unmarshal(b, &at); err != nil {
		return nil, fmt.Errorf("%s: %w", appliedFile, err)
	}
	return at, nil
}

// WriteApplied replaces rewrites-applied.json. Callers hold sync.lock.
func WriteApplied(s *store.Store, at []time.Time) error {
	if at == nil {
		at = []time.Time{}
	}
	b, err := json.Marshal(at)
	if err != nil {
		return err
	}
	return s.WriteLocal(appliedFile, append(b, '\n'))
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

// checkOrigin runs Detect for the fetch of origin that moved its tracking
// ref from tracked. On Recorded and Unexpected it puts the tracking ref
// back (D23), so nothing of the clone changes; Unexpected also writes
// force-push.json and returns ErrForcePush. NoRewrite removes
// force-push.json.
func checkOrigin(s *store.Store, tracked plumbing.Hash, opt Options, res *Result) error {
	r := s.Repo()
	head, err := r.RemoteHead("origin")
	if err != nil {
		return err
	}
	applied, err := ReadApplied(s)
	if err != nil {
		return err
	}
	state, rec, err := Detect(r, tracked, head, applied)
	if err != nil {
		return err
	}
	switch state {
	case NoRewrite:
		// A missing file is not an error for RemoveLocal, so this writes
		// nothing in the usual sync.
		return s.RemoveLocal(forcePushFile)
	case Recorded:
		res.Rewrite = rec
		return r.SetTracking("origin", tracked)
	}
	if err := r.SetTracking("origin", tracked); err != nil {
		return err
	}
	if err := writeForcePush(s, ForcePush{Origin: head.String(), Tracked: tracked.String(), Checked: opt.Now()}); err != nil {
		return err
	}
	return ErrForcePush
}
