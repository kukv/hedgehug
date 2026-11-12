# hedgehug 土台 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** トラック A〜D が互いを待たずに並べて作れるよう、パッケージの骨組み・ルールの強制・domain の中心の型・保存の口と偽物・gitstore・httpapi と画面の骨組みを先に作る。

**Architecture:** 設計書(下の Spec)の 1〜7 節のとおり。Go は domain → app/port → app/<機能>・infra・adapter → cmd の向きで、archtest と depguard で強制する。保存の口 `port.Stores` は、メモリ上の偽物(`app/apptest`)と git 版(`infra/gitstore`)の両方が同じ照合テストを通す。変更記録の付け方(`change.Stamp`)といまの値の決め方(`change.Resolve`)は domain に置き、偽物と gitstore が同じ関数を使う。画面は core・api・ui・app の 4 パッケージで、dependency-cruiser で向きを強制する。

**Tech Stack:** Go 1.27.3、go-git v5、huma v2、oklog/ulid v2、go.yaml.in/yaml/v4、golang.org/x/sys、golangci-lint 2.15.0、gotestsum。pnpm 12.17.0、Node 26.13.0、Svelte 5、Vite、orval、Biome、Vitest、Testing Library、Playwright、dependency-cruiser。ツールの版は mise で固定する。

**Spec:**
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(パッケージ構成。この計画の正本)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(保存形式。「ファイル構成」「変更記録」「いまの値の決め方」「技術構成」)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/01-overview.md`(用語、「あなた」とコミットの作者、書き込みの単位)
- 画面の見本: モック版 19 https://claude.ai/artifact/Qw2jqgdbYtbcA4VTMHGCVA(Artifact の read で `index.html`・`ui.js`・`screens.js` を取得できる)

**持ち場:** 全体(土台なので、どのフォルダも作ってよい)。**待つトラック:** なし。

**進め方(2026-11-12 決定):** 土台を 3 つのラインに分け、ラインごとに別のセッション・別の worktree で進める。PR の向き先は `develop`。タスクごとに PR を作ってこまめに入れる(設計書 8 節「小さく入れる」)。

| ライン | タスク | ブランチ(タスクごとに `origin/develop` から切る) | 始められるとき |
|---|---|---|---|
| 共通 | 1 | `feat/foundation-base` | すぐ |
| Go | 2 → 3 → 4 → 5 → 6 → 7 → 8 | `feat/foundation-go-<タスク番号>` | Task 1 が develop に入ったら |
| 画面 | 9 → 11 → 10 → 12 | `feat/foundation-web-<タスク番号>` | Task 1 が develop に入ったら。Task 10 は Task 8 が、Task 12 は Task 8 が develop に入ってから |

- Go のラインが触るのは `internal/`・`cmd/`・`api/`・`go.mod`・`go.sum`・`web/embed.go`・`web/dist/.gitkeep`・`.github/workflows/ci-backend.yaml` だけ。画面のラインが触るのは `web/`(`embed.go` を除く)・`Makefile`・`.github/workflows/ci-frontend.yaml` だけ。Task 1 のあとは、2 つのラインが同じファイルを触らない。
- 前のタスクの PR が develop に入る前に次のタスクを始めるときは、前のタスクのブランチから切り、前の PR が入ったら develop に rebase する。
- 画面のラインは、Task 11 のあと Task 8 が develop に入っていなければ待つ。待つあいだに A〜D の計画を書いてよい。
- 各セッションへの頼み方の例: 「docs/superpowers/plans/2026-11-12-hedgehug-foundation.md の Go のライン(Task 2〜8)を superpowers:subagent-driven-development で進めて」

**完成の条件:** 次の 3 つがすべて通る。
1. `make check`(tidy・lint・fmt・Go のテスト・web の検査)が緑。
2. `apptest.RunStoresContract` が、偽物と gitstore の両方で通る。
3. `make e2e` で、e2e 用にビルドしたサーバーが画面の全体の枠を返し、Playwright がスマホ・タブレット・デスクトップの 3 幅で通る。

## Global Constraints

- モジュールパスは `github.com/kukv/hedgehug`。Go は `go 1.27.3`。
- 依存の向きは設計書 2 節の表のとおり。外部ライブラリを import してよいのは infra・adapter・cmd だけ。テストも同じ表に従い、例外は `app/apptest` をどのテストからも import してよいことだけ。
- 大きさの上限(設計書 4 節): Go のファイル 300 行(テスト 600 行、生成したファイルは除く)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行(生成したファイルは除く)。超えたら分ける。上限は上げない。
- usecase は 1 ファイル 1 操作(土台では usecase を作らない)。
- 口は git の言葉(コミット、ブランチ、clone)を使わない。git の言葉は `infra/gitstore` の中だけ。
- 内部 ID はすべて ULID(文字列 26 文字)。時刻はミリ秒までの RFC 3339 で、このマシンの時刻帯を付けて書く(`2006-01-02T15:04:05.000Z07:00`)。比べるときは UTC。
- コミットの作者とコミッター: 実行者が人(`user`)なら `<ログイン名> <<ログイン名>@hedgehug.invalid>`、それ以外は `hedgehug[bot] <bot@hedgehug.invalid>`(01「「あなた」の決まり方」)。
- 保存のコミットのメッセージは、1 行目が `Operation.Summary`、空行のあとに `Hedgehug-Op: <操作の ID>`。
- 画面のコードのコメントと UI の文言は日本語、識別子は英語。
- 新しく入れる依存は、手で版を書かずに、ステップのコマンド(`go get …@latest`、`pnpm add -E …`)で入れる。pnpm は `minimumReleaseAge` で 7 日未満の版を入れない。
- 前回のファイルを写すときは `git show cf58222d~1:<path>` で取り出し、ステップに書いた変更だけを加える。

## この計画で決めたこと(設計書の規則の当てはめ)

設計書が「各計画で決める」としたものを、設計書の規則で次のように決めた。

| # | 決めたこと | 根拠 |
|---|---|---|
| 1 | 土台で定義する口は `Stores`・`Clock`・`IDs` だけ。`Replicator`・`StoreProvisioner`・`Index`・`Inbox`・`Views`・`MachineSettings`・`GitHub`・`Secrets`・`Events` は、使うトラックが自分の計画で足す(C: Replicator・Inbox・Events、D: GitHub・Secrets、E: Index・Views、G: StoreProvisioner・MachineSettings)。口は 1 つ 1 ファイル(`app/port/<口>.go`)、偽物は `app/apptest/<口>.go`、照合テストは `app/apptest/<口>_contract.go` に置く | 3 節「先回りして足さない」。1 口 1 ファイルなので、別々のトラックが足しても衝突しない |
| 2 | `Reader`・`Tx` から `Planning()`・`WritePlanning()` を外す。トラック F が、口だけの小さな PR で足す | スプリント・リリースの型が土台にない。3 節「その機能が使うものだけを足していく」 |
| 3 | 種類ごとの Reader は、土台では `Get`(と、ワークスペース・メンバーの `List`)だけ。Writer は `Append(id, drafts...)` の 1 つだけで、usecase は `change.Draft`(種類・項目・値)を渡す。ID・操作・時刻・実行者・`prev` は Stores の実装が `change.Stamp` で付ける | `prev` は「書いた時点の先端」なので、ログを持つ側でしか決められない(保存形式「変更記録」) |
| 4 | いまの値の決め方(保存形式の手順 0〜3)は `domain/change.Resolve` に置き、偽物と gitstore が同じ関数を呼ぶ。土台が扱う変更の種類は `create`・`set`・`add`・`remove` だけ。ほかの種類(`delete`・`restore`・`desc`・`comment` など)は行として保存・読み出しはできるが、Resolve は無視する。各トラックが種類を足す | 偽物と本物がずれないため(3 節「テストのための仕組み」)。3 節の「項目ごとの後勝ちで合わせる処理は infra/gitstore」は、2 つのクローンの log.jsonl を合わせる同期の処理(トラック C)を指す。1 つのログからいまの値を決める規則は入出力のない規則なので domain に置く |
| 5 | `*.yaml` の行末の注釈(値の名前・表示の ID)は土台では書かない。アイテム・プロジェクトの注釈はトラック A、ワークスペースの値の注釈はトラック B が足す | 注釈は名前の変更のたびの作り直しを伴い、その操作を持つトラックが決めるのが速い |
| 6 | 本物のサーバーは Store を 1 つも開かずに起動する。Store の一覧(config.yaml)・追加・初回起動はトラック G | 設計書 8 節の段階 0 の表に入っていない |
| 7 | `cmd/hedgehug` は、CLI ができるまでのあいだ、起動すると `127.0.0.1` の空いた番号で待ち受け、`http://127.0.0.1:<番号>/#token=<トークン>` を標準出力に出す。トークンは起動ごとの乱数(32 バイトの 16 進)。API はすべて `Authorization: Bearer <トークン>` を要る。画面のファイルはトークンなしで返す | 08「トークンのない URL」。トークンを URL のハッシュに置くのは前回と同じ(サーバーのログと Referer に残らない) |
| 8 | 08 の「トークンのない URL」「サーバーに接続できない」の 2 画面はトラック G | 08 の画面 |
| 9 | API のルートは機能ごとのパッケージ `adapter/httpapi/<機能>` に置き、各パッケージが `Register(api huma.API, …)` を持つ。`httpapi/routes.go` は各 Register を 1 行ずつ呼ぶ | 8 節「ルートの登録は機能ごとのファイルに分ける」と持ち場 `adapter/httpapi/<機能>` |
| 10 | 土台の API は `GET /api/system`(版を返す)の 1 つだけ。orval の生成と、トークン付きの呼び出しを通しで確かめるため | 機能の API は各トラック |
| 11 | release(goreleaser)のワークフローは土台に入れない。最初に配る前に、ライセンスの書き出し(前回の `tools/licenses`)と一緒に足す | 配るものがまだない。前回の release はライセンスの書き出しに依存する |
| 12 | 画面の幅の切り替えは、モックの 860px を、全体の枠のコンテナクエリで行う | 6 節「コンテナクエリ」。モックは画面の幅で切り替えている |
| 13 | e2e 用のビルド(`-tags e2e`)は、Stores に `apptest` の偽物とサンプルデータを差し込み、トークンを環境変数 `HEDGEHUG_TOKEN`、待ち受けを `HEDGEHUG_ADDR` で決める | 6 節「画面のテスト」 |
| 14 | 土台を、共通(Task 1)・Go(Task 2〜8)・画面(Task 9〜12)の 3 つのラインに分け、Task 1 のあとは Go と画面を別のセッションで同時に進める | ユーザーの決定(2026-11-12)。設計書 8 節も合わせて直した |
| 15 | 新しく作る Store のブランチは `main`(Task 6 の `Create`) | ユーザーの決定(2026-11-12) |

**決着した食い違い:** `MachineSettings`(ログイン名・同期のタイミング・Store の一覧・アカウント)は `config.yaml` 1 つに置き、実装は `infra/machineconfig`(2026-11-12 決定。設計書 3 節を直した)。

---

## ファイルの地図

```
go.mod / go.sum / mise.toml / Makefile / .golangci.yaml / .octocov.yaml
.github/workflows/ci-backend.yaml / ci-frontend.yaml(security.yaml は今のまま)

cmd/hedgehug/main.go             起動、トークン、待ち受け、URL の表示
cmd/hedgehug/wire.go             //go:build !e2e  本物の部品の組み立て
cmd/hedgehug/wire_e2e.go         //go:build e2e   偽物とサンプルデータの組み立て
cmd/openapi-gen/main.go          api/openapi.yaml を書く

internal/archtest/rules.go       依存の表(設計書 2 節)を関数にしたもの
internal/archtest/rules_test.go  表のテスト(違反の例を含む)
internal/archtest/repo_test.go   このリポジトリを go list で調べ、表と大きさの上限を確かめる
internal/archtest/size.go        行数・ファイル数の数え方

internal/domain/change/record.go   Record・Type・Ref、log.jsonl の 1 行の JSON
internal/domain/change/actor.go    Actor(実行者)
internal/domain/change/draft.go    Draft(usecase が書く変更)
internal/domain/change/stamp.go    Stamp(ID・操作・時刻・実行者・prev を付ける)
internal/domain/change/resolve.go  Resolve(いまの値の決め方 手順 0〜3)
internal/domain/member/member.go
internal/domain/workspace/workspace.go
internal/domain/project/project.go
internal/domain/item/item.go       Item・Kind・親子の規則

internal/app/port/stores.go        Stores・Operation・Reader・Tx・種類ごとの Reader/Writer・ErrNotFound
internal/app/port/clock.go         Clock・IDs
internal/app/apptest/clock.go      偽の Clock・IDs
internal/app/apptest/stores.go     Stores の偽物
internal/app/apptest/stores_read.go 偽物の Reader
internal/app/apptest/stores_contract.go RunStoresContract と準備のデータ
internal/app/apptest/stores_contract_cases.go 照合テストの各場面
internal/app/apptest/sample.go     サンプルデータ
internal/app/apptest/stores_test.go 偽物に照合テストを流す

internal/infra/sysclock/sysclock.go
internal/infra/gitstore/create.go  Store を作る(store.yaml の最初のコミット)
internal/infra/gitstore/stores.go  Stores(Attach・Read・Write)
internal/infra/gitstore/lock.go    Store ごとのファイルロック
internal/infra/gitstore/files.go   log.jsonl の読み書き、パス
internal/infra/gitstore/yaml.go    *.yaml を書く
internal/infra/gitstore/commit.go  1 回の Write = 1 コミット、失敗したら戻す
internal/infra/gitstore/read.go    Reader
internal/infra/gitstore/write.go   Read・Write・Tx
internal/infra/gitstore/*_test.go

internal/adapter/httpapi/server.go     Handler(トークン、/api、画面のファイル)
internal/adapter/httpapi/routes.go     NewAPI と各機能の Register の呼び出し
internal/adapter/httpapi/generate.go   //go:generate
internal/adapter/httpapi/system/system.go  GET /api/system
api/openapi.yaml                       生成物

web/embed.go / web/dist/.gitkeep
web/package.json / pnpm-workspace.yaml / biome.json / tsconfig.base.json / .dependency-cruiser.cjs
web/scripts/check-size.mjs
web/packages/core/   reactive/・features/shell/・model/・port/
web/packages/api/    orval の生成物、fetcher、token、system の口の実装
web/packages/ui/     tokens/・primitives/・layout/
web/app/             main.ts・wire.ts・App.svelte・vite.config.ts
web/playwright.config.ts / web/e2e/frame.spec.ts
```

---

### Task 1: Go の骨組みと依存のルール

モジュール、ツール、lint、依存の表を確かめる archtest、ci-backend を作る。以後のタスクはすべてこのルールの上で作る。

**Files:**
- Create: `go.mod`、`mise.toml`、`Makefile`、`.golangci.yaml`、`.octocov.yaml`
- Create: `cmd/hedgehug/main.go`(仮。Task 8 で書き直す)
- Create: `internal/archtest/rules.go`、`internal/archtest/size.go`
- Test: `internal/archtest/rules_test.go`、`internal/archtest/repo_test.go`
- Create: `.github/workflows/ci-backend.yaml`

**Interfaces:**
- Consumes: なし
- Produces: `archtest.Allowed(from, to string, test bool) bool`。`make fmt`・`make lint`・`make test`・`make check`。

- [ ] **Step 1: モジュールとツールを置く**

```bash
git switch -c feat/foundation-base origin/develop
go mod init github.com/kukv/hedgehug
go mod edit -go=1.27.3
git show cf58222d~1:mise.toml > mise.toml
git show cf58222d~1:.octocov.yaml > .octocov.yaml
mise install
```

`cmd/hedgehug/main.go`(Task 8 で書き直す):

```go
// Command hedgehug は、このマシンで画面のサーバーを動かす。
package main

func main() {}
```

- [ ] **Step 2: Makefile を書く**

`Makefile`(画面の目標は Task 9・10・12 で足す):

```make
.PHONY: generate build fmt fmt-check lint tidy-check test cover clean-cache check

# 生成物(api/openapi.yaml)を書く。
generate:
	go generate ./...

# bin/ にバイナリを作る。
build:
	go build -o bin/hedgehug ./cmd/hedgehug

# 書式を直す(gofumpt と goimports)。
fmt:
	golangci-lint fmt ./...

# 書式が崩れていたら落とす(CI と同じ)。
fmt-check:
	golangci-lint fmt --diff ./...

# lint(CI と同じ)。
lint:
	golangci-lint run ./...

# go.mod・go.sum が整っていなければ落とす(CI と同じ)。
tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

# race 付きでテストする。
test:
	gotestsum --format testdox -- -race ./...

# race とカバレッジ付きでテストする(CI がカバレッジとして出すもの)。
cover:
	gotestsum --format testdox -- -race -coverprofile=coverage.out -covermode=atomic ./...

# Go のビルドとテストのキャッシュを空にする。
clean-cache:
	go clean -cache -testcache

# CI が確かめることを手元でまとめて行う。
check: tidy-check lint fmt-check test
```

- [ ] **Step 3: golangci-lint の設定を書く**

`.golangci.yaml`:

```yaml
version: "2"

linters:
  default: standard
  enable:
    - revive
    - gosec
    - misspell
    - gocritic
    - bodyclose
    - unconvert
    - unparam
    - depguard
    - funlen
    - gocognit
  settings:
    # 大きさの上限(設計書 4 節)。超えたら上限を上げずに分ける。
    funlen:
      lines: 60
      statements: -1
      ignore-comments: true
    gocognit:
      min-complexity: 15
    # 依存の向き(設計書 2 節)のうち、depguard で書けるもの。
    # 機能どうし(app/<機能>・infra/<実装>・adapter/<口>)の禁止と、apptest をテストだけに
    # 限ることは、internal/archtest が確かめる。
    depguard:
      rules:
        domain:
          list-mode: strict
          files:
            - "**/internal/domain/**"
            - "!$test"
          allow:
            - $gostd
            - github.com/kukv/hedgehug/internal/domain
        domain-test:
          list-mode: strict
          files:
            - "**/internal/domain/**/*_test.go"
          allow:
            - $gostd
            - github.com/kukv/hedgehug/internal/domain
            - github.com/kukv/hedgehug/internal/app/apptest
        port:
          list-mode: strict
          files:
            - "**/internal/app/port/**"
          allow:
            - $gostd
            - github.com/kukv/hedgehug/internal/domain
        app:
          list-mode: strict
          files:
            - "**/internal/app/**"
            - "!**/internal/app/port/**"
          allow:
            - $gostd
            - github.com/kukv/hedgehug/internal/domain
            - github.com/kukv/hedgehug/internal/app
        infra:
          list-mode: lax
          files:
            - "**/internal/infra/**"
          deny:
            - pkg: github.com/kukv/hedgehug/internal/adapter
              desc: infra は adapter を import しない(設計書 2 節)
        adapter:
          list-mode: lax
          files:
            - "**/internal/adapter/**"
          deny:
            - pkg: github.com/kukv/hedgehug/internal/infra
              desc: adapter は infra を import しない(設計書 2 節)
  exclusions:
    rules:
      - linters:
          - revive
        text: "^(exported|unused-parameter):"
      - path: (_test\.go|internal/app/apptest/)
        linters:
          - gosec
          - funlen

issues:
  max-issues-per-linter: 0
  max-same-issues: 0

formatters:
  enable:
    - gofumpt
    - goimports
```

- [ ] **Step 4: 依存の表のテストを書く(違反の例を含む)**

`internal/archtest/rules_test.go`:

```go
package archtest

import "testing"

func TestAllowed(t *testing.T) {
	const m = "github.com/kukv/hedgehug/"
	cases := []struct {
		from, to string
		test     bool
		want     bool
	}{
		{m + "internal/domain/item", "fmt", false, true},
		{m + "internal/domain/item", m + "internal/domain/change", false, true},
		{m + "internal/domain/item", "github.com/oklog/ulid/v2", false, false},
		{m + "internal/domain/item", m + "internal/app/port", false, false},
		{m + "internal/domain/item", m + "internal/infra/gitstore", false, false},
		{m + "internal/domain/item", m + "internal/app/apptest", true, true},
		{m + "internal/app/port", m + "internal/domain/item", false, true},
		{m + "internal/app/port", m + "internal/app/items", false, false},
		{m + "internal/app/items", m + "internal/app/port", false, true},
		{m + "internal/app/items", m + "internal/app/items/move", false, true},
		{m + "internal/app/items", m + "internal/app/sync", false, false},
		{m + "internal/app/items", m + "internal/app/apptest", false, false},
		{m + "internal/app/items", m + "internal/app/apptest", true, true},
		{m + "internal/app/items", "github.com/go-git/go-git/v5", false, false},
		{m + "internal/app/apptest", m + "internal/app/port", false, true},
		{m + "internal/app/apptest", m + "internal/infra/gitstore", false, false},
		{m + "internal/infra/gitstore", m + "internal/app/port", false, true},
		{m + "internal/infra/gitstore", m + "internal/app/items", false, false},
		{m + "internal/infra/gitstore", m + "internal/infra/github", false, false},
		{m + "internal/infra/gitstore", "github.com/go-git/go-git/v5", false, true},
		{m + "internal/adapter/httpapi", m + "internal/app/items", false, true},
		{m + "internal/adapter/httpapi", m + "internal/adapter/httpapi/system", false, true},
		{m + "internal/adapter/httpapi", m + "internal/adapter/cli", false, false},
		{m + "internal/adapter/httpapi", m + "internal/infra/gitstore", false, false},
		{m + "internal/adapter/httpapi", m + "internal/app/apptest", false, false},
		{m + "cmd/hedgehug", m + "internal/infra/gitstore", false, true},
		{m + "cmd/hedgehug", m + "internal/app/apptest", false, true},
		{m + "web", m + "internal/domain/item", false, false},
		{m + "internal/archtest", "os/exec", false, true},
	}
	for _, c := range cases {
		if got := Allowed(c.from, c.to, c.test); got != c.want {
			t.Errorf("Allowed(%s, %s, test=%v) = %v, want %v", c.from, c.to, c.test, got, c.want)
		}
	}
}

func TestLineCount(t *testing.T) {
	for in, want := range map[string]int{"": 0, "a": 1, "a\n": 1, "a\nb": 2, "a\nb\n": 2} {
		if got := lineCount([]byte(in)); got != want {
			t.Errorf("lineCount(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestGenerated(t *testing.T) {
	if !isGenerated([]byte("// Code generated by orval. DO NOT EDIT.\npackage x\n")) {
		t.Error("generated file not detected")
	}
	if isGenerated([]byte("package x\n")) {
		t.Error("plain file detected as generated")
	}
}
```

- [ ] **Step 5: テストが落ちることを確かめる**

Run: `go test ./internal/archtest/`
Expected: FAIL(`undefined: Allowed`)

- [ ] **Step 6: 依存の表と大きさの数え方を書く**

`internal/archtest/rules.go`:

```go
// Package archtest は、設計書 2 節の依存の向きと 4 節の大きさの上限を、テストで確かめる。
package archtest

import "strings"

const module = "github.com/kukv/hedgehug"

// unit は、依存の表の 1 行に当たるパッケージのまとまり。
// name は app・infra・adapter の下の名前(internal/app/items/move なら items)。
type unit struct {
	kind string // domain, port, apptest, app, infra, adapter, cmd, other, external
	name string
}

func unitOf(path string) unit {
	rel, ok := strings.CutPrefix(path, module+"/")
	if !ok {
		if path == module {
			return unit{kind: "other"}
		}
		return unit{kind: "external"}
	}
	p := strings.Split(rel, "/")
	switch {
	case p[0] == "cmd":
		return unit{kind: "cmd"}
	case len(p) >= 2 && p[0] == "internal" && p[1] == "domain":
		return unit{kind: "domain"}
	case len(p) >= 3 && p[0] == "internal" && p[1] == "app" && (p[2] == "port" || p[2] == "apptest"):
		return unit{kind: p[2]}
	case len(p) >= 3 && p[0] == "internal" && (p[1] == "app" || p[1] == "infra" || p[1] == "adapter"):
		return unit{kind: p[1], name: p[2]}
	}
	return unit{kind: "other"}
}

// isStd は、標準ライブラリのパッケージかを返す。標準ライブラリのパスは、最初の要素に「.」を含まない。
func isStd(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// Allowed は、from のパッケージが to を import してよいかを返す(設計書 2 節の表)。
// test は、テストのファイルからの import のとき true。テストも同じ表に従い、例外は apptest だけ。
func Allowed(from, to string, test bool) bool {
	f, t := unitOf(from), unitOf(to)
	if test && t.kind == "apptest" {
		return true
	}
	if t.kind == "external" {
		if isStd(to) {
			return true
		}
		return f.kind == "infra" || f.kind == "adapter" || f.kind == "cmd"
	}
	same := t.kind == f.kind && t.name == f.name
	switch f.kind {
	case "domain":
		return t.kind == "domain"
	case "port", "apptest":
		return t.kind == "domain" || t.kind == "port"
	case "app", "infra":
		return t.kind == "domain" || t.kind == "port" || same
	case "adapter":
		return t.kind == "domain" || t.kind == "port" || t.kind == "app" || same
	case "cmd":
		return true
	}
	return false
}
```

`internal/archtest/size.go`:

```go
package archtest

import (
	"bytes"
	"regexp"
)

// 設計書 4 節の上限。
const (
	maxFileLines     = 300
	maxTestFileLines = 600
	maxPackageFiles  = 20
)

var generatedMark = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

func isGenerated(src []byte) bool { return generatedMark.Match(src) }

func lineCount(src []byte) int {
	n := bytes.Count(src, []byte("\n"))
	if len(src) > 0 && src[len(src)-1] != '\n' {
		n++
	}
	return n
}
```

- [ ] **Step 7: テストが通ることを確かめる**

Run: `go test ./internal/archtest/`
Expected: PASS

- [ ] **Step 8: リポジトリ全体を確かめるテストを書く**

`internal/archtest/repo_test.go`:

```go
package archtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type goPackage struct {
	ImportPath   string
	Dir          string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func listPackages(t *testing.T) []goPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkgs []goPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p goPackage
		err := dec.Decode(&p)
		if errors.Is(err, io.EOF) {
			return pkgs
		}
		if err != nil {
			t.Fatalf("decode go list: %v", err)
		}
		pkgs = append(pkgs, p)
	}
}

// TestImports は、すべてのパッケージの import が設計書 2 節の表に合うことを確かめる。
func TestImports(t *testing.T) {
	for _, p := range listPackages(t) {
		for _, imp := range p.Imports {
			if !Allowed(p.ImportPath, imp, false) {
				t.Errorf("%s imports %s (設計書 2 節)", p.ImportPath, imp)
			}
		}
		for _, imp := range append(p.TestImports, p.XTestImports...) {
			if !Allowed(p.ImportPath, imp, true) {
				t.Errorf("tests of %s import %s (設計書 2 節)", p.ImportPath, imp)
			}
		}
	}
}

// TestSizes は、設計書 4 節のファイルとパッケージの大きさの上限を確かめる。
func TestSizes(t *testing.T) {
	for _, p := range listPackages(t) {
		if len(p.GoFiles) > maxPackageFiles {
			t.Errorf("%s has %d non-test files (max %d): split the package", p.ImportPath, len(p.GoFiles), maxPackageFiles)
		}
		checkLines(t, p.Dir, p.GoFiles, maxFileLines)
		checkLines(t, p.Dir, append(p.TestGoFiles, p.XTestGoFiles...), maxTestFileLines)
	}
}

func checkLines(t *testing.T, dir string, files []string, limit int) {
	t.Helper()
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if n := lineCount(src); n > limit && !isGenerated(src) {
			t.Errorf("%s has %d lines (max %d): split the file", filepath.Join(dir, f), n, limit)
		}
	}
}
```

