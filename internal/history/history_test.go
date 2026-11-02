package history_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/history"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "test"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "test", IDPrefix: "TST"}, testAuthor, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func workspace(t *testing.T, s *store.Store) model.ULID {
	t.Helper()
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		return w.ULID
	}
	t.Fatal("the store has no workspace")
	return model.ULID{}
}

// writeTaskAs writes the task u with the title in one commit made by the
// account, and returns the commit.
func writeTaskAs(t *testing.T, s *store.Store, account string, u model.ULID, title string) plumbing.Hash {
	t.Helper()
	ws := workspace(t, s)
	h, err := s.UpdateCommit(store.Op{Name: "update", Task: u, Author: testAuthor, Account: account}, func(tx *store.Tx) error {
		return tx.PutTask(&model.Task{ULID: u, Kind: "task", Title: title, Status: "todo", Workspace: ws})
	})
	if err != nil || h.IsZero() {
		t.Fatalf("write task: %v, %v", h, err)
	}
	return h
}

func writeTask(t *testing.T, s *store.Store, u model.ULID, title string) plumbing.Hash {
	t.Helper()
	return writeTaskAs(t, s, "", u, title)
}

func writeComment(t *testing.T, s *store.Store, u model.ULID, body string) plumbing.Hash {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	h, err := s.UpdateCommit(store.Op{Name: "comment", Task: u, Author: testAuthor}, func(tx *store.Tx) error {
		return tx.PutComment(u, &model.Comment{ULID: model.NewULID(), Author: "a", Created: now, Updated: now, Body: body})
	})
	if err != nil || h.IsZero() {
		t.Fatalf("write comment: %v, %v", h, err)
	}
	return h
}

func head(t *testing.T, s *store.Store) plumbing.Hash {
	t.Helper()
	h, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func setMain(t *testing.T, s *store.Store, h plumbing.Hash) {
	t.Helper()
	r, err := git.PlainOpen(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), h)); err != nil {
		t.Fatal(err)
	}
}

func hashes(cs []history.Commit) []plumbing.Hash {
	out := make([]plumbing.Hash, len(cs))
	for i, c := range cs {
		out[i] = c.Hash
	}
	return out
}

// plumber makes commits of a fixed author time and exactly the given files,
// next to the store's own history.
type plumber struct {
	t *testing.T
	r *git.Repository
}

