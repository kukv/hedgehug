# 構成見直し 回 1: 移動と改名 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 全パッケージを spec 2.1 のディレクトリへ移し、名前を ops → usecase、sync → replicate、context → current、doctor → maintenance に変え、CLI の本体を `internal/app/cli` に移す。挙動は変えない。

**Architecture:** スクリプトで import のパスとパッケージ名を書き換えてから `git mv` する。手で直すのは、相対パス・CLI の main・CI の shard だけにする。mount と context は回 3 で解体するまで `usecase/mount`・`usecase/current` に仮置きする。

**Tech Stack:** Go、perl（一括置換）、golangci-lint 2.15.0（`golangci-lint fmt` で import の並びを直す）。

**Spec:** `docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`（2.1・2.2・4.1・6 節）。概要 plan: `docs/superpowers/plans/2026-11-04-codebase-restructure.md`

## Global Constraints

- 概要 plan の Global Constraints をすべて守る
- この回では、移動・改名・パスの書き換えのほかに何も変えない（関数の分割や依存の整理は回 2 以降）
- `docs/superpowers/specs/` と `docs/superpowers/plans/` の既存の文書に出てくる旧パスは書き換えない（その時点の記録なので）。親設計の書き直しは回 5
- `.goreleaser.yaml` は変えない（`main: ./cmd/hedgehug` と `-X main.version` のまま動くようにする）

## 移動の一覧

| 旧 | 新 | パッケージ名 |
| --- | --- | --- |
| internal/atomicfile | internal/lib/atomicfile | 同じ |
| internal/plural | internal/lib/plural | 同じ |
| internal/gitrepo（gitrepotest を含む） | internal/lib/gitrepo | 同じ |
| internal/github | internal/lib/github | 同じ |
| internal/fakegh | internal/lib/fakegh | 同じ |
| internal/testenv | internal/lib/testenv | 同じ |
| internal/model | internal/app/domain/model | 同じ |
| internal/schema | internal/app/domain/schema | 同じ |
| internal/merge | internal/app/domain/merge | 同じ |
| internal/store | internal/app/storage/store | 同じ |
| internal/index | internal/app/storage/index | 同じ |
| internal/history（historytest を含む） | internal/app/storage/history | 同じ |
| internal/inbox | internal/app/storage/inbox | 同じ |
| internal/query | internal/app/storage/query | 同じ |
| internal/config | internal/app/storage/config | 同じ |
| internal/view | internal/app/storage/view | 同じ |
| internal/sync | internal/app/remote/replicate | `sync` → `replicate` |
| internal/auth | internal/app/remote/auth | 同じ |
| internal/ops | internal/app/usecase | `ops` → `usecase` |
| internal/mount | internal/app/usecase/mount | 同じ（仮置き） |
| internal/context | internal/app/usecase/current | `context` → `current`（仮置き） |
| internal/doctor | internal/app/usecase/maintenance | `doctor` → `maintenance` |
| internal/server（openapigen を含む） | internal/app/server | 同じ |
| cmd/hedgehug（main.go の中身を含む） | internal/app/cli | `main` → `cli` |

調べてわかっていること（main f965465）:

- internal/context を import する 11 ファイルはすべて `hhctx` の別名を付けている。internal/sync は 42 ファイルのうち 11 ファイルが別名（`hhsync` が 10、`isync` が 1）、31 ファイルが別名なし。別名なしで internal/sync を import するファイルは、標準の `sync` を別名なしでは import していない
- ops・mount・doctor を別名付きで import するファイルはない
- ops にはパッケージの関数 `current`（`internal/ops/delete.go:183`、索引を更新してタスクの今の姿を返す）があり、パッケージ `current` の import とぶつかってビルドが落ちる。`refreshed` に改名する（Task 1 Step 4）。ローカル変数の `current`（storeops.go・addhere.go・cmd の add.go）は、2026-11-04 の空実行ではぶつからなかった
- 2026-11-04 に、この plan の Task 1〜2 を空実行して、`go build`・`go vet`・`golangci-lint run`・`go test ./...`（`-race` なし）が通ることを確かめた（確かめた後は元に戻した）。空実行で出た直しは、上の `refreshed` と Task 2 Step 2 の package コメントの 2 つで、どちらも plan に書いてある
- 移動でずれる相対パス: `internal/server/gen.go` の `go:generate`（`../../api/openapi.yaml`）、`internal/server/server_test.go:161` と `internal/server/helpers_test.go:46` の `filepath.Join("..", "..", "api", "openapi.yaml")`、`cmd/hedgehug/main_test.go:54` の `"../../cmd", "../../internal", "../../tools"`、`cmd/hedgehug/e2e_test.go:42` の `"../../tools/devenv"`、`cmd/hedgehug/helpers_test.go:339` の `goBuildArgs(builtBin, ".")`（CLI のテストはバイナリを `go build .` で作る。移動後は main のパッケージを指す必要がある）
- `tools/devenv` と `tools/licenses` も internal のパッケージを import している（スクリプトの対象に含める）
- Go 以外で旧パスを書いているのは、`.github/workflows/ci-backend.yaml`（shard）、`Makefile`（`bench-history`）、`web/` のコメントとテストの説明文 16 か所

