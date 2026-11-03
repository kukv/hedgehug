// Package history keeps, for each commit of a store, the files it changed,
// who wrote it and its Hedgehug trailers, so that the history of a task or a
// file is looked up instead of walked. The index is a cache in
// .hedgehug/local: it is rebuilt when it is missing, of another format, or
// for a history HEAD no longer has.
package history

import (
	"bytes"
	"errors"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

const (
	fileName         = "history" // in .hedgehug/local
	progressInterval = 1000
)

// Commit is what the index keeps of one commit.
type Commit struct {
	Hash      plumbing.Hash
	When      time.Time // author time
	Gen       uint32    // 1 for a root commit, else 1 + the largest of its parents'
	Author    string    // the Hedgehug-Author trailer; "" when none
	Email     string    // the author's email
	Op        string    // Hedgehug-Op
	Task      string    // the first Hedgehug-Task
	Workspace string    // Hedgehug-Workspace
}

// Writer is LogEntry.Writer: Author, else Email.
func (c Commit) Writer() string {
	if c.Author != "" {
		return c.Author
	}
	return c.Email
}

// newer reports whether a is newer than b: later author time, then larger
// generation, then larger hash.
func newer(a, b Commit) bool {
	if !a.When.Equal(b.When) {
		return a.When.After(b.When)
	}
	if a.Gen != b.Gen {
		return a.Gen > b.Gen
	}
	return bytes.Compare(a.Hash[:], b.Hash[:]) > 0
}

// Options of Open. Progress, when set, is told how many of the commits to
// add are done: 0 at the start, then now and then while they are added.
type Options struct {
	Progress func(done, total int)
	// Cancel, when closed, stops Open between two commits with ErrCanceled;
	// nothing is saved then.
	Cancel <-chan struct{}
}

// ErrCanceled: Open was stopped by Options.Cancel.
var ErrCanceled = errors.New("history index build canceled")

// ErrNotIndexed: a commit asked about is not in the index.
var ErrNotIndexed = errors.New("commit not in the history index")

// rec is a Commit as the index keeps it: the strings are numbers into
// History.strs, so that 100,000 of them take little memory and little time
// to load.
type rec struct {
	hash      plumbing.Hash
	sec       int64  // author time, Unix seconds
	zone      int32  // its offset from UTC, seconds
	gen       uint32 // as Commit.Gen
	parent    uint32 // 1 + the index of the first parent; 0 for a root commit
	author    uint32
	email     uint32
	op        uint32
	task      uint32
	workspace uint32
}

// zones are the *time.Location of the offsets met, shared by all indexes.
var zones sync.Map // int (seconds east of UTC) -> *time.Location

func zoneOf(off int) *time.Location {
	if z, ok := zones.Load(off); ok {
		return z.(*time.Location)
	}
	z, _ := zones.LoadOrStore(off, time.FixedZone("", off))
	return z.(*time.Location)
}

// History is the index of the commits of one store.
type History struct {
	repo   *gitrepo.Repo
	tip    plumbing.Hash
	recs   []rec    // in the order they were added; indices are stable
	strs   []string // the strings of recs, in the order they were first met
	strIdx map[string]uint32
	// byHash is made when first needed (hashes), not when the index is
	// loaded: most users of a loaded index never ask for a commit by hash.
	byHash     map[plumbing.Hash]int
	hashesOnce sync.Once
	byPath     map[string][]int // indices into recs, ascending

	// What the file holds (file.go): whether the index came from a file that
	// can be added to, and how much of the index is in it.
	fromFile   bool
	fileID     uint32 // the checksum of the first part of the file; chunks name it
	tailBad    bool   // the file ends in something that is not a chunk
	chunks     int
	chunkBytes int
	mainBytes  int
	savedRecs  int
	savedStrs  int
	savedTip   plumbing.Hash
	pending    map[string][]int // paths changed by the commits put since, for a chunk
}

func newHistory(repo *gitrepo.Repo) *History {
	return &History{repo: repo, byPath: map[string][]int{}}
}

// hashes returns the index of each commit by its hash.
func (h *History) hashes() map[plumbing.Hash]int {
	h.hashesOnce.Do(func() {
		h.byHash = make(map[plumbing.Hash]int, len(h.recs))
		for i, r := range h.recs {
			h.byHash[r.hash] = i
		}
	})
	return h.byHash
}

// intern returns the number of the string v in strs.
func (h *History) intern(v string) uint32 {
	if h.strIdx == nil {
		h.strIdx = make(map[string]uint32, len(h.strs))
		for i, s := range h.strs {
			h.strIdx[s] = uint32(i)
		}
	}
	if i, ok := h.strIdx[v]; ok {
		return i
	}
	i := uint32(len(h.strs)) //nolint:gosec // few distinct strings
	h.strs = append(h.strs, v)
	h.strIdx[v] = i
	return i
}

// commit is the Commit of the i-th record.
func (h *History) commit(i int) Commit {
	r := &h.recs[i]
	return Commit{
		Hash: r.hash, When: time.Unix(r.sec, 0).In(zoneOf(int(r.zone))), Gen: r.gen,
		Author: h.strs[r.author], Email: h.strs[r.email], Op: h.strs[r.op], Task: h.strs[r.task], Workspace: h.strs[r.workspace],
	}
}

// Open loads the index of s, brings it to HEAD and saves it.
func Open(s *store.Store, opt Options) (*History, error) {
	h := newHistory(s.Repo())
	if !h.load(s) {
		h = newHistory(s.Repo())
	}
	head, err := h.repo.Head()
	if err != nil {
		return nil, err
	}
	if head == h.tip {
		return h, nil
	}
	added, reached, err := h.newCommits(head)
	if err != nil {
		return nil, err
	}
	if !reached {
		// The tip is not in HEAD's history: start again.
		h = newHistory(s.Repo())
		if added, _, err = h.newCommits(head); err != nil {
			return nil, err
		}
	}
	if err := h.add(added, opt); err != nil {
		return nil, err
	}
	h.tip = head
	h.save(s) // a failure is ignored: the next Open does the work again
	return h, nil
}

// newCommits returns the commits reachable from head that the index does
// not have, parents before children. reached says whether the walk met the
// indexed tip (or the index is empty), that is, whether the index is a part
// of HEAD's history.
func (h *History) newCommits(head plumbing.Hash) (added []gitrepo.LogEntry, reached bool, err error) {
	reached = h.tip.IsZero()
	if head.IsZero() {
		return nil, reached, nil
	}
	type frame struct {
		e    gitrepo.LogEntry
		next int
	}
	known := h.hashes()
	seen := map[plumbing.Hash]bool{}
	visit := func(hash plumbing.Hash) (*frame, error) {
		if hash == h.tip {
			reached = true
		}
		if _, ok := known[hash]; ok || seen[hash] {
			return nil, nil
		}
		seen[hash] = true
		e, err := h.repo.Entry(hash)
		if err != nil {
			return nil, err
		}
		return &frame{e: e}, nil
	}
	root, err := visit(head)
	if err != nil || root == nil {
		return nil, reached, err
	}
	stack := []*frame{root}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		if f.next == len(f.e.Parents) {
			added = append(added, f.e)
			stack = stack[:len(stack)-1]
			continue
		}
		p := f.e.Parents[f.next]
		f.next++
		pf, err := visit(p)
		if err != nil {
			return nil, false, err
		}
		if pf != nil {
			stack = append(stack, pf)
		}
	}
	return added, reached, nil
}

