# hedgehug 小計画 C2: 同期の実行と状態(サーバー)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 06 の「手動で同期する」「自動で同期する」「同期のエラー」の残り、「同期の結果」「同期の状態の表示」に要るサーバーの値、「画面への反映」のサーバー側、「同期のあとの完全な削除」の呼び出し口を、サーバーの API まで作る。

**Architecture:** 同期の usecase は `app/syncing` に置く。`Service` が 1 つの Store の同期(トークンを得る → 前に受け取った位置を読む → `Replicator.Sync` → 状態を記録 → ミラーへ送る)と「すべて同期」と状態の一覧を持ち、`Auto` が自動の契機(間隔・アプリを開いたとき・オンライン復帰・書き込み後と、タブの待つ条件)を見て `Service` を呼ぶ。`Watch` が Store の版の変化を見張って `Events` に流し、`adapter/httpapi/events` が Server-Sent Events で画面へ送る。このマシンの同期の状態は `infra/statedb`(SQLite)に置く。`infra/gitstore` の `Replicator` には、通信の失敗の見分け、60 秒止まったときの中断、remote の履歴の書き換えの検出、受け取りだけの同期、ミラーへの送信、未送信の数を足す。

**Tech Stack:** Go 1.27.3、go-git v5.19.3、huma v2.40.0(`sse` を含む)、modernc.org/sqlite v1.61.1(新しく足す)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「同期とは」〜「画面への反映」の各節、「同期のあとの完全な削除」
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「同期」(手順 1・7)、「remote の履歴が書き換えられたとき」の「見つけ方」、「このマシンに持つもの」、「技術構成」
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(依存の向き、口、`Events`、8 節)
- 07「接続のたびの照合」「状態」(同期の 401 で認証エラーにする)
- トラック C の計画 `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(決定 1〜15、C1 の型)

**ブランチ:** `origin/develop` から `feat/track-c2`。PR の向き先は `develop`。Task 1・2(domain の文言と口と偽物)は共有部分。PR の出し方(共有部分を先に別の PR にするか、1 本にまとめるか)は、全タスクのあとでユーザーに確かめる。実装のあいだは push しない。

## Global Constraints

トラック C の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 依存は版を明記して入れる。SQLite は `go get modernc.org/sqlite@v1.61.1`(v1.62.0 は 2026-11-07 公開で 7 日たっていない)。
- トークンらしい値(`ghp_`・`gho_`・`ghu_`・`ghs_`・`ghr_`・`github_pat_` で始まる文字列)を、計画にもテストにも書かない(gitleaks)。テストのトークンは `tok-taro` のような値にする。
- 利用者に見せる文言は、仕様の文字どおりに `domain/syncstatus` に置く。仕様にない文言は足さない。
- 本物の組み立てだけで使う型と関数は `//go:build !e2e` のファイルに置く(e2e のビルドも lint するため)。
- API を変えたら `make generate` と `cd web && pnpm gen` で生成物を作り直す。
- `internal/infra/gitstore/replicator.go`(227 行)と `sync_git.go`(269 行)は 300 行の上限が近い。新しいコードは新しいファイルに置く。
- force push はしない。載せ直すときは別名のブランチで出す。
- 確かめるのは `make check`(tidy・lint・fmt・test・web-check)。

---

## この小計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C2-1 | C2 はサーバー(API と SSE)までにする。同期カード、「Store ごとの同期状態」、860px 以下の点、オフラインの帯、同期のトースト、Shift+Ctrl+S、タブが待つ条件を送ること、SSE を受けて画面を更新することは、小計画 **C2 画面** として、A2 のサイドバー(A2 Task 7)・トースト(A2 Task 5)・一覧の状態(A2 Task 6)が develop に入ってから計画を書く | D2 と同じ進め方(D2-1)。いま web/app にはサイドバーもトーストの置き場もない |
| C2-2 | Store の同期に要る設定(名前・remote・ミラー・アカウント・非表示・閲覧のみ)は、新しい口 `port.SyncTargets` で読む。本物はトラック G が config.yaml の Store の一覧から作る。それまで本物の組み立ては Store のない実装(`noSyncTargets`)を使い、e2e は偽物を使う。このため、本物のアプリで実際に同期するのは G が入ってから | D2-11・A2 の決定 2 と同じ。Store の一覧は G の持ち場。A2 の `port.StoreList` は表示中の Store だけで remote を持たない |
| C2-3 | 同期のトークンは、口 `port.SyncTokens`(`StoreToken`・`StoreRejected`)で得る。実装は `app/github.Connector` に足すメソッドで、読み直し・照合・取り直し・401 での認証エラーを D と同じ入口で行う。`Session` は組み立て(cmd)で 1 つ作り、GitHub の API と同期で共用する | app の機能どうしは import しない(設計書 2 節)。07「状態」(同期の 401 で認証エラー) |
| C2-4 | 同期のタイミング(自動同期のオン・オフ、間隔、書き込みのあと同期するまで)は、口 `port.SyncSettings` で読む。実装は `infra/machineconfig` で、config.yaml の `sync: {auto, interval, push_interval}`(秒)を読み、ないものは初期値(オン・5 分・30 秒)にする。書き換え(設定の「一般」)は、その画面を作るトラックが足す | 前回の実装の `sync.interval`・`sync.push_interval`(秒)。保存形式「このマシンに持つもの」(同期のタイミングは config.yaml) |
| C2-5 | このマシンの同期の状態(最後に試みた時刻、最後に成功した時刻、最後の失敗の理由、前に受け取った位置とその remote、ミラーごとの結果)は、口 `port.SyncState` で読み書きし、`infra/statedb` の表に置く。アクティビティの「同期の行」は C4 が足す(読む画面と一緒に作る) | 保存形式「このマシンに持つもの」。設計書 3 節「先回りして足さない」 |
| C2-6 | `infra/statedb` は、表を作る手順を「機能/連番」の名前で登録し、名前の順に 1 度だけ当てる。手順は機能ごとのファイルの `init` で登録するので、ほかのトラック(E の索引、D4 の GitHub の読み取りの情報)は C のファイルを触らずに表を足せる | トラック C の決定 11 |
| C2-7 | remote との通信の失敗は、`infra/gitstore` が port のエラーに直す。401 → `ErrAuthFailed`、403 は送信中なら `ErrWriteDenied`・受け取り中なら `ErrRemoteUnreadable`、404 → `ErrRemoteUnreadable`、60 秒進まない・つながらない・途中で切れた・そのほか → `ErrUnreachable` | 06「同期のエラー」の表。go-git は 401・403・404 をそれぞれ `ErrAuthenticationRequired`・`ErrAuthorizationFailed`・`ErrRepositoryNotFound` で返す |
| C2-8 | remote が 404(リポジトリがない、または権限がない)の短い理由は「remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。」。06 の「同期のエラー」の表に行を足し、ミラーの短い理由にも同じ文を足す | ユーザー回答(2026-11-13)。GitHub は 2 つを同じ応答で返す(07) |
| C2-9 | 同期の直前にトークンを読めない(キーチェーンの項目がない・GH_TOKEN がない・gh が未ログインまたは応答しない)ときは「認証に失敗しました」にする。仕様は変えない | ユーザー回答(2026-11-13) |
| C2-10 | 60 秒の打ち切りは「通信がまったく進まない」時間で数える。go-git の HTTP の通信に、読み書きのたびに時計を戻す見張りを差し込む(`client.InstallProtocol` で http・https に入れる。go-git は remote ごとの HTTP クライアントを受け取らないため)。見張りは通信のあいだだけ動かし、合わせる計算の時間は数えない | 06「同期のエラー」(60 秒間まったく進まないとき) |
| C2-11 | 前に受け取った位置は、口では git の言葉を使わず `Remote.Seen`・`SyncResult.Seen`(文字列)と呼ぶ。`app/syncing` が `SyncState` から読んで渡し、成功したら結果の値を記録する。remote の同一性は、URL から方式・利用者名・末尾の `.git`・末尾の `/` を除き、`git@host:path` を `host/path` にし、小文字にしたもの(`remoteKey`)で比べる。違えば位置を使わない(共通の履歴の検査だけになる) | 保存形式「remote の履歴が書き換えられたとき」(SSH と HTTPS の書き方の違いでは捨てない、付け替えたら持たない) |
| C2-12 | 前に受け取った位置があり、remote が空になった、または同じ名前のブランチがなくなったときも、履歴の書き換え(`ErrRemoteRewritten`)とする | 06(履歴が過去へ巻き戻されたときも同じ) |
| C2-13 | 閲覧のみの Store は、`Remote.ReceiveOnly` で受け取りと合わせだけを行い、送らない。ミラーにも送らず、完全な削除の呼び出し口も呼ばない | 01「閲覧のみの Store」、06「自動で同期する」(受け取りだけ)、06「同期のあとの完全な削除」(閲覧のみでは行わない) |
| C2-14 | 「未送信」は、前の同期で受け取った remote の先端(gitstore の `refs/hedgehug/remote/<ブランチ>`)から辿れない保存の数(`Replicator.Unsent`)。一度も受け取っていなければ、すべての保存を数える。G が Store を追加(clone)するときは、この参照を作ってから使う | 06「同期の状態の表示」(このマシンで保存して、まだ送っていない保存の数) |
| C2-15 | ミラーへの送信(`Replicator.Mirror`)は、ミラーのブランチを `refs/hedgehug/mirror/<URL の SHA-1 の先頭 12 文字>/` に受け取り、空なら送り、自分の先端の祖先なら送り、共通の祖先がない(同じ名前のブランチがない場合を含む)なら `ErrMirrorUnrelated`、それ以外は `ErrMirrorAhead`。送信が重なって拒否されたら `ErrMirrorAhead` | 保存形式「同期」の手順 7、06 のミラーの段落 |
| C2-16 | 「アカウントが設定されていない」は記録せず、状態の一覧で、remote があって割り当てがない Store を同期エラーとして導く。アカウントを割り当てれば、表示は次の同期を待たずに消える | 06「同期のエラー」(試していなくても同期エラー、アカウントを設定すると消える) |
| C2-17 | 同じ Store の同期が動いているときは、新しく始めずに、その結果を待って返す。動いている同期は、始めた要求が切れても最後まで続ける(`context.WithoutCancel`。60 秒の見張りで必ず終わる) | 06「手動で同期する」 |
| C2-18 | 自動の同期は、サーバーの `Auto` が 1 秒ごとに契機を調べて行う。タブは、見えていて待つ条件にあるあいだ `PUT /api/sync/hold`(`{tab, hold: true}`)を 10 秒ごとに送り、解けたら `hold: false` を送る。知らせは 30 秒で切れる(閉じたタブが待たせ続けないため)。オンライン復帰はタブが `POST /api/sync/online` で知らせる。「アプリを開いたとき」はサーバーの起動 | 06「自動で同期する」(見えているタブの条件だけで待つ)。待つ条件はタブにしか分からない |
| C2-19 | 「書き込み後」の契機は、`syncing.NotifyWrites` で包んだ `port.Stores` の `Write` が成功したときに数える。このサーバーの外のプログラム(CLI)の書き込みは、CLI を作るときに足す | 01(CLI はこの仕様で扱わない)。CLI はまだない |
| C2-20 | 「画面への反映」のサーバー側は、`Watch` が 0.5 秒ごとに Store の版(gitstore の先端)を見て、変わった Store を `Events` に `store` として流す。書き手(このタブ、別のタブ、同期)は問わない。同期の状態が変わったら `Service` が `sync` を流す。SSE は `GET /api/events`。ブラウザの EventSource はヘッダーを付けられないので、画面は fetch の読み続けで受ける(C2 画面) | 保存形式「技術構成」(HEAD の変化を見張り SSE で知らせる)。06「画面への反映」(1 秒ほどで反映) |
| C2-21 | 同期の結果は失敗でも 200 で返し、本文の `result`(`synced`・`skipped`・`failed`)と `message`(06 の文言)で分ける。存在しない Store は 404 | 同期の失敗は断り(422)ではなく結果。画面はトーストの種類を `result` で選ぶ |
| C2-22 | 「同期のあとの完全な削除」は、`Service.AfterSuccess`(関数)を、書き込める Store の同期に成功した直後に呼ぶ。中身はトラック G が差し込む | トラック C の決定 13 |
| C2-23 | usecase のパッケージ名は `app/syncing`、API は `adapter/httpapi/syncing` にする(トラック C の計画の持ち場では `sync`)。`sync` は標準ライブラリの名前と重なり、`sync.Mutex` を使うたびに別名が要るため | 読みやすさ。持ち場の意味は変わらない |
| C2-24 | 自動の同期の失敗はトーストもアクティビティも出さない(結果は状態の一覧に出る)。自動の同期で `Auto` は結果を捨てる | 06「自動で同期する」 |

## ファイルの地図

```
docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md(変更)  404 の行(C2-8)

internal/domain/syncstatus/syncstatus.go       Reason・Short・Failed・NoRemote・Synced・All・MirrorsFailed・NothingToSync
internal/domain/syncstatus/syncstatus_test.go

internal/app/port/replicator.go(変更)         Remote.Seen・ReceiveOnly、SyncResult.Seen、Mirror・Unsent、エラーを足す
internal/app/port/synctargets.go               SyncTarget・SyncTargets
internal/app/port/synctokens.go                SyncTokens
internal/app/port/syncsettings.go              SyncTiming・SyncSettings
internal/app/port/syncstate.go                 SyncRecord・MirrorRecord・SyncState
internal/app/port/events.go                    Event・Events
internal/app/port/storeversions.go             StoreVersions
internal/app/apptest/replicator.go             Replicator の偽物
internal/app/apptest/synctargets.go            SyncTargets・SyncSettings・SyncTokens・StoreVersions の偽物
internal/app/apptest/syncstate.go              SyncState の偽物
internal/app/apptest/syncstate_contract.go     RunSyncStateContract
internal/app/apptest/syncstate_test.go
internal/app/apptest/events.go                 Events の偽物(記録する)

internal/infra/statedb/db.go                   Open・Close
internal/infra/statedb/migrate.go              register・migrate
internal/infra/statedb/sync.go                 SyncState の実装と表
internal/infra/statedb/db_test.go・sync_test.go

internal/infra/gitstore/sync_transport.go      見張り(60 秒)と http・https の差し込み、classify
internal/infra/gitstore/sync_transport_test.go
internal/infra/gitstore/sync_git.go(変更)      fetch・send・tip に見張りと classify、fetchTo
internal/infra/gitstore/replicator.go(変更)    Seen・ReceiveOnly・結果の Seen
internal/infra/gitstore/replicator_seen.go     checkSeen・Unsent
internal/infra/gitstore/replicator_mirror.go   Mirror
internal/infra/gitstore/versions.go            Stores.Versions
internal/infra/gitstore/sync_errors_test.go・sync_seen_test.go・sync_mirror_test.go・versions_test.go

internal/infra/machineconfig/sync.go           SyncSettings の実装
internal/infra/machineconfig/file.go(変更)     config に Sync を足す
internal/infra/machineconfig/sync_test.go

internal/app/github/sync_tokens.go             Connector.StoreToken・StoreRejected
internal/app/github/sync_tokens_test.go

internal/app/syncing/service.go                Service・Outcome・SyncStore・1 つずつ行う仕組み
internal/app/syncing/run.go                    1 つの Store の同期の手順、ミラー
internal/app/syncing/reason.go                 エラー → 理由
internal/app/syncing/remotekey.go              remoteKey
internal/app/syncing/all.go                    SyncAll
internal/app/syncing/status.go                 Status
internal/app/syncing/auto.go                   Auto(契機・待つ条件)
internal/app/syncing/due.go                    due(いま同期するか)
internal/app/syncing/notify.go                 NotifyWrites
internal/app/syncing/watch.go                  Watch
internal/app/syncing/*_test.go

internal/adapter/httpapi/events/events.go      Hub(port.Events)と GET /api/events
internal/adapter/httpapi/events/events_test.go
internal/adapter/httpapi/syncing/syncing.go    同期の API
internal/adapter/httpapi/syncing/dto.go
internal/adapter/httpapi/syncing/syncing_test.go
internal/adapter/httpapi/routes.go(変更)       Deps に Sync・Auto・Events・GitHubSession、Register を足す

cmd/hedgehug/wire.go(変更)・nostores.go(変更)・paths.go(変更)・wire_e2e.go(変更)
api/openapi.yaml・web/packages/api/src/generated.ts(生成)
```

## 依存の流れ

```
Task 1 ─→ Task 2 ─→(共有 PR)
               ├─→ Task 3 statedb
               ├─→ Task 4 通信の失敗と 60 秒 ─→ Task 5 Seen・受け取りだけ・未送信 ─→ Task 6 ミラー
               ├─→ Task 7 machineconfig
               ├─→ Task 8 Connector
               └─→ Task 9 Service ─→ Task 10 Auto ─→ Task 11 Watch と Versions
                                                         └─→ Task 12 API と SSE ─→ Task 13 組み立て
```

Task 3・4・7・8・9 は Task 2 のあとで互いに独立。subagent-driven で 1 つずつ順に行う(並べると同じファイルの生成物がぶつかるため)。

---
### Task 1: 同期の文言(domain/syncstatus)と仕様の 404 の行

**Files:**
- Create: `internal/domain/syncstatus/syncstatus.go`
- Test: `internal/domain/syncstatus/syncstatus_test.go`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`(「同期のエラー」の表と、ミラーの段落)

**Interfaces:**
- Produces:
  - `type Reason string`。定数 `NoAccount`・`AuthFailed`・`Unreachable`・`WriteDenied`・`Unreadable`・`NotStore`・`NewerFormat`・`Unrelated`・`Rewritten`・`SendConflict`・`MirrorUnrelated`・`MirrorAhead`
  - `func (r Reason) Short() string`(知らない理由は空)・`func (r Reason) Valid() bool`
  - `func Failed(store string, r Reason) string`・`func NoRemote(store string) string`・`func Synced(store string, sent, received int) string`・`func MirrorsFailed(k int) string`・`const NothingToSync`
  - `type Failure struct{ Store string; Reason Reason }`・`func All(ok int, failures []Failure) []string`

- [ ] **Step 1: 仕様に 404 の行を足す**

`06-sync-notifications.md` の「同期のエラー」の表で、「remote につながらない」の行のすぐ下に次の行を足す。

```markdown
| remote のリポジトリを読めない(リポジトリがない、または権限がない。GitHub はこの 2 つを同じ応答で返す) | 「{Store 名} の同期に失敗しました: remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。」(エラー) | remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。 | 同期エラー |
```

同じ節のミラーの段落の「ミラーの短い理由は、上の表の「認証に失敗しました」「remote に接続できません」「remote への書き込みが許可されていません」と、」を、次に替える。

```markdown
ミラーの短い理由は、上の表の「認証に失敗しました」「remote に接続できません」「remote への書き込みが許可されていません」「remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。」と、
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/domain/syncstatus/syncstatus_test.go`:

```go
package syncstatus_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 06「同期のエラー」の表の「短い理由」と、ミラーの段落の短い理由。
func TestShortIsTheSpecWording(t *testing.T) {
	cases := []struct {
		r    syncstatus.Reason
		want string
	}{
		{syncstatus.NoAccount, "アカウントが設定されていません"},
		{syncstatus.AuthFailed, "認証に失敗しました"},
		{syncstatus.Unreachable, "remote に接続できません"},
		{syncstatus.WriteDenied, "remote への書き込みが許可されていません"},
		{syncstatus.Unreadable, "remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。"},
		{syncstatus.NotStore, "remote が hedgehug の Store ではありません"},
		{syncstatus.NewerFormat, "hedgehug を更新してください"},
		{syncstatus.Unrelated, "remote の履歴がこの Store と関係がありません"},
		{syncstatus.Rewritten, "remote の履歴が書き換えられています"},
		{syncstatus.SendConflict, "送信が他の人の更新と重なりました。もう一度同期してください"},
		{syncstatus.MirrorUnrelated, "ミラーの履歴がこの Store と関係がありません"},
		{syncstatus.MirrorAhead, "ミラーの履歴が先に進んでいます"},
	}
	for _, c := range cases {
		if got := c.r.Short(); got != c.want {
			t.Errorf("%s: Short = %q, want %q", c.r, got, c.want)
		}
		if !c.r.Valid() {
			t.Errorf("%s: not Valid", c.r)
		}
	}
	if syncstatus.Reason("x").Valid() || syncstatus.Reason("x").Short() != "" || syncstatus.Reason("").Valid() {
		t.Error("unknown reason is valid")
	}
}

// 06「同期のエラー」の「手動で同期したときの表示」。
func TestFailed(t *testing.T) {
	if got := syncstatus.Failed("team", syncstatus.NoAccount); got != "team にアカウントが設定されていないため同期できません" {
		t.Errorf("no account: %q", got)
	}
	if got := syncstatus.Failed("team", syncstatus.AuthFailed); got != "team の同期に失敗しました: 認証に失敗しました" {
		t.Errorf("auth: %q", got)
	}
	if got := syncstatus.Failed("team", syncstatus.SendConflict); got != "team の同期に失敗しました: 送信が他の人の更新と重なりました。もう一度同期してください" {
		t.Errorf("conflict: %q", got)
	}
}

// 06「同期の結果」と「手動で同期する」「同期のエラー」の、失敗でない文言。
func TestMessages(t *testing.T) {
	if got := syncstatus.NoRemote("solo"); got != "solo には remote がないため、同期するものはありません" {
		t.Errorf("NoRemote: %q", got)
	}
	if got := syncstatus.Synced("team", 2, 3); got != "team を同期しました(送信 2 件・受信 3 件)" {
		t.Errorf("Synced: %q", got)
	}
	if got := syncstatus.MirrorsFailed(2); got != "ミラー 2 件に送れませんでした" {
		t.Errorf("MirrorsFailed: %q", got)
	}
	if syncstatus.NothingToSync != "同期する Store がありません(remote のある、表示中の Store が要ります)" {
		t.Errorf("NothingToSync: %q", syncstatus.NothingToSync)
	}
}

