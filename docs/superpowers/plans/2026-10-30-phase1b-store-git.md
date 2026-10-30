# フェーズ 1b: Store の読み書きと git コミット Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 1a の codec の上に、Store のディスク読み書き・ロック・原子的な書き込み・1操作1コミットを実装する。

**Architecture:** `internal/gitrepo` が go-git でパス単位の add とコミットを行う。`internal/store` が Store の作成・オープン・読み出しと、`Update(op, fn)` による「ロック → 検査 → 日時の付与と変更の検出 → 原子的な書き込み → コミット（失敗したら巻き戻し）」を担う。制約の規則は `schema.ValidateWrite` に置き、Store は検査の入力（変更前・親の kind・祖先）を集めるだけにする。

**Tech Stack:** Go 1.27.3、`github.com/go-git/go-git/v5` v5.19.3、`golang.org/x/sys`、既存の `go.yaml.in/yaml/v4` と `github.com/oklog/ulid/v2`

**Spec:** `docs/superpowers/specs/2026-10-30-phase1b-store-git-design.md`（実装前に必ず読む）

## Global Constraints

- モジュールは `github.com/kukv/hedgehug`、Go は `go 1.27.3`（`go.mod` のまま）。
- 追加してよい依存は `github.com/go-git/go-git/v5@v5.19.3` と `golang.org/x/sys` だけ。go-git の v6 は使わない。
- 依存の向きは `store → gitrepo`、`store → schema → model`。`gitrepo` は `model` にも `schema` にも依存しない。`model` は `schema` に依存しない。
- テストは標準の `testing` だけで書く（testify などは使わない）。テストは対象と同じパッケージ（`package store` など）に置く。既存のテストの書き方（`t.Fatal`、`t.Errorf("... = %v, want %v")`）に合わせる。
- コード中のコメントは英語（既存コードに合わせる）。コミットメッセージは日本語の Conventional Commits（例: `feat: ...`）で、末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` を付ける。
- 各タスクの最後に `make check`（tidy-check・lint・fmt-check・test）を通す。lint は golangci-lint の standard に revive・gosec・gocritic などを足したもの（`.golangci.yaml`）。gosec がパス変数の読み書き（G304 など）を指摘したら、`//nolint:gosec // <理由>` を付ける。理由は「パスは Store のルートから組み立てる」のように具体的に書く。
- CI は ubuntu / macos / windows の matrix でテストを回す。OS ごとのファイルはビルドタグで分ける（`//go:build unix` / `//go:build windows`）。手元では `GOOS=windows go vet ./...` と `GOOS=darwin go vet ./...` でコンパイルを確かめる。
- ディレクトリのパーミッションは 0750、ファイルは 0600。
- 時刻は `Update` ごとに1つ取り、秒に切り詰める。

## ファイル構成

```
internal/gitrepo/
  repo.go            Repo、Init、Open
  commit.go          Signature、Trailer、Commit、ErrNothingToCommit、(*Repo).Commit
  repo_test.go
internal/store/
  fs.go              writeFile（一時ファイル＋rename）、retryRename
  fs_unix.go         rename、readFile（素の os）
  fs_windows.go      rename（再試行）、readFile（FILE_SHARE_DELETE）
  fs_test.go
  lock.go            ErrLocked、fileLock、acquire、release
  lock_unix.go       tryLock、unlock（flock）
  lock_windows.go    tryLock、unlock（LockFileEx）
  lock_test.go
  store.go           Store、Create、Open、Meta、Schema、ErrNotFound
  read.go            Task、Tasks、Comments、Worklogs、Sprint(s)、Release(s)、Template(s)
  store_test.go
  read_test.go
  helpers_test.go    テスト共通の補助（newTestStore、headLog など）
  tx.go              Op、Tx、Update、変更の検出、巻き戻し、コミットメッセージ
  tx_test.go
  validate.go        ValidationError、SchemaError、ErrCycle、Tx.validate、lineage
  validate_test.go
internal/schema/
  validate.go        ValidateWrite を追加
  validate_test.go   ValidateWrite のテストを追加
```

---

### Task 1: gitrepo（パス単位の add とコミット）

**Files:**
- Create: `internal/gitrepo/repo.go`
- Create: `internal/gitrepo/commit.go`
- Test: `internal/gitrepo/repo_test.go`
- Modify: `go.mod`、`go.sum`

**Interfaces:**
- Consumes: なし
- Produces:
  - `type Signature struct{ Name, Email string }`
  - `type Trailer struct{ Key, Value string }`
  - `type Commit struct { Paths []string; Subject string; Trailers []Trailer; Author Signature; When time.Time }`
  - `var ErrNothingToCommit error`
  - `func Init(dir string) (*Repo, error)` — 既定のブランチは `main`
  - `func Open(dir string) (*Repo, error)`
  - `func (r *Repo) Commit(c Commit) (plumbing.Hash, error)`

確認済みの go-git v5.19.3 の挙動（scratchpad で検証した）:
- `Worktree.AddWithOptions(&git.AddOptions{Path: p, SkipStatus: true})` は全体の status を取らない。ただし `p` がディスクにない（削除した）ときは、`SkipStatus` を付けても全体の status を取る。だから削除は index から直接外す。
- `idx.Remove(p)` のあと `r.Storer.SetIndex(idx)` で書き戻せば、そのパスはコミットのツリーから消える。index にないパスでは `index.ErrEntryNotFound` が返る。
- `Worktree.Commit` は index 全体からツリーを作る。add していない未追跡ファイルは入らない。親のツリーと同じなら `git.ErrEmptyCommit` を返す。

- [ ] **Step 1: 依存を足す**

```bash
go get github.com/go-git/go-git/v5@v5.19.3
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/gitrepo/repo_test.go`:

```go
package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var author = Signature{Name: "Test", Email: "test@example.com"}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func head(t *testing.T, dir string) *object.Commit {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.CommitObject(ref.Hash())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func files(t *testing.T, c *object.Commit) []string {
	t.Helper()
	tree, err := c.Tree()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	if err := tree.Files().ForEach(func(f *object.File) error {
		out = append(out, f.Name)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func TestCommitOnlyNamedPaths(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "tasks/A/B/one.md", "one")
	writeFile(t, dir, "other.md", "not committed")
	when := time.Date(2026, 10, 30, 12, 0, 0, 0, time.FixedZone("", 9*60*60))
	if _, err := r.Commit(Commit{
		Paths:    []string{"tasks/A/B/one.md"},
		Subject:  "hedgehug: add 01JB x",
		Trailers: []Trailer{{"Hedgehug-Op", "add"}, {"Hedgehug-Task", "01JB"}},
		Author:   author,
		When:     when,
	}); err != nil {
		t.Fatal(err)
	}

	c := head(t, dir)
	if got := files(t, c); !slices.Equal(got, []string{"tasks/A/B/one.md"}) {
		t.Errorf("files = %v", got)
	}
	if want := "hedgehug: add 01JB x\n\nHedgehug-Op: add\nHedgehug-Task: 01JB\n"; c.Message != want {
		t.Errorf("message = %q, want %q", c.Message, want)
	}
	if c.Author.Name != "Test" || c.Author.Email != "test@example.com" || !c.Author.When.Equal(when) {
		t.Errorf("author = %+v", c.Author)
	}
	if c.Committer.Name != "Test" || c.Committer.Email != "test@example.com" {
		t.Errorf("committer = %+v", c.Committer)
	}

	ref, err := r.r.Head()
	if err != nil {
		t.Fatal(err)
	}
	if ref.Name().Short() != "main" {
		t.Errorf("branch = %s, want main", ref.Name().Short())
	}
}

func TestCommitRemovedPath(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "a")
	writeFile(t, dir, "b.md", "b")
	if _, err := r.Commit(Commit{Paths: []string{"a.md", "b.md"}, Subject: "two", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "rm", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	c := head(t, dir)
	if got := files(t, c); !slices.Equal(got, []string{"b.md"}) {
		t.Errorf("files = %v", got)
	}
	if c.NumParents() != 1 {
		t.Errorf("parents = %d", c.NumParents())
	}
}

func TestCommitNothing(t *testing.T) {
	r, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(Commit{Subject: "empty", Author: author, When: time.Now()}); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("err = %v, want ErrNothingToCommit", err)
	}
}

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(dir); err != nil {
		t.Fatal(err)
	}
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "a.md", "a")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "a", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.TempDir()); err == nil {
		t.Error("Open of a directory without a repository succeeded")
	}
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/gitrepo/`
Expected: FAIL（`undefined: Init` などのコンパイルエラー）

- [ ] **Step 4: 実装する**

`internal/gitrepo/repo.go`:

```go
// Package gitrepo commits the files of a store with go-git. It knows nothing
// of the file formats; callers name the paths to commit.
package gitrepo

import (
	"fmt"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Repo is a git repository with a work tree.
type Repo struct {
	dir string
	r   *git.Repository
}

// Init creates a repository in dir, with main as its default branch.
func Init(dir string) (*Repo, error) {
	r, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return nil, fmt.Errorf("init %s: %w", dir, err)
	}
	return &Repo{dir: dir, r: r}, nil
}

// Open opens the repository in dir.
func Open(dir string) (*Repo, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, err)
	}
	return &Repo{dir: dir, r: r}, nil
}
```

`internal/gitrepo/commit.go`:

```go
package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ErrNothingToCommit means a commit named no paths.
var ErrNothingToCommit = errors.New("nothing to commit")

// Signature is the author of a commit. The committer is the same.
type Signature struct{ Name, Email string }

// Trailer is one "Key: Value" line at the end of a commit message.
type Trailer struct{ Key, Value string }

// Commit describes one commit.
type Commit struct {
	Paths    []string // relative to the repository root, slash-separated
	Subject  string
	Trailers []Trailer
	Author   Signature
	When     time.Time
}

// Commit stages exactly c.Paths and commits the index. A path that is on
// disk is added; one that is not is dropped from the index.
//
// Paths are staged one by one so that a commit costs the same however many
// files the store holds: go-git's plain Add takes the status of the whole
// work tree.
func (r *Repo) Commit(c Commit) (plumbing.Hash, error) {
	if len(c.Paths) == 0 {
		return plumbing.ZeroHash, ErrNothingToCommit
	}
	w, err := r.r.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	for _, p := range c.Paths {
		_, err := os.Lstat(filepath.Join(r.dir, filepath.FromSlash(p)))
		switch {
		case err == nil:
			// SkipStatus is ignored for a missing path, which is why removals
			// go through unstage instead.
			if err := w.AddWithOptions(&git.AddOptions{Path: p, SkipStatus: true}); err != nil {
				return plumbing.ZeroHash, fmt.Errorf("add %s: %w", p, err)
			}
		case errors.Is(err, fs.ErrNotExist):
			if err := r.unstage(p); err != nil {
				return plumbing.ZeroHash, fmt.Errorf("remove %s: %w", p, err)
			}
		default:
			return plumbing.ZeroHash, err
		}
	}
	sig := &object.Signature{Name: c.Author.Name, Email: c.Author.Email, When: c.When}
	return w.Commit(message(c), &git.CommitOptions{Author: sig, Committer: sig})
}

// unstage drops p from the index without taking the status of the work tree.
func (r *Repo) unstage(p string) error {
	idx, err := r.r.Storer.Index()
	if err != nil {
		return err
	}
	if _, err := idx.Remove(p); err != nil {
		if errors.Is(err, index.ErrEntryNotFound) {
			return nil
		}
		return err
	}
	return r.r.Storer.SetIndex(idx)
}

func message(c Commit) string {
	var b strings.Builder
	b.WriteString(c.Subject)
	b.WriteString("\n")
	if len(c.Trailers) > 0 {
		b.WriteString("\n")
		for _, t := range c.Trailers {
			fmt.Fprintf(&b, "%s: %s\n", t.Key, t.Value)
		}
	}
	return b.String()
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/gitrepo/`
Expected: PASS

- [ ] **Step 6: `make check` を通す**

Run: `make check`
Expected: 成功（go-git が `go.mod` の require に入り、tidy の差分がない）

- [ ] **Step 7: コミット**

```bash
git add go.mod go.sum internal/gitrepo
git commit -m "feat: go-git でパスを指定してコミットする gitrepo を追加する"
```

---

### Task 2: 原子的な書き込みと OS ごとの読み込み・rename

**Files:**
- Create: `internal/store/fs.go`
- Create: `internal/store/fs_unix.go`
- Create: `internal/store/fs_windows.go`
- Test: `internal/store/fs_test.go`
- Modify: `go.mod`、`go.sum`（`golang.org/x/sys` を直接の依存にする）

**Interfaces:**
- Consumes: なし
- Produces（すべて非公開。後のタスクが使う）:
  - `func writeFile(path string, data []byte) error` — 親ディレクトリを作り、一時ファイル＋fsync＋rename で置き換える
  - `func readFile(path string) ([]byte, error)` — Windows では `FILE_SHARE_DELETE` 付きで開く。ないときは `fs.ErrNotExist` を wrap したエラー
  - `func rename(oldpath, newpath string) error` — Windows では一時的なエラーだけ再試行する
  - `func retryRename(rename func(string, string) error, retryable func(error) bool, oldpath, newpath string) error`
  - `const renameRetries = 10`、`var renameInterval = 50 * time.Millisecond`

- [ ] **Step 1: 依存を足す**

```bash
go get golang.org/x/sys@v0.49.0
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/store/fs_test.go`:

```go
package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tasks", "A", "B", "x.md")
	if err := writeFile(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	got, err := readFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Errorf("content = %q, want two", got)
	}
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %v, want only x.md (no temporary files)", entries)
	}
}

func TestReadFileNotExist(t *testing.T) {
	_, err := readFile(filepath.Join(t.TempDir(), "nope.md"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestRetryRename(t *testing.T) {
	old := renameInterval
	renameInterval = time.Millisecond
	t.Cleanup(func() { renameInterval = old })

	errBusy := errors.New("busy")
	errOther := errors.New("other")
	retryable := func(err error) bool { return errors.Is(err, errBusy) }

	for _, tc := range []struct {
		name      string
		fails     int   // calls that fail before one succeeds
		failWith  error // the error of a failing call
		wantCalls int
		wantErr   error
	}{
		{"succeeds at once", 0, nil, 1, nil},
		{"succeeds after retries", 3, errBusy, 4, nil},
		{"does not retry other errors", 5, errOther, 1, errOther},
		{"gives up", 100, errBusy, renameRetries + 1, errBusy},
	} {
		calls := 0
		rename := func(string, string) error {
			calls++
			if calls <= tc.fails {
				return tc.failWith
			}
			return nil
		}
		err := retryRename(rename, retryable, "a", "b")
		if calls != tc.wantCalls || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil && err != nil) {
			t.Errorf("%s: calls = %d, err = %v; want %d, %v", tc.name, calls, err, tc.wantCalls, tc.wantErr)
		}
	}
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/store/`
Expected: FAIL（`undefined: writeFile` など）

- [ ] **Step 4: 実装する**

`internal/store/fs.go`:

```go
// Package store reads and writes the files of one store, and commits every
// write to git as one commit per operation.
package store

import (
	"os"
	"path/filepath"
	"time"
)

// renameRetries is how many times a rename is tried again after a transient
// failure, renameInterval apart.
const renameRetries = 10

var renameInterval = 50 * time.Millisecond

// writeFile replaces the file at path with data. It writes a temporary file
// in the same directory, syncs it and renames it over path, so a reader sees
// either the old or the new content, never a part.
func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// retryRename calls rename until it succeeds, fails with an error retryable
// does not accept, or has been tried again renameRetries times.
func retryRename(rename func(string, string) error, retryable func(error) bool, oldpath, newpath string) error {
	err := rename(oldpath, newpath)
	for i := 0; i < renameRetries && err != nil && retryable(err); i++ {
		time.Sleep(renameInterval)
		err = rename(oldpath, newpath)
	}
	return err
}
```

`internal/store/fs_unix.go`:

```go
//go:build unix

package store

import "os"

func rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // G304: paths are built from the store root
}
```

`internal/store/fs_windows.go`:

```go
//go:build windows

package store

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

// rename retries when another program, such as a virus scanner, has the
// file open for a moment.
func rename(oldpath, newpath string) error {
	return retryRename(os.Rename, transientRenameError, oldpath, newpath)
}

func transientRenameError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// readFile opens path with FILE_SHARE_DELETE, so reading a file does not
// stop another writer from renaming over it.
func readFile(path string) ([]byte, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	return io.ReadAll(f)
}
```

`windows.ERROR_FILE_NOT_FOUND` と `ERROR_PATH_NOT_FOUND` は `syscall.Errno` なので、`errors.Is(err, fs.ErrNotExist)` が真になる。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/store/`
Expected: PASS

- [ ] **Step 6: ほかの OS でコンパイルできることを確かめる**

Run: `GOOS=windows go vet ./internal/store/ && GOOS=darwin go vet ./internal/store/`
Expected: 出力なしで成功

- [ ] **Step 7: `make check` を通す**

Run: `make check`
Expected: 成功（`golang.org/x/sys` が直接の require に移る）

- [ ] **Step 8: コミット**

```bash
git add go.mod go.sum internal/store
git commit -m "feat: 一時ファイルと rename による原子的な書き込みを追加する"
```

---

### Task 3: OS のファイルロック

**Files:**
- Create: `internal/store/lock.go`
- Create: `internal/store/lock_unix.go`
- Create: `internal/store/lock_windows.go`
- Test: `internal/store/lock_test.go`

**Interfaces:**
- Consumes: なし
- Produces:
  - `var ErrLocked error`
  - `const lockWait = 10 * time.Second`、`var lockPoll = 50 * time.Millisecond`
  - `type fileLock struct{ f *os.File }`
  - `func acquire(path string, wait time.Duration) (*fileLock, error)` — 取れなければ `lockPoll` 間隔で取り直し、`wait` を過ぎたら `ErrLocked`
  - `func (l *fileLock) release() error`

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/lock_test.go`:

```go
package store

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLockExcludes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".hedgehug", "local", "lock")
	l, err := acquire(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// A second handle in the same process must not get the lock either.
	if _, err := acquire(path, 100*time.Millisecond); !errors.Is(err, ErrLocked) {
		t.Fatalf("second acquire: err = %v, want ErrLocked", err)
	}
	if err := l.release(); err != nil {
		t.Fatal(err)
	}
	l2, err := acquire(path, time.Second)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := l2.release(); err != nil {
		t.Fatal(err)
	}
}

// TestLockHelperProcess is not a test: TestLockReleasedOnKill runs the test
// binary again with this test selected, to hold the lock in another process.
func TestLockHelperProcess(t *testing.T) {
	path := os.Getenv("HEDGEHUG_LOCK_HELPER")
	if path == "" {
		t.Skip("run by TestLockReleasedOnKill")
	}
	if _, err := acquire(path, time.Second); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("locked")
	time.Sleep(time.Hour) // not select{}: the runtime would call that a deadlock
}

func TestLockReleasedOnKill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(os.Environ(), "HEDGEHUG_LOCK_HELPER="+path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper said %q, %v", line, err)
	}

	if _, err := acquire(path, 100*time.Millisecond); !errors.Is(err, ErrLocked) {
		t.Fatalf("acquire while the helper holds the lock: err = %v, want ErrLocked", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	l, err := acquire(path, 5*time.Second)
	if err != nil {
		t.Fatalf("acquire after the helper was killed: %v", err)
	}
	if err := l.release(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/store/ -run Lock`
Expected: FAIL（`undefined: acquire`）

- [ ] **Step 3: 実装する**

`internal/store/lock.go`:

```go
package store

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ErrLocked means another process kept the store locked for longer than a
// write waits.
var ErrLocked = errors.New("the store is locked by another process")

// lockWait is how long a write waits for the lock, trying every lockPoll.
const lockWait = 10 * time.Second

var lockPoll = 50 * time.Millisecond

// fileLock is an exclusive OS lock on a file. The lock belongs to the file
// handle, so two handles in one process exclude each other, and the OS drops
// it when the process exits.
type fileLock struct{ f *os.File }

func acquire(path string, wait time.Duration) (*fileLock, error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		ok, err := tryLock(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		if ok {
			return &fileLock{f}, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, ErrLocked
		}
		time.Sleep(lockPoll)
	}
}

func (l *fileLock) release() error {
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}
```

`internal/store/lock_unix.go`:

```go
//go:build unix

package store

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) //nolint:gosec // G115: a file descriptor fits in an int
	if errors.Is(err, unix.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

func unlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN) //nolint:gosec // G115: a file descriptor fits in an int
}
```

`internal/store/lock_windows.go`:

```go
//go:build windows

package store

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func tryLock(f *os.File) (bool, error) {
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, new(windows.Overlapped))
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}

func unlock(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/store/ -run Lock -v`
Expected: PASS（`TestLockHelperProcess` は単独では SKIP）

- [ ] **Step 5: ほかの OS でコンパイルできることを確かめる**

Run: `GOOS=windows go vet ./internal/store/ && GOOS=darwin go vet ./internal/store/`
Expected: 成功

- [ ] **Step 6: `make check` を通す**

Run: `make check`
Expected: 成功

- [ ] **Step 7: コミット**

```bash
git add internal/store
git commit -m "feat: Store の書き込みを OS のファイルロックで排他にする"
```

---

### Task 4: Store の作成・オープンと読み出し

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/read.go`
- Create: `internal/store/helpers_test.go`
- Test: `internal/store/store_test.go`、`internal/store/read_test.go`

**Interfaces:**
- Consumes: `gitrepo.Init`、`gitrepo.Open`、`(*gitrepo.Repo).Commit`、`gitrepo.Signature`、`gitrepo.Trailer`（Task 1）。`writeFile`、`readFile`（Task 2）。`lockWait`（Task 3）
- Produces:
  - `var ErrNotFound error`
  - `type Store struct`（非公開フィールド: `dir string`、`repo *gitrepo.Repo`、`mu sync.Mutex`、`meta *model.StoreMeta`、`schema *schema.Schema`、`now func() time.Time`、`lockWait time.Duration`、`commit func(gitrepo.Commit) error`）
  - `func Create(dir string, meta *model.StoreMeta, author gitrepo.Signature) (*Store, error)`
  - `func Open(dir string) (*Store, error)`
  - `func (s *Store) Meta() *model.StoreMeta`、`func (s *Store) Schema() *schema.Schema`
  - `func (s *Store) path(rel string) string` — Store ルートからの相対パス（スラッシュ区切り）を OS のパスにする
  - `const schemaPath = "schema.yaml"`、`const lockPath = ".hedgehug/local/lock"`、`const gitignorePath = ".gitignore"`
  - 読み出し: `Task`、`Tasks`、`Comments`、`Worklogs`、`Sprint`、`Sprints`、`Release`、`Releases`、`Template`、`Templates`（シグネチャは spec の「読み出し」）
  - テスト補助（`helpers_test.go`）: `testNow`、`testAuthor`、`newTestStore(t) *Store`、`headLog(t, dir) []*object.Commit`、`mustWrite(t, s, rel, content)`

- [ ] **Step 1: テスト補助を書く**

`internal/store/helpers_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

var (
	testNow    = time.Date(2026, 10, 30, 12, 0, 0, 0, time.FixedZone("", 9*60*60))
	testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}
)

// newTestStore creates a store in a temporary directory, with a fixed clock.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	meta := &model.StoreMeta{ID: model.NewID(), Name: "test"}
	s, err := Create(filepath.Join(t.TempDir(), "store"), meta, testAuthor)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return testNow }
	return s
}

