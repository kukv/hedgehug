package usecase

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

// dupPlace is where a duplicated task lives: a mounted store and its id there.
type dupPlace struct{ Store, ID string }

// Duplicate is a ULID that live tasks of two or more mounted stores have.
type Duplicate struct {
	ULID   model.ULID
	Places []dupPlace // in the order of the stores
}

// Duplicates finds the ULIDs of live tasks that are in more than one of the
// mounted stores, in ULID order. A store whose index cannot be read is left
// out.
func (ss *Session) Duplicates() ([]Duplicate, error) {
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
			ss.WarnOnce("duplicates:"+m.Name, fmt.Sprintf("could not read the index of %s, so its duplicates are not checked: %v", m.Name, err))
		}
	}
	var out []Duplicate
	for u, ps := range seen {
		if len(ps) > 1 {
			out = append(out, Duplicate{u, ps})
		}
	}
	slices.SortFunc(out, func(a, b Duplicate) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	return out, nil
}

// Text is "in org-b (WEB-45) and org-c (OPS-3); remove the one not needed with
// hedgehug rm and hedgehug purge --older-than 0d --store <store>" (D19).
func (d Duplicate) Text() string {
	parts := d.PlaceLabels()
	list := parts[0]
	if n := len(parts); n > 1 {
		list = strings.Join(parts[:n-1], ", ") + " and " + parts[n-1]
	}
	return "in " + list + "; remove the one not needed with hedgehug rm and hedgehug purge --older-than 0d --store <store>"
}

// PlaceLabels are the places of d as Text names them: "<store> (<id>)".
func (d Duplicate) PlaceLabels() []string {
	parts := make([]string, len(d.Places))
	for i, p := range d.Places {
		parts[i] = fmt.Sprintf("%s (%s)", p.Store, p.ID)
	}
	return parts
}

// In tells whether the store name has the task.
func (d Duplicate) In(name string) bool {
	return slices.ContainsFunc(d.Places, func(p dupPlace) bool { return p.Store == name })
}

// AddDuplicateWarnings puts the warning of each duplicate, "01JBZ… is in org-b
// (WEB-45) and org-c (OPS-3); …", in the report of the first store synced
// that has the task: once for the duplicate, not once for each store it is
// in. The duplicates are found once for all the reports. A store that has no
// remote, or whose sync failed, is not told of here.
func (ss *Session) AddDuplicateWarnings(reports []SyncReport) {
	var told []int
	for i, r := range reports {
		if !r.NoRemote && r.Err == nil {
			told = append(told, i)
		}
	}
	if len(told) == 0 {
		return
	}
	ds, err := ss.Duplicates()
	if err != nil {
		r := &reports[told[0]]
		r.Warnings = append(r.Warnings, "could not check duplicates across stores: "+err.Error())
		return
	}
	for _, d := range ds {
		for _, i := range told {
			if d.In(reports[i].Store) {
				reports[i].Warnings = append(reports[i].Warnings, fmt.Sprintf("%s is %s", d.ULID, d.Text()))
				break
			}
		}
	}
}
