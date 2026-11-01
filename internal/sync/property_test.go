package sync

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"net/url"
	"path"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// TestPropertyConverges is the completion condition of the phase: three
// clones that edit at random, syncing at random, end up with the same tree
// and no value lost.
func TestPropertyConverges(t *testing.T) {
	rounds, steps := 30, 40
	if shortMode() {
		rounds, steps = 5, 20
	}
	for seed := int64(1); seed <= int64(rounds); seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			// Each seed has a cluster of its own in its own directories, and
			// the hooks of the package are not changed here, so the seeds run
			// side by side.
			t.Parallel()
			rnd := rand.New(rand.NewPCG(uint64(seed), 0))
			c := newCluster(t, 3)
			var log []string
			var writes []write
			// The workspace the store began with is written by the first commit.
			first := mustHead(t, c.stores[0])
			for u, w := range c.view(0).ws {
				writes = append(writes, write{0, first, u, "ws.name", js(w.Name)}, write{0, first, u, "ws.id_prefix", js(w.IDPrefix)})
			}
			for range steps {
				i := rnd.IntN(3)
				if rnd.IntN(4) == 0 {
					log = append(log, fmt.Sprintf("sync %d", i))
					c.sync(i)
					continue
				}
				o := ops[rnd.IntN(len(ops))]
				log = append(log, fmt.Sprintf("%s on %d", o.name, i))
				ws := o.do(c, i, rnd)
				log[len(log)-1] += fmt.Sprintf(" (%d writes)", len(ws))
				writes = append(writes, ws...)
			}
			// Several clones add without syncing: ids must collide.
			for i := range 3 {
				log = append(log, fmt.Sprintf("add on %d", i))
				writes = append(writes, opAdd(c, i, rnd)...)
			}
			settle(t, c)
			if err := checkAll(c, writes); err != nil {
				t.Fatalf("seed %d: %v\nops:\n  %s", seed, err, strings.Join(log, "\n  "))
			}
		})
	}
}

// TestPropertyConvergesStores is the same with two stores, a and b, of three
// clones each (clone i of a and of b are one user). Besides the operations of
// a store, a clone moves a task with everything under it from one store to
// the other. Each store converges, no write is lost, and what the forwards
// require holds: nothing live under a forward, nothing blocked by it, no
// comment or worklog entry left beneath it.
func TestPropertyConvergesStores(t *testing.T) {
	rounds, steps := 30, 60
	if shortMode() {
		rounds, steps = 5, 30
	}
	var moves, overwritten, parentMoved, refMoved atomic.Int64
	t.Cleanup(func() {
		t.Logf("moves %d, task files overwritten by a forward %d, parent_moved %d, ref_moved %d", moves.Load(), overwritten.Load(), parentMoved.Load(), refMoved.Load())
		if moves.Load() == 0 {
			t.Error("no task moved between stores in any seed")
		}
	})
	for seed := int64(1); seed <= int64(rounds); seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			t.Parallel()
			rnd := rand.New(rand.NewPCG(uint64(seed), 1))
			a, b := newCluster(t, 3), newCluster(t, 3)
			stores := []*cluster{a, b}
			writes := map[*cluster][]write{}
			for _, c := range stores {
				first := mustHead(t, c.stores[0])
				for u, w := range c.view(0).ws {
					writes[c] = append(writes[c], write{0, first, u, "ws.name", js(w.Name)}, write{0, first, u, "ws.id_prefix", js(w.IDPrefix)})
				}
			}
			var log []string
			for range steps {
				from, to := a, b
				if rnd.IntN(2) == 0 {
					from, to = b, a
				}
				i := rnd.IntN(3)
				name := map[*cluster]string{a: "a", b: "b"}
				switch r := rnd.IntN(8); {
				case r == 0:
					ws := opMoveStore(from, to, i, rnd)
					log = append(log, fmt.Sprintf("move %s to %s on %d (%d writes)", name[from], name[to], i, len(ws)))
					if len(ws) > 0 {
						moves.Add(1)
					}
					writes[to] = append(writes[to], ws...)
				case r < 3:
					log = append(log, fmt.Sprintf("sync %s %d", name[from], i))
					from.sync(i)
				default:
					o := moveOps[rnd.IntN(len(moveOps))]
					ws := o.do(from, i, rnd)
					log = append(log, fmt.Sprintf("%s on %s %d (%d writes)", o.name, name[from], i, len(ws)))
					writes[from] = append(writes[from], ws...)
				}
			}
			for _, c := range stores {
				for i := range 3 {
					writes[c] = append(writes[c], opAdd(c, i, rnd)...)
				}
				settle(t, c)
			}
			for n, c := range stores {
				if err := checkAll(c, writes[c]); err != nil {
					t.Fatalf("seed %d, store %c: %v\nops:\n  %s", seed, 'a'+n, err, strings.Join(log, "\n  "))
				}
			}
			// A task has one destination, so no ULID is live in both stores,
			// and every forward names the other store.
			va, vb := a.view(0), b.view(0)
			for u := range va.tasks {
				if vb.tasks[u] != nil {
					t.Fatalf("seed %d: task %s is in both stores", seed, u)
				}
			}
			for _, x := range []struct {
				from, to *cluster
				v        *view
			}{{a, b, va}, {b, a, vb}} {
				for u, f := range x.v.fwd {
					if f.MovedTo != x.to.stores[0].Meta().ULID {
						t.Fatalf("seed %d: the forward of %s names the store %s, not the other store of the test", seed, u, f.MovedTo)
					}
				}
			}
			for _, c := range stores {
				h := newHistory(c)
				for _, l := range h.lost {
					if l.field == "*" {
						overwritten.Add(1)
					}
				}
				for _, r := range h.repaired {
					switch r.kind {
					case "parent_moved":
						parentMoved.Add(1)
					case "ref_moved":
						refMoved.Add(1)
					}
				}
			}
		})
	}
}

