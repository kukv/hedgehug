# フェーズ 1c: インデックス・短縮 ID・クエリ Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 1b の Store の上に、タスクのインデックス（キャッシュと差分更新）、短縮 ID と ID の解決、クエリ言語、書き込み時の参照先の存在と循環の検査を実装する。

**Architecture:** `internal/store` は index を知らないまま、書き込み時にファイルを直接読んで参照先の存在（`missing_ref`）と新しい循環（`cycle`）を拒否する。`internal/index` は HEAD のツリーの blob からタスクを読み、`.hedgehug/local/index` に gob でキャッシュし、HEAD が動いたら go-git のツリーの差分だけを読み直す。`internal/query` はクエリを構文解析し、index のエントリをスキーマに従って評価・並べ替え・グループ化する。

**Tech Stack:** Go 1.27.3、`github.com/go-git/go-git/v5` v5.19.3、`go.yaml.in/yaml/v4`、`github.com/oklog/ulid/v2`、標準の `encoding/gob`

**Spec:** `docs/superpowers/specs/2026-10-30-phase1c-index-query-design.md`（実装前に必ず読む）

## Global Constraints

- モジュールは `github.com/kukv/hedgehug`、Go は `go 1.27.3`（`go.mod` のまま）。新しい依存ライブラリは足さない。
- 依存の向きは `query → index → store → schema → model`、`index → gitrepo`、`store → gitrepo`。`store` は `index` を import しない。`gitrepo` は `model` にも `schema` にも依存しない。
- テストは標準の `testing` だけで書く（testify などは使わない）。テストは対象と同じパッケージ（`package index` など）に置く。既存のテストの書き方（`t.Fatal`、`t.Errorf("... = %v, want %v")`）に合わせる。
- コード中のコメントは英語（既存コードに合わせる）。コミットメッセージは日本語の Conventional Commits（例: `feat: ...`）で、末尾に空行を挟んで `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` を付ける。
- 各タスクの最後に `make check`（tidy-check・lint・fmt-check・test）を通す。lint は golangci-lint の standard に revive・gosec・gocritic・unparam などを足したもの（`.golangci.yaml`）。gosec がパス変数の読み書き（G304 など）を指摘したら、`//nolint:gosec // <理由>` を付ける。理由は具体的に書く。gocritic の ifElseChain（3つ以上の if-else）は switch に書き換える。
- CI は ubuntu / macos / windows の matrix でテストを回す。手元では `GOOS=windows go vet ./...` と `GOOS=darwin go vet ./...` でコンパイルを確かめる。
- ファイルのパーミッションは 0600、ディレクトリは 0750（既存の `writeFile` がそうする）。
- 違反コードは `schema` の定数を使う: `missing_ref`、`cycle`（Task 1 で足す）。index だけの問題コードは `broken`。

## ファイル構成

```
internal/model/
  id.go              MarshalBinary・UnmarshalBinary を追加
  id_test.go         gob の往復のテストを追加
internal/schema/
  validate.go        CodeMissingRef・CodeCycle、Rank の bad_value
  validate_test.go
internal/gitrepo/
  tree.go            ErrTreeNotFound、Change、HeadTree、DiffTrees、ReadFile、Files
  tree_test.go
internal/store/
  local.go           ReadLocal、WriteLocal、Repo
  local_test.go
  refs.go            ref、refsOf、checkRefs、exists、edges、reaches
  refs_test.go
  validate.go        lineage の変更、ErrCycle と sameParent の削除、checkRefs の呼び出し
  validate_test.go   親がない・循環のテストを書き換え
internal/index/
  index.go           Index、Open、Refresh、Schema、Task、Tasks、Children、Roots、derive
  entry.go           Entry、Prop、newEntry、scalars、(*Entry).Prop、(*Entry).Targets
  build.go           rebuild、update、revalidate、violations、ancestors、descendants、キャッシュ
  shortid.go         ShortID、Resolve、AmbiguousError、shortLengths
  problems.go        Problem、CodeBroken、Problems、Reverse、refs、cycles
  helpers_test.go    テスト共通の補助
  index_test.go
  build_test.go
  shortid_test.go
  problems_test.go
internal/query/
  parse.go           Query、Term、Error、Parse
  eval.go            Options、Result、Group、Run、各キーの述語
  sort.go            field、並び替え、グループ化
  helpers_test.go
  parse_test.go
  eval_test.go
  sort_test.go
```

---

### Task 1: 下準備（ID の gob 対応、Rank の検査、違反コード）

**Files:**
- Modify: `internal/model/id.go`
- Test: `internal/model/id_test.go`
- Modify: `internal/schema/validate.go`
- Test: `internal/schema/validate_test.go`

**Interfaces:**
- Consumes: なし
- Produces:
  - `func (id ID) MarshalBinary() ([]byte, error)`、`func (id *ID) UnmarshalBinary(b []byte) error`
  - `schema.CodeMissingRef = "missing_ref"`、`schema.CodeCycle = "cycle"`
  - `schema.ValidateTask` が、空でなく `Rank.Validate()` に失敗する rank に `Violation{Code: "bad_value", Field: "rank"}` を返す

- [ ] **Step 1: 失敗するテストを書く**

`internal/model/id_test.go` の末尾に追加する（import に `bytes`、`encoding/gob` を足す）:

```go
func TestIDGob(t *testing.T) {
	type holder struct {
		ID     ID
		Parent *ID
		List   []ID
		Zero   ID
	}
	a, b := NewID(), NewID()
	in := holder{ID: a, Parent: &b, List: []ID{b, a}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(in); err != nil {
		t.Fatal(err)
	}
	var out holder
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ID != a || out.Parent == nil || *out.Parent != b || len(out.List) != 2 || out.List[0] != b || out.List[1] != a || !out.Zero.IsZero() {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}
```

`internal/schema/validate_test.go` の末尾に追加する:

```go
func TestValidateTaskRank(t *testing.T) {
	task := taskWith(t, "")
	task.Rank = "a0"
	if vs, _ := ValidateTask(task, Default().For(nil)); len(vs) != 0 {
		t.Errorf("valid rank: violations %v", codes(vs))
	}
	task.Rank = "!"
	vs, _ := ValidateTask(task, Default().For(nil))
	if got := codes(vs); len(got) != 1 || got[0] != "bad_value:rank" {
		t.Errorf("bad rank: violations %v, want [bad_value:rank]", got)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/model/ -run TestIDGob -v` と `go test ./internal/schema/ -run TestValidateTaskRank -v`
Expected: `TestIDGob` は FAIL（gob が非公開フィールドだけの型を扱えず、エラーか空の ID になる）。`TestValidateTaskRank` は FAIL（違反が返らない）。

- [ ] **Step 3: 実装する**

`internal/model/id.go` の `IsZero` の下に追加する:

```go
// MarshalBinary writes id as its 16 bytes, so that the index can keep it in
// a gob cache.
func (id ID) MarshalBinary() ([]byte, error) { return id.u.MarshalBinary() }

// UnmarshalBinary reads what MarshalBinary wrote.
func (id *ID) UnmarshalBinary(b []byte) error { return id.u.UnmarshalBinary(b) }
```

`internal/schema/validate.go` の違反コードの定数に2つ足す:

```go
const (
	CodeUnknownKind   = "unknown_kind"
	CodeUnknownStatus = "unknown_status"
	CodeMissingTitle  = "missing_title"
	CodeBadValue      = "bad_value"
	CodePlacement     = "placement"
	CodeTransition    = "transition"
	CodeRequired      = "required"
	// The store and the index report these: they need every task of the
	// store, which the checks here do not see.
	CodeMissingRef = "missing_ref"
	CodeCycle      = "cycle"
)
```

`ValidateTask` の title の検査の直後に足す:

```go
	if t.Rank != "" {
		if err := t.Rank.Validate(); err != nil {
			violations = append(violations, Violation{CodeBadValue, "rank", err.Error()})
		}
	}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/model/ ./internal/schema/ ./internal/store/`
Expected: PASS（store のテストは rank を付けないので影響しない）

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/model/id.go internal/model/id_test.go internal/schema/validate.go internal/schema/validate_test.go
git commit -m "feat: ID を gob で書けるようにし、Rank の不正値を違反にする

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: gitrepo（HEAD のツリー、ツリーの差分、blob の読み出し）

**Files:**
- Create: `internal/gitrepo/tree.go`
- Test: `internal/gitrepo/tree_test.go`

**Interfaces:**
- Consumes: `Repo`（`repo.go`）、テストの `writeFile(t, dir, rel, content)` と `author`（`repo_test.go`）
- Produces:
  - `var ErrTreeNotFound error`
  - `type Change struct { Path string; Deleted bool }`
  - `func (r *Repo) HeadTree() (plumbing.Hash, error)`
  - `func (r *Repo) DiffTrees(from, to plumbing.Hash, prefix string) ([]Change, error)` — `from` か `to` のツリーがなければ `ErrTreeNotFound` を wrap して返す
  - `func (r *Repo) ReadFile(tree plumbing.Hash, path string) ([]byte, error)` — ファイルがなければ `fs.ErrNotExist` を wrap して返す
  - `func (r *Repo) Files(tree plumbing.Hash, prefix string) iter.Seq2[string, error]`

確認済みの go-git v5.19.3 の挙動:
- `object.DiffTree(a, b)` は `DiffTreeWithOptions(ctx, a, b, nil)` を呼び、rename の検出はしない（追加と削除として出る）。各 `Change` の `Action()` は `merkletrie.Insert` / `Modify` / `Delete`。削除のパスは `c.From.Name`、それ以外は `c.To.Name`。
- `Repository.TreeObject(h)` は、ないツリーに `plumbing.ErrObjectNotFound` を返す。
- `Tree.File(path)` は、ないパスに `object.ErrFileNotFound` を返す。
- `object.NewTreeWalker(tree, true, nil)` の `Next()` は `(name string, entry object.TreeEntry, err error)` を返し、終わりで `io.EOF`。`entry.Mode.IsFile()` でファイルかどうかを判断できる。

- [ ] **Step 1: 失敗するテストを書く**

`internal/gitrepo/tree_test.go` を作る:

```go
package gitrepo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// commitFiles writes files (path → content; empty content removes the path)
// and commits them, returning the new HEAD tree.
func commitFiles(t *testing.T, r *Repo, dir string, files map[string]string) plumbing.Hash {
	t.Helper()
	var paths []string
	for p, c := range files {
		if c == "" {
			if err := removeFile(dir, p); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFile(t, dir, p, c)
		}
		paths = append(paths, p)
	}
	if _, err := r.Commit(Commit{Paths: paths, Subject: "x", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	h, err := r.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func removeFile(dir, rel string) error {
	return os.Remove(filepath.Join(dir, filepath.FromSlash(rel)))
}

func TestDiffTrees(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := commitFiles(t, r, dir, map[string]string{
		"tasks/a.md":  "a1",
		"tasks/b.md":  "b1",
		"schema.yaml": "s1",
	})
	second := commitFiles(t, r, dir, map[string]string{
		"tasks/a.md":  "a2", // modified
		"tasks/b.md":  "",   // deleted
		"tasks/c.md":  "c1", // added
		"schema.yaml": "s2",
	})

	got, err := r.DiffTrees(first, second, "tasks/")
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(got, func(x, y Change) int {
		if x.Path < y.Path {
			return -1
		}
		if x.Path > y.Path {
			return 1
		}
		return 0
	})
	want := []Change{{Path: "tasks/a.md"}, {Path: "tasks/b.md", Deleted: true}, {Path: "tasks/c.md"}}
	if !slices.Equal(got, want) {
		t.Errorf("DiffTrees = %v, want %v", got, want)
	}

	all, err := r.DiffTrees(first, second, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Errorf("DiffTrees without prefix = %v, want 4 changes", all)
	}

	none, err := r.DiffTrees(second, second, "")
	if err != nil || len(none) != 0 {
		t.Errorf("DiffTrees of the same tree = %v, %v", none, err)
	}
}

func TestDiffTreesMissingTree(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := commitFiles(t, r, dir, map[string]string{"tasks/a.md": "a"})
	missing := plumbing.NewHash("0123456789abcdef0123456789abcdef01234567")
	if _, err := r.DiffTrees(missing, head, ""); !errors.Is(err, ErrTreeNotFound) {
		t.Errorf("err = %v, want ErrTreeNotFound", err)
	}
	if _, err := r.ReadFile(missing, "tasks/a.md"); !errors.Is(err, ErrTreeNotFound) {
		t.Errorf("ReadFile: err = %v, want ErrTreeNotFound", err)
	}
}

func TestReadFileAndFiles(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := commitFiles(t, r, dir, map[string]string{
		"tasks/x/y/a.md": "hello",
		"tasks/b.md":     "b",
		"other.md":       "o",
	})
	b, err := r.ReadFile(head, "tasks/x/y/a.md")
	if err != nil || string(b) != "hello" {
		t.Errorf("ReadFile = %q, %v", b, err)
	}
	if _, err := r.ReadFile(head, "tasks/nope.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile of a missing path: err = %v, want fs.ErrNotExist", err)
	}

	var got []string
	for p, err := range r.Files(head, "tasks/") {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}
	slices.Sort(got)
	if want := []string{"tasks/b.md", "tasks/x/y/a.md"}; !slices.Equal(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/gitrepo/ -run 'TestDiffTrees|TestReadFileAndFiles' -v`
Expected: FAIL（`HeadTree` などが未定義でコンパイルできない）

- [ ] **Step 3: 実装する**

`internal/gitrepo/tree.go` を作る:

```go
package gitrepo

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// ErrTreeNotFound means a tree the caller named is not in the repository.
var ErrTreeNotFound = errors.New("tree not found")

// Change is a file that differs between two trees.
type Change struct {
	Path    string // relative to the repository root, slash-separated
	Deleted bool
}

// HeadTree returns the tree of the commit HEAD points at.
func (r *Repo) HeadTree() (plumbing.Hash, error) {
	ref, err := r.r.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	c, err := r.r.CommitObject(ref.Hash())
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return c.TreeHash, nil
}

func (r *Repo) tree(h plumbing.Hash) (*object.Tree, error) {
	t, err := r.r.TreeObject(h)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, fmt.Errorf("%s: %w", h, ErrTreeNotFound)
	}
	return t, err
}

// DiffTrees returns the files under prefix that were added, changed or
// removed from tree from to tree to. Renames come as a removal and an
// addition.
func (r *Repo) DiffTrees(from, to plumbing.Hash, prefix string) ([]Change, error) {
	a, err := r.tree(from)
	if err != nil {
		return nil, err
	}
	b, err := r.tree(to)
	if err != nil {
		return nil, err
	}
	cs, err := object.DiffTree(a, b)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, c := range cs {
		action, err := c.Action()
		if err != nil {
			return nil, err
		}
		ch := Change{Path: c.To.Name}
		if action == merkletrie.Delete {
			ch = Change{Path: c.From.Name, Deleted: true}
		}
		if strings.HasPrefix(ch.Path, prefix) {
			out = append(out, ch)
		}
	}
	return out, nil
}

// ReadFile returns the content of path in tree. A missing path is
// fs.ErrNotExist.
func (r *Repo) ReadFile(tree plumbing.Hash, path string) ([]byte, error) {
	t, err := r.tree(tree)
	if err != nil {
		return nil, err
	}
	f, err := t.File(path)
	if errors.Is(err, object.ErrFileNotFound) {
		return nil, fmt.Errorf("%s: %w", path, fs.ErrNotExist)
	}
	if err != nil {
		return nil, err
	}
	rd, err := f.Reader()
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(rd)
	if cerr := rd.Close(); err == nil {
		err = cerr
	}
	return b, err
}

// Files returns the paths of the files under prefix in tree. The order is
// not fixed.
func (r *Repo) Files(tree plumbing.Hash, prefix string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		t, err := r.tree(tree)
		if err != nil {
			yield("", err)
			return
		}
		w := object.NewTreeWalker(t, true, nil)
		defer w.Close()
		for {
			name, e, err := w.Next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield("", err)
				return
			}
			if !e.Mode.IsFile() || !strings.HasPrefix(name, prefix) {
				continue
			}
			if !yield(name, nil) {
				return
			}
		}
	}
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/gitrepo/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/gitrepo/tree.go internal/gitrepo/tree_test.go
git commit -m "feat: HEAD のツリー、ツリーの差分、blob の読み出しを gitrepo に追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: store（.hedgehug/local の読み書き口）

**Files:**
- Create: `internal/store/local.go`
- Test: `internal/store/local_test.go`

**Interfaces:**
- Consumes: `writeFile`、`readFile`（`fs*.go`）、`ErrNotFound`、`Store.path`
- Produces:
  - `func (s *Store) ReadLocal(name string) ([]byte, error)` — なければ `ErrNotFound` を wrap
  - `func (s *Store) WriteLocal(name string, data []byte) error` — 一時ファイル＋rename。ロックは取らない
  - `func (s *Store) Repo() *gitrepo.Repo`
  - `name` は空、`.`、`..`、`/` か `\` を含むものを拒否する

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/local_test.go` を作る:

