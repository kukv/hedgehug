# hedgehug トラック F2a(バックログのタブ)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 05 の「バックログ」の画面(上にスプリントの枠、下にバックログの枠、行のポイント入力・ステータス・スプリントの選択・「上へ」「下へ」・ドラッグ、「スプリントを作成」「作成」、消えるときのトースト)と、作業項目の並び順をワークスペースで 1 本にそろえる規則(作成と移動で末尾に入る)を作る。

**Architecture:** 枠の取り出しと並べ替えの順位の計算は `domain/planning` の新しい `backlog.go` に置く。読み取り(`Backlog`)と並べ替えの保存(`PlaceItem`)は `app/planning`、API は `adapter/httpapi/planning`。作成・移動の順位は、`app/items`・`app/transfers` が `domain/planning` の関数を呼ぶ(app の機能どうしは import しない)。画面は core の `features/planning/backlog.ts`(状態と保存とトースト)と `backlogDrop.ts`(ドロップ先の計算)、Svelte は描画とポインターのドラッグだけ。

**Tech Stack:** Go 1.27.3、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない(ドラッグは Pointer Events で書く)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/05-planning-reports.md`「数え方の共通ルール」(9〜17 行)、「バックログ」(19〜67 行)、「開始」の最後の段落(122 行)、「存在しないスプリントとリリース」(175 行)、「モックからの変更点」のうちバックログの行(440・441・453・462 行)
- 02 の 41 行(位置を決めない移動と作成は親の子の末尾)、180 行(複製の並び)、319 行(Step から昇格したものは末尾)、717 行(移動は作成と同じ規則)
- 04 の 268 行(並び順は、作成したものが末尾に入り、バックログやツリーで並べ替えると変わる)、482〜497 行(閲覧のみで無効にするもの、ドラッグの代わりの経路)
- 01 の 124 行(閲覧のみの Store)、221 行(ドラッグはマウスとタッチ、代わりの経路)
- 06 の 155 行(画面への反映に「バックログ」)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-f1.md` の F1-5・F1-11・F1-16・F1-21、`internal/app/planning/assign.go` の `SetSprint`

## 始める前に