// add indexes the commits, parents first.
func (h *History) add(added []gitrepo.LogEntry, opt Options) error {
	progress := opt.Progress
	total := len(added)
	if progress != nil && total > 0 {
		progress(0, total) // so that whoever times the work starts at its start
	}
	known := h.hashes()
	for i, e := range added {
		select {
		case <-opt.Cancel:
			return ErrCanceled
		default:
		}
		_, files, err := h.repo.ChangedFiles(e.Hash)
		if err != nil {
			return err
		}
		c := Commit{Hash: e.Hash, When: e.When, Gen: 1, Email: e.Author.Email}
		var parent uint32
		if len(e.Parents) > 0 {
			if pi, ok := known[e.Parents[0]]; ok {
				parent = uint32(pi) + 1 //nolint:gosec // an index
			}
		}
		for _, p := range e.Parents {
			if pi, ok := known[p]; ok {
				c.Gen = max(c.Gen, h.recs[pi].gen+1)
			}
		}
		c.Author, _ = e.Trailer("Hedgehug-Author")
		c.Op, _ = e.Trailer("Hedgehug-Op")
		c.Task, _ = e.Trailer("Hedgehug-Task")
		c.Workspace, _ = e.Trailer("Hedgehug-Workspace")
		h.put(c, parent, files)
		if progress != nil && ((i+1)%progressInterval == 0 || i+1 == total) {
			progress(i+1, total)
		}
	}
	return nil
}

