# hedgehug トラック C5b1(なくなったプロジェクトのルールの同期の修復)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 持ち主のプロジェクトがワークスペースにないプロジェクト別のルールと専用ルールを、同期の修復 (9) で外す。完全に削除したプロジェクトのルールの行は、合わせで取り除く。変更が反映されなかった人には、「同期の修復」の通知を 1 回だけ出す。

**Architecture:** 持ち主の見分け方(`project_rules/<P>/…` の行と、`rules/<R>/scope` が P の専用ルールの行)は `internal/domain/repair/rules.go` に置く。修復の段 `dropOrphanRules` は `internal/app/repairing/rules.go` に置き、`steps` の末尾((8) のあと。06 `:350` の順)に足す。完全に削除したプロジェクトのルールの行は、C5a の `dropPurgedPlan` と同じ所(`mergeTrees`)で `filemerge.DropPurgedRules` を使って取り除く。片側の共通の祖先にない行は、`saves` が `ReceivedSave.DroppedRules` として判定に渡す。通知は C4b2 の仕組み(`noticing.repairs` の振り分け・`AddOnce`)に足す。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite)。web は変えない。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`
  - `:288-292`((9))
  - `:326-332`(完全な削除・別の Store への移動と重なったときの本文)
  - `:344-350`(修復の決まり・順)
  - `:383`(通知を判定する単位)
  - `:455`(開く先)
  - `:538`(アクティビティの文)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`(プロジェクト別のルール・専用ルール。B5a)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`
  - `:180`(合わせで、完全に削除したものの行を取り除く)
  - `:210`(同期の修復の表の (9))
  - `:239`(完全に削除: プロジェクトのルールも同じ保存で取り除く)
  - `:241-245`(`purged.jsonl` の形)
- `docs/superpowers/plans/2026-11-15-hedgehug-track-c5a.md`(C5a-7・C5a-8・C5a-13。この計画はその隣に同じ形で足す)
- `2026-11-14-hedgehug-track-c4b2.md`(C4b2-6〜C4b2-12)
- `2026-11-14-hedgehug-track-b5a.md`(行の形)
- `2026-11-14-hedgehug-track-a5b.md`(プロジェクトの移動でのルールの引き継ぎ)
- `2026-11-12-hedgehug-track-c.md` の C5 の行と決定 9

## Global Constraints

土台・トラック C・C4a・C4b1・C4b2・C5a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限は `internal/archtest/size.go` のとおり。
  - Go ファイルは 300 行、テストは 600 行まで。関数は 60 行、認知的複雑度は 15 まで。
  - パッケージのテスト以外のファイルは 20 個まで。`internal/infra/gitstore`(直下)と `internal/app/port` は 20 個で満杯なので、ファイルを足さない。
  - 足すのは `domain/repair/rules.go`・`app/repairing/rules.go`・`infra/gitstore/filemerge/rules.go`・`infra/gitstore/saves/rules.go`・`app/noticing/repair_rules.go` だけにする。
- 口は git の言葉を使わない。`port` の型の名前とコメントに「コミット」「ブランチ」「マージ」を書かない。
- app の機能どうしは import しない。`noticing` は `repairing`・`transfers` を import しない。
- 修復は 1 件を 1 回の保存にする(C3a-18)。実行者は `change.ActorSync`。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(`go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 修復の保存の件名(06 `:538`): 「プロジェクト「{タイトル}」のルールを削除(プロジェクトがない)」。{タイトル} の決め方は C5b1-4。
- 通知の本文({…} はそのまま置き換える。名前の改行と続いた空白は 1 つの空白にする):
  - 完全に削除されたとき(06 `:292`・`:328`): 「{プロジェクトの ID} は完全に削除されたため、あなたの変更は反映されませんでした」
  - 別の Store へ移されたとき(06 `:292`・`:332`): 「{旧 ID} は {移動した人} が {移動先の Store 名} へ移動したため、あなたの変更は反映されませんでした。移動先は {新 ID} です」
  - 同じ Store の別のワークスペースへ移されたときは作らない。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C5b1-1 | 範囲は次の 3 つ。(10) は C5b2、(11) は C5c、時計のずれは C5d。<br>・修復 (9)<br>・完全に削除したプロジェクトのルールの行の、合わせでの取り除き(トラック C の決定 9 の残り)<br>・その 2 つの通知 | ユーザーの決定(2026-11-15、C5b を 2 本に分ける) |
