package query

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// storeDirs maps each test store to its directory, for writing files past
// the store. Tests do not run in parallel, so it needs no lock.
var storeDirs = map[*store.Store]string{}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	s, err := store.Create(dir, &model.StoreMeta{ID: model.NewID(), Name: "test"}, nil, &model.Workspace{ID: model.NewID(), Name: "test", KeyPrefix: "TST"}, testAuthor)
	if err != nil {
		t.Fatal(err)
	}
	storeDirs[s] = dir
	return s
}

// writeRaw writes a file as given and commits it past the store's checks,
// as a sync or a hand edit could.
func writeRaw(t *testing.T, s *store.Store, rel, content string) {
	t.Helper()
	p := filepath.Join(storeDirs[s], filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func newTask(title string) *model.Task {
	return &model.Task{ID: model.NewID(), Kind: "task", Title: title, Status: "todo"}
}

func setProp(task *model.Task, key, value string) {
	task.Props.Set(key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func setRefs(task *model.Task, key string, ids ...model.ID) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, id := range ids {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id.String()})
	}
	task.Props.Set(key, n)
}

func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	if _, err := s.Update(store.Op{Name: "update", Author: testAuthor}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func mustOpen(t *testing.T, s *store.Store) *index.Index {
	t.Helper()
	ix, err := index.Open(s)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// run parses and runs q, and returns the titles of each group.
func run(t *testing.T, ix *index.Index, q string, opt Options) [][]string {
	t.Helper()
	parsed, err := Parse(q)
	if err != nil {
		t.Fatalf("Parse(%q): %v", q, err)
	}
	res, err := Run(ix, parsed, opt)
	if err != nil {
		t.Fatalf("Run(%q): %v", q, err)
	}
	out := make([][]string, len(res.Groups))
	for i, g := range res.Groups {
		out[i] = []string{}
		for _, e := range g.Tasks {
			out[i] = append(out[i], e.Title)
		}
	}
	return out
}
