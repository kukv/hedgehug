# hedgehug トラック D3(PR と Issue のリンク)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 詳細パネルの GitHub の欄で、PR・Issue を手でリンク・解除でき、開いたときに GitHub から取り直す(改名・移管の追従、チェックの結果を含む)。プロジェクトの概要に「PR と Issue」の一覧を出す。

**Architecture:** リンクの読み方と書き方(変更記録の `links`・`links_off`・`links/<種別:リポジトリ#番号>/<title|state|review>`)は新しい `domain/itemlink` に置き、`domain/item.Item` には足さない(トラック D の決定 6)。usecase は `app/github` に置き(Connector と同じパッケージでないと使えないため)、GitHub への問い合わせは `port.GitHub.Lookup`(GraphQL)、チェックの結果は `port.Checks`(state.db)で持つ。画面は core の `features/github`(状態と規則)、api の `links.ts`(生成された関数の包み)、app の `screens/github`(見た目)に分ける。

**Tech Stack:** Go 1.27.3、net/http(GitHub の GraphQL API `POST /graphql` と REST `GET /repos/{owner}/{name}`)、modernc.org/sqlite(state.db)、huma v2。画面は Svelte 5・TypeScript・Vitest・Playwright。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md`「GitHub を使わない場合」「GitHub との通信に使うアカウント」「GitHub との通信に失敗したとき」「PR と Issue のリンク」の冒頭・「追加」・「表示と更新」・「解除」、「アクティビティに残る文」
- 02 の 331 行(閲覧のみの無効)、438〜442 行(GitHub の欄・PR 用の印)、490・673・674 行(履歴)、554 行(概要の「PR と Issue」)
- 06 の 160 行(チェックの結果の反映)、199・211 行(値を複数持つ項目)、513・522・524〜526 行(アクティビティの文)、538 行(最も早い 1 行)
- 01 の 51 行(PR 用の印)、76 行・246 行(チェックの結果はこのマシンだけ)、94 行(人でない保存の作者)、122〜128 行(閲覧のみ)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` の 103〜106 行(`links`・`links_off`)、154 行(`add`・`remove` と `set links/...`)、264〜266 行(アクティビティと履歴)、282 行・320 行(チェックの結果は state.db)、319 行(解除と付け直し)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-d.md`(決定 1〜12、小計画 D3 の行)
- `docs/superpowers/plans/2026-11-13-hedgehug-track-d2-screen.md` の決定 S2・S15・S20・S23

## Global Constraints

トラック D の計画の Global Constraints をすべて引き継ぐ。とくに次を守る。

- トークンは、ログ・エラーの文・API の応答・Store・config.yaml のどこにも書かない。
- GitHub への 1 回の問い合わせは 30 秒で打ち切る(`infra/github.Client` の `timeout`)。
- GitHub との通信は、その Store に割り当てたアカウントで行う(`Connector.TokenFor`)。別のアカウントで読み直さない。
- 断りの文言は 07 の表のとおりに書き、言い換えない。文言はサーバーの domain が `refusal.New` で持ち、API は 422 の `detail` で返す。画面は `detail` をそのまま出す。
- 仕様にない振る舞いを足さない。モックの振る舞いは決定ではない。
- 大きさの上限: Go は 1 ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・1 パッケージ 20 ファイル(テストを除く)。Svelte 200 行、TS 300 行。`infra/gitstore`・`app/apptest` は 20 ファイルなので**新しいファイルを足さない**。`app/port` は 19 ファイルで、この計画では足さない。`web/app/src/wire.ts` と `web/packages/core/src/index.ts` は 300 行なので、Task 11 で先に減らす。
- 依存は版を明記し、公開から 7 日以上の版だけを使う。この計画は新しい依存を足さない。
- e2e は 4173 番を他の worktree と共有している。流すときは設定を写した一時ファイルで番号を変え、`reuseExistingServer: false` にする(終わったら消す)。
- PR は 1 本(`feat/track-d3` → `develop`)。トラック D の計画の「タスクごとにブランチ」「共有部分を先に小さな PR」は採らない(ユーザーの指示「PR はトラックで 1 本」)。

---

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| D3-1 | 1 本のリンクの値は `itemlink.Ref{Kind, Repo, Number}`(`add`・`remove` の値)で、タイトル・状態・レビューは `set` の項目 `links/<kind>:<repo>#<number>/title`・`/state`・`/review`。解除の記録は `links_off` に同じ `Ref` を `add` する | 保存形式 154・319 行。トラック D の決定 6 |
| D3-2 | リンクの並び(「リンクした順」)は、そのリンクをいまの形にした最後の `add` の行がログの何番目かで決める | 02:554。`change.Resolve` の一覧は値の辞書順なので、並びは itemlink が決める |
| D3-3 | 「もうリンクしています」と Issue の 1 対 1 の比べ方は、リポジトリの大文字小文字を区別せず、番号が同じもの(種別は問わない)。保存するリポジトリ名は GitHub が返した名前(`nameWithOwner`)にする | 07「追加」の表。GitHub の正式な名前で持てば、改名の追従と比べやすい |
| D3-4 | GitHub への問い合わせは GraphQL の `repository(owner,name){ nameWithOwner issueOrPullRequest(number) {...} }` 1 回で行う。レビューの「GitHub が承認済み・修正依頼と判定していればそれ」(`reviewDecision`)は GraphQL にしかないため。リポジトリが `NOT_FOUND` のときは、REST の `GET /repos/{owner}/{name}` で改名・移管のリダイレクトを追い、名前が変わっていれば新しい名前で問い合わせ直す | 07「表示と更新」の改名・移管。前回の実装(main の `internal/lib/github/graphql.go`)も GraphQL。トラック D の計画の Tech Stack(REST)からの変更 |
| D3-5 | GraphQL の失敗の分け方: パスが `repository` の `NOT_FOUND`・`FORBIDDEN`、または `repository` が null → 「リポジトリを読めない」。パスが `repository.issueOrPullRequest` の `NOT_FOUND`、または値が null → 「番号の PR・Issue がない」。`RATE_LIMITED` → 利用上限(解除の時刻は `X-RateLimit-Reset`)。ほか → 「通信できない」。GitHub App 未導入の見分けは D4(D2 の計画の決定 D2-4) | 07 の表。GitHub は「ない」と「権限がない」を区別しない |
| D3-6 | チェックの表示は、`statusCheckRollup.state` が `SUCCESS` → 「CI 成功」、`FAILURE`・`ERROR` → 「CI 失敗」、`PENDING`・`EXPECTED` → 「CI 実行中」、なし → 出さない | 前回の実装と同じ |
| D3-7 | チェックの結果は state.db の表 `github_checks(store, link, checks, at)` に置く(`link` は `Ref.Key()`)。口は `port.Checks`(`port/github.go` に足す。port のファイルを増やさない)。変わったら `port.EventGitHub` の出来事(SSE の `event: github`)を出し、画面は開いている GitHub の欄と概要の一覧を読み直す | 保存形式 282・296・320 行、06:160 |
| D3-8 | 詳細パネルを開いたときの取り直しは、画面が一覧(`GET .../links`)を出したあとに `POST .../links/refresh` を呼ぶ。取り直しは Merged の PR も含めて、開いたアイテムのリンクをすべて読む(Merged を除くのは間隔ごとの取り直しだけ。D4) | 07「表示と更新」の 1 つ目と 2 つ目の箇条 |
| D3-9 | 取り直しをしないとき: 閲覧のみの Store(01)、このマシンにアカウントが 1 つもないとき(07「GitHub を使わない場合」の「更新はしない」。行に失敗の文言も出さない)。アカウントはあるが Store に割り当てがないときは、問い合わせずに、各行に「状態を更新できませんでした: {リポジトリ} を読めるアカウントがありません。…」を出す(07「GitHub との通信に使うアカウント」) | 07:21 と 07:160 は表示が違う |
| D3-10 | 取り直しで値が変わったリンクは、リンクごとに 1 回の保存にする(実行者「GitHub」)。1 回の保存の中では、変わった項目ごとに `set` の行を書く。アクティビティの 1 行(`Operation.Summary`。コミットの件名)は、06 の文(改名、状態、レビュー、タイトルの順)を「、」でつないだもの。アクティビティの画面は変更記録から 1 行ずつ作る(保存形式 263 行)ので、06:538 の「最も早い 1 行」は変更記録の単位で成り立つ | 01:100「1 回の操作が 1 回の保存」。取り直しは操作でないため、リンクを単位にした |
| D3-11 | 改名・移管の追従は、同じ保存で、旧い `Ref` の `remove`、新しい `Ref` の `add`、新しい項目の `set`(タイトル・状態・レビューのすべて。from は旧いリンクの値)を書く。書き換えると、同じアイテムの別のリンク、または別のアイテム(このマシンのすべての Store)の Issue のリンクと同じになるときは書き換えず、旧い名前のまま値だけを取り直す。解除の記録(`links_off`)は書き換えない(07 は「保存したリポジトリ名」=リンクだけを書き換えると書いている) | 07「表示と更新」の最後から 2 つ目の段落 |
| D3-12 | 履歴(詳細パネル)には、手での追加と解除を出さない。実行者「GitHub」の行だけを出す: `set links/.../state`(「{リポジトリ}#{番号} の状態を {状態} に変更」)、`/review`(「…のレビューを {レビュー} に変更」)、`/title`(「…のタイトルを「{タイトル}」に変更」)、同じ保存に `remove` がある `add`(「リンクを {旧}#{番号} から {新}#{番号} に更新」)、`remove` のない `add`(「{リポジトリ}#{番号} を自動でリンク」。D4 が書く)。`from` が null の `set`(リンクしたときの初めの値)と、`from` と `to` が同じ `set` は出さない | 02:490・673・674、保存形式 266 行。07:372 の「履歴に」は、02 と保存形式に合わせて手での追加・解除には当てない |
| D3-13 | 手でリンクしたときの自動化のきっかけ(03:472〜475)は、この小計画では起こさない。きっかけの入口は `app/items` の `automate` にあり、`app/github` からは import できない(app の機能どうしは import しない)。D4「PR・Issue の状態で動く自動化」で、入口を domain に移してから足す | 設計書 4 節。トラック D の小計画の一覧で、きっかけは D4 |
| D3-14 | 閲覧のみの Store への書き込みは、サーバーでは断らない(既存の操作と同じ。画面で無効にする)。取り直しだけはサーバーが `StoreGitHubInfo.ReadOnly` を見て問い合わせない(自動の書き込みなので画面では止められない)。`port.StoreGitHubInfo` に `ReadOnly` を足す | 01:122。A2 の計画 837 行 |
| D3-15 | Issue の 1 対 1 の検査は、`StoreGitHub.List` のすべての Store(非表示・閲覧のみを含む)の、すべてのアイテムのログを読む(アイテムの削除はまだ作られていないので、いまあるアイテムがすべて)。自分の Store は保存の中(tx)で読み直し、ほかの Store は保存の前に読む | 07「追加」。`port.Stores.Write` の約束(Write の中で同じ Store を Read しない) |
| D3-16 | 「プロジェクトのリポジトリにも追加する」は、チェックがオンで、プロジェクトのリポジトリに同じ名前(大文字小文字を区別しない)がないときだけ、同じ保存でプロジェクトの `repos` に GitHub の名前を `add` する。アクティビティは「…をリンク(プロジェクトのリポジトリにも追加)」。プロジェクトの履歴には出ない(既存の履歴はリポジトリの add を出さない) | 07「追加」、02:566 |
| D3-17 | API: `GET /api/stores/{store}/items/{item}/links`(list-links)、`POST .../links/refresh`(refresh-links)、`POST .../links`(add-link)、`POST .../links/remove`(remove-link)、`GET /api/stores/{store}/projects/{project}/links`(list-project-links)。番号は文字列で受け取り、サーバーが「番号を入力してください」を判定する(検査はサーバーに任せる。D2 画面の決定 S15) | A4a のリポジトリの API と同じ形 |
| D3-18 | 「GitHub のアカウントを登録すると使えます」(07「GitHub を使わない場合」)は、GitHub の欄が `listAccounts` を読み、0 件なら「PR をリンク」「Issue をリンク」を無効にして `title` に添える(閲覧のみの「閲覧のみの Store です」と同じ出し方)。閲覧のみのときは閲覧のみの文言を優先する | D2 画面の決定 S2(入口を作るトラックが作る) |
| D3-19 | 「PR 用の印をコピー」は GitHub の欄の見出しの横に置き(位置は仕様にない。概要の「リポジトリを追加」と同じく見出しの横)、`hedgehug:{内部 ID}` をコピーする。閲覧のみでも押せる | 02:440・331、01:51 |
| D3-20 | GitHub の欄の行は「{種別のチップ} {状態のチップ} [レビュー] [チェック] {owner/name}#{番号} — {タイトル}」と解除の「×」。外部リンク(`https://github.com/{repo}/pull/{n}`・`/issues/{n}`)を `{owner/name}#{番号} — {タイトル}` に付け、新しいタブで開く。取り直しに失敗した行は、その下に「状態を更新できませんでした: {理由}」を赤字で出す | 07「表示と更新」、02:331(外部リンクは閲覧のみでも使える) |
| D3-21 | プロジェクトの概要の「PR と Issue」は、A4a の `OverviewRepos.svelte` の「リポジトリと PR」区画の下段に置く。行の ID は詳細パネルを開くリンク。「PR を探せませんでした: {理由}」(自動リンクの読み取りの失敗)は D4 | 02:553・554、A4a の計画 51 行 |
| D3-22 | 解除が断り(Refused)以外で失敗したときの文言は、仕様にないので「リンクを解除できませんでした」とする(A4a の `relationFailedMessage` と同じ扱い)。ユーザー承認済み(2026-11-13) | 仕様にない |
| D3-23 | e2e では、偽の GitHub(`apptest.GitHub`)に PR・Issue を持たせる入口 `POST /e2e/github`(`cmd/hedgehug/links_e2e.go`)を足し、Store `e2e` と `readonly` にアカウント `e2e-account`(ログイン名 `e2e-bot`)を割り当てる。アカウント 0 件の表示は `page.route("**/api/accounts")` の差し替えで確かめる(D2 画面の決定 S23 と同じ) | e2e の偽物はプロセスの中の Go の偽物 |

## 範囲の外にしたもの(このトラックの別の小計画が作る)

| もの | 小計画 | 理由 |
|---|---|---|
| 手でリンクしたときの自動化のきっかけ、条件「リンク中の PR がすべてマージ済みのときだけ」(`AllMerged`)の判定 | D4 | D3-13。`AllMerged` はいまルールの値として持つだけで、判定に使うリンクの値は D3 が足す itemlink から読む |
| GitHub を読む間隔ごとの取り直し、PR の自動リンク、「PR を探せませんでした」、GitHub App 未導入の見分け | D4 | 小計画の一覧 |
| 同期で Issue の 1 対 1 が崩れたときの修復、診断の「Issue の二重リンク」 | C5 | トラック C の計画 67 行 |
| 本物のアプリでのリンク | G | 本物の `StoreGitHub` は G が作る。いまの `noStoreGitHub` は Store を返さないので、本物のアプリでは「読めるアカウントがありません」になる(e2e と usecase のテストで確かめる) |

## ファイルの地図

**Go(新規)**
- `internal/domain/itemlink/itemlink.go` — 型(Ref・Link・Values・Kind・State・Review・Checks)、`FromLog`、表示の言葉
- `internal/domain/itemlink/drafts.go` — 追加・解除・取り直し・改名の変更記録の下書き
- `internal/domain/itemlink/text.go` — 断りの文言、アクティビティの文
- `internal/domain/itemlink/*_test.go`
- `internal/infra/github/lookup.go`(+ `lookup_test.go`) — GraphQL の問い合わせと改名の追従
- `internal/infra/statedb/checks.go`(+ `checks_test.go`) — `github_checks` の表
- `internal/app/github/links.go` — `Links`(口の束)と共通の読み方
- `internal/app/github/link_add.go`・`link_remove.go`・`link_list.go`・`link_refresh.go`・`link_project.go`
- `internal/app/github/links_test.go`・`link_refresh_test.go`
- `internal/adapter/httpapi/github/links.go`(+ `links_test.go`)
- `cmd/hedgehug/links_e2e.go`

**Go(変更)**
- `internal/app/port/github.go`(`Lookup`・`Found`・`Checks`)、`internal/app/port/storegithub.go`(`ReadOnly`)、`internal/app/port/events.go`(`EventGitHub`)
- `internal/app/apptest/github.go`(`Lookup` の偽物、`Checks` の偽物)
- `internal/infra/github/client.go`(GET と POST の共通化)
- `internal/infra/gitstore/yaml.go`・`yaml_notes.go`(+ `yaml_notes_test.go`)
- `internal/domain/history/item.go`(+ `history_test.go`)
- `internal/adapter/httpapi/routes.go`、`internal/adapter/httpapi/github/accounts.go`(`Services` に `Links`)、`internal/adapter/httpapi/events/events.go`(`event: github`)
- `cmd/hedgehug/wire.go`・`wire_e2e.go`・`device_e2e.go`(`wrap` から `/e2e/github` へ)
- `internal/infra/github/live_test.go`
- 生成物: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**画面(新規)**
- `web/packages/core/src/model/link.ts`、`web/packages/core/src/port/links.ts`
- `web/packages/core/src/features/github/links.ts`・`linkDialog.ts`・`projectLinks.ts`・`text.ts`(+ テスト)
- `web/packages/core/src/accountsIndex.ts`・`githubIndex.ts`
- `web/packages/api/src/links.ts`
- `web/app/src/accounts.ts`・`web/app/src/github.ts`・`web/app/src/reloads.ts`
- `web/app/src/screens/github/LinkSection.svelte`・`LinkRow.svelte`・`LinkChips.svelte`・`LinkDialog.svelte`・`ProjectLinks.svelte`
- `web/e2e/links.spec.ts`

**画面(変更)**
- `web/packages/core/src/index.ts`、`web/packages/core/src/model/sync.ts`、`web/packages/core/src/features/sync/reflection.ts`
- `web/packages/api/src/index.ts`・`sync.ts`
- `web/app/src/wire.ts`、アカウントの状態を import している画面
- `web/app/src/screens/item/DetailPanel.svelte`、`web/app/src/screens/project/OverviewRepos.svelte`

## タスクの一覧

| # | タスク | 主なファイル |
|---|---|---|
| 1 | itemlink: 型と読み方 | `domain/itemlink/itemlink.go` |
| 2 | itemlink: 下書きと文言 | `domain/itemlink/drafts.go`・`text.go` |
| 3 | item.yaml に links・links_off | `infra/gitstore/yaml.go`・`yaml_notes.go` |
| 4 | 履歴に GitHub の行 | `domain/history/item.go` |
| 5 | 口と偽物(Lookup・Checks・ReadOnly・EventGitHub) | `app/port`・`app/apptest/github.go` |
| 6 | GitHub の問い合わせ(GraphQL と改名) | `infra/github/lookup.go`・`client.go` |
| 7 | state.db のチェックの結果 | `infra/statedb/checks.go` |
| 8 | usecase: 一覧・追加・解除 | `app/github/links.go`・`link_list.go`・`link_add.go`・`link_remove.go` |
| 9 | usecase: 取り直しとプロジェクトの一覧 | `app/github/link_refresh.go`・`link_project.go` |
| 10 | API・組み立て・e2e の偽物・github-live | `httpapi/github/links.go`・`routes.go`・`cmd/hedgehug` |
| 11 | 画面の場所を空ける(wire.ts・index.ts を分ける) | `app/src/accounts.ts`・`core/src/accountsIndex.ts` |
| 12 | core: 型・口・api の包み・SSE の `github` | `model/link.ts`・`port/links.ts`・`api/src/links.ts` |
| 13 | core: GitHub の欄の状態 | `features/github/links.ts`・`text.ts` |
| 14 | core: リンクのダイアログ | `features/github/linkDialog.ts` |
| 15 | 画面: GitHub の欄とダイアログ | `screens/github/*`・`DetailPanel.svelte`・`app/src/github.ts` |
| 16 | 概要の「PR と Issue」 | `features/github/projectLinks.ts`・`ProjectLinks.svelte` |
| 17 | e2e | `web/e2e/links.spec.ts` |

順: 1→2→(3・4・5 は並べてよい)→6・7→8→9→10→11→12→13→14→15→16→17。

---
### Task 1: itemlink の型と読み方

**Files:**
- Create: `internal/domain/itemlink/itemlink.go`
- Test: `internal/domain/itemlink/itemlink_test.go`

**Interfaces:**
- Consumes: `change.Record`・`change.Resolve`・`change.Options`・`change.NewAdd`・`change.NewSet`・`change.NewRemove`(`internal/domain/change`)
- Produces:
  - `type Kind string`(`PR = "pr"`・`Issue = "issue"`)、`type State string`(`Open`・`Draft`・`Merged`・`Closed`)、`type Review string`(`NoReview = "none"`・`Pending = "pending"`・`Approved = "approved"`・`ChangesRequested = "changes_requested"`)、`type Checks string`(`NoChecks = ""`・`Success = "success"`・`Failure = "failure"`・`Running = "pending"`)
  - `type Ref struct { Kind Kind; Repo string; Number int }`(JSON は `kind`・`repo`・`number`)、`func (r Ref) Key() string`(`pr:org/web#41`)、`func (r Ref) Display() string`(`org/web#41`)、`func (r Ref) Same(o Ref) bool`(リポジトリの大文字小文字と種別を問わず番号が同じ)
  - `type Values struct { Title string; State State; Review Review }`、`type Link struct { Ref; Values }`
  - `type Set struct { Links []Link; Off []Ref; known map[string]Values }`、`func FromLog(log []change.Record) Set`、`func (s Set) Find(r Ref) (Link, bool)`(`Same` で探す)、`func (s Set) HasIssue() (Link, bool)`
  - 項目名: `const LinksField = "links"`、`const OffField = "links_off"`、`func ValueField(r Ref, name string) string`(`links/pr:org/web#41/state`)

- [ ] **Step 1: 失敗するテストを書く**

```go
package itemlink_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

var base = time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)

// logOf は、下書きを 1 行ずつ、1 秒ずつずらした時刻で並べたログにする(prev は前の行)。
func logOf(ds ...change.Draft) []change.Record {
	var out []change.Record
	for i, d := range ds {
		id := change.ID("01TESTREC" + string(rune('A'+i)) + "0000000000000000")
		r := change.Record{ID: id, Op: id, At: base.Add(time.Duration(i) * time.Second), By: "taro",
			Actor: change.User("taro"), T: d.T, F: d.F, From: d.From, To: d.To}
		if len(out) > 0 {
			r.Prev = []change.ID{out[len(out)-1].ID}
		}
		out = append(out, r)
	}
	return out
}

var (
	pr41  = itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	pr40  = itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 40}
	iss12 = itemlink.Ref{Kind: itemlink.Issue, Repo: "org/api", Number: 12}
)

func TestRefNames(t *testing.T) {
	if pr41.Key() != "pr:org/web#41" || pr41.Display() != "org/web#41" {
		t.Errorf("Key = %q, Display = %q", pr41.Key(), pr41.Display())
	}
	if itemlink.ValueField(pr41, "state") != "links/pr:org/web#41/state" {
		t.Errorf("ValueField = %q", itemlink.ValueField(pr41, "state"))
	}
	if !pr41.Same(itemlink.Ref{Kind: itemlink.Issue, Repo: "Org/Web", Number: 41}) || pr41.Same(pr40) {
		t.Error("Same must ignore kind and case, and compare numbers")
	}
}

func TestFromLogReadsLinksValuesAndOrder(t *testing.T) {
	log := logOf(
		change.NewAdd("links", iss12),
		change.NewSet(itemlink.ValueField(iss12, "title"), nil, "ログインが落ちる"),
		change.NewSet(itemlink.ValueField(iss12, "state"), nil, "open"),
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "Add login form"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
		change.NewSet(itemlink.ValueField(pr41, "review"), nil, "pending"),
		change.NewSet(itemlink.ValueField(pr41, "state"), "open", "merged"),
		change.NewAdd("links_off", pr40),
	)
	s := itemlink.FromLog(log)
	if len(s.Links) != 2 || s.Links[0].Ref != iss12 || s.Links[1].Ref != pr41 {
		t.Fatalf("Links = %+v, want issue then PR (the order they were linked)", s.Links)
	}
	if got := s.Links[1].Values; got != (itemlink.Values{Title: "Add login form", State: itemlink.Merged, Review: itemlink.Pending}) {
		t.Errorf("PR values = %+v", got)
	}
	if got := s.Links[0].Values; got != (itemlink.Values{Title: "ログインが落ちる", State: itemlink.Open, Review: itemlink.NoReview}) {
		t.Errorf("issue values = %+v (an issue has no review)", got)
	}
	if len(s.Off) != 1 || s.Off[0] != pr40 {
		t.Errorf("Off = %+v", s.Off)
	}
	if l, ok := s.Find(itemlink.Ref{Kind: itemlink.Issue, Repo: "ORG/WEB", Number: 41}); !ok || l.Ref != pr41 {
		t.Errorf("Find = %+v, %v", l, ok)
	}
	if l, ok := s.HasIssue(); !ok || l.Ref != iss12 {
		t.Errorf("HasIssue = %+v, %v", l, ok)
	}
}

func TestFromLogDropsRemovedAndOrdersRelinkLast(t *testing.T) {
	log := logOf(
		change.NewAdd("links", pr41),
		change.NewAdd("links", pr40),
		change.NewRemove("links", pr41),
		change.NewAdd("links", pr41),
	)
	s := itemlink.FromLog(log)
	if len(s.Links) != 2 || s.Links[0].Ref != pr40 || s.Links[1].Ref != pr41 {
		t.Errorf("Links = %+v, want 40 then the relinked 41", s.Links)
	}
	if _, ok := itemlink.FromLog(logOf(change.NewAdd("links", pr41), change.NewRemove("links", pr41))).Find(pr41); ok {
		t.Error("a removed link is still found")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/itemlink/...`
Expected: FAIL(パッケージ `itemlink` がない)

- [ ] **Step 3: 実装を書く**

```go
// Package itemlink は、アイテムの PR・Issue のリンク(07「PR と Issue のリンク」)を、アイテムのログから読み、
// 書く行を作る。リンクの値は「種別・リポジトリ・番号」(Ref)で、タイトル・状態・レビューは
// set の項目 links/<種別:リポジトリ#番号>/<title|state|review> で持つ(保存形式「変更記録」)。
// 解除した PR の記録は links_off に Ref を持つ。domain/item.Item には足さない(トラック D の決定 6)。
package itemlink

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Kind はリンクの種別。
type Kind string

const (
	PR    Kind = "pr"
	Issue Kind = "issue"
)

// State はリンクの状態。Issue は Open・Closed だけ。
type State string

const (
	Open   State = "open"
	Draft  State = "draft"
	Merged State = "merged"
	Closed State = "closed"
)

// Review は PR のレビューの状態。Issue は NoReview。
type Review string

const (
	NoReview         Review = "none"
	Pending          Review = "pending"
	Approved         Review = "approved"
	ChangesRequested Review = "changes_requested"
)

// Checks は PR のチェックの結果(このマシンだけが持つ。Store に保存しない)。
type Checks string

const (
	NoChecks Checks = ""
	Success  Checks = "success"
	Failure  Checks = "failure"
	Running  Checks = "pending"
)

const (
	// LinksField はリンクの項目、OffField は解除した PR の記録の項目。
	LinksField = "links"
	OffField   = "links_off"
	valuesHead = "links/"
)

// Ref はリンクの値(add・remove の値)。
type Ref struct {
	Kind   Kind   `json:"kind"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

// Key は、set の項目名に使う「種別:リポジトリ#番号」。
func (r Ref) Key() string { return fmt.Sprintf("%s:%s#%d", r.Kind, r.Repo, r.Number) }

// Display は「owner/name#番号」。
func (r Ref) Display() string { return fmt.Sprintf("%s#%d", r.Repo, r.Number) }

// Same は、同じ PR・Issue を指すか。リポジトリの大文字小文字と種別は問わない(07「追加」の
// 「もうリンクしています」。GitHub の PR と Issue は番号を共有する)。
func (r Ref) Same(o Ref) bool { return r.Number == o.Number && strings.EqualFold(r.Repo, o.Repo) }

// ValueField は、リンクの値の項目名(links/<種別:リポジトリ#番号>/<name>)。name は title・state・review。
func ValueField(r Ref, name string) string { return valuesHead + r.Key() + "/" + name }

// Values は、GitHub から読んで Store に保存する値。
type Values struct {
	Title  string
	State  State
	Review Review
}

// Link は、いまのリンク 1 本。
type Link struct {
	Ref
	Values
}

// Set は、アイテムのいまのリンクと、解除した PR の記録。Links は「リンクした順」(D3-2)。
type Set struct {
	Links []Link
	Off   []Ref
	// known は、リンクの値の項目のいまの値(外したリンクのものを含む。付け直したときの from に使う)。
	known map[string]Values
}

// FromLog は、アイテムのログからリンクを読む。
func FromLog(log []change.Record) Set {
	st := change.Resolve(log, change.Options{Multi: func(f string) bool { return f == LinksField || f == OffField }})
	s := Set{known: knownValues(st.Values)}
	for _, raw := range st.Lists[LinksField] {
		var r Ref
		if json.Unmarshal(raw, &r) == nil && r.Number > 0 {
			s.Links = append(s.Links, Link{Ref: r, Values: s.valuesOf(r)})
		}
	}
	for _, raw := range st.Lists[OffField] {
		var r Ref
		if json.Unmarshal(raw, &r) == nil && r.Number > 0 {
			s.Off = append(s.Off, r)
		}
	}
	order := addedAt(log)
	slices.SortStableFunc(s.Links, func(a, b Link) int { return order[a.Ref] - order[b.Ref] })
	return s
}

// addedAt は、各リンクの最後の add の行が、ログの何番目か。
func addedAt(log []change.Record) map[Ref]int {
	out := map[Ref]int{}
	for i, r := range log {
		var ref Ref
		if r.T == change.Add && r.F == LinksField && json.Unmarshal(r.To, &ref) == nil {
			out[ref] = i
		}
	}
	return out
}

// knownValues は、set の項目 links/<key>/<name> の値を、key ごとにまとめる。
func knownValues(values map[string]json.RawMessage) map[string]Values {
	out := map[string]Values{}
	for f, raw := range values {
		rest, ok := strings.CutPrefix(f, valuesHead)
		if !ok {
			continue
		}
		i := strings.LastIndex(rest, "/")
		if i < 0 {
			continue
		}
		key, name := rest[:i], rest[i+1:]
		var v string
		_ = json.Unmarshal(raw, &v)
		cur := out[key]
		switch name {
		case "title":
			cur.Title = v
		case "state":
			cur.State = State(v)
		case "review":
			cur.Review = Review(v)
		}
		out[key] = cur
	}
	return out
}

// valuesOf は、r の値。レビューの値がなければ NoReview。
func (s Set) valuesOf(r Ref) Values {
	v := s.known[r.Key()]
	if v.Review == "" {
		v.Review = NoReview
	}
	return v
}

// Find は、r と同じ PR・Issue(Same)のリンクを探す。
func (s Set) Find(r Ref) (Link, bool) {
	for _, l := range s.Links {
		if l.Same(r) {
			return l, true
		}
	}
	return Link{}, false
}

// HasIssue は、Issue のリンクがあれば、その 1 本を返す。
func (s Set) HasIssue() (Link, bool) {
	for _, l := range s.Links {
		if l.Kind == Issue {
			return l, true
		}
	}
	return Link{}, false
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/itemlink/... && make lint`
Expected: PASS。lint が depguard で `domain/itemlink` を知らないと言うときは、`.golangci.yaml` の domain の許可に `internal/domain/itemlink` を足す(ほかの domain のパッケージと同じ書き方)。

- [ ] **Step 5: コミット**

```bash
git add internal/domain/itemlink .golangci.yaml
git commit -m "feat(domain): アイテムの PR・Issue のリンクをログから読む itemlink を足す(D3 Task 1)"
```

---

### Task 2: itemlink の下書きと文言

**Files:**
- Create: `internal/domain/itemlink/drafts.go`、`internal/domain/itemlink/text.go`
- Test: `internal/domain/itemlink/drafts_test.go`、`internal/domain/itemlink/text_test.go`

**Interfaces:**
- Consumes: Task 1 の `Set`・`Ref`・`Link`・`Values`
- Produces:
  - `func (s Set) Attach(r Ref, v Values) []change.Draft` — リンクの add、値の set(いまの値と違う項目だけ。from はいまの値か null)、`links_off` にあれば remove
  - `func (s Set) Detach(l Link) []change.Draft` — リンクの remove。PR なら `links_off` に add
  - `func (s Set) Refresh(l Link, v Values) []change.Draft` — 値が違う項目の set
  - `func (s Set) Rename(l Link, repo string, v Values) []change.Draft` — 旧の remove、新の add、新の値の set
  - `func StateText(s State) string`(Open・Draft・Merged・Closed)、`func ReviewText(r Review) string`(レビュー待ち・承認済み・修正依頼・なし)
  - 断りの文言: `const MsgNumber = "番号を入力してください"`、`const MsgAlready = "もうリンクしています"`、`const MsgHasIssue = "このアイテムには Issue がリンクされています"`、`func MsgWrongKind(r Ref, want Kind) string`、`func MsgIssueTaken(r Ref, id string) string`
  - アクティビティ: `func AddSummary(id string, r Ref, toProject bool) string`、`func RemoveSummary(id string) string`、`func RefreshSummary(id string, old Link, repo string, v Values) string`

- [ ] **Step 1: 失敗するテストを書く**

`drafts_test.go`:

```go
package itemlink_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// rows は、下書きを「種類 項目 from→to」の文字列にする(比べやすくするため)。
func rows(ds []change.Draft) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = string(d.T) + " " + d.F + " " + string(d.From) + "→" + string(d.To)
	}
	return out
}

