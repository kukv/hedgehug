# hedgehug トラック E2a(検索のテーブルの表示と選択)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 検索の画面のテーブルを仕上げる。固定の 17 列と追加プロパティの列、「表示する列」、列見出しの並べ替え、グループ(`group:` を含む)、各列の値の表示(「—」「(削除された値)」を含む)、行のチェック・「すべて選択」・選択バー・一括変更・一括削除、行末の「…」のメニュー、削除済みの行の無効、パレットの「{N} 件をまとめて変更…」を作る。セルの編集は E2b、ワークスペース・プロジェクトの「テーブル」タブは E2c。

**Architecture:** 並べ替え・グループ分け・列の決め方・セルの値の表示用の文字はサーバー(`domain/query` と `app/search`)が決める(E1-1 と同じく、画面で判定を書き直さない)。画面は、テーブルの設定(表示する列・グループ・列見出しの並べ替え)を E1 の画面の状態 `search` に `{query, table}` として持ち、検索の要求に並べ替えとグループを添える。テーブルの部品は E2c がワークスペース・プロジェクトのタブでも使うので、`web/app/src/screens/table/` に置く。選択は F2b1 の `createRowSelection`・`RowCheck`・`SelectionBar`、一括は F2b1・F2b2 の `bulkDelete.open`・`bulkChange.open` をそのまま使う。

