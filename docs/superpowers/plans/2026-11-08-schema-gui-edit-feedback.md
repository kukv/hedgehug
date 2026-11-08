# スキーマ編集のフィードバック対応 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** PR #105 へのフィードバック F1〜F7 を入れる（名前の変更を編集に統合、「…」をなくす、自由な色、種類の色、種類の文言、enum の値をダイアログで設定、既定の表示名を英語に）。

**Architecture:** Go 側で、PATCH が新しい名前と enum の値の全体を受け、定義の変更とタスクの書き換えを 1 コミットで行う（`schemas.Edit` にタスクの書き換えを足す）。種類に `color` を足す。フロントは編集ダイアログに名前と値の編集を入れ、「名前」ボタンと値の行・値のダイアログをなくす。

**Tech Stack:** Go（huma v2）、Svelte 5、Orval、Vitest、Playwright

**Spec:** `docs/superpowers/specs/2026-11-08-schema-gui-edit-design.md` の「フィードバックによる変更」（F1〜F7）。上の節と食い違うところはこの節が優先する。

## Global Constraints

- 1 操作 1 コミット（op `schema`）。名前の変更・値の編集も、ほかの属性と同じ 1 コミット
- 新しい名前がすでにある定義なら 409 `schema.name_taken`。値の不正（空・重複・同じ今の値が 2 つの from）は 422 `schema.value_invalid`。使用中の値の削除は 422 `schema.in_use`
- 値の個別の API（`add-value`・`delete-value`・`order-values`）はなくす
- 種類の `color` は YAML の `color`（omitempty）。値の扱いはステータスの色と同じ（12 色の名前・`#RRGGBB`・ほかは無視）
- 文言: グループ（container）・アイテム（work）・どの下に置けるか（parents）・最上位（root）。ボタンは「追加」「編集」「マージ」「削除」
- 既定の表示名（新しい Store のみ）: Backlog・Todo・On hold・In progress・Waiting・Review・Done・Canceled / Project・Epic・Story・Task・Bug / Duplicates（Duplicated by）・Causes（Caused by）・Relates to / 画面の blocked_by は Blocked by
- 既定（default）の扱いは変えない
- 全テストは仕上げに 1 回。spec と違う挙動が必要なら止めて報告する

---

### Task A: Go（ドメイン・usecase・サーバー・生成物）

**Files:** `internal/app/domain/schema/schema.go`・`edit.go`・`edit_test.go`・テストデータ、`internal/app/usecase/schemas/edit.go`・`schemaview.go`・テスト、`internal/app/server/schemaedit.go`・テスト、`api/openapi.yaml`、`web/core/api/generated.ts`

**Interfaces（Task B・C が使う）:**
- `Kind.Color string \`yaml:"color,omitempty"\``、`KindDef.Color \`json:"color,omitempty"\``、`KindChange.Color *string`、`KindAddInputBody.color`
- `StatusChange`・`KindChange`・`PropChange` に `Name *string`（新しい名前）
- `PropChange.Values *[]ValueEdit`、`type ValueEdit struct { Name string; From []string }`
- 本文: `change-status`・`change-kind`・`change-prop` に `name?: string`、`change-prop` に `values?: {name: string, from?: string[]}[]`
- `schemas.Edit(ss, fn func(*schema.Schema) (*schema.Schema, []schema.Migration, error), opts schemas.EditOpts)`、`EditOpts{Use []Use; Check bool}`。fn が返す Migration を、変更前のスキーマで各タスクに `ApplyTask` し、変わったタスクを同じ Tx で `PutTask` する（`schemacheck.go` の migrate と同じ。読めないタスクは飛ばす）
- 生成される関数から `addValue`・`deleteValue`・`orderValues` がなくなる