func refJSON(r itemlink.Ref) string { b, _ := json.Marshal(r); return string(b) }

func TestAttachWritesAddAndValuesAndClearsOff(t *testing.T) {
	s := itemlink.FromLog(logOf(change.NewAdd("links_off", pr41)))
	v := itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}
	got := rows(s.Attach(pr41, v))
	want := []string{
		"add links →" + refJSON(pr41),
		`set links/pr:org/web#41/title null→"Add login form"`,
		`set links/pr:org/web#41/state null→"open"`,
		`set links/pr:org/web#41/review null→"pending"`,
		"remove links_off " + refJSON(pr41) + "→",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Attach =\n%q\nwant\n%q", got, want)
	}
}

func TestAttachAfterDetachUsesKnownValuesAsFrom(t *testing.T) {
	log := logOf(
		change.NewAdd("links", iss12),
		change.NewSet(itemlink.ValueField(iss12, "title"), nil, "旧"),
		change.NewSet(itemlink.ValueField(iss12, "state"), nil, "open"),
		change.NewRemove("links", iss12),
	)
	got := rows(itemlink.FromLog(log).Attach(iss12, itemlink.Values{Title: "新", State: itemlink.Open, Review: itemlink.NoReview}))
	want := []string{
		"add links →" + refJSON(iss12),
		`set links/issue:org/api#12/title "旧"→"新"`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("Attach =\n%q\nwant\n%q (same values are not written again; an issue has no review row)", got, want)
	}
}

func TestDetachRemembersOnlyPRs(t *testing.T) {
	s := itemlink.FromLog(logOf(change.NewAdd("links", pr41), change.NewAdd("links", iss12)))
	pr, _ := s.Find(pr41)
	if got := rows(s.Detach(pr)); !slices.Equal(got, []string{"remove links " + refJSON(pr41) + "→", "add links_off →" + refJSON(pr41)}) {
		t.Errorf("Detach(PR) = %q", got)
	}
	iss, _ := s.Find(iss12)
	if got := rows(s.Detach(iss)); !slices.Equal(got, []string{"remove links " + refJSON(iss12) + "→"}) {
		t.Errorf("Detach(issue) = %q", got)
	}
}

func TestRefreshWritesOnlyChangedValues(t *testing.T) {
	s := itemlink.FromLog(logOf(
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "A"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
		change.NewSet(itemlink.ValueField(pr41, "review"), nil, "pending"),
	))
	l, _ := s.Find(pr41)
	if got := s.Refresh(l, l.Values); len(got) != 0 {
		t.Errorf("Refresh with the same values = %q, want nothing", rows(got))
	}
	got := rows(s.Refresh(l, itemlink.Values{Title: "A", State: itemlink.Merged, Review: itemlink.Approved}))
	want := []string{`set links/pr:org/web#41/state "open"→"merged"`, `set links/pr:org/web#41/review "pending"→"approved"`}
	if !slices.Equal(got, want) {
		t.Errorf("Refresh =\n%q\nwant\n%q", got, want)
	}
}

func TestRenameMovesTheLink(t *testing.T) {
	s := itemlink.FromLog(logOf(
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "A"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
	))
	l, _ := s.Find(pr41)
	moved := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web2", Number: 41}
	got := rows(s.Rename(l, "org/web2", itemlink.Values{Title: "A", State: itemlink.Merged, Review: itemlink.NoReview}))
	want := []string{
		"remove links " + refJSON(pr41) + "→",
		"add links →" + refJSON(moved),
		`set links/pr:org/web2#41/title "A"→"A"`,
		`set links/pr:org/web2#41/state "open"→"merged"`,
		`set links/pr:org/web2#41/review "none"→"none"`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("Rename =\n%q\nwant\n%q", got, want)
	}
}
```

`text_test.go`:

```go
package itemlink_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestTexts(t *testing.T) {
	cases := []struct{ got, want string }{
		{itemlink.MsgWrongKind(itemlink.Ref{Repo: "org/web", Number: 3}, itemlink.PR), "org/web#3 は PR ではなく Issue です"},
		{itemlink.MsgWrongKind(itemlink.Ref{Repo: "org/web", Number: 3}, itemlink.Issue), "org/web#3 は Issue ではなく PR です"},
		{itemlink.MsgIssueTaken(itemlink.Ref{Repo: "org/web", Number: 3}, "HH-7"), "org/web#3 は HH-7 にリンクされています"},
		{itemlink.AddSummary("HH-2", pr41, false), "HH-2 に org/web#41 をリンク"},
		{itemlink.AddSummary("HH-2", pr41, true), "HH-2 に org/web#41 をリンク(プロジェクトのリポジトリにも追加)"},
		{itemlink.RemoveSummary("HH-2"), "HH-2 のリンクを解除"},
		{itemlink.StateText(itemlink.Merged), "Merged"},
		{itemlink.ReviewText(itemlink.ChangesRequested), "修正依頼"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%q, want %q", c.got, c.want)
		}
	}
}

