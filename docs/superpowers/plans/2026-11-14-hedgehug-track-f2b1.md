# hedgehug トラック F2b1(行の選択・選択バー・一括削除)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 04「一括選択と一括変更・一括削除」のうち、選択(行のチェック・選択バー・選択から外す規則)と一括削除を、検索のテーブル(トラック E)でも使える共通の部品として作り、バックログのタブの行に付ける。コマンドパレットの「選んでいるアイテム」に、バックログでちょうど 1 件選んだアイテムを登録する。

**Architecture:** サーバーは `app/items` の `delete.go` に、複数の Store にまたがる `BulkDeletePreview`・`BulkDelete` を足す(Store ごとに 1 回の保存、閲覧のみの Store は飛ばして数える)。API は `adapter/httpapi/items/bulk.go`。画面は core の新しい `features/selection/`(`rowSelection.ts` = 選択の状態、`bulkDelete.ts` = 確認ダイアログと実行)を、バックログ(`features/planning/backlog.ts`)がつなぐ。バーは ui の `SelectionBar.svelte`。E はテーブルに同じ部品を付けるだけにする。

**Tech Stack:** Go 1.27.3、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`「一括選択と一括変更・一括削除」のうち冒頭・「選択」・「一括削除」(816〜825・897・899〜911 行)、「閲覧のみの Store のアイテム」(478〜488 行。選択は閲覧のみでも使える)、アクティビティの表の「一括削除」(971 行)と「保存にもアクティビティにも残らない」(973 行。選択)、「モックからの変更点」の 1034・1038・1039・1048・1054 行
- 05 の 35 行(行の先頭の選択のチェック。選択と一括変更は 04 に従う)
- 02 の 632 行(パレットの「選んでいるアイテム」: テーブル・バックログで選択がちょうど 1 件のアイテム)、02「削除」(1 件の削除と同じ root の付け方・保持日数)
- 01 の 106・108 行(複数の Store は Store ごとに 1 回ずつ保存。途中の失敗)、124 行(閲覧のみの Store)
- 08「復元の単位」(328〜336 行)
- `docs/superpowers/plans/2026-11-14-hedgehug-track-f2a.md` の末尾「F2b に渡すもの」、`docs/superpowers/plans/2026-11-14-hedgehug-track-a5c.md` の A5c-9(`selection.provide`)

## 始める前に

- F2 は 2 本に分けた(ユーザーの決定、2026-11-14): F2a バックログのタブ(develop に入った、PR #216)、F2b 選択バー・一括変更・一括削除。
- F2b はさらに 2 本に分ける(ユーザーの決定、2026-11-14): **F2b1 = この計画**(行の選択のチェック・選択バー・選択から外す規則・一括削除・パレットの「選んでいるアイテム」)、**F2b2** = 一括変更(ダイアログ・確認・結果のトースト・自動化と通知・バックログのトーストの末尾「。{K} 件は完了のため、バックログには表示されません」)。それぞれ 1 計画・1 PR。F2b1 の時点のバーは「{N} 件を選択中」「一括削除…」「選択を解除」で、F2b2 が「一括変更…」を先頭に足す。
- E は、F2b1・F2b2 の部品を検索のテーブルに付ける(「すべて選択」・パレットの「{N} 件をまとめて変更…」・テーブルの `selection.provide(2, …)` を含む)。計画の末尾「E に求めること」。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/items` は 20 個で満杯なので、ファイルを足さず `delete.go`(121 行)に書く。`web/app/src/screens/planning/BacklogRow.svelte` は 200 行、`web/packages/core/src/features/planning/backlog.test.ts` は 298 行、`web/app/src/wire.ts` は 298 行で、どれも足せない(ファイルの地図のとおり切り出す・別のファイルに書く)。
- app の機能どうしは import しない(設計書 2 節)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書き、言い換えない。閲覧のみの添え書きは core の `readOnlyNote`(「閲覧のみの Store です」)、削除の失敗は `itemDeleteFailedMessage`(「削除できませんでした」)。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする(CI が差分を見る)。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| F2b1-1 | 選択の 1 件は `{ storeId, itemId }` で見分ける。内部 ID は Store の中でだけ一意なので、`itemId` だけでは比べない | E のテーブルは複数の Store のアイテムを並べる(04)。`features/items/delete.ts` の祖先の比べ方と同じ |
| F2b1-2 | 選択の状態は core の `createRowSelection()`。行は `SelectedRow { storeId, itemId, display, readOnly }`。並びは画面の並び(`prune` に渡した行の順)で、ダイアログの ID の並びに使う。名前はパレットの `Selection`・`createSelection` と分ける | 04 の 822〜825 行。index がパレットの `Selection` をすでに公開している |
| F2b1-3 | 選択から外すのは、画面が読み直すたび(`prune(見えている行)`)。見えている行にないものを外す。バックログでは、すべての枠(スプリントの枠とバックログの枠)の行を上から順に渡す。0 件になればバーを閉じる。タブを離れる(`backlog.close()`)・別のワークスペースを開くと、選択を解除する。同期・別のタブの保存は、Store の出来事の読み直し(F2a-16)で同じく外れる | 04 の 822・823 行 |
| F2b1-4 | 行の先頭のチェックは、閲覧のみの Store でも押せる(無効にしない)。押せないのは選択バーの「一括削除…」(F2b2 では「一括変更…」も)で、選んだ行がすべて閲覧のみのとき無効にして `readOnlyNote` を添える。チェックは見える文字を持たない `<input type="checkbox">` で、`aria-label` は「{ID} を選択」 | 04 の 484 行(選択は閲覧のみでも使える)・825 行。05 の 35 行の行の並びに見出しの文字はない。ui の `Checkbox` は見える `label` が必須なので使わない |
| F2b1-5 | バーは一覧の上に 1 本(バックログでは、スプリントの枠より上)。中身は「{N} 件を選択中」・「一括削除…」・「選択を解除」。保存にもアクティビティにも残さない | 04 の 824・973 行 |
| F2b1-6 | サーバーの一括削除は `items.Service.BulkDelete(ctx, []BulkTarget)`。Store ごとに、最初に出てきた順で 1 回ずつ保存する。閲覧のみの Store のアイテムは書かずに `ReadOnly` に数える(選んだ件数。下にあるものは数えない)。実行のときに削除済み・存在しないアイテムは飛ばす(エラーにしない)。表示中でない Store を含むときは、何も書かずに `port.ErrStoreNotFound`(先に全部の Store を確かめる)。`items.Service` に `StoreList port.StoreList` を足す(1 件の `Delete` は閲覧のみを画面だけで止めているが、一括では閲覧のみのものを数えて返すため、サーバーが判定する) | 04 の 909 行、01 の 106 行 |
| F2b1-7 | 同じ Store で、選んだアイテムの祖先も選んでいるときは、そのアイテムは祖先の下のものとして `root: false` で 1 回だけ書く(1 件の削除で親と一緒に消えたのと同じ)。祖先を選んでいないアイテムは `root: true`。削除済みの一覧には `root: true` のものが出て、その「復元」で下のものと一緒に戻る | 08 の 330・334 行(同じ削除操作で消えた子孫は一緒に戻る)、保存形式の `root`(148 行)。両方を `root: true` にすると、同じ操作の行が削除済みの一覧に重なって出る |
| F2b1-8 | Store ごとの件数 N は、その保存で書いた削除の行の数(下にあるものを含み、重ねて数えない)。アクティビティは「{N} 件を一括削除」。N が 0 の Store(全部がすでに削除済み)は保存しない。N は保存の前に読み取りで数え、保存の中でもう一度同じ計算をして書く(1 件の `Delete` の `deleteSummary` と同じ。数えてから保存までに同期で増減したときは合わないことがある) | 04 の 909・971 行、`delete.go` の `Delete` のコメント |
| F2b1-9 | Store の保存に失敗したら、そこでやめてエラーを返す(先に保存した Store の削除は残る)。画面は「削除できませんでした」をエラーのトーストに出し、ダイアログを閉じて読み直す(選択は解除しない。消えたものは読み直しで外れる) | 01 の 108 行(途中の失敗)。Store をまたいで元に戻す手段がないので、1 件の削除の失敗の表示に合わせる |
| F2b1-10 | 確認ダイアログの中身は、`POST /api/bulk-delete/preview` の応答(`stores` = 削除するアイテムのある Store の数、`retentionDays` = `stores` が 1 のときのその Store の保持日数、ほかは 0、`readOnly` = 閲覧のみの Store の選んだ件数)と、開いた時点の選択の行で作る。数えるのは閲覧のみでない、実行のときに削除されていない選んだアイテム。ダイアログを開いているあいだ、対象は開いた時点の選択に固定する | 04 の 905〜907 行。E のテーブルには計画の概要がないので、保持日数は Store ごとにサーバーから読む |
| F2b1-11 | 確認ダイアログ: 見出し「一括削除」。本文の 1 段落目は「{ID}、{ID} と、その下にあるものを削除します。設定の「削除済み」から戻せます。{N} 日を過ぎると完全に削除されます。」(ID は閲覧のみでない選んだ行を画面の並びで先頭の 10 件まで「、」でつなぎ、11 件目からは「{ID}、…、{ID} ほか {K} 件 と、…」。`stores` が 2 以上なら最後の文を「保持日数を過ぎると完全に削除されます。」)。閲覧のみの行があれば 2 段落目に「閲覧のみの Store の {K} 件は削除しません。」。ボタンは「{N} 件を削除」(N は閲覧のみでない選んだ行の数、危険な操作の見た目) | 04 の 905〜907 行 |
| F2b1-12 | 実行のあと: 選択を解除し、ダイアログを閉じ、開いている詳細パネルが選んだアイテムなら閉じ(下にあるものを開いていれば、詳細パネルの読み直しの 404 で閉じる)、画面を読み直す。トーストは「{M} 件を削除しました」、閲覧のみを残したら「{M} 件を削除しました。{K} 件は閲覧のみの Store のためそのままです」(M・K はサーバーの応答)。M が 0(全部がすでに消えていた)ならトーストを出さずに読み直す | 04 の 911 行。M が 0 のときは、1 件の削除の「すでに消えているのは、消した結果と同じなので、トーストなしで読み直す」に合わせる |
| F2b1-13 | ダイアログを開く前に、開いている詳細パネルが選んだアイテムか、その下のもの(パネルの `ancestors` に選んだアイテムがある)なら、書きかけの確認(`drafts.confirmLeave(isItemDraft)`)を先に聞く。断ったら開かない | 1 件の削除(`features/items/delete.ts` の `open`)と同じ |
| F2b1-14 | パレットの「選んでいるアイテム」: バックログのタブが出ているあいだ `selection.provide(2, …)` を登録し、選択がちょうど 1 件のときだけそのアイテムを返す(`source: "backlog"`)。「ステータスを変更…」は `backlog.setStatus(row, …)` を通す(行のステータスのメニューと同じ経路。消えるときのトーストも同じ) | 02 の 632 行、A5c-9・A5c-10 |
| F2b1-15 | API: `POST /api/bulk-delete/preview`(本文 `{ items: [{ store, item }] }`、応答 `{ stores, retentionDays, readOnly }`)と `POST /api/bulk-delete`(同じ本文、応答 `{ deleted, readOnly }`)。`items` が空なら 422。表示中でない Store は 404 | Store をまたぐ操作なので、`/api/stores/{store}` の下に置かない(`/api/locate` などと同じ) |