- [ ] **Step 9: 違反を置いて、テストと lint が落ちることを確かめる**

`internal/domain/planted/planted.go` を一時的に作る:

```go
package planted

import _ "github.com/kukv/hedgehug/internal/archtest"
```

Run: `go test ./internal/archtest/ -run TestImports`
Expected: FAIL(`github.com/kukv/hedgehug/internal/domain/planted imports github.com/kukv/hedgehug/internal/archtest`)

Run: `golangci-lint run ./internal/domain/...`
Expected: depguard の指摘が 1 件出る。出なければ `.golangci.yaml` の depguard の `files` の書き方を直す。

確かめたら消す: `rm -r internal/domain/planted`

- [ ] **Step 10: 全体が通ることを確かめる**

Run: `make check`
Expected: すべて成功(`go mod tidy` で差分なし、lint 0 件、テスト PASS)

- [ ] **Step 11: ci-backend を写す**

```bash
git show cf58222d~1:.github/workflows/ci-backend.yaml > .github/workflows/ci-backend.yaml
```

次だけ変える。
1. `on.push.branches` を `[main, develop]` にする(PR の向き先が develop のため)。
2. lint ジョブの「Vet the e2e build」の `go vet -tags e2e ./internal/app/cli` を `go vet -tags e2e ./cmd/hedgehug` にする。
3. test-shard の `matrix.include` を次の 2 つにする(前回のパスでの分け方は使えない。時間が延びたら Package times を見て分け直す)。
   ```yaml
          - os: ubuntu-latest
            shard: all
            race: -race
            report: true
          - os: macos-latest
            shard: all
   ```
4. Test の手順の `mod=$(go list -m)` の行と `case "$SHARD" in … esac` を消し、`args=("./...")` の 1 行にする。その上の、前回のパスでの分け方を説明するコメントも消す。

- [ ] **Step 12: Commit**

```bash
git add go.mod mise.toml Makefile .golangci.yaml .octocov.yaml cmd internal .github/workflows/ci-backend.yaml
git commit -m "build: Go の骨組みと依存のルール(archtest・depguard・ci-backend)を置く"
```

---

### Task 2: 変更記録の型(domain/change)

`log.jsonl` の 1 行(保存形式「変更記録」)と、実行者、エンティティの参照、usecase が渡す下書きを作る。

**Files:**
- Create: `internal/domain/change/record.go`、`internal/domain/change/actor.go`、`internal/domain/change/draft.go`
- Test: `internal/domain/change/record_test.go`

**Interfaces:**
- Consumes: なし
- Produces:
  - `type ID string`、`type Type string`(`Create`・`Set`・`Add`・`Remove`)、`const TimeLayout`
  - `type Record struct { ID, Op ID; At time.Time; By string; Actor Actor; T Type; F string; From, To json.RawMessage; Prev []ID; Root *bool }`。`MarshalJSON`・`UnmarshalJSON` で log.jsonl の 1 行になる
  - `type ActorKind string`(`ActorUser`・`ActorAutomation`・`ActorGitHub`・`ActorSync`・`ActorNone`)、`type Actor struct { Kind ActorKind; Login string; Rule ID }`、`func User(login string) Actor`、`(Actor) By() string`、`(Actor) Code() string`、`func ParseActor(code, by string) (Actor, error)`、`const BotLogin = "hedgehug[bot]"`
  - `type Kind string`(`KindMembers`・`KindWorkspace`・`KindProject`・`KindItem`)、`type Ref struct { Kind Kind; ID ID }`
  - `type Draft struct { T Type; F string; From, To json.RawMessage; Root *bool }`、`func Value(v any) json.RawMessage`、`func NewCreate(to any) Draft`、`func NewSet(f string, from, to any) Draft`、`func NewAdd(f string, v any) Draft`、`func NewRemove(f string, v any) Draft`

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/change/record_test.go`:

```go
package change_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

var jst = time.FixedZone("JST", 9*3600)

// 保存形式「アイテムの例」の 1 行と同じ並びで書く。
func TestRecordMarshal(t *testing.T) {
	r := change.Record{
		ID: "01JH7Q", Op: "01JH7Q",
		At:    time.Date(2026, 11, 12, 10, 3, 21, 512_000_000, jst),
		By:    "taro",
		Actor: change.User("taro"),
		T:     change.Set, F: "status",
		From: change.Value("S1"), To: change.Value("S3"),
		Prev: []change.ID{"01JF3K"},
	}
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"01JH7Q","op":"01JH7Q","at":"2026-11-12T10:03:21.512+09:00","by":"taro","actor":"user","t":"set","f":"status","from":"S1","to":"S3","prev":["01JF3K"]}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestRecordRoundTrip(t *testing.T) {
	root := true
	in := change.Record{
		ID: "a", Op: "o", At: time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, jst), By: "hanako",
		Actor: change.Actor{Kind: change.ActorAutomation, Login: "hanako", Rule: "R1"},
		T:     change.Set, F: "release", From: change.Value("REL1"), To: json.RawMessage("null"),
		Prev: []change.ID{"x", "y"}, Root: &root,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out change.Record
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	b2, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(b2) {
		t.Errorf("round trip changed the line:\n%s\n%s", b, b2)
	}
	if !out.At.Equal(in.At) || out.Actor != in.Actor || string(out.To) != "null" {
		t.Errorf("decoded %+v", out)
	}
}

func TestCreateHasEmptyPrev(t *testing.T) {
	b, err := json.Marshal(change.Record{ID: "a", Op: "a", At: time.Unix(0, 0).In(jst), Actor: change.User("t"), By: "t", T: change.Create, To: change.Value(map[string]string{"id": "a"})})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"a","op":"a","at":"1970-01-01T09:00:00.000+09:00","by":"t","actor":"user","t":"create","to":{"id":"a"},"prev":[]}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

func TestActor(t *testing.T) {
	cases := []struct {
		a        change.Actor
		code, by string
	}{
		{change.User("taro"), "user", "taro"},
		{change.Actor{Kind: change.ActorAutomation, Login: "taro", Rule: "R1"}, "automation:R1", "taro"},
		{change.Actor{Kind: change.ActorGitHub}, "github", change.BotLogin},
		{change.Actor{Kind: change.ActorSync}, "sync", change.BotLogin},
		{change.Actor{Kind: change.ActorNone}, "none", change.BotLogin},
	}
	for _, c := range cases {
		if c.a.Code() != c.code || c.a.By() != c.by {
			t.Errorf("%+v: code=%q by=%q", c.a, c.a.Code(), c.a.By())
		}
		back, err := change.ParseActor(c.code, c.by)
		if err != nil || back != c.a {
			t.Errorf("ParseActor(%q, %q) = %+v, %v", c.code, c.by, back, err)
		}
	}
	if _, err := change.ParseActor("robot", "x"); err == nil {
		t.Error("unknown actor accepted")
	}
}

func TestDrafts(t *testing.T) {
	if d := change.NewSet("title", "a", "b"); d.T != change.Set || string(d.From) != `"a"` || string(d.To) != `"b"` {
		t.Errorf("NewSet = %+v", d)
	}
	if d := change.NewAdd("systems", "Y1"); d.T != change.Add || string(d.To) != `"Y1"` || d.From != nil {
		t.Errorf("NewAdd = %+v", d)
	}
	if d := change.NewRemove("systems", "Y1"); d.T != change.Remove || string(d.From) != `"Y1"` || d.To != nil {
		t.Errorf("NewRemove = %+v", d)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/change/`
Expected: FAIL(`undefined: change.Record` ほか)

- [ ] **Step 3: 実装する**

`internal/domain/change/record.go`:

```go
// Package change は、変更の記録(log.jsonl の 1 行)と、記録からいまの値を決める規則を持つ。
package change

import (
	"encoding/json"
	"fmt"
	"time"
)

// ID は内部 ID(ULID)。
type ID string

// Type は変更の種類(保存形式「変更記録」の t)。土台は下の 4 つだけを扱い、ほかの種類はトラックが足す。
type Type string

const (
	Create Type = "create"
	Set    Type = "set"
	Add    Type = "add"
	Remove Type = "remove"
)

// TimeLayout は記録の時刻の書き方(ミリ秒までの RFC 3339、このマシンの時刻帯付き)。
const TimeLayout = "2006-01-02T15:04:05.000Z07:00"

// Record は log.jsonl の 1 行。
type Record struct {
	ID    ID
	Op    ID
	At    time.Time
	By    string
	Actor Actor
	T     Type
	F     string
	From  json.RawMessage
	To    json.RawMessage
	Prev  []ID
	Root  *bool
}

// line は log.jsonl に書く形。キーの並びは保存形式の例と同じ。
type line struct {
	ID    ID              `json:"id"`
	Op    ID              `json:"op"`
	At    string          `json:"at"`
	By    string          `json:"by"`
	Actor string          `json:"actor"`
	T     Type            `json:"t"`
	F     string          `json:"f,omitempty"`
	From  json.RawMessage `json:"from,omitempty"`
	To    json.RawMessage `json:"to,omitempty"`
	Prev  []ID            `json:"prev"`
	Root  *bool           `json:"root,omitempty"`
}

func (r Record) MarshalJSON() ([]byte, error) {
	prev := r.Prev
	if prev == nil {
		prev = []ID{}
	}
	return json.Marshal(line{
		ID: r.ID, Op: r.Op, At: r.At.Format(TimeLayout), By: r.By, Actor: r.Actor.Code(),
		T: r.T, F: r.F, From: r.From, To: r.To, Prev: prev, Root: r.Root,
	})
}

func (r *Record) UnmarshalJSON(b []byte) error {
	var l line
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	at, err := time.Parse(time.RFC3339, l.At)
	if err != nil {
		return fmt.Errorf("record %s: at: %w", l.ID, err)
	}
	actor, err := ParseActor(l.Actor, l.By)
	if err != nil {
		return fmt.Errorf("record %s: %w", l.ID, err)
	}
	*r = Record{
		ID: l.ID, Op: l.Op, At: at, By: l.By, Actor: actor,
		T: l.T, F: l.F, From: l.From, To: l.To, Prev: l.Prev, Root: l.Root,
	}
	return nil
}

// Kind はログを持つエンティティの種類。
type Kind string

const (
	KindMembers   Kind = "members"
	KindWorkspace Kind = "workspace"
	KindProject   Kind = "project"
	KindItem      Kind = "item"
)

// Ref は、どのエンティティのログかを指す。メンバーの一覧は Store に 1 つなので ID は空。
type Ref struct {
	Kind Kind
	ID   ID
}
```

`internal/domain/change/actor.go`:

```go
package change

import (
	"fmt"
	"strings"
)

// BotLogin は、人でない実行者の行の by と、そのコミットの作者の名前(01「「あなた」の決まり方」)。
const BotLogin = "hedgehug[bot]"

// ActorKind は実行者の種類(保存形式「変更記録」の actor)。
type ActorKind string

const (
	ActorUser       ActorKind = "user"
	ActorAutomation ActorKind = "automation"
	ActorGitHub     ActorKind = "github"
	ActorSync       ActorKind = "sync"
	ActorNone       ActorKind = "none"
)

// Actor は 1 回の操作の実行者。Login は人のログイン名(自動化は、きっかけになった操作をした人)。
type Actor struct {
	Kind  ActorKind
	Login string
	Rule  ID // 自動化のルールの内部 ID
}

// User は、人が行った操作の実行者。
func User(login string) Actor { return Actor{Kind: ActorUser, Login: login} }

// By は、行の by に書く値。
func (a Actor) By() string {
	if a.Kind == ActorUser || a.Kind == ActorAutomation {
		return a.Login
	}
	return BotLogin
}

// Code は、行の actor に書く値。
func (a Actor) Code() string {
	if a.Kind == ActorAutomation {
		return "automation:" + string(a.Rule)
	}
	return string(a.Kind)
}

// ParseActor は、行の actor と by から実行者を戻す。
func ParseActor(code, by string) (Actor, error) {
	if rule, ok := strings.CutPrefix(code, "automation:"); ok {
		return Actor{Kind: ActorAutomation, Login: by, Rule: ID(rule)}, nil
	}
	switch k := ActorKind(code); k {
	case ActorUser:
		return User(by), nil
	case ActorGitHub, ActorSync, ActorNone:
		return Actor{Kind: k}, nil
	}
	return Actor{}, fmt.Errorf("unknown actor %q", code)
}
```

`internal/domain/change/draft.go`:

```go
package change

import "encoding/json"

// Draft は usecase が書く変更。ID・操作・時刻・実行者・prev は、Stores の実装が Stamp で付ける。
type Draft struct {
	T    Type
	F    string
	From json.RawMessage
	To   json.RawMessage
	Root *bool
}

// Value は値を JSON にする。JSON にできない値を渡すのは呼び出しの誤りなので panic する。
func Value(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// NewCreate は作成の変更。to は作成時の値すべて。
func NewCreate(to any) Draft { return Draft{T: Create, To: Value(to)} }

// NewSet は、値を 1 つ持つ項目の変更。
func NewSet(f string, from, to any) Draft {
	return Draft{T: Set, F: f, From: Value(from), To: Value(to)}
}

// NewAdd は、値を複数持つ項目への、値 1 つの追加。
func NewAdd(f string, v any) Draft { return Draft{T: Add, F: f, To: Value(v)} }

// NewRemove は、値を複数持つ項目からの、値 1 つの取り除き。
func NewRemove(f string, v any) Draft { return Draft{T: Remove, F: f, From: Value(v)} }
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/change/ && make lint`
Expected: PASS、lint 0 件

- [ ] **Step 5: Commit**

```bash
git add internal/domain/change
git commit -m "feat(domain): 変更記録(log.jsonl の 1 行)の型を足す"
```

---

### Task 3: 記録の付け方といまの値の決め方(Stamp・Resolve)

保存形式「いまの値の決め方」の手順 0〜3 を、入出力のない関数にする。偽物と gitstore はこの関数だけを使う。

**Files:**
- Create: `internal/domain/change/stamp.go`、`internal/domain/change/resolve.go`
- Test: `internal/domain/change/resolve_test.go`、`internal/domain/change/fuzz_test.go`

**Interfaces:**
- Consumes: Task 2 の `Record`・`Draft`・`Actor`・`ID`
- Produces:
  - `type Meta struct { Op ID; At time.Time; Actor Actor }`
  - `func Stamp(log []Record, drafts []Draft, meta Meta, newID func() ID) []Record` — drafts を log の続きの行にする。`prev` は同じ項目の先端すべて。`create` の `prev` は空
  - `func Tips(log []Record, r Record) []ID` — r と同じ項目の先端(辞書順)
  - `type Options struct { Multi func(f string) bool; Valid func(r Record) bool }`
  - `type State struct { Created *Record; Values map[string]json.RawMessage; Lists map[string][]json.RawMessage }`
  - `func Resolve(log []Record, opt Options) State`

**項目の決め方:** `set` は `F` ごと、`add`・`remove` は `F` と値ごとに 1 項目。`create` の行は、どの項目にも入る(最初の `set` の `prev` が `create` を指す。保存形式の例の `"prev":["01JF3KQ8…"]`)。ほかの種類の行は `T` と `F` ごとの項目にし、Resolve は値を決めない。

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/change/resolve_test.go`:

```go
package change_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

func at(min int) time.Time { return time.Date(2026, 11, 12, 10, min, 0, 0, jst) }

func rec(id string, ts time.Time, by string, d change.Draft, prev ...change.ID) change.Record {
	return change.Record{ID: change.ID(id), Op: change.ID(id), At: ts, By: by, Actor: change.User(by),
		T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev}
}

func create(to any) change.Record { return rec("c", at(0), "taro", change.NewCreate(to)) }

var opts = change.Options{Multi: func(f string) bool { return f == "systems" }}

func value(t *testing.T, s change.State, f string) string {
	t.Helper()
	return string(s.Values[f])
}

func TestResolveCreateOnly(t *testing.T) {
	s := change.Resolve([]change.Record{create(map[string]any{"title": "a", "systems": []string{"Y1"}})}, opts)
	if s.Created == nil || value(t, s, "title") != `"a"` {
		t.Fatalf("state %+v", s)
	}
	if got := list(s, "systems"); got != `"Y1"` {
		t.Errorf("systems = %s", got)
	}
	if _, ok := s.Values["systems"]; ok {
		t.Error("multi field in Values")
	}
}

func TestResolveLaterSeenWins(t *testing.T) {
	// A と B が同時に書き(B の時計があと)、C が両方を見て書いた。C の時計が最も遅れていても C が勝つ。
	log := []change.Record{
		create(map[string]any{"title": "a"}),
		rec("A", at(10), "ann", change.NewSet("title", "a", "A"), "c"),
		rec("B", at(11), "bob", change.NewSet("title", "a", "B"), "c"),
		rec("C", at(1), "carl", change.NewSet("title", "B", "C"), "A", "B"),
	}
	if got := value(t, change.Resolve(log, opts), "title"); got != `"C"` {
		t.Errorf("title = %s, want C", got)
	}
}

func TestResolveConcurrentTieBreak(t *testing.T) {
	cases := []struct {
		name string
		a, b change.Record
		want string
	}{
		{"later at wins", rec("A", at(10), "zed", change.NewSet("title", "a", "A"), "c"), rec("B", at(11), "amy", change.NewSet("title", "a", "B"), "c"), `"B"`},
		{"same at: by lowercased", rec("A", at(10), "Bob", change.NewSet("title", "a", "A"), "c"), rec("B", at(10), "alice", change.NewSet("title", "a", "B"), "c"), `"A"`},
		{"same at and by: id", rec("A", at(10), "bob", change.NewSet("title", "a", "A"), "c"), rec("B", at(10), "BOB", change.NewSet("title", "a", "B"), "c"), `"B"`},
		{"at compared in UTC", rec("A", at(10).In(time.UTC), "a", change.NewSet("title", "a", "A"), "c"), rec("B", at(9), "a", change.NewSet("title", "a", "B"), "c"), `"A"`},
	}
	for _, c := range cases {
		log := []change.Record{create(map[string]any{"title": "a"}), c.a, c.b}
		if got := value(t, change.Resolve(log, opts), "title"); got != c.want {
			t.Errorf("%s: title = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestResolveInvalidRowIsIgnored(t *testing.T) {
	// 手順 0: 検査に合わない行は無いものとし、前の値に戻る。
	log := []change.Record{
		create(map[string]any{"prefix": "HH"}),
		rec("A", at(1), "taro", change.NewSet("prefix", "HH", "WEB"), "c"),
		rec("B", at(2), "taro", change.NewSet("prefix", "WEB", "web!"), "A"),
	}
	o := change.Options{Valid: func(r change.Record) bool { return r.F != "prefix" || string(r.To) != `"web!"` }}
	if got := value(t, change.Resolve(log, o), "prefix"); got != `"WEB"` {
		t.Errorf("prefix = %s, want WEB", got)
	}
}

func TestResolveAddRemovePerValue(t *testing.T) {
	// 一方が Y1 を取り除き、他方が同時に Y1 を足し直した。値ごとの後勝ちで、あとの add が勝つ。
	log := []change.Record{
		create(map[string]any{"systems": []string{"Y1", "Y2"}}),
		rec("A", at(1), "ann", change.NewRemove("systems", "Y1"), "c"),
		rec("B", at(2), "bob", change.NewAdd("systems", "Y1"), "c"),
		rec("C", at(3), "ann", change.NewRemove("systems", "Y2"), "c"),
		rec("D", at(3), "ann", change.NewAdd("systems", "Y3"), "c"),
	}
	if got := list(change.Resolve(log, opts), "systems"); got != `"Y1","Y3"` {
		t.Errorf("systems = %s", got)
	}
}

func TestStamp(t *testing.T) {
	log := []change.Record{create(map[string]any{"title": "a"})}
	n := 0
	newID := func() change.ID { n++; return change.ID(fmt.Sprintf("n%d", n)) }
	meta := change.Meta{Op: "op1", At: at(5), Actor: change.Actor{Kind: change.ActorSync}}
	got := change.Stamp(log, []change.Draft{
		change.NewSet("title", "a", "b"),
		change.NewSet("title", "b", "c"),
		change.NewAdd("systems", "Y1"),
	}, meta, newID)
	want := []struct{ id, prev string }{{"n1", "[c]"}, {"n2", "[n1]"}, {"n3", "[c]"}}
	for i, w := range want {
		r := got[i]
		if string(r.ID) != w.id || fmt.Sprint(r.Prev) != w.prev || r.Op != "op1" || !r.At.Equal(at(5)) || r.By != change.BotLogin || r.Actor.Kind != change.ActorSync {
			t.Errorf("row %d = %+v", i, r)
		}
	}
	created := change.Stamp(nil, []change.Draft{change.NewCreate(map[string]any{"id": "x"})}, meta, newID)
	if len(created[0].Prev) != 0 {
		t.Errorf("create prev = %v", created[0].Prev)
	}
}

func TestStampPrevListsAllTips(t *testing.T) {
	log := []change.Record{
		create(map[string]any{"title": "a"}),
		rec("A", at(1), "ann", change.NewSet("title", "a", "A"), "c"),
		rec("B", at(2), "bob", change.NewSet("title", "a", "B"), "c"),
	}
	got := change.Stamp(log, []change.Draft{change.NewSet("title", "B", "C")}, change.Meta{Op: "o", At: at(3), Actor: change.User("c")}, func() change.ID { return "n" })
	if fmt.Sprint(got[0].Prev) != "[A B]" {
		t.Errorf("prev = %v, want both tips", got[0].Prev)
	}
}

func list(s change.State, f string) string {
	var parts []string
	for _, v := range s.Lists[f] {
		parts = append(parts, string(v))
	}
	return strings.Join(parts, ",")
}
```

`internal/domain/change/fuzz_test.go`:

```go
package change_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// randomLog は、1 つのエンティティのログを乱数で作る。prev は、書いた時点の先端の一部だけを見たことにして、
// 同時の変更(先端が複数)を作る。時刻と by は重なりやすくして、比べ方の全段を通す。
func randomLog(seed uint64) []change.Record {
	rng := rand.New(rand.NewPCG(seed, 0))
	log := []change.Record{create(map[string]any{"a": 0, "b": 0, "systems": []string{"Y1"}})}
	bys := []string{"Ann", "ann", "bob"}
	for i := range rng.IntN(25) {
		var d change.Draft
		switch rng.IntN(4) {
		case 0, 1:
			f := []string{"a", "b"}[rng.IntN(2)]
			d = change.NewSet(f, nil, i)
		case 2:
			d = change.NewAdd("systems", fmt.Sprintf("Y%d", rng.IntN(3)))
		default:
			d = change.NewRemove("systems", fmt.Sprintf("Y%d", rng.IntN(3)))
		}
		r := change.Record{ID: change.ID(fmt.Sprintf("r%03d", i)), At: time.Date(2026, 1, 1, 0, rng.IntN(3), 0, 0, time.UTC),
			By: bys[rng.IntN(len(bys))], Actor: change.User("x"), T: d.T, F: d.F, From: d.From, To: d.To}
		tips := change.Tips(log, r)
		for _, p := range tips {
			if rng.IntN(3) > 0 || len(r.Prev) == 0 {
				r.Prev = append(r.Prev, p)
			}
		}
		log = append(log, r)
	}
	return log
}

// どの順で適用しても、同じ値になる(保存形式「テスト」)。
func FuzzResolveOrderIndependent(f *testing.F) {
	for i := range uint64(20) {
		f.Add(i, i*7+1)
	}
	f.Fuzz(func(t *testing.T, seed, perm uint64) {
		log := randomLog(seed)
		want := change.Resolve(log, opts)
		shuffled := append([]change.Record(nil), log...)
		rand.New(rand.NewPCG(perm, 1)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		if got := change.Resolve(shuffled, opts); !reflect.DeepEqual(got, want) {
			t.Fatalf("order changed the result:\n%+v\n%+v", got, want)
		}
	})
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/change/`
Expected: FAIL(`undefined: change.Resolve` ほか)

- [ ] **Step 3: 実装する**

`internal/domain/change/stamp.go`:

```go
package change

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"
)

// Meta は、1 回の操作で書くすべての行に共通する値。
type Meta struct {
	Op    ID
	At    time.Time
	Actor Actor
}

// Stamp は、drafts を log の続きに書く行にする。prev は、書く時点の同じ項目の先端すべてで、
// 先に並べた drafts も含めて数える。create の prev は空。
func Stamp(log []Record, drafts []Draft, meta Meta, newID func() ID) []Record {
	all := slices.Clone(log)
	out := make([]Record, 0, len(drafts))
	for _, d := range drafts {
		r := Record{
			ID: newID(), Op: meta.Op, At: meta.At, By: meta.Actor.By(), Actor: meta.Actor,
			T: d.T, F: d.F, From: d.From, To: d.To, Root: d.Root,
		}
		if r.T != Create {
			r.Prev = Tips(all, r)
		}
		all = append(all, r)
		out = append(out, r)
	}
	return out
}

// key は、行が属する項目。set は F ごと、add・remove は F と値ごと。
func key(r Record) string {
	switch r.T {
	case Set:
		return "set\x00" + r.F
	case Add:
		return "list\x00" + r.F + "\x00" + canonical(r.To)
	case Remove:
		return "list\x00" + r.F + "\x00" + canonical(r.From)
	}
	return string(r.T) + "\x00" + r.F
}

func canonical(v json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, v); err != nil {
		return string(v)
	}
	return b.String()
}

// group は、項目 k の行と、create の行。
func group(log []Record, k string) []Record {
	var g []Record
	for _, r := range log {
		if r.T == Create || key(r) == k {
			g = append(g, r)
		}
	}
	return g
}

func tipsOf(g []Record) []Record {
	seen := map[ID]bool{}
	for _, r := range g {
		for _, p := range r.Prev {
			seen[p] = true
		}
	}
	var tips []Record
	for _, r := range g {
		if !seen[r.ID] {
			tips = append(tips, r)
		}
	}
	return tips
}

// Tips は、r と同じ項目の、log の中の先端の ID を辞書順で返す。
func Tips(log []Record, r Record) []ID {
	var ids []ID
	for _, t := range tipsOf(group(log, key(r))) {
		ids = append(ids, t.ID)
	}
	slices.Sort(ids)
	return ids
}
```

`internal/domain/change/resolve.go`:

```go
package change

import (
	"encoding/json"
	"slices"
	"strings"
)

// Options は、エンティティの種類ごとの決め方。
type Options struct {
	Multi func(f string) bool // 値を複数持つ項目か。nil なら、どれも 1 つ
	Valid func(r Record) bool // 手順 0 の検査。nil なら、どの行も有効
}

// State は、ログから決めたいまの値。Values は値を 1 つ持つ項目、Lists は複数持つ項目(値の JSON の辞書順)。
type State struct {
	Created *Record
	Values  map[string]json.RawMessage
	Lists   map[string][]json.RawMessage
}

// Resolve は、保存形式「いまの値の決め方」の手順 0〜3 で、項目ごとのいまの値を決める。
// 結果は、log の行の並びによらない。
func Resolve(log []Record, opt Options) State {
	valid := filterValid(log, opt.Valid)
	s := State{Values: map[string]json.RawMessage{}, Lists: map[string][]json.RawMessage{}}
	c := winner(creates(valid))
	if c == nil {
		return s
	}
	s.Created = c
	initial := map[string]json.RawMessage{}
	_ = json.Unmarshal(c.To, &initial) // 形の違う create は、初期値なしとして扱う
	multi := opt.Multi
	if multi == nil {
		multi = func(string) bool { return false }
	}
	for f := range fields(valid, initial) {
		if multi(f) {
			if vs := resolveList(valid, f, initial[f]); len(vs) > 0 {
				s.Lists[f] = vs
			}
		} else if v, ok := resolveValue(valid, f, initial); ok {
			s.Values[f] = v
		}
	}
	return s
}

func filterValid(log []Record, valid func(Record) bool) []Record {
	var out []Record
	for _, r := range log {
		known := r.T == Create || r.T == Set || r.T == Add || r.T == Remove
		if known && (valid == nil || valid(r)) {
			out = append(out, r)
		}
	}
	return out
}

func creates(log []Record) []Record {
	var out []Record
	for _, r := range log {
		if r.T == Create {
			out = append(out, r)
		}
	}
	return out
}

func fields(log []Record, initial map[string]json.RawMessage) map[string]bool {
	fs := map[string]bool{}
	for f := range initial {
		fs[f] = true
	}
	for _, r := range log {
		if r.T != Create {
			fs[r.F] = true
		}
	}
	return fs
}

func resolveValue(log []Record, f string, initial map[string]json.RawMessage) (json.RawMessage, bool) {
	w := winner(tipsOf(group(log, key(Record{T: Set, F: f}))))
	if w.T == Create {
		v, ok := initial[f]
		return v, ok
	}
	return w.To, true
}

func resolveList(log []Record, f string, initial json.RawMessage) []json.RawMessage {
	var start []json.RawMessage
	_ = json.Unmarshal(initial, &start)
	inStart := map[string]bool{}
	cands := map[string]bool{}
	for _, v := range start {
		inStart[canonical(v)] = true
		cands[canonical(v)] = true
	}
	for _, r := range log {
		switch {
		case r.F == f && r.T == Add:
			cands[canonical(r.To)] = true
		case r.F == f && r.T == Remove:
			cands[canonical(r.From)] = true
		}
	}
	var out []json.RawMessage
	for c := range cands {
		w := winner(tipsOf(group(log, "list\x00"+f+"\x00"+c)))
		if w.T == Add || (w.T == Create && inStart[c]) {
			out = append(out, json.RawMessage(c))
		}
	}
	slices.SortFunc(out, func(a, b json.RawMessage) int { return strings.Compare(string(a), string(b)) })
	return out
}

// winner は、先端のうち、手順 3 の比べ方で最もあとの行を返す。
func winner(tips []Record) *Record {
	var w *Record
	for i := range tips {
		if w == nil || later(tips[i], *w) {
			w = &tips[i]
		}
	}
	return w
}

// later は、a が b よりあとか。at(UTC)、by(小文字)、id の順に比べる。
func later(a, b Record) bool {
	if !a.At.Equal(b.At) {
		return a.At.After(b.At)
	}
	if la, lb := strings.ToLower(a.By), strings.ToLower(b.By); la != lb {
		return la > lb
	}
	return a.ID > b.ID
}
```

- [ ] **Step 4: テストとファジングが通ることを確かめる**

Run: `go test ./internal/domain/change/ && go test ./internal/domain/change/ -run '^$' -fuzz FuzzResolveOrderIndependent -fuzztime 30s`
Expected: PASS(ファジングで失敗が見つからない)

- [ ] **Step 5: lint と大きさを確かめる**

Run: `make lint && go test ./internal/archtest/`
Expected: lint 0 件(`gocognit` が 15 を超えたら関数を分ける)、archtest PASS

- [ ] **Step 6: Commit**

```bash
git add internal/domain/change
git commit -m "feat(domain): 変更記録の prev の付け方と、いまの値の決め方を足す"
```

---

### Task 4: domain の中心の型(メンバー・ワークスペース・プロジェクト・アイテム)

各トラックが共通に使うエンティティの型を、土台が読み書きする項目だけで作る。各型は「作成の下書きを作る関数」と「ログからいまの値を作る関数」を持つ。項目はトラックが足す。

**Files:**
- Create: `internal/domain/member/member.go`、`internal/domain/workspace/workspace.go`、`internal/domain/project/project.go`、`internal/domain/item/item.go`
- Test: `internal/domain/member/member_test.go`、`internal/domain/workspace/workspace_test.go`、`internal/domain/project/project_test.go`、`internal/domain/item/item_test.go`

**Interfaces:**
- Consumes: Task 2・3 の `change.Record`・`change.Draft`・`change.Resolve`・`change.ID`
- Produces:
  - `member.Member{Login, Name string}`、`member.ValidLogin(s string) bool`、`member.Init() change.Draft`、`member.Join(login string) change.Draft`、`member.Rename(login, from, to string) change.Draft`、`member.FromLog(log []change.Record) []member.Member`
  - `workspace.ID`、`workspace.Workspace{ID ID; Name, Prefix string}`、`workspace.ValidPrefix(s string) bool`、`workspace.Create(w Workspace) (change.Draft, error)`、`workspace.FromLog(log []change.Record) (Workspace, bool)`
  - `project.ID`、`project.Project{ID ID; Place Place; Title string}`、`project.Place{Workspace workspace.ID; Number int}`、`project.Create(p Project) change.Draft`、`project.FromLog(log) (Project, bool)`、`(Project) DisplayID(prefix string) string`
  - `item.ID`、`item.Kind`(`Epic`・`Story`・`Task`・`Bug`・`Step`)、`item.Item{ID ID; Place Place; Kind Kind; Rank, Title string; Created Stamp}`、`item.Place{Workspace workspace.ID; Project project.ID; Parent *ID; Number int}`、`item.Stamp{At time.Time; By string}`、`item.CanPlace(child Kind, parent *Kind) bool`、`item.Create(it Item) change.Draft`、`item.FromLog(log) (Item, bool)`、`(Item) DisplayID(prefix string) string`

**項目名(log.jsonl の `f` と `create` の `to` のキー):** 保存形式の `item.yaml` の例と同じ英小文字。`place` は親・プロジェクト・ワークスペース・番号をまとめた 1 項目(保存形式「変更記録」の `set` の説明)。メンバーは、一覧への追加を `add members <ログイン名>`、表示名を `set name/<小文字のログイン名>` で持つ(一度載ったメンバーは残り続け、外す操作はない。01)。

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/member/member_test.go`:

```go
package member_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
)

func TestValidLogin(t *testing.T) {
	for s, want := range map[string]bool{
		"taro": true, "Taro_2": true, "a-b": true, "": false, "hedgehug[bot]": false, "ta ro": false,
		"abcdefghijabcdefghijabcdefghijabcdefghi": true, "abcdefghijabcdefghijabcdefghijabcdefghij": false,
	} {
		if member.ValidLogin(s) != want {
			t.Errorf("ValidLogin(%q) != %v", s, want)
		}
	}
}

func TestFromLog(t *testing.T) {
	ts := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	var log []change.Record
	n := 0
	add := func(d change.Draft) {
		n++
		meta := change.Meta{Op: "o", At: ts.Add(time.Duration(n) * time.Millisecond), Actor: change.User("x")}
		id := change.ID(fmt.Sprintf("r%02d", n))
		log = append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return id })...)
	}
	add(member.Init())
	add(member.Join("Taro"))
	add(member.Join("hanako"))
	add(member.Rename("taro", "Taro", "太郎"))
	got := member.FromLog(log)
	want := []member.Member{{Login: "hanako", Name: "hanako"}, {Login: "Taro", Name: "太郎"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("FromLog = %+v, want %+v", got, want)
	}
}
```

`internal/domain/workspace/workspace_test.go`:

```go
package workspace_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestValidPrefix(t *testing.T) {
	for s, want := range map[string]bool{
		"HH": true, "WEB2": true, "A": true, "ABCDEFGHIJ": true,
		"": false, "hh": false, "2WEB": false, "ABCDEFGHIJK": false, "W-B": false,
	} {
		if workspace.ValidPrefix(s) != want {
			t.Errorf("ValidPrefix(%q) != %v", s, want)
		}
	}
}

