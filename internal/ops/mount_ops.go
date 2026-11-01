package ops

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// storeNamed reports whether c has a store called name.
func storeNamed(c *config.Config, name string) bool {
	for _, e := range c.Stores {
		if e.Name == name {
			return true
		}
	}
	return false
}

// forTask is the session that works on the task arg names, and arg as that
// session reads it. With StoreName set it is ss itself; a "<store>:" naming
// another store is a usage error (store.conflict). Otherwise the task is
// looked up in every store (mount.Set.Resolve) and the session is
// ss.ForStore(<its store>), with the "<store>:" taken off arg. A task in
// several stores is KindAmbiguous, task.ambiguous, with the candidates as
// "<store>:<id> (<ULID>) <title>" lines and Args "candidates".
// Not found is task.not_found, as Resolve says now.
func (ss *Session) forTask(arg string) (*Session, string, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, "", err
	}
	// A "<store>:" names the store, which is not looked up.
	if name, rest, ok := strings.Cut(arg, ":"); ok && storeNamed(c, name) {
		switch {
		case ss.StoreName == "":
			return ss.ForStore(name), rest, nil
		case name == ss.StoreName:
			return ss, rest, nil
		}
		return nil, "", newErr(KindUsage, "store.conflict",
			fmt.Errorf("%q is in %s, but --store is %s", arg, name, ss.StoreName), "store", ss.StoreName, "named", name)
	}
	// With one store (or none: Select says so) there is nothing to look in:
	// it is the store, decided as Select does, before the task is looked up.
	if ss.StoreName != "" || len(c.Stores) <= 1 {
		return ss, arg, nil
	}
	set, err := ss.Mount()
	if err != nil {
		return nil, "", err
	}
	hit, err := set.Resolve(arg)
	if err != nil {
		return nil, "", ss.mountResolveError(set, arg, err)
	}
	return ss.ForStore(hit.Store.Name), arg, nil
}

// mountResolveError gives the error of mount.Set.Resolve its Kind and Code.
func (ss *Session) mountResolveError(set *mount.Set, arg string, err error) error {
	var me *mount.AmbiguousError
	var ie *index.AmbiguousError
	switch {
	case errors.As(err, &me):
		lines := make([]string, len(me.Candidates))
		for i, h := range me.Candidates {
			lines[i] = fmt.Sprintf("%s (%s) %s", h.Ref(), h.Entry.ULID, h.Entry.Title)
		}
		return newErr(KindAmbiguous, "task.ambiguous", err, "input", arg, "candidates", lines)
	case errors.As(err, &ie):
		// within one store: let that store spell the candidates out
		var found error
		stores, in := set.Stores, arg
		if m, rest, ok := set.SplitStore(arg); ok {
			stores, in = []*mount.Mounted{m}, rest // "<store>:" names the one store
		}
		for _, m := range stores {
			_ = m.With(func(ix *index.Index) {
				if _, rerr := ix.Resolve(in); errors.As(rerr, &ie) {
					_, found = ss.Resolve(ix, in)
				}
			})
			if found != nil {
				return found
			}
		}
		return fmt.Errorf("%q is ambiguous in a store, but no store says so: %w", arg, err)
	case errors.Is(err, store.ErrNotFound):
		return newErr(KindNotFound, "task.not_found", err, "input", arg)
	}
	return classify(err)
}

// forComment is the session that works on the comment arg names: ss itself
// with StoreName set, otherwise the session of the one store that has a
// comment with that ULID. None is comment.not_found; a ULID in several stores
// is KindAmbiguous, comment.ambiguous.
func (ss *Session) forComment(arg string) (*Session, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	// With one store the store is decided first, as for a task.
	if ss.StoreName != "" || len(c.Stores) <= 1 {
		return ss, nil
	}
	set, err := ss.Mount()
	if err != nil {
		return nil, err
	}
	var in []*mount.Mounted
	notFound := commentNotFound(arg, fmt.Errorf("comment %q: %w", arg, store.ErrNotFound))
	for _, m := range set.Stores {
		var refs []store.CommentRef
		var refsErr error
		if err := m.With(func(*index.Index) { refs, refsErr = m.Store.CommentULIDs() }); err != nil {
			return nil, err
		}
		if refsErr != nil {
			return nil, refsErr
		}
		if _, err := resolveComment(refs, arg); err != nil {
			notFound = err
			continue
		}
		in = append(in, m)
	}
	switch len(in) {
	case 0:
		return nil, notFound
	case 1:
		return ss.ForStore(in[0].Name), nil
	}
	lines := make([]string, len(in))
	for i, m := range in {
		lines[i] = m.Name + ":" + strings.ToUpper(arg)
	}
	return nil, newErr(KindAmbiguous, "comment.ambiguous", &ambiguousError{input: arg, noun: "comments", lines: lines}, "input", arg)
}

