# hedgehug トラック B5b(ワークスペースの削除)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ワークスペースの設定の「一般」の「ワークスペースを削除…」と、サイドバーのワークスペースのメニューの「削除…」から、名前を入力する確認ダイアログを通して、ワークスペースとその中のプロジェクト・アイテムを 1 回の保存で論理削除できるようにする。削除済みのワークスペースは画面・操作・名前と接頭辞の重複の検査に出さず、同期の修復は削除済みのワークスペースの下の子を削除済みに入れる。

**Architecture:** 削除は、ワークスペースのログに `delete`(`root: true`)、その中の削除されていないプロジェクトとアイテムに同じ op の `delete`(`root: false`)を書く(保存形式「削除・復元・完全に削除」)。`workspace.Workspace` に `Deleted *change.Deletion` を足し(`change.DeletionOf`)、口は削除済みも返したまま、利用者に見せる・操作する読み方を `port.LiveWorkspace`・`port.LiveWorkspaces` に替える(A4b-3・A4b-4 と同じ形)。修復は、(2) に「削除済みのワークスペースの下の削除されていないプロジェクト」を足し、削除と変更の重なりで、ワークスペースのログの採番の行(`number`)を変更に数えない。画面は core の `features/workspaces/delete.ts` と、名前の入力欄を持つ新しいダイアログ。

