# E2E の読み取りの速さ 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** サーバーの読み取りの重さを減らし、E2E の worker どうしがサーバーを共有しないようにして、CI の E2E の時間を戻す。

**Architecture:** 製品側は振る舞いを変えずに 3 か所を軽くする。(1) view で「プロジェクト → ワークスペース」を覚える(アイテムの `places` と同じ形)。(2) `change.Resolve` が項目ごとにログ全体を走査しないよう、行をキーで 1 回だけ振り分ける。(3) `blockedSet` が、ブロック元のアイテムがあるワークスペースの設定だけを読む。E2E 側は、Playwright の worker ごとに e2e のサーバーを空いた番号で起動する fixture にし、固定の 4173 と `webServer` をやめる。

**Tech Stack:** Go(`internal/infra/gitstore`・`internal/app/apptest`・`internal/domain/change`・`internal/domain/relation`・`internal/app/items`)、Playwright(`web/e2e`)。

**Spec:** なし(性能の修正で、振る舞いを変えない)。調査の結果は下の「調べたこと」。

## 調べたこと

- CI の E2E: 165 件 1.8 分(2026-11-13 08:48)→ 216 件 5.8 分(13:03)。同じテストが phone 3.3 秒 → tablet 7.3 秒 → desktop 18.5 秒と、あとで流れるほど遅い。
- e2e のサーバー(`cmd/hedgehug/wire_e2e.go`)は全テストで 1 つの Store「e2e」を共有する。desktop 72 件は新しいサーバーで 29 秒、phone・tablet のあとのサーバーで 1.6 分。`GET /items/{id}` は 1ms 未満 → 約 48ms(ワークスペース 76・プロジェクト 58・アイテム 164)。
- 原因: `internal/app/items/view.go:72` の `blockedSet`(アイテムの取得・一覧で毎回呼ぶ)が、ワークスペースごとに `Projects().List(w.ID)` を呼ぶ。`List` は全プロジェクトをログから作り直して絞るので、1 回で「ワークスペース数 × プロジェクト数」(約 4,400 回)作り直す。同じ形は `internal/app/relations/snapshot.go:41`・`internal/app/github/links.go:129` にもある。本物の gitstore も同じ形なので、製品でも Store が大きいと遅い。
- Task 1 のあとも、サーバーの CPU(phone と tablet の 144 件)は 110 秒 → 102 秒しか減らなかった。残りの原因は次の 3 つ。
  - `internal/domain/change/stamp.go:62` の `group` を、`Resolve`(`resolve.go:85,115`)が項目ごと・値の候補ごとに呼び、そのたびにログ全行の `key(r)`(追加・削除の行は `json.Compact`)を求める。1 本のログの解決が「項目数 × 行数」になる。振り分けを 1 回にする試作で CPU 102 秒 → 64 秒、`GET /items/{id}` 40ms → 21ms。
  - 試作のあとの残りは `blockedSet` の `Workspaces().Settings`(全ワークスペースの設定を JSON から作り直す)。設定が要るのは、`blocks` の関連を持つアイテムのワークスペースだけ。
  - E2E では 2 つの worker が同じ Store に書く。Store が変わるたびに開いている画面が一覧(ワークスペースごとのプロジェクト一覧。52 ワークスペースで 104 リクエスト)・プロジェクト画面・詳細を読み直す(`web/app/src/wire.ts` の `reflect`。仕様どおり)。ほかの worker の書き込みでも読み直しが起きる。

## Global Constraints

- 大きさの上限: Go は 1 ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・1 パッケージ 20 ファイル(テストを除く)。`infra/gitstore`・`app/apptest`・`app/items` は 20 ファイルなので新しいファイルを足さない。TS は 1 ファイル 300 行。
- 振る舞いを変えない。画面・API・保存形式・仕様は変えない。
- e2e を手元で流すときは、他の worktree のサーバー(4173)と重ならないようにする。Task 4 までは、設定を写した一時ファイルで番号を変え(4273 など)、`reuseExistingServer: false` にする。終わったら消す。
- PR は 1 本(`perf/e2e-ci` → `develop`)。

## Review Focus

