# hedgehug トラック E1(クエリ言語と検索の画面)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 04 のクエリ言語(解析・照合・並べ替え・エラー・候補・検索ヘルプ)と検索の画面を作る。検索の画面は、絞り込み欄・件数・閲覧のみの帯と、テーブルの初めの 7 列を読むだけで出す。画面の状態(絞り込み)を state.db に残し、サイドバーのナビ「検索」とパレットの「検索を開く」「このクエリで」の「検索で開く」から開けるようにする。

**Architecture:** クエリの解析・照合・並べ替えは入出力のない `internal/domain/query` が 1 か所で決める(画面は解析しない。候補のためにカーソルの位置のキーだけを読む)。`internal/app/search` は表示中の Store をその都度読み(パレットと同じ。トラック E の決定 E-2)、アイテムごとに照合の材料 `query.Facts` を作って `query` に渡す。画面の状態は `infra/statedb` の新しい表 `screen_states` に、画面ごとの JSON の文字列で置く(E2〜E7 も同じ表を使う)。API は `internal/adapter/httpapi/search`。画面は core の `features/search/`(検索の流れ・カーソルのキー・候補)と `features/screenstate/`、ui の `patterns/FilterInput.svelte`、app の `screens/search/`。

**Tech Stack:** Go 1.27.3、huma v2、modernc.org/sqlite(入っている)。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`: 「集計の定義」(18〜40 行)、「クエリ言語」の全節(160〜352 行)、「検索」(354〜360 行)、「画面の構成」の絞り込み欄・ツールバー・エラー・人の表示(447〜453 行)、件数(455 行)、「Store をまたぐ画面の同じ ID」(457〜459 行)、「閲覧のみの Store のアイテム」の帯(487 行)、「テーブル」の「列」の初めの列(664 行)・「空のとき」(719〜721 行)、「モックからの変更点」の 979〜993・1049・1050・1052・1068・1069・1078〜1083・1085・1089〜1092 行
- 01: 「画面の移り方」(149 行)、「画面のアドレス」(157〜180 行)、「キーボード」の `/`・Esc・Enter(213〜216 行)、「Store で共有するものと、マシンごとに持つもの」の画面の状態(75 行)
- 02: 「コマンドパレット」の候補の表のコマンド・「このクエリで」(621・631 行)、実行の表(664 行の前後)、「モックからの変更点」772 行
- 08: 「削除済み」の「一覧」(305〜320 行。`is:deleted` の対象)、「検出する項目」(403〜407 行。`is:invalid` の対象)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` の「クエリのキー」の行(328 行。`created`・`updated` の定義)
- `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`(トラック E の分け方と、回された項目の行き先)

## 始める前に

