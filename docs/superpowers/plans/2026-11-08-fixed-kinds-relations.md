# 種類・関連・組み込みのプロパティの固定 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 種類（6 つ）と階層、関連（5 つ）、組み込みのプロパティ（6 つ）を固定の概念にし、GitHub のコメント同期を足す。既存の Store は形式の番号と段ごとの変換コマンドで移す。

**Architecture:** 固定の表（種類・関連・組み込みのプロパティ）をドメインに 1 か所ずつ置き、スキーマ・検証・索引・クエリ・API・画面がそこを引く。形式の番号を PR ごとに上げ、`hh store convert` が段（1→2、2→3、3→4）を順に当てる。古い形を読むコードは変換の段の中にだけ持つ。

**Tech Stack:** Go（huma v2、go-git、go.yaml.in/yaml/v4）、Svelte 5、Orval、Vitest、Playwright

**Spec:** `docs/superpowers/specs/2026-11-08-fixed-kinds-relations-design.md`

## Global Constraints

- PR は 4 本。Part 1〜4 がそれぞれ 1 本の PR。Part の順に進め、各 Part の終わりに `make check` と `make e2e` を 1 回ずつ回してから PR にする
- 形式の番号: Part 1 で 2、Part 2 で 3、Part 3 で 4。Part 4 は上げない
- 古い形式の Store は `hh store convert` と `hh doctor` 以外を 422 `store.old_format` で止める。新しい形式は今までどおり拒否し、409 ではなく 422 `store.newer_format` に分類する
- 変換は同期済み（未取り込み・未 push なし）でだけ実行し、すぐ push する。そうでなければ 409 `store.convert_unsynced`。push していないコミットを残して新しい形式を取り込むと 409 `sync.format_diverged`
- 種類: project・epic・story・task・bug・step。表示名 Project・Epic・Story・Task・Bug・Step。階層は spec の表のとおり。既定の種類は task
- 関連: blocked_by/blocks、causes/caused_by、relates/related_by、duplicates/duplicated_by、clones/cloned_by。表示名は spec の表のとおり（英語）。循環の検査は blocked_by だけ
- 組み込みのプロパティ: priority（enum、値のリスト `priorities:`、既定 low・mid・high・urgent）・start（date）・due（date）・estimate（duration）・points（number）・labels（multi_enum、値のリスト `labels:`、既定は空）。表示名は 優先度・開始日・期限・見積もり・ポイント・ラベル
- プロパティのマージは CLI・API・画面のどこでもできない（422 `migrate.prop_merge`）。ステータスと値のマージは残す
- 画面の文言は日本語。用語はグループ（project・epic）・アイテム（story・task・bug・step）・どの下に置けるか・最上位
- 全テストは各 Part の仕上げに 1 回。途中は対象のパッケージだけ
- spec と違う挙動が必要になったら、実装者は止めて報告する（コントローラーは利用者に聞く）
- 親設計（Claude Docs）の更新は各 Part の終わりにコントローラーが行う

---

## Part 1: 種類と階層、形式の番号と変換の仕組み（format 2）

### Task 1.1: 固定の種類（ドメイン）

**Files:** 作成 `internal/app/domain/schema/kinds.go`・`kinds_test.go`。修正 `schema.go`（`Kinds`・`Kind`・`Role`・`Root`・`DefaultKind`・`Default`/`Dev` の kinds、`templates` のトップレベル）、`check.go:111-141`（種類の検査を消す）、`validate.go:79-95`（ValidatePlacement）、`effective.go:100-110`（Templates）、`carry.go`（hasKind）、`edit.go`（AddKind/ChangeKind/RemoveKind/OrderKinds/ParentsEmptyError/KindChange を消す）、`migrate.go`（migrateKind と TargetKind を消す）、各テスト

**Interfaces（後のタスクが使う）:**

