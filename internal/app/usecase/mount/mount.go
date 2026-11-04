// Package mount is the set of stores that config.yaml lists, opened together,
// and the finding of a task by its ID across them.
package mount

import (
	"io/fs"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
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

// CloseRepo lets go of the repository files the store holds open, so that
// its directory can be renamed (Windows). The store is not to be used after;
// the set is dropped and opened again.
func (m *Mounted) CloseRepo() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Store.Repo().Close()
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
		_ = s.Repo().Close()
		return nil, err
	}
	ix.SetStoreName(m.Name)
	ix.SetQualified(m.qualify)
	m.mu.Lock()
	old := m.Store
	m.Store, m.ix = s, ix
	m.mu.Unlock()
	// the old handle keeps the packs it read open (KeepDescriptors), among
	// them those another handle removed
	_ = old.Repo().Close()
	return ix, nil
}

// Close lets go of the repository files of every store of the set (CloseRepo).
// The set may be used after, as a store opens its files again when it reads.
func (s *Set) Close() {
	for _, m := range s.Stores {
		_ = m.CloseRepo()
	}
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
	// Unfinished are the stores that opened but have a rebuild after a
	// history rewrite left beside them (store.LeftoverError). A store that
	// did not open for want of its directory is in Skipped with the same
	// error.
	Unfinished []Skipped

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
	parents := map[string][]fs.DirEntry{} // read once for the stores in one directory
	perr := map[string]error{}
	for _, e := range c.Stores {
		s, err := store.Open(e.Path)
		parent := filepath.Dir(filepath.Clean(e.Path))
		if _, ok := parents[parent]; !ok {
			parents[parent], perr[parent] = store.ReadParent(e.Path)
		}
		unfinished := perr[parent] == nil && !store.LeftoverIn(e.Path, parents[parent]).None()
		if err != nil {
			if unfinished {
				err = &store.LeftoverError{Name: e.Name}
			}
			set.Skipped = append(set.Skipped, Skipped{Name: e.Name, Err: err})
			continue
		}
		if unfinished {
			set.Unfinished = append(set.Unfinished, Skipped{Name: e.Name, Err: &store.LeftoverError{Name: e.Name}})
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
		_ = s.Repo().Close()
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
