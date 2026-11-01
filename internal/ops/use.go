package ops

import (
	"fmt"

	"github.com/kukv/hedgehug/internal/config"
	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/index"
)

// useKind is the kind of a task that hh use takes.
const useKind = hhctx.ProjectKind

// UseResult is the project hh use set or shows; Project is nil when there is none.
type UseResult struct {
	Store   string       // the store's name
	Project *index.Entry // a copy of the entry as it was read
	ID      string       // the project's id as its store shows it (store-qualified when prefixes are shared)
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
// KindInvalid, use.not_project. A deleted one is KindConflict, task.deleted.
func (ss *Session) Use(arg string) (UseResult, error) {
	set, err := ss.Mount()
	if err != nil {
		return UseResult{}, err
	}
	hit, err := set.Resolve(arg)
	if err != nil {
		return UseResult{}, ss.mountResolveError(set, arg, err)
	}
	var e index.Entry
	var id string
	var ferr error
	if err := hit.Store.With(func(ix *index.Index) {
		cur, ok := ix.Task(hit.Entry.ULID)
		if !ok {
			cur = hit.Entry
		}
		if ferr = NotDeleted(ix, cur); ferr != nil {
			return
		}
		e, id = *cur, ix.DisplayID(cur)
	}); err != nil {
		return UseResult{}, err
	}
	if ferr != nil {
		return UseResult{}, ferr
	}
	if e.Kind != useKind {
		label := hit.Ref()
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
	return UseResult{Store: hit.Store.Name, Project: &e, ID: id, Changed: changed}, nil
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
	m, e, id, ok, err := st.Use.Find(set)
	if err != nil {
		return UseResult{}, err
	}
	if ok {
		return UseResult{Store: m.Name, Project: &e, ID: id}, nil
	}
	ss.warn(hhctx.UseGoneWarning)
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
