# 構成見直し 回 2: OS の境界 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** OS に触る呼び出し（改名・削除・読み書き・ディレクトリ作成・ロック・プロセス起動・シグナル・パスの比較）を `internal/lib/osfs` に集め、forbidigo で osfs 以外からの呼び出しを禁止する。atomicfile は osfs に統合する。github を `github.Options` で受け取る形にし、github から app への依存をなくす。挙動は変えない。

**Architecture:** osfs は `os`・`os/exec`・`syscall` の薄い包み。関数の形（引数と戻り値）は `os` のものと同じにするので、呼ぶ側は `os.X(` を `osfs.X(` に置き換えるだけで済む。OS ごとに違いうるもの（ファイル操作・flock・セッションの分離・シグナル）は `_unix.go` に置く。Windows 版は書かない。

**Tech Stack:** Go、perl（一括置換）、golangci-lint 2.15.0（forbidigo、`golangci-lint fmt`）。

**Spec:** `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`（3 節・6 節の PR 2）。概要 plan: `docs/superpowers/plans/2026-11-04-codebase-restructure.md`

## Global Constraints

- 概要 plan の Global Constraints をすべて守る
- この回では、spec 3 節と github の Options 化のほかに何も変えない。関数の分割や依存の整理は回 3 以降
- 呼び方は今と同じにする（関数の置き換えだけ）。osfs の関数の形は `os` の同名の関数と同じ。エラーの文言も変えない
- テストの中身の変更は、import・型と関数の名前・呼び出し先のパッケージの書き換えだけにする。osfs に移すテスト（atomicfile のテストと `store.WithFileLock` のテスト）は中身を変えずに移す
- `docs/superpowers/specs/` と `docs/superpowers/plans/` の既存の文書に出てくる旧名（atomicfile など）は書き換えない。例外は Task 3 で直す spec 6 節の 1 行だけ

## 調べてわかっていること（main 6d6ce10）

- 禁止する呼び出し（`os.Rename`・`os.Remove`・`os.RemoveAll`・`os.ReadFile`・`os.WriteFile`・`os.OpenFile`・`os.MkdirAll`・`exec.Command`・`syscall.*`・`unix.*`）は、テストとテスト用のパッケージを除いて 31 ファイル 106 行ある。下の forbidigo の設定で空実行し、grep で数えた 106 行と 1 行も違わないことを確かめた（`cli/ui.go:135` の `syscall.SIGTERM` のように呼び出しでない式も検出される）
- 106 行の内訳: lib/gitrepo 13、storage（store 12・config 4・view 1）17、tools 11、remote（replicate 34・auth 1）35、usecase（直下 7・current 1・maintenance 6）14、cli 15、server/openapigen 1
- osfs に置き換えた行では gosec が黙る（`os.*` を直接呼ばないため）。その行の `//nolint:gosec ...` と `// #nosec ...` は不要になるので消す。osfs の中で gosec が指すのは `os.ReadFile`（G304）・`os.OpenFile`（G304）・`exec.Command`（G204）で、そこに nolint を置く（2026-11-04 に view・usecase・tools の 3 か所で空実行して確かめた）
- `store.ErrLocked` は server・cli・usecase・replicate・auth で `errors.Is` と同一性で比べられ、文言は auth のエラーに `%w` で出る。osfs に `ErrLocked` を同じ文言（`the store is locked by another process`）で置き、store は `var ErrLocked = osfs.ErrLocked` にする。比べる側も文言も変わらない
- store のテストは `lockPoll`（`testmain_test.go:13` で 5ms にする。`tx.go:529` の待ちにも使う）と `afterLockMiss`（`lock_gone_test.go:51` で差し替える）を使う。待ちのループは osfs の `LockFile` に移し、store は `lockPoll` と `afterLockMiss` を引数で渡す。これで store のテストは `.release()` → `.Release()` の名前の書き換えだけで済む
- `store.WithFileLock` を使うのは auth（`device.go:75`）と config（`config.go:547`）とそのテスト、usecase のテスト（`auth_test.go:123`）。osfs に移すと、auth と config は store を import しなくなる
- github は config から `config.Host`（`New` の引数）と `config.PlainHTTP`（平文 HTTP の拒否）だけを使う。2026-11-04 利用者が決定: `github.Options` は `API` と `Web` の 2 つだけにし、ループバックの判定（`IsLoopback`・`PlainHTTP`）は github に移す。config の `IsLoopback`・`PlainHTTP` は github を呼ぶ 1 行で残す（`usecase/conn.go:119`・`config/hosts.go:173`・config のテストが使う）。デバイスフローの 3 関数（`RequestDeviceCode`・`PollToken`・`RefreshToken`）は web の URL を文字列で受け取るまま変えない（テストの 13 か所が URL を直接渡しているため）
- `config.Host` を `github.Options` に変えるテストは、github の 7 ファイルと replicate の 3 ファイル（`github.New(config.Host{API: srv.URL}, ...)` が 8 か所）
- `syscall` の禁止に `golang.org/x/sys/unix`（`unix.*`）も含める。flock を osfs に移すと、unix を使うのは osfs だけになる
- パスの比較: Store のディレクトリを `==`・`!=` で比べているのは `usecase/readonly.go:54`・`usecase/session.go:285`・`server/poll.go:130`・`server/poll.go:235` の 4 か所。`osfs.SamePath` に置き換える（unix では今と同じ `==`）。`discover.go` の `filepath.Clean` の map のキーと `mount.go:161` の親ディレクトリのまとめは比較ではないので触らない
- `os.CreateTemp`・`os.Mkdir`・`os.Stat`・`os.Chmod`・`exec.LookPath` などは spec 3 節の禁止の一覧にないので触らない
- lib/gitrepo は go-billy の `osfs`（`github.com/go-git/go-billy/v5/osfs`）を `transport.go` などで import しているが、置き換える 7 ファイルはどれも billy の osfs を import していないので、名前はぶつからない
- CI の shard は `others` が残りのパッケージをすべて拾う（`.github/workflows/ci-backend.yaml:157`）ので、`lib/osfs` のために変えるものはない
- `golangci-lint fmt`（goimports）は使わなくなった import を消す。この plan では import を消す手順を書かず、fmt に任せる
- 2026-11-04 に、この plan の Task 1〜5 と Task 6 Step 1 を空実行し、`go build`・`go vet`・`make lint`・`make fmt-check`・`go test -race ./...` が通ることを確かめた（確かめた後は元に戻した）。空実行で出た直しは plan に入れてある（Task 1 の grep の期待とコメントの名前、Task 2 の `lock_gone_test.go` の `.release()`、import の足し方）

