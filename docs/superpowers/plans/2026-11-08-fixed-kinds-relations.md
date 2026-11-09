# 種類・関連・組み込みのプロパティの固定 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 種類（6 つ）と階層、関連（5 つ）、組み込みのプロパティ（6 つ）を固定の概念にし、GitHub のコメント同期を足す。既存の Store は形式の番号と段ごとの変換コマンドで移す。

**Architecture:** 固定の表（種類・関連・組み込みのプロパティ）をドメインに 1 か所ずつ置き、スキーマ・検証・索引・クエリ・API・画面がそこを引く。形式の番号を PR ごとに上げ、`hh store convert` が段（1→2、2→3、3→4）を順に当てる。古い形を読むコードは変換の段の関数の中にだけ持つ。段はタスク 1 件の関数も持ち、履歴の読み取りと同期のマージの base もそれを通す。

**Tech Stack:** Go（huma v2、go-git、go.yaml.in/yaml/v4）、Svelte 5、Orval、Vitest、Playwright

**Spec:** `docs/superpowers/specs/2026-11-08-fixed-kinds-relations-design.md`

## Global Constraints

- PR は 4 本。Part 1〜4 がそれぞれ 1 本の PR。Part の順に進め、各 Part の終わりに `make check` と `make e2e` を 1 回ずつ回してから PR にする
- 形式の番号: Part 1 で 2、Part 2 で 3、Part 3 で 4。Part 4 は上げない
- 古い形式の Store は `hh store convert` と `hh doctor` 以外を 422 `store.old_format` で止める。新しい形式は今までどおり拒否し、409 ではなく 422 `store.newer_format` に分類する
- 変換はすぐ push する。リモートが同じ古い形式で未同期なら、先に古い形式のまま同期してから変換する。同期が失敗したときだけ 409 `store.convert_unsynced`。push していないコミットを残して新しい形式を取り込むと 409 `sync.format_diverged`
- 古い形は変換の段の関数だけが読む。各段は Store 全体の `Apply` に加えて、タスク 1 件の `ApplyTask`（`updated` を動かさない）を持つ。過去のコミットのタスクファイルを読む入口（`restore --at`・`show --at`・`undo`・inbox の取り消し・レポート・履歴の差分）は、その版の形式から最新まで `Upgrade` を当ててから使う。同期のマージは、base の形式が古ければ base にも当てる。Part 1 の段 1→2 も同じ入口に乗せる。共通の入口は Part 2 の Task 2.2 で作る
- 変換は段 2→3・3→4 で `updated` を動かさない（段 1→2 は今の時刻のまま）。テンプレートの項目と子も段ごとに書き換える
- 変換が止まる値の検査は 422 `convert.bad_value`（タスクと値の一覧）。`--drop-bad-values` で外して報告する。dry run は止める場合と同じ一覧を出して終了コード 1。仕組みは Part 2 の Task 2.2 で作り、Part 3 の段 3→4 も使う
- 変換 3→4 は、`labels:` のリストにないラベルをタスクとテンプレートから外す。リストは format 3 の schema.yaml にすでにあるトップレベルの `labels:`。残したいラベルは、変換の前に手で書いてコミットする（ガイドに書く）
- 種類: project・epic・story・task・bug・step。表示名 Project・Epic・Story・Task・Bug・Step。階層は spec の表のとおり。種類を省いたら親に合わせる（親なし→project、project・epic→task、story・task・bug→step）
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

// DefaultKindUnder は種類を省いたときの種類。parentKind "" は親なし。
func DefaultKindUnder(parentKind string) string // ""→project、project・epic→task、story・task・bug→step

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

- Run は同期している Store なら、先に fetch して未取り込み・未 push がないことを確かめる（`replicate.Unpushed` と fetch の結果）。あれば先に古い形式のまま同期し、失敗したときだけ 409 `store.convert_unsynced`。書いたら push する
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

