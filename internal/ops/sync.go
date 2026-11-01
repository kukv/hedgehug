package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
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

// Sync syncs the stores SyncTargets names and describes what each did. A
// store that failed is in the list with Err set, and the error joins the
// failures as "sync <store>: <err>". With no target the list is empty.
// explicit is a sync the user asked for (hh sync, "sync now" of the GUI): it
// tries a push again that was denied before. The background sync passes false.
func (ss *Session) Sync(all, explicit bool) ([]SyncReport, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	targets, err := ss.SyncTargets(c, all)
	if err != nil {
		return nil, err
	}
	// A store that cannot be opened is reported by its sync; the mount that
	// the cross-store check of the other stores opens need not say it again.
	var broken []string
	for _, e := range targets {
		if len(e.Remotes) == 0 {
			continue
		}
		if _, err := ss.openStoreAt(e.Path); err != nil {
			broken = append(broken, e.Name)
		}
	}
	defer ss.muteOpenWarnings(broken)()
	out := make([]SyncReport, 0, len(targets))
	var errs []error
	for _, e := range targets {
		r, err := ss.SyncStore(c, e, explicit)
		if err != nil {
			errs = append(errs, fmt.Errorf("sync %s: %w", e.Name, err))
			r.Err, r.Error = err, err.Error()
		}
		out = append(out, r)
	}
	return out, errors.Join(errs...)
}

// SyncStore syncs the store e and describes what it did (explicit as in
// Sync). On an error the description holds what is known.
func (ss *Session) SyncStore(c *config.Config, e config.StoreEntry, explicit bool) (SyncReport, error) {
	r := SyncReport{Store: e.Name, Pushed: []string{}, Overwritten: []Overwritten{}, Repaired: []Repaired{}, Invalid: []string{}, Notices: []inbox.Item{}, Warnings: []string{}}
	if len(e.Remotes) == 0 {
		r.NoRemote = true
		return r, nil
	}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return r, err
	}
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		return r, err
	}
	opt.Explicit = explicit
	res, err := ss.RunSync(s, opt, true)
	if err != nil {
		return r, err
	}
	// The push is done: what fails from here is a warning, as in sync.Run.
	ix, err := ss.indexOf(e.Name, s)
	if err != nil {
		r.Warnings = append(r.Warnings, "read the index, so ids are not shown: "+err.Error())
	}
	taskID := func(u model.ULID) string {
		if ix == nil {
			return ""
		}
		if t, ok := ix.Task(u); ok {
			return t.ID
		}
		return ""
	}
	qualify := func(id string) string {
		if ix == nil {
			return id
		}
		return ix.QualifyID(id)
	}
	r.Merges = res.Merges
	r.Unpushable = res.Unpushable
	r.Pushed = append(r.Pushed, res.Pushed...)
	r.Warnings = append(r.Warnings, res.Warnings...)
	r.Notices = append(r.Notices, res.Notices...)
	for _, o := range res.Overwritten {
		x := Overwritten{Path: o.Path, Field: o.Field, Value: jsonValue(o.Value), Author: o.Author}
		if u, ok := TaskOfPath(o.Path); ok {
			x.Task = u.String()
			x.ID = taskID(u)
			x.Label = qualify(x.ID)
		}
		for _, n := range res.Notices {
			if n.Kind == inbox.Overwritten && n.Path == o.Path && NoticeField(n) == o.Field {
				x.Notice = n.ULID.String()
			}
		}
		r.Overwritten = append(r.Overwritten, x)
	}
	for _, x := range res.Repaired {
		rep := Repaired{ULID: x.ULID.String(), ID: taskID(x.ULID), Kind: x.Kind, Value: jsonValue(x.Value), Author: x.Author}
		rep.Label = qualify(rep.ID)
		if x.Kind == "id_collision" {
			var v struct{ Old, New string }
			if json.Unmarshal([]byte(x.Value), &v) == nil && v.Old != "" && v.New != "" {
				rep.Line = fmt.Sprintf("%s (%s) → %s", qualify(v.Old), x.ULID, qualify(v.New))
			}
		}
		r.Repaired = append(r.Repaired, rep)
	}
	for _, u := range res.Invalid {
		r.Invalid = append(r.Invalid, u.String())
	}
	r.Warnings = append(r.Warnings, ss.crossStoreWarnings(e.Name)...)
	r.Warnings = append(r.Warnings, ss.duplicateWarnings(e.Name)...)
	return r, nil
}

// jsonValue is v as JSON when it is, else as a string: the values of the
// sync are one line of JSON, or "-" for none.
func jsonValue(v string) json.RawMessage {
	if json.Valid([]byte(v)) {
		return json.RawMessage(v)
	}
	b, _ := json.Marshal(v) // a string always encodes
	return b
}

