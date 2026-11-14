# hedgehug トラック C5a(計画の同期の修復)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期で起きるスプリント・リリースのずれ((6) 実行中のスプリントが 2 つ、(7) 完了したスプリントに入った未完了のアイテム、(8) 存在しないスプリント・リリースの参照)を同期の修復で直し、完全に削除したスプリント・リリースの行を合わせで取り除き、それぞれをあなた宛ての「同期の修復」の通知として 1 回だけ出す。

**Architecture:** 直す規則は `internal/domain/repair/plan.go`(合わせた結果と `repair.Facts` だけを読む純粋な関数)。修復の段は `internal/app/repairing/plan.go` に 3 つ足し、`steps` の (5) のあとに並べる(06 `:350` の順)。完全に削除したスプリント・リリースの行は、合わせ(`mergeTrees`)の最後に `filemerge.DropPurgedPlan` で取り除く。合わせで取り除いた行のうち片側にしかないものは、`saves` が `ReceivedSave.DroppedPlan` として判定に渡す。通知は C4b2 の仕組み(`noticing.repairs` の振り分け・`AddOnce`・鍵 `repair:{種類}:…`・`Section`)に足す。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite)。web は変えない。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:245-249`((6))、`:326-328`(完全な削除と重なったとき)、`:334-342`((7)・(8))、`:344-350`(修復の決まり・順)、`:366`・`:377-383`(通知・判定の単位)、`:455`(開く先)、`:514-536`(アクティビティの文)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/05-planning-reports.md` `:54-56`・`:75-81`・`:147-175`
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:183`(合わせで `purged.jsonl` にあるスプリント・リリースの行を取り除く)、`:193-218`(同期の修復の表)、`:241-245`(`purged.jsonl` の形)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c4b2.md`(C4b2-2〜C4b2-12。この計画はその上に足す)、`2026-11-14-hedgehug-track-f1.md`(F1-1〜F1-5、F1-24)、`2026-11-12-hedgehug-track-c.md` の C5 の行と決定 9

## Global Constraints

