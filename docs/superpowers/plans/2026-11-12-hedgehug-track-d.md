# hedgehug トラック D(GitHub)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 07 の GitHub 連携(アカウント、PR・Issue のリンク、PR の自動リンク、GitHub を読む間隔、コラボレーターの取り込み、Store の検索)を作る。

**Architecture:** 設計書の 3 節のとおり、usecase(`app/github`)は口(`port.GitHub`・`port.Secrets`・`port.Accounts` ほか)越しにだけ外に触る。GitHub の REST API の実装は `infra/github`、トークンの読み出しは `infra/secret`、アカウントの一覧は `infra/machineconfig`(config.yaml)に置く。GitHub とのやりとりの失敗は `domain/ghcall.Failure`(種類と値)で表し、usecase が利用者に返すときに `ghcall.Refuse` でトラック A の `domain/refusal`(仕様の文言の断り)に直す。API は A の `apierr.From` で 422 の `detail` に文言を入れる。トークンはエラーにもログにも出さない(前回の実装と同じく、エラー文の中の `ghp_`・`gho_`・`ghu_`・`ghs_`・`ghr_`・`github_pat_` で始まる文字列を伏せる)。usecase のテストは `app/apptest` の偽物で、`infra/github` のテストは httptest の偽のサーバーで行い、本物の GitHub での確認は週 1 回の github-live で行う。

**Tech Stack:** Go 1.27.3、net/http(GitHub の REST API v3、`X-GitHub-Api-Version: 2022-11-28`)、github.com/zalando/go-keyring、go.yaml.in/yaml/v4、huma v2。画面は土台の core・api・ui・app の形。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md`(この計画の正本。全節)
- 01「用語と階層」「Store で共有するものと、マシンごとに持つもの」「「あなた」の決まり方」「書き込みの単位」「閲覧のみの Store」「日付と時刻」
- 02 の詳細パネルの GitHub の欄、プロジェクトのリポジトリ、「PR 用の印をコピー」、自動リンクを探せなかったときの表示
- 03 の自動化のきっかけ(PR・Issue の 4 つ)と実行の規則
- 06「自動で同期する」(自動の読み取りを待たせる条件)、「同期のエラー」の「認証に失敗した」、「アクティビティ」、「通知」、「時計のずれの警告」
- 08「一般」の「同期のタイミング」、「Store」の「GitHub に使うアカウント」「追加」「remote の変更」、「メンバー」、「初回起動」
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(1〜8 節)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「変更記録」の `add`・`remove`(リンク・解除した PR の記録)、「このマシンに持つもの」、「復活・残すと決めた現行機能の置き場所」の PR の自動リンク・レビューの状態・自動化のきっかけ・GitHub から取り直した変更の通知
- `docs/superpowers/plans/2026-11-12-hedgehug-foundation.md`(土台の型・口・関数名。この計画はそれに合わせる)

**持ち場:** `internal/domain/account`・`internal/domain/ghcall`・`internal/domain/itemlink`・`internal/domain/prref`、`internal/app/github`、`internal/infra/github`・`internal/infra/secret`・`internal/infra/machineconfig`(未決の点 1)、`internal/adapter/httpapi/github`、`web/packages/core/features/github`、`web/packages/api/src/github.ts`、`web/app/screens/github`、`.github/workflows/github-live.yaml`。

**待つもの:**
- D1 は土台の Task 5(口と apptest)・Task 6(sysclock)・Task 8(httpapi の骨組みと cmd の組み立て)と、トラック A の A1 Task 1(`domain/refusal`・`apierr`・`port.Identity`・`httpapi.Deps` の `IDs`)が develop に入っていること。土台の Task 7(gitstore)は待たない。
- D2 は D1 と、土台の Task 11(ui の基本部品)・Task 12(app の組み立て)。
- D3 は D2 と、トラック A の A2(詳細パネルと GitHub の区画の枠)・A4(プロジェクトのリポジトリの追加と削除)。アイテムの一覧は A1 Task 1 の `ProjectReader.List`・`ItemReader.List` で読む。
- D4a は D3 と、トラック B の自動化の実行の小計画、トラック C の「自動で同期する」と state.db の骨組み。
- D4b は D4a。
- D5 は D1(コラボレーター)と、トラック G の Store の追加(Store の検索のマウント)。

**ブランチ:** タスクごとに `origin/develop` から `feat/track-d<小計画>-<タスク番号>` を切る(例: `feat/track-d1-3`)。PR の向き先は `develop`。共有部分の変更(下の「共有部分の変更」)は、その変更だけの小さな PR にして先に入れる(設計書 8 節「衝突を防ぐルール」)。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- トークンは、ログ・エラーの文・API の応答・Store・config.yaml のどこにも書かない。`gh` の標準エラー出力は読まずに捨てる。テストで、エラーの文にトークンが含まれないことを確かめる。
- GitHub への 1 回の問い合わせは 30 秒で打ち切る(07「GitHub との通信に失敗したとき」)。デバイスフローの待機はこの対象にしない。
- GitHub との通信は、すべて、その Store に割り当てたアカウントで行う。別のアカウントで読み直さない(07「GitHub との通信に使うアカウント」)。
- 断りの文言は 07 の表のとおりに書き、言い換えない。文言はサーバーの domain(`account.Check` と `ghcall.Refuse`)が `refusal.New` で持ち、API は 422 の `detail` で返す。画面は `detail` をそのまま出す(トラック A の決定 1 にそろえる)。
- アカウントの登録・編集・削除・再認証は Store を変えない。保存もアクティビティも作らない(07「アカウント」、01)。
- 仕様にない振る舞いを足さない。モックの振る舞いは決定ではない。

---

## この計画で決めたこと(既存の規則の当てはめ)

| # | 決めたこと | 根拠 |
|---|---|---|
| 1 | トラック D が足す口は `GitHub`・`Secrets`(土台の計画の決定 1)と、`Accounts`(アカウントの一覧)・`StoreGitHub`(Store に割り当てたアカウント・remote・閲覧のみ・非表示を読む)。`Accounts` の本物の実装は D が作る(未決の点 1)。`StoreGitHub` の本物の実装は、Store の一覧を持つトラック G が作り、それまで D は `apptest` の偽物で作る | 設計書 3 節「口は使うトラックが足す」。Store の一覧は G の持ち場(土台の計画の決定 6) |
| 2 | GitHub とのやりとりの失敗は `domain/ghcall.Failure` の 1 つの型で表す。種類(`Kind`)は 07「GitHub との通信に失敗したとき」と「認証の確認」の表の行に 1 対 1 で対応させる。HTTP の 401 は、登録・再認証では「トークンが GitHub に受け付けられませんでした」、それ以外では「使うアカウントが認証エラー」になるので、`infra/github` は 401 を `TokenRejected` として返し、使い分けは usecase が行う。利用者に返すときは `ghcall.Refuse` で 07 の文言の `refusal` に直す | 07 の表。断りの形はトラック A の決定 1(`domain/refusal`・`apierr`)にそろえる |
| 3 | GitHub の応答の `Date` と、このマシンの時計のずれは、`infra/github` が問い合わせのたびに測って `OnSkew` で知らせるだけにする。記録と警告の表示(5 分以上で警告)はトラック C(06「時計のずれの警告」)が受け取る | 07「GitHub との通信に使うアカウント」の最後の段落は 06・08 に表示を委ねている |
| 4 | 利用上限の解除時刻は、`ghcall.Refuse` がこのマシンの時刻帯で「HH:MM」に書く。秒数だけのときに限らず、いつも分に切り上げる(「HH:MM 以降」が解除より前にならないため)。翌日以降は 01「日付と時刻」の書き方で日付を前に足す | 07「GitHub との通信に失敗したとき」。サーバーはこのマシンで動くので、時刻帯は画面と同じ |
| 5 | `gh` のトークンは `gh auth token --hostname github.com --user <ログイン名>` で読む。環境変数は `GH_TOKEN`。キーチェーンは、サービス `hedgehug`、アカウント `github.com/<ログイン名>`(前回と同じ。未決の点 2) | 07「登録」の 4 つの認証方法。前回の実装の場所を引き継ぐ |
| 6 | アイテムのリンクと、解除した PR の記録は、アイテムの `log.jsonl` の `add`・`remove`(項目 `links`・`links_off`)と、`set links/<種別:リポジトリ#番号>/<state・title・review>` で持つ(保存形式の決定どおり)。この読み方は D の `domain/itemlink` に置き、`domain/item.Item` には足さない。`item.yaml` への書き出し(gitstore の `yaml.go`)は、D3 の先頭の共有の小さな PR で足す | 07 がリンクの規則を持つ。item の中心の型(A の持ち場)を広げずに済む |
| 7 | 設定画面の「アカウントとメンバー」の枠は、08 の設定画面の構成に従う。設定画面の枠がまだ develop にないときは、D2 画面が、枠だけ(タブと空の中身)を共有の小さな PR で先に入れ、D はそのタブの中身(`screens/github/AccountsSection.svelte`)だけを持つ | 設計書 8 節「共有部分の変更」 |
| 8 | 詳細パネルの GitHub の欄は、区画の枠をトラック A の A2 が置き(A の計画の漏れの表「詳細パネル: GitHub」)、D3 がその中身(`screens/github/LinkSection.svelte`)を作る | 02 が配置を、07 が中身を定める |
| 9 | state.db の骨組み(開く・表の作り方)は、C と D のうち先に要るトラックが、共有の小さな PR で入れる。D の表(GitHub を読んだ記録、チェックの結果、GitHub App 未導入の組織、トークンの指紋)は D の小計画の中で足す | 設計書 8 節「共有部分の変更」。state.db は複数のトラックが使う |
| 10 | usecase のパッケージは `app/github` 1 つから始め、20 ファイルを超えそうになったら、操作のまとまりで `app/githublink`(リンク)・`app/githubread`(読む間隔)に分ける | 設計書 1 節・4 節 |
| 11 | 本物の GitHub での確認(github-live)は、週 1 回と手動で、リポジトリに設定済みの secret `HEDGEHUG_LIVE_TOKEN` と変数 `HEDGEHUG_LIVE_ORG`(前回と同じもの。D3 からは検証用の組織のリポジトリ `hh-live` を読む)が設定されているときだけ動かす。設定がなければ手順を飛ばして成功する(前回と同じ形) | 設計書 7 節 |
| 12 | トラック A の A1 Task 1(`domain/refusal`・`apierr`・`port.Identity`・`ProjectReader.List`・`ItemReader.List`・`httpapi.Deps` の `IDs`・`Identity`)を前提にし、同じものを D で別に定義しない。保存する操作の「あなた」は `port.Identity.Me` で得る | 親のセッションからの調整(A1 Task 1 が B・C・D より先に入る共有部分) |

## 共有部分の変更

設計書 8 節のとおり、次は、その変更だけの小さな PR にして先に develop へ入れる。

| 小計画 | 変更 | 理由 |
|---|---|---|
| D1 | `app/port` に `github.go`・`secrets.go`・`accounts.go` を足し、`app/apptest` に偽物と照合テストを足す(D1 の Task 1) | 口の追加 |
| D1 | `httpapi.Deps`(`Accounts`・`Secrets`・`GitHub`・`Clock`)・`httpapi/routes.go`・`cmd/hedgehug/wire*.go` と生成物(`api/openapi.yaml`・`web/packages/api/src/generated.ts`)(D1 の Task 7)。`IDs`・`Identity` は A1 Task 1 が足したものを使う | 組み立て |
| D2 | `app/port` の `deviceflow.go`・`hosts.go`・`storegithub.go` と、`accounts.go`・`secrets.go` の追加(D2 Task 1)。設定画面の枠(まだなければ。決定 7。D2 画面) | 口の追加と画面の枠 |
| D3 | `infra/gitstore/yaml.go` の `item.yaml` に `links`・`links_off` を書き出す。Issue の 1 対 1 の検査(すべての Store の、削除済みを含むアイテム)は A1 Task 1 の `ProjectReader.List`・`ItemReader.List` で読む | 保存形式の書き出し |
| D4a | state.db の骨組み(まだなければ。決定 9)。`port.GitHubTokens`・`port.GitHubReads`・`StoreGitHubInfo.Hidden`・`SyncTiming.GitHubEvery`・`CheckRuleDrafts` | 共有の infra と口 |

