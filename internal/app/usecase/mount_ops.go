package usecase

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// ForTask is the session that works on the task arg names, and arg as that
// session reads it. With StoreName set it is ss itself; a "<store>:" naming
// another store is a usage error (store.conflict). Otherwise the task is
// looked up in every store (mount.Set.Resolve) and the session is
// ss.ForStore(<its store>), with the "<store>:" taken off arg. A task in
// several stores is KindAmbiguous, task.ambiguous, with the candidates as
// "<store>:<id> (<ULID>) <title>" lines and Args "candidates".
// Not found is task.not_found, as Resolve says now.
func (ss *Session) ForTask(arg string) (*Session, string, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, "", err
	}
	// A "<store>:" names the store, which is not looked up.
	if name, rest, ok := strings.Cut(arg, ":"); ok && StoreExists(c, name) {
		switch {
		case ss.StoreName == "":
			return ss.ForStore(name), rest, nil
		case name == ss.StoreName:
			return ss, rest, nil
		}
		return nil, "", StoreConflict(arg, name, ss.StoreName)
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
	hit, err := ResolveIn(set, arg)
	if err != nil {
		return nil, "", ss.MountResolveError(set, arg, err)
	}
	return ss.ForStore(hit.Store.Name), arg, nil
}

// StoreConflict is the usage error of an id written for the store named but
// asked in the store --store names.
func StoreConflict(arg, named, storeName string) error {
	return NewErr(KindUsage, "store.conflict",
		fmt.Errorf("%q is in %s, but --store is %s", arg, named, storeName), "store", storeName, "named", named)
}

// MountResolveError gives the error of mount.Set.Resolve its Kind and Code.
func (ss *Session) MountResolveError(set *mount.Set, arg string, err error) error {
	var me *AmbiguousError
	var mv *MovedError
	var ie *index.AmbiguousError
	switch {
	case errors.As(err, &mv):
		return movedErr(mv)
	case errors.As(err, &me):
		lines := make([]string, len(me.Candidates))
		for i, h := range me.Candidates {
			lines[i] = fmt.Sprintf("%s (%s) %s", h.Ref(), h.Entry.ULID, h.Entry.Title)
		}
		return NewErr(KindAmbiguous, "task.ambiguous", err, "input", arg, "candidates", lines)
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
		return NewErr(KindNotFound, "task.not_found", err, "input", arg)
	}
	return Classify(err)
}

// movedErr is the error of an input that names a task that moved to another
// store.
func movedErr(mv *MovedError) error {
	args := []any{"input", mv.Input, "store_ulid", mv.Forward.MovedTo.String()}
	if mv.To != nil {
		args = append(args, "store", mv.To.Name)
	}
	if id := mv.NowID(); id != "" {
		args = append(args, "id", id)
	}
	return NewErr(KindNotFound, "task.moved", mv, args...)
}

// MovedUnknownErr is task.moved for a forward that the index cannot name:
// only the task is known, so the keys of the destination are left out.
func MovedUnknownErr(ulid model.ULID) error {
	return NewErr(KindNotFound, "task.moved", fmt.Errorf("%s moved to another store", ulid), "input", ulid.String())
}

// MovedFor is the task.moved error of arg when a forward in the store of ix
// names it, nil when none does.
func (ss *Session) MovedFor(ix *index.Index, arg string) error {
	f, err := ix.ResolveForward(arg)
	if err != nil {
		return nil
	}
	set, _ := ss.Mount()
	var from *mount.Mounted
	if set != nil {
		from, _ = set.Get(ix.StoreName())
	}
	return movedErr(MovedIn(set, from, arg, f))
}

// ForComment is the session that works on the comment arg names: ss itself
// with StoreName set, otherwise the session of the one store that has a
// comment with that ULID. None is comment.not_found; a ULID in several stores
// is KindAmbiguous, comment.ambiguous.
func (ss *Session) ForComment(arg string) (*Session, error) {
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
	notFound := CommentNotFound(arg, fmt.Errorf("comment %q: %w", arg, store.ErrNotFound))
	for _, m := range set.Stores {
		var refs []store.CommentRef
		var refsErr error
		// With, though the index it brings up to HEAD is not used: With is the
		// one way to reach m.Store that holds the store's lock and opens the
		// store again after another handle repacked it. Mounted has no way to
		// that without building the index, and the index is built once per
		// store for the lookups that follow anyway.
		if err := m.With(func(*index.Index) { refs, refsErr = m.Store.CommentULIDs() }); err != nil {
			return nil, err
		}
		if refsErr != nil {
			return nil, refsErr
		}
		if _, err := ResolveComment(refs, arg); err != nil {
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
	return nil, NewErr(KindAmbiguous, "comment.ambiguous", &ambiguousError{input: arg, noun: "comments", lines: lines}, "input", arg)
}

// crossStore is the refusal of a reference to a task of another store: id is
// the task as the other store names it.
func crossStore(storeName, id string) error {
	return NewErr(KindInvalid, "ref.cross_store",
		fmt.Errorf("%s's %s cannot be referred to (references stay inside a store)", storeName, id),
		"store", storeName, "id", id)
}

// ResolveRef resolves a second id (a parent, a task of a relation)
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
		if name, rest, ok := strings.Cut(arg, ":"); ok && StoreExists(c, name) {
			if name == self {
				in = rest
			} else {
				return nil, ss.otherStoreRef(name, rest, arg)
			}
		}
	}
	e, err := ss.Resolve(ix, in)
	if err == nil || KindOf(err) != KindNotFound || AsError(err).Code == "task.moved" {
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
	return NewErr(KindNotFound, "task.not_found", fmt.Errorf("%q: %w", arg, store.ErrNotFound), "input", arg)
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

// CheckEditRefs refuses a task of a relation the edit t adds over old
// (nil: every reference counts) when the task is in another mounted store.
// One that is nowhere is left to the store's own check.
func (ss *Session) CheckEditRefs(ix *index.Index, old, t *model.Task) error {
	had := map[model.ULID]bool{}
	if old != nil {
		for _, u := range editRefs(old) {
			had[u] = true
		}
	}
	for _, u := range editRefs(t) {
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

// editRefs are the tasks t points at through its five relations.
func editRefs(t *model.Task) []model.ULID {
	var out []model.ULID
	for _, rel := range schema.Relations {
		out = append(out, rel.Get(t)...)
	}
	return out
}

func CommentNotFound(arg string, err error) error {
	return NewErr(KindNotFound, "comment.not_found", err, "input", arg)
}

// ResolveComment finds the comment arg names among refs: a full ULID, in any
// case.
func ResolveComment(refs []store.CommentRef, arg string) (store.CommentRef, error) {
	if len(arg) != 26 {
		return store.CommentRef{}, CommentNotFound(arg, fmt.Errorf("comment %q: give the full ULID of the comment: %w", arg, store.ErrNotFound))
	}
	u := strings.ToUpper(arg)
	for _, r := range refs {
		if r.Comment.String() == u {
			return r, nil
		}
	}
	return store.CommentRef{}, CommentNotFound(arg, fmt.Errorf("comment %q: %w", arg, store.ErrNotFound))
}