## scratchpad に置くスクリプト

ファイル `<scratchpad>/add-import.sh`（リポジトリには入れない。Task 2・3・5 で使う）:

```bash
#!/bin/bash
# 引数のファイルに import を足す（なければ）。使わなくなった import は golangci-lint fmt が消す。
# 使い方: add-import.sh <import パス> <ファイル>...
set -euo pipefail
p=$1
shift
for f in "$@"; do
  grep -qF "\"$p\"" "$f" || perl -0 -pi -e "s{^import \(\n}{import (\n\t\"$p\"\n}m" "$f"
done
```

## 置き換えの対応

| 旧 | 新 |
| --- | --- |
| `atomicfile.WriteFile(path, data)` | `osfs.WriteAtomic(path, data)` |
| `atomicfile.WriteFileInDir(path, data)` | `osfs.WriteAtomicInDir(path, data)` |
| `atomicfile.Rename`・`atomicfile.ReadFile` | `osfs.Rename`・`osfs.ReadFile` |
| `os.Rename`・`os.Remove`・`os.RemoveAll`・`os.ReadFile`・`os.WriteFile`・`os.OpenFile`・`os.MkdirAll` | `osfs.` の同名 |
| `exec.Command` | `osfs.Command` |
| `replicate.spawnDetached` の中の `exec.Command` と `syscall.SysProcAttr` | `osfs.StartDetached` |
| `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` | `signal.NotifyContext(ctx, osfs.StopSignals()...)` |
| store の `fileLock`・`release()`・`sameFile` | `osfs.Lock`・`Release()`・`Holds` |
| `store.WithFileLock` | `osfs.WithFileLock` |
| Store のディレクトリの `a == b`・`a != b` | `osfs.SamePath(a, b)`・`!osfs.SamePath(a, b)` |
| `github.New(config.Host, ...)` | `github.New(github.Options, ...)`（呼ぶ側は `h.GitHub()`） |

---

### Task 1: osfs を作り、atomicfile を統合する

**Files:**
- Move: `internal/lib/atomicfile/` → `internal/lib/osfs/`（`atomicfile.go` → `osfs.go`、`atomicfile_unix.go` → `files_unix.go`、`atomicfile_test.go` → `atomic_test.go`、`testmain_test.go` はそのまま）
- Create: `internal/lib/osfs/proc.go`、`internal/lib/osfs/proc_unix.go`、`internal/lib/osfs/files_test.go`、`internal/lib/osfs/proc_test.go`
- Modify: atomicfile を import する 13 ファイル（`cli/ui.go`、`storage/store/` の ids・local・local_test・merge・read・store・tx・validate、`usecase/current/state.go`、`lib/gitrepo/` の maintain・merge・refs）

**Interfaces:**
- Produces（後の Task が使う）:
  - `osfs.WriteAtomic(path string, data []byte) error`、`osfs.WriteAtomicInDir(path string, data []byte) error`
  - `osfs.Rename(oldpath, newpath string) error`、`osfs.Remove(path string) error`、`osfs.RemoveAll(path string) error`、`osfs.ReadFile(path string) ([]byte, error)`、`osfs.WriteFile(path string, data []byte, perm os.FileMode) error`、`osfs.OpenFile(path string, flag int, perm os.FileMode) (*os.File, error)`、`osfs.MkdirAll(path string, perm os.FileMode) error`
  - `osfs.SamePath(a, b string) bool`
  - `osfs.Command(name string, args ...string) *exec.Cmd`、`osfs.StartDetached(name string, args []string, stderr *os.File) error`、`osfs.StopSignals() []os.Signal`

- [ ] **Step 1: ディレクトリを移し、パッケージ名と関数名を書き換える**

リポジトリのルートで:

```bash
git mv internal/lib/atomicfile internal/lib/osfs
git mv internal/lib/osfs/atomicfile.go internal/lib/osfs/osfs.go
git mv internal/lib/osfs/atomicfile_unix.go internal/lib/osfs/files_unix.go
git mv internal/lib/osfs/atomicfile_test.go internal/lib/osfs/atomic_test.go
perl -pi -e 's/^package atomicfile$/package osfs/' internal/lib/osfs/*.go
# パッケージの中の名前（WriteFileInDir を先に書き換える）
perl -pi -e 's/\bWriteFileInDir\b/WriteAtomicInDir/g; s/\bWriteFile\(/WriteAtomic(/g' internal/lib/osfs/osfs.go internal/lib/osfs/atomic_test.go
# 使う側
mapfile -t f < <(grep -rl 'internal/lib/atomicfile"' --include='*.go' internal tools cmd)
perl -pi -e 's{internal/lib/atomicfile"}{internal/lib/osfs"}; s/\batomicfile\.WriteFileInDir\b/osfs.WriteAtomicInDir/g; s/\batomicfile\.WriteFile\b/osfs.WriteAtomic/g; s/\batomicfile\.(Rename|ReadFile)\b/osfs.$1/g' "${f[@]}"
grep -rn 'atomicfile' --include='*.go' internal tools cmd
```

