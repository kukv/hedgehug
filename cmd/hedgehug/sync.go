package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// syncLockWait is how long a sync waits for another one of the store, such
// as the one in the background, to end.
const syncLockWait = 5 * time.Minute

func (a *app) syncCmd() *cobra.Command {
	var status, background bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the stores with their remotes",
		Long:  "Sync fetches, merges, repairs and pushes. It syncs the store given with --store, or every store that has a remote.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if status {
				return a.syncStatus()
			}
			if background {
				return a.syncBackground()
			}
			return a.sync()
		},
	}
	cmd.Flags().BoolVar(&status, "status", false, "show how the syncs went and the commits not pushed, and sync nothing")
	cmd.Flags().BoolVar(&background, "background", false, "wait for push_interval after a write, then sync the store given with --store")
	_ = cmd.Flags().MarkHidden("background")
	return cmd
}

// syncBackground is what the detached child of a write runs. Its failures
// are in sync.json and the log, not in the exit code: no one waits for it.
func (a *app) syncBackground() error {
	c, _, err := a.ss.Config()
	if err != nil {
		return a.backgroundFailed("", err)
	}
	if a.ss.StoreName == "" {
		return usageErrorf("--background needs --store")
	}
	e, err := c.Select(a.ss.StoreName)
	if err != nil {
		return a.backgroundFailed("", err)
	}
	s, err := store.Open(e.Path)
	if err != nil {
		return a.backgroundFailed(e.Name, err)
	}
	a.stderr = sync.BoundLog(s, a.stderr)
	opt, err := a.ss.SyncOptions(c, e)
	if err == nil {
		err = sync.Background(s, opt, c.Sync.PushInterval, func() error {
			res, err := a.ss.RunSync(s, opt, false)
			for _, w := range res.Warnings { // such as a mirror that is down; for sync.log
				_, _ = fmt.Fprintln(a.stderr, "warning: "+w)
			}
			return err
		})
	}
	if err != nil {
		// The same text a failed run left in sync.json already; the others
		// (syncOptions, the lock) are told of by the next command.
		_, stateErr := sync.UpdateState(s, func(st *sync.State) {
			st.LastError, st.ErrorReported = err.Error(), false
		})
		err = errors.Join(err, stateErr)
	}
	return a.backgroundFailed(e.Name, err)
}

// backgroundFailed logs err for sync.log and ends the child with exit code 0:
// no one waits for it. A store that could not be opened has no sync.json to
// tell of the failure; the log is all there is.
func (a *app) backgroundFailed(name string, err error) error {
	if err != nil {
		_, _ = fmt.Fprintf(a.stderr, "%s sync %s: %v\n", a.now().Format(time.RFC3339), name, err)
	}
	return nil
}

