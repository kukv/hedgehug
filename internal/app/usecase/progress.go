package usecase

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
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

// ProgressOf is the progress of a project or an epic: of the stories, tasks
// and bugs under e that are not deleted, those in the done category over
// those not in the canceled one. Steps are not counted. It is nil unless e
// is a group.
func ProgressOf(ix *index.Index, e *index.Entry) *Count {
	if !schema.IsGroup(e.Kind) {
		return nil
	}
	sc := ix.Schema()
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
			if ch.DeletedAt == nil && schema.CountsForProgress(ch.Kind) {
				c.add(sc, ch)
			}
		}
	}
	walk(e.ULID)
	return c
}

// StepsOf is the breakdown of the steps of a story, a task or a bug: of its
// steps that are not deleted, those in the done category over those not in
// the canceled one. It is nil unless e is of one of those kinds.
func StepsOf(ix *index.Index, e *index.Entry) *Count {
	if !schema.CountsForProgress(e.Kind) {
		return nil
	}
	sc := ix.Schema()
	c := &Count{}
	for _, ch := range ix.Children(e.ULID) {
		if ch.DeletedAt == nil && ch.Kind == "step" {
			c.add(sc, ch)
		}
	}
	return c
}

// add counts the task ch: it is left out when canceled.
func (c *Count) add(sc *schema.Schema, ch *index.Entry) {
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

// ChecklistText is the checklist as the CLI writes it, "☑ 2/5", or "" with
// none.
func ChecklistText(c *Count) string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("☑ %d/%d", c.Done, c.Total)
}

// StepsText is the steps of a story, a task or a bug as the CLI writes them,
// "2/3", or "" with none (a task with no step shows nothing, not "0/0").
func StepsText(c *Count) string {
	if c == nil || c.Total == 0 {
		return ""
	}
	return ProgressFractionText(c)
}

// ProgressFractionText is the progress as the CLI writes it, "3/14", or ""
// with none.
func ProgressFractionText(c *Count) string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%d/%d", c.Done, c.Total)
}

// ProgressText is the checklist, the progress of a group and the steps of a
// story, a task or a bug as one line of the tree writes them: "☑ 2/5  3/14"
// (the progress and the steps are never both there), each alone, or "" with
// none.
func ProgressText(checklist, progress, steps *Count) string {
	var parts []string
	for _, p := range []string{ChecklistText(checklist), ProgressFractionText(progress), StepsText(steps)} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "  ")
}