| C5b1-2 | 持ち主の規則:<br>・`project_rules/<P>/…` の行の持ち主は P。<br>・`rules/<R>/…` の行(`delete rules/<R>` を含む)の持ち主は、同じログの `rules/<R>/scope` の `set` の `to`(専用ルールのプロジェクト)。<br>・scope の行がないルール(ワークスペース全体のルール)は持ち主なし。<br>・`disabled_rules` に入っているワークスペース全体のルールそのものは、外すだけで削除しない | B5a-10(scope は作成のときだけ書き、変えない)。保存形式 `:210` |
| C5b1-3 | (9) の対象: ワークスペースのログに、持ち主のプロジェクトがそのワークスペースのプロジェクト(`Projects().List(ws)`。削除済みを含む)にないルールがある。<br>直し方: プロジェクトごとに 1 回の保存で、次の行を書く。<br>・`projectrule.ResetDrafts` の 4 区画分<br>・削除されていない専用ルールそれぞれの `automation.DeleteDraft`<br>保存の並びは、ワークスペース・プロジェクトの内部 ID の順 | 06 `:290`。保存形式 `:210`。A5b の移動元の書き方(`transfer.CarryRules` の `source`)と同じ行 |
| C5b1-4 | 件名の {タイトル} は、次の順で最初に読めたもの。<br>1. プロジェクトを読めればそのタイトル(同じ Store の別のワークスペースへ移ったとき)<br>2. `transfers.jsonl` のその target の最後の行の `old`(別の Store へ移ったとき)<br>3. `purged.jsonl` の `display`<br>4. どれもなければ内部 ID | 別の Store へ移ったものと、完全に削除したものは、フォルダが消えてタイトルを読めない。残っているのは記録の ID だけ |
| C5b1-5 | 合わせ(`mergeTrees`)の最後で、C5a の取り除きと同じ所で次を行い、そのあとで YAML を作り直す。<br>・`purged.jsonl` の `entity: project` の target を集める。<br>・すべての `workspaces/<ID>/log.jsonl` から、C5b1-2 の持ち主がその target の行を取り除く。<br>・専用ルールの scope は、取り除く前の同じログから読む | 保存形式 `:180`。`purged.jsonl` はプロジェクト別のルールの行を持たない(`:245`)ので、プロジェクトの行で見分ける |
| C5b1-6 | 判定に渡す合わせの条件(C4b2-2 の (c))に、「C5b1-5 で取り除いた行のうち、片側の共通の祖先にない行があるもの」を足す。<br>・`ReceivedSave.DroppedRules []DroppedRules` に `Workspace`・`Project`・`Log`(その側のそのプロジェクトのルールの行)・`New`(そのうち共通の祖先にない行)を持たせる。<br>・両側の分を持つ。<br>・`DroppedPlan` と同じ作りで、`saves` の差分を取る中身は、まとめ方の関数を受け取る形にして 2 つで共有する | C5a-8 と同じ。取り除いた行は `Removed` に出ないので、通知が作られない |
| C5b1-7 | (9) の通知の原因の見分け方。`noticing` の `readCauses`・`causes.of`(プロジェクトの内部 ID)で次のように決める。<br>・`purged.jsonl` にある → 完全な削除の本文。鍵は `repair:purged:{P}`。<br>・別の Store への転送記録がある → 移動の本文。鍵は `repair:moved:{P}:{転送記録の ID}`。<br>・どちらもない(同じ Store の別のワークスペース、手での書き換え) → 作らない。<br>鍵は、プロジェクトそのものが合わせで消えたときの通知(`droppedEntities`)と同じにする。ルールとプロジェクトの両方を変えた人には、同じ本文の通知が 1 行だけ出る | 06 `:292`。06 `:383` の「保存ごとに 1 行」は、本文が同じ出来事の 2 行を求めていない。C4b2-6(移動ごとに 1 件) |
| C5b1-8 | (9) の通知の宛先。修復の保存の直前のログで、外したものをそれぞれ入れた人(人の行だけ)を集め、あなたがいれば 1 件にする。<br>・`remove` した値: その項目のその値の `add` の行のうち、`at` が最もあとの行の `by`。<br>・`set` で null にした `template_default`: その項目の `lastSaver`。<br>・削除した専用ルール: そのルールの行(`rules/<R>/…`)の `lastSaver`。<br>移動した人が書いた移動元の行は `remove` なので、宛先に入らない | 06 `:292`「ルールを変えた人」。修復の保存は `Sides` を持たない(C4b2-9 は restore の行だけ)ので、外したものの行から決める |
| C5b1-9 | 合わせで取り除いた行の通知(`droppedRules`)の作り方。<br>・合わせの `DroppedRules` ごとに、`New` の人の行(`Actor.Kind == change.ActorUser`)の `by` があなたなら 1 件。<br>・本文は完全な削除の本文で、{プロジェクトの ID} は `purged.jsonl` の `display`。`display` が空なら作らない。<br>・鍵は `repair:purged:{P}`(C5b1-7 と同じ) | C5a-8・C5a-9 と同じ |
| C5b1-10 | 開く先は、プロジェクトを指す通知と同じにする。<br>・`Workspace` はルールがあったワークスペース。<br>・`Project` は P。<br>・`Display` は、完全な削除なら `purged.jsonl` の `display`、移動なら転送記録の `old`。<br>受信箱は、これを旧 ID の転送記録の画面へのリンクとして出す | 06 `:455`(反映されなかった通知は、作ったときの ID を出し、旧 ID の転送記録の画面を開く) |
| C5b1-11 | (9) の保存の見分け方: 修復の保存で、ワークスペースのログに C5b1-2 の持ち主 P の行があり、P がその保存のあとでそのワークスペースにないもの。<br>C5b2 の (10) がプロジェクト別のルールを外す保存は、P がワークスペースにあるので、この判定に入らない | C5b2 と見分けるため |
| C5b1-12 | 完全な削除を書く口は、まだない(G)。テストでは、gitstore の `purgeProjectForTest` と、cmd の `purgeProjectAs` を使う。中身は、プロジェクトのフォルダを消す、ルールの行を取り除く、`purged.jsonl` に `entity: project`・`display: HH-P{n}` を足す、の 3 つ。<br>G への申し送りに、次の 2 つを書く。<br>・プロジェクトの完全な削除では、ルールの行も同じ保存で取り除く。<br>・この 2 つの手助けを、本物の書き口に置き換える | 保存形式 `:239`。C5a-12 と同じ |
| C5b1-13 | C4c1(ブランチ feat/track-c4c、別の worktree)も、`port.ReceivedSave` と `saves` の `pendingOf` に手を入れる。C5b1 が `removed.go` の `pendingOf` に足すのは、条件の呼び出し 1 つだけにする。develop にあとから入る方が合わせる | C5a-13 と同じ |
| C5b1-14 | e2e は足さない。2 台の場面は、`cmd/hedgehug` の本物の gitstore と state.db で確かめる | C5a-12 と同じ |