// syncTargets are the stores a command works on: the one --store names, or
// all that have a remote, or with all every store (the status lists those
// with no remote too).
func (a *app) syncTargets(c *config.Config, all bool) ([]config.StoreEntry, error) {
	if a.ss.StoreName != "" {
		e, err := c.Select(a.ss.StoreName)
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

// syncJSON is what a sync of one store did, as --json prints it.
type syncJSON struct {
	Store       string            `json:"store"`
	NoRemote    bool              `json:"no_remote,omitempty"`
	Error       string            `json:"error,omitempty"`
	Merges      int               `json:"merges"`
	Pushed      []string          `json:"pushed"`
	Overwritten []overwrittenJSON `json:"overwritten"`
	Repaired    []repairedJSON    `json:"repaired"`
	Invalid     []string          `json:"invalid"` // ULIDs of the tasks with problems now
	Notices     []inbox.Item      `json:"notices"` // what the sync added to the inbox
	Warnings    []string          `json:"warnings"`
}

// overwrittenJSON is a value that lost in a merge of the sync.
type overwrittenJSON struct {
	Task   string          `json:"task,omitempty"` // the ULID, for a task's file
	ID     string          `json:"id,omitempty"`
	Path   string          `json:"path"`
	Field  string          `json:"field"`
	Value  json.RawMessage `json:"value"`
	Author string          `json:"author,omitempty"`
	Notice string          `json:"notice,omitempty"` // the inbox item, when it is told to me
}

// repairedJSON is something the repair of the sync fixed or told of.
type repairedJSON struct {
	ULID   string          `json:"ulid"`
	ID     string          `json:"id,omitempty"` // the task's id now
	Kind   string          `json:"kind"`
	Value  json.RawMessage `json:"value"`
	Author string          `json:"author,omitempty"`
}

func (a *app) sync() error {
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	targets, err := a.syncTargets(c, false)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if a.jsonOut {
			return a.printJSON([]syncJSON{})
		}
		_, err := fmt.Fprintln(a.stdout, "Nothing to sync: no store has a remote")
		return err
	}
	out := make([]syncJSON, 0, len(targets))
	var errs []error
	for _, e := range targets {
		r, err := a.syncStore(c, e)
		if err != nil {
			errs = append(errs, fmt.Errorf("sync %s: %w", e.Name, err))
			r.Error = describe(err)
		}
		out = append(out, r)
		if !a.jsonOut && err == nil {
			if err := a.printSync(r); err != nil {
				return err
			}
		}
	}
	if a.jsonOut {
		if err := a.printJSON(out); err != nil {
			return err
		}
	}
	return errors.Join(errs...)
}

// syncStore syncs the store e and describes what it did. On an error the
// description holds what is known.
func (a *app) syncStore(c *config.Config, e config.StoreEntry) (syncJSON, error) {
	r := syncJSON{Store: e.Name, Pushed: []string{}, Overwritten: []overwrittenJSON{}, Repaired: []repairedJSON{}, Invalid: []string{}, Notices: []inbox.Item{}, Warnings: []string{}}
	if len(e.Remotes) == 0 {
		r.NoRemote = true
		return r, nil
	}
	s, err := store.Open(e.Path)
	if err != nil {
		return r, err
	}
	opt, err := a.ss.SyncOptions(c, e)
	if err != nil {
		return r, err
	}
	res, err := a.ss.RunSync(s, opt, true)
	if err != nil {
		return r, err
	}
	// The push is done: what fails from here is a warning, as in sync.Run.
	ix, err := index.Open(s)
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
	r.Merges = res.Merges
	r.Pushed = append(r.Pushed, res.Pushed...)
	r.Warnings = append(r.Warnings, res.Warnings...)
	r.Notices = append(r.Notices, res.Notices...)
	for _, o := range res.Overwritten {
		x := overwrittenJSON{Path: o.Path, Field: o.Field, Value: jsonValue(o.Value), Author: o.Author}
		if u, ok := taskOfPath(o.Path); ok {
			x.Task = u.String()
			x.ID = taskID(u)
		}
		for _, n := range res.Notices {
			if n.Kind == inbox.Overwritten && n.Path == o.Path && noticeField(n) == o.Field {
				x.Notice = n.ULID.String()
			}
		}
		r.Overwritten = append(r.Overwritten, x)
	}
	for _, x := range res.Repaired {
		r.Repaired = append(r.Repaired, repairedJSON{ULID: x.ULID.String(), ID: taskID(x.ULID), Kind: x.Kind, Value: jsonValue(x.Value), Author: x.Author})
	}
	for _, u := range res.Invalid {
		r.Invalid = append(r.Invalid, u.String())
	}
	return r, nil
}

// printSync writes what a sync did as text.
func (a *app) printSync(r syncJSON) error {
	var b strings.Builder
	if r.NoRemote {
		fmt.Fprintf(&b, "Nothing to sync: %s has no remote\n", r.Store)
	} else {
		fmt.Fprintf(&b, "Synced %s: %d merges", r.Store, r.Merges)
		if len(r.Pushed) > 0 {
			b.WriteString(", pushed to " + strings.Join(r.Pushed, ", "))
		}
		b.WriteString("\n")
	}
	for _, x := range r.Repaired {
		fmt.Fprintf(&b, "  %s\n", repairedLine(x))
	}
	for _, o := range r.Overwritten {
		who := "an unknown author"
		if o.Author != "" {
			who = o.Author
		}
		what := o.Path
		if o.ID != "" {
			what = o.ID
		}
		hint := ""
		if o.Notice != "" {
			hint = "; hedgehug inbox revert " + o.Notice
		}
		fmt.Fprintf(&b, "  overwritten: %s %s (%s's value was replaced%s)\n", what, o.Field, who, hint)
	}
	if n := len(r.Invalid); n > 0 {
		fmt.Fprintf(&b, "  invalid: %d tasks now have problems; find them with hedgehug ls is:invalid\n", n)
	}
	if _, err := fmt.Fprint(a.stdout, b.String()); err != nil {
		return err
	}
	for _, w := range r.Warnings {
		if _, err := fmt.Fprintf(a.stderr, "warning: %s: %s\n", r.Store, w); err != nil {
			return err
		}
	}
	return nil
}

// repairedLine is the line of one repair: a new id as "ORG-2 (ULID) → ORG-3",
// the others as "repaired: ORG-4 (ULID) kind".
func repairedLine(x repairedJSON) string {
	if x.Kind == "id_collision" {
		var v struct{ Old, New string }
		if err := json.Unmarshal(x.Value, &v); err == nil && v.Old != "" && v.New != "" {
			return fmt.Sprintf("%s (%s) → %s", v.Old, x.ULID, v.New)
		}
	}
	name := x.ULID
	if x.ID != "" {
		name = x.ID + " (" + x.ULID + ")"
	}
	return fmt.Sprintf("repaired: %s %s", name, x.Kind)
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

// taskOfPath is the ULID of the task whose file p is.
func taskOfPath(p string) (model.ULID, bool) {
	rest, ok := strings.CutPrefix(p, "tasks/")
	if !ok {
		return model.ULID{}, false
	}
	name := rest[strings.LastIndex(rest, "/")+1:]
	u, err := model.ParseULID(strings.TrimSuffix(name, ".md"))
	return u, err == nil
}

// noticeField is the field an overwritten item of the inbox is about.
func noticeField(it inbox.Item) string {
	var d struct {
		Field string `json:"field"`
	}
	_ = json.Unmarshal(it.Detail, &d) // an item that does not decode has no field
	return d.Field
}

// statusJSON is how the syncs of one store went, as sync --status --json
// prints it.
type statusJSON struct {
	Store        string     `json:"store"`
	Remotes      []string   `json:"remotes"`
	PendingSince *time.Time `json:"pending_since,omitempty"`
	LastAttempt  *time.Time `json:"last_attempt,omitempty"`
	LastSuccess  *time.Time `json:"last_success,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
	Error        string     `json:"error,omitempty"`    // the store could not be read
	Unpushed     *int       `json:"unpushed,omitempty"` // absent when the store has no remote
}

func (a *app) syncStatus() error {
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	targets, err := a.syncTargets(c, true)
	if err != nil {
		return err
	}
	out := make([]statusJSON, len(targets))
	for i, e := range targets {
		out[i] = a.storeStatus(e)
	}
	if a.jsonOut {
		return a.printJSON(out)
	}
	var b strings.Builder
	for _, x := range out {
		fmt.Fprintf(&b, "%s\n", x.Store)
		if x.Error != "" {
			fmt.Fprintf(&b, "  error:         %s\n", x.Error)
			continue
		}
		fmt.Fprintf(&b, "  last attempt:  %s\n", timeText(x.LastAttempt))
		fmt.Fprintf(&b, "  last success:  %s\n", timeText(x.LastSuccess))
		if x.PendingSince != nil {
			fmt.Fprintf(&b, "  pending since: %s\n", timeText(x.PendingSince))
		}
		if x.LastError != "" {
			fmt.Fprintf(&b, "  last error:    %s\n", x.LastError)
		}
		switch {
		case x.Unpushed == nil:
			b.WriteString("  no remote\n")
		case *x.Unpushed == 1:
			b.WriteString("  1 commit not pushed\n")
		default:
			fmt.Fprintf(&b, "  %d commits not pushed\n", *x.Unpushed)
		}
	}
	_, err = fmt.Fprint(a.stdout, b.String())
	return err
}

// storeStatus reads how the syncs of e went. A store that cannot be read has
// its error in the row, so the status of the others is still shown.
func (a *app) storeStatus(e config.StoreEntry) statusJSON {
	x := statusJSON{Store: e.Name, Remotes: append([]string{}, e.Remotes...)}
	s, err := store.Open(e.Path)
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
	x.PendingSince, x.LastAttempt, x.LastSuccess = timePtr(st.PendingSince), timePtr(st.LastAttempt), timePtr(st.LastSuccess)
	if len(e.Remotes) > 0 {
		n, err := ops.Unpushed(s)
		if err != nil {
			x.Error = err.Error()
			return x
		}
		x.Unpushed = &n
	}
	return x
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func timeText(t *time.Time) string {
	if t == nil {
		return "never"
	}
	return t.Format(time.RFC3339)
}