- トラック E は 7 本に分けた(ユーザーの決定、2026-11-15。`2026-11-15-hedgehug-track-e.md`)。**E1 = この計画**。1 計画・1 PR、ブランチ `feat/track-e`。
- 照合は Store をその都度読む(ユーザーの決定、2026-11-15。決定 E-2)。設計書の書き換えは Task 14 に入れた。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。いまの数: `internal/app/port` は 20 個で満杯(画面の状態の口は `app/search` に置く。F3 の `appreports.Selections` と同じ)、`domain/planning` 14 個、`domain/history` 6 個、`domain/change` 8 個、`domain/mention` 1 個、`infra/statedb` 11 個。`web/packages/core/src/features/palette/palette.ts` は 259 行(「このクエリで」の群は新しい `queryGroup.ts` に置く)、`web/app/src/shell/Sidebar.svelte` 146 行、`web/app/src/App.svelte` 123 行。`web/app/src/wire.ts` は満杯なので、検索の組み立ては新しい `web/app/src/search.ts` に置く(`inbox.ts` と同じ形)。
- 画面のパッケージの依存の向き(`web/.dependency-cruiser.cjs`): ui は Svelte だけを使い、core を import しない。ui の部品の props の型は ui の中で宣言する。
- app の機能どうしは import しない(設計書 2 節)。`app/search` は domain と `app/port` だけを使う。読み方の前例は `internal/app/palette/read.go`(写してよい)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書き、言い換えない。閲覧のみの帯は ui の `ReadOnlyBand`、添え書きは core の `readOnlyNote`。メンバーにいない人は「不明なユーザー」。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする(CI が差分を見る)。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| E1-1 | クエリを解析して一致・エラー・並びを決めるのはサーバーの `domain/query` だけにする。画面は、候補のためにカーソルの位置の語のキーと値を読むだけ(`caret.ts`)。両方とも、04 の 169〜185 行の例(引用符・閉じていない引用符・`-` だけの語・`-sort:`・`http://`・`v2<v3`・`C++>11`・`status:,`)を同じ表でテストする | 判定を 2 か所に持つと食い違うため。04 の 162 行(書き方は暫定で、見直す前提) |
| E1-2 | 語の切り方: 空白で区切る。`"` で囲んだあいだの空白は区切らず、`"` は値から除く。閉じていない `"` から末尾までは 1 つの語。語の先頭の `-` は否定(`-` だけの語は自由な語「-」)。`-` を除いた語が `^[A-Za-z][A-Za-z0-9_]*(:|<=|>=|<|>)` で始まればキー(小文字にそろえる)、ほかは自由な語。値は `,` で区切り(引用符の中でも区切る)、前後の空白を除いた空の値は捨てる | 04 の 171〜177・185 行 |
| E1-3 | エラーは語ごとに、(1) キーが組み込みにも、このマシンのどのワークスペース(非表示の Store を含む)の追加プロパティにもない →「「{キー}」というキーはありません」、(2) 値が 1 つもない →「「{書いた語}」に値がありません」(語は `-` を含めて書いたとおり)、(3) `priority`・`created`・`updated` に `none` →「{キー} に none は使えません」の順で最初の 1 つ。複数の語のエラーは書いた順に 1 行ずつ | 04 の 296〜304 行 |
| E1-4 | 組み込みのキーは `status`・`category`・`kind`・`system`・`assignee`・`watching`・`mentions`・`priority`・`sprint`・`release`・`project`・`parent`・`epic`・関連の 9 個・`workspace`・`store`・`is`・`has`・`points`・`estimate`・`start`・`end`・`created`・`updated`・`sort`・`group`。追加プロパティが組み込みと同じ綴りなら組み込みの意味 | 04 の 189〜214・229 行 |
| E1-5 | 比べ方: 名前・ログイン名・ID は大文字と小文字を区別しない完全一致(自由な語と、テキスト・URL の値だけ部分一致)。ログイン名の値の先頭の `@` は除く。`@me` はアイテムの Store の「あなた」(`port.Identity.Me`。読めない・空なら何にも一致しない)。否定は語の結果を反転する(値のないアイテム・キーを持たないワークスペースのアイテム・読めない日付と数は、否定で一致する側に入る) | 04 の 173・174・216・218・244 行 |
| E1-6 | ID のキー(`project`・`parent`・`epic`・関連の 9 個)は、相手のいまの ID か旧 ID(移動前の ID、旧い接頭辞の ID)のどれかと等しければ一致。自由な語は、いまの ID・旧 ID・タイトル・説明のどれかに部分一致すれば一致し、いまの ID・タイトル・説明のどれにも一致せず旧 ID だけで一致した語があれば、その行に「旧 ID」と添える(`oldIdHit`) | 04 の 183・201〜203・459 行、02 の旧 ID の規則 |
| E1-7 | `is:invalid` は、08「検出する項目」のアイテムを指す 12 項目(親がない・種類に合わない親子・親の循環・ステータスがない・存在しない値・存在しない人・存在しないスプリント・リリース・宙づりの関連・親が別のワークスペース・プロジェクトにある・日付の逆転・番号の重なり・Issue の二重リンク)の判定を、E1 で `domain/repair` の 1 つの関数 `repair.ItemProblems` として作り、`query.Facts.Invalid` を埋める。G の診断は同じ関数を使う。調べるのは削除されていないアイテム(番号の重なりだけは削除済みも数える)。番号の重なりは、同じワークスペースで同じ番号を持つ削除されていないアイテムのどれにも付ける(どちらが番号を持ち続けるかは remote の情報が要り、修復のときに決める) | ユーザーの決定(2026-11-15。トラック E の決定 E-4)。08 の 403〜407 行。検索と診断で、1 つのアイテムが無効かどうかが食い違わないため(04 の 1080 行) |
| E1-8 | `is:deleted`(否定でない語が 1 つでもある)のとき、対象は削除済みのアイテムのうち、ワークスペースが削除されていないもの(08 の一覧に行として出るもの)だけにする。行には「削除済み」のピルと「あと N 日で完全に削除」(N = 削除の日 + 保持日数 − 今日。0 以下なら「あと 0 日」ではなく「保持日数を過ぎています」。08 の一覧と同じ)を出し、行を薄くする。タイトルは押せない文字で出す。タイトルを押して設定の「削除済み」の行を開くのは、その画面を作る G が足す | 04 の 306〜312 行、08 の 307〜320 行 |
| E1-9 | `sort:` は E1 の行の並びに効かせる(テーブルの並び)。`group:` は解析だけして応答の `group` に返し、グループ分けは E2 が作る | 04 の 248・272 行。テーブルのグループは E2 |
| E1-10 | 並び: `sort:` があれば全体を 1 本にしてそのキーで並べ、値が同じなら Store の並び・ワークスペースの並び・アイテムの並び順(`item.SortSiblings` と同じ比べ方)の順。`sort:` がなければ Store の並び、ワークスペースの並び、並び順。Store の並びは `port.StoreList` の順、ワークスペースの並びはサイドバーと同じ(`workspace.RankOf` の順、同じなら内部 ID) | 04 の 268 行、`app/workspaces/order.go` の `rankedInOrder` |
| E1-11 | 今日の日付は、サーバーの `port.Clock` の時刻をこのマシンの時刻帯にした日(`planning.Today`)。`created`・`updated` も、このマシンの時刻帯の日付で比べる | 01 の日付の規則、04 の 211・244 行 |
| E1-12 | `updated` は `history.Updated(log, ctx)`: 履歴に出す行のうち実行者が人か自動化の、いちばん新しい行の時刻(作成を含む)。GitHub・同期の行と、履歴に出ない行(順位の変更など)は使わない | storage-design 328 行、04 の 211 行 |
| E1-13 | `sprint:next` は、計画中(開始していない、完了していない、削除されていない)のスプリントのうち、開始日が最も早いもの。開始日のないものは最後、同じなら `planning.ScreenOrder` の順。新しい関数 `planning.NextSprint` に置く | 04 の 199 行 |
| E1-14 | `mentions:` は、説明と、削除されていないコメントの本文に `mention.Has(body, login)` が真のもの(`@me` は Store の「あなた」)。`none` は説明にもコメントにも `@` のメンションがないもの(`mention` の抽出で 1 人もいない) | 04 の 197 行 |
| E1-15 | 説明とコメントは、クエリに自由な語か `mentions:` があるときだけ読む(`Query.NeedsText`)。`is:overwritten` があるときだけ受信箱を読む(`Query.NeedsInbox`)。`is:invalid` があるときだけ診断の判定をする(`Query.NeedsInvalid`) | 1 回の検索で全アイテムの説明を読まないため(Store をその都度読むので) |
| E1-16 | 画面の状態は state.db の表 `screen_states(screen TEXT PRIMARY KEY, body TEXT NOT NULL)` に、画面ごとの JSON の文字列で置く。サーバーは中身を見ない(64 KiB まで)。検索の画面の鍵は `search`、中身は `{"query": "…"}`。画面はブラウザのタブで最初に読んだ値を覚え、以後はタブの中の値を使い、変えるたびに書く | 01 の 75・178 行。E2〜E7 の画面も同じ表を使う |
| E1-17 | 画面のアドレスは `/search`。サイドバーのナビは「受信箱」の下に「検索」を置く(マイタスク・アクティビティは E4・C4d が上の位置に足す)。いまの画面なら強調する | 01 の 163 行、04 の 57〜64 行 |
| E1-18 | 検索の画面の本体は、テーブルの初めの 7 列(ID・タイトル・ステータス・担当者・優先度・終了日・ポイント)を読むだけで出す。ID の前に Store の点(Store の色)をいつも出す。タイトルは押すと詳細パネルを開く。ステータスは色の点と名前、担当者はアバターと表示名(メンバーにいなければ「不明なユーザー」)、終了日は `dueLabel`。列の操作・セルの編集・選択・行末のメニュー・グループは E2 | 04 の 459・664 行、E の分け方 |
| E1-19 | 絞り込み欄は入力のたびに 150ms 待ってから検索し、古い応答は捨てる。Store が変わった知らせ(`reloads`)で読み直す | 04 の 449 行(入力に合わせて絞り込む) |
| E1-20 | 候補(04 の 278〜292 行)の元は、表示中の Store のカタログ(プロジェクト・ワークスペース)、パレットの索引(アイテム)、Store ごとのメンバー(`GET /api/stores/{store}/members`、最初に `assignee:` を打ったときに読む)。範囲を引数で受け取る形にし、E1 では「表示中のすべての Store」だけを使う。打った値は、カーソルの前の最後の `,` からカーソルまで。選ぶと、その部分を入れるものに置き換える | 04 の 280〜292 行。E4・E7 がワークスペース・プロジェクトの範囲で使う |
| E1-21 | パレット: コマンドに「検索を開く」を「受信箱を開く」の次に足す。「このクエリで」の群は、入力を 150ms 待ってサーバーで解析し、キーが 1 つ以上あってエラーがないときだけ「検索で開く」(ヒント「{N} 件」)を出す。選ぶと、検索の絞り込みを入力にして `/search` を開く(テーブルの設定は変えない)。「ビューとして保存…」は E3、「{N} 件をまとめて変更…」は E2 が同じ群に足す | 02 の 621・631・664 行、04 の 359 行 |
| E1-22 | `store:旧名` の書き換え(Store の名前の変更。08)と、診断の「検索で開く」(08)は、その操作を作る G が、検索の画面の状態(`screen_states` の `search`)を書き換える・開く | 行き先を作るトラックがつなぐ規則 |
| E1-23 | 日付が変わったときに `today`・`-7d` のクエリを計算し直すのは E4(04 の 42 行の仕組みを作る小計画)。E1 は検索を出すたびにその時点の今日で計算する | トラック E の分け方 |
| E1-24 | API: `POST /api/search`(本文 `{query}`)→ `{errors: string[], hasKey: boolean, group: string, deleted: boolean, rows: SearchRow[]}`。件数は `rows` の数。Store をまたぐので `/api/stores/{store}` の下に置かない。画面の状態は `GET /api/screen-states/{screen}`(`{body: string|null}`)と `PUT /api/screen-states/{screen}`(本文 `{body}`)。`screen` は `^[a-z0-9:_/-]{1,200}$` | F2b1-15 と同じ置き方 |

## Review Focus

1. **否定の語で、値のないアイテムが消える。** `-assignee:@me` で担当者のいないアイテムも残る。`-points>3` でポイントのないアイテムも残る(E1-5)。Task 3 の `TestMatchNegationKeepsEmpty` に入れる。
2. **旧い接頭辞をあとから別のワークスペースが使うと、片方しか出ない。** ワークスペース A が接頭辞 HH から WEB に変え、ワークスペース B が HH を使うとき、`HH-1` は B の HH-1 と A の WEB-1(「旧 ID」付き)の両方に一致する(E1-6)。Task 2 の `TestFreeWordOldPrefixBothMatch` と Task 6 の `TestSearchOldPrefixReusedByAnother` に入れる。
3. **閉じていない引用符や `-` だけの語で、解析が崩れる・エラーになる。** `release:"v2.0 beta` は値「v2.0 beta」の 1 語、`- foo` は自由な語「-」と「foo」(E1-2)。Task 1 の表に入れ、Task 9 の `caret.test.ts` でも同じ表を使う。
4. **非表示の Store にしかないプロパティのキーが「キーはありません」になる。** 非表示の Store のワークスペースにだけある `customer` は、エラーにならず、表示中の Store のアイテムには一致しない(04 の 304 行)。Task 7 の `TestSearchKeyOfHiddenStoreIsKnown` に入れる。
5. **候補を選ぶと、カンマの前の値や後ろの語まで消える。** `assignee:tanaka,すず kind:bug` の「すず」のあとで候補「鈴木 @suzuki」を選ぶと `assignee:tanaka,suzuki kind:bug` になる(E1-20)。Task 10 の `candidates.test.ts` の `replaces only the typed part` に入れる。

---

### Task 1: クエリの語の切り方と解析・エラー

**Files:**
- Create: `internal/domain/query/doc.go`(パッケージの説明だけ)、`internal/domain/query/parse.go`、`internal/domain/query/keys.go`
- Test: `internal/domain/query/parse_test.go`

