package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// InboxStore is a store whose inbox an operation works on.
type InboxStore struct {
	Name  string
	Store *store.Store
	Box   *inbox.Box
}

// NoAccountsWarning is the warning of InboxStores for a config without
// accounts, where mentions and assignments cannot be told. The CLI prints
// each warning InboxStores returns as "hedgehug: warning: " + it, which is
// the line it printed before; InboxList, InboxRead and InboxRevert send it
// through Warn, so without the "hedgehug: " prefix.
const NoAccountsWarning = "accounts are not set up in config.yaml; mentions and assignments cannot be told"

// InboxStores opens the stores the operation works on, after the inbox of
// each has taken in what reached the store since it last looked and the
// merge driver's records. The warnings are for the caller to show.
func (ss *Session) InboxStores() ([]InboxStore, []string, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, nil, err
	}
	targets, err := ss.SyncTargets(c, true)
	if err != nil {
		return nil, nil, err
	}
	var warnings []string
	if len(c.Accounts) == 0 {
		warnings = append(warnings, NoAccountsWarning)
	}
	var out []InboxStore
	for _, e := range targets {
		s, err := ss.openStoreAt(e.Path)
		if err != nil {
			return nil, warnings, fmt.Errorf("store %s: %w", e.Name, err)
		}
		box := inbox.Open(s)
		if len(c.Accounts) > 0 {
			head, err := s.Repo().Head()
			if err == nil {
				_, err = box.Catch(c.Me(), head)
			}
			if err != nil {
				return nil, warnings, fmt.Errorf("store %s: %w", e.Name, err)
			}
		}
		out = append(out, InboxStore{e.Name, s, box})
	}
	return out, warnings, nil
}

// openInbox is InboxStores with its warnings given to Warn.
func (ss *Session) openInbox() ([]InboxStore, error) {
	stores, warnings, err := ss.InboxStores()
	for _, w := range warnings {
		ss.warn(w)
	}
	return stores, err
}

// WarnInbox gives Warn what the boxes skipped, once all their reading is
// done.
func (ss *Session) WarnInbox(stores []InboxStore) {
	for _, st := range stores {
		for _, w := range st.Box.Warnings() {
			ss.warn(st.Name + ": " + w)
		}
	}
}

// SyncTargets are the stores an operation works on: the one StoreName
// names, or all that have a remote, or with all every store.
func (ss *Session) SyncTargets(c *config.Config, all bool) ([]config.StoreEntry, error) {
	if ss.StoreName != "" {
		e, err := c.Select(ss.StoreName)
		if err != nil && !errors.Is(err, config.ErrNoStores) {
			return nil, newErr(KindNotFound, "store.not_found", err, "name", ss.StoreName)
		}
		return []config.StoreEntry{e}, err
	}
	if len(c.Stores) == 0 {
		return nil, config.ErrNoStores
	}
	var out []config.StoreEntry
	for _, e := range c.Stores {
		if all || len(e.Remotes) > 0 {
			out = append(out, e)
		}
	}
	return out, nil
}

// InboxItemView is an item as --json prints it: the task as a reference to
// the task as it is now. Target and Content are the words for it in a list.
type InboxItemView struct {
	inbox.Item
	Store   string `json:"store"`
	Task    *Ref   `json:"task"`
	Target  string `json:"-"`
	Content string `json:"-"`
}

// InboxList lists the items of the stores, the newest first.
func (ss *Session) InboxList(all bool) ([]InboxStore, []InboxItemView, error) {
	stores, err := ss.openInbox()
	if err != nil {
		return nil, nil, err
	}
	items, err := ss.InboxListOf(stores, all)
	return stores, items, err
}