## Review Focus

1. **同じ Store の別のワークスペースへの移動:** 太郎さんがプロジェクト「Web」(HH-P1)をワークスペース「メイン」から「サブ」へ移し、同じころ花子さんが「メイン」で「Web」の使わないステータスに「Review」を足した。同期のあと、両方のマシンの「メイン」のログに「Web」のルールが残らない。2 台の「メイン」のログはバイト単位で同じになる。花子さんに通知は出ない。
   - テスト: Task 2 の `TestDropOrphanRulesMovedWithinStore`、Task 4 の `TestOrphanRulesSameStoreNoNotice`、Task 6 の `TestRepairRulesMovedWorkspace`
2. **削除済みのプロジェクト:** 削除済み(論理削除)のプロジェクトのルールは外さない。復元で働く。
   - テスト: Task 1 の `TestOrphanRulesKeepsDeletedProject`
3. **ワークスペース全体のルールを外す一覧:** なくなったプロジェクトの `disabled_rules` に入っていたワークスペース全体のルールは、一覧から外すだけで、ルールそのものは削除しない。削除済みの専用ルールに、もう一度 `delete` を書かない。
   - テスト: Task 1 の `TestOrphanRulesDraftsDisabledAndOwn`
4. **完全な削除とルールの変更の重なり:** 完全に削除したプロジェクトのルールを、別のマシンで同時に変えた。合わせのあと、どちらのログにもそのルールの行が戻らない。変えた人に 1 行だけ出る。古い行(共通の祖先にある行)だけの人には出ない。
   - テスト: Task 3 の `TestSyncDropsPurgedProjectRules`、Task 5 の `TestDroppedRulesNotice`・`TestDroppedRulesOnlyNewRows`
