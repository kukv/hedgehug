# hedgehug トラック C4b2(同期の修復の通知)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期の修復(削除の取り消し・親の削除・番号の重なり・親の循環・親と別のワークスペース/プロジェクトにある子)と、完全な削除・別の Store への移動と重なって変更が反映されなかったことを、あなた宛ての「同期の修復」の通知として受信箱に 1 回だけ出す。

**Architecture:** C4b1 の判定の差し込み口 `port.JudgeFunc` に渡すものを広げる。受け取った保存に加えて、このマシンが書いた修復の保存と、まだ判定していない合わせ(どちらのマシンが作ったものも)を渡す。保存と合わせには、消えたエンティティのフォルダ(消える前のログと、合わせなら共通の祖先にない行)を付ける。宛先と本文は新しい usecase のファイル群 `internal/app/noticing/repair*.go` が、修復の行と、修復の直前のログから決める(C3a-18)。2 台が同じ修復をしても 1 件にするため、通知に修復ごとに決まる鍵(`Notice.Source`)を付け、受信箱は同じ鍵の行があれば足さない。受信箱は、設定の値・スプリント・リリースを開く区画(`Notice.Section`)を持ち、完全な削除の通知は一覧を開いても消さない。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite)、TypeScript・Svelte(web/packages/core、web/app)、vitest。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:25`(非表示の Store)、`:209`(削除と変更の重なり)、`:237-243`(番号)、`:302-306`(循環)、`:310-318`(親と別のワークスペース・プロジェクトにある子)、`:320-324`(親の削除)、`:326-332`(完全な削除・別の Store への移動と重なったとき)、`:344-350`(修復の決まり)、`:352-383`(通知・作らない場合・判定する単位)、`:420-422`(差出人は hedgehug)、`:442-455`(行の表示)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:175-213`(同期の手順・同期の修復の表)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c4b1.md`(判定の差し込み口・判定済みの記録・`noticing`。この計画はその上に足す)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c3a.md` の C3a-18(通知は修復のコミットの行と対象のログから作る)、`2026-11-14-hedgehug-track-c3b.md` の C3b-7(空になった項目)、`2026-11-14-hedgehug-track-b5b.md` `:57`(親がワークスペースのときの呼び方)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C4b2 の行

## Global Constraints

