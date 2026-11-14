# hedgehug トラック C4a(受信箱の土台)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通知を `state.db` に保存し、受信箱の画面(タブと未読数・行・既読の切り替え・すべて既読・さらに読み込む・空のとき)と、サイドバーの「受信箱」の未読数を作る。自動化の通知 2 種(自動化・自動化が制約で止まった)を、いまの捨てる口(`noInbox`)から本物にし、止まった通知の「制約を無視して {ステータス名} にする」「変更しない」を動かす。

**Architecture:** `port.Inbox`(足す口、B4-12)はそのままにし、読み書きの口 `port.InboxBook` を同じファイルに足す。実装は `internal/infra/statedb/inbox.go`(表 `inbox/001`)。usecase は新しいパッケージ `internal/app/inbox`: 一覧・未読数・既読・すべて既読・止まった通知の 2 つのボタン。一覧は、表示中の Store(`port.StoreList`)の行だけを読み、読む前に完全に削除されたもの(`purged.jsonl`)の行を消す。行の対象(アイテムの ID・タイトル・ワークスペース名・削除済み)は、読むときに内部 ID から引く。通知の追加と既読の変化は、新しい出来事 `inbox` で画面に知らせる。画面は core の `features/inbox` と app の `screens/inbox`、アドレス `/inbox`。

