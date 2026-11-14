# hedgehug トラック F1(スプリントとリリース)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 05 のスプリントとリリースを作る。スプリントの作成・編集・開始・完了(確定値の記録と未完了の移動)・削除・復元、リリースの作成・編集・状態の変更・削除・復元、アイテムの「スプリント」「リリース」の欄(詳細パネル・作成ダイアログ・複製)、ワークスペースのタブ「スプリント」「リリース」の画面と、見出しの「実行中: …」を作る。

**Architecture:** スプリントとリリースはワークスペースの `log.jsonl` に、自動化のルールと同じく項目ごとの `set` の行(`sprints/<ID>/name` など)で持つ。読む規則・検査・既定値・並び・確定値の計算は新しい `domain/planning` に置き、操作は新しい `app/planning`、API は新しい `adapter/httpapi/planning` に置く。アイテムは `item.Item` に `Sprint`・`Release` を足し、項目 `sprint`・`release` の `set` の行で変える。画面は `web/packages/core/src/features/planning` と `web/app/src/screens/planning`。

**Tech Stack:** Go 1.27.3、huma v2、go.yaml.in/yaml/v4。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/api`、orval の生成)、e2e は Playwright。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/05-planning-reports.md`「数え方の共通ルール」(9〜17 行)、「完了したスプリントへは入れられない」(54〜56 行)、「並び順」の 49 行(選択の変更と完了は移動先の枠の末尾)、「スプリント」(67〜175 行)、「スプリントの画面」(177〜221 行。ボード・小さいバーンダウン・「途中で追加」・「バーンダウンを見る」を除く)、「リリース」(223〜270 行)、「スプリントとリリースの記録」(272〜288 行)
- 02「プロパティの入力と保存」49〜52 行(詳細パネルのスプリント・リリース)、26 行の表、「アイテムの作成」125・142・154・155 行、「複製」196・197 行、「履歴」488〜489 行
- 03 の 65 行(名前の比べ方)、400・452 行(テンプレートとスプリント)
- 04 の 713 行(アクティビティの文「{ID} の スプリント を {名前} に変更」)、919 行(ワークスペースの見出し)
- 08「削除済み」336・356〜364 行(スプリント・リリースの復元の単位と、名前が重なるときの断り)
- 01 の 124 行(閲覧のみの Store)、132 行(日付が変わったときの「残り N 日」)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「アイテムの例」(`sprint`・`release`)、「変更記録」(`close_sprint`)、117 行(名前を書き添えた `*.yaml` の作り直し)、「削除・復元・完全に削除」、「アクティビティと履歴」の「レポート」
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`「F に求めること」、`2026-11-14-hedgehug-track-c3a.md` の C3a-21

## 始める前に

- トラック F は 3 本に分ける(ユーザーの決定、2026-11-14): F1 スプリントとリリース(この計画)→ F2 バックログ・F3 レポート(F1 のあと並行可)。
- 実行中のスプリントのカードの「ボード」(05「実行中のスプリントのボード」)は、トラック E の計画に入れる(ユーザーの決定、2026-11-14)。F1 のカードのアイテムの欄はリストだけにする(F1-19)。
- A5a(移動の `transfer.Remap`)と A5c(コマンドパレット)は、この計画を書いた時点で develop にない。Task 19 で、develop にあるときだけ合わせる(F1-23)。
- 実装で計画から変えたこと:
  - Task 19 の A5a の分は、F1 の実装のあとに A5a が develop に入ったので、develop を取り込んでから F1 で行った(別のワークスペース・別の Store へ移すと、`transfer.Remap` がスプリント・リリースのあるアイテムに `set sprint null`・`set release null` を同じ操作で書く)。A5c の分(パレットの「作成」の群の「スプリントを作成」「リリースを作成」)は、develop に A5c がまだないので A5c が合わせる。中身はトラック A の計画の A5 の行に書いた(F1-23)。
  - Task 19b を足した。B5b(ワークスペースの削除)と C3a-screen(他のタブで削除されたものへの保存)が先に develop に入ったので、それに合わせた: スプリント・リリースの読み書きは `port.LiveWorkspace` を通し、削除済みのワークスペースは `ErrNotFound`。削除済みのワークスペースのスプリント・リリースの復元は、名前の重なりより先に「ワークスペースも削除されています。先にワークスペースを復元してください」で断る(08 の 356 行。F1-24 の B5b の分はここで済んだ)。同期の修復で、ワークスペースのログのスプリント・リリースの行は、ワークスペースの削除との重なりでは変更に数えない。`SetSprint`・`SetRelease` は、削除済みのアイテムなら削除を取り消して保存する。
  - 計画の概要の API(`PlanningOverview`)に `retentionDays`(その Store の削除済みの保持日数)を足した。削除の確認の本文に使う。
  - `apierr` で `port.ErrInvalidOperation` を 400 にした。
  - 日付の欄(開始日・終了日・予定日)は、API で `YYYY-MM-DD` か空かを確かめ、ほかの形は 422 で断る。
  - 別のタブで削除したスプリント・リリースへの保存(06「他の書き手との重なり」。編集・開始・完了・状態の変更・削除)は C3c に入れた(ユーザーの決定、2026-11-14)。トラック C の計画の C3c の行に書いた。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/port`・`app/apptest`・`app/workspaces`・`infra/gitstore` は 20 個で満杯、`app/items` は 19 個なので、ファイルを足さない(`app/items` には足してよいのは 1 個だが、この計画では足さない)。新しいものは新しいパッケージ `domain/planning`・`app/planning`・`adapter/httpapi/planning` に置く。