- 同じ Tx でプロジェクトを別のワークスペースへ動かしたあとの `Projects().List`: 動いた先に出て、元のワークスペースには出ない。Task 1 の契約テストで確かめる。
- 同じ Tx で新しく作ったプロジェクト: 作る前に `List` を呼んでいても、作ったあとの `List` に出る。Task 1 の契約テストで確かめる。
- `Resolve` で、create の初期値だけを持ち、ほかの行がない項目: 初期値が返る(振り分けで「行のないキー」を空として扱うと値が消える。試作で一度落ちた)。Task 2 のテストで確かめる。
- 別のワークスペースのアイテムにブロックされ、そのブロック元が完了: ブロック中にならない(ブロック元のワークスペースの設定で判定する)。既存の `TestBlockedJudgesBlockerByItsOwnWorkspace` が守る。
- e2e のサーバーが起動に失敗したとき: テストが何分も待たずに、サーバーの標準エラーを付けて落ちる。Task 4 で確かめる。

---

### Task 1: view の中でプロジェクトのワークスペースを覚える

**Files:**
- Modify: `internal/app/apptest/stores_contract_cases.go`(`contractListFollowsMove` の下に `contractProjectListFollowsMove` を足す)
- Modify: `internal/app/apptest/stores_contract.go`(cases の表に 1 行足す)
- Modify: `internal/infra/gitstore/read.go`(`view` に `projectPlaces`、`projects.List`)
- Modify: `internal/infra/gitstore/write.go:89-91`(プロジェクトに行を足したら消す)
- Modify: `internal/app/apptest/stores_read.go`(`view` に `projectPlaces`、`projects.List`)
- Modify: `internal/app/apptest/stores.go:140-142`(同上)

**Interfaces:**
- Produces: `view.projectPlaces map[project.ID]workspace.ID`(両方の view)。外から見える型・関数は変えない。

- [ ] **Step 1: 契約テストを書く**

`contractProjectListFollowsMove(t *testing.T, s port.Stores, id port.StoreID)`。`writeFixture` のあと、1 つの `mustWrite` の中で次を確かめる(`other` はワークスペース `"01CONTRACTWS00000000000009"` を `workspace.Create` で作っておく):

```go
// 1. tx.Projects().List(other) は空、List(cWorkspace.ID) は [cProject]
// 2. cProject の place を other に動かす: tx.WriteProjects().Append(cProject.ID, change.NewSet("place", cProject.Place, moved))
// 3. List(other) は ID が cProject.ID の 1 件、List(cWorkspace.ID) は空
// 4. 新しいプロジェクト "01CONTRACTPR00000000000008" を cWorkspace に project.Create で作る
// 5. List(cWorkspace.ID) はその 1 件
```

表に `{"project list follows a move in the same write", contractProjectListFollowsMove}` を `contractListFollowsMove` の行の下に足す。

- [ ] **Step 2: 流して通ることを確かめる(いまは覚えていないので通る)**

Run: `go test ./internal/app/apptest/ ./internal/infra/gitstore/ -run 'Contract|Stores' -count=1`
Expected: PASS。この時点で通るのは正しい: このテストは、覚える仕組みを足したあとも振る舞いが変わらないことを守るためのもの。

- [ ] **Step 3: 両方の view に覚える仕組みを足す**

`projects.List(ws)` は、`ids(KindProject)` の各 ID について、`projectPlaces[id]` があって `ws` と違えば作り直さずに飛ばす。作り直したプロジェクトは `projectPlaces[id] = p.Place.Workspace` と覚える。gitstore は `items.inProject` と同じ形の `projects.inWorkspace(ws)` を作って `list` に渡す。apptest は `items.List` と同じ形で書く。Tx の `append` で `ref.Kind == change.KindProject` なら `delete(t.projectPlaces, ref.ID)`。`view` の `places` のコメントに並べて、`projectPlaces` のコメントを書く。

- [ ] **Step 4: 流して通ることを確かめる**

Run: `go test ./internal/... -count=1 && go vet -tags e2e ./... && golangci-lint run ./...`
Expected: すべて PASS、lint の指摘なし。

- [ ] **Step 5: Commit**

```bash
git add internal/app/apptest internal/infra/gitstore
git commit -m "perf(store): 1 つの view で、別のワークスペースにあると分かっているプロジェクトを List で作り直さない"
```


### Task 2: `change.Resolve` で、行をキーで 1 回だけ振り分ける

**Files:**
- Modify: `internal/domain/change/resolve.go`(`Resolve`・`resolveValue`・`resolveList`)
- Modify: `internal/domain/change/stamp.go`(振り分けの関数を `group` の隣に置く)
- Test: `internal/domain/change/resolve_test.go`