土台・トラック C・C4a・C4b1 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個(`internal/archtest/size.go`)。`internal/app/port`・`internal/app/apptest` は 20 個で満杯なのでファイルを足さない。`internal/infra/gitstore` は 19 個で、足せるのは 1 個(Task 2 の `replicator_judge.go`)。`gitstore/replicator_mirror.go`(300 行)・`gitstore/saves/saves.go`(295 行)には足さず、新しいファイルに置く。web は .svelte 200 行、.ts 300 行。
- 口は git の言葉を使わない(`port` の型の名前とコメントに「コミット」「ブランチ」「マージ」を書かない。「保存」「合わせ」と書く)。
- app の機能どうしは import しない(`noticing` は `repairing` を import しない。行の形は `domain` の型で読む)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 通知の本文(06 のとおり。{…} はそのまま置き換える。名前の改行と続いた空白は 1 つの空白にする):
  - 削除の取り消し: 「{対象} は {変更した人} が同時に変更したため、削除を取り消しました」。{対象} はアイテム・プロジェクトなら ID、ワークスペースなら「ワークスペース「{名前}」」、設定の値・スプリント・リリースなら「「{名前}」」、コメントなら「{ID} のコメント」、作業時間なら「{ID} の作業時間」(決定 C4b2-9)
  - 親の削除: 「{親の ID} が削除されたため、{子の ID} も削除済みに入りました」。親がワークスペースなら {親の ID} は「ワークスペース「{名前}」」
  - 番号: 「2 台で同時に {旧 ID} が作られたため、あとから届いた方を {新 ID} に振り直しました」
  - 循環: 「親が循環したため、{ID} を {行き先の ID} の下へ移しました」。Task にしたときは続けて「。Step は Task になりました」
  - 親と別のワークスペース: 「{親の ID} が {ワークスペース名} へ移動したため、{旧 ID} を {新 ID} に移動しました」。空になった項目があれば続けて「。合う値がなく空になった項目: A・B」
  - 親と別のプロジェクト: 「{親の ID} が プロジェクト「{名前}」へ移動したため、{ID} をそのプロジェクトに移しました」(06 `:318` の {移動先} と「そのプロジェクト」の読み替えを当て、空白は C4b2-9)
  - 完全な削除: 「{ID} は完全に削除されたため、あなたの変更は反映されませんでした」。祖先が完全に削除されたときは {ID} を祖先の ID にする
  - 別の Store: 「{旧 ID} は {移動した人} が {移動先の Store 名} へ移動したため、あなたの変更は反映されませんでした。移動先は {新 ID} です」。移動したものの下に作った子なら、末尾に「({子の ID}「{タイトル}」)」
  - 人の名前({変更した人}・{移動した人})は Store のメンバーの表示名。メンバーにいなければログイン名。差出人は空(「hedgehug」)
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・`make web-check`。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C4b2-1 | 範囲: 06 の「同期の修復」の通知のうち、修復 (1)〜(5)、削除と変更の重なりの取り消し(アイテム・プロジェクト・コメント・作業時間・設定の値・ワークスペース(R11 の B5b の削除)・スプリント・リリース(R12 の F1 の削除))、完全な削除・別の Store への移動と重なって反映されなかったとき。C5 の修復 (6)〜(11) の通知は C5 が同じ仕組みに足す。08 の通知(診断での修復、のせ直せなかった変更)は G | トラック C の計画の C4b2 の行。08 は G の持ち場(トラック C の計画の漏れの表「remote の履歴にそろえる」は G) |
| C4b2-2 | 判定に渡すもの: (a) 受け取った保存(C4b1 のまま)、(b) このマシンが書いた修復の保存(自分の先端から辿れて remote の先端から辿れず、作者が hedgehug の bot で、行がすべて実行者 `sync`、または行がない保存)、(c) まだ判定していない合わせのうち、消えたエンティティのフォルダに共通の祖先にない行があるもの(どちらのマシンが作った合わせでも)。順は C4b1 と同じ古い順。ほかのこのマシンの保存は、いまのまま渡さない | 06 `:243` ほか(修復をしたのがどのマシンでも、宛先のマシンが作る)。修復をしたのが自分のマシンなら、修復の保存は受け取らない。完全な削除・別の Store への移動は修復の保存がなく、証拠は合わせにだけある(保存形式 `:183`)。合わせを作ったのが相手のマシンのこともある |
| C4b2-3 | `ReceivedSave` に `Repair bool`(同期の修復の保存。C4b2-2 の (b) と同じ見分け方を、受け取った保存にも当てる)・`Merge bool`(合わせ)・`Removed []RemovedEntity` を足す。`RemovedEntity` は `Ref`・`Log`(消える前のログ。保存なら最初の親の、合わせならその側の親の)・`New`(合わせのときだけ。その側の行のうち、共通の祖先の同じエンティティのログにない行)。`Removed` を作るのは修復の保存と合わせだけ(ほかの保存は空)。合わせは、2 つの親のそれぞれについて、親にあって合わせの結果にない `log.jsonl` のエンティティを見る | 修復 (1) の完全な削除と、合わせでのフォルダの削除は、`log.jsonl` の行を足さないので、C4b1 の `Entities` には何も出ない |
| C4b2-4 | `port.JudgeFunc` の引数を `in port.Received`(`Saves []ReceivedSave`・`ReadOnly bool`)にする。`ReadOnly` は同期の remote が `ReceiveOnly` のとき。閲覧のみなら同期の修復の通知を作らない。非表示の Store では作る(受信箱には出ない) | 06 `:346`・`:371`(閲覧のみは作らない)、`:25`(非表示でも受け取った変更からの通知は作る)。`port.StoreList` は表示中の Store しか返さず、非表示の Store が閲覧のみかを知れない |
| C4b2-5 | `Judge` がエラーのときは、(b)・(c) で渡した保存と合わせを判定済みにしない(`markOwn` の対象から外す)。次の同期でもう一度渡す | C4b1-5 と同じ。渡したものを判定済みにすると通知が出ないままになる |
| C4b2-6 | 1 回だけにする鍵: 同期の修復の通知は `Notice.Source` に修復ごとに決まる鍵を持ち、受信箱の `AddOnce` は同じ Store・Source の行があれば足さない(既読・未読も変えない)。鍵: 取り消し `repair:restore:{削除の op}`、親の削除 `repair:cascade:{子の内部 ID}:{削除の op}`、番号 `repair:number:{内部 ID}:{新しい番号}`、循環 `repair:cycle:{内部 ID}:{行き先の内部 ID}`、親と別のワークスペース・プロジェクト `repair:place:{内部 ID}:{ワークスペースの内部 ID}:{プロジェクトの内部 ID}`、完全に削除された相手への関連 `repair:relation:{持ち主の内部 ID}:{相手の内部 ID}`、完全な削除 `repair:purged:{内部 ID}`、別の Store `repair:moved:{内部 ID}` | 06 `:286`・`:348`(同じ修復を複数のマシンが行っても、通知は 1 台につき 1 件)。修復の行の ID はマシンごとに違うが、修復の中身はどのマシンでも同じなので、中身から鍵を作る |
| C4b2-7 | 宛先は行の `by`(大文字と小文字を区別しない)で決める。あなたの別のマシンの変更でも、このマシンに作る。「最後に保存した人」は、修復の保存の直前のログ(いまのログから、その保存と、同じ判定のあとの保存の行を除いたもの)で、実行者が人の行のうち `at` が最後のものの `by`。種類ごとの宛先: 取り消しは削除の行の `by`、親の削除は子のログで最後に保存した人、番号はエンティティの `create` の行の `by`、循環と親と別のワークスペース・プロジェクトは子の `place` を最後に保存した人、修復 (1) の完全な削除は消えたアイテムのログで最後に保存した人、関連は相手を足した `add relations` の行の `by`、合わせで消えたものは `New` の人の行の `by`(1 人に 1 件) | 06 の各節の宛先。行はマシンを持たないので、人で決める(C4b1-7 と同じ)。06 `:324` の「子を変更した・追加した人」は、修復の直前に子へ最後に書いた人を当てる(親の削除と同時の変更は、その人の行が最後になる) |
| C4b2-8 | 修復の種類は、修復の保存の行で見分ける。`restore` の行がある → 取り消し(削除の op は行の `to`。op ごとに 1 件、対象は `root: true` の削除の行か、`deleted` でない f の削除の行があるエンティティ)。`delete`(`root: false`)の行 → 親の削除(親は同じ op で `root: true` の削除の行を持つエンティティ)。アイテム・プロジェクトの `set place` の行は、前と後の場所を比べ、ワークスペースが違う → 親と別のワークスペース、親が違う → 循環、プロジェクトが違う → 親と別のプロジェクト、番号だけが違う → 番号。1 つの保存の中で `set place` を書いた削除されていないアイテムが主語(親と別のワークスペースの保存は、削除済みの下のものも一緒に動かす)。`remove relations` の行 → 関連。行がなく `Removed` がある → 修復 (1) の完全な削除 | 修復の保存は 1 件の修復を 1 つの保存にしている(C3a-18)。件名の文字列に頼らない |
| C4b2-9 | 取り消しの {対象} の呼び方は、修復のアクティビティの文(`repairing.summary`、06 `:514`、B5b-8)と、他のタブの取り消しのトースト(06 `:185`)に合わせる。{変更した人} は、削除の行があるログで、削除の op でなく、削除の f(`deleted` ならすべての f、ほかは f とその下の f)を変えた、人が書いた行(作成・移入の行は数えない)のうち、削除した人でない人の行の `at` が最後のものの `by`。削除した人の行しかなければ、その行の `by`(`repair.hasChange` と同じ数え方を、片側の見分けなしで当てる)。1 つの op で複数の値を削除して取り消したときの対象は、重なった変更がいちばんあとのもの(同じなら f の辞書順)。助詞の前の空白は `repairing.summary` に合わせ、ID のあとに空白を入れ、」や日本語のあとには入れない(親と別のプロジェクトの本文も「プロジェクト「{名前}」へ」とする) | 06 `:209` の本文はアイテムの {ID} だけを書いているので、ほかのものは同じ節のアクティビティとトーストの呼び方を当てる。{変更した人} は 06 `:209` で削除した人とは別の、同時に変えた人(削除した人の名前を出さない)。Task 3 の実装とレビューで決めた |
| C4b2-10 | 対象と開く先: アイテム → `Item`、プロジェクト → `Project`、ワークスペース・設定の値・スプリント・リリース → `Workspace` と `Section`。`Section` は、ワークスペースの削除は `general`、値の f の頭が `statuses/`・`priorities/`・`systems/`・`properties/`・`templates/`・`rules/` ならそれぞれ `statuses`・`priorities`・`systems`・`properties`・`templates`・`automation`、`sprints/` は `sprints`、`releases/` は `releases`、ほかは `general`。画面は `sprints`・`releases` ならワークスペースの「スプリント」「リリース」タブ、ほかは設定タブのそのサブタブを開く。`Section` が空の通知(C4a の自動化)はいまのまま `automation` | 06 `:455`(開く先は、スプリントならワークスペースの「スプリント」タブ、リリースなら「リリース」タブ、プロパティ・値・ワークフロー・テンプレート・接頭辞・ワークスペース名なら設定の該当するサブタブ)。サブタブの ID は web の `workspaceSettingsTabs` |
| C4b2-11 | 完全な削除のために反映されなかった通知(鍵が `repair:purged:` のもの)は、受信箱を開いて `Prune` しても消さない。`Item`(または `Project`)は消えたものの内部 ID で、行の位置に作ったときの ID を出し、クリックで旧 ID のアドレスを開く(別の Store へ移ったものと同じ `moved`・`moved_project` の出し方。行き先の画面が「完全に削除されました」と出す)。消えたものを指すほかの通知は、いまのとおり消す。仕様 06 `:455` にこの文を足す | ユーザーの決定(2026-11-14)。受信箱は開くたびに `purged.jsonl` にあるものの通知を消す(C4a)ので、そのままでは完全な削除を知らせる通知が出たとたんに消える |
| C4b2-12 | 合わせで消えたものの原因は、いまの Store で、消えたもの自身、親、プロジェクト、ワークスペースの順に、`purged.jsonl` の target と、`transfers.jsonl` の別の Store への行(`to_store` がこの Store でない)を探し、最初に見つかったもの。完全な削除なら {ID} はその行の `display`。別の Store なら {旧 ID}・{新 ID}・{移動した人}・{移動先の Store 名} は転送記録の `old`・`new`・`by` と、移動先の Store 名(`to_store` の Store がこのマシンの表示中の Store にあればその名前、なければ `to_store_name`、それも空なら `to_store`。保存形式 `:257`)。ワークスペースが原因のときは、ほかの通知と同じく {ID}・{旧 ID} を「ワークスペース「{名前}」」にし(名前は消える前のログ、なければ `display`)、別の Store では「。移動先は {新 ID} です」を付けない(ワークスペースの転送記録の old・new は接頭辞で、移動先は Store 名で分かる)。消えたもの自身でなく祖先の転送記録なら子の文の末尾を足す。{子の ID} は消えたもののログから作り、接頭辞はいまのワークスペースのもの(なければ内部 ID)、{タイトル} はログのタイトル。どれも見つからなければ作らない | 保存形式 `:183`(どちらだったかは purged.jsonl と transfers.jsonl で見分ける)。祖先が移ったときは、その下のものも転送記録を持つ(一緒に動く)ので、近い方から探せば足りる |
| C4b2-13 | 他のタブの取り消し(C3c の R1〜R12。人の保存が書く `restore` の行)からは通知を作らない。通知は同期の修復の保存(C4b2-2 の見分け方)からだけ作る | 06 `:185` は他のタブの重なりにトーストだけを定めている。人の保存の `restore` は「復元」(G)と見分けられない |
| C4b2-15 | 合わせた結果が自分の版と同じになる同期(相手が自分の完全に削除したものだけに書いていたとき)で、合わせの保存が空として断られ、その Store を同期できなくなる既存の不具合を、この計画で直す(Task 2b)。中身が変わらない合わせも保存する | ユーザーの決定(2026-11-15)。Task 2 の実装で見つかった。完全な削除のために反映されなかった通知の場面そのもの |
| C4b2-16 | 親と別のワークスペースの本文の {親の ID} は、親が移動先へ入る前の ID(同じ Store の転送記録のうち、修復の保存の時刻までの親の最後の行の `old`。なければいまの ID)。プロジェクトだけのときは、親の ID は変わらないのでいまの ID。番号・循環・移動の宛先の「場所を最後に保存した人」には、作成の行も数える(子の追加) | {旧 ID} と並べて、子を作った人が知っている呼び方にする。移動先は {新 ID} で分かる。06 `:314` の「子の追加」。Task 4 の実装で決めた |
| C4b2-14 | e2e は足さない。2 台の場面は `cmd/hedgehug` の本物の gitstore と state.db で確かめる。完全な削除と別の Store への移動は、`cmd/hedgehug` から書く口がない(完全な削除は G、別の Store への移動は 2 つの Store が要る)ので、gitstore のテスト(`purgeForTest` と、フォルダを消して転送記録を書く移動の手助け)で `Removed` の中身を確かめ、`noticing` のテストで本文と宛先を確かめる | C4b1-17 と同じ。`purgeForTest` は gitstore のテストの中にしかない |