**Tech Stack:** Go 1.27.3、huma v2、Svelte 5(`web/app`)、vitest、Playwright。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`「範囲と画面の構成」(サイドバーのメニュー・操作の表の「ワークスペースの削除」)、「名前の重複」(65 行)、「一般」の「削除」(148〜161 行)、「変更が及ぶ範囲」のワークスペースの削除の行、「モックからの変更点」の削除の行
- `04-search-layouts.md` 「ワークスペースのメニュー」(101〜108 行)、432 行(ビューは残る)
- `06-sync-notifications.md` 193・209 行(設定の編集と削除の重なり)、「親が削除されたとき」(320〜324 行)
- `01-overview.md` 149 行(開いている画面がなくなったときの移り先)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「削除・復元・完全に削除」、「同期の修復」の (2) と「削除と変更が重なった」の行
- `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md` の B5b の行と決定 12、`2026-11-13-hedgehug-track-a4b.md`(A4b-1〜A4b-5、Live の読み方)、`2026-11-14-hedgehug-track-c3a.md`(C3a-7・C3a-9・C3a-10)

## Global Constraints

土台とトラック B の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/workspaces`・`app/port`・`app/apptest` は 20 個で満杯。`app/workspaces` は `list.go` を `get.go` に合わせて 1 つ空けてから `delete.go` を足す(Task 2)。`app/port`・`app/apptest` にはファイルを足さない。
- 依存の向き: app の機能どうしは import しない。`domain/repair` は domain だけを import する。
- 依存ライブラリは足さない。足すなら `go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけ(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画やテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 画面の文言・アクティビティの文・断りの文は、仕様 03 の文字どおりに書く。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。`make generate` と `cd web && pnpm gen` で作り直し、同じコミットに入れる。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。e2e はサンプルのワークスペース「Web」を削除しない(同じ worker のほかのテストが使う)。毎回 `createWorkspace` で作ったものを削除する。
- 手元の検証は 1 回で足りる(CI でも流れる)。
- PR はこの小計画で 1 本(向き先 develop、ブランチ `feat/track-b5b`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| B5b-1 | 削除は 1 回の保存で、ワークスペースのログに `change.NewDeleteEntity(true)`、ワークスペースの削除されていないプロジェクトに `NewDeleteEntity(false)`、そのプロジェクトの削除されていないアイテム(Epic から Step まで)に `NewDeleteEntity(false)` を書く。すでに削除済みのプロジェクト・アイテムには書かない(別の操作で削除したものは、ワークスペースを復元しても戻らない)。アクティビティは「ワークスペース「{名前}」を削除」、トーストは「「{名前}」を削除しました」 | 保存形式「削除」(下にあるものにも同じ op)、03 の操作の表、`projects.Delete` と同じ形 |
| B5b-2 | ワークスペースのログの設定(プロパティ・値・テンプレート・自動化・プロジェクト別のルール)には行を足さない。ワークスペースの `delete` の行 1 つで、設定もまとめて削除済みになる | 設定はワークスペースのログの中にあるため。保存形式の「ワークスペースの `log.jsonl` の中にあるもの」 |
| B5b-3 | `workspace.yaml` に `deleted` は足さない | 保存形式の 42 行の workspace.yaml の項目にないため |
| B5b-4 | 削除済みのワークスペースを、次の読み方から除く(`port.LiveWorkspace` は ErrNotFound、`port.LiveWorkspaces` は除いた一覧): `workspaces` の List・Get・Usage・`edit`(設定の変更のすべて)・Rename・ChangePrefix・MoveWorkspace・名前と接頭辞の重複の検査(作成・名前の変更・接頭辞の変更)、`projects` の Create・List、`items.NextNumber`、`automations` の View。作成の rank(`orderedWorkspaces` の最後の rank)は削除済みも含めて計算する(復元したときに並びが重ならないように、rank を使い回さない) | 03「削除」(サイドバー・検索・パレットに出ない)、03「名前の重複」(削除済みは入れない)、トラック B の決定 12。A4b-4 と同じく削除済みは ErrNotFound |
| B5b-5 | 削除済みのワークスペースの下のプロジェクト・アイテムを読むほかの読み方(アイテム・プロジェクトの操作、コメント、関連、GitHub のリンク、自動化の実行)は変えない。B5b-1 で中のプロジェクト・アイテムも削除済みになるので、`LiveProject`・`LiveItem` の段で ErrNotFound になる。同期の前に別のマシンで足された子は、修復 (2)(B5b-6)で削除済みになる | Explore の調べ(下流は LiveProject・LiveItem を先に通る) |
| B5b-6 | 修復 (2) に、削除済みのワークスペースの下の削除されていないプロジェクトを足す。プロジェクトに `NewDeleteEntity(false).InOp(ワークスペースの削除の op)` を書く(1 件ごとに 1 回の保存、文は「{HH-P1} を削除」)。その下のアイテムは、次の回で既存の `repair.Cascades`(プロジェクトが削除済み)が拾う | 06「親が削除されたとき」(ワークスペースの子も一緒に削除済み)。C3a-10 でトラック B に渡された分 |
| B5b-7 | 削除と変更の重なりで、ワークスペースのログの採番の行(`numbering.Type` = `"number"`)を変更に数えない(`repairing/restore.go` の `incidentals.of`)。採番の行は、プロジェクトやアイテムを作ったときに一緒に書く行で、子の追加は B5b-6 で削除済みに入れるため。名前・接頭辞・並び・設定(プロジェクト別のルールを含む)の変更は、これまでどおり変更に数え、ワークスペースの削除を取り消す | 06 193・209 行(設定の編集と削除が重なったら削除を取り消す)と 322 行(子の変更・追加は子も削除済み)の両立 |
| B5b-8 | 削除と変更の重なりの取り消しのアクティビティは「ワークスペース「{名前}」の削除を取り消し」 | 06 514 行の「{ID} の削除を取り消し」の {ID} に、ワークスペースの呼び方(03 の操作の表の「ワークスペース「X」」)を当てた |
| B5b-9 | 確認ダイアログ「ワークスペースを削除」: 本文 1 行目「「{名前}」と、その中の {N} 件のプロジェクト・{M} 件のアイテム、スプリント・リリース・設定を削除します。」、2 行目「設定の「削除済み」から戻せます。{保持日数} 日を過ぎると完全に削除されます。」。その下に名前の入力欄(ラベル「確認のため、ワークスペースの名前を入力してください」)。ボタン「削除」(警告色)は常に押せ、入力の前後の空白を除いた値が名前と完全に一致しないときは、押すとダイアログの中に「名前が一致しません」と出して削除しない。N・M は削除されていないものの数 | 03「削除」。前後の空白を除いて完全一致で比べるのは 03「名前の重複」の比べ方 |
| B5b-10 | 名前の一致はサーバーでは確かめない(API は削除だけ)。確認の入力は画面の誤操作よけ | 03「削除」は画面の確認として書かれている |
| B5b-11 | 入口は 2 つ。「一般」の末尾の区画のボタン「ワークスペースを削除…」(警告色)と、サイドバーのワークスペースのメニューの末尾の「削除…」(警告色)。閲覧のみの Store では、どちらも無効にして「閲覧のみの Store です」と添える。どちらも同じ確認ダイアログを開く | 03「範囲と画面の構成」・「削除」、04 のメニューの表 |
| B5b-12 | 削除できたあと: いまの画面がそのワークスペース(概要・設定)か、その中のプロジェクト・アイテムなら、ホームへ replace で移る(マイタスクの画面はまだないので、ホームがその位置。01 149 行)。開いている詳細パネルのアイテムがそのワークスペースのものなら閉じる。そのあとサイドバーの一覧を読み直す。ほかの画面にいるときは移らない。予告や削除が ErrNotFound(すでに削除済み)のときは、トーストなしで同じ移り方をする | 03「削除」の 1 つ目の箇条、`leaveDeletedProject` と同じ順(先に移る) |
| B5b-13 | API: `GET /api/stores/{store}/workspaces/{workspace}/delete-preview`(`get-workspace-delete-preview`、本文 `{projects, items, retentionDays}`)と `DELETE /api/stores/{store}/workspaces/{workspace}`(`delete-workspace`、204) | プロジェクトの削除の API と同じ形 |

## 他のトラックとの合わせ(develop にあとから入る方が合わせる)

- **C3a-screen**(他の画面で削除されたものへの保存): S9 の「プロジェクトは、ワークスペースが読めることだけを確かめる」を、`port.LiveWorkspace` で削除済みのワークスペースなら取り消さず 404 にする(08「プロジェクト…ワークスペースが削除済み」の復元の断りと同じ)。ワークスペースの設定の変更(`workspaces` の `edit`・Rename・ChangePrefix)を、削除済みのワークスペースに対して保存したときは、06 185 行のとおり変更を残してワークスペースの削除の op を取り消し、トーストの {ID} を「ワークスペース「{名前}」」にする。B5b の間は ErrNotFound(B5b-4)。
- **A5a・A5b**(別のワークスペースへの移動): 移動先の候補のワークスペースは `port.LiveWorkspaces` で読み、移動先が削除済みなら ErrNotFound。
- **F1**(スプリントとリリース): スプリント・リリースの読み書きは `port.LiveWorkspace` を通す。スプリント・リリースの行はワークスペースのログにあるので、B5b-7 と同じく、ワークスペースの削除との重なりでは変更に数えない(ワークスペースの中身で、06 322 行の子にあたる)。`incidentals.of` に F1 の項目を足す。
- **C4**(通知・受信箱): 06 455 行の「削除されたワークスペース」の表示と、修復 (2) の通知「{親の ID} が削除されたため、{子の ID} も削除済みに入りました」で親がワークスペースのときの {親の ID} は「ワークスペース「{名前}」」。
- **G**(削除済み・復元): 削除済みの一覧はワークスペースの行だけを出す(op が同じプロジェクト・アイテムは出さない)。復元は、ワークスペースの削除の op の `restore` の行を、その op の行があるログごとに書く(C3a-7)。名前・接頭辞が重なるときの復元の断り(08)は G。
- **E**(検索・パレット・ビュー): ワークスペースの一覧は `port.LiveWorkspaces` で読む。保存したビューは消さない(04 432 行)。

---

## ファイル構成

```
internal/domain/workspace/workspace.go        Workspace.Deleted、Live
internal/app/port/stores.go                   LiveWorkspace・LiveWorkspaces
internal/app/apptest/stores_contract_values.go contractDeleted にワークスペースを足す
internal/app/apptest/sample.go                DeleteWorkspace(テスト用の補助)
internal/app/workspaces/get.go                list.go の List を移す(list.go は消す)
internal/app/workspaces/delete.go             (新規)DeletePreview・Delete
internal/app/workspaces/{edit,rename,prefix,create,order,usage}.go  Live の読み方
internal/app/projects/{create,list}.go        Live の読み方
internal/app/items/numbers.go                 NextNumber の Live
internal/app/automations/view.go              View の Live
internal/domain/repair/cascade.go             ProjectCascades
internal/app/repairing/cascade.go             プロジェクトの修復 (2)
internal/app/repairing/restore.go             採番の行を数えない、取り消しの文
internal/adapter/httpapi/workspaces/usage.go  delete-preview と DELETE
web/packages/core/src/model/workspace.ts      WorkspaceDeletePreview
web/packages/core/src/port/workspaces.ts      workspaceDeletePreview・deleteWorkspace
web/packages/api/src/workspaces.ts            同上の実装
web/packages/core/src/features/workspaces/delete.ts  (新規)ダイアログの中身と流れ
web/packages/core/src/features/workspaces/sidebarMenu.ts 「削除…」
web/app/src/screens/workspace-settings/DeleteWorkspaceDialog.svelte (新規)
web/app/src/screens/workspace-settings/GeneralPanel.svelte  削除の区画
web/app/src/shell/WorkspaceMenu.svelte        「削除…」
web/app/src/wireActions.ts                    配線
web/e2e/workspace-delete.spec.ts              (新規)
```

---

### Task 1: 削除済みの印と Live の読み方

**Files:**
- Modify: `internal/domain/workspace/workspace.go`、`internal/app/port/stores.go`、`internal/app/apptest/stores_contract_values.go`(`contractDeleted`)、`internal/app/apptest/sample.go`
- Test: `internal/domain/workspace/workspace_test.go`、`internal/app/port/stores_test.go`

**Interfaces:**
- Produces:
  - `workspace.Workspace.Deleted *change.Deletion`(`FromLog` で `change.DeletionOf(log)`)
  - `workspace.Live(ws []Workspace) []Workspace`(削除済みを除く。順はそのまま)
  - `port.LiveWorkspace(r Reader, id workspace.ID) (workspace.Workspace, error)`(ないか削除済みなら ErrNotFound)
  - `port.LiveWorkspaces(r Reader) ([]workspace.Workspace, error)`(内部 ID の順)
  - `apptest.DeleteWorkspace(ctx context.Context, stores port.Stores, store port.StoreID, ws workspace.ID) error`(ワークスペースのログに `NewDeleteEntity(true)` だけを書く。中のものには書かない。ほかのパッケージのテストで、削除済みのワークスペースを作るため)

- [ ] **Step 1: 失敗するテストを書く**

`workspace_test.go`:
- `TestFromLogMarksDeleted`: create の行のあとに `NewDeleteEntity(true)` の行(op `o1`)を足したログ → `Deleted != nil`、`Deleted.Op == "o1"`、`Deleted.Root == true`。さらに `{"t":"restore","f":"deleted","to":"o1"}` の行を足す → `Deleted == nil`。
- `TestLiveSkipsDeleted`: 3 件のうち真ん中が削除済み → 残り 2 件が元の順。

`stores_test.go`:
- `TestLiveWorkspace`: 削除済み → `errors.Is(err, port.ErrNotFound)`。生きている → 返る。`LiveWorkspaces` は削除済みを含まない。

`contractDeleted` に足す: ワークスペースに `delete` の行を書くと、`Workspaces().Get` は `Deleted` 付きで返り、`List` にも含まれ、`Settings` も読める(口は削除済みも返す。A4b-3)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/workspace/ ./internal/app/port/ ./internal/infra/... ./internal/app/apptest/...`
Expected: FAIL(`Deleted` がない)