func TestRefreshSummary(t *testing.T) {
	old := itemlink.Link{Ref: pr41, Values: itemlink.Values{Title: "A", State: itemlink.Open, Review: itemlink.Pending}}
	cases := []struct {
		name string
		repo string
		v    itemlink.Values
		want string
	}{
		{"state", "org/web", itemlink.Values{Title: "A", State: itemlink.Merged, Review: itemlink.Pending}, "HH-2 の org/web#41 の状態を Merged に変更"},
		{"review and title", "org/web", itemlink.Values{Title: "B", State: itemlink.Open, Review: itemlink.Approved},
			"HH-2 の org/web#41 のレビューを 承認済み に変更、HH-2 の org/web#41 のタイトルを「B」に変更"},
		{"rename then state", "org/web2", itemlink.Values{Title: "A", State: itemlink.Closed, Review: itemlink.Pending},
			"HH-2 のリンクを org/web#41 から org/web2#41 に更新、HH-2 の org/web2#41 の状態を Closed に変更"},
	}
	for _, c := range cases {
		if got := itemlink.RefreshSummary("HH-2", old, c.repo, c.v); got != c.want {
			t.Errorf("%s: %q\nwant %q", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/itemlink/...`
Expected: FAIL(`Attach`・`MsgWrongKind` などが未定義)

- [ ] **Step 3: 実装を書く**

`drafts.go`:

```go
package itemlink

import (
	"github.com/kukv/hedgehug/internal/domain/change"
)

// Attach は、r を v の値でリンクする行(07「追加」)。値は、いまの値(外したリンクのものを含む)と違う項目だけ
// 書く。解除の記録にあれば、同じ保存で記録から消す(07「解除」の付け直し)。Issue はレビューの行を書かない。
func (s Set) Attach(r Ref, v Values) []change.Draft {
	out := []change.Draft{change.NewAdd(LinksField, r)}
	out = append(out, s.valueDrafts(r, v)...)
	for _, o := range s.Off {
		if o.Kind == r.Kind && o.Same(r) {
			out = append(out, change.NewRemove(OffField, o))
		}
	}
	return out
}

// Detach は、l を外す行。PR なら解除の記録に足す(自動でもう付けない。07「解除」)。
func (s Set) Detach(l Link) []change.Draft {
	out := []change.Draft{change.NewRemove(LinksField, l.Ref)}
	if l.Kind == PR {
		out = append(out, change.NewAdd(OffField, l.Ref))
	}
	return out
}

// Refresh は、GitHub から読んだ v のうち、l の値と違う項目の行(07「表示と更新」)。
func (s Set) Refresh(l Link, v Values) []change.Draft { return s.valueDrafts(l.Ref, v) }

// Rename は、改名・移管を追って、l を repo のリンクに書き換える行(D3-11)。新しい項目の値は、変わらない項目も
// すべて書き、from は旧いリンクの値にする(履歴は from と to が同じ行を出さず、変わった項目だけが
// 「…の状態を {状態} に変更」として出る。D4 の自動化のきっかけもこの行で判定する)。
func (s Set) Rename(l Link, repo string, v Values) []change.Draft {
	moved := Ref{Kind: l.Kind, Repo: repo, Number: l.Number}
	out := []change.Draft{change.NewRemove(LinksField, l.Ref), change.NewAdd(LinksField, moved)}
	out = append(out,
		change.NewSet(ValueField(moved, "title"), l.Title, v.Title),
		change.NewSet(ValueField(moved, "state"), string(l.State), string(v.State)))
	if l.Kind == PR {
		out = append(out, change.NewSet(ValueField(moved, "review"), string(l.Review), string(v.Review)))
	}
	return out
}

// valueDrafts は、r の値の項目のうち、いまの値と違うものの set。from は、いまの値がなければ null。
func (s Set) valueDrafts(r Ref, v Values) []change.Draft {
	cur, had := s.known[r.Key()]
	var out []change.Draft
	add := func(name, from, to string) {
		if from == to && had {
			return
		}
		var f any = from
		if from == "" {
			f = nil
		}
		out = append(out, change.NewSet(ValueField(r, name), f, to))
	}
	add("title", cur.Title, v.Title)
	add("state", string(cur.State), string(v.State))
	if r.Kind == PR {
		add("review", string(cur.Review), string(v.Review))
	}
	return out
}
```

注意: `had` が false のとき(初めての値)は、同じ値でもすべて書く。`had` が true で、ある項目だけが空(過去に書いていない)なら、空と違う値を書く。テスト `TestAttachAfterDetachUsesKnownValuesAsFrom` で state は `"open"` 同士なので書かない。

`text.go`:

```go
package itemlink

import (
	"fmt"
	"strings"
)

// 07「追加」の断りの文言。
const (
	MsgNumber   = "番号を入力してください"
	MsgAlready  = "もうリンクしています"
	MsgHasIssue = "このアイテムには Issue がリンクされています"
)

// MsgWrongKind は、want の種別でリンクしようとした番号が、別の種別だったときの文言。
func MsgWrongKind(r Ref, want Kind) string {
	if want == PR {
		return r.Display() + " は PR ではなく Issue です"
	}
	return r.Display() + " は Issue ではなく PR です"
}

// MsgIssueTaken は、Issue が別のアイテム id(画面の ID)にリンクされているときの文言。
func MsgIssueTaken(r Ref, id string) string { return r.Display() + " は " + id + " にリンクされています" }

// StateText は、状態の表示(07)。
func StateText(s State) string {
	switch s {
	case Open:
		return "Open"
	case Draft:
		return "Draft"
	case Merged:
		return "Merged"
	case Closed:
		return "Closed"
	}
	return string(s)
}

// ReviewText は、レビューの状態の表示(07)。
func ReviewText(r Review) string {
	switch r {
	case Pending:
		return "レビュー待ち"
	case Approved:
		return "承認済み"
	case ChangesRequested:
		return "修正依頼"
	}
	return "なし"
}

// AddSummary は、手でリンクしたときのアクティビティ(07「アクティビティに残る文」)。
func AddSummary(id string, r Ref, toProject bool) string {
	s := fmt.Sprintf("%s に %s をリンク", id, r.Display())
	if toProject {
		s += "(プロジェクトのリポジトリにも追加)"
	}
	return s
}

// RemoveSummary は、解除のアクティビティ。
func RemoveSummary(id string) string { return id + " のリンクを解除" }

// RefreshSummary は、取り直しの保存のアクティビティ(06 の文を、改名・状態・レビュー・タイトルの順に「、」で
// つなぐ。D3-10)。repo は GitHub の名前(変わっていなければ old.Repo)。
func RefreshSummary(id string, old Link, repo string, v Values) string {
	var parts []string
	now := Ref{Kind: old.Kind, Repo: repo, Number: old.Number}
	if repo != old.Repo {
		parts = append(parts, fmt.Sprintf("%s のリンクを %s から %s に更新", id, old.Display(), now.Display()))
	}
	if v.State != old.State {
		parts = append(parts, fmt.Sprintf("%s の %s の状態を %s に変更", id, now.Display(), StateText(v.State)))
	}
	if old.Kind == PR && v.Review != old.Review {
		parts = append(parts, fmt.Sprintf("%s の %s のレビューを %s に変更", id, now.Display(), ReviewText(v.Review)))
	}
	if v.Title != old.Title {
		parts = append(parts, fmt.Sprintf("%s の %s のタイトルを「%s」に変更", id, now.Display(), v.Title))
	}
	return strings.Join(parts, "、")
}
```

`RefreshSummary` のタイトルは改行を含みうる。`port.Operation.Check` は改行を含む Summary を断るので、Task 9 で `strings.ReplaceAll` の代わりに、ここで改行を空白にする: `v.Title` を `strings.Join(strings.Fields(v.Title), " ")` にしてから書く。テストに 1 行足す:

```go
func TestRefreshSummaryIsOneLine(t *testing.T) {
	old := itemlink.Link{Ref: pr41, Values: itemlink.Values{Title: "A", State: itemlink.Open, Review: itemlink.Pending}}
	got := itemlink.RefreshSummary("HH-2", old, "org/web", itemlink.Values{Title: "B\nC", State: itemlink.Open, Review: itemlink.Pending})
	if got != "HH-2 の org/web#41 のタイトルを「B C」に変更" {
		t.Errorf("%q", got)
	}
}
```

実装の最後の `if` を次にする:

```go
	if v.Title != old.Title {
		title := strings.Join(strings.Fields(v.Title), " ")
		parts = append(parts, fmt.Sprintf("%s の %s のタイトルを「%s」に変更", id, now.Display(), title))
	}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/itemlink/... && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/itemlink
git commit -m "feat(domain): リンクの追加・解除・取り直し・改名の行と文言を itemlink に足す(D3 Task 2)"
```

---

### Task 3: item.yaml に links・links_off

**Files:**
- Modify: `internal/infra/gitstore/yaml.go`(`itemYAML` に 2 つ)、`internal/infra/gitstore/yaml_notes.go`(`itemToYAML` と `linksNode`・`offNode`)
- Test: `internal/infra/gitstore/yaml_notes_test.go`

**Interfaces:**
- Consumes: `itemlink.FromLog`・`itemlink.Set`(Task 1)
- Produces: item.yaml の `links:`(estimate のあと、watchers の前)と `links_off:`。行の形は保存形式 103〜106 行:
  ```yaml
  links:
    - {kind: pr, repo: org/web, number: 41, title: Add login form, state: open, review: pending}
  links_off:
    - {kind: pr, repo: org/web, number: 40}
  ```
  Issue の行は `review` を書かない。リンクがなければ両方とも `[]`(ほかの一覧の項目と同じ書き方。`seqOf` の空のときの出力に合わせる)。

- [ ] **Step 1: 失敗するテストを書く**

`yaml_notes_test.go` に足す(`noteID`・`yWS`・`yProj` は同じパッケージのテストにある。import に `itemlink` と `strings` を足す)。

```go
func TestItemYAMLWritesLinks(t *testing.T) {
	pr41 := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	pr40 := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 40}
	iss := itemlink.Ref{Kind: itemlink.Issue, Repo: "org/api", Number: 12}
	var log []change.Record
	n := 0
	newID := func() change.ID { n++; return noteID(fmt.Sprintf("L%d", n)) }
	add := func(minute int, ds ...change.Draft) {
		meta := change.Meta{Op: noteID(fmt.Sprintf("P%d", minute)), At: time.Date(2026, 11, 13, 9, minute, 0, 0, time.UTC), Actor: change.User("taro")}
		log = append(log, change.Stamp(log, ds, meta, newID)...)
	}
	add(0, item.Create(item.Item{ID: noteID("A1"), Place: item.Place{Workspace: yWS, Project: yProj, Number: 1}, Kind: item.Task, Rank: "V", Title: "t"}))
	add(1,
		change.NewAdd("links", pr41),
		change.NewSet(itemlink.ValueField(pr41, "title"), nil, "Add login form"),
		change.NewSet(itemlink.ValueField(pr41, "state"), nil, "open"),
		change.NewSet(itemlink.ValueField(pr41, "review"), nil, "pending"),
		change.NewAdd("links", iss),
		change.NewSet(itemlink.ValueField(iss, "title"), nil, "落ちる"),
		change.NewSet(itemlink.ValueField(iss, "state"), nil, "closed"),
	)
	add(2, change.NewAdd("links_off", pr40))
	out, err := renderYAML(change.KindItem, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "links:\n" +
		"    - {kind: pr, repo: org/web, number: 41, title: Add login form, state: open, review: pending}\n" +
		"    - {kind: issue, repo: org/api, number: 12, title: 落ちる, state: closed}\n" +
		"links_off:\n" +
		"    - {kind: pr, repo: org/web, number: 40}\n"
	if !strings.Contains(string(out), want) {
		t.Errorf("item.yaml =\n%s\nwant to contain\n%s", out, want)
	}
	if i, j := strings.Index(string(out), "estimate:"), strings.Index(string(out), "links:"); i < 0 || j < i {
		t.Errorf("links must come after estimate:\n%s", out)
	}
}
```

字下げ(`    - `)は `encodeYAML` の設定に合わせる。既存の `relations:` のテストの期待値と同じ字下げにする。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ -run TestItemYAMLWritesLinks`
Expected: FAIL(`links:` がない)

- [ ] **Step 3: 実装を書く**

`yaml.go` の `itemYAML` の `Estimate` と `Watchers` のあいだに足す:

```go
	Estimate  *float64    `yaml:"estimate"`
	Links     *yaml.Node  `yaml:"links"`
	LinksOff  *yaml.Node  `yaml:"links_off"`
	Watchers  []string    `yaml:"watchers,flow"`
```

`yaml_notes.go` の `itemToYAML` で、`Relations` の行のあとに:

```go
		Links:     linksNode(links.Links),
		LinksOff:  offNode(links.Off),
```

関数の先頭(`var st` の前)に `links := itemlink.FromLog(log)` を置く。ファイルの末尾に足す:

```go
// linksNode は、PR・Issue のリンクを、リンクした順に 1 件 1 行で書く(保存形式 103〜106 行)。Issue は review を書かない。
func linksNode(ls []itemlink.Link) *yaml.Node {
	rows := make([]*yaml.Node, len(ls))
	for i, l := range ls {
		kv := []*yaml.Node{
			key("kind"), strNode(string(l.Kind)), key("repo"), strNode(l.Repo), key("number"), intNode(l.Number),
			key("title"), strNode(l.Title), key("state"), strNode(string(l.State)),
		}
		if l.Kind == itemlink.PR {
			kv = append(kv, key("review"), strNode(string(l.Review)))
		}
		rows[i] = mapping(true, kv...)
	}
	return seqOf(rows)
}

// offNode は、解除した PR の記録を 1 件 1 行で書く。
func offNode(rs []itemlink.Ref) *yaml.Node {
	rows := make([]*yaml.Node, len(rs))
	for i, r := range rs {
		rows[i] = mapping(true, key("kind"), strNode(string(r.Kind)), key("repo"), strNode(r.Repo), key("number"), intNode(r.Number))
	}
	return seqOf(rows)
}

func intNode(n int) *yaml.Node {
	v := &yaml.Node{}
	_ = v.Encode(n)
	return v
}
```

`mapping` の引数の形(可変長の `*yaml.Node` か)は `yaml_settings.go:43` に合わせる。`intNode` と同じ働きの関数がすでにあれば、それを使い、`intNode` は足さない。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/gitstore/... && make lint`
Expected: PASS。ほかの item.yaml の期待値のテストが `links: []` などの行で落ちたら、期待値に足す(書き出しが変わったのはこの Task の結果なので)。

- [ ] **Step 5: コミット**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): item.yaml に PR・Issue のリンクと解除の記録を書き出す(D3 Task 3)"
```

---

### Task 4: 履歴に GitHub の行

**Files:**
- Modify: `internal/domain/history/item.go`
- Test: `internal/domain/history/history_test.go`

**Interfaces:**
- Consumes: `itemlink.Ref`・`itemlink.StateText`・`itemlink.ReviewText`(Task 1・2)
- Produces: `ItemRows` が、実行者「GitHub」の links の行を出す(D3-12)。

- [ ] **Step 1: 失敗するテストを書く**

`history_test.go:92` の行 `step{"O15", ..., change.NewSet("links/pr:org/web#1/state", "open", "merged")}` は、これまで「出さない」前提だった。期待値 `want` の先頭から 2 番目を変える。行は新しい順で、O16 は出ず、O15 は O12 より新しいので、先頭に `"GitHub: org/web#1 の状態を Merged に変更"` を足す:

```go
	want := []string{
		"GitHub: org/web#1 の状態を Merged に変更",
		"太郎: ステータスを InProgress → (削除された値)",
		...
```

新しいテストを足す:

```go
func TestItemRowsShowGitHubLinkRows(t *testing.T) {
	gh := change.Actor{Kind: change.ActorGitHub}
	pr := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	moved := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web2", Number: 41}
	log := logOf(
		step{"O1", at(0), taro, item.Create(base)},
		step{"O2", at(1), taro, change.NewAdd("links", pr)},                                   // 手でのリンク: 出さない
		step{"O2", at(1), taro, change.NewSet(itemlink.ValueField(pr, "state"), nil, "open")}, // 初めの値: 出さない
		step{"O3", at(2), gh, change.NewSet(itemlink.ValueField(pr, "review"), "none", "approved")},
		step{"O4", at(3), gh, change.NewSet(itemlink.ValueField(pr, "title"), "A", "B")},
		step{"O5", at(4), gh, change.NewRemove("links", pr)},
		step{"O5", at(4), gh, change.NewAdd("links", moved)},
		step{"O5", at(4), gh, change.NewSet(itemlink.ValueField(moved, "title"), "B", "B")},
		step{"O5", at(4), gh, change.NewSet(itemlink.ValueField(moved, "state"), "open", "closed")},
		step{"O6", at(5), gh, change.NewAdd("links", itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 50})},
		step{"O7", at(6), taro, change.NewRemove("links", moved)}, // 手での解除: 出さない
		step{"O8", at(7), gh, change.NewSet(itemlink.ValueField(moved, "state"), "open", "open")},
	)
	want := []string{
		"GitHub: org/web#50 を自動でリンク",
		"GitHub: org/web2#41 の状態を Closed に変更",
		"GitHub: リンクを org/web#41 から org/web2#41 に更新",
		"GitHub: org/web#41 のタイトルを「B」に変更",
		"GitHub: org/web#41 のレビューを 承認済み に変更",
		"太郎: 作成",
	}
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}
```

`step` が同じ op の複数の行を作れない形なら(`logOf` が step ごとに op を分けるなら)、`logOf` を見て、同じ op 名なら同じ `Op` を使うように直す(既存のテストは op 名がすべて違うので影響しない)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/history/...`
Expected: FAIL(GitHub の行が出ない)

- [ ] **Step 3: 実装を書く**

`item.go` の `itemText` を次にする:

```go
func itemText(log []change.Record, i int, c Context) (string, bool) {
	r := log[i]
	if r.Actor.Kind == change.ActorGitHub && strings.HasPrefix(r.F, itemlink.LinksField) {
		return linkText(log, i)
	}
	switch r.T {
	...(今までどおり)
```

`linkText` は新しいファイル `internal/domain/history/links.go` に置く(`item.go` を 200 行未満に保つ):

```go
package history

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// linkText は、実行者「GitHub」の links の行の履歴の文(D3-12。06 の文から「{ID} の」を除いたもの)。
// 手での追加・解除(実行者が人)は呼ばれない。初めの値(from が null)と、値の変わらない set は出さない。
func linkText(log []change.Record, i int) (string, bool) {
	r := log[i]
	switch r.T {
	case change.Set:
		return linkSetText(r)
	case change.Add:
		var to itemlink.Ref
		if r.F != itemlink.LinksField || json.Unmarshal(r.To, &to) != nil {
			return "", false
		}
		if from, ok := removedInOp(log, r.Op); ok {
			return fmt.Sprintf("リンクを %s から %s に更新", from.Display(), to.Display()), true
		}
		return to.Display() + " を自動でリンク", true
	}
	return "", false
}

func linkSetText(r change.Record) (string, bool) {
	if string(r.From) == "null" || string(r.From) == string(r.To) {
		return "", false
	}
	rest := strings.TrimPrefix(r.F, itemlink.LinksField+"/")
	cut := strings.LastIndex(rest, "/")
	colon := strings.Index(rest, ":")
	if cut < 0 || colon < 0 {
		return "", false
	}
	target, name := rest[colon+1:cut], rest[cut+1:]
	var to string
	if json.Unmarshal(r.To, &to) != nil {
		return "", false
	}
	switch name {
	case "state":
		return fmt.Sprintf("%s の状態を %s に変更", target, itemlink.StateText(itemlink.State(to))), true
	case "review":
		return fmt.Sprintf("%s のレビューを %s に変更", target, itemlink.ReviewText(itemlink.Review(to))), true
	case "title":
		return fmt.Sprintf("%s のタイトルを「%s」に変更", target, to), true
	}
	return "", false
}

// removedInOp は、同じ保存(op)で外したリンク(改名・移管の追従。D3-11)。
func removedInOp(log []change.Record, op change.ID) (itemlink.Ref, bool) {
	for _, r := range log {
		var ref itemlink.Ref
		if r.Op == op && r.T == change.Remove && r.F == itemlink.LinksField && json.Unmarshal(r.From, &ref) == nil {
			return ref, true
		}
	}
	return itemlink.Ref{}, false
}
```

`ItemRows` のコメントの「出すのは…」に「実行者「GitHub」のリンクの値の変更・改名・自動リンク(D3-12)」を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/history/... && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/history
git commit -m "feat(history): GitHub が取り直したリンクの値・改名・自動リンクを履歴に出す(D3 Task 4)"
```

---
### Task 5: 口と偽物(Lookup・Checks・ReadOnly・EventGitHub)

**Files:**
- Modify: `internal/app/port/github.go`、`internal/app/port/storegithub.go`、`internal/app/port/events.go`
- Modify: `internal/app/apptest/github.go`(新しいファイルは作らない。apptest は 20 ファイル)
- Test: `internal/app/apptest/github_test.go`(なければ作る。テストのファイルは数に入らない)

**Interfaces:**
- Consumes: `itemlink.Kind`・`State`・`Review`・`Checks`・`Values`(Task 1)
- Produces:
  ```go
  // port/github.go に足す
  type Found struct {
  	Kind   itemlink.Kind
  	Repo   string // GitHub のいまの名前(改名・移管のあと)
  	Number int
  	itemlink.Values
  	Checks itemlink.Checks
  }
  // GitHub に足すメソッド(Task 6 で足す。Task 5 で足すと infra/github がビルドできなくなるため)
  Lookup(ctx context.Context, token, repo string, number int) (Found, error)

  type Checks interface {
  	Read(ctx context.Context, store StoreID) (map[string]itemlink.Checks, error) // キーは Ref.Key()
  	Save(ctx context.Context, store StoreID, key string, c itemlink.Checks) (changed bool, err error) // NoChecks は消す
  }
  // port/storegithub.go の StoreGitHubInfo に ReadOnly bool
  // port/events.go に EventGitHub EventKind = "github"
  ```
  - `apptest.GitHub` に `Items map[string]port.Found`(キーは `strings.ToLower(repo)+"#"+番号`)、`LookupErr map[string]error`(同じキー)、`Lookups int`、`func (g *GitHub) Put(repo string, f port.Found)`
  - `apptest.Checks`(`NewChecks()`)と `apptest.RunChecksContract(t, open func(t *testing.T) port.Checks)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/apptest/github_test.go`:

```go
package apptest_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestGitHubLookup(t *testing.T) {
	g := &apptest.GitHub{Users: map[string]string{"tok": "taro"}}
	f := port.Found{Kind: itemlink.PR, Repo: "org/web2", Number: 41,
		Values: itemlink.Values{Title: "A", State: itemlink.Open, Review: itemlink.Pending}, Checks: itemlink.Success}
	g.Put("Org/Web", f) // 改名前の名前で引いても、いまの名前で返る
	got, err := g.Lookup(t.Context(), "tok", "org/web", 41)
	if err != nil || got != f {
		t.Errorf("Lookup = %+v, %v", got, err)
	}
	if _, err := g.Lookup(t.Context(), "tok", "org/web", 42); !ghcall.Is(err, ghcall.NotFound) {
		t.Errorf("missing number: %v", err)
	}
	if _, err := g.Lookup(t.Context(), "bad", "org/web", 41); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("unknown token: %v", err)
	}
	if g.Lookups != 3 {
		t.Errorf("Lookups = %d", g.Lookups)
	}
}

func TestChecksContract(t *testing.T) {
	apptest.RunChecksContract(t, func(*testing.T) port.Checks { return apptest.NewChecks() })
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/apptest/ -run 'TestGitHubLookup|TestChecksContract'`
Expected: FAIL(`Put`・`Lookup`・`NewChecks` が未定義)

- [ ] **Step 3: 実装を書く**

`port/github.go`(全体):

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// GitHub は GitHub の API の口。失敗は *ghcall.Failure で返す。トラック D が、使う操作だけを足していく。
type GitHub interface {
	// Viewer は、token の持ち主のログイン名を返す(07「認証の確認」)。
	Viewer(ctx context.Context, token string) (login string, err error)
}

// Found は、GitHub から読んだ PR・Issue。
type Found struct {
	Kind   itemlink.Kind
	Repo   string // GitHub のいまの名前
	Number int
	itemlink.Values
	Checks itemlink.Checks // PR のチェックの結果。Issue は NoChecks
}

// Checks は、このマシンが読んだ PR のチェックの結果(07「表示と更新」。Store に保存しない)の口。
// 実装は infra/statedb。
type Checks interface {
	// Read は、Store のリンクのチェックの結果を、Ref.Key() から引く表で返す。
	Read(ctx context.Context, store StoreID) (map[string]itemlink.Checks, error)
	// Save は、結果を置く。NoChecks なら消す。前と変わったら changed。
	Save(ctx context.Context, store StoreID, key string, c itemlink.Checks) (changed bool, err error)
}
```

`port/storegithub.go` の `StoreGitHubInfo` に足す:

```go
	Remote   bool       // remote を持つか
	ReadOnly bool       // 閲覧のみの Store か(01。リンクの取り直しをしない。D3-14)
```

`port/events.go` の定数に足す:

```go
	EventGitHub EventKind = "github" // このマシンが読んだ GitHub の値(チェックの結果)が変わった
```

`apptest/github.go` の `GitHub` を次にし、ファイルの末尾に `Checks` と照合テストを足す(import に `fmt`・`maps`・`strings`・`testing`・`port`・`itemlink` を足す):

```go
// GitHub は port.GitHub の偽物。Users はトークンからログイン名への対応。Err があれば、どの問い合わせもそれを返す。
// Items は Lookup の答え(キーは小文字のリポジトリ名と番号。Put で入れる)。LookupErr は同じキーの失敗。
// Lookups は Lookup を呼んだ回数。
type GitHub struct {
	mu        sync.Mutex
	Users     map[string]string
	Err       error
	Items     map[string]port.Found
	LookupErr map[string]error
	Lookups   int
}

func itemKey(repo string, number int) string { return fmt.Sprintf("%s#%d", strings.ToLower(repo), number) }

// Put は、repo(引くときの名前)の f.Number を f にする。f.Repo が repo と違えば、改名・移管のあとの名前。
func (g *GitHub) Put(repo string, f port.Found) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Items == nil {
		g.Items = map[string]port.Found{}
	}
	g.Items[itemKey(repo, f.Number)] = f
}

// Fail は、repo の number を引くと err を返すようにする(nil で戻す)。
func (g *GitHub) Fail(repo string, number int, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.LookupErr == nil {
		g.LookupErr = map[string]error{}
	}
	if err == nil {
		delete(g.LookupErr, itemKey(repo, number))
		return
	}
	g.LookupErr[itemKey(repo, number)] = err
}

func (g *GitHub) Lookup(_ context.Context, token, repo string, number int) (port.Found, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Lookups++
	if g.Err != nil {
		return port.Found{}, g.Err
	}
	if _, ok := g.Users[token]; !ok {
		return port.Found{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	if err := g.LookupErr[itemKey(repo, number)]; err != nil {
		return port.Found{}, err
	}
	f, ok := g.Items[itemKey(repo, number)]
	if !ok {
		return port.Found{}, &ghcall.Failure{Kind: ghcall.NotFound, Repo: repo, Number: number}
	}
	return f, nil
}
```

```go
// Checks は port.Checks の偽物。
type Checks struct {
	mu   sync.Mutex
	rows map[port.StoreID]map[string]itemlink.Checks
}

func NewChecks() *Checks { return &Checks{rows: map[port.StoreID]map[string]itemlink.Checks{}} }

func (c *Checks) Read(_ context.Context, store port.StoreID) (map[string]itemlink.Checks, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.rows[store]), nil
}

func (c *Checks) Save(_ context.Context, store port.StoreID, key string, v itemlink.Checks) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rows[store] == nil {
		c.rows[store] = map[string]itemlink.Checks{}
	}
	if c.rows[store][key] == v {
		return false, nil
	}
	if v == itemlink.NoChecks {
		delete(c.rows[store], key)
	} else {
		c.rows[store][key] = v
	}
	return true, nil
}

// RunChecksContract は、port.Checks の実装が守ること(偽物と state.db で同じテストを流す)。
func RunChecksContract(t *testing.T, open func(t *testing.T) port.Checks) {
	t.Run("save, read, change and clear", func(t *testing.T) {
		c, ctx := open(t), t.Context()
		if got, err := c.Read(ctx, "S1"); err != nil || len(got) != 0 {
			t.Fatalf("empty Read = %v, %v", got, err)
		}
		mustSave := func(key string, v itemlink.Checks, want bool) {
			t.Helper()
			if changed, err := c.Save(ctx, "S1", key, v); err != nil || changed != want {
				t.Fatalf("Save(%s, %q) = %v, %v; want changed %v", key, v, changed, err, want)
			}
		}
		mustSave("pr:org/web#1", itemlink.Running, true)
		mustSave("pr:org/web#1", itemlink.Running, false)
		mustSave("pr:org/web#1", itemlink.Success, true)
		mustSave("pr:org/web#2", itemlink.Failure, true)
		if got, _ := c.Read(ctx, "S1"); len(got) != 2 || got["pr:org/web#1"] != itemlink.Success || got["pr:org/web#2"] != itemlink.Failure {
			t.Errorf("Read = %v", got)
		}
		if got, _ := c.Read(ctx, "S2"); len(got) != 0 {
			t.Errorf("another store sees %v", got)
		}
		mustSave("pr:org/web#2", itemlink.NoChecks, true)
		mustSave("pr:org/web#2", itemlink.NoChecks, false)
		if got, _ := c.Read(ctx, "S1"); len(got) != 1 {
			t.Errorf("after clearing: %v", got)
		}
	})
}
```

`apptest/github.go` が 300 行を超えるときは、`Secrets` の偽物を同じパッケージの既存の小さなファイル(`accounts.go` など)へ移す(ファイルを増やさない)。

`port.GitHub` の interface には、このタスクでは `Lookup` を足さない(足すと `infra/github.Client` が口を満たさずビルドが落ちる)。偽物の `Lookup` は先に置き、口への追加は Task 6 で実装と一緒に行う。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/... ./internal/domain/... && make check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/port internal/app/apptest
git commit -m "feat(port): PR・Issue を読む Lookup とチェックの結果の口、閲覧のみの印と github の出来事を足す(D3 Task 5)"
```

---

### Task 6: GitHub の問い合わせ(GraphQL と改名の追従)

**Files:**
- Modify: `internal/infra/github/client.go`(`get` を `do` に広げ、POST を通す)
- Create: `internal/infra/github/lookup.go`
- Test: `internal/infra/github/lookup_test.go`

**Interfaces:**
- Consumes: `port.Found`(Task 5)、`ghcall.Failure`
- Produces: `func (c *Client) Lookup(ctx context.Context, token, repo string, number int) (port.Found, error)`。`infra/github` が `app/port` を import するのは設計書の向き(infra → app/port)どおり。

- [ ] **Step 1: 失敗するテストを書く**

`lookup_test.go`:

```go
package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// graphQL は、POST /graphql の本文の変数を読み、answer(owner, name, number) の JSON を返す偽のサーバーの関数。
func graphQL(t *testing.T, answer func(owner, name string, number int) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer "+fakeToken {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var body struct {
			Query     string `json:"query"`
			Variables struct {
				Owner, Name string
				Number      int
			} `json:"variables"`
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil || !strings.Contains(body.Query, "issueOrPullRequest") {
			t.Errorf("body = %s", b)
		}
		_, _ = w.Write([]byte(answer(body.Variables.Owner, body.Variables.Name, body.Variables.Number)))
	}
}

const prJSON = `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":{"__typename":"PullRequest",
"number":41,"title":"Add login form","state":"OPEN","isDraft":false,"reviewDecision":null,
"reviewRequests":{"totalCount":1},"latestOpinionatedReviews":{"nodes":[]},
"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"FAILURE"}}}]}}}}}`

func TestLookupPR(t *testing.T) {
	c, _ := newClient(t, graphQL(t, func(owner, name string, n int) string {
		if owner != "org" || name != "web" || n != 41 {
			t.Errorf("variables %s/%s#%d", owner, name, n)
		}
		return prJSON
	}))
	got, err := c.Lookup(context.Background(), fakeToken, "org/web", 41)
	if err != nil {
		t.Fatal(err)
	}
	want := itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}
	if got.Kind != itemlink.PR || got.Repo != "org/web" || got.Number != 41 || got.Values != want || got.Checks != itemlink.Failure {
		t.Errorf("Lookup = %+v", got)
	}
}

func TestLookupStatesAndReviews(t *testing.T) {
	cases := []struct {
		name, node string
		state      itemlink.State
		review     itemlink.Review
		checks     itemlink.Checks
	}{
		{"draft, no checks", `"state":"OPEN","isDraft":true,"reviewDecision":null,"reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]}`,
			itemlink.Draft, itemlink.NoReview, itemlink.NoChecks},
		{"merged, decision approved", `"state":"MERGED","isDraft":false,"reviewDecision":"APPROVED","reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]}`,
			itemlink.Merged, itemlink.Approved, itemlink.Success},
		{"closed, latest changes requested", `"state":"CLOSED","isDraft":false,"reviewDecision":"REVIEW_REQUIRED","reviewRequests":{"totalCount":1},"latestOpinionatedReviews":{"nodes":[{"state":"APPROVED"},{"state":"CHANGES_REQUESTED"}]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"PENDING"}}}]}`,
			itemlink.Closed, itemlink.ChangesRequested, itemlink.Running},
		{"latest approved only", `"state":"OPEN","isDraft":false,"reviewDecision":null,"reviewRequests":{"totalCount":1},"latestOpinionatedReviews":{"nodes":[{"state":"APPROVED"}]},"commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"ERROR"}}}]}`,
			itemlink.Open, itemlink.Approved, itemlink.Failure},
	}
	for _, cs := range cases {
		c, _ := newClient(t, graphQL(t, func(string, string, int) string {
			return `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":{"__typename":"PullRequest","number":1,"title":"t",` + cs.node + `}}}}`
		}))
		got, err := c.Lookup(context.Background(), fakeToken, "org/web", 1)
		if err != nil || got.State != cs.state || got.Review != cs.review || got.Checks != cs.checks {
			t.Errorf("%s: %+v, %v", cs.name, got, err)
		}
	}
}

func TestLookupIssue(t *testing.T) {
	c, _ := newClient(t, graphQL(t, func(string, string, int) string {
		return `{"data":{"repository":{"nameWithOwner":"org/api","issueOrPullRequest":{"__typename":"Issue","number":12,"title":"落ちる","state":"CLOSED"}}}}`
	}))
	got, err := c.Lookup(context.Background(), fakeToken, "org/api", 12)
	if err != nil || got.Kind != itemlink.Issue || got.State != itemlink.Closed || got.Review != itemlink.NoReview || got.Checks != itemlink.NoChecks {
		t.Errorf("Lookup = %+v, %v", got, err)
	}
}

func TestLookupFailures(t *testing.T) {
	cases := []struct {
		name, body string
		kind       ghcall.Kind
		number     int
	}{
		{"repository not found", `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`, ghcall.RepoUnreadable, 0},
		{"forbidden", `{"data":{"repository":null},"errors":[{"type":"FORBIDDEN","path":["repository"]}]}`, ghcall.RepoUnreadable, 0},
		{"number not found", `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":null}},"errors":[{"type":"NOT_FOUND","path":["repository","issueOrPullRequest"]}]}`, ghcall.NotFound, 7},
		{"number null without errors", `{"data":{"repository":{"nameWithOwner":"org/web","issueOrPullRequest":null}}}`, ghcall.NotFound, 7},
		{"other error", `{"errors":[{"type":"INTERNAL","message":"x"}]}`, ghcall.Unreachable, 0},
	}
	for _, cs := range cases {
		c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/org/web" { // 改名の確かめ: 改名されていない
				_, _ = w.Write([]byte(`{"full_name":"org/web"}`))
				return
			}
			_, _ = w.Write([]byte(cs.body))
		})
		_, err := c.Lookup(context.Background(), fakeToken, "org/web", 7)
		var f *ghcall.Failure
		if !ghcall.Is(err, cs.kind) || !asFailure(err, &f) || (cs.kind != ghcall.Unreachable && f.Repo != "org/web") || f.Number != cs.number {
			t.Errorf("%s: %v", cs.name, err)
		}
	}
}

func TestLookupRateLimited(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1793000000")
		_, _ = w.Write([]byte(`{"errors":[{"type":"RATE_LIMITED","message":"x"}]}`))
	})
	_, err := c.Lookup(context.Background(), fakeToken, "org/web", 7)
	var f *ghcall.Failure
	if !asFailure(err, &f) || f.Kind != ghcall.RateLimited || f.ResetAt.Unix() != 1793000000 {
		t.Errorf("err = %v", err)
	}
}