## Review Focus

1. **2 台が同じ修復をした:** 削除した人のマシンに、自分の修復の保存と、相手のマシンの同じ修復の保存の両方が届いても、通知は 1 件(Task 3 の `TestRestoreOnceAcrossMachines`、Task 1 の `TestInboxAddOnce`)。
2. **相手のマシンが作った合わせで変更が消えた:** 自分が先に送った変更を、あとから同期した移動した人のマシンが合わせて消した。自分のマシンは、受け取った合わせから通知を作る(Task 2 の `TestRemovedInOthersMerge`、Task 5 の `TestMovedDropped`)。
3. **昔の自分の行で通知しない:** 自分が去年作ったアイテムを、太郎さんが完全に削除し、花子さんが同時に変えた。自分には作らず、花子さんにだけ作る(合わせの `New` だけを見る。Task 5 の `TestDroppedOnlyNewRows`)。
4. **判定に失敗した:** 受信箱に書けず `Judge` がエラーのとき、自分の修復の保存と合わせは判定済みにならず、次の同期で 1 回だけ出る(Task 2 の `TestJudgeErrorKeepsOwnRepair`)。
5. **完全な削除の通知が残る:** 受信箱を開いても「HH-5 は完全に削除されたため…」は消えず、HH-5 への割り当ての通知は消える(Task 1 の `TestInboxPruneKeepsPurgedRepair`)。
6. **閲覧のみ・非表示:** 閲覧のみの Store の同期では、受け取った修復の保存の宛先があなたでも作らない。非表示の Store では作る(Task 3 の `TestRepairSkipsReadOnly`)。