- [ ] **Step 3: 実装する**

`port/stores.go` の Live の方針のコメント(48〜51 行)に、ワークスペースも同じであることを 1 行足す。

- [ ] **Step 4: 通ることを確かめる**

Run: 同上
Expected: PASS(gitstore とメモリの両方の契約テスト)

- [ ] **Step 5: コミット**

```bash
git commit -m "feat: ワークスペースに削除済みの印を持たせ、削除済みを除いて読む口を足す"
```

### Task 2: ワークスペースの削除の操作

**Files:**
- Modify: `internal/app/workspaces/get.go`(`list.go` の `List` をここへ移し、`list.go` を消す)
- Create: `internal/app/workspaces/delete.go`
- Test: `internal/app/workspaces/delete_test.go`(新規)

**Interfaces:**
- Consumes: Task 1 の `port.LiveWorkspace`、`port.LiveProjects`、`port.LiveItems`
- Produces:
  - `type DeletePreview struct { Projects, Items, RetentionDays int }`
  - `func (s Service) DeletePreview(ctx context.Context, store port.StoreID, ws workspace.ID) (DeletePreview, error)`
  - `func (s Service) Delete(ctx context.Context, store port.StoreID, ws workspace.ID) error`
  - どちらも、ないか削除済みなら `port.ErrNotFound`