```go
package schema

type Role string // RoleGroup = "group", RoleItem = "item"（YAML には出ない）

type KindDef struct {
	Name, Label string
	Role        Role
	Parents     []string // "" は「ワークスペースの直下」
}

// Kinds は固定の 6 つ。並びは表示と sort:kind の順。
var Kinds = []KindDef{
	{"project", "Project", RoleGroup, []string{""}},
	{"epic", "Epic", RoleGroup, []string{"project"}},
	{"story", "Story", RoleItem, []string{"project", "epic"}},
	{"task", "Task", RoleItem, []string{"project", "epic"}},
	{"bug", "Bug", RoleItem, []string{"project", "epic"}},
	{"step", "Step", RoleItem, []string{"story", "task", "bug"}},
}

const DefaultKind = "task"

func KindOf(name string) (KindDef, bool)
func CanPlace(kind, parentKind string) bool   // parentKind "" = ワークスペースの直下
func IsGroup(kind string) bool               // project・epic
func CountsForProgress(kind string) bool      // story・task・bug
```

- `Schema` から `Kinds` を消す。`Schema.Templates *Templates`（`yaml:"templates,omitempty"`、形は scoped と同じ）を足し、Decode/Encode の `schemaKeys` に `templates` を足して `kinds` を外す。古い schema.yaml の `kinds` は `Extra` に残る（変換が読む）
- `Check()` は `Extra` に `kinds` があっても問題にしない（変換するまで Store は開かないため）。`templates.default` のキーは固定の種類だけ（`schema.unknown_kind`）
- `ValidatePlacement(kind, parentKind)` は `CanPlace` で判定する。違反のコードは今のまま（`placement`）
- `Effective.Templates()` は store の `templates.default` と scoped の `templates.default` を合わせる（scoped が優先）
- テスト: 6×7（親なしを含む）のすべての組の CanPlace、DefaultKind、Templates の合わせ方、Check の unknown_kind

- [ ] RED → 実装 → `go test ./internal/app/domain/...`
- [ ] コミット `feat(schema): 種類と階層を固定する`

### Task 1.2: 種類の固定（索引・usecase・サーバー・CLI）

**Files:** `internal/app/storage/store/validate.go:281-310`（lineage の container 判定 → IsGroup）、`storage/index/build.go:160-215`、`storage/query/sort.go:96-148`（並びは `schema.Kinds` の順）、`storage/query/eval.go:173-182`（kind: は KindOf）、`usecase/progress.go`（数えるのは CountsForProgress、ステップの内訳 `Steps{Done, Total}` を ProgressOf の結果と task view に足す）、`usecase/task_shared.go:94-101`（DefaultKind → 定数）、`usecase/destination.go`、`usecase/schemas/scoped.go:231-236`（scopedContainer → IsGroup、コード `scoped.not_group`、文「プロジェクトかエピックにだけ付けられます」）、`usecase/planning/release.go:189-220`、`usecase/tasks/template.go`・`usecase/templates/template.go:108-154`・`usecase/schema_findings.go:139-141`（`kinds[].template` → `templates.default`）、`usecase/stores/movestore.go`（種類の carry をなくし配置だけ検査）、`usecase/schemas/schemaview.go`（`kinds` を消し `templates` を足す）、`server/schemaedit.go:46-60,176-220`（種類の 4 つの route を消す）、`server/admin.go` と `cli/migrate.go:32`（migrate の kind を消す。`target: kind` は 400）、`cli/add.go` の help 文、`storage/index/build.go:240` と `storage/reportcache/cache.go:61`（キャッシュの形式を上げる）、`make generate`

- 親を指定せずにプロジェクト以外を作る、ワークスペース直下へ移す要求は、今の配置の違反として拒否されることをテスト（`hh add` に `hh use` のプロジェクトがあればそれが親になることも）
- 進捗: エピックの下にストーリー 2（1 つ完了）とその下にステップ 3（2 つ完了）→ エピックは 1/2、ストーリーの行は Steps 2/3
- [ ] RED → 実装 → 対象パッケージのテストと `go vet ./...`
- [ ] コミット `feat: 種類の固定を索引・進捗・API に通す`

