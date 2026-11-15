# hedgehug トラック C4d(アクティビティと履歴の「同期で受信」)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 表示中の Store の保存のコミットと同期の行を新しい順に並べる「アクティビティ」の画面を作る(絞り込み・300 件ずつ・旧 ID のリンク・メンバーの追加の 1 人 1 行・パレットの「アクティビティを開く」)。あわせて、詳細パネルの「履歴」の行のうち同期で受け取った変更に「同期で受信」と添える。

**Architecture:** git の版だけが持つ新しい口 `port.StoreCommits` が、Store のコミットを新しい順に、第 1 親との差分で足された行と一緒に返す(`infra/gitstore/saves` の差分の処理を使い、SHA ごとに覚える)。同期の行は新しい口 `port.SyncLog` で `state.db` に書く。新しいパッケージ `internal/app/activity` が、コミット 1 つから行(本体・自動化・メンバーの追加)を作り、Store をまたいで並べ、人でない実行者の重なりを 1 行にまとめ、絞り込みと 300 件ずつの区切りを行う。画面は受信箱と同じ形(core の状態・api の口・app の画面)で作る。「同期で受信」は、`judged_saves` のうち、このマシンで作っていない保存の op で決める。

**Tech Stack:** Go 1.27.3(go-git v5、huma v2、modernc.org/sqlite)、TypeScript・Svelte 5(web/packages/core・api・ui、web/app)、vitest、Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:495-577`(アクティビティの全節)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md` `:492`(履歴の「同期で受信」)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/01-overview.md` `:132`(日付が変わったら「今日」「昨日」を更新)、`:163`(アドレスは画面だけ)、`:195`(ナビの並び: マイタスク・受信箱・アクティビティ・検索)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` の「アクティビティと履歴」「remote の履歴が書き換えられたとき」の 3
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C4d の行、`2026-11-13-hedgehug-track-c2.md` の C2-5、`2026-11-14-hedgehug-track-a5a.md` の A5a-11、`2026-11-14-hedgehug-track-b5c.md` の C4 への申し送り、`2026-11-14-hedgehug-track-c3a.md` の C3a-18、`2026-11-14-hedgehug-track-a5c.md` の「受信箱を開く」「アクティビティを開く」、`2026-11-13-hedgehug-track-a4a.md` の「同期で受信」、`2026-11-15-hedgehug-track-e1.md` の E1-17

## 始める前に