- [ ] **Step 1: 失敗するテストを書く**

- `TestDeletePreviewCountsLiveProjectsAndItems`: プロジェクト 3 件(うち 1 件は `apptest.DeleteProject` で削除済み)、生きているプロジェクトにアイテム 2 件と 1 件、うち 1 件を `apptest.DeleteItems` で削除済み → `Projects == 2`、`Items == 2`、`RetentionDays == 30`。
- `TestDeleteWritesRowsInOneOperation`: 削除のあと、ワークスペースのログの最後の行が `t: delete, f: deleted, root: true`、生きていたプロジェクト・アイテムの最後の行が `root: false` で、op が 3 つとも同じ。前に削除済みだったプロジェクト・アイテムのログには行が増えない。アクティビティの文は `ワークスペース「Web」を削除`(名前の改行と続いた空白は 1 つの空白にする)。
- `TestDeleteRefusesWhatIsGone`: 2 回目の Delete と DeletePreview は `port.ErrNotFound`。ない ID も同じ。
- `TestDeleteKeepsSettings`: 削除のあと `Workspaces().Settings` は削除前と同じ値(B5b-2)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/workspaces/ -run 'TestDelete'`
Expected: FAIL(`Delete` がない)

- [ ] **Step 3: 実装する**

`projects.Delete`(`internal/app/projects/delete.go`)と同じ形: 先に Read で名前を読み、Write の中で読み直して行を書く。名前の 1 行化は `projects` の `label` と同じ処理を `workspaces` の中に持つ(機能どうしは import しない)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/workspaces/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git commit -m "feat: ワークスペースと中のプロジェクト・アイテムを 1 回の保存で削除する操作を足す"
```

