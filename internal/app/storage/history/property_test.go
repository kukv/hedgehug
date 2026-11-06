package history_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// TestPropertyMatchesNaive builds random histories (branches, merges, equal
// and increasing author times, main moving about) and checks that the index,
// built step by step and built at once, answers as a walk that compares every
// commit with its parents does.
func TestPropertyMatchesNaive(t *testing.T) {
	seeds, steps := 30, 40
	if testing.Short() {
		seeds, steps = 6, 20
	}
	for seed := 1; seed <= seeds; seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			t.Parallel()
			runProperty(t, seed, steps)
		})
	}
}

func runProperty(t *testing.T, seed, steps int) {
	rnd := rand.New(rand.NewPCG(uint64(seed), 0))
	s := newStore(t)
	p := newPlumber(t, s)
	u1, u2, c1 := model.NewULID(), model.NewULID(), model.NewULID()
	paths := []string{model.TaskPath(u1), model.TaskPath(u2), model.CommentPath(u1, c1), "schema.yaml"}

	var ops []string
	fail := func(format string, args ...any) {
		t.Helper()
		t.Fatalf("seed %d: %s\nops:\n  %s", seed, fmt.Sprintf(format, args...), strings.Join(ops, "\n  "))
	}

	trees := map[plumbing.Hash]map[string]string{}
	var heads []plumbing.Hash // the tips of the branches
	clock := 0
	next := 0
	tick := func() time.Time {
		if rnd.IntN(8) == 0 {
			// Clock skew: older than what was written before, so a child can
			// be older than its parent and When order disagrees with Gen order.
			return t0.Add(time.Duration(clock-1-rnd.IntN(5)) * time.Second)
		}
		if rnd.IntN(2) == 0 {
			clock += 1 + rnd.IntN(3)
		}
		return t0.Add(time.Duration(clock) * time.Second)
	}
	content := func() string { next++; return fmt.Sprint("v", next) }
	mk := func(parents []plumbing.Hash, files map[string]string) plumbing.Hash {
		h := p.commit(tick(), parents, files)
		trees[h] = files
		return h
	}

	root := map[string]string{}
	root[paths[rnd.IntN(len(paths))]] = content()
	heads = append(heads, mk(nil, root))
	ops = append(ops, fmt.Sprintf("root %s", heads[0].String()[:7]))

	var h *history.History
	var main plumbing.Hash
	open := func() {
		t.Helper()
		var err error
		if h, err = history.Open(s, history.Options{}); err != nil {
			fail("open: %v", err)
		}
	}
	check := func(label string, h *history.History, head plumbing.Hash) {
		t.Helper()
		if h.Tip() != head {
			fail("%s: tip %v, want %v", label, h.Tip(), head)
		}
		if got, want := hashes(h.All()), naive(t, s.Repo(), head); !slices.Equal(got, want) {
			fail("%s: All\n got %v\nwant %v", label, got, want)
		}
		for _, q := range paths {
			if got, want := hashes(h.Path(q)), naive(t, s.Repo(), head, q); !slices.Equal(got, want) {
				fail("%s: Path(%s)\n got %v\nwant %v", label, q, got, want)
			}
		}
		if got, want := hashes(h.Task(u1)), naive(t, s.Repo(), head, paths[0], paths[2]); !slices.Equal(got, want) {
			fail("%s: Task(u1)\n got %v\nwant %v", label, got, want)
		}
		if got, want := hashes(h.Task(u2)), naive(t, s.Repo(), head, paths[1]); !slices.Equal(got, want) {
			fail("%s: Task(u2)\n got %v\nwant %v", label, got, want)
		}
		reach := naive(t, s.Repo(), head)
		// AtOrBefore answers as a walk down the first parents does.
		for range 4 {
			from := reach[rnd.IntN(len(reach))]
			at := t0.Add(time.Duration(rnd.IntN(clock+3)-1) * time.Second)
			want, wantOK := plumbing.ZeroHash, false
			if err := s.Repo().WalkFirstParents(from, func(e gitrepo.LogEntry) bool {
				if !e.When.After(at) {
					want, wantOK = e.Hash, true
					return false
				}
				return true
			}); err != nil {
				fail("%s: walk: %v", label, err)
			}
			if got, ok, err := h.AtOrBefore(from, at); err != nil || ok != wantOK || got != want {
				fail("%s: AtOrBefore(%v, %v) = %v, %v, %v; want %v, %v", label, from, at, got, ok, err, want, wantOK)
			}
		}
		for range 4 {
			from := reach[rnd.IntN(len(reach))]
			q := paths[rnd.IntN(len(paths))]
			got, ok, err := h.LastChange(q, from)
			want := naive(t, s.Repo(), from, q)
			if err != nil || ok != (len(want) > 0) || (ok && got.Hash != want[0]) {
				fail("%s: LastChange(%s, %v) = %v, %v, %v; want %v", label, q, from, got.Hash, ok, err, want)
			}
		}
	}

	for range steps {
		switch r := rnd.IntN(10); {
		case r < 5 || len(heads) < 2: // a commit on a branch
			b := rnd.IntN(len(heads))
			files := map[string]string{}
			for k, v := range trees[heads[b]] {
				files[k] = v
			}
			for range 1 + rnd.IntN(2) {
				q := paths[rnd.IntN(len(paths))]
				if old, ok := files[q]; ok && rnd.IntN(5) == 0 {
					files[q] = old // writes the same content: no change
					continue
				}
				files[q] = content()
			}
			c := mk([]plumbing.Hash{heads[b]}, files)
			ops = append(ops, fmt.Sprintf("commit %s on branch %d (parent %s)", c.String()[:7], b, heads[b].String()[:7]))
			heads[b] = c
		case r < 6 && len(heads) < 3: // a new branch from an existing commit
			b := rnd.IntN(len(heads))
			ops = append(ops, fmt.Sprintf("branch %d from %s", len(heads), heads[b].String()[:7]))
			heads = append(heads, heads[b])
		case r < 8: // a merge
			a, b := rnd.IntN(len(heads)), rnd.IntN(len(heads))
			if heads[a] == heads[b] {
				continue
			}
			ta, tb := trees[heads[a]], trees[heads[b]]
			files := map[string]string{}
			how := rnd.IntN(4)
			for _, q := range paths {
				va, oka := ta[q]
				vb, okb := tb[q]
				switch how {
				case 0: // a's
					if oka {
						files[q] = va
					}
				case 1: // b's
					if okb {
						files[q] = vb
					}
				case 2: // each file from a random side (the other when one lacks it)
					switch {
					case oka && okb && rnd.IntN(2) == 0, !oka && okb:
						files[q] = vb
					case oka:
						files[q] = va
					}
				default: // new content where either has the file
					if oka || okb {
						files[q] = content()
					}
				}
			}
			c := mk([]plumbing.Hash{heads[a], heads[b]}, files)
			ops = append(ops, fmt.Sprintf("merge %s of branch %d (%s) and %d (%s), how %d", c.String()[:7], a, heads[a].String()[:7], b, heads[b].String()[:7], how))
			heads[a] = c
		default: // main moves, the index follows
			main = heads[rnd.IntN(len(heads))]
			ops = append(ops, fmt.Sprintf("main to %s, open", main.String()[:7]))
			setMain(t, s, main)
			open()
			check("step", h, main)
		}
	}

	main = heads[rnd.IntN(len(heads))]
	ops = append(ops, fmt.Sprintf("main to %s, open (final)", main.String()[:7]))
	setMain(t, s, main)
	open()
	check("incremental", h, main)

	if err := os.Remove(filepath.Join(s.Dir(), ".hedgehug", "local", "history")); err != nil {
		fail("remove the index: %v", err)
	}
	open()
	check("at once", h, main)
}