**Tech Stack:** Go 1.27.3、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`: 「並べ替え」(246〜268 行)、「まとめ方」(270〜276 行)、「削除済みを探す」の行の無効(312 行)、「画面の構成」の人の表示・件数(453・455 行)、「画面の状態」のテーブルの行(469 行)、「閲覧のみの Store のアイテム」(478〜487 行)、「テーブル」の全節(652〜721 行。セルの編集の 693〜713 行は表示の部分だけ)、「一括選択と一括変更・一括削除」の「選択」(820〜825 行)、「モックからの変更点」の 991・992・1031〜1034・1059・1062・1069・1081・1083〜1086・1089・1092 行
- 02: 「コマンドパレット」の「このクエリで」(631 行)と実行の表の「{N} 件をまとめて変更…」(664 行)
- 03: 「非表示」(251 行。テーブルの列メニューでは非表示のプロパティも使える)、使わない種類のセル(246 行)
- 01: 日付の表示(136 行)と相対時間(138〜145 行)
- `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`(トラック E の分け方)、`2026-11-15-hedgehug-track-e1.md` の末尾「E2(テーブル)」、`2026-11-14-hedgehug-track-f2b1.md`・`2026-11-14-hedgehug-track-f2b2.md` の末尾「E に求めること」

## 始める前に

- E2 は 3 本に分けた(ユーザーの決定、2026-11-15)。**E2a = この計画**(検索のテーブルの表示と選択)→ E2b セルの編集・E2c ワークスペースとプロジェクトの「テーブル」タブ(E2a のあと並行)。各 1 計画・1 PR。ブランチ `feat/track-e2`(develop から)。
- ワークスペースの画面の「ツリー」タブ(プロジェクトの行・プロジェクトのメニューを含む)は E7 に入れる(ユーザーの決定、2026-11-15)。トラック E の分け方の表は Task 11 で書き換える。

## Global Constraints

E1 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。いまの数: `internal/domain/query` 11 個、`internal/app/search` 9 個、`internal/adapter/httpapi/search/search.go` 145 行、`web/app/src/screens/search/SearchScreen.svelte` 166 行、`SearchRow.svelte` 153 行、`web/app/src/search.ts` 107 行、`web/app/src/palette.ts` 75 行、`web/app/src/wire.ts` 298 行(足さない)、`web/packages/core/src/features/palette/queryGroup.ts` 約 85 行。
- 画面のパッケージの依存の向き: ui は core を import しない。ui の部品の props の型は ui の中で宣言する。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書く。人は「不明なユーザー」(core の `unknownUser`)、削除された値は「(削除された値)」(core の `deletedValue`)、閲覧のみは core の `readOnlyNote`。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする。
- e2e のビルドも CI で lint する。手元の検証は 1 回で足りる。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| E2a-1 | 列の鍵: 固定の列は並べ替えのキーと同じ綴り(`id`・`kind`・`title`・`status`・`assignee`・`priority`・`start`・`end`・`points`・`estimate`・`system`・`sprint`・`release`・`project`・`parent`・`created`・`updated`。この並びが列の並び)。追加プロパティの列は `p:` + 小文字のキー(`p:customer`)。組み込みと同じ綴りの追加プロパティ(`p:status`)も別の列になる | 04 の 658 行の 17 列の並び、04 の 1092 行(組み込みと同じ綴りは別のもの) |
| E2a-2 | 対象のワークスペースは、表示中の Store の、削除されていないすべてのワークスペース(行がなくても)。Store の並び(`StoreList`)→ ワークスペースの並び(`workspace.RankOf`、同じなら内部 ID)→ プロパティの並び(`Rank`)の順に見る。追加プロパティの列は、この順で最初に現れた順に並べ、型も最初に現れたもの。非表示のプロパティ(03 の 251 行)も列にする | 04 の 660〜663 行、03 の 251 行 |
| E2a-3 | 列の名前: 組み込みのプロパティに当たる 8 列(ステータス・担当者・優先度・開始日・終了日・ポイント・見積もり時間・関連システム)は、対象のワークスペースの名前がすべて同じならその名前、違えば標準の名前(04 の 658 行の名前)。ほかの固定の列は標準の名前。追加プロパティの列は、名前がすべて同じならその名前、違えば最初に現れたキーの綴り。対象のワークスペースがなければ標準の名前 | 04 の 662 行、1031 行 |
| E2a-4 | 列見出しで並べ替えられるのは、固定の 17 列と、組み込みと同じ綴りでない追加プロパティの列。組み込みと同じ綴りの列は見出しを押せない文字にする(押しても何も変わらない) | 04 の 1092 行(並べ替えには使えない) |
| E2a-5 | 列見出しの並べ替えは、クエリの `sort:` の代わりに `query.Order` に渡す(値の比べ方・値のないものを最後・同じ値は並び順、はクエリの `sort:` と同じ)。「並べ替えなし」に戻すとクエリの `sort:` で並ぶ | 04 の 670・672 行 |
| E2a-6 | グループ分けもサーバーが行う。使うまとめ方は、テーブルの「グループ」の設定が「なし」以外ならそれ、なければクエリの `group:`(E1 で解析済み)が 10 項目のどれかならそれ、どちらでもなければ分けない。応答の `groupBy` に使ったまとめ方、`groups` に `{label, rows}`(`rows` は `rows` の配列の位置。並べ替えたあとの順)を返す | 04 の 272・676 行。E1-1(判定を画面に持たない) |
| E2a-7 | グループの単位と見出し: ステータス・優先度・関連システムは名前(大文字と小文字を区別しない。見出しは最初に現れた綴り)、担当者はログイン名(区別しない。見出しは表示名、メンバーにいなければ「不明なユーザー」)、プロジェクトは Store とプロジェクトの内部 ID(見出しはタイトル)、親は Store と親の内部 ID(見出しは「{ID} {タイトル}」)、種類は種類の名前、スプリントは Store とスプリントの内部 ID(見出しは名前)、作成日・更新日はこのマシンの時刻帯の日(`planning.Today(t)` の日。見出しは 01 の日付の表示 `item.DateText`)。値がないときの見出しは 04 の 678〜689 行の表のとおり。ステータス・優先度・関連システムの値の ID がワークスペースの一覧にないときは「(削除された値)」の見出しにまとめる | 04 の 678〜691 行、1033・1069・1084 行、01 の 136 行。削除された値の見せ方は 01 の「(削除された値)」 |
| E2a-8 | 関連システムで複数の値を持つアイテムは、その値をワークスペースの定義順に並べ、その順にまだないグループを作り、それぞれのグループに入る。グループの見出しの件数は、そのグループの行の数 | 04 の 691 行、1086 行 |
| E2a-9 | 行の値の表示: サーバーは、固定の列の値(ステータス・担当者・優先度・開始日・終了日・ポイント・見積もり時間・関連システムの名前・スプリント名・リリース名・プロジェクト・親・作成と最後に変わった時刻)と、追加プロパティの列ごとの `{text, type}`、「—」にする列の鍵 `unused`、「(削除された値)」にする列の鍵 `removed` を返す。`text` は、単一選択・複数選択は名前(複数は「, 」でつなぐ)、ユーザーは表示名(いなければ「不明なユーザー」)、日付は `YYYY-MM-DD`、数値は数、チェックは `true` か空、URL・テキストはそのまま、値がなければ空。`type` はアイテムのワークスペースのプロパティの型(列の型と違ってもよい) | 04 の 693〜713 行、1032 行(同じキーで型が違っても 1 列) |
| E2a-10 | 「—」にする列(`unused`): 組み込みのプロパティの 7 列(ステータスを除く)は、アイテムのワークスペースでそのプロパティの `Kinds` にアイテムの種類がないとき。スプリントは Story・Task・Bug 以外、リリースは Step。追加プロパティの列は、アイテムのワークスペースにそのキーのプロパティがないか、`Kinds` に種類がないとき | 03 の 246 行、04 の 710・711・713 行。ワークスペースにキーがない列も、値を持てない列なので同じ「—」にする |
| E2a-11 | 「(削除された値)」にする列(`removed`): ステータス・優先度・単一選択で ID が一覧にないとき、関連システム・複数選択で ID のどれかが一覧にないとき(詳細パネルの `choiceRow` と同じ。文字は「(削除された値)」だけを薄く出す) | 04 の 713 行、core の `properties.ts` の `choiceRow` |
| E2a-12 | 画面での表示: ID は Store の点・ID・「旧 ID」(E1 のまま)、種類は種類のアイコンと名前、タイトルは詳細パネル(削除済みは E1 のまま押せない)、ステータスは点と名前、担当者・ユーザーはアバターと表示名、優先度・単一選択・複数選択・関連システム・スプリント・リリース・テキスト・数値はそのままの文字、見積もり時間は「{N} 時間」(`item.Hours` と同じ丸め。core の `hoursText`)、開始日と日付は 01 の日付の表示(`formatDay`)、終了日は `dueLabel`、プロジェクトはタイトル、親は ID(押すと親の詳細パネル、`title` に親のタイトル)、作成日・更新日は `relativeTime`、チェックは押せないチェックボックス、URL はリンク(新しいタブ)。列「更新日」の見出しの `title` は「GitHub と同期による変更は含みません」。セルの編集の部品は E2b が置き換える | 04 の 697〜713 行、01 の 136〜145 行 |
| E2a-13 | テーブルの設定は、画面の状態 `search` の中身を `{query, table: {columns, group, sort}}` にして持つ。`columns` は出す列の鍵の並び(左から)、`group` は `none` か 10 項目の鍵、`sort` は `{key, desc}`(列の鍵)か null。初めは `columns` が `["id","title","status","assignee","priority","end","points"]`、`group` が `none`、`sort` が null。E1 の `{query}` だけの中身・形の合わない値は、その項目だけ初めの値にする | 04 の 469・664 行、E1-16 |
| E2a-14 | 「表示する列」は ui の `ValueMenu`(`multi`、7 件以上で検索欄)で、対象の列すべてを列の並び(E2a-1・E2a-2)で出し、出している列にチェック。タイトルは選べない項目(`disabled`)。出すと `columns` の末尾に足し、隠すと外す。いまの対象にない列の鍵は `columns` に残したまま表示しない(Store を表示に戻すと出る)。列を隠しても、その列の並べ替えは残す | 04 の 665・666 行 |
| E2a-15 | 「グループ」は ui の `Select`(名前「グループ」)。選択肢は「なし」「ステータス」「担当者」「プロジェクト」「親」「種類」「優先度」「スプリント」「関連システム」「作成日」「更新日」(04 の 678 行の表の順) | 04 の 676〜689 行 |
| E2a-16 | 選択: 検索の画面の選択は `web/app/src/search.ts` の `searchSelection`(`createRowSelection`)。結果が変わるたびに、選べる行(削除済みでない行。並べ替えたあとの順に 1 回ずつ)で `prune` する(core の `createSearch` が行う)。検索の画面を離れたら `clear`。「すべて選択」は見出しの行の先頭のチェック(ui の `RowCheck`、名前「すべて選択」)で、選べる行がすべて選ばれていればチェック済み。押すと、チェック済みなら選択をすべて外し、そうでなければ選べる行をすべて選ぶ(`RowSelection` に足す `set` で 1 回で入れる)。削除済みの行のチェックは無効 | 04 の 717・820〜823 行、312 行 |
| E2a-17 | 行末の「…」(ui の `IconButton`、名前「{ID} のメニュー」)は、押すとそのアイテムのプロジェクトのアイテムを `itemsPort.list` で読み、`itemMenuHost.open(item, { rows: 兄弟の並び(compareSiblings), children, row: true }, x, y)` で 02 のアイテムのメニューを開く(メニューの操作のあとは、Store の変更の知らせ(`syncing/watch.go`。書き手を問わない)で検索が読み直す)(ツリーの行と同じ項目。閲覧のみの Store の変更系の項目は host が無効にする)。読めない・アイテムがないときは何もしない。削除済みの行の「…」は無効 | 04 の 717・1059 行、482 行 |
| E2a-18 | パレットの「選んでいるアイテム」に、検索のテーブルで 1 行だけ選んでいるときのアイテムを rank 2(バックログと同じ。同時には出ない)、`source: "table"` で渡す。`setStatus` は `itemsPort.setProperty(storeId, itemId, "status", statusId)` のあと検索を読み直す | F2b1 の「E に求めること」、A5c-9 |
| E2a-19 | パレットの「このクエリで」の群に「{N} 件をまとめて変更…」を「検索で開く」の次に足す(E3 が「ビューとして保存…」をあいだに入れる)。N は応答の行の数。無効の理由は、`is:deleted` の検索なら「削除済みのアイテムは変更できません」、0 件なら「一致するアイテムがありません」、行がすべて閲覧のみの Store のものなら「閲覧のみの Store です」(この順で最初の 1 つ)。選ぶと、パレットを閉じ、「検索で開く」と同じ道で検索の画面を開いて検索が終わるのを待ち(ほかの画面からは、画面の `open` の検索が終わるまで待つ。`settled`)、選べる行をすべて選んでから `bulkChange.open(searchSelection, { after: () => search.reload() })` | 02 の 631・664 行、04 の 822 行 |
| E2a-20 | 一括変更・一括削除の `after` は `search.reload()`。一括変更のあとは選択を残し、一括削除のあとは解除する(F2b の部品がそうしている) | 04 の 897 行、F2b2 の「E に求めること」 |
| E2a-21 | API: `POST /api/search` の本文に `sort`(`{key, desc}` か null。列見出しの並べ替え。追加プロパティは `p:` を付けない小文字のキー)と `group`(テーブルの「グループ」の設定。`none` か空なら設定なし)を足す。応答に `columns`(`{key, name, type, sortable}`。固定の列の `type` は空)、`groupBy`、`groups` と、行の値(E2a-9)を足す。パレットはどちらも送らない | E1-24 |

## Review Focus

1. **関連システムで分けると、同じアイテムが 2 回数えられる。** Web と API を持つアイテムは「Web」と「API」の両方の見出しの下に出て、見出しの件数にはそれぞれ数えるが、ツールバーの「{N} 件」、「{N} 件を選択中」、「すべて選択」は 1 回だけ数える。片方の見出しの下でチェックすると、もう片方の行もチェック済みになる(04 の 691 行)。Task 1 の `TestGroupsSystemsMultiple` と Task 9 の e2e に入れる。
2. **列見出しの並べ替えを「並べ替えなし」に戻すと、`sort:` が効かなくなる。** `sort:-points` のクエリで「タイトル」の見出しを 3 回押すと、ポイントの降順に戻る(04 の 672 行)。Task 2 の `TestSearchHeaderSortOverridesQuery` に入れる。
3. **同じキーで型が違う追加プロパティで、表示が崩れる。** ワークスペース A の `due`(日付)と B の `due`(テキスト「来週」)は 1 列(型は日付)にまとまり、B のアイテムのセルは「来週」と出る(日付として読もうとして空や「Invalid Date」にならない。04 の 663 行)。Task 2 の `TestColumnsSameKeyDifferentType`、Task 3 の `TestCellsOwnType`、Task 7 の `cellView uses the cell's own type` に入れる。
4. **削除済みの行が「すべて選択」に入る。** `is:deleted` の検索では、「すべて選択」を押しても 0 件のまま、行のチェックと「…」は押せない。パレットの「{N} 件をまとめて変更…」は「削除済みのアイテムは変更できません」で無効(04 の 312 行、02 の 664 行)。Task 5 の `selectable leaves out deleted rows`、Task 10 の `bulk change is disabled for is:deleted` に入れる。
5. **選択中のアイテムが一覧から消えても、選択に残る。** 3 件を選んだあと、別のブラウザのタブで 1 件を削除すると「2 件を選択中」になる。絞り込みを変えて一覧から外れたものも外れる(04 の 823 行)。Task 6 の `prunes the selection with each result` と Task 9 の e2e に入れる。