Expected: 最後の grep は `internal/lib/osfs/osfs.go:1` の package コメントだけを出す（Step 2 で直す）。

- [ ] **Step 2: osfs.go の package コメントと関数のコメントを直す**

`internal/lib/osfs/osfs.go` の先頭のコメントを次に置き換える:

```go
// Package osfs is where Hedgehug touches the OS: it renames, removes, reads
// and writes files, makes directories, locks files, starts processes and
// compares paths (spec: OS の境界). The rest of the code calls these instead
// of os, os/exec and syscall, which forbidigo enforces, so that what differs
// between OSes stays in the _unix.go files here.
package osfs
```

同じファイルの関数のコメントの名前を合わせる（Step 1 の perl で `WriteFileInDir` は `WriteAtomicInDir` になっている）: `// WriteFile replaces the file at path with data.` → `// WriteAtomic replaces the file at path with data.`、`// WriteAtomicInDir is WriteFile for a directory that exists:` → `// WriteAtomicInDir is WriteAtomic for a directory that exists:`。関数の中身は変えない。

- [ ] **Step 3: files_unix.go を osfs のファイル操作の全体にする**

`internal/lib/osfs/files_unix.go` を次の内容にする（`Rename` と `ReadFile` は atomicfile のものと同じ）:

```go
//go:build unix

package osfs

import "os"

// Rename renames oldpath to newpath, replacing it.
func Rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

// Remove removes the file or the empty directory at path.
func Remove(path string) error { return os.Remove(path) }

// RemoveAll removes path and everything it holds. A missing path is no error.
func RemoveAll(path string) error { return os.RemoveAll(path) }

// ReadFile returns the content of the file at path.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // G304: callers build paths from a store or repository root
}

// WriteFile writes data to the file at path, made with perm when missing.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// OpenFile opens the file at path with flag, made with perm when flag has
// os.O_CREATE.
func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm) //nolint:gosec // G304: callers build paths from a store or repository root
}

// MkdirAll makes the directory path and the parents it lacks, with perm.
func MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

// SamePath reports whether a and b name the same file. On unix that is the
// same string, as the callers have always compared them.
func SamePath(a, b string) bool { return a == b }
```

- [ ] **Step 4: プロセスとシグナルの関数を書く**

`internal/lib/osfs/proc.go`:

```go
package osfs

import "os/exec"

// Command is exec.Command: name run with args, which the caller sets up and
// runs.
func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...) //nolint:gosec // G204: callers choose the program and its arguments
}
```

`internal/lib/osfs/proc_unix.go`（中身は `internal/app/remote/replicate/spawn_unix.go` の起動の部分と同じ）:

```go
//go:build unix

package osfs

import (
	"os"
	"os/exec"
	"syscall"
)

// StartDetached starts name with args in a session of its own, with no input
// and no output but stderr, and lets it run on after this process ends.
func StartDetached(name string, args []string, stderr *os.File) error {
	cmd := exec.Command(name, args...) //nolint:gosec // G204: callers choose the program and its arguments
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// StopSignals are the signals that ask a process to end: Ctrl-C and SIGTERM.
func StopSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }
```

- [ ] **Step 5: 新しい関数のテストを書く**

`internal/lib/osfs/files_test.go`:

```go
package osfs

import "testing"

func TestSamePath(t *testing.T) {
	t.Parallel()
	if !SamePath("/a/b", "/a/b") {
		t.Error(`SamePath("/a/b", "/a/b") = false`)
	}
	if SamePath("/a/b", "/a/c") {
		t.Error(`SamePath("/a/b", "/a/c") = true`)
	}
}
```

`internal/lib/osfs/proc_test.go`:

```go
package osfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartDetached(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "log")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := StartDetached("sh", []string{"-c", "echo detached >&2"}, f); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), "detached") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the log has %q, want the child's stderr", b)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
```

- [ ] **Step 6: 整形し、ビルドとテストを通す**

```bash
golangci-lint fmt ./...
go build ./... && go vet ./...
go test ./internal/lib/osfs/ ./internal/app/storage/store/ ./internal/lib/gitrepo/ ./internal/app/usecase/current/
golangci-lint run ./internal/lib/osfs/ ./internal/app/storage/store/ ./internal/lib/gitrepo/ ./internal/app/usecase/current/ ./internal/app/cli/
```

Expected: すべて通り、lint は `0 issues.`。

- [ ] **Step 7: コミット**

```bash
git add -A internal
git commit -m "refactor: atomicfile を lib/osfs に統合し、OS に触る薄い関数を置く"
```

---

### Task 2: ファイルのロックを osfs に移す