## Review Focus

1. **Epic とその下の Story を両方選んで一括削除すると、件数が二重になる・削除済みの一覧に 2 行出る。** Epic HH-1(下に Task HH-2・Story HH-3・Step 2 件)と、その Story HH-3 を選んで削除すると、行は 5 つ(Epic だけ `root: true`、HH-3 の行は 1 つで `root: false`)、アクティビティは「5 件を一括削除」、応答の `deleted` は 5(F2b1-7・F2b1-8)。Task 1 の `TestBulkDeleteOverlap` に入れる。
2. **選んだあとに別のタブや同期でアイテムが削除されて、実行が全部失敗する。** 2 件を選び、1 件を先に 1 件の `Delete` で消してから `BulkDelete` すると、残りの 1 件だけ消えて `deleted` は 1(F2b1-6)。Task 1 の `TestBulkDeleteSkipsDeleted` に入れる。
3. **2 つの Store で同じ内部 ID のアイテムを選ぶと、片方しか選べない・外れる。** `toggle({storeId:"S1", itemId:"X"})` と `toggle({storeId:"S2", itemId:"X"})` で 2 件、`prune` に S2 の X だけを渡すと S2 の X だけが残る(F2b1-1)。Task 3 の `selection keeps rows of different stores apart` に入れる。
4. **閲覧のみの Store のバックログで、行のチェックまで無効になる。** 閲覧のみの Store でもチェックは押せ、バーは出て、「一括削除…」だけが無効で「閲覧のみの Store です」が添えられる(F2b1-4)。Task 6 の `SelectionBar` のテストと Task 7 の e2e `read-only store can select but not delete` に入れる。
5. **バックログの行が同期で消えても、選択とバーが残る。** 3 件を選んだ状態で、読み直しの応答から 1 件が消えると「2 件を選択中」、全部消えるとバーが閉じる(F2b1-3)。Task 5 の `prunes the selection on reload` に入れる。

