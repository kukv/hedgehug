# hedgehug トラック D4a(GitHub を読む間隔・間隔ごとの取り直し・自動化のきっかけ)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 07 の「GitHub を読む間隔」で、リンクした PR・Issue を裏で取り直し、リンクの状態の変化で自動化のきっかけ(PR・Issue の 4 つ)を起こし、認証エラーのアカウントを自動で確かめ直す。

**Architecture:** リンクの操作(D3 の `app/github` の `Links`)を新しい機能パッケージ `app/githublink` へ移し、そこに裏で読む `Reader` を足す。トークンは、同期と同じ形(`port.SyncTokens` の前例。C2-3)で、`app/github.Connector` が実装する口 `port.GitHubTokens` から得る。間隔・利用上限・失敗後の待ちの規則は `domain/ghread`、きっかけの判定は `domain/automation` に置き、前回の読み取りの記録と、取り直しの失敗の文言は `infra/statedb` に持つ。GitHub はリポジトリごとに GraphQL 1 回(100 件ずつ)でまとめて読み、利用上限の残りも同じ答えから取る(前回の実装と同じ)。

**Tech Stack:** Go 1.27.3、GitHub GraphQL API v4(`X-GitHub-Api-Version: 2022-11-28`)、modernc.org/sqlite(state.db)、huma v2。画面は Svelte 5(`web/app`)、e2e は Playwright。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md`「状態」(自動の確かめ直し)、「GitHub との通信に使うアカウント」、「GitHub との通信に失敗したとき」、「表示と更新」、「GitHub を読む間隔」、「PR・Issue の状態で動く自動化」
- 03「自動化」の「きっかけとすること」「実行の規則」「制約で止まったとき」
- 06「自動で同期する」(待つ条件)、「アクティビティ」の 538 行(人でない実行者の重なり)
- 08「一般」の「同期のタイミング」(「GitHub を読む間隔」の値。画面は設定「一般」を作るトラック G)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「変更記録」の `by`・`actor`(142・143 行)、「このマシンに持つもの」(state.db)
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md` 2 節(依存の向き)・4 節(大きさの上限)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-d.md`(トラック D の計画。小計画の一覧と漏れの表)、`2026-11-13-hedgehug-track-d3.md`(リンクの型と操作)、`2026-11-13-hedgehug-track-b4.md`(自動化の実行。B4-1・B4-10・B4-12・B4-14)

## Global Constraints

土台とトラック D の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/apptest`・`infra/gitstore` は 20 個で満杯なので、ファイルを足さない。`app/github` は 18 個なので、足すのは 1 個まで(この計画では `recheck.go`)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。テストのトークンは `tok-taro` のような値にする。
- GitHub への 1 回の問い合わせは 30 秒で打ち切る(D1 の `Client` の `timeout` のまま)。
- 文言は仕様の文のとおりに書き、言い換えない。取り直しの失敗は「状態を更新できませんでした: 」に 07「GitHub との通信に失敗したとき」の表示を続ける(D3 の `failText`)。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は `127.0.0.1:4173` のサーバーを使い回すので、走らせる前に `ps aux | grep hedgehug-e2e` で別の worktree のサーバーがないか確かめる(CLAUDE.md)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| D4a-1 | D4 を 2 本に分ける。D4a = 読む間隔の仕組み・間隔ごとの取り直し・自動化のきっかけ 4 つ(手でのリンク・詳細パネルを開いたときの取り直し・間隔ごとの取り直し)・認証エラーの自動の確かめ直し。D4b = PR の自動リンク(プロジェクトにリポジトリを追加したときのすぐの探索と、概要の「PR を探せませんでした」を含む)・GitHub App 未導入(検出・記録・設定の行・帯・「管理者にインストールを依頼」・「再確認」)。トラック D の計画の一覧と漏れの表を直す(Task 1) | ユーザーの決定(2026-11-13) |
| D4a-2 | リンクの操作を `app/github` から `app/githublink` へ移す。トークンは口 `port.GitHubTokens`(`TokenFor`・`Rejected`・`Recheck`)で得て、実装は `app/github.Connector` に足す。`Session` は cmd で 1 つ作って共用する(いまと同じ) | `app/github` はテスト以外のファイルが 18 個で、上限 20 に入らない(設計書 4 節「超えたときは分ける」)。app の機能どうしは import しない(2 節)。同じ形の前例は C2-3 の `port.SyncTokens` |
| D4a-3 | 間隔ごとの取り直しは、リポジトリごとに、そのリポジトリのリンクのうち状態が Merged でないものの番号を、GraphQL 1 回で 100 件ずつまとめて読む(`port.GitHub.ReadMany`)。答えに `rateLimit { limit remaining resetAt }` を含め、利用上限の残りはこの値で数える。改名・移管は `Lookup` と同じに追う | 07「表示と更新」の 2 つ目の箇条(Merged を除くすべてを取り直す)。チェックの結果は PR の更新日時を変えないので、「更新されたものだけ」ではチェックが変わらない。前回の実装も同じ読み方 |
| D4a-4 | 07「GitHub を読む間隔」の「前回から更新されたものだけを読む」は、PR の自動リンクの探索(D4b)に当てる。D4a は前回の読み取りの日時を「間隔を数える」ためだけに持ち、「このマシンがそのリポジトリを初めて読む」の判定には使わない。D4b は探索の記録を別に持つ(下の Task 7 の表 `github_reads` の列を使わない) | D4a の取り直しの記録で「初めて」を判定すると、D4b の初回の探索が 2 回目に見え、初回にきっかけを起こしてしまう(07:301) |
| D4a-5 | 前回の読み取りの記録は Store とリポジトリ(小文字)ごとに持つ: 試した時刻、失敗したか、利用上限(上限・残り・戻る時刻)、読まない期限。state.db の表 `github_reads`。間隔は `wait = 間隔`、残りが上限の 10% 未満で戻る時刻が未来なら `wait = 間隔 × 3`。読むのは `now ≥ 試した時刻 + wait` かつ `now ≥ 読まない期限` のとき。成功でも失敗でも試した時刻を進めるので、失敗のあとは次の間隔まで読まない(アプリを開いても) | 07「GitHub を読む間隔」の箇条 1〜4・9 |
| D4a-6 | 利用上限で断られたら、読まない期限を、07「GitHub との通信に失敗したとき」の解除の時刻にする。時刻も秒数も分からなければ 1 分後。同じ読み取りの中で、同じアカウントのほかのリポジトリは問い合わせず、同じ失敗の文言と期限にする。戻る時刻がもう過ぎていれば(このマシンの時計が進んでいる)、文言は時刻を出さずに「しばらくしてから」にする(`ghcall` の文言で決める。最終レビュー) | 07 の 179 行と「GitHub を読む間隔」の 3 つ目の箇条。上限はトークンごとなので、同じアカウントで続けて問い合わせても断られる |
| D4a-7 | 「GitHub を読む間隔」は config.yaml の `sync.github_interval`(秒)で読む。書かれていなければ 5 分。書き換える画面は設定「一般」を作るトラック G が足す | C2-4 の `sync.interval` と同じ書き方。設定「一般」は G(C2 画面の決定 S1) |
| D4a-8 | 自動で読むのは、「裏で自動的に同期する」がオンで、見えているタブが待つ条件にもオフラインにもないときだけ。待つ条件とオフラインは、自動の同期(`syncing.Auto`)がすでに受けている知らせを使う(`Auto.Waiting`。cmd で `Reader.Waiting` に渡す)。アプリを開いた直後とオンラインに戻ったときは、間隔が過ぎたリポジトリがあれば、1 秒ごとの判定でそのまま読む | 07「GitHub を読む間隔」の箇条 7・8・9、06「自動で同期する」の待つ条件 |
| D4a-9 | 自動で読む Store は、閲覧のみでなく、非表示でなく、アカウントを割り当てたもの。`port.StoreGitHubInfo` に `Hidden` を足す(本物は G。いまの仮 `noStoreGitHub` は Store を返さない)。remote の有無は問わない | 07「GitHub を読む間隔」の箇条 5・10、「表示と更新」の 257・267 行、「削除の影響」の 154 行(アカウントがなければ状態の更新は止まる) |
| D4a-10 | 間隔ごとの取り直しの対象は、削除済みでないアイテムのリンク。削除済みのアイテムのリンクは、詳細パネルを開いたときだけ取り直す(D3 のまま) | 03「実行の規則」(削除済みのアイテムには動かない)、07「PR の自動リンク」の探すリポジトリ(削除済みでないアイテム)。ゴミ箱のアイテムのために上限を使わない |
| D4a-11 | 取り直しの失敗の文言は、state.db の表 `github_link_failures`(Store・リンクの `Ref.Key()`・文言)に持つ。間隔ごとの取り直しと詳細パネルを開いたときの取り直しの両方で、失敗で置き、成功で消す。`Links.List` は置いた文言を行に添える。ただし取り直さない Store(閲覧のみ、このマシンにアカウントがない。D3-9)では、`List` も `Refresh` も置いた文言を出さず、消しもしない(最終レビュー)。文言が変わったら `EventGitHub` を流す | 07「表示と更新」263 行(次の取り直しで成功すれば消す)。裏で読んだ失敗は応答で返せないので、画面が読み直したときに出せるよう、このマシンに持つ |
| D4a-12 | リポジトリごとの失敗(認証エラー・アカウントなし以外の通信の失敗・リポジトリを読めない・上限)は、そのリポジトリの読み取りの対象のリンクすべてに同じ文言を置く。答えに番号がなかったリンクには「{リポジトリ}#{番号} が見つかりません」の文言を置く。Store のアカウントが認証エラーなら、そのリポジトリのリンクに認証エラーの文言を置く | 07「表示と更新」263 行、「GitHub との通信に使うアカウント」160 行 |
| D4a-13 | きっかけの判定は `automation.LinkEvents(before *itemlink.Link, after itemlink.Link)`。before が nil なら「PR をリンクして…のとき」、そうでなければ「…に変わったとき」の行(07 の 292〜299 行)。「リンクした PR がマージされたとき」の `AllMerged` は、保存のあとのそのアイテムのリンクで数える(`automation.AllMerged`。Closed の PR を除いて 1 件以上あり、すべて Merged) | 07「PR・Issue の状態で動く自動化」、`automation.Event.AllMerged` のコメント(B4) |
| D4a-14 | きっかけは、リンクの値を書く保存と同じ保存の中で動かす。1 回の読み取り(間隔ごとの 1 回、詳細パネルを開いたときの 1 回)では、まず対象のリンクをすべて GitHub から読み、そのあとで書く。`AllMerged` を数えるときは、まだ書いていない同じアイテムのリンクも、今回読んだ値で数える。同じアイテムの同じきっかけは、その読み取りの中で 1 回だけ起こす(最初に保存したリンクのときに起こす)。読んだあとで同期などがリンクの値を変えていたら、そのリンクは書かず、`AllMerged` も読んだ古い値ではなく保存してある値で数える(最終レビュー) | 03「変更は、操作と同じ保存の中に入る」、07 の 301 行(1 回の読み取りで 1 回だけ)。1 つのアイテムの 2 本の PR が同じ間隔でマージされたとき、書く順によらず「すべてマージ済みのときだけ」のルールが 1 回動く |
| D4a-15 | GitHub の保存(実行者 `github`)の中で書く自動化の行は、`actor` が `automation:<ルール>`、`by` が `hedgehug[bot]`。`port.CheckRuleDrafts` は、自動化の行を人の操作に加えて GitHub の操作の中でも認める。`change.Stamp` は、実行者が人でない操作の自動化の行の `by` を `change.BotLogin` にする | 保存形式 142 行(`by` は、きっかけになった操作をした人。人でない実行者は `hedgehug[bot]`)。B4-1 は人の操作の中だけを認めていた。B4 の申し送り「自動化の行の by をどうするか」 |
| D4a-16 | 手で「PR をリンク」「Issue をリンク」したときもきっかけを起こす(`by` は操作した人)。このマシンが保存したリンクだけで起こし、同期で受け取ったリンクでは起こさない | 07 の 294〜297 行(「PR をリンクして…のとき」)、301 行。D3-13 の申し送り |
| D4a-17 | 自動化の通知は、`app/items` の `notify` と同じ形(B4-10・B4-14)で、保存のあとにこのマシンの受信箱(`port.Inbox`)へ書く。`app/items` は触らない(A4b が同じパッケージを変えているため、同じ十数行を `app/githublink` に持つ) | 03「変更したときは、あなた宛てに…通知を出す」。app の機能どうしは import しない |
| D4a-18 | 認証エラーの自動の確かめ直しは、`Reader` が「GitHub を読む間隔」ごとに 1 回(起動の直後を含む)、自動で読む条件(D4a-8)のときに `GitHubTokens.Recheck` を呼ぶ。`Recheck` は、認証エラーのアカウントのうちデバイスフロー以外のものについて、トークンを読み直して本人を確かめ、ログイン名が登録と同じなら利用可能に戻す。戻ったアカウントがあれば `EventGitHub` を流し、設定の「アカウントとメンバー」は一覧を読み直す。トーストは出さない | 07「状態」の 111 行 |
| D4a-19 | e2e の組み立ては自動の同期をオフにしている(C2)ので、`Reader` も自動では動かさず、e2e の入口 `POST /e2e/github-read` で 1 回の読み取りを間隔によらず行う | 間隔を待たずに確かめるため(D3-23 の `/e2e/github` と同じ形) |

