package store

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// validateSprintsAndReleases checks the sprints and releases put in the
// operation: each names a workspace that is in the store as the operation
// leaves it.
func (tx *Tx) validateSprintsAndReleases() error {
	for _, ulid := range slices.SortedFunc(maps.Keys(tx.sprints), compareULIDs) {
		if err := tx.checkOwner("sprint", ulid, tx.sprints[ulid].Workspace); err != nil {
			return err
		}
	}
	for _, ulid := range slices.SortedFunc(maps.Keys(tx.releases), compareULIDs) {
		if err := tx.checkOwner("release", ulid, tx.releases[ulid].Workspace); err != nil {
			return err
		}
	}
	return nil
}

func (tx *Tx) checkOwner(what string, ulid, ws model.ULID) error {
	if ws.IsZero() {
		return fmt.Errorf("%s %s has no workspace", what, ulid)
	}
	if _, err := tx.workspace(ws); errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%s %s: workspace %s is not in this store", what, ulid, ws)
	} else if err != nil {
		return err
	}
	return nil
}

// sprintReleaseViolations returns the violations of the sprint and the
// releases that a write of t over old adds; old is nil for a new task. Like
// checkRefs it checks only what is new, so a task that a sync left with a
// sprint of another workspace can still be edited. A sprint or release whose
// file does not decode has no workspace to compare, so it passes.
func (tx *Tx) sprintReleaseViolations(old, t *model.Task) ([]schema.Violation, error) {
	var vs []schema.Violation
	if t.Sprint != nil && (old == nil || old.Sprint == nil || *old.Sprint != *t.Sprint) {
		v, err := tx.refViolation("sprint", "sprint", *t.Sprint, t.Workspace)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v...)
	}
	checked := map[model.ULID]bool{}
	for _, ulid := range t.Releases {
		if checked[ulid] || (old != nil && slices.Contains(old.Releases, ulid)) {
			continue
		}
		checked[ulid] = true
		v, err := tx.refViolation("releases", "release", ulid, t.Workspace)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v...)
	}
	return vs, nil
}

// refViolation checks that the sprint or release ulid (what says which) is in
// the store and in the workspace ws, as the operation leaves them.
func (tx *Tx) refViolation(field, what string, ulid, ws model.ULID) ([]schema.Violation, error) {
	var owner model.ULID
	if what == "sprint" {
		sp, err := tx.sprint(ulid)
		if err != nil {
			return refReadViolation(field, what, ulid, err)
		}
		owner = sp.Workspace
	} else {
		r, err := tx.release(ulid)
		if err != nil {
			return refReadViolation(field, what, ulid, err)
		}
		owner = r.Workspace
	}
	if owner != ws {
		return []schema.Violation{{Code: CodeRefWorkspace, Field: field, Msg: fmt.Sprintf("%s %s is not in the workspace of this task", what, ulid)}}, nil
	}
	return nil, nil
}

func refReadViolation(field, what string, ulid model.ULID, err error) ([]schema.Violation, error) {
	switch {
	case errors.Is(err, ErrNotFound):
		return []schema.Violation{{Code: schema.CodeMissingRef, Field: field, Msg: fmt.Sprintf("%s %s is not in this store", what, ulid)}}, nil
	case errors.As(err, new(*model.DecodeError)):
		return nil, nil
	}
	return nil, err
}

// checkWorkspacesEmptyOfSprints refuses to remove a workspace that still has
// a sprint or a release, as the operation leaves them: the ones put count as
// they will be, the others as they are on disk.
func (tx *Tx) checkWorkspacesEmptyOfSprints() error {
	sprints, releases := map[model.ULID]int{}, map[model.ULID]int{}
	for _, sp := range tx.sprints {
		sprints[sp.Workspace]++
	}
	for _, r := range tx.releases {
		releases[r.Workspace]++
	}
	for sp, err := range tx.s.Sprints() {
		if err == nil && tx.sprints[sp.ULID] == nil {
			sprints[sp.Workspace]++
		}
	}
	for r, err := range tx.s.Releases() {
		if err == nil && tx.releases[r.ULID] == nil {
			releases[r.Workspace]++
		}
	}
	for _, ulid := range slices.SortedFunc(maps.Keys(tx.workspaces), compareULIDs) {
		if tx.workspaces[ulid] != nil {
			continue
		}
		if sprints[ulid] > 0 {
			return fmt.Errorf("workspace %s still has %d sprints", ulid, sprints[ulid])
		}
		if releases[ulid] > 0 {
			return fmt.Errorf("workspace %s still has %d releases", ulid, releases[ulid])
		}
	}
	return nil
}

func compareULIDs(a, b model.ULID) int { return strings.Compare(a.String(), b.String()) }