---

## ファイルの地図

```
internal/app/items/service.go              (変更)StoreList を足す
internal/app/items/delete.go               (変更)BulkTarget・BulkDeletePreview・BulkDeleted・BulkDeletePreview()・BulkDelete()
internal/app/items/delete_test.go          (変更)一括削除のテスト(600 行を超えるなら bulkdelete_test.go に分ける)
internal/adapter/httpapi/items/bulk.go     API 2 本
internal/adapter/httpapi/routes.go         (変更)itemsSvc に StoreList
web/packages/core/src/model/itemAction.ts  (変更)ItemRef・BulkDeletePreview・BulkDeleted
web/packages/core/src/port/itemActions.ts  (変更)bulkDeletePreview・bulkDelete
web/packages/api/src/itemActions.ts        (変更)
web/packages/core/src/features/selection/rowSelection.ts   createRowSelection・selectedText・allReadOnly
web/packages/core/src/features/selection/bulkDelete.ts     createBulkDelete・bulkDeleteDialog・bulkDeletedText
web/packages/core/src/selectionIndex.ts                    公開(index.ts から export *)
web/packages/core/src/features/planning/backlog.ts         (変更)選択をつなぐ・selectedOne
web/packages/core/src/features/planning/backlogSelection.test.ts  (backlog.test.ts は 298 行のため別のファイル)
web/packages/ui/src/patterns/SelectionBar.svelte           バー
web/app/src/screens/planning/RowSprintSelect.svelte        BacklogRow のスプリントのセレクトを切り出す(BacklogRow が 200 行のため)
web/app/src/screens/planning/BacklogRow.svelte・BacklogTab.svelte  (変更)チェック・バー・パレット
web/app/src/screens/item/BulkDeleteDialog.svelte           確認ダイアログ
web/app/src/wireActions.ts                                 (変更)bulkDelete を組み立てる(wire.ts は 298 行のため)
web/app/src/planning.ts                                    (変更)backlogSelection を作り、backlog に渡す
web/e2e/backlog-select.spec.ts
```

