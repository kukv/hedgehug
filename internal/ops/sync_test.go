package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

func TestSyncWithoutRemoteAndStatus(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	reps, err := ts.Sync(false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) != 0 {
		t.Fatalf("Sync reported %d stores; want none, as no store has a remote", len(reps))
	}
	st, err := ts.SyncStatus()
	if err != nil || len(st) != 1 {
		t.Fatalf("SyncStatus = %v, %v", st, err)
	}
	if st[0].Store != "me" || st[0].Unpushed != nil || len(st[0].Remotes) != 0 {
		t.Errorf("SyncStatus row = %+v; want store me without remote and without unpushed", st[0])
	}
	reps, err = ts.Sync(true, true)
	if err != nil || len(reps) != 1 || !reps[0].NoRemote || reps[0].Store != "me" {
		t.Errorf("Sync(all) = %+v, %v; want one NoRemote report of me", reps, err)
	}
}

func TestSyncTargetsNamedStoreNotFound(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	other := &Session{Getenv: ts.Getenv, Now: ts.Now, StoreName: "nope"}
	_, err = other.SyncTargets(c, false)
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindNotFound || e.Code != "store.not_found" {
		t.Fatalf("SyncTargets error = %v; want KindNotFound store.not_found", err)
	}
	if !strings.Contains(err.Error(), `no store is named "nope"`) {
		t.Errorf("message = %q; want it unchanged", err)
	}
}

func TestSyncStorePushesAndStatusReportsIt(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	st, err := ts.SyncStatus()
	if err != nil || len(st) != 1 || len(st[0].Remotes) != 1 || st[0].Remotes[0] != remote {
		t.Fatalf("SyncStatus = %+v, %v; want the remote %s", st, err, remote)
	}
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	targets, err := ts.SyncTargets(c, false)
	if err != nil || len(targets) != 1 {
		t.Fatalf("SyncTargets = %v, %v", targets, err)
	}
	r, err := ts.SyncStore(c, targets[0], true)
	if err != nil {
		t.Fatal(err)
	}
	if r.Store != "me" || r.NoRemote || len(r.Pushed) == 0 {
		t.Fatalf("SyncStore = %+v; want a push", r)
	}
	st, err = ts.SyncStatus()
	if err != nil || len(st) != 1 {
		t.Fatal(st, err)
	}
	if st[0].Unpushed == nil || *st[0].Unpushed != 0 || st[0].LastSuccess == nil || st[0].LastError != "" {
		t.Errorf("SyncStatus after the sync = %+v; want 0 unpushed and a last success", st[0])
	}
	b, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(b), `"pushed":["`) {
		t.Errorf("JSON = %s, %v", b, err)
	}
}

func TestSyncAllReportsFailedStore(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if err := os.RemoveAll(remote); err != nil {
		t.Fatal(err)
	}
	reps, err := ts.Sync(false, true)
	if err == nil || !strings.HasPrefix(err.Error(), "sync me: ") {
		t.Fatalf("Sync error = %v; want one starting with \"sync me: \"", err)
	}
	if len(reps) != 1 || reps[0].Err == nil || reps[0].Store != "me" {
		t.Errorf("reports = %+v; want the failed store with Err set", reps)
	}
	// Error is filled with Err's message, for a caller that has no words of its own.
	if len(reps) == 1 && (reps[0].Error == "" || reps[0].Error != reps[0].Err.Error()) {
		t.Errorf("report Error = %q; want %q", reps[0].Error, reps[0].Err)
	}
}

func TestBeforeCommandStartsNothingWithoutRemote(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	var started int
	ts.BeforeCommand(func(*store.Store, string) error { started++; return nil }, func(string, string) { t.Error("reported") })
	if started != 0 {
		t.Errorf("started %d syncs for a store without a remote", started)
	}
}

