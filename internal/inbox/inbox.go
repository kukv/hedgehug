// Package inbox keeps the notices of one store on this machine: mentions,
// assignments and changes to watched tasks that a sync brought, values a
// merge overwrote and what the repair after a merge changed.
package inbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

type Kind string

const (
	Mention         Kind = "mention"
	Assigned        Kind = "assigned"
	Watch           Kind = "watch"
	Overwritten     Kind = "overwritten"
	Revived         Kind = "revived"
	Repaired        Kind = "repaired"
	Renumbered      Kind = "renumbered"
	PrefixDuplicate Kind = "prefix_duplicate"
)

// Item is one notice.
type Item struct {
	ULID   model.ULID `json:"ulid"`
	Kind   Kind       `json:"kind"`
	Task   model.ULID `json:"task,omitzero"`  // zero for a workspace item
	ID     string     `json:"id,omitempty"`   // the task's id when the item was made
	Path   string     `json:"path,omitempty"` // the file, for overwritten and revived
	Commit string     `json:"commit"`
	// Author is who caused it: the writer of the commit for mention,
	// assigned and watch. It is empty for the items of trailers, records
	// and the notices of a sync, which do not tell who won or caused it
	// (the merge or repair commit's writer only merged).
	Author string    `json:"author"`
	To     string    `json:"to"` // the account told, one of me as the commit or record names it
	Time   time.Time `json:"time"`
	// Detail is by kind: overwritten {"field","value"}; renumbered
	// {"old","new"}; repaired, revived and prefix_duplicate
	// {"kind","workspace","value"} (workspace for a workspace's repair);
	// watch {"changed":[...]} or {"comment"}; mention {"comment"} when a
	// comment mentions.
	Detail json.RawMessage `json:"detail,omitempty"`
}

// Record is what the merge driver writes for one overwritten value or notice.
type Record struct {
	Kind   Kind      `json:"kind,omitempty"` // empty means overwritten
	Path   string    `json:"path"`
	Field  string    `json:"field,omitempty"`
	Value  string    `json:"value,omitempty"`
	Author string    `json:"author"`
	Commit string    `json:"commit"`
	Time   time.Time `json:"time"`
}

const (
	itemsFile  = "inbox.jsonl"
	readFile   = "inbox-read.json"
	markFile   = "inbox-head"
	lockFile   = "inbox.lock"
	recordsDir = "overwritten"
	lockWait   = 10 * time.Second
)

// Box is the inbox of one store on this machine.
type Box struct {
	s        *store.Store
	warnings []string
}

func Open(s *store.Store) *Box { return &Box{s: s} }

// Warnings returns what the box skipped while reading: lines of
// inbox.jsonl and merge driver records that do not decode. They stay in
// place.
func (b *Box) Warnings() []string { return b.warnings }

func (b *Box) warn(w string) {
	if !slices.Contains(b.warnings, w) {
		b.warnings = append(b.warnings, w)
	}
}

// Catch adds the items of the commits from the last Catch to HEAD, and
// then the merge driver's records, and returns the items added. A store
// that has not been caught yet starts at since: the HEAD before a sync, or
// HEAD itself. The commits are read without the lock; the commit reached
// is kept only once its items are in, and only when no other Catch moved
// it meanwhile (the items of both are in either way, once each), so a
// failed Catch is made up by the next.
func (b *Box) Catch(me []string, since plumbing.Hash) ([]Item, error) {
	head, err := b.s.Repo().Head()
	if err != nil {
		return nil, err
	}
	was := b.markText()
	from := since
	if m, ok := b.mark(was); ok {
		from = m
	}
	items, err := Collect(b.s, from, head, me)
	if err != nil {
		return nil, err
	}
	var added []Item
	err = b.locked(func() error {
		var err error
		if added, err = b.add(items); err != nil {
			return err
		}
		if b.markText() == was {
			if err := b.s.WriteLocal(markFile, []byte(head.String()+"\n")); err != nil {
				return err
			}
		}
		more, err := b.importRecords(me)
		added = append(added, more...)
		return err
	})
	return added, err
}

