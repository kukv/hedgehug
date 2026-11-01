package doctor

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// named is a sprint or a release as the checks below see it.
type named struct {
	rel, name string
	ws        model.ULID
}

// checkSprintsAndReleases finds the sprints and releases that have no
// workspace of the store, two of a kind that share a name in one workspace,
// and tasks whose sprint or releases are of another workspace. A file that
// does not decode is left to checkFiles.
func checkSprintsAndReleases(s *store.Store, ix *index.Index) []Finding {
	var out []Finding
	sprints, releases := map[model.ULID]model.ULID{}, map[model.ULID]model.ULID{} // the workspace of each
	var sprintNames, releaseNames []named
	for sp, err := range s.Sprints() {
		if err == nil {
			sprints[sp.ULID] = sp.Workspace
			sprintNames = append(sprintNames, named{model.SprintPath(sp.ULID), sp.Name, sp.Workspace})
		}
	}
	for r, err := range s.Releases() {
		if err == nil {
			releases[r.ULID] = r.Workspace
			releaseNames = append(releaseNames, named{model.ReleasePath(r.ULID), r.Name, r.Workspace})
		}
	}
	out = append(out, checkOwners(ix, sprintNames)...)
	out = append(out, checkOwners(ix, releaseNames)...)
	for t, err := range s.Tasks() {
		if err != nil {
			continue
		}
		e, ok := ix.Task(t.ULID)
		if !ok {
			continue
		}
		if t.Sprint != nil {
			if ws, ok := sprints[*t.Sprint]; ok && ws != t.Workspace {
				out = append(out, otherWorkspace(e, "sprint", "sprint", *t.Sprint))
			} else if !ok && missing(s.Sprint(*t.Sprint)) {
				out = append(out, notInStore(e, "sprint", "sprint", *t.Sprint))
			}
		}
		for _, u := range t.Releases {
			if ws, ok := releases[u]; ok && ws != t.Workspace {
				out = append(out, otherWorkspace(e, "releases", "release", u))
			} else if !ok && missing(s.Release(u)) {
				out = append(out, notInStore(e, "releases", "release", u))
			}
		}
	}
	return out
}

// checkOwners finds, among the sprints or the releases in group, those whose
// workspace is missing or not in the store, and those that share a name with
// another of group in the same workspace.
func checkOwners(ix *index.Index, group []named) []Finding {
	var out []Finding
	hint := func(rel string) string {
		return "fix the file by hand and commit, or restore it with `git restore " + rel + "`"
	}
	for i, a := range group {
		if _, ok := ix.Workspace(a.ws); a.ws.IsZero() {
			out = append(out, Finding{Check: "file", Where: a.rel, Msg: "workspace is missing", Hint: hint(a.rel)})
		} else if !ok {
			out = append(out, Finding{Check: "file", Where: a.rel, Msg: fmt.Sprintf("workspace %s is not a workspace of this store", a.ws), Hint: hint(a.rel)})
		}
		for j, b := range group {
			if i != j && !a.ws.IsZero() && a.ws == b.ws && a.name != "" && strings.EqualFold(a.name, b.name) {
				out = append(out, Finding{
					Check: "file", Where: a.rel, Msg: fmt.Sprintf("has the same name %q as %s in the same workspace", a.name, b.rel),
					Hint: "rename one of them by hand and commit",
				})
			}
		}
	}
	return out
}

// otherWorkspace is the finding on the task e, whose field points at the
// sprint or release ulid of another workspace.
func otherWorkspace(e *index.Entry, field, what string, ulid model.ULID) Finding {
	return Finding{
		Check: "task", Where: taskWhere(e),
		Msg:  fmt.Sprintf("%s: %s %s is not in the workspace of this task", field, what, ulid),
		Hint: "take it off with hedgehug edit, or move the task back to its workspace",
	}
}

// missing reports whether err says the file is not there; a file that does
// not decode is there, and checkFiles tells of it.
func missing[T any](_ T, err error) bool { return errors.Is(err, store.ErrNotFound) }

// notInStore is the finding on the task e, whose field points at a sprint or
// release that is not in the store, as the one for a missing task is.
func notInStore(e *index.Entry, field, what string, ulid model.ULID) Finding {
	return Finding{
		Check: "task", Where: taskWhere(e),
		Msg:  fmt.Sprintf("%s: %s %s is not in this store", field, what, ulid),
		Hint: "take it off with hedgehug edit",
	}
}