土台・トラック C・C4a・C4b1・C4b2 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個(`internal/archtest/size.go`)。`internal/infra/gitstore`(直下)・`internal/app/port`・`internal/app/apptest` は 20 個で満杯なのでファイルを足さない。足すのは `domain/repair/plan.go`・`app/repairing/plan.go`・`infra/gitstore/filemerge/plan.go`・`infra/gitstore/saves/plan.go`・`app/noticing/repair_plan.go`・`app/noticing/repair_plan_gone.go` だけ。
- 口は git の言葉を使わない(`port` の型の名前とコメントに「コミット」「ブランチ」「マージ」を書かない)。
- app の機能どうしは import しない(`noticing` は `repairing`・`planning`(app)を import しない。domain の `planning` は使ってよい)。
- 修復は 1 件を 1 回の保存にする(C3a-18)。実行者は `change.ActorSync`。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(`go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 修復の保存の件名(アクティビティの文。06 `:514-531`):
  - (6) 「スプリント「{名前}」を計画中に戻す」
  - (7) 「{ID} をバックログに戻す」
  - (8) 「{ID} をバックログに戻す(スプリントが完全に削除)」「{ID} のリリースを外す(リリースが完全に削除)」。ワークスペース違いは「{ID} をバックログに戻す(スプリントがワークスペースにない)」「{ID} のリリースを外す(リリースがワークスペースにない)」(C5a-5)
- 通知の本文({…} はそのまま置き換える。名前の改行と続いた空白は 1 つの空白にする):
  - (6) 「実行中のスプリントが 2 つになったため、「{スプリント名}」を計画中に戻しました」
  - (7) 「スプリント「{名前}」は完了しているため、{ID} をバックログに戻しました」
  - (8) 「スプリント「{名前}」は完全に削除されたため、{ID} をバックログに戻しました」「リリース「{名前}」は完全に削除されたため、{ID} のリリースを外しました」「スプリント「{名前}」は {ID} のワークスペースにないため、{ID} をバックログに戻しました」「リリース「{名前}」は {ID} のワークスペースにないため、{ID} のリリースを外しました」。名前が読めなければ「削除されたスプリント」「削除されたリリース」
  - 合わせで取り除いたスプリント・リリースの行: 「「{名前}」は完全に削除されたため、あなたの変更は反映されませんでした」(C4b2-9 の呼び方)
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C5a-1 | 範囲: 修復 (6)・(7)・(8)、`purged.jsonl` にあるスプリント・リリースの行の合わせでの取り除き、その通知 4 種((6)・(7)・(8)・取り除いた行の「反映されませんでした」)。プロジェクト別のルールの行の取り除き(保存形式 `:183` の残り)は、修復 (9) と同じ持ち主の規則なので C5b。(9)〜(10) は C5b、(11) は C5c、時計のずれは C5d | ユーザーの決定(2026-11-15、C5 を 4 本に分ける) |
| C5a-2 | 修復の順: `steps` の (5)(`matchProjects`)のあとに `stopExtraActive`((6))→ `backlogClosed`((7))→ `clearDanglingPlan`((8))。保存形式の表の番号と「順の指定なし」の行は 06 `:350` に合わせて書き直し、古い番号を参照している計画(トラック B の計画 `:17`、B5a の計画 `:59`)も直す(Task 8) | 06 `:350` が (1)〜(11) の順と理由((7) は実行中・完了の区別に使うので (6) が先)を明記している。保存形式の表は同じ修復を「順の指定なし」と (6)〜(9) の別の番号で書いていて、06 と食い違う |
| C5a-3 | (6): 削除済みでないスプリントのうち状態が実行中のものが 2 つ以上なら、開始日(`Start` の文字列)、開始の行(`sprints/<ID>/state` の、いまの値の行。`change.Current`)の `at`、その行の `by`(小文字)、その行の ID の順に比べ、先頭以外を `set sprints/<ID>/state` で計画中にする(`planning.SprintDrafts`)。スプリント 1 つを 1 回の保存にする。削除済みのスプリントは数えない | 06 `:247`。実行中は削除できず(05 `:76`)、削除と開始が重なれば削除が取り消される(C3a)ので、削除済みの実行中は残らない |
| C5a-4 | (7): ワークスペースの作業項目(`planning.IsWork`)で削除済みでなく、ステータスの分類が未着手か進行中で、スプリントが完了(`State == Closed`。削除済みのスプリントも含む)のもののうち、完了の op の行(ワークスペースのログで `Op == Closing.Op` の行)のどれかと、アイテムのスプリントの欄かステータスの欄のいまの値の行のどれかが、`Facts` の別々の側にあるものを `set sprint null` にする。「欄のいまの値の行」は、その欄の `set` の行があれば `change.Current(log, change.Set, 欄, …)` の行、なければ作成の行(作成のときにスプリントを選んだアイテム)。完了したあとに(完了を受け取ってから)再オープンしたものは直さない。06 `:336` と 05 `:171` にこの対象を書く(Task 8) | ユーザーの決定(2026-11-15)。06 `:336`「同期で入ってきたとき」。削除済みは存在するものとして扱う(06 `:340`) |
| C5a-5 | (8): 削除済みでないアイテムの `sprint`・`release` が、アイテムのワークスペースの `planning.FromLog` のスプリント・リリース(削除済みを含む)にないものを、欄ごとに 1 回の保存で `set sprint null`・`set release null` にする。完全に削除された・定義がない・別のワークスペースのもの、のどれも同じ。件名は、どれかのワークスペースの計画にあれば「ワークスペースにない」、なければ「完全に削除」の文。06 の表はスプリントのワークスペース違いとリリースのワークスペース違いの文と本文を持たないので、同じ形で足す(Task 8 で 06 に書く) | 06 `:338-342`。完全に削除したものは行が消える(合わせでも C5a-7 で消える)ので、計画にないことで見分けられる |
| C5a-6 | `purged.jsonl` のスプリント・リリースの行の `display` は名前にする(G に求める)。(8) の通知の名前は、`purged.jsonl` にあればその `display`、どれかのワークスペースの計画にあればその名前、どちらもなければ「削除されたスプリント」「削除されたリリース」 | 06 `:342`(名前は共通の祖先から読む)。完全に削除したときに名前を残すのは `purged.jsonl` だけで、共通の祖先と同じ名前になる |
| C5a-7 | 合わせ(`mergeTrees`)の最後に、`purged.jsonl` の `entity` が `sprint`・`release` の行の target について、すべての `workspaces/<ID>/log.jsonl` から `planning.IsPlanRow` で、`planning.PlanTarget` がその target の行を取り除く。そのあとで YAML を作り直す | 保存形式 `:183`、トラック C の決定 9 |
| C5a-8 | 判定に渡す合わせの条件(C4b2-2 の (c))に、C5a-7 で取り除いた行のうち片側の共通の祖先にない行があるもの、を足す。`ReceivedSave.DroppedPlan []DroppedPlan`(`Workspace`・`Target`・`Log`(その側のその target の行)・`New`(そのうち共通の祖先にない行))。両側の分を持つ | トラック C の C5 の行(取り除いた行は `Removed` に出ないので、通知が作られない) |
| C5a-9 | 通知の宛先と鍵: (6) は開始の行の `by`、鍵 `repair:active:{スプリントの ID}:{開始の行の ID}`。(7)・(8) は修復の直前のアイテムのログで、その欄(`sprint`・`release`)を最後に保存した人(作成の行を含む。C4b2-7 の `lastSaver`)、鍵 `repair:closed:{アイテムの内部 ID}:{その欄の行の ID}`・`repair:planref:{アイテムの内部 ID}:{sprint|release}:{その欄の行の ID}`。取り除いた行は `New` の人の行の `by`(1 人に 1 件)、鍵 `repair:purged:{target}` | C4b2-6・C4b2-7 と同じ作り方。欄の行の ID は、同じ人がもう一度入れて同じ修復が起きたときに 2 件目を出すため |
| C5a-10 | 修復の種類の見分け(C4b2-8 に足す): ワークスペースのログの `set sprints/<ID>/state`(from `active`、to `planned`)→ (6)。アイテムの `set sprint`・`set release`(to null)で、同じ保存に `place` の行がないもの → スプリントがアイテムのワークスペースの計画にあり完了なら (7)、ほかは (8)。(8) の理由は、`purged.jsonl` にあれば完全な削除、どれかのワークスペースの計画にあればワークスペース違い、なければ完全な削除 | 修復 (5) の別のワークスペースへの移動(C3b)も `set sprint null` を書くが、同じ保存に `place` の行がある |
| C5a-11 | 開く先: (6)・スプリントの取り除いた行は `Workspace` と `Section: "sprints"`、リリースの取り除いた行は `Section: "releases"`。(7)・(8) は `Item`(C4b2-10) | 06 `:455` |
| C5a-13 | C4c1(ブランチ feat/track-c4c、別の worktree)も `port.ReceivedSave` と `saves` の `pendingOf` に足す。C5a は `removed.go` の `pendingOf` には条件の呼び出しを 1 つ足すだけにし、中身は `saves/plan.go` に置く。develop にあとから入る方が合わせる | 2 本の PR が同じ所を変えるため |
| C5a-12 | e2e は足さない。2 台の場面は `cmd/hedgehug` の本物の gitstore と state.db で確かめる((6)・(7)・再オープン)。完全な削除は書く口がない(G)ので、gitstore のテストの手助け `purgePlanForTest` で確かめる | C4b2-14 と同じ |

## Review Focus

1. **完了のあとの再オープン:** Sprint 24(HH-7 が Done で入っている)を太郎さんが完了して同期し、花子さんがそれを同期で受け取ってから Done の HH-7 を In Progress に戻し、別の保存との合わせが起きても、HH-7 は Sprint 24 に残り、通知も出ない(Task 1 の `TestClosedSprintItemsNeedsConcurrency`、Task 7 の `TestRepairNoticeReopenStays`)。
2. **別のワークスペースへの移動の修復と混ぜない:** 修復 (5) の移動の保存の `set sprint null` からは、(8) の通知を作らない(Task 5 の `TestPlanRefSkipsPlaceRepair`)。
3. **2 台が同じ修復をした:** 2 台が同じスプリントを計画中に戻しても、受信箱は 1 行(Task 5 の `TestActiveOnceAcrossMachines`)。
4. **スプリントとリリースが両方ない:** 同じアイテムのスプリントとリリースが両方とも存在しないとき、保存は 2 つ、通知も 2 行(Task 2 の `TestDanglingBothFields`、Task 5 の `TestPlanRefBothFields`)。
5. **完全に削除したスプリントの名前を同時に変えた:** 合わせたあと、どちらのマシンのログにも settings の YAML にもそのスプリントの行が戻らず、名前を変えた人に 1 行(Task 3 の `TestSyncDropsPurgedSprintRows`、Task 6 の `TestDroppedPlanNotice`)。

---

### Task 1: 計画の修復の規則(`domain/repair/plan.go`)

**Files:**
- Modify: `internal/domain/planning/plan.go`(`PlanTarget`)、Test: `internal/domain/planning/plan_test.go`
- Create: `internal/domain/repair/plan.go`、Test: `internal/domain/repair/plan_test.go`

**Interfaces:**
- Produces(`planning`): `func PlanTarget(r change.Record) (change.ID, bool)` — `IsPlanRow` の行の、スプリント・リリースの内部 ID(`f` の 2 つ目の区切り。`sprints/<ID>`・`sprints/<ID>/<項目>`・`close_sprint` の `sprints/<ID>/close`)。ほかの行は false
- Produces(`repair`):
  - `func ExtraActive(wsLog []change.Record) []change.ID` — C5a-3 で計画中に戻すスプリント(内部 ID の順)
  - `type ItemLog struct { Item item.Item; Log []change.Record }`
  - `func ClosedSprintItems(wsLog []change.Record, st workspace.Settings, items []ItemLog, f Facts) []item.ID` — C5a-4(内部 ID の順)。`items` はワークスペースのアイテムだけを渡す
  - `type PlanRef struct { Item item.ID; Field string; Target change.ID }` — `Field` は `item.SprintField` か `item.ReleaseField`
  - `func DanglingPlanRefs(items []item.Item, plans map[workspace.ID]planning.Plan) []PlanRef` — C5a-5(アイテムの内部 ID、`sprint`・`release` の順)

- [ ] **Step 1: 失敗するテストを書く**
  - `TestPlanTarget`(表): `set sprints/S1/name` → S1、`delete sprints/S1` → S1、`close_sprint` `sprints/S1/close` → S1、`set releases/R1/state` → R1、アイテムの `set sprint` → false
  - `TestExtraActive`: 実行中 S1(開始日 2026-11-16)・S2(2026-11-17)→ [S2]。開始日が同じなら開始の行の `at` が早い方を残す。`at` も同じなら `by` を小文字で比べ(`Hanako` と `taro` → `Hanako` の方を残す)、それも同じなら行の ID。実行中が 1 つ → 空。削除済みの実行中は数えない
  - `TestClosedSprintItems`: S1 を完了した保存の行(`close_sprint`・`set state`)が `Ours`、HH-9(Task・分類が未着手)の `set sprint S1` が `Theirs` → [HH-9]。分類が完了 → 空。Epic → 空。削除済み → 空。スプリントが計画中 → 空
  - `TestClosedSprintItemsNeedsConcurrency`: 完了の行も HH-7 のスプリントの行も `Facts` にない(HH-7 のステータスの行だけが `Ours`)→ 空。完了の行が `Ours`、HH-7 のステータスを In Progress に戻す行が `Theirs` → [HH-7]。完了の行と HH-9 のスプリントの行が両方 `Ours` → 空。完了の行が `Ours`、スプリント S1 を選んで作った HH-10 の作成の行(`set sprint` の行はない)が `Theirs` → [HH-10]
  - `TestDanglingPlanRefs`: ワークスペース W1 の計画にない S9 を指す HH-1 → `{HH-1, sprint, S9}`。W1 の削除済みのスプリントを指す → 空。W2 の S2 を指す W1 の HH-3 → `{HH-3, sprint, S2}`。アーカイブのリリース → 空。削除済みのアイテム → 空
  - `TestDanglingBothFields`: スプリントもリリースもない HH-4 → `[{HH-4, sprint, …}, {HH-4, release, …}]`
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/planning/ ./internal/domain/repair/` Expected: FAIL(`undefined: PlanTarget` ほか)
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/domain/planning internal/domain/repair
git commit -m "feat(repair): スプリントの実行中の重なり・完了したスプリントの未完了・存在しない参照の規則を足す(C5a)"
```

---

### Task 2: 修復の段 (6)・(7)・(8)(`app/repairing/plan.go`)

**Files:**
- Create: `internal/app/repairing/plan.go`
- Modify: `internal/app/repairing/run.go`(`steps` の末尾に 3 つ)
- Test: `internal/app/repairing/plan_test.go`(既存の `withPlan` を使う)

**Interfaces:**
- Consumes: Task 1 の `repair.ExtraActive`・`repair.ClosedSprintItems`・`repair.DanglingPlanRefs`、`planning.SprintDrafts`、`port.RepairWriter`
- Produces: `func stopExtraActive(ctx, w) error`・`func backlogClosed(ctx, w) error`・`func clearDanglingPlan(ctx, w) error`(どれも `step`)。書く行は C5a-3〜C5a-5、件名は Global Constraints

- [ ] **Step 1: 失敗するテストを書く**(`newEnv`・`e.write`・`e.run` で、合わせの 2 つの側を作る)
  - `TestStopExtraActive`: taro が S1(開始日 2026-11-16)、hanako が S2(2026-11-17)を同時に開始 → 修復の保存 1 つ、件名「スプリント「Sprint 2」を計画中に戻す」、S2 が計画中、S1 は実行中。もう一度 `Run` → 何も書かない
  - `TestBacklogClosed`: taro が S1 を完了、hanako が同時に HH-2(未着手)を S1 に入れた → 件名「HH-2 をバックログに戻す」、HH-2 のスプリントが空、S1 の確定値は変わらない
  - `TestBacklogClosedKeepsReopened`: taro の完了を受け取ったあと(完了の行が共通の祖先にある)、hanako が Done の HH-2 を In Progress に戻した → 何も書かない
  - `TestClearDanglingSprint`: HH-2 が計画にない S9 を指す → 件名「HH-2 をバックログに戻す(スプリントが完全に削除)」。別のワークスペースのスプリントを指す → 「HH-2 をバックログに戻す(スプリントがワークスペースにない)」。リリースは「HH-2 のリリースを外す(リリースが完全に削除)」
  - `TestDanglingBothFields`: スプリントもリリースもない → 保存 2 つ
  - `TestPlanRepairOrder`: 1 回の `Run` で (6)・(7)・(8) がすべて要る場面(S1・S2 が同時に開始、S0 の完了と同時に HH-2 を S0 へ、HH-3 が計画にない S9)→ 修復の保存の件名が「スプリント「Sprint 2」を計画中に戻す」「HH-2 をバックログに戻す」「HH-3 をバックログに戻す(スプリントが完全に削除)」の順
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/repairing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/repairing/ ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/repairing
git commit -m "feat(repairing): 実行中のスプリントの重なり・完了したスプリントの未完了・存在しない参照を同期で直す(C5a)"
```

---

### Task 3: 完全に削除したスプリント・リリースの行を合わせで取り除く

**Files:**
- Create: `internal/infra/gitstore/filemerge/plan.go`、Test: `internal/infra/gitstore/filemerge/plan_test.go`
- Modify: `internal/infra/gitstore/sync_tree.go`(`mergeTrees` の `rebuildYAML` の前に呼ぶ)
- Test: `internal/infra/gitstore/sync_purge_test.go`(`purgePlanForTest` と場面)

**Interfaces:**
- Consumes: Task 1 の `planning.PlanTarget`
- Produces(`filemerge`): `func PurgedPlanTargets(purged []byte) map[string]bool`(`entity` が `sprint`・`release` の行の target。読めない行は飛ばす)、`func DropPurgedPlan(log []byte, targets map[string]bool) []byte`(`PlanTarget` が targets にある行を除く。ほかの行はバイト列のまま)
- テストの手助け(gitstore): `purgePlanForTest(t, root, ws workspace.ID, id change.ID, name string)` — ワークスペースのログからその ID の計画の行を除いて書き直し、`purged.jsonl` に `entity: sprint`(`releases/` なら `release`)・`display: name` の行を足し、保存する

- [ ] **Step 1: 失敗するテストを書く**
  - filemerge: `TestDropPurgedPlan`(S1 の `set`・`delete`・`close_sprint` の行が消え、S2 とアイテムの `sprint` の行は残る)、`TestPurgedPlanTargets`(`item`・`project` の行は数えない)
  - gitstore: `TestSyncDropsPurgedSprintRows`: B が S1 を `purgePlanForTest`、A が同時に S1 の名前を変えて同期 → B が同期 → A・B のワークスペースのログに S1 の行がなく、settings の YAML の `sprints` に S1 がない。2 回目の同期も通り、両方のファイルがバイト単位で同じ
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/... -run 'Purged'` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/infra/gitstore
git commit -m "feat(sync): 完全に削除したスプリント・リリースの行を、合わせたあとに取り除く(C5a)"
```

---

### Task 4: 合わせで取り除いた行を判定に渡す(`saves/plan.go`)

**Files:**
- Modify: `internal/app/port/replicator.go`(`ReceivedSave.DroppedPlan`・`DroppedPlan`)
- Create: `internal/infra/gitstore/saves/plan.go`
- Modify: `internal/infra/gitstore/saves/removed.go`(`pendingOf` の合わせを渡す条件に足すところだけ)
- Test: `internal/infra/gitstore/saves/removed_test.go`、`internal/infra/gitstore/sync_judge_repair_test.go`

**Interfaces:**
- Produces(`port`):
  - `type DroppedPlan struct { Workspace change.ID; Target change.ID; Log []change.Record; New []change.Record }` — 合わせの片側の、取り除かれたスプリント・リリースの行(`Log`)と、そのうち共通の祖先にない行(`New`)
  - `ReceivedSave.DroppedPlan []DroppedPlan` — 合わせのときだけ
- Produces(`saves`): `func droppedPlan(c *object.Commit) ([]port.DroppedPlan, error)` — 合わせ c の 2 つの親のそれぞれについて、c の `purged.jsonl` のスプリント・リリースの target のうち、c のワークスペースのログに行がなく、親のログに行があるもの。並びは 1 つ目の親、2 つ目の親、ワークスペース・target の内部 ID の順。`pendingOf` は `New` を持つものが 1 つでもあれば合わせを渡す

- [ ] **Step 1: 失敗するテストを書く**
  - saves: `TestDroppedPlanInMerge`(片側で S1 を取り除いて `purged.jsonl` に足し、もう片側で S1 の名前を変えた合わせ → `DroppedPlan` に `{W1, S1}`・`New` が名前の 1 行。名前を変えていなければ合わせを渡さない)
  - gitstore: `TestDroppedPlanReachesJudge`(Task 3 の場面で、A の同期の `Judge` に B の合わせが `DroppedPlan` つきで 1 回だけ渡る)
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/... -run 'DroppedPlan'` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/... ./internal/app/... && golangci-lint run ./internal/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/replicator.go internal/infra/gitstore
git commit -m "feat(sync): 合わせで取り除いたスプリント・リリースの行を判定に渡す(C5a)"
```

---

### Task 5: (6)・(7)・(8) の通知(`noticing/repair_plan.go`)

**Files:**
- Create: `internal/app/noticing/repair_plan.go`
- Modify: `internal/app/noticing/repair.go`(`kinds` に `activeSprints`・`planRefs` を足す)
- Test: `internal/app/noticing/repair_plan_test.go`(`fixture` の `repairSave` を使う)

**Interfaces:**
- Consumes: C4b2 の `batch.beforeSave`・`beforeRead`・`lastSaver`・`entityOf`・`oneLine`、`port.Reader` の `Purged()`、`planning.FromLog`、Task 1 の `planning.PlanTarget`
- Produces: `func activeSprints(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`、`func planRefs(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`。見分け方は C5a-10、宛先と鍵は C5a-9、開く先は C5a-11、本文は Global Constraints

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`)
  - `TestActiveNotice`: hanako が開始した Sprint 2 を計画中に戻した修復の保存 → 本文「実行中のスプリントが 2 つになったため、「Sprint 2」を計画中に戻しました」・`Workspace`・`Section == "sprints"`・`Source == "repair:active:{S2}:{開始の行の ID}"`。開始したのが taro → 作らない
  - `TestActiveOnceAcrossMachines`: 同じスプリントを戻した 2 つの修復の保存 → 受信箱は 1 行
  - `TestClosedNotice`: hanako が HH-9 を入れた完了の Sprint 24 から戻した修復 → 「スプリント「Sprint 24」は完了しているため、HH-9 をバックログに戻しました」・`Item`・`Source == "repair:closed:{HH-9}:{スプリントの行の ID}"`
  - `TestClosedNoticeToAdder`: HH-7 を Sprint 24 に入れたのは hanako、完了と同時に Done から In Progress に戻したのは taro → hanako に作る(戻した人ではなく入れた人)。あなたが taro なら作らない
  - `TestPlanRefNotices`(表): `purged.jsonl` に `display: Sprint 3` → 「スプリント「Sprint 3」は完全に削除されたため、HH-4 をバックログに戻しました」。リリース → 「リリース「v2」は完全に削除されたため、HH-4 のリリースを外しました」。別のワークスペース W2 の Sprint 5 → 「スプリント「Sprint 5」は HH-4 のワークスペースにないため、HH-4 をバックログに戻しました」。リリースの同じ場合 → 「リリース「v3」は HH-4 のワークスペースにないため、HH-4 のリリースを外しました」。どこにもない → 「スプリント「削除されたスプリント」は完全に削除されたため…」
  - `TestPlanRefBothFields`: スプリントとリリースの 2 つの修復の保存 → 2 行(鍵が違う)
  - `TestPlanRefSkipsPlaceRepair`: 修復 (5) の移動の保存(`set place` と `set sprint null`)→ (8) の通知を作らない
  - `TestPlanRefLastSaver`: スプリントの欄を最後に保存したのが taro(作成の行は hanako)→ 作らない
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/noticing/` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): スプリントの実行中の重なり・完了したスプリント・存在しない参照の修復を通知する(C5a)"
```

---

### Task 6: 合わせで取り除いた行の通知(`noticing/repair_plan_gone.go`)

**Files:**
- Create: `internal/app/noticing/repair_plan_gone.go`
- Modify: `internal/app/noticing/repair.go`(`kinds` に `droppedPlans`)
- Test: `internal/app/noticing/repair_plan_gone_test.go`

**Interfaces:**
- Consumes: Task 4 の `ReceivedSave.DroppedPlan`、`port.Reader.Purged()`
- Produces: `func droppedPlans(r port.Reader, b batch, i int, me string) ([]port.Notice, error)` — 合わせの `DroppedPlan` ごとに、`New` の人の行(`Actor.Kind == change.ActorUser`)の `by` があなたなら 1 件。名前は `purged.jsonl` の `display`、空なら `planning.FromLog(Log)` の名前、それもなければ作らない

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`)
  - `TestDroppedPlanNotice`: `New` に hanako の名前の変更 → 「「Sprint 3」は完全に削除されたため、あなたの変更は反映されませんでした」・`Workspace`・`Section == "sprints"`・`Source == "repair:purged:{S3}"`。リリース → `Section == "releases"`
  - `TestDroppedPlanOnlyNewRows`: `Log` に hanako の古い行、`New` は jiro の行だけ → 作らない
  - `TestDroppedPlanOnce`: 同じ合わせを 2 回判定 → 1 行。受信箱を開いて `Prune` しても消えない
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 完全に削除したスプリント・リリースと重なって反映されなかった変更を知らせる(C5a)"
```

---

### Task 7: 2 台の場面テスト

**Files:**
- Create: `cmd/hedgehug/plan_repair_scenario_test.go`

**Interfaces:**
- Consumes: Task 1〜6。`cmd/hedgehug/repair_helpers_test.go` の `firstMachine`・`cloneMachine`・`writeAs`・`sync`・`noticed`・`item`、`repair_notice_scenario_test.go` の `wantRepair`・`wantNoRepair`。スプリントは `writeAs` の中で `planning.SprintDrafts`・`planning.CloseDraft` を書く。`wire_e2e.go` は変えない(C5a-12)

- [ ] **Step 1: 場面テストを書く**(A = taro、B = hanako。本物の gitstore とテストごとの state.db)
  - `TestRepairNoticeTwoActive`: A が Sprint 1・Sprint 2 を作って同期、B が clone。A が Sprint 1(開始日 2026-11-16)、B が Sprint 2(2026-11-17)を開始 → A が同期、B が同期 → B に「実行中のスプリントが 2 つになったため、「Sprint 2」を計画中に戻しました」が 1 行。A が同期 → A のスプリントも Sprint 1 だけが実行中、A には作らない
  - `TestRepairNoticeClosedSprint`: A が Sprint 1 を完了、B が同時に HH-2 を Sprint 1 に入れる → A が同期、B が同期 → B に「スプリント「Sprint 1」は完了しているため、HH-2 をバックログに戻しました」、HH-2 のスプリントが空
  - `TestRepairNoticeReopenStays`: HH-2 が Done で入っている Sprint 1 を A が完了して同期 → B が同期して完了を受け取る → B が HH-2 を未着手に戻して保存、A が同時に別のアイテムのタイトルを変える → A が同期、B が同期(合わせ)→ HH-2 は Sprint 1 のまま、どちらにも同期の修復の通知がない
- [ ] **Step 2: 流して、足りない組み立てを直す** — Run: `go test ./cmd/hedgehug/ -run 'RepairNotice(TwoActive|ClosedSprint|ReopenStays)'` Expected: PASS
- [ ] **Step 3: 全体を確かめる** — Run: `go test ./... && golangci-lint run ./... && golangci-lint run --build-tags e2e ./...` Expected: PASS・指摘なし
- [ ] **Step 4: コミット**

```bash
git add cmd/hedgehug
git commit -m "test(sync): 2 台の場面で計画の同期の修復と通知を確かめる(C5a)"
```

---

### Task 8: 仕様・保存形式・トラック C の計画の記録と全体の検証

**Files:**
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:342`(リリースのワークスペース違いの本文)、`:532` のあと(ワークスペース違いのアクティビティの文 2 行)
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:193-218`(表の番号と順、(7) の見つけ方)、`:245`(スプリント・リリースの `display` は名前。「上の表の(1)・(6)」を「(1)・(8)」に)
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/05-planning-reports.md` `:171`
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C5 の行を C5a〜C5d に分け、漏れの表、G への申し送り)、`2026-11-12-hedgehug-track-b.md` `:17`、`2026-11-14-hedgehug-track-b5a.md` `:59`(保存形式の表の番号)