// shortMode reports -short. It reads the flag because the pattern of
// TestNoShortIdentifiers matches the method of testing.
func shortMode() bool {
	f := flag.Lookup("test.short")
	return f != nil && f.Value.String() == "true"
}

// settle syncs the clones in turn until a whole round changes no HEAD.
func settle(t *testing.T, c *cluster) {
	t.Helper()
	heads := func() []plumbing.Hash {
		var out []plumbing.Hash
		for _, s := range c.stores {
			out = append(out, mustHead(t, s))
		}
		return out
	}
	for range 10 {
		before := heads()
		for i := range c.stores {
			c.sync(i)
		}
		if slices.Equal(before, heads()) {
			return
		}
	}
	t.Fatal("the clones still change after 10 rounds of sync")
}

// lost is a Hedgehug-Overwritten trailer.
type lost struct{ path, field, value string }

// repaired is a Hedgehug-Repaired trailer.
type repaired struct {
	ulid, kind, value string
	commit            plumbing.Hash
}

// problems collects what checkAll finds wrong.
type problems []string

func (p *problems) add(format string, args ...any) { *p = append(*p, fmt.Sprintf(format, args...)) }

func (p problems) err() error {
	if len(p) == 0 {
		return nil
	}
	if len(p) > 20 {
		p = append(p[:20:20], fmt.Sprintf("... and %d more", len(p)-20))
	}
	return errors.New(strings.Join(p, "\n"))
}

// checkAll checks the three conditions of the phase: the clones have the
// same tree, no value written is lost, and the invariants hold.
func checkAll(c *cluster, writes []write) error {
	var ps problems
	for i := 1; i < len(c.stores); i++ {
		if c.headTree(0) != c.headTree(i) {
			ps.add("the tree of clone %d differs from clone 0", i)
		}
	}
	if len(ps) > 0 {
		return ps.err()
	}
	h := newHistory(c)
	h.checkWrites(&ps, writes)
	h.checkInvariants(&ps)
	return ps.err()
}

// history is the final state of clone 0 with its history.
type history struct {
	c        *cluster
	repo     *gitrepo.Repo
	v        *view
	entries  map[plumbing.Hash]gitrepo.LogEntry
	lost     []lost
	repaired []repaired
	ancestor map[[2]plumbing.Hash]bool
	ids      map[plumbing.Hash]map[string][]model.ULID // tree → id → tasks holding it
}

