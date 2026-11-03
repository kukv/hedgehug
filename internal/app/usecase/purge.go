package usecase

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/plural"
)

// DefaultPurgeDays is how old hedgehug purge wants what it removes to be,
// when --older-than is not given.
const DefaultPurgeDays = 90

// maxPurgeDays keeps days * 24h inside a time.Duration (about 292 years).
const maxPurgeDays = 36500

// ParseOlderThan reads a period such as 90d: digits and d. 0d is all.
func ParseOlderThan(s string) (days int, err error) {
	digits, ok := strings.CutSuffix(s, "d")
	if ok && digits != "" && strings.Trim(digits, "0123456789") == "" {
		if days, err = strconv.Atoi(digits); err == nil && days <= maxPurgeDays {
			return days, nil
		}
	}
	return 0, newErr(KindUsage, "purge.bad_period",
		fmt.Errorf("%q is not a period; write it in days, such as 90d (0d is everything)", s), "value", s)
}

// PurgeTask is a deleted task purge removes.
type PurgeTask struct {
	ULID  model.ULID `json:"ulid"`
	ID    string     `json:"id"`
	Title string     `json:"title"`
}

// PurgeForward is a forward file purge removes.
type PurgeForward struct {
	ULID    model.ULID `json:"ulid"`
	MovedTo model.ULID `json:"moved_to"`
	MovedAt time.Time  `json:"moved_at"`
}

// PurgePlan is what purge removes from a store.
type PurgePlan struct {
	Store    string         `json:"store"`
	Tasks    []PurgeTask    `json:"tasks" nullable:"false"`
	Forwards []PurgeForward `json:"forwards" nullable:"false"`
}

// Empty reports whether there is nothing to purge.
func (p PurgePlan) Empty() bool { return len(p.Tasks) == 0 && len(p.Forwards) == 0 }

// keys are the ULIDs of the plan, sorted, to tell whether two plans are the
// same.
func (p PurgePlan) keys() []string {
	out := make([]string, 0, len(p.Tasks)+len(p.Forwards))
	for _, t := range p.Tasks {
		out = append(out, t.ULID.String())
	}
	for _, f := range p.Forwards {
		out = append(out, f.ULID.String())
	}
	slices.Sort(out)
	return out
}

// PurgeResult is what a purge did. Commit is empty when nothing was removed.
type PurgeResult struct {
	PurgePlan
	Commit string `json:"commit,omitempty"`
}

// purgePlan works out what to remove from the store name, whose index is ix:
// the tasks deleted at least days ago and the forwards that moved at least
// days ago. A deleted task is removed only when everything under it is
// deleted and old enough too. notDeleted names the deleted tasks that stay
// because a task under them is not deleted; broken counts the forwards that
// do not decode, which stay.
func (ss *Session) purgePlan(name string, ix *index.Index, days int) (plan PurgePlan, notDeleted []string, broken int) {
	cutoff := ss.Now().Add(-time.Duration(days) * 24 * time.Hour)
	plan = PurgePlan{Store: name, Tasks: []PurgeTask{}, Forwards: []PurgeForward{}}
	tasks := ix.Tasks()
	slices.SortFunc(tasks, func(a, b *index.Entry) int { return cmp.Compare(a.ULID.String(), b.ULID.String()) })
	old := make(map[model.ULID]bool, len(tasks))
	for _, e := range tasks {
		if e.DeletedAt != nil && !e.DeletedAt.After(cutoff) {
			old[e.ULID] = true
		}
	}
	for _, e := range tasks {
		if !old[e.ULID] {
			continue
		}
		under := Descendants(ix, e.ULID, every)
		if slices.ContainsFunc(under, func(d *index.Entry) bool { return d.DeletedAt == nil }) {
			notDeleted = append(notDeleted, cmp.Or(ix.DisplayID(e), e.ULID.String()))
			continue
		}
		if slices.ContainsFunc(under, func(d *index.Entry) bool { return !old[d.ULID] }) {
			continue // it goes once what is under it is old enough
		}
		plan.Tasks = append(plan.Tasks, PurgeTask{ULID: e.ULID, ID: ix.DisplayID(e), Title: e.Title})
	}
	for _, f := range ix.Forwards() {
		switch {
		case f.Broken != "":
			broken++
		case !f.MovedAt.After(cutoff):
			plan.Forwards = append(plan.Forwards, PurgeForward{ULID: f.ULID, MovedTo: f.MovedTo, MovedAt: f.MovedAt.UTC()})
		}
	}
	return plan, notDeleted, broken
}