// 06「同期の結果」の「すべて同期」のトースト。
func TestAll(t *testing.T) {
	if got := syncstatus.All(3, nil); !slices.Equal(got, []string{"3 件の Store を同期しました"}) {
		t.Errorf("all ok: %q", got)
	}
	got := syncstatus.All(1, []syncstatus.Failure{
		{Store: "team", Reason: syncstatus.NoAccount},
		{Store: "web", Reason: syncstatus.Unreachable},
	})
	want := []string{"成功 1 件・失敗 2 件", "team: アカウントが設定されていません", "web: remote に接続できません"}
	if !slices.Equal(got, want) {
		t.Errorf("mixed: %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/domain/syncstatus/`
Expected: FAIL(パッケージがない)

- [ ] **Step 4: 実装する**

`internal/domain/syncstatus/syncstatus.go`:

```go
// Package syncstatus は、同期の結果と状態の文言(06「同期のエラー」「同期の結果」)を持つ。
// 文言は仕様の文字どおりに書く。
package syncstatus

import "fmt"

// Reason は、同期できなかった理由。値は state.db と API に書く。
type Reason string

const (
	NoAccount       Reason = "no_account"
	AuthFailed      Reason = "auth_failed"
	Unreachable     Reason = "unreachable"
	WriteDenied     Reason = "write_denied"
	Unreadable      Reason = "unreadable"
	NotStore        Reason = "not_store"
	NewerFormat     Reason = "newer_format"
	Unrelated       Reason = "unrelated"
	Rewritten       Reason = "rewritten"
	SendConflict    Reason = "send_conflict"
	MirrorUnrelated Reason = "mirror_unrelated"
	MirrorAhead     Reason = "mirror_ahead"
)

// shortText は、06「同期のエラー」の表の「短い理由」と、ミラーの段落の短い理由。
var shortText = map[Reason]string{
	NoAccount:       "アカウントが設定されていません",
	AuthFailed:      "認証に失敗しました",
	Unreachable:     "remote に接続できません",
	WriteDenied:     "remote への書き込みが許可されていません",
	Unreadable:      "remote を読めません。ご利用アカウントに付与されているリポジトリ権限をご確認ください。",
	NotStore:        "remote が hedgehug の Store ではありません",
	NewerFormat:     "hedgehug を更新してください",
	Unrelated:       "remote の履歴がこの Store と関係がありません",
	Rewritten:       "remote の履歴が書き換えられています",
	SendConflict:    "送信が他の人の更新と重なりました。もう一度同期してください",
	MirrorUnrelated: "ミラーの履歴がこの Store と関係がありません",
	MirrorAhead:     "ミラーの履歴が先に進んでいます",
}

// Short は短い理由。「すべて同期」のトースト、「Store ごとの同期状態」、設定の「Store」で同じ文を使う。
// 知らない理由は空。
func (r Reason) Short() string { return shortText[r] }

// Valid は、知っている理由か。
func (r Reason) Valid() bool { _, ok := shortText[r]; return ok }

// Failed は、手動で 1 つの Store を同期して失敗したときのトースト(エラー)。
func Failed(store string, r Reason) string {
	if r == NoAccount {
		return store + " にアカウントが設定されていないため同期できません"
	}
	return store + " の同期に失敗しました: " + r.Short()
}

// NoRemote は、remote がない Store を手動で同期したときのトースト。Store の状態は変えない。
func NoRemote(store string) string {
	return store + " には remote がないため、同期するものはありません"
}

// Synced は、1 つの Store の同期に成功したときのトースト。
func Synced(store string, sent, received int) string {
	return fmt.Sprintf("%s を同期しました(送信 %d 件・受信 %d 件)", store, sent, received)
}

// MirrorsFailed は、手動で 1 つの Store を同期して、ミラーに送れなかったときの警告のトースト。
func MirrorsFailed(k int) string { return fmt.Sprintf("ミラー %d 件に送れませんでした", k) }

// NothingToSync は、同期する Store がないときのトースト(06「手動で同期する」)。
const NothingToSync = "同期する Store がありません(remote のある、表示中の Store が要ります)"

// Failure は、「すべて同期」で失敗した 1 つの Store。
type Failure struct {
	Store  string
	Reason Reason
}

// All は、「すべて同期」のトーストの行。1 行目が要約で、失敗があれば、続けて失敗した Store ごとに
// 「{Store 名}: {短い理由}」を 1 行ずつ並べる。
func All(ok int, failures []Failure) []string {
	if len(failures) == 0 {
		return []string{fmt.Sprintf("%d 件の Store を同期しました", ok)}
	}
	lines := []string{fmt.Sprintf("成功 %d 件・失敗 %d 件", ok, len(failures))}
	for _, f := range failures {
		lines = append(lines, f.Store+": "+f.Reason.Short())
	}
	return lines
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test ./internal/domain/syncstatus/ && golangci-lint run ./internal/domain/syncstatus/`
Expected: PASS、指摘なし

- [ ] **Step 6: コミットする**

```bash
git add internal/domain/syncstatus docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md
git commit -m "feat(domain): 同期の結果と状態の文言を足し、remote を読めないときの行を仕様に足す(C2 Task 1)"
```

---

### Task 2: 同期の口と偽物(共有部分)

**Files:**
- Modify: `internal/app/port/replicator.go`
- Create: `internal/app/port/synctargets.go`・`synctokens.go`・`syncsettings.go`・`syncstate.go`・`events.go`・`storeversions.go`
- Create: `internal/app/apptest/replicator.go`・`synctargets.go`・`syncstate.go`・`syncstate_contract.go`・`events.go`
- Test: `internal/app/apptest/syncstate_test.go`・`internal/app/apptest/replicator_test.go`

**Interfaces:**
- Consumes: `syncstatus.Reason`(Task 1)、`account.ID`
- Produces(後のタスクが名前と型のとおりに使う):

```go
// port
type Replicator interface {
	Sync(ctx context.Context, storeID StoreID, remote Remote) (SyncResult, error)
	Mirror(ctx context.Context, storeID StoreID, mirror Remote) error
	Unsent(ctx context.Context, storeID StoreID) (int, error)
}
type Remote struct { URL, Token, Seen string; ReceiveOnly bool }
type SyncResult struct { Sent, Received int; Seen string }
var ErrRemoteRewritten, ErrAuthFailed, ErrRemoteUnreadable, ErrWriteDenied, ErrUnreachable, ErrMirrorUnrelated, ErrMirrorAhead error

type SyncTarget struct { ID StoreID; Name, Remote string; Mirrors []string; Account account.ID; Hidden, ReadOnly bool }
type SyncTargets interface { List(ctx context.Context) ([]SyncTarget, error) }

type SyncTokens interface {
	StoreToken(ctx context.Context, store StoreID) (string, error)
	StoreRejected(ctx context.Context, store StoreID)
}

type SyncTiming struct { Auto bool; Interval, AfterWrite time.Duration }
type SyncSettings interface { Timing(ctx context.Context) (SyncTiming, error) }

type SyncRecord struct { Store StoreID; LastAttempt, LastSuccess time.Time; Reason syncstatus.Reason }
type MirrorRecord struct { Mirror string; At time.Time; Reason syncstatus.Reason }
type SyncState interface {
	Records(ctx context.Context) ([]SyncRecord, error)
	Failed(ctx context.Context, store StoreID, at time.Time, reason syncstatus.Reason) error
	Succeeded(ctx context.Context, store StoreID, at time.Time, remoteKey, seen string) error
	Seen(ctx context.Context, store StoreID, remoteKey string) (string, error)
	Mirrored(ctx context.Context, store StoreID, mirror string, at time.Time, reason syncstatus.Reason) error
	Mirrors(ctx context.Context, store StoreID) ([]MirrorRecord, error)
}

type EventKind string // EventStore = "store"、EventSync = "sync"
type Event struct { Kind EventKind; Store StoreID }
type Events interface { Publish(e Event) }

type StoreVersions interface { Versions(ctx context.Context) (map[StoreID]string, error) }

// apptest
type Replicator struct { mu; Results map[port.StoreID]port.SyncResult; Errs map[port.StoreID]error; MirrorErrs map[string]error; UnsentN map[port.StoreID]int; Gate chan struct{} }
func (r *Replicator) Calls() []ReplicatorCall          // ReplicatorCall{Store port.StoreID; Remote port.Remote; Mirror bool}
type SyncTargets struct { mu; Targets []port.SyncTarget }
type SyncSettings struct { mu; Value port.SyncTiming }
type SyncTokens struct { mu; Tokens map[port.StoreID]string; Errs map[port.StoreID]error }
func (s *SyncTokens) Rejected() []port.StoreID
type StoreVersions struct { mu; Value map[port.StoreID]string }
func NewSyncState() *SyncState
func RunSyncStateContract(t *testing.T, newState func(t *testing.T) port.SyncState)
type Events struct { mu; list []port.Event }
func (e *Events) List() []port.Event
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/apptest/syncstate_test.go`:

```go
package apptest_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestSyncStateContract(t *testing.T) {
	apptest.RunSyncStateContract(t, func(*testing.T) port.SyncState { return apptest.NewSyncState() })
}
```

`internal/app/apptest/replicator_test.go`:

```go
package apptest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestReplicatorRecordsCallsAndReturnsWhatWasSet(t *testing.T) {
	boom := errors.New("boom")
	r := &apptest.Replicator{
		Results:    map[port.StoreID]port.SyncResult{"S1": {Sent: 1, Received: 2, Seen: "v1"}},
		Errs:       map[port.StoreID]error{"S2": port.ErrAuthFailed},
		MirrorErrs: map[string]error{"https://m.invalid/x": boom},
		UnsentN:    map[port.StoreID]int{"S1": 4},
	}
	ctx := context.Background()
	if res, err := r.Sync(ctx, "S1", port.Remote{URL: "u"}); err != nil || res.Sent != 1 || res.Seen != "v1" {
		t.Errorf("Sync S1 = %+v, %v", res, err)
	}
	if _, err := r.Sync(ctx, "S2", port.Remote{}); !errors.Is(err, port.ErrAuthFailed) {
		t.Errorf("Sync S2 = %v", err)
	}
	if err := r.Mirror(ctx, "S1", port.Remote{URL: "https://m.invalid/x"}); !errors.Is(err, boom) {
		t.Errorf("Mirror = %v", err)
	}
	if n, err := r.Unsent(ctx, "S1"); n != 4 || err != nil {
		t.Errorf("Unsent = %d, %v", n, err)
	}
	calls := r.Calls()
	if len(calls) != 3 || calls[0].Store != "S1" || calls[0].Remote.URL != "u" || !calls[2].Mirror {
		t.Errorf("Calls = %+v", calls)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/app/apptest/ -run 'SyncState|Replicator'`
Expected: FAIL(`NewSyncState`・`Replicator` がない)

- [ ] **Step 3: 口を足す**

`internal/app/port/replicator.go` を次に替える(C1 の `Sync`・`Remote`・`SyncResult`・4 つのエラーは残し、足す)。

```go
package port

import (
	"context"
	"errors"
)

// Replicator は、Store をほかの場所(remote・ミラー)と合わせる口。git の版だけが持つ(設計書 3 節)。
// 受け取り、合わせ、送る。合わせたあとの修復は、app/syncing が Stores.Write で保存する。
type Replicator interface {
	// Sync は、remote から受け取り、合わせ、送る(ReceiveOnly なら送らない)。
	Sync(ctx context.Context, storeID StoreID, remote Remote) (SyncResult, error)
	// Mirror は、Store の保存をミラーへ送るだけで、受け取らない(保存形式「同期」の手順 7)。
	// ミラーが空なら最初の送信として送る。共通の履歴がなければ ErrMirrorUnrelated、ミラーにだけ保存が
	// あれば ErrMirrorAhead。どちらも送らない。
	Mirror(ctx context.Context, storeID StoreID, mirror Remote) error
	// Unsent は、まだ remote に送っていない保存の数(06「同期の状態の表示」の未送信)。
	Unsent(ctx context.Context, storeID StoreID) (int, error)
}

// Remote は合わせる相手。Token が空なら、認証なしで読み書きする。
type Remote struct {
	URL   string
	Token string
	// Seen は、前の同期で同じ remote から受け取った位置(前の SyncResult.Seen)。空でなく、remote の履歴が
	// その続きでなければ、Sync は ErrRemoteRewritten を返す。空なら確かめない。
	Seen string
	// ReceiveOnly なら、受け取って合わせるだけで、送らない(閲覧のみの Store)。
	ReceiveOnly bool
}

// SyncResult は、送った保存と受け取った保存の数(06「同期の結果」)と、今回受け取った位置。
// Seen は、次の同期の Remote.Seen に渡す。remote が空のまま終わったら空。
type SyncResult struct {
	Sent     int
	Received int
	Seen     string
}

// 同期を断る理由(06「同期のエラー」)。
// ErrNoCommonHistory・ErrRemoteNotStore・ErrRemoteNewerFormat・ErrRemoteRewritten は、最初の試みで返すときは、
// このマシンの Store を変えない。ErrSendConflict のときと、2 回目からの試みで断るときは、前の試みで合わせた
// 結果がこのマシンに残り、次の同期で続ける。
var (
	ErrNoCommonHistory   = errors.New("the remote has no history in common with the store")
	ErrRemoteNotStore    = errors.New("the remote is not a hedgehug store")
	ErrRemoteNewerFormat = errors.New("the remote was saved by a newer hedgehug")
	ErrSendConflict      = errors.New("sending kept overlapping with other updates")
	ErrRemoteRewritten   = errors.New("the remote history was rewritten")
)

// remote との通信の失敗(06「同期のエラー」)。ミラーへの送信でも同じものを返す。
var (
	ErrAuthFailed       = errors.New("the remote refused the credentials")    // 401
	ErrRemoteUnreadable = errors.New("the remote cannot be read")             // 404、受け取りの 403
	ErrWriteDenied      = errors.New("the remote refused writing")            // 送信の 403
	ErrUnreachable      = errors.New("the remote cannot be reached")          // つながらない、途中で切れた、60 秒進まない
	ErrMirrorUnrelated  = errors.New("the mirror has no history in common")   // ミラーが空でなく、共通の履歴がない
	ErrMirrorAhead      = errors.New("the mirror has saves the store has not") // ミラーにだけ保存がある
)
```

`internal/app/port/synctargets.go`:

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/account"
)

// SyncTarget は、同期に要る Store の設定(08 の Store の一覧のうち、このマシンだけの値)。
type SyncTarget struct {
	ID      StoreID
	Name    string
	Remote  string   // remote の URL。なければ空
	Mirrors []string // ミラーの URL
	Account account.ID // GitHub に使うアカウント。「なし」なら空
	Hidden   bool
	ReadOnly bool
}

// SyncTargets は、同期に要る Store の設定の口。本物の実装はトラック G が足す。
type SyncTargets interface {
	// List は、このマシンのすべての Store(非表示・閲覧のみ・remote なしを含む)を、サイドバーの並びで返す。
	List(ctx context.Context) ([]SyncTarget, error)
}
```

`internal/app/port/synctokens.go`:

```go
package port

import "context"

// SyncTokens は、Store に割り当てたアカウントのトークンを得る口(07「接続のたびの照合」)。
// 実装は app/github の Connector(読み直し・照合・取り直し・認証エラーへの変更を 1 か所で行う)。
type SyncTokens interface {
	// StoreToken は、Store のアカウントのトークン。割り当てがなければ NoAccount、認証エラーなら AuthError、
	// 読めなければ GhTokenMissing などの *ghcall.Failure を返す。
	StoreToken(ctx context.Context, store StoreID) (string, error)
	// StoreRejected は、Store のアカウントのトークンを remote が拒否した(401)ことを知らせ、認証エラーにする
	// (07「状態」)。割り当てがなければ何もしない。
	StoreRejected(ctx context.Context, store StoreID)
}
```

`internal/app/port/syncsettings.go`:

```go
package port

import (
	"context"
	"time"
)

// SyncTiming は、設定の「一般」の「裏で自動的に同期する」と「同期のタイミング」(06「自動で同期する」)。
type SyncTiming struct {
	Auto       bool          // 裏で自動的に同期する
	Interval   time.Duration // 間隔
	AfterWrite time.Duration // 書き込みのあと同期するまで
}

// SyncSettings は、同期のタイミングを読む口(このマシンだけの設定)。
type SyncSettings interface {
	// Timing は、いまの設定。書かれていない項目は初期値(オン・5 分・30 秒)。
	Timing(ctx context.Context) (SyncTiming, error)
}
```

`internal/app/port/syncstate.go`:

```go
package port

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// SyncRecord は、1 つの Store の、このマシンでの同期の記録。
type SyncRecord struct {
	Store       StoreID
	LastAttempt time.Time         // 最後に試みた時刻(成功・失敗を問わない)。なければゼロ
	LastSuccess time.Time         // 最後に成功した時刻。なければゼロ
	Reason      syncstatus.Reason // 最後の試みが失敗なら、その理由。成功なら空
}

// MirrorRecord は、1 つのミラーへの、最後の送信の結果。
type MirrorRecord struct {
	Mirror string            // ミラーの URL
	At     time.Time         // 最後に送ろうとした時刻
	Reason syncstatus.Reason // 送れなかった理由。送れたら空
}

// SyncState は、このマシンの同期の状態(保存形式「このマシンに持つもの」)。
type SyncState interface {
	// Records は、記録のあるすべての Store を、Store ID の順で返す。
	Records(ctx context.Context) ([]SyncRecord, error)
	// Failed は、失敗した試みを記録する。最後に成功した時刻と、前に受け取った位置は変えない。
	Failed(ctx context.Context, store StoreID, at time.Time, reason syncstatus.Reason) error
	// Succeeded は、成功を記録し(理由を消す)、remoteKey の remote から受け取った位置 seen を覚える。
	Succeeded(ctx context.Context, store StoreID, at time.Time, remoteKey, seen string) error
	// Seen は、remoteKey の remote から前に受け取った位置。記録がないか、覚えた remote が違えば空。
	Seen(ctx context.Context, store StoreID, remoteKey string) (string, error)
	// Mirrored は、ミラーへの送信の結果を記録する。reason が空なら送れた。
	Mirrored(ctx context.Context, store StoreID, mirror string, at time.Time, reason syncstatus.Reason) error
	// Mirrors は、Store のミラーごとの最後の結果を、URL の順で返す。
	Mirrors(ctx context.Context, store StoreID) ([]MirrorRecord, error)
}
```

`internal/app/port/events.go`:

```go
package port

// EventKind は、画面に知らせる出来事の種類(06「画面への反映」)。
type EventKind string

const (
	EventStore EventKind = "store" // Store の中身が変わった(書き手を問わない)
	EventSync  EventKind = "sync"  // Store の同期の状態が変わった
)

// Event は、画面に知らせる出来事。
type Event struct {
	Kind  EventKind
	Store StoreID
}

// Events は、出来事を画面に知らせる口。実装は adapter(Server-Sent Events。設計書 3 節)。
// Publish は待たない(遅い画面には届かないことがある)。
type Events interface {
	Publish(e Event)
}
```

`internal/app/port/storeversions.go`:

```go
package port

import "context"

// StoreVersions は、Store ごとのいまの版の印の口。保存のたびに、書き手を問わず変わる(06「画面への反映」)。
type StoreVersions interface {
	// Versions は、このマシンで開いているすべての Store の版の印。読めない Store は含めない。
	Versions(ctx context.Context) (map[StoreID]string, error)
}
```

- [ ] **Step 4: 偽物を足す**

`internal/app/apptest/replicator.go`:

```go
package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
)

// ReplicatorCall は、Replicator の偽物が受けた 1 回の呼び出し。
type ReplicatorCall struct {
	Store  port.StoreID
	Remote port.Remote
	Mirror bool // Mirror の呼び出しなら true
}

// Replicator は port.Replicator の偽物。Sync は Results・Errs に置いた値を返す。Gate が nil でなければ、
// Sync はそこから 1 つ受け取るまで待つ(同期が動いているあいだを作るため)。
type Replicator struct {
	mu         sync.Mutex
	Results    map[port.StoreID]port.SyncResult
	Errs       map[port.StoreID]error
	MirrorErrs map[string]error // ミラーの URL → エラー
	UnsentN    map[port.StoreID]int
	Gate       chan struct{}
	calls      []ReplicatorCall
}

func (r *Replicator) Sync(ctx context.Context, id port.StoreID, remote port.Remote) (port.SyncResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, ReplicatorCall{Store: id, Remote: remote})
	gate := r.Gate
	r.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return port.SyncResult{}, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.Errs[id]; err != nil {
		return port.SyncResult{}, err
	}
	return r.Results[id], nil
}

func (r *Replicator) Mirror(_ context.Context, id port.StoreID, mirror port.Remote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, ReplicatorCall{Store: id, Remote: mirror, Mirror: true})
	return r.MirrorErrs[mirror.URL]
}

func (r *Replicator) Unsent(_ context.Context, id port.StoreID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.UnsentN[id], nil
}

// Calls は、受けた呼び出しを順に返す。
func (r *Replicator) Calls() []ReplicatorCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}
```

`internal/app/apptest/synctargets.go`:

```go
package apptest

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
)

// SyncTargets は port.SyncTargets の偽物。
type SyncTargets struct {
	mu      sync.Mutex
	Targets []port.SyncTarget
}

func (s *SyncTargets) List(context.Context) ([]port.SyncTarget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.Targets), nil
}

// Set は、一覧を入れ替える(テストの途中で設定を変えるため)。
func (s *SyncTargets) Set(ts []port.SyncTarget) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Targets = ts
}

// SyncSettings は port.SyncSettings の偽物。
type SyncSettings struct {
	mu    sync.Mutex
	Value port.SyncTiming
}

func (s *SyncSettings) Timing(context.Context) (port.SyncTiming, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Value, nil
}

// SyncTokens は port.SyncTokens の偽物。Errs にあればそのエラーを、なければ Tokens の値を返す。
type SyncTokens struct {
	mu       sync.Mutex
	Tokens   map[port.StoreID]string
	Errs     map[port.StoreID]error
	rejected []port.StoreID
}

