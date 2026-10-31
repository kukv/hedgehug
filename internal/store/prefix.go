package store

import (
	"slices"
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

// openStore opens a store for PrefixUses; tests count the calls.
var openStore = Open

// PrefixUses opens each of stores once and returns its workspaces whose id
// prefix is one of prefixes, ignoring case, in the order of stores. A store
// that does not open is given to skipped, when it is not nil, and left
// out; a workspace that does not read is left out too.
func PrefixUses(stores []NamedStore, prefixes []string, skipped func(name string, err error)) []PrefixUse {
	var out []PrefixUse
	for _, e := range stores {
		s, err := openStore(e.Path)
		if err != nil {
			if skipped != nil {
				skipped(e.Name, err)
			}
			continue
		}
		for w, err := range s.Workspaces() {
			if err == nil && slices.ContainsFunc(prefixes, func(p string) bool { return strings.EqualFold(w.IDPrefix, p) }) {
				out = append(out, PrefixUse{Store: e.Name, Workspace: w})
			}
		}
	}
	return out
}
