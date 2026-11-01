package ops

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Count is how many of a whole are done.
type Count struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// ChecklistOf is the checklist of the body of e, or nil when it has none.
func ChecklistOf(e *index.Entry) *Count {
	if e.ChecklistTotal == 0 {
		return nil
	}
	return &Count{Done: e.ChecklistDone, Total: e.ChecklistTotal}
}

// ProgressOf is the progress of a project: of the tasks of the work role
// under e that are not deleted, those in the done category over those not
// in the canceled one. It is nil unless e is of the container role.
func ProgressOf(ix *index.Index, e *index.Entry) *Count {
	sc := ix.Schema()
	if k, ok := sc.Kind(e.Kind); !ok || k.Role != schema.RoleContainer {
		return nil
	}
	c := &Count{}
	seen := map[model.ULID]bool{e.ULID: true}
	var walk func(u model.ULID)
	walk = func(u model.ULID) {
		for _, ch := range ix.Children(u) {
			if seen[ch.ULID] {
				continue
			}
			seen[ch.ULID] = true
			walk(ch.ULID)
			if ch.DeletedAt != nil {
				continue
			}
			if k, ok := sc.Kind(ch.Kind); !ok || k.Role != schema.RoleWork {
				continue
			}
			st, _ := sc.Status(ch.Status)
			switch st.Category {
			case schema.CategoryCanceled:
			case schema.CategoryDone:
				c.Done++
				c.Total++
			default:
				c.Total++
			}
		}
	}
	walk(e.ULID)
	return c
}

// ProgressText is the checklist and the progress as the CLI writes them:
// "☑ 2/5  3/14", either alone, or "" with neither.
func ProgressText(checklist, progress *Count) string {
	var parts []string
	if checklist != nil {
		parts = append(parts, fmt.Sprintf("☑ %d/%d", checklist.Done, checklist.Total))
	}
	if progress != nil {
		parts = append(parts, fmt.Sprintf("%d/%d", progress.Done, progress.Total))
	}
	return strings.Join(parts, "  ")
}
