package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// SyncReport is what a sync of one store did, as --json prints it.
type SyncReport struct {
	Store       string        `json:"store"`
	NoRemote    bool          `json:"no_remote,omitempty"`
	Error       string        `json:"error,omitempty"` // Sync sets it to Err's message; the CLI puts its own words
	Merges      int           `json:"merges"`
	Pushed      []string      `json:"pushed"`
	Overwritten []Overwritten `json:"overwritten"`
	Repaired    []Repaired    `json:"repaired"`
	Invalid     []string      `json:"invalid"` // ULIDs of the tasks with problems now
	Notices     []inbox.Item  `json:"notices"` // what the sync added to the inbox
	Warnings    []string      `json:"warnings"`
	// Unpushable is the number of commits not pushed because the store is
	// read-only.
	Unpushable int   `json:"unpushable,omitempty"`
	Err        error `json:"-"` // why the sync failed; the report holds what is known
}

// JSONValue is v as JSON when it is, else as a string: the values of the
// sync are one line of JSON, or "-" for none.
func JSONValue(v string) json.RawMessage {
	if json.Valid([]byte(v)) {
		return json.RawMessage(v)
	}
	b, _ := json.Marshal(v) // a string always encodes
	return b
}

// TimePtr is &t, or nil for the zero time.
func TimePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// FinishRebuild completes a swap that stopped, or removes what a rebuild
// left, for the store e, before it is opened. Nothing left is nothing done.
// While the store is there, the leftover is touched only under sync.lock: a
// rebuild holds it, so a rebuild in the making is not taken for a leftover.
func (ss *Session) FinishRebuild(e config.StoreEntry) error {
	l, err := store.FindLeftover(e.Path)
	if err != nil || l.None() {
		return err
	}
	s, err := store.Open(e.Path)
	if err != nil {
		// No store (or a broken one) to lock: the swap is finished as it is.
		if _, serr := os.Lstat(e.Path); errors.Is(serr, fs.ErrNotExist) {
			return ss.finishLeftover(e, l)
		}
		return nil // the store's own problem; the sync tells of it
	}
	defer func() { _ = s.Repo().Close() }()
	return s.WithLocalLock("sync.lock", SyncLockWait, func() error {
		l, err := store.FindLeftover(e.Path)
		if err != nil || l.None() {
			return err
		}
		return ss.finishLeftover(e, l)
	})
}

