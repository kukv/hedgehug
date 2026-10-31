package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func (a *app) inboxCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Show what happened to you: mentions, assignments, overwritten values, renumbered ids",
		Args:  asUsage(cobra.NoArgs),
		RunE:  func(_ *cobra.Command, _ []string) error { return a.inboxList(all) },
	}
	cmd.Flags().BoolVar(&all, "all", false, "show the items you have read too")

	var readAll bool
	read := &cobra.Command{
		Use:   "read <ULID>... | --all",
		Short: "Mark items read, by their full ULIDs",
		Args: func(_ *cobra.Command, args []string) error {
			switch {
			case readAll && len(args) > 0:
				return usageErrorf("give the ULIDs or --all, not both")
			case !readAll && len(args) == 0:
				return usageErrorf("give the ULIDs of the items, or --all")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error { return a.inboxRead(args, readAll) },
	}
	read.Flags().BoolVar(&readAll, "all", false, "mark every item read")

	revert := &cobra.Command{
		Use:   "revert <ULID>",
		Short: "Write your overwritten value back",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.inboxRevert(args[0]) },
	}
	cmd.AddCommand(read, revert)
	return cmd
}

// inboxStore is a store whose inbox a command works on.
type inboxStore struct {
	name string
	s    *store.Store
	box  *inbox.Box
}

// inboxStores opens the stores the command works on, after the inbox of each
// has taken in what reached the store since it last looked and the merge
// driver's records. What the boxes skipped goes to stderr.
func (a *app) inboxStores() ([]inboxStore, error) {
	c, _, err := a.ss.Config()
	if err != nil {
		return nil, err
	}
	targets, err := a.syncTargets(c, true)
	if err != nil {
		return nil, err
	}
	if len(c.Accounts) == 0 {
		_, _ = fmt.Fprintln(a.stderr, "hedgehug: warning: accounts are not set up in config.yaml; mentions and assignments cannot be told")
	}
	var out []inboxStore
	for _, e := range targets {
		s, err := store.Open(e.Path)
		if err != nil {
			return nil, fmt.Errorf("store %s: %w", e.Name, err)
		}
		box := inbox.Open(s)
		if len(c.Accounts) > 0 {
			head, err := s.Repo().Head()
			if err == nil {
				_, err = box.Catch(c.Me(), head)
			}
			if err != nil {
				return nil, fmt.Errorf("store %s: %w", e.Name, err)
			}
		}
		out = append(out, inboxStore{e.Name, s, box})
	}
	return out, nil
}

// warnInbox prints what the boxes skipped, once all their reading is done.
func (a *app) warnInbox(stores []inboxStore) {
	for _, st := range stores {
		for _, w := range st.box.Warnings() {
			_, _ = fmt.Fprintf(a.stderr, "warning: %s: %s\n", st.name, w)
		}
	}
}

// inboxItemJSON is an item as --json prints it: the task as a reference to
// the task as it is now.
type inboxItemJSON struct {
	inbox.Item
	Store string   `json:"store"`
	Task  *ops.Ref `json:"task"`
}