### Task 1.3: 形式の番号と変換の仕組み

**Files:** `internal/app/domain/model/storemeta.go`（`StoreFormat = 2`、`ErrOldFormat`）、`usecase/session.go:405-409`（OpenStoreAt で古い形式を 422 `store.old_format`、新しい形式を 422 `store.newer_format` に）、`usecase/errors.go`・`server/problem.go`（コード）、作成 `usecase/convert/`（`convert.go`・`step.go`・`step2.go`・テスト）、作成 `cli/store_convert.go`、`usecase/maintenance/doctor.go:49-60,274`（古い形式の finding `store.old_format`、案内「hh store convert」）、`remote/replicate/sync.go:451-490`（mergeCommit で ours と theirs の store.yaml の format を比べ、違えば `ErrFormatDiverged`）、`rebuild.go`（同じ検査）、`usecase/replication/sync.go`（409 `sync.format_diverged`）

**Interfaces:**

```go
package convert

// Step は 1 段の変換。From の形式の木を From+1 の形式に書き換える。
type Step struct {
	From  int
	Apply func(t *Tree, opt Options) (Report, error)
}

// Tree は変換中の Store のファイル（.hedgehug 以下とタスク・コメント）。
// 古い形を読むのは Step の中だけ。
type Tree struct{ /* path → bytes、書き換えと削除 */ }

type Options struct{ Kinds map[string]string } // --kind 旧=新

type Report struct{ Lines []string } // 段ごとに「何をどう変えたか」1 行ずつ

// Run は今の format から最新まで段を順に当てる。dryRun なら何も書かない。
// 書くときは全段を 1 コミット（op convert）にし、format を最新にする。
func Run(ss *usecase.Session, opt Options, dryRun bool) ([]Report, error)
```

- Run は同期している Store なら、先に fetch して未取り込み・未 push がないことを確かめる（`replicate.Unpushed` と fetch の結果）。あれば 409 `store.convert_unsynced`。書いたら push する
- `sync.format_diverged` の状態（ローカルに未 push、リモートが新しい形式）では、Run はローカルの木を変換してコミットし、そのあと同期を走らせる（このときだけ未 push があっても進める）
- 段 1→2（Task 1.3 で書く）: spec の「段ごとの変換の中身」の種類・合わない配置・ワークスペース直下・`kinds[].template` の移動・schema.yaml の `kinds` の削除。テンプレートのファイルの `kind` も同じ対応で書き換える。「未分類」プロジェクトは各ワークスペースに 1 つ、タイトル「未分類」
- CLI: `hh store convert [--dry-run] [--kind old=new]… [--store name]`。dry run は段ごとの Report を出す。`--json` も
- テスト: 段 1→2 の各規則（独自の種類 → project/epic/task、--kind、ストーリーの下のタスク → step、step の下の子の繰り上げ、プロジェクトの下のプロジェクトの繰り上げ、直下のタスク → 未分類、テンプレートの移動）、dry run が書かない、2 回目は何もしない、古い形式で `hh ls` が 422、doctor の finding、未同期で 409、format_diverged からの戻り方（2 つのクローンで）
- [ ] RED → 実装 → 対象パッケージのテスト
- [ ] コミット `feat: Store の形式の番号を上げ、hh store convert を足す`

### Task 1.4: 画面（種類）

