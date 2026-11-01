package ops

import (
	"path/filepath"
	"strings"
	"testing"
)

// addMissingStore sets up org-b in config.yaml at a path where there is no
// store.
func addMissingStore(t *testing.T, ts *testSession) {
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
	ts := newTestSession(t)
	task := ts.addTask("a", nil)
	addMissingStore(t, ts)

	ts.StoreName = "org-b"
	_, err := ts.Show(task.ULID.String())
	if err == nil {
		t.Fatal("Show on a store that does not open succeeded")
	}
	e := AsError(err)
	if e.Kind != KindInvalid || e.Code != "store.open" || !strings.HasPrefix(err.Error(), "cannot open org-b: ") {
		t.Errorf("err = %v (%v %s)", err, e.Kind, e.Code)
	}

	ts.StoreName = "me"
	if _, err := ts.Show(task.ULID.String()); err != nil {
		t.Errorf("Show on me: %v", err)
	}
}

func TestMountWarnsOnceAndOpensOnce(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
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
	if len(ts.warns) != 1 || !strings.HasPrefix(ts.warns[0], "cannot open org-b: ") {
		t.Errorf("warnings = %q", ts.warns)
	}
	c, err := ts.ForStore("me").Mount()
	if err != nil || c != a {
		t.Errorf("ForStore session has another set: %v %v", c == a, err)
	}
	if len(ts.warns) != 1 {
		t.Errorf("warnings = %q", ts.warns)
	}
}

func TestNoStoreNameWarnsOnceAndUsesTheOthers(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	task := ts.addTask("a", nil)
	addMissingStore(t, ts)
	ts.dropMount() // addTask mounted the stores before org-b was added
	ts.warns = nil
	ts.StoreName = ""
	set, err := ts.Mount()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Mount(); err != nil {
		t.Fatal(err)
	}
	if len(ts.warns) != 1 || !strings.HasPrefix(ts.warns[0], "cannot open org-b: ") {
		t.Errorf("warnings = %q", ts.warns)
	}
	hit, err := set.Resolve(task.ID)
	if err != nil || hit.Store.Name != "me" {
		t.Errorf("Resolve(%s) = %v %v", task.ID, hit, err)
	}
	if _, ok := set.Get("org-b"); ok {
		t.Error("org-b is mounted")
	}
}
