# hedgehug トラック C4b1(受け取った保存の判定と、人への通知)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期で受け取った保存を 1 回だけ判定し、あなた宛ての「割り当て」「メンション」「ウォッチ」と、受け取ったステータスの削除からの「自動化」の通知を受信箱に出す。プロジェクトを指す通知(オーナー・プロジェクトの説明へのメンション)を受信箱で開けるようにする。

**Architecture:** 判定した保存の記録は `state.db` の新しい表(`judged/001`)で、口は `port.JudgedSaves`。受け取った保存を読むのは git の版だけの仕事なので、`infra/gitstore` の下に新しいパッケージ `gitstore/saves` を作り、`Replicator` が同期の最後(送ったあと。閲覧のみは受け取ったあと)に、まだ判定していない保存を `port.ReceivedSave` の並びにして、差し込み口 `port.JudgeFunc` に渡し、そのあと自分の先端から辿れる保存をすべて判定済みにする。判定の中身は新しい usecase `internal/app/noticing`(割り当て・メンション・ウォッチ・自動化)。判定の「前」の状態は、いまのログから、判定する保存が足した行を除いて作る(ログは行の和なので、木を読み直さない)。受信箱は `inbox/002` でメンションの書き換えとプロジェクトの列を足し、画面にプロジェクトの対象を足す。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite、huma v2)、TypeScript・Svelte(web/packages/core・api、web/app)、vitest。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:25`(非表示の Store の通知)、`:352-418`(「通知」の表・作らない場合・判定する単位・割り当て・メンション・ウォッチ・自動化)、`:442-455`(行の表示。プロジェクトを指すとき)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md` `:87`・`:182`・`:205`(担当者・作成・複製の通知)、`:374`・`:384`(タイトル・説明)、`:405-410`(メンション)、`:480`・`:487`(コメント・ウォッチ)、`:535`・`:560-572`(プロジェクトのオーナー・説明・コメント)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md` `:368`(ステータスの削除でルールが変わったときの通知)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:225-232`(書き換えのあとの照合)、`:282-285`(state.db に持つもの・Store ID の行を消す)、`:322`(`actor: github` の行からはウォッチ・割り当てを作らない)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c4a.md`(受信箱の口・表・画面。この計画はその上に足す)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の決定 14(Store を追加したときより前の変更から作らない)と C4b の行

## Global Constraints

土台・トラック C・C4a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個(`internal/archtest/size.go`)。`internal/infra/gitstore`・`internal/app/port`・`internal/app/apptest` は 20 個で満杯なので、ファイルを足さない。gitstore の新しい処理は下のパッケージ `internal/infra/gitstore/saves` に置き、gitstore 本体には呼び出しだけを足す。`gitstore/replicator.go` は 296 行なので、足す分は `replicator_mirror.go`(241 行)に置く。web は .svelte 200 行、.ts 300 行。
- 口は git の言葉を使わない(`port` の型の名前とコメントに「コミット」「ブランチ」を書かない。`SaveKey.Commit` は「保存の印」と説明する)。
- app の機能どうしは import しない。判定の差し込み口は `cmd` で組む(`rep.Repair = repairing.Run` と同じ形)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 通知の文言(06・02 のとおり):
  - 割り当て: 「あなたに割り当てました」。プロジェクトのオーナー: 「プロジェクト「{タイトル}」のオーナーになりました」
  - メンション: 書かれた本文全体(最新のもの)
  - ウォッチ: 「{プロパティ名}を {値} に変更」「タイトルを「{新しいタイトル}」に変更」「コメント: {先頭 40 字}」。値の書式は 01 `:154`(空は「なし」、チェックは「オン」「オフ」、複数は「, 」、削除された値は「(削除された値)」)
  - 自動化(ステータスの削除): 「自動化「{ルール名}」は、ステータス「{削除した値}」の削除で、{きっかけ または すること}のステータスを「{書き換え先}」に書き換えました」と、無効にしたときの 03 の文言(`automation.RewrittenText`・`automation.DisabledByDeleteText` をそのまま使う)
  - 受信箱の対象: 「削除されたプロジェクト」(決定 C4b1-12)
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・`make web-check`。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C4b1-1 | 範囲: 判定した保存の記録、受け取った保存を読む口、差し込み口、割り当て・プロジェクトのオーナー・メンション・ウォッチ・受け取ったステータスの削除からの自動化。受信箱のメンションの書き換えとプロジェクトの対象。同期の修復の通知(修復 (1)〜(5)・削除の取り消し・別の Store や完全な削除と重なって反映されなかったとき)は C4b2、上書きは C4c | ユーザーの決定(2026-11-14) |
| C4b1-2 | 「受け取った保存」は、同期で受け取った remote の先端から辿れて、判定済みでない保存(`Hedgehug-Op` を持つもの)。判定は 1 回の同期の最後に 1 回: 送る同期は送ったあと、閲覧のみは受け取ったあと、`SyncSave` は送ったあと。判定のあと、自分の先端から辿れる保存(合わせ・修復・自分の保存を含む)をすべて判定済みにする。判定済みは祖先について閉じているので、辿るのは判定済みに当たったところで止める | 06 `:375`(受け取った保存ごとに 1 回)。`SyncResult.Received` は始めたときの自分の先端から数えるので、合わせたあと送れずに失敗すると次の回で 0 になり使えない。自分の保存は、remote に届く前に判定済みになるので、自分の操作の通知(`notifyRules`)と重ならない |
| C4b1-3 | 判定済みの記録がない Store(clone した直後、この版にしてから初めての同期)は、同期を始めたときの自分の先端から辿れる保存を、通知を作らずに判定済みにしてから判定する | 06 `:372`(Store を追加したときに取得した、それ以前の変更からは作らない)。トラック C の決定 14 は G の追加の処理で覚えるとしたが、G がまだないので同期の側で始めの印を付ける。clone して数日後に初めて同期したときに、そのあいだの変更は通知になる |
| C4b1-4 | 判定済みの照合は、保存の印(`Commit`)が同じか、`Op` が空でなく作者・時刻・`Op` の組が同じとき。表は `judged/001`: `store`・`commit`・`author`・`at`(C4a の固定幅の UTC)・`op`。索引は `(store, commit)` と `(store, op, author, at)` | 保存形式 `:232`・`:282`(書き換えのあとは作者・時刻・`Hedgehug-Op` で照合する) |
| C4b1-5 | 判定の差し込み口が失敗したら、判定済みにせず、同期を失敗にする(このマシンの状態を読み書きできないときの扱い)。次の同期で同じ保存をもう一度判定する | `syncing.run` の「エラーは、このマシンの状態を読み書きできないときだけ返す」。判定済みを先に書くと通知が消えたままになる |
| C4b1-6 | 「判定の前」の状態は、いまのログ(合わせ・修復のあと)から、判定する保存が足した行を除いて作る(`item.FromLog`・`project.FromLog`・`comment.FromLog` に行を減らしたログを渡す)。「判定の後」はいまのログ | 06 `:381-382`(判定の前と後を比べる)。ログは行の和なので、前の木を読み直さなくてよい |
| C4b1-7 | 「自分の変更」は行の `by` で決める(大文字と小文字を区別しない)。項目の後の値を決めた行(`change.Current` の勝ち)の `by` があなたなら作らない。`actor` が `github` の行からは割り当て・ウォッチを作らない。`actor` が `sync`(同期の修復)の行からも割り当て・ウォッチを作らない(修復は「同期の修復」の通知で知らせ、06 の「他の人が変えた」に当たらない) | 06 `:370`・`:384`、保存形式 `:322`。自動化の保存はコミットの作者が bot で、操作した人は行の `by` にだけ残る |
| C4b1-8 | 差出人: 割り当て・ウォッチは、勝った行の `by`。その行の `actor` が自動化なら空(「hedgehug」)。メンションは本文を保存した行の `by`。自動化は空 | 06 `:390`(自動化のときは「hedgehug」)、`:382`(最後に変えた人)、`:418` |
| C4b1-9 | ウォッチの対象: 判定の後のアイテムの `Watchers` にあなたがいて、判定の前にもアイテムがあったもの(判定の中で作られたアイテムは、前の値がないので作らない)。項目はタイトルとワークスペースのプロパティ(`workspace.Settings` のプロパティ。組み込みのステータス・担当者・優先度などを含む)。スプリント・リリース・種類・場所・順位・説明・リンク・関連・作業時間は作らない。コメントは、判定の中で初めて現れたコメント 1 件ごとに 1 行(編集・削除は作らない) | 06 `:382`・`:410-414`、02 `:487`(プロパティとタイトルとコメントの投稿だけ)。作成したアイテムをウォッチするのは作った本人なので、自分の変更 |
| C4b1-10 | メンション: 本文の単位は、アイテムの説明・プロジェクトの説明・コメント 1 件。判定する保存を古い順に見て、その保存で本文が書かれ(説明は `desc` の行、コメントは本文のある `comment` の行)、本文に `@あなたのログイン名`(大文字と小文字を区別しない。`@` の前の文字は問わない。あなたが Store のメンバーにいるとき)があり、書いた行の `by` があなたでなければ、行を作るか書き換える。説明の本文は、その保存の時点の `description.md`(受け取った保存が持つ)。本文の鍵は `item:{ID}:desc`・`project:{ID}:desc`・`comment:{コメント ID}` | 06 `:374`・`:392-402`、02 `:405-406` |
| C4b1-11 | メンションの書き換え: 同じ Store・同じ本文の鍵の行があれば、本文・差出人・時刻(その保存の時刻)を書き換えて未読に戻す(結果は空のまま)。なければ足す(時刻は受信箱の時計。C4a-3)。`inbox/002` で列 `project`・`source`(本文の鍵)を足し、索引 `(store, source)` を足す | 06 `:399`(時刻を保存した時刻に、未読に戻す)、`:451`(作った時刻)。C4a-2 の「C4b が要る列は新しい手順で足す」 |
| C4b1-12 | プロジェクトを指す通知(オーナー・プロジェクトの説明とコメントへのメンション)は `Notice.Project` を持つ。受信箱の対象は、生きていれば `project`(プロジェクト名。クリックでプロジェクトの概要)、削除済みなら `deleted_project`「削除されたプロジェクト」(リンクなし)、Store にない(別の Store へ移った。完全な削除は `Prune` で消える)なら `moved_project`(作ったときの ID を出し、クリックで旧 ID のアドレス `/s/{Store 名}/p/{ID}` を開く)。仕様 06 `:455` に「プロジェクトが削除済みのときは「削除されたプロジェクト」と表示し、リンクにしない」を足す | 06 `:455` のアイテム・ワークスペースと同じ規則の当てはめ(06 はプロジェクトの削除済みの表示を書いていない)。旧 ID のアドレスの解決は A5b の `locate-project` |
| C4b1-13 | `Prune` は、`item`・`project` が targets にある行と、アイテムもプロジェクトもない行のうち `workspace` が targets にあるものを消す | C4a-6 と同じ理由(作ったときのワークスペースでアイテム・プロジェクトの通知を消さない) |
| C4b1-14 | ステータスの削除からの自動化: 判定する保存のうち、ワークスペースのログに `delete statuses/{ID}` の行を持つものごとに、同じ op の `set rules/{ルール ID}/trigger_status`・`set rules/{ルール ID}/value`・`set rules/{ルール ID}/enabled` の行から変わったルールを作り直し、`notifyRules` と同じ文で 1 ルール 1 件。きっかけとすることの両方なら「すること」。作るのは、Store が表示中で閲覧のみでなく、削除の行の `by` があなたでないとき | 06 `:418`(書き込める全員のマシン。閲覧のみ・非表示では作らない。削除した人のマシンは自分の操作で作る)。非表示のあいだに受け取った削除は判定済みになり、再表示しても作らない。作り直しは `automation.StatusDeleteDrafts` の Part の決め方と同じ |
| C4b1-15 | 「あなた」(`port.Identity.Me`)を決められない Store では、通知を作らずに判定済みにする | 06 は「あなた宛て」だけを作る。本物のビルドは G まで `noIdentity`(C4a-14 と同じく、本物のビルドでは G まで出ない) |
| C4b1-16 | 判定済みの記録の後始末: Store をこのマシンから削除したら `JudgedSaves.ForgetStore` を呼ぶ。呼ぶのは G(C4a-7 と同じ)。トラック C の計画の G への申し送りに書く | 保存形式 `:285`、06 `:375`(Store とともに消す) |
| C4b1-17 | e2e は足さない。通知は git の同期でしか生まれず、e2e のビルドは偽の Replicator でメモリ上のデータを使うため。2 台の場面は `cmd/hedgehug` の本物の gitstore と state.db のテストで確かめ、画面の対象の出し分けは vitest で確かめる | `wire_e2e.go` の `apptest.Replicator`。手元の検証は 1 回で足りる |
| C4b1-18 | 上書きとウォッチの両方に当たるときは上書きだけ(06 `:373`)。上書きは C4c が作るので、C4c がウォッチの判定に「上書きを作った項目は飛ばす」を足す。トラック C の計画の C4c の行に書く | 06 `:373`。使わない分岐を先に作らない |

## Review Focus

1. **同期が送れずに失敗したあと:** 受け取って合わせたあと送信が 3 回重なって失敗すると、判定はされず判定済みにもならない。次の同期で同じ保存から通知が 1 回だけ出る(Task 3 の `TestJudgeRetriedAfterFailedPush`)。
2. **自分の保存がのちに remote から戻ってくる:** このマシンでステータスを削除して同期した保存は、別のマシンが合わせて送り返してきても、自動化の通知を 2 件目にしない(Task 3 の `TestOwnSaveNeverJudged`、Task 7 の場面テスト)。
3. **閲覧のみ・非表示の Store:** 閲覧のみの Store は割り当て・メンション・ウォッチを作り、自動化は作らない。非表示の Store は通知を作るが(受信箱には出ない)、自動化は作らない。再表示しても自動化は出ない(Task 6 の `TestAutomationSkipsReadOnlyAndHidden`)。
4. **同じ判定の中で担当者が往復する:** 太郎がアイテムを花子にし、続けて次郎にした 2 つの保存をまとめて受け取ると、花子には割り当てを作らない(Task 5 の `TestAssignOnlyWhenAfterIsMe`)。
5. **メンションの書き換え:** 同じコメントを 2 回の同期で編集して届くと、行は 1 つのまま本文と時刻が変わり未読に戻る。本文からログイン名が消えた編集では、行は書き換わらない(Task 2 の `TestInboxMentionUpsert`、Task 5 の `TestMentionRewrite`)。

---

### Task 1: 判定した保存の記録(`judged/001`)

**Files:**
- Modify: `internal/app/port/syncstate.go`(`SaveKey`・`JudgedSaves`)
- Create: `internal/infra/statedb/judged.go`
- Test: `internal/infra/statedb/judged_test.go`
- Modify: `internal/app/apptest/syncstate.go`(偽物 `JudgedSaves`)
- Test: `internal/app/apptest/syncstate_test.go`(既存。なければ作る)

**Interfaces:**
- Produces(`port`):
  - `type SaveKey struct { Commit string /* 保存の印 */; Author string; At time.Time; Op change.ID /* 合わせ・最初の保存は空 */ }`
  - `type JudgedSaves interface {`
    - `Started(ctx, store StoreID) (bool, error)` — その Store の行が 1 つでもあるか
    - `Known(ctx, store StoreID, k SaveKey) (bool, error)` — C4b1-4 の照合
    - `Mark(ctx, store StoreID, keys []SaveKey) error` — 1 つの取引でまとめて書く。すでにあるものは重ねない
    - `ForgetStore(ctx, store StoreID) error`
  - `}`
- Produces(`statedb`): `func (d *DB) JudgedSaves() port.JudgedSaves`
- Produces(`apptest`): `type JudgedSaves` が同じ振る舞い。`RunJudgedSavesContract(t, func() port.JudgedSaves)` を `syncstate.go` の契約の手本(`RunSyncStateContract`)と同じ形で置き、statedb と偽物の両方のテストから呼ぶ

- [ ] **Step 1: 契約のテストを書く(`RunJudgedSavesContract`)**
  - `Started` は空の Store で false、`Mark` のあと true。別の Store は false のまま
  - `Known`: 同じ `Commit` で true。`Commit` が違っても `Op`・`Author`・`At` が同じなら true。`Op` が空なら `Commit` だけで照合し、`Author`・`At` が同じでも false
  - `Mark` を同じ鍵で 2 回呼んでもエラーにならない
  - `ForgetStore(s1)` で s1 だけ `Started` が false
  - statedb だけ: `Close`・`Open` のあとも残る
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/statedb/ ./internal/app/apptest/ -run Judged` Expected: FAIL(`JudgedSaves` がない)
- [ ] **Step 3: 実装する** — `register("judged/001", …)`(C4b1-4 の列と 2 つの索引)。時刻は `inboxTime` と同じ固定幅
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/statedb/ ./internal/app/apptest/` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/syncstate.go internal/infra/statedb/judged.go internal/infra/statedb/judged_test.go internal/app/apptest/syncstate.go internal/app/apptest/syncstate_test.go
git commit -m "feat(sync): 通知を判定した保存を state.db に覚える(C4b1)"
```