**Files:** 作成 `web/core/kinds.ts`（`KINDS`・`kindLabel`・`canPlace`・`isGroup`・`countsForProgress`、Go の表と同じ）とテスト。`web/core/schema.ts:90-125`（kindOf/canParent/parentRule を kinds.ts に）、`core/schema-edit.ts`（kindIcon/kindColor/kindRoleOf を消し、固定に）、`app/icons/KindIcon.svelte`（step の形を足す。color・icon の props をなくし kind だけで描く。bug は赤）、`IconPicker.svelte` と `ColorPicker` の種類での利用を消す、`app/tree/placement.ts`、`app/detail/PropsPanel.svelte`、`app/table/TableCell.svelte`、`app/tree/TreeView.svelte`、`app/create/CreateTask.svelte`・`CreateProject.svelte`（親に応じて置ける種類だけを出す）、`TreeRow`・`KanbanCard`・`MyTasks`・`TaskDetail`・`BacklogPage`・`ReleasesPage`・`core/planning.ts`・`list/suggest.ts`・`GanttView`・`core/gantt.ts`・`ProjectTemplates`・`CommandPalette`・`WorkspaceOverview`・`Sidebar`、進捗の表示にステップの内訳（「2/3」）を足す、`app/settings/SchemaKinds.svelte` を消し `SettingsSchema.svelte` のタブ・`SchemaDefDialog`・`SchemaDeleteDialog`・`schema-action.ts`・`MigrateDialog` の種類の部分を消す、`core/messages.ts`（store.old_format・store.newer_format・store.convert_unsynced・sync.format_diverged・scoped.not_group）、`pnpm gen`、テストのフィクスチャ（`app/tree/fixtures.ts`・`app/detail/testkit.ts`・`core/testing.ts`・`app/views/testkit.ts`）
- 古い形式の Store を開いたときの画面: その Store の画面に 422 の文と「hh store convert」を出す（ほかの Store は使える）
- [ ] RED → 実装 → `cd web && pnpm vitest run && pnpm check && pnpm lint`
- [ ] コミット `feat(web): 種類を固定の表から描く`

### Task 1.5: E2E とガイド

- E2E: 新しい Store で Project → Epic → Story → Step を作る通し（ツリーと作成ダイアログ）。ストーリーの下にタスクを置けないこと。`schema.spec.ts`・`schema-edit.spec.ts`・`create.spec.ts`・`tree.spec.ts`・`gantt.spec.ts`・`screens.ts` を直す
- ガイド: `docs/guide/{,ja/}planning.md`（テンプレートの `templates.default`、種類）、`stores.md`（hh store convert）、`maintenance.md`（doctor の古い形式）、`sync.md`（format_diverged）、`getting-started.md`
- spec の置き換えの注記: `2026-10-30-phase1a-model-format-design.md`・`2026-11-08-schema-gui-edit-design.md`・GUI spec に「種類は 2026-11-08-fixed-kinds-relations-design.md で固定」の行
- [ ] `make check` と `make e2e`
- [ ] コミット `test(e2e): 固定の種類の通しとガイドの更新`
- [ ] （コントローラー）親設計の種類・階層・進捗の節を書き換え、PR を作る

---

## Part 2: 関連の固定（format 3）

### Task 2.1: 関連（モデルとドメイン）

**Files:** `internal/app/domain/model/task.go`（`Causes`・`Relates`・`Duplicates`・`Clones []ULID` を `taskKeys` の `blocked_by` の後ろに、decode/encode）、`model/template.go`（テンプレートの項目にも 4 つ）、`domain/merge/task.go:38-53`（4 つを ULID の集合の項目として）、`merge/plan.go`、`merge/repair.go:355-415`（removeMovedRefs を 5 つに）、`merge/fields.go:297-298`、作成 `domain/schema/relations.go`（固定の表）、`schema.go`（TypeTaskRef・Prop.Inverse・Prop.Symmetric・Default の関連のプロパティを消す）、`check.go:159-164`、`validate.go:180`、`edit.go`（ErrNotTaskRef、OrderProps の関連の分け）

```go
type Relation struct {
	Name, Label, Inverse, InverseLabel string
	Get func(*model.Task) []model.ULID
	Set func(*model.Task, []model.ULID)
}
var Relations []Relation // blocked_by, causes, relates, duplicates, clones の順
func RelationOf(name string) (Relation, bool) // 名前でも逆方向の名前でも引ける（2 つめの戻り値で向き）
```