// InboxListOf lists the items of stores, which InboxStores opened, and gives
// Warn what their boxes skipped.
func (ss *Session) InboxListOf(stores []InboxStore, all bool) ([]InboxItemView, error) {
	defer ss.WarnInbox(stores)
	type row struct {
		store string
		item  inbox.Item
		ix    *index.Index
	}
	var rows []row
	for _, st := range stores {
		items, err := st.Box.List(all)
		if err != nil {
			return nil, fmt.Errorf("store %s: %w", st.Name, err)
		}
		if len(items) == 0 {
			continue
		}
		ix, err := index.Open(st.Store)
		if err != nil {
			ss.warn(fmt.Sprintf("%s: read the index, so titles are not shown: %v", st.Name, err))
			ix = nil
		}
		for _, it := range items {
			rows = append(rows, row{st.Name, it, ix})
		}
	}
	slices.SortStableFunc(rows, func(x, y row) int {
		if c := y.item.Time.Compare(x.item.Time); c != 0 {
			return c
		}
		return strings.Compare(y.item.ULID.String(), x.item.ULID.String())
	})
	out := make([]InboxItemView, len(rows))
	for i, r := range rows {
		out[i] = InboxItemView{
			Item: r.item, Store: r.store, Task: inboxTaskRef(r.ix, r.item),
			Target: inboxTarget(r.ix, r.item), Content: InboxContent(r.ix, r.item, r.store, len(stores) > 1),
		}
	}
	return out, nil
}

// ParseInboxItem reads the full ULID of an item; a short one is a mistake of
// the arguments (KindUsage).
func ParseInboxItem(arg string) (model.ULID, error) {
	u, err := model.ParseULID(arg)
	if err != nil {
		return model.ULID{}, newErr(KindUsage, "inbox.ulid", fmt.Errorf("%q: give the full ULID of the item", arg), "input", arg)
	}
	return u, nil
}

// ParseInboxItems reads the full ULIDs of items, each once.
func ParseInboxItems(args []string) ([]model.ULID, error) {
	var ulids []model.ULID
	for _, arg := range args {
		u, err := ParseInboxItem(arg)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(ulids, u) {
			ulids = append(ulids, u)
		}
	}
	return ulids, nil
}

// findItem finds the store whose inbox holds the item u.
func findItem(stores []InboxStore, u model.ULID) (InboxStore, inbox.Item, error) {
	for _, st := range stores {
		it, ok, err := st.Box.Item(u)
		if err != nil {
			return InboxStore{}, inbox.Item{}, fmt.Errorf("store %s: %w", st.Name, err)
		}
		if ok {
			return st, it, nil
		}
	}
	return InboxStore{}, inbox.Item{}, newErr(KindNotFound, "inbox.not_found", fmt.Errorf("inbox item %s: %w", u, store.ErrNotFound), "input", u.String())
}

// InboxRead marks items read, by their full ULIDs, or every unread item with
// all. It returns how many it marked.
func (ss *Session) InboxRead(args []string, all bool) (marked int, err error) {
	ulids, err := ParseInboxItems(args)
	if err != nil {
		return 0, err
	}
	stores, err := ss.openInbox()
	if err != nil {
		return 0, err
	}
	read, err := ss.InboxReadOf(stores, ulids, all)
	return len(read), err
}

// InboxReadOf marks read, in stores which InboxStores opened, the items
// ulids or all the unread ones, and returns the ULIDs it marked. It gives
// Warn what the boxes skipped.
func (ss *Session) InboxReadOf(stores []InboxStore, ulids []model.ULID, all bool) ([]string, error) {
	defer ss.WarnInbox(stores)
	read := []string{}
	if all {
		for _, st := range stores {
			items, err := st.Box.List(false)
			if err != nil {
				return nil, fmt.Errorf("store %s: %w", st.Name, err)
			}
			if err := st.Box.MarkAllRead(); err != nil {
				return nil, fmt.Errorf("store %s: %w", st.Name, err)
			}
			for _, it := range items {
				read = append(read, it.ULID.String())
			}
		}
		return read, nil
	}
	byStore := map[int][]model.ULID{}
	for _, u := range ulids {
		st, _, err := findItem(stores, u)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(stores, func(x InboxStore) bool { return x.Name == st.Name })
		byStore[i] = append(byStore[i], u)
	}
	for i, us := range byStore {
		if err := stores[i].Box.MarkRead(us); err != nil {
			return nil, err
		}
	}
	for _, u := range ulids {
		read = append(read, u.String())
	}
	return read, nil
}

