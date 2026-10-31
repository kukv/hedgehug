package gitrepo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// commitHead is commitFiles returning the new HEAD commit instead of its tree.
func commitHead(t *testing.T, r *Repo, dir string, files map[string]string) plumbing.Hash {
	t.Helper()
	commitFiles(t, r, dir, files)
	h, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestMergeBaseAndAncestor(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	base, _ := a.r.Head()
	ha := commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	hb := commitHead(t, b.r, b.dir, map[string]string{"y.md": "b\n"})
	if err := a.r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	theirs, err := b.r.Fetch("origin")
	if err != nil || theirs != ha {
		t.Fatalf("fetched %s, %v", theirs, err)
	}
	mb, err := b.r.MergeBase(hb, theirs)
	if err != nil || mb != base {
		t.Errorf("MergeBase = %s, %v; want %s", mb, err, base)
	}
	if ok, _ := b.r.IsAncestor(base, hb); !ok {
		t.Error("base is not an ancestor of hb")
	}
	if ok, _ := b.r.IsAncestor(hb, theirs); ok {
		t.Error("hb is an ancestor of theirs")
	}
	if ok, err := b.r.IsAncestor(hb, hb); !ok || err != nil {
		t.Errorf("IsAncestor(hb, hb) = %v, %v", ok, err)
	}
}

func TestMergeBaseUnrelatedAndZero(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"a.md": "1\n"})
	other := t.TempDir()
	o, _ := Init(other)
	commitHead(t, o, other, map[string]string{"b.md": "2\n"})
	if err := o.SetRemotes([]string{filepath.Join(dir, ".git")}); err != nil {
		t.Fatal(err)
	}
	theirs, err := o.Fetch("origin")
	if err != nil {
		t.Fatal(err)
	}
	ours, _ := o.Head()
	if mb, err := o.MergeBase(ours, theirs); err != nil || !mb.IsZero() {
		t.Errorf("MergeBase of unrelated = %s, %v", mb, err)
	}
	if mb, err := r.MergeBase(h, plumbing.ZeroHash); err != nil || !mb.IsZero() {
		t.Errorf("MergeBase with zero = %s, %v", mb, err)
	}
	if ok, err := r.IsAncestor(plumbing.ZeroHash, h); !ok || err != nil {
		t.Errorf("IsAncestor(zero, h) = %v, %v", ok, err)
	}
	if ok, err := r.IsAncestor(h, plumbing.ZeroHash); ok || err != nil {
		t.Errorf("IsAncestor(h, zero) = %v, %v", ok, err)
	}
}

func TestTreeOfAndReadAt(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"a.md": "1\n"})
	tree, err := r.TreeOf(h)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := r.HeadTree(); tree != want {
		t.Errorf("TreeOf = %s, want %s", tree, want)
	}
	if tree, err := r.TreeOf(plumbing.ZeroHash); err != nil || !tree.IsZero() {
		t.Errorf("TreeOf(zero) = %s, %v", tree, err)
	}
	if b, err := r.ReadAt(h, "a.md"); err != nil || string(b) != "1\n" {
		t.Errorf("ReadAt = %q, %v", b, err)
	}
	if _, err := r.ReadAt(h, "nope.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadAt missing = %v", err)
	}
	if _, err := r.ReadAt(plumbing.ZeroHash, "a.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadAt(zero) = %v", err)
	}
}

