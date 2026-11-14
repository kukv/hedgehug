# hedgehug トラック C(同期と通知)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 06(同期・通知・アクティビティ)を実装する。2 台のマシンが同じ Store を remote で共有し、項目ごとの後勝ちで合わせ、直すべきずれを同期の修復で直し、あなた宛ての通知と受信箱・アクティビティを出す。

**Architecture:** 同期は git の版だけが持つ口 `port.Replicator` を、`infra/gitstore` が実装する。ファイルを合わせる規則(log.jsonl の行の和、説明の行ごとの 3 方向マージ)は入出力のない `infra/gitstore/filemerge` に置き、ツリー全体の合わせ方(フォルダの削除、別の Store へ移ったものの下の削除、`*.yaml` の作り直し)は gitstore の中に置く。合わせたあとの修復の規則は `domain/repair`、修復を保存する流れと同期の usecase は `app/sync`、通知は `app/inbox`、アクティビティは `app/activity`。このマシンの状態(同期の結果、前に受け取った先端、通知と既読、判定済みの保存)は `infra/statedb`(SQLite)に置く。画面への反映は `adapter/httpapi` の Server-Sent Events。

**Tech Stack:** 土台と同じ(Go 1.27.3、go-git v5、huma v2、Svelte 5)。C2 から modernc.org/sqlite を足す。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`(全文。この計画の正本)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` の「同期」「同期の修復」「remote の履歴が書き換えられたとき」「アクティビティと履歴」「このマシンに持つもの」「技術構成」「テスト」
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(依存の向き、口、8 節「衝突を防ぐルール」)
- 01「書き込みの単位」「閲覧のみの Store」「日付と時刻」、08「remote の履歴が書き換えられたとき」「ミラー」「診断と修復」「保持日数を過ぎたものの自動の完全削除」
- 土台の計画 `docs/superpowers/plans/2026-11-12-hedgehug-foundation.md`(型・口・関数名の正本)

**持ち場:** `internal/domain/repair/`、`internal/app/sync/`・`internal/app/inbox/`・`internal/app/activity/`、`internal/infra/gitstore/` のうち `sync*.go`・`replicator*.go`・`filemerge/`、`internal/infra/statedb/`、`internal/adapter/httpapi/sync/`・`inbox/`・`activity/`・`events/`、`web/packages/core/features/sync/`・`inbox/`・`activity/`、`web/app/screens/inbox/`・`activity/`。

**待つもの:**
- C1: 土台 Task 5(`port.Stores`・`apptest`)、Task 6・7(`gitstore` の `Stores`・`lock`・`readLog`・`renderYAML`・`kindDirs`)と、トラック A の A1 Task 1(共有部分の PR。`item.Item` の `Watchers`、`apptest.Sample` の `number` の行、gitstore の `yaml.go`・`read.go` の変更)が develop に入っていること。C1 は A1 Task 1 の型をそのまま使い、同じ gitstore のファイルを触らない。
- トラック A の A1 Task 1 が足す共有部分(`port.Identity`・`apptest.Identity`、`port.ProjectReader.List`・`port.ItemReader.List`、`domain/refusal`・`httpapi/apierr`、`domain/numbering`・`domain/rank`、`httpapi.Deps` の `IDs`・`Identity`)は、C で別に定義しない。C2 以降の計画は、これらを名前と型のとおりに使う。
- C2 以降は「小計画の一覧」の「待つもの」の列。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。主なもの:

- モジュールは `github.com/kukv/hedgehug`、Go は `go 1.27.3`。依存の向きは設計書 2 節の表(archtest と depguard が確かめる)。外部ライブラリは infra・adapter・cmd だけ。
- 大きさの上限: Go のファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。超えたら分ける。
- 口は git の言葉(コミット、ブランチ、clone、push)を使わない。git の言葉は `infra/gitstore` の中だけ。
- 内部 ID は ULID。時刻はミリ秒までの RFC 3339(`change.TimeLayout`)、比べるときは UTC。
- 人でない実行者の行の `by` とコミットの作者は `hedgehug[bot]`(`change.BotLogin`)、メールは `bot@hedgehug.invalid`。
- 新しい依存は、版を明記して入れる(`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z`)。`@latest` や版なしでは入れない。版は公開から 7 日以上たったものを選ぶ。
- 共有部分(口、domain の中心の型、ui の基本部品、cmd の組み立て)を変えるときは、その変更だけの小さな PR を先に develop へ入れる(設計書 8 節)。

## この計画で決めたこと(仕様と設計書の規則の当てはめ)

| # | 決めたこと | 根拠 |
|---|---|---|
| 1 | トラックを C1〜C5 の 5 つの小計画に分ける(下の一覧)。コード入りで書くのは C1 だけで、C2 以降は前の小計画が develop に入ったときに書く | ユーザーの決定(案 1)。設計書 8 節「計画を書く時期」 |
| 2 | `port.Replicator` は `Sync(ctx, storeID, remote Remote) (SyncResult, error)` の 1 つだけを持つ。`Remote` は URL とトークン。トークンが空なら認証なし。トークンを取り出すのは app/sync(C2)で、D の `Secrets` から取る | 設計書 3 節(Replicator は git の版だけの口)。土台の計画の決定 1(C が Replicator を足す) |
| 3 | 「送信 N 件・受信 M 件」の N・M は、`Hedgehug-Op` を持つコミット(保存)の数。マージのコミットと、Store を作った最初のコミットは数えない | 06「同期の結果」の「N は送った保存の数」、保存形式「同期」の手順 5(マージは保存ではない) |
| 4 | 同期は、このマシンのいまのブランチと、同じ名前の remote のブランチを合わせる。remote が空でなく、その名前のブランチがないときは、共通の履歴がないものとして断る(`port.ErrNoCommonHistory`) | 06「同期のエラー」の「共通の履歴を持たない」。remote の別のブランチに勝手に合わせない |
| 5 | `log.jsonl` などを合わせるとき、形の違う行(`id` か `at` が読めない行)は結果に残さない。同じ `id` で中身が違う行は、バイト列として小さいほうを残す | 保存形式「いまの値の決め方」の手順 0(形の違う行は無いものとする)。どちらのマシンで合わせても同じ結果にするため |
| 6 | hedgehug が知らないファイル(GitHub 上で足した README など)は、片方だけが変えたらその版、両方が別に変えたら中身をバイト列として辞書順であとの版を取る | 保存形式「テスト」の「両方のマシンで同期した結果がバイト単位で同じ」。Store の値には影響しない |
| 7 | マージのコミットの作者とコミッターは `hedgehug[bot] <bot@hedgehug.invalid>`、メッセージは「remote と合わせる」で、`Hedgehug-Op` を持たない | 01「「あなた」の決まり方」(人でない保存は hedgehug[bot])、保存形式「同期」の手順 5 |
| 8 | 説明の「保存の古い版を先にする」は、その版の側で足された `desc` の行のうち、最後の行の `at` で比べる。同じなら、本文をバイト列として辞書順で前のほうを先にする | 06「同時に編集したとき」の説明の行、保存形式「同期」の手順 3 |
| 9 | 合わせたあとに `purged.jsonl` にあるスプリント・リリース・プロジェクト別のルールの行を取り除く処理(保存形式「同期」の手順 3 の 1 つ目)は C5 で作る。行の形はトラック B・F が決めるため | 設計書 3 節「先回りして足さない」 |
| 10 | 認証・接続・書き込みの拒否の見分け、60 秒止まったときの中断、ミラーへの送信、remote の履歴の書き換えの検出は C2 で作る。C1 は、履歴の関係・Store かどうか・形式の版・送信の重なりだけを見分ける | C2 で state.db(前に受け取った先端)と D のアカウントがそろうため |
| 11 | `infra/statedb`(SQLite を開く、表を作る仕組み)は C2 で作る。トラック E は同じパッケージに索引の表を足す | C が最初に state.db を使う。保存形式「このマシンに持つもの」 |
| 12 | 「remote の履歴にそろえる」(08)の操作と画面はトラック G。C2 は書き換えの検出と同期エラーまでを作り、のせ直しに使う合わせ方の関数(C1 の `mergeTrees`)を G に渡す | 08 の操作。06 は検出と同期エラーを定める |
| 13 | 06「同期のあとの完全な削除」は、C2 が「同期に成功した直後」に呼ぶ差し込み口を app/sync に置き、完全な削除の中身はトラック G が差し込む | 完全な削除は 08(G)。呼ぶ時機は 06 |
| 14 | 「Store を追加したときに取得した、それ以前の変更からは、通知を作らない」(06「通知」)は、Store を追加したときの先端を state.db に「判定済み」として覚えて実現する。覚える呼び出しは、Store を追加する G が行う | 06「通知」の判定済みの保存の記録と同じ仕組みで足りる |
| 15 | 「あなた」のログイン名は A の `port.Identity.Me`、断りの文言は A の `refusal.New` と `apierr.From`、番号の振り直しは A の `numbering.Next`・`numbering.Draft`、末尾への並べ替えは A の `rank.After` を使う。C で同じものを定義しない | トラック A の A1 Task 1(共有部分の PR)。設計書 8 節「共有部分の変更」 |

---

## 小計画の一覧