**Files:**
- Create: `internal/lib/osfs/lock.go`、`internal/lib/osfs/lock_test.go`
- Move: `internal/app/storage/store/lock_unix.go` → `internal/lib/osfs/lock_unix.go`
- Modify: `internal/app/storage/store/lock.go`、`tx.go:435,516`、`move.go:156,205`、`local.go:103`、`merge.go:33`
- Modify（テスト）: `internal/app/storage/store/lock_test.go`（`TestWithFileLock` を消し、`.release()` を `.Release()` に）、`tx_test.go:248`、`move_test.go:590,608`
- Modify: `internal/app/remote/auth/device.go:75-76`、`internal/app/storage/config/config.go:547`、`internal/app/remote/auth/minor_test.go:107,113`、`internal/app/remote/auth/device_test.go:344`、`internal/app/usecase/auth_test.go:123`

**Interfaces:**
- Consumes: Task 1 の `osfs.MkdirAll`
- Produces:
  - `var osfs.ErrLocked error`（文言 `the store is locked by another process`）
  - `type osfs.Lock`、`osfs.LockFile(path string, wait, poll time.Duration, onMiss func()) (*osfs.Lock, error)`、`(*osfs.Lock).Holds(path string) bool`、`(*osfs.Lock).Release() error`
  - `osfs.WithFileLock(path string, wait time.Duration, fn func() error) error`
  - `store.ErrLocked` は `osfs.ErrLocked` と同じ値のまま残る

- [ ] **Step 1: osfs のロックのテストを書く**

`internal/lib/osfs/lock_test.go`（`TestWithFileLock` は `internal/app/storage/store/lock_test.go:87-108` から中身を変えずに移す）:

```go
package osfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithFileLock(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "sub", "auth.lock")
	inside := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = WithFileLock(p, time.Second, func() error {
			close(inside)
			<-release
			return nil
		})
	}()
	<-inside
	if err := WithFileLock(p, 20*time.Millisecond, func() error { return nil }); !errors.Is(err, ErrLocked) {
		t.Errorf("second lock = %v, want ErrLocked", err)
	}
	close(release)
	want := errors.New("from fn")
	if err := WithFileLock(p, time.Second, func() error { return want }); !errors.Is(err, want) {
		t.Errorf("err = %v, want fn's", err)
	}
}

func TestLockFileCallsOnMiss(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "lock")
	l, err := LockFile(p, time.Second, time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	misses := 0
	if _, err := LockFile(p, 20*time.Millisecond, 5*time.Millisecond, func() { misses++ }); !errors.Is(err, ErrLocked) {
		t.Fatalf("second LockFile: err = %v, want ErrLocked", err)
	}
	if misses == 0 {
		t.Error("onMiss was not called while the lock was taken")
	}
}

func TestLockHolds(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "lock")
	l, err := LockFile(p, time.Second, time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if !l.Holds(p) {
		t.Error("Holds = false for the file that was locked")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if l.Holds(p) {
		t.Error("Holds = true after the file was removed")
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/lib/osfs/ -run 'Lock'`
Expected: FAIL（`undefined: WithFileLock`・`LockFile`・`ErrLocked`）

- [ ] **Step 3: osfs のロックを書く**

```bash
git mv internal/app/storage/store/lock_unix.go internal/lib/osfs/lock_unix.go
perl -pi -e 's/^package store$/package osfs/' internal/lib/osfs/lock_unix.go
```

`lock_unix.go` の中身（`tryLock`・`unlock`）は変えない。

`internal/lib/osfs/lock.go`（ループは `internal/app/storage/store/lock.go` の `lockFile` と同じ。`poll` と `onMiss` を引数にした）:

```go
package osfs

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ErrLocked means another process kept a lock for longer than it was waited
// for. The text is the one the store has always shown: the store's own
// ErrLocked is this error.
var ErrLocked = errors.New("the store is locked by another process")

// defaultPoll is how often WithFileLock tries a taken lock again.
const defaultPoll = 50 * time.Millisecond

// Lock is an exclusive OS lock on a file. The lock belongs to the file
// handle, so two handles in one process exclude each other, and the OS drops
// it when the process exits.
type Lock struct{ f *os.File }

// LockFile takes the lock on the file at path, made when missing, in a
// directory that exists. While another holder has it, it tries again every
// poll for up to wait, calling onMiss (when not nil) after each try, and then
// fails with ErrLocked.
func LockFile(path string, wait, poll time.Duration, onMiss func()) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // G304: callers build paths from a store or a data directory
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
			return &Lock{f}, nil
		}
		if onMiss != nil {
			onMiss()
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, ErrLocked
		}
		time.Sleep(min(poll, time.Until(deadline)))
	}
}

// Holds reports whether the locked file is still the file at path: false
// once that file was removed or replaced.
func (l *Lock) Holds(path string) bool {
	a, err := l.f.Stat()
	if err != nil {
		return false
	}
	b, err := os.Stat(path)
	return err == nil && os.SameFile(a, b)
}

// Release drops the lock and closes its file.
func (l *Lock) Release() error {
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}

// WithFileLock runs fn holding the OS lock on the file path, made with its
// directory when missing, waiting up to wait for another holder. It is for
// locks outside a store, such as the one that keeps two processes from
// refreshing a token at once.
func WithFileLock(path string, wait time.Duration, fn func() error) (err error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	l, err := LockFile(path, wait, defaultPoll, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rerr := l.Release(); err == nil {
			err = rerr
		}
	}()
	return fn()
}
```

- [ ] **Step 4: store のロックを osfs で書き直す**

`internal/app/storage/store/lock.go` の先頭から `WithFileLock` の終わりまで（今の 1〜77 行）を次に置き換える。`ErrStoreGone` から下は、`acquireStore` の 3 か所（下のコメント）と、`sameFile`・`release` を消すことのほかは変えない。

