# hedgehug トラック C4c1(上書きの検出と通知)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同時の編集で、このマシンが保存した値が負けたとき、このマシンの受信箱に「{項目名}が上書きされました」を項目ごとに 1 行出す。説明の同じ箇所を同時に編集して両方の版を残したとき(同期でも、このマシンの保存でも)は「説明の同じ箇所が同時に編集されたため、両方を残しました」を出す。上書きとウォッチの両方に当たるときは上書きだけにする。

**Architecture:** 負けた行の見つけ方は `change` の純関数 `Lost`(項目ごとの先端の比べ合い。保存形式 `:190`)。「このマシンの行」は行の `op` で決め、このマシンが作った保存の op を `state.db` の判定済みの表に `own` の印として残す(`judged/002`)。判定の差し込み口(C4b2 の `port.Received`)に `Own` を足し、`gitstore/saves` が、受け取った合わせのうち説明の同じ箇所が重なったもの(`Conflicts`)も渡す。通知を決めるのは `internal/app/noticing/overwrite*.go`。受信箱の通知は、C4c2 が値の表示と「自分の値に戻す」に使う項目(`Field`)・あなたの値(`Mine`)・残った値(`Kept`)を持つ(`inbox/004`)。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite、huma v2)、TypeScript・Svelte(web/packages/core・web/app)、vitest、Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:189-217`(同時に編集したとき・合わせ方の表・あなたの値が負けたとき)、`:219-228`(上書きの通知の本文と項目の表。「自分の値に戻す」の `:229-235` は C4c2)、`:363`(通知の表の上書き)、`:371`(閲覧のみでは作らない)、`:373`(上書きとウォッチ)、`:383`(判定する単位)、`:455`(行の対象)、`:488`(空のとき)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md` `:382`(説明の保存の重なり)、`03-workspace-settings.md` `:26`、`05-planning-reports.md` `:163`
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:140-160`(変更記録)、`:181`(説明の合わせ方)、`:190`(上書きは手順 3 の結果から作る)、`:300-310`(いまの値の決め方)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c4b1.md`(判定の差し込み口・判定済みの記録・`noticing`)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c4b2.md`(`port.Received`・合わせを渡す `saves.Pending`・`Notice.Section`・`Inbox.AddOnce`・`inbox/003`・`noticing.section`)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C4c1 の行

## 始める前に

- この計画は C4b2(ブランチ `feat/track-c4b2`)の口の上に書いた。**C4b2 が develop に入ってから**、develop から `feat/track-c4c` を作り直して(または develop を取り込んで)始める。Task 1(`change.Lost`)だけは C4b2 に触らないので、先に始めてよい。
- C4b2 が入ったら、実装の前に次の名前と番号を develop のコードと突き合わせ、違えばこの計画を直す: `port.Received`・`ReceivedSave.Merge`・`saves.Pending`・`pendingOf`・`markOwn` の引数・`noticing.section`/`sections`・`Inbox.AddOnce`・`Notice.Section`、表の手順の番号(`inbox/003` の次が `inbox/004`、`judged/001` の次が `judged/002`)。`docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` は両方のブランチが直しているので、取り込みで重なったら両方の行を残す。
- 実装で計画から変えたことは、この節の下に表で足す。

### 実装で計画から変えたこと

| # | 変えたこと | 理由 |
|---|---|---|
| 1 | Task 2 の表の名前は `judged_saves`(計画の `judged` は誤り) | statedb の実際の表の名前 |
| 2 | 判定がエラーのとき、受け取った未判定の保存より後のこのマシンの保存は `own = 0` のまま(次の同期で受け取った保存として判定される) | `MarkOwn` は判定済みと own を一緒に書くため、C4b2-5 の「後の保存は判定済みにしない」と両立しない。判定のエラーは state.db に書けないときだけで、C4c1-4 の「送った直後に止まった」と同じ扱い |
| 3 | remote が空のときは、自分の先端から辿れる保存をすべて `own = 1` にする | いちばんふつうの場面(このマシンで作った Store を初めて空のリポジトリへ送る)で正しい。別の remote で受け取った保存がある Store を空のリポジトリへ付け替えたときだけ、他のマシンの保存が own になる |
| 4 | 説明の重なりは、合わせの結果に `log.jsonl` が残るエンティティだけを見る(同期が消したもの・別の Store へ移ったものの下は見ない) | 同期の `removedEntities`・`underTransferred` と同じ範囲にする。消えたものに通知を出さない |
| 5 | 共通の祖先は合わせごとに 1 回だけ求め、`mergeRemoved` と `mergeConflicts` で共有する(`saves/base.go`) | 両方が同じ祖先を使う |
| 6 | `workspace.Valid`・`member.Valid` を domain に足し、負けの判定の前に手順 0 の検査で行を落とす | いまの値の決め方が捨てる行で負けを判定しないため |
| 7 | `fieldName(r, b, ref, f)` に判定の batch を渡す | 読み済みのログを読み直さない |
| 8 | 1 つの保存で複数の値に負けた複数の値の項目は、最後の勝ちの行の `Source`・差出人で 1 行 | 06 :221(差出人は上書きした人) |
| 9 | `overwritten` は `Judge` が作り、上書き → ウォッチの順に渡す | C4c1-14 |
| 10 | e2e: 受信箱の「上書き」タブが空であることを確かめるテストを `web/e2e/inbox-empty.spec.ts` に分け、`fixtures.ts` の worker 単位の `ownServer` で専用のサーバーで流す | `description-merge.spec.ts` が上書きの行を残すため。空のタブはサーバー全体の受信箱で決まるので、専用のサーバーでしか確かめられない |
| 11 | 上書きの開く区画の頭(`workflow/`・`choices/`・`initial_status`・`category_default/`・`initial_priority`・`template_default/`)は、同期の修復の通知の開く区画にも効く | 06 :455 に合う |
| 12 | このマシンの説明の保存が成功したあとに受信箱へ書けなかったときは、`slog.Warn` に残して保存の結果を返す(エラーにしない) | 説明の合わせはやり直すと両方の版の塊がもう一度入る(冪等でない)。受信箱に書けないのは state.db に書けないときだけ |

## Global Constraints

土台・トラック C・C4a・C4b1・C4b2 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個(`internal/archtest/size.go`)。`internal/app/port`・`internal/app/apptest`・`internal/infra/gitstore` は 20 個で満杯なので、ファイルを足さない。gitstore の新しい処理は `internal/infra/gitstore/saves` に置く。web は .svelte 200 行、.ts 300 行。
- 口は git の言葉を使わない(`port` の型の名前とコメントに「コミット」「ブランチ」「マージ」を書かない。「保存」「合わせ」と書く)。
- app の機能どうしは import しない(`descriptions` は `noticing` を import しない。受信箱は `port.Inbox` で渡す)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 通知の本文(06 のとおり):
  - 「{項目名}が上書きされました」({項目名} は決定 C4c1-9 の表)
  - 説明: 「説明の同じ箇所が同時に編集されたため、両方を残しました」(web の `descriptionConflictMessage` と同じ文)
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・`make web-check`・`description-merge.spec.ts` の e2e。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C4c1-1 | 範囲: 負けた値の検出と「上書き」の通知(本文だけ)。通知に項目・あなたの値・残った値を保存する。受信箱の「あなたの値 X → 残った値 Y」と「自分の値に戻す」は C4c2。検索の `is:overwritten` はトラック E | ユーザーの決定(2026-11-15) |
| C4c1-2 | 上書きの行は、項目ごとに 1 行。1 つの保存で 2 つの項目に負けたら 2 行。06 `:383` の「上書き、同期の修復: 保存ごとに 1 行」を「上書きは項目ごとに 1 行(別の保存の負けもまとめない)。同期の修復は保存ごとに 1 行」に直す | ユーザーの決定(2026-11-15)。06 `:221-228` の本文と値の表示が 1 項目ごとのため |
| C4c1-3 | 負けの判定(`change.Lost`): 項目(`set` は f、`add`・`remove` は f と値、`comment`・`worklog` は f)ごとに、判定の前(いまのログから判定する行を除いたもの。C4b1-6)の勝ちの行 M と、判定の後(いまのログ)の勝ちの行 W を決める(勝ちの決め方は `change.Resolve`・`change.Current` と同じ。コメントと作業時間は生きている行を先にする)。M がこのマシンの行で、M が判定の後でも先端(W の `prev` から辿れない)で、W が判定する行で、W ≠ M のとき負けとする。値が同じ(`set` は `to` が同じ、`add`・`remove` は同じ種類)なら作らない。M が削除(コメント・作業時間の `to` が null)なら作らない(削除と変更の重なりは C4b2 の取り消しの通知) | 06 `:189`・`:205-206`、保存形式 `:190`・`:304-306`。判定の前に M がすでに負けていたら、そのときに作った。W が M を見て書いた(prev に M がある)なら同時ではない |
| C4c1-4 | 「このマシンの行」は、行の `by` が「あなた」(大文字と小文字を区別しない)で、行の `op` がこのマシンで作った保存の op であるもの。このマシンで作った保存は、同期のたびに、自分の先端から辿れて remote の先端(今回受け取ったもの)から辿れない、`Hedgehug-Op` を持つ、同期の修復でない保存(まだ送っていない保存)として見分け、`judged` の表に `own = 1` で残す(`judged/002`)。見分けるときは判定済みかどうかで止めない(最初の印付け(C4b1-3)が判定済みにした未送信の保存も数える)。判定にも、記録した op と、今回見分けた op の両方を渡す。この版より前に送った保存と、clone で受け取った保存は `own = 0`(通知しない) | 06 `:217`(負けたマシンが作る。同じ人の別のマシンの値が負けても、このマシンには作らない)。行はマシンを持たないので、保存の出自をこのマシンに覚える。送ったあとプロセスが止まって次の同期で受け取った保存として判定された自分の保存は `own = 0` になる(C4b1-5 と同じく、止まったときだけ起きるので受け入れる) |
| C4c1-5 | 作らないもの: 閲覧のみの Store(`in.ReadOnly`)。「あなた」を決められない Store。勝った行の実行者が `github`(PR のリンクの状態・タイトル・レビュー)か `sync`(同期の修復は C4b2 の通知)のもの。項目が `watchers`(ウォッチ中の人)・`links/` で始まる `set`(リンクの状態・タイトル・レビュー)。負けた行の実行者が `user` でも `automation:…` でもないもの。別の Store から移ってきたエンティティ(C4b1-19) | 06 `:199`(ウォッチは勝ち負けが起きない)、`:213`(PR のリンクの状態は通知しない)、`:371`。自動化の行の `by` は操作した人なので、自分の操作で動いた自動化の値は「あなたの値」 |
| C4c1-6 | 移動は場所と順位を同じ保存で書くので、同じエンティティで場所と順位の両方に、同じ op の行が勝ったときは「場所」の 1 行だけにする | 06 `:191`(移動は場所と順位を同じ保存で書く)。順位だけが負けたとき(並べ替え)は「順位」 |
| C4c1-7 | 差出人は勝った行の人(`noticing.sender`。自動化の行は空で「hedgehug」)。あなたの別のマシンに負けたら差出人はあなた | 06 `:217`・`:221` |
| C4c1-8 | 通知の対象: アイテムのログ → `Item`(と `Display`)、プロジェクトのログ → `Project`(と `Display`)、ワークスペースのログ → `Workspace` と `Section`。ただし `project_rules/<プロジェクトの内部 ID>/…` は `Project`(06 `:455` のプロジェクト別のルール)。メンバーのログ → `Workspace` も `Item` も `Project` も空で、受信箱は新しい対象の種類 `store`(Store の点と Store 名。リンクなし。メンバーの表示名を変える画面は G が作る)。`Section` は C4b2 の `noticing.section` に `workflow/` → `workflow`、`choices/` → `properties`、`initial_status`・`category_default/` → `statuses`、`initial_priority` → `priorities`、`template_default/` → `templates` を足したもの | 06 `:455` の当てはめ。サブタブの ID は web の `workspaceSettingsTabs`。06 `:455` に「メンバーの表示名など、ワークスペースにも属さないものは、Store の点と Store 名を出す」を足す |
| C4c1-9 | {項目名}の表(ここにない項目は、アイテム・プロジェクト・メンバーのログでは作らず、ワークスペースのログでは「ワークスペースの設定」)。名前は、いまの名前(削除済みなら `workspace.NameOfDeleted` の名前、引けなければ内部 ID)。「{名前} の{部分}」は 06 の例「Review の名前」に合わせ、名前のあとに空白を入れる。<br>アイテム: `title` タイトル、プロパティ(組み込みを含む。`item.Field(p)`)はプロパティ名、`sprint` スプリント、`release` リリース、`place` 場所、`rank` 順位、`kind` 種類、`relations` 関連、`links`・`links_off` リンク、コメント コメント、作業時間 作業時間。<br>プロジェクト: `title` タイトル、`status` ステータス、`owner` オーナー、`start` 開始日、`end` 終了日、`repos` リポジトリ、`place` 場所、`rank` 順位、コメント コメント。<br>ワークスペース: `name` ワークスペース名、`prefix` 接頭辞、`statuses|priorities|systems/<ID>/<部分>` と `properties/<ID>/<部分>` と `choices/<プロパティ>/<ID>/<部分>` は「{値・プロパティ・選択肢の名前} の{部分}」(部分: `name` 名前、`color` 色、`rank` 並び順、`category` 分類、`type` 種類、`kinds` 使う種類、`hidden` 表示。ほかは「設定」)、`workflow/…` ワークフロー、`initial_status` 初期ステータス、`initial_priority` 初期優先度、`category_default/…` 既定のステータス、`template_default/…` 既定のテンプレート、`templates/<ID>/…` テンプレート「{名前}」、`rules/<ID>/…` 自動化「{名前}」、`sprints/<ID>/<部分>` 「{スプリント名} の{部分}」(`name` 名前、`state` 状態、`start` 開始日、`end` 終了日、`goal` 目標)、`releases/<ID>/<部分>` 「{リリース名} の{部分}」(`name` 名前、`state` 状態、`date` 日付、`description` 説明)、`project_rules/<プロジェクトの内部 ID>/…` 「{プロジェクトの ID} のルール」。<br>メンバー: `name/<ログイン名>` 「{ログイン名} の表示名」、`members` メンバー | 06 `:221`(ワークスペースの設定などは {項目名} にその名前を入れる。例: 「ワークスペース名」「Review の名前」)、`:228`(関連・リンク・プロジェクト別のルール)。テンプレートと自動化のルールは中の項目が多いので、1 つの名前にまとめる |
| C4c1-10 | 通知に保存する値(C4c2 が使う): `Field`(ログの f。コメント・作業時間は `comment:<ID>`・`worklog:<ID>`)、`Mine`(判定の前の値)、`Kept`(判定の後の値)。どちらもログに書く形の JSON。値を 1 つ持つ項目は勝ちの行の `to`(create なら作成の値)、複数持つ項目は `change.ListOf` の配列(値の JSON の辞書順)、コメントと作業時間は行の `to`。`inbox/004` で列 `field`・`mine`・`kept` を足す | 06 `:474`(いまの値が残った値のままか、複数の値は集合で比べる)。整形した文字列は持たない(名前の変更で古くなる) |
| C4c1-11 | 1 回だけにする鍵: `Notice.Source` を `overwrite:{エンティティの内部 ID}:{Field}:{勝った行の ID}`(説明は `overwrite:desc:{エンティティの内部 ID}:{合わせの版の印}`、このマシンの保存の重なりは鍵なしで `Add`)にし、`Inbox.AddOnce` で書く | C4b1-5 の書きかけのやり直しで重ねない。「自分の値に戻す」のあとにまた負けたときは勝った行が違うので、新しい行になる(06 `:233`) |
| C4c1-12 | 説明の同期の重なり: 合わせの 2 つの親と共通の祖先の `description.md` を、同期と同じ規則(`textmerge.Merge`)で合わせ直し、同じ箇所が重なったものを `ReceivedSave.Conflicts` で渡す(合わせは C4b2 の `saves.Pending` の (c) に「説明が重なった合わせ」を足して渡す。作ったのがどちらのマシンでも)。各側の行は、その側の親の `log.jsonl` のうち、共通の祖先の同じエンティティのログにない `desc` の行。どちらかの側の行にこのマシンの行(C4c1-4)があれば、その人(あなた)に 1 行。差出人は、もう一方の側の `desc` の行のうち `at` が最後のものの人。本文から `-----` を探さない(Markdown の水平線と区別できない) | 06 `:200`(両方の人に作る)。先に送った側のマシンは、相手の合わせを受け取るだけなので、合わせ直さないと分からない。`mergeDescription` は重なりを返さない |
| C4c1-13 | 説明のこのマシンの保存の重なり: `descriptions.save` で `Conflict` が真で保存したとき、保存した本人の受信箱に 1 行(鍵なしの `Add`)。差出人は、保存の中で読んだいまの説明の最後の `desc` の行の人(作成・複製で `desc` の行がなければ `create` の行の人)。対象はアイテムまたはプロジェクト | 02 `:382`、06 `:215`(保存した本人にトーストと、同じ文の上書きの通知) |
| C4c1-14 | 上書きとウォッチ: 同じ判定で上書きを作った(エンティティ、項目)は、ウォッチの本文を作らない。ウォッチの項目はタイトルとプロパティで、`Field` が同じなら同じ項目とする | 06 `:373`、C4b1-18 |
| C4c1-15 | 2 台の場面は `cmd/hedgehug` の本物の gitstore と state.db で確かめる(C4b1-17)。画面で見えるこのマシンの保存の重なり(2 つのタブ)は、既存の `description-merge.spec.ts` に受信箱の「上書き」の行を確かめる手順を足す | 同期は e2e のビルドで再現できない。このマシンの保存の重なりは e2e で再現できる |

## Review Focus

1. **同じ人の別のマシンの値:** あなたのノート PC の値を先に受け取り、そのあと同時の田中さんの値を受け取ったデスクトップ PC には作らない(ノート PC にだけ出る。Task 2 の `TestOwnOnlyThisMachine`、Task 8 の場面)。
2. **送ったあとで負けた:** 前の同期で送り終えた自分の値が、次の同期で届いた相手の同時の値に負けたら作る(Task 2 の `TestOwnAfterPush`、Task 8 の場面)。
3. **同時でない変更と同じ値:** 相手があなたの値を見てから変えた(prev にあなたの行がある)ときと、同じ値にしたときは作らない(Task 1 の `TestLostSequential`・`TestLostSameValue`)。
4. **説明の両方を残した:** どちらのマシンが先に送っても、両方のマシンの受信箱に 1 行ずつ出る。別の行の変更なら出ない(Task 3 の `TestMergeConflicts`、Task 8 の場面の 2 つの順番)。
5. **時計のずれ:** あなたの値の `at` があとで勝ったら、あなたのマシンには作らず、相手のマシンに作る(Task 1 の `TestLostClockOrder`、Task 8 の場面)。
6. **ウォッチとの重なり:** ウォッチ中のアイテムのステータスで負けたら、上書きの 1 行だけ(Task 6 の `TestOverwriteSkipsWatch`)。

---

### Task 1: 負けた行の見つけ方(`change.Lost`)

**Files:**
- Create: `internal/domain/change/lost.go`
- Test: `internal/domain/change/lost_test.go`

**Interfaces:**
- Produces:
  - `type Loss struct { T Type; F string; Mine Record /* 負けた行 */; Winner Record /* 勝った行 */ }` — `T` は `Set`・`Add`(`add`・`remove` の項目)・`Comment`・`Worklog`
  - `func Lost(log []Record, fresh func(Record) bool, mine func(Record) bool) []Loss` — 決定 C4c1-3。`fresh` は判定する行か。並びは F、Mine の ID の順
  - `func ListOf(log []Record, f string) []json.RawMessage` — 複数の値を持つ項目 f のいまの値(`Resolve` の `Lists[f]` と同じ値と並び。作成の値を含む)

- [ ] **Step 1: 失敗するテストを書く**(行は `Record` を直接組む。`prev` で同時かどうかを作る)
  - `TestLostConcurrentSet`: 作成 C、M(by hanako、at 10:00、prev [C]、status A)、W(by taro、at 10:01、prev [C]、status B、fresh)→ `[]Loss{{T: Set, F: "status", Mine: M, Winner: W}}`
  - `TestLostSequential`: W の prev が [M] → 空
  - `TestLostSameValue`: W の to が M と同じ → 空
  - `TestLostClockOrder`: M の at が W よりあと → 空(M が勝つ)
  - `TestLostAlreadyLost`: M と同時の古い行 O(fresh でない、at 10:02)に M が判定の前から負けている → 空
  - `TestLostNotMine`: mine が M に false → 空
  - `TestLostList`: M = `remove systems "API"`、W = `add systems "API"`(同時、W があと)→ `T: Add, F: "systems"` の 1 件。両方 `add` なら空
  - `TestLostComment`: 同じコメント ID の M(本文 x)と W(本文 y、あと)→ `T: Comment` の 1 件。M の to が null(削除)なら空。W の to が null(相手の削除)なら M が生きているので空
  - `TestListOf`: 作成の値 `["Web","API"]` と `remove "API"` → `["\"Web\""]`
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/change/ -run 'Lost|ListOf'` Expected: FAIL(`Lost` がない)
- [ ] **Step 3: 実装する** — `groupAll`・`tipsOf`・`winner` を使う。コメントと作業時間の勝ちは `Current` と同じく、生きている先端を先にする
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/change/` Expected: PASS(既存のファジングを含む)
- [ ] **Step 5: コミット**

```bash
git add internal/domain/change/lost.go internal/domain/change/lost_test.go
git commit -m "feat(change): 同時の変更で負けた行を見つける(C4c1)"
```

---

### Task 2: このマシンで作った保存の記録(`judged/002`)と `Received.Own`

**Files:**
- Modify: `internal/app/port/syncstate.go`(`JudgedSaves.MarkOwn`・`Own`)、`internal/app/port/replicator.go`(`Received.Own`)
- Modify: `internal/infra/statedb/judged.go`、Test: `internal/infra/statedb/judged_test.go`
- Modify: `internal/app/apptest/syncstate.go`(偽物と `RunJudgedSavesContract`)
- Modify: `internal/infra/gitstore/saves/judge.go`、Test: `internal/infra/gitstore/saves/judge_test.go`(なければ作る。gitstore の同期のテストに置くなら `internal/infra/gitstore/sync_judge_test.go`)

**Interfaces:**
- Produces(`port`):
  - `JudgedSaves.MarkOwn(ctx, store StoreID, keys []SaveKey) error` — `Mark` と同じく判定済みにし、`own = 1` にする(すでにある行も `own = 1` にする)
  - `JudgedSaves.Own(ctx, store StoreID, op change.ID) (bool, error)` — その op の行に `own = 1` があるか。op が空なら false
  - `Received.Own func(op change.ID) (bool, error)` — このマシンで作った保存の op か(決定 C4c1-4)。閲覧のみの同期では常に false。nil は「どれも違う」
- Produces(`saves`): `Judge` が `in.Own` を渡し、判定のあと自分の保存を `MarkOwn` する。エラーのときの `markOwn` も `MarkOwn` で書く

- [ ] **Step 1: 契約のテストを足す(`RunJudgedSavesContract`)**
  - `MarkOwn` のあと `Own(op)` が true、`Mark` だけの op は false、別の Store は false
  - `Mark` のあとに `MarkOwn` しても true。`MarkOwn` のあとに `Mark` しても true のまま
- [ ] **Step 2: gitstore のテストを足す**(2 つのクローン A・B。C4b1 の同期のテストの手助けを使う)
  - `TestOwnAfterPush`: A で保存 S1 → 同期(送る)→ B で保存 → B を同期 → A を同期。A の判定に渡る `in.Own(S1 の op)` が true、B の保存の op は false
  - `TestOwnPendingSave`: A で保存 S2(まだ送っていない)→ B の保存を受け取る同期。判定の中で `in.Own(S2 の op)` が true
  - `TestOwnFirstSync`: 判定の記録がない A(この版で初めての同期)で保存 S3 → 同期。最初の印付けで S3 が判定済みになっても、判定のあと `Own(S3 の op)` が true
  - `TestOwnOnlyThisMachine`: B で保存した op は、A が受け取ったあとも A の `Own` で false
  - `TestOwnReadOnly`: 閲覧のみの同期で `in.Own` が常に false
- [ ] **Step 3: 失敗を確かめる** — Run: `go test ./internal/infra/statedb/ ./internal/app/apptest/ ./internal/infra/gitstore/... -run 'Judged|Own'` Expected: FAIL
- [ ] **Step 4: 実装する** — `register("judged/002", "ALTER TABLE judged ADD COLUMN own INTEGER NOT NULL DEFAULT 0")`。`saves.Judge` は、自分の先端から辿れて remote の先端から辿れない、Op を持つ、`Repair` でない保存の op の集まりを作り(辿るのは remote の先端の祖先で止め、判定済みでは止めない。C4c1-4)、`in.Own` は「集まりにある、または `judged.Own`」。判定のあと `mark(head)` に続けて、その集まりの鍵を `MarkOwn` する
- [ ] **Step 5: 通ることを確かめる** — Run: `go test ./internal/infra/statedb/ ./internal/app/apptest/ ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 6: コミット**

