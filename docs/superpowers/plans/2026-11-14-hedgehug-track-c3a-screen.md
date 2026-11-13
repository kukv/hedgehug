# hedgehug トラック C3a-screen(他のタブで削除されたものへの保存)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 別のブラウザのタブ(や別のプログラム)で削除されたアイテム・プロジェクトに、このタブから変更を保存したとき、断らずに変更を保存して削除を取り消し、トースト「{ID} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました」を出す。

**Architecture:** usecase の読みを、削除済みを断る `port.LiveItem`・`port.LiveProject` から、`root: true` の削除済みも返す `port.EditableItem`・`port.EditableProject` に替える。保存(tx)の最初に `port.ReviveItem`・`port.ReviveProject` を呼び、削除の op を持つすべてのログ(プロジェクトとそのアイテム)に `change.Undo` の行(C3a の `restore`)を、変更と同じ 1 回の保存で書く。取り消したものの表示 ID は context の入れ物に積み、`httpapi` のミドルウェアが成功した応答のヘッダー `Hedgehug-Restored` に載せる。画面は `fetcher` で 1 か所だけヘッダーを読み、トーストを出す。

**Tech Stack:** Go 1.27.3(huma v2、net/http)、TypeScript・Svelte(web/packages/api・core、web/app)、Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「他の書き手との重なり」(トーストの文)、「同時に編集したとき」の表の最後の行、「親が削除されたとき」、「反映を待つとき」
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「同期の修復」の「順の指定なし: 削除と変更が重なった」の行(`root: false` は対象にしない・op 全体の `restore`)、「テスト」の「親と一緒の削除」、「削除・復元・完全に削除」の「復元」
- 08「復元の単位」(一緒に削除された子も戻る)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-c3a.md` の決定 C3a-7(`restore` の行)・C3a-8(`change.Undo`)・C3a-9(変更に数える行)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C3a-screen の行(完成の条件)

## Global Constraints

土台とトラック C の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個。`app/port` は 20 個で満杯なので、Task 1 で `ids.go` を `identity.go` へ移してから `revive.go` を足す。`app/items` は 19 個なので、ファイルを足さない。`web/app/src/wire.ts` は 275 行なので触らない(配線は `wireBase.ts`)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- トーストの文は仕様のとおり: `{ID} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`。`{ID}` はアイテムなら `HH-12`、プロジェクトなら `HH-P3`(`DisplayID(ワークスペースの接頭辞)`)。
- 応答ヘッダーの名前は `Hedgehug-Restored`。値は表示 ID 1 つ(取り消したものごとに `Header().Add`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・この計画で足した e2e の spec。

## 取り消す操作と取り消さない操作

規則(決定 S1): 保存で、削除済み(`root: true`)のアイテム・プロジェクトの**自分のログに、実行者「あなた」の行を書く**操作は、削除を取り消す。C3a-9 が同期で「変更」に数える行と同じなので、同期と画面で結果がそろう。

| 経路(`/api/stores/{store}/…`) | 対象 | 扱い | Task |
|---|---|---|---|
| `PUT items/{item}/title` | アイテム | 取り消す | 2 |
| `PUT items/{item}/kind` | アイテム | 取り消す | 2 |
| `POST items/{item}/reorder` | アイテム | 取り消す(基準の兄弟が削除済みなら、いまのまま 404) | 2 |
| `POST items/{item}/move` | 移すアイテム | 取り消す(移動先の親・プロジェクトが削除済みなら、いまのまま 404) | 2 |
| `PUT items/{item}/properties/{p}` | アイテム | 取り消す | 3 |
| `POST`・`DELETE items/{item}/properties/{p}/values/{v}` | アイテム | 取り消す | 3 |
| `POST items/{item}/check` | Step | 取り消す | 3 |
| `POST items/{item}/automation/{rule}/apply` | アイテム | 取り消す | 3 |
| `PUT items/{item}/description` | アイテム | 取り消す | 4 |
| `POST items/{item}/comments`、`PUT`・`DELETE …/comments/{c}` | アイテム | 取り消す | 4 |
| `POST items/{item}/worklogs`、`DELETE …/worklogs/{w}` | アイテム | 取り消す | 4 |
| `PUT items/{item}/watch` | アイテム | 取り消す | 4 |
| `POST items/{item}/relations`、`…/relations/remove` | 関連を足す・外す側のアイテム | 取り消す(相手が削除済みなら、いまのまま 404。決定 S5) | 5 |
| `POST items/{item}/links`、`…/links/remove` | アイテム | 取り消す(プロジェクトが削除済みなら、いまのまま 404) | 5 |
| `PUT projects/{p}/title`・`status`・`owner`・`date` | プロジェクト | 取り消す | 6 |
| `POST projects/{p}/repos`、`…/repos/remove` | プロジェクト | 取り消す | 6 |
| `PUT projects/{p}/description` | プロジェクト | 取り消す | 4 |
| `POST projects/{p}/comments`、`PUT`・`DELETE …/comments/{c}` | プロジェクト | 取り消す | 4 |
| `POST items/{item}/links/refresh` | アイテム | いまのまま 404(実行者が GitHub。C3a-9 は人の行だけを数える) | — |
| `POST items/{item}/duplicate`、`…/save-as-template` | 複製元・元のアイテム | いまのまま 404(元のログに書かない) | —(save-as-template の tx の中の読みは決定 S8) |
| `POST items/{item}/steps`、`POST items`(親が削除済み) | 親 | いまのまま 404(親のログに書かない。06「親が削除されたとき」で子も削除済みに入る状態を作らない) | — |
| `DELETE items/{item}`、`DELETE projects/{p}` | 削除済みのもの | いまのまま 404 | — |

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| S1 | 取り消す操作は上の表の規則で決める(人の行を、削除済みのもの自身のログに書く操作)。 | C3a-9(同期で変更に数える行)。同期と画面で同じ規則にする |
| S2 | 親(アイテム・プロジェクト)と一緒に削除された(`root: false`)ものへの保存は、取り消さず、いまのまま 404(画面は既存の「見つからない」の扱い)。 | 保存形式「同期の修復」の「`root: false`(親と一緒の削除)は対象にしない」、「テスト」の「親の削除と子の変更が同時なら子は削除済みのまま」、06「親が削除されたとき」 |
| S3 | 取り消しは、削除の op の行があるすべてのログ(削除されたものがアイテムならそのプロジェクトのアイテム、プロジェクトならプロジェクト自身とそのアイテム)に、`change.Undo(log, op)` の行を書く。変更と同じ 1 回の保存に入れ、アクティビティの文は操作のもの(例: 「HH-12 のタイトルを変更」)のまま。 | 08「復元の単位」(一緒に削除された子も戻る)。01「書き込みの単位」(1 操作 1 保存)。06「アクティビティ」の「{ID} の削除を取り消し」は実行者「同期」の行で、この保存には当たらない |
| S4 | 何も変わらない保存(同じタイトル・同じ値・合わせた説明が同じ・外す値がない など、行を書かずに返す分かれ道)は、取り消さず、削除済みなら `port.ErrNotFound`(404)。 | 仕様の文が「変更を保存したため」。行のない保存は保存ではない |
| S5 | 関連の相手が削除済みなら、いまのまま 404。取り消すのは、操作したアイテム(経路の `{item}`)だけ。 | 相手は候補から選んだもので、変更の対象ではない。削除済みを指す関連を作らない |
| S6 | 取り消しの知らせは、応答ヘッダー `Hedgehug-Restored`(表示 ID)で返す。`httpapi.New` が API の Handler を包むミドルウェアで、context に入れ物を置き、2xx の応答のときだけヘッダーを付ける。画面は `fetcher` で読み、登録した 1 つの受け手(`wireBase.ts` でトーストを出す)に渡す。 | 経路ごとに応答の形が違う。DTO に欄を足すと 30 経路と 30 か所の画面を直すことになる |
| S7 | `ReviveItem`・`ReviveProject` は ID を受け取り、tx の中で読み直して決める(同じ保存の中で 2 回呼んでも、2 回目は何も書かない)。 | ステータスの変更は自動化で同じアイテムをもう一度読む(`items/automate.go`)。C3a の `undo` と同じく、保存の中で読み直したログで決める |
| S8 | 調べて見つかった、tx の中で削除済みを確かめていない 4 か所のうち、コメントの編集・削除、作業時間の削除、説明の保存は、この計画の `EditableItem`・`EditableProject` で直る(Task 4)。save-as-template の tx の中の読み(`workspaces/edit.go:35`)は、ユーザーの承認を得たら Task 5 の Step 3 で `port.LiveItem` を足す(承認待ち)。 | 調べた結果。前の読みと保存のあいだに別のタブが削除すると、削除済みのものに行を書いて、取り消さないまま残る |
| S9 | 削除済み(`root: true`)のアイテムでも、祖先のアイテムかプロジェクトが削除済み(別の操作の削除)なら、取り消さず 404。プロジェクトは、ワークスペースが読めることだけを確かめる(ワークスペースの削除はまだない。C3a-10)。 | 08 の決定の表「親が削除済みのときの復元」(アイテムだけ断る)。取り消すと、06「親が削除されたとき」の、削除済みの親の下に生きた子がある状態を作る |
| S10 | 各 usecase の tx は、最初に `EditableItem`(`EditableProject`)で読んだ削除の印を覚え、すぐ `ReviveItem`(`ReviveProject`)を呼んでから、いまの処理(`LiveItem` などの読み直しを含む)を続ける。何も書かずに返す分かれ道は `return port.Unchanged(覚えた印)` にする。印があればエラーなので、保存ごと取り消しの行も残らない。 | tx は自分の書いたものを読む(`port.Tx`)ので、取り消したあとは既存の読み・重なりの確かめ(関連の snapshot など)がそのまま使える。「行を書く直前に呼ぶ」は、分かれ道の多い usecase(関連・移動)で置き場所がぶれる |

## Review Focus

1. **同じ保存で 2 回呼ぶ取り消し。** ステータスの変更が自動化で同じアイテムに書くとき、`restore` が 2 行にならない。Task 3 のテストに入れる。
2. **一緒に削除された子。** Epic を削除してから Epic のタイトルを変えると、Epic と下の Task がどちらも戻る。Task の側を変えたとき(`root: false`)は 404 のまま。Task 1・Task 2 のテストに入れる。
3. **何も変わらない保存。** 削除済みのアイテムに同じタイトルを保存すると、404 で、`restore` の行は書かれない。Task 2 のテストに入れる。
4. **失敗した応答にヘッダーを付けない。** 取り消しの行を書いたあとに断り(422)で保存が取り消されたとき、`Hedgehug-Restored` を付けない。Task 7 のテストに入れる。
5. **出来事の読み続け(`/api/…/events`)。** ミドルウェアで包んでも、SSE の Flush が効く(`Unwrap` を持つ)。Task 7 のテストに入れる。

---

### Task 1: 取り消しの口(`app/port`)

**Files:**
- Modify: `internal/app/port/identity.go`(`ids.go` の中身を移す)
- Delete: `internal/app/port/ids.go`
- Create: `internal/app/port/revive.go`
- Test: `internal/app/port/revive_test.go`(パッケージ `port_test`。偽物の Store は `apptest`)
- Modify: `internal/app/apptest/stores_contract_isolation.go`・`stores_contract.go`(契約の 1 行)

**Interfaces:**
- Produces:
  - `func EditableItem(r Reader, id item.ID) (item.Item, error)` — 削除されていないか、`Deleted.Root` が true のアイテムを返す。ない・`root: false`・祖先のアイテム(`Place.Parent` をたどる)かプロジェクトが削除済みなら `ErrNotFound`(決定 S9)。
  - `func EditableProject(r Reader, id project.ID) (project.Project, error)` — 削除されていないか、`Deleted.Root` が true のプロジェクト。ない・`root: false` なら `ErrNotFound`。
  - `func ReviveItem(ctx context.Context, tx Tx, id item.ID) error` — tx の中で読み直し、削除済み(`root: true`)なら、そのプロジェクトのアイテムすべて(`tx.Items().List(プロジェクト)`)のうち、ログに削除の op の行があるものへ `change.Undo(log, op)` を書き、表示 ID を context の入れ物に積む。削除されていなければ何もしない。
  - `func ReviveProject(ctx context.Context, tx Tx, id project.ID) error` — プロジェクト自身とそのアイテムに同じことをする。
  - `func Unchanged(d *change.Deletion) error` — `d` が nil なら nil、そうでなければ `ErrNotFound`(決定 S4 の分かれ道で使う)。
  - `func WithRevived(ctx context.Context) context.Context` と `func Revived(ctx context.Context) []string` — 入れ物を置く・積んだ表示 ID を積んだ順に返す(同じ ID は 1 つ。入れ物がなければ積まずに捨てる)。

- [ ] **Step 1: 失敗するテストを書く**

`revive_test.go` に、`apptest` の偽物の Store で次を書く(どれも、削除は `items.Service.Delete`・`projects.Service.Delete` ではなく、`tx.WriteItems().Append(id, change.NewDeleteEntity(true/false))` を 1 回の Write で書いて作る)。

- `TestEditableItemReturnsRootDeleted`: Epic(root: true)と下の Task(root: false)を同じ op で削除。`EditableItem(r, epic)` は `Deleted != nil` のアイテムを返す。`EditableItem(r, task)` は `errors.Is(err, port.ErrNotFound)`。
- `TestReviveItemRestoresTheWholeOp`: 上の状態で、`ctx := port.WithRevived(context.Background())` で Write し、`ReviveItem(ctx, tx, epic)`。Write のあと、Epic と Task がどちらも `Deleted == nil`。`port.Revived(ctx)` が `[]string{"HH-1"}`(Epic の表示 ID)。
- `TestReviveItemTwiceWritesOnce`: 同じ Write の中で `ReviveItem` を 2 回。Epic のログの `restore` の行は 1 つ。
- `TestReviveItemOnLiveDoesNothing`: 削除されていないアイテムで呼ぶと、行も `Revived` も増えない。
- `TestReviveProjectRestoresItems`: プロジェクト(root: true)と下のアイテム(root: false)を同じ op で削除。`ReviveProject` のあと両方が戻り、`Revived` が `[]string{"HH-P1"}`。
- `TestUnchanged`: `Unchanged(nil) == nil`、`errors.Is(Unchanged(&change.Deletion{}), port.ErrNotFound)`。
- `TestEditableItemUnderParentDeletedByAnotherOp`: Task を root: true で削除(op1)、そのあと親の Epic を別の Write で root: true で削除(op2。Task には行を書かない)。`EditableItem(r, task)` は `port.ErrNotFound`。
- `TestEditableItemInProjectDeletedByAnotherOp`: Task を root: true で削除(op1)、そのあとプロジェクトを別の Write で root: true で削除(op2)。`EditableItem(r, task)` は `port.ErrNotFound`。

本物の gitstore でも、利用者の操作の `restore` の行が読めることを、Store の契約に足す(`internal/app/apptest/stores_contract_isolation.go` に書き足し、`stores_contract.go` の表に 1 行足す。apptest にファイルは足さない):
- `contractUserRestoreSeenInTx`: アイテムを削除したあと、実行者 `change.User("taro")` の 1 回の Write の中で `ReviveItem` を 2 回呼ぶ。2 回目の前に `tx.Items().Get` の `Deleted == nil`、Write のあとログの `restore` の行は 1 つ。

Run(Step 2・4 に加える): `go test ./internal/app/apptest/... ./internal/infra/gitstore/...`

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/port/...`
Expected: FAIL(`undefined: port.EditableItem` など)