// headLog returns the commits reachable from HEAD, newest first.
func headLog(t *testing.T, dir string) []*object.Commit {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	it, err := r.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		t.Fatal(err)
	}
	var out []*object.Commit
	if err := it.ForEach(func(c *object.Commit) error {
		out = append(out, c)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// mustWrite writes a file into the store directly, bypassing Update.
func mustWrite(t *testing.T, s *Store, rel, content string) {
	t.Helper()
	p := s.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/store/store_test.go`:

```go
package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestCreateAndOpen(t *testing.T) {
	s := newTestStore(t)
	if s.Meta().Format != model.StoreFormat {
		t.Errorf("format = %d, want %d", s.Meta().Format, model.StoreFormat)
	}

	o, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if o.Meta().ID != s.Meta().ID || o.Meta().Name != "test" {
		t.Errorf("meta = %+v", o.Meta())
	}
	if _, ok := o.Schema().Status("todo"); !ok {
		t.Error("schema.yaml is not the default schema")
	}
	gi, err := os.ReadFile(filepath.Join(s.dir, ".gitignore"))
	if err != nil || string(gi) != ".hedgehug/local/\n" {
		t.Errorf(".gitignore = %q, %v", gi, err)
	}

	log := headLog(t, s.dir)
	if len(log) != 1 {
		t.Fatalf("commits = %d, want 1", len(log))
	}
	if want := "hedgehug: init\n\nHedgehug-Op: init\n"; log[0].Message != want {
		t.Errorf("message = %q, want %q", log[0].Message, want)
	}
	tree, err := log[0].Tree()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".hedgehug/store.yaml", "schema.yaml", ".gitignore"} {
		if _, err := tree.File(p); err != nil {
			t.Errorf("%s is not in the first commit: %v", p, err)
		}
	}
}

func TestCreateRefusesNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(dir, &model.StoreMeta{ID: model.NewID(), Name: "x"}, testAuthor); err == nil {
		t.Error("Create into a non-empty directory succeeded")
	}
}

func TestOpenNotAStore(t *testing.T) {
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
```

`internal/store/read_test.go`:

```go
package store

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

const (
	taskA = "01JBZ8K3QW7M2X9V4T6R5N8P0A"
	taskB = "01JBZ8K3QW7M2X9V4T6R5N8P0B"
	cmt1  = "01JBZ8K3QW7M2X9V4T6R5N8P1A"
	cmt2  = "01JBZ8K3QW7M2X9V4T6R5N8P1B"
)

func id(t *testing.T, s string) model.ID {
	t.Helper()
	v, err := model.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func taskFile(id, title string) string {
	return "---\nid: " + id + "\nkind: task\ntitle: " + title + "\nstatus: todo\n---\n"
}

func TestReadTask(t *testing.T) {
	s := newTestStore(t)
	a := id(t, taskA)
	mustWrite(t, s, model.TaskPath(a), taskFile(taskA, "A"))

	got, err := s.Task(a)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "A" {
		t.Errorf("title = %q", got.Title)
	}
	if _, err := s.Task(id(t, taskB)); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing task: err = %v, want ErrNotFound", err)
	}
}

func TestReadDecodeError(t *testing.T) {
	s := newTestStore(t)
	a := id(t, taskA)
	mustWrite(t, s, model.TaskPath(a), "---\ntitle: no id\n---\n")
	_, err := s.Task(a)
	var de *model.DecodeError
	if !errors.As(err, &de) {
		t.Errorf("err = %v, want a *model.DecodeError", err)
	}
}

func TestTasksSkipsOtherFiles(t *testing.T) {
	s := newTestStore(t)
	a, b := id(t, taskA), id(t, taskB)
	mustWrite(t, s, model.TaskPath(a), taskFile(taskA, "A"))
	mustWrite(t, s, model.TaskPath(b), taskFile(taskB, "B"))
	mustWrite(t, s, "tasks/README.md", "not a task")
	mustWrite(t, s, model.TaskPath(a)+".tmp-123", "a temporary file")

	var titles []string
	for task, err := range s.Tasks() {
		if err != nil {
			t.Fatal(err)
		}
		titles = append(titles, task.Title)
	}
	slices.Sort(titles)
	if !slices.Equal(titles, []string{"A", "B"}) {
		t.Errorf("titles = %v", titles)
	}
}

func TestTasksEmptyStore(t *testing.T) {
	s := newTestStore(t)
	for _, err := range s.Tasks() {
		t.Fatalf("unexpected item, err = %v", err)
	}
}

func TestComments(t *testing.T) {
	s := newTestStore(t)
	a, b := id(t, taskA), id(t, taskB)
	c1, c2 := id(t, cmt1), id(t, cmt2)
	comment := func(id string) string { return "---\nid: " + id + "\nauthor: github.com/me\n---\nbody\n" }
	mustWrite(t, s, model.CommentPath(a, c1), comment(cmt1))
	mustWrite(t, s, model.CommentPath(a, c2), comment(cmt2))
	mustWrite(t, s, model.CommentPath(b, c1), comment(cmt1))

	n := 0
	for c, err := range s.Comments(a) {
		if err != nil {
			t.Fatal(err)
		}
		if c.ID != c1 && c.ID != c2 {
			t.Errorf("unexpected comment %s", c.ID)
		}
		n++
	}
	if n != 2 {
		t.Errorf("comments on A = %d, want 2", n)
	}
}

func TestSprintsReleasesTemplates(t *testing.T) {
	s := newTestStore(t)
	a := id(t, taskA)
	mustWrite(t, s, model.SprintPath(a), "---\nid: "+taskA+"\nname: S1\nstate: planned\n---\n")
	mustWrite(t, s, model.ReleasePath(a), "---\nid: "+taskA+"\nname: v1\nstate: unreleased\n---\n")
	mustWrite(t, s, model.TemplatePath("bug"), "---\nkind: bug\n---\n## 再現手順\n")
	mustWrite(t, s, "sprints/notes.txt", "skipped")
	mustWrite(t, s, "templates/.hidden.md", "skipped")

	if sp, err := s.Sprint(a); err != nil || sp.Name != "S1" {
		t.Errorf("Sprint = %+v, %v", sp, err)
	}
	if r, err := s.Release(a); err != nil || r.Name != "v1" {
		t.Errorf("Release = %+v, %v", r, err)
	}
	if tp, err := s.Template("bug"); err != nil || tp.Name != "bug" || tp.Item.Kind != "bug" {
		t.Errorf("Template = %+v, %v", tp, err)
	}
	if _, err := s.Template("../x"); err == nil {
		t.Error("Template accepted a name outside templates/")
	}

	nSprints, nReleases, nTemplates := 0, 0, 0
	for _, err := range s.Sprints() {
		if err != nil {
			t.Fatal(err)
		}
		nSprints++
	}
	for _, err := range s.Releases() {
		if err != nil {
			t.Fatal(err)
		}
		nReleases++
	}
	for _, err := range s.Templates() {
		if err != nil {
			t.Fatal(err)
		}
		nTemplates++
	}
	if nSprints != 1 || nReleases != 1 || nTemplates != 1 {
		t.Errorf("sprints, releases, templates = %d, %d, %d; want 1 each", nSprints, nReleases, nTemplates)
	}
}
```

`model.TemplateItem` は `Kind` フィールドを持つ（確認済み）。

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/store/`
Expected: FAIL（`undefined: Create` など）

- [ ] **Step 4: `store.go` を実装する**

```go
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

const (
	schemaPath    = "schema.yaml"
	gitignorePath = ".gitignore"
	lockPath      = ".hedgehug/local/lock"
)

// ErrNotFound means a file the caller asked for is not in the store.
var ErrNotFound = errors.New("not found")

// Store is one store: a directory of task files under git.
type Store struct {
	dir  string
	repo *gitrepo.Repo

	mu     sync.Mutex // serializes Update within this Store; the file lock covers other processes
	meta   *model.StoreMeta
	schema *schema.Schema

	// Replaced by tests.
	now      func() time.Time
	lockWait time.Duration
	commit   func(gitrepo.Commit) error
}

func newStore(dir string, repo *gitrepo.Repo, meta *model.StoreMeta, sc *schema.Schema) *Store {
	return &Store{
		dir:      dir,
		repo:     repo,
		meta:     meta,
		schema:   sc,
		now:      time.Now,
		lockWait: lockWait,
		commit: func(c gitrepo.Commit) error {
			_, err := repo.Commit(c)
			return err
		},
	}
}

// Create makes a new store in dir, which must be missing or empty: a git
// repository holding store.yaml, the default schema and a .gitignore, in
// one commit.
func Create(dir string, meta *model.StoreMeta, author gitrepo.Signature) (*Store, error) {
	if err := ensureEmpty(dir); err != nil {
		return nil, err
	}
	repo, err := gitrepo.Init(dir)
	if err != nil {
		return nil, err
	}
	meta.Format = model.StoreFormat
	s := newStore(dir, repo, meta, schema.Default())

	mb, err := meta.Encode()
	if err != nil {
		return nil, err
	}
	sb, err := s.schema.Encode()
	if err != nil {
		return nil, err
	}
	files := []struct {
		rel  string
		data []byte
	}{
		{model.StoreMetaPath, mb},
		{schemaPath, sb},
		{gitignorePath, []byte(".hedgehug/local/\n")},
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		if err := writeFile(s.path(f.rel), f.data); err != nil {
			return nil, err
		}
		paths = append(paths, f.rel)
	}
	if err := s.commit(gitrepo.Commit{
		Paths:    paths,
		Subject:  "hedgehug: init",
		Trailers: []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: "init"}},
		Author:   author,
		When:     s.now().Truncate(time.Second),
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func ensureEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dir, 0o750)
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty", dir)
	}
	return nil
}

// Open opens the store in dir. It reads store.yaml and schema.yaml now;
// Meta and Schema return what it read.
func Open(dir string) (*Store, error) {
	meta, err := readDecode(dir, model.StoreMetaPath, model.DecodeStoreMeta)
	if err != nil {
		return nil, err
	}
	sc, err := readDecode(dir, schemaPath, schema.Decode)
	if err != nil {
		return nil, err
	}
	repo, err := gitrepo.Open(dir)
	if err != nil {
		return nil, err
	}
	return newStore(dir, repo, meta, sc), nil
}

func (s *Store) Meta() *model.StoreMeta { return s.meta }
func (s *Store) Schema() *schema.Schema  { return s.schema }

// path turns rel, slash-separated and relative to the store root, into a
// path on disk.
func (s *Store) path(rel string) string { return filepath.Join(s.dir, filepath.FromSlash(rel)) }
```

- [ ] **Step 5: `read.go` を実装する**

```go
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"path"
	"path/filepath"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
)

// readDecode reads the file rel under dir and decodes it. A missing file is
// ErrNotFound; other errors carry rel.
func readDecode[T any](dir, rel string, decode func([]byte) (T, error)) (T, error) {
	var zero T
	b, err := readFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if errors.Is(err, fs.ErrNotExist) {
		return zero, fmt.Errorf("%s: %w", rel, ErrNotFound)
	}
	if err != nil {
		return zero, err
	}
	v, err := decode(b)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", rel, err)
	}
	return v, nil
}

// walk decodes every file under root that match accepts. It yields decode
// errors and goes on; a missing root yields nothing. The order is not fixed.
func walk[T any](s *Store, root string, match func(rel string) bool, decode func(rel string, b []byte) (T, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		err := filepath.WalkDir(s.path(root), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(s.dir, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !match(rel) {
				return nil
			}
			v, err := readDecode(s.dir, rel, func(b []byte) (T, error) { return decode(rel, b) })
			if !yield(v, err) {
				return fs.SkipAll
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			yield(zero, err)
		}
	}
}

// flatMatch accepts rel when it is the file build gives for the ID in its
// name, like sprints/<ULID>.md.
func flatMatch(build func(model.ID) string) func(string) bool {
	return func(rel string) bool {
		id, err := model.ParseID(strings.TrimSuffix(path.Base(rel), ".md"))
		return err == nil && build(id) == rel
	}
}

func ignoreRel[T any](decode func([]byte) (T, error)) func(string, []byte) (T, error) {
	return func(_ string, b []byte) (T, error) { return decode(b) }
}

func (s *Store) Task(id model.ID) (*model.Task, error) {
	return readDecode(s.dir, model.TaskPath(id), model.DecodeTask)
}

func (s *Store) Tasks() iter.Seq2[*model.Task, error] {
	match := func(rel string) bool {
		_, err := model.ParseTaskPath(rel)
		return err == nil
	}
	return walk(s, "tasks", match, ignoreRel(model.DecodeTask))
}

// Comments returns the comments on task.
func (s *Store) Comments(task model.ID) iter.Seq2[*model.Comment, error] {
	match := func(rel string) bool {
		t, _, err := model.ParseCommentPath(rel)
		return err == nil && t == task
	}
	return walk(s, path.Dir(model.CommentPath(task, model.ID{})), match, ignoreRel(model.DecodeComment))
}

// Worklogs returns the worklog entries on task.
func (s *Store) Worklogs(task model.ID) iter.Seq2[*model.Worklog, error] {
	match := func(rel string) bool {
		t, _, err := model.ParseWorklogPath(rel)
		return err == nil && t == task
	}
	return walk(s, path.Dir(model.WorklogPath(task, model.ID{})), match, ignoreRel(model.DecodeWorklog))
}

func (s *Store) Sprint(id model.ID) (*model.Sprint, error) {
	return readDecode(s.dir, model.SprintPath(id), model.DecodeSprint)
}

func (s *Store) Sprints() iter.Seq2[*model.Sprint, error] {
	return walk(s, "sprints", flatMatch(model.SprintPath), ignoreRel(model.DecodeSprint))
}

func (s *Store) Release(id model.ID) (*model.Release, error) {
	return readDecode(s.dir, model.ReleasePath(id), model.DecodeRelease)
}

func (s *Store) Releases() iter.Seq2[*model.Release, error] {
	return walk(s, "releases", flatMatch(model.ReleasePath), ignoreRel(model.DecodeRelease))
}

func (s *Store) Template(name string) (*model.Template, error) {
	if err := model.ValidTemplateName(name); err != nil {
		return nil, err
	}
	return readDecode(s.dir, model.TemplatePath(name), func(b []byte) (*model.Template, error) {
		return model.DecodeTemplate(name, b)
	})
}

func (s *Store) Templates() iter.Seq2[*model.Template, error] {
	match := func(rel string) bool {
		name, ok := templateName(rel)
		return ok && model.ValidTemplateName(name) == nil
	}
	decode := func(rel string, b []byte) (*model.Template, error) {
		name, _ := templateName(rel)
		return model.DecodeTemplate(name, b)
	}
	return walk(s, "templates", match, decode)
}

func templateName(rel string) (string, bool) {
	if path.Dir(rel) != "templates" || path.Ext(rel) != ".md" {
		return "", false
	}
	return strings.TrimSuffix(path.Base(rel), ".md"), true
}
```

`TestOpenNotAStore` が通ることを確かめる（`store.yaml` がないと `readDecode` が `ErrNotFound` を返す）。

- [ ] **Step 6: テストが通ることを確かめる**

Run: `go test -race ./internal/store/`
Expected: PASS

- [ ] **Step 7: `make check` を通す**

Run: `make check`
Expected: 成功

- [ ] **Step 8: コミット**

```bash
git add internal/store
git commit -m "feat: Store の作成・オープンとファイルの読み出しを追加する"
```

---

### Task 5: schema.ValidateWrite

**Files:**
- Modify: `internal/schema/validate.go`（末尾に追加）
- Test: `internal/schema/validate_test.go`（末尾に追加）

**Interfaces:**
- Consumes: 既存の `ValidateTask`、`ValidatePlacement`、`ValidateTransition`、`Violation`
- Produces: `func ValidateWrite(old, cur *model.Task, parentKind string, e *Effective) []Violation`

- [ ] **Step 1: 失敗するテストを書く**

`internal/schema/validate_test.go` の末尾に追加する（既存の `taskWith` と `codes` を使う）:

```go
func TestValidateWrite(t *testing.T) {
	s := Default()
	s.Workflow = Workflow{
		Transitions: Lists{{"backlog", []string{"todo"}}},
		Required:    Lists{{"in_progress", []string{"assignees"}}},
	}
	e := s.For(nil)

	for _, tc := range []struct {
		name       string
		old        func(*model.Task) // nil: a new task
		cur        func(*model.Task)
		parentKind string
		want       []string
	}{
		{"new and valid", nil, func(*model.Task) {}, "", nil},
		{"new with a bad value", nil, func(t *model.Task) {
			t.Props.Set("priority", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "none"})
		}, "", []string{"bad_value:priority"}},
		{"new needs required fields", nil, func(t *model.Task) { t.Status = "in_progress" }, "", []string{"required:assignees"}},
		{"new in a wrong place", nil, func(t *model.Task) { t.Kind = "bug" }, "task", []string{"placement:parent"}},
		{"unknown status is reported once", nil, func(t *model.Task) { t.Status = "nope" }, "", []string{"unknown_status:status"}},
		{"edit brings a violation", func(*model.Task) {}, func(t *model.Task) { t.Title = "" }, "", []string{"missing_title:title"}},
		{"old violation is left out", func(t *model.Task) { t.Kind = "nope" }, func(t *model.Task) {
			t.Kind = "nope"
			t.Title = "renamed"
		}, "", nil},
		{"transition not allowed", func(t *model.Task) { t.Status = "backlog" }, func(t *model.Task) { t.Status = "done" }, "", []string{"transition:status"}},
		{"same status skips the workflow", func(t *model.Task) { t.Status = "in_progress" }, func(t *model.Task) {
			t.Status = "in_progress"
			t.Title = "renamed"
		}, "", nil},
	} {
		var old *model.Task
		if tc.old != nil {
			old = taskWith(t, "")
			tc.old(old)
		}
		cur := taskWith(t, "")
		tc.cur(cur)
		got := codes(ValidateWrite(old, cur, tc.parentKind, e))
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: violations = %v, want %v", tc.name, got, tc.want)
		}
	}
}
```

import に `"slices"` を足す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/schema/ -run ValidateWrite`
Expected: FAIL（`undefined: ValidateWrite`）

- [ ] **Step 3: 実装する**

`internal/schema/validate.go` の末尾に追加する:

```go
// ValidateWrite checks a write of cur over old, which is nil for a new task.
// parentKind is the kind of cur's parent, empty at the root. It returns only
// the violations the write brings in: ones old already had are left out, so
// a task that a sync left broken can still be edited.
func ValidateWrite(old, cur *model.Task, parentKind string, e *Effective) []Violation {
	vs := taskViolations(cur, parentKind, e)
	switch {
	case old == nil:
		vs = append(vs, ValidateTransition("", cur.Status, cur, e)...)
	case old.Status != cur.Status:
		vs = append(vs, ValidateTransition(old.Status, cur.Status, cur, e)...)
	}
	var had []Violation
	if old != nil {
		had = taskViolations(old, parentKind, e)
	}
	return newViolations(vs, had)
}

func taskViolations(t *model.Task, parentKind string, e *Effective) []Violation {
	vs, _ := ValidateTask(t, e)
	return append(vs, ValidatePlacement(t.Kind, parentKind, e)...)
}

// newViolations returns vs without the ones in had, and without repeats:
// two checks may report the same unknown kind or status. Violations are the
// same when their Code and Field are.
func newViolations(vs, had []Violation) []Violation {
	seen := make(map[[2]string]bool, len(had))
	for _, v := range had {
		seen[[2]string{v.Code, v.Field}] = true
	}
	var out []Violation
	for _, v := range vs {
		k := [2]string{v.Code, v.Field}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/schema/`
Expected: PASS

- [ ] **Step 5: `make check` を通す**

Run: `make check`
Expected: 成功

- [ ] **Step 6: コミット**

```bash
git add internal/schema
git commit -m "feat: 書き込みで新しく生じた違反だけを返す ValidateWrite を追加する"
```

---

### Task 6: Update と Tx（タスクの書き込み、日時、コミット、巻き戻し）

**Files:**
- Create: `internal/store/tx.go`
- Test: `internal/store/tx_test.go`

**Interfaces:**
- Consumes: Task 2〜4 のすべて（`writeFile`、`readFile`、`acquire`、`Store` の非公開フィールド、`s.path`、`s.Task`）
- Produces:
  - `type Op struct { Name string; Task model.ID; Author gitrepo.Signature }`
  - `func (s *Store) Update(op Op, fn func(*Tx) error) error`
  - `type Tx struct`（非公開フィールド: `s *Store`、`done bool`、`order []string`、`pending map[string]*pending`、`tasks map[model.ID]*model.Task`、`taskOrder []model.ID`、`schema *schema.Schema`）
  - `func (tx *Tx) Task(id model.ID) (*model.Task, error)`、`func (tx *Tx) task(id model.ID) (*model.Task, error)`（`done` を見ない内部用）
  - `func (tx *Tx) PutTask(t *model.Task) error`
  - `func (tx *Tx) put(rel string, p *pending) error`
  - `type pending struct { encode func() ([]byte, error); created, updated *time.Time; remove bool }`
  - `var errTxDone error`
  - Task 7 が `PutComment` などとスキーマの `Check()` を、Task 8 が `tx.validate` の呼び出しを `Update` に足す。Task 6 では、検査を入れる場所に印のコメントを置かない。

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/tx_test.go`:

```go
package store

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

func newTask(title string) *model.Task {
	return &model.Task{ID: model.NewID(), Kind: "task", Title: title, Status: "todo"}
}

func addTask(t *testing.T, s *Store, task *model.Task) {
	t.Helper()
	if err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(task)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCommitsOnce(t *testing.T) {
	s := newTestStore(t)
	task := newTask("リリース準備")
	addTask(t, s, task)

	got, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "リリース準備" || !got.Created.Equal(testNow) || !got.Updated.Equal(testNow) {
		t.Errorf("task = %+v", got)
	}
	log := headLog(t, s.dir)
	if len(log) != 2 {
		t.Fatalf("commits = %d, want 2 (init + add)", len(log))
	}
	want := "hedgehug: add " + task.ID.String() + " リリース準備\n\nHedgehug-Op: add\nHedgehug-Task: " + task.ID.String() + "\n"
	if log[0].Message != want {
		t.Errorf("message = %q, want %q", log[0].Message, want)
	}
	if !log[0].Author.When.Equal(testNow) {
		t.Errorf("commit time = %v", log[0].Author.When)
	}
}

func TestUpdateSubjectUsesKey(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	task.Key = "WEB-42"
	addTask(t, s, task)
	if msg := headLog(t, s.dir)[0].Message; !strings.HasPrefix(msg, "hedgehug: add WEB-42 x\n") {
		t.Errorf("message = %q", msg)
	}
}

func TestUpdateWithoutTask(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	if err := s.Update(Op{Name: "bulk", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
		t.Fatal(err)
	}
	if msg := headLog(t, s.dir)[0].Message; msg != "hedgehug: bulk\n\nHedgehug-Op: bulk\n" {
		t.Errorf("message = %q", msg)
	}
}

func TestUpdateNoChange(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)

	later := testNow.Add(time.Hour)
	s.now = func() time.Time { return later }
	same, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(Op{Name: "update", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(same)
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2: an unchanged write must not commit", n)
	}
	got, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Updated.Equal(testNow) {
		t.Errorf("updated = %v, want it unchanged", got.Updated)
	}

	got.Title = "y"
	if err := s.Update(Op{Name: "update", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTask(got)
	}); err != nil {
		t.Fatal(err)
	}
	again, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Updated.Equal(later) || !again.Created.Equal(testNow) {
		t.Errorf("created, updated = %v, %v; want %v, %v", again.Created, again.Updated, testNow, later)
	}
}

func TestUpdateFnError(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	boom := errors.New("boom")
	err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(task); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(task.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("task file exists after a failed fn: %v", err)
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}

func TestUpdateCommitFailureRollsBack(t *testing.T) {
	s := newTestStore(t)
	kept := newTask("kept")
	addTask(t, s, kept)
	before, err := os.ReadFile(s.path(model.TaskPath(kept.ID)))
	if err != nil {
		t.Fatal(err)
	}

	boom := errors.New("commit failed")
	s.commit = func(gitrepo.Commit) error { return boom }
	edited, err := s.Task(kept.ID)
	if err != nil {
		t.Fatal(err)
	}
	edited.Title = "changed"
	added := newTask("added")
	err = s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(edited); err != nil {
			return err
		}
		return tx.PutTask(added)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the commit error", err)
	}
	after, err := os.ReadFile(s.path(model.TaskPath(kept.ID)))
	if err != nil || string(after) != string(before) {
		t.Errorf("edited file was not restored: %q, %v", after, err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(added.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("new file was not removed: %v", err)
	}
}

func TestTxReadsItsWrites(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	if err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		if err := tx.PutTask(task); err != nil {
			return err
		}
		got, err := tx.Task(task.ID)
		if err != nil {
			return err
		}
		if got.Title != "x" {
			t.Errorf("tx.Task = %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTxUnusableAfterUpdate(t *testing.T) {
	s := newTestStore(t)
	var saved *Tx
	if err := s.Update(Op{Name: "noop", Author: testAuthor}, func(tx *Tx) error {
		saved = tx
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := saved.PutTask(newTask("late")); !errors.Is(err, errTxDone) {
		t.Errorf("err = %v, want errTxDone", err)
	}
}

func TestUpdateLockedElsewhere(t *testing.T) {
	s := newTestStore(t)
	s.lockWait = 100 * time.Millisecond
	l, err := acquire(s.path(lockPath), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.release() }()
	err = s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(newTask("x")) })
	if !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked", err)
	}
}

func TestUpdateFromTwoStores(t *testing.T) {
	s := newTestStore(t)
	other, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	const perStore = 5
	var wg sync.WaitGroup
	errs := make(chan error, 2*perStore)
	for _, st := range []*Store{s, other} {
		wg.Go(func() {
			for range perStore {
				task := newTask("x")
				errs <- st.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
					return tx.PutTask(task)
				})
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := len(headLog(t, s.dir)); n != 1+2*perStore {
		t.Errorf("commits = %d, want %d", n, 1+2*perStore)
	}
	n := 0
	for _, err := range s.Tasks() {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 2*perStore {
		t.Errorf("tasks = %d, want %d", n, 2*perStore)
	}
}

func TestUpdateNeedsOpName(t *testing.T) {
	s := newTestStore(t)
	if err := s.Update(Op{Author: testAuthor}, func(*Tx) error { return nil }); err == nil {
		t.Error("Update without Op.Name succeeded")
	}
}
```

`sync.WaitGroup.Go` は Go 1.25 から使える。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/store/ -run 'Update|Tx'`
Expected: FAIL（`undefined: Op` など）

- [ ] **Step 3: 実装する**

`internal/store/tx.go`:

```go
package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

var errTxDone = errors.New("the transaction is used after Update returned")

// Op describes one operation, which becomes one commit.
type Op struct {
	Name   string   // add, update, rm, mv...; the Hedgehug-Op trailer
	Task   model.ID // the main task of the operation; zero if none
	Author gitrepo.Signature
}

// Tx collects the writes of one operation. Its Put methods only queue a
// write; Update writes them all after fn returns.
type Tx struct {
	s    *Store
	done bool

	order   []string // paths in the order first put
	pending map[string]*pending

	tasks     map[model.ID]*model.Task // put tasks; nil for a removed one
	taskOrder []model.ID
	schema    *schema.Schema // set by PutSchema
}

// pending is one queued write. created and updated point into the value
// being written, so Update can stamp it; they are nil for files without
// timestamps.
type pending struct {
	encode           func() ([]byte, error)
	created, updated *time.Time
	remove           bool
}

// change is a write that alters a file, with what was there before.
type change struct {
	rel     string
	data    []byte // nil when removing
	old     []byte
	existed bool
}

func (tx *Tx) put(rel string, p *pending) error {
	if tx.done {
		return errTxDone
	}
	if _, ok := tx.pending[rel]; !ok {
		tx.order = append(tx.order, rel)
	}
	tx.pending[rel] = p
	return nil
}

// Task reads a task as it will be after the writes queued so far.
func (tx *Tx) Task(id model.ID) (*model.Task, error) {
	if tx.done {
		return nil, errTxDone
	}
	return tx.task(id)
}

func (tx *Tx) task(id model.ID) (*model.Task, error) {
	if t, ok := tx.tasks[id]; ok {
		if t == nil {
			return nil, fmt.Errorf("%s: %w", model.TaskPath(id), ErrNotFound)
		}
		return t, nil
	}
	return tx.s.Task(id)
}

// PutTask queues a write of t. Update fills in created and updated.
func (tx *Tx) PutTask(t *model.Task) error {
	if t.ID.IsZero() {
		return errors.New("the task has no id")
	}
	if err := tx.put(model.TaskPath(t.ID), &pending{encode: t.Encode, created: &t.Created, updated: &t.Updated}); err != nil {
		return err
	}
	if _, ok := tx.tasks[t.ID]; !ok {
		tx.taskOrder = append(tx.taskOrder, t.ID)
	}
	tx.tasks[t.ID] = t
	return nil
}

// Update runs one operation: under the store lock it runs fn, stamps and
// writes the files fn put, and commits them as one commit. When fn fails
// nothing is written; when writing or committing fails the files are put
// back as they were. An operation that changes nothing makes no commit.
func (s *Store) Update(op Op, fn func(*Tx) error) (err error) {
	if op.Name == "" {
		return errors.New("the operation has no name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := acquire(s.path(lockPath), s.lockWait)
	if err != nil {
		return err
	}
	defer func() {
		if rerr := l.release(); err == nil {
			err = rerr
		}
	}()

	tx := &Tx{s: s, pending: map[string]*pending{}, tasks: map[model.ID]*model.Task{}}
	err = fn(tx)
	tx.done = true
	if err != nil {
		return err
	}

	now := s.now().Truncate(time.Second)
	changes, err := tx.changes(now)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	commit := tx.commitFor(op, changes, now)
	if err := s.apply(changes); err != nil {
		return err
	}
	if err := s.commit(commit); err != nil {
		return errors.Join(err, s.rollback(changes))
	}
	if tx.schema != nil {
		s.schema = tx.schema
	}
	return nil
}

// changes stamps the queued writes and returns those that alter a file.
// updated is left as it was while comparing, so a write of the same content
// is dropped and keeps its old updated.
func (tx *Tx) changes(now time.Time) ([]change, error) {
	var out []change
	for _, rel := range tx.order {
		p := tx.pending[rel]
		old, err := readFile(tx.s.path(rel))
		existed := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if p.remove {
			if existed {
				out = append(out, change{rel: rel, old: old, existed: true})
			}
			continue
		}
		if p.created != nil && p.created.IsZero() {
			*p.created = now
		}
		data, err := p.encode()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if existed && bytes.Equal(data, old) {
			continue
		}
		if p.updated != nil {
			*p.updated = now
			if data, err = p.encode(); err != nil {
				return nil, fmt.Errorf("%s: %w", rel, err)
			}
		}
		out = append(out, change{rel: rel, data: data, old: old, existed: existed})
	}
	return out, nil
}

// commitFor builds the commit of op. The subject names the task by its key,
// or by its ID when it has none, and gives its title as op leaves it.
func (tx *Tx) commitFor(op Op, cs []change, now time.Time) gitrepo.Commit {
	subject := "hedgehug: " + op.Name
	trailers := []gitrepo.Trailer{{Key: "Hedgehug-Op", Value: op.Name}}
	if !op.Task.IsZero() {
		label, title := op.Task.String(), ""
		if t, err := tx.task(op.Task); err == nil {
			if t.Key != "" {
				label = t.Key
			}
			title = t.Title
		}
		subject += " " + label
		if title != "" {
			subject += " " + title
		}
		trailers = append(trailers, gitrepo.Trailer{Key: "Hedgehug-Task", Value: op.Task.String()})
	}
	paths := make([]string, len(cs))
	for i, c := range cs {
		paths[i] = c.rel
	}
	return gitrepo.Commit{Paths: paths, Subject: subject, Trailers: trailers, Author: op.Author, When: now}
}

// apply writes cs in order. When one fails, the ones before it are rolled
// back.
func (s *Store) apply(cs []change) error {
	for i, c := range cs {
		var err error
		if c.data == nil {
			err = os.Remove(s.path(c.rel))
		} else {
			err = writeFile(s.path(c.rel), c.data)
		}
		if err != nil {
			return errors.Join(err, s.rollback(cs[:i]))
		}
	}
	return nil
}

// rollback puts the files of cs back as they were, last first.
func (s *Store) rollback(cs []change) error {
	var errs []error
	for _, c := range slices.Backward(cs) {
		if c.existed {
			errs = append(errs, writeFile(s.path(c.rel), c.old))
		} else if err := os.Remove(s.path(c.rel)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
```

注意: `change.data` が nil かどうかで削除を見分けている。Encode が空のバイト列を返すことはない（frontmatter の区切りが必ず入る）ので、書き込みの `data` が nil になることはない。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/store/`
Expected: PASS

- [ ] **Step 5: `make check` を通す**

Run: `make check`
Expected: 成功。`commitFor` の未使用の引数を unparam が指摘したら、その引数を消す。

- [ ] **Step 6: コミット**

```bash
git add internal/store
git commit -m "feat: 1操作を1コミットにする Store.Update を追加する"
```

---

### Task 7: Tx の残りの書き込み（コメント・作業時間・スプリント・リリース・テンプレート・スキーマ・削除）

**Files:**
- Modify: `internal/store/tx.go`
- Create: `internal/store/validate.go`（`SchemaError` だけ。Task 8 で同じファイルに足す）
- Test: `internal/store/tx_test.go`（末尾に追加）

**Interfaces:**
- Consumes: Task 6 の `Tx`、`pending`、`tx.put`、`Update`
- Produces:
  - `func (tx *Tx) PutComment(task model.ID, c *model.Comment) error`
  - `func (tx *Tx) PutWorklog(task model.ID, w *model.Worklog) error`
  - `func (tx *Tx) PutSprint(sp *model.Sprint) error`
  - `func (tx *Tx) PutRelease(r *model.Release) error`
  - `func (tx *Tx) PutTemplate(t *model.Template) error`
  - `func (tx *Tx) PutSchema(sc *schema.Schema) error`
  - `func (tx *Tx) RemoveTask(id model.ID) error`
  - `type SchemaError struct{ Problems []schema.Problem }`

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/tx_test.go` の末尾に追加する（import に `"github.com/kukv/hedgehug/internal/schema"` を足す）:

```go
func TestPutEveryFileType(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	c := &model.Comment{ID: model.NewID(), Author: "github.com/me", Body: "コメント\n"}
	w := &model.Worklog{ID: model.NewID(), Author: "github.com/me", Body: "作業\n"}
	sp := &model.Sprint{ID: model.NewID(), Name: "S1", State: "planned"}
	r := &model.Release{ID: model.NewID(), Name: "v1", State: "unreleased"}
	tp := &model.Template{Name: "bug"}
	tp.Item.Kind = "bug"

	if err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(
			tx.PutTask(task),
			tx.PutComment(task.ID, c),
			tx.PutWorklog(task.ID, w),
			tx.PutSprint(sp),
			tx.PutRelease(r),
			tx.PutTemplate(tp),
		)
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(headLog(t, s.dir)); n != 2 {
		t.Errorf("commits = %d, want 2", n)
	}

	n := 0
	for got, err := range s.Comments(task.ID) {
		if err != nil || got.Body != "コメント\n" || !got.Created.Equal(testNow) {
			t.Errorf("comment = %+v, %v", got, err)
		}
		n++
	}
	if n != 1 {
		t.Errorf("comments = %d", n)
	}
	for got, err := range s.Worklogs(task.ID) {
		if err != nil || !got.Updated.Equal(testNow) {
			t.Errorf("worklog = %+v, %v", got, err)
		}
	}
	if got, err := s.Sprint(sp.ID); err != nil || got.Name != "S1" || !got.Created.Equal(testNow) {
		t.Errorf("sprint = %+v, %v", got, err)
	}
	if got, err := s.Release(r.ID); err != nil || got.Name != "v1" {
		t.Errorf("release = %+v, %v", got, err)
	}
	if got, err := s.Template("bug"); err != nil || !got.Updated.Equal(testNow) {
		t.Errorf("template = %+v, %v", got, err)
	}
}

func TestPutTemplateBadName(t *testing.T) {
	s := newTestStore(t)
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return tx.PutTemplate(&model.Template{Name: "../x"})
	})
	if err == nil {
		t.Error("PutTemplate accepted a name outside templates/")
	}
}

func TestRemoveTask(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	c := &model.Comment{ID: model.NewID(), Author: "github.com/me"}
	w := &model.Worklog{ID: model.NewID(), Author: "github.com/me"}
	if err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(task), tx.PutComment(task.ID, c), tx.PutWorklog(task.ID, w))
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(Op{Name: "purge", Task: task.ID, Author: testAuthor}, func(tx *Tx) error {
		return tx.RemoveTask(task.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(task.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("task still readable: %v", err)
	}
	for range s.Comments(task.ID) {
		t.Error("a comment is left")
	}
	log := headLog(t, s.dir)
	if len(log) != 3 {
		t.Fatalf("commits = %d, want 3", len(log))
	}
	if want := "hedgehug: purge " + task.ID.String() + "\n"; !strings.HasPrefix(log[0].Message, want) {
		t.Errorf("message = %q, want prefix %q", log[0].Message, want)
	}
	tree, err := log[0].Tree()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{model.TaskPath(task.ID), model.CommentPath(task.ID, c.ID), model.WorklogPath(task.ID, w.ID)} {
		if _, err := tree.File(p); err == nil {
			t.Errorf("%s is still in the tree", p)
		}
	}
}

func TestPutSchema(t *testing.T) {
	s := newTestStore(t)
	sc := schema.Dev()
	if err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Schema().Status("review"); !ok {
		t.Error("Schema() was not replaced")
	}
	o, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := o.Schema().Status("review"); !ok {
		t.Error("schema.yaml was not written")
	}

	bad := schema.Default()
	bad.Statuses = append(bad.Statuses, bad.Statuses[0]) // a duplicate name
	err = s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(bad) })
	var se *SchemaError
	if !errors.As(err, &se) || len(se.Problems) == 0 {
		t.Errorf("err = %v, want a *SchemaError", err)
	}
}
```

`schema.Schema` のステータスの配列は `Statuses`（確認済み）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/store/ -run 'Put|Remove'`
Expected: FAIL（`tx.PutComment undefined` など）

- [ ] **Step 3: `tx.go` に書き込みを足す**

`PutTask` の後ろに追加する（import に `"path"` を足す）:

```go
func (tx *Tx) PutComment(task model.ID, c *model.Comment) error {
	return tx.put(model.CommentPath(task, c.ID), &pending{encode: c.Encode, created: &c.Created, updated: &c.Updated})
}

func (tx *Tx) PutWorklog(task model.ID, w *model.Worklog) error {
	return tx.put(model.WorklogPath(task, w.ID), &pending{encode: w.Encode, created: &w.Created, updated: &w.Updated})
}

func (tx *Tx) PutSprint(sp *model.Sprint) error {
	return tx.put(model.SprintPath(sp.ID), &pending{encode: sp.Encode, created: &sp.Created, updated: &sp.Updated})
}

func (tx *Tx) PutRelease(r *model.Release) error {
	return tx.put(model.ReleasePath(r.ID), &pending{encode: r.Encode, created: &r.Created, updated: &r.Updated})
}

func (tx *Tx) PutTemplate(t *model.Template) error {
	if err := model.ValidTemplateName(t.Name); err != nil {
		return err
	}
	return tx.put(model.TemplatePath(t.Name), &pending{encode: t.Encode, created: &t.Created, updated: &t.Updated})
}

// PutSchema queues a write of schema.yaml. Update refuses a schema that
// fails its own Check, and checks the tasks of the same operation against
// the new schema.
func (tx *Tx) PutSchema(sc *schema.Schema) error {
	if err := tx.put(schemaPath, &pending{encode: sc.Encode}); err != nil {
		return err
	}
	tx.schema = sc
	return nil
}

// RemoveTask queues the removal of a task file with its comments and
// worklog entries, for purging. Soft deletion is a PutTask with DeletedAt.
func (tx *Tx) RemoveTask(id model.ID) error {
	if tx.done {
		return errTxDone
	}
	rels := []string{model.TaskPath(id)}
	for _, dir := range []string{path.Dir(model.CommentPath(id, model.ID{})), path.Dir(model.WorklogPath(id, model.ID{}))} {
		entries, err := os.ReadDir(tx.s.path(dir))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				rels = append(rels, dir+"/"+e.Name())
			}
		}
	}
	for _, rel := range rels {
		if err := tx.put(rel, &pending{remove: true}); err != nil {
			return err
		}
	}
	tx.tasks[id] = nil
	return nil
}
```

`RemoveTask` は `tx.taskOrder` には足さない（Task 8 の検査は、`tx.tasks` の値が nil のものを飛ばす）。

- [ ] **Step 4: `SchemaError` と、Update でのスキーマの検査を足す**

`internal/store/validate.go`:

```go
package store

import (
	"strings"

	"github.com/kukv/hedgehug/internal/schema"
)

// SchemaError means a schema written with PutSchema has problems.
type SchemaError struct{ Problems []schema.Problem }

func (e *SchemaError) Error() string {
	msgs := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		msgs[i] = p.String()
	}
	return "invalid schema: " + strings.Join(msgs, "; ")
}
```

`tx.go` の `Update` で、`tx.done = true` のエラー処理の直後（`now := ...` の前）に足す:

```go
	if tx.schema != nil {
		if ps := tx.schema.Check(); len(ps) > 0 {
			return &SchemaError{Problems: ps}
		}
	}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/store/`
Expected: PASS

- [ ] **Step 6: `make check` を通す**

Run: `make check`
Expected: 成功

- [ ] **Step 7: コミット**

```bash
git add internal/store
git commit -m "feat: コメント・作業時間・計画・テンプレート・スキーマの書き込みとタスクの削除を追加する"
```

---

### Task 8: 書き込み時のスキーマ検査

**Files:**
- Modify: `internal/store/validate.go`
- Modify: `internal/store/tx.go`（`Update` に検査の呼び出しを1か所足す）
- Test: `internal/store/validate_test.go`

**Interfaces:**
- Consumes: `schema.ValidateWrite`（Task 5）、`(*schema.Schema).For`、`(*schema.Schema).Kind`、`schema.RoleContainer`、`tx.task`・`tx.tasks`・`tx.taskOrder`・`tx.schema`（Task 6・7）
- Produces:
  - `type ValidationError struct { Task model.ID; Violations []schema.Violation }`
  - `var ErrCycle error`
  - `func (tx *Tx) validate(sc *schema.Schema) error`
  - `func (tx *Tx) lineage(t *model.Task, sc *schema.Schema) (parentKind string, ancestors []model.ID, err error)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/store/validate_test.go`:

```go
package store

import (
	"errors"
	"os"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestUpdateRefusesViolation(t *testing.T) {
	s := newTestStore(t)
	task := newTask("")
	err := s.Update(Op{Name: "add", Task: task.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) })
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != task.ID || len(ve.Violations) != 1 || ve.Violations[0].Code != schema.CodeMissingTitle {
		t.Fatalf("err = %v, want a ValidationError for the empty title", err)
	}
	if _, err := os.Stat(s.path(model.TaskPath(task.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("refused task was written: %v", err)
	}
	if n := len(headLog(t, s.dir)); n != 1 {
		t.Errorf("commits = %d, want 1", n)
	}
}

func TestUpdateAllowsOldViolation(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	addTask(t, s, task)
	// A sync may bring a kind this schema does not know.
	mustWrite(t, s, model.TaskPath(task.ID), "---\nid: "+task.ID.String()+"\nkind: nope\ntitle: x\nstatus: todo\n---\n")

	got, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := s.Update(Op{Name: "update", Task: task.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(got) }); err != nil {
		t.Errorf("editing a task with an old violation failed: %v", err)
	}
}

func TestUpdatePlacement(t *testing.T) {
	s := newTestStore(t)
	parent := newTask("parent")
	child := newTask("child")
	child.Kind = "bug" // a bug cannot sit under a task
	child.Parent = &parent.ID
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(parent), tx.PutTask(child))
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != child.ID || ve.Violations[0].Code != schema.CodePlacement {
		t.Errorf("err = %v, want a placement violation on the child", err)
	}
}

func TestUpdateParentAndChildTogether(t *testing.T) {
	s := newTestStore(t)
	parent := newTask("parent")
	parent.Kind = "epic"
	child := newTask("child")
	child.Parent = &parent.ID
	if err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		// The child is put first: the check runs after fn, when both are queued.
		return errors.Join(tx.PutTask(child), tx.PutTask(parent))
	}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMissingParent(t *testing.T) {
	s := newTestStore(t)
	child := newTask("child")
	missing := model.NewID()
	child.Parent = &missing
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(child) })
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateParentCycle(t *testing.T) {
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	a.Parent, b.Parent = &b.ID, &a.ID
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.PutTask(a), tx.PutTask(b))
	})
	if !errors.Is(err, ErrCycle) {
		t.Errorf("err = %v, want ErrCycle", err)
	}
}

func TestUpdateUsesProjectSchema(t *testing.T) {
	s := newTestStore(t)
	project := newTask("project")
	project.Kind = "project"
	addTask(t, s, project)

	sc := schema.Default()
	sc.Scoped = append(sc.Scoped, schema.Scoped{
		Where:    "project:" + project.ID.String(),
		Workflow: &schema.Workflow{Required: schema.Lists{{"in_progress", []string{"assignees"}}}},
	})
	if err := s.Update(Op{Name: "schema", Author: testAuthor}, func(tx *Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}

	inside := newTask("inside")
	inside.Parent = &project.ID
	inside.Status = "in_progress"
	err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(inside) })
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Violations[0].Code != schema.CodeRequired {
		t.Errorf("inside the project: err = %v, want a required violation", err)
	}

	outside := newTask("outside")
	outside.Status = "in_progress"
	if err := s.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(outside) }); err != nil {
		t.Errorf("outside the project: %v", err)
	}
}
```

`schema.Scoped.Workflow` はポインタ（`*Workflow`）。`Statuses` を省略した `Scoped` は `Check()` を通る（確認済み）。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/store/ -run 'Refuses|Allows|Placement|Parent|Project'`
Expected: FAIL（`undefined: ValidationError` など）

- [ ] **Step 3: 実装する**

`internal/store/validate.go` に追加する（import に `"errors"`、`"fmt"`、`"github.com/kukv/hedgehug/internal/model"` を足す）:

```go
// ErrCycle means following a task's parents led back to a task already seen.
var ErrCycle = errors.New("the parents of the task form a cycle")

// ValidationError means a write would bring violations into a task.
type ValidationError struct {
	Task       model.ID
	Violations []schema.Violation
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		msgs[i] = v.String()
	}
	return fmt.Sprintf("task %s: %s", e.Task, strings.Join(msgs, "; "))
}

// validate checks every task put in the operation against sc. The rules are
// schema.ValidateWrite's; this only gathers what it needs: the task as it is
// on disk, the kind of its parent and its ancestors.
func (tx *Tx) validate(sc *schema.Schema) error {
	for _, id := range tx.taskOrder {
		t := tx.tasks[id]
		if t == nil {
			continue // removed in the same operation
		}
		old, err := tx.s.Task(id)
		if errors.Is(err, ErrNotFound) {
			old = nil
		} else if err != nil {
			return err
		}
		parentKind, ancestors, err := tx.lineage(t, sc)
		if err != nil {
			return err
		}
		if vs := schema.ValidateWrite(old, t, parentKind, sc.For(ancestors)); len(vs) > 0 {
			return &ValidationError{Task: id, Violations: vs}
		}
	}
	return nil
}

// lineage returns the kind of t's parent, empty at the root, and the IDs
// For needs: t's ancestors nearest first, led by t itself when t is a
// container. Parents are read as the operation leaves them.
func (tx *Tx) lineage(t *model.Task, sc *schema.Schema) (parentKind string, ancestors []model.ID, err error) {
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
		p, err := tx.task(id)
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

`tx.go` の `Update` で、Task 7 で足したスキーマの検査の直後に足す:

```go
	sc := s.schema
	if tx.schema != nil {
		sc = tx.schema
	}
	if err := tx.validate(sc); err != nil {
		return err
	}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/store/`
Expected: PASS（Task 6・7 のテストも通る。`newTask` は既定のスキーマで正しいタスクを作る）

- [ ] **Step 5: `make check` を通す**

Run: `make check`
Expected: 成功

- [ ] **Step 6: ほかの OS でコンパイルできることを確かめる**

Run: `GOOS=windows go vet ./... && GOOS=darwin go vet ./...`
Expected: 成功

- [ ] **Step 7: コミット**

```bash
git add internal/store
git commit -m "feat: Store の書き込みでタスクをスキーマで検査する"
```

---

## 完了の確認

- [ ] `make check` が通る
- [ ] spec の「完了条件」を1つずつ確かめる
  - 全ファイル型を `Update` で書いて読み戻せる（`TestPutEveryFileType`）
  - 1回の `Update` がちょうど1コミットで、件名と trailer が規則どおり（`TestUpdateCommitsOnce` ほか）
  - 失敗と違反で中途半端な状態が残らない（`TestUpdateFnError`、`TestUpdateCommitFailureRollsBack`、`TestUpdateRefusesViolation`）
- [ ] PR を作り、CI の ubuntu / macos / windows がすべて通ることを確かめる