**Tech Stack:** Go 1.27.3(huma v2、modernc.org/sqlite)、TypeScript・Svelte(web/packages/core・api・ui、web/app)、vitest、Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「通知」(表の「自動化」「自動化が制約で止まった」の行、作らない場合の箇条)、「自動化」、「受信箱」の全節(タブと未読数・並びと行の表示・操作・件数・空のとき)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`「制約で止まったとき」(`:510-519`)、`:355`(ステータスの削除でルールが変わった通知)、`:500-512`
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/01-overview.md` `:79`(通知はこのマシンだけ)、`:112`(既読は保存に入らない)、`:136`(相対時間)、`:155`(不明なユーザー)、`:185`(Ctrl+クリックでは既読にしない)、`:195`(サイドバー)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/08-stores-maintenance.md` `:156-158`(非表示の Store)、`:265`(このマシンから削除)、`:385`(完全な削除で通知が消える)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:239`(完全な削除は、下のものも 1 つずつ `purged.jsonl` に載る)、`:282-285`(`state.db` に持つもの・Store ID の行を消す)
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md` `:152`(`Inbox` は infra/statedb)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C4 の行

## Global Constraints

土台・トラック C の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個(`internal/archtest/size.go`)。`internal/app/port`・`internal/app/apptest`・`internal/app/items` は 20 個で満杯なので、ファイルを足さない(`port/inbox.go`・`apptest/inbox.go` を太らせる)。web は .svelte 200 行、.ts 300 行(`web/scripts/check-size.mjs`)。`web/app/src/wire.ts` は 295 行なので、受信箱の組み立ては新しいファイル `web/app/src/inbox.ts` に書く。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 画面の文言(06 のとおり):
  - 見出し「受信箱」、補足「同期で届いた変更、メンション、割り当て、上書きされた値」
  - タブ: すべて・メンション・割り当て・ウォッチ・上書き・自動化・同期の修復(この順)。未読が 0 のタブは数字を出さない
  - ピル: メンション・割り当て・ウォッチ・上書き・自動化・同期の修復(自動化が制約で止まった通知も「自動化」)
  - ボタン「すべて既読にする」、「さらに読み込む」
  - 差出人がないとき「hedgehug」、メンバーにいないとき「不明なユーザー」
  - 対象: 「削除されたアイテム」「削除されたワークスペース」(リンクにしない)
  - 止まった通知: 「制約を無視して {ステータス名} にする」「変更しない」、結果のピル「適用しました」「このままにしました」、トースト「適用しました」、無効の理由「閲覧のみの Store です」「すでに変更されています(いまは {ステータス名})」「「{ステータス名}」は削除されています」、重なったときのエラーのトースト「{ID} は先に変更されたため、適用しませんでした」
  - 空のとき: 「お知らせはありません」。上書きのタブの補足「このマシンで変えた値が、同期であとから届いた変更に負けると、ここに出ます。試すには、値を変えてからサイドバーの同期ボタンを押してください。」、同期の修復のタブの補足「同期のときに直したもの(番号の重なり、削除された親、親の循環、移動した親に合わせて移した子など)がここに出ます。」
- 一度に出す件数は 300(06「件数」)。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・`make web-check`・この計画で足した e2e の spec。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C4a-1 | 範囲: 通知の保存と既読、受信箱の画面とサイドバー、自動化の通知 2 種、止まった通知の 2 つのボタン。受け取った保存からの通知(割り当て・メンション・ウォッチ・同期の修復・受け取ったステータス削除からの自動化)と判定した保存の記録は C4b、上書きと「自分の値に戻す」は C4c、アクティビティは C4d | ユーザーの決定(2026-11-14) |
| C4a-2 | 表は `inbox/001` の 1 つ。列は `id`(INTEGER PRIMARY KEY AUTOINCREMENT)・`store`・`workspace`・`kind`・`item`・`rule`・`text`・`reason`・`status`・`from_status`・`display`・`sender`・`at`(UTC の固定幅の時刻の文字列。ナノ秒 9 桁にそろえる。`timeText` は末尾の 0 を削り、文字列の比較で並びが狂うため使わない)・`read`(0/1)・`result`(''・'applied'・'kept')。索引は `(store, at, id)`。C4b・C4c が要る列は、そのとき新しい手順で足す | statedb の「機能/連番」の手順(`migrate.go`)。使わない列を先に作らない |
| C4a-3 | 通知を作った時刻は、`statedb` の受信箱が `Add` のときに時計(`port.Clock`)で刻む。呼ぶ側は時刻を渡さない | `workspaces.Service` は時計を持たない。3 つの呼び出し元で同じ刻み方にする |
| C4a-4 | 並びは `at` の新しい順、同じなら `id` の大きい順。続きは、最後の行の `(at, id)` を印にして、それより前の 300 件を読む。301 件目があれば「さらに読み込む」を出す | 06「並びと行の表示」「件数」。読み込むあいだに通知が増えても重ならない |
| C4a-5 | 表示中の Store だけを読む: 一覧・未読数・すべて既読は、`port.StoreList.List` が返す Store の行だけを対象にする。非表示の Store の行は消さず、再表示すれば出る | 08 `:156-158`。`StoreList.List` は非表示を返さない |
| C4a-6 | 完全な削除の行の後始末: 一覧と未読数を読む前に、表示中の Store ごとに `purged.jsonl`(`Reader.Purged().List()`)を読み、`entity` が item・project・workspace の `target` を、通知の `item` に持つ行と、アイテムのない(`item` が空の)行のうち `workspace` に持つ行を消す(`InboxBook.Prune`)。ワークスペースを完全に削除したときは、下のアイテムも 1 つずつ `purged.jsonl` に載る(保存形式 `:239`)ので、アイテムの行はアイテムの ID で消える。アイテムの行を `workspace` で消さないのは、通知の `workspace` が作ったときの値で、別のワークスペースへ移った生きているアイテムの通知まで消えるため。手動の完全な削除(G)と同期で受け取った完全な削除の両方を、この 1 か所で片付ける | 06「完全に削除したアイテム・プロジェクト・ワークスペースの通知は、一覧から消える。完全な削除を同期で受け取ったマシンも、そのものの通知を消す」。受け取りを app に知らせる口がない(`sync_tree.go` のマージで取り込む)ので、読むときに片付ける。消すので未読数も合う |
| C4a-7 | Store をこのマシンから削除したときの後始末は `InboxBook.ForgetStore(store)`。呼ぶのは、その操作を作る G。C4a は口とテストだけを作り、トラック C の計画の G への申し送りの表に書く | このマシンから削除する usecase がまだない(G の持ち場)。保存形式 `:285` は Store ID の行をすべて消すと定めており、G は state.db の全部の表を消す |
| C4a-8 | 行の対象は、読むときに内部 ID から引く: アイテムは通知の Store で `Items().Get`。見つかって削除済みでなければ、いまのワークスペースの接頭辞で ID(`DisplayID`)・種類・タイトルを出し、クリックで詳細パネルを開く。削除済み(`Deleted != nil`)なら「削除されたアイテム」(リンクなし)。見つからない(完全な削除は C4a-6 で消えているので、別の Store へ移った)なら、通知を作ったときの ID(`display` 列)を出し、クリックで `/s/{Store 名}/i/{その ID}`(転送記録の解決、`locate-item`)を開く(ユーザーの承認、2026-11-14)。アイテムのない通知(ルールが変わった通知)は、Store の点とワークスペース名を出し、クリックでワークスペースの設定の「自動化」を開く。ワークスペースが削除済みなら「削除されたワークスペース」(リンクなし) | 06「並びと行の表示」。接頭辞・タイトル・ワークスペースは通知のあとに変わるので、作ったときの値を出さない。別の Store へ移ったものの旧 ID のリンクは 06 の同じ段落 |
| C4a-9 | 止まった通知は、止まったときのステータス(`from_status`)を覚える。`automation.Outcome` に `From change.ID` を足し、`automation.Run` が止まった実行の前のステータス(前のルールが変えたあとの値)を入れる。`port.Notice` に `From`・`Display` を足し、`items.Service.notify`・`githublink.Links.notify` が入れる | 06「ボタンの有効と無効は、通知を作ったときの値を覚えておき、いまの値と比べて決める」。03 `:503`(次のルールは、変えたあとのステータスから検査する) |
| C4a-10 | 止まった通知のボタンの状態は、上から順に決める: (1) 閲覧のみの Store → 2 つとも無効、「閲覧のみの Store です」。(2) アイテムが削除済み・Store にない → ボタンを出さない。(3) いまのステータスが `from_status` と違う → 「制約を無視して…」は無効、「すでに変更されています(いまは {ステータス名})」。(4) 変える先のステータスが削除済み → 「制約を無視して…」は無効、「「{ステータス名}」は削除されています」。「変更しない」は (1) のほか常に有効。結果(`result`)があれば、ボタンの代わりに結果のピルを出す | 06「操作」の順(閲覧のみが先、削除済みはボタンなし)。(3) と (4) が両方当たるときは、いまの値が違うことを先に知らせる(押しても戻す元がもうない) |
| C4a-11 | 「制約を無視して…」は受信箱の新しい経路 `POST /api/inbox/{notice}/apply` にする。`automations.Service.ApplyIgnoringWorkflow` に引数 `from change.ID` を足し、保存の中でいまのステータスが `from` でなければ `refusal.New("{ID} は先に変更されたため、適用しませんでした")` で断る(何も書かない)。適用したら、その行を既読にし `result = applied`。今ある `POST /api/stores/{store}/items/{item}/automation/{rule}/apply` は、画面から呼ぶところがなく、重なりの検査がないので消す(ユーザーの承認、2026-11-14) | 06「押した瞬間に…値が変わっていたときは、保存せず、エラーのトースト」。古い経路を残すと、検査なしで書ける道が残る |
| C4a-12 | 「変更しない」は `POST /api/inbox/{notice}/keep`。既読にし `result = kept`。保存はしない。結果がすでにある行には、apply・keep とも `refusal.New("この通知はすでに処理されています")` で断る | 06「操作」、01 `:112`。2 つのタブで別々に押したときに、2 回目を書かない。断りの文はユーザーの承認(2026-11-14) |
| C4a-13 | 画面への知らせ: `port.EventInbox = "inbox"` を足す。通知を足したとき(`app/inbox.Notifier` が `port.Inbox` を包む)、既読・すべて既読・apply・keep のあとに出す。`Event.Store` は通知の Store(すべて既読は空)。画面は `inbox` を受けたら、未読数と、開いていれば一覧を読み直す | 自動化の通知は保存(`store` の出来事)のあとに書くので、`store` で読み直すと通知が間に合わないことがある。GitHub の取り直しで動いた通知と、別のタブの既読も同じ道で届く |
| C4a-14 | 本物の組み立て(`wire.go`)は `statedb` の受信箱を使い、`noInbox` を消す。`StoreList` は G まで `noStores`(空)なので、本物のビルドの受信箱は G まで何も出さない。e2e のビルド(`wire_e2e.go`)は `e2eReads` で開いた `state.db` の受信箱を使う | 本物の Store の一覧は G の持ち場(`project-rebuild-progress`) |
| C4a-15 | ワークスペースの設定の「自動化」を外から開く: `navigation.go(routeOfWorkspace(…, "settings"), "automation")`。ワークスペースの設定の画面は、開いたあと `navigation.takeSection()` が `"automation"` なら `workspaceSettings.selectTab("automation")` を呼ぶ | いまの `open()` はワークスペースが変わるとタブを「全般」に戻し、外から「自動化」を開けない。区画の受け渡しはプロジェクトの設定(B5a-22)と同じ形 |
| C4a-16 | コマンドパレットの「受信箱を開く」は A5c が C4 に渡したもの。A5c が develop に入っていれば Task 9 で足す。入っていなければ、A5c の計画の該当行に「C4a が先に入ったので A5c が足す」と書く(develop にあとから入る方が合わせる) | A5c の計画「行き先の画面がない候補は作るトラックに回す」。F1 Task 19 と同じ扱い |

## 実装で計画から変えたこと

下の各 Task の文より、この節を優先する。

- `at` 列は固定幅の UTC 文字列(C4a-2 のとおり)で実装済み。
- `inbox.Service` の関数のフィールド名は `ApplyIgnoringWorkflow`(メソッド `Apply` と名前が重なるため)。
- 未読数の API は 7 つのタブを持つ構造体 `InboxCounts`。タブの一覧は `appinbox.Tabs()`。続きの印は `before_at`・`before_id` の両方で、片方だけは 400。
- Route は `{ name: "inbox" }`(`kind` ではない)。C4a-15 は `workspaceSettings.open(store, ws, section)` が区画を受け取る形。
- 画面: 左端の点は `InboxDot.svelte` に分けた。`createInbox` は `canReadAll` と `loaded` も返す。読み込み・書き込みの失敗は、404 なら黙って読み直し、それ以外は読み直してから `toasts.show(e.message, { error: true })`(新しい文言は作らない)。「さらに読み込む」のあとの読み直しは、出していた件数を覆うまで読む。
- 止まった通知でアイテムがないときも `Action` は返し、`visible=false`。
- C4a-16: A5c は develop に入っていない(ブランチ `feat/track-a5c` の計画が「受信箱を開く」を C4 に渡している)ので、パレットのコードは C4a で触らない。トラック C の計画の C4a の行に、A5c が C4a のあとに入るなら A5c が足すと書いた。

## Review Focus

1. **非表示の Store の通知:** 非表示の Store の止まった通知は、一覧にも「すべて」と「自動化」の未読数にも入らず、「すべて既読にする」でも既読にならない。再表示で未読のまま戻る(Task 3 の `TestPageSkipsHiddenStore`、`TestReadAllSkipsHiddenStore`)。
2. **完全に削除したアイテムの通知:** アイテムを完全に削除した(`purged.jsonl` に載った)あと、受信箱を開くと、その行が消え、未読数も減る。ワークスペースの完全な削除では、下のアイテムの通知もルールの通知も消える(Task 3 の `TestPagePrunesPurged`)。
3. **2 つのタブで同じボタン:** タブ A で「変更しない」、タブ B で「制約を無視して…」を押すと、B は断られ、何も保存されない(Task 4 の `TestApplyAfterKeepRefused`)。押す前に別の操作でステータスが変わっていたら、エラーのトーストが出て、ステータスは変わらない(Task 4 の `TestApplyRefusedWhenStatusChanged`)。
4. **連鎖のあとに止まったルール:** 1 つ目のルールが Todo → InProgress に変え、2 つ目が InProgress → Done で止まったとき、通知の「止まったときのステータス」は InProgress で、「制約を無視して Done にする」が有効になる(Task 2 の `TestRunStoppedFromIsAfterEarlierRule`)。
5. **300 件の境目:** 301 件あるとき、最初の 300 件と「さらに読み込む」が出て、押すと 1 件が足され、ボタンが消える。同じ時刻の行が境目にまたがっても重ならず抜けない(Task 1 の `TestInboxListPagesBySameTime`)。

---

### Task 1: 受信箱の口と `state.db` の表

**Files:**
- Modify: `internal/app/port/inbox.go`
- Create: `internal/infra/statedb/inbox.go`
- Test: `internal/infra/statedb/inbox_test.go`
- Modify: `internal/app/apptest/inbox.go`(偽物に `InboxBook` を足す)
- Test: `internal/app/apptest/inbox_test.go`(なければ作る。テストはファイル数に数えない)

**Interfaces:**
- Produces(`port`):
  - `Notice` に `From change.ID`(止まったときのステータス)・`Display string`(作ったときのアイテムの ID。アイテムがなければ空)・`Sender string`(差出人のログイン名。空は「hedgehug」)を足す
  - `type NoticeID int64`
  - `type NoticeResult string`、`NoticeApplied = "applied"`、`NoticeKept = "kept"`
  - `type NoticeRow struct { ID NoticeID; Notice; At time.Time; Read bool; Result NoticeResult }`
  - `type NoticeCursor struct { At time.Time; ID NoticeID }`
  - `type NoticeQuery struct { Stores []StoreID; Kinds []NoticeKind /* 空はすべて */; Before *NoticeCursor; Limit int }`
  - `type InboxBook interface {`
    - `Inbox`
    - `List(ctx, q NoticeQuery) ([]NoticeRow, error)` — C4a-4 の並び。`Stores` が空なら空
    - `Unread(ctx, stores []StoreID) (map[NoticeKind]int, error)`
    - `Get(ctx, id NoticeID) (NoticeRow, error)` — なければ `ErrNotFound`
    - `SetRead(ctx, id NoticeID, read bool) error` — なければ `ErrNotFound`
    - `ReadAll(ctx, stores []StoreID, kinds []NoticeKind) error`
    - `Resolve(ctx, id NoticeID, r NoticeResult) error` — 既読にして結果を書く。なければ `ErrNotFound`、結果がすでにあれば `refusal.New("この通知はすでに処理されています")`
    - `Prune(ctx, store StoreID, targets []change.ID) error` — `item` か `workspace` が targets にある行を消す
    - `ForgetStore(ctx, store StoreID) error`
  - `}`
- Produces(`statedb`): `func (d *DB) Inbox(clock port.Clock) port.InboxBook`
- Produces(`apptest`): `type Inbox` が `port.InboxBook` を満たす。`Clock port.Clock`(nil なら `time.Unix(0,0)` から 1 秒ずつ進める)と、いまの `Err`・`Notices()` を残す

- [ ] **Step 1: 失敗するテストを書く(`statedb/inbox_test.go`)**
  - `TestInboxAddAndList`: 時計を 10:00・10:01・10:02 に進めながら 3 件 Add → `List(Stores: [s1], Limit: 300)` が新しい順で 3 件、`At` が刻んだ時刻、`Read == false`、`From`・`Display`・`Reason`・`Status` が戻る
  - `TestInboxListPagesBySameTime`: 同じ時刻で 5 件 Add → `Limit: 3` で 3 件(id の大きい順)、最後の行を `Before` にして 2 件。合わせて 5 件で重ならない
  - `TestInboxListFiltersStoresAndKinds`: s1・s2 に 1 件ずつ → `Stores: [s1]` は 1 件。`Kinds: [automation_stopped]` は止まった行だけ
  - `TestInboxUnreadAndRead`: 3 件 → `Unread([s1])` が `{automation: 2, automation_stopped: 1}`。`SetRead(id, true)` で 1 減り、`SetRead(id, false)` で戻る。`ReadAll([s1], [automation])` で automation が 0、stopped は 1 のまま
  - `TestInboxResolve`: `Resolve(id, applied)` → `Get` が `Read == true`・`Result == applied`。2 回目は `refusal` のエラー(`errors.As` で文が「この通知はすでに処理されています」)。ない id は `port.ErrNotFound`
  - `TestInboxPrune`: item=i1 の行、workspace=w1 でアイテムなしの行、ほかの行 → `Prune(s1, [i1])` で 1 件消える。`Prune(s1, [w1])` で w1 の行がすべて消える。s2 の同じ ID の行は残る
  - `TestInboxForgetStore`: s1・s2 → `ForgetStore(s1)` で s1 の行だけ消える
  - `TestInboxSurvivesReopen`: Add のあと `Close`・`Open` しても行と既読が残る

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/statedb/ -run TestInbox`
Expected: FAIL(`DB.Inbox` がない)

