package sync

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"net/url"
	"reflect"
	"slices"
	"strings"
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
	ulid, kind string
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
			h.lost = append(h.lost, lost{p, w[1], value})
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
			h.repaired = append(h.repaired, repaired{u, w[1]})
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
// task came after w, knowing it. Two writes of the same value count as one,
// since a merge of equal values loses nothing: a write that came after
// either of them replaces both.
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

func (h *history) wasRepaired(u model.ULID) bool {
	return slices.ContainsFunc(h.repaired, func(r repaired) bool { return r.ulid == u.String() })
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
		if w.field == "purge" || h.purged(w, writes) {
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
			t := h.v.tasks[w.task]
			if t == nil {
				continue // purged
			}
			h.checkTaskWrite(ps, w, t, writes)
		}
	}
}

func (h *history) checkWorkspace(ps *problems, w write, writes []write) {
	ws := h.v.ws[w.task]
	if ws == nil {
		return // removed
	}
	field := strings.TrimPrefix(w.field, "ws.")
	final := js(ws.Name)
	if field == "id_prefix" {
		final = js(ws.IDPrefix)
	}
	if sameJSON(final, w.value) || h.replaced(w, writes) || h.reverts(w, writes) || h.wasRepaired(w.task) ||
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
		if !sameJSON(final, w.value) && !h.replaced(w, writes) && !h.wasRepaired(t.ULID) && !h.wasLost(path, w.field, w.value) && !h.reverts(w, writes) {
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
		x, err := model.DecodeTask(b)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range append([]string{x.ID}, x.FormerIDs...) {
			if id != "" && !slices.Contains(m[id], x.ULID) {
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
	seen := map[string]model.ULID{}
	for _, u := range sortedULIDs(v.tasks) {
		t := v.tasks[u]
		if o, ok := seen[t.ID]; ok {
			ps.add("tasks %s and %s have the id %s", o, u, t.ID)
		}
		seen[t.ID] = u
		ws := v.ws[t.Workspace]
		if ws == nil {
			ps.add("task %s (%s) is in workspace %s, which is not there", u, t.ID, t.Workspace)
		} else if p, _, ok := model.ParseTaskID(t.ID); !ok || p != ws.IDPrefix {
			ps.add("task %s has the id %q in workspace %s (%s)", u, t.ID, ws.Name, ws.IDPrefix)
		}
		if t.Parent == nil {
			continue
		}
		p := v.tasks[*t.Parent]
		switch {
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