アイテムのリンク(`item.yaml` の `links`・`links_off` と、その読み方)は **D が足す**(決定 6)。A は足さない。

---

## 小計画の一覧

| 小計画 | 範囲(仕様 07 の節) | 順 | 待つもの | 完成の条件 |
|---|---|---|---|---|
| **D1 アカウントの登録と GitHub への接続(サーバー)** | 「アカウント」の「登録」「認証の確認」(デバイスフロー以外)、「GitHub との通信に失敗したとき」の失敗の種類と 30 秒の打ち切り、時計のずれの測定、github-live | 1 | 土台 Task 5・6・8 | `make check` が緑。`POST /api/accounts` で gh・環境変数・キーチェーンのアカウントを登録でき、07 の表のすべての断り方を `code` で返す。`Accounts` の照合テストが偽物と config.yaml の両方で通る。github-live が手動で動く |
| **D2 アカウントの画面と残りの操作** | 「GitHub を使わない場合」の入口の無効(共通の部品)、「デバイスフロー」「接続のたびの照合」「状態」「再認証」「編集」「削除の影響」、「GitHub との通信に使うアカウント」(Store の割り当ての読み取り)、設定の「アカウントとメンバー」の GitHub アカウントの画面。前回の実装から取り込む: `config.yaml` の `hosts[].client_id` の読み出し、期限付きトークンの取り直し(期限の前に、ロックを取って更新用のトークン `ghr_` で取り直す。`expires_in` が 0 なら期限なし) | 2 | D1、土台 Task 11・12、A2(ルーター・サイドバー・トースト。画面だけ) | 計画は `2026-11-13-hedgehug-track-d2.md`(サーバー)。画面は A2 のあとで「D2 画面」として書く。設定の画面でアカウントを追加・編集・削除・再認証でき、利用可能と認証エラーが出る。デバイスフローを偽の GitHub で通しで確かめる(Playwright、3 幅)。トークンの指紋を照合する |
| **D3 PR と Issue のリンク(手で付ける・外す・取り直す)** | 「PR と Issue のリンク」の「追加」「表示と更新」(詳細パネルを開いたときの取り直し、改名・移管の追従、失敗の表示)、「解除」、「アクティビティに残る文」のリンクの 3 行 | 3 | D2、A の A2(詳細パネル)・A4(プロジェクトのリポジトリ) | 詳細パネルで PR・Issue をリンク・解除でき、07 の検査の表のすべての断り方を出す。Issue の 1 対 1 をすべての Store で確かめる。取り直しで値が変わったときだけ実行者「GitHub」で保存する |
| **D4a GitHub を読む間隔と間隔ごとの取り直し・自動化のきっかけ** | 「GitHub を読む間隔」(PR の自動リンクの探索を除く)、「表示と更新」の間隔ごとの取り直し、「PR・Issue の状態で動く自動化」(手でのリンク・詳細パネルを開いたときの取り直し・間隔ごとの取り直しの 3 つから起こす)、認証エラーの自動の確かめ直し | 4 | D3、B の自動化の実行、C の「自動で同期する」と state.db の骨組み | 間隔・利用上限(3 倍・解除まで止める・1 分)・失敗後の待ちを、偽の時計で確かめる。リンクの状態の変化で自動化のきっかけが 1 回だけ起き、認証エラーのアカウントが自動で利用可能に戻る |
| **D4b PR の自動リンクと GitHub App 未導入** | 「PR の自動リンク」(プロジェクトにリポジトリを追加したときのすぐの探索と、概要の「PR を探せませんでした」を含む)、「GitHub を読む間隔」の PR の自動リンクの探索(前回から更新されたものだけを読む)、「状態」の GitHub App 未導入(検出・記録・行の表示・帯・「管理者にインストールを依頼」・「再確認」。検出は前回の実装と同じく、App のトークンで組織のリポジトリを読んだときの GitHub の拒否の応答で見分ける。D2 の計画の決定 D2-4) | 4(D4a のあと) | D4a | ブランチ名・印・タイトル・本文・コメントから PR を自動でリンクし、初回の読み取りではきっかけを起こさない。App 未導入の組織を記録し、設定の行・帯・再確認が動く |
| **D5 コラボレーターの取り込みと Store の検索** | 「GitHub のコラボレーターをメンバーに追加」、「GitHub から Store を検索」、「アクティビティに残る文」のコラボレーターの行 | 5(コラボレーターは D2 のあとならいつでも) | D2(コラボレーター)、G の Store の追加(検索のマウント) | コラボレーターを 100 人ずつ最後まで読み、選んだ人を 1 回の保存で加える。Store の検索で、見えるリポジトリを 1 行にまとめ、マウントで Store を追加する |

### 漏れの確かめ(仕様 07 の節 → 小計画)

| 07 の節 | 小計画 |
|---|---|
| (冒頭) | D1〜D5 |
| GitHub を使わない場合 | D2 画面(無効の部品と文言)、各入口は D3・D5 |
| アカウント(冒頭の段落) | D1(マシンごと・Store に保存しない)、D2(デバイスフローの再起動後の利用) |
| 登録 | D1(サーバー)、D2(ダイアログ) |
| 認証の確認 | D1(gh・環境変数・キーチェーンと文言)、D2(画面での出し方とデバイスフロー) |
| 接続のたびの照合 | D2 |
| デバイスフロー | D2 |
| 状態 | D2(利用可能・認証エラーと、認証エラーへの変化)、D4a(自動の確かめ直し)、D4b(GitHub App 未導入の検出・記録・帯・「再確認」。D2 の計画の決定 D2-4) |
| 再認証 | D2 |
| 編集 | D2 |
| 削除の影響 | D2 |
| GitHub との通信に使うアカウント | D2(Store の割り当て)、D1(時計のずれの測定) |
| GitHub との通信に失敗したとき | D1(種類・30 秒・文言・時刻の書き方)、D2(画面での出し方) |
| PR と Issue のリンク(冒頭) | D3 |
| PR の自動リンク | D4b |
| 追加 | D3 |
| 表示と更新 | D3(開いたとき・改名・失敗の表示)、D4a(間隔ごと) |
| GitHub を読む間隔 | D4a(読む仕組みと取り直し)、D4b(自動リンクの探索) |
| 解除 | D3 |
| PR・Issue の状態で動く自動化 | D4a(D4b は自動リンクの保存からも呼ぶ) |
| GitHub のコラボレーターをメンバーに追加 | D5 |
| GitHub から Store を検索 | D5 |
| アクティビティに残る文 | D3(リンクの 3 行)、D5(コラボレーター) |
| モックからの変更点 | 各行の中身は上の節で扱う |

---

# 小計画 D1: アカウントの登録と GitHub への接続(サーバー)

D1 は、gh CLI・環境変数・OS のキーチェーンのアカウントを、GitHub に問い合わせて確かめてから登録する API までを作る。デバイスフローと画面は D2 で作る。

## ファイルの地図

```
internal/domain/account/account.go       Account・Method・Check(07「登録」の検査)
internal/domain/account/account_test.go
internal/domain/ghcall/failure.go        Failure・Kind(07 の失敗の種類)
internal/domain/ghcall/message.go        Refuse(07 の文言の断りに直す)
internal/domain/ghcall/failure_test.go・message_test.go

internal/app/port/github.go              GitHub(Viewer)
internal/app/port/secrets.go             Secrets(Token)
internal/app/port/accounts.go            Accounts(List・Add)
internal/app/apptest/github.go           GitHub・Secrets の偽物
internal/app/apptest/accounts.go         Accounts の偽物
internal/app/apptest/accounts_contract.go RunAccountsContract
internal/app/apptest/accounts_test.go

internal/infra/github/client.go          問い合わせ・失敗の分け方・時計のずれ
internal/infra/github/viewer.go          GET /user
internal/infra/github/client_test.go     httptest の偽のサーバー
internal/infra/github/live_test.go       //go:build live(github-live)
internal/infra/secret/source.go          gh・GH_TOKEN・キーチェーン
internal/infra/secret/keyring.go         go-keyring
internal/infra/secret/source_test.go
internal/infra/machineconfig/file.go     config.yaml の読み書き
internal/infra/machineconfig/accounts.go Accounts の実装
internal/infra/machineconfig/accounts_test.go

internal/app/github/add_account.go       AddAccount
internal/app/github/list_accounts.go     ListAccounts
internal/app/github/accounts_test.go

internal/adapter/httpapi/github/accounts.go  GET・POST /api/accounts(エラーは A の apierr.From)
internal/adapter/httpapi/github/accounts_test.go

internal/adapter/httpapi/routes.go(変更)、cmd/hedgehug/wire.go・wire_e2e.go(変更)、cmd/hedgehug/paths.go
.github/workflows/github-live.yaml
```

---

### Task 1: 口と domain の型と偽物(共有の小さな PR)

D の口と、口が使う domain の型と、`apptest` の偽物・照合テストを、先に 1 つの小さな PR で入れる。ほかのトラックはこの PR を取り込む(rebase)。トラック A の A1 Task 1(`domain/refusal`・`port.Identity`・`apierr`・`httpapi.Deps` の `IDs`)が develop に入ってから始める。

**Files:**
- Create: `internal/domain/account/account.go`、`internal/domain/ghcall/failure.go`、`internal/domain/ghcall/message.go`
- Test: `internal/domain/account/account_test.go`、`internal/domain/ghcall/failure_test.go`、`internal/domain/ghcall/message_test.go`
- Create: `internal/app/port/github.go`、`internal/app/port/secrets.go`、`internal/app/port/accounts.go`
- Create: `internal/app/apptest/github.go`、`internal/app/apptest/accounts.go`、`internal/app/apptest/accounts_contract.go`
- Test: `internal/app/apptest/accounts_test.go`

**Interfaces:**
- Consumes: A1 Task 1 の `refusal.New(message string) error`・`refusal.Refusal{Message string}`、土台の `port.IDs`
- Produces:
  - `account.ID`(`string`)、`account.Method`(`DeviceFlow`・`GhCLI`・`EnvVar`・`Keychain`)、`(Method) Valid() bool`、`account.Host = "github.com"`
  - `account.Account{ID ID; Name, Login string; Method Method}`
  - `account.Check(in Account, existing []Account, self ID) (Account, error)` — 07「登録」の表の文言で `refusal.New` を返す
  - `ghcall.Kind`(下の表)、`ghcall.Failure{Kind Kind; Name, Login, Repo, Org string; Number int; ResetAt time.Time}`、`(*Failure) Error() string`(種類とリポジトリだけ。文言ではない)、`ghcall.Is(err error, k Kind) bool`
  - `ghcall.Refuse(err error, now time.Time) error` — `*Failure` を 07 の表の文言の `refusal` に直す。`*Failure` でないエラーはそのまま返す
  - `port.GitHub{ Viewer(ctx, token string) (login string, err error) }`
  - `port.Secrets{ Token(ctx, a account.Account) (string, error) }`
  - `port.Accounts{ List(ctx) ([]account.Account, error); Add(ctx, a account.Account) error }`
  - `apptest.GitHub{Users map[string]string; Err error}`、`apptest.Secrets{Tokens map[string]string; Err error}`(キーはログイン名)、`apptest.NewAccounts() *apptest.Accounts`
  - `apptest.RunAccountsContract(t *testing.T, newAccounts func(t *testing.T) port.Accounts)`

