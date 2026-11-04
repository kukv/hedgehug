package usecase

import (
	"fmt"
)

// CrossRefs finds the references of the store name to another mounted store.
func (ss *Session) CrossRefs(name string) ([]CrossRef, error) {
	set, err := ss.Mount()
	if err != nil {
		return nil, err
	}
	return CrossRefs(set, name)
}

// CrossRefMsg says what r is: "parent points at org-b's WEB-4; references
// stay inside a store".
func CrossRefMsg(r CrossRef) string {
	return fmt.Sprintf("%s points at %s's %s; references stay inside a store", r.Field, r.Target.Store.Name, r.Target.RawID())
}

// CrossStoreWarnings are the references of the store name to another
// mounted store, as the warnings of a sync: "WEB-1 parent points at
// org-b's WEB-4; references stay inside a store". The store is not named:
// the sync names it where it shows a warning (and --json has it as store).
func (ss *Session) CrossStoreWarnings(name string) []string {
	refs, err := ss.CrossRefs(name)
	if err != nil {
		return []string{"could not check cross-store references: " + err.Error()}
	}
	var out []string
	for _, r := range refs {
		out = append(out, fmt.Sprintf("%s %s", r.From.RawID(), CrossRefMsg(r)))
	}
	return out
}