// naive answers what the index should: every commit reachable from head
// (changing any of paths, when there are some) by comparing the blob of each
// path in the commit with its parents' (ReadAt), ordered by the rule of the
// index.
func naive(t *testing.T, r *gitrepo.Repo, head plumbing.Hash, paths ...string) []plumbing.Hash {
	t.Helper()
	type node struct {
		parents []plumbing.Hash
		when    time.Time
		gen     uint32
	}
	nodes := map[plumbing.Hash]*node{}
	var order []plumbing.Hash // parents before children
	var visit func(h plumbing.Hash)
	visit = func(h plumbing.Hash) {
		if _, ok := nodes[h]; ok {
			return
		}
		e, err := r.Entry(h)
		if err != nil {
			t.Fatal(err)
		}
		n := &node{parents: e.Parents, when: e.When, gen: 1}
		nodes[h] = n
		for _, p := range e.Parents {
			visit(p)
			n.gen = max(n.gen, nodes[p].gen+1)
		}
		order = append(order, h)
	}
	visit(head)

	read := func(c plumbing.Hash, p string) (string, bool) {
		b, err := r.ReadAt(c, p)
		if errors.Is(err, fs.ErrNotExist) {
			return "", false
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(b), true
	}
	changed := func(c plumbing.Hash, p string) bool {
		v, ok := read(c, p)
		ps := nodes[c].parents
		if len(ps) == 0 {
			return ok
		}
		for _, par := range ps {
			pv, pok := read(par, p)
			if pok == ok && pv == v {
				return false
			}
		}
		return true
	}

	var out []plumbing.Hash
	for _, c := range order {
		if len(paths) == 0 || slices.ContainsFunc(paths, func(p string) bool { return changed(c, p) }) {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b plumbing.Hash) int {
		na, nb := nodes[a], nodes[b]
		switch {
		case !na.when.Equal(nb.when):
			return nb.when.Compare(na.when)
		case na.gen != nb.gen:
			return int(nb.gen) - int(na.gen)
		}
		return bytes.Compare(b[:], a[:])
	})
	return out
}
