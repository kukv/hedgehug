# E2E の読み取りの速さ 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 1 回の Read の中で `Projects().List(ws)` をワークスペースごとに呼んでも、全プロジェクトをワークスペースの数だけ作り直さないようにして、CI の E2E の時間を戻す。

**Architecture:** アイテムにはすでに、view の中で「アイテム → プロジェクト」を覚える `places` があり、`Items().List(p)` は別のプロジェクトにあると分かっているアイテムをログから作り直さない(gitstore `read.go:177`、apptest `stores_read.go:152`)。同じ仕組みを「プロジェクト → ワークスペース」に足す。Tx でプロジェクトに行を足したら、そのプロジェクトの分を消す(アイテムと同じ)。振る舞いは変えない。

**Tech Stack:** Go(`internal/infra/gitstore`・`internal/app/apptest`)、Playwright(計測だけ)。

**Spec:** なし(性能の修正で、振る舞いを変えない)。調査の結果は下の「調べたこと」。

## 調べたこと

- CI の E2E: 165 件 1.8 分(2026-11-13 08:48)→ 216 件 5.8 分(13:03)。同じテストが phone 3.3 秒 → tablet 7.3 秒 → desktop 18.5 秒と、あとで流れるほど遅い。
- e2e のサーバー(`cmd/hedgehug/wire_e2e.go`)は全テストで 1 つの Store「e2e」を共有する。desktop 72 件は新しいサーバーで 29 秒、phone・tablet のあとのサーバーで 1.6 分。`GET /items/{id}` は 1ms 未満 → 約 48ms(ワークスペース 76・プロジェクト 58・アイテム 164)。
- 原因: `internal/app/items/view.go:72` の `blockedSet`(アイテムの取得・一覧で毎回呼ぶ)が、ワークスペースごとに `Projects().List(w.ID)` を呼ぶ。`List` は全プロジェクトをログから作り直して絞るので、1 回で「ワークスペース数 × プロジェクト数」(約 4,400 回)作り直す。同じ形は `internal/app/relations/snapshot.go:41`・`internal/app/github/links.go:129` にもある。本物の gitstore も同じ形なので、製品でも Store が大きいと遅い。

## Global Constraints

- 大きさの上限: Go は 1 ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・1 パッケージ 20 ファイル(テストを除く)。`infra/gitstore`・`app/apptest` は 20 ファイルなので新しいファイルを足さない。
- e2e は 4173 番を他の worktree と共有している。流すときは設定を写した一時ファイルで番号を変え(4273 など)、`reuseExistingServer: false` にする。終わったら消す。
- PR は 1 本(`perf/e2e-ci` → `develop`)。

## Review Focus

- 同じ Tx でプロジェクトを別のワークスペースへ動かしたあとの `Projects().List`: 動いた先に出て、元のワークスペースには出ない。Task 1 の契約テストで確かめる。
- 同じ Tx で新しく作ったプロジェクト: 作る前に `List` を呼んでいても、作ったあとの `List` に出る。Task 1 の契約テストで確かめる。

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

### Task 2: E2E の時間を測って確かめる

**Files:** なし(一時ファイルだけ。終わったら消す)

- [ ] **Step 1: e2e のバイナリを作る**

Run: `make e2e-bin`

- [ ] **Step 2: 4273 番で、CI と同じ 2 並列で全件を流す**

`web/playwright.tmp-4273.config.ts`(`playwright.config` を読み、`workers: 2`、`baseURL`・`webServer.url`・`HEDGEHUG_ADDR` を `127.0.0.1:4273`、`reuseExistingServer: false`)で `pnpm exec playwright test -c playwright.tmp-4273.config.ts`。
Expected: 全件 PASS。直す前の手元の時間は 2.9 分。同じテストの phone・tablet・desktop の時間の差が縮む。

- [ ] **Step 3: 一時ファイルを消す**

`web/playwright.tmp-4273.config.ts`・`web/test-results`・`web/playwright-report` を消し、`git status` に計画以外の差分がないことを確かめる。