---

### Task 1: サーバーの一括削除(`app/items`)

**Files:**
- Modify: `internal/app/items/service.go`、`internal/app/items/delete.go`
- Test: `internal/app/items/delete_test.go`(600 行を超えるなら `bulkdelete_test.go`)

**Interfaces:**
- Consumes: `liveBelow`・`appendDeletes`(既存)、`port.StoreList`・`port.StoreInfo`、`r.RetentionDays()`
- Produces:
  - `Service.StoreList port.StoreList`(新しいフィールド)
  - `type BulkTarget struct { Store port.StoreID; ID item.ID }`
  - `type BulkDeletePreview struct { Stores, RetentionDays, ReadOnly int }` — F2b1-10
  - `type BulkDeleted struct { Deleted, ReadOnly int }` — F2b1-12 の M・K
  - `func (s Service) BulkDeletePreview(ctx context.Context, targets []BulkTarget) (BulkDeletePreview, error)`
  - `func (s Service) BulkDelete(ctx context.Context, targets []BulkTarget) (BulkDeleted, error)`

- [ ] **Step 1: 失敗するテストを書く**

テストの組み立ては、Store S1(サンプル、`apptest.Sample`)と S2(サンプル)と S3(サンプル、閲覧のみ)を `apptest.NewStores` に足し、`StoreList: &apptest.StoreList{…}` を渡した `items.Service`。サンプルは Epic HH-1 の下に Task HH-2。`deleteTree` で S1 に Story HH-3(Epic の下)・Step HH-4・HH-5(Task の下)を足す。

- `TestBulkDeleteAcrossStores`: S1 の Task HH-2(下に Step 2 件)と S2 の Epic HH-1(下に Task HH-2)と S3 の HH-2 を渡す → `BulkDeleted{Deleted: 5, ReadOnly: 1}`。S1 のアクティビティの最後の 1 行は「3 件を一括削除」、S2 は「2 件を一括削除」。S3 には何も書かない。S1 の HH-2 の削除の行は `root: true`、Step は `root: false`。
- `TestBulkDeleteOverlap`(Review Focus 1): S1 の Epic HH-1(下に Task HH-2・Story HH-3・Step HH-4・HH-5)と HH-3 を、HH-3 を先にして渡す → HH-1 は `root: true`、HH-3 は `root: false` で、HH-3 の削除の行は 1 つだけ。`Deleted` は 5、アクティビティは「5 件を一括削除」。
- `TestBulkDeleteSkipsDeleted`(Review Focus 2): HH-3 を `Delete` で消してから HH-3・HH-2 を渡す → `Deleted` は HH-2 とその Step 2 件の 3、エラーなし。全部が削除済みなら `Deleted` 0 で、アクティビティの行は増えない(保存しない)。
- `TestBulkDeleteUnknownStore`: S1 の HH-2 と、一覧にない Store "S9" のアイテムを渡す → `errors.Is(err, port.ErrStoreNotFound)` で、S1 にも何も書かない。
- `TestBulkDeletePreview`: `rec.SetRetentionDays(S1, 7)`。S1 の 2 件と S3 の 1 件 → `{Stores: 1, RetentionDays: 7, ReadOnly: 1}`。S1 と S2 → `{Stores: 2, RetentionDays: 0, ReadOnly: 0}`。S1 の削除済みのアイテムだけ → `{Stores: 0, RetentionDays: 0, ReadOnly: 0}`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run 'TestBulkDelete' -v`
Expected: FAIL(`BulkDelete` が未定義)

- [ ] **Step 3: `delete.go` に実装する**

Store ごとの束ね(最初に出てきた順)と閲覧のみの判定は `StoreList.List` を 1 回読んで作る。1 つの Store の計画は、次の小さな関数にして、読み取り(件数とアクティビティ)と保存の中(書く)の両方で呼ぶ:

`func bulkPlan(r port.Reader, ids []item.ID) (roots []rootDelete, n int, err error)` — `rootDelete{ id item.ID; below []item.Item }`。`liveBelow` が `port.ErrNotFound` のものは飛ばす。ほかの選んだアイテムの `below` に入るものは root にしない(F2b1-7)。書く行は `appendDeletes` で、n は書く行の数。

`Operation` は 1 件の `Delete` と同じく `Actor: change.User(me)`(`s.Identity.Me(ctx, store)`)。関数 60 行・認知的複雑度 15 に収まるよう、Store ごとの保存は別の関数にする。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/items/ -v -run 'TestBulkDelete|TestDelete'`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/items/
git commit -m "feat(items): 複数の Store のアイテムを Store ごとに一括削除する(F2b1 Task 1)"
```

---

### Task 2: 一括削除の API

**Files:**
- Create: `internal/adapter/httpapi/items/bulk.go`
- Modify: `internal/adapter/httpapi/items/items.go`(`registerBulk(api, svc)` を 1 行)、`internal/adapter/httpapi/routes.go`(`itemsSvc` に `StoreList: deps.StoreList`)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate`)
- Test: `internal/adapter/httpapi/items/bulk_test.go`

