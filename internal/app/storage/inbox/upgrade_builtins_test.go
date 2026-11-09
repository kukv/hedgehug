package inbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// The commit that converts a store from format 3 (the points are in props) is
// compared with its parent in the newest format: the parent's task is upgraded
// by the real step, so a task that only had its points moved makes no item. A
// parent that did not upgrade would read as no task, and the assignee would
// show as new.
func TestCollectReadsAParentOfFormat3ThroughTheRealStep(t *testing.T) {
	a, _ := twoClones(t)
	x := addTask(t, a, "x", nil)
	dir := a.s.Dir()
	meta, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	commit := func(format int, sc string, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		x.Assignees = []string{"github.com/b"}
		x.Props, x.Points = nil, nil
		if format < model.StoreFormat {
			if err := yaml.Unmarshal([]byte("points: \"3\"\n"), &x.Props); err != nil {
				t.Fatal(err)
			}
		} else {
			three := 3.0
			x.Points = &three
		}
		body, err := x.Encode()
		if err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
			model.StoreMetaPath:    strings.Replace(string(meta), fmt.Sprintf("format: %d", model.StoreFormat), fmt.Sprintf("format: %d", format), 1),
			schema.Path:            sc,
			model.TaskPath(x.ULID): string(body),
		}
		var paths []string
		for rel, data := range files {
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, rel)
		}
		h, err := a.s.Repo().Commit(gitrepo.Commit{
			Paths: paths, Parents: parents, Subject: "x", Author: gitrepo.Signature{Name: "a", Email: "a@x"}, When: time.Now(),
			Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "convert"}, {Key: "Hedgehug-Author", Value: "github.com/a"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	old := commit(3, "props:\n  points: {type: text}\n", head(t, a))
	converted := commit(model.StoreFormat, "", old)
	items, err := Collect(a.s, old, converted, []string{"github.com/b"})
	if err != nil || len(items) != 0 {
		t.Errorf("items = %+v, %v; want none", items, err)
	}
}
