package mount

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/query"
)

// QueryOptions gives the options of the query of one store (its inbox
// tells is:overwritten).
type QueryOptions func(m *Mounted) (query.Options, error)

// Entry is a task found, with its store.
type Entry struct {
	Store *Mounted
	Entry *index.Entry
}

// Group is a group of the result: the tasks of every store with the
// value Key.
type Group struct {
	Key     string
	Entries []Entry
}

// Result is what Query found.
type Result struct {
	Stores  []*Mounted // the stores the query ran on, in config order
	Entries []Entry    // in the order of the groups
	Groups  []Group
}

// Query runs q on each store store: and only (a name) keep, as spec's
// "Store をまたぐ評価" says, and joins what they found.
//   - store:a,b keeps those; -store:a drops it; a name that is not mounted
//     is a *query.Error ("unknown store \"x\"").
//   - a *query.Error with Unknown (a property the store's schema lacks) in
//     one store makes it find nothing, unless every store has it, which is
//     that error.
//   - a value naming a task (project:API-1) that a store does not have matches
//     no task there (so -project:API-1 matches all of them); one written
//     "<store>:<id>" is looked up in that store only. A value no store has is
//     the error of the query.
//   - no sort:: the stores in order, each in its order. sort:: CompareSortKeys
//     across all, ties by store order then the order within the store.
//   - group:: one group per Key across stores; the groups in the order they
//     first come, store by store, the group of no value last.
//
// With only keep, only those stores are opened. Each store is read under
// Mounted.With; what Query returns belongs to the index of the store, so a
// caller that shares the Set with others reads it only while nothing else
// refreshes that index.
func Query(set *Set, q *query.Query, only string, opt QueryOptions) (*Result, error) {
	keep, rest, err := storeFilter(set, q, only)
	if err != nil {
		return nil, err
	}
	res := &Result{Stores: keep}
	var groups []*keyedGroup
	byKey := map[string]*keyedGroup{}
	ran := 0
	var desc, sorted bool
	var unknown error
	// the values naming a task, and the error of one no store has
	type idKey struct {
		pos   int
		value string
	}
	lookups := map[idKey]query.IDLookup{} // the first, or one that found the task
	var idOrder []idKey
	for _, m := range keep {
		var rr *query.Result
		var keys func(*index.Entry) query.SortKey
		var qerr error
		var found []*keyedGroup
		werr := m.With(func(ix *index.Index) {
			o, err := opt(m)
			if err != nil {
				qerr = err
				return
			}
			o.MissingIDs = true
			if rr, qerr = query.Run(ix, rest, o); qerr != nil {
				return
			}
			keys = rr.SortKey
			for _, g := range rr.Groups {
				kg := &keyedGroup{key: g.Key}
				for _, e := range g.Tasks {
					k := keyedEntry{Entry: Entry{Store: m, Entry: e}}
					if keys != nil {
						k.key = keys(e)
					}
					kg.entries = append(kg.entries, k)
				}
				found = append(found, kg)
			}
		})
		if werr != nil {
			return nil, werr
		}
		var qe *query.Error
		if errors.As(qerr, &qe) && qe.Unknown {
			if unknown == nil {
				unknown = qerr
			}
			continue
		}
		if qerr != nil {
			return nil, qerr
		}
		ran++
		for _, l := range rr.IDs {
			k := idKey{l.Pos, l.Value}
			if _, seen := lookups[k]; !seen {
				idOrder = append(idOrder, k)
				lookups[k] = l
			}
			if l.Err == nil {
				lookups[k] = l
			}
		}
		desc = rr.Desc
		for _, g := range found {
			into, ok := byKey[g.key]
			if !ok {
				byKey[g.key] = g
				groups = append(groups, g)
				continue
			}
			into.entries = append(into.entries, g.entries...)
		}
		if keys != nil {
			sorted = true
		}
	}
	if ran == 0 && unknown != nil {
		return nil, unknown
	}
	for _, k := range idOrder {
		if l := lookups[k]; l.Err != nil {
			return nil, &query.Error{Pos: l.Pos, Term: l.Term, Msg: l.Err.Error(), Err: l.Err}
		}
	}
	// the tasks with no value come last, as they do within a store
	if i := slices.IndexFunc(groups, func(g *keyedGroup) bool { return g.key == "" }); i >= 0 {
		groups = append(groups[:i:i], append(groups[i+1:], groups[i])...)
	}
	for _, g := range groups {
		// One store keeps the order query.Run gave; the stores together
		// are ordered by the keys, ties staying in store order.
		if sorted && ran > 1 {
			slices.SortStableFunc(g.entries, func(a, b keyedEntry) int {
				return query.CompareSortKeys(a.key, b.key, desc)
			})
		}
		out := Group{Key: g.key}
		for _, e := range g.entries {
			out.Entries = append(out.Entries, e.Entry)
		}
		res.Groups = append(res.Groups, out)
		res.Entries = append(res.Entries, out.Entries...)
	}
	return res, nil
}

type keyedEntry struct {
	Entry
	key query.SortKey
}

type keyedGroup struct {
	key     string
	entries []keyedEntry
}

// storeFilter reads the store: terms of q and gives the stores to run on, in
// config order, and q without them.
func storeFilter(set *Set, q *query.Query, only string) ([]*Mounted, *query.Query, error) {
	rest := &query.Query{}
	keep := slices.Clone(set.Stores)
	if only != "" {
		m, ok := set.Get(only)
		if !ok {
			return nil, nil, fmt.Errorf("unknown store %q", only)
		}
		keep = []*Mounted{m}
	}
	for _, t := range q.Terms {
		if t.Key != "store" {
			rest.Terms = append(rest.Terms, t)
			continue
		}
		if t.Op != ":" {
			return nil, nil, &query.Error{Pos: t.Pos, Term: t.Raw, Msg: fmt.Sprintf("store cannot be compared with %s", t.Op)}
		}
		for _, v := range t.Values {
			if _, ok := set.Get(v); !ok {
				return nil, nil, &query.Error{Pos: t.Pos, Term: t.Raw, Msg: fmt.Sprintf("unknown store %q", v)}
			}
		}
		keep = slices.DeleteFunc(keep, func(m *Mounted) bool {
			return slices.Contains(t.Values, m.Name) == t.Neg
		})
	}
	return keep, rest, nil
}
