# フェーズ 4f-3: サイドバー・マイタスク・閲覧のみの表示・Store 間の移動 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 閲覧のみの Store で書き込みの操作を出さず（無効と理由）、非表示の Store を一覧から外し、マイタスクをワークスペース別にまとめ、接頭辞が重なる Store の ID に Store 名を付け、コメントの編集・削除を本人だけに出し、Store 間の移動を GUI でできるようにする（spec の 4f の完了条件）。

**Architecture:** フェーズ 4f の3回のうちの3回目。決定事項と spec にない細部は概要 `docs/superpowers/plans/2026-11-04-phase4f-screens.md` にあり（この回は細部 34〜51、4f-1 で決めた API の決まり 52〜70、4f-2 で決めた 71〜91、この回で足す 92〜）、各 Task はそこを前提にする（細部の番号はそこを指す）。API は 4f-1 で入っているので、画面は web だけで作る。Go は変えない。E2E の閲覧のみの Store は harness（TypeScript）でローカルの bare を書けなくして作る。API の形は変えない（足りないものが見つかったら止めて報告する）。画面ごとに 1 Task で、各 Task が画面・入口・単体テスト・E2E をまとめて入れる。

**Tech Stack:** TypeScript、Svelte 5（runes）、Vite、Vitest と @testing-library/svelte、Playwright（`web/e2e`、`harness.ts`）、Biome、svelte-check。依存は足さない。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「画面と範囲」の Sidebar・MyTasks の「後のフェーズ」の欄と「4f の細部」）、`docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`、概要 `docs/superpowers/plans/2026-11-04-phase4f-screens.md`。画面はキャンバス（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）の Sidebar・MyTasks・StoreMove を正とし、概要 plan の「作らないもの」と細部で直したところだけ変える。

**この回の Task:** Task 1（core: 色・非表示・Store 名付きの ID・パレットの `<store>:<ID>`・CLI の行・文言）、Task 2a〜2c（閲覧のみ: 判定・印・帯・TaskDetail、ツリー・カンバン・テーブル、作成・設定・ビュー・通知・パレット）、Task 3（非表示の Store）、Task 4（マイタスク）、Task 5（Store 名付きの ID とコメントの本人確認）、Task 6（Store 間の移動）、Task 7（仕上げ）。番号の順に実行する（Task 2〜6 は Task 1 の core を使う。Task 6 の入口は Task 2a の閲覧のみの判定を使う）。

**この plan の形:** 4e-4・4f-2 と同じく、web のコードは書かない（2026-11-03 に利用者が決定）。Task ごとにファイル・使うもの・作るもの・ふるまい・テストの観点を書き、実装者がテストから書く（TDD）。既存の部品と書き方（4e・4f-2 の画面）に合わせること。

**着手の前に:** main を取り込み、各 Task の「使うもの」の関数・型・部品がまだその場所と名前であることを確かめる（API の形と画面の言葉は変えない）。この plan の行番号は 2026-11-04 の main（1775b2e）のもの。

**PR:** 4f-2 の引き継ぎを完了にした docs（d56b47c）もこの回の PR に入れる（文書だけの PR は出さない。2026-11-04 に利用者が決定）。

## Global Constraints

4d・4e・4f の概要 plan と 4f-2 の plan の Global Constraints をすべて引き継ぐ。