// TaskOfPath is the ULID of the task whose file p is.
func TaskOfPath(p string) (model.ULID, bool) {
	rest, ok := strings.CutPrefix(p, "tasks/")
	if !ok {
		return model.ULID{}, false
	}
	name := rest[strings.LastIndex(rest, "/")+1:]
	u, err := model.ParseULID(strings.TrimSuffix(name, ".md"))
	return u, err == nil
}

// NoticeField is the field an overwritten item of the inbox is about.
func NoticeField(it inbox.Item) string {
	var d struct {
		Field string `json:"field"`
	}
	_ = json.Unmarshal(it.Detail, &d) // an item that does not decode has no field
	return d.Field
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
func (ss *Session) SyncStatus() ([]StoreStatus, error) {
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
		out[i] = ss.storeStatus(e)
	}
	return out, nil
}

func (ss *Session) storeStatus(e config.StoreEntry) StoreStatus {
	x := StoreStatus{Store: e.Name, Remotes: append([]string{}, e.Remotes...)}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		x.Error = err.Error()
		return x
	}
	st, err := sync.ReadState(s)
	if err != nil {
		x.Error = err.Error()
		return x
	}
	x.LastError = st.LastError
	// A broken access.json is a reason in ReadOnly, not an error that hides
	// the rest: the next sync judges the access again.
	acc, err := sync.ReadAccess(s)
	switch {
	case err != nil:
		x.ReadOnly = "access.json is broken; run hedgehug sync"
	case acc.ReadOnly:
		x.ReadOnly = acc.Why()
	}
	x.PendingSince, x.LastAttempt, x.LastSuccess = timePtr(st.PendingSince), timePtr(st.LastAttempt), timePtr(st.LastSuccess)
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

// timePtr is &t, or nil for the zero time.
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// SyncBackground is what the detached child of a write runs (sync
// --background --store <name>). Its failures are in sync.json and in log
// (the sync.log of the store), not in the returned error: no one waits for
// it. Only a missing store name is an error.
func (ss *Session) SyncBackground(log io.Writer) error {
	c, _, err := ss.Config()
	if err != nil {
		return backgroundFailed(ss, log, "", err)
	}
	if ss.StoreName == "" {
		return newErr(KindUsage, "sync.background_needs_store", errors.New("--background needs --store"))
	}
	e, err := c.Select(ss.StoreName)
	if err != nil {
		return backgroundFailed(ss, log, "", err)
	}
	s, err := ss.openStoreAt(e.Path)
	if err != nil {
		return backgroundFailed(ss, log, e.Name, err)
	}
	log = sync.BoundLog(s, log)
	opt, err := ss.SyncOptions(c, e)
	if err == nil {
		err = sync.Background(s, opt, c.Sync.PushInterval, func() error {
			res, err := ss.RunSync(s, opt, false)
			for _, w := range res.Warnings { // such as a mirror that is down; for sync.log
				_, _ = fmt.Fprintln(log, "warning: "+w)
			}
			return err
		})
	}
	if err != nil {
		// The same text a failed run left in sync.json already; the others
		// (SyncOptions, the lock) are told of by the next command.
		_, stateErr := sync.UpdateState(s, func(st *sync.State) {
			st.LastError, st.ErrorReported = err.Error(), false
		})
		err = errors.Join(err, stateErr)
	}
	return backgroundFailed(ss, log, e.Name, err)
}

// backgroundFailed logs err for sync.log and ends the child with exit code 0:
// no one waits for it. A store that could not be opened has no sync.json to
// tell of the failure; the log is all there is.
func backgroundFailed(ss *Session, log io.Writer, name string, err error) error {
	if err != nil {
		_, _ = fmt.Fprintf(log, "%s sync %s: %v\n", ss.Now().Format(time.RFC3339), name, err)
	}
	return nil
}

// SyncsInBackground is whether the store e is synced in the background: only
// a store with a remote is.
func SyncsInBackground(e config.StoreEntry) bool { return len(e.Remotes) > 0 }

// BeforeCommand reports the failure of a background sync that the user has
// not been told of (through report), and starts one for each store with a
// remote that is due (through start; sync.Start for the CLI). What goes
// wrong here is left to the command itself to tell: it fails on the same
// store or config.
func (ss *Session) BeforeCommand(start func(s *store.Store, name string) error, report func(store, msg string)) {
	cfg, _, err := ss.Config()
	if err != nil {
		return
	}
	targets, err := ss.SyncTargets(cfg, false)
	if err != nil {
		return
	}
	for _, e := range targets {
		if !SyncsInBackground(e) {
			continue
		}
		s, err := ss.openStoreAt(e.Path)
		if err != nil {
			continue
		}
		if msg, err := sync.TakeError(s); err == nil && msg != "" {
			report(e.Name, msg)
		}
		if st, err := sync.ReadState(s); err == nil && sync.Due(st, ss.Now(), cfg.Sync.Interval) {
			_ = start(s, e.Name)
		}
	}
}