- [ ] **Step 3: `port/inbox.go` の型と、`statedb/inbox.go`(`register("inbox/001", …)`、C4a-2 の列)を書く**

時刻は `sync.go` の `timeText`/`parseTime`。`Before` は `WHERE at < ? OR (at = ? AND id < ?)`。`IN (...)` は件数ぶんの `?` を作る。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/statedb/`
Expected: PASS

- [ ] **Step 5: `apptest.Inbox` に同じ振る舞いを足し、`apptest/inbox_test.go` に Step 1 のうち `TestInboxListPagesBySameTime`・`TestInboxUnreadAndRead`・`TestInboxResolve`・`TestInboxPrune` と同じ確かめを書く。`go test ./internal/app/apptest/ -run Inbox` が PASS**

- [ ] **Step 6: コミット**

```bash
git add internal/app/port/inbox.go internal/infra/statedb/inbox.go internal/infra/statedb/inbox_test.go internal/app/apptest/inbox.go internal/app/apptest/inbox_test.go
git commit -m "feat(inbox): 通知と既読を state.db に保存する口を足す(C4a)"
```

---

### Task 2: 止まったときのステータスと通知の ID を覚える

**Files:**
- Modify: `internal/domain/automation/run.go`(`Outcome.From`、`Run`)
- Test: `internal/domain/automation/run_test.go`
- Modify: `internal/app/items/automate.go:86-103`(`notify`)
- Modify: `internal/app/githublink/fire.go:166-183`(`notify`)
- Test: `internal/app/items/automate_test.go`、`internal/app/githublink/fire_test.go`(既存のテストのファイル)

**Interfaces:**
- Consumes: Task 1 の `port.Notice.From`・`Display`
- Produces: `automation.Outcome.From change.ID` — 止まった実行(`Draft == nil`)の、検査に使ったいまのステータス。変えた実行では空

- [ ] **Step 1: 失敗するテストを書く**
  - `TestRunStoppedFromIsAfterEarlierRule`(domain): Todo のアイテム。ルール 1「→ InProgress」(制約で通る)、ルール 2「→ Done」(InProgress → Done を制約で禁じる)。結果の 2 件目が `Draft == nil`・`From == InProgress`
  - `TestRunStoppedFromIsCurrent`(domain): ルール 1 件だけが止まる → `From` はアイテムのステータス
  - items: 止まるルールのある既存のテスト(`Inbox.Notices()` を見るもの)に、`From` がアイテムのステータス、`Display` が `"HH-1"` の形であることを足す。変えた通知にも `Display` が入る
  - githublink: 同じく止まった通知の `From`・`Display`

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/automation/ ./internal/app/items/ ./internal/app/githublink/`
Expected: FAIL(`From` がない、空)