5. **プロジェクトとルールの両方を変えた人:** 別の Store へのプロジェクトの移動と同時に、プロジェクトのタイトルとルールの両方を変えた人の受信箱は、1 行だけになる。
   - テスト: Task 4 の `TestOrphanRulesSharesMovedKey`

---

### Task 1: 持ち主の規則と (9) の直し方(`domain/repair/rules.go`)

**Files:**
- Create: `internal/domain/repair/rules.go`、Test: `internal/domain/repair/rules_test.go`

**Interfaces:**
- Produces(`repair`):
  - `func RuleScopes(wsLog []change.Record) map[change.ID]project.ID`: 専用ルールの内部 ID と、その `rules/<R>/scope` の `set` の `to`。
  - `func RuleOwner(r change.Record, scopes map[change.ID]project.ID) (project.ID, bool)`: C5b1-2 の持ち主。持ち主がなければ false。
  - `type OrphanRules struct { Project project.ID; Drafts []change.Draft }`
  - `func Orphans(wsLog []change.Record, inWorkspace func(project.ID) bool) []OrphanRules`: C5b1-3 の対象と行。並びはプロジェクトの内部 ID の順。
    - 行は `projectrule.ResetDrafts` の 4 区画(`SectionStatuses`・`SectionWorkflow`・`SectionTemplates`・`SectionAutomation` の順)と、`automation.FromLog` の Scope がそのプロジェクトのルールそれぞれの `automation.DeleteDraft`(内部 ID の順)。
    - 行がなければ入れない。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestRuleOwner`(表):
    - `add project_rules/P1/hidden_statuses` → P1
    - `set project_rules/P1/template_default/Task` → P1
    - `set rules/R1/name`(R1 の scope が P2)→ P2
    - `delete rules/R1` → P2
    - `set rules/R9/name`(scope なし)→ false
    - `set initial_status` → false
  - `TestOrphans`: 次のワークスペースのログ。
    - P1(ワークスペースにない)に、使わないステータス S1、ワークフロー S1→S2、使うテンプレート T1、既定 Task=T1、外すルール R9 がある。
    - 専用ルール R1(scope P1)がある。
    - P2(ワークスペースにある)にルールがある。
    - 期待: 結果は P1 の 1 件だけ。行は `remove hidden_statuses S1`・`remove workflow/S1 S2`・`remove templates T1`・`set template_default/Task T1→null`・`remove disabled_rules R9`・`delete rules/R1` の順。
  - `TestOrphanRulesKeepsDeletedProject`: `inWorkspace` が削除済みのプロジェクトに true を返す → 空。
  - `TestOrphanRulesDraftsDisabledAndOwn`: 次の 2 つを確かめる。
    - ワークスペース全体のルール R9 が P1 の `disabled_rules` にある → `remove disabled_rules R9` だけで、`delete rules/R9` は書かない。
    - P1 の専用ルール R2 が削除済み → `delete rules/R2` を書かない。
  - `TestOrphansNothingLeft`: P1 がないが、ルールの行がすべて remove 済み(`projectrule.FromLog` で空)で、専用ルールもない → 空。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/repair/` Expected: FAIL(`undefined: RuleScopes` ほか)
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/... && golangci-lint run ./internal/domain/...` Expected: PASS・指摘なし(domain の import の規則を含む)
- [ ] **Step 5: コミット**

```bash
git add internal/domain/repair
git commit -m "feat(repair): なくなったプロジェクトのプロジェクト別のルールと専用ルールを見つける規則を足す(C5b1)"
```

---

### Task 2: 修復の段 (9)(`app/repairing/rules.go`)

**Files:**
- Create: `internal/app/repairing/rules.go`
- Modify: `internal/app/repairing/run.go`(`steps` の末尾に `dropOrphanRules`)
- Test: `internal/app/repairing/rules_test.go`(既存の `newEnv`・`e.write`・`e.run` を使う)

**Interfaces:**
- Consumes: Task 1 の `repair.Orphans`、`port.RepairWriter`、`Reader` の `Projects()`・`Transfers()`・`Purged()`
- Produces: `func dropOrphanRules(ctx context.Context, w port.RepairWriter) error`(`step`)
  - ワークスペースごとに `Projects().List(ws)` の内部 ID を `inWorkspace` にする。
  - `Orphans` の 1 件を 1 回の保存で `tx.WriteWorkspaces().Append(ws, drafts...)` にする。
  - 件名は Global Constraints の文。{タイトル} は C5b1-4 の順で決める。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestDropOrphanRulesMovedWithinStore`: 次の場面を作る。
    - 「Web」がワークスペース「サブ」にある。
    - 「メイン」のログに、「Web」の使わないステータス「Review」と専用ルール「完了で閉じる」が残っている。
    - 期待:
      - 保存は 1 つで、件名は「プロジェクト「Web」のルールを削除(プロジェクトがない)」。
      - 「メイン」の `projectrule.FromLog` に「Web」がない。
      - `automation.FromLog` に「完了で閉じる」がない。
      - もう一度 `Run` しても何も書かない。
  - `TestDropOrphanRulesTitleFallback`(表): プロジェクトを読めないときの {タイトル}。
    - 転送記録の `old` が `HH-P3` → 「プロジェクト「HH-P3」のルールを削除(プロジェクトがない)」
    - `purged.jsonl` の `display` が `HH-P4` → 「HH-P4」
    - どちらもない → 内部 ID
  - `TestDropOrphanRulesKeepsDeleted`: 「Web」が「メイン」で削除済み → 何も書かない。
  - `TestDropOrphanRulesAfterPlan`: 1 回の `Run` で (8) と (9) が両方要る場面。修復の保存の件名が「HH-3 をバックログに戻す(スプリントが完全に削除)」「プロジェクト「Web」のルールを削除(プロジェクトがない)」の順になる。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/repairing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/repairing/ ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/app/repairing