func TestLookupFollowsRename(t *testing.T) {
	asked := []string{}
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/old/web":
			http.Redirect(w, r, "/repositories/99", http.StatusMovedPermanently)
		case "/repositories/99":
			_, _ = w.Write([]byte(`{"full_name":"new-org/web2"}`))
		case "/graphql":
			graphQL(t, func(owner, name string, _ int) string {
				asked = append(asked, owner+"/"+name)
				if owner == "old" {
					return `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`
				}
				return strings.Replace(prJSON, `"org/web"`, `"new-org/web2"`, 1)
			})(w, r)
		}
	})
	got, err := c.Lookup(context.Background(), fakeToken, "old/web", 41)
	if err != nil || got.Repo != "new-org/web2" || strings.Join(asked, ",") != "old/web,new-org/web2" {
		t.Errorf("Lookup = %+v, %v, asked %v", got, err, asked)
	}
}

func asFailure(err error, f **ghcall.Failure) bool { return errors.As(err, f) }
```

(import に `errors` を足す。)

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/github/ -run TestLookup`
Expected: FAIL(`Lookup` が未定義)

- [ ] **Step 3: 実装を書く**

`client.go` の `get` を次の 2 つにする(`get` の呼び出し元はそのまま動く):

```go
// get は、path を GET して、成功なら本文を out に読む。失敗は *ghcall.Failure。
func (c *Client) get(ctx context.Context, token, path string, out any) error {
	_, err := c.do(ctx, http.MethodGet, token, path, nil, out)
	return err
}

// do は、問い合わせを 1 回行う。成功なら本文を out に読み、応答の見出しを返す(GraphQL の利用上限の時刻に使う)。
// GET のリダイレクトは net/http が同じホストの中で追う(改名・移管の追従。D3-4)。
func (c *Client) do(ctx context.Context, method, token, path string, body io.Reader, out any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		// net/http のエラーは URL を含むことがあるので、種類だけを返す。
		return nil, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	defer func() { _ = res.Body.Close() }()
	c.observeSkew(res.Header.Get("Date"))
	if f := c.classify(res); f != nil {
		return res.Header, f
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
		return res.Header, &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return res.Header, nil
}
```

`lookup.go`:

```go
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// lookupQuery は、1 つの番号の PR か Issue を読む GraphQL(D3-4)。レビューは reviewDecision と各レビュアーの
// 最新の意見から、チェックは最後のコミットの statusCheckRollup から決める(前回の実装と同じ)。
const lookupQuery = `query Lookup($owner: String!, $name: String!, $number: Int!) {
repository(owner: $owner, name: $name) { nameWithOwner issueOrPullRequest(number: $number) {
__typename
... on Issue { number title state }
... on PullRequest { number title state isDraft reviewDecision
reviewRequests(first: 1) { totalCount }
latestOpinionatedReviews(first: 100) { nodes { state } }
commits(last: 1) { nodes { commit { statusCheckRollup { state } } } } }
} } }`

type lookupAnswer struct {
	Data struct {
		Repository *struct {
			NameWithOwner      string `json:"nameWithOwner"`
			IssueOrPullRequest *node  `json:"issueOrPullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Type string   `json:"type"`
		Path []string `json:"path"`
	} `json:"errors"`
}

type node struct {
	Typename       string  `json:"__typename"`
	Number         int     `json:"number"`
	Title          string  `json:"title"`
	State          string  `json:"state"`
	IsDraft        bool    `json:"isDraft"`
	ReviewDecision *string `json:"reviewDecision"`
	ReviewRequests struct {
		TotalCount int `json:"totalCount"`
	} `json:"reviewRequests"`
	LatestOpinionatedReviews struct {
		Nodes []struct {
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

// Lookup は port.GitHub.Lookup。リポジトリが見つからなければ、REST で改名・移管を確かめ、名前が変わっていれば
// 新しい名前で 1 回だけ問い合わせ直す。
func (c *Client) Lookup(ctx context.Context, token, repo string, number int) (port.Found, error) {
	f, err := c.lookupOnce(ctx, token, repo, number)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		return f, err
	}
	moved, ok := c.renamed(ctx, token, repo)
	if !ok {
		return f, err
	}
	return c.lookupOnce(ctx, token, moved, number)
}

func (c *Client) lookupOnce(ctx context.Context, token, repo string, number int) (port.Found, error) {
	owner, name, _ := strings.Cut(repo, "/")
	body, _ := json.Marshal(map[string]any{
		"query": lookupQuery, "variables": map[string]any{"owner": owner, "name": name, "number": number},
	})
	var a lookupAnswer
	h, err := c.do(ctx, "POST", token, "/graphql", bytes.NewReader(body), &a)
	if err != nil {
		return port.Found{}, withRepo(err, repo, 0)
	}
	if f := answerFailure(a, repo, number, c.resetAt(h)); f != nil {
		return port.Found{}, f
	}
	n := a.Data.Repository.IssueOrPullRequest
	return found(a.Data.Repository.NameWithOwner, n), nil
}

// withRepo は、Forbidden・NotFound(HTTP の 403・404)をリポジトリを読めない失敗にする。
func withRepo(err error, repo string, number int) error {
	if ghcall.Is(err, ghcall.Forbidden) || ghcall.Is(err, ghcall.NotFound) {
		return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo, Number: number}
	}
	return err
}

// answerFailure は、GraphQL の答えの失敗(D3-5)。失敗でなければ nil。
func answerFailure(a lookupAnswer, repo string, number int, reset time.Time) *ghcall.Failure {
	for _, e := range a.Errors {
		switch {
		case e.Type == "RATE_LIMITED":
			return &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: reset}
		case (e.Type == "NOT_FOUND" || e.Type == "FORBIDDEN") && len(e.Path) == 1:
			return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
		case e.Type == "NOT_FOUND" && len(e.Path) == 2:
			return &ghcall.Failure{Kind: ghcall.NotFound, Repo: repo, Number: number}
		}
	}
	switch {
	case a.Data.Repository == nil && len(a.Errors) == 0:
		return &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
	case a.Data.Repository == nil:
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	case a.Data.Repository.IssueOrPullRequest == nil:
		return &ghcall.Failure{Kind: ghcall.NotFound, Repo: repo, Number: number}
	}
	return nil
}

// renamed は、repo が改名・移管されていれば、いまの名前を返す(REST はリダイレクトで移動先を返す)。
func (c *Client) renamed(ctx context.Context, token, repo string) (string, bool) {
	var r struct {
		FullName string `json:"full_name"`
	}
	if err := c.get(ctx, token, "/repos/"+repo, &r); err != nil || r.FullName == "" || strings.EqualFold(r.FullName, repo) {
		return "", false
	}
	return r.FullName, true
}

func found(repo string, n *node) port.Found {
	f := port.Found{Repo: repo, Number: n.Number}
	f.Title = n.Title
	if n.Typename == "Issue" {
		f.Kind, f.Review = itemlink.Issue, itemlink.NoReview
		f.State = itemlink.Open
		if n.State == "CLOSED" {
			f.State = itemlink.Closed
		}
		return f
	}
	f.Kind = itemlink.PR
	switch {
	case n.State == "MERGED":
		f.State = itemlink.Merged
	case n.State == "CLOSED":
		f.State = itemlink.Closed
	case n.IsDraft:
		f.State = itemlink.Draft
	default:
		f.State = itemlink.Open
	}
	f.Review, f.Checks = n.review(), n.checks()
	return f
}

// review は 07「表示と更新」のレビューの状態の決め方。
func (n *node) review() itemlink.Review {
	if d := n.ReviewDecision; d != nil {
		switch *d {
		case "APPROVED":
			return itemlink.Approved
		case "CHANGES_REQUESTED":
			return itemlink.ChangesRequested
		}
	}
	approved := false
	for _, r := range n.LatestOpinionatedReviews.Nodes {
		switch r.State {
		case "CHANGES_REQUESTED":
			return itemlink.ChangesRequested
		case "APPROVED":
			approved = true
		}
	}
	switch {
	case approved:
		return itemlink.Approved
	case n.ReviewRequests.TotalCount > 0:
		return itemlink.Pending
	}
	return itemlink.NoReview
}

// checks は D3-6 のチェックの結果。
func (n *node) checks() itemlink.Checks {
	if len(n.Commits.Nodes) == 0 || n.Commits.Nodes[0].Commit.StatusCheckRollup == nil {
		return itemlink.NoChecks
	}
	switch n.Commits.Nodes[0].Commit.StatusCheckRollup.State {
	case "SUCCESS":
		return itemlink.Success
	case "FAILURE", "ERROR":
		return itemlink.Failure
	}
	return itemlink.Running
}
```

`port/github.go` の `GitHub` の interface に、`Viewer` の次へ足す:

```go
	// Lookup は、repo(owner/name)の番号 number の PR か Issue を読む(07「追加」「表示と更新」)。
	// 改名・移管されたリポジトリは移動先で読み、Found.Repo に移動先の名前を入れる。
	// リポジトリを読めなければ RepoUnreadable、番号がなければ NotFound(どちらも Repo・Number 付き)。
	Lookup(ctx context.Context, token, repo string, number int) (Found, error)
```

`answerFailure` の引数 `reset` のために import に `time` を足す。`c.resetAt(h)` は `h` が nil でも `h.Get` が空を返すので動く。GraphQL は利用上限でも 200 で返すことがあるので、`do` が成功を返した答えの `RATE_LIMITED` も見る。HTTP の 403・429 の利用上限は `classify` が先に `RateLimited` にする。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/github/... && make check`
Expected: PASS(Task 5 の口を満たし、`cmd/hedgehug` もビルドできる)。`lookup.go` が 300 行・関数が 60 行・複雑度 15 を超えたら、`node` の型と `review`・`checks` を `lookup_node.go` に分ける。

- [ ] **Step 5: コミット**

```bash
git add internal/infra/github internal/app/port/github.go
git commit -m "feat(github): PR・Issue を GraphQL で読み、改名・移管を REST のリダイレクトで追う(D3 Task 6)"
```

---

### Task 7: state.db のチェックの結果

**Files:**
- Create: `internal/infra/statedb/checks.go`
- Test: `internal/infra/statedb/checks_test.go`

**Interfaces:**
- Consumes: `port.Checks`・`apptest.RunChecksContract`(Task 5)
- Produces: `func (d *DB) Checks() port.Checks`

- [ ] **Step 1: 失敗するテストを書く**

```go
package statedb

import (
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestChecksContract(t *testing.T) {
	apptest.RunChecksContract(t, func(t *testing.T) port.Checks { return openTemp(t).Checks() })
}

func TestChecksSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Checks().Save(t.Context(), "S1", "pr:org/web#1", itemlink.Success); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if got, _ := d.Checks().Read(t.Context(), "S1"); got["pr:org/web#1"] != itemlink.Success {
		t.Errorf("after reopen: %v", got)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/statedb/ -run TestChecks`
Expected: FAIL(`Checks` が未定義)

- [ ] **Step 3: 実装を書く**

```go
package statedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func init() {
	register("github/001", `CREATE TABLE github_checks (
		store  TEXT NOT NULL,
		link   TEXT NOT NULL,
		checks TEXT NOT NULL,
		at     TEXT NOT NULL,
		PRIMARY KEY (store, link)
	)`)
}

// Checks は、このマシンが読んだ PR のチェックの結果(port.Checks。D3-7)。
func (d *DB) Checks() port.Checks { return checks{d.db} }

type checks struct{ db *sql.DB }

func (c checks) Read(ctx context.Context, store port.StoreID) (map[string]itemlink.Checks, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT link, checks FROM github_checks WHERE store = ?`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]itemlink.Checks{}
	for rows.Next() {
		var link, v string
		if err := rows.Scan(&link, &v); err != nil {
			return nil, err
		}
		out[link] = itemlink.Checks(v)
	}
	return out, rows.Err()
}