- [ ] **Step 3: 実装する**

`Run` は、ルールごとに検査しているいまのステータスを、止まったとき `Outcome.From` に入れる。`notify` は `n.From = o.From`、`n.Display` はアイテムのワークスペースの接頭辞で `f.item.DisplayID(prefix)`(接頭辞は `notify` の呼び出し元がすでに読んだ設定・ワークスペースから渡す。読み直すなら tx の中で読んだものを `fired` に持たせる)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/automation/ ./internal/app/items/ ./internal/app/githublink/ ./internal/app/workspaces/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git commit -am "feat(automation): 止まった通知に止まったときのステータスとアイテムの ID を持たせる(C4a)"
```

---

### Task 3: 受信箱の一覧と未読数(`internal/app/inbox`)

**Files:**
- Create: `internal/app/inbox/service.go`(`Service`・タブ・`Counts`)
- Create: `internal/app/inbox/page.go`(`Page`・後始末 C4a-6)
- Create: `internal/app/inbox/target.go`(対象の解決 C4a-8・ボタンの状態 C4a-10)
- Test: `internal/app/inbox/page_test.go`、`internal/app/inbox/target_test.go`

**Interfaces:**
- Consumes: Task 1 の `port.InboxBook`、`port.Stores`(`Read` で `Items().Get`・`Workspaces().Get`・`Purged().List()`・設定)、`port.StoreList`
- Produces:
  - `type Tab string`: `TabAll = "all"`・`TabMention = "mention"`・`TabAssign = "assign"`・`TabWatch = "watch"`・`TabOverwrite = "overwrite"`・`TabAutomation = "automation"`・`TabRepair = "repair"`。`func KindsOf(t Tab) []port.NoticeKind`(all は nil。automation は `automation`・`automation_stopped`。ほかは同じ名前の種類 1 つ。Task 1 で `port` に 7 種類の定数 `NoticeMention = "mention"`・`NoticeAssign = "assign"`・`NoticeWatch = "watch"`・`NoticeOverwrite = "overwrite"`・`NoticeRepair = "repair"` を足しておく。書くのは C4b・C4c)、`func TabOf(k port.NoticeKind) Tab`
  - `type Service struct { Book port.InboxBook; Stores port.Stores; List port.StoreList; Events port.Events }`
  - `func (s Service) Counts(ctx) (map[Tab]int, error)` — 7 つのタブすべての鍵を持つ
  - `func (s Service) Page(ctx, tab Tab, before *port.NoticeCursor) (Page, error)`
  - `type Page struct { Rows []Row; More bool; Unread map[Tab]int }`
  - `type Row struct { ID port.NoticeID; Kind port.NoticeKind; At time.Time; Read bool; Text, Reason, Sender string; Store port.StoreInfo; Target Target; Action *Action /* 止まった通知だけ */ }`
  - `type Target struct { Kind TargetKind; Item item.ID; ItemKind item.Kind; Display, Title string; Workspace workspace.ID; WorkspaceName string; Section string }`。`TargetKind`: `"item"`・`"moved"`(旧 ID。`Display` に旧 ID)・`"deleted_item"`・`"workspace"`(`Section == "automation"`)・`"deleted_workspace"`
  - `type Action struct { Status change.ID; StatusName string; Visible bool; ApplyEnabled, KeepEnabled bool; Note string; Result port.NoticeResult }`

- [ ] **Step 1: 失敗するテストを書く**(`apptest.Stores` に `apptest.Sample` を入れ、`apptest.StoreList`・`apptest.Inbox` で組む)
  - `TestPageNewestFirstAndCounts`: 3 件 Add → `Page(TabAll, nil)` が新しい順、`Unread[TabAll] == 3`・`Unread[TabAutomation] == 3`・`Unread[TabMention] == 0`
  - `TestPageTabFilters`: `Page(TabMention, nil)` は空
  - `TestPageMoreAt300`: 301 件 → 300 件と `More == true`。最後の行の `(At, ID)` で次 → 1 件と `More == false`
  - `TestPageSkipsHiddenStore`: Store s2 を `StoreList` から外す → s2 の行は出ず、未読数にも入らない。戻すと未読のまま出る
  - `TestPagePrunesPurged`: アイテムの行とワークスペースの行を Add → `apptest.Stores` で、そのアイテムを完全に削除したことにする(`purged.jsonl` に載せる。apptest の既存の手段を使う) → `Page` に出ず、`Book` からも消えている。ワークスペースを完全に削除すると、そのワークスペースの行がすべて消える
  - `TestTargetItem`: 生きているアイテム → `Kind == "item"`・`Display == "HH-1"`・`Title`・`ItemKind`。接頭辞をあとで変えると、新しい接頭辞で出る
  - `TestTargetDeletedItem`: 削除済み → `"deleted_item"`
  - `TestTargetMoved`: Store にない(完全な削除でない) → `"moved"`・`Display` は通知の `Display`
  - `TestTargetRuleNotice`: アイテムなし → `"workspace"`・`WorkspaceName`・`Section == "automation"`。ワークスペースが削除済み → `"deleted_workspace"`
  - `TestActionStates`(表で): 閲覧のみの Store → `ApplyEnabled == false`・`KeepEnabled == false`・`Note == "閲覧のみの Store です"`。アイテム削除済み → `Visible == false`。いまのステータスが Cancel → `ApplyEnabled == false`・`KeepEnabled == true`・`Note == "すでに変更されています(いまは Cancel)"`。変える先が削除済み → `Note == "「Done」は削除されています"`。どれでもない → 両方有効・`Note == ""`。`Result == kept` の行 → `Result` が入る
  - `TestRowSender`: 自動化の通知は `Sender == ""`

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/inbox/`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: 実装する**