---

### Task 2: 受信箱のメンションの書き換えとプロジェクトの列(`inbox/002`)

**Files:**
- Modify: `internal/app/port/inbox.go`(`Notice.Project`・`Notice.Source`・`Notice.At`、`Inbox.Mention`、`Prune` の説明)
- Modify: `internal/infra/statedb/inbox.go`(`inbox/002`、`Mention`、`Prune`、読み出しの列)
- Test: `internal/infra/statedb/inbox_test.go`
- Modify: `internal/app/apptest/inbox.go`、Test: `internal/app/apptest/inbox_test.go`
- Modify: `internal/app/inbox/notifier.go`(`Mention` を通して `EventInbox` を出す)、Test: `internal/app/inbox/actions_test.go`(`TestNotifierPublishes` の隣)

**Interfaces:**
- Produces(`port`):
  - `Notice` に `Project project.ID`(プロジェクトを指す通知の対象。アイテムの通知では空)・`Source string`(メンションの本文の鍵。C4b1-10)・`At time.Time`(メンションの書き換えに使う保存の時刻。`Add` は使わない)
  - `Inbox` に `Mention(ctx, n Notice) error` — 同じ `Store`・`Source` の行があれば `text`・`sender`・`at = n.At` を書き換え `read = 0`。なければ `Add` と同じく足す(時刻は時計)。`n.Source` が空ならエラー
  - `Prune` は C4b1-13