func (c checks) Save(ctx context.Context, store port.StoreID, key string, v itemlink.Checks) (bool, error) {
	var old string
	err := c.db.QueryRowContext(ctx, `SELECT checks FROM github_checks WHERE store = ? AND link = ?`, string(store), key).Scan(&old)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		old = ""
	case err != nil:
		return false, err
	}
	if old == string(v) {
		return false, nil
	}
	if v == itemlink.NoChecks {
		_, err = c.db.ExecContext(ctx, `DELETE FROM github_checks WHERE store = ? AND link = ?`, string(store), key)
		return err == nil, err
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO github_checks (store, link, checks, at) VALUES (?, ?, ?, ?)
		ON CONFLICT (store, link) DO UPDATE SET checks = excluded.checks, at = excluded.at`,
		string(store), key, string(v), timeText(time.Now()))
	return err == nil, err
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/statedb/... && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/statedb
git commit -m "feat(statedb): PR のチェックの結果を state.db に置く(D3 Task 7)"
```

---
### Task 8: usecase の一覧・追加・解除

**Files:**
- Create: `internal/app/github/links.go`、`link_list.go`、`link_add.go`、`link_remove.go`
- Test: `internal/app/github/links_test.go`

**Interfaces:**
- Consumes: Task 1・2 の itemlink、Task 5 の `port.Found`・`port.Checks`・`port.StoreGitHubInfo.ReadOnly`、`Connector`(`connector.go`)、`project.NormalizeRepo`・`project.IndexRepo`、`refusal.New`、`ghcall.Refuse`
- Produces:
  ```go
  type Links struct {
  	Stores    port.Stores
  	Identity  port.Identity
  	Connector Connector // GitHub・Accounts・Stores(StoreGitHub)・Clock もここから使う
  	Checks    port.Checks
  	Events    port.Events
  }
  type LinkRow struct {
  	itemlink.Link
  	Checks itemlink.Checks
  	Error  string // 取り直しの失敗(「状態を更新できませんでした: …」)。なければ空
  }
  type AddLinkInput struct {
  	Kind         itemlink.Kind
  	Repo         string
  	Number       string // 画面の入力そのまま(「番号を入力してください」の判定はサーバー。D3-17)
  	AddToProject bool
  }
  func (l Links) List(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error)
  func (l Links) Add(ctx context.Context, store port.StoreID, id item.ID, in AddLinkInput) error
  func (l Links) Remove(ctx context.Context, store port.StoreID, id item.ID, ref itemlink.Ref) error
  ```
  アイテムがなければ `port.ErrNotFound`。断りは `refusal`(GitHub の失敗は `ghcall.Refuse` で 07 の文言)。

- [ ] **Step 1: 失敗するテストを書く**

`links_test.go`(テストのヘルパーは Task 9 も使う):

```go
package github_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const (
	linkStore  port.StoreID = "S1"
	otherStore port.StoreID = "S2"
	linkTask                = apptest.SampleTask // HH-2
	linkEpic                = apptest.SampleEpic // HH-1
)

// linkRecorder は、Write に渡した操作を覚える。
type linkRecorder struct {
	*apptest.Stores
	ops []port.Operation
}

func (r *linkRecorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.ops = append(r.ops, op)
	return r.Stores.Write(ctx, id, op, fn)
}

type linkFixture struct {
	links    ghapp.Links
	stores   *linkRecorder
	gh       *apptest.GitHub
	accounts *apptest.Accounts
	infos    *apptest.StoreGitHub
	checks   *apptest.Checks
	events   *apptest.Events
}

var (
	pr41  = port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 41, Values: itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}, Checks: itemlink.Running}
	iss12 = port.Found{Kind: itemlink.Issue, Repo: "org/api", Number: 12, Values: itemlink.Values{Title: "落ちる", State: itemlink.Open, Review: itemlink.NoReview}}
)

// newLinks は、Store S1・S2 にサンプルを入れ、S1 と S2 にアカウント「work」(@taro、環境変数)を割り当てた組み立て。
func newLinks(t *testing.T) *linkFixture {
	t.Helper()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, apptest.NewIDs())
	for _, id := range []port.StoreID{linkStore, otherStore} {
		s.AddStore(id)
		if err := apptest.Sample(t.Context(), s, id); err != nil {
			t.Fatal(err)
		}
	}
	accounts := apptest.NewAccounts()
	work := account.Account{ID: "A1", Name: "work", Login: "taro", Method: account.EnvVar}
	if err := accounts.Add(t.Context(), work); err != nil {
		t.Fatal(err)
	}
	gh := &apptest.GitHub{Users: map[string]string{"tok": "taro"}}
	gh.Put("org/web", pr41)
	gh.Put("org/api", iss12)
	infos := &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{
		{ID: linkStore, Name: "main", Account: "A1"}, {ID: otherStore, Name: "other", Account: "A1"},
	}}
	f := &linkFixture{stores: &linkRecorder{Stores: s}, gh: gh, accounts: accounts, infos: infos, checks: apptest.NewChecks(), events: &apptest.Events{}}
	f.links = ghapp.Links{
		Stores: f.stores, Identity: apptest.Identity{Login: "hanako"},
		Connector: ghapp.Connector{
			Session: ghapp.NewSession(), Accounts: accounts, Secrets: &apptest.Secrets{Tokens: map[string]string{"taro": "tok"}},
			GitHub: gh, Stores: infos, Clock: clock,
		},
		Checks: f.checks, Events: f.events,
	}
	return f
}

func (f *linkFixture) add(t *testing.T, store port.StoreID, id item.ID, kind itemlink.Kind, repo, number string, toProject bool) error {
	t.Helper()
	return f.links.Add(t.Context(), store, id, ghapp.AddLinkInput{Kind: kind, Repo: repo, Number: number, AddToProject: toProject})
}

func (f *linkFixture) list(t *testing.T, store port.StoreID, id item.ID) []ghapp.LinkRow {
	t.Helper()
	rows, err := f.links.List(t.Context(), store, id)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func (f *linkFixture) lastSummary() string {
	if len(f.stores.ops) == 0 {
		return ""
	}
	return f.stores.ops[len(f.stores.ops)-1].Summary
}

// refusalText は、err が断りならその文言、そうでなければ空。
func refusalText(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func displays(rows []ghapp.LinkRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = string(r.Kind) + " " + r.Display() + " " + r.Title + " " + string(r.State)
	}
	return out
}

func TestAddLinkSavesValuesAndChecks(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, " org/web ", "41", false); err != nil {
		t.Fatal(err)
	}
	if got := displays(f.list(t, linkStore, linkTask)); !slices.Equal(got, []string{"pr org/web#41 Add login form open"}) {
		t.Errorf("rows = %q", got)
	}
	if f.lastSummary() != "HH-2 に org/web#41 をリンク" {
		t.Errorf("summary = %q", f.lastSummary())
	}
	if op := f.stores.ops[len(f.stores.ops)-1]; op.Actor != change.User("hanako") {
		t.Errorf("actor = %+v", op.Actor)
	}
	if rows := f.list(t, linkStore, linkTask); rows[0].Checks != itemlink.Running || rows[0].Review != itemlink.Pending {
		t.Errorf("row = %+v", rows[0])
	}
}

func TestAddLinkRefusals(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Fatal(err)
	}
	f.gh.Put("org/api", port.Found{Kind: itemlink.Issue, Repo: "org/api", Number: 13, Values: itemlink.Values{Title: "x", State: itemlink.Open, Review: itemlink.NoReview}})
	cases := []struct {
		name         string
		id           item.ID
		kind         itemlink.Kind
		repo, number string
		want         string
	}{
		{"repo form", linkEpic, itemlink.PR, "org/*", "1", "リポジトリは owner/name の形式で入力してください"},
		{"two slashes", linkEpic, itemlink.PR, "a/b/c", "1", "リポジトリは owner/name の形式で入力してください"},
		{"number", linkEpic, itemlink.PR, "org/web", "0", "番号を入力してください"},
		{"not a number", linkEpic, itemlink.PR, "org/web", "x", "番号を入力してください"},
		{"already (case, kind ignored)", linkTask, itemlink.PR, "ORG/API", "12", "もうリンクしています"},
		{"not found", linkEpic, itemlink.PR, "org/web", "99", "org/web#99 が見つかりません"},
		{"PR but issue", linkEpic, itemlink.PR, "org/api", "12", "org/api#12 は PR ではなく Issue です"},
		{"issue but PR", linkEpic, itemlink.Issue, "org/web", "41", "org/web#41 は Issue ではなく PR です"},
		{"issue taken", linkEpic, itemlink.Issue, "org/api", "12", "org/api#12 は HH-2 にリンクされています"},
		{"item has issue", linkTask, itemlink.Issue, "org/api", "13", "このアイテムには Issue がリンクされています"},
	}
	for _, c := range cases {
		before := len(f.stores.ops)
		err := f.add(t, linkStore, c.id, c.kind, c.repo, c.number, false)
		if got := refusalText(err); got != c.want {
			t.Errorf("%s: %q (%v), want %q", c.name, got, err, c.want)
		}
		if len(f.stores.ops) != before {
			t.Errorf("%s: wrote", c.name)
		}
	}
}

func TestAddIssueTakenInAnotherStore(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, otherStore, linkEpic, itemlink.Issue, "org/api", "12", false); err != nil {
		t.Fatal(err)
	}
	err := f.add(t, linkStore, linkTask, itemlink.Issue, "org/api", "12", false)
	if got := refusalText(err); got != "org/api#12 は HH-1 にリンクされています" {
		t.Errorf("%q (%v)", got, err)
	}
}

func TestAddLinkGitHubFailures(t *testing.T) {
	f := newLinks(t)
	f.infos.Stores[0].Account = ""
	if got := refusalText(f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false)); got != "org/web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください" {
		t.Errorf("no account: %q", got)
	}
	f.infos.Stores[0].Account = "A1"
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.Unreachable})
	if got := refusalText(f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false)); got != "GitHub に接続できません。接続を確かめて、もう一度お試しください" {
		t.Errorf("unreachable: %q", got)
	}
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.TokenRejected})
	if got := refusalText(f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false)); got != "work(@taro)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください" {
		t.Errorf("rejected: %q", got)
	}
}

func TestAddLinkToProject(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", true); err != nil {
		t.Fatal(err)
	}
	if f.lastSummary() != "HH-2 に org/web#41 をリンク(プロジェクトのリポジトリにも追加)" {
		t.Errorf("summary = %q", f.lastSummary())
	}
	var p project.Project
	_ = f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		p, err = r.Projects().Get(apptest.SampleProject)
		return err
	})
	if !slices.Equal(p.Repos, []string{"org/web"}) {
		t.Errorf("repos = %v", p.Repos)
	}
	// プロジェクトにもうあるリポジトリは足さない
	f.gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 42, Values: itemlink.Values{Title: "b", State: itemlink.Open, Review: itemlink.NoReview}})
	if err := f.add(t, linkStore, linkEpic, itemlink.PR, "ORG/web", "42", true); err != nil {
		t.Fatal(err)
	}
	if f.lastSummary() != "HH-1 に org/web#42 をリンク" {
		t.Errorf("summary = %q", f.lastSummary())
	}
}

func TestRemoveLink(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	ref := itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 41}
	if err := f.links.Remove(t.Context(), linkStore, linkTask, ref); err != nil {
		t.Fatal(err)
	}
	if rows := f.list(t, linkStore, linkTask); len(rows) != 0 || f.lastSummary() != "HH-2 のリンクを解除" {
		t.Errorf("rows = %v, summary %q", rows, f.lastSummary())
	}
	// 解除の記録に入り、手でリンクし直すと消える(07「解除」)
	var log []change.Record
	_ = f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: change.ID(linkTask)})
		return err
	})
	if off := itemlink.FromLog(log).Off; len(off) != 1 || off[0] != ref {
		t.Errorf("off = %v", off)
	}
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	_ = f.stores.Read(t.Context(), linkStore, func(r port.Reader) error {
		log, _ = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: change.ID(linkTask)})
		return nil
	})
	if off := itemlink.FromLog(log).Off; len(off) != 0 {
		t.Errorf("off after relink = %v", off)
	}
	// ないリンクの解除は何も保存しない(行のない Write は何も残さない。port.Stores の約束)
	if err := f.links.Remove(t.Context(), linkStore, linkTask, itemlink.Ref{Kind: itemlink.PR, Repo: "org/web", Number: 7}); err != nil {
		t.Fatal(err)
	}
	if rows := f.list(t, linkStore, linkTask); len(rows) != 1 {
		t.Errorf("rows = %v", rows)
	}
}

func TestLinksOfMissingItem(t *testing.T) {
	f := newLinks(t)
	if _, err := f.links.List(t.Context(), linkStore, "01TESTIT0000000000000000NO"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List = %v", err)
	}
	if err := f.add(t, linkStore, "01TESTIT0000000000000000NO", itemlink.PR, "org/web", "41", false); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Add = %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/github/ -run 'Link'`
Expected: FAIL(`ghapp.Links` が未定義)

- [ ] **Step 3: 実装を書く**

`links.go`:

```go
package github

import (
	"context"
	"errors"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Links は、アイテムの PR・Issue のリンクの操作(07「PR と Issue のリンク」)が使う口。1 ファイル 1 操作。
// GitHub への問い合わせは、その Store に割り当てたアカウントで行う(Connector.TokenFor)。
type Links struct {
	Stores    port.Stores
	Identity  port.Identity
	Connector Connector
	Checks    port.Checks
	Events    port.Events
}

// LinkRow は、リンクの一覧の 1 行。
type LinkRow struct {
	itemlink.Link
	Checks itemlink.Checks
	Error  string
}

// itemView は、リンクの操作が読むアイテムまわりの値。
type itemView struct {
	item    item.Item
	display string // 画面の ID(HH-2)
	links   itemlink.Set
	project project.Project
}

func readItem(r port.Reader, id item.ID) (itemView, error) {
	it, err := r.Items().Get(id)
	if err != nil {
		return itemView{}, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: change.ID(id)})
	if err != nil {
		return itemView{}, err
	}
	ws, err := r.Workspaces().Get(it.Place.Workspace)
	if err != nil {
		return itemView{}, err
	}
	p, err := r.Projects().Get(it.Place.Project)
	if err != nil {
		return itemView{}, err
	}
	return itemView{item: it, display: it.DisplayID(ws.Prefix), links: itemlink.FromLog(log), project: p}, nil
}

func (l Links) view(ctx context.Context, store port.StoreID, id item.ID) (itemView, error) {
	var v itemView
	err := l.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		v, err = readItem(r, id)
		return err
	})
	return v, err
}

// lookup は、Store のアカウントで repo の number を読む。トークンが拒否されたら認証エラーにする(07「状態」)。
func (l Links) lookup(ctx context.Context, store port.StoreID, repo string, number int) (port.Found, error) {
	tok, a, err := l.Connector.TokenFor(ctx, store, repo)
	if err != nil {
		return port.Found{}, err
	}
	f, err := l.Connector.GitHub.Lookup(ctx, tok, repo, number)
	if ghcall.Is(err, ghcall.TokenRejected) {
		l.Connector.Rejected(a.ID)
		return port.Found{}, authError(a)
	}
	return f, err
}

func (l Links) now() time.Time { return l.Connector.Clock.Now() }

// saveChecks は、このマシンが読んだチェックの結果を置く。変わったら true(D3-7)。
func (l Links) saveChecks(ctx context.Context, store port.StoreID, r itemlink.Ref, c itemlink.Checks) (bool, error) {
	return l.Checks.Save(ctx, store, r.Key(), c)
}

// issueOwner は、ref の Issue にリンクしているアイテム(self 以外)を、このマシンのすべての Store から探し、
// その画面の ID を返す(07「追加」。D3-15)。
func (l Links) issueOwner(ctx context.Context, ref itemlink.Ref, self item.ID) (string, bool, error) {
	infos, err := l.Connector.Stores.List(ctx)
	if err != nil {
		return "", false, err
	}
	for _, s := range infos {
		var id string
		var found bool
		err := l.Stores.Read(ctx, s.ID, func(r port.Reader) error {
			var err error
			id, found, err = findIssue(r, ref, self)
			return err
		})
		if errors.Is(err, port.ErrStoreNotFound) {
			continue
		}
		if err != nil || found {
			return id, found, err
		}
	}
	return "", false, nil
}

// findIssue は、r の Store の、self 以外のアイテムで、ref の Issue にリンクしているものを探す。
func findIssue(r port.Reader, ref itemlink.Ref, self item.ID) (string, bool, error) {
	wss, err := r.Workspaces().List()
	if err != nil {
		return "", false, err
	}
	for _, w := range wss {
		ps, err := r.Projects().List(w.ID)
		if err != nil {
			return "", false, err
		}
		for _, p := range ps {
			its, err := r.Items().List(p.ID)
			if err != nil {
				return "", false, err
			}
			for _, it := range its {
				if it.ID == self {
					continue
				}
				log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: change.ID(it.ID)})
				if err != nil {
					return "", false, err
				}
				if l, ok := itemlink.FromLog(log).Find(ref); ok && l.Kind == itemlink.Issue {
					return it.DisplayID(w.Prefix), true, nil
				}
			}
		}
	}
	return "", false, nil
}

// refused は、GitHub の失敗を 07 の文言の断りにする。
func (l Links) refused(err error) error { return ghcall.Refuse(err, l.now()) }

// failText は、取り直しの失敗の行の文言(07「表示と更新」)。
func (l Links) failText(err error) string {
	var r *refusal.Refusal
	if !errors.As(l.refused(err), &r) {
		errors.As(l.refused(&ghcall.Failure{Kind: ghcall.Unreachable}), &r)
	}
	return "状態を更新できませんでした: " + r.Message
}

func rowsOf(ls []itemlink.Link, checks map[string]itemlink.Checks, errs map[string]string) []LinkRow {
	out := make([]LinkRow, len(ls))
	for i, l := range ls {
		out[i] = LinkRow{Link: l, Checks: checks[l.Key()], Error: errs[l.Key()]}
	}
	return out
}
```

`findIssue` の入れ子が gocognit 15 を超えたら、プロジェクトの中を見る部分を `findIssueIn(r, p, prefix, ref, self)` に分ける。

`link_list.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// List は、アイテムのリンクを、リンクした順に、このマシンが読んだチェックの結果とともに返す(07「表示と更新」)。
// GitHub には問い合わせない。アイテムがなければ port.ErrNotFound。
func (l Links) List(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error) {
	v, err := l.view(ctx, store, id)
	if err != nil {
		return nil, err
	}
	checks, err := l.Checks.Read(ctx, store)
	if err != nil {
		return nil, err
	}
	return rowsOf(v.links.Links, checks, nil), nil
}
```

`link_add.go`:

```go
package github

import (
	"context"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// AddLinkInput は「PR をリンク」「Issue をリンク」のダイアログの入力。
type AddLinkInput struct {
	Kind         itemlink.Kind
	Repo         string
	Number       string
	AddToProject bool
}

// addPlan は、確かめ終えて保存するもの。
type addPlan struct {
	ref       itemlink.Ref
	values    itemlink.Values
	toProject bool
	summary   string
}

// Add は、PR か Issue を GitHub で確かめてリンクする(07「追加」)。確かめる順は 07 の表のとおり。
// ダイアログは閉じ、トーストは出さない(画面)。手でのリンクでは自動化のきっかけを起こさない(D3-13)。
func (l Links) Add(ctx context.Context, store port.StoreID, id item.ID, in AddLinkInput) error {
	if in.Kind != itemlink.PR && in.Kind != itemlink.Issue {
		return port.ErrNotFound
	}
	plan, err := l.plan(ctx, store, id, in)
	if err != nil {
		return err
	}
	me, err := l.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: plan.summary}
	return l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		if err := checkItem(tx, cur, plan.ref, id); err != nil {
			return err
		}
		if err := tx.WriteItems().Append(id, cur.links.Attach(plan.ref, plan.values)...); err != nil {
			return err
		}
		if plan.toProject && project.IndexRepo(cur.project.Repos, plan.ref.Repo) < 0 {
			return tx.WriteProjects().Append(cur.project.ID, change.NewAdd("repos", plan.ref.Repo))
		}
		return nil
	})
}

// plan は、入力の形・重なり・GitHub・Issue の 1 対 1 を、07 の表の順に確かめる。
func (l Links) plan(ctx context.Context, store port.StoreID, id item.ID, in AddLinkInput) (addPlan, error) {
	repo, err := project.NormalizeRepo(in.Repo)
	if err != nil {
		return addPlan{}, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(in.Number))
	if err != nil || n <= 0 {
		return addPlan{}, refusal.New(itemlink.MsgNumber)
	}
	v, err := l.view(ctx, store, id)
	if err != nil {
		return addPlan{}, err
	}
	if _, ok := v.links.Find(itemlink.Ref{Kind: in.Kind, Repo: repo, Number: n}); ok {
		return addPlan{}, refusal.New(itemlink.MsgAlready)
	}
	f, err := l.lookup(ctx, store, repo, n)
	if err != nil {
		return addPlan{}, l.refused(err)
	}
	ref := itemlink.Ref{Kind: f.Kind, Repo: f.Repo, Number: f.Number}
	if err := l.checkFound(ctx, v, ref, in.Kind, id); err != nil {
		return addPlan{}, err
	}
	if _, err := l.saveChecks(ctx, store, ref, f.Checks); err != nil {
		return addPlan{}, err
	}
	toProject := in.AddToProject && project.IndexRepo(v.project.Repos, f.Repo) < 0
	return addPlan{ref: ref, values: f.Values, toProject: toProject, summary: itemlink.AddSummary(v.display, ref, toProject)}, nil
}

// checkFound は、GitHub から読んだあとの検査(種別の違い・重なり・Issue の 1 対 1)。
func (l Links) checkFound(ctx context.Context, v itemView, ref itemlink.Ref, want itemlink.Kind, id item.ID) error {
	if ref.Kind != want {
		return refusal.New(itemlink.MsgWrongKind(ref, want))
	}
	if _, ok := v.links.Find(ref); ok {
		return refusal.New(itemlink.MsgAlready)
	}
	if ref.Kind != itemlink.Issue {
		return nil
	}
	owner, taken, err := l.issueOwner(ctx, ref, id)
	if err != nil {
		return err
	}
	if taken {
		return refusal.New(itemlink.MsgIssueTaken(ref, owner))
	}
	if _, ok := v.links.HasIssue(); ok {
		return refusal.New(itemlink.MsgHasIssue)
	}
	return nil
}

// checkItem は、保存の中で、同じ Store のいまの値で重なりと Issue の 1 対 1 を確かめ直す。
func checkItem(tx port.Tx, cur itemView, ref itemlink.Ref, id item.ID) error {
	if _, ok := cur.links.Find(ref); ok {
		return refusal.New(itemlink.MsgAlready)
	}
	if ref.Kind != itemlink.Issue {
		return nil
	}
	owner, taken, err := findIssue(tx, ref, id)
	if err != nil {
		return err
	}
	if taken {
		return refusal.New(itemlink.MsgIssueTaken(ref, owner))
	}
	if _, ok := cur.links.HasIssue(); ok {
		return refusal.New(itemlink.MsgHasIssue)
	}
	return nil
}
```

`link_remove.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// Remove は、リンクを確認なしで外す(07「解除」)。PR は解除の記録に入る。ないリンクなら何も保存しない。
// アイテムがなければ port.ErrNotFound。
func (l Links) Remove(ctx context.Context, store port.StoreID, id item.ID, ref itemlink.Ref) error {
	v, err := l.view(ctx, store, id)
	if err != nil {
		return err
	}
	me, err := l.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: itemlink.RemoveSummary(v.display)}
	return l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		link, ok := cur.links.Find(ref)
		if !ok || link.Kind != ref.Kind {
			return nil
		}
		return tx.WriteItems().Append(id, cur.links.Detach(link)...)
	})
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/github/... && make lint`
Expected: PASS。`authError` は `connector.go` にあるものを使う。

- [ ] **Step 5: コミット**

```bash
git add internal/app/github
git commit -m "feat(app): PR・Issue のリンクの一覧・追加・解除の usecase を足す(D3 Task 8)"
```

---

### Task 9: usecase の取り直しとプロジェクトの一覧

**Files:**
- Create: `internal/app/github/link_refresh.go`、`internal/app/github/link_project.go`
- Test: `internal/app/github/link_refresh_test.go`

**Interfaces:**
- Consumes: Task 8 の `Links`・`LinkRow`・`readItem`・`lookup`・`issueOwner`・`rowsOf`・`failText`、Task 2 の `Set.Refresh`・`Set.Rename`・`RefreshSummary`
- Produces:
  ```go
  func (l Links) Refresh(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error)
  type ProjectLinkRow struct {
  	LinkRow
  	Item    item.ID
  	Display string // アイテムの画面の ID
  }
  func (l Links) ProjectLinks(ctx context.Context, store port.StoreID, id project.ID) ([]ProjectLinkRow, error)
  ```

- [ ] **Step 1: 失敗するテストを書く**

`link_refresh_test.go`(Task 8 のヘルパーを使う):

```go
package github_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func (f *linkFixture) refresh(t *testing.T) []ghapp.LinkRow {
	t.Helper()
	rows, err := f.links.Refresh(t.Context(), linkStore, linkTask)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestRefreshSavesChangedValuesAsGitHub(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	writes := len(f.stores.ops)
	f.refresh(t) // 変わっていない: 保存しない
	if len(f.stores.ops) != writes {
		t.Fatalf("saved without a change: %+v", f.stores.ops[writes:])
	}
	changed := pr41
	changed.State, changed.Review, changed.Checks = itemlink.Merged, itemlink.Approved, itemlink.Success
	f.gh.Put("org/web", changed)
	rows := f.refresh(t)
	op := f.stores.ops[len(f.stores.ops)-1]
	if op.Actor != (change.Actor{Kind: change.ActorGitHub}) ||
		op.Summary != "HH-2 の org/web#41 の状態を Merged に変更、HH-2 の org/web#41 のレビューを 承認済み に変更" {
		t.Errorf("op = %+v", op)
	}
	if rows[0].State != itemlink.Merged || rows[0].Checks != itemlink.Success || rows[0].Error != "" {
		t.Errorf("row = %+v", rows[0])
	}
	if ev := f.events.List(); len(ev) == 0 || ev[len(ev)-1] != (port.Event{Kind: port.EventGitHub, Store: linkStore}) {
		t.Errorf("events = %v", ev)
	}
}

func TestRefreshFollowsRename(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	moved := pr41
	moved.Repo = "org/web2"
	f.gh.Put("org/web", moved)
	rows := f.refresh(t)
	if got := displays(rows); !slices.Equal(got, []string{"pr org/web2#41 Add login form open"}) || rows[0].Error != "" {
		t.Errorf("rows = %q %+v", got, rows)
	}
	if f.lastSummary() != "HH-2 のリンクを org/web#41 から org/web2#41 に更新" {
		t.Errorf("summary = %q", f.lastSummary())
	}
}

func TestRefreshKeepsOldNameWhenRenameCollides(t *testing.T) {
	f := newLinks(t)
	f.gh.Put("org/web2", port.Found{Kind: itemlink.PR, Repo: "org/web2", Number: 41, Values: itemlink.Values{Title: "x", State: itemlink.Open, Review: itemlink.NoReview}})
	for _, repo := range []string{"org/web", "org/web2"} {
		if err := f.add(t, linkStore, linkTask, itemlink.PR, repo, "41", false); err != nil {
			t.Fatal(err)
		}
	}
	moved := pr41
	moved.Repo, moved.State = "org/web2", itemlink.Closed
	f.gh.Put("org/web", moved)
	rows := f.refresh(t)
	if got := displays(rows); !slices.Contains(got, "pr org/web#41 Add login form closed") {
		t.Errorf("rows = %q (the old name stays, values are refreshed)", got)
	}
}

func TestRefreshFailureShowsOnTheRow(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	f.gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: "org/web"})
	rows := f.refresh(t)
	if rows[0].Error != "状態を更新できませんでした: org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください" || rows[0].State != itemlink.Open {
		t.Errorf("row = %+v", rows[0])
	}
	f.gh.Fail("org/web", 41, nil)
	if rows := f.refresh(t); rows[0].Error != "" {
		t.Errorf("the error stays after a success: %+v", rows[0])
	}
}

func TestRefreshSkips(t *testing.T) {
	f := newLinks(t)
	if err := f.add(t, linkStore, linkTask, itemlink.PR, "org/web", "41", false); err != nil {
		t.Fatal(err)
	}
	asked := f.gh.Lookups
	f.infos.Stores[0].ReadOnly = true
	f.refresh(t)
	if f.gh.Lookups != asked {
		t.Error("a read-only store was refreshed")
	}
	f.infos.Stores[0].ReadOnly = false
	f.infos.Stores[0].Account = ""
	rows := f.refresh(t)
	if f.gh.Lookups != asked || rows[0].Error != "状態を更新できませんでした: org/web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください" {
		t.Errorf("no account assigned: lookups %d, row %+v", f.gh.Lookups-asked, rows[0])
	}
	if err := f.accounts.Remove(t.Context(), "A1"); err != nil {
		t.Fatal(err)
	}
	if rows := f.refresh(t); rows[0].Error != "" || f.gh.Lookups != asked {
		t.Errorf("no accounts on this machine: row %+v", rows[0])
	}
}

func TestProjectLinks(t *testing.T) {
	f := newLinks(t)
	f.gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 42, Values: itemlink.Values{Title: "b", State: itemlink.Draft, Review: itemlink.NoReview}})
	for _, a := range []struct {
		id     item.ID
		kind   itemlink.Kind
		repo   string
		number string
	}{{linkTask, itemlink.PR, "org/web", "41"}, {linkEpic, itemlink.PR, "org/web", "42"}, {linkTask, itemlink.Issue, "org/api", "12"}} {
		if err := f.add(t, linkStore, a.id, a.kind, a.repo, a.number, false); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := f.links.ProjectLinks(t.Context(), linkStore, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Display+" "+r.Link.Display())
	}
	if want := []string{"HH-1 org/web#42", "HH-2 org/web#41", "HH-2 org/api#12"}; !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q (item number, then link order)", got, want)
	}
}
```

import に `ghapp "github.com/kukv/hedgehug/internal/app/github"` と `item` を足す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/github/ -run 'Refresh|ProjectLinks'`
Expected: FAIL(`Refresh`・`ProjectLinks` が未定義)

- [ ] **Step 3: 実装を書く**

`link_refresh.go`:

```go
package github

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// Refresh は、詳細パネルを開いたときに、アイテムのリンクを GitHub から取り直す(07「表示と更新」。D3-8)。
// 値が変わったリンクは、リンクごとに実行者「GitHub」で保存する(D3-10)。失敗した行には理由を添える。
// 閲覧のみの Store と、このマシンにアカウントが 1 つもないときは問い合わせない(D3-9)。
func (l Links) Refresh(ctx context.Context, store port.StoreID, id item.ID) ([]LinkRow, error) {
	skip, err := l.skipRefresh(ctx, store)
	if err != nil || skip {
		if err != nil {
			return nil, err
		}
		return l.List(ctx, store, id)
	}
	v, err := l.view(ctx, store, id)
	if err != nil {
		return nil, err
	}
	errs := map[string]string{}
	checksChanged := false
	for _, link := range v.links.Links {
		f, err := l.lookup(ctx, store, link.Repo, link.Number)
		if err != nil {
			errs[link.Key()] = l.failText(err)
			continue
		}
		ref, err := l.refreshOne(ctx, store, id, v, link, f)
		if err != nil {
			return nil, err
		}
		changed, err := l.saveChecks(ctx, store, ref, f.Checks)
		if err != nil {
			return nil, err
		}
		checksChanged = checksChanged || changed
	}
	if checksChanged {
		l.Events.Publish(port.Event{Kind: port.EventGitHub, Store: store})
	}
	rows, err := l.List(ctx, store, id)
	for i := range rows {
		rows[i].Error = errs[rows[i].Key()]
	}
	return rows, err
}

// skipRefresh は、問い合わせないときか(D3-9)。
func (l Links) skipRefresh(ctx context.Context, store port.StoreID) (bool, error) {
	infos, err := l.Connector.Stores.List(ctx)
	if err != nil {
		return false, err
	}
	for _, s := range infos {
		if s.ID == store && s.ReadOnly {
			return true, nil
		}
	}
	accounts, err := l.Connector.Accounts.List(ctx)
	return len(accounts) == 0, err
}

// refreshOne は、1 本のリンクを f の値にそろえて保存し、そのあとのリンクの Ref を返す。値が同じなら保存しない。
func (l Links) refreshOne(ctx context.Context, store port.StoreID, id item.ID, v itemView, link itemlink.Link, f port.Found) (itemlink.Ref, error) {
	repo, err := l.renameTo(ctx, v, link, f.Repo)
	if err != nil {
		return itemlink.Ref{}, err
	}
	summary := itemlink.RefreshSummary(v.display, link, repo, f.Values)
	after := itemlink.Ref{Kind: link.Kind, Repo: repo, Number: link.Number}
	if summary == "" {
		return after, nil
	}
	op := port.Operation{Actor: change.Actor{Kind: change.ActorGitHub}, Summary: summary}
	err = l.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		cur, err := readItem(tx, id)
		if err != nil {
			return err
		}
		now, ok := cur.links.Find(link.Ref)
		if !ok || now.Repo != link.Repo || now.Kind != link.Kind {
			return nil // 取り直すあいだに外された・書き換えられた
		}
		drafts := cur.links.Refresh(now, f.Values)
		if repo != now.Repo {
			drafts = cur.links.Rename(now, repo, f.Values)
		}
		return tx.WriteItems().Append(id, drafts...)
	})
	return after, err
}

// renameTo は、書き換えるリポジトリ名。改名・移管がない(大文字小文字だけの違いを含む)か、書き換えると
// 同じアイテムの別のリンク・別のアイテムの Issue と重なるときは、いまの名前(D3-11)。
func (l Links) renameTo(ctx context.Context, v itemView, link itemlink.Link, repo string) (string, error) {
	if strings.EqualFold(repo, link.Repo) {
		return link.Repo, nil
	}
	moved := itemlink.Ref{Kind: link.Kind, Repo: repo, Number: link.Number}
	if _, ok := v.links.Find(moved); ok {
		return link.Repo, nil
	}
	if link.Kind == itemlink.Issue {
		_, taken, err := l.issueOwner(ctx, moved, v.item.ID)
		if err != nil || taken {
			return link.Repo, err
		}
	}
	return repo, nil
}
```

`Refresh` の最初の `if` は読みにくいので、実装では次の形にする:

```go
	skip, err := l.skipRefresh(ctx, store)
	if err != nil {
		return nil, err
	}
	if skip {
		return l.List(ctx, store, id)
	}
```

`TestRefreshKeepsOldNameWhenRenameCollides` では、`v.links.Find(moved)` が `org/web2#41`(2 本目のリンク)を見つけるので書き換えない。

`link_project.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// ProjectLinkRow は、プロジェクトの概要の「PR と Issue」の 1 行(02「概要」)。
type ProjectLinkRow struct {
	LinkRow
	Item    item.ID
	Display string
}

// ProjectLinks は、プロジェクトの下のアイテムのリンクを、アイテムの番号順、同じアイテムの中はリンクした順に返す。
// GitHub には問い合わせない。プロジェクトがなければ port.ErrNotFound。
func (l Links) ProjectLinks(ctx context.Context, store port.StoreID, id project.ID) ([]ProjectLinkRow, error) {
	var out []ProjectLinkRow
	err := l.Stores.Read(ctx, store, func(r port.Reader) error {
		p, err := r.Projects().Get(id)
		if err != nil {
			return err
		}
		ws, err := r.Workspaces().Get(p.Place.Workspace)
		if err != nil {
			return err
		}
		its, err := r.Items().List(id)
		if err != nil {
			return err
		}
		for _, it := range its {
			log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: change.ID(it.ID)})
			if err != nil {
				return err
			}
			for _, link := range itemlink.FromLog(log).Links {
				out = append(out, ProjectLinkRow{LinkRow: LinkRow{Link: link}, Item: it.ID, Display: it.DisplayID(ws.Prefix)})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	checks, err := l.Checks.Read(ctx, store)
	for i := range out {
		out[i].Checks = checks[out[i].Key()]
	}
	return out, err
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/github/... && make lint`
Expected: PASS。`app/github` のテストを除くファイル数が 20 以下(12 + 6 = 18)であることを `make lint`(archtest)が確かめる。

- [ ] **Step 5: コミット**

```bash
git add internal/app/github
git commit -m "feat(app): 詳細パネルを開いたときのリンクの取り直しと、プロジェクトのリンクの一覧を足す(D3 Task 9)"
```

---
### Task 10: API・組み立て・e2e の偽物・github-live