func (s *SyncTokens) StoreToken(_ context.Context, id port.StoreID) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Errs[id]; err != nil {
		return "", err
	}
	return s.Tokens[id], nil
}

func (s *SyncTokens) StoreRejected(_ context.Context, id port.StoreID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejected = append(s.rejected, id)
}

// Rejected は、StoreRejected で知らされた Store を順に返す。
func (s *SyncTokens) Rejected() []port.StoreID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.rejected)
}

// StoreVersions は port.StoreVersions の偽物。
type StoreVersions struct {
	mu    sync.Mutex
	Value map[port.StoreID]string
}

func (s *StoreVersions) Versions(context.Context) (map[port.StoreID]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.Value), nil
}

// Set は、版を入れ替える。
func (s *StoreVersions) Set(v map[port.StoreID]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Value = v
}
```

`internal/app/apptest/events.go`:

```go
package apptest

import (
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Events は port.Events の偽物。知らされた出来事を覚える。
type Events struct {
	mu   sync.Mutex
	list []port.Event
}

func (e *Events) Publish(ev port.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, ev)
}

// List は、知らされた出来事を順に返す。
func (e *Events) List() []port.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.list)
}
```

`internal/app/apptest/syncstate.go`:

```go
package apptest

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// SyncState は port.SyncState の偽物。
type SyncState struct {
	mu      sync.Mutex
	records map[port.StoreID]port.SyncRecord
	seen    map[port.StoreID][2]string // remoteKey, seen
	mirrors map[port.StoreID]map[string]port.MirrorRecord
}

func NewSyncState() *SyncState {
	return &SyncState{
		records: map[port.StoreID]port.SyncRecord{},
		seen:    map[port.StoreID][2]string{},
		mirrors: map[port.StoreID]map[string]port.MirrorRecord{},
	}
}

func (s *SyncState) Records(context.Context) ([]port.SyncRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]port.SyncRecord, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b port.SyncRecord) int { return cmp.Compare(a.Store, b.Store) })
	return out, nil
}

func (s *SyncState) Failed(_ context.Context, id port.StoreID, at time.Time, reason syncstatus.Reason) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.records[id]
	r.Store, r.LastAttempt, r.Reason = id, at, reason
	s.records[id] = r
	return nil
}

func (s *SyncState) Succeeded(_ context.Context, id port.StoreID, at time.Time, remoteKey, seen string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[id] = port.SyncRecord{Store: id, LastAttempt: at, LastSuccess: at}
	s.seen[id] = [2]string{remoteKey, seen}
	return nil
}

func (s *SyncState) Seen(_ context.Context, id port.StoreID, remoteKey string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.seen[id]; ok && v[0] == remoteKey {
		return v[1], nil
	}
	return "", nil
}

func (s *SyncState) Mirrored(_ context.Context, id port.StoreID, mirror string, at time.Time, reason syncstatus.Reason) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mirrors[id] == nil {
		s.mirrors[id] = map[string]port.MirrorRecord{}
	}
	s.mirrors[id][mirror] = port.MirrorRecord{Mirror: mirror, At: at, Reason: reason}
	return nil
}

func (s *SyncState) Mirrors(_ context.Context, id port.StoreID) ([]port.MirrorRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []port.MirrorRecord{}
	for _, m := range s.mirrors[id] {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b port.MirrorRecord) int { return cmp.Compare(a.Mirror, b.Mirror) })
	return out, nil
}
```

`internal/app/apptest/syncstate_contract.go`:

```go
package apptest

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// RunSyncStateContract は、port.SyncState が満たすべき振る舞いを確かめる。偽物と、本物の実装
// (infra/statedb)の両方のテストから呼ぶ。newState は、空の状態を返す。時刻は UTC で比べる。
func RunSyncStateContract(t *testing.T, newState func(t *testing.T) port.SyncState) {
	t1 := time.Date(2026, 11, 13, 10, 0, 0, 123_000_000, time.UTC)
	t2 := t1.Add(time.Hour)
	ctx := context.Background()
	t.Run("empty at first", func(t *testing.T) {
		s := newState(t)
		if rs, err := s.Records(ctx); err != nil || len(rs) != 0 {
			t.Errorf("Records = %+v, %v", rs, err)
		}
		if seen, err := s.Seen(ctx, "S1", "github.com/o/r"); err != nil || seen != "" {
			t.Errorf("Seen = %q, %v", seen, err)
		}
	})
	t.Run("failure keeps the last success and the seen position", func(t *testing.T) {
		s := newState(t)
		must(t, s.Succeeded(ctx, "S1", t1, "github.com/o/r", "v1"))
		must(t, s.Failed(ctx, "S1", t2, syncstatus.AuthFailed))
		rs, err := s.Records(ctx)
		if err != nil || len(rs) != 1 {
			t.Fatalf("Records = %+v, %v", rs, err)
		}
		r := rs[0]
		if r.Store != "S1" || !r.LastAttempt.Equal(t2) || !r.LastSuccess.Equal(t1) || r.Reason != syncstatus.AuthFailed {
			t.Errorf("record = %+v", r)
		}
		if seen, _ := s.Seen(ctx, "S1", "github.com/o/r"); seen != "v1" {
			t.Errorf("Seen = %q", seen)
		}
	})
	t.Run("success clears the reason and remembers the remote", func(t *testing.T) {
		s := newState(t)
		must(t, s.Failed(ctx, "S2", t1, syncstatus.Unreachable))
		must(t, s.Succeeded(ctx, "S2", t2, "github.com/o/r", "v2"))
		must(t, s.Failed(ctx, "S1", t1, syncstatus.NotStore))
		rs, _ := s.Records(ctx)
		if len(rs) != 2 || rs[0].Store != "S1" || rs[1].Store != "S2" {
			t.Fatalf("Records = %+v", rs)
		}
		if r := rs[1]; r.Reason != "" || !r.LastSuccess.Equal(t2) || !r.LastAttempt.Equal(t2) {
			t.Errorf("S2 = %+v", r)
		}
		if r := rs[0]; !r.LastSuccess.IsZero() || r.Reason != syncstatus.NotStore {
			t.Errorf("S1 = %+v", r)
		}
		if seen, _ := s.Seen(ctx, "S2", "github.com/o/other"); seen != "" {
			t.Errorf("Seen for another remote = %q", seen)
		}
	})
	t.Run("mirrors keep the last result per mirror", func(t *testing.T) {
		s := newState(t)
		must(t, s.Mirrored(ctx, "S1", "https://b.invalid/m", t1, syncstatus.MirrorAhead))
		must(t, s.Mirrored(ctx, "S1", "https://a.invalid/m", t1, ""))
		must(t, s.Mirrored(ctx, "S1", "https://b.invalid/m", t2, ""))
		ms, err := s.Mirrors(ctx, "S1")
		if err != nil || len(ms) != 2 || ms[0].Mirror != "https://a.invalid/m" || ms[1].Reason != "" || !ms[1].At.Equal(t2) {
			t.Errorf("Mirrors = %+v, %v", ms, err)
		}
		if ms, _ := s.Mirrors(ctx, "S2"); len(ms) != 0 {
			t.Errorf("Mirrors of another store = %+v", ms)
		}
	})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
```

(`must` が apptest にすでにあれば、そちらを使い、ここでは定義しない。)

`internal/infra/gitstore/sync_scenario_test.go` の `var _ port.Replicator = (*Replicator)(nil)` を消す(gitstore の `Replicator` は Task 5・6 で `Unsent`・`Mirror` を持つまで口を満たさない。Task 6 で戻す)。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go build ./... && go test ./internal/app/... ./internal/infra/gitstore/ && golangci-lint run ./internal/app/... ./internal/infra/gitstore/`
Expected: PASS、指摘なし

- [ ] **Step 6: コミットする**

```bash
git add internal/app/port internal/app/apptest internal/infra/gitstore/sync_scenario_test.go
git commit -m "feat(port): 同期の実行に要る口(Store の設定・トークン・タイミング・状態・出来事・版)と偽物を足す(C2 Task 2)"
```

PR は出さない(出すかどうかと分け方は、全タスクのあとでユーザーに確かめる。共有部分を先に出すなら、この時点のコミットから `feat/track-c2-port` を切って出す)。

---
### Task 3: このマシンの同期の状態(infra/statedb)

**Files:**
- Create: `internal/infra/statedb/db.go`・`migrate.go`・`sync.go`
- Test: `internal/infra/statedb/db_test.go`・`sync_test.go`
- Modify: `go.mod`・`go.sum`

**Interfaces:**
- Consumes: `port.SyncState`・`apptest.RunSyncStateContract`(Task 2)
- Produces: `func Open(path string) (*DB, error)`・`func (d *DB) Close() error`・`func (d *DB) SyncState() port.SyncState`。表を足す手順は、パッケージの中で `register(name, sql string)` を `init` から呼ぶ(名前は「機能/連番」)

- [ ] **Step 1: 依存を足す**

```bash
go get modernc.org/sqlite@v1.61.1
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/infra/statedb/sync_test.go`:

```go
package statedb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func openTemp(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "hedgehug", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestSyncStateContract(t *testing.T) {
	apptest.RunSyncStateContract(t, func(t *testing.T) port.SyncState { return openTemp(t).SyncState() })
}

func TestSyncStateSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	at := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := d.SyncState().Succeeded(ctx, "S1", at, "github.com/o/r", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := d.SyncState().Failed(ctx, "S1", at.Add(time.Minute), syncstatus.Rewritten); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	rs, err := d.SyncState().Records(ctx)
	if err != nil || len(rs) != 1 || rs[0].Reason != syncstatus.Rewritten || !rs[0].LastSuccess.Equal(at) {
		t.Errorf("Records after reopen = %+v, %v", rs, err)
	}
	if seen, _ := d.SyncState().Seen(ctx, "S1", "github.com/o/r"); seen != "v1" {
		t.Errorf("Seen after reopen = %q", seen)
	}
}
```

`internal/infra/statedb/db_test.go`:

```go
package statedb

import (
	"path/filepath"
	"testing"
)

// 表を作る手順は、開くたびに当てても 1 度しか当たらない。
func TestMigrationsRunOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	for range 2 {
		d, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if err := d.db.QueryRow(`SELECT count(*) FROM migrations`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != len(migrations) {
			t.Errorf("migrations = %d, want %d", n, len(migrations))
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// 手順は名前の順に当てる(登録の順ではない)。
func TestMigrationsAreSortedByName(t *testing.T) {
	saved := migrations
	t.Cleanup(func() { migrations = saved })
	migrations = nil
	register("b/001", `CREATE TABLE b (x TEXT)`)
	register("a/001", `CREATE TABLE a (x TEXT)`)
	register("a/002", `ALTER TABLE a ADD COLUMN y TEXT`)
	d, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if _, err := d.db.Exec(`INSERT INTO a (x, y) VALUES ('1', '2')`); err != nil {
		t.Errorf("a.y missing: %v", err)
	}
}
```

- [ ] **Step 3: テストが失敗することを確かめる**

Run: `go test ./internal/infra/statedb/`
Expected: FAIL(`Open` がない)

- [ ] **Step 4: 実装する**

`internal/infra/statedb/db.go`:

```go
// Package statedb は、このマシンのデータ(~/.local/share/hedgehug/state.db、SQLite)を持つ(保存形式「このマシンに持つもの」)。
// 機能ごとに表を分け、表を作る手順は機能のファイルの init で register する。
package statedb

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // database/sql の "sqlite"
)

// DB は state.db。
type DB struct {
	db *sql.DB
}

// Open は、path の state.db を開き(なければフォルダごと作り)、まだ当てていない表を作る手順を当てる。
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	// 書き込みは 1 本の接続で順に行う(SQLite は同時に 1 つしか書けない)。
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }
```

`internal/infra/statedb/migrate.go`:

```go
package statedb

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
	"slices"
)

// migration は、表を作る 1 つの手順。名前は「機能/連番」(例 "sync/001")。一度当てた手順は変えない
// (変えるときは、新しい手順を足す)。
type migration struct {
	name string
	sql  string
}

// migrations は、機能ごとのファイルの init で足す。
var migrations []migration

func register(name, stmt string) { migrations = append(migrations, migration{name: name, sql: stmt}) }

// migrate は、まだ当てていない手順を、名前の順に 1 つずつ当てる。
func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (name TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	ms := slices.Clone(migrations)
	slices.SortFunc(ms, func(a, b migration) int { return cmp.Compare(a.name, b.name) })
	for _, m := range ms {
		if err := apply(db, m); err != nil {
			return fmt.Errorf("state.db %s: %w", m.name, err)
		}
	}
	return nil
}

func apply(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	var name string
	err = tx.QueryRow(`SELECT name FROM migrations WHERE name = ?`, m.name).Scan(&name)
	switch {
	case err == nil:
		return tx.Rollback()
	case !errors.Is(err, sql.ErrNoRows):
		return errors.Join(err, tx.Rollback())
	}
	if _, err := tx.Exec(m.sql); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if _, err := tx.Exec(`INSERT INTO migrations (name) VALUES (?)`, m.name); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}
```

`internal/infra/statedb/sync.go`:

```go
package statedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func init() {
	register("sync/001", `CREATE TABLE sync_status (
		store        TEXT PRIMARY KEY,
		last_attempt TEXT NOT NULL DEFAULT '',
		last_success TEXT NOT NULL DEFAULT '',
		reason       TEXT NOT NULL DEFAULT '',
		remote_key   TEXT NOT NULL DEFAULT '',
		seen         TEXT NOT NULL DEFAULT ''
	)`)
	register("sync/002", `CREATE TABLE mirror_status (
		store  TEXT NOT NULL,
		mirror TEXT NOT NULL,
		at     TEXT NOT NULL,
		reason TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (store, mirror)
	)`)
}

// SyncState は、同期の状態(port.SyncState)。
func (d *DB) SyncState() port.SyncState { return syncState{d.db} }

type syncState struct{ db *sql.DB }

// 時刻は UTC の RFC 3339(ナノ秒まで)で書く。ゼロは空。
func timeText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

func (s syncState) Records(ctx context.Context) ([]port.SyncRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT store, last_attempt, last_success, reason FROM sync_status ORDER BY store`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []port.SyncRecord{}
	for rows.Next() {
		var store, attempt, success, reason string
		if err := rows.Scan(&store, &attempt, &success, &reason); err != nil {
			return nil, err
		}
		r := port.SyncRecord{Store: port.StoreID(store), Reason: syncstatus.Reason(reason)}
		if r.LastAttempt, err = parseTime(attempt); err != nil {
			return nil, err
		}
		if r.LastSuccess, err = parseTime(success); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s syncState) Failed(ctx context.Context, store port.StoreID, at time.Time, reason syncstatus.Reason) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_status (store, last_attempt, reason) VALUES (?, ?, ?)
		ON CONFLICT (store) DO UPDATE SET last_attempt = excluded.last_attempt, reason = excluded.reason`,
		string(store), timeText(at), string(reason))
	return err
}

func (s syncState) Succeeded(ctx context.Context, store port.StoreID, at time.Time, remoteKey, seen string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_status (store, last_attempt, last_success, reason, remote_key, seen)
		VALUES (?, ?, ?, '', ?, ?)
		ON CONFLICT (store) DO UPDATE SET last_attempt = excluded.last_attempt, last_success = excluded.last_success,
			reason = '', remote_key = excluded.remote_key, seen = excluded.seen`,
		string(store), timeText(at), timeText(at), remoteKey, seen)
	return err
}

func (s syncState) Seen(ctx context.Context, store port.StoreID, remoteKey string) (string, error) {
	var key, seen string
	err := s.db.QueryRowContext(ctx, `SELECT remote_key, seen FROM sync_status WHERE store = ?`, string(store)).Scan(&key, &seen)
	switch {
	case err == sql.ErrNoRows:
		return "", nil
	case err != nil:
		return "", err
	case key != remoteKey:
		return "", nil
	}
	return seen, nil
}

func (s syncState) Mirrored(ctx context.Context, store port.StoreID, mirror string, at time.Time, reason syncstatus.Reason) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mirror_status (store, mirror, at, reason) VALUES (?, ?, ?, ?)
		ON CONFLICT (store, mirror) DO UPDATE SET at = excluded.at, reason = excluded.reason`,
		string(store), mirror, timeText(at), string(reason))
	return err
}

func (s syncState) Mirrors(ctx context.Context, store port.StoreID) ([]port.MirrorRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT mirror, at, reason FROM mirror_status WHERE store = ? ORDER BY mirror`, string(store))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []port.MirrorRecord{}
	for rows.Next() {
		var m, at, reason string
		if err := rows.Scan(&m, &at, &reason); err != nil {
			return nil, err
		}
		t, err := parseTime(at)
		if err != nil {
			return nil, err
		}
		out = append(out, port.MirrorRecord{Mirror: m, At: t, Reason: syncstatus.Reason(reason)})
	}
	return out, rows.Err()
}
```

(`err == sql.ErrNoRows` は lint(errorlint)が指摘したら `errors.Is` にする。)

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test ./internal/infra/statedb/ && golangci-lint run ./internal/infra/statedb/ && make tidy-check`
Expected: PASS、指摘なし

- [ ] **Step 6: コミットする**

```bash
git add go.mod go.sum internal/infra/statedb
git commit -m "feat(infra): このマシンの同期の状態を state.db(SQLite)に置く(C2 Task 3)"
```

---

### Task 4: 通信の失敗の見分けと、60 秒止まったときの中断(infra/gitstore)

**Files:**
- Create: `internal/infra/gitstore/sync_transport.go`
- Modify: `internal/infra/gitstore/sync_git.go`(`fetch`・`send`・`tip` に見張りと見分けを入れ、`fetchTo` を足す)
- Test: `internal/infra/gitstore/sync_transport_test.go`・`internal/infra/gitstore/sync_errors_test.go`

**Interfaces:**
- Consumes: `port.ErrAuthFailed`・`ErrRemoteUnreadable`・`ErrWriteDenied`・`ErrUnreachable`(Task 2)
- Produces(gitstore の中):
  - `var stallAfter = 60 * time.Second`(テストで縮める)
  - `func watch(ctx context.Context) (context.Context, func())`・`var errStalled`
  - `func classify(ctx context.Context, err error, sending bool) error`
  - `func (l link) fetchTo(ctx context.Context, repo *git.Repository, branch plumbing.ReferenceName, prefix string) (plumbing.Hash, error)`(Task 6 が使う)

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/gitstore/sync_transport_test.go`:

```go
package gitstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 見張りは、触っているあいだは止めず、stallAfter のあいだ触らなければ errStalled で取り消す。
func TestWatchCancelsOnlyWhenNothingMoves(t *testing.T) {
	saved := stallAfter
	stallAfter = 200 * time.Millisecond
	t.Cleanup(func() { stallAfter = saved })
	ctx, done := watch(context.Background())
	defer done()
	w := ctx.Value(watchKey{}).(*watchdog)
	for range 6 {
		time.Sleep(50 * time.Millisecond)
		w.touch()
	}
	if ctx.Err() != nil {
		t.Fatalf("cancelled while moving: %v", context.Cause(ctx))
	}
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("not cancelled after stalling")
	}
	if !errors.Is(context.Cause(ctx), errStalled) {
		t.Errorf("cause = %v", context.Cause(ctx))
	}
}
```

`internal/infra/gitstore/sync_errors_test.go`:

```go
package gitstore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// localMachine は、Store を作ってサンプルデータを保存し、まだどこにも送っていないマシン。
func localMachine(t *testing.T) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a")
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, dir, "A", t0)
	if err := apptest.Sample(t.Context(), m.stores, m.id); err != nil {
		t.Fatal(err)
	}
	return m
}

// statusServer は、どの要求にも code を返す HTTP の remote。
func statusServer(t *testing.T, code int) port.Remote {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if code == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="x"`)
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return port.Remote{URL: srv.URL + "/o/r.git", Token: "tok-taro"}
}

// 06「同期のエラー」: 受け取りで断られたら、Store を変えずに理由を返す。
func TestSyncClassifiesRefusalsWhileReceiving(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{http.StatusUnauthorized, port.ErrAuthFailed},
		{http.StatusForbidden, port.ErrRemoteUnreadable},
		{http.StatusNotFound, port.ErrRemoteUnreadable},
		{http.StatusInternalServerError, port.ErrUnreachable},
	}
	for _, c := range cases {
		t.Run(http.StatusText(c.code), func(t *testing.T) {
			m := localMachine(t)
			before := snap(t, m)
			_, err := m.rep.Sync(t.Context(), m.id, statusServer(t, c.code))
			if !errors.Is(err, c.want) {
				t.Errorf("Sync = %v, want %v", err, c.want)
			}
			unchanged(t, m, before)
		})
	}
}

func TestSyncUnreachableWhenNothingListens(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	m := localMachine(t)
	if _, err := m.rep.Sync(t.Context(), m.id, port.Remote{URL: url + "/o/r.git"}); !errors.Is(err, port.ErrUnreachable) {
		t.Errorf("Sync = %v", err)
	}
}

// 06「同期のエラー」: git の通信が stallAfter のあいだまったく進まなければ、中断して「接続できません」。
func TestSyncGivesUpWhenTheRemoteStalls(t *testing.T) {
	saved := stallAfter
	stallAfter = 300 * time.Millisecond
	t.Cleanup(func() { stallAfter = saved })
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // 何も返さずに待ち続ける
	}))
	t.Cleanup(srv.Close)
	m := localMachine(t)
	start := time.Now()
	_, err := m.rep.Sync(t.Context(), m.id, port.Remote{URL: srv.URL + "/o/r.git"})
	if !errors.Is(err, port.ErrUnreachable) {
		t.Errorf("Sync = %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("took %v", d)
	}
}

// 06「同期のエラー」: 受け取れても、送信で 403 なら「書き込みが許可されていません」。
func TestRealGitSyncWriteDenied(t *testing.T) {
	remote, _ := newRealRemote(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("service") == "git-receive-pack" || strings.HasSuffix(r.URL.Path, "/git-receive-pack") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	m := localMachine(t)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrWriteDenied) {
		t.Errorf("Sync = %v", err)
	}
}
```

(`newRealRemote` の bare が空なら、最初の送信で 403 になる。空の remote は受け取りが通るので、送信の 403 を確かめられる。)

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/infra/gitstore/ -run 'Watch|Classifies|Unreachable|Stalls|WriteDenied'`
Expected: FAIL(`watch`・`stallAfter` がない。エラーが port のものでない)

- [ ] **Step 3: 見張りと見分けを実装する**

`internal/infra/gitstore/sync_transport.go`:

```go
package gitstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/app/port"
)