- app の機能どうしは import しない(設計書 2 節)。`app/planning` はアイテムを `port` から読み、`app/items` を呼ばない。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書き、言い換えない。日付の表示は 01 の書き方(core の `formatDay`)。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする(CI が差分を見る)。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| F1-1 | 新しい口(`Reader.Planning()`・`Tx.WritePlanning()`)は足さない。スプリントとリリースはワークスペースのログにあるので、自動化と同じく `Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})` で読み、`planning.FromLog` で作り、`WriteWorkspaces().Append` で書く。土台の計画の決定 2(「トラック F が口だけの小さな PR で足す」)はこれで置き換える | `app/automations` の `rulesOf`。`port`・`apptest` は満杯。PR はトラックごとに 1 本(CLAUDE.md) |
| F1-2 | 項目名: スプリントは `sprints/<ID>/name`・`/state`(`planned`・`active`・`closed`)・`/start`・`/end`・`/goal`。リリースは `releases/<ID>/name`・`/state`(`unreleased`・`released`・`archived`)・`/date`・`/description`。作成は自動化と同じく、すべての項目を `from: null` の `set` で書く(作成の行は持たない)。削除は `change.NewDelete("sprints/<ID>")`(`releases/<ID>`)。読むときは `change.Restored` で取り消された削除を除く | トラック C の計画「F に求めること」、C3a-21。こうすると `repair.Overlaps`・`change.Undo`・`workspace.NameOfDeleted` がそのまま効く(Task 1 のテストで確かめる) |
| F1-3 | 完了の行は `t: close_sprint`(`planning.CloseSprint`、`numbering.Type` と同じく domain で宣言)、`f: sprints/<ID>/close`、`to: {"committed":{"points":X,"count":N},"completed":{"points":Y,"count":M}}`。読むときは `change.Current(log, CloseSprint, f, 常に真)` の行を取る(同時に 2 回完了したときも 1 つに決まる)。状態の `closed` は同じ操作の `set sprints/<ID>/state` で書く | 保存形式「変更記録」の `close_sprint`。`f` を `sprints/<ID>/` の下にするのは、削除と完了が重なったときに、C3a-9 が削除を取り消すため |
| F1-4 | アイテムの項目は `sprint`・`release`(値は内部 ID、なしは `null`)。`item.Item` に `Sprint`・`Release change.ID`(なしは空)を足す。作成の行には値のあるときだけ書く | 保存形式「アイテムの例」 |
| F1-5 | 使えない参照の扱い(どの画面・集計も同じ): アイテムのスプリントが、ないか削除済みか別のワークスペースのものなら「バックログ」、リリースが同じなら「なし」として表示し、集計に含めない。参照の値は書き換えない(復元で戻る)。`planning.Plan.UsableSprint(id)`・`UsableRelease(id)` が答える | 05 の 175 行。08「存在しないスプリント・リリース」 |
| F1-6 | 名前の比べ方: 前後の空白を除き、完全に一致するものを重複とする(大文字と小文字・全角と半角を区別する)。保存も前後の空白を除いた形。重複の数に削除済みのものは入れない(復元のときは、削除済みでない同じ名前があれば断る) | 03 の 65 行、05 の 155・270 行 |
| F1-7 | 作成の既定の名前の数字: 名前の末尾の数字の並びを数として 1 を足し、元の桁数より短ければ先頭を 0 で埋める(`Sprint 07` → `Sprint 08`、`Sprint 9` → `Sprint 10`、`S-099` → `S-100`)。基準のスプリント(終了日が最も遅いもの)が複数あれば、開始日が遅いもの、それも同じなら名前の昇順で後のもの。数えるスプリントの数は削除済みを除く | 05 の 97〜99 行。桁を保つのは、`Sprint 07` から `Sprint 8` を作ると並びが崩れるため |
| F1-8 | 検査の文言と順(作成・編集): 05 の 103〜109 行の表の順に最初の 1 つ。開始のダイアログは 05 の 115・119 行(実行中があるときの「実行中のスプリントがあります。先に完了してください」を最初に)。サーバーが `refusal.New` で断り、画面はダイアログの中にその文を出す | 05 の表 |
| F1-9 | 「今日」はサーバーの `Clock.Now()` をこのマシンの時刻帯で日付にしたもの(`planning.Today(now) string`、`YYYY-MM-DD`)。日の終わり時点は、その日の翌日の 0 時(このマシンの時刻帯)より前に書いた行 | 01 の 132 行、05 の 17 行 |
| F1-10 | 確定値の計画(committed): 開始日(今日より後なら今日)の終わり時点で、そのスプリントに入っていた作業項目のうち、その時点の分類が中止でないもの。量はいまのポイント(未設定は 0)、件数は 1。その時点の値は、アイテムのログの `at` がその時刻より前の行だけで `change.Resolve` した値(`planning.ValueAt`)。分類は、いまのワークスペースの設定で決め、設定にないステータスは未着手。削除済みのアイテムと、いま別のワークスペースにあるアイテムは数えない。実績(completed): 完了の操作の時点でスプリントに入っている作業項目のうち、分類が完了のもの | 05 の 138〜145・17・320 行。保存形式「レポート」(出入りの日は `set sprint` の行、量はいまの値)。F3 のバーンダウンが同じ `ValueAt` を使う |
| F1-11 | 枠の末尾(05 の 49 行): スプリントを変えたとき(詳細パネル、のちの F2 のバックログの選択・テーブルのセル)と、完了で移すとき、そのアイテムの `rank` を、移動先の枠のアイテム(スプリントの枠なら、そのスプリントの削除済みでない作業項目。バックログの枠なら、使えるスプリントに入っていない、分類が未着手か進行中の作業項目)の最大の `rank` の `rank.After` にする。完了で複数を移すときは、いまの並び順のまま `rank.After` を続ける。枠が空なら `rank` を変えない。`rank` の行は同じ操作で書く(履歴に出ない) | 05 の 44〜51 行(並びはワークスペースで 1 本。同じ順位は作成の早い方、内部 ID の順 = `item.SortSiblings`) |
| F1-12 | 完了の 1 回の保存: 確定値の行、未完了(分類が未着手か進行中、削除済みを除く)のアイテムの `set sprint`(移動先、バックログは `null`)と `set rank`、状態の `set closed`。完了と中止のアイテムと削除済みのアイテムには触らない。移動先に選べるのは計画中のスプリント(削除済みを除く)とバックログ。それ以外は `port.ErrNotFound` | 05 の 126〜136 行 |
| F1-13 | スプリントの削除の 1 回の保存: ワークスペースに `delete sprints/<ID>`、入っている削除済みでないアイテム(完了・中止を含む)に `set sprint`(`null`)。`rank` は変えない。実行中は断る(「実行中は削除できません」)。リリースも同じ(`set release`)で、どの状態でも消せる | 05 の 147〜153・263〜269 行、保存形式「削除・復元・完全に削除」 |
| F1-14 | 復元(スプリント・リリース): `change.Undo(log, 削除の op)` を、その op の行があるログ(ワークスペースとアイテム)ごとに書く。そのあとで別のスプリントに入れたアイテムは、`Undo` が先端でない行を戻さないので戻らない。削除済みでない同じ名前があれば、「同じ名前のスプリントがあります。先にそのスプリントの名前を変えてください」(リリースは「同じ名前のリリースがあります。先にそのリリースの名前を変えてください」)で断る。アクティビティは「スプリント「{名前}」を復元」「リリース「{名前}」を復元」。F1 では app と API だけを作り、画面(設定の「削除済み」)は G が作る。テストは app と API で行う | 05 の 155・270 行、08 の 336・363・364・368 行。C3a の決定(`restore` の行は C3a、画面は G) |
| F1-15 | アイテムのスプリントを変えられるのは Story・Task・Bug、リリースは Step 以外。それ以外の種類は `port.ErrInvalidOperation`(400)。完了したスプリントは、いま入っているのと同じなら何もせず、ほかは「完了したスプリントには入れられません」(422)。削除済み・ない・別のワークスペースのスプリントとリリースは `port.ErrNotFound`。アーカイブのリリースは、いま入っているのと同じなら何もせず、ほかは `port.ErrInvalidOperation`(選択肢に出さないので、画面の操作では起きない)。同じ値は何も保存しない | 02 の 51・52 行、05 の 39・56・240 行 |
| F1-16 | アクティビティの文: アイテムは「{ID} の スプリント を {名前 または バックログ} に変更」「{ID} の リリース を {名前 または なし} に変更」。スプリント・リリースの操作は 05 の 274〜286 行の表のとおり(リリースの状態は「未リリース」「リリース済み」「アーカイブ」)。名前は改行と連続する空白を 1 つの空白にする(自動化の `oneLine` と同じ) | 05 の 63・274〜286 行、04 の 713 行 |
| F1-17 | 履歴: アイテムの `set sprint` は「スプリントを変更」、`set release` は「リリースを変更」。`rank` の行は出さない(いまのまま) | 02 の 488・489 行 |
| F1-18 | トースト(ボタンなし): スプリントの作成「{名前} を作成しました」(リリースの作成と同じ文。05 にスプリントの作成の文がないため)、開始「{名前} を開始しました」、完了「{名前} を完了しました」、削除「スプリントを削除しました」。リリースの作成「{名前} を作成しました」、状態「{名前} を{状態}にしました」、削除「リリースを削除しました」。編集はトーストを出さない。失敗(断り以外)はエラーのトースト「保存できませんでした」(いまの設定の画面と同じ) | 05 の 120・136・153・239・252・269・288 行 |
| F1-19 | スプリントの画面のカードに、F1 では「バーンダウンを見る」・小さいバーンダウン・「途中で追加」・「表で見る」を出さない(F3 がレポートのタブと一緒に足す)。実行中のカードの「リスト」と「ボード」の切り替えは E が足す。F1 のメニューは「編集…」「削除…」 | ユーザーの決定(2026-11-14、F の分け方とボード) |
| F1-20 | ワークスペースのタブは、01 の並び(概要・ツリー・カンバン・テーブル・バックログ・スプリント・リリース・ガント・レポート・設定)のうち、作ったものだけを出す。F1 で「スプリント」「リリース」を足し、「概要・スプリント・リリース・設定」にする。アドレスは `/s/{store}/w/{prefix}/sprints`・`/releases` | B2 の計画の決定 15(作ったタブだけ出す)、01 の 166・201 行 |
| F1-21 | 作成ダイアログのスプリントの欄は、Story・Task・Bug のときだけ出し、選択肢は「バックログ」と完了していないスプリント(実行中に「(実行中)」を添える。並びは実行中、計画中の開始日の早い順・期間未定は最後・同じは名前の昇順)。初めの値は「バックログ」。Epic・Step に切り替えたら「バックログ」に戻し、ワークスペースを変えたら「バックログ」に戻す。テンプレートで作るときは、もとになるアイテムにだけ入れる。作成の `rank` は変えない(作成のときの並びの位置は F2 が決める) | 02 の 125・142・154・155 行、03 の 452 行、05 の 31・46 行 |
| F1-22 | 複製: スプリントは使える(F1-5)計画中・実行中なら写し、完了・使えないなら写さない(バックログ)。リリースは使えるアーカイブでないものなら写し、ほかは写さない(なし)。種類の変更と同じワークスペースの中の移動では変えない(いまのまま) | 02 の 196・197・214・305 行。削除済みを写すと、復元したときに複製まで入るため |
| F1-23 | A5a・A5c とは、develop にあとから入る方が合わせる。Task 19 は、F1 の実装を始めた時点で develop に A5a(`transfer.Remap`)・A5c(パレット)が入っていれば行い、なければ飛ばす(飛ばしたら A5a・A5c が合わせる。ユーザーの記録に書いてある) | A5a の計画から F への依頼(別のワークスペースへ移すときスプリントとリリースを外す。02 の 231 行)、トラック A の計画の A5 の行(パレットの「作成」のスプリント・リリースは F) |
| F1-24 | 他のトラックに渡すもの(Task 20 で各計画・記録に書く): G = 削除済みの一覧にスプリント・リリースの行(`planning.Plan` の `Deleted` のあるもの)を出し、「復元」は F1 の API を呼ぶ。アイテムの復元で、入っていたスプリントが完了済み(分類が未着手か進行中のとき)・使えないなら `set sprint null`、リリースが使えないなら `set release null` を同じ op で書く(08 の 343〜349 行)。完全に削除するスプリント・リリースを指すアイテムの参照を同じ保存で外す(08 の 385 行)。C5 = 同期の修復の(6)〜(9)のスプリント・リリース。B5b = 削除済みのワークスペースの下のスプリント・リリースの復元の断り(08 の 356 行) | 08、保存形式「同期の修復」 |

