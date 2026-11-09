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

// schemaWithRelates is a schema.yaml of format 2: relates is a task_ref property.
const schemaWithRelates = "props:\n  relates: {type: task_ref, multi: true, symmetric: true}\n"

// A base in format 2 holds the relation in props; the two sides are in format 3,
// where it is a field. Ours took the relation away, and theirs only converted:
// the merge has the base as theirs has it, so the relation stays away.
func TestMergeOverABaseInFormat2DoesNotBringBackARelationOneSideRemoved(t *testing.T) {
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
	y := model.NewULID()
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
	commit := func(format int, sc string, relates, inProps bool, when time.Time, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		x.Relates, x.Props = nil, nil
		if relates && inProps {
			if err := yaml.Unmarshal([]byte("relates: ["+y.String()+"]\n"), &x.Props); err != nil {
				t.Fatal(err)
			}
		}
		if relates && !inProps {
			x.Relates = []model.ULID{y}
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
	base := commit(2, schemaWithRelates, true, true, at(0), mustHead(t, s))
	theirs := commit(model.StoreFormat, string(newSchema), true, false, at(2), base)
	ours := commit(model.StoreFormat, string(newSchema), false, false, at(1), base)

	m, err := mergeTrees(repo, nil, base, ours, theirs, c.opts[0])
	if err != nil {
		t.Fatal(err)
	}
	b, ok := m.files[model.TaskPath(x.ULID)]
	if !ok {
		return // nothing to change of ours is the answer too
	}
	got, err := model.DecodeTask(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Relates) != 0 || slices.ContainsFunc(got.Props, func(f model.Field) bool { return f.Key == "relates" }) {
		t.Errorf("merged relates = %v, props = %v; ours removed the relation", got.Relates, got.Props)
	}
}