---

### Task 1: グループ分けの規則

**Files:**
- Create: `internal/domain/query/group.go`
- Modify: `internal/domain/query/facts.go`(`ParentTitle string` を足す)、`internal/domain/item/format.go`(`func DateText(s string, today time.Time) string` を足し、`dateText` を呼ぶだけにする)
- Test: `internal/domain/query/group_test.go`

**Interfaces:**
- Produces:
  - `var GroupKeys = []string{"status", "assignee", "project", "parent", "kind", "priority", "sprint", "system", "created", "updated"}`
  - `func GroupKey(setting, fromQuery string) string` — E2a-6。どちらも使えなければ ""
  - `type Group struct { Label string; Rows []int }`
  - `func Groups(facts []*Facts, key string, now time.Time) []Group` — `facts` は並べ替えたあとの順。`key` が "" なら nil

- [ ] **Step 1: 失敗するテストを書く**

`group_test.go`(材料は `match_names_test.go` の `facts(t, …)` の助けを使う):
- `TestGroupKey`: `("assignee", "kind")` → `assignee`、`("none", "kind")` → `kind`、`("", "foo")` → ``、`("none", "")` → ``、`("bogus", "status")` → `status`。
- `TestGroupsOrderOfFirstAppearance`: ステータスが Done・Todo・Done の順の 3 行 → `[{Done [0 2]} {Todo [1]}]`(ステータスの定義順ではない)。
- `TestGroupsStatusByNameAcrossWorkspaces`: ワークスペース A の「Review」と B の「review」は 1 つのグループで、見出しは最初の綴り「Review」。
- `TestGroupsEmptyLabels`: 担当者なし →「未割り当て」、プロジェクト・ステータスがない →「なし」、親なし →「親なし」、スプリントなし →「スプリントなし」、関連システムなし →「関連システムなし」。
- `TestGroupsAssigneeUnknown`: `Names` にない担当者 →「不明なユーザー」、ある担当者 → 表示名。`Tanaka` と `tanaka` は 1 つ。
- `TestGroupsParentLabel`: 親 HH-3「ログイン」→「HH-3 ログイン」。
- `TestGroupsSystemsMultiple`(Review Focus 1): 関連システムの定義順が Web・API・iOS のワークスペースで、行 0 が [API, Web]、行 1 が [API] → `[{Web [0]} {API [0 1]}]`(行 0 の値を定義順に並べた順にグループを作る)。
- `TestGroupsRemovedValue`: ステータスの ID が一覧にない行 →「(削除された値)」。
- `TestGroupsCreatedByLocalDay`: このマシンの時刻帯で 11/15 23:30 と 11/16 00:10 に作った 2 行は別のグループで、見出しは今年なら「11/15」「11/16」。
- `TestGroupsNoKey`: `key` が "" なら nil。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/query/ -run 'TestGroup'`
Expected: FAIL(`undefined: GroupKey`)

- [ ] **Step 3: `GroupKey`・`Groups`・`item.DateText` を書く(E2a-6〜E2a-8)**

日は `planning.Today(t)` と同じ `t.Local().Format(time.DateOnly)`(`query` は `planning` を import しない。同じ式を書く)、見出しは `item.DateText(日, now)`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/query/ ./internal/domain/item/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/query internal/domain/item
git commit -m "feat(query): テーブルのグループ分けの規則を足す(E2a Task 1)"
```

---

### Task 2: 列の決め方と、列見出しの並べ替え・グループを検索に通す

**Files:**
- Create: `internal/app/search/columns.go`
- Modify: `internal/app/search/keys.go`(`customKeys` を、キーの一覧と列を 1 回で集める `catalog` にする)、`internal/app/search/search.go`(`TableOptions`・`Result` の足し)、`internal/app/search/facts.go`(`ParentTitle` を埋める)、`internal/adapter/httpapi/search/search.go`(`svc.Search(ctx, q, appsearch.TableOptions{})` に合わせるだけ。本文の足しは Task 4。`Search` を呼ぶのはここだけ)
- Test: `internal/app/search/columns_test.go`、`internal/app/search/search_test.go`