**Files:** `internal/app/domain/model/task.go`（`Causes`・`Relates`・`Duplicates`・`Clones []ULID` を `taskKeys` の `blocked_by` の後ろに、decode/encode）、`model/template.go`（テンプレートの項目と子にも 4 つ。`targets`・`templateKeys`・`Encode`）、`domain/merge/task.go:38-53`（4 つを ULID の集合の項目として）、`merge/plan.go`（テンプレートの項目の 4 つ。blocked_by と同じ規則）、`merge/repair.go:539-570`（`removeMovedRefs` を 5 つに。572- の `dropFromProp` は props の task_ref 用なので消す）、`merge/fields.go:298`（`multi()` の `TypeTaskRef && Multi` の分岐を消す）、作成 `domain/schema/relations.go`（固定の表）、`schema.go`（`TypeTaskRef`・`Prop.Inverse`・`Prop.Symmetric`・`Default` の関連のプロパティを消す。`Prop.Multi` は残す）、`check.go`（35-65 の `requirableFields` と `builtinFilled` に 4 つ。112-132 の `inverse`・`symmetric` の検査を消す。173 の `not_requirable`。`type: task_ref` は `schema.unknown_type` で、文は「関連は固定です。hh dep か詳細の関連の欄で付けます」）、`validate.go:179-181`（`case TypeTaskRef` を消す。133 の `p.Multi` は残す）、`edit.go`（`ErrNotTaskRef` と、350-366 の `OrderProps` の関連の分けを消す）、`migrate.go`（`Multi` は残す）、`domain/schema/testdata/full.yaml`・`domain/model/testdata/task/full.md`

```go
type Relation struct {
	Name, Label, Inverse, InverseLabel string
	Get func(*model.Task) []model.ULID
	Set func(*model.Task, []model.ULID)
}
var Relations []Relation // blocked_by, causes, relates, duplicates, clones の順
// 名前なら inverse=false、逆方向の名前なら inverse=true
func RelationOf(name string) (r Relation, inverse bool, ok bool)
```

- workflow.required に 5 つの名前を書ける（requirableFields と builtinFilled）
- テスト: 5 つの表、`RelationOf` の順・逆、`type: task_ref` が `schema.unknown_type` と案内の文で拒否される、テンプレートの項目と子の 4 つの読み書きとマージ
- [ ] RED → 実装 → テスト → コミット `feat: 関連を 5 つの組み込みの項目に固定する`

### Task 2.2: 関連（索引・クエリ・usecase・サーバー・CLI）、古い形を読む共通の入口、変換（段 2→3）

この Task は、段 1→2 を含む「古い形を読む共通の入口」（spec「古い形を読む入口」）を作る。最初の段 2→3 で必要になるため。

**Files（関連）:**
- 索引・クエリ: `storage/store/refs.go`（refsOf を 5 つに。循環は blocked_by だけ。4 つの自己参照は 422 `ref.self`「自分自身は指せません」）、`storage/store/move.go`（unlink の field は `props.` なし）、`storage/index/entry.go`・`problems.go`（Targets・Reverse・refs を固定の表で。symmetric の分岐を消す）、`storage/index/build.go:220`（`cacheFormat` 12 → 13）、`storage/query/eval.go:184-232,553-622`（キーは表の名前と逆方向の名前。`.inverse` の分岐と表示名のヒントを消し、`<関連の名前>.inverse` のときだけ逆方向の名前で案内する。TypeTaskRef の case を消す）、`storage/query/sort.go:118-131`（task_ref の分岐を消す。関連は並べ替え・グループ化の対象外）、`storage/view/view.go:229-245`（MissingColumns。関連は列にならない）
- usecase: `usecase/schema_findings.go:17`（queryKeys に **8 つ**。blocks は今もある）、`usecase/refs.go`・`mount_ops.go:285-325`（`editRefs` を 5 つに）・`assign.go`（`hh set` のキーに 4 つ。逆方向のキーは `assign.readonly`）、`usecase/tasks/task.go:285-330`（`Dep` を、複数のフラグ・フラグごとに複数の相手を 1 コミットで書く形に。op 名は dep のまま）、`usecase/view.go:32,138`（task JSON と一覧の行にトップレベルの 4 つ。`relations:` の入れ子は作らない。PropView の IsRef をなくす）、`usecase/browse/read.go:425-460`（逆方向を固定の表で。キーは逆方向の名前、ラベルは表示名、行は値があるときだけ）、`usecase/notifications/inbox.go:731`（`revertSet` に 4 つ）、`usecase/stores/movestore*.go`・`movestore_view.go:47,128`、`usecase/sync_shared.go:215-229`（文言）、`remote/replicate/sync.go:1170`、`usecase/tasks/template.go:62-76`（`applyItem` に 4 つ）、`usecase/schemas/schemaview.go`（PropDef の inverse・symmetric、36-37,82）、`usecase/schemas/edit.go:145`（`ErrNotTaskRef`）、`usecasetest` のフィクスチャ
- サーバー・CLI: `server/tasks_write.go:70,97,108-115`（PATCH に 4 つ。作成の本文には足さない。`props` に関連の名前を入れた作成は 400「関連は作成後に付けます」）、`server/schemaedit.go`（`inverse`・`symmetric` の本文 55-73、151、157 と「関連」の文言 148,154,185。`multi` は残す）、`cli/dep.go`（`hh dep add|rm <id> [--blocked-by <id>…] [--causes <id>…] [--relates <id>…] [--duplicates <id>…] [--clones <id>…]`。フラグがなければ使い方のエラー。同じ組は相手ごとに `dep.invalid`）、`cli/show.go:91,102-107,130-135`、`cli/set.go:34`、`cli/completion.go:97,294`、`cli/testdata/**` と `domain/schema/testdata` の golden（`blocked_by` の行に続く関連の行）
- `make generate`