### Task 3: 削除済みのワークスペースを画面と操作から除く

**Files:**
- Modify: `internal/app/workspaces/{get,usage,edit,rename,prefix,create,order}.go`、`internal/app/projects/{create,list}.go`、`internal/app/items/numbers.go`、`internal/app/automations/view.go`
- Test: `internal/app/workspaces/deleted_test.go`、`internal/app/projects/deleted_test.go`、`internal/app/items/deleted_test.go`、`internal/app/automations/deleted_test.go`(どれもあるファイルに足す。なければ作る)

**Interfaces:**
- Consumes: Task 1 の `port.LiveWorkspace`・`port.LiveWorkspaces`・`apptest.DeleteWorkspace`

`order.go` の `orderedWorkspaces` は、削除済みも含める版(作成の rank 用)と、含めない版(サイドバーの List と MoveWorkspace の隣の判定)に分ける(B5b-4)。

- [ ] **Step 1: 失敗するテストを書く**

`workspaces/deleted_test.go`:
- `TestDeletedWorkspaceIsHiddenAndCannotBeChanged`: ワークスペース「A」「B」「C」を作り、「B」を `apptest.DeleteWorkspace` で削除 → `List` は A・C だけ。`Get`・`Usage`・`Rename`・`ChangePrefix`・`AddProperty`(`edit` を通る代表)・`MoveWorkspace` は「B」に対して `port.ErrNotFound`。`MoveWorkspace(C, up)` で C が A の上に来る(B を飛ばす)。
- `TestDeletedWorkspaceNameAndPrefixCanBeReused`: 名前「Web」・接頭辞「HH」を削除したあと、`Create(ctx, store, "Web", "HH")` が通る。別のワークスペースの `Rename` で「Web」、`ChangePrefix` で「HH」にもできる。
- `TestCreateAfterDeleteDoesNotReuseRank`: A・B を作って B を削除し、C を作る → C の rank は B の rank より大きい(作成の rank は削除済みも含めて計算する)。

`projects/deleted_test.go`:
- `TestProjectsOfDeletedWorkspace`: 削除済みのワークスペースに `Create` は `port.ErrNotFound`、`List` も `port.ErrNotFound`。

`items/deleted_test.go`:
- `TestNextNumberOfDeletedWorkspace`: 削除済みのワークスペースの `NextNumber` は `port.ErrNotFound`。

`automations/deleted_test.go`:
- `TestViewOfDeletedWorkspace`: 削除済みのワークスペースの `View` は `port.ErrNotFound`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/workspaces/ ./internal/app/projects/ ./internal/app/items/ ./internal/app/automations/`
Expected: FAIL

- [ ] **Step 3: 実装する**

`r.Workspaces().Get(ws)` を `port.LiveWorkspace(r, ws)` に、重複の検査とサイドバーの `List()` を `port.LiveWorkspaces(r)` に替える。`edit.go` は Read と Write の中の両方で、`Settings` の前に `port.LiveWorkspace` を通す。`automations` の `View` は、最初の読み取りの前に `port.LiveWorkspace` を通す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git commit -m "feat: 削除済みのワークスペースを一覧・設定・作成・重複の検査から除く"
```

### Task 4: 同期の修復(削除済みのワークスペースの下の子・設定の変更との重なり)

**Files:**
- Modify: `internal/domain/repair/cascade.go`、`internal/app/repairing/cascade.go`、`internal/app/repairing/restore.go`、`internal/app/repairing/snapshot.go`(ワークスペースの display)
- Test: `internal/domain/repair/cascade_test.go`、`internal/app/repairing/cascade_test.go`、`internal/app/repairing/restore_test.go`

**Interfaces:**
- Consumes: Task 1 の `Workspace.Deleted`
- Produces:
  - `type repair.ProjectCascade struct { Project project.ID; Op change.ID }`
  - `func repair.ProjectCascades(wss []workspace.Workspace, projects []project.Project) []ProjectCascade`(削除されていないプロジェクトで、ワークスペースが削除済みのもの。op はワークスペースの削除の op。プロジェクトの内部 ID の順)

`cascadeDeleted` は、1 回の読みで `ProjectCascades` と `Cascades` の両方を読み、プロジェクトを先に書く。進まない検査(`slices.Equal(cs, prev)`)は、プロジェクトとアイテムの両方の一覧で比べる。

- [ ] **Step 1: 失敗するテストを書く**

`domain/repair/cascade_test.go`:
- `TestProjectCascadesUnderDeletedWorkspace`: 削除済みのワークスペース W(op `o9`)に、生きているプロジェクト P1・削除済みの P2、生きているワークスペースに P3 → `[{P1, o9}]` だけ。

