package schema

import "github.com/kukv/hedgehug/internal/app/domain/model"

// Relation is one of the five fixed relations between tasks. Each is a
// built-in field of a task, a list of ULIDs, and has a direction: the task
// that holds it shows Label, the task it points at shows InverseLabel under
// the Inverse name. Only the holder stores it.
type Relation struct {
	Name, Label, Inverse, InverseLabel string
	Get                                func(*model.Task) []model.ULID
	Set                                func(*model.Task, []model.ULID)
}

// Relations are blocked_by, causes, relates, duplicates and clones, in the
// order shown.
var Relations = []Relation{
	{
		"blocked_by", "Blocked by", "blocks", "Blocks",
		func(t *model.Task) []model.ULID { return t.BlockedBy }, func(t *model.Task, v []model.ULID) { t.BlockedBy = v },
	},
	{
		"causes", "Causes", "caused_by", "Caused by",
		func(t *model.Task) []model.ULID { return t.Causes }, func(t *model.Task, v []model.ULID) { t.Causes = v },
	},
	{
		"relates", "Relates to", "related_by", "Related by",
		func(t *model.Task) []model.ULID { return t.Relates }, func(t *model.Task, v []model.ULID) { t.Relates = v },
	},
	{
		"duplicates", "Duplicates", "duplicated_by", "Duplicated by",
		func(t *model.Task) []model.ULID { return t.Duplicates }, func(t *model.Task, v []model.ULID) { t.Duplicates = v },
	},
	{
		"clones", "Clones", "cloned_by", "Cloned by",
		func(t *model.Task) []model.ULID { return t.Clones }, func(t *model.Task, v []model.ULID) { t.Clones = v },
	},
}

// RelationOf finds a relation by its name (inverse is false) or by the name
// of its other direction (inverse is true).
func RelationOf(name string) (r Relation, inverse bool, ok bool) {
	for _, r := range Relations {
		switch name {
		case r.Name:
			return r, false, true
		case r.Inverse:
			return r, true, true
		}
	}
	return Relation{}, false, false
}
