package inbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// A commit that converts the store (here, to a format 3 that a step standing
// in for a real one gives) is compared with its parent in one format: the
// parent's task is upgraded, so what the conversion only moved is no item. The
// test must not be parallel.
func TestCollectReadsTheParentOfAConversionInTheNewFormat(t *testing.T) {
	a, b := twoClones(t)
	x := addTask(t, a, "x", nil)
	syncBoth(t, a, b)
	before := head(t, b)

	// Format 5 keeps who works on a task in the file, where format 4 had
	// nobody: the step gives every old task the assignee github.com/b.
	was, wasSteps := model.StoreFormat, upgrade.Steps
	model.StoreFormat = 5
	upgrade.Steps = append(slices.Clone(wasSteps), upgrade.Step{From: 4, Apply: func(raw []byte, _ upgrade.Cx) ([]byte, error) {
		tk, err := model.DecodeTask(raw)
		if err != nil {
			return nil, err
		}
		tk.Assignees = append(tk.Assignees, "github.com/b")
		return tk.Encode()
	}})
	t.Cleanup(func() { model.StoreFormat, upgrade.Steps = was, wasSteps })

	dir := a.s.Dir()
	meta, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	x.Assignees = []string{"github.com/b"}
	body, err := x.Encode()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		model.StoreMetaPath:    strings.Replace(string(meta), "format: 4", "format: 5", 1),
		model.TaskPath(x.ULID): string(body),
	}
	var paths []string
	for rel, data := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, rel)
	}
	if _, err := a.s.Repo().Commit(gitrepo.Commit{
		Paths: paths, Subject: "convert", Author: gitrepo.Signature{Name: "a", Email: "a@x"}, When: time.Now(),
		Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "convert"}, {Key: "Hedgehug-Author", Value: "github.com/a"}},
	}); err != nil {
		t.Fatal(err)
	}
	syncBoth(t, a, b)
	items, err := Collect(b.s, before, head(t, b), []string{"github.com/b"})
	if err != nil || len(items) != 0 {
		t.Errorf("items = %+v, %v; want none", items, err)
	}
}