- `inbox/002`: `ALTER TABLE inbox ADD COLUMN project TEXT NOT NULL DEFAULT ''`、`ADD COLUMN source TEXT NOT NULL DEFAULT ''`、`CREATE INDEX inbox_by_source ON inbox (store, source)`

- [ ] **Step 1: 失敗するテストを書く(statedb と apptest の両方)**
  - `TestInboxMentionUpsert`: 時計 10:00 で `Mention(Source: "comment:c1", Text: "@hanako 見て", Sender: "taro")` → 1 行、`At` は 10:00。既読にする。`Mention(同じ Source, Text: "@hanako 直した", Sender: "jiro", At: 09:30)` → 行は 1 つのまま、本文と差出人が変わり、`At` は 09:30、`Read == false`。別の Store の同じ `Source` は別の行
  - `TestInboxProjectColumns`: `Add(Project: p1, Display: "HH-P1")` → `List` で `Project`・`Display` が戻る
  - `TestInboxPruneProject`: `Prune(s1, [p1])` で `Project == p1` の行が消え、`Workspace == p1` と同じ値でもアイテムの行は残る
  - `TestNotifierMentionPublishes`: `Notifier.Mention` で `EventInbox` が 1 回
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/statedb/ ./internal/app/apptest/ ./internal/app/inbox/` Expected: FAIL
- [ ] **Step 3: 実装する**(`port.Inbox` を満たすほかの型 — `apptest.Inbox`・`statedb` の受信箱・`inbox.Notifier` — に `Mention` を足す。`go build ./...` で漏れがないことを確かめる)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/... && go build ./...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/inbox.go internal/infra/statedb internal/app/apptest/inbox.go internal/app/apptest/inbox_test.go internal/app/inbox
git commit -m "feat(inbox): メンションの行の書き換えと、プロジェクトを指す列を足す(C4b1)"
```

