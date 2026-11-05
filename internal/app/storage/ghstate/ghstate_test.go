package ghstate

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
	"github.com/kukv/hedgehug/internal/lib/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	author := gitrepo.Signature{Name: "Test", Email: "test@example.com"}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "test"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "test", IDPrefix: "TST"}, author, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	for name, c := range map[string]struct {
		r    Repo
		want bool
	}{
		"never":      {Repo{}, true},
		"too soon":   {Repo{LastFetch: now.Add(-4 * time.Minute)}, false},
		"five":       {Repo{LastFetch: now.Add(-5 * time.Minute)}, true},
		"low, soon":  {Repo{LastFetch: now.Add(-14 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, false},
		"low, late":  {Repo{LastFetch: now.Add(-15 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, true},
		"low, reset": {Repo{LastFetch: now.Add(-5 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(-time.Minute)}, true},
		"held back":  {Repo{NotBefore: now.Add(time.Minute)}, false},
		"hold over":  {Repo{NotBefore: now.Add(-time.Minute)}, true},

		"exactly 10%":     {Repo{LastFetch: now.Add(-6 * time.Minute), Limit: 5000, Remaining: 500, ResetAt: now.Add(time.Hour)}, true},
		"no limit known":  {Repo{LastFetch: now.Add(-6 * time.Minute), Remaining: 0, ResetAt: now.Add(time.Hour)}, true},
		"clock went back": {Repo{LastFetch: now.Add(time.Minute)}, true},
		"hold too far":    {Repo{NotBefore: now.Add(2 * time.Hour)}, true},
		"reset too far":   {Repo{LastFetch: now.Add(-6 * time.Minute), Limit: 5000, Remaining: 1, ResetAt: now.Add(2 * time.Hour)}, true},
		"hold at the cap": {Repo{NotBefore: now.Add(time.Hour)}, false},

		"failed, soon":       {Repo{LastFetch: now.Add(-time.Hour), LastTry: now.Add(-4 * time.Minute)}, false},
		"failed, late":       {Repo{LastFetch: now.Add(-time.Hour), LastTry: now.Add(-5 * time.Minute)}, true},
		"failed first, soon": {Repo{LastTry: now.Add(-time.Minute)}, false},
		"fetched after":      {Repo{LastFetch: now.Add(-time.Minute), LastTry: now.Add(-time.Hour)}, false},
		"try went forward":   {Repo{LastTry: now.Add(time.Minute)}, true},
		"failed, low, soon":  {Repo{LastTry: now.Add(-14 * time.Minute), Limit: 5000, Remaining: 1, ResetAt: now.Add(time.Hour)}, false},
	} {
		if got := c.r.Due(now); got != c.want {
			t.Errorf("%s = %v", name, got)
		}
	}
}

func TestNextAt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	for name, c := range map[string]struct {
		r    Repo
		want time.Time
	}{
		"never":      {Repo{}, time.Time{}},
		"read":       {Repo{LastFetch: now.Add(-time.Minute)}, now.Add(4 * time.Minute)},
		"overdue":    {Repo{LastFetch: now.Add(-time.Hour)}, now.Add(-55 * time.Minute)},
		"low":        {Repo{LastFetch: now.Add(-time.Minute), Limit: 10, Remaining: 0, ResetAt: now.Add(time.Hour)}, now.Add(14 * time.Minute)},
		"held":       {Repo{NotBefore: now.Add(time.Minute)}, now.Add(time.Minute)},
		"held, read": {Repo{LastFetch: now.Add(-4 * time.Minute), NotBefore: now.Add(30 * time.Minute)}, now.Add(30 * time.Minute)},
		"clock back": {Repo{LastFetch: now.Add(time.Minute)}, time.Time{}},
	} {
		if got := c.r.NextAt(now); !got.Equal(c.want) {
			t.Errorf("%s = %v, want %v", name, got, c.want)
		}
		if got := c.r.NextAt(now.Add(time.Second)); !got.Equal(c.want) {
			t.Errorf("%s moved with the clock: %v", name, got)
		}
	}
}

func TestSlowedUntil(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	reset := now.Add(time.Hour)
	if got := (Repo{Limit: 5000, Remaining: 4000, ResetAt: reset}).SlowedUntil(now); !got.IsZero() {
		t.Errorf("plenty = %v", got)
	}
	if got := (Repo{Limit: 5000, Remaining: 499, ResetAt: reset}).SlowedUntil(now); !got.Equal(reset) {
		t.Errorf("low = %v", got)
	}
	if got := (Repo{NotBefore: reset}).SlowedUntil(now); !got.Equal(reset) {
		t.Errorf("held = %v", got)
	}
}

func TestReadWrite(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	st, err := Read(s)
	if err != nil || len(st.Repos) != 0 {
		t.Fatalf("empty = %+v, %v", st, err)
	}
	at := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	st.Repos["github.com/o/r"] = Repo{LastFetch: at, Limit: 5000, Remaining: 1}
	if err := Write(s, st); err != nil {
		t.Fatal(err)
	}
	got, err := Read(s)
	if err != nil || !got.Repos["github.com/o/r"].LastFetch.Equal(at) {
		t.Errorf("read = %+v, %v", got, err)
	}
}

func TestReadBrokenFileIsEmpty(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.WriteLocal("github.json", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	st, err := Read(s)
	if err != nil || st.Repos == nil || len(st.Repos) != 0 {
		t.Errorf("broken = %+v, %v", st, err)
	}
}