**Interfaces:**
- Consumes: Task 1 の `BulkDeletePreview`・`BulkDelete`・`BulkTarget`
- Produces: OperationID `preview-bulk-delete`(`POST /api/bulk-delete/preview`、応答 `{ stores, retentionDays, readOnly }`)、`bulk-delete`(`POST /api/bulk-delete`、応答 `{ deleted, readOnly }`)。本文はどちらも `{ items: [{ store: string, item: string }] }`(`minItems: 1`)。生成される TS の関数名は `previewBulkDelete`・`bulkDelete`

- [ ] **Step 1: 失敗するテストを書く**

既存の `items_test.go` の組み立て(`apptest.StoreList` を渡す)で:
- `TestBulkDeleteAPI`: サンプルの Task HH-2 に Step を 2 件足し、HH-2 を `POST /api/bulk-delete` → 200、`{"deleted":3,"readOnly":0}`。
- `TestBulkDeletePreviewAPI`: 同じ本文で `/preview` → `{"stores":1,"retentionDays":30,"readOnly":0}`。
- `TestBulkDeleteAPIRejects`: `items` が空 → 422。一覧にない Store → 404。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/items/ -run 'TestBulkDelete' -v`
Expected: FAIL(404)

- [ ] **Step 3: `bulk.go` に `registerBulk(api huma.API, svc appitems.Service)` を書き、`make generate` を流す**

エラーの写し方は同じパッケージの `delete.go` に合わせる(`port.ErrStoreNotFound` を 404 にする関数が既存にあればそれを使う)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/adapter/httpapi/... && make generate && git diff --stat api/ web/packages/api/src/generated.ts`
Expected: PASS、生成物の差分は 2 本の API の分だけ

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/httpapi/ api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(api): 一括削除の予告と実行の API を足す(F2b1 Task 2)"
```

---

### Task 3: 選択の状態(core の `rowSelection`)

**Files:**
- Create: `web/packages/core/src/features/selection/rowSelection.ts`、`web/packages/core/src/selectionIndex.ts`
- Modify: `web/packages/core/src/index.ts`(`export * from "./selectionIndex";`)
- Test: `web/packages/core/src/features/selection/rowSelection.test.ts`

**Interfaces:**
- Produces:
  - `interface SelectedRow { storeId: string; itemId: string; display: string; readOnly: boolean }`
  - `interface RowSelection { rows: Readable<SelectedRow[]>; toggle(row: SelectedRow, on: boolean): void; clear(): void; prune(visible: SelectedRow[]): void }` — `prune` は、`visible` の順に、選んでいるものだけを残す(`display`・`readOnly` は `visible` の値で置き換える)
  - `function createRowSelection(): RowSelection`
  - `function isSelected(rows: SelectedRow[], storeId: string, itemId: string): boolean`
  - `function selectedText(n: number): string` — 「{N} 件を選択中」
  - `function allReadOnly(rows: SelectedRow[]): boolean` — 1 件以上で、すべて `readOnly`

- [ ] **Step 1: 失敗するテストを書く**

- `toggles rows and clears`: A・B を on → `rows` は 2 件、A を off → B だけ、`clear` → 空。同じ行を 2 回 on しても 1 件。
- `selection keeps rows of different stores apart`(Review Focus 3): F2b1-1 のとおり。
- `prune keeps visible rows in screen order`: B・A の順に選び、`prune([A, C, B])` → `rows` は A・B の順。`prune([C])` → 空。
- `prune refreshes display and readOnly`: 選んだ A の `display` が "HH-1"、`prune` に "HH-9"・`readOnly: true` で渡すと置き換わる。
- `texts`: `selectedText(3)` は「3 件を選択中」。`allReadOnly([])` は false、全部閲覧のみなら true、1 件でも違えば false。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- rowSelection`
Expected: FAIL(モジュールがない)

- [ ] **Step 3: 実装する**(`reactive/store` の `writable`・`readonly` を使う)

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- rowSelection`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/
git commit -m "feat(core): 画面ごとの行の選択の状態を足す(F2b1 Task 3)"
```

---

### Task 4: 一括削除のダイアログと実行(core の `bulkDelete` と口)

**Files:**
- Create: `web/packages/core/src/features/selection/bulkDelete.ts`
- Modify: `web/packages/core/src/model/itemAction.ts`、`web/packages/core/src/port/itemActions.ts`、`web/packages/api/src/itemActions.ts`、`web/packages/core/src/selectionIndex.ts`、`web/packages/core/src/features/items/actions.test-util.ts`(偽の口に 2 つ足す)
- Test: `web/packages/core/src/features/selection/bulkDelete.test.ts`