**失敗の種類と文言(`ghcall.Kind` と `ghcall.Refuse`):** 文言は 07 の表のとおり。`{…}` は `Failure` の値から入れる。

| Kind | 文言(07) |
|---|---|
| `unreachable` | GitHub に接続できません。接続を確かめて、もう一度お試しください |
| `auth_error` | {Name}(@{Login})の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください |
| `repo_unreadable` | {Repo} を読めません。リポジトリの権限と、使うアカウントを確かめてください |
| `app_not_installed` | {Org} に GitHub App が入っていないため、{Repo} を読めません |
| `no_write` | {Repo} に書き込む権限がありません |
| `not_found` | {Repo}#{Number} が見つかりません |
| `rate_limited` | GitHub の利用上限に達しました。{時刻} 以降にもう一度お試しください(時刻が分からなければ「GitHub の利用上限に達しました。しばらくしてからもう一度お試しください」) |
| `no_account` | {Repo} を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください |
| `token_rejected` | トークンが GitHub に受け付けられませんでした。認証方法を確かめてください |
| `login_mismatch` | このトークンは @{Login} のものです。ログイン名を直してください |
| `gh_token_missing` | gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください |
| `gh_timeout` | gh CLI から応答がありません。端末で `gh auth status` を確かめてください |
| `env_missing` | 環境変数 GH_TOKEN が設定されていません |
| `keychain_missing` | OS のキーチェーンに、サービス hedgehug・アカウント github.com/{Login} のトークンが見つかりません |
| `keychain_unavailable` | このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください |
| `forbidden` | (内部の種類。D3 以降で `repo_unreadable`・`app_not_installed`・`no_write` に分ける。分けずに残ったときは、Repo があれば `repo_unreadable`、なければ `unreachable` の文言) |

{時刻} は、解除の時刻を分に切り上げ、このマシンの時刻帯で「HH:MM」にする。解除が `now` の翌日以降なら、前に日付と半角スペースを足す(今年なら「11/13 09:00」、ほかの年なら「2027/01/03 09:00」。01「日付と時刻」)。

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/account/account_test.go`:

```go
package account_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

var existing = []account.Account{
	{ID: "A1", Name: "work", Login: "taro", Method: account.GhCLI},
	{ID: "A2", Name: "home", Login: "taro-home", Method: account.EnvVar},
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

// 07「登録」の表の順に確かめる。最初に当てはまったものを返す。
func TestCheckOrder(t *testing.T) {
	cases := []struct {
		name string
		in   account.Account
		self account.ID
		want string
	}{
		{"empty name", account.Account{Name: " ", Login: "x", Method: account.GhCLI}, "", "表示名とログイン名を入力してください"},
		{"empty login", account.Account{Name: "x", Login: "", Method: account.GhCLI}, "", "表示名とログイン名を入力してください"},
		{"empty before duplicate", account.Account{Name: "WORK", Login: "", Method: account.GhCLI}, "", "表示名とログイン名を入力してください"},
		{"duplicate name ignores case", account.Account{Name: "WORK", Login: "TARO", Method: account.GhCLI}, "", "同じ表示名のアカウントがあります"},
		{"duplicate login ignores case", account.Account{Name: "new", Login: "TARO", Method: account.GhCLI}, "", "同じログイン名のアカウントがあります"},
		{"editing itself is not a duplicate", account.Account{Name: "work", Login: "taro", Method: account.Keychain}, "A1", ""},
	}
	for _, c := range cases {
		_, err := account.Check(c.in, existing, c.self)
		if got := message(err); got != c.want {
			t.Errorf("%s: message = %q, want %q (err %v)", c.name, got, c.want, err)
		}
	}
}

func TestCheckUnknownMethodIsNotARefusal(t *testing.T) {
	// 認証方法は画面の選択肢から選ぶので、知らない値は呼び出しの誤り(500)。仕様の文言はない。
	_, err := account.Check(account.Account{Name: "new", Login: "jiro", Method: "ssh"}, existing, "")
	if err == nil || message(err) != "" {
		t.Errorf("err = %v", err)
	}
}

func TestCheckTrims(t *testing.T) {
	got, err := account.Check(account.Account{Name: "  new ", Login: " jiro\t", Method: account.DeviceFlow}, existing, "")
	if err != nil || got.Name != "new" || got.Login != "jiro" {
		t.Errorf("Check = %+v, %v", got, err)
	}
}
```

`internal/domain/ghcall/failure_test.go`:

```go
package ghcall_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func TestIs(t *testing.T) {
	err := fmt.Errorf("read: %w", &ghcall.Failure{Kind: ghcall.NotFound, Repo: "org/web", Number: 3})
	if !ghcall.Is(err, ghcall.NotFound) || ghcall.Is(err, ghcall.Unreachable) {
		t.Error("Is does not see the wrapped failure")
	}
	if ghcall.Is(errors.New("x"), ghcall.NotFound) {
		t.Error("Is matched a plain error")
	}
}

func TestErrorNamesOnlyKindAndRepo(t *testing.T) {
	f := &ghcall.Failure{Kind: ghcall.NotFound, Repo: "org/web", Number: 3}
	if got := f.Error(); got != "github: not_found (org/web#3)" {
		t.Errorf("Error() = %q", got)
	}
	if got := (&ghcall.Failure{Kind: ghcall.Unreachable}).Error(); got != "github: unreachable" {
		t.Errorf("Error() = %q", got)
	}
}
```

`internal/domain/ghcall/message_test.go`:

```go
package ghcall_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

var jst = time.FixedZone("JST", 9*3600)

// refused は、包んだ Failure を Refuse で直した文言を返す。
func refused(t *testing.T, f *ghcall.Failure, now time.Time) string {
	t.Helper()
	var r *refusal.Refusal
	if err := ghcall.Refuse(fmt.Errorf("op: %w", f), now); !errors.As(err, &r) {
		t.Fatalf("Refuse(%v) = %v, not a refusal", f, err)
	}
	return r.Message
}

func TestMessages(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, jst)
	cases := []struct {
		f    *ghcall.Failure
		want string
	}{
		{&ghcall.Failure{Kind: ghcall.Unreachable}, "GitHub に接続できません。接続を確かめて、もう一度お試しください"},
		{&ghcall.Failure{Kind: ghcall.AuthError, Name: "work", Login: "taro"}, "work(@taro)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください"},
		{&ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: "org/web"}, "org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"},
		{&ghcall.Failure{Kind: ghcall.AppNotInstalled, Org: "org", Repo: "org/web"}, "org に GitHub App が入っていないため、org/web を読めません"},
		{&ghcall.Failure{Kind: ghcall.NoWrite, Repo: "org/web"}, "org/web に書き込む権限がありません"},
		{&ghcall.Failure{Kind: ghcall.NotFound, Repo: "org/web", Number: 12}, "org/web#12 が見つかりません"},
		{&ghcall.Failure{Kind: ghcall.NoAccount, Repo: "org/web"}, "org/web を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください"},
		{&ghcall.Failure{Kind: ghcall.TokenRejected}, "トークンが GitHub に受け付けられませんでした。認証方法を確かめてください"},
		{&ghcall.Failure{Kind: ghcall.LoginMismatch, Login: "hanako"}, "このトークンは @hanako のものです。ログイン名を直してください"},
		{&ghcall.Failure{Kind: ghcall.GhTokenMissing}, "gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください"},
		{&ghcall.Failure{Kind: ghcall.GhTimeout}, "gh CLI から応答がありません。端末で `gh auth status` を確かめてください"},
		{&ghcall.Failure{Kind: ghcall.EnvMissing}, "環境変数 GH_TOKEN が設定されていません"},
		{&ghcall.Failure{Kind: ghcall.KeychainMissing, Login: "taro"}, "OS のキーチェーンに、サービス hedgehug・アカウント github.com/taro のトークンが見つかりません"},
		{&ghcall.Failure{Kind: ghcall.KeychainUnavailable}, "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください"},
		{&ghcall.Failure{Kind: ghcall.Forbidden, Repo: "org/web"}, "org/web を読めません。リポジトリの権限と、使うアカウントを確かめてください"},
		{&ghcall.Failure{Kind: ghcall.Forbidden}, "GitHub に接続できません。接続を確かめて、もう一度お試しください"},
	}
	for _, c := range cases {
		if got := refused(t, c.f, now); got != c.want {
			t.Errorf("%s: %q\nwant %q", c.f.Kind, got, c.want)
		}
	}
}