**Files（古い形を読む共通の入口。段 1→2 も乗せる）:**
- 作成 `usecase/convert/upgrade.go`、修正 `convert/step.go`・`step2.go`（段 1→2 の `ApplyTask`）

```go
// Step に足す。ApplyTask は From の形式のタスク 1 件のファイルを From+1 の形に書き換える。
// タスク 1 件で決まる変換だけを入れる（配置の繰り上げと「未分類」は入れない）。updated を動かさない。
type Step struct {
	From      int
	Apply     func(t *Tree, opt Options) (Report, error)
	ApplyTask func(raw []byte, cx TaskCx) ([]byte, error)
}
type TaskCx struct{ Schema []byte } // その版の schema.yaml
// Upgrade は from の形式のタスクのファイルを、最新の形式まで順に当てる。
func Upgrade(from int, raw []byte, cx TaskCx) ([]byte, error)
```

- `TaskCx` の形は、段が要るものに合わせて実装者が決めてよい。守るのは、段が 1 件の関数を持つことと、入口がすべて `Upgrade` を通ること
- 入口（過去のコミットのタスクファイルを読む所）: `usecase/tasks/delete.go:283-353`（`RestoreAt`）、`usecase/tasks/showat.go:51-66`、`usecase/tasks/undo.go:359,416`、`usecase/notifications/inbox.go:739`（取り消し）、`storage/reportcache/cache.go:80-110`（`readRow`。キャッシュの形式を上げる）、履歴の差分（`merge.TaskChanges` を過去の版に当てる呼び出し。`web/core/history.ts` は API の結果を使う）。実装者は、過去のコミットの blob を `DecodeTask` で読む呼び出しを grep ですべて洗い出し、通す。その版の形式は、そのコミットの `store.yaml` の `format`
- 同期のマージの base: `remote/replicate/sync.go:451-490`（mergeCommit。base の format がマージする側より古ければ、base のタスクにも `Upgrade` を当ててからマージする）、`remote/replicate/rebuild.go`、`domain/merge`（変換した base を渡す口）
- doctor: `usecase/maintenance/doctor.go`・`cli/doctor.go`（`task.legacy_field`。`props` に、今の形式で組み込みの名前になっているものが残っていれば出す。`--fix` は `Upgrade` で移す。移し先のトップレベルにすでに値があるタスクは移さず指摘を残す）

**Files（変換 段 2→3。`usecase/convert/step3.go`）:**
- 動かすのは task_ref 型の値だけ（duplicates・causes・relates の名前の定義の値は同名の項目へ、独自の名前は relates へ。定義が消えて値だけ残った duplicates・causes・relates・clones は ULID なら関連として動かす）。task_ref でない独自プロパティが 4 つの名前を持っていれば 422 `convert.prop_name_taken` で止め、重なる名前と型を一覧する
- 相互の relates は 2 本とも残し、報告に「相互の組 N 件」。独自の task_ref ごとに「<名前>: N 件を relates へ」を 1 行。確認は求めない
- 値の検査: ULID の形でない値を一覧にして止める（422 `convert.bad_value`）。`convert.Options.DropBadValues` と `hh store convert --drop-bad-values`（`cli/store_convert.go`）で外して報告する。dry run は止める場合と同じ一覧を出して終了コード 1。存在しない・削除済み・自分自身を指す値はそのまま動かす。単一の値は 1 要素のリストに、重複は 1 つに
- テンプレート（`templates/*.md` の項目と子。再帰）、ビュー（`views/*.yaml` の `query` の `<名前>.inverse:` → 逆方向の名前、独自の task_ref のキー → `relates:`、`table.columns` から消える名前を外す）、`workflow.required`（独自の task_ref の名前 → `relates`）を書き換える。元の `relates:X` は書き換えず、dry run の報告に一覧する
- schema.yaml の task_ref のプロパティと、`Default()` の duplicates・causes・relates の定義を消す。`updated` は動かさない。`StoreFormat = 3`、`steps` に段 2→3 を足す