`Page` は (1) `List.List` で表示中の Store、(2) Store ごとに `Purged().List()` から item・project・workspace の `Target` を集めて `Book.Prune`、(3) `Book.List(Limit: 301)`、(4) 行ごとに Store ごとの読み(`Stores.Read`)で対象とボタンを解決、(5) `Counts`。対象の読みは Store ごとに 1 回の `Read` にまとめる。ステータス名は通知のワークスペースの設定から引く(削除済みの値の名前も引ける読み方。なければ空)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/inbox/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/inbox
git commit -m "feat(inbox): 受信箱の一覧・未読数・対象の解決を足す(C4a)"
```

---

### Task 4: 既読・すべて既読・止まった通知のボタン・出来事

**Files:**
- Create: `internal/app/inbox/actions.go`(`SetRead`・`ReadAll`・`Keep`・`Apply`)
- Create: `internal/app/inbox/notifier.go`(`Notifier`)
- Test: `internal/app/inbox/actions_test.go`
- Modify: `internal/app/port/events.go`(`EventInbox`)
- Modify: `internal/app/automations/apply.go`(`from` の引数と重なりの断り)
- Test: `internal/app/automations/apply_test.go`(既存)

**Interfaces:**
- Consumes: Task 3 の `Service`、`automations.Service.ApplyIgnoringWorkflow`
- Produces:
  - `port.EventInbox EventKind = "inbox"`
  - `func (s automations.Service) ApplyIgnoringWorkflow(ctx, store port.StoreID, id item.ID, rule, from, status change.ID) error` — 保存の中で、いまのステータスが `from` でなければ `refusal.New("{ID} は先に変更されたため、適用しませんでした")`({ID} はいまのワークスペースの接頭辞の ID)
  - `inbox.Service` に `Apply func(ctx context.Context, store port.StoreID, id item.ID, rule, from, status change.ID) error` のフィールド(cmd が `automations.Service.ApplyIgnoringWorkflow` を渡す。app の機能どうしを import しないため)
  - `func (s Service) SetRead(ctx, id port.NoticeID, read bool) error`
  - `func (s Service) ReadAll(ctx, tab Tab) error` — 表示中の Store の、そのタブの種類だけ
  - `func (s Service) Keep(ctx, id port.NoticeID) error`
  - `func (s Service) Apply(ctx, id port.NoticeID) error` — 止まった通知でなければ `port.ErrInvalidOperation`。結果がすでにあれば Task 1 の断り(保存しない)。閲覧のみの Store は `refusal.New("閲覧のみの Store です")`。`Apply` が成功したら `Book.Resolve(id, applied)`
  - `type Notifier struct { Inbox port.Inbox; Events port.Events }`、`Add` は足してから `Event{Kind: EventInbox, Store: n.Store}` を出す
  - 4 つの操作は、成功したら `EventInbox` を出す

- [ ] **Step 1: 失敗するテストを書く**
  - automations: `TestApplyRefusedWhenStatusChanged`: アイテムが InProgress、`from == Todo` → refusal の文が「HH-1 は先に変更されたため、適用しませんでした」、ログに行が増えない。`from == InProgress` → Done になり、アクティビティの文は「「{ルール名}」の変更を制約を無視して適用」
  - inbox: `TestSetReadPublishes`(既読になり、`EventInbox` が 1 回)・`TestReadAllOnlyTab`(automation のタブだけ既読)・`TestReadAllSkipsHiddenStore`・`TestKeep`(既読・`kept`、保存なし)・`TestApply`(既読・`applied`、ステータスが変わる)・`TestApplyAfterKeepRefused`(先に Keep → Apply は断られ、ステータスは変わらない)・`TestApplyRefusedWhenStatusChanged`(Apply の偽物が refusal を返す → `Book` の行は未処理のまま)・`TestApplyReadOnlyStore`・`TestApplyNotStopped`(自動化の通知に Apply → `ErrInvalidOperation`)・`TestNotifierPublishes`

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/inbox/ ./internal/app/automations/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/inbox internal/app/port/events.go internal/app/automations
git commit -m "feat(inbox): 既読・すべて既読と、止まった通知の 2 つのボタンを足す(C4a)"
```