git commit -m "feat(repairing): なくなったプロジェクトのルールを同期で外す(C5b1)"
```

---

### Task 3: 完全に削除したプロジェクトのルールの行を合わせで取り除く

**Files:**
- Create: `internal/infra/gitstore/filemerge/rules.go`、Test: `internal/infra/gitstore/filemerge/rules_test.go`
- Modify: `internal/infra/gitstore/filemerge/plan.go`(`purged.jsonl` の読み取りを、entity を受け取る非公開の関数にして共有する)
- Modify: `internal/infra/gitstore/sync_tree.go`(`dropPurgedPlan` で `DropPurgedRules` も呼ぶ。関数名は `dropPurged` にする)
- Test: `internal/infra/gitstore/sync_purge_test.go`(`purgeProjectForTest` と場面)

**Interfaces:**
- Consumes: Task 1 の `repair.RuleScopes`・`repair.RuleOwner`
- Produces(`filemerge`):
  - `func PurgedProjects(purged []byte) map[string]bool`: `entity` が `project` の行の target。読めない行は `PurgedPlanTargets` と同じく飛ばす。
  - `func DropPurgedRules(log []byte, projects map[string]bool) []byte`: 持ち主が projects にある行を除く。scope は除く前の log から読む。ほかの行はバイト列のまま残す。
- テストの手助け(gitstore): `purgeProjectForTest(t, root, ws workspace.ID, p project.ID, display string)`
  - `projects/<P>` を消し、ワークスペースのログから P のルールの行を除いて書き直す。
  - `purged.jsonl` に `entity: project`・`display` の行を足して保存する。

- [ ] **Step 1: 失敗するテストを書く**
  - filemerge:
    - `TestDropPurgedRules`: P1 の `project_rules` の行と、P1 の専用ルール R1 の行(scope の行を含む)が消える。P2 のルール、ワークスペース全体のルール、スプリントの行は残る。
    - `TestPurgedProjects`: `item`・`sprint` の行は数えない。
  - gitstore:
    - `TestSyncDropsPurgedProjectRules`: B が「Web」を `purgeProjectForTest` し、A が同時に「Web」の使わないステータスに「Review」を足して同期する。そのあと B が同期する。
      - A・B の「メイン」のログに「Web」のルールの行がない。
      - 2 回目の同期も通る。
      - 両方のファイルがバイト単位で同じになる。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/... -run 'PurgedRules|PurgedProject'` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/...` Expected: PASS
- [ ] **Step 5: コミット**

```bash
git add internal/infra/gitstore
git commit -m "feat(sync): 完全に削除したプロジェクトのルールの行を、合わせたあとに取り除く(C5b1)"
```

---

### Task 4: (9) の通知(`noticing/repair_rules.go` の `orphanRules`)

**Files:**
- Create: `internal/app/noticing/repair_rules.go`
- Modify: `internal/app/noticing/repair.go`(`kinds` の `droppedPlans` のあとに `orphanRules` を足す)
- Modify: `internal/app/noticing/repair_gone.go`(`droppedNotice` の移動の本文を、`movedText(b batch, c cause, who string) string` に切り出して共有する。本文は変えない)
- Test: `internal/app/noticing/repair_rules_test.go`(`fixture` の `repairSave` を使う)

**Interfaces:**
- Consumes:
  - C4b2 の `batch.beforeSave`・`lastSaver`・`readCauses`・`causes.of`・`purgedText`・`nameOf`・`oneLine`
  - Task 1 の `repair.RuleScopes`・`repair.RuleOwner`
- Produces: `func orphanRules(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`
  - 見分け方は C5b1-11、原因と鍵は C5b1-7、宛先は C5b1-8、開く先は C5b1-10。
  - 本文は Global Constraints の文。

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`)
  - `TestOrphanRulesMovedStore`: 修復の保存の内容と、期待する通知。
    - 修復の保存: 別の Store「team-b」へ移った「Web」(旧 ID HH-P1、新 ID WEB-P2、移動は taro)の、hanako が足した使わないステータス「Review」を外した。
    - 本文: 「HH-P1 は 太郎 が team-b へ移動したため、あなたの変更は反映されませんでした。移動先は WEB-P2 です」
    - フィールド: `Project == P1`・`Display == "HH-P1"`・`Source == "repair:moved:{P1}:{転送記録の ID}"`
  - `TestOrphanRulesPurged`: `purged.jsonl` に `display: HH-P3` がある → 「HH-P3 は完全に削除されたため、あなたの変更は反映されませんでした」・`Source == "repair:purged:{P3}"`
  - `TestOrphanRulesSameStoreNoNotice`: 「Web」が同じ Store の「サブ」にある → 作らない。
  - `TestOrphanRulesRecipient`(表):
    - 外した「Review」を足したのが taro → 作らない。
    - 削除した専用ルールを最後に保存したのが hanako → 作る。
    - null にした既定のテンプレートを最後に選んだのが hanako → 作る。
  - `TestOrphanRulesSharesMovedKey`: 同じ判定で、次の 2 つがある。受信箱は 1 行になる。
    - 合わせで消えた「Web」に hanako が書いた行(`droppedEntities`)
    - (9) の修復の保存(`orphanRules`)
  - `TestOrphanRulesSkipsValueRepair`: ワークスペースにある「Web」のルールから値を外した修復の保存(C5b2 の形)→ 作らない。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/noticing/` Expected: PASS(既存の `droppedEntities` のテストも変わらず通る)
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): なくなったプロジェクトのルールの修復で、反映されなかった変更を知らせる(C5b1)"
```