func TestBeforeCommandStartsDueStoreAndReportsFailureOnce(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	addBareRemote(t, ts)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	var started []string
	start := func(_ *store.Store, name string) error { started = append(started, name); return nil }
	var reports []string
	report := func(st, msg string) { reports = append(reports, st+"|"+msg) }

	ts.BeforeCommand(start, report) // never synced: due
	if len(started) != 1 || started[0] != "me" || len(reports) != 0 {
		t.Fatalf("started %v, reports %v; want a start of me and no report", started, reports)
	}

	if _, err := sync.UpdateState(s, func(st *sync.State) {
		st.LastError, st.LastAttempt = "remote is down", ts.Now()
	}); err != nil {
		t.Fatal(err)
	}
	started = nil
	ts.BeforeCommand(start, report)
	if len(reports) != 1 || reports[0] != "me|remote is down" {
		t.Fatalf("reports = %v; want the failure once", reports)
	}
	if len(started) != 0 {
		t.Errorf("started %v; a sync that failed just now is not tried again yet", started)
	}
	ts.BeforeCommand(start, report)
	if len(reports) != 1 {
		t.Errorf("reports = %v; want the failure told once", reports)
	}
}

func TestSyncBackgroundWithoutStoreIsUsageError(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	var log bytes.Buffer
	err := ts.SyncBackground(&log)
	if err == nil || KindOf(err) != KindUsage || err.Error() != "--background needs --store" {
		t.Fatalf("SyncBackground = %v (kind %v); want the usage error", err, KindOf(err))
	}
}

func TestSyncBackgroundLogsFailureAndKeepsItInState(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if err := os.RemoveAll(remote); err != nil {
		t.Fatal(err)
	}
	ts.StoreName = "me"
	var log bytes.Buffer
	if err := ts.SyncBackground(&log); err != nil {
		t.Fatalf("SyncBackground = %v; a background failure is not an error", err)
	}
	if !strings.Contains(log.String(), "2026-10-30T12:00:00Z sync me: ") {
		t.Errorf("log = %q; want a timestamped line for me", log.String())
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	st, err := sync.ReadState(s)
	if err != nil || st.LastError == "" || st.ErrorReported {
		t.Errorf("state = %+v, %v; want an unreported last error", st, err)
	}
}

func TestSyncBackgroundUnknownStoreLogsWithEmptyName(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.StoreName = "nope"
	var log bytes.Buffer
	if err := ts.SyncBackground(&log); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(log.String(), "2026-10-30T12:00:00Z sync : no store is named") {
		t.Errorf("log = %q", log.String())
	}
}

func TestNoticeFieldAndTaskOfPath(t *testing.T) {
	t.Parallel()
	if got := NoticeField(inbox.Item{Detail: json.RawMessage(`{"field":"title"}`)}); got != "title" {
		t.Errorf("NoticeField = %q; want title", got)
	}
	if got := NoticeField(inbox.Item{Detail: json.RawMessage(`not json`)}); got != "" {
		t.Errorf("NoticeField of bad detail = %q; want empty", got)
	}
	u := model.NewULID()
	if got, ok := TaskOfPath("tasks/ab/" + u.String() + ".md"); !ok || got != u {
		t.Errorf("TaskOfPath = %v, %v; want %v", got, ok, u)
	}
	for _, p := range []string{"schema.yaml", "tasks/ab/zzz.md"} {
		if _, ok := TaskOfPath(p); ok {
			t.Errorf("TaskOfPath(%q) ok; want not", p)
		}
	}
}

func TestJSONValueAndTimePtr(t *testing.T) {
	t.Parallel()
	if got := string(jsonValue(`{"a":1}`)); got != `{"a":1}` {
		t.Errorf("jsonValue(json) = %s", got)
	}
	if got := string(jsonValue("-")); got != `"-"` {
		t.Errorf("jsonValue(-) = %s; want a JSON string", got)
	}
	if timePtr(time.Time{}) != nil {
		t.Error("timePtr(zero) is not nil")
	}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if p := timePtr(now); p == nil || !p.Equal(now) {
		t.Errorf("timePtr = %v", p)
	}
}

func TestSyncsInBackground(t *testing.T) {
	t.Parallel()
	if SyncsInBackground(config.StoreEntry{}) || !SyncsInBackground(config.StoreEntry{Remotes: []string{"x"}}) {
		t.Error("SyncsInBackground should be true only with a remote")
	}
}
