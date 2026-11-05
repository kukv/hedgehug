package usecase

import (
	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// taskChange is a status the rules gave a task.
type taskChange struct {
	ulid model.ULID
	automation.Change
}

// runRules evaluates the rules for what a fetch did to t, a task of ix. The
// refusals go on the commit of tx; it returns the status to write after it,
// or nil. The rules come after mirror: with a plan they judge t as it leaves
// it.
func runRules(tx *store.Tx, rules *automation.File, ix *index.Index, sc *schema.Schema, t *model.Task, a applied, plan *mirrorPlan) *taskChange {
	e, _ := ix.Task(t.ULID) // the fetch chose t from ix
	in := automation.Input{
		Events:    automation.EventsOf(a.Old, closedHere(t, rules, a.Next)),
		Task:      t,
		Ancestors: ix.Ancestors(e),
		Schema:    sc,
	}
	if plan != nil {
		in.Task, in.Status = plan.on(t), plan.status
	}
	o := rules.Evaluate(in)
	for _, r := range o.Rejected {
		tx.AddTrailer(automation.TrailerRejected, automation.RejectedTrailer(t.ULID, r))
	}
	if o.Change == nil {
		return nil
	}
	return &taskChange{t.ULID, *o.Change}
}

// applyChanges writes each status as a commit of its own, under the hold that
// the evaluation was made under, so that what the rules passed still stands.
// The transitions were checked rule by rule; the first status and the last
// need not be a transition themselves. A status that cannot be written is a
// warning: the fetch is done. A task in skip is left: the status the rules
// started from was not written.
func applyChanges(ss *Session, s *store.Store, held *store.Locked, changes []taskChange, skip map[model.ULID]bool, rep *GitHubFetchReport) {
	for _, c := range changes {
		if skip[c.ulid] {
			continue
		}
		_, err := ss.UpdateCommit(s, store.Op{Name: "set", Task: c.ulid, Automated: true, SkipTransitions: true, Locked: held}, func(tx *store.Tx) error {
			t, err := tx.Task(c.ulid)
			if err != nil {
				return err
			}
			t.Status = c.Status
			tx.AddTrailer(automation.TrailerApplied, automation.AppliedTrailer(c.Change))
			return tx.PutTask(t)
		})
		if err != nil {
			rep.Warnings = append(rep.Warnings, c.ulid.String()+": "+err.Error())
		}
	}
}