func TestRateLimitTime(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, jst)
	const head = "GitHub の利用上限に達しました。"
	cases := []struct {
		name  string
		reset time.Time
		want  string
	}{
		{"same day, rounded up to the minute", time.Date(2026, 11, 12, 1, 30, 1, 0, time.UTC), head + "10:31 以降にもう一度お試しください"},
		{"next day", time.Date(2026, 11, 13, 9, 0, 0, 0, jst), head + "11/13 09:00 以降にもう一度お試しください"},
		{"next year", time.Date(2027, 1, 3, 9, 0, 0, 0, jst), head + "2027/01/03 09:00 以降にもう一度お試しください"},
		{"unknown", time.Time{}, head + "しばらくしてからもう一度お試しください"},
	}
	for _, c := range cases {
		got := refused(t, &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: c.reset}, now)
		if got != c.want {
			t.Errorf("%s: %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestRefuseKeepsOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	if ghcall.Refuse(boom, time.Now()) != boom || ghcall.Refuse(nil, time.Now()) != nil {
		t.Error("Refuse changed a non-failure")
	}
}
```

`internal/app/apptest/accounts_test.go`:

```go
package apptest_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestFakeAccounts(t *testing.T) {
	apptest.RunAccountsContract(t, func(*testing.T) port.Accounts { return apptest.NewAccounts() })
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/account/ ./internal/domain/ghcall/ ./internal/app/apptest/`
Expected: FAIL(`undefined: account.Check` ほか)

- [ ] **Step 3: domain の型を書く**

`internal/domain/account/account.go`:

```go
// Package account は、GitHub のアカウント(GitHub に接続するときの本人確認の手段に名前を付けたもの)を持つ。
// アカウントはマシンごとに持ち、Store には保存しない。トークンはここに持たない(01・07)。
package account

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// ID はアカウントの内部 ID(ULID)。
type ID string

// Host はアカウントのホスト。github.com に固定する(07「登録」)。
const Host = "github.com"

// Method は認証方法(07「登録」)。
type Method string

const (
	DeviceFlow Method = "device"
	GhCLI      Method = "gh"
	EnvVar     Method = "env"
	Keychain   Method = "keychain"
)

// Valid は、4 つの認証方法のどれかか。
func (m Method) Valid() bool {
	switch m {
	case DeviceFlow, GhCLI, EnvVar, Keychain:
		return true
	}
	return false
}

// Account は登録したアカウント。
type Account struct {
	ID     ID
	Name   string // 表示名
	Login  string // GitHub のログイン名
	Method Method
}

// Check は、登録・編集の入力を 07「登録」の表の順に確かめ、前後の空白を取り除いた値を返す。
// self は編集するアカウントの ID で、同じ表示名・ログイン名の検査から除く(登録では空)。
func Check(in Account, existing []Account, self ID) (Account, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Login = strings.TrimSpace(in.Login)
	if in.Name == "" || in.Login == "" {
		return Account{}, refusal.New("表示名とログイン名を入力してください")
	}
	for _, a := range existing {
		if a.ID != self && strings.EqualFold(a.Name, in.Name) {
			return Account{}, refusal.New("同じ表示名のアカウントがあります")
		}
	}
	for _, a := range existing {
		if a.ID != self && strings.EqualFold(a.Login, in.Login) {
			return Account{}, refusal.New("同じログイン名のアカウントがあります")
		}
	}
	if !in.Method.Valid() {
		return Account{}, fmt.Errorf("unknown account method %q", in.Method)
	}
	return in, nil
}
```

`internal/domain/ghcall/failure.go`:

```go
// Package ghcall は、GitHub とのやりとりの失敗を表す。種類は 07「GitHub との通信に失敗したとき」と
// 「認証の確認」の表の行に 1 対 1 で対応する。usecase は種類で振る舞いを分け(認証エラーにするなど)、
// 利用者に返すときに Refuse で 07 の文言の断り(refusal)に直す。トークンは持たない。
package ghcall

import (
	"errors"
	"fmt"
	"time"
)

// Kind は失敗の種類。
type Kind string

const (
	Unreachable     Kind = "unreachable"
	AuthError       Kind = "auth_error"
	RepoUnreadable  Kind = "repo_unreadable"
	AppNotInstalled Kind = "app_not_installed"
	NoWrite         Kind = "no_write"
	NotFound        Kind = "not_found"
	RateLimited     Kind = "rate_limited"
	NoAccount       Kind = "no_account"
	TokenRejected   Kind = "token_rejected"
	LoginMismatch   Kind = "login_mismatch"
	GhTokenMissing  Kind = "gh_token_missing"
	GhTimeout       Kind = "gh_timeout"
	EnvMissing      Kind = "env_missing"
	KeychainMissing Kind = "keychain_missing"
	// KeychainUnavailable は、このマシンに OS のキーチェーンがない(WSL など)。
	KeychainUnavailable Kind = "keychain_unavailable"
	// Forbidden は、403 で利用上限でないもの。リポジトリを読む操作で、RepoUnreadable・AppNotInstalled・
	// NoWrite に分ける(D3 以降)。
	Forbidden Kind = "forbidden"
)

// Failure は GitHub とのやりとりの失敗。使わない値は空。
type Failure struct {
	Kind    Kind
	Name    string    // アカウントの表示名(AuthError)
	Login   string    // アカウントのログイン名(AuthError・KeychainMissing)、実際のログイン名(LoginMismatch)
	Repo    string    // owner/name
	Org     string    // 組織名(AppNotInstalled)
	Number  int       // PR・Issue の番号(NotFound)
	ResetAt time.Time // 利用上限が戻る時刻。分からなければゼロ(RateLimited)
}

// Error は種類とリポジトリだけを書く(ログ用)。トークンや GitHub の応答の本文は書かない。
func (f *Failure) Error() string {
	switch {
	case f.Repo != "" && f.Number > 0:
		return fmt.Sprintf("github: %s (%s#%d)", f.Kind, f.Repo, f.Number)
	case f.Repo != "":
		return fmt.Sprintf("github: %s (%s)", f.Kind, f.Repo)
	}
	return "github: " + string(f.Kind)
}

// Is は、err が種類 k の Failure を含むか。
func Is(err error, k Kind) bool {
	var f *Failure
	return errors.As(err, &f) && f.Kind == k
}
```

`internal/domain/ghcall/message.go`:

```go
package ghcall

import (
	"errors"
	"fmt"
	"time"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Refuse は、err の Failure を 07 の文言の断り(refusal)に直す。Failure でないエラーはそのまま返す。
// now は利用上限の時刻の書き方に使う(このマシンの時刻帯で書く)。
func Refuse(err error, now time.Time) error {
	var f *Failure
	if !errors.As(err, &f) {
		return err
	}
	return refusal.New(message(f, now))
}

const unreachableText = "GitHub に接続できません。接続を確かめて、もう一度お試しください"

func message(f *Failure, now time.Time) string {
	switch f.Kind {
	case AuthError:
		return fmt.Sprintf("%s(@%s)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください", f.Name, f.Login)
	case RepoUnreadable:
		return f.Repo + " を読めません。リポジトリの権限と、使うアカウントを確かめてください"
	case AppNotInstalled:
		return fmt.Sprintf("%s に GitHub App が入っていないため、%s を読めません", f.Org, f.Repo)
	case NoWrite:
		return f.Repo + " に書き込む権限がありません"
	case NotFound:
		return fmt.Sprintf("%s#%d が見つかりません", f.Repo, f.Number)
	case RateLimited:
		return rateLimited(f.ResetAt, now)
	case NoAccount:
		return f.Repo + " を読めるアカウントがありません。設定の「Store」で GitHub に使うアカウントを選んでください"
	case LoginMismatch:
		return "このトークンは @" + f.Login + " のものです。ログイン名を直してください"
	case KeychainMissing:
		return "OS のキーチェーンに、サービス hedgehug・アカウント github.com/" + f.Login + " のトークンが見つかりません"
	case Forbidden:
		if f.Repo != "" {
			return message(&Failure{Kind: RepoUnreadable, Repo: f.Repo}, now)
		}
		return unreachableText
	}
	if text, ok := fixedText[f.Kind]; ok {
		return text
	}
	return unreachableText
}

// fixedText は、値を入れない文言。
var fixedText = map[Kind]string{
	Unreachable:     unreachableText,
	TokenRejected:   "トークンが GitHub に受け付けられませんでした。認証方法を確かめてください",
	GhTokenMissing:  "gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください",
	GhTimeout:       "gh CLI から応答がありません。端末で `gh auth status` を確かめてください",
	EnvMissing:      "環境変数 GH_TOKEN が設定されていません",
	KeychainUnavailable: "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください",
}

// rateLimited は 07 の利用上限の文言。時刻は分に切り上げ、翌日以降なら日付を前に足す(01「日付と時刻」)。
func rateLimited(reset, now time.Time) string {
	if reset.IsZero() {
		return "GitHub の利用上限に達しました。しばらくしてからもう一度お試しください"
	}
	at := reset.In(now.Location())
	if t := at.Truncate(time.Minute); !t.Equal(at) {
		at = t.Add(time.Minute)
	}
	text := at.Format("15:04")
	ny, nm, nd := now.Date()
	if y, m, d := at.Date(); y != ny || m != nm || d != nd {
		if y == ny {
			text = fmt.Sprintf("%d/%d %s", int(m), d, text)
		} else {
			text = at.Format("2006/01/02") + " " + text
		}
	}
	return "GitHub の利用上限に達しました。" + text + " 以降にもう一度お試しください"
}
```

- [ ] **Step 4: 口を書く**

`internal/app/port/github.go`:

```go
package port

import "context"

// GitHub は GitHub の API の口。失敗は *ghcall.Failure で返す。トラック D が、使う操作だけを足していく。
type GitHub interface {
	// Viewer は、token の持ち主のログイン名を返す(07「認証の確認」)。
	Viewer(ctx context.Context, token string) (login string, err error)
}
```

`internal/app/port/secrets.go`:

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Secrets はトークンの口。トークンは、この口の外では、問い合わせに渡すあいだだけ持つ。
type Secrets interface {
	// Token は、アカウントの認証方法でトークンを読む。読めなければ *ghcall.Failure
	// (GhTokenMissing・GhTimeout・EnvMissing・KeychainMissing)を返す。
	Token(ctx context.Context, a account.Account) (string, error)
}
```

`internal/app/port/accounts.go`:

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Accounts は、このマシンに登録したアカウントの一覧の口(01「マシンごとに持つ」)。
type Accounts interface {
	List(ctx context.Context) ([]account.Account, error) // 登録した順
	Add(ctx context.Context, a account.Account) error
}
```

- [ ] **Step 5: 偽物と照合テストを書く**

`internal/app/apptest/github.go`:

```go
package apptest

import (
	"context"
	"sync"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// GitHub は port.GitHub の偽物。Users はトークンからログイン名への対応。Err があれば、どの問い合わせもそれを返す。
type GitHub struct {
	mu    sync.Mutex
	Users map[string]string
	Err   error
}

func (g *GitHub) Viewer(_ context.Context, token string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Err != nil {
		return "", g.Err
	}
	login, ok := g.Users[token]
	if !ok {
		return "", &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return login, nil
}

// Secrets は port.Secrets の偽物。Tokens はログイン名からトークンへの対応。ない、または Err があれば失敗する。
type Secrets struct {
	mu     sync.Mutex
	Tokens map[string]string
	Err    error
}

func (s *Secrets) Token(_ context.Context, a account.Account) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return "", s.Err
	}
	tok, ok := s.Tokens[a.Login]
	if !ok {
		return "", &ghcall.Failure{Kind: ghcall.KeychainMissing, Login: a.Login}
	}
	return tok, nil
}
```

`internal/app/apptest/accounts.go`:

```go
package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// Accounts は port.Accounts の偽物。
type Accounts struct {
	mu   sync.Mutex
	list []account.Account
}

func NewAccounts() *Accounts { return &Accounts{} }

func (a *Accounts) List(context.Context) ([]account.Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.list), nil
}

func (a *Accounts) Add(_ context.Context, acc account.Account) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.list = append(a.list, acc)
	return nil
}
```

`internal/app/apptest/accounts_contract.go`:

```go
package apptest

import (
	"context"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// RunAccountsContract は、port.Accounts が満たすべき振る舞いを確かめる。偽物と、本物の実装
// (infra/machineconfig)の両方のテストから呼ぶ。newAccounts は、空の一覧を返す。
func RunAccountsContract(t *testing.T, newAccounts func(t *testing.T) port.Accounts) {
	t.Run("empty at first", func(t *testing.T) {
		got, err := newAccounts(t).List(context.Background())
		if err != nil || len(got) != 0 {
			t.Errorf("List = %+v, %v", got, err)
		}
	})
	t.Run("lists in the order added", func(t *testing.T) {
		accs := newAccounts(t)
		want := []account.Account{
			{ID: "01ACC00000000000000000002", Name: "work", Login: "taro", Method: account.GhCLI},
			{ID: "01ACC00000000000000000001", Name: "家", Login: "taro-home", Method: account.DeviceFlow},
		}
		for _, a := range want {
			if err := accs.Add(context.Background(), a); err != nil {
				t.Fatal(err)
			}
		}
		got, err := accs.List(context.Background())
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("List = %+v, %v; want %+v", got, err, want)
		}
	})
}
```

- [ ] **Step 6: テストが通ることを確かめる**

Run: `go test ./internal/domain/... ./internal/app/... && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS(domain/account・ghcall は標準ライブラリと domain だけ)

- [ ] **Step 7: Commit**

```bash
git add internal/domain/account internal/domain/ghcall internal/app/port internal/app/apptest
git commit -m "feat(app): GitHub・トークン・アカウントの口と偽物を足す(トラック D の共有部分)"
```

この PR を先に develop へ入れる。

---

### Task 2: GitHub の API の問い合わせ(infra/github)

GitHub の REST API に問い合わせる部分と、応答を失敗の種類に分ける部分を作る。最初の操作は `GET /user`(`Viewer`)だけ。httptest の偽のサーバーで確かめる。

**Files:**
- Create: `internal/infra/github/client.go`、`internal/infra/github/viewer.go`
- Test: `internal/infra/github/client_test.go`

**Interfaces:**
- Consumes: Task 1 の `ghcall.Failure`・`port.GitHub`
- Produces:
  - `github.Client{BaseURL string; HTTP *http.Client; Now func() time.Time; OnSkew func(time.Duration)}`
  - `github.New(onSkew func(time.Duration)) *github.Client` — `https://api.github.com`、1 回 30 秒で打ち切り
  - `(*Client) Viewer(ctx, token) (string, error)`(`port.GitHub`)
  - (パッケージ内)`(*Client) get(ctx, token, path string, out any) error` — 以後の操作はこれを使う

