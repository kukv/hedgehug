# hedgehug トラック F3(レポート)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 05 の「レポート」(単位と選択・バーンダウン・ベロシティ・累積フロー図・サイクルタイムとリードタイム・見積もりと実績・表で見る)、スプリントの画面の実行中のカードの小さいバーンダウン・「途中で追加」・メニューの「バーンダウンを見る」、ワークスペースの概要の「{スプリント名} の完了ポイント」のカードを作る。

**Architecture:** 数え方はすべて `domain/planning` に置く(F1 の `Confirm`・`ValueAt`・`counted` と同じパッケージにして、確定値とバーンダウンの数え方をそろえる)。読み取りと選択の保存は新しい `app/reports`、API は新しい `adapter/httpapi/reports`。レポートの選択は `state.db`(保存形式 282 行)に、`app/reports` で宣言した口 `reports.Selections` を `infra/statedb` が満たす形で持つ(`transfers.Delivery` と同じ前例。`app/port` は満杯)。図はライブラリを足さず、座標の計算を core の TS に置き、Svelte は SVG を描くだけにする。

**Tech Stack:** Go 1.27.3、huma v2、modernc.org/sqlite(既存)。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/api`、orval の生成)、e2e は Playwright。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/05-planning-reports.md`「数え方の共通ルール」(9〜17 行)、「スプリントの画面」の表の「途中で追加」「バーンダウン」(192・193 行)とメニューの「バーンダウンを見る」(207 行)、「ワークスペース概要の完了ポイント」(290〜297 行)、「レポート」(299〜405 行)、「モックからの変更点」のうちレポートの行
- 04「ワークスペースの概要とプロジェクトの一覧」の「統計」(921〜932 行)の「{スプリント名} の完了ポイント」のカード
- 01 の 132 行(日付が変わったときの表示)、01 の日付の書き方(core の `formatDay`)
- 06 の 155 行(画面への反映に「レポート」)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` 267 行(レポートは `set sprint`・`set status`・`create` の行から取り、量はいまの値)、282 行(レポートの選択は state.db)、284 行(Store をこのマシンから削除したら、その Store の行を消す)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-f1.md` の F1-5・F1-9・F1-10・F1-19、Task 4 の `Work`・`Confirm`・`ValueAt`

## 始める前に

