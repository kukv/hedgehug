package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	Unpushable int `json:"unpushable,omitempty"`
	// GitHubUpdated is whether the sync wrote what it read from GitHub (links
	// and gh) to the store.
	GitHubUpdated bool  `json:"github_updated,omitempty"`
	Err           error `json:"-"` // why the sync failed; the report holds what is known
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

// ReleaseForRebuild lets go of what this session and the server hold of the
// store at dir, before a rebuild changes its files.
func (ss *Session) ReleaseForRebuild(dir string) {
	if ss.BeforeRebuild != nil {
		ss.BeforeRebuild(dir)
	}
	ss.CloseMounted(dir)
	ss.DropMount()
}

// FinishRebuild finishes a rebuild of the store e that stopped, before it is
// opened. Nothing left is nothing done. It works under sync.lock, which a
// rebuild holds, so a rebuild in progress is not taken for one that stopped.
func (ss *Session) FinishRebuild(e config.StoreEntry) error {
	if store.RebuildPhaseIn(e.Path) == store.RebuildNone {
		return nil
	}
	ss.ReleaseForRebuild(e.Path)
	s, err := store.Open(e.Path)
	if err != nil {
		return err
	}
	defer func() { _ = s.Repo().Close() }()
	var b *replicate.Rebuild
	err = s.WithLocalLock("sync.lock", SyncLockWait, func() error {
		var err error
		b, err = replicate.FinishRebuild(s)
		return err
	})
	if err != nil || b == nil {
		return err
	}
	for _, w := range b.Warnings {
		ss.AddWarning(e.Name + ": " + w)
	}
	if b.Kept != "" {
		ss.AddWarning(fmt.Sprintf("the old store of %s, with changes that were not put back, is kept at %s", e.Name, b.Kept))
	}
	return nil
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

// StoreStatus is how the syncs of one store went, as sync --status --json
// prints it.
type StoreStatus struct {
	Store        string     `json:"store"`
	Remotes      []string   `json:"remotes"`
	PendingSince *time.Time `json:"pending_since,omitempty"`
	LastAttempt  *time.Time `json:"last_attempt,omitempty"`
	LastSuccess  *time.Time `json:"last_success,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
	Error        string     `json:"error,omitempty"`     // the store could not be read
	Unpushed     *int       `json:"unpushed,omitempty"`  // absent when the store has no remote
	ReadOnly     string     `json:"read_only,omitempty"` // why the store is read-only, in words; absent when it is not
}

// SyncStatus reads how the syncs of every store went. A store that cannot be
// read has its error in the row, so the status of the others is still shown.
func SyncStatus(ss *Session) ([]StoreStatus, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	targets, err := ss.SyncTargets(c, true)
	if err != nil {
		return nil, err
	}
	out := make([]StoreStatus, len(targets))
	for i, e := range targets {
		out[i] = storeStatus(ss, e)
	}
	return out, nil
}

func storeStatus(ss *Session, e config.StoreEntry) StoreStatus {
	x := StoreStatus{Store: e.Name, Remotes: append([]string{}, e.Remotes...)}
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		x.Error = err.Error()
		return x
	}
	defer func() { _ = s.Repo().Close() }()
	st, err := replicate.ReadState(s)
	if err != nil {
		x.Error = err.Error()
		return x
	}
	x.LastError = st.LastError
	x.ReadOnly = ReadOnlyWhy(s) // a reason in ReadOnly, not an error that hides the rest
	x.PendingSince, x.LastAttempt, x.LastSuccess = TimePtr(st.PendingSince), TimePtr(st.LastAttempt), TimePtr(st.LastSuccess)
	if len(e.Remotes) > 0 {
		n, err := Unpushed(s)
		if err != nil {
			x.Error = err.Error()
			return x
		}
		x.Unpushed = &n
	}
	return x
}

// SyncIntervals is sync.interval and sync.push_interval of config.yaml.
type SyncIntervals struct{ Interval, PushInterval time.Duration }

// SyncSchedule is the schedule of the loop that syncs: the GUI server's loop
// satisfies it.
type SyncSchedule interface {
	NextFetch(store string) (time.Time, bool)
	PushAt(store string, pending time.Time, interval, push time.Duration) (time.Time, bool)
	FetchInterval(interval time.Duration) time.Duration
}

// SyncStatusView is a StoreStatus with when the loop fetches next and when
// the writes waiting are pushed.
type SyncStatusView struct {
	StoreStatus
	NextFetch    *time.Time `json:"next_fetch,omitempty" doc:"When the loop fetches next; absent before its first sync; absent when no server runs the loop"`
	PushAt       *time.Time `json:"push_at,omitempty" doc:"When the background sync pushes the writes not pushed yet: sync.push_interval after pending_since, but not before sync.push_interval after the last sync and not after next_fetch, which syncs too; absent when nothing waits, before the first sync and while a sync runs; absent when no server runs the loop"`
	Interval     float64    `json:"interval" doc:"sync.interval of config.yaml, in seconds (fractions allowed)"`
	PushInterval float64    `json:"push_interval" doc:"sync.push_interval of config.yaml, in seconds (fractions allowed)"`
}

// SyncStatusViews reads how the syncs of every store went, with the
// intervals of config.yaml and without a loop that schedules them: as the
// CLI prints it.
func SyncStatusViews(ss *Session) ([]SyncStatusView, error) {
	iv, err := ss.SyncIntervals()
	if err != nil {
		return nil, err
	}
	sts, err := SyncStatus(ss)
	if err != nil {
		return nil, err
	}
	out := make([]SyncStatusView, len(sts))
	for i, st := range sts {
		out[i] = NewSyncStatusView(st, nil, iv)
	}
	return out, nil
}

// NewSyncStatusView is st with when sch fetches next and when the writes
// waiting are pushed, as it schedules them with the intervals iv. With a nil
// sch, NextFetch and PushAt are nil.
func NewSyncStatusView(st StoreStatus, sch SyncSchedule, iv SyncIntervals) SyncStatusView {
	v := SyncStatusView{
		StoreStatus:  st,
		Interval:     iv.Interval.Seconds(),
		PushInterval: iv.PushInterval.Seconds(),
	}
	if sch == nil {
		return v
	}
	if t, ok := sch.NextFetch(st.Store); ok {
		v.NextFetch = &t
	}
	if st.PendingSince != nil {
		if t, ok := sch.PushAt(st.Store, *st.PendingSince, sch.FetchInterval(iv.Interval), iv.PushInterval); ok {
			v.PushAt = &t
		}
	}
	return v
}
