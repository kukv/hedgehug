package ops

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/mount"
)

// dupPlace is where a duplicated task lives: a mounted store and its id there.
type dupPlace struct{ Store, ID string }

// duplicate is a ULID that live tasks of two or more mounted stores have.
type duplicate struct {
	ULID   model.ULID
	Places []dupPlace // in the order of the stores
}

// duplicates finds the ULIDs of live tasks that are in more than one of the
// mounted stores, in ULID order. A store whose index cannot be read is left
// out.
func (ss *Session) duplicates() ([]duplicate, error) {
	set, err := ss.Mount()
	if err != nil {
		return nil, err
	}
	seen := map[model.ULID][]dupPlace{}
	for _, m := range set.Stores {
		err := m.With(func(ix *index.Index) {
			for _, e := range ix.Tasks() {
				if e.DeletedAt == nil {
					seen[e.ULID] = append(seen[e.ULID], dupPlace{m.Name, e.ID})
				}
			}
		})
		if err != nil {
			ss.warnOnce(m.Name, fmt.Sprintf("could not read the index of %s, so its duplicates are not checked: %v", m.Name, err))
		}
	}
	var out []duplicate
	for u, ps := range seen {
		if len(ps) > 1 {
			out = append(out, duplicate{u, ps})
		}
	}
	slices.SortFunc(out, func(a, b duplicate) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	return out, nil
}

// text is "in org-b (WEB-45) and org-c (OPS-3); remove the one not needed with
// hedgehug rm and hedgehug purge" (D19).
func (d duplicate) text() string {
	parts := make([]string, len(d.Places))
	for i, p := range d.Places {
		parts[i] = fmt.Sprintf("%s (%s)", p.Store, p.ID)
	}
	list := parts[0]
	if n := len(parts); n > 1 {
		list = strings.Join(parts[:n-1], ", ") + " and " + parts[n-1]
	}
	return "in " + list + "; remove the one not needed with hedgehug rm and hedgehug purge"
}

// in tells whether the store name has the task.
func (d duplicate) in(name string) bool {
	return slices.ContainsFunc(d.Places, func(p dupPlace) bool { return p.Store == name })
}

// duplicateFindings are the tasks of the store name that live in another
// mounted store too, as warnings of doctor. ds is what duplicates found.
func duplicateFindings(name string, ds []duplicate) []doctor.Finding {
	var out []doctor.Finding
	for _, d := range ds {
		if d.in(name) {
			out = append(out, doctor.Finding{Check: "duplicate_across_stores", Where: d.ULID.String(), Msg: d.text(), Severity: doctor.Warning})
		}
	}
	return out
}

// duplicateWarnings are the same as the warnings of a sync of the store name:
// "01JBZ… is in org-b (WEB-45) and org-c (OPS-3); …".
func (ss *Session) duplicateWarnings(name string) []string {
	ds, err := ss.duplicates()
	if err != nil {
		return []string{"could not check duplicates across stores: " + err.Error()}
	}
	var out []string
	for _, d := range ds {
		if d.in(name) {
			out = append(out, fmt.Sprintf("%s is %s", d.ULID, d.text()))
		}
	}
	return out
}

// forwardTargetFindings are the forwards of the store name whose moved_to is
// no mounted store, as infos of doctor: the store may just not be mounted.
func (ss *Session) forwardTargetFindings(name string) []doctor.Finding {
	set, err := ss.Mount()
	if err != nil {
		ss.warn("could not check the forwards: " + err.Error())
		return nil
	}
	m, ok := set.Get(name)
	if !ok {
		return nil
	}
	var out []doctor.Finding
	err = m.With(func(ix *index.Index) {
		for _, f := range ix.Forwards() {
			if f.Broken != "" || slices.ContainsFunc(set.Stores, func(o *mount.Mounted) bool { return o.ULID == f.MovedTo }) {
				continue
			}
			where := f.ULID.String()
			if f.ID != "" {
				where = fmt.Sprintf("%s (%s)", f.ID, f.ULID)
			}
			out = append(out, doctor.Finding{Check: "forward_target", Where: where, Msg: fmt.Sprintf("moved to a store that is not mounted (ULID %s)", f.MovedTo), Severity: doctor.Info})
		}
	})
	if err != nil {
		ss.warnOnce(name, fmt.Sprintf("could not read the index of %s, so its forwards are not checked: %v", name, err))
	}
	doctor.Sort(out)
	return out
}