**Interfaces:**
- Consumes: Task 2 の `previewBulkDelete`・`bulkDelete`(生成物)、Task 3 の `RowSelection`・`SelectedRow`、`Detail`(`view`・`close`・`reload`)、`Drafts.confirmLeave`・`isItemDraft`、`itemDeleteFailedMessage`
- Produces:
  - model: `interface ItemRef { storeId: string; itemId: string }`、`interface BulkDeletePreview { stores: number; retentionDays: number; readOnly: number }`、`interface BulkDeleted { deleted: number; readOnly: number }`
  - port: `ItemActionsPort.bulkDeletePreview(items: ItemRef[]): Promise<BulkDeletePreview>`、`bulkDelete(items: ItemRef[]): Promise<BulkDeleted>`
  - `interface BulkDeleteDialog { title: string; lines: string[]; ok: string }`
  - `function bulkDeleteDialog(rows: SelectedRow[], p: BulkDeletePreview): BulkDeleteDialog` — F2b1-11
  - `function bulkDeletedText(r: BulkDeleted): string` — F2b1-12 の文
  - `interface BulkDeleteDeps { actions: Pick<ItemActionsPort, "bulkDeletePreview" | "bulkDelete">; toasts: Toasts; detail: Pick<Detail, "view" | "close" | "reload">; drafts: Pick<Drafts, "confirmLeave"> }`
  - `interface BulkDelete { state: Readable<BulkDeleteDialog | null>; open(selection: RowSelection, after: () => Promise<void>): Promise<void>; confirm(): Promise<void>; cancel(): void }`
  - `function createBulkDelete(deps: BulkDeleteDeps): BulkDelete`

- [ ] **Step 1: 失敗するテストを書く**

- `dialog text`: 行 HH-1・HH-2、`{stores:1, retentionDays:30, readOnly:0}` → `title` 「一括削除」、`lines` は `["HH-1、HH-2 と、その下にあるものを削除します。設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。"]`、`ok` 「2 件を削除」。
- `dialog lists ten ids`: 閲覧のみでない 12 行 → 1 段落目は「HH-1、HH-2、…、HH-10 ほか 2 件 と、その下にあるものを削除します。…」(10 件を「、」でつなぐ)、`ok` 「12 件を削除」。
- `dialog for several stores and read-only rows`: 閲覧のみでない 2 行と閲覧のみの 1 行、`{stores:2, retentionDays:0, readOnly:1}` → 1 段落目の最後は「保持日数を過ぎると完全に削除されます。」、2 段落目は「閲覧のみの Store の 1 件は削除しません。」、閲覧のみの行の ID は 1 段落目に出さず、`ok` 「2 件を削除」。
- `deleted text`: `{deleted:4, readOnly:0}` → 「4 件を削除しました」。`{deleted:4, readOnly:1}` → 「4 件を削除しました。1 件は閲覧のみの Store のためそのままです」。
- `confirm deletes, clears and reloads`: 選択 2 件で `open` → 偽の口の `bulkDeletePreview` に 2 件の `ItemRef`(閲覧のみの行も含めて全部送る)。`confirm` → `bulkDelete` に同じ 2 件、トースト「3 件を削除しました」、`selection.rows` は空、`state` は null、`after` が 1 回、`detail.reload` が 1 回。
- `confirm closes the open panel`: 詳細パネルが選んだ S1 の X を開いている → `detail.close` が呼ばれる。S2 の X を開いているときは呼ばない(F2b1-1)。
- `nothing left to delete`: `bulkDelete` が `{deleted:0, readOnly:0}` → トーストを出さず、選択を解除して `after` を呼ぶ。
- `failure`: `bulkDelete` が投げる → エラーのトースト「削除できませんでした」(`Refused` ならその文)、`state` は null、選択はそのまま、`after` が呼ばれる。
- `asks about drafts first`: パネルの `ancestors` に選んだアイテムがあり、`confirmLeave` が false → 予告を読まず、`state` は null のまま。
- `fixed while open`: `open` のあとに `selection.toggle` で 1 件足しても、`confirm` で送るのは開いた時点の 2 件。
- `cancel`: 予告を待っているあいだに `cancel` → 予告が返っても `state` は null。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- bulkDelete`
Expected: FAIL

- [ ] **Step 3: 実装する**

`open` の流れと世代(`seq`)・実行中(`busy`)の扱いは `features/items/delete.ts` の `createItemDelete` に合わせる。api の口は `call(async () => …)` で包み、本文は `{ items: items.map((i) => ({ store: i.storeId, item: i.itemId })) }`。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- bulkDelete && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/ web/packages/api/src/itemActions.ts
git commit -m "feat(core): 一括削除の確認ダイアログと実行を足す(F2b1 Task 4)"
```

---

### Task 5: バックログに選択をつなぐ(core)

**Files:**
- Modify: `web/packages/core/src/features/planning/backlog.ts`
- Test: `web/packages/core/src/features/planning/backlogSelection.test.ts`(既存の `backlog.test.ts` の組み立て関数があれば import して使う。なければ同じ形で書く)

