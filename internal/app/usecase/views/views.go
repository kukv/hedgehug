// Package views is the operations on the saved views of a store.
package views

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/storage/view"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// SavedTable and SavedKanban are the sections of a view, with defaults.
type SavedTable struct {
	Columns []string `json:"columns"`
	GroupBy string   `json:"group_by"`
}

type SavedKanban struct {
	Columns  string `json:"columns"`
	Swimlane string `json:"swimlane"`
}

// SavedView is a saved view as --json prints it: every setting, with the
// defaults of the ones the file leaves out.
type SavedView struct {
	Name   string      `json:"name"`
	Layout string      `json:"layout"`
	Query  string      `json:"query"`
	Table  SavedTable  `json:"table"`
	Kanban SavedKanban `json:"kanban"`
}

func SavedViewOf(v *view.View) SavedView {
	return SavedView{
		Name: v.Name, Layout: v.Layout, Query: v.Query,
		Table:  SavedTable{Columns: v.TableColumns(), GroupBy: v.GroupBy()},
		Kanban: SavedKanban{Columns: v.KanbanColumns(), Swimlane: v.Swimlane()},
	}
}

// ViewChange is what a save sets; a nil field is left as it is (or takes
// its default in a new view).
type ViewChange struct {
	Layout, Query, GroupBy, KanbanColumns, Swimlane *string
	Columns                                         *[]string
}

// ViewResult is what a view operation did.
type ViewResult struct {
	View      SavedView // as saved; for ViewRemove and ViewRename, as it was (just the name for a file that did not decode)
	OldName   string    // ViewRename
	Committed bool
	Commit    string // hex hash; "" when nothing was committed
}

// ViewWrite is what a write to a view answers, in the CLI's --json and in
// the server's response.
type ViewWrite struct {
	View   SavedView `json:"view"`
	Commit *string   `json:"commit"`
}

// Write is the answer to the write: the view, and the commit.
func (r ViewResult) Write() ViewWrite {
	return ViewWrite{View: r.View, Commit: usecase.CommitPtr(r.Commit)}
}

func viewOp(detail string) store.Op { return store.Op{Name: "view", Detail: detail} }

func badViewName(name string) error {
	if err := view.ValidName(name); err != nil {
		return usecase.NewErr(usecase.KindUsage, "view.bad_name", err, "value", name)
	}
	return nil
}

func viewNotFound(name string) error {
	return usecase.NewErr(usecase.KindNotFound, "view.not_found", fmt.Errorf("no view named %q: %w", name, view.ErrNotFound), "name", name)
}

// sameNameOf finds a view other than name whose name is the same but for
// case.
func sameNameOf(es []view.Entry, name string) (string, bool) {
	for _, e := range es {
		if e.Name != name && view.SameName(e.Name, name) {
			return e.Name, true
		}
	}
	return "", false
}

func nameTaken(name, existing string) error {
	if existing == name {
		return usecase.NewErr(usecase.KindConflict, "view.name_taken", fmt.Errorf("view %q already exists", name), "name", name, "existing", existing)
	}
	return usecase.NewErr(usecase.KindConflict, "view.name_taken",
		fmt.Errorf("view %q already exists as %q; names that differ only in case are the same name", name, existing), "name", name, "existing", existing)
}

// ViewSave creates the view name, or changes the settings ch gives of the
// view that has it.
func ViewSave(ss *usecase.Session, name string, ch ViewChange) (ViewResult, error) {
	return writeView(ss, name, ch, true)
}

// ViewPatch is what a PATCH of a view does: it renames the view to *to, or changes the settings ch
// gives, but not both (one operation is one commit, and a rename keeps the bytes of the file).
func ViewPatch(ss *usecase.Session, name string, to *string, ch ViewChange) (ViewResult, error) {
	if to == nil {
		return ViewUpdate(ss, name, ch)
	}
	if ch != (ViewChange{}) {
		return ViewResult{}, usecase.NewErr(usecase.KindUsage, "view.rename_with_settings",
			errors.New("a view is renamed or has its settings changed, not both in one request"))
	}
	return ViewRename(ss, name, *to)
}

