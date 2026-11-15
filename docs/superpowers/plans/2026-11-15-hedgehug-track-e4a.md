# hedgehug トラック E4a(マイタスクと日付の変わり目)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** マイタスクの画面(絞り込み・まとめ方・行・空のとき)を作り、サイドバーのナビ「マイタスク」と件数のバッジ、パレットのコマンド「マイタスクを開く」を足す。仮のホームをなくし、画面を指さない URL(`/`)では最後に開いていた画面を開き、なければマイタスクを開く。日付が変わったときに、日付で決まる表示(期限の強調、マイタスクのグループとバッジ、`today` を含む検索の結果)を更新する。ワークスペースの概要は E4b で作る。

**Architecture:** マイタスクの対象と絞り込みの一致は、サーバーの新しい `POST /api/my-tasks` が決める(クエリの解析はサーバーだけ。E1-1)。サーバーは対象の条件と絞り込みを AND にし、`sort:`・`group:` を使わずに既定の並びで返す。まとめ方(期限・プロジェクト・ワークスペース)は画面が作る。期限の区分は画面の「今日」で決めるので、日付が変わると描き直すだけで区分が替わる。日付の変わり目は、app の時計(`syncNow`)から作る `today`(日付が変わったときだけ値が替わる)で知らせる。日付が変わったら、Store が変わった出来事を 1 つ受けたのと同じに扱う(画面への反映の `reflection` に `store` を積む。入力中・ダイアログ中は、出来事と同じく済むまで待つ)。これで、サーバーが今日で計算するもの(`today` の検索・削除済みの残り日数・マイタスクとバッジ)も読み直される。最後に開いていた画面は、画面の状態の鍵 `last-screen` に、出した画面のアドレスを書いて覚える。

