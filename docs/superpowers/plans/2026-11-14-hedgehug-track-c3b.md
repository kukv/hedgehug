# hedgehug トラック C3b(親とワークスペースが違う子の修復)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期で合わせたあと、削除されていない子が親と別のワークスペースにあれば、その子を親のワークスペースとプロジェクトへ 1 件ずつ移す(その子の下の削除済みのものは同じ保存で一緒に移す)。番号は親のワークスペースの次の番号に振り直し、値を引き直し、転送記録を残す(修復 (5) の 1 つ目の箇条)。

**Architecture:** `internal/domain/repair` に、直す子を 1 件選ぶ `WorkspaceMismatch`、一緒に移す削除済みのものを集める `DeletedBelow`、新しい番号を振る `NumberByOld` を足す。`internal/app/repairing` に段 `matchWorkspaces` を足し、C3a の `matchProjects` の前に置く。行は A5a の同じ Store の別のワークスペースへの移動(`internal/app/transfers/move.go` の `writeWithin`)と同じ形で書く。値の引き直しは `transfer.Remap`、転送記録は `transfer.Record` を使う。転送記録の `to_store` に自分の Store ID を書くため、`port.RepairWriter` に `Store()` を足す。番号の行は `numbering.Drafts`(番号の順の `number` の行。A5a の transfers と共用し、transfers の動きは変わらない)で書く。

**Tech Stack:** Go 1.27.3。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「親と別のワークスペース・プロジェクトにある子」(1 つ目の箇条と「同期の修復」としての保存)、「修復の決まり」、「アクティビティ」の「対象」の表の「同期で、親と別のワークスペース・プロジェクトにあった子を移した」の行
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「同期の修復」の表の (5) の行(`:206`)、「移動・番号・旧 ID」
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`「別のワークスペースと別の Store への移動」「値の引き直し」
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c3a.md` の決定 C3a-2・C3a-3・C3a-5・C3a-16・C3a-18
- `docs/superpowers/plans/2026-11-14-hedgehug-track-a5a.md` の「持ち場と境目」の C3b の行と、決定 A5a-3
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C3b の行(完成の条件)

## Global Constraints