// crossStore is the refusal of a reference to a task of another store: id is
// the task as the other store names it.
func crossStore(storeName, id string) error {
	return newErr(KindInvalid, "ref.cross_store",
		fmt.Errorf("%s's %s cannot be referred to (references stay inside a store)", storeName, id),
		"store", storeName, "id", id)
}

// ResolveRef resolves a second id (a blocker, a parent, a task_ref value)
// within the store of ix. An input not in the store that some other mounted
// store has is refused: KindInvalid, ref.cross_store, Args "store", "id".
// A "<store>:" naming this store is taken off; naming another store is
// ref.cross_store when that store has the task and task.not_found when it
// does not. The stores are mounted only when the input is not in ix, or
// names a store.
func (ss *Session) ResolveRef(ix *index.Index, arg string) (*index.Entry, error) {
	self := ix.StoreName()
	if self == "" {
		self = ss.StoreName
	}
	in := arg
	if c, _, err := ss.Config(); err == nil {
		if name, rest, ok := strings.Cut(arg, ":"); ok && storeNamed(c, name) {
			if name == self {
				in = rest
			} else {
				return nil, ss.otherStoreRef(name, rest, arg)
			}
		}
	}
	e, err := ss.Resolve(ix, in)
	if err == nil || KindOf(err) != KindNotFound {
		return e, err
	}
	set, merr := ss.Mount()
	if merr != nil {
		return nil, err
	}
	for _, m := range set.Stores {
		if m.Name == self {
			continue
		}
		if te, ok := findIn(m, in); ok {
			return nil, crossStore(m.Name, idOrULID(te))
		}
	}
	return nil, err
}

// otherStoreRef is the error of the input arg, "<name>:<rest>", that names
// a store other than the one the reference is made in.
func (ss *Session) otherStoreRef(name, rest, arg string) error {
	if set, err := ss.Mount(); err == nil {
		if m, ok := set.Get(name); ok {
			if te, ok := findIn(m, rest); ok {
				return crossStore(name, idOrULID(te))
			}
		}
	}
	return newErr(KindNotFound, "task.not_found", fmt.Errorf("%q: %w", arg, store.ErrNotFound), "input", arg)
}

// findIn is the task arg names in the store m.
func findIn(m *mount.Mounted, arg string) (*index.Entry, bool) {
	var e *index.Entry
	err := m.With(func(ix *index.Index) {
		if u, err := ix.Resolve(arg); err == nil {
			e, _ = ix.Task(u)
		}
	})
	return e, err == nil && e != nil
}

// idOrULIDShown is the id of e as ix shows it, or its ULID when it has none.
func idOrULIDShown(ix *index.Index, e *index.Entry) string {
	if e.ID != "" {
		return ix.DisplayID(e)
	}
	return e.ULID.String()
}

// idOrULID is the id of e, or its ULID when it has none. The id is not
// qualified: crossStore writes the store name apart from it.
func idOrULID(e *index.Entry) string {
	if e.ID != "" {
		return e.ID
	}
	return e.ULID.String()
}

// checkEditRefs refuses a blocker or task_ref value the edit t adds over old
// (nil: every reference counts) when the task is in another mounted store.
// One that is nowhere is left to the store's own check.
func (ss *Session) checkEditRefs(ix *index.Index, old, t *model.Task) error {
	had := map[model.ULID]bool{}
	sc := ix.Schema()
	if old != nil {
		for _, u := range editRefs(old, sc) {
			had[u] = true
		}
	}
	for _, u := range editRefs(t, sc) {
		if had[u] {
			continue
		}
		had[u] = true
		if _, ok := ix.Task(u); ok {
			continue
		}
		if _, err := ss.ResolveRef(ix, u.String()); AsError(err) != nil && AsError(err).Code == "ref.cross_store" {
			return err
		}
	}
	return nil
}

// editRefs are the blockers and task_ref values of t.
func editRefs(t *model.Task, sc *schema.Schema) []model.ULID {
	out := append([]model.ULID(nil), t.BlockedBy...)
	for _, f := range t.Props {
		p, ok := sc.Prop(f.Key)
		if !ok || p.Type != schema.TypeTaskRef || f.Value == nil {
			continue
		}
		items := []*yaml.Node{f.Value}
		if f.Value.Kind == yaml.SequenceNode {
			items = f.Value.Content
		}
		for _, n := range items {
			if n.Kind != yaml.ScalarNode {
				continue
			}
			if u, err := model.ParseULID(n.Value); err == nil {
				out = append(out, u)
			}
		}
	}
	return out
}