- develop(C4c2 の PR #224・C5b1 の PR #225・E2a の PR #226 を含む)から `feat/track-c4d` を作ってある。
- 実装で計画から変えたことは、この節の下に表で足す。

### 実装で計画から変えたこと

| # | 変えたこと | 理由 |
|---|---|---|
| 1 | Task 1: 契約 `RunStoreCommitsContract` は `apptest/storelist.go` に置いた。StoreLog は共有の契約に入れず、偽物と saves のテストで確かめる。偽物の Op は整列後の行から決め、StoreLog は 1 回の Write の前後で比べる。テスト用の包み `apptest.RetentionAfterWrite` を足した | `commits.go` が 300 行を超え、apptest は 20 ファイルで満杯だった。保持日数を変える口がまだなく、契約から StoreLog を確かめられない |
| 2 | Task 2: `saves.History(repo, from, recall, remember)`(`seen map` をやめた)。`Commit.At` は、コミットの op と同じ op の行の `at`(ミリ秒)。そういう行がないときだけ作者の時刻。差分は 1 回だけ取る(`diffOf`) | 作者の時刻は秒までしか持たず、同じ秒の保存の並びが保存の順と食い違う。覚える場所は呼び出し側が持つ |
| 3 | C4d-5: 行が 1 行もなく作者が bot のコミットは、件名が「保持日数を過ぎた削除済み 」で始まれば実行者なし、それ以外は同期 | 完全な削除の記録(`purged.jsonl`)は実行者を持たない。仕様の固定の文なので、G は文どおり保存するだけでよい |
| 4 | C4d-7: 別の Store へ子ごと移したアイテムの移動元の行は、ID を出さない | 消したあとは親が読めず、根が決まらない |
| 5 | C4d-11: 重なりの鍵に実行者の種類を含める | GitHub と同期の同じ値の変更を、1 行にまとめてしまう |
| 6 | C4d-14・Task 3 の `Row.Key`: 鍵は `{SHA}:{6 桁の 0 埋めの連番}`(最終レビューで 4 桁から 6 桁に)。API の行は `cursor`(`{UTC RFC3339 ミリ秒}\|{鍵}`、`activity.Cursor`)を持ち、画面は最後の行の cursor を `before` に渡す。`Query` に `Limit` はない。知らない種類・読めない `before`・Store なしのワークスペースは `ErrInvalidOperation`(API は 422) | 連番を文字列で比べるため桁をそろえる。続きの印を画面が組み立てずに済む |
| 7 | C4d-13: 同期の行の書き込みに失敗しても、同期は成功として続ける(`slog.Warn`)。同期の行の時刻はミリ秒に切る | 同期の行は記録にすぎない。続きの印がミリ秒なので、行の時刻も合わせる |
| 8 | C4d-15: 選んでいたワークスペースが一覧から消えたら「すべて」に戻す。`store` と `sync` の出来事による読み直しは、同じ tick で 1 回にまとめる。いまの ID のリンクは内部 ID を引いて詳細パネルを開き、引けなければアドレスへ移る | 消えたワークスペースで 0 件のまま動けなくなる。出来事が続くと読み直しが重なる |
| 9 | Task 6: 履歴の `Row` に `Actor`(実行者の種類)も足した。`/e2e/receive` の書き先は、`wire` が返す `port.JudgedSaves` を `wrap` に渡す(`wire` のシグネチャは `(Deps, JudgedSaves, error)`) | 「同期」の行に添えない判定(C4d-16)に実行者が要る。e2e の入口が本物と同じ `judged_saves` に書くため |
| 10 | Task 7: 種類の選択肢は `activityKindOptions`(`kindOptions` は既存の名前と重なる)。読み直しは、1 回の読みで行が増えなければ止める | 名前の重なり。`more` が偽のまま読み続けないため |
| 11 | Task 10: e2e の helper `apiTask`・`renameByApi`・`receiveMark` を足した | 履歴の e2e の準備を 1 か所にまとめるため |
| 12 | 最終レビュー: C4d-11 の鍵に、行の `Prev`(並べたもの)を入れる。行のないコミットは、件名に加えてフォルダごと消したエンティティ(並べたもの)を入れる。Store と実行者(自動化はルールの ID まで)は今のまま | 全履歴で同じ鍵の最も早い 1 行だけを残すと、順に起きた繰り返し(自動化が Todo→Done、人が戻す、自動化がまた Done)の 3 つ目が消える。06 `:538` がまとめるのは複数のマシンから届いたものだけ |
| 13 | 最終レビュー: `activity.Service` に `Versions`(`port.StoreVersions`)と `Memo`(ポインタ)を足し、Store の版の印が変わらなければ、重なりをまとめたあとのコミットの行をそのまま使う(`Stores.Read` も `Commits` も呼ばない)。Store 名と色は覚えず、いまの一覧の値を入れる。同期の行は毎回読む。1 回の作り直しの中では、アイテム・プロジェクトを ID ごとに覚える。`httpapi.Deps` に `Versions` を足し、`wire.go`・`wire_e2e.go` は Store を渡す | 要求ごとに全履歴から行を作り直し、そのあいだ Store のロックを持っていた。同期の行は版を変えずに足されることがある(送るだけの同期・e2e の入口) |
| 14 | 最終レビュー: コミットの行の時刻をミリ秒に切る | 同期の行と同じく、秒以下を持つコミットが続きの印で次のページから抜ける |
| 15 | 最終レビュー: gitstore が覚える保存のコミットは、説明の全文(`EntityRows.Description`)を持たず、行のないエンティティを落とす | アクティビティは説明を使わない。SHA ごとに覚えるので、全文を抱え続ける |
| 16 | 最終レビュー: `/e2e/sync-log` は、足したあと本物の同期と同じく出来事 `sync` を出す | 通しのテストで、絞り込みを変えずに同期の行が出ることを確かめるため |

## Global Constraints

土台・トラック C の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個(`internal/archtest/size.go`)。いま 20 個で満杯なのは `infra/gitstore`・`app/port`・`app/apptest`・`app/transfers`・`app/items`・`app/workspaces`。300 行に近いのは `port/stores.go`(300)・`apptest/stores.go`(299)・`apptest/syncstate.go`(298)・`saves/saves.go`(295)。web は .svelte 200 行・.ts 300 行(`app/src/wire.ts` は 298 行なので、Task 9 の 1 行のほかは足さない)。
- app の機能どうしは import しない(`activity` は `history`・`inbox`・`transfers` などを import しない)。infra は app を import できないので、app で宣言する口は構造的型付けで満たす(前例 `app/reports.Selections`)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- e2e のビルドだけで使う関数は `//go:build e2e` のファイルに、本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e のビルドも lint する(`golangci-lint run --build-tags e2e ./...`)。
- API を変えたら `make generate` のあと `pnpm --dir web gen` で `generated.ts` を作り直す。
- 文言(06・02 のとおり):
  - 見出し「アクティビティ」、補足「Store の git 履歴。1 つの変更が 1 つのコミットです」
  - 絞り込みのラベル「ワークスペース」「種類」。ワークスペースの選択肢「すべて」「{Store 名} / {ワークスペース名}」。種類の選択肢「すべて」「人の変更だけ」「同期だけ」「自動化だけ」
  - 日付の見出し「今日」「昨日」「YYYY/MM/DD」。時刻「HH:MM」
  - 実行者の名前「同期」「自動化」「GitHub」「不明なユーザー」。実行者のない行は名前を出さずアイコン「hh」、自動化のアイコン「A」
  - 同期の行「origin と同期 (送信 {N} 件・受信 {M} 件)」
  - メンバーの追加「メンバー {ログイン名} を追加」(A5a-11)
  - 「さらに読み込む」、0 件「この条件のコミットはありません」
  - ナビ・パレット「アクティビティ」「アクティビティを開く」
  - 履歴の添え書き「同期で受信」
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint・`make web-check`・`activity.spec.ts`・`activity-empty.spec.ts`・`history.spec.ts`・`palette.spec.ts` の e2e。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C4d-1 | 範囲: 06「アクティビティ」の全節、同期の行の記録(C2-5)、旧 ID のリンク、メンバーの追加の 1 人 1 行(A5a-11)、移動元にないワークスペースを選択肢に出さない(B5c)、パレットの「アクティビティを開く」(A5c)、サイドバーの「アクティビティ」(E1-17)、履歴の「同期で受信」(A4a)。remote の履歴にそろえたのせ直しのコミットと、保持日数を過ぎたものの完全な削除の保存はトラック G が書く。アクティビティは C4d-5・C4d-6 の一般の規則で出すので、G で足すものはない | トラック C の C4d の行。「同期で受信」はユーザーの決定(2026-11-15) |
| C4d-2 | コミットの読み方: 新しい口 `port.StoreCommits.Commits(ctx, store)` が、HEAD から辿れるコミットのうち、親が 1 つで `Hedgehug-Op` を持つものを、作者の時刻の新しい順(同じなら SHA の辞書順)にすべて返す。各コミットは SHA・件名・op・作者名・作者の時刻と、第 1 親との差分で足された行(`port.EntityRows`、`saves.entities` と同じ処理)、フォルダごと消えたエンティティ(`Removed`。別の Store への移動の移動元・完全な削除)、Store 全体の記録(`.hedgehug/log.jsonl`)を変えたか(`StoreLog`)を持つ。マージと最初のコミット(「Store を作成」)は返さない。読んだコミットは SHA ごとに `gitstore.Stores` のメモリに覚え、2 回目からは差分を取らない。ロックは HEAD を読むあいだだけ持ち、コミットを辿るあいだは持たない(コミットは書き換わらない) | 保存形式「アクティビティと履歴」(1 コミット 1 行、マージは出さない)。06 の重なりの 1 行は「最も早い保存」を残すので、ページの外にある古い保存まで見ないと決まらない。コミットは書き換わらないので SHA で覚えてよい |
| C4d-3 | 行に使う変更記録: コミットの op と同じ op の行。1 行もなければ(同期の修復の (2)、remote の履歴にそろえたのせ直し)、そのコミットで足されたすべての行。対象とワークスペースを決めるエンティティは、その行を持つものと `Removed` のもの | 保存形式「アクティビティと履歴」、C3a-18 |
| C4d-4 | コミット 1 つから作る行: (1) 本体 1 行(文はコミットの件名)。(2) 自動化の行(`actor` が `automation:<ID>`)ごとに 1 行、文は `automation.ActivityText(ルール名, automation.ChangeText(プロパティ名, 値))`(`app/automations` の最近の実行と同じ。ルール名は対象のワークスペースのログから `automation.Names`、値は `item.Display`)。(3) コミットが members 以外のエンティティにも行を足していて、members の `add` の行があれば、その行ごとに「メンバー {ログイン名} を追加」を 1 行(実行者は本体と同じ)。members だけのコミット(メンバーの追加・変更の操作)は本体の件名だけ。(2)(3) は本体と同じ時刻・SHA | 06 `:503`(自動化の変更)、A5a-11。メンバーだけの操作は件名がすでに「メンバー…」なので、二重に出さない |
| C4d-5 | 本体の実行者: C4d-3 の行のうち自動化でない最初の行の `actor`(人はその `by`)。そのような行がなければ、作者名が `hedgehug[bot]` なら「同期」(ただし、件名が「保持日数を過ぎた削除済み 」で始まるものは実行者なし)、それ以外は作者名の人 | 修復 (2) は作者が bot(C3a-18)、のせ直しは作者が「あなた」(保存形式) |
| C4d-6 | 種類の絞り込み: 人の変更だけ = 実行者が人の行(同期で受け取った他の人の保存、のせ直しを含む)。同期だけ = 同期の行と実行者「同期」の行。自動化だけ = 実行者が自動化と GitHub の行。実行者のない行(`none`)は「すべて」だけ | 06 `:507`・`:536`・`:553-556` |
| C4d-7 | 対象: アイテムは、C4d-3 の行を持つアイテムが 1 つならそれ、2 つ以上なら、その中で親がその中にない(いまの親で見る)ものがちょうど 1 つのときだけそれ(テンプレートでまとめて作った親、子ごとの移動)。一括変更のように根が 2 つ以上なら出さない。Store にないアイテム(別の Store へ子ごと移した)は親が読めないので、移動元の行は ID を出さない。プロジェクトは、行を持つプロジェクトが 1 つならそれ、なければ対象のアイテムのいまのプロジェクト。自動化の行の対象は、その行のアイテム | 06 `:571`。ユーザーの決定(2026-11-15): 一括変更のように根が複数なら ID を出さない(本文に件数がある) |
| C4d-8 | 対象の表示: Store にあり削除済みでない → いまの ID(プロジェクトはタイトルと ID)とリンク。削除済み → いまの ID を文字だけ。Store にない → 転送記録(`Transfers().List()`、同じ `Target` の最後の行)の `Old` を旧 ID のリンク(`locate-item`・`locate-project`)。転送記録にもなければ完全に削除の記録(`Purged().List()`)の `Display` を文字だけ。どれにもなければ出さない | 06 `:571`(削除済みでなければリンク)、A5a の C4 への申し送り(旧 ID は `locate-item` で開く) |
| C4d-9 | ワークスペース: 行が属するワークスペースは、行を持つエンティティの「いまの」ワークスペース(ワークスペースはそれ自身、プロジェクト・アイテムは Store にあるもの(削除済みを含む)の `Workspace`)の集合。Store にないエンティティ(別の Store へ移した・完全に削除した)はワークスペースを持たない。ワークスペースが 1 つも決まらず、members の行があるか、`StoreLog` が true か、本体の実行者がなし(`none`)のコミットは「Store 全体」の印を持つ(06 `:551` のメンバーの追加・変更、保持日数の変更、保持日数を過ぎたものの完全な削除)。ワークスペースの絞り込みでは、選んだワークスペースを集合に含む行、同じ Store の同期の行、同じ Store の「Store 全体」の行を出す。行に出すワークスペース名は、集合が 1 つのときだけその名前 | 06 `:551`・`:559`(別の Store へ移した記録は移動先に属し「すべて」でだけ見える)。ユーザーの決定(2026-11-15): 同じ Store の別のワークスペースへ移したアイテムの古い行は移動先で出す。保存形式「アクティビティと履歴」の移したワークスペース |
| C4d-10 | ワークスペースの選択肢は、画面がサイドバーの Store の一覧(catalog)から作る。いまの Store にあるワークスペースだけなので、移動元にないワークスペースは出ない | B5c の申し送り、06 `:551` |
| C4d-11 | 重なりの 1 行: 実行者が人でない行のうち、重なりの鍵が同じものは、時刻が最も早い行(同じなら SHA の辞書順で前)だけ残す。鍵は Store と実行者(種類。自動化はルールの ID まで)と、本体なら C4d-3 の行の (エンティティ・項目・from・to・Prev) の集合(行がないときは件名と、フォルダごと消したエンティティの集合)、自動化の行・メンバーの追加の行ならその 1 行の (エンティティ・項目・from・to・Prev)。06 がまとめるのは複数のマシンから届いた同じ変更だけで、2 台が同じ版から作った変更は Prev が同じ、順に起きた繰り返しは Prev が違う。同じものを 2 度消すことはないので、別のときの同じ件名の完全な削除はまとまらない。300 件はまとめたあとで数える | 06 `:538`、保存形式「人でない実行者の同じ変更」 |
| C4d-12 | 実行者の名前はサーバーが決める: 人はその Store のメンバーの表示名(大文字小文字を区別せずに探す。いなければ「不明なユーザー」でアバターは中身のない丸)、自動化「自動化」、GitHub「GitHub」、同期「同期」、なしは空 | 06 `:569-570`、01「不明なユーザー」。履歴と同じくサーバーで作る |
| C4d-13 | 同期の行: 送信と受信の合計が 1 以上の成功した同期ごとに、新しい口 `port.SyncLog.Add(ctx, store, at, sent, received)` で `state.db` の表 `sync_log` に書く。書くのは `syncing` の `run`(手動・すべて・自動)と `deliver`(書き込みのあとの同期)の `Succeeded` の直後。時刻は同期を終えた時刻(`Succeeded` に渡す時刻)をミリ秒に切ったもの。書けなくても同期は成功として続ける(`slog.Warn`)。Store をこのマシンから削除したときに消すため `ForgetStore` を持つ(呼ぶのは G の削除。`JudgedSaves.ForgetStore` と同じ) | 06 `:509`、C2-5、保存形式「このマシンに持つもの」 |
| C4d-14 | 並びと区切り: Store ごとの行と同期の行を、時刻の新しい順、同じなら鍵(コミットの行は `{SHA}:{6 桁の 0 埋めの連番}`、同期の行は `sync:{ID}`)の辞書順の逆で並べる。API の行は `cursor`(`{UTC RFC3339 ミリ秒}|{鍵}`)を持ち、`before`(前のページの最後の行の `cursor`)を受け、その後ろの 300 件と、さらにあるか(`more`)を返す。画面の読み直しは、受信箱の `again` と同じく、先頭から 300 件ずつ、出していた件数を覆うまで読む(何回「さらに読み込む」を押したあとでも同じ) | 06 `:575`(300 件、日付をまたいで数える)。受信箱の続きの印と同じ形 |
| C4d-15 | 画面: アドレス `/activity`。サイドバーのナビは「受信箱」と「検索」のあいだ。絞り込みは画面の状態で、アドレスにも Store にも入れない。日付の見出しと時刻はこのマシンの時刻帯で、「今日」「昨日」は `syncNow` の日付で決めて日付が変わったら更新する。行のアイテムの ID は、いまの ID なら詳細パネルで開き、旧 ID ならアドレス `{ name: "item", store, item: 旧 ID }` を開く(転送記録から探す)。プロジェクトは `{ name: "project", store, project: ID, tab: "overview" }`。読み直しは出来事 `store`・`github`・`sync` で(`github` は既存の `reloads` の仕組みで `store` と一緒に呼ばれる)、アクティビティの画面を開いているときだけ。同じ tick の出来事は 1 回にまとめる。選んでいたワークスペースが一覧から消えたら「すべて」に戻す | 06 `:563-571`、01 `:132`・`:163`・`:195`、受信箱の `linkOf` と同じ |
| C4d-16 | 「同期で受信」: 履歴の行の op が、その Store の `judged_saves` にあり、同じ op でこのマシンで作った印(`own = 1`)のものがないとき。新しい口は app/history が宣言する `ReceivedOps`(`Received(ctx, store, ops []change.ID) (map[change.ID]bool, error)`)で、`statedb.DB` のメソッドが満たす。remote のない Store、まだ同期していない保存は記録がないので添えない。Store を追加したときの clone で入った保存は「受信」に数える(ユーザーの決定、2026-11-15。最初の同期のあとから添える)。実行者が「同期」の行(同期の修復)には添えない(このマシンの修復の保存は `own = 1` にならない(`TestOwnExcludesRepair`)ので材料で区別できず、実行者の名前「同期」が同期から来たことをすでに示す) | 02 `:492`。`judged_saves` の `own` はこのマシンで作った保存の印(C4c1-4)で、最初の同期の印付けのあとも `Judge` が送っていない自分の保存を `own = 1` にする |
| C4d-17 | e2e: 同期は e2e で再現できないので、e2e のビルドだけに入口 `POST /e2e/sync-log`(本文 `{store, sent, received}`)と `POST /e2e/receive`(本文 `{store, item}`。そのアイテムの最後の行の op を、このマシンで作っていない保存として `judged_saves` に書く)を足す。保存のコミットは、偽物 `apptest.CommitLog`(`port.Stores` を包み、`Write` の前後で行を比べてコミットを記録する)が持つ | `/e2e/notice` と同じ形(C4c2-12) |

## Review Focus

1. **時計が進んだマシンの保存:** 未来の時刻の保存も、その時刻で先頭に並ぶ(06 `:565`)。並びは作者の時刻で、コミットの順ではない(Task 4 の `TestPageOrdersByTime` に未来の時刻の行を入れる)。
2. **同じ変更が 2 台から届く:** 2 台が同時に作った「スプリント「Sprint 25」を計画中に戻す」(実行者「同期」)と、同じ GitHub の取り直しが 1 行ずつになり、SHA は早い方。人の保存の同じ値は 2 行のまま(Task 4 の `TestPageMergesNonHumanDuplicates`)。
3. **移動したもの:** 別の Store へ移したアイテムの、移動元に残る行は、旧 ID のリンクで出て、ワークスペースの絞り込みでは「すべて」だけに出る。同じ Store の別のワークスペースへ移したアイテムの古い行は、移動先のワークスペースで出る(Task 3 の `TestRowsMovedOut`・`TestRowsMovedWithinStore`)。
4. **非表示の Store と閲覧のみ:** 非表示の Store の行も同期の行も出ない。閲覧のみの Store の行は出る(Task 4 の `TestPageHiddenStore`)。
5. **さらに読み込む中に保存が増える:** 2 ページ目を読む前に新しい保存が入っても、続きの印 `before` で読むので、行が重ならず抜けない(Task 4 の `TestPageCursorStable`)。
6. **長い履歴:** 数千コミットの Store で、2 回目からの読み取りは差分を取り直さず、辿るあいだに保存がロックで待たされない(Task 2 の `TestHistoryUsesSeen`・`TestCommitsDoesNotHoldLock`)。

---

### Task 1: コミットの口と偽物(`port.StoreCommits`・`apptest.CommitLog`)

**Files:**
- Modify: `internal/app/port/storelist.go`(`StoreVersions` の下に `Commit` と `StoreCommits`)
- Modify: `internal/app/apptest/clock.go`(`identity.go` の中身をそのまま移す。apptest のファイル数を 1 つ空けるため。中身は変えない)
- Delete: `internal/app/apptest/identity.go`
- Create: `internal/app/apptest/commits.go`(偽物 `CommitLog` と、テスト用の包み `RetentionAfterWrite`)
- Modify: `internal/app/apptest/storelist.go`(契約 `RunStoreCommitsContract`)
- Test: `internal/app/apptest/commits_test.go`

**Interfaces:**
- Produces:
  ```go
  // Commit は、保存のコミット 1 つ(C4d-2)。
  type Commit struct {
      SHA     string
      Summary string
      Op      change.ID
      Author  string
      At      time.Time
      Changes []EntityRows // 第 1 親との差分で足された行
      Removed []change.Ref // フォルダごと消えたエンティティ
      StoreLog bool        // .hedgehug/log.jsonl を変えた
  }
  // StoreCommits は、Store の保存のコミットの口(C4d-2)。git の版だけが持つ。
  type StoreCommits interface {
      // Commits は、HEAD から辿れる、親が 1 つで Hedgehug-Op を持つコミットを、At の新しい順(同じなら SHA の辞書順)で返す。
      Commits(ctx context.Context, store StoreID) ([]Commit, error)
  }
  ```
  - `type CommitLog struct { port.Stores; Clock port.Clock }`、`func NewCommitLog(s port.Stores, c port.Clock) *CommitLog`、`func (c *CommitLog) Commits(ctx, store) ([]port.Commit, error)`。`Write` は内側の `Write` の前後で、members・ワークスペース・プロジェクト・アイテム(`List` で辿れるもの)の行の ID を比べ、足された行を `Changes`、`List` で辿れなくなったエンティティを `Removed`、`RetentionDays` の変化を `StoreLog` にして 1 件記録する。SHA は通し番号の 40 桁の 16 進、Author は人なら `Login`・ほかは `change.BotLogin`、At は足された行の `At`(行がなければ Clock)、Op は、整列後の行のうち実行者が操作の実行者と同じ最初の行の op。どれも変わらなければ記録しない(本物は空の保存をコミットしない)
  - `RetentionAfterWrite{Stores; Days}` は、`Write` のあとで保持日数を `Days` にする(偽物の `Write` の中からは保持日数を変えられないため)。
  - `func RunStoreCommitsContract(t *testing.T, newStores func(t *testing.T) (port.Stores, port.StoreCommits, port.StoreID))`

- [ ] **Step 1: 失敗するテストを書く** — `storelist.go` の契約に次のケースを書き、`commits_test.go` から `NewCommitLog(NewStores(...), clock)` で流す。
  - `TestCommitsNewestFirst`: 3 回の保存(アイテムの作成、タイトルの変更、members への `member.Join("hanako")`)のあと、`Commits` が 3 件を新しい順に返し、`Summary` が各 `Operation.Summary`、`Op` が足した行の op、`Changes` の `Ref` がアイテム(2 件)・members(1 件)。
  - `TestCommitsEmptyWriteNotRecorded`: 行のない `Write` のあと件数が増えない。
  - `TestCommitsRemoved`: アイテムを `Remove` し、`WriteTransfers` で転送記録だけを書いた保存が 1 件記録され、`Changes` は空、`Removed` がそのアイテム。`StoreLog` は保持日数を変える口がまだないので共有の契約に入れず、偽物のテスト(`RetentionAfterWrite` で 1 回の `Write` の前後を比べる)と saves のテストで確かめる。
  - `TestCommitsBotAuthor`: 実行者 `change.Actor{Kind: change.ActorGitHub}` の保存の `Author` が `"hedgehug[bot]"`、人の保存は `Login`。
- [ ] **Step 2: 落ちることを確かめる** — `go test ./internal/app/apptest/ -run Commits` → 未定義で FAIL。
- [ ] **Step 3: 口と偽物を書く**(上の Interfaces のとおり)。
- [ ] **Step 4: 通ることを確かめる** — `go test ./internal/app/apptest/ ./internal/archtest/` → PASS(ファイル数 20 以内)。
- [ ] **Step 5: コミット** — `git commit -m "feat(port): 保存のコミットを新しい順に返す口と、その偽物を足す(C4d Task 1)"`

### Task 2: gitstore のコミットの一覧(`saves.History`・`Stores.Commits`)

**Files:**
- Create: `internal/infra/gitstore/saves/history.go`
- Modify: `internal/infra/gitstore/versions.go`(`Commits` メソッド)、`internal/infra/gitstore/stores.go`(`Stores` に覚える場所 `commits map[plumbing.Hash]port.Commit`)
- Test: `internal/infra/gitstore/saves/history_test.go`、`internal/infra/gitstore/contract_test.go`(`RunStoreCommitsContract` を流す)

**Interfaces:**
- Consumes: Task 1 の `port.Commit`・`port.StoreCommits`
- Produces: `func History(repo *git.Repository, from plumbing.Hash, recall func(plumbing.Hash) (port.Commit, bool), remember func(plumbing.Hash, port.Commit)) ([]port.Commit, error)`(`recall` が返すコミットは差分を取らずに使い、新しく読んだものは `remember` で覚えさせる。`Commit.At` は、コミットの op と同じ op の行の `at`(ミリ秒)。そういう行がないときだけ作者の時刻)。`func (s *Stores) Commits(ctx context.Context, id port.StoreID) ([]port.Commit, error)`(`open(root)` でロックを取り、`s.mu` で覚える場所を守る)

- [ ] **Step 1: 失敗するテストを書く** — `history_test.go`(本物の go-git のリポジトリ):
  - `TestHistorySkipsMergeAndFirst`: 最初のコミット(op なし)・保存 2 つ・マージ(親 2 つ)を作ると、返すのは保存 2 つだけ。
  - `TestHistoryOrdersByAuthorTime`: 作者の時刻が逆順になるように 2 つの保存を作る(先に作った方が未来の時刻)と、時刻の新しい順に返る。
  - `TestHistoryAddedRows`: 2 つ目の保存の `Changes` は、その保存で足した行だけ(1 つ目の行を含まない)。
  - `TestHistoryUsesSeen`: `recall` が返すコミットの値(わざと違う `Summary`)がそのまま返る。
  - `TestHistoryRemovedAndStoreLog`: アイテムのフォルダを消したコミットの `Removed` がそのアイテム、`.hedgehug/log.jsonl` に行を足したコミットの `StoreLog` が true。
  - `TestCommitsDoesNotHoldLock`(gitstore): `Commits` が辿っているあいだ(辿る関数に差し込み口を置く)に `Write` が終わる。
  - `contract_test.go` で `apptest.RunStoreCommitsContract` を本物で流す。
- [ ] **Step 2: 落ちることを確かめる** — `go test ./internal/infra/gitstore/...` → FAIL。
- [ ] **Step 3: 書く** — `repo.Log(&git.LogOptions{From: from})` で辿り、`len(c.ParentHashes) != 1` と op のないものを飛ばし、`entities(c)` で行を得て、同じ差分から `Removed`(log.jsonl が消えたエンティティ)と `StoreLog` を決め、最後に `At` の新しい順・SHA の辞書順で並べ直す。
- [ ] **Step 4: 通ることを確かめる** — `go test ./internal/infra/gitstore/... ./internal/archtest/` → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(gitstore): Store の保存のコミットを、足した行と一緒に新しい順に返す(C4d Task 2)"`

### Task 3: コミット 1 つから行を作る(`app/activity`)

**Files:**
- Create: `internal/app/activity/row.go`(型)、`internal/app/activity/commit.go`(本体・自動化・メンバーの行)、`internal/app/activity/target.go`(対象とワークスペース)
- Test: `internal/app/activity/commit_test.go`

**Interfaces:**
- Consumes: Task 1 の `port.Commit`、`port.Reader`
- Produces:
  ```go
  type ActorKind string // "user" | "automation" | "github" | "sync" | "none"
  type Target struct {
      Display string // いまの ID か旧 ID か完全に削除したときの ID
      Title   string // プロジェクトだけ
      Link    string // "current" | "moved" | ""(文字だけ)
  }
  type Row struct {
      Key        string // "{SHA}:{6 桁の 0 埋めの連番}" か "sync:{ID}"
      At         time.Time
      Store      port.StoreInfo
      Actor      ActorKind
      Who        string // C4d-12
      Avatar     string // アバターに渡す名前。不明なユーザーは ""
      Text       string
      Workspaces []workspace.ID
      Storewide  bool
      Workspace  string // 名前。Workspaces が 1 つのときだけ
      Item       *Target
      Project    *Target
      SHA        string // 7 桁。同期の行は ""
      dedup      string // C4d-11 の鍵。人の行は ""
  }
  func commitRows(r port.Reader, store port.StoreInfo, c port.Commit) ([]Row, error) // C4d-3〜C4d-9・C4d-12
  ```

- [ ] **Step 1: 失敗するテストを書く** — `apptest.NewStores` と `apptest.NewCommitLog` で保存を作り、`Commits` の結果を `commitRows` に通す。
  - `TestRowsUserSave`: 「HH-1 のタイトルを変更」の保存 → 1 行、`Actor` user、`Who` はメンバーの表示名、`Item.Display` "HH-1"・`Link` "current"、`Project.Title` がそのアイテムのプロジェクト、`Workspace` がワークスペース名、`SHA` が 7 桁。
  - `TestRowsUnknownUser`: メンバーにいない人の保存 → `Who` 「不明なユーザー」、`Avatar` ""。
  - `TestRowsAutomation`: 人のステータスの変更と同じ保存に自動化の行(`Draft.ByRule`)がある → 2 行。2 行目は `Actor` automation、`Who` 「自動化」、`Text` 「「{ルール名}」でステータスを Done に変更」(`automation.ActivityText`)、SHA は同じ。
  - `TestRowsMemberJoin`: アイテムの行と `member.Join("hanako")`・`member.Join("jiro")` が同じ保存 → 本体と「メンバー hanako を追加」「メンバー jiro を追加」の 3 行。members だけの保存 → 本体の 1 行だけ、`Storewide` true、`Workspaces` 空。
  - `TestRowsTargetRoot`: 親と子 2 つを 1 回で作った保存 → `Item` は親。2 つのプロジェクトのアイテムを 1 回で変えた保存 → `Item` nil。
  - `TestRowsRepairWithoutOpRows`: 作者 `hedgehug[bot]`、行の op がコミットの op と違う(修復 (2))コミット → `Actor` sync、`Who` 「同期」、対象はその行のアイテム。
  - `TestRowsMovedOut`: アイテムを Store から `Remove` し、転送記録 `Old: "HH-3"` があるとき → `Item` {"HH-3", "", "moved"}、`Workspaces` 空、`Storewide` false。完全に削除の記録だけのとき → `Link` ""。
  - `TestRowsMovedWithinStore`: ワークスペース A で変えたアイテムを B へ移したあと → 古い保存の行の `Workspaces` は [B]。
  - `TestRowsDeletedItem`: 削除済みのアイテム → `Link` ""、ID はいまの ID。
  - `TestRowsRetention`: 保持日数を変えた保存 → `Storewide` true、対象なし。
- [ ] **Step 2: 落ちることを確かめる** — `go test ./internal/app/activity/` → FAIL。
- [ ] **Step 3: 書く** — 決定 C4d-3〜C4d-9・C4d-12 のとおり。値の名前とメンバー名は Store を 1 回読んで引く。
- [ ] **Step 4: 通ることを確かめる** — `go test ./internal/app/activity/ ./internal/archtest/` → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(activity): 保存のコミット 1 つから、本体・自動化・メンバーの追加の行を作る(C4d Task 3)"`

### Task 4: 同期の行の記録と、並び・重なり・絞り込み・区切り

**Files:**
- Modify: `internal/app/port/syncstate.go`(`SyncLine`・`SyncLog`)
- Create: `internal/infra/statedb/synclog.go`(表 `sync_log`、`register("sync/003", …)`)
- Modify: `internal/app/syncing/service.go`(`Service` に `Log port.SyncLog`。nil なら書かない)、`internal/app/syncing/run.go`(`Succeeded` の直後)、`internal/app/syncing/deliver.go`(同じ)
- Create: `internal/app/activity/service.go`(`Service`・`Page`)、`internal/app/activity/filter.go`
- Test: `internal/infra/statedb/synclog_test.go`、`internal/app/syncing/service_test.go`(足す)、`internal/app/activity/service_test.go`

**Interfaces:**
- Consumes: Task 3 の `Row`・`commitRows`、Task 1 の `port.StoreCommits`
- Produces:
  ```go
  // port/syncstate.go
  type SyncLine struct { ID int64; Store StoreID; At time.Time; Sent, Received int }
  type SyncLog interface {
      Add(ctx context.Context, store StoreID, at time.Time, sent, received int) error // 両方 0 なら何もしない
      List(ctx context.Context, store StoreID) ([]SyncLine, error)                  // 新しい順
      ForgetStore(ctx context.Context, store StoreID) error
  }
  // app/activity
  type Kind string // "all" | "user" | "sync" | "automation"
  type Query struct { Store port.StoreID; Workspace workspace.ID; Kind Kind; Before string } // Store・Workspace が空なら「すべて」
  type Page struct { Rows []Row; More bool }
  type Service struct { Stores port.Stores; Commits port.StoreCommits; Log port.SyncLog; List port.StoreList }
  func (s Service) Page(ctx context.Context, q Query) (Page, error)
  ```

- [ ] **Step 1: 失敗するテストを書く**
  - `synclog_test.go`: `TestSyncLogAddList`(0・0 は書かない、新しい順)、`TestSyncLogForgetStore`。
  - `service_test.go`(syncing): `TestRunLogsSyncLine` — 偽の Replicator が `Sent: 2, Received: 1` を返すと `Log.List` に 1 行、`Sent: 0, Received: 0` なら増えない。書き込みのあとの同期(`Deliver`)でも同じ。
  - `service_test.go`(activity): 2 つの Store(1 つは `StoreList` に入れない = 非表示)と同期の行で:
    - `TestPageOrdersByTime`: 行が時刻の新しい順。未来の時刻の保存が先頭。同期の行の `Text` は「origin と同期 (送信 2 件・受信 1 件)」、`Who` 「同期」、`SHA` ""。
    - `TestPageMergesNonHumanDuplicates`: 同じ (アイテム・`status`・from・to・Prev) の自動化の行が 2 つのコミット(2 台が同じ版から作ったもの)にある → 早い方の 1 行。順に起きた同じ変更の繰り返し(Prev が違う) → 別の行。実行者「同期」の同じ件名で行のないコミット 2 つ → 1 行(消したエンティティが違えば 2 行)。人の同じ変更 2 つ → 2 行。
    - `TestPageKindFilter`: `user` は人の行だけ(受け取った他の人の保存を含む)、`sync` は同期の行と実行者「同期」、`automation` は自動化と GitHub、`none` の行は `all` だけ。
    - `TestPageWorkspaceFilter`: ワークスペース A を選ぶと、A の行・同じ Store の同期の行・同じ Store の `Storewide` の行が出て、B だけの行と別の Store の行は出ない。A と B にまたがる行は A でも B でも出る。
    - `TestPageHiddenStore`: 非表示の Store の行と同期の行は出ない。
    - `TestPageLimitAndMore`: 305 行 → 300 行と `More` true。最後の行の `{時刻}|{鍵}` を `Before` に渡すと残りの 5 行と `More` false。
    - `TestPageCursorStable`: 1 ページ目のあとに新しい保存を足してから 2 ページ目を読んでも、重なりも抜けもない。
- [ ] **Step 2: 落ちることを確かめる** — `go test ./internal/infra/statedb/ ./internal/app/syncing/ ./internal/app/activity/` → FAIL。
- [ ] **Step 3: 書く** — 決定 C4d-6・C4d-9・C4d-11・C4d-13・C4d-14 のとおり。重なりは絞り込みの前にまとめる(「自動化だけ」で数えても 1 行)。
- [ ] **Step 4: 通ることを確かめる** — 同じコマンド → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(activity): 同期の行を記録し、Store をまたいで並べて、重なり・絞り込み・300 件の区切りを行う(C4d Task 4)"`

### Task 5: アクティビティの API と組み立て

**Files:**
- Create: `internal/adapter/httpapi/activity/activity.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps` に `Commits port.StoreCommits`・`SyncLog port.SyncLog`、`activity.Register`)、`cmd/hedgehug/wire.go`(`Commits: stores`、`SyncLog: db.SyncLog()`、`syncing.Service.Log`)、`cmd/hedgehug/wire_e2e.go`(`apptest.NewCommitLog` で Stores を包み、`SyncLog` は本物の state.db)
- Create: `cmd/hedgehug/synclog_e2e.go`(`POST /e2e/sync-log`、`device_e2e.go` の入口の並びにつなぐ)
- Modify: `api/openapi.yaml`、`web/packages/api/src/generated.ts`(生成)
- Test: `internal/adapter/httpapi/activity/activity_test.go`