- トラック F は 3 本に分ける(ユーザーの決定、2026-11-14): F1 スプリントとリリース(develop に入った、PR #206)→ F2 バックログ・F3 レポート。F の続きは F3 から進め、F3 は計画 1 本・PR 1 本にする(ユーザーの決定、2026-11-14)。F2 は、行の選択と一括変更(04)をトラック E とどう分けるかを決めてから計画を書く。
- 実行中のスプリントのカードの「リスト」と「ボード」の切り替えはトラック E が足す(F1 の「始める前に」)。F3 はカードの「アイテム」の欄に触らない。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/port`・`app/apptest`・`app/workspaces`・`infra/gitstore` は 20 個で満杯なので、ファイルを足さない。`domain/planning` は 7 個、`infra/statedb` は 7 個。
- app の機能どうしは import しない(設計書 2 節)。`app/reports` は `app/planning` を呼ばない。共通の数え方は `domain/planning` に置く。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書き、言い換えない。日付の表示は 01 の書き方(core の `formatDay`)。ポイントの表記は F1 の `pt`(core の `features/planning/sprints.ts`)、時間の表記は `formatHours`(core の `features/notes/hours.ts`)と Go の `worklog.Format`。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする(CI が差分を見る)。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| F3-1 | 数え方はすべて `domain/planning` に置く。新しいファイルは `report.go`(単位・選択・丸め)、`burndown.go`、`velocity.go`、`flow.go`、`cycle.go`、`estimate.go`(計 13 個)。非公開の `counted`・`idAt`・`ValueAt` をそのまま使い、確定値とバーンダウンの数え方をそろえる | 05 の 338 行の注記は「総量(開始日)と確定の計画が違うとき」だけ出す。数え方がずれると、手を入れていない完了スプリントでも注記が出る |
| F3-2 | 完了の記録の時刻と op を `planning.Sprint.Closing`(`*Closing{At time.Time; Op change.ID}`)で持つ。`confirmedOf` が使う `close_sprint` の行(`change.Current` が決めた 1 行)の `At` と `Op` | 完了による持ち越し(05 の 320 行)と、表示する日の上限(05 の 332 行の「完了した日」)に要る |
| F3-3 | 完了による持ち越し: アイテムのログに、`Op` がそのスプリントの `Closing.Op` の `set sprint` の行(完了で移した行)があれば、そのアイテムのスプリントの範囲は、日の終わりとその行の `At` のうち早い方の時点の値で求める(その行より前の行だけで `ValueAt`)。分類は日の終わりの時点のまま。表示する日は完了した日までなので、完了で移したアイテムは完了した日の終わりまで範囲に残る。完了の行をログから除いて求める形にはしない(同じ日にさらに別の枠へ移した行の `prev` が除いた行を指し、`change.Resolve` があとの行を先端に選んで、範囲から消えるため) | 05 の 320・332 行。F1 の `close.go` は、移す行を完了と同じ op で書く |
| F3-4 | 単位は `planning.Unit`(`points`・`count`)。ポイントを選んでいても、ワークスペースの削除済みでない作業項目に `Points` のあるものが 1 件もなければ件数で数え、`Fallback` を真にする(画面は「ポイントのあるアイテムがないため、件数で数えています」)。単位はバーンダウン・ベロシティ・スプリントのカードの小さいバーンダウンと「途中で追加」に効く。累積フロー図は件数、サイクルタイムと見積もりと実績は単位によらない。概要の完了ポイントは常にポイント | 05 の 193・296・305〜307 行 |
| F3-5 | レポートの選択は `planning.Selection{Unit; Sprint change.ID; Layers; Estimate change.ID}`(`Layers` は `category`・`status`、`Estimate` の空は「すべて」)。`state.db` の表 `report_selections`(Store ID・ワークスペース ID ごとに 1 行)に、利用者が選んだ項目だけを置く(選んでいない項目は空のまま。単位だけ変えた人の行のスプリントは空なので、次のスプリントを開始すると、バーンダウンは新しい実行中のスプリントになる)。読むときに直す: 選択がなければ単位はポイント・層は分類・対象はすべて。読めない単位・層は既定に戻す。バーンダウンのスプリントは、選択肢(F3-6)にあればそれ、なければ 05 の 310〜312 行の順(実行中 → 終了日が最も遅い完了、同じ終了日は開始日が遅いもの、それも同じなら名前の昇順で先 → 未選択)。見積もりと実績の対象は、選択肢にあればそれ、なければ「すべて」。2 つのスプリントは別々に確かめる(片方が消えても、もう片方の選択は残す) | 05 の 309〜312 行。保存形式 282 行。05 の 309 行の「保存したスプリント」は、選択ごとに確かめるのが利用者の選んだものを最も残すため |
| F3-6 | レポートのスプリントの選択肢(バーンダウンと、見積もりと実績の対象)は、削除済みでない実行中と完了のスプリントを開始日の新しい順。同じ開始日は名前の昇順 | 05 の 154・308・387 行。同じ日付の並びは F1 の画面の並びと同じく名前の昇順(05 の 453 行) |
| F3-7 | 選択の保存は、送られた項目だけを state.db の行(読むときに直す前の値)に重ねる。Store に書かない(`Stores.Write` を通さない)ので、閲覧のみの Store でも変えられる。ワークスペースがないか削除済みなら `port.ErrNotFound`。単位・層が読めない値なら `port.ErrInvalidOperation`(400)。スプリントの ID は確かめずに残す(読むときに F3-5 で直す) | 05 の 309・362 行 |
| F3-8 | 「小数第 2 位を四捨五入して小数第 1 位まで」は Go の `planning.Round1(x float64) float64` だけで行い、API は丸めた数を返す。画面は `toFixed(1)` で常に小数第 1 位まで書く(`3` は「3.0」)。`Round1` は、浮動小数点の誤差を消すために小数第 3 位で一度丸めてから、小数第 1 位に四捨五入する(`3.55` → `3.6`、`6.4` → `6.4`、`0.05` → `0.1`) | 05 の 351・380・390 行。丸める場所を 1 つにして、仕様の例(3.55 → 3.6)を Go のテストで確かめる |
| F3-9 | バーンダウンの表の「総量」は、表示する日より後の日も、その日の終わり時点で求めた値(いまの状態と同じ)を出す。「残り」だけ「—」 | 05 の 401 行(「—」にするのは「残り」だけと書いてある) |
| F3-10 | 「表示する日」が 0 日(開始日が今日より後)のときは、バーンダウンの図と表の代わりに「開始日({日付})から表示します」を出す。小さいバーンダウンも同じで、「途中で追加」も出さない | 05 の 192・193・332・405 行 |
| F3-11 | 累積フロー図の期間の検査は、画面で行い(合わなければ日付入力の下に理由を出し、図は直前の期間のまま)、API でも同じ文で 422 にする(`planning.CheckPeriod(from, to, today string) error`)。期間を渡さなければ直近 21 日(今日の 20 日前〜今日) | 05 の 357〜362 行 |
| F3-12 | 累積フロー図の層は API が並べる(下から上の順)。分類のときは完了・進行中・未着手・中止(キー `done`・`doing`・`todo`・`canceled`、色は画面が分類の色の token で決める)。ステータスのときは、分類の順(完了・進行中・未着手・中止)にまとめて、各分類の中はワークスペースのステータスの定義順。色はステータスの色。一覧にないステータスにいた分はキー `deleted`、名前「(削除された値)」の層にして一番上に積み、期間のどこかの日に 1 件以上あるときだけ出す。分類のときは、一覧にないステータスは未着手(05 の 17 行) | 05 の 365〜367 行 |
| F3-13 | ステータスやスプリントの時刻の並び(サイクルタイム・累積フロー図)は、アイテムのログの各行の時刻で `ValueAt` した値の列(`steps`)で求める。同じ値が続く点はまとめる。時計がずれて `at` が前後しても、`change.Resolve` が `prev` で決めた値を使う。アイテムごとに一度だけ作り、日ごとには作り直さない | 保存形式 267 行。F1-10 の `ValueAt` と同じ規則にそろえる |
| F3-14 | 見積もりと実績の合計の行: ポイントは行のポイントの合計で、ポイントのある行がなければ「—」。見積もり時間も、見積もり時間のある行がなければ「—」。差は見積もり時間のある行の差の合計で、なければ「—」。差が 0 の行は「0 時間」(符号なし、警告の色にしない)。作業時間と見積もり時間は 100 分の 1 時間の整数で持つ(見積もり時間は `math.Round(e*100)`) | 05 の 390〜392 行。「ないときは「—」」の列の規則を合計の行にも当てる |
| F3-15 | スプリントのカードの小さいバーンダウンと「途中で追加」、概要の完了ポイントは、新しい API `GET …/reports/active`(`app/reports` の `Active`)で読む。F1 の `SprintCards` と `get-planning` は変えない | `app/reports` が数え方と選択(単位)を持つので、F1 の `app/planning` に選択の口を足さない |
| F3-16 | 「バーンダウンを見る」は、そのスプリントをバーンダウンのスプリントにした選択を保存してから、レポートのタブへ移る。閲覧のみの Store でも押せる(選択は Store に書かない)。計画中は無効で「計画中のスプリントにはバーンダウンがありません」。メニューの並びは「編集…」「バーンダウンを見る」「削除…」 | 05 の 202〜208 行、F3-7 |
| F3-17 | ワークスペースのタブは「概要・スプリント・リリース・レポート・設定」にする(作ったものだけ出す、F1-20)。アドレスは `/s/{store}/w/{prefix}/reports` | 01 の並び、F1-20 |
| F3-18 | 概要のタブに、統計のカードの並び(04「統計」)の場所を作り、F3 は「{スプリント名} の完了ポイント」のカードだけを置く。ほかの 3 枚(プロジェクト・未完了のアイテム・期限切れ)と、プロジェクトの一覧・最近の変更は、04 を持つトラック E が同じ並びに足す | ユーザーの決定(F3 の範囲に概要の完了ポイントを入れる、2026-11-14)。04 の統計の残りは E の持ち場 |
| F3-19 | 画面の読み直し: Store の出来事(06 の 155 行)で、開いているレポートと、スプリントの画面・概要の `active` を読み直す(`wire.ts` の `reflect` に足す)。日付が変わったとき(`dayOf($syncNow)` が変わったとき)も読み直す(01 の 132 行)。累積フロー図の期間が初めの期間のときは、日付が変わると新しい直近 21 日にする | 06 の 155 行、01 の 132 行 |
| F3-20 | 縦軸の切りのよい最大値は、最大値以上で最小の `{1, 2, 5} × 10^k`(k は整数。1 未満の最大値は 1)。値がすべて 0 なら 1。目盛りは 0・最大値の半分・最大値の 3 本 | 05 の 339・348・368 行 |
| F3-21 | サイクルタイム・リードタイムは、削除済みでない、いまこのワークスペースにある作業項目のうち、いまの分類が完了のもの。日数は(時刻の差の秒)÷ 86400。中央値と 85% の値を `Round1` で丸める。サイクルタイムの「着手を経ていない {M} 件」は、完了に到達した記録はあるが着手の時刻がないものの数 | 05 の 374〜381 行 |

## Review Focus

1. **完了で移したアイテムが、完了した日の残りから消える。** 実行中に未完了 2 件(5 pt)のまま完了してバックログへ移したスプリントで、完了した日の「残り」が 5 pt のまま残る(F3-3)。Task 2 の `TestBurnCarryOver` に入れる。
2. **手を入れていない完了スプリントに、確定の計画の注記が出る。** 開始日に入れて完了まで何も変えなかったスプリントで、`Committed` が nil になる。途中で足したアイテムがあっても同じ(計画にも総量(開始日)にも入らない)(F3-1)。Task 2 の `TestBurnCommittedNote` に入れる。
3. **再び開いて完了したアイテムのサイクルタイムが、最初の着手から数えられる。** 完了 → 進行中 → 完了のアイテムは、再び開いた時刻から数える。完了の分類どうし(Done → Released)の移動では到達の時刻が延びない(05 の 375・376 行)。Task 5 のテストに入れる。
4. **選んだスプリントを削除したら、レポートの画面が空になる。** 保存したバーンダウンのスプリントが削除済み・計画中・存在しないとき、既定の順で選び直す。見積もりと実績の対象は「すべて」に戻すが、バーンダウンの選択は残す(F3-5)。Task 1 の `TestResolveSelection` と Task 8 のテストに入れる。
5. **ポイントのないワークスペースで、図が 0 のまま。** ポイントが 1 件もないワークスペースでは件数で数え、注記を出す。スプリントのカードの小さいバーンダウンも件数になる(F3-4)。Task 1 の `TestEffectiveUnit` と Task 8 のテストに入れる。

---

## ファイルの地図

```
internal/domain/planning/plan.go          (変更)Sprint.Closing
internal/domain/planning/report.go        Unit・Measure・Layers・Selection・ReportSprints・ResolveSelection・EffectiveUnit・Round1・CheckPeriod・steps
internal/domain/planning/burndown.go      BurnDay・Burndown・Burn・Added・Progress
internal/domain/planning/velocity.go      VelocityBar・Velocity
internal/domain/planning/flow.go          Layer・FlowDay・Flow
internal/domain/planning/cycle.go         Durations・Cycle
internal/domain/planning/estimate.go      EstimateRow・EstimateTotal・Estimates
internal/infra/statedb/reports.go         report_selections の表と ReportSelections()
internal/app/reports/service.go           Selections・Service・Select
internal/app/reports/report.go            Period・Report・Service.Report
internal/app/reports/active.go            Active・Service.Active
internal/adapter/httpapi/reports/*.go     API
internal/adapter/httpapi/routes.go        (変更)Deps.Reports と Register
cmd/hedgehug/wire.go・wire_e2e.go         (変更)組み立て
web/packages/core/src/model/report.ts・port/reports.ts
web/packages/core/src/features/reports/chart.ts・reports.ts・texts.ts・flowPeriod.ts・estimates.ts
web/packages/core/src/features/planning/sprints.ts   (変更)sprintMenu の「バーンダウンを見る」
web/packages/core/src/features/router/route.ts       (変更)WorkspaceTab に reports
web/packages/api/src/reports.ts
web/app/src/screens/reports/*.svelte
web/app/src/screens/planning/SprintCard.svelte・SprintBurndown.svelte・SprintsTab.svelte (変更・新規)
web/app/src/screens/workspace/WorkspaceScreen.svelte・OverviewTab.svelte・SprintPointsCard.svelte
web/app/src/wire.ts                       (変更)reports と読み直し
web/e2e/reports.spec.ts
```

---

### Task 1: 単位・選択・丸めと、完了の記録の時刻(`domain/planning`)

**Files:**
- Modify: `internal/domain/planning/plan.go`(`Sprint.Closing`、`confirmedOf` が行も返す)
- Create: `internal/domain/planning/report.go`
- Test: `internal/domain/planning/report_test.go`、`plan_test.go`(追記)

**Interfaces:**
- Produces:
  - `type Closing struct { At time.Time; Op change.ID }`、`Sprint.Closing *Closing`(完了の記録がなければ nil)
  - `type Unit string`、`const UnitPoints Unit = "points"`、`UnitCount Unit = "count"`
  - `func Measure(it item.Item, u Unit) float64` — ポイント(なしは 0)か 1
  - `type Layers string`、`const LayersCategory Layers = "category"`、`LayersStatus Layers = "status"`
  - `type Selection struct { Unit Unit; Sprint change.ID; Layers Layers; Estimate change.ID }`
  - `func ReportSprints(live []Sprint) []Sprint` — F3-6
  - `func ResolveSelection(saved Selection, found bool, live []Sprint) Selection` — F3-5
  - `func EffectiveUnit(its []item.Item, ws workspace.ID, u Unit) (Unit, bool)` — F3-4。`its` は削除済みを含んでよい
  - `func Round1(x float64) float64` — F3-8
  - `func CheckPeriod(from, to, today string) error` — F3-11。`refusal.New` で断る
  - `func steps(log []change.Record, f string) []step`、`type step struct { At time.Time; Value change.ID; Created bool }` — F3-13。作成の行の時刻を最初の点にする(`Created` が真)

- [ ] **Step 1: 失敗するテストを書く**

- `TestSprintClosing`: 完了の行(`t: close_sprint`)の `At`・`Op` が `Closing` に入る。同時に 2 回完了したログでは `Confirmed` と同じ行の `At`・`Op`。完了していなければ nil。
- `TestResolveSelection`: スプリント A(実行中、開始 11/10)、B(完了、終了 11/09、開始 10/27)、C(完了、終了 11/09、開始 10/28)、D(計画中)、E(削除済みの完了)。(a) 選択なし → `{points, A, category, ""}`。(b) 保存した Sprint が D・E・存在しない ID → A。(c) A がなく保存なし → C(終了日が同じなら開始日が遅いもの)。(d) 実行中も完了もない → Sprint は空。(e) Sprint が B で Estimate が E → Sprint は B のまま、Estimate は空。(f) Unit `"hours"`・Layers `"x"` → `points`・`category`。
- `TestReportSprints`: 実行中と完了だけを開始日の新しい順、同じ開始日は名前の昇順。計画中と削除済みは出さない。
- `TestEffectiveUnit`: ポイントのある作業項目が 1 件でもあれば `(points, false)`。Epic だけにポイントがある・削除済みのものだけにある・別のワークスペースのものだけにある → `(count, true)`。`count` を渡せば `(count, false)`。
- `TestRound1`: `3.55` → `3.6`、`6.4` → `6.4`、補間で作った `3 + 0.55*(4-3)`(浮動小数点で 3.55 ちょうどにならない値)→ `3.6`、`0.05` → `0.1`、`2.25` → `2.3`、`0` → `0`。
- `TestCheckPeriod`(今日 2026-11-14): 開始が空 → 「開始日を入力してください」、終了が空 → 「終了日を入力してください」、`2026-11-10`〜`2026-11-09` → 「終了日は開始日以降にしてください」、終了 `2026-11-15` → 「終了日は今日以前にしてください」、同じ日は nil。両方空なら開始の文。
- `TestSteps`: 作成(Todo)→ InProgress → Done の行で 3 点。同じ値の `set` が続けば 1 点にまとめる。あとの行の `at` が前の行より前(時計のずれ)でも `prev` でつながっていれば、あとの行の値が最後の点。作成の行がなければ空。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: FAIL(`Closing`・`ResolveSelection` などが未定義)

- [ ] **Step 3: 書く**

`Round1` は `math.Round(math.Round(x*1000)/100) / 10`(小数第 3 位で誤差を消してから、小数第 1 位に四捨五入)。`steps` は、ログの行を `At` の順(同じなら ID の順)に並べ、各行の `At` の直後(`At.Add(time.Nanosecond)`)で `ValueAt` した値を点にする。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): レポートの単位・選択・丸めと、完了の記録の時刻を持つ(F3 Task 1)"
```

### Task 2: バーンダウン・途中で追加・完了ポイント(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/burndown.go`
- Test: `internal/domain/planning/burndown_test.go`

**Interfaces:**
- Consumes: Task 1 の `Closing`・`Unit`・`Measure`、F1 の `ValueAt`・`EndOfDay`・`Today`・`Category`・`counted`・`idAt`・`Confirm`
- Produces:
  - `type BurnDay struct { Date string; Total float64; Remaining *float64; Ideal float64 }`
  - `type Burndown struct { Days []BurnDay; Shown int; Committed *float64 }` — `Days` は開始日〜終了日のすべての日。`Shown` は表示する日の数(05 の 332 行)。`Remaining` は `Shown` より後の日で nil。`Committed` は、完了したスプリントで、総量(開始日)が確定の計画(単位に合わせた値)と違うときだけ、その確定の計画
  - `func Burn(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, u Unit, now time.Time) Burndown`
  - `func Added(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, u Unit, now time.Time) float64` — 05 の 192 行。表示する日が 0 日なら 0
  - `func Progress(its []item.Item, ws workspace.ID, sprint change.ID, st workspace.Settings) (done, total Totals)` — 05 の 295・296 行(中止を除く作業項目。完了の分類のものが done)

- [ ] **Step 1: 失敗するテストを書く**

時刻帯は F1 の `timeline_test.go` と同じく `time.Local` で決める。スプリント S1 は開始 2026-11-02、終了 2026-11-05(4 日)。

- `TestBurnDaysAndIdeal`: 11/1 に S1 に入れた A(3 pt)・B(5 pt)。B を 11/3 に完了。今日 11/4、実行中。`Days` は 11/02〜11/05 の 4 件、`Shown` は 3。総量は 8・8・8・8、残りは 8・8・3・nil、理想は 8・5.333…・2.666…・0(`8 - 8*k/3`)。
- `TestBurnScopeChange`: 11/3 に C(2 pt)を足す → 11/3 から総量 10。11/4 に A を中止 → 11/4 から総量 7(中止を除く)。11/3 に外した D は 11/3 から総量に入らない。理想は開始日の総量から引き、引き直さない。
- `TestBurnCarryOver`(Review Focus 1): 11/4 15:00 に完了し、未完了の A(3 pt)・C(2 pt)を S2 へ移す(完了の行と同じ op の `set sprint`)。今日 11/6。`Shown` は 3(11/02〜11/04)、11/04 の残りは 5。A を同じ 11/4 の 16:00 にバックログへ移しても(その行の `prev` は完了で移した行)、11/04 の残りは 5 のまま。11/4 の 14:00(完了の前)に S1 から外した B は、11/04 の範囲に入らない。
- `TestBurnCommittedNote`(Review Focus 2): 開始日に入れて何も変えずに完了したスプリントは `Committed` が nil。途中(11/3)に足したアイテムがあっても nil。完了のあとに開始日からあったアイテムを削除したら、`Committed` が確定の計画の値。件数の単位では、件数の確定値で比べる。
- `TestBurnFutureStart`: 開始日が明日なら `Shown` は 0、すべての `Remaining` は nil。
- `TestBurnUnitCount`: 件数の単位では、ポイントのないアイテムも 1 件。
- `TestBurnExcluded`: 削除済みのアイテム・いま別のワークスペースにあるアイテム・Epic は、過去の日も数えない。作成のときにスプリントを選んだアイテムは、作成の日から範囲に入る。スプリントの削除で外れたアイテムは、外れた日から範囲に入らない。
- `TestAdded`: 開始日の終わりに A・B、今日 C(2 pt)と D(中止、1 pt)がある → 2。開始日にあって今日外れたものは引かない。今日が開始日より前なら 0。
- `TestProgress`: 中止を除いた作業項目のポイントの合計と、完了の分類のものの合計。Epic と削除済みは数えない。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/ -run 'Burn|Added|Progress'`
Expected: FAIL

- [ ] **Step 3: 書く**

範囲の判定は `idAt(w.Log, item.SprintField, at) == s.ID`。`at` は `EndOfDay(d, time.Local)` と、そのアイテムの完了で移した行(F3-3)の `At` のうち早い方。分類は `EndOfDay(d, time.Local)` の時点の `idAt(…, "status", …)` を `Category` に渡す。量は `Measure(w.Item, u)`(いまの値)。表示する日の最後は、終了日・今日・(`Closing` があれば)`Today(Closing.At)` のうち最も早い日。`Committed` を出すかの比べ方は、両方を `strconv.FormatFloat(v, 'g', 12, 64)` で 12 桁の文字列にしてから比べる(画面の `pt` と同じ桁。0.1 + 0.2 の誤差で注記を出さない)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): バーンダウン・途中で追加・完了ポイントを数える(F3 Task 2)"
```

### Task 3: ベロシティ(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/velocity.go`
- Test: `internal/domain/planning/velocity_test.go`

**Interfaces:**
- Consumes: F1 の `Confirm`、Task 1 の `Unit`・`Round1`
- Produces:
  - `type VelocityBar struct { Sprint Sprint; Planned, Done float64 }`
  - `func Velocity(works []Work, ws workspace.ID, live []Sprint, st workspace.Settings, u Unit, now time.Time) ([]VelocityBar, *float64)` — 棒は削除済みでない実行中と完了を開始日の早い順(同じ開始日は名前の昇順)。2 つ目の値は「完了した直近 3 スプリントの平均」(完了がなければ nil)

- [ ] **Step 1: 失敗するテストを書く**

- `TestVelocityClosedUsesConfirmed`: 完了したスプリントは `Confirmed` の値(ポイントの単位なら `Points`、件数なら `Count`)。完了のあとにアイテムを変えても変わらない。`Confirmed` が nil なら 0。
- `TestVelocityActive`: 実行中のスプリントの計画は `Confirm` の `Committed`、完了は今日の終わりの範囲の完了の分類のもの。
- `TestVelocityAverage`: 完了 4 件(開始日の順に完了 1・2・3・5 pt)と実行中 1 件 → 平均は直近 3 件(2・3・5)で `3.3`。完了 2 件(1・2)→ `1.5`。完了 1 件(4)→ `4`。完了なし → nil。
- `TestVelocityOrder`: 計画中と削除済みは棒に出さない。開始日の早い順。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/ -run Velocity`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): ベロシティと完了した直近 3 スプリントの平均を数える(F3 Task 3)"
```

### Task 4: 累積フロー図(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/flow.go`
- Test: `internal/domain/planning/flow_test.go`

**Interfaces:**
- Consumes: Task 1 の `steps`・`Layers`、F1 の `Category`・`EndOfDay`・`counted`
- Produces:
  - `type Layer struct { Key, Name, Color string }` — F3-12。分類の層の `Color` は空
  - `type FlowDay struct { Date string; Counts []int }` — `Counts` は層と同じ順
  - `func Flow(works []Work, ws workspace.ID, st workspace.Settings, from, to string, l Layers) ([]Layer, []FlowDay)` — `from`〜`to` のすべての日。層は下から上の順

- [ ] **Step 1: 失敗するテストを書く**

設定のステータスは既定(Todo・InProgress・Done・Pending・Cancel、`workspace` の既定値)。

- `TestFlowCategory`: 11/1 作成(Todo)、11/2 に InProgress、11/3 に Done の A と、11/2 作成で 11/3 に Cancel の B。期間 11/01〜11/03 → 層のキーは `done`・`doing`・`todo`・`canceled`、名前は 完了・進行中・未着手・中止。11/01 は `[0,0,1,0]`、11/02 は `[0,1,1,0]`、11/03 は `[1,0,0,1]`。
- `TestFlowStatus`: ステータスの層は Done・InProgress・Todo・Pending・Cancel の順(分類の順、中は定義の順)、名前と色はステータスのもの。一覧にないステータスにいた日があれば、最後(一番上)に `deleted`「(削除された値)」の層が付き、なければ付かない。分類の層では、一覧にないステータスは未着手に数える。
- `TestFlowExcluded`: 作成前の日は数えない。削除済み・別のワークスペース・Epic・Step は数えない。中止は数える。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/ -run Flow`
Expected: FAIL

- [ ] **Step 3: 書く**

アイテムごとに `steps(w.Log, "status")` を一度作り、各日の `EndOfDay` より前の最後の点の値を使う(F3-13)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): 累積フロー図を分類とステータスの層で数える(F3 Task 4)"
```

### Task 5: サイクルタイムとリードタイム(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/cycle.go`
- Test: `internal/domain/planning/cycle_test.go`

**Interfaces:**
- Consumes: Task 1 の `steps`・`Round1`
- Produces:
  - `type Durations struct { N int; Median, P85 *float64; Skipped int }` — `N` は数えた件数、`Median`・`P85` は日数(`Round1` 済み。0 件なら nil)。`Skipped` はサイクルタイムで着手の時刻がなかった件数(リードタイムでは 0)
  - `func Cycle(works []Work, ws workspace.ID, st workspace.Settings) (cycle, lead Durations)` — F3-21

- [ ] **Step 1: 失敗するテストを書く**

- `TestCycleBasic`: 11/1 0:00 作成(Todo)、11/2 0:00 InProgress、11/4 12:00 Done → サイクル 2.5 日、リード 3.5 日。
- `TestCycleReopened`(Review Focus 3。設定に完了の分類のステータス Released を足す): 11/1 作成 → 11/2 InProgress → 11/3 Done → 11/5 InProgress → 11/6 Done → サイクルは 11/5〜11/6 の 1 日、リードは 11/1〜11/6 の 5 日。Done から Released(完了の分類のもう 1 つのステータス)へ 11/8 に移しても変わらない。
- `TestCycleNoStart`: Todo から直接 Done → リードには数え、サイクルには数えず `Skipped` が 1。作成のときから Done のもの → どちらにも数えない。いまの分類が完了でないもの・中止 → 数えない。作成のときから InProgress で Done にしたもの → 着手は作成の時刻。
- `TestCycleNegative`: Done の行の `at` が作成より前(時計のずれ)→ 0 日。
- `TestCycleStats`: 日数 1・2・3・4・10 → 中央値 3、85% は 6.4。1・2・3・4 → 中央値 2.5、85% は 3.6。0 件 → `Median`・`P85` は nil、`N` は 0。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/ -run Cycle`
Expected: FAIL

- [ ] **Step 3: 書く**

到達の時刻は、`steps` の分類が完了以外から完了へ変わった最後の点。区間の始まりは、到達より前で完了から完了以外へ変わった最後の点(なければ作成の点)。着手は、区間の始まり以後・到達より前で、最初に分類が進行中である点。85% の値は、昇順に並べた日数の位置 `(N-1)*0.85` の前後 2 つの線形補間。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): サイクルタイムとリードタイムの中央値と 85% の値を求める(F3 Task 5)"
```

### Task 6: 見積もりと実績(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/estimate.go`
- Test: `internal/domain/planning/estimate_test.go`

**Interfaces:**
- Consumes: `worklog.FromLog`、Task 1 の `Round1`
- Produces:
  - `type EstimateRow struct { Item item.Item; Points *float64; Estimate *int; Worked int; Diff *int; PerPoint *float64 }` — 時間は 100 分の 1 時間の整数(F3-14)。`PerPoint` は時間/pt(`Round1` 済み。ポイントがないか 0 なら nil)
  - `type EstimateTotal struct { Points *float64; Estimate *int; Worked int; Diff *int }`
  - `func Estimates(works []Work, ws workspace.ID, sprint change.ID) ([]EstimateRow, EstimateTotal)` — `sprint` が空なら「すべて」。行は表示 ID の番号(`Place.Number`)の順

- [ ] **Step 1: 失敗するテストを書く**

- `TestEstimatesRows`: A(3 pt、見積もり 4 時間、作業 2.5 時間 + 2.5 時間)→ 作業 500、差 +100、時間/pt 1.7(5 ÷ 3 = 1.666…)。B(見積もりなし、0 pt、作業 1 時間)→ 差 nil、時間/pt nil。C(中止、見積もり 2 時間、作業 1.75 時間)→ 入る、差 -25。D(ポイントも見積もりもない、作業あり)・E(見積もりあり、作業なし)・Epic・削除済み → 入らない。番号の順。
- `TestEstimatesTotal`: 上の A・B・C で、ポイント 3、見積もり 600、作業 825、差 75(A と C の差の合計)。見積もりのある行がなければ `Estimate`・`Diff` は nil、ポイントのある行がなければ `Points` は nil。
- `TestEstimatesSprint`: `sprint` を渡すと、いまそのスプリントに入っているものだけ。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/ -run Estimates`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): 見積もりと実績の行と合計を求める(F3 Task 6)"
```

### Task 7: レポートの選択を state.db に置く(`infra/statedb`)

**Files:**
- Create: `internal/infra/statedb/reports.go`
- Test: `internal/infra/statedb/reports_test.go`

**Interfaces:**
- Consumes: Task 1 の `planning.Selection`
- Produces: `func (d *DB) ReportSelections() ReportSelections`、`type ReportSelections struct{ db *sql.DB }` に
  - `Get(ctx context.Context, store port.StoreID, ws workspace.ID) (planning.Selection, bool, error)`
  - `Put(ctx context.Context, store port.StoreID, ws workspace.ID, sel planning.Selection) error`
  (Task 8 の `reports.Selections` を満たす。statedb は app/reports を import しない)

- [ ] **Step 1: 失敗するテストを書く**

- `TestReportSelections`: なければ `false`。`Put` のあと `Get` で同じ値(空の `Sprint`・`Estimate` も保つ)。2 回目の `Put` で上書き。別の Store・別のワークスペースの行と混ざらない。開き直しても残る(`Open` を 2 回)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/statedb/ -run ReportSelections`
Expected: FAIL

- [ ] **Step 3: 書く**

`register("reports/001", …)` で表 `report_selections (store TEXT NOT NULL, workspace TEXT NOT NULL, unit TEXT NOT NULL, sprint TEXT NOT NULL, layers TEXT NOT NULL, estimate TEXT NOT NULL, PRIMARY KEY (store, workspace))`。書き込みは `INSERT … ON CONFLICT (store, workspace) DO UPDATE`。列に Store ID を持つので、G が作る「このマシンから削除」の Store ごとの行の削除(保存形式 284 行)がこの表にも効く(末尾「G に求めること」)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/statedb/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/statedb
git commit -m "feat(statedb): レポートの選択をワークスペースごとに保存する(F3 Task 7)"
```

### Task 8: レポートの読み取りと選択の保存(`app/reports`)

**Files:**
- Create: `internal/app/reports/service.go`、`report.go`、`active.go`
- Test: `internal/app/reports/report_test.go`、`active_test.go`、`select_test.go`(メモリ上の `Selections` の偽物はこのパッケージのテストに置く)

**Interfaces:**
- Consumes: Task 1〜6 の `domain/planning`、`port.LiveWorkspace`、`port.Stores`・`port.Clock`
- Produces:
  - `type Selections interface { Get(ctx context.Context, store port.StoreID, ws workspace.ID) (planning.Selection, bool, error); Put(ctx context.Context, store port.StoreID, ws workspace.ID, sel planning.Selection) error }`
  - `type Service struct { Stores port.Stores; Selections Selections; Clock port.Clock }`
  - `type Period struct { From, To string }`(両方空なら直近 21 日)
  - `type Report struct { Selection planning.Selection; Unit planning.Unit; Fallback bool; Sprints []planning.Sprint; Burn *planning.Sprint; Burndown planning.Burndown; Velocity []planning.VelocityBar; Average *float64; Period Period; Layers []planning.Layer; Flow []planning.FlowDay; Cycle, Lead planning.Durations; Estimates []EstimateRow; EstimateTotal planning.EstimateTotal }` — `Selection` は F3-5 で直したもの。`Burn` はバーンダウンのスプリント(未選択なら nil)。`Period` は使った期間
  - `type EstimateRow struct { planning.EstimateRow; Display string }` — `Display` は表示 ID(`HH-12`)
  - `func (s Service) Report(ctx context.Context, store port.StoreID, ws workspace.ID, p Period) (Report, error)` — 期間が合わなければ `CheckPeriod` の断り(422)
  - `type Active struct { Sprint *planning.Sprint; Unit planning.Unit; Burndown planning.Burndown; Added float64; Done, Total planning.Totals }` — `Sprint` は実行中(なければ nil で、ほかは空)。`Done`・`Total` は `Progress`
  - `func (s Service) Active(ctx context.Context, store port.StoreID, ws workspace.ID) (Active, error)`
  - `type Patch struct { Unit *planning.Unit; Sprint *change.ID; Layers *planning.Layers; Estimate *change.ID }` — nil の項目は変えない
  - `func (s Service) Select(ctx context.Context, store port.StoreID, ws workspace.ID, p Patch) error` — F3-7。`Selections.Get` の生の行に重ねて `Put` する

- [ ] **Step 1: 失敗するテストを書く**

`apptest` のメモリ上の Store にワークスペース・スプリント・アイテムを書いて確かめる(F1 の `app/planning` のテストと同じ組み方)。時計は `apptest` の固定の時計。

- `TestReportDefaults`: 選択の保存がない → 単位ポイント、層は分類、対象はすべて、バーンダウンは実行中のスプリント、期間は今日の 20 日前〜今日。
- `TestReportSavedSprintGone`(Review Focus 4): 保存したバーンダウンのスプリントを削除する → 実行中のスプリントを選ぶ。保存した対象のスプリントを削除する → 対象は「すべて」、バーンダウンの選択は残る。
- `TestReportFallback`(Review Focus 5): ポイントのある作業項目がない → `Unit` は件数、`Fallback` は真。`Active` の `Unit` も件数。
- `TestReportPeriodRefusal`: 終了日が今日より後 → 「終了日は今日以前にしてください」の断り。
- `TestReportGoneWorkspace`: 削除済みのワークスペース → `port.ErrNotFound`。
- `TestActive`: 実行中のスプリントがない → `Sprint` が nil。あれば `Done`・`Total` は中止を除いたポイント、`Added` は単位に合わせる。
- `TestSelectPatch`: 単位だけ `count` にしたあと、別のスプリントを開始すると、`Report` のバーンダウンは新しい実行中のスプリント(保存した行の `Sprint` は空のまま)。バーンダウンのスプリントだけを送ると、前に保存した単位は残る。
- `TestSelect`: 閲覧のみの Store でも保存できる(Store にコミットが増えない)。単位 `"hours"` → `port.ErrInvalidOperation`。削除済みのワークスペース → `port.ErrNotFound`。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/reports/`
Expected: FAIL

- [ ] **Step 3: 書く**

読み取りは 1 回の `Stores.Read` の中で、`port.LiveWorkspace`、ワークスペースのログ(`planning.FromLog`)、設定、全プロジェクトの全アイテムとそのログ(F1 の `app/planning` の `works` と同じ読み方)を読む。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/reports/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/reports
git commit -m "feat(reports): レポートと実行中のスプリントの数字を読み、選択を保存する(F3 Task 8)"
```

### Task 9: API と組み立て

**Files:**
- Create: `internal/adapter/httpapi/reports/reports.go`、`dto.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps.Reports appreports.Selections` と `reports.Register`)、`cmd/hedgehug/wire.go`(`Reports: db.ReportSelections()`)、`cmd/hedgehug/wire_e2e.go`(`e2eReads` の state.db の `ReportSelections()`)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(生成)
- Test: `internal/adapter/httpapi/reports/reports_test.go`

**Interfaces:**
- Consumes: Task 8
- Produces(パスの頭は `/api/stores/{store}/workspaces/{workspace}`。本文の型の名前は `Report` で始める):
  - `GET …/reports?from=&to=`(`get-report`)→ `ReportView`。日付は `YYYY-MM-DD` か空でなければ 422
  - `GET …/reports/active`(`get-report-active`)→ `ReportActive`
  - `PUT …/reports/selection`(`put-report-selection`)本文 `ReportSelectionPatch{unit?, sprint?, layers?, estimate?}`(どれも省略可。省略した項目は変えない)→ 204
  - DTO: スプリントは F1 の `Sprint` の DTO と同じ項目の `ReportSprint`(OpenAPI のスキーマ名が重ならないよう別の型にする)。`Burndown{days[{date, total, remaining(null可), ideal}], shown, committed(null可)}`、`velocity[{sprint, planned, done}]`、`average(null可)`、`period{from, to}`、`layers[{key, name, color}]`、`flow[{date, counts[]}]`、`cycle`・`lead{n, median, p85, skipped}`、`estimates[{id, display, title, points, estimate, worked, diff, perPoint}]`、`estimateTotal{points, estimate, worked, diff}`、`unit`、`fallback`、`selection`、`sprints[]`、`burn(null可)`

- [ ] **Step 1: 失敗するテストを書く**

- `TestGetReport`: サンプルのワークスペースで 200、`unit` が `points`、`period` が直近 21 日。`from=2026-13-01` → 422。`to` が明日 → 422 で本文に「終了日は今日以前にしてください」。
- `TestPutSelection`: `{unit: "count"}` で 204 のあと `GET` の `selection.unit` が `count`。`unit: "x"` → 400。ないワークスペース → 404。
- `TestGetActive`: 実行中のスプリントがなければ `sprint` が null。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/reports/`
Expected: FAIL

- [ ] **Step 3: 書き、組み立てと生成を行う**

Run: `make generate`
Expected: `api/openapi.yaml` と `web/packages/api/src/generated.ts` に 3 つの操作が足される

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/adapter/... ./cmd/... && go build -tags e2e ./cmd/hedgehug`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/adapter internal/app cmd api web/packages/api/src/generated.ts
git commit -m "feat(api): レポートの読み取りと選択の保存の API を足す(F3 Task 9)"
```

### Task 10: 画面の型・口・API

**Files:**
- Create: `web/packages/core/src/model/report.ts`、`web/packages/core/src/port/reports.ts`、`web/packages/api/src/reports.ts`
- Modify: `web/packages/core/src/index.ts`(または該当の index)、`web/packages/api/src/index.ts`
- Test: `web/packages/api/src/reports.test.ts`

**Interfaces:**
- Consumes: Task 9 の生成された関数 `getReport`・`getReportActive`・`putReportSelection`
- Produces:
  - core の型: `ReportUnit = "points" | "count"`、`ReportLayers = "category" | "status"`、`ReportSelection{unit; sprint: string; layers; estimate: string}`、`ReportSprint`(F1 の `Sprint` と同じ形。`type ReportSprint = Sprint`)、`BurnDay{date; total; remaining: number | null; ideal}`、`Burndown{days; shown; committed: number | null}`、`VelocityBar{sprint: ReportSprint; planned; done}`、`FlowLayer{key; name; color}`、`FlowDay{date; counts: number[]}`、`Durations{n; median: number | null; p85: number | null; skipped}`、`EstimateRow{id; display; title; points: number | null; estimate: number | null; worked; diff: number | null; perPoint: number | null}`、`EstimateTotal{points: number | null; estimate: number | null; worked; diff: number | null}`、`Report{selection; unit; fallback; sprints; burn: ReportSprint | null; burndown; velocity; average: number | null; period: {from; to}; layers; flow; cycle; lead; estimates; estimateTotal}`、`ReportActive{sprint: ReportSprint | null; unit; burndown; added; done: Totals; total: Totals}`
  - `interface ReportsPort { report(storeId: string, workspaceId: string, period: { from: string; to: string } | null): Promise<Report>; active(storeId: string, workspaceId: string): Promise<ReportActive>; select(storeId: string, workspaceId: string, patch: Partial<ReportSelection>): Promise<void> }`
  - `function createReportsPort(): ReportsPort`(空の一覧が null で届いたら空にする。F1 の `planning.ts` と同じ)

- [ ] **Step 1: 失敗するテストを書く**

- `reports.test.ts`: `report` が `from`・`to` を問い合わせの文字列に付ける(`null` なら付けない)。null で届いた `velocity`・`flow`・`estimates`・`days` を空の配列にする。422 の断りは `call` が投げるもの(F1 と同じ)になる。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- reports`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/api test -- reports`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages
git commit -m "feat(web): レポートの型と口を足す(F3 Task 10)"
```

### Task 11: 図の座標(`core/features/reports/chart.ts`)

**Files:**
- Create: `web/packages/core/src/features/reports/chart.ts`
- Test: `web/packages/core/src/features/reports/chart.test.ts`

**Interfaces:**
- Produces:
  - `niceMax(max: number): number` — F3-20
  - `ticks(max: number): number[]` — `[0, max / 2, max]`
  - `type Frame = { width: number; height: number }`
  - `linePoints(values: (number | null)[], max: number, f: Frame): { x: number; y: number }[]` — null の点は飛ばす(x は位置で決める。点が 1 つなら x は 0)
  - `pathOf(points: { x: number; y: number }[]): string` — `M x y L x y …`。点がなければ空文字
  - `stackPaths(days: number[][], max: number, f: Frame): string[]` — 層ごとの面の `d`(下の層から)
  - `bars(groups: number[][], max: number, f: Frame): { x: number; y: number; width: number; height: number }[][]` — 群ごとに棒(ベロシティは 1 群に計画・完了の 2 本)

- [ ] **Step 1: 失敗するテストを書く**

- `niceMax`: `0` → 1、`0.4` → 1、`1` → 1、`3` → 5、`7` → 10、`12` → 20、`45` → 50、`50` → 50、`51` → 100。
- `linePoints`: `[8, 4, 0]`、max 8、幅 200・高さ 100 → `(0,0)`・`(100,50)`・`(200,100)`。`[8, null]` は 1 点。
- `stackPaths`: 2 層・2 日 `[[1,1],[2,0]]`、max 4、幅 100・高さ 100 → 下の層の上端が y=75、上の層の上端が 1 日目 y=25・2 日目 y=75。
- `bars`: 2 群、max 10、`[[10,5],[0,0]]` → 1 本目の高さが f.height、2 本目が半分、0 の棒の高さは 0。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- chart`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- chart`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(web): 図の縦軸と座標を求める(F3 Task 11)"
```

### Task 12: レポートの画面の状態と文(`core/features/reports`)

**Files:**
- Create: `web/packages/core/src/features/reports/reports.ts`、`texts.ts`、`flowPeriod.ts`、`estimates.ts`
- Modify: `web/packages/core/src/features/router/route.ts`(`WorkspaceTab` に `"reports"`、`workspaceTabOf("reports")`)
- Test: 同じ名前の `*.test.ts`、`route.test.ts`(追記)

**Interfaces:**
- Consumes: Task 10 の `ReportsPort`、F1 の `pill`・`pt`、`formatDay`、`formatHours`
- Produces:
  - `reports.ts`: `interface ReportsState { storeId; workspaceId; report: Report }`、`interface Reports { state: Readable<ReportsState | null>; open(storeId, workspaceId): Promise<void>; reload(): Promise<void>; close(): void; select(patch: Partial<ReportSelection>): Promise<void>; setPeriod(p: { from: string; to: string } | null): Promise<void> }`、`createReports(port: ReportsPort): Reports`。`select` は変えた項目だけを保存してから読み直す(保存に失敗したらエラーのトースト「保存できませんでした」を出せるよう、投げる)。期間は `Reports` の中だけに持ち、`close` したら(画面を離れたら)初めの期間に戻す(05 の 362 行)
  - `interface ActiveReports { state: Readable<{ storeId; workspaceId; active: ReportActive } | null>; open; reload; close }`、`createActiveReports(port): ActiveReports`(スプリントの画面と概要が使う)
  - `texts.ts`:
    - `unitLabel(u: ReportUnit): "pt" | "件"`
    - `amount(v: number, u: ReportUnit): string` — `${pt(v)} ${unitLabel(u)}`(件数も `pt` で数を書く)
    - `fallbackNote = "ポイントのあるアイテムがないため、件数で数えています"`
    - `burnStatus(s: ReportSprint, today: string): string` — 完了は「完了」、実行中は `pill`
    - `burnTotals(b: Burndown, u): { total: string; remaining: string }` — 「総量 {値} {単位}」「残り {値} {単位}」(表示する最後の日の値)
    - `committedNote(c: number, u): string` — 「確定の計画 {X} {単位}と開始日の総量が違うのは、完了のあとにアイテムを削除・移動したり、ステータスの分類を変えたりして、いまの状態から数え直しているためです」
    - `notStarted(start: string, thisYear: number): string` — 「開始日({日付})から表示します」
    - `averageText(avg: number | null, u): string` — 「完了した直近 3 スプリントの平均 {値} {単位}」(値は `toFixed(1)`)、nil なら「完了した直近 3 スプリントの平均 —」
    - `durationLines(d: Durations, kind: "cycle" | "lead"): { label: string; value: string; p85: string | null }` — ラベルは「サイクルタイム 中央値(着手→完了・{N} 件)」、`skipped > 0` なら「サイクルタイム 中央値(着手→完了・{N} 件、着手を経ていない {M} 件を除く)」、「リードタイム 中央値(作成→完了・{N} 件)」。値は「{x.x} 日」か「—」、85% は「85% は {x.x} 日以内」か null
    - `addedText(v: number, u): string` — 「途中で追加 +{N} {単位}」
    - `pointsCard(a: ReportActive | null): { heading: string; value: string }` — 「{名前} の完了ポイント」「{完了} / {全体} pt」、実行中がなければ「実行中のスプリントなし」「—」
  - `flowPeriod.ts`: `defaultPeriod(today: string): { from: string; to: string }`(20 日前〜今日)、`checkPeriod(from, to, today): string | null`(Go の `CheckPeriod` と同じ文と順)、`periodLabel(p: { from; to } | null, thisYear): string` — 「直近 21 日・件数」か「{開始} 〜 {終了}・件数」
  - `estimates.ts`: `hoursText(h: number | null): string`(「{時間} 時間」か「—」)、`diffText(d: number | null): { text: string; over: boolean }`(「+1 時間」・「-0.25 時間」・「0 時間」・「—」)、`perPointText(v: number | null): string`(`toFixed(1)` か「—」)、`pointsText(p: number | null): string`(「{pt} pt」か「—」)、`emptyEstimates`・`sprintScopeNote` の文(05 の 389・393 行)

- [ ] **Step 1: 失敗するテストを書く**

- `reports.test.ts`: `open` で読み、別のワークスペースを開くと前の state を消す。`select({unit: "count"})` で `port.select` に `{unit: "count"}` だけを渡してから読み直す。`setPeriod` で期間を付けて読み直し、`close` して `open` し直すと期間なしで読む。`reload` の失敗では前の state のまま。
- `texts.test.ts`: 上の文を仕様の例で(`averageText(3.3, "points")` →「完了した直近 3 スプリントの平均 3.3 pt」、`averageText(4, "count")` →「… 4.0 件」、`durationLines({n: 5, median: 3, p85: 6.4, skipped: 2}, "cycle")` のラベルに「着手を経ていない 2 件を除く」、値「3.0 日」、「85% は 6.4 日以内」。0 件は「—」で 85% は null)。`pointsCard` の 2 つの場合。
- `flowPeriod.test.ts`: `defaultPeriod("2026-11-14")` → `2026-10-25`〜`2026-11-14`。`checkPeriod` の 4 つの文。`periodLabel(null)` →「直近 21 日・件数」。
- `estimates.test.ts`: `diffText(100)` →「+1 時間」で `over` 真、`diffText(-25)` →「-0.25 時間」、`diffText(0)` →「0 時間」で `over` 偽、`diffText(null)` →「—」。
- `route.test.ts`: `/s/work/w/HH/reports` と `workspaceTabOf("reports")`。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- reports flowPeriod estimates texts route`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(web): レポートの画面の状態と文を持つ(F3 Task 12)"
```

### Task 13: レポートのタブ(単位・バーンダウン・ベロシティ・表で見る)

**Files:**
- Create: `web/app/src/screens/reports/ReportsTab.svelte`、`ReportHead.svelte`(冒頭の文・単位・注記)、`BurndownSection.svelte`、`LineChart.svelte`(バーンダウンの折れ線。小さいバーンダウンも使う)、`VelocitySection.svelte`、`BarChart.svelte`、`DataTable.svelte`(「表で見る」の折りたたみと表)
- Modify: `web/app/src/screens/workspace/WorkspaceScreen.svelte`(タブ「レポート」、F3-17)、`web/app/src/wire.ts`(`reports`・`activeReports` を作り、`reflect` で読み直す、F3-19)
- Test: `web/e2e/reports.spec.ts`

**Interfaces:**
- Consumes: Task 11・12
- Produces: `web/e2e/reports.spec.ts` の準備の関数 `startedSprint(page, info)` — ワークスペース・プロジェクト・Epic と Story A(3 pt)・Story B(2 pt)を作り、Sprint 1 に入れて開始し、A を InProgress → Done にする(`sprints.spec.ts` の helpers `createWorkspace`・`projectWithEpic`・`storyUnderEpic` を使う)。Task 14・15 の e2e も使う

画面の中身(上から):
- 「git 履歴から集計しています。」
- 単位のセレクト(「ポイント」「件数」)。`fallback` のとき、横に `fallbackNote`。
- バーンダウン: 見出し「バーンダウン」、「スプリントを選択」のセレクト(`sprints`。未選択なら「スプリントを選んでください」を本体に出す)、`burnStatus`、`burnTotals` の 2 つ、`LineChart`(「理想」は点線、「残り」は実線で最後の点に値を添える。横軸は日付、縦軸は `niceMax` と `ticks`、凡例「理想」「残り」)、`committed` があれば `committedNote`。`shown === 0` なら図・表・総量の代わりに `notStarted`。未選択のときと `shown === 0` のときは「表で見る」も出さない(05 の 405 行)。`DataTable`(列は「日付」「総量 ({単位})」「残り ({単位})」「理想 ({単位})」、残りが null なら「—」)。
- ベロシティ: 見出し「ベロシティ」と `averageText`、`BarChart`(棒に値、凡例「計画」「完了」)、`DataTable`(「スプリント」「計画 ({単位})」「完了 ({単位})」)。棒がなければ「開始したスプリントがありません」で、表も出さない。
- 図の上の値・凡例・表の文字は、色だけに頼らない(凡例は文字で書く)。`DataTable` の開閉は保存しない(初めは閉じる)。

- [ ] **Step 1: 失敗する e2e を書く**

- `reports burndown and velocity`: `startedSprint` のあと、レポートのタブに「git 履歴から集計しています。」「総量 5 pt」「残り 2 pt」、ベロシティの「表で見る」を開くと Sprint 1 の行(計画 5・完了 3)、「完了した直近 3 スプリントの平均 —」。単位を「件数」にすると「総量 2 件」、再読み込みしても単位のセレクトが「件数」。
- `points fallback`: ポイントのない Story だけのワークスペースで「ポイントのあるアイテムがないため、件数で数えています」。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test reports`
Expected: FAIL(タブ「レポート」がない)

- [ ] **Step 3: 書く**

選択の変更は `reports.select(...)`(変えた項目だけ)。失敗したらエラーのトースト「保存できませんでした」。閲覧のみの Store でもセレクトを無効にしない(F3-7)。読み直しは `ReportsTab` の `$effect` で `reports.open(sid, wid)`(離れたら `close`)、`dayOf($syncNow)` が変わったら `reports.reload()`。

- [ ] **Step 4: 通ることと型・大きさを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test reports && make web-check`
Expected: PASS(Svelte 200 行・TS 300 行の上限を含む)

- [ ] **Step 5: コミット**

```bash
git add web/app web/e2e
git commit -m "feat(web): レポートのタブに単位・バーンダウン・ベロシティと表を出す(F3 Task 13)"
```

### Task 14: 累積フロー図・サイクルタイム・見積もりと実績

**Files:**
- Create: `web/app/src/screens/reports/FlowSection.svelte`、`AreaChart.svelte`、`CycleSection.svelte`、`EstimatesSection.svelte`
- Modify: `web/app/src/screens/reports/ReportsTab.svelte`
- Test: `web/e2e/reports.spec.ts`(追記)

**Interfaces:**
- Consumes: Task 11・12、Task 13 の `startedSprint`・`DataTable`

画面の中身:
- 累積フロー図: 見出し「累積フロー図」と `periodLabel`。右に「開始」「終了」の日付入力と、「分類」「ステータス」の切り替え(押している方を `aria-pressed`)。初めの期間でないときは「直近 21 日に戻す」。入力が変わったら `checkPeriod` で確かめ、合わなければ日付入力の下に理由を出して図は前のまま(入力は残す)。合えば `reports.setPeriod`。`AreaChart`(層は下から積み、分類の層の色は分類の token、ステータスの層の色はステータスの色。凡例は層の名前)、`DataTable`(「日付」と層ごとの列)。
- サイクルタイムとリードタイム: `durationLines` の 2 組(ラベル、値、85% の行)。
- 見積もりと実績: 見出し「見積もりと実績」と「対象」のセレクト(「すべて」と `sprints`)。スプリントを選んでいるとき表の上に `sprintScopeNote`。表の列は「アイテム」(ID とタイトル。押すと詳細パネル)、「ポイント」「見積もり時間」「作業時間」「差」「時間/pt」、最後に「合計」の行(時間/pt は空)。差が正なら警告の色。行がなければ `emptyEstimates`。

- [ ] **Step 1: 失敗する e2e を書く**

- `reports flow, cycle and estimates`: `startedSprint` のあと、A の詳細パネルで見積もり 2 時間と作業時間 1.5 時間を記録する。レポートのタブで、累積フロー図の見出しの横に「直近 21 日・件数」。終了日を明日にすると「終了日は今日以前にしてください」。「ステータス」に切り替えると凡例に「Done」。サイクルタイムのラベルに「着手→完了・1 件」。見積もりと実績の表に A の行(差「-0.5 時間」)と「合計」の行。対象を Sprint 1 にすると「作業時間は、アイテムに付いた記録のすべてです(別のスプリントにいたあいだの記録も含みます)。」。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test reports`
Expected: FAIL(累積フロー図がない)

- [ ] **Step 3: 書く**

- [ ] **Step 4: 通ることと型・大きさを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test reports && make web-check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/app web/e2e
git commit -m "feat(web): 累積フロー図・サイクルタイム・見積もりと実績を出す(F3 Task 14)"
```

### Task 15: スプリントのカードと概要の完了ポイント

**Files:**
- Create: `web/app/src/screens/planning/SprintBurndown.svelte`(「途中で追加」・小さいバーンダウン・「表で見る」)、`web/app/src/screens/workspace/OverviewTab.svelte`(統計のカードの並び、F3-18)、`web/app/src/screens/workspace/SprintPointsCard.svelte`
- Modify: `web/packages/core/src/features/planning/sprints.ts`(`sprintMenu` に `burndown`、F3-16)と `sprints.test.ts`、`web/app/src/screens/planning/SprintCard.svelte`(実行中のカードに `SprintBurndown`、メニューの「バーンダウンを見る」。200 行を超えるなら、メニューの部分を子に分ける)、`SprintsTab.svelte`(`activeReports` を開く)、`WorkspaceScreen.svelte`(概要のタブに `OverviewTab`)
- Test: `web/e2e/reports.spec.ts`(追記)

**Interfaces:**
- Consumes: Task 12 の `ActiveReports`・`addedText`・`notStarted`・`pointsCard`、Task 13 の `LineChart`・`DataTable`・`startedSprint`
- Produces: `sprintMenu(s: Sprint, readOnly: boolean)` の戻りの `id` に `"burndown"` を足す。並びは「編集…」「バーンダウンを見る」「削除…」。「バーンダウンを見る」は閲覧のみの Store でも押せ、計画中は無効で `hint`「計画中のスプリントにはバーンダウンがありません」

- [ ] **Step 1: 失敗するテストを書く**

- `sprints.test.ts`: 計画中のスプリントの「バーンダウンを見る」は無効で上の文。閲覧のみの Store の実行中のスプリントでは「バーンダウンを見る」だけ押せる。
- e2e `sprint card and overview`: `startedSprint` のあと、概要のタブに「Sprint 1 の完了ポイント」と「3 / 5 pt」。スプリントのタブの実行中のカードの「表で見る」を開くと今日の行の残りが「2」。(「途中で追加」は開始日の翌日から出るので、e2e では確かめず Task 2 の `TestAdded` で確かめる。)メニューの「バーンダウンを見る」でレポートのタブへ移り、「スプリントを選択」が Sprint 1。計画中の Sprint 2 のメニューの「バーンダウンを見る」は無効。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- sprints && make e2e-bin && pnpm -C web exec playwright test reports`
Expected: FAIL

- [ ] **Step 3: 書く**

- 実行中のカード: `active.added > 0` かつ `shown > 0` なら `addedText`。小さいバーンダウンは `LineChart`(理想と残り)。`shown === 0` なら `notStarted`。「表で見る」はレポートのバーンダウンと同じ列。単位は `active.unit`。
- 「バーンダウンを見る」: `reports.select({ sprint: id })` のあとレポートのタブへ移る(`navigation.go`)。保存に失敗したらエラーのトースト「保存できませんでした」で移らない。
- 概要のタブ: 統計の並びに `SprintPointsCard`(`pointsCard`)。

- [ ] **Step 4: 通ることと型・大きさを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test && make e2e-bin && pnpm -C web exec playwright test reports && make web-check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 実行中のスプリントのカードに小さいバーンダウンを、概要に完了ポイントを出す(F3 Task 15)"
```

### Task 16: 記録と全体の確かめ

**Files:**
- Modify: `docs/superpowers/plans/2026-11-14-hedgehug-track-f1.md`(F1-19 の行)

- [ ] **Step 1: 記録を直す**

F1 の計画の F1-19 の行に「(小さいバーンダウン・途中で追加・バーンダウンを見る・表で見るは F3 で済んだ)」を足す。ユーザーの記録(メモリの `project-rebuild-progress.md`)の状態の欄に、F3 の進み具合と F3-18(概要の残りの 3 枚は E)を書く。

- [ ] **Step 2: 全体の確かめ**

Run: `make check`
Expected: PASS

- [ ] **Step 3: コミット**

```bash
git add docs/superpowers/plans
git commit -m "docs: F1 の計画に F3 で済んだものを書く(F3 Task 16)"
```

---

## G に求めること

トラック G(Store の運用と保守)の計画を書くときに、次のことを入れる。

- Store をこのマシンから削除したとき(08「このマシンから削除」、保存形式 284 行)、`state.db` の `report_selections` の、その Store ID の行も消す(F3 Task 7)。

## E に求めること

トラック E(検索とレイアウト)の計画を書くときに、次のことを入れる。

- ワークスペースの概要のタブ(`web/app/src/screens/workspace/OverviewTab.svelte`)の統計のカードの並びに、F3 が「{スプリント名} の完了ポイント」のカードを置いた(F3-18)。04「統計」の残りの 3 枚(プロジェクト・未完了のアイテム・期限切れ)を、04 の並び(プロジェクト・未完了のアイテム・期限切れ・完了ポイント)で同じ並びに足し、プロジェクトの一覧と最近の変更を足す。