// stallAfter は、git の通信がまったく進まないとき、中断するまでの時間(06「同期のエラー」)。テストで縮める。
var stallAfter = 60 * time.Second

// errStalled は、通信が stallAfter のあいだ進まなかったこと。
var errStalled = errors.New("git: no progress")

// go-git は remote ごとの HTTP クライアントを受け取らないので、http と https の通信に見張りを入れる。
// 見張りは要求の context にあるときだけ働く(watch の外の通信は、そのまま通す)。
func init() {
	t := githttp.NewClient(&http.Client{Transport: watchedTransport{base: http.DefaultTransport}})
	client.InstallProtocol("https", t)
	client.InstallProtocol("http", t)
}

type watchKey struct{}

type watchdog struct {
	timer *time.Timer
	after time.Duration
}

func (w *watchdog) touch() { w.timer.Reset(w.after) }

// watch は、通信が stallAfter のあいだ進まなければ errStalled で取り消される ctx を返す。done で見張りを止める。
// 合わせる計算の時間は数えないよう、通信の呼び出しだけを包む。
func watch(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	w := &watchdog{after: stallAfter}
	w.timer = time.AfterFunc(stallAfter, func() { cancel(errStalled) })
	return context.WithValue(ctx, watchKey{}, w), func() {
		w.timer.Stop()
		cancel(nil)
	}
}

// watchedTransport は、要求と応答の本文を読み書きするたびに、見張りの時計を戻す。
type watchedTransport struct{ base http.RoundTripper }

func (t watchedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w, _ := req.Context().Value(watchKey{}).(*watchdog)
	if w == nil {
		return t.base.RoundTrip(req)
	}
	w.touch()
	if req.Body != nil {
		req = req.Clone(req.Context())
		req.Body = touchingBody{ReadCloser: req.Body, w: w}
	}
	res, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	w.touch()
	res.Body = touchingBody{ReadCloser: res.Body, w: w}
	return res, nil
}

type touchingBody struct {
	io.ReadCloser
	w *watchdog
}

func (b touchingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.w.touch()
	}
	return n, err
}

// classify は、remote との通信の失敗を、06「同期のエラー」の理由に当たる port のエラーに直す(決定 C2-7)。
// ctx は watch の ctx、sending は送信の途中か。
func classify(ctx context.Context, err error, sending bool) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(context.Cause(ctx), errStalled):
		return fmt.Errorf("%w: %w", port.ErrUnreachable, errStalled)
	case errors.Is(err, transport.ErrAuthenticationRequired):
		return fmt.Errorf("%w: %w", port.ErrAuthFailed, err)
	case errors.Is(err, transport.ErrAuthorizationFailed) && sending:
		return fmt.Errorf("%w: %w", port.ErrWriteDenied, err)
	case errors.Is(err, transport.ErrAuthorizationFailed), errors.Is(err, transport.ErrRepositoryNotFound):
		return fmt.Errorf("%w: %w", port.ErrRemoteUnreadable, err)
	}
	return fmt.Errorf("%w: %w", port.ErrUnreachable, err)
}
```

- [ ] **Step 4: 通信の呼び出しを包む**

`internal/infra/gitstore/sync_git.go` の `fetch` を、次の `fetch` と `fetchTo` に替える(中身は prefix を引数にしたことと、見張りと見分けを入れたことだけが違う)。

```go
// fetch は、remote の全ブランチを受け取り、branch と同じ名前のブランチの先端を返す。
// remote が空なら、ゼロのハッシュと nil。remote が空でなく、そのブランチがなければ port.ErrNoCommonHistory。
func (l link) fetch(ctx context.Context, repo *git.Repository, branch plumbing.ReferenceName) (plumbing.Hash, error) {
	return l.fetchTo(ctx, repo, branch, remotePrefix)
}

// fetchTo は、fetch と同じことを、受け取ったブランチを prefix の下に置いて行う(ミラーは別の場所に置く)。
func (l link) fetchTo(ctx context.Context, repo *git.Repository, branch plumbing.ReferenceName, prefix string) (plumbing.Hash, error) {
	wctx, done := watch(ctx)
	defer done()
	err := l.remote.FetchContext(wctx, &git.FetchOptions{
		RefSpecs:   []config.RefSpec{config.RefSpec("+refs/heads/*:" + prefix + "*")},
		RemoteName: remoteName,
		Auth:       l.auth,
		Force:      true,
		// remote で消えたブランチは手元からも消す。消えた main の古い先端と合わせない(決定 4)。
		Prune: true,
		Tags:  git.NoTags,
	})
	switch {
	case errors.Is(err, transport.ErrEmptyRemoteRepository):
		return plumbing.ZeroHash, nil
	case err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate):
		return plumbing.ZeroHash, classify(wctx, err, false)
	}
	ref, err := repo.Reference(plumbing.ReferenceName(prefix+branch.Short()), true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.ZeroHash, port.ErrNoCommonHistory
	}
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return ref.Hash(), nil
}
```

`send` の最初の行を見張りで包み、失敗の返し方を見分けに替える。

```go
func (l link) send(ctx context.Context, branch plumbing.ReferenceName, theirs, mine plumbing.Hash) error {
	wctx, done := watch(ctx)
	defer done()
	spec := config.RefSpec(branch.String() + ":" + branch.String())
	err := l.remote.PushContext(wctx, &git.PushOptions{RemoteName: remoteName, RefSpecs: []config.RefSpec{spec}, Auth: l.auth})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	// (C1 のコメントはそのまま残す)
	if strings.Contains(err.Error(), "non-fast-forward") {
		return errRejected
	}
	// (C1 のコメントはそのまま残す)
	tip, lerr := l.tip(ctx, branch)
	if lerr != nil {
		return errors.Join(classify(wctx, err, true), lerr)
	}
	if tip == mine {
		return nil
	}
	if tip != theirs {
		return errRejected
	}
	return classify(wctx, err, true)
}
```

`tip` を見張りで包み、失敗を見分ける。

```go
func (l link) tip(ctx context.Context, branch plumbing.ReferenceName) (plumbing.Hash, error) {
	wctx, done := watch(ctx)
	defer done()
	refs, err := l.remote.ListContext(wctx, &git.ListOptions{Auth: l.auth})
	if errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return plumbing.ZeroHash, nil
	}
	if err != nil {
		return plumbing.ZeroHash, classify(wctx, err, false)
	}
	for _, r := range refs {
		if r.Name() == branch {
			return r.Hash(), nil
		}
	}
	return plumbing.ZeroHash, nil
}
```

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ && golangci-lint run ./internal/infra/gitstore/`
Expected: PASS(C1 のテストも含む)、指摘なし。`sync_git.go` が 300 行を超えたら、`fetchTo` と `tip` を `sync_link.go` に移す

- [ ] **Step 6: コミットする**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): remote との通信の失敗を見分け、60 秒進まない通信を中断する(C2 Task 4)"
```

---

### Task 5: 履歴の書き換えの検出・受け取りだけの同期・未送信の数(infra/gitstore)

**Files:**
- Modify: `internal/infra/gitstore/replicator.go`(`syncRun` に `seen`・`receiveOnly`、`once` と `result`)
- Create: `internal/infra/gitstore/replicator_seen.go`(`checkSeen`・`Unsent`)
- Test: `internal/infra/gitstore/sync_seen_test.go`

**Interfaces:**
- Consumes: `port.Remote.Seen`・`ReceiveOnly`、`port.SyncResult.Seen`、`port.ErrRemoteRewritten`(Task 2)
- Produces: `func (r *Replicator) Unsent(ctx context.Context, id port.StoreID) (int, error)`。`Sync` の結果の `Seen` は、成功のあとに remote が持つ先端のハッシュ(16 進 40 文字)

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/gitstore/sync_seen_test.go`:

```go
package gitstore

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// bareOf は、テストの remote の bare のフォルダ。
func bareOf(t *testing.T, remote port.Remote) string {
	t.Helper()
	remotesMu.Lock()
	defer remotesMu.Unlock()
	dir, ok := remotes[strings.TrimPrefix(remote.URL, "https://remote.invalid")]
	if !ok {
		t.Fatalf("no bare for %s", remote.URL)
	}
	return dir
}

// setRemoteBranch は、remote の main を h にする(h がゼロなら消す)。利用者が git で書き換えた場面を作る。
func setRemoteBranch(t *testing.T, remote port.Remote, h plumbing.Hash) {
	t.Helper()
	repo, err := git.PlainOpen(bareOf(t, remote))
	if err != nil {
		t.Fatal(err)
	}
	name := plumbing.NewBranchReferenceName("main")
	if h.IsZero() {
		err = repo.Storer.RemoveReference(name)
	} else {
		err = repo.Storer.SetReference(plumbing.NewHashReference(name, h))
	}
	if err != nil {
		t.Fatal(err)
	}
}

func remoteHead(t *testing.T, remote port.Remote) plumbing.Hash {
	t.Helper()
	repo, err := git.PlainOpen(bareOf(t, remote))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := repo.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash()
}

func rename(t *testing.T, m machine, title string) {
	t.Helper()
	m.write(t, "タイトルを変更", set("title", "", title))
}

// 成功した同期は、remote が持つ先端を Seen として返す。次の同期にその Seen を渡しても、続きなら通る。
func TestSyncReturnsSeenAndContinuesFromIt(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Minute))
	resA, err := a.rep.Sync(t.Context(), a.id, remote)
	if err != nil {
		t.Fatal(err)
	}
	if resA.Seen != remoteHead(t, remote).String() {
		t.Fatalf("Seen = %q, remote head %s", resA.Seen, remoteHead(t, remote))
	}
	rename(t, b, "B の名前")
	b.sync(t, remote)
	resA2, err := a.rep.Sync(t.Context(), a.id, port.Remote{URL: remote.URL, Seen: resA.Seen})
	if err != nil || resA2.Received != 1 || resA2.Seen != headCommit(t, a.dir).Hash.String() {
		t.Errorf("second sync = %+v, %v", resA2, err)
	}
}

// 保存形式「remote の履歴が書き換えられたとき」: 前に受け取った位置の続きでなければ、Store を変えずに止める。
// 巻き戻したとき、空にしたとき(決定 C2-12)も同じ。
func TestSyncStopsWhenTheRemoteWasRewritten(t *testing.T) {
	cases := map[string]func(t *testing.T, remote port.Remote, old plumbing.Hash){
		"rewound": func(t *testing.T, remote port.Remote, old plumbing.Hash) { setRemoteBranch(t, remote, old) },
		"emptied": func(t *testing.T, remote port.Remote, _ plumbing.Hash) { setRemoteBranch(t, remote, plumbing.ZeroHash) },
	}
	for name, rewrite := range cases {
		t.Run(name, func(t *testing.T) {
			remote := newRemote(t)
			a := firstMachine(t, remote)
			old := remoteHead(t, remote)
			rename(t, a, "新しい名前")
			res, err := a.rep.Sync(t.Context(), a.id, remote)
			if err != nil {
				t.Fatal(err)
			}
			rewrite(t, remote, old)
			rename(t, a, "未送信の名前")
			before := snap(t, a)
			_, err = a.rep.Sync(t.Context(), a.id, port.Remote{URL: remote.URL, Seen: res.Seen})
			if !errors.Is(err, port.ErrRemoteRewritten) {
				t.Errorf("Sync = %v", err)
			}
			unchanged(t, a, before)
		})
	}
}

// 前に受け取った位置がなければ(付け替えた・追加した直後)、書き換えの検査はしない。読めない位置も同じ。
func TestSyncWithoutSeenSkipsTheRewriteCheck(t *testing.T) {
	for _, seen := range []string{"", "not-a-hash"} {
		remote := newRemote(t)
		a := firstMachine(t, remote)
		old := remoteHead(t, remote)
		rename(t, a, "新しい名前")
		a.sync(t, remote)
		setRemoteBranch(t, remote, old)
		if _, err := a.rep.Sync(t.Context(), a.id, port.Remote{URL: remote.URL, Seen: seen}); err != nil {
			t.Errorf("Seen %q: %v", seen, err)
		}
	}
}

// 決定 C2-13: 受け取りだけの同期は、合わせても送らない。
func TestSyncReceiveOnlyDoesNotSend(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Minute))
	rename(t, a, "A の名前")
	a.sync(t, remote)
	rename(t, b, "B の名前")
	before := remoteHead(t, remote)
	res, err := b.rep.Sync(t.Context(), b.id, port.Remote{URL: remote.URL, ReceiveOnly: true})
	if err != nil || res.Sent != 0 || res.Received != 1 || res.Seen != before.String() {
		t.Errorf("Sync = %+v, %v", res, err)
	}
	if remoteHead(t, remote) != before {
		t.Error("the remote moved")
	}
}

// 決定 C2-14: 未送信は、前に受け取った remote の先端から辿れない保存の数。
func TestUnsentCountsSavesNotSent(t *testing.T) {
	m := localMachine(t)
	n, err := m.rep.Unsent(t.Context(), m.id)
	if err != nil || n == 0 {
		t.Fatalf("never synced: %d, %v", n, err)
	}
	remote := newRemote(t)
	m.sync(t, remote)
	if n, err := m.rep.Unsent(t.Context(), m.id); err != nil || n != 0 {
		t.Errorf("after sync: %d, %v", n, err)
	}
	rename(t, m, "1")
	rename(t, m, "2")
	if n, err := m.rep.Unsent(t.Context(), m.id); err != nil || n != 2 {
		t.Errorf("after 2 saves: %d, %v", n, err)
	}
}
```

`set` は `sync_scenario_test.go` のもの(サンプルのタスク `apptest.SampleTask` の値を変える)。

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/infra/gitstore/ -run 'Seen|Rewritten|ReceiveOnly|Unsent'`
Expected: FAIL(`Seen` が空、書き換えを見つけない、`Unsent` がない)

- [ ] **Step 3: 実装する**

`internal/infra/gitstore/replicator_seen.go`:

```go
package gitstore

import (
	"context"
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// checkSeen は、remote の先端 theirs が、前に受け取った位置 s.seen の続きかを確かめる(保存形式「remote の
// 履歴が書き換えられたとき」の見つけ方)。続きでなければ port.ErrRemoteRewritten。位置がなければ確かめない。
func (s syncRun) checkSeen(theirs plumbing.Hash) error {
	if s.seen.IsZero() || s.seen == theirs {
		return nil
	}
	if theirs.IsZero() {
		return port.ErrRemoteRewritten // remote が空になった(決定 C2-12)
	}
	seen, err := s.repo.CommitObject(s.seen)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return port.ErrRemoteRewritten
	}
	if err != nil {
		return err
	}
	tc, err := s.repo.CommitObject(theirs)
	if err != nil {
		return err
	}
	ok, err := seen.IsAncestor(tc)
	if err != nil {
		return err
	}
	if !ok {
		return port.ErrRemoteRewritten
	}
	return nil
}

// parseSeen は、Remote.Seen をハッシュにする。読めなければゼロ(確かめない)。
func parseSeen(s string) plumbing.Hash {
	if !plumbing.IsHash(s) {
		return plumbing.ZeroHash
	}
	return plumbing.NewHash(s)
}

// Unsent は、remote に送っていない保存の数(決定 C2-14)。前の同期で受け取った remote の先端から辿れない保存を
// 数える。一度も受け取っていなければ、すべての保存を数える。
func (r *Replicator) Unsent(_ context.Context, id port.StoreID) (int, error) {
	root, err := r.stores.dir(id)
	if err != nil {
		return 0, err
	}
	repo, err := git.PlainOpen(root)
	if err != nil {
		return 0, err
	}
	head, err := repo.Head()
	if err != nil {
		return 0, err
	}
	var theirs plumbing.Hash
	ref, err := repo.Reference(plumbing.ReferenceName(remotePrefix+head.Name().Short()), true)
	switch {
	case err == nil:
		theirs = ref.Hash()
	case !errors.Is(err, plumbing.ErrReferenceNotFound):
		return 0, err
	}
	return countSaves(repo, head.Hash(), theirs)
}
```

`internal/infra/gitstore/replicator.go` を変える。

`Sync` の `syncRun` を作る行を次に替える。

```go
	s := syncRun{
		r: r, repo: repo, root: root, link: newLink(repo, remote), branch: head.Name(), start: head.Hash(),
		seen: parseSeen(remote.Seen), receiveOnly: remote.ReceiveOnly,
	}
```

`syncRun` に 2 つの欄を足す。

```go
	// seen は、前の同期で同じ remote から受け取った位置(ゼロなら確かめない)。receiveOnly なら送らない。
	seen        plumbing.Hash
	receiveOnly bool
```

`once` を次に替える。

```go
// once は、受け取り・合わせる・送るを 1 回行う。送信が拒否されたら errRejected。
func (s syncRun) once(ctx context.Context) (port.SyncResult, error) {
	theirs, err := s.link.fetch(ctx, s.repo, s.branch)
	if errors.Is(err, port.ErrNoCommonHistory) && !s.seen.IsZero() {
		return port.SyncResult{}, port.ErrRemoteRewritten // 前に受け取ったブランチが消えた(決定 C2-12)
	}
	if err != nil {
		return port.SyncResult{}, err
	}
	if err := s.checkSeen(theirs); err != nil {
		return port.SyncResult{}, err
	}
	if !theirs.IsZero() {
		if err := checkRemoteStore(s.repo, theirs); err != nil {
			return port.SyncResult{}, err
		}
		if err := s.integrate(theirs); err != nil {
			return port.SyncResult{}, err
		}
	}
	if s.receiveOnly {
		return s.result(theirs, theirs, false)
	}
	if s.r.beforeSend != nil {
		if err := s.r.beforeSend(); err != nil {
			return port.SyncResult{}, err
		}
	}
	head, err := s.repo.Head()
	if err != nil {
		return port.SyncResult{}, err
	}
	if err := s.link.send(ctx, s.branch, theirs, head.Hash()); err != nil {
		return port.SyncResult{}, err
	}
	// 送ったので、remote の先端は自分の先端になった。未送信の数え方(Unsent)の基準を進める。
	if err := s.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(remotePrefix+s.branch.Short()), head.Hash())); err != nil {
		return port.SyncResult{}, err
	}
	return s.result(theirs, head.Hash(), true)
}
```

`result` を次に替える。

```go
// result は、送った保存(remote の先端 theirs になかったもの)と、受け取った保存(始めたときの自分になかったもの)を
// 数える。now は同期のあとに remote が持つ先端(Seen になる)。sent が false なら、送った数は 0。
func (s syncRun) result(theirs, now plumbing.Hash, sent bool) (port.SyncResult, error) {
	var res port.SyncResult
	if !now.IsZero() {
		res.Seen = now.String()
	}
	if sent {
		head, err := s.repo.Head()
		if err != nil {
			return port.SyncResult{}, err
		}
		if res.Sent, err = countSaves(s.repo, head.Hash(), theirs); err != nil {
			return port.SyncResult{}, err
		}
	}
	received, err := countSaves(s.repo, theirs, s.start)
	if err != nil {
		return port.SyncResult{}, err
	}
	res.Received = received
	return res, nil
}
```

C1 のテストは `machine.sync` の結果を `port.SyncResult{Sent: 1}` のような件数だけの値と比べている。`Seen` が入ると比べが合わなくなるので、`sync_helpers_test.go` の `machine.sync` は件数だけを返すようにする(`Seen` を見るテストは `m.rep.Sync` を直接呼ぶ)。

```go
// sync は、同期して、送った数と受け取った数だけを返す(Seen は空にする。件数を比べるテストのため)。
func (m machine) sync(t *testing.T, remote port.Remote) port.SyncResult {
	t.Helper()
	res, err := m.rep.Sync(t.Context(), m.id, remote)
	if err != nil {
		t.Fatalf("sync of %s: %v", m.dir, err)
	}
	res.Seen = ""
	return res
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ && golangci-lint run ./internal/infra/gitstore/`
Expected: PASS(C1 のテストも含む)、指摘なし。`replicator.go` が 300 行以下

- [ ] **Step 5: コミットする**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): remote の履歴の書き換えを見つけ、受け取りだけの同期と未送信の数を足す(C2 Task 5)"
```

---

### Task 6: ミラーへの送信(infra/gitstore)

**Files:**
- Create: `internal/infra/gitstore/replicator_mirror.go`
- Test: `internal/infra/gitstore/sync_mirror_test.go`

**Interfaces:**
- Consumes: `link.fetchTo`・`link.send`・`errRejected`(Task 4)、`port.ErrMirrorUnrelated`・`ErrMirrorAhead`(Task 2)
- Produces: `func (r *Replicator) Mirror(ctx context.Context, id port.StoreID, mirror port.Remote) error`(これで `*Replicator` が `port.Replicator` を満たす)

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/gitstore/sync_mirror_test.go`:

```go
package gitstore

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// 保存形式「同期」の手順 7: 空のミラーには最初の送信として送り、遅れているミラーには続きを送る。
func TestMirrorSendsToEmptyAndBehindMirrors(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatalf("empty mirror: %v", err)
	}
	if remoteHead(t, mirror) != headCommit(t, a.dir).Hash {
		t.Error("the mirror did not get the head")
	}
	rename(t, a, "新しい名前")
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatalf("behind mirror: %v", err)
	}
	if remoteHead(t, mirror) != headCommit(t, a.dir).Hash {
		t.Error("the mirror did not catch up")
	}
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Errorf("same head: %v", err)
	}
}