```bash
git add internal/app/port/syncstate.go internal/app/port/replicator.go internal/infra/statedb/judged.go internal/infra/statedb/judged_test.go internal/app/apptest/syncstate.go internal/infra/gitstore/saves/judge.go internal/infra/gitstore/
git commit -m "feat(sync): このマシンで作った保存を覚えて判定に渡す(C4c1)"
```

---

### Task 3: 説明が重なった合わせを判定に渡す(`ReceivedSave.Conflicts`)

**Files:**
- Modify: `internal/app/port/replicator.go`(`DescConflict`・`ReceivedSave.Conflicts`)
- Create: `internal/infra/gitstore/saves/conflicts.go`、Test: `internal/infra/gitstore/saves/conflicts_test.go`
- Modify: `internal/infra/gitstore/saves/removed.go`(`pendingOf` が説明の重なった合わせも渡す)

**Interfaces:**
- Produces(`port`):
  - `type DescConflict struct { Ref change.Ref; Sides [2][]change.Record /* 各側で、共通の祖先のあとに足された desc の行 */ }`
  - `ReceivedSave.Conflicts []DescConflict` — 合わせのときだけ持つ
- Produces(`saves`): `func mergeConflicts(c *object.Commit) ([]port.DescConflict, error)` — 2 つの親の共通の祖先がなければ空