// SyncTargets are the stores an operation works on: the one StoreName
// names, or all that have a remote, or with all every store.
func (ss *Session) SyncTargets(c *config.Config, all bool) ([]config.StoreEntry, error) {
	if ss.StoreName != "" {
		e, err := c.Select(ss.StoreName)
		if err != nil && !errors.Is(err, config.ErrNoStores) {
			return nil, NewErr(KindNotFound, "store.not_found", err, "name", ss.StoreName)
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

// RepairedContent describes a repair after a merge by its kind.
func RepairedContent(ix *index.Index, it inbox.Item) string {
	var d RepairDetail
	_ = json.Unmarshal(it.Detail, &d) // an item that does not decode tells no kind
	switch d.Kind {
	case "parent_moved":
		return "moved to the root because its parent moved to another store"
	case "ref_moved":
		return movedRefContent(ix, d.Value)
	case "sprint_detached", "release_detached":
		// The value is the name of the sprint or release (its ULID when it
		// cannot be read).
		var name string
		if json.Unmarshal(d.Value, &name) == nil && name != "" {
			what := strings.TrimSuffix(d.Kind, "_detached")
			return fmt.Sprintf("%s: removed from %s %s (it is not in the workspace the task moved to)", d.Kind, what, name)
		}
	case "parent_cycle", "parent_deleted":
		// The value tells where the repair took the task from and put it
		// (null is the root); the index only gives names to the ULIDs.
		var m struct{ From, To *model.ULID }
		if json.Unmarshal(d.Value, &m) == nil {
			from := "the root"
			if m.From != nil {
				from = TaskName(ix, *m.From)
			}
			if d.Kind == "parent_deleted" {
				return fmt.Sprintf("parent_deleted: moved to the root (its parent %s was deleted)", from)
			}
			to := "root"
			if m.To != nil {
				to = TaskName(ix, *m.To)
			}
			return fmt.Sprintf("parent_cycle: moved back to %s (was under %s)", to, from)
		}
	}
	if v := CompactJSON(d.Value); v != "" && v != "null" {
		return fmt.Sprintf("%s (was %s)", d.Kind, v)
	}
	return d.Kind
}

// DetachedName replaces the ULID that value (JSON) holds, for the kinds
// sprint_detached and release_detached, with the name of the sprint or
// release in s; the ULID stays when the file cannot be read. Other kinds
// keep their value.
func DetachedName(s *store.Store, kind, value string) string {
	var u model.ULID
	if (kind != "sprint_detached" && kind != "release_detached") || s == nil || json.Unmarshal([]byte(value), &u) != nil {
		return value
	}
	var name string
	if kind == "sprint_detached" {
		if sp, err := s.Sprint(u); err == nil {
			name = sp.Name
		}
	} else if r, err := s.Release(u); err == nil {
		name = r.Name
	}
	if name == "" {
		return value
	}
	b, _ := json.Marshal(name) // a string always encodes
	return string(b)
}

// finishLeftover is replicate.FinishLeftover, telling where an old store was kept.
func (ss *Session) finishLeftover(e config.StoreEntry, l store.Leftover) error {
	kept, err := replicate.FinishLeftoverKept(e.Path, l)
	for _, p := range kept {
		ss.AddWarning(fmt.Sprintf("the old store of %s, with changes that were not put back, is kept at %s", e.Name, p))
	}
	return err
}

// RepairDetail is the Detail of repaired, revived and prefix_duplicate.
type RepairDetail struct {
	Kind      string          `json:"kind"`
	Workspace model.ULID      `json:"workspace"`
	Value     json.RawMessage `json:"value"`
}

// movedRefContent says what a ref_moved repair removed: the field and the
// task it named, by the id the task had when it moved.
func movedRefContent(ix *index.Index, value json.RawMessage) string {
	var v struct {
		Field  string
		Target model.ULID
	}
	if json.Unmarshal(value, &v) != nil || v.Field == "" {
		return "ref_moved"
	}
	name := v.Target.String()
	if f, ok := ForwardOf(ix, v.Target); ok && f.ID != "" {
		name = f.ID
	}
	return fmt.Sprintf("%s to %s was removed because it moved to another store", v.Field, name)
}

// TaskName is the task u as "ID (ULID) title", or its ULID when ix lacks it.
func TaskName(ix *index.Index, u model.ULID) string {
	if ix != nil {
		if _, ok := ix.Task(u); ok {
			return RefName(ix, u)
		}
	}
	return u.String()
}

// CompactJSON is v on one line without spaces between tokens.
func CompactJSON(v json.RawMessage) string {
	var b bytes.Buffer
	if json.Compact(&b, v) != nil {
		return string(v)
	}
	return b.String()
}

// Overwritten is a value that lost in a merge of the sync.
type Overwritten struct {
	Task string `json:"task,omitempty"` // the ULID, for a task's file
	ID   string `json:"id,omitempty"`
	Path string `json:"path"`
	// Label is the id as a reader sees it, which --json leaves out
	Label  string          `json:"-"`
	Field  string          `json:"field"`
	Value  json.RawMessage `json:"value"`
	Author string          `json:"author,omitempty"`
	Notice string          `json:"notice,omitempty"` // the inbox item, when it is told to me
}

// Repaired is something the repair of the sync fixed or told of.
type Repaired struct {
	ULID   string          `json:"ulid"`
	ID     string          `json:"id,omitempty"` // the task's id now
	Kind   string          `json:"kind"`
	Value  json.RawMessage `json:"value"`
	Author string          `json:"author,omitempty"`
	// Label is the id as a reader sees it, and Line the text of a new id
	// ("ORG-2 → ORG-3"; "" for the other kinds); --json leaves them out
	Label string `json:"-"`
	Line  string `json:"-"`
}

// ForwardOf is the forward of u in ix, which may be nil.
func ForwardOf(ix *index.Index, u model.ULID) (*index.ForwardEntry, bool) {
	if ix == nil {
		return nil, false
	}
	return ix.Forward(u)
}

// ReadOnlyWhy is why the store s is read-only, in words, or "" when it is
// writable. A broken access.json is a reason, not an error: the next sync
// judges the access again.
func ReadOnlyWhy(s *store.Store) string {
	acc, err := replicate.ReadAccess(s)
	switch {
	case err != nil:
		return "access.json is broken; run hedgehug sync"
	case acc.ReadOnly:
		return acc.Why()
	}
	return ""
}