## Review Focus

1. **削除と同時の編集でスプリントが戻らない。** 別の人がスプリントの名前を変えた同期で、削除が取り消されてスプリントとアイテムの参照が戻る(F1-2)。Task 1 のテストに入れる。
2. **途中で足したアイテムが計画に入る。** 開始日の翌日に足したアイテムは、確定値の計画に入らず、完了していれば実績に入る。開始日より前に足して開始日に外したものは計画に入らない(F1-10)。Task 4 のテストに入れる。
3. **完了で移したアイテムの並びが崩れる。** 未完了 3 件を移動先のスプリントの末尾へ、元の並び順のまま移す。移動先が空のスプリントでも並びが保たれる(F1-11)。Task 7 のテストに入れる。
4. **名前を変えても item.yaml の注釈が古い。** スプリントの名前を変えた保存で、入っているアイテムの `sprint:` の行末の注釈が新しい名前になる。同期で受け取った改名でも同じ(F1-4、保存形式 117 行)。Task 5 のテストに入れる。
5. **削除済み・完了済みのスプリントを詳細パネルで選べる。** 選択肢に出さず、いま入っている完了済みのスプリントだけを残す。削除済みのスプリントに入っているアイテムは「バックログ」と出る(F1-5・F1-15)。Task 17 のテストに入れる。

---

## ファイルの地図

```
internal/domain/planning/plan.go          Sprint・Release・Plan・FromLog・UsableSprint・UsableRelease
internal/domain/planning/drafts.go        SprintDrafts・ReleaseDrafts・削除・CloseSprint の行
internal/domain/planning/check.go         CheckSprint・CheckStart・CheckRelease・名前
internal/domain/planning/defaults.go      SprintDefaults・Today・RemainingDays
internal/domain/planning/order.go         ScreenOrder・PlannedOrder・ReleaseOrder・WorkOrder
internal/domain/planning/timeline.go      ValueAt・EndOfDay・Work・Confirm
internal/domain/planning/frame.go         FrameEnd・Category・IsWork
internal/domain/item/item.go              (変更)Sprint・Release
internal/domain/history/item.go           (変更)「スプリントを変更」「リリースを変更」
internal/infra/gitstore/yaml_settings.go・yaml_notes.go・yaml_regen.go・yaml.go・sync_tree_more.go・commit.go (変更)
internal/app/planning/service.go          Service・読み取りの共通
internal/app/planning/sprints.go          作成・編集・開始
internal/app/planning/close.go            完了
internal/app/planning/remove.go           スプリント・リリースの削除と復元
internal/app/planning/releases.go         リリースの作成・編集・状態
internal/app/planning/assign.go           アイテムの SetSprint・SetRelease
internal/app/planning/view.go             Overview・SprintCards・ReleaseCards
internal/app/items/create.go・duplicate.go (変更)作成のスプリント・複製
internal/adapter/httpapi/planning/*.go    API
internal/adapter/httpapi/items/*.go       (変更)sprint・release の DTO、作成の sprint
cmd/hedgehug/wire.go・wire_e2e.go         (変更)組み立て
web/packages/core/src/model/planning.ts・port/planning.ts・features/planning/*.ts
web/packages/api/src/planning.ts
web/app/src/screens/planning/*.svelte
web/app/src/screens/workspace/WorkspaceScreen.svelte (変更)タブと見出し
web/app/src/screens/item/DetailProperties.svelte・CreateItemFields.svelte (変更)
web/e2e/sprints.spec.ts・releases.spec.ts
```

---

### Task 1: スプリントとリリースをログから読む(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/plan.go`、`internal/domain/planning/drafts.go`
- Test: `internal/domain/planning/plan_test.go`

**Interfaces:**
- Produces:
  - `type SprintState string`(`Planned = "planned"`・`Active = "active"`・`Closed = "closed"`)、`type ReleaseState string`(`Unreleased = "unreleased"`・`Released = "released"`・`Archived = "archived"`)
  - `type Totals struct { Points float64 \`json:"points"\`; Count int \`json:"count"\` }`、`type Confirmed struct { Committed, Completed Totals }`(JSON は `committed`・`completed`)
  - `type Sprint struct { ID change.ID; Name string; State SprintState; Start, End, Goal string; Confirmed *Confirmed; Deleted *change.Deletion }`
  - `type Release struct { ID change.ID; Name string; State ReleaseState; Date, Description string; Deleted *change.Deletion }`
  - `type Plan struct { Sprints []Sprint; Releases []Release }`(削除済みを含み、内部 ID の順)
  - `func FromLog(log []change.Record) Plan` — 名前の空のものは除く。`Deleted` は、その `delete` の行(取り消されていないもの)の op・at
  - `func (p Plan) LiveSprints() []Sprint`・`LiveReleases() []Release`(削除済みを除く、内部 ID の順)
  - `func (p Plan) UsableSprint(id change.ID) (Sprint, bool)`・`UsableRelease(id change.ID) (Release, bool)`(削除済みでなく、ある。F1-5)
  - `const CloseSprint change.Type = "close_sprint"`
  - `func SprintDrafts(old, s Sprint) []change.Draft`・`ReleaseDrafts(old, r Release) []change.Draft` — `old.ID` が空なら作成で全項目を `from: null` で書く。ほかは変わった項目だけ(`automation.Drafts` と同じ形)
  - `func DeleteSprint(id change.ID) change.Draft`・`DeleteRelease(id change.ID) change.Draft`
  - `func CloseDraft(id change.ID, c Confirmed) change.Draft` — `{T: CloseSprint, F: "sprints/<ID>/close", To: c}`

- [ ] **Step 1: 失敗するテストを書く**

- `TestFromLogSprint`: `SprintDrafts(Sprint{}, {ID:"S1", Name:"Sprint 1", State:Planned, Start:"2026-11-02", End:"2026-11-15", Goal:"ログイン"})` の行を読み、同じ値が返る。続けて名前を「Sprint 1b」にした行で名前が変わる。
- `TestFromLogDeleteAndRestore`: `DeleteSprint("S1")` の行(op `o1`)で `Deleted.Op == "o1"`・`LiveSprints()` が空・`UsableSprint("S1")` が false。`change.NewRestore("sprints/S1", "o1")` の行を足すと `Deleted == nil` に戻る。リリースも同じ。
- `TestFromLogClose`: `CloseDraft("S1", Confirmed{Committed: Totals{8, 3}, Completed: Totals{5, 2}})` を読むと `Confirmed` が同じ。`prev` でつながらない close の行が 2 つあれば、`at` があとの行の値。
- `TestPlanFieldsWorkWithRepair`: `workspace.NameOfDeleted(log, "sprints/S1") == "Sprint 1"`。`repair.Overlaps` に、片側の `delete sprints/S1`(人)ともう片側の `set sprints/S1/name`(人)を渡すと、`{Op: 削除の op, F: "sprints/S1"}` を 1 つ返す。`change.Undo(log, 削除の op)` が `restore` の行を返す(F1-2)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: `plan.go`・`drafts.go` を書く**

読み方は `domain/automation/log.go` の `collect` と同じ(`change.Resolve(log, change.Options{})` の値を `sprints/<ID>/<項目>` で集め、削除は生のログから `change.Restored` で除く)。close の行は `Resolve` が読まないので、スプリントごとに `change.Current(log, CloseSprint, "sprints/<ID>/close", func(change.Record) bool { return true })` で読む。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): スプリントとリリースをワークスペースのログから読み書きする(F1 Task 1)"
```

### Task 2: 検査・既定値・並び(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/check.go`、`defaults.go`、`order.go`
- Test: `internal/domain/planning/check_test.go`、`defaults_test.go`、`order_test.go`