- [ ] **Step 1: 失敗するテストを書く**(go-git のメモリ上のリポジトリで、祖先・2 つの親・合わせを組む)
  - `TestMergeConflicts`: 祖先の説明「a\nb\nc」、親 0 が 2 行目を「x」、親 1 が 2 行目を「y」に変え、それぞれ `desc` の行を足す → `Conflicts` が 1 件、`Sides[0]`・`Sides[1]` がそれぞれの `desc` の行
  - `TestMergeConflictsOrderFree`: 親を入れ替えても重なりの有無は同じ
  - `TestMergeNoConflict`: 親 0 が 1 行目、親 1 が 3 行目を変えた → 空
  - `TestMergeOneSide`: 片方だけが変えた → 空
  - `TestPendingPassesConflictMerge`: 消えたエンティティがなく説明だけが重なった合わせを `Pending` が `Merge: true` で返す
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/saves/ -run 'Merge|Pending'` Expected: FAIL
- [ ] **Step 3: 実装する** — 共通の祖先は、同期の合わせと同じく `親 0.MergeBase(親 1)` の最初のもの(`gitstore/replicator.go` の `merge` と同じ選び方。合わせの親は [自分, remote] の順)。片側に `description.md` がなければ空の本文として扱う(`mergeDescription` と同じ)。両方の親で `description.md` が祖先と違い、親どうしも違うものを `textmerge.Merge(祖先, 親 0, 親 1)` にかけ、`conflict` が真なら 1 件。`Ref` はパスから(`log.jsonl` と同じフォルダ)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/replicator.go internal/infra/gitstore/saves/
git commit -m "feat(sync): 説明の同じ箇所が重なった合わせを判定に渡す(C4c1)"
```