---

### Task 1: CLI 以外のパッケージを移して改名する

**Files:**
- Move: 上の一覧の `internal/*` すべて（cmd/hedgehug を除く）
- Modify: リポジトリ内のすべての `*.go`（import と修飾子）、`web/**/*.ts`・`web/**/*.svelte` のコメント
- Modify: `internal/app/server/gen.go:3`、`internal/app/server/server_test.go:161`、`internal/app/server/helpers_test.go:46`
- Modify: `internal/app/usecase/delete.go`・`move.go`・`delete_test.go`（関数 `current` → `refreshed`）

**Interfaces:**
- Produces: 新しい import パス（一覧の「新」）と新しいパッケージ名 `usecase`・`replicate`・`current`・`maintenance`。Task 2 の CLI はこれを使う

- [ ] **Step 1: 書き換えのスクリプトを scratchpad に置く**

ファイル `<scratchpad>/move.sh`（リポジトリには入れない）:

```bash
#!/bin/bash
# 構成見直し 回 1 の移動と改名。リポジトリのルートで 1 回だけ実行する。
set -euo pipefail
M=github.com/kukv/hedgehug/internal
moves=(
  atomicfile:lib/atomicfile
  plural:lib/plural
  gitrepo:lib/gitrepo
  github:lib/github
  fakegh:lib/fakegh
  testenv:lib/testenv
  model:app/domain/model
  schema:app/domain/schema
  merge:app/domain/merge
  store:app/storage/store
  index:app/storage/index
  history:app/storage/history
  inbox:app/storage/inbox
  query:app/storage/query
  config:app/storage/config
  view:app/storage/view
  sync:app/remote/replicate
  auth:app/remote/auth
  ops:app/usecase
  mount:app/usecase/mount
  context:app/usecase/current
  doctor:app/usecase/maintenance
  server:app/server
)

# 1. パスの書き換え（import とコメント）。新しいパスはどれも lib/ か app/ で
#    始まり、旧パッケージに lib・app はないので、書き換えた後のパスに再び一致しない。
#    直前の / は許す（import の "github.com/kukv/hedgehug/internal/..." を対象にするため）。
mapfile -t gofiles < <(git ls-files '*.go')
mapfile -t webfiles < <(git ls-files 'web/*.ts' 'web/*.svelte')
for m in "${moves[@]}"; do
  old=${m%%:*} new=${m#*:}
  perl -pi -e "s#(?<![\\w])internal/$old(?![\\w])#internal/$new#g" "${gofiles[@]}" "${webfiles[@]}"
done

# 2. ディレクトリの移動。ops を先に移してから、その下に mount などを入れる。
mkdir -p internal/lib internal/app/domain internal/app/storage internal/app/remote
for m in "${moves[@]}"; do
  old=${m%%:*} new=${m#*:}
  git mv "internal/$old" "internal/$new"
done

# 3. パッケージ名の変更（パッケージの中）。
rename_pkg() { # dir old new
  perl -pi -e "s/^package $2(_test)?\$/package $3\$1/; s#^// Package $2 #// Package $3 #" "$1"/*.go
}
rename_pkg internal/app/usecase ops usecase
rename_pkg internal/app/remote/replicate sync replicate
rename_pkg internal/app/usecase/current context current
rename_pkg internal/app/usecase/maintenance doctor maintenance

# 4. 修飾子の書き換え（使う側）。
mapfile -t gofiles < <(git ls-files '*.go')
importers() { grep -lF "\"$M/$1\"" "${gofiles[@]}" || true; }
mapfile -t f < <(importers app/usecase)
[ ${#f[@]} -gt 0 ] && perl -pi -e 's/\bops\.([A-Z])/usecase.$1/g' "${f[@]}"
mapfile -t f < <(importers app/usecase/maintenance)
[ ${#f[@]} -gt 0 ] && perl -pi -e 's/\bdoctor\.([A-Z])/maintenance.$1/g' "${f[@]}"
mapfile -t f < <(importers app/usecase/current)
[ ${#f[@]} -gt 0 ] && perl -pi -e "s#^(\\s+)hhctx (\"$M/app/usecase/current\")#\$1\$2#; s/\\bhhctx\\./current./g" "${f[@]}"
mapfile -t f < <(importers app/remote/replicate)
for x in "${f[@]}"; do
  if grep -qE "^\s+(hhsync|isync) \"$M/app/remote/replicate\"" "$x"; then
    # 別名付き: 別名を外し、別名の修飾子だけを直す（標準の sync. はそのまま）。
    perl -pi -e "s#^(\\s+)(hhsync|isync) (\"$M/app/remote/replicate\")#\$1\$3#; s/\\b(hhsync|isync)\\./replicate./g" "$x"
  else
    perl -pi -e 's/\bsync\.([A-Z])/replicate.$1/g' "$x"
  fi
done
```

