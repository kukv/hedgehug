# フェーズ 4e-4: 通知・アクティビティ・プロジェクトの設定・スキーマ・同期と保守 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通知の既読と revert、アクティビティの絞り込み、プロジェクトの上書きの編集、schema check と migrate、doctor と修復、purge を GUI で動かし、spec の「画面と範囲」をすべて GUI で動くようにする。

**Architecture:** フェーズ 4e の4回のうちの最後。決定事項と spec にない細部は概要 `docs/superpowers/plans/2026-11-03-phase4e-screens.md` にあり（この回は細部 59〜80 と、細部 13・16・18・27・34 のうち 4e-4 に送った部分、この回で足した 91〜105）、各 Task はそこを前提にする（細部の番号はそこを指す）。API・ops・CLI は 4e-1 で入っているので、この回は web だけ（Go は触らない。足りないものが見つかったら止めて報告する）。画面ごとに 1 Task で、各 Task がルート・画面・入口（サイドバー・タブ・設定のナビ・パレット）・単体テスト・E2E をまとめて入れる（入口は画面と同じ Task で出す。「まだない画面への入口は出さない」を Task の途中でも守るため）。

**Tech Stack:** TypeScript、Svelte 5（runes）、Vite、Vitest と @testing-library/svelte、Playwright（`web/e2e`、`harness.ts`）、Biome、svelte-check。依存は足さない。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「画面と範囲」の Inbox・Activity・ProjectSettings・SettingsSchema・SettingsGeneral・SettingsNav・Sidebar・CommandPalette・States、「4d・4e フロントエンド」の E2E）と、概要 `docs/superpowers/plans/2026-11-03-phase4e-screens.md`。画面はキャンバス（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）の Inbox・Activity・ProjectSettings・SettingsSchema・SettingsGeneral・SettingsNav を正とする（概要 plan の細部で作らないと決めたものは除く）。

**この回の Task:** Task 1（通知）、Task 2（アクティビティ）、Task 3（プロジェクトの設定）、Task 4（設定のナビと同期と保守の骨組み・歯車）、Task 5（doctor と修復・purge）、Task 6（スキーマ）、Task 7（仕上げ: 出さない検査の反転、E2E の全件、spec と概要 plan と引き継ぎ）。番号の順に実行する（`router.ts`・`Sidebar.svelte`・`palette/search.ts`・`App.svelte` を順に足していくため）。

**この plan の形（2026-11-03 に利用者が決定）:** 4e-3 と違い、コードは書かない。Task ごとにファイル・使うもの・作るもの・ふるまい・テストの観点を書き、実装者がテストから書く（TDD）。既存の部品と書き方（4e-2・4e-3 の画面）に合わせること。

**着手の前に:** 各 Task の「使うもの」の関数・型・部品がまだその場所と名前であることを確かめる。違えば実在のものに合わせる（API の形と画面の言葉は変えない）。

**server の前提（plan を書くときに `internal/server`・`internal/ops` のコードで確かめた。2026-11-03、main の e525019）:**
- `GET /api/notifications` は `store` を省くと全 Store の inbox を返す（開けない Store は省いて警告）。`PATCH /api/notifications/{id}`・`POST /api/notifications/read`・`POST /api/notifications/{id}/revert` も `store` を省けば全 Store から ULID で引く（`InboxStores`・`InboxReadOf`）
- `GET /api/activity` は `store` を省くと全 Store。`workspace=` の ULID・名前・接頭辞は Store ごとに引き、どの Store にもないものだけ 404 `workspace.not_found`（`ops.Session.LogIn`）。`hh log` も `--store` なしなら全 Store
- `GET /api/tasks?store=` の `q` が空なら全タスク（`-status:canceled` の既定は画面のフィルタ欄のもので、API にはない）。`is:invalid` はクエリにある（`internal/query/eval.go`）
- `ScopedView.scoped` はそのプロジェクトの上書きだけ（Store の値と合わせたものではない）。Store の値は `GET /api/schema` の `statuses`・`workflow`
- SSE の `schema.changed` はある（`internal/server/events.go`）。scoped の `PUT`・`DELETE` の後に届くかは Task 3 の最初に server のテスト（`internal/server` の scoped のテスト）で確かめ、届かないなら止めて報告する
- 2 人の利用者の作り方は `cmd/hedgehug/helpers_test.go` の `twoUsers`（bare の remote、`store init --remote`、2 人目は `store add <名前> --remote`）

## Global Constraints

4d の概要 plan と 4e の概要 plan の Global Constraints をすべて引き継ぐ。