- [ ] **Step 3: `ids.go` を `identity.go` へ移し、`revive.go` を書く**

入れ物は context の値(`map` と順の slice を持つ構造体のポインタ)。ログへの書き込みは、`change.Ref` の種類で `tx.WriteItems()`・`tx.WriteProjects()` を選ぶ(`repairing/snapshot.go` の `appendTo` と同じ分け方)。ログに op の行があるかは `slices.ContainsFunc(log, func(r change.Record) bool { return r.Op == op })`。表示 ID の接頭辞は `tx.Workspaces().Get(ワークスペース).Prefix`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/port/... ./internal/archtest/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/port internal/app/apptest
git commit -m "feat: 削除済みのものへの変更で削除を取り消す口を足す"
```

### Task 2: タイトル・種類・並べ替え・移動

**Files:**
- Modify: `internal/app/items/service.go`(`withID` の隣に `editableID` を足す)、`rename.go`、`changekind.go`、`reorder.go`、`move.go`
- Test: `internal/app/items/deleted_test.go`(書き足す。600 行を超えるなら `restructure_helpers_test.go` の隣に `revive_test.go` を作る。テストのファイルは数の上限に入らない)

**Interfaces:**
- Consumes: Task 1 の `EditableItem`・`ReviveItem`・`Unchanged`・`WithRevived`・`Revived`
- Produces: `func (s Service) editableID(ctx context.Context, store port.StoreID, id item.ID) (item.Item, string, error)` — `withID` と同じで、読みが `port.EditableItem`。`withID` は複製・Step の追加が使い続ける。

- [ ] **Step 1: 失敗するテストを書く**

どれも、アイテムを `root: true` で削除してから(別のタブの削除に当たる)、`port.WithRevived` の context で操作を呼ぶ。

- `TestRenameDeletedItemRevives`: `Rename` が成功し、アイテムが `Deleted == nil`、タイトルが変わり、`Revived(ctx) == []string{"HH-1"}`。アクティビティ(コミットの Summary)は「HH-1 のタイトルを変更」の 1 件。
- `TestRenameDeletedItemSameTitleIsNotFound`: 同じタイトルで `Rename` は `port.ErrNotFound`、`restore` の行がない。
- `TestRenameItemDeletedWithParentIsNotFound`: Epic の削除で一緒に削除された Task(root: false)の `Rename` は `port.ErrNotFound`。
- `TestRenameDeletedEpicRevivesChildren`: Epic(root: true)と Task(root: false)を同じ op で削除し、Epic を `Rename`。Task も戻る。
- `TestChangeKindDeletedItemRevives`、`TestReorderDeletedItemRevives`、`TestMoveDeletedItemRevives`: それぞれ成功して `Deleted == nil`。
- `TestMoveToDeletedParentIsNotFound`: 移動先の親が削除済みなら `port.ErrNotFound`(移すアイテムは削除済みのまま変わらない)。
- `TestRenameItemUnderParentDeletedLaterIsNotFound`: Task を削除(root: true)したあと、別の保存で親の Epic を削除。Task の `Rename` は `port.ErrNotFound`、Task のログに `restore` がない(決定 S9)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run 'Deleted|Revive'`
Expected: FAIL(`ErrNotFound`)