## Review Focus

1. **書くまでのあいだに、利用者がリンクを解除・付け直した。** 読んだあとで書くので、その間に解除されたリンクに値を書いてはいけない。保存の中で、いまのリンクを読み直してから比べる(D3 の `writeRefresh` と同じ)。Task 11 のテストに入れる。
2. **2 本の PR が同じ間隔でマージされた。** 「すべてマージ済みのときだけ」のルールが、書く順によらず 1 回だけ動く(D4a-14)。Task 11 のテストに入れる。
3. **利用上限に途中で当たった。** 残りのリポジトリを問い合わせず、行に同じ文言が付き、期限まで読まない(D4a-6)。Task 12 のテストに入れる。
4. **認証エラーのアカウントの Store。** 問い合わせずに認証エラーの文言を行に付け、確かめ直しで戻ったあとは次の間隔で読む(D4a-12・D4a-18)。Task 12・Task 13 のテストに入れる。
5. **閲覧のみ・非表示・アカウントなしの Store、削除済みのアイテム。** 読まない(D4a-9・D4a-10)。Task 12 のテストに入れる。

---

## ファイルの地図

```
internal/domain/ghread/ghread.go             Record・Due(間隔・3 倍・読まない期限)・Limited
internal/domain/ghread/ghread_test.go
internal/domain/automation/link.go           LinkEvents・AllMerged
internal/domain/automation/link_test.go
internal/domain/change/stamp.go              (変更)人でない操作の自動化の行の by
internal/app/port/stores.go                  (変更)CheckRuleDrafts が GitHub の操作を認める
internal/app/port/github.go                  (変更)ReadMany・Batch・Rate
internal/app/port/githubtokens.go            GitHubTokens
internal/app/port/githubreads.go             GitHubReads
internal/app/port/storegithub.go             (変更)StoreGitHubInfo.Hidden
internal/app/port/syncsettings.go            (変更)SyncTiming.GitHubEvery
internal/app/apptest/github.go               (変更)ReadMany の偽物
internal/app/apptest/stores_contract_rules.go(変更)GitHub の操作の自動化の行
internal/infra/machineconfig/sync.go         (変更)sync.github_interval
internal/infra/github/read_many.go           ReadMany(GraphQL のまとめ読み)
internal/infra/github/read_many_test.go
internal/infra/statedb/reads.go              github_reads・github_link_failures
internal/infra/statedb/reads_test.go
internal/app/github/recheck.go               Connector.Recheck
internal/app/github/recheck_test.go
internal/app/githublink/                     links.go・link_add.go・link_list.go・link_project.go・link_refresh.go・link_remove.go(app/github から移す)
internal/app/githublink/fire.go              きっかけを保存の中で動かす・通知
internal/app/githublink/write.go             読んだ値を書く(手での取り直しと間隔ごとの共通)
internal/app/githublink/read.go              ReadOnce(1 回の読み取り)
internal/app/githublink/reader.go            Reader(間隔・待つ条件・確かめ直し)
internal/app/syncing/auto.go                 (変更)Waiting
internal/adapter/httpapi/routes.go           (変更)githublink.Links
cmd/hedgehug/wire.go・wire_e2e.go・links_e2e.go (変更)組み立てと e2e の入口
web/app/src/wire.ts                          (変更)github の知らせでアカウントの一覧を読み直す
web/e2e/links.spec.ts                        (変更)間隔ごとの取り直し・きっかけ・失敗の文言
docs/superpowers/plans/2026-11-12-hedgehug-track-d.md (変更)D4a・D4b
```