---

### Task 5: 合わせで取り除いたルールの行を判定に渡し、通知する

**Files:**
- Modify: `internal/app/port/replicator.go`(`ReceivedSave.DroppedRules`・`DroppedRules`)
- Create: `internal/infra/gitstore/saves/rules.go`
- Modify: `internal/infra/gitstore/saves/plan.go`(`droppedFrom`・`droppedIn` を、まとめ方の関数を受け取る形にして共有する)
- Modify: `internal/infra/gitstore/saves/removed.go`(`pendingOf` の条件に `hasNewRules` を足すところだけ)
- Modify: `internal/app/noticing/repair_rules.go`(`droppedRules`)、`internal/app/noticing/repair.go`(`kinds` に `droppedRules`)
- Test: `internal/infra/gitstore/saves/removed_test.go`、`internal/infra/gitstore/sync_judge_repair_test.go`、`internal/app/noticing/repair_rules_test.go`

**Interfaces:**
- Consumes: Task 3 の `filemerge.PurgedProjects`、Task 1 の `repair.RuleScopes`・`repair.RuleOwner`
- Produces(`port`):
  - `type DroppedRules struct { Workspace change.ID; Project change.ID; Log []change.Record; New []change.Record }`
  - `ReceivedSave.DroppedRules []DroppedRules`: 合わせのときだけ持つ。
