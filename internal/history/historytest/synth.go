// Package historytest makes big stores for measuring the history index and
// the pack consolidation. It is used by the benchmarks of internal/history
// only (make bench-history).
package historytest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/oklog/ulid/v2"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// Prefix is the id prefix of the workspace of a synthesized store: its tasks
// are ME-1, ME-2 and so on.
const Prefix = "ME"

// Accounts are the people who wrote the synthesized commits. Alice's last
// commit is UndoWindow commits back; the ones after it are Bob's and Carol's.
var Accounts = []config.Account{
	{Name: "alice", Host: "github.com", Login: "alice"},
	{Name: "bob", Host: "github.com", Login: "bob"},
	{Name: "carol", Host: "github.com", Login: "carol"},
}

// UndoWindow is how many commits back Alice's last commit is (the newest
// commit is 1).
const UndoWindow = 100

// layout is the version of what Synth makes; it is part of the name of the
// directory, so that a store of an older layout is not used again.
const layout = 3

// Start is the time of the first commit; each next one is a minute later.
var Start = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// CacheDir is where synthesized stores are kept: $HH_BENCH_CACHE, or
// hh-bench in the user cache directory (~/.cache on Linux).
func CacheDir(tb testing.TB) string {
	tb.Helper()
	if d := os.Getenv("HH_BENCH_CACHE"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil {
		tb.Fatal(err)
	}
	return filepath.Join(base, "hh-bench")
}

// Synth returns the directory of a store of the given number of commits
// that change one of tasks tasks each (5% write a comment, 2% are merges of
// two branches). It is made once and kept in CacheDir as
// c<commits>-t<tasks>-s<seed>-v<layout>; the same arguments give the same store. The
// store has no history index and a clean work tree. Do not change it: Clone
// it first.
func Synth(tb testing.TB, commits, tasks int, seed uint64) string {
	tb.Helper()
	cache := CacheDir(tb)
	dir := filepath.Join(cache, fmt.Sprintf("c%d-t%d-s%d-v%d", commits, tasks, seed, layout))
	if _, err := os.Stat(filepath.Join(dir, ".done")); err == nil {
		return filepath.Join(dir, "store")
	}
	if err := os.MkdirAll(cache, 0o750); err != nil {
		tb.Fatal(err)
	}
	tmp, err := os.MkdirTemp(cache, "tmp-")
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmp) }() // gone once renamed; else nothing is left of a stop half way
	if err := synth(filepath.Join(tmp, "store"), commits, tasks, seed); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".done"), nil, 0o600); err != nil {
		tb.Fatal(err)
	}
	_ = os.RemoveAll(dir) // an old half-made one
	if err := os.Rename(tmp, dir); err != nil {
		tb.Fatal(err)
	}
	return filepath.Join(dir, "store")
}

// Clone copies the store src to dst. The files of the packs are linked, not
// copied, when the file system allows (a pack is never changed once written).
func Clone(tb testing.TB, src, dst string) {
	tb.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o750)
		}
		if strings.HasSuffix(rel, ".pack") || strings.HasSuffix(rel, ".idx") {
			if os.Link(p, to) == nil { //nolint:gosec // our own copy of a store
				return nil
			}
		}
		return copyFile(p, to)
	})
	if err != nil {
		tb.Fatal(err)
	}
}

func copyFile(from, to string) error {
	in, err := os.Open(from) //nolint:gosec // a store of the benchmark
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm()) //nolint:gosec // a store of the benchmark
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// entry is one entry of a tree being made.
type entry struct {
	name string
	hash plumbing.Hash
	dir  bool
}

// maker builds objects in memory, with trees that are never changed: a change
// makes a new tree on the path to the file.
type maker struct {
	repo   *git.Repository
	mem    *memory.Storage
	trees  map[plumbing.Hash][]entry
	hashes []plumbing.Hash
	seen   map[plumbing.Hash]bool
}