**Interfaces:**
- Consumes: Task 1 の `Sprint`・`Release`
- Produces:
  - `func NormalizeName(s string) string`(= `workspace.NormalizeName`)
  - `func CheckSprint(s Sprint, live []Sprint, today string) error` — 05 の 103〜109 行の表の順で最初の 1 つを `refusal.New`。`live` に `s.ID` 自身が入っていてもよい(自分を除いて比べる)。名前は比べる前に `NormalizeName`
  - `func CheckStart(start, end, today string) error` — 「開始日と終了日を入力してください」→「終了日は開始日以降にしてください」→「開始日は今日以前にしてください」の順
  - `func CheckRelease(r Release, live []Release) error` — 「名前を入力してください」→「同じ名前のリリースがあります」
  - `func SprintDefaults(live []Sprint, today string) (name, start, end string)` — F1-7
  - `func Today(now time.Time) string`(`now.Local().Format(time.DateOnly)`)、`func AddDays(ymd string, n int) string`
  - `func RemainingDays(end, today string) (n int, overdue bool)` — 終了日の当日は 1、過ぎたら overdue
  - `func ScreenOrder(live []Sprint) []Sprint` — 実行中、計画中(開始日の早い順・期間未定は最後・同じは名前の昇順)、完了(終了日の新しい順・同じは名前の昇順)
  - `func PlannedOrder(live []Sprint) []Sprint` — 計画中だけを `ScreenOrder` の計画中の順で
  - `func ReleaseOrder(live []Release) []Release` — 未リリース・リリース済み・アーカイブ、次に予定日の早い順(なしは同じ状態の最後)、同じは名前の昇順

- [ ] **Step 1: 失敗するテストを書く**

- `TestCheckSprint`: 表の行ごとに 1 件(名前が空白だけ →「名前を入力してください」、自分以外に同じ名前 →「同じ名前のスプリントがあります」、大文字違いの「sprint 1」は通る、実行中で終了日が空 →「実行中のスプリントには開始日と終了日が必要です」、終了日 < 開始日 →「終了日は開始日以降にしてください」、完了で開始日が明日 →「開始日は今日以前にしてください」、計画中で開始日が明日は通る、計画中で両方空は通る)。2 つの条件に当たるときは表の上の文。
- `TestCheckStart`: 空 →「開始日と終了日を入力してください」、逆転、明日の開始日、同じ日は通る、過去の開始日は通る。
- `TestSprintDefaults`: (a) スプリントなし、今日 2026-11-14 → `Sprint 1`・`2026-11-14`・`2026-11-27`。(b) `Sprint 7`(終了 2026-11-15)→ `Sprint 8`・`2026-11-16`・`2026-11-29`。(c) さらに `Sprint 8`(期間未定)がある → `Sprint 9`。(d) `Sprint 9` もある → `Sprint 10`。(e) 基準が `リリース前` で 3 件 → `Sprint 4`。(f) `Sprint 07` → `Sprint 08`、`S-099` → `S-100`。(g) 削除済みの `Sprint 8` は重なりにも数にも入れない。
- `TestRemainingDays`: 終了日が今日 → 1・false、明日 → 2、昨日 → overdue。
- `TestScreenOrder`・`TestReleaseOrder`: 05 の 25・181・256 行の並びを 1 つの入力で確かめる(同じ開始日は名前の昇順、期間未定は計画中の最後、予定日なしは同じ状態の最後)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: FAIL

- [ ] **Step 3: 3 つのファイルを書く**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): スプリントとリリースの検査・既定値・並びを足す(F1 Task 2)"
```

### Task 3: アイテムにスプリントとリリースを持たせる

**Files:**
- Modify: `internal/domain/item/item.go`(`Item` のコメントと項目、`Create`、`FromLog`)、`internal/domain/history/item.go`(`setText`)
- Test: `internal/domain/item/item_test.go`、`internal/domain/history/item_test.go`(いまのテストのファイルに足す)

**Interfaces:**
- Produces: `item.Item.Sprint`・`item.Item.Release`(`change.ID`、なしは空)。`item.SprintField = "sprint"`・`item.ReleaseField = "release"`。`Create` は値があるときだけ `sprint`・`release` を `to` に入れる。

- [ ] **Step 1: 失敗するテストを書く**

- `TestSprintAndRelease`: `Create(Item{…, Sprint: "S1"})` の行と、`change.NewSet("release", nil, "R1")` の行を読むと `Sprint == "S1"`・`Release == "R1"`。`NewSet("sprint", "S1", nil)` で空に戻る。
- `TestHistorySprintRelease`: `set sprint` の行が「スプリントを変更」、`set release` の行が「リリースを変更」の 1 行になる。同じ op の `set rank` は出ない。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/item/ ./internal/domain/history/`
Expected: FAIL

- [ ] **Step 3: 項目と文を足す**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/item internal/domain/history
git commit -m "feat(item): アイテムにスプリントとリリースを持たせ、履歴に「スプリントを変更」「リリースを変更」を出す(F1 Task 3)"
```

### Task 4: ある時点の値と確定値(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/timeline.go`、`internal/domain/planning/frame.go`
- Test: `internal/domain/planning/timeline_test.go`、`frame_test.go`

**Interfaces:**
- Consumes: Task 1・Task 3
- Produces:
  - `type Work struct { Item item.Item; Log []change.Record }`(F3 も使う)
  - `func IsWork(k item.Kind) bool`(Story・Task・Bug)
  - `func Category(st workspace.Settings, status change.ID) workspace.Category` — 一覧にないものは `CategoryTodo`
  - `func EndOfDay(ymd string, loc *time.Location) time.Time` — 翌日の 0 時
  - `func ValueAt(log []change.Record, f string, t time.Time) (json.RawMessage, bool)` — `at` が t より前の行だけで `change.Resolve`(作成の行の `to` の値を含む)した f の値。作成が t より前になければ false
  - `func Amount(it item.Item) float64`(ポイント、なしは 0)
  - `func Confirm(works []Work, ws workspace.ID, s Sprint, st workspace.Settings, now time.Time) Confirmed` — F1-10。`works` は削除済みを含んでよく、`Deleted` のあるもの、`Place.Workspace != ws`、作業項目でないものを除く
  - `func FrameEnd(works []Work, ws workspace.ID, sprint change.ID, plan Plan, st workspace.Settings) string` — F1-11 の移動先の枠の最大の `rank`(空の枠は空文字)。`sprint` が空ならバックログの枠

- [ ] **Step 1: 失敗するテストを書く**

時刻帯は `time.Local` を `Asia/Tokyo` にしてテストする(`t.Setenv("TZ", …)` ではなく、関数に loc を渡す形で `EndOfDay` を確かめ、`Confirm` は `now` の `Location` を使う)。スプリント S1 の開始日 2026-11-02、終了日 2026-11-15、完了 2026-11-15 15:00。

- `TestConfirmCommitted`: (a) 11/1 に作成と同時に S1 に入れた 3 pt → 計画に入る。(b) 11/3 に入れた 2 pt で完了済み → 計画に入らず、実績に入る。(c) 11/1 に入れて 11/2 23:00 に外し 11/5 に戻した 5 pt → 計画に入らない。(d) 11/2 の終わりに中止だった 1 pt → 計画に入らない。(e) 削除済みのアイテム → どちらにも入らない。(f) いま別のワークスペースにあるアイテム → 入らない。(g) ポイントなし → 0 pt・1 件。(h) 開始日が明日(同期で届いた場合)→ 今日の終わりで数える。期待値は `Committed{Points: 3+…, Count: …}` を数字で書く。
- `TestConfirmCompleted`: 完了の時点で S1 に入っていて分類が完了のものだけ。中止は入らない。設定にないステータスは未着手として未完了。
- `TestValueAtOrder`: 同じ時刻より前の 2 行が `prev` でつながっていれば、あとの行の値(時計がずれて `at` が前でも)。
- `TestFrameEnd`: S1 の枠は S1 の削除済みでない作業項目の最大 `rank`。バックログの枠は、使えるスプリントに入っていない(削除済みのスプリントに入っているものを含む)未着手・進行中の作業項目の最大。Epic と完了・中止は数えない。空の枠は空文字。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: FAIL

- [ ] **Step 3: 2 つのファイルを書く**