// ViewUpdate changes the settings ch gives of the view name, which must exist: unlike ViewSave it
// never creates one, so an update that comes after the view was renamed or removed fails.
func ViewUpdate(ss *usecase.Session, name string, ch ViewChange) (ViewResult, error) {
	return writeView(ss, name, ch, false)
}

func writeView(ss *usecase.Session, name string, ch ViewChange, create bool) (ViewResult, error) {
	if err := badViewName(name); err != nil {
		return ViewResult{}, err
	}
	if ch.Query != nil {
		if _, err := usecase.ParseQuery(*ch.Query); err != nil {
			return ViewResult{}, err
		}
	}
	s, err := ss.OpenStore()
	if err != nil {
		return ViewResult{}, err
	}
	if err := ss.CheckWritable(s); err != nil {
		return ViewResult{}, err
	}
	verb := "save"
	if !create {
		verb = "update"
	}
	var v *view.View
	hash, err := ss.UpdateCommit(s, viewOp(verb+" "+name), func(tx *store.Tx) error {
		es, err := view.List(s.Dir())
		if err != nil {
			return err
		}
		if other, ok := sameNameOf(es, name); ok && create {
			return nameTaken(name, other)
		}
		i := slices.IndexFunc(es, func(e view.Entry) bool { return e.Name == name })
		switch {
		case i < 0 && !create:
			return viewNotFound(name)
		case i < 0:
			v = view.New(name)
			v.Layout = "tree"
		case es[i].Err != nil:
			return usecase.NewErr(usecase.KindInvalid, "view.broken",
				fmt.Errorf("%s: %w; fix it by hand or remove it with hedgehug view rm", es[i].Rel, es[i].Err), "name", name)
		default:
			v = es[i].View
		}
		if err := applyViewChange(v, ch, s); err != nil {
			return err
		}
		data, err := v.Encode()
		if err != nil {
			return err
		}
		return tx.PutFile(model.ViewPath(name), data)
	})
	if err != nil {
		return ViewResult{}, err
	}
	return ViewResult{View: SavedViewOf(v), Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}

// applyViewChange sets what ch gives and checks the view that results.
func applyViewChange(v *view.View, ch ViewChange, s *store.Store) error {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&v.Layout, ch.Layout)
	set(&v.Query, ch.Query)
	if ch.Columns != nil || ch.GroupBy != nil {
		if v.Table == nil {
			v.Table = &view.Table{}
		}
		if ch.Columns != nil {
			v.Table.Columns = slices.Clone(*ch.Columns)
		}
		set(&v.Table.GroupBy, ch.GroupBy)
	}
	if ch.KanbanColumns != nil || ch.Swimlane != nil {
		if v.Kanban == nil {
			v.Kanban = &view.Kanban{}
		}
		set(&v.Kanban.Columns, ch.KanbanColumns)
		set(&v.Kanban.Swimlane, ch.Swimlane)
	}
	if ps := v.Problems(); len(ps) > 0 {
		return usecase.NewErr(usecase.KindUsage, "view.bad_value", errors.New(strings.Join(ps, "; ")))
	}
	if ch.Columns != nil {
		if miss := v.MissingColumns(s.Schema()); len(miss) > 0 {
			return usecase.NewErr(usecase.KindInvalid, "view.bad_column", fmt.Errorf("column %q is neither a field (%s) nor a property of the schema", miss[0], strings.Join(view.Builtins, ", ")), "column", miss[0])
		}
	}
	if v.Layout == "tree" {
		q, err := usecase.ParseQuery(v.Query)
		if err != nil {
			return err
		}
		if usecase.HasTerm(q, "sort") || usecase.HasTerm(q, "group") {
			//nolint:revive,staticcheck // the message is a sentence, as the CLI prints it
			return usecase.NewErr(usecase.KindUsage, "view.tree_order", errors.New("a tree view orders tasks by rank; its query takes neither group: nor sort:"))
		}
	}
	return nil
}

