package planning

import "github.com/kukv/hedgehug/internal/domain/change"

type field struct {
	name     string
	from, to any
}

// drafts は、fields から set の行を作る。fresh なら作成で、すべての項目を from: null で書く。ほかは変わった項目だけ。
func drafts(prefix string, id change.ID, fresh bool, fields []field) []change.Draft {
	var ds []change.Draft
	for _, f := range fields {
		name := prefix + string(id) + "/" + f.name
		switch {
		case fresh:
			ds = append(ds, change.NewSet(name, nil, f.to))
		case f.from != f.to:
			ds = append(ds, change.NewSet(name, f.from, f.to))
		}
	}
	return ds
}

// SprintDrafts は、old から s への行。old.ID が空なら作成で、すべての項目を書く。
func SprintDrafts(old, s Sprint) []change.Draft {
	return drafts(sprintPrefix, s.ID, old.ID == "", []field{
		{"name", old.Name, s.Name},
		{"state", old.State, s.State},
		{"start", old.Start, s.Start},
		{"end", old.End, s.End},
		{"goal", old.Goal, s.Goal},
	})
}

// ReleaseDrafts は、old から r への行。old.ID が空なら作成で、すべての項目を書く。
func ReleaseDrafts(old, r Release) []change.Draft {
	return drafts(releasePrefix, r.ID, old.ID == "", []field{
		{"name", old.Name, r.Name},
		{"state", old.State, r.State},
		{"date", old.Date, r.Date},
		{"description", old.Description, r.Description},
	})
}

// DeleteSprint は、スプリント id の削除の行。
func DeleteSprint(id change.ID) change.Draft { return change.NewDelete(sprintPrefix + string(id)) }

// DeleteRelease は、リリース id の削除の行。
func DeleteRelease(id change.ID) change.Draft { return change.NewDelete(releasePrefix + string(id)) }

// CloseDraft は、スプリント id の完了の確定値の行(F1-3)。
func CloseDraft(id change.ID, c Confirmed) change.Draft {
	return change.Draft{T: CloseSprint, F: sprintPrefix + string(id) + "/close", To: change.Value(c)}
}
