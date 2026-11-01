# CI の短縮とローカルのゴミの削減 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Windows をやめ、テストを shard に分けて同時に回し、テスト自体の直列と固定の待ちを減らして、CI の待ちを約 7.5 分から 2 分前後にする。あわせて、ローカルのビルドキャッシュと一時ファイルが溜まらないようにする。

**Architecture:** アプリの構造は変えない（ポートとアダプター化はしない）。CI はテストを `os × shard` のマトリクスに分け、`test` という名前のまとめのジョブで結果を1つにする。テスト用の部品として `internal/testenv`（一時ディレクトリの付け替えと残りの掃除、条件を待つ `WaitFor`）を足す。

**Tech Stack:** Go 1.27.3、GitHub Actions、gotestsum 1.13.0、golangci-lint v2.15.0、mise、GoReleaser v2

**Spec:** `docs/superpowers/specs/2026-11-02-ci-speedup-refactor-design.md`

## Global Constraints

- **ローカルでテストを回さない。** 確かめてよいのは `go build ./...`・`go vet ./...`・`golangci-lint run ./...`・`golangci-lint fmt --diff ./...` と、Task 4 の `go test -count=1 ./internal/testenv/` だけ。それ以外のテストは CI で確かめる（利用者の指示。Windows 側のストレージに響くため）
- Task 4 の `go test` だけは `TMPDIR` を worktree の中の `.tmp/`（gitignore 済み）に向ける: `mkdir -p .tmp && TMPDIR="$PWD/.tmp" go test -count=1 ./internal/testenv/`
- この worktree の Bash は、git を呼ぶかどうか判定できないコマンドを拒む。コマンドは1行ずつの単純な形にし、`gh`・`sed` などの引数を変数や `$(...)` で組み立てない。パスに `github` を含む値を変数に入れて使わない。ファイルの編集は Edit・Write で行う
- Bash の timeout は 600000
- コードのコメントは英語（既存の流儀）。直列のテストの理由のコメントだけは既存の流儀どおり日本語の `// 直列: …`（例: `internal/store/move_test.go:533`）
- コミットのメッセージは `feat:`/`fix:`/`docs:`/`refactor:`/`test:`/`chore:`/`ci:` の接頭辞と日本語の要約。末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`
- 1 Task = 1 コミット（Task 7 は手順ごと）
- 過去の spec・plan（`docs/superpowers/` の 2026-11-02 より前のもの）は記録なので直さない
- 触るのは各 Task に書いた範囲だけ。隣のコードを「ついでに」直さない
- ブランチは `ci-speedup`（Task 2 で今のブランチの名前を変える）

## spec にない細部（plan で決めたこと。利用者の承認を得る）

1. **キャッシュのキーは shard ごとに分ける。** spec の最初の版は「shard では分けず、保存は shard 1 だけ」としていたが、各 shard がビルドするパッケージは違うので、shard 1 のキャッシュでは shard 2・3 が当たらない。キーに shard の番号を入れ、各 shard が自分の分を保存する（いまの1つ約 385MB × 6 + nogit 315MB ≒ 2.6GB。リポジトリの上限 10GB に収まる）。spec はこの plan と同じコミットで直した
2. **CRLF の扱いと同じく、ビューの名前の予約語（`CON`・`NUL` など、`internal/view/name.go`）の検査は残す。** WSL の Store を Windows の git で clone する人がいると、その名前のファイルを作れないため。データの持ち運びの約束で、Windows 用の分岐ではない
3. **Windows に触れたコメントのうち、そのまま残るコードの理由を説明しているもの（`mount.go:57`、`rewrite.go:278`、`session.go:240`、`storage.go:11`、`maintain.go:150`、`remote.go:272`、`tx.go:431` など）は直さない。** 最小の変更にとどめる。消すコードに付いていたコメントは一緒に消す
4. **sleep を減らすための本番コードの小さな変更**（挙動は変わらない）:
   - `store.acquire` と `store.lockStore` の待ちを `min(lockPoll, time.Until(deadline))` にする（`gitrepo.lockIndex` と同じ形）。待ちの上限ちょうどで諦めるようになる（いまは最大 50ms 遅れる）
   - `gitrepo.indexLockPoll` を const から var にする（テストで縮めるため）
   - `server.Poller` に `interval` フィールドを足す（既定は `pollInterval`。テストで 10ms にする）
   - `store` に `onLockWait` フックを足す（`lockStore` が待ちに入る直前に呼ぶ。テストの合図用。既定は何もしない）
5. **`internal/testenv` に `WaitFor` も置く。** 条件が成り立つまで 5ms 間隔で確かめ、10 秒で `t.Fatalf` する。別のパッケージ `testwait` にはしない（テストの部品を1つにまとめる）
6. **lint の 2分12秒の内訳は、PR で1回だけ `--verbose` を付けて確かめ、確かめたら外す**（Task 7）
7. **ruleset の必須チェックの変更は、PR の CI が緑になってから利用者の承認を得て行う**（Task 7）

---

### Task 1: ローカルのビルドキャッシュを worktree 間で共有し、Makefile を分ける

**Files:**
- Modify: `mise.toml`
- Modify: `Makefile`
- Modify: `CONTRIBUTING.md:15-23`

**Interfaces:**
- Produces: `make test`（race のみ）、`make cover`（race＋カバレッジ）、`make clean-cache`。Task 3 が `release-check` の windows の行を消す

- [ ] **Step 1: mise.toml に GOFLAGS を足す**

```toml
[tools]
"github:gotestyourself/gotestsum" = "1.13.0"

[env]
# Without -trimpath the build cache keys the packages of this module by their
# absolute directory, so every worktree compiles and caches them again.
# -trimpath makes the worktrees share one cache.
GOFLAGS = "-trimpath"
```

- [ ] **Step 2: Makefile の test を分け、clean-cache を足す**

`test` と `check` を次に替え、`cover` と `clean-cache` を足す。`.PHONY` にも足す。

```make
.PHONY: generate build fmt fmt-check lint tidy-check test cover clean-cache check release-check

# Run tests with the race detector. Coverage is make cover: a build with it is
# another variant in the build cache.
test:
	gotestsum --format testdox -- -race ./...

# Run tests with the race detector and coverage (mirrors the CI test step).
cover:
	gotestsum --format testdox -- -race -coverprofile=coverage.out -covermode=atomic ./...

# Empty the Go build and test caches. Go keeps entries for 5 days and has no
# size limit, so run this when the cache takes too much space.
clean-cache:
	go clean -cache -testcache