- [ ] **Step 2: スクリプトを実行する**

Run: `bash <scratchpad>/move.sh && git status --short | head`
Expected: エラーなしで終わり、`R  internal/ops/... -> internal/app/usecase/...` のような rename が並ぶ。`internal/` の直下が `app` と `lib` だけになっている（`ls internal` で確かめる）

- [ ] **Step 3: server の相対パスを 1 段深くする**

`internal/app/server/gen.go:3`:

```go
//go:generate go run ./openapigen -o ../../../api/openapi.yaml
```

`internal/app/server/server_test.go:161` と `internal/app/server/helpers_test.go:46` の `filepath.Join("..", "..", "api", "openapi.yaml")` を、どちらも次にする:

```go
filepath.Join("..", "..", "..", "api", "openapi.yaml")
```

- [ ] **Step 4: usecase の関数 current を refreshed に改名する**

`internal/app/usecase/delete.go:181-183` の関数（索引を更新してタスクの今の姿を返す）は、パッケージ `current` の import とぶつかる（`current already declared through import of package current`）。呼んでいる箇所（`delete.go` の 96・153・318 行、`move.go` の 115・119 行、`delete_test.go` の 153 行）ごと改名する。

```bash
perl -pi -e 's/\bcurrent\(ix, /refreshed(ix, /g; s/^func current\(/func refreshed(/; s#^// current refreshes ix#// refreshed refreshes ix#' internal/app/usecase/delete.go internal/app/usecase/move.go internal/app/usecase/delete_test.go
```

- [ ] **Step 5: import の並びを直してビルドする**

Run: `golangci-lint fmt ./... && go build ./... && go vet ./...`
Expected: cmd/hedgehug を含めてビルドと vet が通る（CLI はまだ cmd/hedgehug にあり、新しいパスを import している）。

ほかのエラーが出たら、スクリプトの置換が足りない箇所なので、エラーの行を直す（置換の漏れを手で直すだけで、ロジックは変えない）

- [ ] **Step 6: 旧パスが残っていないことを確かめる**

Run:

```bash
git grep -nE '(^|[^[:alnum:]_])internal/(atomicfile|plural|gitrepo|github|fakegh|testenv|model|schema|merge|store|index|history|inbox|query|config|view|sync|auth|ops|mount|context|doctor|server)([^[:alnum:]_]|$)' -- ':!docs/superpowers' ':!.github' ':!Makefile'
```

Expected: 何も出ない（`.github` と `Makefile` は Task 3 で直す）

- [ ] **Step 7: 移したパッケージのテストを回す**

Run: `go test ./internal/... ./tools/...`
Expected: PASS（`-race` なし。全テストは Task 3 で 1 回だけ回す）

- [ ] **Step 8: コミット**

