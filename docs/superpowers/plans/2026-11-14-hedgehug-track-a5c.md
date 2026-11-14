# トラック A5c(コマンドパレット) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ctrl+K(Cmd+K)で開くコマンドパレットを作る。アイテム・プロジェクト・ワークスペースを ID(旧 ID を含む)・タイトルで探して開き、コマンド・選んでいるアイテムへの操作・作成・設定への移動・最近開いたものを、仕様 02「コマンドパレット」の群・並び・上限どおりに出す。

**Architecture:** パレットを開いたときに、サーバーから「索引」(表示中のすべての Store の、削除されていないアイテムとプロジェクトの ID・タイトル・旧 ID)と「最近開いたもの」を 1 回ずつ読み、入力ごとの絞り込み・並び・上限は core の純粋な関数で決める(1 文字ごとにサーバーへ問い合わせない。応答の追い越しがなく、規則を Vitest で固められる)。旧 ID の一覧は転送記録と旧い接頭辞から `internal/domain/transfer` の純粋な関数で作る。最近開いたものは state.db に内部 ID で覚え、詳細パネルを開いた 1 か所(`detail.open`)で記録する。候補の実行は既存の口(作成ダイアログ・移動ダイアログ・詳細パネル・同期・画面の移動)を呼ぶだけにする。

**Tech Stack:** Go(huma・domain/app/infra/adapter の層、SQLite の statedb)、TypeScript(core・api の orval)、Svelte 5(ui・app)、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(「コマンドパレット」の全節、「別のワークスペースと別の Store への移動」の「旧 ID から辿れる場所」「移した日時の表示」、記録済みの決定の表の「パレットの…」の 5 行)、`01-overview.md`(キーボードの表の Ctrl+K・Esc、「新しいタブで開く」、「マシンごとに持つもの」の最近開いたアイテム、画面の移り方)、`03-workspace-settings.md`(115 行の旧い接頭辞、543 行の使わないステータス、500 行・654 行の自動化のきっかけ)、`06-sync-notifications.md`(23 行の「すべての Store を同期」、60・169 行の反映を待つとき)、`2026-11-12-hedgehug-storage-design.md`(282 行: 最近開いたアイテムは state.db)。

**実装担当にタスクを渡すときは、タスクの本文と一緒に「Global Constraints」「持ち場と境目」「決めたこと」「タスク間の名前」の節を必ず渡す。** 食い違ったときは、この 4 節と仕様が本文より優先する。

## Global Constraints