**Interfaces:**
- Consumes: Task 3 の `RowSelection`・`SelectedRow`、`SelectedItem`(`features/palette/selection.ts`)
- Produces:
  - `BacklogDeps` に `selection: RowSelection` と `isReadOnly(storeId: string): boolean` を足す
  - `Backlog.selection: RowSelection`(deps のもの)
  - 読み直しのあとの `prune` には、内部の `visibleRows(s: BacklogState): SelectedRow[]`(全枠の行を上から)を渡す
  - `Backlog.selectedOne(): SelectedItem | null` — F2b1-14。`source: "backlog"`、`setStatus` は `setStatus(row, id)`

- [ ] **Step 1: 失敗するテストを書く**

- `prunes the selection on reload`(Review Focus 5): スプリントの枠の A とバックログの枠の B・C を選び、偽の口の `backlog` が C を返さなくなってから `reload` → `selection.rows` は A・B(上からの順)。全部返さなくなれば空。
- `reload failure keeps the selection`: 読み直しが投げたら、選択はそのまま。
- `clears on close and on another workspace`: 選んだあと `close()` → 空。選んだあと別のワークスペースを `open` → 空。同じワークスペースの `open` では残る。
- `rows carry read-only`: `isReadOnly` が true の Store なら、`prune` のあとの行の `readOnly` は true。
- `selectedOne`: 0 件・2 件なら null。1 件なら `{storeId, itemId, source: "backlog"}` で、その `setStatus("done")` は `items.setProperty(storeId, itemId, "status", "done")` を呼び、行がバックログの枠にあれば消えるときのトースト(F2a-10 の(1))を出す。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- backlogSelection`
Expected: FAIL

- [ ] **Step 3: 実装する**(`reload` で `state.set` した直後に `prune`、`open` の別のワークスペースと `close` で `clear`。`backlog.ts` は 300 行に収める)

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- backlog`
Expected: PASS(既存の `backlog.test.ts` も、deps に `selection: createRowSelection()`・`isReadOnly: () => false` を足して通す)

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/features/planning/
git commit -m "feat(core): バックログの読み直しで選択から外し、1 件の選択をパレットに渡す(F2b1 Task 5)"
```

---

### Task 6: 選択バーと行のチェック・一括削除のダイアログ(画面)

**Files:**
- Create: `web/packages/ui/src/patterns/SelectionBar.svelte`、`web/app/src/screens/planning/RowSprintSelect.svelte`、`web/app/src/screens/item/BulkDeleteDialog.svelte`
- Modify: `web/packages/ui/src/index.ts`、`web/app/src/screens/planning/BacklogRow.svelte`・`BacklogTab.svelte`、`web/app/src/planning.ts`、`web/app/src/wireActions.ts`、`web/app/src/App.svelte`(`BulkDeleteDialog` を `DeleteItemDialog` の隣に置く)
- Test: `web/packages/ui/src/patterns/SelectionBar.test.ts`(`web/app` には単体テストの仕組みがないので、行のチェックは Task 7 の e2e で確かめる)

**Interfaces:**
- Consumes: Task 3 の `selectedText`・`allReadOnly`・`isSelected`・`createRowSelection`、Task 4 の `createBulkDelete`、Task 5 の `backlog.selection`・`backlog.selectedOne`、`selection`(`web/app/src/palette.ts`)
- Produces:
  - `SelectionBar` の Props: `{ text: string; readOnly: boolean; ondelete: () => void; onclear: () => void }`。ボタンは「一括削除…」「選択を解除」。`readOnly` なら「一括削除…」を無効にして `title` に `readOnlyNote`。F2b2 が `onchange` と「一括変更…」を先頭に足す
  - `web/app/src/planning.ts` の `backlogSelection`、`web/app/src/wireActions.ts` の `bulkDelete`
  - `RowSprintSelect` の Props: `{ row: BacklogRow; sprint: string; overview: PlanningOverview | null; readOnly: boolean }`(`BacklogRow` のスプリントの `Select` と `pickSprint` をそのまま移す)

- [ ] **Step 1: 失敗するテストを書く**

- `SelectionBar`: `text` 「2 件を選択中」が出る。「一括削除…」で `ondelete`、「選択を解除」で `onclear`。`readOnly` なら「一括削除…」が `disabled` で `title` が「閲覧のみの Store です」、「選択を解除」は押せる(Review Focus 4)。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/ui test -- SelectionBar`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `BacklogRow`: 先頭(持ち手の前)に `<input type="checkbox" aria-label={`${row.display} を選択`} checked={isSelected($rows, storeId, row.id)} onchange={(e) => backlog.selection.toggle({...}, e.currentTarget.checked)}>`。行の `storeId` は `BacklogTab` から渡す。スプリントのセレクトは `RowSprintSelect` に移して 200 行に収める。
- `BacklogTab`: 枠の `{#each}` の上に、`$rows.length > 0` のとき `SelectionBar`(`text={selectedText($rows.length)}`・`readOnly={allReadOnly($rows)}`・`ondelete={() => void bulkDelete.open(backlog.selection, () => backlog.reload())}`・`onclear={() => backlog.selection.clear()}`)。`$effect` で `selection.provide(2, () => backlog.selectedOne())` を登録し、返す関数で外す。
- `planning.ts`: `export const backlogSelection = createRowSelection();` を作って `createBacklog` に `selection: backlogSelection, isReadOnly: (id) => catalog.store(id)?.readOnly ?? false` を渡す。
- `wireActions.ts`: `export const bulkDelete = createBulkDelete({ actions: itemActionsPort, toasts, detail, drafts })`。画面を閉じるとき(既存の `itemDelete.cancel()` の並び)に `bulkDelete.cancel()` も呼ぶ。
- `BulkDeleteDialog.svelte`: `DeleteItemDialog.svelte` と同じ形で `ConfirmDialog`(`danger`)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/
git commit -m "feat(web): バックログの行の選択と選択バー、一括削除のダイアログを足す(F2b1 Task 6)"
```

---

### Task 7: e2e と計画の記録

**Files:**
- Create: `web/e2e/backlog-select.spec.ts`
- Modify: `docs/superpowers/plans/2026-11-14-hedgehug-track-f2a.md`(末尾「F2b に渡すもの」の各行に、F2b1 で済んだもの・F2b2 に回したものを 1 行ずつ添える)、`docs/superpowers/plans/2026-11-14-hedgehug-track-a5c.md`(374・375 行の F2 の行に「F2b1 で足した」を添える)

- [ ] **Step 1: e2e を書く**

`backlog.spec.ts` の `createWorkspace`・`projectWithEpic`・`storyUnderEpic` で、Story を 3 件(接頭辞-2・-3・-4)作り、バックログのタブを開く。

- `selects rows and clears`: -2・-3 のチェック(`getByLabel("{ID} を選択")`)を押す → 「2 件を選択中」が出る。「選択を解除」 → バーが消え、チェックが外れる。
- `bulk deletes the selected rows`: -2・-3 を選び「一括削除…」 → ダイアログ「一括削除」の本文が「{-2}、{-3} と、その下にあるものを削除します。設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。」。「2 件を削除」 → トースト「2 件を削除しました」、-2・-3 の行とバーが消え、-4 は残る。
- `palette sees one selected row`: -4 だけを選んで `ControlOrMeta+k` → 「{-4} に対して」の群が出る。Esc で閉じ、-2 も選んで開き直すと出ない。
- `leaving the tab clears the selection`: -4 を選んで「スプリント」のタブへ移り、「バックログ」へ戻ると、バーが出ていない。
- `read-only store can select but not delete`(Review Focus 4): `/s/readonly/w/HH/backlog`(e2e の閲覧のみの Store。サンプルの Task HH-2 がバックログの枠にある)で「HH-2 を選択」は押せ、「1 件を選択中」が出て、「一括削除…」は無効で `title` が「閲覧のみの Store です」。

- [ ] **Step 2: 流して通ることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/backlog-select.spec.ts`
Expected: 5 passed