func newHistory(c *cluster) *history {
	t := c.t
	t.Helper()
	h := &history{
		c: c, repo: c.stores[0].Repo(), v: c.view(0), entries: map[plumbing.Hash]gitrepo.LogEntry{},
		ancestor: map[[2]plumbing.Hash]bool{}, ids: map[plumbing.Hash]map[string][]model.ULID{},
	}
	log, err := h.repo.Log(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range log {
		h.entries[e.Hash] = e
		for _, v := range e.TrailerValues("Hedgehug-Overwritten") {
			w := strings.Fields(v)
			if len(w) < 4 {
				t.Fatalf("bad Hedgehug-Overwritten %q", v)
			}
			p, err := url.PathUnescape(w[0])
			if err != nil {
				t.Fatal(err)
			}
			// path, field, JSON (which may hold spaces), author
			value := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(v, w[0]+" "+w[1]+" "), " "+w[len(w)-1]))
			field, err := url.PathUnescape(w[1])
			if err != nil {
				t.Fatal(err)
			}
			h.lost = append(h.lost, lost{p, field, value})
		}
		for _, v := range e.TrailerValues("Hedgehug-Repaired") {
			w := strings.Fields(v)
			if len(w) < 4 {
				t.Fatalf("bad Hedgehug-Repaired %q", v)
			}
			u, err := url.PathUnescape(w[0])
			if err != nil {
				t.Fatal(err)
			}
			value := strings.Join(w[2:len(w)-1], " ")
			h.repaired = append(h.repaired, repaired{u, w[1], value, e.Hash})
		}
	}
	return h
}

func (h *history) isAncestor(a, b plumbing.Hash) bool {
	k := [2]plumbing.Hash{a, b}
	if v, ok := h.ancestor[k]; ok {
		return v
	}
	v, err := h.repo.IsAncestor(a, b)
	if err != nil {
		h.c.t.Fatal(err)
	}
	h.ancestor[k] = v
	return v
}

// replaced reports whether another write to the same field of the same
// task came after w, knowing it. Two writes of the same value count as one
// (spec: an equal value on both sides is not an overwrite, R58), so a write
// that came after either of them replaces both.
func (h *history) replaced(w write, writes []write) bool {
	return slices.ContainsFunc(writes, func(same write) bool {
		if same.task != w.task || same.field != w.field || !sameJSON(same.value, w.value) {
			return false
		}
		return slices.ContainsFunc(writes, func(o write) bool {
			return o.commit != same.commit && o.task == w.task && o.field == w.field && h.isAncestor(same.commit, o.commit)
		})
	})
}

// reverts reports whether w returns the field to a value it had before, in
// the history the writer started from (A to B and back to A, a restore after
// a delete). A three-way merge sees the base and the two sides only, so a
// side that went away from a value and came back is the side that did not
// change it, and the other side's value stays: such a write is a net-zero
// change, and it is not counted as lost.
func (h *history) reverts(w write, writes []write) bool {
	return slices.ContainsFunc(writes, func(o write) bool {
		return o.commit != w.commit && o.task == w.task && o.field == w.field && sameJSON(o.value, w.value) && h.isAncestor(o.commit, w.commit)
	})
}

// purged reports whether the task of w was purged by a write that came
// after w, knowing it. A purge takes the writes before it with the task, and
// a side that changed the task meanwhile gets its own version back.
func (h *history) purged(w write, writes []write) bool {
	return slices.ContainsFunc(writes, func(o write) bool {
		return o.field == "purge" && o.task == w.task && h.isAncestor(w.commit, o.commit)
	})
}

// repairFields are the fields a kind of repair or notice changes, as the
// writes name them.
var repairFields = map[string][]string{
	"parent_cycle":       {"parent"},
	"parent_deleted":     {"parent"},
	"parent_moved":       {"parent"},
	"workspace_mismatch": {"workspace", "id"},
	"id_collision":       {"id"},
	"id_renumbered":      {"id"},
	"revived":            {"deleted_at"},
	"workspace_revived":  {"ws.name", "ws.id_prefix"},
	"id_prefix":          {"ws.id_prefix"},
}

// wasRepaired reports whether a repair or notice of the task or workspace u
// touched the field.
func (h *history) wasRepaired(u model.ULID, field string) bool {
	return slices.ContainsFunc(h.repaired, func(r repaired) bool {
		return r.ulid == u.String() && slices.Contains(repairFields[r.kind], field)
	})
}

// sameJSON compares two JSON values, times by the instant.
func sameJSON(a, b string) bool {
	var x, y any
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return a == b
	}
	if s, ok := x.(string); ok {
		if t, ok := y.(string); ok {
			if ts, err := time.Parse(time.RFC3339Nano, s); err == nil {
				if tt, err := time.Parse(time.RFC3339Nano, t); err == nil {
					return ts.Equal(tt)
				}
			}
		}
	}
	return reflect.DeepEqual(x, y)
}

