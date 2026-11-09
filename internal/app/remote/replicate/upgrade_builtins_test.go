package replicate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// A base in format 3 holds the points in props (a property of the store, here a
// text); the two sides are in format 4, where it is a field. Ours took the
// points away, and theirs only converted and renamed the task: the merge has the
// base as theirs has it, so the points stay away.
func TestMergeOverABaseInFormat3DoesNotBringBackPointsOneSideRemoved(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	s := c.stores[0]
	dir, repo := s.Dir(), s.Repo()
	var ws model.ULID
	for w, err := range s.Workspaces() {
		if err == nil {
			ws = w.ULID
			break
		}
	}
	rank, err := model.Between("", "")
	if err != nil {
		t.Fatal(err)
	}
	x := &model.Task{ULID: model.NewULID(), ID: "WEB-1", Workspace: ws, Kind: "task", Title: "x", Status: "todo", Rank: rank}
	meta, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	newSchema, err := schema.Default().Encode()
	if err != nil {
		t.Fatal(err)
	}
	at := func(m int) time.Time { return time.Date(2026, 11, 9, 12, m, 0, 0, time.UTC) }
	three := 3.0
	// the task as a commit of the format has it: the points in props (format 3) or in the field
	commit := func(format int, sc, title string, points, inProps bool, when time.Time, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		x.Title, x.Points, x.Props = title, nil, nil
		if points && inProps {
			if err := yaml.Unmarshal([]byte("points: \"3\"\n"), &x.Props); err != nil {
				t.Fatal(err)
			}
		}
		if points && !inProps {
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
			p := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, rel)
		}
		h, err := repo.Commit(gitrepo.Commit{Paths: paths, Parents: parents, Subject: "x", Author: gitrepo.Signature{Name: "a", Email: "a@x"}, When: when})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	// theirs renames the task, so that the merge has a file to write
	base := commit(3, "props:\n  points: {type: text}\n", "x", true, true, at(0), mustHead(t, s))
	theirs := commit(model.StoreFormat, string(newSchema), "x renamed", true, false, at(2), base)
	ours := commit(model.StoreFormat, string(newSchema), "x", false, false, at(1), base)

	m, err := mergeTrees(repo, nil, base, ours, theirs, c.opts[0])
	if err != nil {
		t.Fatal(err)
	}
	b, ok := m.files[model.TaskPath(x.ULID)]
	if !ok {
		t.Fatal("the merge does not write the task, though theirs renamed it")
	}
	got, err := model.DecodeTask(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "x renamed" {
		t.Errorf("title = %q; what theirs changed is merged", got.Title)
	}
	if got.Points != nil || slices.ContainsFunc(got.Props, func(f model.Field) bool { return f.Key == "points" }) {
		t.Errorf("merged points = %v, props = %v; ours removed the points", got.Points, got.Props)
	}
}