- テスト:
  - 関連: 5 つの関連の読み書き・クエリ（順・逆）・逆方向の表示（キーは逆方向の名前）・`.inverse` が逆方向の名前を案内して拒否される・自己参照の拒否（blocked_by は循環のまま）・`hh dep` の複数のフラグと相手と、フラグなし・関連は並べ替えと列の対象外
  - 古い形を読む入口: 変換前の版（形式 2 の関連が `props`、形式 1 の種類）を `restore --at`・`show --at`・`undo`・inbox の取り消し・レポート・履歴の差分で読み、値が失われず `props` に孤立しないこと。base が古い形式のマージで、片方が外した関連が戻らないこと（2 つのクローンで）。`task.legacy_field` と `--fix`
  - 変換 段 2→3: spec「段 2→3 の細部」の各項（名前の重なりで止まる・相互の組・ULID でない値で止まる・`--drop-bad-values`・テンプレートとビューの書き換え・意味が変わる `relates:X` の報告・`updated` が動かない）、1 から 3 へ 1 回で変換、2 回目は何もしない
- [ ] RED → 実装 → テスト → コミット `feat: 関連の固定と古い形を読む入口、変換の段 2→3`

### Task 2.3: 画面（関連）

**Files:** 作成 `web/core/relations.ts`（固定の表。Go の表と一致をテスト）、`app/detail/relation-groups.ts`・`Relations.svelte`（固定の表で、関連を足すときに 5 つから選ぶ。逆方向のグループは順方向のあとに表示名で）、`TaskDetail.svelte:105,120`（blocked_by の先読みを 5 つに）、`prop-rows.ts`、`app/table/columns.ts`・`TableCell.svelte`（`taskref` のエディタを消す）、`app/stores/move-plan.ts`、`app/kanban/drop.ts:63,105,122`（必須ダイアログは英語の表示名。4 つは「詳細で足す」側）、`core/history.ts`（4 つの差分。名前は `props.` なし）、`core/schema.ts:105,115,134`、`core/doctor.ts:144-145`（`schema.inverse_*` の文を消し、`task.legacy_field` の文を足す）、`core/messages.ts:157`（`schema.not_task_ref` を消し、`schema.unknown_type` の関連の案内、自己参照、400 の文）、`app/settings/SchemaRelations.svelte` を消し、`SettingsSchema.svelte`・`schema-text.ts`・`SchemaDefDialog`（関連の部分）・`SchemaDeleteDialog`（29 行の task_ref の分岐）・`MigrateDialog`（relation の分岐）・`router.ts:20`（`SCHEMA_TABS` から relations を外す。`?tab=relations` は既定のタブへ）、`app/list/suggest.ts`（QUERY_KEYS に 8 つ）、`core/scoped.ts`（REQUIRABLE_BUILTINS に 4 つ）、`app/detail/testkit.ts` などのフィクスチャ、`pnpm gen`
- [ ] RED → 実装 → `pnpm vitest run && pnpm check && pnpm lint` → コミット `feat(web): 関連を固定の表から描く`

### Task 2.4: E2E とガイド

- E2E: 詳細で causes と clones を足し、相手の詳細に Caused by・Cloned by で出る。クエリ `caused_by:`。`schema*.spec.ts`・`screens.ts` の関連タブを直す
- ガイド（英日 `docs/guide/{,ja/}`）: `planning.md` に「関連」の節を新設（5 つの関連、`hh dep`、逆方向を含むクエリのキー、`relates:X` と `related_by:X` の 2 回で求める 1 行）、`stores.md`（段 2→3 で失うもの・変わるもの: 相互の relates、独自の task_ref の名前、クエリの意味の変化。名前が重なる独自プロパティの直し方: 旧形式の Store は `hh` で編集できないので、schema.yaml とタスクのファイルを手で直してコミットする。古いクローンの警告に関連を足す）、`maintenance.md:116`（ja は 91。補完の説明に `--causes` など）と doctor の `task.legacy_field`・`--fix`（回復の手順に足す）、`getting-started.md`（関連の 1 行）
- 既存 spec の注記は、spec 側で済み（「ドキュメントの更新」の節）。この Task では足さない
- [ ] `make check` と `make e2e`
- [ ] コミット `test(e2e): 関連の通しとガイドの更新`
- [ ] （コントローラー）親設計の関連とクエリの節を書き換え、PR を作る

---

## Part 3: 組み込みのプロパティとラベル（format 4）

### Task 3.1: プロパティ（モデルとドメイン）