- トラック F は 3 本に分ける(ユーザーの決定、2026-11-14): F1 スプリントとリリース(develop に入った、PR #206)→ F2 バックログ・F3 レポート(develop に入った、PR #211)。
- F2 は 2 本に分ける(ユーザーの決定、2026-11-14): **F2a = この計画**(バックログのタブ。行の選択のチェックは除く)、**F2b** = 選択バー・一括変更・一括削除(04「一括選択と一括変更・一括削除」)を、検索のテーブル(トラック E)でも使える共通の部品として作り、バックログの行にチェックを付ける。E はテーブルに付けるだけ。F2a は 1 計画・1 PR。
- 05 の行の並びの先頭の「選択のチェック」と、コマンドパレットの「選んでいるアイテム」へのバックログの登録(A5c の計画の `selection.provide(2, …)`)は F2b に入れる(1 件の選択はチェックで決まるため)。
- 実行中のスプリントのカードの「ボード」は E(F1 の「始める前に」)。F2a はスプリントの画面に触らない。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/items`・`app/port`・`app/apptest`・`app/workspaces`・`infra/gitstore` は 20 個で満杯なので、ファイルを足さない(`app/items` は既存のファイルを直す)。`domain/planning` は 13 個、`app/planning` は 8 個、`app/transfers` は 17 個。`web/packages/core/src/features/items/detail.ts` は 291 行。
- app の機能どうしは import しない(設計書 2 節)。`app/items`・`app/transfers` は `app/planning` を呼ばず、共通の計算は `domain/planning` に置く。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書き、言い換えない。ポイントの表記は core の `pt`(`features/planning/sprints.ts`)、期間は `periodText`、閲覧のみの添え書きは `readOnlyNote`、保存の失敗は `sprintSaveFailed`(「保存できませんでした」)。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする(CI が差分を見る)。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| F2a-1 | 枠の中身: スプリントの枠は、削除済みでない実行中・計画中のスプリントを `ScreenOrder` の順(完了を除く)。各枠のアイテムは、そのスプリントの削除済みでない作業項目(完了・中止を含む)。バックログの枠は、使えるスプリント(F1-5)に入っていない、分類が未着手か進行中の作業項目。どの枠もワークスペースの並び順(`WorkOrder`)。スプリントの枠の「N 件 · X pt」は中止を除き完了を含む(`Totals`、F1 のカードと同じ)。バックログの枠は枠の全件 | 05 の 23〜28 行。判定は F1 の `inFrame`(`FrameEnd`)と同じ関数を使い、枠の末尾と表示がずれないようにする |
| F2a-2 | 作業項目の順位は、ワークスペースの作業項目の並び(`WorkOrder`)の中で決める。行の直前に置くときは、基準の行の順位 `hi` と、並びで基準より前にある、`hi` より小さい最も近い順位 `lo` の間(`rank.Between(lo, hi)`。なければ `lo` は空)。直後に置くときは、基準の順位 `lo` と、並びで基準より後ろにある、`lo` より大きい最も近い順位 `hi` の間(なければ `rank.After(lo)`)。動かすアイテム自身は並びから除いて数える。同じ順位が並ぶとき(F1-11 の `rank.After(枠の末尾)` が枠の外のアイテムと同じ値を作ることがあり、ふつうに起きる)も、必ず間の値ができる。同じ順位の群の中には入らず、群の手前(直後に置くときは群の後ろ)に入る | 05 の 44〜51 行。ツリーの `items.Reorder` の `gap` も、Task 12 から同じ `rank.Around` で値を作る |
| F2a-3 | 並べ替えの保存は `app/planning` の `PlaceItem(store, id, sprint *change.ID, anchor *item.ID, after bool)`。`sprint` は移動先の枠(nil はバックログ)。`anchor` があれば、その行の直前(`after` なら直後)に置き、枠が変わるなら `set sprint` も同じ保存に入れる。アクティビティは「{ID} の順位を変更」の 1 行(履歴には `set sprint` が「スプリントを変更」として残り、`set rank` は残らない)。`anchor` がなく枠が変わるときは `SetSprint`(枠の末尾、「{ID} の スプリント を {名前 または バックログ} に変更」)。`anchor` がなく同じ枠のときは枠の末尾(`rank.After(自分を除いた FrameEnd)`、「{ID} の順位を変更」)で、すでに末尾なら何もしない | 05 の 48・49・63〜67 行 |
| F2a-4 | `PlaceItem` の検査: `anchor` が、移動先の枠にいる、同じワークスペースの削除されていない別の作業項目でなければ `port.ErrNotFound`。移動先のスプリントは `planning.AssignableSprint`(完了済みは「完了したスプリントには入れられません」の断り、Epic・Step も断る)。同じ位置(同じ枠で、基準が自分の次の行への直前、前の行への直後、自分自身)は何もしない。保存の中で読み直し、削除済み(root: true)なら `assign`・`reorder` と同じく `EditableItem → ReviveItem → LiveItem` で削除を取り消してから書く | 05 の 50・56 行、06「他の書き手との重なり」(C3a-screen の規則) |
| F2a-5 | 作業項目に新しい順位を付ける操作は、ワークスペースの並びの末尾にも入れる: 新しい順位は `rank.After(max(親の子の末尾, ワークスペースの作業項目の末尾))`(`planning.EndRank`)。当てるのは、作成(`create.go`)、テンプレートの作成(`createtemplate.go` の子。順に `rank.After` を続ける)、複製(`duplicate.go`)、Step から作業項目への昇格(`changekind.go`)、同じワークスペースの中の移動(`items/move.go` の移すアイテム)。Epic と Step は今のまま(親の子の末尾)。親の子の末尾の規則(02 の 41・180・319 行)も同時に満たす | 05 の 46 行、04 の 268 行、02 の 717 行(移動は作成と同じ規則)。F1-21 で F2 に回された |
| F2a-6 | 別のワークスペースへの移動(アイテムの移動・プロジェクトの移動。同じ Store と別の Store)では、移す集まりの作業項目を、移動元の並び順のまま、移動先のワークスペースの作業項目の末尾に続けて置く(`planning.AppendWorks`)。移すアイテムが作業項目なら、その順位は親の子の末尾も満たす(F2a-5 の値から続ける)。作業項目でない移すアイテム(Epic)は今のまま親の子の末尾。下の Step は変えない。A5a-3 の「移すアイテムだけ rank も」を、作業項目について広げる | 02 の 717 行。移動先では作業項目が新しく並びに入るので、作成と同じく末尾に入れる。今のままだと移動元の順位のまま移動先の並びに散らばる |
| F2a-7 | 読み取りは新しい API `GET /api/stores/{store}/workspaces/{workspace}/backlog`(`app/planning` の `Backlog`)。行は `id`・`display`・`kind`・`title`・`priority`・`status`・`points`・`assignee`・`project`(ステータスのメニューがプロジェクトの使うステータスに絞るため)。枠は `sprint`・`total`(スプリントの枠だけ)・`rows`。設定とメンバーは画面がワークスペースとメンバーの口で読む(F1 の `createSprints` と同じ) | F1 の `SprintCards` と同じ形にして、開始のダイアログに枠をそのまま渡せるようにする |
| F2a-8 | 行の操作: タイトルは詳細パネルを開く。ステータスは詳細パネルと同じ `StatusButton`(`project` で使うステータスに絞る)で、`items.setProperty(…, "status", …)` で保存する。ポイントはその場の入力で、確定(Enter かフォーカスを外す)で `items.setProperty(…, "points", 文字列)`。空は値の削除。値の検査はサーバー(`item/input.go` の「ポイントは 0 以上の数で入力してください」)の断りをトーストに出し、入力を保存済みの値に戻す。スプリントのセレクトは F1 の `sprintChoices`(「(実行中)」を添える)で、`planning.setItemSprint`。同じ値は送らない。「上へ」「下へ」は同じ枠の前の行の直前・次の行の直後への `PlaceItem`。先頭の「上へ」と末尾の「下へ」は無効 | 05 の 34〜40 行。02 のステータスとポイントの規則をそのまま使う |
| F2a-9 | 断りと失敗: 断り(`Refused`)はその文を、ほかの失敗は「保存できませんでした」をエラーのトーストに出す(F1 の `assignPlan` と同じ)。どちらも枠を読み直す | F1-18 |
| F2a-10 | 消えるときのトースト「{ID} は{完了|中止}のため、バックログには表示されません」(通常の色): (1) バックログの枠の行のステータスで、保存の応答のアイテムの分類が完了・中止のとき。(2) バックログの行のタイトルから開いた詳細パネルで、ステータスの保存の応答のアイテムが、開いている枠のスプリントのどれにも入っておらず、分類が完了・中止のとき(開いたアイテムの ID を core の `backlog` が持ち、タブを離れるか別の行から開くと替わる)。(3) スプリントの枠の完了・中止の行を、スプリントのセレクトかドラッグでバックログの枠へ移したとき。同期・自動化の変更とスプリントの枠の中では出さない | 05 の 30 行 |
| F2a-11 | 詳細パネルからの知らせ: `DetailDeps` に `onPropertySaved?(storeId: string, item: Item): void` を足し、`setProperty` が保存の応答のアイテムで呼ぶ。`wire.ts` が `backlog.noticeDetail` につなぐ | `detail.ts` は開いた場所を知らない。300 行の上限の中で 2 行で足りる形にする |
| F2a-12 | ドラッグ: 行の持ち手の `pointerdown` で始め(`setPointerCapture`、持ち手は `touch-action: none`)、`pointermove` で `document.elementFromPoint` の `data-row`(内部 ID)・`data-frame`(スプリントの ID、バックログは空文字)を読み、行の上なら「その行の直前」、枠の空いた部分なら「枠の末尾」の印を出し、`pointerup` で core の `dropPlacement` が決めた保存をする。`pointercancel` と Esc でやめる。マウスとタッチはどちらも Pointer Events で動く。閲覧のみの Store では持ち手を無効にする | 01 の 221 行、05 の 48〜51 行。既存のドラッグの部品はない |
| F2a-13 | タブは「概要・バックログ・スプリント・リリース・レポート・設定」(01 の並びのうち作ったもの)。アドレスは `/s/{store}/w/{prefix}/backlog` | 01 の 166・201 行、F1-20 |
| F2a-14 | 枠の見出し: 名前、状態(「実行中」「計画中」)、期間(`periodText`。未定は「期間未定」)、「N 件 · X pt」、ゴールがあれば「ゴール: …」。計画中の枠に「開始」を常に出し、実行中のスプリントがあるときは無効にして「実行中のスプリントを先に完了してください」と添える(`startDisabledReason`)。押すとスプリントの画面と同じ開始のダイアログ。空の枠は「バックログからここにドラッグ、または行の「スプリント」で選択」。バックログの枠の見出しは「バックログ」と「N 件 · X pt · スプリントに入っていない未完了の Story・Task・Bug」、空なら「バックログは空です」。見出しに「スプリントを作成」(作成のダイアログ)と「作成」(`createDialog.open({ workspaceId, kind: "Story" })`)。どちらも閲覧のみで無効 | 05 の 24〜31・122 行 |
| F2a-15 | スプリントのダイアログ(作成・開始)は `WorkspaceScreen` のものを使い、出すタブを「スプリント」と「バックログ」に広げる。開始のダイアログの `card` の型は `Pick<SprintCard, "sprint" | "total">` にする | ダイアログを 1 か所に保つ。A5b の、Store の読み直しで開き直さない作り(`sid`・`wid` の `$derived`)はそのまま |
| F2a-16 | 画面の読み直し: Store の出来事で開いているバックログを読み直す(`wire.ts` の `reflect` の store の群に足す)。閲覧のみの Store では、ポイントの入力・ステータス・スプリントのセレクト・「上へ」「下へ」・持ち手・「開始」・「スプリントを作成」・「作成」を無効にし、`readOnlyNote` を添える。タイトルは押せる | 06 の 155 行、01 の 124 行、04 の 482 行 |

## Review Focus

1. **同じ順位のアイテムがあると、ドラッグや「上へ」が何もしない。** A・B・C を作り(順位 V・W・X)、A を空のスプリント S に入れ(順位は V のまま)、C も S に入れると C は `rank.After(V)` = W で B と同じ順位になる。S の枠で C の「上へ」を押すと、C が A の前に入る。バックログの枠の B を S の枠の C の直前にドロップすると、B が S の C の前に入る(F2a-2)。Task 1 の `TestPlacementTies` と Task 4 の `TestPlaceItemTies` に入れる。
2. **作ったアイテムが、バックログの真ん中に入る。** Epic E1 の下の Story(順位 V)と Epic E2 の下の Story(順位 V)があるワークスペースで、E1 の下に Story を作ると、バックログの末尾に入る(F2a-5)。Task 2 の `TestCreateAppendsToWorkspaceOrder` に入れる。
3. **完了したアイテムをスプリントからバックログへ移すと、何も知らせずに消える。** スプリントの枠の Done の行のスプリントで「バックログ」を選ぶと、「{ID} は完了のため、バックログには表示されません」が出て、行が消える(F2a-10)。Task 6 のテストと Task 10 の e2e に入れる。
4. **別のタブでスプリントを完了したあとに、そのスプリントの枠へドロップする。** 断り「完了したスプリントには入れられません」をエラーのトーストに出し、枠を読み直す(F2a-4・F2a-9)。Task 4 の `TestPlaceItemClosedSprint` と Task 6 のテストに入れる。
5. **閲覧のみの Store で、ドラッグだけ動く。** 持ち手を無効にし、ポインターを押してもドラッグが始まらない(F2a-12・F2a-16)。Task 8 のテストに入れる。

---

## ファイルの地図

```
internal/domain/planning/backlog.go        BacklogFrames・WorkEnd・EndRank・Placement・AppendWorks
internal/app/items/create.go・createtemplate.go・duplicate.go・changekind.go・move.go (変更)F2a-5
internal/app/transfers/move.go・project_move.go (変更)F2a-6(パッケージのファイル数の上限 20 のため、順位の関数は move.go に置く)
internal/app/planning/backlog.go           Backlog(読み取り)
internal/app/planning/place.go             PlaceItem(保存)
internal/adapter/httpapi/planning/backlog.go  API
web/packages/core/src/model/planning.ts・port/planning.ts (変更)
web/packages/core/src/features/planning/backlog.ts       createBacklog
web/packages/core/src/features/planning/backlogDrop.ts   dropPlacement・moveUp・moveDown・goneText
web/packages/core/src/features/items/detail.ts           (変更)onPropertySaved
web/packages/core/src/features/router/route.ts           (変更)WorkspaceTab に backlog
web/packages/api/src/planning.ts                         (変更)
web/app/src/screens/planning/BacklogTab.svelte・BacklogFrame.svelte・BacklogRow.svelte・PointsInput.svelte
web/packages/ui/src/patterns/dragRows.ts          ポインターのドラッグ(Svelte の action)
web/app/src/screens/planning/StartSprintDialog.svelte・SprintDialogs.svelte (変更)card の型
web/app/src/screens/workspace/WorkspaceScreen.svelte     (変更)タブとダイアログ
web/app/src/planning.ts・wire.ts                         (変更)backlog と読み直し
web/e2e/backlog.spec.ts
```

---

### Task 1: 枠と順位の計算(`domain/planning`)

**Files:**
- Create: `internal/domain/planning/backlog.go`
- Test: `internal/domain/planning/backlog_test.go`

**Interfaces:**
- Consumes: `counted`・`inFrame`・`Category`・`WorkOrder`・`ScreenOrder`・`Plan.LiveSprints`・`Plan.UsableSprint`(既存)、`rank.Between`・`rank.After`
- Produces:
  - `type Frame struct { Sprint *Sprint; Total Totals; Items []item.Item }` — `Sprint` が nil ならバックログの枠。`Total` はスプリントの枠は中止を除き、バックログの枠は全件
  - `func BacklogFrames(its []item.Item, ws workspace.ID, plan Plan, st workspace.Settings) []Frame` — F2a-1。実行中・計画中の枠を `ScreenOrder` の順に、最後にバックログの枠
  - `func WorkEnd(its []item.Item, ws workspace.ID) string` — ワークスペースの削除されていない作業項目の最大の順位(なければ空)
  - `func EndRank(siblingEnd string, its []item.Item, ws workspace.ID, kind item.Kind) string` — F2a-5。作業項目なら `rank.After(max(siblingEnd, WorkEnd))`、それ以外は `rank.After(siblingEnd)`
  - `func Placement(its []item.Item, ws workspace.ID, moving, anchor item.ID, after bool) (string, bool)` — F2a-2。新しい順位。`anchor` が並びにない(作業項目でない・削除済み・別のワークスペース)なら false
  - `func AppendWorks(moved []item.Item, start string) map[item.ID]string` — F2a-6。`moved` のうち作業項目を `WorkOrder` の順に、`start` から `rank.After` を続けた順位

- [ ] **Step 1: 失敗するテストを書く**

- `TestBacklogFrames`: 実行中 S1・計画中 S2(開始 11/20)・計画中 S3(期間未定)・完了 S0・削除済み S4。S1 に Done の A(3 pt)・中止の B(5 pt)・Todo の C(ポイントなし)。S4 に Todo の D。スプリントなしの Done の E、Todo の F、Epic G。結果は S1・S2・S3・バックログの 4 枠。S1 の `Items` は並び順で A・B・C、`Total` は `{Points: 3, Count: 2}`。バックログの枠は D・F(削除済みのスプリントの D はバックログ)で、E と G は出ない。
- `TestEndRank`: 作業項目の末尾 X、親の子の末尾 V のとき Story は `rank.After("X")`、Step は `rank.After("V")`。親の子の末尾 Z が大きいとき Story は `rank.After("Z")`。
- `TestPlacement`: 並び A(V)・B(W)・C(X)。C を A の直前 → `Between("", "V")`。A を C の直後 → `rank.After("X")`。A を B の直後 → `Between("W", "X")`。
- `TestPlacementTies`(Review Focus 1): 並び A(V)・B(W、作成が早い)・C(W)。C を A の直前 → `Between("", "V")`。A を C の直後 → `rank.After("W")`(B・C の群の後ろ)。F(Y)を C の直前 → `Between("V", "W")`(B・C の群の手前)で、結果の値が V より大きく W より小さいこと。
- `TestPlacementNotInOrder`: `anchor` が Epic・削除済み・別のワークスペースなら false。
- `TestAppendWorks`: 移す A(Story、順位 Y)・その下の Step・B(Task、順位 W)を `start` "X" から → B は `rank.After("X")`、A は `rank.After(rank.After("X"))`。Step は結果に入らない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/planning/ -run 'TestBacklogFrames|TestEndRank|TestPlacement|TestAppendWorks'`
Expected: FAIL(未定義)

- [ ] **Step 3: `backlog.go` に上の 5 つを書く**

`Placement` は、`moving` を除いた作業項目を `WorkOrder` で並べ、F2a-2 のとおり基準から前(後ろ)へ、基準と違う順位が出るまで進んで `lo`(`hi`)を決める。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning/backlog.go internal/domain/planning/backlog_test.go
git commit -m "feat(planning): バックログの枠と、ワークスペースの並びでの順位を計算する(F2a)"
```

---

### Task 2: 作成と同じワークスペースの中の移動で、並びの末尾に入れる(`app/items`)

**Files:**
- Modify: `internal/app/items/create.go:133`、`createtemplate.go:86-90`、`duplicate.go:81`、`changekind.go:84`、`move.go:176`
- Test: `internal/app/items/create_test.go`・`createtemplate_test.go`・`duplicate_test.go`・`changekind_test.go`・`move_test.go`(追記)

**Interfaces:**
- Consumes: `planning.EndRank`・`planning.WorkEnd`(Task 1)
- Produces: なし(順位の値だけが変わる)

ワークスペースの全アイテムは、`r.Projects().List(ws)` の各プロジェクトの `r.Items().List(p.ID)` で読む(`app/planning/service.go` の `items` と同じ読み方。`app/items` は満杯なので、既存のファイルに非公開の関数 `workspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error)` を `create.go` の `lastRank` の隣に置く)。

- [ ] **Step 1: 失敗するテストを書く**

- `TestCreateAppendsToWorkspaceOrder`(Review Focus 2): Epic E1 の下の Story S1(順位 V)、Epic E2 の下の Story S2(順位 W)。E1 の下に Story を作ると、順位は `rank.After("W")`。E2 の下に Step を作ると、その Story の子の末尾(`rank.After(子の末尾)`)のまま。
- テンプレート: Epic のテンプレートで子の Story 2 件を作ると、子の順位はワークスペースの作業項目の末尾から `rank.After` を続けた値。
- 複製: 複製した Story の順位はワークスペースの末尾。
- 昇格: Step を Story にすると、順位は `max(Epic の子の末尾, ワークスペースの作業項目の末尾)` の `rank.After`。
- 移動: 同じワークスペースの別の Epic へ Story を移すと、順位は同じ規則。Epic の移動はプロジェクトの直下の末尾のまま。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run 'Workspace|Template|Duplicate|Promot|Move'`
Expected: 新しいテストが FAIL

