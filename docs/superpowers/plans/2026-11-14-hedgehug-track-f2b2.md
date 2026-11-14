# hedgehug トラック F2b2(一括変更)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 04「一括変更」を、検索のテーブル(トラック E)でも使える共通の部品として作り、バックログの選択バーに「一括変更…」を足す。ダイアログ「選択したアイテムを一括変更」・確認・Store ごとの保存・自動化・結果のトースト・バックログのトーストの末尾まで。

**Architecture:** サーバーは新しいパッケージ `internal/app/bulkchange`(`app/items` は 20 ファイルで満杯。ステータスの検査・自動化は `items`、スプリント・リリースは `planning` にあり、app の機能どうしは import できないので、domain と port だけを使う新しい機能にする。自動化を items の外で動かす前例は `app/githublink/fire.go`)。1 件ごとの判定は 1 つの純粋な関数 `judge` で、選択肢・予告・実行の 3 つの口が使う。API は `internal/adapter/httpapi/bulkchange`。画面は core の `features/selection/` に `bulkChangeForm.ts`(項目・選択肢・文)・`bulkChangeResult.ts`(結果の文)・`bulkChange.ts`(流れ)を足し、ui の `SelectionBar` に「一括変更…」を足し、app にダイアログを置く。バックログは F2b1 の選択(`backlog.selection`)をそのまま渡し、トーストの末尾だけを差し込む。

**Tech Stack:** Go 1.27.3、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`「一括選択と一括変更・一括削除」の冒頭・「選択」・「一括変更」(816〜897 行)、アクティビティの表の「一括変更」(970 行)、比較の規則(174・987 行)、「モックからの変更点」の 1035〜1038・1054・1055 行
- 05 の 30 行(バックログの一括変更のトーストの末尾)・49 行(スプリントに移すときは枠の末尾に、いまの順のまま続ける)・56 行(完了したスプリントは選択肢に出さない)
- 01 の 106・108 行(Store ごとに 1 回ずつ保存。一括変更は変えられたものだけを保存し、Store ごとの保存は互いに独立)
- 03 の 65 行(値の名前は大文字と小文字を区別せずに比べる)、「実行の規則」(511〜521 行。一括変更はアイテムごとにルールを判定。担当者を非表示にしただけなら値は使い続ける)、「制約で止まったとき」(525 行)
- 06 の 352〜392 行(割り当ては受け取った側が作る。自動化の通知は変えたアイテムごとに 1 件)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-f2b1.md` の末尾「F2b2 に渡すもの」「E に求めること」

## 始める前に

