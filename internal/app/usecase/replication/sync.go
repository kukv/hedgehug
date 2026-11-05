// Package replication syncs the stores with their remotes.
package replication

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
)

// Sync syncs the stores SyncTargets names and describes what each did. A
// store that failed is in the list with Err set, and the error joins the
// failures as "sync <store>: <err>". With no target the list is empty.
// explicit is a sync the user asked for (hh sync, "sync now" of the GUI): it
// tries a push again that was denied before. The background sync passes false.
func Sync(ss *usecase.Session, all, explicit bool) ([]usecase.SyncReport, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	targets, err := ss.SyncTargets(c, all)
	if err != nil {
		return nil, err
	}
	if ss.StoreName == "" && !all {
		targets = append(targets, fetchOnly(ss, c, targets)...)
	}
	// A store that cannot be opened is reported by its sync; the mount that
	// the cross-store check of the other stores opens need not say it again.
	var broken []string
	for _, e := range targets {
		if len(e.Remotes) == 0 {
			continue
		}
		if err := ss.FinishRebuild(e); err != nil {
			broken = append(broken, e.Name)
			continue
		}
		if s, err := ss.OpenStoreAt(e.Path); err != nil {
			broken = append(broken, e.Name)
		} else {
			_ = s.Repo().Close()
		}
	}
	defer ss.MuteOpenWarnings(broken)()
	out := make([]usecase.SyncReport, 0, len(targets))
	var errs []error
	for _, e := range targets {
		r, err := SyncStore(ss, c, e, explicit)
		if err != nil {
			errs = append(errs, fmt.Errorf("sync %s: %w", e.Name, err))
			r.Err, r.Error = err, err.Error()
		}
		out = append(out, r)
	}
	ss.AddDuplicateWarnings(out)
	return out, errors.Join(errs...)
}

// SyncStore syncs the store e and describes what it did (explicit as in
// Sync). On an error the description holds what is known.
func SyncStore(ss *usecase.Session, c *config.Config, e config.StoreEntry, explicit bool) (usecase.SyncReport, error) {
	r := usecase.SyncReport{Store: e.Name, Pushed: []string{}, Overwritten: []usecase.Overwritten{}, Repaired: []usecase.Repaired{}, Invalid: []string{}, Notices: []inbox.Item{}, Warnings: []string{}}
	if len(e.Remotes) == 0 {
		r.NoRemote = true
		fetchGitHub(ss, c, e, &r, nil)
		return r, nil
	}
	// A swap that stopped is finished first.
	if err := ss.FinishRebuild(e); err != nil {
		return r, err
	}
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return r, err
	}
	defer func() { _ = s.Repo().Close() }()
	opt, err := ss.SyncOptions(c, e)
	if err != nil {
		return r, err
	}
	opt.Explicit = explicit
	res, err := ss.RunSync(s, opt, true)
	if err != nil {
		return r, err
	}
	// The push is done: what fails from here is a warning, as in replicate.Run.
	ix, err := ss.IndexOf(e.Name, s)
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
		x := usecase.Overwritten{Path: o.Path, Field: o.Field, Value: usecase.JSONValue(o.Value), Author: o.Author}
		if u, ok := model.TaskOfPath(o.Path); ok {
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
		rep := usecase.Repaired{ULID: x.ULID.String(), ID: taskID(x.ULID), Kind: x.Kind, Value: usecase.JSONValue(x.Value), Author: x.Author}
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
	r.Warnings = append(r.Warnings, ss.CrossStoreWarnings(e.Name)...)
	fetchGitHub(ss, c, e, &r, func() (replicate.Result, error) {
		again, err := ss.OpenStoreAt(e.Path)
		if err != nil {
			return replicate.Result{}, err
		}
		defer func() { _ = again.Repo().Close() }()
		return ss.RunSync(again, opt, true)
	})
	return r, nil
}