---

### Task 1: トラック D の計画を D4a・D4b に直す

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-d.md`(28 行の待つもの、73 行の共有部分の変更、86 行の小計画の一覧、100〜112 行の漏れの表)

- [ ] **Step 1: 一覧の D4 の行を D4a と D4b の 2 行にする**

D4a の範囲・待つもの・完成の条件は、この計画の Goal と D4a-1 のとおり。D4b の範囲は D4a-1 の D4b、待つものは D4a、完成の条件は「ブランチ名・印・タイトル・本文・コメントから PR を自動でリンクし、初回の読み取りではきっかけを起こさない。App 未導入の組織を記録し、設定の行・帯・再確認が動く」。73 行の D4 の共有部分は「D4a: `port.GitHubTokens`・`port.GitHubReads`・`StoreGitHubInfo.Hidden`・`SyncTiming.GitHubEvery`・`CheckRuleDrafts`」にする。

- [ ] **Step 2: 漏れの表を直す**

「状態」= D2 と D4a(自動の確かめ直し)と D4b(GitHub App 未導入)。「PR の自動リンク」= D4b。「表示と更新」= D3 と D4a(間隔ごと)。「GitHub を読む間隔」= D4a(読む仕組みと取り直し)と D4b(自動リンクの探索)。「PR・Issue の状態で動く自動化」= D4a(D4b は自動リンクの保存からも呼ぶ)。

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/plans/2026-11-12-hedgehug-track-d.md
git commit -m "docs(plan): トラック D の一覧で D4 を D4a・D4b に分ける"
```

---

### Task 2: `domain/ghread`(間隔・利用上限・失敗後の待ち)

**Files:**
- Create: `internal/domain/ghread/ghread.go`
- Test: `internal/domain/ghread/ghread_test.go`