**Interfaces:**
- Produces: なし(公開の関数・型は変えない)。`group`・`Tips`・`Current` はそのまま。

- [ ] **Step 1: テストとベンチマークを書く**

`TestResolveKeepsInitialValueOfAFieldWithoutRows`: create の初期値が `{"a":1,"b":2}`、`a` に set の行が 1 本(`To` は `3`)。`Resolve` の `Values["a"]` が `3`、`Values["b"]` が `2`。もう 1 つ、複数値の項目で、初期値 `["x"]`、ほかに別の値 `"y"` の add の行だけがあるとき、`Lists` が `["x","y"]`(`Options.Multi` でその項目を複数値にする)。
`BenchmarkResolveLongLog`: create 1 行と、200 個の項目に 1 本ずつ set、50 個の値の add を持つログを `Resolve` する。

- [ ] **Step 2: 流す(いまも通る)。ベンチマークの値を控える**

Run: `go test ./internal/domain/change/ -run Resolve -count=1 && go test ./internal/domain/change/ -run '^$' -bench BenchmarkResolveLongLog -benchmem`
Expected: PASS。ns/op を報告に書く。テストは、振り分けたあとも、行のない項目が create の行だけで決まることを守るためのもの。

- [ ] **Step 3: 振り分けを入れる**

`groupAll(log []Record) groups`(`stamp.go`): create 以外の行を `key(r)` で振り分け、各キーに create の行を足したものと、create の行だけのものを持つ。`func (g groups) of(k string) []Record` は、キーの行があればそれ、なければ create の行だけを返す(`group(log, k)` と同じ集合。並びは `tipsOf`・`winner` に関係しない)。`Resolve` は `filterValid` のあとに 1 回だけ `groupAll` を作り、`resolveValue`・`resolveList` は `group(log, k)` の代わりに `g.of(k)` を使う。

- [ ] **Step 4: 流す**

Run: `go test ./internal/... -count=1 && go test ./internal/domain/change/ -run '^$' -bench BenchmarkResolveLongLog -benchmem && golangci-lint run ./...`
Expected: PASS、lint の指摘なし。ns/op が Step 2 より下がる(報告に前後を書く)。

- [ ] **Step 5: Commit**

```bash
git add internal/domain/change
git commit -m "perf(change): いまの値を決めるとき、行を項目ごとに 1 回だけ振り分ける"
```

### Task 3: ブロック中の判定で、ブロック元のワークスペースの設定だけを読む

**Files:**
- Modify: `internal/domain/relation/relation.go:150-165`(`BlockedSet`)
- Test: `internal/domain/relation/relation_test.go`
- Modify: `internal/app/items/view.go:70-100`(`blockedSet`)

**Interfaces:**
- Consumes: なし。
- Produces: `relation.BlockedSet(all []item.Item, closed func(item.Item) bool) map[item.ID]bool` の型は変えない。`closed` は、`blocks` の関連を 1 つ以上持つアイテムにだけ呼ぶ(コメントに書く)。

- [ ] **Step 1: 失敗するテストを書く**

`TestBlockedSetAsksClosedOnlyForBlockers`(`relation_test.go`): アイテム A(B を blocks)、B(関連なし)、C(C から A へ relates)。`closed` は呼ばれたアイテムの ID を記録して false を返す。結果は `{B: true}`、`closed` が呼ばれたのは A だけ。

- [ ] **Step 2: 流して落ちることを確かめる**

Run: `go test ./internal/domain/relation/ -run TestBlockedSetAsksClosedOnlyForBlockers -count=1`
Expected: FAIL(いまは B と C にも `closed` を呼ぶ)。

- [ ] **Step 3: `BlockedSet` を直す**

アイテムごとに、`TypeBlocks` の関連があるときだけ `closed` を呼ぶ(関連がなければ飛ばす)。結果は変わらない。

- [ ] **Step 4: `blockedSet` を直す**

ワークスペースの一覧・プロジェクト・アイテムの読み方は今のまま。`Workspaces().Settings` はループの中で読まず、`closed` の中で、そのアイテムのワークスペースの設定を初めて要るときに 1 回だけ読んで覚える(`map[workspace.ID]workspace.Settings`)。`port.ErrNotFound` は今と同じく空の設定として扱う。それ以外のエラーは最初の 1 つを覚え、`BlockedSet` のあとで返す。`finished` の辞書はなくす。