**Interfaces:**
- Produces:
  - `type Op string`(`OpEq ":"`・`OpLt "<"`・`OpLe "<="`・`OpGt ">"`・`OpGe ">="`)
  - `type Term struct { Raw string; Not bool; Key string /* 小文字。自由な語は "" */; Op Op; Values []string }`
  - `type Sort struct { Key string; Desc bool }`
  - `type Query struct { Terms []Term /* sort・group を除く */; Sort *Sort; Group string; Deleted bool; HasKey bool; NeedsText bool; NeedsInbox bool; NeedsInvalid bool }`
  - `func Parse(text string, custom func(key string) bool) (Query, []string)` — 2 つ目はエラーの文(書いた順)。`custom` は小文字のキーが、どれかのワークスペースの追加プロパティにあるか
  - `func Builtin(key string) bool`(E1-4 の組み込み。`keys.go`)

- [ ] **Step 1: 失敗するテストを書く**

表のテスト `TestParse` に、少なくとも次の行を入れる(期待は `Terms` の `Key`・`Not`・`Op`・`Values` と、エラー):

| 入力 | 期待 |
|---|---|
| `status:InProgress kind:story,bug` | 2 語。`status` `[InProgress]`、`kind` `[story bug]` |
| `Status:done` | キー `status` |
| `-assignee:@me` | `Not`、キー `assignee`、値 `[@me]` |
| `release:"v2.0 beta"` | 値 `[v2.0 beta]` |
| `release:"v2.0 beta` | 値 `[v2.0 beta]`(閉じていない) |
| `"ガント 表"` | 自由な語 `[ガント 表]` |
| `- foo` | 自由な語 `-` と `foo`(`-` は否定にしない) |
| `-sort:priority` | `Sort{priority, false}` |
| `sort:-priority sort:id` | `Sort{priority, true}`(最初だけ) |
| `group:kind,status -group:-x` | `Group` は `kind` |
| `points>=3 end<today` | `OpGe` `[3]`、`OpLt` `[today]` |
| `http://x` | エラー「「http」というキーはありません」 |
| `v2<v3` | エラー「「v2」というキーはありません」 |
| `C++>11` | 自由な語 `[C++>11]`、エラーなし |
| `status:` / `status:,` / `-status:` | エラー「「status:」に値がありません」/「「status:,」に値がありません」/「「-status:」に値がありません」 |
| `priority:none` / `created:none` / `-updated:none` | 「priority に none は使えません」/「created に none は使えません」/「updated に none は使えません」 |
| `customer:x`(`custom` が真) / `Customer:x`(偽) | エラーなし / 「「customer」というキーはありません」 |
| `is:deleted,blocked` / `-is:deleted` | `Deleted` 真 / 偽 |
| `foo` / `foo sort:id` | `HasKey` 偽 / 真 |
| `foo` / `mentions:a` / `kind:bug` | `NeedsText` 真 / 真 / 偽 |
| `is:overwritten` / `-is:invalid` | `NeedsInbox` 真 / `NeedsInvalid` 真 |
| `` | 語なし、エラーなし |

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/query/ -run TestParse`
Expected: FAIL(`undefined: Parse`)

- [ ] **Step 3: `Parse` と `Builtin` を書く(E1-2・E1-3・E1-4)**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/query/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/query
git commit -m "feat(query): クエリの語を切り、キーとエラーを決める(E1 Task 1)"
```

---

### Task 2: 照合の材料と、名前・ID・自由な語のキー

**Files:**
- Create: `internal/domain/query/facts.go`、`internal/domain/query/match.go`、`internal/domain/query/match_names.go`
- Test: `internal/domain/query/match_names_test.go`(材料を作る `facts(t, …)` の助けもここに置く)

**Interfaces:**
- Consumes: Task 1 の `Query`・`Term`
- Produces:
  - `type Ref struct { Display string; Aliases []string }` — 相手のいまの ID と旧 ID
  - `type Facts struct` — アイテム 1 件の照合と並べ替えの材料:
    `StoreName string; StoreOrder int; WorkspaceOrder int; Workspace workspace.Workspace; Settings workspace.Settings; Item item.Item /* Blocked・Deleted を含む */; Display string; Aliases []string; Description string; Comments []string; Project Ref; ProjectTitle string; Parent *Ref; Ancestors []Ref; Relations map[string][]Ref /* 関連のキー(blocks など)→ 相手。削除済みの相手を除く */; SprintName string; SprintCurrent bool; SprintNext bool; ReleaseName string; Me string; Names map[string]string /* ログイン名を小文字にしたもの → 表示名 */; HasPR bool; Overwritten bool; Invalid bool; Created time.Time; Updated time.Time`
  - `type Hit struct { Match bool; OldID bool }`
  - `func Match(q Query, f Facts, today string) Hit` — すべての語に一致すれば `Match`。`OldID` は E1-6

- [ ] **Step 1: 失敗するテストを書く**

テストは `Match(mustParse(t, "…"), f, "2026-11-15")`。少なくとも:
- `TestStatusAndCategory`: ステータス「InProgress」(分類 doing)のアイテムに `status:inprogress` 真、`status:Done` 偽、`category:doing` 真、`category:done,canceled` 偽。
- `TestKindSystemPriorityWorkspaceStore`: `kind:Story`、`system:api`(関連システム 2 つのうち 1 つ)、`system:none`(関連システムなし)、`priority:HIGH`、`workspace:web`(名前)、`workspace:hh`(接頭辞)、`store:Team`。
- `TestAssigneeWatching`: `assignee:@Tanaka`、`assignee:@me`(`Me` が tanaka)、`Me` が空なら `assignee:@me` 偽、`assignee:none`、`watching:none`、`watching:suzuki`。
- `TestIDKeys`: `project:hh-p1`、旧 ID `project:OLD-P1`、`parent:HH-1`(祖父母も)、`epic:none`(親なし)、`blocked_by:HH-3`、`relates_to:HH-3`(どちらの向きでも)、`blocks:none`。
- `TestFreeWord`: `HH-1` が `HH-12` に部分一致、タイトル・説明の部分一致(大文字と小文字を区別しない)、`"ガント 表"`。
- `TestFreeWordOldPrefixBothMatch`(Review Focus 2): いまの ID `WEB-1`・旧 ID `HH-1` のアイテムに `HH-1` は `{Match: true, OldID: true}`、いまの ID `HH-1` のアイテムには `{true, false}`。
- `TestIsAndHasUnknown`: `is:foo` 偽、`-is:foo` 真、`has:foo` 偽。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/query/ -run 'Test(Status|Kind|Assignee|IDKeys|FreeWord|IsAndHas)'`
Expected: FAIL

- [ ] **Step 3: `Facts`・`Match` と、名前・ID・自由な語のキーを書く(E1-5・E1-6)**

キーごとの判定は `match_names.go` に置き、`Match` は語の結果を `Not` で反転して AND する。名前・ID の比較は `strings.EqualFold`、部分一致は両方を `strings.ToLower` して `strings.Contains`。Task 3 のキーは、まだ `false` を返す分岐にしておく。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/query/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/query
git commit -m "feat(query): 名前・ID・自由な語のキーで照合する(E1 Task 2)"
```

---

### Task 3: 数・日付・状態・追加プロパティのキー

**Files:**
- Create: `internal/domain/query/dates.go`、`internal/domain/query/match_values.go`
- Modify: `internal/domain/mention/mention.go`(`Any`)
- Test: `internal/domain/query/match_values_test.go`、`internal/domain/mention/mention_test.go`

**Interfaces:**
- Consumes: Task 2 の `Facts`・`Match`
- Produces: `func ParseDate(v, today string) (string, bool)` — `2026-12-01`・`today`・`+3d`・`3d`・`-7d` を `YYYY-MM-DD` に。読めなければ false

- [ ] **Step 1: 失敗するテストを書く**

