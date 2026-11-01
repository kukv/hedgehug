package ops

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
)

// hookConfigWrite makes ts write config.yaml through another Config, right
// before the operation updates it, as another process could.
func hookConfigWrite(t *testing.T, ts *testSession, fn func(*config.Config) error) {
	t.Helper()
	ts.BeforeUpdate = func() {
		c, _, err := ts.Config()
		if err != nil {
			t.Error(err)
			return
		}
		if err := c.Update(fn); err != nil {
			t.Error(err)
		}
		ts.BeforeUpdate = nil
	}
}

func namedStores(t *testing.T, ts *testSession, name string) []config.StoreEntry {
	t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	var out []config.StoreEntry
	for _, e := range c.Stores {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}

func TestStoreInitRechecksExistenceInUpdate(t *testing.T) {
	ts := newTestSession(t)
	theirs := filepath.Join(t.TempDir(), "theirs")
	hookConfigWrite(t, ts, func(c *config.Config) error { return c.AddStore("team", theirs, nil) })
	_, err := ts.StoreInit(StoreInitInput{Name: "team", Prefix: "TM"})
	wantErr(t, err, KindConflict, "store.exists")
	got := namedStores(t, ts, "team")
	if len(got) != 1 || got[0].Path != theirs {
		t.Errorf("stores named team = %+v; want only the other writer's", got)
	}
}

func TestStoreAddRechecksExistenceInUpdate(t *testing.T) {
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Sync(false); err != nil {
		t.Fatal(err)
	}
	other := otherSession(t, ts, ts.Now())
	theirs := filepath.Join(t.TempDir(), "theirs")
	hookConfigWrite(t, other, func(c *config.Config) error { return c.AddStore("copy", theirs, nil) })
	_, err := other.StoreAdd("copy", []string{remote}, "")
	wantErr(t, err, KindConflict, "store.exists")
	got := namedStores(t, other, "copy")
	if len(got) != 1 || got[0].Path != theirs {
		t.Errorf("stores named copy = %+v; want only the other writer's", got)
	}
}

func TestStoreRemoveRechecksExistenceInUpdate(t *testing.T) {
	ts := newTestSession(t)
	if _, err := ts.StoreInit(StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	hookConfigWrite(t, ts, func(c *config.Config) error { return c.RemoveStore("team") })
	_, err := ts.StoreRemove("team", false)
	wantErr(t, err, KindNotFound, "store.not_found")
}