`ValueAt` は、`log` を `at < t` の行に絞ってから `change.Resolve(rows, change.Options{})` の `Values[f]`。絞った行に作成の行がなければ false。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning
git commit -m "feat(planning): ある時点の値と、スプリントの確定値・枠の末尾を求める(F1 Task 4)"
```

### Task 5: workspace.yaml・item.yaml に書き出す

**Files:**
- Modify: `internal/infra/gitstore/yaml_settings.go`(`settingsOf` の型、`workspaceToYAML` に `sprints`・`releases`)、`yaml_notes.go`(`itemToYAML` に `sprint`・`release` と注釈)、`yaml.go`(`itemYAML` の項目、`renderYAML`)、`yaml_regen.go`(`txSettings` が計画も覚え、`changedWorkspaces` が計画の名前の変化も見る)、`sync_tree_more.go`(`treeSettings`)、`commit.go`(呼び出しの型)
- Test: `internal/infra/gitstore/yaml_settings_test.go`、`yaml_notes_test.go`(いまのファイルに足す)

**Interfaces:**
- Consumes: Task 1・Task 3
- Produces: `type settingsOf func(workspace.ID) (wsInfo, bool)`、`type wsInfo struct { Settings workspace.Settings; Plan planning.Plan }`(gitstore の中だけ)。`item.yaml` の `systems` の次に `sprint:`・`release:`(行末に名前の注釈。使えない参照には付けない)。`workspace.yaml` の `rules` の次に `sprints:`・`releases:`(各行 `id`・`name`・`state`・`start`・`end`・`goal`・`committed`・`completed`・`deleted`、リリースは `id`・`name`・`state`・`date`・`description`・`deleted`。内部 ID の順、削除済みも出す)。

- [ ] **Step 1: 失敗するテストを書く**

- `TestWorkspaceYAMLSprints`: スプリント 1 つ(完了、確定値あり)とリリース 1 つ(削除済み)の `workspace.yaml` が、期待する文字列と一致する(キーの並びは上のとおり)。
- `TestItemYAMLSprintComment`: アイテムの `sprint: <ID>  # Sprint 4`・`release: null`。
- `TestRenameSprintRewritesItemYAML`: 1 回目の保存でスプリントとアイテム(そのスプリント入り)を書き、2 回目の保存でスプリントの名前だけを変えると、アイテムのログは変わらず、`item.yaml` の注釈が新しい名前になる。
- `TestSyncRenameRewritesItemYAML`: いまの同期のテストの形で、相手の改名を受け取ったあとの `item.yaml` の注釈が新しい名前になる(`treeSettings` の道)。
- `TestCloseSprintRowRoundTrip`: `planning.CloseDraft` の行を書いて読み直すと、`Changes().Of` に同じ行があり、`planning.FromLog` の `Confirmed` が同じ。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/gitstore/ -run 'Sprint|CloseSprint'`
Expected: FAIL

- [ ] **Step 3: 書き出しと作り直しを広げる**

`changedWorkspaces` は、設定に加えて、スプリントとリリースの `ID → 名前` の対応が変わったときも変わったとする(状態や日付だけの変更では `item.yaml` を作り直さない)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/gitstore/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): workspace.yaml にスプリントとリリース、item.yaml に名前の注釈を書く(F1 Task 5)"
```

### Task 6: スプリントの作成・編集・開始(`app/planning`)

**Files:**
- Create: `internal/app/planning/service.go`、`internal/app/planning/sprints.go`
- Test: `internal/app/planning/sprints_test.go`

**Interfaces:**
- Consumes: Task 1・2
- Produces:
  - `type Service struct { Stores port.Stores; IDs port.IDs; Identity port.Identity; Clock port.Clock }`
  - `type SprintInput struct { Name, Start, End, Goal string }`
  - `func (s Service) CreateSprint(ctx, store port.StoreID, ws workspace.ID, in SprintInput) (planning.Sprint, error)` — 計画中で作る
  - `func (s Service) UpdateSprint(ctx, store, ws, id change.ID, in SprintInput) error` — 状態は変えない
  - `func (s Service) StartSprint(ctx, store, ws, id change.ID, start, end string) error` — 計画中だけ(ほかは `port.ErrNotFound`)
  - 共通(service.go): `planOf(r port.Reader, ws) (planning.Plan, []change.Record, error)`(ワークスペースが削除済みでもないなら `ErrNotFound`)、`works(r port.Reader, ws) ([]planning.Work, error)`(ワークスペースの削除済みでないプロジェクトの全アイテムとログ)、`oneLine(s string) string`

- [ ] **Step 1: 失敗するテストを書く**

`apptest.NewStores` とサンプルのワークスペースで確かめる(`internal/app/automations/rules_test.go` の形)。

- `TestCreateSprint`: 作ったスプリントが計画中で、名前は前後の空白を除いた形。アクティビティの Summary が「スプリント「Sprint 1」を作成」。同じ名前は「同じ名前のスプリントがあります」で断り、何も保存しない。
- `TestUpdateSprint`: 名前と日付を変える。実行中のスプリントの終了日を空にすると「実行中のスプリントには開始日と終了日が必要です」。Summary は新しい名前の「スプリント「…」を編集」。
- `TestStartSprint`: 計画中を開始すると実行中になり、開始日と終了日が入る。Summary「スプリント「…」を開始」。ほかに実行中があれば「実行中のスプリントがあります。先に完了してください」。明日の開始日は「開始日は今日以前にしてください」。0 件でも開始できる。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/planning/`
Expected: FAIL

- [ ] **Step 3: 書く**

保存の形は `app/automations/service.go` の `edit` と同じ(Read で Summary を決め、Write の中で読み直して検査し、行を書く)。今日は `planning.Today(s.Clock.Now())`。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/planning
git commit -m "feat(planning): スプリントの作成・編集・開始を足す(F1 Task 6)"
```

### Task 7: スプリントの完了

**Files:**
- Create: `internal/app/planning/close.go`
- Test: `internal/app/planning/close_test.go`

**Interfaces:**
- Consumes: Task 4 の `Confirm`・`FrameEnd`、Task 6 の共通
- Produces: `func (s Service) CloseSprint(ctx, store port.StoreID, ws workspace.ID, id change.ID, dest *change.ID) error` — `dest` が nil ならバックログ。F1-12

- [ ] **Step 1: 失敗するテストを書く**

- `TestCloseSprintMovesUndone`: S1(実行中)に、並び順で A(進行中)・B(完了)・C(未着手)・D(中止)・E(削除済み、未着手)。移動先 S2(計画中、すでに X が入っている)。完了すると、A と C だけが S2 に入り、`rank` は X のあとに A、C の順。B・D は S1 のまま、E は触らない(ログに行が足されない)。S1 は完了、確定値が入る。Summary「スプリント「…」を完了」。アイテムの履歴に「スプリントを変更」が A と C に 1 件ずつ。
- `TestCloseSprintToEmptyBacklog`: 移動先がバックログで、バックログの枠が空なら、`rank` は変えず、並び順は A・C のまま。
- `TestCloseSprintRefusals`: 計画中・完了済みのスプリント、移動先が実行中・完了・削除済み・ないスプリントは `port.ErrNotFound`。
- `TestCloseSprintZero`: 0 件でも完了でき、確定値は 0。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/planning/ -run Close`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/app/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/planning
git commit -m "feat(planning): スプリントの完了で確定値を記録し、未完了を移動先の末尾へ移す(F1 Task 7)"
```

### Task 8: リリースの作成・編集・状態の変更

**Files:**
- Create: `internal/app/planning/releases.go`
- Test: `internal/app/planning/releases_test.go`

**Interfaces:**
- Produces:
  - `type ReleaseInput struct { Name, Date, Description string }`
  - `func (s Service) CreateRelease(ctx, store, ws, in ReleaseInput) (planning.Release, error)` — 未リリースで作る
  - `func (s Service) UpdateRelease(ctx, store, ws, id change.ID, in ReleaseInput) error`
  - `func (s Service) SetReleaseState(ctx, store, ws, id change.ID, to planning.ReleaseState) error` — 同じ状態は何も保存しない。リリース済みにするとき予定日が空なら今日を同じ保存で入れる。確認は画面が出す(サーバーは断らない)

- [ ] **Step 1: 失敗するテストを書く**

