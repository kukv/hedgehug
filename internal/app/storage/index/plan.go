package index

import (
	"cmp"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// readPlans decodes every sprint and release file of tree. A file that does
// not decode is skipped; doctor reports it. Like the workspaces they are read
// from the tree each time, as they hold YAML nodes that gob cannot carry.
func (ix *Index) readPlans(tree plumbing.Hash) error {
	sprints := map[model.ULID]*model.Sprint{}
	if err := readPlanFiles(ix, tree, "sprints/", func(b []byte) {
		if sp, err := model.DecodeSprint(b); err == nil {
			sprints[sp.ULID] = sp
		}
	}); err != nil {
		return err
	}
	releases := map[model.ULID]*model.Release{}
	if err := readPlanFiles(ix, tree, "releases/", func(b []byte) {
		if r, err := model.DecodeRelease(b); err == nil {
			releases[r.ULID] = r
		}
	}); err != nil {
		return err
	}
	ix.sprints, ix.releases = sprints, releases
	return nil
}

func readPlanFiles(ix *Index, tree plumbing.Hash, dir string, decode func([]byte)) error {
	for p, err := range ix.repo.Files(tree, dir) {
		if err != nil {
			return err
		}
		if path.Ext(p) != ".md" {
			continue
		}
		b, err := ix.repo.ReadFile(tree, p)
		if err != nil {
			return err
		}
		decode(b)
	}
	return nil
}

// byDate orders two plans by their dates, those with none last, then ULID.
func byDate(a, b model.Date, au, bu model.ULID) int {
	switch {
	case a.IsZero() != b.IsZero():
		if a.IsZero() {
			return 1
		}
		return -1
	case !a.IsZero():
		if c := a.Time().Compare(b.Time()); c != 0 {
			return c
		}
	}
	return cmp.Compare(au.String(), bu.String())
}

// Sprints returns the sprints of the workspace ws, or of the store when ws is
// zero, by start date then ULID, those with no start last.
func (ix *Index) Sprints(ws model.ULID) []*model.Sprint {
	out := slices.Collect(maps.Values(ix.sprints))
	if !ws.IsZero() {
		out = slices.DeleteFunc(out, func(sp *model.Sprint) bool { return sp.Workspace != ws })
	}
	slices.SortFunc(out, func(a, b *model.Sprint) int { return byDate(a.Start, b.Start, a.ULID, b.ULID) })
	return out
}

// Sprint returns the sprint of ulid.
func (ix *Index) Sprint(ulid model.ULID) (*model.Sprint, bool) {
	sp, ok := ix.sprints[ulid]
	return sp, ok
}

// SprintNamed returns the sprint of the workspace ws called name, ignoring
// case.
func (ix *Index) SprintNamed(ws model.ULID, name string) (*model.Sprint, bool) {
	for _, sp := range ix.Sprints(ws) {
		if strings.EqualFold(sp.Name, name) {
			return sp, true
		}
	}
	return nil, false
}

// NextSprint returns the planned sprint of the workspace ws that starts
// first.
func (ix *Index) NextSprint(ws model.ULID) (*model.Sprint, bool) {
	for _, sp := range ix.Sprints(ws) {
		if sp.State == model.SprintPlanned {
			return sp, true
		}
	}
	return nil, false
}

// Releases returns the releases of the workspace ws, or of the store when ws
// is zero, by date then ULID, those with no date last.
func (ix *Index) Releases(ws model.ULID) []*model.Release {
	out := slices.Collect(maps.Values(ix.releases))
	if !ws.IsZero() {
		out = slices.DeleteFunc(out, func(r *model.Release) bool { return r.Workspace != ws })
	}
	slices.SortFunc(out, func(a, b *model.Release) int { return byDate(a.Date, b.Date, a.ULID, b.ULID) })
	return out
}

// Release returns the release of ulid.
func (ix *Index) Release(ulid model.ULID) (*model.Release, bool) {
	r, ok := ix.releases[ulid]
	return r, ok
}

// ReleaseNamed returns the release of the workspace ws called name, ignoring
// case.
func (ix *Index) ReleaseNamed(ws model.ULID, name string) (*model.Release, bool) {
	for _, r := range ix.Releases(ws) {
		if strings.EqualFold(r.Name, name) {
			return r, true
		}
	}
	return nil, false
}