| 小計画 | 範囲(仕様 06 の節) | 待つもの | 完成の条件 |
|---|---|---|---|
| **C1 合わせ方**(この文書でコード入り) | 「同期とは」のうち受信・合わせる・送信。「同時に編集したとき」の合わせ方(項目ごとの後勝ちは土台の `change.Resolve`、複数の値、説明の行)。「同期のエラー」のうち、共通の履歴がない・Store でない・新しい版の形式・送信が重なり続けた(最大 3 回)。「完全な削除と重なったとき」のうちフォルダを消したほうを取ること。「別の Store へ移動したものと重なったとき」(移ったものの下に作られたフォルダも消す)。保存形式「テスト」のうち、2 つのクローンでの場面テスト | 土台 Task 5・6・7 | 2 つのクローンを同期すると、両方の Store のファイルがバイト単位で同じになる。合わせ方はどちらのマシンで行っても同じ。断るべき remote を断る |
| **C2 同期の実行と状態** | 「手動で同期する」「自動で同期する」「オフライン」「同期のエラー」の残り(アカウントなし・認証・接続・書き込みの拒否・remote の履歴の書き換え・60 秒の中断)とミラー、「同期の結果」「同期の状態の表示」(同期カード・Store ごとの同期状態・860px 以下の点)、「画面への反映」の 4 節(Events・SSE・反映を待つ条件)、「同期のあとの完全な削除」の呼び出し口。`infra/statedb` の土台 | C1、土台 Task 8・12、トラック A の A1 Task 1(`port.Identity`・`apierr`・`httpapi.Deps`)、トラック D の最初の小計画(アカウントと `Secrets`) | 画面の「すべて同期」と自動の同期で、2 台が同じ Store を共有できる。エラーの表のすべての行の文言が出る。別のタブの保存が 1 秒ほどで出る |
| **C3a 同期の修復(アイテム・プロジェクト)**(計画: `2026-11-14-hedgehug-track-c3a.md`) | 「修復の決まり」(順序の枠組みと、閲覧のみでは保存しない)、「同時に編集したとき」の削除と変更の重なり(削除の取り消し)、(1)「完全な削除と重なったとき」の残り(祖先が完全に削除されたもの、完全に削除された相手への関連)、(2)「親が削除されたとき」、(3)「番号の重なりの修復」、(4)「親が循環したとき」、(5)「親と別のワークスペース・プロジェクトにある子」の 2 つ目の箇条(プロジェクトだけが違う子)。`domain/repair` | C2、トラック A(アイテムの作成・移動・削除・関連。番号の振り直しは `numbering`、末尾への移動は `rank.After`、一覧は `ItemReader.List`・`ProjectReader.List`)、トラック B(ステータスと分類・既定のステータス) | 保存形式「同期の修復」の表の該当行を、2 つのクローンの場面テストで再現し、どちらのマシンで同期しても同じ値になる |
| **C3a-screen 他のタブで削除したものへの保存** | 「他の書き手との重なり」の削除の取り消しとトースト | C3a | 別のタブで削除したアイテム・プロジェクトへの保存で、削除が取り消され、トーストが出る |
| **C3c 他のタブで削除したコメント・設定の値への保存**(計画: `2026-11-14-hedgehug-track-c3c.md`) | 「他の書き手との重なり」の削除の取り消しとトーストの残り。自分のコメントの編集(削除済みのコメントに編集の行を書けば戻る)と、ワークスペースの設定の値(プロパティ・値・テンプレート・自動化のルール)を変える経路(`change.Undo` でワークスペースのログと、同じ op の行があるプロジェクト・アイテムのログを戻す)。取り消す経路と取り消さない経路、トーストの文、応答ヘッダー `Hedgehug-Restored-Name`・`Hedgehug-Restored-Comment` の符号化は、計画の表と決定 R1〜R10。別のタブで削除したスプリント・リリースへの保存(06「他の書き手との重なり」。スプリントの編集・開始・完了、リリースの編集・状態の変更)も入れる(ユーザーの決定、2026-11-14。F1 の計画)。削除は F1 の `delete` の行(`sprints/<ID>`・`releases/<ID>`)で、`change.Undo` で戻せる。削除済みのワークスペースの設定(名前・接頭辞・並び・プロパティ・値・ワークフロー・テンプレート・自動化・プロジェクト別のルール)への保存も入れる。変更を残し、ワークスペースの削除の op を、その op の行があるすべてのログで戻す(B5b の計画「他のトラックとの合わせ」。計画の決定 R11) | C3a-screen | 別のタブで削除したコメント・設定の値・ワークスペースへの保存で、削除が取り消され、トーストが出る |
| **C3b ワークスペースが違う子の修復** | (5)「親と別のワークスペース・プロジェクトにある子」の 1 つ目の箇条。行は自分で作らず、A5a の `internal/domain/transfer` を使う(値の引き直しは `transfer.Remap` に `statusChoice` を nil、`people` を nil で渡す。番号は `transfer.Number`。転送記録は `transfer.Record` を `Tx.WriteTransfers().Append` で書く。A5a の計画「持ち場と境目」) | C3a、A5a | 場面テストで、子が親のワークスペースの次の番号に移り、転送記録が残る |
| **C4 通知・受信箱・アクティビティ** | 「通知」の全節(通知を判定する単位、割り当て、メンション、ウォッチ、自動化、同期の修復)、「上書きの通知」(「自分の値に戻す」を含む)、「受信箱」の全節、「アクティビティ」の全節。`port.Inbox` | C2(statedb)、C3a(修復の通知の材料)、トラック A(担当者・`item.Item.Watchers`・コメント・説明、「あなた」の `port.Identity`)、トラック B(プロパティ名・自動化の実行) | 2 つのクローンの場面で、受け取った側の受信箱に 06 の文言どおりの行が 1 回だけ出る。アクティビティが 06 の表の文で並ぶ |
| **C5 設定・計画・GitHub に関わる修復と時計のずれ** | (6)「スプリントの実行中が 2 つになったとき」、(7)「完了したスプリントにアイテムが入ったとき」、(8)「存在しないスプリント・リリースを指すとき」、(9)「プロジェクトがなくなったときのルール」、(10)「削除された値を指す設定」「削除された値を指すプロジェクト別のルール」「外された初期ステータス・既定のステータス」(直さないことの確認)「完了・中止の分類にされた初期ステータス」、(11)「1 つしかないものが 2 つになったとき」(同期でワークスペースの削除を取り消したとき、その名前・接頭辞を新しいワークスペースがもう使っている場合を含む。B5b)。`purged.jsonl` の行の取り除き(決定 9)。スプリント・リリースの分(F1 が develop に入ったあと。F1 の計画の F1-24): 行はワークスペースの `log.jsonl` の `set`(`sprints/<ID>/<項目>`・`releases/<ID>/<項目>`)、削除の `delete`(`sprints/<ID>`・`releases/<ID>`)、完了の確定値の `close_sprint` で、`planning.IsPlanRow` で見分け、`planning.FromLog` で読む(削除済みは `Deleted`。06 のとおり、削除済みは存在するものとして扱う)。(6)・(7)・(8) と、(11) のスプリント・リリースの名前(比べ方は `planning.NormalizeName`)は、これで読んで直す。(8) の直しは、アイテムの `set sprint null`・`set release null`。`purged.jsonl` の `entity: sprint`・`release` の行の取り除きは、その ID の `planning.IsPlanRow` の行を消す。「時計のずれの警告」 | C3a・C4、トラック B(設定)、トラック F(スプリント・リリース)、トラック D(Issue のリンク、GitHub の応答の日時) | 保存形式「同期の修復」の表の残りの行を場面テストで再現する。時計のずれの文言が 3 か所に出る |

### 漏れの確認(仕様 06 の節 → 小計画)

| 06 の節 | 小計画 |
|---|---|
| 同期とは | C1(受信・合わせる・送信)、C2(ミラー) |
| 手動で同期する | C2 |
| 自動で同期する | C2 |
| オフライン | C2 |
| 同期のエラー | C1(共通の履歴・Store でない・新しい版・送信の重なり)、C2(残り) |
| 同期の結果 | C2(件数は C1 の `SyncResult`) |
| 同期の状態の表示 | C2 |
| 時計のずれの警告 | C5 |
| 画面への反映 / 反映するもの / 反映を待つとき / 反映で変わる画面 / 他の書き手との重なり | C2(他の書き手との重なりのトーストは、削除の取り消しと合わせて C3a-screen(アイテム・プロジェクト)と C3c(コメント・設定の値)) |
| 同時に編集したとき | C1(合わせ方)、C3a(削除と変更の重なり)、C4(上書きの通知) |
| 上書きの通知 | C4 |
| 番号の重なりの修復 | C3a |
| スプリントの実行中が 2 つになったとき | C5 |
| 1 つしかないものが 2 つになったとき | C5 |
| 削除された値を指す設定 | C5 |
| 削除された値を指すプロジェクト別のルール | C5 |
| プロジェクトがなくなったときのルール | C5 |
| 外された初期ステータス・既定のステータス | C5 |
| 完了・中止の分類にされた初期ステータス | C5 |
| 親が循環したとき | C3a |
| 親と別のワークスペース・プロジェクトにある子 | C3a(プロジェクト)、C3b(ワークスペース) |
| 親が削除されたとき | C3a |
| 完全な削除と重なったとき | C1(フォルダ)、C3a(祖先・関連) |
| 別の Store へ移動したものと重なったとき | C1(フォルダ)、C4(通知) |
| 完了したスプリントにアイテムが入ったとき | C5 |
| 存在しないスプリント・リリースを指すとき | C5 |
| 修復の決まり | C3a(枠組み)、C5 |
| 通知 / 通知を判定する単位 / 割り当て / メンション / ウォッチ / 自動化 / 同期の修復 | C4 |
| 受信箱 / タブと未読数 / 並びと行の表示 / 操作 / 件数 / 空のとき | C4 |
| アクティビティ / 対象 / 絞り込み / 表示 / 件数 | C4 |
| 同期のあとの完全な削除 | C2(呼び出し口)、G(中身) |
| 保存形式「テスト」(場面テスト、時計のずれ、親と一緒の削除、remote の履歴にそろえる) | C1(場面テストの土台・時計のずれ)、C3a(親と一緒の削除)、C2(書き換えの検出)、G(のせ直し) |

---

# C1 合わせ方

**Goal:** `infra/gitstore` に `port.Replicator` を実装し、2 つのクローンが同じ remote を通して、項目ごとの後勝ちで同じ中身にそろうようにする。

**Architecture:** `Replicator.Sync` は、Store のロックを持ったまま、受け取り(fetch)→ 合わせる → 送る(push)を、送信が重なって拒否されたら最大 3 回まで繰り返す。合わせ方は、3 つのツリー(共通の祖先・自分・remote)をファイルの中身の表(`tree`)にして、入出力のない `mergeTrees` で 1 つのツリーにし、作業ツリーに書いて 2 つの親を持つコミットにする。片方がもう片方の祖先なら、合わせずに進める(fast-forward)。

**C1 のファイル:**

```
internal/app/port/replicator.go               Replicator・Remote・SyncResult・エラー(共有部分。Task 1 を先に PR)
internal/infra/gitstore/filemerge/log.go      MergeLog(log.jsonl などの行の和)
internal/infra/gitstore/filemerge/text.go     MergeText(説明の行ごとの 3 方向マージ)
internal/infra/gitstore/filemerge/*_test.go
internal/infra/gitstore/sync_tree.go          tree と mergeTrees(フォルダの削除、ファイルごとの合わせ方)
internal/infra/gitstore/sync_tree_more.go     移ったものの下の削除、*.yaml の作り直し、説明の古い版
internal/infra/gitstore/sync_tree_test.go
internal/infra/gitstore/replicator.go         Replicator・Sync・1 回分の受信と合わせと送信
internal/infra/gitstore/sync_git.go           remote との通信、ツリーの読み書き、保存の数え方
internal/infra/gitstore/sync_helpers_test.go  テスト用の remote(プロセスの中の git サーバー)と 2 台のマシン
internal/infra/gitstore/sync_scenario_test.go 2 つのクローンの場面テスト
internal/infra/gitstore/sync_refuse_test.go   断る場面のテスト
```

---

### Task 1: Replicator の口(共有部分の小さな PR)

**Files:**
- Create: `internal/app/port/replicator.go`