- [ ] **Step 3: 5 か所の `rank.After(lastRank(…))` を `planning.EndRank(lastRank(…), wsItems, ws, kind)` にする**

テンプレートの子は、最初の子を `EndRank("", wsItems, ws, kind)`、続きを `rank.After(前の値)` にする(Step の子は今のまま)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/items/`
Expected: PASS(既存のテストで順位の値を見ているものは、新しい規則の値に直す。直したテストは差分に「F2a-5」と書く)

- [ ] **Step 5: コミット**

```bash
git add internal/app/items/
git commit -m "feat(items): 作業項目の作成と移動で、ワークスペースの並びの末尾に入れる(F2a)"
```

---

### Task 3: 別のワークスペースへの移動で、作業項目を移動先の並びの末尾に続ける(`app/transfers`)

**Files:**
- Modify: `internal/app/transfers/move.go`(順位の関数 `workRanks`・`workspaceItems`・`rankLine` を末尾に足す。パッケージのファイル数の上限 20 のため新しいファイルは作らない)
- Modify: `internal/app/transfers/move.go:120-145,213-224`、`project_move.go`(順位の行を書く箇所)
- Test: `internal/app/transfers/move_test.go`・`project_move_test.go`(追記。別の Store の場合の既存テストのファイルにも 1 件)

**Interfaces:**
- Consumes: `planning.EndRank`・`planning.WorkEnd`・`planning.AppendWorks`(Task 1)
- Produces: `func workRanks(dst destination, root item.Item, moved []item.Item, siblingEnd string, dstItems []item.Item) (rootRank string, ranks map[item.ID]string)`(非公開。移すアイテムの順位と、移す作業項目の順位)

- [ ] **Step 1: 失敗するテストを書く**

- アイテムの移動: 移動先のワークスペースの作業項目の末尾が X。Epic(下に Story A(順位 Y)・Task B(順位 W))を移すと、Epic はプロジェクトの直下の末尾、B は `rank.After("X")`、A はその次。Step の順位は変わらない。
- Story を移すと、その順位は `EndRank(移動先の親の子の末尾, 移動先, Story)`。
- 別の Store への移動でも同じ値になる。
- プロジェクトの移動: プロジェクトの作業項目が、移動元の並び順のまま、移動先の作業項目の末尾に続く。
- 移動先にある削除済みのアイテムは末尾の計算に入れない(`WorkEnd` は削除されていないものだけ)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ -run 'Rank|Order'`
Expected: FAIL

