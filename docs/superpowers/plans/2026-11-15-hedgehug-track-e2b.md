# hedgehug トラック E2b(テーブルのセルの編集)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** テーブルのセルを、04「セル」の表どおりの編集の部品にする(ステータス・優先度・単一選択はボタンとメニュー、担当者・ユーザー・スプリント・リリースはセレクト、日付・数値・テキストは入力欄、チェックはチェックボックス、関連システム・複数選択は開いたままのメニュー)。閲覧のみの Store の行は無効にして「閲覧のみの Store です」と添え、削除済みの行と「—」のセルは編集できないままにする。

**Architecture:** 検索の行に、編集に要る生の値(値の内部 ID・追加プロパティの値・スプリントとリリースの内部 ID)をサーバーが足す。画面は、行のワークスペースごとに設定・メンバー・スプリントとリリースの概要を 1 回読み(詳細パネルの `createContexts` を使う)、列と行から詳細パネルと同じ `PropertyRow` を作って、詳細パネルの `PropertyField` で描く。保存は詳細パネルと同じ口(`items.setProperty`・`addValue`・`removeValue`・`planning.setItemSprint`・`setItemRelease`)で、02 の検査・アクティビティ・割り当ての通知・自動化はサーバーの既存の処理に任せる。保存のあとは、テーブルを読み直し、`itemRefresh.afterChange`(詳細パネル・プロジェクトの画面・マイタスクなど)も呼ぶ。