func (h *History) put(c Commit, parent uint32, files []string) {
	i := len(h.recs)
	_, off := c.When.Zone()
	h.recs = append(h.recs, rec{
		hash: c.Hash, parent: parent, sec: c.When.Unix(), zone: int32(off), gen: c.Gen, //nolint:gosec // an offset is within a day
		author: h.intern(c.Author), email: h.intern(c.Email), op: h.intern(c.Op), task: h.intern(c.Task), workspace: h.intern(c.Workspace),
	})
	h.hashes()[c.Hash] = i
	for _, p := range files {
		h.byPath[p] = append(h.byPath[p], i)
		if h.fromFile {
			if h.pending == nil {
				h.pending = map[string][]int{}
			}
			h.pending[p] = append(h.pending[p], i)
		}
	}
}

// Tip is the HEAD the index was brought to; the zero hash for a store
// without commits.
func (h *History) Tip() plumbing.Hash { return h.tip }

// Has reports whether the commit c is in the index.
func (h *History) Has(c plumbing.Hash) bool {
	_, ok := h.hashes()[c]
	return ok
}

func (h *History) sorted(idx []int) []Commit {
	out := make([]Commit, len(idx))
	for i, j := range idx {
		out[i] = h.commit(j)
	}
	slices.SortFunc(out, func(a, b Commit) int {
		switch {
		case newer(a, b):
			return -1
		case newer(b, a):
			return 1
		}
		return 0
	})
	return out
}

// All returns every commit, newest first.
func (h *History) All() []Commit {
	idx := make([]int, len(h.recs))
	for i := range idx {
		idx[i] = i
	}
	return h.sorted(idx)
}

// Path returns the commits that changed the file p, newest first.
func (h *History) Path(p string) []Commit { return h.sorted(h.byPath[p]) }

// Task returns the commits that changed the file of the task u, its
// comments or its worklog entries, newest first.
func (h *History) Task(u model.ULID) []Commit {
	taskFile := model.TaskPath(u)
	prefixes := []string{
		path.Dir(model.CommentPath(u, model.ULID{})) + "/",
		path.Dir(model.WorklogPath(u, model.ULID{})) + "/",
	}
	set := map[int]struct{}{}
	for p, idx := range h.byPath {
		if p != taskFile && !slices.ContainsFunc(prefixes, func(pre string) bool { return strings.HasPrefix(p, pre) }) {
			continue
		}
		for _, i := range idx {
			set[i] = struct{}{}
		}
	}
	all := make([]int, 0, len(set))
	for i := range set {
		all = append(all, i)
	}
	return h.sorted(all)
}

// Authors returns the distinct non-empty Hedgehug-Author values, sorted.
func (h *History) Authors() []string {
	set := map[uint32]struct{}{}
	for _, r := range h.recs {
		if h.strs[r.author] != "" {
			set[r.author] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, h.strs[a])
	}
	slices.Sort(out)
	return out
}

// LastChange returns the newest commit in the history of from that changed
// the file p; false when none did. It is ErrNotIndexed when from is not in
// the index.
func (h *History) LastChange(p string, from plumbing.Hash) (Commit, bool, error) {
	if !h.Has(from) {
		return Commit{}, false, ErrNotIndexed
	}
	cs := h.Path(p)
	if from == h.tip {
		if len(cs) == 0 {
			return Commit{}, false, nil
		}
		return cs[0], true, nil
	}
	var fromGen uint32
	if i, ok := h.hashes()[from]; ok {
		fromGen = h.recs[i].gen
	}
	for _, c := range cs {
		if c.Hash == from {
			return c, true, nil
		}
		// An ancestor has a smaller generation than from.
		if fromGen > 0 && c.Gen >= fromGen {
			continue
		}
		ok, err := h.repo.IsAncestor(c.Hash, from)
		if err != nil {
			return Commit{}, false, err
		}
		if ok {
			return c, true, nil
		}
	}
	return Commit{}, false, nil
}

// Entries reads up to limit (0: all) of cs as log entries.
func (h *History) Entries(cs []Commit, limit int) ([]gitrepo.LogEntry, error) {
	if limit > 0 && len(cs) > limit {
		cs = cs[:limit]
	}
	out := make([]gitrepo.LogEntry, 0, len(cs))
	for _, c := range cs {
		e, err := h.repo.Entry(c.Hash)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// AtOrBefore returns the newest commit on the first-parent line from the
// commit from that was made at or before t; false when there is none. It
// is ErrNotIndexed when from is not in the index.
func (h *History) AtOrBefore(from plumbing.Hash, t time.Time) (plumbing.Hash, bool, error) {
	i, ok := h.hashes()[from]
	if !ok {
		return plumbing.ZeroHash, false, ErrNotIndexed
	}
	for {
		r := &h.recs[i]
		if !time.Unix(r.sec, 0).After(t) {
			return r.hash, true, nil
		}
		if r.parent == 0 {
			return plumbing.ZeroHash, false, nil
		}
		i = int(r.parent) - 1
	}
}