```go
package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// ErrLocked means another process kept the store locked for longer than a
// write waits. It is osfs.ErrLocked, which osfs.WithFileLock also returns.
var ErrLocked = osfs.ErrLocked

// lockWait is how long a write waits for the lock, trying every lockPoll.
const lockWait = 10 * time.Second

var lockPoll = 50 * time.Millisecond

// afterLockMiss is called each time acquire finds the lock taken, with the
// lock file open; tests use it to know a waiter is blocked.
var afterLockMiss = func() {}

func acquire(path string, wait time.Duration) (*osfs.Lock, error) {
	path = filepath.Clean(path)
	if err := osfs.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return lockFile(path, wait)
}

// lockFile takes the lock on the file at path, made when missing, in a
// directory that exists.
func lockFile(path string, wait time.Duration) (*osfs.Lock, error) {
	return osfs.LockFile(path, wait, lockPoll, func() { afterLockMiss() })
}
```

`acquireStore` の中を次のとおり直す:
- 戻り値の型 `(*fileLock, error)` → `(*osfs.Lock, error)`
- `if !storeHere(dir) || !sameFile(l.f, path) {` → `if !storeHere(dir) || !l.Holds(path) {`
- その中の `_ = l.release()` → `_ = l.Release()`

ファイルの末尾の `func sameFile(...)` と `func (l *fileLock) release() error` を消す。

store のほかのファイル:

```bash
perl -pi -e 's/\*fileLock\b/*osfs.Lock/g; s/\.release\(\)/.Release()/g' internal/app/storage/store/*.go
bash <scratchpad>/add-import.sh github.com/kukv/hedgehug/internal/lib/osfs internal/app/storage/store/move.go
```

（`.release()` は store の中ではロックにしか使っていない。テストでは lock_test・lock_gone_test・tx_test・move_test が使う。`tx.go` などは Task 1 で osfs を import している。）

`internal/app/storage/store/lock_test.go` から `func TestWithFileLock` を丸ごと消す（Step 1 で osfs に移した）。

- [ ] **Step 5: auth と config を osfs.WithFileLock にする**

```bash
perl -pi -e 's/\bstore\.WithFileLock\(/osfs.WithFileLock(/g' internal/app/remote/auth/device.go internal/app/storage/config/config.go internal/app/remote/auth/minor_test.go internal/app/remote/auth/device_test.go internal/app/usecase/auth_test.go
perl -pi -e 's/\bstore\.ErrLocked\b/osfs.ErrLocked/g' internal/app/remote/auth/device.go internal/app/remote/auth/minor_test.go internal/app/remote/auth/device_test.go
```

```bash
bash <scratchpad>/add-import.sh github.com/kukv/hedgehug/internal/lib/osfs internal/app/remote/auth/device.go internal/app/storage/config/config.go internal/app/remote/auth/minor_test.go internal/app/remote/auth/device_test.go internal/app/usecase/auth_test.go
```

使わなくなった `internal/app/storage/store` の import は、Step 6 の `golangci-lint fmt`（goimports）が消す（`usecase/auth_test.go` はほかでも store を使うので残る）。

- [ ] **Step 6: 整形し、ビルドとテストを通す**

```bash
golangci-lint fmt ./...
go build ./... && go vet ./...
go test ./internal/lib/osfs/ ./internal/app/storage/store/ ./internal/app/storage/config/ ./internal/app/remote/auth/
go list -f '{{join .Imports "\n"}}' ./internal/app/remote/auth ./internal/app/storage/config | grep -c 'internal/app/storage/store'
```

Expected: テストが通る。最後の行は `0`（auth と config が store を import しない。spec 3 節）。

- [ ] **Step 7: コミット**

```bash
git add -A internal
git commit -m "refactor: ファイルのロックと WithFileLock を osfs に移す"
```

---

### Task 3: github を github.Options で受け取る形にする

**Files:**
- Create: `internal/lib/github/plain.go`
- Modify: `internal/lib/github/client.go:18,92-93,108`、`internal/lib/github/device.go:11,34`
- Modify: `internal/app/storage/config/hosts.go:124-141`（`IsLoopback`・`PlainHTTP` を github に委ね、`Host.GitHub` を足す）
- Modify: `internal/app/usecase/auth.go:98,342`、`internal/app/usecase/conn.go:149`
- Modify（テスト）: `internal/lib/github/` の client_test・edges_test・insecure_test・neterr_test・origin_test・safety_test・timeout_test、`internal/app/remote/replicate/` の github_test・github_forget_test・explicit_recheck_test
- Modify: `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md:196`

**Interfaces:**
- Produces:
  - `type github.Options struct{ API, Web string }`、`github.New(o github.Options, token string, hc *http.Client) *github.Client`
  - `github.IsLoopback(host string) bool`、`github.PlainHTTP(s string) bool`
  - `(config.Host).GitHub() github.Options`
  - `config.IsLoopback`・`config.PlainHTTP` は今と同じ形で残る

- [ ] **Step 1: ループバックの判定を github に移す**

`internal/lib/github/plain.go`（中身は `internal/app/storage/config/hosts.go:124-141` と同じ）:

```go
package github

import (
	"net"
	"net/url"
	"strings"
)

// IsLoopback reports whether host (a name or an IP, with or without the
// brackets of an IPv6 literal) is the local machine: 127.0.0.0/8, ::1 or
// localhost. A token is sent over plain http only there.
func IsLoopback(host string) bool {
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// PlainHTTP reports whether s is an http URL (not https) whose host is not
// loopback.
func PlainHTTP(s string) bool {
	u, err := url.Parse(s)
	return err == nil && strings.EqualFold(u.Scheme, "http") && !IsLoopback(u.Hostname())
}
```