**Interfaces:**
- Consumes: Task 4 の `activity.Service`・`Query`・`Page`
- Produces: `GET /api/activity`(operationId `list-activity`)。クエリ `store`・`workspace`・`kind`(既定 `all`)・`before`。本文 `{ rows: ActivityRow[], more: boolean }`。`ActivityRow` は `{ key, cursor, at, actor, who, avatar, text, store: {id, name, color}, workspace: string, item?: {display, link}, project?: {display, title, link}, sha }`(`link` は `"current" | "moved" | ""`)。`workspace` と `store` の片方だけ・知らない `kind` は 422

- [ ] **Step 1: 失敗するテストを書く** — `TestListActivity`(行の JSON の形と 300 件)、`TestListActivityFilters`(`kind=sync`・`store`+`workspace`)、`TestListActivityRejects`(`kind=bot`・`workspace` だけ → 422)。
- [ ] **Step 2: 落ちることを確かめる** — `go test ./internal/adapter/httpapi/activity/` → FAIL。
- [ ] **Step 3: 書く** — 受信箱の `inbox.Register` と同じ形。`make generate`、`pnpm --dir web gen`。
- [ ] **Step 4: 通ることを確かめる** — `go test ./... && golangci-lint run --build-tags e2e ./...` → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(api): アクティビティの一覧の API を足し、本物と e2e の組み立てにつなぐ(C4d Task 5)"`

