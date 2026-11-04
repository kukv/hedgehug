package stores_test

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func namedStores(t *testing.T, ts *usecasetest.Session, name string) []config.StoreEntry {
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
	t.Parallel()
	ts := usecasetest.New(t)
	theirs := filepath.Join(t.TempDir(), "theirs")
	usecasetest.HookConfigWrite(t, ts, func(c *config.Config) error { return c.AddStore("team", theirs, nil) })
	_, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM"})
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.exists")
	got := namedStores(t, ts, "team")
	if len(got) != 1 || got[0].Path != theirs {
		t.Errorf("stores named team = %+v; want only the other writer's", got)
	}
}

func TestStoreAddRechecksExistenceInUpdate(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	remote := usecasetest.AddBareRemote(t, ts)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	other := usecasetest.Other(t, ts, ts.Now())
	theirs := filepath.Join(t.TempDir(), "theirs")
	usecasetest.HookConfigWrite(t, other, func(c *config.Config) error { return c.AddStore("copy", theirs, nil) })
	_, err := stores.StoreAdd(other.Session, "copy", []string{remote}, "", "")
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.exists")
	got := namedStores(t, other, "copy")
	if len(got) != 1 || got[0].Path != theirs {
		t.Errorf("stores named copy = %+v; want only the other writer's", got)
	}
}

func TestStoreRemoveRechecksExistenceInUpdate(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM"}); err != nil {
		t.Fatal(err)
	}
	usecasetest.HookConfigWrite(t, ts, func(c *config.Config) error { return c.RemoveStore("team") })
	_, err := stores.StoreRemove(ts.Session, "team", true)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")
}