`client.go:108` と `device.go:34` の `config.PlainHTTP(` を `PlainHTTP(` にする。

- [ ] **Step 2: Options を足し、New をそれで受け取る**

`internal/lib/github/client.go` の `New`（92 行目のあたり）を次に置き換える:

```go
// Options is what a client needs of a GitHub host: the base of its REST API,
// and the base of its web pages, where the device flow runs.
type Options struct {
	API string // the REST API base, without a trailing slash
	Web string // the web base, for the device flow
}

// New makes a client of the host o with token. hc nil is a client with a
// 30-second timeout.
func New(o Options, token string, hc *http.Client) *Client {
	return &Client{api: o.API, token: token, hc: defaultHTTP(hc)}
}
```

`client.go` と `device.go` の `internal/app/storage/config` の import は、Step 5 の `golangci-lint fmt` が消す。

- [ ] **Step 3: config を github に委ねる**

`internal/app/storage/config/hosts.go` の `IsLoopback` と `PlainHTTP`（124〜141 行）を次に置き換え、`Host` の型（16〜21 行）のすぐ下に `GitHub` を足す:

```go
// IsLoopback reports whether host (a name or an IP, with or without the
// brackets of an IPv6 literal) is the local machine: 127.0.0.0/8, ::1 or
// localhost. A token is sent over plain http only there.
func IsLoopback(host string) bool { return github.IsLoopback(host) }

// PlainHTTP reports whether s is an http URL (not https) whose host is not
// loopback.
func PlainHTTP(s string) bool { return github.PlainHTTP(s) }
```

```go
// GitHub is what the GitHub client needs of the host.
func (h Host) GitHub() github.Options { return github.Options{API: h.API, Web: h.Web} }
```

```bash
bash <scratchpad>/add-import.sh github.com/kukv/hedgehug/internal/lib/github internal/app/storage/config/hosts.go
```

使わなくなった `net` は Step 5 の `golangci-lint fmt` が消す。

- [ ] **Step 4: 呼ぶ側とテストを書き換える**

```bash
# usecase の 3 か所
perl -pi -e 's/\bgithub\.New\(h, /github.New(h.GitHub(), /' internal/app/usecase/auth.go
perl -pi -e 's/\bgithub\.New\(c\.Host\(acct\.Host\), /github.New(c.Host(acct.Host).GitHub(), /' internal/app/usecase/auth.go
perl -pi -e 's/\bgithub\.New\(c\.Host\(host\), /github.New(c.Host(host).GitHub(), /' internal/app/usecase/conn.go
# replicate のテスト
perl -pi -e 's/\bgithub\.New\(config\.Host\{/github.New(github.Options{/g' internal/app/remote/replicate/github_test.go internal/app/remote/replicate/github_forget_test.go internal/app/remote/replicate/explicit_recheck_test.go
# github のテスト
cd internal/lib/github
perl -pi -e 's/\bconfig\.Host\{Host: "github\.com", /Options{/g; s/\bconfig\.Host\b/Options/g' client_test.go edges_test.go insecure_test.go neterr_test.go origin_test.go safety_test.go timeout_test.go
cd -
grep -rn 'config\.' internal/lib/github/
```

Expected: 最後の grep は、コメントの `config.yaml` だけを出す（`config.Host`・`config.PlainHTTP` は残らない）。`usecase/auth.go:98` の `h` が `config.Host` でなければ、その行の perl が当たらない。その場合は `github.New(` に渡している `config.Host` の値の後ろに `.GitHub()` を付ける。

使わなくなった `internal/app/storage/config` の import は、github の 7 ファイルと replicate の 3 ファイルから Step 5 の `golangci-lint fmt` が消す。

- [ ] **Step 5: 整形し、ビルドとテストを通す**

```bash
golangci-lint fmt ./...
go build ./... && go vet ./...
go test ./internal/lib/github/ ./internal/app/storage/config/ ./internal/app/remote/replicate/ -run 'GitHub|Insecure|PlainHTTP|Loopback|Host|Device|Token|Timeout|Origin|Safety|Edge|Net|Forget|Recheck'
go test ./internal/app/usecase/ -run 'Conn|Auth'
go list -deps ./internal/lib/github | grep -c 'internal/app/'
```

Expected: テストが通る。最後の行は `0`（github が app に依存しない）。

- [ ] **Step 6: spec の 1 行を決定に合わせる**

`docs/superpowers/specs/2026-11-04-codebase-restructure-design.md:196` の

```
| 2. OS の境界 | 3 節。github を `github.Options`（ホストと平文 HTTP の可否）で受け取る形にする。forbidigo を有効にする | 変わらない |
```

を次にする:

```
| 2. OS の境界 | 3 節。github を `github.Options`（API と web の URL）で受け取る形にし、平文 HTTP の判定（IsLoopback・PlainHTTP）を config から github に移す（2026-11-04 利用者が決定）。forbidigo を有効にする | 変わらない |
```

- [ ] **Step 7: コミット**

```bash
git add -A internal/lib/github internal/app/storage/config internal/app/usecase internal/app/remote/replicate docs/superpowers/specs/2026-11-04-codebase-restructure-design.md
git commit -m "refactor: github を github.Options で受け取り、平文 HTTP の判定を github に移す"
```

---