---

### Task 4: 受信箱の項目・値の列(`inbox/004`)と対象 `store`

**Files:**
- Modify: `internal/app/port/inbox.go`(`Notice.Field`・`Mine`・`Kept`)
- Modify: `internal/infra/statedb/inbox.go`(300 行を超えるなら書き込みを `inbox_write.go` へ)、Test: `internal/infra/statedb/inbox_test.go`
- Modify: `internal/app/apptest/inbox.go`、Test: `internal/app/apptest/inbox_test.go`
- Modify: `internal/app/inbox/target.go`(`TargetStore`)、Test: `internal/app/inbox/target_test.go`
- Modify: `internal/adapter/httpapi/inbox/dto.go`(対象の種類 `store`)、`api/openapi.yaml`(生成)
- Modify: web の受信箱の行の対象の表示(`web/packages/core` の受信箱の対象の型と `web/app/src/screens` の受信箱の行)、Test: vitest

**Interfaces:**
- Produces(`port`): `Notice.Field string`・`Notice.Mine json.RawMessage`・`Notice.Kept json.RawMessage`(上書きだけ。決定 C4c1-10)
- Produces(`inbox`): `TargetStore TargetKind = "store"` — `Item`・`Project`・`Workspace` がすべて空の通知。Store の点と Store 名を出し、リンクにしない