func (h *history) wasLost(path, field, value string) bool {
	return slices.ContainsFunc(h.lost, func(l lost) bool {
		return l.path == path && l.field == field && sameJSON(l.value, value)
	})
}

func (h *history) checkWrites(ps *problems, writes []write) {
	sc := h.c.stores[0].Schema()
	for _, w := range writes {
		if w.field == "purge" || w.field == "ws.rm" || h.purged(w, writes) {
			continue
		}
		if _, ok := h.entries[w.commit]; !ok {
			ps.add("%s of %s written by clone %d in %s is not in the history", w.field, w.task, w.clone, w.commit)
			continue
		}
		switch {
		case strings.HasPrefix(w.field, "schema."):
			if w.field == "schema.status" {
				var name string
				_ = json.Unmarshal([]byte(w.value), &name)
				if _, ok := sc.Status(name); !ok {
					ps.add("the status %s added by clone %d is not in schema.yaml", w.value, w.clone)
				}
			} else if name := strings.Trim(w.value, `"`); !slices.ContainsFunc(sc.Props, func(p schema.Prop) bool { return p.Name == name }) {
				ps.add("the prop %s added by clone %d is not in schema.yaml", w.value, w.clone)
			}
		case strings.HasPrefix(w.field, "ws."):
			h.checkWorkspace(ps, w, writes)
		default:
			if h.v.fwd[w.task] != nil {
				h.checkForwarded(ps, w)
				continue
			}
			t := h.v.tasks[w.task]
			if t == nil {
				if !slices.ContainsFunc(writes, func(o write) bool { return o.task == w.task && o.field == "purge" }) {
					ps.add("task %s is gone, and no clone purged it (%s %s)", w.task, w.field, w.value)
				}
				continue
			}
			h.checkTaskWrite(ps, w, t, writes)
		}
	}
}

// checkForwarded checks a write to a task that left the store for another.
// The write went with the task when the move-out knew of it; otherwise it was
// made beside the move and lost to the forward, which the merge records as
// the task file overwritten whole (Field "*").
func (h *history) checkForwarded(ps *problems, w write) {
	for _, e := range h.entries {
		if slices.Contains(e.TrailerValues("Hedgehug-Op"), "move-out") &&
			slices.Contains(e.TrailerValues("Hedgehug-Task"), w.task.String()) &&
			(e.Hash == w.commit || h.isAncestor(w.commit, e.Hash)) {
			return
		}
	}
	// A comment is a file of its own under the forward, dropped by itself.
	file := model.TaskPath(w.task)
	if w.field == "comment" {
		var c string
		_ = json.Unmarshal([]byte(w.value), &c)
		u, err := model.ParseULID(c)
		if err != nil {
			h.c.t.Fatal(err)
		}
		file = model.CommentPath(w.task, u)
	}
	if slices.ContainsFunc(h.lost, func(l lost) bool { return l.path == file && l.field == "*" }) {
		return
	}
	ps.add("task %s moved to another store, and %s %s written by clone %d beside the move is not in the move-out or in Hedgehug-Overwritten", w.task, w.field, w.value, w.clone)
}

func (h *history) checkWorkspace(ps *problems, w write, writes []write) {
	ws := h.v.ws[w.task]
	if ws == nil {
		if !slices.ContainsFunc(writes, func(o write) bool { return o.task == w.task && o.field == "ws.rm" }) {
			ps.add("workspace %s is gone, and no clone removed it (%s %s)", w.task, w.field, w.value)
		}
		return
	}
	field := strings.TrimPrefix(w.field, "ws.")
	final := js(ws.Name)
	if field == "id_prefix" {
		final = js(ws.IDPrefix)
	}
	if sameJSON(final, w.value) || h.replaced(w, writes) || h.reverts(w, writes) || h.wasRepaired(w.task, w.field) ||
		h.wasLost(model.WorkspacePath(w.task), field, w.value) {
		return
	}
	ps.add("workspace %s: %s %s written by clone %d is lost (final %s)", w.task, field, w.value, w.clone, final)
}