**Files:**
- Create: `internal/adapter/httpapi/github/links.go`、`internal/adapter/httpapi/github/links_test.go`、`cmd/hedgehug/links_e2e.go`
- Modify: `internal/adapter/httpapi/github/accounts.go`(`Services` に `Links`、`Register` で `registerLinks`)、`internal/adapter/httpapi/routes.go`(`Deps.Checks`、`githubServices`、hub を先に作る)、`internal/adapter/httpapi/events/events.go`(`event: github`)、`cmd/hedgehug/wire.go`・`wire_e2e.go`・`device_e2e.go`、`internal/infra/github/live_test.go`、`.github/workflows/github-live.yaml`(要れば)
- 生成物: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: Task 8・9 の `ghapp.Links`、Task 7 の `(*statedb.DB).Checks()`、Task 5 の `apptest.Checks`・`apptest.GitHub.Put`
- Produces(OpenAPI。画面の Task 12 が使う):
  - `GET /api/stores/{store}/items/{item}/links` → `LinkRow[]`(operationId `list-links`)
  - `POST /api/stores/{store}/items/{item}/links/refresh` → `LinkRow[]`(`refresh-links`)
  - `POST /api/stores/{store}/items/{item}/links` 本文 `LinkInput` → 204(`add-link`)
  - `POST /api/stores/{store}/items/{item}/links/remove` 本文 `LinkRef` → 204(`remove-link`)
  - `GET /api/stores/{store}/projects/{project}/links` → `ProjectLinkRow[]`(`list-project-links`)
  - `LinkRow = {kind: "pr"|"issue", repo, number, title, state: "open"|"draft"|"merged"|"closed", review: "none"|"pending"|"approved"|"changes_requested", checks: ""|"success"|"failure"|"pending", error}`
  - `ProjectLinkRow = LinkRow & {item, display}`、`LinkInput = {kind, repo, number: string, addToProject: boolean}`、`LinkRef = {kind, repo, number: integer}`
  - SSE: `event: github`、`data: {"store": "..."}`
  - e2e: `POST /e2e/github?repo=…&number=…&kind=pr|issue&title=…&state=…&review=…&checks=…&moved=…&gone=…`(`moved` があれば、その名前を GitHub のいまの名前として返す。`gone` があれば、その番号を「見つかりません」にする)

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/github/links_test.go`(`accounts_test.go` の `newServices` を使う):

```go
package github_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestLinksAPI(t *testing.T) {
	_, api := humatest.New(t)
	ghapi.Register(api, linkServices(t))
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"

	res := api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": false})
	if res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	res = api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "x", "addToProject": false})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "番号を入力してください") {
		t.Errorf("refusal: %d %s", res.Code, res.Body)
	}
	res = api.Get(base)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"repo":"org/web","number":41,"title":"Add login form","state":"open","review":"pending","checks":"pending","error":""`) {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	if res = api.Post(base+"/refresh", map[string]any{}); res.Code != http.StatusOK {
		t.Errorf("refresh: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/links")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"display":"HH-2"`) {
		t.Errorf("project links: %d %s", res.Code, res.Body)
	}
	if res = api.Post(base+"/remove", map[string]any{"kind": "pr", "repo": "org/web", "number": 41}); res.Code != http.StatusNoContent {
		t.Errorf("remove: %d %s", res.Code, res.Body)
	}
	if res = api.Get("/api/stores/S1/items/01TESTIT0000000000000000NO/links"); res.Code != http.StatusNotFound {
		t.Errorf("missing item: %d", res.Code)
	}
}
```

`linkServices` は同じファイルに置く:

```go
// linkServices は、accounts_test.go の newServices に、Store S1(サンプル)と、S1 に割り当てたアカウント A1(@taro)と、
// org/web#41 を持つ偽の GitHub で作った Links を足したもの。
func linkServices(t *testing.T) ghapi.Services {
	t.Helper()
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	gh.Put("org/web", port.Found{Kind: itemlink.PR, Repo: "org/web", Number: 41,
		Values: itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}, Checks: itemlink.Running})
	svc := newServices(gh)
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	accounts := apptest.NewAccounts()
	if err := accounts.Add(t.Context(), account.Account{ID: "A1", Name: "work", Login: "taro", Method: account.EnvVar}); err != nil {
		t.Fatal(err)
	}
	svc.Links = ghapp.Links{
		Stores: s, Identity: apptest.Identity{Login: "taro"},
		Connector: ghapp.Connector{
			Session: ghapp.NewSession(), Accounts: accounts, Secrets: &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}},
			GitHub: gh, Stores: &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Account: "A1"}}}, Clock: clock,
		},
		Checks: apptest.NewChecks(), Events: &apptest.Events{},
	}
	return svc
}
```

`internal/adapter/httpapi/events/events_test.go` に、`port.EventGitHub` を `event: github` で書くテストを足す(既存の `writeFrame` のテストの形に合わせる):

```go
func TestWriteFrameGitHub(t *testing.T) {
	var b strings.Builder
	if err := writeFrame(&b, port.Event{Kind: port.EventGitHub, Store: "S1"}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "event: github\ndata: {\"store\":\"S1\"}\n\n" {
		t.Errorf("%q", b.String())
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: FAIL(`Services.Links` がない、`event: github` が `store` になる)

- [ ] **Step 3: 実装を書く**

`internal/adapter/httpapi/github/links.go`:

```go
package github

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// LinkRow は、PR・Issue のリンクの 1 行。
type LinkRow struct {
	Kind   string `json:"kind" enum:"pr,issue"`
	Repo   string `json:"repo" doc:"owner/name"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state" enum:"open,draft,merged,closed"`
	Review string `json:"review" enum:"none,pending,approved,changes_requested" doc:"Issue は none"`
	Checks string `json:"checks" enum:",success,failure,pending" doc:"このマシンが読んだ PR のチェックの結果。なければ空"`
	Error  string `json:"error" doc:"取り直しの失敗(「状態を更新できませんでした: …」)。なければ空"`
}

// ProjectLinkRow は、プロジェクトの概要の「PR と Issue」の 1 行。
type ProjectLinkRow struct {
	LinkRow
	Item    string `json:"item" doc:"アイテムの内部 ID"`
	Display string `json:"display" doc:"アイテムの画面の ID(HH-2)"`
}

// LinkInput は「PR をリンク」「Issue をリンク」の本文。
type LinkInput struct {
	Kind         string `json:"kind" enum:"pr,issue"`
	Repo         string `json:"repo"`
	Number       string `json:"number" doc:"入力のまま。正の整数でなければ断る"`
	AddToProject bool   `json:"addToProject" doc:"プロジェクトのリポジトリにも追加する"`
}

// LinkRef は、解除するリンク。
type LinkRef struct {
	Kind   string `json:"kind" enum:"pr,issue"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

type linksPath struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type projectLinksPath struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

type addLinkInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  LinkInput
}

type removeLinkInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  LinkRef
}

type linkRowsOutput struct{ Body []LinkRow }

type projectLinkRowsOutput struct{ Body []ProjectLinkRow }

func registerLinks(api huma.API, svc ghapp.Links) {
	const path = "/api/stores/{store}/items/{item}/links"
	huma.Register(api, huma.Operation{OperationID: "list-links", Method: http.MethodGet, Path: path, Summary: "アイテムの PR・Issue のリンクを、リンクした順に返す(GitHub には問い合わせない)"},
		func(ctx context.Context, in *linksPath) (*linkRowsOutput, error) {
			rows, err := svc.List(ctx, port.StoreID(in.Store), item.ID(in.Item))
			return linkRows(rows, err)
		})
	huma.Register(api, huma.Operation{OperationID: "refresh-links", Method: http.MethodPost, Path: path + "/refresh", Summary: "アイテムのリンクを GitHub から取り直し、一覧を返す"},
		func(ctx context.Context, in *linksPath) (*linkRowsOutput, error) {
			rows, err := svc.Refresh(ctx, port.StoreID(in.Store), item.ID(in.Item))
			return linkRows(rows, err)
		})
	huma.Register(api, huma.Operation{OperationID: "add-link", Method: http.MethodPost, Path: path, DefaultStatus: http.StatusNoContent, Summary: "PR か Issue を GitHub で確かめてリンクする"},
		func(ctx context.Context, in *addLinkInput) (*struct{}, error) {
			b := in.Body
			err := svc.Add(ctx, port.StoreID(in.Store), item.ID(in.Item), ghapp.AddLinkInput{
				Kind: itemlink.Kind(b.Kind), Repo: b.Repo, Number: b.Number, AddToProject: b.AddToProject,
			})
			return nil, apierr.From(err)
		})
	huma.Register(api, huma.Operation{OperationID: "remove-link", Method: http.MethodPost, Path: path + "/remove", DefaultStatus: http.StatusNoContent, Summary: "リンクを外す(PR は解除の記録に入る)"},
		func(ctx context.Context, in *removeLinkInput) (*struct{}, error) {
			b := in.Body
			err := svc.Remove(ctx, port.StoreID(in.Store), item.ID(in.Item), itemlink.Ref{Kind: itemlink.Kind(b.Kind), Repo: b.Repo, Number: b.Number})
			return nil, apierr.From(err)
		})
	huma.Register(api, huma.Operation{OperationID: "list-project-links", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/links", Summary: "プロジェクトの下のアイテムのリンクを、アイテムの番号順に返す"},
		func(ctx context.Context, in *projectLinksPath) (*projectLinkRowsOutput, error) {
			rows, err := svc.ProjectLinks(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &projectLinkRowsOutput{Body: []ProjectLinkRow{}}
			for _, r := range rows {
				out.Body = append(out.Body, ProjectLinkRow{LinkRow: linkRow(r.LinkRow), Item: string(r.Item), Display: r.Display})
			}
			return out, nil
		})
}

func linkRows(rows []ghapp.LinkRow, err error) (*linkRowsOutput, error) {
	if err != nil {
		return nil, apierr.From(err)
	}
	out := &linkRowsOutput{Body: []LinkRow{}}
	for _, r := range rows {
		out.Body = append(out.Body, linkRow(r))
	}
	return out, nil
}

func linkRow(r ghapp.LinkRow) LinkRow {
	return LinkRow{
		Kind: string(r.Kind), Repo: r.Repo, Number: r.Number, Title: r.Title, State: string(r.State),
		Review: string(r.Review), Checks: string(r.Checks), Error: r.Error,
	}
}
```

`accounts.go`: `Services` に `Links ghapp.Links` を足し、`Register` の最後に `registerLinks(api, s.Links)` を足す。

`events.go` の `writeFrame`:

```go
func writeFrame(w io.Writer, e port.Event) error {
	kind := "store"
	switch e.Kind {
	case port.EventSync:
		kind = "sync"
	case port.EventGitHub:
		kind = "github"
	}
	...
```

`events.go` の OpenAPI の説明(90 行あたり「`event: store` または `event: sync` の行」)を「`event: store`・`event: sync`・`event: github` のどれかの行」に直す。

`routes.go`:
- `Deps` に `Checks port.Checks // このマシンが読んだ PR のチェックの結果(D3)` を足す。
- `NewAPI` で、hub を `ghapi.Register` より前に作る(`hub := deps.Events; if hub == nil { hub = events.NewHub() }` を `system.Register` の前へ移す)。`githubServices(deps, hub)` に渡す。
- `githubServices`:

```go
func githubServices(d Deps, hub port.Events) ghapi.Services {
	s := d.GitHubSession
	if s == nil {
		s = ghapp.NewSession()
	}
	conn := ghapp.Connector{
		Session: s, Accounts: d.Accounts, Secrets: d.Secrets, GitHub: d.GitHub, Device: d.DeviceFlow,
		Hosts: d.Hosts, Stores: d.StoreGitHub, Clock: d.Clock,
	}
	return ghapi.Services{
		...(今までどおり)
		Links: ghapp.Links{Stores: d.Stores, Identity: d.Identity, Connector: conn, Checks: d.Checks, Events: hub},
	}
}
```

コメント「D3 で Connector を使う API を足すときは…」は消す(足したので)。`*events.Hub` が `port.Events` を満たすことは `syncing.Service` の組み立てで確かめ済み。

`cmd/hedgehug/wire.go`: `httpapi.Deps` に `Checks: db.Checks()` を足す(`db` は 39 行で開いている state.db)。

`cmd/hedgehug/wire_e2e.go`:
- アカウント `e2e-account` を最初から入れる: `accounts := apptest.NewAccounts()` のあとに `_ = accounts.Add(ctx, account.Account{ID: "e2e-account", Name: "e2e", Login: "e2e-bot", Method: account.EnvVar})`。`Secrets` の `e2eTokens` に `"e2e-bot": "e2e-gh-token"` を、`e2eGitHub` の `Users` に `"e2e-gh-token": "e2e-bot"` を足す(`device_e2e.go` の変数の作り方に合わせる)。
- `StoreGitHub` の 2 つの Store に `Account: "e2e-account"` を、`readonly` に `ReadOnly: true` を足す。
- `Checks: apptest.NewChecks()` を足す。

`cmd/hedgehug/links_e2e.go`:

```go
//go:build e2e

package main

import (
	"crypto/subtle"
	"net/http"
	"strconv"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// githubHandler は、POST /e2e/github?repo=…&number=…&kind=…&title=…&state=…&review=…&checks=…&moved=…&gone=… を受け、
// 偽の GitHub(e2eGitHub)に PR・Issue を置く(D3-23)。ほかの要求は next に渡す。API と同じトークンを求める。
func githubHandler(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/github" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		q := r.URL.Query()
		n, err := strconv.Atoi(q.Get("number"))
		if r.Method != http.MethodPost || err != nil || q.Get("repo") == "" {
			http.Error(w, "POST with repo and number", http.StatusBadRequest)
			return
		}
		if q.Get("gone") != "" { // 番号を消したことにする(取り直しの失敗の確かめ)
			e2eGitHub.Fail(q.Get("repo"), n, &ghcall.Failure{Kind: ghcall.NotFound, Repo: q.Get("repo"), Number: n})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		e2eGitHub.Fail(q.Get("repo"), n, nil)
		now := q.Get("moved")
		if now == "" {
			now = q.Get("repo")
		}
		review := itemlink.Review(q.Get("review"))
		if review == "" {
			review = itemlink.NoReview
		}
		e2eGitHub.Put(q.Get("repo"), port.Found{
			Kind: itemlink.Kind(q.Get("kind")), Repo: now, Number: n,
			Values: itemlink.Values{Title: q.Get("title"), State: itemlink.State(q.Get("state")), Review: review},
			Checks: itemlink.Checks(q.Get("checks")),
		})
		w.WriteHeader(http.StatusNoContent)
	})
}
```

`device_e2e.go` の `wrap` を `func wrap(token string, h http.Handler) http.Handler { return githubHandler(token, e2eDeviceFlow.handler(token, h)) }` にする。`e2eGitHub` が `*apptest.GitHub` でなければ(別の型で包んでいれば)、その型に `Put` を通す。

`internal/infra/github/live_test.go` に足す(検証用の組織のリポジトリ `hh-live`。トラック D の決定 11):

```go
func TestLiveLookup(t *testing.T) {
	token := liveToken(t)
	org := os.Getenv("HEDGEHUG_LIVE_ORG")
	// hh-live の #1 があることだけを前提にする(PR か Issue かは問わない。前回の github-live も hh-live の
	// PR と開いている Issue があることを前提にしていた)。
	f, err := New(nil).Lookup(context.Background(), token, org+"/hh-live", 1)
	if err != nil || f.Number != 1 || f.Title == "" || !strings.EqualFold(f.Repo, org+"/hh-live") {
		t.Errorf("Lookup = %+v, %v", f, err)
	}
	if _, err := New(nil).Lookup(context.Background(), token, org+"/hh-live", 999999); err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Errorf("missing number: %v", err)
	}
}
```

`.github/workflows/github-live.yaml` は、`go test -tags live ./internal/infra/github/...` を流しているなら変えない。

- [ ] **Step 4: 生成と確かめ**

Run:
```bash
make generate && (cd web && pnpm gen)
go test ./internal/adapter/... ./cmd/... && make check
```
Expected: PASS。`api/openapi.yaml` と `web/packages/api/src/generated.ts` に `listLinks`・`refreshLinks`・`addLink`・`removeLink`・`listProjectLinks` と型が出る。

- [ ] **Step 5: コミット**

```bash
git add internal/adapter internal/infra/github/live_test.go cmd/hedgehug api/openapi.yaml web/packages/api/src/generated.ts .github/workflows
git commit -m "feat(api): PR・Issue のリンクの API と github の出来事を足し、e2e の偽の GitHub に PR・Issue を置けるようにする(D3 Task 10)"
```

---
### Task 11: 画面の場所を空ける(wire.ts・index.ts を分ける)

**Files:**
- Create: `web/packages/core/src/accountsIndex.ts`、`web/app/src/accounts.ts`、`web/app/src/reloads.ts`
- Modify: `web/packages/core/src/index.ts`、`web/app/src/wire.ts`、`web/app/src/screens/settings/AccountsSection.svelte`・`AccountDialog.svelte`・`SignInDialog.svelte`

**Interfaces:**
- Consumes: なし(振る舞いを変えない移動)
- Produces:
  - `web/app/src/accounts.ts` が `accountList`・`accountForm`・`signIn` を export する(wire.ts からは消す)
  - `web/app/src/reloads.ts` の `export const reloads = new Set<() => Promise<unknown>>()`。wire.ts の `reflect` が、出来事 `store` と `github` のときに呼ぶ(Task 12 で `github` を足す)
  - wire.ts は `timer` を export する

- [ ] **Step 1: 移す前の確かめ**

Run: `cd web && wc -l app/src/wire.ts packages/core/src/index.ts && pnpm size`
Expected: どちらも 300 行。`pnpm size` は通る。

- [ ] **Step 2: core の index.ts を分ける**

`packages/core/src/accountsIndex.ts` を作り、index.ts の 2〜30 行(`features/accounts/form`・`list`・`signIn`・`text` の export)と、`model/account` の型の export(`Account` 〜 `TokenValues`)と、`export type { AccountsPort } from "./port/accounts";` を、そのまま移す。index.ts のそれらを消し、`export * from "./automation";` の次の行に `export * from "./accountsIndex";` を置く。

- [ ] **Step 3: app の wire.ts を分ける**

`app/src/accounts.ts`:

```ts
import { createAccountsPort } from "@hedgehug/api";
import { createAccountForm, createAccountList, createSignIn } from "@hedgehug/core";
import { navigation, timer, toasts } from "./wire";

// 設定「アカウントとメンバー」の GitHub アカウント(07)。wire.ts が 300 行なので、ここで作る。
// 3 つは互いを呼ぶので、呼ぶのは作ったあと。
const accountsPort = createAccountsPort();
export const accountList = createAccountList({
  accounts: accountsPort,
  toasts,
  reauthDevice: (account) => signIn.reauth(account),
});
export const accountForm = createAccountForm({
  accounts: accountsPort,
  toasts,
  signIn: (target, code) => signIn.begin(target, code),
  onSaved: () => void accountList.reload(),
});
export const signIn = createSignIn({
  accounts: accountsPort,
  toasts,
  timer,
  back: (target, failure) => accountForm.back(target, failure),
  onDone: () => void accountList.reload(),
});

// 画面を移ったら、アカウントのダイアログを閉じる(01「画面の移り方」。wire.ts のほかのダイアログと同じ)。
navigation.route.subscribe(() => {
  accountForm.close();
  signIn.close();
  accountList.cancelDelete();
});
```

wire.ts から、上と同じ 3 つの作成(「設定「アカウントとメンバー」の GitHub アカウント」の段落)、`navigation.route.subscribe` の中の `accountForm.close(); signIn.close(); accountList.cancelDelete();`、import の `createAccountsPort`・`createAccountForm`・`createAccountList`・`createSignIn` を消す。`const timer = {` を `export const timer = {` にする。

`app/src/reloads.ts`:

```ts
// 画面への反映(06「画面への反映」)で読み直す、wire.ts の外で作った状態。wire.ts が 300 行なので、
// 読み直しの関数はここに登録し、wire.ts の reflect が呼ぶ。
export const reloads = new Set<() => Promise<unknown>>();
```

wire.ts の `reflect` の `if (kinds.includes("store")) { ... }` のあとに足す:

```ts
  jobs.push(...[...reloads].map((reload) => reload()));
```

(Task 12 で `github` の出来事を足したら、`store` か `github` のときだけ呼ぶ形に直す。)import に `import { reloads } from "./reloads";` を足す。

`AccountsSection.svelte`・`AccountDialog.svelte`・`SignInDialog.svelte` の `import { ... } from "../../wire";` のうち、`accountList`・`accountForm`・`signIn` を `"../../accounts"` から読むように直す。

- [ ] **Step 4: 振る舞いが変わらないことを確かめる**

Run: `cd web && pnpm check && pnpm lint && pnpm test && pnpm size && wc -l app/src/wire.ts packages/core/src/index.ts`
Expected: すべて通る。wire.ts と index.ts は 280 行前後。続けて e2e の `accounts.spec.ts` を流す(番号を変えた一時の設定で。Global Constraints):

```bash
cd web && sed 's/4173/4273/g; s/reuseExistingServer: !process.env.CI/reuseExistingServer: false/' playwright.config.ts > playwright.d3.config.ts
pnpm exec playwright test -c playwright.d3.config.ts e2e/accounts.spec.ts
rm playwright.d3.config.ts
```
Expected: PASS(設定の `reuseExistingServer` の書き方が違えば、sed の式をそれに合わせる)

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src web/app/src
git commit -m "refactor(web): アカウントの状態と公開を別のファイルに移し、wire.ts と core の index.ts に場所を空ける(D3 Task 11)"
```

---

### Task 12: core の型・口・api の包み・SSE の github

**Files:**
- Create: `web/packages/core/src/model/link.ts`、`web/packages/core/src/port/links.ts`、`web/packages/core/src/githubIndex.ts`、`web/packages/api/src/links.ts`、`web/packages/api/src/links.test.ts`
- Modify: `web/packages/core/src/index.ts`(`export * from "./githubIndex";`)、`web/packages/core/src/model/sync.ts`(`SyncEventKind` に `"github"`)、`web/packages/core/src/features/sync/reflection.ts`(順に `"github"`)と `reflection.test.ts`、`web/packages/api/src/sync.ts`(`kindOf`)、`web/packages/api/src/index.ts`、`web/app/src/wire.ts`(`reflect`)

**Interfaces:**
- Consumes: Task 10 の生成された `listLinks`・`refreshLinks`・`addLink`・`removeLink`・`listProjectLinks`
- Produces:
  ```ts
  // model/link.ts
  export type LinkKind = "pr" | "issue";
  export type LinkState = "open" | "draft" | "merged" | "closed";
  export type LinkReview = "none" | "pending" | "approved" | "changes_requested";
  export type LinkChecks = "" | "success" | "failure" | "pending";
  export interface LinkRow { kind: LinkKind; repo: string; number: number; title: string; state: LinkState; review: LinkReview; checks: LinkChecks; error: string; }
  export interface ProjectLinkRow extends LinkRow { item: string; display: string; }
  export interface LinkInput { kind: LinkKind; repo: string; number: string; addToProject: boolean; }
  export type LinkTarget = Pick<LinkRow, "kind" | "repo" | "number">;
  export const linkKey = (r: LinkTarget) => `${r.kind}:${r.repo}#${r.number}`;
  // port/links.ts
  export interface LinksPort {
    list(storeId: string, itemId: string): Promise<LinkRow[]>;
    refresh(storeId: string, itemId: string): Promise<LinkRow[]>;
    add(storeId: string, itemId: string, input: LinkInput): Promise<void>;
    remove(storeId: string, itemId: string, target: LinkTarget): Promise<void>;
    listProject(storeId: string, projectId: string): Promise<ProjectLinkRow[]>;
  }
  // api
  export function createLinksPort(): LinksPort;
  ```

- [ ] **Step 1: 失敗するテストを書く**

`packages/api/src/links.test.ts`(`accounts.test.ts` と同じ `fetch` の差し替え):

```ts
import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createLinksPort } from "./links";

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const row = { kind: "pr", repo: "org/web", number: 41, title: "A", state: "open", review: "pending", checks: "", error: "" };

describe("links port", () => {
  it("reads rows with the narrow types", async () => {
    respond(200, [row]);
    expect(await createLinksPort().list("S1", "I1")).toEqual([row]);
  });

  it("sends the add body and reads 204", async () => {
    const fetch = respond(204, undefined);
    await createLinksPort().add("S1", "I1", { kind: "pr", repo: "org/web", number: "41", addToProject: true });
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/stores/S1/items/I1/links");
    expect(JSON.parse(String(init.body))).toEqual({ kind: "pr", repo: "org/web", number: "41", addToProject: true });
  });

  it("turns 422 into Refused and 404 into NotFound", async () => {
    respond(422, { detail: "番号を入力してください" });
    await expect(
      createLinksPort().add("S1", "I1", { kind: "pr", repo: "org/web", number: "x", addToProject: false }),
    ).rejects.toEqual(new Refused("番号を入力してください"));
    respond(404, { detail: "not found" });
    await expect(createLinksPort().refresh("S1", "I1")).rejects.toBeInstanceOf(NotFound);
  });
});
```

`reflection.test.ts` に足す:

```ts
  it("applies a github event", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.s.send("github");
    await t.advance(0);
    expect(t.applied.at(-1)).toEqual(["github"]);
  });
```

つないだとき・すべてを読み直すときの既存の期待値 `["store", "sync"]` は、並びに `"github"` を足すので `["store", "sync", "github"]` に直す(`order` をすべて当てる場面。`reflection.ts` の 77 行と 123 行)。

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm test --filter @hedgehug/api --filter @hedgehug/core`(このリポジトリの vitest の流し方に合わせる)
Expected: FAIL(`createLinksPort` がない、`"github"` が型にない)

- [ ] **Step 3: 実装を書く**

`model/link.ts` は Interfaces のとおり(コメント: 「PR・Issue のリンク(07「PR と Issue のリンク」)。checks はこのマシンが読んだ値で、Store に保存しない」)。`port/links.ts` も Interfaces のとおり(コメント: 「リンクの口。実装は api パッケージにある。断りは Refused、ないものは NotFound で投げる」)。

`githubIndex.ts`(Task 13・14・16 で export を足していく):

```ts
export type {
  LinkChecks,
  LinkInput,
  LinkKind,
  LinkReview,
  LinkRow,
  LinkState,
  LinkTarget,
  ProjectLinkRow,
} from "./model/link";
export { linkKey } from "./model/link";
export type { LinksPort } from "./port/links";
```

index.ts の `export * from "./accountsIndex";` の次に `export * from "./githubIndex";` を足す。

`api/src/links.ts`:

```ts
import type { LinkChecks, LinkKind, LinkReview, LinkRow, LinkState, LinksPort, ProjectLinkRow } from "@hedgehug/core";
import { call } from "./errors";
import { addLink, listLinks, listProjectLinks, refreshLinks, removeLink } from "./generated";

// 生成された型は enum を string で持つので、core の狭い型にそろえる。
type Wire = { kind: string; state: string; review: string; checks: string };
const narrow = <T extends Wire>(r: T) => ({
  ...r,
  kind: r.kind as LinkKind,
  state: r.state as LinkState,
  review: r.review as LinkReview,
  checks: r.checks as LinkChecks,
});

// createLinksPort は、PR・Issue のリンクの口(07)。
export function createLinksPort(): LinksPort {
  return {
    list: (storeId, itemId) =>
      call(async () => ((await listLinks(storeId, itemId)) ?? []).map(narrow) as LinkRow[]),
    refresh: (storeId, itemId) =>
      call(async () => ((await refreshLinks(storeId, itemId)) ?? []).map(narrow) as LinkRow[]),
    add: (storeId, itemId, input) =>
      call(async () => {
        await addLink(storeId, itemId, input);
      }),
    remove: (storeId, itemId, target) =>
      call(async () => {
        await removeLink(storeId, itemId, { kind: target.kind, repo: target.repo, number: target.number });
      }),
    listProject: (storeId, projectId) =>
      call(async () => ((await listProjectLinks(storeId, projectId)) ?? []).map(narrow) as ProjectLinkRow[]),
  };
}
```

生成された関数の名前・引数(本文を第 3 引数で取るか)は `generated.ts` を見て合わせる。

`api/src/index.ts` に `export { createLinksPort } from "./links";` を足す。

`model/sync.ts`: `export type SyncEventKind = "store" | "sync" | "github";`(コメントに「github はこのマシンが読んだ GitHub の値(チェックの結果)が変わった」を足す)。

`reflection.ts`: `const order: SyncEventKind[] = ["store", "sync", "github"];`

`api/src/sync.ts`: `kindOf` を `name === "store" || name === "sync" || name === "github" ? name : null` にする。

`app/src/wire.ts` の Task 11 で足した行を次にする:

```ts
  if (kinds.includes("store") || kinds.includes("github")) jobs.push(...[...reloads].map((r) => r()));
```

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm check && pnpm lint && pnpm test && pnpm size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages web/app/src/wire.ts
git commit -m "feat(web): リンクの型・口・api の包みと、github の出来事の反映を足す(D3 Task 12)"
```

---

### Task 13: core の GitHub の欄の状態

**Files:**
- Create: `web/packages/core/src/features/github/text.ts`、`web/packages/core/src/features/github/links.ts`、`web/packages/core/src/features/github/links.test.ts`、`web/packages/core/src/features/github/fixtures.test-util.ts`
- Modify: `web/packages/core/src/githubIndex.ts`

**Interfaces:**
- Consumes: Task 12 の `LinksPort`・`LinkRow`・`linkKey`、`AccountsPort.list`、`Toasts`、`Refused`・`NotFound`
- Produces:
  ```ts
  // text.ts
  export const noLinksText = "リンクはありません";
  export const unlinkedMessage = "リンクを解除しました";
  export const unlinkFailedMessage = "リンクを解除できませんでした"; // 仮(D3-22)
  export const noAccountNote = "GitHub のアカウントを登録すると使えます";
  export const markerCopiedMessage = "コピーしました。PR の説明に貼ってください";
  export const markerCopyFailedMessage = "コピーできませんでした。";
  export const markerOf = (itemId: string) => `hedgehug:${itemId}`;
  export const linkDialogTitle = (k: LinkKind) => (k === "pr" ? "PR をリンク" : "Issue をリンク");
  export const kindLabel = (k: LinkKind) => (k === "pr" ? "PR" : "Issue");
  export const stateLabel: Record<LinkState, string>; // Open・Draft・Merged・Closed
  export function reviewLabel(r: LinkReview): string; // none は ""
  export function checksLabel(c: LinkChecks): string; // "" は ""
  export const showsProgress = (r: LinkRow) => r.kind === "pr" && (r.state === "open" || r.state === "draft");
  export const linkText = (r: LinkTarget & { title: string }) => `${r.repo}#${r.number} — ${r.title}`;
  export const linkUrl = (r: LinkTarget) => `https://github.com/${r.repo}/${r.kind === "pr" ? "pull" : "issues"}/${r.number}`;
  // links.ts
  export interface LinkListState { loaded: boolean; rows: LinkRow[]; hasAccount: boolean; }
  export interface LinkList {
    state: Readable<LinkListState>;
    load(storeId: string, itemId: string): Promise<void>; // 一覧を出してから取り直す
    reload(): Promise<void>; // 一覧だけ(画面への反映)
    remove(row: LinkRow): Promise<void>;
    reset(): void;
  }
  export function createLinkList(deps: { links: LinksPort; accounts: Pick<AccountsPort, "list">; toasts: Toasts; onChanged(): void }): LinkList;
  ```

- [ ] **Step 1: 失敗するテストを書く**

`fixtures.test-util.ts`:

```ts
import type { LinkRow } from "../../model/link";
import type { LinksPort } from "../../port/links";

export const row = (p: Partial<LinkRow> = {}): LinkRow => ({
  kind: "pr",
  repo: "org/web",
  number: 41,
  title: "Add login form",
  state: "open",
  review: "pending",
  checks: "",
  error: "",
  ...p,
});

// fakeLinks は、呼び出しを覚え、答えを差し替えられる LinksPort。
export function fakeLinks() {
  const calls: string[] = [];
  const fake = {
    calls,
    rows: [row()] as LinkRow[],
    refreshed: null as LinkRow[] | null,
    fail: null as unknown,
    port: {} as LinksPort,
  };
  const check = () => {
    if (fake.fail) throw fake.fail;
  };
  fake.port = {
    async list(s, i) {
      calls.push(`list ${s} ${i}`);
      check();
      return fake.rows;
    },
    async refresh(s, i) {
      calls.push(`refresh ${s} ${i}`);
      check();
      return fake.refreshed ?? fake.rows;
    },
    async add(s, i, input) {
      calls.push(`add ${s} ${i} ${input.kind} ${input.repo} ${input.number} ${input.addToProject}`);
      check();
    },
    async remove(s, i, t) {
      calls.push(`remove ${s} ${i} ${t.kind}:${t.repo}#${t.number}`);
      check();
    },
    async listProject(s, p) {
      calls.push(`project ${s} ${p}`);
      check();
      return [];
    },
  };
  return fake;
}
```

`links.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { createLinkList } from "./links";
import { fakeLinks, row } from "./fixtures.test-util";
import { checksLabel, linkText, linkUrl, markerOf, reviewLabel, showsProgress, stateLabel } from "./text";

function setup(accounts: unknown[] = [{ id: "A1" }]) {
  const links = fakeLinks();
  const toasts: { text: string; error: boolean }[] = [];
  let changed = 0;
  const list = createLinkList({
    links: links.port,
    accounts: { list: async () => accounts as never },
    toasts: { show: (text: string, o?: { error?: boolean }) => toasts.push({ text, error: !!o?.error }) } as never,
    onChanged: () => {
      changed += 1;
    },
  });
  return { links, list, toasts, changed: () => changed };
}

describe("text", () => {
  it("writes the row words of 07", () => {
    expect(linkText(row())).toBe("org/web#41 — Add login form");
    expect(linkUrl(row())).toBe("https://github.com/org/web/pull/41");
    expect(linkUrl(row({ kind: "issue", number: 3 }))).toBe("https://github.com/org/web/issues/3");
    expect(stateLabel.merged).toBe("Merged");
    expect([reviewLabel("none"), reviewLabel("pending"), reviewLabel("approved"), reviewLabel("changes_requested")]).toEqual(["", "レビュー待ち", "承認済み", "修正依頼"]);
    expect([checksLabel(""), checksLabel("success"), checksLabel("failure"), checksLabel("pending")]).toEqual(["", "CI 成功", "CI 失敗", "CI 実行中"]);
    expect([showsProgress(row()), showsProgress(row({ state: "draft" })), showsProgress(row({ state: "merged" })), showsProgress(row({ kind: "issue" }))]).toEqual([true, true, false, false]);
    expect(markerOf("01ABC")).toBe("hedgehug:01ABC");
  });
});

describe("createLinkList", () => {
  it("shows the stored rows first, then the refreshed ones", async () => {
    const t = setup();
    t.links.refreshed = [row({ state: "merged", error: "" })];
    await t.list.load("S1", "I1");
    expect(t.links.calls).toEqual(["list S1 I1", "refresh S1 I1"]);
    expect(get(t.list.state)).toMatchObject({ loaded: true, hasAccount: true, rows: [row({ state: "merged" })] });
  });

  it("keeps the refresh errors when the list is read again", async () => {
    const t = setup();
    t.links.refreshed = [row({ error: "状態を更新できませんでした: x" })];
    await t.list.load("S1", "I1");
    await t.list.reload();
    expect(get(t.list.state).rows[0]?.error).toBe("状態を更新できませんでした: x");
  });

  it("knows when this machine has no account", async () => {
    const t = setup([]);
    await t.list.load("S1", "I1");
    expect(get(t.list.state).hasAccount).toBe(false);
  });

  it("removes with a toast and reads again", async () => {
    const t = setup();
    await t.list.load("S1", "I1");
    await t.list.remove(row());
    expect(t.links.calls).toContain("remove S1 I1 pr:org/web#41");
    expect(t.toasts).toEqual([{ text: "リンクを解除しました", error: false }]);
    expect(t.changed()).toBe(1);
  });

  it("shows a refusal, keeps quiet when the item is gone, and shows the fallback otherwise", async () => {
    const t = setup();
    await t.list.load("S1", "I1");
    t.links.fail = new Refused("断り");
    await t.list.remove(row());
    t.links.fail = new NotFound();
    await t.list.remove(row());
    t.links.fail = new Error("x");
    await t.list.remove(row());
    expect(t.toasts).toEqual([
      { text: "断り", error: true },
      { text: "リンクを解除できませんでした", error: true },
    ]);
  });

  it("keeps the refreshed rows when the list is read during the refresh", async () => {
    const t = setup();
    let release: () => void = () => undefined;
    t.links.port.refresh = () =>
      new Promise((resolve) => {
        release = () => resolve([row({ state: "merged", error: "状態を更新できませんでした: x" })]);
      });
    const loading = t.list.load("S1", "I1");
    await new Promise((r) => setTimeout(r, 0)); // 一覧を出し、取り直しを待つところまで進める
    await t.list.reload(); // 取り直しの途中の画面への反映
    release();
    await loading;
    expect(get(t.list.state).rows[0]).toMatchObject({ state: "merged", error: "状態を更新できませんでした: x" });
  });

  it("drops answers for an item that is no longer open", async () => {
    const t = setup();
    const first = t.list.load("S1", "I1");
    t.list.reset();
    await first;
    expect(get(t.list.state)).toMatchObject({ loaded: false, rows: [] });
  });
});
```

`NotFound` の作り方(引数)は `model/errors.ts` に合わせる。

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- features/github`
Expected: FAIL(`./links`・`./text` がない)

- [ ] **Step 3: 実装を書く**

`text.ts`(Interfaces のとおり。`stateLabel` は `{ open: "Open", draft: "Draft", merged: "Merged", closed: "Closed" }`、`reviewLabel` は `{ none: "", pending: "レビュー待ち", approved: "承認済み", changes_requested: "修正依頼" }[r]`、`checksLabel` は `{ "": "", success: "CI 成功", failure: "CI 失敗", pending: "CI 実行中" }[c]`)。先頭のコメント: 「GitHub の欄の文言(07「PR と Issue のリンク」「GitHub を使わない場合」、02「GitHub」)。unlinkFailedMessage は仕様にない仮の文言(D3-22)」。

`links.ts`:

```ts
import { NotFound, Refused } from "../../model/errors";
import { type LinkRow, linkKey } from "../../model/link";
import type { AccountsPort } from "../../port/accounts";
import type { LinksPort } from "../../port/links";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { unlinkFailedMessage, unlinkedMessage } from "./text";

// 詳細パネルの GitHub の欄(07「表示と更新」「解除」)の状態: 一覧、取り直しの失敗、アカウントの有無。

export interface LinkListState {
  // loaded は、一覧を読み終えたか(読むまでは「リンクはありません」を出さない)。
  loaded: boolean;
  rows: LinkRow[];
  // hasAccount は、このマシンにアカウントが 1 つ以上あるか(なければリンクの追加を無効にする。07)。
  hasAccount: boolean;
}

export interface LinkList {
  state: Readable<LinkListState>;
  load(storeId: string, itemId: string): Promise<void>;
  reload(): Promise<void>;
  remove(row: LinkRow): Promise<void>;
  reset(): void;
}

export interface LinkListDeps {
  links: LinksPort;
  accounts: Pick<AccountsPort, "list">;
  toasts: Toasts;
  // onChanged は、解除したあと(プロジェクトの概要の一覧を読み直す)。
  onChanged(): void;
}

const blank: LinkListState = { loaded: false, rows: [], hasAccount: true };

export function createLinkList(deps: LinkListDeps): LinkList {
  const state = writable<LinkListState>(blank);
  let target: { storeId: string; itemId: string } | null = null;
  // errors は、最後の取り直しの失敗の文言(一覧だけを読み直しても、行に残す)。
  let errors = new Map<string, string>();
  // lists・refreshes は、古い答えを捨てるための通し番号。一覧と取り直しは別に数える(取り直しの途中に画面への
  // 反映の一覧が入っても、取り直しの答えと失敗の文言を捨てない)。取り直しの答えを出したら、それより前に
  // 出した一覧の答え(古い値)も捨てる。
  let lists = 0;
  let refreshes = 0;

  const withErrors = (rows: LinkRow[]) =>
    rows.map((r) => ({ ...r, error: r.error || errors.get(linkKey(r)) || "" }));
  const show = (rows: LinkRow[]) => state.update((s) => ({ ...s, loaded: true, rows: withErrors(rows) }));

  // list は、一覧を出す。失敗は出さない(画面への反映は失敗を出さない。A2 の決定 29)。
  async function list() {
    if (!target) return;
    const t = target;
    lists += 1;
    const mine = lists;
    try {
      const rows = await deps.links.list(t.storeId, t.itemId);
      if (mine === lists && target === t) show(rows);
    } catch (e) {
      if (e instanceof NotFound && mine === lists && target === t) show([]);
    }
  }

  // refresh は、GitHub から取り直した一覧を出し、失敗の文言を覚える。
  async function refresh() {
    if (!target) return;
    const t = target;
    refreshes += 1;
    const mine = refreshes;
    try {
      const rows = await deps.links.refresh(t.storeId, t.itemId);
      if (mine !== refreshes || target !== t) return;
      lists += 1;
      errors = new Map(rows.filter((r) => r.error).map((r) => [linkKey(r), r.error]));
      show(rows);
    } catch (e) {
      if (e instanceof NotFound && mine === refreshes && target === t) show([]);
    }
  }

  async function readAccounts() {
    try {
      const rows = await deps.accounts.list();
      if (target) state.update((s) => ({ ...s, hasAccount: rows.length > 0 }));
    } catch {
      // 読めなければ、いまの表示のまま(サーバーが断る)。
    }
  }

  return {
    state: readonly(state),
    async load(storeId, itemId) {
      if (target?.storeId !== storeId || target.itemId !== itemId) {
        target = { storeId, itemId };
        errors = new Map();
        state.set(blank);
      }
      await Promise.all([list(), readAccounts()]);
      await refresh();
    },
    reload: list,
    async remove(row) {
      if (!target) return;
      try {
        await deps.links.remove(target.storeId, target.itemId, row);
        deps.toasts.show(unlinkedMessage);
      } catch (e) {
        if (!(e instanceof NotFound)) {
          deps.toasts.show(e instanceof Refused ? e.message : unlinkFailedMessage, { error: true });
        }
      }
      await list();
      deps.onChanged();
    },
    reset() {
      target = null;
      lists += 1;
      refreshes += 1;
      errors = new Map();
      state.set(blank);
    },
  };
}
```

`withErrors` の `r.error ||` は、取り直しの答え(`r.error` を持つ)をそのまま使い、一覧の答え(いつも空)には覚えた失敗を足すため。

`githubIndex.ts` に足す:

```ts
export { createLinkList, type LinkList, type LinkListState } from "./features/github/links";
export {
  checksLabel,
  kindLabel,
  linkDialogTitle,
  linkText,
  linkUrl,
  markerCopiedMessage,
  markerCopyFailedMessage,
  markerOf,
  noAccountNote,
  noLinksText,
  reviewLabel,
  showsProgress,
  stateLabel,
  unlinkedMessage,
} from "./features/github/text";
```

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm check && pnpm lint && pnpm test && pnpm size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src
git commit -m "feat(core): 詳細パネルの GitHub の欄の状態(一覧・取り直し・解除・アカウントの有無)を足す(D3 Task 13)"
```

---

### Task 14: core のリンクのダイアログ

**Files:**
- Create: `web/packages/core/src/features/github/linkDialog.ts`、`web/packages/core/src/features/github/linkDialog.test.ts`
- Modify: `web/packages/core/src/githubIndex.ts`

**Interfaces:**
- Consumes: Task 12 の `LinksPort`・`LinkKind`、Task 13 の `fakeLinks`
- Produces:
  ```ts
  export interface LinkDialogState {
    kind: LinkKind;
    repos: string[];      // プロジェクトのリポジトリ(あれば選ぶ。なければ入力)
    repo: string;
    number: string;
    addToProject: boolean; // repos が空のときだけ使う。既定はオン
    asking: boolean;       // GitHub に問い合わせている(「リンク」を無効にし「GitHub に問い合わせています」を出す)
    error: string;         // 断りの文言(赤字)
  }
  export interface LinkDialog {
    state: Readable<LinkDialogState | null>;
    open(kind: LinkKind, storeId: string, itemId: string, repos: string[]): void;
    update(p: Partial<Pick<LinkDialogState, "repo" | "number" | "addToProject">>): void;
    submit(): Promise<void>;
    close(): void;
  }
  export function createLinkDialog(deps: { links: LinksPort; onAdded(): void }): LinkDialog;
  ```

- [ ] **Step 1: 失敗するテストを書く**

```ts
import { describe, expect, it } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeLinks } from "./fixtures.test-util";
import { createLinkDialog } from "./linkDialog";