---

### Task 1: 受信箱の 1 回だけ足す口・開く区画・消さない通知(`inbox/003`)

**Files:**
- Modify: `internal/app/port/inbox.go`(`Notice.Section`、`Inbox.AddOnce`、`Prune` の説明)
- Modify: `internal/infra/statedb/inbox.go`(`inbox/003`、`AddOnce`、`Prune`、読み出しの列。300 行を超えるなら書き込みを `inbox_write.go` に移す)
- Test: `internal/infra/statedb/inbox_test.go`
- Modify: `internal/app/apptest/inbox.go`、Test: `internal/app/apptest/inbox_test.go`
- Modify: `internal/app/inbox/notifier.go`(`AddOnce` を通し、足したときだけ `EventInbox` を出す)、Test: `internal/app/inbox/actions_test.go`

**Interfaces:**
- Produces(`port`):
  - `Notice.Section string` — ワークスペースを指す通知の開く区画(C4b2-10)。空は自動化(`automation`)
  - `Inbox.AddOnce(ctx, n Notice) (bool, error)` — 同じ `Store`・`Source` の行がなければ `Add` と同じく足して true。あれば何もせず false。`n.Source` が空ならエラー
  - `Prune` は、`kind = repair` で `source` が `repair:purged:` で始まる行を消さない(C4b2-11)
- `inbox/003`: `ALTER TABLE inbox ADD COLUMN section TEXT NOT NULL DEFAULT ''`