- [ ] **Step 3: `move.go` に順位の関数を書き、`placeLines` が移す作業項目にも `set rank` を書くようにする**

順位が今と同じアイテムには行を書かない(既存の「順位が変わるなら rank も書く」と同じ)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers/
git commit -m "feat(transfers): 別のワークスペースへ移した作業項目を、移動先の並びの末尾に続ける(F2a)"
```

---

### Task 4: バックログの読み取りと並べ替えの保存(`app/planning`)

**Files:**
- Create: `internal/app/planning/backlog.go`、`internal/app/planning/place.go`
- Test: `internal/app/planning/backlog_test.go`、`place_test.go`

**Interfaces:**
- Consumes: `planning.BacklogFrames`・`planning.Placement`・`planning.FrameEnd`・`planning.AssignableSprint`(Task 1・既存)、`items`・`planOf`・`row`・`assign`・`SetSprint`・`sprintErr`・`refValue`(既存)
- Produces:
  - `type BacklogRow struct { Row; Priority change.ID; Project project.ID }`
  - `type BacklogFrame struct { Sprint *planning.Sprint; Total planning.Totals; Rows []BacklogRow }`
  - `func (s Service) Backlog(ctx context.Context, store port.StoreID, ws workspace.ID) ([]BacklogFrame, error)` — ワークスペースがないか削除済みなら `port.ErrNotFound`
  - `func (s Service) PlaceItem(ctx context.Context, store port.StoreID, id item.ID, sprint *change.ID, anchor *item.ID, after bool) error` — F2a-3・F2a-4

- [ ] **Step 1: 失敗するテストを書く**

- `TestBacklog`: Task 1 の `TestBacklogFrames` と同じ材料で、枠が 4 つ、行に表示用 ID・優先度・プロジェクトが入る。
- `TestPlaceItemReorder`: バックログの枠の A・B・C で、C を A の直前へ → 書いた行は `set rank` 1 行、Summary「HH-3 の順位を変更」。
- `TestPlaceItemAcrossFrames`: バックログの B を S1 の X の直前へ → 1 回の保存で `set sprint`(S1)と `set rank`、Summary「HH-2 の順位を変更」。
- `TestPlaceItemEmptyArea`: `anchor` なしで別の枠なら `SetSprint` と同じ行と Summary「HH-2 の スプリント を Sprint 1 に変更」。同じ枠で末尾でなければ `rank.After(自分を除いた FrameEnd)` と「順位を変更」、末尾なら何も保存しない。
- `TestPlaceItemSamePosition`: 基準が同じ枠の次の行(直前)・自分自身なら何も保存しない。
- `TestPlaceItemTies`(Review Focus 1): Task 1 の `TestPlacementTies` の並びで、保存のあとの `WorkOrder` が期待どおり。
- `TestPlaceItemClosedSprint`(Review Focus 4): 完了済みのスプリントへは `refusal` の「完了したスプリントには入れられません」。Epic は `port.ErrInvalidOperation`。基準が別の枠・削除済みなら `port.ErrNotFound`。
- `TestPlaceItemRevives`: 別の画面で削除された(root: true)アイテムを並べ替えると、同じ保存で削除を取り消す(`assign` のテストと同じ形)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/planning/ -run 'TestBacklog|TestPlaceItem'`
Expected: FAIL

