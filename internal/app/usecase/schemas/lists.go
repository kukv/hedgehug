package schemas

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// EditPriorities makes the edit of the priorities that values describe, as
// the edit of the values of an enum property: the tasks follow a name that
// changes, and a value that no edit keeps and a task uses (a deleted one
// counted) refuses the change.
func EditPriorities(ss *usecase.Session, values []schema.ValueEdit) (EditWrite, error) {
	return editList(ss, schema.TargetPriority, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, []string, error) {
		return sc.ChangePriorities(values)
	})
}

// EditLabels is EditPriorities for the labels.
func EditLabels(ss *usecase.Session, values []schema.ValueEdit) (EditWrite, error) {
	return editList(ss, schema.TargetLabel, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, []string, error) {
		return sc.ChangeLabels(values)
	})
}

// AddLabels puts the labels names at the end of the list.
func AddLabels(ss *usecase.Session, names []string) (EditWrite, error) {
	return editList(ss, schema.TargetLabel, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, []string, error) {
		eds := keepAll(sc.Labels)
		for _, n := range names {
			eds = append(eds, schema.ValueEdit{Name: n})
		}
		return sc.ChangeLabels(eds)
	})
}

// RemoveLabels takes the labels names off the list. A label that a task uses
// refuses the change.
func RemoveLabels(ss *usecase.Session, names []string) (EditWrite, error) {
	return editList(ss, schema.TargetLabel, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, []string, error) {
		gone := map[string]bool{}
		for _, n := range names {
			v, ok := schema.MatchValue(sc.Labels, n)
			if !ok {
				return nil, nil, nil, fmt.Errorf("label %q: %w", n, schema.ErrNotDefined)
			}
			gone[v] = true
		}
		var keep []string
		for _, l := range sc.Labels {
			if !gone[l] {
				keep = append(keep, l)
			}
		}
		return sc.ChangeLabels(keepAll(keep))
	})
}

// keepAll is the edits that leave values as they are.
func keepAll(values []string) []schema.ValueEdit {
	eds := make([]schema.ValueEdit, len(values))
	for i, v := range values {
		eds[i] = schema.ValueEdit{Name: v, From: []string{v}}
	}
	return eds
}

// editList is Edit for a change of the priorities or of the labels, which
// finds the values it removes once it is made.
func editList(ss *usecase.Session, target schema.Target, change func(*schema.Schema) (*schema.Schema, []schema.Migration, []string, error)) (EditWrite, error) {
	var removed []string
	return Edit(ss, func(sc *schema.Schema) (*schema.Schema, []schema.Migration, error) {
		next, ms, r, err := change(sc)
		removed = r
		return next, ms, err
	}, EditOpts{UseAfter: func() []Use {
		us := make([]Use, len(removed))
		for i, v := range removed {
			us[i] = Use{Target: target, Name: v}
		}
		return us
	}})
}
