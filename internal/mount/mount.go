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
)

// Mounted is a store of stores[] that opened.
type Mounted struct {
	Name  string // as config.yaml names it
	Entry config.StoreEntry
	Store *store.Store
	ULID  model.ULID // store.yaml's ulid

	// the index, opened by Index on first use and kept
	mu gosync.Mutex
	ix *index.Index
	// use serializes With: an index is not safe for concurrent use. It also
	// guards Store and ix when With opens them again.
	use gosync.Mutex
	now func() time.Time // Options.Now, for the store opened again
	// qualify is the Set's shared prefixes, given to the index as it opens
	qualify func(prefix string) bool
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
	ix.SetQualified(m.qualify)
	m.ix = ix
	return ix, nil
}

// With runs fn on the index of the store, brought up to HEAD, while no other
// With uses it. The index (and m.Store) is not to be kept past fn. When the
// index cannot be brought up to HEAD, as after another handle repacked the
// store, the store and its index are opened again once. An error then is
// returned and fn is not run.
func (m *Mounted) With(fn func(ix *index.Index)) error {
	m.use.Lock()
	defer m.use.Unlock()
	ix, err := m.Index()
	if err == nil {
		if err = ix.Refresh(); err == nil {
			fn(ix)
			return nil
		}
	}
	if ix, err = m.reopen(); err != nil {
		return err
	}
	fn(ix)
	return nil
}

// reopen opens the store and its index again; With holds m.use.
func (m *Mounted) reopen() (*index.Index, error) {
	s, err := store.Open(m.Entry.Path)
	if err != nil {
		return nil, err
	}
	if m.now != nil {
		s.SetNow(m.now)
	}
	ix, err := index.Open(s)
	if err != nil {
		return nil, err
	}
	ix.SetStoreName(m.Name)
	ix.SetQualified(m.qualify)
	m.mu.Lock()
	m.Store, m.ix = s, ix
	m.mu.Unlock()
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

	unopened func(Skipped) // Options.Unopened
}

// Options is how Open opens a store.
type Options struct {
	Now func() time.Time // given to store.SetNow; nil keeps the store's clock
	// Unopened is told of a store whose index does not open while Resolve
	// looks in every store, each time it happens; nil drops it. The caller
	// decides how often to say it.
	Unopened func(Skipped)
	// Qualified tells which id prefixes show with the store's name. nil has
	// Open take SharedPrefixes of the config; a caller that opens indexes of
	// its own as well gives the same function to both.
	Qualified func(prefix string) bool
}

// Open opens every store of c.Stores in order. A store that does not open
// goes to Skipped with why.
func Open(c *config.Config, opt Options) *Set {
	set := &Set{unopened: opt.Unopened}
	for _, e := range c.Stores {
		s, err := store.Open(e.Path)
		if err != nil {
			set.Skipped = append(set.Skipped, Skipped{Name: e.Name, Err: err})
			continue
		}
		if opt.Now != nil {
			s.SetNow(opt.Now)
		}
		set.Stores = append(set.Stores, &Mounted{Name: e.Name, Entry: e, Store: s, ULID: s.Meta().ULID, now: opt.Now})
	}
	shared := opt.Qualified
	if shared == nil {
		shared = SharedPrefixes(c)
	}
	for _, m := range set.Stores {
		m.qualify = shared
	}
	return set
}

// SharedPrefixes tells which id prefixes two or more of the stores of c use
// (index.SetQualified takes it), case aside, reading the workspaces of each
// store without opening any index and keeping none. A store that does not
// open has no prefix to count.
func SharedPrefixes(c *config.Config) func(prefix string) bool {
	seen := map[string]int{} // upper-cased prefix: number of stores
	for _, e := range c.Stores {
		s, err := store.Open(e.Path)
		if err != nil {
			continue
		}
		mine := map[string]bool{}
		for w, err := range s.Workspaces() {
			if err != nil {
				continue // the store's own checks tell of it
			}
			mine[strings.ToUpper(w.IDPrefix)] = true
		}
		for p := range mine {
			seen[p]++
		}
	}
	shared := map[string]bool{}
	for p, n := range seen {
		if n >= 2 {
			shared[p] = true
		}
	}
	return func(prefix string) bool { return shared[strings.ToUpper(prefix)] }
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