**Tech Stack:** Go 1.27.3、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`: 「集計の定義」(18〜42 行。42 行が日付の変わり目)、「ナビ」(55〜64 行)、「マイタスク」の全節(112〜158 行)、「組み立て方」(169〜179 行。179 行の範囲と `is:deleted`)、「候補」(278〜292 行。288 行のマイタスクの `project:`)、「エラー」(294 行〜)、「検索ヘルプ」(316 行)、「モックからの変更点」のマイタスクの 4 行
- 01: 「日付と時刻」(132 行)、「画面の移り方」(149 行。開き直したときの画面)、「画面のアドレス」(161〜176 行)
- 02: コマンドパレットのコマンドの群(621 行。「マイタスクを開く」は「新規作成」の次)
- `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`(トラック E の分け方)、`2026-11-15-hedgehug-track-e1.md` の末尾「E4(マイタスクなど)」、`2026-11-15-hedgehug-track-e3a.md` の末尾「E4(マイタスクと概要)」(概要の分は E4b)、`2026-11-13-hedgehug-track-a2.md` の決定 6、`2026-11-13-hedgehug-track-a3a.md` の決定 22

## 始める前に

- E4 は 2 本に分けた(ユーザーの決定、2026-11-15)。**E4a = この計画**。E4b = ワークスペースの概要(統計のカード「プロジェクト」「未完了のアイテム」「期限切れ」、プロジェクトの一覧と行のプロジェクトのメニュー、最近の変更)で、E2c(ワークスペースの「テーブル」タブ)が develop に入ってから始め、「期限切れ」カードを押すとテーブルのタブを開くところまで E4b で作る。
- ブランチは `feat/track-e4`(develop c2459c83 から)。push は `git push -u origin feat/track-e4`。
- 同時に E3b(保存したビュー、ブランチ `feat/track-e3b`)が `route.ts`・`Screen.svelte`・`Sidebar.svelte`・`palette/actions.ts`・`search.ts`・core の `createSearch` を変えている。この計画は `createSearch` の引数を変えない。あとから develop に入る方が衝突を解く。

## Global Constraints

E1・E2a・E3a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。いまの数: `internal/app/search/search.go` 81 行(テスト以外 11 個)、`internal/adapter/httpapi/search/search.go` 169 行、`internal/adapter/httpapi/routes.go` 208 行、`cmd/hedgehug/wire_e2e.go` 223 行、`web/app/src/wire.ts` **299 行**(Task 7 で時計を外へ出してから足す)、`web/app/src/search.ts` 188 行、`web/app/src/App.svelte` 146 行、`web/app/src/shell/Sidebar.svelte` 168 行、`web/app/src/shell/Screen.svelte` 75 行、`web/app/src/screens/table/ItemTable.svelte` 102 行、`web/packages/core/src/features/screen/screen.ts` 232 行、`features/router/resolve.ts` 220 行、`features/router/route.ts` 134 行、`features/palette/actions.ts` 253 行、`features/search/candidates.ts` 187 行、`web/packages/core/src/index.ts` 207 行、`web/e2e/helpers.ts` 295 行。
- 画面のパッケージの依存の向き: ui は core を import しない。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書く。閲覧のみは core の `readOnlyNote`(「閲覧のみの Store です」)、削除済みの行の添え書きは `deletedNote`、期限は `dueLabel`。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする。
- e2e のビルドも CI で lint する。手元の検証は 1 回で足りる。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| E4a-1 | E4 を E4a(この計画)と E4b(ワークスペースの概要。E2c のあと)に分ける | ユーザーの決定(2026-11-15) |
| E4a-2 | サーバーに `POST /api/my-tasks`(本文 `{filter}`、応答 `{errors, rows}`)を足す。対象の条件 `assignee:@me -category:done,canceled -kind:epic` と絞り込みを空白でつないで 1 つのクエリとして解析する(語はすべて AND なので、つないでも意味は変わらない)。`sort:`・`group:` は使わず、行は既定の並び(Store・ワークスペース・アイテムの並び順)で返す。行の形は検索の `SearchRow` | 04 の 118・128 行(条件は対象の条件に加わる)、04 の 136 行(グループの中の並びは表で決まる)。E1-1(解析はサーバーだけ) |
| E4a-3 | 絞り込みに否定でない `is:deleted` があるときは、04 の 179 行のとおり対象が削除済みのアイテムだけになる。削除済みの行は、タイトルをリンクにせず、ステータスのボタンを無効にし、`deletedNote(daysLeft)` を添える(検索のテーブルの削除済みの行と同じ) | 04 の 179 行、E2a の削除済みの行の扱い |
| E4a-4 | `SearchRow` に `statusId`(アイテムのステータスの ID。なければ空)を足す。マイタスクの行のステータスのボタン(`StatusButton`)に渡す。ステータスのメニューの中身は、行のワークスペースの設定(`workspacesPort.get`。ワークスペースごとに 1 回読み、Store の読み直しで捨てる)で作る。設定を読めるまでは、ボタンの代わりにステータスの名前を文字で出す | 04 の 151 行(ステータスのボタンは 02 の規則) |
| E4a-5 | まとめ方は画面が作る(`groupMyTasks`)。「期限」の区分は画面の今日(`today` の日付)で決める。週は月曜に始まり日曜に終わる。グループの中は終了日の早い順で、同じ日はサーバーの既定の並びを保つ(安定した並べ替え)。「期限なし」は既定の並びのまま。「プロジェクト」「ワークスペース」では、終了日がないものを末尾にする | 04 の 133〜147 行 |
| E4a-6 | 名前の順は、検索の `sort:` と同じく文字コードの順で比べる(E1-10)。プロジェクト名が同じグループは別のグループのまま、サイドバーの並び(Store・ワークスペース・プロジェクト)で並べる。ワークスペース名が同じなら Store の並び、Store も同じならワークスペースの並び | 04 の 135 行(同名は Store の順) |
| E4a-7 | 見出し: 期限のグループは「期限切れ」「今日」「今週」「来週以降」「期限なし」で、「期限切れ」「今日」は警告の色。プロジェクトのグループはプロジェクトのタイトル。ワークスペースのグループはワークスペース名で、前に Store のバッジ(`StoreBadge`)を出す。各見出しの後ろに件数を添える。件数のないグループは出さない | 04 の 133〜147 行 |
| E4a-8 | 空のとき: 対象(絞り込みなし)が 0 件なら「担当しているタスクはありません」と「新しく割り当てられると、ここと受信箱に出ます。」。対象があり、絞り込みで 0 件なら「一致するものはありません」と「絞り込みを変えてみてください。」。書き方に誤りがあるときは、エラーを赤字で出し、空の表示は「一致するものはありません」の方にする(E3a-19 と同じ) | 04 の 153〜158・128 行 |
| E4a-9 | 画面の状態の鍵 `my-tasks` に `{filter, groupBy}` を書く。初めは `{filter: "", groupBy: "due"}`。形の合わない項目だけ初めの値にする(E2a-13 と同じ)。絞り込みの入力は 150 ms 待ってから問い合わせる(検索と同じ `searchWaitMs`)。古い問い合わせの応答は捨てる | 04 の 129 行、01 の画面の状態 |
| E4a-10 | バッジは、対象(絞り込みなし)の件数。`POST /api/my-tasks` を絞り込み空で問い合わせた行の数で、0 件なら出さない。読み上げの名前は「未完了 {N} 件」(受信箱の「未読 {N} 件」と同じ組み立て)。Store の出来事・自分の書き込みのあと(`reloads`・`itemChanges`)・日付の変わり目で読み直す | 04 の 59・122 行 |
| E4a-11 | 絞り込み欄の候補は、表示中のすべての Store から出す(検索と同じ)。ただし `project:` の候補は、バッジで読んだ対象の行があるプロジェクトだけにする。`CandidateScope` に `{ kind: "myTasks"; projects: ReadonlySet<string> }`(鍵は `{storeId}/{projectId}`)を足す | 04 の 288 行 |
| E4a-12 | 絞り込み欄の横の「検索ヘルプ」は、検索の画面と同じ `SearchHelp` を開く | 04 の 316 行(絞り込み欄の横) |
| E4a-13 | マイタスクのアドレスは `/my-tasks`。`/` は「画面を指さない URL」で、開くと最後に開いていた画面へ移る(履歴に積まない)。最後の画面がないとき(初めて開いた)はマイタスクへ移る。最後の画面がもうないとき(削除・非表示など)は、ほかのアドレスと同じく、見つからない理由のトーストを出してマイタスクへ移る。解釈できないアドレスと、これまで仮のホームへ移していたところ(画面がなくなったとき、旧 ID で開けないとき、ワークスペースの削除のあと、移動の移り先がないとき)は、すべてマイタスクへ移す | 01 の 149・168・169 行、A2 の決定 6 |
| E4a-14 | 最後の画面は、画面の状態の鍵 `last-screen` に `{path}`(`formatRoute` の形)で書く。書くのは、画面の解決が終わって出した画面ごと(`/` とアイテムの入口は書かない。アイテムの入口はツリーのアドレスに替わってから書く)。複数のブラウザのタブでは、あとに出した画面が残る | 01 の 149・175 行 |
| E4a-15 | ホームにあった版の表示を画面から削除する。使われなくなる web の system の部品(core の `features/system`・`port/system.ts`、api の `system.ts`)と、サーバーの `GET /api/system`(`internal/adapter/httpapi/system`)も削除する。`hh --version` はそのまま | ユーザーの決定(2026-11-15)。仕様に版を画面に出す規定がない |
| E4a-16 | 日付の変わり目: app の時計 `syncNow` は、30 秒ごとに加えて、画面に戻ったとき(`visibilitychange`・フォーカス)にも進める。core の `createToday(now)` は、`dayOf(now)` が替わったときだけ値が替わる。日付が替わったら(最初の値は除く)、core の `Reflection` に足す `queue(kind)` で `store` を積む。積んだものは、出来事と同じく、待つ条件(`held`。ダイアログ・メニュー・入力欄のフォーカスほか)になく見えているときに `reflect` される。期限の強調は `today` で描く(プロジェクトのツリー・プロジェクトの概要・検索のツリー・検索のテーブル) | 01 の 132 行(1 分ごとの確認と、画面に戻ったとき・スリープから戻ったとき)、04 の 42 行、A3a の決定 22、E1-23 |
| E4a-17 | 01 の 132 行の更新の対象の持ち主: マイタスクのグループとバッジ・期限の強調と「期限切れ」「今日」・`today` などを含む検索の結果・検索の削除済みの行の「あと N 日」は E4a。概要の「期限切れ」カードは E4b(`reflect` で読み直す)。ガントの今日の線は E6。スプリントの「残り N 日」(F1)・アクティビティの「今日」「昨日」(C4d)・累積フロー図(F3)は `syncNow` で作ってある。設定の「削除済み」の「あと N 日」と自動の完全削除は G6 | 01 の 132 行、トラック G の分け方 |
| E4a-18 | 入力中の欄と開いているダイアログは動かさない: 日付の変わり目の読み直しは `reflection.queue("store")` で Store の出来事と同じ待ち(`held`・`visible`)を通るので、06 の反映の規則のまま。`today` で描き直す期限の表示は、値を書き換えないので待たない。マイタスクの絞り込み欄は、読み直しで中身を書き換えない(読み直しは、いまの絞り込みで問い合わせ直すだけ) | 01 の 132 行 |
| E4a-19 | パレットのコマンド「マイタスクを開く」を「新規作成」の次に置く。サイドバーのナビは、マイタスク・受信箱・アクティビティ・検索の順 | 02 の 621 行、04 の 57〜62 行 |
| E4a-20 | e2e の `tokenUrl()` の初めのパスを `/my-tasks` にする。同じ worker のサーバーをほかのテストと共有するので、`/` で開くと前のテストの最後の画面が開くため。`/` のテストは、先に `PUT /api/screen-states/last-screen` で最後の画面を置いてから開く | `web/e2e/fixtures.ts`(worker ごとに 1 つのサーバー) |
| E4a-21 | e2e のサーバーでは、閲覧のみの Store のサンプルの Task(HH-2「ログイン画面を作る」)の担当者を taro にする。閲覧のみの Store の行のステータスのボタンが無効になることを確かめるため(閲覧のみの Store には API で書けない) | 04 の 151 行 |
| E4a-22 | `LayoutSwitch` に読み上げの名前の引数 `label`(初めは「レイアウト」)を足し、まとめ方の切り替え(`label="まとめ方"`)にも使う | 部品を増やさないため |

## Review Focus

1. **日曜日に「今週」がずれる。** 今日が日曜(例 2026-11-15)のとき、終了日が 11-16 のアイテムは「来週以降」に入り、「今週」の見出しは出ない。今日が月曜(11-16)なら、11-22(日)までが「今週」、11-23 が「来週以降」。Task 3 の `due groups on sunday`・`due groups on monday` に入れる。
2. **絞り込みでバッジの件数が減る。** マイタスクで `priority:high` と絞り込んでも、サイドバーのバッジは対象全体の件数のまま。Task 4 の `base ignores the filter` と Task 6 の e2e に入れる。
3. **日付の変わり目の読み直しで、打ちかけの絞り込みが消える・古い応答で戻る。** 絞り込みを打っている途中に読み直しが走っても、欄の文字はそのままで、最後に打った絞り込みの結果だけが出る。Task 4 の `reload keeps the typed filter`・`late answer is dropped` に入れる。
4. **最後の画面がもうないと、マイタスクへ移れずに止まる・繰り返す。** `last-screen` が削除済みのワークスペースのアドレスのとき、`/` を開くと、トーストを 1 回出してマイタスクが開く。Task 2 の `restore goes to my tasks when gone` と Task 6 の e2e に入れる。
5. **同じ名前のプロジェクトが 2 つの Store にあると、1 つのグループに混ざる。** Store「e2e」と Store「team」の両方に「運営」があるとき、まとめ方「プロジェクト」では「運営」の見出しが 2 つ、サイドバーの Store の順に出る。Task 3 の `same project name stays apart` に入れる。

---

### Task 1: マイタスクの API(サーバー)

**Files:**
- Create: `internal/app/search/mytasks.go`、`internal/app/search/mytasks_test.go`、`internal/adapter/httpapi/search/mytasks.go`
- Modify: `internal/app/search/search.go`(行を集める部分を `Search` と共有する関数に分ける)、`internal/adapter/httpapi/search/search.go`(`SearchRow.StatusID`、登録)、`internal/adapter/httpapi/search/search_test.go`、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`)、`web/packages/core/src/features/search/types.ts`(`SearchRow.statusId`)、`web/packages/api/src/search.ts`(行の写しを `rowOf` に分けて export し、`statusId` を写す)、行を作っている core のテストの材料(`fixtures.test-util.ts` など。`statusId: ""` を足す)