// ミラーにだけ保存があれば、上書きしないため送らない。
func TestMirrorRefusesMirrorAhead(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatal(err)
	}
	b := cloneMachine(t, mirror, "B", t0.Add(time.Minute))
	rename(t, b, "ミラーだけの名前")
	b.sync(t, mirror)
	before := remoteHead(t, mirror)
	rename(t, a, "A の名前")
	if err := a.rep.Mirror(t.Context(), a.id, mirror); !errors.Is(err, port.ErrMirrorAhead) {
		t.Errorf("Mirror = %v", err)
	}
	if remoteHead(t, mirror) != before {
		t.Error("the mirror was overwritten")
	}
}

// 空でなく、共通の履歴がないミラーには送らない。
func TestMirrorRefusesUnrelatedMirror(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	pushRaw(t, mirror, map[string]string{"README.md": "x\n"})
	if err := a.rep.Mirror(t.Context(), a.id, mirror); !errors.Is(err, port.ErrMirrorUnrelated) {
		t.Errorf("Mirror = %v", err)
	}
}

// ミラーに送っても、remote の受け取った先端(未送信の数え方)は変わらない。
func TestMirrorDoesNotTouchTheRemoteRefs(t *testing.T) {
	remote, mirror := newRemote(t), newRemote(t)
	a := firstMachine(t, remote)
	rename(t, a, "未送信")
	if err := a.rep.Mirror(t.Context(), a.id, mirror); err != nil {
		t.Fatal(err)
	}
	if n, err := a.rep.Unsent(t.Context(), a.id); err != nil || n != 1 {
		t.Errorf("Unsent = %d, %v", n, err)
	}
}
```

(`pushRaw` が main 以外のブランチに送るなら、`TestMirrorRefusesUnrelatedMirror` はブランチがない場合として同じ `ErrMirrorUnrelated` を確かめる。)

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/infra/gitstore/ -run Mirror`
Expected: FAIL(`Mirror` がない)

- [ ] **Step 3: 実装する**

`internal/infra/gitstore/replicator_mirror.go`:

```go
package gitstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// mirrorPrefix は、ミラーから受け取ったブランチを置く場所。ミラーの URL ごとに分け、remote のもの
// (remotePrefix。未送信の数え方に使う)とは混ぜない。
const mirrorPrefix = "refs/hedgehug/mirror/"

func mirrorRefPrefix(url string) string {
	sum := sha256.Sum256([]byte(url))
	return mirrorPrefix + hex.EncodeToString(sum[:])[:12] + "/"
}

// Mirror は、Store の先端をミラーへ送る(保存形式「同期」の手順 7、決定 C2-15)。ミラーからは受け取って
// 合わせない。履歴は書き換えない(強制しない)。
func (r *Replicator) Mirror(ctx context.Context, id port.StoreID, mirror port.Remote) error {
	root, err := r.stores.dir(id)
	if err != nil {
		return err
	}
	unlock, err := open(root)
	if err != nil {
		return err
	}
	defer unlock()
	repo, err := git.PlainOpen(root)
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	l := newLink(repo, mirror)
	theirs, err := l.fetchTo(ctx, repo, head.Name(), mirrorRefPrefix(mirror.URL))
	if errors.Is(err, port.ErrNoCommonHistory) {
		return port.ErrMirrorUnrelated
	}
	if err != nil {
		return err
	}
	mine := head.Hash()
	if theirs == mine {
		return nil
	}
	if !theirs.IsZero() {
		if err := mirrorBehind(repo, theirs, mine); err != nil {
			return err
		}
	}
	err = l.send(ctx, head.Name(), theirs, mine)
	if errors.Is(err, errRejected) {
		return port.ErrMirrorAhead // 送るあいだに、ミラーが先に進んだ
	}
	return err
}

// mirrorBehind は、ミラーの先端 theirs が自分の先端 mine の祖先なら nil。共通の祖先がなければ
// ErrMirrorUnrelated、あればミラーにだけ保存があるので ErrMirrorAhead。
func mirrorBehind(repo *git.Repository, theirs, mine plumbing.Hash) error {
	tc, err := repo.CommitObject(theirs)
	if err != nil {
		return err
	}
	mc, err := repo.CommitObject(mine)
	if err != nil {
		return err
	}
	if ok, err := tc.IsAncestor(mc); err != nil || ok {
		return err
	}
	bases, err := mc.MergeBase(tc)
	if err != nil {
		return err
	}
	if len(bases) == 0 {
		return port.ErrMirrorUnrelated
	}
	return port.ErrMirrorAhead
}
```

`sync_scenario_test.go` の先頭に、Task 2 で外した `var _ port.Replicator = (*Replicator)(nil)` を戻す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ && golangci-lint run ./internal/infra/gitstore/`
Expected: PASS、指摘なし

- [ ] **Step 5: コミットする**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): ミラーへ送る(空なら最初の送信、先に進んだミラーと関係のないミラーには送らない)(C2 Task 6)"
```

---
### Task 7: 同期のタイミング(infra/machineconfig)

**Files:**
- Create: `internal/infra/machineconfig/sync.go`
- Modify: `internal/infra/machineconfig/file.go`(`config` に `Sync`)
- Test: `internal/infra/machineconfig/sync_test.go`

**Interfaces:**
- Consumes: `port.SyncSettings`・`port.SyncTiming`(Task 2)
- Produces: `func (f *File) SyncSettings() port.SyncSettings`

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/machineconfig/sync_test.go`:

```go
package machineconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestSyncTimingDefaults(t *testing.T) {
	got, err := Open(filepath.Join(t.TempDir(), "config.yaml")).SyncSettings().Timing(context.Background())
	want := port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}
	if err != nil || got != want {
		t.Errorf("Timing = %+v, %v", got, err)
	}
}

func TestSyncTimingReadsTheSyncSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sync:\n  auto: false\n  interval: 900\n  push_interval: 10\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Open(path).SyncSettings().Timing(context.Background())
	want := port.SyncTiming{Auto: false, Interval: 15 * time.Minute, AfterWrite: 10 * time.Second}
	if err != nil || got != want {
		t.Errorf("Timing = %+v, %v", got, err)
	}
}