- 画面の文言はすべて日本語。コードの識別子・コメントは英語（既存のコードと同じ）
- コンポーネントは色・寸法の 16 進数や px を直接書かず、`web/app/styles/tokens.css` の CSS 変数を使う。足りないトークンは `tokens.css` に Foundations の値で足す
- ステータスは色だけでなく形で区別する（`icons` の既存のアイコン）
- 行は ID だけを出し、ULID は ID にカーソルを合わせたときに出してコピーできるようにする（`ui/IdLabel.svelte`）
- 複数 Store の項目が並ぶ画面では、Store が2つ以上のとき項目ごとに Store のバッジを出す（`ui/StoreBadge.svelte`）
- トーストは左下に 6 秒、取り消し付きは 10 秒（`toast.svelte.ts`）。取り消しにくい操作（scoped の全削除、doctor の修復、purge、migrate）だけ確認ダイアログ（`ui/Dialog.svelte`）
- ダイアログと操作の足元には同じ操作の `hh` コマンドを出し、コピーできるようにする（`ui/CliLine.svelte`、文字列は `web/core/cli.ts` に関数を足して作る）。CLI の表記は実際のコマンドに合わせる（下の「CLI の表記」）
- エラーはサーバーの problem+json をそのまま `errorMessage`（`web/core/messages.ts`）でトーストに出す。事前に判定しない（細部 62）。足りない Code の文言は `messages.ts` に足し、`messages.test.ts` の網羅の検査を通す
- 画面の取り直しは SSE（`c.conn`）の通知で行う。`tasks.changed`・`schema.changed`・`resync`・未読数の変化（`c.conn.subscribe`）のどれで取り直すかを Task ごとに書く
- `svelte-check` は警告ゼロ、`biome ci` は指摘ゼロ。各 Task の終わりに `cd web && pnpm fix && pnpm check && pnpm exec biome ci . && pnpm exec vitest run`（全件）。E2E は `make e2e`（その Task で足した spec だけを先に `cd web && pnpm exec playwright test e2e/<file>` で流してよい）
- まだない画面への入口は出さない。後のフェーズの画面と項目（自動化の通知のタブ・自動化での絞り込み、テンプレートの欄、アカウント・Store・自動化の設定、GUI サーバーの欄、merge driver の「登録する」、インデックスの作り直し、repack、同期の間隔のプリセット）は出さない。出さないことはテストで確かめる
- spec・設計と違う挙動にしたくなったら、実装を止めて報告する（自分で決めない）
- テストは前面で実行し、Bash の timeout は 600000。落ちたテストは失敗の出力と実行条件を残す
- コミットのメッセージは日本語で、末尾は `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

## CLI の表記（実際のコマンド）

| 操作 | コマンド |
|---|---|
| 通知の一覧・既読・戻す | `hh inbox [--all] [--store S]`、`hh inbox read <ULID>... \| --all`、`hh inbox revert <ULID>` |
| アクティビティ | `hh log [--workspace a,b] [--limit N] [--store S]`（人・同期の絞り込みは CLI にない。足元には出さない） |
| プロジェクトの上書き | `hh schema scope show <ID>`、`hh schema scope set <ID> [--statuses a,b] [--transition from=to1,to2]… [--required status=f1,f2]…`、`hh schema scope rm <ID>` |
| スキーマ | `hh schema check [--store S]`、`hh schema migrate status\|kind\|prop <old> <new> [--dry-run]`、`hh schema migrate value <prop> <old> <new> [--dry-run]` |
| doctor | `hh doctor [--fix] [--store S]` |
| purge | `hh purge --older-than <N>d [--store S]` |

`--store` は Store が2つ以上のときだけ付ける（既存の `cli.ts` の関数と同じ）。

## この回で足した細部（91〜105）

概要 plan の細部 59〜80 に足すもの。Task 7 で概要 plan の「4e-4 の plan で足した細部」として写す。どれも API の形（Store ごとの API）と既存の画面の決まりから導いたもの。

91. **通知は全 Store**: `GET /api/notifications?all=true` を `store` なしで1回呼ぶ（全 Store が返る）。未読を上、その中は時刻の新しい順に並べる。Store が2つ以上なら行に Store のバッジ
92. 通知のタブの数は、そのタブの未読の数（キャンバスの数の位置）。0 なら数を出さない。「割り当て」「ウォッチ」のタブは作らず「すべて」に入る（細部 59 のタブのとおり）
93. 「すべて既読にする」は `store` なしで `POST /api/notifications/read {all: true}` を1回。未読が 0 なら無効
94. 上書きの行のボタンはキャンバスどおり「自分の値に戻す」と「このままにする」（= 既読にする）。`prefix_duplicate` などワークスペースの通知（`task` がない）は ID のリンクを出さない。ほかの未読の行は「既読にする」。行の ID のリンク（タスク詳細を開く）を押すと、その通知を既読にしてから開く（細部 61 の「行を開いたとき」）
95. 通知の一覧は、未読数（`c.conn.unread`）が変わったとき・`resync`・`tasks.changed` で取り直す（既読の付け替えはほかのタブや CLI からも起きるため）
96. **アクティビティの範囲**: `GET /api/activity` を `store` なしで呼ぶ（全 Store）。ワークスペースの絞り込みは選んだワークスペースの ULID をカンマで `workspace=` に渡す。URL の `?workspace=` も ULID のカンマ区切り。人・同期の切り替えは URL の `?sync=exclude|only`（なしはすべて）。足元は `hh log --workspace <名前,…>`（`hh log` も `--store` なしで全 Store。選んだワークスペースの名前が別の Store の名前と重なるときだけ、その分を ULID で書く）
97. アクティビティの行の時刻は author の時刻をブラウザのローカルの時刻で `HH:MM`、日付の見出しは「今日 · 11/04（水）」「昨日 · 11/03（火）」、それより前は「11/02（月）」（年が違えば `2025/11/02（日）`）。行の ID はタスク詳細へのリンク（削除済み・移動済みでも開いた先が案内する）。`tasks.changed` で取り直す
98. **プロジェクトの設定のルート**: `/projects/<ULID>/settings`。`ProjectHeader` のタブの右端に「設定」。プロジェクトでないタスク（kind が project 以外）の `/projects/<ULID>/settings` は、今のプロジェクトの画面と同じ扱い（既存の `ProjectPage` の振る舞いに合わせる）
99. ProjectSettings の「保存」は、`statuses`・`transitions`・`required` の3つのキーのうち画面で変えたキーだけを `PUT` の本文に入れる。入れたキーは差分ではなく、そのプロジェクトの上書きの全体を持つ（`transitions`・`required` は from・ステータスごとの全部の項目の map。1つの項目だけを送らない）。Store と同じに戻すときは `statuses: []`、`transitions: {}`・`required: {}`（4e-1 の「空の指定はその部分を Store の値に戻す」）。変えていなければ「保存」は無効。保存したら `schema.changed` で画面と `SchemaCache` を取り直す
100. **設定のナビ**: 「このマシン」の段に「同期と保守」（`/settings/general`）、「Store ごとの設定」の段に「ワークスペース」「スキーマ」（`/settings/schema/<store>`）。キャンバスの「スキーマとテンプレート」は「スキーマ」とする（テンプレートはフェーズ6。細部 27）。`/settings` は `/settings/general` に置き換える。サイドバーの歯車は `/settings/general`（細部 18）
101. **同期と保守の範囲**: 同期の状態・doctor は全 Store を Store ごとの段で並べる。purge は Store を選んで行う（Store が2つ以上のときだけ選択欄）。同期の間隔は Store によらず config.yaml の値なので1つだけ出す（最初の Store の `SyncStatus.interval`・`push_interval`）
102. doctor は画面を開いたときに各 Store へ `GET /api/doctor?store=` を1回送る。「もう一度実行」で取り直す。`tasks.changed` では取り直さない（重い検査のため）。修復の後は自動で取り直す
103. **スキーマの範囲**: 選んだ Store（URL）の `GET /api/schema?store=` と `GET /api/schema/check?store=`。件数は `GET /api/tasks?store=<S>`（削除済みを除く全タスク）をフロントで数える。`schema.changed`・`tasks.changed`（その Store）で取り直す
104. migrate のダイアログは新しい名前の入力欄と、入力が止まってから送る `dry_run: true` の結果（変わる件数と、`skipped` の件数）を出し、「名前を変える」（新しい名前が今の定義にないとき）か「併合する」（あるとき）で実行する。行のボタンは「名前を変える…」と「併合する…」の2つで、併合は今の定義から相手を選ぶ。値（`value`）の migrate は enum・multi_enum のプロパティの値の行に出す
105. パレットのコマンドは、その画面を作った Task で足す: 「通知を開く」（Task 1）、「アクティビティを開く」（Task 2）、「プロジェクトの設定を開く」（Task 3。プロジェクトの画面とそのタスクの詳細を開いているときだけ）、「設定の同期と保守を開く」（Task 4）、「doctor を実行」（Task 5。同期と保守を開く）、「設定のスキーマを開く」（Task 6。Store ごと）

---

### Task 1: 通知（細部 59〜63、91〜95）

**Files:**
- Create: `web/core/notifications.ts`、`web/core/notifications.test.ts`（一覧の集め方・並び・タブの分け方・文の組み立て。フレームワークに依存しない）
- Create: `web/app/inbox/InboxPage.svelte`、`web/app/inbox/InboxPage.test.ts`
- Modify: `web/app/router.ts`・`router.test.ts`（`{ name: "inbox" }` と `/inbox`）
- Modify: `web/app/App.svelte`（ルートの振り分け）
- Modify: `web/app/sidebar/Sidebar.svelte`・`Sidebar.test.ts`（「通知」の行と未読数。マイタスクの行の下、キャンバスの Sidebar の並び）
- Modify: `web/app/palette/search.ts`・`search.test.ts`・`CommandPalette.test.ts`（「通知を開く」）
- Modify: `web/core/cli.ts`・`cli.test.ts`（`cliInbox`。足元に出すものだけ）
- Modify: `web/core/messages.ts`（足りない Code があれば）
- Create: `web/e2e/inbox.spec.ts`。必要なら `web/e2e/harness.ts` に2人目の利用者を作る関数を足す

**使うもの:** `GET /api/notifications?all=true`（`Notification[]`。`store` なし）、`PATCH /api/notifications/{id}`（`{read: true}`）、`POST /api/notifications/read`（`{all: true}`）、`POST /api/notifications/{id}/revert`、`c.stores.stores()`（Store の数。バッジを出すか）、`c.conn.unread`・`c.conn.subscribe`・`c.conn.on("tasks.changed"|"resync")`、`ui/StoreBadge`・`ui/IdLabel`・`ui/Tabs`・`ui/Button`・`ui/CliLine`、`toast`、`errorMessage`

**作るもの:**
- `web/core/notifications.ts`: `type InboxTab = "all" | "overwritten" | "mention" | "repair"`、`tabOf(kind): InboxTab[]`（`overwritten` → 上書き、`mention` → メンション、`repaired`・`revived`・`renumbered`・`prefix_duplicate`・`rewrite_applied`・`purged_change` → 同期の修復。どれも「すべて」にも入る。`assigned`・`watch` は「すべて」だけ）、`sortNotifications(items)`（未読を上、時刻の新しい順、同じなら ULID の大きい順）、`notificationText(n): { label: string; text: string; mine?: string }`（kind と detail から日本語の文。下の表）
- `InboxPage.svelte`: 見出し「通知」と説明（キャンバスの「同期で届いた変更から · 自分の書き込みは通知しません · 既読はこのマシンに保存」）、「すべて既読にする」、タブ（すべて・上書き・メンション・同期の修復。数は未読）、行（未読の点、種類の名前、Store のバッジ、ID、タイトル、時刻、文、上書きなら差分とボタン）、空の状態（「通知はありません」、タブごと）、足元の `hh inbox`

**文の表（`notificationText`）:** 作者は `n.author` の login（空なら出さない）。値は JSON をそのまま短く（文字列は引用符なし、配列はカンマ区切り、null は「空」）。

| kind | 種類の名前 | 文 |
|---|---|---|
| overwritten | 上書き | 「あなたが設定した <field> が、同時に編集した値で上書きされました」（細部 60）。detail.value は負けた自分の値なので、`<field>: <value>（あなた）` を取り消し線で出す（相手の値は inbox が持たないので出さない） |
| mention | メンション | 「<作者> があなたをメンションしました」（detail.comment があればコメントで、と足す） |
| assigned | 割り当て | 「<作者> があなたを担当者に追加しました」 |
| watch | ウォッチ | 「<作者> が <changed の項目名> を変更しました」/ comment なら「<作者> がコメントしました」 |
| repaired | 同期の修復 | detail.kind ごと（`parent_cycle`・`parent_deleted`・`parent_moved`・`ref_moved`・`sprint_detached`・`release_detached` など。`internal/ops/inbox.go` の英語の文の日本語版を、同じ分け方で書く） |
| revived | 同期の修復 | 「削除と同時にあなたが編集していたため、編集を優先して復活させました」 |
| renumbered | 同期の修復 | 「ID を <old> から <new> に振り直しました。<old> でも引けます」 |
| prefix_duplicate | 同期の修復 | 「接頭辞がほかのワークスペースと重なっています」（detail の workspace があれば名前） |
| rewrite_applied | 同期の修復 | 「履歴の書き換え（purge）を取り込みました」 |
| purged_change | 同期の修復 | 「完全に削除されたタスクへの、まだ送っていない変更を捨てました」 |

文は `internal/ops/inbox.go`（CLI の `hh inbox` の英語の文）と意味を揃える。未知の kind は「種類: <kind>」の文で出す（落とさない）。

**ふるまい:**
- 行の ID・タイトルを押すと `PATCH {read: true}`（未読のときだけ）→ タスク詳細へ（⌘/Ctrl+クリック・中クリックは新しいタブで、既読にはする。`onLinkClick` と同じ）
- 「自分の値に戻す」→ `POST .../revert`。成功で「<ID> の <field> を自分の値に戻しました」のトースト（取り消しなし。戻したこと自体も CLI で `hh undo` できるが、トーストには付けない）。失敗はサーバーのエラーの文言（細部 62）
- 「このままにする」「既読にする」→ `PATCH {read: true}`
- 「すべて既読にする」は確認しない（取り消しにくい操作ではない）
- サイドバーの「通知」は `/inbox`、未読数はバッジ（0 なら出さない）。今の画面なら強調（既存の行と同じ）

**テストの観点（単体）:** タブの分け方（全 kind。割り当て・ウォッチは「すべて」にだけ入る）・並び・文の表の各行（detail の欠けを含む）・未知の kind・ワークスペースの通知に ID のリンクがない・未読数の変化で取り直す・既読・revert の成功と失敗のトースト・「すべて既読にする」と未読 0 での無効・タブの数・空の状態・Store が1つならバッジを出さない・自動化・割り当て・ウォッチのタブを出さない・足元の CLI。サイドバーは「通知」の行と未読数（`Sidebar.test.ts` の「通知とアクティビティの入口は 4e-4 まで出さない」は、通知の行が出ることとアクティビティの行がまだ出ないことに直す）

**E2E（`inbox.spec.ts`。spec の「通知の既読と revert」）:**
- 2 人目の利用者（別の `HEDGEHUG_HOME` と別のアカウント。bare の remote を作り、両方がその remote を持つ Store にする）を用意し、`cmd/hedgehug/inbox_test.go` の `conflict` と同じ順（自分が先に set → 相手が後で set → 相手 sync → 自分 sync → 相手 sync）で title の上書きを作る（LWW は時刻で決まるので、相手の set は自分の set から 1 秒以上あけてから行う。Go のテストの偽の時計の代わり）。`HEDGEHUG_AUTO_SYNC=0` のまま CLI の `hh sync` で同期する
- 通知の画面を開くと上書きの行が出て、サイドバーに未読数 1。「自分の値に戻す」でタスクのタイトルが自分の値に戻り（`hh show`）、行が既読になり、未読数が消える
- メンションの行を既読にする（コメントの `@e2e`）と未読数が減る

- [ ] Step 1: `notifications.test.ts` を書いて落ちることを確かめる → `notifications.ts` を書いて通す
- [ ] Step 2: `cli.ts` の関数のテスト → 実装
- [ ] Step 3: `router` のテスト → 実装
- [ ] Step 4: `InboxPage.test.ts` → `InboxPage.svelte`、`App.svelte` の振り分け
- [ ] Step 5: サイドバーとパレットのテストを直す → 実装
- [ ] Step 6: `inbox.spec.ts`（と harness の関数）を書いて通す
- [ ] Step 7: 検査（Global Constraints）を流してコミット `feat(web): 通知の画面（既読・すべて既読・自分の値に戻す）とサイドバーの入口`

---

### Task 2: アクティビティ（細部 64〜67、96〜97）

**Files:**
- Create: `web/core/activity.ts`・`activity.test.ts`（日付の見出しの組み立て、作者の表示、op の日本語名、trailer の行）
- Create: `web/app/activity/ActivityPage.svelte`・`ActivityPage.test.ts`、`web/app/activity/WorkspacePicker.svelte`・`WorkspacePicker.test.ts`（検索つきのチェックボックスのリスト）
- Modify: `router.ts`・`router.test.ts`（`{ name: "activity"; workspaces: string[]; sync: "" | "exclude" | "only" }` と `/activity?workspace=a,b&sync=only`。router が `?` を残すルートに足す）
- Modify: `App.svelte`、`Sidebar.svelte`・`Sidebar.test.ts`（「アクティビティ」の行。通知の下）、`palette/search.ts` とテスト（「アクティビティを開く」）、`cli.ts`・`cli.test.ts`（`cliLog`）
- Create: `web/e2e/activity.spec.ts`

**使うもの:** `GET /api/activity?limit=&sync=&workspace=`（`LogView[]`）、`c.stores.workspaces()`（Store ごとのワークスペースと接頭辞）、`c.conn.on("tasks.changed"|"resync")`、`ui/StoreBadge`・`ui/Avatar`・`ui/IdLabel`・`ui/CliLine`・`ui/Menu`（ポップオーバーの作りがあれば）

**作るもの:**
- `activity.ts`: `opLabel(op)`（`add` 作成、`set` 変更、`status` ステータスの変更、`mv` 移動、`move-in` 別の Store から移動、`move-out` 別の Store へ移動、`rm` 削除、`restore` 復元、`comment` コメント、`merge` 同期のマージ、`repair` 同期の修復、`schema` スキーマの変更、`view` ビューの変更、`workspace` ワークスペースの変更、`purge` 完全な削除、`revert-overwritten` 上書きを戻す、`undo` 取り消し。実在の op は `internal/ops` の `Hedgehug-Op` の値をすべて拾って表にし、未知の op はそのまま出す）、`authorLabel(log)`（op が merge・repair なら「同期（<作者名>）」、ほかは作者名。細部 66）、`groupByDay(logs, now)`（細部 97 の見出し）、`trailerLine(log)`（`<sha 7 桁> · <key>: <value> · …`。trailer がなければ sha だけ）
- `ActivityPage.svelte`: 見出し「アクティビティ」と説明（「各 Store の git log · 誰がいつ何を変えたか」）、ワークスペースの絞り込み（「n / m ワークスペース」のボタン → ポップオーバー: 探す欄、Store ごとの見出し、チェックボックス、「n 件を選択中 · 未選択のときはすべて」「選択を解除」）、「すべて」「人」「同期」の切り替え（自動化はフェーズ5なので出さない）、足元の `hh log`、日ごとの一覧（時刻・アバター・作者・op の名前・件名・ID・trailer の行・Store のバッジ）、「さらに読み込む」（`limit` を 100 ずつ増やす。返った件数が `limit` 未満なら出さない）、空の状態

**ふるまい:** 絞り込みを変えると URL を `router.replace` で変え、取り直す（`limit` は 100 に戻す）。探す欄は名前と接頭辞の部分一致（大文字小文字を区別しない）。ポップオーバーは Esc と外の押下で閉じる。

**テストの観点（単体）:** op の表・作者の表示（merge・repair）・日付の見出し（今日・昨日・それ以前・年違い）・trailer の行・URL と絞り込みの往復・「さらに読み込む」の出る条件と limit・探す欄・選択の解除・Store が1つならバッジなし・自動化の切り替えを出さない・足元の CLI（Store が1つ・複数）。サイドバーはアクティビティの行が出る（`Sidebar.test.ts` の 4e-4 の検査はここで消す）

**E2E（`activity.spec.ts`）:** CLI で2つのワークスペースにタスクを作り、アクティビティを開くと両方の作成の行が出る。ワークスペースを1つ選ぶと他方の行が消え、URL に `?workspace=` が付き、再読み込みしても残る。「同期」にすると行がない（同期していない）ことの空の状態

- [ ] Step 1: `activity.test.ts` → `activity.ts`
- [ ] Step 2: `cliLog` のテスト → 実装、router のテスト → 実装
- [ ] Step 3: `WorkspacePicker.test.ts` → 実装
- [ ] Step 4: `ActivityPage.test.ts` → 実装、`App.svelte`
- [ ] Step 5: サイドバーとパレットのテストを直す → 実装
- [ ] Step 6: `activity.spec.ts` を書いて通す。`routes.spec.ts` の「none of the screens of 4e-4」は、通知とアクティビティの行が出ることの検査に直す
- [ ] Step 7: 検査を流してコミット `feat(web): アクティビティの画面（人と同期・ワークスペースの絞り込み）とサイドバーの入口`

---

### Task 3: プロジェクトの設定（細部 68〜71、98〜99）

**Files:**
- Create: `web/app/project/ProjectSettings.svelte`・`ProjectSettings.test.ts`、`web/core/scoped.ts`・`scoped.test.ts`（画面の編集中の値と Store の値から `PUT` の本文を作る、違反の件数のクエリ）
- Modify: `router.ts`・`router.test.ts`（`ProjectTab` に `"settings"`）、`project/ProjectHeader.svelte`・`ProjectHeader.test.ts`（タブの右端に「設定」。`// 4e-4 adds …` のコメントを消す）、`project/ProjectPage.svelte`（設定のタブで `ProjectSettings` を出す）
- Modify: `cli.ts`・`cli.test.ts`（`cliScopeSet`・`cliScopeRm`）、`palette/search.ts` とテスト（「プロジェクトの設定を開く」）
- Create: `web/e2e/project-settings.spec.ts`

