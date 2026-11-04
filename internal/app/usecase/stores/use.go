package stores

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// useKind is the kind of a task that hh use takes.
const useKind = usecase.ProjectKind

// UseResult is the project hh use set or shows; Project is nil when there is none.
type UseResult struct {
	Store   string // the store's name
	Project *UseProject
	ID      string // the project's id as its store shows it (store-qualified when prefixes are shared)
	Changed bool
}

// UseProject is the project in the JSON of hh use. ID is as the file has it,
// not as the text shows it with the store's name: the store is a field of its
// own.
type UseProject struct {
	ID    string     `json:"id"`
	ULID  model.ULID `json:"ulid"`
	Title string     `json:"title"`
}

// UseView is the JSON of hh use when there is a project.
type UseView struct {
	Store   string     `json:"store"`
	Project UseProject `json:"project"`
}

func useProjectOf(e index.Entry) *UseProject {
	id := e.ID
	if id == "" {
		id = e.ULID.String()
	}
	return &UseProject{ID: id, ULID: e.ULID, Title: e.Title}
}

// View is r as the JSON of hh use: {store, project}, or {} when there is none.
func (r UseResult) View() any {
	if r.Project == nil {
		return struct{}{}
	}
	return UseView{Store: r.Store, Project: *r.Project}
}

// loadContext reads context.json, warning of a broken one.
func loadContext(ss *usecase.Session) (current.State, config.Paths, error) {
	_, p, err := ss.Config()
	if err != nil {
		return current.State{}, p, err
	}
	return current.Load(p.DataDir, ss.AddWarning), p, nil
}

// Use makes the project arg names (any store, "<store>:" allowed) the
// default parent of new tasks. With --store the project is looked for in
// that store only, and a "<store>:" naming another one is store.conflict, as
// for the other commands that take an id. A task whose kind is not project is
// KindInvalid, use.not_project. A deleted one is KindConflict, task.deleted.
func Use(ss *usecase.Session, arg string) (UseResult, error) {
	set, err := ss.Mount()
	if err != nil {
		return UseResult{}, err
	}
	hit, err := useResolve(ss, set, arg)
	if err != nil {
		return UseResult{}, err
	}
	var e index.Entry
	var id string
	var ferr error
	if err := hit.Store.With(func(ix *index.Index) {
		cur, ok := ix.Task(hit.Entry.ULID)
		if !ok {
			cur = hit.Entry
		}
		if ferr = usecase.NotDeleted(ix, cur); ferr != nil {
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
		return UseResult{}, usecase.NewErr(usecase.KindInvalid, "use.not_project",
			fmt.Errorf("%s is a %s, not a project", label, e.Kind), "id", label, "kind", e.Kind)
	}
	st, p, err := loadContext(ss)
	if err != nil {
		return UseResult{}, err
	}
	next := current.Ref{Store: hit.Store.ULID, Project: e.ULID}
	changed := st.Use == nil || *st.Use != next
	if changed {
		st.Use = &next
		if err := current.Save(p.DataDir, st); err != nil {
			return UseResult{}, err
		}
	}
	return UseResult{Store: hit.Store.Name, Project: useProjectOf(e), ID: id, Changed: changed}, nil
}

// useResolve finds the project arg names, in the store --store names when
// there is one.
func useResolve(ss *usecase.Session, set *mount.Set, arg string) (usecase.Hit, error) {
	if ss.StoreName == "" {
		hit, err := usecase.ResolveIn(set, arg)
		if err != nil {
			return usecase.Hit{}, ss.MountResolveError(set, arg, err)
		}
		return hit, nil
	}
	c, _, err := ss.Config()
	if err != nil {
		return usecase.Hit{}, err
	}
	scoped := ss.StoreName + ":" + arg
	if name, _, ok := strings.Cut(arg, ":"); ok && usecase.StoreExists(c, name) {
		if name != ss.StoreName {
			return usecase.Hit{}, usecase.StoreConflict(arg, name, ss.StoreName)
		}
		scoped = arg
	}
	if _, ok := set.Get(ss.StoreName); !ok {
		// not found, or one that does not open, as the other commands say it
		if _, _, err := ss.Entry(); err != nil {
			return usecase.Hit{}, err
		}
		for _, sk := range set.Skipped {
			if sk.Name == ss.StoreName {
				return usecase.Hit{}, usecase.OpenError(sk.Name, sk.Err)
			}
		}
	}
	hit, err := usecase.ResolveIn(set, scoped)
	var moved *usecase.MovedError
	if errors.Is(err, store.ErrNotFound) && !errors.As(err, &moved) {
		// say what the user typed, not the store-qualified form made here
		return usecase.Hit{}, usecase.NewErr(usecase.KindNotFound, "task.not_found", fmt.Errorf("%q: %w", arg, store.ErrNotFound), "input", arg)
	}
	if err != nil {
		// the input as the user typed it, not the form made here
		var me *usecase.AmbiguousError
		if errors.As(err, &me) {
			me.Input = arg
		}
		if moved != nil {
			moved.Input = arg
		}
		return usecase.Hit{}, ss.MountResolveError(set, arg, err)
	}
	return hit, nil
}

// UseShow is the project hh use set, or a zero result. A project that is
// gone (its store is not mounted, it is not there, deleted or a forward)
// is warned of (D12) and shown as none.
func UseShow(ss *usecase.Session) (UseResult, error) {
	st, _, err := loadContext(ss)
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
	m, e, id, ok, err := usecase.FindProject(*st.Use, set)
	if err != nil {
		return UseResult{}, err
	}
	if ok {
		return UseResult{Store: m.Name, Project: useProjectOf(e), ID: id}, nil
	}
	ss.AddWarning(usecase.UseGoneWarning)
	return UseResult{}, nil
}

// UseClear removes it; Changed is false when there was none.
func UseClear(ss *usecase.Session) (UseResult, error) {
	st, p, err := loadContext(ss)
	if err != nil {
		return UseResult{}, err
	}
	if st.Use == nil {
		return UseResult{}, nil
	}
	st.Use = nil
	if err := current.Save(p.DataDir, st); err != nil {
		return UseResult{}, err
	}
	return UseResult{Changed: true}, nil
}
