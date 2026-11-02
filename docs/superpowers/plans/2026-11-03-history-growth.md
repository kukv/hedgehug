# 長期運用での履歴の増加 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 1 Store あたり 10 万コミットで、履歴を使う操作を `show` 200ms・`log <id>` 1 秒に収め、pack が増え続けないようにする。

**Architecture:** 新しいパッケージ `internal/history` が、コミットごとの「変わったファイル・書いた人・trailer」を `.hedgehug/local/history` に持つ（消せば作り直すキャッシュ）。HEAD が動いたら増えたコミットだけを足す。`ops`・`index`・`sync`・`server` は、履歴をたどる代わりにこの索引を引く。first-parent の読み方は途中で止められる形にし、同期の最後の `Repack` を、小さい pack を段階的にまとめる `Consolidate` に置き換える。

**Tech Stack:** Go、go-git v5（v5.19.3）、`encoding/binary`、gotestsum。

**Spec:** `docs/superpowers/specs/2026-11-03-history-growth-design.md`（親設計 rev 359 に反映済み）

## Global Constraints

- 対象の規模: 1 Store あたり 10 万コミット。
- 速さの目標（10 万コミット）: `show`（History 5 件）・`undo`・GUI のタスク詳細 200ms、`log <id>` の全件・`restore --at` 1 秒、索引の読み込みと 1 回の引き 50ms、普段の pack の統合 1 秒。`undo` の目標は自分のコミットを取り消すときで、取り消すものがないときは記録だけ。
- 「変えた」の意味: 親が 1 つならその親と比べて違えば変えた。マージは**すべての親**と比べて違うときだけ変えた。親がなければ、ファイルがあれば変えた（`gitrepo.LastChange` と同じ）。
- 並び: 時刻（author）の新しい順。同じ時刻なら世代の大きい方、それも同じなら hash の大きい方を新しいとする。世代 = 親がなければ 1、あれば親の世代の最大 + 1。
- 索引はキャッシュ。消しても、形式の版が違っても、読めなくても作り直す。保存はロックなしの丸ごと書き換え（`Store.WriteLocal`）。
- tip（入れた HEAD）を含まない履歴に HEAD がなっていたら、索引を捨てて作り直す。
- 最初の作成: clone する操作（`StoreAdd`、`doctor --fix` の clone し直し）の後とサーバーの起動時。ないときに履歴を使う CLI の操作が来たらその場で作り、1 秒を超えたら stderr に `Indexing the history of <store>: <n>/<total> commits` を出す。
- pack: 64 MiB 以上の pack はまとめ直さない。オブジェクトは捨てない。きっかけは「ゆるいオブジェクト > 6,700」か「64 MiB 未満の pack > 16」。同期の最後（push の後）で、`sync.lock` の中、Store の書き込みのロックは取らない。remote 側は対象外。
- 古い履歴の切り捨てはしない。`doctor` の Verify、同期のマージ（base を持つ `LastChange`）、マージドライバー、inbox は変えない。
- 速さの計測は `make bench-history`（手元）。CI には含めない。
- コードのコメント・識別子・CLI の文言は英語、ドキュメントは日本語（リポジトリの決まり）。英語の数の表現は `internal/plural` を使う。
- テストは今の書き方に合わせる（`t.Parallel()`、`testenv`、golden は `-update` で更新）。落ちたテストは失敗の出力と実行条件を記録する。

## ファイルの構成

| ファイル | 役割 |
| --- | --- |
| `internal/gitrepo/changed.go`（新規） | `Repo.ChangedFiles`（1 コミットが変えたファイル）、`Repo.Entry`（1 コミットを LogEntry で読む） |
| `internal/gitrepo/log.go` | `Repo.WalkFirstParents`（途中で止められる first-parent の読み方）。`LogFrom` はその上に作る |
| `internal/gitrepo/maintain.go` | `Repo.Consolidate`、`Repo.SmallPacks`。`Repack` を置き換える |
| `internal/history/history.go`（新規） | `History` 型、`Open`、増分の更新と作り直し、引き |
| `internal/history/file.go`（新規） | 索引のファイルの形式（二進、版つき） |
| `internal/history/history_test.go`・`property_test.go`（新規） | 意味と増分のテスト、突き合わせのプロパティテスト |
| `internal/history/bench_test.go`・`internal/history/historytest/synth.go`（新規） | 10 万コミットの合成と計測 |
| `internal/ops/read.go`・`people.go`・`movestore.go`・`undo.go`・`delete.go`・`storeops.go`・`rebuild.go`・`session.go` | 索引を使う、途中で止める、最初の作成、進みの通知 |
| `internal/index/forward.go` | 転送ファイルの元の ID を索引で引く |
| `internal/sync/sync.go` | `headAuthor` を索引で引く、`Consolidate` を呼ぶ |
| `internal/server/poll.go` | HEAD の移動と起動時に索引を更新する |
| `cmd/hedgehug/main.go` | 進みの表示（stderr） |
| `Makefile` | `bench-history` |

---

### Task 1: gitrepo に「1 コミットが変えたファイル」と途中で止める読み方を足す

**Files:**
- Create: `internal/gitrepo/changed.go`
- Modify: `internal/gitrepo/log.go`（`LogFrom` を `WalkFirstParents` の上に作り直す）
- Test: `internal/gitrepo/changed_test.go`、`internal/gitrepo/log_test.go`

**Interfaces:**
- Produces:
  - `func (r *Repo) ChangedFiles(h plumbing.Hash) (LogEntry, []string, error)` — h の LogEntry と、h が変えたファイルのパス（昇順、重複なし）。意味は Global Constraints の「変えた」。
  - `func (r *Repo) Entry(h plumbing.Hash) (LogEntry, error)` — h を LogEntry にする。
  - `func (r *Repo) WalkFirstParents(from plumbing.Hash, fn func(LogEntry) bool) error` — from から first-parent をたどり、fn が false を返したら止める。from がゼロなら何もしない。

- [ ] **Step 1: 失敗するテストを書く**

`internal/gitrepo/changed_test.go`（既存のテストの補助 `newRepo`・`commitFiles` 相当がなければ、`commit_merge_test.go` の作り方に合わせて同じファイルに小さな補助を書く）:

```go
func TestChangedFiles(t *testing.T) {
	t.Parallel()
	r := initRepo(t)
	root := commitFiles(t, r, nil, map[string]string{"tasks/a.md": "1", "schema.yaml": "s"})
	c1 := commitFiles(t, r, []plumbing.Hash{root}, map[string]string{"tasks/a.md": "2", "comments/x/y.md": "c"})
	side := commitFiles(t, r, []plumbing.Hash{root}, map[string]string{"tasks/b.md": "b"})
	// A merge that takes each side's content changes nothing of its own.
	clean := mergeFiles(t, r, c1, side, map[string]string{"tasks/a.md": "2", "comments/x/y.md": "c", "tasks/b.md": "b", "schema.yaml": "s"})
	// A merge whose a.md differs from both parents changed a.md.
	both := commitFiles(t, r, []plumbing.Hash{root}, map[string]string{"tasks/a.md": "3"})
	mixed := mergeFiles(t, r, c1, both, map[string]string{"tasks/a.md": "4", "comments/x/y.md": "c", "schema.yaml": "s"})
	for _, c := range []struct {
		name string
		h    plumbing.Hash
		want []string
	}{
		{"root", root, []string{"schema.yaml", "tasks/a.md"}},
		{"one parent", c1, []string{"comments/x/y.md", "tasks/a.md"}},
		{"clean merge", clean, nil},
		{"merge that made new content", mixed, []string{"tasks/a.md"}},
	} {
		e, got, err := r.ChangedFiles(c.h)
		if err != nil {
			t.Fatal(err)
		}
		if e.Hash != c.h || !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWalkFirstParentsStops(t *testing.T) {
	t.Parallel()
	r := initRepo(t)
	var h plumbing.Hash
	for i := range 5 {
		var parents []plumbing.Hash
		if !h.IsZero() {
			parents = []plumbing.Hash{h}
		}
		h = commitFiles(t, r, parents, map[string]string{"f": fmt.Sprint(i)})
	}
	n := 0
	if err := r.WalkFirstParents(h, func(LogEntry) bool { n++; return n < 2 }); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("visited %d commits, want 2", n)
	}
	all, err := r.LogFrom(h, 0)
	if err != nil || len(all) != 5 {
		t.Fatalf("LogFrom: %d entries, %v", len(all), err)
	}
}
```

`commitFiles`・`mergeFiles` は go-git の plumbing でオブジェクトを直接書く補助（作業ツリーを使わない）。`initRepo` は `Init(t.TempDir())`。`mergeFiles(t, r, a, b, files)` は親 a・b のコミットを作る。files にないパスは消えたものとして扱う（ツリーは files だけから作る）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/gitrepo -run 'TestChangedFiles|TestWalkFirstParentsStops' -count=1`
Expected: FAIL（`r.ChangedFiles undefined`、`r.WalkFirstParents undefined`）

- [ ] **Step 3: 実装する**

`internal/gitrepo/changed.go`:

```go
package gitrepo