- 依存は追加しない(足すときは `go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` で版を明記し、公開から 7 日以上の版だけ。Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル(テストを除く)、Svelte 200 行、TS 300 行。いまの数: `internal/app/port` 20・`internal/app/apptest` 20(どちらも新しいファイルを足さない)、`internal/infra/gitstore` 19、`internal/app/transfers` 17、`internal/infra/statedb` 7、`internal/domain/transfer` 9。`web/app/src/wire.ts` は 295 行なので、パレットの組み立ては別のファイル(`web/app/src/palette.ts`)に置く。
- 向き: adapter→app→domain、infra→app/port。app の機能どうしは import しない(テストも)。新しい `internal/app/palette` は `app/transfers` を import しない(全 Store を読む小さな処理は写す)。
- 画面の文言は仕様の文字どおり。仕様にない文言は「決めたこと」の A5c-14 の文を使い、Task 11 で仕様に書き足す。モックの動きは写さない。
- パレットを開く・閉じる・候補を選ぶことは Store を変えない(保存にもアクティビティにも残さない)。最近開いたものは state.db だけに書く。
- 各タスクは TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクごとにレビューする。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR はトラックで 1 本(develop 向け、ブランチ `feat/track-a5c`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。push は HTTPS(`https://github.com/kukv/hedgehug.git`)。
- 計画やテストにトークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e のビルドも CI で lint する。
- 確認のコマンド: Go は `go test ./internal/... ./cmd/...`、全体は `make check`、画面は `pnpm -C web test`・`pnpm -C web check`、API の型は `make generate` のあと `pnpm -C web gen`、e2e は `pnpm -C web e2e`。手元の検証は 1 回で足りる。
- `api/openapi.yaml`・`web/packages/api/src/generated.ts` は手でマージしない。API を変えたタスクで作り直してコミットする。
- 本文のテストの名前と期待は下書きで、実行していない。仕様と「決めたこと」が正。

## Review Focus

1. **日本語入力の変換中の Enter・Esc**: 変換を確定する Enter(`event.isComposing` または `keyCode === 229`)で候補を実行しない。変換中の Esc でパレットを閉じない(01「Esc」)。テストは Task 9(部品のテスト)。
2. **入力欄にフォーカスがあっても Ctrl+K で開く。ダイアログを開いているときは開かない**: C キー・/ キーと違い、入力中でも開く(01 の表は入力中を除いていない)。`[aria-modal="true"]` があれば開かない。パレット自身も `aria-modal="true"` なので、パレットを開いているあいだは C キーと反映(`tab.ts` の `holdReasons`)が止まる。テストは Task 9。
3. **画面を移る・戻る・進むでパレットを閉じる**: `navigation.route` が変わったら閉じる(01 174 行)。候補が画面を移すときも、移す前に閉じる。テストは Task 8・9。
4. **旧い接頭辞をあとから別のワークスペースが使っているとき**: 「WEB-3」で、いまの WEB の 3 番と、旧い接頭辞 WEB の 3 番(いまは MOB-3)の両方が出て、後者に「旧 ID」と添える。テストは Task 1(`Aliases`)と Task 6(`candidates`)。
5. **最近開いたものの消え方**: 削除・非表示の Store では出さないが覚えておき、復元・再表示で戻る。完全に削除されたもの・このマシンから消えた Store のものは、覚えている一覧から消す。別の Store へ移したものは、いまの ID で出る。テストは Task 4。

---

## 持ち場と境目

| 入れるもの | 仕様 |
|---|---|
| パレットの枠(Ctrl+K、プレースホルダ、下端の案内、↑↓・Enter・Ctrl+Enter・Esc・背景のクリック、見出し、上限、「ほか N 件あります…」、「一致するものはありません」) | 02「コマンドパレット」の 1 段落目と操作の表、01 キーボードの表 |
| コマンドの群のうち、行き先がいまあるもの: 新規作成、設定を開く、すべての Store を同期、ワークスペースを追加 | 02 候補の表の「コマンド」、06 23 行 |
| 選んでいるアイテムの群(ステータスを変更…、{種類} を追加、詳細を開く、別のワークスペースへ移動…)。選んでいるアイテムは、詳細パネルとツリーのフォーカス | 02 次の群の表の「選んでいるアイテム」、実行の表 |
| 作成の群(プロジェクトを作成、スプリントを作成、リリースを作成) | 02「作成」、F1 の計画の F1-23 |
| 設定の群のうち、行き先がいまあるもの: 設定: アカウントとメンバー、設定: ワークスペース({名前})、設定: プロジェクト({名前}) | 02「設定」 |
| 最近開いたもの(state.db、記録と読み出し、消し方) | 02「最近開いたもの」、01 マシンごとに持つもの、保存形式 282 行 |
| アイテム・プロジェクト・ワークスペースの群(旧 ID・旧い接頭辞を含む) | 02 候補の表、285 行、03 115 行 |

| 入れないもの | 回す先 | 理由 |
|---|---|---|
| 「このクエリで」の群、ビューの群、コマンド「検索を開く」「マイタスクを開く」、選んでいるアイテムのテーブルの選択 | E | A5 を 3 本に分けた決定(2026-11-14)で「このクエリで」は E に回した。同じ規則で、行き先の画面(検索・ビュー・マイタスク・テーブル)を作るトラックが群・候補・選択の口を足す。A5c は群の順と上限の枠を作り、空の群は出さない |
| 選んでいるアイテムのバックログの選択 | F2 | バックログは F2 |
| コマンド「受信箱を開く」「アクティビティを開く」 | C4 | 画面は C4 |
| 設定: Store、設定: 削除済み、設定: 診断と修復、ワークスペースを追加の Store の初期値の既定の Store(08) | G | 設定のタブと既定の Store は G。G が入るまでは、ワークスペースの追加ダイアログのいまの初期値(書き込める先頭の Store。B2-18)のまま |

渡す先の計画への書き足しは Task 11 で行う(トラック C の計画の C4 の行、トラック A の計画の A5 の行、A5c の計画の末尾「他のトラックに渡すもの」)。

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| A5c-1 | 1 本の計画・1 本の PR(サーバーと画面を分けない) | トラックごとに 1 本(CLAUDE.md) |
| A5c-2 | アイテムとプロジェクトの候補は、パレットを開いたときに `GET /api/palette-index` を 1 回読み、core で絞る。読み終わる前の入力は、読み終わったときに絞り直す。読めなかったときは、アイテム・プロジェクトの群を出さずに他の群だけ出す(トーストは出さない) | 応答の追い越しをなくし、並びと上限を純粋な関数にするため |
| A5c-3 | 索引に入れるのは、表示中の Store(`StoreList.List`。閲覧のみを含む)の、それ自身・プロジェクト・ワークスペースのどれも削除されていないアイテムと、それ自身・ワークスペースが削除されていないプロジェクト。各行の旧 ID は `transfer.Aliases`(A5c-4) | 02 候補の表の出る条件 |
| A5c-4 | 旧 ID の一覧は `transfer.Aliases(records []Record, entity string, target change.ID, oldPrefixes []string, number int, sep string) []Alias`。`Alias{ID string; At *time.Time}`。(a) 全 Store の転送記録のうち `Entity == entity` かつ `Target == target` の行の `Old`(At は記録の At)、(b) いまのワークスペースの旧い接頭辞ごとに `{旧い接頭辞}{sep}{number}`(At は nil)。重複は除く(At の古い方を残す)。いまの ID と同じ文字列は呼ぶ側(Task 3)が除く。並びは (a) を At の古い順 → (b) を旧い接頭辞の並び。別の Store へ移しても内部 ID は変わらない(`ItemWriter.Import` は id を変えない)ので、Target で全 Store の記録を集められる | 02 283・285 行、03 115 行 |
| A5c-5 | 一致: 入力を前後の空白を除いて小文字にし、空なら一致を調べない。アイテムは、いまの ID か旧 ID のどれかを小文字にしたものが入力を含む → 「ID の一致」、そうでなくタイトルを小文字にしたものが入力を含む → 「タイトルの一致」。プロジェクトも同じ(ID・旧 ID → タイトル)。ワークスペースは名前か接頭辞が含む。コマンド・作成・設定・選んでいるアイテムの操作は名前が含む(大文字小文字を区別しない) | 02 候補の表(「含む」で比べる。並びの「一致する」は ID 側で含むものを指すと読んだ。タイトルだけのものを後にする規則と両立する) |
| A5c-6 | 旧 ID で一致したアイテムのヒントは「{プロジェクト名}・旧 ID・{日付} に移動」、旧い接頭辞で一致したものは「{プロジェクト名}・旧 ID」。プロジェクトは「旧 ID・{日付} に移動」・「旧 ID」(ワークスペース名の代わり)。いまの ID でも一致したら旧 ID を添えない。複数の旧 ID が一致したら At の新しいもの。{日付} は `formatDay(dayOf(at), 今年)` | 02 285 行(候補の表の「旧 ID」より詳しい方に合わせる) |
| A5c-7 | 群の順・上限は 02 のとおり: コマンド(最大 8)、選んでいるアイテム(最大 4)、作成(最大 3)、設定(最大 6)、最近開いたもの(最大 8)、アイテム(40 −(コマンド+プロジェクト+ワークスペース+ビューの出した件数)、最低 17)、プロジェクト・ワークスペース・ビュー(各 5)。「このクエリで」とビューは E が足す枠だけ(A5c では常に空)。上限で切った群の末尾に押せない 1 行「ほか N 件あります。入力を続けて絞り込んでください」(N は切った件数)。どの群にも候補がないとき「一致するものはありません」。選択できるのは押せる行だけ | 02 候補の段落 |
| A5c-8 | 並び: アイテムは ID の一致 → タイトルの一致、それぞれ Store の並び → ワークスペースの並び → 番号の昇順(並びは core の `catalog.stores` の順。索引に Store・ワークスペースの ID と番号を持たせる)。プロジェクトは ID の一致 → タイトルの一致、それぞれサイドバーの並び(`catalog.stores` のプロジェクトの順)。ワークスペースはサイドバーの並び | 02 候補の段落 |
| A5c-9 | 選んでいるアイテムは core の `createSelection()` に、出どころが `provide(rank, get)` で登録する。パレットを開いた時点に `rank` の小さい順に `get()` を呼び、最初の null でないもの。詳細パネル `rank 0`(`detail.target()`)、ツリーのフォーカス `rank 1`(`ProjectTree.svelte` が出ているあいだ登録し、フォーカスのある行のアイテム)。テーブル・バックログは E・F2 が `rank 2` で足す。見出しの「同じ ID のアイテムが別の Store にもある」は索引で判定し、Store の色の点を頭に付ける | 02 選んでいるアイテムの表 |
| A5c-10 | 選んでいるアイテムの操作: 閲覧のみの Store なら「詳細を開く」だけ。詳細パネルがそのアイテムを開いているなら「詳細を開く」を出さない。「{種類} を追加」は `childKindOf(kind)` が null でないとき、`itemDialog.open({ workspaceId, projectId, kind: 子の種類, parentId })`。「別のワークスペースへ移動…」は Epic・Story・Task・Bug だけで、`transferDialog.open(item)`。「ステータスを変更…」はパレットの中の一覧に切り替え、項目は `statusOptions(settings, item.status, true, item.projectId)`(settings はアイテムのワークスペースの設定を `workspaces.get` で読む)。選ぶと、詳細パネルの出どころなら `detail.setProperty(itemId, "status", id)`、ツリーなら `projectScreen.setStatus(itemId, id)`(メニューと同じ経路。自動化のきっかけはサーバーが同じに扱う)を呼んでパレットを閉じる。無効の項目は選べず、理由をヒントに出す | 02 実行の表、03 543・654 行 |
| A5c-11 | 作成の群: 「プロジェクトを作成」は `projectDialog.open(here の workspaceId)`。書き込めるワークスペースがないときは、`startValues` と同じ 2 つの文で断る(A5c-14)。「スプリントを作成」「リリースを作成」は、ワークスペースかプロジェクトの画面のときだけ出し、そのワークスペースの `sprints`・`releases` のタブへ `navigation.go` してから、core の `createPlanRequest()` に `{ workspaceId, kind: "sprint" \| "release" }` を置く。タブの部品は、出ているあいだ要求を見て、自分のワークスペースのものなら取り出して作成ダイアログを開く(すでにそのタブを出しているときも開く)。閲覧のみの Store のワークスペースでは、タブを開いてダイアログは開かず、トースト「閲覧のみの Store です」 | 02「作成」、F1-23 |
| A5c-12 | Ctrl+Enter(Cmd+Enter)で新しいタブに開くもの(`links.openTab(route)`): アイテム・最近開いたもの(`/s/{Store 名}/i/{いまの ID}`)、プロジェクト(概要)、ワークスペース(概要)、設定を開く(`/settings/accounts`)、設定: …(各タブ)。それ以外は Enter と同じ | 02 操作の表、01 186 行 |
| A5c-13 | 最近開いたもの: state.db の表 `recent_items(item TEXT PRIMARY KEY, store TEXT NOT NULL, at TEXT NOT NULL)`。`detail.open` が開けたあと `palette.opened(storeId, itemId)` を呼ぶ(行・パレット・リンク・旧 ID のアドレスのどれでも通る。失敗は無視)。覚えるのは新しい 50 件(超えた古いものを消す)。読み出しは新しい順に、内部 ID を表示中の Store の削除されていないアイテムから探し(Store は覚えた Store を先に、なければほかの表示中の Store。別の Store への移動でも内部 ID は同じ)、見つかったものを 8 件まで返す。見つからないもののうち、`SyncTargets.List`(非表示を含む、このマシンの Store)のどの Store の `Purged()` にあるか、どの Store にもないものは表から消す。削除済み・非表示の Store のものは消さずに飛ばす | 02「最近開いたもの」、保存形式 282 行 |
| A5c-14 | 仕様にない文言: プロジェクトの作成で書き込めるワークスペースがないとき「先にワークスペースを作ってください」、閲覧のみの Store しかないとき「プロジェクトを作成できる Store がありません(閲覧のみの Store だけです)」(アイテムの作成の文の名詞を替えた)、閲覧のみのワークスペースでスプリント・リリースを作成したとき「閲覧のみの Store です」(既存の `readOnlyNote`)、パレットの入力欄の名前(aria-label)「コマンドパレット」、ステータスの一覧の見出し「{ID} のステータス」 | Task 11 で 02 に書き足す |
| A5c-15 | 選んでいるアイテムの見出しは「{ID} に対して」。ステータスの一覧に切り替えたあとも入力欄は残し、入力で項目を名前で絞る。Esc はパレットごと閉じる | 02 実行の表 |
| A5c-16 | パレットを開いているあいだ、画面が変わったら(`navigation.route` の購読)パレットを閉じる。候補がダイアログを開くとき・画面を移すときは、先にパレットを閉じてから呼ぶ。書きかけがあるときの「入力を破棄」は、呼ばれた側(`navigation.go`・`detail.open`)の既存の確認がそのまま出る | 02 1 段落目、01 174 行 |
| A5c-17 | API: `GET /api/palette-index` → `{ items: [{storeId, workspaceId, projectId, itemId, number, display, title, projectTitle, aliases: [{id, at}]}], projects: [{storeId, workspaceId, projectId, display, title, workspaceName, aliases}] }`(`at` は `change.TimeLayout` の文字列か null。並びは問わない)。`GET /api/recent-items` → `[{storeId, itemId, display, title, projectTitle}]`。`POST /api/recent-items`(本文 `{storeId, itemId}` → 204)。OperationID は `palette-index`・`list-recent-items`・`open-recent-item` | — |
| A5c-18 | サーバーの置き場所: `internal/app/palette`(新規。`Service{Stores, StoreList, SyncTargets, Recent port.RecentItems, Clock}`)、口 `port.RecentItems` は `internal/app/port/syncstate.go` に足す(port は 20 ファイルで上限)、実装は `internal/infra/statedb/recent.go`、API は `internal/adapter/httpapi/palette/`。e2e の組み立ては、`e2eReads` と同じ本物の statedb の `Recent()` を渡す(apptest に偽物を足さない) | Global Constraints の大きさ |

---

## ファイルの地図

| 場所 | 新規・変更 | 役割 |
|---|---|---|
| `internal/domain/transfer/aliases.go` | 新規 | `Alias`・`Aliases` |
| `internal/app/port/syncstate.go` | 変更 | `RecentItems` の口 |
| `internal/infra/statedb/recent.go` | 新規 | 表と `DB.Recent()` |
| `internal/app/palette/service.go`・`read.go`・`index.go`・`recent.go` | 新規 | 全 Store の読み取り・索引・最近開いたもの |
| `internal/adapter/httpapi/palette/palette.go`・`internal/adapter/httpapi/routes.go` | 新規・変更 | API |
| `cmd/hedgehug/wire.go`・`wire_e2e.go` | 変更 | 組み立て |
| `web/packages/api/src/palette.ts`・`index.ts` | 新規・変更 | `createPalettePort` |
| `web/packages/core/src/model/palette.ts`・`port/palette.ts` | 新規 | 型と口 |
| `web/packages/core/src/paletteIndex.ts` | 新規 | core のパレットの公開(既存の `notesIndex` などと同じ形) |
| `web/packages/core/src/features/palette/match.ts`・`candidates.ts`・`selection.ts`・`palette.ts`・`actions.ts`・`planRequest.ts` | 新規 | 一致と並び・群と上限・選んでいるアイテム・状態・実行・スプリント/リリースの作成の要求 |
| `web/packages/core/src/features/items/detail.ts` | 変更 | `DetailDeps.onOpened` |
| `web/app/src/palette.ts`・`web/app/src/palette/CommandPalette.svelte`・`PaletteRow.svelte` | 新規 | 組み立てと部品 |
| `web/app/src/App.svelte`・`wire.ts`(1 行)・`screens/project/ProjectTree.svelte`・`screens/planning/SprintsTab.svelte`・`ReleasesTab.svelte` | 変更 | Ctrl+K、記録、選択の登録、作成の要求 |
| `web/e2e/palette.spec.ts` | 新規 | 通しのテスト |

## タスク間の名前

- Go: `transfer.Alias{ID string; At *time.Time}`、`transfer.Aliases(records []transfer.Record, entity string, target change.ID, oldPrefixes []string, number int, sep string) []transfer.Alias`。
- Go: `port.RecentItems{ Touch(ctx, store port.StoreID, item change.ID, at time.Time, keep int) error; List(ctx) ([]port.Recent, error); Forget(ctx, items []change.ID) error }`、`port.Recent{Store port.StoreID; Item change.ID; At time.Time}`(List は At の新しい順)。
- Go: `palette.Service.Index(ctx) (palette.Index, error)`、`Opened(ctx, store port.StoreID, item change.ID) error`、`Recent(ctx) ([]palette.RecentItem, error)`。`palette.Index{Items []IndexItem; Projects []IndexProject}`。
- TS core: `PaletteIndex`・`PaletteItem`・`PaletteProject`・`PaletteAlias{ id: string; at: string | null }`・`RecentItem`(A5c-17 の形)、`PalettePort{ index(): Promise<PaletteIndex>; recent(): Promise<RecentItem[]>; opened(storeId: string, itemId: string): Promise<void> }`。
- TS core: `Candidate`(`{ key: string; group: GroupId; label: string; hint?: string; disabled?: boolean; reason?: string; route?: Route; run: () => void | Promise<void> }`)、`Row = { kind: "head"; text: string } | { kind: "more"; text: string } | { kind: "candidate"; c: Candidate }`、`GroupId = "command" | "query" | "selected" | "create" | "settings" | "recent" | "item" | "project" | "workspace" | "view"`。
- TS core: `buildRows(groups: Partial<Record<GroupId, Candidate[]>>, input: string, head: Partial<Record<GroupId, string>> = {}): Row[]`(上限・見出し・「ほか N 件」・「一致するものはありません」。選んでいるアイテムの見出し「{ID} に対して」は `head.selected` で渡す。「一致するものはありません」は `kind: "more"` の行)。
- TS core: `createSelection(): Selection`、`Selection{ provide(rank: number, get: () => SelectedItem | null): () => void; current(): SelectedItem | null }`、`SelectedItem{ storeId: string; itemId: string; source: "detail" | "tree" | string; setStatus(statusId: string): Promise<void> }`。
- TS core: `createPlanRequest(): PlanRequest`、`PlanRequest{ request: Readable<{ workspaceId: string; kind: "sprint" | "release" } | null>; ask(r): void; take(workspaceId: string, kind): boolean }`。
- TS core: `createPalette(deps: PaletteDeps): Palette`、`Palette{ state: Readable<PaletteState | null>; open(): Promise<void>; close(): void; input(text: string): void; move(dir: "up" | "down"): void; choose(index?: number, newTab?: boolean): Promise<void> }`。

---

### Task 1: `domain/transfer`: `Aliases`

**Files:**
- Create: `internal/domain/transfer/aliases.go`
- Test: `internal/domain/transfer/aliases_test.go`

**Interfaces:**
- Consumes: `transfer.Record`(record.go)、`change.ID`。
- Produces: `transfer.Alias`・`transfer.Aliases`(タスク間の名前)。

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestAliasesCollectsMovesOldestFirstThenOldPrefixes(t *testing.T) {
	at1 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	at2 := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	recs := []transfer.Record{
		{Entity: "item", Target: "x1", Old: "MB-4", New: "WEB-7", At: at2},
		{Entity: "item", Target: "x1", Old: "HH-12", New: "MB-4", At: at1},
		{Entity: "project", Target: "x1", Old: "HH-P1", At: at1},
		{Entity: "item", Target: "x2", Old: "HH-3", At: at1},
	}
	got := transfer.Aliases(recs, "item", "x1", []string{"W"}, 7, "-")
	want := []transfer.Alias{{ID: "HH-12", At: &at1}, {ID: "MB-4", At: &at2}, {ID: "W-7"}}
	if diff := cmp.Diff(want, got); diff != "" { t.Fatal(diff) }
}

func TestAliasesDropsDuplicates(t *testing.T) // 同じ Old の記録が 2 行 → 1 つ(At の古い方)
func TestAliasesProjectUsesPSeparator(t *testing.T) // sep "-P"、旧い接頭辞 "HH" と番号 1 → "HH-P1"
```

`workspace.OldPrefixes` はいまの接頭辞を含まない。いまの ID と同じ別名(旧い接頭辞をあとで戻したときなど)は Task 3 が除く。

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/domain/transfer/ -run Aliases` が「undefined: transfer.Aliases」で落ちる。
- [ ] **Step 3: `Aliases` を実装する**(A5c-4)。
- [ ] **Step 4: 通す** — 同じコマンドが PASS。
- [ ] **Step 5: コミット** — `feat: アイテム・プロジェクトの旧 ID の一覧を転送記録と旧い接頭辞から作る`

### Task 2: state.db の最近開いたもの

**Files:**
- Modify: `internal/app/port/syncstate.go`(`RecentItems`・`Recent` を足す)
- Create: `internal/infra/statedb/recent.go`
- Test: `internal/infra/statedb/recent_test.go`

**Interfaces:**
- Produces: `port.RecentItems`・`port.Recent`(タスク間の名前)、`func (d *DB) Recent() port.RecentItems`。手順の名前は `palette/001`。

- [ ] **Step 1: 失敗するテストを書く**(既存の `checks_test.go` と同じく一時ファイルに `Open`)

```go
func TestRecentTouchKeepsNewestAndMovesToTop(t *testing.T) {
	// a・b・c を順に Touch(keep 2)→ List は c, b。b を再び Touch → b, c。At は TEXT で往復しても同じ。
}
func TestRecentTouchUpdatesStore(t *testing.T) // 同じ item を別の Store で Touch → Store が替わる(1 行のまま)
func TestRecentForgetRemovesRows(t *testing.T)
```

- [ ] **Step 2: 失敗を確かめる** — `go test ./internal/infra/statedb/ -run Recent` が落ちる。
- [ ] **Step 3: 実装する** — `register("palette/001", "CREATE TABLE recent_items(item TEXT PRIMARY KEY, store TEXT NOT NULL, at TEXT NOT NULL)")`。`Touch` は upsert のあと、`at` の新しい順で `keep` 件を超えた行を消す(1 トランザクション)。時刻は `timeText`・`parseTime`。
- [ ] **Step 4: 通す**。
- [ ] **Step 5: コミット** — `feat: 最近開いたアイテムを state.db に覚える`

### Task 3: `app/palette`: 索引

**Files:**
- Create: `internal/app/palette/service.go`・`read.go`・`index.go`
- Test: `internal/app/palette/index_test.go`(`apptest.NewStores`・`apptest.StoreList` で組む)

**Interfaces:**
- Consumes: `port.Stores.Read`・`port.StoreList.List`・`port.LiveItem`・`port.LiveProject`・`port.LiveWorkspace`・`workspace.OldPrefixes`・`Reader.Transfers().List()`・`item.Item.DisplayID`、Task 1 の `Aliases`。
- Produces: `palette.Service`(A5c-18 のフィールド)、`Service.Index(ctx) (Index, error)`。`IndexItem{Store port.StoreID; Workspace workspace.ID; Project project.ID; Item item.ID; Number int; Display, Title, ProjectTitle string; Aliases []transfer.Alias}`、`IndexProject{Store; Workspace; Project; Display, Title, WorkspaceName string; Aliases}`。

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestIndexListsLiveItemsAndProjectsOfShownStores(t *testing.T)
// 表示中の 2 つの Store(1 つは閲覧のみ)のアイテム・プロジェクトが入る。削除済みのアイテム、削除済みのプロジェクトの下のアイテム、
// 削除済みのワークスペースのプロジェクト、StoreList に出ない Store のものは入らない。Display は "HH-12"・"HH-P1"、ProjectTitle が入る。
func TestIndexAliasesFollowTransfersAcrossStores(t *testing.T)
// Store A の HH-12 を Store B の MB-4 へ移した記録(Store A の転送記録、Target はアイテムの内部 ID)→ B のアイテムの Aliases に HH-12(At つき)
func TestIndexAliasesIncludeOldPrefixes(t *testing.T)
// ワークスペースの接頭辞を WEB → W に変えた → W-3 の Aliases に WEB-3(At なし)。いまの ID と同じものは入らない
```

- [ ] **Step 2: 失敗を確かめる**。
- [ ] **Step 3: 実装する** — `read.go` に表示中の Store を 1 つずつ `Read` する処理(`transfers/locate_read.go` の `snapshots` を写す。転送記録は全 Store の分を集めてから `Aliases` に渡す)。プロジェクトの旧 ID は entity `"project"`・sep `"-P"`、アイテムは `"item"`・`"-"`。`Aliases` の結果から、いまの `Display` と同じものを除く。
- [ ] **Step 4: 通す**。
- [ ] **Step 5: コミット** — `feat: パレットの索引(アイテム・プロジェクトと旧 ID)を全 Store から読む`

### Task 4: `app/palette`: 最近開いたもの

**Files:**
- Create: `internal/app/palette/recent.go`
- Test: `internal/app/palette/recent_test.go`(`RecentItems` の偽物はテストファイルの中に置く。apptest に足さない)

**Interfaces:**
- Consumes: `port.RecentItems`(Task 2)、`port.SyncTargets.List`、`Reader.Purged().List()`、Task 3 の `read.go`。
- Produces: `Service.Opened(ctx, store port.StoreID, item change.ID) error`(`Touch(…, s.Clock.Now(), 50)`)、`Service.Recent(ctx) ([]RecentItem, error)`、`RecentItem{Store port.StoreID; Item item.ID; Display, Title, ProjectTitle string}`。

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestRecentReturnsNewestEightLiveItems(t *testing.T)        // 10 件覚え、新しい 8 件をいまの ID で返す
func TestRecentSkipsDeletedAndHiddenButKeepsThem(t *testing.T)  // 削除済み・非表示の Store(SyncTargets にあり StoreList にない)のものは返さず、Forget もしない
func TestRecentForgetsPurgedAndRemovedStores(t *testing.T)      // Purged にある・どの SyncTargets の Store にもない → Forget に渡る
func TestRecentFindsItemMovedToAnotherStore(t *testing.T)       // 覚えた Store から別の Store へ移した(内部 ID は同じ)→ 移動先のいまの ID で返す
```

- [ ] **Step 2: 失敗を確かめる**。
- [ ] **Step 3: 実装する**(A5c-13)。
- [ ] **Step 4: 通す**。
- [ ] **Step 5: コミット** — `feat: 最近開いたアイテムを記録し、いまの ID で返す`

### Task 5: API・組み立て・api パッケージの口

**Files:**
- Create: `internal/adapter/httpapi/palette/palette.go`(+ `palette_test.go`)、`web/packages/api/src/palette.ts`(+ `palette.test.ts`)、`web/packages/core/src/model/palette.ts`、`web/packages/core/src/port/palette.ts`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps.Recent port.RecentItems`、`palette.Register`)、`cmd/hedgehug/wire.go`(`db.Recent()`)、`cmd/hedgehug/wire_e2e.go`(`e2eReads` の DB の `Recent()`)、`web/packages/api/src/index.ts`、core の index(型と口の公開)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(生成)

**Interfaces:**
- Consumes: Task 3・4 の `palette.Service`。
- Produces: A5c-17 の 3 つの API、`createPalettePort(): PalettePort`(タスク間の名前)。

- [ ] **Step 1: 失敗するテストを書く** — Go: `TestPaletteIndexReturnsItemsWithAliases`(`humatest` で `GET /api/palette-index`。`at` が `change.TimeLayout` か null)、`TestRecentItemsRoundTrip`(POST → GET で 1 件)。TS: `palette.test.ts` で fetch の偽物に対し `index()`・`recent()`・`opened()` が正しいパスと本文を使う(既存の `transfers.test.ts` の形)。
- [ ] **Step 2: 失敗を確かめる**。
- [ ] **Step 3: 実装する** — `make generate` → `pnpm -C web gen`。
- [ ] **Step 4: 通す** — `go test ./internal/adapter/... ./cmd/...`、`pnpm -C web test --filter api`、`make e2e-bin` が通る。
- [ ] **Step 5: コミット** — `feat: パレットの索引と最近開いたものの API`

### Task 6: core: 一致・並び・群と上限

**Files:**
- Create: `web/packages/core/src/features/palette/match.ts`・`candidates.ts`
- Test: `match.test.ts`・`candidates.test.ts`

**Interfaces:**
- Consumes: `PaletteIndex`、`StoreNode[]`(並び)、`formatDay`・`dayOf`、`routeOf…`(既存のアドレスの関数)。
- Produces: `matchItems(index: PaletteIndex, stores: StoreNode[], input: string, now: Date): { c: PaletteItem; hint: string }[]`(並び済み)、`matchProjects(…)`、`matchWorkspaces(stores, input)`、`byName<T extends { label: string }>(list: T[], input: string): T[]`、`buildRows`(タスク間の名前)。

- [ ] **Step 1: 失敗するテストを書く**

```ts
test("ID の一致がタイトルの一致より先で、それぞれ Store・ワークスペースの並び・番号の順", ...);
test("旧 ID で一致したら「{プロジェクト名}・旧 ID・{日付} に移動」、旧い接頭辞なら「{プロジェクト名}・旧 ID」", ...);
test("WEB-3 で、いまの WEB-3 と旧い接頭辞 WEB の 3 番(いまの MOB-3)の両方が出る", ...); // Review Focus 4
test("ID は小文字で比べる: 「hh-1」で HH-1・HH-12 が出る", ...);
test("アイテムの上限は 40 − 他の群の件数、最低 17。切ったら「ほか N 件あります。入力を続けて絞り込んでください」", ...);
test("プロジェクト・ワークスペースは 5 件まで。コマンドは 8 件まで", ...);
test("群の順はコマンド・選んでいるアイテム・作成・設定・最近開いたもの・アイテム・プロジェクト・ワークスペース", ...);
test("何も一致しないとき「一致するものはありません」の 1 行", ...);
test("入力が空のときは、アイテム・プロジェクト・ワークスペース・作成・設定を出さない", ...);
```

- [ ] **Step 2: 失敗を確かめる** — `pnpm -C web test -- palette`。
- [ ] **Step 3: 実装する**(A5c-5〜A5c-8)。
- [ ] **Step 4: 通す**。
- [ ] **Step 5: コミット** — `feat: パレットの候補の一致・並び・上限`

### Task 7: core: 選んでいるアイテム・作成の要求・詳細パネルの記録

**Files:**
- Create: `web/packages/core/src/features/palette/selection.ts`・`planRequest.ts`(+ テスト)
- Modify: `web/packages/core/src/features/items/detail.ts`(`DetailDeps.onOpened?(storeId: string, itemId: string): void` を、開けたあとに呼ぶ)、`detail` のテスト

**Interfaces:**
- Produces: `createSelection`・`Selection`・`SelectedItem`・`createPlanRequest`・`PlanRequest`(タスク間の名前)。

- [ ] **Step 1: 失敗するテストを書く** — `selection`: 詳細(rank 0)とツリー(rank 1)の両方があれば詳細、詳細が null ならツリー、登録を外す関数で外れる。`planRequest`: `ask` のあと、別のワークスペースの `take` は false で残り、同じワークスペース・種類の `take` は true で消える。`detail`: `open` が成功したら `onOpened(store, item)` が 1 回、見つからないときは呼ばない。
- [ ] **Step 2〜4: 失敗を確かめ、実装し、通す**。
- [ ] **Step 5: コミット** — `feat: パレットの選んでいるアイテムと、スプリント・リリースの作成の要求`

### Task 8: core: パレットの状態と実行

**Files:**
- Create: `web/packages/core/src/features/palette/palette.ts`・`actions.ts`(+ `palette.test.ts`・`actions.test.ts`)

**Interfaces:**
- Consumes: Task 6・7、`PalettePort`、`Catalog`、`Navigation`、`ItemDialog`・`ProjectDialog`・`WorkspaceDialog`・`TransferDialog`(既存の `open`)、`Detail`、`Sync.syncAll`、`WorkspacesPort.get`、`statusOptions`、`childKindOf`、`Toasts`、`Links.openTab` の代わりの `openTab(route: Route): void`、`here(): Here`。
- Produces: `createPalette(deps: PaletteDeps): Palette`。`PaletteState{ input: string; rows: Row[]; selected: number; mode: "main" | { status: SelectedItem }; footer: string }`。`footer` は `paletteFooter(mac: boolean)` で「↑↓ で選択 ・ Enter で開く ・ Ctrl+Enter で新しいタブ ・ Esc で閉じる」(Mac は「Cmd+Enter」)。

- [ ] **Step 1: 失敗するテストを書く**

```ts
test("開くと索引と最近開いたものを読み、入力が空なら コマンド・選んでいるアイテム・最近開いたもの を出す", ...);
test("1 文字ごとに候補を作り直し、選択を先頭の押せる行に戻す。↑↓ は端で止まり、見出しと「ほか N 件」を飛ばす", ...);
test("索引を読む前の入力は、読んだあとに絞り直す。読めなかったらアイテム・プロジェクトを出さない", ...);
test("ダイアログを開く候補は、先に close してから open を呼ぶ", ...);
test("Ctrl+Enter: アイテムは openTab(/s/{Store名}/i/{ID})、新規作成は Enter と同じ", ...);
test("閲覧のみの Store のアイテムでは、選んでいるアイテムの群は「詳細を開く」だけ", ...);
test("詳細パネルで開いているアイテムには「詳細を開く」を出さない", ...);
test("Step には「{種類} を追加」「別のワークスペースへ移動…」を出さない。Epic は「Story を追加」", ...);
test("ステータスを変更…: 一覧に切り替え、遷移で移れない項目は選べず理由をヒントに出す。選ぶと setStatus を呼んで閉じる", ...);
test("スプリントを作成: ワークスペース・プロジェクトの画面だけで出し、sprints のタブへ go してから ask する", ...);
test("スプリントを作成: 閲覧のみの Store では、タブへ移り「閲覧のみの Store です」", ...);
test("プロジェクトを作成: 書き込めるワークスペースがなければ A5c-14 の文", ...);
test("すべての Store を同期: sync.syncAll を呼ぶ(remote のない場合の文はサーバーが返す)", ...);
test("設定: ワークスペース({名前}) はワークスペース・プロジェクトの画面だけで出し、その設定タブへ go", ...);
test("ルートが変わったら閉じる", ...); // Review Focus 3
```

- [ ] **Step 2〜4: 失敗を確かめ、実装し、通す**(A5c-9〜A5c-12、A5c-15・16)。
- [ ] **Step 5: コミット** — `feat: パレットの状態と候補の実行`

### Task 9: 画面の部品と配線

**Files:**
- Create: `web/app/src/palette.ts`、`web/app/src/palette/CommandPalette.svelte`・`PaletteRow.svelte`(+ `CommandPalette.test.ts`)
- Modify: `web/app/src/App.svelte`(Ctrl+K と部品)、`web/app/src/wire.ts`(`createDetail` に `onOpened` を 1 行。300 行を超えるなら `wireBase.ts` に `palettePort` を置く)、`web/app/src/screens/project/ProjectTree.svelte`(`selection.provide(1, …)`)、`web/app/src/screens/planning/SprintsTab.svelte`・`ReleasesTab.svelte`(`planRequest` を見て作成ダイアログを開く)

**Interfaces:**
- Consumes: Task 8 の `createPalette`、Task 7 の `selection`・`planRequest`。

- [ ] **Step 1: 失敗するテストを書く**(`@testing-library/svelte`)

```ts
test("プレースホルダ「ID・タイトル・コマンドを入力」と下端の案内を出し、role=dialog・aria-modal=true", ...);
test("変換中の Enter では実行せず、変換中の Esc では閉じない", ...); // Review Focus 1
test("Esc・背景のクリックで閉じる", ...);
test("Ctrl+Enter で choose(…, true)", ...);
```

App のテスト(または `palette.ts` の関数のテスト)で: 入力欄にフォーカスがあっても Ctrl+K で開く、`[aria-modal="true"]` があれば開かない(Review Focus 2)。

- [ ] **Step 2〜4: 失敗を確かめ、実装し、通す** — `pnpm -C web test`・`pnpm -C web check`。
- [ ] **Step 5: コミット** — `feat: コマンドパレットの部品と Ctrl+K`

### Task 10: 通しのテスト(e2e)

**Files:**
- Create: `web/e2e/palette.spec.ts`

- [ ] **Step 1: テストを書く** — サンプルデータで: (1) Ctrl+K → ID の一部を入れてアイテムを Enter → 詳細パネル。閉じてもう一度開くと「最近開いたもの」に出る。(2) アイテムを別のワークスペースへ移し、旧 ID を入れると「旧 ID・{日付} に移動」の候補が出て、選ぶと移した先が開く。(3) ツリーの行にフォーカス → Ctrl+K →「ステータスを変更…」→ 選ぶとツリーのステータスが変わる。(4) ワークスペースの画面で「スプリントを作成」→ スプリントのタブと作成ダイアログ。(5) ダイアログを開いているときは Ctrl+K で開かない。ワークスペース名は他の spec と重ならない名前(「パレット用」など)で作る。
- [ ] **Step 2: 通す** — `pnpm -C web e2e -- palette`(1 回)。
- [ ] **Step 3: コミット** — `test: コマンドパレットを e2e で確かめる`

### Task 11: 仕様の文言・他のトラックへの書き足し・PR

**Files:**
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(A5c-14 の文言を「コマンドパレット」の節に。A5c-6 のヒントの文を候補の表の「アイテム」の行に合わせる)、`docs/superpowers/plans/2026-11-12-hedgehug-track-a.md`(A5 の行に A5c の範囲と回した先)、`docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C4 の行に「パレットのコマンド「受信箱を開く」「アクティビティを開く」をコマンドの群に足す」)、この計画の末尾に「他のトラックに渡すもの」(E: 「このクエリで」・ビューの群・「検索を開く」「マイタスクを開く」・テーブルの選択を `selection.provide(2, …)`。F2: バックログの選択を `provide(2, …)`。C4: 上のコマンド。G: 「設定: Store」「設定: 削除済み」「設定: 診断と修復」、ワークスペースを追加の Store の初期値を既定の Store にする)

- [ ] **Step 1: 書き足す**。
- [ ] **Step 2: `make check` が通る**。
- [ ] **Step 3: コミット** — `docs: パレットの仕様の文言と、他のトラックに渡すものを書き足す`
- [ ] **Step 4: PR を出す** — develop 向け。本文に範囲・回した先・計画から変えたこと。末尾に `🤖 Generated with [Claude Code](https://claude.com/claude-code)`。