---

### Task 5: 受信箱の API・出来事・組み立て

**Files:**
- Create: `internal/adapter/httpapi/inbox/inbox.go`、`internal/adapter/httpapi/inbox/dto.go`
- Test: `internal/adapter/httpapi/inbox/inbox_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps.Inbox` を `port.InboxBook` に替え、`inbox.Register` を足し、items・workspaces・githublink に渡す `port.Inbox` は `appinbox.Notifier` で包む)
- Modify: `internal/adapter/httpapi/automations/automations.go`(古い apply の経路と、その本文の型・テストを消す)
- Modify: `internal/adapter/httpapi/events/events.go`(`writeFrame` と Description に `inbox`)
- Modify: `cmd/hedgehug/wire.go`(`db.Inbox(clock)`、`noInbox` を消す)、`cmd/hedgehug/wire_e2e.go`(`e2eReads` の db の `Inbox`)、`cmd/hedgehug/links.go`(Notifier を渡す)
- Modify: `api/openapi.yaml`(`make generate`)

**Interfaces:**
- Consumes: Task 3・4 の `inbox.Service`
- Produces(HTTP):
  - `GET /api/inbox?tab={tab}&before_at={RFC3339Nano}&before_id={id}`(OperationID `list-inbox`)→ `InboxPage { rows: InboxRow[]; more: bool; unread: Record<tab, number> }`。tab が 7 つ以外なら 400
  - `GET /api/inbox/unread`(`inbox-unread`)→ `InboxUnread { counts: Record<tab, number> }`
  - `PUT /api/inbox/{notice}/read`(`set-inbox-read`)本文 `{ read: bool }` → 204
  - `POST /api/inbox/read-all`(`read-all-inbox`)本文 `{ tab }` → 204
  - `POST /api/inbox/{notice}/apply`(`apply-inbox-notice`)→ 204。断りは 422 の文
  - `POST /api/inbox/{notice}/keep`(`keep-inbox-notice`)→ 204
  - `InboxRow`: `id`・`kind`・`at`・`read`・`text`・`reason`・`sender`・`store { id, name, color, readOnly }`・`target { kind, item, itemKind, display, title, workspace, workspaceName, section }`・`action?`(`status`・`statusName`・`visible`・`applyEnabled`・`keepEnabled`・`note`・`result`)
  - 出来事の種類 `inbox`