**Interfaces:**
- Produces: `const MyTasksQuery = "assignee:@me -category:done,canceled -kind:epic"`、`func (s Service) MyTasks(ctx context.Context, filter string) (Result, error)`(`Result` の `Errors`・`Rows` だけを使う)。API `POST /api/my-tasks`、本文 `MyTasksQuery{filter string}`、応答 `MyTasksResult{errors []string, rows []SearchRow}`(`operationId: my-tasks`)。`SearchRow.statusId string`(json `statusId`、なければ空)。api の `rowOf(x): SearchRow`

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/search/mytasks_test.go`(材料は `newFixture`・`addItem`・`task`。担当者・ステータス・終了日は `item.Item` の欄に直接入れる。`f` の「あなた」は `loginByStore` で `taro`):
- `TestMyTasksBase`: 担当 taro の未着手の Task・進行中の Step・担当 taro の Epic・担当 taro の完了の Task・担当 hanako の Task・担当なしの Bug を作る → 行は Task と Step の 2 件だけ。
- `TestMyTasksFilterAnds`: 担当 taro の Task 2 件(優先度 High と Low)→ `MyTasks(ctx, "priority:high")` は High の 1 件。
- `TestMyTasksIgnoresSortAndGroup`: 担当 taro の Task「b」(番号 1)・「a」(番号 2)→ `MyTasks(ctx, "sort:title group:status")` の行の並びは `MyTasks(ctx, "")` と同じ(番号 1 → 2)。
- `TestMyTasksFilterError`: `MyTasks(ctx, "status:")` → `Errors` が `["「status:」に値がありません"]`、`Rows` は空。
- `TestMyTasksDeleted`: 担当 taro の Task を削除済みにする → `MyTasks(ctx, "")` に出ず、`MyTasks(ctx, "is:deleted")` に出る(`DaysLeft` がある)。

`internal/adapter/httpapi/search/search_test.go` に `TestMyTasksAPI`: `POST /api/my-tasks` に `{"filter":""}` で 200、行の `statusId` がアイテムのステータスの ID。`{"filter":"status:"}` で `errors` が 1 件、`rows` が `[]`(null でない)。既存の `TestSearchAPIRowValues` に `statusId` の確かめを足す。

- [ ] **Step 2: 失敗することを確かめる**

Run: `go test ./internal/app/search/ ./internal/adapter/httpapi/search/`
Expected: FAIL(`MyTasks` がない)

- [ ] **Step 3: 実装する**

`Search` のうち、解析のあと(`entries` を読み、`query.Match` で集め、`query.Order` で並べる部分)を `func (s Service) collect(ctx context.Context, q query.Query, columns []Column, sortBy *query.Sort) ([]*Row, time.Time, error)` に分け、`Search` と `MyTasks` で使う。`MyTasks` は `MyTasksQuery + " " + filter` を `query.Parse` し、エラーがあれば `Result{Errors}` を返す。なければ `q.Sort = nil`・`q.Group = ""` にして `collect(ctx, q, nil, nil)` の行を返す。ハンドラーは `toRow` を使い、`Status` の隣に `StatusID: string(f.Item.Status)` を入れる。

- [ ] **Step 4: 通ることを確かめる**

Run: `make generate && go test ./internal/... && pnpm -C web check && pnpm -C web test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal api web
git commit -m "feat(search): マイタスクの API と、検索の行のステータスの ID を足す(E4a Task 1)"
```

---

### Task 2: マイタスクのアドレスと最後の画面(core)

**Files:**
- Create: `web/packages/core/src/features/screen/lastScreen.ts`、`lastScreen.test.ts`
- Modify: `web/packages/core/src/features/router/route.ts`・`route.test.ts`、`features/router/resolve.ts` と、そのテスト(`resolve*.test.ts`)、`features/screen/screen.ts` と、そのテスト(`screen*.test.ts`)、`features/transfers/leaveMoved.ts`・テスト、`features/workspaces/delete.ts`・テスト、`web/packages/core/src/index.ts`(`createLastScreen`・`lastScreenKey` を公開)、`{ name: "home" }` を移り先に使っているほかのテスト(`grep -rn '"home"' web/packages/core/src`)

**Interfaces:**
- Produces:
  - `Route` に `{ name: "mytasks" }`。`parseRoute("/my-tasks")` → `{name: "mytasks"}`、`parseRoute("/")` → `{name: "home"}`、解釈できないパス → `{name: "mytasks"}`。`formatRoute({name: "mytasks"})` → `"/my-tasks"`。
  - `Resolved` の `{ name: "home"; gone?: Gone }` を `{ name: "mytasks"; gone?: Gone } | { name: "home" }` にする。見つからないときの解決はすべて `mytasks`(gone を添える)。`home` の解決は `home`(最後の画面へ移るまでのあいだだけ)。
  - `screen.ts`・`leaveMoved.ts`・`delete.ts` の移り先 `{name: "home"}` を `{name: "mytasks"}` にする(E4a-13)。
  - `lastScreenKey = "last-screen"`。`interface LastScreen { start(): () => void }`、`createLastScreen(deps: { navigation: Navigation; current: Readable<Resolved | null>; screenStates: ScreenStates }): LastScreen`。`start` は (1) `current` が `home`・`item` 以外で null でなくなるたびに `screenStates.write(lastScreenKey, { path: formatRoute(get(navigation.route)) })`、(2) `navigation.route` が `home` になるたびに `screenStates.read<{path: string} | null>(lastScreenKey, null)` を読み、`path` が文字列なら `navigation.replace(parseRoute(path))`、でなければ `navigation.replace({name: "mytasks"})`。止める関数を返す。

- [ ] **Step 1: 失敗するテストを書く**

`route.test.ts`: `parses my tasks`(上の 3 つの parse と format)、既存の「解釈できないアドレスはホーム」のテストを「マイタスク」に直す(`/settings/nope`・`/s/x` → `{name: "mytasks"}`)。

`lastScreen.test.ts`(navigation と screenStates は既存のテストの偽物を使う。`features/router/history.test-util.ts`・`screenstate` の偽の port):
- `writes the shown screen`: route を `{name: "workspace", store: "e2e", prefix: "HH", tab: "sprints"}`、current を workspace の解決にすると、`last-screen` に `{"path":"/s/e2e/w/HH/sprints"}` が書かれる。
- `does not write home or item`: current が `home`・`item` のときは書かない。
- `restore goes to the last screen`: `last-screen` が `{"path":"/inbox"}` のとき、route を `home` にすると `replace({name: "inbox"})`。
- `restore goes to my tasks when empty`: 保存がない・`null`・壊れた JSON のとき `replace({name: "mytasks"})`。
- `restore does not override a later move`: 画面の状態を読んでいるあいだに route が `inbox` に替わったら、読み終えても `replace` しない(読み終えたときに route がまだ `home` のときだけ移る)。
- `restore goes to my tasks when gone`(Review Focus 4): `last-screen` が削除済みのワークスペースのアドレスで、screen の解決が `{name: "mytasks", gone}` を返すとき、トーストは 1 回だけで、移り先は `mytasks`(`screen.test.ts` の既存の gone のテストと同じ組み立てで、`createScreen` と `createLastScreen` を一緒に動かす)。

`screen*.test.ts`・`resolve*.test.ts`・`leaveMoved`・`delete` の既存のテストの期待を `home` から `mytasks` に直す。

- [ ] **Step 2: 失敗することを確かめる**

Run: `pnpm -C web test -- route lastScreen screen resolve leaveMoved delete`
Expected: FAIL

- [ ] **Step 3: 実装する**

上の Interfaces のとおり。`resolve.ts` の `const home` を `const notFound = { name: "mytasks" } as const` にし、`r.name !== "home"` の見分けを `r.name !== "mytasks"` にする。`screen.ts` の「置き換えたホームの解決は gone を持たない」の分岐は `mytasks` で同じに働く。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS(app の `Screen.svelte` は Task 5 まで `home` の分岐で仮の画面を出す。`check` が `home` の型で落ちるなら、`Screen.svelte` の else の分岐に `mytasks` を入れる最小の直しだけをこの Task で行う)

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app
git commit -m "feat(core): マイタスクのアドレスと、最後に開いていた画面を覚えて開く仕組みを足す(E4a Task 2)"
```