### Task 6: 履歴の「同期で受信」(サーバー)

**Files:**
- Modify: `internal/domain/history/history.go`(`Row` に `Op change.ID` と `Actor change.ActorKind`)、`item.go`・`project.go`(`e.rec.Op` を入れる)
- Modify: `internal/app/history/service.go`(`ReceivedOps` を宣言し、`Service` に `Received ReceivedOps`。nil なら添えない。`Item`・`Project` が返す行の型を `Row{dhistory.Row; Received bool}` にする)
- Modify: `internal/infra/statedb/judged.go`(`func (d *DB) Received(ctx, store port.StoreID, ops []change.ID) (map[change.ID]bool, error)`)
- Modify: `internal/adapter/httpapi/notes/history.go`(`HistoryRow` に `Received bool json:"received"`)、`routes.go`(`Deps` に `Received apphistory.ReceivedOps`)、`cmd/hedgehug/wire.go`・`wire_e2e.go`(`Received: db`。`wire` は `(Deps, port.JudgedSaves, error)` を返す)
- Create: `cmd/hedgehug/receive_e2e.go`(`POST /e2e/receive`。書き先は `wire` が返す `port.JudgedSaves`)
- Modify: `api/openapi.yaml`、`generated.ts`
- Test: `internal/infra/statedb/judged_test.go`、`internal/app/history/history_test.go`、`internal/adapter/httpapi/notes/history_test.go`、`internal/infra/gitstore/sync_judge_own_test.go`

