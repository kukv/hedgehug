package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
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

// mirrorState is how a mirror stands to the rewrites HEAD records.
type mirrorState int

const (
	mirrorCurrent  mirrorState = iota // merged and pushed as usual
	mirrorStale                       // only the history before a rewrite: force pushed
	mirrorDiverged                    // holds commits of neither history: left alone
)

// staleMirror tells how the mirror whose main is at mirror stands to the
// rewrites in HEAD's rewrites.yaml. A mirror that is at or behind a before
// HEAD lacks, or holds one, has the history that was rewritten: it is not
// merged (that brings back what the rewrite removed) but force pushed to.
// A mirror that holds a before and commits on top of it has commits that
// neither history has; force pushing would drop them, so it is left alone.
func staleMirror(r *gitrepo.Repo, head, mirror plumbing.Hash) (mirrorState, error) {
	if head.IsZero() || mirror.IsZero() {
		return mirrorCurrent, nil
	}
	mine, err := ancestorOrMissing(r, mirror, head)
	if err != nil || mine {
		return mirrorCurrent, err
	}
	data, err := r.ReadAt(head, model.RewritesPath)
	if errors.Is(err, fs.ErrNotExist) {
		return mirrorCurrent, nil
	}
	if err != nil {
		return mirrorCurrent, err
	}
	recs, err := model.DecodeRewrites(data)
	if err != nil {
		return mirrorCurrent, nil //nolint:nilerr // a broken record says nothing
	}
	state := mirrorCurrent
	for _, rec := range recs {
		before := plumbing.NewHash(rec.Before)
		if before.IsZero() {
			continue
		}
		kept, err := ancestorOrMissing(r, before, head)
		if err != nil {
			return mirrorCurrent, err
		}
		if kept {
			continue
		}
		behind, err := ancestorOrMissing(r, mirror, before)
		if err != nil {
			return mirrorCurrent, err
		}
		if behind {
			return mirrorStale, nil
		}
		holds, err := ancestorOrMissing(r, before, mirror)
		if err != nil {
			return mirrorCurrent, err
		}
		if holds {
			state = mirrorDiverged
		}
	}
	if state == mirrorCurrent {
		// A mirror that branched off before a before and has commits of its
		// own can still hold what a rewrite purged; merging it brings that
		// back. One that does not touch a purged file is merged as usual.
		purged, err := mirrorHoldsPurged(r, head, mirror, recs)
		if err != nil {
			return mirrorCurrent, err
		}
		if purged {
			return mirrorDiverged, nil
		}
	}
	return state, nil
}

// mirrorHoldsPurged tells whether a commit that only the mirror has (not in
// HEAD's history) writes a file of a task a rewrite of recs purged: the task
// file, or a comment or worklog of it.
func mirrorHoldsPurged(r *gitrepo.Repo, head, mirror plumbing.Hash, recs []model.Rewrite) (bool, error) {
	var ulids []string
	for _, rec := range recs {
		for _, u := range rec.ULIDs {
			ulids = append(ulids, u.String())
		}
	}
	if len(ulids) == 0 {
		return false, nil
	}
	purged := func(p string) bool {
		return slices.ContainsFunc(ulids, func(u string) bool { return strings.Contains(p, u) })
	}
	own, err := r.Between(mirror, head)
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return false, nil
		}
		return false, err
	}
	for _, c := range own {
		to, err := r.TreeOf(c.Hash)
		if err != nil {
			return false, err
		}
		if len(c.Parents) == 0 {
			for p, err := range r.Files(to, "") {
				if err != nil {
					return false, err
				}
				if purged(p) {
					return true, nil
				}
			}
			continue
		}
		from, err := r.TreeOf(c.Parents[0])
		if err != nil {
			return false, err
		}
		changes, err := r.DiffTrees(from, to, "")
		if err != nil {
			return false, err
		}
		for _, ch := range changes {
			if !ch.Deleted && purged(ch.Path) {
				return true, nil
			}
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