**Interfaces:**
- Produces:
  ```go
  // Record は、Store のリポジトリ 1 つの、前回の読み取りの記録(D4a-5)。ゼロ値は「まだ読んでいない」。
  type Record struct {
      Attempt   time.Time // 前回に試した時刻(このマシンの時計)
      Failed    bool      // 前回は失敗した
      Limit     int       // 利用上限(0 は分からない)
      Remaining int
      ResetAt   time.Time // 上限が戻る時刻
      NotBefore time.Time // これより前は読まない(利用上限で断られたとき)
  }
  func Due(now time.Time, r Record, every time.Duration) bool
  func Low(now time.Time, r Record) bool          // 残りが 10% 未満で、戻る時刻が未来
  func Limited(now, reset time.Time) time.Time    // 断られたときの読まない期限。reset がゼロなら now+1 分
  ```

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestDue(t *testing.T) {
    now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
    every := 5 * time.Minute
    cases := []struct {
        name string
        r    ghread.Record
        want bool
    }{
        {"まだ読んでいない", ghread.Record{}, true},
        {"間隔が過ぎた", ghread.Record{Attempt: now.Add(-5 * time.Minute)}, true},
        {"間隔の途中", ghread.Record{Attempt: now.Add(-4 * time.Minute)}, false},
        {"失敗のあとも間隔まで待つ", ghread.Record{Attempt: now.Add(-4 * time.Minute), Failed: true}, false},
        {"残り 10% 未満なら 3 倍", ghread.Record{Attempt: now.Add(-14 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, false},
        {"3 倍が過ぎた", ghread.Record{Attempt: now.Add(-15 * time.Minute), Limit: 5000, Remaining: 499, ResetAt: now.Add(time.Hour)}, true},
        {"ちょうど 10% は 3 倍にしない", ghread.Record{Attempt: now.Add(-5 * time.Minute), Limit: 5000, Remaining: 500, ResetAt: now.Add(time.Hour)}, true},
        {"上限が戻ったら 3 倍をやめる", ghread.Record{Attempt: now.Add(-5 * time.Minute), Limit: 5000, Remaining: 1, ResetAt: now.Add(-time.Second)}, true},
        {"読まない期限の前", ghread.Record{Attempt: now.Add(-time.Hour), NotBefore: now.Add(time.Minute)}, false},
        {"読まない期限ちょうど", ghread.Record{Attempt: now.Add(-time.Hour), NotBefore: now}, true},
    }
    for _, c := range cases {
        if got := ghread.Due(now, c.r, every); got != c.want {
            t.Errorf("%s: Due = %v, want %v", c.name, got, c.want)
        }
    }
}

func TestLimited(t *testing.T) {
    now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
    if got := ghread.Limited(now, time.Time{}); !got.Equal(now.Add(time.Minute)) {
        t.Errorf("分からないとき = %v, want 1 分後", got)
    }
    reset := now.Add(23 * time.Minute)
    if got := ghread.Limited(now, reset); !got.Equal(reset) {
        t.Errorf("戻る時刻 = %v, want %v", got, reset)
    }
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/ghread/`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: `ghread.go` を書く**

パッケージの説明に 07「GitHub を読む間隔」を書く。`Low` は `r.Limit > 0 && r.Remaining*10 < r.Limit && now.Before(r.ResetAt)`。`Due` は `wait := every; if Low { wait *= 3 }` のあと `!now.Before(r.NotBefore) && (r.Attempt.IsZero() || now.Sub(r.Attempt) >= wait)`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/ghread/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/ghread
git commit -m "feat(domain): GitHub を読む間隔と利用上限の規則を足す(D4a Task 2)"
```

---

### Task 3: `automation.LinkEvents`・`AllMerged`(きっかけの判定)

**Files:**
- Create: `internal/domain/automation/link.go`
- Test: `internal/domain/automation/link_test.go`

**Interfaces:**
- Produces:
  ```go
  // LinkEvents は、リンクの値が before から after になったときに起きる、PR・Issue のきっかけ(07 の 292〜299 行)。
  // before が nil なら、いまリンクした(手でのリンク・自動リンク)。順は Triggers の順。
  func LinkEvents(before *itemlink.Link, after itemlink.Link) []Trigger
  // AllMerged は、マージされずに閉じた PR を除くリンク中の PR が 1 件以上あり、すべて Merged か(D4a-13)。Issue は数えない。
  func AllMerged(links []itemlink.Link) bool
  ```

- [ ] **Step 1: 失敗するテストを書く**

表で書く。`pr(state, review)` は PR のリンク、`issue(state)` は Issue のリンクを作るテストの関数。

| before | after | 期待 |
|---|---|---|
| nil | pr(Open, NoReview) | [TriggerPROpen] |
| nil | pr(Open, Pending) | [TriggerPROpen, TriggerReview] |
| nil | pr(Draft, Pending) | [TriggerReview] |
| nil | pr(Draft, NoReview) | [] |
| nil | pr(Merged, Pending) | [TriggerPRMerged] |
| nil | pr(Closed, NoReview) | [] |
| nil | issue(Closed) | [TriggerIssueClosed] |
| nil | issue(Open) | [] |
| pr(Draft, NoReview) | pr(Open, NoReview) | [TriggerPROpen] |
| pr(Closed, NoReview) | pr(Open, NoReview) | [TriggerPROpen] |
| pr(Open, NoReview) | pr(Open, NoReview) | [] |
| pr(Open, NoReview) | pr(Open, Pending) | [TriggerReview] |
| pr(Draft, NoReview) | pr(Draft, Pending) | [TriggerReview] |
| pr(Open, Approved) | pr(Open, Pending) | [] |
| pr(Open, ChangesRequested) | pr(Open, Pending) | [] |
| pr(Merged, NoReview) | pr(Merged, Pending) | [] |
| pr(Open, NoReview) | pr(Closed, Pending) | [] |
| pr(Open, Approved) | pr(Merged, Approved) | [TriggerPRMerged] |
| pr(Merged, Approved) | pr(Merged, Approved) | [] |
| issue(Open) | issue(Closed) | [TriggerIssueClosed] |
| issue(Closed) | issue(Closed) | [] |

`AllMerged` は: [] → false、[pr Merged] → true、[pr Merged, pr Open] → false、[pr Merged, pr Closed] → true、[pr Closed] → false、[pr Merged, issue Open] → true、[issue Closed] → false。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/automation/ -run 'TestLinkEvents|TestAllMerged'`
Expected: FAIL(`LinkEvents` がない)

- [ ] **Step 3: `link.go` を書く**

レビューのきっかけは、after が Open か Draft で Review が Pending、かつ「before が nil」か「before の Review が NoReview」(before の状態は問わない。Draft のまま依頼されたときも起きる)。Open のきっかけは、after が Open で、before が nil か before が Open でない。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/automation/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/automation/link.go internal/domain/automation/link_test.go
git commit -m "feat(domain): リンクの値の変化から PR・Issue のきっかけを判定する(D4a Task 3)"
```

---

### Task 4: GitHub の保存の中の自動化の行(`by` は `hedgehug[bot]`)

**Files:**
- Modify: `internal/domain/change/stamp.go`(`Stamp` の自動化の行)、`internal/domain/change/stamp_test.go`
- Modify: `internal/app/port/stores.go:19-31`(`CheckRuleDrafts`)
- Modify: `internal/app/apptest/stores_contract_rules.go`(偽物と gitstore の両方の照合)

**Interfaces:**
- Produces: `port.CheckRuleDrafts(actor, drafts)` は、`actor.Kind` が `ActorUser` か `ActorGitHub` のときに自動化の行を認める。`Stamp` は、自動化の行の `by` を、操作の実行者が人なら `meta.Actor.Login`、人でなければ `change.BotLogin` にする。

- [ ] **Step 1: 失敗するテストを書く**

`stamp_test.go` に `TestStampRuleRowInGitHubOperation`: `meta.Actor = change.Actor{Kind: change.ActorGitHub}`、draft に `.ByRule("01JF…")` を付けると、記録の `By == "hedgehug[bot]"`、`Actor.Code() == "automation:01JF…"`。照合テスト(`stores_contract_rules.go`)に、`Operation{Actor: change.Actor{Kind: change.ActorGitHub}}` で `ByRule` の行が書け、読み直した行の `by` が `hedgehug[bot]` になることと、`ActorSync` の操作では `ErrInvalidOperation` になることを足す。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/change/ ./internal/app/apptest/ ./internal/infra/gitstore/`
Expected: FAIL(`by` が空、GitHub の操作で `ErrInvalidOperation`)

- [ ] **Step 3: `Stamp` と `CheckRuleDrafts` を直す**

`CheckRuleDrafts` のコメントに D4a-15 を足す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/change/ ./internal/app/... ./internal/infra/gitstore/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/change internal/app/port/stores.go internal/app/apptest/stores_contract_rules.go
git commit -m "feat: GitHub の保存の中で自動化の行を書けるようにする(D4a Task 4)"
```

---

### Task 5: 口と設定の値を足す

**Files:**
- Modify: `internal/app/port/github.go`、`internal/app/port/storegithub.go`、`internal/app/port/syncsettings.go`
- Create: `internal/app/port/githubtokens.go`、`internal/app/port/githubreads.go`
- Modify: `internal/app/apptest/github.go`(`ReadMany` の偽物)、`internal/app/apptest/github_test.go`
- Modify: `internal/infra/machineconfig/sync.go`、`internal/infra/machineconfig/sync_test.go`
- Modify: `cmd/hedgehug/nostores.go` ほか、口が増えて組み立てが通らなくなる箇所

**Interfaces:**
- Produces:
  ```go
  // port/github.go の GitHub に足す:
  // ReadMany は、repo の番号 numbers の PR・Issue を、100 件ずつの GraphQL でまとめて読む(D4a-3)。
  // 改名・移管は Lookup と同じに追い、Batch.Repo に移動先の名前を入れる。答えにない番号は Missing。
  // リポジトリを読めない・利用上限・通信の失敗は *ghcall.Failure(Lookup と同じ種類)。
  ReadMany(ctx context.Context, token, repo string, numbers []int) (Batch, error)

  type Batch struct {
      Repo    string        // GitHub のいまの名前
      Found   map[int]Found // 番号から引く
      Missing []int
      Rate    Rate          // 最後の答えの rateLimit
  }
  type Rate struct {
      Limit, Remaining int
      ResetAt          time.Time
  }

  // port/githubtokens.go
  // GitHubTokens は、GitHub に問い合わせるトークンの口(D4a-2)。実装は app/github.Connector。
  type GitHubTokens interface {
      // TokenFor は、Store に割り当てたアカウントのトークンとアカウント。割り当てがなければ NoAccount、
      // 認証エラーなら AuthError の *ghcall.Failure。別のアカウントでは読まない。
      TokenFor(ctx context.Context, store StoreID, repo string) (string, account.Account, error)
      // Rejected は、a のトークンが拒否された(401)ことを知らせ、認証エラーにする。
      Rejected(id account.ID)
      // Recheck は、認証エラーのアカウント(デバイスフローを除く)を確かめ直し、戻ったものがあれば true(D4a-18)。
      Recheck(ctx context.Context) (bool, error)
  }

  // port/githubreads.go
  // GitHubReads は、GitHub を読んだ記録と、取り直しの失敗の文言の口(D4a-5・D4a-11)。実装は infra/statedb。
  type GitHubReads interface {
      // Records は、Store のリポジトリ(小文字)から引く、前回の読み取りの記録。
      Records(ctx context.Context, store StoreID) (map[string]ghread.Record, error)
      SaveRecord(ctx context.Context, store StoreID, repo string, r ghread.Record) error
      // Failures は、Store のリンクの Ref.Key() から引く、取り直しの失敗の文言。
      Failures(ctx context.Context, store StoreID) (map[string]string, error)
      // SetFailure は、文言を置く。空なら消す。前と変わったら changed。
      SetFailure(ctx context.Context, store StoreID, key, text string) (changed bool, err error)
  }

  // StoreGitHubInfo に足す:
  Hidden bool // 非表示の Store か(08。GitHub を自動で読まない。D4a-9)

  // SyncTiming に足す:
  GitHubEvery time.Duration // GitHub を読む間隔(D4a-7)
  ```
- `apptest.GitHub.ReadMany` は、`Put` した値から番号ごとに引き、`Fail(repo, n, err)` の失敗があれば、その番号は `Missing` ではなくその失敗を返す(リポジトリ全体の失敗の確かめに使う。`Fail(repo, 0, err)` はリポジトリの失敗)。`Rate` は、偽物のフィールド `Rate port.Rate` をそのまま返す。呼ばれた回数を `ReadManyCalls(repo) int` で返す。

- [ ] **Step 1: 失敗するテストを書く**

- `machineconfig/sync_test.go`: `sync: {github_interval: 900}` で `GitHubEvery == 15*time.Minute`、書かれていなければ `5*time.Minute`。
- `apptest/github_test.go`: `Put` した 2 件と、ない 1 件を `ReadMany` すると `Found` に 2 件、`Missing == []int{3}`。`Fail(repo, 0, &ghcall.Failure{Kind: ghcall.RateLimited})` のあとは RateLimited のエラー。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/machineconfig/ ./internal/app/apptest/`
Expected: FAIL

- [ ] **Step 3: 口・偽物・設定の読み取りを書く**

`syncYAML` に `GitHubInterval int \`yaml:"github_interval,omitempty"\``(秒)、`defaultTiming` に `GitHubEvery: 5 * time.Minute`。`port.GitHub` のインターフェースに `ReadMany` を足すのは Task 6(本物の `infra/github.Client` と同じコミット)にする。この Task では `Batch`・`Rate` の型、`GitHubTokens`・`GitHubReads`、`Hidden`、`GitHubEvery`、偽物の `ReadMany` メソッドだけを足す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go build ./... && go test ./internal/infra/machineconfig/ ./internal/app/apptest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/port internal/app/apptest internal/infra/machineconfig cmd
git commit -m "feat: GitHub を読む間隔の口と設定の値を足す(D4a Task 5)"
```

---

### Task 6: `infra/github` の `ReadMany`

**Files:**
- Create: `internal/infra/github/read_many.go`
- Test: `internal/infra/github/read_many_test.go`
- Modify: `internal/app/port/github.go`(`GitHub` に `ReadMany` を足す)

**Interfaces:**
- Consumes: Task 5 の `port.Batch`・`port.Rate`。D3 の `lookup.go` の `node`・`found`・`answerFailure` の分け方・`renamed`。
- Produces: `func (c *Client) ReadMany(ctx context.Context, token, repo string, numbers []int) (port.Batch, error)`

- [ ] **Step 1: 失敗するテストを書く**(httptest の偽のサーバー。`lookup_test.go` と同じ組み立て)

- `TestReadManyBatches`: 150 件を頼むと GraphQL を 2 回送り、1 回目の問い合わせの本文に `n0: issueOrPullRequest(number: 1)` から 100 個の別名があり、`rateLimit { limit remaining resetAt }` を含む。答えの `rateLimit` が `{5000, 4321, "2026-11-13T11:00:00Z"}` なら `Batch.Rate` がその値(2 回目の値)。
- `TestReadManyMissing`: 別名 `n1` が null なら、その番号は `Missing`。別名のパスの `NOT_FOUND`(パスの長さ 2)も `Missing`。
- `TestReadManyRepoUnreadable`: `repository` が null で、REST の `/repos/{repo}` も 404 → `ghcall.RepoUnreadable`(Repo 付き)。
- `TestReadManyRenamed`: `repository` が null で、REST の `/repos/old/web` が `full_name: new/web` → `new/web` で読み直し、`Batch.Repo == "new/web"`。
- `TestReadManyRateLimited`: `errors[].type == "RATE_LIMITED"` → `ghcall.RateLimited`、`ResetAt` は `X-RateLimit-Reset` の時刻。
- `TestReadManyNoNumbers`: 番号が空なら問い合わせず、`Batch{Repo: repo}` を返す。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/github/ -run TestReadMany`
Expected: FAIL

- [ ] **Step 3: `read_many.go` を書き、`port.GitHub` に `ReadMany` を足す**

問い合わせは `query($owner: String!, $name: String!) { rateLimit { limit remaining resetAt } repository(owner: $owner, name: $name) { nameWithOwner n0: issueOrPullRequest(number: 12) { ...F } … } }` で、フィールドは `lookupQuery` と同じ(フラグメント `F` にまとめ、`lookup.go` もそれを使うように直してよい)。番号は文字列に埋め込む(整数だけなので安全)。リポジトリの失敗のときだけ `renamed` で 1 回読み直す。300 行を超えそうなら、答えの型を `read_many_answer.go` に分ける。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/github/ && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infra/github internal/app/port/github.go
git commit -m "feat(infra): リポジトリのリンクを GraphQL でまとめて読む(D4a Task 6)"
```

---

### Task 7: `infra/statedb` の読み取りの記録と失敗の文言

**Files:**
- Create: `internal/infra/statedb/reads.go`
- Test: `internal/infra/statedb/reads_test.go`

**Interfaces:**
- Consumes: Task 2 の `ghread.Record`、Task 5 の `port.GitHubReads`。
- Produces: `func (d *DB) GitHubReads() port.GitHubReads`

- [ ] **Step 1: 失敗するテストを書く**

- `TestReadsRecordRoundTrip`: `SaveRecord(s, "Org/Web", r)` のあと `Records(s)["org/web"]` が r と同じ(時刻は UTC・ナノ秒まで)。別の Store には出ない。2 回目の `SaveRecord` で上書きされる。
- `TestReadsFailures`: `SetFailure(s, "pr:org/web#1", "状態を更新できませんでした: …")` が `changed == true`、同じ文言で `false`、空で消えて `true`、消えたものを空で `false`。`Failures(s)` に置いたものだけが出る。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/statedb/ -run TestReads`
Expected: FAIL

- [ ] **Step 3: `reads.go` を書く**

`init` で `register("github/002", …)` に表 `github_reads (store TEXT, repo TEXT, attempt TEXT, failed INTEGER, rate_limit INTEGER, remaining INTEGER, reset_at TEXT, not_before TEXT, PRIMARY KEY (store, repo))`、`register("github/003", …)` に表 `github_link_failures (store TEXT, link TEXT, text TEXT NOT NULL, PRIMARY KEY (store, link))`。時刻は `time.RFC3339Nano` の UTC、ゼロ値は空文字。repo は `strings.ToLower` で持つ。`SetFailure` は `checks.go` の `Save` と同じ形(トランザクションで読み比べてから書く)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/statedb/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infra/statedb/reads.go internal/infra/statedb/reads_test.go
git commit -m "feat(infra): GitHub を読んだ記録と取り直しの失敗を state.db に持つ(D4a Task 7)"
```

---

### Task 8: `Connector.Recheck`(認証エラーの自動の確かめ直し)

**Files:**
- Create: `internal/app/github/recheck.go`
- Test: `internal/app/github/recheck_test.go`

**Interfaces:**
- Consumes: `Session.state`・`markOK`・`markBroken`(D2)、`port.Accounts.List`、`port.Secrets.Token`、`port.GitHub.Viewer`、`fingerprint`。
- Produces: `func (c Connector) Recheck(ctx context.Context) (bool, error)`。これで `Connector` は `port.GitHubTokens` を満たす(テストで `var _ port.GitHubTokens = github.Connector{}` を確かめる)。

- [ ] **Step 1: 失敗するテストを書く**(`manage_test.go` と同じ組み立て)

- `TestRecheckRecovers`: gh のアカウント taro を `Rejected` で認証エラーにし、偽の GitHub のトークンの持ち主を taro にすると、`Recheck` が `true` を返し、`ListAccounts` の状態が「利用可能」になる。
- `TestRecheckStaysOnMismatch`: トークンの持ち主が jiro なら `false`、認証エラーのまま。
- `TestRecheckStaysOnRejected`: `Viewer` が `TokenRejected` なら `false`、認証エラーのまま。`Unreachable` でもそのまま(エラーは返さず次の間隔でまた試す)。
- `TestRecheckSkipsDeviceFlow`: デバイスフローのアカウントは問い合わせない(偽の GitHub の `Viewer` が呼ばれない)。
- `TestRecheckIgnoresHealthy`: 認証エラーでないアカウントには問い合わせない。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/github/ -run TestRecheck`
Expected: FAIL

- [ ] **Step 3: `recheck.go` を書く**

アカウントを `Accounts.List` で読み、`Session.state(id)` が認証エラーで `Method != account.DeviceFlow` のものだけ、`Secrets.Token` → `GitHub.Viewer`。ログイン名が大文字小文字を問わず同じなら `Session.markOK(id, fingerprint(tok))`。トークンが読めない・拒否・別人・通信の失敗は何もしない。`Accounts.List` の失敗だけを返す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/github/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/github/recheck.go internal/app/github/recheck_test.go
git commit -m "feat(app): 認証エラーのアカウントを確かめ直す入口を足す(D4a Task 8)"
```

---

### Task 9: リンクの操作を `app/githublink` へ移す

**Files:**
- Move: `internal/app/github/{links,link_add,link_list,link_project,link_refresh,link_remove}.go` と、そのテスト(`links_test.go`・`link_refresh_test.go`)を `internal/app/githublink/` へ(`git mv`)
- Modify: `internal/adapter/httpapi/routes.go:128`、`internal/adapter/httpapi/github/links.go`(型の参照)、`cmd/hedgehug/wire_e2e.go`・`links_e2e*.go`(参照があれば)

**Interfaces:**
- Consumes: Task 5 の `port.GitHubTokens`。
- Produces:
  ```go
  package githublink
  type Links struct {
      Stores   port.Stores
      Identity port.Identity
      Tokens   port.GitHubTokens
      GitHub   port.GitHub
      Accounts port.Accounts     // skipRefresh(このマシンにアカウントが 1 つもない)
      StoreGH  port.StoreGitHub  // 閲覧のみの判定と、Issue の 1 対 1 の Store の一覧
      Checks   port.Checks
      Reads    port.GitHubReads  // Task 10 から使う
      Inbox    port.Inbox        // Task 11 から使う
      Events   port.Events
      Clock    port.Clock
  }
  ```
  外から呼ぶメソッド(`List`・`Add`・`Remove`・`Refresh`・プロジェクトのリポジトリの操作)の名前と引数は変えない。

- [ ] **Step 1: ファイルを移し、`Connector` を使っていた箇所を書き換える**

`l.Connector.TokenFor` → `l.Tokens.TokenFor`、`l.Connector.Rejected` → `l.Tokens.Rejected`、`l.Connector.GitHub` → `l.GitHub`、`l.Connector.Stores` → `l.StoreGH`、`l.Connector.Accounts` → `l.Accounts`、`l.Connector.Clock` → `l.Clock`。`authError` を使っていた箇所は、`TokenFor` が返す失敗をそのまま返す形にするか、`githublink` に同じ 3 行の関数を置く(`ghcall.Failure{Kind: ghcall.AuthError, Name, Login}`)。テストは、`apptest` の偽物と、テストのファイルの中に置く `port.GitHubTokens` の偽物(`fakeTokens`。アカウントの割り当て・認証エラー・`Rejected` の記録を持つ)で組み立て直す。`app/github` のテストのうちリンクを使うもの(`links_test.go` の組み立ての関数など)が `Connector` の振る舞い(照合・取り直し)に頼っていたなら、その確かめは `app/github` の `connector_test.go` に既にあることを確かめてから外す。

- [ ] **Step 2: 移す前と同じテストが通ることを確かめる**

Run: `go build ./... && go test ./internal/app/github/ ./internal/app/githublink/ ./internal/adapter/... ./internal/archtest/`
Expected: PASS(テストの数は移す前の `app/github` のリンクのテストと同じ)

- [ ] **Step 3: lint**

Run: `make lint`
Expected: エラーなし(depguard と archtest が `app/githublink` を app の機能として扱う)

- [ ] **Step 4: Commit**

```bash
git add -A internal/app/github internal/app/githublink internal/adapter cmd
git commit -m "refactor: リンクの操作を app/githublink へ移し、トークンを口で得る(D4a Task 9)"
```

---

### Task 10: 取り直しの失敗の文言をこのマシンに持つ

**Files:**
- Modify: `internal/app/githublink/link_list.go`、`internal/app/githublink/link_refresh.go`
- Test: `internal/app/githublink/link_refresh_test.go`

**Interfaces:**
- Consumes: Task 5 の `port.GitHubReads.Failures`・`SetFailure`。
- Produces: `func (l Links) setFailure(ctx context.Context, store port.StoreID, key, text string) (bool, error)`(Task 12 も使う)。`List` は `Reads.Failures` の文言を `LinkRow.Error` に入れる。

- [ ] **Step 1: 失敗するテストを書く**

- `TestRefreshFailureIsKept`: 偽の GitHub で番号を消した(`Fail(repo, n, NotFound)`)状態で `Refresh` すると、行の `Error` が「状態を更新できませんでした: org/web#12 が見つかりません」。そのあとの `List`(GitHub に問い合わせない)の行にも同じ文言が出る。
- `TestRefreshSuccessClearsFailure`: そのあと値を戻して `Refresh` すると `Error` が空になり、`List` でも空。
- `TestRefreshFailurePublishes`: 文言が置かれたとき・消えたときに `EventGitHub` が 1 回ずつ流れる。同じ文言のままなら流れない。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run 'TestRefreshFailure|TestRefreshSuccess'`
Expected: FAIL

- [ ] **Step 3: 書く**

`Refresh` は、失敗した行で `setFailure(key, 文言)`、取り直せた行で `setFailure(key, "")`。チェックの結果か失敗の文言が変わったら `EventGitHub` を 1 回だけ流す(いまの `checksChanged` に合わせる)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink
git commit -m "feat(app): 取り直しの失敗の文言をこのマシンに持ち、次の成功で消す(D4a Task 10)"
```

---

### Task 11: 保存の中できっかけを動かす(手でのリンクと、詳細パネルを開いたときの取り直し)

**Files:**
- Create: `internal/app/githublink/fire.go`、`internal/app/githublink/write.go`
- Modify: `internal/app/githublink/link_add.go`、`internal/app/githublink/link_refresh.go`
- Test: `internal/app/githublink/fire_test.go`

**Interfaces:**
- Consumes: Task 3 の `automation.LinkEvents`・`AllMerged`、`automation.FromLog`・`Run`・`StoppedText`・`AppliedText`・`ChangeText`(B4)、`item.Display`、Task 4(GitHub の操作の中の自動化の行)。
- Produces:
  ```go
  // pass は、1 回の読み取り(D4a-14)。同じアイテムの同じきっかけを 1 回だけ起こすために、起こしたものを覚える。
  // fetched は、今回 GitHub から読んだ値(Ref.Key() から引く)。AllMerged をまだ書いていないリンクも含めて数えるのに使う。
  type pass struct {
      fired   map[item.ID]map[automation.Trigger]bool
      fetched map[string]itemlink.Values
  }
  func newPass() *pass

  // fire は、保存 tx の中で、アイテム id のリンクが before から after になったきっかけでルールを動かし、
  // 変えた行を書く。p が nil なら 1 回きりの操作(手でのリンク)。削除済みのアイテムでは動かさない。
  func fire(tx port.Tx, id item.ID, before *itemlink.Link, after itemlink.Link, p *pass) ([]fired, error)

  // notify は、保存のあとに、動いたルールの通知を受信箱に書く(B4-10・B4-14 と同じ形)。
  func (l Links) notify(ctx context.Context, store port.StoreID, fs []fired) error

  // writeFound は、取り直した値 f をリンク link に書く 1 回の保存(実行者「GitHub」)と、その中のきっかけ。
  // D3 の refreshOne・writeRefresh・renameTo をここへまとめ、間隔ごとの取り直し(Task 12)も使う。
  // 値が変わらなければ保存もきっかけもない。チェックの結果はいまと同じに置く。
  func (l Links) writeFound(ctx context.Context, store port.StoreID, id item.ID, link itemlink.Link, f port.Found, p *pass) (checksChanged bool, err error)
  ```

- [ ] **Step 1: 失敗するテストを書く**(ワークスペースのログにルールを置く組み立ては B4 の `app/items` のテストと同じ。ルール「PR マージで完了」= きっかけ `pr_merged`、すること ステータスを Done に)

- `TestAddOpenPRFires`: ルール「PR が Open で In Progress」があるアイテムに、Open の PR を手でリンクすると、同じ 1 回の保存で、ステータスが In Progress になり、その行の実行者が `automation:<ルール>`、`by` が操作した人。受信箱に「「PR が Open で In Progress」でステータスを In Progress に変更しました」が 1 件。
- `TestAddClosedPRDoesNotFire`: Closed の PR を手でリンクしても動かない。
- `TestRefreshMergedFires`: Open でリンクしてある PR が、詳細パネルを開いたときの取り直しで Merged になると、同じ保存でステータスが Done になり、自動化の行の `by` が `hedgehug[bot]`。保存は 1 回(コミットが 1 つ増える)。
- `TestRefreshAllMergedOrder`(Review Focus 2): 「すべてマージ済みのときだけ」のルールがあるアイテムに Open の PR が 2 本あり、取り直しで 2 本とも Merged になると、ルールは 1 回だけ動き、1 本目のリンクの保存の中で動く。2 本の並びを逆にしても 1 回だけ動く。
- `TestRefreshSameTriggerOnce`: 同じアイテムの 2 本の PR が、1 回の取り直しで両方 Merged になったとき、`AllMerged` を付けないルールも 1 回だけ動く(通知も 1 件)。
- `TestRefreshUnlinkedMeanwhile`(Review Focus 1): GitHub から読んだあと、書く前に、そのリンクを解除する(偽の `Stores` の書き込みの前に `Remove` を差し込む)と、値もきっかけも書かない。
- `TestFireSkipsDeletedItem`: 削除済みのアイテムで取り直しても、値は書くがルールは動かない。
- `TestRefreshStoppedNotifies`: ワークフローの制約で止まったルールは、何も書かず、受信箱に止まった理由つきの通知を 1 件書く。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run 'TestAdd.*Fire|TestRefresh(Merged|AllMerged|SameTrigger|Unlinked|Stopped)|TestFireSkips'`
Expected: FAIL

- [ ] **Step 3: `fire.go`・`write.go` を書き、`Add`・`Refresh` から使う**

`fire` の流れ: `tx.Items().Get(id)` で削除済みなら何もしない → `automation.LinkEvents(before, after)` から、`p` が覚えている(アイテム・きっかけ)を除く → ワークスペースのログから `automation.FromLog` → 設定(`tx.Workspaces().Get(ws)` の設定)→ きっかけごとに `automation.Run(st, rules, it, automation.Event{Trigger: t, AllMerged: …})`。`AllMerged` は、保存のあとのアイテムのリンク(`itemlink.FromLog` で読み直したもの)の値を、`p.fetched` にある値で置き換えてから `automation.AllMerged` で数える。動いたルールの行(`o.Draft`)を `tx.WriteItems().Append` で書き、1 つ目のルールがステータスを変えたら、次のきっかけのルールは変えたあとのアイテムで判定する(`automation.Run` は it を値で受けるので、`o.Draft` を当てたあとの it を作り直す)。起こしたきっかけを `p.fired` に覚える。`Refresh` は、すべてのリンクを `lookup` してから(`p.fetched` に入れ)、リンクごとに `writeFound` で書く。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS(D3 から移したテストを含む)

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink
git commit -m "feat(app): 手でのリンクと取り直しの保存の中で、PR・Issue のきっかけを動かす(D4a Task 11)"
```

---

### Task 12: `ReadOnce`(1 回の間隔ごとの取り直し)

**Files:**
- Create: `internal/app/githublink/read.go`
- Test: `internal/app/githublink/read_test.go`

**Interfaces:**
- Consumes: Task 2 の `ghread`、Task 5・6 の `ReadMany`、Task 7 の `Reads`、Task 10 の `setFailure`、Task 11 の `writeFound`・`newPass`・`notify`。
- Produces:
  ```go
  // ReadOnce は、自動で読む Store(D4a-9)の、リンクのあるリポジトリのうち、読む時期が来たもの(force なら全部)を 1 回読む。
  // every は「GitHub を読む間隔」。1 つの Store・リポジトリの失敗で、ほかを止めない。
  func (l Links) ReadOnce(ctx context.Context, every time.Duration, force bool) error
  ```

- [ ] **Step 1: 失敗するテストを書く**(偽の時計 `apptest.Clock`、偽の GitHub、Task 7 の本物の statedb を `t.TempDir()` で開いてよい。または `GitHubReads` の偽物をテストのファイルに置く)

- `TestReadOnceRefreshesNonMerged`: Store の 2 つのアイテムに `org/web#1`(Open)・`org/web#2`(Merged)・`org/api#3`(Open)。GitHub の #1 を Merged に変えて `ReadOnce` すると、`ReadMany` は `org/web` に `[1]`、`org/api` に `[3]` で 1 回ずつ呼ばれ(#2 は頼まない)、#1 が Merged で保存される(実行者「GitHub」、ルールがあれば動く)。
- `TestReadOnceWaitsForInterval`: 続けて `ReadOnce` しても問い合わせない。時計を 5 分進めると問い合わせる。`every` を 15 分にすると 15 分まで待つ。
- `TestReadOnceFailureWaits`: `org/web` が `Unreachable` で失敗すると、#1 の行の文言が「状態を更新できませんでした: GitHub に接続できません。接続を確かめて、もう一度お試しください」。時計を 4 分進めても問い合わせない。5 分で問い合わせ、成功すれば文言が消える。
- `TestReadOnceLowRateTriples`: 答えの `Rate` が `{5000, 400, now+1h}` なら、次は 15 分まで読まない。
- `TestReadOnceRateLimitedStopsAccount`(Review Focus 3): 同じアカウントの Store に `org/web`・`org/api` があり、`org/web` が `RateLimited`(ResetAt = now+23 分)なら、`org/api` は問い合わせず、両方の行に「状態を更新できませんでした: GitHub の利用上限に達しました。HH:MM 以降にもう一度お試しください」(HH:MM は偽の時計の時刻帯で now+23 分を分に切り上げたもの)。時計を 22 分進めても読まず、23 分で読む。ResetAt がゼロなら 1 分後に読む。
- `TestReadOnceMissingNumber`: 答えの `Missing` に #1 があれば、#1 の行に「状態を更新できませんでした: org/web#1 が見つかりません」、ほかの番号は取り直す。
- `TestReadOnceAuthError`(Review Focus 4): Store のアカウントが認証エラーなら、`ReadMany` を呼ばず、行に「状態を更新できませんでした: {表示名}(@{ログイン名})の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください」。`ReadMany` が `TokenRejected` を返したら `Rejected` を呼び、同じ文言にする。
- `TestReadOnceSkipsStores`(Review Focus 5): 閲覧のみ・非表示・アカウントなしの Store は問い合わせず、文言も置かない。削除済みのアイテムのリンクの番号は頼まない。
- `TestReadOnceRenamed`: `Batch.Repo` が `org/web2` なら、D3 の改名と同じにリンクの名前を書き換える(`writeFound` の改名の規則のまま)。記録のキーは古い名前のまま(次の読み取りは、書き換えた名前のリンクで数える)。
- `TestReadOnceForce`: `force` なら、間隔の途中でも読む(e2e の入口)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run TestReadOnce`
Expected: FAIL

- [ ] **Step 3: `read.go` を書く**

流れ: `StoreGH.List` → 対象の Store(D4a-9)ごとに、すべてのワークスペース・プロジェクト・削除済みでないアイテムのリンクを集め、リポジトリ(小文字)ごとに Merged でない番号とリンクを持つ → `Reads.Records(store)` → `ghread.Due` のリポジトリだけ → アカウントごとに利用上限で止めたかを覚え(D4a-6)、`Tokens.TokenFor` → `GitHub.ReadMany` → 記録(試した時刻・失敗・`Rate`・読まない期限)を `SaveRecord` → 失敗なら対象のリンクに `setFailure`、成功なら `p.fetched` に入れ、`Missing` に文言を置き、見つかったリンクは `setFailure(key, "")` と `writeFound(…, p)`。読み取りの最後に、動いたルールの通知を書き、チェックの結果か失敗の文言が変わった Store ごとに `EventGitHub` を 1 回流す。関数 60 行・複雑度 15 を超えないよう、集める・決める・書くを分ける。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink
git commit -m "feat(app): GitHub を読む間隔ごとにリンクを取り直す(D4a Task 12)"
```

---

### Task 13: `Reader`(間隔・待つ条件・確かめ直し)と組み立て

**Files:**
- Create: `internal/app/githublink/reader.go`、`internal/app/githublink/reader_test.go`
- Modify: `internal/app/syncing/auto.go`(`Waiting`)、`internal/app/syncing/auto_test.go`
- Modify: `cmd/hedgehug/wire.go`、`cmd/hedgehug/wire_e2e.go`、`cmd/hedgehug/links_e2e.go`、`internal/adapter/httpapi/routes.go`(`Links` の組み立て)

**Interfaces:**
- Produces:
  ```go
  // Waiting は、見えているタブが待つ条件にあるか、オフラインのタブがあるか(06。D4a-8)。期限の切れた知らせは数えない。
  func (a *Auto) Waiting() bool

  // Reader は、GitHub を自動で読む(07「GitHub を読む間隔」)。
  type Reader struct {
      Links    Links
      Settings port.SyncSettings
      Waiting  func() bool // cmd で syncing.Auto.Waiting を渡す
      // 内部: 前回に確かめ直した時刻
  }
  // Step は、自動で読む条件のときだけ、確かめ直し(間隔ごとに 1 回)と ReadOnce を行う。
  func (r *Reader) Step(ctx context.Context) error
  // Run は、ctx が終わるまで 1 秒ごとに Step を呼ぶ。失敗はログに出して続ける(syncing.Auto.Run と同じ形)。
  func (r *Reader) Run(ctx context.Context)
  ```
- e2e の入口: `POST /e2e/github-read` で `Links.ReadOnce(ctx, every, true)` を 1 回行う(`/e2e/github` と同じトークンの確かめ)。

- [ ] **Step 1: 失敗するテストを書く**

- `TestAutoWaiting`(`syncing`): `Hold("t1", true)` のあと `Waiting() == true`、30 秒(`holdFor`)過ぎると `false`。`Network("t1", false)` のあと `true`。
- `TestReaderStopsWhenAutoOff`: `Timing.Auto == false` なら、`ReadOnce` も `Recheck` もしない(偽の GitHub が呼ばれない)。
- `TestReaderWaits`: `Waiting` が true のあいだは読まず、false になった次の `Step` で読む。
- `TestReaderRecheckEveryInterval`: 最初の `Step` で `Recheck` を 1 回、4 分後の `Step` では呼ばず、5 分後に呼ぶ。`Recheck` が true を返したら `EventGitHub` を流す。
- `TestReaderUsesGitHubEvery`: `GitHubEvery` が 30 分なら、`ReadOnce` に 30 分を渡す。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/syncing/ ./internal/app/githublink/ -run 'TestAutoWaiting|TestReader'`
Expected: FAIL

- [ ] **Step 3: 書いて組み立てる**

`wire.go` で `githublink.Links` を 1 つ作り(`Tokens` は同期と同じ `Connector`、`Reads: db.GitHubReads()`、`Inbox` は `httpapi.Deps` と同じもの)、`httpapi.Deps` に渡して `routes.go` はそれを使う(いまは routes.go が組み立てているので、`Deps` に `Links githublink.Links` を足して受け取る形にする。adapter は app を import してよい)。`go (&githublink.Reader{Links: links, Settings: file.SyncSettings(), Waiting: auto.Waiting}).Run(ctx)`。`wire_e2e.go` は `Reader` を動かさず、`githubReadHandler` を `wrap` に足す(`links_e2e.go`)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go build ./... && go build -tags e2e ./... && go test ./internal/app/... ./internal/adapter/... ./cmd/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink internal/app/syncing internal/adapter cmd
git commit -m "feat: GitHub を読む間隔で裏で読み、認証エラーを確かめ直す(D4a Task 13)"
```

---

### Task 14: 画面の読み直しと e2e

**Files:**
- Modify: `web/app/src/wire.ts:204`(github の知らせでアカウントの一覧を読み直す)
- Modify: `web/e2e/links.spec.ts`、`web/e2e/helpers.ts`(`/e2e/github-read` を呼ぶ関数)

- [ ] **Step 1: e2e を書く**(3 幅。既存の `links.spec.ts` の組み立てと `/e2e/github` を使う)

- 「間隔ごとの取り直しで、詳細パネルのリンクの行が変わる」: Open の PR をリンクし、詳細パネルを開いたまま、`/e2e/github` で Merged に変えて `/e2e/github-read` を呼ぶと、行のチップが Merged になる(SSE で読み直す)。
- 「取り直しの失敗の文言が、開き直しても残り、次の成功で消える」: 番号を消して(`gone`)`/e2e/github-read` → 行に「状態を更新できませんでした: {repo}#{n} が見つかりません」。ページを読み直しても出る。値を戻して `/e2e/github-read` → 消える。
- 「PR のマージで自動化が動く」: 設定の自動化で「リンクした PR がマージされたとき → ステータスを Done に」のルールを作り(`automation.spec.ts` の手順)、Open の PR をリンクしたアイテムで、PR を Merged にして `/e2e/github-read` → アイテムのステータスが Done になり、履歴に「「{ルール名}」でステータスを Done に変更」が出る。

- [ ] **Step 2: e2e が落ちることを確かめる**

Run: `ps aux | grep hedgehug-e2e` で別の worktree のサーバーがないことを確かめてから `cd web && pnpm exec playwright test e2e/links.spec.ts`
Expected: 新しい 3 件が FAIL(`/e2e/github-read` は Task 13 で入っているので、落ちる理由は画面の読み直しか、まだ書いていない手順)

- [ ] **Step 3: `wire.ts` の `reflect` で、`kinds` に `github` があれば `accountList.reload()` も呼ぶ**

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm exec playwright test e2e/links.spec.ts && pnpm run check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "test(e2e): 間隔ごとの取り直し・失敗の文言・マージでの自動化を確かめる(D4a Task 14)"
```

---

### Task 15: 仕上げ

- [ ] **Step 1: すべての確認**

Run: `make check`(lint・Go のテスト・web の確認・archtest・大きさの上限)と、e2e 全体を 1 回。
Expected: すべて緑。

- [ ] **Step 2: 最終レビュー**(subagent-driven-development の最後の全体レビュー。Review Focus の 5 つを確かめさせる)

- [ ] **Step 3: PR を出す**

向き先 develop、タイトル「feat: トラック D4a(GitHub を読む間隔と自動化のきっかけ)」。本文に、できること・決めたこと(D4a-1〜D4a-19)・確認の結果・使ったスキルと省いた手順を書く。D4b の範囲は D4a-1 のとおりで、この PR の残りではない。

---

## A4b が入ったときに足すもの

develop にはまだアイテムの削除の印がない(A4b が足す)。D4a は削除済みのアイテムを見分けられないので、A4b が develop に入ったら、次をこの順に足す。コードの「削除の印は A4b が足す」のコメントは、この節を指す。

| 場所 | 足すこと | 確かめるテスト |
|---|---|---|
| `githublink.fire`(`fire.go`) | 削除済みのアイテムでは、ルールを動かさずに何も返さない(エラーにしない)。リンクの値は書く | `TestFireSkipsDeletedItem`(03「削除済みのアイテムには動かない」)。Task 11 の Step 1 の一覧にあるが、削除の印がないのでまだ書いていない。A4b が入ったときに書く |
| `githublink.collectProject`(`read_collect.go`。`ReadOnce`) | アイテムの一覧を、削除済みを除いた読み手(A4b の live-items の読み手)で読み、削除済みのアイテムのリンクの番号を頼まない(D4a-10) | `ReadOnce` のテストを足す: 削除済みのアイテムだけがリンクしている番号は `ReadMany` に頼まず、文言も置かない |
| `githublink.writeRefresh`・`readItem`(`write.go`・`links.go`。詳細パネルを開いたときの削除済みのアイテムの取り直し) | D4a-10 は「削除済みのアイテムのリンクも、詳細パネルを開いたときは取り直す」とする。一方 A4b-4 は「削除済みのアイテムの読み取りは 404」とする。どちらを採るかは A4b が決め、決めたほうに合わせて `readItem` と Refresh のテストを直す | A4b の決定に合わせて、Refresh の削除済みのアイテムのテストを足す |