- [ ] **Step 3: 読みを替え、tx の最初で取り消す(決定 S10)**

- `rename.go`・`changekind.go`・`reorder.go`: 前の読みを `editableID` にする。tx の最初で `it, err := port.EditableItem(tx, id)`・`port.ReviveItem(ctx, tx, id)` を呼び、そのあとの処理(`LiveItem` の読み直しを含む)は変えない。行を書かずに返す分かれ道(`rename.go:34`・`reorder.go:52`)は `return port.Unchanged(it.Deleted)`。
- `move.go`: 前の読み(`:67`)を `EditableItem` に。tx の最初(`:50` の前)で同じく `EditableItem`・`ReviveItem`。移動先の読み(`:97`・`:113`・`:126`)は `LiveItem`・`LiveProject` のまま(削除済みなら 404 で、保存ごと取り消しも残らない)。Summary が空で返す分かれ道は、前の段(`:41`)では削除済みなら `port.ErrNotFound`、tx の中(`:55`)では `port.Unchanged(it.Deleted)`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/items/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/items
git commit -m "feat: 別の画面で削除されたアイテムのタイトル・種類・並び・場所を保存したら削除を取り消す"
```

### Task 3: プロパティ・値・Step のチェック・自動化の手動の適用

**Files:**
- Modify: `internal/app/items/view.go`(`editContext` の読みを `EditableItem`)、`setproperty.go`、`togglevalue.go`、`automate.go`
- Modify: `internal/app/automations/apply.go`
- Test: `internal/app/items/deleted_test.go`(または Task 2 の `revive_test.go`)、`internal/app/automations/` の既存のテストのファイル

**Interfaces:**
- Consumes: Task 1 の口

- [ ] **Step 1: 失敗するテストを書く**

- `TestSetPropertyDeletedItemRevives`: 削除済み(root: true)のアイテムの優先度を変えると成功し、`Deleted == nil`、`Revived == []string{"HH-1"}`。
- `TestSetStatusDeletedItemWithAutomationRevivesOnce`: 「完了で親を完了にする」自動化のあるワークスペースで、削除済みの Task のステータスを完了にする。Task のログの `restore` は 1 行。親(削除されていない)は自動化で完了になる。
- `TestToggleValueDeletedItemRevives`(関連システムに値を足す)、`TestCheckStepDeletedRevives`。
- `TestSetPropertyDeletedItemSameValueIsNotFound`: 同じ値なら `port.ErrNotFound`、`restore` なし。
- `TestApplyAutomationDeletedItemRevives`(`automations` のテスト): 成功して `Deleted == nil`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/items/ ./internal/app/automations/ -run 'Deleted|Revive'`
Expected: FAIL