**使うもの:** 上書きの読み取りは `c.schema` の `SchemaView.scoped`（`where: project:<ULID>` の項目。scoped の GET の API はない）、`PUT /api/schema/scoped/{project}`（`ScopedPutInputBody`）、`DELETE /api/schema/scoped/{project}`、`c.schema`（Store の `SchemaView`。ステータス・カテゴリ・既定・ワークフロー）、`GET /api/tasks?q=is:invalid project:<ULID>&store=`（細部 70）、`c.conn.on("schema.changed"|"tasks.changed")`、`ui/Dialog`・`ui/Button`・`ui/CliLine`・`icons`（ステータスの形）

**作るもの:**
- 表示するステータス: Store のステータスをスキーマの順にチェックボックスで並べる（形のアイコンとカテゴリ）。カテゴリの既定のステータス（`default: true`）は外せない印（チェックを外せず「カテゴリの既定は外せません」の説明）。上書きがなければ「Store と同じ」と出し、あれば「Store と同じに戻す」
- ワークフローの制約: 遷移の制約と必須の項目を、Store の制約（外せない。「Store」の印）とこのプロジェクトで足したもの（外せる）を並べて出す。「遷移の制約を足す」（from のステータスと、行ける先のステータスの複数選択）、「必須の項目を足す」（ステータスと、項目の複数選択。項目は `SchemaView.props` の名前と組み込みの項目のうち、4e-1 の検査が通るもの）
- 「保存」（細部 99）と「すべて Store の値に戻す…」（確認ダイアログ。`templates` も外れることを出す。細部 69）。足元に今の編集を表す `hh schema scope set …`（ダイアログの足元に `hh schema scope rm <ID>`）
- いまの違反（細部 70）: 件数と ID のリンクの一覧。0 なら「違反しているタスクはありません」
- キャンバスのテンプレート・自動化・「最終更新」は作らない（細部 71）