**Interfaces:**
- Consumes: Task 1 の `query.GroupKey`・`query.Groups`・`query.Group`
- Produces:
  - `type Column struct { Key, Name string; Type workspace.PropertyType /* 固定の列は "" */; Sortable bool; Builtin string /* 組み込みのプロパティの鍵(status・systems など)。ほかは "" */ }`
  - `type TableOptions struct { Sort *query.Sort; Group string }`
  - `Result` に `Columns []Column; GroupBy string; Groups []query.Group` を足す
  - `func (s Service) Search(ctx context.Context, text string, opt TableOptions) (Result, error)`

- [ ] **Step 1: 失敗するテストを書く**

`columns_test.go`:
- `TestColumnsFixedOrder`: 追加プロパティがないとき、鍵が E2a-1 の 17 個の順、名前が「ID」「種類」「タイトル」「ステータス」「担当者」「優先度」「開始日」「終了日」「ポイント」「見積もり時間」「関連システム」「スプリント」「リリース」「プロジェクト」「親」「作成日」「更新日」、すべて `Sortable`。
- `TestColumnsBuiltinNames`: 全ワークスペースで「ポイント」を「SP」に改名 → 列名「SP」。1 つだけ改名 →「ポイント」。
- `TestColumnsCustomMerged`: Store 1 の A に `customer`(テキスト、名前「顧客」)、Store 2 の B に `Customer`(テキスト、名前「お客様」)→ 列 1 つ、鍵 `p:customer`、名前 `customer`(最初に現れたキーの綴り)。
- `TestColumnsSameKeyDifferentType`(Review Focus 3): A の `due` が日付、B の `due` がテキスト → 列 1 つ、型は日付(Store の並び → ワークスペースの並びで A が先)。
- `TestColumnsBuiltinSpelling`: 追加プロパティのキー `status` → 鍵 `p:status`、`Sortable` が偽。
- `TestColumnsHiddenStoreLeftOut`: 非表示の Store にだけある `customer` は列にない(キーとしてはエラーにならない E1 の振る舞いは変えない)。
- `TestColumnsHiddenPropertyKept`: 非表示にした「優先度」も列にある。

`search_test.go` に足す:
- `TestSearchHeaderSortOverridesQuery`(Review Focus 2): `sort:-points` に `TableOptions{Sort: &query.Sort{Key: "title"}}` でタイトルの順、`TableOptions{}` でポイントの降順。
- `TestSearchGroupSettingWins`: `group:kind` に `TableOptions{Group: "assignee"}` → `GroupBy` が `assignee`。`Group: "none"` → `kind`。`group:foo` だけ → `GroupBy` が ""、`Groups` が nil。
- `TestSearchGroupsUseSortedRows`: `sort:title group:status` の `Groups` の `Rows` が、並べ替えたあとの行の位置を指す。
- `TestSearchColumnsOnError`: `status:` のエラーでも `Columns` は返る(「表示する列」を開けるように)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/search/`
Expected: FAIL

- [ ] **Step 3: 列・オプション・グループを書く(E2a-1〜E2a-6)**

`catalog(ctx) (keys map[string]bool, columns []Column, err error)` は、E1 の `customKeys` の読み方(`SyncTargets` のすべての Store)のまま、表示中の Store(`StoreList`)の分だけ、ワークスペースを `workspace.RankOf`(ワークスペースのログ)→ 内部 ID の順にして列を集める。並べ替えは `opt.Sort` があれば `q.Sort` の代わりに使う。グループは `query.GroupKey(opt.Group, q.Group)` で決め、並べ替えたあとの `Facts` で `query.Groups` を呼ぶ。`ParentTitle` は親のアイテムのタイトル(`ancestors` が親を読むところで埋める)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/... && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal
git commit -m "feat(search): テーブルの列を決め、列見出しの並べ替えとグループを検索に通す(E2a Task 2)"
```

---

### Task 3: 行の値の表示用の材料

**Files:**
- Create: `internal/app/search/cells.go`
- Test: `internal/app/search/cells_test.go`

**Interfaces:**
- Consumes: Task 2 の `Column`
- Produces:
  - `type Cell struct { Text string; Type workspace.PropertyType }`
  - `type Cells struct { Unused []string; Removed []string; Custom map[string]Cell /* 列の鍵(p:…)→ 値 */; Systems []string /* 名前。定義順。一覧にない ID は「(削除された値)」 */ }`
  - `func cellsOf(f query.Facts, columns []Column) Cells`(パッケージの中だけ。`Row` に `Cells Cells` を足し、`Search` が埋める)

- [ ] **Step 1: 失敗するテストを書く**

`cells_test.go`:
- `TestCellsUnusedByKind`: 優先度の `Kinds` に Step がないワークスペースの Step → `Unused` に `priority`。Epic → `sprint` と(Epic が使うなら入らない)。Step → `release` と `sprint`。Story → どちらも入らない。ステータスはどの種類でも入らない。
- `TestCellsUnusedMissingProperty`: 列 `p:customer` があり、アイテムのワークスペースに `customer` がない → `Unused` に `p:customer`、`Custom` にない。
- `TestCellsRemoved`: 優先度の ID が一覧にない → `Removed` に `priority`。関連システム [Web, 消えた ID] → `Removed` に `system`、`Systems` は `["Web", "(削除された値)"]`。複数選択の `p:tags` に消えた ID → `Removed` に `p:tags`。
- `TestCellsCustomText`: 単一選択「A社」→「A社」、複数選択 [x, y] → 「x, y」、ユーザー tanaka(名簿に「田中」)→「田中」、名簿にない人 →「不明なユーザー」、日付 → `2026-12-01`、数値 3.5 →「3.5」、チェックのオン →「true」・オフ → 空、値なし → 空。
- `TestCellsOwnType`(Review Focus 3): 列 `p:due` の型が日付で、アイテムのワークスペースの `due` がテキスト「来週」→ `Custom["p:due"]` が `{来週 text}`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/search/ -run TestCells`
Expected: FAIL

- [ ] **Step 3: `cellsOf` を書き、`Search` で行ごとに埋める(E2a-9〜E2a-11)**

値は `item.Current(it, p)`、ユーザーの表示名は `Facts.Names`(なければ `unknownUser = "不明なユーザー"`)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/search/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/search
git commit -m "feat(search): 行の値の表示用の材料(「—」・削除された値・追加プロパティ)を作る(E2a Task 3)"
```

---

### Task 4: 検索の API にテーブルの設定と応答の足しを通す

