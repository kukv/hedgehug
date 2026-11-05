package usecase

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

// ContextWorkspace is the workspace of the project of hedgehug use, when that
// project is in the store of ix.
func ContextWorkspace(ss *Session, ix *index.Index) (model.ULID, error) {
	_, p, err := ss.Config()
	if err != nil {
		return model.ULID{}, err
	}
	st := current.Load(p.DataDir, ss.AddWarning)
	if st.Use != nil {
		if e, ok := ix.Task(st.Use.Project); ok {
			return e.Workspace, nil
		}
	}
	return model.ULID{}, nil
}

// PickWorkspace is the workspace arg names, else the context's; zero when
// there is neither.
func PickWorkspace(ss *Session, ix *index.Index, arg string) (model.ULID, error) {
	if arg != "" {
		w, err := ResolveWorkspace(ix, arg)
		if err != nil {
			return model.ULID{}, err
		}
		return w.ULID, nil
	}
	return ContextWorkspace(ss, ix)
}

// plan is what find reads of a sprint or a release.
type plan[T any] struct {
	workspace model.ULID
	value     T
}

// find picks the one of hits that a name means: narrowed by workspace, then
// by the context. what is sprint or release.
func find[T any](ss *Session, ix *index.Index, what, name, workspace string, hits []plan[T]) (T, error) {
	var zero T
	if workspace != "" {
		ws, err := PickWorkspace(ss, ix, workspace)
		if err != nil {
			return zero, err
		}
		hits = slices.DeleteFunc(hits, func(h plan[T]) bool { return h.workspace != ws })
	}
	if len(hits) > 1 {
		ws, err := ContextWorkspace(ss, ix)
		if err != nil {
			return zero, err
		}
		if !ws.IsZero() {
			hits = slices.DeleteFunc(hits, func(h plan[T]) bool { return h.workspace != ws })
		}
	}
	switch len(hits) {
	case 0:
		return zero, NewErr(KindNotFound, what+".not_found", fmt.Errorf("%s %q is not in this store", what, name), "name", name)
	case 1:
		return hits[0].value, nil
	}
	return zero, NewErr(KindAmbiguous, what+".ambiguous",
		fmt.Errorf("%d workspaces have a %s called %q; give --workspace", len(hits), what, name), "name", name)
}

// FindSprint finds the sprint name, or a ULID, means.
func FindSprint(ss *Session, ix *index.Index, name, workspace string) (*model.Sprint, error) {
	var hits []plan[*model.Sprint]
	for _, sp := range ix.Sprints(model.ULID{}) {
		if strings.EqualFold(sp.Name, name) || sp.ULID.String() == name {
			hits = append(hits, plan[*model.Sprint]{sp.Workspace, sp})
		}
	}
	return find(ss, ix, "sprint", name, workspace, hits)
}

// FindRelease is FindSprint for a release.
func FindRelease(ss *Session, ix *index.Index, name, workspace string) (*model.Release, error) {
	var hits []plan[*model.Release]
	for _, r := range ix.Releases(model.ULID{}) {
		if strings.EqualFold(r.Name, name) || r.ULID.String() == name {
			hits = append(hits, plan[*model.Release]{r.Workspace, r})
		}
	}
	return find(ss, ix, "release", name, workspace, hits)
}
