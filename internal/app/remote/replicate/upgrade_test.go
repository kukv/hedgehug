package replicate

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

// oldKindsSchema is a schema.yaml of format 1: it lists the custom kind chore.
const oldKindsSchema = `kinds:
  - {name: project, role: container}
  - {name: chore, role: work}
`

// fromBeforeAConversion makes the three commits of a merge in which the base
// is in format 1 and the two sides are in format 2: the task x is a chore at
// the base, and the sides have it as a task, as hh store convert writes it,
// with ours then changing its kind to bug (ours at minute 1, theirs at minute
// 2, so that theirs is the later of two changes to the kind).
func fromBeforeAConversion(t *testing.T, c *cluster) (x *model.Task, base, ours, theirs plumbing.Hash) {
	t.Helper()
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
	x = &model.Task{ULID: model.NewULID(), ID: "WEB-1", Workspace: ws, Kind: "chore", Title: "x", Status: "todo", Rank: rank}
	meta, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(model.StoreMetaPath))) //nolint:gosec // the test's store
	if err != nil {
		t.Fatal(err)
	}
	newSchema, err := schema.Default().Encode()
	if err != nil {
		t.Fatal(err)
	}
	at := func(m int) time.Time { return time.Date(2026, 11, 9, 12, m, 0, 0, time.UTC) }
	commit := func(format int, sc, kind string, when time.Time, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		x.Kind = kind
		body, err := x.Encode()
		if err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
			model.StoreMetaPath:    strings.Replace(string(meta), "format: 2", "format: "+string(rune('0'+format)), 1),
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
	base = commit(1, oldKindsSchema, "chore", at(0), mustHead(t, s))
	theirs = commit(2, string(newSchema), "task", at(2), base)
	ours = commit(2, string(newSchema), "bug", at(1), base)
	return x, base, ours, theirs
}

func TestSidesGivesTheBaseOfAnOlderFormatInTheFormatOfTheSides(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	x, base, ours, theirs := fromBeforeAConversion(t, c)
	b, _, _, err := Sides(c.stores[0].Repo(), base, ours, theirs, model.TaskPath(x.ULID))
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.DecodeTask(b)
	if err != nil || got.Kind != "task" {
		t.Errorf("base = kind %q, %v; want task (it was a chore before the conversion)", got.Kind, err)
	}
	// a file that is not a task is the base as it is
	b, _, _, err = Sides(c.stores[0].Repo(), base, ours, theirs, schema.Path)
	if err != nil || string(b) != oldKindsSchema {
		t.Errorf("schema.yaml of the base = %q, %v", b, err)
	}
}

// What one side changed after converting is not lost to the other side, which
// only converted: the merge has the base in the same format as the sides.
func TestMergeOverABaseOfAnOlderFormatKeepsWhatOneSideChanged(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	x, base, ours, theirs := fromBeforeAConversion(t, c)
	m, err := mergeTrees(c.stores[0].Repo(), nil, base, ours, theirs, c.opts[0])
	if err != nil {
		t.Fatal(err)
	}
	b, ok := m.files[model.TaskPath(x.ULID)]
	if !ok {
		// the merge changed nothing of ours, which is the answer too
		return
	}
	got, err := model.DecodeTask(b)
	if err != nil || got.Kind != "bug" {
		t.Errorf("merged kind = %q, %v; want bug", got.Kind, err)
	}
}