// warnPurgeLeft tells of what purge leaves although it is old enough.
func (ss *Session) warnPurgeLeft(notDeleted []string, broken int) {
	if len(notDeleted) > 0 {
		ss.warn("kept (a task under each is not deleted): " + strings.Join(notDeleted, ", "))
	}
	if broken > 0 {
		ss.warn(plural.Count(broken, "broken forward file") + " left; see hedgehug doctor")
	}
}

// PurgeList lists what Purge would remove from the store (a store-unit
// command) and warns of what it leaves.
func (ss *Session) PurgeList(days int) (PurgePlan, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return PurgePlan{}, err
	}
	_, ix, err := ss.OpenForWrite()
	if err != nil {
		return PurgePlan{}, err
	}
	plan, notDeleted, broken := ss.purgePlan(e.Name, ix, days)
	ss.warnPurgeLeft(notDeleted, broken)
	return plan, nil
}

// Purge removes the deleted tasks (with their comments and work logs) and the
// forwards that are at least days old, in one commit (Hedgehug-Op: purge).
// Without yes it is confirm.required. None is no commit and no error. With
// yes it removes what there is under the lock.
func (ss *Session) Purge(days int, yes bool) (PurgeResult, error) {
	return ss.purge(days, nil, yes)
}

// PurgeConfirmed is Purge for what the caller showed to the user and the user
// agreed to (PurgeList lists it): it fails with purge.changed when what is to
// be removed under the lock is no longer that.
func (ss *Session) PurgeConfirmed(days int, confirmed PurgePlan) (PurgeResult, error) {
	if confirmed.Empty() {
		return PurgeResult{}, confirmRequired("purge")
	}
	return ss.purge(days, &confirmed, true)
}

// purge is Purge; a nil confirmed skips the comparison.
func (ss *Session) purge(days int, confirmed *PurgePlan, yes bool) (PurgeResult, error) {
	_, e, err := ss.Entry()
	if err != nil {
		return PurgeResult{}, err
	}
	s, ix, err := ss.OpenForWrite()
	if err != nil {
		return PurgeResult{}, err
	}
	plan, notDeleted, broken := ss.purgePlan(e.Name, ix, days)
	if plan.Empty() && confirmed == nil {
		return PurgeResult{PurgePlan: plan}, nil
	}
	if !yes {
		return PurgeResult{}, confirmRequired("purge")
	}
	op := store.Op{Name: "purge", Detail: fmt.Sprintf("older than %dd", days)}
	var lockedNotDeleted []string
	var lockedBroken int
	hash, err := ss.UpdateCommit(s, op, func(tx *store.Tx) error {
		// Read again under the lock: what came in by a sync since is removed
		// too when the removal was not confirmed one by one; if it was, a
		// change is refused (purge.changed).
		if err := ix.Refresh(); err != nil {
			return err
		}
		plan, lockedNotDeleted, lockedBroken = ss.purgePlan(e.Name, ix, days)
		if confirmed != nil && !slices.Equal(plan.keys(), confirmed.keys()) {
			return newErr(KindConflict, "purge.changed", errors.New("what is to be purged changed after the confirmation; run purge again"))
		}
		for _, t := range plan.Tasks {
			if err := tx.RemoveTask(t.ULID); err != nil {
				return err
			}
		}
		for _, f := range plan.Forwards {
			if err := tx.RemoveTask(f.ULID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return PurgeResult{}, err
	}
	// What the first look left was told of by the caller (PurgeList) or is
	// not asked about; what is left only since the lock is told of now.
	ss.warnPurgeLeft(
		slices.DeleteFunc(lockedNotDeleted, func(id string) bool { return slices.Contains(notDeleted, id) }),
		max(0, lockedBroken-broken))
	return PurgeResult{PurgePlan: plan, Commit: commitHex(hash)}, nil
}