func (h *history) checkTaskWrite(ps *problems, w write, t *model.Task, writes []write) {
	path := model.TaskPath(t.ULID)
	lose := func() {
		ps.add("task %s (%s): %s %s written by clone %d is lost", t.ULID, t.ID, w.field, w.value, w.clone)
	}
	switch w.field {
	case "id":
		var id string
		_ = json.Unmarshal([]byte(w.value), &id)
		if t.ID != id && !slices.Contains(t.FormerIDs, id) {
			lose()
		}
		h.checkNotReused(ps, w, id)
	case "body":
		if !slices.Contains(strings.Split(t.Body, "\n"), w.value) && !h.replaced(w, writes) {
			lose()
		}
	case "labels":
		if !slices.Contains(t.Labels, w.value) && !h.replaced(w, writes) {
			lose()
		}
	case "comment":
		found := false
		for cm, err := range h.c.stores[0].Comments(t.ULID) {
			if err != nil {
				h.c.t.Fatal(err)
			}
			found = found || js(cm.ULID.String()) == w.value
		}
		if !found {
			lose()
		}
	default:
		var final string
		switch w.field {
		case "title":
			final = js(t.Title)
		case "status":
			final = js(t.Status)
		case "workspace":
			final = js(t.Workspace.String())
		case "parent":
			final = parentJSON(t.Parent)
		case "deleted_at":
			final = deletedJSON(t.DeletedAt)
		default:
			ps.add("unknown field %s", w.field)
			return
		}
		if !sameJSON(final, w.value) && !h.replaced(w, writes) && !h.wasRepaired(t.ULID, w.field) && !h.wasLost(path, w.field, w.value) && !h.reverts(w, writes) {
			ps.add("task %s (%s): %s %s written by clone %d is lost (final %s)", t.ULID, t.ID, w.field, w.value, w.clone, final)
		}
	}
}

// idsAt returns, for the tree of commit, the tasks that hold each id in
// their id or former ids.
func (h *history) idsAt(tree plumbing.Hash) map[string][]model.ULID {
	if m, ok := h.ids[tree]; ok {
		return m
	}
	t := h.c.t
	m := map[string][]model.ULID{}
	for p, err := range h.repo.Files(tree, "tasks/") {
		if err != nil {
			t.Fatal(err)
		}
		b, err := h.repo.ReadFile(tree, p)
		if err != nil {
			t.Fatal(err)
		}
		if model.IsForward(b) {
			continue
		}
		x, err := model.DecodeTask(b)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range append([]string{x.ID}, x.FormerIDs...) {
			if id != "" && !slices.Contains(m[id], x.ULID) && !slices.Contains(h.c.foreign[x.ULID], id) {
				m[id] = append(m[id], x.ULID)
			}
		}
	}
	h.ids[tree] = m
	return m
}

// checkNotReused checks that the id a write gave was not held by another
// task in the state the writer started from, and that its number is above
// the numbers of purged tasks.
func (h *history) checkNotReused(ps *problems, w write, id string) {
	e := h.entries[w.commit]
	if len(e.Parents) == 0 {
		return
	}
	tree, err := h.repo.TreeOf(e.Parents[0])
	if err != nil {
		h.c.t.Fatal(err)
	}
	for _, u := range h.idsAt(tree)[id] {
		if u != w.task {
			ps.add("task %s was given %s by clone %d in %s (%s), which task %s already had", w.task, id, w.clone, w.commit.String()[:7], e.Subject, u)
		}
	}
	b, err := h.repo.ReadFile(tree, model.NumbersPath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		h.c.t.Fatal(err)
	}
	nums, err := model.DecodeNumbers(b)
	if err != nil {
		h.c.t.Fatal(err)
	}
	if p, n, ok := model.ParseTaskID(id); ok && n <= nums[p] {
		ps.add("task %s was given %s by clone %d, but numbers.yaml records %s up to %d", w.task, id, w.clone, p, nums[p])
	}
}