`repairing/cascade_test.go`:
- `TestCascadeUnderDeletedWorkspace`: 削除済みのワークスペースの下に、生きているプロジェクト(HH-P1)と、その下の生きている Epic と Task(Epic の子)がある → Run のあと、3 つとも `Deleted.Op` がワークスペースの削除の op で `Root == false`。アクティビティに「HH-P1 を削除」「HH-1 を削除」「HH-2 を削除」が順に出る(実行者は同期)。

`repairing/restore_test.go`(既存の 2 つの側を作る補助を使う):
- `TestWorkspaceDeleteUndoneBySettingsChange`: 一方がワークスペースを削除(Task 2 の `Delete` と同じ行)、もう一方が同時にプロパティを追加 → Run のあと、ワークスペース・中のプロジェクト・アイテムの削除が取り消され(`Deleted == nil`)、アクティビティの文は「ワークスペース「Web」の削除を取り消し」。
- `TestWorkspaceDeleteKeptWhenChildIsAdded`: 一方がワークスペースを削除、もう一方が同時にそのワークスペースにアイテムを作る(採番の行がワークスペースのログに入る) → ワークスペースの削除は取り消さず、作ったアイテムも削除済み(op はワークスペースの削除の op)。
- `TestWorkspaceDeleteKeptWhenProjectIsAdded`: もう一方が同時にプロジェクトを作る → ワークスペースの削除は残り、作ったプロジェクトとその下が削除済み。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `restore.go` の `incidentals.of`: `ref.Kind == change.KindWorkspace && c.T == numbering.Type` を一緒に書いた行とする(B5b-7)。関数のコメントに理由を 1 行足す。
- `summary`: `f.F == change.DeletedField` でワークスペースのログのときは `"ワークスペース「" + 名前 + "」の削除を取り消し"`(名前はログから `workspace.FromLog` で読み、改行と続いた空白を 1 つにする)。アイテム・プロジェクトはこれまでどおり。
- `snapshot.go` の `entity.display` のコメント(「ワークスペースは空」)は、summary がワークスペースを別に扱うので変えなくてよい。変えるならコメントだけ。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/ ./internal/infra/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git commit -m "feat: 同期の修復で、削除済みのワークスペースの下の子を削除済みに入れ、子の追加では削除を取り消さない"
```

### Task 5: HTTP の口と画面の port

**Files:**
- Modify: `internal/adapter/httpapi/workspaces/usage.go`(2 つの操作を足す)、`dto.go`(`WorkspaceDeletePreview`)、`api/openapi.yaml`(生成)、`web/packages/api/src/generated.ts`(生成)、`web/packages/core/src/model/workspace.ts`、`web/packages/core/src/port/workspaces.ts`、`web/packages/api/src/workspaces.ts`
- Test: `internal/adapter/httpapi/workspaces/workspaces_test.go`、`web/packages/api/src/workspaces.test.ts`(あれば。なければ近いテストのファイル)

**Interfaces:**
- Produces:
  - `GET /api/stores/{store}/workspaces/{workspace}/delete-preview` → 200 `{projects: number, items: number, retentionDays: number}`(operationId `get-workspace-delete-preview`)
  - `DELETE /api/stores/{store}/workspaces/{workspace}` → 204(operationId `delete-workspace`)
  - どちらも削除済み・ないものは 404(`apierr.From`)
  - core: `interface WorkspaceDeletePreview { projects: number; items: number; retentionDays: number }`、`WorkspacesPort.workspaceDeletePreview(storeId: string, workspaceId: string): Promise<WorkspaceDeletePreview>`、`WorkspacesPort.deleteWorkspace(storeId: string, workspaceId: string): Promise<void>`(404 は core の `NotFound`)

- [ ] **Step 1: 失敗するテストを書く**

Go: `TestDeleteWorkspaceAPI`: 予告が `{"projects":1,"items":2,"retentionDays":30}`(サンプルのワークスペース)、DELETE が 204、そのあと予告・DELETE・`GET /workspaces/{workspace}` が 404、`GET /workspaces` に出ない。
TS: `workspaceDeletePreview` と `deleteWorkspace` が正しいパスとメソッドを呼び、404 を `NotFound` にする。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/workspaces/` と `cd web && pnpm -F @hedgehug/api test`(パッケージ名は `web/packages/api/package.json` のもの)
Expected: FAIL

- [ ] **Step 3: 実装し、`make generate` と `cd web && pnpm gen` で生成物を作り直す**

- [ ] **Step 4: 通ることを確かめる**