- [ ] **Step 1: 失敗するテストを書く**
  - statedb: `TestInboxOverwriteValues`: `AddOnce(Kind: overwrite, Field: "status", Mine: "\"A\"", Kept: "\"B\"")` → `List` で同じ値。古い行(列を足す前)は空
  - apptest: 同じ値を返す
  - target: `TestTargetStore`: 3 つとも空の通知 → `Kind == TargetStore`
  - vitest: 対象 `store` の行が「● {Store 名}」で、リンクでない
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/statedb/ ./internal/app/apptest/ ./internal/app/inbox/` Expected: FAIL
- [ ] **Step 3: 実装する** — `register("inbox/004", "ALTER TABLE inbox ADD COLUMN field TEXT NOT NULL DEFAULT ''; ALTER TABLE inbox ADD COLUMN mine TEXT NOT NULL DEFAULT ''; ALTER TABLE inbox ADD COLUMN kept TEXT NOT NULL DEFAULT ''")`。`resolve` は 3 つとも空なら `TargetStore`(`workspaceTarget` より先に見る)。`make generate` で `api/openapi.yaml` を作り直し、web の API の型を作り直す
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/... && make web-check` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/inbox.go internal/infra/statedb/ internal/app/apptest/ internal/app/inbox/ internal/adapter/httpapi/inbox/ api/openapi.yaml web/
git commit -m "feat(inbox): 上書きの項目と値を保存し、Store を指す行を出す(C4c1)"
```

---

### Task 5: 上書きの通知(`noticing/overwrite*.go`)

**Files:**
- Create: `internal/app/noticing/overwrite.go`(判定と通知)、`internal/app/noticing/overwrite_names.go`(決定 C4c1-9 の表と `Section`)
- Modify: `internal/app/noticing/service.go`(`Judge` から呼ぶ)、`internal/app/noticing/repair_restore.go`(`sections` に C4c1-8 の頭を足す)
- Test: `internal/app/noticing/overwrite_test.go`、`internal/app/noticing/overwrite_names_test.go`、`internal/app/noticing/fixture_test.go`(`in.Own` を渡せるようにする)

**Interfaces:**
- Consumes: Task 1 の `change.Lost`・`change.ListOf`、Task 2 の `Received.Own`、Task 4 の `Notice.Field`・`Mine`・`Kept`、C4b2 の `Inbox.AddOnce`・`Notice.Section`・`section`
- Produces: `func overwrites(r port.Reader, b batch, store port.StoreID, me string, own func(change.ID) (bool, error)) ([]port.Notice, error)`、`func fieldName(r port.Reader, ref change.Ref, f string) (name string, ok bool, err error)`、`type overwritten map[change.Ref]map[string]bool`(Task 6 が使う。作った (エンティティ, Field))

- [ ] **Step 1: 失敗するテストを書く**(fixture でアイテムを作り、`save` で「このマシンの保存」と「受け取った保存」を作る。`in.Own` は前者の op だけ true)
  - `TestOverwriteItemStatus`: 自分が status を InProgress、同時に taro が Done(あと)→ 1 行。`Text == "ステータスが上書きされました"`・`Sender == "taro"`・`Item`・`Display == "HH-1"`・`Field == "status"`・`Mine` が InProgress の ID・`Kept` が Done の ID・`Source == "overwrite:{内部 ID}:status:{W の ID}"`
  - `TestOverwriteTwoFields`: タイトルとステータスの両方で負ける → 2 行(C4c1-2)
  - `TestOverwriteSystems`: 06 `:225` の例。前「Web, API」、自分が API を外し、taro が同時に API を足し直す → 1 行「関連システムが上書きされました」・`Mine == ["Web"]`・`Kept == ["API","Web"]`(値の JSON の辞書順)
  - `TestOverwriteMoveOnlyPlace`: 移動(place と rank)で負ける → 「場所が上書きされました」の 1 行
  - `TestOverwriteSkips`(表で): 閲覧のみ・`Own` が false・勝った行の実行者が `github`・`sync`・項目が `watchers`・`links/…/state` → 0 行
  - `TestOverwriteAutomationSender`: 勝った行の実行者が自動化 → `Sender == ""`
  - `TestOverwriteWorkspace`(表で): ステータス Review の名前 → 「Review の名前が上書きされました」・`Section == "statuses"`、`name` → 「ワークスペース名が上書きされました」・`general`、`workflow/A` → 「ワークフローが上書きされました」・`workflow`、スプリント「Sprint 25」の終了日 → 「Sprint 25 の終了日が上書きされました」・`sprints`、`rules/R/name` → 「自動化「{名前}」が上書きされました」・`automation`、`project_rules/P1/hidden_statuses` → 「HH-P1 のルールが上書きされました」・`Project == P1`
  - `TestOverwriteProject`: プロジェクトのオーナー → 「オーナーが上書きされました」・`Project`・`Display == "HH-P1"`。プロジェクトのコメントの編集 → 「コメントが上書きされました」・`Project`
  - `TestOverwriteComment`: 自分のコメントの編集が taro の同時の編集に負ける → 「コメントが上書きされました」・`Item`・`Field == "comment:{ID}"`
  - `TestOverwriteMemberName`: `name/hanako` → 「hanako の表示名が上書きされました」・`Workspace`・`Item`・`Project` が空
  - `TestOverwriteOnce`: 同じ判定を 2 回 → 1 行(`AddOnce`)
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/ -run Overwrite` Expected: FAIL
- [ ] **Step 3: 実装する** — `Judge` は `!in.ReadOnly` のとき `overwrites` を呼び、`AddOnce` で書く。`mine` は「`by` があなた、実行者が人か自動化、`own(op)`」。`fresh` は `b.judged`。プロパティ名はアイテムのいまのワークスペースの設定(削除済みのプロパティは `workspace.NameOfDeleted`)。`fieldName` が false の項目は作らない
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/noticing/` Expected: PASS(C4b1・C4b2 のテストを含む)
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing/
git commit -m "feat(noticing): 負けた値から上書きの通知を作る(C4c1)"
```

