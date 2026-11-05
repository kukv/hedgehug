package automation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// Fresh are the problems of after that before does not have. The messages are
// compared as a multiset: the paths hold indexes, which an edit in front of a
// problem shifts.
func Fresh(before, after []schema.Problem) []schema.Problem {
	known := make([]string, len(before))
	for i, p := range before {
		known[i] = p.Msg
	}
	var fresh []schema.Problem
	for _, p := range after {
		if i := slices.Index(known, p.Msg); i >= 0 {
			known = slices.Delete(known, i, i+1)
		} else {
			fresh = append(fresh, p)
		}
	}
	return fresh
}

// Check lists the defects of f against sc. project says whether a ULID is a
// project of the store that is not deleted.
func (f *File) Check(sc *schema.Schema, project func(model.ULID) bool) []schema.Problem {
	var ps []schema.Problem
	add := func(path, format string, args ...any) {
		ps = append(ps, schema.Problem{Path: path, Msg: fmt.Sprintf(format, args...)})
	}
	// where reads a where; the ULID is zero and ok false when it has a defect,
	// which is then reported.
	where := func(path, w string) (model.ULID, bool) {
		s, found := strings.CutPrefix(w, WherePrefix)
		if !found {
			add(path, `where must be "project:<ULID>"`)
			return model.ULID{}, false
		}
		u, err := model.ParseULID(s)
		if err != nil {
			add(path, "%v", err)
			return model.ULID{}, false
		}
		if !project(u) {
			add(path, "%s is not a project of this store, or is deleted", w)
			return model.ULID{}, false
		}
		return u, true
	}

	ids := map[string]bool{}
	for i, r := range f.Rules {
		at := fmt.Sprintf("rules[%d]", i)
		switch {
		case r.ID == "":
			add(at+".id", "id is empty")
		case strings.ContainsAny(r.ID, " \t\r\n"):
			add(at+".id", "id %q has a space or a line break, which the trailer of a commit cannot hold", r.ID)
		case ids[r.ID]:
			add(at+".id", "id %q is used twice", r.ID)
		}
		ids[r.ID] = true
		if !slices.Contains(Events, r.On) {
			add(at+".on", "%q is not one of %s", r.On, strings.Join(Events, ", "))
		}
		if r.When != "" && r.When != AllLinkedPRsMerged {
			add(at+".when", "%q is not %s", r.When, AllLinkedPRsMerged)
		}
		scope := sc.For(nil)
		if r.Where != "" {
			if u, ok := where(at+".where", r.Where); ok {
				scope = sc.For([]model.ULID{u})
			}
		}
		switch {
		case r.Set.Status != "" && r.Set.Category != "":
			add(at+".set", "set takes status or category, not both")
		case r.Set.Status == "" && r.Set.Category == "":
			add(at+".set", "set needs a status or a category")
		case r.Set.Status != "":
			if !slices.ContainsFunc(scope.Statuses(), func(s schema.Status) bool { return s.Name == r.Set.Status }) {
				add(at+".set.status", "status %q does not exist, or the project hides it", r.Set.Status)
			}
		default:
			if !slices.Contains(schema.Categories, schema.Category(r.Set.Category)) {
				add(at+".set.category", "category %q does not exist", r.Set.Category)
			}
		}
	}

	seen := map[string]bool{}
	for i, s := range f.Scoped {
		at := fmt.Sprintf("scoped[%d]", i)
		if seen[s.Where] {
			add(at+".where", "%s is used twice", s.Where)
		}
		seen[s.Where] = true
		where(at+".where", s.Where)
		for j, id := range s.Disable {
			if !ids[id] {
				add(fmt.Sprintf("%s.disable[%d]", at, j), "there is no rule with the id %q", id)
			}
		}
	}

	return append(ps, f.IssueFields.problems("issue_fields")...)
}