**Files:** `domain/model/task.go`（`Priority string`、`Start`・`Due *Date`、`Estimate *Duration`、`Points *float64` を taskKeys に。日付と期間は `model/date.go`・`model/duration.go` の既存の型を使い、プロパティの date・duration と同じ書式にする。77-104 の decode は、読めないキーだけを `Extra` に退避して `bad_value` の違反にする。タスクは壊れた扱いにしない）、`model/template.go`・`domain/merge/plan.go:65,71`（テンプレートの項目と子にも 5 つ。`labels` はすでにある）、`domain/merge/task.go`（5 つをスカラーの項目として）、`domain/merge/yaml.go:110-128,341-354`（`labels:` を集合としてマージ。base と比べて追加・削除を反映。`priorities:` は全体が後勝ちのまま）、`domain/schema/schema.go`（`Priorities []string`・`Labels []string`、`yaml:"priorities"`・`yaml:"labels"`。キーがなければ priorities は既定の low・mid・high・urgent、labels は空。`Default` の props を消す）、作成 `domain/schema/builtins.go`（組み込みのプロパティの表: 名前・表示名・型）、`check.go`（36-39 の `requirableFields` と `builtinFilled` に 5 つ。`points` が 0 でも入っているとみなす。`priorities: []` は `schema.values_empty`、labels の空は有効。重複は大文字小文字を無視して判定）、`validate.go:55-66,146-196`（priority は `priorities` に、labels の各値は `labels` にあること。大文字小文字を無視して照合し、保存はリストの綴りにする。違反のコードは `bad_value`。ラベルのリストが空のときの文は「設定の「ラベル」で足してください」）、`edit.go:253-318,285-300`（`ChangePriorities(values []ValueEdit)`・`ChangeLabels(values []ValueEdit)`。`editValues` を使い、重複は大文字小文字を無視。名前の変更とマージは Migration（`TargetPriority`・`TargetLabel`）でタスクを書き換える。priorities の最後の 1 つは消せない）、`migrate.go`（プロパティのマージを `ErrPropMerge` で拒否。`TargetPriority`・`TargetLabel` の ApplyTask）
- [ ] RED → 実装 → テスト → コミット `feat: 組み込みのプロパティと値のリストを足す`

### Task 3.2: プロパティ（索引・クエリ・usecase・サーバー・CLI）、変換（段 3→4）

**Files:**
- 索引・クエリ: `storage/index/entry.go`（5 つの型つきの項目）、`storage/query/eval.go`（`label:` は未知の値を検査しない。`priority:` は今のまま）・`sort.go`（priority は priorities の順、start・due・estimate・points の比較と並べ替え）、`storage/view/view.go`・`name.go`（`Builtins` に 5 つ。`label` は単数のまま。start・due を gantt の既定に。MissingColumns・BadGanttProps）、`usecase/views/views.go`、`usecase/maintenance/doctor.go:576-607`（`checkViewColumns`）、`storage/reportcache/cache.go:27,80-110`（`readRow` は Task 2.2 の `Upgrade` を通した版から points を読む。`format = 2` を上げる）、`usecase/planning/sprint.go:72-86`、`usecase/reports/reports.go:518`、`usecase/browse/complete.go`
- usecase・サーバー: `usecase/view.go:148-167`（task JSON にトップレベルの 5 つ。points は数、日付は `YYYY-MM-DD`、期間は `1h30m`、値がなければ `null`。一覧の行にも載せる。`props` に出さない）、`usecase/assign.go`（`hh set priority=high` などを組み込みへ。空の値は消す）、`server/tasks_write.go:29-36,114-116`（PATCH・POST に 5 つ。消すのは `null`）、`usecase/notifications/inbox.go:727,814-857`（labels の revert、`revertProp`、日付が RFC3339 で戻る扱いを `revertField` の側にも）、`usecase/tasks/template.go`、`server/schemaedit.go`（`PATCH /api/schema/priorities`・`/api/schema/labels`、本文 `{values: [{name, from?}]}`）、`usecase/schemas/schemaview.go`（priorities・labels）、`usecase/stores/movestore.go:873-930`（priority・labels を移動先のリストで検査し、なければ 422 `move.invalid`）、`usecase/maintenance/doctor.go`（`task.legacy_field` の名前に priority・start・due・estimate・points を足す）、`usecasetest/session.go:1236-1245`（`TestSchema` が `Default()` に依存。priority・points・estimate・props を使うテストの直し）
- CLI: `cli/migrate.go:31-37`・`server/admin.go:43-54`（対象に `priority`・`label`。prop のマージは 422 `migrate.prop_merge`）、`cli/schema.go:33`（`hh schema labels add <名前>…`・`rm <名前>…`。使われていないラベルだけ消せる）、`cli/completion.go:294`（`setFields` に 5 つ）、`cli/set.go:23,34`、`cli/show.go:87,106-111`（組み込みの行）、`cli/automation.go`・`server/automations.go`（同期する項目の説明）
- GitHub のラベル（push・mirror・issue の作成でラベルを送らず、取り込まない）: `domain/automation/file.go`（`IssueFieldNames` と push の既定から labels を外す。設定に labels があれば 422）、`usecase/github_push.go:19-53,183`・`github_issues.go:55,97,133,218`・`integration/issue.go:224`・`domain/model/github_values.go:17-20,75-79`（`IssueValueKeys` の labels など）、`lib/github/issues.go:83-87`（`Client.Labels()`）、`lib/github/graphql.go:64`（タスクの取得から labels を外す。`hh gh import --label` の絞り込みは残す）、`lib/fakegh/server.go:297`・`github.go:234`・`query.go:55`。使われなくなったものは残さず消す
- テストの直し: `usecase/maintenance/maintain_test.go:352-390`（`TestMigrateNamesTheTaskOfAViolation` はプロパティのマージで書かれている。status か value の migrate に書き換え、`migrate.prop_merge` の拒否は別に足す）
- キャッシュの形式、`make generate`