- [ ] **Step 1: 失敗するテストを書く(statedb と apptest の両方)**
  - `TestInboxAddOnce`: `AddOnce(Kind: repair, Source: "repair:restore:op1", Text: "HH-1 は hanako が…")` → true・1 行。既読にする。同じ Source でもう一度 → false、行は 1 つで既読のまま。別の Store の同じ Source → true
  - `TestInboxSection`: `Add(Section: "sprints")` → `List` で `Section == "sprints"`
  - `TestInboxPruneKeepsPurgedRepair`: アイテム i1 の `repair`(Source `repair:purged:i1`)と `assign` → `Prune(s1, [i1])` で `assign` だけ消える
  - `TestNotifierAddOnce`: 足したときだけ `EventInbox` が 1 回、重なったときは出さない
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/statedb/ ./internal/app/apptest/ ./internal/app/inbox/` Expected: FAIL
- [ ] **Step 3: 実装する**(`port.Inbox` を満たすすべての型に `AddOnce` を足す。`go build ./...` で漏れがないことを確かめる)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/... && go build ./...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/inbox.go internal/infra/statedb internal/app/apptest/inbox.go internal/app/apptest/inbox_test.go internal/app/inbox
git commit -m "feat(inbox): 同じ鍵の通知を 1 回だけ足す口と、開く区画の列を足す(C4b2)"
```

---

### Task 2: 自分の修復の保存と合わせを判定に渡す(`gitstore/saves`)

**Files:**
- Modify: `internal/app/port/replicator.go`(`Received`・`ReceivedSave` の 3 つのフィールド・`RemovedEntity`・`JudgeFunc`)
- Create: `internal/infra/gitstore/saves/removed.go`(修復の保存の見分け、消えたエンティティ、合わせの `New`)
- Modify: `internal/infra/gitstore/saves/judge.go`(渡すものを C4b2-2 に広げ、`markOwn` から渡したものを外す)、`internal/infra/gitstore/saves/saves.go`(`Unjudged` から呼ぶところだけ。行は増やさない)
- Test: `internal/infra/gitstore/saves/removed_test.go`
- Create: `internal/infra/gitstore/replicator_judge.go`(`replicator_mirror.go` から `baseline`・`finish`・`judge` を移し、`ReadOnly` を渡す)
- Modify: `internal/infra/gitstore/commit.go`(bot の名前を `saves.BotName` から読む)
- Test: `internal/infra/gitstore/sync_judge_repair_test.go`
- Modify: `internal/app/noticing/service.go`(`Judge` の引数を `port.Received` に。C4b1 の判定は `in.Saves` で同じく動く)、`internal/app/noticing/*_test.go`(呼び方だけ)

**Interfaces:**
- Produces(`port`):
  - `type RemovedEntity struct { Ref change.Ref; Log []change.Record; New []change.Record }`(C4b2-3)
  - `ReceivedSave` に `Repair bool`・`Merge bool`・`Removed []RemovedEntity`
  - `type Received struct { Saves []ReceivedSave; ReadOnly bool }`
  - `type JudgeFunc func(ctx context.Context, store StoreID, in Received, read func(fn func(r Reader) error) error) error`
- Produces(`saves`):
  - `const BotName`(gitstore の `botName` を移す)
  - `func Pending(repo *git.Repository, theirs, head plumbing.Hash, known func(port.SaveKey) (port.Match, error)) ([]port.ReceivedSave, error)` — C4b2-2 の (a)・(b)・(c) を古い順に。`saves.Judge` は `Unjudged(theirs)` の代わりにこれを使う
  - (b) と受け取った保存の `Repair` の見分け: 作者の名前が `BotName` で、足した行がすべて `Actor.Kind == change.ActorSync` か、行がない
- `markOwn` は、`Pending` が返した保存と合わせの印を `Mark` しない(C4b2-5)

- [ ] **Step 1: 失敗するテストを書く**
  - saves: `TestRepairSaveFlag`(bot の作者で実行者 `sync` の行の保存 → `Repair`。bot の作者で実行者 `automation` の行 → `Repair` でない。人の作者 → でない)・`TestRemovedOnRepairPurge`(アイテムのフォルダを消した bot の保存 → `Removed` にそのアイテムの `Ref` と消える前のログ、`New` は空)・`TestRemovedInMerge`(片側でフォルダを消し、もう片側で 1 行足した合わせ → `Merge`・`Removed` の `New` がその 1 行。足していない側だけなら合わせを返さない)・`TestOwnRepairPassed`(自分の先端にだけある修復の保存は渡し、自分の人の保存は渡さない)
  - gitstore(`sync_helpers_test.go` の 2 台、`noRepair` でない本物に近い修復の代わりに、bot の実行者 `sync` で 1 行書く修復の手助けを置く): `TestJudgeGetsOwnRepair`(A の同期の修復の保存が A の `Judge` に渡る。もう一度同期しても渡らない)・`TestRemovedInOthersMerge`(A がアイテムに 1 行書いて送る → B が同じアイテムを `purgeForTest` してから同期して合わせを送る → A の同期で、B の合わせが `Merge`・`Removed` の `New` に A の行を持って渡る)・`TestJudgeErrorKeepsOwnRepair`(`Judge` がエラー → 次の同期で、同じ修復の保存と合わせがもう一度渡る)・`TestJudgeReadOnlyFlag`(閲覧のみの同期は `ReadOnly == true`)
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/...` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/... ./internal/app/noticing/ && golangci-lint run ./internal/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/replicator.go internal/infra/gitstore internal/app/noticing
git commit -m "feat(sync): 自分の修復の保存と、消えたものがある合わせも判定に渡す(C4b2)"
```