**Interfaces:**
- Produces: `type ReceivedOps interface { Received(ctx context.Context, store port.StoreID, ops []change.ID) (map[change.ID]bool, error) }`、`type Row struct { dhistory.Row; Received bool }`、API の `HistoryRow.received`(required)

- [ ] **Step 1: 失敗するテストを書く**
  - `judged_test.go` `TestReceivedOps`: `Mark` だけの op → true、`MarkOwn` した op → false、`Mark` と `MarkOwn` の両方がある op → false、記録のない op → false、別の Store の記録は数えない。
  - `history_test.go` `TestItemHistoryReceived`: 偽の `ReceivedOps` が 2 つ目の保存の op を返すと、その行だけ `Received` true。`Received` が nil なら全部 false。
  - `history_test.go` `TestItemHistoryReceivedSkipsSync`: 実行者「同期」の行は、op が受け取ったものでも `Received` false。
  - `sync_judge_own_test.go`(gitstore)`TestOwnBeforeRemote`: remote なしで作った Store で保存してから、空の remote を付けて最初の同期をすると、その保存が `own = 1` になる(`Received` で false)。B が書いた保存を受け取ると true。落ちたら `saves.ownSaves` の空の remote の扱いを直す。
  - `history_test.go`(httpapi)で `received` が JSON に出る。