```bash
git add -A
git commit -m "refactor: パッケージを internal/lib と internal/app の層に移し、ops・sync・context・doctor を改名する

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: CLI の本体を internal/app/cli に移す

**Files:**
- Move: `cmd/hedgehug/*`（`testdata/` を含む）→ `internal/app/cli/`。`cmd/hedgehug/main.go` は `internal/app/cli/run.go` にする
- Create: `cmd/hedgehug/main.go`
- Modify: `internal/app/cli/run.go`（旧 main.go の `main` と `version`）、`internal/app/cli/helpers_test.go:339`、`internal/app/cli/main_test.go:54`、`internal/app/cli/e2e_test.go:42`

**Interfaces:**
- Consumes: Task 1 の新しい import パス
- Produces: `func cli.Main(v string) int`。`cmd/hedgehug/main.go` だけがこれを呼ぶ

- [ ] **Step 1: ファイルを移してパッケージ名を変える**

```bash
mkdir -p internal/app/cli
git mv cmd/hedgehug/main.go internal/app/cli/run.go
for f in cmd/hedgehug/*; do git mv "$f" internal/app/cli/; done
perl -pi -e 's/^package main(_test)?$/package cli$1/' internal/app/cli/*.go
```

- [ ] **Step 2: run.go の main を Main にする**

`internal/app/cli/run.go` の先頭の package コメント（今の 1〜2 行、`// Command hedgehug is a local-first task manager ...`）を次にする（revive の package-comments が `Package cli ...` の形を求める）。元の文は Step 3 で cmd/hedgehug/main.go に移す:

```go
// Package cli is the body of the hh command: its subcommands, flags and
// output. cmd/hedgehug only calls Main.
package cli
```

`version` の宣言（今の 25〜27 行）を次にする:

```go
// version is what --version prints. Main sets it from the version that
// cmd/hedgehug gets with -ldflags at release; the tests keep "dev".
var version = "dev"
```

`func main()`（今の 48〜59 行）を次にする:

```go
// Main runs the command with the arguments of the process and returns its
// exit code. v is the version --version prints.
func Main(v string) int {
	version = v
	return run(os.Args[1:], &env{
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		getenv:     os.Getenv,
		isTerminal: isTerminal,
		now:        time.Now,
		getwd:      os.Getwd,
		openURL:    openURL,
	})
}
```

- [ ] **Step 3: cmd/hedgehug/main.go を作る**

```go
// Command hedgehug is a local-first task manager that uses git as its storage
// and sync layer, with a CLI and a GUI in a single binary. Its body is
// internal/app/cli.
package main

import (
	"os"

	"github.com/kukv/hedgehug/internal/app/cli"
)

// version is what --version prints. GoReleaser sets it with -ldflags at
// release.
var version = "dev"

func main() {
	os.Exit(cli.Main(version))
}
```

- [ ] **Step 4: テストの相対パスを直す**

`internal/app/cli/helpers_test.go:339`（`buildHedgehug`）: CLI のテストはパッケージ自身ではなく main のパッケージをビルドする。

```go
		if out, err := exec.Command("go", goBuildArgs(builtBin, "../../../cmd/hedgehug")...).CombinedOutput(); err != nil { //nolint:gosec // the arguments are ours
```

`internal/app/cli/main_test.go:54`:

```go
	for _, root := range []string{"../../../cmd", "../../../internal", "../../../tools"} {
```

`internal/app/cli/e2e_test.go:42`:

```go
		if out, err := exec.Command("go", goBuildArgs(devenvBin, "../../../tools/devenv")...).CombinedOutput(); err != nil { //nolint:gosec // the arguments are ours
```

ほかに相対パスがないことを確かめる。Run: `grep -n '"\.\./\|goBuildArgs(' internal/app/cli/*.go`
Expected: 上の 3 か所だけ

- [ ] **Step 5: ビルドとテスト**

Run: `golangci-lint fmt ./... && go build ./... && go vet ./... && go test ./internal/app/cli/ -run 'TestRun|TestNoShortIdentifiers' -v 2>&1 | tail -20`
Expected: PASS。`TestRun` の `--version prints the version` が `hedgehug dev` を出す

- [ ] **Step 6: ビルドしたバイナリの版を確かめる**

Run: `go build -ldflags '-X main.version=v9.9.9' -o <scratchpad>/hh ./cmd/hedgehug && <scratchpad>/hh --version`
Expected: `hedgehug v9.9.9`（GoReleaser の `-X main.version` が今までどおり効く）

- [ ] **Step 7: コミット**

```bash
git add -A
git commit -m "refactor: CLI の本体を internal/app/cli に移し、cmd/hedgehug は main.go だけにする

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: CI と Makefile のパスを直し、全体を確かめる

**Files:**
- Modify: `.github/workflows/ci-backend.yaml:136-157`、`Makefile:69`

**Interfaces:**
- Consumes: Task 1・2 の新しいパス

- [ ] **Step 1: shard のパスを書き換える**

`.github/workflows/ci-backend.yaml` の shard の説明（今の 136〜139 行）と `case`（今の 151〜157 行）を次にする。shard の名前と分け方（どのテストがどの shard か）は変えない。旧 ops の shard は `internal/app/usecase` のパッケージ 1 つだけを指す（仮置きの mount・current・maintenance は今までどおり others に入る）。

```yaml
      # 何を回すかは、パッケージごとの時間（下の Package times）で均した分け方:
      # ops = internal/app/usecase、cli = internal/app/cli、sync-property・sync-property-stores = replicate の
      # 2つの property テスト（seed が多く重い）、sync-server = replicate の残りと server、
      # others = 残りのパッケージすべて（新しいパッケージは何もしなくても others に入る）。
```

```bash
          case "$SHARD" in
            ops) args=("$mod/internal/app/usecase") ;;
            cli) args=("$mod/internal/app/cli") ;;
            sync-property) args=(-run '^TestPropertyConverges$' "$mod/internal/app/remote/replicate") ;;
            sync-property-stores) args=(-run '^TestPropertyConvergesStores$' "$mod/internal/app/remote/replicate") ;;
            sync-server) args=(-skip "$prop" "$mod/internal/app/remote/replicate" "$mod/internal/app/server") ;;
            others) args=($(go list ./... | grep -vxF -e "$mod/internal/app/usecase" -e "$mod/internal/app/cli" -e "$mod/internal/app/remote/replicate" -e "$mod/internal/app/server")) ;;
            *) echo "unknown shard $SHARD"; exit 1 ;;
          esac
```

- [ ] **Step 2: Makefile の bench-history を直す**

`Makefile:69`:

```make
	HH_BENCH=1 go test ./internal/app/storage/history -run '^$$' -bench . -benchtime 5x -timeout 60m
```

- [ ] **Step 3: 旧パスがどこにも残っていないことを確かめる**

Run: Task 1 Step 6 の `git grep` から `':!.github' ':!Makefile'` を外して実行する
Expected: 何も出ない

- [ ] **Step 4: 生成物が変わらないことを確かめる**

Run: `go generate ./... && git diff --exit-code -- api/openapi.yaml`
Expected: 差分なし（OpenAPI はパッケージの場所に依存しない）

- [ ] **Step 5: 全体の検査**

Run: `make tidy-check lint fmt-check && make test`
Expected: すべて PASS。落ちたら、失敗の出力と実行条件（`make test` は `-race` 付き）を作業記録に残してから直す

- [ ] **Step 6: コミットして PR を作る**

```bash
git add -A
git commit -m "ci: shard と Makefile のパスを新しい構成に合わせる

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push -u origin HEAD
gh pr create --title "refactor: 構成見直し 回 1（移動と改名）" --body "$(cat <<'EOF'
spec: docs/superpowers/specs/2026-11-04-codebase-restructure-design.md（2.1・2.2・4.1・6 節）
plan: docs/superpowers/plans/2026-11-04-restructure1-move.md

- 全パッケージを internal/lib と internal/app の層へ移す
- ops → usecase、sync → replicate、context → current、doctor → maintenance に改名
- CLI の本体を internal/app/cli に移し、cmd/hedgehug は main.go だけにする
- mount と context は回 3 で解体するまで usecase/mount・usecase/current に仮置き
- CI の shard のパスを書き換え（分け方は変えない）

挙動の変更はありません。

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: CI の `test` を含むすべてのチェックが通る。shard ごとの時間が main と大きく変わっていない（shard の見直しは回 5）