- [ ] **Step 3: 読みを替え、tx の最初で取り消す(決定 S10)**

`editContext`(`view.go:27`。呼ぶのは書き込みの `setproperty.go`・`togglevalue.go`・`checkstep.go` だけ)を `EditableItem` に。`writeValue`(`setproperty.go:114`)の tx の最初で `EditableItem`・`ReviveItem` を呼び、`:114` の `LiveItem` はそのあとに残す。行が 0 のとき(`setproperty.go:39-40`・`:122`、`togglevalue.go:47`)は `port.Unchanged(覚えた印)`。`automate.go` は変えない(取り消したあとに呼ばれるので `LiveItem` で読める。`:39-45` の削除済みの親は動かさない)。`automations/apply.go` は `:32` を `EditableItem`、`applyStatus` の最初で `EditableItem`・`ReviveItem`、`:69-70` を `Unchanged`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/items/... ./internal/app/automations/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/items internal/app/automations
git commit -m "feat: 別の画面で削除されたアイテムのプロパティと値を保存したら削除を取り消す"
```

### Task 4: 説明・コメント・作業時間・ウォッチ

**Files:**
- Modify: `internal/app/descriptions/saveitem.go`・`saveproject.go`・`save.go`
- Modify: `internal/app/comments/service.go`・`post.go`・`edit.go`・`delete.go`
- Modify: `internal/app/worklogs/service.go`・`add.go`・`delete.go`
- Modify: `internal/app/watching/service.go`・`set.go`
- Test: 各パッケージの既存のテストのファイル

**Interfaces:**
- Consumes: Task 1 の口(プロジェクトの説明・コメントは `EditableProject`・`ReviveProject`)

- [ ] **Step 1: 失敗するテストを書く**

それぞれ、アイテム(コメント・説明はプロジェクトも)を `root: true` で削除してから操作し、成功・`Deleted == nil`・`Revived` が表示 ID 1 つ、を確かめる。

- `TestSaveItemDescriptionDeletedRevives`・`TestSaveProjectDescriptionDeletedRevives`
- `TestSaveDescriptionDeletedUnchangedIsNotFound`: 合わせた結果が保存済みと同じなら `port.ErrNotFound`。
- `TestPostCommentDeletedRevives`・`TestEditCommentDeletedRevives`・`TestDeleteCommentDeletedRevives`(アイテムとプロジェクトの両方。表で回す)
- `TestEditCommentOnItemDeletedWithParentIsNotFound`: `root: false` のアイテムのコメントの編集は `port.ErrNotFound`(いまは確かめずに書いている。決定 S8)。
- `TestAddWorklogDeletedRevives`・`TestDeleteWorklogDeletedRevives`
- `TestWatchDeletedRevives`、`TestWatchDeletedUnchangedIsNotFound`(すでにウォッチ中で入れる)

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/descriptions/ ./internal/app/comments/ ./internal/app/worklogs/ ./internal/app/watching/ -run 'Deleted|Revive'`
Expected: FAIL