---

### Task 6: ウォッチを飛ばす・説明の重なりの通知(同期とこのマシンの保存)

**Files:**
- Modify: `internal/app/noticing/service.go`(`judgeIn` に `overwritten` を渡す)、`internal/app/noticing/watch.go`
- Create: `internal/app/noticing/overwrite_desc.go`、Test: `internal/app/noticing/overwrite_desc_test.go`
- Modify: `internal/app/descriptions/service.go`(`Inbox port.Inbox`)・`save.go`、Test: `internal/app/descriptions/descriptions_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Descriptions` に `notifier` を渡す)
- Test: `internal/app/noticing/watch_test.go`

**Interfaces:**
- Consumes: Task 3 の `ReceivedSave.Conflicts`、Task 5 の `overwritten`
- Produces: `func descConflicts(r port.Reader, b batch, store port.StoreID, me string, own func(change.ID) (bool, error)) ([]port.Notice, error)`、`descriptions.Service.Inbox port.Inbox`

- [ ] **Step 1: 失敗するテストを書く**
  - `TestOverwriteSkipsWatch`: ウォッチ中のアイテムのステータスで負ける → 上書きの 1 行だけ。同じ判定でタイトルは負けずに変わっていればウォッチの「タイトルを…」は出る
  - `TestDescConflictNotice`: `Merge: true` の保存に `Conflicts`(側 0 にこのマシンの `desc` の行、側 1 に taro の行)→ 1 行。本文は C4c1-12、`Sender == "taro"`、`Field == "desc"`、`Source == "overwrite:desc:{内部 ID}:{合わせの版の印}"`。どちらの側にもこのマシンの行がなければ 0 行。閲覧のみなら 0 行
  - `TestSaveConflictNotifies`(descriptions): base「a\nb」で保存の中のいまの説明が「a\nx」(taro の `desc`)、入力が「a\ny」→ 受信箱に 1 行(`Sender == "taro"`・`Item`・`Display`)。重ならない保存では 0 行
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/ ./internal/app/descriptions/` Expected: FAIL
- [ ] **Step 3: 実装する** — ウォッチは `overwritten[ref][item.Field(p)]`・`overwritten[ref]["title"]` のとき飛ばす。`descriptions.save` は Write が成功したあと、`res.Conflict` なら `Inbox.Add`(差出人は保存の中で読む。C4c1-13)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/... ./internal/adapter/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing/ internal/app/descriptions/ internal/adapter/httpapi/routes.go
git commit -m "feat(noticing): 説明の重なりを知らせ、上書きした項目のウォッチを飛ばす(C4c1)"
```