- [ ] **Step 3: 計画の記録を直す**(上の Files のとおり)

- [ ] **Step 4: コミット**

```bash
git add web/e2e/backlog-select.spec.ts docs/superpowers/plans/
git commit -m "test(e2e): バックログの選択と一括削除を通しで確かめ、F2a・A5c の計画に記録する(F2b1 Task 7)"
```

---

## 仕上げ

- 最終レビュー(superpowers:requesting-code-review)で、ブランチ全体を仕様 04「一括選択と一括変更・一括削除」の選択・一括削除と、05 の 35 行と照らす。
- PR は develop 向けに 1 本。本文に、計画から変えたことと、F2b2 に渡すもの・E に求めることを書く。

---

## F2b2 に渡すもの

- 選択バーの「一括変更…」(`SelectionBar` の先頭に足し、`onchange` を受ける。閲覧のみの無効も同じ)と、ダイアログ「選択したアイテムを一括変更」から結果のトーストまで(04 の 827〜897 行)。
- 一括変更の対象は `RowSelection.rows` を開いた時点で固定し、変更のあとは選択を解除しない(04 の 897 行)。消えたものは `prune` で外れる。
- バックログの一括変更のトーストの末尾「。{K} 件は完了のため、バックログには表示されません」(`goneText` と同じ分類の判定)。
- Store ごとの保存と閲覧のみの数え方は、F2b1-6 の `BulkDelete` の束ね方に合わせる。

## E に求めること

- 検索のテーブルの行の先頭のチェックと見出しの「すべて選択」(04 の 717 行)を `createRowSelection` でつなぎ、テーブルの読み直しのたびに `prune`(並べ替えたあとの順、同じアイテムは 1 回)、画面を離れたら `clear`。
- テーブルの上に `SelectionBar` と、一括削除は `bulkDelete.open(selection, 読み直し)`。複数の Store の保持日数と閲覧のみの数え方はサーバー(F2b1-10)が返す。
- パレットの `selection.provide(2, …)` をテーブルにも登録する(ちょうど 1 件のとき)。「{N} 件をまとめて変更…」は F2b2 の一括変更のダイアログを開く。