**Interfaces:**
- Consumes: 土台の `port.StoreID`
- Produces: `port.Replicator`・`port.Remote{URL, Token string}`・`port.SyncResult{Sent, Received int}`・`port.ErrNoCommonHistory`・`port.ErrRemoteNotStore`・`port.ErrRemoteNewerFormat`・`port.ErrSendConflict`

- [ ] **Step 1: ブランチを切って口を書く**

```bash
git switch -c feat/track-c1-port origin/develop
```

`internal/app/port/replicator.go`:

```go
package port

import (
	"context"
	"errors"
)

// Replicator は、Store をほかの場所(remote)と合わせる口。git の版だけが持つ(設計書 3 節)。
// 受け取り、合わせ、送る。合わせたあとの修復は、app/sync が Stores.Write で保存する。
type Replicator interface {
	Sync(ctx context.Context, storeID StoreID, remote Remote) (SyncResult, error)
}

// Remote は合わせる相手。Token が空なら、認証なしで読み書きする。
type Remote struct {
	URL   string
	Token string
}

// SyncResult は、送った保存と受け取った保存の数(06「同期の結果」)。
type SyncResult struct {
	Sent     int
	Received int
}

// 同期を断る理由(06「同期のエラー」)。どれも、このマシンの Store を変えずに返す。
var (
	ErrNoCommonHistory   = errors.New("the remote has no history in common with the store")
	ErrRemoteNotStore    = errors.New("the remote is not a hedgehug store")
	ErrRemoteNewerFormat = errors.New("the remote was saved by a newer hedgehug")
	ErrSendConflict      = errors.New("sending kept overlapping with other updates")
)
```

- [ ] **Step 2: 確かめる**

Run: `make check`
Expected: すべて成功(archtest が、port が標準ライブラリだけを使っていることを確かめる)

- [ ] **Step 3: Commit と PR**

```bash
git add internal/app/port/replicator.go
git commit -m "feat(port): Store を remote と合わせる口(Replicator)を足す"
git push -u origin feat/track-c1-port
gh pr create --base develop --title "feat(port): Replicator の口を足す(トラック C1)" --body "トラック C の計画(docs/superpowers/plans/2026-11-12-hedgehug-track-c.md)の C1 Task 1。共有部分(口)の変更なので、先に入れる。"
```

この PR が develop に入ってから Task 2 以降を始める(Task 2・3 は口を使わないので、PR の確認を待つあいだに進めてよい)。

---

### Task 2: ファイルの行を合わせる規則(filemerge)

**Files:**
- Create: `internal/infra/gitstore/filemerge/log.go`、`internal/infra/gitstore/filemerge/text.go`
- Test: `internal/infra/gitstore/filemerge/log_test.go`、`internal/infra/gitstore/filemerge/text_test.go`

**Interfaces:**
- Consumes: なし(標準ライブラリだけ)
- Produces:
  - `filemerge.MergeLog(a, b []byte) []byte` — 2 つの版の行を `id` で重ねずに合わせ、`at`(UTC)・`id` の順に並べる。形の違う行は残さない(決定 5)
  - `filemerge.MergeText(base, older, newer string) string` — 行ごとの 3 方向マージ。同じ箇所を両方が変えたら `older`、`-----`、`newer` の順に両方を残す

- [ ] **Step 1: ブランチを切る**

```bash
git switch -c feat/track-c1-merge origin/develop
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/infra/gitstore/filemerge/log_test.go`:

```go
package filemerge

import "testing"

func TestMergeLogUnionsAndSorts(t *testing.T) {
	a := []byte(`{"id":"A1","at":"2026-11-12T10:00:00.000+09:00","t":"create"}
{"id":"A2","at":"2026-11-12T10:05:00.000+09:00","t":"set"}
`)
	b := []byte(`{"id":"A1","at":"2026-11-12T10:00:00.000+09:00","t":"create"}
{"id":"B1","at":"2026-11-12T01:03:00.000Z","t":"set"}
`)
	// B1 は UTC で 01:03 = JST 10:03 なので、A1 と A2 のあいだに入る。
	want := `{"id":"A1","at":"2026-11-12T10:00:00.000+09:00","t":"create"}
{"id":"B1","at":"2026-11-12T01:03:00.000Z","t":"set"}
{"id":"A2","at":"2026-11-12T10:05:00.000+09:00","t":"set"}
`
	if got := string(MergeLog(a, b)); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := string(MergeLog(b, a)); got != want {
		t.Errorf("not symmetric:\n%s", got)
	}
}

func TestMergeLogSameAtSortsByID(t *testing.T) {
	a := []byte(`{"id":"B","at":"2026-11-12T10:00:00.000Z"}` + "\n")
	b := []byte(`{"id":"A","at":"2026-11-12T10:00:00.000Z"}` + "\n")
	want := `{"id":"A","at":"2026-11-12T10:00:00.000Z"}` + "\n" + `{"id":"B","at":"2026-11-12T10:00:00.000Z"}` + "\n"
	if got := string(MergeLog(a, b)); got != want {
		t.Errorf("got\n%s", got)
	}
}

func TestMergeLogDropsMalformedLines(t *testing.T) {
	a := []byte("not json\n" + `{"id":"","at":"2026-11-12T10:00:00.000Z"}` + "\n" + `{"id":"X","at":"yesterday"}` + "\n" + `{"id":"OK","at":"2026-11-12T10:00:00.000Z"}` + "\n")
	want := `{"id":"OK","at":"2026-11-12T10:00:00.000Z"}` + "\n"
	if got := string(MergeLog(a, nil)); got != want {
		t.Errorf("got\n%s", got)
	}
}

func TestMergeLogSameIDDifferentBytesIsDeterministic(t *testing.T) {
	a := []byte(`{"id":"X","at":"2026-11-12T10:00:00.000Z","v":2}` + "\n")
	b := []byte(`{"id":"X","at":"2026-11-12T10:00:00.000Z","v":1}` + "\n")
	if string(MergeLog(a, b)) != string(MergeLog(b, a)) {
		t.Error("the result depends on the order")
	}
}

func TestMergeLogEmpty(t *testing.T) {
	if got := MergeLog(nil, nil); len(got) != 0 {
		t.Errorf("got %q", got)
	}
}
```

`internal/infra/gitstore/filemerge/text_test.go`:

```go
package filemerge

import "testing"

func TestMergeText(t *testing.T) {
	base := "a\nb\nc\n"
	cases := []struct {
		name, older, newer, want string
	}{
		{"only one side changed", base, "a\nb\nC\n", "a\nb\nC\n"},
		{"both changed to the same", "a\nB\nc\n", "a\nB\nc\n", "a\nB\nc\n"},
		{"different lines are both kept", "A\nb\nc\n", "a\nb\nC\n", "A\nb\nC\n"},
		{"same line keeps both, older first", "a\nX\nc\n", "a\nY\nc\n", "a\nX\n-----\nY\nc\n"},
		{"added lines at different places", "a\nb\nb2\nc\n", "a0\na\nb\nc\n", "a0\na\nb\nb2\nc\n"},
		{"one deletes a line, other edits another", "a\nc\n", "a\nb\nC\n", "a\nC\n"},
	}
	for _, c := range cases {
		if got := MergeText(base, c.older, c.newer); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMergeTextFromEmptyBase(t *testing.T) {
	// 説明のなかったものに、2 人が別々に書いた。同じ箇所なので両方を残す。
	if got := MergeText("", "x\n", "y\n"); got != "x\n-----\ny\n" {
		t.Errorf("got %q", got)
	}
}
```

- [ ] **Step 3: テストが落ちることを確かめる**

Run: `go test ./internal/infra/gitstore/filemerge/`
Expected: FAIL(`undefined: MergeLog`、`undefined: MergeText`)

- [ ] **Step 4: 実装する**

`internal/infra/gitstore/filemerge/log.go`:

```go
// Package filemerge は、Store のファイルを 2 つの版から合わせる規則を持つ(保存形式「同期」の手順 3)。
// 入出力をしない。どちらの版を a・b に渡しても同じ結果になる。
package filemerge

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"time"
)

type row struct {
	id   string
	at   time.Time
	line []byte
}

// MergeLog は、log.jsonl・transfers.jsonl・purged.jsonl の 2 つの版の行を、id で重ねずに合わせ、
// at(UTC)・id の順に並べる。形の違う行(id か at が読めない行)は残さない。
// 同じ id で中身が違う行は、バイト列として小さいほうを残す。
func MergeLog(a, b []byte) []byte {
	rows := map[string]row{}
	for _, src := range [][]byte{a, b} {
		for _, line := range bytes.Split(src, []byte("\n")) {
			r, ok := parseRow(line)
			if !ok {
				continue
			}
			if old, seen := rows[r.id]; !seen || bytes.Compare(r.line, old.line) < 0 {
				rows[r.id] = r
			}
		}
	}
	list := make([]row, 0, len(rows))
	for _, r := range rows {
		list = append(list, r)
	}
	slices.SortFunc(list, func(x, y row) int {
		if c := x.at.Compare(y.at); c != 0 {
			return c
		}
		return strings.Compare(x.id, y.id)
	})
	var out bytes.Buffer
	for _, r := range list {
		out.Write(r.line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

func parseRow(line []byte) (row, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return row{}, false
	}
	var v struct {
		ID string `json:"id"`
		At string `json:"at"`
	}
	if json.Unmarshal(line, &v) != nil || v.ID == "" {
		return row{}, false
	}
	at, err := time.Parse(time.RFC3339, v.At)
	if err != nil {
		return row{}, false
	}
	return row{id: v.ID, at: at, line: line}, true
}
```

`internal/infra/gitstore/filemerge/text.go`:

```go
package filemerge

import (
	"slices"
	"strings"
)

// conflictSeparator は、同じ箇所を両方が変えたときに、2 つの版のあいだにはさむ行(06「同時に編集したとき」)。
const conflictSeparator = "-----"

// MergeText は、base から older と newer に分かれた説明を、行ごとに合わせる。別々の行の変更は両方残す。
// 同じ箇所を両方が変えたときは、older の版、「-----」、newer の版の順に両方を残す。
func MergeText(base, older, newer string) string {
	switch {
	case older == newer, newer == base:
		return older
	case older == base:
		return newer
	}
	b, o, n := strings.Split(base, "\n"), strings.Split(older, "\n"), strings.Split(newer, "\n")
	mo, mn := matchLines(b, o), matchLines(b, n)
	var out []string
	bi, oi, ni := 0, 0, 0
	for {
		next := nextStable(mo, mn, bi, oi, ni)
		bEnd, oEnd, nEnd := len(b), len(o), len(n)
		if next >= 0 {
			bEnd, oEnd, nEnd = next, mo[next], mn[next]
		}
		out = append(out, mergeChunk(b[bi:bEnd], o[oi:oEnd], n[ni:nEnd])...)
		if next < 0 {
			return strings.Join(out, "\n")
		}
		out = append(out, b[next])
		bi, oi, ni = next+1, mo[next]+1, mn[next]+1
	}
}

// nextStable は、bi から先で、older と newer の両方で base と同じ位置に残っている行を探す。なければ -1。
func nextStable(mo, mn []int, bi, oi, ni int) int {
	for k := bi; k < len(mo); k++ {
		if mo[k] >= oi && mn[k] >= ni {
			return k
		}
	}
	return -1
}

// mergeChunk は、安定な行のあいだの 1 区間を合わせる。
func mergeChunk(base, o, n []string) []string {
	switch {
	case slices.Equal(o, n), slices.Equal(n, base):
		return o
	case slices.Equal(o, base):
		return n
	}
	// 説明のなかった末尾に両方が書いたときなど、空の行だけの版は無視しない。
	return slices.Concat(o, []string{conflictSeparator}, n)
}

// matchLines は、最長共通部分列で base の各行に対応する other の行の位置を返す(対応しなければ -1)。
func matchLines(base, other []string) []int {
	lcs := make([][]int, len(base)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(other)+1)
	}
	for i := len(base) - 1; i >= 0; i-- {
		for j := len(other) - 1; j >= 0; j-- {
			if base[i] == other[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	match := make([]int, len(base))
	i, j := 0, 0
	for i < len(base) && j < len(other) {
		switch {
		case base[i] == other[j]:
			match[i] = j
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			match[i] = -1
			i++
		default:
			j++
		}
	}
	for ; i < len(base); i++ {
		match[i] = -1
	}
	return match
}
```

`MergeTextFromEmptyBase` の場合、`base` は `[""]`、`older` は `["x", ""]`、`newer` は `["y", ""]` に分かれる。末尾の空の要素(最後の改行のあと)が安定な行になり、前の区間が `x`・`-----`・`y` になる。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test ./internal/infra/gitstore/filemerge/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件。`matchLines` の認知的複雑度が 15 を超えたら、表を作る部分を関数に分ける。

- [ ] **Step 6: Commit**

```bash
git add internal/infra/gitstore/filemerge
git commit -m "feat(gitstore): log.jsonl の行と説明の行を合わせる規則を足す"
```

---

### Task 3: ツリー全体の合わせ方(mergeTrees)

3 つのツリー(共通の祖先・自分・remote)から、合わせたツリーを作る。入出力をしないので、どちらのマシンで合わせても同じになることを、自分と remote を入れ替えたテストで確かめる。

**Files:**
- Create: `internal/infra/gitstore/sync_tree.go`、`internal/infra/gitstore/sync_tree_more.go`
- Test: `internal/infra/gitstore/sync_tree_test.go`

**Interfaces:**
- Consumes: Task 2 の `filemerge.MergeLog`・`MergeText`、土台の `renderYAML(kind change.Kind, log []change.Record) ([]byte, error)`・`kindDirs`・`item.FromLog`・`project.FromLog`
- Produces(パッケージの中):
  - `type tree map[string][]byte`(Store の根からの相対パス、区切りは `/`)
  - `mergeTrees(base, a, b tree) (tree, error)` — a と b を入れ替えても同じ結果。Task 4 と、トラック G の「remote の履歴にそろえる」が使う
  - `parseLog(b []byte) []change.Record`

**ファイルごとの合わせ方(保存形式「同期」の手順 3):**

| ファイル | 合わせ方 |
|---|---|
| `*/log.jsonl`、`transfers.jsonl`、`purged.jsonl` | `filemerge.MergeLog` |
| `items/<ID>/description.md`、`projects/<ID>/description.md` | `filemerge.MergeText`。古い版は決定 8 |
| エンティティの `*.yaml`(`members/members.yaml`・`workspaces/<ID>/workspace.yaml`・`projects/<ID>/project.yaml`・`items/<ID>/item.yaml`) | 合わせた `log.jsonl` から `renderYAML` で作り直す |
| そのほか(`.hedgehug/store.yaml` と、hedgehug が知らないファイル) | 片方だけが変えたらその版。両方が別に変えたら決定 6 |
| エンティティのフォルダ(`workspaces/<ID>`・`projects/<ID>`・`items/<ID>`) | 共通の祖先にあって、片方が消していたら消す。別の Store へ移ったもの(`transfers.jsonl` にあり、フォルダがない)の下のフォルダも消す |

- [ ] **Step 1: 失敗するテストを書く**

`internal/infra/gitstore/sync_tree_test.go`:

```go
package gitstore

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// line は、log.jsonl の 1 行を作る。
func line(t *testing.T, r change.Record) string {
	t.Helper()
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

func at(min int) time.Time { return time.Date(2026, 11, 12, 10, min, 0, 0, time.UTC) }

func itemCreate(t *testing.T, id string, parent *item.ID, when time.Time) string {
	t.Helper()
	it := item.Item{ID: item.ID(id), Place: item.Place{Workspace: "W1", Project: "P1", Parent: parent, Number: 1}, Kind: item.Task, Rank: "a", Title: "t"}
	if parent == nil {
		it.Kind = item.Epic
	}
	d := item.Create(it)
	return line(t, change.Record{ID: change.ID("c" + id), Op: "o", At: when, By: "taro", Actor: change.User("taro"), T: d.T, To: d.To})
}

func setTitle(t *testing.T, id, recID, to string, when time.Time, by string, prev ...change.ID) string {
	t.Helper()
	d := change.NewSet("title", "t", to)
	return line(t, change.Record{ID: change.ID(recID), Op: change.ID(recID), At: when, By: by, Actor: change.User(by), T: d.T, F: d.F, From: d.From, To: d.To, Prev: prev})
}

func mustMerge(t *testing.T, base, a, b tree) tree {
	t.Helper()
	got, err := mergeTrees(base, a, b)
	if err != nil {
		t.Fatal(err)
	}
	swapped, err := mergeTrees(base, b, a)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.EqualFunc(got, swapped, func(x, y []byte) bool { return string(x) == string(y) }) {
		t.Fatalf("the result depends on which side merges:\n%v\n%v", keys(got), keys(swapped))
	}
	return got
}

func keys(t tree) []string { return sortedKeys(t) }

func TestMergeTreesLaterTitleWinsAndYAMLIsRebuilt(t *testing.T) {
	create := itemCreate(t, "I1", nil, at(0))
	base := tree{"items/I1/log.jsonl": []byte(create)}
	a := tree{"items/I1/log.jsonl": []byte(create + setTitle(t, "I1", "rA", "A", at(1), "ann", "cI1"))}
	b := tree{"items/I1/log.jsonl": []byte(create + setTitle(t, "I1", "rB", "B", at(2), "bob", "cI1"))}
	got := mustMerge(t, base, a, b)
	if n := strings.Count(string(got["items/I1/log.jsonl"]), "\n"); n != 3 {
		t.Errorf("log has %d lines, want 3", n)
	}
	if y := string(got["items/I1/item.yaml"]); !strings.Contains(y, "title: B\n") {
		t.Errorf("item.yaml =\n%s", y)
	}
}

func TestMergeTreesDeletedFolderWins(t *testing.T) {
	create := itemCreate(t, "I1", nil, at(0))
	base := tree{"items/I1/log.jsonl": []byte(create), "items/I1/item.yaml": []byte("x")}
	a := tree{} // 完全に削除した
	b := tree{"items/I1/log.jsonl": []byte(create + setTitle(t, "I1", "rB", "B", at(2), "bob", "cI1"))}
	got := mustMerge(t, base, a, b)
	for p := range got {
		if strings.HasPrefix(p, "items/I1/") {
			t.Errorf("%s is left", p)
		}
	}
}

func TestMergeTreesNewFolderOnOneSideIsKept(t *testing.T) {
	a := tree{"items/I2/log.jsonl": []byte(itemCreate(t, "I2", nil, at(0)))}
	got := mustMerge(t, tree{}, a, tree{})
	if _, ok := got["items/I2/item.yaml"]; !ok {
		t.Errorf("new item lost: %v", keys(got))
	}
}

func TestMergeTreesDropsChildrenOfTransferred(t *testing.T) {
	// a は Epic E1 を別の Store へ移した(フォルダを消し、転送記録を足した)。b は同時に E1 の下に Task T1 を作り、
	// T1 の下に Step S1 を作った。T1 と S1 は反映しない(06「別の Store へ移動したものと重なったとき」)。
	epic := itemCreate(t, "E1", nil, at(0))
	e1 := item.ID("E1")
	t1 := item.ID("T1")
	base := tree{"items/E1/log.jsonl": []byte(epic)}
	a := tree{"transfers.jsonl": []byte(`{"id":"X1","op":"X1","at":"2026-11-12T10:05:00.000Z","by":"taro","target":"E1","old":"HH-1","new":"WEB-31","to_store":"S2","to_store_name":"team-b","to_remote":""}` + "\n")}
	b := tree{
		"items/E1/log.jsonl": []byte(epic),
		"items/T1/log.jsonl": []byte(itemCreate(t, "T1", &e1, at(3))),
		"items/S1/log.jsonl": []byte(itemCreate(t, "S1", &t1, at(4))),
	}
	got := mustMerge(t, base, a, b)
	for p := range got {
		if strings.HasPrefix(p, "items/") {
			t.Errorf("%s is left", p)
		}
	}
	if _, ok := got["transfers.jsonl"]; !ok {
		t.Error("transfers.jsonl lost")
	}
}

func TestMergeTreesSameStoreMoveKeepsChildren(t *testing.T) {
	// 同じ Store の別のワークスペースへの移動も転送記録を残すが、フォルダは残るので、子は消さない。
	epic := itemCreate(t, "E1", nil, at(0))
	e1 := item.ID("E1")
	transfer := []byte(`{"id":"X1","op":"X1","at":"2026-11-12T10:05:00.000Z","by":"taro","target":"E1","old":"HH-1","new":"WEB-31"}` + "\n")
	base := tree{"items/E1/log.jsonl": []byte(epic)}
	a := tree{"items/E1/log.jsonl": []byte(epic), "transfers.jsonl": transfer}
	b := tree{"items/E1/log.jsonl": []byte(epic), "items/T1/log.jsonl": []byte(itemCreate(t, "T1", &e1, at(3)))}
	if got := mustMerge(t, base, a, b); got["items/T1/log.jsonl"] == nil {
		t.Error("child of an item moved within the store was dropped")
	}
}

func TestMergeTreesDescription(t *testing.T) {
	desc := func(id string, when time.Time) string {
		return line(t, change.Record{ID: change.ID(id), Op: change.ID(id), At: when, By: "x", Actor: change.User("x"), T: "desc"})
	}
	create := itemCreate(t, "I1", nil, at(0))
	base := tree{"items/I1/log.jsonl": []byte(create), "items/I1/description.md": []byte("a\nb\n")}
	a := tree{"items/I1/log.jsonl": []byte(create + desc("dA", at(9))), "items/I1/description.md": []byte("a\nNEW-A\n")}
	b := tree{"items/I1/log.jsonl": []byte(create + desc("dB", at(5))), "items/I1/description.md": []byte("a\nNEW-B\n")}
	got := mustMerge(t, base, a, b)
	// b の保存のほうが古いので先に置く。
	if d := string(got["items/I1/description.md"]); d != "a\nNEW-B\n-----\nNEW-A\n" {
		t.Errorf("description = %q", d)
	}
}

func TestMergeTreesUnknownFiles(t *testing.T) {
	base := tree{"README.md": []byte("base")}
	if got := mustMerge(t, base, tree{"README.md": []byte("base")}, tree{"README.md": []byte("edited")}); string(got["README.md"]) != "edited" {
		t.Errorf("one side's edit lost: %q", got["README.md"])
	}
	if got := mustMerge(t, base, tree{"README.md": []byte("aaa")}, tree{"README.md": []byte("zzz")}); string(got["README.md"]) != "zzz" {
		t.Errorf("both edited: %q", got["README.md"])
	}
	if got := mustMerge(t, base, tree{}, tree{"README.md": []byte("base")}); got["README.md"] != nil {
		t.Errorf("one side's removal lost")
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/gitstore/ -run MergeTrees`
Expected: FAIL(`undefined: mergeTrees`、`undefined: tree`、`undefined: sortedKeys`)