**失敗の分け方:** 送れない・30 秒たった・5xx・応答が読めない → `unreachable`。401 → `token_rejected`。429、または 403 で `X-RateLimit-Remaining: 0` か `Retry-After` がある → `rate_limited`(`Retry-After` の秒数があれば今の時刻に足す、なければ `X-RateLimit-Reset` の時刻、どちらもなければゼロ)。そのほかの 403 → `forbidden`。404 → `not_found`。

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/github/client_test.go`:

```go
package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

const fakeToken = "fake-token"

var now = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

func newClient(t *testing.T, h http.HandlerFunc) (*Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	var skews []time.Duration
	c := New(func(d time.Duration) { skews = append(skews, d) })
	c.BaseURL = srv.URL
	c.Now = func() time.Time { return now }
	return c, &skews
}

func TestViewer(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" || r.Header.Get("Authorization") != "Bearer "+fakeToken ||
			r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"login":"taro","id":1}`))
	})
	login, err := c.Viewer(context.Background(), fakeToken)
	if err != nil || login != "taro" {
		t.Errorf("Viewer = %q, %v", login, err)
	}
}

func TestFailures(t *testing.T) {
	cases := []struct {
		name  string
		h     http.HandlerFunc
		kind  ghcall.Kind
		reset time.Time
	}{
		{"401", status(401, nil), ghcall.TokenRejected, time.Time{}},
		{"403 permission", status(403, nil), ghcall.Forbidden, time.Time{}},
		{"404", status(404, nil), ghcall.NotFound, time.Time{}},
		{"500", status(500, nil), ghcall.Unreachable, time.Time{}},
		{"rate limit with reset", status(403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1794484800"}), ghcall.RateLimited, time.Unix(1794484800, 0)},
		{"retry after seconds", status(429, map[string]string{"Retry-After": "90"}), ghcall.RateLimited, now.Add(90 * time.Second)},
		{"rate limit unknown time", status(429, nil), ghcall.RateLimited, time.Time{}},
		{"broken json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) }, ghcall.Unreachable, time.Time{}},
	}
	for _, c := range cases {
		client, _ := newClient(t, c.h)
		_, err := client.Viewer(context.Background(), fakeToken)
		var f *ghcall.Failure
		if !ghcall.Is(err, c.kind) {
			t.Errorf("%s: err = %v, want %s", c.name, err, c.kind)
			continue
		}
		f = err.(*ghcall.Failure)
		if !f.ResetAt.Equal(c.reset) {
			t.Errorf("%s: ResetAt = %v, want %v", c.name, f.ResetAt, c.reset)
		}
		if strings.Contains(err.Error(), fakeToken) {
			t.Errorf("%s: the error shows the token", c.name)
		}
	}
}

func TestTimeoutIsUnreachable(t *testing.T) {
	release := make(chan struct{})
	c, _ := newClient(t, func(http.ResponseWriter, *http.Request) { <-release })
	defer close(release)
	c.HTTP.Timeout = 50 * time.Millisecond
	_, err := c.Viewer(context.Background(), fakeToken)
	if !ghcall.Is(err, ghcall.Unreachable) || strings.Contains(err.Error(), fakeToken) {
		t.Errorf("err = %v", err)
	}
}

func TestSkewIsReportedOnEveryAnswer(t *testing.T) {
	c, skews := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Date", now.Add(6*time.Minute).Format(http.TimeFormat))
		http.Error(w, "", http.StatusUnauthorized)
	})
	_, _ = c.Viewer(context.Background(), fakeToken)
	if len(*skews) != 1 || (*skews)[0] != 6*time.Minute {
		t.Errorf("skews = %v", *skews)
	}
}

func TestDefaults(t *testing.T) {
	c := New(nil)
	if c.BaseURL != "https://api.github.com" || c.HTTP.Timeout != 30*time.Second {
		t.Errorf("New = %+v", c)
	}
}

func status(code int, headers map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		http.Error(w, `{"message":"no"}`, code)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/github/`
Expected: FAIL(`undefined: New`)

- [ ] **Step 3: 実装する**

`internal/infra/github/client.go`:

```go
// Package github は、GitHub の REST API への問い合わせ(port.GitHub の実装)を持つ。
// トークンは Authorization の見出しにだけ載せ、エラーの文には書かない。
package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

const (
	apiURL     = "https://api.github.com"
	apiVersion = "2022-11-28"
	// timeout は 1 回の問い合わせの打ち切り(07「GitHub との通信に失敗したとき」)。
	timeout = 30 * time.Second
	// maxBody は、読む応答の大きさの上限。
	maxBody = 10 << 20
)

// Client は GitHub の API に問い合わせる。
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Now     func() time.Time
	// OnSkew は、応答の Date からこのマシンの時計を引いたずれを、応答のたびに受け取る(トラック C が記録する)。
	OnSkew func(time.Duration)
}

func New(onSkew func(time.Duration)) *Client {
	return &Client{BaseURL: apiURL, HTTP: &http.Client{Timeout: timeout}, Now: time.Now, OnSkew: onSkew}
}

// get は、path を GET して、成功なら本文を out に読む。失敗は *ghcall.Failure。
func (c *Client) get(ctx context.Context, token, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	res, err := c.HTTP.Do(req)
	if err != nil {
		// net/http のエラーは URL を含むことがあるので、種類だけを返す。
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	defer res.Body.Close()
	c.observeSkew(res.Header.Get("Date"))
	if f := c.classify(res); f != nil {
		return f
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
		return &ghcall.Failure{Kind: ghcall.Unreachable}
	}
	return nil
}

// classify は、成功でない応答を失敗の種類に分ける。成功なら nil。
func (c *Client) classify(res *http.Response) *ghcall.Failure {
	code := res.StatusCode
	limited := code == http.StatusTooManyRequests ||
		(code == http.StatusForbidden && (res.Header.Get("X-RateLimit-Remaining") == "0" || res.Header.Get("Retry-After") != ""))
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusUnauthorized:
		return &ghcall.Failure{Kind: ghcall.TokenRejected}
	case limited:
		return &ghcall.Failure{Kind: ghcall.RateLimited, ResetAt: c.resetAt(res.Header)}
	case code == http.StatusForbidden:
		return &ghcall.Failure{Kind: ghcall.Forbidden}
	case code == http.StatusNotFound:
		return &ghcall.Failure{Kind: ghcall.NotFound}
	}
	return &ghcall.Failure{Kind: ghcall.Unreachable}
}

// resetAt は、利用上限が戻る時刻。秒数(Retry-After)があれば今に足し、なければ X-RateLimit-Reset の時刻。
// どちらも分からなければゼロ(07「GitHub との通信に失敗したとき」)。
func (c *Client) resetAt(h http.Header) time.Time {
	if n, err := strconv.Atoi(h.Get("Retry-After")); err == nil {
		return c.Now().Add(time.Duration(n) * time.Second)
	}
	if n, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		return time.Unix(n, 0)
	}
	return time.Time{}
}

func (c *Client) observeSkew(date string) {
	if c.OnSkew == nil || date == "" {
		return
	}
	t, err := http.ParseTime(date)
	if err != nil {
		return
	}
	c.OnSkew(t.Sub(c.Now()))
}
```

`internal/infra/github/viewer.go`:

```go
package github

import "context"

// Viewer は、token の持ち主のログイン名を返す(GET /user)。
func (c *Client) Viewer(ctx context.Context, token string) (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	if err := c.get(ctx, token, "/user", &u); err != nil {
		return "", err
	}
	return u.Login, nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/infra/github/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件

- [ ] **Step 5: Commit**

```bash
git add internal/infra/github
git commit -m "feat(infra): GitHub の API の問い合わせと失敗の分け方を足す"
```

---

### Task 3: トークンの読み出し(infra/secret)

gh CLI・環境変数 `GH_TOKEN`・OS のキーチェーンから、アカウントのトークンを読む。読めないときは 07「認証の確認」の表の種類で返す。

**Files:**
- Create: `internal/infra/secret/source.go`、`internal/infra/secret/keyring.go`
- Test: `internal/infra/secret/source_test.go`

**Interfaces:**
- Consumes: Task 1 の `account.Account`・`ghcall.Failure`・`port.Secrets`
- Produces:
  - `secret.Source{Getenv func(string) string; Run func(ctx, name string, args ...string) ([]byte, error); Keyring Keyring; GhTimeout time.Duration}`
  - `secret.New() *secret.Source` — `os.Getenv`、`exec`、OS のキーチェーン、gh は 10 秒
  - `secret.Keyring{ Get(service, user string) (string, error) }`
  - `secret.Service = "hedgehug"`、`secret.KeychainKey(login string) string`(`github.com/<ログイン名>`)
  - `(*Source) Token(ctx, a) (string, error)`(`port.Secrets`)。デバイスフローは D2 で足す

- [ ] **Step 1: 依存を入れる**

```bash
go get github.com/zalando/go-keyring@v0.2.8
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/infra/secret/source_test.go`:

```go
package secret

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

type fakeKeyring map[string]string

func (k fakeKeyring) Get(service, user string) (string, error) {
	if k == nil {
		return "", errors.New("no secret service on this machine")
	}
	v, ok := k[service+"|"+user]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func acc(m account.Method) account.Account {
	return account.Account{ID: "A1", Name: "work", Login: "taro", Method: m}
}

func TestGhToken(t *testing.T) {
	var gotArgs []string
	s := &Source{GhTimeout: time.Second, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotArgs = append([]string{name}, args...)
		return []byte(" gho_abc \n"), nil
	}}
	tok, err := s.Token(context.Background(), acc(account.GhCLI))
	want := []string{"gh", "auth", "token", "--hostname", "github.com", "--user", "taro"}
	if err != nil || tok != "gho_abc" || !slices.Equal(gotArgs, want) {
		t.Errorf("Token = %q, %v; args %v", tok, err, gotArgs)
	}
}