- `TestParseDate`: 今日 `2026-11-15` で `today`→`2026-11-15`、`+3d`・`3d`→`2026-11-18`、`-7d`→`2026-11-08`、`2026-12-01`→同じ、`abc`・`2026-13-01`・`d`→false。
- `TestCompare`: `points>=3`(3 で真・2.5 で偽・なしで偽)、`points:none`、`estimate<=2`、`end<today`(終了日なしは偽)、`start>-7d`、`end:abc`(何にも一致しない)、`kind>story`(`:` と同じ)、`created:today`・`updated>-7d`(時刻をこのマシンの時刻帯の日にして比べる)。
- `TestMatchNegationKeepsEmpty`(Review Focus 1): 担当者なしに `-assignee:@me` 真、ポイントなしに `-points>3` 真、`-end:abc` 真。
- `TestSprintRelease`: `sprint:current`(`SprintCurrent`)、`sprint:next`、`sprint:"Sprint 3"`、`sprint:none`、`release:v2.0.0`、`release:none`。
- `TestIsHas`: `is:blocked`(`Item.Blocked`)、`is:overdue`(未完了で終了日が今日より前。今日は偽。完了は偽)、`is:overwritten`、`is:invalid`(`Invalid`)、`has:end`、`has:sprint`、`has:release`、`has:pr`、チェックのプロパティはオンだけ `has:` 真。
- `TestMentions`: 説明の `@tanaka`、削除されていないコメントの `@me`(`Me` が suzuki で本文に `@suzuki`)、`mentions:none`(どこにもメンションなし。`mail@example` だけの本文はメンションありとみなす)。
- `internal/domain/mention/mention_test.go` の `TestAny`: `@hanako 見て`・`cc:@Hanako`・`mail@hanako`・`` `@hanako` ``(コードの中)は真、`hanako`・`@`・`@ 見て`・`@。` は偽。
- `TestCustomProperties`: 単一選択・複数選択(選択肢の名前、`none`)、数値(比較)、日付(比較)、ユーザー(`@me`)、チェック(`true`、`false` はオフと未設定の両方)、テキスト・URL(部分一致、`none`)。キーを持たないワークスペースのアイテムは偽、否定で真。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/query/ ./internal/domain/mention/ -run 'Test(ParseDate|Compare|MatchNegation|SprintRelease|IsHas|Mentions|CustomProperties|Any)'`
Expected: FAIL

- [ ] **Step 3: `ParseDate` と、Task 2 で残したキーの判定を書く**

追加プロパティの値は `Item.Choices`(選択式)と `Item.Fields`(ほかの型。ログの `set` の値のまま)から読む。メンションは `mention.Has`。`mentions:none` のために、`internal/domain/mention/mention.go` に `func Any(body string) bool`(`@` の直後に `member.ValidLogin` に合う 1 文字以上の並びがある。`@` の前の文字とコードの中かは問わない。`Has` と同じ読み方)を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/query/ ./internal/domain/mention/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/query internal/domain/mention
git commit -m "feat(query): 数・日付・状態・追加プロパティのキーで照合する(E1 Task 3)"
```

---

### Task 4: 並べ替え

**Files:**
- Create: `internal/domain/query/sort.go`
- Test: `internal/domain/query/sort_test.go`

**Interfaces:**
- Consumes: Task 1 の `Sort`、Task 2 の `Facts`
- Produces: `func Order[T any](rows []T, facts func(T) *Facts, s *Sort)` — その場で並べ替える(E1-10。`app/search` は Store の情報や旧 ID の印を持つ行をそのまま並べる)。表にないキーは `s` が nil と同じ

- [ ] **Step 1: 失敗するテストを書く**

- `TestOrderDefault`: Store の順 → ワークスペースの順 → 並び順(`Rank` → 作成の時刻 → 内部 ID)。
- `TestOrderKeys`: `id`(番号)、`title`(`cmp.Compare` の文字列の順。名前の並べ方は `planning.ScreenOrder` と同じ)、`kind`(Epic・Story・Task・Bug・Step)、`status`・`priority`(ワークスペースの定義の順)、`assignee`(表示名の順)、`project`(プロジェクトのタイトル)、`parent`(親の ID)、`sprint`・`release`(名前)、`start`・`end`・`points`・`estimate`、`created`・`updated`(古いものが先)、降順。
- `TestOrderEmptyLast`: 値のないアイテムは昇順でも降順でも最後。
- `TestOrderTieByPlace`: 値が同じものは Store の順・ワークスペースの順・並び順。
- `TestOrderSystemsAcrossWorkspaces`: 関連システムは、各アイテムの値をワークスペースの定義の順に並べ直して先頭から比べ、値の数が少ない方が先。2 つのワークスペースで同じ名前は 1 つにし、(Store、ワークスペース)の順に、最初に現れた位置で順を決める(04 の 266 行の例を作る)。
- `TestOrderUnknownKey`: `sort:foo` は既定の並び。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/query/ -run TestOrder`
Expected: FAIL

- [ ] **Step 3: `Order` を書く**

選択肢の順を使うもの(ステータス・優先度・関連システム・単一選択・複数選択)は、`rows` に出てくるワークスペースを (StoreOrder, WorkspaceOrder) の順に見て、名前(小文字)→ 順位の表を先に作る。追加プロパティの並びは `sort:<キー>`(組み込みと同じ綴りは組み込み)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/query/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/query
git commit -m "feat(query): 並べ替えのキーと既定の並びを決める(E1 Task 4)"
```

---

### Task 5: 最後に変わった時刻と、次のスプリント

**Files:**
- Create: `internal/domain/history/updated.go`、`internal/domain/planning/next.go`
- Modify: `internal/domain/change/deletion.go`(`DaysLeft`)
- Test: `internal/domain/history/updated_test.go`、`internal/domain/planning/next_test.go`、`internal/domain/change/deletion_test.go`

**Interfaces:**
- Produces:
  - `func Updated(log []change.Record, c Context) time.Time` — E1-12。履歴の行を作る仕組み(`ItemRows`)と同じ規則で行に出る記録だけを見る
  - `func NextSprint(live []Sprint) *Sprint` — E1-13。なければ nil
  - `func (d Deletion) DaysLeft(retention int, today string) int` — 削除した日(このマシンの時刻帯)+ 保持日数 − 今日の日数。0 以下なら保持日数を過ぎている(E1-8。G の設定の「削除済み」も同じ関数を使う)

- [ ] **Step 1: 失敗するテストを書く**

- `TestUpdatedSkipsGitHubSyncAndRank`: 作成(人)→ 人のステータスの変更 → 自動化の変更 → GitHub のリンクの値の変更 → 同期の修復 → 順位の変更(`rank`)の順のログで、自動化の変更の時刻。
- `TestUpdatedCreateOnly`: 作成だけなら作成の時刻。
- `TestUpdatedHiddenValueDeletion`: `HiddenOps` に入る操作の行(値の削除で書いた行)は使わない。
- `TestDaysLeft`: 11/01 に削除・保持日数 30・今日 11/15 → 16。10/01 に削除 → -15。
- `TestNextSprint`: 計画中の 3 つ(開始日 12/01・なし・11/20)→ 11/20。実行中・完了・削除済みは選ばない。同じ開始日は `ScreenOrder` の順。計画中がなければ nil。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/history/ ./internal/domain/planning/ ./internal/domain/change/ -run 'TestUpdated|TestNextSprint|TestDaysLeft'`
Expected: FAIL

- [ ] **Step 3: `Updated`・`NextSprint`・`DaysLeft` を書く**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/history/ ./internal/domain/planning/ ./internal/domain/change/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/history internal/domain/planning internal/domain/change
git commit -m "feat: 最後に変わった時刻・次のスプリント・削除済みの残り日数を求める(E1 Task 5)"
```

---

### Task 5a: 診断の判定(親子・番号・Issue)

**Files:**
- Create: `internal/domain/repair/problems.go`(型と `ItemProblems`)、`internal/domain/repair/problems_tree.go`
- Test: `internal/domain/repair/problems_test.go`

**Interfaces:**
- Produces:
  - `type Problem string`(`ProblemNoParent`・`ProblemBadParentKind`・`ProblemParentCycle`・`ProblemNoStatus`・`ProblemMissingValue`・`ProblemMissingPerson`・`ProblemMissingPlan`・`ProblemDanglingRelation`・`ProblemParentElsewhere`・`ProblemDateOrder`・`ProblemNumberOverlap`・`ProblemIssueTwice`)
  - `type StoreState struct { Items []item.Item /* 削除済みを含む Store のすべて */; Settings map[workspace.ID]workspace.Settings; Plans map[workspace.ID]planning.Plan /* 削除済みを含む */; Members []member.Member; Issues map[item.ID][]string /* アイテムがリンクしている Issue の鍵(itemlink の Ref.Key) */ }`
  - `func ItemProblems(s StoreState) map[item.ID][]Problem` — 削除されていないアイテムごとの問題(問題のないアイテムは入れない。並びは上の型の順)

- [ ] **Step 1: 失敗するテストを書く**