func (m *maker) put(o *plumbing.MemoryObject) (plumbing.Hash, error) {
	h, err := m.mem.SetEncodedObject(o)
	if err != nil {
		return h, err
	}
	if !m.seen[h] {
		m.seen[h] = true
		m.hashes = append(m.hashes, h)
	}
	return h, nil
}

func (m *maker) blob(data []byte) (plumbing.Hash, error) {
	o := &plumbing.MemoryObject{}
	o.SetType(plumbing.BlobObject)
	_, _ = o.Write(data)
	return m.put(o)
}

func (m *maker) load(h plumbing.Hash) ([]entry, error) {
	if h.IsZero() {
		return nil, nil
	}
	if es, ok := m.trees[h]; ok {
		return es, nil
	}
	t, err := object.GetTree(m.repo.Storer, h)
	if err != nil {
		return nil, err
	}
	es := make([]entry, len(t.Entries))
	for i, e := range t.Entries {
		es[i] = entry{e.Name, e.Hash, e.Mode == filemode.Dir}
	}
	m.trees[h] = es
	return es, nil
}

func sortKey(e entry) string {
	if e.dir {
		return e.name + "/"
	}
	return e.name
}

// set returns the tree root with the file at parts holding the blob.
func (m *maker) set(root plumbing.Hash, parts []string, blob plumbing.Hash) (plumbing.Hash, error) {
	old, err := m.load(root)
	if err != nil {
		return root, err
	}
	es := append([]entry(nil), old...)
	i := -1
	for j, e := range es {
		if e.name == parts[0] {
			i = j
		}
	}
	var next entry
	if len(parts) == 1 {
		next = entry{parts[0], blob, false}
	} else {
		var child plumbing.Hash
		if i >= 0 {
			child = es[i].hash
		}
		sub, err := m.set(child, parts[1:], blob)
		if err != nil {
			return root, err
		}
		next = entry{parts[0], sub, true}
	}
	if i >= 0 {
		es[i] = next
	} else {
		es = append(es, next)
	}
	sort.Slice(es, func(a, b int) bool { return sortKey(es[a]) < sortKey(es[b]) })
	t := &object.Tree{}
	for _, e := range es {
		mode := filemode.Regular
		if e.dir {
			mode = filemode.Dir
		}
		t.Entries = append(t.Entries, object.TreeEntry{Name: e.name, Mode: mode, Hash: e.hash})
	}
	o := &plumbing.MemoryObject{}
	if err := t.Encode(o); err != nil {
		return root, err
	}
	h, err := m.put(o)
	if err != nil {
		return root, err
	}
	m.trees[h] = es
	return h, nil
}

type author struct {
	acct config.Account
	sig  object.Signature
}

func (m *maker) commit(tree plumbing.Hash, parents []plumbing.Hash, a author, when time.Time, msg string) (plumbing.Hash, error) {
	sig := object.Signature{Name: a.sig.Name, Email: a.sig.Email, When: when}
	c := &object.Commit{Author: sig, Committer: sig, Message: msg, TreeHash: tree, ParentHashes: parents}
	o := &plumbing.MemoryObject{}
	if err := c.Encode(o); err != nil {
		return plumbing.ZeroHash, err
	}
	return m.put(o)
}

func newULID(r *rand.Rand, when time.Time) (model.ULID, error) {
	var e [10]byte
	for i := range e {
		e[i] = byte(r.Uint32()) //nolint:gosec // the low byte
	}
	u, err := ulid.New(ulid.Timestamp(when), strings.NewReader(string(e[:])))
	if err != nil {
		return model.ULID{}, err
	}
	return model.ParseULID(u.String())
}