**ふるまい:** 保存は1回の `PUT`（1コミット）。成功で「プロジェクトの設定を保存しました」のトースト。拒否（既定のステータスを外す、未知の項目など）はサーバーのエラーの文言で出し、画面の編集は残す。ほかの人の変更（`schema.changed`）が来たとき、編集中でなければ取り直し、編集中なら「ほかで変更されました」の帯と「取り直す」を出す（編集を勝手に捨てない）。

**テストの観点（単体）:** `PUT` の本文（変えた部分だけ・Store と同じに戻す `[]`・遷移と必須の追加と削除）・既定のステータスを外せない・Store の制約を外せない・保存の無効（変えていない）・全削除の確認ダイアログと `DELETE`・違反の一覧・`schema.changed` の扱い（編集中と編集中でない）・足元の CLI・テンプレートと自動化の欄がない・タブの右端に「設定」

**E2E（`project-settings.spec.ts`）:** CLI でプロジェクトを作り、設定のタブでステータスを1つ外して保存 → `hh schema scope show` に出る、プロジェクトのカンバンの列からそのステータスが消える。必須の項目（`in_progress` に `points`）を足して保存 → カンバンで進行中に動かすと必須の項目のダイアログが出る（既存の `kanban.spec.ts` と同じ確かめ方）。「すべて Store の値に戻す…」→ `hh schema scope show` が上書きなしになる