- `TestNoParent`: 親のない Story、削除済みの親の Task、Store にない親の Step → `ProblemNoParent`。親のない Epic は問題なし。
- `TestBadParentKind`: Story の下の Story、Epic の下の Step、親を持つ Epic(親が Store になくても)→ `ProblemBadParentKind`。
- `TestParentCycle`: A → B → A の 2 件ともに `ProblemParentCycle`。輪の外から輪に入る C は付かない。
- `TestParentElsewhere`: 親が別のワークスペース、または同じワークスペースの別のプロジェクトにある → `ProblemParentElsewhere`。
- `TestNumberOverlap`: 同じワークスペースで番号 5 の削除されていないアイテム 2 件 → 2 件とも。番号 7 の削除されていないアイテムと削除済みのアイテム → 削除されていない方だけ(削除済みは結果に入れない)。別のワークスペースの同じ番号は問題なし。
- `TestIssueTwice`: 1 つの Issue に 2 件がリンク → 2 件とも。1 件が 2 つの Issue にリンク → その 1 件。
- 削除済みのアイテムは、どの問題があっても結果に入れない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/repair/ -run 'Test(NoParent|BadParentKind|ParentCycle|ParentElsewhere|NumberOverlap|IssueTwice)'`
Expected: FAIL

- [ ] **Step 3: 型・`ItemProblems` と、親子・番号・Issue の判定を書く**

種類と親の組は `item.CanPlace`、場所は `Item.Place`。Task 5b の判定は、まだ何も足さない分岐にしておく。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/repair
git commit -m "feat(repair): 診断のアイテムの問題(親子・番号・Issue)を判定する(E1 Task 5a)"
```

---

### Task 5b: 診断の判定(値・人・計画・関連・日付)

**Files:**
- Create: `internal/domain/repair/problems_values.go`
- Test: `internal/domain/repair/problems_values_test.go`

**Interfaces:**
- Consumes: Task 5a の `StoreState`・`Problem`

- [ ] **Step 1: 失敗するテストを書く**

- `TestNoStatus`: ステータスがワークスペースのステータスにない(空を含む)→ `ProblemNoStatus`。
- `TestMissingValue`: 優先度・関連システム・単一選択・複数選択の値がワークスペースの一覧にない → `ProblemMissingValue`。プロパティごと消えた選択式の値も同じ。
- `TestMissingPerson`: 担当者・ユーザー型のプロパティのログイン名がメンバーにない(大文字と小文字を区別しない)→ `ProblemMissingPerson`。
- `TestMissingPlan`: スプリントかリリースがワークスペースの計画にない → `ProblemMissingPlan`。削除済みのスプリントを指すものは問題なし。
- `TestDanglingRelation`: 関連の相手が Store にない → `ProblemDanglingRelation`。相手が削除済みなら問題なし。
- `TestDateOrder`: 終了日が開始日より前 → `ProblemDateOrder`。同じ日・片方だけは問題なし。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/repair/ -run 'Test(NoStatus|MissingValue|MissingPerson|MissingPlan|DanglingRelation|DateOrder)'`
Expected: FAIL

- [ ] **Step 3: 判定を書く**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/repair
git commit -m "feat(repair): 診断のアイテムの問題(値・人・計画・関連・日付)を判定する(E1 Task 5b)"
```

---

### Task 6: Store を読んで照合の材料を作る

**Files:**
- Create: `internal/app/search/service.go`(`Service` と口)、`internal/app/search/read.go`(Store 1 つを読む)、`internal/app/search/facts.go`(アイテム 1 件の `query.Facts`)
- Test: `internal/app/search/facts_test.go`(`apptest` の偽物の Store にアイテムを足して確かめる)

**Interfaces:**
- Consumes: Task 2 の `query.Facts`・`query.Ref`、Task 5 の `history.Updated`・`planning.NextSprint`・`change.Deletion.DaysLeft`、Task 5a の `repair.ItemProblems`・`repair.StoreState`、`port.Stores`・`port.StoreList`・`port.SyncTargets`・`port.Identity`・`port.InboxBook`・`port.Clock`
- Produces:
  - `type Service struct { Stores port.Stores; StoreList port.StoreList; SyncTargets port.SyncTargets; Identity port.Identity; Inbox port.InboxBook; Clock port.Clock }`
  - `type entry struct { info port.StoreInfo; facts query.Facts; daysLeft *int /* 削除済みだけ。change.Deletion.DaysLeft */ }`(パッケージの中だけ)
  - `func (s Service) entries(ctx context.Context, q query.Query) ([]entry, error)` — 表示中の Store を `StoreList` の順に 1 つずつ読む(読み取りを入れ子にしない)。`q.Deleted` なら削除済みのアイテム(ワークスペースが削除されていないもの)、でなければ削除されていないアイテム

- [ ] **Step 1: 失敗するテストを書く**

- `TestFactsBasic`: ステータス・担当者の表示名(`Names`)・プロジェクトの `Ref`(旧 ID を含む)・祖先・関連(削除済みの相手を除く)・スプリントの `SprintCurrent`・`SprintNext`・`HasPR`(PR のリンクだけ。Issue のリンクだけのアイテムは偽)・`Created`・`Updated`・`Me`・`Item.Blocked`。
- `TestFactsTextOnlyWhenNeeded`: `NeedsText` が偽なら `Description`・`Comments` は空、真なら説明と削除されていないコメントの本文。
- `TestFactsInvalid`: `is:invalid` のクエリで、終了日が開始日より前のアイテムだけ `Invalid` が真。`NeedsInvalid` が偽なら判定しない(どれも偽)。
- `TestFactsOverwrittenOnlyWhenNeeded`: 受信箱に未読の「上書き」(`port.NoticeOverwrite`)があるアイテムだけ `Overwritten`。既読のもの・別のアイテムのものは偽。`NeedsInbox` が偽なら受信箱を読まない(偽物の受信箱が呼ばれたら失敗)。
- `TestSearchOldPrefixReusedByAnother`(Review Focus 2): ワークスペース A の接頭辞を HH から WEB に変え、ワークスペース B が HH を使うとき、A のアイテムの `Aliases` に `HH-1` がある。
- `TestEntriesDeleted`: `Deleted` のとき、削除済みのアイテムだけ。ワークスペースごと削除したものは出さない。`daysLeft` は Store の保持日数(`RetentionDays`)で `DaysLeft` を求めたもの。
- `TestEntriesHiddenStoreLeftOut`: 非表示の Store のアイテムは出ない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/search/`
Expected: FAIL

- [ ] **Step 3: `Service`・`entries`・材料の作り方を書く**

ワークスペースは `port.LiveWorkspaces`(B5b の取り決め)、並びは `workspace.RankOf` → 内部 ID(E1-10)。旧 ID は `transfer.Aliases`(パレットの `index.go` と同じ。すべての Store の転送記録を集めてから)。ブロック中は `relation.BlockedSet`(閉じたかは、ブロック元のワークスペースの設定の分類)。PR のリンクは `itemlink.FromLog` の `Links` のうち `Kind` が `itemlink.PR` のもの、コメントは `comment.FromLog`、スプリント・リリースは `planning.FromLog`(ワークスペースのログ)と `planning.ActiveSprint`・`planning.NextSprint`。`Invalid` は、`NeedsInvalid` のとき Store ごとに `repair.StoreState`(すべてのアイテム・設定・計画・メンバー・Issue のリンク)を作って `repair.ItemProblems` を 1 回呼び、問題のあるアイテムを真にする。受信箱は `Inbox.List` を `Kinds: []port.NoticeKind{port.NoticeOverwrite}` で最後まで読み、未読の `Item` を集める。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/search/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/search
git commit -m "feat(search): 表示中の Store を読んで照合の材料を作る(E1 Task 6)"
```

---

### Task 7: 検索の結果

**Files:**
- Create: `internal/app/search/search.go`(`Search`)、`internal/app/search/keys.go`(追加プロパティのキーの一覧)
- Test: `internal/app/search/search_test.go`

**Interfaces:**
- Consumes: Task 1〜4 の `query.Parse`・`Match`・`Order`、Task 6 の `entries`
- Produces:
  - `type Row struct { Store port.StoreInfo; Facts query.Facts; OldID bool; DaysLeft *int /* 削除済みだけ。0 以下なら保持日数を過ぎている */ }`
  - `type Result struct { Errors []string; HasKey bool; Group string; Deleted bool; Rows []Row }`
  - `func (s Service) Search(ctx context.Context, text string) (Result, error)` — 先にすべての Store(非表示を含む)のワークスペースの設定を読んで追加プロパティのキーを集め、`Parse` する。エラーがあれば `Rows` は空で、アイテムは読まない