**Files:**
- Create: `internal/adapter/httpapi/search/table.go`(列・グループ・行の足しの型と変換。`search.go` は 145 行なので分ける)
- Modify: `internal/adapter/httpapi/search/search.go`、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`)、`web/packages/api/src/search.ts`、`web/packages/core/src/port/search.ts`、`web/packages/core/src/features/search/types.ts`
- Test: `internal/adapter/httpapi/search/search_test.go`

**Interfaces:**
- Consumes: Task 2・3 の `TableOptions`・`Result`・`Column`・`Cells`
- Produces(E2a-21):
  - 本文 `SearchQuery { query: string; sort?: {key: string; desc: boolean} | null; group?: string }`
  - 応答 `SearchResult` に `columns: SearchColumn[]`(`{key, name, type, sortable}`)、`groupBy: string`、`groups: SearchGroup[]`(`{label, rows: number[]}`。分けないときは空の配列)
  - `SearchRow` に `start: string`、`estimate: number | null`、`systems: string[]`、`sprint: string`、`release: string`、`project: {display: string; title: string}`、`parent: {itemId: string; display: string; title: string} | null`、`created: string`、`updated: string`(RFC 3339)、`unused: string[]`、`removed: string[]`、`custom: Record<string, {text: string; type: string}>`
  - core: `interface SearchTableRequest { sort: { key: string; desc: boolean } | null; group: string }`、`SearchPort.search(query: string, table?: SearchTableRequest): Promise<SearchResult>`、上の型を `types.ts` に

- [ ] **Step 1: 失敗するテストを書く**

`search_test.go` に足す:
- `TestSearchAPITable`: サンプルの Store で `{"query":"","sort":{"key":"title","desc":true},"group":"kind"}` → 200、`groupBy` が `kind`、`groups` の `rows` の位置の行の `kind` がその見出しと同じ、`columns` の先頭 3 つの `key` が `id`・`kind`・`title`。
- `TestSearchAPIRowValues`: 終了日・関連システム・親のあるアイテムの行に `systems`・`parent.display`・`parent.title`・`created`・`updated` が入る。
- `TestSearchAPIWithoutTable`: E1 の本文 `{"query":"kind:bug"}` のままでも 200(`groups` は空の配列、`groupBy` は空)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/search/`
Expected: FAIL

- [ ] **Step 3: 型と変換を書き、`make generate` を流し、api と core の口と型を合わせる**

`api/src/search.ts` の `createSearchPort` は `table` があれば `sort`・`group` を本文に入れる。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./... && make lint && pnpm -C web check`
Expected: PASS(`api/openapi.yaml` と `generated.ts` が変わっている)

- [ ] **Step 5: コミット**

```bash
git add internal api web/packages
git commit -m "feat(search): 検索の API にテーブルの設定と、列・グループ・行の値を足す(E2a Task 4)"
```

---

### Task 5: テーブルの設定と選べる行(core)

**Files:**
- Create: `web/packages/core/src/features/search/table.ts`
- Modify: `web/packages/core/src/index.ts`、`web/packages/core/src/features/selection/rowSelection.ts`(`set`)
- Test: `web/packages/core/src/features/search/table.test.ts`、`web/packages/core/src/features/selection/rowSelection.test.ts`

**Interfaces:**
- Consumes: Task 4 の `SearchColumn`・`SearchRow`・`SearchTableRequest`、selection の `SelectedRow`
- Produces:
  - `interface TableSettings { columns: string[]; group: string; sort: { key: string; desc: boolean } | null }`、`const defaultTable: TableSettings`(E2a-13)
  - `function readTable(raw: unknown): TableSettings` — 形の合わない項目だけ初めの値
  - `const groupChoices: { id: string; label: string }[]`(E2a-15 の 11 個)
  - `function shownColumns(t: TableSettings, columns: SearchColumn[]): SearchColumn[]` — `t.columns` の順で、対象にあるものだけ
  - `function columnChoices(t: TableSettings, columns: SearchColumn[]): { id: string; label: string; checked: boolean; disabled?: boolean }[]` — 対象の列の並び。タイトルは `disabled`
  - `function toggleColumn(t: TableSettings, key: string): TableSettings` — タイトルは何もしない。出すと末尾
  - `function nextSort(t: TableSettings, key: string): TableSettings` — 昇順 → 降順 → なし。別の列なら昇順から
  - `function tableRequest(t: TableSettings): SearchTableRequest` — `p:` を外して小文字のキーにする
  - `function selectable(rows: SearchRow[]): SelectedRow[]` — 削除済みを除き、行の順に 1 回ずつ
  - `function headerCheck(rows: SearchRow[], selected: SelectedRow[]): boolean` — 選べる行が 1 件以上あり、すべて選ばれているか
  - `RowSelection.set(rows: SelectedRow[]): void` — 選択を rows に入れ替える(同じ行は 1 つ。変わらなければ知らせない)

- [ ] **Step 1: 失敗するテストを書く**

`table.test.ts`:
- `readTable`: `undefined` → `defaultTable`。`{columns: "x", group: "kind", sort: {key: 1}}` → `columns` は初めの値、`group` は `kind`、`sort` は null。`{group: "bogus"}` → `none`。
- `toggleColumn`: 初めの値に `start` → 末尾に `start`。`priority` → 外れる。`title` → 変わらない。
- `nextSort`: null →`{title, false}` →`{title, true}` → null。`{title, true}` で `points` →`{points, false}`。
- `tableRequest`: `{sort: {key: "p:Customer", desc: true}, group: "none"}` → `{sort: {key: "customer", desc: true}, group: "none"}`。
- `shownColumns`: `columns` に対象にない `p:gone` があっても出さず、`t.columns` の順。
- `columnChoices`: タイトルの行が `disabled`、出している列が `checked`、並びは対象の列の並び。
- `selectable leaves out deleted rows`(Review Focus 4): 削除済みの行は入らない。
- `headerCheck`: 選べる行 2 件のうち 2 件選択で真、1 件で偽、選べる行 0 件で偽。
- `rowSelection.test.ts`: `set` で 3 件を入れると `rows` が 3 件で、知らせは 1 回。同じ 3 件をもう一度 `set` しても知らせない。`set([])` で空。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- table rowSelection`
Expected: FAIL

- [ ] **Step 3: `table.ts` と `set` を書く**

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): テーブルの設定と選べる行、選択の入れ替えを足す(E2a Task 5)"
```

---

### Task 6: 検索の流れにテーブルの設定と選択をつなぐ(core)

**Files:**
- Modify: `web/packages/core/src/features/search/search.ts`
- Test: `web/packages/core/src/features/search/search.test.ts`

**Interfaces:**
- Consumes: Task 5 の `TableSettings`・`readTable`・`tableRequest`・`selectable`、selection の `RowSelection`
- Produces:
  - `createSearch` の deps に `selection?: Pick<RowSelection, "prune">`
  - `SearchState` に `table: TableSettings`
  - `setTable(next: TableSettings): void` — 画面の状態に書く。`sort` か `group` が変わったときだけ、待たずに検索する(列の出し入れでは検索しない)
  - `settled(): Promise<void>` — 待ち・飛んでいる検索・`prepareQuery` で入れ替えてまだ検索していない絞り込みがなくなったら解ける(どれもなければすぐ)。`prepareQuery` のあとは、`open` の検索の応答で解ける

- [ ] **Step 1: 失敗するテストを書く**

`search.test.ts` に足す:
- 画面の状態 `{"query":"x"}`(E1 の形)を `open` すると、`table` が `defaultTable`。
- `setTable` で `columns` だけ変えると、画面の状態に書くが検索しない。`group` を変えると、`port.search` が `("x", {sort: null, group: "kind"})` で 1 回呼ばれる。
- `setQuery` の検索にも、いまの `tableRequest` が添えられる。
- `prunes the selection with each result`(Review Focus 5): 応答が着くたびに `selection.prune` が `selectable(rows)` で呼ばれる。エラーの応答(行なし)では空で呼ばれる。
- `settled`: `setQuery` の直後に呼ぶと、150ms の待ちと応答のあとで解ける。何もなければすぐ解ける。`prepareQuery("kind:bug")` のあとに呼ぶと、`open` を呼ぶまで解けず、`open` の検索の応答で解ける(そのときの `result` は `kind:bug` の応答)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- search.test`
Expected: FAIL

