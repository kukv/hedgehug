package ops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/doctor"
	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// takeFx is a store "me" with a remote, cloned by a second user (clone),
// and the first user's history rewrite that purges ME-2 pushed.
type takeFx struct {
	ts, clone *testSession
	remote    string
	keep      *model.Task // ME-1
	gone      *model.Task // ME-2
	mine      *model.Task // an unpushed task of the clone
}

func newTakeFx(t *testing.T) *takeFx {
	t.Helper()
	ts := newTestSession(t)
	f := &takeFx{ts: ts, remote: addBareRemote(t, ts)}
	f.keep = ts.addTask("keep", nil)
	f.gone = ts.addTask("gone", nil)
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	f.clone = cloneSession(t, ts, f.remote, ts.Now().Add(time.Minute))
	f.mine = f.clone.addTask("mine", nil)
	// the clone's unpushed edit of the task that is purged
	s, _, err := f.clone.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.clone.Update(s, store.Op{Name: "update", Task: f.gone.ULID}, func(tx *store.Tx) error {
		x, err := tx.Task(f.gone.ULID)
		if err != nil {
			return err
		}
		x.Title = "gone, edited"
		return tx.PutTask(x)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.RewriteHistory(RewriteInput{Arg: "ME-2", Typed: "me"}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *takeFx) path(t *testing.T) string {
	t.Helper()
	_, e, err := f.clone.Entry()
	if err != nil {
		t.Fatal(err)
	}
	return e.Path
}

// The sync of a clone that finds the rewrite takes it in: BeforeSwap is told
// first, the unpushed work is put back and pushed, nothing is left beside
// the store, and the inbox tells of it.
func TestSyncTakesInAHistoryRewrite(t *testing.T) {
	f := newTakeFx(t)
	path := f.path(t)
	var told []string
	f.clone.BeforeSwap = func(p string) {
		told = append(told, p)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("the store is gone before BeforeSwap: %v", err)
		}
	}
	rs, err := f.clone.Sync(false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(told, []string{path}) {
		t.Errorf("BeforeSwap was told %v, want %s", told, path)
	}
	if len(rs) != 1 || !slices.Contains(rs[0].Pushed, "origin") {
		t.Fatalf("report = %+v", rs)
	}
	// the sync tells of what it took in
	var noticed []inbox.Kind
	for _, n := range rs[0].Notices {
		noticed = append(noticed, n.Kind)
	}
	if !slices.Contains(noticed, inbox.RewriteApplied) || !slices.Contains(noticed, inbox.PurgedChange) {
		t.Errorf("notices of the report = %v", noticed)
	}
	if left, err := sync.FindLeftover(path); err != nil || !left.None() {
		t.Errorf("leftover = %+v, %v", left, err)
	}
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(f.mine.ULID); err != nil {
		t.Errorf("the unpushed task: %v", err)
	}
	if _, err := s.Task(f.gone.ULID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the purged task = %v, want not found", err)
	}
	if _, err := s.Task(f.keep.ULID); err != nil {
		t.Errorf("the kept task: %v", err)
	}
	if got, want := originHead(t, f.remote), mustHead(t, s); got != want {
		t.Errorf("origin is at %s, the clone at %s", got, want)
	}
	items, err := inbox.Open(s).List(true)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []inbox.Kind
	for _, it := range items {
		kinds = append(kinds, it.Kind)
	}
	if !slices.Contains(kinds, inbox.RewriteApplied) || !slices.Contains(kinds, inbox.PurgedChange) {
		t.Errorf("inbox kinds = %v", kinds)
	}
	for _, it := range items {
		switch it.Kind {
		case inbox.RewriteApplied:
			if got := InboxContent(nil, it, "me", false); got != "the history rewrite of me was taken in (2 unpushed changes put back)" {
				t.Errorf("content = %q", got)
			}
		case inbox.PurgedChange:
			if got := InboxContent(nil, it, "me", false); got != "unpushed changes to the purged task "+f.gone.ID+" were not taken in" {
				t.Errorf("content = %q", got)
			}
		}
	}
	// the log and the activity know the replay commit
	lr, err := f.clone.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var replays int
	for _, v := range StoreLogViews(lr.Logs) {
		if v.Op != nil && *v.Op == "replay" {
			replays++
		}
	}
	if replays != 1 {
		t.Errorf("replay commits in the log = %d, want 1", replays)
	}
	// the next sync has nothing more to take in
	if _, err := f.clone.Sync(false, true); err != nil {
		t.Errorf("second sync: %v", err)
	}
}

func mustHead(t *testing.T, s *store.Store) plumbing.Hash {
	t.Helper()
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return head
}

// A write that lands between the rebuild and the swap stops the swap with a
// warning; the next sync makes the rebuild again.
func TestSyncRebuildStaleIsAWarning(t *testing.T) {
	f := newTakeFx(t)
	var late *model.Task
	f.clone.BeforeSwap = func(string) {
		if late == nil {
			late = f.clone.addTask("late", nil)
		}
	}
	rs, err := f.clone.Sync(false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || !slices.ContainsFunc(rs[0].Warnings, func(w string) bool { return strings.Contains(w, "changed while it was rebuilt") }) {
		t.Fatalf("warnings = %+v", rs)
	}
	if left, _ := sync.FindLeftover(f.path(t)); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
	if _, err := f.clone.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []model.ULID{late.ULID, f.mine.ULID} {
		if _, err := s.Task(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	if _, err := s.Task(f.gone.ULID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the purged task = %v", err)
	}
}

// breakSwap leaves the clone's store as a swap that stopped after the old
// store was moved away: no store at the path, the old one and the rebuild
// beside it.
func (f *takeFx) breakSwap(t *testing.T) (path string) {
	t.Helper()
	path = f.path(t)
	ss := f.clone.Session
	// make the rebuild without the swap: let BeforeSwap move the store away
	// and stop the sync there.
	var staleErr error
	ss.BeforeSwap = func(p string) {
		staleErr = os.Rename(p, p+".before-rewrite-20260101T000000.000000000")
	}
	if _, err := ss.Sync(false, true); err == nil {
		t.Fatal("the sync did not fail with the store gone")
	}
	if staleErr != nil {
		t.Fatal(staleErr)
	}
	ss.BeforeSwap = nil
	return path
}

// A store whose swap stopped does not open, says why, and is finished by the
// next sync, which then takes the rewrite in as usual.
func TestSyncFinishesAStoppedSwap(t *testing.T) {
	f := newTakeFx(t)
	path := f.breakSwap(t)
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the store is there: %v", err)
	}
	f.clone.warns = nil
	_, err := f.clone.OpenStore()
	var le *sync.LeftoverError
	if err == nil || !strings.Contains(err.Error(), sync.LeftoverMessage("me")) || !errors.As(err, &le) {
		t.Errorf("OpenStore = %v", err)
	}
	if _, err := f.clone.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	if left, _ := sync.FindLeftover(path); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(f.mine.ULID); err != nil {
		t.Error(err)
	}
}

// A store with a leftover beside it opens with a warning, once.
func TestOpenStoreWarnsOfALeftover(t *testing.T) {
	f := newTakeFx(t)
	path := f.path(t)
	if err := os.MkdirAll(path+".before-rewrite-1", 0o750); err != nil {
		t.Fatal(err)
	}
	f.clone.warns = nil
	for range 2 {
		if _, err := f.clone.OpenStore(); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.clone.warns; len(got) != 1 || got[0] != sync.LeftoverMessage("me") {
		t.Errorf("warnings = %q", got)
	}
	// the mount tells of it too, and the store is in it
	f.clone.warns = nil
	set, err := f.clone.Mount()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Get("me"); !ok {
		t.Error("the store is not mounted")
	}
	if len(set.Unfinished) != 1 {
		t.Errorf("Unfinished = %+v", set.Unfinished)
	}
}

// doctor finds a leftover as a warning and --fix finishes it, also when the
// store itself is missing.
func TestDoctorRewriteLeftover(t *testing.T) {
	f := newTakeFx(t)
	path := f.breakSwap(t)
	rs, err := f.clone.Doctor(false)
	if err != nil {
		t.Fatal(err)
	}
	var got []doctor.Finding
	for _, r := range rs {
		for _, x := range r.Findings {
			if x.Check == "rewrite_leftover" {
				got = append(got, x)
			}
		}
	}
	if len(got) != 2 {
		t.Fatalf("findings = %+v", rs)
	}
	for _, x := range got {
		if x.Severity != doctor.Warning || x.Where != path || !strings.Contains(x.Msg, "is left from a rebuild after a history rewrite") || x.Hint != "run hedgehug doctor --fix" {
			t.Errorf("finding = %+v", x)
		}
	}
	rs, err = f.clone.Doctor(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		for _, x := range r.Findings {
			if x.Check == "rewrite_leftover" {
				t.Errorf("after --fix: %+v", x)
			}
		}
	}
	if _, err := store.Open(path); err != nil {
		t.Errorf("the store does not open: %v", err)
	}
	if left, _ := sync.FindLeftover(path); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
}

// With the store there, a leftover is removed: the old store of a finished
// swap, and a rebuild never swapped in.
func TestDoctorFixRemovesTheOldStore(t *testing.T) {
	f := newTakeFx(t)
	path := f.path(t)
	for _, suffix := range []string{".before-rewrite-1", ".rewrite-new"} {
		if err := os.MkdirAll(filepath.Join(path+suffix, "x"), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.clone.Doctor(true); err != nil {
		t.Fatal(err)
	}
	if left, _ := sync.FindLeftover(path); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
}