- [ ] Step 1: `scoped.test.ts` → `scoped.ts`
- [ ] Step 2: `cli.ts`・router のテスト → 実装
- [ ] Step 3: `ProjectSettings.test.ts` → 実装、`ProjectHeader`・`ProjectPage`
- [ ] Step 4: パレットのテスト → 実装
- [ ] Step 5: `project-settings.spec.ts` を書いて通す
- [ ] Step 6: 検査を流してコミット `feat(web): プロジェクトの設定タブ（表示するステータスとワークフローの制約）`

---

### Task 4: 設定のナビと同期と保守の骨組み・歯車（細部 75、100〜101）

**Files:**
- Modify: `router.ts`・`router.test.ts`（`SettingsPage` に `{ page: "general" }` と `/settings/general`。`/settings` は general に置き換える。既存の `{ page: "workspaces"; store }` は変えない。スキーマのルートは Task 6）
- Modify: `settings/SettingsRoute.svelte`・`settings/SettingsNav.svelte`・`settings/settings.test.ts`（ナビの2段。「このマシン」: 同期と保守。「Store ごとの設定」: Store の選択・ワークスペース（スキーマは Task 6）。general を開いているときの Store の選択は、Store ごとの項目のリンク先の Store を選ぶ（今の Store がないので最初の Store））
- Create: `web/app/settings/SettingsGeneral.svelte`・`SettingsGeneral.test.ts`（この Task では「同期のタイミング」と「同期の状態」だけ。doctor と purge は Task 5）
- Modify: `sidebar/Sidebar.svelte`・`Sidebar.test.ts`（歯車を `/settings/general`）、`palette/search.ts` とテスト（「設定の同期と保守を開く」）
- Modify: `web/e2e/routes.spec.ts`（歯車と `/settings` の行き先の検査を直す）

**使うもの:** `c.conn` の `sync`（Store ごとの `SyncStatus`）、`sidebar/sync-summary.ts`・`SyncCard.svelte`（4d の同期のカードと同じ値・同じ言い方を使う。部品を使い回せるならそのまま使う）、`cliSync`

**作るもの:** 見出し「同期と保守」と説明「このマシンだけの設定です（Hedgehug の設定ファイル config.yaml の sync）」（パスは `HEDGEHUG_HOME` で変わり API にもないので、キャンバスの `~/.config/hedgehug/config.yaml` は出さない。細部 27 の脚注と同じ）。「同期のタイミング」: push は `push_interval` 秒後にまとめて送る、fetch は表示中 `interval`・非表示中 10 分（細部 8 の「非表示時の 10 分は画面の説明文で出す」）、変えるには config.yaml の `sync.interval`・`sync.push_interval` を編集して `hh ui` を起動し直す旨。プリセット・GitHub の間隔は出さない（決定 2）。秒は「30 秒」「2 分」のように読める形にする（`web/core/format.ts` に関数があれば使う）。「同期の状態」: Store ごとに 4d の同期のカードと同じ内容（最後の成功・エラー・送っていない件数・「今すぐ同期」）