Run: 同上
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git commit -m "feat: ワークスペースの削除の予告と削除の API を足す"
```

### Task 6: 削除の流れ(core)

**Files:**
- Create: `web/packages/core/src/features/workspaces/delete.ts`、`delete.test.ts`
- Modify: `web/packages/core/src/features/workspaces/sidebarMenu.ts`(と test)、`web/packages/core/src/actionsIndex.ts`(公開)

**Interfaces:**
- Consumes: Task 5 の port
- Produces:
  - `type DeleteWorkspaceTarget = { storeId: string; id: string; name: string; prefix: string }`
  - `workspaceDeleteDialog(ws: DeleteWorkspaceTarget, p: WorkspaceDeletePreview): { workspace: DeleteWorkspaceTarget; title: string; lines: string[]; label: string; ok: string }`
  - `workspaceDeletedMessage(name: string): string` → `「${name}」を削除しました`
  - `nameMismatch = "名前が一致しません"`
  - `createWorkspaceDelete(deps: { actions: Pick<WorkspacesPort, "workspaceDeletePreview" | "deleteWorkspace">; catalog: Catalog; toasts: Toasts; afterDelete(ws: DeleteWorkspaceTarget): Promise<void> }): WorkspaceDelete`
  - `interface WorkspaceDelete { state: Readable<(dialog の中身 & { error: string | null }) | null>; open(ws): Promise<void>; confirm(typed: string): Promise<void>; cancel(): void }`
  - `leaveDeletedWorkspace(deps: { navigation: Pick<Navigation, "route" | "replace">; catalog: Pick<Catalog, "stores" | "load">; detail: { view: Readable<...>; close(): void } }, ws: DeleteWorkspaceTarget): Promise<void>`(B5b-12)
  - サイドバーのメニューの項目 id `"delete"`、文言「削除…」、閲覧のみなら `disabled` と `hint: readOnlyNote`

`createWorkspaceDelete` は `createProjectDelete`(`features/projects/delete.ts`)と同じ世代(seq)と busy の扱いにする。`confirm(typed)` は `typed.trim() !== name` なら `error` に `nameMismatch` を入れて何もしない。入力が変わったら画面が `error` を消せるよう、`clearError(): void` も持つ。

`leaveDeletedWorkspace` の「いまの画面がそのワークスペースのものか」: ルートの store がそのワークスペースの Store の名前で、`workspace` ルートなら `prefix` が一致、`project`・`item` ルートなら ID が `${prefix}-` で始まる。詳細パネルは、開いているアイテムの `storeId` と `workspaceId` が一致すれば閉じる。

- [ ] **Step 1: 失敗するテストを書く**

- `workspaceDeleteDialog`: 名前「Web」、`{projects: 2, items: 5, retentionDays: 30}` → `title: "ワークスペースを削除"`、`lines: ["「Web」と、その中の 2 件のプロジェクト・5 件のアイテム、スプリント・リリース・設定を削除します。", "設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。"]`、`label: "確認のため、ワークスペースの名前を入力してください"`、`ok: "削除"`。
- `confirm("Wb")` → `deleteWorkspace` を呼ばず、`state.error === "名前が一致しません"`。`confirm(" Web ")` → 呼ぶ、トースト「「Web」を削除しました」、`afterDelete` を 1 回呼ぶ、`state` は null。
- 予告が `NotFound` → ダイアログを出さず `afterDelete`。削除が `NotFound` → トーストなしで `afterDelete`。ほかのエラー → エラーのトースト、`afterDelete` を呼ばない。
- 閲覧のみの Store では `open` が何もしない。
- `leaveDeletedWorkspace`: ルートが `/s/e2e/w/HH/settings`・`/s/e2e/p/HH-P1/overview`・`/s/e2e/i/HH-12` → ホームへ replace。`/s/e2e/w/WEB`(別のワークスペース)とホーム → 移らない。どちらも `catalog.load` は呼ぶ。開いている詳細パネルが同じワークスペースのアイテム → `close`、別なら閉じない。
- `sidebarMenu`: 末尾に「削除…」。閲覧のみなら無効で「閲覧のみの Store です」。

- [ ] **Step 2: 落ちることを確かめる**

Run: `cd web && pnpm -F @hedgehug/core test`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: 同上
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git commit -m "feat: ワークスペースの削除の確認と、削除のあとの移り先を core に足す"
```

### Task 7: 画面(ダイアログ・一般・サイドバーのメニュー)と e2e

**Files:**
- Create: `web/app/src/screens/workspace-settings/DeleteWorkspaceDialog.svelte`、`web/e2e/workspace-delete.spec.ts`
- Modify: `web/app/src/screens/workspace-settings/GeneralPanel.svelte`、`web/app/src/shell/WorkspaceMenu.svelte`、`web/app/src/wireActions.ts`(ダイアログを開いたまま画面を移ったら `cancel` するのは、プロジェクトの削除と同じ)、ダイアログを置く場所(プロジェクトの削除のダイアログと同じ置き方)