```go
package store

import (
	"errors"
	"testing"
)

func TestLocalFiles(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ReadLocal("index"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing file: err = %v, want ErrNotFound", err)
	}
	if err := s.WriteLocal("index", []byte("data")); err != nil {
		t.Fatal(err)
	}
	b, err := s.ReadLocal("index")
	if err != nil || string(b) != "data" {
		t.Errorf("ReadLocal = %q, %v", b, err)
	}
	// The file is under .hedgehug/local, which git ignores.
	if _, err := readFile(s.path(".hedgehug/local/index")); err != nil {
		t.Errorf("file not in .hedgehug/local: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`} {
		if err := s.WriteLocal(bad, nil); err == nil {
			t.Errorf("WriteLocal(%q) succeeded", bad)
		}
		if _, err := s.ReadLocal(bad); err == nil {
			t.Errorf("ReadLocal(%q) succeeded", bad)
		}
	}
	if s.Repo() == nil {
		t.Error("Repo() = nil")
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/store/ -run TestLocalFiles -v`
Expected: FAIL（`ReadLocal` が未定義）

- [ ] **Step 3: 実装する**

`internal/store/local.go` を作る:

```go
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/kukv/hedgehug/internal/gitrepo"
)

// localDir holds what belongs to this machine only, like the lock and the
// index. git ignores it.
const localDir = ".hedgehug/local"

// ReadLocal reads the file name of .hedgehug/local. A missing file is
// ErrNotFound.
func (s *Store) ReadLocal(name string) ([]byte, error) {
	if err := validLocalName(name); err != nil {
		return nil, err
	}
	rel := localDir + "/" + name
	b, err := readFile(s.path(rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", rel, ErrNotFound)
	}
	return b, err
}

// WriteLocal replaces the file name of .hedgehug/local with data, through a
// temporary file and a rename. It takes no lock: what is kept here is a
// cache that any writer may replace.
func (s *Store) WriteLocal(name string, data []byte) error {
	if err := validLocalName(name); err != nil {
		return err
	}
	return writeFile(s.path(localDir+"/"+name), data)
}

func validLocalName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid local file name %q", name)
	}
	return nil
}

// Repo returns the git repository of the store, for reading its history.
func (s *Store) Repo() *gitrepo.Repo { return s.repo }
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/store/ -run TestLocalFiles -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/store/local.go internal/store/local_test.go
git commit -m "feat: .hedgehug/local の読み書き口と Repo を Store に追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: store（書き込み時の参照先の存在 `missing_ref`）

**Files:**
- Create: `internal/store/refs.go`
- Test: `internal/store/refs_test.go`
- Modify: `internal/store/validate.go`（`validate`、`lineage`、`sameParent` の削除）
- Modify: `internal/store/validate_test.go`（`TestUpdateMissingParent` の書き換え）

**Interfaces:**
- Consumes: `Tx.task`、`ErrNotFound`、`ValidationError`、`schema.CodeMissingRef`（Task 1）
- Produces:
  - `func refsOf(t *model.Task, sc *schema.Schema) []ref`（`type ref struct{ field string; id model.ID }`）
  - `func (tx *Tx) checkRefs(old, t *model.Task, sc *schema.Schema) ([]schema.Violation, error)`
  - `func (tx *Tx) exists(id model.ID) (bool, error)`
  - `lineage(t, sc, read)` から引数 `newParent` をなくす。直接の親でも祖先でも、見つからない・デコードできないタスクに当たったら、そこでたどるのをやめる（直接の親なら `parentKind` は空文字）。`ErrCycle` の扱いは Task 5 で変えるので、この Task では残す。

背景: 1b では、親を変える書き込みで親が見つからないと `lineage` が `ErrNotFound` を返していた。spec では、これを `missing_ref` の違反に置き換える。デコードできない親も「そこで止める」に含めるのは、壊れた親の下のタスクを編集できるようにするため（index の側でも、壊れた親は種類がわからないものとして扱う）。

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/validate_test.go` の `TestUpdateMissingParent` を次に置き換える:

```go
func TestUpdateMissingParent(t *testing.T) {
	s := newTestStore(t)
	child := newTask("child")
	missing := model.NewID()
	child.Parent = &missing
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(child) })
	wantViolation(t, err, child.ID, schema.CodeMissingRef, "parent")
	if _, err := os.Stat(s.path(model.TaskPath(child.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused task was written: %v", err)
	}

	// Moving an existing task under a missing parent is refused too.
	moved := newTask("moved")
	addTask(t, s, moved)
	before, err := os.ReadFile(s.path(model.TaskPath(moved.ID)))
	if err != nil {
		t.Fatal(err)
	}
	moved.Parent = &missing
	err = s.Update(Op{Name: "mv", Task: moved.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(moved) })
	wantViolation(t, err, moved.ID, schema.CodeMissingRef, "parent")
	if after, err := os.ReadFile(s.path(model.TaskPath(moved.ID))); err != nil || string(after) != string(before) {
		t.Errorf("refused move was written: %q, %v", after, err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2 (init + add)", n)
	}
}
```

`internal/store/refs_test.go` を作る:

```go
package store

import (
	"errors"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// wantViolation checks that err is a ValidationError on task with a
// violation of code on field.
func wantViolation(t *testing.T, err error, task model.ID, code, field string) {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != task {
		t.Errorf("err = %v, want a ValidationError on %s", err, task)
		return
	}
	for _, v := range ve.Violations {
		if v.Code == code && v.Field == field {
			return
		}
	}
	t.Errorf("violations = %v, want %s on %s", ve.Violations, code, field)
}

// idList is a property value listing ids.
func idList(ids ...model.ID) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, id := range ids {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id.String()})
	}
	return n
}

func put(s *Store, tasks ...*model.Task) error {
	return s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		for _, t := range tasks {
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestUpdateMissingBlocker(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	task.BlockedBy = []model.ID{model.NewID()}
	wantViolation(t, put(s, task), task.ID, schema.CodeMissingRef, "blocked_by")
}

func TestUpdateMissingTaskRefProp(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	task.Props.Set("duplicates", idList(model.NewID()))
	wantViolation(t, put(s, task), task.ID, schema.CodeMissingRef, "duplicates")
}

func TestUpdateKeepsOldMissingRef(t *testing.T) {
	s := newTestStore(t)
	blocker, task := newTask("blocker"), newTask("task")
	task.BlockedBy = []model.ID{blocker.ID}
	addTask(t, s, blocker)
	addTask(t, s, task)
	if err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(blocker.ID) }); err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task with an old missing blocker failed: %v", err)
	}
}

func TestUpdateRefToDeletedTask(t *testing.T) {
	s := newTestStore(t)
	gone := newTask("gone")
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	gone.DeletedAt = &at
	addTask(t, s, gone)
	task := newTask("task")
	task.BlockedBy = []model.ID{gone.ID}
	if err := put(s, task); err != nil {
		t.Errorf("a reference to a deleted task was refused: %v", err)
	}
}

func TestUpdateRefToTaskRemovedInSameOp(t *testing.T) {
	s := newTestStore(t)
	blocker := newTask("blocker")
	addTask(t, s, blocker)
	task := newTask("task")
	task.BlockedBy = []model.ID{blocker.ID}
	err := s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.RemoveTask(blocker.ID), tx.PutTask(task))
	})
	wantViolation(t, err, task.ID, schema.CodeMissingRef, "blocked_by")
}

func TestUpdateRefToTaskMadeInSameOp(t *testing.T) {
	s := newTestStore(t)
	blocker, task := newTask("blocker"), newTask("task")
	task.BlockedBy = []model.ID{blocker.ID}
	if err := put(s, task, blocker); err != nil {
		t.Errorf("a reference to a task made in the same operation was refused: %v", err)
	}
}