- F2b は 2 本に分けた(ユーザーの決定、2026-11-14): F2b1 = 行の選択のチェック・選択バー・一括削除・パレットの「選んでいるアイテム」(develop に入った、PR #217)、**F2b2 = この計画**。1 計画・1 PR。
- パレットの「{N} 件をまとめて変更…」(検索のテーブルを開いてすべて選び、このダイアログを開く)は、テーブルを作る E が付ける(計画の末尾「E に求めること」)。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/items` は 20 個で満杯(この計画では触らない)。`web/app/src/wire.ts` は 298 行で足せない(組み立ては `wireActions.ts`、174 行)。`web/packages/core/src/features/planning/backlog.test.ts` は 299 行で足せない。
- app の機能どうしは import しない(設計書 2 節)。`app/bulkchange` は domain と `app/port` だけを使う。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書き、言い換えない。閲覧のみの添え書きは core の `readOnlyNote`(「閲覧のみの Store です」)。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする(CI が差分を見る)。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| F2b2-1 | 項目は `status`・`assignee`・`priority`・`sprint`・`release`・`systemsAdd`・`systemsRemove`・`points`・`estimate`・`start`・`end` の 11 個(この順。ダイアログの項目のセレクトの順)。表示名は「ステータス」「担当者」「優先度」「スプリント」「リリース」「関連システムを追加」「関連システムを削除」「ポイント」「見積もり時間」「開始日」「終了日」。文の {項目}(ボタン・確認・理由・アクティビティ)は表示名、ただし `systemsAdd`・`systemsRemove` のアクティビティと理由は「関連システム」 | 04 の 834・837 行・878 行 |
| F2b2-2 | 値は名前で送る(ステータス・優先度・スプリント・リリース・関連システムは名前、担当者はログイン名)。サーバーはアイテムのワークスペース(担当者は Store のメンバー)の中で、前後の空白を除き大文字と小文字を区別せずに比べ、最初に一致したものを使う。「分類で変更」は名前の代わりに分類(`todo`・`doing`・`done`・`canceled`)を送る | 04 の 174・836・987 行、03 の 65 行。ID はワークスペースごとに違うので名前で合わせる |
| F2b2-3 | 選択肢はサーバーの `POST /api/bulk-change/choices` が、閲覧のみでない、削除されていない選んだアイテムから作る(閲覧のみのアイテムは変わらないので、その値を選んでも何も変わらない)。各一覧は、選んだ順にアイテムのワークスペース(担当者は Store)を見て、それぞれの一覧の順で、まだ出ていない名前(大文字と小文字を区別しない)を足す。表記は最初に出たもの。ステータスは、選んだどのアイテムのプロジェクトでも使わない名前を除く(`projectrule.Uses`)。スプリントは完了していない削除されていないもの、リリースはアーカイブしていない削除されていないもの。担当者は `{login, name}` | 04 の 842〜849 行、05 の 56 行。E のテーブルは Store をまたぐので画面では作れない |
| F2b2-4 | 概要の「{N} 件({種類}・{種類}…)」の N は選んだ行の数(閲覧のみを含む)。種類は choices の応答の `kinds`(選んだ削除されていないアイテムの種類を `Epic・Story・Task・Bug・Step` の順で重ねずに) | 04 の 833 行 |
| F2b2-5 | 1 件の判定 `judge` は、次の順で最初に当たった理由で残す: 閲覧のみ →「閲覧のみ」/ 削除済み・存在しない →「削除済み」/ 種類 →「{種類} には{項目}がない」(組み込みのプロパティの `Kinds` にない。非表示は見ない)・「{種類} はスプリントに入れない」(Story・Task・Bug 以外)・「Step はリリースに入れない」/ 値がない →「値がない」(関連システムを追加は選んだ値のうち 1 つでもない)・「担当者がこの Store のメンバーにいません」/ 値がすでに同じ → 数えない / ステータスをプロジェクトが使わない →「このプロジェクトでは使わないステータス」/ 遷移の制約 → `projectrule.CheckTransition` の断りの文 / 日付 →「終了日が開始日より前になる」 | 04 の 862〜874 行。非表示は 03 の 521 行(担当者を非表示にしただけなら値は使い続ける)に合わせる |
| F2b2-6 | 「分類で変更」は、そのアイテムのワークスペースの `CategoryDefaults[分類]` を行き先にする。いまのステータスがすでにその分類なら「値がすでに同じ」と同じ(数えない)。行き先をプロジェクトが使わない・遷移が禁じられているときは F2b2-5 の理由で残す | 04 の 842 行(カンバンのドラッグと同じ) |
| F2b2-7 | 実行のときに削除済み(別のタブで消えた `root: true` を含む)のものは、1 件の編集と違って削除を取り消さずに「削除済み」で残す | 04 の 867 行 |
| F2b2-8 | スプリントへ移すアイテムは、同じ保存で、ワークスペースごとに移動先の枠の末尾(`planning.FrameEnd`)から `planning.AppendWorks` で、いまの並び順のまま続ける。移動先の枠が空なら順位を変えない(1 件の `SetSprint` と同じ) | 05 の 49 行、`app/planning/assign.go` の `SetSprint` |
| F2b2-9 | 値が同じかの比べ方: ステータス・優先度・リリースは内部 ID、スプリントは `sameFrame` と同じ(使えないスプリントへの参照はバックログと同じ)、リリースの「なし」は使えないリリースへの参照と同じ、担当者はログイン名(大文字と小文字を区別しない)、ポイント・見積もり時間は数(空は値なし)、日付は文字列。関連システムの追加はアイテムが選んだ値をすべて持つとき、削除は 1 つも持たないとき同じ。一部だけなら、追加は持たない値だけ、削除は持つ値だけを書く | 04 の 876 行 |
| F2b2-10 | 書く行は 1 件の操作と同じ: ステータス・担当者・優先度・ポイント・見積もり時間・開始日・終了日は `change.NewSet(item.Field(p), from, to)`、関連システムは値ごとに `change.NewAdd`・`change.NewRemove`、スプリントは `item.SprintField`(と順位の `rank`)、リリースは `item.ReleaseField`。担当者はメンバーのログイン名の表記で書く。空(なし・バックログ・未割り当て・値の削除)は null | `items/setproperty.go`・`togglevalue.go`、`planning/assign.go` |
| F2b2-11 | 入力の検査は予告と実行の最初に 1 回: ポイントは `item.ParsePoints`、見積もり時間は `item.ParseEstimate` の断り(422、文はダイアログに出す)。日付の形でない値は `item.ErrBadInput`(422)。関連システムで値が 0 個、項目が 11 個にない、ステータスで名前と分類の両方か両方なしは `port.ErrInvalidOperation`(400)。表示中でない Store は何も書かずに `port.ErrStoreNotFound`(404、先に全部を確かめる) | 04 の 852 行(ポイント・見積もり時間の断り)、F2b1-6 |
| F2b2-12 | Store ごとに、最初に出てきた順で 1 回ずつ保存する。閲覧のみの Store は保存しない。変わるアイテムが 0 件の Store も保存しない。アクティビティは「{N} 件の{項目}を一括変更」で、N はその Store で選んだ件数(変えずに残すもの・同じ値のものを含む)。保存は判定を保存の中でもう一度行って書く(予告から保存までに変わったものはその時点の判定) | 04 の 878 行、01 の 106 行 |
| F2b2-13 | Store の保存が失敗したら、その Store で変わるはずだったアイテムの数 M を `failed` に `{Store 名, M}` で入れ、次の Store へ進む(先に保存した Store の変更は残る)。その Store で別の理由で残したものは、その理由のまま | 01 の 108 行、04 の 882 行。1 件の失敗で止める一括削除(F2b1-9)とは逆 |
| F2b2-14 | 自動化: 保存の中で、利用者の行を全部書いたあとに、(1) ステータスを変えたアイテムごとに選んだ順で、きっかけ「ステータスが次になったとき」のルールを動かす(`automation.Run`)。(2) ステータスを変えた Step の親ごとに、最初に出てきた順で 1 回、操作の前(変えた Step の前の値をすべて差し替えた状態)と後(利用者の値)で `automation.StepsDone` を判定し、合えば親で「Step がすべて完了したとき」を動かす。(2) の後の状態は (1) より先に読む(自動化の変更を数えない。B4-8)。親が削除済みなら動かさない。担当者のきっかけはない(03 のきっかけは 6 つ)。保存のあとで、動いたルールごとに受信箱へ通知を出す(`githublink/fire.go` の `notify` と同じ文) | 03 の 513・516・517 行、04 の 880 行 |
| F2b2-15 | 割り当ての通知は、新しい担当者のマシンが同期で受け取ったときに作る(C4b1 の判定)。一括変更では、このマシンで作るものはない(担当者の行はアイテムごとに書くので、受け取った側でアイテムごとに 1 行になる) | 06 の 384・388 行 |
| F2b2-16 | 予告 `POST /api/bulk-change/preview` は、保存せずに実行と同じ判定をして `{changed, kept, failed: [], offBacklog: {done:0, canceled:0}, valueText}` を返す。`valueText` は確認の本文の {値}: 名前は送った名前(前後の空白を除く)、担当者は最初に一致したメンバーの表示名、空は「なし」(スプリントは「バックログ」)、見積もり時間は `item.Hours` +「 時間」、ポイントは数、日付は `item.Display` の日付、関連システムは名前を「, 」でつなぐ。分類のときは空 | 04 の 858 行 |
| F2b2-17 | 実行 `POST /api/bulk-change` は `{changed, kept: [{store, item, reason}], failed: [{store, name, count}], offBacklog: {done, canceled}, valueText}`。`kept` は選んだ順。ID の表示は画面が開いた時点の選択の行(`SelectedRow.display`)から引く(完全に削除されて読めないアイテムにも ID を出せ、画面に見えている ID とそろうため)。`offBacklog` は、保存できた Store で変えたアイテムのうち、保存のあとに作業項目で、完了していない使えるスプリントに入っておらず、分類が完了・中止のものの数(自動化の変更を含む、保存のあとの値) | 05 の 30 行。分類での変更と同じ保存の自動化で、送った値と結果が違うことがあるため |
| F2b2-18 | 理由の一覧(core の `reasonList`)は、`kept` を(ID は F2b2-17 のとおり行から引き)理由ごとに最初に出た順でまとめ「{ID}・{ID}・{ID} ほか {K} 件({理由})」(ID は 3 件まで。4 件目からは「ほか K 件」)、続けて `failed` を 1 つずつ「{Store 名} の {M} 件(保存できませんでした)」。全体を「、」でつなぐ。M(そのまま)の件数は `kept` の数と `failed` の count の和 | 04 の 882・893 行 |
| F2b2-19 | 結果のトースト: すべて変わった「{N} 件を変えました」/ 一部だけ(エラーの色)「{N} 件を変えました。{M} 件はそのままです: {理由の一覧}」/ 0 件で残したものあり(エラーの色)「変更できたものはありません: {理由の一覧}」/ 0 件で残したものなし(エラーの色)「変わるものはありません」。バックログは変わったものがあるとき末尾に `offBacklogText`(「。{K} 件は完了のため、バックログには表示されません」、中止だけは「中止のため」、混じれば「完了・中止のため」。K が 0 なら空) を続ける | 04 の 884〜891・895 行、05 の 30 行 |
| F2b2-20 | 流れ: 「一括変更…」で、その時点の選択の行を対象に固定し、choices を読んでダイアログを開く(初めの項目は「ステータス」、値は選択肢の先頭。選択肢が空なら値なしでボタンを無効)。ボタンで予告を読み、断り(422)ならその文をダイアログの下に出して開いたまま。`changed` が 0 なら確認を出さずに結果のトースト(F2b2-19)を出してダイアログを閉じる。1 以上なら確認ダイアログ(見出し「一括変更」、本文 F2b2-21、ボタン「{N} 件を変更」)。確認で実行し、トーストを出し、ダイアログを閉じ、詳細パネルを読み直し、画面を読み直す。選択は解除しない(消えたものは読み直しの `prune` で外れる)。予告・実行の断りでない失敗は「保存できませんでした」(`sprintSaveFailed`)をエラーのトーストに出し、閉じて読み直す | 04 の 829・856・897 行 |
| F2b2-21 | 確認の本文: 「{N} 件の{項目}を {値} に変更します。」。分類は「{N} 件のステータスを 分類「{未着手|進行中|完了|中止}」(各ワークスペースの既定のステータス)に変更します。」。関連システムを追加は「{N} 件に関連システム「{値}」を追加します。」、削除は「{N} 件から関連システム「{値}」を削除します。」。N は予告の `changed` | 04 の 858 行 |
| F2b2-22 | ダイアログの説明は常に 04 の 836 行の文。ポイント・見積もり時間・開始日・終了日では 2 行目に「空にすると値を削除します。」。値の入力: ステータス・優先度・スプリント・リリース・担当者はセレクト(ステータスは末尾に見出し「分類で変更」の下に「未着手」「進行中」「完了」「中止」、スプリントは先頭「バックログ」、リリースは先頭「なし」、担当者は先頭「未割り当て」で表示名)。関連システムは ui の `ValueMenu`(`multi`)を開くボタン(02 の入力と同じ)で、何も選んでいなければ「{項目}を変更」を無効。ポイント・見積もり時間は数の入力欄、日付は日付の入力欄 | 04 の 836・840〜851 行 |
| F2b2-23 | 選択バーの「一括変更…」は「一括削除…」の前。選んだ行がすべて閲覧のみなら「一括削除…」と同じく無効にして `title` に `readOnlyNote` | 04 の 824・825 行、F2b1-4 |
| F2b2-24 | API はどれも Store をまたぐので `/api/stores/{store}` の下に置かない(F2b1-15 と同じ)。本文は `{ items: [{store, item}], field, value, values, category }`(choices は `items` だけ)。`items` が空なら 422 | F2b1-15 |

## Review Focus

1. **同じ親の Step 3 件を一括で完了にすると、「Step がすべて完了したとき」が 3 回動く。** 親 Story の下の Step 3 件(どれも Todo)を Done にすると、ルールは 1 回だけ動き、受信箱の自動化は親の 1 件(F2b2-14)。Task 3 の `TestBulkStepsDoneOnce` に入れる。
2. **2 つの Store のうち 1 つの保存が失敗すると、全部が失敗に見える・成功した Store の変更が消える。** S1・S2 の 2 件ずつを Done にし、S2 の保存だけ失敗させると、S1 の 2 件は Done、結果は `changed: 2`、`failed: [{S2, "team", 2}]`、トーストは「2 件を変えました。2 件はそのままです: team の 2 件(保存できませんでした)」(F2b2-13・F2b2-18)。Task 3 の `TestBulkStoreFailure` と Task 5 の `partial with a failed store` に入れる。
3. **ワークスペースごとにステータスの表記が違うと、片方しか変わらない。** ワークスペース A の「Done」と B の「done」を持つ 2 件に「Done」を選ぶと、両方が変わる(F2b2-2)。Task 2 の `TestJudgeNameIgnoresCase` に入れる。
4. **スプリントへまとめて入れると、並びが崩れる・枠の途中に入る。** バックログの -3・-2(並び順で -2 が先)を、すでに -9 が入っているスプリントへ移すと、枠は -9・-2・-3 の順(F2b2-8)。Task 3 の `TestBulkSprintKeepsOrder` に入れる。
5. **分類で「完了」にしたとき、すでに完了のアイテムまで書き換える・数える。** Done・Cancel・Todo の 3 件に分類「完了」を選ぶと、Done は同じ値として数えず書かない。Todo と Cancel(分類は中止で完了ではない)は、ワークスペースの完了の既定のステータスに変わる(F2b2-6)。Task 2 の `TestJudgeCategory` に入れる。

---

## ファイルの地図

```
internal/app/bulkchange/service.go      Service・Target・Field・Input・Kept・Failed・Result・Choices・fieldName
internal/app/bulkchange/input.go        parseInput(入力の検査。F2b2-11)・valueText
internal/app/bulkchange/judge.go        itemContext・verdict・judge(1 件の判定。F2b2-5〜10)
internal/app/bulkchange/judge_value.go  項目ごとの値の行き先(judge が呼ぶ。300 行に収めるため分ける)
internal/app/bulkchange/choices.go      Choices
internal/app/bulkchange/run.go          Preview・Run・Store ごとの保存・offBacklog
internal/app/bulkchange/automate.go     保存の中の自動化(F2b2-14)と、保存のあとの通知
internal/app/bulkchange/*_test.go
internal/adapter/httpapi/bulkchange/bulkchange.go  API 3 本
internal/adapter/httpapi/routes.go      (変更)Register を 1 行
web/packages/core/src/model/bulkChange.ts           型
web/packages/core/src/port/itemActions.ts           (変更)bulkChangeChoices・bulkChangePreview・bulkChange
web/packages/api/src/itemActions.ts                 (変更)
web/packages/core/src/features/selection/bulkChangeForm.ts    項目・選択肢・説明・確認の文
web/packages/core/src/features/selection/bulkChangeResult.ts  理由の一覧・結果のトースト・offBacklogText
web/packages/core/src/features/selection/bulkChange.ts        流れ(createBulkChange)
web/packages/core/src/selectionIndex.ts             (変更)公開
web/packages/ui/src/patterns/SelectionBar.svelte    (変更)「一括変更…」
web/app/src/screens/item/BulkChangeDialog.svelte    ダイアログ「選択したアイテムを一括変更」と確認
web/app/src/screens/item/BulkChangeValue.svelte     値の入力
web/app/src/wireActions.ts・App.svelte・screens/planning/BacklogTab.svelte  (変更)
web/e2e/backlog-bulk-change.spec.ts
```

---

### Task 1: 型・入力の検査・選択肢(`app/bulkchange`)

**Files:**
- Create: `internal/app/bulkchange/service.go`、`input.go`、`choices.go`
- Test: `internal/app/bulkchange/choices_test.go`、`input_test.go`

**Interfaces:**
- Consumes: `port.Stores`・`port.StoreList`・`port.Identity`・`port.Clock`・`port.Inbox`、`port.LiveItem`、`projectrule.FromLog`・`projectrule.Uses`、`planning.FromLog`(計画の読み方は `app/planning` の `planOf` に合わせる)
- Produces:
  - `type Service struct { Stores port.Stores; StoreList port.StoreList; Identity port.Identity; Clock port.Clock; Inbox port.Inbox }`
  - `type Target struct { Store port.StoreID; ID item.ID }`
  - `type Field string` と定数 `FieldStatus = "status"`・`FieldAssignee`・`FieldPriority`・`FieldSprint`・`FieldRelease`・`FieldSystemsAdd = "systemsAdd"`・`FieldSystemsRemove = "systemsRemove"`・`FieldPoints`・`FieldEstimate`・`FieldStart`・`FieldEnd`
  - `type Input struct { Field Field; Value string; Values []string; Category workspace.Category }`
  - `type Kept struct { Store port.StoreID; ID item.ID; Reason string }`、`type Failed struct { Store port.StoreID; Name string; Count int }`、`type OffBacklog struct { Done, Canceled int }`
  - `type Result struct { Changed int; Kept []Kept; Failed []Failed; OffBacklog OffBacklog; ValueText string }`
  - `type Member struct { Login, Name string }`
  - `type Choices struct { Kinds, Statuses, Priorities, Sprints, Releases, Systems []string; Members []Member }`
  - `func fieldName(f Field, activity bool) string` — F2b2-1(`activity` が true なら関連システムの 2 つは「関連システム」)
  - `type parsed struct { field Field; text string; points, estimate *float64; date string; names []string; category workspace.Category }` と `func parseInput(in Input) (parsed, error)` — F2b2-11
  - `func (s Service) Choices(ctx context.Context, targets []Target) (Choices, error)` — F2b2-3・F2b2-4
  - `func (s Service) groups(ctx context.Context, targets []Target) ([]storeGroup, error)` — `storeGroup{ id port.StoreID; name string; readOnly bool; targets []Target }`。最初に出た順。一覧にない Store があれば `port.ErrStoreNotFound`(Task 3 も使う)

- [ ] **Step 1: 失敗するテストを書く**

組み立て(`setup(t)`)は F2b1 の `items/bulkdelete_test.go` の `setupBulk` と同じ形: S1 = `apptest.Sample` + `apptest.SampleMobile`、S2 = `apptest.Sample`、S3 = `apptest.Sample`(閲覧のみ)、`apptest.StoreList` は main・team・readonly。

- `TestChoicesMerge`: S1 の HH-2(Web)と MB-1(Mobile)を渡す → `Statuses` は Web のステータスの順に Mobile にしかない名前を続けたもの、`Kinds` は `["Epic", "Task"]`、`Members` は `[{taro, taro}, {hanako, hanako}]`。
- `TestChoicesIgnoreCase`: S2 のステータス Done の名前を "done" に変え(`workspaces` の値の名前の書き換えの行を直接書く)、S1 の HH-2 と S2 の HH-2 → `Statuses` に "Done" が 1 回だけ(表記は先に出た S1 のもの)。
- `TestChoicesSkipReadOnlyAndUnused`: S3 の HH-2 だけ → 一覧はどれも空(`Kinds` も空)。S1 の HH-2 のプロジェクトで Pending を使わない(プロジェクト別のルールの行を書く)→ `Statuses` に Pending がない。
- `TestChoicesSprints`: S1 のワークスペースに計画中のスプリント「S-1」・完了したスプリント「S-0」、アーカイブのリリース「R-0」・計画中の「R-1」→ `Sprints` は `["S-1"]`、`Releases` は `["R-1"]`。
- `TestChoicesUnknownStore`: 一覧にない Store "S9" → `errors.Is(err, port.ErrStoreNotFound)`。
- `TestParseInput`: `{FieldPoints, "-1"}` →「ポイントは 0 以上の数で入力してください」の断り。`{FieldEstimate, "1.234"}` →「見積もり時間は小数第 2 位までで入力してください」。`{FieldStart, "2026/11/14"}` → `item.ErrBadInput`。`{FieldSystemsAdd, Values: nil}`・`{Field: "title"}`・`{FieldStatus, Value: "Done", Category: "done"}`・`{FieldStatus}` → `port.ErrInvalidOperation`。`{FieldPoints, ""}` → `points == nil` でエラーなし。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/bulkchange/ -v`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: 実装する**(名前の比べ方は `strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))` を `sameName` 1 か所に置く)

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/bulkchange/ -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/bulkchange/
git commit -m "feat(bulkchange): 一括変更の入力の検査と選択肢を足す(F2b2 Task 1)"
```

---

### Task 2: 1 件の判定(`judge`)

**Files:**
- Create: `internal/app/bulkchange/judge.go`、`judge_value.go`
- Test: `internal/app/bulkchange/judge_test.go`

**Interfaces:**
- Consumes: Task 1 の `parsed`・`fieldName`・`sameName`、`projectrule.Uses`・`projectrule.CheckTransition`、`planning.AssignableSprint`・`Plan.UsableSprint`・`Plan.UsableRelease`・`planning.IsWork`、`item.Field`・`item.Current`・`item.SprintField`・`item.ReleaseField`・`item.CheckDates`
- Produces:
  - `type itemContext struct { it item.Item; readOnly, deleted bool; settings workspace.Settings; rules projectrule.Rules; plan planning.Plan; members []member.Member }`
  - `type verdict struct { drafts []change.Draft; reason string; sprint *change.ID }` — `drafts` が空で `reason` も空なら「値がすでに同じ」。`sprint` はスプリントへ移すときの行き先(空文字はバックログ。順位は Task 3 が足す)
  - `func judge(c itemContext, in parsed) verdict` — F2b2-5〜F2b2-10。理由の文は F2b2-5 の括弧の中(「{ID}(…)」の … の部分)。削除済み・存在しないアイテムは `it` に ID だけを入れて `deleted: true`

- [ ] **Step 1: 失敗するテストを書く**(表のテスト。`itemContext` はサンプルの設定 `workspace.DefaultSettings` から組む)

- `TestJudgeOrder`: 閲覧のみかつ削除済み →「閲覧のみ」。削除済みの Epic に優先度 →「削除済み」。
- `TestJudgeKinds`: 優先度の `Kinds` から Epic を外した設定で Epic →「Epic には優先度がない」。Epic にスプリント →「Epic はスプリントに入れない」。Step にリリース →「Step はリリースに入れない」。関連システムを追加の `Kinds` から Task を外して Task →「Task には関連システムがない」。優先度を非表示にしただけ(`Hidden: true`)なら変える。
- `TestJudgeNameIgnoresCase`(Review Focus 3): 名前 " done " → Done の ID への `set` の行。ない名前「Closed」→「値がない」。
- `TestJudgeSame`: いまと同じステータス → `drafts` も `reason` も空。関連システムを追加で Web・API を選び、アイテムが Web・API を持つ → 空。Web だけ持つ → API の `add` の行 1 つ。削除で Web・API を選び、何も持たない → 空。
- `TestJudgeCategory`(Review Focus 5): 分類 `done` で、Done → 空、Todo → `CategoryDefaults[done]` への行、Cancel → 同じ行。
- `TestJudgeStatusRules`: プロジェクトが Done を使わない →「このプロジェクトでは使わないステータス」。ワークフローで Todo → Done を禁じた設定 → `projectrule.CheckTransition` の断りの文そのもの。
- `TestJudgeAssignee`: "HANAKO" → 担当者を `hanako` にする行(表記はメンバーのもの)。"jiro" →「担当者がこの Store のメンバーにいません」。"" → null の行。
- `TestJudgeSprintRelease`: 名前「s-1」(計画中の S-1)→ `sprint` が S-1 の ID で、`item.SprintField` の行。「バックログ」(空)で、いま使えないスプリントを指している → 空。リリース「R-1」→ `item.ReleaseField` の行。ない名前 →「値がない」。
- `TestJudgeDates`: 終了日 2026-11-10 のアイテムに開始日 2026-11-12 →「終了日が開始日より前になる」。空の開始日 → null の行。
- `TestJudgeNumbers`: ポイント 3 → `set` の行。いま 3 で 3 → 空。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/bulkchange/ -run TestJudge -v`
Expected: FAIL

- [ ] **Step 3: 実装する**(関数 60 行・認知的複雑度 15 に収めるよう、`judge` は共通の理由(閲覧のみ・削除済み)を見て、項目ごとの関数を `judge_value.go` から呼ぶ)

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/bulkchange/ -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/bulkchange/
git commit -m "feat(bulkchange): 一括変更の 1 件ごとの判定を足す(F2b2 Task 2)"
```

---

### Task 3: 予告・実行・自動化(`app/bulkchange`)

**Files:**
- Create: `internal/app/bulkchange/run.go`、`automate.go`
- Test: `internal/app/bulkchange/run_test.go`、`automate_test.go`

**Interfaces:**
- Consumes: Task 1 の `groups`・`parseInput`、Task 2 の `judge`、`automation.FromLog`・`automation.Run`・`automation.StepsDone`・`automation.StoppedText`・`automation.AppliedText`・`automation.ChangeText`、`planning.FrameEnd`・`planning.AppendWorks`・`planning.Category`、`item.Display`
- Produces:
  - `func (s Service) Preview(ctx context.Context, targets []Target, in Input) (Result, error)` — F2b2-16
  - `func (s Service) Run(ctx context.Context, targets []Target, in Input) (Result, error)` — F2b2-12・13・17
  - `func valueText(in parsed, first itemContext) string`(F2b2-16。Task 1 の `input.go` に置く)

- [ ] **Step 1: 失敗するテストを書く**

Task 1 の `setup`。保存の失敗は、テストの中の `failing{port.Stores; fail port.StoreID}`(`Write` で `fail` の Store なら `errors.New("boom")`)で包む。

- `TestRunAcrossStores`: S1 の HH-1・HH-2 と S2 の HH-2 と S3 の HH-2 を Done → `Changed: 3`、`Kept: [{S3, HH-2 の内部 ID, "閲覧のみ"}]`。S1 のアクティビティの最後の行は「2 件のステータスを一括変更」、S2 は「1 件のステータスを一括変更」、S3 には書かない。
- `TestRunSkipsSameAndCountsSelected`: S1 の HH-1(Done)・HH-2(Todo)を Done → `Changed: 1`、`Kept` は空、アクティビティは「2 件のステータスを一括変更」(F2b2-12)。全部同じなら保存しない(アクティビティの行が増えない)。
- `TestRunKeepsDeleted`: HH-2 を先に削除してから HH-1・HH-2 → HH-2 は「削除済み」で残り、削除は取り消されない(F2b2-7)。
- `TestBulkStoreFailure`(Review Focus 2): S1・S2 の HH-1・HH-2 を Done、S2 の保存を失敗させる → S1 の 2 件は Done、`Changed: 2`、`Failed: [{S2, "team", 2}]`、エラーは返さない。
- `TestBulkSprintKeepsOrder`(Review Focus 4): S1 にスプリント S-1(計画中)、HH-9 を S-1 に入れ、Story HH-3・HH-4(並び順 HH-3 が先)をバックログに作り、HH-4・HH-3 の順で渡して S-1 へ → S-1 の枠の並び(順位の昇順)は HH-9・HH-3・HH-4。
- `TestPreviewDoesNotWrite`: 予告のあと、アクティビティの行は増えない。`Changed`・`Kept` は実行と同じ。`ValueText` はステータスなら "Done"、見積もり時間 "1.5" なら「1.5 時間」、スプリントの空なら「バックログ」、担当者 "HANAKO" なら「hanako」(表示名)。
- `TestRunUnknownStore`: 一覧にない Store を含む → `port.ErrStoreNotFound` で、S1 にも書かない。
- `TestOffBacklog`: S1 にスプリント S-1(計画中)。バックログの HH-3(Todo)と S-1 の HH-4(Todo)を Done → `OffBacklog{Done: 1}`(HH-4 はスプリントの枠に残る)。Cancel にすれば `Canceled: 1`。
- `TestBulkAutomation`(automate_test.go): きっかけ「ステータスが Done になったとき」・すること「担当者を taro に」のルール。HH-1・HH-2 を Done → 2 件とも担当者 taro(同じ保存の中、行の `actor` は自動化)、`apptest.Inbox` の自動化の通知は 2 件。
- `TestBulkStepsDoneOnce`(Review Focus 1): Story の下に Step 3 件(Todo)、きっかけ「Step がすべて完了したとき」・すること「ステータスを Done に」のルール。3 件を Done → Story は Done、通知は Story の 1 件だけ。Step 2 件だけを Done → 動かない。
- `TestBulkAutomationStopped`: ルールの行き先をワークフローで禁じる → 通知は止まったもの(`port.NoticeAutomationStopped`)で、アイテムは変わらない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/bulkchange/ -run 'TestRun|TestBulk|TestPreview|TestOffBacklog' -v`
Expected: FAIL

- [ ] **Step 3: 実装する**

Store ごとの流れ(`run.go`): `groups` → 閲覧のみの Store は全部「閲覧のみ」。ほかは `Stores.Read` で各アイテムの `itemContext` を作って `judge`(予告と、保存の前の件数)。実行では、変わるものがあれば `Stores.Write(ctx, id, port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%d 件の%sを一括変更", len(group.targets), fieldName(f, true))}, …)` の中でもう一度 `judge` して書き、スプリントの順位(F2b2-8)、自動化(`automate.go`)の順。失敗したら `Failed`。保存のあとに通知と `offBacklog`(`Stores.Read` で変えたアイテムを読み直す)。`Kept` は最後に選んだ順に並べ直す。自動化の通知の組み立ては `app/githublink/fire.go` の `fired`・`notify` と同じ形で書く(app の機能どうしは import できないため)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/bulkchange/ -v && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/bulkchange/
git commit -m "feat(bulkchange): 一括変更の予告と Store ごとの保存、自動化を足す(F2b2 Task 3)"
```

---

### Task 4: 一括変更の API

**Files:**
- Create: `internal/adapter/httpapi/bulkchange/bulkchange.go`
- Modify: `internal/adapter/httpapi/routes.go`(`bulkchange.Register(api, appbulkchange.Service{Stores: deps.Stores, StoreList: deps.StoreList, Identity: deps.Identity, Clock: deps.Clock, Inbox: notifier})`)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`)
- Test: `internal/adapter/httpapi/bulkchange/bulkchange_test.go`

**Interfaces:**
- Consumes: Task 1〜3 の `Choices`・`Preview`・`Run`
- Produces: OperationID `bulk-change-choices`(`POST /api/bulk-change/choices`、本文 `{items}`、応答 `{kinds, statuses, priorities, sprints, releases, systems, members: [{login, name}]}`)、`preview-bulk-change`(`POST /api/bulk-change/preview`)、`bulk-change`(`POST /api/bulk-change`)。後の 2 本の本文は `{items: [{store, item}], field, value, values, category}`(`field` は 11 個の enum、`category` は `""`・`todo`・`doing`・`done`・`canceled`)、応答は `{changed, kept: [{store, item, reason}], failed: [{store, name, count}], offBacklog: {done, canceled}, valueText}`。配列は空でも `[]`(`nullable:"false"`)。生成される TS の関数名は `bulkChangeChoices`・`previewBulkChange`・`bulkChange`

- [ ] **Step 1: 失敗するテストを書く**(`internal/adapter/httpapi/items/bulk_test.go` の組み立てに合わせる)

- `TestBulkChangeAPI`: サンプルの HH-1・HH-2 を `{field:"status", value:"Done"}` で `POST /api/bulk-change` → 200、`changed` 2、`kept` は `[]`。
- `TestBulkChangePreviewAPI`: 同じ本文で `/preview` → `changed` 2、`valueText` "Done"。`{field:"points", value:"-1"}` → 422、本文の detail が「ポイントは 0 以上の数で入力してください」。
- `TestBulkChangeChoicesAPI`: → `statuses` の先頭が "Todo"、`members` に `{login:"taro", name:"taro"}`。
- `TestBulkChangeAPIRejects`: `items` が空 → 422。一覧にない Store → 404。`field` が "title" → 422(enum)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/bulkchange/ -v`
Expected: FAIL

- [ ] **Step 3: 実装し、`make generate` を流す**(エラーは `apierr.From`)

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/adapter/httpapi/... && make generate && git diff --stat api/ web/packages/api/src/generated.ts`
Expected: PASS、生成物の差分は 3 本の API の分だけ

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/httpapi/ api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(api): 一括変更の選択肢・予告・実行の API を足す(F2b2 Task 4)"
```

---

### Task 5: 一括変更の文と口(core の `bulkChangeForm`・`bulkChangeResult`)

**Files:**
- Create: `web/packages/core/src/model/bulkChange.ts`、`web/packages/core/src/features/selection/bulkChangeForm.ts`、`bulkChangeResult.ts`
- Modify: `web/packages/core/src/port/itemActions.ts`、`web/packages/api/src/itemActions.ts`、`web/packages/core/src/selectionIndex.ts`、`web/packages/core/src/index.ts`(model を公開しているなら同じ形で)、`web/packages/core/src/features/items/actions.test-util.ts`(偽の口に 3 つ)
- Test: `web/packages/core/src/features/selection/bulkChangeForm.test.ts`、`bulkChangeResult.test.ts`

**Interfaces:**
- Consumes: Task 4 の生成物
- Produces:
  - model: `type BulkField = "status" | "assignee" | "priority" | "sprint" | "release" | "systemsAdd" | "systemsRemove" | "points" | "estimate" | "start" | "end"`、`interface BulkChangeInput { field: BulkField; value: string; values: string[]; category: Category | "" }`、`interface BulkChoices { kinds: string[]; statuses: string[]; priorities: string[]; sprints: string[]; releases: string[]; systems: string[]; members: Member[] }`、`interface BulkChangeResult { changed: number; kept: { store: string; item: string; reason: string }[]; failed: { store: string; name: string; count: number }[]; offBacklog: { done: number; canceled: number }; valueText: string }`
  - port: `bulkChangeChoices(items: ItemRef[]): Promise<BulkChoices>`、`bulkChangePreview(items: ItemRef[], input: BulkChangeInput): Promise<BulkChangeResult>`、`bulkChange(items: ItemRef[], input: BulkChangeInput): Promise<BulkChangeResult>`
  - `const bulkFields: { id: BulkField; label: string }[]`(F2b2-1 の順と表示名)
  - `function changeButton(field: BulkField): string` — 「ステータスを変更」、関連システムの 2 つは「関連システムを追加」「関連システムを削除」
  - `interface BulkOption { value: string; label: string; group?: string }` と `function valueOptions(field: BulkField, c: BulkChoices): BulkOption[]` — F2b2-22。分類の 4 つは `value` が `category:todo` などで `group` が「分類で変更」。スプリント・リリース・担当者の先頭の `value` は `""`。担当者の `value` はログイン名、`label` は表示名
  - `function inputOf(field: BulkField, option: string): BulkChangeInput`(`category:` で始まれば `category`、ほかは `value`)
  - `function summaryText(n: number, kinds: string[]): string` —「{N} 件({種類}・{種類})」
  - `function explanation(field: BulkField): string[]` — F2b2-22
  - `function confirmLines(input: BulkChangeInput, r: BulkChangeResult): string[]` — F2b2-21(分類のラベルは「未着手」「進行中」「完了」「中止」)
  - `function reasonList(r: BulkChangeResult, rows: SelectedRow[]): string` — F2b2-18(行にないものは内部 ID のまま)
  - `function bulkChangeToast(r: BulkChangeResult, rows: SelectedRow[], suffix: string): { text: string; error: boolean }` — F2b2-19
  - `function offBacklogText(r: BulkChangeResult): string` — F2b2-19 の末尾

- [ ] **Step 1: 失敗するテストを書く**

- `fields and buttons`: `bulkFields` の 11 個の表示名と順。`changeButton("status")` は「ステータスを変更」、`changeButton("systemsAdd")` は「関連システムを追加」。
- `value options`: ステータスの選択肢は名前のあとに `{value:"category:todo", label:"未着手", group:"分類で変更"}` ほか 3 つ。スプリントの先頭は `{value:"", label:"バックログ"}`、リリースは「なし」、担当者は「未割り当て」で、メンバーは表示名。優先度に「なし」はない。
- `explanation`: ステータスでは 1 行(04 の 836 行の文)、ポイントでは 2 行目「空にすると値を削除します。」。
- `summary`: `summaryText(3, ["Story", "Task"])` は「3 件(Story・Task)」。
- `confirm lines`: ステータス Done・`changed` 3 →「3 件のステータスを Done に変更します。」。分類 done →「3 件のステータスを 分類「完了」(各ワークスペースの既定のステータス)に変更します。」。関連システムを追加・`valueText` "Web, API" →「3 件に関連システム「Web, API」を追加します。」、削除 →「3 件から関連システム「Web, API」を削除します。」。スプリントの空・`valueText` 「バックログ」→「3 件のスプリントを バックログ に変更します。」。
- `reason list`: 行 HH-1〜HH-5 で、kept が HH-1・HH-2・HH-3・HH-4(閲覧のみ)と HH-5(削除済み)(どれも store と内部 ID で、表示は行から引く)→「HH-1・HH-2・HH-3 ほか 1 件(閲覧のみ)、HH-5(削除済み)」。failed `{name:"team", count:2}` を足すと末尾に「、team の 2 件(保存できませんでした)」。
- `toast`: 全部 →「2 件を変えました」(error false)。一部 →「2 件を変えました。1 件はそのままです: HH-5(削除済み)」(error true)。0 件で残りあり →「変更できたものはありません: HH-5(削除済み)」。0 件で残りなし →「変わるものはありません」。`suffix` を渡すと、変わったものがあるときだけ末尾に付く。
- `off backlog`: `{done:2, canceled:0}` →「。2 件は完了のため、バックログには表示されません」、`{0,1}` →「。1 件は中止のため、…」、`{1,1}` →「。2 件は完了・中止のため、…」、`{0,0}` → 空。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- bulkChange`
Expected: FAIL

- [ ] **Step 3: 実装する**(api の口は F2b1 の `bulkDelete` と同じく `call(async () => …)` で包む)

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- bulkChange && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/ web/packages/api/src/itemActions.ts
git commit -m "feat(core): 一括変更の項目・選択肢・確認と結果の文を足す(F2b2 Task 5)"
```

---

### Task 6: 一括変更の流れ(core の `bulkChange`)

**Files:**
- Create: `web/packages/core/src/features/selection/bulkChange.ts`
- Modify: `web/packages/core/src/selectionIndex.ts`
- Test: `web/packages/core/src/features/selection/bulkChange.test.ts`

**Interfaces:**
- Consumes: Task 5 のすべて、F2b1 の `RowSelection`・`SelectedRow`・`ItemRef`、`Detail`(`reload`)、`Toasts`、`Refused`、`sprintSaveFailed`
- Produces:
  - `interface BulkChangeForm { stage: "form"; summary: string; field: BulkField; options: BulkOption[]; option: string; values: string[]; text: string; error: string | null; busy: boolean }` — `option` はセレクトの値、`values` は関連システムで選んだ名前、`text` は数・日付の入力
  - `interface BulkChangeConfirm { stage: "confirm"; title: string; lines: string[]; ok: string }`
  - `interface BulkChangeDeps { actions: Pick<ItemActionsPort, "bulkChangeChoices" | "bulkChangePreview" | "bulkChange">; toasts: Toasts; detail: Pick<Detail, "reload"> }`
  - `interface BulkChangeOpen { after: () => Promise<void>; suffix?: (r: BulkChangeResult) => string }`
  - `interface BulkChange { state: Readable<BulkChangeForm | BulkChangeConfirm | null>; open(selection: RowSelection, o: BulkChangeOpen): Promise<void>; setField(f: BulkField): void; setOption(v: string): void; toggleValue(name: string): void; setText(t: string): void; submit(): Promise<void>; confirm(): Promise<void>; cancel(): void }`
  - `function canSubmit(f: BulkChangeForm): boolean`(関連システムで `values` が空、選択肢のある項目で `options` が空なら false)
  - `function createBulkChange(deps: BulkChangeDeps): BulkChange`

- [ ] **Step 1: 失敗するテストを書く**

- `opens with status`: 選択 2 件で `open` → `bulkChangeChoices` に 2 件、`state` は `stage: "form"`・`field: "status"`・`option` は選択肢の先頭・`summary` は「2 件(Task)」。
- `fixed at open`: `open` のあとに `selection.toggle` で 1 件足しても、`submit` の予告に送るのは 2 件。
- `field change resets the value`: `setField("points")` → `option` は空・`text` は空・`error` は null。
- `refusal stays in the dialog`: 予告が `Refused("ポイントは 0 以上の数で入力してください")` → `state.error` がその文で `stage` は form のまま。
- `nothing changes`: 予告が `changed: 0` で kept 1 件 → 確認を出さず、エラーのトースト「変更できたものはありません: …」、`state` は null、`bulkChange` は呼ばない、`after` は呼ばない。
- `confirm runs and keeps the selection`: 予告 `changed: 2` → `stage: "confirm"`・`title` 「一括変更」・`ok` 「2 件を変更」。`confirm` → `bulkChange` に同じ 2 件と入力、トースト「2 件を変えました」、`state` は null、選択は 2 件のまま、`after` と `detail.reload` が 1 回ずつ。
- `suffix`: `open` に `suffix: offBacklogText`、結果 `offBacklog.done` 1 → トーストの末尾に「。1 件は完了のため、バックログには表示されません」。
- `run failure`: `bulkChange` が投げる(`Refused` でない)→ エラーのトースト「保存できませんでした」、`state` は null、`after` が呼ばれる。
- `cancel while loading`: choices を待っているあいだに `cancel` → 返っても `state` は null。
- `systems`: `setField("systemsAdd")` → `canSubmit` は false、`toggleValue("Web")` → true、もう一度 `toggleValue("Web")` → false。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- bulkChange.test`
Expected: FAIL

- [ ] **Step 3: 実装する**(世代 `seq` と実行中 `busy` の扱いは F2b1 の `createBulkDelete` に合わせる)

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- bulkChange && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/
git commit -m "feat(core): 一括変更のダイアログ・確認・実行の流れを足す(F2b2 Task 6)"
```

---

### Task 7: 選択バーの「一括変更…」とダイアログ(画面)

**Files:**
- Create: `web/app/src/screens/item/BulkChangeDialog.svelte`、`web/app/src/screens/item/BulkChangeValue.svelte`
- Modify: `web/packages/ui/src/patterns/SelectionBar.svelte`、`web/packages/ui/src/patterns/SelectionBar.test.ts`、`web/app/src/wireActions.ts`、`web/app/src/App.svelte`(`BulkDeleteDialog` の隣に `BulkChangeDialog`)、`web/app/src/screens/planning/BacklogTab.svelte`
- Test: `web/packages/ui/src/patterns/SelectionBar.test.ts`

**Interfaces:**
- Consumes: Task 5・6 のすべて、ui の `Dialog`・`Select`・`Input`・`Button`・`ValueMenu`・`ConfirmDialog`
- Produces:
  - `SelectionBar` の Props に `onchange: () => void` を足す。ボタンは「一括変更…」「一括削除…」「選択を解除」の順で、`readOnlyNote` を渡したら「一括変更…」も無効にして `title` にその文(F2b2-23)
  - `web/app/src/wireActions.ts` の `export const bulkChange = createBulkChange({ actions: itemActionsPort, toasts, detail })`。画面を閉じるときの `bulkDelete.cancel()` の並びに `bulkChange.cancel()`
  - `BulkChangeValue` の Props: `{ form: BulkChangeForm }`(入力は `bulkChange.setOption`・`toggleValue`・`setText` を呼ぶ)

- [ ] **Step 1: 失敗するテストを書く**

- `SelectionBar`: ボタンが「一括変更…」「一括削除…」「選択を解除」の順。「一括変更…」で `onchange`。`readOnlyNote` を渡したら「一括変更…」と「一括削除…」が `disabled` で `title` がその文、「選択を解除」は押せる。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/ui test -- SelectionBar`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `BulkChangeDialog`: `stage: "form"` のとき `Dialog`(見出し「選択したアイテムを一括変更」)に、概要(`summary`)、項目のセレクト(`bulkFields`、`aria-label` は「項目」)、`BulkChangeValue`(ラベル「値」)、`explanation` の行、`error`(エラーの色)、ボタン「キャンセル」と `changeButton(field)`(`canSubmit` が false か `busy` なら無効)。`stage: "confirm"` のとき `ConfirmDialog`(`title`・`lines`・`ok`、危険な見た目にしない)。
- `BulkChangeValue`: 選択肢のある項目は `Select`(`group` のある項目は `<optgroup label="分類で変更">`。ui の `Select` が `optgroup` を持たなければ `<select>` をこのファイルで書く)。関連システムはボタン(選んだ名前を「, 」でつなぐ。何もなければ「選択」)から `ValueMenu`(`multi`)。ポイント・見積もり時間は `Input`(`inputmode="decimal"`)、日付は `Input`(`type="date"`)。
- `BacklogTab`: `SelectionBar` に `onchange={() => void bulkChange.open(backlog.selection, { after: () => backlog.reload(), suffix: offBacklogText })}`。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/
git commit -m "feat(web): 選択バーに一括変更を足し、一括変更のダイアログと確認を置く(F2b2 Task 7)"
```

---

### Task 8: e2e と計画の記録

**Files:**
- Create: `web/e2e/backlog-bulk-change.spec.ts`
- Modify: `docs/superpowers/plans/2026-11-14-hedgehug-track-f2b1.md`(末尾「F2b2 に渡すもの」の各行に「F2b2 で足した」を添える。「E に求めること」の最後の行に、一括変更のダイアログは `bulkChange.open(selection, { after: 読み直し })` で開くことを 1 行足す)

- [ ] **Step 1: e2e を書く**

`backlog-select.spec.ts`(F2b1)の組み立てで、Story を 3 件(接頭辞-2・-3・-4)作り、バックログのタブを開く。

- `bulk changes the status`: -2・-3 を選び「一括変更…」→ ダイアログ「選択したアイテムを一括変更」、概要「2 件(Story)」、項目「ステータス」。値「Done」で「ステータスを変更」→ 確認「2 件のステータスを Done に変更します。」→「2 件を変更」→ トースト「2 件を変えました。2 件は完了のため、バックログには表示されません」、-2・-3 の行とバーが消え、-4 は残る。
- `nothing to change`: -4 を選び、値をいまと同じステータスにして「ステータスを変更」→ 確認は出ず、トースト「変わるものはありません」、選択は残る。
- `points refusal`: -4 を選び、項目「ポイント」、値「-1」→ ダイアログの中に「ポイントは 0 以上の数で入力してください」、ダイアログは開いたまま。値「3」→ 確認 →「1 件を変えました」、行のポイントが 3、選択は残る(「1 件を選択中」)。
- `read-only store`: `/s/readonly/w/HH/backlog` で「HH-2 を選択」を押すと「一括変更…」が無効で `title` が「閲覧のみの Store です」。

- [ ] **Step 2: 流して通ることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/backlog-bulk-change.spec.ts`
Expected: 4 passed

- [ ] **Step 3: 計画の記録を直す**(上の Files のとおり)

- [ ] **Step 4: コミット**

```bash
git add web/e2e/backlog-bulk-change.spec.ts docs/superpowers/plans/
git commit -m "test(e2e): バックログの一括変更を通しで確かめ、F2b1 の計画に記録する(F2b2 Task 8)"
```

---

## 仕上げ

- 最終レビュー(superpowers:requesting-code-review)で、ブランチ全体を仕様 04「一括変更」・05 の 30 行・03「実行の規則」と照らす。
- PR は develop 向けに 1 本。本文に、計画から変えたことと、E に求めることを書く。

---

## E に求めること

- 検索のテーブルの `SelectionBar` に `onchange={() => bulkChange.open(selection, { after: 読み直し })}` を渡す(`suffix` は渡さない。バックログだけの末尾のため)。
- パレットの「{N} 件をまとめて変更…」は、検索のテーブルを開いてすべての行を選んでから `bulkChange.open` を呼ぶ(04 の 822 行)。
- 選択肢・判定・Store ごとの保存は、テーブルが Store・ワークスペースをまたいでもサーバー(F2b2-3・F2b2-12)が扱う。