```

- [ ] **Step 3: CONTRIBUTING のコマンド一覧を直す**

```bash
make check          # everything CI checks: tidy, lint, fmt, test
make test           # tests with the race detector
make cover          # tests with the race detector and coverage
make fmt            # format with gofumpt + goimports
make release-check  # goreleaser config and cross-compiling
make build          # bin/hedgehug
make clean-cache    # empty the Go build cache when it grows too large
```

- [ ] **Step 4: GOFLAGS が効き、別の場所のビルドとキャッシュを共有することを確かめる**

```bash
go env GOFLAGS
```
Expected: `-trimpath`。空なら、このシェルで mise の環境が読まれていない。`eval "$(mise env -s bash)"` を先に実行してから確かめる（CONTRIBUTING の mise の説明は Task 3 で足す）。

キャッシュの共有は、リポジトリの複製で同じパッケージの export data のパスを比べる（テストは回さない）:

```bash
mkdir -p .tmp/copy
cp -r go.mod go.sum internal .tmp/copy/
go list -export -f '{{.Export}}' ./internal/model
go -C .tmp/copy list -export -f '{{.Export}}' ./internal/model
rm -rf .tmp/copy
```
Expected: 3行目と4行目が同じパス（-trimpath なしなら別のパスになる）。`.tmp/copy` は自分の go.mod を持つので、元のモジュールの `./...` には入らない

- [ ] **Step 5: Commit**

```bash
git add mise.toml Makefile CONTRIBUTING.md
git commit -m "chore: worktree の間でビルドキャッシュを共有し、make test からカバレッジを外す"
```

---

### Task 2: CI をテストの shard とまとめのジョブに組み直し、Windows を外す

**Files:**
- Modify: `.github/workflows/ci.yaml`（全体を下の内容にする）
- Modify: `CONTRIBUTING.md:50-52`

**Interfaces:**
- Produces: ジョブ `test-shard`（表示名 `test (<os>, <n>)`）、まとめのジョブ `test`（表示名 `test`、必須チェックにする名前）。各 shard の手順の要約（`$GITHUB_STEP_SUMMARY`）にパッケージごとの時間の表。Task 7 がこの表で shard を均す

- [ ] **Step 1: ci.yaml を書き換える**

`lint` は `skip-save-cache` と `--verbose`（Task 7 で外す）を足すだけ。`test-without-git` はそのまま（ワークフロー全体の `GOFLAGS` が効く）。

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}

# The same build everywhere as on a developer's machine (mise.toml): without
# -trimpath the build cache keys the packages by their directory.
env:
  GOFLAGS: -trimpath

jobs:
  lint:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false

      - name: Verify go.mod and go.sum are tidy
        run: |
          go mod tidy
          git diff --exit-code -- go.mod go.sum

      # The lint cache is saved on a push to main only, as the Go caches are,
      # so that a pull request cannot poison what main reads.
      - uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9.3.0
        with:
          version: v2.15.0
          skip-save-cache: ${{ github.event_name != 'push' }}
          args: --verbose

      - name: Format check
        run: golangci-lint fmt --diff ./...

  # hedgehug は Linux と macOS 向けに配る（Windows では WSL で Linux 版を使う）。
  # データの置き場所やファイルシステムが OS ごとに違うので、テストは2つの OS で回す。
  # 1つのジョブでは4コアで CPU 律速になるので、パッケージを3つの shard に分けて同時に回す。
  test-shard:
    name: test (${{ matrix.os }}, ${{ matrix.shard }})
    strategy:
      # 1つの shard の失敗で、ほかの結果を隠さない。
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest]
        shard: [1, 2, 3]
        # -race はテストを数倍遅くする。検出するデータ競合は OS によらないので、
        # ubuntu だけで付ける。カバレッジとテストの結果も ubuntu の分をまとめる。
        include:
          - os: ubuntu-latest
            race: -race
            report: true
    runs-on: ${{ matrix.os }}
    timeout-minutes: 15
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false

      # mise.toml に固定したツール（gotestsum）を入れる。Renovate の mise manager がバージョンを追う。
      - uses: jdx/mise-action@82a612c5ab8bcda7cc5db6b7e81a8e7584c3dc6e # v5.2.3

      # Go のビルドキャッシュとモジュールキャッシュを復元する。キャッシュポイズニングを避けるため、
      # 保存は main への push だけで行い（PR は復元のみ）、復元したモジュールは go mod verify で確かめる。
      # shard ごとにビルドするパッケージが違うので、キーに shard を入れる。
      - name: Locate Go caches
        id: go-env
        shell: bash
        run: |
          echo "gocache=$(go env GOCACHE)" >> "$GITHUB_OUTPUT"
          echo "gomodcache=$(go env GOMODCACHE)" >> "$GITHUB_OUTPUT"

      - uses: actions/cache/restore@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0
        id: go-cache
        with:
          path: |
            ${{ steps.go-env.outputs.gocache }}
            ${{ steps.go-env.outputs.gomodcache }}
          key: go-${{ runner.os }}-${{ runner.arch }}-test${{ matrix.race }}-shard${{ matrix.shard }}-${{ hashFiles('go.mod', 'go.sum') }}

      - name: Verify the module cache
        run: go mod verify

      # shard 1 と 2 は重いパッケージを1つずつ、shard 3 は残りすべて。
      # 新しいパッケージは何もしなくても shard 3 に入る。
      - name: Choose the packages
        id: pkgs
        shell: bash
        env:
          SHARD: ${{ matrix.shard }}
        run: |
          mod=$(go list -m)
          case "$SHARD" in
            1) pkgs="$mod/internal/ops" ;;
            2) pkgs="$mod/cmd/hedgehug" ;;
            3) pkgs=$(go list ./... | grep -vxF -e "$mod/internal/ops" -e "$mod/cmd/hedgehug" | tr '\n' ' ') ;;
          esac
          echo "list=$pkgs" >> "$GITHUB_OUTPUT"

      # 復元したキャッシュには main のテスト結果も入っているので、-count=1 で毎回テストを実行する。
      - name: Test
        shell: bash
        env:
          RACE: ${{ matrix.race }}
          REPORT: ${{ matrix.report }}
          SHARD: ${{ matrix.shard }}
          PKGS: ${{ steps.pkgs.outputs.list }}
        run: |
          cover=()
          if [ -n "$REPORT" ]; then cover=(-coverprofile="coverage-$SHARD.out" -covermode=atomic); fi
          # shellcheck disable=SC2086 # PKGS is a list of packages
          gotestsum --format testdox --junitfile "test-report-$SHARD.xml" --jsonfile "test-$SHARD.json" -- \
            ${RACE:+"$RACE"} -count=1 "${cover[@]}" $PKGS

      # パッケージごとの時間を手順の要約に出す。shard の分け方を均すときに使う。
      - name: Package times
        if: ${{ !cancelled() }}
        shell: bash
        env:
          SHARD: ${{ matrix.shard }}
        run: |
          {
            echo "| seconds | package |"
            echo "| ---: | --- |"
            jq -r 'select(.Test == null and (.Action == "pass" or .Action == "fail")) | "\(.Elapsed)\t\(.Package)"' "test-$SHARD.json" |
              sort -rn | awk -F'\t' '{ printf "| %s | %s |\n", $1, $2 }'
          } >> "$GITHUB_STEP_SUMMARY"

      - uses: actions/cache/save@55cc8345863c7cc4c66a329aec7e433d2d1c52a9 # v6.1.0
        if: github.event_name == 'push' && steps.go-cache.outputs.cache-hit != 'true'
        with:
          path: |
            ${{ steps.go-env.outputs.gocache }}
            ${{ steps.go-env.outputs.gomodcache }}
          key: ${{ steps.go-cache.outputs.cache-primary-key }}

      - uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        if: ${{ !cancelled() && matrix.report }}
        with:
          name: test-results-${{ matrix.shard }}
          path: |
            test-report-${{ matrix.shard }}.xml
            coverage-${{ matrix.shard }}.out
          retention-days: 1
          if-no-files-found: ignore

  # 必須チェックにするのはこのジョブだけ。shard の数や分け方を変えても名前は変わらない。
  # ubuntu の shard の結果とカバレッジを1つにまとめて投稿し、どれかの shard が
  # 失敗・取り消しになっていれば失敗する。
  test:
    needs: test-shard
    if: ${{ always() }}
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      contents: read
      actions: read
      checks: write
      pull-requests: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false

      - uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        if: ${{ !cancelled() }}
        with:
          pattern: test-results-*
          merge-multiple: true

      # go test のカバレッジのファイルは、先頭の mode の行のあとに行を並べただけなので、
      # 2つ目以降の mode の行を除いてつなげば1つになる。
      - name: Merge the coverage
        if: ${{ !cancelled() }}
        shell: bash
        run: |
          shopt -s nullglob
          files=(coverage-*.out)
          if [ ${#files[@]} -eq 0 ]; then exit 0; fi
          { echo "mode: atomic"; for f in "${files[@]}"; do tail -n +2 "$f"; done; } > coverage.out

      # 書き込めない GITHUB_TOKEN の PR（fork と Dependabot）では投稿が失敗して CI が赤くなるため、
      # 同じリポジトリの PR だけで投稿する。
      - name: Test Report
        uses: dorny/test-reporter@d1b5dff021aa968b288b02285a8bf47471c5928a # v3.2.0
        if: ${{ !cancelled() && github.event_name == 'pull_request' && github.event.pull_request.head.repo.full_name == github.repository && github.actor != 'dependabot[bot]' }}
        with:
          name: Test Report
          path: test-report-*.xml
          reporter: java-junit

      - name: Report coverage
        uses: k1LoW/octocov-action@a167dc0dee441b7ffc45e1b862ab55ec0d87f278 # v1.5.2
        if: ${{ !cancelled() && github.event_name == 'pull_request' && github.event.pull_request.head.repo.full_name == github.repository && github.actor != 'dependabot[bot]' }}

      - name: Every shard passed
        env:
          RESULT: ${{ needs.test-shard.result }}
        run: test "$RESULT" = success

  # Hedgehug は git なしで動く（親設計の「外部依存ゼロ」）。git を PATH から外して全テストを流し、
  # どこかが git の実行ファイルを呼ぶようになったらここで気付く。
  test-without-git:
    （いまの内容をそのまま残す）
```