---

### Task 2b: 中身が変わらない合わせも保存する(C4b2-15)

**Files:**
- Modify: `internal/infra/gitstore/replicator.go`(`merge` の保存で空の合わせを許す)
- Test: `internal/infra/gitstore/sync_judge_repair_test.go`(`TestRemovedInOthersMerge` を、A がアイテムだけに書く場面に戻す)、`internal/infra/gitstore/sync_purge_test.go`(`TestSyncAfterPurgeOnlyOthersWrote`)

- [ ] **Step 1: 失敗するテストを書く** — `TestSyncAfterPurgeOnlyOthersWrote`: B がアイテムを `purgeForTest`、A がそのアイテムだけに 1 行足して同期 → B の同期がエラーにならず、2 回目の同期も通り、A と B の中身が同じ。`TestRemovedInOthersMerge` から Epic への書き込みを外す
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/ -run 'PurgeOnlyOthersWrote|RemovedInOthersMerge'` Expected: FAIL(`cannot create empty commit`)
- [ ] **Step 3: 実装する**(合わせの保存に `AllowEmptyCommits: true`)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット** — `fix(sync): 合わせた結果が自分の版と同じでも合わせを保存し、同期が止まらないようにする(C4b2)`

---

### Task 3: 削除の取り消しと、親の削除の通知

**Files:**
- Modify: `internal/domain/repair/overlap.go`(`Changer`)、Test: `internal/domain/repair/overlap_test.go`
- Create: `internal/app/noticing/repair.go`(修復の保存の振り分け、修復の直前のログ、宛先、名前、`AddOnce` で書く)
- Create: `internal/app/noticing/repair_restore.go`(取り消し・親の削除)
- Modify: `internal/app/noticing/service.go`(`Judge` が修復の通知も作る。閲覧のみなら作らない)
- Test: `internal/app/noticing/repair_restore_test.go`、`internal/app/noticing/fixture_test.go`(修復の保存を `actor: sync` で書く手助け)

**Interfaces:**
- Consumes: Task 1 の `Inbox.AddOnce`・`Notice.Section`、Task 2 の `port.Received`・`ReceivedSave.Repair`
- Produces:
  - `func repair.Changer(log []change.Record, d change.Record) string` — C4b2-9 の {変更した人} のログイン名(見つからなければ空。空なら本文の {変更した人} は「不明なユーザー」)
  - `noticing` の内部: `func (b batch) beforeSave(ref change.Ref, i int) []change.Record`(C4b2-7。いまのログから saves[i:] の行を除く)、`lastSaver(log []change.Record, field string) string`(field が空ならすべての項目)、`nameOf(r port.Reader, login string) string`(表示名)

- [ ] **Step 1: 失敗するテストを書く**(あなたは `taro`)
  - domain: `TestChanger`(削除の op でない人の行のうち最後の `by`。実行者 `sync`・`automation` の行と、ほかの f の行は数えない)
  - `TestRestoreItem`: taro の HH-1 の削除を、hanako の変更で取り消した修復の保存 → `repair`・本文「HH-1 は 花子 が同時に変更したため、削除を取り消しました」(hanako の表示名が 花子)・`Item`・`Sender == ""`・`Source == "repair:restore:{op}"`
  - `TestRestoreVariants`(表で): プロジェクト → `Project`・「HH-P1 は…」、ワークスペース → 「ワークスペース「Web」は…」・`Workspace`・`Section == "general"`、ステータス Review → 「「Review」は…」・`Section == "statuses"`、スプリント → `Section == "sprints"`、リリース → `Section == "releases"`、自動化のルール → `Section == "automation"`、コメント → 「HH-1 のコメントは…」・`Item`、作業時間 → 「HH-1 の作業時間は…」
  - `TestRestoreNotDeleter`: 削除したのが hanako → 作らない
  - `TestRestoreOnceAcrossMachines`: 同じ削除の op を取り消した 2 つの修復の保存(行の ID が違う)を 1 回の `Judge` と、続けて 2 回目の `Judge` で → 受信箱の行は 1 つ
  - `TestRestoreByPersonSkipped`: 人の保存の `restore` の行(C3c) → 作らない(C4b2-13)
  - `TestCascadeNotice`: hanako が HH-1 を削除、taro が子 HH-2 を変更 → 修復の保存(HH-2 に `root: false` の削除) → 本文「HH-1 が削除されたため、HH-2 も削除済みに入りました」・`Item == HH-2`。親がワークスペース → 「ワークスペース「Web」が削除されたため…」。子に最後に書いたのが hanako → 作らない
  - `TestRepairSkipsReadOnly`: `ReadOnly: true` → 修復の通知を作らない(割り当ては作る)
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/repair/ ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/... ./internal/app/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/domain/repair internal/app/noticing
git commit -m "feat(noticing): 削除の取り消しと親の削除を、同期の修復の通知にする(C4b2)"
```

---

### Task 4: 番号・循環・親と別のワークスペース/プロジェクトの通知

**Files:**
- Create: `internal/app/noticing/repair_place.go`
- Test: `internal/app/noticing/repair_place_test.go`

