package ops

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/config"
	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/store"
)

// useKind is the kind of a task that hh use takes.
const useKind = "project"

// UseResult is the project hh use set or shows; Project is nil when there is none.
type UseResult struct {
	Store   string // the store's name
	Project *index.Entry
	Index   *index.Index
	Changed bool
}

// loadContext reads context.json, warning of a broken one.
func (ss *Session) loadContext() (hhctx.State, config.Paths, error) {
	_, p, err := ss.Config()
	if err != nil {
		return hhctx.State{}, p, err
	}
	return hhctx.Load(p.DataDir, ss.warn), p, nil
}

// Use makes the project arg names (any store, "<store>:" allowed) the
// default parent of new tasks. A task whose kind is not project is
// KindInvalid, use.not_project. A deleted one is not found.
func (ss *Session) Use(arg string) (UseResult, error) {
	set, err := ss.Mount()
	if err != nil {
		return UseResult{}, err
	}
	hit, err := set.Resolve(arg)
	if err != nil {
		return UseResult{}, ss.mountResolveError(set, arg, err)
	}
	e := hit.Entry
	label := hit.Ref()
	if e.DeletedAt != nil {
		return UseResult{}, newErr(KindNotFound, "task.not_found",
			fmt.Errorf("%s is deleted: %w", label, store.ErrNotFound), "input", arg)
	}
	if e.Kind != useKind {
		return UseResult{}, newErr(KindInvalid, "use.not_project",
			fmt.Errorf("%s is a %s, not a project", label, e.Kind), "id", label, "kind", e.Kind)
	}
	st, p, err := ss.loadContext()
	if err != nil {
		return UseResult{}, err
	}
	next := hhctx.Ref{Store: hit.Store.ULID, Project: e.ULID}
	changed := st.Use == nil || *st.Use != next
	if changed {
		st.Use = &next
		if err := hhctx.Save(p.DataDir, st); err != nil {
			return UseResult{}, err
		}
	}
	ix, err := hit.Store.Index()
	if err != nil {
		return UseResult{}, err
	}
	return UseResult{Store: hit.Store.Name, Project: e, Index: ix, Changed: changed}, nil
}

// UseShow is the project hh use set, or a zero result. A project that is
// gone (its store is not mounted, it is not there, deleted or a forward)
// is warned of (D12) and shown as none.
func (ss *Session) UseShow() (UseResult, error) {
	st, _, err := ss.loadContext()
	if err != nil {
		return UseResult{}, err
	}
	if st.Use == nil {
		return UseResult{}, nil
	}
	set, err := ss.Mount()
	if err != nil {
		return UseResult{}, err
	}
	var m *mount.Mounted
	for _, c := range set.Stores {
		if c.ULID == st.Use.Store {
			m = c
			break
		}
	}
	if m != nil {
		var res UseResult
		err := m.With(func(ix *index.Index) {
			if e, ok := ix.Task(st.Use.Project); ok && e.DeletedAt == nil && e.Kind == useKind {
				res = UseResult{Store: m.Name, Project: e, Index: ix}
			}
		})
		if err != nil {
			return UseResult{}, err
		}
		if res.Project != nil {
			return res, nil
		}
	}
	ss.warn("the project of hedgehug use is not found (hedgehug use --clear removes it)")
	return UseResult{}, nil
}

// UseClear removes it; Changed is false when there was none.
func (ss *Session) UseClear() (UseResult, error) {
	st, p, err := ss.loadContext()
	if err != nil {
		return UseResult{}, err
	}
	if st.Use == nil {
		return UseResult{}, nil
	}
	st.Use = nil
	if err := hhctx.Save(p.DataDir, st); err != nil {
		return UseResult{}, err
	}
	return UseResult{Changed: true}, nil
}
