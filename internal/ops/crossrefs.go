package ops

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/mount"
)

// crossRefs finds the references of the store name to another mounted store.
func (ss *Session) crossRefs(name string) ([]mount.CrossRef, error) {
	set, err := ss.Mount()
	if err != nil {
		return nil, err
	}
	return mount.CrossRefs(set, name)
}

// crossRefMsg says what r is: "parent points at org-b's WEB-4; references
// stay inside a store".
func crossRefMsg(r mount.CrossRef) string {
	return fmt.Sprintf("%s points at %s's %s; references stay inside a store", r.Field, r.Target.Store.Name, r.Target.RawID())
}

// crossStoreFindings are the references of the store name to another mounted
// store, as warnings of doctor. A store whose references cannot be read is a
// warning of the session, not a finding: the other checks stand without it.
func (ss *Session) crossStoreFindings(name string) []doctor.Finding {
	refs, err := ss.crossRefs(name)
	if err != nil {
		ss.warn("could not check cross-store references: " + err.Error())
		return nil
	}
	var out []doctor.Finding
	for _, r := range refs {
		// the same words as the Where of a check of the store's own tasks
		where := taskLabel(r.From.ID, r.From.ID, r.From.ULID) + " " + r.From.Title
		out = append(out, doctor.Finding{Check: "cross_store_ref", Where: where, Msg: crossRefMsg(r), Severity: doctor.Warning})
	}
	doctor.Sort(out)
	return out
}

// crossStoreWarnings are the references of the store name to another
// mounted store, as the warnings of a sync: "WEB-1 parent points at
// org-b's WEB-4; references stay inside a store". The store is not named:
// the sync names it where it shows a warning (and --json has it as store).
func (ss *Session) crossStoreWarnings(name string) []string {
	refs, err := ss.crossRefs(name)
	if err != nil {
		return []string{"could not check cross-store references: " + err.Error()}
	}
	var out []string
	for _, r := range refs {
		out = append(out, fmt.Sprintf("%s %s", r.From.RawID(), crossRefMsg(r)))
	}
	return out
}