func TestCreateAndFromLog(t *testing.T) {
	if _, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "hh"}); err == nil {
		t.Error("bad prefix accepted")
	}
	if _, err := workspace.Create(workspace.Workspace{ID: "W", Name: "", Prefix: "HH"}); err == nil {
		t.Error("empty name accepted")
	}
	d, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"})
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	meta := change.Meta{Op: "o", At: ts, Actor: change.User("taro")}
	log := change.Stamp(nil, []change.Draft{d}, meta, func() change.ID { return "c" })
	// GitHub 上で手で書き換えた不正な接頭辞の行は無いものとし、前の値に戻る(保存形式の手順 0)。
	meta.At = ts.Add(time.Second)
	log = append(log, change.Stamp(log, []change.Draft{change.NewSet("prefix", "HH", "h h")}, meta, func() change.ID { return "s" })...)
	w, ok := workspace.FromLog(log)
	if !ok || w != (workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"}) {
		t.Errorf("FromLog = %+v, %v", w, ok)
	}
	if _, ok := workspace.FromLog(nil); ok {
		t.Error("empty log gave a workspace")
	}
}
```

`internal/domain/project/project_test.go`:

```go
package project_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

func TestCreateAndFromLog(t *testing.T) {
	p := project.Project{ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Title: "ログイン"}
	log := change.Stamp(nil, []change.Draft{project.Create(p)}, change.Meta{Op: "o", At: time.Unix(0, 0), Actor: change.User("taro")}, func() change.ID { return "c" })
	got, ok := project.FromLog(log)
	if !ok || got != p {
		t.Errorf("FromLog = %+v, %v", got, ok)
	}
	if got.DisplayID("HH") != "HH-P1" {
		t.Errorf("DisplayID = %s", got.DisplayID("HH"))
	}
}
```

`internal/domain/item/item_test.go`:

```go
package item_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

func kind(k item.Kind) *item.Kind { return &k }

// 01「用語と階層」の親子の表。
func TestCanPlace(t *testing.T) {
	cases := []struct {
		child  item.Kind
		parent *item.Kind
		want   bool
	}{
		{item.Epic, nil, true},
		{item.Epic, kind(item.Epic), false},
		{item.Story, kind(item.Epic), true},
		{item.Task, kind(item.Epic), true},
		{item.Bug, kind(item.Epic), true},
		{item.Task, nil, false},
		{item.Task, kind(item.Story), false},
		{item.Step, kind(item.Story), true},
		{item.Step, kind(item.Task), true},
		{item.Step, kind(item.Bug), true},
		{item.Step, kind(item.Epic), false},
		{item.Step, kind(item.Step), false},
		{item.Kind("Saga"), nil, false},
	}
	for _, c := range cases {
		if got := item.CanPlace(c.child, c.parent); got != c.want {
			t.Errorf("CanPlace(%s, %v) = %v", c.child, c.parent, got)
		}
	}
}

func TestCreateAndFromLog(t *testing.T) {
	parent := item.ID("E")
	it := item.Item{
		ID:    "T",
		Place: item.Place{Workspace: "W", Project: "P", Parent: &parent, Number: 12},
		Kind:  item.Task, Rank: "a0V", Title: "ログイン画面を作る",
	}
	ts := time.Date(2026, 11, 10, 9, 0, 0, 0, time.FixedZone("JST", 9*3600))
	log := change.Stamp(nil, []change.Draft{item.Create(it)}, change.Meta{Op: "o", At: ts, Actor: change.User("hanako")}, func() change.ID { return "c" })
	log = append(log, change.Stamp(log, []change.Draft{change.NewSet("title", it.Title, "ログイン画面")}, change.Meta{Op: "o2", At: ts.Add(time.Hour), Actor: change.User("taro")}, func() change.ID { return "s" })...)
	got, ok := item.FromLog(log)
	if !ok {
		t.Fatal("no item")
	}
	if got.Title != "ログイン画面" || got.Kind != item.Task || *got.Place.Parent != "E" || got.Place.Number != 12 {
		t.Errorf("FromLog = %+v", got)
	}
	if got.Created.By != "hanako" || !got.Created.At.Equal(ts) {
		t.Errorf("Created = %+v", got.Created)
	}
	if got.DisplayID("HH") != "HH-12" {
		t.Errorf("DisplayID = %s", got.DisplayID("HH"))
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/...`
Expected: FAIL(`undefined: member.ValidLogin` ほか)

- [ ] **Step 3: 実装する**

`internal/domain/member/member.go`:

```go
// Package member は、Store のメンバー(担当者・オーナー・メンションの候補)を持つ。
package member

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Member は、メンバーの 1 人。ログイン名は大文字と小文字を区別せずに比べる(01)。
type Member struct {
	Login string
	Name  string // 表示名。初期値はログイン名
}

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,39}$`)

// ValidLogin は、ログイン名に使えるか(英数字・ハイフン・アンダースコア、39 文字以内。07・08)。
func ValidLogin(s string) bool { return loginPattern.MatchString(s) }

// Init は、メンバーの一覧のログを始める作成の変更。Store を作った最初の保存で書く。
func Init() change.Draft { return change.NewCreate(map[string]any{"members": []string{}}) }

// Join は、メンバーの一覧への追加。
func Join(login string) change.Draft { return change.NewAdd("members", login) }

// Rename は、表示名の変更。
func Rename(login, from, to string) change.Draft {
	return change.NewSet("name/"+strings.ToLower(login), from, to)
}

var opts = change.Options{
	Multi: func(f string) bool { return f == "members" },
	Valid: func(r change.Record) bool {
		if r.F != "members" || r.T != change.Add {
			return true
		}
		var login string
		return json.Unmarshal(r.To, &login) == nil && ValidLogin(login)
	},
}

// FromLog は、メンバーの一覧のログから、ログイン名(小文字)の順の一覧を作る。
func FromLog(log []change.Record) []Member {
	s := change.Resolve(log, opts)
	var out []Member
	for _, raw := range s.Lists["members"] {
		var login string
		if json.Unmarshal(raw, &login) != nil {
			continue
		}
		m := Member{Login: login, Name: login}
		if v, ok := s.Values["name/"+strings.ToLower(login)]; ok {
			_ = json.Unmarshal(v, &m.Name)
		}
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b Member) int {
		return strings.Compare(strings.ToLower(a.Login), strings.ToLower(b.Login))
	})
	return out
}
```

`internal/domain/workspace/workspace.go`:

```go
// Package workspace は、ワークスペース(接頭辞を持ち、プロパティ・ステータス・ワークフローなどを持つ単位)を持つ。
package workspace

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// ID はワークスペースの内部 ID。
type ID = change.ID

// Workspace は、土台が扱う項目だけを持つ。プロパティ・ワークフローなどはトラック B が足す。
type Workspace struct {
	ID     ID
	Name   string
	Prefix string
}

var prefixPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,9}$`)

// ValidPrefix は、接頭辞に使えるか(英大文字で始まる 10 文字以内の英大文字と数字。01)。
func ValidPrefix(s string) bool { return prefixPattern.MatchString(s) }

// Create は、ワークスペースの作成の変更。名前が空、または接頭辞が使えなければ断る。
func Create(w Workspace) (change.Draft, error) {
	if strings.TrimSpace(w.Name) == "" {
		return change.Draft{}, errors.New("workspace name is empty")
	}
	if !ValidPrefix(w.Prefix) {
		return change.Draft{}, errors.New("workspace prefix is not valid")
	}
	return change.NewCreate(map[string]any{"id": w.ID, "name": w.Name, "prefix": w.Prefix}), nil
}

var opts = change.Options{Valid: func(r change.Record) bool {
	switch {
	case r.T == change.Set && r.F == "prefix":
		var p string
		return json.Unmarshal(r.To, &p) == nil && ValidPrefix(p)
	case r.T == change.Create:
		var v struct{ Prefix string }
		return json.Unmarshal(r.To, &v) == nil && ValidPrefix(v.Prefix)
	}
	return true
}}

// FromLog は、ワークスペースのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Workspace, bool) {
	s := change.Resolve(log, opts)
	if s.Created == nil {
		return Workspace{}, false
	}
	var w Workspace
	_ = json.Unmarshal(s.Values["id"], &w.ID)
	_ = json.Unmarshal(s.Values["name"], &w.Name)
	_ = json.Unmarshal(s.Values["prefix"], &w.Prefix)
	return w, true
}
```

`internal/domain/project/project.go`:

```go
// Package project は、プロジェクト(Epic をまとめる単位)を持つ。
package project

import (
	"encoding/json"
	"strconv"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ID はプロジェクトの内部 ID。
type ID = change.ID

// Place は、プロジェクトの場所(ワークスペースと番号)。1 つの項目として変える。
type Place struct {
	Workspace workspace.ID `json:"workspace"`
	Number    int          `json:"number"`
}

// Project は、土台が扱う項目だけを持つ。オーナー・日付・ステータス・説明はトラック A が足す。
type Project struct {
	ID    ID
	Place Place
	Title string
}

// DisplayID は、画面の ID(HH-P1)。
func (p Project) DisplayID(prefix string) string {
	return prefix + "-P" + strconv.Itoa(p.Place.Number)
}

// Create は、プロジェクトの作成の変更。
func Create(p Project) change.Draft {
	return change.NewCreate(map[string]any{"id": p.ID, "place": p.Place, "title": p.Title})
}

// FromLog は、プロジェクトのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Project, bool) {
	s := change.Resolve(log, change.Options{})
	if s.Created == nil {
		return Project{}, false
	}
	var p Project
	_ = json.Unmarshal(s.Values["id"], &p.ID)
	_ = json.Unmarshal(s.Values["place"], &p.Place)
	_ = json.Unmarshal(s.Values["title"], &p.Title)
	return p, true
}
```

`internal/domain/item/item.go`:

```go
// Package item は、アイテム(Epic・Story・Task・Bug・Step)を持つ。
package item

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ID はアイテムの内部 ID。
type ID = change.ID

// Kind はアイテムの種類。
type Kind string

const (
	Epic  Kind = "Epic"
	Story Kind = "Story"
	Task  Kind = "Task"
	Bug   Kind = "Bug"
	Step  Kind = "Step"
)

// CanPlace は、child の種類を parent の種類の下に置けるか(01「用語と階層」の表)。
// parent が nil なら、プロジェクトの直下。
func CanPlace(child Kind, parent *Kind) bool {
	switch child {
	case Epic:
		return parent == nil
	case Story, Task, Bug:
		return parent != nil && *parent == Epic
	case Step:
		return parent != nil && (*parent == Story || *parent == Task || *parent == Bug)
	}
	return false
}

// Place は、アイテムの場所(ワークスペース・プロジェクト・親・番号)。1 つの項目として変える。
type Place struct {
	Workspace workspace.ID `json:"workspace"`
	Project   project.ID   `json:"project"`
	Parent    *ID          `json:"parent"`
	Number    int          `json:"number"`
}

// Stamp は、作成の時刻と、作成した人。
type Stamp struct {
	At time.Time
	By string
}

// Item は、土台が扱う項目だけを持つ。ステータス・担当者などはトラック A・B が足す。
type Item struct {
	ID      ID
	Place   Place
	Kind    Kind
	Rank    string
	Title   string
	Created Stamp
}

// DisplayID は、画面の ID(HH-12)。
func (it Item) DisplayID(prefix string) string {
	return prefix + "-" + strconv.Itoa(it.Place.Number)
}

// Create は、アイテムの作成の変更。作成の時刻と人は、変更記録の at と by から取る。
func Create(it Item) change.Draft {
	return change.NewCreate(map[string]any{
		"id": it.ID, "place": it.Place, "kind": it.Kind, "rank": it.Rank, "title": it.Title,
	})
}

// FromLog は、アイテムのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Item, bool) {
	s := change.Resolve(log, change.Options{})
	if s.Created == nil {
		return Item{}, false
	}
	it := Item{Created: Stamp{At: s.Created.At, By: s.Created.By}}
	_ = json.Unmarshal(s.Values["id"], &it.ID)
	_ = json.Unmarshal(s.Values["place"], &it.Place)
	_ = json.Unmarshal(s.Values["kind"], &it.Kind)
	_ = json.Unmarshal(s.Values["rank"], &it.Rank)
	_ = json.Unmarshal(s.Values["title"], &it.Title)
	return it, true
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/... && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件

- [ ] **Step 5: Commit**

```bash
git add internal/domain
git commit -m "feat(domain): メンバー・ワークスペース・プロジェクト・アイテムの中心の型を足す"
```

---

### Task 5: 保存の口(app/port)と、メモリ上の偽物・照合テスト(app/apptest)

設計書 3 節の `Stores` を定義し、メモリ上の偽物と、偽物と本物の両方に流す照合テストを作る。各トラックの usecase のテストは、この偽物だけで動く。

**Files:**
- Create: `internal/app/port/stores.go`、`internal/app/port/clock.go`
- Create: `internal/app/apptest/clock.go`、`internal/app/apptest/stores.go`、`internal/app/apptest/stores_read.go`、`internal/app/apptest/stores_contract.go`、`internal/app/apptest/stores_contract_cases.go`、`internal/app/apptest/sample.go`
- Test: `internal/app/apptest/stores_test.go`

**Interfaces:**
- Consumes: Task 2〜4 の `change`・`member`・`workspace`・`project`・`item`
- Produces:
  - `port.StoreID`、`port.ErrNotFound`、`port.ErrStoreNotFound`
  - `port.Stores`(`Read`・`Write`)、`port.Operation{Actor change.Actor; Summary string}`、`port.Reader`、`port.Tx`
  - `port.WorkspaceReader{Get; List}`、`port.ProjectReader{Get}`、`port.ItemReader{Get}`、`port.MemberReader{List}`、`port.ChangeReader{Of(ref change.Ref) ([]change.Record, error)}`
  - `port.WorkspaceWriter`・`port.ProjectWriter`・`port.ItemWriter` は `Append(id, drafts ...change.Draft) error`、`port.MemberWriter` は `Append(drafts ...change.Draft) error`
  - `port.Clock{Now() time.Time}`、`port.IDs{New() change.ID}`
  - `apptest.NewClock(start time.Time) *apptest.Clock`、`apptest.NewIDs() *apptest.IDs`
  - `apptest.NewStores(clock port.Clock, ids port.IDs) *apptest.Stores`、`(*Stores) AddStore(id port.StoreID)`
  - `apptest.RunStoresContract(t *testing.T, newStores func(t *testing.T) (port.Stores, port.StoreID))`
  - `apptest.Sample(ctx context.Context, s port.Stores, id port.StoreID) error`(サンプルデータ。e2e とトラックのテストが使う)

**口の約束(照合テストで確かめる):**
- `Write` の `fn` がエラーを返したら何も残さず、そのエラーをそのまま返す。
- 1 回の `Write` で書いた行は、同じ `Op`・同じ `At`・`Operation.Actor` を持つ。`prev` は `change.Stamp` で付ける。
- `Tx` の読み取りは、同じ `Tx` で書いたものを含む。
- 同じ Store への `Write` は 1 つずつ順に行う(あとの `Write` の行の `prev` は、前の `Write` の行を指す)。
- 見つからないエンティティは `port.ErrNotFound`、知らない Store は `port.ErrStoreNotFound`。
- `fn` の中から同じ `Stores` の `Read`・`Write` を呼んではいけない(呼んだときの動きは決めない)。

- [ ] **Step 1: 口を書く**

`internal/app/port/stores.go`:

```go
// Package port は、usecase が外に触るための口を決める。口は「何をしたいか」の言葉で決め、
// git の言葉(コミット、ブランチ、clone)を使わない(設計書 3 節)。
// 口は 1 つ 1 ファイルに置く。新しい口は、使うトラックが足す。
package port

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// StoreID は Store ID(.hedgehug/store.yaml の store)。
type StoreID string

var (
	ErrNotFound      = errors.New("not found")
	ErrStoreNotFound = errors.New("store not found")
)

// Stores は Store のデータの口。1 回の操作 = 1 回の Write。
type Stores interface {
	Read(ctx context.Context, storeID StoreID, fn func(r Reader) error) error
	// Write は fn がエラーなら何も残さない。同じ Store への Write は 1 つずつ順に行う。
	Write(ctx context.Context, storeID StoreID, op Operation, fn func(tx Tx) error) error
}

// Operation は 1 回の操作。
type Operation struct {
	Actor   change.Actor // あなた・自動化・同期・GitHub
	Summary string       // アクティビティの 1 行
}

// Reader は Store の読み取り。種類ごとの関数は、各トラックが使うものだけを足す。
type Reader interface {
	Workspaces() WorkspaceReader
	Projects() ProjectReader
	Items() ItemReader
	Members() MemberReader
	Changes() ChangeReader // 履歴とアクティビティ
}

// Tx は 1 回の Write の中の読み書き。読み取りは、同じ Tx で書いたものを含む。
type Tx interface {
	Reader
	WriteWorkspaces() WorkspaceWriter
	WriteProjects() ProjectWriter
	WriteItems() ItemWriter
	WriteMembers() MemberWriter
}

type WorkspaceReader interface {
	Get(id workspace.ID) (workspace.Workspace, error)
	List() ([]workspace.Workspace, error) // 内部 ID の順
}

type ProjectReader interface {
	Get(id project.ID) (project.Project, error)
}

type ItemReader interface {
	Get(id item.ID) (item.Item, error)
}

type MemberReader interface {
	List() ([]member.Member, error)
}

type ChangeReader interface {
	// Of は、エンティティのログを、書いた順に返す。
	Of(ref change.Ref) ([]change.Record, error)
}

type WorkspaceWriter interface {
	Append(id workspace.ID, drafts ...change.Draft) error
}

type ProjectWriter interface {
	Append(id project.ID, drafts ...change.Draft) error
}

type ItemWriter interface {
	Append(id item.ID, drafts ...change.Draft) error
}

type MemberWriter interface {
	Append(drafts ...change.Draft) error
}
```

`internal/app/port/clock.go`:

```go
package port

import (
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Clock は現在時刻(このマシンの時計。補正しない)。
type Clock interface {
	Now() time.Time
}

// IDs は内部 ID(ULID)の採番。
type IDs interface {
	New() change.ID
}
```

- [ ] **Step 2: 偽の時計と採番を書く**

`internal/app/apptest/clock.go`:

```go
// Package apptest は、口のメモリ上の偽物と、口ごとの照合テストを持つ。どのテストからも import してよい。
package apptest

import (
	"fmt"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Clock は、呼ぶたびに Step だけ進む時計。
type Clock struct {
	mu   sync.Mutex
	now  time.Time
	Step time.Duration
}

func NewClock(start time.Time) *Clock { return &Clock{now: start, Step: time.Millisecond} }

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.Step)
	return t
}

// IDs は、辞書順に並ぶ 26 文字の ID を順に返す。
type IDs struct {
	mu sync.Mutex
	n  int
}

func NewIDs() *IDs { return &IDs{} }

func (g *IDs) New() change.ID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return change.ID(fmt.Sprintf("01TEST%020d", g.n))
}
```