---

### Task 7: 2 台の場面テストと e2e

**Files:**
- Test: `cmd/hedgehug/notices_overwrite_test.go`(C4b1 の 2 台の場面テストの手助けを使う)
- Modify: `web/e2e/description-merge.spec.ts`

- [ ] **Step 1: 場面テストを書く**(本物の gitstore と state.db、マシン A・B・C。A と C は同じ人 hanako)
  - A がステータスを InProgress、B(taro)が同時に Done(あと)。A が先に送る順と B が先に送る順の両方で、A の受信箱に「ステータスが上書きされました」が 1 行、B には 0 行
  - A の値が送られたあと、C(hanako の別のマシン)が A の値を受け取り、そのあと B の値を受け取る → C には 0 行、A には 1 行(Review Focus 1)
  - A の時計が進んでいて A が勝つ → B に 1 行、A に 0 行(Review Focus 5)
  - 説明の 2 行目を A と B が同時に変える。A が先に送る順と B が先に送る順の両方で、A と B に 1 行ずつ(Review Focus 4)
- [ ] **Step 2: e2e を足す** — `description-merge.spec.ts` の 2 つのタブの場面のあと、受信箱の「上書き」タブに「説明の同じ箇所が同時に編集されたため、両方を残しました」の行が 1 つある
- [ ] **Step 3: 通ることを確かめる** — Run: `go test ./cmd/hedgehug/ -run Overwrite && make e2e-bin && (cd web && pnpm exec playwright test e2e/description-merge.spec.ts)` Expected: PASS
- [ ] **Step 4: コミット**

