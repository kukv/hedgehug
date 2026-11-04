# 構成見直し 回 5: テストと仕上げ Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** テストを、試しているものの場所に置き直す（`final_*_test.go` の配り直し、組み立てたバイナリを動かすテストを `cli/scenario` へ、history のベンチの上の層の計測を usecase へ）。depguard で層の向きと領域どうしの import の禁止を強制し、CI の shard を今のパッケージの時間で分け直し、親設計の「パッケージ構成」を今の配置に書き直す（PR 5a）。続けて、本番のコードにあるテストのためだけの差し込み口を、消す・普通の引数にする・本物の手段で確かめる形に直す（PR 5b）。

**Architecture:** 5a は本番のコードを `WorkspaceView` の 1 か所しか変えない（領域どうしの import をなくすため）。ほかはテストのファイルの移動と、lint・CI・Makefile・文書。5b は本番のコードの差し込み口を外し、同じ Task でそれを使っていたテストの「状態の作り方」を書き換える。どちらの PR も挙動を変えない。

**Tech Stack:** Go、golangci-lint 2.15.0（depguard）、GitHub Actions、`tools/restructure/carve`（同じパッケージ内の宣言の移動。回 3 の道具）、Claude Docs（親設計）。

**Spec:** `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`（5 節、7・8 節、決定 2・12・16・17・18・19、6 節の PR 5a・5b）。概要 plan: `docs/superpowers/plans/2026-11-04-codebase-restructure.md`

## Global Constraints

- 概要 plan の Global Constraints をすべて守る
- 挙動を変えない。CLI の出力（`--json` を含む）・終了コード・stderr の文言・コミットの中身・ファイルの形式・server の応答（`api/openapi.yaml`）は変わらない。例外は 5b の F1・F2（spec 決定 19: object が欠けた Store では、一覧・読み取り・同期がエラーで止まる）だけ
- テストの中身の変更は、5a ではファイル・パッケージの移動に伴う import・パッケージ名・補助の名前の書き換えだけ。5b では spec 決定 18 の「状態の作り方」の書き換えだけ（確かめる内容は変えない。変える場合はその Task に書いた 1 行のとおり）
- 各 Task の終わりに `go build ./...` と `go vet ./...` を通し、Task に書いたテストだけを回す（対象を絞る。`TMPDIR` は worktree の `.tmp` に向ける: `mkdir -p .tmp && export TMPDIR=$PWD/.tmp`）。`go test -race ./...` 全体は 5a の Task 9 と 5b の最後の Task で 1 回ずつ
- テストが落ちたら、失敗の出力と実行条件（コマンド、`-race` の有無）を報告に残す。推測で直さない
- spec・この plan と違う挙動が要るとわかったら、止めて報告する（自分で決めない）
- `docs/superpowers/specs/` と `docs/superpowers/plans/` の既存の文書は、この plan が指示する箇所の他は書き換えない
- コミットは利用者が席にいるときだけ（1Password の署名）。離席中は `git add` で index に積む

## この回で決めたこと（2026-11-05 利用者が決定。spec の決定 16〜18 に書いた）

1. **`cli/scenario` には、組み立てたバイナリ（hedgehug・devenv）だけを相手にするテストを置く**（決定 16）。`e2e_test.go`・`e2e_3a_test.go`（`TestE2EAuthLoginAndStatus` を除く）・`e2e_3b_test.go`・`e2e_3c_test.go`。cli の非公開の `run`・`env` と `testEnv` を使う `TestScenario`（`scenario_test.go`）と `TestE2EAuthLoginAndStatus` は cli に残す（後者は `auth_test.go` へ）。cli の中身をテストのために公開する案（`cli.Run(args, Env)` と `cli/clitest`）は採らない。`golden_test.go` はテストを持たない golden の比較の補助で、cli の 18 のテストが使うので cli に残す
2. **バイナリを組み立てる補助は `lib/testenv.Build(tb, pkg, name)` にする。** cli（merge driver・バックグラウンド同期のテストが hedgehug を組み立てる）と scenario の両方が使う。hedgehug を知らない汎用の形（import path と名前を受ける）
3. **領域どうしの import は、depguard で本番のファイルだけ禁止する**（決定 17）。本番の違反は `usecase/stores/store_view.go` → `workspaces`（`WorkspaceView`・`WorkspaceList`）の 1 か所で、回 4 の plan の決めごと 2 で足した import。`WorkspaceView` と `WorkspaceViewOf` を土台（`usecase/workspace_view.go`）に下ろして消す。テストの準備で別の領域の操作を使う import（約 40 か所）と usecasetest は対象外
4. **history のベンチの上の層の計測は、測っている操作の場所へ移す。** Show・ShowAfterOneCommit・LogTask は `usecase/browse`、Undo・RestoreAt は `usecase/tasks`、FixedCost（go-git の固定費と 1 回の書き込み）は `usecase` 直下。history に残るのは Build・OpenFresh・OpenAfterOneCommit（書き込みは `store.Update` で直接）・Consolidate。共通の補助（合成した Store、作業ディレクトリ、索引を作ったコピー）は `historytest`、ベンチ用の Session は `usecasetest.BenchSession`。`make bench-history` は 4 パッケージを `-p 1` で動かす（パッケージを並べて動かすと計測が互いに乱れるため）
5. **CI の shard は 6 つのまま分け直す。** main 4a9cf71 の CI（ubuntu、-race）では、`ops`（usecase 直下だけ）が 12.7 秒、`others` が 2.5 分（history 65 秒、stores 32 秒、tasks 23 秒、maintenance 21 秒）で最も遅い。新しい分け方: `usecase` = `internal/app/usecase/...`、`cli` = `internal/app/cli/...`（scenario を含む）、`storage` = `internal/app/storage/...`、`sync-property`・`sync-property-stores` は今のまま、`others` = 残り全部（replicate の property 以外、server、lib、domain、remote/auth、cmd、tools、web）。shard の数を増やさないのは、macOS の runner が待たされる（main 4a9cf71 の CI で macOS の ops が 5 分遅れて始まった）ため
6. **テストのための差し込み口（5b）は、下の「差し込み口の一覧」の A・B・C を直し、D は残す**（決定 18）

## 調べてわかっていること（main 4a9cf71）

- 層の向きに反する依存（2 節）は、`storage/history/bench_test.go` が `usecase`・`usecase/browse`・`usecase/tasks` を import している 1 ファイルだけ。下書きの depguard の規則（Task 7）を今のコードに当てると、この 3 つと `stores/store_view.go` → `workspaces` の 4 件だけが出る（確かめ済み）
- `final_*_test.go` は 3 つ: `internal/app/usecase/final_review_test.go`（8 テストと補助 2 つ）、`internal/app/usecase/final_fixes_test.go`（9 テスト）、`internal/app/cli/final_review_test.go`（6 テストと補助 1 つ）
- cli の e2e のファイルは、`buildHedgehug`・`goBuildArgs`（`helpers_test.go`）と TestMain の準備のほかに cli へ依存しない。`devUser`・`buildDevenv`・`initDevenv`・`sleepToNextSecond` などは e2e のファイルの中だけで使う。`buildHedgehug` は cli の `merge_driver_test.go`・`merge_driver_more_test.go`・`background_test.go` も使う
- `golden_test.go` を使うテストは cli の 18 ファイル。scenario に移るテストは golden を使わない
- `TestPropertyConverges`・`TestPropertyConvergesStores` は replicate の `property_test.go` にだけある

---

# PR 5a: テストと仕上げ

ブランチ `restructure5-tests`（main 4a9cf71 から）。

### Task 1: `testenv.Build` — バイナリを組み立てる補助

**Files:**
- Modify: `internal/lib/testenv/testenv.go`
- Create: `internal/lib/testenv/testdata/hello/main.go`
- Test: `internal/lib/testenv/testenv_test.go`
- Modify: `internal/app/cli/helpers_test.go:326-366`（`buildHedgehug`・`goBuildArgs`）

**Interfaces:**
- Produces: `func Build(tb testing.TB, pkg, name string) string` — main パッケージ `pkg`（import path か、テストのディレクトリからの相対パス）を、テストのプロセスで 1 回だけ、`name` という名前のバイナリに組み立て、そのパスを返す。テストのバイナリが `-race` 付きなら `-race` で組み立てる。置き場所は `os.MkdirTemp("", name+"-bin-")`（`Setup` の後なら、テストのプロセスのディレクトリの中で、cleanup が消す）

- [ ] **Step 1: 失敗するテストを書く**

`internal/lib/testenv/testdata/hello/main.go`:

```go
// Command hello is what TestBuild builds.
package main

import "fmt"

func main() { fmt.Println("hello") }
```

`internal/lib/testenv/testenv_test.go` の末尾に足す:

```go
func TestBuild(t *testing.T) {
	bin := Build(t, "./testdata/hello", "hello")
	if filepath.Base(bin) != "hello" {
		t.Errorf("Build = %s, want a binary called hello", bin)
	}
	out, err := exec.Command(bin).Output() //nolint:gosec // the binary the test built
	if err != nil || string(out) != "hello\n" {
		t.Fatalf("%s: %q, %v", bin, out, err)
	}
	if again := Build(t, "./testdata/hello", "hello"); again != bin {
		t.Errorf("the second Build = %s, want the first one, %s", again, bin)
	}
}
```

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test -count=1 ./internal/lib/testenv -run TestBuild`
Expected: FAIL（`undefined: Build`）

- [ ] **Step 3: 実装する**

`internal/lib/testenv/testenv.go` の import に `"os/exec"`・`"runtime/debug"`・`"sync"` を足し、パッケージの doc の最後の文を「WaitFor waits for a condition, and Build builds a command for the tests to run.」にして、末尾に足す:

```go
// builds are the binaries Build made in this process, by package.
var (
	buildsMu sync.Mutex
	builds   = map[string]*built{}
)

type built struct {
	once sync.Once
	path string
	err  error
}

// Build builds the main package pkg (an import path, or a path relative to
// the directory of the test) once in this process, as a binary called name in
// a temporary directory of its own, and returns its path. The build has -race
// when the test binary has it: the packages are then in the build cache from
// the build of the tests, and are not compiled again.
func Build(tb testing.TB, pkg, name string) string {
	tb.Helper()
	buildsMu.Lock()
	b := builds[pkg]
	if b == nil {
		b = &built{}
		builds[pkg] = b
	}
	buildsMu.Unlock()
	b.once.Do(func() {
		dir, err := os.MkdirTemp("", name+"-bin-")
		if err != nil {
			b.err = err
			return
		}
		b.path = filepath.Join(dir, name)
		args := []string{"build"}
		if raceEnabled() {
			args = append(args, "-race")
		}
		args = append(args, "-o", b.path, pkg)
		if out, err := exec.Command("go", args...).CombinedOutput(); err != nil { //nolint:gosec // the arguments are the caller's
			b.err = fmt.Errorf("go build %s: %w\n%s", pkg, err, out)
		}
	})
	if b.err != nil {
		tb.Fatal(b.err)
	}
	return b.path
}