---

### Task 3: 受け取った保存を読み、判定の差し込み口を呼ぶ(`gitstore/saves`)

**Files:**
- Modify: `internal/app/port/replicator.go`(`ReceivedSave`・`EntityRows`・`JudgeFunc`)
- Create: `internal/infra/gitstore/saves/saves.go`(辿る・判定済みの印・保存ごとの足した行)
- Test: `internal/infra/gitstore/saves/saves_test.go`
- Modify: `internal/infra/gitstore/replicator_mirror.go`(`syncRun.judge`)、`internal/infra/gitstore/replicator.go`(`Replicator` の 2 つのフィールドと、`once`・`saveOnce` からの呼び出しだけ。300 行を超えるなら `result` を `replicator_mirror.go` へ移す)
- Test: `internal/infra/gitstore/sync_judge_test.go`(テストはファイル数に数えない)

**Interfaces:**
- Consumes: Task 1 の `port.JudgedSaves`・`port.SaveKey`
- Produces(`port`):
  - `type EntityRows struct { Ref change.Ref; Rows []change.Record /* その保存が足した行 */; Description *string /* その保存に desc の行があれば、保存の時点の説明 */ }`
  - `type ReceivedSave struct { Key SaveKey; Entities []EntityRows }`
  - `type JudgeFunc func(ctx context.Context, store StoreID, saves []ReceivedSave, read func(fn func(r Reader) error) error) error` — saves は古い順(親が先。同じ深さは時刻、印の順)。read は同期のロックの中で Store のいまの状態を読む(`repairWriter.Read` と同じ)