**テストの観点:** ルートの往復（general・workspaces、`/settings` の置き換え）・ナビの2段と今の項目の強調・Store の選択が2つ以上のときだけ・間隔の表示（秒と分）・同期の状態が Store ごと・プリセットと GUI サーバーの欄がない・歯車の行き先・パレット

- [ ] Step 1: router のテスト → 実装
- [ ] Step 2: `settings.test.ts`（ナビ）→ `SettingsNav`・`SettingsRoute`
- [ ] Step 3: `SettingsGeneral.test.ts` → 実装
- [ ] Step 4: サイドバー・パレットのテスト → 実装、`routes.spec.ts` を直して通す
- [ ] Step 5: 検査を流してコミット `feat(web): 設定の同期と保守（同期のタイミングと状態）と設定のナビの「このマシン」`

---

### Task 5: doctor と修復・purge（細部 76〜79、102）

**Files:**
- Create: `web/app/settings/DoctorSection.svelte`・`DoctorSection.test.ts`、`web/app/settings/PurgeSection.svelte`・`PurgeSection.test.ts`、`web/core/doctor.ts`・`doctor.test.ts`（finding を `check` ごとにまとめる、`check` の日本語名）
- Modify: `settings/SettingsGeneral.svelte`（2つの段を足す）、`cli.ts`・`cli.test.ts`（`cliDoctor`・`cliPurge`）、`palette/search.ts` とテスト（「doctor を実行」）、`messages.ts`（`purge.*` の文言が揃っているか確かめる）
- Create: `web/e2e/maintenance.spec.ts`

**使うもの:** `GET /api/doctor?store=`（`FindingsOutputBody`）、`POST /api/doctor/fix?store=`（`DoctorReport`）、`GET /api/purge?older_than=<N>d&store=`（`PurgePlan`）、`POST /api/purge?store=`（`{older_than, confirmed}` → `PurgeResult`。409 `purge.changed`）、`ui/Dialog`・`ui/IdLabel`・`ui/CliLine`、`toast`

**作るもの:**
- 診断（hh doctor）: Store ごとの段。finding を `check` ごとにまとめ（`check` の日本語名は `internal/doctor` などの `check` の値をすべて拾って表にする。未知の値はそのまま）、severity（エラー・警告）を形と色で出し、`msg` と `hint` をそのまま出す（英語。サーバーの文言）。`task` があれば「<ID> を開く」（ID はキャッシュかタスクの取得で引く。引けなければ ULID の短縮を出さず「タスクを開く」）。finding がなければ「問題は見つかりませんでした」。「もう一度実行」。`hh doctor` の足元
- 修復: finding があるときだけ「修復…」→ 確認ダイアログ（直すもの: 親の循環・削除済みの親の子・親と違うワークスペースのタスク・重なった ID、1コミット。force push の後なら origin から clone し直す。CLI の `--fix` の説明に合わせる。足元 `hh doctor --fix`）→ `POST /api/doctor/fix` → 結果（`repaired` の件数と一覧、`rebuilt` があればその件数、残りの finding）を段に出し、トースト「<n> 件を修復しました」
- purge: 「N 日より古いもの」の数の入力（既定 90、0 で全部）、「確かめる」→ 予定の一覧（タスクの ID・タイトル、転送ファイルの件数）。0 件なら「消すものはありません」→「完全に削除…」→ 確認ダイアログ（取り消せないこと、件数、足元 `hh purge --older-than <N>d`）→ 実行 → トースト（コミットの短い sha）。409 `purge.changed` は文言を出し、予定を取り直す。Store が2つ以上なら Store の選択欄（細部 101）。merge driver の未登録は doctor の finding のまま出す（「登録する」は作らない。細部 79）

**テストの観点:** check のまとめ方と日本語名・未知の check・finding なし・「<ID> を開く」・もう一度実行・修復の確認と結果・修復の失敗のトースト・purge の入力（負の数・空・小数は「確かめる」を無効にする）・予定の一覧・0 件・確認ダイアログ・成功・409 で取り直す・Store の選択・作らないもの（インデックスの作り直し・repack・登録する）が出ない・足元の CLI

**E2E（`maintenance.spec.ts`）:** doctor: 問題のない Store で「問題は見つかりませんでした」。タスクの問題（`task` の付く finding。壊し方は `cmd/hedgehug/doctor_test.go` から、ファイルの書き換えと git commit で作れるものを選ぶ）を作ると、その finding と「<ID> を開く」が出て、押すとタスク詳細が開く。purge: CLI でタスクを作って `hh rm`、0 日で「確かめる」→ 予定に出る → 実行 → `hh show <ID>` が見つからない

- [ ] Step 1: `doctor.test.ts` → `doctor.ts`、`cli.ts` のテスト → 実装
- [ ] Step 2: `DoctorSection.test.ts` → 実装
- [ ] Step 3: `PurgeSection.test.ts` → 実装、`SettingsGeneral` に入れる
- [ ] Step 4: パレットのテスト → 実装
- [ ] Step 5: `maintenance.spec.ts` を書いて通す
- [ ] Step 6: 検査を流してコミット `feat(web): 同期と保守の doctor と修復、期間を指定した purge`

---

### Task 6: スキーマ（細部 72〜74、103〜104）

**Files:**
- Create: `web/app/settings/SettingsSchema.svelte`・`SettingsSchema.test.ts`、`web/app/settings/MigrateDialog.svelte`・`MigrateDialog.test.ts`、`web/core/schema-usage.ts`・`schema-usage.test.ts`（定義ごとの件数）
- Modify: `router.ts`・`router.test.ts`（`SettingsPage` に `{ page: "schema"; store: string }` と `/settings/schema/<store>`）、`settings/SettingsRoute.svelte`・`SettingsNav.svelte`・`settings.test.ts`（「スキーマ」の項目）、`cli.ts`・`cli.test.ts`（`cliSchemaCheck`・`cliSchemaMigrate`）、`palette/search.ts` とテスト（「設定のスキーマを開く」Store ごと）
- Create: `web/e2e/schema.spec.ts`