- [ ] **Step 3: 実装する**

`internal/infra/gitstore/sync_tree.go`:

```go
package gitstore

import (
	"bytes"
	"encoding/json"
	"path"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// tree は、Store のファイル。キーは Store の根からの相対パス(区切りは /)、値は中身。
type tree map[string][]byte

// mergeTrees は、共通の祖先 base と 2 つの版 a・b を合わせる(保存形式「同期」の手順 3)。
// 入出力をせず、a と b を入れ替えても同じ結果になる。
func mergeTrees(base, a, b tree) (tree, error) {
	gone := removedEntities(base, a, b)
	out := tree{}
	for _, p := range sortedKeys(base, a, b) {
		if gone[entityDir(p)] || isEntityYAML(p) {
			continue
		}
		if v, ok := mergeFile(p, base, a, b); ok {
			out[p] = v
		}
	}
	for _, d := range underTransferred(out) {
		removeDir(out, d)
	}
	if err := rebuildYAML(out); err != nil {
		return nil, err
	}
	return out, nil
}

// entityDir は、p が入っているエンティティのフォルダ(items/<ID> など)。エンティティのファイルでなければ空。
func entityDir(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) >= 3 && (parts[0] == "items" || parts[0] == "projects" || parts[0] == "workspaces") {
		return parts[0] + "/" + parts[1]
	}
	return ""
}

// isEntityYAML は、log.jsonl から作り直す読む用の YAML か。
func isEntityYAML(p string) bool {
	dir, name := path.Split(p)
	for _, k := range kindDirs {
		if name != k.yaml {
			continue
		}
		if k.yaml == "members.yaml" {
			return dir == "members/"
		}
		return entityDir(p) != "" && strings.HasPrefix(dir, k.dir+"/")
	}
	return false
}

// removedEntities は、共通の祖先にあって、どちらかの版で消えたエンティティのフォルダ。
func removedEntities(base, a, b tree) map[string]bool {
	gone := map[string]bool{}
	for p := range base {
		d := entityDir(p)
		if d != "" && (!hasDir(a, d) || !hasDir(b, d)) {
			gone[d] = true
		}
	}
	return gone
}

func hasDir(t tree, d string) bool {
	for p := range t {
		if strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	return false
}

func removeDir(t tree, d string) {
	for p := range t {
		if strings.HasPrefix(p, d+"/") {
			delete(t, p)
		}
	}
}

// mergeFile は、1 つのファイルを合わせる。結果にファイルが残らなければ false。
func mergeFile(p string, base, a, b tree) ([]byte, bool) {
	name := path.Base(p)
	switch {
	case name == "log.jsonl" || p == "transfers.jsonl" || p == "purged.jsonl":
		_, inA := a[p]
		_, inB := b[p]
		if !inA && !inB {
			return nil, false
		}
		return filemerge.MergeLog(a[p], b[p]), true
	case name == "description.md" && entityDir(p) != "":
		return []byte(mergeDescription(p, base, a, b)), true
	}
	return mergeOther(p, base, a, b)
}

// mergeOther は、そのほかのファイルを合わせる(この計画の決定 6)。
func mergeOther(p string, base, a, b tree) ([]byte, bool) {
	vb, inBase := base[p]
	va, inA := a[p]
	vbb, inB := b[p]
	sameA := inA == inBase && bytes.Equal(va, vb)
	sameB := inB == inBase && bytes.Equal(vbb, vb)
	switch {
	case sameA:
		return vbb, inB
	case sameB:
		return va, inA
	case !inA:
		return vbb, inB
	case !inB:
		return va, true
	case bytes.Compare(va, vbb) >= 0:
		return va, true
	}
	return vbb, true
}

// sortedKeys は、ツリーのパスの和を、辞書順で返す。
func sortedKeys(ts ...tree) []string {
	seen := map[string]bool{}
	for _, t := range ts {
		for p := range t {
			seen[p] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// parseLog は、log.jsonl の中身を変更記録にする。形の違う行は読み飛ばす。
func parseLog(b []byte) []change.Record {
	var out []change.Record
	for _, l := range bytes.Split(b, []byte("\n")) {
		var r change.Record
		if len(bytes.TrimSpace(l)) > 0 && json.Unmarshal(l, &r) == nil {
			out = append(out, r)
		}
	}
	return out
}
```

`mergeOther` の「片方が消し、もう片方が変えた」場合(`!inA` で b が変えた)は、変えたほうを残す。hedgehug のファイルはフォルダ単位で消すので、ここに来るのは hedgehug が知らないファイルだけである。

`internal/infra/gitstore/sync_tree_more.go`:

```go
package gitstore

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/infra/gitstore/filemerge"
)

// descType は、説明を保存した変更記録の種類(保存形式「変更記録」の desc)。型はトラック A が足す。
const descType change.Type = "desc"

// mergeDescription は、説明を行ごとに合わせる。保存の古い版を先にする(この計画の決定 8)。
func mergeDescription(p string, base, a, b tree) string {
	logPath := entityDir(p) + "/log.jsonl"
	textA, textB := string(a[p]), string(b[p])
	atA := lastDesc(a[logPath], base[logPath])
	atB := lastDesc(b[logPath], base[logPath])
	aOlder := atA.Before(atB) || (atA.Equal(atB) && textA <= textB)
	if aOlder {
		return filemerge.MergeText(string(base[p]), textA, textB)
	}
	return filemerge.MergeText(string(base[p]), textB, textA)
}

// lastDesc は、side で足された(base にない)desc の行のうち、最後の時刻。
func lastDesc(side, base []byte) time.Time {
	inBase := map[change.ID]bool{}
	for _, r := range parseLog(base) {
		inBase[r.ID] = true
	}
	var last time.Time
	for _, r := range parseLog(side) {
		if r.T == descType && !inBase[r.ID] && r.At.After(last) {
			last = r.At
		}
	}
	return last
}

// underTransferred は、別の Store へ移ったもの(transfers.jsonl にあり、フォルダがない)の下にある
// エンティティのフォルダを返す(保存形式「同期」の手順 3 の最後)。消す前に全部を決める。
func underTransferred(t tree) []string {
	targets := transferTargets(t["transfers.jsonl"])
	if len(targets) == 0 {
		return nil
	}
	var out []string
	for _, p := range sortedKeys(t) {
		d := entityDir(p)
		if d == "" || !strings.HasSuffix(p, "/log.jsonl") {
			continue
		}
		for _, anc := range ancestors(t, d) {
			if targets[anc] && !hasDir(t, dirOfAncestor(t, anc)) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

func transferTargets(b []byte) map[string]bool {
	targets := map[string]bool{}
	for _, l := range bytes.Split(b, []byte("\n")) {
		var v struct {
			Target string `json:"target"`
		}
		if json.Unmarshal(l, &v) == nil && v.Target != "" {
			targets[v.Target] = true
		}
	}
	return targets
}

// ancestors は、エンティティの祖先の内部 ID(親のアイテムを辿り、プロジェクトとワークスペースまで)。
// 祖先のフォルダがなければ、そこで辿るのをやめる。
func ancestors(t tree, d string) []string {
	var out []string
	seen := map[string]bool{}
	for d != "" && !seen[d] {
		seen[d] = true
		log := parseLog(t[d+"/log.jsonl"])
		switch {
		case strings.HasPrefix(d, "items/"):
			it, ok := item.FromLog(log)
			if !ok {
				return out
			}
			out = append(out, string(it.Place.Workspace), string(it.Place.Project))
			if it.Place.Parent == nil {
				return out
			}
			out = append(out, string(*it.Place.Parent))
			d = "items/" + string(*it.Place.Parent)
		case strings.HasPrefix(d, "projects/"):
			if p, ok := project.FromLog(log); ok {
				out = append(out, string(p.Place.Workspace))
			}
			return out
		default:
			return out
		}
	}
	return out
}

// dirOfAncestor は、祖先の ID のフォルダ。アイテム・プロジェクト・ワークスペースのどれかにあればそのフォルダ、
// どれにもなければ、アイテムのフォルダの名前を返す(どれも無いので、hasDir は false になる)。
func dirOfAncestor(t tree, id string) string {
	for _, k := range []string{"items/", "projects/", "workspaces/"} {
		if hasDir(t, k+id) {
			return k + id
		}
	}
	return "items/" + id
}

// rebuildYAML は、合わせた log.jsonl から、読む用の YAML を作り直す。
func rebuildYAML(t tree) error {
	for _, p := range sortedKeys(t) {
		if !strings.HasSuffix(p, "/log.jsonl") {
			continue
		}
		dir := strings.TrimSuffix(p, "/log.jsonl")
		kind, ok := kindOfDir(dir)
		if !ok {
			continue
		}
		y, err := renderYAML(kind, parseLog(t[p]))
		if err != nil {
			return err
		}
		if y != nil {
			t[dir+"/"+kindDirs[kind].yaml] = y
		}
	}
	return nil
}

// kindOfDir は、フォルダ(members、items/<ID> など)の種類。
func kindOfDir(dir string) (change.Kind, bool) {
	top, _, _ := strings.Cut(dir, "/")
	for kind, k := range kindDirs {
		if k.dir == top && (kind == change.KindMembers) == (dir == "members") {
			return kind, true
		}
	}
	return "", false
}
```

`.hedgehug/log.jsonl`(保持日数の変更)も `log.jsonl` なので `MergeLog` で合わせる。`kindOfDir` が種類を返さないので、YAML は作り直さない(`store.yaml` を log から作るのはトラック G)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ -run MergeTrees && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS(gitstore のパッケージのファイルが 20 個以下、どのファイルも 300 行以下)