- `TestCreateRelease`: Summary「リリース「v2.1.0」を作成」。同じ名前は「同じ名前のリリースがあります」。空は「名前を入力してください」。
- `TestReleaseState`: 予定日なしでリリース済みにすると予定日が今日になる。予定日ありなら変えない。アーカイブ → 未リリースもできる。Summary は「リリース「v2.1.0」をリリース済みにする」「…をアーカイブにする」「…を未リリースにする」。
- `TestUpdateRelease`: 名前・予定日・説明を変え、Summary「リリース「…」を編集」。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `go test ./internal/app/planning/`
Expected: 書く前は FAIL、書いたあと PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/planning
git commit -m "feat(planning): リリースの作成・編集・状態の変更を足す(F1 Task 8)"
```

### Task 9: スプリントとリリースの削除と復元

**Files:**
- Create: `internal/app/planning/remove.go`
- Test: `internal/app/planning/remove_test.go`

**Interfaces:**
- Produces:
  - `func (s Service) DeleteSprint(ctx, store, ws, id change.ID) error`・`DeleteRelease(...)` — F1-13
  - `func (s Service) RestoreSprint(ctx, store, ws, id change.ID) error`・`RestoreRelease(...)` — F1-14。削除済みでないものは `port.ErrNotFound`

- [ ] **Step 1: 失敗するテストを書く**

- `TestDeleteSprint`: 計画中の S1 に A(未着手)・B(完了)・C(削除済み)。削除すると S1 は削除済み、A と B のスプリントが空、C は変わらない。`rank` は変わらない。Summary「スプリント「…」を削除」。実行中は「実行中は削除できません」で断る。
- `TestRestoreSprint`: 削除のあと、A を別のスプリント S2 に入れてから復元すると、S1 が戻り、B は S1 に戻り、A は S2 のまま。Summary「スプリント「…」を復元」。削除済みでない同じ名前の S3 があれば「同じ名前のスプリントがあります。先にそのスプリントの名前を変えてください」。
- `TestRestoreSprintKeepsConcurrentAdd`: 削除の保存と同じ時刻に別の op で D に `set sprint S1` を書いた(同期の形。`prev` が削除の行を見ていない)ログで、復元すると D も S1 に入った状態(05 の 155 行)。
- `TestDeleteRestoreRelease`: リリースも同じ(どの状態でも消せる、文言はリリースの文)。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `go test ./internal/app/planning/`

- [ ] **Step 5: コミット**

```bash
git add internal/app/planning
git commit -m "feat(planning): スプリントとリリースの削除と復元を足す(F1 Task 9)"
```

### Task 10: アイテムのスプリントとリリースを変える

**Files:**
- Create: `internal/app/planning/assign.go`
- Test: `internal/app/planning/assign_test.go`

**Interfaces:**
- Produces:
  - `func (s Service) SetSprint(ctx, store port.StoreID, id item.ID, sprint *change.ID) error` — F1-11・F1-15・F1-16。F2 のバックログの選択も使う
  - `func (s Service) SetRelease(ctx, store port.StoreID, id item.ID, release *change.ID) error`

- [ ] **Step 1: 失敗するテストを書く**

- `TestSetSprint`: Story をバックログから S1(計画中、Y が入っている)に入れると、`sprint` と、Y のあとの `rank` が 1 回の保存で書かれる。Summary「HH-3 の スプリント を Sprint 1 に変更」。バックログに戻すと「HH-3 の スプリント を バックログ に変更」で、バックログの枠の末尾へ。
- `TestSetSprintRefusals`: Epic・Step は `ErrInvalidOperation`。完了した S0 へは「完了したスプリントには入れられません」。いま S0 に入っているアイテムで S0 を選ぶと何も保存しない。削除済み・ない・別のワークスペースのスプリントは `ErrNotFound`。削除済みのアイテムは `ErrNotFound`(`port.LiveItem`)。
- `TestSetRelease`: Epic にも入れられる。Step は `ErrInvalidOperation`。アーカイブのリリースは、いま入っていれば何もせず、ほかは `ErrInvalidOperation`。Summary「HH-1 の リリース を v2.1.0 に変更」「… を なし に変更」。`rank` は書かない。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `go test ./internal/app/planning/`

- [ ] **Step 5: コミット**

```bash
git add internal/app/planning
git commit -m "feat(planning): アイテムのスプリントとリリースを変える(F1 Task 10)"
```

### Task 11: 画面の読み取り(概要・スプリントのカード・リリースのカード)

**Files:**
- Create: `internal/app/planning/view.go`
- Test: `internal/app/planning/view_test.go`

**Interfaces:**
- Produces:
  - `type Overview struct { Sprints []planning.Sprint /* ScreenOrder */; Releases []planning.Release /* ReleaseOrder */; Active *planning.Sprint; Defaults SprintInput; Today string }`
  - `func (s Service) Overview(ctx, store, ws) (Overview, error)` — 詳細パネル・作成ダイアログの選択肢、見出しの「実行中」、作成ダイアログの既定に使う
  - `type Row struct { ID item.ID; Display string; Kind item.Kind; Title string; Points *float64; Status change.ID; Assignee string }`
  - `type SprintCard struct { Sprint planning.Sprint; Total, Done planning.Totals /* 中止を除く */; Undone, Canceled int; Items []Row /* 削除済みでない作業項目すべて、ワークスペースの並び順 */ }`
  - `type ReleaseCard struct { Release planning.Release; Target, Done, Undone int /* Story・Task・Bug、中止を除く */; Items []Row /* Step 以外すべて(Epic・中止を含む)、並び順 */ }`
  - `func (s Service) SprintCards(ctx, store, ws) ([]SprintCard, error)`(ScreenOrder)、`ReleaseCards(...)`(ReleaseOrder)
  - `planning.WorkOrder(its []item.Item)`(Task 2 の order.go に足す。`item.SortSiblings` と同じ比べ方をワークスペースの全体に当てる)

- [ ] **Step 1: 失敗するテストを書く**

- `TestSprintCards`: S1 に 3 pt 完了・2 pt 進行中・ポイントなし未着手・1 pt 中止 → `Total{5, 3}`・`Done{3, 1}`・`Undone 2`・`Canceled 1`、Items は 4 件で並び順。Epic を S1 に入れた手書きのログ(あり得ない値)は数えない。完了したスプリントは `Confirmed` を持つ。
- `TestReleaseCards`: Epic・Story(完了)・Task(中止)・Step(手書きで入れた)→ Target 1・Done 1・Undone 0、Items は Epic・Story・Task。
- `TestOverview`: 削除済みは出ない。`Active` は実行中。`Defaults` は Task 2 の既定。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `go test ./internal/app/planning/ ./internal/domain/planning/`

- [ ] **Step 5: コミット**

```bash
git add internal/app/planning internal/domain/planning
git commit -m "feat(planning): スプリントとリリースのカードと選択肢を読む(F1 Task 11)"
```

### Task 12: 作成ダイアログのスプリントと複製

**Files:**
- Modify: `internal/app/items/create.go`(`CreateInput.Sprint *change.ID`、`build` でもとになるアイテムにだけ入れる)、`internal/app/items/duplicate.go`(`copyOf`)
- Test: `internal/app/items/items_test.go`・`duplicate_test.go`・`template_test.go`(いまのファイルに足す)

**Interfaces:**
- Consumes: `planning.FromLog`・`UsableSprint`・`UsableRelease`・`IsWork`
- Produces: `CreateInput.Sprint`。作成でスプリントを選べるのは作業項目で、計画中か実行中の使えるスプリントだけ。完了は「完了したスプリントには入れられません」、ほかは `port.ErrNotFound`、Epic・Step は `port.ErrInvalidOperation`

- [ ] **Step 1: 失敗するテストを書く**

- `TestCreateWithSprint`: Story を S1 に入れて作ると `Sprint == "S1"`。完了の S0 は断る。テンプレートで子と Step も作るとき、スプリントはもとになるアイテムだけ(03 の 452 行)。
- `TestDuplicateSprintRelease`: 複製元が実行中の S1・リリース R1 → 写す。完了の S0 → 写さない。アーカイブの R0 → 写さない。削除済みの S2・R2 → 写さない(F1-22)。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `go test ./internal/app/items/`

- [ ] **Step 5: コミット**

```bash
git add internal/app/items
git commit -m "feat(items): 作成ダイアログのスプリントと、複製のスプリント・リリースを足す(F1 Task 12)"
```

### Task 13: API と組み立て

**Files:**
- Create: `internal/adapter/httpapi/planning/planning.go`・`dto.go`・`items.go`
- Modify: `internal/adapter/httpapi/routes.go`(登録)、`internal/adapter/httpapi/items/*.go`(Item の DTO に `sprint`・`release`、作成の body に `sprint`)、`cmd/hedgehug/wire.go`・`wire_e2e.go`(`appplanning.Service` を渡す)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`)
- Test: `internal/adapter/httpapi/planning/planning_test.go`

**Interfaces:**
- Produces(`base = /api/stores/{store}/workspaces/{workspace}`):
  - `GET {base}/planning` → `PlanningOverview{ sprints, releases, active, defaults{name,start,end}, today }`
  - `GET {base}/sprints` → `SprintCard[]`、`POST {base}/sprints`(`SprintInput`)→ 201、`PUT {base}/sprints/{sprint}`、`POST {base}/sprints/{sprint}/start`(`{start,end}`)、`POST {base}/sprints/{sprint}/close`(`{destination: id|null}`)、`DELETE {base}/sprints/{sprint}`、`POST {base}/sprints/{sprint}/restore`
  - `GET {base}/releases` → `ReleaseCard[]`、`POST {base}/releases`、`PUT {base}/releases/{release}`、`PUT {base}/releases/{release}/state`(`{state}`)、`DELETE {base}/releases/{release}`、`POST {base}/releases/{release}/restore`
  - `PUT /api/stores/{store}/items/{item}/sprint`(`{sprint: id|null}`)・`/release`(`{release: id|null}`)→ 204
  - OperationID は `get-planning`・`list-sprint-cards`・`create-sprint`・`update-sprint`・`start-sprint`・`close-sprint`・`delete-sprint`・`restore-sprint`・`list-release-cards`・`create-release`・`update-release`・`set-release-state`・`delete-release`・`restore-release`・`set-item-sprint`・`set-item-release`
  - 断りは `apierr` のいまの対応(`refusal` は 422 と文、`ErrNotFound` は 404、`ErrInvalidOperation` は 400)

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/automations/automations_test.go` の形(humatest)。

- `TestSprintAPI`: 作成 → 201、同じ名前 → 422「同じ名前のスプリントがあります」、開始 → 204、カードに出る、完了(destination null)→ 204、削除 → 204、復元 → 204。
- `TestReleaseAPI`: 作成・状態・削除・復元。
- `TestItemSprintAPI`: `PUT …/sprint` で入り、アイテムの GET の `sprint` に出る。完了のスプリント → 422。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: FAIL

- [ ] **Step 3: 書いて、生成物を作り直す**

Run: `make generate`

- [ ] **Step 4: テストと lint が通ることを確かめる**

Run: `go test ./... && make lint`
Expected: PASS(archtest のファイル数と依存の向きを含む)

- [ ] **Step 5: コミット**

```bash
git add internal/adapter internal/app cmd api web/packages/api/src/generated.ts
git commit -m "feat(api): スプリントとリリースの API と、アイテムのスプリント・リリースを足す(F1 Task 13)"
```

### Task 14: 画面の型・口・API とタブ・見出し

**Files:**
- Create: `web/packages/core/src/model/planning.ts`、`web/packages/core/src/port/planning.ts`、`web/packages/api/src/planning.ts`、`web/packages/core/src/features/planning/overview.ts`
- Modify: `web/packages/core/src/model/item.ts`(`sprint`・`release`。空は `""`)、`web/packages/api/src/items.ts`(読み書き)、`web/packages/core/src/features/router/route.ts`・`resolve.ts`(`WorkspaceTab` に `"sprints" | "releases"`)、`web/app/src/screens/workspace/WorkspaceScreen.svelte`(タブ、見出しの「実行中」)、`web/app/src/wire.ts`(口)、`web/packages/core/src/index.ts`
- Test: `web/packages/core/src/features/planning/overview.test.ts`、`features/router/*.test.ts`(いまのファイルに足す)、`web/packages/api/src/planning.test.ts`

**Interfaces:**
- Produces:
  - `model/planning.ts`: `Sprint`・`Release`・`Totals`・`SprintCard`・`ReleaseCard`・`PlanRow`・`PlanningOverview`(API の DTO と同じ項目、camelCase)
  - `port/planning.ts`: `PlanningPort` に `overview`・`sprintCards`・`releaseCards`・`createSprint`・`updateSprint`・`startSprint`・`closeSprint`・`deleteSprint`・`createRelease`・`updateRelease`・`setReleaseState`・`deleteRelease`・`setItemSprint`・`setItemRelease`(引数は `storeId, workspaceId, …`。復元は G が足す)
  - `features/planning/overview.ts`: `activeLabel(o: PlanningOverview, thisYear: number): string | null` →「実行中: Sprint 4(11/2〜11/15)」
- タブは「概要」「スプリント」「リリース」「設定」(F1-20)。見出しの補足は、実行中のスプリントがあるときだけ `PageHead` の crumbs の後ろに出す。

- [ ] **Step 1: 失敗するテストを書く**

- `activeLabel`: 実行中なし → null、今年の日付 → 「実行中: Sprint 4(11/2〜11/15)」、ほかの年 → `formatDay` の年つきの書き方。
- ルーター: `/s/work/w/HH/sprints` と `/releases` を読み書きでき、知らないタブは概要。
- API: `setItemSprint(…, null)` が `{sprint: null}` を送る。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- planning router`
Expected: FAIL

- [ ] **Step 3: 書く**

画面は Store の出来事(`store`)で概要を読み直す(いまの設定の画面と同じ仕組み)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web check && pnpm -C web test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): スプリントとリリースの型・口・タブと、見出しの「実行中」を足す(F1 Task 14)"
```

### Task 15: スプリントの画面(カード)

**Files:**
- Create: `web/packages/core/src/features/planning/sprints.ts`、`web/app/src/screens/planning/SprintsTab.svelte`、`SprintCard.svelte`、`PlanItems.svelte`(カードとリリースのアイテムの欄で共用)
- Modify: `web/app/src/screens/workspace/WorkspaceScreen.svelte`(タブの中身)
- Test: `web/packages/core/src/features/planning/sprints.test.ts`

**Interfaces:**
- Produces(`sprints.ts`):
  - `sections(cards: SprintCard[]): { state: SprintState; label: "実行中" | "計画中" | "完了"; cards: SprintCard[] }[]`(空の状態は出さない)
  - `pill(s: Sprint, today: string): string | null` —「残り {N} 日」「期限を過ぎています」(実行中だけ)
  - `progressText(c: SprintCard): string` —「{完了} / {全体} 件 · {完了} / {全体} pt」、完了したスプリントは「 · 確定 {実績}/{計画} pt」を続ける。ポイントの書き方は詳細パネルのポイントと同じ
  - `progressRatio(c: SprintCard): number`(全体が 0 なら 0)
  - `periodText(s: Sprint, thisYear: number): string` —「開始日 〜 終了日」か「期間未定」
  - `startDisabledReason(o: PlanningOverview): string | null` —「実行中のスプリントを先に完了してください」
- 画面: 05 の 179〜208 行。カードの見出し・期間・「開始」(計画中、実行中があるときは無効で理由を添える)・「完了…」(実行中)・メニュー(「編集…」「削除…」、実行中の「削除…」は無効で「実行中は削除できません」)・ゴール「ゴール: …」・進捗のバー・「アイテム {N} 件」の折りたたみ(実行中だけ開く)。アイテムの行は種類・ID・タイトル(押すと詳細パネル)・ポイント・ステータス・担当者、なければ「ありません」。スプリントがないときは「スプリントはまだありません」と「期間を決めて Story・Task・Bug をまとめると、バーンダウンとベロシティが見られます。」と「スプリントを作成」。右上にも「スプリントを作成」。閲覧のみの Store では書く操作を無効にして「閲覧のみの Store です」(01 の 124 行)。日付が変わったら「残り N 日」を更新する(01 の 132 行。いまの日付の変化の知らせを使う)

- [ ] **Step 1: 失敗するテストを書く**

`sections` の並びと空の状態、`pill`(当日 1 日、過ぎたら「期限を過ぎています」、計画中は null)、`progressText`(05 の 191 行の例と、0 件の「0 / 0 件 · 0 / 0 pt」、完了の「 · 確定 5/8 pt」)、`startDisabledReason`。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `pnpm -C web check && pnpm -C web test`

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): スプリントの画面のカードを足す(F1 Task 15)"
```

### Task 16: スプリントのダイアログ(作成・編集・開始・完了・削除)

**Files:**
- Create: `web/packages/core/src/features/planning/sprintDialogs.ts`、`web/app/src/screens/planning/SprintDialog.svelte`、`StartSprintDialog.svelte`、`CloseSprintDialog.svelte`、`DeleteSprintDialog.svelte`
- Test: `web/packages/core/src/features/planning/sprintDialogs.test.ts`

**Interfaces:**
- Produces:
  - `startBody(c: SprintCard): string` —「{N} 件・{X} pt で始めます。」(中止を除く)
  - `startDefaults(s: Sprint, today: string): { start: string; end: string }` — 開始日は今日、終了日はスプリントの終了日が今日以降ならその日、なければ今日の 13 日後
  - `closeBody(c: SprintCard): string` —「完了 {X} 件・未完了 {Y} 件です。」+ 中止があれば「中止 {Z} 件はスプリントに残り、集計には含めません。」
  - `closeDestinations(o: PlanningOverview, closing: string): { id: string | null; label: string }[]` と既定(計画中の最も早いもの、なければバックログ)
  - `deleteBody(name: string, items: number, retentionDays: number): string` — 05 の 151 行の文
  - 各ダイアログは保存の断りの文をダイアログの中に出し、閉じない。成功でトースト(F1-18)
- ダイアログの文言: タイトル「スプリントを作成」「スプリントを編集」、ボタン「作成」「保存」。開始のボタン「開始」。完了のボタン「完了」、未完了が 1 件以上のときだけ「未完了の {Y} 件の移動先」のセレクト。削除のタイトル「スプリントを削除」、ボタン「削除」(危険な見た目)。作成ダイアログの初めの値は `PlanningOverview.defaults`。保持日数は Store の保持日数を読む(いまの口)

- [ ] **Step 1: 失敗するテストを書く**

各関数を 05 の文で確かめる(0 件の「0 件・0 pt で始めます。」、中止ありの完了の本文、移動先の並びと既定、削除の本文のアイテムあり・なし)。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `pnpm -C web check && pnpm -C web test`

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): スプリントの作成・編集・開始・完了・削除のダイアログを足す(F1 Task 16)"
```

### Task 17: 詳細パネルと作成ダイアログのスプリント・リリース

**Files:**
- Create: `web/packages/core/src/features/planning/choices.ts`
- Modify: `web/app/src/screens/item/DetailProperties.svelte`(プロパティの並びの後ろに 2 行)、`web/packages/core/src/features/items/detail.ts`(読み込みに概要を足し、`setSprint`・`setRelease`)、`createDialog.ts`・`createDialogTypes.ts`・`createDialogSwitch` の規則(スプリントの欄)、`web/app/src/screens/item/CreateItemFields.svelte`
- Test: `web/packages/core/src/features/planning/choices.test.ts`、`features/items/createDialogSwitch.test.ts`(足す)

**Interfaces:**
- Produces(`choices.ts`):
  - `sprintChoices(o: PlanningOverview, current: string): { id: string; label: string }[]` — 先頭に `{id: "", label: "バックログ"}`、続いて完了していないスプリント(実行中は「{名前}(実行中)」)、いまのスプリントが完了済みならそれも残す。削除済み・ないスプリントの `current` は `""` として扱う
  - `releaseChoices(o: PlanningOverview, current: string)` — 「なし」と、アーカイブでないもの、いまのものがアーカイブなら残す
  - `showSprint(kind: Kind): boolean`(Story・Task・Bug)、`showRelease(kind: Kind): boolean`(Step 以外)
- 詳細パネルの選択は保存の断りの文をトーストに出し、値を戻す(いまのプロパティの行と同じ)。閲覧のみの Store では無効

- [ ] **Step 1: 失敗するテストを書く**

- `sprintChoices`: 完了・削除済みを出さない。いま完了の S0 に入っていれば S0 を残す。実行中の「(実行中)」。削除済みのスプリントに入っているアイテムの選択は「バックログ」。
- `releaseChoices`: アーカイブを出さず、いまのアーカイブは残す。
- 作成ダイアログ: 初めは「バックログ」。Epic に切り替えると欄が消え、Story に戻すと「バックログ」。ワークスペースを変えると「バックログ」。作成の入力に `sprint` が入る。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `pnpm -C web check && pnpm -C web test`

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 詳細パネルと作成ダイアログにスプリントとリリースを足す(F1 Task 17)"
```

### Task 18: リリースの画面

**Files:**
- Create: `web/packages/core/src/features/planning/releases.ts`、`web/app/src/screens/planning/ReleasesTab.svelte`、`ReleaseCard.svelte`、`ReleaseDialog.svelte`、`DeleteReleaseDialog.svelte`
- Modify: `web/app/src/screens/workspace/WorkspaceScreen.svelte`
- Test: `web/packages/core/src/features/planning/releases.test.ts`

**Interfaces:**
- Produces(`releases.ts`):
  - `stateLabel(s: ReleaseState)`(未リリース・リリース済み・アーカイブ)と色(青・緑・灰)
  - `overdue(r: Release, today: string): boolean`(未リリースで予定日が今日より前)
  - `progressText(c: ReleaseCard): string` —「{完了} / {対象} 件完了」+ 残りがあれば「 · 残り {n} 件」
  - `stateMenu(r: Release): { to: ReleaseState; label: string }[]` — いまの状態を除く 2 つ「{状態}にする」
  - `releaseConfirm(c: ReleaseCard): string | null` — 未完了が 1 件以上で「{名前} には未完了のアイテムが {N} 件あります。このままリリース済みにしますか?」
  - `deleteReleaseBody(name: string, items: number, retentionDays: number): string` — 05 の 267 行
- 画面: 05 の 254〜267 行。ボタン「リリース済みにする」(未リリースだけ)、メニュー「編集…」・状態 2 つ・「削除…」。「アイテム {N} 件」の折りたたみ(Task 15 の `PlanItems`)、なければ「ありません。アイテムの「リリース」で選べます」。リリースがないときは「リリースはまだありません」と「バージョンを作って Epic・Story・Task・Bug をまとめると、何が残っているかが見えます。」。右上に「リリースを作成」。作成のダイアログの名前の欄の例は `v2.1.0`、予定日、説明。確認のダイアログはタイトル「リリース済みにする」、ボタン「リリース済みにする」。保存の断りはダイアログの中に出し、成功でトースト(F1-18)。閲覧のみの Store では書く操作を無効にして「閲覧のみの Store です」

- [ ] **Step 1〜4: 失敗するテストを書き、落ちることを確かめ、書き、通す**

各関数を 05 の文で確かめる(予定日が今日は過ぎていない、残り 0 件で「 · 残り」を出さない、確認は未完了 0 件で null)。

Run: `pnpm -C web check && pnpm -C web test`

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): リリースの画面を足す(F1 Task 18)"
```

### Task 19: A5a・A5c に合わせる(develop にあるときだけ。F1-23)

**Files:**(A5a・A5c が入れたファイル。入っていなければこのタスクは飛ばし、飛ばしたことを PR の本文に書く)
- Modify: `internal/domain/transfer/`(A5a の `Remap`)、パレットの「作成」の群(A5c)

- [ ] **Step 1: 失敗するテストを書く**

- A5a: 別のワークスペースへ移すと、移したアイテムに `set sprint null`・`set release null` が同じ操作で書かれる(値のあるときだけ。02 の 231 行)。
- A5c: パレットの「作成」の群に「スプリントを作成」「リリースを作成」が、ワークスペースかプロジェクトの画面を開いているときだけ出る。選ぶと、開いているワークスペースの「スプリント」(「リリース」)のタブを開き、作成ダイアログを開く(02 の 619・652 行)。

- [ ] **Step 2〜4: 落ちることを確かめ、書き、通す**

Run: `go test ./... && pnpm -C web test`

- [ ] **Step 5: コミット**

```bash
git add -A internal web
git commit -m "feat: 移動でスプリントとリリースを外し、パレットにスプリントとリリースの作成を足す(F1 Task 19)"
```

### Task 20: e2e と他のトラックへの記録

**Files:**
- Create: `web/e2e/sprints.spec.ts`、`web/e2e/releases.spec.ts`
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C5 の行に F1-24 の C5 の分。「F に求めること」は F1 で済んだと書く)、`docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(B5 の行に B5b の分)、`docs/superpowers/plans/2026-11-12-hedgehug-foundation.md`(決定 2 の行に「F1-1 で口を足さないことにした」)

- [ ] **Step 1: e2e を書く**

- `sprints.spec.ts`: スプリントのタブで「スプリントを作成」→ 既定の名前「Sprint 1」で作成 → トースト「Sprint 1 を作成しました」。Story の詳細パネルで「スプリント」に Sprint 1 を選ぶ → カードに「アイテム 1 件」。「開始」→ トースト「Sprint 1 を開始しました」、見出しに「実行中: Sprint 1(…)」、もう 1 つ作ったスプリントの「開始」が無効で「実行中のスプリントを先に完了してください」。「完了…」で本文「完了 0 件・未完了 1 件です。」、移動先「バックログ」で完了 → 詳細パネルのスプリントが「バックログ」。計画中のスプリントの「削除…」→ 本文に「設定の「削除済み」から戻せます。」→ トースト「スプリントを削除しました」。同じ名前で作ると、ダイアログに「同じ名前のスプリントがあります」。
- `releases.spec.ts`: 「リリースを作成」→ v2.1.0 → Story を入れる → 「リリース済みにする」で確認「v2.1.0 には未完了のアイテムが 1 件あります。このままリリース済みにしますか?」→ 状態のピルが「リリース済み」、予定日が今日。メニューの「アーカイブにする」→ Story の詳細パネルの選択肢に v2.1.0 が残る。別の Story の選択肢には出ない。

- [ ] **Step 2: 流して通ることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test sprints releases`
Expected: PASS

- [ ] **Step 3: 他のトラックの計画を直す**

上の Files の 3 つに F1-24・F1-1 を書く。G の計画はまだないので、G へ渡すもの(F1-24 の G の分)はこの計画の末尾「G に求めること」に置き、ユーザーの記録(メモリ)にも書く。

- [ ] **Step 4: 全体の確かめ**

Run: `make check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/e2e docs/superpowers/plans
git commit -m "test(e2e): スプリントとリリースの流れを確かめ、他のトラックの計画に渡すものを書く(F1 Task 20)"
```

---

## G に求めること

トラック G(Store の運用と保守)の計画を書くときに、次のことを入れる。

- 設定の「削除済み」に、スプリント・リリースの行を出す(`planning.FromLog` の `Deleted` のあるもの。名前、削除した日時と人)。「復元」は F1 の `POST …/sprints/{id}/restore`・`…/releases/{id}/restore` を呼び、トーストは 08 の 368 行。名前が重なるときの「復元」の無効と文は F1 の断りの文と同じ。ワークスペースが削除済みなら、F1 の API は名前の重なりより先に断る(Task 19b)ので、「復元」の無効と文もそれに合わせる。
- アイテムの復元(アイテム・プロジェクト・ワークスペースのどれでも)で、入っていたスプリントが完了済みで分類が未着手か進行中なら `set sprint null`、使えない(F1-5)なら `set sprint null`、リリースが使えないなら `set release null` を同じ op で書き、トーストに 08 の 368 行の文を続ける。
- 完全に削除(手動・自動)するスプリント・リリースを指すアイテムの参照を、同じ保存で `set sprint null`・`set release null` で外す(08 の 385 行)。スプリント・リリースの行は `log.jsonl` から取り除き、`purged.jsonl` に `entity: sprint`・`release` で書く。