- workflow.required に 5 つの名前を書ける（requirableFields と builtinFilled）
- [ ] RED → 実装 → テスト → コミット `feat: 関連を 5 つの組み込みの項目に固定する`

### Task 2.2: 関連（索引・クエリ・usecase・サーバー・CLI・変換）

**Files:** `storage/store/refs.go`（refsOf を 5 つに、循環は blocked_by だけ）、`storage/store/move.go`（unlink の field 名）、`storage/index/entry.go`・`problems.go`（Targets・Reverse・refs を固定の表で）、`storage/query/eval.go:184-232,553-622`（キーは表の名前と逆方向の名前、`.inverse` と表示名のヒントを消す、TypeTaskRef の case を消す）、`usecase/schema_findings.go:17`（queryKeys に 9 つ）、`usecase/refs.go`・`mount_ops.go`・`assign.go`（`hh set` のキーに 4 つ）、`usecase/tasks/task.go:290-330` と `cli/dep.go`（`hh dep add|rm <id> --blocked-by|--causes|--relates|--duplicates|--clones <id>…`）、`usecase/view.go`（task JSON に `relations: {causes: [...], ...}` を足し、`blocked_by` は今のまま。PropView の IsRef をなくす）、`usecase/browse/read.go:425-460`（逆方向の表示を固定の表で）、`server/tasks_write.go:70,97`（PATCH に 4 つ）、`usecase/notifications/inbox.go:731`、`usecase/stores/movestore*.go`、`remote/replicate/sync.go:1170`、キャッシュの形式、`make generate`
- 変換の段 2→3（`usecase/convert/step3.go`）: `props:` の duplicates・causes・relates をトップレベルへ（relates は保存されていた側からの向きありになる）、独自の task_ref のプロパティの値を relates に足して消す、schema.yaml の task_ref のプロパティを消す。StoreFormat = 3
- テスト: 5 つの関連の読み書き・クエリ（順・逆）・逆方向の表示・`.inverse` が未知のキー・task_ref の型が拒否される・`hh dep` の各フラグ・段 2→3・1 から 3 へ 1 回で変換
- [ ] RED → 実装 → テスト → コミット `feat: 関連の固定を索引・クエリ・API・変換に通す`

### Task 2.3: 画面（関連）

**Files:** `app/detail/relation-groups.ts`・`Relations.svelte`（固定の表で、関連を足すときに 5 つから選ぶ）、`TaskDetail.svelte`、`prop-rows.ts`、`app/table/columns.ts`、`app/stores/move-plan.ts`、`app/kanban/drop.ts`、`core/history.ts`（4 つの差分）、`core/schema.ts`、`app/settings/SchemaRelations.svelte` を消し、`SettingsSchema.svelte`・`schema-text.ts`・`SchemaDefDialog`（関連の部分）・`SchemaDeleteDialog`・`router.ts`（relations のタブ）、`app/list/suggest.ts`（QUERY_KEYS に 9 つ）、`core/scoped.ts`（REQUIRABLE_BUILTINS に 4 つ）、テストのフィクスチャ、`pnpm gen`
- [ ] RED → 実装 → `pnpm vitest run && pnpm check && pnpm lint` → コミット `feat(web): 関連を固定の表から描く`

### Task 2.4: E2E とガイド

- E2E: 詳細で causes と clones を足し、相手の詳細に Caused by・Cloned by で出る。クエリ `caused_by:`
- ガイド: クエリのキー、`hh dep`、`maintenance.md:104`
- [ ] `make check` と `make e2e` → コミット → （コントローラー）親設計の関連とクエリの節、PR

---

## Part 3: 組み込みのプロパティとラベル（format 4）

### Task 3.1: プロパティ（モデルとドメイン）