- [ ] **Step 3: 読みを替え、tx の最初で取り消す(決定 S10)**

- 前の読み(保存の前の `Stores.Read`):
  - `descriptions/saveitem.go:31`・`saveproject.go:18` を `EditableItem`・`EditableProject` に(書き込みだけが使う)。`descriptions/get.go` は変えない。
  - `comments/service.go` の `display(r, t)` は一覧(`list.go`)と投稿の tx(`post.go:31`)が使う。`display` は `LiveItem`・`LiveProject` のまま残し、隣に `editableDisplay(r Reader, t Target) (string, error)`(読みが `EditableItem`・`EditableProject`)を足す。`displayOf`(`:78`、書き込みだけが使う)は `editableDisplay` を呼ぶ。編集・削除(`edit.go`・`delete.go`)は保存の前に `displayOf` を呼んで対象を確かめる。
  - `worklogs/service.go:32` の `display` は一覧と共用なので残し、`displayOf`(`:44`、追加と削除だけが使う)の中の読みを `EditableItem` にする。
  - `watching/service.go:21` の `displayOf`(`set.go` だけが使う)を `EditableItem` に。
- tx の中: どれも最初で `EditableItem`・`EditableProject` と `ReviveItem`・`ReviveProject` を呼ぶ。説明は `save.go:29` の前、コメントは投稿(`post.go:31`)・編集と削除(`service.go:89-99` の `currentOf` の前)、作業時間は追加(`add.go:34`)・削除(`delete.go:25` の前)、ウォッチは `set.go:29`。何も書かずに返す分かれ道(説明 `save.go:35-36`、コメントの編集 `edit.go:34-35`、ウォッチの行が 0)は `port.Unchanged(覚えた印)`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/descriptions/... ./internal/app/comments/... ./internal/app/worklogs/... ./internal/app/watching/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/descriptions internal/app/comments internal/app/worklogs internal/app/watching
git commit -m "feat: 別の画面で削除されたものの説明・コメント・作業時間・ウォッチを保存したら削除を取り消す"
```

### Task 5: 関連・PR と Issue のリンク・テンプレートとして保存の読み

**Files:**
- Modify: `internal/app/relations/service.go`・`snapshot.go`・`add.go`・`remove.go`
- Modify: `internal/app/githublink/links.go`・`link_add.go`・`link_remove.go`
- Modify: `internal/app/workspaces/edit.go`(または `template_item.go`。tx の中の読み)
- Test: 各パッケージの既存のテストのファイル

**Interfaces:**
- Consumes: Task 1 の口

- [ ] **Step 1: 失敗するテストを書く**

- `TestAddRelationFromDeletedRevives`: 削除済み(root: true)のアイテムから、削除されていない相手へ「ブロックしている」を足すと成功し、自分だけ戻る。`reverse` で相手のログに書く向き(「ブロックされている」)も同じ。
- `TestAddRelationToDeletedIsNotFound`: 相手が削除済みなら `port.ErrNotFound`、自分は変わらない(決定 S5)。
- `TestRemoveRelationFromDeletedRevives`。
- `TestAddLinkDeletedItemRevives`・`TestRemoveLinkDeletedItemRevives`(`githublink`。既存の GitHub の偽物を使う)。
- `TestAddLinkItemInDeletedProjectIsNotFound`: プロジェクトの削除で一緒に削除されたアイテムは `port.ErrNotFound`。
- `TestRefreshLinksDeletedItemIsNotFound`: 取り直し(実行者 GitHub)は、いまのまま `port.ErrNotFound`。
- `TestSaveAsTemplateDeletedInTxIsNotFound`(`workspaces`): 前の読みのあと、Write の前にアイテムを削除する(`apptest` の Store の書き込みのフックか、2 回目の読みで削除する偽物)と `port.ErrNotFound`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/relations/ ./internal/app/githublink/ ./internal/app/workspaces/ -run 'Deleted|Revive'`
Expected: FAIL