- [ ] **Step 3: 書く**

画面の状態の中身は `{query, table}`。`replaceQuery`・`prepareQuery` は `table` を変えない(04 の 359 行: 絞り込みだけを置き換える)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 検索の流れにテーブルの設定と選択の整理をつなぐ(E2a Task 6)"
```

---

### Task 7: セルの表示の中身(core)と、無効にできる行のチェック(ui)

**Files:**
- Create: `web/packages/core/src/features/search/cells.ts`
- Modify: `web/packages/core/src/index.ts`、`web/packages/ui/src/patterns/RowCheck.svelte`(`disabled?: boolean` を足す)
- Test: `web/packages/core/src/features/search/cells.test.ts`、`web/packages/ui/src/patterns/RowCheck.test.ts`

**Interfaces:**
- Consumes: Task 4 の `SearchRow`・`SearchColumn`、core の `dueLabel`・`formatDay`・`relativeTime`・`deletedValue`・`unknownUser`
- Produces:
  - `type CellView = { kind: "empty" } | { kind: "unused" } | { kind: "removed" } | { kind: "text"; text: string } | { kind: "due"; text: string; emphasis: "overdue" | "today" | null } | { kind: "check"; checked: boolean } | { kind: "url"; href: string } | { kind: "relative"; text: string } | { kind: "person"; name: string }`
  - `function cellView(row: SearchRow, column: SearchColumn, now: Date): CellView` — ID・種類・タイトル・ステータス・担当者・親は呼ぶ側(部品)が描くので、それ以外の列。`unused` が先、次に `removed`
  - `function hoursText(n: number): string` — 小数第 2 位で丸め、末尾の 0 を省く(`item.Hours` と同じ)
  - `const updatedHint = "GitHub と同期による変更は含みません"`

- [ ] **Step 1: 失敗するテストを書く**

`cells.test.ts`(今日は 2026-11-15):
- `unused` に `priority` があれば、値があっても `{kind: "unused"}`(「—」)。`removed` に `priority` があれば `{kind: "removed"}`(「(削除された値)」)。
- `end` が `2026-11-10` で分類 todo → `{due, "期限切れ 11/10", overdue}`。完了 → 強調なし。
- `start` が `2027-01-05` → `{text, "2027/01/05"}`。空 → `{empty}`。
- `estimate` 1.255 → `{text, "1.26 時間"}`、2 → `{text, "2 時間"}`。
- `system` → 名前を「, 」でつなぐ。`sprint` 空 → `{empty}`。
- `created` が 3 時間前 → `{relative, "3 時間前"}`。
- `p:due`(列の型は日付)で `custom` が `{text: "来週", type: "text"}` → `{text, "来週"}`(`cellView uses the cell's own type`、Review Focus 3)。`{text: "2026-12-01", type: "date"}` → `{text, "12/1"}`。
- `p:owner` `{text: "田中", type: "user"}` → `{person, "田中"}`(アバターと表示名)。`p:done` `{text: "true", type: "check"}` → `{check, true}`。`p:site` `{text: "https://example.com", type: "url"}` → `{url}`。
- `hoursText`: 0.5 →「0.5」、1.005 →「1.01」、3 →「3」。

`RowCheck.test.ts`: `disabled` なら押しても `onchange` を呼ばない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- cells && pnpm -C web --filter @hedgehug/ui test -- RowCheck`
Expected: FAIL

- [ ] **Step 3: 書く(E2a-12)**

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test && pnpm -C web --filter @hedgehug/ui test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages
git commit -m "feat(core): テーブルのセルの表示の中身を足し、行のチェックを無効にできるようにする(E2a Task 7)"
```

---

### Task 8: テーブルの部品(列・列見出しの並べ替え・グループ・セル)とツールバー

**Files:**
- Create: `web/app/src/screens/table/ItemTable.svelte`(見出しの行・グループの見出しの行・行の並び)、`web/app/src/screens/table/TableHead.svelte`(列見出し。並べ替えのボタンと ↑↓、`aria-sort`)、`web/app/src/screens/table/TableRow.svelte`(`SearchRow.svelte` を移して、列ごとに描く)、`web/app/src/screens/table/TableCell.svelte`(`cellView` の描き分け)、`web/app/src/screens/table/TableToolbar.svelte`(「表示する列」・「グループ」・件数)
- Delete: `web/app/src/screens/search/SearchTable.svelte`、`web/app/src/screens/search/SearchRow.svelte`
- Modify: `web/app/src/screens/search/SearchScreen.svelte`(ツールバーとテーブルを差し替え、`search.setTable` をつなぐ)、`web/e2e/search.spec.ts`(E1 の「列の見出し 7 つ」の確かめを、この Task 以降の形に合わせる)
- Create: `web/e2e/search-table.spec.ts`
- Test: `web/e2e/search-table.spec.ts`(`web/app` には単体テストがない。中身は Task 5・7 の core のテスト)

**Interfaces:**
- Consumes: Task 5 の `shownColumns`・`columnChoices`・`toggleColumn`・`nextSort`・`groupChoices`、Task 6 の `search.setTable`、Task 7 の `cellView`・`updatedHint`、ui の `ValueMenu`・`Select`・`Button`・`KindIcon`・`StatusDot`・`Avatar`
- Produces:
  - `ItemTable` の props `{ rows: SearchRow[]; columns: SearchColumn[] /* 出す列 */; groups: SearchGroup[]; sort: TableSettings["sort"]; onsort(key: string): void; selection: RowSelection; onmenu(row: SearchRow, rect: DOMRect): void }`(選択と「…」は Task 9 がつなぐ。この Task では `selection` を受け取るだけで、チェックの列はまだ描かない)
  - `TableToolbar` の props `{ count: number | null; columns: SearchColumn[] /* 対象の列すべて */; table: TableSettings; onchange(next: TableSettings): void; children?: Snippet /* 画面ごとの操作 */ }`

- [ ] **Step 1: 部品を書く**

- ツールバーは左から「表示する列」(ボタン。押すと `ValueMenu` を `multi` で開き、選ぶたびに `toggleColumn`)、「グループ」(`Select`)、件数「{N} 件」(E1 のまま)。
- 列見出しは、`sortable` ならボタンで、押すと `nextSort`。いまの並べ替えの列に「↑」(昇順)か「↓」(降順)、`aria-sort` は `ascending`・`descending`・`none`。`sortable` が偽なら押せない文字。「更新日」の見出しの `title` は `updatedHint`。
- グループがあるときは、`groups` の順に、見出しの行(`<th colspan>` に見出しと件数)と、その `rows` の位置の行を描く。同じアイテムが複数の見出しの下に出るので、`each` の鍵はグループの位置と行の位置を合わせたものにする。
- セルは E2a-12 のとおり。ID・種類・タイトル・ステータス・担当者・親は `TableRow` で描き、ほかは `TableCell` が `cellView` で描く(「—」は `unused`、「(削除された値)」は薄い字)。`TableRow` も、担当者が `unused` なら「—」、ステータスが `removed` なら「(削除された値)」にする。親は押すと `detail.open(row.storeId, row.parent.itemId)`。
- `SearchScreen.svelte` は 200 行以内に収める(ツールバーとテーブルを部品に出した分で足りる)。

