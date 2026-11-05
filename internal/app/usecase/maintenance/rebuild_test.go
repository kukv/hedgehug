package maintenance_test

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

// forcePushFx is a store "me" with a remote (ts), cloned by a second user
// (clone) who has unpushed work: a task of their own, and with editGone an
// edit of the task gone. Origin is then rewritten without gone, as a git
// filter-repo leaves it.
type forcePushFx struct {
	ts, clone        *usecasetest.Session
	remote           string
	keep, gone, mine *model.Task
}

func newForcePushFx(t *testing.T, editGone bool) *forcePushFx {
	t.Helper()
	ts := usecasetest.New(t)
	f := &forcePushFx{ts: ts, remote: usecasetest.AddBareRemote(t, ts)}
	f.keep = ts.AddTask("keep", nil)
	f.gone = ts.AddTask("gone", nil)
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	f.clone = usecasetest.Clone(t, ts, f.remote, ts.Now().Add(time.Minute))
	f.mine = f.clone.AddTask("mine", nil)
	if editGone {
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
	}
	gitrepotest.RewriteRemote(t, f.remote, model.TaskPath(f.gone.ULID), path.Dir(model.CommentPath(f.gone.ULID, model.ULID{})))
	return f
}

func (f *forcePushFx) path(t *testing.T) string {
	t.Helper()
	_, e, err := f.clone.Entry()
	if err != nil {
		t.Fatal(err)
	}
	return e.Path
}

// stoppedRebuild leaves the clone's store as a rebuild that stopped before it
// changed anything: rebuild.json says main goes to the head of origin. The
// sync brings the objects of the new history.
func (f *forcePushFx) stoppedRebuild(t *testing.T) (dir string) {
	t.Helper()
	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	dir = f.path(t)
	h := originHead(t, f.remote).String()
	f.writeRecord(t, dir, `{"phase":"apply","old_head":"`+gitrepotest.HeadHash(t, dir)+`","target":"`+h+`","origin":"`+h+`","mark":"`+h+`"}`)
	return dir
}

