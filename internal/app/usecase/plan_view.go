package usecase

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

// PlanRef names the sprint or a release of a task.
type PlanRef struct {
	ULID  string `json:"ulid"`
	Name  string `json:"name,omitempty" doc:"Empty when the store has no such file"`
	State string `json:"state,omitempty"`
}

// Text is the name of p, or its ULID when the store has no such file.
func (p PlanRef) Text() string {
	if p.Name != "" {
		return p.Name
	}
	return p.ULID
}

func sprintRef(ix *index.Index, ulid model.ULID) PlanRef {
	r := PlanRef{ULID: ulid.String()}
	if sp, ok := ix.Sprint(ulid); ok {
		r.Name, r.State = sp.Name, sp.State
	}
	return r
}

func releaseRef(ix *index.Index, ulid model.ULID) PlanRef {
	r := PlanRef{ULID: ulid.String()}
	if rel, ok := ix.Release(ulid); ok {
		r.Name, r.State = rel.Name, rel.State
	}
	return r
}

// ApplyTaskAssign is ApplyAssign with sprint and releases, whose values are
// names in the workspace of t, or ULIDs. t has its workspace already.
func ApplyTaskAssign(t *model.Task, a Assign, ix *index.Index, resolve func(string) (model.ULID, error)) error {
	switch a.Key {
	case "sprint":
		if a.Op != "=" {
			return NewErr(KindInvalid, "assign.single", fmt.Errorf("sprint holds one value; use sprint=<name>"), "key", a.Key)
		}
		if a.Value == "" {
			t.Sprint = nil
			return nil
		}
		ulid, err := sprintOf(ix, t.Workspace, a.Value)
		if err != nil {
			return err
		}
		t.Sprint = &ulid
		return nil
	case "releases":
		return setULIDs(&t.Releases, a, func(v string) (model.ULID, error) { return releaseOf(ix, t.Workspace, v) })
	}
	return ApplyAssign(t, a, ix.Schema(), resolve)
}

// sprintOf finds the sprint of the workspace ws that v names. A ULID is taken
// as it is: the store checks what it points at.
func sprintOf(ix *index.Index, ws model.ULID, v string) (model.ULID, error) {
	if sp, ok := ix.SprintNamed(ws, v); ok {
		return sp.ULID, nil
	}
	if u, err := model.ParseULID(v); err == nil {
		return u, nil
	}
	return model.ULID{}, NewErr(KindNotFound, "sprint.not_found", fmt.Errorf("sprint %q is not in the workspace of the task", v), "name", v)
}

// releaseOf is sprintOf for a release.
func releaseOf(ix *index.Index, ws model.ULID, v string) (model.ULID, error) {
	if r, ok := ix.ReleaseNamed(ws, v); ok {
		return r.ULID, nil
	}
	if u, err := model.ParseULID(v); err == nil {
		return u, nil
	}
	return model.ULID{}, NewErr(KindNotFound, "release.not_found", fmt.Errorf("release %q is not in the workspace of the task", v), "name", v)
}