- [ ] **Step 2: 通しのテストを書く**

`search-table.spec.ts`(作ったアイテムのタイトルに日時を入れて、サンプルの件数に頼らない):
- 「表示する列」で「開始日」を選ぶと、列が右端に付き、メニューは開いたまま。「タイトル」は選べない。再読み込みしても「開始日」の列が残る。
- 「タイトル」の見出しを押すと ↑、もう一度で ↓、もう一度で印が消える。
- 「グループ」を「種類」にすると、見出しの行「Story {N}」などが出る。「なし」に戻して `group:kind` を打っても同じ。「グループ」を「担当者」にすると、`group:kind` があっても担当者で分かれる。
- 関連システムを持たないアイテムの「関連システム」の列を出すと空、優先度を使わない種類(サンプルの設定で使わない種類があればそれ、なければ設定で外してから)の行の「優先度」は「—」。

- [ ] **Step 3: 流して確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/search-table.spec.ts e2e/search.spec.ts`
Expected: PASS

- [ ] **Step 4: 型と lint と大きさを確かめる**

Run: `pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/app web/e2e
git commit -m "feat(web): 検索のテーブルに列・表示する列・列見出しの並べ替え・グループ・セルの表示を足す(E2a Task 8)"
```

---

### Task 9: 選択・選択バー・一括・行末のメニュー

**Files:**
- Create: `web/packages/core/src/features/search/rowMenu.ts`、`web/app/src/screens/table/SelectAllCheck.svelte`(見出しの行の「すべて選択」)
- Modify: `web/packages/core/src/index.ts`、`web/app/src/screens/table/ItemTable.svelte`・`TableRow.svelte`(先頭のチェックの列と末尾の「…」の列)、`web/app/src/screens/search/SearchScreen.svelte`(`SelectionBar`・パレットの登録・離れたときの解除)、`web/app/src/search.ts`(`searchSelection`・`openRowMenu`・`createSearch` に `selection` を渡す)、`web/e2e/search-table.spec.ts`
- Test: `web/packages/core/src/features/search/rowMenu.test.ts`、`web/e2e/search-table.spec.ts`

**Interfaces:**
- Consumes: Task 5 の `selectable`・`headerCheck`・`RowSelection.set`、F2b1 の `createRowSelection`・`isSelected`・`selectedText`・`allReadOnly`・ui の `RowCheck`・`SelectionBar`、`wireActions` の `bulkChange`・`bulkDelete`・`itemMenuHost`、core の `menuAt`・`compareSiblings`、`palette.ts` の `selection`
- Produces:
  - `function rowMenuContext(items: Item[], itemId: string): { item: Item; ctx: ItemMenuHostContext } | null`(core。E2a-17。`ctx.rows` は `compareSiblings` で並べたプロジェクトのアイテム、`ctx.children` は親が `itemId` のもの、`ctx.row` は true。アイテムがなければ null)
  - `export const searchSelection: RowSelection`、`export async function openRowMenu(row: SearchRow, rect: { left: number; bottom: number }): Promise<void>`(`web/app/src/search.ts`。`itemsPort.list` → `rowMenuContext` → `itemMenuHost.open`。読めなければ何もしない)

- [ ] **Step 1: 失敗するテストを書く**

`rowMenu.test.ts`:
- 兄弟 3 件(順位 b・a・c)と子 1 件のプロジェクトで、`ctx.rows` が順位の順(a・b・c)、`ctx.children` が子 1 件、`ctx.row` が真。
- 一覧にない `itemId` → null。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- rowMenu`
Expected: FAIL

- [ ] **Step 3: `rowMenuContext` と画面を書く**

- 行の先頭は `RowCheck`(名前「{ID} を選択」、`checked` は `isSelected`、`disabled` は削除済みの行)。見出しの行の先頭は「すべて選択」(E2a-16。外すときは `clear`、選ぶときは `set(selectable(rows))`)。
- テーブルの上に、1 件以上選んでいるときだけ `SelectionBar`(`text` は `selectedText`、すべて閲覧のみなら `readOnlyNote`、`onchange` は `bulkChange.open(searchSelection, { after: () => search.reload() })`、`ondelete` は `bulkDelete.open(searchSelection, () => search.reload())`、`onclear` は `searchSelection.clear()`)。
- 行の末尾は「…」(`IconButton`、名前「{ID} のメニュー」、削除済みの行は無効)。押すと `openRowMenu(row, ボタンの getBoundingClientRect())`。
- `SearchScreen.svelte` で `selection.provide(2, …)`(E2a-18)を `$effect` で登録し、画面を離れるとき(`onMount` の戻り値)に `searchSelection.clear()`。

- [ ] **Step 4: 通しのテストを足す**

`search-table.spec.ts` に足す:
- 関連システムを 2 つ持つアイテムを作り、「グループ」を「関連システム」にすると、2 つの見出しの下に出る。片方でチェックすると両方チェック済みで「1 件を選択中」(Review Focus 1)。
- 2 件をチェックすると「2 件を選択中」のバー。「一括変更…」で「選択したアイテムを一括変更」が開き、優先度を変えるとトーストが出て、行の優先度が変わり、選択は残る。「一括削除…」で削除すると行が消え、バーが閉じる。
- 3 件を選んだあと、別のページ(新しいタブ)でそのうち 1 件を削除すると「2 件を選択中」になる(Review Focus 5)。
- 「すべて選択」で全件が選ばれ、もう一度で外れる。
- `is:deleted` で、行のチェックと「…」が押せない(Review Focus 4)。
- 行の「…」で 02 のアイテムのメニューが開き、「詳細を開く」で詳細パネルが開く。「…」の「削除…」で削除すると、その行が消える(Store の変更の知らせで読み直す)。
- 閲覧のみの Store の行だけを選ぶと、「一括変更…」「一括削除…」が無効。

- [ ] **Step 5: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/search-table.spec.ts`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add web
git commit -m "feat(web): 検索のテーブルに行の選択・選択バー・一括変更と一括削除・行末のメニューを足す(E2a Task 9)"
```

---

### Task 10: パレットの「{N} 件をまとめて変更…」

**Files:**
- Create: `web/packages/core/src/features/search/changeAll.ts`
- Modify: `web/packages/core/src/features/palette/queryGroup.ts`(応答から件数・`is:deleted`・すべて閲覧のみを控え、2 つ目の候補を出す)、`web/packages/core/src/features/palette/palette.ts`(deps に `bulkChangeAll(query: string): Promise<void>` を足し、群に渡すだけ)、`web/packages/core/src/index.ts`、`web/app/src/palette.ts`、`web/app/src/search.ts`(`bulkChangeAll` の組み立て)、`web/e2e/search-table.spec.ts`
- Test: `web/packages/core/src/features/search/changeAll.test.ts`、`web/packages/core/src/features/palette/queryGroup.test.ts`(なければ `palette.test.ts` の「このクエリで」のテストのファイル)、`web/e2e/search-table.spec.ts`