**使うもの:** `GET /api/schema?store=`（`SchemaView`）、`GET /api/schema/check?store=`（`FindingsOutputBody`）、`POST /api/schema/migrate?store=`（`MigrateInputBody` → `MigrateOutputBody`）、`GET /api/tasks?store=`（件数）、`c.conn.on("schema.changed"|"tasks.changed")`、`ui/Dialog`・`ui/CliLine`・`icons`

**作るもの:**
- 見出し「スキーマ」と Store 名、説明（Store の `.hedgehug/schema.yaml` を読む。編集は YAML で行う旨。「YAML を開く」は作らない）
- ステータス（形・名前・表示名・カテゴリ・既定・件数）、種類（アイコン・名前・表示名・既定・件数）、プロパティ（名前・表示名・型・値・件数。enum・multi_enum は値ごとの件数）、ワークフロー（遷移の制約と必須の項目）の閲覧。件数は `schema-usage.ts`（ステータス・種類は一致の数、プロパティは値を持つタスクの数、値は持つタスクの数）
- 上書きのあるプロジェクトの一覧（`SchemaView.scoped` の `where`。プロジェクトの ID・タイトルとそのプロジェクトの設定タブへのリンク）
- schema check の結果（`check` ごと、なければ「スキーマとタスクは合っています」）。足元 `hh schema check`
- migrate（細部 104）: ステータス・種類・プロパティ・値の各行の「名前を変える…」「併合する…」→ `MigrateDialog`（dry_run の件数 → 確認 → 実行 → トースト「<n> 件を書き換えました」と `skipped` があればその件数）。足元 `hh schema migrate …`
- キャンバスの追加・削除・色・並び替え・未保存の変更の帯・「YAML を開く」・テンプレートは作らない（細部 73）

**テストの観点:** 各表の件数・上書きのプロジェクトの一覧とリンク・check の結果と空・migrate の dry_run の送り方（入力が止まってから、空や同じ名前では送らない）・名前を変えると併合の分かれ・実行の結果・失敗のトースト・`schema.changed` で取り直す・Store の選択で URL が変わる・作らないものが出ない・足元の CLI・ナビの「スキーマ」

**E2E（`schema.spec.ts`）:** スキーマの画面で既定のステータスの件数が CLI で作ったタスクの数と合う。プロジェクトの上書きを CLI で作ると一覧に出る。ステータスの「名前を変える…」で dry_run の件数が出て、実行すると `hh ls --json` のタスクのステータスが新しい名前になる。check は問題なしの文言

- [ ] Step 1: `schema-usage.test.ts` → 実装、`cli.ts` のテスト → 実装
- [ ] Step 2: `MigrateDialog.test.ts` → 実装
- [ ] Step 3: `SettingsSchema.test.ts` → 実装、ナビとルート
- [ ] Step 4: パレットのテスト → 実装
- [ ] Step 5: `schema.spec.ts` を書いて通す
- [ ] Step 6: 検査を流してコミット `feat(web): 設定のスキーマ（閲覧・schema check・migrate）`

---

### Task 7: 仕上げ（細部 80）