function setup() {
  const links = fakeLinks();
  let added = 0;
  const dialog = createLinkDialog({ links: links.port, onAdded: () => (added += 1) });
  return { links, dialog, added: () => added };
}

describe("createLinkDialog", () => {
  it("picks the first project repository and does not add it again", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", ["org/web", "org/api"]);
    expect(get(t.dialog.state)).toMatchObject({ kind: "pr", repo: "org/web", number: "", addToProject: true, asking: false, error: "" });
    t.dialog.update({ number: "41" });
    await t.dialog.submit();
    expect(t.links.calls).toEqual(["add S1 I1 pr org/web 41 false"]);
    expect(get(t.dialog.state)).toBeNull();
    expect(t.added()).toBe(1);
  });

  it("sends the typed repository and the check when the project has none", async () => {
    const t = setup();
    t.dialog.open("issue", "S1", "I1", []);
    t.dialog.update({ repo: "org/api", number: "12" });
    await t.dialog.submit();
    expect(t.links.calls).toEqual(["add S1 I1 issue org/api 12 true"]);
  });

  it("keeps the dialog open with the refusal", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", []);
    t.links.fail = new Refused("番号を入力してください");
    await t.dialog.submit();
    expect(get(t.dialog.state)).toMatchObject({ asking: false, error: "番号を入力してください" });
    expect(t.added()).toBe(0);
  });

  it("asks once at a time and drops the answer after closing", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", ["org/web"]);
    t.dialog.update({ number: "41" });
    const first = t.dialog.submit();
    expect(get(t.dialog.state)?.asking).toBe(true);
    await t.dialog.submit(); // 問い合わせ中: 送らない
    t.dialog.close();
    await first;
    expect(t.links.calls).toHaveLength(1);
    expect(get(t.dialog.state)).toBeNull();
  });

  it("closes when the item is gone and keeps quiet on other failures", async () => {
    const t = setup();
    t.dialog.open("pr", "S1", "I1", ["org/web"]);
    t.links.fail = new Error("network");
    await t.dialog.submit();
    expect(get(t.dialog.state)).toMatchObject({ asking: false, error: "" });
    t.links.fail = new NotFound();
    await t.dialog.submit();
    expect(get(t.dialog.state)).toBeNull();
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- linkDialog`
Expected: FAIL(`./linkDialog` がない)

- [ ] **Step 3: 実装を書く**

```ts
import { NotFound, Refused } from "../../model/errors";
import type { LinkKind } from "../../model/link";
import type { LinksPort } from "../../port/links";
import { get, type Readable, readonly, writable } from "../../reactive/store";

// 「PR をリンク」「Issue をリンク」のダイアログ(07「追加」)。題名と種別だけが違う。検査はサーバーに任せ、
// 断りはダイアログの赤字に出す。問い合わせのあいだは「リンク」を無効にする(D2 画面の決定 S15 と同じ)。
// 断り以外の失敗は出さない(D2 画面の決定 S20)。リンクできたら閉じ、トーストは出さない。

export interface LinkDialogState {
  kind: LinkKind;
  repos: string[];
  repo: string;
  number: string;
  addToProject: boolean;
  asking: boolean;
  error: string;
}

export interface LinkDialog {
  state: Readable<LinkDialogState | null>;
  open(kind: LinkKind, storeId: string, itemId: string, repos: string[]): void;
  update(p: Partial<Pick<LinkDialogState, "repo" | "number" | "addToProject">>): void;
  submit(): Promise<void>;
  close(): void;
}

export interface LinkDialogDeps {
  links: LinksPort;
  // onAdded は、リンクしたあと(GitHub の欄と概要の一覧を読み直す)。
  onAdded(): void;
}

export function createLinkDialog(deps: LinkDialogDeps): LinkDialog {
  const state = writable<LinkDialogState | null>(null);
  let target: { storeId: string; itemId: string } | null = null;
  // opened は、開くたびに増やす(閉じたあとに届いた答えを捨てる)。
  let opened = 0;

  const patch = (p: Partial<LinkDialogState>) => state.update((s) => (s ? { ...s, ...p } : s));

  function close() {
    opened += 1;
    target = null;
    state.set(null);
  }

  return {
    state: readonly(state),
    open(kind, storeId, itemId, repos) {
      opened += 1;
      target = { storeId, itemId };
      state.set({ kind, repos, repo: repos[0] ?? "", number: "", addToProject: true, asking: false, error: "" });
    },
    update: patch,
    async submit() {
      const s = get(state);
      if (!s || !target || s.asking) return;
      const mine = opened;
      const t = target;
      patch({ asking: true, error: "" });
      try {
        await deps.links.add(t.storeId, t.itemId, {
          kind: s.kind,
          repo: s.repo,
          number: s.number,
          addToProject: s.repos.length === 0 && s.addToProject,
        });
      } catch (e) {
        if (mine !== opened) return;
        if (e instanceof NotFound) {
          close();
          deps.onAdded();
          return;
        }
        patch({ asking: false, error: e instanceof Refused ? e.message : "" });
        return;
      }
      if (mine !== opened) return;
      close();
      deps.onAdded();
    },
    close,
  };
}
```

`githubIndex.ts` に `export { createLinkDialog, type LinkDialog, type LinkDialogState } from "./features/github/linkDialog";` を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm check && pnpm lint && pnpm test && pnpm size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src
git commit -m "feat(core): PR・Issue をリンクするダイアログの状態を足す(D3 Task 14)"
```

---
### Task 15: 画面の GitHub の欄とダイアログ

**Files:**
- Create: `web/app/src/github.ts`、`web/app/src/screens/github/LinkSection.svelte`・`LinkRow.svelte`・`LinkChips.svelte`・`LinkDialog.svelte`
- Modify: `web/app/src/screens/item/DetailPanel.svelte`(109〜112 行の仮の枠を `<LinkSection {v} />` に替える)

**Interfaces:**
- Consumes: Task 12〜14 の `createLinksPort`・`createLinkList`・`createLinkDialog`・文言、`createAccountsPort`、`reloads`(Task 11)、`DetailView`(`v.item.id`・`v.store.id`・`v.readOnly`・`v.project.repos`)、`readOnlyNote`
- Produces: `web/app/src/github.ts` の `linkList`・`linkDialog`・`projectLinks`(Task 16 で足す)。`LinkChips.svelte`(Task 16 も使う)

app には部品のテストの基盤がない(A3b の記録)。状態と規則は Task 13・14 の vitest で確かめ、ここは svelte-check・Biome・ビルドと Task 17 の e2e で確かめる。

- [ ] **Step 1: 配線を書く**

`app/src/github.ts`:

```ts
import { createAccountsPort, createLinksPort } from "@hedgehug/api";
import { createLinkDialog, createLinkList } from "@hedgehug/core";
import { reloads } from "./reloads";
import { catalog, detail, navigation, toasts } from "./wire";

// GitHub の欄(07「PR と Issue のリンク」)。wire.ts が 300 行なので、ここで作る。
const port = createLinksPort();
export const linkList = createLinkList({
  links: port,
  accounts: createAccountsPort(),
  toasts,
  onChanged: () => undefined, // Task 16 で概要の一覧の読み直しにする
});
// リンクしたら、一覧と、プロジェクトのリポジトリ(「プロジェクトのリポジトリにも追加する」で増え、次に開く
// ダイアログの選択肢になる)を読み直す。projectEditor.ts と同じく、カタログを読んでから詳細パネルを読み直す。
export const linkDialog = createLinkDialog({
  links: port,
  onAdded: () => void Promise.all([linkList.reload(), catalog.load().then(() => detail.reload())]),
});
reloads.add(() => linkList.reload());
// 画面を移ったら、ダイアログを閉じる(01「画面の移り方」。前のアイテム宛てのダイアログを残さない)。
navigation.route.subscribe(() => linkDialog.close());
```

- [ ] **Step 2: 部品を書く**

`LinkChips.svelte`:

```svelte
<script lang="ts">
  import { checksLabel, kindLabel, type LinkRow, reviewLabel, showsProgress, stateLabel } from "@hedgehug/core";

  // リンクの行のチップ(07「表示と更新」): 種別と状態、Open・Draft の PR だけレビューとチェック(なしは出さない)。
  interface Props {
    row: LinkRow;
  }

  let { row }: Props = $props();
  const review = $derived(showsProgress(row) ? reviewLabel(row.review) : "");
  const checks = $derived(showsProgress(row) ? checksLabel(row.checks) : "");
</script>

<span class="chip {row.state}">{kindLabel(row.kind)} {stateLabel[row.state]}</span>
{#if review}<span class="chip">{review}</span>{/if}
{#if checks}<span class="chip {row.checks}">{checks}</span>{/if}

<style>
  .chip {
    padding: 0 6px;
    border-radius: 9px;
    font-size: 11.5px;
    white-space: nowrap;
    background: var(--bg-soft, var(--line));
    color: var(--ink);
  }
  .open,
  .success {
    color: var(--ok, var(--ink));
  }
  .merged {
    color: var(--accent, var(--ink));
  }
  .closed,
  .failure {
    color: var(--danger);
  }
</style>
```

色の名前(`--ok`・`--accent`・`--bg-soft`)は `web/packages/ui` のテーマの変数を見て、あるものに替える(ないものの代わりは書いたとおり)。

`LinkRow.svelte`:

```svelte
<script lang="ts">
  import { type LinkRow, linkText, linkUrl } from "@hedgehug/core";
  import { IconButton } from "@hedgehug/ui";
  import LinkChips from "./LinkChips.svelte";

  // リンクの 1 行: チップ、「{owner/name}#{番号} — {タイトル}」(GitHub を新しいタブで開く)、解除の「×」。
  // 取り直しに失敗した行は、その下に理由を赤字で出す(07「表示と更新」)。
  interface Props {
    row: LinkRow;
    disabled: boolean;
    title?: string;
    onremove: () => void;
  }

  let { row, disabled, title, onremove }: Props = $props();
</script>

<div class="link-row">
  <LinkChips {row} />
  <a class="row-title" href={linkUrl(row)} target="_blank" rel="noopener noreferrer">{linkText(row)}</a>
  <IconButton label="{row.repo}#{row.number} のリンクを解除" size="xs" {disabled} {title} onclick={onremove}>×</IconButton>
</div>
{#if row.error}
  <p class="row-error" role="alert">{row.error}</p>
{/if}

<style>
  .link-row {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
    overflow-wrap: anywhere;
  }
  .row-title:hover {
    text-decoration: underline;
  }
  .row-error {
    color: var(--danger);
    font-size: 12.5px;
    margin: 0;
  }
</style>
```

`LinkDialog.svelte`:

```svelte
<script lang="ts">
  import { askingText, linkDialogTitle } from "@hedgehug/core";
  import { Button, Checkbox, Dialog, Input, Select } from "@hedgehug/ui";
  import { linkDialog } from "../../github";

  // 「PR をリンク」「Issue をリンク」(07「追加」)。リポジトリは、プロジェクトにあれば選び、なければ入力する。
  const dialog = linkDialog.state;

  function onkeydown(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) void linkDialog.submit();
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title={linkDialogTitle(s.kind)} onclose={linkDialog.close}>
    <div class="form">
      <div class="field">
        <span>リポジトリ</span>
        {#if s.repos.length > 0}
          <Select
            label="リポジトリ"
            options={s.repos.map((r) => ({ value: r, label: r }))}
            bind:value={() => s.repo, (v) => linkDialog.update({ repo: v })}
          />
        {:else}
          <Input
            label="リポジトリ"
            placeholder="owner/name"
            focusOnMount
            {onkeydown}
            bind:value={() => s.repo, (v) => linkDialog.update({ repo: v })}
          />
        {/if}
      </div>
      <div class="field">
        <span>番号</span>
        <Input
          label="番号"
          focusOnMount={s.repos.length > 0}
          {onkeydown}
          bind:value={() => s.number, (v) => linkDialog.update({ number: v })}
        />
      </div>
      {#if s.repos.length === 0}
        <Checkbox
          label="プロジェクトのリポジトリにも追加する"
          bind:checked={() => s.addToProject, (v) => linkDialog.update({ addToProject: v })}
        />
      {/if}
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
      {#if s.asking}
        <p class="muted">{askingText}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={linkDialog.close}>キャンセル</Button>
      <Button variant="primary" disabled={s.asking} onclick={() => void linkDialog.submit()}>リンク</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .muted {
    color: var(--muted);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
```

`Input` に `focusOnMount` の真偽を渡せないときは、`{#if}` で 2 つに分ける。

`LinkSection.svelte`:

```svelte
<script lang="ts">
  import {
    type DetailView,
    type LinkKind,
    markerCopiedMessage,
    markerCopyFailedMessage,
    markerOf,
    noAccountNote,
    noLinksText,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button } from "@hedgehug/ui";
  import { onDestroy } from "svelte";
  import { linkDialog, linkList } from "../../github";
  import { toasts } from "../../wire";
  import LinkDialog from "./LinkDialog.svelte";
  import LinkRow from "./LinkRow.svelte";

  // 詳細パネルの GitHub の欄(02「GitHub」、07「PR と Issue のリンク」)。開いたら一覧を出し、GitHub から取り直す。
  // 閲覧のみの Store では、リンクの追加と解除を無効にする(印のコピーと外部リンクは使える)。アカウントが
  // 1 つもなければ、リンクの追加を無効にする(07「GitHub を使わない場合」)。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();
  const model = linkList.state;
  const storeId = $derived(v.store.id);
  const itemId = $derived(v.item.id);
  const addNote = $derived(v.readOnly ? readOnlyNote : $model.hasAccount ? undefined : noAccountNote);
  const removeNote = $derived(v.readOnly ? readOnlyNote : undefined);

  $effect(() => {
    void linkList.load(storeId, itemId);
  });
  onDestroy(() => linkList.reset());

  function open(kind: LinkKind) {
    linkDialog.open(kind, storeId, itemId, v.project.repos);
  }

  async function copyMarker() {
    try {
      await navigator.clipboard.writeText(markerOf(itemId));
      toasts.show(markerCopiedMessage);
    } catch {
      toasts.show(markerCopyFailedMessage, { error: true });
    }
  }
</script>

<section class="peek-sec" aria-label="GitHub">
  <div class="sec-head">
    <h3>GitHub</h3>
    <span class="grow"></span>
    <Button size="sm" onclick={() => void copyMarker()}>PR 用の印をコピー</Button>
  </div>
  {#if $model.loaded && $model.rows.length === 0}
    <p class="muted">{noLinksText}</p>
  {/if}
  {#each $model.rows as row (`${row.kind}:${row.repo}#${row.number}`)}
    <LinkRow {row} disabled={v.readOnly} title={removeNote} onremove={() => void linkList.remove(row)} />
  {/each}
  <div class="add-row">
    <Button size="sm" disabled={addNote !== undefined} title={addNote} onclick={() => open("pr")}>PR をリンク</Button>
    <Button size="sm" disabled={addNote !== undefined} title={addNote} onclick={() => open("issue")}>Issue をリンク</Button>
  </div>
  <LinkDialog />
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .add-row {
    display: flex;
    gap: 6px;
  }
</style>
```

`v.project` の名前と `repos` の場所は `features/items/detail.ts` の `DetailView` を見て合わせる(Explore の報告では `v.project.repos`)。

`DetailPanel.svelte`: 109〜112 行を `<LinkSection {v} />` に替え、import に `import LinkSection from "../github/LinkSection.svelte";` を足す。`.peek-sec h3` と `.muted` のスタイルが DetailPanel.svelte でほかに使われていなければ消す(使っていれば残す)。

- [ ] **Step 3: 確かめる**

Run: `cd web && pnpm check && pnpm lint && pnpm test && pnpm size && pnpm build`
Expected: PASS(各ファイルは 200 行未満)

- [ ] **Step 4: コミット**

```bash
git add web/app/src
git commit -m "feat(web): 詳細パネルの GitHub の欄(リンクの行・解除・PR 用の印・リンクのダイアログ)を作る(D3 Task 15)"
```

---

### Task 16: 概要の「PR と Issue」

**Files:**
- Create: `web/packages/core/src/features/github/projectLinks.ts`、`projectLinks.test.ts`、`web/app/src/screens/github/ProjectLinks.svelte`
- Modify: `web/packages/core/src/githubIndex.ts`、`web/app/src/github.ts`、`web/app/src/screens/project/OverviewRepos.svelte`(区画の最後に `<ProjectLinks {view} />`)

**Interfaces:**
- Consumes: Task 12 の `LinksPort.listProject`・`ProjectLinkRow`、Task 13 の `fakeLinks`・`LinkChips.svelte`(Task 15)
- Produces:
  ```ts
  export interface ProjectLinksState { loaded: boolean; rows: ProjectLinkRow[] }
  export interface ProjectLinks {
    state: Readable<ProjectLinksState>;
    load(storeId: string, projectId: string): Promise<void>;
    reload(): Promise<void>;
    reset(): void;
  }
  export function createProjectLinks(deps: { links: LinksPort }): ProjectLinks;
  ```

- [ ] **Step 1: 失敗するテストを書く**

```ts
import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeLinks, row } from "./fixtures.test-util";
import { createProjectLinks } from "./projectLinks";

describe("createProjectLinks", () => {
  it("reads the rows of the open project and reads them again", async () => {
    const links = fakeLinks();
    const rows = [{ ...row(), item: "I1", display: "HH-2" }];
    links.port.listProject = async (s, p) => {
      links.calls.push(`project ${s} ${p}`);
      return rows;
    };
    const p = createProjectLinks({ links: links.port });
    await p.reload(); // 開いていなければ何もしない
    await p.load("S1", "P1");
    await p.reload();
    expect(links.calls).toEqual(["project S1 P1", "project S1 P1"]);
    expect(get(p.state)).toEqual({ loaded: true, rows });
  });

  it("empties when the project is gone", async () => {
    const links = fakeLinks();
    links.port.listProject = async () => {
      throw new NotFound();
    };
    const p = createProjectLinks({ links: links.port });
    await p.load("S1", "P1");
    expect(get(p.state)).toEqual({ loaded: true, rows: [] });
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- projectLinks`
Expected: FAIL

- [ ] **Step 3: 実装を書く**

`projectLinks.ts`:

```ts
import { NotFound } from "../../model/errors";
import type { ProjectLinkRow } from "../../model/link";
import type { LinksPort } from "../../port/links";
import { type Readable, readonly, writable } from "../../reactive/store";

// プロジェクトの概要の「PR と Issue」(02「概要」)。GitHub には問い合わせず、Store に保存した値と、
// このマシンが読んだチェックの結果を出す。失敗は出さない(A2 の決定 29)。

export interface ProjectLinksState {
  loaded: boolean;
  rows: ProjectLinkRow[];
}

export interface ProjectLinks {
  state: Readable<ProjectLinksState>;
  load(storeId: string, projectId: string): Promise<void>;
  reload(): Promise<void>;
  reset(): void;
}

const blank: ProjectLinksState = { loaded: false, rows: [] };

export function createProjectLinks(deps: { links: LinksPort }): ProjectLinks {
  const state = writable<ProjectLinksState>(blank);
  let target: { storeId: string; projectId: string } | null = null;
  let loads = 0;

  async function read() {
    if (!target) return;
    const t = target;
    loads += 1;
    const mine = loads;
    try {
      const rows = await deps.links.listProject(t.storeId, t.projectId);
      if (mine === loads) state.set({ loaded: true, rows });
    } catch (e) {
      if (e instanceof NotFound && mine === loads) state.set({ loaded: true, rows: [] });
    }
  }

  return {
    state: readonly(state),
    async load(storeId, projectId) {
      if (target?.storeId !== storeId || target.projectId !== projectId) {
        target = { storeId, projectId };
        state.set(blank);
      }
      await read();
    },
    reload: read,
    reset() {
      target = null;
      loads += 1;
      state.set(blank);
    },
  };
}
```

`githubIndex.ts` に `export { createProjectLinks, type ProjectLinks, type ProjectLinksState } from "./features/github/projectLinks";` を足す。

`app/src/github.ts` に足し、`linkList` の `onChanged` と `linkDialog` の `onAdded` で概要の一覧も読み直す:

```ts
export const projectLinks = createProjectLinks({ links: port });
reloads.add(() => projectLinks.reload());
```

```ts
  onChanged: () => void projectLinks.reload(),
```

```ts
export const linkDialog = createLinkDialog({
  links: port,
  onAdded: () =>
    void Promise.all([linkList.reload(), projectLinks.reload(), catalog.load().then(() => detail.reload())]),
});
```

(`projectLinks` を `linkList` より前に作る。)

`ProjectLinks.svelte`:

```svelte
<script lang="ts">
  import { linkText, linkUrl, noLinksText, type ProjectView } from "@hedgehug/core";
  import { onDestroy } from "svelte";
  import { projectLinks } from "../../github";
  import { detail, links } from "../../wire";
  import LinkChips from "./LinkChips.svelte";

  // 「リポジトリと PR」の区画の下段「PR と Issue」(02「概要」)。行の ID を押すと、そのアイテムの詳細パネルを開く。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();
  const model = projectLinks.state;
  const storeId = $derived(view.store.id);
  const projectId = $derived(view.project.id);

  $effect(() => {
    void projectLinks.load(storeId, projectId);
  });
  onDestroy(() => projectLinks.reset());

  const route = (display: string) => ({ name: "item" as const, store: view.store.name, item: display });
</script>

<h3>PR と Issue</h3>
{#if $model.loaded && $model.rows.length === 0}
  <p class="empty">{noLinksText}</p>
{/if}
<ul class="links" aria-label="PR と Issue">
  {#each $model.rows as r (`${r.item}:${r.kind}:${r.repo}#${r.number}`)}
    <li class="link">
      <LinkChips row={r} />
      <a class="row-title" href={linkUrl(r)} target="_blank" rel="noopener noreferrer">{linkText(r)}</a>
      <a
        class="id"
        href={links.href(route(r.display))}
        onclick={(e) => links.click(e, route(r.display), () => void detail.open(storeId, r.item))}
        onauxclick={(e) => links.click(e, route(r.display))}>{r.display}</a
      >
    </li>
  {/each}
</ul>

<style>
  h3 {
    font-size: 13px;
    margin: 8px 0 0;
  }
  .empty {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
  .links {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .link {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
    overflow-wrap: anywhere;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
</style>
```

`ProjectView` の `store`・`project` の名前と、詳細パネルを開く経路の名前(`route`)は、`RelationsSection.svelte` と `OverviewRepos.svelte` に合わせる。

`OverviewRepos.svelte` の `</section>` の直前に `<ProjectLinks {view} />` を置き、import を足す。区画のコメントに「下段の「PR と Issue」は ProjectLinks」を足す。

- [ ] **Step 4: 確かめる**

Run: `cd web && pnpm check && pnpm lint && pnpm test && pnpm size && pnpm build`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src web/app/src
git commit -m "feat(web): プロジェクトの概要に「PR と Issue」の一覧を出す(D3 Task 16)"
```

---

### Task 17: e2e

**Files:**
- Create: `web/e2e/links.spec.ts`
- Modify: `web/e2e/notes-readonly.spec.ts`(閲覧のみの GitHub の欄)

**Interfaces:**
- Consumes: Task 10 の `/e2e/github`、Task 15・16 の画面。ヘルパー `tokenUrl`・`unique`・`prefixOf`・`createWorkspace`・`newTaskUnder`・`newTask`(`web/e2e/helpers.ts`)

偽の GitHub は 1 つのサーバーを全部の幅(phone・tablet・desktop)と並列のテストで共有するので、リポジトリ名に `prefixOf(info)` を入れて重ならないようにする。

- [ ] **Step 1: テストを書く**

```ts
import { expect, type Page, type TestInfo, test } from "@playwright/test";
import { createWorkspace, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// gh は、e2e のサーバーの偽の GitHub に PR・Issue を置く(D3-23)。
async function gh(page: Page, q: Record<string, string>) {
  const res = await page.request.post(`/e2e/github?${new URLSearchParams(q)}`, {
    headers: { Authorization: "Bearer e2e-token" },
  });
  expect(res.status()).toBe(204);
}

// setup は、新しいワークスペース・プロジェクト(リポジトリなし)・Epic・Task を作り、Task の詳細パネルを開く。
async function setup(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, `リンク ${prefix}`), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const repo = `org-${prefix.toLowerCase()}/web`;
  return { prefix, panel, repo, github: panel.getByRole("region", { name: "GitHub" }) };
}

async function link(page: Page, section: ReturnType<Page["getByRole"]>, kind: "PR" | "Issue", repo: string | null, n: string) {
  await section.getByRole("button", { name: `${kind} をリンク` }).click();
  const dialog = page.getByRole("dialog", { name: `${kind} をリンク` });
  // repo が null なら、プロジェクトのリポジトリから選ぶ(セレクトが出るのを待つ)
  if (repo === null) await expect(dialog.getByRole("combobox", { name: "リポジトリ" })).toBeVisible();
  else await dialog.getByRole("textbox", { name: "リポジトリ" }).fill(repo);
  await dialog.getByRole("textbox", { name: "番号" }).fill(n);
  await dialog.getByRole("button", { name: "リンク" }).click();
  return dialog;
}

test("PR をリンクし、プロジェクトのリポジトリにも追加し、解除する", async ({ page }, info) => {
  const { prefix, github, repo } = await setup(page, info);
  await gh(page, { repo, number: "41", kind: "pr", title: "Add login form", state: "open", review: "pending", checks: "failure" });
  await expect(github.getByText("リンクはありません")).toBeVisible();
  const dialog = await link(page, github, "PR", repo, "41");
  await expect(dialog).toBeHidden();
  await expect(github.getByRole("link", { name: `${repo}#41 — Add login form` })).toHaveAttribute("href", `https://github.com/${repo}/pull/41`);
  await expect(github.getByText("PR Open")).toBeVisible();
  await expect(github.getByText("レビュー待ち")).toBeVisible();
  await expect(github.getByText("CI 失敗")).toBeVisible();
  // プロジェクトのリポジトリに加わり、概要の「PR と Issue」に出る
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  await expect(page.getByRole("list", { name: "リポジトリ" }).getByText(repo)).toBeVisible();
  await expect(page.getByRole("list", { name: "PR と Issue" }).getByRole("link", { name: `${prefix}-2` })).toBeVisible();
  // 解除
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const again = page.getByRole("region", { name: "詳細パネル" }).getByRole("region", { name: "GitHub" });
  await again.getByRole("button", { name: `${repo}#41 のリンクを解除` }).click();
  await expect(page.getByText("リンクを解除しました")).toBeVisible();
  await expect(again.getByText("リンクはありません")).toBeVisible();
});

test("リンクの断り方(07「追加」の表)", async ({ page }, info) => {
  const { github, repo } = await setup(page, info);
  await gh(page, { repo, number: "1", kind: "issue", title: "落ちる", state: "open" });
  await gh(page, { repo, number: "2", kind: "pr", title: "直す", state: "draft" });
  let dialog = await link(page, github, "PR", "org/*", "1");
  await expect(dialog.getByRole("alert")).toHaveText("リポジトリは owner/name の形式で入力してください");
  await dialog.getByRole("textbox", { name: "リポジトリ" }).fill(repo);
  await dialog.getByRole("textbox", { name: "番号" }).fill("x");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("番号を入力してください");
  await dialog.getByRole("textbox", { name: "番号" }).fill("99");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText(`${repo}#99 が見つかりません`);
  await dialog.getByRole("textbox", { name: "番号" }).fill("1");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText(`${repo}#1 は PR ではなく Issue です`);
  await dialog.getByRole("button", { name: "キャンセル" }).click();
  // 1 本目の Issue は付き、プロジェクトにリポジトリができたので 2 回目からは選ぶ
  dialog = await link(page, github, "Issue", repo, "1");
  await expect(dialog).toBeHidden();
  dialog = await link(page, github, "Issue", null, "2");
  await expect(dialog.getByRole("alert")).toHaveText(`${repo}#2 は Issue ではなく PR です`);
  await dialog.getByRole("textbox", { name: "番号" }).fill("1");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("もうリンクしています");
});

test("開いたときに取り直し、改名を追い、失敗を行に出す", async ({ page }, info) => {
  const { prefix, github, repo } = await setup(page, info);
  await gh(page, { repo, number: "5", kind: "pr", title: "A", state: "open" });
  await expect(await link(page, github, "PR", repo, "5")).toBeHidden();
  // GitHub でマージされ、リポジトリが改名された(旧い名前で引くと新しい名前が返り、新しい名前でも引ける)
  const moved = `${repo}2`;
  await gh(page, { repo, number: "5", kind: "pr", title: "A", state: "merged", moved });
  await gh(page, { repo: moved, number: "5", kind: "pr", title: "A", state: "merged" });
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const section = panel.getByRole("region", { name: "GitHub" });
  await expect(section.getByRole("link", { name: `${moved}#5 — A` })).toBeVisible();
  await expect(section.getByText("PR Merged")).toBeVisible();
  const history = panel.getByRole("region", { name: "履歴" });
  await expect(history.getByText(`リンクを ${repo}#5 から ${moved}#5 に更新`)).toBeVisible();
  await expect(history.getByText(`${moved}#5 の状態を Merged に変更`)).toBeVisible();
  // 取り直しに失敗した行には理由を添え、保存してある値のまま出す
  await gh(page, { repo: moved, number: "5", gone: "1" });
  await page.reload();
  await expect(section.getByRole("alert")).toHaveText(`状態を更新できませんでした: ${moved}#5 が見つかりません`);
  await expect(section.getByText("PR Merged")).toBeVisible();
});

test("アカウントが 1 つもなければ、リンクの追加を無効にする", async ({ page }, info) => {
  await page.route("**/api/accounts", (route) => route.fulfill({ json: [] }));
  const { github } = await setup(page, info);
  for (const name of ["PR をリンク", "Issue をリンク"]) {
    const button = github.getByRole("button", { name });
    await expect(button).toBeDisabled();
    await expect(button).toHaveAttribute("title", "GitHub のアカウントを登録すると使えます");
  }
  await expect(github.getByRole("button", { name: "PR 用の印をコピー" })).toBeEnabled();
});
```

`notes-readonly.spec.ts` に足す(既存の閲覧のみのテストと同じ開き方で):

```ts
test("閲覧のみの Store では、リンクの追加と解除を無効にし、印のコピーは押せる", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const github = page.getByRole("region", { name: "詳細パネル" }).getByRole("region", { name: "GitHub" });
  for (const name of ["PR をリンク", "Issue をリンク"]) {
    await expect(github.getByRole("button", { name })).toBeDisabled();
    await expect(github.getByRole("button", { name })).toHaveAttribute("title", "閲覧のみの Store です");
  }
  await expect(github.getByRole("button", { name: "PR 用の印をコピー" })).toBeEnabled();
});
```

- [ ] **Step 2: 流す**

Run(4173 を避ける。Global Constraints):
```bash
make e2e-bin
cd web && sed 's/4173/4273/g; s/reuseExistingServer: !process.env.CI/reuseExistingServer: false/' playwright.config.ts > playwright.d3.config.ts
pnpm exec playwright test -c playwright.d3.config.ts e2e/links.spec.ts e2e/notes-readonly.spec.ts e2e/accounts.spec.ts
```
Expected: PASS(3 幅)。落ちたら、まず `ps aux | grep hedgehug-e2e` で別の worktree のサーバーを使っていないかを確かめる。

- [ ] **Step 3: すべての e2e と確かめ**

Run:
```bash
cd web && pnpm exec playwright test -c playwright.d3.config.ts
rm playwright.d3.config.ts
cd .. && make check
```
Expected: すべて PASS。

- [ ] **Step 4: コミット**

```bash
git add web/e2e
git commit -m "test(e2e): PR・Issue のリンク・解除・断り・取り直し・閲覧のみ・アカウントなしを確かめる(D3 Task 17)"
```

---

## D3 のあと

- D4 は、この計画の `itemlink`(`Set.Attach` は自動リンクにも使える)、`port.GitHub.Lookup`、`port.Checks`、`Links.refreshOne`(間隔ごとの取り直し)を使う。自動化のきっかけ(D3-13)は D4 で、`app/items` の `automate` の入口を domain に移してから足す。手でリンクしたときのきっかけも D4 で足す(07:301「手でリンクしたときと同じに」)。
- GitHub App 未導入の見分け(D3-5 で「リポジトリを読めない」にしている GraphQL の `FORBIDDEN`)は D4。
- 本物のアプリでリンクできるのは、トラック G が本物の `StoreGitHub` を作ってから(`noStoreGitHub` は Store を返さない)。
- トラック G へ: 本物の `StoreGitHub` は `StoreGitHubInfo.ReadOnly` を埋める(閲覧のみの Store の取り直しを止めるため。D3-9)。
- アイテムの削除を作るトラックへ: Issue の 1 対 1 の検査(`findIssue`)が削除済みのアイテムも数えることを、受け入れ条件として確かめる。