### Task 4: forbidigo を有効にし、lib・storage・tools の呼び出しを osfs に置き換える

**Files:**
- Modify: `.golangci.yaml`
- Modify: `internal/lib/gitrepo/` の conn・lock・maintain・merge・mergehead・refs・remote、`internal/app/storage/store/` の local・store・tx、`internal/app/storage/config/config.go`、`internal/app/storage/view/list.go`、`tools/devenv/main.go`、`tools/licenses/main.go`

**Interfaces:**
- Consumes: Task 1 の osfs の関数
- Produces: forbidigo の設定。Task 5 はこの設定で lint を通す

- [ ] **Step 1: 置き換えのスクリプトを scratchpad に置く**

ファイル `<scratchpad>/to-osfs.sh`（リポジトリには入れない）:

```bash
#!/bin/bash
# 引数のファイルの os.* と exec.Command を osfs に置き換え、その行の gosec の
# nolint を消し、osfs の import を足す。リポジトリのルートで実行する。
set -euo pipefail
M=github.com/kukv/hedgehug/internal/lib/osfs
for f in "$@"; do
  perl -pi -e 'my $n = s/\bos\.(Rename|Remove|RemoveAll|ReadFile|WriteFile|OpenFile|MkdirAll)\(/osfs.$1(/g; $n += s/\bexec\.Command\(/osfs.Command(/g; s{\s*//\s*(?:nolint:gosec|#nosec)\b.*$}{} if $n' "$f"
  grep -qF "\"$M\"" "$f" || perl -0 -pi -e "s{^import \(\n}{import (\n\t\"$M\"\n}m" "$f"
done
golangci-lint fmt "$@"
```

- [ ] **Step 2: forbidigo を有効にする**

`.golangci.yaml` の `linters.enable` の `unparam` の下に `- forbidigo` を足し、`linters` の下（`exclusions` の前）に `settings` を、`exclusions.rules` の末尾に 2 つのルールを足す:

```yaml
    - unparam
    - forbidigo
    # depguard は層を分けるパッケージができたときに、そのルールと一緒に有効にする。
    # ルール無しで有効にすると、標準ライブラリ以外の import をすべて拒否する。
  settings:
    forbidigo:
      analyze-types: true
      forbid:
        # OS に触る呼び出しは internal/lib/osfs を通す（spec: OS の境界）。
        - pattern: ^os\.(Rename|Remove|RemoveAll|ReadFile|WriteFile|OpenFile|MkdirAll)$
          msg: use the function of the same name in internal/lib/osfs
        - pattern: ^exec\.Command$
          msg: use osfs.Command
        - pattern: ^(syscall|unix)\.
          msg: keep syscall and golang.org/x/sys/unix in internal/lib/osfs
  exclusions:
    rules:
      - linters:
          - revive
        text: "^(exported|unused-parameter):"
      - path: _test\.go
        linters:
          - gosec
      # osfs と、テストのファイルとテスト用のパッケージは OS を直接呼んでよい。
      - path: (_test\.go|internal/lib/(osfs|testenv|fakegh)/|internal/lib/gitrepo/gitrepotest/|internal/app/storage/history/historytest/)
        linters:
          - forbidigo
```

Run: `golangci-lint run ./... 2>&1 | grep -c '(forbidigo)'`
Expected: `101`（106 行のうち、Task 2 で store/lock.go・lock_unix.go の 5 行がなくなった）

- [ ] **Step 3: lib・storage・tools を置き換える**

```bash
bash <scratchpad>/to-osfs.sh \
  internal/lib/gitrepo/conn.go internal/lib/gitrepo/lock.go internal/lib/gitrepo/maintain.go \
  internal/lib/gitrepo/merge.go internal/lib/gitrepo/mergehead.go internal/lib/gitrepo/refs.go \
  internal/lib/gitrepo/remote.go \
  internal/app/storage/store/local.go internal/app/storage/store/store.go internal/app/storage/store/tx.go \
  internal/app/storage/config/config.go internal/app/storage/view/list.go \
  tools/devenv/main.go tools/licenses/main.go
git diff --stat
```

Expected: 14 ファイルが変わる。`git diff` で、変わった行が `os.X(` → `osfs.X(` と、その行の末尾の nolint の削除と、import だけであることを見る。

- [ ] **Step 4: ビルドと lint を確かめる**

```bash
go build ./... && go vet ./...
golangci-lint run ./... 2>&1 | grep '(forbidigo)' | grep -vcE '^internal/app/(remote|usecase|cli|server)/'
golangci-lint run ./... 2>&1 | grep -c '(forbidigo)'
golangci-lint run ./... 2>&1 | grep -v '(forbidigo)' | grep -E '\.go:[0-9]+:[0-9]+:'
```

Expected: 2 行目は `0`（残りは Task 5 の層だけ）。3 行目は `65`。4 行目は何も出さない（forbidigo のほかの指摘がない）。

- [ ] **Step 5: 置き換えたパッケージのテストを通す**