**Files:**
- Modify: 残っている「4e-4 まで出さない」の検査とコメント（`grep -rn "4e-4" web/app web/core web/e2e` が 0 件になるまで）
- Modify: `docs/superpowers/plans/2026-11-03-phase4e-screens.md`（「4e-4 の plan で足した細部」に 91〜105 を写し、実装で変えたものを直す）、`docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（4e-4 の決定の段落）、引き継ぎ `docs/superpowers/plans/2026-11-03-phase4e4-handoff.md`（新規）
- Modify: この plan の末尾に「決めたこと」（実装中に決めたもの・直したもの）

- [x] Step 1: spec の「画面と範囲」の各行（Sidebar〜SettingsNav）を1つずつ見て、フェーズ4で作るものがすべて GUI にあることを確かめ、表にして plan の末尾に残す。足りないものはこの Task で作る
- [x] Step 2: spec の E2E の一覧（作成、編集と自動保存、ツリーのドラッグ、カンバンの移動、CLI の変更が SSE で反映、通知の既読と revert、保存済みビュー、undo、削除と復元）がすべて `web/e2e` にあることを確かめる
- [x] Step 3: 全体の検査: `go vet ./...`・`gofmt -l .`・`go test -race ./...`、`cd web && pnpm fix && pnpm check && pnpm exec biome ci . && pnpm exec vitest run && pnpm build`、`make e2e` を3回（3回とも全件 PASS。落ちたら失敗の出力を残して直す）
- [x] Step 4: ドキュメントを直してコミット `docs(plan): 4e-4 の決めたことを概要 plan と spec に写し、引き継ぎを書く`

---

## 画面と範囲の確かめ（4e-4 の終わり）

Task 7 で、spec の「画面と範囲」の各行と E2E の一覧を、コードを見て一つずつ確かめた。`grep -rn "4e-4" web/app web/core web/e2e` は 0 件（「まだ出さない」の検査やコメントは残っていない）。足りないものはなかった。

| アートボード | 確かめたこと（場所） | 結果 |
|---|---|---|
| Sidebar | 保存済みビューの一覧と layout の切り替え（`sidebar/Sidebar.svelte`、`views/ViewPage.svelte`）、ワークスペースの行（Store のバッジ・接頭辞）› プロジェクトと進捗（`done/total`）、行の「…」の「設定を開く」「削除…」、`SyncCard` の状態と「今すぐ同期」、通知の未読数 | ある |
| Workspace | `WorkspaceHeader` のタブ（概要・ツリー・カンバン・テーブル・設定）、`WorkspaceOverview` の件数とプロジェクトの一覧（進捗・期限・担当）とプロジェクトの外のルートのタスク、タスク・プロジェクトの作成ボタン | ある |
| WorkspaceSettings | `WorkspaceSettings.svelte` の名前の編集、「接頭辞を変更…」（`PrefixChange`）、ULID のコピー、削除（残っていれば無効で理由） | ある |
| Main（ツリー） | `tree/TreeView.svelte`・`keyboard.ts`（追加・字下げ・並べ替え・完了、ドラッグ）、`FilterBar`、`list/SaveViewButton` | ある |
| Project | `ProjectHeader`（ワークスペース › プロジェクトと Store のバッジ、ワークスペースへ戻る）、進捗、タスク詳細の見出しのパンくずと Store のバッジ（`detail/TaskDetail.svelte`） | ある |
| Kanban | `kanban/KanbanView.svelte`（ステータス・カテゴリの列、担当者・プロジェクトのスイムレーン、ドラッグ、必須の項目の `RequiredDialog`、ビューを保存） | ある |
| Table | `table/TableView.svelte`・`TableCell.svelte`（セルの編集、並び替え、グループ、列の選択） | ある |
| MyTasks | `my/MyTasks.svelte`・`my/groups.ts`（期限・プロジェクトでまとめる） | ある |
| TaskDetail | `detail/` のプロパティ・`BodyEditor`・`Comments`・`Relations`（逆方向とバックリンク）・`Checklist`・ウォッチ・`History`（この時点に戻す）・`DeleteDialogs` | ある |
| Inbox | `inbox/InboxPage.svelte`（一覧、既読、自分の値に戻す） | ある |
| Activity | `activity/ActivityPage.svelte`・`WorkspacePicker.svelte`（人と同期、ワークスペースの複数選択） | ある |
| ProjectSettings | `project/ProjectSettings.svelte`・`ConstraintEditor.svelte`・`ProjectViolations.svelte`（表示するステータス、遷移の制約、必須の項目、Store の値と並べる、全部戻す） | ある |
| CreateTask / CreateProject | `create/CreateTask.svelte`・`CreateProject.svelte`・`WorkspaceChoice.svelte`（ワークスペースが2つ以上のときだけ選択欄、親があれば固定） | ある |
| CreateStore | `create/CreateStore.svelte` | ある |
| Dialogs | 削除の確認（`DeleteDialogs`）、ビューを保存・改名・削除（`views/`）、ワークスペースの追加・接頭辞・削除・移動（`workspace/`） | ある |
| CommandPalette | `palette/CommandPalette.svelte`・`search.ts`（ID・タイトルの検索、画面の移動、操作） | ある |
| States | `states/Empty.svelte`（空の状態）・`Disconnected.svelte`・`Banners.svelte`（同期のエラー、オフライン）・`NoToken.svelte`、壊れたファイルは `TaskDetail` の帯で `hh doctor` へ案内 | ある |
| Onboarding | `states/Onboarding.svelte`（Store が1つもないとき） | ある |
| SettingsSchema | `settings/SettingsSchema.svelte`・`SchemaCheck.svelte`・`MigrateDialog.svelte` | ある |
| SettingsGeneral | `settings/SettingsGeneral.svelte`（同期の状態と間隔）・`DoctorSection.svelte`・`PurgeSection.svelte` | ある |
| SettingsWorkspaces | `settings/SettingsWorkspaces.svelte`（一覧、追加、設定タブへ） | ある |
| SettingsNav | `settings/SettingsNav.svelte`・`SettingsLayout.svelte` | ある |

「後のフェーズ」の欄にあるもの（ガント・バックログ・スプリント・リリース・レポートのタブ、一括編集、PR・Issue、作業時間、自動化、アカウント・Store の設定など）と、フェーズ4で作らないアートボードは、spec のとおり出していない。

### E2E の一覧（spec の「4d・4e フロントエンド」）

| 項目 | `web/e2e` の spec |
|---|---|
| 作成 | `create.spec.ts` |
| 編集と自動保存 | `edit.spec.ts` |
| ツリーのドラッグ | `tree.spec.ts`（「dragging a row after another reorders; …」ほか） |
| カンバンの移動 | `kanban.spec.ts` |
| CLI の変更が SSE で反映 | `live.spec.ts` |
| 通知の既読と revert | `inbox.spec.ts` |
| 保存済みビュー | `views.spec.ts` |
| undo | `undo.spec.ts` |
| 削除と復元 | `delete.spec.ts` |

すべてある。このほかに、この回で `activity.spec.ts`・`project-settings.spec.ts`・`maintenance.spec.ts`・`schema.spec.ts` を足した。

## 決めたこと

実装中に plan が決めていなかったもの。概要 plan の細部 106〜129 に写してある。

- **通知**: 足元は `hh inbox --all`。既読にした上書きの行はボタンなし。同期の修復の文は detail の種類ごとに分ける。タブは `/inbox?tab=overwritten|mention|repair` でアドレスに持つ
- **アクティビティ**: すべての trailer を出す。タスクはあるが ID がないログは ULID を出す。`limit` の上限は 1000。日時を読めないコミットは「日時不明」にまとめる
- **プロジェクトの設定**: scoped を1件返す GET がないので `SchemaView.scoped` から読む。`PUT`・`DELETE` のあとに `SchemaCache.reload(store)`。すでにエントリのあるステータスへの追加はそのエントリに併合する。必須にできる項目は `internal/schema/check.go` から取る。`ConstraintEditor` と `ProjectViolations` に分ける
- **同期と保守**: `/settings` は `/settings/general` に置き換える。窓が隠れているあいだの fetch は `max(interval, 10 分)`。`SyncCard` は任意の `store` を受ける。ナビの Store の選択欄は作り直さず、選びを残す
- **doctor**: API は `DoctorReport[]`。修復のあとは応答の findings を使い、二度目の GET は送らない。ID は `GET /api/tasks/{ulid}` で引く。severity が info のものも扱う。E2E の harness は `Hedgehug.env()` で `hedgehug` を PATH に載せる（merge driver が見つかるように）
- **purge**: 日数はテキスト欄で、NFKC で全角を半角にして 0〜36500 を受ける
- **スキーマ**: `schema.changed` では check だけを取り直す。migrate のあとに行が消えたらフォーカスを h1 へ。定義が1つなら「併合する…」を出さない。ワークフローは文の一覧で見せる。check の欄は `SchemaCheck.svelte`。タブは `/settings/schema/<store>?tab=kinds|props|workflow`

## 組み上げで直した点

plan の誤りで、組み上げの途中で実物に合わせたもの。

- Task 3: plan の `GET /api/schema/scoped/{project}` は実在しない（scoped の API は `PUT`・`DELETE` だけ）。上書きは `GET /api/schema` の `SchemaView.scoped` から読むように直した（37419ae）。使わなくなる `cliScopeShow` も外した（fddc3ec）
- Task 5: doctor の応答の型は plan の `FindingsOutputBody` ではなく `DoctorReport[]`（Store ごとの配列）だった。それに合わせ、修復のあとも応答の findings を使う
- Task 1・3: plan が挙げた `cliInboxRead`・`cliInboxRevert`・`cliScopeShow` は呼ぶ画面がなく、使われないので作らずに外した（d174483、fddc3ec）
- Task 6: 概要 plan の細部 74 と plan の足元の「`hh migrate …`」は、実際のコマンドでは `hh schema migrate …`。足元の文字列を直し、`cliSchemaMigrate` の使われない `dryRun` も外した（f039062）
- Task 6: 通知とスキーマのタブはアドレスに持たず状態だけだったため、戻る・再読み込みで失われた。アドレスに持たせ、タブの一覧の出どころを1つにした（d5993e7、98dd30d）