---

### Task 3: 今日の値と、マイタスクのまとめ方(core)

**Files:**
- Create: `web/packages/core/src/features/dates/today.ts`、`today.test.ts`、`web/packages/core/src/features/mytasks/group.ts`、`group.test.ts`、`web/packages/core/src/features/mytasks/view.ts`、`view.test.ts`、`web/packages/core/src/myTasksIndex.ts`(公開物をまとめ、`index.ts` から `export *`)
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Produces:
  - `createToday(now: Readable<Date>): Readable<Date>`: 購読すると今の値をすぐ渡し、そのあとは `dayOf` が替わったときだけ渡す。`onDayChange(today: Readable<Date>, run: () => void): () => void`: 最初の値では呼ばず、替わるたびに `run` を呼ぶ。
  - `type MyTasksGroupBy = "due" | "project" | "workspace"`、`myTasksGroupChoices = [{id: "due", label: "期限"}, {id: "project", label: "プロジェクト"}, {id: "workspace", label: "ワークスペース"}]`
  - `interface MyTaskGroup { key: string; label: string; warn: boolean; store: { name: string; color: string } | null; rows: SearchRow[] }`(`store` はワークスペースのまとめ方だけ)
  - `groupMyTasks(rows: SearchRow[], by: MyTasksGroupBy, today: string, stores: StoreNode[]): MyTaskGroup[]`(`today` は `YYYY-MM-DD`。`stores` は catalog の並び。ワークスペース名と並びに使う)
  - `myTasksEmpty(baseCount: number): { title: string; body: string }`(0 件なら「担当しているタスクはありません」「新しく割り当てられると、ここと受信箱に出ます。」、ほかは「一致するものはありません」「絞り込みを変えてみてください。」)
  - `myTasksTitle = "マイタスク"`、`myTasksNote = "あなたが担当しているタスク"`、`myTasksPlaceholder = "絞り込み 例: priority:high"`、`myTasksBadge(n: number): string`(「未完了 {n} 件」)

- [ ] **Step 1: 失敗するテストを書く**

`today.test.ts`:
- `emits only when the day changes`: `writable(new Date(2026, 10, 15, 10))` から作り、同じ日の 23:59 を入れても購読は呼ばれず、翌日 0:00 で呼ばれる(呼ばれた回数 2 = 最初と翌日)。
- `onDayChange skips the first value`: 最初は呼ばれず、日付が替わると 1 回呼ばれる。