- [ ] **Step 5: Commit**

```bash
git add internal/infra/gitstore/sync_tree.go internal/infra/gitstore/sync_tree_more.go internal/infra/gitstore/sync_tree_test.go
git commit -m "feat(gitstore): 3 つのツリーから、どちらで合わせても同じツリーを作る"
```

---

### Task 4: Replicator(受け取り・合わせる・送る)と、2 つのクローンの場面テスト

**Files:**
- Create: `internal/infra/gitstore/replicator.go`、`internal/infra/gitstore/sync_git.go`
- Test: `internal/infra/gitstore/sync_helpers_test.go`、`internal/infra/gitstore/sync_scenario_test.go`、`internal/infra/gitstore/sync_refuse_test.go`

**Interfaces:**
- Consumes: Task 1 の `port.Replicator` ほか、Task 3 の `tree`・`mergeTrees`、土台の `Stores`・`lock`・`(*Stores).dir`・`readStoreYAML` の `storeYAML` 型・`Format`・`botName`・`botEmail`・`signature`
- Produces:
  - `gitstore.NewReplicator(s *Stores) *gitstore.Replicator`(`port.Replicator` を満たす)
  - パッケージの中: `(*Replicator).beforeSend func() error`(テスト用)、`readTree(repo, hash) (tree, error)`、`countSaves(repo, from, exclude) (int, error)`

**Sync の流れ:**

1. Store のロックを取る(Write と同じロック。合わせているあいだに保存が割り込まない)。
2. 最大 3 回、次を繰り返す(06「同期のエラー」)。
   1. remote の全ブランチを `refs/hedgehug/remote/*` に受け取る。remote が空なら、合わせずに送る(最初の送信)。
   2. remote に、いまのブランチと同じ名前のブランチがなければ `port.ErrNoCommonHistory`(決定 4)。
   3. remote の先端のツリーに `.hedgehug/store.yaml` がなければ `port.ErrRemoteNotStore`、`format` がこの hedgehug より新しければ `port.ErrRemoteNewerFormat`。どちらも Store を変えない。
   4. 自分と remote の先端の関係で分ける。同じか remote が自分の祖先なら合わせない。自分が remote の祖先なら、remote の先端へ進める。どちらでもなければ、共通の祖先(なければ `port.ErrNoCommonHistory`)から `mergeTrees` で合わせ、2 つの親を持つコミットにする(決定 7)。
   5. 送る。他の人の更新と重なって拒否されたら、1 に戻る。
3. 3 回とも拒否されたら `port.ErrSendConflict`。このとき、合わせたコミットはこのマシンに残る(受け取った保存は残り、次の同期で続ける。06「同期のエラー」)。

- [ ] **Step 1: テスト用の依存を入れる**

テストの、プロセスの中の git サーバーが `go-billy` を直接使う。

```bash
go get github.com/go-git/go-billy/v5@v5.9.2
```

- [ ] **Step 2: テストの道具を書く**

`internal/infra/gitstore/sync_helpers_test.go`:

```go
package gitstore

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// テストの remote は、https://remote.invalid/<名前> を、プロセスの中の git サーバーで、一時フォルダの
// 空のリポジトリ(bare)につなぐ。git の実行ファイルを使わない(CI の test-without-git でも動く)。
// 同期のテストは、ほかのテストと並べて走らせない。

var (
	remotesMu sync.Mutex
	remotes   = map[string]string{} // URL のパス → bare のフォルダ
	installed sync.Once
)

type remoteLoader struct{}

func (remoteLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	remotesMu.Lock()
	dir, ok := remotes[ep.Path]
	remotesMu.Unlock()
	if !ok {
		return nil, transport.ErrRepositoryNotFound
	}
	return filesystem.NewStorage(osfs.New(dir), cache.NewObjectLRUDefault()), nil
}

func newRemote(t *testing.T) port.Remote {
	t.Helper()
	installed.Do(func() { client.InstallProtocol("https", server.NewClient(remoteLoader{})) })
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, true); err != nil {
		t.Fatal(err)
	}
	p := "/" + filepath.Base(dir) + ".git"
	remotesMu.Lock()
	remotes[p] = dir
	remotesMu.Unlock()
	return port.Remote{URL: "https://remote.invalid" + p}
}

// letterIDs は、マシンごとに重ならない ID を順に返す。apptest.IDs は 2 台で同じ ID を返すので使わない。
type letterIDs struct {
	mu     sync.Mutex
	letter string
	n      int
}

func (g *letterIDs) New() change.ID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return change.ID(fmt.Sprintf("01%s%023d", g.letter, g.n))
}

// machine は、1 台のマシンの Store。
type machine struct {
	stores *Stores
	rep    *Replicator
	id     port.StoreID
	dir    string
	clock  *apptest.Clock
}

func newMachine(t *testing.T, dir, letter string, start time.Time) machine {
	t.Helper()
	clock := apptest.NewClock(start)
	s := New(clock, &letterIDs{letter: letter})
	id, err := s.Attach(dir)
	if err != nil {
		t.Fatal(err)
	}
	return machine{stores: s, rep: NewReplicator(s), id: id, dir: dir, clock: clock}
}

// firstMachine は、Store を作り、サンプルデータを保存して、空の remote へ最初に送ったマシン。
func firstMachine(t *testing.T, remote port.Remote) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a")
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	m := newMachine(t, dir, "A", t0)
	if err := apptest.Sample(t.Context(), m.stores, m.id); err != nil {
		t.Fatal(err)
	}
	if _, err := m.rep.Sync(t.Context(), m.id, remote); err != nil {
		t.Fatal(err)
	}
	return m
}

// cloneMachine は、remote を写して取り付けたマシン(Store の追加の仕組みはトラック G が作る)。
func cloneMachine(t *testing.T, remote port.Remote, letter string, start time.Time) machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), letter)
	if _, err := git.PlainClone(dir, false, &git.CloneOptions{URL: remote.URL}); err != nil {
		t.Fatal(err)
	}
	return newMachine(t, dir, letter, start)
}

func (m machine) sync(t *testing.T, remote port.Remote) port.SyncResult {
	t.Helper()
	res, err := m.rep.Sync(t.Context(), m.id, remote)
	if err != nil {
		t.Fatalf("sync of %s: %v", m.dir, err)
	}
	return res
}

func (m machine) write(t *testing.T, summary string, fn func(tx port.Tx) error) {
	t.Helper()
	if err := m.stores.Write(t.Context(), m.id, port.Operation{Actor: change.User("taro"), Summary: summary}, fn); err != nil {
		t.Fatal(err)
	}
}

// files は、作業ツリーのファイル(.git を除く)を読む。
func files(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// sameFiles は、2 台の Store のファイルがバイト単位で同じことを確かめる(保存形式「テスト」)。
func sameFiles(t *testing.T, a, b machine) {
	t.Helper()
	fa, fb := files(t, a.dir), files(t, b.dir)
	for p, v := range fa {
		if fb[p] != v {
			t.Errorf("%s differs:\n--- a\n%s\n--- b\n%s", p, v, fb[p])
		}
	}
	for p := range fb {
		if _, ok := fa[p]; !ok {
			t.Errorf("%s only on b", p)
		}
	}
}
```

- [ ] **Step 3: 失敗する場面テストを書く**

`internal/infra/gitstore/sync_scenario_test.go`:

```go
package gitstore

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
)

var _ port.Replicator = (*Replicator)(nil)

func title(t *testing.T, m machine, id item.ID) string {
	t.Helper()
	var got string
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		got = it.Title
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func set(field string, from, to any) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet(field, from, to))
	}
}

func TestSyncFirstSendAndClone(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	if got := title(t, b, apptest.SampleTask); got != "ログイン画面を作る" {
		t.Errorf("clone reads %q", got)
	}
	sameFiles(t, a, b)
	if res := b.sync(t, remote); res != (port.SyncResult{}) {
		t.Errorf("nothing to do, got %+v", res)
	}
}

func TestSyncCountsSaves(t *testing.T) {
	remote := newRemote(t)
	dir := t.TempDir() + "/a"
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	a := newMachine(t, dir, "A", t0)
	if err := apptest.Sample(t.Context(), a.stores, a.id); err != nil {
		t.Fatal(err)
	}
	// Store を作った最初のコミットは保存ではないので数えない(この計画の決定 3)。
	if res := a.sync(t, remote); res != (port.SyncResult{Sent: 1}) {
		t.Errorf("first send = %+v, want Sent 1", res)
	}
	b := cloneMachine(t, remote, "B", t0)
	a.write(t, "1", set("title", "", "x"))
	a.write(t, "2", set("title", "", "y"))
	a.sync(t, remote)
	if res := b.sync(t, remote); res != (port.SyncResult{Received: 2}) {
		t.Errorf("fast-forward = %+v, want Received 2", res)
	}
}

func TestSyncKeepsBothFieldsAndConverges(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "タイトル", set("title", "ログイン画面を作る", "ログイン"))
	b.write(t, "順位", set("rank", "a", "b"))
	a.sync(t, remote)
	// b は a の 1 件を受け取り、合わせて、自分の 1 件を送る。マージのコミットは数えない。
	if res := b.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("b sync = %+v", res)
	}
	a.sync(t, remote)
	sameFiles(t, a, b)
	if got := title(t, a, apptest.SampleTask); got != "ログイン" {
		t.Errorf("title = %q", got)
	}
	y := files(t, a.dir)["items/"+string(apptest.SampleTask)+"/item.yaml"]
	if !strings.Contains(y, "rank: b\n") || !strings.Contains(y, "title: ログイン\n") {
		t.Errorf("item.yaml =\n%s", y)
	}
}

func TestSyncLaterSaveWinsWhicheverSyncsFirst(t *testing.T) {
	for _, bFirst := range []bool{false, true} {
		remote := newRemote(t)
		a := firstMachine(t, remote)
		b := cloneMachine(t, remote, "B", t0.Add(time.Hour)) // b の時計のほうが進んでいる
		a.write(t, "a", set("title", "", "A"))
		b.write(t, "b", set("title", "", "B"))
		first, second := a, b
		if bFirst {
			first, second = b, a
		}
		first.sync(t, remote)
		second.sync(t, remote)
		first.sync(t, remote)
		sameFiles(t, a, b)
		if got := title(t, a, apptest.SampleTask); got != "B" {
			t.Errorf("bFirst=%v: title = %q, want B", bFirst, got)
		}
	}
}

func TestSyncSeenChangeWinsOverClock(t *testing.T) {
	// a と b が同時に書き、c が両方を見てから書いた。c の時計が最も遅れていても c が勝つ(保存形式「テスト」)。
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(2*time.Hour))
	c := cloneMachine(t, remote, "C", t0.Add(-time.Hour))
	a.write(t, "a", set("title", "", "A"))
	b.write(t, "b", set("title", "", "B"))
	a.sync(t, remote)
	b.sync(t, remote)
	c.sync(t, remote)
	c.write(t, "c", set("title", "B", "C"))
	c.sync(t, remote)
	a.sync(t, remote)
	b.sync(t, remote)
	for _, m := range []machine{a, b, c} {
		if got := title(t, m, apptest.SampleTask); got != "C" {
			t.Errorf("%s: title = %q, want C", m.dir, got)
		}
	}
	sameFiles(t, a, b)
	sameFiles(t, a, c)
}

func TestSyncKeepsBothAddsOfMultiValue(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("jiro")) })
	b.write(t, "b", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("saburo")) })
	a.sync(t, remote)
	b.sync(t, remote)
	a.sync(t, remote)
	sameFiles(t, a, b)
	var logins []string
	err := a.stores.Read(t.Context(), a.id, func(r port.Reader) error {
		ms, err := r.Members().List()
		for _, m := range ms {
			logins = append(logins, m.Login)
		}
		return err
	})
	if err != nil || strings.Join(logins, ",") != "hanako,jiro,saburo,taro" {
		t.Errorf("members = %v, %v", logins, err)
	}
}

func TestSyncRetriesWhenSendOverlaps(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", set("title", "", "A"))
	once := false
	a.rep.beforeSend = func() error {
		if !once {
			once = true
			b.write(t, "b", set("rank", "a", "z"))
			b.sync(t, remote)
		}
		return nil
	}
	if res := a.sync(t, remote); res.Received != 1 {
		t.Errorf("a sync = %+v, want the overlapping save received", res)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}

func TestSyncGivesUpAfterThreeOverlaps(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", set("title", "", "A"))
	n := 0
	a.rep.beforeSend = func() error {
		n++
		b.write(t, "b", set("rank", "", string(rune('a'+n))))
		b.sync(t, remote)
		return nil
	}
	_, err := a.rep.Sync(t.Context(), a.id, remote)
	if !errors.Is(err, port.ErrSendConflict) || n != 3 {
		t.Errorf("err = %v after %d tries, want ErrSendConflict after 3", err, n)
	}
	// 受け取って合わせたものは残り、次の同期で続ける。
	a.rep.beforeSend = nil
	a.sync(t, remote)
	b.sync(t, remote)
	sameFiles(t, a, b)
}
```

