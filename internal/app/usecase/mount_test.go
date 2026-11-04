package usecase_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/browse"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// addMissingStore sets up org-b in config.yaml at a path where there is no
// store.
func addMissingStore(t *testing.T, ts *usecasetest.Session) {
	t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("org-b", filepath.Join(t.TempDir(), "gone"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreThatDoesNotOpenIsAnErrorOnlyWhenNamed(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	task := ts.AddTask("a", nil)
	addMissingStore(t, ts)

	ts.StoreName = "org-b"
	_, err := browse.Show(ts.Session, task.ULID.String())
	if err == nil {
		t.Fatal("Show on a store that does not open succeeded")
	}
	e := usecase.AsError(err)
	if e.Kind != usecase.KindInvalid || e.Code != "store.open" || !strings.HasPrefix(err.Error(), "cannot open org-b: ") {
		t.Errorf("err = %v (%v %s)", err, e.Kind, e.Code)
	}

	ts.StoreName = "me"
	if _, err := browse.Show(ts.Session, task.ULID.String()); err != nil {
		t.Errorf("Show on me: %v", err)
	}
}

func TestMountWarnsOnceAndOpensOnce(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	addMissingStore(t, ts)
	a, err := ts.Mount()
	if err != nil {
		t.Fatal(err)
	}
	b, err := ts.Mount()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("Mount opened the stores twice")
	}
	if len(a.Stores) != 1 || a.Stores[0].Name != "me" || len(a.Skipped) != 1 || a.Skipped[0].Name != "org-b" {
		t.Errorf("stores %v, skipped %v", a.Stores, a.Skipped)
	}
	if len(ts.Warns) != 1 || !strings.HasPrefix(ts.Warns[0], "cannot open org-b: ") {
		t.Errorf("warnings = %q", ts.Warns)
	}
	c, err := ts.ForStore("me").Mount()
	if err != nil || c != a {
		t.Errorf("ForStore session has another set: %v %v", c == a, err)
	}
	if len(ts.Warns) != 1 {
		t.Errorf("warnings = %q", ts.Warns)
	}
}

func TestNoStoreNameWarnsOnceAndUsesTheOthers(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	task := ts.AddTask("a", nil)
	addMissingStore(t, ts)
	ts.DropMount() // AddTask mounted the stores before org-b was added
	ts.Warns = nil
	ts.StoreName = ""
	set, err := ts.Mount()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Mount(); err != nil {
		t.Fatal(err)
	}
	if len(ts.Warns) != 1 || !strings.HasPrefix(ts.Warns[0], "cannot open org-b: ") {
		t.Errorf("warnings = %q", ts.Warns)
	}
	hit, err := usecase.ResolveIn(set, task.ID)
	if err != nil || hit.Store.Name != "me" {
		t.Errorf("Resolve(%s) = %v %v", task.ID, hit, err)
	}
	if _, ok := set.Get("org-b"); ok {
		t.Error("org-b is mounted")
	}
}
