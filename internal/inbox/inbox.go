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
	Path   string     `json:"path,omitempty"` // the file, for overwritten and revived
	Commit string     `json:"commit"`
	// Author is who caused it: the writer of the commit for mention,
	// assigned and watch; the account it tells, as the trailer or the
	// record names it, for the others (for overwritten: whose value lost).
	Author string    `json:"author"`
	Time   time.Time `json:"time"`
	// Detail is by kind: overwritten {"field","value"}; renumbered
	// {"old","new"}; repaired and prefix_duplicate {"kind","workspace","value"}
	// (workspace for a workspace's repair); watch {"changed":[...]} or
	// {"comment"}; mention {"comment"} when a comment mentions.
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
	lockFile   = "inbox.lock"
	recordsDir = "overwritten"
	lockWait   = 10 * time.Second
)

// Box is the inbox of one store on this machine.
type Box struct{ s *store.Store }

func Open(s *store.Store) *Box { return &Box{s} }

// Add adds the items that the inbox does not have yet: by key, the same
// overwritten value, the same duplicate prefix or name, or the same kind of
// notice of the same commit and task. An item without a ULID is given one.
// It returns the ones added.
func (b *Box) Add(items []Item) ([]Item, error) {
	var added []Item
	err := b.locked(func() error {
		have, err := b.items()
		if err != nil {
			return err
		}
		keys := map[string]bool{}
		for _, it := range have {
			keys[key(it)] = true
		}
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
			return nil
		}
		var buf bytes.Buffer
		for _, it := range append(have, added...) {
			line, err := json.Marshal(it)
			if err != nil {
				return err
			}
			buf.Write(append(line, '\n'))
		}
		return b.s.WriteLocal(itemsFile, buf.Bytes())
	})
	if err != nil {
		return nil, err
	}
	return added, nil
}

// List returns the items, newest first; only the unread ones unless all.
func (b *Box) List(all bool) ([]Item, error) {
	items, err := b.items()
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
		items, err := b.items()
		if err != nil {
			return err
		}
		for _, u := range ulids {
			if !slices.ContainsFunc(items, func(it Item) bool { return it.ULID == u }) {
				return fmt.Errorf("inbox item %s: %w", u, store.ErrNotFound)
			}
		}
		return b.markRead(ulids)
	})
}

// MarkAllRead marks every item read.
func (b *Box) MarkAllRead() error {
	return b.locked(func() error {
		items, err := b.items()
		if err != nil {
			return err
		}
		ulids := make([]model.ULID, len(items))
		for i, it := range items {
			ulids[i] = it.ULID
		}
		return b.markRead(ulids)
	})
}

// Item returns the item ulid; ok is false when there is none.
func (b *Box) Item(ulid model.ULID) (Item, bool, error) {
	items, err := b.items()
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
func (b *Box) Import(me []string) error {
	names, err := b.s.LocalFiles(recordsDir)
	if err != nil {
		return err
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
			return err
		}
		var r Record
		if err := json.Unmarshal(data, &r); err != nil {
			return fmt.Errorf(".hedgehug/local/%s: %w", name, err)
		}
		if slices.Contains(me, r.Author) {
			items = append(items, fromRecord(r))
		}
		done = append(done, name)
	}
	if _, err := b.Add(items); err != nil {
		return err
	}
	for _, name := range done {
		if err := b.s.RemoveLocal(name); err != nil {
			return err
		}
	}
	return nil
}

func fromRecord(r Record) Item {
	it := Item{Kind: r.Kind, Task: taskOf(r.Path), Path: r.Path, Commit: r.Commit, Author: r.Author, Time: r.Time}
	if r.Kind == "" {
		it.Kind = Overwritten
		it.Detail = overwrittenDetail(r.Field, r.Value)
	}
	return it
}

// overwrittenDetail holds the field and the value, which is JSON; a value
// that is not is kept as a string.
func overwrittenDetail(field, value string) json.RawMessage {
	v := json.RawMessage(value)
	if !json.Valid(v) {
		v, _ = json.Marshal(value) // a string always encodes
	}
	d, _ := json.Marshal(struct {
		Field string          `json:"field"`
		Value json.RawMessage `json:"value"`
	}{field, v}) // valid JSON always encodes
	return d
}

// repairDetail is the Detail of repaired and prefix_duplicate.
type repairDetail struct {
	Kind      string          `json:"kind"`
	Workspace model.ULID      `json:"workspace,omitzero"`
	Value     json.RawMessage `json:"value"`
}

// key is what two items that tell the same thing share.
func key(it Item) string {
	var k []any
	switch it.Kind {
	case Overwritten:
		var d struct {
			Field string
			Value json.RawMessage
		}
		_ = json.Unmarshal(it.Detail, &d) // a detail that does not decode keys as empty
		k = []any{it.Kind, it.Path, d.Field, compact(d.Value), it.Author}
	case PrefixDuplicate:
		var d repairDetail
		_ = json.Unmarshal(it.Detail, &d)
		k = []any{it.Kind, d.Workspace.String(), compact(d.Value), d.Kind}
	default:
		// The path tells apart revived files that are not tasks, and the
		// repair's kind two repairs of one task in one commit.
		var d repairDetail
		if it.Kind == Repaired {
			_ = json.Unmarshal(it.Detail, &d)
		}
		k = []any{it.Kind, it.Commit, it.Task.String(), it.Path, d.Kind}
	}
	b, _ := json.Marshal(k) // strings always encode
	return string(b)
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

// items reads inbox.jsonl, one item a line.
func (b *Box) items() ([]Item, error) {
	data, err := b.s.ReadLocal(itemsFile)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Item
	for i, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var it Item
		if err := json.Unmarshal(line, &it); err != nil {
			return nil, fmt.Errorf(".hedgehug/local/%s:%d: %w", itemsFile, i+1, err)
		}
		out = append(out, it)
	}
	return out, nil
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

// markRead adds ulids to the set of the items read; the caller holds the lock.
func (b *Box) markRead(ulids []model.ULID) error {
	read, err := b.read()
	if err != nil {
		return err
	}
	for _, u := range ulids {
		read[u] = true
	}
	all := make([]model.ULID, 0, len(read))
	for u := range read {
		all = append(all, u)
	}
	slices.SortFunc(all, func(x, y model.ULID) int { return strings.Compare(x.String(), y.String()) })
	data, err := json.Marshal(all)
	if err != nil {
		return err
	}
	return b.s.WriteLocal(readFile, append(data, '\n'))
}