**Files:** `domain/model/task.go`（`Priority string`、`Start`・`Due *Date`、`Estimate *Duration`、`Points *float64` を taskKeys に。日付と期間はプロパティの date・duration と同じ書式で読み書きする。既存の型があればそれを使う）、`domain/merge/task.go`（5 つをスカラーの項目として）、`domain/schema/schema.go`（`Priorities []string`・`Labels []string`、`yaml:"priorities"`・`yaml:"labels"`、Default の priorities = low・mid・high・urgent、labels = 空。Default の props を消す）、作成 `domain/schema/builtins.go`（組み込みのプロパティの表: 名前・表示名・型）、`check.go`（priorities・labels の空・重複、`requirableFields` と `builtinFilled` に 6 つ）、`validate.go:45-75`（priority は priorities に、labels の各値は labels にあること。コード `schema.bad_value`）、`edit.go`（`ChangePriorities(values []ValueEdit)`・`ChangeLabels(values []ValueEdit)`。editValues を使い、名前の変更とマージは Migration（`TargetPriority`・`TargetLabel`）でタスクを書き換える）、`migrate.go`（プロパティのマージを `ErrPropMerge` で拒否、TargetPriority・TargetLabel の ApplyTask）
- [ ] RED → 実装 → テスト → コミット `feat: 組み込みのプロパティと値のリストを足す`

### Task 3.2: プロパティ（索引・クエリ・usecase・サーバー・CLI・変換）

**Files:** `storage/index/entry.go`（5 つの型つきの項目）、`storage/query/eval.go`・`sort.go`（priority は priorities の順、start・due・estimate・points の比較と並べ替え）、`storage/view/view.go`・`name.go`（start・due を組み込みとして gantt の既定に、MissingColumns・BadGanttProps）、`usecase/views/views.go`、`usecase/maintenance/doctor.go:564-572`、`storage/reportcache/cache.go:104`、`usecase/planning/sprint.go:74`、`usecase/reports/reports.go`、`domain/report/report.go:162`、`usecase/browse/complete.go`、`usecase/view.go`（task JSON にトップレベルの 5 つ）、`usecase/assign.go`（`hh set priority=high` などを組み込みへ）、`server/tasks_write.go`（PATCH・POST に 5 つ）、`usecase/notifications/inbox.go:727-852`、`usecase/github_push.go`・`github_issues.go`（labels は値のリストにあるものだけ push・mirror する。リストにない GitHub のラベルは取り込まず、取り込めなかったことを inbox に）、`usecase/tasks/template.go`、`server/schemaedit.go`（`PATCH /api/schema/priorities`・`/api/schema/labels`、本文 `{values: [{name, from?}]}`）、`usecase/schemas/schemaview.go`（priorities・labels）、`server/admin.go`・`cli/migrate.go`（prop のマージは 422 `migrate.prop_merge`）、キャッシュの形式、`make generate`
- 変換の段 3→4（`usecase/convert/step4.go`）: `props:` の priority・start・due・estimate・points をトップレベルへ、型が合わない値は Report にして止める（422 `convert.bad_value`、タスクと値の一覧）、priority の定義の values を `priorities:` へ（定義がなければ既定）、`labels:` にないラベルをタスクから外す（dry run で一覧）、schema.yaml の組み込みと同じ名前のプロパティを消す。StoreFormat = 4
- テスト: 6 つの読み書き・クエリ・並べ替え・必須の項目、値のリストの編集と名前の変更・マージでのタスクの書き換え、リストにない値の拒否、プロパティのマージの拒否（CLI・API）、段 3→4、1 から 4 へ 1 回で変換
- [ ] RED → 実装 → テスト → コミット `feat: 組み込みのプロパティを索引・API・変換に通す`

### Task 3.3: 画面（プロパティ）