// ViewList lists the views of the store by name. A file that is not a view
// it can read is left out with a warning.
func ViewList(ss *usecase.Session) ([]SavedView, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return nil, err
	}
	es, err := view.List(s.Dir())
	if err != nil {
		return nil, err
	}
	out := []SavedView{}
	for _, e := range es {
		if e.Err != nil {
			ss.AddWarning(fmt.Sprintf("%s: %v; run hedgehug doctor", e.Rel, e.Err))
			continue
		}
		out = append(out, SavedViewOf(e.View))
	}
	return out, nil
}

// ViewGet reads the view called exactly name.
func ViewGet(ss *usecase.Session, name string) (SavedView, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return SavedView{}, err
	}
	e, ok, err := view.Find(s.Dir(), name)
	if err != nil {
		return SavedView{}, err
	}
	if !ok {
		return SavedView{}, viewNotFound(name)
	}
	if e.Err != nil {
		return SavedView{}, usecase.NewErr(usecase.KindInvalid, "view.broken", fmt.Errorf("%s: %w; fix it by hand or remove it with hedgehug view rm", e.Rel, e.Err), "name", name)
	}
	return SavedViewOf(e.View), nil
}

// ViewRemove removes the view called exactly name, readable or not.
func ViewRemove(ss *usecase.Session, name string) (ViewResult, error) {
	s, err := ss.OpenStore()
	if err != nil {
		return ViewResult{}, err
	}
	if err := ss.CheckWritable(s); err != nil {
		return ViewResult{}, err
	}
	var v *view.View
	hash, err := ss.UpdateCommit(s, viewOp("rm "+name), func(tx *store.Tx) error {
		e, ok, err := view.Find(s.Dir(), name)
		if err != nil {
			return err
		}
		if !ok {
			return viewNotFound(name)
		}
		v = e.View
		return tx.RemoveFile(e.Rel)
	})
	if err != nil {
		return ViewResult{}, err
	}
	if v == nil {
		v = view.New(name)
	}
	return ViewResult{View: SavedViewOf(v), Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}

// ViewRename moves the view oldName to newName, as its bytes are. A name
// that differs from oldName only in case is refused: on a file system that
// folds case it is the same file.
func ViewRename(ss *usecase.Session, oldName, newName string) (ViewResult, error) {
	if err := badViewName(newName); err != nil {
		return ViewResult{}, err
	}
	s, err := ss.OpenStore()
	if err != nil {
		return ViewResult{}, err
	}
	if err := ss.CheckWritable(s); err != nil {
		return ViewResult{}, err
	}
	v := view.New(newName)
	hash, err := ss.UpdateCommit(s, viewOp("rename "+oldName+" "+newName), func(tx *store.Tx) error {
		es, err := view.List(s.Dir())
		if err != nil {
			return err
		}
		i := slices.IndexFunc(es, func(e view.Entry) bool { return e.Name == oldName })
		if i < 0 {
			return viewNotFound(oldName)
		}
		for _, e := range es {
			if view.SameName(e.Name, newName) {
				return nameTaken(newName, e.Name)
			}
		}
		data, err := osfs.ReadFile(filepath.Join(s.Dir(), filepath.FromSlash(es[i].Rel)))
		if err != nil {
			return err
		}
		if es[i].View != nil {
			if dv, err := view.Decode(newName, data); err == nil {
				v = dv
			}
		}
		if err := tx.RemoveFile(es[i].Rel); err != nil {
			return err
		}
		return tx.PutFile(model.ViewPath(newName), data)
	})
	if err != nil {
		return ViewResult{}, err
	}
	return ViewResult{View: SavedViewOf(v), OldName: oldName, Committed: !hash.IsZero(), Commit: usecase.CommitHex(hash)}, nil
}