import (
	"slices"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Entry reads the commit h as a LogEntry.
func (r *Repo) Entry(h plumbing.Hash) (LogEntry, error) {
	c, err := r.commit(h)
	if err != nil {
		return LogEntry{}, err
	}
	return entry(c), nil
}

// ChangedFiles returns the commit h and the files it changed, sorted: for a
// commit with one parent, those that differ from the parent; for a merge,
// those that differ from every parent, so that a merge that takes one
// side's content changed nothing of its own (as LastChange counts); for a
// root commit, every file. Only the trees whose hashes differ are read.
func (r *Repo) ChangedFiles(h plumbing.Hash) (LogEntry, []string, error) {
	c, err := r.commit(h)
	if err != nil {
		return LogEntry{}, nil, err
	}
	t, err := r.tree(c.TreeHash)
	if err != nil {
		return LogEntry{}, nil, err
	}
	if c.NumParents() == 0 {
		var out []string
		if err := r.diffTrees(nil, t, "", func(p string) { out = append(out, p) }); err != nil {
			return LogEntry{}, nil, err
		}
		slices.Sort(out)
		return entry(c), out, nil
	}
	count := map[string]int{}
	for _, ph := range c.ParentHashes {
		p, err := r.commit(ph)
		if err != nil {
			return LogEntry{}, nil, err
		}
		pt, err := r.tree(p.TreeHash)
		if err != nil {
			return LogEntry{}, nil, err
		}
		if err := r.diffTrees(pt, t, "", func(path string) { count[path]++ }); err != nil {
			return LogEntry{}, nil, err
		}
	}
	var out []string
	for p, n := range count {
		if n == c.NumParents() {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return entry(c), out, nil
}

// diffTrees calls fn with every file path that differs between a and b
// (either may be nil, an empty tree), going down only into directories
// whose hashes differ.
func (r *Repo) diffTrees(a, b *object.Tree, dir string, fn func(string)) error {
	byName := func(t *object.Tree) map[string]object.TreeEntry {
		m := map[string]object.TreeEntry{}
		if t != nil {
			for _, e := range t.Entries {
				m[e.Name] = e
			}
		}
		return m
	}
	am, bm := byName(a), byName(b)
	names := map[string]bool{}
	for n := range am {
		names[n] = true
	}
	for n := range bm {
		names[n] = true
	}
	for n := range names {
		ea, inA := am[n]
		eb, inB := bm[n]
		if inA && inB && ea.Hash == eb.Hash && ea.Mode == eb.Mode {
			continue
		}
		var sa, sb *object.Tree
		var err error
		if inA && ea.Mode == filemode.Dir {
			if sa, err = r.tree(ea.Hash); err != nil {
				return err
			}
		} else if inA {
			fn(dir + n)
		}
		if inB && eb.Mode == filemode.Dir {
			if sb, err = r.tree(eb.Hash); err != nil {
				return err
			}
		} else if inB && !(inA && ea.Mode != filemode.Dir) {
			fn(dir + n)
		}
		if sa != nil || sb != nil {
			if err := r.diffTrees(sa, sb, dir+n+"/", fn); err != nil {
				return err
			}
		}
	}
	return nil
}
```

`fn` は同じパスで 1 度だけ呼ぶ（ファイル → ファイルの変更で 2 度呼ばない）ことに注意する。上のコードは「a 側がファイルなら a で呼び、b 側は a がファイルでないときだけ呼ぶ」ことでこれを守る。

`internal/gitrepo/log.go` の `LogFrom` を置き換える:

```go
// WalkFirstParents calls fn with the commits from from along the first
// parents, newest first, until fn returns false. The zero hash has none.
func (r *Repo) WalkFirstParents(from plumbing.Hash, fn func(LogEntry) bool) error {
	for h := from; !h.IsZero(); {
		c, err := r.r.CommitObject(h)
		if err != nil {
			return fmt.Errorf("commit %s: %w", h, err)
		}
		if !fn(entry(c)) {
			return nil
		}
		h = plumbing.ZeroHash
		if len(c.ParentHashes) > 0 {
			h = c.ParentHashes[0]
		}
	}
	return nil
}

// LogFrom returns the commits from from along the first parents, newest
// first: the history of the line from was made on, without the commits a
// merge brought in. A limit above zero caps the number of entries. The zero
// hash has an empty log.
func (r *Repo) LogFrom(from plumbing.Hash, limit int) ([]LogEntry, error) {
	var out []LogEntry
	err := r.WalkFirstParents(from, func(e LogEntry) bool {
		out = append(out, e)
		return limit <= 0 || len(out) < limit
	})
	return out, err
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/gitrepo -count=1`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/gitrepo/changed.go internal/gitrepo/changed_test.go internal/gitrepo/log.go internal/gitrepo/log_test.go
git commit -m "feat(gitrepo): 1 コミットが変えたファイルと、途中で止められる first-parent の読み方を足す"
```

---

### Task 2: 履歴の索引（internal/history）

**Files:**
- Create: `internal/history/history.go`、`internal/history/file.go`
- Test: `internal/history/history_test.go`、`internal/history/testmain_test.go`（他パッケージの `testmain_test.go` と同じ形）

**Interfaces:**
- Consumes: Task 1 の `Repo.ChangedFiles`、`Repo.Entry`。既存の `Repo.Head`、`Repo.IsAncestor`、`Store.ReadLocal`/`WriteLocal`、`model.TaskPath`/`CommentPath`/`WorklogPath`。
- Produces:

```go
package history

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
func (c Commit) Writer() string

// Options of Open. Progress, when set, is told how many of the commits to
// add are done, now and then while they are added.
type Options struct {
	Progress func(done, total int)
}

// ErrNotIndexed: a commit asked about is not in the index.
var ErrNotIndexed = errors.New("commit not in the history index")

func Open(s *store.Store, opt Options) (*History, error) // load, bring to HEAD, save
func (h *History) Tip() plumbing.Hash
func (h *History) Has(c plumbing.Hash) bool
func (h *History) All() []Commit                       // newest first
func (h *History) Path(p string) []Commit              // commits that changed p, newest first
func (h *History) Task(u model.ULID) []Commit          // task file, comments, worklogs; newest first
func (h *History) Authors() []string                   // distinct non-empty Author values, sorted
func (h *History) LastChange(p string, from plumbing.Hash) (Commit, bool, error) // in the history of from; ErrNotIndexed when !Has(from)
func (h *History) Entries(cs []Commit, limit int) ([]gitrepo.LogEntry, error)   // reads up to limit (0: all) of cs as LogEntry
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/history/history_test.go`。Store は `store` パッケージのテスト補助（`internal/store/helpers_test.go` を見て、外から使える `store.Init` 相当と `Update` でタスクを書く）で作る。ここでは意味と増分だけを確かめ、マージは `gitrepo` の Task 1 と同じ補助（このパッケージにも小さく書く。plumbing で直接コミットを作り、`refs/heads/main` を動かす）で作る。

```go
func TestOpenOrdersAndLooksUp(t *testing.T) {
	t.Parallel()
	s := newStore(t)                           // a store with its first commit
	u := model.NewULID()
	c1 := writeTask(t, s, u, "one")            // a commit that writes tasks/…/<u>.md
	c2 := writeComment(t, s, u, "hi")          // a comment of u
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
}

func TestOpenAddsOnlyNewCommits(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeTask(t, s, model.NewULID(), "a")
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	writeTask(t, s, model.NewULID(), "b")
	var totals []int
	_, err := history.Open(s, history.Options{Progress: func(_, total int) { totals = append(totals, total) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) == 0 || totals[len(totals)-1] != 1 {
		t.Fatalf("progress totals %v, want one commit to add", totals)
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
	resetMain(t, s, base) // HEAD back: the indexed tip is no longer in its history
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

func TestSameSecondOrder(t *testing.T) {
	t.Parallel()
	// Two commits of the same author time on one line: the child is newer
	// (larger generation). Two of the same time on two sides: the larger
	// hash is newer.
	// … build with plumbing commits whose author time is fixed …
}

func TestMergeCountsAsChangeOnlyAgainstAllParents(t *testing.T) {
	t.Parallel()
	// A clean merge of two sides that each changed u: Task(u) has the two
	// side commits and not the merge. A merge whose u differs from both:
	// Task(u) has the merge too.
}

func TestLastChangeFrom(t *testing.T) {
	t.Parallel()
	// p changed at c1 and c3 on one line: LastChange(p, tip) is c3,
	// LastChange(p, c2) is c1, LastChange(p, a commit not indexed) is
	// ErrNotIndexed.
}

func TestAuthors(t *testing.T) {
	t.Parallel()
	// Commits with Hedgehug-Author alice and bob, and one without: Authors
	// is [alice bob].
}
```

`TestSameSecondOrder`・`TestMergeCountsAsChangeOnlyAgainstAllParents`・`TestLastChangeFrom`・`TestAuthors` は、上のコメントのとおりの履歴を補助で作り、`hashes(...)` と期待する並びを `slices.Equal` で比べる形で書き切る。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/history -count=1`
Expected: FAIL（パッケージがない）

- [ ] **Step 3: 実装する**

`internal/history/history.go` の骨格:

```go
// Package history keeps, for each commit of a store, the files it changed,
// who wrote it and its Hedgehug trailers, so that the history of a task or a
// file is looked up instead of walked. The index is a cache in
// .hedgehug/local: it is rebuilt when it is missing, of another format, or
// for a history HEAD no longer has.
package history

const fileName = "history" // in .hedgehug/local

type History struct {
	repo    *gitrepo.Repo
	tip     plumbing.Hash
	commits []Commit             // in the order they were added; indices are stable
	byHash  map[plumbing.Hash]int
	byPath  map[string][]int     // indices into commits, ascending
}

func Open(s *store.Store, opt Options) (*History, error) {
	h := &History{repo: s.Repo()}
	loaded := h.load(s)               // false: start empty
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
	if loaded && !h.tip.IsZero() && !reached {
		*h = History{repo: s.Repo()}  // the tip is not in HEAD's history: rebuild
		if added, _, err = h.newCommits(head); err != nil {
			return nil, err
		}
	}
	if err := h.add(added, opt.Progress); err != nil {
		return nil, err
	}
	h.tip = head
	h.save(s) // a failure is ignored: the next Open does the work again
	return h, nil
}
```

- `newCommits(head)`: スタックで head から親をたどる。`byHash` にあるコミットでは止まる（その先に降りない）。止まったコミットが `h.tip` なら `reached = true`。tip がゼロ（空の索引）なら `reached = true`。返す `added` は親が子より先に来る順（帰りがけで並べる、または取り出した順を逆にしてから世代の計算で親を先に処理する）。`added` を集めるのにコミットのオブジェクトは読むが、ファイルの差分はまだ取らない（total を先に知るため）。
- `add(added, progress)`: 各コミットに `repo.ChangedFiles` を呼び、`Commit` を作る（`Author` は `Trailer("Hedgehug-Author")`、`Task` は `Trailer("Hedgehug-Task")`、`Workspace` は `Trailer("Hedgehug-Workspace")`、`Op` は `Trailer("Hedgehug-Op")`、`Email` は `Author.Email`）。世代は親の世代（`byHash` で引く。親は先に足されている）の最大 + 1。`byPath` に追記する。`progress` があれば、1,000 件ごとと最後に `progress(done, total)` を呼ぶ。
- 並びの比較 `newer(a, b Commit) bool`: `When` → `Gen` → `bytes.Compare(Hash)` の順。`All`・`Path`・`Task` は対象の添字を集め、`newer` で新しい順に並べて返す。
- `Task(u)`: `model.TaskPath(u)` と、`path.Dir(model.CommentPath(u, model.ULID{})) + "/"`・`path.Dir(model.WorklogPath(u, model.ULID{})) + "/"` で始まるパスの添字の和（今の `ops.TaskLog` と同じ範囲）。`byPath` のキーを走査してよい（10 万コミットでもキーは数万）。
- `LastChange(p, from)`: `!Has(from)` なら `ErrNotIndexed`。`from == tip` なら `Path(p)` の先頭。そうでなければ `Path(p)` を新しい順に見て、`c.Hash == from` か `repo.IsAncestor(c.Hash, from)` が真になる最初のもの。
- `Entries(cs, limit)`: 先頭から limit 件（0 は全件）を `repo.Entry` で読む。

`internal/history/file.go`（二進の形式。gob は 10 万件で遅いので使わない）:

```go
// The file: magic "HHHI", format (uvarint), tip (20 bytes), the strings
// table (uvarint count, then each as uvarint length + bytes), the commits
// (uvarint count, then each: hash 20 bytes, When as varint Unix seconds +
// uvarint zone offset index into strings… ), the paths (uvarint count, then
// each: string index, uvarint count, ascending commit indices as uvarint
// deltas).
const fileFormat = 1
```

- 文字列（書いた人、email、Op、Task、Workspace、パス、時刻の zone 名）は表に入れて番号で持つ。
- 時刻は Unix 秒と zone の offset（秒）で持ち、読み込みで `time.Unix(sec, 0).In(time.FixedZone("", off))` に戻す（`LogEntry.When` の表示に zone が要る。`object.Signature.When` と同じ offset を保つ）。
- 読み込みでは magic・版・長さを確かめ、どれかが合わなければ `load` は false（作り直し）。
- `save` は `bytes.Buffer` に書いて `s.WriteLocal(fileName, buf.Bytes())`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/history -count=1 -race`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/history
git commit -m "feat(history): コミットごとの変わったファイルと trailer を持つ履歴の索引を足す"
```

---

### Task 3: 突き合わせのプロパティテスト

**Files:**
- Create: `internal/history/property_test.go`

**Interfaces:**
- Consumes: Task 2 の `history.Open`・`Path`・`Task`・`LastChange`・`All`。`gitrepo.Repo.ReadAt`。

- [ ] **Step 1: テストを書く**

`internal/sync/property_test.go` と同じ形（シードごとの `t.Run`、`t.Parallel()`、`shortMode()` で回数を減らす。`shortMode` はこのパッケージにも同じものを置く）。1 シードで:

1. plumbing で直接コミットを作る補助で、ランダムな履歴を作る: 2〜3 本の枝、各コミットは 4 つのパス（`tasks/…/<u1>.md`、`tasks/…/<u2>.md`、`comments/…/<u1>/<c>.md`、`schema.yaml`）のうちランダムに 1〜2 個を書く、ときどき枝をマージ（マージの中身は、片方を取る／両方を取る／新しい中身にする、をランダムに）、時刻はランダムに「前と同じ秒」か「1〜3 秒後」。`refs/heads/main` はランダムな時点で進める。
2. 途中の何か所かで `history.Open` を呼ぶ（増分）。最後に `.hedgehug/local/history` を消して `history.Open` を呼んだもの（一度に作った索引）も用意する。
3. 素直な実装（索引を使わない）で期待値を作る:

```go
// naive answers what the index should: every commit reachable from head
// that changed p, by comparing p's blob in the commit with its parents'
// (ReadAt), ordered by the rule of the index.
func naive(t *testing.T, r *gitrepo.Repo, head plumbing.Hash, p string) []plumbing.Hash
```

   - 到達できるコミットは `IsAncestor` を使わず、親を自前でたどって集める。
   - 変えたかどうか: 親がなければ p があれば変えた。あれば、どの親とも blob の hash（`ReadAt` の中身の比較で可）が違えば変えた。
   - 世代も自前で計算する。並びは `When` → 世代 → hash の新しい順。
4. 期待値と、増分の索引・一度に作った索引の両方の `Path(p)` の hash の並びが一致することを、4 つのパスすべてで確かめる。`Task(u1)` は `tasks/…/<u1>.md` と `comments/…/<u1>/…` の和と一致すること。`LastChange(p, c)` を、ランダムに選んだ到達できるコミット c で、`naive` を c から求めた先頭と一致することを確かめる。

失敗したら、シードと作った操作の記録を `t.Fatalf` に出す（`property_test.go` の `log` と同じ）。

- [ ] **Step 2: 実行する**

Run: `go test ./internal/history -run TestProperty -count=1`
Expected: PASS。落ちたら Task 2 の実装を直す（テストを緩めない）。落ちたときの出力とシードは台帳に記録する。

- [ ] **Step 3: コミット**

```bash
git add internal/history/property_test.go
git commit -m "test(history): 索引の答えを素直な実装と突き合わせるプロパティテストを足す"
```

---

### Task 4: ops が履歴の索引を使う（log・show・GUI の履歴・人の一覧・移動の再開）と、進みの表示

**Files:**
- Modify: `internal/ops/session.go`（`Progress` を足す）、`internal/ops/read.go`（`TaskLog`、`LogIn`）、`internal/ops/people.go`、`internal/ops/movestore.go`（`moveInRecorded`）、`cmd/hedgehug/main.go`（進みの表示）
- Test: `internal/ops/read_test.go` などの既存のテスト、`cmd/hedgehug/log_test.go`、golden（`cmd/hedgehug/testdata`）

**Interfaces:**
- Consumes: `history.Open`、`History.Task`・`All`・`Authors`・`Entries`、`Commit.Writer`。
- Produces:
  - `Session.Progress func(store string, done, total int)` — nil は何もしない。
  - `func (ss *Session) openHistory(s *store.Store, name string) (*history.History, error)` — `history.Open` に、`Progress` を store 名つきで渡す。

- [ ] **Step 1: 失敗するテストを書く**

1. マージを含む履歴での `log <id>`（`cmd/hedgehug/log_test.go` に足す）: 2 つのクローン（`sync_test.go` の補助）で同じタスクを別のフィールドで変えて同期し、マージができた Store で `log <id>` が「両側の 2 コミット」を新しい順に出し、マージコミットを出さないことを確かめる。今の go-git の絞り込みでは誤る（マージが出るか、片側が抜ける）ことを、まず失敗で確かめる。
2. Store 全体の `log -n 2` が、マージを含む履歴で時刻の新しい順の 2 件になること。
3. `People` が `Hedgehug-Author` のあるコミットの作者を含むこと（既存のテストがあればそのまま通ること）。
4. `Session.Progress` が、索引のない Store で `log <id>` を呼ぶと `total` を 1 回以上受け取ること。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug -run 'TestLog' -count=1`
Expected: 1・2 が FAIL

- [ ] **Step 3: 実装する**

- `TaskLog(s, ulid, limit)`: `ss.openHistory(s, name)` → `h.Entries(h.Task(ulid), limit)`。store 名は呼び出し側の `ix.StoreName()` を渡せるよう、引数に `name string` を足す（`read.go:318`・`556` の 2 か所を直す）。
- `LogIn`（Store 全体）: `m.Store.Repo().Log(nil, readLimit)` を `openHistory` → `All()` に置き換える。`--workspace` の絞り込みは `logWorkspace` と同じ判定を `Commit` の `Workspace`・`Task` で行う関数 `commitWorkspace(ix, c history.Commit)` を足し、絞ってから limit 件を `Entries` で読む。`logWorkspace`（LogEntry 版）を使う箇所がほかになければ消す。`m.With` の中で読む今の形は保つ（ロックと開き直しのため）。
- `People`: `Repo().Log(nil, 0)` の代わりに `openHistory` → `Authors()`。
- `moveInRecorded`: `All()` から `Op == "move-in"` のものだけを `Entries` で読み、今の判定（`Hedgehug-Moved-From`、`Hedgehug-Task` の集合）をそのまま行う。
- CLI（`cmd/hedgehug/main.go` の Session を作るところ）: `Progress` を、最初の呼び出しから 1 秒たってから、1 秒に 1 回まで `Indexing the history of <store>: <done>/<total> commits` を stderr に 1 行で出す関数にする。`done == total` のときは、それまでに 1 行でも出していれば最後の 1 行を出す。数の表現は `internal/plural` を使う（`commits` の単複）。
- サーバーの Session は `Progress` を設定しない（nil）。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/ops ./cmd/hedgehug ./internal/server -count=1`
Expected: PASS。golden の差が出たら、差がマージの扱いの直しと並びの直しだけであることを確かめてから `-update` で更新する。

- [ ] **Step 5: コミット**

```bash
git add internal/ops cmd/hedgehug
git commit -m "feat(ops): log・show・GUI の履歴・人の一覧・移動の再開を履歴の索引で引く"
```

---

### Task 5: 転送ファイルの元の ID と同期の修復の作者を索引で引く

**Files:**
- Modify: `internal/index/forward.go`（`lastTask`・`resolveForwardIDs`・`taskAt` の、base ゼロの `LastChange`/`LastChanges`）、`internal/sync/sync.go`（`headAuthor`）
- Test: `internal/index/forward_test.go`、`internal/sync` の既存のテスト

**Interfaces:**
- Consumes: `history.Open`、`History.LastChange`、`history.ErrNotIndexed`。

- [ ] **Step 1: テストを確かめる**

既存のテスト（`forward_*_test.go`、同期の修復の作者を見るテスト）がそのまま通ることを基準にする。足すテスト: 転送ファイルの元の ID が、移動の前にコメントが書かれていても（タスクのファイルではなくコメントが最後の変更でも）正しい ID になること（ファイル単位の索引でないと誤る）。

- [ ] **Step 2: 実装する**

- `index` は `history` を import する（`history` は `index` を import しないこと。循環しないことを `go build ./...` で確かめる）。
- `lastTask(ulid)`・`resolveForwardIDs`: `history.Open(ix.s, history.Options{})` を 1 回呼び、`h.LastChange(model.TaskPath(ulid), from)` で引く。`from` が `ErrNotIndexed` のとき（索引にないコミット）は、今の `ix.repo.LastChange(from, ZeroHash, p)` に戻る。戻り値の `Commit` から親は `repo.Entry(c.Hash)` の `Parents` で得る。
- 索引の更新は転送ファイルが増えたときだけ起きる（`newForwardEntry`・`resolveForwardIDs` が呼ばれたときだけ `history.Open` する）。毎回の `index.Open` で履歴の索引を読まないこと。
- `sync.headAuthor(r, head)` を `headAuthor(s *store.Store, head plumbing.Hash)` にし、最初の呼び出しで `history.Open(s, history.Options{})` を 1 回だけ行う（クロージャで持つ）。`h.LastChange(p, head)` が `ErrNotIndexed` なら今の `r.LastChange(head, ZeroHash, p)` に戻る。呼び出し元（`mergeAuthor`、`repairHead`、`prefixDuplicates`）を直す。

- [ ] **Step 3: 通ることを確かめる**

Run: `go test ./internal/index ./internal/sync -count=1 -race`
Expected: PASS（同期のプロパティテストを含む）

- [ ] **Step 4: コミット**

```bash
git add internal/index internal/sync
git commit -m "feat(index,sync): 転送ファイルの元の ID と修復の作者を履歴の索引で引く"
```

---

### Task 6: undo と restore --at を途中で止める

**Files:**
- Modify: `internal/ops/undo.go`（`UndoTarget`、`undoInTx`）、`internal/ops/delete.go`（`commitAt`）
- Test: `internal/ops/undo_test.go`（あれば）、`cmd/hedgehug/undo_test.go`、`cmd/hedgehug/restore_at_test.go`

**Interfaces:**
- Consumes: Task 1 の `Repo.WalkFirstParents`。

- [ ] **Step 1: テストを書く**

`UndoTarget` の判定を 1 件ずつ受け取る形に分けたことを確かめるテスト: 自分のコミットが 3 件目にある 10 件の履歴で、`WalkFirstParents` の fn が 3 回だけ呼ばれること（数えるために `undoScanner` を直接テストする）。

```go
func TestUndoScannerStopsAtTarget(t *testing.T) {
	t.Parallel()
	log := []gitrepo.LogEntry{ /* other, other, mine, other … */ }
	var sc undoScanner
	n := 0
	for _, e := range log {
		n++
		if _, ok := sc.next(e, "me"); ok {
			break
		}
	}
	if n != 3 {
		t.Fatalf("read %d entries, want 3", n)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/ops -run TestUndoScannerStopsAtTarget -count=1`
Expected: FAIL（`undoScanner` がない）

- [ ] **Step 3: 実装する**

```go
// undoScanner is UndoTarget one entry at a time, newest first.
type undoScanner struct{ undone map[string]bool }

// next reports whether e is the commit to undo, having seen the newer ones.
func (s *undoScanner) next(e gitrepo.LogEntry, me string) (gitrepo.LogEntry, bool) {
	if s.undone == nil {
		s.undone = map[string]bool{}
	}
	for _, h := range e.TrailerValues("Hedgehug-Undo") {
		s.undone[h] = true
	}
	author, _ := e.Trailer("Hedgehug-Author")
	op, _ := e.Trailer("Hedgehug-Op")
	if author == me && !slices.Contains(undoSkip, op) && !s.undone[e.Hash.String()] && len(e.Parents) > 0 {
		return e, true
	}
	return gitrepo.LogEntry{}, false
}

func UndoTarget(log []gitrepo.LogEntry, me string) (gitrepo.LogEntry, bool) {
	var s undoScanner
	for _, e := range log {
		if c, ok := s.next(e, me); ok {
			return c, true
		}
	}
	return gitrepo.LogEntry{}, false
}
```

`undoInTx` は `r.LogFrom(head, 0)` + `UndoTarget` をやめ、`r.WalkFirstParents(head, func(e) bool { c, ok = s.next(e, me); return !ok })` にする。

`commitAt` は `r.WalkFirstParents(head, func(e) bool { if !e.When.After(t) { found = e.Hash; return false }; return true })` にする。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/ops ./cmd/hedgehug -run 'Undo|RestoreAt|Restore' -count=1`
Expected: PASS（結果が今と同じ）

- [ ] **Step 5: コミット**

```bash
git add internal/ops
git commit -m "perf(ops): undo と restore --at を、対象が見つかったところで止める"
```

---

### Task 7: 索引を先に作る（clone の後、doctor --fix、サーバー）

**Files:**
- Modify: `internal/ops/storeops.go`（`StoreAdd`）、`internal/ops/rebuild.go`（`rebuildStore` の入れ替えの後）、`internal/server/poll.go`（`refresh`）
- Test: `cmd/hedgehug/store_add_test.go`、`cmd/hedgehug/doctor_forcepush_test.go`、`internal/server` の poll のテスト

**Interfaces:**
- Consumes: Task 4 の `ss.openHistory`。

- [ ] **Step 1: テストを書く**

- `store add` の後、`.hedgehug/local/history` があること。
- `doctor --fix` で clone し直した後、`.hedgehug/local/history` があり、その tip が新しい HEAD であること。
- サーバー: Store を mount した poll の後（`hasLooked` が真になった後に少し待つ。既存のテストの待ち方に合わせる）、`.hedgehug/local/history` があること。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./cmd/hedgehug -run 'TestStoreAdd|TestDoctorForcePush' -count=1`
Expected: 足したテストが FAIL

- [ ] **Step 3: 実装する**

- `StoreAdd`: `updateConfig` の後、`warnPrefixDuplicates` の前に `if _, err := ss.openHistory(s, name); err != nil { ss.warn(...) }`。作れなくても Store は足す（索引はキャッシュなので、次に使うときに作り直す）。
- `rebuildStore`: 入れ替えが済み、新しい Store を開けるところで同じことを行う。
- `poll.refresh(st)`: `index` の更新の後、`st.histBusy`（`atomic.Bool`、`polled` に足す）が偽なら真にして `go func() { defer st.histBusy.Store(false); _, _ = history.Open(st.s, history.Options{}) }()`。poll を待たせないためにゴルーチンで行う。`Release`（Store を手放す）では、そのゴルーチンが終わるのを待たなくてよい（索引の保存は失敗しても無視される）が、Windows でディレクトリを入れ替える `BeforeSwap` の前に終わっている必要がある。`Release` で `st.histDone`（`sync.WaitGroup`）を待つ。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./cmd/hedgehug ./internal/server ./internal/ops -count=1 -race`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/ops internal/server cmd/hedgehug
git commit -m "feat: clone の後とサーバーの poll で履歴の索引を作っておく"
```

---

### Task 8: pack を段階的にまとめる

**Files:**
- Modify: `internal/gitrepo/maintain.go`（`Repack` を `Consolidate` に置き換え、`SmallPacks` を足す）、`internal/sync/sync.go`（きっかけ）
- Test: `internal/gitrepo/maintain_test.go`（既存の Repack のテストを置き換える）、`internal/sync` のテスト

**Interfaces:**
- Produces:

```go
// PackCap is the size from which a pack is left as it is.
const PackCap = 64 << 20

// SmallPacks counts the packs smaller than PackCap.
func (r *Repo) SmallPacks() (int, error)

// Consolidate writes the loose objects and the small packs that the
// geometric rule picks into one new pack, then removes what it put in.
func (r *Repo) Consolidate() error
```

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestPickGeometric(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		sizes []int64 // ascending
		want  int     // how many of the smallest to merge
	}{
		{[]int64{10, 10, 10, 100}, 3},   // 10 < 2*10, 10 < 2*20: merge 3; 100 >= 2*30
		{[]int64{10, 25, 100}, 0},       // already geometric
		{[]int64{10, 15, 40, 50}, 4},    // 50 < 2*65
	} {
		if got := pickGeometric(c.sizes); got != c.want {
			t.Errorf("%v: %d, want %d", c.sizes, got, c.want)
		}
	}
}
```

`pickGeometric(sizes)`: 昇順の sizes で、`sizes[i] < 2 * (sizes[0]+…+sizes[i-1])` を満たす最大の i に 1 を足した数（なければ 0）。

ほかに:
- 小さい pack が 3 つ（fetch を 3 回した Store。既存の同期のテストの補助で作る）とゆるいオブジェクトがある Store で `Consolidate` → pack が 1 つ増えて 3 つ減り、ゆるいオブジェクトが 0、`Verify` が通る。
- 64 MiB 以上の pack は触らない: `PackCap` をテストで小さくできるよう、`var packCap int64 = PackCap` にしてテストで書き換える（`afterPackWrite` と同じやり方）。
- 途中で失敗（`afterPackWrite` が error を返す）しても一時ファイルが残らず、元の pack とゆるいオブジェクトがそのまま。
- 統合の間に書かれたゆるいオブジェクトは消えない（選ぶのは始めに見たものだけ）: `afterPackWrite` の中でオブジェクトを 1 つ書き、統合の後もそれがゆるいまま残ることを確かめる。
- 古い pack を消した後、別に開いていた `Repo`（別のハンドル）が `mount.Mounted.With` の開き直しで読めること（`internal/mount` か `internal/ops` の既存の「repack の後に開き直す」テストを `Consolidate` に合わせて直す）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/gitrepo -run 'TestPickGeometric|TestConsolidate' -count=1`
Expected: FAIL

- [ ] **Step 3: 実装する**

- pack の一覧: `.git/objects/pack/pack-*.idx` を glob し、対の `.pack` の大きさを `os.Stat` で取る。`packCap` 以上は除く。大きさの昇順に並べる。
- 選ぶ: `k := pickGeometric(sizes)`。ゆるいオブジェクトの hash を `ForEachObjectHash` で集める。`k < 2 && len(loose) == 0` なら何もしない。
- 選んだ pack のオブジェクトの hash は、`.idx` を `idxfile.NewDecoder` で読み、`EntriesByOffset` で集める。
- `r.writePack(append(packObjs, loose...))`（重複は除く）。今の `writePack` をそのまま使う。
- `Reindex()` の後、選んだ pack の `.pack`・`.idx` を `removeReadOnly` で消す（消せなくても続ける。Windows で開いているもの）。ゆるいオブジェクトは集めたものだけを `DeleteLooseObject` で消す。
- `Repack` と `LooseObjects` の呼び出し元を `Consolidate` に直す。`sync.go`:

```go
	n, err := r.LooseObjects()
	warn("count the loose objects", err)
	packs, perr := r.SmallPacks()
	warn("count the packs", perr)
	if (err == nil && n > repackAt) || (perr == nil && packs > packsAt) {
		warn("consolidate the packs", r.Consolidate())
	}
```

`packsAt = 16` を `repackAt` と並べて置く（テストで書き換えられる `var`）。`Result.Warnings` のコメントの `repacking` を `consolidating the packs` に直す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/gitrepo ./internal/sync ./internal/mount ./internal/ops -count=1 -race`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/gitrepo internal/sync internal/mount internal/ops
git commit -m "feat(gitrepo): 小さい pack を段階的にまとめ、同期の最後に pack の数でもまとめる"
```

---

### Task 9: 10 万コミットでの計測と記録、仕上げ

**Files:**
- Create: `internal/history/historytest/synth.go`、`internal/history/bench_test.go`
- Modify: `Makefile`、`docs/superpowers/specs/2026-11-03-history-growth-design.md`（末尾に「計測の結果」）、本 plan の末尾（plan から変えた点）

**Interfaces:**
- Produces: `func Synth(t testing.TB, commits, tasks int, seed uint64) (dir string)` — 合成した Store のディレクトリ。`$HH_BENCH_CACHE`（既定 `$(go env GOCACHE)/hh-bench`）の下に `c<commits>-t<tasks>-s<seed>` として置き、あれば使い回す。

- [ ] **Step 1: 合成を書く**

`Synth` は `store` の初期化（`store.Init` 相当）で Store を作り、go-git の plumbing（`storer.SetEncodedObject` で blob・tree・commit を直接書く。作業ツリーと index は使わない）で、`commits` 件のコミットを作る。各コミットはランダムなタスク 1 つ（5% はコメント、2% は 2 本の枝のマージ）を変える。trailer（`Hedgehug-Op`・`Hedgehug-Task`・`Hedgehug-Author`）を付ける。最後に `refs/heads/main` を動かし、作業ツリーを HEAD に合わせる（`git` の CLI は使わない）。ディレクトリを一時名で作ってから rename し、途中で止まっても半端なものを使い回さない。

- [ ] **Step 2: 計測を書く**

`bench_test.go`（`HH_BENCH` が空なら `b.Skip()`）:
- `BenchmarkOpenFresh`（索引が最新の `history.Open`）
- `BenchmarkOpenAfterOneCommit`（1 コミット足してから `history.Open`。保存を含む）
- `BenchmarkShow`・`BenchmarkLogTask`（`ops.Session.Show`・`Log` を古いタスクと新しいタスクで。索引が最新のとき）
- `BenchmarkUndo`（自分のコミットが 100 件前にあるとき。対象がないときも測って記録する）
- `BenchmarkRestoreAt`
- `BenchmarkBuild`（索引を消してから作る。記録だけ）
- `BenchmarkConsolidate`（fetch を 20 回した小さい pack の統合と、64 MiB 近くまでの統合）

`Makefile`:

```make
# Measure the history index and pack consolidation on a synthesized store of
# 100,000 commits (not part of check or CI; see the history-growth spec).
bench-history:
	HH_BENCH=1 go test ./internal/history -run '^$$' -bench . -benchtime 5x -timeout 60m
```

- [ ] **Step 3: 測って記録する**

Run: `make bench-history`
Expected: Global Constraints の目標をすべて満たす。満たさないものがあれば、その場で直す（索引の形式、`Open` の読み込み、保存の追記化など）。直せない目標が出たら、作業を止めて利用者に報告する（spec の決定を勝手に変えない）。

結果を spec の末尾に表で足す（目標、計測値、測った条件: CPU・OS・Go の版・コミット数・タスク数）。

- [ ] **Step 4: 全体を確かめる**

Run: `make check`
Expected: PASS（lint 0）

- [ ] **Step 5: コミット**

```bash
git add internal/history Makefile docs/superpowers
git commit -m "test(history): 10 万コミットでの計測を足し、結果を spec に記録する"
```

---

## 実装の順と PR

- Task 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 の順。
- PR は 1 つ（spec の決定を 1 つのまとまりとして入れる）。分けるときは Task 1〜7（索引）と 8（pack）の 2 つにできる。

---

## plan から変えた点

Task 1〜9 の実装で plan と違うことになった点。判断の根拠は `.superpowers/sdd/2026-11-03-history-growth/progress.md` の Ruling にある。

- **Task 2**: 4 つのテストは、plan にコードがなくコメントの指示だけだったので、実装者が書き切った。
- **Task 3**: 索引のファイルは、パスを整列して書き（同じ索引は同じバイト列になる）、時差が ±18 時間を超えるものは壊れたものとして作り直す。
- **Task 4**: `TaskLog` に Store の名前の引数を足した（呼び出し 2 箇所）。ブリーフの「マージコミットを出さない」は、2 つのクローンが同じタスクの別のフィールドを変えた場合には誤りで、フィールド単位のマージの結果は両方の親と違うので、spec 2.3（案 A）どおりマージも出す。進みの表示のため、`history` は作るコミットがあるとき最初に `progress(0, total)` を 1 回呼ぶ（1 秒の計時が作り始めからになる）。`history.Open` が失敗したときは、`headAuthor`・`lastChanger` は失敗を伝えず、従来の全履歴の歩きに戻る。
- **Task 5**: `replay`（force push の後の head は旧 Store の索引にない）では索引を開かずに歩く。転送ファイルの元の ID を引く `lastTask` は、更新ごとに 1 つの `lastChanger` を共有し、索引を 1 回だけ開く。
- **Task 7**: `history.Options.Cancel` と `ErrCanceled` を足した（plan にない）。サーバーの索引づくりは自分の Store のハンドルを開き、Release・パスの変更・対象から外れた Store・終了のときは中断して終わるのを待つ。やり直しは 3 回までにした。
- **Task 8**: `Repack` を `Consolidate` に置き換えた。1 回の統合でまとめる量は、幾何の規則で選んだ pack の合計が `packCap` を超えないよう、大きい方から外す（plan は規則の結果をそのまま使う）。2 つ未満になればゆるいオブジェクトだけを書く。古い pack を消した後にもう一度 `Reindex` する。消せない古い pack は失敗にせず次の統合に任せる。統合のテストは pack の大きさに頼らない形にした。
- **Task 9**:
  - 「`store.Init` 相当」は `store.Create`。`Synth` は `store.Create` で Store を作り、go-git の plumbing で全オブジェクトを pack 1 つに書く（ゆるいオブジェクトを 60 万個作らない）。キャッシュの名前は `c<commits>-t<tasks>-s<seed>-v<layout>` で、合成の形を変えたときに古い Store を使わないよう版を付けた。置き場所は `$HH_BENCH_CACHE`、なければユーザーのキャッシュディレクトリの `hh-bench`（`go env GOCACHE` ではない）。
  - 合成する Store のタスクは階層を付けた（プロジェクト 50 件の下にタスクが約 100 件ずつ）。すべて最上位に置くと、書き込みのたびに兄弟 5,000 件を読む時間が `undo` の計測に入る。
  - 計測の途中で、索引の読み込みを速くするために `history` の中身を変えた（ファイルの形式は同じ）: コミットごとの文字列を番号で持つ、hash の引き（`byHash`）を使うときまで作らない、時差の `*time.Location` を使い回す。10 万コミットでの読み込みは 40〜50ms から 11〜13ms になった。
  - `BenchmarkShowAfterOneCommit` と、索引のファイルの大きさの記録（`BenchmarkBuild`）を足した。spec 6.2 の「HEAD が 1 つ進んだ直後」と「索引のファイルの大きさ」に当たる。
  - 速さの目標のうち 3 つ（HEAD が 1 つ進んだ直後の `show`、`undo`、`restore --at`）は、最初の計測で満たせなかった。利用者の決定（2026-11-03）で、次の 3 つを同じ branch で直した。
    - `restore --at`・`show --at` の時刻の解決を索引で引く: 索引にコミットごとの first parent を持たせ（ファイルの版は 2）、`History.AtOrBefore` で HEAD から first-parent の線をたどる。索引が開けないときは今までの歩きに戻る。`commitAt` は `Session` のメソッドになった。歩きと同じ答えになることは、マージを含むプロパティテストで確かめる。
    - go-git の `KeepDescriptors` を有効にする（`gitrepo.newRepo`）: 1 つのオブジェクトを読むたびに pack を開き直さない。`Repo.Close` が手放す。`Consolidate` は古い pack を消す前に手放し、`store remove` は消す前にハンドルを閉じる。
    - 索引の保存を追記にする: HEAD が進んだとき、新しいコミットと変わったパスを CRC 付きの塊としてファイルの終わりに足す（版は 3、`Store.AppendLocal` を足した）。塊が欠けている・合わない・前の tip につながらないときは、そこまでを読んで残りは git から足し、次の保存でファイルを書き直す。塊が 100 を超えるか、元の半分に近づいたら書き直す。
  - なお `show`（1 つ進んだ直後）と `undo` は、go-git の固定費（pack の索引の読み込み約 150ms、書き込みの tree の組み立て約 200ms）のため目標に届かない。これは別のブレストで扱う（spec の決定 9）。