- Produces(`saves`):
  - `func droppedRules(c *object.Commit, base func() (*object.Tree, error)) ([]port.DroppedRules, error)`: `droppedPlan` と同じ並び。
  - `func hasNewRules(ds []port.DroppedRules) bool`
- Produces(`noticing`): `func droppedRules(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`(C5b1-9・C5b1-10)

- [ ] **Step 1: 失敗するテストを書く**
  - saves の `TestDroppedRulesInMerge`: 次の合わせを作る。
    - 片側で「Web」を取り除き、`purged.jsonl` に足した。
    - もう片側で「Web」の使わないステータスに「Review」を足した。
    - 期待: `DroppedRules` に `{メイン, P1}` があり、`New` は add の 1 行。ルールを変えていなければ、合わせを渡さない。
  - gitstore の `TestDroppedRulesReachJudge`: Task 3 の場面で、A の同期の `Judge` に、B の合わせが `DroppedRules` つきで 1 回だけ渡る。
  - noticing(あなたは `hanako`):
    - `TestDroppedRulesNotice`: `New` に hanako の add がある → 本文「HH-P1 は完全に削除されたため、あなたの変更は反映されませんでした」・`Project == P1`・`Display == "HH-P1"`・`Source == "repair:purged:{P1}"`
    - `TestDroppedRulesOnlyNewRows`: `Log` に hanako の古い行があり、`New` は jiro の行だけ → 作らない。
    - `TestDroppedRulesOnce`: 同じ合わせを 2 回判定する → 1 行。受信箱を開いて `Prune` しても消えない。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/... ./internal/app/noticing/ -run 'DroppedRules'` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/infra/gitstore/... ./internal/app/... && golangci-lint run ./internal/...` Expected: PASS・指摘なし(C5a の `DroppedPlan` のテストも変わらず通る)
- [ ] **Step 5: コミット**

```bash
git add internal/app/port/replicator.go internal/infra/gitstore internal/app/noticing
git commit -m "feat(sync): 合わせで取り除いたプロジェクトのルールの行を判定に渡し、反映されなかった変更を知らせる(C5b1)"
```

---

### Task 6: 2 台の場面テスト

**Files:**
- Create: `cmd/hedgehug/rules_repair_scenario_test.go`

**Interfaces:**
- Consumes:
  - Task 1〜5
  - `cmd/hedgehug/repair_helpers_test.go` の `firstMachine`・`cloneMachine`・`writeAs`・`sync`・`noticed`
  - `plan_purge_scenario_test.go` の `commitAll`
  - `repair_notice_scenario_test.go` の `wantRepair`・`wantNoRepair`