- Produces(`gitstore`): `Replicator` のフィールド `Judge port.JudgeFunc` と `Judged port.JudgedSaves`。どちらかが nil なら判定しない
- Produces(`saves`):
  - `func Unjudged(repo *git.Repository, from plumbing.Hash, known func(port.SaveKey) (bool, error)) ([]port.ReceivedSave, error)` — from から親を辿り、判定済みに当たったら止める。`Hedgehug-Op` のない保存(合わせ・最初の保存)は返さない
  - `func Reachable(repo *git.Repository, from plumbing.Hash, known func(port.SaveKey) (bool, error)) ([]port.SaveKey, error)` — 判定済みでないものすべて(合わせ・最初の保存を含む)
  - `func KeyOf(c *object.Commit) port.SaveKey`(作者はコミットの作者名、時刻は作者の時刻、`Op` はトレーラー)
  - 足した行は、保存の木と最初の親の木の `log.jsonl` を比べ、親にない `id` の行。エンティティの `Ref` はフォルダから(`kindOfDir` と同じ規則。`saves` から gitstore は import できないので、規則を `saves` に置いて gitstore から使うか、gitstore が `func(path string) (change.Ref, bool)` を渡す)
- `syncRun.judge(ctx, theirs plumbing.Hash) error` の手順(C4b1-2・C4b1-3・C4b1-5):
  1. `Judged.Started` が false なら、`Reachable(s.start)` を `Mark`(通知なし)
  2. `Unjudged(theirs)` を `Judge` に渡す(空なら呼ばない)。エラーなら返す(判定済みにしない)
  3. `Reachable(HEAD)` を `Mark`

- [ ] **Step 1: 失敗するテストを書く**
  - saves: `TestUnjudgedStopsAtKnown`(3 つの保存のうち 1 つ目を判定済みにすると 2 件、古い順)・`TestUnjudgedSkipsMerge`・`TestRowsAddedBySave`(アイテムの `set assignee` を書いた保存の `Entities` がそのアイテムの `Ref` と 1 行)・`TestDescriptionAtSave`(説明を 2 回変えた 2 つの保存の `Description` がそれぞれの時点の本文)・`TestKnownByOpAfterRewrite`(作者・時刻・op が同じ別の印の保存は判定済み)
  - gitstore(`sync_helpers_test.go` の 2 台): `TestJudgeReceivesOthersSave`(B の保存を A が同期 → `Judge` が B の保存 1 件で呼ばれる。もう一度同期しても呼ばれない)・`TestJudgeBaselineOnFirstSync`(記録のない A に、clone より前の保存は渡らず、clone のあとの B の保存だけが渡る)・`TestOwnSaveNeverJudged`(A の保存を送り、B が合わせて送り返したあと、A の同期で A の保存は渡らない)・`TestJudgeRetriedAfterFailedPush`(送信の失敗で Sync がエラー → 判定済みにならず、次の成功した同期で同じ保存が渡る)・`TestJudgeErrorFailsSync`(Judge がエラー → Sync がエラー、次の同期でもう一度渡る)・`TestJudgeReceiveOnly`(閲覧のみでも渡る)・`TestJudgeOnSyncSave`(`SyncSave` で受け取った保存も渡る)
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/...` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/... && golangci-lint run ./internal/infra/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/replicator.go internal/infra/gitstore
git commit -m "feat(sync): 受け取った保存を一度だけ判定の差し込み口へ渡す(C4b1)"
```

---