**Tech Stack:** Go 1.27.3、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`: 「セル」(693〜713 行)、「削除済みを探す」の行の無効(312 行)、「閲覧のみの Store のアイテム」(478〜485 行)
- 02: 「プロパティの種類」「入力の方法と保存」(43〜87 行)、「ステータスの変更」(89〜100 行)
- 03: 使わない種類のセル(246 行)、非表示のプロパティもテーブルでは使える(250 行)
- `docs/superpowers/plans/2026-11-15-hedgehug-track-e2a.md` の末尾「E2b(セルの編集)」、`2026-11-15-hedgehug-track-e.md`、`2026-11-15-hedgehug-track-e1.md` の末尾「E2(テーブル)」の 808・810 行

## 始める前に

- E2 は 3 本に分けた(ユーザーの決定、2026-11-15)。**E2b = この計画**。E2c(ワークスペース・プロジェクトの「テーブル」タブ)と並行。ブランチ `feat/track-e2b`(develop 1173eeb7 から)。
- PR #233(E3b 保存したビュー)が開いている。E3b は `SearchBody.svelte` に `search` を props で渡すように変え、ビューの画面も `SearchBody` を使う。Task 6 で develop を取り込み、E3b が入っていればビューの画面のテーブルも同じ編集になるようにつなぐ(E2b-12)。

## Global Constraints

E2a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。いまの数: `internal/adapter/httpapi/search/search.go` 171 行、`table.go` 99 行、`web/app/src/screens/table/TableRow.svelte` 176 行、`ItemTable.svelte` 105 行、`web/app/src/search.ts` 196 行、`web/app/src/wire.ts` 299 行(足さない)、`web/packages/core/src/features/items/properties.ts` 185 行。
- 画面のパッケージの依存の向き: ui は core を import しない。packages は app を import しない。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおり。閲覧のみは core の `readOnlyNote`(「閲覧のみの Store です」)、保存の失敗は core の `sprintSaveFailed`(「保存できませんでした」)、削除された値は `deletedValue`、人は `unknownUser`。
- API を変えたら `make generate` と `cd web && pnpm gen` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする。
- e2e は、同じ Store を使うテストの前に `resetSearchScreen` を呼ぶ(`search-table-helpers.ts` の `openWith` が呼ぶ)。手元の検証は 1 回で足りる。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| E2b-1 | 検索の行に `values` を足す: `priority`(優先度の内部 ID)、`systems`(関連システムの内部 ID の並び)、`choices`(追加の選択式プロパティの内部 ID → 値の内部 ID の並び)、`fields`(追加プロパティの内部 ID → 値。items の API の `fields` と同じ形)、`sprint`・`release`(内部 ID。なければ空)。ステータス・担当者・開始日・終了日・ポイント・見積もり時間は、行にある `statusId`・`assignee.login`・`start`・`end`・`points`・`estimate` を使う。マイタスクの行も同じ形(`rowOf` を共有)なので、同じ値が入る | E2a の行は表示用の名前しか持たず、選択肢のチェック・入力欄の初めの値・同じ値なら保存しない(02 の 58 行)の判定に内部 ID が要る |
| E2b-2 | 選択肢の元(ワークスペースの設定・Store のメンバーと「あなた」・スプリントとリリースの概要)は、結果が届いたときに、行のワークスペースごとに 1 回ずつ読む(詳細パネルの `createContexts`)。読めるまでと読めなかった行は E2a の表示のまま。Store が変わった知らせで捨てて読み直す(読めるまで前のものを使う)。E2a の申し送りの「セルを押したときに読む」は採らない | セレクト(担当者・スプリント・リリース)と入力欄は、押す前から値と選択肢を出している必要がある(04 の 703・707・711・712 行)。マイタスクのステータスのボタン(E4a)と同じ読み方 |
| E2b-3 | 列とプロパティの当て方: 組み込みに当たる 8 列(`status`・`assignee`・`priority`・`start`・`end`・`points`・`estimate`・`system`)は、行のワークスペースの組み込みのプロパティ(`builtin` で `key` が同じもの。`system` は `systems`)。追加プロパティの列 `p:{小文字のキー}` は、行のワークスペースの追加プロパティで、キーを小文字にしたものが同じもの。非表示のプロパティも編集できる | 03 の 250 行(テーブルでは非表示でも使える)、E2a-1 の列の鍵 |
| E2b-4 | 列の型と行のワークスペースのプロパティの型が違うときは、行のワークスペースのプロパティの型の部品で編集する(列 `due` が日付でも、テキストの `due` を持つワークスペースの行はテキストの入力欄) | 04 の 663 行(型が違っても 1 列)、E2a-9(セルは自身の型で表示) |
| E2b-5 | 部品は詳細パネルの `PropertyField` をそのまま使う(ステータスは中で `StatusButton`。遷移の制約とプロジェクトの使うステータスは 02 の「ステータスの変更」どおり)。スプリント・リリースは ui の `Select` に core の `sprintChoices`・`releaseChoices`・`chosen`・`asOptions`。URL の型のセルは E2a のリンクの表示のまま(編集は詳細パネル) | 04 の 697〜712 行は 02 の 58〜72 行の表と同じ部品を指す。URL は 04 の 709 行 |
| E2b-6 | 編集できないセル: ID・種類・タイトル・プロジェクト・親・作成日・更新日(E2a のまま)、`unused` の列(「—」)、削除済みの行のすべてのセル(E2a の表示のまま。削除済みのピルと日数を添えているので、「閲覧のみの Store です」は添えない)、選択肢の元を読めていない行。閲覧のみの Store の行は、部品を出して無効にし、`title` に「閲覧のみの Store です」 | 04 の 312・482・483・713 行、03 の 246 行。削除済みの見せ方はマイタスクの行(E4a)と同じ |
| E2b-7 | 保存: プロパティは `items.setProperty(storeId, itemId, propertyId, value)`、関連システム・複数選択は `addValue`・`removeValue`、スプリント・リリースは `planning.setItemSprint`・`setItemRelease`(空は null)。いまの値と同じなら送らない(プロパティは `PropertyField` が、スプリント・リリースは `chosen` と比べて)。成功してもトーストは出さない。断られたらその文、ほかの失敗(404・通信)は「保存できませんでした」をエラーのトーストに出す。成否にかかわらず、`itemRefresh.afterChange()` とテーブルの読み直しをする | 04 の 713 行、E2a の申し送り(パレットの「ステータスを変更」と同じ出し方)、F1-18 |
| E2b-8 | 保存できなかったセルは、その行の部品を作り直して保存済みの値に戻す(詳細パネルの `version` と同じ)。読み直しで値が変わらないと入力欄に断られた文字が残るため | 詳細パネルの `DetailProperties.svelte` と同じ |
| E2b-9 | グループの行は、グループの見出しとアイテム(Store と内部 ID)で見分ける(E2a はグループの位置と行の位置)。同じ見出しが 2 つあるとき(別の Store の同じ ID とタイトルの親など)は、2 つ目から `#2`・`#3` を付ける。セルを編集して並べ替えやグループが変わっても、入力中の文字や開いているメニューが別のアイテムの行に移らない。関連システムのグループで、開いているメニューから外した値のグループの行は、そのグループから消えるのでメニューも閉じる(ほかのグループの同じアイテムの行は残る) | 04 の 691 行(同じアイテムが複数のグループに出る)。行を描き直さずにメニューを残すと、選択肢のチェックが保存と食い違う |
| E2b-10 | 担当者のセルは、`PropertyField` の `user` のセレクト(「未割り当て」、メンバー、あなたに「(あなた)」、いまの人がメンバーにいなければ「不明なユーザー」)。E2a のアバターの表示は、選択肢の元を読めていない行と削除済みの行だけに残す | 04 の 703 行、01 |
| E2b-11 | テーブルの部品 `ItemTable` は、セルの編集の元 `cells: TableCells` と読み直し `reload: () => Promise<void>` を props で受ける(検索の画面は `searchCells` と `search.reload`)。E2c のワークスペース・プロジェクトのタブは、同じ `createTableCells` で自分の `TableCells` を作って渡す | E2c が同じ部品を使う(E2a の申し送り) |
| E2b-12 | E3b(PR #233)が develop に入っていたら、Task 6 で取り込み、`SearchBody` が props の `search` の `reload` を `ItemTable` に渡すようにする(ビューの画面のテーブルも同じ編集になる) | 04 の 661 行(検索とビューのテーブルは同じ)。CLAUDE.md の「残タスクを残さない」 |

## Review Focus

1. **断られた入力が残る。** 開始日 2026-11-20 のアイテムの終了日に 2026-11-10 を入れて Tab で離れると、「終了日は開始日以降にしてください」が出て、終了日の欄は空(元の値)に戻る。ポイントに -1 を入れたときも同じ(02 の 76〜81 行)。Task 3 の `save shows the refusal and still reloads`、Task 5 の e2e `a refused end date goes back to the saved value` に入れる。
2. **グループ分けの列を編集すると、入力が別の行に移る。** 優先度でグループ分けしたテーブルで行の優先度を変えると、行は別のグループに移る。移ったあとのその位置の別の行に、前の行のメニューや入力中の文字が残らない(E2b-9)。Task 2 の `groupKeys`、Task 5 の e2e `editing the grouped column moves the row without carrying the menu` に入れる。
3. **Tab で次のセルに移る途中で読み直す。** 開始日を入れて Tab で終了日の欄に移ると、開始日の保存と読み直しが走る。読み直しで終了日の欄の入力中の文字が消えない(`PropertyField` の `editing`)。Task 4 の e2e `tab from start to end saves both` に入れる。
4. **関連システムのメニューが、保存のたびに閉じる。** 関連システムのセルのメニューで 2 つ続けて選ぶと、どちらも保存され、メニューは開いたまま(04 の 707 行)。Task 4 の e2e `the systems menu stays open while choosing two` に入れる。
5. **保存の前にほかで削除された。** 別のタブで削除したアイテムのセルを変えると、「保存できませんでした」が出て、テーブルを読み直すと行が消える。Task 3 の `save shows the save failure for a not-found item`、Task 5 の e2e `a row deleted elsewhere shows the save failure` に入れる。

---

### Task 1: 検索の行に編集に要る値を足す(サーバーと API)

**Files:**
- Modify: `internal/adapter/httpapi/search/search.go`(`SearchRow` に `Values`)、`internal/adapter/httpapi/search/table.go`(`SearchValues` と `fillValues`)
- Modify: `internal/adapter/httpapi/search/search_test.go`
- Regenerate: `api/openapi.yaml`、`web/packages/api/src/generated.ts`
- Modify: `web/packages/core/src/features/search/types.ts`(`SearchValues`、`SearchRow.values`)、`web/packages/api/src/search.ts`(`rowOf`)、`web/packages/api/src/search.test.ts`
- Modify: 行の型を直書きしているテスト(`web/packages/core/src/features/search/cells.test.ts`・`view.test.ts`・`table.test.ts`・`fixtures.test-util.ts`・`searchTree.test-util.ts`、`features/mytasks/base.test.ts`・`group.test.ts`。`pnpm check` が落とすものすべて)に `values: emptyValues` を足す

**Interfaces:**
- Produces(Go): `type SearchValues struct { Priority string; Systems []string; Choices map[string][]string; Fields map[string]json.RawMessage; Sprint string; Release string }`(json は `priority`・`systems`・`choices`・`fields`・`sprint`・`release`。`systems`・`choices`・`fields` は `nullable:"false"`)。`SearchRow.Values SearchValues \`json:"values"\``。
- Produces(TS core): `export interface SearchValues { priority: string; systems: string[]; choices: Record<string, string[]>; fields: Record<string, string | number | boolean | null>; sprint: string; release: string }`、`SearchRow.values: SearchValues`、`export const emptyValues: SearchValues`(`features/search/types.ts`。テストの材料と画面の初めの値に使う)。

- [ ] **Step 1: 失敗するテストを書く**

`search_test.go` に `TestSearchAPIRowRaw`: `kind:bug` のテーブルの行で、`Values.Priority == string(apptest.SamplePriorityMedium)`、`Values.Systems == []string{string(apptest.SampleSystemWeb)}`、`Values.Choices` と `Values.Fields` が nil でない(JSON で `{}`)、`Values.Sprint == ""`。サンプルのバグの優先度・関連システムが違えば、`apptest/sample.go` の値に合わせる。

`web/packages/api/src/search.test.ts` に `rowOf copies values and fills nulls`: `values: { priority: "P", systems: null, choices: null, fields: null, sprint: "S", release: "" }` の DTO から `{ priority: "P", systems: [], choices: {}, fields: {}, sprint: "S", release: "" }`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/search/ -run TestSearchAPIRowRaw`
Expected: FAIL(`Values` がない)

- [ ] **Step 3: 実装する**

`fillValues(row *SearchRow, it item.Item)` を `table.go` に置き、`fillTable` から呼ぶ。中身は items の `toAPI`(`internal/adapter/httpapi/items/items.go:48`)の `Systems`・`Choices`・`Fields` の写し方と同じ(パッケージをまたいで import しない)。`make generate`、`cd web && pnpm gen`。core の型と `emptyValues`、`rowOf` の写し(null は空)。テストの行の直書きに `values: emptyValues` を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/adapter/httpapi/search/ && cd web && pnpm check && pnpm --filter @hedgehug/api test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/httpapi/search api/openapi.yaml web/packages
git commit -m "feat(search): 検索の行に、セルの編集に要る値の内部 ID と追加プロパティの値を足す(E2b Task 1)"
```

---

### Task 2: 列と行から編集の部品の中身を決める(core)

**Files:**
- Modify: `web/packages/core/src/features/items/properties.ts`(1 つのプロパティの行 `propertyRow` を取り出し、`propertyRows` はそれを使う)
- Create: `web/packages/core/src/features/search/cellEdit.ts`、`cellEdit.test.ts`
- Modify: `web/packages/core/src/searchIndex.ts`、`web/packages/core/src/index.ts`(`propertyRow as itemPropertyRow` を足す)

**Interfaces:**
- Consumes: Task 1 の `SearchRow.values`。`Context`(`features/items/detailContext.ts` の `{ settings, people, plan }`)。
- Produces:
  - `propertyRow(settings: WorkspaceSettings, p: WorkspaceProperty, value: PropertyInput, people: People, project?: string): PropertyRow`(`properties.ts`。`propertyRows` の 1 件ぶん。非表示・種類で絞らない)
  - `rowValues(row: SearchRow): ItemValues`(`cellEdit.ts`。`status: row.statusId`、`assignee: row.assignee?.login ?? ""`、`priority`・`systems`・`choices`・`fields` は `row.values`、`start`・`end`・`points`・`estimate` は行)
  - `export type CellEditor = { kind: "property"; row: PropertyRow } | { kind: "plan"; key: "sprint" | "release"; options: { value: string; label: string }[]; value: string } | { kind: "display" }`
  - `cellEditor(row: SearchRow, column: SearchColumn, ctx: Context | undefined): CellEditor`
  - `groupKeys(groups: SearchGroup[]): string[]`(見出しの文字。2 回目からは `${label}#2` のように数を付ける)

- [ ] **Step 1: 失敗するテストを書く**

`cellEdit.test.ts`(設定は `features/items/settings.test-util.ts` の `settings`、行は `cells.test.ts` の `row` と同じ形を作る):
- `display without a context`: `ctx` が undefined なら `{ kind: "display" }`。
- `display for deleted rows, unused cells, and display-only columns`: `deleted` がある行、`unused` に `priority` がある行の `priority`、列 `id`・`kind`・`title`・`project`・`parent`・`created`・`updated` はどれも `display`。
- `display for a url property`: 型 `url` の追加プロパティの列は `display`。
- `maps builtin columns`: 列 `priority` → `kind: "property"`、`row.property.key === "priority"`、`row.value === row.values.priority`、優先度の選択肢に「なし」がない。列 `system` → プロパティの `key === "systems"`、`value` が `values.systems`。列 `assignee` → `type === "user"`、選択肢の先頭が「未割り当て」。
- `maps a custom column by lower-cased key, with the row's own type`: 行のワークスペースの追加プロパティ `Due`(型 `text`)と列 `{ key: "p:due", type: "date" }` → `row.property.type === "text"`、`value` が `values.fields[その ID]`。
- `edits a hidden builtin property`: 優先度を `hidden: true` にしても `kind: "property"`。
- `plan cells`: Story の列 `sprint` → `kind: "plan"`、`options[0]` が `{ value: "", label: "バックログ" }`、`value` は `chosen(...)`。列 `release` も同じ(先頭「なし」)。
- `rowValues reads ids`: `statusId`・`assignee.login`・`values` から `ItemValues`。
- `groupKeys numbers repeats`: `["A","B","A"]` → `["A","B","A#2"]`。

`properties.test.ts`(あるもの)に `propertyRow matches propertyRows for one property` を足す。

- [ ] **Step 2: 落ちることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- cellEdit`
Expected: FAIL(`cellEdit` がない)

- [ ] **Step 3: 実装する**

`cellEditor` は、`display` の判定(E2b-6)→ `sprint`・`release`(`sprintChoices`/`releaseChoices` と `asOptions`、`chosen`。`row.values.sprint`)→ 列からプロパティを探す(E2b-3。見つからなければ `display`)→ 型 `url` は `display` → `propertyRow(ctx.settings, p, valueOf(rowValues(row), p), ctx.people, row.projectId)`。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): テーブルのセルの列と行から、詳細パネルと同じプロパティの行を作る(E2b Task 2)"
```

---

### Task 3: セルの選択肢の元と保存(core)

**Files:**
- Create: `web/packages/core/src/features/search/tableCells.ts`、`tableCells.test.ts`
- Modify: `web/packages/core/src/searchIndex.ts`

**Interfaces:**
- Consumes: `createContexts`(`features/items/detailContext.ts`)、`ItemsPort`、`PlanningPort`、`WorkspacesPort`、`CatalogPort`、`Toasts`、`Refused`、`sprintSaveFailed`、Task 2 の `CellEditor`。
- Produces:
  ```ts
  export type CellEdit =
    | { kind: "property"; propertyId: string; value: PropertyInput }
    | { kind: "toggle"; propertyId: string; valueId: string; on: boolean }
    | { kind: "plan"; key: "sprint" | "release"; id: string };
  export interface TableCells {
    // loaded は、選択肢の元を読み終えるたびに増える(行が描き直すため)。
    loaded: Readable<number>;
    context(storeId: string, workspaceId: string): Context | undefined;
    // load は、まだ読んでいない行のワークスペースの元を読み始める。
    load(rows: SearchRow[]): void;
    // reset は、元を捨てる(次の load で読み直す。読み終えるまでは前の元を返す)。
    reset(): void;
    // save は、1 回保存する。保存したら true。成否にかかわらず changed() と reload() を待つ。
    save(row: SearchRow, edit: CellEdit, reload: () => Promise<void>): Promise<boolean>;
  }
  export function createTableCells(deps: {
    items: Pick<ItemsPort, "setProperty" | "addValue" | "removeValue">;
    planning: PlanningPort;
    workspaces: WorkspacesPort;
    catalogPort: CatalogPort;
    toasts: Pick<Toasts, "show">;
    changed: () => Promise<void>;
  }): TableCells;
  ```

- [ ] **Step 1: 失敗するテストを書く**

`tableCells.test.ts`(ポートは `vi.fn` の偽物):
- `loads each workspace once`: 同じワークスペースの行 2 つと別のワークスペースの行 1 つで `load` → `workspaces.get` が 2 回。読み終えると `context` が返り、`loaded` が増える。
- `keeps the old context until reloaded`: `reset` のあと `load` して読み終えるまでは前の `context`、読み終えたら新しいもの。`reset` の前に始めた読み込みの結果は入れない。
- `retries a failed workspace`: 1 回目の `get` が失敗したら、次の `load` で読み直す。
- `save property`: `{ kind: "property", propertyId: "pri", value: "v2" }` → `items.setProperty("s1", "i1", "pri", "v2")`、トーストなし、`changed` と `reload` を 1 回ずつ、true。
- `save toggle`: on なら `addValue`、off なら `removeValue`。
- `save plan`: `{ kind: "plan", key: "sprint", id: "" }` → `planning.setItemSprint("s1", "i1", null)`。いまの値(`chosen` の値)と同じ id なら送らず、`changed`・`reload` も呼ばず true。
- `save shows the refusal and still reloads`: `setProperty` が `new Refused("終了日は開始日以降にしてください")` → その文をエラーのトースト、`changed`・`reload` を呼び、false。
- `save shows the save failure for a not-found item`: `NotFound` → 「保存できませんでした」、false。

- [ ] **Step 2: 落ちることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- tableCells`
Expected: FAIL

- [ ] **Step 3: 実装する**

元の読み方は `web/app/src/myTasks.ts` の設定の読み方(`round` で古い結果を捨て、読めたものを `loaded` に残す)と同じにし、読む中身は `createContexts(deps).of(storeId, workspaceId, false)`。`reset` は `createContexts` を作り直して `round` を進める。plan の「同じ値」は `context` の `plan` と `row.values` で `chosen` を求めて比べる(元がなければ比べずに送る)。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): テーブルのセルの選択肢の元をワークスペースごとに読み、保存と失敗のトーストと読み直しをまとめる(E2b Task 3)"
```

---

### Task 4: セルを編集の部品にする(画面)

**Files:**
- Create: `web/app/src/tableCells.ts`(検索の画面の `searchCells`)、`web/app/src/screens/table/TableEditCell.svelte`
- Modify: `web/app/src/screens/table/TableRow.svelte`(ステータス・担当者・その他の編集できる列を `TableEditCell` に渡す)、`ItemTable.svelte`(props `cells`・`reload`)、`web/app/src/screens/search/SearchBody.svelte`
- Create: `web/e2e/search-table-edit.spec.ts`

**Interfaces:**
- Consumes: Task 2 の `cellEditor`、Task 3 の `createTableCells`・`TableCells`・`CellEdit`、`PropertyField`(`screens/item/PropertyField.svelte`)、ui の `Select`、`itemRefresh`(`wireActions.ts`)、`reloads`(`reloads.ts`)、`search`(`search.ts`)。
- Produces:
  - `web/app/src/tableCells.ts`: `export const searchCells: TableCells`(`createTableCells({ items: itemsPort, planning: planningPort, workspaces: workspacesPort, catalogPort, toasts, changed: () => itemRefresh.afterChange() })`。`search.state` の行が変わるたびに `load`、`reloads` に `reset` を登録)
  - `ItemTable` の props に `cells: TableCells`、`reload: () => Promise<void>`
  - `TableEditCell.svelte` の props: `row: SearchRow`、`column: SearchColumn`、`now: Date`、`cells: TableCells`、`reload: () => Promise<void>`。`cellEditor` が `display` なら E2a の表示(ステータスは点と名前、担当者はアバターと名前、ほかは `TableCell`)、`property` なら `PropertyField`(`disabled={row.readOnly}`、`onchange` → `{ kind: "property" }`、`ontoggle` → `{ kind: "toggle" }`)、`plan` なら `Select`(名前は列の名前、`disabled`・`title` は閲覧のみのとき)

- [ ] **Step 1: 失敗する e2e を書く**

`search-table-edit.spec.ts`(`search-table-helpers.ts` の `seed`・`openWith`・`rowOf`・`send` を使う。列は「表示する列」のメニューで出すか、画面の状態に `columns` を入れてから開く):
- `changes the priority from the cell menu`: Story の行の優先度のボタン → メニューで別の優先度 → セルがその名前、トーストなし。API でアイテムを読むと優先度が変わっている。
- `clears points by emptying the input`: ポイントに 3 を入れて Enter → 3、空にして Enter → 空。
- `tab from start to end saves both`: 開始日に 2026-11-20、Tab で終了日に移って 2026-11-25、Tab → 両方保存されている(API で読む)。
- `the systems menu stays open while choosing two`: 関連システムのボタン → 2 つ選ぶ → メニューは開いたまま、両方保存。
- `picks an assignee and a sprint from selects`: 担当者のセレクトに「(あなた)」の付いた項目と「未割り当て」がある。選ぶと保存。スプリント(API で作ったもの)のセレクトで選ぶと保存。Epic の行のスプリントのセルは「—」。
- `the status button follows the workflow`: ステータスのボタンでメニューが開き、選ぶと変わる。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e-bin && cd web && pnpm exec playwright test search-table-edit`
Expected: FAIL(優先度のボタンがない)

- [ ] **Step 3: 実装する**

`TableRow.svelte` の `status`・`assignee` の分岐と `TableCell` への分岐を、編集できる列(`status`・`assignee`・`priority`・`start`・`end`・`points`・`estimate`・`system`・`sprint`・`release`・`p:` で始まる列)では `TableEditCell` に替える。`TableEditCell` は `$cells.loaded`(`cells.loaded` の購読)を読んで `cells.context(row.storeId, row.workspaceId)` を引き直す。数値のセルの右寄せは `TableCell` と同じ。`SearchBody` は `cells={searchCells}`・`reload={() => search.reload()}` を渡す。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm check && pnpm lint && pnpm size && make -C .. e2e-bin && pnpm exec playwright test search-table`
Expected: PASS(E2a の `search-table.spec.ts`・`search-table-select.spec.ts` も)

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 検索のテーブルのセルを、04 の表どおりの編集の部品にする(E2b Task 4)"
```

---

### Task 5: 編集できないセル・断られた入力・グループの行

**Files:**
- Modify: `web/app/src/screens/table/TableRow.svelte`(保存できなかったら行の部品を作り直す `version`)、`TableEditCell.svelte`(`save` の結果を返す)、`ItemTable.svelte`(グループの行の鍵)
- Modify: `web/e2e/search-table-edit.spec.ts`

**Interfaces:**
- Consumes: Task 2 の `groupKeys`、Task 3 の `save` の真偽。
- Produces: `TableEditCell` の props に `onfail: () => void`(保存が false のとき)。`TableRow` は `version` を持ち、編集のセルを `{#key version}` で包む。`ItemTable` のグループは `{#each groups as group, g (keys[g])}`、行は `(\`${keys[g]}/${row.storeId}/${row.itemId}\`)`(`keys = groupKeys(groups)`)。

- [ ] **Step 1: 失敗する e2e を書く**

- `read-only rows disable the cells`: `openWith(page, info, "store:readonly ログイン画面")` で、優先度のボタン・ステータスのボタン・ポイントの入力欄が無効で、`title` が「閲覧のみの Store です」。列見出しの並べ替えと行のチェックは使える。
- `deleted rows show values without editors`: アイテムを API で削除して `is:deleted {mark}` → 優先度のセルにボタンがなく、名前の文字だけ。
- `a refused end date goes back to the saved value`: 開始日 2026-11-20 のアイテムの終了日に 2026-11-10 を入れて Tab → トースト「終了日は開始日以降にしてください」、終了日の欄は空。
- `editing the grouped column moves the row without carrying the menu`: グループを「優先度」にして、2 行のうち 1 行の優先度を変える → その行が別の見出しの下に移り、メニューは閉じ、もう 1 行の優先度のボタンは元の名前のまま。
- `a row deleted elsewhere shows the save failure`: 表示したあと API でアイテムを削除し、そのポイントの欄に入力して Enter → 「保存できませんでした」、行が消える。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e-bin && cd web && pnpm exec playwright test search-table-edit`
Expected: FAIL(断られた終了日が欄に残る、グループの行の鍵が位置のまま)

- [ ] **Step 3: 実装する**

`TableEditCell` は `cells.save` の結果が false なら `onfail()`。`TableRow` は `onfail` で `version += 1`。`ItemTable` のグループの鍵を `groupKeys` にする。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm check && pnpm lint && make -C .. e2e-bin && pnpm exec playwright test search-table`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 閲覧のみ・削除済みの行のセルを編集できないようにし、断られた入力を保存済みの値に戻す(E2b Task 5)"
```

---

### Task 6: develop の取り込みと文書

**Files:**
- Modify(E3b が入っていれば): `web/app/src/screens/search/SearchBody.svelte`(`reload={() => search.reload()}` を props の `search` にする)
- Modify: `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`、`2026-11-15-hedgehug-track-e1.md`(808・810 行)、`2026-11-15-hedgehug-track-e2a.md`(末尾「E2b(セルの編集)」)、この計画の末尾「E2c に渡すもの」

- [ ] **Step 1: develop を取り込む**

Run: `git fetch origin && git merge origin/develop`
衝突は、両方の変更を残して解く。PR #233 が入っていれば、`SearchBody` の `ItemTable` に props の `search` の `reload` を渡し、ビューの画面で優先度を変える e2e を `search-table-edit.spec.ts` に 1 つ足す(`views.spec.ts` のビューの作り方を使う)。

- [ ] **Step 2: 文書を直す**

- e1 の 808・810 行の括弧に「セルの編集は E2b で足した」「削除済みの行・閲覧のみの行のセルの無効は E2b で足した」を添える。
- e2a の「E2b(セルの編集)」の各行に「E2b で足した」を添え、選択肢を読むときのことは「E2b-2 で結果が届いたときに読むことにした」と添える。
- トラック E の分け方の E2b の行の中身はそのまま。
- この計画の末尾「E2c に渡すもの」を実装に合わせて直す。

- [ ] **Step 3: 全体を確かめる**

Run: `make check && make e2e`
Expected: どちらも緑

- [ ] **Step 4: コミット**

```bash
git add -A
git commit -m "docs: E2b で足したセルの編集を、トラック E・E1・E2a の計画に書き添える(E2b Task 6)"
```

## E2c に渡すもの

- `ItemTable` は `cells: TableCells` と `reload` を props で受ける。ワークスペース・プロジェクトのタブは、`createTableCells` を画面ごとに 1 つ作り(`web/app/src/tableCells.ts` の `searchCells` と同じ組み立て)、その画面の結果の行で `load`、`reloads` で `reset` する。
- 行は `values`(E2b-1)を持つので、範囲を足した検索の API の行もそのまま編集できる。