// InboxRevert writes the value of an overwritten item back.
func (ss *Session) InboxRevert(arg string) (TaskResult, error) {
	u, err := ParseInboxItem(arg)
	if err != nil {
		return TaskResult{}, err
	}
	stores, err := ss.openInbox()
	if err != nil {
		return TaskResult{}, err
	}
	return ss.InboxRevertOf(stores, u)
}

// InboxRevertOf writes the value of the overwritten item u back, found in
// stores which InboxStores opened. It gives Warn what the boxes skipped.
func (ss *Session) InboxRevertOf(stores []InboxStore, u model.ULID) (TaskResult, error) {
	defer ss.WarnInbox(stores)
	st, it, err := findItem(stores, u)
	if err != nil {
		return TaskResult{}, err
	}
	if it.Kind != inbox.Overwritten {
		return TaskResult{}, notRevertible(fmt.Errorf("inbox item %s is %s; only an overwritten item can be reverted", u, it.Kind))
	}
	if it.Task.IsZero() {
		return TaskResult{}, notRevertible(fmt.Errorf("inbox item %s is about %s, which is not a task; only the values of tasks can be reverted", u, it.Path))
	}
	var d struct {
		Field string          `json:"field"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(it.Detail, &d); err != nil {
		return TaskResult{}, fmt.Errorf("inbox item %s does not decode: %w", u, err)
	}
	// The write goes through a session of its own on the store of the item:
	// ss is shared and keeps its store and its writer.
	sub := ss.ForStore(st.Name)
	s, ix, err := sub.Open()
	if err != nil {
		return TaskResult{}, err
	}
	e, ok := ix.Task(it.Task)
	if !ok {
		return TaskResult{}, newErr(KindNotFound, "task.gone", fmt.Errorf("the task of inbox item %s (%s) is gone: %w", u, it.Task, store.ErrNotFound), "id", it.Task.String())
	}
	if err := NotDeleted(ix, e); err != nil {
		return TaskResult{}, err
	}
	hash, err := sub.UpdateCommit(s, store.Op{Name: "revert-overwritten", Task: e.ULID}, func(tx *store.Tx) error {
		if d.Field == "parent" || d.Field == "workspace" {
			return sub.revertMove(tx, ix, e, d.Field, d.Value)
		}
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		if err := revertField(t, d.Field, d.Value, ix.Schema(), sub.ResolveULID(ix)); err != nil {
			return err
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return TaskResult{}, err
	}
	if err := st.Box.MarkRead([]model.ULID{u}); err != nil {
		return TaskResult{}, err
	}
	return TaskResult{Index: ix, ULID: e.ULID, Committed: !hash.IsZero(), Commit: commitHex(hash)}, nil
}

// inboxTaskRef names the task of the item as it is now, or as the item has
// it when the task is gone; nil for an item of no task.
func inboxTaskRef(ix *index.Index, it inbox.Item) *Ref {
	if it.Task.IsZero() {
		return nil
	}
	r := &Ref{ULID: it.Task.String(), ID: it.ID}
	if ix != nil {
		if e, ok := ix.Task(it.Task); ok {
			r.ID, r.Title = e.ID, e.Title
		}
	}
	return r
}

// inboxTarget is what the item is about: the task as "ID (ULID) title", or
// the workspace of a workspace's item.
func inboxTarget(ix *index.Index, it inbox.Item) string {
	if it.Task.IsZero() {
		if w, ok := detailWorkspace(ix, it); ok {
			return "workspace " + w.Name
		}
		return "-"
	}
	if ix != nil {
		if e, ok := ix.Task(it.Task); ok {
			return RefName(ix, e.ULID)
		}
	}
	if it.ID != "" {
		return it.ID + " (" + it.Task.String() + ")"
	}
	return it.Task.String()
}

// repairDetail is the Detail of repaired, revived and prefix_duplicate.
type repairDetail struct {
	Kind      string          `json:"kind"`
	Workspace model.ULID      `json:"workspace"`
	Value     json.RawMessage `json:"value"`
}

func detailWorkspace(ix *index.Index, it inbox.Item) (*model.Workspace, bool) {
	var d repairDetail
	if ix == nil || json.Unmarshal(it.Detail, &d) != nil {
		return nil, false
	}
	return ix.Workspace(d.Workspace)
}

// InboxContent says what happened, by the kind of the item. An item that
// tells who caused it ends with "by <account>"; the ones from trailers and
// records do not know.
func InboxContent(ix *index.Index, it inbox.Item, storeName string, manyStores bool) string {
	by := ""
	if it.Author != "" {
		by = " by " + it.Author
	}
	switch it.Kind {
	case inbox.Overwritten:
		var d struct {
			Field string          `json:"field"`
			Value json.RawMessage `json:"value"`
		}
		_ = json.Unmarshal(it.Detail, &d) // an item that does not decode tells no field
		return fmt.Sprintf("%s: %s was replaced%s", d.Field, compactJSON(d.Value), by)
	case inbox.Renumbered:
		var d struct{ Old, New string }
		_ = json.Unmarshal(it.Detail, &d)
		return fmt.Sprintf("%s was renumbered to %s; #%s in text still points to the task that kept it", d.Old, d.New, d.Old)
	case inbox.Mention:
		where := "the body"
		if hasComment(it) {
			where = "a comment"
		}
		return "mentioned in " + where + by
	case inbox.Assigned:
		return "assigned" + by
	case inbox.Watch:
		var d struct {
			Changed []string `json:"changed"`
		}
		_ = json.Unmarshal(it.Detail, &d)
		if len(d.Changed) > 0 {
			return strings.Join(d.Changed, ", ") + " changed" + by
		}
		return "new comment" + by
	case inbox.Revived:
		return "deleted by you, but changed on the other side, so it is back"
	case inbox.Repaired:
		return RepairedContent(ix, it)
	case inbox.PrefixDuplicate:
		return duplicateContent(ix, it, storeName, manyStores)
	}
	return string(it.Detail)
}

// hasComment reports whether the Detail of a mention names a comment.
func hasComment(it inbox.Item) bool {
	var d struct {
		Comment string `json:"comment"`
	}
	return json.Unmarshal(it.Detail, &d) == nil && d.Comment != ""
}

// RepairedContent describes a repair after a merge by its kind.
func RepairedContent(ix *index.Index, it inbox.Item) string {
	var d repairDetail
	_ = json.Unmarshal(it.Detail, &d) // an item that does not decode tells no kind
	switch d.Kind {
	case "parent_cycle", "parent_deleted":
		// The value tells where the repair took the task from and put it
		// (null is the root); the index only gives names to the ULIDs.
		var m struct{ From, To *model.ULID }
		if json.Unmarshal(d.Value, &m) == nil {
			from := "the root"
			if m.From != nil {
				from = taskName(ix, *m.From)
			}
			if d.Kind == "parent_deleted" {
				return fmt.Sprintf("parent_deleted: moved to the root (its parent %s was deleted)", from)
			}
			to := "root"
			if m.To != nil {
				to = taskName(ix, *m.To)
			}
			return fmt.Sprintf("parent_cycle: moved back to %s (was under %s)", to, from)
		}
	}
	if v := compactJSON(d.Value); v != "" && v != "null" {
		return fmt.Sprintf("%s (was %s)", d.Kind, v)
	}
	return d.Kind
}

// taskName is the task u as "ID (ULID) title", or its ULID when ix lacks it.
func taskName(ix *index.Index, u model.ULID) string {
	if ix != nil {
		if _, ok := ix.Task(u); ok {
			return RefName(ix, u)
		}
	}
	return u.String()
}

// duplicateContent tells of a prefix or a name two workspaces share, and how
// to change it.
func duplicateContent(ix *index.Index, it inbox.Item, storeName string, manyStores bool) string {
	var d repairDetail
	_ = json.Unmarshal(it.Detail, &d)
	ws, prefix := d.Workspace.String(), ""
	if w, ok := detailWorkspace(ix, it); ok {
		ws, prefix = w.Name, w.IDPrefix
	}
	var value string
	_ = json.Unmarshal(d.Value, &value) // a string for the duplicates of one store
	// others names the other workspaces of the store that share the name or
	// the prefix, as the item's kind says.
	others := func(same func(*model.Workspace) bool) string {
		var names []string
		if ix != nil && value != "" {
			for _, w := range ix.Workspaces() {
				if w.ULID != d.Workspace && same(w) {
					names = append(names, "workspace "+w.Name)
				}
			}
		}
		if len(names) == 0 {
			return "another workspace"
		}
		return strings.Join(names, ", ")
	}
	if d.Kind == "name_duplicate" {
		if value == "" {
			value = ws
		}
		text := others(func(w *model.Workspace) bool { return strings.EqualFold(w.Name, value) })
		return fmt.Sprintf("%s is also the name of %s; change it with hedgehug workspace rename %s <new name>", value, text, ws)
	}
	var other struct{ Store, Workspace string }
	var text string
	if json.Unmarshal(d.Value, &other) == nil && other.Workspace != "" {
		text = fmt.Sprintf("workspace %s in store %s", other.Workspace, other.Store)
	} else {
		if value != "" {
			prefix = value
		}
		text = others(func(w *model.Workspace) bool { return strings.EqualFold(w.IDPrefix, value) })
	}
	where := ""
	if manyStores {
		where = " --store " + storeName
	}
	return fmt.Sprintf("%s is also used by %s; change it with hedgehug%s workspace prefix %s <new prefix>", prefix, text, where, ws)
}

// compactJSON is v on one line without spaces between tokens.
func compactJSON(v json.RawMessage) string {
	var b bytes.Buffer
	if json.Compact(&b, v) != nil {
		return string(v)
	}
	return b.String()
}

var errNotRevertible = errors.New("cannot be reverted")

// notRevertible marks err as a value that no write can put back.
func notRevertible(err error) error { return newErr(KindInvalid, "inbox.not_revertable", err) }

// refused says why the fields that no write of a value can restore are
// refused, and what to use.
var refused = map[string]string{
	"*":          "is the whole file, which is not one value",
	"id":         "is given by the store with the workspace prefix; renumbering belongs to hedgehug mv --workspace",
	"former_ids": "keeps the ids the task has had and is never rolled back",
	"created":    "is set by the store when the task is made",
	"created_by": "tells who made the task, and no command changes it",
	"body":       "is merged line by line and has no single value; edit it with hedgehug edit",
}

// revertField puts the value raw, as JSON, back into the field of t, as
// hedgehug set would write it. The parent is the job of revertParent.
func revertField(t *model.Task, field string, raw json.RawMessage, sc *schema.Schema, resolve func(string) (model.ULID, error)) error {
	switch field {
	case "title", "kind", "status":
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		return ApplyAssign(t, Assign{Key: field, Op: "=", Value: v}, sc, resolve)
	case "rank":
		var v model.Rank
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("rank: %w", err)
		}
		t.Rank = v
		return nil
	case "sprint":
		var v *model.ULID
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("sprint: %w", err)
		}
		t.Sprint = v
		return nil
	case "assignees":
		return revertSet(&t.Assignees, field, raw)
	case "watchers":
		return revertSet(&t.Watchers, field, raw)
	case "labels":
		return revertSet(&t.Labels, field, raw)
	case "repos":
		return revertSet(&t.Repos, field, raw)
	case "blocked_by":
		return revertSet(&t.BlockedBy, field, raw)
	case "releases":
		return revertSet(&t.Releases, field, raw)
	case "links":
		return revertLink(t, raw)
	}
	if p, ok := strings.CutPrefix(field, "props."); ok {
		return revertProp(t, p, raw, sc, resolve)
	}
	if why, ok := refused[field]; ok {
		return notRevertible(fmt.Errorf("%s %s: %w", field, why, errNotRevertible))
	}
	return notRevertible(fmt.Errorf("%s is a key this version does not know and no command writes it: %w", field, errNotRevertible))
}

// revertSet puts back the value of a list that is merged as a set: the
// value is one item, which is added unless the list has it, or the whole
// list, which replaces it.
func revertSet[T comparable](dst *[]T, field string, raw json.RawMessage) error {
	var list []T
	if err := json.Unmarshal(raw, &list); err == nil {
		*dst = list
		return nil
	}
	var item T
	if err := json.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	if !slices.Contains(*dst, item) {
		*dst = append(*dst, item)
	}
	return nil
}

// revertMove puts the parent or the workspace that lost back as hedgehug mv
// --parent or mv --workspace writes it: the task goes to the end of its
// siblings, and to another workspace with its subtree and new ids. A parent
// that is null is the root.
func (ss *Session) revertMove(tx *store.Tx, ix *index.Index, e *index.Entry, field string, raw json.RawMessage) error {
	var u *model.ULID
	if err := json.Unmarshal(raw, &u); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	var p *index.Entry
	var w *model.Workspace
	root := false
	switch {
	case field == "parent" && u == nil:
		root = true
	case field == "parent":
		var ok bool
		if p, ok = ix.Task(*u); !ok {
			return newErr(KindNotFound, "task.not_found", fmt.Errorf("parent: %s is not in this store: %w", u, store.ErrNotFound), "input", u.String())
		}
	case u == nil:
		return notRevertible(errors.New("workspace: the value is empty"))
	default:
		var ok bool
		if w, ok = ix.Workspace(*u); !ok {
			return newErr(KindNotFound, "workspace.not_found", fmt.Errorf("workspace: %s is not in this store: %w", u, store.ErrNotFound), "input", u.String())
		}
	}
	_, err := ss.MoveInTx(tx, ix, e, p, w, root, "", "")
	return err
}

// revertLink puts the link raw back in place of the one with its ref.
func revertLink(t *model.Task, raw json.RawMessage) error {
	var l model.Link
	if err := json.Unmarshal(raw, &l); err != nil || l.Ref == "" {
		return notRevertible(fmt.Errorf("links: %s is not a link", raw))
	}
	if i := slices.IndexFunc(t.Links, func(x model.Link) bool { return x.Ref == l.Ref }); i >= 0 {
		t.Links[i] = l
	} else {
		t.Links = append(t.Links, l)
	}
	return nil
}

// revertProp puts the value raw back in the property key. A list becomes
// the property's values; null clears it.
func revertProp(t *model.Task, key string, raw json.RawMessage, sc *schema.Schema, resolve func(string) (model.ULID, error)) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("props.%s: %w", key, err)
	}
	values := []any{v}
	if list, ok := v.([]any); ok {
		values = list
	}
	if len(values) == 0 {
		values = []any{nil}
	}
	for i, x := range values {
		text, err := propText(sc, key, x)
		if err != nil {
			return err
		}
		as := Assign{Key: key, Op: "=", Value: text}
		if i > 0 {
			as.Op = "+="
		}
		if err := ApplyAssign(t, as, sc, resolve); err != nil {
			return err
		}
	}
	return nil
}

// propText is the text hedgehug set reads for the scalar x of the property
// key; "" for null, which clears it.
func propText(sc *schema.Schema, key string, x any) (string, error) {
	switch v := x.(type) {
	case nil:
		return "", nil
	case string:
		// A date reads back from the file as a time.
		if p, ok := sc.Prop(key); ok && p.Type == schema.TypeDate {
			if tm, err := time.Parse(time.RFC3339, v); err == nil {
				return tm.Format(time.DateOnly), nil
			}
		}
		return v, nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	}
	return "", notRevertible(fmt.Errorf("props.%s: %v is not a value hedgehug set reads; %w", key, x, errNotRevertible))
}