- [ ] **Step 1: 失敗するテストを書く**

- `TestSearchMatchesAndOrders`: `kind:bug sort:-points` で Bug だけがポイントの降順。
- `TestSearchErrorsSkipItems`: `status:` で `Errors` 1 行、`Rows` 空。`port.Stores` を包んだ偽物で、`Items().List` が 1 回も呼ばれないことを確かめる。
- `TestSearchKeyOfHiddenStoreIsKnown`(Review Focus 4): 非表示の Store(`SyncTargets` にあり `StoreList` にない)のワークスペースにだけある `customer` は、エラーにならず 0 件。
- `TestSearchDeletedDaysLeft`: 保持日数 30、11/01 に削除、今日 11/15 → `DaysLeft` 16。
- `TestSearchMeIsPerStore`: Store S1 の「あなた」が tanaka、S2 が suzuki のとき、`assignee:@me` は S1 の tanaka 担当と S2 の suzuki 担当の両方。
- `TestSearchGroupAndHasKey`: `group:kind foo` で `Group` は `kind`、`HasKey` 真。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/search/ -run TestSearch`
Expected: FAIL

- [ ] **Step 3: `Search` を書く**

追加プロパティのキーの一覧は、`SyncTargets` のすべての Store(非表示を含む)の、削除されていないワークスペースの設定の、組み込みでないプロパティの `Key` を小文字にしたもの(04 の 304 行)。今日は `planning.Today(s.Clock.Now())`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/search/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/search
git commit -m "feat(search): クエリで絞り込み、並べた結果を返す(E1 Task 7)"
```

---

### Task 8: 画面の状態の保存と、検索・画面の状態の API

**Files:**
- Create: `internal/app/search/screens.go`(口 `ScreenStates`)、`internal/infra/statedb/screens.go`、`internal/adapter/httpapi/search/search.go`、`internal/adapter/httpapi/search/screens.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps.Screens appsearch.ScreenStates`、`search.Register`)、`cmd/hedgehug/wire.go`・`cmd/hedgehug/wire_e2e.go`(`Screens: db.ScreenStates()`)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`)
- Test: `internal/infra/statedb/screens_test.go`、`internal/adapter/httpapi/search/search_test.go`

**Interfaces:**
- Consumes: Task 7 の `Service.Search`・`Result`・`Row`
- Produces:
  - `type ScreenStates interface { Get(ctx context.Context, screen string) (string, bool, error); Put(ctx context.Context, screen, body string) error }`(`app/search`。`port` は 20 個で満杯)
  - `func (d *DB) ScreenStates() ScreenStates`(`statedb` の型。移行 `screens/001`。`ReportSelections` と同じ形)
  - API(E1-24): `POST /api/search`(operationId `search`)、`GET /api/screen-states/{screen}`(`screen-state-get`)、`PUT /api/screen-states/{screen}`(`screen-state-put`)
  - 本文の型 `SearchResult { errors, hasKey, group, deleted, rows }`、`SearchRow { storeId, storeName, storeColor, readOnly, workspaceId, projectId, itemId, display, oldIdHit, title, kind, status: {name, category} | null, assignee: {login, name: string|null} | null, priority: {name} | null, end: string, points: number|null, deleted: {daysLeft: number} | null }`(`name` が null はメンバーにいない人)、`ScreenState { body: string|null }`

- [ ] **Step 1: 失敗するテストを書く**

- `TestScreenStatesRoundTrip`: `Get` でないとき false、`Put` → `Get` で同じ文字列、もう一度 `Put` で上書き。
- `TestSearchAPI`: サンプルの Store で `POST /api/search` `{"query":"kind:bug"}` が 200 で行の `kind` がどれも `Bug`。`{"query":"status:"}` は 200 で `errors` が `["「status:」に値がありません"]`、`rows` が空。
- `TestScreenStateAPI`: `PUT /api/screen-states/search` `{"body":"{\"query\":\"x\"}"}` のあと `GET` で同じ。`screen` が `A B` は 422、本文が 64 KiB を超えると 422。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/infra/statedb/ ./internal/adapter/httpapi/search/`
Expected: FAIL

- [ ] **Step 3: 口・表・API を書き、組み立てに足し、`make generate` を流す**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./... && make lint`
Expected: PASS(`api/openapi.yaml` と `generated.ts` が変わっている)

- [ ] **Step 5: コミット**

```bash
git add internal api web/packages/api/src/generated.ts cmd
git commit -m "feat(search): 検索と画面の状態の API を足す(E1 Task 8)"
```

---

### Task 9: 画面の検索の流れ・画面の状態・カーソルのキー

**Files:**
- Create: `web/packages/api/src/search.ts`(`createSearchPort`・`createScreenStatesPort`)、`web/packages/core/src/features/screenstate/screenStates.ts`、`web/packages/core/src/features/search/search.ts`、`web/packages/core/src/features/search/caret.ts`
- Modify: `web/packages/core/src/features/router/route.ts`(`{ name: "search" }` と `/search`)、`web/packages/core/src/index.ts`
- Test: `screenStates.test.ts`・`search.test.ts`・`caret.test.ts`・`route.test.ts`

**Interfaces:**
- Consumes: Task 8 の API
- Produces:
  - `interface SearchPort { search(query: string): Promise<SearchResult> }`、`interface ScreenStatesPort { get(screen: string): Promise<string | null>; put(screen: string, body: string): Promise<void> }`
  - `createScreenStates(port): { read<T>(screen: string, fallback: T): Promise<T>; write<T>(screen: string, value: T): void }` — ブラウザのタブで最初の `read` だけサーバーから読み、以後はタブの中の値(E1-16)。読めない・壊れた JSON は `fallback`
  - `createSearch({ port, screenStates, wait = 150 }): { state: Readable<SearchState>; open(): Promise<void>; setQuery(text: string): void; replaceQuery(text: string): Promise<void>; reload(): Promise<void> }`、`SearchState = { query: string; result: SearchResult | null; loading: boolean }`。`setQuery` は書いて `wait` ms 待って検索し、古い応答を捨てる(E1-19)。`replaceQuery` はすぐ検索する(パレットから)
  - `interface CaretTerm { key: string; not: boolean; start: number /* 打った値の始まり */; end: number /* カーソル */; typed: string }`、`function caretTerm(text: string, caret: number): CaretTerm | null`

- [ ] **Step 1: 失敗するテストを書く**

- `caret.test.ts`: Task 1 の表の入力について、語の始まりのキーの読み方が同じになる(`http://x` は `http`、`C++>11` は null、`- foo` は null)。`assignee:tanaka,すず|`(`|` はカーソル)→ `{key: "assignee", start: 16, typed: "すず"}`。`-parent:"HH|` → `{key: "parent", not: true, typed: "HH"}`。キーのない語・空白の上は null。
- `search.test.ts`: `setQuery` を 3 回続けて呼ぶと、150ms のあとに 1 回だけ検索する。遅れて返った古い応答は捨てる。`open` は画面の状態の `search` の `query` で検索する。`setQuery` で画面の状態に書く。
- `screenStates.test.ts`: 2 回目の `read` はサーバーを読まない。`write` のあとの `read` は書いた値。
- `route.test.ts`: `/search` ⇄ `{ name: "search" }`。`/search/x` はホーム。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- search screenstate route`
Expected: FAIL

- [ ] **Step 3: 口・流れ・カーソルのキー・ルートを書く**

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages
git commit -m "feat(core): 検索の流れと画面の状態を足す(E1 Task 9)"
```

---

### Task 10: 絞り込み欄の候補

**Files:**
- Create: `web/packages/core/src/features/search/candidates.ts`
- Test: `web/packages/core/src/features/search/candidates.test.ts`

**Interfaces:**
- Consumes: Task 9 の `CaretTerm`、カタログの `StoreNode`、パレットの索引の `PaletteIndex`
- Produces:
  - `interface FilterCandidate { label: string; insert: string; dot?: string /* Store の色。同じ ID が別の Store にもあるとき */ }`
  - `interface CandidateSources { stores: StoreNode[]; items: PaletteItem[]; members: { storeId: string; login: string; name: string }[] }`
  - `type CandidateScope = { kind: "all" } | { kind: "workspace"; storeId: string; workspaceId: string }`(E1 は `all` だけを使う。E4・E7 が `workspace` を使う)
  - `function filterCandidates(term: CaretTerm, src: CandidateSources, scope: CandidateScope): FilterCandidate[]` — 04 の 282〜292 行。10 件まで
  - `function applyCandidate(text: string, term: CaretTerm, c: FilterCandidate): { text: string; caret: number }`