- [ ] **Step 1: 失敗するテストを書く(`humatest`)**: `TestListInbox`(行の形と `unread`)・`TestListInboxBadTab`(400)・`TestSetRead`・`TestReadAll`・`TestApplyRefusal`(422 と文)・`TestKeep`。events: 既存のテストに `inbox` の枠が `event: inbox` で書かれる確かめを足す
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/adapter/httpapi/...` Expected: FAIL
- [ ] **Step 3: 実装し、`make generate` で `api/openapi.yaml` を書き直す**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./... && golangci-lint run ./... && golangci-lint run --build-tags e2e ./...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add -A internal/adapter/httpapi cmd/hedgehug api/openapi.yaml
git commit -m "feat(inbox): 受信箱の API と出来事を足し、state.db の受信箱を組み立てる(C4a)"
```

---

### Task 6: 画面の受信箱の状態(core・api)

**Files:**
- Create: `web/packages/core/src/model/inbox.ts`(型)、`web/packages/core/src/port/inbox.ts`(`InboxPort`)
- Create: `web/packages/core/src/features/inbox/inbox.ts`・`inbox.test.ts`・`inbox.test-util.ts`
- Create: `web/packages/core/src/inboxIndex.ts`(エクスポート。`index.ts` から再公開)
- Create: `web/packages/api/src/inbox.ts`・`inbox.test.ts`(`createInboxPort`)
- Modify: `web/packages/core/src/model/sync.ts:36`(`SyncEventKind` に `"inbox"`)、`web/packages/api/src/sync.ts:16-17`(`kindOf`)、`web/packages/core/src/features/sync/reflection.ts:32`(`order`)
- Modify: `web/packages/api/src/generated.ts`(`pnpm gen`)

**Interfaces:**
- Consumes: Task 5 の API
- Produces:
  - `InboxPort { list(tab, before?): Promise<InboxPage>; unread(): Promise<Record<InboxTab, number>>; setRead(id, read); readAll(tab); apply(id); keep(id) }`
  - `createInbox(deps: { port: InboxPort; toasts: Toasts }) => { state: Readable<InboxState>; unread: Readable<Record<InboxTab, number>>; open(): Promise<void>; select(tab): Promise<void>; more(): Promise<void>; toggle(row): Promise<void>; markRead(row): Promise<void>; readAll(): Promise<void>; apply(row): Promise<void>; keep(row): Promise<void>; reload(): Promise<void>; reloadUnread(): Promise<void> }`
  - `InboxState { tab: InboxTab; rows: InboxRow[]; more: boolean; loading: boolean }`
  - `inboxTabs: { id: InboxTab; label: string }[]`(Global Constraints の順と名前)、`pillOf(kind): string`、`emptyOf(tab): { title: string; note?: string }`

- [ ] **Step 1: 失敗するテストを書く(vitest)**
  - `open` で「すべて」の 300 件と `more`。`more()` で末尾に足す(最後の行の `at`・`id` を渡す)
  - `select("automation")` で一覧を読み直す
  - `toggle` で既読と未読が入れ替わり、`unread` も変わる(読み直しの結果で)
  - `readAll` は表示中のタブだけを渡す。`unread[tab] == 0` のとき、画面がボタンを無効にするための値が出る(`canReadAll`)
  - `apply` が成功 → トースト「適用しました」、行を読み直す。422 → エラーのトースト(断りの文そのまま)と、行の読み直し
  - `keep` → トーストなし、行の読み直し
  - `pillOf("automation_stopped") === "自動化"`、`emptyOf("overwrite")` が Global Constraints の文
  - api: `createInboxPort` が `before_at`・`before_id` を付ける。422 を `Refused` に変える
- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm -r test` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `make web-check` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add web/packages
git commit -m "feat(web): 受信箱の型・口・状態を足す(C4a)"
```

---

### Task 7: 受信箱の画面とサイドバー

**Files:**
- Modify: `web/packages/core/src/features/router/route.ts`(`{ kind: "inbox" }`、`/inbox`)と `route.test.ts`
- Create: `web/app/src/inbox.ts`(組み立て。`reloads` に `inbox` の出来事で `reloadUnread`・開いていれば `reload` を登録)
- Create: `web/app/src/screens/inbox/InboxScreen.svelte`(見出し・補足・「すべて既読にする」・`Tabs`・行・「さらに読み込む」・空のとき)
- Create: `web/app/src/screens/inbox/InboxRow.svelte`(点・ピル・対象・相対時間・差出人と本文)
- Create: `web/app/src/screens/inbox/InboxAction.svelte`(止まった通知の理由・2 つのボタン・結果のピル)
- Modify: `web/app/src/shell/Screen.svelte`(`inbox` を振り分ける)、`web/app/src/shell/Sidebar.svelte`(「受信箱」の項目と「すべて」の未読数)
- Modify: ワークスペースの設定の画面(`workspaceSettings.open` を呼ぶところ)と `web/packages/core/src/features/workspaces/settings.ts`(C4a-15)

**Interfaces:**
- Consumes: Task 6 の `createInbox`、`createLinks`、`relativeTime`、`Avatar`、`KindIcon`、`Markdown`(`tokenize`)、`Tabs`、`detail.open`、`routeOfWorkspace`
- Produces: アドレス `/inbox`。サイドバーの「受信箱」は `aria-current` で現在を示し、未読数は 0 のとき出さない