- [ ] **Step 2: 落ちることを確かめる** — `go test ./internal/infra/statedb/ ./internal/app/history/ ./internal/adapter/httpapi/notes/` → FAIL。
- [ ] **Step 3: 書く** — 決定 C4d-16。`make generate`、`pnpm --dir web gen`。
- [ ] **Step 4: 通ることを確かめる** — `go test ./... && golangci-lint run --build-tags e2e ./...` → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(history): 同期で受け取った保存の履歴の行に印を付ける(C4d Task 6)"`

### Task 7: 画面の土台(アドレス・口・状態・表示の値)

**Files:**
- Modify: `web/packages/core/src/features/router/route.ts`・`resolve.ts`(`{ name: "activity" }`、綴り `/activity`)と各テスト
- Create: `web/packages/core/src/model/activity.ts`、`port/activity.ts`、`features/activity/activity.ts`(状態)、`features/activity/view.ts`(表示の値)、`activityIndex.ts`、テスト `activity.test.ts`・`activity.test-util.ts`・`view.test.ts`
- Modify: `web/packages/core/src/index.ts`(`export * from "./activityIndex"`)
- Create: `web/packages/api/src/activity.ts`(`createActivityPort()`)・`activity.test.ts`、Modify: `web/packages/api/src/index.ts`