Run: `go test ./internal/lib/gitrepo/... ./internal/app/storage/... ./tools/...`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add .golangci.yaml internal/lib/gitrepo internal/app/storage tools
git commit -m "refactor: forbidigo で OS の直接の呼び出しを禁止し、lib・storage・tools を osfs にする"
```

---

### Task 5: remote・usecase・cli・server の呼び出しを osfs に置き換える

**Files:**
- Move: `internal/app/remote/replicate/spawn_unix.go` → `internal/app/remote/replicate/spawn.go`
- Modify: `internal/app/remote/replicate/` の background・rebuild、`internal/app/remote/auth/source.go`、`internal/app/usecase/` の maintain・storeops・views・readonly・session、`internal/app/usecase/current/state.go`、`internal/app/usecase/maintenance/doctor.go`、`internal/app/cli/` の comment・edit・editor・merge_driver・ui、`internal/app/server/poll.go`、`internal/app/server/openapigen/main.go`

**Interfaces:**
- Consumes: Task 1 の `osfs.StartDetached`・`osfs.StopSignals`・`osfs.SamePath`・osfs のファイル操作、Task 4 の to-osfs.sh と forbidigo の設定

- [ ] **Step 1: 機械的に置き換える**

```bash
bash <scratchpad>/to-osfs.sh \
  internal/app/remote/replicate/background.go internal/app/remote/replicate/rebuild.go \
  internal/app/remote/auth/source.go \
  internal/app/usecase/maintain.go internal/app/usecase/storeops.go internal/app/usecase/views.go \
  internal/app/usecase/current/state.go internal/app/usecase/maintenance/doctor.go \
  internal/app/cli/comment.go internal/app/cli/edit.go internal/app/cli/editor.go \
  internal/app/cli/merge_driver.go internal/app/cli/ui.go \
  internal/app/server/openapigen/main.go
```

- [ ] **Step 2: spawn を osfs.StartDetached にする**

```bash
git mv internal/app/remote/replicate/spawn_unix.go internal/app/remote/replicate/spawn.go
```

`internal/app/remote/replicate/spawn.go` を次の内容にする（`//go:build !windows` は消す。OS の差は osfs に移った）:

```go
package replicate

import "github.com/kukv/hedgehug/internal/lib/osfs"

// spawnDetached starts name with args in a session of its own, with no
// input and no output but its stderr, which goes to the log.
func spawnDetached(name string, args []string, logPath string) error {
	log, err := openLog(logPath)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }() // the child has its own copy
	return osfs.StartDetached(name, args, log)
}
```

- [ ] **Step 3: hh ui のシグナルを osfs.StopSignals にする**

`internal/app/cli/ui.go:135`:

```go
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
```

を次にする:

```go
	return signal.NotifyContext(context.Background(), osfs.StopSignals()...)
```

`ui.go` の import から `"syscall"` を消す（`os` はほかで使うので残る）。

- [ ] **Step 4: Store のディレクトリの比較を osfs.SamePath にする**

- `internal/app/usecase/readonly.go:54`: `if e.Path == s.Dir() {` → `if osfs.SamePath(e.Path, s.Dir()) {`
- `internal/app/usecase/session.go:285`: `if m.Entry.Path != dir {` → `if !osfs.SamePath(m.Entry.Path, dir) {`
- `internal/app/server/poll.go:130`: `if st.path != path {` → `if !osfs.SamePath(st.path, path) {`
- `internal/app/server/poll.go:235`: `if st == nil || st.path != e.Path {` → `if st == nil || !osfs.SamePath(st.path, e.Path) {`

```bash
bash <scratchpad>/add-import.sh github.com/kukv/hedgehug/internal/lib/osfs internal/app/usecase/readonly.go internal/app/usecase/session.go internal/app/server/poll.go
```

`ui.go` の使わなくなった `"syscall"` は Step 5 の `golangci-lint fmt` が消す。

- [ ] **Step 5: 整形し、ビルドと lint を通す**

```bash
golangci-lint fmt ./...
go build ./... && go vet ./...
make lint
make fmt-check
grep -rnE '\bos\.(Rename|Remove|RemoveAll|ReadFile|WriteFile|OpenFile|MkdirAll)\(|exec\.Command\(|\bsyscall\.|\bunix\.' --include='*.go' internal cmd tools | grep -v '_test\.go:' | grep -vE '^internal/lib/(osfs|testenv|fakegh)/|^internal/lib/gitrepo/gitrepotest/|^internal/app/storage/history/historytest/'
```

Expected: lint と fmt-check が通る。最後の grep は何も出さない。

- [ ] **Step 6: コミット**

```bash
git add -A internal/app
git commit -m "refactor: remote・usecase・cli・server の OS の呼び出しを osfs にする"
```

---

### Task 6: 全体を確かめる

**Files:** なし（確かめるだけ。直しが出たら、その原因の Task のファイルを直してコミットする）

- [ ] **Step 1: 完了条件を確かめる**

```bash
test ! -e internal/lib/atomicfile && echo "atomicfile なし"
go list -deps ./internal/lib/github | grep -c 'internal/app/'
go list -f '{{join .Imports "\n"}}' ./internal/app/remote/auth ./internal/app/storage/config | grep -c 'internal/app/storage/store'
grep -rln '"syscall"\|golang.org/x/sys/unix' --include='*.go' internal cmd tools | grep -v '_test\.go$'
```

Expected: 1 行目は `atomicfile なし`。2・3 行目は `0`。4 行目は `internal/lib/osfs/` のファイルと `internal/lib/testenv/testenv.go` だけ。

- [ ] **Step 2: 全テストを 1 回回す**

Run: `make test`
Expected: PASS。落ちたら、失敗の出力と実行条件（コマンド、`-race` の有無）を作業記録に残してから直す。

- [ ] **Step 3: push して PR を作る**

利用者に確かめてから push し、PR を作る。PR の説明には、forbidigo の禁止の一覧、`github.Options` の決定（2026-11-04 利用者が決定）、`unix.*` も禁止に含めたこと、挙動を変えていないことを書く。