**Interfaces:**
- Consumes: Task 3 の `beforeSave`・`lastSaver`・振り分け
- 空になった項目(C3b-7): 修復の直前のアイテム(`item.FromLog(beforeSave)`)に、`transfer.Remap(it, 移動元の設定, 移動先の設定, 移動先のプロジェクトのルール, nil, nil)` をかけ直し、`Emptied` を「・」でつなぐ。設定とルールは、移動元・移動先のワークスペースのログの修復の直前のもの(`beforeSave`)から作る

- [ ] **Step 1: 失敗するテストを書く**(あなたは `taro`)
  - `TestNumberNotice`: taro が作った HH-3 を HH-4 に振り直した修復 → 「2 台で同時に HH-3 が作られたため、あとから届いた方を HH-4 に振り直しました」・`Item`・`Display == "HH-4"`。プロジェクト HH-P2 → HH-P3 も同じ形で `Project`。作ったのが hanako → 作らない
  - `TestCycleNotice`: taro が最後に親を保存した HH-5 を HH-1 の下へ移した修復 → 「親が循環したため、HH-5 を HH-1 の下へ移しました」。同じ保存に `set kind`(Step → Task) → 末尾に「。Step は Task になりました」。親を最後に保存したのが hanako → 作らない
  - `TestPlaceProjectNotice`: 親 HH-2 がプロジェクト「サイト」へ移り、taro が親を付けた子 HH-6 を移した修復 → 「HH-2 が プロジェクト「サイト」へ移動したため、HH-6 をそのプロジェクトに移しました」
  - `TestPlaceWorkspaceNotice`: 親 HH-2 がワークスペース「Docs」(接頭辞 DOC)へ移り、子 HH-6 を DOC-4 にした修復 → 「HH-2 が Docs へ移動したため、HH-6 を DOC-4 に移動しました」・`Item`・`Display == "DOC-4"`。移動先にない担当者と優先度の値がある → 末尾「。合う値がなく空になった項目: 担当者・優先度」(並びは `Emptied` の順)。同じ保存で動いた削除済みの下のものには作らない
  - `TestPlaceOnceAcrossMachines`: 同じ移動の 2 つの修復の保存 → 1 行
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/noticing/` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 番号・親の循環・親に合わせた移動を、同期の修復の通知にする(C4b2)"
```

---

### Task 5: 完全な削除・別の Store への移動と重なったときの通知

**Files:**
- Create: `internal/app/noticing/repair_gone.go`
- Test: `internal/app/noticing/repair_gone_test.go`

**Interfaces:**
- Consumes: Task 2 の `Removed`・`Merge`、Task 3 の振り分け、`port.Reader` の `Purged()`・`Transfers()`
- 対象(C4b2-11): アイテムなら `Item` と `Display`(消える前の ID)、プロジェクトなら `Project` と `Display`、ワークスペースなら `Workspace`

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`)
  - `TestPurgedUnderNotice`: 祖先 HH-1 が完全に削除され、hanako が最後に書いた子 HH-7 を消した修復の保存(行なし、`Removed`) → 「HH-1 は完全に削除されたため、あなたの変更は反映されませんでした」・`Item == HH-7`・`Source == "repair:purged:{HH-7 の内部 ID}"`
  - `TestPurgedRelationNotice`: hanako が足した HH-9 への関連を外した修復(`remove relations`) → 「HH-9 は完全に削除されたため…」・`Item` は持ち主
  - `TestPurgedDropped`: 合わせで消えた HH-5(自身が `purged.jsonl` にある)の `New` に hanako の行 → 「HH-5 は完全に削除されたため、あなたの変更は反映されませんでした」
  - `TestDroppedOnlyNewRows`: `Log` には hanako の古い行があり、`New` は jiro の行だけ → 作らない
  - `TestMovedDropped`: 合わせで消えた HH-5 に転送記録(old HH-5、new WEB-12、by taro、to_store_name team-b) → 「HH-5 は 太郎 が team-b へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-12 です」・`Item`・`Display == "HH-5"`・`Source == "repair:moved:{内部 ID}"`
  - `TestMovedChildDropped`: 移った HH-5 の下に hanako が作った子(転送記録なし、タイトル「リリースノート」、番号 31) → 末尾「(HH-31「リリースノート」)」
  - `TestDroppedNoCause`: 転送記録も `purged.jsonl` もない → 作らない
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 完全な削除と別の Store への移動で反映されなかった変更を知らせる(C4b2)"
```

---

### Task 6: 受信箱の開く区画と、完全な削除の行の出し方(API と画面、仕様)