// raceEnabled reports whether this binary was built with -race.
func raceEnabled() bool {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, s := range bi.Settings {
		if s.Key == "-race" && s.Value == "true" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test -count=1 ./internal/lib/testenv`
Expected: PASS

- [ ] **Step 5: cli の `buildHedgehug` を `testenv.Build` で書き直す**

`internal/app/cli/helpers_test.go` の `var ( buildOnce sync.Once … )`（326-330 行）、`buildHedgehug`（332-352 行）、`goBuildArgs`（354-366 行）を消し、代わりに置く:

```go
// buildHedgehug builds the command once for all tests and returns the path of
// the binary, which is named hedgehug so that a merge driver finds it on PATH.
func buildHedgehug(t *testing.T) string {
	t.Helper()
	return testenv.Build(t, "github.com/kukv/hedgehug/cmd/hedgehug", "hedgehug")
}
```

`e2e_test.go` の `buildDevenv` は `goBuildArgs` を使っているので、同じ Step で書き直す（`devenvOnce`・`devenvBin`・`devenvErr` の var も消す）:

```go
// buildDevenv builds tools/devenv once for all tests and returns the path of
// the binary.
func buildDevenv(t *testing.T) string {
	t.Helper()
	return testenv.Build(t, "github.com/kukv/hedgehug/tools/devenv", "devenv")
}
```

`golangci-lint fmt ./internal/app/cli/ ./internal/lib/testenv/` で import を整える（`runtime/debug`・`sync` などの使われなくなった import が消え、`testenv` が入る）。`helpers_test.go` の TestMain のコメント「Everything the tests make, the binaries buildHedgehug and buildDevenv make included, …」はそのまま正しい（Build も `os.MkdirTemp("", …)` に置く）。

- [ ] **Step 6: 確かめる**

Run: `go build ./... && go vet ./internal/app/cli/ ./internal/lib/testenv/ && go test -count=1 ./internal/app/cli -run 'TestMergeDriver|TestE2EAliceAndBob'`
Expected: PASS

- [ ] **Step 7: コミット**

```bash
git add internal/lib/testenv internal/app/cli/helpers_test.go internal/app/cli/e2e_test.go
git commit -m "test(testenv): バイナリを組み立てる補助 Build を足す"
```

### Task 2: `cli/scenario` — 組み立てたバイナリを動かすテストを移す

**Files:**
- Move: `internal/app/cli/e2e_test.go` → `internal/app/cli/scenario/e2e_test.go`（`e2e_3a_test.go`・`e2e_3b_test.go`・`e2e_3c_test.go` も同じ）
- Create: `internal/app/cli/scenario/doc.go`、`internal/app/cli/scenario/main_test.go`
- Modify: `internal/app/cli/auth_test.go`（`TestE2EAuthLoginAndStatus` と `e2e_3a_test.go` の冒頭のコメントのうちそのテストの分を受け取る）

**Interfaces:**
- Consumes: Task 1 の `testenv.Build`
- Produces: パッケージ `internal/app/cli/scenario`（本番のコードは doc.go だけ。テストは `package scenario`）

- [ ] **Step 1: `TestE2EAuthLoginAndStatus` を cli の `auth_test.go` へ移す**

```bash
go -C tools/restructure build -o "$PWD/.tmp/" ./carve
printf 'internal/app/cli/e2e_3a_test.go auth_test.go TestE2EAuthLoginAndStatus\n' > .tmp/round5-auth.carve
.tmp/carve .tmp/round5-auth.carve
golangci-lint fmt ./internal/app/cli/
```

`TestE2EAuthLoginAndStatus` の doc の最後「with go-keyring's mock as the keychain (see above).」の「 (see above).」を「: a built binary would use the keychain of the machine the tests run on.」に置き換える（参照先のコメントは e2e_3a_test.go にもともとない）。確かめ方: `grep -n "see above" internal/app/cli/auth_test.go` が 0 行。

- [ ] **Step 2: e2e の 4 ファイルを移し、パッケージ名を変える**

```bash
mkdir -p internal/app/cli/scenario
git mv internal/app/cli/e2e_test.go internal/app/cli/e2e_3a_test.go internal/app/cli/e2e_3b_test.go internal/app/cli/e2e_3c_test.go internal/app/cli/scenario/
sed -i '1s/^package cli$/package scenario/' internal/app/cli/scenario/e2e*_test.go
```

`internal/app/cli/scenario/doc.go`:

```go
// Package scenario holds the tests that run the built hedgehug, and the devenv
// of the development environment, across commands, as their users do. It has
// no code of its own: the tests reach nothing of package cli but the binary.
package scenario
```

`internal/app/cli/scenario/main_test.go`（cli の TestMain のうち、バイナリを動かすテストに要るものだけ。エディタの役と keyring の mock は要らない）:

```go
package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/lib/testenv"
)

// buildHedgehug builds the command once for all tests and returns the path of
// the binary, which is named hedgehug so that a merge driver finds it on PATH.
func buildHedgehug(t *testing.T) string {
	t.Helper()
	return testenv.Build(t, "github.com/kukv/hedgehug/cmd/hedgehug", "hedgehug")
}

func TestMain(m *testing.M) {
	// Everything the tests make, the binaries included, goes in a directory
	// of this process that cleanup removes.
	cleanup, err := testenv.Setup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		os.Exit(1)
	}
	// A program built with -race sleeps a second when it exits, by default,
	// so that goroutines still running at the exit can be caught in a race.
	// The binaries the tests run leave no goroutine of their own work running
	// when they exit, so for them the sleep only makes each run a second
	// longer.
	_ = os.Setenv("GORACE", strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0"))
	// hedgehug is on PATH, as doctor expects of a store with a merge driver.
	pathDir, err := os.MkdirTemp("", "hedgehug-path-")
	if err == nil {
		if err := os.WriteFile(filepath.Join(pathDir, "hedgehug"), nil, 0o700); err == nil { //nolint:gosec // an empty file that only has to be found
			_ = os.Setenv("PATH", pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}
```

`buildDevenv` は Task 1 で `testenv.Build` を呼ぶ形になっていて、`e2e_test.go` と一緒に移る。`golangci-lint fmt ./internal/app/cli/...` で import を整える（`e2e_3a_test.go` から `go-keyring` などが消える）。

- [ ] **Step 3: 確かめる**

Run: `go build ./... && go vet ./internal/app/cli/... && go test -count=1 ./internal/app/cli/scenario && go test -count=1 ./internal/app/cli -run 'TestE2EAuthLoginAndStatus|TestScenario'`
Expected: PASS。`go test -list . ./internal/app/cli/scenario` に `TestE2EAliceAndBob`・`TestE2EPhase3a`・`TestE2EPhase3b`・`TestE2EPhase3c` の 4 つが出て、`TestE2EAuthLoginAndStatus` は出ない

- [ ] **Step 4: lint**

Run: `golangci-lint run ./internal/app/cli/... ./internal/lib/testenv/...`
Expected: 0 issues（scenario は cli の depguard の規則 entry の対象。import は usecasetest・fakegh・gitrepotest・testenv だけで、どれも許されている）

- [ ] **Step 5: コミット**

```bash
git add -A internal/app/cli
git commit -m "test(cli): 組み立てたバイナリを動かすテストを cli/scenario に移す"
```

### Task 3: cli の `final_review_test.go` を配り直す

**Files:**
- Delete: `internal/app/cli/final_review_test.go`
- Modify: `internal/app/cli/sync_test.go`、`use_test.go`、`inbox_test.go`、`ls_test.go`、`helpers_test.go`

テストの名前は変えない（golden ファイルの `testdata/<テスト名>/` が名前で決まるため）。

- [ ] **Step 1: carve で配る**

`.tmp/round5-cli.carve`:

```
internal/app/cli/final_review_test.go helpers_test.go testEnv.breakStore
internal/app/cli/final_review_test.go sync_test.go TestSyncWarnsOfACrossStoreRefOnce
internal/app/cli/final_review_test.go use_test.go TestUseJSONIDIsPlainWhenThePrefixIsShared TestUseWithStoreLooksInThatStoreOnly
internal/app/cli/final_review_test.go inbox_test.go TestInboxLeavesOutAStoreThatDoesNotOpen
internal/app/cli/final_review_test.go ls_test.go TestStoreFlagKeepsOtherBrokenStoresQuiet TestLsStatusOfAnotherSchema
```

```bash
.tmp/carve .tmp/round5-cli.carve
golangci-lint fmt ./internal/app/cli/
cat internal/app/cli/final_review_test.go   # package cli だけが残っている
git rm -qf internal/app/cli/final_review_test.go   # carve で書き換えた後なので -f
```

（`.tmp/carve` は Task 2 で組み立てたもの。なければ `go -C tools/restructure build -o "$PWD/.tmp/" ./carve`）

- [ ] **Step 2: 確かめる**

Run: `go vet ./internal/app/cli/ && go test -count=1 ./internal/app/cli -run 'TestSyncWarnsOfACrossStoreRefOnce|TestUseJSONIDIsPlainWhenThePrefixIsShared|TestUseWithStoreLooksInThatStoreOnly|TestInboxLeavesOutAStoreThatDoesNotOpen|TestStoreFlagKeepsOtherBrokenStoresQuiet|TestLsStatusOfAnotherSchema'`
Expected: PASS（6 テスト）

- [ ] **Step 3: コミット**

```bash
git add -A internal/app/cli
git commit -m "test(cli): final_review_test のテストを試しているコマンドのファイルへ移す"
```

### Task 4: usecase の `final_review_test.go`・`final_fixes_test.go` を配り直す

**Files:**
- Delete: `internal/app/usecase/final_review_test.go`、`internal/app/usecase/final_fixes_test.go`
- Create: `internal/app/usecase/write_view_test.go`
- Modify: `internal/app/usecase/usecasetest/session.go`（補助 2 つ）、`internal/app/usecase/mount_test.go`、`session_test.go`、`errors_test.go`、`stores/use_test.go`、`replication/sync_test.go`、`notifications/inbox_test.go`、`tasks/addhere_test.go`、`tasks/edit_test.go`、`comments/comment_test.go`

**Interfaces:**
- Produces: `func (ts *Session) AddGoneStore(t *testing.T, withRemote bool)`（旧 `addGoneStore(t, ts, withRemote)`）、`func CountContaining(warns []string, sub string) int`（旧 `countContaining`）。どちらも `usecasetest`

行き先（テストの名前は変えない）:

| テスト | 行き先 |
| --- | --- |
| TestStoreNameKeepsTheWarningsOfOtherStoresQuiet、TestWarnMayCallBackIntoTheSession | `usecase/mount_test.go` |
| TestSessionConcurrentOperations、TestForStoreCopiesEveryExportedField、TestSessionUpdateCommitAndBeforeUpdate | `usecase/session_test.go` |
| TestAsError、TestNotFoundCodes | `usecase/errors_test.go` |
| TestResultsCarryTheCommitAndAFreshView、TestResultIndexIsFromBeforeTheWrite | `usecase/write_view_test.go`（新規。`write_view.go` の結果の型を試す。`usecase` の `export_test.go` の `ResultIndexHas` を使うので土台に残る） |
| TestUseWithStoreLooksInThatStoreOnly、TestUseWithStoreAmbiguityKeepsTheInput | `usecase/stores/use_test.go` |
| TestSyncDoesNotWarnTwiceOfAStoreItReports | `usecase/replication/sync_test.go` |
| TestInboxSkipsAStoreThatDoesNotOpen、TestInboxStoreWhoseInboxCannotBeReadIsWarnedOf | `usecase/notifications/inbox_test.go` |
| TestAddHereUnknownStoreIsStoreNotFound | `usecase/tasks/addhere_test.go` |
| TestEditIsOneCall | `usecase/tasks/edit_test.go` |
| TestCommentAddCleansTheBody | `usecase/comments/comment_test.go` |

- [ ] **Step 1: 補助 2 つを usecasetest に移す**

`final_review_test.go` の `addGoneStore` と `countContaining`（doc を含む 21〜55 行。消す前に `sed -n '21,55p'` で、この 2 つの関数とその doc・後ろの空行だけであることを確かめる）を `sed -i '21,55d' internal/app/usecase/final_review_test.go` で消し、`internal/app/usecase/usecasetest/session.go` の末尾に足す（中身は同じ。`ts` が受け手になる）:

```go
// AddGoneStore sets up a store called gone in config.yaml at a path where
// there is no store, with a remote when withRemote.
func (ts *Session) AddGoneStore(t *testing.T, withRemote bool) {
	t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	var remotes []string
	if withRemote {
		remotes = []string{filepath.Join(t.TempDir(), "remote.git")}
	}
	if err := c.AddStore("gone", filepath.Join(t.TempDir(), "gone"), remotes); err != nil {
		t.Fatal(err)
	}
	if err := c.SetStoreAccount("gone", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.DropMount()
	ts.Warns = nil
}

// CountContaining is how many of warns have sub in them.
func CountContaining(warns []string, sub string) int {
	var n int
	for _, w := range warns {
		if strings.Contains(w, sub) {
			n++
		}
	}
	return n
}
```

呼び出しを書き換える:

```bash
sed -i -e 's/addGoneStore(t, ts, /ts.AddGoneStore(t, /g' -e 's/\bcountContaining(/usecasetest.CountContaining(/g' internal/app/usecase/final_review_test.go
```

- [ ] **Step 2: 土台に残るテストを carve で配る**

`.tmp/round5-base.carve`:

```
internal/app/usecase/final_review_test.go mount_test.go TestStoreNameKeepsTheWarningsOfOtherStoresQuiet TestWarnMayCallBackIntoTheSession
internal/app/usecase/final_fixes_test.go session_test.go TestSessionConcurrentOperations TestForStoreCopiesEveryExportedField TestSessionUpdateCommitAndBeforeUpdate
internal/app/usecase/final_fixes_test.go errors_test.go TestAsError TestNotFoundCodes
internal/app/usecase/final_fixes_test.go write_view_test.go TestResultsCarryTheCommitAndAFreshView TestResultIndexIsFromBeforeTheWrite
```

```bash
.tmp/carve .tmp/round5-base.carve
```

- [ ] **Step 3: 領域へ移るテストを、いったん土台の別ファイルに切り出してから移す**

carve は同じパッケージの中でしか動かないので、領域ごとの一時ファイルに切り出し、ディレクトリを移してパッケージ名を変え、行き先のファイルへもう一度 carve する。

`.tmp/round5-out.carve`:

```
internal/app/usecase/final_review_test.go r5_stores_test.go TestUseWithStoreLooksInThatStoreOnly TestUseWithStoreAmbiguityKeepsTheInput
internal/app/usecase/final_review_test.go r5_replication_test.go TestSyncDoesNotWarnTwiceOfAStoreItReports
internal/app/usecase/final_review_test.go r5_notifications_test.go TestInboxSkipsAStoreThatDoesNotOpen TestInboxStoreWhoseInboxCannotBeReadIsWarnedOf
internal/app/usecase/final_review_test.go r5_tasks_addhere_test.go TestAddHereUnknownStoreIsStoreNotFound
internal/app/usecase/final_fixes_test.go r5_tasks_edit_test.go TestEditIsOneCall
internal/app/usecase/final_fixes_test.go r5_comments_test.go TestCommentAddCleansTheBody
```

```bash
.tmp/carve .tmp/round5-out.carve
u=internal/app/usecase
mv_to() { # file domain
  git mv -f "$u/$1" "$u/$2/$1" 2>/dev/null || mv "$u/$1" "$u/$2/$1"
  sed -i "1s/^package usecase_test$/package $2_test/" "$u/$2/$1"
}
mv_to r5_stores_test.go stores
mv_to r5_replication_test.go replication
mv_to r5_notifications_test.go notifications
mv_to r5_tasks_addhere_test.go tasks
mv_to r5_tasks_edit_test.go tasks
mv_to r5_comments_test.go comments
cat > .tmp/round5-in.carve <<'EOF'
internal/app/usecase/stores/r5_stores_test.go use_test.go TestUseWithStoreLooksInThatStoreOnly TestUseWithStoreAmbiguityKeepsTheInput
internal/app/usecase/replication/r5_replication_test.go sync_test.go TestSyncDoesNotWarnTwiceOfAStoreItReports
internal/app/usecase/notifications/r5_notifications_test.go inbox_test.go TestInboxSkipsAStoreThatDoesNotOpen TestInboxStoreWhoseInboxCannotBeReadIsWarnedOf
internal/app/usecase/tasks/r5_tasks_addhere_test.go addhere_test.go TestAddHereUnknownStoreIsStoreNotFound
internal/app/usecase/tasks/r5_tasks_edit_test.go edit_test.go TestEditIsOneCall
internal/app/usecase/comments/r5_comments_test.go comment_test.go TestCommentAddCleansTheBody
EOF
.tmp/carve .tmp/round5-in.carve
rm -f $u/*/r5_*_test.go
golangci-lint fmt ./internal/app/usecase/...
golangci-lint fmt ./internal/app/usecase/...   # 2 回目: 1 回目の後、session_test.go の import に空行が残る
cat $u/final_review_test.go $u/final_fixes_test.go   # どちらも package usecase_test だけが残っている
git rm -qf $u/final_review_test.go $u/final_fixes_test.go
```

（r5_ のファイルは git に入る前に消えるので、`git mv` が使えないときは `mv` で移す。最後の `rm` は carve が中身を移した後の、パッケージ名と import だけが残ったファイルを消す）

移ったテストのうち、土台の名前を `usecase.` なしで呼んでいるものはない（すべて外部テストパッケージ `usecase_test` からの移動なので、土台の名前は既に `usecase.` 付き）。`stores` という名前のローカル変数（`TestInboxStoreWhoseInboxCannotBeReadIsWarnedOf` の `stores, err := notifications.InboxStores(…)`）は notifications_test では import 名とぶつからないので、そのままでよい。

- [ ] **Step 4: 確かめる**

Run: `go build ./... && go vet ./internal/app/usecase/... && go test -count=1 ./internal/app/usecase ./internal/app/usecase/stores ./internal/app/usecase/replication ./internal/app/usecase/notifications ./internal/app/usecase/tasks ./internal/app/usecase/comments -run 'TestStoreNameKeepsTheWarningsOfOtherStoresQuiet|TestWarnMayCallBackIntoTheSession|TestSessionConcurrentOperations|TestForStoreCopiesEveryExportedField|TestSessionUpdateCommitAndBeforeUpdate|TestAsError|TestNotFoundCodes|TestResultsCarryTheCommitAndAFreshView|TestResultIndexIsFromBeforeTheWrite|TestUseWithStoreLooksInThatStoreOnly|TestUseWithStoreAmbiguityKeepsTheInput|TestSyncDoesNotWarnTwiceOfAStoreItReports|TestInboxSkipsAStoreThatDoesNotOpen|TestInboxStoreWhoseInboxCannotBeReadIsWarnedOf|TestAddHereUnknownStoreIsStoreNotFound|TestEditIsOneCall|TestCommentAddCleansTheBody'`
Expected: PASS（17 テスト。`-v` で数えるなら `go test -v … | grep -c '^--- PASS'` が 17 以上）

`find internal -name 'final_*_test.go'` が何も出さない。

- [ ] **Step 5: コミット**

```bash
git add -A internal/app/usecase
git commit -m "test(usecase): final_review・final_fixes のテストを試している操作のファイルへ移す"
```

### Task 5: history のベンチの上の層の計測を usecase へ移す

**Files:**
- Create: `internal/app/storage/history/historytest/bench.go`
- Create: `internal/app/usecase/usecasetest/bench.go`
- Create: `internal/app/usecase/browse/bench_test.go`、`internal/app/usecase/tasks/bench_test.go`、`internal/app/usecase/bench_test.go`
- Modify: `internal/app/storage/history/bench_test.go`
- Modify: `Makefile`（`bench-history`）

**Interfaces:**
- Produces（historytest）: `const BenchCommits = 100_000`、`const BenchTasks = 5_000`、`func BenchSource(b *testing.B) string`、`func WorkDir(b *testing.B) string`、`func Indexed(b *testing.B, src string) string`
- Produces（usecasetest）: `func BenchSession(b *testing.B, dir, account string) *usecase.Session`

- [ ] **Step 1: 共通の補助を historytest に移す**

`internal/app/storage/history/historytest/bench.go`:

```go
package historytest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// The benchmarks of the history index, and of the commands that read it, run
// on a synthesized store of BenchCommits commits and BenchTasks tasks (see
// make bench-history). They are skipped unless HH_BENCH is set.
const (
	BenchCommits = 100_000
	BenchTasks   = 5_000
	benchSeed    = 1
)

// BenchSource is the synthesized store of the benchmarks. It skips b unless
// HH_BENCH is set.
func BenchSource(b *testing.B) string {
	b.Helper()
	if os.Getenv("HH_BENCH") == "" {
		b.Skip("set HH_BENCH=1 (make bench-history) to measure on a store of 100,000 commits")
	}
	return Synth(b, BenchCommits, BenchTasks, benchSeed)
}

// WorkDir is a directory next to the synthesized stores, so that the packs
// can be linked into the copies.
func WorkDir(b *testing.B) string {
	b.Helper()
	d, err := os.MkdirTemp(CacheDir(b), "work-")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// Indexed is a copy of the synthesized store with its history index and its
// task index (both caches in .hedgehug/local) made, in its own directory.
func Indexed(b *testing.B, src string) string {
	b.Helper()
	dir := filepath.Join(WorkDir(b), "store")
	Clone(b, src, dir)
	s, err := store.Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := history.Open(s, history.Options{}); err != nil {
		b.Fatal(err)
	}
	if _, err := index.Open(s); err != nil {
		b.Fatal(err)
	}
	_ = s.Repo().Close()
	return dir
}
```

（historytest は history を import するが、history の内部テスト（`package history`）は historytest を import していないので循環しない。`grep -l historytest internal/app/storage/history/*_test.go | xargs grep -l '^package history$'` が何も出さないことを確かめる）

- [ ] **Step 2: ベンチ用の Session を usecasetest に置く**

`internal/app/usecase/usecasetest/bench.go`（旧 `history/bench_test.go` の `session` と同じ中身）:

```go
package usecasetest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/history/historytest"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// BenchSession is a session on the store at dir, as the account (by name: one
// of historytest.Accounts, or dave, who wrote nothing), at a time after every
// commit of the synthesized store of the benchmarks.
func BenchSession(b *testing.B, dir, account string) *usecase.Session {
	b.Helper()
	home := b.TempDir()
	env := map[string]string{"HOME": home, "HEDGEHUG_HOME": filepath.Join(home, "hh"), "HEDGEHUG_AUTO_SYNC": "0"}
	now := historytest.Start.Add(2 * historytest.BenchCommits * time.Minute)
	ss := &usecase.Session{
		Getenv: func(k string) string { return env[k] },
		Now:    func() time.Time { return now },
		Warn:   func(string) {},
	}
	c, _, err := ss.Config()
	if err != nil {
		b.Fatal(err)
	}
	if err := c.AddStore("me", dir, nil); err != nil {
		b.Fatal(err)
	}
	if err := c.Save(); err != nil {
		b.Fatal(err)
	}
	var acct config.Account
	for _, a := range append(historytest.Accounts, config.Account{Name: "dave", Host: "github.com", Login: "dave"}) {
		if a.Name == account {
			acct = a
		}
	}
	f, err := os.OpenFile(filepath.Join(env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		b.Fatal(err)
	}
	_, err = f.WriteString("accounts:\n  - name: " + acct.Name + "\n    host: " + acct.Host + "\n    login: " + acct.Login + "\n")
	if err := errors.Join(err, f.Close()); err != nil {
		b.Fatal(err)
	}
	return ss
}
```

- [ ] **Step 3: ベンチを分ける**

関数の切り出しは、関数ごとに `func BenchmarkX(` の行から、その関数を閉じる行頭の `}` まで（直前の doc コメントを含む）を、行番号を確かめてから移す（`grep -n '^func \|^}' internal/app/storage/history/bench_test.go` で範囲を見る）。名前の書き換えは次の sed で行う（移した先のファイルに対して）:

```bash
sed -i -e 's/\bbenchSource(b)/historytest.BenchSource(b)/g' -e 's/\bindexed(/historytest.Indexed(/g' -e 's/\bworkDir(b)/historytest.WorkDir(b)/g' -e 's/\bsession(b, /usecasetest.BenchSession(b, /g' -e 's/\bbenchCommits\b/historytest.BenchCommits/g' <file>
```

各ファイルの import（`golangci-lint fmt` が使われないものを消し、足りないものは足さないので、最初からこの一覧で書く）:
- `usecase/browse/bench_test.go`: `os`、`path/filepath`、`testing`、`domain/model`、`storage/history/historytest`、`storage/store`、`usecase/browse`、`usecase/usecasetest`
- `usecase/tasks/bench_test.go`: `os`、`path/filepath`、`testing`、`time`、`storage/history/historytest`、`usecase/tasks`、`usecase/usecasetest`
- `usecase/bench_test.go`: `os`、`path/filepath`、`testing`、`domain/model`、`storage/history/historytest`、`storage/store`、`usecase/usecasetest`、`lib/gitrepo`
- `storage/history/bench_test.go`（残り）: `crypto/rand`、`os`、`path/filepath`、`testing`、go-git の `git`・`plumbing`・`plumbing/format/packfile`・`plumbing/storer`・`storage/memory`、`domain/model`、`storage/history`、`storage/history/historytest`、`storage/store`、`lib/gitrepo`（`usecase`・`browse`・`tasks`・`config`・`index`・`errors`・`time` は要らなくなる）

`internal/app/usecase/browse/bench_test.go`（`package browse_test`）に、旧 `history/bench_test.go` の `BenchmarkShow`・`BenchmarkShowAfterOneCommit`・`BenchmarkLogTask` を、doc を含めてそのまま移し、名前を書き換える: `benchSource(b)` → `historytest.BenchSource(b)`、`indexed(` → `historytest.Indexed(`、`workDir(b)` → `historytest.WorkDir(b)`、`session(b, ` → `usecasetest.BenchSession(b, `、`browse.` はそのまま（外部テストパッケージ）。

`internal/app/usecase/tasks/bench_test.go`（`package tasks_test`）に `BenchmarkUndo`・`BenchmarkRestoreAt` を同じ書き換えで移す。`BenchmarkRestoreAt` の `benchCommits` は `historytest.BenchCommits` にする。

`internal/app/usecase/bench_test.go`（`package usecase_test`）に `BenchmarkFixedCost` を同じ書き換えで移す。

`internal/app/storage/history/bench_test.go` からは上の 6 つと `session` を消し、`benchSource`・`workDir`・`indexed` と定数も消して、残る `BenchmarkBuild`・`BenchmarkOpenFresh`・`BenchmarkOpenAfterOneCommit`・`BenchmarkConsolidate`（と `addPack`）の呼び出しを `historytest.BenchSource`・`historytest.WorkDir`・`historytest.Indexed` に書き換える。ファイルの冒頭のコメント（「The benchmarks run on a synthesized store …」）は historytest に移ったので消す。

`BenchmarkOpenAfterOneCommit` の書き込みは Session を使わず、store で直接する（計る区間は `history.Open` だけで、変わらない）:

```go
		dir := filepath.Join(historytest.WorkDir(b), "store")
		historytest.Clone(b, src, dir)
		s, err := store.Open(dir)
		if err != nil {
			b.Fatal(err)
		}
		var task model.ULID
		for t, err := range s.Tasks() {
			if err != nil {
				b.Fatal(err)
			}
			task = t.ULID
			break
		}
		a := historytest.Accounts[0]
		op := store.Op{Name: "edit", Task: task, Author: gitrepo.Signature{Name: a.Login, Email: a.Login + "@users.noreply.github.com"}}
		if _, err := s.Update(op, func(tx *store.Tx) error {
			t, err := tx.Task(task)
			if err != nil {
				return err
			}
			t.Title += " (changed)"
			return tx.PutTask(t)
		}); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
```

（`historytest.Accounts[0]` が alice であることを `synth.go:40` で確かめる。旧コードの `session(b, dir, "alice")` と同じ作者になる）

```bash
golangci-lint fmt ./internal/app/storage/history/... ./internal/app/usecase/...
```

- [ ] **Step 4: Makefile**

```make
# Measure the history index, pack consolidation, and the commands that read
# the history (show, log, undo, restore --at, one write) on a synthesized store
# of 100,000 commits (not part of check or CI; see the history-growth spec).
# The store is made once in $HH_BENCH_CACHE (default: hh-bench in the user
# cache directory). -p 1 keeps the packages from measuring at the same time.
bench-history:
	HH_BENCH=1 go test -p 1 ./internal/app/storage/history ./internal/app/usecase ./internal/app/usecase/browse ./internal/app/usecase/tasks -run '^$$' -bench . -benchtime 5x -timeout 60m
```

- [ ] **Step 5: 確かめる（HH_BENCH なしでは skip されることと、ベンチが組み立つこと）**

Run: `go vet ./internal/app/storage/history/... ./internal/app/usecase/... && go test -count=1 -v -run '^$' -bench . -benchtime 1x ./internal/app/storage/history ./internal/app/usecase ./internal/app/usecase/browse ./internal/app/usecase/tasks 2>&1 | grep -c -- '--- SKIP: Benchmark'`
Expected: 10（history の Build・OpenFresh・OpenAfterOneCommit・Consolidate、browse の Show・ShowAfterOneCommit・LogTask、tasks の Undo・RestoreAt、usecase の FixedCost。サブベンチを持つものは `b.Run` の前の `BenchSource` で親が skip するので 1 つずつ。数が違えば名前を見て、移し忘れか重複かを確かめる）。
`grep -n 'usecase' internal/app/storage/history/bench_test.go` が 0 行。
本物の計測（`make bench-history`）は合成に時間がかかるので回さない。

- [ ] **Step 6: コミット**

```bash
git add -A internal/app/storage/history internal/app/usecase Makefile
git commit -m "test(bench): history のベンチの上の層の計測を usecase へ移す"
```

### Task 6: `WorkspaceView` を土台に下ろす

**Files:**
- Create: `internal/app/usecase/workspace_view.go`
- Modify: `internal/app/usecase/workspaces/workspace.go:17-40`、`internal/app/usecase/stores/store_view.go:7,51-73`
- Modify（名前の書き換え）: `internal/app/cli/meanwhile_test.go`、`internal/app/server/admin.go`、`admin_test.go`、`admin_more_test.go`、`tasks/undo_test.go`、`workspaces/workspace_test.go`（Step 2 のループの対象のうち `cli/workspace.go` と `usecase/readonly_test.go` は参照がなく、差分が出なくてよい）

**Interfaces:**
- Produces: `usecase.WorkspaceView`（旧 `workspaces.WorkspaceView`。フィールドと JSON は同じ）、`func WorkspaceViewOf(ix *index.Index, w *model.Workspace) WorkspaceView`（旧 `workspaces.WorkspaceViewOf`）。`workspaces.WorkspaceList` は残り、`[]usecase.WorkspaceView` を返す

- [ ] **Step 1: 型と関数を土台へ移す**

`internal/app/usecase/workspace_view.go`:

```go
package usecase

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
)

// WorkspaceView is a workspace as the CLI prints it with --json.
type WorkspaceView struct {
	ULID      string    `json:"ulid"`
	Name      string    `json:"name"`
	IDPrefix  string    `json:"id_prefix"`
	PrefixSet time.Time `json:"prefix_set"`
	Tasks     int       `json:"tasks"`   // not deleted
	Deleted   int       `json:"deleted"` // deleted and not purged yet
}

// WorkspaceViewOf is w as ix counts its tasks.
func WorkspaceViewOf(ix *index.Index, w *model.Workspace) WorkspaceView {
	var live, deleted int
	for _, e := range ix.Tasks() {
		switch {
		case e.Workspace != w.ULID:
		case e.DeletedAt == nil:
			live++
		default:
			deleted++
		}
	}
	return WorkspaceView{ULID: w.ULID.String(), Name: w.Name, IDPrefix: w.IDPrefix, PrefixSet: w.PrefixSet, Tasks: live, Deleted: deleted}
}
```

（移す前に `workspaces/workspace.go:17-40` を読み、WorkspaceView の doc とフィールドのタグ・`doc:` が上と同じかを確かめる。違えば workspace.go の方をそのまま写す）

`workspaces/workspace.go` から `type WorkspaceView` と `func WorkspaceViewOf` の宣言（doc を含む）を消す。

- [ ] **Step 2: 呼び出しを書き換える**

```bash
for f in internal/app/cli/workspace.go internal/app/cli/meanwhile_test.go internal/app/server/admin.go internal/app/server/admin_test.go internal/app/server/admin_more_test.go internal/app/usecase/readonly_test.go internal/app/usecase/tasks/undo_test.go internal/app/usecase/workspaces/workspace_test.go internal/app/usecase/stores/store_view.go; do
  gofmt -w -r 'workspaces.WorkspaceView -> usecase.WorkspaceView' "$f"
  gofmt -w -r 'workspaces.WorkspaceViewOf -> usecase.WorkspaceViewOf' "$f"
done
f=internal/app/usecase/workspaces/workspace.go
gofmt -w -r 'WorkspaceView -> usecase.WorkspaceView' "$f"
gofmt -w -r 'WorkspaceViewOf -> usecase.WorkspaceViewOf' "$f"
```

`internal/app/usecase/stores/store_view.go` の `StoreInitViewOf` を、workspaces を呼ばない形にする（`workspaces.WorkspaceList` と同じく、開いた Store の索引の全ワークスペースを見る）:

```go
	if _, ix, err := ss.Open(); err == nil {
		if wss := ix.Workspaces(); len(wss) == 1 {
			v.Workspace = usecase.WorkspaceViewOf(ix, wss[0])
		}
	}
	return v
```

（旧: `if ws, err := workspaces.WorkspaceList(ss); err == nil && len(ws) == 1 { v.Workspace = ws[0] }`。`WorkspaceList` は `ss.Open()` の `ix.Workspaces()` を `WorkspaceViewOf` で写すだけなので同じ）

`gofmt -r` は import を足し引きしないので、ここで必ず fmt をかける（stores の `workspaces` の import が消え、`usecase` が入る）:

```bash
golangci-lint fmt ./internal/...
```

- [ ] **Step 3: 確かめる**

Run: `go build ./... && go vet ./internal/... && grep -rn 'usecase/workspaces"' internal/app/usecase/stores/*.go | grep -v _test.go; go test -count=1 ./internal/app/server -run TestOpenAPIUpToDate && go test -count=1 ./internal/app/usecase/stores ./internal/app/usecase/workspaces && go test -count=1 ./internal/app/cli -run 'TestStoreInit|TestWorkspace'`
Expected: grep は 0 行。テストは PASS（`TestOpenAPIUpToDate` が通る＝ server の応答の形と `api/openapi.yaml` が変わらない。huma は型の名前で schema を付けるので、パッケージが変わっても `WorkspaceView` のまま）

`git diff --stat api/ web/core/api/` が空。

- [ ] **Step 4: コミット**

```bash
git add -A internal
git commit -m "refactor(usecase): WorkspaceView を土台に下ろし、stores から workspaces への import をなくす"
```

### Task 7: depguard — 層の向きと、領域どうしの import

**Files:**
- Modify: `.golangci.yaml`（`linters.settings.depguard.rules` に 6 つの規則を足す）

- [ ] **Step 1: 規則を足す**

`.golangci.yaml` の `entry:` の規則の後（`forbidigo:` の行の前）に足す:

```yaml
        # 層の向き（spec 決定 2）: lib → domain → storage → remote → usecase → cli・server。
        # 各層は自分・下の層・lib だけを import する。テストも同じ（spec 5 節）。
        # テスト用の部品（決定 12）は、どの層のテストからも import してよい。
        lib:
          list-mode: lax
          files:
            - "**/internal/lib/**"
          allow:
            - github.com/kukv/hedgehug/internal/app/storage/history/historytest
            - github.com/kukv/hedgehug/internal/app/storage/mount/mounttest
            - github.com/kukv/hedgehug/internal/app/usecase/usecasetest
          deny:
            - pkg: github.com/kukv/hedgehug/internal/app
              desc: lib は hedgehug を知らない（spec 決定 1）
        domain:
          list-mode: lax
          files:
            - "**/internal/app/domain/**"
          allow:
            - github.com/kukv/hedgehug/internal/app/storage/history/historytest
            - github.com/kukv/hedgehug/internal/app/storage/mount/mounttest
            - github.com/kukv/hedgehug/internal/app/usecase/usecasetest
          deny:
            - pkg: github.com/kukv/hedgehug/internal/app/storage
              desc: domain は storage より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/remote
              desc: domain は remote より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase
              desc: domain は usecase より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/cli
              desc: domain は cli より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/server
              desc: domain は server より下の層（spec 決定 2）
        storage:
          list-mode: lax
          files:
            - "**/internal/app/storage/**"
          allow:
            - github.com/kukv/hedgehug/internal/app/usecase/usecasetest
          deny:
            - pkg: github.com/kukv/hedgehug/internal/app/remote
              desc: storage は remote より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase
              desc: storage は usecase より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/cli
              desc: storage は cli より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/server
              desc: storage は server より下の層（spec 決定 2）
        remote:
          list-mode: lax
          files:
            - "**/internal/app/remote/**"
          allow:
            - github.com/kukv/hedgehug/internal/app/usecase/usecasetest
          deny:
            - pkg: github.com/kukv/hedgehug/internal/app/usecase
              desc: remote は usecase より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/cli
              desc: remote は cli より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/server
              desc: remote は server より下の層（spec 決定 2）
        usecase:
          list-mode: lax
          files:
            - "**/internal/app/usecase/**"
          deny:
            - pkg: github.com/kukv/hedgehug/internal/app/cli
              desc: usecase は cli より下の層（spec 決定 2）
            - pkg: github.com/kukv/hedgehug/internal/app/server
              desc: usecase は server より下の層（spec 決定 2）
        # usecase の領域どうしは import しない。共有するものは土台（usecase 直下）に置く（spec 2.3）。
        # テストは準備に別の領域の操作を使ってよいので、本番のファイルだけにかける（決定 17）。
        # usecasetest はテスト用の部品なので対象外。
        usecase-domains:
          list-mode: lax
          files:
            - "**/internal/app/usecase/*/**"
            - "!$test"
            - "!**/internal/app/usecase/usecasetest/**"
          deny:
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/accounts
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/browse
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/comments
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/maintenance
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/notifications
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/replication
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/schemas
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/stores
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/tasks
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/views
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/watch
              desc: 領域どうしは土台を通す（spec 2.3）
            - pkg: github.com/kukv/hedgehug/internal/app/usecase/workspaces
              desc: 領域どうしは土台を通す（spec 2.3）
```

（storage の規則の allow に historytest・mounttest がないのは、どちらも storage の中で同じ層だから。usecase の規則に allow がないのは、テスト用の部品がすべて usecase 以下の層にあるから）

- [ ] **Step 2: 通ることを確かめる**

Run: `golangci-lint run --enable-only depguard ./...`
Expected: `0 issues.`（Task 5・6 の前のコードでは、history のベンチの 3 件と stores→workspaces の 1 件が出ていた。plan を書くときに確かめた）

- [ ] **Step 3: 規則が効くことを確かめる（わざと違反を入れて戻す）**

domain の側は、既存のパッケージに上の層の import を足すと import の循環でビルドが先に失敗するので、新しいパッケージで試す。領域の側は、Task 6 の後は領域どうしの本番の import がないので循環しない。

```bash
mkdir -p internal/app/domain/zzprobe
cat > internal/app/domain/zzprobe/probe.go <<'EOF'
package zzprobe

import _ "github.com/kukv/hedgehug/internal/app/storage/store"
EOF
cat > internal/app/usecase/tasks/zz_depguard_probe.go <<'EOF'
package tasks

import _ "github.com/kukv/hedgehug/internal/app/usecase/views"
EOF
cat > internal/app/usecase/tasks/zz_depguard_probe_test.go <<'EOF'
package tasks_test

import _ "github.com/kukv/hedgehug/internal/app/usecase/views"
EOF
golangci-lint run --enable-only depguard ./internal/app/domain/zzprobe/ ./internal/app/usecase/tasks/ ; rm -r internal/app/domain/zzprobe internal/app/usecase/tasks/zz_depguard_probe.go internal/app/usecase/tasks/zz_depguard_probe_test.go
```

Expected: 2 件（`zzprobe/probe.go` の `list 'domain'` と、`zz_depguard_probe.go` の `list 'usecase-domains'`）。`zz_depguard_probe_test.go` は出ない（テストは対象外: 決定 17）。消した後 `git status --short` に probe が残っていないこと。

- [ ] **Step 4: lint 全体**

Run: `make lint && make fmt-check`
Expected: 0 issues

- [ ] **Step 5: コミット**

```bash
git add .golangci.yaml
git commit -m "build(lint): 層の向きと領域どうしの import を depguard で強制する"
```

### Task 8: CI の shard を分け直す

**Files:**
- Modify: `.github/workflows/ci-backend.yaml`（`test-shard` の matrix・コメント・`Test` の case）

- [ ] **Step 1: matrix とコメントを書き換える**

```yaml
        shard: [usecase, cli, storage, sync-property, sync-property-stores, others]
```

`Test` の手順の上のコメント:

```yaml
      # 何を回すかは、パッケージごとの時間（下の Package times）で均した分け方:
      # usecase = internal/app/usecase/...、cli = internal/app/cli/...、storage = internal/app/storage/...、
      # sync-property・sync-property-stores = replicate の 2 つの property テスト（seed が多く重い）、
      # others = 残りのパッケージすべて（replicate の残りと server を含む。新しいパッケージは何もしなくても
      # usecase・cli・storage の下か others に入る）。
      # 復元したキャッシュには main のテスト結果も入っているので、-count=1 で毎回テストを実行する。
```

- [ ] **Step 2: case を書き換える**

```bash
          case "$SHARD" in
            usecase) args=("$mod/internal/app/usecase/...") ;;
            cli) args=("$mod/internal/app/cli/...") ;;
            storage) args=("$mod/internal/app/storage/...") ;;
            sync-property) args=(-run '^TestPropertyConverges$' "$mod/internal/app/remote/replicate") ;;
            sync-property-stores) args=(-run '^TestPropertyConvergesStores$' "$mod/internal/app/remote/replicate") ;;
            others) args=(-skip "$prop" $(go list ./... | grep -vE -e "^$mod/internal/app/(usecase|cli)(/|\$)" -e "^$mod/internal/app/storage/")) ;;
            *) echo "unknown shard $SHARD"; exit 1 ;;
          esac
```

（grep は ERE（`-E`）にする。BRE の `\|` は GNU の拡張で、macOS の runner の BSD grep では使えないことがある。`-skip "$prop"` は others の全パッケージにかかるが、`TestPropertyConverges(Stores)?` は replicate にしかない。旧 `sync-server` の分は others に入る）

- [ ] **Step 3: 分け方が全パッケージを 1 回ずつ覆うことを確かめる**

```bash
mod=$(go list -m)
all=$(go list ./... | sort)
u=$(go list "$mod/internal/app/usecase/..."); c=$(go list "$mod/internal/app/cli/..."); s=$(go list "$mod/internal/app/storage/...")
o=$(go list ./... | grep -vE -e "^$mod/internal/app/(usecase|cli)(/|\$)" -e "^$mod/internal/app/storage/")
diff <(printf '%s\n' $u $c $s $o | sort) <(printf '%s\n' $all) && echo cover-ok
printf '%s\n' $u $c $s $o | sort | uniq -d   # 何も出ない
printf '%s\n' $o | grep -c replicate          # 1（replicate は others に入る）
```

Expected: `cover-ok`、重複なし、`1`

- [ ] **Step 4: actionlint があれば当てる**

Run: `command -v actionlint && actionlint .github/workflows/ci-backend.yaml || echo "actionlint なし（CI で確かめる）"`

- [ ] **Step 5: コミット**

```bash
git add .github/workflows/ci-backend.yaml
git commit -m "ci: shard を usecase・cli・storage の配下で分け直す"
```

### Task 9: 親設計を書き直し、5a の全体を確かめる

この Task はコントローラ（Claude Docs の connector を持つ側）が行う。実装のサブエージェントには渡さない。

**Files:**
- 親設計（Claude Docs、https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「パッケージ構成」節のコードブロック（block id `mz4fg2vqpzh.13921`）と、その直後に足す段落

- [ ] **Step 1: 全体を確かめる（5a で 1 回だけ）**

Run: `go build ./... && go vet ./... && go vet -tags e2e ./internal/app/cli && make lint && make fmt-check && make tidy-check && mkdir -p .tmp && TMPDIR=$PWD/.tmp go test -race ./...`
Expected: すべて通る。`find internal -name 'final_*_test.go'` が空。`go list -deps -test ./internal/app/storage/... | grep -c 'internal/app/usecase'` が 0（usecasetest を除く層の向き）。所要は約 3 分（-race 全テスト）。落ちたら失敗の出力・コマンド・-race の有無を報告に残す

- [ ] **Step 2: 親設計のコードブロックを書き直す**

Claude Docs の connector で、親設計（container `{"kind":"project","id":"3db8fbf2-a7ff-4e6f-83b2-69ed612917fd"}`）を `read` し直し（他の人が直していないか）、block `mz4fg2vqpzh.13921` のコードブロックを次の中身に `update` で置き換える。

```
cmd/hedgehug/            cli.Main を呼ぶだけ
cmd/fakegh/              GitHub API のフェイクのサーバー（開発環境と E2E が使う）
internal/lib/            hedgehug を知らない部品
  osfs/                  OS に触る処理（改名、削除、読み書き、ディレクトリ作成、ロック、プロセス起動、パスの比較）
  plural/                数と名詞の英語の書き方（"1 task"、"2 tasks"）
  gitrepo/               go-git によるコミット、履歴、作業ツリーの状態と検査、fetch / push / clone
  giturl/                remote URL のホスト・owner・リポジトリの読み取り、repo ID
  github/                GraphQL クライアント、リンク、自動化
  fakegh/                GitHub API のフェイク（cmd/fakegh とテストが使う）
  testenv/               テストのプロセスの一時ディレクトリ、条件の待ち合わせ、バイナリの組み立て
internal/app/domain/     hedgehug の型と規則
  model/                 タスク、コメント、ULID、rank
  schema/                スキーマの読み込みと検証、マイグレーション
  merge/                 フィールド単位3-wayマージ、不変条件の修復
internal/app/storage/    Store とローカルのファイル
  store/                 ファイルの読み書き、ロック、1操作1コミット、force push 復旧の途中残りの検出
  index/                 インデックス、ID の解決、参照と循環の検査
  history/               コミットごとの変更ファイル・作者・trailer の索引
  inbox/                 通知の作成、保存、既読
  query/                 クエリ言語のパーサと評価
  config/                ローカル設定、設定と Store の場所、コミットの作者
  view/                  保存済みビューの形式、読み書き、検証
  mount/                 マウント中の Store の集合の開け閉め
  current/               context.json（hh use のプロジェクト、リポジトリごとの最後のプロジェクト）
internal/app/remote/     リモートとの通信
  replicate/             同期の手順（fetch・マージ・修復・push、リトライ）、バックグラウンド同期、force push からの復旧
  auth/                  トークン取得、Device Flow
internal/app/usecase/    CLI とサーバーが共通に呼ぶ操作（表示を持たない）。直下は土台: Session、エラー、返す型、ID の解決、行き先の判断
  tasks/ comments/ browse/ views/ schemas/ workspaces/
  stores/ replication/ notifications/ accounts/ maintenance/ watch/
                         領域ごとの操作（maintenance は doctor と schema check、watch は GUI のための変化の監視）
internal/app/cli/        CLI のコマンド定義と表示
  prompt/                確認と選択の部品
  scenario/              組み立てたバイナリを動かす通しのテスト
internal/app/server/     huma v2 の REST API、SSE、同期ループ、埋め込みの配信
web/core/                API クライアント、SSE の適用、タスクのキャッシュ（フレームワーク非依存の TypeScript）
web/app/                 Svelte 5 のコンポーネントとビュー
web/e2e/                 Playwright の E2E
web/embed.go             ビルド結果（web/dist。gitignore）を go:embed
api/openapi.yaml         サーバーから生成した OpenAPI 3.1（コミットする）
```

コードブロックの直後に段落を 1 つ足す:

> 層は lib → domain → storage → remote → usecase → cli・server の向きで、各層は自分と下の層と lib だけを import します。usecase の領域どうしは import せず、共有するものは土台に置きます。cli と server は usecase だけを通して動き、storage・remote・gitrepo・github を直接使いません。どれも golangci-lint の depguard で強制します（構成見直しの spec: docs/superpowers/specs/2026-11-04-codebase-restructure-design.md）。

書いた後に `read` で、コードブロックが上と同じになっていることと、段落の位置（コードブロックの直後、「技術選定」の見出しの前）を確かめる。

- [ ] **Step 3: memory と PR**

memory の `codebase-restructure.md` に 5a の完了（コミット、テストの件数、CI の shard の時間）を書く。push して PR を作る（本文に「この回で決めたこと」1〜5 と、spec 5 節からのずれ（決定 16）を書く）。マージ後、CI の Package times で 6 つの shard の時間を見て、最も遅い shard と最も速い shard を memory に書く（次に shard を均すときの材料）。

---

# PR 5b: テストのための差し込み口（spec 決定 18）と、壊れた Store のエラー（決定 19）

5a のマージ後の main から切る（ブランチ `restructure5b-seams`）。5a でテストのファイルが移っているので、下の Task のパスは 5a の後の場所で書いてある（例: `TestSessionConcurrentOperations` は `usecase/session_test.go`、`TestResultsCarryTheCommitAndAFreshView` は `usecase/write_view_test.go`）。

**下の Task のコードの差分は、main 4a9cf71 のコピーの上で 1 Task ずつ試して取ったもの。** 前の Task が同じファイル（とくに `storage/store/lock.go`・`store.go`・`move.go`、`lib/gitrepo/lock.go`、`replicate/sync.go`、`usecase/session.go`・`usecasetest/session.go`）を変えているので、差分の行番号と前後の行がずれることがある。差分は「何をどう変えるか」として読み、その時点のファイルに当てる。食い違って当てられないとき（消すはずの行がない等）は、止めて報告する。

## 差し込み口の一覧（main 4a9cf71 で数えた 44 項目と、その扱い）

| 分類 | 扱い | 項目（Task） |
| --- | --- | --- |
| A | 本番で誰も呼ばない関数を消す・テストへ移す | `store.acquire`（A1）、`Session.ForgetMounts`・`ForgetWriter`（A2）、`inbox.Box.Import`（A3）、`replicate.FinishLeftover`（A4）、`stores.PrepareMoveStore`（A5） |
| B | 普通の引数・設定にする | `store.afterLockMiss`・`Store.lockWait`（B1）、`config.goos`（B2）、`gitrepo.packCap`（B3）、`auth.lockWait`（B4）、`replicate.retryWait`・`repackAt`・`packsAt`（B5）、`maintenance.lookPath`（B11）、`replicate.spawn`・`SetSpawn`・`usecasetest.StubSpawn`（B12）、`cli env.uiContext`（B13） |
| C | 本物の手段で確かめる形にテストを書き直して外す | `store.lockPoll`・`gitrepo.indexLockPoll`・`indexLockWait`（C1）、`Store.commit`・`newStoreHook`（C2）、`store.afterLock`（C3）、`store.openStore`（C4）、`index.readHook`・`listHook`（C5）、`gitrepo.beforePush`（C11）、`BeforeUpdate` の使い道のうち chmod 版（C12）・「呼ばれたら失敗」版（C13）・`breakConfig`（C14）、`replicate.onFullRepair`（F3。F1・F2 の後でなければ外から確かめられない） |
| D | 一瞬への割り込み・外から見えない内部の確認なので残す | `Session.BeforeUpdate`・`cli env.beforeUpdate`（割り込みの用途: `cli/meanwhile_test.go`、`usecasetest.HookConfigWrite` の 7 件、`stores/movestore_test.go`・`movestore_gui_test.go` の remove 版、`stores/storeops_test.go` の「書く瞬間にロックを持っている」）、`replicate.beforeRename`・`beforeCommit`・`Options.BeforePush`、`gitrepo.afterClone`・`afterPackWrite`、`store.onLockWait`、`server/loop` の `syncFn`・`setSyncFn`・`wakes`・`deciding`・`decided`・`parked`、`watch.Watcher.openHistory`、`index.Index.openHistory`、`gitrepo.Live`、`mount.Mounted.IndexOpened` |
| 対象外 | 差し込み口ではない | `Tx.PutSprint`・`PutWorklog`・`PutRelease`・`PutTemplate`（スプリントなどを書くコマンドがまだないだけで、store の書き込み API と検証そのもの。2026-11-05 利用者が残すと決定） |

## 5b で決めたこと（2026-11-05。1・2・3 は利用者が決定、4〜9 は plan で決めた）

1. **壊れた Store のエラーを握りつぶさない（spec 決定 19）。** go-git の `TreeWalker` は下の tree が読めないと io.EOF を返すので、`gitrepo.Files` が一覧を黙って切っていた。go-git の `Tree.File` は blob が欠けても `ErrFileNotFound` を返すので、`gitrepo.ReadFile` が「ファイルなし」を返していた。replicate の `snapshot.Read` は読み取りのエラーをすべて「ファイルなし」にしていた。直すと、object が欠けた Store では、一覧・読み取り・同期がエラーで止まる（挙動の変化。壊れていない Store は変わらない）。F1〜F2
2. **`Tx.Put*` の 4 つは残す**（上の表の「対象外」）
3. **`StoreRemove` の「書く瞬間にロックを持っている」の検査は `BeforeUpdate` のまま残す**（D。外からロックを握る形にすると検出できる範囲が狭まる）
4. store の待ち時間は可変長の関数オプション `store.WithLockWait(d)`（`Open`・`Create` の呼び出し側を変えずに済む）
5. gitrepo の pack の上限は、`SmallPacks()`・`Consolidate()` の署名を変えず、上限を取る `SmallPacksBelow(cap)`・`ConsolidateBelow(cap)` を足す（前の 2 つは `PackCap` で後の 2 つを呼ぶ）
6. replicate の `Options` の待ち時間・閾値は、ゼロ値が既定（本番の呼び出し側は何も渡さない）
7. `usecasetest.(*Session).Renew()` は Session の公開のフィールドを写す。写し漏れを防ぐため、`TestForStoreCopiesEveryExportedField` と同じ形の反射のテストを足す（A2）。B12 で Session にフィールド（`Spawn`）が増えるので、そのテストが Renew の写し忘れを捕まえる
8. `hh ui` の止め方は、Main が「Ctrl-C と SIGTERM を待つ context を作る関数」を env に入れる形（Main の先頭で signal を握ると、ほかのコマンドの Ctrl-C の挙動が変わるため）
9. `osfs.LookPath(name, path)` は unix の探し方だけ（Windows は spec 決定 8 で対象外）

Task の順: A1〜A5 → C1〜C5（store・gitrepo・index）→ B1〜B5 → B11〜B13 → C11〜C14 → F1〜F3 → 仕上げ。C4 で `openStore` が消えるので、B1 の「`openStore` の型を固定する」注は C4 の後では当てはまらない（飛ばす）。

### Task A1: store の `acquire` をテストのファイルへ移す

外す差し込み口: `internal/app/storage/store/lock.go` の `acquire`（本番の呼び出しは `acquireStore` 側で、`acquire` を呼ぶのはテストだけだった）。
使っていたテスト: `lock_test.go`（TestLockExcludes ほか 6 か所）、`tx_test.go` の TestUpdateLockedElsewhere、`move_test.go` の TestLockInOrder・TestLockInOrderFailureReleasesWhatItTook。呼び出しは変えない（同じ package なので、関数をテストのファイルへ移すだけで済む）。

**Files:**
- Modify: `internal/app/storage/store/lock.go`
- Modify（テスト）: `internal/app/storage/store/lock_test.go`

**Interfaces:**
- Produces: なし（`acquire(path string, wait time.Duration) (*osfs.Lock, error)` は `lock_test.go` の非公開の補助になる。本番のコードには残らない）

- [ ] **Step 1: 本番で呼ばれていないことを確かめる**

Run: `grep -rn 'acquire(' internal --include='*.go' | grep -v 'func acquire'`
Expected: `lock_test.go`・`tx_test.go`・`move_test.go` の行だけ（`acquireStore(` は別の関数で、一致しない）。

- [ ] **Step 2: `lock.go` から `acquire` を消す**

```diff
--- a/internal/app/storage/store/lock.go
+++ b/internal/app/storage/store/lock.go
@@
 var afterLockMiss = func() {}
 
-func acquire(path string, wait time.Duration) (*osfs.Lock, error) {
-	path = filepath.Clean(path)
-	if err := osfs.MkdirAll(filepath.Dir(path), 0o750); err != nil {
-		return nil, err
-	}
-	return lockFile(path, wait)
-}
-
 // lockFile takes the lock on the file at path, made when missing, in a
```

`lock.go` は `filepath`・`osfs` をほかでも使うので import は変えない（`go vet` で確かめる）。

- [ ] **Step 3: `lock_test.go` に置く**

import に `"github.com/kukv/hedgehug/internal/lib/osfs"` を足し（標準ライブラリの import 群の後ろに空行を 1 つ挟む）、`TestLockExcludes` の前に次を置く。テストのファイルなので `osfs.MkdirAll` でなく `os.MkdirAll` を使う（`os` は既に import 済み）。

```go
// acquire takes the lock file at path as another process does: it makes the
// directories above it and waits up to wait.
func acquire(path string, wait time.Duration) (*osfs.Lock, error) {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return lockFile(path, wait)
}
```

（この時点の `lockFile` はまだ 2 引数。B1 で 3 引数にする。）

- [ ] **Step 4: ビルドとテストを回す**

Run: `go vet ./internal/app/storage/store && go test -count=1 ./internal/app/storage/store -run 'TestLock|TestUpdateLockedElsewhere|TestAcquireStore'`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/storage/store/lock.go internal/app/storage/store/lock_test.go
git commit -m "refactor(store): 他プロセスのロックを作る acquire をテストのファイルへ移す"
```

---

### Task A2: `Session.ForgetMounts`・`ForgetWriter` を消し、usecasetest が新しい Session を作る

外す差し込み口: `usecase/session.go` の `Session.ForgetMounts`・`Session.ForgetWriter`。
使っていたもの:
- `usecasetest/session.go` の 4 つの補助（`DropDefaultAccount`・`NameAccountOfStores`・`DropAccounts`・`AsAccount`）が config.yaml を書いた後に `ts.ForgetWriter()` を呼んでいた
- テスト: `TestSessionConcurrentOperations`、`TestResultsCarryTheCommitAndAFreshView`、`comments` の `TestPrepareErrorsAreReturnedBare`、`tasks/task_test.go` の TestWatchAddsAndRemovesMe・TestWriterHost、`stores/use_test.go` の TestUseShowWarnsOfAProjectThatIsGone（`ForgetMounts`）

方針: 「config を書き換えたら新しい Session に取り替える」を `usecasetest.(*Session).Renew()` にする。補助が自分で `Renew` を呼ぶので、テストの側の `ForgetWriter` の行は要らなくなる。`UseAccount` も config を書き換えるので `Renew` を呼ぶ（これで 3 か所の `UseAccount(); ForgetWriter()` の 2 行目が消える）。

**5a で移ったあとのパスで書く:**
- `TestSessionConcurrentOperations` は `internal/app/usecase/session_test.go`
- `TestResultsCarryTheCommitAndAFreshView` は `internal/app/usecase/write_view_test.go`（package usecase_test のまま）
実装者は、5a が済んだ main で、この 2 つのテスト名を `grep -rn 'ForgetWriter' internal` で探して直す。

**Files:**
- Modify: `internal/app/usecase/session.go`
- Modify: `internal/app/usecase/usecasetest/session.go`
- Modify（テスト）: `internal/app/usecase/session_test.go`（TestSessionConcurrentOperations）、`internal/app/usecase/write_view_test.go`（TestResultsCarryTheCommitAndAFreshView）、`internal/app/usecase/comments/comment_prepare_test.go`、`internal/app/usecase/tasks/task_test.go`、`internal/app/usecase/stores/use_test.go`

**Interfaces:**
- Produces: `func (ts *usecasetest.Session) Renew()` — `ts.Session` を、公開のフィールドを引き継いだ新しい `*usecase.Session` に取り替える。writer・トークン・mount のキャッシュは空になる（新しい実行と同じ）。

- [ ] **Step 1: 呼び出しの一覧を取る**

Run: `grep -rn 'ForgetWriter\|ForgetMounts' internal`
Expected: `usecasetest/session.go` の 4 か所、`tasks/task_test.go` の 2 か所、`comment_prepare_test.go`、`stores/use_test.go`、TestSessionConcurrentOperations の 1 か所、TestResultsCarryTheCommitAndAFreshView の 1 か所と、定義 2 つ。これ以外に出たら止めて報告する。

- [ ] **Step 2: `session.go` から 2 つの関数を消す**

`internal/app/usecase/session.go`:

```diff
--- a/internal/app/usecase/session.go
+++ b/internal/app/usecase/session.go
@@ -120,10 +120,6 @@
 	}
 }
 
-// ForgetMounts lets go of the stores the session mounted, as a new run of
-// the command starts without them.
-func (ss *Session) ForgetMounts() { ss.mounts = nil }
-
 // mountCache is the cache of ss, made on first use.
 func (ss *Session) mountCache() *mountCache {
 	ss.mountMu.Lock()
@@ -757,11 +753,3 @@
 func Unpushed(s *store.Store) (int, error) {
 	return replicate.Unpushed(s.Repo())
 }
-
-// ForgetWriter forgets the writer the session cached, so that it is read from
-// config.yaml again after a test changed the accounts.
-func (ss *Session) ForgetWriter() {
-	ss.writerMu.Lock()
-	defer ss.writerMu.Unlock()
-	ss.writerDone = false
-}
```

- [ ] **Step 3: `usecasetest/session.go` に `Renew` を足し、補助を直す**

`Renew` は `usecase.Session` の公開のフィールドを 1 つずつ写す（`*o` のコピーは mutex を含むので `go vet` が拒む）。Session の公開のフィールドを増やしたときは、ここにも足す（`ForStore` の写し方と同じ一覧）。

`internal/app/usecase/usecasetest/session.go`:

```diff
--- a/internal/app/usecase/usecasetest/session.go
+++ b/internal/app/usecase/usecasetest/session.go
@@ -86,6 +86,30 @@
 	}
 }
 
+// Renew replaces the session with a new one that has the same outside world
+// (Getenv, Now, the hooks and the rest of the exported fields), as a new run
+// of the command starts: the writer, the tokens and the stores mounted are
+// read again. The helpers that change config.yaml call it; call it after a
+// change made by hand, or for a session that has not cached anything.
+func (ts *Session) Renew() {
+	o := ts.Session
+	ts.Session = &usecase.Session{
+		Getenv:         o.Getenv,
+		Now:            o.Now,
+		Loc:            o.Loc,
+		StoreName:      o.StoreName,
+		Warn:           o.Warn,
+		AfterWrite:     o.AfterWrite,
+		BackgroundSync: o.BackgroundSync,
+		BeforeUpdate:   o.BeforeUpdate,
+		BeforeSwap:     o.BeforeSwap,
+		Keyring:        o.Keyring,
+		HTTPClient:     o.HTTPClient,
+		Run:            o.Run,
+		Progress:       o.Progress,
+	}
+}
+
 // DropDefaultAccount removes DefaultAccountYAML, and the stores' reference to
 // it, for the tests that have no account or set up their own.
 func (ts *Session) DropDefaultAccount() {
@@ -102,7 +126,7 @@
 	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
 		ts.T.Fatal(err)
 	}
-	ts.ForgetWriter()
+	ts.Renew()
 }
 
 // NameAccountOfStores gives the stores that have no account the account name,
@@ -143,7 +167,7 @@
 	if err := os.WriteFile(p, out, 0o600); err != nil {
 		ts.T.Fatal(err)
 	}
-	ts.ForgetWriter()
+	ts.Renew()
 }
 
 // FirstAccountName is the name: of the first account in the accounts: text.
@@ -188,7 +212,7 @@
 	if err := os.WriteFile(p, out, 0o600); err != nil {
 		ts.T.Fatal(err)
 	}
-	ts.ForgetWriter()
+	ts.Renew()
 }
 
 // BeforeCommand is replication.BeforeCommand for the session.
@@ -782,7 +806,7 @@
 	if err := os.WriteFile(p.ConfigFile, []byte(text), 0o600); err != nil {
 		ts.T.Fatal(err)
 	}
-	ts.ForgetWriter()
+	ts.Renew()
 }
 
 func (ts *Session) SetMembers(m replicate.Members) {
@@ -894,6 +918,7 @@
 func (ts *Session) UseAccount() {
 	ts.T.Helper()
 	ts.AppendConfig("accounts:\n  - name: alice\n    host: github.com\n    login: alice\n")
+	ts.Renew()
 }
 
 // WantErr checks that err has the kind and the code.
```

- [ ] **Step 4: テストを直す**

`tasks/task_test.go`: `ts.UseAccount()` の後ろの `ts.ForgetWriter()` を 2 か所とも消す（1 つ目は行末にコメントがあるので、`ts.UseAccount() // a new session: the writer is read again` の 1 行にする）。

`internal/app/usecase/tasks/task_test.go`:

```diff
--- a/internal/app/usecase/tasks/task_test.go
+++ b/internal/app/usecase/tasks/task_test.go
@@ -218,8 +218,7 @@
 		t.Fatalf("Watch without an account: kind %v, err %v; want KindConflict account.none", usecase.KindOf(err), err)
 	}
 
-	ts.UseAccount()
-	ts.ForgetWriter() // the writer is read once per session; read it again
+	ts.UseAccount() // a new session: the writer is read again
 	watchers := func() []string {
 		t.Helper()
 		_, ix, err := ts.Open()
@@ -410,7 +409,6 @@
 		t.Errorf("WriterHost without an account = %q, %v", h, err)
 	}
 	ts.UseAccount()
-	ts.ForgetWriter()
 	if h, err := ts.WriterHost(); err != nil || h != "github.com" {
 		t.Errorf("WriterHost = %q, %v; want github.com", h, err)
 	}
```

`stores/use_test.go`: `ForgetMounts` を `Renew` に。

`internal/app/usecase/stores/use_test.go`:

```diff
--- a/internal/app/usecase/stores/use_test.go
+++ b/internal/app/usecase/stores/use_test.go
@@ -161,7 +161,7 @@
 				t.Fatal(err)
 			}
 			gone(t, ts, id)
-			ts.ForgetMounts() // the stores are opened again, as a new run does
+			ts.Renew() // the stores are opened again, as a new run does
 			r, err := stores.UseShow(ts.Session)
 			if err != nil || r.Project != nil || r.Store != "" {
 				t.Errorf("UseShow = %+v, %v; want none", r, err)
```

`TestSessionConcurrentOperations`（`usecase/session_test.go`）: 1 つ目の goroutine が writer を読み直す、という元の意図を `Renew` で書く。

```diff
-	ts.ForgetWriter() // the first of the goroutines reads the writer again
+	ts.Renew() // the first of the goroutines reads the writer again
```

`TestResultsCarryTheCommitAndAFreshView`（`usecase/write_view_test.go`）: `UseAccount` が `Renew` するので、次の行を消す。

```diff
 	ts.UseAccount()
-	ts.ForgetWriter()
 	if _, err := tasks.Set(ts.Session, "ME-1", []usecase.Assign{{Key: "title", Op: "=", Value: "z"}}); err != nil {
```

`comments/comment_prepare_test.go` の `TestPrepareErrorsAreReturnedBare` は `Renew` では済まない。準備の部品（`cs`・`es`・`ces`）が作った時の Session を持ち続け、`Prepare…` はその Session の writer を読むので、`ts.Session` を取り替えても効かない（試したところ、`PrepareComment = <nil>` で落ちた。記録は末尾）。そこで「writer をまだ読んでいない Session」を `ForStore`（本番の公開 API）で作り、準備の部品はそれで作る。writer を壊れた config から読ませるのも、その Session に対して行う。確かめる内容は元と同じ（壊れた config の読み込みエラーが、そのまま返る・失敗は覚えられない）。

`internal/app/usecase/comments/comment_prepare_test.go`:

```diff
--- a/internal/app/usecase/comments/comment_prepare_test.go
+++ b/internal/app/usecase/comments/comment_prepare_test.go
@@ -43,19 +43,22 @@
 	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
 		t.Fatal(err)
 	}
-	cs, err := comments.BeginComment(ts.Session, "ME-1")
+	c, err := comments.CommentAdd(ts.Session, "ME-1", "one")
 	if err != nil {
 		t.Fatal(err)
 	}
-	c, err := comments.CommentAdd(ts.Session, "ME-1", "one")
+	// ss has read no writer yet, as ts.Session has: the prepared sessions
+	// below keep it, and read the writer when they are asked to prepare.
+	ss := ts.ForStore(ts.StoreName)
+	cs, err := comments.BeginComment(ss, "ME-1")
 	if err != nil {
 		t.Fatal(err)
 	}
-	es, err := tasks.BeginEdit(ts.Session, "ME-1")
+	es, err := tasks.BeginEdit(ss, "ME-1")
 	if err != nil {
 		t.Fatal(err)
 	}
-	ces, err := comments.OpenComment(ts.Session, c.Ref.Comment.String())
+	ces, err := comments.OpenComment(ss, c.Ref.Comment.String())
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -72,8 +75,7 @@
 	if err := os.WriteFile(p.ConfigFile, []byte("stores: [\n"), 0o600); err != nil {
 		t.Fatal(err)
 	}
-	ts.ForgetWriter()
-	_, _, boom := ts.Writer()
+	_, _, boom := ss.Writer()
 	if boom == nil {
 		t.Fatal("Writer read a config that does not parse")
 	}
@@ -92,7 +94,7 @@
 	if err := os.WriteFile(p.ConfigFile, good, 0o600); err != nil {
 		t.Fatal(err)
 	}
-	if _, _, err := ts.Writer(); err != nil {
+	if _, _, err := ss.Writer(); err != nil {
 		t.Errorf("Writer after the config is back = %v; want the error not to be cached", err)
 	}
 	// Nothing to read for an empty body, a body that stays, or a broken task.
```

- [ ] **Step 4b: `Renew` が公開のフィールドをすべて写すことをテストで固定する**

`Renew` は Session の公開のフィールドを 1 つずつ写すので、フィールドが増えたときの写し忘れを、`TestForStoreCopiesEveryExportedField` と同じ反射で捕まえる。`internal/app/usecase/session_test.go`（5a で `TestForStoreCopiesEveryExportedField` が移った先。`fakeKeyring` も同じパッケージのテストにある）に足す:

```go
func TestRenewCopiesEveryExportedField(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	v := reflect.ValueOf(ts.Session).Elem()
	for i := range v.NumField() {
		f, sf := v.Field(i), v.Type().Field(i)
		if !sf.IsExported() || !f.IsZero() {
			continue // what New set stays as it is
		}
		switch f.Kind() {
		case reflect.Func:
			f.Set(reflect.MakeFunc(f.Type(), func([]reflect.Value) []reflect.Value {
				out := make([]reflect.Value, f.Type().NumOut())
				for j := range out {
					out[j] = reflect.Zero(f.Type().Out(j))
				}
				return out
			}))
		case reflect.String:
			f.SetString("x")
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Pointer:
			f.Set(reflect.New(f.Type().Elem()))
		case reflect.Interface:
			f.Set(reflect.ValueOf(&fakeKeyring{}))
		default:
			t.Fatalf("Session.%s is a %s; teach this test to set it", sf.Name, f.Kind())
		}
	}
	old := ts.Session
	ts.Renew()
	if ts.Session == old {
		t.Fatal("Renew kept the old session")
	}
	got := reflect.ValueOf(ts.Session).Elem()
	for i := range got.NumField() {
		sf := got.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		if got.Field(i).IsZero() {
			t.Errorf("Renew does not copy Session.%s", sf.Name)
		}
		if got.Field(i).Kind() == reflect.Pointer && got.Field(i).Pointer() != v.Field(i).Pointer() {
			t.Errorf("Renew does not share Session.%s", sf.Name)
		}
	}
}
```

検出できることを確かめる: Step 3 の `Renew` の中の 1 行（例: `Warn: o.Warn,`）を消して `go test -count=1 ./internal/app/usecase -run TestRenewCopiesEveryExportedField` を回し、`Renew does not copy Session.Warn` で落ちることを見て、戻す。

- [ ] **Step 5: ビルドとテストを回す**

Run: `go build ./... && go vet ./...`
Run: `go test -count=1 ./internal/app/usecase/... ./internal/app/server/...`
Expected: PASS（`server` のテストも `UseAccount`・`DropAccounts` などを使うので回す）

- [ ] **Step 6: コミット**

```bash
git add internal/app/usecase
git commit -m "refactor(usecase): テスト専用の ForgetWriter・ForgetMounts を消し、usecasetest が新しい Session に取り替える"
```

---

### Task A3: `inbox.Box.Import` をテストのファイルへ移す

外す差し込み口: `internal/app/storage/inbox/inbox.go` の `Box.Import`（本番は `Collect` の中で `importRecords` を直接使う）。
使っていたテスト: `abort_test.go` の TestImportDropsAbortedMerge ほか 4 か所、`collect_test.go` の 2 か所。呼び出しは変えない（同じ package のテストのファイルに、同じ名前のメソッドを定義する）。

**Files:**
- Modify: `internal/app/storage/inbox/inbox.go`
- Modify（テスト）: `internal/app/storage/inbox/collect_test.go`

- [ ] **Step 1: 本番で呼ばれていないことを確かめる**

Run: `grep -rn '\.Import(' internal --include='*.go' | grep -v _test.go`
Expected: 出力なし

- [ ] **Step 2: 本体の `Import` を消し、説明を `importRecords` に移す**

`internal/app/storage/inbox/inbox.go`:

```diff
--- a/internal/app/storage/inbox/inbox.go
+++ b/internal/app/storage/inbox/inbox.go
@@ -327,17 +327,11 @@
 	return Item{}, false, nil
 }
 
-// Import adds the merge driver's records in .hedgehug/local/overwritten/
+// importRecords adds the merge driver's records in .hedgehug/local/overwritten/
 // and removes the files. The records whose author is not one of me are
 // removed too: the driver writes every value that lost, whoever wrote it.
-// A record that does not decode is left in place, with a warning.
-func (b *Box) Import(me []string) error {
-	return b.locked(func() error {
-		_, err := b.importRecords(me)
-		return err
-	})
-}
-
+// A record that does not decode is left in place, with a warning. The caller
+// holds the inbox lock.
 func (b *Box) importRecords(me []string) ([]Item, error) {
 	names, err := b.s.LocalFiles(recordsDir)
 	if err != nil {
```

- [ ] **Step 3: テストのファイルに包みを置く**

`internal/app/storage/inbox/collect_test.go`:

```diff
--- a/internal/app/storage/inbox/collect_test.go
+++ b/internal/app/storage/inbox/collect_test.go
@@ -32,6 +32,15 @@
 	return store.Op{Name: name, Task: task, Author: gitrepo.Signature{Name: login, Email: login + "@x"}, Account: c.acct}
 }
 
+// Import adds the merge driver's records to the inbox under the inbox lock,
+// as Collect does, and drops what it imported.
+func (b *Box) Import(me []string) error {
+	return b.locked(func() error {
+		_, err := b.importRecords(me)
+		return err
+	})
+}
+
 // twoClones makes a store written by github.com/a, pushed to a bare remote,
 // and a clone of it written by github.com/b.
 func twoClones(t *testing.T) (*clone, *clone) {
```

- [ ] **Step 4: テストを回す**

Run: `go vet ./internal/app/storage/inbox && go test -count=1 ./internal/app/storage/inbox`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/storage/inbox
git commit -m "refactor(inbox): テストだけが使う Box.Import をテストのファイルへ移す"
```

---

### Task A4: `replicate.FinishLeftover` を消す

外す差し込み口: `internal/app/remote/replicate/rebuild.go` の `FinishLeftover`（`FinishLeftoverKept` の戻り値の `kept` を捨てるだけ。本番は usecase の `finishLeftover` が `FinishLeftoverKept` を呼ぶ）。
使っていたテスト（9 か所）: `rebuild_keep_test.go`（2）・`rebuild_stale_test.go`（2）・`rebuild_test.go`（5）。`FinishLeftoverKept` を呼び、`kept` は `_` で捨てる。テスト名と確かめる内容は変えない。

**Files:**
- Modify: `internal/app/remote/replicate/rebuild.go`
- Modify: `internal/app/usecase/sync_shared.go`（コメントの名前だけ）
- Modify（テスト）: `internal/app/remote/replicate/rebuild_keep_test.go`、`rebuild_stale_test.go`、`rebuild_test.go`

- [ ] **Step 1: 本番で呼ばれていないことを確かめる**

Run: `grep -rn 'FinishLeftover(' internal --include='*.go' | grep -v _test.go`
Expected: `rebuild.go` の定義だけ。

- [ ] **Step 2: 本体を直す**

`FinishLeftover` を消し、説明を `FinishLeftoverKept` に統合する。コメントに出る旧名も直す。

`internal/app/remote/replicate/rebuild.go`:

```diff
--- a/internal/app/remote/replicate/rebuild.go
+++ b/internal/app/remote/replicate/rebuild.go
@@ -478,7 +478,7 @@
 // for a commit made in between, and puts the old store back if there is one);
 // then it brings over what changed in
 // .hedgehug/local since the copy (any other failure to look at the old
-// store leaves b.Dir, for FinishLeftover). A rename that fails changes nothing
+// store leaves b.Dir, for FinishLeftoverKept). A rename that fails changes nothing
 // or puts the old store back; only when that fails too is a leftover
 // (FindLeftover) all there is. The removal of the old store may fail
 // without failing Swap; it is a leftover then. Swap leaves the old store at
@@ -523,12 +523,12 @@
 	if err := checkUnchanged(before, b); err != nil {
 		var merr error
 		if errors.Is(err, ErrRebuildStale) {
-			// If the old store cannot be put back, FinishLeftover must not
+			// If the old store cannot be put back, FinishLeftoverKept must not
 			// put the rebuild in its place: it would drop that commit.
 			merr = osfs.WriteFile(filepath.Join(b.Dir, staleMarker), []byte("the old store has a commit this rebuild lacks\n"), 0o600)
 		}
 		if rerr := osfs.Rename(before, dir); rerr != nil {
-			// merr says that FinishLeftover may not know the rebuild is stale.
+			// merr says that FinishLeftoverKept may not know the rebuild is stale.
 			return errors.Join(err, rerr, merr)
 		}
 		removeKeepMarker(dir)
@@ -598,17 +598,12 @@
 	return nil
 }
 
-// FinishLeftover completes a swap that stopped, or removes what a rebuild
+// FinishLeftoverKept completes a swap that stopped, or removes what a rebuild
 // left. With no store at dir, the rebuilt one takes its place; with only an
-// old one left, it is put back. Then what remains beside the store goes.
-func FinishLeftover(dir string, l store.Leftover) error {
-	_, err := FinishLeftoverKept(dir, l)
-	return err
-}
-
-// FinishLeftoverKept is FinishLeftover that also returns where it kept an
-// old store that carries keepMarker: that one is renamed to keptPrefix, not
-// removed; when it cannot be, it stays and the error names it.
+// old one left, it is put back. Then what remains beside the store goes. It
+// returns where it kept an old store that carries keepMarker: that one is
+// renamed to keptPrefix, not removed; when it cannot be, it stays and the
+// error names it.
 func FinishLeftoverKept(dir string, l store.Leftover) (kept []string, err error) {
 	keptPaths := &kept
 	err = finishLeftover(dir, l, keptPaths)
```

`internal/app/usecase/sync_shared.go`:

```diff
--- a/internal/app/usecase/sync_shared.go
+++ b/internal/app/usecase/sync_shared.go
@@ -169,7 +169,7 @@
 	return string(b)
 }
 
-// finishLeftover is replicate.FinishLeftover, telling where an old store was kept.
+// finishLeftover is replicate.FinishLeftoverKept, telling where an old store was kept.
 func (ss *Session) finishLeftover(e config.StoreEntry, l store.Leftover) error {
 	kept, err := replicate.FinishLeftoverKept(e.Path, l)
 	for _, p := range kept {
```

- [ ] **Step 3: テストの 9 か所を直す**

`err = FinishLeftover(dir, l)` は `_, err = FinishLeftoverKept(dir, l)`、`if err := FinishLeftover(dir, l);` は `if _, err := FinishLeftoverKept(dir, l);`。次のコマンドで一括で直せる。

Run:
```bash
sed -i 's/err = FinishLeftover(dir, l)/_, err = FinishLeftoverKept(dir, l)/; s/if err := FinishLeftover(dir, l);/if _, err := FinishLeftoverKept(dir, l);/' internal/app/remote/replicate/rebuild_keep_test.go internal/app/remote/replicate/rebuild_test.go internal/app/remote/replicate/rebuild_stale_test.go
sed -i 's/by FinishLeftover (that/by FinishLeftoverKept (that/' internal/app/remote/replicate/rebuild_stale_test.go
sed -i 's/"FinishLeftover = %v/"FinishLeftoverKept = %v/' internal/app/remote/replicate/rebuild_keep_test.go
```

結果の diff（確認用）:

`internal/app/remote/replicate/rebuild_keep_test.go`:

```diff
--- a/internal/app/remote/replicate/rebuild_keep_test.go
+++ b/internal/app/remote/replicate/rebuild_keep_test.go
@@ -75,9 +75,9 @@
 	if err != nil {
 		t.Fatal(err)
 	}
-	err = FinishLeftover(dir, l)
+	_, err = FinishLeftoverKept(dir, l)
 	if err == nil || !strings.Contains(err.Error(), before) {
-		t.Fatalf("FinishLeftover = %v; want an error naming %s", err, before)
+		t.Fatalf("FinishLeftoverKept = %v; want an error naming %s", err, before)
 	}
 	if b, err := os.ReadFile(filepath.Join(before, "f")); err != nil || string(b) != "old" {
 		t.Errorf("the old store = %q, %v", b, err)
@@ -103,7 +103,7 @@
 	if err != nil {
 		t.Fatal(err)
 	}
-	if err := FinishLeftover(dir, l); err != nil {
+	if _, err := FinishLeftoverKept(dir, l); err != nil {
 		t.Fatal(err)
 	}
 	if b, err := os.ReadFile(filepath.Join(dir, "f")); err != nil || string(b) != "old" {
```

`internal/app/remote/replicate/rebuild_stale_test.go`:

```diff
--- a/internal/app/remote/replicate/rebuild_stale_test.go
+++ b/internal/app/remote/replicate/rebuild_stale_test.go
@@ -27,7 +27,7 @@
 	if err != nil {
 		t.Fatal(err)
 	}
-	if err := FinishLeftover(dir, l); err != nil {
+	if _, err := FinishLeftoverKept(dir, l); err != nil {
 		t.Fatal(err)
 	}
 	if b, err := os.ReadFile(filepath.Join(dir, "f")); err != nil || string(b) != "new" {
@@ -39,7 +39,7 @@
 }
 
 // A rebuild found stale whose old store could not be put back is not put in
-// its place by FinishLeftover (that would drop the commit of the gap): the
+// its place by FinishLeftoverKept (that would drop the commit of the gap): the
 // old store is, and the rebuild goes.
 func TestFinishLeftoverKeepsTheOldStoreOfAStaleRebuild(t *testing.T) {
 	t.Parallel()
@@ -61,7 +61,7 @@
 	if err != nil {
 		t.Fatal(err)
 	}
-	if err := FinishLeftover(dir, l); err != nil {
+	if _, err := FinishLeftoverKept(dir, l); err != nil {
 		t.Fatal(err)
 	}
 	b, err := os.ReadFile(filepath.Join(dir, "f"))
```

`internal/app/remote/replicate/rebuild_test.go`:

```diff
--- a/internal/app/remote/replicate/rebuild_test.go
+++ b/internal/app/remote/replicate/rebuild_test.go
@@ -384,7 +384,7 @@
 	if _, err := store.Open(dir); err == nil {
 		t.Fatal("a store without a directory opens")
 	}
-	if err := FinishLeftover(dir, l); err != nil {
+	if _, err := FinishLeftoverKept(dir, l); err != nil {
 		t.Fatal(err)
 	}
 	if exists(t, before) || exists(t, l.New) {
@@ -416,7 +416,7 @@
 	if err != nil || l.None() {
 		t.Fatalf("leftover = %+v, %v", l, err)
 	}
-	if err := FinishLeftover(dir, l); err != nil {
+	if _, err := FinishLeftoverKept(dir, l); err != nil {
 		t.Fatal(err)
 	}
 	if l, err := store.FindLeftover(dir); err != nil || !l.None() || !exists(t, dir) {
@@ -427,7 +427,7 @@
 		t.Fatal(err)
 	}
 	l, _ = store.FindLeftover(dir)
-	if err := FinishLeftover(dir, l); err != nil {
+	if _, err := FinishLeftoverKept(dir, l); err != nil {
 		t.Fatal(err)
 	}
 	b, err := os.ReadFile(filepath.Join(dir, "f"))
@@ -464,7 +464,7 @@
 	if err != nil {
 		t.Fatal(err)
 	}
-	if err := FinishLeftover(dir, l); err != nil {
+	if _, err := FinishLeftoverKept(dir, l); err != nil {
 		t.Fatal(err)
 	}
 	if b, err := os.ReadFile(filepath.Join(local(dir), "late.json")); err != nil || string(b) != "late" {
@@ -488,7 +488,7 @@
 		t.Fatal(err)
 	}
 	l, _ = store.FindLeftover(dir)
-	if err := FinishLeftover(dir, l); err == nil {
+	if _, err := FinishLeftoverKept(dir, l); err == nil {
 		t.Error("an unreadable local file did not fail")
 	}
 	if !exists(t, before) {
```

- [ ] **Step 4: テストを回す**

Run: `grep -rn 'FinishLeftover\b' internal` （Expected: テスト関数の名前 `TestFinishLeftover…` 以外は出ない。`\b` の都合で `FinishLeftoverKept` は一致しない）
Run: `go vet ./internal/app/remote/... ./internal/app/usecase && go test -count=1 ./internal/app/remote/replicate`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/remote/replicate internal/app/usecase/sync_shared.go
git commit -m "refactor(replicate): 戻り値を捨てるだけの FinishLeftover を消す"
```

---

### Task A5: `stores.PrepareMoveStore` を消し、テストは本番の入口で確かめる

外す差し込み口: `internal/app/usecase/stores/movestore.go` の `PrepareMoveStore`。
本番の呼び出しの確認: cli は `stores.MoveStore`（`cli/mv.go:95`）、server は `MoveStore` と `PrepareMovePlan`。どちらも `PrepareMoveStore` を呼ばない（`grep -rn PrepareMoveStore internal/app/cli internal/app/server` は空）。`movestore_gui_test.go` のコメントが「PrepareMoveStore（the CLI's entry）」と書いていたが、実際の CLI の入口は `MoveStore`（`--yes` なしで `confirm.required` を受け取る）である。
`PrepareMoveStore` が返していた `MoveView` は、`MoveStore` を `Yes: false` で呼んだときの `*MoveConfirmError` の `Args["plan"]` と同じ値（CLI も server もこれを受け取る）。確認前の検査（`store.read_only`・`choice.required`・`move.invalid`）も同じ `openMove`・`load` を通る。

方針: テストの package に `planViewOf` を置き、本番の入口 `MoveStore`（Yes なし）の `Args["plan"]` を取り出す。

使っていたテスト: `movestore_view_test.go` の TestPrepareMoveStoreReturnsTheView、`movestore_test.go` の TestMoveStoreSameStoreAndReadOnly・TestMoveStoreHidden（3 回）・TestMoveStoreUnlinksSprintAndReleases、`movestore_gui_test.go` の TestMovePlanScopedChoice。
`TestPrepareMoveStoreReturnsTheView` は消す。確かめていた内容（From・To・Tasks の数・先頭のタスク）は、同じ `MoveView` を `MoveStore` の確認から取る TestMoveConfirmationHoldsOnlyTheView（From・To・Tasks の数・先頭の ULID・id・title）が確かめているので、残さない。

**Files:**
- Modify: `internal/app/usecase/stores/movestore.go`
- Modify（テスト）: `internal/app/usecase/stores/movestore_test.go`、`movestore_gui_test.go`、`movestore_view_test.go`

**Interfaces:**
- Produces（テストの package だけ）: `func planViewOf(ss *usecase.Session, in stores.MoveStoreInput) (stores.MoveView, error)` — `in.Yes` を false にして `MoveStore` を呼び、確認の `Args["plan"]` の `MoveView` を返す。確認の前に出るエラーはそのまま返す。

- [ ] **Step 1: 本番の呼び出しがないことを確かめる**

Run: `grep -rn 'PrepareMoveStore' internal --include='*.go' | grep -v _test.go`
Expected: `movestore.go` の定義とコメントだけ。cli・server の行が出たら止めて報告する。

- [ ] **Step 2: 本体を消す**

`PrepareMovePlan` の説明が `PrepareMoveStore` を指していたので、説明を自分で完結させる（`move.invalid` の一文は元の `PrepareMoveStore` の説明から移す）。

`internal/app/usecase/stores/movestore.go`:

```diff
--- a/internal/app/usecase/stores/movestore.go
+++ b/internal/app/usecase/stores/movestore.go
@@ -163,24 +163,9 @@
 	Resumed bool // step 7 was done before; only step 8 ran (D12)
 }
 
-// PrepareMoveStore does steps 1 to 4 and 6's checks, and returns the plan to
-// confirm as the confirmation shows it (the entry for the GUI). A task the destination schema refuses is KindInvalid, move.invalid,
-// with one line per task and violation. A scoped entry that applies and no
-// in.Scoped is choice.required.
-func PrepareMoveStore(ss *usecase.Session, in MoveStoreInput) (MoveView, error) {
-	j, err := openMove(ss, in)
-	if err != nil {
-		return MoveView{}, err
-	}
-	w, err := j.load()
-	if err != nil {
-		return MoveView{}, err
-	}
-	return w.plan.View(), nil
-}
-
-// PrepareMovePlan is PrepareMoveStore for the GUI: the plan as GUIView shows
-// it. A scoped entry that applies and no in.Scoped is no error: the plan says
+// PrepareMovePlan does steps 1 to 4 and 6's checks for the GUI, and returns
+// the plan as GUIView shows it. A task the destination schema refuses is
+// KindInvalid, move.invalid, with one line per task and violation. A scoped entry that applies and no in.Scoped is no error: the plan says
 // scoped.required, and is made as if the entry were not carried.
 func PrepareMovePlan(ss *usecase.Session, in MoveStoreInput) (MovePlanView, error) {
 	in.planOnly = true
```

- [ ] **Step 3: テストを直す**

`internal/app/usecase/stores/movestore_test.go`:

```diff
--- a/internal/app/usecase/stores/movestore_test.go
+++ b/internal/app/usecase/stores/movestore_test.go
@@ -23,6 +23,18 @@
 // argsOf are the Args of err, which has been checked to be an *Error.
 func argsOf(err error) map[string]any { return usecase.AsError(err).Args }
 
+// planViewOf is the plan of a move as the confirmation shows it: what the
+// CLI and the server get from MoveStore when the move is not confirmed. An
+// error from the checks before the confirmation is returned as it is.
+func planViewOf(ss *usecase.Session, in stores.MoveStoreInput) (stores.MoveView, error) {
+	in.Yes = false
+	_, err := stores.MoveStore(ss, in)
+	if v, ok := argsOf(err)["plan"].(stores.MoveView); ok {
+		return v, nil
+	}
+	return stores.MoveView{}, err
+}
+
 func TestMoveStore(t *testing.T) {
 	t.Parallel()
 	f := usecasetest.NewMoveFx(t)
@@ -449,7 +461,7 @@
 		hs, hd := f.Head(t, "org-a"), f.Head(t, "org-b")
 		_, err = stores.MoveStore(f.TS.Session, f.In())
 		usecasetest.MustCode(t, err, usecase.KindConflict, "store.read_only")
-		_, err = stores.PrepareMoveStore(f.TS.Session, f.In())
+		_, err = planViewOf(f.TS.Session, f.In())
 		usecasetest.MustCode(t, err, usecase.KindConflict, "store.read_only")
 		if f.Head(t, "org-a") != hs || f.Head(t, "org-b") != hd {
 			t.Errorf("%s read-only: a store was written", name)
@@ -508,7 +520,7 @@
 func TestMoveStoreHidden(t *testing.T) {
 	t.Parallel()
 	f := usecasetest.NewMoveFx(t)
-	p, err := stores.PrepareMoveStore(f.TS.Session, f.In())
+	p, err := planViewOf(f.TS.Session, f.In())
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -529,12 +541,12 @@
 		}
 	}
 	set("org-a", "github.com/alice", "github.com/bob", "github.com/Carol")
-	p, _ = stores.PrepareMoveStore(f.TS.Session, f.In())
+	p, _ = planViewOf(f.TS.Session, f.In())
 	if p.Hidden != nil {
 		t.Errorf("hidden = %v when the destination's list is unknown", p.Hidden)
 	}
 	set("org-b", "github.com/alice", "github.com/carol")
-	p, err = stores.PrepareMoveStore(f.TS.Session, f.In())
+	p, err = planViewOf(f.TS.Session, f.In())
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -563,7 +575,7 @@
 	if err != nil {
 		t.Fatal(err)
 	}
-	p, err := stores.PrepareMoveStore(f.TS.Session, f.In())
+	p, err := planViewOf(f.TS.Session, f.In())
 	if err != nil {
 		t.Fatal(err)
 	}
```

`internal/app/usecase/stores/movestore_gui_test.go`:

```diff
--- a/internal/app/usecase/stores/movestore_gui_test.go
+++ b/internal/app/usecase/stores/movestore_gui_test.go
@@ -85,7 +85,7 @@
 }
 
 // A scoped choice that is not given is no error for the plan the GUI shows
-// first; PrepareMoveStore (the CLI's entry) still asks for it.
+// first; MoveStore (the CLI's entry) still asks for it.
 func TestMovePlanScopedChoice(t *testing.T) {
 	t.Parallel()
 	f := usecasetest.NewMoveFx(t)
@@ -96,7 +96,7 @@
 			Statuses: []string{"backlog", "todo", "review", "in_progress", "done", "canceled"},
 		}}
 	})
-	_, err := stores.PrepareMoveStore(f.TS.Session, f.In())
+	_, err := planViewOf(f.TS.Session, f.In())
 	usecasetest.MustCode(t, err, usecase.KindUsage, "choice.required")
 
 	v, err := stores.PrepareMovePlan(f.TS.Session, f.In())
```

`internal/app/usecase/stores/movestore_view_test.go`:

```diff
--- a/internal/app/usecase/stores/movestore_view_test.go
+++ b/internal/app/usecase/stores/movestore_view_test.go
@@ -47,20 +47,6 @@
 	}
 }
 
-// PrepareMoveStore is the entry for a caller that asks in its own way: it
-// returns the same view.
-func TestPrepareMoveStoreReturnsTheView(t *testing.T) {
-	t.Parallel()
-	f := usecasetest.NewMoveFx(t)
-	v, err := stores.PrepareMoveStore(f.TS.Session, f.In())
-	if err != nil {
-		t.Fatal(err)
-	}
-	if v.From != "org-a" || v.To != "org-b" || len(v.Tasks) != 3 || v.Tasks[0].ULID != f.P {
-		t.Errorf("view = %+v", v)
-	}
-}
-
 // The words for what a scoped schema loses are usecase's, for every client.
 func TestScopedPlanLines(t *testing.T) {
 	t.Parallel()
```

- [ ] **Step 4: テストを回す**

Run: `go vet ./internal/app/usecase/... && go test -count=1 ./internal/app/usecase/stores`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/usecase/stores
git commit -m "refactor(stores): 呼び元のない PrepareMoveStore を消し、テストは MoveStore の確認で確かめる"
```

---

### Task C1: ロックの待ち時間（lockPoll・indexLockPoll・indexLockWait）を const に戻す

外す差し込み口: `store.lockPoll`（lock.go）、`gitrepo.indexLockPoll`、`gitrepo.indexLockWait`（lock.go）。いずれも本番は定数のまま、テストだけが `var` を書き換えていた。

使っていたテスト:
- `store` の `testmain_test.go`（全テストで `lockPoll = 5ms`）
- `gitrepo` の `testmain_test.go`（全テストで `indexLockPoll = 5ms`）
- `gitrepo/lock_test.go` の `TestCommitWaitsForIndexLock`、`TestFastForwardWaitsForIndexLock`（`indexLockWait = 5ms`）

確かめる性質は変えない。2 件の「WaitsFor」テストは本物の 2 秒を待ち、`ErrGitBusy`・他人の index.lock が残ること・HEAD が動かないこと・x.md が書かれないことを見る。`osfs.LockFile` と `lockIndex` は待ち時間を `min(poll, 残り)` で切るので、失敗系のテストは poll に関わらず wait の時刻に終わる。解放を待つ側（`TestCommitTakesTheLockWhenItIsReleased`）だけが最大 100ms 遅くなる。

注意: `store/tx.go` の `lockStore`（`time.Sleep(min(lockPoll, ...))`）も `lockPoll` を使う。const にしても型は `time.Duration` の型付き定数のままなので、そのままコンパイルできる（確認済み）。

**Files:**
- Modify: `internal/app/storage/store/lock.go`
- Modify: `internal/app/storage/store/testmain_test.go`
- Modify: `internal/lib/gitrepo/lock.go`
- Modify: `internal/lib/gitrepo/testmain_test.go`
- Modify: `internal/lib/gitrepo/lock_test.go`

**Interfaces:**
- Produces: なし（`lockPoll`・`indexLockWait`・`indexLockPoll` が const になるだけ）

- [ ] **Step 1: 本番の var を const にする**

`internal/app/storage/store/lock.go`:

```go
// 変更前
var lockPoll = 50 * time.Millisecond
// 変更後
const lockPoll = 50 * time.Millisecond
```

`internal/lib/gitrepo/lock.go`:

```go
// 変更前
var indexLockWait = 2 * time.Second

var indexLockPoll = 100 * time.Millisecond
// 変更後
const indexLockWait = 2 * time.Second

const indexLockPoll = 100 * time.Millisecond
```

- [ ] **Step 2: テストの書き換えを外す**

`internal/app/storage/store/testmain_test.go` 全体を次にする（`time` の import とコメント・代入を消す）:

```go
package store

import (
	"testing"

	"github.com/kukv/hedgehug/internal/lib/testenv"
)

func TestMain(m *testing.M) {
	testenv.Main(m)
}
```

`internal/lib/gitrepo/testmain_test.go` 全体を次にする:

```go
package gitrepo

import (
	"testing"

	"github.com/kukv/hedgehug/internal/lib/testenv"
)

func TestMain(m *testing.M) {
	testenv.Main(m)
}
```

`internal/lib/gitrepo/lock_test.go`: `TestCommitWaitsForIndexLock` と `TestFastForwardWaitsForIndexLock` から次の 2 行を消し、先頭に `t.Parallel()` を足す（変数を書き換えなくなったので並行にでき、2 秒が重ならずに済む）。

```go
// 消す（2 か所）
	indexLockWait = 5 * time.Millisecond
	t.Cleanup(func() { indexLockWait = 2 * time.Second })
```

```go
func TestCommitWaitsForIndexLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	...
func TestFastForwardWaitsForIndexLock(t *testing.T) {
	t.Parallel()
	remote := newBare(t)
	...
```

`lock_test.go` の `time` の import は他で使っているので残る。

- [ ] **Step 3: テストを回す**

Run: `go test -count=1 ./internal/lib/gitrepo -run 'WaitsForIndexLock|TakesTheLock'`
Expected: PASS（`TestCommitWaitsForIndexLock` 約 2.0s、`TestFastForwardWaitsForIndexLock` 約 2.0s、`TestCommitTakesTheLockWhenItIsReleased` 約 0.1s）

Run: `go test -count=1 ./internal/lib/gitrepo ./internal/app/storage/store`
Expected: PASS

- [ ] **Step 4: 検出できることを確かめる**

`internal/lib/gitrepo/lock.go` の `lockIndex` の期限切れの分岐を一時的に壊し、`go test -count=1 ./internal/lib/gitrepo -run 'WaitsForIndexLock'` が落ちることを見て、戻す。

壊し方 1（他人のロックを消す）:

```go
		if !time.Now().Before(deadline) {
			_ = osfs.Remove(p)
			return nil, ErrGitBusy
```
Expected: FAIL（`the index.lock of another process is gone` が 2 テストとも）

壊し方 2（ロックなしで先へ進む）:

```go
		if !time.Now().Before(deadline) {
			return func() {}, nil
```
Expected: FAIL（`Commit with index.lock = <nil>`、`FastForward with index.lock = <nil>`、`HEAD moved`、`x.md was written`）

- [ ] **Step 5: コミット**

```bash
git add internal/app/storage/store/lock.go internal/app/storage/store/testmain_test.go internal/lib/gitrepo/lock.go internal/lib/gitrepo/testmain_test.go internal/lib/gitrepo/lock_test.go
git commit -m "refactor(store,gitrepo): ロックの待ち時間をテストで書き換えない"
```

---

### Task C2: Store.commit と newStoreHook を、本物の .git/index.lock に置き換える

外す差し込み口: `Store.commit` フィールド（store.go。本番は `repo.Commit` を入れるだけ）、`newStoreHook`（store.go）。`Store.commit` の呼び出し 3 か所（store.go の Create、merge.go の CommitMerge、tx.go の Update）は `s.repo.Commit` を直接呼ぶ。

使っていたテスト:
- `store/rank_test.go`: `TestRerankIsRestoredWhenTheOperationFails`、`TestRerankIsRestoredForEveryTask`
- `store/merge_test.go`: `TestCommitMergeRollsBack`
- `store/tx_test.go`: `TestUpdateCommitFailureRollsBack`
- `store/ids_test.go`: `TestUpdateRestoresIDsOnFailure`
- `store/store_test.go`: `TestCreateCleansUpOnFailure`、`TestCreateCanRetryAfterFailure`
- `store/workspace_test.go`: `TestPurgeSubjectWithoutIDUsesULID`（`s.commit(...)` を直に呼んでいただけ。`s.repo.Commit` に替える）

手段: 本物の `.git/index.lock` を置く。コミットは `lockIndex` が 2 秒待ってから `ErrGitBusy` で失敗する。失敗の種類は元のテスト（任意の error）より具体的になる（`errors.Is(err, gitrepo.ErrGitBusy)`）。失敗後のロールバック（`s.rollback`、`tx.restoreIDs`、`undoCreate`）は本番と同じ道筋を通る。置くのはどのテストも呼び出しの前（Update のコールバックの中でなくてよい。ロックはコミットまで何も邪魔しない）。Create だけは、ディレクトリがまだ無いので、`now`（Init の後・ファイル書き込みの前に 1 回だけ呼ばれる）の中で置く。

各テストは 2 秒かかるようになる。`t.Parallel()` が付いているので、パッケージ全体では重なる。Create 系 3 件は元は並行でなかった（パッケージ変数のため）が、並行にする。

rank の 2 件は「コミット時点の rank」を直接見ていた。外から見える形に変える: ロックを置いたまま失敗させて rank が元に戻ることを見る（元の後半と同じ）→ ロックを外して同じ書き込みをやり直し、成功した時に rank が `a5` / `a7` から変わっている（再採番が走る）ことを見る。確かめる内容は「失敗した書き込みは rank を元に戻す」「その書き込みでは再採番が実際に起きる」で、後者は「コミット時点の値」ではなく「成功する同じ操作の結果」で見る。

**Files:**
- Modify: `internal/lib/gitrepo/gitrepotest/lock.go`
- Modify: `internal/app/storage/store/store.go`
- Modify: `internal/app/storage/store/merge.go`
- Modify: `internal/app/storage/store/tx.go`
- Test: `internal/app/storage/store/rank_test.go`、`merge_test.go`、`tx_test.go`、`ids_test.go`、`store_test.go`、`workspace_test.go`

**Interfaces:**
- Produces: `func HoldIndexLock(t testing.TB, dir string) (release func())`（`gitrepotest`）。`dir` の `.git/index.lock` を作る。`release` は消す（2 回呼んでもよい）。テストの cleanup も消す。
- Produces（store_test.go のテスト内）: `func busyAfterInit(t *testing.T, dir string) func() time.Time`（`Create` の `now` に渡す）

- [ ] **Step 1: gitrepotest に HoldIndexLock を足す**

`internal/lib/gitrepo/gitrepotest/lock.go` の import を次にし、`LockObjects` の前に関数を足す。

```go
import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// HoldIndexLock creates .git/index.lock in the work tree dir, as a git run by
// hand does, so that a commit there waits for it and then fails with
// gitrepo.ErrGitBusy. release removes it; the cleanup of the test does too.
func HoldIndexLock(t testing.TB, dir string) (release func()) {
	t.Helper()
	p := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	release = func() {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Error(err)
		}
	}
	t.Cleanup(release)
	return release
}
```

（`gitrepo` 自身のテストは `gitrepotest` を import できない（gitrepotest が gitrepo を import している）ので、`gitrepo/lock_test.go` は従来どおり自前で `os.WriteFile` する。）

- [ ] **Step 2: 本番の差し込み口を外す**

`internal/app/storage/store/store.go`:

```go
// Store の構造体から次の行を消す
	commit   func(gitrepo.Commit) (plumbing.Hash, error)

// newStore から次の行を消す
		commit:   repo.Commit,

// 次の 2 つを消す
// newStoreHook lets tests change the store Create made before it writes.
var newStoreHook = func(*Store) {}
//   と Create の中の
	newStoreHook(s)

// Create の最後のコミットを替える
	if _, err := s.repo.Commit(gitrepo.Commit{
```

`s.commit` を消したので `store.go` の `"github.com/go-git/go-git/v5/plumbing"` の import が未使用になる。その 1 行と直後の空行を消す。`Store` 構造体の「Replaced by tests.」の下には `now`・`lockWait` が残る。

`internal/app/storage/store/merge.go`: `s.commit(c)` → `s.repo.Commit(c)`

`internal/app/storage/store/tx.go`: `s.commit(commit)` → `s.repo.Commit(commit)`

`internal/app/storage/store/workspace_test.go`（`TestPurgeSubjectWithoutIDUsesULID`）: `s.commit(gitrepo.Commit{...})` → `s.repo.Commit(gitrepo.Commit{...})`

- [ ] **Step 3: rank・merge・tx・ids のテストを書き換える**

各ファイルの import に `"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"` を足し、使わなくなった `"github.com/go-git/go-git/v5/plumbing"`（rank_test・ids_test・store_test・tx_test。merge_test は他で使うので残す）を消す。

`rank_test.go` の `TestRerankIsRestoredWhenTheOperationFails`（`a, b` と `writeShared` までは変更なし）:

```go
	release := gitrepotest.HoldIndexLock(t, s.dir)
	if err := put(s, b); !errors.Is(err, gitrepo.ErrGitBusy) {
		t.Fatalf("put with index.lock = %v, want ErrGitBusy", err)
	}
	if b.Rank != "a5" {
		t.Errorf("b.Rank = %s after a failed write, want a5", b.Rank)
	}
	// The same write succeeds once git is done, and then b is reranked: the
	// failed one had reranked it too, before it was put back.
	release()
	mustPut(t, s, b)
	if b.Rank == "a5" {
		t.Error("b was not reranked in the operation")
	}
```

`TestRerankIsRestoredForEveryTask`（`writeShared` まで変更なし）:

```go
	release := gitrepotest.HoldIndexLock(t, s.dir)
	// a is first of the a5 group, f is last of the a7 group: both change.
	if err := put(s, a, f); !errors.Is(err, gitrepo.ErrGitBusy) {
		t.Fatalf("put with index.lock = %v, want ErrGitBusy", err)
	}
	if a.Rank != "a5" || f.Rank != "a7" {
		t.Errorf("ranks = %s, %s after a failed write, want a5, a7", a.Rank, f.Rank)
	}
	// The same write succeeds once git is done, and then both are reranked:
	// the failed one had reranked them too, before it put them back.
	release()
	mustPut(t, s, a, f)
	if a.Rank == "a5" || f.Rank == "a7" {
		t.Errorf("ranks after the write = %s, %s; want both reranked", a.Rank, f.Rank)
	}
```

`merge_test.go` の `TestCommitMergeRollsBack`:

```go
	gitrepotest.HoldIndexLock(t, s.dir)
	if _, err := s.CommitMerge(head, []plumbing.Hash{head}, files, Op{Name: "merge", Author: testAuthor}, nil); !errors.Is(err, gitrepo.ErrGitBusy) {
		t.Fatalf("CommitMerge with index.lock = %v, want ErrGitBusy", err)
	}
```
（`s.commit = ...` の行と、元の `err == nil || errors.Is(err, gitrepo.ErrWorkTreeChanged)` の条件を置き換える。後続の old.md・new.md の確認は変更なし。）

`tx_test.go` の `TestUpdateCommitFailureRollsBack`: `boom := ...` と `s.commit = ...` の 2 行を `gitrepotest.HoldIndexLock(t, s.dir)` 1 行にし、判定を次にする。

```go
	if !errors.Is(err, gitrepo.ErrGitBusy) {
		t.Fatalf("err = %v, want ErrGitBusy", err)
```

`ids_test.go` の `TestUpdateRestoresIDsOnFailure`（後半。前半の「refused write」は変更なし）:

```go
	y := newTaskIn(ws, "y")
	gitrepotest.HoldIndexLock(t, s.dir)
	if err := put(s, y); !errors.Is(err, gitrepo.ErrGitBusy) {
		t.Fatalf("put with index.lock = %v, want ErrGitBusy", err)
	}
```

- [ ] **Step 4: Create 系のテストを書き換える**

`internal/app/storage/store/store_test.go`。`TestCreateCleansUpOnFailure` の前に補助を足し、3 つのテストを次にする（`Create` の引数は元のまま、最後の `now` だけ `busyAfterInit(t, dir)` にする。成功を期待する再試行は `time.Now` のまま）。

```go
// busyAfterInit is the clock of a Create that fails: Create asks for the time
// once git init is done and before it writes a file, and the .git/index.lock
// made then keeps its commit out, as a git run by hand would.
func busyAfterInit(t *testing.T, dir string) func() time.Time {
	return func() time.Time {
		gitrepotest.HoldIndexLock(t, dir)
		return time.Now()
	}
}

func TestCreateCleansUpOnFailure(t *testing.T) {
	t.Parallel()
	t.Run("a directory Create made is removed", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "store")
		if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", busyAfterInit(t, dir)); !errors.Is(err, gitrepo.ErrGitBusy) {
			t.Fatalf("err = %v, want ErrGitBusy", err)
		}
		// 以下 os.Stat(dir) の確認は変更なし
	})
	t.Run("an empty directory is emptied again", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", busyAfterInit(t, dir)); !errors.Is(err, gitrepo.ErrGitBusy) {
			t.Fatalf("err = %v, want ErrGitBusy", err)
		}
		// 以下 os.ReadDir(dir) の確認は変更なし
	})
}

func TestCreateCanRetryAfterFailure(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "store")
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", busyAfterInit(t, dir)); err == nil {
		t.Fatal("Create with a failing commit succeeded")
	}
	if _, err := Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "x"}, nil, &model.Workspace{ULID: model.NewULID(), Name: "x", IDPrefix: "TST"}, testAuthor, "", time.Now); err != nil {
		t.Errorf("retry: %v", err)
	}
}
```

失敗した Create は `undoCreate` が `.git` ごと消すので、置いたロックも消える（`HoldIndexLock` の cleanup は消えたファイルを許す）。

- [ ] **Step 5: テストを回す**

Run: `go vet ./internal/app/storage/store ./internal/lib/gitrepo/...`
Expected: 出力なし

Run: `go test -count=1 ./internal/app/storage/store -run 'TestCreate|Rerank|TestCommitMergeRollsBack|TestUpdateCommitFailureRollsBack|TestUpdateRestoresIDsOnFailure|TestPurgeSubject'`
Expected: PASS（各 2 秒前後のテストが並行に走り、全体で約 2.1s）

- [ ] **Step 6: 検出できることを確かめる**

次のとおりに一時的に壊し、指定のテストが落ちることを見て、戻す（`go test -count=1 ./internal/app/storage/store -run '<テスト名>'`）。

| 壊し方 | 落ちるべきテスト | 実測 |
|---|---|---|
| `store.go` の `err = errors.Join(err, undoCreate(dir, made))` を `_ = made` にする | `TestCreateCleansUpOnFailure`、`TestCreateCanRetryAfterFailure` | FAIL（`Stat(dir) err = <nil>`、`dir has 6 entries`、`retry: ... is not empty`） |
| `merge.go` の `return errors.Join(cerr, s.rollback(changes))` を `return cerr` にする | `TestCommitMergeRollsBack` | FAIL（`old.md = ""`、`new.md stayed`） |
| `tx.go` の `errors.Join(err, s.rollback(changes))` を `err` にする | `TestUpdateCommitFailureRollsBack` | FAIL（`edited file was not restored`、`new file was not removed`） |
| `ids.go` の `restoreIDs` の `for _, o := range tx.oldRanks { ... }` を消す | `TestRerankIsRestoredWhenTheOperationFails`、`TestRerankIsRestoredForEveryTask` | FAIL（`b.Rank = a6 ... want a5`、`ranks = a4, a8 ... want a5, a7`） |
| `ids.go` の `o.t.ID, o.t.FormerIDs = o.id, o.former` を `_ = o` にする | `TestUpdateRestoresIDsOnFailure` | FAIL（`after a failed commit: id = TST-1`） |
| `tx.go` の `err = tx.rerank()` を `err = nil` にする（再採番が走らない） | rank の 2 件 | FAIL（rank が変わらず変更なし＝コミットされず、`put with index.lock = <nil>`） |

- [ ] **Step 7: コミット**

```bash
git add internal/lib/gitrepo/gitrepotest/lock.go internal/app/storage/store/store.go internal/app/storage/store/merge.go internal/app/storage/store/tx.go internal/app/storage/store/rank_test.go internal/app/storage/store/merge_test.go internal/app/storage/store/tx_test.go internal/app/storage/store/ids_test.go internal/app/storage/store/store_test.go internal/app/storage/store/workspace_test.go
git commit -m "refactor(store): コミットの失敗を本物の index.lock で作る"
```

---

### Task C3: LockInOrder の afterLock を、本物のロックで確かめる

外す差し込み口: `store.afterLock`（move.go。各ストアのロックを取るたびに呼ばれ、順番を記録していた）。

使っていたテスト: `store/move_test.go` の `TestLockInOrder`（`afterLock` で順番を集め、`[a b]` かを見ていた。直列だった）。

確かめる内容を、実装内部の順番から外から見える性質に変える（決まりの「回数・順序」の項）:
1. `TestLockInOrderTakesTheSmallestFirst`（新）: `a`（小さい ULID）のロックを外（別プロセスの代わり）が握っている間、`LockInOrder(b, a)` は `a` を待ち、`b` を取っていない。`b` のロックを `acquire(..., 0)` で取れることで見る。
2. `TestLockInOrderDoesNotDeadlock`（新）: 同じ 2 つのディレクトリに別ハンドルを開き、`LockInOrder(a, b)` と `LockInOrder(b2, a2)`（逆順）を同時に 20 回呼んで、どれもエラーにならない。`lockWait` を 2 秒にしておき、デッドロックなら `ErrLocked` で落ちる。
3. `TestLockInOrder`（既存）は順番の確認だけを消す。残りの確認（Release まで待つ、保持者は待たない、Release 後の扱い）は変更なし。直列の理由（パッケージ変数）が無くなるので `t.Parallel()` を付ける。

**Files:**
- Modify: `internal/app/storage/store/move.go`
- Test: `internal/app/storage/store/move_test.go`

**Interfaces:**
- Produces: なし

- [ ] **Step 1: 本番の差し込み口を外す**

`internal/app/storage/store/move.go`:

```go
// 消す
// afterLock is called with each store after LockInOrder took its lock;
// tests use it to see the order.
var afterLock = func(*Store) {}

// LockInOrder の for の中から消す
		afterLock(s)
```

- [ ] **Step 2: テストを書き換える**

`internal/app/storage/store/move_test.go` の `TestLockInOrder` の冒頭（`afterLock` を差し替える部分と `order` の確認）を次のように替え、その前に 2 つのテストを足す。`TestLockInOrder` の `l.Release()` 以降は変更なし。

```go
// LockInOrder takes the smallest ULID first: while a is held by another
// process, LockInOrder(b, a) waits for a and has not taken b.
func TestLockInOrderTakesTheSmallestFirst(t *testing.T) {
	t.Parallel()
	a, _ := newMoveStore(t, "a", "AAA", nil)
	b, _ := newMoveStore(t, "b", "BBB", nil)
	other, err := acquire(a.path(lockPath), time.Second) // another process holds a
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		l   *Locked
		err error
	}
	done := make(chan result, 1)
	go func() {
		l, err := LockInOrder(b, a)
		done <- result{l, err}
	}()
	select {
	case r := <-done:
		t.Fatalf("LockInOrder did not wait for a: %v", r.err)
	case <-time.After(200 * time.Millisecond):
	}
	// b is free: LockInOrder is waiting for a, and has not taken b first.
	free, err := acquire(b.path(lockPath), 0)
	if err != nil {
		t.Fatalf("b is held while LockInOrder waits for a: %v", err)
	}
	if err := free.Release(); err != nil {
		t.Fatal(err)
	}
	if err := other.Release(); err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.err != nil {
		t.Fatal(r.err)
	}
	r.l.Release()
}

// Two callers that name the same stores in opposite orders, each with its own
// handles, do not wait for each other for good.
func TestLockInOrderDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	a, _ := newMoveStore(t, "a", "AAA", nil)
	b, _ := newMoveStore(t, "b", "BBB", nil)
	a2, err := Open(a.dir)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := Open(b.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []*Store{a, b, a2, b2} {
		s.lockWait = 2 * time.Second
	}
	for range 20 {
		start := make(chan struct{})
		errs := make(chan error, 2)
		for _, ss := range [][]*Store{{a, b}, {b2, a2}} {
			go func() {
				<-start
				l, err := LockInOrder(ss...)
				if err == nil {
					l.Release()
				}
				errs <- err
			}()
		}
		close(start)
		for range 2 {
			if err := <-errs; err != nil {
				t.Fatalf("LockInOrder: %v", err)
			}
		}
	}
}

func TestLockInOrder(t *testing.T) {
	t.Parallel()
	a, _ := newMoveStore(t, "a", "AAA", nil)
	b, _ := newMoveStore(t, "b", "BBB", nil)

	l, err := LockInOrder(b, a, b)
	if err != nil {
		t.Fatal(err)
	}

	// A writer without the hold waits for Release, in this process too.
	// （以下は変更なし）
```

`newMoveStore` が作るストアの ULID は a < b（後に作るほど大きい）。`slices` の import は他で使われているので残る。

- [ ] **Step 3: テストを回す**

Run: `go test -count=1 ./internal/app/storage/store -run 'TestLockInOrder'`
Expected: PASS（`TestLockInOrderTakesTheSmallestFirst` 約 0.2s、`TestLockInOrderDoesNotDeadlock` 約 0.5s、`TestLockInOrder` 約 0.15s）

- [ ] **Step 4: 検出できることを確かめる**

`move.go` の `LockInOrder` の並べ替えを一時的に外す（`slices.SortFunc(uniq, ...)` の 3 行を消し、`strings`・`slices` が未使用にならないよう `_ = strings.Compare` と `_ = slices.SortFunc[[]*Store]` を置く）。

Run: `go test -count=1 ./internal/app/storage/store -run 'TestLockInOrder(DoesNot|Takes)'` を 3 回
Expected: 3 回とも FAIL。実測:
- `TestLockInOrderTakesTheSmallestFirst`: `b is held while LockInOrder waits for a: the store is locked by another process`（3/3）
- `TestLockInOrderDoesNotDeadlock`: `LockInOrder: lock a: the store is locked by another process`（または `lock b`）（3/3、約 2 秒で落ちる）

どちらも「順番を壊すと落ちる」。前者は時間に左右されて誤って落ちることはない（待ちが短すぎると見逃すだけ。待ちは 200ms、順番が壊れた時に `b` を取るのはマイクロ秒）。後者は逆順のインターリーブが起きた時に落ちる確率的な検出だが、20 回の繰り返しで 3/3 検出した。前者が決定的な検出、後者は補助。壊した後は戻す。

- [ ] **Step 5: コミット**

```bash
git add internal/app/storage/store/move.go internal/app/storage/store/move_test.go
git commit -m "refactor(store): ロックの順番を本物のロックで確かめる"
```

---

### Task C4: PrefixUses の openStore を外す

外す差し込み口: `store.openStore`（prefix.go。`Open` を入れ、テストが呼び出し回数を数えていた）。`PrefixUses` は `Open` を直接呼ぶ。

使っていたテスト: `store/prefix_test.go` の `TestPrefixUses`（直列）。

確かめる内容: 結果（`uses` が 1 件で `Store == "a"`・`Workspace.Name == "test"`、`skipped` が `["gone"]`、prefix `WEB` では 0 件）はそのまま。「各ストアを 1 回だけ open する」という回数の確認は、外から数えられないので捨てる。つまり、このテストは「prefix の大小を無視して一致する workspace を返す」「開けないストアを skipped に渡して飛ばす」の 2 つを確かめる。関数のコメントの「opens each of stores once」は挙動の説明として残す。

**Files:**
- Modify: `internal/app/storage/store/prefix.go`
- Test: `internal/app/storage/store/prefix_test.go`

**Interfaces:**
- Produces: なし

- [ ] **Step 1: 本番**

`internal/app/storage/store/prefix.go`:

```go
// 消す
// openStore opens a store for PrefixUses; tests count the calls.
var openStore = Open

// 変更
		s, err := Open(e.Path)
```

- [ ] **Step 2: テスト**

`internal/app/storage/store/prefix_test.go` の冒頭を次にし、`opened` の確認（`if opened != 2 { ... }` の 3 行）を消す。

```go
func TestPrefixUses(t *testing.T) {
	t.Parallel()
	s := newTestStore(t) // workspace "test" with the prefix TST
	var skipped []string
	// 以下は変更なし
```

（`// 直列: パッケージ変数 openStore の差し替え` のコメント、`opened := 0`、`openStore = ...`、`defer ...` を消す。）

- [ ] **Step 3: テストを回す**

Run: `go test -count=1 ./internal/app/storage/store -run 'TestPrefixUses'`
Expected: PASS

- [ ] **Step 4: 検出できることを確かめる**

回数の確認は捨てたので、残した 2 つの性質を確かめる。`prefix.go` の一致を `strings.EqualFold(w.IDPrefix, p)` から `w.IDPrefix == p` に壊すと、prefix `tst`（小文字）で 0 件になり `uses = []` で FAIL することを見て戻す。`skipped(e.Name, err)` の行を消すと `skipped = []` で FAIL することを見て戻す。（実測: 前者は `uses = []`、後者は `skipped = []` で FAIL。どちらも元からある確認で、このタスクでは変えていない。）

- [ ] **Step 5: コミット**

```bash
git add internal/app/storage/store/prefix.go internal/app/storage/store/prefix_test.go
git commit -m "refactor(store): PrefixUses の open を差し替えない"
```

---

### Task C5: index の readHook・listHook を、object の退避で確かめる

外す差し込み口: `index.readHook`（build.go。読む前に呼ばれ、error を返すと読みが失敗する）、`index.listHook`（build.go。一覧を取る prefix ごとに呼ばれる）。`ix.read` は `ix.repo.ReadFile` を呼ぶだけになる（薄いラッパーとして残す。他の呼び出しが多いので消さない）。

使っていたテスト:
- `index/index_test.go` の `TestFailedUpdateThenOpenEqualsARebuild`（`readHook` で `comments/` の読みを失敗させた）
- `index/comments_test.go` の `TestUpdateReadsOnlyTheNotesOfTheTouchedTask`（`listHook`・`readHook` で一覧・読みを記録した）

手段: テストのストアの object はすべてバラ（loose）で、`.git/objects/xx/yyyy…` が 1 つのファイルになっている。その 1 つを `<名前>.hidden` に改名すると、読みは `object not found` で本当に失敗する。戻せば元に戻る。pack 済みの object には使えないので、補助は退避に失敗したらテストを失敗させる（loose でなくなったら気づける）。

補助（`index/helpers_test.go`）:
`hideObjectAt(t, dir, rel) (restore func())`。HEAD のツリーから `rel`（ファイルでもディレクトリでもよい）の object のハッシュを引き、退避する。cleanup でも戻す。

確かめる内容:
- (a) `TestFailedUpdateThenOpenEqualsARebuild`: タスク a に付けたコメントの blob を退避して `Refresh` を失敗させ、戻してから `Open` した結果が再構築と一致する（元と同じ）。
- (b) `TestUpdateReadsOnlyTheNotesOfTheTouchedTask`: 触っていないタスク b の object（コメントの blob、b のコメントのディレクトリの tree、b のタスクの blob）を退避しても更新が成功する。「読んでいない・一覧を取っていない」を、記録ではなく「読めば失敗する状態でも成功する」で示す。元の `listed == noteDirs(a)` の完全一致は、外から見えないので捨てる（b の一覧を取れば b の tree が無くて失敗するので、性質としては同じことを見ている）。

注意（試して分かったこと）: go-git のハンドルは読んだ object をキャッシュする（`gitrepo.Open` が作るストレージの LRU）。(b) で、最初のハンドルで `Open` した後にそのまま `Refresh` すると、`Open` が読んだ b の tree が残っていて、b の一覧を取る壊れた実装でも成功してしまう（試して検出できなかった）。そこで (b) は次の形にした: 最初のハンドルで `Open` してキャッシュファイルを書く → a にコメントを付ける → b の object を退避 → 新しいハンドル（`store.Open(dir)`）で `Open(s2)` する。`Open` はキャッシュから HEAD まで更新（`update`）するので、新しいハンドルは何もキャッシュしておらず、読んだ object だけが退避の影響を受ける。(a) は a のコメントの blob が最初のハンドルに一度も読まれていない（コミットで書かれただけ）ので、同じハンドルの `Refresh` で足りる。

**Files:**
- Modify: `internal/app/storage/index/build.go`
- Modify: `internal/app/storage/index/comments.go`
- Test: `internal/app/storage/index/helpers_test.go`
- Test: `internal/app/storage/index/index_test.go`
- Test: `internal/app/storage/index/comments_test.go`

**Interfaces:**
- Produces（テスト内）: `func hideObjectAt(t *testing.T, dir, rel string) (restore func())`

- [ ] **Step 1: 本番の差し込み口を外す**

`internal/app/storage/index/build.go`:

```go
// 消す
// readHook, when set, is called before each read of a file; an error it
// returns fails the read. Tests set it.
var readHook func(p string) error

// listHook, when set, is called with each prefix whose comment and worklog
// files are listed. Tests set it.
var listHook func(prefix string)

// read の中から消す
	if readHook != nil {
		if err := readHook(p); err != nil {
			return nil, err
		}
	}
```

`internal/app/storage/index/comments.go` の `readNotesUnder` から消す:

```go
	if listHook != nil {
		listHook(prefix)
	}
```

- [ ] **Step 2: 補助を足す**

`internal/app/storage/index/helpers_test.go` の import に `gogit "github.com/go-git/go-git/v5"` を足し（標準ライブラリの import ブロックの後に空行を置いて）、末尾に足す。

```go
// hideObjectAt moves the loose object of the file or directory at rel in
// HEAD of the repository in dir out of the object database, so that a read
// of it fails with plumbing.ErrObjectNotFound. restore puts it back; the
// cleanup of the test does too. The test fails when the object is not loose:
// the objects of a test store are, until something packs them.
func hideObjectAt(t *testing.T, dir, rel string) (restore func()) {
	t.Helper()
	r, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	tree, err := c.Tree()
	if err != nil {
		t.Fatal(err)
	}
	e, err := tree.FindEntry(rel)
	if err != nil {
		t.Fatal(err)
	}
	h := e.Hash.String()
	loose := filepath.Join(dir, ".git", "objects", h[:2], h[2:])
	hidden := loose + ".hidden"
	if err := os.Rename(loose, hidden); err != nil {
		t.Fatalf("%s is not a loose object: %v", rel, err)
	}
	restore = func() {
		if _, err := os.Stat(hidden); err == nil {
			if err := os.Rename(hidden, loose); err != nil {
				t.Error(err)
			}
		}
	}
	t.Cleanup(restore)
	return restore
}
```

- [ ] **Step 3: テストを書き換える**

`index_test.go` の `TestFailedUpdateThenOpenEqualsARebuild`（`"errors"` の import を消す。`strings` は他で使う）:

```go
func TestFailedUpdateThenOpenEqualsARebuild(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	a, b := newTask("a"), newTask("b")
	put(t, s, a, b)
	ix := mustOpen(t, s)
	a.Title, b.Title = "a2", "b2"
	b.Body = "@bob #TST-1\n"
	put(t, s, a, b)
	c := newComment("github.com/x", "@carol\n")
	putComment(t, s, a.ULID, c)

	restore := hideObjectAt(t, dir, model.CommentPath(a.ULID, c.ULID))
	err := ix.Refresh()
	restore()
	if err == nil {
		t.Fatal("Refresh succeeded with a failed read")
	}
	// 以下（Open と再構築の比較）は変更なし
```

`comments_test.go`（import に `"path"` を足す。`TestUpdateReadsOnlyTheNotesOfTheTouchedTask` の冒頭から `Refresh` の確認までを置き換える。`a`・`b` の Mentions・Comments の最後の 2 つの確認は変更なし。直前のコメントに説明を足す）:

```go
// An update lists and reads the comments and worklog entries of the tasks
// it touches only, under their own directories.
//
// The test takes the objects b has out of the object database, so that a read
// of one fails: the comment file, the directory of its comments and the task
// file. The update succeeds only when it reads none of them. It is run by
// Open on a second handle on the store, from the cache the first one wrote,
// because a handle keeps the objects it has read.
func TestUpdateReadsOnlyTheNotesOfTheTouchedTask(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	a, b := newTask("a"), newTask("b")
	put(t, s, a, b)
	bc := newComment("github.com/x", "@bob\n")
	putComment(t, s, b.ULID, bc)
	mustOpen(t, s) // writes the cache
	putComment(t, s, a.ULID, newComment("github.com/x", "@alice\n"))

	hideObjectAt(t, dir, model.CommentPath(b.ULID, bc.ULID))
	hideObjectAt(t, dir, path.Dir(model.CommentPath(b.ULID, bc.ULID)))
	hideObjectAt(t, dir, model.TaskPath(b.ULID))
	s2, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Open(s2)
	if err != nil {
		t.Fatalf("the update read a file of a task it did not touch: %v", err)
	}
	// 以下 a・b の確認は変更なし
```

- [ ] **Step 4: テストを回す**

Run: `go vet ./internal/app/storage/index`
Expected: 出力なし

Run: `go test -count=1 ./internal/app/storage/index -run 'TestFailedUpdateThenOpenEqualsARebuild|TestUpdateReadsOnlyTheNotes'`
Expected: PASS

- [ ] **Step 5: 検出できることを確かめる**

次のとおり一時的に壊し、`go test -count=1 ./internal/app/storage/index -run '<テスト名>'` が落ちることを見て、戻す。

1. `index.go` の `Refresh` の末尾 `if err != nil { return err }` + `ix.save()` を `ix.tree = head; ix.save(); return err` にする（失敗しても部分更新のキャッシュを保存する）→ `TestFailedUpdateThenOpenEqualsARebuild` が FAIL（`Open after the failed update = ... want ...`。実測。comment・mentions が再構築と違う）
2. `build.go` の `update` の `var dirs []string` の次の `for ulid := range noted {` を `for ulid := range ix.tasks {` にする（触っていないタスクの一覧も取る）→ `TestUpdateReadsOnlyTheNotesOfTheTouchedTask` が FAIL（実測 `the update read a file of a task it did not touch: <hash>: object not found`）
3. `index.go` の `Open` の `err = ix.update(tree, head)` を `err = gitrepo.ErrTreeNotFound` にする（毎回再構築する）→ 同テストが FAIL（実測 `the update read a file of a task it did not touch: tasks/8/0/....md: file does not exist`）

（参考: 3 つ目の別の壊し方として `update` の「コメントがあるタスクのファイルを読み直す」ループを全タスクにしても、そこは `fs.ErrNotExist` を握りつぶす作りなので検出できない。検出できる壊し方は上の 2 と 3。）

- [ ] **Step 6: コミット**

```bash
git add internal/app/storage/index/build.go internal/app/storage/index/comments.go internal/app/storage/index/helpers_test.go internal/app/storage/index/index_test.go internal/app/storage/index/comments_test.go
git commit -m "refactor(index): 読みの失敗と一覧の確認を object の退避で行う"
```

---

### Task B1: store のロックを普通の引数・オプションにする（`afterLockMiss`・`Store.lockWait`）

外す差し込み口:
1. `store/lock.go` の `afterLockMiss`（パッケージ変数）→ `acquireStore`・`lockFile` の引数 `onMiss func()`。`osfs.LockFile` が既に `onMiss` を持つので、そこへ通すだけ。本番の 3 か所（`local.go` の `WithLocalLock`、`move.go` の `LockInOrder`、`tx.go` の `lockStore`）は `nil` を渡す。
2. `Store.lockWait`（`newStore` が定数を入れ、テストがフィールドを書き換えていた）→ `store.Option` の `WithLockWait(d)`。`Open(dir, opts ...Option)`・`Create(..., now, opts ...Option)` が受ける。store には `Options` 型がないので、可変長の関数オプションにした（呼び出し側は変わらない）。既定は今の 10 秒のまま。

使っていたテスト:
- `afterLockMiss`: `lock_gone_test.go` の TestAcquireStoreWaitingWhileTheLockFileIsRemoved（`once.Do(close)` する関数を `onMiss` 引数に渡す。「直列」の注記が要らなくなる）
- `lockWait`: `tx_test.go` の TestUpdateLockedElsewhere（20ms）、`move_test.go` の TestLockInOrder（20ms）・TestLockInOrderFailureReleasesWhatItTook（10ms）、`merge_locked_test.go` の TestCommitMergeWaitsWithoutHoldingTheMutex（3 秒）、`ids_test.go` の TestConcurrentAddsGetDistinctIDs（1 分。2 つのハンドルの両方に渡す）

テストの作り方の注意（TestLockInOrder）: このテストは前半で「別のハンドルのロック待ちが Release まで待つ（60ms 以上）」を見て、後半で「Release 済みの Locked を指す Op は、他の保持者がいると 20ms で ErrLocked になる」を見る。待ち時間が前半と後半で違うので、`a` は最初から 20ms で作り、前半の「待つ側」だけを、同じディレクトリを既定の待ち時間で開いた `waiter` にする。後半は元どおり `a`（Locked が持つハンドル）で確かめる（`waiter` にすると、Locked が解放済みであることを見なくなる）。

**Files:**
- Modify: `internal/app/storage/store/lock.go`、`local.go`、`move.go`、`tx.go`、`store.go`
- Modify: `internal/app/storage/store/prefix.go`（下の注）
- Modify（テスト）: `lock_gone_test.go`、`helpers_test.go`、`move_test.go`、`tx_test.go`、`merge_locked_test.go`、`ids_test.go`、`lock_test.go`（A1 の `acquire` が `lockFile` の新しい引数を渡す）

**Interfaces:**
- Produces: `type Option func(*Store)`、`func WithLockWait(d time.Duration) Option`、`func Open(dir string, opts ...Option) (*Store, error)`、`func Create(dir string, meta *model.StoreMeta, sc *schema.Schema, ws *model.Workspace, author gitrepo.Signature, account string, now func() time.Time, opts ...Option) (*Store, error)`
- 非公開: `lockFile(path string, wait time.Duration, onMiss func())`、`acquireStore(dir, path string, wait time.Duration, onMiss func())`

- [ ] **Step 1: ロックの引数を通す**

`lock.go`（A1 の後の状態からの diff）:

```diff
--- a/internal/app/storage/store/lock.go
+++ b/internal/app/storage/store/lock.go
@@
-// afterLockMiss is called each time acquire finds the lock taken, with the
-// lock file open; tests use it to know a waiter is blocked.
-var afterLockMiss = func() {}
-
 // lockFile takes the lock on the file at path, made when missing, in a
-// directory that exists.
-func lockFile(path string, wait time.Duration) (*osfs.Lock, error) {
-	return osfs.LockFile(path, wait, lockPoll, func() { afterLockMiss() })
+// directory that exists. onMiss, when set, is called each time the lock is
+// found taken, with the lock file open.
+func lockFile(path string, wait time.Duration, onMiss func()) (*osfs.Lock, error) {
+	return osfs.LockFile(path, wait, lockPoll, onMiss)
 }
@@
-// acquireStore takes the lock file at path in the store at dir. It creates no
+// acquireStore takes the lock file at path in the store at dir; onMiss is
+// lockFile's. It creates no
@@
-func acquireStore(dir, path string, wait time.Duration) (*osfs.Lock, error) {
+func acquireStore(dir, path string, wait time.Duration, onMiss func()) (*osfs.Lock, error) {
@@
-	l, err := lockFile(filepath.Clean(path), wait)
+	l, err := lockFile(filepath.Clean(path), wait, onMiss)
```

（`osfs.LockFile` は `onMiss` が nil でよい。`internal/lib/osfs/lock_test.go` の TestLockFileCallsOnMiss が nil で呼んでいる。）

`lock_test.go`（A1 で置いた `acquire`）の最後の行を `return lockFile(path, wait, nil)` にする。

本番の呼び出し 3 か所に `nil` を足す:

`internal/app/storage/store/local.go`:

```diff
--- a/internal/app/storage/store/local.go
+++ b/internal/app/storage/store/local.go
@@ -94,7 +94,7 @@
 	if err := validLocalName(name); err != nil {
 		return err
 	}
-	l, err := acquireStore(s.dir, s.path(LocalDir+"/"+name), wait)
+	l, err := acquireStore(s.dir, s.path(LocalDir+"/"+name), wait, nil)
 	if err != nil {
 		return err
 	}
```

`internal/app/storage/store/move.go`:

```diff
--- a/internal/app/storage/store/move.go
+++ b/internal/app/storage/store/move.go
@@ -184,7 +184,7 @@
 	})
 	l := &Locked{}
 	for _, s := range uniq {
-		fl, err := acquireStore(s.dir, s.path(lockPath), s.lockWait)
+		fl, err := acquireStore(s.dir, s.path(lockPath), s.lockWait, nil)
 		if err != nil {
 			l.Release()
 			return nil, fmt.Errorf("lock %s: %w", s.meta.Name, err)
```

`internal/app/storage/store/tx.go`:

```diff
--- a/internal/app/storage/store/tx.go
+++ b/internal/app/storage/store/tx.go
@@ -518,7 +518,7 @@
 	deadline := time.Now().Add(s.lockWait)
 	for {
 		s.mu.Lock()
-		l, err := acquireStore(s.dir, s.path(lockPath), 0)
+		l, err := acquireStore(s.dir, s.path(lockPath), 0, nil)
 		if err == nil {
 			return l, nil
 		}
```

- [ ] **Step 2: `WithLockWait` を足す**

`internal/app/storage/store/store.go`:

```diff
--- a/internal/app/storage/store/store.go
+++ b/internal/app/storage/store/store.go
@@ -56,13 +56,23 @@
 	idNumbers IDNumbers // counts id numbers for Update; nil reads every task
 	siblings  Siblings  // finds siblings for Update; nil reads every task
 
+	lockWait time.Duration // how long a write waits for the file lock
+
 	// Replaced by tests.
-	now      func() time.Time
-	lockWait time.Duration
-	commit   func(gitrepo.Commit) (plumbing.Hash, error)
+	now    func() time.Time
+	commit func(gitrepo.Commit) (plumbing.Hash, error)
 }
 
-func newStore(dir string, repo *gitrepo.Repo, meta *model.StoreMeta, sc *schema.Schema) *Store {
+// Option changes how a store is opened or created.
+type Option func(*Store)
+
+// WithLockWait sets how long a write waits for the file lock held by another
+// process before it fails with ErrLocked; the default is 10 seconds.
+func WithLockWait(d time.Duration) Option {
+	return func(s *Store) { s.lockWait = d }
+}
+
+func newStore(dir string, repo *gitrepo.Repo, meta *model.StoreMeta, sc *schema.Schema, opts ...Option) *Store {
 	s := &Store{
 		dir:      dir,
 		repo:     repo,
@@ -72,6 +82,9 @@
 		commit:   repo.Commit,
 	}
 	s.schema.Store(sc)
+	for _, o := range opts {
+		o(s)
+	}
 	return s
 }
 
@@ -86,7 +99,7 @@
 // Hedgehug-Author trailer of the commit. now tells the time of what it
 // writes, there and later.
 // When Create fails it removes what it made, so it can be run again.
-func Create(dir string, meta *model.StoreMeta, sc *schema.Schema, ws *model.Workspace, author gitrepo.Signature, account string, now func() time.Time) (_ *Store, err error) {
+func Create(dir string, meta *model.StoreMeta, sc *schema.Schema, ws *model.Workspace, author gitrepo.Signature, account string, now func() time.Time, opts ...Option) (_ *Store, err error) {
 	if ws == nil {
 		return nil, errors.New("the store needs a workspace")
 	}
@@ -118,7 +131,7 @@
 	if sc == nil {
 		sc = schema.Default()
 	}
-	s := newStore(dir, repo, &m, sc)
+	s := newStore(dir, repo, &m, sc, opts...)
 	s.now = now
 	newStoreHook(s)
 
@@ -211,7 +224,7 @@
 // Open opens the store in dir. It reads store.yaml and schema.yaml now;
 // Meta returns what it read. Update reads schema.yaml again, and Schema
 // returns the schema read last.
-func Open(dir string) (*Store, error) {
+func Open(dir string, opts ...Option) (*Store, error) {
 	meta, err := readDecode(dir, model.StoreMetaPath, model.DecodeStoreMeta)
 	if err != nil {
 		return nil, err
@@ -224,7 +237,7 @@
 	if err != nil {
 		return nil, err
 	}
-	return newStore(dir, repo, meta, sc), nil
+	return newStore(dir, repo, meta, sc, opts...), nil
 }
 
 // SetNow makes the store read the time from now, in place of time.Now, for
```

注: 下の `prefix.go` と（後の）`prefix_test.go` の `openStore` の差分は、この下書きを C4 より前の状態で試したときのもの。C4 で `openStore` は消えているので、**この 2 つの差分は飛ばす**（当てる行がないことを確かめるだけ。`grep -n openStore internal/app/storage/store/*.go` が 0 行）。

`internal/app/storage/store/prefix.go`（飛ばす）:

```diff
--- a/internal/app/storage/store/prefix.go
+++ b/internal/app/storage/store/prefix.go
@@ -17,7 +17,7 @@
 }
 
 // openStore opens a store for PrefixUses; tests count the calls.
-var openStore = Open
+var openStore = func(dir string) (*Store, error) { return Open(dir) }
 
 // PrefixUses opens each of stores once and returns its workspaces whose id
 // prefix is one of prefixes, ignoring case, in the order of stores. A store
```

- [ ] **Step 3: テストを直す**

`newTestStore`・`newMoveStore` がオプションを受け取る:

`internal/app/storage/store/helpers_test.go`:

```diff
--- a/internal/app/storage/store/helpers_test.go
+++ b/internal/app/storage/store/helpers_test.go
@@ -23,10 +23,10 @@
 )
 
 // newTestStore creates a store in a temporary directory, with a fixed clock.
-func newTestStore(t *testing.T) *Store {
+func newTestStore(t *testing.T, opts ...Option) *Store {
 	t.Helper()
 	meta := &model.StoreMeta{ULID: model.NewULID(), Name: "test"}
-	s, err := Create(filepath.Join(t.TempDir(), "store"), meta, nil, &model.Workspace{ULID: testWorkspaceULID, Name: "test", IDPrefix: "TST"}, testAuthor, "", time.Now)
+	s, err := Create(filepath.Join(t.TempDir(), "store"), meta, nil, &model.Workspace{ULID: testWorkspaceULID, Name: "test", IDPrefix: "TST"}, testAuthor, "", time.Now, opts...)
 	if err != nil {
 		t.Fatal(err)
 	}
```

`internal/app/storage/store/lock_gone_test.go`:

```diff
--- a/internal/app/storage/store/lock_gone_test.go
+++ b/internal/app/storage/store/lock_gone_test.go
@@ -28,7 +28,7 @@
 	if err := os.RemoveAll(dir); err != nil {
 		t.Fatal(err)
 	}
-	if _, err := acquireStore(dir, lock, time.Second); !errors.Is(err, ErrStoreGone) {
+	if _, err := acquireStore(dir, lock, time.Second, nil); !errors.Is(err, ErrStoreGone) {
 		t.Fatalf("err = %v; want ErrStoreGone", err)
 	}
 	if _, err := os.Stat(dir); !os.IsNotExist(err) {
@@ -36,23 +36,20 @@
 	}
 }
 
-// Serial: it replaces the package variable afterLockMiss.
-//
 // A writer that waits for the lock while the store is removed does not get
 // the lock of the file that is gone.
 func TestAcquireStoreWaitingWhileTheLockFileIsRemoved(t *testing.T) {
 	dir, lock := fakeStoreDir(t)
-	held, err := acquireStore(dir, lock, time.Second)
+	held, err := acquireStore(dir, lock, time.Second, nil)
 	if err != nil {
 		t.Fatal(err)
 	}
 	blocked := make(chan struct{})
 	var once sync.Once
-	afterLockMiss = func() { once.Do(func() { close(blocked) }) }
-	t.Cleanup(func() { afterLockMiss = func() {} })
+	onMiss := func() { once.Do(func() { close(blocked) }) }
 	errc := make(chan error, 1)
 	go func() {
-		l, err := acquireStore(dir, lock, 5*time.Second)
+		l, err := acquireStore(dir, lock, 5*time.Second, onMiss)
 		if err == nil {
 			_ = l.Release()
 		}
@@ -78,7 +75,7 @@
 	if err := os.RemoveAll(filepath.Join(dir, ".hedgehug")); err != nil {
 		t.Fatal(err)
 	}
-	if _, err := acquireStore(dir, lock, time.Second); !errors.Is(err, ErrStoreGone) {
+	if _, err := acquireStore(dir, lock, time.Second, nil); !errors.Is(err, ErrStoreGone) {
 		t.Fatalf("err = %v; want ErrStoreGone", err)
 	}
 	if _, err := os.Stat(filepath.Join(dir, ".hedgehug")); !os.IsNotExist(err) {
@@ -108,7 +105,7 @@
 func TestAcquireStoreHappyPath(t *testing.T) {
 	t.Parallel()
 	dir, lock := fakeStoreDir(t)
-	l, err := acquireStore(dir, lock, time.Second)
+	l, err := acquireStore(dir, lock, time.Second, nil)
 	if err != nil {
 		t.Fatal(err)
 	}
```

`internal/app/storage/store/tx_test.go`:

```diff
--- a/internal/app/storage/store/tx_test.go
+++ b/internal/app/storage/store/tx_test.go
@@ -239,8 +239,7 @@
 
 func TestUpdateLockedElsewhere(t *testing.T) {
 	t.Parallel()
-	s := newTestStore(t)
-	s.lockWait = 20 * time.Millisecond
+	s := newTestStore(t, WithLockWait(20*time.Millisecond))
 	l, err := acquire(s.path(lockPath), time.Second)
 	if err != nil {
 		t.Fatal(err)
```

`internal/app/storage/store/merge_locked_test.go`:

```diff
--- a/internal/app/storage/store/merge_locked_test.go
+++ b/internal/app/storage/store/merge_locked_test.go
@@ -13,8 +13,7 @@
 // Update, which needs s.mu, goes through at once.
 func TestCommitMergeWaitsWithoutHoldingTheMutex(t *testing.T) {
 	// 直列: パッケージ変数 onLockWait の差し替え
-	s := newTestStore(t)
-	s.lockWait = 3 * time.Second
+	s := newTestStore(t, WithLockWait(3*time.Second))
 	head, err := s.repo.Head()
 	if err != nil {
 		t.Fatal(err)
```

`internal/app/storage/store/ids_test.go`:

```diff
--- a/internal/app/storage/store/ids_test.go
+++ b/internal/app/storage/store/ids_test.go
@@ -141,14 +141,13 @@
 
 func TestConcurrentAddsGetDistinctIDs(t *testing.T) {
 	t.Parallel()
-	s := newTestStore(t)
+	// the test checks that no ID is given twice, not how long a writer waits
+	s := newTestStore(t, WithLockWait(time.Minute))
 	ws := testWorkspace(t, s)
-	other, err := Open(s.dir) // a second handle: only the file lock serializes them
+	other, err := Open(s.dir, WithLockWait(time.Minute)) // a second handle: only the file lock serializes them
 	if err != nil {
 		t.Fatal(err)
 	}
-	// the test checks that no ID is given twice, not how long a writer waits
-	s.lockWait, other.lockWait = time.Minute, time.Minute
 	var wg sync.WaitGroup
 	tasks := make([]*model.Task, 20)
 	for i := range tasks {
```

`internal/app/storage/store/move_test.go`:

```diff
--- a/internal/app/storage/store/move_test.go
+++ b/internal/app/storage/store/move_test.go
@@ -20,11 +20,11 @@
 
 // newMoveStore makes a store with the id prefix prefix and the schema sc
 // (the default when nil). Stores made later have larger ULIDs.
-func newMoveStore(t *testing.T, name, prefix string, sc *schema.Schema) (*Store, *model.Workspace) {
+func newMoveStore(t *testing.T, name, prefix string, sc *schema.Schema, opts ...Option) (*Store, *model.Workspace) {
 	t.Helper()
 	ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: prefix}
 	meta := &model.StoreMeta{ULID: model.NewULID(), Name: name}
-	s, err := Create(filepath.Join(t.TempDir(), name), meta, sc, ws, testAuthor, "", time.Now)
+	s, err := Create(filepath.Join(t.TempDir(), name), meta, sc, ws, testAuthor, "", time.Now, opts...)
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -534,7 +534,7 @@
 
 func TestLockInOrder(t *testing.T) {
 	// 直列: パッケージ変数 afterLock の差し替え
-	a, _ := newMoveStore(t, "a", "AAA", nil)
+	a, _ := newMoveStore(t, "a", "AAA", nil, WithLockWait(20*time.Millisecond))
 	b, _ := newMoveStore(t, "b", "BBB", nil)
 	var order []string
 	afterLock = func(s *Store) { order = append(order, s.meta.Name) }
@@ -548,10 +548,15 @@
 		t.Errorf("locked in the order %v, want a then b", order)
 	}
 
-	// A writer without the hold waits for Release, in this process too.
+	// A writer without the hold waits for Release, in this process too. It
+	// is a handle that waits longer than a does.
+	waiter, err := Open(a.dir)
+	if err != nil {
+		t.Fatal(err)
+	}
 	done := make(chan error, 1)
 	go func() {
-		_, err := a.Update(Op{Name: "x", Author: testAuthor}, func(tx *Tx) error { return tx.PutFile("a.txt", []byte("x")) })
+		_, err := waiter.Update(Op{Name: "x", Author: testAuthor}, func(tx *Tx) error { return tx.PutFile("a.txt", []byte("x")) })
 		done <- err
 	}()
 	select {
@@ -582,7 +587,6 @@
 	}
 	// An Op that still names the released Locked takes the file lock too:
 	// with another holder on it, it waits and fails.
-	a.lockWait = 20 * time.Millisecond
 	other, err := acquire(a.path(lockPath), time.Second)
 	if err != nil {
 		t.Fatal(err)
@@ -596,8 +600,7 @@
 func TestLockInOrderFailureReleasesWhatItTook(t *testing.T) {
 	t.Parallel()
 	a, _ := newMoveStore(t, "a", "AAA", nil)
-	b, _ := newMoveStore(t, "b", "BBB", nil)
-	b.lockWait = 10 * time.Millisecond
+	b, _ := newMoveStore(t, "b", "BBB", nil, WithLockWait(10*time.Millisecond))
 	other, err := acquire(b.path(lockPath), time.Second) // another process holds b
 	if err != nil {
 		t.Fatal(err)
```

`internal/app/storage/store/prefix_test.go`（飛ばす。C4 で書き直し済み）:

```diff
--- a/internal/app/storage/store/prefix_test.go
+++ b/internal/app/storage/store/prefix_test.go
@@ -10,7 +10,7 @@
 	s := newTestStore(t) // workspace "test" with the prefix TST
 	opened := 0
 	openStore = func(dir string) (*Store, error) { opened++; return Open(dir) }
-	defer func() { openStore = Open }()
+	defer func() { openStore = func(dir string) (*Store, error) { return Open(dir) } }()
 	var skipped []string
 	uses := PrefixUses([]NamedStore{
 		{Name: "gone", Path: filepath.Join(t.TempDir(), "gone")},
```

- [ ] **Step 4: ビルドとテストを回す**

Run: `go build ./... && go vet ./...`
Run: `go test -count=1 ./internal/app/storage/store`
Expected: PASS

- [ ] **Step 5: 差し込み口が消えたことを確かめる**

Run: `grep -rn 'afterLockMiss\|\.lockWait = \|lockWait, other' internal --include='*.go'`
Expected: 出力なし（`s.lockWait` を読む行は残る）

- [ ] **Step 6: コミット**

```bash
git add internal/app/storage/store
git commit -m "refactor(store): ロックの待ち時間と待ちの通知を、オプションと引数で渡す"
```

---

### Task B2: `config` の OS 名を引数で渡す（`goos`）

外す差し込み口: `internal/app/storage/config/paths.go` の `goos`（`runtime.GOOS` を入れたパッケージ変数。テストが書き換えていた）。
使っていたテスト: `paths_test.go` の TestResolve（6 つのサブケース）、TestResolveNeedsHome。「直列」の注記が要らなくなる。

`Resolve(getenv)` の形は変えず、中身を非公開の `resolve(goos, getenv)` にして、`Resolve` が `runtime.GOOS` を渡す。

**Files:**
- Modify: `internal/app/storage/config/paths.go`
- Modify（テスト）: `internal/app/storage/config/paths_test.go`

**Interfaces:**
- Produces: `func resolve(goos string, getenv func(string) string) (Paths, error)`、`func defaultConfigDir(goos, home string) string`（どちらも非公開）

- [ ] **Step 1: 本体を直す**

`internal/app/storage/config/paths.go`:

```diff
--- a/internal/app/storage/config/paths.go
+++ b/internal/app/storage/config/paths.go
@@ -8,9 +8,6 @@
 	"runtime"
 )
 
-// goos is runtime.GOOS; tests replace it.
-var goos = runtime.GOOS
-
 // Paths is where the settings and the stores are, as absolute paths.
 type Paths struct {
 	Home       string // the user's home directory
@@ -24,6 +21,11 @@
 // its own, then the defaults of the OS. Relative values are ignored, as the
 // XDG spec says.
 func Resolve(getenv func(string) string) (Paths, error) {
+	return resolve(runtime.GOOS, getenv)
+}
+
+// resolve is Resolve for the OS named goos (a runtime.GOOS value).
+func resolve(goos string, getenv func(string) string) (Paths, error) {
 	h, err := home(getenv)
 	if err != nil {
 		return Paths{}, err
@@ -37,7 +39,7 @@
 	}
 	cfg := abs(getenv("XDG_CONFIG_HOME"))
 	if cfg == "" {
-		cfg = defaultConfigDir(h)
+		cfg = defaultConfigDir(goos, h)
 	}
 	data := abs(getenv("XDG_DATA_HOME"))
 	if data == "" {
@@ -58,7 +60,7 @@
 	return h, nil
 }
 
-func defaultConfigDir(home string) string {
+func defaultConfigDir(goos, home string) string {
 	if goos == "darwin" {
 		return filepath.Join(home, "Library", "Application Support")
 	}
```

- [ ] **Step 2: テストを直す**

`internal/app/storage/config/paths_test.go`:

```diff
--- a/internal/app/storage/config/paths_test.go
+++ b/internal/app/storage/config/paths_test.go
@@ -10,7 +10,6 @@
 	return func(k string) string { return m[k] }
 }
 
-// 直列: パッケージ変数 goos の差し替え
 func TestResolve(t *testing.T) {
 	h := filepath.Join(t.TempDir(), "home")
 	hh := filepath.Join(t.TempDir(), "hh")
@@ -55,10 +54,7 @@
 		},
 	} {
 		t.Run(tc.name, func(t *testing.T) {
-			old := goos
-			goos = tc.goos
-			defer func() { goos = old }()
-			p, err := Resolve(envOf(tc.env))
+			p, err := resolve(tc.goos, envOf(tc.env))
 			if err != nil {
 				t.Fatal(err)
 			}
@@ -75,12 +71,8 @@
 	}
 }
 
-// 直列: パッケージ変数 goos の差し替え
 func TestResolveNeedsHome(t *testing.T) {
-	old := goos
-	goos = "linux"
-	defer func() { goos = old }()
-	if _, err := Resolve(envOf(nil)); err == nil {
+	if _, err := resolve("linux", envOf(nil)); err == nil {
 		t.Error("Resolve with no HOME succeeded")
 	}
 }
```

- [ ] **Step 3: テストを回す**

Run: `go vet ./internal/app/storage/config && go test -count=1 ./internal/app/storage/config`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add internal/app/storage/config
git commit -m "refactor(config): OS 名を引数で渡す"
```

---

### Task B3: `gitrepo` のパックの上限を引数で渡す（`packCap`）

外す差し込み口: `internal/lib/gitrepo/maintain.go` の `packCap`（`PackCap` を入れたパッケージ変数。テストが書き換えていた）。
方針: `SmallPacks()`・`Consolidate()` の形は変えず（本番の呼び出し側の `replicate/sync.go` と、ほかのテスト・ベンチは変えない）、上限を渡せる `SmallPacksBelow(packCap)`・`ConsolidateBelow(packCap)` を足し、前の 2 つは `PackCap` を渡して呼ぶ。内部の `smallPacks` も上限を引数に取る。
使っていたテスト: `maintain_test.go` の TestConsolidateLeavesPacksFromPackCap、TestConsolidateMergesAboutPackCapAtMost。「パッケージ変数を書き換えるので並列にしない」という制約がなくなるので `t.Parallel()` を付ける。

**Files:**
- Modify: `internal/lib/gitrepo/maintain.go`
- Modify（テスト）: `internal/lib/gitrepo/maintain_test.go`

**Interfaces:**
- Produces: `func (r *Repo) SmallPacksBelow(packCap int64) (int, error)`、`func (r *Repo) ConsolidateBelow(packCap int64) error`。`SmallPacks()`・`Consolidate()` は `PackCap` を渡す今のまま。

- [ ] **Step 1: 本体を直す**

`internal/lib/gitrepo/maintain.go`:

```diff
--- a/internal/lib/gitrepo/maintain.go
+++ b/internal/lib/gitrepo/maintain.go
@@ -26,17 +26,14 @@
 // PackCap is the size from which a pack is left as it is.
 const PackCap = 64 << 20
 
-// packCap is PackCap; tests lower it.
-var packCap int64 = PackCap
-
-// smallPack is a pack below packCap, by the path of its .idx.
+// smallPack is a pack below a cap, by the path of its .idx.
 type smallPack struct {
 	idx  string
 	size int64
 }
 
 // smallPacks lists the packs below packCap, the smallest first.
-func (r *Repo) smallPacks() ([]smallPack, error) {
+func (r *Repo) smallPacks(packCap int64) ([]smallPack, error) {
 	idxs, err := filepath.Glob(filepath.Join(r.dir, ".git", "objects", "pack", "pack-*.idx"))
 	if err != nil {
 		return nil, err
@@ -65,7 +62,12 @@
 
 // SmallPacks counts the packs smaller than PackCap.
 func (r *Repo) SmallPacks() (int, error) {
-	ps, err := r.smallPacks()
+	return r.SmallPacksBelow(PackCap)
+}
+
+// SmallPacksBelow counts the packs smaller than packCap.
+func (r *Repo) SmallPacksBelow(packCap int64) (int, error) {
+	ps, err := r.smallPacks(packCap)
 	return len(ps), err
 }
 
@@ -124,11 +126,18 @@
 // made are removed. An old pack that cannot be removed (Windows, open file)
 // is left for a later Consolidate.
 func (r *Repo) Consolidate() error {
+	return r.ConsolidateBelow(PackCap)
+}
+
+// ConsolidateBelow is Consolidate with packCap in place of PackCap: packs of
+// packCap or more are left as they are, and one run merges about packCap at
+// most.
+func (r *Repo) ConsolidateBelow(packCap int64) error {
 	los, ok := r.r.Storer.(storer.LooseObjectStorer)
 	if !ok {
 		return errors.New("the object storage of the repository cannot be consolidated")
 	}
-	packs, err := r.smallPacks()
+	packs, err := r.smallPacks(packCap)
 	if err != nil {
 		return err
 	}
```

- [ ] **Step 2: テストを直す**

`packSizes` は既定の上限を使う（`r.smallPacks(PackCap)`）。2 つのテストの `packCap = …` の代入は、局所の定数・変数にして `…Below(packCap)` へ渡す。

`internal/lib/gitrepo/maintain_test.go`:

```diff
--- a/internal/lib/gitrepo/maintain_test.go
+++ b/internal/lib/gitrepo/maintain_test.go
@@ -86,7 +86,7 @@
 // smallest first.
 func packSizes(t *testing.T, r *Repo) []int64 {
 	t.Helper()
-	ps, err := r.smallPacks()
+	ps, err := r.smallPacks(PackCap)
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -243,22 +243,20 @@
 	}
 }
 
-// The tests below replace package variables, so they are not parallel.
-
 func TestConsolidateLeavesPacksFromPackCap(t *testing.T) {
+	t.Parallel()
 	dir := t.TempDir()
 	r, _ := Init(dir)
 	for i := range 3 {
 		addPack(t, r, i)
 	}
 	before := packFiles(t, dir)
-	packCap = 1 // every pack is "large" now
-	t.Cleanup(func() { packCap = PackCap })
-	if n, _ := r.SmallPacks(); n != 0 {
+	const packCap = 1 // every pack is "large" now
+	if n, _ := r.SmallPacksBelow(packCap); n != 0 {
 		t.Fatalf("SmallPacks = %d, want 0", n)
 	}
 	commitFiles(t, r, dir, map[string]string{"loose.md": "x"})
-	if err := r.Consolidate(); err != nil {
+	if err := r.ConsolidateBelow(packCap); err != nil {
 		t.Fatal(err)
 	}
 	after := packFiles(t, dir)
@@ -427,8 +425,7 @@
 }
 
 func TestConsolidateMergesAboutPackCapAtMost(t *testing.T) {
-	// replaces packCap: not parallel
-	t.Cleanup(func() { packCap = PackCap })
+	t.Parallel()
 	three := func() (*Repo, string) {
 		dir := t.TempDir()
 		r, _ := Init(dir)
@@ -441,11 +438,11 @@
 	// two fit under the cap, three do not: two are merged
 	r, dir := three()
 	sz := wantPick(t, r, 3)
-	packCap = sz[0] + sz[1] + 1
+	packCap := sz[0] + sz[1] + 1
 	if sz[2] >= packCap {
 		t.Fatalf("a pack is over the cap: %v, %d", sz, packCap)
 	}
-	if err := r.Consolidate(); err != nil {
+	if err := r.ConsolidateBelow(packCap); err != nil {
 		t.Fatal(err)
 	}
 	// counted by the files: the merged pack may come out at the cap or under it
@@ -457,7 +454,6 @@
 	}
 
 	// not even two fit under the cap: nothing is merged
-	packCap = PackCap
 	r, dir = three()
 	sz = wantPick(t, r, 3)
 	packCap = sz[2] + 1
@@ -465,7 +461,7 @@
 		t.Fatalf("two packs fit under the cap: %v, %d", sz, packCap)
 	}
 	before := packFiles(t, dir)
-	if err := r.Consolidate(); err != nil {
+	if err := r.ConsolidateBelow(packCap); err != nil {
 		t.Fatal(err)
 	}
 	if after := packFiles(t, dir); !slices.Equal(before, after) {
```

- [ ] **Step 3: テストを回す**

Run: `go vet ./internal/lib/gitrepo && go test -count=1 ./internal/lib/gitrepo -run 'Consolidate|SmallPacks'`
Expected: PASS
Run: `go test -count=1 ./internal/lib/gitrepo`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add internal/lib/gitrepo
git commit -m "refactor(gitrepo): パックの上限を引数で渡せるようにする"
```

---

### Task B4: `auth` のロックの待ち時間を `Options` に置く（`lockWait`）

外す差し込み口: `internal/app/remote/auth/device.go` の `lockWait`（30 秒を入れたパッケージ変数）。
`auth.Options` に `LockWait time.Duration` を足す。ゼロ値は今の 30 秒（`defaultLockWait`）。本番の呼び出し側（usecase が `auth.Options` を作る所）は何も足さない（既定を使う）。
使っていたテスト: `device_test.go` の TestDeviceRefreshLockBusy、`minor_test.go` の TestWithLock（どちらも 5ms で渡す。「直列」の注記が要らなくなるので `t.Parallel()` を付ける）。

**Files:**
- Modify: `internal/app/remote/auth/device.go`、`source.go`
- Modify（テスト）: `internal/app/remote/auth/device_test.go`、`minor_test.go`

**Interfaces:**
- Produces: `auth.Options.LockWait time.Duration`（ゼロ値は 30 秒）

- [ ] **Step 1: 本体を直す**

`internal/app/remote/auth/device.go`:

```diff
--- a/internal/app/remote/auth/device.go
+++ b/internal/app/remote/auth/device.go
@@ -15,8 +15,9 @@
 // refreshBefore is how long before its end an access token is refreshed.
 const refreshBefore = 5 * time.Minute
 
-// lockWait is how long a refresh waits for another one to end.
-var lockWait = 30 * time.Second
+// defaultLockWait is how long a refresh waits for another one to end, when
+// Options.LockWait is not set.
+const defaultLockWait = 30 * time.Second
 
 // Prompt shows the user the code and where to enter it.
 type Prompt func(dc *github.DeviceCode)
@@ -93,7 +94,7 @@
 	if opt.LockFile == "" {
 		return errors.New("auth: LockFile is not set")
 	}
-	err := osfs.WithFileLock(opt.LockFile, lockWait, fn)
+	err := osfs.WithFileLock(opt.LockFile, opt.lockWait(), fn)
 	if errors.Is(err, osfs.ErrLocked) {
 		return fmt.Errorf("another hedgehug is refreshing or changing the token; wait a little and try again: %w", err)
 	}
```

`internal/app/remote/auth/source.go`:

```diff
--- a/internal/app/remote/auth/source.go
+++ b/internal/app/remote/auth/source.go
@@ -33,9 +33,10 @@
 	Run      func(name string, args ...string) (stdout, stderr []byte, err error) // nil runs the command
 	Keyring  Keyring                                                              // nil is the OS keychain
 	Now      func() time.Time
-	Host     config.Host  // of the account, for the device flow
-	HTTP     *http.Client // nil is a client with a 30-second timeout
-	LockFile string       // auth.lock; a refresh holds it
+	Host     config.Host   // of the account, for the device flow
+	HTTP     *http.Client  // nil is a client with a 30-second timeout
+	LockFile string        // auth.lock; a refresh holds it
+	LockWait time.Duration // how long a refresh waits for the lock; zero is 30 seconds
 	Sleep    func(time.Duration)
 	Context  context.Context // nil is never canceled; the device flow's wait ends with ErrCanceled when it ends
 }
@@ -49,6 +50,13 @@
 	return o.Keyring
 }
 
+func (o Options) lockWait() time.Duration {
+	if o.LockWait > 0 {
+		return o.LockWait
+	}
+	return defaultLockWait
+}
+
 func (o Options) now() time.Time {
 	if o.Now != nil {
 		return o.Now()
```

- [ ] **Step 2: テストを直す**

`internal/app/remote/auth/device_test.go`:

```diff
--- a/internal/app/remote/auth/device_test.go
+++ b/internal/app/remote/auth/device_test.go
@@ -405,17 +405,14 @@
 	}
 }
 
-// 直列: パッケージ変数 lockWait の差し替え
 func TestDeviceRefreshLockBusy(t *testing.T) {
-	old := lockWait
-	lockWait = 5 * time.Millisecond
-	t.Cleanup(func() { lockWait = old })
+	t.Parallel()
 	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
 	a := config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: "device"}}
 	k := newMem()
 	_ = SaveDeviceToken(a, DeviceToken{AccessToken: "x", RefreshToken: "r", ExpiresAt: now}, k)
 	lock := filepath.Join(t.TempDir(), "auth.lock")
-	opt := Options{Keyring: k, Host: config.Host{Web: "http://127.0.0.1:1", ClientID: "Iv23x"}, Now: func() time.Time { return now }, LockFile: lock}
+	opt := Options{Keyring: k, Host: config.Host{Web: "http://127.0.0.1:1", ClientID: "Iv23x"}, Now: func() time.Time { return now }, LockFile: lock, LockWait: 5 * time.Millisecond}
 	var err error
 	if lerr := osfs.WithFileLock(lock, time.Second, func() error {
 		_, err = Token(a, opt)
```

`internal/app/remote/auth/minor_test.go`:

```diff
--- a/internal/app/remote/auth/minor_test.go
+++ b/internal/app/remote/auth/minor_test.go
@@ -86,8 +86,8 @@
 	}
 }
 
-// 直列: パッケージ変数 lockWait の差し替え
 func TestWithLock(t *testing.T) {
+	t.Parallel()
 	if err := WithLock(Options{}, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "LockFile") {
 		t.Errorf("no lock file = %v", err)
 	}
@@ -100,12 +100,9 @@
 	if err := WithLock(Options{LockFile: lock}, func() error { return want }); !errors.Is(err, want) {
 		t.Errorf("WithLock = %v, want fn's error", err)
 	}
-	old := lockWait
-	lockWait = 5 * time.Millisecond
-	t.Cleanup(func() { lockWait = old })
 	var busy error
 	if err := osfs.WithFileLock(lock, time.Second, func() error {
-		busy = WithLock(Options{LockFile: lock}, func() error { return nil })
+		busy = WithLock(Options{LockFile: lock, LockWait: 5 * time.Millisecond}, func() error { return nil })
 		return nil
 	}); err != nil {
 		t.Fatal(err)
```

- [ ] **Step 3: テストを回す**

Run: `go vet ./internal/app/remote/auth && go test -count=1 ./internal/app/remote/auth`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add internal/app/remote/auth
git commit -m "refactor(auth): ロックの待ち時間を Options で渡す"
```

---

### Task B5: `replicate.Options` に再試行の待ち・repack の閾値を置く（`retryWait`・`repackAt`・`packsAt`）

外す差し込み口: `internal/app/remote/replicate/sync.go` の `retryWait`・`repackAt`・`packsAt`（パッケージ変数）。
`replicate.Options` に `RetryWait time.Duration`・`RepackAt int`・`PacksAt int` を足す。ゼロ値は今の既定（500ms・6700・16）。本番の呼び出し側（`usecase/session.go` の `replicate.Options{…}`）は何も足さない。
使っていたテスト:
- `testmain_test.go`: 全テストに `retryWait = 1ms` を入れていた → `helpers_test.go` の `cluster.add` が作る `Options` に `RetryWait: time.Millisecond` を入れる（`newCluster` で作るクラスタが対象。`TestPushRoundsRunOut` ほか、拒まれた push をやり直すテストはどれもクラスタを使う）。`testmain_test.go` は `testenv.Main(m)` だけになる
- `round_test.go` の TestConsolidateByLooseObjects・TestConsolidateByPackCount: `replace(t, &repackAt, …)` を `c.opts[0].RepackAt = …` に

注意（TestConsolidateByPackCount の作り方が変わる）: 元のテストは `repackAt = 0`・`packsAt = 0` で「小さいパックが 1 つでもあれば repack」を作っていたが、ゼロ値は既定を意味するので `0` を渡せない。`RepackAt = 1`（ルーズオブジェクトが 1 を超えたら）にして、パックの数は `PacksAt = 1`（パックが 2 つ以上）で作る。そのため、パックを 2 つ作る手順（`RepackAt = 1` で `a` と `b` を続けて sync する。`Consolidate` は小さいパックが 1 つだけだと新しいパックに loose だけを入れる）を足した。確かめる内容は同じ（上限の両方より下なら何もしない、パックの数が上限を超えたら repack する）。

**Files:**
- Modify: `internal/app/remote/replicate/sync.go`
- Modify（テスト）: `internal/app/remote/replicate/helpers_test.go`、`testmain_test.go`、`round_test.go`

**Interfaces:**
- Produces: `replicate.Options.RetryWait time.Duration`・`RepackAt int`・`PacksAt int`（ゼロ値は既定）

- [ ] **Step 1: 本体を直す**

`internal/app/remote/replicate/sync.go`:

```diff
--- a/internal/app/remote/replicate/sync.go
+++ b/internal/app/remote/replicate/sync.go
@@ -49,6 +49,11 @@
 	GitHub     *GitHubRepo  // origin on GitHub; nil when it is not
 	Explicit   bool         // hh sync, not the background one: a push_denied is tried again
 
+	// The zero value of each is its default.
+	RetryWait time.Duration // the first wait after a rejected push; it doubles (default 500ms)
+	RepackAt  int           // consolidate when there are more loose objects (default 6700)
+	PacksAt   int           // or more packs below gitrepo.PackCap (default 16)
+
 	// origin, when set, is the commit the repair takes as the main of origin
 	// (the replay after a history rewrite, where it is not the one tracked).
 	origin plumbing.Hash
@@ -87,13 +92,24 @@
 
 // Replaced by tests.
 var (
-	retryWait    = 500 * time.Millisecond // the first wait after a rejected push; it doubles
-	repackAt     = 6700                   // consolidate when there are more loose objects
-	packsAt      = 16                     // or more packs below gitrepo.PackCap
-	beforeCommit = func() {}              // runs before each fast-forward, merge or repair commit
-	onFullRepair = func() {}              // runs when the repair reads every file of HEAD
+	beforeCommit = func() {} // runs before each fast-forward, merge or repair commit
+	onFullRepair = func() {} // runs when the repair reads every file of HEAD
+)
+
+const (
+	defaultRetryWait = 500 * time.Millisecond
+	defaultRepackAt  = 6700
+	defaultPacksAt   = 16
 )
 
+// orDefault is v, or def when v is zero.
+func orDefault[T int | time.Duration](v, def T) T {
+	if v == 0 {
+		return def
+	}
+	return v
+}
+
 // Run syncs the store once. A store with no remote does nothing. Whether
 // it succeeds or not, it then fills the inbox from what HEAD gained.
 func Run(s *store.Store, opt Options) (res Result, err error) {
@@ -159,7 +175,7 @@
 	if err := r.RegisterMergeDriver(); err != nil {
 		return res, err
 	}
-	wait := retryWait
+	wait := orDefault(opt.RetryWait, defaultRetryWait)
 	// A mirror that cannot be reached is left out, and told of as a
 	// warning; origin is the remote the sync is for.
 	down := map[string]error{}
@@ -258,7 +274,7 @@
 	warn("count the loose objects", err)
 	packs, perr := r.SmallPacks()
 	warn("count the packs", perr)
-	if (err == nil && n > repackAt) || (perr == nil && packs > packsAt) {
+	if (err == nil && n > orDefault(opt.RepackAt, defaultRepackAt)) || (perr == nil && packs > orDefault(opt.PacksAt, defaultPacksAt)) {
 		warn("consolidate the packs", r.Consolidate())
 	}
 	return res, nil
```

（`orDefault` は 2 種類の型で 3 回使うだけの小さな関数。一か所でしか使わない抽象にしないため、ジェネリクスの型を `int | time.Duration` に絞った。）

- [ ] **Step 2: テストを直す**

`internal/app/remote/replicate/helpers_test.go`:

```diff
--- a/internal/app/remote/replicate/helpers_test.go
+++ b/internal/app/remote/replicate/helpers_test.go
@@ -75,6 +75,7 @@
 	c.opts = append(c.opts, Options{
 		Remotes: []string{c.remote}, Author: gitrepo.Signature{Name: fmt.Sprint("u", i), Email: fmt.Sprintf("u%d@users.noreply.github.com", i)},
 		Account: acct, Me: []string{acct}, Now: time.Now,
+		RetryWait: time.Millisecond, // a rejected push is tried again at once
 	})
 }
 
```

`internal/app/remote/replicate/testmain_test.go`:

```diff
--- a/internal/app/remote/replicate/testmain_test.go
+++ b/internal/app/remote/replicate/testmain_test.go
@@ -2,13 +2,10 @@
 
 import (
 	"testing"
-	"time"
 
 	"github.com/kukv/hedgehug/internal/lib/testenv"
 )
 
 func TestMain(m *testing.M) {
-	// A rejected push is tried again at once (it waits 500ms, doubling).
-	retryWait = time.Millisecond
 	testenv.Main(m)
 }
```

`internal/app/remote/replicate/round_test.go`:

```diff
--- a/internal/app/remote/replicate/round_test.go
+++ b/internal/app/remote/replicate/round_test.go
@@ -219,7 +219,7 @@
 
 func TestConsolidateByLooseObjects(t *testing.T) {
 	c := newCluster(t, 1)
-	replace(t, &repackAt, 0)
+	c.opts[0].RepackAt = 1 // consolidate with more than one loose object
 	c.addTask(0, "a")
 	r := c.sync(0)
 	if n, err := c.stores[0].Repo().LooseObjects(); err != nil || n != 0 || len(r.Warnings) != 0 {
@@ -229,22 +229,24 @@
 
 func TestConsolidateByPackCount(t *testing.T) {
 	c := newCluster(t, 1)
-	replace(t, &repackAt, 0)
+	c.opts[0].RepackAt = 1 // consolidate with more than one loose object
 	c.addTask(0, "a")
 	c.sync(0)
-	if n, err := c.stores[0].Repo().SmallPacks(); err != nil || n == 0 {
-		t.Fatalf("packs %d, %v", n, err)
+	c.addTask(0, "b")
+	c.sync(0) // the loose objects of b make a second pack; the first is left
+	if n, err := c.stores[0].Repo().SmallPacks(); err != nil || n != 2 {
+		t.Fatalf("packs %d, %v; want 2", n, err)
 	}
 	// few loose objects, and as many packs as the limit: nothing happens
-	replace(t, &repackAt, 1<<20)
-	replace(t, &packsAt, 1<<20)
-	c.addTask(0, "b")
+	c.opts[0].RepackAt = 1 << 20
+	c.opts[0].PacksAt = 1 << 20
+	c.addTask(0, "c")
 	c.sync(0)
 	if n, _ := c.stores[0].Repo().LooseObjects(); n == 0 {
 		t.Fatal("consolidated below both limits")
 	}
 	// more packs than the limit
-	replace(t, &packsAt, 0)
+	c.opts[0].PacksAt = 1
 	r := c.sync(0)
 	if n, err := c.stores[0].Repo().LooseObjects(); err != nil || n != 0 || len(r.Warnings) != 0 {
 		t.Errorf("loose objects %d, %v; warnings %v", n, err, r.Warnings)
```

- [ ] **Step 3: テストを回す**

Run: `go vet ./internal/app/remote/replicate ./internal/app/usecase/... && go test -count=1 ./internal/app/remote/replicate -run 'Consolidate|PushRounds|Reject'`
Expected: PASS
Run: `go test -count=1 ./internal/app/remote/replicate`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add internal/app/remote/replicate
git commit -m "refactor(replicate): 再試行の待ちと repack の閾値を Options で渡す"
```

---

### Task B11: doctor の hedgehug 探しに PATH を渡す（`lookPath`）

外す差し込み口: `maintenance.lookPath`（パッケージ変数）と、`helpers_test.go` の `init()` による全 doctor テストの差し替え。
使っていたテスト: `TestMergeDriverChecks`（`lookPath` を 2 回書き換え）、`init()` 経由で `maintenance` パッケージの全テスト（`run` ヘルパ・`Run` を直に呼ぶ `doctor_test.go` の 1 件・`Doctor` を呼ぶ外部テスト）。
`cli` のテストは、これまで **プロセスの** PATH（`TestMain` が先頭に hedgehug を足したもの）を `exec.LookPath` が読んでいた。`Run` が `Session.Getenv` から PATH を読むようになるので、`testEnv.getenv` が PATH を返すようにする。

**Files:**
- Modify: `internal/lib/osfs/proc.go`（`LookPath` を足す）
- Modify: `internal/app/usecase/maintenance/doctor.go`、`internal/app/usecase/maintenance/maintain.go`
- Test: `internal/lib/osfs/proc_test.go`、`internal/app/usecase/maintenance/{helpers_test,doctor_test,maintain_test}.go`、`internal/app/cli/{helpers_test,doctor_test}.go`

**Interfaces:**
- Produces: `osfs.LookPath(name, path string) (string, error)`（`exec.LookPath` と同じ探し方を、PATH を引数に取る形で。実行ビットのある通常ファイルを、`path` の各ディレクトリから順に探す。空の要素は `.`。見つからなければ `*exec.Error{Err: exec.ErrNotFound}`）
- Produces: `maintenance.Run(dir string, getenv func(string) string) ([]usecase.Finding, error)`（`getenv` は PATH を読むだけ。呼び出し側は `Doctor` の 1 か所で、`ss.Getenv` を渡す）
- Consumes: 既存の `usecase.Session.Getenv`

- [ ] **Step 1: osfs に LookPath を足してテストを書く**

`exec.LookPath` は `os.Getenv("PATH")` を読む。osfs は OS に触る層なので、PATH を引数に取る版をここに置く。`os.Stat` は forbidigo の対象外。Windows の PATHEXT は扱わない（osfs の `files_unix.go` は `//go:build unix` で、本番は unix だけが対象）。

```diff
--- a/internal/lib/osfs/proc.go
+++ b/internal/lib/osfs/proc.go
@@ -1,9 +1,29 @@
 package osfs
 
-import "os/exec"
+import (
+	"os"
+	"os/exec"
+	"path/filepath"
+)
 
 // Command is exec.Command: name run with args, which the caller sets up and
 // runs.
 func Command(name string, args ...string) *exec.Cmd {
 	return exec.Command(name, args...) //nolint:gosec // G204: callers choose the program and its arguments
 }
+
+// LookPath finds the executable called name in the directories of path, a
+// list in the form of the PATH variable. It is exec.LookPath with the list
+// given instead of read from the process.
+func LookPath(name, path string) (string, error) {
+	for _, dir := range filepath.SplitList(path) {
+		if dir == "" {
+			dir = "." // as in a shell, an empty entry is the current directory
+		}
+		p := filepath.Join(dir, name)
+		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0 {
+			return p, nil
+		}
+	}
+	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
+}
```

テスト（`proc_test.go` の末尾に足す。temp ディレクトリに実行ファイルと、実行ビットのないファイルを置く）:

```diff
--- a/internal/lib/osfs/proc_test.go
+++ b/internal/lib/osfs/proc_test.go
@@ -46,3 +46,27 @@ func TestStartDetached(t *testing.T) {
 		time.Sleep(10 * time.Millisecond)
 	}
 }
+
+func TestLookPath(t *testing.T) {
+	t.Parallel()
+	bin, other := t.TempDir(), t.TempDir()
+	if err := os.WriteFile(filepath.Join(bin, "tool"), []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // a stand-in for a program
+		t.Fatal(err)
+	}
+	if err := os.WriteFile(filepath.Join(other, "plain"), []byte("x"), 0o600); err != nil {
+		t.Fatal(err)
+	}
+	path := other + string(os.PathListSeparator) + bin
+	if got, err := LookPath("tool", path); err != nil || got != filepath.Join(bin, "tool") {
+		t.Errorf("LookPath(tool) = %q, %v; want the file in %s", got, err, bin)
+	}
+	if _, err := LookPath("plain", path); err == nil {
+		t.Error("a file that is not executable was found")
+	}
+	if _, err := LookPath("tool", other); err == nil {
+		t.Error("tool was found on a PATH that does not hold it")
+	}
+	if _, err := LookPath("tool", ""); err == nil {
+		t.Error("tool was found on an empty PATH")
+	}
+}
```

- [ ] **Step 2: Run を getenv 引数にして、パッケージ変数を消す**

`doctor.go`: `var lookPath = exec.LookPath` と `"os/exec"` の import を消し、`Run` と `checkMergeDriver` に PATH を通す。`maintain.go`: `Doctor` の呼び出し側が `ss.Getenv` を渡す。

```diff
--- a/internal/app/usecase/maintenance/doctor.go
+++ b/internal/app/usecase/maintenance/doctor.go
@@ -10,7 +10,6 @@ import (
 	"io/fs"
 	"maps"
 	"os"
-	"os/exec"
 	"path"
 	"path/filepath"
 	"slices"
@@ -30,9 +29,6 @@ import (
 	"github.com/kukv/hedgehug/internal/lib/plural"
 )
 
-// lookPath finds hedgehug on PATH; tests replace it.
-var lookPath = exec.LookPath
-
 // fileRoots are the directories of a store whose files doctor reads.
 var fileRoots = []string{"tasks", "comments", "worklogs", "sprints", "releases", "templates", "workspaces", "views"}
 
@@ -45,8 +41,9 @@ var indexCodes = []string{
 
 // Run checks the store in dir without opening it with store.Open, which
 // refuses a store whose schema.yaml is broken. The findings are sorted. The
-// error is for files that cannot be read at all.
-func Run(dir string) ([]usecase.Finding, error) {
+// error is for files that cannot be read at all. getenv gives PATH, on which
+// hedgehug is looked for.
+func Run(dir string, getenv func(string) string) ([]usecase.Finding, error) {
 	var out []usecase.Finding
 
 	// 1. store.yaml and schema.yaml.
@@ -126,7 +123,7 @@ func Run(dir string) ([]usecase.Finding, error) {
 			})
 		}
 	}
-	out = append(out, checkMergeDriver(dir, repo)...)
+	out = append(out, checkMergeDriver(dir, repo, getenv("PATH"))...)
 	out = append(out, checkInbox(dir)...)
 	if err := repo.Verify(); err != nil {
 		out = append(out, usecase.Finding{Check: "git", Where: ".git", Msg: "a git object cannot be read: " + err.Error()})
@@ -143,7 +140,7 @@ func Run(dir string) ([]usecase.Finding, error) {
 // checkMergeDriver finds a store whose .gitattributes send files to the
 // hedgehug merge driver while .git/config does not register it, or while
 // hedgehug cannot be found on PATH for git to run it.
-func checkMergeDriver(dir string, repo *gitrepo.Repo) []usecase.Finding {
+func checkMergeDriver(dir string, repo *gitrepo.Repo, path string) []usecase.Finding {
 	data, err := osfs.ReadFile(filepath.Join(dir, store.AttributesPath))
 	if err != nil || !slices.ContainsFunc(strings.Split(string(data), "\n"), func(l string) bool {
 		return !strings.HasPrefix(strings.TrimSpace(l), "#") && slices.Contains(strings.Fields(l), "merge=hedgehug")
@@ -162,7 +159,7 @@ func checkMergeDriver(dir string, repo *gitrepo.Repo) []usecase.Finding {
 			Hint: "run hedgehug sync once",
 		}}
 	}
-	if _, err := lookPath("hedgehug"); err != nil {
+	if _, err := osfs.LookPath("hedgehug", path); err != nil {
 		return []usecase.Finding{{
 			Check: "merge_driver", Where: where,
 			Msg:  "hedgehug is not on PATH, so git cannot run the merge driver",
```

```diff
--- a/internal/app/usecase/maintenance/maintain.go
+++ b/internal/app/usecase/maintenance/maintain.go
@@ -168,7 +168,7 @@ func doctorStore(ss *usecase.Session, e config.StoreEntry, only bool, repaired [
 	if openErr == nil {
 		defer func() { _ = opened.Repo().Close() }()
 	}
-	fs, err := Run(e.Path)
+	fs, err := Run(e.Path, ss.Getenv)
 	if err != nil {
 		return DoctorReport{}, err
 	}
```

- [ ] **Step 3: maintenance のテストを PATH で書き直す**

`init()` をやめ、テストごとに「hedgehug を 1 つ置いた temp ディレクトリを PATH にする」`Getenv` を作る（`hedgehugOnPath`）。`TestMergeDriverChecks` の「PATH に無い」側は、空の temp ディレクトリだけを PATH にした `Getenv` で `Run` を呼ぶ。外部テスト（`maintain_test.go`）は、これまで `t.Setenv("PATH", …)` でプロセスの PATH を変えていた（しかも `init()` の差し替えで効いていなかった）ので、`ts.Env["PATH"]` に入れる。

```diff
--- a/internal/app/usecase/maintenance/helpers_test.go
+++ b/internal/app/usecase/maintenance/helpers_test.go
@@ -42,9 +42,25 @@ func newBareStore(t *testing.T) string {
 	return dir
 }
 
-func init() {
-	// hedgehug is on PATH for every test but the ones that say otherwise.
-	lookPath = func(string) (string, error) { return "/bin/hedgehug", nil }
+// pathWith is a Getenv whose PATH is dir alone.
+func pathWith(dir string) func(string) string {
+	return func(k string) string {
+		if k == "PATH" {
+			return dir
+		}
+		return ""
+	}
+}
+
+// hedgehugOnPath is a Getenv whose PATH holds a hedgehug, as it does for
+// every test but the ones that say otherwise.
+func hedgehugOnPath(t *testing.T) func(string) string {
+	t.Helper()
+	bin := t.TempDir()
+	if err := os.WriteFile(filepath.Join(bin, "hedgehug"), []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // a stand-in for the binary
+		t.Fatal(err)
+	}
+	return pathWith(bin)
 }
 
 // rewriteSchema writes schema.yaml as the default schema after edit changed
```

```diff
--- a/internal/app/usecase/maintenance/doctor_test.go
+++ b/internal/app/usecase/maintenance/doctor_test.go
@@ -3,7 +3,6 @@ package maintenance
 import (
 	"errors"
 	"os"
-	"os/exec"
 	"path/filepath"
 	"slices"
 	"strings"
@@ -33,7 +32,7 @@ func writeFile(t *testing.T, dir, rel, content string) {
 
 func run(t *testing.T, dir string) []usecase.Finding {
 	t.Helper()
-	fs, err := Run(dir)
+	fs, err := Run(dir, hedgehugOnPath(t))
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -456,11 +455,13 @@ func TestMergeDriverChecks(t *testing.T) {
 
 	dir = newBareStore(t)
 	registerDriver(t, dir, gitrepo.MergeDriver)
-	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
-	if fs := run(t, dir); !hasFinding(fs, "merge_driver", "hedgehug is not on PATH") {
+	fs, err := Run(dir, pathWith(t.TempDir())) // a PATH with no hedgehug
+	if err != nil {
+		t.Fatal(err)
+	}
+	if !hasFinding(fs, "merge_driver", "hedgehug is not on PATH") {
 		t.Errorf("findings = %+v", fs)
 	}
-	lookPath = func(string) (string, error) { return "/bin/hedgehug", nil }
 	for _, f := range run(t, dir) {
 		if f.Check == "merge_driver" {
 			t.Errorf("registered and on PATH: %+v", f)
@@ -694,7 +695,7 @@ func TestDoctorViews(t *testing.T) {
 	}); err != nil {
 		t.Fatal(err)
 	}
-	fs, err := Run(dir)
+	fs, err := Run(dir, hedgehugOnPath(t))
 	if err != nil {
 		t.Fatal(err)
 	}
```

```diff
--- a/internal/app/usecase/maintenance/maintain_test.go
+++ b/internal/app/usecase/maintenance/maintain_test.go
@@ -29,8 +29,8 @@ func newCheckedSession(t *testing.T) *usecasetest.Session {
 	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o700); err != nil { //nolint:gosec // a stand-in for the binary
 		t.Fatal(err)
 	}
-	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
 	ts := usecasetest.New(t)
+	ts.Env["PATH"] = bin
 	s, err := ts.OpenStore()
 	if err != nil {
 		t.Fatal(err)
```

- [ ] **Step 4: cli のテストの Getenv が PATH を返すようにする**

`cli` の `testEnv.getenv` は PATH を知らなかった。`TestMain` が hedgehug を足したプロセスの PATH を返す。`TestDoctorMergeDriverAndInbox` は、プロセスの PATH を `t.Setenv` で空にしていた（そのため「直列」だった）のをやめ、`te.env["PATH"] = ""` にする。

```diff
--- a/internal/app/cli/helpers_test.go
+++ b/internal/app/cli/helpers_test.go
@@ -111,8 +112,11 @@ func (te *testEnv) getenv(k string) string {
 	if v, ok := te.env[k]; ok {
 		return v
 	}
-	if k == "HEDGEHUG_AUTO_SYNC" {
+	switch k {
+	case "HEDGEHUG_AUTO_SYNC":
 		return "0"
+	case "PATH":
+		return os.Getenv("PATH") // TestMain put hedgehug on it, as doctor expects
 	}
 	return ""
 }
```

```diff
--- a/internal/app/cli/doctor_test.go
+++ b/internal/app/cli/doctor_test.go
@@ -144,14 +144,12 @@ func TestDoctorFixTakesNoArguments(t *testing.T) {
 
 func TestDoctorMergeDriverAndInbox(t *testing.T) {
 	te := newStoreEnv(t)
-	// Not parallel: PATH is the process's. TestMain put hedgehug on it.
-	path := os.Getenv("PATH")
-	t.Setenv("PATH", "")
+	te.env["PATH"] = "" // TestMain put hedgehug on the PATH of the process
 	code, out, _ := te.run("doctor")
 	if code != 1 || !strings.Contains(out, "hedgehug is not on PATH") {
 		t.Errorf("exit %d\n%s", code, out)
 	}
-	t.Setenv("PATH", path)
+	delete(te.env, "PATH")
 	rec := filepath.Join(te.storeDir("me"), ".hedgehug", "local", "overwritten", "01JC1111111111111111111111.json")
 	if err := os.MkdirAll(filepath.Dir(rec), 0o750); err != nil {
 		t.Fatal(err)
```

- [ ] **Step 5: テストを回す**

Run: `go test -count=1 ./internal/lib/osfs -run 'TestLookPath'` → PASS
Run: `go test -count=1 ./internal/app/usecase/maintenance` → PASS
Run: `go test -count=1 ./internal/app/cli -run 'Doctor'` → PASS
Run: `go vet ./internal/...`

- [ ] **Step 6: コミット**

```bash
git add internal/lib/osfs/proc.go internal/lib/osfs/proc_test.go internal/app/usecase/maintenance internal/app/cli/helpers_test.go internal/app/cli/doctor_test.go
git commit -m "refactor(maintenance): doctor が PATH を Getenv から読む"
```

### Task B12: 背景 sync の起動を引数にする（`spawn` / `SetSpawn` / `StubSpawn`）

外す差し込み口: `replicate.spawn`（パッケージ変数）、`replicate.SetSpawn`、`usecasetest.StubSpawn`。
使っていたテスト（11 件）: `replicate/background_test.go` の `TestStartRespectsAutoSyncAndLock`・`TestStartGivesTheChildItsCommand`、`usecase/session_test.go` の `TestBackgroundSyncStartsOnlyWhenAsked`、`usecase/replication/sync_test.go` の `TestBeforeCommandStartsNothingWithoutRemote`・`TestBeforeCommandStartsDueStoreAndReportsFailureOnce`、`cli/background_test.go` の 5 件（`TestWriteStartsBackgroundSync`・`TestWriteWithoutRemoteStartsNothing`・`TestStoreFlagOnAStoreWithoutRemoteStartsNothing`・`TestReadStartsBackgroundSyncOnlyWhenDue`・`TestSyncStartsNothing`）、`cli/account_required_test.go` の `TestNoAccountStartsNoBackgroundSync`。

形は `Session.Run` と同じ「nil は本物」。`replicate.Start` が `spawn replicate.Spawn` を受け（nil なら今の `spawnDetached`）、`usecase.Session.Spawn` が持つ（`ForStore` が写す）。cli の `env.spawn` から `Session.Spawn` へ渡す。cli は depguard で `replicate` を import できない（spec 4.2）ので、`env.spawn` の型は同じ形の無名の func にする（`replicate.Spawn` に代入できる）。

**Files:**
- Modify: `internal/app/remote/replicate/background.go`
- Modify: `internal/app/usecase/session.go`、`internal/app/usecase/replication/sync.go`
- Modify: `internal/app/usecase/usecasetest/session.go`（`StubSpawn` を `SpawnFunc` に置き換える）
- Modify: `internal/app/cli/run.go`
- Test: `internal/app/remote/replicate/background_test.go`、`internal/app/usecase/session_test.go`、`internal/app/usecase/replication/sync_test.go`、`internal/app/cli/{helpers_test,background_test,account_required_test}.go`

**Interfaces:**
- Produces: `type replicate.Spawn func(name string, args []string, logPath string) error`
- Produces: `replicate.Start(s *store.Store, name string, getenv func(string) string, spawn Spawn) error`（nil の `spawn` は `spawnDetached`）
- Produces: `usecase.Session.Spawn replicate.Spawn`（nil は子プロセスを切り離して起動する）
- Produces: `usecasetest.SpawnFunc(f func(args []string)) replicate.Spawn`（プロセスを起こさず、子の引数を `f` に渡す）
- Produces: cli の `env.spawn`・テストの `testEnv.spawn`（`func(name string, args []string, logPath string) error`）
- Consumes: なし（`replicate.Start` の呼び出し元は `Session.MarkWritten` と `replication.BeforeCommand` の 2 か所）

- [ ] **Step 1: replicate と Session を直す**

```diff
--- a/internal/app/remote/replicate/background.go
+++ b/internal/app/remote/replicate/background.go
@@ -46,9 +46,14 @@ func Due(st State, now time.Time, interval time.Duration) bool {
 	return !st.PendingSince.IsZero() || now.Sub(st.LastSuccess) >= interval
 }
 
+// Spawn starts the process name with args detached from this one, with its
+// stderr appended to the file at logPath.
+type Spawn func(name string, args []string, logPath string) error
+
 // Start starts "hedgehug sync --background --store <name>" detached, unless
-// HEDGEHUG_AUTO_SYNC=0 or another background sync holds bg.lock.
-func Start(s *store.Store, name string, getenv func(string) string) error {
+// HEDGEHUG_AUTO_SYNC=0 or another background sync holds bg.lock. spawn starts
+// the process; nil starts it as a process of its own, detached.
+func Start(s *store.Store, name string, getenv func(string) string, spawn Spawn) error {
 	if getenv("HEDGEHUG_AUTO_SYNC") == "0" {
 		return nil
 	}
@@ -65,6 +70,9 @@ func Start(s *store.Store, name string, getenv func(string) string) error {
 	if err != nil {
 		return err
 	}
+	if spawn == nil {
+		spawn = spawnDetached
+	}
 	return spawn(exe, []string{"sync", "--background", "--store", name}, logPath(s))
 }
 
@@ -72,18 +80,6 @@ func logPath(s *store.Store) string {
 	return filepath.Join(s.Dir(), ".hedgehug", "local", logName)
 }
 
-// spawn starts the process detached from this one; tests replace it.
-var spawn = spawnDetached
-
-// SetSpawn replaces how Start starts the child, and returns the function
-// that puts the old one back. It is for tests only, so that they never
-// start a process.
-func SetSpawn(f func(name string, args []string, logPath string) error) (restore func()) {
-	old := spawn
-	spawn = f
-	return func() { spawn = old }
-}
-
 // Background is the body of sync --background: wait for push_interval after
 // pending_since, run, repeat while something is pending. run takes sync.lock
 // itself. A run that fails
```

```diff
--- a/internal/app/usecase/session.go
+++ b/internal/app/usecase/session.go
@@ -52,6 +52,9 @@ type Session struct {
 	// that store open, such as the GUI server's index, lets go of it.
 	// ForStore copies it. nil does nothing.
 	BeforeSwap func(path string)
+	// Spawn starts the background sync as a process; nil starts it detached.
+	// ForStore copies it.
+	Spawn      replicate.Spawn
 	Keyring    Keyring      // nil is the OS keychain
 	HTTPClient *http.Client // nil is a client with a 30-second timeout
 	// Progress is told how far the history index of a store has come while it
@@ -111,6 +114,7 @@ func (ss *Session) ForStore(name string) *Session {
 		BackgroundSync: ss.BackgroundSync,
 		BeforeUpdate:   ss.BeforeUpdate,
 		BeforeSwap:     ss.BeforeSwap,
+		Spawn:          ss.Spawn,
 		Keyring:        ss.Keyring,
 		HTTPClient:     ss.HTTPClient,
 		Run:            ss.Run,
@@ -565,7 +569,7 @@ func (ss *Session) MarkWritten(s *store.Store) {
 		return
 	}
 	if ss.BackgroundSync && len(e.Remotes) > 0 {
-		if err := replicate.Start(s, e.Name, ss.Getenv); err != nil {
+		if err := replicate.Start(s, e.Name, ss.Getenv, ss.Spawn); err != nil {
 			ss.AddWarning("the background sync did not start: " + err.Error())
 		}
 	}
```

```diff
--- a/internal/app/usecase/replication/sync.go
+++ b/internal/app/usecase/replication/sync.go
@@ -239,7 +239,7 @@ func BeforeCommand(ss *usecase.Session, report func(store, msg string)) {
 			report(e.Name, msg)
 		}
 		if st, err := replicate.ReadState(s); err == nil && replicate.Due(st, ss.Now(), cfg.Sync.Interval) {
-			_ = replicate.Start(s, e.Name, ss.Getenv) // starts a process of its own; it does not keep s
+			_ = replicate.Start(s, e.Name, ss.Getenv, ss.Spawn) // starts a process of its own; it does not keep s
 		}
 		_ = s.Repo().Close()
 	}
```

- [ ] **Step 2: usecasetest の StubSpawn を SpawnFunc にする**

グローバルを書き換えて `restore` を返す代わりに、`Spawn` 値を返す（下の diff の最初のハンク。同じファイルの 2 つ目のハンクは Task C13 のもの）。

```diff
--- a/internal/app/usecase/usecasetest/session.go
+++ b/internal/app/usecase/usecasetest/session.go
@@ -197,11 +197,10 @@ func (ts *Session) BeforeCommand(report func(store, msg string)) {
 	replication.BeforeCommand(ts.Session, report)
 }
 
-// StubSpawn replaces the start of the background sync with f, which gets the
-// arguments of the child, and returns the function that puts the old one
-// back. It is global: a test that uses it does not run in parallel.
-func StubSpawn(f func(args []string)) (restore func()) {
-	return replicate.SetSpawn(func(_ string, args []string, _ string) error { f(args); return nil })
+// SpawnFunc is a Spawn that starts no process: f gets the arguments of the
+// child. A Session or a CLI environment takes it as its Spawn.
+func SpawnFunc(f func(args []string)) replicate.Spawn {
+	return func(_ string, args []string, _ string) error { f(args); return nil }
 }
 
 // HasDefaultAccount reports whether config.yaml still has DefaultAccountYAML.
```

- [ ] **Step 3: cli の env と Session を繋ぐ**

`run.go` には Task B13 も触る箇所がある。ここでは `spawn` の 2 か所だけ（`env` に 1 行、`Session` の組み立てに 1 行）。

```diff
--- a/internal/app/cli/run.go
+++ b/internal/app/cli/run.go
@@ env struct
 	keyring        usecase.Keyring        // nil is the OS keychain
+	spawn          func(name string, args []string, logPath string) error // starts the background sync; nil starts it detached
 	httpClient     *http.Client           // nil is a client with a 30-second timeout
@@ run()
 		BeforeUpdate:   e.beforeUpdate,
+		Spawn:          e.spawn,
 		Keyring:        e.keyring,
```

（`gofmt -w` で桁がそろう。）

- [ ] **Step 4: テストを書き直す**

グローバルを書き換えなくなったので、`defer …()` が消え、各テストは自分の `Session`・`testEnv` に値を入れる。「直列」と書いたコメントは理由がなくなるので、replicate・usecase・replication のテストからは消す（テスト自体を並列にはしない）。cli は `testEnv.spawn` を `te.run` が `env.spawn` に渡す。

```diff
--- a/internal/app/remote/replicate/background_test.go
+++ b/internal/app/remote/replicate/background_test.go
@@ -30,17 +30,17 @@ func holdLocal(t *testing.T, s *store.Store, name string) (release func()) {
 func TestStartRespectsAutoSyncAndLock(t *testing.T) {
 	c := newCluster(t, 1)
 	calls := 0
-	defer SetSpawn(func(string, []string, string) error { calls++; return nil })()
-	_ = Start(c.stores[0], "s", func(k string) string { return map[string]string{"HEDGEHUG_AUTO_SYNC": "0"}[k] })
+	spawn := func(string, []string, string) error { calls++; return nil }
+	_ = Start(c.stores[0], "s", func(k string) string { return map[string]string{"HEDGEHUG_AUTO_SYNC": "0"}[k] }, spawn)
 	if calls != 0 {
 		t.Error("started with HEDGEHUG_AUTO_SYNC=0")
 	}
-	if err := Start(c.stores[0], "s", func(string) string { return "" }); err != nil || calls != 1 {
+	if err := Start(c.stores[0], "s", func(string) string { return "" }, spawn); err != nil || calls != 1 {
 		t.Errorf("calls = %d, err = %v", calls, err)
 	}
 	release := holdLocal(t, c.stores[0], "bg.lock") // as a waiting child would
 	defer release()
-	if err := Start(c.stores[0], "s", func(string) string { return "" }); err != nil || calls != 1 {
+	if err := Start(c.stores[0], "s", func(string) string { return "" }, spawn); err != nil || calls != 1 {
 		t.Errorf("started while another background sync waits: calls = %d, err = %v", calls, err)
 	}
 }
@@ -49,8 +49,8 @@ func TestStartGivesTheChildItsCommand(t *testing.T) {
 	c := newCluster(t, 1)
 	var args []string
 	var log string
-	defer SetSpawn(func(_ string, a []string, l string) error { args, log = a, l; return nil })()
-	if err := Start(c.stores[0], "work", func(string) string { return "" }); err != nil {
+	spawn := func(_ string, a []string, l string) error { args, log = a, l; return nil }
+	if err := Start(c.stores[0], "work", func(string) string { return "" }, spawn); err != nil {
 		t.Fatal(err)
 	}
 	if got := strings.Join(args, " "); got != "sync --background --store work" {
```

```diff
--- a/internal/app/usecase/session_test.go
+++ b/internal/app/usecase/session_test.go
@@ -4,7 +4,6 @@ import (
 	"testing"
 
 	"github.com/kukv/hedgehug/internal/app/domain/model"
-	"github.com/kukv/hedgehug/internal/app/remote/replicate"
 	"github.com/kukv/hedgehug/internal/app/storage/store"
 	"github.com/kukv/hedgehug/internal/app/usecase"
 	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
@@ -47,8 +46,7 @@ func TestResolveNotFoundIsKindNotFound(t *testing.T) {
 }
 
 // A write to a store with a remote starts the background sync once when the
-// session asks for it (BackgroundSync), and never when it does not. It does
-// not run in parallel: replicate.SetSpawn replaces what Start does for all.
+// session asks for it (BackgroundSync), and never when it does not.
 func TestBackgroundSyncStartsOnlyWhenAsked(t *testing.T) {
 	for _, tc := range []struct {
 		background bool
@@ -59,7 +57,7 @@ func TestBackgroundSyncStartsOnlyWhenAsked(t *testing.T) {
 		ts.Env["HEDGEHUG_AUTO_SYNC"] = "1"
 		ts.BackgroundSync = tc.background
 		var started int
-		restore := replicate.SetSpawn(func(string, []string, string) error { started++; return nil })
+		ts.Spawn = usecasetest.SpawnFunc(func([]string) { started++ })
 		s, ix, err := ts.Open()
 		if err != nil {
 			t.Fatal(err)
@@ -68,7 +66,6 @@ func TestBackgroundSyncStartsOnlyWhenAsked(t *testing.T) {
 		if _, err := ts.Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
 			t.Fatal(err)
 		}
-		restore()
 		if started != tc.want {
 			t.Errorf("BackgroundSync=%v: the background sync started %d times; want %d", tc.background, started, tc.want)
 		}
```

```diff
--- a/internal/app/usecase/replication/sync_test.go
+++ b/internal/app/usecase/replication/sync_test.go
@@ -122,13 +122,11 @@ func TestSyncAllReportsFailedStore(t *testing.T) {
 	}
 }
 
-// The tests of BeforeCommand do not run in parallel: StubSpawn replaces what
-// replicate.Start does for all.
 func TestBeforeCommandStartsNothingWithoutRemote(t *testing.T) {
 	ts := usecasetest.New(t)
 	ts.Env["HEDGEHUG_AUTO_SYNC"] = "1"
 	var started int
-	defer usecasetest.StubSpawn(func([]string) { started++ })()
+	ts.Spawn = usecasetest.SpawnFunc(func([]string) { started++ })
 	ts.BeforeCommand(func(string, string) { t.Error("reported") })
 	if started != 0 {
 		t.Errorf("started %d syncs for a store without a remote", started)
@@ -144,7 +142,7 @@ func TestBeforeCommandStartsDueStoreAndReportsFailureOnce(t *testing.T) {
 		t.Fatal(err)
 	}
 	var started []string
-	defer usecasetest.StubSpawn(func(args []string) { started = append(started, args[len(args)-1]) })()
+	ts.Spawn = usecasetest.SpawnFunc(func(args []string) { started = append(started, args[len(args)-1]) })
 	var reports []string
 	report := func(st, msg string) { reports = append(reports, st+"|"+msg) }
 
```

```diff
--- a/internal/app/cli/helpers_test.go
+++ b/internal/app/cli/helpers_test.go
@@ -38,7 +38,8 @@ type testEnv struct {
 	tmp      string            // the directory of temporary files, set by editWith
 	dir      string            // the current directory of the runs; "" for none
 
-	beforeUpdate func() // Session.BeforeUpdate of the next runs; meanwhile sets it
+	beforeUpdate func()                                                 // Session.BeforeUpdate of the next runs; meanwhile sets it
+	spawn        func(name string, args []string, logPath string) error // what starts the background sync in the next runs; nil starts a process
 
 	ucOnce sync.Once
 	ucSess *usecasetest.Session
@@ -131,6 +135,7 @@ func (te *testEnv) run(args ...string) (code int, stdout, stderr string) {
 		getwd:      func() (string, error) { return te.dir, nil },
 
 		beforeUpdate: te.beforeUpdate,
+		spawn:        te.spawn,
 	}
 	if args == nil {
 		args = []string{} // cobra reads os.Args, the flags of go test, for nil
```

```diff
--- a/internal/app/cli/background_test.go
+++ b/internal/app/cli/background_test.go
@@ -41,7 +41,7 @@ func settled(t *testing.T) *testEnv {
 func TestWriteStartsBackgroundSync(t *testing.T) {
 	alice := settled(t)
 	var started []string
-	defer usecasetest.StubSpawn(func(args []string) { started = append(started, strings.Join(args, " ")) })()
+	alice.spawn = usecasetest.SpawnFunc(func(args []string) { started = append(started, strings.Join(args, " ")) })
 	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
 	alice.ok("add", "x")
 	if len(started) != 1 || !strings.Contains(started[0], "sync --background --store org") {
@@ -57,7 +57,7 @@ func TestWriteWithoutRemoteStartsNothing(t *testing.T) {
 	te := newTestEnv(t)
 	te.ok("store", "init", "solo", "--prefix", "SOL")
 	started := 0
-	defer usecasetest.StubSpawn(func([]string) { started++ })()
+	te.spawn = usecasetest.SpawnFunc(func([]string) { started++ })
 	te.env["HEDGEHUG_AUTO_SYNC"] = "1"
 	te.ok("add", "x")
 	if started != 0 {
@@ -69,7 +69,7 @@ func TestStoreFlagOnAStoreWithoutRemoteStartsNothing(t *testing.T) {
 	te := newTestEnv(t)
 	te.ok("store", "init", "solo", "--prefix", "SOL")
 	started := 0
-	defer usecasetest.StubSpawn(func([]string) { started++ })()
+	te.spawn = usecasetest.SpawnFunc(func([]string) { started++ })
 	te.env["HEDGEHUG_AUTO_SYNC"] = "1"
 	te.ok("--store", "solo", "ls")
 	if started != 0 {
@@ -80,7 +80,7 @@ func TestStoreFlagOnAStoreWithoutRemoteStartsNothing(t *testing.T) {
 func TestReadStartsBackgroundSyncOnlyWhenDue(t *testing.T) {
 	alice := settled(t)
 	var started int
-	defer usecasetest.StubSpawn(func([]string) { started++ })()
+	alice.spawn = usecasetest.SpawnFunc(func([]string) { started++ })
 	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
 	alice.ok("ls")
 	if started != 0 {
@@ -96,7 +96,7 @@ func TestReadStartsBackgroundSyncOnlyWhenDue(t *testing.T) {
 func TestSyncStartsNothing(t *testing.T) {
 	alice, _, _ := twoUsers(t)
 	var started int
-	defer usecasetest.StubSpawn(func([]string) { started++ })()
+	alice.spawn = usecasetest.SpawnFunc(func([]string) { started++ })
 	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
 	alice.now = alice.now.Add(time.Hour)
 	alice.ok("sync", "--status")
```

```diff
--- a/internal/app/cli/account_required_test.go
+++ b/internal/app/cli/account_required_test.go
@@ -47,7 +47,7 @@ func TestNoAccountStartsNoBackgroundSync(t *testing.T) {
 	alice.now = alice.now.Add(time.Hour) // the store is due
 	alice.dropDefaultAccount()
 	started := 0
-	defer usecasetest.StubSpawn(func([]string) { started++ })()
+	alice.spawn = usecasetest.SpawnFunc(func([]string) { started++ })
 	alice.env["HEDGEHUG_AUTO_SYNC"] = "1"
 	for _, args := range [][]string{{"auth", "status"}, {"help"}, {"ls", "--help"}} {
 		alice.run(args...)
```

- [ ] **Step 4b: `usecasetest.(*Session).Renew` に `Spawn` を足す**

A2 の `Renew` は Session の公開のフィールドを 1 つずつ写すので、増えた `Spawn` を足す（A2 の `TestRenewCopiesEveryExportedField` が、足さないと `Renew does not copy Session.Spawn` で落ちる）。`Renew` の中の、`Run` を写している行の次に足す:

```go
		Spawn:          o.Spawn,
```

（フィールドの並びと桁は、その時点の `Renew` に合わせる）

- [ ] **Step 5: テストを回す**

Run: `go test -count=1 ./internal/app/remote/replicate -run 'TestStart|TestBackground'` → PASS
Run: `go test -count=1 ./internal/app/usecase -run 'TestBackgroundSync|TestRenewCopiesEveryExportedField|TestForStoreCopiesEveryExportedField'` → PASS
Run: `go test -count=1 ./internal/app/usecase/replication -run 'TestBeforeCommand'` → PASS
Run: `go test -count=1 ./internal/app/cli -run 'Background|StartsNothing|NoAccountStarts|TestReadStarts|TestSyncStarts|TestWrite'` → PASS
Run: `go vet ./internal/...` と `golangci-lint run ./internal/app/cli/... ./internal/app/remote/replicate/...`（depguard が cli の replicate import を止める）

- [ ] **Step 6: コミット**

```bash
git add internal/app/remote/replicate internal/app/usecase/session.go internal/app/usecase/session_test.go internal/app/usecase/replication internal/app/usecase/usecasetest/session.go internal/app/cli
git commit -m "refactor(replicate): 背景 sync の起動を引数で渡す"
```

注意: `usecasetest/session.go` と `cli/helpers_test.go` は Task B11・C13 とも触る。ファイル単位の `git add` ではなく、`git add -p` でその Task のハンクだけを入れるか、Task を順に実行して前の Task をコミットしてから進める。

### Task B13: hh ui の終了待ちを Main が作る（`env.uiContext` の nil 分岐）

外す差し込み口: `cli.env.uiContext` の「本番は nil」と、`ui.go` の `uiContextOrSignals` の nil 分岐。
使っていたテスト: `cli/ui_test.go` の `startUI` を使う 6 件（`TestUIStartsAndStops`・`TestUIOpensTheBrowser`・`TestUIReusesTheRunningServer`・`TestUIReuseWarnsWhenTheBrowserDoesNotOpen`・`TestUIOverwritesAStaleUIJSON`・`TestUIKeepsAUIJSONThatIsNotItsOwn`）。`te.run("ui", …)` を直に呼ぶ 3 か所（`TestUIReusesTheRunningServer`・`TestUIReuseWarnsWhenTheBrowserDoesNotOpen`・`TestUIPortInUse`）は `uiContext` を入れないが、`ui` が context を作る前に返る（動いているサーバを再利用する・ポートが埋まっている）ので呼ばれない。

`Main` が signal の context を **作る関数** を `env.uiContext` に入れる。context 値そのものを `Main` の先頭で作る形にはしない: `signal.NotifyContext` は呼んだ時点から Ctrl-C を握るので、`hh ui` 以外のコマンドが Ctrl-C で止まらなくなる（挙動が変わる）。関数なら、握り始めるのは今と同じ `hh ui` が context を要る時点になる。nil の分岐は `ui.go` から消え、`a.uiContext()` を直に呼ぶ。

**Files:**
- Modify: `internal/app/cli/run.go`、`internal/app/cli/ui.go`
- Test: `internal/app/cli/ui_test.go`（変更なし。`startUI` が自分の context を入れる形はそのまま）

**Interfaces:**
- Produces: `signalContext() (context.Context, context.CancelFunc)`（`signal.NotifyContext(context.Background(), osfs.StopSignals()...)`）
- Changes: `env.uiContext` のコメント（「テスト用、nil は signal」→「context を作る関数。バイナリのものは signal で終わる」）。型は変わらない

- [ ] **Step 1: Main に signalContext を入れ、ui.go の分岐を消す**

`run.go` の `env` 構造体では `uiContext` のコメントだけが変わる（`spawn` の行は Task B12 のもの）。他の変更は下の diff のとおり。

```diff
--- a/internal/app/cli/run.go
+++ b/internal/app/cli/run.go
@@ env struct
-	// uiContext is for tests: what hh ui runs until; nil is Ctrl-C and SIGTERM.
+	// uiContext makes the context hh ui runs until. The binary's ends with
+	// Ctrl-C and SIGTERM, caught from the moment hh ui asks for it.
 	uiContext func() (context.Context, context.CancelFunc)
```

```diff
--- a/internal/app/cli/run.go
+++ b/internal/app/cli/run.go
@@ -9,6 +9,7 @@ import (
 	"io"
 	"net/http"
 	"os"
+	"os/signal"
 	"strings"
 	"time"
 
@@ -17,6 +18,7 @@ import (
 
 	"github.com/kukv/hedgehug/internal/app/cli/prompt"
 	"github.com/kukv/hedgehug/internal/app/usecase"
+	"github.com/kukv/hedgehug/internal/lib/osfs"
 )
 
 // version is what --version prints. Main sets it from the version that
@@ -61,9 +65,16 @@ func Main(v string) int {
 		getwd:      os.Getwd,
 		openURL:    openURL,
 		keyring:    kr,
+		uiContext:  signalContext,
 	})
 }
 
+// signalContext is a context that Ctrl-C (Ctrl-Break on Windows) and SIGTERM
+// end.
+func signalContext() (context.Context, context.CancelFunc) {
+	return signal.NotifyContext(context.Background(), osfs.StopSignals()...)
+}
+
 func isTerminal(f any) bool {
 	file, ok := f.(*os.File)
 	return ok && term.IsTerminal(int(file.Fd())) //nolint:gosec // a file descriptor fits in an int
```

```diff
--- a/internal/app/cli/ui.go
+++ b/internal/app/cli/ui.go
@@ -10,7 +10,6 @@ import (
 	"net"
 	"net/http"
 	"os"
-	"os/signal"
 	"path/filepath"
 	"strconv"
 	"time"
@@ -89,7 +88,7 @@ func (a *app) ui(port int, noOpen bool) error {
 	}
 	defer removeOwnUIJSON(file, info.Token)
 
-	ctx, stop := a.uiContextOrSignals()
+	ctx, stop := a.uiContext()
 	defer stop()
 	srv.Start(ctx)
 	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
@@ -124,15 +123,6 @@ func (a *app) warnBrowser(err error) {
 	}
 }
 
-// uiContextOrSignals is the context hh ui runs until: the test's, or one
-// that Ctrl-C (Ctrl-Break on Windows) and SIGTERM end.
-func (a *app) uiContextOrSignals() (context.Context, context.CancelFunc) {
-	if a.uiContext != nil {
-		return a.uiContext()
-	}
-	return signal.NotifyContext(context.Background(), osfs.StopSignals()...)
-}
-
 // readRunningUI reads ui.json and reports whether the server it names answers.
 func readRunningUI(file string) (uiInfo, bool) {
 	var info uiInfo
```

- [ ] **Step 2: テストを回す**

Run: `go test -count=1 ./internal/app/cli -run 'TestUI' -v` → 7 件 PASS（`-v` で件数を確かめる）
Run: `go vet ./internal/app/cli` と `golangci-lint run ./internal/app/cli/...`

- [ ] **Step 3: コミット**

```bash
git add internal/app/cli/run.go internal/app/cli/ui.go
git commit -m "refactor(cli): hh ui の終了待ちの context を Main が作る"
```

### Task C11: Push の「読んだコミットを送る」を Conn.Auth の中のコミットで確かめる（`beforePush`）

外す差し込み口: `gitrepo.beforePush`（パッケージ変数と、`Push` 内の呼び出し）。
使っていたテスト: `remote_test.go` の `TestPushSendsTheCommitItRead`。

確かめたこと（コードを読んだ結果）: `Push` は `r.Head()` で送るコミット `h` を読み、その後で `r.conn.endpoint(u)` を呼ぶ。`endpoint` は `Conn.Auth` を呼ぶ。`beforePush()` はその後（`Open` の後）にあった。つまり `Auth` の中でコミットすれば、「`Head` を読んだ後、送る前」への割り込みになり、`beforePush` と同じ瞬間を作れる（`Open(r.dir)` はコミットの後なので、`p` は新しい HEAD を見るが、refspec は `h` を送る）。確かめる内容は変わらない。`Fetch` も `Auth` を呼ぶので、テストの後半の `Fetch` では何もしない旗を立てる。

**Files:**
- Modify: `internal/lib/gitrepo/remote.go`
- Test: `internal/lib/gitrepo/remote_test.go`

**Interfaces:**
- Produces: なし（変数と呼び出しを消すだけ）

- [ ] **Step 1: beforePush を消し、テストを Conn.Auth に書き直す**

```diff
--- a/internal/lib/gitrepo/remote.go
+++ b/internal/lib/gitrepo/remote.go
@@ -140,10 +140,6 @@ func (r *Repo) RemoteHead(remote string) (plumbing.Hash, error) {
 	return ref.Hash(), nil
 }
 
-// beforePush runs once Push has read the commit it sends; tests replace it to
-// commit meanwhile.
-var beforePush = func() {}
-
 // Push sends main to the remote as a fast-forward. It returns ErrRejected when
 // the remote is not an ancestor of main, and nil when there is nothing to send.
 // The commit main is at when Push starts is sent, and becomes
@@ -174,7 +170,6 @@ func (r *Repo) Push(remote string) error {
 		return fmt.Errorf("push %s: %w", remote, err)
 	}
 	defer func() { _ = p.Close() }()
-	beforePush()
 	err = p.r.Push(&git.PushOptions{
 		RemoteName: remote,
 		RemoteURL:  to,
```

```diff
--- a/internal/lib/gitrepo/remote_test.go
+++ b/internal/lib/gitrepo/remote_test.go
@@ -9,6 +9,7 @@ import (
 	"testing"
 
 	"github.com/go-git/go-git/v5/config"
+	"github.com/go-git/go-git/v5/plumbing/transport"
 )
 
 // noGit empties PATH so that a call to the git binary fails the test.
@@ -114,11 +115,22 @@ func TestPushSendsTheCommitItRead(t *testing.T) {
 	a, _ := cloneTwo(t, remote)
 	commitFiles(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
 	sent, _ := a.r.Head()
-	beforePush = func() { commitFiles(t, a.r, a.dir, map[string]string{"y.md": "during\n"}) }
-	defer func() { beforePush = func() {} }()
+	// Push reads HEAD, then asks Conn for the auth: a commit made there is one
+	// made while the push starts.
+	pushing := true
+	a.r.SetConn(Conn{Auth: func(string) (transport.AuthMethod, error) {
+		if pushing { // the Fetch below asks for the auth too
+			commitFiles(t, a.r, a.dir, map[string]string{"y.md": "during\n"})
+		}
+		return nil, nil
+	}})
 	if err := a.r.Push("origin"); err != nil {
 		t.Fatal(err)
 	}
+	pushing = false
+	if now, _ := a.r.Head(); now == sent {
+		t.Fatal("no commit was made while the push started")
+	}
 	if got, _ := a.r.RemoteHead("origin"); got != sent {
 		t.Errorf("RemoteHead = %s, want the pushed %s", got, sent)
 	}
```

- [ ] **Step 2: テストを回す**

Run: `go test -count=1 ./internal/lib/gitrepo -run 'TestPush'` → PASS

- [ ] **Step 3: 検出できることを確かめる**

`remote.go` の `Push` で、`endpoint` を呼んだ後（`// When the remote refuses the push before it reads the pack` の直前）に `h` を読み直す 4 行を足す（送るコミットを Auth の後で決める不具合）:

```go
	h, err = r.Head()
	if err != nil {
		return err
	}
```

Run: `go test -count=1 ./internal/lib/gitrepo -run 'TestPushSends'`
Expected: FAIL（`RemoteHead = 6743673e…, want the pushed a5bafb65…` と `the remote has …, want …`。コミットのハッシュは実行ごとに違う）。確かめたら 4 行を消して PASS に戻す。

- [ ] **Step 4: コミット**

```bash
git add internal/lib/gitrepo/remote.go internal/lib/gitrepo/remote_test.go
git commit -m "refactor(gitrepo): Push の割り込みを Conn.Auth で作る"
```

### Task C12: move の 2 つ目の書き込みの失敗を、書けないディレクトリで作る（BeforeUpdate の chmod 版）

外す差し込み口: `Session.BeforeUpdate`（と `cli` の `env.beforeUpdate`）の、move の「source への書き込み」で失敗させる用途。`BeforeUpdate` 自体は残る。
使っていたテスト: `cli/mv_store_resumed_test.go` の `unfinishedMove`（`TestMvStoreRerunSaysItFinishedTheMove`・`TestMvStoreRerunJSONResumed` が使う）、`usecase/stores/movestore_unfinished_test.go` の `TestMoveStoreUnfinishedByAnIOErrorIsAConflict`。

今は「2 回目の `BeforeUpdate`（= source への書き込み）で、AA-2（`stores` の方は G）のファイルを `chmod 0` にして読めなくする」。事前に、そのファイルの **ディレクトリ** を `0o500`（書き込み不可）にする。plan の読み取りと destination への書き込みは通り、source でファイルを消す・一時ファイルを作る所で `permission denied` になる（試した結果。エラーの文言は `open …/.01M6….md.tmp-…: permission denied`）。確かめる内容（destination に入った後で source が失敗し、`moved into bravo-long; run the same command again` と `move.unfinished`・KindConflict になり、同じコマンドで続きができる）は同じ。root では書き込み不可が効かないので、今ある `os.Geteuid() == 0` の skip を残す。ディレクトリのモードは `os.Stat` で控えて戻し、`t.Cleanup` でも戻す（temp ディレクトリを消せるように）。

**Files:**
- Test: `internal/app/cli/mv_store_resumed_test.go`、`internal/app/usecase/stores/movestore_unfinished_test.go`

**Interfaces:**
- Produces: なし

- [ ] **Step 1: 2 つのテストを書き直す**

```diff
--- a/internal/app/cli/mv_store_resumed_test.go
+++ b/internal/app/cli/mv_store_resumed_test.go
@@ -3,27 +3,31 @@ package cli
 import (
 	"encoding/json"
 	"os"
+	"path/filepath"
 	"strings"
 	"testing"
 )
 
 // unfinishedMove stops the move of AA-1 (with AA-2 under it) to bravo-long
-// after the destination was written: the source file of AA-2 cannot be read.
+// after the destination was written: the directory of the source file of
+// AA-2 cannot be written to, so the source cannot drop the file.
 func unfinishedMove(t *testing.T, te *testEnv) {
 	t.Helper()
 	if os.Geteuid() == 0 {
-		t.Skip("a file that cannot be read needs a permission that is enforced")
+		t.Skip("a directory that cannot be written to needs a permission that is enforced")
 	}
 	path, _ := te.storeTaskFile("alpha", "AA-2")
-	calls := 0
-	te.beforeUpdate = func() {
-		if calls++; calls == 2 { // the write into the source
-			_ = os.Chmod(path, 0)
-		}
+	dir := filepath.Dir(path)
+	fi, err := os.Stat(dir)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := os.Chmod(dir, 0o500); err != nil {
+		t.Fatal(err)
 	}
+	t.Cleanup(func() { _ = os.Chmod(dir, fi.Mode().Perm()) }) // so that the directory of the test can be removed
 	code, _, errs := te.run("mv", "AA-1", "--store", "bravo-long", "--yes")
-	te.beforeUpdate = nil
-	if err := os.Chmod(path, 0o600); err != nil {
+	if err := os.Chmod(dir, fi.Mode().Perm()); err != nil {
 		t.Fatal(err)
 	}
 	if code == 0 || !strings.Contains(errs, "moved into bravo-long; run the same command again") {
```

```diff
--- a/internal/app/usecase/stores/movestore_unfinished_test.go
+++ b/internal/app/usecase/stores/movestore_unfinished_test.go
@@ -17,20 +17,22 @@ import (
 func TestMoveStoreUnfinishedByAnIOErrorIsAConflict(t *testing.T) {
 	t.Parallel()
 	if os.Geteuid() == 0 {
-		t.Skip("a file that cannot be read needs a permission that is enforced")
+		t.Skip("a directory that cannot be written to needs a permission that is enforced")
 	}
 	f := usecasetest.NewMoveFx(t)
 	as, _ := f.TS.ForStore("org-a").OpenStore()
-	gPath := filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.G)))
-	calls := 0
-	f.TS.BeforeUpdate = func() {
-		if calls++; calls == 2 { // the write into the source
-			_ = os.Chmod(gPath, 0)
-		}
+	// The source cannot drop the file of G, so the move stops after the
+	// destination was written.
+	gDir := filepath.Dir(filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.G))))
+	fi, err := os.Stat(gDir)
+	if err != nil {
+		t.Fatal(err)
 	}
-	_, err := stores.MoveStore(f.TS.Session, f.In())
-	f.TS.BeforeUpdate = nil
-	_ = os.Chmod(gPath, 0o600)
+	if err := os.Chmod(gDir, 0o500); err != nil {
+		t.Fatal(err)
+	}
+	t.Cleanup(func() { _ = os.Chmod(gDir, fi.Mode().Perm()) }) // so that the directory of the test can be removed
+	_, err = stores.MoveStore(f.TS.Session, f.In())
 	usecasetest.MustCode(t, err, usecase.KindConflict, "move.unfinished")
 	if e := usecase.AsError(err); e == nil || e.Args["store"] != "org-b" {
 		t.Errorf("args = %v; want store org-b", e)
```

- [ ] **Step 2: テストを回す**

Run: `go test -count=1 ./internal/app/cli -run 'TestMvStoreRerun' -v` → 2 件 PASS（SKIP でないこと。root で回していないこと）
Run: `go test -count=1 ./internal/app/usecase/stores -run 'TestMoveStoreUnfinishedByAnIOError' -v` → PASS

- [ ] **Step 3: 検出できることを確かめる**

(a) `stores`: `usecase/stores/movestore.go` の「半分の move は競合として返す」分岐 `if kind == usecase.KindUnknown {` を `if kind == usecase.KindUnknown && false {` にする。
Run: `go test -count=1 ./internal/app/usecase/stores -run 'TestMoveStoreUnfinishedByAnIOError'`
Expected: FAIL（`err = moved into org-b; run the same command again to finish the source: open …/tasks/4/P/.01M6….md.tmp-…: permission denied; want kind 4 code move.unfinished`）。確かめたら戻す。

(b) `cli`: テストの `os.Chmod(dir, 0o500)` を `0o700` にする（失敗を作らない）。
Run: `go test -count=1 ./internal/app/cli -run 'TestMvStoreRerun'`
Expected: FAIL（`the first run: exit 0, stderr ""`。止まらない move では、書き直したテストの前提が成り立たないことを見る）。確かめたら `0o500` に戻す。

- [ ] **Step 4: コミット**

```bash
git add internal/app/cli/mv_store_resumed_test.go internal/app/usecase/stores/movestore_unfinished_test.go
git commit -m "test(move): source への書き込みの失敗を書けないディレクトリで作る"
```

### Task C13: config.yaml が書かれていないことを、ファイルで確かめる（BeforeUpdate の「呼ばれたら失敗」版）

外す差し込み口: `Session.BeforeUpdate` の、「`UpdateConfig` が呼ばれない」ことを確かめる用途（`ts.BeforeUpdate = func() { t.Error(…) }`）。
使っていたテスト: `usecase/stores/storeset_test.go` の `TestStoreSetWritesNothingWhenUnchanged`・`TestStoreSetErrors`、`usecase/accounts/auth_test.go` の `TestStoreDefault`。

`UpdateConfig` が呼ばれないことと、`config.yaml` が書かれないことは同じ結果になる（呼ばれれば `c.Update` が書き直し、更新時刻が変わる）。`TestStoreDefault` はすでに「更新時刻を過去にして、時刻とバイト列を比べる」形なので、その形を `usecasetest.Session.FreezeConfig` に切り出し、`BeforeUpdate` の 2 行だけを消す。`TestStoreSetErrors` は、最初に凍結して最後に検査する（途中の拒否された呼び出しのどれかが書いても落ちる）。

**Files:**
- Modify: `internal/app/usecase/usecasetest/session.go`（`FreezeConfig` を足す。同じファイルの別ハンクは Task B12）
- Test: `internal/app/usecase/stores/storeset_test.go`、`internal/app/usecase/accounts/auth_test.go`

**Interfaces:**
- Produces: `func (ts *Session) FreezeConfig(what string) (unchanged func())`（`config.yaml` の更新時刻を 2020-01-01 にし、返す関数が「時刻もバイト列も変わっていない」ことを `ts.T` に対して確かめる。`what` は失敗の文言に入る）

- [ ] **Step 1: FreezeConfig を足す**

```diff
--- a/internal/app/usecase/usecasetest/session.go
+++ b/internal/app/usecase/usecasetest/session.go
@@ -1259,6 +1258,33 @@ func HookConfigWrite(t *testing.T, ts *Session, fn func(*config.Config) error) {
 	}
 }
 
+// FreezeConfig sets the modification time of config.yaml to the past and
+// returns the check that the file was not written since: its time and its
+// bytes are what they were. what says what must not have written it.
+func (ts *Session) FreezeConfig(what string) (unchanged func()) {
+	ts.T.Helper()
+	p := filepath.Join(ts.Env["HEDGEHUG_HOME"], "config.yaml")
+	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
+	if err := os.Chtimes(p, old, old); err != nil {
+		ts.T.Fatal(err)
+	}
+	before, err := os.ReadFile(p) //nolint:gosec // the config of the test
+	if err != nil {
+		ts.T.Fatal(err)
+	}
+	return func() {
+		ts.T.Helper()
+		if fi, err := os.Stat(p); err != nil {
+			ts.T.Error(err)
+		} else if !fi.ModTime().Equal(old) {
+			ts.T.Errorf("config.yaml was written by %s: modified %v", what, fi.ModTime())
+		}
+		if after, _ := os.ReadFile(p); string(after) != string(before) { //nolint:gosec // the config of the test
+			ts.T.Errorf("config.yaml changed by %s:\n%s", what, after)
+		}
+	}
+}
+
 // LogOf is the first-parent log of HEAD of r, newest first, capped at limit
 // when it is above zero.
 func LogOf(r *gitrepo.Repo, limit int) ([]gitrepo.LogEntry, error) {
```

- [ ] **Step 2: テストを書き直す**

`TestStoreDefault` の変更は、`ts.BeforeUpdate = …` と `ts.BeforeUpdate = nil` の 2 行を消すだけ（時刻とバイト列の検査は元からある）。

```diff
--- a/internal/app/usecase/stores/storeset_test.go
+++ b/internal/app/usecase/stores/storeset_test.go
@@ -58,19 +58,20 @@ func TestStoreSetWritesNothingWhenUnchanged(t *testing.T) {
 	if _, err := stores.StoreSet(ts.Session, "org-a", &teal, nil); err != nil {
 		t.Fatal(err)
 	}
-	ts.BeforeUpdate = func() { t.Error("config.yaml is updated for the same values") }
+	unchanged := ts.FreezeConfig("a call with the same values")
 	if v, err := stores.StoreSet(ts.Session, "org-a", &teal, &no); err != nil || v.Color != "teal" || v.Hidden {
 		t.Errorf("same values: %+v, %v", v, err)
 	}
 	if _, err := stores.StoreSet(ts.Session, "org-a", nil, nil); err != nil {
 		t.Errorf("nothing given: %v", err)
 	}
+	unchanged()
 }
 
 func TestStoreSetErrors(t *testing.T) {
 	t.Parallel()
 	ts := usecasetest.NewTwoStore(t)
-	ts.BeforeUpdate = func() { t.Error("a refused call updates config.yaml") }
+	unchanged := ts.FreezeConfig("a refused call")
 	red, grey, teal := "red", "grey", "teal"
 
 	_, err := stores.StoreSet(ts.Session, "org-a", &red, nil)
@@ -90,6 +91,7 @@ func TestStoreSetErrors(t *testing.T) {
 	if e := usecase.AsError(err); e.Args["name"] != "nope" || len(e.Args["candidates"].([]string)) != 2 {
 		t.Errorf("args = %v", e.Args)
 	}
+	unchanged()
 }
 
 func TestStoreViewReadOnly(t *testing.T) {
```

```diff
--- a/internal/app/usecase/accounts/auth_test.go
+++ b/internal/app/usecase/accounts/auth_test.go
@@ -381,11 +397,9 @@ func TestStoreDefault(t *testing.T) {
 		t.Fatal(err)
 	}
 	before, _ := os.ReadFile(p.ConfigFile)
-	ts.BeforeUpdate = func() { t.Error("config.yaml is updated for the same name") }
 	if name, err := stores.StoreDefault(ts.Session, "me"); err != nil || name != "me" {
 		t.Errorf("same = %q, %v", name, err)
 	}
-	ts.BeforeUpdate = nil
 	if fi, _ := os.Stat(p.ConfigFile); !fi.ModTime().Equal(old) {
 		t.Errorf("modified: %v", fi.ModTime())
 	}
```

- [ ] **Step 3: テストを回す**

Run: `go test -count=1 ./internal/app/usecase/stores -run 'TestStoreSet'` → PASS
Run: `go test -count=1 ./internal/app/usecase/accounts -run 'TestStoreDefault'` → PASS

- [ ] **Step 4: 検出できることを確かめる**

(a) `usecase/stores/storeops.go` の `StoreSet` の `if (color != nil && *color != cur.Color) || …` を `if true || (color != nil && …` にする（同じ値でも書く）。
Run: `go test -count=1 ./internal/app/usecase/stores -run 'TestStoreSet'`
Expected: `TestStoreSetWritesNothingWhenUnchanged` が FAIL（`config.yaml was written by a call with the same values: modified …`）。戻す。

(b) `StoreSet` の先頭に `cc, _, _ := ss.Config()` と `_ = ss.UpdateConfig(cc, func(*config.Config) error { return nil })` を足す（拒否される呼び出しでも書く）。
Expected: `TestStoreSetErrors` が FAIL（`config.yaml was written by a refused call: modified …`）。戻す。

- [ ] **Step 5: コミット**

```bash
git add internal/app/usecase/usecasetest/session.go internal/app/usecase/stores/storeset_test.go internal/app/usecase/accounts/auth_test.go
git commit -m "test(stores): config.yaml が書かれないことをファイルで確かめる"
```

### Task C14: keychain の巻き戻しを、書けない config で見る（`breakConfig`）

外す差し込み口: `Session.BeforeUpdate` の、「config の更新を壊す」用途（`accounts/auth_test.go` の `breakConfig`）。
使っていたテスト: `TestAuthLoginRollsBackTheKeychain` の `old value`・`no old value`（`another writer added the account` は `HookConfigWrite` を使うので変えない）、`TestAuthLoginRollbackFailureIsSaid`。

今は、更新が `config.yaml` を読む直前に中身を壊して失敗させる。事前に `config.yaml` のあるディレクトリを `0o500` にし、ファイルを置き換えられなくする。`auth.lock` はこのディレクトリにあり、**無いと作れない**（そこで失敗すると keychain に触る前に終わり、巻き戻しのテストが空振りする）ので、先に 1 度ロックを取って放し、ファイルを作っておく。root では効かないので skip する。

**Files:**
- Test: `internal/app/usecase/accounts/auth_test.go`

**Interfaces:**
- Produces: なし（`breakConfig` の呼び出し側は変わらない）

- [ ] **Step 1: breakConfig を書き直す**

（import に `path/filepath` と `internal/lib/osfs` を足す。ほかのハンクは Task C13 のもの。）

```diff
--- a/internal/app/usecase/accounts/auth_test.go
+++ b/internal/app/usecase/accounts/auth_test.go
@@ -4,6 +4,7 @@ import (
 	"encoding/json"
 	"errors"
 	"os"
+	"path/filepath"
 	"strings"
 	"testing"
 	"time"
@@ -15,6 +16,7 @@ import (
 	"github.com/kukv/hedgehug/internal/app/usecase/stores"
 	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
 	"github.com/kukv/hedgehug/internal/lib/fakegh"
+	"github.com/kukv/hedgehug/internal/lib/osfs"
 )
 
 func TestAuthLoginNewAccountByDevice(t *testing.T) {
@@ -152,20 +154,34 @@ func TestAuthLoginOtherLoginWritesNothing(t *testing.T) {
 	}
 }
 
-// breakConfig makes the next config update fail: config.yaml is not YAML
-// when the update reads it.
+// breakConfig makes the config updates fail: the directory of config.yaml
+// cannot be written to, so the file cannot be replaced. The lock file of the
+// directory is made first, so that the login still gets to the keychain.
 func breakConfig(t *testing.T, ts *usecasetest.Session) {
 	t.Helper()
+	if os.Geteuid() == 0 {
+		t.Skip("a directory that cannot be written to needs a permission that is enforced")
+	}
 	_, p, err := ts.Config()
 	if err != nil {
 		t.Fatal(err)
 	}
-	ts.BeforeUpdate = func() {
-		if err := os.WriteFile(p.ConfigFile, []byte("accounts: [\n"), 0o600); err != nil {
-			t.Error(err)
-		}
-		ts.BeforeUpdate = nil
+	lock, err := osfs.LockFile(ts.LockFile(), 0, 0, nil)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := lock.Release(); err != nil {
+		t.Fatal(err)
+	}
+	dir := filepath.Dir(p.ConfigFile)
+	fi, err := os.Stat(dir)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if err := os.Chmod(dir, 0o500); err != nil {
+		t.Fatal(err)
 	}
+	t.Cleanup(func() { _ = os.Chmod(dir, fi.Mode().Perm()) }) // so that the directory of the test can be removed
 }
 
 // A login that cannot write config.yaml puts the keychain back: to the old
```

- [ ] **Step 2: テストを回す**

Run: `go test -count=1 ./internal/app/usecase/accounts -run 'TestAuthLoginRoll' -v` → 4 件 PASS（SKIP でないこと）

- [ ] **Step 3: 検出できることを確かめる**

`usecase/accounts/auth.go` の巻き戻し（`rerr := auth.WithLock(opt, func() error { if had { … } return auth.Delete(acct, k) })`）の中身を `_, _ = prev, had; return nil` にして、何も戻さないようにする。
Run: `go test -count=1 ./internal/app/usecase/accounts -run 'TestAuthLoginRoll'`
Expected: FAIL（`old value`: `keychain = "t-alice"; want the old value`、`no old value`: `keychain = "{…access_token…}"; want nothing`、`another writer added the account`、`TestAuthLoginRollbackFailureIsSaid`）。戻す。

- [ ] **Step 4: コミット**

```bash
git add internal/app/usecase/accounts/auth_test.go
git commit -m "test(accounts): config の書き込みの失敗を書けないディレクトリで作る"
```

注意: `auth_test.go` は Task C13 とも触る。`git add -p` で、その Task のハンクだけを入れる。

### Task F1: gitrepo の Files と ReadFile が、欠けた object のエラーを返す

壊れた（object が欠けた）Store の挙動だけを変える例外。壊れていない Store の挙動は変えない。

**Files:**
- Modify: `internal/lib/gitrepo/tree.go`
- Test: `internal/lib/gitrepo/tree_test.go`

**外すもの:** なし（不具合の修正）。

**わかったこと:**
- `object.TreeWalker.Next` は下の tree の `GetTree` が失敗すると err を `io.EOF` に替えて返す。`Files` は下の tree が欠けると一覧を黙って打ち切る。
- 調べているうちに同じ種類の穴がもう 1 つ出た。go-git の `Tree.File` は、blob が欠けているときも、途中の tree が欠けているときも `object.ErrFileNotFound` を返す。そのため `Repo.ReadFile` は blob が欠けた壊れた Store で `fs.ErrNotExist`（「ファイルなし」）を返していた。Task F2 は `ReadFile` の `fs.ErrNotExist` だけを「ない」と読むので、`ReadFile` を先に直す必要がある。
- `Files` の `subtree` のエラーを yield した後に return しない点（旧 125-133 行）: `subtree` は失敗すると `ok=false` を返すので、すぐ下の `if !ok { return }` で必ず止まる。実害はない。書き換えない。
- `Files` の呼び出し側は、本番 5 か所とテスト（replicate の 3 か所、gitrepo の tree_test）。すべて、エラーを受けると止まる。握りつぶしはない。

| 呼び出し | エラーの扱い |
|---|---|
| `storage/index/build.go:60` | `return err` |
| `storage/index/workspace.go:68` | `return err` |
| `storage/index/comments.go:78` | `return err` |
| `remote/replicate/sync.go:1041`（newSnapshot） | `return nil, err` |
| `remote/replicate/sync.go:1134`（filesUnderForwards） | 最初の 1 件で `return err == nil, err` |
| `replicate/property_test.go:670,736`、`forward_test.go:142` | `t.Fatal(err)` |

**Interfaces:**
- Produces: `func (r *Repo) ReadFile(tree plumbing.Hash, path string) ([]byte, error)`（署名は同じ）。tree の中にない path だけが `fs.ErrNotExist`。tree・blob が tree の中にあるのにリポジトリにないときは、`fs.ErrNotExist` ではない（`plumbing.ErrObjectNotFound` を包んだ）エラー。
- Produces: `Files` は、下の tree が読めないとき `("", err)` を yield して止まる。err は `<tree のパス>: object not found`。

- [ ] **Step 1: 失敗するテストを書く**

`internal/lib/gitrepo/tree_test.go` の末尾に足す（import は既存の `errors`・`io/fs`・`path/filepath` で足りる）。`removeLoose`（storage_test.go）と `isLoose`（maintain_test.go）は同じパッケージにある。loose でなければ、テストの前提が崩れているので落とす。

```go
// A directory below the prefix whose tree object is gone is an error, not a
// listing that stops there.
func TestFilesMissingTreeBelowPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := commitFiles(t, r, dir, map[string]string{"tasks/x/a.md": "a", "tasks/y/b.md": "b", "tasks/z.md": "z"})
	tr, err := r.tree(head)
	if err != nil {
		t.Fatal(err)
	}
	e, err := tr.FindEntry("tasks/x")
	if err != nil {
		t.Fatal(err)
	}
	if !isLoose(dir, e.Hash) {
		t.Fatal("the tree object of tasks/x is not loose")
	}
	removeLoose(t, filepath.Join(dir, ".git"), e.Hash)
	r, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var failed bool
	var got []string
	for p, err := range r.Files(head, "tasks/") {
		if err != nil {
			failed = true
			continue
		}
		got = append(got, p)
	}
	if !failed {
		t.Errorf("Files(tasks/) gave no error with the tree object of tasks/x gone; listed %v", got)
	}
}

// A blob that is in the tree but not in the repository is an error, not a
// missing file.
func TestReadFileMissingBlobObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	head := commitFiles(t, r, dir, map[string]string{"tasks/x/a.md": "a", "tasks/b.md": "b"})
	tr, err := r.tree(head)
	if err != nil {
		t.Fatal(err)
	}
	e, err := tr.FindEntry("tasks/x/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if !isLoose(dir, e.Hash) {
		t.Fatal("the blob of tasks/x/a.md is not loose")
	}
	removeLoose(t, filepath.Join(dir, ".git"), e.Hash)
	r, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadFile(head, "tasks/x/a.md"); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile of a path whose blob is gone: err = %v, want an error that is not fs.ErrNotExist", err)
	}
	if b, err := r.ReadFile(head, "tasks/b.md"); err != nil || string(b) != "b" {
		t.Errorf("ReadFile of another file = %q, %v", b, err)
	}
	if _, err := r.ReadFile(head, "tasks/nope.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile of a missing path: err = %v, want fs.ErrNotExist", err)
	}
}
```

Open で開き直すのは、書いた側のハンドルが読んだ object をメモリに持っているため（開き直さないと欠けても読める）。

- [ ] **Step 2: 落ちることを見る**

Run: `go test -count=1 ./internal/lib/gitrepo -run 'TestFilesMissingTreeBelowPrefix|TestReadFileMissingBlobObject'`
Expected: FAIL（修正前の実測。`TestFilesMissingTreeBelowPrefix` は `Files(tasks/) gave no error ...; listed []`。`TestReadFileMissingBlobObject` は ReadFile が `fs.ErrNotExist` を返すので落ちる）

- [ ] **Step 3: `Files` を、TreeWalker をやめて自分で辿る形に直す**

`internal/lib/gitrepo/tree.go`。変更前（`Files` の末尾）:

```go
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
			if name = dir + name; !e.Mode.IsFile() || !strings.HasPrefix(name, prefix) {
				continue
			}
			if !yield(name, nil) {
				return
			}
		}
	}
}
```

変更後:

```go
		r.walkFiles(t, dir, prefix, yield)
	}
}

// walkFiles yields the files below t, whose path is dir, that start with
// prefix, in entry order. It does not enter a directory that no such file is
// below. A directory whose tree cannot be read is yielded as an error and the
// walk stops: object.TreeWalker would turn that error into the end of the
// listing. It reports whether the caller wants more.
func (r *Repo) walkFiles(t *object.Tree, dir, prefix string, yield func(string, error) bool) bool {
	for _, e := range t.Entries {
		name := dir + e.Name
		if e.Mode == filemode.Dir {
			// No file below it can start with prefix.
			if d := name + "/"; !strings.HasPrefix(d, prefix) && !strings.HasPrefix(prefix, d) {
				continue
			}
			sub, err := r.r.TreeObject(e.Hash)
			if err != nil {
				yield("", fmt.Errorf("%s: %w", name, err))
				return false
			}
			if !r.walkFiles(sub, name+"/", prefix, yield) {
				return false
			}
			continue
		}
		if !e.Mode.IsFile() || !strings.HasPrefix(name, prefix) {
			continue
		}
		if !yield(name, nil) {
			return false
		}
	}
	return true
}
```

枝刈り（`!HasPrefix(d, prefix) && !HasPrefix(prefix, d)`）が要る理由: 既存の `TestFilesMissingSubtreeObject` は、`tasks/x` の tree が欠けた Store で `Files(head, "tasks/b")` が成功することを確かめている。旧実装は walker が `tasks/b.md` を返した後で `tasks/x` に当たって EOF になり、たまたま成功していた。枝刈りがないと、prefix と関係のない壊れた directory で失敗してしまう。枝刈りで落とす directory の下には prefix に合うファイルがないので、壊れていない Store の結果は変わらない。名前の順は walker と同じ（tree の entry の順、directory は入った先を先に）。

`errors`・`io` の import は `tree.go` の他の関数（`tree`、`ReadFile`）がまだ使う。

- [ ] **Step 4: `ReadFile` を、blob と tree の欠けを「ない」と混ぜない形に直す**

変更前:

```go
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
```

変更後（`rd` より下は同じ）:

```go
// ReadFile returns the content of path in tree. A path that is not in the
// tree is fs.ErrNotExist. A tree or blob that is in the tree but not in the
// repository is an error that is not fs.ErrNotExist: go-git reports both as
// a missing file.
func (r *Repo) ReadFile(tree plumbing.Hash, path string) ([]byte, error) {
	t, err := r.tree(tree)
	if err != nil {
		return nil, err
	}
	e, err := t.FindEntry(path)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err != nil || !e.Mode.IsFile() {
		return nil, fmt.Errorf("%s: %w", path, fs.ErrNotExist)
	}
	blob, err := r.r.BlobObject(e.Hash)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	rd, err := blob.Reader()
	if err != nil {
		return nil, err
	}
```

`FindEntry` の他のエラー（path にない、途中が directory でない、path の形が不正）は、従来どおり `fs.ErrNotExist` にする。directory を指す path も従来どおり「ない」。`ErrObjectNotFound` だけを別にする。

- [ ] **Step 5: テストを回す**

Run: `go test -count=1 ./internal/lib/gitrepo`
Expected: PASS（修正後の実測: パッケージ全体 2.7 秒。`TestFilesMissingSubtreeObject` も通る）

- [ ] **Step 6: コミット**

```bash
git add internal/lib/gitrepo/tree.go internal/lib/gitrepo/tree_test.go
git commit -m "fix(gitrepo): 欠けた tree・blob を Files と ReadFile が黙って流さない"
```

---

### Task F2: replicate の snapshot が、読めないファイルを「消えた」とみなさない

**Files:**
- Modify: `internal/app/remote/replicate/sync.go`
- Create: `internal/app/remote/replicate/unreadable_test.go`

**外すもの:** なし（不具合の修正）。`snapshot.Read` が `ReadFile` のすべてのエラーを nil（ファイルなし）にしていた。

**Interfaces:**
- Consumes: Task F1 の `ReadFile`（欠けた blob は `fs.ErrNotExist` ではない）。
- Produces: `snapshot.err error`（`Read` が `fs.ErrNotExist` 以外で失敗した最初のエラー）。`Read` の署名は変えない。
- Produces: テスト用 `func (c *cluster) hideFile(i int, rel string)`（Task F3 も使う）と `func remoteHead(t *testing.T, c *cluster) plumbing.Hash`。

**署名を変えなかった理由:** `snapshot.Read` は `merge.Files`（`internal/app/domain/merge/repair.go` の interface。`Read(path string) []byte`）を満たすためのもの。`merge.Repair` は `Read` を 10 か所以上で呼び、テストの `memFiles` もこの形で、domain の層まで直すことになる。すでに同じ事情の `Author` に `keepError`（最初のエラーを覚えて、呼び出し側が `merge.Repair` の後で調べる）がある。それに合わせ、`snapshot` が最初のエラーを覚え、呼び出し側が `merge.Repair` の後で調べる。呼び出しは 2 か所（merge と repairHead）で、どちらも結果を使う前（書き込み・コミット・push の前）に調べる。

**調べたこと:** gitrepo の「ない」を表すエラーは `fs.ErrNotExist`（`ReadFile` の doc コメントのとおり。`schemaAt` など他の呼び出し側も同じ判定をしている）。

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/remote/replicate/unreadable_test.go` を作る。

```go
package replicate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// hideFile moves the loose blob of the file at rel in clone i out of the
// object store, so that the file cannot be read from HEAD, and opens the
// store again. The blob comes back when the test ends.
func (c *cluster) hideFile(i int, rel string) {
	c.t.Helper()
	dir := c.stores[i].Dir()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		c.t.Fatal(err)
	}
	h := plumbing.ComputeHash(plumbing.BlobObject, data).String()
	loose := filepath.Join(dir, ".git", "objects", h[:2], h[2:])
	if _, err := os.Stat(loose); err != nil {
		c.t.Fatalf("the blob of %s is not a loose object: %v", rel, err)
	}
	aside := filepath.Join(c.t.TempDir(), h)
	if err := os.Rename(loose, aside); err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { _ = os.Rename(aside, loose) })
	// The open handle keeps the objects it has read in memory.
	s, err := store.Open(dir)
	if err != nil {
		c.t.Fatal(err)
	}
	c.stores[i] = s
	// The blob may be in a pack too, which a fetch brings.
	if _, err := s.Repo().ReadFile(c.headTree(i), rel); err == nil {
		c.t.Fatalf("%s can still be read: the blob is packed", rel)
	}
}

// A file of HEAD that cannot be read is not a file that is gone: the repair
// fails and commits nothing.
func TestRepairHeadFailsOnUnreadableFile(t *testing.T) {
	c := newCluster(t, 1)
	a := c.addTask(0, "a")
	b := c.addTask(0, "b")
	c.set(0, b.ULID, func(t *model.Task) { t.ID = a.ID }) // the repair has a commit to make
	before := mustHead(t, c.stores[0])
	c.hideFile(0, model.TaskPath(a.ULID))
	if r, err := RepairHead(c.stores[0], c.opts[0]); err == nil {
		t.Errorf("RepairHead = %+v, nil; want an error", r)
	}
	if after := mustHead(t, c.stores[0]); after != before {
		t.Errorf("HEAD moved to %s from %s", after, before)
	}
}

// The same for a merge: a parent that cannot be read is not a deleted one,
// so nothing is merged and nothing is pushed.
func TestMergeFailsOnUnreadableFile(t *testing.T) {
	c := newCluster(t, 2)
	a := c.addTask(1, "a") // written by clone 1 itself, so that its blob is loose there
	c.sync(1)
	c.sync(0)
	c.addTask(0, "x")
	c.sync(0)
	y := c.addTask(1, "y")
	c.set(1, y.ULID, func(t *model.Task) { p := a.ULID; t.Parent = &p })
	c.hideFile(1, model.TaskPath(a.ULID))
	head, origin := mustHead(t, c.stores[1]), remoteHead(t, c)
	if r, err := Run(c.stores[1], c.opts[1]); err == nil {
		t.Errorf("Run = %+v, nil; want an error", r)
	}
	if after := mustHead(t, c.stores[1]); after != head {
		t.Errorf("HEAD moved to %s from %s", after, head)
	}
	if after := remoteHead(t, c); after != origin {
		t.Errorf("origin moved to %s from %s", after, origin)
	}
}

// remoteHead is the head of the bare remote itself, not of a clone's copy.
func remoteHead(t *testing.T, c *cluster) plumbing.Hash {
	t.Helper()
	r, err := gitrepo.Open(c.remote)
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}
```

テストの前提（テストが本物の壊し方をしていることの確認）:
- 壊し方は、loose の blob ファイルを `.git/objects/xx/…` から退避すること。`hideFile` は、その blob が loose であること（`os.Stat`）と、開き直した Store で本当に `ReadFile` が失敗すること（pack にも入っていないこと）を確かめ、どちらかが崩れたら落ちる。
- 書いた側のハンドルは読んだ object をメモリに持つので、`store.Open` で開き直す（これがないとテストは常に成功する）。
- merge のテストで、`a` を clone 1 自身が書いたものにしているのは、clone 0 から fetch した blob は pack に入っていて退避できないため（実測: 最初は a を clone 0 で書いて、blob が pack にあり成功してしまった）。

- [ ] **Step 2: 落ちることを見る（`ReadFile` を F1 のまま、`snapshot` は未修正）**

Run: `go test -count=1 ./internal/app/remote/replicate -run 'FailsOnUnreadableFile'`
Expected: FAIL。F1 の前の `ReadFile` だと、`RepairHead = [], nil`（実測。blob の欠けが「ファイルなし」になり、衝突を見落として何も直さない）。F1 の後でも、修正前の snapshot では 2 つとも落ちる:

```
--- FAIL: TestRepairHeadFailsOnUnreadableFile
    unreadable_test.go:57: RepairHead = [], nil; want an error
--- FAIL: TestMergeFailsOnUnreadableFile
    unreadable_test.go:78: Run = {Merges:1 ... Repaired:[{... Kind:id_collision ...}] Pushed:[origin] ...}, nil; want an error
    unreadable_test.go:81: HEAD moved to b3f9a393... from c40a027e...
    unreadable_test.go:84: origin moved to b3f9a393... from 7a8337c8...
```

マージのほうは、読めないファイルがあるまま merge コミットを作り、origin に push していた。

- [ ] **Step 3: `snapshot` を直す**

`internal/app/remote/replicate/sync.go`。変更前:

```go
type snapshot struct {
	r     *gitrepo.Repo
	tree  plumbing.Hash
	over  map[string][]byte
	paths []string // sorted
}
```
```go
func (s *snapshot) Read(p string) []byte {
	if d, ok := s.over[p]; ok {
		return d
	}
	b, err := s.r.ReadFile(s.tree, p)
	if err != nil {
		return nil
	}
	return b
}
```

変更後:

```go
type snapshot struct {
	r     *gitrepo.Repo
	tree  plumbing.Hash
	over  map[string][]byte
	paths []string // sorted
	err   error    // the first failure of Read other than a missing file
}
```
```go
// Read returns the file at p, nil when there is none. A file that exists but
// cannot be read is nil too, and is kept in s.err: merge.Repair cannot take
// the error, so its caller must look at s.err before it uses the result.
func (s *snapshot) Read(p string) []byte {
	if d, ok := s.over[p]; ok {
		return d
	}
	b, err := s.r.ReadFile(s.tree, p)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) && s.err == nil {
			s.err = err
		}
		return nil
	}
	return b
}
```

呼び出し側の 2 か所（`merge.Repair` の直後。どちらも、`maps.Copy`・`dirty`・コミットの前）。

merge（変更前 `if err = errors.Join(err, authorErr); err != nil {` の次に `maps.Copy(files, fixed)`）:

```go
	if err = errors.Join(err, authorErr, snap.err, baseSnap.err); err != nil {
		return nil, err
	}
	maps.Copy(files, fixed)
```

repairHead（`fixed, repaired, dropped, err := merge.Repair(...)` の直後）:

```go
		if err = errors.Join(err, authorErr, snap.err); err != nil {
			return err
		}
```

`newSnapshot`（Paths の一覧を作る `r.Files` のエラー）は、もともとエラーを返しているので変更なし。

- [ ] **Step 4: テストを回す**

Run: `go test -count=1 ./internal/app/remote/replicate -run 'FailsOnUnreadableFile'`
Expected: PASS（実測 0.1 秒）

- [ ] **Step 5: 検出できることを確かめる**

`sync.go` の 2 か所の `errors.Join(err, authorErr, snap.err, baseSnap.err)`・`errors.Join(err, authorErr, snap.err)` から `snap.err`・`baseSnap.err` を外す（`sed -i 's/authorErr, snap.err, baseSnap.err)/authorErr)/; s/authorErr, snap.err)/authorErr)/' sync.go`）。
Run: `go test -count=1 ./internal/app/remote/replicate -run 'FailsOnUnreadableFile'`
Expected: 2 つとも FAIL（実測。Step 2 の出力と同じ形）。元に戻して PASS を確かめる。

- [ ] **Step 6: パッケージのテストを回す**

Run: `go test -count=1 ./internal/app/remote/replicate`
Expected: PASS（実測 13 秒。修正前も同じくらい）

- [ ] **Step 7: コミット**

```bash
git add internal/app/remote/replicate/sync.go internal/app/remote/replicate/unreadable_test.go
git commit -m "fix(replicate): 読めないファイルを消えたとみなさず同期を失敗させる"
```

---

### Task F3: `onFullRepair` を外し、本物の手段で「全ファイルを読まない」を確かめる

**Files:**
- Modify: `internal/app/remote/replicate/sync.go`
- Modify: `internal/app/remote/replicate/round_test.go`

**外すもの:** `replicate.onFullRepair`（sync.go の「Replaced by tests」の var 1 行と、`repairHead` の `onFullRepair()` の呼び出し 1 行）。
**使っていたテスト:** `TestCleanStoreSkipsFullRepair`（round_test.go）、`TestPrefixDuplicateInHeadBeforeSync`（round_test.go）。ほかに使うテストはない（grep 済み）。

**何を確かめるテストになったか:**
- `TestCleanStoreSkipsFullRepair`: 衝突した id の repair が（全ファイルを読んで）直し、そのあとの、きれいな Store の同期は、HEAD のタスクの blob が読めなくても成功する（＝全ファイルを読んでいない）。
- `TestPrefixDuplicateInHeadBeforeSync`: workspace の prefix の重複を知らせる同期は、タスクの blob が読めなくても 2 回とも成功する（＝workspaces だけ読む）。
- 「読んだ回数」は確かめなくなった。確かめるのは「全ファイルを読む repair に入れば失敗するはずの状況で、成功すること」。F2 より前は、読めないファイルが握りつぶされるので、どちらも「常に成功」して検出できなかった。

**Interfaces:**
- Consumes: Task F2 の `(*cluster).hideFile`。

- [ ] **Step 1: `onFullRepair` を外す**

`sync.go` の var から:

```go
	onFullRepair = func() {}              // runs when the repair reads every file of HEAD
```

`repairHead` の:

```go
		if full {
			onFullRepair()
			prefix = ""
		}
```
を
```go
		if full {
			prefix = ""
		}
```
にする。`gofmt` は var のそろえを変えない（実測。`gofmt -l` は何も出さない）。

- [ ] **Step 2: テストを書き直す**

`round_test.go`。変更前:

```go
func TestCleanStoreSkipsFullRepair(t *testing.T) {
	c := newCluster(t, 1)
	n := 0
	replace(t, &onFullRepair, func() { n++ })
	a := c.addTask(0, "a")
	c.sync(0)
	if n != 0 {
		t.Errorf("a clean store read every file %d times", n)
	}
	b := c.addTask(0, "b")
	c.set(0, b.ULID, func(t *model.Task) { t.ID = a.ID })
	if r := c.sync(0); n != 1 || len(r.Repaired) != 1 {
		t.Errorf("full repairs %d, repaired %+v", n, r.Repaired)
	}
}
```

変更後（直前にコメントを付ける）:

```go
// A clean store reads the workspaces alone: a task file that cannot be read
// does not stop the sync, as it would if the repair read every file.
func TestCleanStoreSkipsFullRepair(t *testing.T) {
	c := newCluster(t, 1)
	a := c.addTask(0, "a")
	c.sync(0)
	b := c.addTask(0, "b")
	c.set(0, b.ULID, func(t *model.Task) { t.ID = a.ID })
	if r := c.sync(0); len(r.Repaired) != 1 {
		t.Errorf("repaired %+v", r.Repaired)
	}
	c.hideFile(0, model.TaskPath(a.ULID))
	c.sync(0)
}
```

`TestPrefixDuplicateInHeadBeforeSync`（コメントは既存のまま）。変更前の先頭:

```go
	c := newCluster(t, 1)
	n := 0
	replace(t, &onFullRepair, func() { n++ })
	w := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "WEB", Created: time.Now(), PrefixSet: time.Now()}
```
変更後:

```go
	c := newCluster(t, 1)
	x := c.addTask(0, "x")
	c.sync(0) // the index of HEAD is built; a later sync reads what changed
	w := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "WEB", Created: time.Now(), PrefixSet: time.Now()}
```

`c.putFile(0, model.WorkspacePath(w.ULID), string(b))` の次の行に足す:

```go
	c.hideFile(0, model.TaskPath(x.ULID)) // a task file that cannot be read
```

末尾の（`for` の後の）:

```go
	if n != 0 {
		t.Errorf("read every file %d times", n)
	}
```
を消す。

テスト中の `c.sync(0)` を足した理由: 同期の前にインデックスのキャッシュがないと、`index.Open` が全タスクを読み直し（ここでは blob の欠けで失敗する）、repair まで進まない。実測: これを入れないと `sync 0: tasks/…: object not found` で落ちた。`TestCleanStoreSkipsFullRepair` は最初の `c.sync(0)` が同じ役をする。

- [ ] **Step 3: テストを回す**

Run: `go test -count=1 ./internal/app/remote/replicate -run 'TestCleanStoreSkipsFullRepair|TestPrefixDuplicateInHeadBeforeSync'`
Expected: PASS（実測 0.1 秒）

- [ ] **Step 4: 検出できることを確かめる**

`repairHead` の `prefix := "workspaces/"` を `prefix := ""` にする（repair を常に全ファイルにする）。
Run: 同じコマンド
Expected: 2 つとも FAIL（実測）:

```
--- FAIL: TestCleanStoreSkipsFullRepair
    round_test.go:110: sync 0: tasks/T/4/01M6KVE9QVK8GNPEMDSF8C984T.md: object not found
--- FAIL: TestPrefixDuplicateInHeadBeforeSync
    round_test.go:146: sync 0: tasks/X/6/01M6KVE9SFSH5H7YQ0E3RVDH6X.md: object not found
```

元に戻して PASS を確かめる。（`needsRepair` が常に true を返す壊し方も同じ結果になる: どちらも `full` が真になるため。）

- [ ] **Step 5: 影響するパッケージのテストを回す**

Run: `go vet ./internal/lib/gitrepo ./internal/app/remote/... ./internal/app/storage/... && go test -count=1 ./internal/lib/gitrepo ./internal/app/remote/... ./internal/app/storage/index ./internal/app/storage/store ./internal/app/storage/inbox ./internal/app/usecase/watch ./internal/app/usecase/replication ./internal/app/usecase/tasks`
Expected: すべて PASS（実測 15 秒、-race なし）

- [ ] **Step 6: コミット**

```bash
git add internal/app/remote/replicate/sync.go internal/app/remote/replicate/round_test.go
git commit -m "test(replicate): onFullRepair をやめ、読めない blob で全ファイルを読まないことを確かめる"
```

---

### Task Z: 5b の全体を確かめる

- [ ] **Step 1: 差し込み口が残っていないことを確かめる**

Run:

```bash
grep -rn -e 'afterLockMiss' -e 'afterLock\b' -e 'newStoreHook' -e 'openStore\b' -e 'readHook' -e 'listHook' -e 'lockPoll =' -e 'indexLockPoll =' -e 'indexLockWait =' -e '\bgoos\b' -e 'packCap =' -e 'retryWait =' -e 'repackAt =' -e 'packsAt =' -e 'lookPath =' -e 'SetSpawn' -e 'StubSpawn' -e 'beforePush' -e 'onFullRepair' -e 'ForgetMounts' -e 'ForgetWriter' -e 'FinishLeftover(' -e 'PrepareMoveStore' -e 'func (b \*Box) Import' -e 'uiContext' internal | grep -v _test.go
```

Expected: 0 行（上の表の A・B・C がすべて消えている。名前を変えて残したもの（`store.WithLockWait`・`Session.Spawn` など）は引っかからない）。D の名前（`beforeRename`・`beforeCommit`・`BeforePush`・`afterClone`・`afterPackWrite`・`onLockWait`・`syncFn`・`openHistory`・`Live`・`IndexOpened`・`BeforeUpdate`）は残っている。

- [ ] **Step 2: 全体を確かめる（5b で 1 回だけ）**

Run: `go build ./... && go vet ./... && go vet -tags e2e ./internal/app/cli && make lint && make fmt-check && make tidy-check && mkdir -p .tmp && TMPDIR=$PWD/.tmp go test -race ./...`
Expected: すべて通る。所要は約 3〜4 分（-race 全テスト。C で 2 秒のテストが増えた）。落ちたら失敗の出力・コマンド・-race の有無を報告に残す

- [ ] **Step 3: memory と PR**

memory の `codebase-restructure.md` に 5b の完了を書く。push して PR を作る。本文に、上の「差し込み口の一覧」の表、「5b で決めたこと」1〜9、挙動の変化（決定 19: object が欠けた Store では一覧・読み取り・同期がエラーで止まる）を書く。