```bash
git add cmd/hedgehug/notices_overwrite_test.go web/e2e/description-merge.spec.ts
git commit -m "test: 上書きの通知を 2 台の場面と e2e で確かめる(C4c1)"
```

---

### Task 8: 仕様と計画の記録、全体の検証

**Files:**
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`(`:383` を C4c1-2 の文に、`:455` に C4c1-8 の 1 文)
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C4c2 の行に「C4c1 の `Notice.Field`・`Mine`・`Kept` を使う」、「G への申し送り」の表に 2 行「メンバーの表示名を変える画面ができたら、受信箱の対象 `store` をその画面へのリンクにする」「保持日数をログに入れるときは、C4c1-9 の表に `retention` 保持日数を足す」、新しい節「E への申し送り」に「`is:overwritten` は、受信箱の未読の上書きの通知のうち `Item` があるもの(このマシンの `InboxBook`)で決める。閲覧のみの Store とアイテムを指さない上書きは 0 件(04 `:1082`)」)
- Modify: この計画の「始める前に」(実装で変えたこと)

- [ ] **Step 1: 文書を直す**(上の 3 か所)
- [ ] **Step 2: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./... && make web-check` Expected: すべて成功
- [ ] **Step 3: コミット**

```bash
git add docs/
git commit -m "docs: C4c1 の決定を仕様と計画に記録する"
```