`group.test.ts`(行は `fixtures.test-util.ts` の行を元に `end`・`projectId`・`project.title`・`workspaceId`・`storeId` だけ替える。catalog は Store「e2e」(WEB・Mobile)と Store「team」(運営)):
- `due groups on sunday`(Review Focus 1): 今日 `2026-11-15`(日)、終了日 11-14・11-15・11-16・なし → 「期限切れ」「今日」「来週以降」「期限なし」の 4 つ。「今週」は出ない。「期限切れ」「今日」だけ `warn: true`。
- `due groups on monday`: 今日 `2026-11-16`(月)、終了日 11-22・11-23 → 「今週」に 11-22、「来週以降」に 11-23。
- `due keeps server order on ties`: 同じ終了日の 2 行は渡した順のまま。終了日の早い行が先。
- `project groups by title`: タイトル「b」「a」のプロジェクト → 「a」「b」の順。中は終了日の早い順で、終了日のない行は末尾。
- `same project name stays apart`(Review Focus 5): Store「e2e」と Store「team」の両方に「運営」→ 見出し「運営」が 2 つ、Store「e2e」の方が先。
- `workspace groups carry the store`: まとめ方「ワークスペース」の見出しは catalog のワークスペース名で、`store` が `{name, color}`。同名のワークスペースは Store の並び。

`view.test.ts`: `myTasksEmpty(0)`・`myTasksEmpty(3)` の文、`myTasksBadge(2)` が「未完了 2 件」。

- [ ] **Step 2: 失敗することを確かめる**

Run: `pnpm -C web test -- today group view`
Expected: FAIL

- [ ] **Step 3: 実装する**

`groupMyTasks` は、まとめ方ごとに行をグループに配る関数と、グループの並べ方を分ける。日付は `YYYY-MM-DD` の文字列のまま比べる。今週の日曜は、`today` の曜日(月=1〜日=7)から `7 - 曜日` 日あと。日付の足し算は `new Date(y, m - 1, d + n, 12)` を `dayOf` で文字列に戻す(activity の `yesterdayOf` と同じ形)。グループの中の並びは `Array.prototype.sort` の安定性を使う。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 今日の値と、マイタスクのまとめ方を足す(E4a Task 3)"
```

---

### Task 4: マイタスクの状態とバッジの件数・候補の範囲(core・api)

**Files:**
- Create: `web/packages/core/src/port/myTasks.ts`、`web/packages/core/src/features/mytasks/myTasks.ts`、`myTasks.test.ts`、`web/packages/core/src/features/mytasks/base.ts`、`base.test.ts`、`web/packages/api/src/myTasks.ts`
- Modify: `web/packages/core/src/myTasksIndex.ts`、`web/packages/core/src/features/search/candidates.ts`・`candidates.test.ts`、`web/packages/api/src/index.ts`

**Interfaces:**
- Consumes: Task 1 の `POST /api/my-tasks`・api の `rowOf`、Task 3 の `MyTasksGroupBy`
- Produces:
  - `interface MyTasksPort { list(filter: string): Promise<{ errors: string[]; rows: SearchRow[] }> }`、api の `createMyTasksPort(): MyTasksPort`
  - `myTasksScreen = "my-tasks"`、`interface MyTasksState { filter: string; groupBy: MyTasksGroupBy; rows: SearchRow[]; errors: string[]; loaded: boolean }`
  - `createMyTasks(deps: { port: MyTasksPort; screenStates: ScreenStates; timer: Timer; toasts: Toasts }): MyTasks`、`interface MyTasks { state: Readable<MyTasksState>; open(): Promise<void>; setFilter(text: string): void; setGroupBy(by: MyTasksGroupBy): void; reload(): Promise<void> }`。`open` は画面の状態を読んで 1 回問い合わせる。`setFilter` は状態の `filter` をすぐ替えて画面の状態に書き、`searchWaitMs` 待って問い合わせる。`setGroupBy` は書くだけで問い合わせない。読めないときはエラーのトーストを出し、前の行のまま(`loaded` は変えない)
  - `createMyTaskBase(deps: { port: MyTasksPort }): MyTaskBase`、`interface MyTaskBase { rows: Readable<SearchRow[] | null>; reload(): Promise<void> }`(読めないときは前の値のまま、トーストは出さない)、`myTaskProjects(rows: SearchRow[]): ReadonlySet<string>`(`{storeId}/{projectId}`)
  - `CandidateScope` に `{ kind: "myTasks"; projects: ReadonlySet<string> }`。`project:` だけ `projects` に入るプロジェクトに絞り、ほかのキーは `all` と同じ

- [ ] **Step 1: 失敗するテストを書く**

`myTasks.test.ts`(port・timer・screenStates は偽物。`search.test.ts` の偽物の作り方に合わせる):
- `open reads the saved state`: 画面の状態 `{"filter":"priority:high","groupBy":"project"}` → 状態の `filter`・`groupBy` がそれ、port は `"priority:high"` で 1 回呼ばれる。
- `bad saved fields fall back`: `{"filter":3,"groupBy":"nope"}` → `filter: ""`・`groupBy: "due"`。
- `setFilter waits then asks`: `setFilter("a")`・`setFilter("ab")` を続けて呼ぶと、150 ms のあと port は `"ab"` で 1 回だけ呼ばれ、画面の状態には `ab` が書かれる。
- `late answer is dropped`(Review Focus 3): `"a"` の応答が `"ab"` の応答よりあとに届いても、行は `"ab"` の応答のまま。
- `reload keeps the typed filter`(Review Focus 3): `setFilter("ab")` のあとに `reload()` を呼んでも、状態の `filter` は `ab` のままで、port は `"ab"` で呼ばれる。
- `errors empty the rows`: 応答が `{errors: ["x"], rows: []}` なら、状態の `errors` が `["x"]`、`rows` が空。
- `setGroupBy writes without asking`: port は呼ばれず、画面の状態に `groupBy` が書かれる。

`base.test.ts`:
- `base ignores the filter`(Review Focus 2): `reload()` は port を `""` で呼ぶ。
- `keeps rows on failure`: 2 回目の読み込みが失敗しても、`rows` は 1 回目の値のまま。
- `myTaskProjects` が `{storeId}/{projectId}` の集まりを返す。

`candidates.test.ts`: `my tasks limit project candidates`: `projects` に `s1/p1` だけがあるとき、`project:` の候補は p1 だけ。`workspace:`・`assignee:` の候補は `all` と同じ。

- [ ] **Step 2: 失敗することを確かめる**

Run: `pnpm -C web test -- myTasks base candidates`
Expected: FAIL

- [ ] **Step 3: 実装する**

上の Interfaces のとおり。古い応答を捨てるのは、問い合わせごとの通し番号(`search.ts` の作りと同じ)。画面の状態の読み方は、形の合わない項目だけ初めの値にする(E4a-9)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages
git commit -m "feat(core): マイタスクの状態・バッジの件数・候補の範囲を足す(E4a Task 4)"
```

---

### Task 5: マイタスクの画面

**Files:**
- Create: `web/app/src/myTasks.ts`(`myTasks`・`myTaskBase` を作り、`reloads`・`itemChanges` に登録する。`myTaskBase` はいつも読み直し、`myTasks` と行のワークスペースの設定はマイタスクの画面を開いているときだけ読み直す(検索の `reloads` と同じ見分け)。行のワークスペースの設定の読み込み)、`web/app/src/screens/mytasks/MyTasksScreen.svelte`、`MyTaskGroups.svelte`、`MyTaskRow.svelte`、`web/e2e/my-tasks.spec.ts`、`web/e2e/my-tasks-helpers.ts`
- Modify: `web/app/src/shell/Screen.svelte`(`mytasks` の分岐)、`web/app/src/search.ts`(`createCandidates` に範囲の引数 `scope: () => CandidateScope`(初めは `all`)を足す)、`web/packages/ui/src/patterns/LayoutSwitch.svelte`(`label`)、`cmd/hedgehug/wire_e2e.go`(E4a-21)