- [ ] **Step 1: 失敗するテストを書く**

- `project:モバ` → 「HH-P1 モバイルアプリ刷新」、入れるもの `HH-P1`。`project:hh-p` でも出る(ID でも探す)。
- `assignee:田中` → 「田中 美咲 @tanaka」、入れるもの `tanaka`。同じログイン名は 1 つ(大文字と小文字を区別しない)。
- `parent:ログイン`・`blocks:HH-1` → 「HH-12 ログイン画面の改修」、入れるもの `HH-12`。
- `workspace:we` → 「Web HH」、入れるもの `HH`。
- 一致なしは空。11 件以上あれば 10 件。
- 別の Store に同じ ID(`HH-12`)があれば `dot` に Store の色。
- `replaces only the typed part`(Review Focus 5): `assignee:tanaka,すず kind:bug` のカーソル「すず」のあとで `suzuki` を選ぶと `assignee:tanaka,suzuki kind:bug`、カーソルは `suzuki` の直後。
- キーが `status` など候補のないキーなら空。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- candidates`
Expected: FAIL

- [ ] **Step 3: `filterCandidates`・`applyCandidate` を書く**

探す文字は、打った値とラベルの両方を小文字にして部分一致(日本語はそのまま)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 絞り込み欄の候補を出す(E1 Task 10)"
```

---

### Task 11: 絞り込み欄の部品と `/` キー

**Files:**
- Create: `web/packages/ui/src/patterns/FilterInput.svelte`
- Modify: `web/packages/ui/src/index.ts`、`web/app/src/App.svelte`(`/` キー)
- Test: `web/packages/ui/src/patterns/FilterInput.test.ts`

**Interfaces:**
- Consumes: なし(ui は core を import しない)
- Produces: `FilterInput` の props `{ value: string; placeholder: string; options: FilterOption[]; errors: string[]; oninput(text: string, caret: number): void; onchoose(index: number): void; onhelp(): void }`、`FilterOption = { label: string; dot?: string }`(ui の中で宣言する。app が core の `FilterCandidate` を渡す)。入力欄に `data-filter-input`

- [ ] **Step 1: 失敗するテストを書く**

- 候補は入力欄にフォーカスがあるあいだだけ出る。↑↓ で選び、Enter かクリックで `onchoose(選んだ位置)`。変換中(`isComposing` または keyCode 229)の Enter は何もしない。
- 候補が出ているときの Esc は候補だけを閉じ、イベントを外へ伝えない(詳細パネルが閉じない)。2 回目の Esc は外へ伝わる。
- 右に「検索ヘルプ」のボタン。押すと `onhelp`。
- `errors` は欄の下に 1 行ずつ赤字(`role="alert"`)。
- `App.svelte`: `/` を押すと `[data-filter-input]` にフォーカスして `/` を入れない。入力中・ダイアログ(`[aria-modal="true"]`)を開いているときと、欄のない画面では何もしない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/ui test -- FilterInput`
Expected: FAIL

- [ ] **Step 3: 部品と `/` キーを書く**

候補の行の Store の点は `PaletteRow` の `store-dot` と同じ描き方。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/ui test && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/ui web/app/src/App.svelte
git commit -m "feat(ui): 絞り込み欄の部品と / キーを足す(E1 Task 11)"
```

---

### Task 12: 検索の画面とサイドバーのナビ

**Files:**
- Create: `web/packages/core/src/features/search/view.ts`(画面に出す文と行の形)、`web/packages/core/src/features/search/help.ts`(検索ヘルプの説明文と表)、`web/app/src/search.ts`(組み立てと `reloads` への登録)、`web/app/src/screens/search/SearchScreen.svelte`、`SearchTable.svelte`、`SearchRow.svelte`、`SearchHelp.svelte`
- Modify: `web/app/src/shell/Screen.svelte`(`search` の画面)、`web/app/src/shell/Sidebar.svelte`(ナビ「検索」)
- Test: `web/packages/core/src/features/search/view.test.ts`(`web/app` には単体テストがないので、表示の中身は core で確かめ、描画は Task 14 の e2e で確かめる)

**Interfaces:**
- Consumes: Task 9 の `createSearch`・`createScreenStates`、Task 10 の `filterCandidates`・`applyCandidate`、Task 11 の `FilterInput`、core の `dueLabel`・`readOnlyNote`、ui の `ReadOnlyBand`・`Avatar`・`StatusDot`
- Produces:
  - `function emptyText(query: string, rows: number, errors: string[]): { title: string; body: string | null; create: boolean } | null` — 行があれば null
  - `function readOnlyBands(rows: SearchRow[]): string[]` — 閲覧のみの Store ごとに 1 本(最初に出た順)
  - `function deletedNote(daysLeft: number): string`
  - `function assigneeText(a: SearchRow["assignee"]): string | null`
  - `const searchHelp: { lead: string; rows: [string, string][] }`

- [ ] **Step 1: 失敗するテストを書く**

`view.test.ts`:
- `emptyText`: エラーあり →「一致するものはありません」「絞り込みを変えてみてください。」。0 件で絞り込みあり → 同じ。絞り込みが空(前後の空白だけを含む)で 0 件 →「まだ何もありません」、`body` null、`create` 真。
- `readOnlyBands`: 閲覧のみの Store「team」の行が 2 件と「docs」の行が 1 件 →「team は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。」と docs の 1 本ずつ。閲覧のみの行がなければ空。
- `deletedNote`: 16 →「あと 16 日で完全に削除」、1 →「あと 1 日で完全に削除」、0 と -1 →「保持日数を過ぎています」。
- `assigneeText`: `{login: "tanaka", name: "田中"}` →「田中」、`name` null →「不明なユーザー」、null → null。
- `searchHelp`: 説明文が 04 の 320 行のとおり、表が 04 の 326〜352 行の 27 行と同じ順・同じ文。

画面に出すもの(Task 14 の e2e で確かめる): 見出し「検索」、補足「すべての Store のアイテムを、条件で絞り込んで見ます」、欄の案内「絞り込み 例: assignee:@me status:InProgress system:Web」、ツールバーの「{N} 件」(行の数)、列の見出し ID・タイトル・ステータス・担当者・優先度・終了日・ポイント、ID の前の Store の点、`oldIdHit` の行の ID の横の「旧 ID」、タイトルで詳細パネル、終了日の `dueLabel`、削除済みの行の薄い表示と「削除済み」のピルと `deletedNote`(タイトルは押せない)、「作成」で作成ダイアログ、「検索ヘルプ」のダイアログ(見出し「検索ヘルプ」、閉じるボタン「閉じる」)、サイドバーの「受信箱」の下の「検索」(`/search` のとき `aria-current="page"`)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- view`
Expected: FAIL

- [ ] **Step 3: 表示の関数・画面・ナビ・組み立てを書く**

候補の元のアイテムはパレットの口 `index()` を、検索の画面を開いたときに 1 回読む。メンバーは最初に `assignee:` の候補を出すときに Store ごとに読む。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/app
git commit -m "feat(web): 検索の画面とサイドバーのナビを足す(E1 Task 12)"
```

---

### Task 13: パレットの「検索を開く」と「このクエリで」

**Files:**
- Create: `web/packages/core/src/features/palette/queryGroup.ts`(入力を待って解析を頼み、いまの入力の応答だけを群にする)
- Modify: `web/packages/core/src/features/palette/actions.ts`(コマンド)、`web/packages/core/src/features/palette/palette.ts`(群を `buildRows` に渡すだけ。259 行なので流れは `queryGroup.ts` に置く)、`PaletteDeps` に `search: Pick<SearchPort, "search">` と `openSearch(query: string): Promise<void>`、`web/app/src/palette.ts`(組み立て)
- Test: `actions.test.ts`・`palette.test.ts`

**Interfaces:**
- Consumes: Task 9 の `SearchPort`・`replaceQuery`

- [ ] **Step 1: 失敗するテストを書く**

- コマンドの群に「検索を開く」が「受信箱を開く」の次。選ぶと `/search` へ(Ctrl+Enter では新しいタブ)。
- 入力 `kind:bug` で 150ms のあとに「このクエリで」の見出しと「検索で開く」(ヒント「3 件」。件数は応答の `rows` の数)。選ぶとパレットを閉じて `openSearch("kind:bug")`。
- `foo`(キーなし)・`status:`(エラー)では群を出さない。入力が変わったあとに古い応答が返っても群を出さない。
- アイテムの候補は群があってもなくても変わらない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- palette`
Expected: FAIL

- [ ] **Step 3: コマンドと群を書く**

`openSearch` は、`search.replaceQuery(query)` を呼んでから `navigation.go({ name: "search" })`。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(palette): 「検索を開く」と「このクエリで」の「検索で開く」を足す(E1 Task 13)"
```