func TestGhFailures(t *testing.T) {
	missing := &Source{GhTimeout: time.Second, Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("exit status 1: no oauth token for gho_LEAK")
	}}
	_, err := missing.Token(context.Background(), acc(account.GhCLI))
	if !ghcall.Is(err, ghcall.GhTokenMissing) || strings.Contains(err.Error(), "gho_LEAK") {
		t.Errorf("missing: %v", err)
	}
	empty := &Source{GhTimeout: time.Second, Run: func(context.Context, string, ...string) ([]byte, error) { return []byte("\n"), nil }}
	if _, err := empty.Token(context.Background(), acc(account.GhCLI)); !ghcall.Is(err, ghcall.GhTokenMissing) {
		t.Errorf("empty: %v", err)
	}
	hung := &Source{GhTimeout: 20 * time.Millisecond, Run: func(ctx context.Context, string, ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if _, err := hung.Token(context.Background(), acc(account.GhCLI)); !ghcall.Is(err, ghcall.GhTimeout) {
		t.Errorf("hung: %v", err)
	}
}

func TestEnvToken(t *testing.T) {
	env := map[string]string{"GH_TOKEN": " ghp_env "}
	s := &Source{Getenv: func(k string) string { return env[k] }}
	if tok, err := s.Token(context.Background(), acc(account.EnvVar)); err != nil || tok != "ghp_env" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	env["GH_TOKEN"] = "  "
	if _, err := s.Token(context.Background(), acc(account.EnvVar)); !ghcall.Is(err, ghcall.EnvMissing) {
		t.Errorf("unset: %v", err)
	}
}

func TestKeychainToken(t *testing.T) {
	s := &Source{Keyring: fakeKeyring{"hedgehug|github.com/taro": "ghp_kc"}}
	if tok, err := s.Token(context.Background(), acc(account.Keychain)); err != nil || tok != "ghp_kc" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	s.Keyring = fakeKeyring{"hedgehug|github.com/taro": " "}
	if _, err := s.Token(context.Background(), acc(account.Keychain)); !ghcall.Is(err, ghcall.KeychainMissing) {
		t.Errorf("blank: %v", err)
	}
	s.Keyring = fakeKeyring{}
	if _, err := s.Token(context.Background(), acc(account.Keychain)); !ghcall.Is(err, ghcall.KeychainMissing) {
		t.Errorf("absent: %v", err)
	}
	s.Keyring = fakeKeyring(nil)
	if _, err := s.Token(context.Background(), acc(account.Keychain)); !ghcall.Is(err, ghcall.KeychainUnavailable) {
		t.Errorf("no keychain: %v", err)
	}
}

func TestDeviceFlowIsNotReadHere(t *testing.T) {
	if _, err := (&Source{}).Token(context.Background(), acc(account.DeviceFlow)); err == nil {
		t.Error("device flow token read in D1")
	}
}
```

- [ ] **Step 3: テストが落ちることを確かめる**

Run: `go test ./internal/infra/secret/`
Expected: FAIL(`undefined: Source`)

- [ ] **Step 4: 実装する**

`internal/infra/secret/source.go`:

```go
// Package secret は、アカウントのトークンを、その認証方法の置き場所から読む(port.Secrets の実装)。
// トークンと、gh の出力はエラーの文に書かない。
package secret

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Service は、キーチェーンでトークンを置くサービスの名前(前回と同じ)。
const Service = "hedgehug"

// ghTimeout は、gh CLI の応答を待つ時間(07「認証の確認」)。
const ghTimeout = 10 * time.Second

// KeychainKey は、キーチェーンのアカウントの名前(github.com/<ログイン名>)。
func KeychainKey(login string) string { return account.Host + "/" + login }

// Keyring は OS のキーチェーン。テストでは差し替える。
type Keyring interface {
	Get(service, user string) (string, error)
}

// Source はトークンの読み出し。
type Source struct {
	Getenv    func(string) string
	Run       func(ctx context.Context, name string, args ...string) ([]byte, error)
	Keyring   Keyring
	GhTimeout time.Duration
}

func New() *Source {
	return &Source{Getenv: os.Getenv, Run: run, Keyring: osKeyring{}, GhTimeout: ghTimeout}
}

// Token は、アカウントの認証方法でトークンを読む。
func (s *Source) Token(ctx context.Context, a account.Account) (string, error) {
	switch a.Method {
	case account.GhCLI:
		return s.ghToken(ctx, a.Login)
	case account.EnvVar:
		if v := strings.TrimSpace(s.Getenv("GH_TOKEN")); v != "" {
			return v, nil
		}
		return "", &ghcall.Failure{Kind: ghcall.EnvMissing}
	case account.Keychain:
		v, err := s.Keyring.Get(Service, KeychainKey(a.Login))
		switch {
		case err != nil && !errors.Is(err, ErrNotFound):
			// 見つからない以外の失敗は、キーチェーンそのものが使えない(WSL など。前回の実装と同じ扱い)。
			return "", &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
		case err != nil || strings.TrimSpace(v) == "":
			return "", &ghcall.Failure{Kind: ghcall.KeychainMissing, Login: a.Login}
		}
		return strings.TrimSpace(v), nil
	}
	return "", fmt.Errorf("no token source for the method %q", a.Method)
}

func (s *Source) ghToken(ctx context.Context, login string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.GhTimeout)
	defer cancel()
	out, err := s.Run(ctx, "gh", "auth", "token", "--hostname", account.Host, "--user", login)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", &ghcall.Failure{Kind: ghcall.GhTimeout}
	}
	tok := strings.TrimSpace(string(out))
	if err != nil || tok == "" {
		return "", &ghcall.Failure{Kind: ghcall.GhTokenMissing}
	}
	return tok, nil
}

// run はコマンドを実行し、標準出力を返す。標準エラー出力はトークンを含むことがあるので読まない。
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output() //nolint:gosec // 実行するのは gh だけで、引数はこのパッケージが決める
}
```

`internal/infra/secret/keyring.go`:

```go
package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// ErrNotFound は、キーチェーンに項目がないこと。ほかの失敗は、キーチェーンそのものが使えないこと。
var ErrNotFound = errors.New("not in the keychain")

// osKeyring は OS のキーチェーン(macOS のキーチェーン、Linux の Secret Service)。
type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error) {
	v, err := keyring.Get(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/infra/secret/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/infra/secret
git commit -m "feat(infra): gh・環境変数・キーチェーンからトークンを読む"
```

---

### Task 4: アカウントの一覧の保存(infra/machineconfig)

アカウントの一覧(トークンを除く)を `~/.config/hedgehug/config.yaml` の `accounts` に置く(保存形式「このマシンに持つもの」)。未決の点 1 の推奨案で書く。答えが違えば、このタスクだけを置き場所に合わせて直す。

**Files:**
- Create: `internal/infra/machineconfig/file.go`、`internal/infra/machineconfig/accounts.go`
- Test: `internal/infra/machineconfig/accounts_test.go`

**Interfaces:**
- Consumes: Task 1 の `port.Accounts`・`apptest.RunAccountsContract`
- Produces:
  - `machineconfig.Open(path string) *machineconfig.File`
  - `(*File) Accounts() port.Accounts`
  - (パッケージ内)`config` 構造体。トラック G が、Store の一覧などの項目を同じ構造体に足す

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/machineconfig/accounts_test.go`:

```go
package machineconfig

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestContract(t *testing.T) {
	apptest.RunAccountsContract(t, func(t *testing.T) port.Accounts {
		return Open(filepath.Join(t.TempDir(), "hedgehug", "config.yaml")).Accounts()
	})
}

func TestPersistsWithoutTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hedgehug", "config.yaml")
	a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.Keychain}
	if err := Open(path).Accounts().Add(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	got, err := Open(path).Accounts().List(context.Background())
	if err != nil || len(got) != 1 || got[0] != a {
		t.Errorf("reopened List = %+v, %v", got, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "accounts:\n  - id: 01ACC00000000000000000001\n    name: work\n    login: taro\n    method: keychain\n"
	if string(b) != want {
		t.Errorf("config.yaml =\n%s\nwant\n%s", b, want)
	}
	if strings.Contains(string(b), "token") {
		t.Error("config.yaml has a token field")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, %v", info.Mode().Perm(), err)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/machineconfig/`
Expected: FAIL(`undefined: Open`)

- [ ] **Step 3: 実装する**

`internal/infra/machineconfig/file.go`:

```go
// Package machineconfig は、このマシンの設定(~/.config/hedgehug/config.yaml)を持つ。
// トークンはここに置かない(OS のキーチェーンなど。infra/secret)。
package machineconfig

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"go.yaml.in/yaml/v4"
)

// config は config.yaml の中身。項目は、持つトラックが足す(トラック G が Store の一覧などを足す)。
type config struct {
	Accounts []accountYAML `yaml:"accounts"`
}

// File は config.yaml。同じプロセスの中の読み書きは 1 つずつ行う。
type File struct {
	path string
	mu   sync.Mutex
}

func Open(path string) *File { return &File{path: path} }

// update は、config.yaml を読み、fn で変えて書く。fn がエラーなら書かない。
func (f *File) update(fn func(c *config) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, err := f.load()
	if err != nil {
		return err
	}
	if err := fn(&c); err != nil {
		return err
	}
	return f.save(c)
}

func (f *File) read() (config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.load()
}

func (f *File) load() (config, error) {
	var c config
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, yaml.Unmarshal(b, &c)
}

// save は、同じフォルダの一時ファイルに書いてから置き換える(途中で止まっても壊れた設定を残さない)。
func (f *File) save(c config) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.path)
}
```

`internal/infra/machineconfig/accounts.go`:

```go
package machineconfig

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

type accountYAML struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Login  string `yaml:"login"`
	Method string `yaml:"method"`
}

// Accounts は、config.yaml の accounts を読み書きする port.Accounts。
func (f *File) Accounts() port.Accounts { return accounts{f} }

type accounts struct{ f *File }

func (a accounts) List(context.Context) ([]account.Account, error) {
	c, err := a.f.read()
	if err != nil {
		return nil, err
	}
	out := make([]account.Account, 0, len(c.Accounts))
	for _, y := range c.Accounts {
		out = append(out, account.Account{ID: account.ID(y.ID), Name: y.Name, Login: y.Login, Method: account.Method(y.Method)})
	}
	return out, nil
}

func (a accounts) Add(_ context.Context, acc account.Account) error {
	return a.f.update(func(c *config) error {
		c.Accounts = append(c.Accounts, accountYAML{ID: string(acc.ID), Name: acc.Name, Login: acc.Login, Method: string(acc.Method)})
		return nil
	})
}
```

`os.CreateTemp` は 0o600 でファイルを作るので、置き換えたあとの config.yaml も 0o600 になる。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/infra/machineconfig/ && make lint && go test ./internal/archtest/`
Expected: PASS。YAML の書き出しが引用符の付け方だけで `want` と違えば、書き出しに合わせて `want` を直してよい。

- [ ] **Step 5: Commit**

```bash
git add internal/infra/machineconfig
git commit -m "feat(infra): アカウントの一覧を config.yaml に置く"
```

---

### Task 5: アカウントの登録と一覧の usecase(app/github)

07「登録」「認証の確認」の流れを usecase にする。入力の検査 → トークンを読む → GitHub に問い合わせる → ログイン名を照らす → 登録、の順で、どこかで断ったら何も登録しない。利用者に返す失敗は、`ghcall.Refuse` で 07 の文言の断り(`refusal`)に直す。

**Files:**
- Create: `internal/app/github/add_account.go`、`internal/app/github/list_accounts.go`
- Test: `internal/app/github/accounts_test.go`

**Interfaces:**
- Consumes: Task 1 の口と偽物と `ghcall.Refuse`、A1 Task 1 の `refusal.Refusal`、土台の `port.IDs`・`port.Clock`・`apptest.NewIDs`・`apptest.NewClock`
- Produces:
  - `github.AddAccount{Accounts port.Accounts; Secrets port.Secrets; GitHub port.GitHub; IDs port.IDs; Clock port.Clock}`
  - `github.AddAccountInput{Name, Login string; Method account.Method}`
  - `(AddAccount) Do(ctx, in AddAccountInput) (account.Account, error)` — 断るときは `*refusal.Refusal`(文言は 07 の表)。デバイスフローは `ErrDeviceFlow`(D2 のサインインで登録する)
  - `github.ListAccounts{Accounts port.Accounts}`、`(ListAccounts) Do(ctx) ([]account.Account, error)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/github/accounts_test.go`:

```go
package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

type fixture struct {
	accounts *apptest.Accounts
	secrets  *apptest.Secrets
	gh       *apptest.GitHub
	add      github.AddAccount
}

func newFixture() fixture {
	f := fixture{
		accounts: apptest.NewAccounts(),
		secrets:  &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro", "jiro": "tok-jiro"}},
		gh:       &apptest.GitHub{Users: map[string]string{"tok-taro": "Taro", "tok-jiro": "hanako"}},
	}
	f.add = github.AddAccount{
		Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh,
		IDs: apptest.NewIDs(), Clock: apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)),
	}
	return f
}