**Interfaces:**
- Consumes: Task 1〜4 のすべて。`StatusButton`(`status`・`settings`・`project`・`disabled`・`onchange`)、`setRowStatus`、`detail.open`、`links.click`、`dueLabel`、`deletedNote`、`readOnlyNote`、`StoreBadge`、`KindIcon`、`FilterInput`、`SearchHelp`
- Produces: `MyTasksScreen.svelte` の引数 `{ ondrawer: () => void; alert: boolean }`。app の `myTasks: MyTasks`・`myTaskBase: MyTaskBase`・`myTaskSettings(storeId: string, workspaceId: string): WorkspaceSettings | undefined` と、読み終わりで描き直すための `myTaskSettingsLoaded: Readable<number>`

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/wire_e2e.go` に、閲覧のみの Store のサンプルの Task の担当者を taro にする関数を足し、`e2eStores` の並びに入れる(`readonlyProject` と同じ書き方)。

`web/e2e/my-tasks.spec.ts`(アイテムは `send` で `POST /api/stores/{e2eStore}/items`(サンプルのプロジェクト・Epic の下。`apiTask` と同じ本文)に作り、担当者・終了日・優先度・ステータスは `PUT /api/stores/{store}/items/{item}/properties/{property}`(本文 `{value}`。`property` は `assignee`・`end`・`priority`・`status`。優先度・ステータスの値は ID なので、`GET /api/stores/{store}/workspaces/{workspace}` の設定から名前で引く)で入れる。手順は `web/e2e/my-tasks-helpers.ts` にまとめる。絞り込みはテストごとに一意の印(`unique`)を入れて、ほかのテストのアイテムを除く。初めに `PUT /api/screen-states/my-tasks` を `{"filter":"","groupBy":"due"}` に戻す):
- `groups my tasks by due`: 担当 taro の Task を、終了日が昨日・今日・なしの 3 つ作る。サイドバーの「マイタスク」→ 見出し「マイタスク」と「あなたが担当しているタスク」。印で絞り込むと、見出し「期限切れ 1」「今日 1」「期限なし 1」がこの順に出る。昨日の行の期限は「期限切れ {日付}」。
- `switches grouping and keeps it`: まとめ方の「プロジェクト」(`getByRole("group", {name: "まとめ方"})` の中のボタン)を押すと、見出しがプロジェクトのタイトルになる。再読み込みしても「プロジェクト」が押された状態で、絞り込みも残る。
- `shows filter errors and empty states`: 絞り込み `status:` → 赤字「「status:」に値がありません」と「一致するものはありません」。印に一致しない語 → 「一致するものはありません」と「絞り込みを変えてみてください。」。
- `changes status from the row`: 行のステータスのボタン → メニューで完了の分類のステータスを選ぶと、その行が一覧から消える。
- `read-only rows cannot change status`: 絞り込み `ログイン画面を作る store:readonly` → 行のステータスのボタンが無効で、`title` が「閲覧のみの Store です」。
- `opens the detail panel from the title`: タイトルを押すと、そのアイテムの詳細パネルが開く。

- [ ] **Step 2: 失敗することを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/my-tasks.spec.ts`
Expected: FAIL(画面がない)

- [ ] **Step 3: 画面を組む**

- `MyTasksScreen.svelte`: `PageHead`(`myTasksTitle`・`myTasksNote`)、絞り込み欄(`FilterInput`。候補は `createCandidates` に `{kind: "myTasks", projects: myTaskProjects($base ?? [])}` を渡す。打ち方と候補の選び方は `SearchScreen.svelte` と同じ)、「検索ヘルプ」(`SearchHelp`)、まとめ方の切り替え(`LayoutSwitch` に `label="まとめ方"`・`myTasksGroupChoices`)、本体(`MyTaskGroups`)。`onMount` で `myTasks.open()`。
- `MyTaskGroups.svelte`: `groupMyTasks($view.rows, $view.groupBy, dayOf($today), $stores)`(`today` は Task 7 で作る。この Task では `syncNow` から `dayOf` で作り、Task 7 で `today` に替える)。行が 0 件のときの空の表示は、マイタスクの状態が `loaded` で、バッジの行(`$myTaskBase`)が null でないときだけ `myTasksEmpty($myTaskBase.length)` で出す(読み込み中や読めないときに「担当しているタスクはありません」と出さない)。見出しは `<h2>` に名前と件数、`warn` なら警告の色、`store` があれば前に `StoreBadge`。
- `MyTaskRow.svelte`: 種類のアイコン、ID、タイトル(`links.click(e, {name: "item", store, item: display}, () => void detail.open(storeId, itemId))`。削除済みの行はリンクにせず `deletedNote` を添える)、Store の点(`TableIdCell` と同じ書き方)とプロジェクトのタイトル、期限(`dueLabel(row.end, row.status?.category, $today)`)、優先度の名前、ステータスのボタン(`myTaskSettings` があれば `StatusButton`、なければ名前の文字。`disabled` は `row.readOnly || row.deleted !== undefined`)。ステータスを選ぶと `setRowStatus({items, toasts, reload: () => Promise.all([myTasks.reload(), myTaskBase.reload()])}, ...)`。
- `Screen.svelte`: `current?.name === "mytasks"` で `MyTasksScreen`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./cmd/... && pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/my-tasks.spec.ts e2e/tree-readonly.spec.ts e2e/notes-readonly.spec.ts e2e/detail.spec.ts e2e/project.spec.ts e2e/search-tree.spec.ts e2e/search-table.spec.ts`(閲覧のみの Store のサンプルを変えたため、それを開くテストも流す)
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add cmd web
git commit -m "feat(web): マイタスクの画面を足す(E4a Task 5)"
```

---

### Task 6: ナビとバッジ・パレット・ホームの置き換え・版の表示の削除