// fetchGitHub reads GitHub for the store e after its sync. push sends what it
// wrote; nil for a store with no remote. What fails here is a warning, as
// the sync is done.
func fetchGitHub(ss *usecase.Session, c *config.Config, e config.StoreEntry, r *usecase.SyncReport, push func() (replicate.Result, error)) {
	f, err := integration.Fetch(ss, c, e)
	r.GitHubUpdated = f.Committed
	r.Warnings = append(r.Warnings, f.Warnings...)
	if err != nil {
		r.Warnings = append(r.Warnings, "read GitHub: "+err.Error())
		return
	}
	if !f.Committed || push == nil {
		return
	}
	res, err := push()
	if err != nil {
		r.Warnings = append(r.Warnings, "push the values from GitHub: "+err.Error())
		return
	}
	r.Merges += res.Merges
	for _, remote := range res.Pushed {
		if !slices.Contains(r.Pushed, remote) {
			r.Pushed = append(r.Pushed, remote)
		}
	}
	r.Warnings = append(r.Warnings, res.Warnings...)
	r.Notices = append(r.Notices, res.Notices...)
}

// NoticeField is the field an overwritten item of the inbox is about.
func NoticeField(it inbox.Item) string {
	var d struct {
		Field string `json:"field"`
	}
	_ = json.Unmarshal(it.Detail, &d) // an item that does not decode has no field
	return d.Field
}

// SyncBackground is what the detached child of a write runs (sync
// --background --store <name>). Its failures are in sync.json and in log
// (the sync.log of the store), not in the returned error: no one waits for
// it. Only a missing store name is an error.
func SyncBackground(ss *usecase.Session, log io.Writer) error {
	c, _, err := ss.Config()
	if err != nil {
		return backgroundFailed(ss, log, "", err)
	}
	if ss.StoreName == "" {
		return usecase.NewErr(usecase.KindUsage, "sync.background_needs_store", errors.New("--background needs --store"))
	}
	e, err := c.Select(ss.StoreName)
	if err != nil {
		return backgroundFailed(ss, log, "", err)
	}
	if err := ss.FinishRebuild(e); err != nil {
		return backgroundFailed(ss, log, e.Name, err)
	}
	s, err := ss.OpenStoreAt(e.Path)
	if err != nil {
		return backgroundFailed(ss, log, e.Name, err)
	}
	defer func() { _ = s.Repo().Close() }()
	log = replicate.BoundLog(s, log)
	opt, err := ss.SyncOptions(c, e)
	if err == nil {
		err = replicate.Background(s, opt, c.Sync.PushInterval, func() error {
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
		_, stateErr := replicate.UpdateState(s, func(st *replicate.State) {
			st.LastError, st.ErrorReported = err.Error(), false
		})
		err = errors.Join(err, stateErr)
	}
	return backgroundFailed(ss, log, e.Name, err)
}

// backgroundFailed logs err for sync.log and ends the child with exit code 0:
// no one waits for it. A store that could not be opened has no sync.json to
// tell of the failure; the log is all there is.
func backgroundFailed(ss *usecase.Session, log io.Writer, name string, err error) error {
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
// remote that is due (replicate.Start). It reads config.yaml itself. What
// goes wrong here is left to the command itself to tell: it fails on the same
// store or config.
func BeforeCommand(ss *usecase.Session, report func(store, msg string)) {
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
		s, err := ss.OpenStoreAt(e.Path)
		if err != nil {
			continue
		}
		if msg, err := replicate.TakeError(s); err == nil && msg != "" {
			report(e.Name, msg)
		}
		if st, err := replicate.ReadState(s); err == nil && replicate.Due(st, ss.Now(), cfg.Sync.Interval) {
			_ = replicate.Start(s, e.Name, ss.Getenv) // starts a process of its own; it does not keep s
		}
		_ = s.Repo().Close()
	}
}

// fetchOnly are the stores with no remote that name a repository on GitHub:
// they have nothing to sync, but GitHub is read for them. Those in done are
// left out.
func fetchOnly(ss *usecase.Session, c *config.Config, done []config.StoreEntry) []config.StoreEntry {
	var out []config.StoreEntry
	for _, e := range c.Stores {
		if len(e.Remotes) == 0 && !slices.ContainsFunc(done, func(d config.StoreEntry) bool { return d.Name == e.Name }) && integration.HasTargets(ss, e) {
			out = append(out, e)
		}
	}
	return out
}
