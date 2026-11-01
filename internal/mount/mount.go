// Package mount is the set of stores that config.yaml lists, opened together,
// and the finding of a task by its ID across them.
package mount

import (
	"strings"
	gosync "sync"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// Mounted is a store of stores[] that opened.
type Mounted struct {
	Name   string // as config.yaml names it
	Entry  config.StoreEntry
	Store  *store.Store
	ULID   model.ULID  // store.yaml's ulid
	Access sync.Access // read-only state; the zero value when access.json is missing or does not read

	// the index, opened by Index on first use and kept
	mu gosync.Mutex
	ix *index.Index
}

// Index opens the index of the store on first use and keeps it. The index
// knows the store's name (index.SetStoreName). An error is not kept, so the
// next call opens again.
func (m *Mounted) Index() (*index.Index, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ix != nil {
		return m.ix, nil
	}
	ix, err := index.Open(m.Store)
	if err != nil {
		return nil, err
	}
	ix.SetStoreName(m.Name)
	m.ix = ix
	return ix, nil
}

// Skipped is a store of stores[] that did not open.
type Skipped struct {
	Name string
	Err  error
}

// Set is the stores of config.yaml, in its order.
type Set struct {
	Stores  []*Mounted
	Skipped []Skipped

	warn func(string) // Options.Warn
}

// Options is how Open opens a store.
type Options struct {
	Now func() time.Time // given to store.SetNow; nil keeps the store's clock
	// Warn gets the warning of a store whose index does not open while Resolve
	// looks in every store; nil drops it.
	Warn func(msg string)
}

// Open opens every store of c.Stores in order. A store that does not open
// goes to Skipped with why; Open itself fails only for what is not about
// one store.
func Open(c *config.Config, opt Options) (*Set, error) {
	set := &Set{warn: opt.Warn}
	for _, e := range c.Stores {
		s, err := store.Open(e.Path)
		if err != nil {
			set.Skipped = append(set.Skipped, Skipped{Name: e.Name, Err: err})
			continue
		}
		if opt.Now != nil {
			s.SetNow(opt.Now)
		}
		// A missing access.json is the zero value; one that does not read
		// is too, and checkWritable of ops reads it again for a write.
		acc, err := sync.ReadAccess(s)
		if err != nil {
			acc = sync.Access{}
		}
		set.Stores = append(set.Stores, &Mounted{Name: e.Name, Entry: e, Store: s, ULID: s.Meta().ULID, Access: acc})
	}
	return set, nil
}

// Get is the mounted store named name (case counts).
func (s *Set) Get(name string) (*Mounted, bool) {
	for _, m := range s.Stores {
		if m.Name == name {
			return m, true
		}
	}
	return nil, false
}

// SplitStore reads "<store>:<rest>": when the part before the first ":" is
// the name of a mounted store, it returns that store and rest. Otherwise
// ok is false and the input is used whole.
func (s *Set) SplitStore(arg string) (m *Mounted, rest string, ok bool) {
	name, rest, found := strings.Cut(arg, ":")
	if !found {
		return nil, "", false
	}
	m, ok = s.Get(name)
	if !ok {
		return nil, "", false
	}
	return m, rest, true
}