func TestUpdateUnderBrokenParent(t *testing.T) {
	s := newTestStore(t)
	parent, child := newTask("parent"), newTask("child")
	child.Parent = &parent.ID
	addTask(t, s, parent)
	addTask(t, s, child)
	mustWrite(t, s, model.TaskPath(parent.ID), "not a task file\n")
	got, err := s.Task(child.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task under a broken parent failed: %v", err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/store/ -run 'TestUpdateMissing|TestUpdateKeepsOldMissingRef|TestUpdateRefTo|TestUpdateUnderBrokenParent' -v`
Expected: FAIL（`TestUpdateMissingParent` は `ErrNotFound` が返る。blocked_by と duplicates は拒否されない。`TestUpdateUnderBrokenParent` は親のデコードエラーで失敗する）

- [ ] **Step 3: 実装する**

`internal/store/refs.go` を作る:

```go
package store

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// ref is one reference from a task to another task of the store.
type ref struct {
	field string // parent, blocked_by or the name of a task_ref property
	id    model.ID
}

// refsOf returns the references of t: its parent, its blockers and the
// values of its task_ref properties. Property values that are not IDs are
// left out; ValidateTask reports them.
func refsOf(t *model.Task, sc *schema.Schema) []ref {
	var out []ref
	if t.Parent != nil {
		out = append(out, ref{"parent", *t.Parent})
	}
	for _, id := range t.BlockedBy {
		out = append(out, ref{"blocked_by", id})
	}
	for _, f := range t.Props {
		p, ok := sc.Prop(f.Key)
		if !ok || p.Type != schema.TypeTaskRef || f.Value == nil {
			continue
		}
		items := []*yaml.Node{f.Value}
		if f.Value.Kind == yaml.SequenceNode {
			items = f.Value.Content
		}
		for _, n := range items {
			if n.Kind != yaml.ScalarNode {
				continue
			}
			if id, err := model.ParseID(n.Value); err == nil {
				out = append(out, ref{f.Key, id})
			}
		}
	}
	return out
}

// checkRefs returns the violations of the references that a write of t over
// old adds; old is nil for a new task. A reference old already had is not
// checked, so a task left with a missing reference by a purge or a sync can
// still be edited.
func (tx *Tx) checkRefs(old, t *model.Task, sc *schema.Schema) ([]schema.Violation, error) {
	had := map[ref]bool{}
	if old != nil {
		for _, r := range refsOf(old, sc) {
			had[r] = true
		}
	}
	var vs []schema.Violation
	for _, r := range refsOf(t, sc) {
		if had[r] {
			continue
		}
		had[r] = true // a reference written twice is checked once
		ok, err := tx.exists(r.id)
		if err != nil {
			return nil, err
		}
		if !ok {
			vs = append(vs, schema.Violation{Code: schema.CodeMissingRef, Field: r.field, Msg: fmt.Sprintf("task %s is not in this store", r.id)})
		}
	}
	return vs, nil
}

// exists reports whether the task id is in the store as the operation
// leaves it. A deleted task counts, and so does a file that does not
// decode: the task is there, only broken.
func (tx *Tx) exists(id model.ID) (bool, error) {
	_, err := tx.task(id)
	switch {
	case err == nil, errors.As(err, new(*model.DecodeError)):
		return true, nil
	case errors.Is(err, ErrNotFound):
		return false, nil
	}
	return false, err
}
```

`internal/store/validate.go` を変える。`validate` の本体を次にする:

```go
func (tx *Tx) validate(sc *schema.Schema) error {
	for _, id := range tx.taskOrder {
		t := tx.tasks[id]
		if t == nil {
			continue // removed in the same operation
		}
		old, err := tx.s.Task(id)
		// A file that does not decode is checked like a new task, so a
		// write can repair it.
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			old = nil
		} else if err != nil {
			return err
		}
		parentKind, ancestors, err := lineage(t, sc, tx.task)
		if err != nil {
			return err
		}
		var oldParentKind string
		var oldE *schema.Effective
		if old != nil {
			var oldAncestors []model.ID
			if oldParentKind, oldAncestors, err = lineage(old, sc, tx.s.Task); err != nil {
				return err
			}
			oldE = sc.For(oldAncestors)
		}
		vs := schema.ValidateWrite(old, oldParentKind, oldE, t, parentKind, sc.For(ancestors))
		rvs, err := tx.checkRefs(old, t, sc)
		if err != nil {
			return err
		}
		if vs = append(vs, rvs...); len(vs) > 0 {
			return &ValidationError{Task: id, Violations: vs}
		}
	}
	return nil
}
```

`validate` の上のコメントの末尾に、次の1文を足す: `References to other tasks are checked by checkRefs.`

`lineage` を次にする（`newParent` をなくし、見つからない・デコードできないタスクで止める。`ErrCycle` はこの Task では残す）:

```go
// lineage returns the kind of t's parent, empty at the root, and the IDs
// For needs: t's ancestors nearest first, led by t itself when t is a
// container. Parents are read with read: as the operation leaves them for
// the task being written, from disk for the task as it was.
//
// The walk stops at the first task that is missing or does not decode, and
// keeps the ancestors found so far, with an empty parentKind when the direct
// parent is the one missing. So a task whose parent was purged or broken can
// still be edited; checkRefs refuses a write that adds a missing parent.
func lineage(t *model.Task, sc *schema.Schema, read func(model.ID) (*model.Task, error)) (parentKind string, ancestors []model.ID, err error) {
	if k, ok := sc.Kind(t.Kind); ok && k.Role == schema.RoleContainer {
		ancestors = append(ancestors, t.ID)
	}
	seen := map[model.ID]bool{t.ID: true}
	for cur := t; cur.Parent != nil; {
		id := *cur.Parent
		if seen[id] {
			return "", nil, fmt.Errorf("task %s: %w", t.ID, ErrCycle)
		}
		seen[id] = true
		p, err := read(id)
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			break
		}
		if err != nil {
			return "", nil, fmt.Errorf("parent of %s: %w", cur.ID, err)
		}
		if cur == t {
			parentKind = p.Kind
		}
		ancestors = append(ancestors, id)
		cur = p
	}
	return parentKind, ancestors, nil
}
```

`sameParent` は使われなくなるので消す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/store/ -v`
Expected: PASS（`TestUpdateUnderPurgedParent` と `TestUpdateUnderPurgedGrandparent` も引き続き通る）

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/store/refs.go internal/store/refs_test.go internal/store/validate.go internal/store/validate_test.go
git commit -m "feat: 書き込みで新しく足した参照の先がない場合を拒否する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: store（書き込み時の循環 `cycle`）

**Files:**
- Modify: `internal/store/refs.go`
- Test: `internal/store/refs_test.go`
- Modify: `internal/store/validate.go`（`lineage` の循環の扱い、`ErrCycle` の削除）
- Modify: `internal/store/validate_test.go`（`TestUpdateParentCycle` の書き換え）

**Interfaces:**
- Consumes: Task 4 の `ref`、`refsOf`、`checkRefs`、`exists`、`wantViolation`、`put`
- Produces:
  - `func edges(t *model.Task, field string) []model.ID`
  - `func (tx *Tx) reaches(from, target model.ID, field string) (bool, error)`
  - `checkRefs` が、新しく足した parent / blocked_by の辺で循環ができると `Violation{Code: "cycle", Field: <parent | blocked_by>}` を返す
  - `ErrCycle` をなくす。`lineage` は循環に当たったら止める

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/validate_test.go` の `TestUpdateParentCycle` を次に置き換える:

```go
func TestUpdateParentCycle(t *testing.T) {
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	a.Parent, b.Parent = &b.ID, &a.ID
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(a), tx.PutTask(b))
	})
	wantViolation(t, err, a.ID, schema.CodeCycle, "parent")
}
```

`internal/store/refs_test.go` の末尾に追加する:

```go
func TestUpdateRefusesNewCycles(t *testing.T) {
	tests := []struct {
		name  string
		field string
		build func(a, b, c *model.Task) (setup []*model.Task, write *model.Task)
	}{
		{"parent self", "parent", func(a, _, _ *model.Task) ([]*model.Task, *model.Task) {
			a.Parent = &a.ID
			return nil, a
		}},
		{"parent move under own child", "parent", func(a, b, _ *model.Task) ([]*model.Task, *model.Task) {
			b.Parent = &a.ID
			w := *a
			w.Parent = &b.ID
			return []*model.Task{a, b}, &w
		}},
		{"blocked_by self", "blocked_by", func(a, _, _ *model.Task) ([]*model.Task, *model.Task) {
			a.BlockedBy = []model.ID{a.ID}
			return nil, a
		}},
		{"blocked_by two", "blocked_by", func(a, b, _ *model.Task) ([]*model.Task, *model.Task) {
			b.BlockedBy = []model.ID{a.ID}
			w := *a
			w.BlockedBy = []model.ID{b.ID}
			return []*model.Task{a, b}, &w
		}},
		{"blocked_by three", "blocked_by", func(a, b, c *model.Task) ([]*model.Task, *model.Task) {
			b.BlockedBy = []model.ID{c.ID}
			c.BlockedBy = []model.ID{a.ID}
			w := *a
			w.BlockedBy = []model.ID{b.ID}
			return []*model.Task{a, c, b}, &w // each after what it points at
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			a, b, c := newTask("a"), newTask("b"), newTask("c")
			setup, write := tt.build(a, b, c)
			for _, task := range setup {
				addTask(t, s, task)
			}
			// The setup wrote a; write a fresh copy so the write is a change.
			write.Title = "written"
			wantViolation(t, put(s, write), write.ID, schema.CodeCycle, tt.field)
		})
	}
}

func TestUpdateAllowsEditInOldCycle(t *testing.T) {
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	// A sync or a hand edit may leave cycles; write them past Update.
	mustWrite(t, s, model.TaskPath(a.ID), "---\nid: "+a.ID.String()+"\nkind: task\ntitle: a\nstatus: todo\nparent: "+b.ID.String()+"\nblocked_by: ["+b.ID.String()+"]\n---\n")
	mustWrite(t, s, model.TaskPath(b.ID), "---\nid: "+b.ID.String()+"\nkind: task\ntitle: b\nstatus: todo\nparent: "+a.ID.String()+"\nblocked_by: ["+a.ID.String()+"]\n---\n")
	got, err := s.Task(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task in an old cycle failed: %v", err)
	}
}

func TestUpdateAllowsBlockerChain(t *testing.T) {
	s := newTestStore(t)
	a, b, c := newTask("a"), newTask("b"), newTask("c")
	b.BlockedBy = []model.ID{c.ID}
	addTask(t, s, c)
	addTask(t, s, b)
	a.BlockedBy = []model.ID{b.ID, c.ID}
	if err := put(s, a); err != nil {
		t.Errorf("a chain without a cycle was refused: %v", err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/store/ -run 'TestUpdateParentCycle|TestUpdateRefusesNewCycles|TestUpdateAllowsEditInOldCycle|TestUpdateAllowsBlockerChain' -v`
Expected: FAIL（parent の循環は `ErrCycle` で返り、blocked_by の循環は拒否されない。古い循環の編集は `ErrCycle` で失敗する）

- [ ] **Step 3: 実装する**

`internal/store/refs.go` の `checkRefs` の `if !ok { ... }` のあとに、循環の検査を足す。ループの本体は次のようになる:

```go
	for _, r := range refsOf(t, sc) {
		if had[r] {
			continue
		}
		had[r] = true // a reference written twice is checked once
		ok, err := tx.exists(r.id)
		if err != nil {
			return nil, err
		}
		if !ok {
			vs = append(vs, schema.Violation{Code: schema.CodeMissingRef, Field: r.field, Msg: fmt.Sprintf("task %s is not in this store", r.id)})
			continue
		}
		if r.field != "parent" && r.field != "blocked_by" {
			continue
		}
		cycle, err := tx.reaches(r.id, t.ID, r.field)
		if err != nil {
			return nil, err
		}
		if cycle {
			vs = append(vs, schema.Violation{Code: schema.CodeCycle, Field: r.field, Msg: fmt.Sprintf("%s %s leads back to this task", r.field, r.id)})
		}
	}
```

`checkRefs` のコメントに次の1文を足す: `A new parent or blocker that leads back to t through the same kind of reference is a cycle; cycles made only of references that were already there are let through.`

`refs.go` の末尾に追加する:

```go
// edges returns the tasks t points at through field: its parent or its
// blockers.
func edges(t *model.Task, field string) []model.ID {
	if field == "parent" {
		if t.Parent == nil {
			return nil
		}
		return []model.ID{*t.Parent}
	}
	return t.BlockedBy
}

// reaches reports whether following field from the task from leads to
// target, as the operation leaves the tasks. The walk stops at tasks that
// are missing or do not decode.
func (tx *Tx) reaches(from, target model.ID, field string) (bool, error) {
	seen := map[model.ID]bool{}
	stack := []model.ID{from}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == target {
			return true, nil
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		t, err := tx.task(id)
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			continue
		}
		if err != nil {
			return false, err
		}
		stack = append(stack, edges(t, field)...)
	}
	return false, nil
}
```

`internal/store/validate.go` で、`ErrCycle` の宣言とそのコメントを消し、`lineage` の循環の扱いを次にする:

```go
		if seen[id] {
			break // a cycle the store already had; checkRefs refuses new ones
		}
```

`lineage` のコメントの最後の段落の末尾に、次の1文を足す: `It also stops when it comes back to a task it has seen.`

`fmt` が `validate.go` の他の場所で使われていることを確かめる（`ValidationError.Error` と `lineage` のエラーで使う）。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/store/refs.go internal/store/refs_test.go internal/store/validate.go internal/store/validate_test.go
git commit -m "feat: 書き込みで新しくできる親とブロッカーの循環を拒否する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: index（エントリと全件の構築、子と根、スキーマの違反）

**Files:**
- Create: `internal/index/index.go`
- Create: `internal/index/entry.go`
- Create: `internal/index/build.go`
- Create: `internal/index/helpers_test.go`
- Test: `internal/index/index_test.go`

**Interfaces:**
- Consumes: `store.Store.Repo()`（Task 3）、`gitrepo.Repo.HeadTree / Files / ReadFile`（Task 2）、`schema.ValidateTask / ValidatePlacement / For`、`model.ParseTaskPath / DecodeTask`
- Produces:
  - `type Entry struct { ID model.ID; Key string; FormerKeys []string; Kind, Title string; Parent *model.ID; Rank model.Rank; Status string; Assignees, Labels []string; BlockedBy []model.ID; Props []Prop; Created, Updated time.Time; DeletedAt *time.Time; Text string; Violations []schema.Violation; Broken string }`
  - `type Prop struct { Key string; Values []string }`
  - `func (e *Entry) Prop(key string) []string` — 値がなければ nil
  - `type Index struct`（非公開フィールド: `s *store.Store`、`repo *gitrepo.Repo`、`tree plumbing.Hash`、`sc *schema.Schema`、`tasks map[model.ID]*Entry`、`children map[model.ID][]*Entry`、`roots []*Entry`）
  - `func Open(s *store.Store) (*Index, error)` — この Task では常に全件を作り直す（キャッシュは Task 7）
  - `func (ix *Index) Schema() *schema.Schema`、`Task(id) (*Entry, bool)`、`Tasks() []*Entry`、`Children(id) []*Entry`、`Roots() []*Entry`
  - 非公開: `newEntry(id, data) (*Entry, *model.Task)`、`(ix *Index) derive()`、`rebuild(tree)`、`revalidate(ids, decoded)`、`violations(t)`、`ancestors(t)`、`parentKind(t)`、`readSchema(tree)`、`sortByRank(es)`
  - テスト補助（`helpers_test.go`）: `testAuthor`、`newStore(t) (*store.Store, string)`、`newTask(title) *model.Task`、`put(t, s, tasks...)`、`commitRaw(t, s, dir, rel, content)`、`mustOpen(t, s) *Index`

- [ ] **Step 1: テスト補助と失敗するテストを書く**

`internal/index/helpers_test.go` を作る:

```go
package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// newStore creates a store in a temporary directory and returns it with its
// directory.
func newStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	s, err := store.Create(dir, &model.StoreMeta{ID: model.NewID(), Name: "test"}, testAuthor)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func newTask(title string) *model.Task {
	return &model.Task{ID: model.NewID(), Kind: "task", Title: title, Status: "todo"}
}

// put writes tasks in one operation.
func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	if err := s.Update(store.Op{Name: "update", Author: testAuthor}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// commitRaw writes a file as given and commits it past the store's checks,
// as a sync or a hand edit could.
func commitRaw(t *testing.T, s *store.Store, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

// rawTask is a task file with the given extra frontmatter lines.
func rawTask(id model.ID, lines string) string {
	return "---\nid: " + id.String() + "\n" + lines + "---\n"
}

func mustOpen(t *testing.T, s *store.Store) *Index {
	t.Helper()
	ix, err := Open(s)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func titles(es []*Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Title
	}
	return out
}
```

`internal/index/index_test.go` を作る:

```go
package index

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestOpenIndexesTasks(t *testing.T) {
	s, _ := newStore(t)
	epic := newTask("Epic")
	epic.Kind = "epic"
	first, second := newTask("First"), newTask("Second")
	first.Parent, second.Parent = &epic.ID, &epic.ID
	first.Rank, second.Rank = "a1", "a0"
	first.Labels = []string{"backend"}
	first.Body = "Some BODY text\n"
	first.Props.Set("priority", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "high"})
	put(t, s, epic, first, second)

	ix := mustOpen(t, s)
	e, ok := ix.Task(first.ID)
	if !ok {
		t.Fatal("first is not in the index")
	}
	if e.Title != "First" || e.Kind != "task" || e.Parent == nil || *e.Parent != epic.ID || e.Rank != "a1" || !slices.Equal(e.Labels, []string{"backend"}) {
		t.Errorf("entry = %+v", e)
	}
	if got := e.Prop("priority"); !slices.Equal(got, []string{"high"}) {
		t.Errorf("priority = %v, want [high]", got)
	}
	if !strings.Contains(e.Text, "first") || !strings.Contains(e.Text, "some body text") {
		t.Errorf("text = %q, want the title and body in lower case", e.Text)
	}
	if e.Created.IsZero() || e.Broken != "" || len(e.Violations) != 0 {
		t.Errorf("entry = %+v", e)
	}
	if got := titles(ix.Children(epic.ID)); !slices.Equal(got, []string{"Second", "First"}) {
		t.Errorf("children = %v, want [Second First] (by rank)", got)
	}
	if got := titles(ix.Roots()); !slices.Equal(got, []string{"Epic"}) {
		t.Errorf("roots = %v, want [Epic]", got)
	}
	if len(ix.Tasks()) != 3 {
		t.Errorf("tasks = %d, want 3", len(ix.Tasks()))
	}
	if ix.Schema() == nil || len(ix.Schema().Statuses) == 0 {
		t.Error("schema not read")
	}
}

func TestOpenBrokenFile(t *testing.T) {
	s, dir := newStore(t)
	id := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(id), "not a task file\n")
	other := model.NewID()
	commitRaw(t, s, dir, model.TaskPath(other), rawTask(model.NewID(), "kind: task\ntitle: x\nstatus: todo\n"))

	ix := mustOpen(t, s)
	for _, want := range []model.ID{id, other} {
		e, ok := ix.Task(want)
		if !ok || e.Broken == "" {
			t.Errorf("entry %s = %+v, want a broken entry", want, e)
		}
	}
	if len(ix.Roots()) != 2 {
		t.Errorf("roots = %d, want the 2 broken entries", len(ix.Roots()))
	}
}

func TestOpenViolations(t *testing.T) {
	s, dir := newStore(t)
	parent := newTask("parent")
	put(t, s, parent)
	unknown, misplaced, orphan := model.NewID(), model.NewID(), model.NewID()
	commitRaw(t, s, dir, model.TaskPath(unknown), rawTask(unknown, "kind: nope\ntitle: x\nstatus: todo\n"))
	// A bug cannot sit under a task.
	commitRaw(t, s, dir, model.TaskPath(misplaced), rawTask(misplaced, "kind: bug\ntitle: x\nstatus: todo\nparent: "+parent.ID.String()+"\n"))
	// The parent is missing, so its kind is unknown: placement is not checked.
	commitRaw(t, s, dir, model.TaskPath(orphan), rawTask(orphan, "kind: epic\ntitle: x\nstatus: todo\nparent: "+model.NewID().String()+"\n"))

	ix := mustOpen(t, s)
	check := func(id model.ID, want ...string) {
		t.Helper()
		e, _ := ix.Task(id)
		var got []string
		for _, v := range e.Violations {
			got = append(got, v.Code+":"+v.Field)
		}
		if !slices.Equal(got, want) {
			t.Errorf("violations of %s = %v, want %v", e.Title, got, want)
		}
	}
	check(unknown, schema.CodeUnknownKind+":kind") // once, though two checks report it
	check(misplaced, schema.CodePlacement+":parent")
	check(orphan)
	check(parent.ID)
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/index/ -v`
Expected: FAIL（`Open` などが未定義でコンパイルできない）

- [ ] **Step 3: 実装する**

`internal/index/entry.go` を作る:

```go
package index

import (
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Entry is what the index keeps of one task.
type Entry struct {
	ID         model.ID
	Key        string
	FormerKeys []string
	Kind       string
	Title      string
	Parent     *model.ID
	Rank       model.Rank
	Status     string
	Assignees  []string
	Labels     []string
	BlockedBy  []model.ID
	Props      []Prop // in file order; queries read them by the schema's types
	Created    time.Time
	Updated    time.Time
	DeletedAt  *time.Time
	Text       string             // title and body in lower case, for free-text search
	Violations []schema.Violation // against the schema where the task sits
	Broken     string             // why the file did not decode; empty when it did
}

// Prop is one property of a task.
type Prop struct {
	Key    string
	Values []string // one for a scalar, one per item for a list of scalars, none otherwise
}

// Prop returns the values of the property key; nil when the task has none.
func (e *Entry) Prop(key string) []string {
	for _, p := range e.Props {
		if p.Key == key {
			return p.Values
		}
	}
	return nil
}

// newEntry decodes the task file data, found at the path of id. A file that
// does not decode, or whose id is not the one of its path, gives a broken
// entry and no task.
func newEntry(id model.ID, data []byte) (*Entry, *model.Task) {
	t, err := model.DecodeTask(data)
	if err == nil && t.ID != id {
		err = fmt.Errorf("id %s does not match the file name", t.ID)
	}
	if err != nil {
		return &Entry{ID: id, Broken: err.Error()}, nil
	}
	e := &Entry{
		ID:         t.ID,
		Key:        t.Key,
		FormerKeys: t.FormerKeys,
		Kind:       t.Kind,
		Title:      t.Title,
		Parent:     t.Parent,
		Rank:       t.Rank,
		Status:     t.Status,
		Assignees:  t.Assignees,
		Labels:     t.Labels,
		BlockedBy:  t.BlockedBy,
		Created:    t.Created,
		Updated:    t.Updated,
		DeletedAt:  t.DeletedAt,
		Text:       strings.ToLower(t.Title + "\n" + t.Body),
	}
	for _, f := range t.Props {
		e.Props = append(e.Props, Prop{Key: f.Key, Values: scalars(f.Value)})
	}
	return e, t
}

// scalars returns the values of a property node: one for a scalar, one per
// item for a list of scalars, none for null or any other shape.
func scalars(n *yaml.Node) []string {
	switch {
	case n == nil || n.ShortTag() == "!!null":
		return nil
	case n.Kind == yaml.ScalarNode:
		return []string{n.Value}
	case n.Kind == yaml.SequenceNode:
		var out []string
		for _, it := range n.Content {
			if it.Kind != yaml.ScalarNode {
				return nil
			}
			out = append(out, it.Value)
		}
		return out
	}
	return nil
}
```

`internal/index/index.go` を作る:

```go
// Package index keeps an index of the tasks of a store, for queries and for
// telling tasks apart by short IDs. It reads task files from the tree of
// HEAD, so a change that is not committed is not seen.
package index

import (
	"cmp"
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// Index is the index of one store, as of one tree of HEAD.
type Index struct {
	s     *store.Store
	repo  *gitrepo.Repo
	tree  plumbing.Hash
	sc    *schema.Schema // schema.yaml as tree has it
	tasks map[model.ID]*Entry

	// Derived from tasks by derive.
	children map[model.ID][]*Entry
	roots    []*Entry
}

// Open builds the index of s from the tree of HEAD.
func Open(s *store.Store) (*Index, error) {
	ix := &Index{s: s, repo: s.Repo()}
	head, err := ix.repo.HeadTree()
	if err != nil {
		return nil, err
	}
	if err := ix.rebuild(head); err != nil {
		return nil, err
	}
	return ix, nil
}

// Schema returns schema.yaml as the indexed tree has it.
func (ix *Index) Schema() *schema.Schema { return ix.sc }

// Task returns the entry of id. Deleted tasks are in the index too.
func (ix *Index) Task(id model.ID) (*Entry, bool) {
	e, ok := ix.tasks[id]
	return e, ok
}

// Tasks returns every entry, deleted ones included. The order is not fixed.
func (ix *Index) Tasks() []*Entry { return slices.Collect(maps.Values(ix.tasks)) }

// Children returns the tasks whose parent is id, by rank then ID.
func (ix *Index) Children(id model.ID) []*Entry { return slices.Clone(ix.children[id]) }

// Roots returns the tasks with no parent, and those whose parent is not in
// the index, by rank then ID.
func (ix *Index) Roots() []*Entry { return slices.Clone(ix.roots) }

// derive fills what is computed from the entries. Call it after any change
// to tasks.
func (ix *Index) derive() {
	ix.children = map[model.ID][]*Entry{}
	ix.roots = nil
	for _, e := range ix.tasks {
		if e.Parent != nil {
			ix.children[*e.Parent] = append(ix.children[*e.Parent], e)
		}
		if e.Parent == nil || ix.tasks[*e.Parent] == nil {
			ix.roots = append(ix.roots, e)
		}
	}
	for _, cs := range ix.children {
		sortByRank(cs)
	}
	sortByRank(ix.roots)
}

func sortByRank(es []*Entry) {
	slices.SortFunc(es, func(a, b *Entry) int {
		if c := cmp.Compare(a.Rank, b.Rank); c != 0 {
			return c
		}
		return cmp.Compare(a.ID.String(), b.ID.String())
	})
}
```

`internal/index/build.go` を作る:

```go
package index

import (
	"fmt"
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// readSchema decodes schema.yaml as tree has it.
func (ix *Index) readSchema(tree plumbing.Hash) (*schema.Schema, error) {
	b, err := ix.repo.ReadFile(tree, schema.Path)
	if err != nil {
		return nil, err
	}
	sc, err := schema.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", schema.Path, err)
	}
	return sc, nil
}

// rebuild indexes every task file of tree.
func (ix *Index) rebuild(tree plumbing.Hash) error {
	sc, err := ix.readSchema(tree)
	if err != nil {
		return err
	}
	tasks := map[model.ID]*Entry{}
	decoded := map[model.ID]*model.Task{}
	for p, err := range ix.repo.Files(tree, "tasks/") {
		if err != nil {
			return err
		}
		id, err := model.ParseTaskPath(p)
		if err != nil {
			continue // not a task file
		}
		b, err := ix.repo.ReadFile(tree, p)
		if err != nil {
			return err
		}
		tasks[id], decoded[id] = newEntry(id, b)
	}
	ix.tree, ix.sc, ix.tasks = tree, sc, tasks
	ix.derive()
	return ix.revalidate(slices.Collect(maps.Keys(tasks)), decoded)
}

// revalidate checks the tasks ids against the schema and keeps the
// violations in their entries. decoded holds tasks already decoded; the
// others are read from the tree again. Broken and missing entries are
// skipped.
func (ix *Index) revalidate(ids []model.ID, decoded map[model.ID]*model.Task) error {
	for _, id := range ids {
		e := ix.tasks[id]
		if e == nil || e.Broken != "" {
			continue
		}
		t := decoded[id]
		if t == nil {
			b, err := ix.repo.ReadFile(ix.tree, model.TaskPath(id))
			if err != nil {
				return err
			}
			if t, err = model.DecodeTask(b); err != nil {
				return fmt.Errorf("%s: %w", model.TaskPath(id), err)
			}
		}
		e.Violations = ix.violations(t)
	}
	return nil
}

// violations checks t against the schema where it sits. Placement is not
// checked when the parent is missing or broken, since its kind is unknown.
func (ix *Index) violations(t *model.Task) []schema.Violation {
	e := ix.sc.For(ix.ancestors(t))
	vs, _ := schema.ValidateTask(t, e)
	if parent, known := ix.parentKind(t); known {
		for _, v := range schema.ValidatePlacement(t.Kind, parent, e) {
			if !slices.Contains(vs, v) { // ValidateTask reports an unknown kind too
				vs = append(vs, v)
			}
		}
	}
	return vs
}

// parentKind returns the kind of t's parent, empty at the root, and whether
// it is known: it is not when the parent is missing or broken.
func (ix *Index) parentKind(t *model.Task) (string, bool) {
	if t.Parent == nil {
		return "", true
	}
	p := ix.tasks[*t.Parent]
	if p == nil || p.Broken != "" {
		return "", false
	}
	return p.Kind, true
}

// ancestors returns what Schema.For needs for t: its ancestors nearest
// first, led by t itself when it is a container. The walk stops at a
// missing or broken task, and when it comes back to a task it has seen.
func (ix *Index) ancestors(t *model.Task) []model.ID {
	var out []model.ID
	if k, ok := ix.sc.Kind(t.Kind); ok && k.Role == schema.RoleContainer {
		out = append(out, t.ID)
	}
	seen := map[model.ID]bool{t.ID: true}
	for p := t.Parent; p != nil && !seen[*p]; {
		seen[*p] = true
		e := ix.tasks[*p]
		if e == nil || e.Broken != "" {
			break
		}
		out = append(out, e.ID)
		p = e.Parent
	}
	return out
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/index/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/index/
git commit -m "feat: HEAD のツリーからタスクのインデックスを作る

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: index（キャッシュと差分更新、Refresh）

**Files:**
- Modify: `internal/index/index.go`（`Open` を差し替え、`Refresh` を追加）
- Modify: `internal/index/build.go`（`cache`、`load`、`save`、`update`、`descendants` を追加）
- Test: `internal/index/build_test.go`

**Interfaces:**
- Consumes: Task 6 の全部、`store.Store.ReadLocal / WriteLocal`（Task 3）、`gitrepo.Repo.DiffTrees`、`gitrepo.ErrTreeNotFound`（Task 2）
- Produces:
  - `func Open(s *store.Store) (*Index, error)` — キャッシュを読み、HEAD との差分だけを取り込む
  - `func (ix *Index) Refresh() error` — HEAD が変わっていれば差分を取り込む。エラーを返したあとの `Index` は使わず、`Open` し直す
  - 非公開: `const cacheName = "index"`、`const cacheFormat = 1`、`type cache struct { Format int; Tree plumbing.Hash; Tasks []*Entry }`、`(ix *Index) load() (plumbing.Hash, bool)`、`save()`、`update(from, to) error`、`descendants(ids) []model.ID`

- [ ] **Step 1: 失敗するテストを書く**

`internal/index/build_test.go` を作る:

```go
package index

import (
	"bytes"
	"encoding/gob"
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// readCache decodes the cache file of s.
func readCache(t *testing.T, s *store.Store) cache {
	t.Helper()
	b, err := s.ReadLocal(cacheName)
	if err != nil {
		t.Fatal(err)
	}
	var c cache
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil {
		t.Fatal(err)
	}
	return c
}

func writeCache(t *testing.T, s *store.Store, c cache) {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteLocal(cacheName, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func TestOpenWritesAndUsesCache(t *testing.T) {
	s, _ := newStore(t)
	task := newTask("original")
	put(t, s, task)
	ix := mustOpen(t, s)

	c := readCache(t, s)
	if c.Format != cacheFormat || c.Tree != ix.tree || len(c.Tasks) != 1 || c.Tasks[0].ID != task.ID {
		t.Fatalf("cache = %+v", c)
	}
	// Open trusts a cache for the same tree: an edited title shows that it
	// was read instead of the files.
	c.Tasks[0].Title = "from cache"
	writeCache(t, s, c)
	if e, _ := mustOpen(t, s).Task(task.ID); e.Title != "from cache" {
		t.Errorf("title = %q, want the cached one", e.Title)
	}
}

func TestOpenRebuildsBadCache(t *testing.T) {
	s, _ := newStore(t)
	task := newTask("original")
	put(t, s, task)
	ix := mustOpen(t, s)
	stale := readCache(t, s)
	stale.Tasks[0].Title = "stale"

	tests := []struct {
		name  string
		write func()
	}{
		{"garbage", func() {
			if err := s.WriteLocal(cacheName, []byte("garbage")); err != nil {
				t.Fatal(err)
			}
		}},
		{"other format", func() {
			c := stale
			c.Format = cacheFormat + 1
			writeCache(t, s, c)
		}},
		{"missing tree", func() {
			c := stale
			c.Tree = plumbing.NewHash("0123456789abcdef0123456789abcdef01234567")
			writeCache(t, s, c)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.write()
			e, ok := mustOpen(t, s).Task(task.ID)
			if !ok || e.Title != "original" {
				t.Errorf("entry = %+v, want the title from the files", e)
			}
			if c := readCache(t, s); c.Format != cacheFormat || c.Tree != ix.tree {
				t.Errorf("cache not rewritten: %+v", c)
			}
		})
	}
}

func TestRefreshReadsOnlyChanges(t *testing.T) {
	s, _ := newStore(t)
	kept, edited, purged := newTask("kept"), newTask("edited"), newTask("purged")
	put(t, s, kept, edited, purged)
	ix := mustOpen(t, s)
	keptBefore, _ := ix.Task(kept.ID)

	added := newTask("added")
	edited.Title = "edited again"
	put(t, s, added, edited)
	if err := s.Update(store.Op{Name: "purge", Author: testAuthor}, func(tx *store.Tx) error { return tx.RemoveTask(purged.ID) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}

	if e, ok := ix.Task(added.ID); !ok || e.Title != "added" {
		t.Errorf("added = %+v", e)
	}
	if e, _ := ix.Task(edited.ID); e.Title != "edited again" {
		t.Errorf("edited = %q", e.Title)
	}
	if _, ok := ix.Task(purged.ID); ok {
		t.Error("purged task still indexed")
	}
	if keptAfter, _ := ix.Task(kept.ID); keptAfter != keptBefore {
		t.Error("an unchanged entry was read again")
	}
	if c := readCache(t, s); c.Tree != ix.tree || len(c.Tasks) != 3 {
		t.Errorf("cache = %d tasks at %s, want 3 at %s", len(c.Tasks), c.Tree, ix.tree)
	}
	// Open from that cache gives the same tasks.
	got := titles(mustOpen(t, s).Tasks())
	slices.Sort(got)
	if want := []string{"added", "edited again", "kept"}; !slices.Equal(got, want) {
		t.Errorf("tasks after reopening = %v, want %v", got, want)
	}
}

func TestRefreshRevalidatesOnSchemaChange(t *testing.T) {
	s, dir := newStore(t)
	task := newTask("x")
	put(t, s, task)
	commitRaw(t, s, dir, model.TaskPath(task.ID), rawTask(task.ID, "kind: task\ntitle: x\nstatus: todo\nprops:\n  priority: high\n"))
	ix := mustOpen(t, s)
	if e, _ := ix.Task(task.ID); len(e.Violations) != 0 {
		t.Fatalf("violations = %v", e.Violations)
	}

	sc := schema.Default()
	sc.Props[0].Values = []string{"low", "mid"} // priority without high
	if err := s.Update(store.Op{Name: "schema", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(task.ID); len(e.Violations) != 1 || e.Violations[0].Code != schema.CodeBadValue {
		t.Errorf("violations = %v, want a bad_value on priority", e.Violations)
	}
}

func TestRefreshRevalidatesChildren(t *testing.T) {
	s, _ := newStore(t)
	parent, child := newTask("parent"), newTask("child")
	parent.Kind, child.Kind = "epic", "story"
	child.Parent = &parent.ID
	put(t, s, parent, child)
	ix := mustOpen(t, s)

	parent.Kind = "task" // a story cannot sit under a task
	put(t, s, parent)
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if e, _ := ix.Task(child.ID); len(e.Violations) != 1 || e.Violations[0].Code != schema.CodePlacement {
		t.Errorf("child violations = %v, want a placement violation", e.Violations)
	}
}

func TestRefreshWithoutChange(t *testing.T) {
	s, _ := newStore(t)
	put(t, s, newTask("x"))
	ix := mustOpen(t, s)
	if err := ix.Refresh(); err != nil {
		t.Errorf("Refresh = %v", err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/index/ -run 'TestOpen.*Cache|TestRefresh' -v`
Expected: FAIL（`cache`、`Refresh` が未定義でコンパイルできない）

- [ ] **Step 3: 実装する**

`internal/index/build.go` の import に `bytes`、`encoding/gob`、`github.com/kukv/hedgehug/internal/gitrepo` を足し、次を追加する:

```go
const (
	cacheName   = "index" // in .hedgehug/local
	cacheFormat = 1       // bump when Entry or cache changes
)

// cache is the index as it is kept on disk, for one tree of HEAD.
type cache struct {
	Format int
	Tree   plumbing.Hash
	Tasks  []*Entry
}

// load reads the cache into tasks and returns the tree it is for. It
// reports false when there is no usable cache.
func (ix *Index) load() (plumbing.Hash, bool) {
	b, err := ix.s.ReadLocal(cacheName)
	if err != nil {
		return plumbing.ZeroHash, false
	}
	var c cache
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil || c.Format != cacheFormat {
		return plumbing.ZeroHash, false
	}
	ix.tasks = make(map[model.ID]*Entry, len(c.Tasks))
	for _, e := range c.Tasks {
		ix.tasks[e.ID] = e
	}
	return c.Tree, true
}

// save writes the cache. A failure is ignored: the next Open rebuilds. It
// takes no lock, since a cache is right for its tree whoever writes last.
func (ix *Index) save() {
	c := cache{Format: cacheFormat, Tree: ix.tree, Tasks: slices.Collect(maps.Values(ix.tasks))}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
		return
	}
	_ = ix.s.WriteLocal(cacheName, buf.Bytes())
}

// update brings the index from tree from, which tasks match, to tree to: it
// reads the task files that changed, and checks again the tasks whose
// violations may have changed. A change to schema.yaml checks every task.
func (ix *Index) update(from, to plumbing.Hash) error {
	sc, err := ix.readSchema(to)
	if err != nil {
		return err
	}
	var changes []gitrepo.Change
	if from != to {
		if changes, err = ix.repo.DiffTrees(from, to, ""); err != nil {
			return err
		}
	}
	ix.tree, ix.sc = to, sc
	decoded := map[model.ID]*model.Task{}
	var touched []model.ID
	all := false
	for _, c := range changes {
		if c.Path == schema.Path {
			all = true
			continue
		}
		id, err := model.ParseTaskPath(c.Path)
		if err != nil {
			continue
		}
		touched = append(touched, id)
		if c.Deleted {
			delete(ix.tasks, id)
			continue
		}
		b, err := ix.repo.ReadFile(to, c.Path)
		if err != nil {
			return err
		}
		ix.tasks[id], decoded[id] = newEntry(id, b)
	}
	ix.derive()
	if all {
		return ix.revalidate(slices.Collect(maps.Keys(ix.tasks)), decoded)
	}
	// A task's placement depends on its parent's kind, and its schema on its
	// ancestors, so the descendants of a changed task are checked again.
	return ix.revalidate(ix.descendants(touched), decoded)
}

// descendants returns ids and all their descendants, each once. A cycle of
// parents is walked once.
func (ix *Index) descendants(ids []model.ID) []model.ID {
	seen := map[model.ID]bool{}
	var out []model.ID
	stack := slices.Clone(ids)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		for _, c := range ix.children[id] {
			stack = append(stack, c.ID)
		}
	}
	return out
}
```

`internal/index/index.go` の `Open` を次に置き換え、`Refresh` を足す（import に `errors` を足す）:

```go
// Open opens the index of s. It reads the cache in .hedgehug/local and
// brings it to the tree of HEAD, reading only the task files that changed;
// with no usable cache it reads every task file. It writes the cache back.
func Open(s *store.Store) (*Index, error) {
	ix := &Index{s: s, repo: s.Repo()}
	head, err := ix.repo.HeadTree()
	if err != nil {
		return nil, err
	}
	tree, ok := ix.load()
	if ok {
		err = ix.update(tree, head)
	}
	if !ok || errors.Is(err, gitrepo.ErrTreeNotFound) {
		err = ix.rebuild(head)
	}
	if err != nil {
		return nil, err
	}
	ix.save()
	return ix, nil
}

// Refresh brings the index to the tree of HEAD when HEAD has moved, as after
// a write or a sync. After an error, Open the index again.
func (ix *Index) Refresh() error {
	head, err := ix.repo.HeadTree()
	if err != nil {
		return err
	}
	if head == ix.tree {
		return nil
	}
	err = ix.update(ix.tree, head)
	if errors.Is(err, gitrepo.ErrTreeNotFound) {
		err = ix.rebuild(head)
	}
	if err != nil {
		return err
	}
	ix.save()
	return nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/index/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/index/
git commit -m "feat: インデックスをキャッシュし、HEAD の差分だけを読み直す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: index（短縮 ID と ID の解決）

**Files:**
- Create: `internal/index/shortid.go`
- Modify: `internal/index/index.go`（`Index` に `short map[model.ID]int` を足し、`derive` で計算する）
- Test: `internal/index/shortid_test.go`

**Interfaces:**
- Consumes: Task 6 の `Index`、`derive`、`store.ErrNotFound`
- Produces:
  - `func (ix *Index) ShortID(id model.ID) string`
  - `func (ix *Index) Resolve(s string) (model.ID, error)` — 見つからなければ `store.ErrNotFound` を wrap
  - `type AmbiguousError struct { Input string; Candidates []model.ID }`（`Candidates` は ID の文字列順）
  - 非公開: `const minShort = 4`、`shortLengths(tasks map[model.ID]*Entry) map[model.ID]int`

- [ ] **Step 1: 失敗するテストを書く**

`internal/index/shortid_test.go` を作る:

```go
package index

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func mustID(t *testing.T, s string) model.ID {
	t.Helper()
	id, err := model.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// indexOf builds an index of entries without a store.
func indexOf(es ...*Entry) *Index {
	ix := &Index{tasks: map[model.ID]*Entry{}}
	for _, e := range es {
		ix.tasks[e.ID] = e
	}
	ix.derive()
	return ix
}

func TestShortID(t *testing.T) {
	a := mustID(t, "01JBZ8K3QW7M2X9V4T6RAAA001")
	b := mustID(t, "01JBZ8K3QW7M2X9V4T6RBBB001") // shares the last 4 with a
	c := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0C")
	ix := indexOf(&Entry{ID: a}, &Entry{ID: b}, &Entry{ID: c})
	for id, want := range map[model.ID]string{a: "A001", b: "B001", c: "8P0C"} {
		if got := ix.ShortID(id); got != want {
			t.Errorf("ShortID(%s) = %q, want %q", id, got, want)
		}
	}
	// With another task ending in AAA001, a needs one more character.
	d := mustID(t, "01JBZ8K3QW7M2X9V4T6RCAA001")
	ix = indexOf(&Entry{ID: a}, &Entry{ID: b}, &Entry{ID: d})
	if got := ix.ShortID(a); got != "AAA001" {
		t.Errorf("ShortID(a) = %q, want AAA001", got)
	}
	if got := indexOf(&Entry{ID: c}).ShortID(c); got != "8P0C" {
		t.Errorf("lone task: ShortID = %q, want 4 characters", got)
	}
}

func TestResolve(t *testing.T) {
	a := mustID(t, "01JBZ8K3QW7M2X9V4T6RAAA001")
	b := mustID(t, "01JBZ8K3QW7M2X9V4T6RBBB001")
	d := mustID(t, "01JBZ8K3QW7M2X9V4T6RCAA001") // shares AA001 with a
	ix := indexOf(
		&Entry{ID: a, Key: "WEB-42", FormerKeys: []string{"API-7"}},
		&Entry{ID: b, Key: "WEB-43"},
		&Entry{ID: d},
	)
	tests := []struct {
		in   string
		want model.ID
	}{
		{"WEB-42", a},
		{"web-42", a},
		{"api-7", a},
		{"01JBZ8K3QW7M2X9V4T6RBBB001", b},
		{"01jbz8k3qw7m2x9v4t6rbbb001", b},
		{"AAA001", a},
		{"bbb001", b},
		{"B001", b},
	}
	for _, tt := range tests {
		got, err := ix.Resolve(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Resolve(%q) = %v, %v, want %v", tt.in, got, err, tt.want)
		}
	}

	var amb *AmbiguousError
	if _, err := ix.Resolve("001"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(too short) err = %v, want ErrNotFound", err)
	}
	if _, err := ix.Resolve("A001"); !errors.As(err, &amb) {
		t.Errorf("Resolve(A001) err = %v, want AmbiguousError", err)
	} else if len(amb.Candidates) != 2 || amb.Candidates[0] != a || amb.Candidates[1] != d {
		t.Errorf("candidates = %v", amb.Candidates)
	}
	if _, err := ix.Resolve("ZZZZ"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(ZZZZ) err = %v, want ErrNotFound", err)
	}
	if _, err := ix.Resolve(""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(empty) err = %v, want ErrNotFound", err)
	}
}
```

注意: `"A001"` は a（…AAA001）と d（…CAA001）の両方の末尾なので曖昧になる。`"B001"` は b だけ。

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/index/ -run 'TestShortID|TestResolve' -v`
Expected: FAIL（`ShortID` などが未定義）

- [ ] **Step 3: 実装する**

`internal/index/shortid.go` を作る:

```go
package index

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// minShort is the shortest short ID.
const minShort = 4

// ShortID returns the last characters of id that tell it apart from every
// other task of the store, deleted ones included: at least minShort. The
// end of a ULID is random, while its start is a timestamp that tasks made
// together share. A short ID can grow as tasks are added, so history keeps
// full IDs.
func (ix *Index) ShortID(id model.ID) string {
	s := id.String()
	n, ok := ix.short[id]
	if !ok {
		n = minShort
	}
	return s[len(s)-n:]
}

// shortLengths returns the length of the short ID of each task. Sorting the
// reversed IDs puts the IDs that share the longest ending next to each
// other, so each needs comparing with its neighbors only.
func shortLengths(tasks map[model.ID]*Entry) map[model.ID]int {
	type item struct {
		rev string
		id  model.ID
	}
	items := make([]item, 0, len(tasks))
	for id := range tasks {
		items = append(items, item{reverse(id.String()), id})
	}
	slices.SortFunc(items, func(a, b item) int { return cmp.Compare(a.rev, b.rev) })
	out := make(map[model.ID]int, len(items))
	for i, it := range items {
		n := minShort
		for _, j := range []int{i - 1, i + 1} {
			if j >= 0 && j < len(items) {
				n = max(n, commonPrefix(it.rev, items[j].rev)+1)
			}
		}
		out[it.id] = min(n, len(it.rev))
	}
	return out
}

func reverse(s string) string {
	b := []byte(s) // IDs are ASCII
	slices.Reverse(b)
	return string(b)
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// AmbiguousError means an input matches more than one task.
type AmbiguousError struct {
	Input      string
	Candidates []model.ID // in ID order
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q matches %d tasks", e.Input, len(e.Candidates))
}

// Resolve finds the task s names, ignoring case. It tries in order a key, a
// former key, a full ULID, and the end of a ULID of at least minShort
// characters, and returns what the first stage that matches finds. Deleted
// tasks count.
func (ix *Index) Resolve(s string) (model.ID, error) {
	u := strings.ToUpper(s)
	stages := []func(*Entry) bool{
		func(e *Entry) bool { return e.Key != "" && strings.EqualFold(e.Key, s) },
		func(e *Entry) bool {
			return slices.ContainsFunc(e.FormerKeys, func(k string) bool { return strings.EqualFold(k, s) })
		},
		func(e *Entry) bool { return len(u) == 26 && e.ID.String() == u },
		func(e *Entry) bool { return len(u) >= minShort && strings.HasSuffix(e.ID.String(), u) },
	}
	for _, match := range stages {
		var hits []model.ID
		for _, e := range ix.tasks {
			if match(e) {
				hits = append(hits, e.ID)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		}
		slices.SortFunc(hits, func(a, b model.ID) int { return cmp.Compare(a.String(), b.String()) })
		return model.ID{}, &AmbiguousError{Input: s, Candidates: hits}
	}
	return model.ID{}, fmt.Errorf("%q: %w", s, store.ErrNotFound)
}
```

`internal/index/index.go` の `Index` の「Derived from tasks by derive.」の下に `short map[model.ID]int // length of each short ID` を足し、`derive` の末尾に `ix.short = shortLengths(ix.tasks)` を足す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/index/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/index/
git commit -m "feat: 短縮 ID と、キー・古いキー・短縮 ID による解決を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: index（問題の検出と逆方向の関連）

**Files:**
- Create: `internal/index/problems.go`
- Modify: `internal/index/index.go`（`Index` に `problems map[model.ID][]Problem` を足し、`derive` で計算する）
- Modify: `internal/index/entry.go`（`(*Entry).Targets` を追加）
- Test: `internal/index/problems_test.go`

**Interfaces:**
- Consumes: Task 6 の `Index`、`derive`、Task 8 のテスト補助 `indexOf`、`schema.CodeMissingRef / CodeCycle`（Task 1）
- Produces:
  - `type Problem struct { Code, Field, Msg string }`、`const CodeBroken = "broken"`
  - `func (ix *Index) Problems(id model.ID) []Problem`
  - `func (ix *Index) Reverse(id model.ID, rel string) []*Entry` — ID の文字列順
  - `func (e *Entry) Targets(rel string) []model.ID` — `rel` が `blocked_by` なら `BlockedBy`、それ以外は props の `rel` の値のうち ID として読めるもの
  - 非公開: `findProblems()`、`refs(e)`、`cycles(next)`

注意: `indexOf`（Task 8）は `sc` を設定しない。この Task から `derive` が `ix.sc` を使うので、`indexOf` の `ix := &Index{...}` に `sc: schema.Default()` を足す（`shortid_test.go` の import に `schema` を足す）。

- [ ] **Step 1: 失敗するテストを書く**

`internal/index/problems_test.go` を作る:

```go
package index

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func problemCodes(ps []Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Code+":"+p.Field)
	}
	slices.Sort(out)
	return out
}

func ptr(id model.ID) *model.ID { return &id }

func TestProblems(t *testing.T) {
	a, b, c, d := model.NewID(), model.NewID(), model.NewID(), model.NewID()
	x, y, z, self := model.NewID(), model.NewID(), model.NewID(), model.NewID()
	missing := model.NewID()
	ix := indexOf(
		&Entry{ID: a, Broken: "bad yaml"},
		&Entry{ID: b, Violations: []schema.Violation{{Code: schema.CodeUnknownKind, Field: "kind", Msg: "m"}}},
		&Entry{ID: c, Parent: ptr(missing), BlockedBy: []model.ID{missing}, Props: []Prop{{Key: "duplicates", Values: []string{missing.String(), b.String()}}}},
		&Entry{ID: d, Parent: ptr(x)}, // points into the cycle, is not on it
		&Entry{ID: x, Parent: ptr(y)},
		&Entry{ID: y, Parent: ptr(x), BlockedBy: []model.ID{z}},
		&Entry{ID: z, BlockedBy: []model.ID{y}},
		&Entry{ID: self, BlockedBy: []model.ID{self}},
	)
	tests := []struct {
		id   model.ID
		want []string
	}{
		{a, []string{"broken:"}},
		{b, []string{"unknown_kind:kind"}},
		{c, []string{"missing_ref:blocked_by", "missing_ref:duplicates", "missing_ref:parent"}},
		{d, nil},
		{x, []string{"cycle:parent"}},
		{y, []string{"cycle:blocked_by", "cycle:parent"}},
		{z, []string{"cycle:blocked_by"}},
		{self, []string{"cycle:blocked_by"}},
	}
	for _, tt := range tests {
		if got := problemCodes(ix.Problems(tt.id)); !slices.Equal(got, tt.want) {
			t.Errorf("problems of %s = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestReverse(t *testing.T) {
	a, b, c := model.NewID(), model.NewID(), model.NewID()
	ix := indexOf(
		&Entry{ID: a, Title: "a", BlockedBy: []model.ID{c}, Props: []Prop{
			{Key: "duplicates", Values: []string{c.String()}},
			{Key: "relates", Values: []string{b.String()}},
		}},
		&Entry{ID: b, Title: "b", BlockedBy: []model.ID{c}, Props: []Prop{{Key: "relates", Values: []string{a.String()}}}},
		&Entry{ID: c, Title: "c", Props: []Prop{{Key: "relates", Values: []string{a.String()}}}},
	)
	want := func(es []*Entry, titles ...string) {
		t.Helper()
		var got []string
		for _, e := range es {
			got = append(got, e.Title)
		}
		slices.Sort(got)
		if !slices.Equal(got, titles) {
			t.Errorf("got %v, want %v", got, titles)
		}
	}
	want(ix.Reverse(c, "blocked_by"), "a", "b")
	want(ix.Reverse(c, "duplicates"), "a")
	want(ix.Reverse(a, "duplicates"))
	// relates is symmetric: both directions, each task once.
	want(ix.Reverse(a, "relates"), "b", "c")
	want(ix.Reverse(c, "relates"), "a")
}

func TestTargets(t *testing.T) {
	a, b := model.NewID(), model.NewID()
	e := &Entry{BlockedBy: []model.ID{a}, Props: []Prop{{Key: "duplicates", Values: []string{b.String(), "not an id"}}}}
	if got := e.Targets("blocked_by"); !slices.Equal(got, []model.ID{a}) {
		t.Errorf("Targets(blocked_by) = %v", got)
	}
	if got := e.Targets("duplicates"); !slices.Equal(got, []model.ID{b}) {
		t.Errorf("Targets(duplicates) = %v", got)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/index/ -run 'TestProblems|TestReverse|TestTargets' -v`
Expected: FAIL（`Problems` などが未定義）

- [ ] **Step 3: 実装する**

`internal/index/entry.go` の末尾に追加する:

```go
// Targets returns the tasks e points at through rel: blocked_by, or a
// task_ref property. Property values that are not IDs are left out.
func (e *Entry) Targets(rel string) []model.ID {
	if rel == "blocked_by" {
		return e.BlockedBy
	}
	var out []model.ID
	for _, v := range e.Prop(rel) {
		if id, err := model.ParseID(v); err == nil {
			out = append(out, id)
		}
	}
	return out
}
```

`internal/index/problems.go` を作る:

```go
package index

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// CodeBroken is the problem of a task file that does not decode.
const CodeBroken = "broken"

// Problem is something wrong with a task: what is:invalid finds, and what is
// shown as a warning after a sync.
type Problem struct {
	Code  string // a schema violation code, missing_ref, cycle or broken
	Field string
	Msg   string
}

// Problems returns what is wrong with the task id.
func (ix *Index) Problems(id model.ID) []Problem { return slices.Clone(ix.problems[id]) }

// findProblems finds what is wrong with each task: a file that does not
// decode, schema violations, references to tasks not in the store, and
// cycles of parents or blockers.
func (ix *Index) findProblems() map[model.ID][]Problem {
	out := map[model.ID][]Problem{}
	add := func(id model.ID, p Problem) { out[id] = append(out[id], p) }
	for _, e := range ix.tasks {
		if e.Broken != "" {
			add(e.ID, Problem{CodeBroken, "", e.Broken})
			continue
		}
		for _, v := range e.Violations {
			add(e.ID, Problem{v.Code, v.Field, v.Msg})
		}
		for _, r := range ix.refs(e) {
			if ix.tasks[r.id] == nil {
				add(e.ID, Problem{schema.CodeMissingRef, r.field, fmt.Sprintf("task %s is not in this store", r.id)})
			}
		}
	}
	parents := func(e *Entry) []model.ID {
		if e.Parent == nil {
			return nil
		}
		return []model.ID{*e.Parent}
	}
	for _, id := range ix.cycles(parents) {
		add(id, Problem{schema.CodeCycle, "parent", "the parents form a cycle"})
	}
	for _, id := range ix.cycles(func(e *Entry) []model.ID { return e.BlockedBy }) {
		add(id, Problem{schema.CodeCycle, "blocked_by", "the blockers form a cycle"})
	}
	return out
}

// ref is one reference from a task to another.
type ref struct {
	field string
	id    model.ID
}

// refs returns the references of e: its parent, its blockers and its
// task_ref properties, by the store's schema.
func (ix *Index) refs(e *Entry) []ref {
	var out []ref
	if e.Parent != nil {
		out = append(out, ref{"parent", *e.Parent})
	}
	for _, id := range e.BlockedBy {
		out = append(out, ref{"blocked_by", id})
	}
	for _, p := range e.Props {
		if def, ok := ix.sc.Prop(p.Key); ok && def.Type == schema.TypeTaskRef {
			for _, id := range e.Targets(p.Key) {
				out = append(out, ref{p.Key, id})
			}
		}
	}
	return out
}

// cycles returns the tasks that lie on a cycle of next, by Tarjan's
// algorithm for strongly connected components: a component of more than
// one task, or a task that points at itself. Edges to tasks not in the
// index are ignored.
func (ix *Index) cycles(next func(*Entry) []model.ID) []model.ID {
	order := map[model.ID]int{}
	low := map[model.ID]int{}
	onStack := map[model.ID]bool{}
	var stack, out []model.ID
	var visit func(id model.ID)
	visit = func(id model.ID) {
		n := len(order)
		order[id], low[id] = n, n
		stack = append(stack, id)
		onStack[id] = true
		self := false
		for _, to := range next(ix.tasks[id]) {
			switch {
			case ix.tasks[to] == nil:
			case to == id:
				self = true
			case !hasKey(order, to):
				visit(to)
				low[id] = min(low[id], low[to])
			case onStack[to]:
				low[id] = min(low[id], order[to])
			}
		}
		if low[id] != order[id] {
			return
		}
		var comp []model.ID
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			comp = append(comp, top)
			if top == id {
				break
			}
		}
		if len(comp) > 1 || self {
			out = append(out, comp...)
		}
	}
	for id := range ix.tasks {
		if !hasKey(order, id) {
			visit(id)
		}
	}
	return out
}

func hasKey[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

// Reverse returns the tasks that point at id through rel: blocked_by or a
// task_ref property. For a symmetric property it also returns the tasks id
// points at. Each task appears once, in ID order.
func (ix *Index) Reverse(id model.ID, rel string) []*Entry {
	seen := map[model.ID]bool{}
	var out []*Entry
	add := func(e *Entry) {
		if e != nil && !seen[e.ID] {
			seen[e.ID] = true
			out = append(out, e)
		}
	}
	for _, e := range ix.tasks {
		if slices.Contains(e.Targets(rel), id) {
			add(e)
		}
	}
	if p, ok := ix.sc.Prop(rel); ok && p.Symmetric {
		if self := ix.tasks[id]; self != nil {
			for _, to := range self.Targets(rel) {
				add(ix.tasks[to])
			}
		}
	}
	slices.SortFunc(out, func(a, b *Entry) int { return cmp.Compare(a.ID.String(), b.ID.String()) })
	return out
}
```

`internal/index/index.go` の `Index` の derive の欄に `problems map[model.ID][]Problem` を足し、`derive` の末尾に `ix.problems = ix.findProblems()` を足す。

`internal/index/shortid_test.go` の `indexOf` を `ix := &Index{sc: schema.Default(), tasks: map[model.ID]*Entry{}}` にする。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/index/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/index/
git commit -m "feat: タスクの問題（壊れたファイル・違反・参照先なし・循環）と逆方向の関連をインデックスで求める

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: query（構文解析）

**Files:**
- Create: `internal/query/parse.go`
- Test: `internal/query/parse_test.go`

**Interfaces:**
- Consumes: なし
- Produces:
  - `type Query struct { Terms []Term }`
  - `type Term struct { Pos int; Raw string; Neg bool; Key string; Op string; Values []string }` — `Pos` は1始まり。自由語は `Key` と `Op` が空で `Values` が1つ
  - `type Error struct { Pos int; Term string; Msg string }`（`*Error` が error を実装。構文と意味の両方のエラーに使う）
  - `func Parse(s string) (*Query, error)`

規則（spec の「構文」）:
- 空白（引用符の外）で項に分ける。
- 項の先頭の `-` は否定。
- 引用符の外で最初に現れる `:`、`<=`、`>=`、`<`、`>` で key と値に分ける。なければ自由語。
- 値は引用符の外の `,` で分ける。引用符は取り除く（値の中に `"` そのものは書けない）。
- 空の項（`-` だけ）、空の key、空の値、閉じていない引用符はエラー。

- [ ] **Step 1: 失敗するテストを書く**

`internal/query/parse_test.go` を作る:

```go
package query

import (
	"errors"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want []Term
	}{
		{"", nil},
		{"status:doing", []Term{{Pos: 1, Raw: "status:doing", Key: "status", Op: ":", Values: []string{"doing"}}}},
		{"-label:wip", []Term{{Pos: 1, Raw: "-label:wip", Neg: true, Key: "label", Op: ":", Values: []string{"wip"}}}},
		{"priority:high,urgent", []Term{{Pos: 1, Raw: "priority:high,urgent", Key: "priority", Op: ":", Values: []string{"high", "urgent"}}}},
		{"due<2026-11-10", []Term{{Pos: 1, Raw: "due<2026-11-10", Key: "due", Op: "<", Values: []string{"2026-11-10"}}}},
		{"points>=3", []Term{{Pos: 1, Raw: "points>=3", Key: "points", Op: ">=", Values: []string{"3"}}}},
		{"points<=3", []Term{{Pos: 1, Raw: "points<=3", Key: "points", Op: "<=", Values: []string{"3"}}}},
		{"points>3", []Term{{Pos: 1, Raw: "points>3", Key: "points", Op: ">", Values: []string{"3"}}}},
		{`"検索語"`, []Term{{Pos: 1, Raw: `"検索語"`, Values: []string{"検索語"}}}},
		{"login", []Term{{Pos: 1, Raw: "login", Values: []string{"login"}}}},
		{`-"a b"`, []Term{{Pos: 1, Raw: `-"a b"`, Neg: true, Values: []string{"a b"}}}},
		{`"a:b"`, []Term{{Pos: 1, Raw: `"a:b"`, Values: []string{"a:b"}}}},
		{`label:"needs review"`, []Term{{Pos: 1, Raw: `label:"needs review"`, Key: "label", Op: ":", Values: []string{"needs review"}}}},
		{`label:"a,b",c`, []Term{{Pos: 1, Raw: `label:"a,b",c`, Key: "label", Op: ":", Values: []string{"a,b", "c"}}}},
		{"  kind:task \t is:blocked ", []Term{
			{Pos: 1, Raw: "kind:task", Key: "kind", Op: ":", Values: []string{"task"}},
			{Pos: 2, Raw: "is:blocked", Key: "is", Op: ":", Values: []string{"blocked"}},
		}},
	}
	for _, tt := range tests {
		q, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(q.Terms, tt.want) {
			t.Errorf("Parse(%q) = %+v, want %+v", tt.in, q.Terms, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in  string
		pos int
	}{
		{"status:", 1},
		{":x", 1},
		{"-", 1},
		{`"abc`, 1},
		{`kind:task "abc`, 2},
		{"kind:task label:a,,b", 2},
		{"label:a,", 1},
		{`-""`, 1},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in)
		var qe *Error
		if !errors.As(err, &qe) || qe.Pos != tt.pos {
			t.Errorf("Parse(%q) err = %v, want an Error at term %d", tt.in, err, tt.pos)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"status:doing", `-label:"a,b",c`, `"x`, "due<=2026-11-10", "-", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s) // must not panic
	})
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/query/ -run 'TestParse' -v`
Expected: FAIL（`Parse` が未定義でコンパイルできない）

- [ ] **Step 3: 実装する**

`internal/query/parse.go` を作る:

```go
// Package query parses and runs the query language that hh ls and the
// GUI's filter share, over the index of a store.
package query

import (
	"fmt"
	"strings"
	"unicode"
)

// Query is a parsed query: terms that must all match.
type Query struct{ Terms []Term }

// Term is one space-separated part of a query.
type Term struct {
	Pos    int      // 1-based position in the query
	Raw    string   // the term as written
	Neg    bool     // led by -
	Key    string   // empty for free text
	Op     string   // ":", "<", ">", "<=" or ">="; empty for free text
	Values []string // any of them matches; one for free text
}

// Error is a query that cannot be parsed or run. Pos is the term it is
// about.
type Error struct {
	Pos  int
	Term string
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("term %d (%s): %s", e.Pos, e.Term, e.Msg) }

// Parse parses s. It checks the syntax only; Run checks keys and values
// against the store's schema.
func Parse(s string) (*Query, error) {
	toks, err := split(s)
	if err != nil {
		return nil, err
	}
	q := &Query{}
	for i, tok := range toks {
		t, err := parseTerm(i+1, tok)
		if err != nil {
			return nil, err
		}
		q.Terms = append(q.Terms, t)
	}
	return q, nil
}

// split cuts s at white space outside double quotes. The quotes stay in
// the tokens.
func split(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	quoted := false
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			b.WriteRune(r)
		case unicode.IsSpace(r) && !quoted:
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if quoted {
		return nil, &Error{Pos: len(out) + 1, Term: b.String(), Msg: "unclosed quote"}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out, nil
}

func parseTerm(pos int, tok string) (Term, error) {
	fail := func(msg string) (Term, error) { return Term{}, &Error{Pos: pos, Term: tok, Msg: msg} }
	t := Term{Pos: pos, Raw: tok}
	body := tok
	if strings.HasPrefix(body, "-") {
		t.Neg = true
		body = body[1:]
	}
	if body == "" {
		return fail("empty term")
	}
	i, op := operator(body)
	if i < 0 {
		v := unquote(body)
		if v == "" {
			return fail("empty text")
		}
		t.Values = []string{v}
		return t, nil
	}
	t.Key, t.Op = body[:i], op
	if t.Key == "" {
		return fail("missing key")
	}
	if strings.Contains(t.Key, `"`) {
		return fail("a key cannot be quoted")
	}
	for _, v := range splitValues(body[i+len(op):]) {
		v = unquote(v)
		if v == "" {
			return fail("empty value")
		}
		t.Values = append(t.Values, v)
	}
	return t, nil
}

// operator finds the first ':', "<=", ">=", '<' or '>' outside quotes, and
// returns where it is and which it is; -1 when there is none.
func operator(s string) (int, string) {
	quoted := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == ':':
			return i, ":"
		case c == '<' || c == '>':
			if i+1 < len(s) && s[i+1] == '=' {
				return i, s[i : i+2]
			}
			return i, s[i : i+1]
		}
	}
	return -1, ""
}

// splitValues cuts s at commas outside quotes.
func splitValues(s string) []string {
	var out []string
	quoted, start := false, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

func unquote(s string) string { return strings.ReplaceAll(s, `"`, "") }
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/query/ -v` と `go test ./internal/query/ -run FuzzParse -fuzz FuzzParse -fuzztime 10s`
Expected: PASS（fuzz もパニックしない）

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/query/
git commit -m "feat: クエリ言語の構文解析を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: query（評価）

**Files:**
- Create: `internal/query/eval.go`
- Create: `internal/query/helpers_test.go`
- Test: `internal/query/eval_test.go`

**Interfaces:**
- Consumes: Task 10 の `Query`、`Term`、`Error`。`index.Index` の `Schema / Tasks / Task / Resolve / Problems`、`index.Entry` の `Prop / Targets`（Task 6〜9）
- Produces:
  - `type Options struct { Me []string }`
  - `type Result struct { Groups []Group }`、`type Group struct { Key string; Tasks []*index.Entry }`
  - `func Run(ix *index.Index, q *Query, opt Options) (*Result, error)` — この Task では常に1グループで、rank → ID の順（sort と group は Task 12）
  - 非公開: `type pred func(*index.Entry) bool`、`type compiler struct { ix *index.Index; sc *schema.Schema; opt Options; deleted bool }`、`(c *compiler) term / fail / status / assignee / matchText / resolve / ref / project / inverse / targeted / taskRef / prop / compare / is / blocked`、`var parsers map[schema.PropType]func(string) (float64, error)`、`holds`、`single`、`not`、`anyOf`、`matchAll`、`byRank`

決めごと（spec にない細部）:
- `status:` の値がカテゴリ名とステータス名の両方に当たる（既定のスキーマの `todo`、`done`、`canceled`）ときは、カテゴリとして扱う。親設計の「status にはカテゴリ名も使える」の例がカテゴリの意味で書かれているため。
- `project:none` はエラーにする（「どの祖先にも当たらない」は意味が曖昧なため）。
- 比較できる型の値は float64 に直して比べる: number はそのまま、date と datetime は Unix 秒、duration は分。

- [ ] **Step 1: テスト補助と失敗するテストを書く**

`internal/query/helpers_test.go` を作る:

```go
package query

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// storeDirs maps each test store to its directory, for writing files past
// the store. Tests do not run in parallel, so it needs no lock.
var storeDirs = map[*store.Store]string{}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	s, err := store.Create(dir, &model.StoreMeta{ID: model.NewID(), Name: "test"}, testAuthor)
	if err != nil {
		t.Fatal(err)
	}
	storeDirs[s] = dir
	return s
}

// writeRaw writes a file as given and commits it past the store's checks,
// as a sync or a hand edit could.
func writeRaw(t *testing.T, s *store.Store, rel, content string) {
	t.Helper()
	p := filepath.Join(storeDirs[s], filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: testAuthor, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

func newTask(title string) *model.Task {
	return &model.Task{ID: model.NewID(), Kind: "task", Title: title, Status: "todo"}
}

func setProp(task *model.Task, key, value string) {
	task.Props.Set(key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func setRefs(task *model.Task, key string, ids ...model.ID) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, id := range ids {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id.String()})
	}
	task.Props.Set(key, n)
}

func put(t *testing.T, s *store.Store, tasks ...*model.Task) {
	t.Helper()
	if err := s.Update(store.Op{Name: "update", Author: testAuthor}, func(tx *store.Tx) error {
		for _, task := range tasks {
			if err := tx.PutTask(task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func mustOpen(t *testing.T, s *store.Store) *index.Index {
	t.Helper()
	ix, err := index.Open(s)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// run parses and runs q, and returns the titles of each group.
func run(t *testing.T, ix *index.Index, q string, opt Options) [][]string {
	t.Helper()
	parsed, err := Parse(q)
	if err != nil {
		t.Fatalf("Parse(%q): %v", q, err)
	}
	res, err := Run(ix, parsed, opt)
	if err != nil {
		t.Fatalf("Run(%q): %v", q, err)
	}
	out := make([][]string, len(res.Groups))
	for i, g := range res.Groups {
		out[i] = []string{}
		for _, e := range g.Tasks {
			out[i] = append(out[i], e.Title)
		}
	}
	return out
}
```

`internal/query/eval_test.go` を作る:

```go
package query

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
)

// fixture is a store with tasks P, E, a, b, c and the deleted d, made in
// that order so that their IDs, and with no rank their order, follow it.
//
//	P (project, todo)
//	└── E (epic, todo)
//	    └── a (task, in_progress, label backend, assignee alice,
//	           priority high, due 2026-11-05, estimate 2h, points 3,
//	           blocked by b, duplicates c, body "検索語 で探す")
//	b (task, backlog, key WEB-1, priority urgent, due 2026-11-20,
//	   relates c, body "Fix LOGIN 検索語")
//	c (task, done, label wip, priority low)
//	d (task, todo, deleted)
type fixture struct {
	ix  *index.Index
	ids map[string]model.ID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	s := newStore(t)
	p, e, a, b, c, d := newTask("P"), newTask("E"), newTask("a"), newTask("b"), newTask("c"), newTask("d")
	p.Kind = "project"
	e.Kind, e.Parent = "epic", &p.ID
	a.Parent, a.Status = &e.ID, "in_progress"
	a.Labels, a.Assignees = []string{"backend"}, []string{"github.com/alice"}
	setProp(a, "priority", "high")
	setProp(a, "due", "2026-11-05")
	setProp(a, "estimate", "2h")
	setProp(a, "points", "3")
	a.BlockedBy = []model.ID{b.ID}
	setRefs(a, "duplicates", c.ID)
	a.Body = "検索語 で探す\n"
	b.Status, b.Key = "backlog", "WEB-1"
	setProp(b, "priority", "urgent")
	setProp(b, "due", "2026-11-20")
	setRefs(b, "relates", c.ID)
	b.Body = "Fix LOGIN 検索語\n"
	c.Status, c.Labels = "done", []string{"wip"}
	setProp(c, "priority", "low")
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	d.DeletedAt = &at
	put(t, s, p, e, a, b, c, d)
	f := &fixture{ix: mustOpen(t, s), ids: map[string]model.ID{}}
	for _, task := range []*model.Task{p, e, a, b, c, d} {
		f.ids[task.Title] = task.ID
	}
	return f
}

func TestRunFilters(t *testing.T) {
	f := newFixture(t)
	me := Options{Me: []string{"github.com/alice"}}
	tests := []struct {
		q    string
		opt  Options
		want []string
	}{
		{"", Options{}, []string{"P", "E", "a", "b", "c"}},
		{"status:in_progress", Options{}, []string{"a"}},
		{"status:doing", Options{}, []string{"a"}},
		{"status:todo", Options{}, []string{"P", "E", "b"}}, // the category, which holds backlog
		{"status:in_progress,done", Options{}, []string{"a", "c"}},
		{"-status:done", Options{}, []string{"P", "E", "a", "b"}},
		{"kind:task", Options{}, []string{"a", "b", "c"}},
		{"label:BACKEND", Options{}, []string{"a"}},
		{"label:none", Options{}, []string{"P", "E", "b"}},
		{"assignee:@me", me, []string{"a"}},
		{"assignee:github.com/alice", Options{}, []string{"a"}},
		{"parent:" + f.ids["E"].String(), Options{}, []string{"a"}},
		{"parent:none", Options{}, []string{"P", "b", "c"}},
		{"project:" + f.ids["P"].String(), Options{}, []string{"E", "a"}},
		{"priority:high,urgent", Options{}, []string{"a", "b"}},
		{"due<2026-11-10", Options{}, []string{"a"}},
		{"due>=2026-11-05", Options{}, []string{"a", "b"}},
		{"due:2026-11-20", Options{}, []string{"b"}},
		{"due:none", Options{}, []string{"P", "E", "c"}},
		{"estimate>1h30m", Options{}, []string{"a"}},
		{"points:3", Options{}, []string{"a"}},
		{"blocked_by:" + f.ids["b"].String(), Options{}, []string{"a"}},
		{"blocked_by:web-1", Options{}, []string{"a"}},
		{"blocks:" + f.ids["a"].String(), Options{}, []string{"b"}},
		{"duplicates:" + f.ids["c"].String(), Options{}, []string{"a"}},
		{"重複されている:" + f.ids["a"].String(), Options{}, []string{"c"}},
		{"relates:" + f.ids["c"].String(), Options{}, []string{"b"}},
		{"relates:" + f.ids["b"].String(), Options{}, []string{"c"}}, // symmetric
		{"relates:none", Options{}, []string{"P", "E", "a"}},
		{"is:blocked", Options{}, []string{"a"}},
		{"is:invalid", Options{}, []string{}},
		{"is:deleted", Options{}, []string{"d"}},
		{"-is:deleted", Options{}, []string{"P", "E", "a", "b", "c"}},
		{"検索語", Options{}, []string{"a", "b"}},
		{`"fix login"`, Options{}, []string{"b"}},
		{`-"検索語"`, Options{}, []string{"P", "E", "c"}},
		// The example of the parent design, within phase 1c.
		{`status:doing priority:high,urgent due<2026-11-10 -label:wip kind:task is:blocked "検索語"`, Options{}, []string{"a"}},
	}
	for _, tt := range tests {
		got := run(t, f.ix, tt.q, tt.opt)
		if len(got) != 1 || !slices.Equal(got[0], tt.want) {
			t.Errorf("%q = %v, want [%v]", tt.q, got, tt.want)
		}
	}
}

func TestRunIsInvalid(t *testing.T) {
	s := newStore(t)
	ok := newTask("ok")
	put(t, s, ok)
	bad := model.NewID()
	writeRaw(t, s, model.TaskPath(bad), "---\nid: "+bad.String()+"\nkind: nope\ntitle: bad\nstatus: todo\n---\n")
	if got := run(t, mustOpen(t, s), "is:invalid", Options{}); len(got) != 1 || !slices.Equal(got[0], []string{"bad"}) {
		t.Errorf("is:invalid = %v, want [[bad]]", got)
	}
}

func TestRunErrors(t *testing.T) {
	f := newFixture(t)
	tests := []struct {
		q   string
		opt Options
		pos int
	}{
		{"foo:bar", Options{}, 1},
		{"kind:task status:nope", Options{}, 2},
		{"kind:nope", Options{}, 1},
		{"priority:nope", Options{}, 1},
		{"label<3", Options{}, 1},
		{"priority>high", Options{}, 1},
		{"due<tomorrow", Options{}, 1},
		{"due<none", Options{}, 1},
		{"parent:ZZZZ", Options{}, 1},
		{"assignee:@me", Options{}, 1},
		{"is:nope", Options{}, 1},
		{"project:none", Options{}, 1},
	}
	for _, tt := range tests {
		q, err := Parse(tt.q)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.q, err)
		}
		_, err = Run(f.ix, q, tt.opt)
		var qe *Error
		if !errors.As(err, &qe) || qe.Pos != tt.pos {
			t.Errorf("Run(%q) err = %v, want an Error at term %d", tt.q, err, tt.pos)
		}
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/query/ -run 'TestRun' -v`
Expected: FAIL（`Run` などが未定義でコンパイルできない）

- [ ] **Step 3: 実装する**

`internal/query/eval.go` を作る:

```go
package query

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Options holds what a query needs from outside the store.
type Options struct {
	Me []string // the accounts assignee:@me stands for
}

// Result is what a query found.
type Result struct{ Groups []Group }

// Group is the tasks with one value of the group: field, or all the tasks
// found when the query has no group:.
type Group struct {
	Key   string // the value; empty for tasks with no value, and with no group:
	Tasks []*index.Entry
}

type pred func(*index.Entry) bool

// compiler turns terms into predicates, checking keys and values against
// the schema.
type compiler struct {
	ix      *index.Index
	sc      *schema.Schema
	opt     Options
	deleted bool // the query names is:deleted
}

// Run finds the tasks of ix that match every term of q. Deleted tasks are
// left out unless q names is:deleted.
func Run(ix *index.Index, q *Query, opt Options) (*Result, error) {
	c := &compiler{ix: ix, sc: ix.Schema(), opt: opt}
	var preds []pred
	for _, t := range q.Terms {
		p, err := c.term(t)
		if err != nil {
			return nil, err
		}
		if t.Neg {
			p = not(p)
		}
		preds = append(preds, p)
	}
	hits := []*index.Entry{}
	for _, e := range ix.Tasks() {
		if (e.DeletedAt == nil || c.deleted) && matchAll(preds, e) {
			hits = append(hits, e)
		}
	}
	slices.SortFunc(hits, byRank)
	return &Result{Groups: []Group{{Tasks: hits}}}, nil
}

func (c *compiler) fail(t Term, format string, args ...any) error {
	return &Error{Pos: t.Pos, Term: t.Raw, Msg: fmt.Sprintf(format, args...)}
}

func (c *compiler) term(t Term) (pred, error) {
	if t.Key == "" {
		text := strings.ToLower(t.Values[0])
		return func(e *index.Entry) bool { return strings.Contains(e.Text, text) }, nil
	}
	if t.Op != ":" {
		p, ok := c.sc.Prop(t.Key)
		if !ok || parsers[p.Type] == nil {
			return nil, c.fail(t, "%s cannot be compared with %s", t.Key, t.Op)
		}
		return c.compare(t, p)
	}
	switch t.Key {
	case "status":
		return c.status(t)
	case "kind":
		return c.matchText(t, func(e *index.Entry) []string { return single(e.Kind) }, func(v string) error {
			if _, ok := c.sc.Kind(v); !ok {
				return fmt.Errorf("unknown kind %q", v)
			}
			return nil
		})
	case "label":
		return c.matchText(t, func(e *index.Entry) []string { return e.Labels }, nil)
	case "assignee":
		return c.assignee(t)
	case "parent":
		return c.ref(t, func(e *index.Entry) []model.ID {
			if e.Parent == nil {
				return nil
			}
			return []model.ID{*e.Parent}
		})
	case "project":
		return c.project(t)
	case "blocked_by":
		return c.ref(t, func(e *index.Entry) []model.ID { return e.BlockedBy })
	case "blocks":
		return c.inverse(t, "blocked_by")
	case "is":
		return c.is(t)
	}
	if p, ok := c.sc.Prop(t.Key); ok {
		return c.prop(t, p)
	}
	for _, p := range c.sc.Props {
		if p.Type == schema.TypeTaskRef && p.Inverse == t.Key {
			return c.inverse(t, p.Name)
		}
	}
	return nil, c.fail(t, "unknown key %q", t.Key)
}

// matchText matches when a value of the task equals a value of the term,
// ignoring case; none matches a task with no value. check, when set, refuses
// values the term cannot take.
func (c *compiler) matchText(t Term, get func(*index.Entry) []string, check func(string) error) (pred, error) {
	none := false
	var want []string
	for _, v := range t.Values {
		if v == "none" {
			none = true
			continue
		}
		if check != nil {
			if err := check(v); err != nil {
				return nil, c.fail(t, "%v", err)
			}
		}
		want = append(want, v)
	}
	return func(e *index.Entry) bool {
		got := get(e)
		if none && len(got) == 0 {
			return true
		}
		for _, g := range got {
			for _, w := range want {
				if strings.EqualFold(g, w) {
					return true
				}
			}
		}
		return false
	}, nil
}

// status matches a status or a category, which stands for every status in
// it. A name that is both is taken for the category.
func (c *compiler) status(t Term) (pred, error) {
	var names []string
	for _, v := range t.Values {
		switch {
		case v == "none":
			names = append(names, v)
		case slices.Contains(schema.Categories, schema.Category(v)):
			for _, st := range c.sc.Statuses {
				if st.Category == schema.Category(v) {
					names = append(names, st.Name)
				}
			}
		default:
			if _, ok := c.sc.Status(v); !ok {
				return nil, c.fail(t, "unknown status or category %q", v)
			}
			names = append(names, v)
		}
	}
	t.Values = names
	return c.matchText(t, func(e *index.Entry) []string { return single(e.Status) }, nil)
}

// assignee matches assignees; @me stands for the accounts in Options.Me.
func (c *compiler) assignee(t Term) (pred, error) {
	var vs []string
	for _, v := range t.Values {
		if v != "@me" {
			vs = append(vs, v)
			continue
		}
		if len(c.opt.Me) == 0 {
			return nil, c.fail(t, "@me needs an account to be set up")
		}
		vs = append(vs, c.opt.Me...)
	}
	t.Values = vs
	return c.matchText(t, func(e *index.Entry) []string { return e.Assignees }, nil)
}

// resolve turns the values of t into tasks; none is reported apart.
func (c *compiler) resolve(t Term) (none bool, ids map[model.ID]bool, err error) {
	ids = map[model.ID]bool{}
	for _, v := range t.Values {
		if v == "none" {
			none = true
			continue
		}
		id, err := c.ix.Resolve(v)
		if err != nil {
			return false, nil, c.fail(t, "%v", err)
		}
		ids[id] = true
	}
	return none, ids, nil
}

// ref matches when the task points at one of the term's tasks through get;
// none matches a task that points at none.
func (c *compiler) ref(t Term, get func(*index.Entry) []model.ID) (pred, error) {
	none, ids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	return func(e *index.Entry) bool {
		got := get(e)
		if none && len(got) == 0 {
			return true
		}
		for _, g := range got {
			if ids[g] {
				return true
			}
		}
		return false
	}, nil
}

// project matches tasks that have one of the term's tasks among their
// ancestors.
func (c *compiler) project(t Term) (pred, error) {
	none, ids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	if none {
		return nil, c.fail(t, "project does not take none")
	}
	return func(e *index.Entry) bool {
		seen := map[model.ID]bool{e.ID: true}
		for p := e.Parent; p != nil && !seen[*p]; {
			if ids[*p] {
				return true
			}
			seen[*p] = true
			a, ok := c.ix.Task(*p)
			if !ok {
				break
			}
			p = a.Parent
		}
		return false
	}, nil
}

// inverse matches tasks that one of the term's tasks points at through rel,
// as blocks: does for blocked_by; none matches a task no task points at.
func (c *compiler) inverse(t Term, rel string) (pred, error) {
	none, ids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	pointed := c.targeted(rel)
	return func(e *index.Entry) bool {
		if none && !pointed[e.ID] {
			return true
		}
		for id := range ids {
			if x, ok := c.ix.Task(id); ok && slices.Contains(x.Targets(rel), e.ID) {
				return true
			}
		}
		return false
	}, nil
}

// targeted returns the tasks some task points at through rel.
func (c *compiler) targeted(rel string) map[model.ID]bool {
	out := map[model.ID]bool{}
	for _, e := range c.ix.Tasks() {
		for _, id := range e.Targets(rel) {
			out[id] = true
		}
	}
	return out
}

// taskRef matches tasks whose property p points at one of the term's
// tasks. For a symmetric property the tasks those point back at match too,
// and none matches a task with no such relation either way.
func (c *compiler) taskRef(t Term, p schema.Prop) (pred, error) {
	none, ids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	var back map[model.ID]bool
	if p.Symmetric {
		back = c.targeted(p.Name)
	}
	return func(e *index.Entry) bool {
		got := e.Targets(p.Name)
		if none && len(got) == 0 && !back[e.ID] {
			return true
		}
		for _, g := range got {
			if ids[g] {
				return true
			}
		}
		if p.Symmetric {
			for id := range ids {
				if x, ok := c.ix.Task(id); ok && slices.Contains(x.Targets(p.Name), e.ID) {
					return true
				}
			}
		}
		return false
	}, nil
}

// prop matches a property by the type the schema gives it.
func (c *compiler) prop(t Term, p schema.Prop) (pred, error) {
	get := func(e *index.Entry) []string { return e.Prop(p.Name) }
	switch p.Type {
	case schema.TypeTaskRef:
		return c.taskRef(t, p)
	case schema.TypeEnum, schema.TypeMultiEnum:
		return c.matchText(t, get, func(v string) error {
			if !slices.ContainsFunc(p.Values, func(x string) bool { return strings.EqualFold(x, v) }) {
				return fmt.Errorf("%q is not one of %v", v, p.Values)
			}
			return nil
		})
	case schema.TypeBool:
		return c.matchText(t, get, func(v string) error {
			if v != "true" && v != "false" {
				return fmt.Errorf("%q is not true or false", v)
			}
			return nil
		})
	}
	if parsers[p.Type] != nil {
		return c.compare(t, p)
	}
	return c.matchText(t, get, nil)
}

// parsers turn values of the types that can be compared into numbers:
// dates and times into Unix seconds, durations into minutes.
var parsers = map[schema.PropType]func(string) (float64, error){
	schema.TypeNumber: func(s string) (float64, error) { return strconv.ParseFloat(s, 64) },
	schema.TypeDate: func(s string) (float64, error) {
		d, err := model.ParseDate(s)
		return float64(d.Time().Unix()), err
	},
	schema.TypeDatetime: func(s string) (float64, error) {
		tm, err := time.Parse(time.RFC3339, s)
		return float64(tm.Unix()), err
	},
	schema.TypeDuration: func(s string) (float64, error) {
		d, err := model.ParseDuration(s)
		return time.Duration(d).Minutes(), err
	},
}

// compare matches a property of a type in parsers with the term's operator;
// ":" is equality. Values in the file that do not parse never match.
func (c *compiler) compare(t Term, p schema.Prop) (pred, error) {
	parse := parsers[p.Type]
	none := false
	var want []float64
	for _, v := range t.Values {
		if v == "none" {
			if t.Op != ":" {
				return nil, c.fail(t, "none cannot be compared")
			}
			none = true
			continue
		}
		x, err := parse(v)
		if err != nil {
			return nil, c.fail(t, "%v", err)
		}
		want = append(want, x)
	}
	op := t.Op
	return func(e *index.Entry) bool {
		got := e.Prop(p.Name)
		if none && len(got) == 0 {
			return true
		}
		for _, g := range got {
			x, err := parse(g)
			if err != nil {
				continue
			}
			for _, w := range want {
				if holds(op, x, w) {
					return true
				}
			}
		}
		return false
	}, nil
}

func holds(op string, a, b float64) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return a == b
}

// is matches the built-in conditions.
func (c *compiler) is(t Term) (pred, error) {
	var ps []pred
	for _, v := range t.Values {
		switch v {
		case "blocked":
			ps = append(ps, c.blocked)
		case "invalid":
			ps = append(ps, func(e *index.Entry) bool { return len(c.ix.Problems(e.ID)) > 0 })
		case "deleted":
			c.deleted = true
			ps = append(ps, func(e *index.Entry) bool { return e.DeletedAt != nil })
		default:
			return nil, c.fail(t, "unknown condition is:%s", v)
		}
	}
	return anyOf(ps), nil
}

// blocked reports whether a blocker of e is open: in the index and not in
// the done or canceled category.
func (c *compiler) blocked(e *index.Entry) bool {
	for _, id := range e.BlockedBy {
		b, ok := c.ix.Task(id)
		if !ok {
			continue
		}
		st, _ := c.sc.Status(b.Status)
		if st.Category != schema.CategoryDone && st.Category != schema.CategoryCanceled {
			return true
		}
	}
	return false
}

func single(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func not(p pred) pred { return func(e *index.Entry) bool { return !p(e) } }

func anyOf(ps []pred) pred {
	return func(e *index.Entry) bool {
		for _, p := range ps {
			if p(e) {
				return true
			}
		}
		return false
	}
}

func matchAll(ps []pred, e *index.Entry) bool {
	for _, p := range ps {
		if !p(e) {
			return false
		}
	}
	return true
}

func byRank(a, b *index.Entry) int {
	if c := cmp.Compare(a.Rank, b.Rank); c != 0 {
		return c
	}
	return cmp.Compare(a.ID.String(), b.ID.String())
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/query/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/query/
git commit -m "feat: クエリをインデックスに対して評価する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: query（sort と group）

**Files:**
- Create: `internal/query/sort.go`
- Modify: `internal/query/eval.go`（`compiler` に `order *sortSpec` と `group *field` を足し、`Run` で `sort:` / `group:` を扱う）
- Test: `internal/query/sort_test.go`

**Interfaces:**
- Consumes: Task 11 の `compiler`、`Run`、`single`、`byRank`、`parsers`、テスト補助 `run`、`newStore`、`newTask`、`setProp`、`put`、`mustOpen`
- Produces:
  - 非公開: `type field struct { values func(*index.Entry) []string; multi bool; less func(a, b string) bool }`、`type sortSpec struct { f field; desc bool }`、`(c *compiler) fieldOf(t, name, group) (field, error)`、`(c *compiler) orderBy(t) error`、`sortEntries(es, o)`、`groupEntries(es, f) []Group`、`byPlace`、`byNumber`、`byText`
  - `Run` が `sort:` と `group:` を扱う。`group:` があれば `Groups` は値ごと（タスクがなければ0個）、なければちょうど1つ

規則（spec の「API」）:
- sort の field: `status`、`kind`、`title`、`created`、`updated`、props の名前。group はこれに `label`、`assignee` を足したもの。
- 順序: status はスキーマのステータスの順、kind はスキーマの種類の順、enum / multi_enum は `values` の順、number / date / datetime / duration は値の大小、それ以外は大文字小文字を区別しない文字列の順。
- sort: 空の値は昇順でも降順でも最後。同点は rank → ID。複数値の field は最初の値で比べる。
- group: 複数値の field（label、assignee、multi_enum、`multi: true` の props）は値ごとのグループに1回ずつ出る。空の値のグループ（`Key` は空）は最後。グループの中は sort の順。
- `sort:` / `group:` は1つずつまで。否定、比較演算子、複数の値はエラー。

- [ ] **Step 1: 失敗するテストを書く**

`internal/query/sort_test.go` を作る:

```go
package query

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
)

// sortFixture holds three tasks:
//
//	t1: rank a2, todo, due 2026-11-20, priority low, labels [x y]
//	t2: rank a0, in_progress, no due, priority urgent, labels [y]
//	t3: rank a1, done, due 2026-11-05, priority high, no labels
func sortFixture(t *testing.T) *index.Index {
	t.Helper()
	s := newStore(t)
	t1, t2, t3 := newTask("t1"), newTask("t2"), newTask("t3")
	t1.Rank, t2.Rank, t3.Rank = model.Rank("a2"), model.Rank("a0"), model.Rank("a1")
	t2.Status, t3.Status = "in_progress", "done"
	setProp(t1, "due", "2026-11-20")
	setProp(t3, "due", "2026-11-05")
	setProp(t1, "priority", "low")
	setProp(t2, "priority", "urgent")
	setProp(t3, "priority", "high")
	t1.Labels, t2.Labels = []string{"x", "y"}, []string{"y"}
	put(t, s, t1, t2, t3)
	return mustOpen(t, s)
}

func TestRunSort(t *testing.T) {
	ix := sortFixture(t)
	tests := []struct {
		q    string
		want []string
	}{
		{"", []string{"t2", "t3", "t1"}}, // rank
		{"sort:due", []string{"t3", "t1", "t2"}},
		{"sort:-due", []string{"t1", "t3", "t2"}}, // no value stays last
		{"sort:priority", []string{"t1", "t3", "t2"}},
		{"sort:-priority", []string{"t2", "t3", "t1"}},
		{"sort:title", []string{"t1", "t2", "t3"}},
		{"sort:status", []string{"t1", "t2", "t3"}},
	}
	for _, tt := range tests {
		got := run(t, ix, tt.q, Options{})
		if len(got) != 1 || !reflect.DeepEqual(got[0], tt.want) {
			t.Errorf("%q = %v, want [%v]", tt.q, got, tt.want)
		}
	}
}

func TestRunGroup(t *testing.T) {
	ix := sortFixture(t)
	tests := []struct {
		q        string
		wantKeys []string
		want     [][]string
	}{
		{"group:status", []string{"todo", "in_progress", "done"}, [][]string{{"t1"}, {"t2"}, {"t3"}}},
		{"group:label", []string{"x", "y", ""}, [][]string{{"t1"}, {"t2", "t1"}, {"t3"}}},
		{"group:priority sort:due", []string{"low", "high", "urgent"}, [][]string{{"t1"}, {"t3"}, {"t2"}}},
		{"group:due", []string{"2026-11-05", "2026-11-20", ""}, [][]string{{"t3"}, {"t1"}, {"t2"}}},
		{"group:status kind:bug", []string{}, [][]string{}},
	}
	for _, tt := range tests {
		q, err := Parse(tt.q)
		if err != nil {
			t.Fatal(err)
		}
		res, err := Run(ix, q, Options{})
		if err != nil {
			t.Fatalf("Run(%q): %v", tt.q, err)
		}
		keys := []string{}
		for _, g := range res.Groups {
			keys = append(keys, g.Key)
		}
		if !reflect.DeepEqual(keys, tt.wantKeys) {
			t.Errorf("%q keys = %v, want %v", tt.q, keys, tt.wantKeys)
		}
		if got := run(t, ix, tt.q, Options{}); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q = %v, want %v", tt.q, got, tt.want)
		}
	}
}

func TestRunSortErrors(t *testing.T) {
	ix := sortFixture(t)
	tests := []struct {
		q   string
		pos int
	}{
		{"sort:due sort:title", 2},
		{"group:status group:kind", 2},
		{"sort:label", 1},
		{"group:nope", 1},
		{"-sort:due", 1},
		{"sort<due", 1},
		{"sort:due,title", 1},
	}
	for _, tt := range tests {
		q, err := Parse(tt.q)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Run(ix, q, Options{})
		var qe *Error
		if !errors.As(err, &qe) || qe.Pos != tt.pos {
			t.Errorf("Run(%q) err = %v, want an Error at term %d", tt.q, err, tt.pos)
		}
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/query/ -run 'TestRunSort|TestRunGroup' -v`
Expected: FAIL（`sort:` が未知のキーとしてエラーになる）

- [ ] **Step 3: 実装する**

`internal/query/sort.go` を作る:

```go
package query

import (
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/schema"
)

// field is something a query can sort or group by.
type field struct {
	values func(*index.Entry) []string // empty: no value
	multi  bool                        // group: puts a task in one group per value
	less   func(a, b string) bool      // the order of two values
}

// sortSpec is the sort: of a query.
type sortSpec struct {
	f    field
	desc bool
}

// orderBy reads a sort: or group: term.
func (c *compiler) orderBy(t Term) error {
	switch {
	case t.Neg:
		return c.fail(t, "%s cannot be negated", t.Key)
	case t.Op != ":":
		return c.fail(t, "%s cannot be compared with %s", t.Key, t.Op)
	case len(t.Values) != 1:
		return c.fail(t, "%s takes one field", t.Key)
	}
	name := t.Values[0]
	if t.Key == "group" {
		if c.group != nil {
			return c.fail(t, "a query takes one group:")
		}
		f, err := c.fieldOf(t, name, true)
		if err != nil {
			return err
		}
		c.group = &f
		return nil
	}
	if c.order != nil {
		return c.fail(t, "a query takes one sort:")
	}
	desc := strings.HasPrefix(name, "-")
	f, err := c.fieldOf(t, strings.TrimPrefix(name, "-"), false)
	if err != nil {
		return err
	}
	c.order = &sortSpec{f: f, desc: desc}
	return nil
}

// fieldOf returns the field name. label and assignee can be grouped by
// only.
func (c *compiler) fieldOf(t Term, name string, group bool) (field, error) {
	text := func(get func(*index.Entry) string) func(*index.Entry) []string {
		return func(e *index.Entry) []string { return single(get(e)) }
	}
	when := func(get func(*index.Entry) time.Time) func(*index.Entry) []string {
		return func(e *index.Entry) []string {
			if tm := get(e); !tm.IsZero() {
				return []string{tm.Format(time.RFC3339)}
			}
			return nil
		}
	}
	byTime := byNumber(parsers[schema.TypeDatetime])
	switch name {
	case "status":
		var names []string
		for _, st := range c.sc.Statuses {
			names = append(names, st.Name)
		}
		return field{values: text(func(e *index.Entry) string { return e.Status }), less: byPlace(names)}, nil
	case "kind":
		var names []string
		for _, k := range c.sc.Kinds {
			names = append(names, k.Name)
		}
		return field{values: text(func(e *index.Entry) string { return e.Kind }), less: byPlace(names)}, nil
	case "title":
		return field{values: text(func(e *index.Entry) string { return e.Title }), less: byText}, nil
	case "created":
		return field{values: when(func(e *index.Entry) time.Time { return e.Created }), less: byTime}, nil
	case "updated":
		return field{values: when(func(e *index.Entry) time.Time { return e.Updated }), less: byTime}, nil
	case "label", "assignee":
		if !group {
			return field{}, c.fail(t, "%s can be grouped by, not sorted by", name)
		}
		get := func(e *index.Entry) []string { return e.Labels }
		if name == "assignee" {
			get = func(e *index.Entry) []string { return e.Assignees }
		}
		return field{values: get, multi: true, less: byText}, nil
	}
	p, ok := c.sc.Prop(name)
	if !ok {
		return field{}, c.fail(t, "cannot sort or group by %q", name)
	}
	f := field{
		values: func(e *index.Entry) []string { return e.Prop(p.Name) },
		multi:  p.Multi || p.Type == schema.TypeMultiEnum,
		less:   byText,
	}
	switch {
	case p.Type == schema.TypeEnum || p.Type == schema.TypeMultiEnum:
		f.less = byPlace(p.Values)
	case parsers[p.Type] != nil:
		f.less = byNumber(parsers[p.Type])
	}
	return f, nil
}

// byPlace orders values by their place in order; values not in it come
// after, by string.
func byPlace(order []string) func(a, b string) bool {
	place := func(s string) int {
		if i := slices.Index(order, s); i >= 0 {
			return i
		}
		return len(order)
	}
	return func(a, b string) bool {
		if pa, pb := place(a), place(b); pa != pb {
			return pa < pb
		}
		return a < b
	}
}

// byNumber orders values by what parse makes of them; values that do not
// parse come after, by string.
func byNumber(parse func(string) (float64, error)) func(a, b string) bool {
	return func(a, b string) bool {
		x, errA := parse(a)
		y, errB := parse(b)
		switch {
		case errA == nil && errB == nil && x != y:
			return x < y
		case errA == nil && errB != nil:
			return true
		case errA != nil && errB == nil:
			return false
		}
		return a < b
	}
}

func byText(a, b string) bool { return strings.ToLower(a) < strings.ToLower(b) }

// compare orders two tasks by the sort field. No value comes last, in either
// direction.
func (o *sortSpec) compare(a, b *index.Entry) int {
	va, vb := first(o.f.values(a)), first(o.f.values(b))
	switch {
	case va == "" && vb == "":
		return 0
	case va == "":
		return 1
	case vb == "":
		return -1
	}
	n := 0
	switch {
	case o.f.less(va, vb):
		n = -1
	case o.f.less(vb, va):
		n = 1
	}
	if o.desc {
		n = -n
	}
	return n
}

func first(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// sortEntries sorts es by o, then by rank and ID; by rank and ID alone when
// o is nil.
func sortEntries(es []*index.Entry, o *sortSpec) {
	slices.SortFunc(es, func(a, b *index.Entry) int {
		if o != nil {
			if n := o.compare(a, b); n != 0 {
				return n
			}
		}
		return byRank(a, b)
	})
}

// groupEntries splits es, already sorted, by the values of f. A task with
// several values is in the group of each; the group of tasks with no value
// comes last.
func groupEntries(es []*index.Entry, f field) []Group {
	buckets := map[string][]*index.Entry{}
	for _, e := range es {
		vs := f.values(e)
		if !f.multi && len(vs) > 1 {
			vs = vs[:1]
		}
		if len(vs) == 0 {
			vs = []string{""}
		}
		seen := map[string]bool{}
		for _, v := range vs {
			if !seen[v] {
				seen[v] = true
				buckets[v] = append(buckets[v], e)
			}
		}
	}
	keys := slices.Collect(maps.Keys(buckets))
	slices.SortFunc(keys, func(a, b string) int {
		switch {
		case a == b:
			return 0
		case a == "":
			return 1
		case b == "":
			return -1
		case f.less(a, b):
			return -1
		case f.less(b, a):
			return 1
		}
		return strings.Compare(a, b)
	})
	out := make([]Group, len(keys))
	for i, k := range keys {
		out[i] = Group{Key: k, Tasks: buckets[k]}
	}
	return out
}
```

`internal/query/eval.go` を変える。`compiler` に2つのフィールドを足す:

```go
type compiler struct {
	ix      *index.Index
	sc      *schema.Schema
	opt     Options
	deleted bool      // the query names is:deleted
	order   *sortSpec // sort:, if any
	group   *field    // group:, if any
}
```

`Run` を次にする:

```go
// Run finds the tasks of ix that match every term of q, sorted and grouped
// as its sort: and group: say. Deleted tasks are left out unless q names
// is:deleted.
func Run(ix *index.Index, q *Query, opt Options) (*Result, error) {
	c := &compiler{ix: ix, sc: ix.Schema(), opt: opt}
	var preds []pred
	for _, t := range q.Terms {
		if t.Key == "sort" || t.Key == "group" {
			if err := c.orderBy(t); err != nil {
				return nil, err
			}
			continue
		}
		p, err := c.term(t)
		if err != nil {
			return nil, err
		}
		if t.Neg {
			p = not(p)
		}
		preds = append(preds, p)
	}
	hits := []*index.Entry{}
	for _, e := range ix.Tasks() {
		if (e.DeletedAt == nil || c.deleted) && matchAll(preds, e) {
			hits = append(hits, e)
		}
	}
	sortEntries(hits, c.order)
	if c.group == nil {
		return &Result{Groups: []Group{{Tasks: hits}}}, nil
	}
	return &Result{Groups: groupEntries(hits, *c.group)}, nil
}
```

`groupEntries` は、タスクがなければ空のスライス（`[]Group{}`）を返すこと（`make([]Group, 0)` になるので満たす）。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/query/ -v`
Expected: PASS

- [ ] **Step 5: make check を通してコミットする**

```bash
make check
git add internal/query/
git commit -m "feat: クエリの sort と group を追加する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: 完了条件の確認

**Files:** なし（確認だけ。問題が見つかったら、その原因のタスクのファイルを直す）

- [ ] **Step 1: 全体のチェック**

Run: `make check`
Expected: tidy-check・lint・fmt-check・test がすべて通る

- [ ] **Step 2: ほかの OS でのコンパイル**

Run: `GOOS=windows go vet ./... && GOOS=darwin go vet ./...`
Expected: エラーなし

- [ ] **Step 3: 依存の向き**

Run: `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/... | grep -E '^github.com/kukv/hedgehug/internal/(store|gitrepo|schema|model):'`
Expected: `store` の行に `internal/index` も `internal/query` も出ない。`gitrepo` の行に `internal/model` も `internal/schema` も出ない。

- [ ] **Step 4: spec の完了条件を1つずつ確かめる**

- Store に書いたタスクを index で引け、`Open` を繰り返すとキャッシュから同じ内容が読める（`TestOpenWritesAndUsesCache`）。コミットのたびに差分だけが読み直される（`TestRefreshReadsOnlyChanges`）。
- キー・古いキー・短縮 ID でタスクを解決できる（`TestResolve`）。
- 親設計のクエリの例が期待どおりに評価される（`TestRunFilters` の最後の行）。
- 新しく生じる参照先なしと循環が書き込みで拒否され（`TestUpdateMissing*`、`TestUpdateRefusesNewCycles`）、もとからあるものは `is:invalid` で見つけられる（`TestProblems`、`TestRunIsInvalid`）。

すべて満たしていれば、このタスクではコミットしない。