**Interfaces:**
- Produces:
  - `type ActivityKind = "all" | "user" | "sync" | "automation"`、`type ActivityFilter = { workspace: string /* "" か "{storeId}/{workspaceId}" */; kind: ActivityKind }`
  - `ActivityPort { list(filter: ActivityFilter, before?: string): Promise<{ rows: ActivityRow[]; more: boolean }> }`
  - `createActivity({ port, toasts }): Activity`。`Activity = { state: Readable<ActivityState>; open(); more(); setFilter(f: Partial<ActivityFilter>); reload(): Promise<void> }`、`ActivityState = { filter, rows, more, loading, loaded }`。読み方は受信箱の `first`・`again`・`more` と通し番号で古い答えを捨てる形を写す。`again` は先頭から 300 件ずつ、出していた件数を覆うまで読む(1 回の読みで行が増えなければ止める。C4d-14)
  - `view.ts`: `activityDays(rows: ActivityRow[], today: string): { head: string; rows: ActivityRow[] }[]`(このマシンの時刻帯の `dayOf` でまとめる。見出しは「今日」「昨日」、ほかは `formatFullDay` の「YYYY/MM/DD」)、`formatFullDay(ymd: string): string`(`features/dates/date.ts` に足す)、`clockOf(at: string): string`(「HH:MM」)、`activityKindOptions`(4 つ。`kindOptions` は既存の名前と重なる)、`workspaceOptions(stores: StoreNode[]): SelectOption[]`(「すべて」と「{Store 名} / {ワークスペース名}」)、`activityEmpty = "この条件のコミットはありません"`

- [ ] **Step 1: 失敗するテストを書く** — `route.test.ts`(`/activity` の読み書き)、`resolve.test.ts`、`view.test.ts`(`activityDays` が今日・昨日・去年の行を「今日」「昨日」「2025/11/14」にまとめ、同じ日の中は新しい順のまま。`today` の日付が変わると見出しが変わる。`clockOf` が「09:05」)、`activity.test.ts`(`open` で `before` なし、`more` で前の最後の行の `{at}|{key}` を `before` に、`setFilter` で先頭から読み直す、3300 行を出しているときの `reload` が 11 回読んで同じ件数を出す、古い答えを捨てる)、api の `activity.test.ts`(クエリの綴り。`workspace` の `"{storeId}/{workspaceId}"` を `store`・`workspace` に分ける)。
- [ ] **Step 2: 落ちることを確かめる** — `pnpm --dir web test` → FAIL。
- [ ] **Step 3: 書く**。
- [ ] **Step 4: 通ることを確かめる** — `make web-check` → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(web): アクティビティのアドレス・口・状態と表示の値を足す(C4d Task 7)"`

### Task 8: 実行者のアイコン(ui)

**Files:**
- Create: `web/packages/ui/src/patterns/ActorIcon.svelte`、`ActorIcon.test.ts`
- Modify: `web/packages/ui/src/index.ts`