- [ ] **Step 3: 読みを替え、tx の最初で取り消す(決定 S10)**

- 関連: snapshot は生きているものだけを持つ(`snapshot.go:42,47`)。前の読み(`service.go:26-29`)では、自分(経路の `{item}`)を `EditableItem` で確かめ、相手は snapshot のまま確かめる。tx の中は、最初に自分を `EditableItem`・`ReviveItem` してから snapshot を作る(取り消した自分が snapshot に入り、重なりの確かめはいまのまま働く)。外すとき(`remove.go:36-37` の分かれ道)は `port.Unchanged(覚えた印)`。
- リンク: `readItem`(`links.go:54`)に、自分を `EditableItem` で読む版 `readEditableItem` を足し、追加(`link_add.go:79`)と外す(`link_remove.go:29`)の tx の最初で `ReviveItem` を呼んでから使う。プロジェクト(`links.go:66`)は `LiveProject` のまま。祖先やプロジェクトが削除済みのアイテムは `EditableItem` が断る(決定 S9)。取り直し(`write.go:71`)は `readItem` のまま。`link_remove.go:34-35` の分かれ道は `Unchanged`。
- テンプレートとして保存(ユーザーの承認を得た場合だけ。決定 S8): tx の中(`workspaces/edit.go:35` の前)で、元のアイテムを `port.LiveItem` で読み直す。承認がなければ、Step 1 の `TestSaveAsTemplateDeletedInTxIsNotFound` も書かない。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/relations/... ./internal/app/githublink/... ./internal/app/workspaces/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/relations internal/app/githublink internal/app/workspaces
git commit -m "feat: 別の画面で削除されたアイテムの関連とリンクを保存したら削除を取り消す"
```

### Task 6: プロジェクトのタイトル・ステータス・オーナー・日付・リポジトリ

**Files:**
- Modify: `internal/app/projects/get.go`(`Get` の隣に `editable` を足す)、`edit.go`、`rename.go`・`setstatus.go`・`setowner.go`・`setdate.go`・`addrepo.go`・`removerepo.go`
- Test: `internal/app/projects/deleted_test.go`

**Interfaces:**
- Consumes: Task 1 の口
- Produces: `func (s Service) editable(ctx context.Context, store port.StoreID, id project.ID) (project.Project, error)` — `Get` と同じで、読みが `port.EditableProject`。`Get`(GET の経路)は変えない。

- [ ] **Step 1: 失敗するテストを書く**

表で回す `TestEditDeletedProjectRevives`: タイトル・ステータス・オーナー・開始日・リポジトリの追加・リポジトリの解除のそれぞれで、プロジェクト(root: true)と下のアイテム(root: false)を同じ op で削除してから操作すると、成功し、プロジェクトと下のアイテムが戻り、`Revived == []string{"HH-P1"}`。`TestEditDeletedProjectSameValueIsNotFound`: 同じタイトルなら `port.ErrNotFound`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/projects/ -run 'Deleted|Revive'`
Expected: FAIL