**Interfaces:**
- Consumes: Task 6 の `createWorkspaceDelete`・`leaveDeletedWorkspace`

`DeleteWorkspaceDialog.svelte` は ui の `Dialog`・`Button` で作る(`ConfirmDialog` は入力欄を持たないので使わない)。本文の行、ラベル付きの入力欄、`error` があれば入力欄の下に出す(`role="alert"`)、「キャンセル」と「削除」(警告色)。入力欄で Enter は「削除」と同じ。入力が変わったら `clearError`。

GeneralPanel の末尾に削除の区画を置き、ボタン「ワークスペースを削除…」(警告色)。閲覧のみの Store では `disabled` と `title={readOnlyNote}`(名前・接頭辞の欄と同じ)。

- [ ] **Step 1: 失敗する e2e を書く**

`workspace-delete.spec.ts`:
- `deletes a workspace from the general settings after typing its name`: `createWorkspace` で作り、`projectWithEpic` でプロジェクトと Epic を足す。設定の「一般」で「ワークスペースを削除…」→ ダイアログ「ワークスペースを削除」に「「{名前}」と、その中の 1 件のプロジェクト・1 件のアイテム、スプリント・リリース・設定を削除します。」。違う名前を入れて「削除」→「名前が一致しません」が出て、ダイアログは開いたまま。正しい名前を入れて「削除」→ トースト「「{名前}」を削除しました」、ホーム(見出し「hedgehug」)へ移り、サイドバーからそのワークスペースが消える。そのワークスペースのアドレス `/s/e2e/w/{接頭辞}` はホームを開く。同じ名前と接頭辞で `createWorkspace` がもう一度できる。
- `opens the same dialog from the sidebar menu`: 作ったワークスペースのサイドバーのメニューの「削除…」→ 同じダイアログ。「キャンセル」で閉じ、ワークスペースは残る。
- `disables deleting in a read-only store`: 閲覧のみの Store のワークスペースのメニューの「削除…」が無効。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e`(または `make e2e-bin && cd web && pnpm e2e workspace-delete`)
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `make check` と、上の e2e
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git commit -m "feat: ワークスペースの設定とサイドバーのメニューからワークスペースを削除できるようにする"
```

### Task 8: トラック B の計画の表を直す

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(B5b の行)

- [ ] **Step 1:** B5b の行の範囲に「同期の修復の、削除と変更の重なり(採番の行を変更に数えない。B5b-7)」を足し、完成の条件に「削除済みのワークスペースの下の子は、同期の修復で削除済みに入る」を足す。
- [ ] **Step 2: コミット**

```bash
git commit -m "docs: トラック B の計画の B5b の行に、同期の修復の重なりの扱いを足す"
```

## Review Focus

1. **削除済みのワークスペースと同じ接頭辞のワークスペースを作ったとき**: 画面のアドレス(`/s/e2e/w/HH`)、`items.FindByNumber`、GitHub の自動リンクの表(`githublink/autolink_find.go`)が、生きている方を引く。サーバーで接頭辞からワークスペースを引くところは `LiveWorkspaces` を通すか、削除済みのプロジェクト・アイテムしか持たないので結果に出ないことを確かめる(Task 3 の `TestDeletedWorkspaceNameAndPrefixCanBeReused` に、作り直したワークスペースで `NextNumber` が 1 から始まることと、`FindByNumber` が新しい方を引くことを足す)。
2. **削除済みのワークスペースの設定画面を開いたままの別のタブ**: 設定の保存は 404 になり、画面はいまの保存の失敗と同じエラーのトーストを出す(削除の取り消しは C3a-screen。「他のトラックとの合わせ」)。Task 6 の `leaveDeletedWorkspace` は削除した本人のタブだけ。Task 7 の e2e に、2 つ目のページで設定を開いたまま 1 つ目のページで削除し、2 つ目のページで名前を保存するとエラーのトーストが出て、再読み込みでホームが開くことを足す。
3. **同期で子の追加と削除が重なったとき**: Task 4 の 2 つのテストで、子の追加ではワークスペースの削除を取り消さず、設定の変更では取り消す。
4. **確認の入力の前後の空白**: 「 Web 」は一致、「web」(大文字と小文字が違う)は不一致(Task 6 のテストに「web」を足す)。
5. **ワークスペースが 1 つだけの Store**: 最後の 1 つを削除できる(Store はワークスペース 0 件でも Store。保存形式「store.yaml」)。サイドバーの Store の枠は空のまま出る(Task 7 の e2e で作った Store ではなく、Task 2 のテストで最後の 1 つを削除できることを足す)。