**Interfaces:**
- Produces: `ActorIcon` props `{ kind: "user" | "automation" | "github" | "sync" | "none"; name: string; title: string }`。user は `Avatar`(`name`・`title` をそのまま)、automation は「A」、github は GitHub の印の SVG、sync は同期の矢印の SVG、none は「hh」。どれも `role="img"` と `aria-label={title}`(none の title は「hedgehug」)

- [ ] **Step 1: 失敗するテストを書く** — 種類ごとに `getByRole("img", { name })` で見つかり、automation の中身が「A」、none が「hh」、user で `name` が空なら中身のない丸(`Avatar` のテストと同じ確かめ方)。
- [ ] **Step 2: 落ちることを確かめる** — `pnpm --dir web --filter @hedgehug/ui test` → FAIL。
- [ ] **Step 3: 書く**(`KindIcon` の大きさと色のトークンに合わせる)。
- [ ] **Step 4: 通ることを確かめる** — 同じコマンド → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(ui): アクティビティの実行者のアイコンを足す(C4d Task 8)"`

### Task 9: アクティビティの画面・ナビ・パレット

**Files:**
- Create: `web/app/src/activity.ts`(`createActivity` の実体、`reloads` と同期の出来事への登録、行のリンク `activityLinkOf`)
- Create: `web/app/src/screens/activity/ActivityScreen.svelte`、`ActivityRow.svelte`、`ActivityFilters.svelte`
- Modify: `web/app/src/shell/Screen.svelte`(分岐)、`web/app/src/shell/Sidebar.svelte`(「受信箱」と「検索」のあいだに「アクティビティ」)、`web/app/src/reloads.ts`(同期の出来事で読み直す登録簿 `syncReloads` を足し、`wire.ts` の `reflect` の `sync` の分岐から呼ぶのは 1 行だけ)
- Modify: `web/packages/core/src/features/palette/actions.ts`(「受信箱を開く」の次に `moveTo(d, "command:activity", g, "アクティビティを開く", { name: "activity" })`)、`palette.test.ts`・`actions.test.ts`(並びの期待値)
- Test: `web/e2e/activity.spec.ts`、`web/e2e/activity-empty.spec.ts`(`ownServer`)、`web/e2e/palette.spec.ts`(足す)

**Interfaces:**
- Consumes: Task 7 の `createActivity`・`activityDays`・`workspaceOptions`・`activityKindOptions`・`clockOf`、Task 8 の `ActorIcon`、`StoreBadge`、`Select`、`links`、`detail.open`、`syncNow`
- Produces: 画面だけ

- [ ] **Step 1: 失敗する e2e を書く**
  - `activity.spec.ts`(3 つの幅): サイドバーの「ナビ」から「アクティビティ」を開くと、見出し「アクティビティ」と補足が出る。タスクを作ってタイトルを変えると、「今日」の下の先頭に「{ID} のタイトルを変更」の行が、時刻「HH:MM」・Store のバッジ「e2e」・ID のリンク・7 桁の SHA と一緒に出る(画面を開いたまま保存しても読み直される)。ID のリンクで詳細パネルが開く。「種類」を「同期だけ」にすると、`POST /e2e/sync-log`(送信 2・受信 1)で足した「origin と同期 (送信 2 件・受信 1 件)」だけが出て、SHA の欄は空。「ワークスペース」を「e2e / Mobile」にすると、e2e の別のワークスペースの行が出ない。
  - `activity-empty.spec.ts`: 種類「自動化だけ」で「この条件のコミットはありません」。
  - `palette.spec.ts`: パレットの「アクティビティを開く」で `/activity` に移る。
  - core の `palette.test.ts`・`actions.test.ts` の並び(「受信箱を開く」「アクティビティを開く」「検索を開く」…)。
- [ ] **Step 2: 落ちることを確かめる** — `make e2e-bin && pnpm --dir web exec playwright test activity activity-empty palette` → FAIL。
- [ ] **Step 3: 書く** — 決定 C4d-15。行は 06 `:567-571` の順(時刻・アイコン・名前と本文・バッジとワークスペース名と ID とプロジェクト・SHA)。「さらに読み込む」は受信箱と同じ `Button size="sm"`。
- [ ] **Step 4: 通ることを確かめる** — 同じ e2e と `make web-check` → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(web): アクティビティの画面・サイドバーのナビ・パレットのコマンドを足す(C4d Task 9)"`

### Task 10: 履歴の「同期で受信」(画面)

**Files:**
- Modify: `web/packages/core/src/model/history.ts`(`received: boolean`)、`web/app/src/screens/notes/HistorySection.svelte`(`received` の行に「同期で受信」)
- Test: `web/e2e/history.spec.ts`(足す)、`web/e2e/helpers.ts`(準備の helper `apiTask`・`renameByApi`・`receiveMark`)

- [ ] **Step 1: 失敗する e2e を書く** — タスクのタイトルを変え、`POST /e2e/receive`(そのアイテム)を呼んでから詳細パネルの「履歴」を開くと、先頭の行に「同期で受信」が出て、2 行目(作成)には出ない。
- [ ] **Step 2: 落ちることを確かめる** — `pnpm --dir web exec playwright test history` → FAIL。
- [ ] **Step 3: 書く**。
- [ ] **Step 4: 通ることを確かめる** — 同じ e2e と `make web-check` → PASS。
- [ ] **Step 5: コミット** — `git commit -m "feat(web): 同期で受け取った履歴の行に「同期で受信」を添える(C4d Task 10)"`

### Task 11: 計画と記録の書き足し、最終の確認、PR

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C4d の行に「同期で受信」を足し、漏れの表の「同期で受信」を C4d に)、この計画の「実装で計画から変えたこと」
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md` `:492`(「同期で受け取った変更」のあとに「(Store を追加したときに受け取ったものを含む。同期の修復の行には添えない)」)
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「このマシンに持つもの」の `state.db` の中身の「同期の行」は表 `sync_log` と書く。「アクティビティと履歴」に C4d-3・C4d-4 の (2)(3)・C4d-7 の対象・C4d-9 の対象のワークスペースの決め方を 1 文ずつ)

- [ ] **Step 1: 書き足す**(上のとおり)。
- [ ] **Step 2: 全体の確認** — `make lint && make test && golangci-lint run --build-tags e2e ./... && make web-check` と、e2e の `activity`・`activity-empty`・`history`・`palette` → すべて緑。
- [ ] **Step 3: 最終レビュー**(superpowers:requesting-code-review で、ブランチ全体を見てもらう)。指摘を直す。
- [ ] **Step 4: PR を 1 本出す** — 向き先 develop、タイトル「feat: トラック C4d(アクティビティと履歴の「同期で受信」)」。本文の見出しは前の PR に合わせる(「概要」「範囲」「ユーザーに決めてもらったこと」「この計画で決めたこと」「計画から変えた主なもの」「develop の取り込み」「他のトラックへ渡すもの」「確かめたこと」)。「他のトラックへ渡すもの」: G = Store をこのマシンから削除するときに `SyncLog.ForgetStore` も呼ぶこと、のせ直しと保持日数の完全な削除の保存はアクティビティに一般の規則で出ること。末尾は `🤖 Generated with [Claude Code](https://claude.com/claude-code)`。