- [ ] **Step 3: `backlog.go` と `place.go` を書く**

`PlaceItem` は、保存の前(Summary を決める)と保存の中(読み直して行を作る)で同じ判定関数を呼ぶ(`assign` と同じ形)。同じ位置は `errSameRef` で何も保存しない。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/planning/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/planning/backlog.go internal/app/planning/place.go internal/app/planning/backlog_test.go internal/app/planning/place_test.go
git commit -m "feat(planning): バックログの枠を読み、行の位置とスプリントを 1 回で保存する(F2a)"
```

---

### Task 5: API と api パッケージの口

**Files:**
- Create: `internal/adapter/httpapi/planning/backlog.go`
- Modify: `internal/adapter/httpapi/planning/planning.go`(登録)、`dto.go`(必要なら)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(生成)、`web/packages/core/src/model/planning.ts`、`web/packages/core/src/port/planning.ts`、`web/packages/api/src/planning.ts`
- Test: `internal/adapter/httpapi/planning/planning_test.go`(追記)、`web/packages/api/src/planning.test.ts`(追記)

**Interfaces:**
- Consumes: `Service.Backlog`・`Service.PlaceItem`(Task 4)
- Produces:
  - `GET /api/stores/{store}/workspaces/{workspace}/backlog`(operationId `get-backlog`)→ `{ frames: [{ sprint: Sprint | null, total: Totals, rows: BacklogRow[] }] }`
  - `POST /api/stores/{store}/items/{item}/plan-position`(operationId `place-item`、204)本文 `{ sprint: string | null, anchor: string | null, after: boolean }`
  - core: `interface BacklogRow extends PlanRow { priority: string; project: string }`、`interface BacklogFrame { sprint: Sprint | null; total: Totals; rows: BacklogRow[] }`
  - `PlanningPort.backlog(storeId, workspaceId): Promise<BacklogFrame[]>`、`PlanningPort.placeItem(storeId, itemId, sprintId: string | null, anchorId: string | null, after: boolean): Promise<void>`

- [ ] **Step 1: 失敗するテストを書く**

- Go: `get-backlog` が枠を返す。ないワークスペースは 404。`place-item` の断りは 422 で本文に「完了したスプリントには入れられません」、基準がないと 404(既存の `apierr` の対応どおり)。
- api: `backlog` が `null` の一覧を空の配列にする。`placeItem` が本文 `{sprint, anchor, after}` を送る。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/planning/ && pnpm -C web --filter @hedgehug/api test`
Expected: FAIL

- [ ] **Step 3: 登録し、`make generate` で生成物を作り直し、api の口を書く**

- [ ] **Step 4: 通ることを確かめる**