func (f fixture) count(t *testing.T) int {
	t.Helper()
	list, err := f.accounts.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return len(list)
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func TestAddAccount(t *testing.T) {
	f := newFixture()
	got, err := f.add.Do(context.Background(), github.AddAccountInput{Name: " work ", Login: "taro", Method: account.Keychain})
	if err != nil {
		t.Fatal(err)
	}
	// GitHub のログイン名は大文字と小文字を区別しないので、Taro と taro は同じ人。
	if got.ID == "" || got.Name != "work" || got.Login != "taro" || f.count(t) != 1 {
		t.Errorf("added %+v, count %d", got, f.count(t))
	}
	list, _ := github.ListAccounts{Accounts: f.accounts}.Do(context.Background())
	if len(list) != 1 || list[0] != got {
		t.Errorf("ListAccounts = %+v", list)
	}
}

func TestAddAccountRefusesAndAddsNothing(t *testing.T) {
	cases := []struct {
		name  string
		in    github.AddAccountInput
		setup func(f fixture)
		want  string
	}{
		{"input", github.AddAccountInput{Name: "", Login: "taro", Method: account.GhCLI}, nil,
			"表示名とログイン名を入力してください"},
		{"no token", github.AddAccountInput{Name: "w", Login: "saburo", Method: account.Keychain}, nil,
			"OS のキーチェーンに、サービス hedgehug・アカウント github.com/saburo のトークンが見つかりません"},
		{"token rejected", github.AddAccountInput{Name: "w", Login: "taro", Method: account.GhCLI},
			func(f fixture) { f.gh.Users = map[string]string{} },
			"トークンが GitHub に受け付けられませんでした。認証方法を確かめてください"},
		{"unreachable", github.AddAccountInput{Name: "w", Login: "taro", Method: account.GhCLI},
			func(f fixture) { f.gh.Err = &ghcall.Failure{Kind: ghcall.Unreachable} },
			"GitHub に接続できません。接続を確かめて、もう一度お試しください"},
		{"login mismatch", github.AddAccountInput{Name: "w", Login: "jiro", Method: account.EnvVar}, nil,
			"このトークンは @hanako のものです。ログイン名を直してください"},
	}
	for _, c := range cases {
		f := newFixture()
		if c.setup != nil {
			c.setup(f)
		}
		_, err := f.add.Do(context.Background(), c.in)
		if got := message(err); got != c.want || f.count(t) != 0 {
			t.Errorf("%s: message %q (err %v), count %d; want %q", c.name, got, err, f.count(t), c.want)
		}
	}
}

func TestAddAccountDeviceFlowIsSeparate(t *testing.T) {
	f := newFixture()
	_, err := f.add.Do(context.Background(), github.AddAccountInput{Name: "w", Login: "taro", Method: account.DeviceFlow})
	if !errors.Is(err, github.ErrDeviceFlow) || f.count(t) != 0 {
		t.Errorf("err = %v", err)
	}
}

func TestAddAccountChecksDuplicatesBeforeAskingGitHub(t *testing.T) {
	f := newFixture()
	if _, err := f.add.Do(context.Background(), github.AddAccountInput{Name: "work", Login: "taro", Method: account.GhCLI}); err != nil {
		t.Fatal(err)
	}
	f.gh.Err = errors.New("must not be called")
	_, err := f.add.Do(context.Background(), github.AddAccountInput{Name: "WORK", Login: "jiro", Method: account.GhCLI})
	if got := message(err); got != "同じ表示名のアカウントがあります" {
		t.Errorf("err = %v", err)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/github/`
Expected: FAIL(`undefined: github.AddAccount`)

- [ ] **Step 3: 実装する**

`internal/app/github/add_account.go`:

```go
// Package github は、GitHub 連携の usecase(仕様 07)を持つ。
package github

import (
	"context"
	"errors"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// ErrDeviceFlow は、デバイスフローのアカウントをこの操作で登録しようとしたとき(D2 のサインインで登録する)。
var ErrDeviceFlow = errors.New("a device flow account is added by signing in")

// AddAccount は、アカウントを登録する(07「登録」「認証の確認」)。GitHub で本人を確かめられたときだけ登録する。
// Store を変えないので、保存もアクティビティも作らない。
type AddAccount struct {
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	IDs      port.IDs
	Clock    port.Clock
}

type AddAccountInput struct {
	Name   string
	Login  string
	Method account.Method
}

func (u AddAccount) Do(ctx context.Context, in AddAccountInput) (account.Account, error) {
	a, err := u.verify(ctx, in)
	if err != nil {
		return account.Account{}, ghcall.Refuse(err, u.Clock.Now())
	}
	a.ID = account.ID(u.IDs.New())
	if err := u.Accounts.Add(ctx, a); err != nil {
		return account.Account{}, err
	}
	return a, nil
}

// verify は、入力を確かめ、トークンを読み、GitHub でログイン名を照らす。
func (u AddAccount) verify(ctx context.Context, in AddAccountInput) (account.Account, error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	a, err := account.Check(account.Account{Name: in.Name, Login: in.Login, Method: in.Method}, existing, "")
	if err != nil {
		return account.Account{}, err
	}
	if a.Method == account.DeviceFlow {
		return account.Account{}, ErrDeviceFlow
	}
	token, err := u.Secrets.Token(ctx, a)
	if err != nil {
		return account.Account{}, err
	}
	login, err := u.GitHub.Viewer(ctx, token)
	if err != nil {
		return account.Account{}, err
	}
	if !strings.EqualFold(login, a.Login) {
		return account.Account{}, &ghcall.Failure{Kind: ghcall.LoginMismatch, Login: login}
	}
	return a, nil
}
```

`internal/app/github/list_accounts.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// ListAccounts は、登録したアカウントを登録した順に返す。
type ListAccounts struct {
	Accounts port.Accounts
}

func (u ListAccounts) Do(ctx context.Context) ([]account.Account, error) {
	return u.Accounts.List(ctx)
}
```

`app/github` と `infra/github` は同じパッケージ名だが、互いに import しない(archtest が確かめる)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/app/github/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS(app/github は domain と port だけを import する)

- [ ] **Step 5: Commit**

```bash
git add internal/app/github
git commit -m "feat(app): GitHub のアカウントを確かめてから登録する usecase を足す"
```

---

### Task 6: アカウントの API(httpapi/github)

`GET /api/accounts` と `POST /api/accounts` を作る。エラーは A1 Task 1 の `apierr.From` で直す(断りは 422 で `detail` に 07 の文言)。

**Files:**
- Create: `internal/adapter/httpapi/github/accounts.go`
- Test: `internal/adapter/httpapi/github/accounts_test.go`

**Interfaces:**
- Consumes: Task 5 の `AddAccount`・`ListAccounts`、A1 Task 1 の `apierr.From(err error) error`
- Produces:
  - `github.Register(api huma.API, list ghapp.ListAccounts, add ghapp.AddAccount)`(`ghapp` は `internal/app/github`)
  - `GET /api/accounts`(operationId `list-accounts`): `{"accounts": [{"id","name","login","method"}]}`
  - `POST /api/accounts`(operationId `add-account`): 本文 `{"name","login","method"}`(`method` は `gh`・`env`・`keychain`)、成功は 201 とアカウント。断りは 422 で `detail` に文言

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/github/accounts_test.go`:

```go
package github_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func newAPI(t *testing.T, gh *apptest.GitHub) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	accounts := apptest.NewAccounts()
	secrets := &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}}
	ghapi.Register(api,
		ghapp.ListAccounts{Accounts: accounts},
		ghapp.AddAccount{Accounts: accounts, Secrets: secrets, GitHub: gh, IDs: apptest.NewIDs(),
			Clock: apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC))})
	return api
}

func TestAddAndList(t *testing.T) {
	api := newAPI(t, &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}})
	res := api.Post("/api/accounts", map[string]string{"name": "work", "login": "taro", "method": "gh"})
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"login":"taro"`) {
		t.Fatalf("POST: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts")
	var body struct {
		Accounts []struct{ ID, Name, Login, Method string }
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Accounts) != 1 || body.Accounts[0].Method != "gh" || body.Accounts[0].ID == "" {
		t.Errorf("GET: %s", res.Body)
	}
}

func TestRefusalsAre422WithTheSpecText(t *testing.T) {
	cases := []struct {
		name string
		gh   *apptest.GitHub
		body map[string]string
		want string
	}{
		{"input", &apptest.GitHub{}, map[string]string{"name": " ", "login": "taro", "method": "gh"}, "表示名とログイン名を入力してください"},
		{"mismatch", &apptest.GitHub{Users: map[string]string{"tok-taro": "jiro"}}, map[string]string{"name": "w", "login": "taro", "method": "gh"}, "このトークンは @jiro のものです。ログイン名を直してください"},
		{"unreachable", &apptest.GitHub{Err: &ghcall.Failure{Kind: ghcall.Unreachable}}, map[string]string{"name": "w", "login": "taro", "method": "gh"}, "GitHub に接続できません。接続を確かめて、もう一度お試しください"},
	}
	for _, c := range cases {
		res := newAPI(t, c.gh).Post("/api/accounts", c.body)
		var body struct{ Detail string }
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		if res.Code != http.StatusUnprocessableEntity || body.Detail != c.want {
			t.Errorf("%s: %d %s, want detail %q", c.name, res.Code, res.Body, c.want)
		}
	}
}

func TestDeviceFlowIsNotAcceptedHere(t *testing.T) {
	res := newAPI(t, &apptest.GitHub{}).Post("/api/accounts", map[string]string{"name": "w", "login": "taro", "method": "device"})
	if res.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d", res.Code)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/github/`
Expected: FAIL(`undefined: ghapi.Register`)

- [ ] **Step 3: 実装する**

`internal/adapter/httpapi/github/accounts.go`:

```go
// Package github は、GitHub 連携の API(仕様 07)を持つ。
package github

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/account"
)

type accountBody struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Login  string `json:"login"`
	Method string `json:"method" enum:"device,gh,env,keychain"`
}

func toBody(a account.Account) accountBody {
	return accountBody{ID: string(a.ID), Name: a.Name, Login: a.Login, Method: string(a.Method)}
}

type listOutput struct {
	Body struct {
		Accounts []accountBody `json:"accounts"`
	}
}

type addInput struct {
	Body struct {
		Name   string `json:"name"`
		Login  string `json:"login"`
		Method string `json:"method" enum:"gh,env,keychain" doc:"デバイスフローはサインインの API で登録する"`
	}
}

type addOutput struct {
	Body accountBody
}

// Register は、アカウントの API を登録する。
func Register(api huma.API, list ghapp.ListAccounts, add ghapp.AddAccount) {
	huma.Register(api, huma.Operation{
		OperationID: "list-accounts",
		Method:      http.MethodGet,
		Path:        "/api/accounts",
		Summary:     "登録したアカウントを返す",
	}, func(ctx context.Context, _ *struct{}) (*listOutput, error) {
		accs, err := list.Do(ctx)
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &listOutput{}
		out.Body.Accounts = make([]accountBody, 0, len(accs))
		for _, a := range accs {
			out.Body.Accounts = append(out.Body.Accounts, toBody(a))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID:   "add-account",
		Method:        http.MethodPost,
		Path:          "/api/accounts",
		Summary:       "GitHub で確かめてからアカウントを登録する",
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *addInput) (*addOutput, error) {
		a, err := add.Do(ctx, ghapp.AddAccountInput{Name: in.Body.Name, Login: in.Body.Login, Method: account.Method(in.Body.Method)})
		if err != nil {
			return nil, apierr.From(err)
		}
		return &addOutput{Body: toBody(a)}, nil
	})
}
```

`method` が `device` の本文は、huma の enum の検査で 422 になる。画面は、`gh`・`env`・`keychain` だけをこの API に送る。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/adapter/httpapi/github/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS(adapter は app・domain・adapter/httpapi の中と huma だけ)

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/httpapi/github
git commit -m "feat(adapter): アカウントの一覧と登録の API を足す"
```

---

### Task 7: 組み立て(共有の小さな PR)

本物と e2e 用の組み立てに、D の部品を差し込む。生成物を作り直す。この PR は、組み立てと生成物だけを変える。`IDs` は A1 Task 1 が `httpapi.Deps` に足したものを使う。

**Files:**
- Modify: `internal/adapter/httpapi/routes.go`(`Deps` に 4 つのフィールド、Register を 1 行)
- Modify: `cmd/hedgehug/wire.go`、`cmd/hedgehug/wire_e2e.go`
- Create: `cmd/hedgehug/paths.go`
- Modify(生成): `api/openapi.yaml`、`web/packages/api/src/generated.ts`
- Test: `internal/adapter/httpapi/server_test.go`(1 件足す)

**Interfaces:**
- Consumes: Task 2〜6 のすべて、A1 Task 1 の `httpapi.Deps{Version; Stores; IDs; Identity}`、土台の `sysclock.Clock`
- Produces: `httpapi.Deps{…; Accounts port.Accounts; Secrets port.Secrets; GitHub port.GitHub; Clock port.Clock}`、`configPath() (string, error)`(`$XDG_CONFIG_HOME/hedgehug/config.yaml`、なければ `~/.config/hedgehug/config.yaml`)

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/server_test.go` に足す(import に `"time"` と `"github.com/kukv/hedgehug/internal/app/apptest"` を足す):

```go
func TestAccountsAreRegistered(t *testing.T) {
	srv := httptest.NewServer(httpapi.New("secret", fstest.MapFS{}, httpapi.Deps{
		Version:  "test",
		IDs:      apptest.NewIDs(),
		Accounts: apptest.NewAccounts(),
		Secrets:  &apptest.Secrets{},
		GitHub:   &apptest.GitHub{},
		Clock:    apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)),
	}))
	t.Cleanup(srv.Close)
	code, body := get(t, srv.URL+"/api/accounts", "secret")
	if code != http.StatusOK || !strings.Contains(body, `"accounts":[]`) {
		t.Errorf("status %d, body %s", code, body)
	}
}
```

Run: `go test ./internal/adapter/httpapi/`
Expected: FAIL(`unknown field Accounts in struct literal`)

- [ ] **Step 2: 組み立てを書く**

`internal/adapter/httpapi/routes.go` の `Deps` に足す:

```go
	// トラック D(GitHub)
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Clock    port.Clock
```

`NewAPI` の登録の並びに 1 行足す(import は `ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"` と `ghapp "github.com/kukv/hedgehug/internal/app/github"`):

```go
	ghapi.Register(api,
		ghapp.ListAccounts{Accounts: deps.Accounts},
		ghapp.AddAccount{Accounts: deps.Accounts, Secrets: deps.Secrets, GitHub: deps.GitHub, IDs: deps.IDs, Clock: deps.Clock})
```

`cmd/openapi-gen/main.go` は変えない(登録は値を呼ばないので、空の口でも OpenAPI は作れる)。

`cmd/hedgehug/paths.go`:

```go
package main

import (
	"os"
	"path/filepath"
)

// configPath は config.yaml の場所(保存形式「このマシンに持つもの」)。XDG_CONFIG_HOME があればその下、
// なければ ~/.config の下。
func configPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "hedgehug", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "hedgehug", "config.yaml"), nil
}
```

`cmd/hedgehug/wire.go` の `wire` を次にする(A1 Task 1 が足した `IDs`・`Identity` はそのまま。import に `infra/github`・`infra/machineconfig`・`infra/secret` を足す):

```go
func wire(context.Context) (httpapi.Deps, error) {
	stores := gitstore.New(sysclock.Clock{}, sysclock.IDs{})
	cfg, err := configPath()
	if err != nil {
		return httpapi.Deps{}, err
	}
	return httpapi.Deps{
		Stores:   stores,
		IDs:      sysclock.IDs{},
		Identity: noIdentity{},
		Accounts: machineconfig.Open(cfg).Accounts(),
		Secrets:  secret.New(),
		GitHub:   github.New(nil), // 時計のずれはトラック C が受け取る(この計画の決定 3)
		Clock:    sysclock.Clock{},
	}, nil
}
```

`cmd/hedgehug/wire_e2e.go` の返す値を次にする(`ids` は A1 Task 1 が取り出した採番):

```go
	return httpapi.Deps{
		Stores:   stores,
		IDs:      ids,
		Identity: apptest.Identity{Login: "taro"},
		Accounts: apptest.NewAccounts(),
		Secrets:  &apptest.Secrets{Tokens: map[string]string{"taro": "e2e-gh-token"}},
		GitHub:   &apptest.GitHub{Users: map[string]string{"e2e-gh-token": "taro"}},
		Clock:    apptest.NewClock(time.Now()),
	}, nil
```

- [ ] **Step 3: 生成物を作り直して、全体を確かめる**

```bash
make generate
cd web && pnpm gen && cd ..
go build ./... && go vet -tags e2e ./cmd/hedgehug
make check
```

Expected: `api/openapi.yaml` に `list-accounts`・`add-account` が増え、`generated.ts` に `listAccounts`・`addAccount` が増える。`make check` がすべて成功。

- [ ] **Step 4: Commit**

```bash
git add internal/adapter/httpapi/routes.go internal/adapter/httpapi/server_test.go cmd/hedgehug api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat: GitHub のアカウントの API を組み立てに差し込む"
```

この PR を先に develop へ入れる。ほかのトラックは rebase のあと `make generate` と `pnpm gen` で生成物を作り直す(設計書 8 節「生成物」)。

---

### Task 8: 本物の GitHub での確認(github-live)

週 1 回と手動で、本物の GitHub に対して `infra/github` を確かめる。D1 では `Viewer` だけを確かめ、D3 以降で操作を足す。

**Files:**
- Create: `internal/infra/github/live_test.go`
- Create: `.github/workflows/github-live.yaml`

**Interfaces:**
- Consumes: Task 2 の `github.New`・`Viewer`
- Produces: ビルドタグ `live` のテスト `TestLive*`。secret `HEDGEHUG_LIVE_TOKEN`、変数 `HEDGEHUG_LIVE_ORG`(どちらも設定済み)

- [ ] **Step 1: テストを書く**

`internal/infra/github/live_test.go`:

```go
//go:build live

package github

import (
	"context"
	"os"
	"strings"
	"testing"
)

// 本物の GitHub に問い合わせる(github-live)。リポジトリに設定済みの secret HEDGEHUG_LIVE_TOKEN と
// 変数 HEDGEHUG_LIVE_ORG(検証用の組織。リポジトリを読む操作は D3 から確かめる)が要る。
func liveToken(t *testing.T) string {
	t.Helper()
	token, org := os.Getenv("HEDGEHUG_LIVE_TOKEN"), os.Getenv("HEDGEHUG_LIVE_ORG")
	if token == "" || org == "" {
		t.Skip("HEDGEHUG_LIVE_TOKEN and HEDGEHUG_LIVE_ORG are not set")
	}
	return token
}

func TestLiveViewer(t *testing.T) {
	token := liveToken(t)
	// トークンの持ち主のログイン名は設定に持たないので、空でない名前が返ることだけを確かめる。
	got, err := New(nil).Viewer(context.Background(), token)
	if err != nil || got == "" {
		t.Errorf("Viewer = %q, %v", got, err)
	}
}

func TestLiveRejectsBadToken(t *testing.T) {
	liveToken(t)
	_, err := New(nil).Viewer(context.Background(), "not-a-real-token")
	if err == nil || !strings.Contains(err.Error(), "token_rejected") {
		t.Errorf("err = %v", err)
	}
}
```

Run: `go vet -tags live ./internal/infra/github/ && go test -tags live ./internal/infra/github/ -run TestLive -v`
Expected: vet が通る。変数がなければ 2 件とも SKIP。

- [ ] **Step 2: ワークフローを書く**

```bash
git show cf58222d~1:.github/workflows/github-live.yaml > .github/workflows/github-live.yaml
```

次だけ変える。
1. 先頭のコメントを次にする。
   ```yaml
   # 本物の GitHub に対して、infra/github の問い合わせが通ることを確かめる(設計書 7 節)。
   # 変数 HEDGEHUG_LIVE_ORG と secret の HEDGEHUG_LIVE_TOKEN が未設定なら、手順を実行せずに成功する。
   # トークンは job の env に置かず、テストの step にだけ渡す。
   ```
2. `HEDGEHUG_LIVE_ON` の条件は変えない(設定済みの `vars.HEDGEHUG_LIVE_ORG` と `secrets.HEDGEHUG_LIVE_TOKEN` を使う)。
3. テストの step の `env` を `HEDGEHUG_LIVE_ORG: ${{ vars.HEDGEHUG_LIVE_ORG }}` と `HEDGEHUG_LIVE_TOKEN: ${{ secrets.HEDGEHUG_LIVE_TOKEN }}` にし、`run` を `go test -tags live -count=1 -run 'TestLive' ./internal/infra/github` にする。

Run: `actionlint .github/workflows/github-live.yaml`
Expected: 指摘なし

- [ ] **Step 3: Commit**

```bash
git add internal/infra/github/live_test.go .github/workflows/github-live.yaml
git commit -m "ci: 本物の GitHub で infra/github を確かめる github-live を足す"
```

secret `HEDGEHUG_LIVE_TOKEN` と変数 `HEDGEHUG_LIVE_ORG` はリポジトリに設定済み(前回の github-live と同じもの)なので、新しい設定は要らない。

---

## D1 のあと

- D2 の計画は `2026-11-13-hedgehug-track-d2.md` に書いた(サーバー)。画面は A2 のあとで書く。GitHub App 未導入は D4b へ移した(D2 の計画の決定 D2-4)。
- D1 の時点では、画面からアカウントを登録する入口はない(API だけ)。

---

## 未決の点

仕様と既存の規則で決まらない点。推奨案で計画を書いた(1)か、その小計画を書く前に答えが要る(2・3)。

| # | 点 | 推奨案 | 要る時期 |
|---|---|---|---|
| 1 | アカウントの一覧の置き場所。設計書 3 節は `MachineSettings` を `infra/statedb` に置き、保存形式の設計は `config.yaml` に置く(土台の計画で伝えた食い違い)。D は G より先にアカウントの一覧が要る | **決定(2026-11-12): `config.yaml` 1 つ。** D1 が `infra/machineconfig` を作り、G が同じパッケージに Store の一覧などを足す。設計書 3 節は直した | 決定済み |
| 2 | 「OS のキーチェーン」で、どこに置いたトークンを読むか。仕様に、利用者への伝え方の文言がない | **決定(2026-11-12): 前回の実装どおり。** サービス `hedgehug`・アカウント `github.com/{ログイン名}`。見つからないときの文言に場所を出し、キーチェーンのない環境では gh CLI か環境変数を案内する(仕様 07「認証の確認」に反映)。画面からキーチェーンへ入れる入口は持たない | 決定済み |
| 3 | デバイスフローに使う GitHub App(client ID)。仕様は「hedgehug の GitHub App」を前提にするが、どの App か、client ID をどう持つかが決まっていない(前回は設定ファイルに書かせていた) | **決定(2026-11-12): `config.yaml` の `hosts[].client_id`(前回の実装と同じ形)に書き、組み込まない。GitHub App と OAuth App のどちらも受け付け、種類は確かめない。** ないときは「アカウントを追加」で断る(仕様 07「デバイスフロー」に反映) | 決定済み |
