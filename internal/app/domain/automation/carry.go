package automation

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// projectOf reads a where; it is false when where names no project.
func projectOf(where string) (model.ULID, bool) {
	s, ok := strings.CutPrefix(where, WherePrefix)
	if !ok {
		return model.ULID{}, false
	}
	u, err := model.ParseULID(s)
	return u, err == nil
}

// clone copies f so that its rules and scoped items can change. The copy
// shares the document of f, which Encode reads for the comments.
func (f *File) clone() *File {
	c := *f
	c.Rules = slices.Clone(f.Rules)
	c.Scoped = slices.Clone(f.Scoped)
	return &c
}

// Projects are the projects that the where of a rule or a scoped item names,
// in the order of the file, each once.
func (f *File) Projects() []model.ULID {
	var out []model.ULID
	add := func(where string) {
		if u, ok := projectOf(where); ok && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	for _, r := range f.Rules {
		add(r.Where)
	}
	for _, s := range f.Scoped {
		add(s.Where)
	}
	return out
}

// Carry returns f with what of src applies to the projects and fits sc, the
// schema of f with the scoped entries that come along, and what it left out,
// one line each, without repeats: "rule <id>" for a where rule whose id f
// has or whose status or category sc does not have for the project, "disable
// <id>" for a disable whose rule f has not, with the rules carried, or whose
// rule was left out (a rule of f with its id is another rule). f is not
// changed. The result may still fail Check; the caller checks.
func (f *File) Carry(src *File, projects []model.ULID, sc *schema.Schema) (next *File, dropped []string) {
	next = f.clone()
	drop := func(what, id string) {
		if line := what + " " + id; !slices.Contains(dropped, line) {
			dropped = append(dropped, line)
		}
	}
	for _, r := range src.Rules {
		u, ok := projectOf(r.Where)
		if !ok || !slices.Contains(projects, u) {
			continue
		}
		if _, taken := next.Rule(r.ID); taken || !r.fits(u, sc) {
			drop("rule", r.ID)
			continue
		}
		next.Rules = append(next.Rules, r)
	}
	for _, s := range src.Scoped {
		u, ok := projectOf(s.Where)
		if !ok || !slices.Contains(projects, u) {
			continue
		}
		for _, id := range s.Disable {
			if _, ok := next.Rule(id); ok && !slices.Contains(dropped, "rule "+id) {
				next.disableIn(u, s.Where, id)
			} else {
				drop("disable", id)
			}
		}
	}
	return next, dropped
}

// fits reports whether what r sets exists for the project u in sc. A set with
// nothing in it fits: Check reports it.
func (r Rule) fits(u model.ULID, sc *schema.Schema) bool {
	if r.Set.Status != "" && !slices.ContainsFunc(sc.For([]model.ULID{u}).Statuses(), func(s schema.Status) bool { return s.Name == r.Set.Status }) {
		return false
	}
	if r.Set.Category != "" {
		if _, ok := sc.DefaultStatus(schema.Category(r.Set.Category)); !ok {
			return false
		}
	}
	return true
}

// scopedIndex is the index of the scoped item of the project u, or -1.
func (f *File) scopedIndex(u model.ULID) int {
	return slices.IndexFunc(f.Scoped, func(s Scoped) bool {
		p, ok := projectOf(s.Where)
		return ok && p == u
	})
}

// disableIn adds id to the scoped item of the project u, which is new with the
// given where when there is none.
func (f *File) disableIn(u model.ULID, where, id string) {
	i := f.scopedIndex(u)
	if i < 0 {
		f.Scoped = append(f.Scoped, Scoped{Where: where})
		i = len(f.Scoped) - 1
	}
	if !slices.Contains(f.Scoped[i].Disable, id) {
		f.Scoped[i].Disable = append(slices.Clone(f.Scoped[i].Disable), id)
	}
}

// Without returns f without the where rules and the scoped items of the
// projects; a disable elsewhere that names a rule that went, and that no rule
// left has, goes with it. f is not changed.
func (f *File) Without(projects []model.ULID) *File {
	in := func(where string) bool {
		u, ok := projectOf(where)
		return ok && slices.Contains(projects, u)
	}
	next := f.clone()
	next.Scoped = slices.DeleteFunc(next.Scoped, func(s Scoped) bool { return in(s.Where) })
	next.Rules = slices.DeleteFunc(next.Rules, func(r Rule) bool { return in(r.Where) })
	gone := func(id string) bool {
		_, kept := next.Rule(id)
		return !kept && slices.ContainsFunc(f.Rules, func(r Rule) bool { return r.ID == id && in(r.Where) })
	}
	for i := range next.Scoped {
		next.Scoped[i].Disable = slices.DeleteFunc(slices.Clone(next.Scoped[i].Disable), gone)
	}
	next.dropEmptyScoped()
	return next
}

// LeftOut is what of f applies to the projects and dst does not hold, in the
// words of Carry: what a Carry that dst came from left out, when only the two
// files are left to tell it.
func (f *File) LeftOut(dst *File, projects []model.ULID) []string {
	var out []string
	add := func(line string) {
		if !slices.Contains(out, line) {
			out = append(out, line)
		}
	}
	for _, r := range f.Rules {
		u, ok := projectOf(r.Where)
		if !ok || !slices.Contains(projects, u) {
			continue
		}
		if !slices.ContainsFunc(dst.Rules, func(d Rule) bool {
			p, ok := projectOf(d.Where)
			return d.ID == r.ID && ok && p == u
		}) {
			add("rule " + r.ID)
		}
	}
	for _, s := range f.Scoped {
		u, ok := projectOf(s.Where)
		if !ok || !slices.Contains(projects, u) {
			continue
		}
		i := dst.scopedIndex(u)
		for _, id := range s.Disable {
			if i < 0 || !slices.Contains(dst.Scoped[i].Disable, id) {
				add("disable " + id)
			}
		}
	}
	return out
}