func TestCommitWithParentsAndLastChange(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	base, _ := a.r.Head()
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "a1\n"})
	ha := commitHead(t, a.r, a.dir, map[string]string{"y.md": "a2\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	hb := commitHead(t, b.r, b.dir, map[string]string{"z.md": "b\n"})
	writeFile(t, b.dir, "x.md", "a1\n")
	writeFile(t, b.dir, "y.md", "a2\n")
	m, err := b.r.Commit(Commit{Paths: []string{"x.md", "y.md"}, Subject: "merge", Parents: []plumbing.Hash{hb, theirs}, Author: Signature{"b", "b@x"}, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := b.r.r.CommitObject(m)
	if c.NumParents() != 2 {
		t.Errorf("parents = %d", c.NumParents())
	}
	e, ok, err := b.r.LastChange(ha, base, "x.md")
	if err != nil || !ok || e.Subject == "" {
		t.Errorf("LastChange = %+v, %v, %v", e, ok, err)
	}
	if _, ok, _ := b.r.LastChange(hb, base, "x.md"); ok {
		t.Error("x.md was not changed on b's side")
	}
	// The merge takes x.md from theirs, so it is not a change of its own.
	e, ok, err = b.r.LastChange(m, base, "x.md")
	if err != nil || !ok || e.Hash == m {
		t.Errorf("LastChange through the merge = %+v, %v, %v", e, ok, err)
	}
	if _, ok, _ := b.r.LastChange(m, m, "x.md"); ok {
		t.Error("LastChange counts base itself")
	}
}

func TestMergeCommitWithTheSameTreeAsOurs(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	hb := commitHead(t, b.r, b.dir, map[string]string{"x.md": "b\n"})
	// Ours wins: the tree is HEAD's, but the two parents make it a commit.
	m, err := b.r.Commit(Commit{Paths: []string{"x.md"}, Subject: "merge", Parents: []plumbing.Hash{hb, theirs}, Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := b.r.Head(); got != m {
		t.Errorf("HEAD = %s, want %s", got, m)
	}
	// One parent and the same tree is still nothing to commit.
	if _, err := b.r.Commit(Commit{Paths: []string{"x.md"}, Subject: "s", Parents: []plumbing.Hash{m}, Author: author, When: time.Now()}); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("Commit with one parent and no change = %v", err)
	}
}

func TestLastChangeLatestOnEitherBranch(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	base, _ := a.r.Head()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	commitAt := func(c clone, content string, when time.Time) plumbing.Hash {
		writeFile(t, c.dir, "x.md", content)
		h, err := c.r.Commit(Commit{Paths: []string{"x.md"}, Subject: content, Author: author, When: when})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	// a changes x.md later than b, but b's change is the first parent.
	commitAt(a, "a", t0.Add(2*time.Hour))
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	commitAt(b, "b", t0.Add(time.Hour))
	hb, _ := b.r.Head()
	// The merge keeps b's content: it is the same as the first parent, so
	// the latest change is a's, found only through the second parent.
	writeFile(t, b.dir, "x.md", "b")
	writeFile(t, b.dir, "y.md", "y")
	m, err := b.r.Commit(Commit{Paths: []string{"x.md", "y.md"}, Subject: "merge", Parents: []plumbing.Hash{hb, theirs}, Author: author, When: t0.Add(3 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if e, ok, _ := b.r.LastChange(m, base, "x.md"); !ok || e.Subject != "a" {
		t.Errorf("LastChange(m) = %+v, %v", e, ok)
	}
	// A merge whose content differs from both parents is a change.
	writeFile(t, b.dir, "x.md", "merged")
	m2, err := b.r.Commit(Commit{Paths: []string{"x.md"}, Subject: "merge2", Parents: []plumbing.Hash{m, theirs}, Author: author, When: t0.Add(4 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if e, ok, _ := b.r.LastChange(m2, base, "x.md"); !ok || e.Hash != m2 {
		t.Errorf("LastChange(m2) = %+v, %v", e, ok)
	}
}

func TestLogFromFollowsFirstParent(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	seed, _ := a.r.Head()
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	hb := commitHead(t, b.r, b.dir, map[string]string{"y.md": "b\n"})
	writeFile(t, b.dir, "x.md", "a\n")
	m, err := b.r.Commit(Commit{Paths: []string{"x.md"}, Subject: "merge", Parents: []plumbing.Hash{hb, theirs}, Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	es, err := b.r.LogFrom(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []plumbing.Hash
	for _, e := range es {
		got = append(got, e.Hash)
	}
	if want := []plumbing.Hash{m, hb, seed}; !slices.Equal(got, want) {
		t.Errorf("LogFrom = %v, want %v", got, want)
	}
	if es, _ := b.r.LogFrom(m, 2); len(es) != 2 {
		t.Errorf("LogFrom limit 2 = %d entries", len(es))
	}
	if es, err := b.r.LogFrom(plumbing.ZeroHash, 0); err != nil || len(es) != 0 {
		t.Errorf("LogFrom(zero) = %v, %v", es, err)
	}
}

func TestFastForward(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	h := commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	if err := b.r.FastForward(theirs, []string{"x.md"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.r.Head(); got != h {
		t.Errorf("HEAD = %s", got)
	}
	if st, _ := b.r.Status(); len(st) != 0 {
		t.Errorf("status after fast-forward = %+v", st)
	}
}

func TestFastForwardRemovesAndKeepsOtherFiles(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "x\n", "d/y.md": "y\n"})
	_ = a.r.Push("origin")
	_, _ = b.r.Fetch("origin")
	if err := b.r.FastForward(mustRemoteHead(t, b.r), []string{"x.md", "d/y.md"}); err != nil {
		t.Fatal(err)
	}
	h := commitHead(t, a.r, a.dir, map[string]string{"x.md": ""})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	// A file the fast-forward is not told about stays as it is.
	writeFile(t, b.dir, "local.md", "mine\n")
	if err := b.r.FastForward(theirs, []string{"x.md"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.r.Head(); got != h {
		t.Errorf("HEAD = %s, want %s", got, h)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "x.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("x.md is left: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(b.dir, "local.md")); string(got) != "mine\n" {
		t.Errorf("local.md = %q", got)
	}
	st, _ := b.r.Status()
	if len(st) != 1 || st[0].Path != "local.md" {
		t.Errorf("status = %+v", st)
	}
	if err := b.r.FastForward(theirs, []string{"x.md"}); err != nil {
		t.Errorf("FastForward of a path already gone = %v", err)
	}
}

func mustRemoteHead(t *testing.T, r *Repo) plumbing.Hash {
	t.Helper()
	h, err := r.RemoteHead("origin")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestFastForwardMissingCommitChangesNothing(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"x.md": "x\n"})
	missing := plumbing.NewHash("0123456789abcdef0123456789abcdef01234567")
	if err := r.FastForward(missing, []string{"x.md"}); err == nil {
		t.Fatal("FastForward to a missing commit succeeded")
	}
	if got, _ := r.Head(); got != h {
		t.Errorf("HEAD = %s, want %s", got, h)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "x.md")); string(got) != "x\n" {
		t.Errorf("x.md = %q", got)
	}
	if st, _ := r.Status(); len(st) != 0 {
		t.Errorf("status = %+v", st)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "index.lock")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("index.lock is left after a failed fast-forward")
	}
}

func TestFastForwardRefusesPathsOutsideTheWorkTree(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	h := commitHead(t, r, dir, map[string]string{"x.md": "x\n"})
	for _, p := range []string{"../x.md", "/x.md", ".git/config", ".git"} {
		if err := r.FastForward(h, []string{p}); err == nil {
			t.Errorf("FastForward(%q) succeeded", p)
		}
	}
}

func TestBetween(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	base := commitHead(t, r, dir, map[string]string{"a.md": "1\n"})
	commitHead(t, r, dir, map[string]string{"a.md": "2\n"})
	h := commitHead(t, r, dir, map[string]string{"a.md": "3\n"})
	es, err := r.Between(h, base)
	if err != nil || len(es) != 2 || es[1].Hash != h {
		t.Errorf("Between = %v, %v", es, err)
	}
	if es, err := r.Between(h, plumbing.ZeroHash); err != nil || len(es) != 3 {
		t.Errorf("Between(h, zero) = %v, %v", es, err)
	}
	if es, err := r.Between(base, h); err != nil || len(es) != 0 {
		t.Errorf("Between(base, h) = %v, %v", es, err)
	}
}

func TestBetweenAcrossAMerge(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	base, _ := a.r.Head()
	ha := commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	hb := commitHead(t, b.r, b.dir, map[string]string{"y.md": "b\n"})
	writeFile(t, b.dir, "x.md", "a\n")
	m, err := b.r.Commit(Commit{Paths: []string{"x.md"}, Subject: "merge", Parents: []plumbing.Hash{hb, theirs}, Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	es, err := b.r.Between(m, base)
	if err != nil {
		t.Fatal(err)
	}
	var got []plumbing.Hash
	for _, e := range es {
		got = append(got, e.Hash)
	}
	// ha and hb are both before the merge; the merge comes last.
	if len(got) != 3 || got[2] != m || !slices.Contains(got, ha) || !slices.Contains(got, hb) {
		t.Errorf("Between = %v", got)
	}
	// Seen from a's side, only b's commits and the merge are new.
	es, err = b.r.Between(m, ha)
	if err != nil || len(es) != 2 || es[0].Hash != hb || es[1].Hash != m {
		t.Errorf("Between(m, ha) = %v, %v", es, err)
	}
}

// commitWith commits path with content on top of parents.
func commitWith(t *testing.T, r *Repo, dir, path, content string, when time.Time, parents ...plumbing.Hash) plumbing.Hash {
	t.Helper()
	writeFile(t, dir, path, content)
	h, err := r.Commit(Commit{Paths: []string{path}, Subject: content, Parents: parents, Author: author, When: when})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestBetweenPutsParentsFirstWhateverTheTime(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := commitWith(t, r, dir, "a.md", "base", t0)
	// The child's clock is behind its parent's.
	c1 := commitWith(t, r, dir, "a.md", "c1", t0.Add(2*time.Hour), base)
	c2 := commitWith(t, r, dir, "a.md", "c2", t0.Add(time.Hour), c1)
	// A side branch from base, older than c1.
	s1 := commitWith(t, r, dir, "b.md", "s1", t0.Add(30*time.Minute), base)
	m := commitWith(t, r, dir, "a.md", "m", t0.Add(30*time.Minute), c2, s1)
	es, err := r.Between(m, base)
	if err != nil {
		t.Fatal(err)
	}
	var got []plumbing.Hash
	for _, e := range es {
		got = append(got, e.Hash)
	}
	if want := []plumbing.Hash{s1, c1, c2, m}; !slices.Equal(got, want) {
		t.Errorf("Between = %v, want %v", got, want)
	}
	// The parents' order does not change it.
	m2 := commitWith(t, r, dir, "a.md", "m2", t0.Add(30*time.Minute), s1, c2)
	es, err = r.Between(m2, base)
	if err != nil {
		t.Fatal(err)
	}
	got = got[:0]
	for _, e := range es {
		got = append(got, e.Hash)
	}
	if want := []plumbing.Hash{s1, c1, c2, m2}; !slices.Equal(got, want) {
		t.Errorf("Between with swapped parents = %v, want %v", got, want)
	}
	// LastChange is the latest by time, even before a later commit in order.
	if e, ok, _ := r.LastChange(c2, base, "a.md"); !ok || e.Hash != c1 {
		t.Errorf("LastChange = %+v, %v; want c1", e, ok)
	}
}

func TestSideIsWalkedOnceAndSharedSafely(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	base := commitHead(t, r, dir, map[string]string{"a.md": "1\n"})
	commitHead(t, r, dir, map[string]string{"a.md": "2\n"})
	h := commitHead(t, r, dir, map[string]string{"b.md": "3\n"})
	first, err := r.side(h, base)
	if err != nil || len(first) != 2 {
		t.Fatalf("side = %v, %v", first, err)
	}
	// The second call returns the slice the first one kept.
	if again, _ := r.side(h, base); &again[0] != &first[0] {
		t.Error("side walked the history again")
	}
	// Under -race, many readers of one fresh (from, base) do not race.
	r2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				if es, err := r2.Between(h, base); err != nil || len(es) != 2 || es[1].Hash != h {
					t.Errorf("Between = %v, %v", es, err)
				}
				return
			}
			if e, ok, err := r2.LastChange(h, base, "a.md"); err != nil || !ok || e.Hash == h {
				t.Errorf("LastChange = %+v, %v, %v", e, ok, err)
			}
		}()
	}
	wg.Wait()
}

func TestMergeBaseCrissCrossIsSymmetric(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Either of the two bases is the newer one in one of the cases, so that
	// go-git's own order of the bases cannot pass for the smallest hash.
	for _, swap := range []bool{false, true} {
		dir := t.TempDir()
		r, _ := Init(dir)
		base := commitWith(t, r, dir, "a.md", "base", t0)
		tx, ty := t0.Add(time.Minute), t0.Add(2*time.Minute)
		if swap {
			tx, ty = ty, tx
		}
		x := commitWith(t, r, dir, "x.md", "x", tx, base)
		y := commitWith(t, r, dir, "y.md", "y", ty, base)
		mx := commitWith(t, r, dir, "a.md", "mx", t0.Add(3*time.Minute), x, y)
		my := commitWith(t, r, dir, "a.md", "my", t0.Add(4*time.Minute), y, x)
		ab, err := r.MergeBase(mx, my)
		if err != nil {
			t.Fatal(err)
		}
		ba, err := r.MergeBase(my, mx)
		if err != nil {
			t.Fatal(err)
		}
		want := min(x.String(), y.String())
		if ab != ba || ab.String() != want {
			t.Errorf("swap %v: MergeBase(mx, my) = %s, MergeBase(my, mx) = %s; want %s", swap, ab, ba, want)
		}
	}
}

// ffSetup gives b a fetched commit of a's that changes x.md.
func ffSetup(t *testing.T) (b clone, theirs plumbing.Hash) {
	t.Helper()
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	if err := a.r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	theirs, err := b.r.Fetch("origin")
	if err != nil {
		t.Fatal(err)
	}
	return b, theirs
}

// assertRefused checks that a fast-forward was refused and left HEAD at
// before and x.md as want ("" for missing).
func assertRefused(t *testing.T, b clone, err error, before plumbing.Hash, want string) {
	t.Helper()
	if !errors.Is(err, ErrWorkTreeChanged) {
		t.Errorf("FastForward = %v, want ErrWorkTreeChanged", err)
	}
	if got, _ := b.r.r.Storer.Reference(plumbing.NewBranchReferenceName("main")); got.Hash() != before {
		t.Errorf("main = %s, want %s", got.Hash(), before)
	}
	got, rerr := os.ReadFile(filepath.Join(b.dir, "x.md"))
	if want == "" && !errors.Is(rerr, fs.ErrNotExist) || want != "" && string(got) != want {
		t.Errorf("x.md = %q, %v; want %q", got, rerr, want)
	}
	if _, err := os.Stat(filepath.Join(b.dir, ".git", "index.lock")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("index.lock is left")
	}
}

func TestFastForwardRefusesAChangedPath(t *testing.T) {
	b, theirs := ffSetup(t)
	before, _ := b.r.Head()
	writeFile(t, b.dir, "x.md", "mine\n")
	assertRefused(t, b, b.r.FastForward(theirs, []string{"x.md"}), before, "mine\n")
}

func TestFastForwardRefusesWhenHeadMovedAway(t *testing.T) {
	b, theirs := ffSetup(t)
	hb := commitHead(t, b.r, b.dir, map[string]string{"y.md": "b\n"})
	assertRefused(t, b, b.r.FastForward(theirs, []string{"x.md"}), hb, "")
}

func TestFastForwardFinishesAStoppedOne(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	h := commitHead(t, a.r, a.dir, map[string]string{"x.md": "x\n", "y.md": "y\n", "seed.md": ""})
	if err := a.r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	theirs, err := b.r.Fetch("origin")
	if err != nil {
		t.Fatal(err)
	}
	// A run that stopped after writing x.md and removing seed.md, but
	// before y.md and the reference.
	writeFile(t, b.dir, "x.md", "x\n")
	if err := removeFile(b.dir, "seed.md"); err != nil {
		t.Fatal(err)
	}
	if err := b.r.FastForward(theirs, []string{"x.md", "y.md", "seed.md"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.r.Head(); got != h {
		t.Errorf("HEAD = %s, want %s", got, h)
	}
	if st, _ := b.r.Status(); len(st) != 0 {
		t.Errorf("status = %+v", st)
	}
}

func TestFastForwardRefusesADetachedHead(t *testing.T) {
	b, theirs := ffSetup(t)
	before, _ := b.r.Head()
	if err := b.r.r.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, before)); err != nil {
		t.Fatal(err)
	}
	assertRefused(t, b, b.r.FastForward(theirs, []string{"x.md"}), before, "")
}
