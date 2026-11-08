package tasks

import (
	"errors"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// templateFor finds the template an add expands: the one named, else the
// default of the kind of t where t goes. Without one it is nil. A default that
// is missing or that the project leaves out is a warning, for the add to go
// on; hedgehug doctor reports the first and the schema allows the second.
func templateFor(ss *usecase.Session, s *store.Store, ix *index.Index, in AddInput, t *model.Task) (*model.Template, error) {
	if in.Template == "" && in.NoTemplate {
		return nil, nil
	}
	var ancestors []model.ULID
	if t.Parent != nil {
		if p, ok := ix.Task(*t.Parent); ok {
			ancestors = append(ancestors, ix.Ancestors(p)...)
		}
	}
	only, defaults := ix.Schema().For(ancestors).Templates()
	name := in.Template
	if name == "" {
		if name = defaults[t.Kind]; name == "" {
			return nil, nil
		}
	}
	if only != nil && !slices.Contains(only, name) {
		if in.Template == "" {
			ss.AddWarning(fmt.Sprintf("the default template %s of %s is not one the project uses; it is not applied", name, t.Kind))
			return nil, nil
		}
		return nil, usecase.NewErr(usecase.KindInvalid, "template.not_allowed", fmt.Errorf("the project does not use template %s", name), "name", name)
	}
	tpl, err := s.Template(name)
	switch {
	case err == nil:
		return tpl, nil
	case !errors.Is(err, store.ErrNotFound):
		return nil, usecase.NewErr(usecase.KindInvalid, "template.invalid", fmt.Errorf("template %s: %w", name, err), "name", name)
	case in.Template != "":
		return nil, usecase.NewErr(usecase.KindNotFound, "template.not_found", fmt.Errorf("template %s is not in this store", name), "name", name)
	}
	ss.AddWarning(fmt.Sprintf("the default template %s of %s is not in this store; it is not applied", name, t.Kind))
	return nil, nil
}

// applyItem gives t what it has not been given from the item. parent gives
// the assignees of assignees: inherit, and is nil at the root.
func applyItem(t *model.Task, it *model.TemplateItem, parent []string) {
	if t.Title == "" {
		t.Title = it.Title
	}
	if t.Body == "" {
		t.Body = it.Body
	}
	t.Assignees = slices.Clone(it.Assignees.Logins)
	if it.Assignees.Inherit {
		t.Assignees = slices.Clone(parent)
	}
	t.Watchers = slices.Clone(it.Watchers)
	t.Labels = slices.Clone(it.Labels)
	if len(t.Repos) == 0 {
		t.Repos = slices.Clone(it.Repos)
	}
	t.BlockedBy = slices.Clone(it.BlockedBy)
	t.Sprint = it.Sprint
	t.Releases = slices.Clone(it.Releases)
	t.Links = slices.Clone(it.Links)
	t.Props = slices.Clone(it.Props)
}

// dropForeign takes off t the sprint and the releases that are not of its
// workspace, as the design says for a template that is expanded, and says
// what it took off.
func dropForeign(ix *index.Index, t *model.Task) (usecase.Detached, bool) {
	d := usecase.Detached{Task: t.ULID}
	if t.Sprint != nil {
		sp, ok := ix.Sprint(*t.Sprint)
		if !ok || sp.Workspace != t.Workspace {
			d.Sprint = t.Sprint.String()
			if ok {
				d.Sprint = sp.Name
			}
			t.Sprint = nil
		}
	}
	t.Releases = slices.DeleteFunc(t.Releases, func(u model.ULID) bool {
		r, ok := ix.Release(u)
		if ok && r.Workspace == t.Workspace {
			return false
		}
		name := u.String()
		if ok {
			name = r.Name
		}
		d.Releases = append(d.Releases, name)
		return true
	})
	return d, d.Sprint != "" || len(d.Releases) > 0
}

// expanded is what a template adds to the task of an add.
type expanded struct {
	tasks    []*model.Task // the children and their children, parents first
	detached []usecase.Detached
}

// detachedWithIDs is the detached with the ids the write gave the tasks.
func (x *expanded) detachedWithIDs(root *model.Task) []usecase.Detached {
	for i := range x.detached {
		d := &x.detached[i]
		for _, t := range append([]*model.Task{root}, x.tasks...) {
			if t.ULID == d.Task {
				d.ID = t.ID
			}
		}
	}
	return x.detached
}

// expand makes the tasks of items under parent. A child takes the default kind
// and status of the schema when the item has none, and no template of its own.
func (x *expanded) expand(ix *index.Index, parent *model.Task, items []model.TemplateItem) error {
	for i := range items {
		it := &items[i]
		if it.Title == "" {
			return usecase.NewErr(usecase.KindInvalid, "template.no_title", fmt.Errorf("a child of the template under %q has no title", parent.Title))
		}
		c := &model.Task{ULID: model.NewULID(), Workspace: parent.Workspace, Parent: &parent.ULID, Kind: it.Kind, Status: it.Status}
		var err error
		if c.Kind == "" {
			c.Kind = schema.DefaultKind
		}
		if c.Status == "" {
			if c.Status, err = usecase.DefaultTodo(ix.Schema()); err != nil {
				return err
			}
		}
		applyItem(c, it, parent.Assignees)
		if d, ok := dropForeign(ix, c); ok {
			x.detached = append(x.detached, d)
		}
		c.CreatedBy = parent.CreatedBy
		if c.CreatedBy != "" {
			usecase.AddWatchers(c, c.CreatedBy)
		}
		usecase.WatchNewAssignees(c, nil)
		x.tasks = append(x.tasks, c)
		if err := x.expand(ix, c, it.Children); err != nil {
			return err
		}
	}
	return nil
}