### Task 4: 通知の材料の関数(domain)

**Files:**
- Create: `internal/domain/mention/mention.go`、Test: `internal/domain/mention/mention_test.go`
- Modify: `internal/domain/automation/follow.go`(`FollowedFromRows`)、Test: `internal/domain/automation/follow_test.go`
- Modify: `internal/domain/history/item.go` か新しいファイル `internal/domain/history/watch.go`(`WatchText`)、Test: `internal/domain/history/watch_test.go`

**Interfaces:**
- Produces:
  - `func mention.Has(body, login string) bool` — `@` のあとにログイン名が続き、その直後がログイン名に使える文字(`member.ValidLogin` の文字)でない。大文字と小文字を区別しない。`@` の前の文字は問わない
  - `func automation.FollowedFromRows(rows []change.Record, rules []Rule) (deleted change.ID, changes []RuleChange)` — rows は 1 つの保存がワークスペースのログに足した行。`delete statuses/{ID}` がなければ deleted は空。`RuleChange.Rule` は rules(判定の後)の同じ ID のもの。Part は C4b1-14
  - `func history.WatchText(p workspace.Property, v json.RawMessage, c Context) string` — 「{プロパティ名}を {値} に変更」。値は履歴と同じ `vocab.display`
  - `func history.CommentHead(body string) string` — 「コメント: {先頭 40 字}」(文字で 40。改行は空白 1 つにする)

- [ ] **Step 1: 失敗するテストを書く**
  - mention(表で): `"@hanako 見て"`・`"cc:@Hanako"`・`"mail@hanako"` は true。`"@hanako2"`・`"@hana"`・`"hanako"` は false(login `hanako`)
  - automation: `TestFollowedFromRowsRewritten`(`delete statuses/s1` と `set rules/r1/trigger_status` → `deleted == s1`、Part「きっかけ」、Disabled false)・`TestFollowedFromRowsDisabled`(`enabled` の行もある → Disabled true)・`TestFollowedFromRowsBoth`(両方 → 「すること」)・`TestFollowedFromRowsNoDelete`(空)
  - history: 担当者 `hanako` → 「担当者を hanako に変更」(表示名があればその名前。履歴の `Names` と同じ)、空 → 「なし」、チェック → 「オン」、複数 → 「, 」、削除された値 → 「(削除された値)」。`CommentHead` が 41 文字目で切る
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/...` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/domain
git commit -m "feat(domain): メンション・ステータス削除のルール・ウォッチの文の材料を足す(C4b1)"
```

---

### Task 5: 割り当て・オーナー・メンションの判定(`internal/app/noticing`)

**Files:**
- Create: `internal/app/noticing/service.go`(`Service`・`Judge`・前と後の組み立て)
- Create: `internal/app/noticing/assign.go`、`internal/app/noticing/mention.go`
- Test: `internal/app/noticing/assign_test.go`、`internal/app/noticing/mention_test.go`、`internal/app/noticing/fixture_test.go`

**Interfaces:**
- Consumes: Task 2 の `Notice.Project`・`Source`・`At`・`Inbox.Mention`、Task 3 の `port.ReceivedSave`・`port.JudgeFunc`、Task 4 の `mention.Has`
- Produces:
  - `type Service struct { Inbox port.Inbox; Identity port.Identity; List port.StoreList }`
  - `func (s Service) Judge(ctx context.Context, store port.StoreID, saves []port.ReceivedSave, read func(fn func(r port.Reader) error) error) error` — `port.JudgeFunc` を満たす。`Identity.Me` がエラーなら何もしない(C4b1-15)。1 回の `read` の中で前と後を作り、通知はそのあと書く
  - 内部の型 `batch`: 判定する行の ID の集合、`before(ref) []change.Record`(いまのログから集合の行を除く)、`after(ref)`