`internal/infra/gitstore/sync_refuse_test.go`:

```go
package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/kukv/hedgehug/internal/app/port"
)

// pushRaw は、hedgehug を通さずに、ファイルを書いたコミットを remote へ送る(利用者が git で行った操作の代わり)。
func pushRaw(t *testing.T, remote port.Remote, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := repo.Worktree()
	for p, v := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	sig := &object.Signature{Name: "someone", Email: "someone@example.invalid", When: t0}
	if _, err := wt.Commit("raw", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
	rm := git.NewRemote(repo.Storer, &config.RemoteConfig{Name: "raw", URLs: []string{remote.URL}})
	if err := rm.Push(&git.PushOptions{RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"}}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncRefusesRemoteThatIsNotAStore(t *testing.T) {
	remote := newRemote(t)
	pushRaw(t, remote, map[string]string{"README.md": "hello"})
	_, _, dir := newStore(t)
	m := newMachine(t, dir, "A", t0)
	before := files(t, dir)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrRemoteNotStore) {
		t.Errorf("err = %v", err)
	}
	if len(files(t, dir)) != len(before) {
		t.Error("the store changed")
	}
}

func TestSyncRefusesUnrelatedHistory(t *testing.T) {
	remote := newRemote(t)
	firstMachine(t, remote)
	_, _, dir := newStore(t) // 同じ Store ID でも、別に作った Store は履歴を共有しない
	m := newMachine(t, dir, "Z", t0)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrNoCommonHistory) {
		t.Errorf("err = %v", err)
	}
}

func TestSyncRefusesNewerFormat(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	// b が、新しい版の hedgehug で保存したことにする。
	if err := os.WriteFile(filepath.Join(b.dir, ".hedgehug", "store.yaml"), []byte("store: 01STORE0000000000000000001\nformat: 2\nretention_days: 30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, _ := git.PlainOpen(b.dir)
	wt, _ := repo.Worktree()
	if _, err := wt.Add(".hedgehug/store.yaml"); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "newer", Email: "newer@hedgehug.invalid", When: t0}
	if _, err := wt.Commit("newer", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
	b.sync(t, remote)
	before := files(t, a.dir)
	if _, err := a.rep.Sync(t.Context(), a.id, remote); !errors.Is(err, port.ErrRemoteNewerFormat) {
		t.Errorf("err = %v", err)
	}
	if after := files(t, a.dir); after[".hedgehug/store.yaml"] != before[".hedgehug/store.yaml"] {
		t.Error("the store changed")
	}
}

func TestSyncRefusesRemoteWithoutTheBranch(t *testing.T) {
	remote := newRemote(t)
	repo := t.TempDir()
	r, _ := git.PlainInit(repo, false)
	wt, _ := r.Worktree()
	sig := &object.Signature{Name: "x", Email: "x@example.invalid", When: t0}
	if _, err := wt.Commit("empty", &git.CommitOptions{Author: sig, Committer: sig, AllowEmptyCommits: true}); err != nil {
		t.Fatal(err)
	}
	rm := git.NewRemote(r.Storer, &config.RemoteConfig{Name: "raw", URLs: []string{remote.URL}})
	if err := rm.Push(&git.PushOptions{RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/other"}}); err != nil {
		t.Fatal(err)
	}
	_, _, dir := newStore(t)
	m := newMachine(t, dir, "A", t0)
	if _, err := m.rep.Sync(t.Context(), m.id, remote); !errors.Is(err, port.ErrNoCommonHistory) {
		t.Errorf("err = %v", err)
	}
}
```

`newStore` は土台の `create_test.go` の道具で、`Create` と `Attach` を済ませた Store を返す。`newMachine` で同じフォルダをもう一度取り付けると、マシンの時計と ID を差し替えられる。

- [ ] **Step 4: テストが落ちることを確かめる**

Run: `go test ./internal/infra/gitstore/ -run Sync`
Expected: FAIL(`undefined: Replicator`、`undefined: NewReplicator`)

- [ ] **Step 5: remote との通信とツリーの読み書きを書く**

`internal/infra/gitstore/sync_git.go`:

```go
package gitstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/port"
)

// remotePrefix は、受け取った remote のブランチを置く場所。利用者の git の remote の設定には触らない。
const remotePrefix = "refs/hedgehug/remote/"

// errRejected は、送信が他の人の更新と重なって拒否されたこと。Sync が受け取りからやり直す。
var errRejected = errors.New("rejected: the remote moved")

type link struct {
	remote *git.Remote
	auth   transport.AuthMethod
}

func newLink(repo *git.Repository, rm port.Remote) link {
	r := git.NewRemote(repo.Storer, &config.RemoteConfig{Name: "hedgehug", URLs: []string{rm.URL}})
	var auth transport.AuthMethod
	if rm.Token != "" {
		auth = &githttp.BasicAuth{Username: "x-access-token", Password: rm.Token}
	}
	return link{remote: r, auth: auth}
}

// fetch は、remote の全ブランチを受け取り、branch と同じ名前のブランチの先端を返す。
// remote が空なら、ゼロのハッシュと nil。remote が空でなく、そのブランチがなければ port.ErrNoCommonHistory。
func (l link) fetch(ctx context.Context, repo *git.Repository, branch plumbing.ReferenceName) (plumbing.Hash, error) {
	err := l.remote.FetchContext(ctx, &git.FetchOptions{
		RefSpecs: []config.RefSpec{config.RefSpec("+refs/heads/*:" + remotePrefix + "*")},
		Auth:     l.auth,
		Force:    true,
	})
	switch {
	case errors.Is(err, transport.ErrEmptyRemoteRepository):
		return plumbing.ZeroHash, nil
	case err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate):
		return plumbing.ZeroHash, err
	}
	ref, err := repo.Reference(plumbing.ReferenceName(remotePrefix+branch.Short()), true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.ZeroHash, port.ErrNoCommonHistory
	}
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return ref.Hash(), nil
}

// send は、branch を remote の同じ名前のブランチへ送る。履歴を書き換えない(強制しない)。
func (l link) send(ctx context.Context, branch plumbing.ReferenceName) error {
	spec := config.RefSpec(branch.String() + ":" + branch.String())
	err := l.remote.PushContext(ctx, &git.PushOptions{RefSpecs: []config.RefSpec{spec}, Auth: l.auth})
	switch {
	case err == nil, errors.Is(err, git.NoErrAlreadyUpToDate):
		return nil
	case errors.Is(err, git.ErrNonFastForwardUpdate), strings.Contains(err.Error(), "non-fast-forward"):
		return errRejected
	}
	return err
}

// checkRemoteStore は、remote の先端が hedgehug の Store で、この hedgehug が読める形式かを確かめる。
func checkRemoteStore(repo *git.Repository, h plumbing.Hash) error {
	c, err := repo.CommitObject(h)
	if err != nil {
		return err
	}
	f, err := c.File(storeFile)
	if errors.Is(err, object.ErrFileNotFound) {
		return port.ErrRemoteNotStore
	}
	if err != nil {
		return err
	}
	text, err := f.Contents()
	if err != nil {
		return err
	}
	var y storeYAML
	if yaml.Unmarshal([]byte(text), &y) != nil || y.Store == "" {
		return port.ErrRemoteNotStore
	}
	if y.Format > Format {
		return port.ErrRemoteNewerFormat
	}
	return nil
}

// readTree は、コミットのファイルをすべて読む。ゼロのハッシュなら空。
func readTree(repo *git.Repository, h plumbing.Hash) (tree, error) {
	out := tree{}
	if h.IsZero() {
		return out, nil
	}
	c, err := repo.CommitObject(h)
	if err != nil {
		return nil, err
	}
	files, err := c.Files()
	if err != nil {
		return nil, err
	}
	err = files.ForEach(func(f *object.File) error {
		r, err := f.Reader()
		if err != nil {
			return err
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		out[f.Name] = b
		return err
	})
	return out, err
}

// writeTree は、作業ツリーを from から to の中身にする。新しく作ったファイルを返す(失敗したら消すため)。
func writeTree(root string, from, to tree) (created []string, err error) {
	for p := range from {
		if _, keep := to[p]; !keep {
			if err := os.Remove(filepath.Join(root, p)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return created, err
			}
			_ = os.Remove(filepath.Dir(filepath.Join(root, p))) // 空になったフォルダだけが消える
		}
	}
	for _, p := range sortedKeys(to) {
		if old, ok := from[p]; ok && string(old) == string(to[p]) {
			continue
		}
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return created, err
		}
		if _, ok := from[p]; !ok {
			created = append(created, p)
		}
		if err := os.WriteFile(full, to[p], 0o644); err != nil {
			return created, err
		}
	}
	return created, nil
}

// countSaves は、from から辿れて exclude から辿れない保存(Hedgehug-Op を持つコミット)の数。
func countSaves(repo *git.Repository, from, exclude plumbing.Hash) (int, error) {
	if from.IsZero() {
		return 0, nil
	}
	seen := map[plumbing.Hash]bool{}
	if !exclude.IsZero() {
		if err := walk(repo, exclude, func(c *object.Commit) { seen[c.Hash] = true }); err != nil {
			return 0, err
		}
	}
	n := 0
	err := walk(repo, from, func(c *object.Commit) {
		if !seen[c.Hash] && strings.Contains(c.Message, "\nHedgehug-Op: ") {
			n++
		}
	})
	return n, err
}

func walk(repo *git.Repository, from plumbing.Hash, fn func(c *object.Commit)) error {
	iter, err := repo.Log(&git.LogOptions{From: from})
	if err != nil {
		return err
	}
	return iter.ForEach(func(c *object.Commit) error {
		fn(c)
		return nil
	})
}
```