- [ ] **Step 5: 流す**

Run: `go test ./internal/... -count=1 && golangci-lint run ./...`
Expected: PASS(`internal/app/items/blocked_test.go` の既存テストを含む)、lint の指摘なし。

- [ ] **Step 6: Commit**

```bash
git add internal/domain/relation internal/app/items
git commit -m "perf(items): ブロック中の判定で、ブロック元のあるワークスペースの設定だけを読む"
```

### Task 4: E2E のサーバーを worker ごとに空いた番号で起動する

**Files:**
- Create: `web/e2e/fixtures.ts`
- Modify: `web/playwright.config.ts`(`webServer`・`baseURL`・`addr` を外す。コメントを直す)
- Modify: `web/e2e/*.spec.ts`(`test`・`expect` の import 元を `./fixtures` にする。型の import は `@playwright/test` のまま)
- Modify: `CLAUDE.md:40`(e2e の番号の項を、worker ごとに空いた番号で起動する今のやり方に書き直す)

**Interfaces:**
- Produces: `web/e2e/fixtures.ts` が `test`(`@playwright/test` の `test` を、worker スコープの fixture `server: { url: string }` と、`baseURL` を `server.url` にする上書きで広げたもの)と `expect` を export する。

- [ ] **Step 1: fixture を書く**

`server`(scope: `"worker"`): `resolve(import.meta.dirname, "../../bin/hedgehug-e2e")` を、環境変数 `HEDGEHUG_ADDR=127.0.0.1:0`・`HEDGEHUG_TOKEN=e2e-token` で起動する。標準出力の 1 行 `http://127.0.0.1:<番号>/#token=e2e-token`(`cmd/hedgehug/main.go:44`)から `http://127.0.0.1:<番号>` を取り出して渡す。15 秒以内に出なければ、または先に終了したら、サーバーの標準エラーを付けたエラーで落とし、プロセスを SIGKILL する。後片付けは SIGINT を送り、5 秒で終わらなければ SIGKILL。`baseURL` は `async ({ server }, use) => use(server.url)`。

- [ ] **Step 2: 設定と spec を移す**

`playwright.config.ts` から `addr`・`use.baseURL`・`webServer` を外す。各 spec の `import { expect, ..., test } from "@playwright/test"` を、`test`・`expect` は `./fixtures` から、型は `@playwright/test` から読む形にする。`helpers.ts` は型だけを使うので変えない。

- [ ] **Step 3: 起動失敗の確かめ**

`HEDGEHUG_TOKEN` を空にした起動(e2e のビルドはトークンが空なら起動しない。`cmd/hedgehug/wire_e2e.go:34`)を一時的に試し、テストが 15 秒以内にサーバーの標準エラー付きで落ちることを確かめてから元に戻す。報告に出力を書く。

- [ ] **Step 4: 全件を流す**

Run: `make e2e-bin && cd web && pnpm exec playwright test --workers 2 && pnpm lint && pnpm check`
Expected: 全件 PASS。4173 番を使わない(流しているあいだ `lsof -nP -iTCP:4173 -sTCP:LISTEN` に hedgehug-e2e が出ない)。

- [ ] **Step 5: CLAUDE.md を直す**

40 行目の「e2e は `127.0.0.1:4173` 固定で、すでに動いているサーバーを使い回す。…」を、「e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。ほかの worktree のセッションと番号は重ならない。`--repeat-each` は同じ名前のワークスペースが重なるので、繰り返しの確認には使えない。」に置き換える。

- [ ] **Step 6: Commit**

```bash
git add web/e2e web/playwright.config.ts CLAUDE.md
git commit -m "test(e2e): worker ごとに e2e のサーバーを空いた番号で起動する"
```

### Task 5: E2E の時間を測って確かめる

**Files:** なし

- [ ] **Step 1: CI と同じ 2 並列で全件を流す**

Run: `make e2e-bin && cd web && pnpm exec playwright test --workers 2`
Expected: 全件 PASS。直す前の手元は 2.9 分。phone・tablet・desktop で同じテストの時間の差が縮む(幅ごとの合計を報告に書く)。

- [ ] **Step 2: 片付ける**

`web/test-results`・`web/playwright-report` を消し、`git status` に差分がないことを確かめる。