// アカウントの追加などで config.yaml を書き直しても、sync の項目は残る。
func TestSyncSectionSurvivesRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("sync:\n  interval: 60\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := Open(path)
	a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.GhCLI}
	if err := f.Accounts().Add(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	got, err := Open(path).SyncSettings().Timing(context.Background())
	if err != nil || got.Interval != time.Minute || !got.Auto {
		t.Errorf("Timing = %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/infra/machineconfig/ -run Sync`
Expected: FAIL(`SyncSettings` がない)

- [ ] **Step 3: 実装する**

`internal/infra/machineconfig/file.go` の `config` に 1 行足す。

```go
type config struct {
	Accounts []accountYAML `yaml:"accounts"`
	Hosts    []hostYAML    `yaml:"hosts,omitempty"`
	Sync     syncYAML      `yaml:"sync,omitempty"`
}
```

`internal/infra/machineconfig/sync.go`:

```go
package machineconfig

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// syncYAML は config.yaml の sync(決定 C2-4。前回の実装と同じ鍵。秒で書く)。書かれていない項目は初期値。
type syncYAML struct {
	Auto         *bool `yaml:"auto,omitempty"`
	Interval     int   `yaml:"interval,omitempty"`      // 間隔(秒)
	PushInterval int   `yaml:"push_interval,omitempty"` // 書き込みのあと同期するまで(秒)
}

// 06「自動で同期する」の初期値。
var defaultTiming = port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}

// SyncSettings は、config.yaml の sync を読む port.SyncSettings。
func (f *File) SyncSettings() port.SyncSettings { return syncSettings{f} }

type syncSettings struct{ f *File }

func (s syncSettings) Timing(context.Context) (port.SyncTiming, error) {
	c, err := s.f.read()
	if err != nil {
		return port.SyncTiming{}, err
	}
	t := defaultTiming
	if c.Sync.Auto != nil {
		t.Auto = *c.Sync.Auto
	}
	if c.Sync.Interval > 0 {
		t.Interval = time.Duration(c.Sync.Interval) * time.Second
	}
	if c.Sync.PushInterval > 0 {
		t.AfterWrite = time.Duration(c.Sync.PushInterval) * time.Second
	}
	return t, nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/machineconfig/ && golangci-lint run ./internal/infra/machineconfig/`
Expected: PASS(D の `TestPersistsWithoutTokens` も通る。`sync` が空なら書かない)、指摘なし

- [ ] **Step 5: コミットする**

```bash
git add internal/infra/machineconfig
git commit -m "feat(machineconfig): config.yaml の sync から同期のタイミングを読む(C2 Task 7)"
```

---

### Task 8: 同期のトークンの入口(app/github の Connector)

**Files:**
- Create: `internal/app/github/sync_tokens.go`
- Test: `internal/app/github/sync_tokens_test.go`

**Interfaces:**
- Consumes: `Connector.TokenFor`・`storeAccount`・`Rejected`(D2)、`port.SyncTokens`(Task 2)
- Produces: `func (c Connector) StoreToken(ctx context.Context, store port.StoreID) (string, error)`・`func (c Connector) StoreRejected(ctx context.Context, store port.StoreID)`。`github.Connector` が `port.SyncTokens` を満たす

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/github/sync_tokens_test.go`(`connector_test.go` の `newConn` を使う。S1 はアカウント A1(work、gh、`tok-taro`)、S2 は割り当てなし):

```go
package github_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

var _ port.SyncTokens = github.Connector{}

func TestStoreTokenUsesTheAssignedAccount(t *testing.T) {
	f := newConn(t)
	tok, err := f.conn.StoreToken(context.Background(), "S1")
	if err != nil || tok != "tok-taro" {
		t.Errorf("StoreToken = %q, %v", tok, err)
	}
	if _, err := f.conn.StoreToken(context.Background(), "S2"); !ghcall.Is(err, ghcall.NoAccount) {
		t.Errorf("no account: %v", err)
	}
}

// 07「状態」: 同期で remote がトークンを拒否したら、そのアカウントを認証エラーにする。
func TestStoreRejectedMarksTheAccountBroken(t *testing.T) {
	f := newConn(t)
	f.conn.StoreRejected(context.Background(), "S1")
	if _, err := f.conn.StoreToken(context.Background(), "S1"); !ghcall.Is(err, ghcall.AuthError) {
		t.Errorf("after rejection: %v", err)
	}
	f.conn.StoreRejected(context.Background(), "S2") // 割り当てがなければ何もしない
	f.conn.StoreRejected(context.Background(), "S9")
}
```

(`newConn` の Store の値が違えば、テストの Store ID をそれに合わせる。)

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/app/github/ -run Store`
Expected: FAIL(`StoreToken` がない)

- [ ] **Step 3: 実装する**

`internal/app/github/sync_tokens.go`:

```go
package github

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// StoreToken は、Store の同期に使うトークン(port.SyncTokens。決定 C2-3)。読み直し・照合・取り直し・
// 認証エラーは TokenFor と同じ。
func (c Connector) StoreToken(ctx context.Context, store port.StoreID) (string, error) {
	tok, _, err := c.TokenFor(ctx, store, "")
	return tok, err
}

// StoreRejected は、Store に割り当てたアカウントのトークンを remote が拒否した(401)ことを受け、
// そのアカウントを認証エラーにする(07「状態」)。割り当てがなければ何もしない。
func (c Connector) StoreRejected(ctx context.Context, store port.StoreID) {
	a, err := c.storeAccount(ctx, store)
	if err != nil || a.ID == "" {
		return
	}
	c.Rejected(a.ID)
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/github/ && golangci-lint run ./internal/app/github/`
Expected: PASS、指摘なし

- [ ] **Step 5: コミットする**

```bash
git add internal/app/github
git commit -m "feat(app): 同期のトークンを Connector から得て、remote の拒否で認証エラーにする(C2 Task 8)"
```

---

### Task 9: 同期の usecase(app/syncing の Service)

**Files:**
- Create: `internal/app/syncing/service.go`・`run.go`・`reason.go`・`remotekey.go`・`all.go`・`status.go`
- Test: `internal/app/syncing/fixture_test.go`・`service_test.go`・`all_test.go`・`status_test.go`・`remotekey_test.go`

**Interfaces:**
- Consumes: Task 1 の `syncstatus`、Task 2 の口と偽物、`ghcall.Failure`
- Produces:

```go
type Service struct {
	Targets port.SyncTargets; Tokens port.SyncTokens; Replicator port.Replicator
	State port.SyncState; Events port.Events; Clock port.Clock
	AfterSuccess func(ctx context.Context, store port.StoreID) // nil でよい
	// mu・running は内部
}
type Outcome struct { Store port.StoreID; Name string; Skipped bool; Reason syncstatus.Reason; Sent, Received, MirrorFailures int }
func (o Outcome) OK() bool
func (o Outcome) Message() string
func (s *Service) SyncStore(ctx context.Context, id port.StoreID) (Outcome, error)
func (s *Service) once(ctx context.Context, t port.SyncTarget) (Outcome, error) // Task 10 の Auto が使う
type AllOutcome struct { Nothing bool; Outcomes []Outcome; Lines []string }
func (s *Service) SyncAll(ctx context.Context) (AllOutcome, error)
type StoreStatus struct { Store port.StoreID; Name string; Hidden, ReadOnly bool; Reason syncstatus.Reason; Unsent int; LastSuccess time.Time }
func (s *Service) Status(ctx context.Context) ([]StoreStatus, error)
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/syncing/fixture_test.go`:

```go
package syncing_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

var at = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

// testClock は、テストが進める時計。
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fixture struct {
	targets *apptest.SyncTargets
	tokens  *apptest.SyncTokens
	rep     *apptest.Replicator
	state   *apptest.SyncState
	events  *apptest.Events
	clock   *testClock
	svc     *syncing.Service

	mu    sync.Mutex
	after []port.StoreID
}

// S1 team: remote・ミラー・アカウントあり。S2 solo: remote なし。S3 docs: アカウントなし。
// S4 ro: 閲覧のみ(ミラーあり)。S5 hidden: 非表示。
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: "S1", Name: "team", Remote: "https://github.com/o/team.git", Mirrors: []string{"https://github.com/o/team-mirror"}, Account: "A1"},
			{ID: "S2", Name: "solo"},
			{ID: "S3", Name: "docs", Remote: "git@github.com:o/docs.git"},
			{ID: "S4", Name: "ro", Remote: "https://github.com/o/ro", Mirrors: []string{"https://m.invalid/ro"}, Account: "A1", ReadOnly: true},
			{ID: "S5", Name: "hidden", Remote: "https://github.com/o/hidden", Account: "A1", Hidden: true},
		}},
		tokens: &apptest.SyncTokens{Tokens: map[port.StoreID]string{"S1": "tok-taro", "S4": "tok-taro", "S5": "tok-taro"}},
		rep: &apptest.Replicator{
			Results: map[port.StoreID]port.SyncResult{"S1": {Sent: 2, Received: 1, Seen: "v2"}, "S4": {Received: 3, Seen: "r1"}},
			Errs:    map[port.StoreID]error{},
			UnsentN: map[port.StoreID]int{"S1": 4},
		},
		state:  apptest.NewSyncState(),
		events: &apptest.Events{},
		clock:  &testClock{now: at},
	}
	f.svc = &syncing.Service{
		Targets: f.targets, Tokens: f.tokens, Replicator: f.rep, State: f.state, Events: f.events, Clock: f.clock,
		AfterSuccess: func(_ context.Context, id port.StoreID) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.after = append(f.after, id)
		},
	}
	return f
}

// syncs は、Store ごとの Sync(ミラーを除く)の回数。
func (f *fixture) syncs() map[port.StoreID]int {
	n := map[port.StoreID]int{}
	for _, c := range f.rep.Calls() {
		if !c.Mirror {
			n[c.Store]++
		}
	}
	return n
}

func (f *fixture) afterCalls() []port.StoreID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]port.StoreID(nil), f.after...)
}
```

`internal/app/syncing/service_test.go`:

```go
package syncing_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func TestSyncStoreSucceeds(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S1")
	if err != nil || !out.OK() || out.Sent != 2 || out.Received != 1 {
		t.Fatalf("SyncStore = %+v, %v", out, err)
	}
	if got := out.Message(); got != "team を同期しました(送信 2 件・受信 1 件)" {
		t.Errorf("Message = %q", got)
	}
	calls := f.rep.Calls()
	want := port.Remote{URL: "https://github.com/o/team.git", Token: "tok-taro"}
	if len(calls) != 2 || calls[0].Remote != want || !calls[1].Mirror || calls[1].Remote.URL != "https://github.com/o/team-mirror" || calls[1].Remote.Token != "tok-taro" {
		t.Errorf("calls = %+v", calls)
	}
	rs, _ := f.state.Records(context.Background())
	if len(rs) != 1 || !rs[0].LastSuccess.Equal(at) || rs[0].Reason != "" {
		t.Errorf("records = %+v", rs)
	}
	if seen, _ := f.state.Seen(context.Background(), "S1", "github.com/o/team"); seen != "v2" {
		t.Errorf("Seen = %q", seen)
	}
	if got := f.afterCalls(); !slices.Equal(got, []port.StoreID{"S1"}) {
		t.Errorf("AfterSuccess = %v", got)
	}
	if evs := f.events.List(); len(evs) != 1 || evs[0] != (port.Event{Kind: port.EventSync, Store: "S1"}) {
		t.Errorf("events = %+v", evs)
	}
}

// 決定 C2-11: 前に受け取った位置を、同じ remote(書き方の違いは問わない)の次の同期に渡す。
func TestSyncStorePassesSeenForTheSameRemote(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	ts, _ := f.targets.List(ctx)
	ts[0].Remote = "git@github.com:O/team"
	f.targets.Set(ts)
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	ts[0].Remote = "https://github.com/o/other.git"
	f.targets.Set(ts)
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, c := range f.rep.Calls() {
		if !c.Mirror {
			seen = append(seen, c.Remote.Seen)
		}
	}
	if !slices.Equal(seen, []string{"", "v2", ""}) {
		t.Errorf("Seen passed = %q", seen)
	}
}

// 06「同期のエラー」の表: remote の失敗ごとの理由と、記録と、手動のトースト。
func TestSyncStoreRecordsTheReason(t *testing.T) {
	cases := []struct {
		err  error
		want syncstatus.Reason
	}{
		{port.ErrAuthFailed, syncstatus.AuthFailed},
		{port.ErrUnreachable, syncstatus.Unreachable},
		{port.ErrWriteDenied, syncstatus.WriteDenied},
		{port.ErrRemoteUnreadable, syncstatus.Unreadable},
		{port.ErrRemoteNotStore, syncstatus.NotStore},
		{port.ErrRemoteNewerFormat, syncstatus.NewerFormat},
		{port.ErrNoCommonHistory, syncstatus.Unrelated},
		{port.ErrRemoteRewritten, syncstatus.Rewritten},
		{port.ErrSendConflict, syncstatus.SendConflict},
		{errors.New("disk full"), syncstatus.Unreachable},
	}
	for _, c := range cases {
		t.Run(string(c.want)+c.err.Error(), func(t *testing.T) {
			f := newFixture(t)
			f.rep.Errs["S1"] = c.err
			out, err := f.svc.SyncStore(context.Background(), "S1")
			if err != nil || out.OK() || out.Reason != c.want {
				t.Fatalf("SyncStore = %+v, %v", out, err)
			}
			if got, want := out.Message(), "team の同期に失敗しました: "+c.want.Short(); got != want {
				t.Errorf("Message = %q, want %q", got, want)
			}
			rs, _ := f.state.Records(context.Background())
			if len(rs) != 1 || rs[0].Reason != c.want || !rs[0].LastAttempt.Equal(at) || !rs[0].LastSuccess.IsZero() {
				t.Errorf("records = %+v", rs)
			}
			if len(f.afterCalls()) != 0 || f.syncs()["S1"] != 1 || len(f.rep.Calls()) != 1 {
				t.Errorf("after = %v, calls = %+v", f.afterCalls(), f.rep.Calls())
			}
			rejected := f.tokens.Rejected()
			if (c.want == syncstatus.AuthFailed) != (len(rejected) == 1) {
				t.Errorf("rejected = %v", rejected)
			}
		})
	}
}

// 決定 C2-9: トークンを読めなければ「認証に失敗しました」。GitHub に届かなければ「接続できません」。
func TestSyncStoreTokenFailures(t *testing.T) {
	cases := []struct {
		kind ghcall.Kind
		want syncstatus.Reason
	}{
		{ghcall.KeychainMissing, syncstatus.AuthFailed},
		{ghcall.EnvMissing, syncstatus.AuthFailed},
		{ghcall.GhTokenMissing, syncstatus.AuthFailed},
		{ghcall.GhTimeout, syncstatus.AuthFailed},
		{ghcall.AuthError, syncstatus.AuthFailed},
		{ghcall.Unreachable, syncstatus.Unreachable},
		{ghcall.RateLimited, syncstatus.Unreachable},
	}
	for _, c := range cases {
		f := newFixture(t)
		f.tokens.Errs = map[port.StoreID]error{"S1": &ghcall.Failure{Kind: c.kind}}
		out, err := f.svc.SyncStore(context.Background(), "S1")
		if err != nil || out.Reason != c.want || len(f.rep.Calls()) != 0 {
			t.Errorf("%s: %+v, %v", c.kind, out, err)
		}
		if rs, _ := f.state.Records(context.Background()); len(rs) != 1 || rs[0].Reason != c.want {
			t.Errorf("%s: records = %+v", c.kind, rs)
		}
	}
}

// 決定 C2-16: 割り当てがない(トークンの入口が NoAccount を返した場合も)なら、同期せず、記録しない。
func TestSyncStoreWithoutAccount(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S3")
	if err != nil || out.Reason != syncstatus.NoAccount || out.Message() != "docs にアカウントが設定されていないため同期できません" {
		t.Errorf("S3 = %+v, %v", out, err)
	}
	f.tokens.Errs = map[port.StoreID]error{"S1": &ghcall.Failure{Kind: ghcall.NoAccount}}
	if out, _ := f.svc.SyncStore(context.Background(), "S1"); out.Reason != syncstatus.NoAccount {
		t.Errorf("S1 = %+v", out)
	}
	if rs, _ := f.state.Records(context.Background()); len(rs) != 0 || len(f.rep.Calls()) != 0 {
		t.Errorf("records = %+v, calls = %+v", rs, f.rep.Calls())
	}
}

func TestSyncStoreWithoutRemote(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S2")
	if err != nil || !out.Skipped || out.OK() || out.Message() != "solo には remote がないため、同期するものはありません" {
		t.Errorf("S2 = %+v, %v", out, err)
	}
	if rs, _ := f.state.Records(context.Background()); len(rs) != 0 || len(f.events.List()) != 0 {
		t.Errorf("records = %+v, events = %+v", rs, f.events.List())
	}
}

// 決定 C2-13: 閲覧のみは受け取るだけ。ミラーに送らず、完全な削除の呼び出し口も呼ばない。
func TestSyncStoreReadOnly(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S4")
	if err != nil || !out.OK() || out.Received != 3 {
		t.Fatalf("S4 = %+v, %v", out, err)
	}
	calls := f.rep.Calls()
	if len(calls) != 1 || !calls[0].Remote.ReceiveOnly {
		t.Errorf("calls = %+v", calls)
	}
	if len(f.afterCalls()) != 0 {
		t.Errorf("AfterSuccess = %v", f.afterCalls())
	}
}

// 06 のミラーの段落: ミラーに送れなくても、同期は成功のまま。ミラーごとに記録する。
func TestSyncStoreMirrorFailure(t *testing.T) {
	f := newFixture(t)
	f.rep.MirrorErrs = map[string]error{"https://github.com/o/team-mirror": port.ErrMirrorAhead}
	out, err := f.svc.SyncStore(context.Background(), "S1")
	if err != nil || !out.OK() || out.MirrorFailures != 1 {
		t.Fatalf("S1 = %+v, %v", out, err)
	}
	ms, _ := f.state.Mirrors(context.Background(), "S1")
	if len(ms) != 1 || ms[0].Reason != syncstatus.MirrorAhead {
		t.Errorf("mirrors = %+v", ms)
	}
}

// 06「手動で同期する」: 同じ Store の同期が動いていれば、新しく始めずに、その結果を使う。
func TestSyncStoreJoinsTheRunningSync(t *testing.T) {
	f := newFixture(t)
	f.rep.Gate = make(chan struct{})
	outs := make(chan port.StoreID, 2)
	for range 2 {
		go func() {
			out, err := f.svc.SyncStore(context.Background(), "S1")
			if err == nil && out.OK() {
				outs <- out.Store
			}
		}()
	}
	waitFor(t, func() bool { return f.syncs()["S1"] == 1 })
	time.Sleep(50 * time.Millisecond) // 2 つ目が始まっていないことを確かめる時間
	f.rep.Gate <- struct{}{}
	for range 2 {
		select {
		case <-outs:
		case <-time.After(2 * time.Second):
			t.Fatal("a caller did not get the result")
		}
	}
	if n := f.syncs()["S1"]; n != 1 {
		t.Errorf("Sync ran %d times", n)
	}
}

// 決定 C2-17: 始めた要求が切れても、同期は最後まで続ける。
func TestSyncStoreOutlivesTheRequest(t *testing.T) {
	f := newFixture(t)
	f.rep.Gate = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() {
		out, _ := f.svc.SyncStore(ctx, "S1")
		done <- out.OK()
	}()
	waitFor(t, func() bool { return f.syncs()["S1"] == 1 })
	cancel()
	f.rep.Gate <- struct{}{}
	if ok := <-done; !ok {
		t.Error("the sync was cancelled with the request")
	}
}

func TestSyncStoreUnknownStore(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.SyncStore(context.Background(), "S9"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("err = %v", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
```

`internal/app/syncing/all_test.go`:

```go
package syncing_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 06「同期の結果」: 表示中で remote がある Store をすべて同期し、1 つのトーストにまとめる。
func TestSyncAllMixed(t *testing.T) {
	f := newFixture(t)
	all, err := f.svc.SyncAll(context.Background())
	if err != nil || all.Nothing {
		t.Fatalf("SyncAll = %+v, %v", all, err)
	}
	want := []string{"成功 2 件・失敗 1 件", "docs: アカウントが設定されていません"}
	if !slices.Equal(all.Lines, want) {
		t.Errorf("Lines = %q", all.Lines)
	}
	n := f.syncs()
	if n["S1"] != 1 || n["S4"] != 1 || n["S5"] != 0 || len(all.Outcomes) != 3 {
		t.Errorf("syncs = %v, outcomes = %+v", n, all.Outcomes)
	}
}

func TestSyncAllSucceeds(t *testing.T) {
	f := newFixture(t)
	f.targets.Set([]port.SyncTarget{{ID: "S1", Name: "team", Remote: "https://github.com/o/team", Account: "A1"}})
	all, err := f.svc.SyncAll(context.Background())
	if err != nil || !slices.Equal(all.Lines, []string{"1 件の Store を同期しました"}) {
		t.Errorf("SyncAll = %+v, %v", all, err)
	}
}

// 06「手動で同期する」: 同期する Store がなければ、何も同期せず、そのトーストを出す。
func TestSyncAllWithNothingToSync(t *testing.T) {
	f := newFixture(t)
	f.targets.Set([]port.SyncTarget{{ID: "S2", Name: "solo"}, {ID: "S5", Name: "hidden", Remote: "https://github.com/o/h", Account: "A1", Hidden: true}})
	all, err := f.svc.SyncAll(context.Background())
	if err != nil || !all.Nothing || !slices.Equal(all.Lines, []string{syncstatus.NothingToSync}) || len(f.rep.Calls()) != 0 {
		t.Errorf("SyncAll = %+v, %v", all, err)
	}
}
```

`internal/app/syncing/status_test.go`:

```go
package syncing_test

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 06「同期の状態の表示」: remote がある Store の状態(非表示も含め、画面が分ける)。
func TestStatus(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Minute)
	f.rep.Errs["S1"] = port.ErrAuthFailed
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Status(ctx)
	if err != nil || len(st) != 4 {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	if s := st[0]; s.Store != "S1" || s.Name != "team" || s.Reason != syncstatus.AuthFailed || s.Unsent != 4 || !s.LastSuccess.Equal(at) {
		t.Errorf("S1 = %+v", s)
	}
	if s := st[1]; s.Store != "S3" || s.Reason != syncstatus.NoAccount || !s.LastSuccess.IsZero() {
		t.Errorf("S3 = %+v", s)
	}
	if s := st[2]; s.Store != "S4" || !s.ReadOnly || s.Reason != "" {
		t.Errorf("S4 = %+v", s)
	}
	if s := st[3]; s.Store != "S5" || !s.Hidden {
		t.Errorf("S5 = %+v", s)
	}
}
```

`internal/app/syncing/remotekey_test.go`(内部のテスト):

```go
package syncing

import "testing"

func TestRemoteKey(t *testing.T) {
	cases := map[string]string{
		"https://github.com/O/Repo.git":                   "github.com/o/repo",
		"https://github.com/o/repo/":                      "github.com/o/repo",
		"https://x-access-token:abc@github.com/o/repo":    "github.com/o/repo",
		"git@github.com:o/repo.git":                       "github.com/o/repo",
		"ssh://git@github.com/o/repo":                     "github.com/o/repo",
		" https://github.com/o/repo ":                     "github.com/o/repo",
		"https://ghe.example.com/team/o/repo.git":         "ghe.example.com/team/o/repo",
	}
	for in, want := range cases {
		if got := remoteKey(in); got != want {
			t.Errorf("remoteKey(%q) = %q, want %q", in, got, want)
		}
	}
	if remoteKey("https://github.com/o/a") == remoteKey("https://github.com/o/b") {
		t.Error("different repositories have the same key")
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/app/syncing/`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: 実装する**

`internal/app/syncing/service.go`:

```go
// Package syncing は、Store の同期の usecase(06「同期とは」〜「同期の状態の表示」)。
// パッケージ名は標準ライブラリの sync と重ならないようにした(決定 C2-23)。
package syncing

import (
	"context"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// Service は Store の同期。サーバーに 1 つだけ作り、API と自動の同期(Auto)で共用する。
type Service struct {
	Targets    port.SyncTargets
	Tokens     port.SyncTokens
	Replicator port.Replicator
	State      port.SyncState
	Events     port.Events
	Clock      port.Clock
	// AfterSuccess は、書き込める Store の同期に成功した直後に呼ぶ(06「同期のあとの完全な削除」。中身は
	// トラック G が差し込む。決定 C2-22)。nil なら呼ばない。
	AfterSuccess func(ctx context.Context, store port.StoreID)

	mu      sync.Mutex
	running map[port.StoreID]*call
}

// call は、動いている 1 つの同期。終わると done を閉じる。
type call struct {
	done chan struct{}
	out  Outcome
	err  error
}

// Outcome は、1 つの Store の同期の結果。
type Outcome struct {
	Store          port.StoreID
	Name           string
	Skipped        bool              // remote がないので何もしなかった
	Reason         syncstatus.Reason // 失敗なら理由
	Sent           int
	Received       int
	MirrorFailures int // 送れなかったミラーの数
}

// OK は、同期に成功したか。
func (o Outcome) OK() bool { return !o.Skipped && o.Reason == "" }

// Message は、手動で 1 つの Store を同期したときのトースト(06「同期の結果」「同期のエラー」)。
func (o Outcome) Message() string {
	switch {
	case o.Skipped:
		return syncstatus.NoRemote(o.Name)
	case o.Reason != "":
		return syncstatus.Failed(o.Name, o.Reason)
	}
	return syncstatus.Synced(o.Name, o.Sent, o.Received)
}

// SyncStore は、1 つの Store を同期する(Store メニューと設定の「今すぐ同期」、「Store ごとの同期状態」)。
// 非表示の Store も同期する。Store がなければ port.ErrStoreNotFound。失敗した同期はエラーでなく Outcome で返す。
func (s *Service) SyncStore(ctx context.Context, id port.StoreID) (Outcome, error) {
	targets, err := s.Targets.List(ctx)
	if err != nil {
		return Outcome{}, err
	}
	for _, t := range targets {
		if t.ID == id {
			return s.once(ctx, t)
		}
	}
	return Outcome{}, port.ErrStoreNotFound
}

// once は、t を同期する。同じ Store の同期が動いていれば、新しく始めずに、その結果を待って返す。
// 同期は、始めた要求が切れても最後まで続ける(決定 C2-17)。
func (s *Service) once(ctx context.Context, t port.SyncTarget) (Outcome, error) {
	s.mu.Lock()
	if c, ok := s.running[t.ID]; ok {
		s.mu.Unlock()
		<-c.done
		return c.out, c.err
	}
	if s.running == nil {
		s.running = map[port.StoreID]*call{}
	}
	c := &call{done: make(chan struct{})}
	s.running[t.ID] = c
	s.mu.Unlock()

	c.out, c.err = s.run(context.WithoutCancel(ctx), t)

	s.mu.Lock()
	delete(s.running, t.ID)
	s.mu.Unlock()
	close(c.done)
	return c.out, c.err
}

func (s *Service) publish(id port.StoreID) {
	if s.Events != nil {
		s.Events.Publish(port.Event{Kind: port.EventSync, Store: id})
	}
}
```

`internal/app/syncing/run.go`:

```go
package syncing

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// run は、1 つの Store の同期の手順(保存形式「同期」)。トークンを得て、前に受け取った位置を読み、
// 受け取り・合わせ・送り、記録し、ミラーへ送る。エラーは、このマシンの状態を読み書きできないときだけ返す。
func (s *Service) run(ctx context.Context, t port.SyncTarget) (Outcome, error) {
	out := Outcome{Store: t.ID, Name: t.Name}
	switch {
	case t.Remote == "":
		out.Skipped = true
		return out, nil
	case t.Account == "":
		out.Reason = syncstatus.NoAccount // 記録しない。状態は Status が導く(決定 C2-16)
		return out, nil
	}
	at := s.Clock.Now()
	tok, err := s.Tokens.StoreToken(ctx, t.ID)
	if err != nil {
		r, ok := tokenReason(err)
		switch {
		case !ok:
			return out, err
		case r == syncstatus.NoAccount:
			out.Reason = r
			return out, nil
		}
		return s.failed(ctx, out, at, r)
	}
	key := remoteKey(t.Remote)
	seen, err := s.State.Seen(ctx, t.ID, key)
	if err != nil {
		return out, err
	}
	res, err := s.Replicator.Sync(ctx, t.ID, port.Remote{URL: t.Remote, Token: tok, Seen: seen, ReceiveOnly: t.ReadOnly})
	if err != nil {
		r := remoteReason(err)
		if r == syncstatus.AuthFailed {
			s.Tokens.StoreRejected(ctx, t.ID)
		}
		return s.failed(ctx, out, at, r)
	}
	if err := s.State.Succeeded(ctx, t.ID, at, key, res.Seen); err != nil {
		return out, err
	}
	out.Sent, out.Received = res.Sent, res.Received
	if !t.ReadOnly {
		if out.MirrorFailures, err = s.mirror(ctx, t, tok); err != nil {
			return out, err
		}
		if s.AfterSuccess != nil {
			s.AfterSuccess(ctx, t.ID)
		}
	}
	s.publish(t.ID)
	return out, nil
}

func (s *Service) failed(ctx context.Context, out Outcome, at time.Time, r syncstatus.Reason) (Outcome, error) {
	out.Reason = r
	if err := s.State.Failed(ctx, out.Store, at, r); err != nil {
		return out, err
	}
	s.publish(out.Store)
	return out, nil
}

// mirror は、remote への同期に成功したあと、ミラーへ順に送り、送れなかった数を返す(06 のミラーの段落)。
// ミラーに送れなくても、Store の状態は変えない。
func (s *Service) mirror(ctx context.Context, t port.SyncTarget, tok string) (int, error) {
	failures := 0
	for _, m := range t.Mirrors {
		var r syncstatus.Reason
		if err := s.Replicator.Mirror(ctx, t.ID, port.Remote{URL: m, Token: tok}); err != nil {
			r = remoteReason(err)
			failures++
			if r == syncstatus.AuthFailed {
				s.Tokens.StoreRejected(ctx, t.ID)
			}
		}
		if err := s.State.Mirrored(ctx, t.ID, m, s.Clock.Now(), r); err != nil {
			return failures, err
		}
	}
	return failures, nil
}
```

`internal/app/syncing/reason.go`:

```go
package syncing

import (
	"errors"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// tokenReason は、トークンを得られなかった理由。*ghcall.Failure でなければ false(このマシンの失敗)。
// 読めない・拒否・認証エラーは「認証に失敗しました」(決定 C2-9)、GitHub に届かないときは「接続できません」。
func tokenReason(err error) (syncstatus.Reason, bool) {
	var f *ghcall.Failure
	if !errors.As(err, &f) {
		return "", false
	}
	switch f.Kind {
	case ghcall.NoAccount:
		return syncstatus.NoAccount, true
	case ghcall.Unreachable, ghcall.RateLimited:
		return syncstatus.Unreachable, true
	}
	return syncstatus.AuthFailed, true
}

var remoteReasons = []struct {
	err    error
	reason syncstatus.Reason
}{
	{port.ErrAuthFailed, syncstatus.AuthFailed},
	{port.ErrRemoteUnreadable, syncstatus.Unreadable},
	{port.ErrWriteDenied, syncstatus.WriteDenied},
	{port.ErrRemoteNotStore, syncstatus.NotStore},
	{port.ErrRemoteNewerFormat, syncstatus.NewerFormat},
	{port.ErrNoCommonHistory, syncstatus.Unrelated},
	{port.ErrRemoteRewritten, syncstatus.Rewritten},
	{port.ErrSendConflict, syncstatus.SendConflict},
	{port.ErrMirrorUnrelated, syncstatus.MirrorUnrelated},
	{port.ErrMirrorAhead, syncstatus.MirrorAhead},
}

// remoteReason は、Replicator の失敗の理由。どれにも当たらなければ「remote に接続できません」
// (06: 途中で切れたときは、接続できないと同じ扱い)。
func remoteReason(err error) syncstatus.Reason {
	for _, r := range remoteReasons {
		if errors.Is(err, r.err) {
			return r.reason
		}
	}
	return syncstatus.Unreachable
}
```

`internal/app/syncing/remotekey.go`:

```go
package syncing

import "strings"

// remoteKey は、remote が同じかを比べる形(決定 C2-11)。方式・利用者名・末尾の .git と / を除き、
// git@host:path を host/path にし、小文字にする。SSH と HTTPS の書き方の違いでは変わらない。
func remoteKey(raw string) string {
	s := strings.TrimSpace(raw)
	if _, rest, ok := strings.Cut(s, "://"); ok {
		host, path, _ := strings.Cut(rest, "/")
		if i := strings.LastIndex(host, "@"); i >= 0 {
			host = host[i+1:]
		}
		s = host + "/" + path
	} else if i := strings.Index(s, "@"); i >= 0 && !strings.Contains(s[:i], "/") {
		s = strings.Replace(s[i+1:], ":", "/", 1)
	}
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return strings.ToLower(s)
}
```

`internal/app/syncing/all.go`:

```go
package syncing

import (
	"context"
	"errors"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// AllOutcome は、「すべて同期」の結果。Lines は 1 つのトーストの行(06「同期の結果」)。
type AllOutcome struct {
	Nothing  bool // 表示中で remote がある Store がなく、何もしなかった
	Outcomes []Outcome
	Lines    []string
}

// SyncAll は、表示中で remote がある Store をすべて同期する(サイドバーの「すべて同期」ほか)。
// remote がない Store と非表示の Store は数えない。
func (s *Service) SyncAll(ctx context.Context) (AllOutcome, error) {
	targets, err := s.Targets.List(ctx)
	if err != nil {
		return AllOutcome{}, err
	}
	var picked []port.SyncTarget
	for _, t := range targets {
		if t.Remote != "" && !t.Hidden {
			picked = append(picked, t)
		}
	}
	if len(picked) == 0 {
		return AllOutcome{Nothing: true, Lines: []string{syncstatus.NothingToSync}}, nil
	}
	outs := make([]Outcome, len(picked))
	errs := make([]error, len(picked))
	var wg sync.WaitGroup
	for i, t := range picked {
		wg.Go(func() { outs[i], errs[i] = s.once(ctx, t) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return AllOutcome{}, err
	}
	ok := 0
	var failures []syncstatus.Failure
	for _, o := range outs {
		if o.OK() {
			ok++
			continue
		}
		failures = append(failures, syncstatus.Failure{Store: o.Name, Reason: o.Reason})
	}
	return AllOutcome{Outcomes: outs, Lines: syncstatus.All(ok, failures)}, nil
}
```

`internal/app/syncing/status.go`:

```go
package syncing

import (
	"context"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// StoreStatus は、remote がある 1 つの Store の同期の状態(06「同期の状態の表示」)。
type StoreStatus struct {
	Store       port.StoreID
	Name        string
	Hidden      bool
	ReadOnly    bool
	Reason      syncstatus.Reason // 同期エラーなら理由
	Unsent      int
	LastSuccess time.Time // 最後に成功した同期。なければゼロ
}

// Status は、remote がある Store の同期の状態を、サイドバーの並びで返す。非表示の Store も含める(画面が
// カードと点から除き、設定の「Store」では出す)。remote があって割り当てがない Store は、試していなくても
// 「アカウントが設定されていません」の同期エラー(決定 C2-16)。
func (s *Service) Status(ctx context.Context) ([]StoreStatus, error) {
	targets, err := s.Targets.List(ctx)
	if err != nil {
		return nil, err
	}
	records, err := s.State.Records(ctx)
	if err != nil {
		return nil, err
	}
	byStore := map[port.StoreID]port.SyncRecord{}
	for _, r := range records {
		byStore[r.Store] = r
	}
	out := []StoreStatus{}
	for _, t := range targets {
		if t.Remote == "" {
			continue
		}
		rec := byStore[t.ID]
		st := StoreStatus{Store: t.ID, Name: t.Name, Hidden: t.Hidden, ReadOnly: t.ReadOnly, Reason: rec.Reason, LastSuccess: rec.LastSuccess}
		if t.Account == "" {
			st.Reason = syncstatus.NoAccount
		}
		if st.Unsent, err = s.Replicator.Unsent(ctx, t.ID); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/app/syncing/ && golangci-lint run ./internal/app/syncing/ && go test ./internal/archtest/`
Expected: PASS、指摘なし

- [ ] **Step 5: コミットする**

```bash
git add internal/app/syncing
git commit -m "feat(app): Store の同期・すべて同期・同期の状態の usecase を足す(C2 Task 9)"
```

---
### Task 10: 自動の同期(app/syncing の Auto と NotifyWrites)

**Files:**
- Create: `internal/app/syncing/auto.go`・`due.go`・`notify.go`
- Test: `internal/app/syncing/auto_test.go`・`due_test.go`・`notify_test.go`

**Interfaces:**
- Consumes: `Service`・`Service.once`(Task 9)、`port.SyncSettings`(Task 2)
- Produces:

```go
func NewAuto(svc *Service, settings port.SyncSettings) *Auto
func (a *Auto) Online()
func (a *Auto) Wrote(id port.StoreID)
func (a *Auto) Hold(tab string, hold bool)
func (a *Auto) Step(ctx context.Context) error
func (a *Auto) Run(ctx context.Context)
func NotifyWrites(s port.Stores, onWrite func(port.StoreID)) port.Stores
```

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/syncing/due_test.go`(内部のテスト):

```go
package syncing

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 06「自動で同期する」の契機のうち、間隔と書き込み後。
func TestDue(t *testing.T) {
	t0 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	timing := port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}
	ok := port.SyncRecord{LastAttempt: t0, LastSuccess: t0}
	failed := port.SyncRecord{LastAttempt: t0, Reason: syncstatus.AuthFailed}
	cases := []struct {
		name  string
		now   time.Time
		rec   port.SyncRecord
		wrote time.Time
		want  bool
	}{
		{"never tried", t0, port.SyncRecord{}, time.Time{}, true},
		{"interval not yet", t0.Add(4 * time.Minute), ok, time.Time{}, false},
		{"interval passed", t0.Add(5 * time.Minute), ok, time.Time{}, true},
		{"failed attempts count too", t0.Add(5 * time.Minute), failed, time.Time{}, true},
		{"write, waiting", t0.Add(2*time.Minute + 29*time.Second), ok, t0.Add(2 * time.Minute), false},
		{"write, quiet for 30s", t0.Add(2*time.Minute + 30*time.Second), ok, t0.Add(2 * time.Minute), true},
		// 06 の例: 10:00 に認証エラー、10:02 に保存しても、10:02:30 では同期せず 10:05 まで待つ。
		{"error store waits for the interval", t0.Add(2*time.Minute + 30*time.Second), failed, t0.Add(2 * time.Minute), false},
		{"error store at the interval", t0.Add(5 * time.Minute), failed, t0.Add(2 * time.Minute), true},
	}
	for _, c := range cases {
		if got := due(c.now, c.rec, c.wrote, timing); got != c.want {
			t.Errorf("%s: due = %v", c.name, got)
		}
	}
}
```

`internal/app/syncing/auto_test.go`:

```go
package syncing_test

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

func newAuto(t *testing.T) (*fixture, *syncing.Auto, *apptest.SyncSettings) {
	t.Helper()
	f := newFixture(t)
	settings := &apptest.SyncSettings{Value: port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}}
	return f, syncing.NewAuto(f.svc, settings), settings
}

func step(t *testing.T, a *syncing.Auto) {
	t.Helper()
	if err := a.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// 06「自動で同期する」: 開いた直後に、対象(remote・表示中・アカウントあり)のすべての Store。
// 閲覧のみも対象。間隔が過ぎるまでは、もう一度は同期しない。
func TestAutoSyncsTargetsWhenOpened(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a)
	n := f.syncs()
	if n["S1"] != 1 || n["S4"] != 1 || n["S2"] != 0 || n["S3"] != 0 || n["S5"] != 0 {
		t.Fatalf("syncs = %v", n)
	}
	f.clock.Advance(4 * time.Minute)
	step(t, a)
	if n := f.syncs(); n["S1"] != 1 {
		t.Errorf("synced before the interval: %v", n)
	}
	f.clock.Advance(time.Minute)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 || n["S4"] != 2 {
		t.Errorf("not synced at the interval: %v", n)
	}
}

// 「書き込み後」: 最後の保存から 30 秒、次の保存がなければ、その Store。
func TestAutoSyncsAfterWritesSettle(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a)
	f.clock.Advance(time.Minute)
	a.Wrote("S1")
	f.clock.Advance(20 * time.Second)
	a.Wrote("S1") // 続けて保存すると、最後の保存から数え直す
	f.clock.Advance(20 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 1 {
		t.Fatalf("synced while writes continued: %d", n)
	}
	f.clock.Advance(10 * time.Second)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 || n["S4"] != 1 {
		t.Fatalf("after quiet: %v", n)
	}
	f.clock.Advance(40 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 2 {
		t.Errorf("synced again without a write: %d", n)
	}
}

// 同期エラーの Store は、書き込み後でも、最後に試みてから間隔が過ぎるまで待つ。
func TestAutoErrorStoreWaitsForTheInterval(t *testing.T) {
	f, a, _ := newAuto(t)
	f.rep.Errs["S1"] = port.ErrAuthFailed
	step(t, a)
	f.clock.Advance(2 * time.Minute)
	a.Wrote("S1")
	f.clock.Advance(31 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 1 {
		t.Fatalf("error store synced after a write: %d", n)
	}
	f.clock.Advance(3 * time.Minute)
	step(t, a)
	if n := f.syncs()["S1"]; n != 2 {
		t.Errorf("not synced at the interval: %d", n)
	}
}

// 見えているタブが待つ条件にあるあいだは始めず、解けたら続けて同期する。知らせは 30 秒で切れる。
func TestAutoWaitsWhileATabHolds(t *testing.T) {
	f, a, _ := newAuto(t)
	a.Hold("tab-1", true)
	step(t, a)
	if len(f.rep.Calls()) != 0 {
		t.Fatal("synced while held")
	}
	a.Hold("tab-1", false)
	step(t, a)
	if n := f.syncs()["S1"]; n != 1 {
		t.Fatalf("not synced after release: %d", n)
	}
	a.Online()
	a.Hold("tab-2", true)
	f.clock.Advance(31 * time.Second)
	step(t, a)
	if n := f.syncs()["S1"]; n != 2 {
		t.Errorf("an expired hold still blocks: %d", n)
	}
}

// オンライン復帰: 間隔を待たずに、対象のすべての Store。
func TestAutoSyncsWhenBackOnline(t *testing.T) {
	f, a, _ := newAuto(t)
	step(t, a)
	f.clock.Advance(time.Minute)
	a.Online()
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 || n["S4"] != 2 {
		t.Errorf("syncs = %v", n)
	}
}

// 「裏で自動的に同期する」がオフなら、どの契機でも同期しない。
func TestAutoOffStopsEverything(t *testing.T) {
	f, a, settings := newAuto(t)
	settings.Value.Auto = false
	a.Wrote("S1")
	a.Online()
	f.clock.Advance(time.Hour)
	step(t, a)
	if len(f.rep.Calls()) != 0 {
		t.Errorf("synced while off: %+v", f.rep.Calls())
	}
}
```

(`apptest.SyncSettings` の `Value` を直接書き換えるのは、Step と同時に動かさないテストだけで行う。)

`internal/app/syncing/notify_test.go`:

```go
package syncing_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// 決定 C2-19: Write が成功したときだけ知らせる。
func TestNotifyWrites(t *testing.T) {
	inner := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)), apptest.NewIDs())
	inner.AddStore("S1")
	var got []port.StoreID
	s := syncing.NotifyWrites(inner, func(id port.StoreID) { got = append(got, id) })
	op := port.Operation{Actor: change.User("taro"), Summary: "x"}
	if err := s.Write(context.Background(), "S1", op, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := s.Write(context.Background(), "S1", op, func(port.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if !slices.Equal(got, []port.StoreID{"S1"}) {
		t.Errorf("notified = %v", got)
	}
	if err := s.Read(context.Background(), "S1", func(port.Reader) error { return nil }); err != nil {
		t.Errorf("Read = %v", err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/app/syncing/ -run 'Due|Auto|Notify'`
Expected: FAIL(`due`・`NewAuto`・`NotifyWrites` がない)

- [ ] **Step 3: 実装する**

`internal/app/syncing/due.go`:

```go
package syncing

import (
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// eligible は、自動で同期する対象か(06「自動で同期する」: remote があり、非表示でなく、アカウントがある)。
func eligible(t port.SyncTarget) bool { return t.Remote != "" && !t.Hidden && t.Account != "" }

// due は、間隔か書き込み後の契機で、いま同期するか(06「自動で同期する」)。rec は記録、wrote は最後の
// 保存の時刻(なければゼロ)。同期エラーの Store は、書き込み後でも、最後に試みてから間隔が過ぎるまで待つ。
func due(now time.Time, rec port.SyncRecord, wrote time.Time, t port.SyncTiming) bool {
	if rec.LastAttempt.IsZero() || now.Sub(rec.LastAttempt) >= t.Interval {
		return true
	}
	if wrote.IsZero() || now.Sub(wrote) < t.AfterWrite {
		return false
	}
	return rec.Reason == ""
}
```

`internal/app/syncing/auto.go`:

```go
package syncing

import (
	"context"
	"sync"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// holdFor は、タブの「待つ条件にある」知らせが有効な時間(決定 C2-18)。タブは続くあいだ 10 秒ごとに知らせ直す。
const holdFor = 30 * time.Second

// Auto は、自動の同期(06「自動で同期する」)。契機は、間隔、アプリを開いたとき(作ったとき)、オンライン復帰
// (Online)、書き込み後(Wrote)。見えているタブが待つ条件にあるあいだ(Hold)は始めない。結果は捨てる
// (自動の同期はトーストもアクティビティも出さない。決定 C2-24)。
type Auto struct {
	Service  *Service
	Settings port.SyncSettings

	mu    sync.Mutex
	all   bool                       // 「アプリを開いたとき」「オンライン復帰」が待っている
	wrote map[port.StoreID]time.Time // Store ごとの最後の保存の時刻
	holds map[string]time.Time       // 待つ条件にあるタブと、知らせの期限
}

// NewAuto は、「アプリを開いたとき」の契機を持った Auto を作る。
func NewAuto(svc *Service, settings port.SyncSettings) *Auto {
	return &Auto{Service: svc, Settings: settings, all: true, wrote: map[port.StoreID]time.Time{}, holds: map[string]time.Time{}}
}

// Online は、オフラインからつながったことを知らせる。
func (a *Auto) Online() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.all = true
}

// Wrote は、Store に保存したことを知らせる。
func (a *Auto) Wrote(id port.StoreID) {
	now := a.Service.Clock.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.wrote[id] = now
}

// Hold は、見えているタブ tab が、待つ条件にあるか(hold)を知らせる。
func (a *Auto) Hold(tab string, hold bool) {
	now := a.Service.Clock.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if hold {
		a.holds[tab] = now.Add(holdFor)
		return
	}
	delete(a.holds, tab)
}

// Run は、ctx が終わるまで、1 秒ごとに Step を呼ぶ。Step の失敗は、次の Step でやり直す。
func (a *Auto) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		_ = a.Step(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Step は、いま同期する Store を選び、1 つずつ同期する。
func (a *Auto) Step(ctx context.Context) error {
	timing, err := a.Settings.Timing(ctx)
	if err != nil {
		return err
	}
	now := a.Service.Clock.Now()
	all, wrote, held := a.take(now, timing.Auto)
	if held || !timing.Auto {
		return nil
	}
	picked, err := a.pick(ctx, now, all, wrote, timing)
	if err != nil {
		return err
	}
	for _, t := range picked {
		if _, err := a.Service.once(ctx, t); err != nil {
			return err
		}
		a.synced(t.ID, now)
	}
	return nil
}

// take は、待っている契機を読む。待つ条件にあるタブがあれば held。そうでなければ「アプリを開いた・
// オンライン復帰」の契機をここで使い切る。自動同期がオフなら、契機をすべて捨てる。
func (a *Auto) take(now time.Time, auto bool) (all bool, wrote map[port.StoreID]time.Time, held bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !auto {
		a.all = false
		clear(a.wrote)
		return false, nil, false
	}
	for tab, until := range a.holds {
		if now.Before(until) {
			return false, nil, true
		}
		delete(a.holds, tab)
	}
	all, a.all = a.all, false
	wrote = make(map[port.StoreID]time.Time, len(a.wrote))
	for id, at := range a.wrote {
		wrote[id] = at
	}
	return all, wrote, false
}

func (a *Auto) pick(ctx context.Context, now time.Time, all bool, wrote map[port.StoreID]time.Time, timing port.SyncTiming) ([]port.SyncTarget, error) {
	targets, err := a.Service.Targets.List(ctx)
	if err != nil {
		return nil, err
	}
	records, err := a.Service.State.Records(ctx)
	if err != nil {
		return nil, err
	}
	byStore := map[port.StoreID]port.SyncRecord{}
	for _, r := range records {
		byStore[r.Store] = r
	}
	var picked []port.SyncTarget
	for _, t := range targets {
		if eligible(t) && (all || due(now, byStore[t.ID], wrote[t.ID], timing)) {
			picked = append(picked, t)
		}
	}
	return picked, nil
}

// synced は、始めた時刻 started より前の保存を、同期で送ったものとして消す(同期のあいだの保存は残す)。
func (a *Auto) synced(id port.StoreID, started time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if at, ok := a.wrote[id]; ok && !at.After(started) {
		delete(a.wrote, id)
	}
}
```

`internal/app/syncing/notify.go`:

```go
package syncing

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// NotifyWrites は、Write が成功するたびに onWrite を呼ぶ port.Stores を返す(「書き込み後」の契機。
// 決定 C2-19)。同期の修復の保存(C3)は、包む前の Stores で書く(修復で次の同期を呼ばないため)。
func NotifyWrites(s port.Stores, onWrite func(port.StoreID)) port.Stores {
	return notifying{Stores: s, onWrite: onWrite}
}

type notifying struct {
	port.Stores
	onWrite func(port.StoreID)
}

func (n notifying) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if err := n.Stores.Write(ctx, id, op, fn); err != nil {
		return err
	}
	n.onWrite(id)
	return nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/app/syncing/ && golangci-lint run ./internal/app/syncing/`
Expected: PASS、指摘なし

- [ ] **Step 5: コミットする**

```bash
git add internal/app/syncing
git commit -m "feat(app): 自動の同期の契機(間隔・開いたとき・オンライン復帰・書き込み後)とタブの待つ条件を足す(C2 Task 10)"
```

---

### Task 11: Store の変化の見張り(app/syncing の Watch と gitstore の Versions)

**Files:**
- Create: `internal/app/syncing/watch.go`・`internal/infra/gitstore/versions.go`
- Test: `internal/app/syncing/watch_test.go`・`internal/infra/gitstore/versions_test.go`

**Interfaces:**
- Consumes: `port.StoreVersions`・`port.Events`(Task 2)
- Produces: `type Watch struct { Versions port.StoreVersions; Events port.Events }`・`func (w *Watch) Step(ctx context.Context) error`・`func (w *Watch) Run(ctx context.Context, every time.Duration)`、`func (s *Stores) Versions(ctx context.Context) (map[port.StoreID]string, error)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/syncing/watch_test.go`:

```go
package syncing_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

// 06「画面への反映」: 版が変わった Store を、書き手を問わず知らせる。最初は覚えるだけ。
func TestWatchPublishesChangedStores(t *testing.T) {
	versions := &apptest.StoreVersions{Value: map[port.StoreID]string{"S1": "a", "S2": "b"}}
	events := &apptest.Events{}
	w := &syncing.Watch{Versions: versions, Events: events}
	ctx := context.Background()
	if err := w.Step(ctx); err != nil || len(events.List()) != 0 {
		t.Fatalf("first step: %v, %+v", err, events.List())
	}
	versions.Set(map[port.StoreID]string{"S1": "a2", "S3": "c"}) // S1 が変わり、S2 が消え、S3 が増えた
	if err := w.Step(ctx); err != nil {
		t.Fatal(err)
	}
	want := []port.Event{
		{Kind: port.EventStore, Store: "S1"},
		{Kind: port.EventStore, Store: "S2"},
		{Kind: port.EventStore, Store: "S3"},
	}
	if got := events.List(); !slices.Equal(got, want) {
		t.Errorf("events = %+v", got)
	}
	if err := w.Step(ctx); err != nil || len(events.List()) != 3 {
		t.Errorf("unchanged step published: %+v", events.List())
	}
}
```

`internal/infra/gitstore/versions_test.go`:

```go
package gitstore

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
)

var _ port.StoreVersions = (*Stores)(nil)

func TestVersionsChangeWithEverySave(t *testing.T) {
	m := localMachine(t)
	v1, err := m.stores.Versions(t.Context())
	if err != nil || v1[m.id] == "" {
		t.Fatalf("Versions = %v, %v", v1, err)
	}
	rename(t, m, "新しい名前")
	v2, err := m.stores.Versions(t.Context())
	if err != nil || v2[m.id] == v1[m.id] || v2[m.id] != headCommit(t, m.dir).Hash.String() {
		t.Errorf("after a save: %v -> %v, %v", v1, v2, err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/app/syncing/ -run Watch; go test ./internal/infra/gitstore/ -run Versions`
Expected: FAIL(`Watch`・`Versions` がない)

- [ ] **Step 3: 実装する**

`internal/app/syncing/watch.go`:

```go
package syncing

import (
	"context"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Watch は、Store の版の変化を見張り、変わった Store を Events に流す(06「画面への反映」、決定 C2-20)。
// 書き手(このタブ、別のタブ、同期)を問わない。
type Watch struct {
	Versions port.StoreVersions
	Events   port.Events

	last map[port.StoreID]string
}

// Step は、版を読み、前の Step から変わった Store(増えた・消えたものを含む)を、Store ID の順に流す。
// 最初の Step は覚えるだけ。
func (w *Watch) Step(ctx context.Context) error {
	now, err := w.Versions.Versions(ctx)
	if err != nil {
		return err
	}
	if w.last != nil {
		var changed []port.StoreID
		for id, v := range now {
			if w.last[id] != v {
				changed = append(changed, id)
			}
		}
		for id := range w.last {
			if _, ok := now[id]; !ok {
				changed = append(changed, id)
			}
		}
		slices.Sort(changed)
		for _, id := range changed {
			w.Events.Publish(port.Event{Kind: port.EventStore, Store: id})
		}
	}
	w.last = now
	return nil
}

// Run は、ctx が終わるまで、every ごとに Step を呼ぶ。Step の失敗は、次の Step でやり直す。
func (w *Watch) Run(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		_ = w.Step(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
```

`internal/infra/gitstore/versions.go`:

```go
package gitstore

import (
	"context"
	"maps"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Versions は、取り付けた Store ごとの先端のコミット(port.StoreVersions)。保存・同期・ほかのプログラムの
// コミットのどれでも変わる。読めない Store は含めない。
func (s *Stores) Versions(context.Context) (map[port.StoreID]string, error) {
	s.mu.Lock()
	dirs := maps.Clone(s.dirs)
	s.mu.Unlock()
	out := make(map[port.StoreID]string, len(dirs))
	for id, dir := range dirs {
		repo, err := git.PlainOpen(dir)
		if err != nil {
			continue
		}
		head, err := repo.Head()
		if err != nil {
			continue
		}
		out[id] = head.Hash().String()
	}
	return out, nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/syncing/ ./internal/infra/gitstore/ && golangci-lint run ./internal/app/syncing/ ./internal/infra/gitstore/`
Expected: PASS、指摘なし

- [ ] **Step 5: コミットする**

```bash
git add internal/app/syncing internal/infra/gitstore
git commit -m "feat: Store の版の変化を見張り、画面への反映の出来事を流す(C2 Task 11)"
```

---
### Task 12: 同期の API と Server-Sent Events(adapter/httpapi)

**Files:**
- Create: `internal/adapter/httpapi/events/events.go`・`internal/adapter/httpapi/syncing/syncing.go`・`dto.go`
- Modify: `internal/adapter/httpapi/routes.go`
- Test: `internal/adapter/httpapi/events/events_test.go`・`internal/adapter/httpapi/syncing/syncing_test.go`
- Generate: `api/openapi.yaml`・`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: `appsyncing.Service`・`Auto`(Task 9・10)、`port.Events`
- Produces:
  - `events.NewHub() *Hub`・`(*Hub).Publish(port.Event)`・`(*Hub).Listeners() int`・`events.Register(api huma.API, h *Hub)`
  - `syncing.Register(api huma.API, s Services)`・`type Services struct { Sync *appsyncing.Service; Auto *appsyncing.Auto }`
  - `httpapi.Deps` に `Sync *appsyncing.Service`・`Auto *appsyncing.Auto`・`Events *events.Hub`・`GitHubSession *ghapp.Session`

| 操作 | operationId | 本文 | 応答 |
|---|---|---|---|
| `GET /api/sync` | `list-sync-status` | なし | 200 `SyncStatusList` |
| `POST /api/sync` | `sync-all` | なし | 200 `SyncAllResult` |
| `POST /api/stores/{store}/sync` | `sync-store` | なし | 200 `SyncStoreResult`、404 |
| `PUT /api/sync/hold` | `hold-sync` | `SyncHold` | 204 |
| `POST /api/sync/online` | `sync-online` | なし | 204 |
| `GET /api/events` | `watch-events` | なし | 200 `text/event-stream`(`store`・`sync` の出来事。どちらも `{"store": "<Store ID>"}`) |

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/events/events_test.go`:

```go
package events_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	"github.com/kukv/hedgehug/internal/app/port"
)

// 06「画面への反映」: つないでいる画面に、出来事を Server-Sent Events で送る。
func TestEventsStreamsPublishedEvents(t *testing.T) {
	hub := events.NewHub()
	mux := http.NewServeMux()
	events.Register(humago.New(mux, huma.DefaultConfig("t", "0")), hub)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}
	deadline := time.Now().Add(2 * time.Second)
	for hub.Listeners() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no listener")
		}
		time.Sleep(5 * time.Millisecond)
	}
	hub.Publish(port.Event{Kind: port.EventStore, Store: "S1"})
	hub.Publish(port.Event{Kind: port.EventSync, Store: "S2"})
	var lines []string
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() && len(lines) < 4 {
		if l := sc.Text(); l != "" {
			lines = append(lines, l)
		}
	}
	want := []string{"event: store", `data: {"store":"S1"}`, "event: sync", `data: {"store":"S2"}`}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("stream =\n%s", strings.Join(lines, "\n"))
	}
}

// 画面がつないでいなくても、Publish は待たない。切れた画面は数えない。
func TestPublishWithoutListeners(t *testing.T) {
	hub := events.NewHub()
	hub.Publish(port.Event{Kind: port.EventStore, Store: "S1"})
	if hub.Listeners() != 0 {
		t.Error("listeners")
	}
}
```

`internal/adapter/httpapi/syncing/syncing_test.go`:

```go
package syncing_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/syncing"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appsyncing "github.com/kukv/hedgehug/internal/app/syncing"
)

type env struct {
	api  humatest.TestAPI
	rep  *apptest.Replicator
	auto *appsyncing.Auto
}

func newEnv(t *testing.T) env {
	t.Helper()
	rep := &apptest.Replicator{
		Results:    map[port.StoreID]port.SyncResult{"S1": {Sent: 1, Received: 2}},
		Errs:       map[port.StoreID]error{"S3": port.ErrWriteDenied},
		MirrorErrs: map[string]error{"https://github.com/o/m": port.ErrMirrorAhead},
		UnsentN:    map[port.StoreID]int{"S1": 3},
	}
	svc := &appsyncing.Service{
		Targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: "S1", Name: "team", Remote: "https://github.com/o/team", Mirrors: []string{"https://github.com/o/m"}, Account: "A1"},
			{ID: "S2", Name: "solo"},
			{ID: "S3", Name: "web", Remote: "https://github.com/o/web", Account: "A1"},
		}},
		Tokens:     &apptest.SyncTokens{Tokens: map[port.StoreID]string{"S1": "tok-taro", "S3": "tok-taro"}},
		Replicator: rep,
		State:      apptest.NewSyncState(),
		Events:     &apptest.Events{},
		Clock:      apptest.NewClock(time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)),
	}
	auto := appsyncing.NewAuto(svc, &apptest.SyncSettings{Value: port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}})
	_, api := humatest.New(t)
	syncing.Register(api, syncing.Services{Sync: svc, Auto: auto})
	return env{api: api, rep: rep, auto: auto}
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return v
}

func TestSyncStoreResults(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		store, result, message, mirror string
	}{
		{"S1", "synced", "team を同期しました(送信 1 件・受信 2 件)", "ミラー 1 件に送れませんでした"},
		{"S2", "skipped", "solo には remote がないため、同期するものはありません", ""},
		{"S3", "failed", "web の同期に失敗しました: remote への書き込みが許可されていません", ""},
	}
	for _, c := range cases {
		res := e.api.Post("/api/stores/" + c.store + "/sync")
		if res.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", c.store, res.Code, res.Body)
		}
		got := decode[syncing.SyncStoreResult](t, res.Body.Bytes())
		if got.Result != c.result || got.Message != c.message || got.MirrorWarning != c.mirror {
			t.Errorf("%s: %+v", c.store, got)
		}
	}
	if res := e.api.Post("/api/stores/S9/sync"); res.Code != http.StatusNotFound {
		t.Errorf("unknown store: %d", res.Code)
	}
}

func TestSyncAllAndStatus(t *testing.T) {
	e := newEnv(t)
	res := e.api.Post("/api/sync")
	all := decode[syncing.SyncAllResult](t, res.Body.Bytes())
	if res.Code != http.StatusOK || all.Result != "failed" || !slices.Equal(all.Lines, []string{"成功 1 件・失敗 1 件", "web: remote への書き込みが許可されていません"}) {
		t.Errorf("sync-all: %d %+v", res.Code, all)
	}
	res = e.api.Get("/api/sync")
	st := decode[syncing.SyncStatusList](t, res.Body.Bytes())
	if res.Code != http.StatusOK || len(st.Stores) != 2 {
		t.Fatalf("status: %d %s", res.Code, res.Body)
	}
	if s := st.Stores[0]; s.Store != "S1" || s.Error != "" || s.Unsent != 3 || s.LastSuccess != "2026-11-13T10:00:00Z" {
		t.Errorf("S1 = %+v", s)
	}
	if s := st.Stores[1]; s.Store != "S3" || s.Error != "remote への書き込みが許可されていません" || s.LastSuccess != "" {
		t.Errorf("S3 = %+v", s)
	}
}

// 決定 C2-18: タブの待つ条件とオンライン復帰を、自動の同期に伝える。
func TestHoldAndOnline(t *testing.T) {
	e := newEnv(t)
	if res := e.api.Put("/api/sync/hold", map[string]any{"tab": "t1", "hold": true}); res.Code != http.StatusNoContent {
		t.Fatalf("hold: %d %s", res.Code, res.Body)
	}
	if err := e.auto.Step(context.Background()); err != nil || len(e.rep.Calls()) != 0 {
		t.Fatalf("synced while held: %v %+v", err, e.rep.Calls())
	}
	if res := e.api.Put("/api/sync/hold", map[string]any{"tab": "t1", "hold": false}); res.Code != http.StatusNoContent {
		t.Fatalf("release: %d", res.Code)
	}
	if res := e.api.Post("/api/sync/online"); res.Code != http.StatusNoContent {
		t.Fatalf("online: %d", res.Code)
	}
	if err := e.auto.Step(context.Background()); err != nil || len(e.rep.Calls()) == 0 {
		t.Errorf("not synced after release: %v", err)
	}
}
```

(`apptest.Clock` は呼ぶたびに 1 ミリ秒進むので、`LastSuccess` が `10:00:00.001Z` などになるなら、時計の `Step` を 0 にするか、期待値をそれに合わせる。)

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: FAIL(`events`・`syncing` のパッケージがない)

- [ ] **Step 3: 実装する**

`internal/adapter/httpapi/events/events.go`:

```go
// Package events は、port.Events の実装と、画面に出来事を送る Server-Sent Events(06「画面への反映」)。
package events

import (
	"context"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"

	"github.com/kukv/hedgehug/internal/app/port"
)

// buffer は、1 つの画面に溜めておける出来事の数。溢れた分は落とす(画面は次の出来事で読み直す)。
const buffer = 64

// Hub は、つないでいる画面ごとに出来事を配る port.Events。
type Hub struct {
	mu   sync.Mutex
	subs map[chan port.Event]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[chan port.Event]struct{}{}} }

// Publish は、つないでいるすべての画面に e を送る。待たない。
func (h *Hub) Publish(e port.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Listeners は、つないでいる画面の数。
func (h *Hub) Listeners() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

func (h *Hub) subscribe() (<-chan port.Event, func()) {
	ch := make(chan port.Event, buffer)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// StoreChanged は、Store の中身が変わった出来事(event: store)。
type StoreChanged struct {
	Store string `json:"store"`
}

// SyncChanged は、Store の同期の状態が変わった出来事(event: sync)。
type SyncChanged struct {
	Store string `json:"store"`
}

// Register は、GET /api/events を登録する。画面は fetch で読み続ける(EventSource はヘッダーを付けられない。
// 決定 C2-20)。
func Register(api huma.API, h *Hub) {
	sse.Register(api, huma.Operation{
		OperationID: "watch-events", Method: http.MethodGet, Path: "/api/events",
		Summary: "Store の変化と同期の状態の変化を、Server-Sent Events で送り続ける",
	}, map[string]any{"store": StoreChanged{}, "sync": SyncChanged{}},
		func(ctx context.Context, _ *struct{}, send sse.Sender) {
			ch, stop := h.subscribe()
			defer stop()
			for {
				select {
				case <-ctx.Done():
					return
				case e := <-ch:
					var err error
					switch e.Kind {
					case port.EventStore:
						err = send.Data(StoreChanged{Store: string(e.Store)})
					case port.EventSync:
						err = send.Data(SyncChanged{Store: string(e.Store)})
					}
					if err != nil {
						return
					}
				}
			}
		})
}
```

`internal/adapter/httpapi/syncing/dto.go`:

```go
package syncing

// SyncStoreStatus は、remote がある 1 つの Store の同期の状態(06「同期の状態の表示」)。
type SyncStoreStatus struct {
	Store       string `json:"store"`
	Name        string `json:"name"`
	Hidden      bool   `json:"hidden"`
	ReadOnly    bool   `json:"readOnly"`
	Error       string `json:"error" doc:"同期エラーなら短い理由(06 の表の文言)。なければ空"`
	Unsent      int    `json:"unsent" doc:"未送信の保存の数"`
	LastSuccess string `json:"lastSuccess" doc:"最後に成功した同期の時刻(RFC 3339、UTC)。なければ空"`
}

// SyncStatusList は、同期の状態の一覧(サイドバーの並び。非表示の Store も含む)。
type SyncStatusList struct {
	Stores []SyncStoreStatus `json:"stores" nullable:"false"`
}

// SyncStoreResult は、1 つの Store を手動で同期した結果(決定 C2-21)。
type SyncStoreResult struct {
	Result        string `json:"result" enum:"synced,skipped,failed" doc:"synced はトースト、skipped は案内、failed はエラーのトースト"`
	Message       string `json:"message"`
	MirrorWarning string `json:"mirrorWarning" doc:"ミラーに送れなかったときの警告のトースト。なければ空"`
}

// SyncAllResult は、「すべて同期」の結果。Lines は 1 つのトーストの行。
type SyncAllResult struct {
	Result string   `json:"result" enum:"synced,failed,nothing"`
	Lines  []string `json:"lines" nullable:"false"`
}

// SyncHold は、見えているタブが、自動の同期を待たせる条件にあるかの知らせ(決定 C2-18)。
type SyncHold struct {
	Tab  string `json:"tab" minLength:"1" maxLength:"64"`
	Hold bool   `json:"hold"`
}
```

`internal/adapter/httpapi/syncing/syncing.go`:

```go
// Package syncing は、同期の API(仕様 06「手動で同期する」「同期の状態の表示」「自動で同期する」)を持つ。
package syncing

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appsyncing "github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// Services は、同期の API が使う usecase。どちらもサーバーに 1 つ。
type Services struct {
	Sync *appsyncing.Service
	Auto *appsyncing.Auto
}

type storeInput struct {
	Store string `path:"store"`
}

type statusOutput struct{ Body SyncStatusList }

type storeOutput struct{ Body SyncStoreResult }

type allOutput struct{ Body SyncAllResult }

type holdInput struct{ Body SyncHold }

func Register(api huma.API, s Services) {
	huma.Register(api, huma.Operation{
		OperationID: "list-sync-status", Method: http.MethodGet, Path: "/api/sync",
		Summary: "remote がある Store の同期の状態を返す",
	}, func(ctx context.Context, _ *struct{}) (*statusOutput, error) {
		st, err := s.Sync.Status(ctx)
		if err != nil {
			return nil, err
		}
		return &statusOutput{Body: toStatusList(st)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "sync-all", Method: http.MethodPost, Path: "/api/sync",
		Summary: "表示中で remote がある Store をすべて同期する",
	}, func(ctx context.Context, _ *struct{}) (*allOutput, error) {
		all, err := s.Sync.SyncAll(ctx)
		if err != nil {
			return nil, err
		}
		return &allOutput{Body: toAll(all)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "sync-store", Method: http.MethodPost, Path: "/api/stores/{store}/sync",
		Summary: "1 つの Store を同期する", Errors: []int{http.StatusNotFound},
	}, func(ctx context.Context, in *storeInput) (*storeOutput, error) {
		out, err := s.Sync.SyncStore(ctx, port.StoreID(in.Store))
		if err != nil {
			return nil, apierr.From(err)
		}
		return &storeOutput{Body: toStoreResult(out)}, nil
	})
	registerAuto(api, s.Auto)
}

func registerAuto(api huma.API, a *appsyncing.Auto) {
	huma.Register(api, huma.Operation{
		OperationID: "hold-sync", Method: http.MethodPut, Path: "/api/sync/hold",
		Summary: "見えているタブが、自動の同期を待たせる条件にあるかを知らせる", DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, in *holdInput) (*struct{}, error) {
		a.Hold(in.Body.Tab, in.Body.Hold)
		return nil, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "sync-online", Method: http.MethodPost, Path: "/api/sync/online",
		Summary: "オフラインからつながったことを知らせる(自動の同期の契機)", DefaultStatus: http.StatusNoContent,
	}, func(context.Context, *struct{}) (*struct{}, error) {
		a.Online()
		return nil, nil
	})
}

func toStatusList(st []appsyncing.StoreStatus) SyncStatusList {
	out := SyncStatusList{Stores: []SyncStoreStatus{}}
	for _, s := range st {
		last := ""
		if !s.LastSuccess.IsZero() {
			last = s.LastSuccess.UTC().Format(time.RFC3339)
		}
		out.Stores = append(out.Stores, SyncStoreStatus{
			Store: string(s.Store), Name: s.Name, Hidden: s.Hidden, ReadOnly: s.ReadOnly,
			Error: s.Reason.Short(), Unsent: s.Unsent, LastSuccess: last,
		})
	}
	return out
}

func toStoreResult(o appsyncing.Outcome) SyncStoreResult {
	r := SyncStoreResult{Result: "synced", Message: o.Message()}
	switch {
	case o.Skipped:
		r.Result = "skipped"
	case !o.OK():
		r.Result = "failed"
	}
	if o.MirrorFailures > 0 {
		r.MirrorWarning = syncstatus.MirrorsFailed(o.MirrorFailures)
	}
	return r
}

func toAll(a appsyncing.AllOutcome) SyncAllResult {
	r := SyncAllResult{Result: "synced", Lines: a.Lines}
	switch {
	case a.Nothing:
		r.Result = "nothing"
	case len(a.Lines) > 1:
		r.Result = "failed"
	}
	return r
}
```

`internal/adapter/httpapi/routes.go` を変える。

`Deps` に足す。

```go
	// トラック C(同期)。Sync と Auto はサーバーに 1 つで、cmd が作る(自動の同期を動かすため)。
	Sync   *appsyncing.Service
	Auto   *appsyncing.Auto
	Events *events.Hub // nil なら NewAPI が作る(知らせる者がいない)
	// GitHubSession は、GitHub の API と同期のトークンの入口で共用する Session。nil なら githubServices が作る。
	GitHubSession *ghapp.Session
```

`NewAPI` の登録に足す。

```go
	hub := deps.Events
	if hub == nil {
		hub = events.NewHub()
	}
	events.Register(api, hub)
	syncapi.Register(api, syncapi.Services{Sync: deps.Sync, Auto: deps.Auto})
```

`githubServices` の最初の行を替える。

```go
	s := d.GitHubSession
	if s == nil {
		s = ghapp.NewSession()
	}
```

import に `"github.com/kukv/hedgehug/internal/adapter/httpapi/events"`・`syncapi "github.com/kukv/hedgehug/internal/adapter/httpapi/syncing"`・`appsyncing "github.com/kukv/hedgehug/internal/app/syncing"` を足す。

- [ ] **Step 4: 生成物を作り直し、テストを通す**

```bash
make generate
cd web && pnpm gen && cd ..
go test ./internal/adapter/... ./internal/archtest/ && golangci-lint run ./internal/adapter/...
```

Expected: PASS、指摘なし。`api/openapi.yaml` に 6 つの操作と、`SyncStoreStatus` ほかのスキーマが入る。`make web-check` も通る

- [ ] **Step 5: コミットする**

```bash
git add internal/adapter api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(adapter): 同期の API と、Store と同期の変化を送る Server-Sent Events を足す(C2 Task 12)"
```

---

### Task 13: 組み立て(cmd/hedgehug)

**Files:**
- Modify: `cmd/hedgehug/wire.go`・`nostores.go`・`paths.go`・`wire_e2e.go`
- Test: `cmd/hedgehug/main_test.go`(`statePath` のテストを足す)

**Interfaces:**
- Consumes: Task 3〜12 のすべて
- Produces: 本物の組み立てで、同期の API・自動の同期・見張りが動く(Store の一覧は G が入るまで空。決定 C2-2)

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/main_test.go` に足す(`configPath` のテストがあれば、その形に合わせる)。

```go
func TestStatePathFollowsXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	got, err := statePath()
	if err != nil || got != filepath.Join("/tmp/xdg-data", "hedgehug", "state.db") {
		t.Errorf("statePath = %q, %v", got, err)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/tmp/home")
	got, err = statePath()
	if err != nil || got != filepath.Join("/tmp/home", ".local", "share", "hedgehug", "state.db") {
		t.Errorf("statePath without XDG = %q, %v", got, err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./cmd/hedgehug/ -run StatePath`
Expected: FAIL(`statePath` がない)

- [ ] **Step 3: 実装する**

`cmd/hedgehug/paths.go` に足す。

```go
// statePath は state.db の場所(保存形式「このマシンに持つもの」)。XDG_DATA_HOME があればその下、
// なければ ~/.local/share の下。
func statePath() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "hedgehug", "state.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "hedgehug", "state.db"), nil
}
```

`cmd/hedgehug/nostores.go` に足す。

```go
// noSyncTargets は、Store の一覧(トラック G)が入るまでの仮。同期する Store はない(決定 C2-2)。
type noSyncTargets struct{}

func (noSyncTargets) List(context.Context) ([]port.SyncTarget, error) { return nil, nil }
```

`cmd/hedgehug/wire.go` の `wire` を次に替える(`noIdentity`・`tokenAndAddr`・`newToken` はそのまま)。

```go
// wire は、本物の部品を口に差し込み、自動の同期と Store の見張りを ctx が終わるまで動かす。
// Store の一覧の読み込みはトラック G が足す(この計画の決定 6、C2 の決定 C2-2)。
func wire(ctx context.Context) (httpapi.Deps, error) {
	clock, ids := sysclock.Clock{}, sysclock.IDs{}
	stores := gitstore.New(clock, ids)
	cfg, err := configPath()
	if err != nil {
		return httpapi.Deps{}, err
	}
	state, err := statePath()
	if err != nil {
		return httpapi.Deps{}, err
	}
	db, err := statedb.Open(state)
	if err != nil {
		return httpapi.Deps{}, err
	}
	file := machineconfig.Open(cfg)
	gh := github.New(nil) // 時計のずれはトラック C(C5)が受け取る(トラック D の計画の決定 3)
	secrets := secret.New()
	session := ghapp.NewSession()
	hub := events.NewHub()
	svc := &syncing.Service{
		Targets: noSyncTargets{},
		Tokens: ghapp.Connector{
			Session: session, Accounts: file.Accounts(), Secrets: secrets, GitHub: gh, Device: gh,
			Hosts: file.Hosts(), Stores: noStoreGitHub{}, Clock: clock,
		},
		Replicator: gitstore.NewReplicator(stores), State: db.SyncState(), Events: hub, Clock: clock,
	}
	auto := syncing.NewAuto(svc, file.SyncSettings())
	go auto.Run(ctx)
	go (&syncing.Watch{Versions: stores, Events: hub}).Run(ctx, 500*time.Millisecond)
	return httpapi.Deps{
		Stores:        syncing.NotifyWrites(stores, auto.Wrote),
		IDs:           ids,
		Identity:      noIdentity{},
		Accounts:      file.Accounts(),
		Secrets:       secrets,
		GitHub:        gh,
		DeviceFlow:    gh,
		Hosts:         file.Hosts(),
		StoreGitHub:   noStoreGitHub{},
		Clock:         clock,
		Sync:          svc,
		Auto:          auto,
		Events:        hub,
		GitHubSession: session,
	}, nil
}
```

(state.db は閉じない。プロセスが終わるときに閉じる。サーバーを止める 5 秒のあいだも、動いている同期が記録できるようにするため。)

import に `"time"`・`"github.com/kukv/hedgehug/internal/adapter/httpapi/events"`・`ghapp "github.com/kukv/hedgehug/internal/app/github"`・`"github.com/kukv/hedgehug/internal/app/syncing"`・`"github.com/kukv/hedgehug/internal/infra/statedb"` を足す。

`cmd/hedgehug/wire_e2e.go` の `return httpapi.Deps{...}` の前に足し、`Deps` に 4 つの欄を足す(e2e は自動の同期を動かさない。同期の API は偽物で答える)。

```go
	hub := events.NewHub()
	syncSvc := &syncing.Service{
		Targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: e2eStore, Name: "チーム", Remote: "https://example.invalid/team.git", Account: "e2e-account"},
		}},
		Tokens:     &apptest.SyncTokens{Tokens: map[port.StoreID]string{e2eStore: "e2e-gh-token"}},
		Replicator: &apptest.Replicator{},
		State:      apptest.NewSyncState(),
		Events:     hub,
		Clock:      apptest.NewClock(time.Now()),
	}
	auto := syncing.NewAuto(syncSvc, &apptest.SyncSettings{Value: port.SyncTiming{Auto: false}})
```

```go
		Sync:   syncSvc,
		Auto:   auto,
		Events: hub,
```

- [ ] **Step 4: 全体を確かめる**

```bash
make check
go build -tags e2e ./cmd/hedgehug/ && golangci-lint run --build-tags e2e ./cmd/hedgehug/
```

Expected: すべて PASS。`go run ./cmd/hedgehug` で起動し、表示された URL のトークンで `curl -H "Authorization: Bearer <トークン>" http://127.0.0.1:<番号>/api/sync` が `{"stores":[]}` を返し、`~/.local/share/hedgehug/state.db`(または `XDG_DATA_HOME` の下)ができる(確かめたら止める)

- [ ] **Step 5: コミットする**

```bash
git add cmd/hedgehug
git commit -m "feat(cmd): 同期の usecase・自動の同期・Store の見張り・state.db を組み立てる(C2 Task 13)"
```

---

## C2 のあと

- **C2 画面**(決定 C2-1): A2 の Task 5・6・7 が develop に入ったら、計画を書く。中身は、同期カード(要約・最終同期・24 時間・「すべて同期」のボタンと案内)、「Store ごとの同期状態」のメニュー、860px 以下の点、オフラインの帯(自動同期のオン・オフで文を替える)、手動の同期のトースト(`result` で種類を選ぶ。オフラインなら送らずに 06 の文言)、Shift+Ctrl+S、待つ条件の判定と `PUT /api/sync/hold`、`online` の知らせ、`GET /api/events` を fetch で読み続けて一覧の状態を読み直すこと(待つ条件のあいだは溜めて 1 回で)。core に置くロジック(要約の数え方、24 時間、待つ条件)は TS の単体テストで確かめる。
- **トラック G**: `port.SyncTargets` と `port.StoreGitHub` の本物(config.yaml の Store の一覧)。Store を clone で追加したら `refs/hedgehug/remote/<ブランチ>` を作る(決定 C2-14)。Store をこのマシンから削除したら、state.db の `sync_status`・`mirror_status` のその Store の行を消す(保存形式「このマシンに持つもの」。`SyncState` に消す関数を足す)。`Service.AfterSuccess` に完全な削除を差し込む。設定の「一般」の同期のタイミングの書き換え。
- **C3**: 同期の修復の保存は、`NotifyWrites` で包む前の `port.Stores` で書く(修復で「書き込み後」の同期を起こさないため)。`Service.run` の、`Succeeded` の前に修復を足す。
- **C4**: アクティビティの同期の行は、`SyncState` に「成功した同期の行」を足して記録する(決定 C2-5)。受信箱の出来事は `port.EventKind` に `inbox` を足す。
- **C5**: D の `github.New(onSkew)` に時計のずれの記録を差し込む。

## 実装で計画から変えたこと

(実装のあとで書く)