func (f *forcePushFx) writeRecord(t *testing.T, dir, rec string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(store.LocalDir), store.RebuildFile)
	if err := os.WriteFile(p, []byte(rec+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func noRebuild(t *testing.T, dir string) {
	t.Helper()
	if p := store.RebuildPhaseIn(dir); p != store.RebuildNone {
		t.Errorf("phase = %q", p)
	}
}

// forcePushFindings are the findings of r that tell of anything but the merge
// driver, which a test without hedgehug on PATH always has.
func forcePushFindings(r maintenance.DoctorReport) []usecase.Finding {
	var out []usecase.Finding
	for _, x := range r.Findings {
		if x.Check != "merge_driver" {
			out = append(out, x)
		}
	}
	return out
}

func originHead(t *testing.T, bare string) plumbing.Hash {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Reference("refs/heads/main", true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash()
}

func mustHead(t *testing.T, s *store.Store) plumbing.Hash {
	t.Helper()
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return head
}

// A sync that finds origin force pushed stops and changes nothing; doctor
// tells of it.
func TestSyncStopsOnAForcePush(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	before := mustHead(t, s)
	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	if s, err = f.clone.OpenStore(); err != nil || mustHead(t, s) != before {
		t.Errorf("the store changed: %v", err)
	}
	rs, err := maintenance.Doctor(f.clone.Session, false)
	if err != nil || len(rs) != 1 || len(forcePushFindings(rs[0])) != 1 {
		t.Fatalf("Doctor = %+v, %v; want one finding", rs, err)
	}
	x := forcePushFindings(rs[0])[0]
	if x.Check != "unexpected_force_push" || x.Where != "me" || !strings.Contains(x.Msg, "differs from this store's") || !strings.Contains(x.Hint, "doctor --fix") {
		t.Errorf("finding = %+v", x)
	}
	if rs[0].Rebuilt != nil {
		t.Errorf("doctor without --fix rebuilt the store: %+v", rs[0].Rebuilt)
	}
}

// doctor --fix rebuilds the store: BeforeRebuild is told first, the unpushed
// work is put back except what was done to the task that is gone, a copy of
// the old store is kept where the report says, and the next sync pushes the
// work.
func TestDoctorFixRebuildsTheStore(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.path(t)
	var told []string
	f.clone.BeforeRebuild = func(p string) {
		told = append(told, p)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("the store is gone before BeforeRebuild: %v", err)
		}
	}
	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	rs, err := maintenance.Doctor(f.clone.Session, true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt == nil {
		t.Fatalf("Doctor = %+v, %v; want a rebuild", rs, err)
	}
	b := rs[0].Rebuilt
	if b.Replayed != 2 || !slices.Equal(b.Dropped, []string{f.gone.ID}) || !strings.HasPrefix(b.Kept, dir+".kept-") {
		t.Errorf("rebuilt = %+v", b)
	}
	if _, err := os.Stat(b.Kept); err != nil {
		t.Errorf("the old store is not at Kept: %v", err)
	}
	if !slices.Equal(told, []string{dir}) {
		t.Errorf("BeforeRebuild was told %v, want %s", told, dir)
	}
	if fs := forcePushFindings(rs[0]); len(fs) != 0 {
		t.Errorf("findings after --fix: %+v", fs)
	}
	noRebuild(t, dir)
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(f.mine.ULID); err != nil {
		t.Errorf("the unpushed task: %v", err)
	}
	if _, err := s.Task(f.keep.ULID); err != nil {
		t.Errorf("the kept task: %v", err)
	}
	if _, err := s.Task(f.gone.ULID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the task that is gone = %v, want not found", err)
	}
	if _, ok, err := replicate.ReadForcePush(s); err != nil || ok {
		t.Errorf("force-push.json = %v, %v", ok, err)
	}
	if _, err := replication.Sync(f.clone.Session, false, true); err != nil {
		t.Fatalf("the sync after --fix: %v", err)
	}
	if s, err = f.clone.OpenStore(); err != nil {
		t.Fatal(err)
	}
	if got, want := originHead(t, f.remote), mustHead(t, s); got != want {
		t.Errorf("origin is at %s, the clone at %s", got, want)
	}
}

// With nothing dropped no old store is kept, and the report says so.
func TestDoctorFixKeepsNoOldStoreWhenNothingWasDropped(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, false)
	dir := f.path(t)
	if _, err := replication.Sync(f.clone.Session, false, true); !errors.Is(err, replicate.ErrForcePush) {
		t.Fatalf("Sync = %v, want ErrForcePush", err)
	}
	rs, err := maintenance.Doctor(f.clone.Session, true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt == nil {
		t.Fatalf("Doctor = %+v, %v; want a rebuild", rs, err)
	}
	if b := rs[0].Rebuilt; b.Replayed != 1 || len(b.Dropped) != 0 || b.Kept != "" {
		t.Errorf("rebuilt = %+v", b)
	}
	if m, err := filepath.Glob(dir + ".*"); err != nil || len(m) != 0 {
		t.Errorf("something is left beside the store: %v, %v", m, err)
	}
}

// A store that was not force pushed is not rebuilt.
func TestDoctorFixDoesNotRebuildAStoreThatWasNotForcePushed(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	usecasetest.AddBareRemote(t, ts)
	ts.AddTask("a", nil)
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	rs, err := maintenance.Doctor(ts.Session, true)
	if err != nil || len(rs) != 1 || rs[0].Rebuilt != nil {
		t.Fatalf("Doctor = %+v, %v; want no rebuild", rs, err)
	}
}

// A store whose rebuild stopped does not open, says why, and is finished by
// the next sync.
func TestSyncFinishesAStoppedRebuild(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.stoppedRebuild(t)
	f.clone.Warns = nil
	_, err := f.clone.OpenStore()
	var le *store.LeftoverError
	if err == nil || !strings.Contains(err.Error(), store.LeftoverMessage("me")) || !errors.As(err, &le) {
		t.Errorf("OpenStore = %v", err)
	}
	set, err := f.clone.Mount()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Get("me"); ok {
		t.Error("the store is mounted")
	}
	if _, err := replication.Sync(f.clone.Session, false, true); err != nil {
		t.Fatalf("Sync = %v", err)
	}
	noRebuild(t, dir)
	s, err := f.clone.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(f.gone.ULID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the task that is gone = %v, want not found", err)
	}
	if _, err := s.Task(f.keep.ULID); err != nil {
		t.Error(err)
	}
}

// A store with only the old objects left to remove opens with a warning, once.
func TestOpenStoreWarnsOfAnUnfinishedPrune(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.path(t)
	f.writeRecord(t, dir, `{"phase":"prune"}`)
	f.clone.Warns = nil
	for range 2 {
		if _, err := f.clone.OpenStore(); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.clone.Warns; len(got) != 1 || got[0] != store.LeftoverMessage("me") {
		t.Errorf("warnings = %q", got)
	}
	// the mount tells of it too, and the store is in it
	f.clone.Warns = nil
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

// doctor --fix finishes a rebuild that stopped; doctor without --fix says the
// store does not open, and looks at nothing in it.
func TestDoctorFixFinishesAStoppedRebuild(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.stoppedRebuild(t)
	var le *store.LeftoverError
	if _, err := maintenance.Doctor(f.clone.Session, false); !errors.As(err, &le) {
		t.Fatalf("Doctor of a store that stopped = %v, want a LeftoverError", err)
	}
	if _, err := maintenance.Doctor(f.clone.Session, true); err != nil {
		t.Fatal(err)
	}
	noRebuild(t, dir)
	if _, err := f.clone.OpenStore(); err != nil {
		t.Errorf("the store does not open: %v", err)
	}
}

// A file edited by hand since the rebuild stopped is not overwritten: doctor
// --fix tells of it, and the store stays closed until it is put back.
func TestDoctorFixStopsAtAFileEditedByHand(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.stoppedRebuild(t)
	p := filepath.Join(dir, filepath.FromSlash(model.TaskPath(f.gone.ULID)))
	was, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("by hand\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := maintenance.Doctor(f.clone.Session, true)
	if err != nil || len(rs) != 1 || !usecase.FindingsFail(rs[0].Findings) {
		t.Fatalf("Doctor = %+v, %v; want a finding", rs, err)
	}
	if got, _ := os.ReadFile(p); string(got) != "by hand\n" {
		t.Errorf("the file = %q", got)
	}
	if store.RebuildPhaseIn(dir) != store.RebuildApply {
		t.Errorf("phase = %q", store.RebuildPhaseIn(dir))
	}
	if err := os.WriteFile(p, was, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenance.Doctor(f.clone.Session, true); err != nil {
		t.Fatal(err)
	}
	noRebuild(t, dir)
}

// A rebuild.json that cannot be read keeps the store closed and is told of.
func TestDoctorFixTellsOfABrokenRecord(t *testing.T) {
	t.Parallel()
	f := newForcePushFx(t, true)
	dir := f.path(t)
	f.writeRecord(t, dir, "not json")
	if _, err := f.clone.OpenStore(); err == nil {
		t.Error("the store opened")
	}
	rs, err := maintenance.Doctor(f.clone.Session, true)
	if err != nil || len(rs) != 1 || !usecase.FindingsFail(rs[0].Findings) {
		t.Fatalf("Doctor = %+v, %v; want a finding", rs, err)
	}
}