**Interfaces:**
- Consumes: Task 6 の `settled`、Task 9 の `searchSelection`、Task 5 の `selectable`・`RowSelection.set`
- Produces:
  - `function createChangeAll(deps: { openSearch(query: string): Promise<void>; settled(): Promise<void>; result(): SearchResult | null; selection: Pick<RowSelection, "set">; openBulkChange(): Promise<void> }): (query: string) => Promise<void>`(core。E2a-19)
  - `export const bulkChangeAll: (query: string) => Promise<void>`(`web/app/src/search.ts`。`openSearch` はパレットの「検索で開く」と同じもの、`openBulkChange` は `bulkChange.open(searchSelection, { after: () => search.reload() })`)

- [ ] **Step 1: 失敗するテストを書く**

`changeAll.test.ts`:
- `openSearch("kind:bug")` → `settled` → `selection.set(selectable(rows))` → `openBulkChange` の順に 1 回ずつ呼ぶ。
- `settled` のあとの結果が 0 件・エラーなら `set([])` だけで、`openBulkChange` を呼ばない。

`queryGroup.test.ts`:
- `kind:bug`(3 件、閲覧のみでない)で、「検索で開く」の次に「3 件をまとめて変更…」。選ぶとパレットを閉じて `bulkChangeAll("kind:bug")`。
- 0 件 →「0 件をまとめて変更…」が無効で理由「一致するアイテムがありません」。
- 行がすべて閲覧のみ → 理由「閲覧のみの Store です」。
- `bulk change is disabled for is:deleted`(Review Focus 4): `is:deleted kind:bug`(応答の `deleted` が真)→ 理由「削除済みのアイテムは変更できません」(0 件でもこの理由)。
- キーなし・エラーでは、どちらの候補も出さない(E1 のまま)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- changeAll palette queryGroup`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: 通しのテストを足す**

`search-table.spec.ts` に足す: マイタスクなど検索でない画面でパレットを開き、`kind:bug` を打って「{N} 件をまとめて変更…」を選ぶと、検索の画面の欄が `kind:bug` で、「{N} 件を選択中」になり、一括変更のダイアログが開いている。

- [ ] **Step 5: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/search-table.spec.ts`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add web
git commit -m "feat(palette): 「このクエリで」に「{N} 件をまとめて変更…」を足す(E2a Task 10)"
```

---

### Task 11: 文書と、すべての確かめ

**Files:**
- Modify: `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`(下記)、`docs/superpowers/plans/2026-11-15-hedgehug-track-e1.md`(末尾「E2(テーブル)」の各行に「E2a で足した」または「E2b・E2c に回した」を添える)、`docs/superpowers/plans/2026-11-14-hedgehug-track-f2b1.md`・`2026-11-14-hedgehug-track-f2b2.md`(末尾「E に求めること」の各行に「E2a で足した」を添える)、この計画の末尾「E2b・E2c に渡すもの」

- [ ] **Step 1: 文書を直す**

- トラック E の分け方(`2026-11-15-hedgehug-track-e.md`): 決めたことの表に「E-5 E2 は 3 本に分ける(E2a・E2b・E2c)。ユーザーの決定(2026-11-15)」と「E-6 ワークスペースの画面の「ツリー」タブ(プロジェクトの行・プロジェクトのメニュー)は E7。ユーザーの決定(2026-11-15)」を足す。小計画の一覧の E2 の行を E2a(この計画の Goal の中身)・E2b(04「セル」の編集の部品、閲覧のみ・削除済みの行のセルの無効)・E2c(ワークスペース・プロジェクトの「テーブル」タブ、待つものは E2a)の 3 行にし、E3 の待つものを E2a にする。E7 の行の中身に「ワークスペースの画面の「ツリー」タブ・プロジェクトの行・プロジェクトのメニュー(A4b の計画の E に回した行)」を足す。「ほかのトラックから回された項目の行き先」の E2 を E2a・E2c に直す(A2 の決定 15 のテーブル → E2c、F2b1・F2b2 → E2a)。
- この計画の末尾「E2b・E2c に渡すもの」を、実装で変えたことに合わせて直す。

- [ ] **Step 2: すべてを確かめる**

Run: `make check && make e2e-bin && pnpm -C web exec playwright test e2e/search-table.spec.ts e2e/search.spec.ts`
Expected: PASS

- [ ] **Step 3: コミット**

```bash
git add docs
git commit -m "docs: トラック E の分け方を E2a・E2b・E2c と E7 のワークスペースのツリーに書き換える(E2a Task 11)"
```

## E2b・E2c に渡すもの

### E2b(セルの編集)

- セルは `web/app/src/screens/table/TableCell.svelte`(と `TableRow.svelte` のステータス・担当者)が `cellView` で読むだけで描いている。E2b は、04 の 693〜713 行の編集の部品に置き換える。保存は詳細パネルと同じ口(`itemsPort.setProperty`・`addValue`・`removeValue`、スプリントは計画の口)を使い、アクティビティ・割り当ての通知・自動化・02 の規則はサーバーの既存の処理に任せる。
- 選択肢(ステータス・優先度・単一選択・複数選択・関連システム・メンバー・スプリント・リリース)は、セルを押したときにそのアイテムのワークスペースの設定を読む(テーブルは複数のワークスペースにまたがる)。
- 列の型(`SearchColumn.type`)とアイテムのワークスペースのプロパティの型(`custom[鍵].type`)が違うセルの編集をどうするかは、E2b で決める(E2a はアイテム自身の型で表示している。E2a-9)。
- 閲覧のみの Store の行と削除済みの行のセルを無効にし、閲覧のみは「閲覧のみの Store です」と添える(04 の 482・312 行)。
- `unused` の列は「—」のまま編集できない(03 の 246 行)。

### E2c(ワークスペース・プロジェクトの「テーブル」タブ)

- テーブルの部品は `web/app/src/screens/table/` にある(`ItemTable`・`TableToolbar` など)。検索の画面と同じものを使う。
- 検索の API(`POST /api/search`)に、範囲(Store とワークスペース、またはプロジェクト)を足す。対象のワークスペース(列の決め方。E2a-2)も、ワークスペースの画面はそのワークスペース、プロジェクトの画面はそのプロジェクトのワークスペースにする(04 の 660 行)。
- 画面の状態は、ワークスペースごと・プロジェクトごと・タブごとに別の鍵にする(04 の 474 行)。E1 の鍵の規則は `^[a-z0-9:_/-]{1,200}$` で、内部 ID(大文字を含む)をそのまま入れられない。鍵の作り方か規則のどちらかを E2c で決める。
- 選択は画面ごとに持ち、別の画面に移ると解除する(04 の 822 行)。タブを移ったときも解除する。
- 空のとき(04 の 719〜721 行)の「作成」は、そのワークスペース・プロジェクトを初めの値にして作成ダイアログを開く。