- テストは `apptest.Stores`(`apptest.Sample`)に、判定する保存の行をふつうの保存で書き、その行を `ReceivedSave` にして `Judge` を呼ぶ(偽物の `read` は `Stores.Read`)

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`)
  - `TestAssignToMe`: taro がアイテムの担当者を hanako にした保存 → `assign`・本文「あなたに割り当てました」・`Sender == "taro"`・`Item`・`Display == "HH-1"`・`Workspace`
  - `TestAssignOnlyWhenAfterIsMe`: hanako にした保存と jiro にした保存を 1 回で → 作らない。前から hanako のアイテムにタイトルだけ変えた保存 → 作らない
  - `TestAssignByOwnChange`: by が `Hanako`(大文字) → 作らない
  - `TestAssignByAutomation`: 自動化の行(`actor: automation:r1`、by taro)で hanako → 作り、`Sender == ""`。by が hanako の自動化 → 作らない
  - `TestAssignSkipsGithubAndSync`: `actor: github`・`sync` の行 → 作らない
  - `TestAssignOnCreate`: taro が担当者 hanako のアイテムを作成した保存 → 作る(作成・複製・Step・テンプレートは `create` の行の値)
  - `TestOwnerToMe`: プロジェクトのオーナーを hanako → 本文「プロジェクト「{タイトル}」のオーナーになりました」・`Project`・`Display == "HH-P1"`・`Item == ""`
  - `TestMentionInComment`: taro のコメント「@hanako 見てください」 → `mention`・本文はコメント全体・`Source == "comment:{ID}"`・`Sender == "taro"`・`Item`
  - `TestMentionInDescription`: 説明に `@hanako`(`Description` がその本文) → `Source == "item:{ID}:desc"`。プロジェクトの説明 → `Project`・`Source == "project:{ID}:desc"`
  - `TestMentionRewrite`: 同じコメントの編集を 2 回の `Judge` で → 偽物の受信箱の行が 1 つ、本文が新しい方、`At` が 2 回目の保存の時刻、未読。2 回目の本文に `@hanako` がない → 書き換わらない
  - `TestMentionOwnOrNonMember`: hanako 自身が書いた本文、hanako がメンバーにいない Store → 作らない
  - `TestNoIdentity`: `Identity` がエラー → 何も作らず nil
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/noticing/` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 受け取った保存から割り当て・オーナー・メンションの通知を作る(C4b1)"
```

---

### Task 6: ウォッチと、ステータスの削除からの自動化の判定

**Files:**
- Create: `internal/app/noticing/watch.go`、`internal/app/noticing/automation.go`
- Test: `internal/app/noticing/watch_test.go`、`internal/app/noticing/automation_test.go`

**Interfaces:**
- Consumes: Task 4 の `history.WatchText`・`history.CommentHead`・`automation.FollowedFromRows`、Task 5 の `batch`
- Produces: `Judge` がウォッチ(C4b1-9)と自動化(C4b1-14)も作る。ウォッチの通知は `Kind: watch`・`Item`・`Display`・`Sender`(C4b1-8)

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`。hanako がウォッチ中のアイテム)
  - `TestWatchStatusChanged`: taro が Todo → Done → 「ステータスを Done に変更」・`Sender == "taro"`
  - `TestWatchSameValue`: Todo → Done → Todo の 2 つの保存を 1 回で → 作らない
  - `TestWatchTitle`: 「タイトルを「新しい名前」に変更」
  - `TestWatchTwoFields`: ステータスと優先度を 1 つの保存で → 2 行
  - `TestWatchComment`: 2 件のコメントの投稿 → 2 行「コメント: …」。コメントの編集・削除 → 作らない
  - `TestWatchSkips`: 説明・関連・作業時間・スプリント・移動・種類の変更、ウォッチしていないアイテム、hanako の変更、`github`・`sync` の行、判定の中で作られたアイテム → 作らない
  - `TestWatchByAutomation`: 自動化の行 → 作り、`Sender == ""`
  - `TestAutomationFromStatusDelete`: taro がステータス Review を削除し、ルール「レビューへ」のきっかけを InProgress に書き換えた保存 → `automation`・本文が `automation.RewrittenText("レビューへ", "Review", "きっかけ", "InProgress")`・`Rule`・`Workspace`・`Item == ""`。無効にした → `DisabledByDeleteText`。ルールが変わらない削除 → 作らない
  - `TestAutomationSkipsReadOnlyAndHidden`: `StoreList` に閲覧のみの印 → 自動化は作らず、割り当ては作る。`StoreList` にない(非表示) → 自動化は作らず、割り当ては作る
  - `TestAutomationOwnDelete`: 削除の行の by が hanako → 作らない
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): ウォッチと、受け取ったステータスの削除からの自動化の通知を作る(C4b1)"
```

---

### Task 7: 組み立てと 2 台の場面テスト

**Files:**
- Modify: `cmd/hedgehug/wire.go`(`rep.Judge`・`rep.Judged = db.JudgedSaves()`。`noticing.Service` の `Inbox` は C4a の `appinbox.Notifier`)
- Modify: `cmd/hedgehug/repair_helpers_test.go`(マシンごとのログイン名で書く `writeAs`、`machine` に `state.db` の受信箱と判定の記録を持たせる手助け)
- Create: `cmd/hedgehug/notice_scenario_test.go`

**Interfaces:**
- Consumes: Task 1〜6
- `wire_e2e.go` は偽の Replicator なので変えない(C4b1-17)

- [ ] **Step 1: 場面テストを書く**(A = taro、B = hanako。どちらも本物の gitstore と、テストごとの state.db)
  - `TestNoticeScenarioAssignOnce`: A がアイテムを作って担当者を hanako にし同期 → B が同期すると、B の受信箱の「割り当て」に 1 行。B がもう一度同期しても 1 行
  - `TestNoticeScenarioMention`: A がコメント「@hanako 見て」を投稿して同期 → B に `mention` が 1 行。A がコメントを「@hanako 直した」に編集して同期 → B の行は 1 つのまま本文が変わり未読
  - `TestNoticeScenarioWatch`: B がウォッチ中のアイテムのステータスを A が変えて同期 → B に「ステータスを Done に変更」
  - `TestNoticeScenarioStatusDelete`: A がルールのきっかけにしたステータスを削除して同期 → B に自動化の 1 行。A の受信箱は A の操作の 1 行だけ。B が何か保存して同期し、A がもう一度同期しても、A の自動化の通知は 1 行のまま(Review Focus 2)
  - `TestNoticeScenarioCloneBaseline`: A の保存のあとで C を clone → C の最初の同期では、clone より前の保存から通知を作らない
- [ ] **Step 2: 流して、足りない組み立てを直す** — Run: `go test ./cmd/hedgehug/ -run NoticeScenario` Expected: PASS
- [ ] **Step 3: 全体を確かめる** — Run: `go test ./... && golangci-lint run ./... && golangci-lint run --build-tags e2e ./...` Expected: PASS・指摘なし
- [ ] **Step 4: コミット**

```bash
git add cmd/hedgehug
git commit -m "feat(sync): 同期の判定を組み立て、2 台の場面で通知を確かめる(C4b1)"
```

---

### Task 8: 受信箱のプロジェクトの対象(API と画面)

**Files:**
- Modify: `internal/app/inbox/target.go`(`TargetProject`・`TargetDeletedProject`・`TargetMovedProject`、`projectTarget`)、`internal/app/inbox/page.go`(振り分け)
- Test: `internal/app/inbox/target_test.go`
- Modify: `internal/adapter/httpapi/inbox/dto.go`(`target.project`、kind の列挙)、`api/openapi.yaml`(`make generate`)、`web/packages/api/src/generated.ts`(`pnpm gen`)
- Modify: `web/packages/core/src/model/inbox.ts`(`InboxTargetKind` に 3 つ、`project`)
- Create or Modify: 行き先を決める関数(`web/packages/core/src/features/inbox/` の既存の行き先の関数。なければ `target.ts` を作る)と、その vitest
- Modify: `web/app/src/screens/inbox/InboxRow.svelte`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:455`(C4b1-12 の 1 文)