func (a *app) inboxList(all bool) error {
	stores, err := a.inboxStores()
	if err != nil {
		return err
	}
	defer a.warnInbox(stores)
	type row struct {
		store string
		item  inbox.Item
		ix    *index.Index
	}
	var rows []row
	for _, st := range stores {
		items, err := st.box.List(all)
		if err != nil {
			return fmt.Errorf("store %s: %w", st.name, err)
		}
		if len(items) == 0 {
			continue
		}
		ix, err := index.Open(st.s)
		if err != nil {
			_, _ = fmt.Fprintf(a.stderr, "warning: %s: read the index, so titles are not shown: %v\n", st.name, err)
			ix = nil
		}
		for _, it := range items {
			rows = append(rows, row{st.name, it, ix})
		}
	}
	slices.SortStableFunc(rows, func(x, y row) int {
		if c := y.item.Time.Compare(x.item.Time); c != 0 {
			return c
		}
		return strings.Compare(y.item.ULID.String(), x.item.ULID.String())
	})
	if a.jsonOut {
		out := make([]inboxItemJSON, len(rows))
		for i, r := range rows {
			out[i] = inboxItemJSON{Item: r.item, Store: r.store, Task: inboxTaskRef(r.ix, r.item)}
		}
		return a.printJSON(out)
	}
	if len(rows) == 0 {
		msg := "No unread items"
		if all {
			msg = "No items"
		}
		_, err := fmt.Fprintln(a.stdout, msg)
		return err
	}
	st := a.style()
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s  %-16s  %s  %s\n", st.id(r.item.ULID.String()), r.item.Kind, inboxTarget(r.ix, r.item), inboxContent(r.ix, r.item, r.store, len(stores) > 1))
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// inboxTaskRef names the task of the item as it is now, or as the item has
// it when the task is gone; nil for an item of no task.
func inboxTaskRef(ix *index.Index, it inbox.Item) *ops.Ref {
	if it.Task.IsZero() {
		return nil
	}
	r := &ops.Ref{ULID: it.Task.String(), ID: it.ID}
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
			return ops.RefName(ix, e.ULID)
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

// inboxContent says what happened, by the kind of the item. An item that
// tells who caused it ends with "by <account>"; the ones from trailers and
// records do not know.
func inboxContent(ix *index.Index, it inbox.Item, storeName string, manyStores bool) string {
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
		return repairedContent(ix, it)
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

// repairedContent describes a repair after a merge by its kind.
func repairedContent(ix *index.Index, it inbox.Item) string {
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
			return ops.RefName(ix, u)
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

// parseItemULID reads the full ULID of an item; a short one is a usage error.
func parseItemULID(arg string) (model.ULID, error) {
	u, err := model.ParseULID(arg)
	if err != nil {
		return model.ULID{}, usageErrorf("%q: give the full ULID of the item", arg)
	}
	return u, nil
}

// findItem finds the store whose inbox holds the item u.
func findItem(stores []inboxStore, u model.ULID) (inboxStore, inbox.Item, error) {
	for _, st := range stores {
		it, ok, err := st.box.Item(u)
		if err != nil {
			return inboxStore{}, inbox.Item{}, fmt.Errorf("store %s: %w", st.name, err)
		}
		if ok {
			return st, it, nil
		}
	}
	return inboxStore{}, inbox.Item{}, fmt.Errorf("inbox item %s: %w", u, store.ErrNotFound)
}

func (a *app) inboxRead(args []string, all bool) error {
	var ulids []model.ULID
	for _, arg := range args {
		u, err := parseItemULID(arg)
		if err != nil {
			return err
		}
		if !slices.Contains(ulids, u) {
			ulids = append(ulids, u)
		}
	}
	stores, err := a.inboxStores()
	if err != nil {
		return err
	}
	defer a.warnInbox(stores)
	read := []string{}
	if all {
		for _, st := range stores {
			items, err := st.box.List(false)
			if err != nil {
				return fmt.Errorf("store %s: %w", st.name, err)
			}
			if err := st.box.MarkAllRead(); err != nil {
				return fmt.Errorf("store %s: %w", st.name, err)
			}
			for _, it := range items {
				read = append(read, it.ULID.String())
			}
		}
	} else {
		byStore := map[int][]model.ULID{}
		for _, u := range ulids {
			st, _, err := findItem(stores, u)
			if err != nil {
				return err
			}
			i := slices.IndexFunc(stores, func(x inboxStore) bool { return x.name == st.name })
			byStore[i] = append(byStore[i], u)
		}
		for i, us := range byStore {
			if err := stores[i].box.MarkRead(us); err != nil {
				return err
			}
		}
		for _, u := range ulids {
			read = append(read, u.String())
		}
	}
	if a.jsonOut {
		return a.printJSON(map[string][]string{"read": read})
	}
	_, err = fmt.Fprintf(a.stdout, "Marked %d %s read\n", len(read), plural(len(read), "item", "items"))
	return err
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (a *app) inboxRevert(arg string) error {
	u, err := parseItemULID(arg)
	if err != nil {
		return err
	}
	stores, err := a.inboxStores()
	if err != nil {
		return err
	}
	defer a.warnInbox(stores)
	st, it, err := findItem(stores, u)
	if err != nil {
		return err
	}
	if it.Kind != inbox.Overwritten {
		return fmt.Errorf("inbox item %s is %s; only an overwritten item can be reverted", u, it.Kind)
	}
	if it.Task.IsZero() {
		return fmt.Errorf("inbox item %s is about %s, which is not a task; only the values of tasks can be reverted", u, it.Path)
	}
	var d struct {
		Field string          `json:"field"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(it.Detail, &d); err != nil {
		return fmt.Errorf("inbox item %s does not decode: %w", u, err)
	}
	a.ss.StoreName = st.name
	s, ix, err := a.ss.Open()
	if err != nil {
		return err
	}
	e, ok := ix.Task(it.Task)
	if !ok {
		return fmt.Errorf("the task of inbox item %s (%s) is gone: %w", u, it.Task, store.ErrNotFound)
	}
	if err := ops.NotDeleted(ix, e); err != nil {
		return err
	}
	committed, err := a.ss.Update(s, store.Op{Name: "revert-overwritten", Task: e.ULID}, func(tx *store.Tx) error {
		if d.Field == "parent" || d.Field == "workspace" {
			return a.revertMove(tx, ix, e, d.Field, d.Value)
		}
		t, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		if err := revertField(t, d.Field, d.Value, ix.Schema(), a.ss.ResolveULID(ix)); err != nil {
			return err
		}
		return tx.PutTask(t)
	})
	if err != nil {
		return err
	}
	if err := st.box.MarkRead([]model.ULID{u}); err != nil {
		return err
	}
	return a.printTask(ix, e.ULID, verb(committed, "Reverted"))
}

var errNotRevertible = errors.New("cannot be reverted")

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
		return ops.ApplyAssign(t, ops.Assign{Key: field, Op: "=", Value: v}, sc, resolve)
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
		return fmt.Errorf("%s %s: %w", field, why, errNotRevertible)
	}
	return fmt.Errorf("%s is a key this version does not know and no command writes it: %w", field, errNotRevertible)
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
func (a *app) revertMove(tx *store.Tx, ix *index.Index, e *index.Entry, field string, raw json.RawMessage) error {
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
			return fmt.Errorf("parent: %s is not in this store: %w", u, store.ErrNotFound)
		}
	case u == nil:
		return errors.New("workspace: the value is empty")
	default:
		var ok bool
		if w, ok = ix.Workspace(*u); !ok {
			return fmt.Errorf("workspace: %s is not in this store: %w", u, store.ErrNotFound)
		}
	}
	_, err := a.ss.MoveInTx(tx, ix, e, p, w, root, "", "")
	return err
}

// revertLink puts the link raw back in place of the one with its ref.
func revertLink(t *model.Task, raw json.RawMessage) error {
	var l model.Link
	if err := json.Unmarshal(raw, &l); err != nil || l.Ref == "" {
		return fmt.Errorf("links: %s is not a link", raw)
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
		as := ops.Assign{Key: key, Op: "=", Value: text}
		if i > 0 {
			as.Op = "+="
		}
		if err := ops.ApplyAssign(t, as, sc, resolve); err != nil {
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
	return "", fmt.Errorf("props.%s: %v is not a value hedgehug set reads; %w", key, x, errNotRevertible)
}
