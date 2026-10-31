package sync

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// cluster is a bare remote and n clones of one store, each with its own
// writer account.
type cluster struct {
	t      *testing.T
	remote string
	stores []*store.Store
	opts   []Options
	last   model.Rank // the last rank given by addTask
}

func newCluster(t *testing.T, n int) *cluster {
	t.Helper()
	c := &cluster{t: t, remote: filepath.Join(t.TempDir(), "r.git")}
	if err := gitrepo.InitBare(c.remote); err != nil {
		t.Fatal(err)
	}
	first, err := store.Create(filepath.Join(t.TempDir(), "s0"), &model.StoreMeta{ULID: model.NewULID(), Name: "s"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "web", IDPrefix: "WEB"}, gitrepo.Signature{Name: "u0", Email: "u0@x"}, "github.com/u0", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	c.add(first, 0)
	if _, err := Run(first, c.opts[0]); err != nil { // the first push
		t.Fatal(err)
	}
	for i := 1; i < n; i++ {
		dir := filepath.Join(t.TempDir(), fmt.Sprint("s", i))
		if _, err := gitrepo.Clone(c.remote, dir); err != nil {
			t.Fatal(err)
		}
		s, err := store.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		c.add(s, i)
	}
	return c
}

func (c *cluster) add(s *store.Store, i int) {
	acct := fmt.Sprintf("github.com/u%d", i)
	c.stores = append(c.stores, s)
	c.opts = append(c.opts, Options{
		Remotes: []string{c.remote}, Author: gitrepo.Signature{Name: fmt.Sprint("u", i), Email: fmt.Sprintf("u%d@users.noreply.github.com", i)},
		Account: acct, Me: []string{acct}, Now: time.Now,
	})
}

func (c *cluster) sync(i int) Result {
	c.t.Helper()
	r, err := Run(c.stores[i], c.opts[i])
	if err != nil {
		c.t.Fatalf("sync %d: %v", i, err)
	}
	return r
}

func (c *cluster) op(i int, name string, task model.ULID) store.Op {
	return store.Op{Name: name, Task: task, Author: c.opts[i].Author, Account: c.opts[i].Account}
}

// addTask creates a root task in the first workspace of clone i.
func (c *cluster) addTask(i int, title string) *model.Task {
	c.t.Helper()
	s := c.stores[i]
	var ws model.ULID
	for w, err := range s.Workspaces() {
		if err == nil {
			ws = w.ULID
			break
		}
	}
	r, err := model.Between(c.last, "") // distinct ranks: rerank is not what these tests look at
	if err != nil {
		c.t.Fatal(err)
	}
	c.last = r
	x := &model.Task{ULID: model.NewULID(), Workspace: ws, Kind: "task", Title: title, Status: "todo", Rank: r}
	if _, err := s.Update(c.op(i, "add", x.ULID), func(tx *store.Tx) error { return tx.PutTask(x) }); err != nil {
		c.t.Fatal(err)
	}
	return x
}

func (c *cluster) set(i int, ulid model.ULID, fn func(*model.Task)) {
	c.t.Helper()
	s := c.stores[i]
	if _, err := s.Update(c.op(i, "update", ulid), func(tx *store.Tx) error {
		x, err := tx.Task(ulid)
		if err != nil {
			return err
		}
		fn(x)
		return tx.PutTask(x)
	}); err != nil {
		c.t.Fatal(err)
	}
}

// task reads the task from clone i; nil when it is not there.
func (c *cluster) task(i int, ulid model.ULID) *model.Task {
	x, err := c.stores[i].Task(ulid)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		c.t.Fatal(err)
	}
	return x
}

func (c *cluster) headTree(i int) plumbing.Hash {
	h, err := c.stores[i].Repo().HeadTree()
	if err != nil {
		c.t.Fatal(err)
	}
	return h
}

func mustHead(t *testing.T, s *store.Store) plumbing.Hash {
	t.Helper()
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}