**Files:**
- Create: `web/e2e/last-screen.spec.ts`、`web/app/src/lastScreen.ts`(`createLastScreen({navigation, current: screen.current, screenStates}).start()` をモジュールを読んだときに 1 回呼ぶ。`App.svelte` から import する。`wire.ts` は 299 行なので触らない。`/` から移るのは、一覧を読む前でもよい: アドレスを替えておけば、`screen.start()` のあとの解決がその画面を出す)
- Modify: `web/app/src/shell/Sidebar.svelte`(ナビの先頭に「マイタスク」とバッジ)、`web/packages/core/src/features/palette/actions.ts`・`actions.test.ts`(「マイタスクを開く」)、`web/app/src/shell/Screen.svelte`(仮のホームと `version` をなくし、`home` のあいだは本文を空にする)、`web/app/src/App.svelte`(`system` と `version` をなくす)、削除: `web/packages/core/src/features/system/`・`web/packages/core/src/port/system.ts`・`web/packages/api/src/system.ts`(と公開の行)・`internal/adapter/httpapi/system/`(`routes.go` の登録と、ほかに使われなくなる `Deps.Version` も)、`internal/adapter/httpapi/server_test.go`・`internal/archtest/rules_test.go`(下の Step 1)、`api/openapi.yaml`・`generated.ts`(`make generate`)、`web/e2e/helpers.ts`(`tokenUrl` の初めのパス)、`web/e2e/frame.spec.ts`・`project-delete.spec.ts`・`workspace-delete.spec.ts`・`palette.spec.ts`(見出し「hedgehug」を「マイタスク」に)、`web/e2e/my-tasks.spec.ts`

**Interfaces:**
- Consumes: Task 2 の `createLastScreen`・`lastScreenKey`、Task 3 の `myTasksBadge`、Task 5 の `myTaskBase`

- [ ] **Step 1: 失敗するテストを書く**

`actions.test.ts`: `my tasks command follows new`: コマンドの群の 2 番目が「マイタスクを開く」で、実行すると `navigation.go({name: "mytasks"})`。

`web/e2e/last-screen.spec.ts`(E4a-20。`PUT /api/screen-states/last-screen` の本文は `{"body": "{\"path\":\"…\"}"}`):
- `opens the last screen`: ワークスペースの「スプリント」タブを開いてから `tokenUrl("/")` を開くと、「スプリント」タブが選ばれた画面が開き、アドレスは `/s/e2e/w/{接頭辞}/sprints`。
- `opens my tasks the first time`: `last-screen` を `null` にしてから `tokenUrl("/")` → 見出し「マイタスク」、アドレスは `/my-tasks`。
- `opens my tasks when the last screen is gone`(Review Focus 4): ワークスペースを作って概要を開き、API でそのワークスペースを削除してから `tokenUrl("/")` → 見出し「マイタスク」と、削除の理由のトースト(既存の `workspace-delete.spec.ts` の文)が 1 回。
- `unknown address opens my tasks`: `tokenUrl("/nope")` → 見出し「マイタスク」。

`my-tasks.spec.ts` に足す:
- `badge counts all my tasks`(Review Focus 2): サイドバーの「マイタスク」のバッジ(`getByRole("img", {name: /^未完了 \d+ 件$/})`)の数を読み、担当 taro の Task を 1 つ作ると 1 増える。マイタスクで印に一致しない語で絞り込んでも、バッジの数は変わらない。
- `palette opens my tasks`: 受信箱を開いてから Ctrl+K で「マイタスクを開く」を選ぶと、見出し「マイタスク」。

`frame.spec.ts` は `"/#token=e2e-token"` をそのまま開いているので、`tokenUrl()`(`/my-tasks`)を開く形に替え、版「dev」の確かめを、見出し「マイタスク」が出ることに替える(トークンが合わなければマイタスクの読み込みが 401 で出ない)。同じファイルの再読み込みとアドレスのテストも、開くアドレスが `/my-tasks` になった前提で期待を直す。

サーバー: `internal/adapter/httpapi/server_test.go` の `TestSystemNeedsToken` は `/api/system` でトークンを確かめているので、`TestAPINeedsToken` として `GET /api/stores` で確かめる形に替える(トークンなし・違うトークンは 401、正しいトークンは 200)。`internal/archtest/rules_test.go` の `internal/adapter/httpapi` → `internal/adapter/httpapi/system` の行は、`internal/adapter/httpapi/stores` に替える。

- [ ] **Step 2: 失敗することを確かめる**

Run: `pnpm -C web test -- actions && make e2e-bin && pnpm -C web exec playwright test e2e/last-screen.spec.ts e2e/my-tasks.spec.ts e2e/frame.spec.ts`
Expected: FAIL

- [ ] **Step 3: 実装する**

- サイドバー: ナビの先頭に `{name: "mytasks"}` へのリンク「マイタスク」。`$myTaskBase` が 1 件以上なら、受信箱と同じ形のバッジ(`role="img"`、`aria-label={myTasksBadge(n)}`)。いまの画面がマイタスクなら `aria-current="page"`。アプリを開いたときに `myTaskBase.reload()` を 1 回呼ぶ(以後は `reloads`)。
- パレット: `moveTo(d, "command:mytasks", g, "マイタスクを開く", {name: "mytasks"})` を「新規作成」の次に置く。
- 版: Interfaces の削除を行い、`grep -rn "getSystem\|SystemInfo\|system\.version\|Deps.Version"` で残りがないことを確かめる(`hh --version` が使う版の変数は残す)。

- [ ] **Step 4: 通ることを確かめる**

Run: `make generate && go test ./... && pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test`
Expected: PASS(e2e はすべて流す。`tokenUrl()` の初めのパスを替えたため)

- [ ] **Step 5: コミット**

```bash
git add internal api web
git commit -m "feat(web): ナビのマイタスクとバッジ・パレットのコマンド・最後の画面を足し、仮のホームと版の表示をなくす(E4a Task 6)"
```

---

### Task 7: 日付が変わったときの更新

**Files:**
- Create: `web/app/src/clock.ts`(`syncNow`・`today`・`tickClock` と 30 秒ごとの進め方を `wire.ts` から移す)、`web/e2e/day-change.spec.ts`
- Modify: `web/packages/core/src/features/sync/reflection.ts`・`reflection.test.ts`(`Reflection` に `queue(kind: SyncEventKind): void` を足す。中の `add` と同じく積んで `flush` する)、`web/app/src/wire.ts`(`clock.ts` を使い、`recheck` で `tickClock()`、`onDayChange(today, () => reflection.queue("store"))` を `startSync` に足す。`syncNow` は `clock.ts` から読み直して公開を保つ)、`web/app/src/screens/project/ProjectTree.svelte`・`ProjectOverview.svelte`・`web/app/src/screens/tree/SearchTreeBody.svelte`(`new Date()` を `$today` に)、`web/app/src/screens/table/ItemTable.svelte`(`now` を `rows` と `$today` で作り直す)、`web/app/src/screens/mytasks/MyTaskGroups.svelte`・`MyTaskRow.svelte`(Task 5 の仮の今日を `$today` に)

**Interfaces:**
- Consumes: Task 3 の `createToday`・`onDayChange`
- Produces: app の `today: Readable<Date>`(`clock.ts`)、`tickClock(): void`

- [ ] **Step 1: 失敗するテストを書く**

`reflection.test.ts`: `queue waits while held`: `held()` が true のあいだに `queue("store")` を呼んでも `apply` は呼ばれず、`held()` を false にして 0.5 秒進めると `apply(["store"])` が 1 回呼ばれる。`queue applies when free`: 待つ条件がなければ、すぐ `apply(["store"])`。