- 移動は、`writeAs` の中で、プロジェクトの `set place` と `transfer.CarryRules`(`internal/domain/transfer/rules.go:66`)の移動元・移動先の行を書く(`app/transfers` は組み立てない)。
- 手助け: `purgeProjectAs(t, m machine, login string, ws workspace.ID, p project.ID, display string)`(C5b1-12。`purgeSprintAs` と同じ作り)。

- [ ] **Step 1: 場面テストを書く**(A = taro、B = hanako。本物の gitstore とテストごとの state.db)
  - `TestRepairRulesMovedWorkspace`: 手順と期待。
    1. A がワークスペース「メイン」「サブ」とプロジェクト「Web」を作り、「Web」に専用ルールを 1 つ作って同期する。B が clone する。
    2. A が「Web」を「サブ」へ移す。B は同時に、「メイン」で「Web」の使わないステータスに「Review」を足す。
    3. A が同期し、B が同期し、A がもう一度同期する。
    - 期待:
      - A・B の「メイン」に「Web」のルールも専用ルールもない。
      - 「サブ」の「Web」のルールは、移動で引き継いだまま。
      - 2 台の「メイン」のログが同じ。
      - どちらにも同期の修復の通知がない。
      - 修復の件名「プロジェクト「Web」のルールを削除(プロジェクトがない)」が、アクティビティで 1 行になる。
  - `TestRepairRulesPurgedProject`: 手順と期待。
    1. A が「Web」(HH-P1)を `purgeProjectAs` する。B は同時に、「Web」の使わないステータスに「Review」を足す。
    2. A が同期し、B が同期する。
    - 期待:
      - B に「HH-P1 は完全に削除されたため、あなたの変更は反映されませんでした」が 1 行出る。
      - A が同期したあと、2 台のログに「Web」のルールの行がない。
      - A には通知を作らない。
- [ ] **Step 2: 流して、足りない組み立てを直す** — Run: `go test ./cmd/hedgehug/ -run 'RepairRules'` Expected: PASS
- [ ] **Step 3: 全体を確かめる** — Run: `go test ./... && golangci-lint run ./... && golangci-lint run --build-tags e2e ./...` Expected: PASS・指摘なし
- [ ] **Step 4: コミット**

```bash
git add cmd/hedgehug
git commit -m "test(sync): 2 台の場面で、なくなったプロジェクトのルールの修復と通知を確かめる(C5b1)"
```

---

### Task 7: 保存形式・トラック C の計画の記録と全体の検証

**Files:**
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`
  - `:180`: 「`purged.jsonl` に載っているスプリント・リリース・プロジェクト別のルールの行は」を、「`purged.jsonl` に載っているスプリント・リリースの行と、`purged.jsonl` に載っているプロジェクトのプロジェクト別のルール・専用ルール(`scope` がそのプロジェクトのルール)の行は」にする。
  - `:245` の段落: 「同期の修復(上の表の(1)・(8))」を「(1)・(8)・(9)と合わせでの取り除き」にする。
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`
  - C5b の行を C5b1(この計画のファイル名と C5b1-1 の範囲)と C5b2(10 の各節)の 2 行に分ける。
  - 漏れの表の「プロジェクトがなくなったときのルール」を C5b1 にする。ほかの (10) の 4 行は C5b2 にする。
  - 通知の行の「C5b((9)・(10))」を「C5b1((9))、C5b2((10))」にする。
  - 決定 9 の「C5b(プロジェクト別のルール)」を「C5b1(プロジェクト別のルール・専用ルール。プロジェクトの行で見分ける)」にする。
  - G への申し送りに C5b1-12 の 2 点を足す。

- [ ] **Step 1: 保存形式とトラック C の計画を書き直す**(上のとおり)。
  - `grep -n 'C5b' docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` で、分けていない「C5b」が残っていないことを確かめる。
- [ ] **Step 2: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./...` Expected: すべて PASS
- [ ] **Step 3: コミット**

```bash
git add docs
git commit -m "docs: なくなったプロジェクトのルールの修復(C5b1)を保存形式とトラック C の計画に書く"
```