- [ ] **Step 3: 照合テストを書く**

`internal/app/apptest/stores_contract.go`:

```go
package apptest

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// RunStoresContract は、port.Stores が満たすべき振る舞いを確かめる。偽物と、本物の実装(infra/gitstore)の
// 両方のテストから呼ぶ。newStores は、空の Store を 1 つだけ持つ Stores と、その Store の ID を返す。
func RunStoresContract(t *testing.T, newStores func(t *testing.T) (port.Stores, port.StoreID)) {
	cases := []struct {
		name string
		fn   func(t *testing.T, s port.Stores, id port.StoreID)
	}{
		{"write then read", contractWriteRead},
		{"records are stamped", contractStamped},
		{"failed write leaves nothing", contractFailedWrite},
		{"tx sees its own writes", contractTxSeesOwn},
		{"not found", contractNotFound},
		{"writes are serialized", contractSerialized},
		{"workspaces are listed in id order", contractList},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, id := newStores(t)
			c.fn(t, s, id)
		})
	}
}

var ctx = context.Background()

// 照合テストの準備のデータ。
var (
	cWorkspace = workspace.Workspace{ID: "01CONTRACTWS00000000000001", Name: "Web", Prefix: "HH"}
	cProject   = project.Project{ID: "01CONTRACTPR00000000000001", Place: project.Place{Workspace: cWorkspace.ID, Number: 1}, Title: "ログイン"}
	cItem      = item.Item{ID: "01CONTRACTIT00000000000001", Place: item.Place{Workspace: cWorkspace.ID, Project: cProject.ID, Number: 1}, Kind: item.Epic, Rank: "a", Title: "認証"}
	taro       = port.Operation{Actor: change.User("taro"), Summary: "準備"}
)

func writeFixture(tx port.Tx) error {
	d, err := workspace.Create(cWorkspace)
	if err != nil {
		return err
	}
	return errors.Join(
		tx.WriteMembers().Append(member.Init(), member.Join("taro")),
		tx.WriteWorkspaces().Append(cWorkspace.ID, d),
		tx.WriteProjects().Append(cProject.ID, project.Create(cProject)),
		tx.WriteItems().Append(cItem.ID, item.Create(cItem)),
	)
}

func mustWrite(t *testing.T, s port.Stores, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) {
	t.Helper()
	if err := s.Write(ctx, id, op, fn); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func mustRead(t *testing.T, s port.Stores, id port.StoreID, fn func(r port.Reader) error) {
	t.Helper()
	if err := s.Read(ctx, id, fn); err != nil {
		t.Fatalf("Read: %v", err)
	}
}

func sameItem(a, b item.Item) bool {
	a.Created, b.Created = item.Stamp{}, item.Stamp{}
	pa, pb := a.Place.Parent, b.Place.Parent
	a.Place.Parent, b.Place.Parent = nil, nil
	return a == b && (pa == nil) == (pb == nil) && (pa == nil || *pa == *pb)
}

func setTitle(id item.ID, from, to string) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewSet("title", from, to))
	}
}
```

`internal/app/apptest/stores_contract_cases.go`:

```go
package apptest

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func contractWriteRead(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		w, err := r.Workspaces().Get(cWorkspace.ID)
		if err != nil || w != cWorkspace {
			t.Errorf("workspace = %+v, %v", w, err)
		}
		p, err := r.Projects().Get(cProject.ID)
		if err != nil || p != cProject {
			t.Errorf("project = %+v, %v", p, err)
		}
		it, err := r.Items().Get(cItem.ID)
		if err != nil || !sameItem(it, cItem) || it.Created.By != "taro" || it.Created.At.IsZero() {
			t.Errorf("item = %+v, %v", it, err)
		}
		ms, err := r.Members().List()
		if err != nil || !slices.Equal(ms, []member.Member{{Login: "taro", Name: "taro"}}) {
			t.Errorf("members = %+v, %v", ms, err)
		}
		return nil
	})
}

func contractStamped(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "修復"}, setTitle(cItem.ID, "認証", "ログイン"))
	mustRead(t, s, id, func(r port.Reader) error {
		ws, _ := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || len(log) != 2 || len(ws) != 1 {
			t.Fatalf("logs = %+v / %+v, %v", ws, log, err)
		}
		c, set := log[0], log[1]
		if c.T != change.Create || len(c.Prev) != 0 || c.By != "taro" || c.Actor != change.User("taro") {
			t.Errorf("create = %+v", c)
		}
		if c.Op != ws[0].Op || !c.At.Equal(ws[0].At) {
			t.Errorf("rows of one write differ: %+v / %+v", c, ws[0])
		}
		if set.Op == c.Op || !slices.Equal(set.Prev, []change.ID{c.ID}) || set.By != change.BotLogin || set.Actor.Kind != change.ActorSync {
			t.Errorf("set = %+v", set)
		}
		if it, _ := r.Items().Get(cItem.ID); it.Title != "ログイン" {
			t.Errorf("title = %q", it.Title)
		}
		return nil
	})
}

func contractFailedWrite(t *testing.T, s port.Stores, id port.StoreID) {
	boom := errors.New("boom")
	err := s.Write(ctx, id, taro, func(tx port.Tx) error {
		if err := writeFixture(tx); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write error = %v, want boom", err)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		if _, err := r.Items().Get(cItem.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("item after failed write: %v", err)
		}
		if ms, _ := r.Members().List(); len(ms) != 0 {
			t.Errorf("members after failed write: %+v", ms)
		}
		return nil
	})
}

func contractTxSeesOwn(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := writeFixture(tx); err != nil {
			return err
		}
		if err := tx.WriteItems().Append(cItem.ID, change.NewSet("title", "認証", "二段階認証")); err != nil {
			return err
		}
		it, err := tx.Items().Get(cItem.ID)
		if err != nil || it.Title != "二段階認証" {
			t.Errorf("item in tx = %+v, %v", it, err)
		}
		return nil
	})
}

func contractNotFound(t *testing.T, s port.Stores, id port.StoreID) {
	mustRead(t, s, id, func(r port.Reader) error {
		if _, err := r.Workspaces().Get("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("workspace: %v", err)
		}
		if _, err := r.Items().Get("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("item: %v", err)
		}
		if _, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: "01NOPE00000000000000000000"}); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("changes: %v", err)
		}
		return nil
	})
	if err := s.Read(ctx, "no-such-store", func(port.Reader) error { return nil }); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("Read of unknown store: %v", err)
	}
	if err := s.Write(ctx, "no-such-store", taro, func(port.Tx) error { return nil }); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("Write to unknown store: %v", err)
	}
}

func contractSerialized(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			if err := s.Write(ctx, id, taro, setTitle(cItem.ID, "", fmt.Sprint(i))); err != nil {
				t.Errorf("Write %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	mustRead(t, s, id, func(r port.Reader) error {
		log, _ := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		prevs := map[change.ID]bool{}
		for _, rec := range log[1:] {
			if len(rec.Prev) != 1 {
				t.Errorf("concurrent tips: %+v", rec)
			}
			prevs[rec.Prev[0]] = true
		}
		if len(log) != 11 || len(prevs) != 10 {
			t.Errorf("writes were not serialized: %d rows, %d distinct prev", len(log), len(prevs))
		}
		return nil
	})
}

func contractList(t *testing.T, s port.Stores, id port.StoreID) {
	second := workspace.Workspace{ID: "01CONTRACTWS00000000000000", Name: "API", Prefix: "API"}
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		d, err := workspace.Create(second)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(second.ID, d)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		ws, err := r.Workspaces().List()
		if err != nil || !slices.Equal(ws, []workspace.Workspace{second, cWorkspace}) {
			t.Errorf("List = %+v, %v", ws, err)
		}
		return nil
	})
}
```

`internal/app/apptest/stores_test.go`:

```go
package apptest_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestFakeStores(t *testing.T) {
	apptest.RunStoresContract(t, func(t *testing.T) (port.Stores, port.StoreID) {
		s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
		s.AddStore("S1")
		return s, "S1"
	})
}

func TestSample(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		ws, err := r.Workspaces().List()
		if err != nil || len(ws) != 1 || ws[0].Prefix != "HH" {
			t.Errorf("workspaces = %+v, %v", ws, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 4: テストが落ちることを確かめる**

Run: `go test ./internal/app/...`
Expected: FAIL(`undefined: apptest.NewStores` ほか)

- [ ] **Step 5: 偽物を書く**

`internal/app/apptest/stores.go`:

```go
package apptest