Run: `make generate && go test ./internal/adapter/httpapi/... && pnpm -C web --filter @hedgehug/api test`
Expected: PASS、`git status` で生成物の差分がコミット対象に入っている

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/httpapi/planning api/openapi.yaml web/packages
git commit -m "feat(api): バックログの読み取りと行の位置の保存の API を足す(F2a)"
```

---

### Task 6: バックログの状態と保存(core `backlog.ts`・`backlogDrop.ts`)

**Files:**
- Create: `web/packages/core/src/features/planning/backlog.ts`、`backlogDrop.ts`
- Modify: `web/packages/core/src/index.ts`(公開)
- Test: `backlog.test.ts`、`backlogDrop.test.ts`(同じフォルダ。偽物の口は `fixtures.test-util.ts` に足す)

**Interfaces:**
- Consumes: `PlanningPort.backlog`・`placeItem`・`setItemSprint`(Task 5)、`ItemsPort.setProperty`、`sprintChoices`(F1)、`sprintSaveFailed`、`Refused`、`Toasts`
- Produces:
  - `type Drop = { sprint: string | null; anchor: string | null; after: boolean }`
  - `function dropPlacement(frames: BacklogFrame[], itemId: string, target: { frame: string | null; row: string | null }): Drop | null` — ドロップ先(`frame` はスプリントの ID、バックログは null。`row` は行の内部 ID、空いた部分は null)から保存の中身。同じ位置(自分自身、同じ枠の自分の次の行、同じ枠の空いた部分で自分が末尾)なら null
  - `function moveUp(frames, itemId): Drop | null`・`moveDown(frames, itemId): Drop | null` — 同じ枠の前の行の直前・次の行の直後。端なら null
  - `function goneText(display: string, category: "done" | "canceled"): string` — 「{ID} は完了のため、バックログには表示されません」
  - `interface BacklogState { storeId; workspaceId; frames: BacklogFrame[]; settings: WorkspaceSettings; members: Member[] }`
  - `function createBacklog(deps: { port: Pick<PlanningPort, "backlog" | "placeItem" | "setItemSprint">; items: Pick<ItemsPort, "setProperty">; workspaces; listMembers; toasts: Toasts; openDetail(storeId: string, itemId: string): void }): Backlog`
  - `interface Backlog { state; open(storeId, workspaceId); reload(); close(); setStatus(row, statusId): Promise<void>; setPoints(row, text): Promise<boolean>; setSprint(row, sprintId: string): Promise<void>; place(row, drop: Drop): Promise<void>; openDetail(row): void; noticeDetail(storeId: string, item: Item): void }`

- [ ] **Step 1: 失敗するテストを書く**

- `dropPlacement`: バックログの枠 A・B・C で、C を A の行へ → `{sprint: null, anchor: A, after: false}`。A を B の行へ → null(同じ位置)。A を S1 の空いた部分へ → `{sprint: S1, anchor: null, after: false}`。C をバックログの空いた部分へ → null(末尾)、A なら `{sprint: null, anchor: null, after: false}`。
- `moveUp`・`moveDown`: 先頭の `moveUp` と末尾の `moveDown` は null。B の `moveUp` は `{sprint: null, anchor: A, after: false}`、B の `moveDown` は `{sprint: null, anchor: C, after: true}`。
- `createBacklog`: `open` で 3 つを読み、別のワークスペースへ `open` すると state を消してから読む(`createSprints` と同じ)。
- `setStatus`(Review Focus 3 の(1)): バックログの枠の行を Done(分類が完了)にすると、`setProperty(… "status" …)` を呼び、トースト「HH-3 は完了のため、バックログには表示されません」。スプリントの枠の行では出さない。断られたらその文、ほかの失敗は「保存できませんでした」をエラーのトーストに出し、どちらも `reload`。
- `setPoints`: `"3"` を送る。同じ値は送らない。断り「ポイントは 0 以上の数で入力してください」はエラーのトーストで false。
- `setSprint`(Review Focus 3): スプリントの枠の Done の行で「バックログ」(`""`)を選ぶと `setItemSprint(…, null)` とトースト「… は完了のため、…」。同じ値は送らない。
- `place`: S1 の中止の行をバックログの枠へドロップすると `placeItem` とトースト「HH-5 は中止のため、バックログには表示されません」。断り「完了したスプリントには入れられません」(Review Focus 4)はエラーのトーストと `reload`。
- `noticeDetail`(F2a-10 の(2)): `openDetail(row)` で開いたアイテムが、保存の応答で開いている枠のどのスプリントにも入っておらず完了なら、トースト。別のアイテム、スプリントに入っているアイテム、`close` のあとは出さない。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- backlog`
Expected: FAIL

- [ ] **Step 3: 2 つのファイルを書く**

読み直しが重なったときは最後に始めたものだけを使う(`createSprints` と同じ)。分類は `settings.statuses` の `category` で決め、ないものは未着手。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test && pnpm -C web --filter @hedgehug/core check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src
git commit -m "feat(core): バックログの状態・保存・ドロップ先・消えるときのトーストを足す(F2a)"
```

---

### Task 7: 詳細パネルからの知らせ(`detail.ts`)

**Files:**
- Modify: `web/packages/core/src/features/items/detail.ts`(`DetailDeps`・`setProperty`)
- Test: `web/packages/core/src/features/items/detail.test.ts`(追記)

**Interfaces:**
- Consumes: `ItemsPort.setProperty` の応答の `Item`
- Produces: `DetailDeps.onPropertySaved?(storeId: string, item: Item): void`

- [ ] **Step 1: 失敗するテストを書く**

`setProperty(s1.id, "status", "st2")` が成功すると、`onPropertySaved` が Store の ID と応答のアイテムで 1 回呼ばれる。断られたときは呼ばれない。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- detail.test`
Expected: FAIL

- [ ] **Step 3: `setProperty` で応答を受け取り、`saved` の前に `deps.onPropertySaved?.(v.store.id, item)` を呼ぶ**

`detail.ts` は 300 行を超えないこと。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/features/items
git commit -m "feat(core): 詳細パネルのプロパティの保存を知らせる口を足す(F2a)"
```

---

### Task 8: バックログのタブの画面

**Files:**
- Create: `web/app/src/screens/planning/BacklogTab.svelte`、`BacklogFrame.svelte`、`BacklogRow.svelte`、`PointsInput.svelte`、`web/packages/ui/src/patterns/dragRows.ts`・`dragRows.test.ts`(`@hedgehug/app` にはテストの仕組みがないので、jsdom のテストがある ui に置き、`index.ts` から公開する)
- Modify: `web/packages/core/src/features/router/route.ts`(`WorkspaceTab` に `backlog`、`workspaceTabs` の 2 番目)、`route.test.ts`、`web/app/src/screens/workspace/WorkspaceScreen.svelte`(タブ・描画・ダイアログを出すタブ)、`StartSprintDialog.svelte`・`SprintDialogs.svelte`(`card` の型、F2a-15)、`web/app/src/planning.ts`(`backlog` を作る)、`web/app/src/wire.ts`(`reflect` に `backlog.reload()`、`detail` の `onPropertySaved` を `backlog.noticeDetail` へ)

**Interfaces:**
- Consumes: `createBacklog`・`dropPlacement`・`moveUp`・`moveDown`(Task 6)、`StatusButton`(`screens/item`)、`sprintChoices`・`periodText`・`pt`・`startDisabledReason`・`readOnlyNote`、`createDialog.open`
- Produces:
  - `function dragRows(handle: HTMLElement, opts: { itemId: string; disabled: boolean; ondrop(target: { frame: string | null; row: string | null }): void; onhover(target | null): void })`(Svelte の action。F2a-12)
  - ルート `/s/{store}/w/{prefix}/backlog`

- [ ] **Step 1: 失敗するテストを書く**

- `route.test.ts`: `/s/a/w/HH/backlog` が `tab: "backlog"`、タブの並びが `overview, backlog, sprints, releases, reports, settings`。
- `dragRows.test.ts`(jsdom): `pointerdown` → `pointermove`(`elementFromPoint` を差し替えて `data-row="r2"`・`data-frame="s1"` の要素)→ `pointerup` で `ondrop({frame: "s1", row: "r2"})`。枠の空いた部分(`data-frame=""` で行なし)なら `{frame: null, row: null}`。Esc と `pointercancel` では呼ばない。`disabled` なら `pointerdown` で始まらない(Review Focus 5)。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- route && pnpm -C web --filter @hedgehug/ui test -- dragRows`
Expected: FAIL