`test-without-git` の中身は今の ci.yaml の該当部分を一字も変えずに残す（上の「（いまの内容をそのまま残す）」は、この plan の中での省略で、ファイルにこの文字を書かない）。

- [ ] **Step 2: ワークフローを検査する**

```bash
actionlint .github/workflows/ci.yaml
```
Expected: 出力なし。actionlint が無ければ `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/ci.yaml`（ビルドキャッシュを使うだけでテストは回らない）。

- [ ] **Step 3: CONTRIBUTING の CI の説明を直す**

```markdown
- CI runs on every pull request: `ci.yaml` (tidy, lint, format, and the tests on Linux
  and macOS, split into shards that run at once) and `security.yaml` (hidden-content
  scan, gitleaks, dependency scanning, zizmor, actionlint). All of them must pass.
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yaml CONTRIBUTING.md
git commit -m "ci: テストを OS と shard のマトリクスに分け、結果をまとめるジョブを置く。Windows を外す"
```

- [ ] **Step 5: ブランチの名前を変えて push し、draft PR を作る**

```bash
git branch -m ci-speedup
git push -u origin ci-speedup
gh pr create --draft --title "CI の短縮とローカルのゴミの削減" --body "spec: docs/superpowers/specs/2026-11-02-ci-speedup-refactor-design.md（説明は仕上げで書く）

🤖 Generated with [Claude Code](https://claude.com/claude-code)"
```

この時点では Windows のコードが残っているので、ubuntu と macOS のテストは今と同じく通るはず。ruleset が `test (windows-latest)` を必須にしているのでマージはできないが、CI は回る。CI の結果（各 shard の手順の要約にあるパッケージごとの時間、lint の `--verbose` の出力）は Task 7 で使う。CI を待たずに Task 3 に進む。

---

### Task 3: Windows のコードと配布をやめる

**Files:**
- Delete: `internal/store/lock_windows.go`、`internal/sync/spawn_windows.go`、`internal/sync/childlog_windows.go`、`internal/atomicfile/atomicfile_windows.go`
- Modify: `internal/atomicfile/atomicfile.go`、`internal/atomicfile/atomicfile_test.go`
- Modify: `internal/sync/childlog_unix.go`、`internal/sync/background.go:231,273`
- Modify: `internal/config/paths.go:52-92`、`internal/config/paths_test.go`
- Modify: `internal/gitrepo/transport.go:59-68`、`internal/gitrepo/conn.go:80-82`、`internal/gitrepo/remote_test.go:220-232`、`internal/gitrepo/conn_test.go:203`
- Modify: `cmd/hedgehug/editor.go:12-18`、`cmd/hedgehug/main.go:49-55`、`cmd/hedgehug/main_test.go:15`（コメント）
- Modify: `tools/devenv/main.go:306`
- Modify: テストの `runtime.GOOS == "windows"` の分岐と `USERPROFILE`（下の一覧）
- Modify: `.goreleaser.yaml`、`Makefile`（release-check）、`README.md`、`README.ja.md`、`CONTRIBUTING.md`、`.github/ISSUE_TEMPLATE/bug_report.yaml`、`.github/ISSUE_TEMPLATE/feature_request.yaml`

**Interfaces:**
- Produces: なし（Windows では `go build` が通らなくなる）

- [ ] **Step 1: Windows 専用のファイルを消す**

```bash
git rm internal/store/lock_windows.go internal/sync/spawn_windows.go internal/sync/childlog_windows.go internal/atomicfile/atomicfile_windows.go
```

対になる `*_unix.go` の名前とビルド制約（`//go:build unix` と `//go:build !windows`）はそのまま残す。

- [ ] **Step 2: Windows でだけ使っていた関数を消す**

- `internal/atomicfile/atomicfile.go`: `renameRetries`・`renameInterval`・`retryRename` を消す（`time` の import も不要になれば消す）。パッケージのコメントの「and reads them without stopping such a replacement」は、Windows の FILE_SHARE_DELETE の説明なので「Package atomicfile replaces files whole, so that a reader never sees a part of a write.」にする
- `internal/atomicfile/atomicfile_test.go`: `TestRetryRename` を消す
- `internal/sync/childlog_unix.go`: `redirectStderr(to *os.File, _ bool)` を `redirectStderr(to *os.File)` にし、コメントの「closeOld is for Windows.」を消す
- `internal/sync/background.go:231` の `redirectStderr(nul, true)` を `redirectStderr(nul)`、`:273` の `redirectStderr(f, false)` を `redirectStderr(f)` にする

- [ ] **Step 3: 本番コードの Windows の分岐を消す**

`internal/config/paths.go`:

```go
// home is the user's home directory, $HOME.
func home(getenv func(string) string) (string, error) {
	h := abs(getenv("HOME"))
	if h == "" {
		return "", errors.New("the home directory is not known: $HOME is not set to an absolute path")
	}
	return h, nil
}

func defaultConfigDir(home string) string {
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Application Support")
	}
	return filepath.Join(home, ".config")
}

func defaultDataDir(home string) string {
	return filepath.Join(home, ".local", "share")
}
```

呼び出し側（`Resolve` の中の `defaultConfigDir(getenv, h)` と `defaultDataDir(getenv, h)`）も引数を合わせる。

`internal/gitrepo/transport.go`: `driveRE` と `localPath` を消し、`fileLoader.Load` は `p := filepath.FromSlash(ep.Path)` にする。`regexp` の import が不要になれば消す。

`internal/gitrepo/conn.go:76-84`:

```go
	if p, ok := strings.CutPrefix(u, "file://"); ok {
		if p == "" {
			return "", false
		}
		return filepath.FromSlash(p), true
	}
	if filepath.IsAbs(u) {
		return u, true
	}
	return "", false
```

（Unix では `filepath.IsAbs` は `/` で始まるかどうかなので、`strings.HasPrefix(u, "/")` は同じ判定になり不要。）

`cmd/hedgehug/editor.go`:

```go
// defaultEditor is the editor when neither $VISUAL nor $EDITOR is set.
const defaultEditor = "vi"
```

`runtime` の import が不要になれば消す。

`cmd/hedgehug/main.go:49-55`: `init` の `cobra.MousetrapHelpText = ""` とそのコメントを消す（Windows でだけ効く設定）。`cmd/hedgehug/main_test.go:15-22` の `TestNoExplorerCheck`（この設定を確かめるだけのテスト）をコメントごと消し、`cobra` の import が不要になれば消す。

`tools/devenv/main.go:306`: `case "HEDGEHUG_HOME", "HOME", "USERPROFILE":` を `case "HEDGEHUG_HOME", "HOME":` にする。

- [ ] **Step 4: テストの Windows の分岐を消す**

次をすべて直す。`runtime` などの import が不要になれば消す。

| 場所 | 直し方 |
| --- | --- |
| `internal/config/paths_test.go` の `"Windows defaults"` の行 | 行を消す。`xc` が使われなくなれば消す |
| `internal/config/paths_test.go:94`、`internal/server/helpers_test.go:93`、`internal/ops/helpers_test.go:75,206`、`internal/ops/mount_ops_test.go:417` の `"USERPROFILE": home` | 消す |
| `internal/gitrepo/remote_test.go` の `TestLocalPath` | テストごと消す（`localPath` を消したため） |
| `internal/gitrepo/conn_test.go:203` の `` `C:\x\y.git`: true, "file:///C:/x/y.git": true, `` | 消す |
| `internal/gitrepo/lock_test.go:223-225`、`internal/gitrepo/gitrepotest/lock.go:15-18`、`internal/sync/rebuild_test.go:394`、`internal/ops/discover_test.go:290`、`internal/ops/movestore_unfinished_test.go:17`、`cmd/hedgehug/mv_store_resumed_test.go:15`、`cmd/hedgehug/promised_test.go:17-18` | `runtime.GOOS == "windows" \|\|` を消して root の条件だけ残す。Windows だけで飛ばしていたものは `if` ごと消す。コメントの「on Windows, and」も消す |
| `internal/server/events_test.go:153-154`、`internal/ops/maintain_test.go:71-72`、`cmd/hedgehug/e2e_test.go:36-37`、`cmd/hedgehug/helpers_test.go:231-232,308-309` | `.exe` を足す `if` を消す |
| `internal/doctor/doctor_test.go:679-682` | `if runtime.GOOS == "windows" { delete(… "views/trail .yaml" …) }` の4行を消す（`caseSensitive` の分岐は macOS のために残す。`:734` のコメントの「Windows and macOS do not」は「macOS does not」にする） |
| `internal/view/list_test.go:31-46` | コメント「Windows cannot create CON.yaml.」と `con` を消し、`writeView(t, dir, "CON.yaml", …)` を無条件に呼び、`want` は初めから `[]string{"CON", "a", "b", "broken"}` にする |
| `cmd/hedgehug/golden_test.go:34-40` | `escaped` とその置き換え、`tmpPathRE` で `\` を `/` にする処理を消し、`s = strings.ReplaceAll(s, tmp, "<TMP>")` だけにする。`tmpPathRE` がほかで使われなければ消す |
| `cmd/hedgehug/ui_test.go:140` | `runtime.GOOS != "windows" &&` を消す |
| `cmd/hedgehug/helpers_test.go:64,413,417`、`cmd/hedgehug/e2e_test.go:70,87`、`cmd/hedgehug/e2e_3a_test.go:113,182` の `USERPROFILE` | `USERPROFILE` の部分だけ消す |
| `internal/gitrepo/storage_test.go:44` のコメント | 直さない（細部 3） |

最後に取りこぼしを探す:

```bash
grep -rnE --include='*.go' 'GOOS *[!=]= *"windows"|USERPROFILE|APPDATA|\.exe"' internal cmd tools
```
Expected: 出力なし

- [ ] **Step 5: 配布と文書を直す**

`.goreleaser.yaml`: 2つの `goos` から `- windows` を消し、`archives` の `format_overrides`（windows の zip）を消す。

`Makefile` の `release-check` から `GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/hedgehug` を消す。

`README.md` の説明の段落のあとに足す:

```markdown
It runs on Linux and macOS. On Windows, use it in WSL.
```

`README.ja.md` の説明の段落のあとに足す:

```markdown
Linux と macOS で動きます。Windows では WSL の中で使ってください。
```

`CONTRIBUTING.md`:
- 7行目: `hedgehug is a single Go binary for Linux and macOS (on Windows, it runs in WSL). You need:`
- 必要なものの一覧に足す: `- [mise](https://mise.jdx.dev/) (optional) — it installs gotestsum and sets `GOFLAGS=-trimpath` from `mise.toml`, so that worktrees share one build cache`
- 86行目: `the binaries for Linux and macOS (amd64 and arm64), stamps the version that`
- 「Getting started」の最後に足す:

```markdown
On Windows, develop in WSL. The disk of WSL (ext4.vhdx) grows with what is written and does
not shrink when files are deleted; run `wsl --manage <distro> --set-sparse true` once on the
Windows side so that the space of deleted files goes back to the drive.
```

`.github/ISSUE_TEMPLATE/bug_report.yaml:30`: OS の選択肢から `- Windows` を消し、`- Windows (WSL)` を足す（WSL で使う人が OS を選べるように）。

`.github/ISSUE_TEMPLATE/feature_request.yaml:9-10`: `It ships for Linux and macOS (on Windows, it runs in WSL), so a` / `proposal has to work on both.` にする。

- [ ] **Step 6: ビルドと検査**

```bash
go build ./... && go vet ./... && GOOS=darwin GOARCH=arm64 go vet ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
go mod tidy && git diff --exit-code -- go.mod go.sum
goreleaser check
```
Expected: すべて成功。go.mod・go.sum の差分なし（`go mod tidy` は全 OS を見るので wincred などは残る）。goreleaser が無ければ `goreleaser check` は飛ばし、PR の説明に書く。

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: Windows 向けのコードと配布をやめる（Windows では WSL を使う）"
```

---

### Task 4: `internal/testenv` で一時ディレクトリを1か所にまとめ、残りを掃除する

**Files:**
- Create: `internal/testenv/testenv.go`
- Create: `internal/testenv/testenv_test.go`
- Create: `testmain_test.go`（下の 21 パッケージ）
- Modify: `cmd/hedgehug/helpers_test.go:215-247`（TestMain）
- Modify: `internal/server/main_test.go`

**Interfaces:**
- Produces:
  - `func Main(m *testing.M)` — Setup してテストを回し、後始末して `os.Exit`
  - `func Setup() (cleanup func(), err error)` — `$TMPDIR/hedgehug-test-<pid>` を作って `TMPDIR` を向け、残りを掃除する
  - `func Sweep(base string)` — base の中の、プロセスがもう無い `hedgehug-test-<pid>` を消す
  - `func WaitFor(t testing.TB, what string, cond func() bool)` — Task 6 が使う

- [ ] **Step 1: 失敗するテストを書く**

`internal/testenv/testenv_test.go`:

```go
package testenv

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMain is also the child of TestSweepRemovesTheDirectoryOfAGoneProcess:
// with TESTENV_CHILD set it makes its directory, says where, and waits to be
// killed.
func TestMain(m *testing.M) {
	if os.Getenv("TESTENV_CHILD") == "1" {
		if _, err := Setup(); err != nil {
			os.Exit(2)
		}
		dir := os.Getenv("TMPDIR")
		// A read-only directory with a file in it, as some tests leave: the
		// file cannot be removed until the directory is writable again.
		ro := filepath.Join(dir, "ro")
		if os.Mkdir(ro, 0o700) != nil || os.WriteFile(filepath.Join(ro, "f"), nil, 0o600) != nil || os.Chmod(ro, 0o500) != nil {
			os.Exit(2)
		}
		os.Stdout.WriteString(dir + "\n")
		time.Sleep(time.Hour)
	}
	Main(m)
}

func TestSetupPointsTMPDIRAtTheProcess(t *testing.T) {
	t.Parallel()
	want := prefix + strconv.Itoa(os.Getpid())
	if got := filepath.Base(os.TempDir()); got != want {
		t.Fatalf("TMPDIR is %s, want a directory named %s", os.TempDir(), want)
	}
	if !strings.HasPrefix(t.TempDir(), os.TempDir()) {
		t.Errorf("t.TempDir is not in %s", os.TempDir())
	}
}

func TestSweepRemovesTheDirectoryOfAGoneProcess(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^$") //nolint:gosec // this test binary
	cmd.Env = append(os.Environ(), "TESTENV_CHILD=1", "TMPDIR="+base)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	child := strings.TrimSpace(line)
	if filepath.Dir(child) != base {
		t.Fatalf("the child made %s, not in %s", child, base)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	alive := filepath.Join(base, prefix+strconv.Itoa(os.Getpid()))
	other := filepath.Join(base, prefix+"x")
	for _, d := range []string{alive, other} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	Sweep(base)
	if _, err := os.Stat(child); !os.IsNotExist(err) {
		t.Errorf("the directory of the killed child is still there: %v", err)
	}
	for _, d := range []string{alive, other} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s was removed: %v", d, err)
		}
	}
}