---

### Task 14: 通しのテストと文書

**Files:**
- Create: `web/e2e/search.spec.ts`
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(口の表の `Index` の行を消す。`statedb/` の説明を「受信箱、保存したビュー、画面の状態」に)、`docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「このマシンに持つもの」の索引の段を消し、「クエリのキー」の行の「`state.db` の索引」を「Store をその都度読む」に。関連システムの行も同じ)、`docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(決定 11 の後半を「トラック E は索引の表を作らない(E の決定 E-2)」に)、この計画の末尾「他の小計画・トラックに渡すもの」

- [ ] **Step 1: 通しのテストを書く**

`search.spec.ts`(作ったアイテムのタイトルに日時を入れて、サンプルの件数に頼らない):
- サイドバーの「検索」で開き、`/search` になり、ナビの「検索」が `aria-current="page"`。見出し「検索」・補足・欄の案内・列の見出し 7 つが出る。
- 作ったアイテムのタイトルの語を打つと、その行(Store の点つき)が出て「1 件」。タイトルを押すと詳細パネルが開く。
- `is:deleted` と削除したアイテムのタイトルで、その行に「削除済み」と「あと 30 日で完全に削除」が出て、タイトルが押せない。
- `kind:` で「「kind:」に値がありません」が出て「0 件」。
- `project:` に続けて日本語のプロジェクト名の一部を打つと候補が出て、Enter で `project:{ID}` が入る。
- `/` で欄に移る。
- 「検索ヘルプ」を開いて閉じる。
- 再読み込みで絞り込みが残る。
- パレットで `kind:bug` を打ち「検索で開く」を選ぶと、検索の画面の欄が `kind:bug` になる。
- 閲覧のみの Store のアイテムが出るクエリで、帯が出る。

- [ ] **Step 2: 流して確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/search.spec.ts`
Expected: PASS

- [ ] **Step 3: 文書を直し、この計画の末尾に「他の小計画・トラックに渡すもの」を書く**

中身: E2(テーブルは E1 で初めの 7 列を読むだけで出した(E1-18)。グループ・列・セル・選択・「まとめて変更」・削除済みの行の無効)、E3(「ビューとして保存…」・レイアウトの切り替え・注記)、E4(日付が変わったときの計算し直し・`CandidateScope` の `workspace`)、E7(ツリーの絞り込み欄)、G(診断は `repair.ItemProblems` と `change.Deletion.DaysLeft` を使う・削除済みの行のタイトルから設定の「削除済み」・`store:旧名` の書き換え・診断の「検索で開く」)。

- [ ] **Step 4: すべてを確かめる**

Run: `make check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/e2e docs
git commit -m "test(e2e): 検索の画面を通しで確かめ、設計書の索引を書き換える(E1 Task 14)"
```

## 他の小計画・トラックに渡すもの

E1 で作ったものと、それぞれの小計画・トラックが足すものの境目。

### E2(テーブル)

- 検索の画面のテーブルは、E1 で初めの 7 列(ID・タイトル・ステータス・担当者・優先度・終了日・ポイント)を読むだけで出した(E1-18)。`web/app/src/screens/search/SearchTable.svelte` と `SearchRow.svelte` がそれで、列の見出しは `SearchTable.svelte` の `columns` に決め打ちしてある。E2 は、列の選択・並べ替え・幅、セルの編集、行の選択と「まとめて変更」、グループ分けをここに足す。(列の選択・並べ替え・行の選択と「まとめて変更」・グループ分けは E2a で足した。この 2 つのファイルは `web/app/src/screens/table/` の部品に置き換えて消した。セルの編集は E2b に回した。幅は 04 に規定がないので足さない)
- グループ分けの元は、応答の `group`(`POST /api/search`。E1-9、E1-24)。E1 は解析して返すだけで、画面は使っていない。(E2a で足した。画面が `groups` を使ってグループの見出しを出す)
- 削除済みの行(`is:deleted`)は、E1 では薄く出してタイトルを押せない文字にしただけ(`SearchRow.svelte` の `row.deleted`)。E2 は、ステータスのボタン・セル・選択のチェックボックス・アイテムのメニューを無効にし、「すべて選択」に含めない(04 の 312 行)。(削除済みの行の選択のチェックボックスとアイテムのメニューの無効と、「すべて選択」に含めないことは E2a で足した。テーブルにステータスのボタンはなく、ステータスのセルは表示だけなので、削除済みの行・閲覧のみの行のステータスとセルの無効は、セルの編集と一緒に E2b に回した)
- 「このクエリで」の群(`web/packages/core/src/features/palette/queryGroup.ts`)に、E2 が「{N} 件をまとめて変更…」を足す(E1-21)。(E2a で足した)
- 画面の状態は `screen_states` の表を使う(E1-16)。テーブルの設定は、`search` の鍵の `{"query": …}` に足すか、別の鍵にする。(E2a で足した。`search` の鍵の `{"query": …}` に、列・並べ替え・グループの設定を足した)

### E3(保存したビュー)

- 「このクエリで」の群に「ビューとして保存…」を足す(E1-21)。(E3b に回した。E3b で足した)
- 検索の画面に、レイアウトの切り替え(テーブル・ボード・カレンダーなど)と、レイアウトごとの注記を足す。E1 の画面(`web/app/src/screens/search/SearchScreen.svelte`)の `.toolbar` に件数だけがあり、切り替えの置き場所を空けてある。(E3a で足した。切り替えは作ってあるレイアウト(ツリー・テーブル)だけで、カンバン・ガントは E5・E6 が足す。注記は `layoutNote`。検索の画面のツリーも E3a で作った)
- 保存したビューを開くときは、`web/app/src/search.ts` の `search.setQuery` で絞り込みを入れ替え、`/search` を開く(パレットの「検索で開く」と同じ道)。(E3b に回した。E3b ではビューに自分のアドレス(`/views/{id}`)と画面の状態(`view:{id}`)を持たせた(01 の 165 行、04 の 409〜420 行))

### E4(マイタスクなど)

- 日付が変わったときに `today`・`-7d` のクエリを計算し直す仕組み(04 の 42 行)は E4 が作る。E1 は検索を出すたびにその時点の今日で計算する(E1-11、E1-23)。(E4a で足した。時計は `web/app/src/clock.ts`)
- `web/packages/core/src/features/search/candidates.ts` の `CandidateScope` は、E1 では `{ kind: "all" }` だけを使う。E4 は `{ kind: "workspace"; storeId; workspaceId }` で、ワークスペースの範囲の候補を使う。範囲を絞る処理は `scopedStores` と `itemEntries`・`memberEntries` に書いてあり、E1 のテスト(`candidates.test.ts`)で確かめてある。(E4b に回した)

### E7(ツリー)

- ツリーの絞り込み欄は、E1 の `FilterInput`(`web/packages/ui/src/patterns/FilterInput.svelte`)と `caret.ts`・`candidates.ts` をそのまま使う。クエリの解析と一致の判定はサーバーの `domain/query` にだけある(E1-1)ので、画面で判定を書き直さない。範囲は `CandidateScope` の `workspace` か、プロジェクトの範囲を足す。

### G(診断と設定)

- 診断の「無効なアイテム」は、E1 で作った `repair.ItemProblems`(`internal/domain/repair/problems.go`)を使う。`is:invalid` と食い違わないため、別の判定を作らない(E1-7)。
- 設定の「削除済み」の残り日数は `change.Deletion.DaysLeft`(`internal/domain/change/`)を使う。検索の行の「あと N 日で完全に削除」と同じ数になる(E1-8)。
- 削除済みの行のタイトルを押したら、設定の「削除済み」を開き、そのアイテムの行を選んだ状態にする(04 の 312 行)。E1 は押せない文字で出しているだけ(`SearchRow.svelte`)。
- Store の名前を変えるとき、検索の画面の状態(`screen_states` の `search` の鍵の `query`)にある `store:旧名` を新しい名前に書き換える(E1-22)。保存したビューのクエリと `view:*` の画面の状態も同じく書き換える(E3b-21)。
- 診断の「検索で開く」は、`screen_states` の `search` を書いて `/search` を開く(E1-22)。
