package notifications_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// history is a store whose history has a commit of format 2 with a task_ref
// property fixes, then a commit of the newest format with fixes a text property.
type history struct {
	ts         *usecasetest.Session
	old, newer string // hashes
	dir        string
	meta       string // store.yaml of the newest format
}

func commitFiles(t *testing.T, dir, subject string, files map[string]string) string {
	t.Helper()
	r, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	var paths []string
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, rel)
	}
	h, err := r.Commit(gitrepo.Commit{Paths: paths, Subject: subject, Author: config.CommitAuthor(usecasetest.TestAccount), When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return h.String()
}

func newHistory(t *testing.T) *history {
	t.Helper()
	ts := usecasetest.New(t)
	_, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	meta, err := os.ReadFile(filepath.Join(e.Path, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	sc, err := os.ReadFile(filepath.Join(e.Path, "schema.yaml")) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	h := &history{ts: ts, dir: e.Path}
	h.old = commitFiles(t, e.Path, "format 2", map[string]string{
		model.StoreMetaPath: strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), "format: 2", 1),
		"schema.yaml":       "props:\n  fixes: {type: task_ref, multi: true, label: Fixes}\n  plain: {type: text}\n",
	})
	h.newer = commitFiles(t, e.Path, "newest", map[string]string{
		model.StoreMetaPath: string(meta),
		"schema.yaml":       strings.Replace(string(sc), "props:\n", "props:\n  fixes: {type: text}\n", 1),
	})
	if !strings.Contains(string(sc), "props:\n") {
		t.Fatal("the schema of the test store has no props")
	}
	h.meta = string(meta)
	return h
}

// The revert of an item reads the format and the schema.yaml of the item's own
// commit: a property that was a task_ref there is a relation, which is not
// reverted, whatever the value; the same name in the newest format is reverted.
func TestInboxRevertRefusesAnItemOfARelationFromBeforeFormat3(t *testing.T) {
	t.Parallel()
	for _, value := range []string{`"` + model.NewULID().String() + `"`, `null`, `[]`} {
		h := newHistory(t)
		task := h.ts.AddTask("t", nil)
		u := h.ts.AddItem(overwritten(task, h.old, "props.fixes", value))
		head := h.ts.Head()
		_, err := notifications.InboxRevert(h.ts.Session, u.String())
		usecasetest.WantErr(t, err, usecase.KindInvalid, "inbox.not_revertable")
		if err != nil && !strings.Contains(err.Error(), "dep") {
			t.Errorf("value %s: %v", value, err)
		}
		if h.ts.Head() != head {
			t.Errorf("value %s: a refused revert wrote", value)
		}
	}
}

func TestInboxRevertPutsBackAPropertyOfTheNewestFormat(t *testing.T) {
	t.Parallel()
	h := newHistory(t)
	task := h.ts.AddTask("t", nil)
	u := h.ts.AddItem(overwritten(task, h.newer, "props.fixes", `"x"`))
	if _, err := notifications.InboxRevert(h.ts.Session, u.String()); err != nil {
		t.Fatal(err)
	}
	_, ix, err := h.ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := ix.Task(task.ULID)
	if got := e.Prop("fixes"); len(got) != 1 || got[0] != "x" {
		t.Errorf("props.fixes = %v", got)
	}
}

// A commit that cannot be read stops the revert, with an error and no write.
func TestInboxRevertStopsWhenTheItemsCommitCannotBeRead(t *testing.T) {
	t.Parallel()
	h := newHistory(t)
	task := h.ts.AddTask("t", nil)
	nometa := commitFiles(t, h.dir, "nometa", map[string]string{model.StoreMetaPath: "format: ["})
	_ = commitFiles(t, h.dir, "restore", map[string]string{model.StoreMetaPath: h.meta})
	for name, c := range map[string]struct{ commit, value string }{"broken store.yaml": {nometa, `"x"`}, "not a hash": {"c1", `"y"`}} {
		u := h.ts.AddItem(overwritten(task, c.commit, "props.fixes", c.value))
		head := h.ts.Head()
		_, err := notifications.InboxRevert(h.ts.Session, u.String())
		usecasetest.WantErr(t, err, usecase.KindInvalid, "inbox.not_revertable")
		if err != nil && !strings.Contains(err.Error(), "cannot be read") {
			t.Errorf("%s: %v", name, err)
		}
		if h.ts.Head() != head {
			t.Errorf("%s: a refused revert wrote", name)
		}
	}
}