**Interfaces:**
- Produces(Go): `Target` に `Project project.ID`。`TargetProject = "project"`(`Display` はいまの接頭辞のプロジェクトの ID、`Title` はプロジェクト名、`WorkspaceName`)・`TargetDeletedProject = "deleted_project"`・`TargetMovedProject = "moved_project"`(`Display` は通知の `Display`)。アイテムもプロジェクトもない通知はいまのまま `workspace`
- Produces(web): `project` → `{ name: "project", store, project: display, tab: "overview" }`。`moved_project` → 同じ形で `project: 通知の display`。`deleted_project` は「削除されたプロジェクト」でリンクなし。クリックで既読にするのと、Ctrl+クリック等で既読にしないのは、アイテムと同じ `links.click`

- [ ] **Step 1: 失敗するテストを書く**
  - Go: `TestTargetProject`(生きている → `project`・`Title`・`Display`。接頭辞をあとで変えると新しい ID)・`TestTargetDeletedProject`・`TestTargetMovedProject`
  - vitest: 3 つの kind の行き先(project・moved_project はプロジェクトの概要のアドレス、deleted_project は行き先なし)と、表示名「削除されたプロジェクト」
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/inbox/ && cd web && pnpm -r test` Expected: FAIL
- [ ] **Step 3: 実装し、`make generate`・`pnpm gen` で生成物を書き直し、06 `:455` に文を足す**(「アイテムが削除済みのときは…」の文のあとに「プロジェクトが削除済みのときは「削除されたプロジェクト」と表示し、リンクにしない。」)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./... && make web-check` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/inbox internal/adapter/httpapi/inbox api/openapi.yaml web docs/superpowers/specs
git commit -m "feat(inbox): プロジェクトを指す通知を受信箱で開けるようにする(C4b1)"
```

---

### Task 9: 計画の記録と全体の検証

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(小計画の一覧・漏れの表・G への申し送り)

- [ ] **Step 1: トラック C の計画を書き直す**
  - C4b の行を C4b1(この計画。範囲は C4b1-1、完成の条件は「2 つのクローンの場面で、受け取った側の受信箱に割り当て・メンション・ウォッチ・自動化の行が 06 の文言どおり 1 回だけ出る」)と C4b2(同期の修復の通知: 修復 (1)〜(5)、削除の取り消し(R11・R12 を含む)、別の Store への移動・完全な削除と重なって反映されなかったとき。修復 (2) の親がワークスペースのときの「ワークスペース「{名前}」」(B5b)、(5) の「。合う値がなく空になった項目: A・B」(C3b-7)。C4b1 の `port.JudgeFunc` に、このマシンが書いた修復の保存と、合わせで捨てた自分の行を足して作る)に分ける
  - C4c の行に C4b1-18(上書きを作った項目はウォッチを飛ばす)を足す
  - 漏れの表の「割り当て / メンション / ウォッチ / 同期の修復」を「C4b1(割り当て・メンション・ウォッチ)、C4b2(同期の修復)」に、「自動化」を「C4a・C4b1」にする
  - G への申し送りに「Store をこのマシンから削除したら `JudgedSaves.ForgetStore` を呼ぶ(C4b1-16)。Store を追加したら、その時点の保存を判定済みにする必要はない(C4b1-3 で最初の同期が印を付ける)」を足す
- [ ] **Step 2: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./... && make web-check` Expected: すべて PASS
- [ ] **Step 3: コミット**

```bash
git add docs
git commit -m "docs: トラック C の C4b を C4b1・C4b2 に分け、C4b1 の申し送りを書く"
```