- 画面の文言はすべて日本語。コードの識別子・コメントは英語（既存のコードと同じ）
- コンポーネントは色・寸法の 16 進数や px を直接書かず、`web/app/styles/tokens.css` の CSS 変数を使う。足りないトークンは `tokens.css` に Foundations の値で足す
- ダイアログの足元には同じ操作の `hh` コマンドを出す（`ui/CliLine.svelte`、文字列は `web/core/cli.ts`）。GUI が出す `hh` の行は実在するコマンドとフラグだけ（下の「CLI の表記」）
- エラーはサーバーの problem+json を `errorMessage`（`web/core/messages.ts`）で出す。分岐は `type`（Code）で行い、status では分岐しない（細部 65）
- 閲覧のみの判定は `StoreDirectory.readOnly(store)`、理由は `readOnlyReason`（`web/core/stores.ts`。4f-2 の細部 73）。非表示は `StoreItem.hidden`。どちらも `StoreDirectory` の購読で追従する（SSE の `stores.changed`・`sync.state` で変わる）
- 無効にした操作は消さずに `disabled` にし、理由を `title` と読み上げ（`aria-describedby` か `Menu` の `hint`）で出す。キーボードの操作は理由をトーストで出す（細部 37）
- 概要 plan の「作らないもの」は出さない（StoreMove の `purge --rewrite-history` の導線と「消すには …」の文。出さないことをテストで確かめる）
- `svelte-check` は警告ゼロ、`biome ci` は指摘ゼロ。各 Task の終わりに `cd web && pnpm fix && pnpm check && pnpm exec biome ci . && pnpm exec vitest run`（全件）。E2E は `make e2e`（その Task の spec だけを先に `cd web && pnpm exec playwright test e2e/<file>` で流してよい。そのときは `make e2e-bin` で作ったバイナリを使う）
- spec・設計と違う挙動にしたくなったら、実装を止めて報告する（自分で決めない）
- テストは前面で実行し、Bash の timeout は 600000。落ちたテストは失敗の出力と実行条件を残す
- コミットのメッセージは日本語で、末尾は `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

## CLI の表記（実際のコマンド）

| 操作 | コマンド |
|---|---|
| Store 間の移動 | `hh mv <ID> --store <名前> [--workspace <名前>] [--parent <ID>] [--scoped carry\|drop] --yes`（`--workspace` は移動先のワークスペースが2つ以上のときだけ、`--scoped` は選んだときだけ。`internal/app/cli/mv.go` のフラグ） |

## この回で足した細部（92〜）

概要 plan の細部 34〜51 に足すもの。Task 7 で概要 plan の「4f-3 の plan で足した細部」として写す。

92. **「Store が2つ以上」の判定は、表示中か否かを問わない**: `useManyStores`（`web/app/svelte-store.ts`）・`TreeView` の `multi`・`KanbanView` の `manyStores`・`useListScope` の `saveBase` は今のまま、マウント中の全 Store で数える（非表示を外すと、Store のバッジが出たり消えたりして、同じタスクの見た目が設定で変わるため。細部 43・45 も全 Store で数える）
93. **非表示の Store を外す場所**（細部 38 を、今のコードの場所に当てた）: サイドバーのワークスペース（`Sidebar.svelte` の `workspaces`。ホームで強調する「最初のワークスペース」も表示中の最初）、サイドバーの保存済みビュー（`ViewEntry.store`）、サイドバーのマイタスクの件数（`myCount`）、ツリー・カンバン・テーブル（`TreeView` の `roots`、`KanbanView`・`TableView` の `tasks`、ブロック中・中止の件数と一覧）、マイタスク、パレットのワークスペース・ビュー・Store の一覧・タスクの検索・「最近開いたもの」、作成のワークスペースの選択（`CreateTask`・`CreateProject`。`resolveCreateContext` の既定も表示中の最初）、ワークスペースの追加の Store の選択（`WorkspaceAdd`）。外すのはクライアント（`task.store`・`entry.store` を見る）。通知・アクティビティ・設定（Store ごとの設定の選択欄・同期と保守・purge）は外さない。ワークスペースやプロジェクトのページの中（URL で開いたもの）は、その Store のものなので外さない
94. **表示中が0件にはならない**: 4f-2 の細部 25 で最後の1つは非表示にできないので、「表示中の Store がない」状態は作らない（config.yaml を手で書いた場合は、今の「ワークスペースがない」表示のまま）
95. **Store 名付きの ID の計算**: API は Store 名付きの ID を返さない（`TaskView` は `id` と `store` だけ）。`StoreDirectory` に `sharedPrefixes(): Set<string>`（マウント中の全 Store の `workspaces()` の接頭辞を大文字にして、2つ以上の Store にあるもの。Go の `mount.SharedPrefixes`（`internal/app/storage/mount/mount.go`）と同じ規則。Go が削除済みのワークスペースの接頭辞も数えるかを読んで合わせ、`workspaces()` に出ないものを数える必要があれば止めて報告する）と `displayId(store, id): string`（`id` が `<接頭辞>-<番号>` の形で、接頭辞が `sharedPrefixes` にあれば `<store>:<id>`、ほかは `id`）を足す。`IdLabel` に `store?: string` を足し、渡されたら `displayId` で出す。`IdLabel` を使わず ID を出している場所（下の Task 5 の表）も `displayId` を通す。トースト・ダイアログの見出しの ID も同じ
96. **パレットの `<store>:<ID>`**: `detectMode`（`web/app/palette/search.ts`）は `:` を含む語をクエリとして扱うので、入力全体が `<マウント中の Store 名>:<ID>` の形なら ID として扱い、`GET /api/tasks/{id}` に `<store>:<ID>` をそのまま渡す（サーバーの `ForTask` が `store:ID` を解決する。`internal/app/usecase/mount_ops.go`）。Store 名でない語の `:`（`status:todo` など）は今のままクエリ
97. **`store.read_only` の文言の理由**: `messages.ts` の `store.read_only` は `args.reason`（英文）をそのまま出していたので、`readOnlyReason` を通す（理由の言い方を画面のどこでも揃える）
98. **閲覧のみの帯**: `web/app/states/StoreBands.svelte`（props `store`）を作り、閲覧のみなら「この Store は閲覧のみです（理由）」、非表示なら「この Store は非表示です」と「表示する」（細部 38）を出す。TaskDetail（今の削除済み・壊れた帯と同じ並び）、ワークスペースのページ、プロジェクトのページ、ビューのページ（その Store に保存されたビュー）の上に置く。見た目は `Banners.svelte` の `band` と同じトークン
99. **閲覧のみで無効にするものの足し分**（細部 35 を、今のコードの場所に当てた）: 同じ Store の中の移動（「別のワークスペースへ移動…」と、ツリーの字下げ・並べ替え）、カンバンの列の「+」とツリーの追加（作成）、保存済みビューのレイアウトの切り替えと列・まとめ方の保存（`views.update`。コミットを作るため）、通知の「元に戻す」（`revert`）。閲覧のみの Store のワークスペースへの作成は、作成のダイアログでそのワークスペースを無効にして理由を出す（細部 35）。プロジェクトの作成も同じ。ワークスペースの追加は、閲覧のみの Store を選択欄で無効にする（`ViewSave` と同じ形）
100. **E2E の閲覧のみの Store**: harness に `Hedgehug.readOnlyStore(name, prefix)` を足す。`Peer.publish(name, prefix)` で別の home に Store を作って push し、その bare の `objects` の下のディレクトリをすべて 0o500 にしてから（Go の `gitrepotest.LockObjects` と同じ）`hh store add <name> --remote <bare>` する。マウントの同期の `CheckAccess` が `gitrepo.Writable` で書けないことを見つけ、`not_writable`（「remote に書き込めません」）の閲覧のみになる。片付けの前に 0o700 に戻す。root で走っているときは `test.skip`（root は権限を無視するため）。E2E は `HEDGEHUG_AUTO_SYNC=0` なので、`store add` のあとに `hh sync` を流し、画面を見る前に `hh store list --json` の `read_only` が入ったことを確かめる（入らなければ harness の問題として止めて報告する）
101. **Store 間の移動のあとのキャッシュ**: `MoveStoreResultView` に `commit` がないので、`applyWrite`・`noteOwnCommit` は使わない。成功したら `cache.refreshTrees()`（画面のツリーだけを取り直す）を呼び、移動したタスクを ULID で開く（ULID は移動で変わらない）。ツリーでない一覧（マイタスク・カンバン・テーブル）は poll の `tasks.changed` で取り直る。`commit` がないので、この `tasks.changed` は外からの変更（ライブの印）として出る。これは受け入れる（2026-11-04。API の形を変えないため）
102. **移動のダイアログの見出しの子孫の数**: 予定（`MovePlanView.tasks`）の件数から1を引いた数を「子孫 n」で出す（最初の予定を取るまでは出さない）。キャンバスのコメント・作業時間の数は、数える API がないので出さない
103. **パレットの「別の Store へ移動…」**: `CommandItem.icon` に `store` を足す（`icons` に Store の形がなければ、キャンバスの SettingsNav の円柱を足す）
104. **非表示の Store も移動先に出す**: 移動先のラジオは非表示の Store も並べる（非表示は表示の設定で、書き込みの先として選べなくする理由がないため。細部 46 の「移動元は出さない」以外は全 Store）

---

### Task 1: core（細部 40・43 の core・95〜97）

**Files:**
- Modify: `web/core/stores.ts`・`stores.test.ts`（色の割り当て、`hidden(store)`・`visibleWorkspaces()`、`sharedPrefixes`・`displayId`）
- Modify: `web/core/cli.ts`・`cli.test.ts`（`cliMvStore`）
- Modify: `web/core/messages.ts`・`messages.test.ts`（細部 97）
- Modify: `web/app/palette/search.ts`・`search.test.ts`（細部 96 の判定。画面への組み込みは Task 5）
- Modify: `web/app/test-utils.ts`・`web/app/views/testkit.ts`（偽の `StoreDirectory` に足したメソッド。`pnpm check` の指摘に従う）

**使うもの:** `StoreDirectory.stores()`・`workspaces()`・`tone()`・`readOnly()`、`TONES`、`StoreItem.color`・`hidden`、`WorkspaceEntry`、`cliLine`・`Placeholder`・`Arg`、`errorMessage` の `str`、`readOnlyReason`、`detectMode`・`ID` の正規表現

**作るもの:**
- `StoreDirectory.tone(store)`（細部 40）: 閲覧のみなら `"readonly"`（今のまま）。`StoreItem.color` があればその色。なければ今の並び順の割り当て（`color` を持つ Store も順番を1つ使う。色が重なってよい）。知らない Store は今のまま `"readonly"`
- `hidden(store): boolean`（`StoreItem.hidden`。知らない Store は false）、`visibleWorkspaces(): WorkspaceEntry[]`（`workspaces()` から非表示の Store のものを外す。順は同じ）
- `sharedPrefixes(): Set<string>`・`displayId(store, id)`（細部 95）。`workspaces()` が変わったときだけ作り直す（毎回数えない）
- `cliMvStore({id, to, workspace?, parent?, scoped?}): string` → `hh mv <ID> --store <名前> [--workspace] [--parent] [--scoped] --yes`
- `isStoreQualifiedId(input, storeNames): boolean`（細部 96。`<名前>:<ID>` の `<名前>` が `storeNames` にあり、`<ID>` が `ID` の正規表現に合うこと）と、`detectMode` がそれを ID として返すこと（`detectMode` に省略できる引数 `storeNames?: string[]` を足す。省略なら今のまま。呼び出し元の `CommandPalette.svelte` が渡すのは Task 5）

**テストの観点:** `tone` の `color`・並び順・閲覧のみの灰色（色があっても）・色が重なる・知らない Store、`hidden`・`visibleWorkspaces` の順、`sharedPrefixes`（大文字小文字・1つの Store に同じ接頭辞が2つは数えない・非表示の Store も数える）、`displayId`（重なる・重ならない・ID の形でない文字列はそのまま）、`cliMvStore` の各フラグと省略、`store.read_only` の3つの理由が日本語になること、`detectMode` の `org-b:API-12`（ID）・`status:todo`（クエリ）・知らない名前の `x:API-1`（クエリ）

- [x] Step 1: `stores.test.ts` → `stores.ts`
- [x] Step 2: `cli.test.ts` → `cli.ts`、`messages.test.ts` → `messages.ts`
- [x] Step 3: `search.test.ts` → `search.ts`
- [x] Step 4: 検査を流してコミット `feat(web): Store の色と非表示、Store 名付きの ID、Store 間の移動の CLI の行`

---

### Task 2a: 閲覧のみの判定・印・帯・TaskDetail（細部 34・36・37・98・100）

**Files:**
- Create: `web/app/read-only.ts`・`read-only.test.ts`（`useReadOnly(store)`: `StoreDirectory` を購読して理由の文（`readOnlyReason` 済み）か null を返す。`readOnlyToast(reason)`: キーボードの操作で出すトースト「この Store は閲覧のみです（理由）」）
- Create: `web/app/states/StoreBands.svelte`・`StoreBands.test.ts`（閲覧のみの帯。非表示の帯は Task 3 で足す）
- Modify: `web/app/ui/StoreBadge.svelte`・テスト（閲覧のみなら `title` と読み上げを「<名前>（閲覧のみ）」。`useReadOnly` で判断する。`tone` は色だけ: 知らない Store も灰色で返るため）
- Modify: `web/app/detail/TaskDetail.svelte`・`TaskDetail.test.ts`、`DeleteDialogs.svelte`、`web/app/workspace/move-menu.ts`・テスト（`moveWorkspaceItem` に閲覧のみの無効と理由）
- Modify: `web/app/workspace/WorkspacePage.svelte`・`web/app/project/ProjectPage.svelte`・`web/app/views/ViewPage.svelte`（`StoreBands` を上に置く）
- Modify: `web/e2e/harness.ts`（細部 100 の `readOnlyStore`）
- Create: `web/e2e/readonly.spec.ts`

**使うもの:** `StoreDirectory.readOnly()`・`tone()`・`subscribe`、`readOnlyReason`、`useSubscribe`、`toast`、`ui/Button`（`disabled`・`title` が通る）、`ui/Menu` の `MenuItem.disabled`・`hint`、`Banners.svelte` の `band` のトークン、TaskDetail の `locked`（今は削除済みか壊れたとき。`readonly` を `BodyEditor`・`Checklist`・`Relations`・`Comments`・`History`・`PropsPanel`・タイトルの `FieldInput` に配っている）、harness の `Peer.publish`

**ふるまい:**
- 判定（細部 34）: `useReadOnly(store)`。`StoreItem.read_only` か `sync.read_only` のどちらかで閲覧のみ。どちらも消えれば書ける状態に戻る（同期で変わる）
- TaskDetail: 閲覧のみなら `locked` に含めて、タイトル・本文・プロパティ・チェックリスト・関連・コメントの投稿と編集と削除・履歴の復元を読むだけにする（今の配線）。ウォッチのボタン（今は削除済みのときだけ無効）・「…」の「別のワークスペースへ移動…」・削除は `disabled` と理由。上に `StoreBands`（今の削除済み・壊れた帯と同じ並び）
- 印（細部 36）: `StoreBadge` の `title` と読み上げ（`aria-label`）が「<名前>（閲覧のみ）」。灰色は今の `tone`
- サーバーが 409 `store.read_only` を返したとき（判定の前に書いた・同期で変わった）は今のまま `errorMessage`（細部 97 で理由が日本語になる）

**テストの観点（単体）:** `useReadOnly` の2つの元と戻り、`StoreBadge` の `title`・`aria-label`、`StoreBands` の文、TaskDetail の各入口が閲覧のみで読むだけ・無効（タイトル・本文・ウォッチ・「…」・削除・復元・コメント）と、書ける Store では今のまま、`moveWorkspaceItem` の理由、ワークスペース・プロジェクト・ビューのページの帯

**E2E（`readonly.spec.ts`）:** `hh.readOnlyStore("ro", "RO")`（細部 100）で、`ro` の別の home に `RO-1` を作って push しておく
- サイドバーの `ro` のワークスペースのバッジが「ro（閲覧のみ）」
- `RO-1` を開くと帯「この Store は閲覧のみです（remote に書き込めません）」、タイトル・本文が編集できない、ウォッチ・削除が無効、コメントの欄がない
- 書ける Store（`demo`）のタスクは今のまま編集できる

- [x] Step 1: `read-only.ts`・`StoreBadge`・`StoreBands` のテスト → 実装
- [x] Step 2: TaskDetail・`move-menu.ts`・各ページの帯のテスト → 実装
- [x] Step 3: harness の `readOnlyStore`（先に `hh store list --json` の `read_only` を確かめる）と `readonly.spec.ts` を書いて通す
- [x] Step 4: 検査を流してコミット `feat(web): 閲覧のみの Store の印と帯を出し、タスクの詳細を読むだけにする`

---

### Task 2b: 閲覧のみのツリー・カンバン・テーブル（細部 35・37・99）

**Files:**
- Modify: `web/app/table/TableCell.svelte`・`TableView.svelte`・テスト（閲覧のみの Store の行のセルに `readonly`。今の `TableCell` は `readonly` を渡していない）
- Modify: `web/app/kanban/KanbanView.svelte`・`drop.ts`・`dnd.ts`・テスト
- Modify: `web/app/tree/TreeView.svelte`・`keyboard.ts`・`dnd.ts`・`AddRow.svelte`・テスト
- Modify: `web/e2e/readonly.spec.ts`

**使うもの:** Task 2a の `useReadOnly`・`readOnlyToast`、`StoreDirectory.readOnly()`、`app/kanban/dnd.ts` の `cardDnd`・`cellDnd`、`drop.ts` の判定、`app/tree/keyboard.ts` の `keyAction`・`KeyAction`、`app/tree/dnd.ts` の `rowDnd`・`dropDecision`、`TreeView` の `menuItems`・`pickFromMenu`、`moveWorkspaceItem`

**ふるまい:**
- テーブル: 閲覧のみの Store の行のセルは編集の入口を出さない（読むだけ。理由は行の `title`）。「+」の作成は Task 2c の作成のダイアログに任せる
- カンバン: 閲覧のみの Store のカードはドラッグできない（`draggable` を外す）。閲覧のみの Store のカードを動かす落としは `drop.ts` の判定で断る。列の「+」は、全 Store が閲覧のみのときだけ無効（細部 99）
- ツリー: 閲覧のみの Store の行で Enter（下に追加）・Tab・Shift+Tab・Alt+↑↓・Space・S・K を受けたら書かずに `readOnlyToast`（細部 37）。ドラッグは始めない・閲覧のみの Store の行への落としも断る。追加の行（`AddRow`）は閲覧のみの Store の場所では出さない。「…」の項目は無効と理由

**テストの観点（単体）:** テーブルのセルが読むだけで、書ける Store の行は今のまま、カンバンのドラッグなしと落としの拒否と列の「+」、ツリーの各キーでトースト（書き込みの API を呼ばない）、ドラッグの開始と落としの拒否、追加の行

**E2E（`readonly.spec.ts` に足す）:** ツリーで `RO-1` を選んで Space を押すとトーストが出て、`hh show RO-1 --json` の status が変わらない。テーブルで `RO-1` のタイトルのセルが編集できない

- [ ] Step 1: テーブルのテスト → 実装
- [ ] Step 2: カンバンのテスト → 実装
- [ ] Step 3: ツリーのテスト → 実装
- [ ] Step 4: E2E を足して通す
- [ ] Step 5: 検査を流してコミット `feat(web): 閲覧のみの Store のタスクをツリー・カンバン・テーブルで編集させない`

---

### Task 2c: 閲覧のみの作成・ワークスペース・設定・ビュー・通知・パレット（細部 35・37・99）

**Files:**
- Modify: `web/app/create/CreateTask.svelte`・`CreateProject.svelte`・`context.ts`・テスト
- Modify: `web/app/workspace/WorkspaceAdd.svelte`・`WorkspaceSettings.svelte`・`WorkspaceOverview.svelte`（作成のボタン）・`web/app/sidebar/Sidebar.svelte`（ワークスペースの「…」の設定・削除）・テスト
- Modify: `web/app/project/ProjectSettings.svelte`・テスト
- Modify: `web/app/settings/SettingsSchema.svelte`（migrate の入口。Store ごとの設定の Store が閲覧のみのとき）・`DoctorSection.svelte`（その Store の修復）・`PurgeSection.svelte`（閲覧のみの Store を対象にしたとき）・`settings.test.ts` か各テスト
- Modify: `web/app/views/ViewPage.svelte`（改名・削除・レイアウトの切り替え・保存）・テスト
- Modify: `web/app/inbox/InboxPage.svelte`（`revert`）・テスト
- Modify: `web/app/palette/search.ts`・`CommandPalette.svelte`・テスト
- Modify: `web/e2e/readonly.spec.ts`

**使うもの:** Task 2a の `useReadOnly`、`WorkspaceChoice` の `isDisabled`・`note`、`resolveCreateContext`、`ViewSave.svelte` の閲覧のみの書き方（`ro`・`.off`・「閲覧のみ」）、`ui/Button`・`ui/Menu`、`taskCommands`・`CommandEnv`

**ふるまい:**
- 作成: ワークスペースの選択で閲覧のみのものは無効と理由（「<Store>（閲覧のみ）: 理由」）。今いる場所が閲覧のみなら既定は書ける最初。書けるワークスペースがなければ送信を無効にし「書き込める Store がありません」。プロジェクトの作成も同じ
- ワークスペースの追加: 閲覧のみの Store は選べない（ViewSave と同じ）。ワークスペースの設定（名前・接頭辞・削除）とサイドバーの「…」の設定は開けるが、保存・削除は無効（読むだけ）。概要の作成のボタンは無効と理由
- プロジェクトの設定・スキーマの migrate・doctor の修復・purge・ビューの改名・削除・レイアウトの切り替えと保存・通知の「元に戻す」は、その Store が閲覧のみなら無効と理由
- パレット: 閲覧のみの Store のタスクに状態の変更・移動のコマンドを出さない。作成・ワークスペースの追加は、書ける Store がなければ出さない

**テストの観点（単体）:** 作成の無効と既定と全部閲覧のみ、ワークスペースの追加・設定・サイドバーの「…」・概要の作成、プロジェクトの設定、migrate・修復・purge、ビュー、通知の「元に戻す」、パレットのコマンドの出し分け

**E2E（`readonly.spec.ts` に足す）:** 作成のダイアログで `ro` のワークスペースが無効、`demo` のワークスペースは選べる。ワークスペースの追加で `ro` が選べない

- [ ] Step 1: 作成のテスト → 実装
- [ ] Step 2: ワークスペース・プロジェクト・設定のテスト → 実装
- [ ] Step 3: ビュー・通知・パレットのテスト → 実装
- [ ] Step 4: E2E を足して通す
- [ ] Step 5: 検査を流してコミット `feat(web): 閲覧のみの Store への作成・設定・ビューの書き込みを無効にする`

---

### Task 3: 非表示の Store（細部 38〜40・93・94・98）

**Files:**
- Modify: `web/app/sidebar/Sidebar.svelte`・テスト（ワークスペース・保存済みビュー・マイタスクの件数）
- Modify: `web/app/tree/TreeView.svelte`・`web/app/kanban/KanbanView.svelte`・`web/app/table/TableView.svelte`・各テスト（全 Store の一覧で非表示の Store のタスクを外す。ワークスペース・プロジェクトのページの中は外さない）
- Modify: `web/app/palette/search.ts`・`CommandPalette.svelte`・テスト（ワークスペース・ビュー・Store の一覧・タスクの検索・最近開いたもの）
- Modify: `web/app/create/CreateTask.svelte`・`CreateProject.svelte`・`context.ts`・`web/app/workspace/WorkspaceAdd.svelte`・テスト
- Modify: `web/app/App.svelte`（ホームの最初のワークスペースは表示中の最初）
- Modify: `web/app/states/StoreBands.svelte`・テスト（非表示の帯と「表示する」）
- Create: `web/e2e/hidden.spec.ts`

**使うもの:** Task 1 の `hidden()`・`visibleWorkspaces()`、`ViewEntry.store`、`TaskView.store`、`PATCH /api/stores/{name}`（`{hidden: false}`。`StoreCard.svelte` の `patch` と同じ形で、あとに `c.stores.reload()`）、`toast`、`errorMessage`

**ふるまい:**
- 細部 93 の場所で、非表示の Store のものを外す。Store の色は非表示でも `tone` が持つので、表示し直したとき同じ色（細部 39。テストで確かめる）
- 外すのは画面の一覧だけで、取得（`watchList`・`watchTree`）の問い合わせは変えない（非表示はこのマシンの表示の設定で、問い合わせの意味を変えない。マイタスクの件数の案内（Task 4）が外したものを数える）
- ツリーで非表示の Store のタスクを親に持つ表示中の Store の子は出さない（Store をまたぐ親子はないので起きない。テストは要らない）
- URL で直接開いた非表示の Store のワークスペース・プロジェクト・タスク・ビューは見せ、上に `StoreBands` の「この Store は非表示です」と「表示する」（`PATCH {hidden: false}`。成功でトースト「<名前> を表示しました」、失敗は `errorMessage`）
- 通知・アクティビティ・設定は外さない（細部 93）

**テストの観点（単体）:** サイドバーのワークスペースとビューと件数、ホームの強調、ツリー・カンバン・テーブルの全 Store の一覧で外れる・ワークスペースのページでは外れない、ブロック中・中止の件数、パレットの各一覧と検索と最近開いたもの、作成とワークスペースの追加の選択と既定、非表示の帯と「表示する」の `PATCH` と再読み込み、通知とアクティビティは外れない、色が非表示の前後で同じ

**E2E（`hidden.spec.ts`）:** Store を2つ（`demo` と `hh store init second --prefix SC`）。`second` に `SC-1`
- `hh store set second --hidden=true` → サイドバーから `second` のワークスペースが消え、全 Store のツリーに `SC-1` が出ない、パレットで「SC-1」のタイトルを探しても出ない
- `SC-1` を URL で開くと帯が出て、「表示する」でサイドバーに戻る（`hh store list --json` の `hidden` が false）
- 非表示にしても `second` のバッジの色が表示し直したあとと同じ

- [ ] Step 1: Sidebar と App のテスト → 実装
- [ ] Step 2: ツリー・カンバン・テーブルのテスト → 実装
- [ ] Step 3: パレット・作成・ワークスペースの追加のテスト → 実装
- [ ] Step 4: `StoreBands` の非表示の帯のテスト → 実装
- [ ] Step 5: `hidden.spec.ts` を書いて通す
- [ ] Step 6: 検査を流してコミット `feat(web): 非表示の Store をサイドバー・一覧・パレット・作成から外す`

---

### Task 4: マイタスク（細部 41・42）

**Files:**
- Modify: `web/app/my/groups.ts`・`groups.test.ts`（`MyGroupBy` に `"workspace"`）
- Modify: `web/app/my/MyTasks.svelte`・`MyTasks.test.ts`（まとめ方の3つ目、非表示の Store の案内）
- Modify: `web/e2e/my.spec.ts`

**使うもの:** `groupMyTasks`・`MyGroup`、`StoreDirectory.workspaces()`（並び）・`hidden()`・`tone()`、`StoreBadge`、`hrefOf`（`/settings/stores`）

**ふるまい:**
- まとめ方は「期限」「ワークスペース」「プロジェクト」の順（キャンバスの順。細部 41 の「期限・プロジェクト・ワークスペース」はまとめ方の種類で、ボタンの順はキャンバス）。今のメニューの形のまま3つにする
- ワークスペースのまとめ: キーは `${store}/${workspace.ulid}`。見出しはワークスペースの名前と `StoreBadge`（compact）。並びはサイドバーと同じ（`workspaces()` の順。そこにないワークスペースは最後に名前順）。件数は今の見出しと同じ形
- 非表示の Store の案内（細部 42）: 問い合わせの結果のうち非表示の Store のタスクの件数を Store ごとに数え、「非表示の Store に n 件（org-b）」（Store が2つ以上なら「（org-b、org-c）」）。リンク「マウント中の Store を開く」で `/settings/stores`。0件なら出さない。キャンバスの「表示する」のボタンは作らない（細部 42 のとおり設定へのリンク）

**テストの観点:** `groupMyTasks` の `workspace`（並び・知らないワークスペース・同じ名前で Store が違う）、見出しのバッジ、まとめ方の3つ、案内の件数・複数の Store・0件で出さない・リンク先

**E2E（`my.spec.ts` に足す）:** Store を2つにして両方に自分のタスク → 「ワークスペース」で2つの見出しとバッジ。片方を非表示 → その見出しが消え、「非表示の Store に 1 件（second）」とリンク

- [ ] Step 1: `groups.test.ts` → `groups.ts`
- [ ] Step 2: `MyTasks.test.ts` → `MyTasks.svelte`
- [ ] Step 3: `my.spec.ts` を足して通す
- [ ] Step 4: 検査を流してコミット `feat(web): マイタスクをワークスペース別にまとめ、非表示の Store の件数を出す`

---

### Task 5: Store 名付きの ID とコメントの本人確認（細部 43・44・95・96）

**Files:**
- Modify: `web/app/ui/IdLabel.svelte`・テスト（`store?: string`）
- Modify（`IdLabel` の呼び出し元に `store` を渡す）: `tree/TreeRow.svelte`、`settings/PurgeSection.svelte`、`inbox/InboxPage.svelte`、`activity/ActivityPage.svelte`、`workspace/WorkspaceOverview.svelte`（2か所）、`detail/Relations.svelte`、`project/ProjectViolations.svelte`、`table/TableCell.svelte`（2か所）、`kanban/KanbanCard.svelte`、`my/MyTasks.svelte`
- Modify（`IdLabel` を使わずに ID を出す場所に `displayId`）: `palette/CommandPalette.svelte`、`detail/TaskDetail.svelte`（見出し・パンくず・帯）、`project/ProjectHeader.svelte`、`detail/TaskPicker.svelte`、`create/CreateTask.svelte`、`detail/DeleteDialogs.svelte`、`table/TableView.svelte`（まとめの見出し）、`settings/SettingsSchema.svelte`、`workspace/WorkspaceMove.svelte`・`detail/History.svelte`・`create/CreateProject.svelte`・`kanban/RequiredDialog.svelte` のトーストと見出し。実装の前に `grep -rn "\.id}" web/app --include=*.svelte` で漏れを探し、この表にないものも同じに直して、Task 7 の表に書く
- Modify: `web/app/palette/search.ts`・`CommandPalette.svelte`（Task 1 の `detectMode` に Store 名を渡す。渡すのは `c.stores.stores().map((s) => s.name)`（マウント中の全 Store）で、Task 3 で非表示を外したパレットの `env.storeNames` ではない。非表示の Store のタスクも URL と同じく `<store>:<ID>` で開けるようにするため）
- Modify: `web/app/detail/Comments.svelte`・`Comments.test.ts`（本人確認）
- Modify: `web/e2e/palette.spec.ts`・`edit.spec.ts`（か、新しく `ids.spec.ts`）

**使うもの:** Task 1 の `displayId`・`isStoreQualifiedId`、`AccountsCache`（`load()`・`accounts()`・`loaded()`・`subscribe`。`useSubscribe(c.accounts, …)` と `void c.accounts.load()`。4f-2 の細部 74）、`CommentView.author`、`AuthStatus.id`（`host/login`）

**ふるまい:**
- ID（細部 43）: 接頭辞が2つ以上の Store にあるときだけ `<store>:<ID>`。ULID のホバーは今のまま。CLI の行（`cliMv` など）の ID も同じ値を使う（`hh` は `store:ID` を受ける）
- パレット（細部 96）: `org-b:API-12` で `GET /api/tasks/org-b:API-12` を引いて開く
- コメント（細部 44）: TaskDetail を開いたら `AccountsCache.load()`。コメントの編集・削除は `author` が `accounts()` のどれかの `id` と大文字小文字を無視して一致するときだけ出す（サーバーの `config.SameAccount` は `strings.EqualFold`）。`author` が空なら誰にも出さない。読み込み中・読めないときは出さない（サーバーが断るので、出して失敗させない）。閲覧のみの Store では Task 2a のとおり出さない

**テストの観点:** `IdLabel` の `store` あり・なし・重ならない接頭辞、各呼び出し元の1つ以上で `<store>:<ID>` が出ること（ツリー・テーブル・カンバン・TaskDetail の見出し・パレット）、パレットの `store:ID` の検索、コメントの自分・他人・大文字小文字の違い・空の author・アカウントの読み込み前

**E2E:** Store を2つ（`demo`（DM）と `hh store init second --prefix DM`。同じ接頭辞）
- ツリーで両方の `DM-1` が `demo:DM-1`・`second:DM-1` で出る。パレットに `second:DM-1` を入れると `second` の方が開く
- コメント: `peer`（bob）がコメントしたタスクで、自分のコメントには「編集」「削除」があり、bob のコメントにはない（`inbox.spec.ts` の `peer` の使い方）

- [ ] Step 1: `IdLabel` のテスト → 実装、呼び出し元と `displayId` の場所
- [ ] Step 2: パレットのテスト → 実装
- [ ] Step 3: `Comments.test.ts` → `Comments.svelte`
- [ ] Step 4: E2E を書いて通す
- [ ] Step 5: 検査を流してコミット `feat(web): 接頭辞が重なる Store の ID に Store 名を付け、コメントの編集を本人だけにする`

---

### Task 6: Store 間の移動（細部 45〜50・101〜104）

**Files:**
- Create: `web/app/stores/StoreMove.svelte`・`StoreMove.test.ts`（ダイアログ）、`web/app/stores/move-plan.ts`・`move-plan.test.ts`（予定の表示の文を作る純粋な関数: 参照の種類の言い方、外す参照の並び、ボタンの文）
- Modify: `web/app/workspace/dialogs-state.svelte.ts`・`WorkspaceDialogs.svelte`（`{kind: "move-store"; task}` と `openStoreMove(task)`）
- Modify: `web/app/workspace/move-menu.ts`（`MOVE_STORE_VALUE` と `moveStoreItem(task, stores)`）、`web/app/detail/TaskDetail.svelte`・`web/app/tree/TreeView.svelte`（「…」に足す）
- Modify: `web/app/palette/search.ts`・`CommandPalette.svelte`・テスト（「別の Store へ移動…」、細部 103）
- Modify: `web/core/messages.ts`・`messages.test.ts`（足りない Code があれば）
- Create: `web/e2e/store-move.spec.ts`

**使うもの:** `POST /api/tasks/{id}/move-store/plan`（本文 `MoveStoreFields` `{to, workspace?, parent?, scoped?}`。応答 `MovePlanView`）、`POST /api/tasks/{id}/move-store`（本文 `MoveStoreInputBody`。`confirmed` に見せた `MovePlanView`。応答 `MoveStoreResultView`）、`StoreDirectory.stores()`・`workspaces()`・`readOnly()`・`tone()`・`displayId()`、`TaskPicker`（`store` に移動先の Store を渡す）、`WorkspaceMove.svelte` の書き方（`Dialog`・`FormError`・`notes`・足元のボタン）、`ui/Dialog`・`ui/Button`・`ui/CliLine`、`toast`（`link`）、`router.href({name: "task", ulid})`・`router.go`、`cache.refreshTrees()`、`errorMessage`、Task 1 の `cliMvStore`

**ふるまい:**
- 入口（細部 45）: TaskDetail・ツリーの「…」とパレットに「別の Store へ移動…」。マウント中の Store が2つ以上で、タスクの Store が閲覧のみでないときだけ出す（閲覧のみなら出さない。細部 45 は「出す条件」なので、無効ではなく出さない）
- ダイアログ（細部 46・102）: 見出し「別の Store へ移動」、下に ID・タイトル・子孫の数。移動先の Store のラジオ（色の点・名前・`remotes[0]`。閲覧のみは無効で「閲覧のみのため移動できません（理由）」。移動元は出さない。非表示の Store も出す（細部 104））。最初は未選択で、予定は取らない。Store を選ぶと、移動先のワークスペースが2つ以上なら選択欄（「<Store> のワークスペース」、未選択）、1つなら名前だけ。親は任意で「親を選ぶ」の `TaskPicker`（`store` は移動先、選んだら名前と「外す」）。Store・ワークスペース・親・scoped を変えるたびに予定を取り直す（古い応答は捨てる。`seq`）。ワークスペースが未選択のあいだは取らない
- 予定（細部 47）: 「移動先で変わること」に ID（「移動先の <ワークスペース> で振り直します」。番号は予告しない）、親（`parent` があれば「<移動元の親> から外れて、<親> の下に置きます」、なければ「<ワークスペース> のルートに置きます」。移動元に親がなければ前半を省く）、移動元（「転送ファイルを残し、古い ID からも辿れます」）。「外す参照」に `unlinked` を種類（`blocked_by` は「ブロックされている」、`props.<名前>` は `<名前>`（細部 67）、`sprint`・`releases` は「スプリント」「リリース」）・相手の `target.label`・`task.id` で並べる。0件なら節を出さない。`hidden` があれば注意の帯「見えなくなる人: …」と「移動前の内容は <移動元> の git 履歴に残り、<移動元> を読める人は過去の内容を見られます」（`purge --rewrite-history` の導線と「消すには …」は出さない。作らないもの）。`scoped.required` か `scoped.choice` があれば「プロジェクトの表示の設定」のラジオ（「持っていく」`carry`・「持っていかない」`drop`。未選択なら「移動」を無効）と、持っていくときに外れるもの（`dropped`）
- 実行（細部 48）: ボタンは「移動」（外す参照があれば「n 件の参照を外して移動」）。予定がないあいだ・取り直し中は無効。`confirmed` に見せた予定をそのまま添えて送る。409 `move.changed` なら予定を取り直して「内容が変わりました。確かめてから移動してください」を出し、ダイアログに留まる。成功したら `cache.refreshTrees()`、ダイアログを閉じ、移動先のタスクを開き（`router.go`）、トースト「<old_id> を <to> に移動しました（<new_id>）」（`tasks[0]`。ID は `displayId` を通す）。足元は `cliMvStore`（選んだ値で作る。Store が未選択なら `<Store>` の `Placeholder`）
- 違反（細部 49）: 400 `move.invalid` は `args.violations`（文字列の配列）を、422 `schema.violation` は `args.violations`（オブジェクトの配列）を `errorMessage` の今の文に任せ、ダイアログの中にタスクごとに並べる（`move.invalid` の文字列は先頭の空白を落として1行ずつ）。細部 66
- そのほかのエラー（`store.read_only`・`choice.required`・`move.exists` など）は `FormError` に `errorMessage`
- undo の `undo.blocked`（細部 50）は 4e のトーストのまま（変えない。テストで確かめるだけ）

**テストの観点（単体）:** 入口の出し分け（Store が1つ・閲覧のみ・2つ以上）、移動元を出さない・閲覧のみの無効と理由・非表示の Store を出す、ワークスペースの選択欄（1つ・2つ以上）、親の `TaskPicker` の `store`、選ぶたびの予定の取り直しと古い応答の破棄、予定の表示（ID・親の3つの形・移動元・外す参照の種類と `props.` の除去・0件で出さない・`hidden` の帯・scoped のラジオと `dropped`）、ボタンの文、`confirmed` の本文、409 で取り直し、成功で開いてトースト、`move.invalid` と `schema.violation` の並び、足元の CLI の行、作らないもの（`purge`・`rewrite-history`・「消すには」）を出さないこと、パレットのコマンド

**E2E（`store-move.spec.ts`）:** Store を2つ（`demo` と `hh store init second --prefix SC --workspace a` のあとに `hh workspace add b --prefix SB --store second`。ワークスペースのコマンドは `hh workspace --help` で確かめる）。`demo` に親子（`DM-1` の子に `DM-2`）と、`DM-1` を blocked_by に持つ `DM-3`（移動しないタスクから移動するタスクへの参照。`unlinked` はこの向きを `task: DM-3`・`target: DM-1` で返す: `internal/app/server/movestore_test.go` の `TestMoveStorePlanAPI`）
- `DM-1` の TaskDetail の「…」→「別の Store へ移動…」→ `second` → ワークスペース `a` → 予定に子孫 1・外す参照（`DM-3` の「ブロックされている」）→「1 件の参照を外して移動」→ 移動先のタスクが開き、トースト。`hh show SC-1 --json` が `second` にあり、`hh show DM-1` が転送を辿る
- 予定を出したあとに `hh` で `DM-2` に子を足してから「移動」→「内容が変わりました…」が出て、予定が取り直される
- 閲覧のみの Store（Task 2a の `readOnlyStore`）は移動先で無効
- ツリーの「…」とパレットからも開ける

- [ ] Step 1: `move-plan.test.ts` → `move-plan.ts`
- [ ] Step 2: `StoreMove.test.ts` → `StoreMove.svelte`、ダイアログの状態
- [ ] Step 3: 入口（TaskDetail・ツリー・パレット）のテスト → 実装
- [ ] Step 4: `store-move.spec.ts` を書いて通す
- [ ] Step 5: 検査を流してコミット `feat(web): 別の Store へ移動するダイアログ`

---

### Task 7: 仕上げ

- [ ] Step 1: 細部 34〜51 と 92〜104 の全行について、実装とテストの場所を plan の末尾に表で残す。足りないものがあればこの回で入れる
- [ ] Step 2: 「作らないもの」を出していないことを grep とテストで確かめる（`rewrite-history`・`purge --rewrite`・`消すには`）。spec の 4f の完了条件（フェーズ3の機能が GUI でもできる）を、概要 plan の Goal の項目ごとに、どの回のどの E2E で確かめたかを表にする
- [ ] Step 3: 概要 plan に「4f-3 の plan で足した細部」（92〜104 と、実装で決まった 105〜）を写す
- [ ] Step 4: 全体の検査（`go vet ./...`・`gofmt -l .`・`golangci-lint run ./...`・`go test -race ./...`（Go は変えないが、生成物と合わせて確かめる）、`pnpm gen` と `go generate ./internal/app/server` のあとの生成物の差分なし、`pnpm fix`・`pnpm check`・`biome ci`・`vitest`・`pnpm build`、`make e2e` を3回）を流し、結果を plan の末尾に残す
- [ ] Step 5: 引き継ぎ `docs/superpowers/plans/2026-11-04-phase4f3-handoff.md`（4f-2 と同じ形。4f の完了と、フェーズ4の次）を書いてコミット `docs(plan): 4f-3 の実装で決めた点と引き継ぎを書く`。PR はこの回の作業と引き継ぎを1つにまとめて出す（引き継ぎだけの PR は出さない）
