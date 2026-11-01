package sync

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/merge"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func (c *cluster) comment(i int, task model.ULID, body string, id model.ULID) {
	c.t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := c.stores[i].Update(c.op(i, "comment", task), func(tx *store.Tx) error {
		return tx.PutComment(task, &model.Comment{ULID: id, Author: "u", Created: now, Updated: now, Body: body})
	}); err != nil {
		c.t.Fatal(err)
	}
}

// A task that clone 0 moved to another store, which clone 1 edited,
// commented on, gave a child and a blocker: the forward wins, what clone 1
// did to the task is recorded, and the child and the blocker are repaired.
func TestSyncForwardWinsAndRepairs(t *testing.T) {
	c := newCluster(t, 2)
	x := c.addTask(0, "x")
	z := c.addTask(0, "z")
	old := model.NewULID()
	c.comment(0, x.ULID, "old\n", old)
	c.sync(0)
	c.sync(1)

	dest := model.NewULID()
	if _, err := c.stores[0].Update(c.op(0, "move-out", x.ULID), func(tx *store.Tx) error {
		return tx.MoveOut(store.MoveOut{To: dest, Tasks: []model.ULID{x.ULID}})
	}); err != nil {
		t.Fatal(err)
	}

	c.set(1, x.ULID, func(t *model.Task) { t.Title = "edited" })
	c.comment(1, x.ULID, "edited old\n", old) // a comment from before the move, changed
	fresh := model.NewULID()
	c.comment(1, x.ULID, "new\n", fresh)
	child := c.addTask(1, "child")
	c.set(1, child.ULID, func(t *model.Task) { p := x.ULID; t.Parent = &p })
	c.set(1, z.ULID, func(t *model.Task) { t.BlockedBy = []model.ULID{x.ULID} })

	c.sync(0)
	r := c.sync(1)
	c.sync(0)
	if c.headTree(0) != c.headTree(1) {
		t.Fatal("trees differ")
	}

	b, err := c.stores[1].Repo().ReadFile(c.headTree(1), model.TaskPath(x.ULID))
	if err != nil || !model.IsForward(b) {
		t.Fatalf("x is not a forward: %q %v", b, err)
	}
	for _, id := range []model.ULID{old, fresh} {
		if _, err := c.stores[1].Repo().ReadFile(c.headTree(1), model.CommentPath(x.ULID, id)); err == nil {
			t.Errorf("comment %s stays", id)
		}
	}
	if got := c.task(1, child.ULID); got.Parent != nil {
		t.Errorf("child parent = %v", got.Parent)
	}
	if got := c.task(1, z.ULID); len(got.BlockedBy) != 0 {
		t.Errorf("blocked_by = %v", got.BlockedBy)
	}

	// What clone 1 did to the moved task is told as lost, whole, by file.
	lostPaths := map[string]string{}
	for _, o := range r.Overwritten {
		if o.Field == "*" {
			lostPaths[o.Path] = o.Value
		}
	}
	for _, p := range []string{model.TaskPath(x.ULID), model.CommentPath(x.ULID, old), model.CommentPath(x.ULID, fresh)} {
		if lostPaths[p] == "" {
			t.Errorf("%s is not told as overwritten: %+v", p, r.Overwritten)
		}
	}
	if !strings.Contains(lostPaths[model.TaskPath(x.ULID)], "edited") || !strings.Contains(lostPaths[model.CommentPath(x.ULID, fresh)], "new") {
		t.Errorf("lost values: %+v", lostPaths)
	}
	kinds := map[string]merge.Repaired{}
	for _, x := range r.Repaired {
		kinds[x.Kind] = x
	}
	if kinds["parent_moved"].ULID != child.ULID || kinds["ref_moved"].ULID != z.ULID {
		t.Errorf("repaired = %+v", r.Repaired)
	}
	// The comment changed against the move was not revived for good.
	for _, n := range r.Notices {
		if n.Kind == "revived" {
			t.Errorf("notice for the removed comment: %+v", n)
		}
	}
	e, err := c.stores[1].Repo().LogFrom(mustHead(t, c.stores[1]), 5)
	if err != nil {
		t.Fatal(err)
	}
	var trailers []string
	for _, l := range e {
		for _, tr := range l.Trailers {
			trailers = append(trailers, tr.Key+": "+tr.Value)
		}
	}
	for _, want := range []string{"Hedgehug-Repaired: " + child.ULID.String() + " parent_moved", "Hedgehug-Repaired: " + z.ULID.String() + " ref_moved"} {
		if !slices.ContainsFunc(trailers, func(s string) bool { return strings.HasPrefix(s, want) }) {
			t.Errorf("no trailer %q in %q", want, trailers)
		}
	}
}

// A forward with a comment under it left in HEAD is removed by the repair of
// HEAD, as a sync does it after a merge made by hand.
func TestRepairHeadDropsFilesUnderForward(t *testing.T) {
	c := newCluster(t, 1)
	x := c.addTask(0, "x")
	if _, err := c.stores[0].Update(c.op(0, "move-out", x.ULID), func(tx *store.Tx) error {
		return tx.MoveOut(store.MoveOut{To: model.NewULID(), Tasks: []model.ULID{x.ULID}})
	}); err != nil {
		t.Fatal(err)
	}
	c.comment(0, x.ULID, "late\n", model.NewULID())
	if _, err := RepairHead(c.stores[0], c.opts[0]); err != nil {
		t.Fatal(err)
	}
	for p, err := range c.stores[0].Repo().Files(c.headTree(0), "comments/") {
		if err != nil {
			t.Fatal(err)
		}
		t.Errorf("%s stays", p)
	}
	e, err := c.stores[0].Repo().LogFrom(mustHead(t, c.stores[0]), 1)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := e[0].Trailer("Hedgehug-Overwritten")
	if !ok || !strings.Contains(v, " * ") {
		t.Errorf("trailer = %q", v)
	}
}