import (
	"context"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Stores は port.Stores のメモリ上の偽物。Read と Write は 1 つずつ順に行う。
type Stores struct {
	mu     sync.Mutex
	clock  port.Clock
	ids    port.IDs
	stores map[port.StoreID]map[change.Ref][]change.Record
}

func NewStores(clock port.Clock, ids port.IDs) *Stores {
	return &Stores{clock: clock, ids: ids, stores: map[port.StoreID]map[change.Ref][]change.Record{}}
}

// AddStore は、空の Store を足す。
func (s *Stores) AddStore(id port.StoreID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stores[id] = map[change.Ref][]change.Record{}
}

func (s *Stores) Read(_ context.Context, id port.StoreID, fn func(r port.Reader) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, ok := s.stores[id]
	if !ok {
		return port.ErrStoreNotFound
	}
	return fn(&view{base: logs})
}

func (s *Stores) Write(_ context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, ok := s.stores[id]
	if !ok {
		return port.ErrStoreNotFound
	}
	t := &tx{
		view: view{base: logs, pending: map[change.Ref][]change.Record{}},
		meta: change.Meta{Op: s.ids.New(), At: s.clock.Now(), Actor: op.Actor},
		ids:  s.ids,
	}
	if err := fn(t); err != nil {
		return err
	}
	for ref, recs := range t.pending {
		logs[ref] = append(logs[ref], recs...)
	}
	return nil
}

type tx struct {
	view
	meta change.Meta
	ids  port.IDs
}

func (t *tx) append(ref change.Ref, drafts []change.Draft) error {
	recs := change.Stamp(t.log(ref), drafts, t.meta, t.ids.New)
	t.pending[ref] = append(t.pending[ref], recs...)
	return nil
}

func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
func (t *tx) WriteProjects() port.ProjectWriter     { return idWriter{t, change.KindProject} }
func (t *tx) WriteItems() port.ItemWriter           { return idWriter{t, change.KindItem} }
func (t *tx) WriteMembers() port.MemberWriter       { return membersWriter{t} }

type idWriter struct {
	t    *tx
	kind change.Kind
}

func (w idWriter) Append(id change.ID, drafts ...change.Draft) error {
	return w.t.append(change.Ref{Kind: w.kind, ID: id}, drafts)
}

type membersWriter struct{ t *tx }

func (w membersWriter) Append(drafts ...change.Draft) error {
	return w.t.append(change.Ref{Kind: change.KindMembers}, drafts)
}
```

`internal/app/apptest/stores_read.go`:

```go
package apptest

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// view は、保存済みのログ(base)と、同じ Tx で書いたログ(pending)を合わせて読む。
type view struct {
	base    map[change.Ref][]change.Record
	pending map[change.Ref][]change.Record
}

func (v *view) log(ref change.Ref) []change.Record {
	return append(slices.Clone(v.base[ref]), v.pending[ref]...)
}

func (v *view) ids(kind change.Kind) []change.ID {
	seen := map[change.ID]bool{}
	for _, m := range []map[change.Ref][]change.Record{v.base, v.pending} {
		for ref := range m {
			if ref.Kind == kind {
				seen[ref.ID] = true
			}
		}
	}
	out := make([]change.ID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.SortFunc(out, func(a, b change.ID) int { return strings.Compare(string(a), string(b)) })
	return out
}

func (v *view) Workspaces() port.WorkspaceReader { return workspaces{v} }
func (v *view) Projects() port.ProjectReader     { return projects{v} }
func (v *view) Items() port.ItemReader           { return items{v} }
func (v *view) Members() port.MemberReader       { return members{v} }
func (v *view) Changes() port.ChangeReader       { return changes{v} }

type workspaces struct{ v *view }

func (r workspaces) Get(id workspace.ID) (workspace.Workspace, error) {
	w, ok := workspace.FromLog(r.v.log(change.Ref{Kind: change.KindWorkspace, ID: id}))
	if !ok {
		return workspace.Workspace{}, port.ErrNotFound
	}
	return w, nil
}

func (r workspaces) List() ([]workspace.Workspace, error) {
	var out []workspace.Workspace
	for _, id := range r.v.ids(change.KindWorkspace) {
		if w, err := r.Get(id); err == nil {
			out = append(out, w)
		}
	}
	return out, nil
}

type projects struct{ v *view }

func (r projects) Get(id project.ID) (project.Project, error) {
	p, ok := project.FromLog(r.v.log(change.Ref{Kind: change.KindProject, ID: id}))
	if !ok {
		return project.Project{}, port.ErrNotFound
	}
	return p, nil
}

type items struct{ v *view }

func (r items) Get(id item.ID) (item.Item, error) {
	it, ok := item.FromLog(r.v.log(change.Ref{Kind: change.KindItem, ID: id}))
	if !ok {
		return item.Item{}, port.ErrNotFound
	}
	return it, nil
}

type members struct{ v *view }

func (r members) List() ([]member.Member, error) {
	return member.FromLog(r.v.log(change.Ref{Kind: change.KindMembers})), nil
}

type changes struct{ v *view }

func (r changes) Of(ref change.Ref) ([]change.Record, error) {
	log := r.v.log(ref)
	if len(log) == 0 {
		return nil, port.ErrNotFound
	}
	return log, nil
}
```

`internal/app/apptest/sample.go`:

```go
package apptest

import (
	"context"
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// サンプルデータの内部 ID。トラックのテストと e2e が参照してよい。
const (
	SampleWorkspace workspace.ID = "01SAMPLEWS0000000000000001"
	SampleProject   project.ID   = "01SAMPLEPR0000000000000001"
	SampleEpic      item.ID      = "01SAMPLEIT0000000000000001"
	SampleTask      item.ID      = "01SAMPLEIT0000000000000002"
)

// Sample は、メンバー 2 人(taro・hanako)、ワークスペース「Web」(HH)、プロジェクト HH-P1、
// Epic HH-1 とその下の Task HH-2 を、1 回の保存で書く。
func Sample(ctx context.Context, s port.Stores, id port.StoreID) error {
	ws := workspace.Workspace{ID: SampleWorkspace, Name: "Web", Prefix: "HH"}
	pr := project.Project{ID: SampleProject, Place: project.Place{Workspace: ws.ID, Number: 1}, Title: "ログイン"}
	epic := item.Item{ID: SampleEpic, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Number: 1}, Kind: item.Epic, Rank: "a", Title: "認証"}
	parent := epic.ID
	task := item.Item{ID: SampleTask, Place: item.Place{Workspace: ws.ID, Project: pr.ID, Parent: &parent, Number: 2}, Kind: item.Task, Rank: "a", Title: "ログイン画面を作る"}
	op := port.Operation{Actor: change.User("taro"), Summary: "サンプルデータを作成"}
	return s.Write(ctx, id, op, func(tx port.Tx) error {
		d, err := workspace.Create(ws)
		if err != nil {
			return err
		}
		return errors.Join(
			tx.WriteMembers().Append(member.Init(), member.Join("taro"), member.Join("hanako")),
			tx.WriteWorkspaces().Append(ws.ID, d),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
			tx.WriteItems().Append(epic.ID, item.Create(epic)),
			tx.WriteItems().Append(task.ID, item.Create(task)),
		)
	})
}
```

- [ ] **Step 6: テストが通ることを確かめる**

Run: `go test -race ./internal/app/... && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS(apptest が domain と port だけを import している)

- [ ] **Step 7: Commit**

```bash
git add internal/app
git commit -m "feat(app): 保存の口と、メモリ上の偽物・照合テスト・サンプルデータを足す"
```

---

### Task 6: 時計と採番(sysclock)、Store の作成と取り付け(gitstore)

本物の時計と ULID の採番を作る。gitstore に、Store を作る(`store.yaml` の最初のコミット)・開く・ファイルロックをかける部分を作る。

**Files:**
- Create: `internal/infra/sysclock/sysclock.go`
- Test: `internal/infra/sysclock/sysclock_test.go`
- Create: `internal/infra/gitstore/create.go`、`internal/infra/gitstore/stores.go`、`internal/infra/gitstore/lock.go`
- Test: `internal/infra/gitstore/create_test.go`

**Interfaces:**
- Consumes: Task 5 の `port.Clock`・`port.IDs`・`port.StoreID`・`port.ErrStoreNotFound`、Task 2 の `change.BotLogin`
- Produces:
  - `sysclock.Clock{}`(`port.Clock`)、`sysclock.IDs{}`(`port.IDs`)
  - `gitstore.Create(dir string, id port.StoreID, at time.Time) error` — 空のフォルダ(またはないフォルダ)に Store を作る
  - `gitstore.New(clock port.Clock, ids port.IDs) *gitstore.Stores`
  - `(*Stores) Attach(dir string) (port.StoreID, error)` — Store を開いて使えるようにする。`ErrNotStore`・`ErrNewerFormat`
  - `gitstore.Format = 1`
  - (パッケージ内)`(*Stores) dir(id port.StoreID) (string, error)`、`lock(root string) (unlock func(), err error)`

- [ ] **Step 1: 依存を入れる**

```bash
go get github.com/go-git/go-git/v5@v5.19.3 go.yaml.in/yaml/v4@v4.0.0-rc.6 golang.org/x/sys@v0.49.0 github.com/oklog/ulid/v2@v2.1.2
```

(前回の go.mod と同じ版。)

- [ ] **Step 2: 失敗するテストを書く**

`internal/infra/sysclock/sysclock_test.go`:

```go
package sysclock_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/infra/sysclock"
)

func TestIDsAreSortedULIDs(t *testing.T) {
	var ids sysclock.IDs
	prev := ids.New()
	for range 1000 {
		id := ids.New()
		if len(id) != 26 || id <= prev {
			t.Fatalf("id %q after %q", id, prev)
		}
		prev = id
	}
}

func TestClockHasLocalZone(t *testing.T) {
	if got := (sysclock.Clock{}).Now().Location(); got.String() != "Local" {
		t.Errorf("location = %s", got)
	}
}
```

`internal/infra/gitstore/create_test.go`:

```go
package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

var t0 = time.Date(2026, 11, 12, 9, 0, 0, 0, time.FixedZone("JST", 9*3600))

func newStore(t *testing.T) (*Stores, port.StoreID, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "store")
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	s := New(apptest.NewClock(t0), apptest.NewIDs())
	id, err := s.Attach(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, id, dir
}

func TestCreateWritesStoreYAMLInOneCommit(t *testing.T) {
	_, id, dir := newStore(t)
	if id != "01STORE0000000000000000001" {
		t.Errorf("id = %s", id)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".hedgehug", "store.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := "store: 01STORE0000000000000000001\nformat: 1\nretention_days: 30\n"
	if string(b) != want {
		t.Errorf("store.yaml =\n%s\nwant\n%s", b, want)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	// 最初のコミットは保存ではないので Hedgehug-Op を持たない(保存形式「.hedgehug/store.yaml」)。
	if head.Name() != "refs/heads/main" {
		t.Errorf("branch = %s, want refs/heads/main", head.Name())
	}
	if c.Author.Name != "hedgehug[bot]" || c.Author.Email != "bot@hedgehug.invalid" || c.NumParents() != 0 || c.Message != "Store を作成\n" {
		t.Errorf("commit = %+v", c)
	}
}

func TestCreateRefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Create(dir, "01STORE0000000000000000001", t0); err == nil {
		t.Error("Create on a non-empty dir succeeded")
	}
}

func TestAttachRefuses(t *testing.T) {
	s := New(apptest.NewClock(t0), apptest.NewIDs())
	if _, err := s.Attach(t.TempDir()); !errors.Is(err, ErrNotStore) {
		t.Errorf("plain dir: %v", err)
	}
	_, _, dir := newStore(t)
	if err := os.WriteFile(filepath.Join(dir, ".hedgehug", "store.yaml"), []byte("store: X\nformat: 2\nretention_days: 30\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach(dir); !errors.Is(err, ErrNewerFormat) {
		t.Errorf("newer format: %v", err)
	}
}

func TestUnknownStore(t *testing.T) {
	s := New(apptest.NewClock(t0), apptest.NewIDs())
	if _, err := s.dir("nope"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("dir: %v", err)
	}
}

func TestLockIsExclusive(t *testing.T) {
	_, _, dir := newStore(t)
	unlock, err := lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan struct{})
	go func() {
		u, err := lock(dir)
		if err == nil {
			u()
		}
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("second lock was taken while the first was held")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	<-got
}
```

- [ ] **Step 3: テストが落ちることを確かめる**

Run: `go test ./internal/infra/...`
Expected: FAIL(`undefined: sysclock.IDs`、`undefined: Create` ほか)

- [ ] **Step 4: 実装する**

`internal/infra/sysclock/sysclock.go`:

```go
// Package sysclock は、このマシンの時計と、内部 ID(ULID)の採番を持つ。
package sysclock

import (
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Clock はこのマシンの時計。時刻帯はこのマシンのもの(記録の at に書く)。
type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

// IDs は ULID の採番。同じミリ秒の中でも増えていく。
type IDs struct{}

func (IDs) New() change.ID { return change.ID(ulid.Make().String()) }
```

`internal/infra/gitstore/create.go`:

```go
package gitstore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Format は、この hedgehug が書く Store の形式の版。
const Format = 1

// defaultRetentionDays は削除済みの保持日数の既定(08「保持日数」)。
const defaultRetentionDays = 30

var (
	ErrNotStore    = errors.New("not a hedgehug store")
	ErrNewerFormat = errors.New("the store was made by a newer hedgehug")
)

const storeFile = ".hedgehug/store.yaml"

type storeYAML struct {
	Store         string `yaml:"store"`
	Format        int    `yaml:"format"`
	RetentionDays int    `yaml:"retention_days"`
}

// Create は、空のフォルダ(またはないフォルダ)に Store を作る。最初のコミットで store.yaml だけを書く。
// このコミットは保存ではないので Hedgehug-Op を持たない(保存形式「.hedgehug/store.yaml」)。
func Create(dir string, id port.StoreID, at time.Time) error {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("create store in %s: the folder is not empty", dir)
	}
	// ブランチは GitHub の既定に合わせて main にする(2026-11-12 決定)。
	repo, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}
	b, err := encodeYAML(storeYAML{Store: string(id), Format: Format, RetentionDays: defaultRetentionDays})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hedgehug"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, storeFile), b, 0o644); err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	if _, err := wt.Add(storeFile); err != nil {
		return err
	}
	sig := signature(botName, botEmail, at)
	_, err = wt.Commit("Store を作成\n", &git.CommitOptions{Author: sig, Committer: sig})
	return err
}

func readStoreYAML(dir string) (storeYAML, error) {
	b, err := os.ReadFile(filepath.Join(dir, storeFile))
	if errors.Is(err, os.ErrNotExist) {
		return storeYAML{}, ErrNotStore
	}
	if err != nil {
		return storeYAML{}, err
	}
	var y storeYAML
	if err := yaml.Unmarshal(b, &y); err != nil || y.Store == "" {
		return storeYAML{}, ErrNotStore
	}
	if y.Format > Format {
		return storeYAML{}, ErrNewerFormat
	}
	return y, nil
}

// encodeYAML は、字下げ 2 の YAML にする。
func encodeYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

`internal/infra/gitstore/stores.go`:

```go
// Package gitstore は、Store を git のリポジトリで持つ port.Stores の実装。
// git の言葉(コミット、ブランチ、clone)は、このパッケージの外に出さない。
package gitstore

import (
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

const (
	botName  = change.BotLogin
	botEmail = "bot@hedgehug.invalid"
)

// Stores は、取り付けた Store を git のリポジトリとして読み書きする。
type Stores struct {
	clock port.Clock
	ids   port.IDs

	mu   sync.Mutex
	dirs map[port.StoreID]string

	// beforeCommit は、ファイルを書いたあと、コミットの前に呼ぶ。テストで失敗を起こすためのもの。
	beforeCommit func() error
}

func New(clock port.Clock, ids port.IDs) *Stores {
	return &Stores{clock: clock, ids: ids, dirs: map[port.StoreID]string{}}
}

// Attach は、dir の Store を開いて使えるようにし、その Store ID を返す。
func (s *Stores) Attach(dir string) (port.StoreID, error) {
	y, err := readStoreYAML(dir)
	if err != nil {
		return "", err
	}
	id := port.StoreID(y.Store)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirs[id] = dir
	return id, nil
}

func (s *Stores) dir(id port.StoreID) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.dirs[id]
	if !ok {
		return "", port.ErrStoreNotFound
	}
	return d, nil
}

// author は、保存の作者とコミッター(01「「あなた」の決まり方」)。
func author(a change.Actor, at time.Time) *object.Signature {
	if a.Kind == change.ActorUser {
		return signature(a.Login, a.Login+"@hedgehug.invalid", at)
	}
	return signature(botName, botEmail, at)
}

func signature(name, email string, at time.Time) *object.Signature {
	return &object.Signature{Name: name, Email: email, When: at}
}
```

`internal/infra/gitstore/lock.go`:

```go
package gitstore

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lock は、Store ごとのファイルロックをかける。1 回の操作のあいだ持つ。画面のサーバーと CLI が
// 同じ Store に書いても、コミットは 1 つずつ順に作られる(保存形式「技術構成」)。
// flock はファイルを開くごとに働くので、同じプロセスの中の 2 つの操作も順になる。
func lock(root string) (unlock func(), err error) {
	f, err := os.OpenFile(filepath.Join(root, ".git", "hedgehug.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
```

`stores.go` の `Read`・`Write` は Task 7 で足す。この時点では `Stores` は `port.Stores` を満たさない。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/infra/... && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件。gosec が `0o644`・`0o755` を指摘したら、Store のファイルは GitHub 上で人が読むもので秘密ではないので、その行に `//nolint:gosec // Store のファイルは共有するもので秘密ではない` を付ける。

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/infra
git commit -m "feat(infra): 時計と ULID の採番、Store の作成・取り付け・ファイルロックを足す"
```

---

### Task 7: gitstore の読み書き(1 回の Write = 1 コミット)

gitstore に `Read`・`Write` を足し、偽物と同じ照合テストを通す。git 版だけの約束(コミットの作者・メッセージ・`*.yaml` の作り直し・途中で失敗したときの戻し方)を、gitstore のテストで確かめる。

**Files:**
- Create: `internal/infra/gitstore/files.go`(パスと log.jsonl の読み書き)
- Create: `internal/infra/gitstore/read.go`(Reader)
- Create: `internal/infra/gitstore/write.go`(Read・Write・Tx)
- Create: `internal/infra/gitstore/commit.go`(ファイルを書いてコミット、失敗したら戻す)
- Create: `internal/infra/gitstore/yaml.go`(`*.yaml` の形)
- Test: `internal/infra/gitstore/contract_test.go`、`internal/infra/gitstore/write_test.go`

**Interfaces:**
- Consumes: Task 5 の `port.*`・`apptest.RunStoresContract`、Task 3 の `change.Stamp`、Task 4 の `*.FromLog`、Task 6 の `Stores`・`lock`・`author`・`encodeYAML`
- Produces: `*gitstore.Stores` が `port.Stores` を満たす(`var _ port.Stores = (*Stores)(nil)`)

**ファイルの場所(保存形式「ファイル構成」):** メンバーは `members/log.jsonl`・`members/members.yaml`、ワークスペースは `workspaces/<ID>/log.jsonl`・`workspace.yaml`、プロジェクトは `projects/<ID>/…`・`project.yaml`、アイテムは `items/<ID>/…`・`item.yaml`。ID はパスに使うので、英数字だけを認める(それ以外は書き込みを断る)。

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/gitstore/contract_test.go`:

```go
package gitstore

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

var _ port.Stores = (*Stores)(nil)

func TestContract(t *testing.T) {
	apptest.RunStoresContract(t, func(t *testing.T) (port.Stores, port.StoreID) {
		s, id, _ := newStore(t)
		return s, id
	})
}
```

`internal/infra/gitstore/write_test.go`:

```go
package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func headCount(t *testing.T, dir string) int {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	_ = iter.ForEach(func(*object.Commit) error { n++; return nil })
	return n
}

func TestOneWriteIsOneCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	repo, _ := git.PlainOpen(dir)
	head, _ := repo.Head()
	c, _ := repo.CommitObject(head.Hash())
	if headCount(t, dir) != 2 {
		t.Errorf("commits = %d, want 2 (store.yaml + one save)", headCount(t, dir))
	}
	if c.Author.Name != "taro" || c.Author.Email != "taro@hedgehug.invalid" || c.Committer.Name != "taro" {
		t.Errorf("author = %+v, committer = %+v", c.Author, c.Committer)
	}
	subject, trailer, _ := strings.Cut(c.Message, "\n\n")
	if subject != "サンプルデータを作成" || !strings.HasPrefix(trailer, "Hedgehug-Op: 01TEST") {
		t.Errorf("message = %q", c.Message)
	}
	status := worktreeStatus(t, dir)
	if !status.IsClean() {
		t.Errorf("worktree is not clean after a save:\n%s", status)
	}
}

func TestEmptyWriteMakesNoCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro")}, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if headCount(t, dir) != 1 {
		t.Errorf("an empty write made a commit")
	}
}

func TestYAMLIsRebuiltFromLog(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("hanako"), Summary: "HH-2 のタイトルを変更"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "items", string(apptest.SampleTask), "item.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := `id: 01SAMPLEIT0000000000000002
workspace: 01SAMPLEWS0000000000000001
project: 01SAMPLEPR0000000000000001
parent: 01SAMPLEIT0000000000000001
number: 2
kind: Task
rank: a
title: ログイン画面
created: {at: "2026-11-12T09:00:00.000+09:00", by: taro}
`
	if string(b) != want {
		t.Errorf("item.yaml =\n%s\nwant\n%s", b, want)
	}
	log, err := os.ReadFile(filepath.Join(dir, "items", string(apptest.SampleTask), "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(log), "\n"); n != 2 {
		t.Errorf("log.jsonl has %d lines, want 2", n)
	}
}

func TestFailedCommitLeavesNothing(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	s.beforeCommit = func() error { return boom }
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro"), Summary: "x"}, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "x")),
			tx.WriteItems().Append("01NEWITEM00000000000000001", change.NewCreate(map[string]any{"id": "01NEWITEM00000000000000001"})),
		)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write error = %v", err)
	}
	if status := worktreeStatus(t, dir); !status.IsClean() {
		t.Errorf("worktree after a failed commit:\n%s", status)
	}
	if _, err := os.Stat(filepath.Join(dir, "items", "01NEWITEM00000000000000001")); !os.IsNotExist(err) {
		t.Errorf("new folder left: %v", err)
	}
	if headCount(t, dir) != 2 {
		t.Errorf("a failed write made a commit")
	}
}

func TestRefusesIDThatIsNotAPath(t *testing.T) {
	s, id, _ := newStore(t)
	err := s.Write(t.Context(), id, port.Operation{Actor: change.User("taro")}, func(tx port.Tx) error {
		return tx.WriteItems().Append("../../etc", change.NewCreate(map[string]any{}))
	})
	if err == nil {
		t.Error("an id with a slash was written")
	}
}

func TestReopenReadsTheSameData(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	s2 := New(apptest.NewClock(t0), apptest.NewIDs())
	if _, err := s2.Attach(dir); err != nil {
		t.Fatal(err)
	}
	err := s2.Read(t.Context(), id, func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleTask)
		if err != nil || it.Title != "ログイン画面を作る" || it.DisplayID("HH") != "HH-2" {
			t.Errorf("item = %+v, %v", it, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func worktreeStatus(t *testing.T, dir string) git.Status {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	st, err := wt.Status()
	if err != nil {
		t.Fatal(err)
	}
	return st
}
```

`.git/hedgehug.lock` は `.git` の中なので、作業ツリーの状態には出ない。`TestYAMLIsRebuiltFromLog` の `created.at` は、`newStore` の時計を最初に読む Sample の `Write` の時刻で、`t0` になる。期待する `item.yaml` が、YAML の書き出しの引用符の付け方だけで食い違ったら(例: `at` の値の引用符)、書き出しに合わせて `want` を直してよい。キーの並びと値が食い違ったら、実装を直す。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/gitstore/`
Expected: FAIL(`*Stores does not implement port.Stores (missing method Read)`)

- [ ] **Step 3: パスと log.jsonl の読み書きを書く**

`internal/infra/gitstore/files.go`:

```go
package gitstore

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

var safeID = regexp.MustCompile(`^[0-9A-Za-z]+$`)

// kindDirs は、種類ごとのフォルダと、読む用の YAML の名前。
var kindDirs = map[change.Kind]struct{ dir, yaml string }{
	change.KindMembers:   {"members", "members.yaml"},
	change.KindWorkspace: {"workspaces", "workspace.yaml"},
	change.KindProject:   {"projects", "project.yaml"},
	change.KindItem:      {"items", "item.yaml"},
}

// dirOf は、エンティティのフォルダ(Store の根からの相対パス)。
func dirOf(ref change.Ref) (string, error) {
	k, ok := kindDirs[ref.Kind]
	if !ok {
		return "", fmt.Errorf("unknown kind %q", ref.Kind)
	}
	if ref.Kind == change.KindMembers {
		return k.dir, nil
	}
	if !safeID.MatchString(string(ref.ID)) {
		return "", fmt.Errorf("id %q cannot be a folder name", ref.ID)
	}
	return filepath.Join(k.dir, string(ref.ID)), nil
}

// readLog は、log.jsonl を読む。ファイルがなければ空。形の違う行(GitHub 上で手で書き換えたもの)は読み飛ばす。
func readLog(root string, ref change.Ref) ([]change.Record, error) {
	d, err := dirOf(ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(root, d, "log.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []change.Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var r change.Record
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}

// listIDs は、種類のフォルダの下のエンティティの ID を、辞書順で返す。
func listIDs(root string, kind change.Kind) ([]change.ID, error) {
	entries, err := os.ReadDir(filepath.Join(root, kindDirs[kind].dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []change.ID
	for _, e := range entries {
		if e.IsDir() && safeID.MatchString(e.Name()) {
			ids = append(ids, change.ID(e.Name()))
		}
	}
	slices.SortFunc(ids, func(a, b change.ID) int { return strings.Compare(string(a), string(b)) })
	return ids, nil
}

// appendLog は、log.jsonl に行を足す。ファイルを新しく作ったら true。
func appendLog(root string, ref change.Ref, recs []change.Record) (created bool, err error) {
	d, err := dirOf(ref)
	if err != nil {
		return false, err
	}
	path := filepath.Join(root, d, "log.jsonl")
	_, statErr := os.Stat(path)
	created = errors.Is(statErr, os.ErrNotExist)
	if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
		return created, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return created, err
	}
	w := bufio.NewWriter(f)
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			_ = f.Close()
			return created, err
		}
		_, _ = w.Write(append(b, '\n'))
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return created, err
	}
	return created, f.Close()
}
```

- [ ] **Step 4: 読む用の YAML の形を書く**

`internal/infra/gitstore/yaml.go`:

```go
package gitstore

import (
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 読む用の YAML の形。キーの並びは保存形式「アイテムの例」と同じ。行末の注釈(値の名前・表示の ID)は
// トラック A・B が足す。

type memberYAML struct {
	Login string `yaml:"login"`
	Name  string `yaml:"name"`
}

type workspaceYAML struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Prefix string `yaml:"prefix"`
}

type projectYAML struct {
	ID        string `yaml:"id"`
	Workspace string `yaml:"workspace"`
	Number    int    `yaml:"number"`
	Title     string `yaml:"title"`
}

type createdYAML struct {
	At string `yaml:"at"`
	By string `yaml:"by"`
}

type itemYAML struct {
	ID        string      `yaml:"id"`
	Workspace string      `yaml:"workspace"`
	Project   string      `yaml:"project"`
	Parent    *string     `yaml:"parent"`
	Number    int         `yaml:"number"`
	Kind      string      `yaml:"kind"`
	Rank      string      `yaml:"rank"`
	Title     string      `yaml:"title"`
	Created   createdYAML `yaml:"created,flow"`
}

// renderYAML は、エンティティのログから読む用の YAML を作る。作成の行がなければ nil。
func renderYAML(kind change.Kind, log []change.Record) ([]byte, error) {
	var v any
	switch kind {
	case change.KindMembers:
		ms := []memberYAML{}
		for _, m := range member.FromLog(log) {
			ms = append(ms, memberYAML(m))
		}
		v = ms
	case change.KindWorkspace:
		w, ok := workspace.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = workspaceYAML{ID: string(w.ID), Name: w.Name, Prefix: w.Prefix}
	case change.KindProject:
		p, ok := project.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = projectYAML{ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Title: p.Title}
	case change.KindItem:
		it, ok := item.FromLog(log)
		if !ok {
			return nil, nil
		}
		v = itemToYAML(it)
	}
	return encodeYAML(v)
}

func itemToYAML(it item.Item) itemYAML {
	y := itemYAML{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title,
		Created: createdYAML{At: it.Created.At.Format(change.TimeLayout), By: it.Created.By},
	}
	if it.Place.Parent != nil {
		p := string(*it.Place.Parent)
		y.Parent = &p
	}
	return y
}
```

`memberYAML(m)` の変換は、`member.Member` と `memberYAML` のフィールドの名前・型・並びが同じなので書ける(タグは無視される)。

- [ ] **Step 5: Reader を書く**

`internal/infra/gitstore/read.go`:

```go
package gitstore

import (
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// view は、作業ツリーのログと、同じ Tx で書いたログ(pending)を合わせて読む。作業ツリーは、
// ロックを持っているあいだ、いつもいまのコミットと同じ中身である。
type view struct {
	root    string
	pending map[change.Ref][]change.Record
}

func (v *view) log(ref change.Ref) ([]change.Record, error) {
	base, err := readLog(v.root, ref)
	if err != nil {
		return nil, err
	}
	return append(base, v.pending[ref]...), nil
}

func (v *view) ids(kind change.Kind) ([]change.ID, error) {
	ids, err := listIDs(v.root, kind)
	if err != nil {
		return nil, err
	}
	for ref := range v.pending {
		if ref.Kind == kind && !slices.Contains(ids, ref.ID) {
			ids = append(ids, ref.ID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

func (v *view) Workspaces() port.WorkspaceReader { return workspaces{v} }
func (v *view) Projects() port.ProjectReader     { return projects{v} }
func (v *view) Items() port.ItemReader           { return items{v} }
func (v *view) Members() port.MemberReader       { return members{v} }
func (v *view) Changes() port.ChangeReader       { return changes{v} }

// get は、ログを読んで from で値にする。作成の行がなければ port.ErrNotFound。
func get[T any](v *view, ref change.Ref, from func([]change.Record) (T, bool)) (T, error) {
	var zero T
	log, err := v.log(ref)
	if err != nil {
		return zero, err
	}
	x, ok := from(log)
	if !ok {
		return zero, port.ErrNotFound
	}
	return x, nil
}

type workspaces struct{ v *view }

func (r workspaces) Get(id workspace.ID) (workspace.Workspace, error) {
	return get(r.v, change.Ref{Kind: change.KindWorkspace, ID: id}, workspace.FromLog)
}

func (r workspaces) List() ([]workspace.Workspace, error) {
	ids, err := r.v.ids(change.KindWorkspace)
	if err != nil {
		return nil, err
	}
	var out []workspace.Workspace
	for _, id := range ids {
		if w, err := r.Get(id); err == nil {
			out = append(out, w)
		}
	}
	return out, nil
}

type projects struct{ v *view }

func (r projects) Get(id project.ID) (project.Project, error) {
	return get(r.v, change.Ref{Kind: change.KindProject, ID: id}, project.FromLog)
}

type items struct{ v *view }

func (r items) Get(id item.ID) (item.Item, error) {
	return get(r.v, change.Ref{Kind: change.KindItem, ID: id}, item.FromLog)
}

type members struct{ v *view }

func (r members) List() ([]member.Member, error) {
	log, err := r.v.log(change.Ref{Kind: change.KindMembers})
	if err != nil {
		return nil, err
	}
	return member.FromLog(log), nil
}

type changes struct{ v *view }

func (r changes) Of(ref change.Ref) ([]change.Record, error) {
	log, err := r.v.log(ref)
	if err != nil {
		return nil, err
	}
	if len(log) == 0 {
		return nil, port.ErrNotFound
	}
	return log, nil
}
```

`readLog` は ID がパスに使えないときエラーを返す。`Get("01NOPE…")` は英数字なのでファイルがなく、空のログから `port.ErrNotFound` になる。

- [ ] **Step 6: Read・Write と Tx を書く**

`internal/infra/gitstore/write.go`:

```go
package gitstore

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func (s *Stores) Read(_ context.Context, id port.StoreID, fn func(r port.Reader) error) error {
	root, err := s.dir(id)
	if err != nil {
		return err
	}
	unlock, err := lock(root)
	if err != nil {
		return err
	}
	defer unlock()
	return fn(&view{root: root})
}

func (s *Stores) Write(_ context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	root, err := s.dir(id)
	if err != nil {
		return err
	}
	unlock, err := lock(root)
	if err != nil {
		return err
	}
	defer unlock()
	t := &tx{
		view: view{root: root, pending: map[change.Ref][]change.Record{}},
		meta: change.Meta{Op: s.ids.New(), At: s.clock.Now(), Actor: op.Actor},
		ids:  s.ids,
	}
	if err := fn(t); err != nil {
		return err
	}
	if len(t.pending) == 0 {
		return nil
	}
	return s.commit(root, t, op)
}

type tx struct {
	view
	meta change.Meta
	ids  port.IDs
}

func (t *tx) append(ref change.Ref, drafts []change.Draft) error {
	if _, err := dirOf(ref); err != nil {
		return err
	}
	log, err := t.log(ref)
	if err != nil {
		return err
	}
	t.pending[ref] = append(t.pending[ref], change.Stamp(log, drafts, t.meta, t.ids.New)...)
	return nil
}

func (t *tx) WriteWorkspaces() port.WorkspaceWriter { return idWriter{t, change.KindWorkspace} }
func (t *tx) WriteProjects() port.ProjectWriter     { return idWriter{t, change.KindProject} }
func (t *tx) WriteItems() port.ItemWriter           { return idWriter{t, change.KindItem} }
func (t *tx) WriteMembers() port.MemberWriter       { return membersWriter{t} }

type idWriter struct {
	t    *tx
	kind change.Kind
}

func (w idWriter) Append(id change.ID, drafts ...change.Draft) error {
	return w.t.append(change.Ref{Kind: w.kind, ID: id}, drafts)
}

type membersWriter struct{ t *tx }

func (w membersWriter) Append(drafts ...change.Draft) error {
	return w.t.append(change.Ref{Kind: change.KindMembers}, drafts)
}
```

- [ ] **Step 7: コミットと戻し方を書く**

`internal/infra/gitstore/commit.go`:

```go
package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// commit は、t の pending を log.jsonl に足し、*.yaml を作り直して、1 つのコミットにする。
// 途中で失敗したら、作業ツリーとインデックスを操作の前のコミットに戻す(保存形式「技術構成」)。
func (s *Stores) commit(root string, t *tx, op port.Operation) error {
	repo, err := git.PlainOpen(root)
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	var newDirs []string
	err = s.writeAndCommit(wt, t, op, &newDirs)
	if err != nil {
		return errors.Join(err, rollback(root, wt, head.Hash(), newDirs))
	}
	return nil
}

func (s *Stores) writeAndCommit(wt *git.Worktree, t *tx, op port.Operation, newDirs *[]string) error {
	refs := make([]change.Ref, 0, len(t.pending))
	for ref := range t.pending {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b change.Ref) int {
		return strings.Compare(string(a.Kind)+"/"+string(a.ID), string(b.Kind)+"/"+string(b.ID))
	})
	for _, ref := range refs {
		paths, created, err := writeEntity(t, ref)
		if created {
			d, _ := dirOf(ref)
			*newDirs = append(*newDirs, d)
		}
		if err != nil {
			return err
		}
		for _, p := range paths {
			if _, err := wt.Add(p); err != nil {
				return err
			}
		}
	}
	if s.beforeCommit != nil {
		if err := s.beforeCommit(); err != nil {
			return err
		}
	}
	sig := author(op.Actor, t.meta.At)
	_, err := wt.Commit(op.Summary+"\n\nHedgehug-Op: "+string(t.meta.Op)+"\n", &git.CommitOptions{Author: sig, Committer: sig})
	return err
}

// writeEntity は、1 つのエンティティの log.jsonl に行を足し、読む用の YAML を作り直す。
// 書いたファイル(Store の根からの相対パス)と、log.jsonl を新しく作ったかを返す。
func writeEntity(t *tx, ref change.Ref) (paths []string, created bool, err error) {
	d, err := dirOf(ref)
	if err != nil {
		return nil, false, err
	}
	base, err := readLog(t.root, ref)
	if err != nil {
		return nil, false, err
	}
	created, err = appendLog(t.root, ref, t.pending[ref])
	if err != nil {
		return nil, created, err
	}
	paths = append(paths, filepath.Join(d, "log.jsonl"))
	y, err := renderYAML(ref.Kind, append(base, t.pending[ref]...))
	if err != nil || y == nil {
		return paths, created, err
	}
	name := filepath.Join(d, kindDirs[ref.Kind].yaml)
	if err := os.WriteFile(filepath.Join(t.root, name), y, 0o644); err != nil {
		return paths, created, err
	}
	return append(paths, name), created, nil
}

// rollback は、作業ツリーとインデックスを head に戻し、新しく作ったフォルダを消す。
func rollback(root string, wt *git.Worktree, head plumbing.Hash, newDirs []string) error {
	errs := []error{wt.Reset(&git.ResetOptions{Commit: head, Mode: git.HardReset})}
	for _, d := range newDirs {
		errs = append(errs, os.RemoveAll(filepath.Join(root, d)))
	}
	return errors.Join(errs...)
}
```

`writeEntity` の中の `readLog` は、`appendLog` の前の作業ツリーのログ(= 保存済みのログ)を読む。`t.log(ref)` を使うと pending が二重に入るので使わない。

メンバーのフォルダ(`members`)は、`dirOf` が `members` を返す。最初の保存で新しく作られれば、失敗したときに `members` ごと消える。

- [ ] **Step 8: テストが通ることを確かめる**

Run: `go test -race ./internal/infra/gitstore/`
Expected: PASS(照合テスト 7 件と、gitstore だけのテスト)

失敗したら、照合テストの名前で、偽物と食い違う振る舞いを直す。照合テストの期待を、本物に合わせて変えてはいけない(変えるのは、口の約束そのものを見直すときだけで、その変更は偽物にも流す)。

- [ ] **Step 9: lint と大きさを確かめる**

Run: `make check`
Expected: すべて成功。ファイルが 300 行・関数が 60 行・認知的複雑度が 15 を超えたら分ける。gosec が Store の中のファイルの読み書き(G301・G304・G306)を指摘したら、パスは `dirOf` で検査済みの ID から作り、中身は共有するもので秘密ではないので、その行に理由付きの `//nolint:gosec` を付ける。

- [ ] **Step 10: Commit**

```bash
git add internal/infra/gitstore
git commit -m "feat(infra): gitstore の読み書きを足す(1 回の Write = 1 コミット、失敗したら戻す)"
```

---

### Task 8: HTTP の API の骨組みと起動(httpapi・cmd)

huma の API を機能ごとのパッケージで登録する形を作る。トークンの検査、画面のファイルの配信、`api/openapi.yaml` の生成、本物と e2e 用の組み立てを作る。

**Files:**
- Create: `internal/adapter/httpapi/server.go`、`internal/adapter/httpapi/routes.go`、`internal/adapter/httpapi/generate.go`
- Create: `internal/adapter/httpapi/system/system.go`
- Test: `internal/adapter/httpapi/server_test.go`
- Create: `cmd/openapi-gen/main.go`、`api/openapi.yaml`(生成物)
- Modify: `cmd/hedgehug/main.go`(Task 1 の仮を書き直す)
- Create: `cmd/hedgehug/wire.go`、`cmd/hedgehug/wire_e2e.go`
- Create: `web/embed.go`、`web/dist/.gitkeep`

**Interfaces:**
- Consumes: Task 5 の `port.Stores`・`apptest.NewStores`・`apptest.Sample`、Task 6 の `gitstore.New`・`sysclock`
- Produces:
  - `httpapi.Deps{Version string; Stores port.Stores}`(トラックがフィールドを足す)
  - `httpapi.NewAPI(mux *http.ServeMux, deps Deps) huma.API`
  - `httpapi.New(token string, static fs.FS, deps Deps) http.Handler`
  - `system.Register(api huma.API, version string)` — `GET /api/system`、operationId `get-system`、本文 `{"version": string}`
  - `web.Dist() fs.FS`
  - 機能の API の足し方: `internal/adapter/httpapi/<機能>/` に `Register(api huma.API, …)` を置き、`routes.go` の `NewAPI` に 1 行足す

**トークン(この計画の決定 7):** `/api/` の下はすべて `Authorization: Bearer <トークン>` が要り、なければ・合わなければ 401。比べ方は定数時間。画面のファイルはトークンなしで返す(トークンのない画面を出すため)。`/api/` 以外で、ファイルがないパスには `index.html` を返す(画面のアドレス。01)。

- [ ] **Step 1: 依存を入れる**

```bash
go get github.com/danielgtaylor/huma/v2@v2.40.0
```

- [ ] **Step 2: 画面のファイルを埋め込む口を置く**

`web/dist/.gitkeep` を空のファイルで作る(`.gitignore` は `/web/dist/*` を無視し、`.gitkeep` だけを残す設定になっている)。

`web/embed.go`(前回の `web/embed.go` と同じ):

```go
// Package web は、make web が web/dist に作る画面を持つ。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist は、web/dist の中身を根に持つ画面のファイル。make web をしていなければ .gitkeep だけ。
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist は埋め込み済みなので、Sub が失敗するのは名前の誤りだけ
	}
	return sub
}
```

- [ ] **Step 3: 失敗するテストを書く**

`internal/adapter/httpapi/server_test.go`:

```go
package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>hedgehug</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	srv := httptest.NewServer(httpapi.New("secret", static, httpapi.Deps{Version: "v0.0.0-test"}))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestSystemNeedsToken(t *testing.T) {
	srv := newServer(t)
	for _, token := range []string{"", "wrong"} {
		if code, _ := get(t, srv.URL+"/api/system", token); code != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", token, code)
		}
	}
	code, body := get(t, srv.URL+"/api/system", "secret")
	if code != http.StatusOK || !strings.Contains(body, `"version":"v0.0.0-test"`) {
		t.Errorf("status %d, body %s", code, body)
	}
}

func TestUnknownAPIIs404WithToken(t *testing.T) {
	if code, _ := get(t, newServer(t).URL+"/api/nope", "secret"); code != http.StatusNotFound {
		t.Errorf("status %d, want 404", code)
	}
}

func TestStaticFilesNeedNoToken(t *testing.T) {
	srv := newServer(t)
	if code, body := get(t, srv.URL+"/assets/app.js", ""); code != http.StatusOK || body != "console.log(1)" {
		t.Errorf("asset: %d %q", code, body)
	}
	// 画面のアドレス(例: /ws/HH)は、index.html を返して画面が開く。
	for _, path := range []string{"/", "/ws/HH/items"} {
		if code, body := get(t, srv.URL+path, ""); code != http.StatusOK || !strings.Contains(body, "<title>hedgehug</title>") {
			t.Errorf("%s: %d %q", path, code, body)
		}
	}
}

func TestOpenAPIIsNotServed(t *testing.T) {
	srv := newServer(t)
	for _, path := range []string{"/openapi.json", "/docs"} {
		if _, body := get(t, srv.URL+path, "secret"); strings.Contains(body, "openapi") {
			t.Errorf("%s is served", path)
		}
	}
}
```

- [ ] **Step 4: テストが落ちることを確かめる**

Run: `go test ./internal/adapter/...`
Expected: FAIL(`undefined: httpapi.New`)

- [ ] **Step 5: 実装する**

`internal/adapter/httpapi/system/system.go`:

```go
// Package system は、サーバーそのものについての API を持つ。
package system

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type output struct {
	Body struct {
		Version string `json:"version" doc:"hedgehug の版"`
	}
}

// Register は GET /api/system を登録する。
func Register(api huma.API, version string) {
	huma.Register(api, huma.Operation{
		OperationID: "get-system",
		Method:      http.MethodGet,
		Path:        "/api/system",
		Summary:     "サーバーの版を返す",
	}, func(context.Context, *struct{}) (*output, error) {
		out := &output{}
		out.Body.Version = version
		return out, nil
	})
}
```

`internal/adapter/httpapi/routes.go`:

```go
package httpapi

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/system"
	"github.com/kukv/hedgehug/internal/app/port"
)

// Deps は、API が使う口。トラックが、自分の API に要る口を足す。
type Deps struct {
	Version string
	Stores  port.Stores
}

// NewAPI は、mux に API を登録する。機能の API は httpapi/<機能> の Register を 1 行ずつ足す。
func NewAPI(mux *http.ServeMux, deps Deps) huma.API {
	cfg := huma.DefaultConfig("hedgehug", deps.Version)
	cfg.OpenAPIPath = "" // OpenAPI は make generate で api/openapi.yaml に書く。サーバーでは出さない
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	api := humago.New(mux, cfg)

	system.Register(api, deps.Version)

	return api
}
```

`internal/adapter/httpapi/server.go`:

```go
// Package httpapi は、画面のための HTTP の API と、画面のファイルの配信を持つ。
package httpapi

import (
	"crypto/subtle"
	"errors"
	"io/fs"
	"net/http"
	"strings"
)

// New は、トークンで守った API と、画面のファイルを返す Handler を作る。
func New(token string, static fs.FS, deps Deps) http.Handler {
	api := http.NewServeMux()
	NewAPI(api, deps)
	root := http.NewServeMux()
	root.Handle("/api/", requireToken(token, api))
	root.Handle("/", spa(static))
	return root
}

// requireToken は、Authorization: Bearer <トークン> が合わなければ 401 を返す。
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spa は、ファイルがあればそれを、なければ index.html を返す(画面のアドレスは画面が解釈する)。
func spa(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if _, err := fs.Stat(static, name); errors.Is(err, fs.ErrNotExist) {
				r = r.Clone(r.Context())
				r.URL.Path = "/"
			}
		}
		files.ServeHTTP(w, r)
	})
}
```

`internal/adapter/httpapi/generate.go`:

```go
package httpapi

//go:generate go run ../../../cmd/openapi-gen ../../../api/openapi.yaml
```

`cmd/openapi-gen/main.go`:

```go
// Command openapi-gen は、httpapi の API の OpenAPI を、引数のファイルに書く(make generate)。
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: openapi-gen <output.yaml>")
		os.Exit(2)
	}
	api := httpapi.NewAPI(http.NewServeMux(), httpapi.Deps{Version: "dev"})
	// orval は OpenAPI 3.0 を読むので、3.1 から下げて書く。
	b, err := api.OpenAPI().DowngradeYAML()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[1], b, 0o644); err != nil { //nolint:gosec // 生成物はリポジトリに入れる公開のファイル
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 6: テストが通ることを確かめ、OpenAPI を生成する**

Run: `go test ./internal/adapter/... && mkdir -p api && make generate && head -20 api/openapi.yaml`
Expected: PASS。`api/openapi.yaml` に `openapi: 3.0.3` と `/api/system` の `get-system` がある。

- [ ] **Step 7: 起動と組み立てを書く**

`cmd/hedgehug/main.go`:

```go
// Command hedgehug は、このマシンで画面のサーバーを動かす。CLI(hh ui ほか)ができるまでは、
// 起動すると待ち受け、画面の URL を標準出力に出す(この計画の決定 7)。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/web"
)

// version は、リリースのビルドで -ldflags "-X main.version=…" から入る。
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hedgehug:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	deps, err := wire(ctx)
	if err != nil {
		return err
	}
	deps.Version = version
	token, addr := tokenAndAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: httpapi.New(token, web.Dist(), deps), ReadHeaderTimeout: 10 * time.Second}
	fmt.Printf("http://%s/#token=%s\n", ln.Addr(), token)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read は失敗しない(Go 1.24 から)
	return hex.EncodeToString(b)
}
```

`cmd/hedgehug/wire.go`:

```go
//go:build !e2e

package main

import (
	"context"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/infra/gitstore"
	"github.com/kukv/hedgehug/internal/infra/sysclock"
)

// wire は、本物の部品を口に差し込む。Store の一覧の読み込みはトラック G が足す(この計画の決定 6)。
func wire(context.Context) (httpapi.Deps, error) {
	stores := gitstore.New(sysclock.Clock{}, sysclock.IDs{})
	return httpapi.Deps{Stores: stores}, nil
}

// tokenAndAddr は、起動ごとのトークンと、127.0.0.1 の空いた番号を返す。
func tokenAndAddr() (token, addr string) { return newToken(), "127.0.0.1:0" }
```

`cmd/hedgehug/wire_e2e.go`:

```go
//go:build e2e

package main

import (
	"context"
	"os"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
)

// e2eStore は、e2e 用のビルドのサンプルデータを入れた Store の ID。
const e2eStore = "01E2ESTORE0000000000000001"

// wire は、Playwright の通しのテスト用に、口に apptest の偽物とサンプルデータを差し込む(設計書 6 節)。
func wire(ctx context.Context) (httpapi.Deps, error) {
	stores := apptest.NewStores(apptest.NewClock(time.Now()), apptest.NewIDs())
	stores.AddStore(e2eStore)
	if err := apptest.Sample(ctx, stores, e2eStore); err != nil {
		return httpapi.Deps{}, err
	}
	return httpapi.Deps{Stores: stores}, nil
}

// tokenAndAddr は、テストが決めたトークン(HEDGEHUG_TOKEN)と待ち受け(HEDGEHUG_ADDR)を返す。
func tokenAndAddr() (token, addr string) {
	return os.Getenv("HEDGEHUG_TOKEN"), os.Getenv("HEDGEHUG_ADDR")
}
```

- [ ] **Step 8: 両方のビルドと、起動を確かめる**

```bash
go build ./... && go vet -tags e2e ./cmd/hedgehug
go build -o bin/hedgehug ./cmd/hedgehug && (bin/hedgehug & pid=$!; sleep 1; kill $pid)
```

Expected: ビルドが通り、`http://127.0.0.1:<番号>/#token=<64 文字>` が 1 行出る。

- [ ] **Step 9: 全体を確かめる**

Run: `make check`
Expected: すべて成功。archtest が、httpapi が infra を import していないこと、cmd だけが infra と apptest を import していることを確かめる。

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum internal/adapter cmd api web/embed.go web/dist/.gitkeep
git commit -m "feat(adapter): HTTP の API の骨組み・トークン・画面の配信・OpenAPI の生成と起動を足す"
```

---

### Task 9: 画面の pnpm workspace と core(依存の向き・大きさの検査を含む)

`web/` を pnpm workspace にし、Svelte にも DOM にも依存しない `core` を作る。画面の依存の向き(dependency-cruiser)と大きさの上限(確認用スクリプト)を、ここで入れる。

**Files:**
- Create: `web/package.json`、`web/pnpm-workspace.yaml`、`web/biome.json`、`web/tsconfig.base.json`、`web/.dependency-cruiser.cjs`、`web/scripts/check-size.mjs`
- Create: `web/packages/core/package.json`、`web/packages/core/tsconfig.json`
- Create: `web/packages/core/src/index.ts`、`src/reactive/store.ts`、`src/model/system.ts`、`src/port/system.ts`、`src/features/shell/shell.ts`、`src/features/system/system.ts`
- Test: `web/packages/core/src/reactive/store.test.ts`、`src/features/shell/shell.test.ts`、`src/features/system/system.test.ts`
- Modify: `Makefile`(`web-check` を足し、`check` に入れる)

**Interfaces:**
- Consumes: なし
- Produces(`@hedgehug/core` から export):
  - `Readable<T>`・`Writable<T>`・`writable<T>(initial: T): Writable<T>`・`get<T>(s: Readable<T>): T`・`readonly<T>(s: Writable<T>): Readable<T>` — Svelte のストアの約束どおり `subscribe` を持つので、Svelte の画面で `$store` と書ける
  - `SystemInfo{ version: string }`、`SystemPort{ getSystem(): Promise<SystemInfo> }`
  - `createShell(): Shell`、`Shell{ drawerOpen: Readable<boolean>; openDrawer(); closeDrawer(); toggleDrawer() }`
  - `createSystemInfo(port: SystemPort): SystemInfoState`、`SystemInfoState{ version: Readable<string | null>; load(): Promise<boolean> }`
  - `pnpm lint`・`pnpm check`・`pnpm test`・`pnpm deps`・`pnpm size`(web/ で実行)

- [ ] **Step 1: workspace の設定を書く**

`web/pnpm-workspace.yaml`: 前回の `git show cf58222d~1:web/pnpm-workspace.yaml` の中身(サプライチェーン強化の設定と `allowBuilds`)をそのまま写し、先頭に次を足す。

```yaml
packages:
  - packages/*
  - app
```

`web/package.json`:

```json
{
  "name": "hedgehug-web",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "packageManager": "pnpm@12.17.0",
  "engines": {
    "node": "v26.13.0"
  },
  "scripts": {
    "build": "pnpm --filter @hedgehug/app build",
    "lint": "biome ci .",
    "fix": "biome check --write .",
    "check": "pnpm -r --if-present check",
    "test": "pnpm -r --if-present test",
    "deps": "depcruise packages app --config .dependency-cruiser.cjs",
    "size": "node scripts/check-size.mjs"
  },
  "devDependencies": {
    "@biomejs/biome": "2.6.4",
    "svelte": "5.57.3",
    "typescript": "6.0.3"
  }
}
```

`svelte` と `typescript` を根にも置くのは、dependency-cruiser が `.svelte` と `.ts` を読むのに使うため。

`web/tsconfig.base.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "lib": ["ES2023", "DOM", "DOM.Iterable"],
    "types": [],
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "noEmit": true
  }
}
```

`web/biome.json`: 前回の `git show cf58222d~1:web/biome.json` を写し、`files.includes` だけを次にする。

```json
    "includes": [
      "packages/**/*.ts",
      "packages/**/*.svelte",
      "packages/**/*.css",
      "!packages/api/src/generated.ts",
      "app/**/*.ts",
      "app/**/*.svelte",
      "app/**/*.css",
      "e2e/**/*.ts",
      "scripts/**/*.mjs",
      "*.ts",
      "*.cjs"
    ]
```

- [ ] **Step 2: core のパッケージを置く**

`web/packages/core/package.json`:

```json
{
  "name": "@hedgehug/core",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "exports": {
    ".": "./src/index.ts"
  },
  "scripts": {
    "check": "tsc -p .",
    "test": "vitest run"
  },
  "devDependencies": {
    "typescript": "6.0.3",
    "vitest": "5.0.5"
  }
}
```

`web/packages/core/tsconfig.json`(`lib` から DOM を外し、core が DOM を使えないようにする):

```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "lib": ["ES2023"]
  },
  "include": ["src"]
}
```

```bash
cd web
pnpm install
pnpm add -D -E -w dependency-cruiser
```

`pnpm add` は、公開から 7 日以上たった最新の版を入れる(`minimumReleaseAge`)。

- [ ] **Step 3: 失敗するテストを書く**

`web/packages/core/src/reactive/store.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get, readonly, writable } from "./store";

describe("writable", () => {
  it("calls a subscriber at once and on each change", () => {
    const s = writable(1);
    const seen: number[] = [];
    const stop = s.subscribe((v) => seen.push(v));
    s.set(2);
    s.update((v) => v + 1);
    stop();
    s.set(9);
    expect(seen).toEqual([1, 2, 3]);
  });

  it("does not call subscribers when the value is the same", () => {
    const s = writable("a");
    const seen: string[] = [];
    s.subscribe((v) => seen.push(v));
    s.set("a");
    expect(seen).toEqual(["a"]);
  });

  it("reads the current value with get, and readonly hides set", () => {
    const s = writable({ n: 1 });
    const r = readonly(s);
    s.set({ n: 2 });
    expect(get(r)).toEqual({ n: 2 });
    expect("set" in r).toBe(false);
  });
});
```

`web/packages/core/src/features/shell/shell.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createShell } from "./shell";

describe("shell", () => {
  it("opens, closes and toggles the drawer", () => {
    const shell = createShell();
    expect(get(shell.drawerOpen)).toBe(false);
    shell.openDrawer();
    expect(get(shell.drawerOpen)).toBe(true);
    shell.closeDrawer();
    expect(get(shell.drawerOpen)).toBe(false);
    shell.toggleDrawer();
    expect(get(shell.drawerOpen)).toBe(true);
  });
});
```

`web/packages/core/src/features/system/system.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { SystemPort } from "../../port/system";
import { get } from "../../reactive/store";
import { createSystemInfo } from "./system";

describe("system info", () => {
  it("loads the version from the server", async () => {
    const port: SystemPort = { getSystem: async () => ({ version: "v1.2.3" }) };
    const info = createSystemInfo(port);
    expect(get(info.version)).toBeNull();
    expect(await info.load()).toBe(true);
    expect(get(info.version)).toBe("v1.2.3");
  });

  it("keeps the version empty when the server fails", async () => {
    const port: SystemPort = {
      getSystem: async () => {
        throw new Error("401");
      },
    };
    const info = createSystemInfo(port);
    expect(await info.load()).toBe(false);
    expect(get(info.version)).toBeNull();
  });
});
```

- [ ] **Step 4: テストが落ちることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test`
Expected: FAIL(`./store` などが見つからない)

- [ ] **Step 5: 実装する**

`web/packages/core/src/reactive/store.ts`:

```ts
// 小さな購読の仕組み。Svelte のストアの約束(subscribe が今の値をすぐ渡し、止める関数を返す)に従うので、
// Svelte の画面では $store と書ける。core は Svelte を import しない。

export type Subscriber<T> = (value: T) => void;
export type Unsubscriber = () => void;

export interface Readable<T> {
  subscribe(run: Subscriber<T>): Unsubscriber;
}

export interface Writable<T> extends Readable<T> {
  set(value: T): void;
  update(fn: (value: T) => T): void;
}

export function writable<T>(initial: T): Writable<T> {
  let value = initial;
  const subscribers = new Set<Subscriber<T>>();
  const set = (next: T) => {
    if (Object.is(next, value)) return;
    value = next;
    for (const run of [...subscribers]) run(value);
  };
  return {
    subscribe(run) {
      subscribers.add(run);
      run(value);
      return () => {
        subscribers.delete(run);
      };
    },
    set,
    update: (fn) => set(fn(value)),
  };
}

export function readonly<T>(store: Writable<T>): Readable<T> {
  return { subscribe: store.subscribe };
}

export function get<T>(store: Readable<T>): T {
  let value: T | undefined;
  store.subscribe((v) => {
    value = v;
  })();
  return value as T;
}
```

`web/packages/core/src/model/system.ts`:

```ts
// 画面で使う型。API の型はそのまま使わず、api パッケージがこの型に直す(設計書 6 節)。
export interface SystemInfo {
  version: string;
}
```

`web/packages/core/src/port/system.ts`:

```ts
import type { SystemInfo } from "../model/system";

// サーバーとの口。実装は api パッケージにある。
export interface SystemPort {
  getSystem(): Promise<SystemInfo>;
}
```

`web/packages/core/src/features/shell/shell.ts`:

```ts
import { type Readable, readonly, writable } from "../../reactive/store";

// 画面の全体の枠の状態。狭い幅でサイドバーを引き出しとして開け閉めする。
export interface Shell {
  drawerOpen: Readable<boolean>;
  openDrawer(): void;
  closeDrawer(): void;
  toggleDrawer(): void;
}

export function createShell(): Shell {
  const drawerOpen = writable(false);
  return {
    drawerOpen: readonly(drawerOpen),
    openDrawer: () => drawerOpen.set(true),
    closeDrawer: () => drawerOpen.set(false),
    toggleDrawer: () => drawerOpen.update((open) => !open),
  };
}
```

`web/packages/core/src/features/system/system.ts`:

```ts
import type { SystemPort } from "../../port/system";
import { type Readable, readonly, writable } from "../../reactive/store";

// サーバーの版。読めなかったときは null のまま(つながらないときの画面はトラック G)。
export interface SystemInfoState {
  version: Readable<string | null>;
  load(): Promise<boolean>;
}

export function createSystemInfo(port: SystemPort): SystemInfoState {
  const version = writable<string | null>(null);
  return {
    version: readonly(version),
    async load() {
      try {
        version.set((await port.getSystem()).version);
        return true;
      } catch {
        return false;
      }
    },
  };
}
```

`web/packages/core/src/index.ts`:

```ts
export { createShell, type Shell } from "./features/shell/shell";
export { createSystemInfo, type SystemInfoState } from "./features/system/system";
export type { SystemInfo } from "./model/system";
export type { SystemPort } from "./port/system";
export {
  get,
  type Readable,
  readonly,
  type Subscriber,
  type Unsubscriber,
  type Writable,
  writable,
} from "./reactive/store";
```

- [ ] **Step 6: テストが通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm --filter @hedgehug/core check`
Expected: PASS、型の誤りなし

- [ ] **Step 7: 依存の向きと大きさの検査を書く**

`web/.dependency-cruiser.cjs`:

```js
// 画面のパッケージの依存の向き(設計書 6 節の表)。テストのファイルは vitest などを使うので、from から外す。
const notTest = "\\.test\\.ts$";
const svelte = "(^|/)node_modules/(\\.pnpm/[^/]+/node_modules/)?svelte/";

module.exports = {
  forbidden: [
    {
      name: "core-imports-only-itself",
      comment: "core は標準の TypeScript だけを使う(Svelte も DOM のライブラリも使わない)",
      severity: "error",
      from: { path: "^packages/core/", pathNot: notTest },
      to: { pathNot: "^packages/core/" },
    },
    {
      name: "api-imports-only-core",
      comment: "api が使ってよいのは core だけ",
      severity: "error",
      from: { path: "^packages/api/", pathNot: notTest },
      to: { path: "^packages/(ui|app)/|^app/" },
    },
    {
      name: "ui-imports-only-svelte",
      comment: "ui が使ってよいのは Svelte だけ",
      severity: "error",
      from: { path: "^packages/ui/", pathNot: notTest },
      to: { pathNot: ["^packages/ui/", svelte] },
    },
    {
      name: "packages-do-not-import-app",
      severity: "error",
      from: { path: "^packages/" },
      to: { path: "^app/" },
    },
  ],
  options: {
    doNotFollow: { path: "node_modules" },
    tsConfig: { fileName: "tsconfig.base.json" },
    enhancedResolveOptions: {
      exportsFields: ["exports"],
      conditionNames: ["svelte", "import", "default"],
      extensions: [".ts", ".js", ".mjs", ".svelte"],
    },
  },
};
```

`web/scripts/check-size.mjs`:

```js
// 設計書 4 節の大きさの上限: Svelte のコンポーネント 200 行、TypeScript のファイル 300 行。
// 生成したファイル(orval が作る API クライアント)は除く。超えたら上限を上げずに分ける。
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const limits = { ".svelte": 200, ".ts": 300 };
const skip = new Set(["node_modules", "dist", "test-results", "playwright-report"]);
const generated = /Generated by orval|Code generated .* DO NOT EDIT/;

let failed = false;

function lineCount(src) {
  if (src === "") return 0;
  return src.split("\n").length - (src.endsWith("\n") ? 1 : 0);
}

function walk(dir) {
  for (const name of readdirSync(dir)) {
    if (skip.has(name)) continue;
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      walk(path);
      continue;
    }
    const ext = Object.keys(limits).find((e) => name.endsWith(e));
    if (!ext) continue;
    const src = readFileSync(path, "utf8");
    if (generated.test(src.slice(0, 400))) continue;
    const n = lineCount(src);
    if (n > limits[ext]) {
      console.error(`${path}: ${n} 行(上限 ${limits[ext]} 行)。ファイルを分けてください`);
      failed = true;
    }
  }
}

for (const dir of ["packages", "app", "e2e"]) {
  try {
    walk(dir);
  } catch (e) {
    if (e.code !== "ENOENT") throw e;
  }
}
process.exit(failed ? 1 : 0);
```

- [ ] **Step 8: 違反を置いて、検査が落ちることを確かめる**

```bash
cd web
mkdir -p packages/ui/src
echo 'export const x = 1;' > packages/ui/src/planted.ts
echo 'import "../../ui/src/planted";' > packages/core/src/planted.ts
pnpm deps
```

Expected: FAIL(`core-imports-only-itself` の違反が 1 件)

```bash
node -e 'console.log("export const a = 1;\n".repeat(301))' > packages/core/src/planted.ts
pnpm size
```

Expected: FAIL(`packages/core/src/planted.ts: 301 行(上限 300 行)`)

確かめたら消す: `rm packages/core/src/planted.ts && rm -r packages/ui`

- [ ] **Step 9: Makefile に画面の検査を足す**

`Makefile` の `.PHONY` に `web-check` を足し、次を足す。`check` の行を `check: tidy-check lint fmt-check test web-check` にする。

```make
# 画面を確かめる: Biome、型、単体テスト、依存の向き、大きさの上限(CI の web と同じ)。
web-check:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size
```

- [ ] **Step 10: 全体を確かめる**

Run: `make check`
Expected: すべて成功

- [ ] **Step 11: Commit**

```bash
git add Makefile web/package.json web/pnpm-workspace.yaml web/pnpm-lock.yaml web/biome.json web/tsconfig.base.json web/.dependency-cruiser.cjs web/scripts web/packages/core
git commit -m "feat(web): pnpm workspace と core を足し、依存の向きと大きさを検査する"
```

---

### Task 10: 画面の api パッケージ(orval とトークン)

`api/openapi.yaml` から orval で API クライアントを作り、core の口(`SystemPort`)を実装する。トークンは URL のハッシュ(`#token=…`)から取り、すべての呼び出しに `Authorization: Bearer` で付ける。

**Files:**
- Create: `web/packages/api/package.json`、`web/packages/api/tsconfig.json`、`web/packages/api/orval.config.ts`
- Create: `web/packages/api/src/fetcher.ts`、`src/token.ts`、`src/system.ts`、`src/index.ts`、`src/generated.ts`(生成物)
- Test: `web/packages/api/src/token.test.ts`、`src/system.test.ts`
- Modify: `web/package.json`(`gen` を足す)、`Makefile`(`web-check` に生成物の検査を足す)

**Interfaces:**
- Consumes: Task 8 の `api/openapi.yaml`(`get-system`)、Task 9 の `SystemPort`・`SystemInfo`
- Produces(`@hedgehug/api` から export):
  - `tokenFromHash(hash: string): string | null`
  - `setToken(token: string): void`、`ApiError`(`status: number`)
  - `createSystemPort(): SystemPort`
  - 機能の API の足し方: トラックは `src/<機能>.ts` に、core の口の実装を 1 つ足す。`generated.ts` は手で直さず `pnpm gen` で作り直す

- [ ] **Step 1: パッケージを置く**

`web/packages/api/package.json`:

```json
{
  "name": "@hedgehug/api",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "exports": {
    ".": "./src/index.ts"
  },
  "scripts": {
    "gen": "orval",
    "check": "tsc -p .",
    "test": "vitest run"
  },
  "dependencies": {
    "@hedgehug/core": "workspace:*"
  },
  "devDependencies": {
    "orval": "8.46.0",
    "typescript": "6.0.3",
    "vitest": "5.0.5"
  }
}
```

`web/packages/api/tsconfig.json`:

```json
{
  "extends": "../../tsconfig.base.json",
  "include": ["src", "orval.config.ts"]
}
```

`web/packages/api/orval.config.ts`(前回の設定を、新しい置き場所に合わせたもの):

```ts
import { defineConfig } from "orval";

export default defineConfig({
  hedgehug: {
    input: "../../../api/openapi.yaml",
    output: {
      target: "src/generated.ts",
      client: "fetch",
      mode: "single",
      urlEncodeParameters: true,
      override: {
        fetch: { includeHttpResponseReturnType: false },
        mutator: { path: "src/fetcher.ts", name: "fetcher" },
      },
    },
  },
});
```

`web/package.json` の `scripts` に `"gen": "pnpm --filter @hedgehug/api gen"` を足す。

```bash
cd web && pnpm install
```

- [ ] **Step 2: 失敗するテストを書く**

`web/packages/api/src/token.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { tokenFromHash } from "./token";

describe("tokenFromHash", () => {
  it("reads the token from the hash that the server printed", () => {
    expect(tokenFromHash("#token=abc123")).toBe("abc123");
  });

  it("returns null when the URL has no token", () => {
    expect(tokenFromHash("")).toBeNull();
    expect(tokenFromHash("#foo=1")).toBeNull();
    expect(tokenFromHash("#token=")).toBeNull();
  });
});
```

`web/packages/api/src/system.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, setToken } from "./fetcher";
import { createSystemPort } from "./system";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("system port", () => {
  it("calls GET /api/system with the token and returns the model", async () => {
    const fetch = vi.fn(async () => new Response(JSON.stringify({ version: "v1" }), { status: 200 }));
    vi.stubGlobal("fetch", fetch);
    setToken("tok");
    await expect(createSystemPort().getSystem()).resolves.toEqual({ version: "v1" });
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/system");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer tok");
  });

  it("throws ApiError with the status when the server refuses", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("unauthorized", { status: 401 })));
    await expect(createSystemPort().getSystem()).rejects.toMatchObject({ status: 401 });
    await expect(createSystemPort().getSystem()).rejects.toBeInstanceOf(ApiError);
  });
});
```

- [ ] **Step 3: テストが落ちることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/api test`
Expected: FAIL(`./token` などが見つからない)

- [ ] **Step 4: 実装し、生成する**

`web/packages/api/src/fetcher.ts`:

```ts
// orval が作る呼び出しが通る fetch。トークンを付け、失敗した応答は ApiError にする。

let token = "";

export function setToken(value: string): void {
  token = value;
}

export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function fetcher<T>(url: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(url, { ...init, headers });
  if (!res.ok) throw new ApiError(res.status, `${res.status} ${res.statusText}`);
  const text = await res.text();
  return (text === "" ? undefined : JSON.parse(text)) as T;
}
```

`web/packages/api/src/token.ts`:

```ts
// サーバーが出す URL は http://127.0.0.1:<番号>/#token=<トークン>。トークンをハッシュに置くのは、
// サーバーのログと Referer に残さないため(この計画の決定 7)。
export function tokenFromHash(hash: string): string | null {
  const token = new URLSearchParams(hash.replace(/^#/, "")).get("token");
  return token ? token : null;
}
```

`web/packages/api/src/system.ts`:

```ts
import type { SystemPort } from "@hedgehug/core";
import { getSystem } from "./generated";

export function createSystemPort(): SystemPort {
  return {
    async getSystem() {
      const body = await getSystem();
      return { version: body.version };
    },
  };
}
```

`web/packages/api/src/index.ts`:

```ts
export { ApiError, setToken } from "./fetcher";
export { createSystemPort } from "./system";
export { tokenFromHash } from "./token";
```

```bash
cd web && pnpm gen
```

`src/generated.ts` ができ、`getSystem` が `fetcher` を通して `/api/system` を呼ぶ。生成された関数の名前が `getSystem` でなければ(operationId `get-system` から作られる)、`system.ts` の import を生成された名前に合わせる。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/api test && pnpm --filter @hedgehug/api check && pnpm deps && pnpm size`
Expected: PASS。`pnpm deps` は、api が core だけを使っていることを確かめる。`pnpm size` は `generated.ts` を数えない。

- [ ] **Step 6: 生成物が古くないかを Makefile で確かめる**

`Makefile` の `web-check` を次にする。

```make
web-check:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm gen
	git diff --exit-code -- web/packages/api/src/generated.ts
	cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size
```

Run: `make check`
Expected: すべて成功

- [ ] **Step 7: Commit**

```bash
git add Makefile web/package.json web/pnpm-lock.yaml web/packages/api
git commit -m "feat(web): orval の API クライアントと、トークン付きの呼び出しを足す"
```

---

### Task 11: ui のトークンと基本部品・全体の枠

見た目だけの Svelte 部品を作る。部品は props を受けて events(コールバックの props)を返すだけで、状態の決め方は持たない。見た目はモック版 19 の CSS を写す(振る舞いはモックから写さない)。幅による形の切り替えは、全体の枠のコンテナクエリで行う(この計画の決定 12)。

**Files:**
- Create: `web/packages/ui/package.json`、`tsconfig.json`、`svelte.config.js`、`vitest.config.ts`、`vitest.setup.ts`
- Create: `web/packages/ui/src/tokens/tokens.css`、`src/tokens/base.css`
- Create: `web/packages/ui/src/primitives/Button.svelte`・`IconButton.svelte`・`Input.svelte`・`Select.svelte`・`Switch.svelte`・`Tabs.svelte`・`Dialog.svelte`・`Menu.svelte`・`Toasts.svelte`
- Create: `web/packages/ui/src/layout/AppFrame.svelte`・`PageHead.svelte`・`Brand.svelte`
- Create: `web/packages/ui/src/types.ts`、`src/index.ts`
- Test: `web/packages/ui/src/primitives/primitives.test.ts`、`src/primitives/overlays.test.ts`、`src/layout/layout.test.ts`、`src/test-snippet.ts`

**Interfaces:**
- Consumes: なし(ui が使ってよいのは Svelte だけ)
- Produces(`@hedgehug/ui` から export):
  - `Button{ variant?: "default"|"primary"|"danger"|"ghost"; size?: "md"|"sm"|"xs"; type?; disabled?; title?; onclick?; children }`
  - `IconButton{ label: string; size?: "md"|"sm"|"xs"; active?; disabled?; title?; onclick?; children }`
  - `Input{ value (bindable); label: string; placeholder?; size?: "md"|"sm"; mono?; disabled?; title? }`
  - `Select{ value (bindable); label: string; options: SelectOption[]; size?: "md"|"sm"; disabled?; title? }`
  - `Switch{ checked (bindable); label: string; disabled?; title? }`
  - `Tabs{ tabs: Tab[]; current: string; onselect(id: string) }`
  - `Dialog{ open: boolean; title: string; wide?: boolean; onclose(); children; footer? }` — Esc と、ダイアログの外の押下で `onclose`
  - `Menu{ open: boolean; x: number; y: number; items: MenuItem[]; onselect(id: string); onclose() }` — Esc と、メニューの外の押下で `onclose`
  - `Toasts{ toasts: Toast[]; ondismiss(id: string); onaction?(id: string) }`
  - `AppFrame{ drawerOpen: boolean; onclosedrawer(); sidebar: Snippet; children: Snippet }`
  - `PageHead{ title: string; ondrawer(); crumbs?: Snippet; actions?: Snippet }`
  - `Brand{}`
  - 型 `SelectOption{ value: string; label: string }`、`Tab{ id: string; label: string; count?: number }`、`MenuItem{ id: string; label: string; hint?: string; danger?: boolean; disabled?: boolean } | { separator: true }`、`Toast{ id: string; text: string; error?: boolean; action?: string }`
  - CSS: `@hedgehug/ui/tokens.css`、`@hedgehug/ui/base.css`

- [ ] **Step 1: パッケージを置く**

`web/packages/ui/package.json`:

```json
{
  "name": "@hedgehug/ui",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "exports": {
    ".": {
      "svelte": "./src/index.ts",
      "default": "./src/index.ts"
    },
    "./tokens.css": "./src/tokens/tokens.css",
    "./base.css": "./src/tokens/base.css"
  },
  "scripts": {
    "check": "svelte-check --tsconfig ./tsconfig.json --fail-on-warnings",
    "test": "vitest run"
  },
  "dependencies": {
    "svelte": "5.57.3"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "7.3.1",
    "@testing-library/jest-dom": "7.0.1",
    "@testing-library/svelte": "5.4.2",
    "jsdom": "30.1.3",
    "svelte-check": "4.7.7",
    "typescript": "6.0.3",
    "vite": "8.3.5",
    "vitest": "5.0.5"
  }
}
```

`web/packages/ui/tsconfig.json`:

```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "types": ["svelte"]
  },
  "include": ["src", "vitest.setup.ts", "vitest.config.ts"]
}
```

`web/packages/ui/svelte.config.js`(前回と同じ):

```js
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

export default {
  preprocess: vitePreprocess(),
};
```

`web/packages/ui/vitest.config.ts`:

```ts
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts"],
    setupFiles: ["./vitest.setup.ts"],
  },
});
```

`web/packages/ui/vitest.setup.ts`:

```ts
import "@testing-library/jest-dom/vitest";
```

```bash
cd web && pnpm install
```

- [ ] **Step 2: トークンと土台の CSS を写す**

`web/packages/ui/src/tokens/tokens.css`(モック版 19 の `index.html` の 10〜65 行目そのまま。色・影・文字・寸法と、暗い配色):

```css
:root {
  --ink: #18181b; --ink-2: #3f3f46; --muted: #63636b; --muted-2: #8b8b93;
  --line: #e4e4df; --line-input: #dcdcd6; --sunken: #f6f6f3; --sidebar: #f4f4f1; --surface: #ffffff; --raised: #ffffff;
  --accent: #3451d1; --accent-ink: #1e2c78; --accent-weak: #e3e7f8; --accent-faint: #f6f7fd; --on-accent: #ffffff;
  --success: #15803d; --danger: #b91c1c; --danger-weak: #fdecec; --warning: #854d0e; --warning-weak: #fbf0d9;
  --hover: #f1f1ee; --scrim: rgba(24, 24, 27, 0.42);
  --shadow-card: 0 1px 2px rgba(24, 24, 27, 0.06), 0 8px 24px rgba(24, 24, 27, 0.06);
  --shadow-pop: 0 8px 24px rgba(24, 24, 27, 0.16); --shadow-dialog: 0 24px 60px rgba(24, 24, 27, 0.3);
  /* value colors: bg / fg */
  --c-gray-bg: #eeeeec; --c-gray-fg: #3f3f46; --c-blue-bg: #e3e7f8; --c-blue-fg: #26399a; --c-teal-bg: #e3f3f0; --c-teal-fg: #0b6b61;
  --c-green-bg: #e7f4ea; --c-green-fg: #14532d; --c-olive-bg: #ecfccb; --c-olive-fg: #3f6212; --c-amber-bg: #fbf0d9; --c-amber-fg: #854d0e;
  --c-orange-bg: #fbebdd; --c-orange-fg: #9a4307; --c-red-bg: #fdecec; --c-red-fg: #991b1b; --c-pink-bg: #fce7f3; --c-pink-fg: #9d174d; --c-purple-bg: #eee8fb; --c-purple-fg: #5b33b8;
  /* bars and dots */
  --c-gray-dot: #8b8b93; --c-blue-dot: #3451d1; --c-teal-dot: #0b6b61; --c-green-dot: #15803d; --c-olive-dot: #4d7c0f; --c-amber-dot: #a16207;
  --c-orange-dot: #c2410c; --c-red-dot: #b91c1c; --c-pink-dot: #be185d; --c-purple-dot: #6b3fd1;
  --c-slate-bg: #e5eaf0; --c-slate-fg: #334155; --c-slate-dot: #64748b; --c-brown-bg: #f5ebdd; --c-brown-fg: #78350f; --c-brown-dot: #92400e; --c-rose-bg: #ffe4e6; --c-rose-fg: #9f1239; --c-rose-dot: #e11d48; --c-magenta-bg: #fae8ff; --c-magenta-fg: #86198f; --c-magenta-dot: #c026d3; --c-indigo-bg: #e6e6fb; --c-indigo-fg: #3730a3; --c-indigo-dot: #4f46e5; --c-sky-bg: #e0f2fe; --c-sky-fg: #075985; --c-sky-dot: #0284c7; --c-cyan-bg: #dcf5f9; --c-cyan-fg: #155e75; --c-cyan-dot: #0891b2; --c-yellow-bg: #fdf8c8; --c-yellow-fg: #713f12; --c-yellow-dot: #ca8a04;
  /* store tones */
  --t-teal-bg: #e3f3f0; --t-teal-fg: #0b6b61; --t-orange-bg: #fbebdd; --t-orange-fg: #9a4307; --t-purple-bg: #eee8fb; --t-purple-fg: #5b33b8;
  --t-sky-bg: #e0f2fe; --t-sky-fg: #075985; --t-pink-bg: #fce7f3; --t-pink-fg: #9d174d; --t-olive-bg: #ecfccb; --t-olive-fg: #3f6212; --t-brown-bg: #f5ebdd; --t-brown-fg: #78350f;
  --a0-bg: #dde3fa; --a0-fg: #26399a; --a1-bg: #fce7d6; --a1-fg: #8a3a06; --a2-bg: #dcefe2; --a2-fg: #14532d; --a3-bg: #eee8fb; --a3-fg: #5b33b8;
  --k-epic: #6b3fd1; --k-story: #15803d; --k-task: #3451d1; --k-bug: #b91c1c; --k-step: #63636b; --k-project: #18181b;
  --cat-todo: #8b8b93; --cat-doing: #3451d1; --cat-done: #15803d; --cat-canceled: #c4c4c0;
  --font-sans: "IBM Plex Sans JP", system-ui, -apple-system, "Segoe UI", "Hiragino Sans", "Noto Sans JP", sans-serif;
  --font-mono: "JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  --sidebar-w: 248px; --radius: 8px; --control-h: 32px;
}
@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) {
  --ink: #ececea; --ink-2: #c9c9c5; --muted: #a1a19b; --muted-2: #7c7c77; --line: #2f2f2c; --line-input: #3a3a36; --sunken: #1b1b19; --sidebar: #161615; --surface: #1f1f1d; --raised: #262624;
  --accent: #8b9cf0; --accent-ink: #c7d0fb; --accent-weak: #2a3060; --accent-faint: #22243a; --on-accent: #10132c;
  --success: #6fcf8c; --danger: #f08b8b; --danger-weak: #3d2020; --warning: #e7c06a; --warning-weak: #3a2f17; --hover: #2a2a27; --scrim: rgba(0, 0, 0, 0.6);
  --shadow-card: 0 1px 2px rgba(0, 0, 0, 0.4); --shadow-pop: 0 8px 24px rgba(0, 0, 0, 0.5); --shadow-dialog: 0 24px 60px rgba(0, 0, 0, 0.6);
  --c-gray-bg: #2e2e2b; --c-gray-fg: #c9c9c5; --c-blue-bg: #263063; --c-blue-fg: #c3cdfa; --c-teal-bg: #163a36; --c-teal-fg: #8fdcd0; --c-green-bg: #183823; --c-green-fg: #9be2b0;
  --c-olive-bg: #2b3a14; --c-olive-fg: #c4e58a; --c-amber-bg: #3a2f17; --c-amber-fg: #f1cf7e; --c-orange-bg: #40260f; --c-orange-fg: #f6b27f; --c-red-bg: #3d2020; --c-red-fg: #f5a3a3;
  --c-pink-bg: #3c1a2c; --c-pink-fg: #f4a6cc; --c-purple-bg: #2f2350; --c-purple-fg: #cdb8fb;
  --c-gray-dot: #8f8f8a; --c-blue-dot: #8b9cf0; --c-teal-dot: #4fb8a9; --c-green-dot: #5cc27c; --c-olive-dot: #9ccb4f; --c-amber-dot: #e0b04a; --c-orange-dot: #ee8f4f; --c-red-dot: #ee7474; --c-pink-dot: #ec7fb2; --c-purple-dot: #a88bf3;
  --c-slate-bg: #262c35; --c-slate-fg: #c3ccd8; --c-slate-dot: #8a97a8; --c-brown-bg: #3a2a18; --c-brown-fg: #e9bf8a; --c-brown-dot: #c98d55; --c-rose-bg: #401a22; --c-rose-fg: #f8a5b4; --c-rose-dot: #f26b84; --c-magenta-bg: #3a1840; --c-magenta-fg: #eeaaf7; --c-magenta-dot: #d877ea; --c-indigo-bg: #25234d; --c-indigo-fg: #c0bdf7; --c-indigo-dot: #8f8af0; --c-sky-bg: #10324a; --c-sky-fg: #93cff5; --c-sky-dot: #5bb6ee; --c-cyan-bg: #123840; --c-cyan-fg: #8ddcec; --c-cyan-dot: #4cc3db; --c-yellow-bg: #3a3510; --c-yellow-fg: #f3e27a; --c-yellow-dot: #e5c83a;
  --t-teal-bg: #163a36; --t-teal-fg: #8fdcd0; --t-orange-bg: #40260f; --t-orange-fg: #f6b27f; --t-purple-bg: #2f2350; --t-purple-fg: #cdb8fb; --t-sky-bg: #10324a; --t-sky-fg: #93cff5;
  --t-pink-bg: #3c1a2c; --t-pink-fg: #f4a6cc; --t-olive-bg: #2b3a14; --t-olive-fg: #c4e58a; --t-brown-bg: #3a2a18; --t-brown-fg: #e9bf8a;
  --a0-bg: #263063; --a0-fg: #c3cdfa; --a1-bg: #40260f; --a1-fg: #f6b27f; --a2-bg: #183823; --a2-fg: #9be2b0; --a3-bg: #2f2350; --a3-fg: #cdb8fb;
  --k-epic: #a88bf3; --k-story: #5cc27c; --k-task: #8b9cf0; --k-bug: #ee7474; --k-step: #a1a19b; --k-project: #ececea;
  --cat-todo: #8f8f8a; --cat-doing: #8b9cf0; --cat-done: #5cc27c; --cat-canceled: #55554f; color-scheme: dark; } }
:root[data-theme="dark"] {
  --ink: #ececea; --ink-2: #c9c9c5; --muted: #a1a19b; --muted-2: #7c7c77; --line: #2f2f2c; --line-input: #3a3a36; --sunken: #1b1b19; --sidebar: #161615; --surface: #1f1f1d; --raised: #262624;
  --accent: #8b9cf0; --accent-ink: #c7d0fb; --accent-weak: #2a3060; --accent-faint: #22243a; --on-accent: #10132c;
  --success: #6fcf8c; --danger: #f08b8b; --danger-weak: #3d2020; --warning: #e7c06a; --warning-weak: #3a2f17; --hover: #2a2a27; --scrim: rgba(0, 0, 0, 0.6);
  --shadow-card: 0 1px 2px rgba(0, 0, 0, 0.4); --shadow-pop: 0 8px 24px rgba(0, 0, 0, 0.5); --shadow-dialog: 0 24px 60px rgba(0, 0, 0, 0.6);
  --c-gray-bg: #2e2e2b; --c-gray-fg: #c9c9c5; --c-blue-bg: #263063; --c-blue-fg: #c3cdfa; --c-teal-bg: #163a36; --c-teal-fg: #8fdcd0; --c-green-bg: #183823; --c-green-fg: #9be2b0;
  --c-olive-bg: #2b3a14; --c-olive-fg: #c4e58a; --c-amber-bg: #3a2f17; --c-amber-fg: #f1cf7e; --c-orange-bg: #40260f; --c-orange-fg: #f6b27f; --c-red-bg: #3d2020; --c-red-fg: #f5a3a3;
  --c-pink-bg: #3c1a2c; --c-pink-fg: #f4a6cc; --c-purple-bg: #2f2350; --c-purple-fg: #cdb8fb;
  --c-gray-dot: #8f8f8a; --c-blue-dot: #8b9cf0; --c-teal-dot: #4fb8a9; --c-green-dot: #5cc27c; --c-olive-dot: #9ccb4f; --c-amber-dot: #e0b04a; --c-orange-dot: #ee8f4f; --c-red-dot: #ee7474; --c-pink-dot: #ec7fb2; --c-purple-dot: #a88bf3;
  --c-slate-bg: #262c35; --c-slate-fg: #c3ccd8; --c-slate-dot: #8a97a8; --c-brown-bg: #3a2a18; --c-brown-fg: #e9bf8a; --c-brown-dot: #c98d55; --c-rose-bg: #401a22; --c-rose-fg: #f8a5b4; --c-rose-dot: #f26b84; --c-magenta-bg: #3a1840; --c-magenta-fg: #eeaaf7; --c-magenta-dot: #d877ea; --c-indigo-bg: #25234d; --c-indigo-fg: #c0bdf7; --c-indigo-dot: #8f8af0; --c-sky-bg: #10324a; --c-sky-fg: #93cff5; --c-sky-dot: #5bb6ee; --c-cyan-bg: #123840; --c-cyan-fg: #8ddcec; --c-cyan-dot: #4cc3db; --c-yellow-bg: #3a3510; --c-yellow-fg: #f3e27a; --c-yellow-dot: #e5c83a;
  --t-teal-bg: #163a36; --t-teal-fg: #8fdcd0; --t-orange-bg: #40260f; --t-orange-fg: #f6b27f; --t-purple-bg: #2f2350; --t-purple-fg: #cdb8fb; --t-sky-bg: #10324a; --t-sky-fg: #93cff5;
  --t-pink-bg: #3c1a2c; --t-pink-fg: #f4a6cc; --t-olive-bg: #2b3a14; --t-olive-fg: #c4e58a; --t-brown-bg: #3a2a18; --t-brown-fg: #e9bf8a;
  --a0-bg: #263063; --a0-fg: #c3cdfa; --a1-bg: #40260f; --a1-fg: #f6b27f; --a2-bg: #183823; --a2-fg: #9be2b0; --a3-bg: #2f2350; --a3-fg: #cdb8fb;
  --k-epic: #a88bf3; --k-story: #5cc27c; --k-task: #8b9cf0; --k-bug: #ee7474; --k-step: #a1a19b; --k-project: #ececea;
  --cat-todo: #8f8f8a; --cat-doing: #8b9cf0; --cat-done: #5cc27c; --cat-canceled: #55554f; color-scheme: dark; }
```

`web/packages/ui/src/tokens/base.css`(モックの 66〜75 行目):

```css
*,
*::before,
*::after {
  box-sizing: border-box;
}
html,
body {
  height: 100%;
}
body {
  margin: 0;
  background: var(--surface);
  color: var(--ink);
  font-family: var(--font-sans);
  font-size: 13px;
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
}
button,
input,
select,
textarea {
  font: inherit;
  color: inherit;
}
button {
  cursor: pointer;
}
code,
kbd {
  font-family: var(--font-mono);
  font-size: 11.5px;
}
a {
  color: var(--accent);
}
:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
@media (prefers-reduced-motion: reduce) {
  * {
    transition: none !important;
    animation: none !important;
  }
}
#app {
  height: 100%;
}
```

- [ ] **Step 3: 失敗するテストを書く**

`web/packages/ui/src/test-snippet.ts`(テストで children を渡すための小さな道具。テストのファイルからだけ使う):

```ts
import { createRawSnippet } from "svelte";

export function text(value: string) {
  return createRawSnippet(() => ({ render: () => `<span>${value}</span>` }));
}
```

`web/packages/ui/src/primitives/primitives.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { text } from "../test-snippet";
import Button from "./Button.svelte";
import IconButton from "./IconButton.svelte";
import Tabs from "./Tabs.svelte";

describe("Button", () => {
  it("calls onclick", async () => {
    const onclick = vi.fn();
    render(Button, { props: { variant: "primary", onclick, children: text("保存") } });
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(onclick).toHaveBeenCalledOnce();
  });

  it("does not call onclick when disabled, and shows the note", async () => {
    const onclick = vi.fn();
    render(Button, {
      props: { disabled: true, title: "閲覧のみの Store です", onclick, children: text("作成") },
    });
    const button = screen.getByRole("button", { name: "作成" });
    await fireEvent.click(button);
    expect(onclick).not.toHaveBeenCalled();
    expect(button).toHaveAttribute("title", "閲覧のみの Store です");
  });
});

describe("IconButton", () => {
  it("is named by its label", () => {
    render(IconButton, { props: { label: "メニュー", children: text("≡") } });
    expect(screen.getByRole("button", { name: "メニュー" })).toBeInTheDocument();
  });
});

describe("Tabs", () => {
  it("marks the current tab and reports the selected one", async () => {
    const onselect = vi.fn();
    const tabs = [
      { id: "tree", label: "ツリー" },
      { id: "board", label: "カンバン", count: 3 },
    ];
    render(Tabs, { props: { tabs, current: "tree", onselect } });
    expect(screen.getByRole("tab", { name: "ツリー" })).toHaveAttribute("aria-selected", "true");
    await fireEvent.click(screen.getByRole("tab", { name: /カンバン/ }));
    expect(onselect).toHaveBeenCalledWith("board");
  });
});
```

`web/packages/ui/src/primitives/overlays.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { text } from "../test-snippet";
import Dialog from "./Dialog.svelte";
import Menu from "./Menu.svelte";
import Toasts from "./Toasts.svelte";

describe("Dialog", () => {
  it("renders nothing when closed", () => {
    render(Dialog, { props: { open: false, title: "削除", onclose: vi.fn(), children: text("本文") } });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("closes on Escape and on a press outside, not inside", async () => {
    const onclose = vi.fn();
    render(Dialog, { props: { open: true, title: "削除", onclose, children: text("本文") } });
    const dialog = screen.getByRole("dialog", { name: "削除" });
    await fireEvent.mouseDown(dialog);
    expect(onclose).not.toHaveBeenCalled();
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledTimes(1);
    await fireEvent.mouseDown(dialog.parentElement as HTMLElement);
    expect(onclose).toHaveBeenCalledTimes(2);
  });
});

describe("Menu", () => {
  const items = [
    { id: "copy", label: "ID をコピー", hint: "C" },
    { separator: true as const },
    { id: "delete", label: "削除", danger: true, disabled: true },
  ];

  it("reports the selected item and skips a disabled one", async () => {
    const onselect = vi.fn();
    render(Menu, { props: { open: true, x: 10, y: 10, items, onselect, onclose: vi.fn() } });
    await fireEvent.click(screen.getByRole("menuitem", { name: /ID をコピー/ }));
    await fireEvent.click(screen.getByRole("menuitem", { name: /削除/ }));
    expect(onselect).toHaveBeenCalledExactlyOnceWith("copy");
  });

  it("closes on Escape", async () => {
    const onclose = vi.fn();
    render(Menu, { props: { open: true, x: 0, y: 0, items, onselect: vi.fn(), onclose } });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledOnce();
  });
});

describe("Toasts", () => {
  it("reports dismiss and action", async () => {
    const ondismiss = vi.fn();
    const onaction = vi.fn();
    const toasts = [{ id: "t1", text: "保存しました", action: "開く" }];
    render(Toasts, { props: { toasts, ondismiss, onaction } });
    expect(screen.getByRole("status")).toHaveTextContent("保存しました");
    await fireEvent.click(screen.getByRole("button", { name: "開く" }));
    await fireEvent.click(screen.getByRole("button", { name: "閉じる" }));
    expect(onaction).toHaveBeenCalledWith("t1");
    expect(ondismiss).toHaveBeenCalledWith("t1");
  });
});
```

`web/packages/ui/src/layout/layout.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { text } from "../test-snippet";
import AppFrame from "./AppFrame.svelte";
import PageHead from "./PageHead.svelte";

describe("AppFrame", () => {
  it("renders the sidebar and the main area", () => {
    render(AppFrame, {
      props: { drawerOpen: false, onclosedrawer: vi.fn(), sidebar: text("サイド"), children: text("本体") },
    });
    expect(screen.getByRole("complementary")).toHaveTextContent("サイド");
    expect(screen.getByRole("main")).toHaveTextContent("本体");
  });

  it("closes the drawer when the scrim is pressed", async () => {
    const onclosedrawer = vi.fn();
    render(AppFrame, {
      props: { drawerOpen: true, onclosedrawer, sidebar: text("サイド"), children: text("本体") },
    });
    await fireEvent.click(screen.getByRole("button", { name: "メニューを閉じる" }));
    expect(onclosedrawer).toHaveBeenCalledOnce();
  });
});

describe("PageHead", () => {
  it("shows the title and reports the drawer button", async () => {
    const ondrawer = vi.fn();
    render(PageHead, { props: { title: "マイタスク", ondrawer } });
    expect(screen.getByRole("heading", { name: "マイタスク" })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "メニュー" }));
    expect(ondrawer).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 4: テストが落ちることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/ui test`
Expected: FAIL(`./Button.svelte` などが見つからない)

- [ ] **Step 5: 型と基本部品を書く**

`web/packages/ui/src/types.ts`:

```ts
export interface SelectOption {
  value: string;
  label: string;
}

export interface Tab {
  id: string;
  label: string;
  count?: number;
}

export type MenuItem =
  | { id: string; label: string; hint?: string; danger?: boolean; disabled?: boolean }
  | { separator: true };

export interface Toast {
  id: string;
  text: string;
  error?: boolean;
  action?: string;
}
```

`web/packages/ui/src/primitives/Button.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    variant?: "default" | "primary" | "danger" | "ghost";
    size?: "md" | "sm" | "xs";
    type?: "button" | "submit";
    disabled?: boolean;
    title?: string;
    onclick?: (event: MouseEvent) => void;
    children: Snippet;
  }

  let {
    variant = "default",
    size = "md",
    type = "button",
    disabled = false,
    title,
    onclick,
    children,
  }: Props = $props();
</script>

<button {type} class="btn {variant} {size}" {disabled} {title} {onclick}>{@render children()}</button>

<style>
  .btn {
    height: var(--control-h);
    padding: 0 12px;
    border: 1px solid var(--line-input);
    background: var(--surface);
    border-radius: 7px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
    color: var(--ink);
  }
  .btn:hover { background: var(--hover); }
  .btn:disabled { opacity: 0.45; cursor: not-allowed; }
  .primary { background: var(--accent); border-color: var(--accent); color: var(--on-accent); }
  .primary:hover { filter: brightness(1.08); background: var(--accent); }
  .danger { color: var(--danger); border-color: color-mix(in srgb, var(--danger) 40%, var(--line-input)); }
  .ghost { border-color: transparent; background: transparent; color: var(--muted); }
  .ghost:hover { color: var(--ink); background: var(--hover); }
  .sm { height: 28px; padding: 0 10px; font-size: 12.5px; }
  .xs { height: 24px; padding: 0 8px; font-size: 12px; }
</style>
```

`web/packages/ui/src/primitives/IconButton.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    label: string;
    size?: "md" | "sm" | "xs";
    active?: boolean;
    disabled?: boolean;
    title?: string;
    onclick?: (event: MouseEvent) => void;
    children: Snippet;
  }

  let { label, size = "md", active = false, disabled = false, title, onclick, children }: Props =
    $props();
</script>

<button
  type="button"
  class="icon-btn {size}"
  class:on={active}
  aria-label={label}
  {disabled}
  {title}
  {onclick}>{@render children()}</button
>

<style>
  .icon-btn {
    width: 30px;
    height: 30px;
    border: 0;
    background: transparent;
    border-radius: 6px;
    display: inline-grid;
    place-items: center;
    color: var(--muted);
    flex: none;
  }
  .icon-btn:hover,
  .on { background: var(--hover); color: var(--ink); }
  .icon-btn:disabled { opacity: 0.45; cursor: not-allowed; }
  .sm { width: 26px; height: 26px; }
  .xs { width: 22px; height: 22px; }
</style>
```

`web/packages/ui/src/primitives/Input.svelte`:

```svelte
<script lang="ts">
  interface Props {
    value?: string;
    label: string;
    placeholder?: string;
    size?: "md" | "sm";
    mono?: boolean;
    disabled?: boolean;
    title?: string;
  }

  let {
    value = $bindable(""),
    label,
    placeholder,
    size = "md",
    mono = false,
    disabled = false,
    title,
  }: Props = $props();
</script>

<input class="input {size}" class:mono aria-label={label} {placeholder} {disabled} {title} bind:value />

<style>
  .input {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .input:focus { outline: 2px solid var(--accent); outline-offset: -1px; border-color: transparent; }
  .input:disabled { opacity: 0.45; cursor: not-allowed; }
  .sm { height: 28px; font-size: 12.5px; }
  .mono { font-family: var(--font-mono); }
</style>
```

`web/packages/ui/src/primitives/Select.svelte`:

```svelte
<script lang="ts">
  import type { SelectOption } from "../types";

  interface Props {
    value?: string;
    label: string;
    options: SelectOption[];
    size?: "md" | "sm";
    disabled?: boolean;
    title?: string;
  }

  let { value = $bindable(""), label, options, size = "md", disabled = false, title }: Props =
    $props();
</script>

<select class="select {size}" aria-label={label} {disabled} {title} bind:value>
  {#each options as option (option.value)}
    <option value={option.value}>{option.label}</option>
  {/each}
</select>

<style>
  .select {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .select:focus { outline: 2px solid var(--accent); outline-offset: -1px; border-color: transparent; }
  .select:disabled { opacity: 0.45; cursor: not-allowed; }
  .sm { height: 28px; font-size: 12.5px; }
</style>
```

`web/packages/ui/src/primitives/Switch.svelte`:

```svelte
<script lang="ts">
  interface Props {
    checked?: boolean;
    label: string;
    disabled?: boolean;
    title?: string;
  }

  let { checked = $bindable(false), label, disabled = false, title }: Props = $props();
</script>

<span class="switch" {title}>
  <input type="checkbox" role="switch" aria-label={label} {disabled} bind:checked />
  <span></span>
</span>

<style>
  .switch { position: relative; width: 34px; height: 20px; flex: none; display: inline-block; }
  input { opacity: 0; width: 100%; height: 100%; margin: 0; position: absolute; cursor: pointer; z-index: 1; }
  input:disabled { cursor: not-allowed; }
  span span { position: absolute; inset: 0; border-radius: 999px; background: var(--line-input); transition: background 0.15s; }
  span span::after {
    content: "";
    position: absolute;
    width: 16px;
    height: 16px;
    left: 2px;
    top: 2px;
    border-radius: 50%;
    background: var(--surface);
    transition: transform 0.15s;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  }
  input:checked + span { background: var(--accent); }
  input:checked + span::after { transform: translateX(14px); }
  input:focus-visible + span { outline: 2px solid var(--accent); outline-offset: 2px; }
  input:disabled + span { opacity: 0.45; }
</style>
```

`web/packages/ui/src/primitives/Tabs.svelte`:

```svelte
<script lang="ts">
  import type { Tab } from "../types";

  interface Props {
    tabs: Tab[];
    current: string;
    onselect: (id: string) => void;
  }

  let { tabs, current, onselect }: Props = $props();
</script>

<div class="tabs" role="tablist">
  {#each tabs as tab (tab.id)}
    <button
      type="button"
      role="tab"
      class="tab"
      class:on={tab.id === current}
      aria-selected={tab.id === current}
      onclick={() => onselect(tab.id)}
    >
      {tab.label}
      {#if tab.count !== undefined}<span class="count">{tab.count}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: 2px;
    padding: 0 24px;
    border-bottom: 1px solid var(--line);
    overflow-x: auto;
    scrollbar-width: none;
    flex: none;
  }
  .tab {
    border: 0;
    background: transparent;
    padding: 8px 10px 9px;
    color: var(--muted);
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
    white-space: nowrap;
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .tab:hover { color: var(--ink); }
  .on { color: var(--ink); border-bottom-color: var(--accent); font-weight: 500; }
  .count {
    font-size: 11px;
    background: var(--accent);
    color: var(--on-accent);
    border-radius: 999px;
    padding: 0 6px;
    font-variant-numeric: tabular-nums;
  }
  @container frame (max-width: 860px) {
    .tabs { padding: 0 max(16px, env(safe-area-inset-right, 0px)) 0 max(16px, env(safe-area-inset-left, 0px)); }
  }
</style>
```

- [ ] **Step 6: 重ねて出す部品を書く**

`web/packages/ui/src/primitives/Dialog.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";
  import IconButton from "./IconButton.svelte";

  interface Props {
    open: boolean;
    title: string;
    wide?: boolean;
    onclose: () => void;
    children: Snippet;
    footer?: Snippet;
  }

  let { open, title, wide = false, onclose, children, footer }: Props = $props();
  const titleId = $props.id();

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape") onclose();
  }

  // ダイアログの外(暗くした背景)を押したときだけ閉じる。
  function onmousedown(event: MouseEvent) {
    if (event.target === event.currentTarget) onclose();
  }
</script>

<svelte:window {onkeydown} />

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="scrim" {onmousedown}>
    <div class="dialog" class:wide role="dialog" aria-modal="true" aria-labelledby={titleId}>
      <div class="dlg-head">
        <h2 id={titleId}>{title}</h2>
        <IconButton label="閉じる" size="sm" onclick={onclose}>×</IconButton>
      </div>
      <div class="dlg-body">{@render children()}</div>
      {#if footer}<div class="dlg-foot">{@render footer()}</div>{/if}
    </div>
  </div>
{/if}

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: var(--scrim);
    z-index: 40;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: max(44px, env(safe-area-inset-top, 0px)) 16px 16px;
    overflow-y: auto;
  }
  .dialog {
    background: var(--surface);
    border-radius: 12px;
    box-shadow: var(--shadow-dialog);
    width: min(480px, 100%);
    display: flex;
    flex-direction: column;
    max-height: calc(100vh - 80px);
  }
  .wide { width: min(720px, 100%); }
  .dlg-head { display: flex; align-items: center; padding: 14px 16px 8px 20px; }
  .dlg-head h2 { margin: 0; font-size: 15px; flex: 1; }
  .dlg-body { padding: 4px 20px 12px; display: flex; flex-direction: column; gap: 12px; overflow-y: auto; }
  .dlg-foot { display: flex; gap: 8px; align-items: center; padding: 12px 16px; border-top: 1px solid var(--line); }
</style>
```

`web/packages/ui/src/primitives/Menu.svelte`:

```svelte
<script lang="ts">
  import type { MenuItem } from "../types";

  interface Props {
    open: boolean;
    x: number;
    y: number;
    items: MenuItem[];
    onselect: (id: string) => void;
    onclose: () => void;
  }

  let { open, x, y, items, onselect, onclose }: Props = $props();
  let element: HTMLDivElement | undefined = $state();

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape") onclose();
  }

  // メニューの外を押したら閉じる。
  function onpointerdown(event: PointerEvent) {
    if (open && element && !element.contains(event.target as Node)) onclose();
  }
</script>

<svelte:window {onkeydown} {onpointerdown} />

{#if open}
  <div class="menu-pop" role="menu" style:left="{x}px" style:top="{y}px" bind:this={element}>
    {#each items as item, i (i)}
      {#if "separator" in item}
        <div class="menu-sep" role="separator"></div>
      {:else}
        <button
          type="button"
          role="menuitem"
          class="menu-item"
          class:danger={item.danger}
          disabled={item.disabled}
          onclick={() => onselect(item.id)}
        >
          <span class="menu-label">{item.label}</span>
          {#if item.hint}<span class="menu-hint">{item.hint}</span>{/if}
        </button>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .menu-pop {
    position: fixed;
    z-index: 60;
    background: var(--raised);
    border: 1px solid var(--line);
    border-radius: 10px;
    box-shadow: var(--shadow-pop);
    padding: 4px;
    display: flex;
    flex-direction: column;
    max-height: 420px;
    overflow-y: auto;
  }
  .menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    border: 0;
    background: transparent;
    padding: 6px 8px;
    border-radius: 6px;
    text-align: left;
    min-height: 32px;
  }
  .menu-item:hover:not(:disabled) { background: var(--hover); }
  .menu-item:disabled { opacity: 0.45; cursor: not-allowed; }
  .danger { color: var(--danger); }
  .menu-label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .menu-hint { color: var(--muted); font-size: 11.5px; white-space: nowrap; }
  .menu-sep { height: 1px; background: var(--line); margin: 4px 0; }
</style>
```

`web/packages/ui/src/primitives/Toasts.svelte`:

```svelte
<script lang="ts">
  import type { Toast } from "../types";

  interface Props {
    toasts: Toast[];
    ondismiss: (id: string) => void;
    onaction?: (id: string) => void;
  }

  let { toasts, ondismiss, onaction }: Props = $props();
</script>

<div class="toasts">
  {#each toasts as toast (toast.id)}
    <div class="toast" class:error={toast.error} role="status">
      <span>{toast.text}</span>
      {#if toast.action}
        <button type="button" class="toast-btn" onclick={() => onaction?.(toast.id)}>{toast.action}</button>
      {/if}
      <button type="button" class="toast-x" aria-label="閉じる" onclick={() => ondismiss(toast.id)}>×</button>
    </div>
  {/each}
</div>

<style>
  .toasts {
    pointer-events: none;
    position: fixed;
    bottom: max(16px, env(safe-area-inset-bottom, 0px));
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 80;
    width: min(560px, calc(100vw - 32px));
  }
  .toast {
    pointer-events: auto;
    background: var(--ink);
    color: var(--surface);
    border-radius: 8px;
    padding: 8px 8px 8px 14px;
    display: flex;
    align-items: center;
    gap: 10px;
    box-shadow: var(--shadow-pop);
    font-size: 12.5px;
  }
  .toast span { flex: 1; white-space: pre-line; }
  .error { background: var(--danger); }
  .toast-btn { border: 0; background: transparent; color: inherit; font-weight: 600; text-decoration: underline; }
  .toast-x { border: 0; background: transparent; color: inherit; opacity: 0.7; font-size: 16px; width: 26px; }
</style>
```

- [ ] **Step 7: 全体の枠を書く**

`web/packages/ui/src/layout/AppFrame.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    drawerOpen: boolean;
    onclosedrawer: () => void;
    sidebar: Snippet;
    children: Snippet;
  }

  let { drawerOpen, onclosedrawer, sidebar, children }: Props = $props();
</script>

<!-- 枠の幅で形を変える(コンテナクエリ)。860px 以下では、サイドバーを引き出しにする。 -->
<div class="frame">
  <div class="app" class:drawer-open={drawerOpen}>
    <aside class="side">{@render sidebar()}</aside>
    <button type="button" class="side-scrim" aria-label="メニューを閉じる" onclick={onclosedrawer}></button>
    <main class="main">{@render children()}</main>
  </div>
</div>

<style>
  .frame { container: frame / inline-size; height: 100%; }
  .app { display: grid; grid-template-columns: var(--sidebar-w) minmax(0, 1fr); height: 100%; }
  .side {
    background: var(--sidebar);
    border-right: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px 10px 10px max(10px, env(safe-area-inset-left, 0px));
    overflow-y: auto;
    min-height: 0;
  }
  .side-scrim { display: none; border: 0; padding: 0; }
  .main { display: flex; flex-direction: column; min-width: 0; min-height: 0; height: 100%; }
  @container frame (max-width: 860px) {
    .app { grid-template-columns: minmax(0, 1fr); }
    .side {
      position: fixed;
      top: 0;
      bottom: 0;
      left: 0;
      width: min(290px, 86vw);
      z-index: 50;
      transform: translateX(-102%);
      transition: transform 0.18s;
      padding-top: max(12px, env(safe-area-inset-top, 0px));
    }
    .drawer-open .side { transform: none; box-shadow: var(--shadow-dialog); }
    .drawer-open .side-scrim { display: block; position: fixed; inset: 0; background: var(--scrim); z-index: 49; }
  }
</style>
```

`web/packages/ui/src/layout/PageHead.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";
  import IconButton from "../primitives/IconButton.svelte";

  interface Props {
    title: string;
    ondrawer: () => void;
    crumbs?: Snippet;
    actions?: Snippet;
  }

  let { title, ondrawer, crumbs, actions }: Props = $props();
</script>

<header class="page-head">
  <span class="drawer-btn">
    <IconButton label="メニュー" onclick={ondrawer}>
      <svg class="ico" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </IconButton>
  </span>
  <div class="head-main">
    {#if crumbs}<div class="crumbs">{@render crumbs()}</div>{/if}
    <h1 class="title">{title}</h1>
  </div>
  {#if actions}<div class="head-actions">{@render actions()}</div>{/if}
</header>

<style>
  .page-head {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 18px max(24px, env(safe-area-inset-right, 0px)) 10px 24px;
  }
  .head-main { flex: 1; min-width: 0; }
  .crumbs { display: flex; align-items: center; gap: 6px; color: var(--muted); font-size: 12px; flex-wrap: wrap; }
  .title {
    margin: 2px 0 0;
    font-size: 20px;
    font-weight: 600;
    letter-spacing: 0.005em;
    line-height: 1.35;
    text-wrap: balance;
  }
  .head-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
  .drawer-btn { display: none; }
  .ico { stroke: currentColor; fill: none; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
  @container frame (max-width: 860px) {
    .page-head {
      padding: 12px max(16px, env(safe-area-inset-right, 0px)) 8px max(16px, env(safe-area-inset-left, 0px));
      flex-wrap: wrap;
    }
    .head-actions { width: 100%; justify-content: flex-start; }
    .drawer-btn { display: inline-grid; margin-left: -6px; }
  }
</style>
```

jsdom はコンテナクエリを解釈しないので、`PageHead` のテストではメニューのボタンが押せる。幅による表示の切り替えは Task 12 の Playwright で確かめる。

`web/packages/ui/src/layout/Brand.svelte`:

```svelte
<div class="side-top">
  <span class="logo" aria-hidden="true">hh</span>
  <span class="brand">hedgehug</span>
</div>

<style>
  .side-top { display: flex; align-items: center; gap: 8px; padding: 2px 4px 4px; }
  .logo {
    width: 28px;
    height: 28px;
    border-radius: 8px;
    background: var(--ink);
    color: var(--surface);
    display: grid;
    place-items: center;
    font-family: var(--font-mono);
    font-weight: 500;
    font-size: 12px;
  }
  .brand { font-weight: 600; font-size: 14px; letter-spacing: 0.01em; }
</style>
```

`web/packages/ui/src/index.ts`:

```ts
export { default as AppFrame } from "./layout/AppFrame.svelte";
export { default as Brand } from "./layout/Brand.svelte";
export { default as PageHead } from "./layout/PageHead.svelte";
export { default as Button } from "./primitives/Button.svelte";
export { default as Dialog } from "./primitives/Dialog.svelte";
export { default as IconButton } from "./primitives/IconButton.svelte";
export { default as Input } from "./primitives/Input.svelte";
export { default as Menu } from "./primitives/Menu.svelte";
export { default as Select } from "./primitives/Select.svelte";
export { default as Switch } from "./primitives/Switch.svelte";
export { default as Tabs } from "./primitives/Tabs.svelte";
export { default as Toasts } from "./primitives/Toasts.svelte";
export type { MenuItem, SelectOption, Tab, Toast } from "./types";
```

- [ ] **Step 8: テストと検査が通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/ui test && pnpm --filter @hedgehug/ui check && pnpm lint && pnpm deps && pnpm size`
Expected: PASS、svelte-check の警告 0、Biome 0 件、ui が Svelte だけを使っている、どのコンポーネントも 200 行以下。

Biome が `.svelte` の CSS の 1 行の書き方を直すよう言ったら `pnpm fix` で直す(書式だけで、規則は変わらない)。`test-snippet.ts` は `.test.ts` で終わらないので dependency-cruiser の `ui-imports-only-svelte` の対象になるが、`svelte` だけを import しているので通る。

- [ ] **Step 9: Commit**

```bash
git add web/pnpm-lock.yaml web/packages/ui
git commit -m "feat(web): ui のトークン・基本部品・全体の枠を足す(見た目はモック版 19)"
```

---

### Task 12: 画面の app と通しのテスト、ci-frontend

core と api と ui を結び付ける `app` を作り、Go のバイナリに埋め込んで、e2e 用のビルドを Playwright で 3 つの幅で確かめる。最後に ci-frontend を入れる。

**Files:**
- Create: `web/app/package.json`、`tsconfig.json`、`svelte.config.js`、`vite.config.ts`、`index.html`
- Create: `web/app/src/main.ts`、`src/wire.ts`、`src/App.svelte`
- Create: `web/playwright.config.ts`、`web/e2e/frame.spec.ts`
- Modify: `web/package.json`(`e2e` と `@playwright/test`)、`Makefile`(`web`・`build`・`e2e-bin`・`e2e`)
- Create: `.github/workflows/ci-frontend.yaml`

**Interfaces:**
- Consumes: Task 8 の e2e 用ビルド(`HEDGEHUG_ADDR`・`HEDGEHUG_TOKEN`、`web.Dist()`)、Task 9 の `createShell`・`createSystemInfo`、Task 10 の `createSystemPort`・`setToken`・`tokenFromHash`、Task 11 の `AppFrame`・`Brand`・`PageHead`
- Produces:
  - `web/app/src/wire.ts` の `shell`・`system`(組み立て。トラックは、自分の機能の状態をここで作って足す)
  - `web/app/src/App.svelte`(全体の枠。サイドバーと本体の中身は、トラックが `screens/<画面>/` を作って差し替える)
  - `make web`・`make build`・`make e2e-bin`・`make e2e`

**仮の画面:** 本体には見出し「hedgehug」と、サーバーから読んだ版だけを出す。トークン付きの API の呼び出しを通しで確かめるためのもので、トラック A が最初の画面を作るときに差し替える。

- [ ] **Step 1: app のパッケージを置く**

`web/app/package.json`:

```json
{
  "name": "@hedgehug/app",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "check": "svelte-check --tsconfig ./tsconfig.json --fail-on-warnings"
  },
  "dependencies": {
    "@fontsource/ibm-plex-sans-jp": "5.3.1",
    "@fontsource/jetbrains-mono": "5.3.0",
    "@hedgehug/api": "workspace:*",
    "@hedgehug/core": "workspace:*",
    "@hedgehug/ui": "workspace:*",
    "svelte": "5.57.3"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "7.3.1",
    "svelte-check": "4.7.7",
    "typescript": "6.0.3",
    "vite": "8.3.5"
  }
}
```

`web/app/tsconfig.json`:

```json
{
  "extends": "../tsconfig.base.json",
  "compilerOptions": {
    "types": ["svelte", "vite/client"]
  },
  "include": ["src", "vite.config.ts"]
}
```

`web/app/svelte.config.js`:

```js
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

export default {
  preprocess: vitePreprocess(),
};
```

`web/app/index.html`:

```html
<!doctype html>
<html lang="ja">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover" />
    <title>hedgehug</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

`web/app/vite.config.ts`(前回の `web/vite.config.ts` から、`keepGitkeep`・`woff2Only`・開発用の proxy を引き継ぐ。ライセンスの書き出しと CodeMirror の分割は、使うときに足す):

```ts
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig, type Plugin } from "vite";

// Go は web/dist を埋め込む(`//go:embed all:dist`)。git は dist/.gitkeep だけを持ち、make web の前でも
// go build が通るようにしている。Vite はビルドのたびに出力先を空にするので、.gitkeep を置き直す。
function keepGitkeep(): Plugin {
  let outDir = "";
  return {
    name: "hedgehug-keep-gitkeep",
    apply: "build",
    configResolved(config) {
      outDir = config.build.outDir;
    },
    closeBundle() {
      mkdirSync(outDir, { recursive: true });
      writeFileSync(join(outDir, ".gitkeep"), "");
    },
  };
}

// @fontsource は woff2 と woff を並べ、Vite は両方を写す。この画面を開くブラウザはどれも woff2 を読むので、
// woff を外して dist を woff2 だけにする。
const woff2Only = {
  postcssPlugin: "hedgehug-woff2-only",
  Declaration: {
    src(decl: { value: string }) {
      decl.value = decl.value.replace(/,\s*url\([^)]*\.woff\)\s*format\(["']woff["']\)/g, "");
    },
  },
};

// 開発中は、pnpm dev が画面を出し、/api を、起動したサーバーへ送る。HEDGEHUG_UI は、サーバーが出した URL
// (http://127.0.0.1:<番号>/#token=<トークン>)。
const ui = process.env.HEDGEHUG_UI ? new URL(process.env.HEDGEHUG_UI) : null;

export default defineConfig({
  plugins: [svelte(), keepGitkeep()],
  css: { postcss: { plugins: [woff2Only] } },
  build: {
    outDir: join(import.meta.dirname, "..", "dist"),
    emptyOutDir: true,
  },
  server: {
    open: ui ? `/${ui.hash}` : false,
    proxy: ui ? { "/api": { target: ui.origin, changeOrigin: true } } : undefined,
  },
});
```

`web/package.json` の `devDependencies` に `"@playwright/test": "1.64.1"`、`scripts` に `"e2e": "playwright test"` を足す。

```bash
cd web && pnpm install
```

- [ ] **Step 2: 組み立てと全体の枠を書く**

`web/app/src/wire.ts`:

```ts
import { createSystemPort } from "@hedgehug/api";
import { createShell, createSystemInfo } from "@hedgehug/core";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。
export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
```

`web/app/src/main.ts`:

```ts
import "@fontsource/ibm-plex-sans-jp/400.css";
import "@fontsource/ibm-plex-sans-jp/500.css";
import "@fontsource/ibm-plex-sans-jp/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@hedgehug/ui/tokens.css";
import "@hedgehug/ui/base.css";
import { setToken, tokenFromHash } from "@hedgehug/api";
import { mount } from "svelte";
import App from "./App.svelte";

setToken(tokenFromHash(location.hash) ?? "");

const target = document.getElementById("app");
if (!target) throw new Error("#app がありません");
mount(App, { target });
```

`web/app/src/App.svelte`:

```svelte
<script lang="ts">
  import { AppFrame, Brand, PageHead } from "@hedgehug/ui";
  import { shell, system } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  system.load();
</script>

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Brand />
  {/snippet}
  <PageHead title="hedgehug" ondrawer={shell.openDrawer} />
  <div class="content">
    {#if $version}<p class="version">{$version}</p>{/if}
  </div>
</AppFrame>

<style>
  .content { flex: 1; overflow: auto; padding: 16px 24px 40px; min-height: 0; }
  .version { color: var(--muted); font-family: var(--font-mono); margin: 0; }
</style>
```

- [ ] **Step 3: ビルドして、埋め込みを確かめる**

`Makefile` の `.PHONY` に `web e2e-bin e2e` を足し、`build` を次に置き換え、`web`・`e2e-bin`・`e2e` を足す。

```make
# 画面を web/dist にビルドする。バイナリはこれを埋め込む。
web:
	cd web && pnpm install --frozen-lockfile && pnpm build

# bin/ に、画面を埋め込んだバイナリを作る。
build: web
	go build -o bin/hedgehug ./cmd/hedgehug

# 通しのテストが動かすバイナリを作る(e2e タグ: 口に apptest の偽物とサンプルデータを差し込む)。
# バイナリは web/dist を埋め込むので、web を先に作る。
e2e-bin: web
	go build -tags e2e -o bin/hedgehug-e2e ./cmd/hedgehug

# Playwright で、スマホ・タブレット・デスクトップの 3 つの幅で通しで確かめる。
e2e: e2e-bin
	cd web && pnpm exec playwright install chromium
	cd web && pnpm e2e
```

Run: `make build && ls web/dist && cd web && pnpm --filter @hedgehug/app check`
Expected: `web/dist` に `index.html`・`assets/`・`.gitkeep` があり、`git status` で `web/dist` の中身が無視されている(`.gitkeep` だけが追跡されている)。svelte-check の警告 0。

- [ ] **Step 4: 通しのテストを書く**

`web/playwright.config.ts`:

```ts
import { defineConfig, devices } from "@playwright/test";

// e2e 用にビルドしたサーバー(bin/hedgehug-e2e。口に apptest の偽物とサンプルデータを差し込んだもの)を、
// 決めた番号とトークンで起動する。幅は、スマホ・タブレット・デスクトップの 3 つ(設計書 6 節)。
const addr = "127.0.0.1:4173";

export default defineConfig({
  testDir: "e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://${addr}`,
    trace: "retain-on-failure",
    // Playwright は、言わなければ Chromium のサンドボックスを切る。
    launchOptions: { chromiumSandbox: true },
  },
  webServer: {
    command: "../bin/hedgehug-e2e",
    url: `http://${addr}/`,
    env: { HEDGEHUG_ADDR: addr, HEDGEHUG_TOKEN: "e2e-token" },
    reuseExistingServer: !process.env.CI,
  },
  projects: [
    { name: "phone", use: { ...devices["Desktop Chrome"], viewport: { width: 390, height: 844 } } },
    { name: "tablet", use: { ...devices["Desktop Chrome"], viewport: { width: 820, height: 1180 } } },
    { name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 800 } } },
  ],
});
```

`web/e2e/frame.spec.ts`:

```ts
import { expect, test } from "@playwright/test";

const url = "/#token=e2e-token";

test("shows the frame and reads the server with the token", async ({ page }) => {
  await page.goto(url);
  await expect(page.getByRole("heading", { name: "hedgehug" })).toBeVisible();
  // e2e 用のビルドは -ldflags で版を入れないので "dev"。トークンが合わなければ 401 で出ない。
  await expect(page.getByText("dev", { exact: true })).toBeVisible();
});

test("the sidebar is a drawer only when the frame is 860px or narrower", async ({ page }, info) => {
  await page.goto(url);
  const sidebar = page.getByRole("complementary");
  const menu = page.getByRole("button", { name: "メニュー", exact: true });
  if (info.project.name === "desktop") {
    await expect(sidebar).toBeInViewport();
    await expect(menu).toBeHidden();
    return;
  }
  await expect(sidebar).not.toBeInViewport();
  await menu.click();
  await expect(sidebar).toBeInViewport();
  // 暗くした背景の、引き出しに重ならない右上を押す。
  const width = page.viewportSize()?.width ?? 0;
  await page.getByRole("button", { name: "メニューを閉じる" }).click({ position: { x: width - 10, y: 10 } });
  await expect(sidebar).not.toBeInViewport();
});
```

- [ ] **Step 5: 通しのテストを流す**

Run: `make e2e`
Expected: 6 件(2 つのテスト × 3 つの幅)が PASS。

- [ ] **Step 6: ci-frontend を写す**

```bash
git show cf58222d~1:.github/workflows/ci-frontend.yaml > .github/workflows/ci-frontend.yaml
```

次だけ変える。
1. `on.push.branches` を `[main, develop]` にする。
2. web ジョブの「Verify the API types are up to date」の `git diff --exit-code -- core/api/generated.ts` を `git diff --exit-code -- packages/api/src/generated.ts` にし、その上のコメントの `core/api/generated.ts` も同じに直す。
3. web ジョブの「Test」のあとに、次の 2 つを足す。
   ```yaml
      - name: Dependency rules
        run: pnpm deps

      - name: Size limits
        run: pnpm size
   ```

- [ ] **Step 7: 全体を確かめる**

Run: `make check && make e2e`
Expected: すべて成功

- [ ] **Step 8: Commit**

```bash
git add Makefile web/package.json web/pnpm-lock.yaml web/app web/playwright.config.ts web/e2e .github/workflows/ci-frontend.yaml
git commit -m "feat(web): 画面の全体の枠を組み立て、3 つの幅の通しのテストと ci-frontend を足す"
```

---

## 土台のあと

- 土台の最後の PR が develop に入ったら、トラック A・B・C・D の計画を書く(設計書 8 節。この計画の実装中に並べて書いてよい)。各計画の冒頭に、持ち場のフォルダと、待つトラックを書く。
- release のワークフローは、最初に配る前に、ライセンスの書き出しと一緒に足す(決定 11)。