func TestWaitFor(t *testing.T) {
	t.Parallel()
	n := 0
	WaitFor(t, "three looks", func() bool { n++; return n == 3 })
	if n != 3 {
		t.Errorf("cond was called %d times", n)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

```bash
mkdir -p .tmp && TMPDIR="$PWD/.tmp" go test -count=1 ./internal/testenv/
```
Expected: コンパイルエラー（`Setup`・`Main`・`Sweep`・`WaitFor`・`prefix` が無い）

- [ ] **Step 3: 実装する**

`internal/testenv/testenv.go`:

```go
// Package testenv is for the tests of the other packages. Main gives the
// tests of a package a temporary directory of their own process and removes
// the directories of test processes that are gone, so that a run stopped half
// way leaves nothing behind once the next one has run. WaitFor waits for a
// condition.
package testenv

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// prefix starts the name of the directory of a test process; its pid follows.
const prefix = "hedgehug-test-"

// Main runs the tests of m with Setup and exits with their code.
func Main(m *testing.M) {
	cleanup, err := Setup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// Setup removes the directories of test processes that are gone from the
// temporary directory, makes hedgehug-test-<pid> there and points TMPDIR at
// it, so that t.TempDir and os.MkdirTemp put everything in it, children of
// the test included. cleanup removes it and points TMPDIR back.
func Setup() (cleanup func(), err error) {
	base := os.TempDir()
	Sweep(base)
	dir := filepath.Join(base, prefix+strconv.Itoa(os.Getpid()))
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	old, had := os.LookupEnv("TMPDIR")
	if err := os.Setenv("TMPDIR", dir); err != nil {
		return nil, err
	}
	return func() {
		if had {
			_ = os.Setenv("TMPDIR", old)
		} else {
			_ = os.Unsetenv("TMPDIR")
		}
		removeAll(dir)
	}, nil
}

// Sweep removes the hedgehug-test-<pid> directories in base whose process is
// gone. A process it may not signal is taken as alive.
func Sweep(base string) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		rest, ok := strings.CutPrefix(e.Name(), prefix)
		if !ok || !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(rest)
		if err != nil || pid <= 0 {
			continue
		}
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			removeAll(filepath.Join(base, e.Name()))
		}
	}
}

// removeAll removes dir, making its directories writable first when a test
// left one read-only.
func removeAll(dir string) {
	if os.RemoveAll(dir) == nil {
		return
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		// A directory that could not be read comes with err set; it is the
		// one to make writable.
		if d != nil && d.IsDir() {
			_ = os.Chmod(p, 0o700) //nolint:gosec // G302: a directory of the tests, about to be removed
		}
		return nil
	})
	_ = os.RemoveAll(dir)
}

// WaitFor calls cond every 5ms until it is true, and fails the test when it is
// not within 10 seconds. what names what is waited for.
func WaitFor(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
```

- [ ] **Step 4: テストが通ることを確かめる**

```bash
TMPDIR="$PWD/.tmp" go test -count=1 ./internal/testenv/
ls -A .tmp
rm -rf .tmp
```
Expected: PASS。`ls -A .tmp` は何も出さない（テストのディレクトリも、止めた子のディレクトリも残っていない）。テストはこの1回だけ回す。失敗したら、回し直す前にコードを読んで原因を確かめる

- [ ] **Step 5: 各パッケージの TestMain から呼ぶ**

次の 21 パッケージに `testmain_test.go` を作る（`cmd/fakegh` と `tools/devenv` にはすでに `main_test.go` があるので、全パッケージ `testmain_test.go` にそろえる）。`package` はそのディレクトリの既存のテストファイルと同じ名前にする（`cmd/fakegh` と `tools/devenv` は `main`）。すでに TestMain がある場合は作らずに Step 6 のとおり直す。

`cmd/fakegh`、`internal/atomicfile`、`internal/auth`、`internal/config`、`internal/context`、`internal/doctor`、`internal/fakegh`、`internal/github`、`internal/gitrepo`、`internal/inbox`、`internal/index`、`internal/merge`、`internal/model`、`internal/mount`、`internal/ops`、`internal/query`、`internal/schema`、`internal/store`、`internal/sync`、`internal/view`、`tools/devenv`

```go
package store

import (
	"testing"

	"github.com/kukv/hedgehug/internal/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }
```

作る前に、各パッケージに TestMain がないことを確かめる:

```bash
grep -rln --include='*_test.go' 'func TestMain' internal cmd tools
```
Expected: `internal/server/main_test.go`、`cmd/hedgehug/helpers_test.go`、`internal/testenv/testenv_test.go` だけ

- [ ] **Step 6: 既存の TestMain を直す**

`internal/server/main_test.go` を次にする（hedgehug を作ったディレクトリは `TMPDIR` の下にできるので、後始末は testenv が行う）:

```go
package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }
```

`hhBin` を作る処理（`events_test.go` の `os.MkdirTemp("", "hh-server-test")`）はそのままでよい。

`cmd/hedgehug/helpers_test.go` の TestMain は、エディタ役の分岐のあと、ほかの準備の前に Setup を呼ぶ。`pathDir`・`builtDir`・`devenvDir` は `TMPDIR` の下にできるので、個別の `os.RemoveAll` を消して `cleanup()` に任せる:

```go
func TestMain(m *testing.M) {
	if script := os.Getenv("HH_TEST_EDITOR_SCRIPT"); script != "" {
		os.Exit(testEditor(script, os.Args[len(os.Args)-1]))
	}
	// Everything the tests make, the binaries buildHedgehug and buildDevenv
	// make included, goes in a directory of this process that cleanup removes.
	cleanup, err := testenv.Setup()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		os.Exit(1)
	}
	// No test reaches the keychain of the machine.
	keyring.MockInit()
	// （GORACE の設定と PATH の設定は今のまま）
	code := m.Run()
	cleanup()
	os.Exit(code)
}
```

`buildDevenv` のコメントの「TestMain removes it」は「the directory of testenv holds it」に直す。

- [ ] **Step 7: ビルドと検査**

```bash
go vet ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
```
Expected: 成功

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "test: テストの一時ディレクトリをプロセスごとの1か所にまとめ、止めた実行の残りを次の実行で消す"
```

---

### Task 5: 直列のテストに t.Parallel を付ける

**Files:** 下の一覧のテストファイルと `internal/query/helpers_test.go`

**Interfaces:**
- Consumes: なし

規則:
- 付けるのはトップレベルのテスト関数だけ。関数の最初の行（`t.Helper()` があればその前ではなくテスト関数の先頭）に `t.Parallel()` を書く。サブテストには付けない
- 並列のテストは、同じパッケージの直列のテストがすべて終わってから走る。直列のテストがパッケージ変数を差し替えて元に戻すなら、並列のテストがその変数を読んでも競合しない
- 直列のまま残すテストには、関数の前に `// 直列: <理由>` を1行付ける（下の表の「コメント」）

- [ ] **Step 1: query のヘルパーのグローバルな map をやめる**

`internal/query/helpers_test.go`: `storeDirs` を消し、`newStore` の `storeDirs[s] = dir` を消し、`writeRaw` の `storeDirs[s]` を `s.Dir()` にする（`store.Store.Dir()` は `internal/store/local.go:130` にある）。

- [ ] **Step 2: 全部付けるファイル**

ファイルの中のトップレベルの `func Test…(t *testing.T)` すべてに付ける。

- `cmd/hedgehug/scenario_test.go`、`cmd/fakegh/main_test.go`、`internal/fakegh/server_test.go`
- `internal/github/` の `client_test.go`・`edges_test.go`・`safety_test.go`・`origin_test.go`・`insecure_test.go`・`insecure_device_test.go`・`neterr_test.go`・`timeout_test.go`
- `internal/auth/secret_test.go`
- `internal/config/` の `config_test.go`・`hosts_test.go`・`common_rules_test.go`・`loopback_test.go`
- `internal/query/` の `eval_test.go`・`sort_test.go`・`parse_test.go`・`sortkey_test.go`・`fixes_test.go`
- `internal/merge/` の `yaml_test.go`・`kinds_test.go`・`changes_test.go`・`text_test.go`・`repair_sprint_test.go`
- `internal/model/` の `rank_test.go`・`numbers_test.go`・`taskid_test.go`・`storemeta_test.go`・`duration_test.go`・`workspace_test.go`・`comment_test.go`・`node_test.go`・`date_test.go`・`path_test.go`・`ulid_test.go`・`template_test.go`・`sprint_test.go`・`frontmatter_test.go`
- `internal/schema/` の `migrate_test.go`・`check_test.go`・`effective_test.go`・`validate_test.go`・`schema_test.go`
- `internal/view/` の `view_test.go`・`list_test.go`・`name_test.go`

- [ ] **Step 3: 一部だけ付けるファイル**

| ファイル | 付ける | 直列のまま（コメント） |
| --- | --- | --- |
| `internal/auth/source_test.go` | TestTokenGh, TestTokenEnv, TestTokenKeychain, TestTokenNone, TestDeviceTokenRoundTrip | TestClientID（`// 直列: パッケージ変数 builtinClientID の差し替え`） |
| `internal/auth/minor_test.go` | TestDeleteNotFoundIsNoError, TestBrokenDeviceTokenJSON, TestTokenUnknownKind, TestDeviceNotLoggedInSaysWhatToRun, TestWrapKeyring, TestOptionsKeychain | TestWithLock（`// 直列: パッケージ変数 lockWait の差し替え`） |
| `internal/auth/device_test.go` | TestDeviceLogin, TestDeviceLoginWaits, TestDeviceLoginExpires, TestDeviceLoginSlowDown, TestDeviceLoginErrors, TestDeviceRefresh, TestDeviceTokenEdges, TestDeviceRefreshKeepFails, TestDeviceNoExpiry, TestDeviceRefreshNeedsHost, TestDeviceRefreshLockFails | TestDeviceRefreshLockBusy（`// 直列: パッケージ変数 lockWait の差し替え`） |
| `internal/config/paths_test.go` | TestResolveDataDir | TestResolve, TestResolveNeedsHome（`// 直列: パッケージ変数 goos の差し替え`） |
| `internal/config/author_test.go` | TestCommitAuthorFromAccount, TestCommitAuthorWithoutAccountReadsGitConfig | TestAuthor（`// 直列: パッケージ変数 goos の差し替え`）、TestAuthorInclude（`// 直列: パッケージ変数 goos の差し替えと t.Chdir`） |
| `internal/atomicfile/atomicfile_test.go` | TestWriteFile, TestReadFileNotExist | なし（TestRetryRename は Task 3 で消した） |

- [ ] **Step 4: 付けないファイルにコメントを足す**

- `cmd/hedgehug/auth_test.go`: ファイルの最初のテストの前に `// 直列: このファイルのテストはすべて keyring.MockInit で go-keyring のモックを作り直す` を1行足す
- `internal/store/prefix_test.go` の TestPrefixUses: `// 直列: パッケージ変数 openStore の差し替え`
- `internal/sync/background_stderr_test.go`: 既にコメントがあるので変えない

- [ ] **Step 5: 取りこぼしを確かめる**

```bash
grep -rLE --include='*_test.go' 't\.Parallel\(\)' internal cmd tools
```
Expected: 次だけが残る（ヘルパー・Fuzz・TestMain だけのファイルと、付けないと決めたファイル）。`*/helpers_test.go`、`*/testmain_test.go`、`internal/server/main_test.go`、`cmd/hedgehug/golden_test.go`、`internal/merge/fuzz_test.go`、`internal/model/fuzz_test.go`、`internal/sync/ops_test.go`、`cmd/hedgehug/auth_test.go`、`internal/store/prefix_test.go`、`internal/sync/background_stderr_test.go`

- [ ] **Step 6: ビルドと検査**

```bash
go vet ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "test: グローバルな状態を触らないテストを並列にする"
```

---

### Task 6: テストの固定の待ちを、条件や合図を待つ形にする

**Files:**
- Modify: `internal/store/lock.go:47`、`internal/store/tx.go:524`、`internal/gitrepo/lock.go:17`、`internal/server/poll.go`（本番コード。細部 4）
- Modify: 下の各テスト

**Interfaces:**
- Consumes: `testenv.WaitFor(t testing.TB, what string, cond func() bool)`（Task 4）
- Produces: `store.onLockWait func()`（パッケージ内）、`server.Poller.interval time.Duration`

- [ ] **Step 1: 本番コードの小さな変更**

`internal/store/lock.go` の `acquire` の最後:

```go
		time.Sleep(min(lockPoll, time.Until(deadline)))
```

`internal/store/tx.go` の `lockStore`:

```go
// onLockWait is called when lockStore waits for a lock held elsewhere; tests
// replace it to know that the wait began.
var onLockWait = func() {}

func (s *Store) lockStore() (*fileLock, error) {
	deadline := time.Now().Add(s.lockWait)
	for {
		s.mu.Lock()
		l, err := acquire(s.path(lockPath), 0)
		if err == nil {
			return l, nil
		}
		s.mu.Unlock()
		if !errors.Is(err, ErrLocked) || time.Now().After(deadline) {
			return nil, err
		}
		onLockWait()
		time.Sleep(min(lockPoll, time.Until(deadline)))
	}
}
```

`internal/gitrepo/lock.go:17`: `const indexLockPoll = 100 * time.Millisecond` を `var indexLockPoll = 100 * time.Millisecond` にする。

`internal/server/poll.go`: `Poller` に `interval time.Duration // how long Run waits between looks; pollInterval` を足し、`internal/server/poll.go:80` の `&Poller{…}` に `interval: pollInterval` を足し、`Run` の `time.NewTimer(pollInterval)` を `time.NewTimer(p.interval)` にする。

- [ ] **Step 2: パッケージのテストで待ちの間隔を縮める**

Task 4 で作った `testmain_test.go` の TestMain で、テストが始まる前に1回だけ設定する（並列のテストより前なので競合しない）。3つとも import に `"time"` を足す（`"testing"` と testenv はすでにある）:

```go
// internal/store/testmain_test.go
func TestMain(m *testing.M) {
	// Waits for a lock look again every 5ms, so that a test of a lock that
	// cannot be taken ends soon after its wait.
	lockPoll = 5 * time.Millisecond
	testenv.Main(m)
}
```

```go
// internal/gitrepo/testmain_test.go
func TestMain(m *testing.M) {
	indexLockPoll = 5 * time.Millisecond
	testenv.Main(m)
}
```

```go
// internal/sync/testmain_test.go
func TestMain(m *testing.M) {
	// A rejected push is tried again at once (it waits 500ms, doubling).
	retryWait = time.Millisecond
	testenv.Main(m)
}
```

`internal/sync/round_test.go:203` の `replace(t, &retryWait, time.Millisecond)` は同じ値になるので消す。

- [ ] **Step 3: server の poll のテスト**

`internal/server/poll_test.go` に足す:

```go
// settle makes the poller look twice without publishing: the first look sets
// the HEADs and the stamps, the second lets the inbox mark settle. pollMu
// keeps it from overlapping Run.
func (p *Poller) settle() {
	p.poll(false)
	p.poll(false)
}
```

- `pollServer` と `poll_moved_test.go` の準備で、`go ts.srv.poller.Run(ctx)` の前に `ts.srv.poller.interval = 10 * time.Millisecond` を入れる
- `poll_test.go:64,82,102,118,134,188` と `poll_moved_test.go:36` の `time.Sleep(2 * pollInterval)` を `ts.srv.poller.settle()` にする
- 「最初の見回しは何も送らない」を確かめるテスト（`time.Sleep(2 * pollInterval)` の直後に `select { … default: }` があるもの、`:159`）は、`ts.srv.poller.poll(true)` を2回呼んでから `select` で何も来ていないことを確かめる（hub は購読者のバッファに同期で積むので、2回呼んだあとに空なら送っていない）

- [ ] **Step 4: server の loop のテスト**

`internal/server/clock_test.go`: `eventually` の本体を `testenv.WaitFor(t, what, cond)` を呼ぶだけにする。`fakeClock` に `After` の呼び出し回数を数える `afters int` を足し、`After` の中で `c.afters++`、取り出す関数を足す:

```go
// afterCalls is how many times After was called: the loop calls it each time
// it parks, so a new call means the loop looked again.
func (c *fakeClock) afterCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.afters
}
```

`internal/server/loop_test.go`:
- `:51`・`:78`・`:170` の `clk.Advance(…); time.Sleep(100 * time.Millisecond)` を次にする（そのあとの確かめはそのまま）:

```go
	n := clk.afterCalls()
	clk.Advance(2 * time.Minute) // （それぞれの元の値）
	ts.srv.loop.Wake()
	eventually(t, "the loop to look again", func() bool { return clk.afterCalls() > n })
	eventually(t, "the loop to park", ts.srv.loop.parked)
```

- `:192` の `time.Sleep(200 * time.Millisecond)` は、sync が走っている最中なので `parked` は使えない。`afterCalls` の増加を待ったあと、`ts.srv.loop.mu` の下で `active` を読み、`active != 1 || calls.Load() != 1` なら失敗にする
- `:148` の `time.Sleep(100 * time.Millisecond)` は 30ms にする（Drain が待ち始めたことを知る合図がないため残す）
- `TestDrainGivesUp` の `Drain(200 * time.Millisecond)` は `Drain(20 * time.Millisecond)` にする（上限の `2*time.Second` の確かめはそのまま）

（`Loop` の `parked()`・`active`・`mu` は `internal/server/loop.go:32-37,371` にある。`Run` は `step()` が返ってから `l.clock.After(wait)` を呼ぶ（`loop.go:65-66`）ので、`After` の呼び出しが増えたら、その周の判断は終わっている。）

- [ ] **Step 5: store・gitrepo・auth・inbox・sync のテスト**

| 場所 | 直し方 |
| --- | --- |
| `internal/store/lock_test.go:22,71,100` の `100*time.Millisecond` | `20*time.Millisecond` |
| `internal/store/tx_test.go:243` の `s.lockWait = 100 * time.Millisecond` | `20 * time.Millisecond` |
| `internal/store/move_test.go:557` の `time.After(300 * time.Millisecond)` | `time.After(60 * time.Millisecond)` |
| `internal/store/move_test.go:582` の `100 * time.Millisecond` | `20 * time.Millisecond` |
| `internal/store/move_test.go:597` の `50 * time.Millisecond` | `10 * time.Millisecond` |
| `internal/store/merge_locked_test.go:31` の `time.Sleep(200 * time.Millisecond)` | 下のコード。`onLockWait` を差し替えるので、このテストは直列にして `// 直列: パッケージ変数 onLockWait の差し替え` を付ける |
| `internal/gitrepo/lock_test.go:53` の `time.Sleep(150 * time.Millisecond)` | `time.Sleep(20 * time.Millisecond)` |
| `internal/gitrepo` の `indexLockWait = 50 * time.Millisecond`（TestCommitWaitsForIndexLock、TestFastForwardWaitsForIndexLock） | `5 * time.Millisecond` |
| `internal/auth/minor_test.go:96-98`、`internal/auth/device_test.go:293-295` の `lockWait = 50 * time.Millisecond` | `5 * time.Millisecond` |
| `internal/inbox/collect_test.go:696` の `time.Sleep(20 * time.Millisecond)` | 下のコード |
| `internal/sync/forward_test.go:170` の `time.Sleep(1100 * time.Millisecond)` | 下のコード |
| `internal/sync` の TestBackgroundWaitsAndRuns の push_interval 50ms | 10ms（確かめる値も同じに） |

`merge_locked_test.go`:

```go
	waiting := make(chan struct{}, 1)
	onLockWait = func() {
		select {
		case waiting <- struct{}{}:
		default:
		}
	}
	defer func() { onLockWait = func() {} }()
	// （CommitMerge を goroutine で始める、今のコード）
	<-waiting // CommitMerge is waiting for the file lock, without s.mu
```

`collect_test.go:696` の前後（Catch が何も書かないことを、mtime を過去にして確かめる）:

```go
	past := time.Now().Add(-time.Hour)
	for _, name := range []string{itemsFile, readFile, markFile} {
		_ = os.Chtimes(b.s.LocalPath(name), past, past) // a file that is not there is left so
	}
	before := box.Stamp()
```

（`itemsFile`・`readFile`・`markFile` は `internal/inbox/inbox.go:72-74`、`Stamp` は `inbox.go:90` にあり、`Stamp` は `b.s.LocalPath(name)` の大きさと mtime を見る。）

`forward_test.go:170` のループ（2つ目の move-out を1秒後の時刻にする）:

```go
	base := time.Now()
	for i := range dest {
		at := base.Add(time.Duration(2*i) * time.Second)
		c.stores[i].SetNow(func() time.Time { return at })
		// （今の Update の呼び出し）
	}
```

`time.Sleep` と `if i > 0` を消す。実装の前に、このテストの sync が実時間と比べる所（`opt.Now` など）がないことを確かめる。比べる所があれば、sleep を残してこの行は直さず、PR の説明に理由を書く。

- [ ] **Step 6: cmd/hedgehug のテスト**

| 場所 | 直し方 |
| --- | --- |
| `background_test.go:16-40` の `waitChildDone` のループ | 本体を `testenv.WaitFor(t, "the background child to finish", func() bool { …今の条件… })` にする |
| `auth_test.go:262,287`、`ui_test.go:87,160` の `time.Sleep(10 * time.Millisecond)` のループ | それぞれ `testenv.WaitFor` にする |
| `ui_test.go:173` の openURL の中の `time.Sleep(50 * time.Millisecond)` | `10 * time.Millisecond` |
| `e2e_test.go:117`、`e2e_3c_test.go:162` の `time.Sleep(1100 * time.Millisecond)` | `sleepToNextSecond()` |
| `comment_test.go:296` の `time.Sleep(300 * time.Millisecond)` | 変えない（cmd から store の待ちを知る方法がない） |

`e2e_test.go` に足す:

```go
// sleepToNextSecond waits past the start of the next second. Commit times are
// in whole seconds, so what is written after it is in a later second than
// anything written before.
func sleepToNextSecond() {
	now := time.Now()
	time.Sleep(now.Truncate(time.Second).Add(time.Second).Sub(now) + 5*time.Millisecond)
}
```

- [ ] **Step 7: 残りを確かめる**

```bash
grep -rnE --include='*_test.go' 'time\.Sleep\(' internal cmd tools
```
Expected: 残すと決めたものだけ（`store/lock_test.go:50` の1時間、`server/loop_test.go` の 30ms、`gitrepo/lock_test.go` の 20ms、`cmd/hedgehug/ui_test.go` の 10ms、`cmd/hedgehug/comment_test.go` の 300ms、`testenv` の中）。ほかに出たら、この Task の表に照らして直す

- [ ] **Step 8: ビルドと検査**

```bash
go vet ./... && golangci-lint run ./... && golangci-lint fmt --diff ./...
```

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "test: 固定の待ちを、条件と合図を待つ形にし、待ちの間隔をテストで縮める"
```

---

### Task 7: CI で確かめて仕上げる

この Task は SDD の実装者ではなく、コーディネーター（このセッション）が行う。

- [ ] **Step 1: push して CI を見る**

```bash
git push
gh pr checks --watch
```

失敗したら、ログを読んで原因を直す（`superpowers:systematic-debugging`）。ローカルでテストを回して確かめない。直したら push して CI で確かめる。

- [ ] **Step 2: shard を均す**

各 `test (ubuntu-latest, n)` の手順の要約から、パッケージごとの時間を集める:

```bash
gh run view <run id> --json jobs --jq '.jobs[] | select(.name | startswith("test (")) | {name, startedAt, completedAt}'
```

（要約の表は Actions の画面か `gh api repos/kukv/hedgehug/check-runs/<job id>` の `output.summary` で読む。）

ubuntu の3つの shard の Test の時間の最大が、最小の 1.5 倍を超えていたら分け方を直す。
- 1つのパッケージだけで全体の 1/3 を超えるときは、そのパッケージをテストの名前で2つに分ける。`go test -list . ./<pkg>` で名前の一覧を取り（テストは回らない）、合計の時間が近くなる頭文字の区切りを選び、shard を足して `-run '^Test[A-L]'` と `-skip '^Test[A-L]'` を `case` に書く。マトリクスの `shard` も増やす
- そうでなければ、shard 1・2 に入れるパッケージを入れ替える

直したら push し、もう1回 CI を見る。

- [ ] **Step 3: lint の時間を確かめる**

lint の `--verbose` の出力から、時間のかかっている段（パッケージの読み込み・型検査・各 linter）を読む。
- 特定の linter が大半なら、その linter の設定（対象の絞り方）を利用者に提案する（勝手に外さない）
- パッケージの読み込みが大半なら、lint のジョブに Go のビルドキャッシュの復元・保存（キー `go-Linux-X64-lint-<hash>`、保存は main の push だけ）を足す

どちらにしても、最後に `args: --verbose` を外す。

- [ ] **Step 4: 必須チェックを付け替える（利用者の承認を得てから）**

PR の CI がすべて緑になったら、利用者に ruleset の変更の承認を求める。承認されたら:

```bash
mkdir -p .tmp
gh api repos/kukv/hedgehug/rulesets/26175024 > .tmp/ruleset.json
jq '{name, target, enforcement, conditions, bypass_actors: (.bypass_actors // []), rules: [.rules[] | if .type == "required_status_checks" then .parameters.required_status_checks |= ([.[] | select(.context | startswith("test (") | not)] + [{context: "test"}]) else . end]}' .tmp/ruleset.json > .tmp/ruleset-new.json
gh api -X PUT repos/kukv/hedgehug/rulesets/26175024 --input .tmp/ruleset-new.json --jq '.rules[] | select(.type=="required_status_checks") | .parameters.required_status_checks[].context'
```
Expected: `test (…)` の3つが消え、`test` が入っている

- [ ] **Step 5: 親設計を直す**

親設計（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の Windows に触れる箇所を、docs の検索（`{"kind":"search","text":"Windows"}`）で探して直す。2026-11-02 の時点では8か所。

| 今の記述 | 直したあと |
| --- | --- |
| ロックは OS のファイルロックで取る（Unix は flock、Windows は LockFileEx）。… | ロックは OS のファイルロック（flock）で取る。… |
| Windows の rename は、ウイルス対策ソフトなどが…再試行する | 項目ごと消す |
| OS の資格情報ストア（macOS Keychain、Windows 資格情報マネージャー、Secret Service）… | OS の資格情報ストア（macOS Keychain、Secret Service）… |
| 入れ替えの手順の「（Windows で開いたファイルを含むディレクトリを rename できないため。2026-11-01 決定）」 | 変えない（手順はそのまま残るため） |
| macOS / Windows / Linux に対応 | macOS / Linux に対応（Windows は WSL で使う。2026-11-02 決定） |
| macOS / Linux / Windows / WSL の差を吸収する | macOS / Linux / WSL の差を吸収する |
| goreleaser による macOS / Linux / Windows（amd64 / arm64）向けのバイナリ、Homebrew と Scoop での配布 | goreleaser による macOS / Linux（amd64 / arm64）向けのバイナリ、Homebrew での配布 |
| Windows でのファイルロックと rename → OS のロックと上限付きの再試行（…） | Windows でのファイルロックと rename → 2026-11-02 に Windows を対象外にした（WSL で使う） |

- [ ] **Step 6: PR の説明を書いて ready にする**

説明に入れるもの: 目的、利用者の決定、変更の一覧（Task 1〜6）、CI の前後の時間（main の 41355134695 と、この PR の最後の CI のジョブごとの時間）、テストの件数（前: 2110。消したテスト: TestRetryRename、TestLocalPath、paths_test の Windows の行、conn_test の Windows のパス）、ローカルの容量の対策（`-trimpath`・`make clean-cache`・`testenv`・`wsl --set-sparse`）、残した待ちとその理由。

```bash
gh pr edit --body-file .tmp/pr-body.md
gh pr ready
```

- [ ] **Step 7: 記録を更新する**

- メモリ `ci-windows-slow` を「Windows をやめて解消（2026-11-02）」に直す
- メモリ `ci-speedup` に今回の決定（shard、`test` の必須チェック、-trimpath、testenv）を足す
- メモリ `phase4-gui-progress` に、4d-1 の Task 2（ci.yaml・Makefile）はこの PR の後の形に合わせ直すことを書く