**Files:** `core/save.ts`（組み込みはトップレベル）、`app/detail/PropsPanel.svelte`・`prop-rows.ts`（組み込みの行: 優先度は priorities から選ぶ、ラベルは labels から複数選ぶ）、`app/kanban/KanbanCard.svelte`・`app/tree/marks.ts`、`core/gantt.ts`・`GanttView.svelte`、`core/planning.ts`・`PlanRow.svelte`、`app/my/groups.ts`・`MyTasks.svelte`、`app/detail/Worklogs.svelte`、`app/create/CreateProject.svelte`・`CreateTask.svelte`、`app/table/columns.ts`・`TableCell.svelte`、`app/kanban/drop.ts`・`RequiredDialog.svelte`、`core/scoped.ts`（REQUIRABLE_BUILTINS に 6 つ）、`app/planning/BulkDialog.svelte`、`core/history.ts`、スキーマ画面: 「優先度」「ラベル」タブ（`SchemaValues` を使う。保存で PATCH `priorities`・`labels`）、プロパティのタブは独自のプロパティだけ（型から task_ref を除き、マージのボタンをなくす）、`MigrateDialog`（prop を除く）、`core/messages.ts`、`pnpm gen`
- [ ] RED → 実装 → `pnpm vitest run && pnpm check && pnpm lint` → コミット `feat(web): 組み込みのプロパティと優先度・ラベルのタブ`

### Task 3.4: E2E とガイド

- E2E: ラベルのタブで値を足し、詳細でそのラベルを付ける。優先度の値の名前を変えると、タスクの優先度も変わる
- ガイド: `planning.md`（points・テンプレートの priority）、`getting-started.md:72`（labels は値のリストに必要）
- [ ] `make check` と `make e2e` → コミット → （コントローラー）親設計のプロパティの節、PR

---

## Part 4: GitHub のコメント同期（format は上げない）

### Task 4.1: GitHub クライアントと偽の API

**Files:** `internal/lib/github/issues.go`（issue のコメントの一覧・作成・更新・削除。REST `GET/POST /repos/{o}/{r}/issues/{n}/comments`、`PATCH/DELETE /repos/{o}/{r}/issues/comments/{id}`）、`internal/lib/fakegh/`（state の Issue に Comments（id・author・body・updated）、`server.go` の route、`admin.go` に `/_fakegh/issue/comment` の追加・編集・削除）、`domain/automation/file.go:33`（IssueFieldNames に comments、既定には入れない）、`automation/issue.go`
- [ ] RED → 実装 → テスト → コミット `feat(github): issue のコメントの API と偽の API`

### Task 4.2: push と mirror

**Files:** `domain/model/comment.go`（`gh` キー: `[]{ref, id, body_synced}`）、`usecase/github_push.go`（comments が push の項目なら、hedgehug のコメントを作成・更新・削除。本文の末尾に `<!-- hedgehug:<コメントの ULID> -->`。GitHub 側の本文が body_synced と違えば上書きして inbox に知らせる）、`usecase/github_issues.go`・`github_fetch.go`（mirror なら issue のコメントを取り込み、作成・更新・削除。作者 `github:<login>`、`gh` を付ける）、`usecase/comments/comment.go:141-156`（mirror で取り込んだコメントの編集・削除は 422 `comment.mirrored`）、`domain/merge` のコメントのレコードが `gh` を保つこと
- テスト（偽の API で）: push の追加・編集・削除、GitHub での手の編集の上書きと inbox、mirror の追加・編集・削除、取り込んだコメントの編集の拒否、comments を項目に入れないときは何もしない
- [ ] RED → 実装 → テスト → コミット `feat: GitHub の issue とコメントを同期する`

### Task 4.3: 画面とガイド

**Files:** `core/automations.ts`・`core/github.ts`（項目の表示名「コメント」）、`IssueSyncTable.svelte`・`RuleChips.svelte`、`app/detail/Comments.svelte`（GitHub から取り込んだコメントに印、編集と削除のボタンを出さない）、`core/messages.ts`（comment.mirrored）、`docs/guide/{,ja/}github.md`
- [ ] RED → 実装 → `pnpm vitest run && pnpm check && pnpm lint`、`make check` と `make e2e` → コミット → （コントローラー）親設計の GitHub の節、PR
