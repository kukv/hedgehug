package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// The base git gives the driver is a file of the history, which may be in an
// older store format than the sides: a side that converted and then changed the
// kind keeps its change against a side that only converted.
func TestMergeDriverUpgradesABaseOfAnOlderFormat(t *testing.T) {
	alice, _, _ := twoUsers(t)
	task := *task1(t, alice)
	dir := alice.storeDir("org")
	r, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	read := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))) //nolint:gosec // the store of the test
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	meta, newSchema := read(model.StoreMetaPath), read(schema.Path)
	const oldSchema = "kinds:\n  - {name: project, role: container}\n  - {name: chore, role: work}\n"
	at := func(m int) time.Time { return time.Date(2026, 11, 9, 12, m, 0, 0, time.UTC) }
	taskFile := func(kind string) string {
		t.Helper()
		x := task
		x.Kind = kind
		b, err := x.Encode()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	commit := func(format, sc, kind string, when time.Time, parent plumbing.Hash) plumbing.Hash {
		t.Helper()
		files := map[string]string{
			model.StoreMetaPath:       strings.Replace(meta, "format: 2", "format: "+format, 1),
			schema.Path:               sc,
			model.TaskPath(task.ULID): taskFile(kind),
		}
		var paths []string
		for rel, data := range files {
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, rel)
		}
		h, err := r.Commit(gitrepo.Commit{Paths: paths, Parents: []plumbing.Hash{parent}, Subject: "x", Author: gitrepo.Signature{Name: "a", Email: "a@x"}, When: when})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	// The base is in format 1 with the task a chore. Both sides converted it
	// (kind task); ours then made it a bug, earlier than theirs converted.
	base := commit("1", oldSchema, "chore", at(0), head)
	theirsCommit := commit("2", newSchema, "task", at(2), base)
	commit("2", newSchema, "bug", at(1), base) // HEAD

	tmp := t.TempDir()
	baseFile, ours, theirs := filepath.Join(tmp, "base"), filepath.Join(tmp, "ours"), filepath.Join(tmp, "theirs")
	for p, kind := range map[string]string{baseFile: "chore", ours: "bug", theirs: "task"} {
		if err := os.WriteFile(p, []byte(taskFile(kind)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	t.Setenv("GITHEAD_"+theirsCommit.String(), "theirs")
	if code, _, errs := alice.run("merge-driver", baseFile, ours, theirs, model.TaskPath(task.ULID)); code != 0 {
		t.Fatalf("merge-driver: %d %s", code, errs)
	}
	merged, err := os.ReadFile(ours) //nolint:gosec // the file of the test
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.DecodeTask(merged)
	if err != nil || got.Kind != "bug" {
		t.Errorf("merged kind = %q, %v; want bug", got.Kind, err)
	}
}