func (h *history) checkInvariants(ps *problems) {
	v := h.v
	tree, err := h.repo.HeadTree()
	if err != nil {
		h.c.t.Fatal(err)
	}
	for _, u := range sortedULIDs(v.fwd) {
		for _, p := range []string{model.CommentPath(u, model.ULID{}), model.WorklogPath(u, model.ULID{})} {
			for f, err := range h.repo.Files(tree, path.Dir(p)+"/") {
				if err != nil {
					h.c.t.Fatal(err)
				}
				ps.add("%s is left under %s, which moved to another store", f, u)
			}
		}
	}
	seen := map[string]model.ULID{}
	for _, u := range sortedULIDs(v.tasks) {
		t := v.tasks[u]
		if o, ok := seen[t.ID]; ok {
			ps.add("tasks %s and %s have the id %s", o, u, t.ID)
		}
		seen[t.ID] = u
		for _, o := range sortedULIDs(v.tasks) {
			if x := v.tasks[o]; o != u && x.ID != "" && slices.Contains(t.FormerIDs, x.ID) && !h.collided(u, x.ID) && !h.givenTogether(u, o, x.ID) && !slices.Contains(h.c.foreign[u], x.ID) {
				ps.add("task %s (%s) has %s, which is the id of task %s, in its former ids, and no id_collision explains it", u, t.ID, x.ID, o)
			}
		}
		ws := v.ws[t.Workspace]
		if ws == nil {
			ps.add("task %s (%s) is in workspace %s, which is not there", u, t.ID, t.Workspace)
		} else if p, _, ok := model.ParseTaskID(t.ID); !ok || p != ws.IDPrefix {
			ps.add("task %s has the id %q in workspace %s (%s)", u, t.ID, ws.Name, ws.IDPrefix)
		}
		for _, b := range t.BlockedBy {
			if v.fwd[b] != nil {
				ps.add("task %s (%s) is blocked by %s, which moved to another store", u, t.ID, b)
			}
		}
		if t.Parent == nil {
			continue
		}
		p := v.tasks[*t.Parent]
		switch {
		case v.fwd[*t.Parent] != nil:
			if t.DeletedAt == nil {
				ps.add("task %s (%s) is live under %s, which moved to another store", u, t.ID, *t.Parent)
			}
		case p == nil:
			if t.DeletedAt == nil {
				ps.add("task %s (%s) has the parent %s, which is purged", u, t.ID, *t.Parent)
			}
		case p.Workspace != t.Workspace:
			ps.add("task %s (%s) is in a workspace other than its parent %s", u, t.ID, p.ID)
		case p.DeletedAt != nil && t.DeletedAt == nil:
			ps.add("task %s (%s) is live under the deleted %s", u, t.ID, p.ID)
		}
		for x, steps := t.Parent, 0; x != nil; steps++ {
			if *x == u || steps > len(v.tasks) {
				ps.add("task %s (%s) is in a parent cycle", u, t.ID)
				break
			}
			q := v.tasks[*x]
			if q == nil {
				break
			}
			x = q.Parent
		}
	}
}

// givenTogether reports whether the tasks a and b were each given id by
// clones that did not know of the other's. One of them was renumbered (a
// move to another workspace, a new prefix) before the merge met the other
// holder, so no duplicate id was left for a repair to see, and the id stays
// in the former ids of the one and the id of the other.
func (h *history) givenTogether(a, b model.ULID, id string) bool {
	for _, ca := range h.introduced(a, id) {
		for _, cb := range h.introduced(b, id) {
			if !h.isAncestor(ca, cb) && !h.isAncestor(cb, ca) {
				return true
			}
		}
	}
	return false
}

// introduced returns the commits that give the task the id: it is the task's
// id there and was not in any parent.
func (h *history) introduced(task model.ULID, id string) []plumbing.Hash {
	idAt := func(c plumbing.Hash) string {
		b, err := h.repo.ReadAt(c, model.TaskPath(task))
		if errors.Is(err, fs.ErrNotExist) {
			return ""
		}
		if err != nil {
			h.c.t.Fatal(err)
		}
		if model.IsForward(b) {
			return ""
		}
		x, err := model.DecodeTask(b)
		if err != nil {
			h.c.t.Fatal(err)
		}
		return x.ID
	}
	var out []plumbing.Hash
	for _, e := range h.entries {
		if idAt(e.Hash) == id && !slices.ContainsFunc(e.Parents, func(p plumbing.Hash) bool { return idAt(p) == id }) {
			out = append(out, e.Hash)
		}
	}
	return out
}

// collided reports whether a repair renumbered the task from id because
// another task had it too: an id_collision, or an id_renumbered when the
// task also had a prefix other than its workspace's.
func (h *history) collided(task model.ULID, id string) bool {
	return slices.ContainsFunc(h.repaired, func(r repaired) bool {
		if r.ulid != task.String() {
			return false
		}
		switch r.kind {
		case "id_collision":
			var v struct{ Old string }
			return json.Unmarshal([]byte(r.value), &v) == nil && v.Old == id
		case "id_renumbered":
			var old string
			return json.Unmarshal([]byte(r.value), &old) == nil && old == id
		}
		return false
	})
}