func newPlumber(t *testing.T, s *store.Store) *plumber {
	t.Helper()
	r, err := git.PlainOpen(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	return &plumber{t: t, r: r}
}

func (p *plumber) put(o plumbing.EncodedObject) plumbing.Hash {
	p.t.Helper()
	h, err := p.r.Storer.SetEncodedObject(o)
	if err != nil {
		p.t.Fatal(err)
	}
	return h
}

// tree stores the tree of files (path to content).
func (p *plumber) tree(prefix string, files map[string]string) plumbing.Hash {
	p.t.Helper()
	tree := &object.Tree{}
	dirs := map[string]bool{}
	for path, content := range files {
		rest, ok := strings.CutPrefix(path, prefix)
		if !ok {
			continue
		}
		if dir, _, nested := strings.Cut(rest, "/"); nested {
			dirs[dir] = true
			continue
		}
		o := p.r.Storer.NewEncodedObject()
		o.SetType(plumbing.BlobObject)
		w, err := o.Writer()
		if err != nil {
			p.t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			p.t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			p.t.Fatal(err)
		}
		tree.Entries = append(tree.Entries, object.TreeEntry{Name: rest, Mode: filemode.Regular, Hash: p.put(o)})
	}
	for d := range dirs {
		tree.Entries = append(tree.Entries, object.TreeEntry{Name: d, Mode: filemode.Dir, Hash: p.tree(prefix+d+"/", files)})
	}
	key := func(e object.TreeEntry) string {
		if e.Mode == filemode.Dir {
			return e.Name + "/"
		}
		return e.Name
	}
	slices.SortFunc(tree.Entries, func(a, b object.TreeEntry) int { return strings.Compare(key(a), key(b)) })
	o := p.r.Storer.NewEncodedObject()
	if err := tree.Encode(o); err != nil {
		p.t.Fatal(err)
	}
	return p.put(o)
}

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func (p *plumber) commit(at time.Time, parents []plumbing.Hash, files map[string]string) plumbing.Hash {
	p.t.Helper()
	sig := object.Signature{Name: "t", Email: "t@example.com", When: at}
	c := &object.Commit{Author: sig, Committer: sig, Message: "x\n", TreeHash: p.tree("", files), ParentHashes: parents}
	o := p.r.Storer.NewEncodedObject()
	if err := c.Encode(o); err != nil {
		p.t.Fatal(err)
	}
	return p.put(o)
}

func TestOpenOrdersAndLooksUp(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u := model.NewULID()
	c1 := writeTask(t, s, u, "one")
	c2 := writeComment(t, s, u, "hi")
	c3 := writeTask(t, s, model.NewULID(), "other")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := hashes(h.Task(u)); !slices.Equal(got, []plumbing.Hash{c2, c1}) {
		t.Fatalf("Task: %v", got)
	}
	if got := hashes(h.Path(model.TaskPath(u))); !slices.Equal(got, []plumbing.Hash{c1}) {
		t.Fatalf("Path: %v", got)
	}
	if h.All()[0].Hash != c3 || h.Tip() != c3 {
		t.Fatal("All/Tip")
	}
	c := h.All()[0]
	if c.Op != "update" || c.Email != testAuthor.Email || c.Gen != 4 || c.Writer() != testAuthor.Email {
		t.Fatalf("commit fields: %+v", c)
	}
}

func TestOpenAddsOnlyNewCommits(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeTask(t, s, model.NewULID(), "a")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	writeTask(t, s, model.NewULID(), "b")
	var totals, dones []int
	h, err := history.Open(s, history.Options{Progress: func(done, total int) { dones = append(dones, done); totals = append(totals, total) }})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(totals, []int{1}) || !slices.Equal(dones, []int{1}) {
		t.Fatalf("progress done %v of %v, want 1 of 1", dones, totals)
	}
	if len(h.All()) != 3 {
		t.Fatalf("%d commits, want 3", len(h.All()))
	}
}

func TestProgressEveryThousand(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	p := newPlumber(t, s)
	var h plumbing.Hash
	for i := range 1001 {
		var parents []plumbing.Hash
		if !h.IsZero() {
			parents = []plumbing.Hash{h}
		}
		h = p.commit(t0.Add(time.Duration(i)*time.Second), parents, map[string]string{"f": strconv.Itoa(i)})
	}
	setMain(t, s, h)
	var dones []int
	if _, err := history.Open(s, history.Options{Progress: func(done, total int) {
		if total != 1001 {
			t.Errorf("total %d", total)
		}
		dones = append(dones, done)
	}}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dones, []int{1000, 1001}) {
		t.Fatalf("progress %v", dones)
	}
}

func TestOpenRebuildsWhenTipIsGone(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	base := head(t, s)
	writeTask(t, s, model.NewULID(), "a")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	setMain(t, s, base) // HEAD back, as a force push could: the indexed tip is no longer in its history
	c := writeTask(t, s, model.NewULID(), "b")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(h.All()); n != 2 { // the first commit and c
		t.Fatalf("%d commits, want 2", n)
	}
	if h.Tip() != c {
		t.Fatal("tip")
	}
}

func TestOpenRebuildsBrokenFile(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeTask(t, s, model.NewULID(), "a")
	if err := s.WriteLocal("history", []byte("garbage")); err != nil {
		t.Fatal(err)
	}
	h, err := history.Open(s, history.Options{})
	if err != nil || len(h.All()) != 2 {
		t.Fatalf("%v, %d", err, len(h.All()))
	}
}

func TestFileRoundTripAndDamage(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u := model.NewULID()
	writeTaskAs(t, s, "alice", u, "a")
	writeComment(t, s, u, "hi")
	first, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	same := func(label string, got *history.History) {
		t.Helper()
		a, b := first.All(), got.All()
		if len(a) != len(b) || got.Tip() != first.Tip() {
			t.Fatalf("%s: %d commits, want %d", label, len(b), len(a))
		}
		for i := range a {
			_, offA := a[i].When.Zone()
			_, offB := b[i].When.Zone()
			if a[i].Hash != b[i].Hash || !a[i].When.Equal(b[i].When) || offA != offB || a[i].Gen != b[i].Gen ||
				a[i].Author != b[i].Author || a[i].Email != b[i].Email || a[i].Op != b[i].Op || a[i].Task != b[i].Task || a[i].Workspace != b[i].Workspace {
				t.Fatalf("%s: commit %d: %+v, want %+v", label, i, b[i], a[i])
			}
		}
		if !slices.Equal(hashes(got.Task(u)), hashes(first.Task(u))) {
			t.Fatalf("%s: Task differs", label)
		}
	}
	loaded, err := history.Open(s, history.Options{Progress: func(int, int) { t.Error("a saved index was built again") }})
	if err != nil {
		t.Fatal(err)
	}
	same("loaded", loaded)
	// Every shorter file, and a changed format, is rebuilt, not trusted.
	for n := range len(data) {
		if err := s.WriteLocal("history", data[:n]); err != nil {
			t.Fatal(err)
		}
		h, err := history.Open(s, history.Options{})
		if err != nil {
			t.Fatalf("truncated to %d: %v", n, err)
		}
		same("truncated", h)
	}
	bad := bytes.Clone(data)
	bad[4]++ // the format version
	if err := s.WriteLocal("history", bad); err != nil {
		t.Fatal(err)
	}
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	same("other format", h)
}

func TestSameSecondOrder(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	p := newPlumber(t, s)
	f := func(n string) map[string]string { return map[string]string{"f": n} }
	root := p.commit(t0, nil, f("0"))
	// a and b are one line, a fully in the same second as b: b is newer by
	// generation. c and d are on two sides with a: the larger hash is newer.
	a := p.commit(t0.Add(time.Second), []plumbing.Hash{root}, f("a"))
	b := p.commit(t0.Add(time.Second), []plumbing.Hash{a}, f("b"))
	c := p.commit(t0.Add(time.Second), []plumbing.Hash{root}, f("c"))
	d := p.commit(t0.Add(time.Second), []plumbing.Hash{root}, f("d"))
	m := p.commit(t0.Add(time.Second), []plumbing.Hash{b, c, d}, f("m"))
	setMain(t, s, m)
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// m has generation 4, b 3, a/c/d 2 (a hash tie-break among them), root 1
	// and an earlier time.
	gen2 := []plumbing.Hash{a, c, d}
	slices.SortFunc(gen2, func(x, y plumbing.Hash) int { return bytes.Compare(y[:], x[:]) })
	want := append([]plumbing.Hash{m, b}, gen2...)
	want = append(want, root)
	if got := hashes(h.All()); !slices.Equal(got, want) {
		t.Fatalf("All: %v\nwant %v", got, want)
	}
	// c and d, two sides of the same time and generation.
	if got := hashes(h.Path("f")); !slices.Equal(got, want) {
		t.Fatalf("Path: %v\nwant %v", got, want)
	}
	// A line of commits of one time is in the order of its generations.
	if got := h.All(); got[0].Gen != 4 || got[1].Gen != 3 || got[len(got)-1].Gen != 1 {
		t.Fatalf("generations: %+v", got)
	}
}

func TestMergeCountsAsChangeOnlyAgainstAllParents(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	p := newPlumber(t, s)
	u := model.NewULID()
	tp := model.TaskPath(u)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	file := func(content, other string) map[string]string {
		return map[string]string{tp: content, "other": other}
	}
	r := p.commit(at(0), nil, file("0", "0"))
	s1 := p.commit(at(1), []plumbing.Hash{r}, file("1", "0"))
	s2 := p.commit(at(2), []plumbing.Hash{r}, file("2", "0"))
	// A clean merge of two sides that each changed u takes s2's content: it
	// equals s2, so it changed nothing of its own.
	m1 := p.commit(at(3), []plumbing.Hash{s1, s2}, file("2", "0"))
	s4 := p.commit(at(4), []plumbing.Hash{m1}, file("4", "0"))
	s5 := p.commit(at(5), []plumbing.Hash{m1}, file("5", "0"))
	// A merge whose u differs from both parents changed u.
	m2 := p.commit(at(6), []plumbing.Hash{s4, s5}, file("6", "0"))
	setMain(t, s, m2)
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []plumbing.Hash{m2, s5, s4, s2, s1, r}
	if got := hashes(h.Task(u)); !slices.Equal(got, want) {
		t.Fatalf("Task: %v\nwant %v", got, want)
	}
	if got := hashes(h.Path("other")); !slices.Equal(got, []plumbing.Hash{r}) {
		t.Fatalf("Path other: %v", got)
	}
}

func TestLastChangeFrom(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u, other := model.NewULID(), model.NewULID()
	c1 := writeTask(t, s, u, "one")
	c2 := writeTask(t, s, other, "other")
	c3 := writeTask(t, s, u, "two")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := model.TaskPath(u)
	for _, c := range []struct {
		name string
		path string
		from plumbing.Hash
		want plumbing.Hash
		ok   bool
	}{
		{"at the tip", p, c3, c3, true},
		{"before the second change", p, c2, c1, true},
		{"at the first change", p, c1, c1, true},
		{"other not yet written", model.TaskPath(other), c1, plumbing.ZeroHash, false},
		{"other at the tip", model.TaskPath(other), c3, c2, true},
	} {
		got, ok, err := h.LastChange(c.path, c.from)
		if err != nil || ok != c.ok || (ok && got.Hash != c.want) {
			t.Errorf("%s: %v, %v, %v; want %v, %v", c.name, got.Hash, ok, err, c.want, c.ok)
		}
	}
	missing := plumbing.NewHash("1111111111111111111111111111111111111111")
	if _, _, err := h.LastChange(p, missing); !errors.Is(err, history.ErrNotIndexed) {
		t.Fatalf("a commit not indexed: %v", err)
	}
	if h.Has(missing) || !h.Has(c2) {
		t.Fatal("Has")
	}
}

func TestAuthors(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeTaskAs(t, s, "bob", model.NewULID(), "b")
	writeTaskAs(t, s, "alice", model.NewULID(), "a")
	writeTaskAs(t, s, "bob", model.NewULID(), "b2")
	writeTask(t, s, model.NewULID(), "nobody")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := h.Authors(); !slices.Equal(got, []string{"alice", "bob"}) {
		t.Fatalf("Authors: %v", got)
	}
	for _, c := range h.All() {
		if c.Author == "" && c.Writer() != c.Email {
			t.Fatalf("Writer of %+v", c)
		}
		if c.Author != "" && c.Writer() != c.Author {
			t.Fatalf("Writer of %+v", c)
		}
	}
}

func TestEntries(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	u := model.NewULID()
	c1 := writeTask(t, s, u, "one")
	c2 := writeComment(t, s, u, "hi")
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	cs := h.Task(u)
	es, err := h.Entries(cs, 0)
	if err != nil || len(es) != 2 || es[0].Hash != c2 || es[1].Hash != c1 {
		t.Fatalf("Entries: %+v, %v", es, err)
	}
	es, err = h.Entries(cs, 1)
	if err != nil || len(es) != 1 || es[0].Hash != c2 {
		t.Fatalf("Entries limit 1: %+v, %v", es, err)
	}
}