- [ ] **Step 1: 仕様と保存形式を書き直す**
  - 06 `:336` の「同期で未完了のアイテムが入ってきたときは」を「同期で未完了のアイテムが入ってきたとき(完了と同時に、別のマシンでアイテムを入れた、または未完了に戻したとき)は」にし、段落の末尾に「完了を受け取ったあとで未完了に戻したアイテムは、完了したスプリントに残す」を足す。05 `:171` も同じ対象に書き直す
  - 06 `:342` の本文の列に「「リリース「{名前}」は {ID} のワークスペースにないため、{ID} のリリースを外しました」」を足す。アクティビティの表の「存在しないリリースを外した」の行のあとに、「同期で、ワークスペースにないスプリントを外した | 同期 | {ID} をバックログに戻す(スプリントがワークスペースにない)」「同期で、ワークスペースにないリリースを外した | 同期 | {ID} のリリースを外す(リリースがワークスペースにない)」を足す
  - 保存形式の表: 番号を 06 `:350` の (1)〜(11) にそろえる((6) 実行中のスプリントが 2 つ、(7) 完了したスプリント、(8) 存在しないスプリント・リリース、(9) なくなったプロジェクトのルール、(10) 削除された値を指す設定・ルール・既定のテンプレート・自動化と初期ステータスの分類、(11) 1 つしかないもの)。「順の指定なし」は削除と変更の重なりだけを残し、「06 が順を決めていない」の文を「削除と変更の重なりは (1) の前に行う」にする。(7) の見つけ方を「完了したスプリントに、未完了のアイテムがあり、完了の行と、アイテムのスプリントかステータスの行が、合わせの別々の側にある(完了のあとに未完了に戻したものは直さない)」にする。`purged.jsonl` の段落に「スプリント・リリースの `display` は名前」を足し、「上の表の(1)・(6)」を「(1)・(8)」にする。トラック B の計画 `:17` の「(7)〜(9)」を「(8)〜(11)」に、B5a の計画 `:59` の「(7)・(8)」を「(9)・(10)」にする。`grep -rn '同期の修復」の表の\|同期の修復」の (' docs internal` で、ほかに古い番号の参照がないことを確かめる
- [ ] **Step 2: トラック C の計画を書き直す**
  - C5 の行を 4 行にする: C5a(この計画のファイル名と C5a-1 の範囲)、C5b 設定の修復((9)・(10)、プロジェクト別のルールの行の合わせでの取り除き)、C5c 重複の修復((11)、B5b のワークスペースの削除の取り消しとの重なり)、C5d 時計のずれの警告(C5a〜C5c と並行可)
  - 漏れの表の C5 を、節ごとに C5a〜C5d に書き分ける
  - G への申し送りに「スプリント・リリースの完全な削除の `purged.jsonl` の行の `display` は名前(C5a-6)。行の取り除きと参照の `set sprint null`・`set release null` は同じ保存で書く(保存形式)」を足す
- [ ] **Step 3: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./...` Expected: すべて PASS
- [ ] **Step 4: コミット**

```bash
git add docs
git commit -m "docs: 計画の同期の修復(C5a)を仕様・保存形式・トラック C の計画に書く"
```
