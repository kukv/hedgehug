# hedgehug トラック D4b(PR の自動リンクと GitHub App 未導入)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 07 の「PR の自動リンク」と「状態」の GitHub App 未導入を作る。GitHub を読む間隔の読み取りと、プロジェクトにリポジトリを足したときに PR を探してアイテムに自動でリンクし、探せなかった理由をプロジェクトの概要に出す。App 未導入の組織を見分けて記録し、設定の「アカウントとメンバー」に行の表示・帯・依頼文のコピー・「再確認」を出す。

**Architecture:** PR の文字列から ID と印を読む規則は新しい `domain/prlink` に置く。GitHub からの PR の一覧の読み取りは `infra/github` の `ReadPulls`(GraphQL、更新の新しい順に 100 件ずつ)。探した記録(初めてか・どこまで見たか・失敗の文言)と App 未導入の組織の記録は `infra/statedb` の新しい表に持つ。自動リンクの保存とその中のきっかけは、D4a の `app/githublink` の `ReadOnce` の同じ読み取りの中で、同じ `pass` を使って行う。App 未導入の記録と消去は、`app/githublink` の GitHub の呼び出しを 1 か所(`call`)に集めて、そこで行う。「再確認」は `app/github` に置く。

**Tech Stack:** Go 1.27.3、GitHub GraphQL API v4・REST(`/user/installations`)、modernc.org/sqlite(state.db)、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`)、e2e は Playwright。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md`「状態」(103〜125 行)、「削除の影響」154 行、「GitHub との通信に使うアカウント」、「GitHub との通信に失敗したとき」、「PR の自動リンク」(185〜217 行)、「GitHub を読む間隔」、「PR・Issue の状態で動く自動化」301 行、「アクティビティに残る文」
- 02「プロジェクトの概要」の「リポジトリ」(553 行。「PR を探せませんでした: {理由}」)
- 03「ワークスペースの設定」の ID の接頭辞(113 行。古い ID は読まない)
- 06「アクティビティ」の 524 行(「{ID} に {リポジトリ}#{番号} を自動でリンク」)、414 行(通知を作らない)
- 08「非表示」156 行
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` 319 行(自動リンクと `links_off`)、「このマシンに持つもの」(state.db)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-d.md`(一覧の D4b の行)、`2026-11-13-hedgehug-track-d4a.md`(読み取りの仕組み・`pass`・`fire`)、`2026-11-13-hedgehug-track-d2.md` の D2-4

## 始める前に

- トラック A4b(PR #177)は develop に入った(2026-11-14)。D4b は A4b の削除済みでない読み手 `port.LiveItem`・`port.LiveProjects`・`port.LiveItems` を使う(D4b-6)。
- A4b は D4a の計画の末尾「A4b が入ったときに足すもの」の 3 行のうち、`readItem` を削除済みで 404 にする行だけを入れた(A4b-4)。残りの 2 行(`fire` と `collectProject` の削除済みの判定)は、この計画の Task 0 で入れる。

## Global Constraints

土台とトラック D・D4a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/port`・`app/apptest` は 20 個で満杯なので、ファイルを足さない(口は `port/github.go`、偽物は `apptest/github.go` に足す。`apptest/github.go` が 300 行を超えるなら、PR の偽物を `apptest/github.go` から同じパッケージの小さいファイル `storegithub.go`(40 行)へ移す)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。テストのトークンは `tok-taro` のような値にする。
- GitHub への 1 回の問い合わせは 30 秒で打ち切る(D1 の `Client` の `timeout` のまま)。
- 文言は仕様の文のとおりに書き、言い換えない。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| D4b-1 | ID と印の読み方(`domain/prlink`)。ブランチ名は `(?i)^(?:hh|hedgehug)/([a-z][a-z0-9]{0,9}-[1-9][0-9]*)(?:-|$)` で、ID は大文字にする。タイトル・本文・コメントの ID は `[A-Z][A-Z0-9]{0,9}-[1-9][0-9]*` で、前後が英数字か `-` のものは読まない(大文字と小文字を区別する)。印は `(?:hedgehug|hh):([0-9A-Za-z]{26})` で、前後が英数字のものは読まない。内部の ID は大文字にする。本文に印が 1 つでもあれば、タイトルと本文の ID は読まない(タイトルの印は読む)。コメントは印と ID を読む。番号の先頭の 0(`WEB-01`)は ID としない | 07 の 201〜205 行。大文字と小文字を区別しないとあるのはブランチ名だけ。接頭辞の形は `workspace.ValidPrefix`。前回の実装の `found`(ハイフンを隣に持つものを除く)。前回は本文の最後の行の印だけを見たが、仕様は「本文に印があるとき」なので本文のどこでも |
| D4b-2 | ID の接頭辞は、その Store のいまのワークスペースの接頭辞だけで引く(旧い接頭辞は使わない)。番号は、そのワークスペースの削除済みでないプロジェクトの、削除済みでないアイテムから引く。プロジェクトの ID(`WEB-P1`)は D4b-1 の形に当たらない | 07 の 205・209 行、03 の 113 行 |
| D4b-3 | 探すリポジトリ: Store の削除済みでないプロジェクトの `repos` と、削除済みでないアイテムのリンク(状態を問わない。Merged も含む)のリポジトリ。小文字で重ねない。問い合わせる名前は最初に見つけた書き方。D4a の取り直しの集め方(`collect`)を広げ、取り直すリンクのないリポジトリも、探すために並べる。読む時期(`ghread.Due`)と利用上限の記録は、リポジトリごとの D4a の記録 1 つで、取り直しと探索の両方を決める | 07 の 191 行、195 行(「GitHub を読む間隔」の読み取りと同じ読み取りで) |
| D4b-4 | 探した記録は、取り直しの記録と別に、state.db の表 `github_scans`(Store・リポジトリの小文字・読んだか・どこまで見たか・失敗の文言)に持つ。「読んだか」は一度でも探し終えたら真にし、どこまで見たか(GitHub の `updatedAt` の最大)とは別に持つ(PR が 1 本もないリポジトリが、いつまでも「初めて」にならないように)。初めてなら、更新の新しい順に 100 件だけを見て、きっかけを起こさない。2 回目からは、`updatedAt` が記録の時刻以上の PR をページをめくって最後まで見る。失敗したら読んだか・どこまでは変えず、失敗の文言だけを置く。成功で文言を消す | 07 の 195・301 行、D4a-4。どこまで見たかは GitHub の時刻で持つ(このマシンの時計がずれていても漏れない) |
| D4b-5 | リンクの保存は、アイテムと PR の組ごとに 1 回(実行者「GitHub」、アクティビティ「{ID} に {リポジトリ}#{番号} を自動でリンク」)。保存の中でアイテムのリンクを読み直し、同じリポジトリ(大文字と小文字を区別しない)と番号のリンクが種別を問わずあるとき、`links_off` にあるときは書かない。Closed の PR は読む前に除く。リンクのリポジトリ名は GitHub の答えの `nameWithOwner`。値はタイトル・状態・レビューの状態、チェックの結果はこのマシンに置く(D3 の `saveChecks`) | 07 の 209〜213 行、保存形式 319 行。`Set.Attach` は `links_off` から消してしまうので、呼ぶ前に確かめる |
| D4b-6 | 削除済みのアイテムには付けず、削除済みのプロジェクトのリポジトリは探さない。読み手は A4b の `port.LiveProjects`・`port.LiveItems`。保存の中でも `port.LiveItem` で読み直し、その間に削除されていれば書かない | 07 の 191・209 行。ユーザーの決定(2026-11-14、実装は A4b のあと) |
| D4b-7 | きっかけは、D4a の `fire(tx, id, nil, link, p)` を同じ保存の中で呼ぶ。`p` は同じ読み取りの取り直しと共用し、同じアイテムの同じきっかけは 1 回だけ。そのリポジトリが初めて(D4b-4)なら呼ばない。プロジェクトにリポジトリを足したときのすぐの探索でも同じ | 07 の 301 行、D4a-14 |
| D4b-8 | 同じ読み取りの順: 1 つのリポジトリで、まずリンクを取り直し(`ReadMany`)、続けて PR を探す(`ReadPulls`)。Store のすべての読む時期が来たリポジトリを読み終えてから、取り直しを書き、そのあとで自動リンクを書く。取り直しで利用上限に断られたら、そのリポジトリは探さず、探した記録にも同じ文言を置く。探索で断られたら、D4a-6 と同じに、そのアカウントのほかのリポジトリを問い合わせない | D4a-6・14 |
| D4b-9 | プロジェクトにリポジトリを足したとき(概要の「リポジトリを追加」と、「PR をリンク」の「プロジェクトのリポジトリにも追加する」)は、保存のあとに、そのリポジトリだけを、間隔と読まない期限によらず探す(`Links.ScanRepo`)。呼ぶのは HTTP の口(`app/projects` と `app/githublink` は互いに import しない)。応答は待たせず裏で動かし、終わったら `EventGitHub` を流す(画面は知らせで概要を読み直す)。D4a の取り直しの記録(`github_reads`)は変えない | 07 の 195・278 行。app の機能どうしは import しない(設計書 2 節)。探索は 1 回で数十秒かかることがあり、追加の操作を待たせない |
| D4b-10 | 探す Store は、ReadOnce と同じく閲覧のみ・非表示・アカウントなしを除く。概要の「PR を探せませんでした: {理由}」は、Store のアカウントがなく、閲覧のみでも非表示でもないときは、どのリポジトリの行にも「読めるアカウントがない」の文言を出す(記録を置かずに、その場で作る)。閲覧のみの Store では出さない。それ以外は `github_scans` の失敗の文言を出す | 07 の 154・160・191 行、02 の 553 行 |
| D4b-11 | App 未導入の見分け(`infra/github`): (1) GraphQL の `FORBIDDEN` の文に `Resource not accessible by integration` があるとき、(2) 文に `OAuth App access restrictions` があるとき、(3) リポジトリを読めない(`NOT_FOUND`、`repository` が null、HTTP の 404)ときに `GET /user/installations` を読み、リポジトリの owner の導入がない、または導入が `selected` でそのリポジトリを含まないとき。(3) の一覧を読めない(App のトークンでない)ときは、リポジトリを読めないのまま。組織名はリポジトリの owner | 前回の実装(`ErrNotAccessible`・`AppMissing`)。GitHub は App が入っていない非公開のリポジトリを 404 で隠す。(2) は OAuth App の組織の制限(07 の 103 行)。文の言い回しは GitHub の docs「OAuth app access restrictions」の名前で、Task 2 で確かめ、テストの答えに固定する |
| D4b-12 | App 未導入の記録は state.db の表 `github_app_missing`(アカウント・組織の小文字・表示する組織名・断られたリポジトリ)。置くのは、`app/githublink` の GitHub の呼び出し(`Lookup`・`ReadMany`・`ReadPulls`)が `AppNotInstalled` を返したとき。消すのは、同じアカウントでその組織のリポジトリの呼び出しが成功したとき。どちらも `githublink.call` の 1 か所で行い、変わったら `EventGitHub` を流す | 07 の 113 行。コラボレーターの取得と Store の検索(D5・G)は、作るときに同じ口を使う |
| D4b-13 | 一覧の状態は、認証エラー > App 未導入 > 利用可能の順で 1 つ。HTTP の行の `status` に `app_missing` を足し、`orgs`(表示する組織名、記録の順)を足す。認証エラーのあいだは `orgs` を返さない(帯も出さない)が、記録は消さない | 07 の 105 行 |
| D4b-14 | 「再確認」は `POST /api/accounts/{id}/recheck`。記録の組織ごとに、記録したリポジトリを `GitHub.CheckRepo` で読み、読めたら記録を消す。App 未導入なら残す。答えは `{cleared: [組織名], still: [組織名]}`。トークンが拒否されたら認証エラーにして、行の赤字に認証エラーの文言。ほかの通信の失敗は、組織を残して行の赤字にその文言(07 の表)。画面は、cleared があればトースト「{表示名}: {組織名・組織名} を読めるようになりました」、still の帯に赤字「まだ読めません」。赤字は次にその行を操作するまで、または別のタブへ移るまで残す(いまの再認証の赤字と同じ) | 07 の 123・125 行 |
| D4b-15 | 帯は、アカウント一覧の上に、App 未導入の行の組織ごとに 1 本。文「{表示名}: {組織名} に GitHub App が入っていないため、PR と Issue を読めません。」とボタン「管理者にインストールを依頼」。押すと依頼文「{組織名} に GitHub App をインストールしてください(@{ログイン名} が使います)」をコピーし、トースト「コピーしました」。コピーできなければ、帯の中に「コピーできませんでした。文を選択してコピーしてください」と依頼文を出す(依頼文は選べる文字) | 07 の 115〜121 行。成功の表示は「PR 用の印をコピー」と同じトースト |
| D4b-16 | e2e の入口: `POST /e2e/github-pull?repo&number&branch&title&body&state&comment&updated`(偽の PR を置く)、`POST /e2e/github-app?owner&missing=1|0`(その owner のリポジトリを App 未導入にする・戻す)、`POST /e2e/github-repo?repo&unreadable=1|0`(リポジトリ全体を読めなくする・戻す。`Fail(repo, 0, RepoUnreadable)`)。読み取りは D4a の `/e2e/github-read` | D4a-19 と同じ形 |

## Review Focus

1. **初めての読み取りできっかけが動く。** 初めて探すリポジトリで、Merged の PR を自動でリンクしても「リンクした PR がマージされたとき」のルールが動かない。2 回目からは動く(D4b-4・7)。Task 7 のテストに入れる。
2. **解除した PR が戻る。** 解除の記録(`links_off`)にある PR は、ブランチ名に ID があっても付かない。手でリンクし直したあとは、もう一度解除しない限り付いたまま(D4b-5)。Task 7 のテストに入れる。
3. **ハイフンに隣り合う ID と旧い接頭辞。** 「PRE-WEB-7」から WEB-7 を取らず、接頭辞を変える前の ID は読まない(D4b-1・2)。Task 1・Task 6 のテストに入れる。
4. **App 未導入が消えない・残らない。** その組織のリポジトリを同じアカウントで読めたら記録が消え、帯と行の組織名が消える。別のアカウントで読めても消えない(D4b-12)。Task 5 のテストに入れる。
5. **認証エラーと App 未導入が重なる。** 認証エラーのあいだは帯を出さず、再認証で戻ると帯がまた出る(D4b-13)。Task 10 のテストに入れる。

---

## ファイルの地図

```
internal/domain/prlink/prlink.go              Text・Mentions・Read(D4b-1)
internal/domain/prlink/prlink_test.go
internal/domain/itemlink/text.go              (変更)AutoSummary
internal/app/port/github.go                   (変更)Pull・Pulls・GitHub.ReadPulls・CheckRepo・PRScans・AppMissing
internal/app/apptest/github.go                (変更)PR と App 未導入の偽物
internal/infra/github/pulls.go                ReadPulls
internal/infra/github/appmissing.go           App 未導入の見分け(D4b-11)・CheckRepo
internal/infra/github/lookup.go・read_many.go (変更)見分けを通す
internal/infra/statedb/scans.go               github_scans・github_app_missing
internal/app/githublink/call.go               GitHub の呼び出しの 1 か所(D4b-12)
internal/app/githublink/autolink_find.go      Store の中で ID・印からアイテムを引く
internal/app/githublink/autolink.go           自動リンクの保存
internal/app/githublink/read.go・read_collect.go (変更)探索を同じ読み取りに入れる
internal/app/githublink/scan.go               ScanRepo・RepoFailures
internal/app/github/app_missing.go            AccountState の orgs・RecheckOrgs
internal/app/github/list_accounts.go          (変更)状態の順
internal/adapter/httpapi/github/accounts.go   (変更)status・orgs・recheck
internal/adapter/httpapi/github/links.go      (変更)リポジトリの失敗の文言・リンクのあとの探索
internal/adapter/httpapi/projects/edit.go     (変更)リポジトリを足したあとの探索
cmd/hedgehug/links.go・wire.go・wire_e2e.go・links_e2e.go (変更)組み立てと e2e の入口
web/packages/core/src/model/account.ts・features/accounts/(変更)app_missing・orgs・再確認・依頼文
web/app/src/screens/settings/AccountItem.svelte・AccountsSection.svelte・AppMissingBand.svelte
web/packages/core/src/features/github/(変更)リポジトリの失敗の文言
web/app/src/screens/project/OverviewRepos.svelte (変更)「PR を探せませんでした: 」
web/e2e/autolink.spec.ts・accounts.spec.ts   e2e
```

---

### Task 0: D4a の削除済みの判定を入れる(D4a の計画の末尾「A4b が入ったときに足すもの」)

**Files:**
- Modify: `internal/app/githublink/fire.go:66-80`、`internal/app/githublink/read_collect.go:60-97`
- Test: `internal/app/githublink/fire_test.go`、`internal/app/githublink/read_test.go`(いまのテストのファイルに足す)

**Interfaces:**
- Consumes: `port.LiveItem`・`port.LiveProject`・`port.LiveProjects`・`port.LiveItems`(A4b)
- Produces: `fire` は、アイテムかそのプロジェクトが削除済みなら、ルールを動かさずに `nil, nil` を返す(リンクの値の保存は呼び出し側のまま)。`collect` は削除済みのプロジェクトとアイテムのリンクを集めない。

- [ ] **Step 1: 失敗するテストを書く**

- `TestFireSkipsDeletedItem`: 「リンクした PR がマージされたとき → ステータスを Done」のルール。Open の PR をリンクしたアイテムを削除し(A4b の削除)、ReadOnce の前に PR を Merged にする → ReadOnce のあとも、削除済みのアイテムのリンクの値は変わらず(取り直さない)、ステータスも変わらない。`fire` を直接呼ぶテスト(保存の中で削除済みのアイテムに対して呼ぶ)では、自動化の行が書かれず、エラーもない(03「削除済みのアイテムには動かない」)。
- `TestReadOnceSkipsDeletedItems`: 削除済みのアイテムだけがリンクしている番号は `ReadMany` に頼まず(`ReadManyCalls == 0`)、文言も置かない。削除済みのプロジェクトのアイテムも同じ。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run 'TestFireSkipsDeletedItem|TestReadOnceSkipsDeletedItems'`
Expected: FAIL

- [ ] **Step 3: 直す**

`fire` は `tx.Items().Get` の代わりに `port.LiveItem` と、そのアイテムのプロジェクトの `port.LiveProject` で読み、`port.ErrNotFound` なら `nil, nil`。`collect` は `port.LiveProjects`、`collectProject` は `port.LiveItems`。「A4b が足す」のコメントを消す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink
git commit -m "fix: 削除済みのアイテムでリンクのきっかけを動かさず、取り直しも頼まない(D4b Task 0)"
```

---

### Task 1: `domain/prlink`(PR の文字列から ID と印を読む)

**Files:**
- Create: `internal/domain/prlink/prlink.go`
- Test: `internal/domain/prlink/prlink_test.go`

**Interfaces:**
- Produces:
  ```go
  // Text は、PR の中で ID と印を探す文字列(07「見る場所と形」)。Comments は古い順に 100 件まで。
  type Text struct {
      Branch, Title, Body string
      Comments            []string
  }
  // Mentions は、PR が指すもの。IDs は「WEB-1」の形(大文字)、Marks は内部の ID(大文字)。見つけた順で重なりなし。
  type Mentions struct {
      IDs   []string
      Marks []string
  }
  func Read(t Text) Mentions
  // SplitID は「WEB-1」を接頭辞と番号に分ける。
  func SplitID(id string) (prefix string, number int, ok bool)
  ```

- [ ] **Step 1: 失敗するテストを書く**

`TestRead` を表で書く(`ulid` は 26 文字の `01JF0000000000000000000001` のような値)。

| 入力 | IDs | Marks |
|---|---|---|
| Branch `hh/WEB-1-login` | [WEB-1] | [] |
| Branch `hedgehug/web-12` | [WEB-12] | [] |
| Branch `HH/WEB-1` | [WEB-1] | [] |
| Branch `hh/WEB-1x` | [] | [] |
| Branch `feature/WEB-1` | [] | [] |
| Title `WEB-3 を直す` | [WEB-3] | [] |
| Title `PRE-WEB-7` | [] | [] |
| Title `WEB-7-2` | [] | [] |
| Title `web-3` | [] | [] |
| Title `WEB-P1` | [] | [] |
| Title `WEB-01` | [] | [] |
| Title `(WEB-4)、WEB-5。` | [WEB-4, WEB-5] | [] |
| Body `hedgehug:` + ulid(小文字) | [] | [ulid 大文字] |
| Body `hh:` + ulid + `\nWEB-9`、Title `WEB-8` | [] | [ulid] |
| Title `hh:` + ulid、Body `WEB-9` | [WEB-9] | [ulid] |
| Body `xhh:` + ulid | [] | [] |
| Body `WEB-2`、Comments [`WEB-3`、`hh:` + ulid] | [WEB-2, WEB-3] | [ulid] |
| Body `hh:` + ulid、Comments [`WEB-3`] | [WEB-3] | [ulid] |
| Branch `hh/WEB-1`、Title `WEB-1` | [WEB-1] | [] |

`TestSplitID`: `WEB-12` → (WEB, 12, true)、`WEB` → false、`WEB-0` → false、`A1-3` → (A1, 3, true)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/prlink/`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: `prlink.go` を書く**

パッケージの説明に 07「見る場所と形」と D4b-1 を書く。RE2 には後読みがないので、前回の実装の `found` と同じに、当たった位置の前後の 1 文字を見て除く(ID は英数字か `-`、印は英数字)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/prlink/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/prlink
git commit -m "feat(domain): PR のブランチ名・タイトル・本文・コメントから ID と印を読む(D4b Task 1)"
```

---

### Task 2: App 未導入の見分けと `CheckRepo`(`infra/github`)

**Files:**
- Create: `internal/infra/github/appmissing.go`
- Modify: `internal/infra/github/lookup.go`(`answerFailure` に文を渡す、`Lookup` の最後で見分ける)、`internal/infra/github/read_many.go`(同じ)
- Modify: `internal/app/port/github.go`(`GitHub.CheckRepo`)、`internal/app/apptest/github.go`(`CheckRepo`・`MissApp`)
- Test: `internal/infra/github/appmissing_test.go`

**Interfaces:**
- Consumes: `ghcall.Failure{Kind: ghcall.AppNotInstalled, Org, Repo}`(文言は `ghcall` にある)
- Produces:
  ```go
  // port.GitHub に足す。
  // CheckRepo は、repo を読めるかを確かめる(「再確認」)。読めれば nil。失敗は Lookup と同じ種類。
  CheckRepo(ctx context.Context, token, repo string) error
  // infra/github
  // explain は、リポジトリを読めない失敗 f を、App 未導入なら AppNotInstalled に置き換える(D4b-11 の (3))。
  func (c *Client) explain(ctx context.Context, token, repo string, err error) error
  // apptest.GitHub に足す。MissApp(owner, true) で、その owner のリポジトリの問い合わせ(Lookup・ReadMany・ReadPulls・
  // CheckRepo)が AppNotInstalled を返す。false で戻す。
  func (g *GitHub) MissApp(owner string, missing bool)
  ```

- [ ] **Step 1: GitHub の答えを確かめる**

OAuth App の組織の制限で GitHub が返す文を、GitHub の docs(「OAuth app access restrictions」の節と GraphQL の errors)で確かめ、テストの答えに入れる文を決める。確かめた URL を `appmissing.go` のコメントに書く。分からなければ、`OAuth App access restrictions` を含むことだけを条件にする(D4b-11)。

- [ ] **Step 2: 失敗するテストを書く**(`httptest` の偽の GitHub。いまの `lookup_test.go` の組み立てを使う)

- `TestLookupAppNotAccessible`: GraphQL が `{"errors":[{"type":"FORBIDDEN","path":["repository"],"message":"Resource not accessible by integration"}]}` → `ghcall.Is(err, AppNotInstalled)`、`Org == "example-org"`、`Repo == "example-org/web"`。
- `TestLookupOAuthRestricted`: `FORBIDDEN` で文に `OAuth App access restrictions` → AppNotInstalled。
- `TestLookupHiddenByMissingApp`: `NOT_FOUND`(path 1)、`/repos/example-org/web` は 404(改名でない)、`/user/installations` は `{"installations":[{"id":1,"account":{"login":"other"},"repository_selection":"all"}]}` → AppNotInstalled。
- `TestLookupSelectedWithoutRepo`: 導入が `example-org` で `selected`、`/user/installations/1/repositories` に `web` がない → AppNotInstalled。`web` があれば RepoUnreadable。
- `TestLookupNotAppToken`: `/user/installations` が 403 → RepoUnreadable のまま。
- `TestReadManyAppNotInstalled`: `ReadMany` でも同じ 3 つの見分けが効く(表で)。
- `TestCheckRepo`: `repository { id }` が返れば nil、`NOT_FOUND` かつ導入あり → RepoUnreadable、401 → TokenRejected。

- [ ] **Step 3: テストが落ちることを確かめる**

Run: `go test ./internal/infra/github/`
Expected: FAIL

- [ ] **Step 4: 見分けを書く**

`answerFailure`・`readManyAnswer.failure` は、`FORBIDDEN` の文が (1)(2) に当たれば `AppNotInstalled` を返す(GraphQL の errors の要素に `Message` を足す)。`Lookup`・`ReadMany`・`CheckRepo` は、最後に RepoUnreadable が残ったときだけ `explain` を通す(改名の確かめのあと)。`/user/installations` は 100 件ずつめくる。`CheckRepo` の問い合わせは `query($owner,$name){ rateLimit{limit remaining resetAt} repository(owner:$owner,name:$name){ id } }`。

- [ ] **Step 5: 偽物を足し、テストが通ることを確かめる**

Run: `go test ./internal/infra/github/ ./internal/app/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/infra/github internal/app/port/github.go internal/app/apptest/github.go
git commit -m "feat(github): GitHub App 未導入の組織を見分け、リポジトリを読めるか確かめる(D4b Task 2)"
```

---

### Task 3: PR の一覧を読む(`ReadPulls`)

**Files:**
- Create: `internal/infra/github/pulls.go`
- Modify: `internal/app/port/github.go`、`internal/app/apptest/github.go`(`PutPull`・`ReadPulls`・`ReadPullsCalls`)
- Test: `internal/infra/github/pulls_test.go`、`internal/app/apptest/github_test.go`

**Interfaces:**
- Produces:
  ```go
  // Pull は、自動リンクのために読んだ PR(07「PR の自動リンク」)。
  type Pull struct {
      Number    int
      prlink.Text                 // ブランチ名・タイトル・本文・コメント(古い順に 100 件まで)
      itemlink.Values             // タイトル・状態・レビューの状態(Lookup と同じ読み方)
      Checks    itemlink.Checks
      UpdatedAt time.Time         // GitHub の時刻
  }
  // Pulls は ReadPulls の答え。Repo は GitHub の nameWithOwner。
  type Pulls struct {
      Repo  string
      Pulls []Pull
      Rate  Rate
  }
  // port.GitHub に足す。
  // ReadPulls は repo の PR を更新の新しい順に読む。since がゼロなら最初の 100 件だけ、そうでなければ
  // UpdatedAt が since 以上のものを最後まで(100 件ずつめくる)。改名・移管は Lookup と同じに追う。
  // 失敗は Lookup と同じ種類(App 未導入の見分けを含む)。
  ReadPulls(ctx context.Context, token, repo string, since time.Time) (Pulls, error)
  ```

- [ ] **Step 1: 失敗するテストを書く**

- `TestReadPullsFirst`: since がゼロ。偽の GitHub が 1 ページ目に 100 件と `hasNextPage: true` を返しても、問い合わせは 1 回で 100 件。問い合わせの変数に `first: 100`、`orderBy: {field: UPDATED_AT, direction: DESC}` があること。`headRefName`・`body`・`comments(first: 100)` の本文が `Text` に入り、状態・レビューは `Lookup` と同じに読まれる(Draft・Merged・レビュー待ちの 3 件で確かめる)。
- `TestReadPullsSince`: 1 ページ目の最後の `updatedAt` が since より新しく、2 ページ目の途中に since より古いものがある → 2 回問い合わせ、since 以上のものだけを返す(since ちょうどを含む)。
- `TestReadPullsRenamed`: 1 回目が `NOT_FOUND`、`/repos/old/web` が `full_name: new/web` → `Pulls.Repo == "new/web"`。
- `TestReadPullsRate`: `rateLimit` を `Pulls.Rate` に入れる(最後のページの値)。

apptest の `TestGitHubReadPulls`: `PutPull("o/web", Pull{Number: 3, UpdatedAt: t1})` と t2 の 2 件で、since ゼロなら新しい順に 2 件、since = t2 なら 1 件。`Fail("o/web", 0, err)` で失敗する。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/github/ ./internal/app/apptest/`
Expected: FAIL

- [ ] **Step 3: `pulls.go` と偽物を書く**

問い合わせは `lookup.go` の `nodeFragment` を使い、`headRefName body updatedAt comments(first: 100) { nodes { body } }` を足す。ページは `pageInfo { hasNextPage endCursor }`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/github/ ./internal/app/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infra/github internal/app/port/github.go internal/app/apptest
git commit -m "feat(github): 自動リンクのために PR を更新の新しい順に読む(D4b Task 3)"
```

---

### Task 4: 探した記録と App 未導入の記録(state.db)

**Files:**
- Create: `internal/infra/statedb/scans.go`
- Modify: `internal/app/port/github.go`
- Test: `internal/infra/statedb/scans_test.go`

**Interfaces:**
- Produces:
  ```go
  // ScanRecord は、Store のリポジトリ 1 つの、PR を探した記録(D4b-4)。ゼロ値は「まだ探していない」。
  type ScanRecord struct {
      Read    bool      // 一度でも探し終えた
      Since   time.Time // 見た PR の updatedAt の最大(GitHub の時刻)
      Failure string    // 前回の失敗の文言(07 の表の文。空なら成功)
  }
  type PRScans interface {
      Scans(ctx context.Context, store StoreID) (map[string]ScanRecord, error) // キーはリポジトリの小文字
      SaveScan(ctx context.Context, store StoreID, repo string, r ScanRecord) (changed bool, err error)
  }
  // MissingOrg は、App 未導入の組織の記録(D4b-12)。
  type MissingOrg struct {
      Org  string // 表示する組織名(最初に断られたリポジトリの owner の書き方)
      Repo string // 断られたリポジトリ(「再確認」で読む)
  }
  type AppMissing interface {
      List(ctx context.Context) (map[account.ID][]MissingOrg, error) // 記録した順
      Put(ctx context.Context, a account.ID, m MissingOrg) (changed bool, err error) // 同じ組織(小文字)なら Repo だけ置き換え、changed は false
      Clear(ctx context.Context, a account.ID, org string) (changed bool, err error)  // org は大文字と小文字を区別しない
  }
  // statedb
  func (d *DB) PRScans() port.PRScans
  func (d *DB) AppMissing() port.AppMissing
  ```

- [ ] **Step 1: 失敗するテストを書く**

`TestPRScans`: 保存して読み直すと同じ値(時刻は UTC で比べる)。同じ値の保存は changed が false、文言が変われば true。Store ごとに分かれる。`TestAppMissing`: Put → List に出る、`Example-Org` と `example-org` は 1 件、Clear で消えて changed、ないものの Clear は false。アカウントごとに分かれ、記録した順で返る。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/statedb/`
Expected: FAIL

- [ ] **Step 3: 表を足す**

`register("github/004", ...)` で `github_scans(store, repo, read, since, failure, PRIMARY KEY(store, repo))`、`register("github/005", ...)` で `github_app_missing(account, org_key, org, repo, seq, PRIMARY KEY(account, org_key))`。時刻は `timeText`・`parseTime`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/statedb/ && go build ./... && go build -tags e2e ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infra/statedb internal/app/port/github.go
git commit -m "feat(statedb): PR を探した記録と GitHub App 未導入の組織を持つ(D4b Task 4)"
```

---

### Task 5: GitHub の呼び出しを 1 か所にし、App 未導入を記録する(`githublink.call`)

**Files:**
- Create: `internal/app/githublink/call.go`
- Modify: `internal/app/githublink/links.go`(`Links` に `Scans port.PRScans`・`Missing port.AppMissing`、`lookup` を `call` に)、`read.go`(`readMany` を `call` に)
- Modify: `internal/app/githublink/reads_fake_test.go`(`memScans`・`memMissing`)
- Modify: `cmd/hedgehug/links.go`、`internal/adapter/httpapi` の `Deps`(`PRScans`・`AppMissing`)、`cmd/hedgehug/wire.go`・`wire_e2e.go`(`e2eReads` が DB を返すようにする)
- Test: `internal/app/githublink/call_test.go`

**Interfaces:**
- Consumes: `port.GitHubTokens.TokenFor`、`port.AppMissing`(Task 4)
- Produces:
  ```go
  // call は、Store のアカウントで repo について GitHub に問い合わせる 1 か所(D4b-12)。トークンが拒否されたら
  // 認証エラーにする(D4a の lookup・readMany と同じ)。AppNotInstalled なら記録を置き、成功なら repo の owner の
  // 記録を消す。記録が変わったら EventGitHub(Store は空)を流す。
  func (l Links) call(ctx context.Context, store port.StoreID, repo string, f func(token string) error) error
  ```

- [ ] **Step 1: 失敗するテストを書く**

- `TestCallRecordsAppMissing`: 偽の GitHub に `MissApp("example-org", true)`、`Links.Refresh` で `example-org/web` のリンクを取り直す → `Missing.List` にアカウントの `{Org: "example-org", Repo: "example-org/web"}`、行の文言は「状態を更新できませんでした: example-org に GitHub App が入っていないため、example-org/web を読めません」、`EventGitHub` が流れる。
- `TestCallClearsOnSuccess`: 記録がある状態で `MissApp(..., false)` にして取り直す → 記録が消え、`EventGitHub`。
- `TestCallOtherAccountDoesNotClear`: 別の Store(別のアカウント)で同じ組織を読めても、最初のアカウントの記録は残る。
- `TestCallOtherOwnerDoesNotClear`: 同じアカウントで別の owner のリポジトリを読めても残る。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run TestCall`
Expected: FAIL

- [ ] **Step 3: `call.go` を書き、`lookup`・`readMany` を `call` で書き直す**

既存のテストがそのまま通ること。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go build ./... && go build -tags e2e ./... && go test ./internal/app/... ./internal/adapter/... ./cmd/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink internal/adapter cmd
git commit -m "feat: GitHub の呼び出しを 1 か所にし、App 未導入の組織を記録・消去する(D4b Task 5)"
```

---

### Task 6: Store の中で ID と印からアイテムを引く

**Files:**
- Create: `internal/app/githublink/autolink_find.go`
- Test: `internal/app/githublink/autolink_find_test.go`

**Interfaces:**
- Consumes: `prlink.Read`・`prlink.SplitID`(Task 1)、`port.LiveProjects`・`port.LiveItems`(A4b)
- Produces:
  ```go
  // storeItems は、Store の削除済みでないアイテムを、ID(いまの接頭辞と番号)と内部の ID から引く表(D4b-2・6)。
  type storeItems struct {
      byID   map[string]item.ID // 「WEB-1」→ 内部の ID
      live   map[item.ID]string // 内部の ID → 表示の ID
  }
  func readStoreItems(r port.Reader) (storeItems, error)
  // targets は、PR p が指すアイテム(重なりなし。見つけた順)。
  func (s storeItems) targets(p port.Pull) []item.ID
  ```

- [ ] **Step 1: 失敗するテストを書く**

apptest の Store に、ワークスペース WEB(アイテム WEB-1・WEB-2)と API(API-1)を作る。

- ブランチ `hh/WEB-1`、タイトル `API-1` → [WEB-1 の ID, API-1 の ID]。
- 本文に WEB-2 の印 → [WEB-2]。
- 本文に存在しない ULID の印 → []。
- `WEB-9`(ない番号)→ []。
- ワークスペース WEB の接頭辞を WEBX に変えたあと、`WEB-1` → []、`WEBX-1` → [WEB-1 の ID](旧い ID は読まない)。
- WEB-2 を削除したあと(A4b の削除)、`WEB-2` と WEB-2 の印 → []。削除済みのプロジェクトのアイテムも [](D4b-6)。
- 同じアイテムを ID と印の両方で指す → 1 件。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run TestStoreItems`
Expected: FAIL

- [ ] **Step 3: `autolink_find.go` を書く**

ワークスペースの一覧 → `LiveProjects` → `LiveItems`。表示の ID は `it.DisplayID(ws.Prefix)`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink
git commit -m "feat: PR が指す ID と印から、Store の削除済みでないアイテムを引く(D4b Task 6)"
```

---

### Task 7: 自動リンクの保存ときっかけ

**Files:**
- Create: `internal/app/githublink/autolink.go`
- Modify: `internal/domain/itemlink/text.go`(`AutoSummary`)、`internal/domain/itemlink/text_test.go`
- Test: `internal/app/githublink/autolink_test.go`

**Interfaces:**
- Consumes: `fire`・`pass`・`l.notify`・`l.saveChecks`(D4a)、`storeItems`(Task 6)
- Produces:
  ```go
  // AutoSummary は「{ID} に {リポジトリ}#{番号} を自動でリンク」(06 の 524 行)。
  func AutoSummary(id string, r Ref) string
  // autoLink は、PR p を、Store の items が指すアイテムに自動でリンクする(D4b-5〜7)。Closed の PR は何もしない。
  // アイテムごとに 1 回の保存(実行者「GitHub」)。first なら(そのリポジトリを初めて探した)きっかけを起こさない。
  // p が nil でなければ、同じ読み取りのきっかけの重なりを除く。リンクを書いたら true。
  func (l Links) autoLink(ctx context.Context, store port.StoreID, repo string, pull port.Pull, items storeItems, first bool, p *pass) (bool, error)
  ```

- [ ] **Step 1: 失敗するテストを書く**

- `TestAutoLinkAdds`: ブランチ `hh/WEB-1` の Open の PR → WEB-1 にリンク(種別 PR、リポジトリは `Pulls.Repo`、タイトル・状態・レビュー)。操作の実行者は `ActorGitHub`、要約は「WEB-1 に example-org/web#5 を自動でリンク」。チェックの結果が置かれる。
- `TestAutoLinkSkipsClosed`: Closed の PR → 何も保存しない。
- `TestAutoLinkSkipsExisting`: WEB-1 に `Example-Org/Web#5` の Issue のリンクがある → 保存しない(大文字と小文字・種別を問わない)。
- `TestAutoLinkSkipsUnlinked`: WEB-1 で PR #5 を解除したあと(`Links.Remove`)→ 保存しない。手で「PR をリンク」し直し、もう一度解除せずに読むと、そのまま 1 本(重ねない)。
- `TestAutoLinkFiresAfterFirst`: 「リンクした PR がマージされたとき → ステータスを Done」のルール。first=true で Merged の PR → リンクは付き、ステータスは変わらない。first=false → Done になり、自動化の行は同じ保存に入り、通知が受信箱に入る。
- `TestAutoLinkOncePerPass`: 同じ pass で、WEB-1 を指す Open の PR 2 本 → 「PR がリンクされて Open のとき」のルールは 1 回だけ。
- `TestAutoLinkManyItems`: 1 本の PR が WEB-1 と WEB-2 を指す → 2 回の保存、両方にリンク。
- `TestAutoLinkDeletedMeanwhile`: 読んだあと、保存の前に WEB-1 を削除 → 書かない。
- `TestAutoLinkNoWatchNotice`: WEB-1 をウォッチしている人がいても、ウォッチの通知を作らない(06 の 414 行)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/itemlink/ ./internal/app/githublink/ -run 'TestAutoSummary|TestAutoLink'`
Expected: FAIL

- [ ] **Step 3: `autolink.go` を書く**

保存の中で `port.LiveItem` と `readItem` で読み直し、`links.Find` を大文字と小文字・種別を問わずに見る(`itemlink.Ref.Same` を種別を外して使う小さな関数)、`Off` を見る、`Attach` を書く、first でなければ `fire(tx, id, nil, link, p)`。保存のあと `notify`・`saveChecks`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/itemlink/ ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/itemlink internal/app/githublink
git commit -m "feat: 見つけた PR をアイテムに自動でリンクし、2 回目からきっかけを起こす(D4b Task 7)"
```

---

### Task 8: 間隔の読み取りで PR を探す

**Files:**
- Modify: `internal/app/githublink/read.go`、`read_collect.go`
- Create: `internal/app/githublink/read_scan.go`(読み取りの中の探索。`read.go` を 300 行に収めるため)
- Test: `internal/app/githublink/read_scan_test.go`

**Interfaces:**
- Consumes: `call`(Task 5)、`autoLink`(Task 7)、`ReadPulls`(Task 3)、`PRScans`(Task 4)
- Produces:
  - `repoLinks` に `scan bool`(探すリポジトリか)を足す。`collect` は、取り直すリンクのないリポジトリ(Merged だけ・プロジェクトのリポジトリだけ)も `scan` だけの要素として並べる(D4b-3)。
  - `repoResult` に `pulls port.Pulls`・`scanText string` を足す。
  - `func (rd *reading) scan(ctx context.Context, s port.StoreGitHubInfo, r *repoLinks, rec port.ScanRecord) (port.Pulls, string, error)`

- [ ] **Step 1: 失敗するテストを書く**

- `TestReadOnceScansProjectRepo`: プロジェクトに `example-org/web` があり、リンクは 0 本。PR #5(`hh/WEB-1`)→ ReadOnce で WEB-1 にリンク。記録は `Read: true`、`Since` は PR の `UpdatedAt`。`ReadMany` は呼ばれない(取り直す番号がない)。
- `TestReadOnceScansLinkedRepoOfMergedLinks`: プロジェクトにリポジトリがなく、WEB-2 が Merged の `other/api#1` をリンク → `other/api` を探す。
- `TestReadOnceSecondScanFires`: 1 回目のあと、Merged の PR #6(`hh/WEB-1`、UpdatedAt が後)を足して間隔を進める → 2 回目で付き、マージのルールが動く。since より古い PR は付かない(偽物が返さない)。
- `TestReadOnceScanFailure`: `Fail("example-org/web", 0, RepoUnreadable)` → 記録の `Failure` が「example-org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください」、`Read` は false のまま。次の成功で文言が消える。
- `TestReadOnceScanRateLimited`: `ReadPulls` が利用上限 → 同じアカウントのほかのリポジトリを問い合わせず、それらの記録にも同じ文言。D4a の読み取りの記録の読まない期限も置かれる。
- `TestReadOnceRefreshThenScanOnePass`: Open の PR #5 を WEB-1 が手でリンク済みで、取り直しで Merged、同じ読み取りの探索で PR #6(Merged、`hh/WEB-1`)を見つける → マージのルールは 1 回(pass の共用。D4b-7)。
- `TestReadOnceSkipsDeletedProjectRepo`: 削除済みのプロジェクトだけが持つリポジトリは探さない。
- `TestReadOnceNotDueNoScan`: 間隔の途中なら探さない(`ReadPullsCalls == 0`)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ -run TestReadOnce`
Expected: 新しいテストが FAIL

- [ ] **Step 3: 読み取りに探索を入れる**

`readStore` で `PRScans.Scans` を読み、`fetch` のあとに `r.scan` なら `scan` を呼ぶ(`fetch` が利用上限なら呼ばずに同じ文言)。書く順は D4b-8: `writeResults` のあと、結果ごとに `readStoreItems` を 1 回読み、PR ごとに `autoLink`(first は記録の `Read` が false)。記録は `SaveScan`(成功: `Read: true`、`Since` は今回見た `UpdatedAt` の最大と前の値の大きいほう、`Failure: ""`)。記録の文言が変わったら `EventGitHub`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/githublink/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/githublink
git commit -m "feat: GitHub を読む間隔の読み取りで PR を探して自動でリンクする(D4b Task 8)"
```

---

### Task 9: リポジトリを足したときの探索と、概要の失敗の文言(サーバー)

**Files:**
- Create: `internal/app/githublink/scan.go`
- Modify: `internal/adapter/httpapi/github/links.go`(リポジトリの失敗の文言の口、「PR をリンク」のあとの探索)、`internal/adapter/httpapi/projects/edit.go:102-108`(「リポジトリを追加」のあとの探索)
- Test: `internal/app/githublink/scan_test.go`、`internal/adapter/httpapi/github/links_test.go`、`internal/adapter/httpapi/projects/edit_test.go`

**Interfaces:**
- Produces:
  ```go
  // ScanRepo は、Store の repo を、間隔と読まない期限によらず 1 回探す(D4b-9)。探さない Store(D4b-10)では何もしない。
  // 取り直しの記録は変えない。終わったら EventGitHub を流す。
  func (l Links) ScanRepo(ctx context.Context, store port.StoreID, repo string) error
  // RepoFailures は、プロジェクトのリポジトリごとの「PR を探せませんでした: 」に続ける文言(D4b-10)。キーはリポジトリの書き方のまま。
  func (l Links) RepoFailures(ctx context.Context, store port.StoreID, id project.ID) (map[string]string, error)
  ```
  - HTTP: `GET /api/stores/{store}/projects/{project}/repo-failures` → `{failures: [{repo, text}]}`(operationId `list-repo-failures`)。
  - 「リポジトリを追加」(`add-project-repo`)と「PR をリンク」(`add-link` で `addToProject` が真)の成功のあと、口の層で `go deps.Links.ScanRepo(context.WithoutCancel(ctx), store, repo)` を呼ぶ。失敗はログに出す(応答は変えない)。

- [ ] **Step 1: 失敗するテストを書く**

- `TestScanRepoFirstNoTrigger`: マージのルールがあり、リポジトリを初めて探す → リンクは付き、ルールは動かない。
- `TestScanRepoIgnoresInterval`: 直前に ReadOnce で読み、読まない期限も未来 → それでも探す。`Reads.Records` は変わらない。
- `TestScanRepoSkipsReadOnly`: 閲覧のみの Store → 問い合わせない。
- `TestRepoFailures`: 記録の文言がそのリポジトリに出る。アカウントのない Store では、どのリポジトリにも「example-org/web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください」。閲覧のみの Store では空。
- HTTP: `TestAddRepoStartsScan`(偽の `Links` の代わりに apptest の GitHub の `ReadPullsCalls` が 1 になるのを待つ)、`TestListRepoFailures`(JSON の形)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/githublink/ ./internal/adapter/...`
Expected: FAIL

- [ ] **Step 3: `scan.go` と口を書く**

`ScanRepo` は Task 8 の `scan` と書き方の部分を使う(pass は新しく作る)。OpenAPI と `web/packages/api` の生成物を `make generate` で更新する。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `make generate && go test ./internal/app/... ./internal/adapter/...`
Expected: PASS(`web/packages/api` の生成物の差分はコミットに含める)

- [ ] **Step 5: Commit**

```bash
git add internal web/packages/api
git commit -m "feat: リポジトリを足したらすぐ PR を探し、探せなかった理由を返す(D4b Task 9)"
```

---

### Task 10: アカウントの App 未導入の状態と「再確認」(サーバー)

**Files:**
- Create: `internal/app/github/app_missing.go`
- Modify: `internal/app/github/list_accounts.go`(`AccountState.Orgs`、`ListAccounts.Missing`)
- Modify: `internal/adapter/httpapi/github/accounts.go`(`status` に `app_missing`、`orgs`、`POST /api/accounts/{id}/recheck`)
- Modify: `cmd/hedgehug/wire.go`・`wire_e2e.go`
- Test: `internal/app/github/app_missing_test.go`、`internal/adapter/httpapi/github/accounts_test.go`

**Interfaces:**
- Produces:
  ```go
  // AccountState に足す。Orgs は App 未導入の組織名(記録の順)。AuthError のときは空(D4b-13)。
  Orgs []string
  // RecheckOrgs は「再確認」(D4b-14)。
  type RecheckOrgs struct {
      Accounts port.Accounts
      Session  *Session
      Conn     *Connector     // トークン(Connector.Token)
      GitHub   port.GitHub
      Missing  port.AppMissing
  }
  type RecheckResult struct{ Cleared, Still []string }
  func (u RecheckOrgs) Do(ctx context.Context, id account.ID) (RecheckResult, error)
  ```
  - HTTP の行: `status` は `ok`・`auth_error`・`app_missing`、`orgs: string[]`。`POST /api/accounts/{id}/recheck` → `{cleared: string[], still: string[]}`(operationId `recheck-account`)。失敗は再認証と同じ形(本文の `detail` に 07 の表の文言)。

- [ ] **Step 1: 失敗するテストを書く**

- `TestListAccountsAppMissing`: 記録あり → `Orgs == ["example-org"]`。同じアカウントが認証エラー → `AuthError`、`Orgs` は空、記録は残る。
- `TestRecheckOrgsClears`: 2 組織の記録、片方だけ `MissApp(false)` → `Cleared == [読めた組織]`、`Still == [残り]`、記録は残りだけ。
- `TestRecheckOrgsRejected`: トークンが拒否 → 認証エラーになり、認証エラーの文言のエラー。記録は変えない。
- `TestRecheckOrgsUnreachable`: 通信の失敗 → 07 の「通信できない」の文言のエラー。記録は変えない。
- HTTP: `TestAccountsStatusAppMissing`(行の JSON)、`TestRecheckAccount`。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/github/ ./internal/adapter/httpapi/github/`
Expected: FAIL

- [ ] **Step 3: 書く**

`RecheckOrgs` は組織ごとに `CheckRepo(記録の Repo)`。AppNotInstalled は Still、成功は Clear して Cleared、TokenRejected は `Session` に認証エラーを置いてすぐ返す、ほかの失敗はすぐ返す(それまでに読めた組織は消したまま)。`make generate`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go build ./... && go build -tags e2e ./... && go test ./internal/app/... ./internal/adapter/... ./cmd/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal cmd web/packages/api
git commit -m "feat: アカウントに GitHub App 未導入の組織を出し、「再確認」で読み直す(D4b Task 10)"
```

---

### Task 11: 設定の行・帯・依頼文のコピー・「再確認」(画面)

**Files:**
- Modify: `web/packages/core/src/model/account.ts`(`status` に `"app_missing"`、`orgs: string[]`)
- Modify: `web/packages/core/src/features/accounts/list.ts`(`recheck(row)`、行ごとの `still` の組織)、文言は `features/accounts` の文言のファイル
- Create: `web/app/src/screens/settings/AppMissingBand.svelte`
- Modify: `web/app/src/screens/settings/AccountItem.svelte`、`AccountsSection.svelte`
- Test: `web/packages/core/src/features/accounts/list.test.ts`、`web/packages/core/src/features/accounts/text.test.ts`

**Interfaces:**
- Produces(core):
  ```ts
  export const appMissingBandText = (name: string, org: string) => `${name}: ${org} に GitHub App が入っていないため、PR と Issue を読めません。`;
  export const installRequestText = (org: string, login: string) => `${org} に GitHub App をインストールしてください(@${login} が使います)`;
  export const recheckedText = (name: string, orgs: string[]) => `${name}: ${orgs.join("・")} を読めるようになりました`;
  export const copiedText = "コピーしました";
  export const copyFailedText = "コピーできませんでした。文を選択してコピーしてください";
  export const stillUnreadableText = "まだ読めません";
  // AccountList に足す。成功で一覧を読み直し、cleared があればトースト。still は帯の赤字に使う。失敗は行の赤字。
  recheck(row: AccountRow): Promise<void>;
  ```

- [ ] **Step 1: 失敗するテストを書く**

core: `recheck` が `asking` を立て、終わったら下ろす。`{cleared: ["a"], still: ["b"]}` → トースト「Taro: a を読めるようになりました」、`stillOf(row.id)` が `["b"]`。失敗 → 行の赤字。別の行を操作したら前の赤字と still が消える。文言の関数の表のテスト。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test`
Expected: FAIL

- [ ] **Step 3: 画面を書く**

行: `app_missing` なら橙の「GitHub App 未導入」、組織名(`・` でつなぐ)、「再確認」ボタン(問い合わせ中は無効、「GitHub に問い合わせています」)。帯: `AccountsSection` の一覧の上に、`app_missing` の行の組織ごとに `AppMissingBand`(文、「管理者にインストールを依頼」、still なら赤字「まだ読めません」、コピーの失敗なら文言と依頼文)。橙は `--c-orange-bg`・`--c-orange-fg`(なければ `@hedgehug/ui` の色の名前に合わせる)。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm run check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "feat(web): 設定に GitHub App 未導入の行・帯・依頼文のコピー・「再確認」を出す(D4b Task 11)"
```

---

### Task 12: 概要の「PR を探せませんでした」(画面)

**Files:**
- Modify: `web/packages/core/src/features/github/`(プロジェクトのリポジトリの失敗の文言を読むモデル。`ProjectLinks` のモデルの隣)
- Modify: `web/app/src/screens/project/OverviewRepos.svelte`、`web/app/src/reloads.ts`(github の知らせで読み直す)
- Test: core のモデルのテスト

**Interfaces:**
- Produces: `export const scanFailedText = (reason: string) => \`PR を探せませんでした: ${reason}\`;`、リポジトリの書き方から文言を引く `failureOf(repo: string): string`(大文字と小文字を区別しない)。

- [ ] **Step 1: 失敗するテストを書く**

モデルが `list-repo-failures` を読み、`failureOf("Example-Org/web")` が `example-org/web` の文言を返す。github の知らせで読み直す。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test`
Expected: FAIL

- [ ] **Step 3: `OverviewRepos.svelte` のリポジトリの行の下に、文言があれば赤字で「PR を探せませんでした: {理由}」を出す**

200 行を超えるなら、行を `OverviewRepoRow.svelte` に分ける。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm run check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "feat(web): プロジェクトの概要のリポジトリの行に「PR を探せませんでした」を出す(D4b Task 12)"
```

---

### Task 13: e2e の入口と通しのテスト

**Files:**
- Modify: `cmd/hedgehug/links_e2e.go`(`/e2e/github-pull`・`/e2e/github-app`・`/e2e/github-repo`。D4b-16)、`web/e2e/helpers.ts`
- Create: `web/e2e/autolink.spec.ts`
- Modify: `web/e2e/accounts.spec.ts`

- [ ] **Step 1: e2e を書く**(3 幅)

- 「リポジトリを足すと PR が自動でリンクされる」: `/e2e/github-pull` でブランチ `hh/{ID}` の Open の PR を置き、プロジェクトの概要で「リポジトリを追加」→ そのアイテムの詳細パネルの GitHub の欄に PR の行が出て、履歴に「GitHub」の「{ID} に {repo}#{n} を自動でリンク」。
- 「2 回目からはマージのルールが動く」: 自動化に「リンクした PR がマージされたとき → ステータスを Done」を作り、リポジトリを足して 1 本目(Merged)を付けてもステータスは変わらない。2 本目の Merged の PR を置いて `/e2e/github-read` → Done。
- 「探せなかった理由が概要に出る」: `/e2e/github-repo?repo=…&unreadable=1` でリポジトリ全体を読めなくして `/e2e/github-read` → 概要のリポジトリの行に「PR を探せませんでした: {repo} を読めません。…」。`unreadable=0` で戻して `/e2e/github-read` → 消える。
- 「App 未導入の帯と再確認」(`accounts.spec.ts`): `/e2e/github-app?owner=…&missing=1` → `/e2e/github-read` → 設定のアカウントの行に「GitHub App 未導入」と組織名、一覧の上に帯。「管理者にインストールを依頼」でトースト「コピーしました」(クリップボードの権限を与える)。「再確認」→ 帯に「まだ読めません」。`missing=0` にして「再確認」→ トースト「{e2e のアカウントの表示名}: {組織} を読めるようになりました」、帯が消え、行は「利用可能」。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e-bin && cd web && pnpm exec playwright test e2e/autolink.spec.ts e2e/accounts.spec.ts`
Expected: 入口を足す前は FAIL

- [ ] **Step 3: 入口を書き、通ることを確かめる**

Run: `make e2e-bin && cd web && pnpm exec playwright test e2e/autolink.spec.ts e2e/accounts.spec.ts`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd web
git commit -m "test(e2e): PR の自動リンク・探せなかった理由・App 未導入の帯と再確認を確かめる(D4b Task 13)"
```

---

### Task 14: 仕上げ

- [ ] **Step 1: すべての確認**

Run: `make check`(lint・Go のテスト・web の確認・archtest・大きさの上限)と、e2e 全体を 1 回。
Expected: すべて緑。

- [ ] **Step 2: 最終レビュー**(subagent-driven-development の最後の全体レビュー。Review Focus の 5 つを確かめさせる)

- [ ] **Step 3: PR を出す**

`git push -u origin feat/track-d4b`。向き先 develop、タイトル「feat: トラック D4b(PR の自動リンクと GitHub App 未導入)」。本文に、できること・決めたこと(D4b-1〜D4b-16)・確認の結果・使ったスキルと省いた手順を書く。