**Files（変換 段 3→4。`usecase/convert/step4.go`）:**
- `props:` の priority・start・due・estimate・points をトップレベルへ（タスクとテンプレートの項目と子）。`updated` は動かさない
- priorities: すでにある `priorities:` → priority の定義の values → 既定、の順。定義が enum でなければ既定
- ラベル: format 3 の schema.yaml にすでにあるトップレベルの `labels:` をリストとして使う。リストにないラベルをタスクとテンプレートから外す。照合は大文字小文字を無視し、保存はリストの綴り。dry run は外れるラベルを一覧する。ない Store は全ラベルが外れる（残すには、変換の前に schema.yaml に `labels:` を手で書いてコミットする。ガイドに書く）
- 型が合わない値: 直せるものは直す（`"3"` → 3、datetime → 書かれたオフセットのまま日付（時刻が 00:00 でないものは、落とした時刻を報告に一覧して止めない）、1 件だけの配列 → その値、空文字 → 外す）。直せない値と、トップレベルと `props` の二重があれば、422 `convert.bad_value`（タスクと値の一覧。`--drop-bad-values` を案内）で止める。`--drop-bad-values` で、直せない値を外して報告にすべて出す。二重のキーは `props` 側を移してトップレベルを外す。dry run は止める場合と同じ一覧で終了コード 1（Task 2.2 の仕組みを使う）
- schema.yaml の組み込みと同じ名前のプロパティを消す。automations.yaml の push・mirror の項目から labels を取り除き、各タスクの `gh.issues[].synced` の `labels` とリンクごとの `fields` の `labels` も取り除く。`StoreFormat = 4`、`steps` に段 3→4 を足す
- テスト:
  - 6 つの読み書き・クエリ・並べ替え・必須の項目、ラベルが GitHub に送られず取り込まれないこと（偽の API で）、automations の labels が 422
  - 値のリスト: 編集と名前の変更・マージでのタスクの書き換え、大文字小文字を無視した照合、リストにない値の拒否、空のリストの案内、`priorities: []` の拒否、labels の同時追加のマージ、読めない値の退避と `hh set` での修正、移動先のリストにない値の拒否、プロパティのマージの拒否（CLI・API）、`hh schema labels add|rm`、`hh migrate priority|label`
  - 変換 段 3→4: spec「段 3→4 の細部」の各項、テンプレートの書き換え、1 から 4 へ 1 回で変換、2 回目は何もしない、レポートが変換をまたぐスプリントの points を読むこと
- [ ] RED → 実装 → テスト → コミット `feat: 組み込みのプロパティを索引・API・変換に通す`

### Task 3.3: 画面（プロパティ）