- [ ] **Step 6: Replicator を書く**

`internal/infra/gitstore/replicator.go`:

```go
package gitstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// maxSendTries は、送信が他の人の更新と重なったときに、受け取りからやり直す最大の回数(06「同期のエラー」)。
const maxSendTries = 3

// Replicator は、gitstore の Store を remote と合わせる(port.Replicator)。
type Replicator struct {
	stores *Stores

	// beforeSend は、送る直前に呼ぶ。テストで、送信が他の人の更新と重なる場面を作るためのもの。
	beforeSend func() error
}

func NewReplicator(s *Stores) *Replicator { return &Replicator{stores: s} }

// Sync は、受け取り、合わせ、送る。合わせているあいだは、Store のロックを持つ。
func (r *Replicator) Sync(ctx context.Context, id port.StoreID, remote port.Remote) (port.SyncResult, error) {
	root, err := r.stores.dir(id)
	if err != nil {
		return port.SyncResult{}, err
	}
	unlock, err := lock(root)
	if err != nil {
		return port.SyncResult{}, err
	}
	defer unlock()
	repo, err := git.PlainOpen(root)
	if err != nil {
		return port.SyncResult{}, err
	}
	head, err := repo.Head()
	if err != nil {
		return port.SyncResult{}, err
	}
	s := syncRun{r: r, repo: repo, root: root, link: newLink(repo, remote), branch: head.Name(), start: head.Hash()}
	for range maxSendTries {
		res, err := s.once(ctx)
		if errors.Is(err, errRejected) {
			continue
		}
		return res, err
	}
	return port.SyncResult{}, port.ErrSendConflict
}

// syncRun は、1 回の Sync の状態。start は、同期を始めたときの自分の先端(受け取った数を数える基準)。
type syncRun struct {
	r      *Replicator
	repo   *git.Repository
	root   string
	link   link
	branch plumbing.ReferenceName
	start  plumbing.Hash
}

// once は、受け取り・合わせる・送るを 1 回行う。送信が拒否されたら errRejected。
func (s syncRun) once(ctx context.Context) (port.SyncResult, error) {
	theirs, err := s.link.fetch(ctx, s.repo, s.branch)
	if err != nil {
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
	if s.r.beforeSend != nil {
		if err := s.r.beforeSend(); err != nil {
			return port.SyncResult{}, err
		}
	}
	if err := s.link.send(ctx, s.branch); err != nil {
		return port.SyncResult{}, err
	}
	return s.result(theirs)
}

// integrate は、自分の先端に remote の先端 theirs を取り込む。
func (s syncRun) integrate(theirs plumbing.Hash) error {
	head, err := s.repo.Head()
	if err != nil {
		return err
	}
	ours := head.Hash()
	oc, err := s.repo.CommitObject(ours)
	if err != nil {
		return err
	}
	tc, err := s.repo.CommitObject(theirs)
	if err != nil {
		return err
	}
	if ours == theirs {
		return nil
	}
	if behind, err := tc.IsAncestor(oc); err != nil || behind {
		return err
	}
	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}
	if ahead, err := oc.IsAncestor(tc); err != nil {
		return err
	} else if ahead {
		return wt.Reset(&git.ResetOptions{Commit: theirs, Mode: git.HardReset})
	}
	bases, err := oc.MergeBase(tc)
	if err != nil {
		return err
	}
	if len(bases) == 0 {
		return port.ErrNoCommonHistory
	}
	return s.merge(wt, bases[0].Hash, ours, theirs)
}

// merge は、3 つのツリーを合わせて作業ツリーに書き、2 つの親を持つコミットにする。失敗したら元に戻す。
func (s syncRun) merge(wt *git.Worktree, base, ours, theirs plumbing.Hash) error {
	trees := make([]tree, 3)
	for i, h := range []plumbing.Hash{base, ours, theirs} {
		t, err := readTree(s.repo, h)
		if err != nil {
			return err
		}
		trees[i] = t
	}
	merged, err := mergeTrees(trees[0], trees[1], trees[2])
	if err != nil {
		return err
	}
	created, err := writeTree(s.root, trees[1], merged)
	if err == nil {
		err = wt.AddWithOptions(&git.AddOptions{All: true})
	}
	if err == nil {
		sig := signature(botName, botEmail, s.r.stores.clock.Now())
		_, err = wt.Commit("remote と合わせる\n", &git.CommitOptions{Author: sig, Committer: sig, Parents: []plumbing.Hash{ours, theirs}})
	}
	if err != nil {
		errs := []error{err, wt.Reset(&git.ResetOptions{Commit: ours, Mode: git.HardReset})}
		for _, p := range created {
			errs = append(errs, os.Remove(filepath.Join(s.root, p)))
		}
		return errors.Join(errs...)
	}
	return nil
}

// result は、送った保存(remote の先端 theirs になかったもの)と、受け取った保存(始めたときの自分になかったもの)を数える。
func (s syncRun) result(theirs plumbing.Hash) (port.SyncResult, error) {
	head, err := s.repo.Head()
	if err != nil {
		return port.SyncResult{}, err
	}
	sent, err := countSaves(s.repo, head.Hash(), theirs)
	if err != nil {
		return port.SyncResult{}, err
	}
	received, err := countSaves(s.repo, theirs, s.start)
	if err != nil {
		return port.SyncResult{}, err
	}
	return port.SyncResult{Sent: sent, Received: received}, nil
}
```

`integrate` の中の `if behind, err := …; err != nil || behind { return err }` は、remote が自分の祖先(受け取るものがない)なら何もせず、比べられなければエラーを返す。

- [ ] **Step 7: テストが通ることを確かめる**

Run: `go test -race ./internal/infra/gitstore/`
Expected: PASS(土台の照合テスト・gitstore のテストと、C1 の場面テスト・断る場面のテスト)

うまくいかないときに確かめること:
- `wt.Reset` の HardReset が、remote の先端で増えたファイルを作業ツリーに出さない、または消えたファイルを残すときは、`integrate` の進める処理を `writeTree(s.root, readTree(ours), readTree(theirs))` のあとに `Reset` する形にする(`writeTree` で中身をそろえ、`Reset` でブランチとインデックスを動かす)。
- `AddWithOptions{All: true}` が消したファイルをインデックスから外さないときは、`writeTree` で消したパスを返し、それぞれに `wt.Remove(p)` を呼ぶ。
- 送信の拒否が `git.ErrNonFastForwardUpdate` でなく別の文で返るときは、`send` の見分けにその文を足す(前回の実装も `non-fast-forward` の文字で見分けていた)。

- [ ] **Step 8: lint・大きさ・git なしでのテストを確かめる**

Run: `make check && PATH="$(dirname "$(command -v go)"):$(dirname "$(command -v gotestsum)")" CGO_ENABLED=0 go test -count=1 ./internal/infra/gitstore/`
Expected: すべて成功。git の実行ファイルが PATH になくても同期のテストが通る(テストの remote はプロセスの中の git サーバー)。`replicator.go`・`sync_git.go` が 300 行を超えたら、`merge` と `result` を `replicator_merge.go` に分ける。

- [ ] **Step 9: Commit と PR**

```bash
git add go.mod go.sum internal/infra/gitstore
git commit -m "feat(gitstore): 受け取り・合わせる・送るの同期と、2 つのクローンの場面テストを足す"
git push -u origin feat/track-c1-merge
gh pr create --base develop --title "feat(gitstore): 同期の合わせ方(トラック C1)" --body "トラック C の計画の C1 Task 2〜4。2 つのクローンを同期すると、両方の Store のファイルがバイト単位で同じになる。"
```

---

## C1 のあと

- C1 が develop に入ったら、C2 の計画をこの文書に足す(コード入り)。C2 を書く前に、トラック D の最初の小計画(アカウントと `Secrets`)の口の名前を確かめる。
- C2 で `cmd/hedgehug` の組み立てに `gitstore.NewReplicator` を差し込む(共有部分なので小さな PR)。

## F に求めること

トラック F(スプリント・リリース)の計画を書くときに、次のことを入れる。C1 のあとの作業ではない。

- (F1 で済んだ。F1 の `domain/planning` が `delete` の行で書き、`change.DeletionOfField` で `change.Restored` を使って読む)スプリントとリリースの削除は、`delete` の行で書く(`f` は `sprints/<ID>`・`releases/<ID>` のような形)。読むときは `change.Restored` を使って、取り消された削除を除く。こうすると、C3a の「削除と変更の重なり」の取り消し(`change.Undo`)が、スプリント・リリースにもそのまま効く。
- コメントと作業時間は、`change.Current` が生きている先端を取るので、編集と削除が重なっても編集が残る。修復は要らない。

## 未決の点

| # | 点 | 推奨案 |
|---|---|---|
| 1 | 新しく作る Store のブランチの名前。土台の `gitstore.Create` は go-git の既定(`master`)で作るが、GitHub で作ったリポジトリの既定は `main`。決定 4 では、名前の違う remote と合わせない(空のリポジトリへの最初の送信は通る) | **決定(2026-11-12): `main`。** 土台の計画の Task 6 を直した |
| 2 | 時計のずれの警告(06「時計のずれの警告」)が使う「GitHub の応答に付く日時」を、どの口で受け取るか。GitHub の口はトラック D が定める | D の `GitHub` の口の各呼び出しの結果に、応答の日時を含める。C5 がそれを受け取って比べる |
| 3 | 自動化の通知(06「自動化」)を作る場所。自動化を動かすのはトラック B、通知を持つのは C4 の `Inbox` | C4 で `port.Inbox` を定め、B の自動化の実行が `Inbox` に書く。C4 が先に入るまで、B は通知を作らない(B と C4 の計画のどちらにも書く) |