func message(op, label string, task model.ULID, acct string) string {
	var b strings.Builder
	b.WriteString("hedgehug: " + op)
	if label != "" {
		b.WriteString(" " + label)
	}
	b.WriteString("\n\nHedgehug-Op: " + op + "\n")
	if acct != "" {
		b.WriteString("Hedgehug-Author: " + acct + "\n")
	}
	if !task.IsZero() {
		b.WriteString("Hedgehug-Task: " + task.String() + "\n")
	}
	return b.String()
}

type taskState struct {
	t    *model.Task
	path string
}

//nolint:gocyclo,gocognit,cyclop,funlen,maintidx // one straight sequence: make the objects, write them, check out
func synth(dir string, commits, tasks int, seed uint64) error {
	if tasks < 2 || commits < tasks+2*UndoWindow {
		return fmt.Errorf("a synthesized store needs at least %d commits for %d tasks", tasks+2*UndoWindow, tasks)
	}
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // not a secret
	alice := Accounts[0]
	ws := &model.Workspace{ULID: model.NewULID(), Name: "me", IDPrefix: Prefix}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "me"}, nil, ws, config.CommitAuthor(alice), alice.ID(), func() time.Time { return Start })
	if err != nil {
		return err
	}
	if err := s.Repo().Close(); err != nil {
		return err
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return err
	}
	m := &maker{repo: repo, mem: memory.NewStorage(), trees: map[plumbing.Hash][]entry{}, seen: map[plumbing.Hash]bool{}}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	tip := head.Hash()
	ic, err := repo.CommitObject(tip)
	if err != nil {
		return err
	}
	tree := ic.TreeHash

	authors := make([]author, len(Accounts))
	for i, a := range Accounts {
		c := config.CommitAuthor(a)
		authors[i] = author{a, object.Signature{Name: c.Name, Email: c.Email}}
	}
	ranks, err := model.NBetween("", "", tasks)
	if err != nil {
		return err
	}
	// The first tasks are projects and the others are tasks under them, about
	// 100 each, as in a store that is used: a write reads the siblings of the
	// task it writes, so tasks all at the top level would measure that.
	projects := max(1, tasks/100)
	states := make([]taskState, 0, tasks)
	statuses := []string{"todo", "in_progress", "done", "backlog", "waiting"}
	slot := 0 // commits made, the side commits of merges included
	when := func() time.Time { return Start.Add(time.Duration(slot+1) * time.Minute) }
	undoTask := tasks / 2 // the task of Alice's commit UndoWindow back; nobody touches it after

	writeTask := func(base plumbing.Hash, ts taskState) (plumbing.Hash, error) {
		data, err := ts.t.Encode()
		if err != nil {
			return base, err
		}
		b, err := m.blob(data)
		if err != nil {
			return base, err
		}
		return m.set(base, strings.Split(ts.path, "/"), b)
	}
	edit := func(i int, at time.Time) {
		t := states[i].t
		t.Status = statuses[r.IntN(len(statuses))]
		t.Title = fmt.Sprintf("Task %d rev %d", i+1, r.IntN(1000000))
		t.Body = fmt.Sprintf("Notes %d\n", r.IntN(1000000))
		t.Updated = at
	}
	pick := func() int {
		for {
			i := r.IntN(len(states))
			if i != undoTask {
				return i
			}
		}
	}
	for slot < commits {
		left := commits - slot // commits to make, this one included
		at := when()
		parent := []plumbing.Hash{tip}
		var (
			op, label string
			task      model.ULID
			next      plumbing.Hash
		)
		a := authors[r.IntN(len(authors))]
		if left < UndoWindow {
			a = authors[1+r.IntN(2)] // not Alice
		}
		switch {
		case len(states) < tasks: // add the tasks first
			u, err := newULID(r, at)
			if err != nil {
				return err
			}
			i := len(states)
			ts := taskState{&model.Task{
				ULID: u, ID: fmt.Sprintf("%s-%d", Prefix, i+1), Workspace: ws.ULID, Kind: "task", Title: fmt.Sprintf("Task %d", i+1),
				Rank: ranks[i], Status: "todo", Created: at, CreatedBy: a.acct.ID(), Updated: at, Body: "Notes\n",
			}, model.TaskPath(u)}
			if i < projects {
				ts.t.Kind = "project"
			} else {
				p := states[r.IntN(projects)].t.ULID
				ts.t.Parent = &p
			}
			states = append(states, ts)
			if next, err = writeTask(tree, ts); err != nil {
				return err
			}
			op, label, task = "add", ts.t.ID, u
		case left == UndoWindow:
			a = authors[0]
			edit(undoTask, at)
			ts := states[undoTask]
			if next, err = writeTask(tree, ts); err != nil {
				return err
			}
			op, label, task = "edit", ts.t.ID, ts.t.ULID
		case left > UndoWindow+3 && r.IntN(100) < 2:
			// two branches from tip, then their merge: the merge changes nothing of its own
			i, j := pick(), pick()
			for j == i {
				j = pick()
			}
			edit(i, at)
			sideTree, err := writeTask(tree, states[i])
			if err != nil {
				return err
			}
			side, err := m.commit(sideTree, parent, a, at, message("edit", states[i].t.ID, states[i].t.ULID, a.acct.ID()))
			if err != nil {
				return err
			}
			slot++
			at = when()
			edit(j, at)
			mainTree, err := writeTask(tree, states[j])
			if err != nil {
				return err
			}
			mainC, err := m.commit(mainTree, parent, a, at, message("edit", states[j].t.ID, states[j].t.ULID, a.acct.ID()))
			if err != nil {
				return err
			}
			slot++
			at = when()
			merged, err := writeTask(mainTree, states[i]) // both changes
			if err != nil {
				return err
			}
			if tip, err = m.commit(merged, []plumbing.Hash{mainC, side}, a, at, message("merge", "", model.ULID{}, "")); err != nil {
				return err
			}
			tree = merged
			slot++
			continue
		case r.IntN(100) < 5:
			i := pick()
			cu, err := newULID(r, at)
			if err != nil {
				return err
			}
			c := &model.Comment{ULID: cu, Author: a.acct.ID(), Created: at, Updated: at, Body: fmt.Sprintf("Comment %d\n", r.IntN(1000000))}
			data, err := c.Encode()
			if err != nil {
				return err
			}
			b, err := m.blob(data)
			if err != nil {
				return err
			}
			if next, err = m.set(tree, strings.Split(model.CommentPath(states[i].t.ULID, cu), "/"), b); err != nil {
				return err
			}
			op, label, task = "comment", states[i].t.ID, states[i].t.ULID
		default:
			i := pick()
			edit(i, at)
			if next, err = writeTask(tree, states[i]); err != nil {
				return err
			}
			op, label, task = "edit", states[i].t.ID, states[i].t.ULID
		}
		if tip, err = m.commit(next, parent, a, at, message(op, label, task, a.acct.ID())); err != nil {
			return err
		}
		tree = next
		slot++
	}

	if err := writePack(repo, m); err != nil {
		return err
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), tip)
	if err := repo.Storer.SetReference(ref); err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	// the work tree and the git index follow HEAD
	if err := wt.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: tip}); err != nil {
		return err
	}
	st, err := wt.Status()
	if err != nil {
		return err
	}
	if !st.IsClean() {
		return errors.New("the work tree of the synthesized store is not clean")
	}
	return nil
}

// writePack writes every object made into one pack of the repository.
func writePack(repo *git.Repository, m *maker) error {
	pw, ok := repo.Storer.(storer.PackfileWriter)
	if !ok {
		return errors.New("the storage cannot write a pack")
	}
	w, err := pw.PackfileWriter()
	if err != nil {
		return err
	}
	// With deltas, as the packs of a real store are (a fetch, or
	// Consolidate with the default pack window).
	if _, err := packfile.NewEncoder(w, m.mem, false).Encode(m.hashes, gitconfig.DefaultPackWindow); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}