- [ ] **Step 3: 画面を書く**

- `BacklogTab`: `backlog.open(sid, wid)` を `$effect` で開き、閉じるときに `close`(`SprintsTab` と同じく `sid`・`wid` の `$derived` で開き直さない)。スプリントの枠、続けてバックログの枠を `BacklogFrame` で並べる。文言は F2a-14。
- `BacklogFrame`: `data-frame` を持つ。見出し、計画中の「開始」、空の文。ドラッグ中は、ドロップ先の行の上に線、枠の空いた部分なら枠の下に線を出す。
- `BacklogRow`: `data-row` を持ち、05 の 34 行の並び(チェックを除く): 持ち手、種類、ID、タイトル(`backlog.openDetail`)、優先度(名前。ないか削除された値は `deletedValue`)、`StatusButton`、`PointsInput`、担当者(`PlanItems` と同じ出し方)、スプリントのセレクト、「上へ」「下へ」。閲覧のみは F2a-16。
- `PointsInput`: 確定で `backlog.setPoints`、false なら保存済みの値に戻す。
- `WorkspaceScreen`: タブに `{ id: "backlog", label: "バックログ" }` を 2 番目に足し、`BacklogTab` に `oncreate`(スプリントの作成のダイアログ)・`onstart`(開始のダイアログ)を渡す。`shown` の条件を `tab === "sprints" || tab === "backlog"` にする。

各 Svelte は 200 行以内。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/features/router web/packages/ui/src web/app/src
git commit -m "feat(web): バックログのタブを足す(枠・行の操作・ドラッグ・開始と作成)(F2a)"
```

---

### Task 9: e2e

**Files:**
- Create: `web/e2e/backlog.spec.ts`

**Interfaces:**
- Consumes: `createWorkspace`・`prefixOf`・`projectWithEpic`・`storyUnderEpic`・`tokenUrl`・`unique`(`web/e2e/helpers.ts`)

- [ ] **Step 1: テストを書く**

`creates, reorders and plans items on the backlog`: ワークスペースを作り、Epic の下に Story を 3 件(接頭辞-2・-3・-4)作る。`/s/e2e/w/{prefix}/backlog` を開く。
- バックログの枠の見出しに「3 件 · 0 pt · スプリントに入っていない未完了の Story・Task・Bug」。
- 「スプリントを作成」→ 既定の名前で作成 → 枠「Sprint 1」に「計画中」「期間未定」「0 件 · 0 pt」と「バックログからここにドラッグ、または行の「スプリント」で選択」。
- -4 の「上へ」→ 並びが -2・-4・-3。
- -2 の行のポイントに 3 を入れて Enter → 見出しが「3 件 · 3 pt …」。
- -3 の行のスプリントで Sprint 1 を選ぶ → Sprint 1 の枠に -3。
- -2 の持ち手を、マウスで Sprint 1 の -3 の行へドラッグ(`page.mouse.down/move/up`)→ Sprint 1 の枠が -2・-3。
- Sprint 1 の -2 のステータスを Done にする → トーストは出ない(スプリントの枠)。続けて -2 のスプリントで「バックログ」を選ぶ → トースト「{prefix}-2 は完了のため、バックログには表示されません」、行がどの枠にもない(Review Focus 3)。
- 「開始」→ 開始のダイアログ → 開始 → 枠が「実行中」。
- 「作成」→ 作成のダイアログの種類が Story。作るとバックログの枠の末尾に入る(Review Focus 2)。

- [ ] **Step 2: 流して通ることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test backlog.spec.ts`
Expected: PASS

- [ ] **Step 3: コミット**

```bash
git add web/e2e/backlog.spec.ts
git commit -m "test(e2e): バックログの並べ替えとスプリントへの出し入れを通しで確かめる(F2a)"
```

---

### Task 10: 計画の記録

**Files:**
- Modify: `docs/superpowers/plans/2026-11-14-hedgehug-track-f1.md`(F1-21 の「作成の rank は F2 が決める」の行に、F2a-5 で決めたことを 1 行足す)、`docs/superpowers/plans/2026-11-14-hedgehug-track-a5a.md`(A5a-3 の行に、作業項目の順位は F2a-6 で移動先の並びの末尾に続けることを 1 行足す)、この計画の末尾に「F2b に渡すもの」

- [ ] **Step 1: 書く**

「F2b に渡すもの」: 行の選択のチェック(05 の 34 行の先頭)、選択バー・一括変更・一括削除(04)を共通の部品として作り `BacklogRow` と E のテーブルで使う、一括変更のトーストの末尾の「。{K} 件は完了のため、バックログには表示されません」(`goneText` と同じ分類の判定)、コマンドパレットの `selection.provide(2, …)`(A5c が develop に入っていれば F2b が足し、入っていなければ A5c が足す)。

- [ ] **Step 2: コミット**

```bash
git add docs/superpowers/plans
git commit -m "docs(plans): F2a で決めた並び順を F1・A5a の計画に書き、F2b に渡すものを残す(F2a)"
```

---

### Task 11: 順位の末尾の値を短く保つ(`rank.After` を段と数えで作る)

最終レビューで見つけ、ユーザーの決定(2026-11-14)で F2a に入れた。F2a-5・F2a-6 で作業項目の順位がワークスペース全体の末尾に続くようになり、今の `After`(最後の文字を進め、`z` なら `V` を足す)では順位の文字列が約 31 件ごとに 1 文字伸びる。

**Files:**
- Modify: `internal/domain/rank/rank.go`(`After`)
- Test: `internal/domain/rank/rank_test.go`

**Interfaces:**
- Produces: `func After(s string) string` — 署名は変えない。値を「先頭の `z` の並び(段) + 数え(先頭が `z` でない、桁数を変えない 62 進数)」と見て、数えを 1 つ進める(桁数は変えない。`'0'` で終わるなら `'1'` にする。`"V"` → `"W"`、`"Vz"` → `"W1"`、`"azz"` → `"b01"`)。数えが尽きたら(先頭が `z` になる)、`z` を 1 つ増やして 1 桁広い数えの最初の値にする(`"y"` → `"z01"`、`"zy"` → `"zz01"`)。`z` だけの値は `s + First`(`"z"` → `"zV"`)。`digits` にない文字(手での書き換え)を含む値は、繰り上げがその文字に届いたときだけ `s + First` にする(`"V_"` → `"V_V"`。届かなければ数えを進めるだけで、`"_V"` → `"_W"`)。空なら `First`。結果は常に `s` より大きく、`'0'` で終わらない。前の作りが書いた値(`"zV"`・`"zzzW"` など)からも続けられる。末尾の繰り上げだけ(`"Vz"` → `"W"`)では、`z` だけの並びの後ろに続けるときに約 31 件ごとに 1 文字伸びるままなので、この形にした(62 倍ごとに 2 文字)