// Start makes at the commit a store's first Catch starts from, unless one
// is set already; a sync sets it at the HEAD it starts from, so a Catch
// that runs meanwhile does not start after the commits the sync brings.
func (b *Box) Start(at plumbing.Hash) error {
	return b.locked(func() error {
		if b.markText() != "" {
			return nil
		}
		return b.s.WriteLocal(markFile, []byte(at.String()+"\n"))
	})
}

// markText is what inbox-head holds; "" when it is missing.
func (b *Box) markText() string {
	data, err := b.s.ReadLocal(markFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// mark is the commit that inbox-head, as read, names; ok is false when it
// is missing, or with a warning when it names no commit of the repository.
func (b *Box) mark(data string) (plumbing.Hash, bool) {
	if data == "" {
		return plumbing.ZeroHash, false
	}
	if !plumbing.IsHash(data) {
		b.warn(fmt.Sprintf(".hedgehug/local/%s: %q is no commit; reading from the given start", markFile, data))
		return plumbing.ZeroHash, false
	}
	h := plumbing.NewHash(data)
	if _, err := b.s.Repo().TreeOf(h); err != nil {
		b.warn(fmt.Sprintf(".hedgehug/local/%s: commit %s does not read (%v); reading from the given start", markFile, data, err))
		return plumbing.ZeroHash, false
	}
	return h, true
}

// Add adds the items that the inbox does not have yet, by key: the same
// overwritten value, the same duplicate prefix or name, the same repair,
// or the same kind of notice of the same commit and task. An item without
// a ULID is given one. It returns the ones added.
func (b *Box) Add(items []Item) ([]Item, error) {
	var added []Item
	err := b.locked(func() error {
		var err error
		added, err = b.add(items)
		return err
	})
	return added, err
}

// add is Add for a caller that holds the lock. It appends to inbox.jsonl,
// keeping the lines that do not decode.
func (b *Box) add(items []Item) ([]Item, error) {
	have, data, err := b.items()
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, it := range have {
		keys[key(it)] = true
	}
	var added []Item
	for _, it := range items {
		k := key(it)
		if keys[k] {
			continue
		}
		keys[k] = true
		if it.ULID.IsZero() {
			it.ULID = model.NewULID()
		}
		added = append(added, it)
	}
	if len(added) == 0 {
		return nil, nil
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	for _, it := range added {
		line, err := json.Marshal(it)
		if err != nil {
			return nil, err
		}
		data = append(append(data, line...), '\n')
	}
	if err := b.s.WriteLocal(itemsFile, data); err != nil {
		return nil, err
	}
	return added, nil
}

// List returns the items, newest first; only the unread ones unless all.
func (b *Box) List(all bool) ([]Item, error) {
	items, _, err := b.items()
	if err != nil {
		return nil, err
	}
	read, err := b.read()
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, it := range items {
		if all || !read[it.ULID] {
			out = append(out, it)
		}
	}
	slices.SortStableFunc(out, func(x, y Item) int {
		if c := y.Time.Compare(x.Time); c != 0 {
			return c
		}
		return strings.Compare(y.ULID.String(), x.ULID.String())
	})
	return out, nil
}

// MarkRead marks the items read. A ULID of no item is store.ErrNotFound,
// and nothing is marked.
func (b *Box) MarkRead(ulids []model.ULID) error {
	return b.locked(func() error {
		items, _, err := b.items()
		if err != nil {
			return err
		}
		for _, u := range ulids {
			if !slices.ContainsFunc(items, func(it Item) bool { return it.ULID == u }) {
				return fmt.Errorf("inbox item %s: %w", u, store.ErrNotFound)
			}
		}
		return b.markRead(items, ulids)
	})
}

// MarkAllRead marks every item read.
func (b *Box) MarkAllRead() error {
	return b.locked(func() error {
		items, _, err := b.items()
		if err != nil {
			return err
		}
		ulids := make([]model.ULID, len(items))
		for i, it := range items {
			ulids[i] = it.ULID
		}
		return b.markRead(items, ulids)
	})
}

// Item returns the item ulid; ok is false when there is none.
func (b *Box) Item(ulid model.ULID) (Item, bool, error) {
	items, _, err := b.items()
	if err != nil {
		return Item{}, false, err
	}
	for _, it := range items {
		if it.ULID == ulid {
			return it, true, nil
		}
	}
	return Item{}, false, nil
}

// Import adds the merge driver's records in .hedgehug/local/overwritten/
// and removes the files. The records whose author is not one of me are
// removed too: the driver writes every value that lost, whoever wrote it.
// A record that does not decode is left in place, with a warning.
func (b *Box) Import(me []string) error {
	return b.locked(func() error {
		_, err := b.importRecords(me)
		return err
	})
}

func (b *Box) importRecords(me []string) ([]Item, error) {
	names, err := b.s.LocalFiles(recordsDir)
	if err != nil {
		return nil, err
	}
	var items []Item
	var done []string
	for _, n := range names {
		if !strings.HasSuffix(n, ".json") {
			continue
		}
		name := recordsDir + "/" + n
		data, err := b.s.ReadLocal(name)
		if err != nil {
			return nil, err
		}
		it, err := b.fromRecord(data)
		if err != nil {
			b.warn(fmt.Sprintf(".hedgehug/local/%s: skipped: %v", name, err))
			continue
		}
		switch b.fate(it.Commit) {
		case recordPending:
			continue
		case recordAborted:
			done = append(done, name)
			continue
		}
		if _, ok := isMe(me, it.To); ok {
			items = append(items, it)
		}
		done = append(done, name)
	}
	added, err := b.add(items)
	if err != nil {
		return nil, err
	}
	for _, name := range done {
		if err := b.s.RemoveLocal(name); err != nil {
			return added, err
		}
	}
	return added, nil
}

type recordFate int

const (
	recordSettled recordFate = iota // the merge it was written for is done, or it names none
	recordPending                   // that merge is under way
	recordAborted                   // that merge was given up: its commit never reached HEAD
)

// fate tells what became of the merge the driver wrote a record for. The
// record names the commit being merged in; it is not in HEAD's history once
// the merge has been aborted. A record without a commit (a rebase, a
// cherry-pick) has no merge to wait for. A commit that cannot be judged (an
// error reading the history) keeps its record: dropping is irreversible. After
// a squash merge the commit never becomes an ancestor of HEAD, so its records
// are dropped as if the merge were aborted.
func (b *Box) fate(commit string) recordFate {
	if !plumbing.IsHash(commit) {
		return recordSettled
	}
	r := b.s.Repo()
	if _, merging, err := r.MergeHead(); err != nil || merging {
		return recordPending
	}
	head, err := r.Head()
	if err != nil {
		return recordPending
	}
	in, err := r.IsAncestor(plumbing.NewHash(commit), head)
	switch {
	case err != nil:
		return recordPending
	case !in:
		return recordAborted
	}
	return recordSettled
}

// RecordsPath is the directory of the merge driver's records, relative to
// the store.
const RecordsPath = store.LocalDir + "/" + recordsDir

// CheckRecord returns why a merge driver record cannot be imported, or nil.
func CheckRecord(data []byte) error {
	_, err := decodeRecord(data)
	return err
}

// decodeRecord reads a merge driver's record and checks that the inbox can
// make an item of it.
func decodeRecord(data []byte) (Record, error) {
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	if r.Path == "" {
		return r, errors.New("the record has no path")
	}
	switch r.Kind {
	case "":
		if r.Field == "" {
			return r, errors.New("the record has no field")
		}
	case Revived:
	default:
		return r, fmt.Errorf("unknown kind %q", r.Kind)
	}
	return r, nil
}

// fromRecord makes the item of a merge driver's record. The id is the
// task's in the work tree, when it reads.
func (b *Box) fromRecord(data []byte) (Item, error) {
	r, err := decodeRecord(data)
	if err != nil {
		return Item{}, err
	}
	it := Item{Kind: r.Kind, Task: taskOf(r.Path), Path: r.Path, Commit: r.Commit, To: r.Author, Time: r.Time}
	switch r.Kind {
	case "":
		it.Kind, it.Detail = Overwritten, overwrittenDetail(r.Field, r.Value)
	case Revived:
		it.Detail = mustJSON(repairDetail{Kind: string(Revived), Value: jsonOrNull(r.Value)})
	}
	if !it.Task.IsZero() {
		if t, err := b.s.Task(it.Task); err == nil {
			it.ID = t.ID
		}
	}
	return it, nil
}

// overwrittenDetail holds the field and the value, which is JSON; a value
// that is not is kept as a string.
func overwrittenDetail(field, value string) json.RawMessage {
	v := json.RawMessage(value)
	if !json.Valid(v) {
		v = mustJSON(value)
	}
	return mustJSON(struct {
		Field string          `json:"field"`
		Value json.RawMessage `json:"value"`
	}{field, v})
}

// repairDetail is the Detail of repaired, revived and prefix_duplicate.
type repairDetail struct {
	Kind      string          `json:"kind"`
	Workspace model.ULID      `json:"workspace,omitzero"`
	Value     json.RawMessage `json:"value"`
}

// jsonOrNull is v when it is JSON, else null (as for the - of a trailer).
func jsonOrNull(v string) json.RawMessage {
	if json.Valid([]byte(v)) {
		return json.RawMessage(v)
	}
	return json.RawMessage("null")
}

// mustJSON encodes what always encodes: strings, ULIDs and valid JSON.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// key is what two items that tell the same thing share. Only mention,
// assigned and watch are told per commit: an overwritten value, a repair
// or a duplicate prefix that two merge commits carry is one item.
func key(it Item) string {
	var k []any
	switch it.Kind {
	case Overwritten:
		var d struct {
			Field string
			Value json.RawMessage
		}
		_ = json.Unmarshal(it.Detail, &d) // a detail that does not decode keys as empty
		k = []any{it.Kind, it.Path, d.Field, compact(d.Value), strings.ToLower(it.To)}
	case PrefixDuplicate:
		var d repairDetail
		_ = json.Unmarshal(it.Detail, &d)
		k = []any{it.Kind, d.Workspace.String(), compact(d.Value), d.Kind}
	case Renumbered:
		k = []any{it.Kind, it.Task.String(), it.Path, "id_collision", compact(it.Detail)}
	case Repaired, Revived:
		var d repairDetail
		_ = json.Unmarshal(it.Detail, &d)
		k = []any{it.Kind, it.Task.String(), it.Path, d.Kind, d.Workspace.String(), compact(d.Value)}
	default:
		k = []any{it.Kind, it.Commit, it.Task.String()}
	}
	return string(mustJSON(k))
}

func compact(v json.RawMessage) string {
	var buf bytes.Buffer
	if json.Compact(&buf, v) != nil {
		return string(v)
	}
	return buf.String()
}

func (b *Box) locked(fn func() error) error {
	return b.s.WithLocalLock(lockFile, lockWait, fn)
}

// items reads inbox.jsonl, one item a line, and returns its bytes too. A
// line that does not decode is skipped, with a warning.
func (b *Box) items() ([]Item, []byte, error) {
	data, err := b.s.ReadLocal(itemsFile)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var out []Item
	for i, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var it Item
		if err := json.Unmarshal(line, &it); err != nil {
			b.warn(fmt.Sprintf(".hedgehug/local/%s:%d: skipped: %v", itemsFile, i+1, err))
			continue
		}
		out = append(out, it)
	}
	return out, data, nil
}

// read reads inbox-read.json, the set of the items read.
func (b *Box) read() (map[model.ULID]bool, error) {
	data, err := b.s.ReadLocal(readFile)
	if errors.Is(err, store.ErrNotFound) {
		return map[model.ULID]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var ulids []model.ULID
	if err := json.Unmarshal(data, &ulids); err != nil {
		return nil, fmt.Errorf(".hedgehug/local/%s: %w", readFile, err)
	}
	out := make(map[model.ULID]bool, len(ulids))
	for _, u := range ulids {
		out[u] = true
	}
	return out, nil
}

// markRead adds ulids to the set of the items read, which keeps only the
// ULIDs of items; the caller holds the lock.
func (b *Box) markRead(items []Item, ulids []model.ULID) error {
	read, err := b.read()
	if err != nil {
		return err
	}
	for _, u := range ulids {
		read[u] = true
	}
	var keep []model.ULID
	for _, it := range items {
		if read[it.ULID] {
			keep = append(keep, it.ULID)
			delete(read, it.ULID)
		}
	}
	slices.SortFunc(keep, func(x, y model.ULID) int { return strings.Compare(x.String(), y.String()) })
	data, err := json.Marshal(keep)
	if err != nil {
		return err
	}
	return b.s.WriteLocal(readFile, append(data, '\n'))
}