**Files:**
- Modify: `internal/app/inbox/target.go`(`workspaceTarget` が `n.Section` を使う。空なら `automation`)、Test: `internal/app/inbox/target_test.go`
- Modify: `web/packages/core/src/features/inbox/target.ts`(`workspaceRouteOf`)、Test: `web/packages/core/src/features/inbox/target.test.ts`
- Modify: `web/app/src/inbox.ts`(`workspace` の行き先を `workspaceRouteOf` から)
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:455`

**Interfaces:**
- Produces(web): `workspaceRouteOf(section: string): { tab: WorkspaceTab; section?: string }` — `sprints`・`releases` はそのタブで区画なし、ほかは `settings` タブとその区画(空は `automation`)

- [ ] **Step 1: 失敗するテストを書く**
  - Go: `TestTargetWorkspaceSection`(`Section: "sprints"` の通知 → `Target.Section == "sprints"`、空 → `automation`)、`TestTargetPurgedRepairIsMoved`(アイテムが Store にない修復の通知 → `moved`・作ったときの `Display`)
  - vitest: `sprints` → `{ tab: "sprints" }`、`releases` → `{ tab: "releases" }`、`statuses` → `{ tab: "settings", section: "statuses" }`、`""` → `{ tab: "settings", section: "automation" }`
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/inbox/ && cd web && pnpm -r test` Expected: FAIL
- [ ] **Step 3: 実装し、06 `:455` に文を足す**(「完全に削除したアイテム・プロジェクト・ワークスペース(08)の通知は、一覧から消える。」のあとに「ただし、完全な削除のために変更が反映されなかったことを知らせる「同期の修復」の通知は消さず、位置に作ったときの ID を出し、クリックで旧 ID の転送記録の画面(02)を開く。」)
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./... && make web-check` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/inbox web docs/superpowers/specs
git commit -m "feat(inbox): 同期の修復の通知から設定の区画・スプリント・リリースを開く(C4b2)"
```

---

### Task 7: 組み立てと 2 台の場面テスト

**Files:**
- Modify: `cmd/hedgehug/wire.go`(`rep.Judge` は `noticing.Service.Judge` のまま。型が変わった分だけ)
- Modify: `cmd/hedgehug/repair_helpers_test.go`(必要なら、同じログイン名の 2 台目を clone する手助け)
- Create: `cmd/hedgehug/repair_notice_scenario_test.go`

**Interfaces:**
- Consumes: Task 1〜6。`wire_e2e.go` は変えない(C4b2-14)

- [ ] **Step 1: 場面テストを書く**(A = taro、B = hanako。本物の gitstore と、テストごとの state.db。修復は本物の `repairing.Run`)
  - `TestRepairNoticeRestoreOnMyMachine`: B が HH-1 を変えて同期 → A が HH-1 を削除して同期(A のマシンが取り消しの修復を書く)→ A の受信箱「同期の修復」に「HH-1 は hanako が同時に変更したため、削除を取り消しました」(場面のメンバーに表示名がなければログイン名)が 1 行。A がもう一度同期しても 1 行。B には作らない
  - `TestRepairNoticeRestoreFromOther`: A が HH-1 を削除して同期 → B が HH-1 を変えて同期(B のマシンが修復を書く)→ A が同期すると、A に同じ本文が 1 行
  - `TestRepairNoticeRestoreTwoMachinesSameLogin`: A と A2(どちらも taro)。A2 も同期すると、A2 にも 1 行。A は 1 行のまま
  - `TestRepairNoticeCascade`: A が Epic HH-1 を削除して同期、B が子 HH-2 を変えて同期 → B に「HH-1 が削除されたため、HH-2 も削除済みに入りました」
  - `TestRepairNoticeNumber`: A と B が同時にアイテムを作り(どちらも HH-3)、A が先に同期 → B の同期で B の HH-3 が HH-4 に振り直り、B に「2 台で同時に HH-3 が作られたため、あとから届いた方を HH-4 に振り直しました」
  - `TestRepairNoticeReadOnly`: B の同期を閲覧のみにして、A の修復の保存を受け取る → B に作らない
- [ ] **Step 2: 流して、足りない組み立てを直す** — Run: `go test ./cmd/hedgehug/ -run RepairNotice` Expected: PASS
- [ ] **Step 3: 全体を確かめる** — Run: `go test ./... && golangci-lint run ./... && golangci-lint run --build-tags e2e ./...` Expected: PASS・指摘なし
- [ ] **Step 4: コミット**

```bash
git add cmd/hedgehug
git commit -m "test(sync): 2 台の場面で同期の修復の通知が 1 回だけ出ることを確かめる(C4b2)"
```

---

### Task 8: 計画の記録と全体の検証

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C4b2 の行、C5 の行、漏れの表、G への申し送り)

- [ ] **Step 1: トラック C の計画を書き直す**
  - C4b2 の行に計画のファイル名(`2026-11-14-hedgehug-track-c4b2.md`)と範囲 C4b2-1 を書く
  - C5 の行に「同期の修復の通知は C4b2 の仕組み(修復の保存を `noticing` の振り分けに足し、鍵 `repair:{種類}:…` で `AddOnce`)に足す。(6)〜(11) の宛先と本文は 06 の各節。開く先は C4b2-10 の `Section`」を足す
  - 漏れの表の「別の Store へ移動したものと重なったとき」を「C1(フォルダ)、C4b2(通知)」のまま確かめ、「割り当て / メンション / ウォッチ / 同期の修復」の C4b2 を「C4b2((1)〜(5)・取り消し・反映されなかったとき)、C5((6)〜(11))」にする
  - G への申し送りに「08 の診断での修復と、のせ直せなかった変更の通知は、`Notice.Kind = repair` で、鍵は `repair:diagnose:…`・`repair:rebase:…` のように C4b2-6 の形で作り `AddOnce` で書く」を足す
- [ ] **Step 2: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./... && make web-check` Expected: すべて PASS
- [ ] **Step 3: コミット**

```bash
git add docs
git commit -m "docs: トラック C の計画に C4b2 の範囲と C5・G への申し送りを書く"
```