- [ ] **Step 1: 失敗するテストを書く**

- `TestAfter`: 上の例の値。`TestAfterFromAnyValue`: `"z"`・`"zz"`・`"zV"`・`"zzzW"`・`"Vz"`・`"Az"`・`"azz"`・長い `z` の並びなどから、`s` より大きく `'0'` で終わらない値を作る。
- `TestAfterStaysShort`: `""` から `After` を 10,000 回続けた値の長さが 6 以下、`"zzzzzzzzzz"` から 1,000 回続けた値の長さが 14 以下で、各回の値が前の値より大きい。
- `TestBetweenWithCarry`: `Between("VAz", "W")` が `"VAz"` より大きく `"W"` より小さい(`Between` の内側の `After(rest)` も新しい作りで進める)。
- `TestBetweenAfterProperty`: 固定の種の乱数の値 `a` で、`Between(a, After(a))` が `a` と `After(a)` の間にある。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/rank/`
Expected: FAIL

- [ ] **Step 3: `After` を書き直す**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./...`
Expected: PASS(順位の値をそのまま見ている既存のテストがあれば、新しい値に直し、差分に「Task 11」と書く)

- [ ] **Step 5: コミット**

---

### Task 12: ツリーの「上へ移動」「下へ移動」で、同じ順位の兄弟があっても動かす

最終レビューで見つけ、ユーザーの決定(2026-11-14)で F2a に入れた。F1-11 の `SetSprint` は順位を `rank.After(枠の末尾)` にするので、兄弟と同じ順位がふつうに生まれる。ツリーの `items.Reorder` の `gap` は前後の順位が同じだと何もしない(例: Epic の下の Story A(V)・B(W)・C を同じスプリントに入れて C が W になると、C の「上へ移動」が動かない)。

**Files:**
- Modify: `internal/domain/rank/rank.go`(`Around` を足す)、`internal/domain/planning/backlog.go`(`Placement` が `Around` を使う)、`internal/app/items/reorder.go`(`gap` が `Around` を使う)
- Test: `internal/domain/rank/rank_test.go`、`internal/app/items/reorder_test.go`

**Interfaces:**
- Produces: `func Around(sorted []string, j int, after bool) string` — 昇順の順位の列 `sorted` の `j` 番目の直前(`after` なら直後)に入る値。直前なら `hi = sorted[j]`、`lo` は `j` より前で `hi` より小さい最も近い値(なければ空)。直後なら `lo = sorted[j]`、`hi` は `j` より後ろで `lo` より大きい最も近い値(なければ `After(lo)`)。F2a-2 と同じ規則
- `planning.Placement` と `items.gap` は、この関数で値を作る(同じ規則を 2 か所に書かない)。`gap` の「すでにその位置なら何もしない」は今のまま

- [ ] **Step 1: 失敗するテストを書く**

- `TestAround`: `["V","W","W","X"]` の 2 番目の直前 → `Between("V","W")`、1 番目の直後 → `Between("W","X")`、3 番目の直後 → `After("X")`。
- `TestReorderTies`: 兄弟 A(V)・B(W、作成が早い)・C(W)。C を B の直前へ → C が A と B の間に入り、並びが A・C・B になる。保存は `set rank` の 1 行。
- 既存の `TestPlacement`・`TestPlacementTies` はそのまま通る。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/rank/ ./internal/app/items/ -run 'Around|Reorder'`
Expected: FAIL

- [ ] **Step 3: `Around` を書き、`Placement` と `gap` を置き換える**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./...` と `golangci-lint run ./...`
Expected: PASS

- [ ] **Step 5: コミット**

---

## 最後に

- 最終レビュー(superpowers:requesting-code-review)で、ブランチ全体を仕様 05「バックログ」と照らす。
- PR は develop 向けに 1 本。本文に、計画から変えたことと、F2b に渡すものを書く。

---

## F2b に渡すもの

- 行の選択のチェック(05 の 34 行の先頭)。F2b1 で足した。
- 選択バー・一括変更・一括削除(04)。共通の部品として作り、`BacklogRow` と E のテーブルで使う。選択バーと一括削除は F2b1 で足した。バーの「一括変更…」と一括変更は F2b2 に回した。
- 一括変更のトーストの末尾の「。{K} 件は完了のため、バックログには表示されません」。`goneText` と同じ分類で判定する。F2b2 に回した。
- コマンドパレットの `selection.provide(2, …)`。A5c が develop に入っていれば F2b が足し、入っていなければ A5c が足す。F2b1 で足した。

## 実装で計画から変えたこと

1. `PlaceItem`: 完了したスプリントに残っている作業項目をバックログの枠の行に置いたときも、`set sprint` に null を書く(仕様 05 の 48 行「バックログの枠なら、スプリントから外れる」)。置き先は表示中の枠(実行中・計画中のスプリントかバックログ)に限り、同じ枠の中の移動では、動かすアイテムがその枠の行であることを求め、満たさなければ見つからないとする。
2. 詳細パネルの「表示されなくなった」トースト: 開いているアイテムが今表示されなくなっているかを覚え、表示されなくなる変化のたびに出す(Done にして戻してまた Done にすると 2 回出る。表示されないまま状態以外を保存しても繰り返さない)。
3. スプリントのダイアログ(作成・開始): 開いたタブを覚え、そのタブでだけ表示する(「スプリントかバックログ」ではなく)。一方のタブで開いた開始ダイアログが、タブを切り替えたあとに残らないようにするため。
4. バックログの再読み込みと詳細パネルの通知は、`wire.ts` の `reflect` を直接変えず、`planning.ts` の `reloads` と `reloads.ts` の `propertySaved` の集合でつなぐ。`wire.ts` から `planning.ts` を import すると初期化が循環するため。
5. ポイントの入力: 打ち換えずに離れたときは保存せず、最後に保存された値に戻す(詳細パネルの `PropertyField` と同じ)。同期で届いた値を上書きしないため。
6. ドラッグ: ドラッグ中の Esc はドラッグの取り消しだけにし、ほかへ伝えない(伝播を止める)。
7. 狭い幅: バックログの行は、操作の部品をタイトルの下に折り返し、スマートフォンの幅でもタブが横にスクロールしないようにする。
8. Task 9 の e2e: スプリントの作成ダイアログは初めの日付を入れる(F1-7)ので、e2e は日付を消してから「期間未定」を確かめる。