`web/e2e/day-change.spec.ts`(Playwright の `page.clock`。`page.clock.install({ time: new Date() })` を `goto` の前に呼ぶ):
- `my tasks regroup when the day changes`: 担当 taro・終了日が今日の Task を作り、印で絞り込んだマイタスクに「今日 1」が出る。`page.clock.fastForward("24:00:00")` のあと、入力欄の印はそのままで、見出しが「期限切れ 1」に替わる。
- `tree due label updates`: 同じ Task をプロジェクトのツリーで開き、行の期限が「今日 {日付}」。`fastForward("24:00:00")` のあと「期限切れ {日付}」。

- [ ] **Step 2: 失敗することを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/day-change.spec.ts`
Expected: FAIL(ツリーの期限が替わらない)

- [ ] **Step 3: 実装する**

E4a-16 のとおり。`wire.ts` は 300 行を超えないこと(`now` の writable と `setInterval` を `clock.ts` へ移して空けた行に足す)。`ItemTable.svelte` の `now` は `void rows; void $today; return new Date();`。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/day-change.spec.ts e2e/my-tasks.spec.ts e2e/search-table.spec.ts e2e/search-tree.spec.ts e2e/project.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 日付が変わったら、期限の表示とマイタスクを描き直し、日付で決まる結果を読み直す(E4a Task 7)"
```

---

### Task 8: 文書と、すべての確かめ

**Files:**
- Modify: `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`、`docs/superpowers/plans/2026-11-15-hedgehug-track-e1.md`(末尾「E4(マイタスクなど)」の各行に「E4a で足した」を添える)、`docs/superpowers/plans/2026-11-15-hedgehug-track-e3a.md`(末尾「E4(マイタスクと概要)」に「E4b に回した」を添える)、`docs/superpowers/plans/2026-11-13-hedgehug-track-a2.md`(決定 6 に「E4a で替えた」)、`docs/superpowers/plans/2026-11-13-hedgehug-track-a3a.md`(決定 22 と表の行に「E4a で足した」)、この計画の末尾

- [ ] **Step 1: 文書を直す**

- トラック E の分け方: 決めたことの表に「E-8 E4 は 2 本に分ける(E4a・E4b)。E4b は E2c のあと。ユーザーの決定(2026-11-15)」を足す。小計画の一覧の E4 の行を、E4a(この計画の Goal の中身。待つものは E3a)と E4b(ワークスペースの概要の統計 3 枚・プロジェクトの一覧と行のプロジェクトのメニュー・最近の変更・「期限切れ」カードからテーブルのタブ。待つものは E2c)の 2 行にする。「ほかのトラックから回された項目の行き先」の A2(決定 6)・A3a(決定 22)・A5c の「マイタスクを開く」を E4a に、F3 を E4b にする。
- この計画の末尾に「実装で計画から変えたこと」と「E4b に渡すもの」(概要の「期限切れ」カードの件数は、日付の変わり目の `reflection.queue("store")` で `reloads` と同じく読み直されること、`today` の場所、`LayoutSwitch` の `label`)を書く。

- [ ] **Step 2: develop を取り込み、「ホーム」への移り先が残っていないか確かめる**

`{name: "home"}` は型としては残る(`/` の意味)ので、ほかのトラック(E3b はビューの削除のあとなどに `replace({name: "home"})` を足している)が「マイタスクへ移る」つもりで書いた `home` は、型の検査を通ったまま、最後の画面を開く動きに変わる。`git merge origin/develop` のあと、`grep -rn '"home"' web/packages/core/src web/app/src` で、`route.ts` の `/` の解釈・`resolve.ts` の `home` の解決・`lastScreen.ts` 以外に、移り先として `home` を使っているところがあれば `mytasks` に直し、そのテストの期待も直す。

- [ ] **Step 3: すべてを確かめる**

Run: `make check && make e2e-bin && pnpm -C web exec playwright test`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add docs web
git commit -m "docs: トラック E の分け方を E4a・E4b に書き換える(E4a Task 8)"
```

## 実装で計画から変えたこと

- API: `web/packages/api/src/generated.ts` は `make generate` では作り直されない(作り直されるのは `api/openapi.yaml` だけ)。`pnpm -C web/packages/api gen` で作る。api の行の写しは `rowOf`(`search.ts`)に分け、マイタスクの口も使う。マイタスクの行は列を持たない(`collect` に列 nil)。
- 最後の画面: `last-screen` の `path` が `/`(home)に解釈されるときは、移り続けないようマイタスクへ移る。マイタスクに解決したときは、打ったアドレスではなく `/my-tasks` を覚える。
- `createMyTasks` は、画面の状態を読み終える前の `setFilter`・`setGroupBy` を積んでおき、読んだ値に重ねて 1 回書き、1 回問い合わせる(E3a-23 と同じ)。
- 削除済みの行の判定は `row.deleted !== null`(`deleted` は生きている行で null)。削除済みの行はステータスをボタンにせず文字で出す(ボタンの「閲覧のみの Store です」を出さないため)。マイタスクの画面の候補の元を Store の変化で読み直すため、`search.ts` に `reloadCandidates()` を足した。e2e の絞り込みの印は `unique` ではなく空白のない `prefixOf` で作る。
- `Deps.Version` は huma の設定(`huma.DefaultConfig`)が読むので残した。core の `model/system.ts` は削除した。
- サーバーの「最後の画面がもうない」とき理由のトーストが出るのは転送の記録があるとき(01)なので、e2e はワークスペースを別の Store へ移してから削除する形にした。転送の記録のない削除では、トーストなしでマイタスクを出す。最後の画面として開き直したアドレスがそうなったときは、アドレスも `/my-tasks` に替える(`lastScreen.ts` の `follow`)。解決できないアドレスを打ったときは、アドレスは打ったまま残る。
- `search.spec.ts` の「再読み込みで絞り込みが残る」は、絞り込みを含む PUT を待つ形に直した(`tokenUrl()` の初めのパスを替えたことで出た待ち方の不安定さ)。
- 時計は `web/app/src/clock.ts`(`syncNow`・`today`・`tickClock`。30 秒ごとの進め方はモジュールを読んだときに始める)。`wire.ts` は `export { syncNow } from "./clock"` で公開を保つ(299 行)。`Reflection.queue(kind)` を足し、日付の変わり目に `queue("store")` を呼ぶ。e2e は、絞り込み欄にフォーカスがあると読み直しを待つため、日付を進める前に欄からフォーカスを外す。

## E4b に渡すもの

- 今日の日付は `web/app/src/clock.ts` の `today`。概要の期限の判定もこれを使う。
- ui の `LayoutSwitch` は `label` を受け取る。概要など、画面が 2 つ以上のレイアウトの切り替えを持つときは、切り替えごとに名前を付ける。
- 概要の「期限切れ」カードの件数は、日付の変わり目の `reflection.queue("store")` で、`reloads` と同じく読み直される。カード側で日付を見張る仕組みは足さない。
- ワークスペースの概要のプロジェクトの一覧の行のプロジェクトのメニュー(04 の 84 行)は、E3a の `openProjectMenu`(`web/app/src/projectRows.ts`)を使う。