土台・トラック C・C3a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個。`internal/infra/gitstore` と `internal/app/apptest` はテスト以外のファイルが 20 個で満杯なので、ファイルを足さない(触るのは `sync_repair.go`(256 行)と `sync.go`(268 行)だけ)。`cmd/hedgehug/repair_scenario_test.go` は 294 行なので、場面テストは新しいファイルに書く。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(`go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- アクティビティの文(06 の表のとおり): `{旧 ID} を {新 ID} に移動(親に合わせる)`。旧 ID・新 ID は、その保存で移した削除されていない子のもの(1 保存 1 行)。実行者は「同期」(`change.Actor{Kind: change.ActorSync}`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C3b-1 | 1 回の修復で、削除されていない子 1 件を、1 回の保存で移す。孫は、子を移したあとに読み直したとき、親(移した子)とワークスペースが違う子として次の保存で移す。直す子がなくなるまで繰り返す(C3a の `matchProjects` と同じ形)。その子の下の削除済みのもの(削除済みのものだけをたどって届くもの。`repair.DeletedBelow`)は、同じ保存で一緒に移す。例: HH-1 を Mobile へ移した人と、HH-1 の下に Task HH-3・その下に Step HH-4 を作った人が同期すると、「HH-3 を MB-3 に移動(親に合わせる)」「HH-4 を MB-4 に移動(親に合わせる)」の 2 回の保存になる | ユーザーの決定(2026-11-14)。保存形式 `:327`(子に `set place` を書き、`transfers.jsonl` に 1 行足す)と C3a の `matchProjects`、決定 C3a-18(1 コミットが通知 1 件・アクティビティ 1 行)にそろえる。削除済みのものは (5) の直す対象ではない(06「削除されていない子」)が、残すと復元したときに親とワークスペースが違う子になるので、いちばん近い削除されていない祖先と一緒に移す。削除されていない子の下の削除されていないものは、(2) のあとなので削除済みのものの下にはいない。生きている親の直下で子そのものが削除済みのとき(送る前に作って削除した子など)は移さない。06 は直す対象を「削除されていない子」とし、復元したあとに残ったずれは診断(06「修復の決まり」2 つ目、08)が同期と同じ直し方で直す。一緒に移す削除済みのもののうち、すでに移動先のワークスペースにあるものは移さない |
| C3b-2 | 直す子の選び方: 削除されていない子で、親も削除されておらず、親とワークスペースが違うもの。深さの浅い順、同じなら旧い番号の小さい順、同じなら内部 ID の辞書順の最初の 1 件 | 06「親から順に」と「旧い番号の小さい順、同じなら内部 ID の辞書順」。深さは C3a の `depthOf` と同じ数え方 |
| C3b-3 | 番号: 直す子に親のワークスペースの次の番号(`numbering.Next`)を振り、一緒に移す削除済みのものに、旧い番号の小さい順(同じなら内部 ID の辞書順)で続きの番号を振る。`transfer.Number`(兄弟の順の深さ優先)は使わない。トラック C の計画の C3b の行の「番号は `transfer.Number`」は、この決定に合わせて直す(Task 4) | 06:314 と保存形式 `:206` が順を「旧い番号の小さい順」と定めている。直す子どうしの順は C3b-2 で決まる。`transfer.Number` は利用者の移動(02)の順 |
| C3b-4 | 行は A5a の `writeWithin` と同じ形にする。アイテムごとに `set place`(ワークスペースとプロジェクトを親のものにし、番号を新しい番号にする。親と順位は変えない)と `transfer.Remap` の行、親のワークスペースに番号ごとの `number` の行、アイテムごとに転送記録(`entity: item`、`to_store` は自分の Store ID)。`Remap` の移動元の設定は、そのアイテムのいまのワークスペースのもの。移動先のルールは、親のプロジェクトのプロジェクト別のルール。`people` と `statusChoice` は nil を渡す | 06「ステータスは、確認ダイアログを出せないので…同じ分類の、移動先の既定のステータスに置き換える」(`Remap` は `statusChoice` が nil なら分類の既定にする)。同じ Store なので担当者とウォッチはそのまま。スプリントとリリースは `Remap` が外す(F1) |
| C3b-5 | `port.RepairWriter` に `Store() (port.StoreID, error)` を足す。gitstore は `.hedgehug/store.yaml` の Store ID を読む(`readStoreYAML`)。apptest は、Store ID のフィールドを `StoreID` に改名し(メソッド `Store` と同名にできないため)、`w.StoreID, nil` を返す | 転送記録の `to_store` が空だと `TransferWriter.Append` が断る。同じ Store への移動は自分の Store ID を書く(保存形式「移動・番号・旧 ID」、`transferTargets`) |
| C3b-6 | 段の順は `matchWorkspaces` → `matchProjects`。親のワークスペースへ移すときはプロジェクトも親のものになるので、ワークスペースが違う子は `matchProjects` に残らない | 06「修復の決まり」は (5) の中の順を定めていない。どの順でも結果は同じで、先にワークスペースを直すほうが行が少ない |
| C3b-7 | 通知の「合う値がなく空になった項目: A・B」は C3b では記録しない。C4 が、修復のコミットの直前の状態に `transfer.Remap` をかけ直して `Emptied` を得る。トラック C の計画の C4 の行に書く(Task 4) | 決定 C3a-18(通知は修復のコミットの件名・行と、対象のログから作る。材料を別に記録しない) |
| C3b-8 | 直す子が Step でもよい(Step が親の Task と別のワークスペースに残ったとき)。`transfer.Collect`(Step を断る)は使わない | 02 の利用者の移動は Step を単独で移せないが、修復は子を親に合わせるもので、Step も子に含まれる |
| C3b-9 | 移動先は、親の `Place.Workspace`・`Place.Project` を `Get` で読む(`Live*` で断らない)。修復の順で (2) が先に、削除済みのワークスペース・プロジェクトの下のアイテムを削除済みにするので、(5) の時点で削除されていない親のワークスペースとプロジェクトは削除されていない | 06「修復の決まり」の順、B5b-6。修復のエラーは同期を止めるので、起きない場合のために断る分岐を足さない |

## Review Focus

1. **Step だけが別のワークスペースに残った:** Task は移ったが、別の人が足した Step は移動元に残った。このとき Step が親の Task のワークスペースへ移り、新しい番号になる(Task 3 の `TestMatchWorkspacesMovesStep`)。
2. **削除済みの下のもの:** 移す子の下に削除済みの Step があっても、同じ保存で一緒に移って番号が振られ、転送記録が残る(Task 3 の `TestMatchWorkspacesMovesDeletedBelow`)。
3. **孫まで残った:** 子と孫が移動元に残ったとき、子が先に小さい番号で移り、孫はそのあとの保存で移る。アクティビティは 2 行(Task 3 の `TestMatchWorkspacesMovesChildThenGrandchild`)。親の親のプロジェクトが削除済みなら、(2) が先に子を削除済みにし、(5) は何もしない(Task 3 の `TestMatchWorkspacesAfterCascade`)。
4. **同じ親の下に直す子が 2 つ:** 同じ深さなら旧い番号の小さい子から移り、先に移った子が小さい新しい番号になる(Task 2 の `TestWorkspaceMismatchPicksSmallerOldNumber`、Task 3 の `TestMatchWorkspacesTwoChildren`)。
5. **同期の順によらない:** A が先でも B が先でも、両方のマシンのファイルが同じになる。修復済みのものを 3 台目が同期しても、何も書かない(Task 4 の `TestRepairScenarioWorkspaceMismatch`。`bothOrders` と `settled`)。

---

### Task 1: `RepairWriter.Store()`

**Files:**
- Modify: `internal/app/port/replicator.go`(`RepairWriter`)
- Modify: `internal/infra/gitstore/sync_repair.go:242-256`(`repairWriter`)
- Modify: `internal/app/apptest/sync.go:113-130`(`RepairWriter`)
- Test: `internal/infra/gitstore/sync_repair_test.go`

**Interfaces:**
- Produces: `port.RepairWriter` のメソッド `Store() (port.StoreID, error)`。修復を書いている Store の ID を返す。

- [ ] **Step 1: 落ちるテストを書く**

`internal/infra/gitstore/sync_repair_test.go` に `TestRepairWriterStore` を足す。Store を `Create(dir, "01STORE0000000000000000001", …)` で作り(同じファイルの既存のテストの組み立てを使う)、`repairWriter{stores: …, root: dir}.Store()` が `"01STORE0000000000000000001"` と nil を返すことを確かめる。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/infra/gitstore/ -run TestRepairWriterStore`
Expected: FAIL(`Store` が定義されていない)

- [ ] **Step 3: 実装する**

`port.RepairWriter` に `Store() (StoreID, error)` を足す(コメント: 「修復を書いている Store の ID。転送記録の to_store に書く」)。gitstore の `repairWriter.Store` は `readStoreYAML(w.root)` の `Store`。apptest の `RepairWriter` は、Store ID のフィールドを `StoreID` に改名し(メソッド `Store` と同名にできないため)、`w.StoreID, nil` を返す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ ./internal/app/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/port/replicator.go internal/infra/gitstore/sync_repair.go internal/infra/gitstore/sync_repair_test.go internal/app/apptest/sync.go
git commit -m "feat(port): 修復から自分の Store ID を読めるようにする(C3b)"
```

### Task 2: 直す子の選び方、一緒に移す削除済みのもの、番号の振り方

**Files:**
- Modify: `internal/domain/repair/place.go`
- Test: `internal/domain/repair/place_test.go`

**Interfaces:**
- Produces:
  - `type Stray struct { Child item.ID; Workspace workspace.ID; Project project.ID }`。直す子と、移す先(親のワークスペースとプロジェクト)。
  - `func WorkspaceMismatch(items []item.Item) (Stray, bool)`。決定 C3b-2 の最初の 1 件。なければ false。
  - `func DeletedBelow(child item.ID, items []item.Item) []item.Item`。child の下で、削除済みのものだけをたどって届く削除済みのもの(決定 C3b-1)。並びは問わない。
  - `func NumberByOld(child item.Item, deleted []item.Item, next int) map[item.ID]int`。決定 C3b-3 の番号(child が next、deleted が旧い番号の順で続き)。

- [ ] **Step 1: 落ちるテストを書く**

`place_test.go` に足す(アイテムはテストの中で `item.Item` を直に組む。ワークスペース W1・W2、プロジェクト P1(W1)・P2(W2))。
- `TestWorkspaceMismatchFindsChildInOtherWorkspace`: 親 E(W2/P2、番号 1)の下の子 T(W1/P1、番号 2)→ `Stray{T, W2, P2}`, true。
- `TestWorkspaceMismatchIgnoresSameWorkspace`: 親と子が同じワークスペースでプロジェクトだけ違う → false(C3a の `ProjectMismatch` の持ち場)。
- `TestWorkspaceMismatchIgnoresDeleted`: 子が削除済み、または親が削除済み → false。
- `TestWorkspaceMismatchPicksShallowest`: T(深さ 1)と、その下の S(深さ 2、T とも別のワークスペース)→ T。
- `TestWorkspaceMismatchPicksSmallerOldNumber`: 同じ親の下で同じ深さの T5(番号 5)と T3(番号 3)→ T3。番号が同じなら内部 ID の小さいほう。
- `TestDeletedBelowFollowsOnlyDeleted`: T の下に削除済みの S1(その下に削除済みの S2)と、削除されていない S3 → S1・S2 だけ(S3 は自分で直す子になる)。
- `TestNumberByOldUsesOldNumbers`: 子 T(番号 7)、削除済みの S9(番号 9、順位が先)・S4(番号 4)のとき、`NumberByOld(T, [S9, S4], 10)` が `{T: 10, S4: 11, S9: 12}`(子は旧い番号が大きくても先)。番号が同じものは内部 ID の順。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/repair/ -run 'WorkspaceMismatch|DeletedBelow|NumberByOld'`
Expected: FAIL(定義されていない)

- [ ] **Step 3: 実装する**

`WorkspaceMismatch` は `ProjectMismatch` と同じ形にする(`item.Live` で子を選び、`depthOf` で深さを数え、`slices.MinFunc` で選ぶ)。比べる順は、深さ → `Place.Number` → 内部 ID。`DeletedBelow` は親から子の表を作り、削除済みの子だけをたどる(一度通ったものは数えない)。`NumberByOld` は `deleted` を写して並べ替え(元のスライスは変えない)、child に `next`、続けて 1 つずつ振る。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/repair/place.go internal/domain/repair/place_test.go
git commit -m "feat(repair): 親とワークスペースが違う子を選び、一緒に移す削除済みのものと番号を決める(C3b)"
```

### Task 3: 修復の段 `matchWorkspaces`

**Files:**
- Create: `internal/app/repairing/workspace.go`
- Modify: `internal/app/repairing/run.go`(`steps` の `matchProjects` の前に `matchWorkspaces`)
- Test: `internal/app/repairing/workspace_test.go`

**Interfaces:**
- Consumes: `repair.WorkspaceMismatch`・`repair.DeletedBelow`・`repair.NumberByOld`(Task 2)、`port.RepairWriter.Store()`(Task 1)、`transfer.Remap`・`transfer.Record`・`numbering.Next`・`numbering.Draft`・`projectrule.FromLog`、`readAllItems`(`cycle.go`)、`syncActor`(`run.go`)
- Produces: `func matchWorkspaces(ctx context.Context, w port.RepairWriter) error`(パッケージ内の段)

- [ ] **Step 1: 落ちるテストを書く**

`workspace_test.go` にテスト用の部品 `addMobile(e *env)` を作る。`newEnv` の Store に、ワークスペース「Mobile」(接頭辞 MB、内部 ID `01SAMPLEWS0000000000000009`)を `workspace.CreateWithSettings(ws, workspace.DefaultSettings(gen))` で足す。`gen` は `01SAMPLEM9V` に 15 桁の連番を続けた値の内部 ID を返す。プロジェクト MB-P1(`01SAMPLEPR0000000000000009`)、Epic MB-1(`01SAMPLEIT0000000000000009`)と番号の行(項目 1・プロジェクト 1)も足す。`project2` と内部 ID が重ならないようにする。もう 1 つの部品 `moveEpicToMobile(e)` は、Epic HH-1 だけを Mobile の MB-P1 へ移す。行は `set place`(番号 2)と、Mobile のワークスペースへの `number 2` で、HH-2 は Web に残る(子が移動元に残った状態)。

- `TestMatchWorkspacesMovesChild`: `moveEpicToMobile` のあと `e.run(nil, nil)`。
  - `wantOps` が `"HH-2 を MB-3 に移動(親に合わせる)"` の 1 件。
  - HH-2 の `Place` が Mobile / MB-P1 / 番号 3 で、親は HH-1 のまま。
  - ステータスは、Review(進行中の分類で、Mobile にはない)から、Mobile の進行中の分類の既定になる。
  - 転送記録が 1 行ある(`Entity: item`、`Target: SampleTask`、`Old: "HH-2"`、`New: "MB-3"`、`ToStore: "S1"`)。
  - Mobile のログに `number 3` の行がある。
  - 修復の行の実行者はどれも同期で、op は 1 つ。
- `TestMatchWorkspacesMovesStep`: `addStep` のあと `moveEpicToMobile`。続けて HH-2 にも `set place`(Mobile / MB-P1 / 番号 3)と、Mobile のワークスペースへの `number 3` を書く。Web に残った Step HH-3 が直り、`HH-3 を MB-4 に移動(親に合わせる)` の 1 件になる。
- `TestMatchWorkspacesMovesChildThenGrandchild`: `addStep` のあと `moveEpicToMobile`。ops は `HH-2 を MB-3 に移動(親に合わせる)`、`HH-3 を MB-4 に移動(親に合わせる)` の順の 2 件で、op が別になる。
- `TestMatchWorkspacesMovesDeletedBelow`: `addStep` のあと Step HH-3 を削除済みにしてから `moveEpicToMobile`。`HH-2 を MB-3 に移動(親に合わせる)` の 1 件で、HH-3 も同じ保存で Mobile / 番号 4 になり、削除済みのまま、転送記録が 2 行になる。
- `TestMatchWorkspacesAfterCascade`: `moveEpicToMobile` のあと、MB-P1 のログに `change.NewDeleteEntity(true)` だけを書く(下のアイテムには書かない)。Run のあと HH-1・HH-2 は削除済みで(修復 (2))、HH-2 は Web に残り、ops に「に移動(親に合わせる)」で終わるものがない。
- `TestMatchWorkspacesTwoChildren`: HH-1 の下に Task を足す(番号 5)。内部 ID は `01AAAAAAAA0000000000000005` にして、内部 ID の順なら HH-2 より先になるようにする。そのあと `moveEpicToMobile` を書く。ops は `HH-2 を MB-3 に移動(親に合わせる)`、`HH-5 を MB-4 に移動(親に合わせる)` の順になる(旧い番号の順)。
- `TestMatchWorkspacesRemovesSprint`: 移す前の HH-2 に `set sprint`(Web のスプリントの内部 ID)を書いておく。直したあと `Sprint` が空になる。

(`e.run` は 2 回目の `Run` が何も書かないことも確かめる。)

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/repairing/ -run MatchWorkspaces`
Expected: FAIL(HH-2 が Web に残る)

- [ ] **Step 3: 実装する**

- `matchWorkspaces` は `matchProjects` と同じ繰り返しにする。読み直す → `repair.WorkspaceMismatch` で 1 件選ぶ → 前と同じ 1 件ならエラー `"親とワークスペースが違う子の修復が進みません"` → `moveToWorkspace(w, s)` で移す。
- `moveToWorkspace` は `w.Read` の中で次を読み、`w.Write` で行を書く(決定 C3b-4)。
  - 移すもの: 直す子と `repair.DeletedBelow`。
  - 移動先のワークスペース・設定・ログ。番号は `numbering.Next(log, numbering.Items)`、ルールは `projectrule.FromLog(log).Of(s.Project)`。
  - 移すもののワークスペースごとの設定と接頭辞。
  - 番号は `repair.NumberByOld`。移動先は親の `Place` を `Get` で読む(決定 C3b-9)。
  - アクティビティの文は、直す子の旧 ID と新 ID で作る。新 ID は移動先の接頭辞と新しい番号で、`transfers` の `displayID` と同じく `{接頭辞}-{番号}`。
- 行の並びは `writeWithin` に合わせる。アイテムごとの行、移動先の番号の行(番号の順)、転送記録(`ToStore` は `w.Store()`)。
- 関数が 60 行を超えるなら、読む部分を `readStrayMove`(返すのは移すものと移動先の材料をまとめた構造体)に分ける。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/repairing/`
Expected: PASS(C3a の `TestProjectMismatchMovesChildren` なども通る)

- [ ] **Step 5: Commit**

```bash
git add internal/app/repairing/workspace.go internal/app/repairing/workspace_test.go internal/app/repairing/run.go
git commit -m "feat(repairing): 親とワークスペースが違う子を、親のワークスペースの次の番号へ移す(C3b)"
```

### Task 4: 2 台のマシンの場面テストと、トラック C の計画の直し

**Files:**
- Create: `cmd/hedgehug/repair_workspace_scenario_test.go`(`//go:build !e2e`)
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(C3b の行・C4 の行)
- Modify: `docs/superpowers/plans/2026-11-14-hedgehug-track-a5a.md:65`(C3b の行)

**Interfaces:**
- Consumes: `bothOrders`・`scenario`・`machine`・`newTask`・`taskB`(`cmd/hedgehug/repair_*_test.go`)、Task 3 の段(`repairing.Run` は `newMachine` が組み込み済み)

- [ ] **Step 1: 落ちるテストを書く**

`TestRepairScenarioWorkspaceMismatch` を書く。
- A は、1 回の保存でワークスペース「Mobile」を足す。このファイルに `addMobile(t, m)` として書く。中身は次のとおり(`sampleProject2` と内部 ID が重ならないようにする)。
  - ワークスペース: 接頭辞 MB、内部 ID `01SAMPLEWS0000000000000009`。`workspace.CreateWithSettings(ws, workspace.DefaultSettings(gen))` で作る。`gen` は `01SAMPLEM9V` に 15 桁の連番を続けた値を返す。
  - プロジェクト MB-P1(`01SAMPLEPR0000000000000009`)
  - Epic MB-1(`01SAMPLEIT0000000000000009`)
  - 番号の行(項目 1・プロジェクト 1)
- A は、続く 1 回の保存で、Epic HH-1 と Task HH-2 を Mobile の MB-P1 へ移す。行は次のとおり(A5a の `writeWithin` と同じ形を生の行で書く)。
  - `set place`: 番号 2・3
  - Mobile への `number 2`・`number 3`
  - 転送記録 2 行: `HH-1→MB-2`、`HH-2→MB-3`
- B は、`newTask(t, m, taskB)` で HH-1 の下に Task HH-3 を Web に作る。
- `bothOrders` の両方の順で、両方のマシンで次を確かめる。
  - taskB の `Place` が Mobile / MB-P1 / 番号 4。
  - 転送記録に `{Target: taskB, Old: "HH-3", New: "MB-4"}` がある。
  - `play` の中でファイルが同じであること、`settled` で何も増えないことも確かめる。

- [ ] **Step 2: 落ちる・通ることを確かめる**

Run: `go test ./cmd/hedgehug/ -run TestRepairScenarioWorkspaceMismatch`
Expected: Task 3 が入っているので PASS。落ちたら、2 台で番号や行が食い違っていないかを見る(決定 C3b-2・C3b-3 は、どちらのマシンが修復しても同じ結果になる)。確かめのために、一時的に `steps` から `matchWorkspaces` を外すと FAIL になることを 1 度見る。見たら戻す(戻したことを差分で確かめる)。

- [ ] **Step 3: トラック C の計画を直す**

`docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` を直す。
- C3b の行の「番号は `transfer.Number`」を、次の文にする。「番号は、親のワークスペースの次の番号。削除されていない子を 1 件ずつ、深さの浅い順・旧い番号の小さい順(同じなら内部 ID の辞書順)に移す(06:314、保存形式 `:206`。C3b の計画の決定 C3b-1〜C3b-3)」。
- `2026-11-14-hedgehug-track-a5a.md:65` の C3b の行の「`transfer.Number`・」を消し、「番号は C3b の計画の決定 C3b-3」と足す。行の頭に計画のファイル名 `2026-11-14-hedgehug-track-c3b.md` を書く。
- C4 の行の範囲に、次の文を足す。「同期の修復 (5) のワークスペースの移動の通知の「。合う値がなく空になった項目: A・B」は、修復のコミットの直前の状態に `transfer.Remap`(people・statusChoice は nil、移動先のルールは親のプロジェクトのもの)をかけ直して `Emptied` から作る(C3b の決定 C3b-7)」。

- [ ] **Step 4: 全体を確かめる**

Run: `make lint && make test && golangci-lint run --build-tags e2e ./...`
Expected: どれもエラーなし

- [ ] **Step 5: Commit**

```bash
git add cmd/hedgehug/repair_workspace_scenario_test.go docs/superpowers/plans/2026-11-12-hedgehug-track-c.md docs/superpowers/plans/2026-11-14-hedgehug-track-a5a.md
git commit -m "test: 親とワークスペースが違う子の修復を 2 台の場面で確かめ、トラック C の計画を直す(C3b)"
```

## このあと

最終レビュー(superpowers:requesting-code-review)のあと、develop 向けに PR を 1 本出す。PR の本文には、決定 C3b-1〜C3b-9 と、トラック C の計画で直した 2 か所を書く。