- [ ] **Step 3: 読みを替え、tx の最初で取り消す(決定 S10)**

前の読み(各ファイルの `s.Get`)を `s.editable` に。何も変わらない分かれ道(`setdate.go:37-38`・`setowner.go:32-33`・`setstatus.go:23-24`・`rename.go:30-31`・`removerepo.go:24-25`)は前の段の値で決まるので `port.Unchanged(p.Deleted)`。`edit.go` の `save` は tx の最初(`:21`)で `EditableProject`・`ReviveProject` を呼び、行を作る関数が 0 行を返したとき(`:29` の前)は `port.Unchanged(覚えた印)`。`rename.go:23` の兄弟の読みは `LiveProjects` のまま。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/projects/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/projects
git commit -m "feat: 別の画面で削除されたプロジェクトを保存したら削除を取り消す"
```

### Task 7: 応答ヘッダー `Hedgehug-Restored`

**Files:**
- Modify: `internal/adapter/httpapi/server.go`(`New` で `api` を包む)
- Test: `internal/adapter/httpapi/server_test.go`、`internal/adapter/httpapi/items/edit_test.go`(1 経路の通し)

**Interfaces:**
- Consumes: Task 1 の `WithRevived`・`Revived`
- Produces: `func restoredHeader(next http.Handler) http.Handler`(`server.go`)。`New` は `requireToken(token, restoredHeader(api))`。

- [ ] **Step 1: 失敗するテストを書く**

`server_test.go` に、`restoredHeader` で包んだ偽の Handler で:
- `TestRestoredHeaderOnSuccess`: Handler が context に `"HH-1"` を積んで 200 を返すと、`res.Header.Values("Hedgehug-Restored") == []string{"HH-1"}`。本文を書かずに `WriteHeader(204)` だけでも付く。
- `TestRestoredHeaderNotOnFailure`: 積んでから 422 を返すと、ヘッダーがない。
- `TestRestoredHeaderKeepsFlusher`: 包んだ `ResponseWriter` で `http.NewResponseController(w).Flush()` がエラーにならない。

`items/edit_test.go` に `TestRenameDeletedItemSetsRestoredHeader`: 削除済みのアイテムへ `PUT …/title` が 200 で、`Hedgehug-Restored: HH-1`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: FAIL

- [ ] **Step 3: `restoredHeader` を書く**

`r.WithContext(port.WithRevived(r.Context()))` で次へ渡す。`ResponseWriter` を包み、最初の `WriteHeader`(`Write` が先なら 200 として)の時点で、状態が 2xx なら `port.Revived(ctx)` の表示 ID を 1 つずつ `Header().Add`。包んだものに `Unwrap() http.ResponseWriter` を持たせる。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/adapter/httpapi/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/httpapi
git commit -m "feat: 削除を取り消した保存の応答に Hedgehug-Restored を付ける"
```

