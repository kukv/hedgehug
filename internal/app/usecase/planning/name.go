// Package planning is the operations on the sprints and the releases of a
// store.
package planning

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// reservedNames are the words a query or --to reads as something else.
var reservedNames = []string{"current", "next", "none", "backlog"}

const maxNameLen = 64

// badName refuses a name that a query could not write: what is sprint or
// release.
func badName(what, name string) error {
	bad := func(why string) error {
		return usecase.NewErr(usecase.KindUsage, what+".bad_name", fmt.Errorf("%q cannot name a %s: %s", name, what, why), "value", name)
	}
	switch {
	case name == "" || utf8.RuneCountInString(name) > maxNameLen:
		return bad(fmt.Sprintf("use 1 to %d characters", maxNameLen))
	case name != strings.TrimSpace(name):
		return bad("it starts or ends with a space")
	case strings.ContainsAny(name, `,"`) || strings.ContainsFunc(name, unicode.IsControl):
		return bad(`a comma, a double quote or a control character is not allowed`)
	case slices.Contains(reservedNames, strings.ToLower(name)):
		return bad(fmt.Sprintf("%s is a reserved word", strings.ToLower(name)))
	}
	return nil
}

// contextWorkspace is the workspace of the project of hedgehug use, when that
// project is in the store of ix.
func contextWorkspace(ss *usecase.Session, ix *index.Index) (model.ULID, error) {
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

// pickWorkspace is the workspace arg names, else the context's; zero when
// there is neither.
func pickWorkspace(ss *usecase.Session, ix *index.Index, arg string) (model.ULID, error) {
	if arg != "" {
		w, err := usecase.ResolveWorkspace(ix, arg)
		if err != nil {
			return model.ULID{}, err
		}
		return w.ULID, nil
	}
	return contextWorkspace(ss, ix)
}

// writeWorkspace is the workspace a new sprint or release goes to: arg, the
// context, or the only one of the store.
func writeWorkspace(ss *usecase.Session, ix *index.Index, arg string) (model.ULID, error) {
	ws, err := pickWorkspace(ss, ix, arg)
	if err != nil || !ws.IsZero() {
		return ws, err
	}
	return usecase.OnlyWorkspace(ix)
}

// plan is what find reads of a sprint or a release.
type plan[T any] struct {
	workspace model.ULID
	value     T
}

// find picks the one of hits that a name means: narrowed by workspace, then
// by the context. what is sprint or release.
func find[T any](ss *usecase.Session, ix *index.Index, what, name, workspace string, hits []plan[T]) (T, error) {
	var zero T
	if workspace != "" {
		ws, err := pickWorkspace(ss, ix, workspace)
		if err != nil {
			return zero, err
		}
		hits = slices.DeleteFunc(hits, func(h plan[T]) bool { return h.workspace != ws })
	}
	if len(hits) > 1 {
		ws, err := contextWorkspace(ss, ix)
		if err != nil {
			return zero, err
		}
		if !ws.IsZero() {
			hits = slices.DeleteFunc(hits, func(h plan[T]) bool { return h.workspace != ws })
		}
	}
	switch len(hits) {
	case 0:
		return zero, usecase.NewErr(usecase.KindNotFound, what+".not_found", fmt.Errorf("%s %q is not in this store", what, name), "name", name)
	case 1:
		return hits[0].value, nil
	}
	return zero, usecase.NewErr(usecase.KindAmbiguous, what+".ambiguous",
		fmt.Errorf("%d workspaces have a %s called %q; give --workspace", len(hits), what, name), "name", name)
}

func findSprint(ss *usecase.Session, ix *index.Index, name, workspace string) (*model.Sprint, error) {
	var hits []plan[*model.Sprint]
	for _, sp := range ix.Sprints(model.ULID{}) {
		if strings.EqualFold(sp.Name, name) || sp.ULID.String() == name {
			hits = append(hits, plan[*model.Sprint]{sp.Workspace, sp})
		}
	}
	return find(ss, ix, "sprint", name, workspace, hits)
}

func findRelease(ss *usecase.Session, ix *index.Index, name, workspace string) (*model.Release, error) {
	var hits []plan[*model.Release]
	for _, r := range ix.Releases(model.ULID{}) {
		if strings.EqualFold(r.Name, name) || r.ULID.String() == name {
			hits = append(hits, plan[*model.Release]{r.Workspace, r})
		}
	}
	return find(ss, ix, "release", name, workspace, hits)
}

// parseDate reads a date of key; an empty value is no date.
func parseDate(key, v string) (model.Date, error) {
	if v == "" {
		return model.Date{}, nil
	}
	d, err := model.ParseDate(v)
	if err != nil {
		return model.Date{}, usecase.NewErr(usecase.KindUsage, "assign.bad_value", fmt.Errorf("%s: %q is not a date (YYYY-MM-DD)", key, v), "key", key, "value", v)
	}
	return d, nil
}

func dateText(d model.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.String()
}

// workspaceRef names the workspace ws for a view.
func workspaceRef(ix *index.Index, ws model.ULID) usecase.WorkspaceRef {
	r := usecase.WorkspaceRef{ULID: ws.String()}
	if w, ok := ix.Workspace(ws); ok {
		r.Name, r.IDPrefix, r.Known = w.Name, w.IDPrefix, true
	}
	return r
}