- [ ] **Step 1: 失敗するテストを書く**（ドメイン）
  - `ChangeStatus("todo", StatusChange{Name: ptr("open"), Label: ptr("Open")})` が定義の名前・表示名と、ワークフロー・上書きの参照を書き換え、`[]Migration{{status todo→open}}` を返す。新しい名前が既存なら `ErrNameTaken`
  - `ChangeKind` も同じ（親の参照・templates.default を書き換える）。`ChangeProp` も同じ（required を書き換える）
  - `ChangeProp("priority", PropChange{Values: &[]ValueEdit{{Name:"none"},{Name:"low",From:[]string{"low"}},{Name:"high",From:[]string{"high","urgent"}},{Name:"medium",From:[]string{"mid"}}}})` が values を `[none low high medium]` にし、Migration として value urgent→high、mid→medium を返し、削除された値はなし。どの from にもない今の値は「削除」として返す（`removed []string`、usecase が使用中を判定する）
  - 不正: 空の名前、重複した名前、同じ今の値が 2 つの from、今の値にない from → `ErrValueInvalid`。enum でないプロパティに Values → `ErrNotEnum`
  - `ChangeKind(..., KindChange{Color: ptr("teal")})` が color を設定し、`""` で消す
  - `Default()` の表示名が F7 の表のとおり。`Dev()` の review が Review
  - 値の個別の関数（`AddValue`・`RemoveValue`・`OrderValues`）は ChangeProp の Values に置き換えて消す（呼び出し元とテストも）

  関数の形: `ChangeStatus(name, ch) (*Schema, []Migration, error)`、`ChangeKind` 同じ、`ChangeProp(name, ch) (*Schema, []Migration, []string, error)`（3 つめは削除された値）。名前の変更は既存の `Migrate`（migrate.go）の rename を使い、参照の書き換えを二重に書かない。

- [ ] **Step 2: usecase**
  - `Edit` に `EditOpts` とタスクの書き換えを足す。名前の変更でタスクが書き換わり、1 コミットになることをテスト（`LastOp == "schema"`、タスクの status が新しい名前）
  - 削除された値ごとに `Use{TargetValue, prop, value}` で使用中を判定。使用中なら 422 `schema.in_use`（args に `value` も足す）
  - `ErrNameTaken` → 409 `schema.name_taken`
  - `ChangeKind` で Role・Parents を変えるときの検証（Check）は今までどおり。Name だけの変更では Check しない（名前の書き換えで違反は生まれない）

- [ ] **Step 3: サーバー**
  - 本文に `name` と（prop は）`values` を足す。`add-kind`・`change-kind` に `color`
  - `add-value`・`delete-value`・`order-values` を消す
  - テスト: 名前の変更が 1 コミットでタスクも書き換える、409 `schema.name_taken`、値の全体の編集（追加・名前の変更・マージ・削除・並び）、使用中の値の削除 422、不正な values 422、種類の color
  - `make generate && cd web && pnpm gen`

- [ ] **Step 4:** `go test ./internal/app/domain/schema/ ./internal/app/usecase/schemas/ ./internal/app/server/` が通る。`cd web && pnpm check` で、消えた関数を使っている箇所が型エラーになるのは Task C で直すので、ここではエラーの一覧を報告に残す（Task C の実装者に渡す）

- [ ] **Step 5: コミット** `feat: スキーマの編集で名前と enum の値を 1 コミットで変え、種類に色を足す`

---

### Task B: フロントの表示（種類の色・blocked_by・色の選択）

**Files:** `web/app/icons/KindIcon.svelte`、`web/core/schema-edit.ts`（`kindColorOf`）、KindIcon の呼び出し元、`web/app/detail/relation-groups.ts`・`web/app/stores/move-plan.ts`・`web/app/settings/SchemaRelations.svelte`（Blocked by）、`web/app/settings/ColorPicker.svelte`（自由な色）、各テスト

- [ ] `KindIcon` に `color?: string`。`statusColor` と同じ規則で描く（関数名は共通の `colorToken` などに改名してよい。同じ規則を 2 か所に持たない）。color がなければ今の色（bug は --danger、ほかは --icon）
- [ ] `kindColorOf(schema, name)` を足し、`kindIconOf` を渡している全呼び出し元で color も渡す
- [ ] blocked_by の画面の表示を「Blocked by」に（3 か所。逆方向の表示があればそれも英語に: Blocks）
- [ ] `ColorPicker`: 12 色と自動に加えて、自由な色の欄（`<input type="color">`。選ぶと `#rrggbb` を返す）。今の値が 12 色にない `#hex` なら、その欄を選択中にして値を入れる。safeColor を通らない値は今までどおり「カスタム（値）」で保持
- [ ] テスト: KindIcon の color、Relations の Blocked by、ColorPicker の自由な色の選択と保持
- [ ] `cd web && pnpm vitest run && pnpm check`（Task A の型エラーのうち Task C の範囲のものは残ってよい。報告に書く）
- [ ] コミット `feat(web): 種類の色と自由な色を描き、blocked_by を英語で出す`