**Files:** `core/save.ts`（組み込みはトップレベル）、`core/schema.ts:111-119`（`BUILTIN` に 5 つ）、`core/schema-usage.ts`（優先度とラベルの数え方）、`app/detail/PropsPanel.svelte`・`prop-rows.ts`（組み込みの行は、ステータスの下に優先度・開始日・期限・見積もり・ポイント・ラベルの固定の順。優先度は priorities から選び、ラベルは labels から複数選ぶ）、ラベルを選ぶ共通の部品（リストが空なら「設定の「ラベル」で足してください」のリンク。`PropsPanel.svelte:101-110`・`app/table/TableCell.svelte:155`・`app/kanban/RequiredDialog.svelte:99-104`・`app/planning/BulkDialog.svelte:46,73`・`app/create/CreateTask.svelte` の 5 か所で使う）、`app/kanban/KanbanCard.svelte:32-35`・`app/tree/marks.ts`・`app/tree/TreeRow.svelte:57`、`core/gantt.ts`・`GanttView.svelte:178`（開始と終了は組み込みの start・due と独自の日付プロパティから選べる。書き込みは、組み込みはトップレベル、独自は `props`）、`core/planning.ts:87`・`PlanRow.svelte`、`app/my/groups.ts`・`MyTasks.svelte`、`app/detail/Worklogs.svelte:99`、`app/create/CreateProject.svelte:90-113`・`CreateTask.svelte`（作成ダイアログは優先度・期限・ラベル）、`app/table/columns.ts`（列名は今の名前。ラベルは `label`）、`app/kanban/drop.ts`、`core/scoped.ts`（REQUIRABLE_BUILTINS に 6 つ）、`core/history.ts:71-72`、`core/cli.ts`、スキーマ画面: 「優先度」「ラベル」タブ（`SchemaValues` を使う。保存で PATCH `priorities`・`labels`。優先度の最後の 1 つは消せない）、`router.ts`、`SettingsSchema.svelte`、プロパティのタブは独自のプロパティだけ（型から task_ref を除き、マージのボタンをなくす）、`MigrateDialog`（prop を除く）、`core/messages.ts`、`core/automations.ts:125`・`core/github.ts:133`・`IssueSyncTable.svelte`（同期する項目からラベルを外す）、`web/e2e/gantt.spec.ts`・`kanban.spec.ts`・`edit.spec.ts`（priority・points を props で使っている）、`pnpm gen`
- [ ] RED → 実装 → `pnpm vitest run && pnpm check && pnpm lint` → コミット `feat(web): 組み込みのプロパティと優先度・ラベルのタブ`

### Task 3.4: E2E とガイド

- E2E: ラベルのタブで値を足し、詳細でそのラベルを付ける。優先度の値の名前を変えると、タスクの優先度も変わる
- ガイド（英日 `docs/guide/{,ja/}`）: `stores.md`（変換の例の出力に段 3→4 の報告の行。**変換の前に、残したいラベルを schema.yaml のトップレベルの `labels:` に手で書いてコミットする手順**: 何もしないと全ラベルが外れる。旧形式の Store は `hh` で編集できないのでこれが唯一の手段。変換は、そのとき既にある `labels:` をリストとして使い、dry run が外れるラベルを一覧する。型が合わない値で止まる場合の例と `--drop-bad-values`）、`planning.md:85-86`（points。レポートは履歴から読む）と `:123-124`（テンプレートの `priority`）、`getting-started.md:61-76`（`labels+=ui` の例の前に、ラベルをリストに足す手順 `hh schema labels add`）、`github.md:88-90`（同期する項目からラベルを外す）
- 既存 spec の注記は、spec 側で済み。この Task では足さない
- [ ] `make check` と `make e2e` → コミット → （コントローラー）親設計のプロパティの節、PR

---

## Part 4: GitHub のコメント同期（format は上げない）

### Task 4.1: GitHub クライアントと偽の API

**Files:** `internal/lib/github/live_test.go`（実 API の定期テストにコメントの一覧・作成・更新・削除を加える）、`internal/lib/github/issues.go`（issue のコメントの一覧・作成・更新・削除。REST `GET/POST /repos/{o}/{r}/issues/{n}/comments`、`PATCH/DELETE /repos/{o}/{r}/issues/comments/{id}`。一覧は全ページを読み、途中で失敗したらエラーにする）、`internal/lib/fakegh/`（`state.go:59` の Issue に Comments（id・author・body・updated）、`server.go` の route、`admin.go` に `/_fakegh/issue/comment` の追加・編集・削除、書き込みの失敗を起こす印）、`cmd/fakegh/main.go`・`main_test.go`（`fakegh issue comment add/edit/rm`）、`tools/devenv/main.go`（fakegh の seed）、`domain/automation/file.go:33`（IssueFieldNames に comments、既定には入れない）、`domain/automation/issue.go`（`For()` が `comments` を渡す）
- [ ] RED → 実装 → テスト → コミット `feat(github): issue のコメントの API と偽の API`

### Task 4.2: push と mirror