### Task 8: 画面のトースト

**Files:**
- Modify: `web/packages/api/src/fetcher.ts`(ヘッダーを読む)、`web/packages/api/src/index.ts`(公開)
- Create: `web/packages/core/src/features/sync/restored.ts`(文)、`web/packages/core/src/index.ts`(公開)
- Modify: `web/app/src/wireBase.ts`(受け手の登録)
- Test: `web/packages/api/src/fetcher.test.ts`(新規)、`web/packages/core/src/features/sync/restored.test.ts`

**Interfaces:**
- Produces:
  - api: `export function onRestored(listener: (ids: string[]) => void): void` — `fetcher` が、成功した応答の `Hedgehug-Restored` を `", "` で分けた ID の並び(空でなければ)で呼ぶ受け手を 1 つ置く。
  - core: `export function restoredToast(id: string): string` — `${id} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`。

- [ ] **Step 1: 失敗するテストを書く**

- `fetcher.test.ts`: `fetch` を差し替え、ヘッダー `Hedgehug-Restored: HH-1` の 200 で、受け手が `["HH-1"]` で 1 回呼ばれる。ヘッダーがない応答・422 の応答では呼ばれない。
- `restored.test.ts`: `restoredToast("HH-12") === "HH-12 は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました"`。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web test`
Expected: FAIL

- [ ] **Step 3: 書く**

`fetcher` は `res.ok` のときだけ読む。`wireBase.ts` で `onRestored((ids) => { for (const id of ids) toasts.show(restoredToast(id)); })`(通常のトースト。エラー・警告の印は付けない)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web deps && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/api web/packages/core web/app/src/wireBase.ts
git commit -m "feat: 別の画面で削除されていたものへの保存でトーストを出す"
```

### Task 9: e2e(2 つのタブ)

**Files:**
- Create: `web/e2e/deleted-elsewhere.spec.ts`

**Interfaces:**
- Consumes: `web/e2e/helpers.ts` の `createWorkspace`・`newTaskUnder`・`newProject`・`prefixOf`・`tokenUrl`・`unique`、Task 8 のトースト

このタブは、説明を編集しているあいだ(06「反映を待つとき」)なので、別のタブの削除が反映されず、詳細パネルが開いたまま保存できる。

- [ ] **Step 1: テストを書く**

- `"saving a description of an item deleted in another tab restores it"`: このタブでアイテムを作り詳細パネルの「説明」の「説明を追加」を押して入力する。別のタブ(`page.context().newPage()`)で同じアイテムを開き、操作メニューの「削除」→ 確認ダイアログの「削除」で削除する。このタブで Ctrl+Enter で保存すると、トースト「{prefix}-2 は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました」が出て、説明が表示される。別のタブでアイテムのアドレスを開き直すと、詳細パネルが開く(削除済みでない)。
- `"saving a description of a project deleted in another tab restores it"`: このタブでプロジェクトの概要の説明を編集中に、別のタブでそのプロジェクトを削除し、このタブで保存すると、トースト「{prefix}-P1 は別の画面で…」が出る。

(削除の操作の名前・ダイアログのボタンは `web/e2e/project-delete.spec.ts` と `item-actions.spec.ts` のものを使う。)

- [ ] **Step 2: 流して通ることを確かめる**

Run: `make e2e-bin && pnpm -C web e2e e2e/deleted-elsewhere.spec.ts`
Expected: 2 passed

- [ ] **Step 3: コミット**

```bash
git add web/e2e/deleted-elsewhere.spec.ts
git commit -m "test: 別のタブで削除したものへの保存を e2e で確かめる"
```

### Task 10: 全体の検証

- [ ] **Step 1:** `make lint && make test && golangci-lint run --build-tags e2e ./...` がすべて通る。
- [ ] **Step 2:** 上の「取り消す操作と取り消さない操作」の表の各行に、Task 2〜7 のテストが 1 つ以上あることを確かめる(足りなければ、その行の Task のテストのファイルに足してコミット)。