---

### Task C: スキーマ画面（編集ダイアログ・文言・値）

**Files:** `web/app/settings/SchemaDefDialog.svelte`、`SchemaRowActions.svelte`、各タブ、`SettingsSchema.svelte`、`MigrateDialog.svelte`（rename の経路を使わなくなるなら mode を merge だけに）、`SchemaValueDialog.svelte`（消す）、`schema-action.ts`、`schema-text.ts`、`web/core/messages.ts`（`schema.name_taken`、in_use の値）、テスト、`web/e2e/schema-edit.spec.ts`・`schema.spec.ts`

- [ ] F1: 編集ダイアログの先頭に名前の欄（編集でも出す）。変わったら PATCH の `name` に入れる。成功のトーストは今の形のまま（名前が変わったら新しい名前で）。行の「名前」ボタンをなくす。MigrateDialog はマージだけにする（rename の文言と分岐を消す）
- [ ] F2: ボタンの文言を「追加」「編集」「マージ」「削除」に（aria-label も「編集（todo）」の形に）
- [ ] F4: 種類のダイアログに色（ColorPicker）。追加と編集の両方
- [ ] F5: 種類のタブと種類のダイアログの文言: 役割の列と選択肢を「グループ」「アイテム」、説明文を「役割はグループ（ほかの種類をまとめる単位）とアイテム（作業の項目）の 2 つです。どの下に置けるかに反する配置は書き込み時に拒否されます。」、列とダイアログの「置ける親」を「どの下に置けるか」、「ルート」を「最上位」、理由の文の「置ける親」も「どの下に置けるか」に（messages.ts の parents_empty、削除ダイアログ、`referencesOf` の文も）
- [ ] F6: プロパティのダイアログ（enum と multi_enum、追加と編集）に値の編集欄:
  - 値ごとに 1 行（テキスト欄、並べ替えのつまみ ⌥↑↓ とドラッグ、削除ボタン）と「値を追加」
  - 行のテキストを変えると名前の変更、行の削除は削除（使用中の値は削除ボタンを無効にし「タスクが使っているため削除できません。マージで別の値へ移してください」）
  - 行に「マージ」（同じダイアログの中で、ほかの値を選ぶとその値に統合する。統合した行は消え、統合先の from に足す）
  - 保存で PATCH の `values` に全体を送る（変わっていなければ送らない）
  - プロパティ一覧の値の行・「値を追加」ボタン・`SchemaValueDialog` をなくす
  - 削除の可否は usage.values の件数で決める（件数そのものは表示しない）
- [ ] テスト: 名前の変更が PATCH の name で送られる、名前ボタンがない、文言、種類の色、値の編集（追加・名前の変更・マージ・削除・並べ替えの body、使用中の値の削除の無効）、MigrateDialog がマージだけ
- [ ] E2E: `schema-edit.spec.ts` を新しい UI に合わせる（ボタン名、名前の変更を編集で 1 回確かめる、値の追加を編集で 1 回確かめる）。`schema.spec.ts` の rename の手順も編集ダイアログに
- [ ] `cd web && pnpm vitest run && pnpm check && pnpm lint`、`cd web && pnpm exec biome check e2e`
- [ ] コミット `feat(web): スキーマの編集ダイアログで名前と enum の値を変え、文言を直す`

---

### Task D: 仕上げ（コントローラー）

- [ ] GUI spec の決定の行に、フィードバックの変更（F1〜F7）を 1 行で追記
- [ ] 親設計のスキーマ節の追記の行に「種類は色とアイコンを持つ」を足す
- [ ] `make check` と `make e2e` を 1 回