**Files:**
- `domain/model/comment.go`（`gh` キー: `[]{ref, id, body_synced}`。`body_synced` は正規化した本文の全文）、`storage/store/tx.go:566-580`（`gh` だけを書くときは `Updated` を進めない書き込みを足す）
- push: `usecase/github_push.go`（comments が push の項目なら、作者が自分（`me`）のコメントだけを作成・更新・削除。作者が空のコメントと、削除済みで未 push のコメントは送らない。本文の末尾に `<!-- hedgehug:<コメントの ULID> -->`。作る前に issue のコメントを調べ、同じマーカーがあれば `gh` の ID を戻す。マーカーがなければ本文が同じでも書き直す。GitHub 側の本文が `body_synced` と違えば上書きして inbox に知らせる。PATCH が 404 なら作り直して `gh` を更新、DELETE の 404 は成功。1 回の同期で作るのは 30 件まで。作者が `github:` のコメントは送らない。ULID の順に作る。タスクごとの 1 コミットで `gh` を書く）、`usecase/notifications/inbox.go:527-540`（field `comments:<コメントの ULID>`）、`usecase/replication/sync.go:177-182`（`NoticeField`）
- mirror: `usecase/github_fetch.go`（repo ごとの読み取りの loop の約 105〜170 行。コメントの取得は `planMirror` の中ではなく、この読み取りの段階。全ページ読めたときだけ削除を反映し、読みきれなければ警告して何も変えない。コメントの取得は REST で、`ghstate` のレート制限の記録には入れない）、`usecase/github_apply.go:15-18`（`fetched` にコメントの一覧）、`usecase/github_issues.go`（先頭の mirror のリンクの issue だけ。コメントを取り込み・更新・削除。ULID は `DerivedULID(GitHub の投稿時刻, "<ref>#<コメントの ID>")`。作者 `github:<login>`（bot は `dependabot[bot]`、消えたユーザーは `ghost`）、`gh` を付ける。watcher は足さない）。同じ issue の `gh` の ID があるコメントは、モードが変わっても作らず取り込まない（`usecase/integration/link.go` の `dropGH` はコメントの `gh` を触らない）
- 取り込んだコメントの扱い: `usecase/comments/comment.go:141-156,48-60`（編集・削除は 422 `comment.mirrored`。検査の順は、削除済み → 取り込み（作者が `github:`）→ 作者）、`usecase/read_shared.go:36-46`（`CommentView` に `github`（真偽））、`usecase/browse/read.go:389`、`server/comments.go`、`cli/show.go`・`cli/comment.go:97`（文言）、`api/` の OpenAPI
- undo: `usecase/tasks/undo.go:74,301`（`gh` のあるコメントの追加は 422 `undo.comment_pushed` で断る。消したいときは `hh comment rm`）
- `cli/automation.go:202` 付近・`server/automations.go`（項目の説明）
- テスト（偽の API で）: push の追加・編集・削除、GitHub での手の編集の上書きと inbox（どのコメントか分かる）、mirror の追加・編集・削除、2 つのクローンが同じコメントを取り込んで 1 件に畳まれること（inbox の「上書き」にならないことも）、複数リンクとモードの切り替え、作者が自分のコメントだけ push すること、初回の 30 件の上限と残りが次の同期に回ること、マーカーによる `gh` の ID の再発見、GitHub で消されたコメントの作り直し、一覧が途中で失敗しても削除しないこと、取り込んだコメントの編集の拒否、`gh` のあるコメントの追加の undo の拒否、`gh` を古い hedgehug が保つこと、comments を項目に入れないときは何もしないこと、`gh` だけの書き込みで `Updated` が進まないこと
- [ ] RED → 実装 → テスト → コミット `feat: GitHub の issue とコメントを同期する`

### Task 4.3: 画面とガイド

**Files:** `core/automations.ts:112-127`（`ISSUE_FIELDS` に comments の行と note）・`core/github.ts:128-137`（`FIELDS` に comments。表示名「コメント」）、`IssueSyncTable.svelte`・`RuleChips.svelte`、`core/notifications.ts:240-251`（「コメント（先頭の数文字）」）、`app/detail/Comments.svelte`（`github` が真のコメントに印、編集と削除のボタンを出さない）、`core/messages.ts`（comment.mirrored）、`pnpm gen`、`docs/guide/{,ja/}github.md`（`comments` を足す前に全クローンを更新する、`@host/login`、取り込んだコメントは編集できない）、`docs/guide/{,ja/}sync.md`（コメントは 1 件 1 ファイルで LWW、`gh` を持つコメントの扱い）
- 既存 spec（phase5 の 5c）の注記は、spec 側で済み
- [ ] RED → 実装 → `pnpm vitest run && pnpm check && pnpm lint`、`make check` と `make e2e` → コミット → （コントローラー）親設計の GitHub の節、PR