- [ ] **Step 1: 失敗するテストを書く**: `route.test.ts` に `parseRoute("/inbox")`・`formatRoute({kind:"inbox"})`。`settings.test.ts`(既存)に「`open` のあと区画 `automation` を渡すと tab が automation」
- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm -r test` Expected: FAIL
- [ ] **Step 3: 画面を実装する**
  - 行のクリック: 対象が `item` → 既読にして `detail.open(store, item)`。`moved` → 既読にして `/s/{Store 名}/i/{Display}` へ移る。`workspace` → 既読にして設定の「自動化」(C4a-15)。`deleted_*` はリンクにしない。リンクは `links.click(e, route, action)` で、Ctrl+クリック・Shift+クリック・中クリックは新しいタブで開き、既読にしない(01 `:185`)
  - 左端の点: 未読は目立つ色、押すと `toggle`。押せることが分かる名前(`aria-label` 「既読にする」「未読にする」)
  - 差出人: `sender` が空なら名前「hedgehug」。メンバーにいなければ「不明なユーザー」と中身のないアバター
  - 本文は `Markdown`(`tokenize`)。止まった通知は本文の下に `reason` と `InboxAction`
  - 「すべて既読にする」は見出しの右。いまのタブの未読が 0 なら無効
- [ ] **Step 4: 通ることを確かめる** — Run: `make web-check` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 受信箱の画面とサイドバーの未読数を足す(C4a)"
```

---

### Task 8: e2e

**Files:**
- Create: `web/e2e/inbox.spec.ts`
- Modify: `web/e2e/helpers.ts`(要るなら、ルールと制約を API で作る手助け)

- [ ] **Step 1: spec を書く**(API で、新しいワークスペースに「→ Done」のルールと、InProgress → Done を禁じる制約を作り、アイテムを InProgress にしてルールを止める)
  - `止まった通知が受信箱に出て、制約を無視して適用できる`: サイドバーの「受信箱」に未読数 1 → 開くと「自動化」のピル、本文「「{ルール名}」が制約で止まりました」と理由。「制約を無視して Done にする」を押す → トースト「適用しました」、結果のピル「適用しました」、未読数が消える。アイテムのステータスが Done
  - `変更しないとこのままにしました`: 「変更しない」 → ピル「このままにしました」、ステータスは変わらない
  - `先にステータスを変えるとボタンが無効`: 通知のあとに API で Cancel にする → 受信箱で「制約を無視して…」が無効で「すでに変更されています(いまは Cancel)」
  - `既読の切り替えとすべて既読`: 点を押すと未読数が減り、もう一度で戻る。「自動化」のタブで「すべて既読にする」 → 未読数が消え、ボタンが無効
  - `別のタブに未読数が届く`: 2 つ目のページで受信箱を開いたまま、1 つ目で既読にする → 2 つ目の未読数が消える
  - `空のとき`: 「上書き」のタブで「お知らせはありません」と補足の文
- [ ] **Step 2: 流す** — Run: `make e2e-bin && cd web && pnpm exec playwright test e2e/inbox.spec.ts` Expected: 3 つの幅で PASS
- [ ] **Step 3: コミット**

```bash
git add web/e2e
git commit -m "test(e2e): 受信箱の自動化の通知と操作を確かめる(C4a)"
```

---

### Task 9: パレットの「受信箱を開く」・計画と仕様の記録・全体の検証

**Files:**
- Modify(A5c が develop に入っていれば): コマンドパレットのコマンドの一覧に「受信箱を開く」(`/inbox` へ移る)と、そのテスト
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C4 の行を C4a〜C4d に分ける)
- Modify(A5c が develop に入っていなければ): `docs/superpowers/plans/2026-11-14-hedgehug-track-a5c.md` の「受信箱を開く」の行(ブランチ feat/track-a5c にあるなら、その計画を書いたセッションに渡すため、トラック C の計画の申し送りに書く)

- [ ] **Step 1: `git log origin/develop` で A5c が入っているか確かめ、入っていればパレットに「受信箱を開く」を足してテストを通す。入っていなければ、トラック C の計画に「A5c が C4a のあとに入るときに足す」と書く**
- [ ] **Step 2: トラック C の計画の C4 の行を、次の 4 行に分ける**
  - C4a(この計画): 範囲は C4a-1。完成の条件は「自動化の止まった通知が受信箱に出て、2 つのボタンと既読が動く」
  - C4b: 受け取った保存からの通知(割り当て・メンション・ウォッチ・同期の修復・受け取ったステータス削除からの自動化)、判定した保存の記録。同期の修復の文: 修復 (2) の親がワークスペースのときの「ワークスペース「{名前}」」(B5b)、(5) のワークスペースの移動の「。合う値がなく空になった項目: A・B」(C3b-7)、別の Store へ移動したものと重なったときの通知(A5a)、削除済みのワークスペース・スプリント・リリースへの保存の通知(R11・R12)
  - C4c: 上書きの通知と「自分の値に戻す」(`InboxBook.Resolve` と行の `Action` の形を、止まった通知と合わせて使う)
  - C4d: アクティビティ(画面・絞り込み・同期の行)、旧 ID のリンク(`locate-item`・`locate-project`)、メンバーの `join` の行から「メンバー {ログイン名} を追加」(A5a-11)
  - G への申し送りの表に「Store をこのマシンから削除したら `InboxBook.ForgetStore` を呼ぶ(C4a-7)。本物の `StoreList`・`Identity` が入ると、本物のビルドの受信箱が出る(C4a-14)」を足す
- [ ] **Step 3: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./... && make web-check` Expected: すべて PASS
- [ ] **Step 4: コミット**

```bash
git add -A docs web
git commit -m "docs: トラック C の C4 を C4a〜C4d に分け、C4a の申し送りを書く"
```
