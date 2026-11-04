package usecase

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// readerOf is a session of its own over the stores of ts, whose repositories
// the test counts.
func readerOf(ts *testSession, storeName string) *Session {
	return &Session{Getenv: ts.Getenv, Now: ts.Now, StoreName: storeName, Warn: func(string) {}}
}

// Close lets go of every repository the session opened: the stores of Open,
// the mounted set, the inbox stores, and the set of a listing.
func TestSessionCloseClosesEveryRepoItOpened(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	ts.StoreName = "org-a"
	task := ts.addTask("one", nil)
	root := filepath.Dir(ts.env["HOME"])
	before := gitrepo.Live(root)

	reader := readerOf(ts, "")
	byName := readerOf(ts, "org-a")
	touch := func() {
		if _, _, err := byName.Open(); err != nil {
			t.Fatal(err)
		}
		set, err := reader.Mount()
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range set.Stores {
			if _, err := m.Index(); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := reader.InboxList(false); err != nil {
			t.Fatal(err)
		}
		q, err := ParseQuery("")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := reader.List(q); err != nil {
			t.Fatal(err)
		}
		if n, err := reader.MountedCount(); err != nil || n != 2 {
			t.Fatalf("MountedCount = %d, %v, want 2", n, err)
		}
	}
	touch()
	if got := gitrepo.Live(root); got <= before {
		t.Fatalf("live repos after the reads = %d, want more than %d", got, before)
	}
	reader.Close()
	byName.Close()
	if got := gitrepo.Live(root); got != before {
		t.Errorf("live repos after Close = %d, want %d", got, before)
	}

	// Close may be called again, and the session works after it
	reader.Close()
	touch()
	if _, err := byName.Show(task.ID); err != nil {
		t.Errorf("Show after Close: %v", err)
	}
	reader.Close()
	byName.Close()
	if got := gitrepo.Live(root); got != before {
		t.Errorf("live repos after the second Close = %d, want %d", got, before)
	}
}

// The stores a session mounted are let go of when it drops them, as
// StoreRemove does by updating config.yaml before it deletes a directory.
func TestDropMountClosesTheSet(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	root := filepath.Dir(ts.env["HOME"])
	reader := readerOf(ts, "")
	before := gitrepo.Live(root)
	if _, err := reader.Mount(); err != nil {
		t.Fatal(err)
	}
	if got := gitrepo.Live(root); got != before+2 {
		t.Fatalf("live repos after Mount = %d, want %d", got, before+2)
	}
	reader.DropMount()
	if got := gitrepo.Live(root); got != before {
		t.Errorf("live repos after dropMount = %d, want %d", got, before)
	}
}
