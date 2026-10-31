package store

import (
	"strings"

	"github.com/kukv/hedgehug/internal/model"
)

// NamedStore is a store as config names it.
type NamedStore struct{ Name, Path string }

// PrefixUse is a workspace of another store that has an id prefix.
type PrefixUse struct {
	Store     string // the name of the store
	Workspace *model.Workspace
}

// PrefixUses opens each of stores and returns its workspaces whose id prefix
// is prefix, ignoring case, in the order of stores. A store that does not
// open is given to skipped, when it is not nil, and left out; a workspace
// that does not read is left out too.
func PrefixUses(stores []NamedStore, prefix string, skipped func(name string, err error)) []PrefixUse {
	var out []PrefixUse
	for _, e := range stores {
		s, err := Open(e.Path)
		if err != nil {
			if skipped != nil {
				skipped(e.Name, err)
			}
			continue
		}
		for w, err := range s.Workspaces() {
			if err == nil && strings.EqualFold(w.IDPrefix, prefix) {
				out = append(out, PrefixUse{Store: e.Name, Workspace: w})
			}
		}
	}
	return out
}
