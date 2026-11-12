# hedgehug トラック B2(プロパティと値、設定の画面)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 03 の「プロパティ」「値の編集」と設定の画面(8 つのサブタブ、一般、作成ダイアログ、サイドバーのメニュー)を作る。値やプロパティを削除すると、それを使っていたアイテムの値も置き換わる・消える。

**Architecture:** 設定はワークスペースの `log.jsonl` の項目で持つ。B1 の `statuses/<ID>/name` などに、選択肢 `choices/<プロパティ ID>/<値 ID>/…` と削除の行 `t: delete` を足す。アイテムの値(`status`・`priority`・`systems`・`fields/<プロパティ ID>`)は、先に共有の小さな PR で `domain/item` と `item.yaml` に足す。操作は `app/workspaces` に 1 ファイル 1〜数操作で置き、HTTP は `adapter/httpapi/workspaces` に足す。画面のロジックは `core/features/workspaces`、見た目は `app/screens/workspace-settings`。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、go-git、Svelte 5、orval、Vitest、Playwright)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`(正本)。範囲は「範囲と画面の構成」「ワークスペースの作成」「一般」の名前・接頭辞、「プロパティ」「値の編集」「変更が及ぶ範囲」のプロパティ・値の行
- 04「サイドバー」の「並び順と並べ替え」「Store のメニュー」「ワークスペースのメニュー」
- 01「画面のアドレス」(ワークスペースのタブ。設定のサブタブは履歴に積まない)、「書き込みの単位」「閲覧のみの Store」
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「アイテムの例」「変更記録」「いまの値の決め方」「同期の修復」)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(B の全体の計画。決定 1〜13 と漏れの表を引き継ぐ)
- `docs/superpowers/plans/2026-11-13-hedgehug-track-a2.md`(画面の作り。ルーター・サイドバー・トースト・Refused)

**持ち場:** `internal/domain/workspace`、`internal/app/workspaces`、`internal/adapter/httpapi/workspaces`、`internal/infra/gitstore/yaml*.go`(注釈と書き出し)、`web/packages/core/src/features/workspaces`、`web/packages/api/src/workspaces.ts`、`web/app/src/screens/workspace-settings`、`web/app/src/shell`(Store とワークスペースのメニューの項目)。共有部分は Task 1(`domain/change`・`domain/item`・`gitstore` の item.yaml・`apptest` の照合テスト)。

**待つもの:** B1(#149)と A2(#157)が develop に入っていること(入っている)。

## Global Constraints

土台と B の計画の Global Constraints をすべて引き継ぐ。主なもの:

- 依存の向き: `app/workspaces` は `domain` と `app/port` だけを import する。`app/items` など、ほかの `app/<機能>` は import しない。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージ 20 ファイル(テストを除く)、Svelte 200 行、TS 300 行。`internal/archtest` と lint が確かめる。
- 画面の文言、アクティビティの文、断りの文は、仕様 03 の文字どおりに書く。仕様にない文言は足さない。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。API を変えたら `make generate` と `cd web && pnpm gen` で作り直し、同じコミットに入れる。
- 依存は足さない。足すなら版を明記し、公開から 7 日以上たった版だけを使う。
- 計画やテストにトークンらしい値(`ghp_` など)を書かない。
- 各タスクは TDD で進める(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクの終わりに `make check` を通す(画面のタスクは `cd web && pnpm check` と該当の e2e も)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| B2-1 | アイテムの `status`・`priority`・`systems`・選択式の追加プロパティ `fields/<プロパティ ID>` は B2 で足す。項目の追加と `item.yaml` への書き出しは、共有の小さな PR(Task 1)で先に入れる。初期値、ステータスの変更操作、遷移の検査、詳細パネルの入力は A3 が作る | ユーザー決定(2026-11-13) |
| B2-2 | `item.Item` に `Status`・`Priority`(`change.ID`。ないときは空)・`Systems`(`[]change.ID`)・`Choices`(`map[change.ID][]change.ID`。単一選択は 1 つだけ入る)を足す。`fields/<ID>` の行は、単一選択は `set`(値の ID か null)、複数選択は `add`・`remove`。どちらかは、その項目に `add`・`remove` の行があるか、作成時の値が配列かで決める | 保存形式「アイテムの例」(`fields: {F1: F1V2}`)と「変更記録」(複数選択は `add`・`remove`)。`domain/item` はプロパティの型を知らないので、行の形から決める |
| B2-3 | 削除は `change.Delete`(`t: "delete"`)の行で書く。値は `f: "statuses/<ID>"`・`"choices/<プロパティ ID>/<ID>"`、プロパティは `f: "properties/<ID>"`。削除の行がある値・プロパティは設定から除く。復元(`restore`)を読むのは、復元を作るトラック(C の同期の修復、G の削除済み)が足す | 保存形式「同期の修復」(8)「値(ステータス・テンプレート)に delete がある」。B2 は復元を作らない |
| B2-4 | 選択肢は `choices/<プロパティ ID>/<値 ID>/{name,color,rank}` に持ち、`Property.Choices` で返す | 決定 2(1 つの値ごとに 1 項目)を選択肢に当てはめた |
| B2-5 | 作成後に足すプロパティ・値は、項目ごとの `set` の行(`from` は null)で書く。使う種類は `add`・`remove` | 項目ごとの後勝ち(06)。B1 の作成の 1 行と同じ項目名 |
| B2-6 | 値とプロパティの並べ替えは、隣と `rank` を入れ替える(2 行)。ワークスペースの並べ替えも、アイテムの「上へ移動」(A1・A2)と同じく、隣と入れ替える | 03「並べ替え」(隣と入れ替える)。ワークスペースは既存の作りにそろえる |
| B2-7 | ワークスペースの並びは、ワークスペースのログの `rank` に持つ。作成のときに、Store の末尾(いちばん大きい `rank` の `rank.After`)を `set rank` で書く。一覧は `rank`、同じなら内部 ID の順。B1 のころに作った `rank` のないワークスペースは先頭に並ぶ | 04「並び順と並べ替え」(新しく作ったものは末尾)。`workspace.Workspace` の型は変えない(B の決定 1) |
| B2-8 | 単一の値(ステータス・優先度・単一選択)の削除では、使っているアイテムに `set` を書いて代わりの値(単一選択は null も可)に置き換える。複数の値(関連システム・複数選択)の削除と、追加プロパティの削除では、`remove`(複数)か `set` null(単一)でアイテムから値を外す。アイテムの書き込みは、ワークスペースの削除の行と同じ保存に入れる | 03「値の編集」の「削除」、「プロパティ」の「削除」。01「書き込みの単位」 |
| B2-9 | 「履歴には残らない」(複数の値の削除、プロパティの削除)は、履歴の画面(A4)が、ワークスペースのログの `delete` と同じ `op` のアイテムの行を出さないことで満たす。B2 は行を書くだけ | 行を書かずに読むときに除く案は、同期の修復(C)が値の削除を取り消したときに古い値が戻ってしまう |
| B2-10 | 置き換えの検査をしない・自動化を動かさない(03)。B2 の時点では遷移の制約も自動化もないので、何もしない。B3・B4 は、値の削除の操作に「検査を呼ばない」ことを守る | 03「削除」 |
| B2-11 | 使っているアイテムの件数・接頭辞の確認の N と ID の例は、`GET …/workspaces/{workspace}/usage` で 1 回で返す。件数はすべてのアイテム(アイテムの削除は A4 なので、いまは削除済みがない)。A4 がアイテムの削除を入れるときに、削除済みを除く | 03「接頭辞」(削除済みでないアイテムの番号の小さい方から 3 件)、「値の編集」(削除済みを除く件数) |
| B2-12 | 操作に合わない要求(ステータスの使う種類や非表示を変える、組み込みを消す、分類の違うステータスを分類の既定にする、ない値を代わりにする)は `port.ErrNotFound`(404)にする。画面はこれらを押せないようにしている | 仕様に文言がない。画面からは起きない |
| B2-13 | 値を足すときの名前の空は「名前は空にできません」で断る | 03「名前」の空の文言を、追加にも当てはめた(追加の重複は同じ文言で断ると 03 にある) |
| B2-14 | 色は 03 の 18 色の名前で持つ(B の決定 5)。値を足したときの色は `Colors[len(いまの値) % 18]` | 03「色」 |
| B2-15 | ワークスペースの画面のタブは、B2 では「概要」と「設定」だけを出す。ほかのタブ(ツリー・カンバンほか)は、作るトラック(E・F)が足す。アドレスは `/s/{store}/w/{prefix}/{tab}`(タブを省くと概要)。設定のサブタブはアドレスに入れず、画面の状態として持つ | 01「画面のアドレス」(ワークスペースはタブを区別する。設定のサブタブの切り替えは履歴に積まない)。A2 のプロジェクトのアドレスの形にそろえる |
| B2-16 | 「ワークフロー」「テンプレート」「自動化」のサブタブは押せるようにし、中は空にする(B3・B4 が中身を足す)。「プロジェクト別のルールがあるプロジェクト」の 1 行は B5 | 03「範囲と画面の構成」(サブタブの 8 つ)。仕様にない文言を足さない |
| B2-17 | Store のメニューは B2 が作り、「ワークスペースを追加」だけを置く。ほかの項目(今すぐ同期・Store の設定・サイドバーで非表示)は C・G が足す。ワークスペースのメニューには「設定を開く」「上へ移動」「下へ移動」を足す。「削除…」は B5 | 04「Store のメニュー」「ワークスペースのメニュー」。A2 が「プロジェクトを作成」だけを置いたのと同じ分け方 |
| B2-18 | 作成ダイアログの Store の候補は、閲覧のみでない Store。非表示の Store と既定の Store はまだない(G)ので、非表示の判定と、パレットからの初期値は G が足す。コマンドパレットの「ワークスペースを追加」は、パレットを作るトラック(E)が足す | 03「ワークスペースの作成」。いまの StoreInfo に非表示と既定がない |
| B2-19 | `workspace.yaml` に設定(プロパティ・選択肢・値・初期値・分類の既定・並び)を書き出す。`item.yaml` の `status`・`priority`・`systems`・`fields` の行末に、値の名前(`fields` はプロパティ名: 値の名前)を注釈で書く。値・プロパティの名前を変える保存と、値・プロパティを消す保存では、そのワークスペースのすべての `item.yaml` を同じコミットで作り直す。同期のあとの作り直しは C の持ち場(漏れの表) | B の決定 11、保存形式「アイテムの例」の下の段落 |
| B2-20 | カンバンの WIP 上限の付け替え(ステータスの改名・削除)と、保存したビューの検索条件・列の扱いは、カンバンと保存したビューを作るトラック(E)が作る。B2 の時点では付け替える上限がない | 03「名前」「削除」。いま WIP 上限も保存したビューもない |
| B2-21 | 値・プロパティの削除で、テンプレート・ワークフロー・自動化・プロジェクト別のルールを書き換えるのは、それぞれを作る B3・B4・B5 が、値の削除の操作に足す | B の小計画の一覧 |

## 漏れの表(B2 の範囲 → タスク、範囲の外 → 持ち主)

| 仕様の節・項目 | 持ち場 |
|---|---|
| 03 範囲と画面の構成: 8 つのサブタブ、閲覧のみの見た目 | Task 11 |
| 03 範囲と画面の構成: サイドバーのワークスペースのメニュー「設定を開く」「上へ移動」「下へ移動」 | Task 7(並び)・Task 14(メニュー) |
| 同: 「削除…」、サブタブの上の「プロジェクト別のルールがあるプロジェクト」の 1 行 | B5 |
| 03 冒頭の表: プロパティ・値・初期値・分類の既定の行(文とトースト) | Task 4・5(文)、Task 12・13(トースト) |
| 03 名前の重複(プロパティ・値) | Task 3 |
| 03 同期で同じ名前・キーが 2 つ、初期ステータスの戻し、削除の取り消し | トラック C(B の決定 10) |
| 03 ワークスペースの作成: ダイアログ、Store の候補、作ったあとに概要を開く、Store がないときの文 | Task 14 |
| 同: 非表示の Store の候補と文、パレットから開いたときの既定の Store | G(B2-18) |
| 同: コマンドパレットの「ワークスペースを追加」 | E(B2-18) |
| 同: Store の追加ダイアログで作るワークスペース | G(08 の Store の追加) |
| 03 一般 / 名前・接頭辞の画面(確認ダイアログの N と ID の例) | Task 6(件数)・Task 11(画面) |
| 03 プロパティ: 一覧・組み込み・追加・使う種類・非表示・並べ替え・削除 | Task 3・4(サーバー)、Task 13(画面) |
| 同: 使わない種類のアイテムで詳細パネル・作成ダイアログに出さない、テーブルのセルを「—」に | A3(詳細パネル・作成ダイアログ)、E(テーブル) |
| 同: 非表示のプロパティを詳細パネルと作成ダイアログから隠す | A3 |
| 同: 削除で保存したビューに添える文、検索欄の「「customer」というキーはありません」 | E(保存したビューがまだない。B2-20) |
| 03 値の編集: 色・名前・分類・分類ごとの既定・初期ステータス・初期優先度・並べ替え・削除 | Task 3・5(サーバー)、Task 12(画面) |
| 同: ステータスの改名・削除での WIP 上限 | E(B2-20) |
| 同: 削除でのテンプレート・ワークフロー・自動化・プロジェクト別のルールの書き換え | B3・B4・B5(B2-21)。ステータスの削除ダイアログの「ワークフローの制約と自動化(プロジェクト別のルールを含む)からも削除されます。」の文は Task 12 で出す |
| 同: 初期値と分類の既定を使う場面(作成・Step・複製・Space・カンバン・自動化・移動) | A3・A4・E・B4 |
| 03 変更が及ぶ範囲: プロパティ・値の行 | Task 4・5 |
| 保存形式: item.yaml・workspace.yaml の注釈と書き出し | Task 1(項目)・Task 8(注釈と設定の書き出し) |
| 保存形式: 同期のあとの `*.yaml` の作り直し | C |
| 履歴の画面で、削除と同じ保存のアイテムの行を出さない | A4(B2-9) |
| 削除済みのアイテムを件数から除く | A4(B2-11) |
| サイドバーのドラッグでの並べ替え(ワークスペース・プロジェクト) | 持ち主未定(A2 の残り。B2 はメニューだけ) |

## タスクの順と PR

| Task | 中身 | PR |
|---|---|---|
| 1 | アイテムの値と削除の行(共有) | 単独の小さな PR を先に develop へ |
| 2〜9 | サーバー(設定の読み書き、検査、操作、件数、並び、yaml、API) | B2 の PR(Task 1 の上に積む) |
| 10〜15 | 画面と e2e | 同じ B2 の PR |

Task 2〜6 は順に行う(4・5 は 2・3 の型と検査を使う)。Task 7・8 は Task 2 のあとならいつでもよい。Task 9 は 4〜7 のあと。Task 10 以降は Task 9 の API のあと。

## ファイルの地図

```
共有 PR(Task 1)
internal/domain/change/record.go          Delete を足す
internal/domain/change/draft.go           NewDelete
internal/domain/item/values.go            Status・Priority・Systems・Choices の読み取り、FieldKey、MultiChoice
internal/domain/item/item.go              Item に項目を足し、FromLog から readValues を呼ぶ
internal/infra/gitstore/yaml.go           item.yaml に status・priority・systems・fields
internal/app/apptest/stores_contract_values.go  照合テスト(偽物と gitstore)

サーバー(Task 2〜9)
internal/domain/workspace/settings.go     Property.Choices
internal/domain/workspace/lists.go        List(値の一覧の指し方)・Label・Values
internal/domain/workspace/settings_log.go 削除・選択肢を読む
internal/domain/workspace/drafts.go       PropertyDrafts・ValueDrafts・SwapRanks の行
internal/domain/workspace/property_check.go  プロパティの検査と文言
internal/domain/workspace/value_check.go     値の検査と文言
internal/domain/workspace/order.go        RankOf・SortByRank(ワークスペースの並び)
internal/app/workspaces/edit.go           edit(設定の 1 回の変更の共通部分)
internal/app/workspaces/property_add.go   AddProperty
internal/app/workspaces/property_edit.go  RenameProperty・SetPropertyKinds・SetPropertyHidden・MoveProperty
internal/app/workspaces/property_delete.go DeleteProperty
internal/app/workspaces/value_add.go      AddValue
internal/app/workspaces/value_edit.go     RenameValue・SetValueColor・SetValueCategory・MoveValue
internal/app/workspaces/value_marks.go    SetInitial・SetCategoryDefault
internal/app/workspaces/value_delete.go   DeleteValue
internal/app/workspaces/items.go          workspaceItems(ワークスペースのすべてのアイテム)
internal/app/workspaces/usage.go          Usage
internal/app/workspaces/order.go          MoveWorkspace、List の並び、作成の rank
internal/infra/gitstore/yaml_settings.go  workspace.yaml の設定、item.yaml の注釈
internal/infra/gitstore/commit.go         ワークスペースを書いたら、そのアイテムの item.yaml も作り直す
internal/adapter/httpapi/workspaces/settings.go   プロパティ・値の API
internal/adapter/httpapi/workspaces/usage.go      件数と並べ替えの API

画面(Task 10〜15)
web/packages/core/src/features/router/route.ts     ワークスペースのタブ
web/packages/core/src/model/workspace.ts           WorkspaceSettings ほかの型
web/packages/core/src/port/workspaces.ts           WorkspacesPort
web/packages/core/src/features/workspaces/settings.ts   設定の画面の状態と操作
web/packages/core/src/features/workspaces/values.ts     値の行(件数・印・無効の理由・削除の本文)
web/packages/core/src/features/workspaces/properties.ts プロパティの行
web/packages/core/src/features/workspaces/createDialog.ts ワークスペースの作成ダイアログ
web/packages/api/src/workspaces.ts                 WorkspacesPort の実装
web/app/src/screens/workspace/WorkspaceScreen.svelte タブ
web/app/src/screens/workspace-settings/*.svelte     設定の画面
web/app/src/shell/StoreMenu.svelte・WorkspaceMenu.svelte
web/e2e/settings.spec.ts
```

---

### Task 1(共有 PR): アイテムの値と、削除の行

**Files:**
- Modify: `internal/domain/change/record.go`(`Type` の定数)
- Modify: `internal/domain/change/draft.go`
- Test: `internal/domain/change/record_test.go`
- Create: `internal/domain/item/values.go`
- Modify: `internal/domain/item/item.go`
- Test: `internal/domain/item/values_test.go`
- Modify: `internal/infra/gitstore/yaml.go`
- Test: `internal/infra/gitstore/write_test.go`(item.yaml の中身)
- Create: `internal/app/apptest/stores_contract_values.go`
- Modify: `internal/app/apptest/stores_contract.go`(場面を 1 行足す)

**Interfaces:**
- Produces:
  - `change.Delete Type = "delete"`、`change.NewDelete(f string) change.Draft`
  - `item.Item` の `Status change.ID`・`Priority change.ID`・`Systems []change.ID`・`Choices map[change.ID][]change.ID`
  - `item.FieldKey(property change.ID) string`(`"fields/" + id`)
  - `item.MultiChoice(log []change.Record, property change.ID) bool`

この PR は単独で develop に出す(B2-1)。Task 2 以降は、この PR の上に積む。

- [ ] **Step 1: 削除の行のテストを書く**

`internal/domain/change/record_test.go` に足す(import に `encoding/json`・`strings`・`time` がなければ足す):

```go
func TestDeleteRoundTrip(t *testing.T) {
	d := change.NewDelete("statuses/01VAL00000000000000000000A")
	if d.T != change.Delete || d.F != "statuses/01VAL00000000000000000000A" || d.From != nil || d.To != nil {
		t.Fatalf("draft = %+v", d)
	}
	r := change.Record{
		ID: "01REC00000000000000000000A", Op: "01OP000000000000000000000A",
		At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), By: "taro", Actor: change.User("taro"),
		T: d.T, F: d.F,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"t":"delete","f":"statuses/01VAL00000000000000000000A"`) {
		t.Errorf("line = %s", b)
	}
	var back change.Record
	if err := json.Unmarshal(b, &back); err != nil || back.T != change.Delete || back.F != d.F {
		t.Errorf("back = %+v, %v", back, err)
	}
	// Resolve は削除の行を値として読まない(削除を読むのは、エンティティごとの FromLog)。
	if s := change.Resolve([]change.Record{r}, change.Options{}); s.Created != nil || len(s.Values) != 0 {
		t.Errorf("resolve = %+v", s)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ -run TestDeleteRoundTrip`
Expected: FAIL(`change.NewDelete` が未定義)

- [ ] **Step 3: 実装する**

`record.go` の定数に足す:

```go
	Remove Type = "remove"
	// Delete は削除。f は消すもの(値なら "statuses/<ID>"、プロパティなら "properties/<ID>")。
	Delete Type = "delete"
```

`draft.go` に足す:

```go
// NewDelete は、f が指すものの削除。from も to も持たない。
func NewDelete(f string) Draft { return Draft{T: Delete, F: f} }
```

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/domain/change/`
Expected: PASS

- [ ] **Step 5: アイテムの値のテストを書く**

`internal/domain/item/values_test.go`:

```go
package item_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const (
	sTodo   change.ID = "01STATUS00000000000000000A"
	sDoing  change.ID = "01STATUS00000000000000000B"
	pMedium change.ID = "01PRIO0000000000000000000A"
	yWeb    change.ID = "01SYS00000000000000000000A"
	yAPI    change.ID = "01SYS00000000000000000000B"
	fEnv    change.ID = "01PROP0000000000000000000A" // 単一選択
	fTags   change.ID = "01PROP0000000000000000000B" // 複数選択
	vProd   change.ID = "01CHOICE00000000000000000A"
	vA      change.ID = "01CHOICE00000000000000000B"
	vB      change.ID = "01CHOICE00000000000000000C"
)

// rows は、drafts を 1 行ずつ別の操作として並べる(prev は Stamp が付ける)。
func rows(drafts ...change.Draft) []change.Record {
	var log []change.Record
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	for i, d := range drafts {
		id := change.ID(fmt.Sprintf("01REC%021d", i))
		meta := change.Meta{Op: id, At: at.Add(time.Duration(i) * time.Second), Actor: change.User("taro")}
		log = append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return id })...)
	}
	return log
}

func TestValuesFromLog(t *testing.T) {
	base := item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x"}
	log := rows(
		item.Create(base),
		change.NewSet("status", nil, sTodo),
		change.NewSet("status", sTodo, sDoing),
		change.NewSet("priority", nil, pMedium),
		change.NewAdd("systems", yWeb),
		change.NewAdd("systems", yAPI),
		change.NewRemove("systems", yWeb),
		change.NewSet(item.FieldKey(fEnv), nil, vProd),
		change.NewAdd(item.FieldKey(fTags), vA),
		change.NewAdd(item.FieldKey(fTags), vB),
	)
	it, ok := item.FromLog(log)
	if !ok {
		t.Fatal("not created")
	}
	if it.Status != sDoing || it.Priority != pMedium || !reflect.DeepEqual(it.Systems, []change.ID{yAPI}) {
		t.Errorf("item = %+v", it)
	}
	want := map[change.ID][]change.ID{fEnv: {vProd}, fTags: {vA, vB}}
	if !reflect.DeepEqual(it.Choices, want) {
		t.Errorf("choices = %v, want %v", it.Choices, want)
	}
	if item.MultiChoice(log, fEnv) || !item.MultiChoice(log, fTags) {
		t.Error("MultiChoice: 単一は false、複数は true")
	}
}

// 値のないアイテムは、どれも空。単一選択を null にしたら、選択肢から消える。
func TestValuesCleared(t *testing.T) {
	base := item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x"}
	it, _ := item.FromLog(rows(item.Create(base)))
	if it.Status != "" || it.Priority != "" || it.Systems != nil || it.Choices != nil {
		t.Errorf("empty item = %+v", it)
	}
	it, _ = item.FromLog(rows(
		item.Create(base),
		change.NewSet(item.FieldKey(fEnv), nil, vProd),
		change.NewSet(item.FieldKey(fEnv), vProd, nil),
	))
	if it.Choices != nil {
		t.Errorf("choices = %v", it.Choices)
	}
}

// 作成の行に配列で書いた fields は、複数選択として読む(A3 が作成で書く形)。
func TestChoicesInCreate(t *testing.T) {
	log := rows(change.NewCreate(map[string]any{
		"id": "01ITEM0000000000000000000A", "kind": "Task", "title": "x",
		"status": sTodo, "systems": []change.ID{yWeb},
		item.FieldKey(fEnv): vProd, item.FieldKey(fTags): []change.ID{vA},
	}))
	it, _ := item.FromLog(log)
	want := map[change.ID][]change.ID{fEnv: {vProd}, fTags: {vA}}
	if it.Status != sTodo || !reflect.DeepEqual(it.Systems, []change.ID{yWeb}) || !reflect.DeepEqual(it.Choices, want) {
		t.Errorf("item = %+v", it)
	}
}
```

ID は `change.ValidID` に合う 26 文字にしてある。既存の `item_test.go` に同じ名前の関数や定数があれば、名前をずらす。

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/domain/item/ -run 'TestValues|TestChoices'`
Expected: FAIL(`item.FieldKey`・`it.Status` が未定義)

- [ ] **Step 7: 実装する**

`item.go` の `Item` に足す(`Watchers` の下):

```go
	// 値は内部 ID で指す(保存形式「アイテムの例」)。初期値と変更の操作は A3 が足す(B2-1)。
	Status   change.ID                 // なければ空
	Priority change.ID                 // なければ空
	Systems  []change.ID               // 関連システム
	Choices  map[change.ID][]change.ID // 選択式の追加プロパティの値。単一選択は 1 つ。なければ nil
```

`FromLog` を、ログから複数の項目を決める形にし、最後に `readValues` を呼ぶ。`var opts` は使わなくなるので消す:

```go
func FromLog(log []change.Record) (Item, bool) {
	s := change.Resolve(log, change.Options{Multi: multi(log)})
	if s.Created == nil {
		return Item{}, false
	}
	// …(既存の読み取りはそのまま)
	readValues(&it, s)
	return it, true
}
```

`values.go`:

```go
package item

import (
	"encoding/json"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

const fieldsPrefix = "fields/"

// FieldKey は、追加プロパティの値の項目名(保存形式「変更記録」の fields/<プロパティの内部 ID>)。
func FieldKey(property change.ID) string { return fieldsPrefix + string(property) }

// MultiChoice は、追加プロパティ p の値を複数選択の形(add・remove か、作成の配列)で持っているか。
func MultiChoice(log []change.Record, p change.ID) bool { return multi(log)(FieldKey(p)) }

// multi は、値を複数持つ項目か。fields/<ID> は、その項目に add・remove の行があるか、作成の値が
// 配列なら複数選択(B2-2)。domain/item はプロパティの型を知らないので、行の形で決める。
func multi(log []change.Record) func(f string) bool {
	lists := map[string]bool{"watchers": true, "systems": true}
	for _, r := range log {
		switch {
		case (r.T == change.Add || r.T == change.Remove) && strings.HasPrefix(r.F, fieldsPrefix):
			lists[r.F] = true
		case r.T == change.Create:
			var to map[string]json.RawMessage
			_ = json.Unmarshal(r.To, &to)
			for f, v := range to {
				if strings.HasPrefix(f, fieldsPrefix) && strings.HasPrefix(strings.TrimSpace(string(v)), "[") {
					lists[f] = true
				}
			}
		}
	}
	return func(f string) bool { return lists[f] }
}

func readValues(it *Item, s change.State) {
	it.Status = id(s.Values["status"])
	it.Priority = id(s.Values["priority"])
	it.Systems = ids(s.Lists["systems"])
	for f, raw := range s.Values {
		if v := id(raw); strings.HasPrefix(f, fieldsPrefix) && v != "" {
			it.choice(change.ID(strings.TrimPrefix(f, fieldsPrefix)), []change.ID{v})
		}
	}
	for f, list := range s.Lists {
		if vs := ids(list); strings.HasPrefix(f, fieldsPrefix) && len(vs) > 0 {
			it.choice(change.ID(strings.TrimPrefix(f, fieldsPrefix)), vs)
		}
	}
}

func (it *Item) choice(p change.ID, vs []change.ID) {
	if it.Choices == nil {
		it.Choices = map[change.ID][]change.ID{}
	}
	it.Choices[p] = vs
}

// id は、値の内部 ID。null や内部 ID でない値は空。
func id(raw json.RawMessage) change.ID {
	var v change.ID
	if json.Unmarshal(raw, &v) != nil || !change.ValidID(v) {
		return ""
	}
	return v
}

func ids(list []json.RawMessage) []change.ID {
	var out []change.ID
	for _, raw := range list {
		if v := id(raw); v != "" {
			out = append(out, v)
		}
	}
	return out
}
```

`s.Lists` の並びは値の JSON の辞書順(`change.State` のとおり)なので、テストの `vA`・`vB` はこの順に並ぶ。

- [ ] **Step 8: 通るのを確かめる**

Run: `go test ./internal/domain/item/`
Expected: PASS

- [ ] **Step 9: item.yaml のテストを書く**

`internal/infra/gitstore/write_test.go` に、既存の item.yaml のテスト(`watchers` を確かめているもの)と同じ準備で、値を持つアイテムと値のないアイテムを書き、次を確かめるテスト `TestItemYAMLValues` を足す。定数は Step 5 と同じ値をこのファイルに置く:

```go
	for _, want := range []string{
		"status: " + string(sDoing) + "\n",
		"priority: null\n",
		"systems: [" + string(yAPI) + "]\n",
		"fields:\n    " + string(fEnv) + ": " + string(vProd) + "\n    " + string(fTags) + ": [" + string(vA) + ", " + string(vB) + "]\n",
	} {
		if !strings.Contains(y, want) {
			t.Errorf("item.yaml に %q がない:\n%s", want, y)
		}
	}
```

字下げの幅は、既存の `encodeYAML` の設定に合わせる(テストを書くときに、いまの item.yaml の出力で確かめる)。値のないアイテムは `status: null`・`priority: null`・`systems: []`・`fields: {}` になることも確かめる。

- [ ] **Step 10: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ -run TestItemYAMLValues`
Expected: FAIL

- [ ] **Step 11: 実装する**

`yaml.go` の `itemYAML` の `Created` と `Watchers` のあいだに、保存形式「アイテムの例」の並びで足す:

```go
	Created  createdYAML `yaml:"created,flow"`
	Status   *string     `yaml:"status"`
	Priority *string     `yaml:"priority"`
	Systems  []string    `yaml:"systems,flow"`
	Fields   yaml.Node   `yaml:"fields"`
	Watchers []string    `yaml:"watchers,flow"`
```

`renderYAML` の `change.KindItem` の枝で、`itemToYAML(it, log)` とログも渡す。`itemToYAML` では、空の ID を nil、`Systems` を空でも `[]string{}` にする。`Fields` は `yaml.Node{Kind: yaml.MappingNode}` を作り、プロパティの ID の辞書順に、キーと値の `yaml.Node` を足す。値は `item.MultiChoice(log, p)` が真なら `yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}`(各 ID を `ScalarNode` で)、偽なら `ScalarNode` 1 つ。空なら `Style: yaml.FlowStyle` の空の `MappingNode`(`{}` で出る)。

- [ ] **Step 12: 通るのを確かめる**

Run: `go test ./internal/infra/gitstore/`
Expected: PASS

- [ ] **Step 13: 照合テストを書く**

`internal/app/apptest/stores_contract_values.go`:

```go
package apptest

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// contractItemValues は、アイテムの値(ステータス・優先度・関連システム・選択肢)と、ワークスペースの
// 削除の行が、書いたとおりに読めることを確かめる。
func contractItemValues(t *testing.T, s port.Stores, id port.StoreID) {
	const (
		st   change.ID = "01CONTRACTST00000000000001"
		pr   change.ID = "01CONTRACTPI00000000000001"
		sy   change.ID = "01CONTRACTSY00000000000001"
		prop change.ID = "01CONTRACTPP00000000000001"
		ch   change.ID = "01CONTRACTCH00000000000001"
	)
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(cWorkspace.ID, change.NewDelete("statuses/"+string(st))); err != nil {
			return err
		}
		return tx.WriteItems().Append(cItem.ID,
			change.NewSet("status", nil, st), change.NewSet("priority", nil, pr),
			change.NewAdd("systems", sy), change.NewAdd(item.FieldKey(prop), ch))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		it, err := r.Items().Get(cItem.ID)
		want := map[change.ID][]change.ID{prop: {ch}}
		if err != nil || it.Status != st || it.Priority != pr ||
			!reflect.DeepEqual(it.Systems, []change.ID{sy}) || !reflect.DeepEqual(it.Choices, want) {
			t.Errorf("item = %+v, %v", it, err)
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		if err != nil || log[len(log)-1].T != change.Delete || log[len(log)-1].F != "statuses/"+string(st) {
			t.Errorf("workspace log = %+v, %v", log, err)
		}
		return nil
	})
}
```

`stores_contract.go` の `cases` の末尾に `{"item values and deletes round trip", contractItemValues},` を足す。

- [ ] **Step 14: 偽物と gitstore の両方で通るのを確かめる**

Run: `go test ./internal/app/apptest/ ./internal/infra/gitstore/`
Expected: PASS。gitstore が `delete` の行を書けない・読めないときは、行の種類を検査している箇所(`grep -rn "change.Remove" internal/infra/gitstore`)に `change.Delete` を足す。

- [ ] **Step 15: 全体を確かめてコミットし、PR を出す**

Run: `make check`
Expected: PASS

```bash
git add internal/domain/change internal/domain/item internal/infra/gitstore internal/app/apptest
git commit -m "feat(item): アイテムのステータス・優先度・関連システム・選択肢の値と、削除の行を足す"
```

PR は develop 向けに単独で出す(ブランチ `feat/track-b2-item-values`。題「アイテムの値と削除の行(トラック B2 の共有部分)」)。本文に B2-1〜B2-3 と、A3 への申し送りを書く: 作成時の初期値・変更の操作・選択式でない追加プロパティ(テキスト・数値など)の値は A3 が `fields/<ID>` に足す。`readValues` は内部 ID でない値を読み飛ばすので、型ごとの読み分けは A3 で足す。

---

### Task 2: 設定のログに選択肢と削除を足す

**Files:**
- Modify: `internal/domain/workspace/settings.go`(`Property.Choices`)
- Create: `internal/domain/workspace/lists.go`
- Modify: `internal/domain/workspace/settings_log.go`
- Create: `internal/domain/workspace/drafts.go`
- Test: `internal/domain/workspace/lists_test.go`、`internal/domain/workspace/settings_log_test.go`

**Interfaces:**
- Consumes: `change.NewDelete`・`change.Delete`(Task 1)
- Produces:
  - `Property.Choices []Value`(選択肢。なければ nil)
  - `type ListKind string`、定数 `ListStatuses = "statuses"`・`ListPriorities = "priorities"`・`ListSystems = "systems"`・`ListChoices = "choices"`
  - `type List struct{ Kind ListKind; Property change.ID }`(`Property` は選択肢のときだけ)
  - `func ParseList(s string) List`("statuses"・"priorities"・"systems" 以外はプロパティの ID として選択肢)
  - `func (l List) Ref(v change.ID) string`(`"statuses/<v>"`・`"choices/<p>/<v>"`)、`func (l List) Field(v change.ID, name string) string`
  - `func (s Settings) Values(l List) ([]Value, bool)`(選択肢は単一・複数選択のプロパティだけ。ないなら false)
  - `func (s Settings) Label(l List) string`(「ステータス」「優先度」「関連システム」「「顧客」の選択肢」)
  - `func (s Settings) Single(l List) bool`(ステータス・優先度・単一選択の選択肢なら true)
  - `func (s Settings) Property(id change.ID) (Property, bool)`
  - `func PropertyDrafts(p Property) []change.Draft`、`func ValueDrafts(l List, v Value) []change.Draft`
  - `func SwapRanks(fa, ra, fb, rb string) []change.Draft`、`func NextRank(ranks ...string) string`

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/workspace/lists_test.go`:

```go
package workspace_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func TestListRefs(t *testing.T) {
	const p change.ID = "01PROP0000000000000000000A"
	const v change.ID = "01VAL00000000000000000000A"
	cases := []struct {
		in       string
		ref, fld string
	}{
		{"statuses", "statuses/" + string(v), "statuses/" + string(v) + "/name"},
		{"systems", "systems/" + string(v), "systems/" + string(v) + "/name"},
		{string(p), "choices/" + string(p) + "/" + string(v), "choices/" + string(p) + "/" + string(v) + "/name"},
	}
	for _, c := range cases {
		l := workspace.ParseList(c.in)
		if l.Ref(v) != c.ref || l.Field(v, "name") != c.fld {
			t.Errorf("%s: ref %q field %q", c.in, l.Ref(v), l.Field(v, "name"))
		}
	}
}

func TestLabelsAndSingle(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	single := workspace.Property{ID: "01PROP0000000000000000000A", Name: "顧客", Type: workspace.TypeSingle}
	multi := workspace.Property{ID: "01PROP0000000000000000000B", Name: "タグ", Type: workspace.TypeMulti}
	text := workspace.Property{ID: "01PROP0000000000000000000C", Name: "メモ", Type: workspace.TypeText}
	s.Properties = append(s.Properties, single, multi, text)
	cases := []struct {
		list   string
		label  string
		single bool
		ok     bool
	}{
		{"statuses", "ステータス", true, true},
		{"priorities", "優先度", true, true},
		{"systems", "関連システム", false, true},
		{string(single.ID), "「顧客」の選択肢", true, true},
		{string(multi.ID), "「タグ」の選択肢", false, true},
		{string(text.ID), "", false, false},
		{"01NOPE00000000000000000000", "", false, false},
	}
	for _, c := range cases {
		l := workspace.ParseList(c.list)
		_, ok := s.Values(l)
		if ok != c.ok || (ok && (s.Label(l) != c.label || s.Single(l) != c.single)) {
			t.Errorf("%s: ok %v label %q single %v", c.list, ok, s.Label(l), s.Single(l))
		}
	}
}
```

`seqIDs()` は、`settings_test.go` に同じ役の関数があればそれを使う。なければ同じファイルに置く:

```go
// seqIDs は、01SEQ000…01、02… の内部 ID を順に返す。
func seqIDs() func() change.ID {
	n := 0
	return func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01SEQ%021d", n))
	}
}
```

`internal/domain/workspace/settings_log_test.go` に足す(既存のテストの準備の関数 `rows`(なければ Task 1 の `rows` と同じものをこのパッケージのテストに置く)を使う):

```go
// 作成後に足したプロパティと選択肢、値の追加と削除が、設定に出る(B2-3〜B2-5)。
func TestSettingsAddsAndDeletes(t *testing.T) {
	ids := seqIDs()
	w := workspace.Workspace{ID: "01WS0000000000000000000000", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(ids)
	create, err := workspace.CreateWithSettings(w, s)
	if err != nil {
		t.Fatal(err)
	}
	cust := workspace.Property{
		ID: "01PROP0000000000000000000A", Key: "customer", Name: "顧客", Type: workspace.TypeSingle,
		Kinds: []string{"Story", "Task"}, Rank: workspace.NextRank(s.Properties[len(s.Properties)-1].Rank),
	}
	l := workspace.List{Kind: workspace.ListChoices, Property: cust.ID}
	a := workspace.Value{ID: "01CHOICE00000000000000000A", Name: "A社", Color: "gray", Rank: "V"}
	b := workspace.Value{ID: "01CHOICE00000000000000000B", Name: "B社", Color: "slate", Rank: "W"}
	review := workspace.Value{ID: "01STATUS00000000000000000R", Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	drafts := []change.Draft{create}
	drafts = append(drafts, workspace.PropertyDrafts(cust)...)
	drafts = append(drafts, workspace.ValueDrafts(l, a)...)
	drafts = append(drafts, workspace.ValueDrafts(l, b)...)
	drafts = append(drafts, workspace.ValueDrafts(workspace.ParseList("statuses"), review)...)
	drafts = append(drafts,
		change.NewDelete(l.Ref(b.ID)),
		change.NewDelete("systems/"+string(s.Systems[0].ID)),
	)
	got, ok := workspace.SettingsFromLog(rows(drafts...))
	if !ok {
		t.Fatal("not created")
	}
	p, _ := got.Property(cust.ID)
	cust.Choices = []workspace.Value{a}
	if !reflect.DeepEqual(p, cust) {
		t.Errorf("property = %+v\nwant %+v", p, cust)
	}
	if last := got.Statuses[len(got.Statuses)-1]; last != review {
		t.Errorf("statuses = %+v", got.Statuses)
	}
	if len(got.Systems) != 5 || got.Systems[0].Name != "API" {
		t.Errorf("systems = %+v", got.Systems)
	}
	// プロパティを消すと、その選択肢ごと消える。
	got, _ = workspace.SettingsFromLog(rows(append(drafts, change.NewDelete("properties/"+string(cust.ID)))...))
	if _, ok := got.Property(cust.ID); ok {
		t.Error("deleted property is still there")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/`
Expected: FAIL(`workspace.ParseList` などが未定義)

- [ ] **Step 3: 実装する**

`settings.go` の `Property` に `Choices []Value // 選択肢(単一選択・複数選択だけ)。Rank の順` を足す。

`lists.go`:

```go
package workspace

import (
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// ListKind は、値の一覧の種類。ログの項目名の頭と同じ。
type ListKind string

const (
	ListStatuses   ListKind = "statuses"
	ListPriorities ListKind = "priorities"
	ListSystems    ListKind = "systems"
	ListChoices    ListKind = "choices" // 追加プロパティの選択肢
)

// List は、値の一覧の 1 つ。Property は選択肢のときだけ持つ。
type List struct {
	Kind     ListKind
	Property change.ID
}

// ParseList は、API のパスの {list} を一覧にする。statuses・priorities・systems 以外は、プロパティの ID。
func ParseList(s string) List {
	switch k := ListKind(s); k {
	case ListStatuses, ListPriorities, ListSystems:
		return List{Kind: k}
	}
	return List{Kind: ListChoices, Property: change.ID(s)}
}

// Ref は、値 v を指す項目名の頭(削除の行の f)。
func (l List) Ref(v change.ID) string {
	if l.Kind == ListChoices {
		return string(ListChoices) + "/" + string(l.Property) + "/" + string(v)
	}
	return string(l.Kind) + "/" + string(v)
}

// Field は、値 v の項目 name(name・color・category・rank)の項目名。
func (l List) Field(v change.ID, name string) string { return l.Ref(v) + "/" + name }

// Property は、id のプロパティ。
func (s Settings) Property(id change.ID) (Property, bool) {
	i := slices.IndexFunc(s.Properties, func(p Property) bool { return p.ID == id })
	if i < 0 {
		return Property{}, false
	}
	return s.Properties[i], true
}

// Values は、一覧 l の値。選択肢は、単一選択・複数選択のプロパティのときだけ。
func (s Settings) Values(l List) ([]Value, bool) {
	switch l.Kind {
	case ListStatuses:
		return s.Statuses, true
	case ListPriorities:
		return s.Priorities, true
	case ListSystems:
		return s.Systems, true
	}
	p, ok := s.Property(l.Property)
	if !ok || (p.Type != TypeSingle && p.Type != TypeMulti) {
		return nil, false
	}
	return p.Choices, true
}

// Label は、アクティビティの文の {一覧}(03 冒頭の表の下)。
func (s Settings) Label(l List) string {
	switch l.Kind {
	case ListStatuses:
		return "ステータス"
	case ListPriorities:
		return "優先度"
	case ListSystems:
		return "関連システム"
	}
	p, _ := s.Property(l.Property)
	return "「" + p.Name + "」の選択肢"
}

// Single は、1 つの値を持つ一覧か(ステータス・優先度・単一選択)。03「削除」の「単一の値」。
func (s Settings) Single(l List) bool {
	switch l.Kind {
	case ListStatuses, ListPriorities:
		return true
	case ListSystems:
		return false
	}
	p, _ := s.Property(l.Property)
	return p.Type == TypeSingle
}
```

`drafts.go`:

```go
package workspace

import (
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// PropertyDrafts は、作成後に足すプロパティの行(B2-5)。使う種類は 1 つずつ add。
func PropertyDrafts(p Property) []change.Draft {
	base := "properties/" + string(p.ID) + "/"
	ds := []change.Draft{
		change.NewSet(base+"key", nil, p.Key),
		change.NewSet(base+"name", nil, p.Name),
		change.NewSet(base+"type", nil, p.Type),
		change.NewSet(base+"builtin", nil, false),
		change.NewSet(base+"hidden", nil, false),
		change.NewSet(base+"rank", nil, p.Rank),
	}
	for _, k := range p.Kinds {
		ds = append(ds, change.NewAdd(base+"kinds", k))
	}
	return ds
}

// ValueDrafts は、一覧 l に足す値の行。分類はステータスだけ。
func ValueDrafts(l List, v Value) []change.Draft {
	ds := []change.Draft{
		change.NewSet(l.Field(v.ID, "name"), nil, v.Name),
		change.NewSet(l.Field(v.ID, "color"), nil, v.Color),
		change.NewSet(l.Field(v.ID, "rank"), nil, v.Rank),
	}
	if l.Kind == ListStatuses {
		ds = append(ds, change.NewSet(l.Field(v.ID, "category"), nil, v.Category))
	}
	return ds
}

// SwapRanks は、隣どうしの並びの入れ替え(B2-6)。fa の順位 ra と fb の順位 rb を取り替える。
func SwapRanks(fa, ra, fb, rb string) []change.Draft {
	return []change.Draft{change.NewSet(fa, ra, rb), change.NewSet(fb, rb, ra)}
}

// NextRank は、ranks のどれよりもあとに並ぶ値。ranks が空なら rank.First。
func NextRank(ranks ...string) string {
	last := ""
	for _, r := range ranks {
		last = max(last, r)
	}
	return rank.After(last)
}
```

`settings_log.go` を次のように直す:

- `SettingsFromLog` で、`change.Resolve` の前に、ログの `t == change.Delete` の行の `F` を集めた `deleted map[string]bool` を作り、`builder` に持たせる(`Resolve` は削除の行を読み飛ばすため、生のログから集める)。
- `builder` に `choices map[change.ID]map[change.ID]*Value` を足す。`set` で `parts[0] == "choices" && len(parts) == 4` のとき、`setValue(b.choice(parts[1], parts[2]), parts[3], raw)`。
- `settings()` で、名前の空のものを除くのに加えて、`deleted["properties/<ID>"]`・`deleted["<一覧>/<ID>"]`・`deleted["choices/<p>/<v>"]` のものを除く。選択肢は `byRank` で並べ、持ち主のプロパティの `Choices` に入れる(1 つもなければ nil のまま。B1 の照合テストの `reflect.DeepEqual` を壊さないため)。消えたプロパティの選択肢は捨てる。
- 関数が 60 行・認知的複雑度 15 を超えそうなら、`dropDeleted`・`attachChoices` に分ける。

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/domain/workspace/ ./internal/app/... ./internal/infra/gitstore/`
Expected: PASS(B1 の照合テストも通る)

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace
git commit -m "feat(workspace): 設定に選択肢と削除の行を足し、値の一覧の指し方を決める"
```

---

### Task 3: プロパティと値の検査と文言

**Files:**
- Create: `internal/domain/workspace/property_check.go`
- Create: `internal/domain/workspace/value_check.go`
- Test: `internal/domain/workspace/property_check_test.go`、`internal/domain/workspace/value_check_test.go`

**Interfaces:**
- Consumes: `Settings`・`Property`・`Value`・`refusal.New`
- Produces:
  - `var ReservedKeys []string`(34 個)、`var AddableTypes []PropertyType`(8 型)
  - `func NormalizeKinds(kinds []string) []string`(`Kinds` の並び、重複と知らない種類を除く)
  - `func CheckNewProperty(s Settings, name, key string) error`
  - `func CheckPropertyName(s Settings, id change.ID, name string) error`
  - `func CheckKinds(kinds []string) error`
  - `func CategoryName(c Category) string`
  - `func CheckValueName(vs []Value, id change.ID, name string) error`(`id` が空なら追加)
  - `func CheckStatusCategory(s Settings, id change.ID, to Category) error`
  - `func CheckDeleteStatus(s Settings, id change.ID) error`、`func CheckDeletePriority(s Settings, id change.ID) error`
  - `func CheckInitialStatus(s Settings, id change.ID) error`
  - `func (s Settings) DefaultCategory(id change.ID) (Category, bool)`(id を既定にしている分類)

- [ ] **Step 1: 失敗するテストを書く**

`property_check_test.go`(断りの比べ方は B1 の `check_test.go` の関数を使う。なければ `errors.As` で `*refusal.Refusal` の `Message` を比べる `msg(err) string` を置く):

```go
func TestCheckNewProperty(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	s.Properties = append(s.Properties, workspace.Property{ID: "01PROP0000000000000000000A", Key: "customer", Name: "顧客", Type: workspace.TypeSingle})
	cases := []struct{ name, key, want string }{
		{"", "x", "名前を入力してください"},
		{"顧客", "x", "同じ名前のプロパティがあります"},
		{"ステータス", "x", "同じ名前のプロパティがあります"}, // 組み込みとも比べる
		{"環境", "Env", "キーは英小文字で始まる英小文字・数字・_ にしてください"},
		{"環境", "1env", "キーは英小文字で始まる英小文字・数字・_ にしてください"},
		{"環境", "", "キーは英小文字で始まる英小文字・数字・_ にしてください"},
		{"環境", "customer", "このキーは使用できません(もうあるか、予約されています)"},
		{"環境", "systems", "このキーは使用できません(もうあるか、予約されています)"}, // 組み込みのキー
		{"環境", "blocked_by", "このキーは使用できません(もうあるか、予約されています)"}, // 予約語
		{"環境", "env_2", ""},
		{"顧客 ", "x", ""}, // 呼ぶ側が NormalizeName を通す前なので、ここでは別の名前
	}
	for _, c := range cases {
		if got := msg(workspace.CheckNewProperty(s, c.name, c.key)); got != c.want {
			t.Errorf("%q %q: %q, want %q", c.name, c.key, got, c.want)
		}
	}
	if len(workspace.ReservedKeys) != 34 {
		t.Errorf("reserved = %d", len(workspace.ReservedKeys))
	}
}

func TestCheckPropertyNameAndKinds(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	if got := msg(workspace.CheckPropertyName(s, "assignee", "")); got != "名前は空にできません" {
		t.Error(got)
	}
	if got := msg(workspace.CheckPropertyName(s, "assignee", "優先度")); got != "同じ名前のプロパティがあります" {
		t.Error(got)
	}
	if err := workspace.CheckPropertyName(s, "assignee", "担当者"); err != nil { // 自分と同じ名前はよい
		t.Error(err)
	}
	if got := msg(workspace.CheckKinds(nil)); got != "少なくとも 1 つの種類で使ってください" {
		t.Error(got)
	}
	if got := workspace.NormalizeKinds([]string{"Task", "Epic", "Task", "Nope"}); !slices.Equal(got, []string{"Epic", "Task"}) {
		t.Errorf("kinds = %v", got)
	}
}
```

`value_check_test.go`:

```go
func TestCheckValueName(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	todo := s.Statuses[0].ID
	cases := []struct {
		id         change.ID
		name, want string
	}{
		{"", "", "名前は空にできません"},
		{"", "todo", "同じ名前の値があります"}, // 大文字と小文字を区別しない
		{"", "Review", ""},
		{todo, "TODO", ""}, // 自分の大文字と小文字だけを変えるのはよい
		{todo, "done", "同じ名前の値があります"},
	}
	for _, c := range cases {
		if got := msg(workspace.CheckValueName(s.Statuses, c.id, c.name)); got != c.want {
			t.Errorf("%q: %q, want %q", c.name, got, c.want)
		}
	}
}

// 作ったばかりのワークスペースで考える。Todo は初期ステータスで未着手の既定、InProgress は進行中の既定、
// Pending はどちらでもない(03「分類ごとの既定のステータス」「初期ステータス」)。
func TestStatusRules(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	todo, doing, pending := s.Statuses[0].ID, s.Statuses[1].ID, s.Statuses[3].ID
	both := "「Todo」は初期ステータスで、未着手の既定のステータスでもあります。先に別のステータスを、初期ステータスと未着手の既定にしてください"
	cases := []struct{ name, got, want string }{
		{"delete todo", msg(workspace.CheckDeleteStatus(s, todo)), both},
		{"delete doing", msg(workspace.CheckDeleteStatus(s, doing)), "「InProgress」は進行中の既定のステータスです。先に別のステータスを既定にしてください"},
		{"delete pending", msg(workspace.CheckDeleteStatus(s, pending)), ""},
		{"todo to done", msg(workspace.CheckStatusCategory(s, todo, workspace.CategoryDone)), both},
		{"todo to doing", msg(workspace.CheckStatusCategory(s, todo, workspace.CategoryDoing)), "「Todo」は未着手の既定のステータスです。先に別のステータスを既定にしてください"},
		{"todo stays", msg(workspace.CheckStatusCategory(s, todo, workspace.CategoryTodo)), ""},
		{"pending to done", msg(workspace.CheckStatusCategory(s, pending, workspace.CategoryDone)), ""},
		{"initial done", msg(workspace.CheckInitialStatus(s, s.Statuses[2].ID)), "完了・中止のステータスは初期ステータスにできません"},
		{"initial pending", msg(workspace.CheckInitialStatus(s, pending)), ""},
	}
	// Pending を初期ステータスにすると、初期ステータスだけに当たる。
	s2 := s
	s2.InitialStatus = pending
	cases = append(cases,
		struct{ name, got, want string }{"initial pending to done", msg(workspace.CheckStatusCategory(s2, pending, workspace.CategoryDone)), "「Pending」は初期ステータスです。完了・中止にするには、先に別のステータスを初期ステータスにしてください"},
		struct{ name, got, want string }{"delete initial pending", msg(workspace.CheckDeleteStatus(s2, pending)), "初期ステータスは削除できません。先に別のステータスを初期ステータスにしてください"},
	)
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: %q, want %q", c.name, c.got, c.want)
		}
	}
	if got := msg(workspace.CheckDeletePriority(s, s.InitialPriority)); got != "初期優先度は削除できません。先に別の優先度を初期優先度にしてください" {
		t.Error(got)
	}
	if err := workspace.CheckDeletePriority(s, s.Priorities[0].ID); err != nil {
		t.Error(err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ -run 'TestCheck|TestStatusRules'`
Expected: FAIL(未定義)

- [ ] **Step 3: 実装する**

`property_check.go`:

```go
package workspace

import (
	"regexp"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// ReservedKeys は、検索の条件のキー(03「追加プロパティ」の 34 個)。
var ReservedKeys = []string{
	"status", "kind", "system", "assignee", "priority", "sprint", "release", "project", "parent", "epic",
	"workspace", "store", "is", "has", "watching", "sort", "points", "estimate", "start", "end",
	"category", "group", "mentions", "created", "updated", "blocks", "blocked_by", "causes", "caused_by",
	"duplicates", "duplicated_by", "relates_to", "clones", "cloned_by",
}

// AddableTypes は、追加プロパティに選べる 8 型(03「追加プロパティ」の表の順)。
var AddableTypes = []PropertyType{TypeText, TypeNumber, TypeDate, TypeSingle, TypeMulti, TypeUser, TypeCheck, TypeURL}

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// NormalizeKinds は、使う種類を Kinds の並びにし、重複と知らない種類を除く。
func NormalizeKinds(kinds []string) []string {
	var out []string
	for _, k := range Kinds {
		if slices.Contains(kinds, k) {
			out = append(out, k)
		}
	}
	return out
}

// propertyNameTaken は、id 以外に同じ名前のプロパティがあるか(組み込みと非表示を含む。完全一致)。
func propertyNameTaken(s Settings, id change.ID, name string) bool {
	return slices.ContainsFunc(s.Properties, func(p Property) bool { return p.ID != id && p.Name == name })
}

// CheckNewProperty は、プロパティの追加の検査(03「追加プロパティ」の表の順)。name は NormalizeName を通した形。
func CheckNewProperty(s Settings, name, key string) error {
	switch {
	case name == "":
		return refusal.New("名前を入力してください")
	case propertyNameTaken(s, "", name):
		return refusal.New("同じ名前のプロパティがあります")
	case !keyPattern.MatchString(key):
		return refusal.New("キーは英小文字で始まる英小文字・数字・_ にしてください")
	case slices.Contains(ReservedKeys, key) ||
		slices.ContainsFunc(s.Properties, func(p Property) bool { return p.Key == key }):
		return refusal.New("このキーは使用できません(もうあるか、予約されています)")
	}
	return nil
}

// CheckPropertyName は、プロパティの改名の検査(03「一覧」の名前)。
func CheckPropertyName(s Settings, id change.ID, name string) error {
	switch {
	case name == "":
		return refusal.New("名前は空にできません")
	case propertyNameTaken(s, id, name):
		return refusal.New("同じ名前のプロパティがあります")
	}
	return nil
}

// CheckKinds は、使う種類の変更の検査(03「使う種類」)。kinds は NormalizeKinds を通した形。
func CheckKinds(kinds []string) error {
	if len(kinds) == 0 {
		return refusal.New("少なくとも 1 つの種類で使ってください")
	}
	return nil
}
```

`value_check.go`:

```go
package workspace

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

var categoryNames = map[Category]string{
	CategoryTodo: "未着手", CategoryDoing: "進行中", CategoryDone: "完了", CategoryCanceled: "中止",
}

// CategoryName は、分類の画面の名前。
func CategoryName(c Category) string { return categoryNames[c] }

// CheckValueName は、値の名前の検査(03「名前」)。同じ一覧の中で、大文字と小文字を区別せずに比べる。
// id が空なら追加。name は NormalizeName を通した形。
func CheckValueName(vs []Value, id change.ID, name string) error {
	switch {
	case name == "":
		return refusal.New("名前は空にできません")
	case slices.ContainsFunc(vs, func(v Value) bool { return v.ID != id && strings.EqualFold(v.Name, name) }):
		return refusal.New("同じ名前の値があります")
	}
	return nil
}

// DefaultCategory は、id を既定のステータスにしている分類。
func (s Settings) DefaultCategory(id change.ID) (Category, bool) {
	for _, c := range Categories {
		if s.CategoryDefaults[c] == id {
			return c, true
		}
	}
	return "", false
}

func statusName(s Settings, id change.ID) string {
	for _, v := range s.Statuses {
		if v.ID == id {
			return v.Name
		}
	}
	return ""
}

func bothText(name string, c Category) error {
	return refusal.New(fmt.Sprintf("「%s」は初期ステータスで、%sの既定のステータスでもあります。先に別のステータスを、初期ステータスと%sの既定にしてください",
		name, CategoryName(c), CategoryName(c)))
}

func defaultText(name string, c Category) error {
	return refusal.New(fmt.Sprintf("「%s」は%sの既定のステータスです。先に別のステータスを既定にしてください", name, CategoryName(c)))
}

// CheckStatusCategory は、ステータスの分類の変更の検査(03「分類ごとの既定のステータス」)。
func CheckStatusCategory(s Settings, id change.ID, to Category) error {
	i := slices.IndexFunc(s.Statuses, func(v Value) bool { return v.ID == id })
	if i < 0 || s.Statuses[i].Category == to {
		return nil
	}
	name := s.Statuses[i].Name
	def, isDefault := s.DefaultCategory(id)
	initial := id == s.InitialStatus && (to == CategoryDone || to == CategoryCanceled)
	switch {
	case isDefault && initial:
		return bothText(name, def)
	case isDefault:
		return defaultText(name, def)
	case initial:
		return refusal.New(fmt.Sprintf("「%s」は初期ステータスです。完了・中止にするには、先に別のステータスを初期ステータスにしてください", name))
	}
	return nil
}

// CheckDeleteStatus は、ステータスの削除の検査(03「初期ステータス」「分類ごとの既定のステータス」)。
func CheckDeleteStatus(s Settings, id change.ID) error {
	def, isDefault := s.DefaultCategory(id)
	initial := id == s.InitialStatus
	switch {
	case isDefault && initial:
		return bothText(statusName(s, id), def)
	case initial:
		return refusal.New("初期ステータスは削除できません。先に別のステータスを初期ステータスにしてください")
	case isDefault:
		return defaultText(statusName(s, id), def)
	}
	return nil
}

// CheckDeletePriority は、優先度の削除の検査(03「初期優先度」)。
func CheckDeletePriority(s Settings, id change.ID) error {
	if id == s.InitialPriority {
		return refusal.New("初期優先度は削除できません。先に別の優先度を初期優先度にしてください")
	}
	return nil
}

// CheckInitialStatus は、初期ステータスにできるか(03「初期ステータス」。未着手か進行中だけ)。
func CheckInitialStatus(s Settings, id change.ID) error {
	for _, v := range s.Statuses {
		if v.ID == id && (v.Category == CategoryDone || v.Category == CategoryCanceled) {
			return refusal.New("完了・中止のステータスは初期ステータスにできません")
		}
	}
	return nil
}
```

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/domain/workspace/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace
git commit -m "feat(workspace): プロパティと値の検査と断りの文言を足す"
```

---

### Task 4: プロパティの操作

**Files:**
- Create: `internal/app/workspaces/edit.go`、`internal/app/workspaces/items.go`
- Create: `internal/app/workspaces/property_add.go`、`property_edit.go`、`property_delete.go`
- Test: `internal/app/workspaces/property_test.go`
- Modify: `internal/app/workspaces/summary_test.go`(アクティビティの文)

**Interfaces:**
- Consumes: Task 2・3 のすべて、`item.FieldKey`(Task 1)、`port.Tx`
- Produces:
  - `type NewProperty struct{ Name, Key string; Type workspace.PropertyType; Kinds []string }`
  - `func (s Service) AddProperty(ctx, store port.StoreID, ws workspace.ID, in NewProperty) (workspace.Property, error)`
  - `func (s Service) RenameProperty(ctx, store, ws, id change.ID, name string) error`
  - `func (s Service) SetPropertyKinds(ctx, store, ws, id change.ID, kinds []string) error`
  - `func (s Service) SetPropertyHidden(ctx, store, ws, id change.ID, hidden bool) error`
  - `func (s Service) MoveProperty(ctx, store, ws, id change.ID, up bool) error`
  - `func (s Service) DeleteProperty(ctx, store, ws, id change.ID) error`
  - (パッケージ内)`func (s Service) edit(ctx, store, ws, summary func(workspace.Settings) (string, error), rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error)) error`
  - (パッケージ内)`func workspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error)`

各操作の決め方(03「プロパティ」):

| 操作 | アクティビティの文 | 断り・何もしない |
|---|---|---|
| AddProperty | `プロパティ「X」を追加` | 型が `AddableTypes` にない → `port.ErrNotFound`。使う種類が空 → すべて。検査は `CheckNewProperty`。並びは末尾(`NextRank`) |
| RenameProperty | `プロパティ「A」を「B」に改名` | `CheckPropertyName`。同じ名前なら行なし |
| SetPropertyKinds | `プロパティ「X」を使う種類を変更` | ステータス → `ErrNotFound`。`CheckKinds`。差分を `add`・`remove` で書き、差分がなければ行なし |
| SetPropertyHidden | `プロパティ「X」を非表示に設定`(表示は `…を表示に設定`) | 追加プロパティとステータス → `ErrNotFound`。同じ値なら行なし |
| MoveProperty | `プロパティの順番を変更` | 端なら行なし。隣と `SwapRanks` |
| DeleteProperty | `プロパティ「X」を削除` | 組み込み → `ErrNotFound`。値を持つアイテム: 複数選択は値ごとに `remove`、それ以外は `set` null。最後にワークスペースへ `NewDelete("properties/<ID>")` |

ない ID(プロパティ・ワークスペース)は、どの操作も `port.ErrNotFound`。

- [ ] **Step 1: 失敗するテストを書く**

`property_test.go`。プロジェクトとアイテムは `Stores.Write` の中で `project.Create`・`item.Create` を直接書いて用意する(`app/projects`・`app/items` は import しない):

```go
package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// settingsOf は、保存されている設定を返す。
func settingsOf(t *testing.T, s workspaces.Service, ws workspace.ID) workspace.Settings {
	t.Helper()
	var st workspace.Settings
	if err := s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		st, err = r.Workspaces().Settings(ws)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return st
}

// putItem は、ws にプロジェクトとアイテムを 1 つ作り、drafts を足す。アイテムの ID を返す。
func putItem(t *testing.T, s workspaces.Service, ws workspace.ID, drafts ...change.Draft) item.ID {
	t.Helper()
	p := project.Project{ID: s.IDs.New(), Place: project.Place{Workspace: ws, Number: 1}, Rank: "V", Title: "p", Status: project.Todo}
	it := item.Item{ID: s.IDs.New(), Place: item.Place{Workspace: ws, Project: p.ID, Number: 1}, Kind: item.Epic, Rank: "V", Title: "x"}
	err := s.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("taro"), Summary: "準備"}, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(p.ID, project.Create(p)),
			tx.WriteItems().Append(it.ID, append([]change.Draft{item.Create(it)}, drafts...)...),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func itemOf(t *testing.T, s workspaces.Service, id item.ID) item.Item {
	t.Helper()
	var it item.Item
	if err := s.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return it
}

func TestAddProperty(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	p, err := s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: " 顧客 ", Key: "customer", Type: workspace.TypeSingle})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := settingsOf(t, s, w.ID).Property(p.ID)
	if !ok || got.Name != "顧客" || got.Builtin || len(got.Kinds) != 5 || got.Rank <= settingsOf(t, s, w.ID).Properties[7].Rank {
		t.Errorf("property = %+v", got)
	}
	_, err = s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "顧客", Key: "c2", Type: workspace.TypeText})
	rejected(t, err, "同じ名前のプロパティがあります")
	_, err = s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "x", Key: "x", Type: workspace.TypeDuration})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("duration: %v", err)
	}
}

func TestEditProperty(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	ctx := t.Context()
	rejected(t, s.RenameProperty(ctx, store, w.ID, "assignee", "優先度"), "同じ名前のプロパティがあります")
	if err := s.RenameProperty(ctx, store, w.ID, "assignee", "担当"); err != nil {
		t.Fatal(err)
	}
	rejected(t, s.SetPropertyKinds(ctx, store, w.ID, "assignee", nil), "少なくとも 1 つの種類で使ってください")
	if err := s.SetPropertyKinds(ctx, store, w.ID, "assignee", []string{"Task", "Bug"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPropertyHidden(ctx, store, w.ID, "points", true); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveProperty(ctx, store, w.ID, "assignee", true); err != nil { // ステータスの上へ
		t.Fatal(err)
	}
	st := settingsOf(t, s, w.ID)
	a, _ := st.Property("assignee")
	pts, _ := st.Property("points")
	if a.Name != "担当" || len(a.Kinds) != 2 || !pts.Hidden || st.Properties[0].ID != "assignee" {
		t.Errorf("settings = %+v", st.Properties)
	}
	for _, err := range []error{
		s.SetPropertyKinds(ctx, store, w.ID, "status", []string{"Task"}),
		s.SetPropertyHidden(ctx, store, w.ID, "status", true),
		s.DeleteProperty(ctx, store, w.ID, "assignee"),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("fixed: %v", err)
		}
	}
	// 先頭で上へは、何も保存しない。
	_, before := stored(t, s, w.ID)
	if err := s.MoveProperty(ctx, store, w.ID, "assignee", true); err != nil {
		t.Fatal(err)
	}
	if _, after := stored(t, s, w.ID); len(after) != len(before) {
		t.Error("moved at the top")
	}
}

// 追加プロパティを消すと、アイテムの値も消える(03「削除」)。
func TestDeleteProperty(t *testing.T) {
	s := setup(t)
	w, _ := s.Create(t.Context(), store, "Web", "HH")
	tags, _ := s.AddProperty(t.Context(), store, w.ID, workspaces.NewProperty{Name: "タグ", Key: "tags", Type: workspace.TypeMulti})
	a, _ := s.AddValue(t.Context(), store, w.ID, workspace.ParseList(string(tags.ID)), "A")
	id := putItem(t, s, w.ID, change.NewAdd(item.FieldKey(tags.ID), a.ID))
	if err := s.DeleteProperty(t.Context(), store, w.ID, tags.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := settingsOf(t, s, w.ID).Property(tags.ID); ok {
		t.Error("property remains")
	}
	if it := itemOf(t, s, id); it.Choices != nil {
		t.Errorf("choices = %v", it.Choices)
	}
}
```

`TestDeleteProperty` は Task 5 の `AddValue` を使う。Task 4 の時点では、このテストだけ `t.Skip("Task 5 の AddValue を待つ")` を最初の行に置き、Task 5 の Step 1 で外す。

`summary_test.go` の `TestActivitySummaries` に、上の表の 6 つの文が出ることを足す(`AddProperty` → `RenameProperty` → `SetPropertyKinds` → `SetPropertyHidden`(非表示と表示)→ `MoveProperty` → `DeleteProperty` の順に呼び、`want` に文を足す)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/`
Expected: FAIL(未定義)

- [ ] **Step 3: 実装する**

`edit.go`:

```go
package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// edit は、設定の 1 回の変更(01「書き込みの単位」)。アクティビティの文は保存の前に決めるので、読んだ設定から
// summary で作る。行は、書く Tx の中で読み直した設定から rows で作る(検査もここで行う)。行がなければ何も
// 保存しない。rows はアイテムにも書いてよい(同じ保存に入る)。
func (s Service) edit(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(workspace.Settings) (string, error),
	rows func(tx port.Tx, st workspace.Settings) ([]change.Draft, error),
) error {
	actor, err := s.me(ctx, store)
	if err != nil {
		return err
	}
	var st workspace.Settings
	if err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		st, err = r.Workspaces().Settings(ws)
		return err
	}); err != nil {
		return err
	}
	text, err := summary(st)
	if err != nil {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: actor, Summary: text}, func(tx port.Tx) error {
		st, err := tx.Workspaces().Settings(ws)
		if err != nil {
			return err
		}
		ds, err := rows(tx, st)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(ws, ds...)
	})
}
```

`items.go`:

```go
package workspaces

import (
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// workspaceItems は、ワークスペースのすべてのアイテム(プロジェクトの並び、番号の順)。
// アイテムの削除(A4)が入ったら、削除済みを含むかを呼ぶ側で決める(B2-11)。
func workspaceItems(r port.Reader, ws workspace.ID) ([]item.Item, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, its...)
	}
	return out, nil
}
```

`property_add.go`(他の操作も同じ形で書く):

```go
// NewProperty は、追加プロパティのダイアログの入力(03「追加プロパティ」)。
type NewProperty struct {
	Name  string
	Key   string
	Type  workspace.PropertyType
	Kinds []string // 空ならすべて
}

// AddProperty は、追加プロパティを作る。
func (s Service) AddProperty(ctx context.Context, store port.StoreID, ws workspace.ID, in NewProperty) (workspace.Property, error) {
	if !slices.Contains(workspace.AddableTypes, in.Type) {
		return workspace.Property{}, port.ErrNotFound
	}
	p := workspace.Property{ID: s.IDs.New(), Key: in.Key, Name: workspace.NormalizeName(in.Name), Type: in.Type,
		Kinds: workspace.NormalizeKinds(in.Kinds)}
	if len(p.Kinds) == 0 {
		p.Kinds = slices.Clone(workspace.Kinds)
	}
	err := s.edit(ctx, store, ws,
		func(workspace.Settings) (string, error) { return fmt.Sprintf("プロパティ「%s」を追加", oneLine(p.Name)), nil },
		func(_ port.Tx, st workspace.Settings) ([]change.Draft, error) {
			if err := workspace.CheckNewProperty(st, p.Name, p.Key); err != nil {
				return nil, err
			}
			ranks := make([]string, 0, len(st.Properties))
			for _, x := range st.Properties {
				ranks = append(ranks, x.Rank)
			}
			p.Rank = workspace.NextRank(ranks...)
			return workspace.PropertyDrafts(p), nil
		})
	if err != nil {
		return workspace.Property{}, err
	}
	return p, nil
}
```

`property_edit.go` の 4 つの操作は、`summary` で `st.Property(id)` を引き(なければ `port.ErrNotFound`)、上の表の文を返す。`rows` の中で表の検査をし、行を返す。項目名は `"properties/" + id + "/name"` など。使う種類の差分は、`NormalizeKinds` した新しい組と、いまの `Kinds` を比べて、足すものを `change.NewAdd(base+"kinds", k)`、外すものを `change.NewRemove(base+"kinds", k)` にする。並べ替えは、`st.Properties` の中の位置 `i` と、上なら `i-1`・下なら `i+1` の `rank` を `workspace.SwapRanks` で入れ替える。

`property_delete.go`:

```go
// DeleteProperty は、追加プロパティを消す(03「プロパティ」の「削除」)。値の入っているアイテムから値を外す。
func (s Service) DeleteProperty(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.edit(ctx, store, ws,
		func(st workspace.Settings) (string, error) {
			p, ok := st.Property(id)
			if !ok || p.Builtin {
				return "", port.ErrNotFound
			}
			return fmt.Sprintf("プロパティ「%s」を削除", oneLine(p.Name)), nil
		},
		func(tx port.Tx, st workspace.Settings) ([]change.Draft, error) {
			p, ok := st.Property(id)
			if !ok || p.Builtin {
				return nil, port.ErrNotFound
			}
			its, err := workspaceItems(tx, ws)
			if err != nil {
				return nil, err
			}
			for _, it := range its {
				if ds := clearChoice(p, it.Choices[id]); len(ds) > 0 {
					if err := tx.WriteItems().Append(it.ID, ds...); err != nil {
						return nil, err
					}
				}
			}
			return []change.Draft{change.NewDelete("properties/" + string(id))}, nil
		})
}

// clearChoice は、アイテムからプロパティ p の値 vs を外す行。複数選択は値ごとに remove、ほかは set null。
func clearChoice(p workspace.Property, vs []change.ID) []change.Draft {
	f := item.FieldKey(p.ID)
	if len(vs) == 0 {
		return nil
	}
	if p.Type != workspace.TypeMulti {
		return []change.Draft{change.NewSet(f, vs[0], nil)}
	}
	ds := make([]change.Draft, 0, len(vs))
	for _, v := range vs {
		ds = append(ds, change.NewRemove(f, v))
	}
	return ds
}
```

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/app/workspaces/`
Expected: PASS(`TestDeleteProperty` は Skip)

- [ ] **Step 5: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat(app): プロパティの追加・改名・使う種類・非表示・並べ替え・削除を足す"
```

---

### Task 5: 値の操作

**Files:**
- Create: `internal/app/workspaces/value_add.go`、`value_edit.go`、`value_marks.go`、`value_delete.go`
- Test: `internal/app/workspaces/value_test.go`
- Modify: `internal/app/workspaces/property_test.go`(`TestDeleteProperty` の Skip を外す)、`summary_test.go`

**Interfaces:**
- Consumes: Task 2〜4
- Produces:
  - `func (s Service) AddValue(ctx, store, ws, l workspace.List, name string) (workspace.Value, error)`
  - `func (s Service) RenameValue(ctx, store, ws, l workspace.List, id change.ID, name string) error`
  - `func (s Service) SetValueColor(ctx, store, ws, l workspace.List, id change.ID, color workspace.Color) error`
  - `func (s Service) SetStatusCategory(ctx, store, ws, id change.ID, c workspace.Category) error`
  - `func (s Service) MoveValue(ctx, store, ws, l workspace.List, id change.ID, up bool) error`
  - `func (s Service) SetInitialStatus(ctx, store, ws, id change.ID) error`、`SetInitialPriority(…)`
  - `func (s Service) SetCategoryDefault(ctx, store, ws, c workspace.Category, id change.ID) error`
  - `func (s Service) DeleteValue(ctx, store, ws, l workspace.List, id, replacement change.ID) error`(`replacement` が空: ステータス・優先度は初期値、単一選択は「なし」)

| 操作 | アクティビティの文({一覧} は `st.Label(l)`) | 断り・何もしない |
|---|---|---|
| AddValue | `{一覧}に「X」を追加` | 一覧がない → `ErrNotFound`。`CheckValueName(vs, "", name)`。色は `Colors[len(vs)%18]`、分類は未着手、並びは末尾 |
| RenameValue | `{一覧}「A」を「B」に改名` | `CheckValueName(vs, id, name)`。同じ名前(完全一致)なら行なし |
| SetValueColor | `{一覧}「X」の色を変更` | 18 色にない → `ErrNotFound`。同じ色なら行なし |
| SetStatusCategory | `ステータス「X」の分類を変更` | 知らない分類 → `ErrNotFound`。`CheckStatusCategory`。同じなら行なし |
| MoveValue | `{一覧}の順番を変更` | 端なら行なし |
| SetInitialStatus | `ステータス「X」を初期ステータスに設定` | `CheckInitialStatus`。いまと同じなら行なし |
| SetInitialPriority | `優先度「X」を初期優先度に設定` | いまと同じなら行なし |
| SetCategoryDefault | `分類「{分類}」の既定を「X」に変更` | X の分類が違う → `ErrNotFound`。同じなら行なし |
| DeleteValue | `{一覧}「X」を削除` | ステータスは `CheckDeleteStatus`、優先度は `CheckDeletePriority`。代わりの値が一覧にないか、消す値と同じ → `ErrNotFound`。複数の一覧で代わりの値を渡したら → `ErrNotFound` |

削除で書く行(B2-8): 使っているすべてのアイテムに、ステータスは `set status`(代わり)、優先度は `set priority`、関連システムは `remove systems`、単一選択は `set fields/<p>`(代わりか null)、複数選択は `remove fields/<p>`。ワークスペースに `NewDelete(l.Ref(id))`。遷移の制約と自動化は呼ばない(B2-10)。

- [ ] **Step 1: 失敗するテストを書く**

`value_test.go`:

```go
func TestValueEdits(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	statuses := workspace.ParseList("statuses")
	rejected(t, func() error { _, err := s.AddValue(ctx, store, w.ID, statuses, "todo"); return err }(), "同じ名前の値があります")
	review, err := s.AddValue(ctx, store, w.ID, statuses, " Review ")
	if err != nil {
		t.Fatal(err)
	}
	// いまの値は 5 つなので、色は Colors[5](桃)。分類は未着手。
	if review.Name != "Review" || review.Color != workspace.Colors[5] || review.Category != workspace.CategoryTodo {
		t.Errorf("review = %+v", review)
	}
	if err := s.RenameValue(ctx, store, w.ID, statuses, review.ID, "Reviewing"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetValueColor(ctx, store, w.ID, statuses, review.ID, "purple"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatusCategory(ctx, store, w.ID, review.ID, workspace.CategoryDoing); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveValue(ctx, store, w.ID, statuses, review.ID, true); err != nil {
		t.Fatal(err)
	}
	st := settingsOf(t, s, w.ID)
	got := st.Statuses[4]
	if got.ID != review.ID || got.Name != "Reviewing" || got.Color != "purple" || got.Category != workspace.CategoryDoing {
		t.Errorf("statuses = %+v", st.Statuses)
	}
	if err := s.SetValueColor(ctx, store, w.ID, statuses, review.ID, "black"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("black: %v", err)
	}
}

func TestInitialAndDefaults(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	todo, doing, done, pending := st.Statuses[0].ID, st.Statuses[1].ID, st.Statuses[2].ID, st.Statuses[3].ID
	rejected(t, s.SetInitialStatus(ctx, store, w.ID, done), "完了・中止のステータスは初期ステータスにできません")
	if err := s.SetInitialStatus(ctx, store, w.ID, doing); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCategoryDefault(ctx, store, w.ID, workspace.CategoryTodo, pending); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCategoryDefault(ctx, store, w.ID, workspace.CategoryTodo, done); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("wrong category: %v", err)
	}
	if err := s.SetInitialPriority(ctx, store, w.ID, st.Priorities[0].ID); err != nil {
		t.Fatal(err)
	}
	st = settingsOf(t, s, w.ID)
	if st.InitialStatus != doing || st.CategoryDefaults[workspace.CategoryTodo] != pending || st.InitialPriority != st.Priorities[0].ID {
		t.Errorf("settings = %+v", st)
	}
	// Todo は初期でも既定でもなくなったので消せる。
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("statuses"), todo, ""); err != nil {
		t.Fatal(err)
	}
}

// 値を消すと、使っているアイテムが置き換わる・値が外れる(03「削除」、B2-8)。
func TestDeleteValueReplaces(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	pending, cancel := st.Statuses[3].ID, st.Statuses[4].ID
	high, low := st.Priorities[1].ID, st.Priorities[3].ID
	web, api := st.Systems[0].ID, st.Systems[1].ID
	env, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeSingle})
	envs := workspace.ParseList(string(env.ID))
	prod, _ := s.AddValue(ctx, store, w.ID, envs, "本番")
	id := putItem(t, s, w.ID,
		change.NewSet("status", nil, pending), change.NewSet("priority", nil, high),
		change.NewAdd("systems", web), change.NewAdd("systems", api),
		change.NewSet(item.FieldKey(env.ID), nil, prod.ID))
	steps := []struct {
		list        string
		value, repl change.ID
	}{
		{"statuses", pending, cancel},
		{"priorities", high, ""}, // 初期優先度(Medium)に置き換える
		{"systems", web, ""},
		{string(env.ID), prod.ID, ""}, // 「なし」
	}
	for _, c := range steps {
		if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList(c.list), c.value, c.repl); err != nil {
			t.Fatalf("%s: %v", c.list, err)
		}
	}
	it := itemOf(t, s, id)
	if it.Status != cancel || it.Priority != st.InitialPriority || !slices.Equal(it.Systems, []change.ID{api}) || it.Choices != nil {
		t.Errorf("item = %+v", it)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.ParseList("priorities"), low, low); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("self replacement: %v", err)
	}
	st = settingsOf(t, s, w.ID)
	if len(st.Statuses) != 4 || len(st.Priorities) != 4 || len(st.Systems) != 5 {
		t.Errorf("settings = %+v", st)
	}
}
```

`summary_test.go` に、上の表の文がすべて出ることを足す(例: `ステータスに「Review」を追加`、`「環境」の選択肢「本番」を削除`、`分類「未着手」の既定を「Pending」に変更`)。

`property_test.go` の `TestDeleteProperty` の `t.Skip` を外す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/`
Expected: FAIL(未定義)

- [ ] **Step 3: 実装する**

Task 4 の `edit` を使い、上の表のとおり書く。値と一覧の引き方は共通の関数にまとめる:

```go
// valueOf は、一覧 l の値 id。一覧か値がなければ ErrNotFound。
func valueOf(st workspace.Settings, l workspace.List, id change.ID) ([]workspace.Value, workspace.Value, error) {
	vs, ok := st.Values(l)
	if !ok {
		return nil, workspace.Value{}, port.ErrNotFound
	}
	i := slices.IndexFunc(vs, func(v workspace.Value) bool { return v.ID == id })
	if i < 0 {
		return nil, workspace.Value{}, port.ErrNotFound
	}
	return vs, vs[i], nil
}
```

`value_delete.go` のアイテムの行は、一覧の種類ごとに分ける:

```go
// replaceRows は、アイテム it から値 id を外す・置き換える行(B2-8)。
func replaceRows(st workspace.Settings, l workspace.List, it item.Item, id, repl change.ID) []change.Draft {
	switch l.Kind {
	case workspace.ListStatuses:
		if it.Status == id {
			return []change.Draft{change.NewSet("status", id, repl)}
		}
	case workspace.ListPriorities:
		if it.Priority == id {
			return []change.Draft{change.NewSet("priority", id, repl)}
		}
	case workspace.ListSystems:
		if slices.Contains(it.Systems, id) {
			return []change.Draft{change.NewRemove("systems", id)}
		}
	case workspace.ListChoices:
		return replaceChoice(st, l.Property, it.Choices[l.Property], id, repl)
	}
	return nil
}

func replaceChoice(st workspace.Settings, p change.ID, vs []change.ID, id, repl change.ID) []change.Draft {
	if !slices.Contains(vs, id) {
		return nil
	}
	f := item.FieldKey(p)
	if !st.Single(workspace.List{Kind: workspace.ListChoices, Property: p}) {
		return []change.Draft{change.NewRemove(f, id)}
	}
	if repl == "" {
		return []change.Draft{change.NewSet(f, id, nil)}
	}
	return []change.Draft{change.NewSet(f, id, repl)}
}
```

代わりの値の決め方: `repl == ""` なら、ステータスは `st.InitialStatus`、優先度は `st.InitialPriority`、単一選択は空のまま(null)。複数の一覧(`!st.Single(l)`)で `repl != ""` なら `ErrNotFound`。単一の一覧で `repl != ""` なら、`repl != id` で同じ一覧にあること。検査は `rows` の中で、アイテムを書く前に行う。

- [ ] **Step 4: 通るのを確かめる**

Run: `go test ./internal/app/workspaces/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat(app): 値の追加・改名・色・分類・並べ替え・初期値・分類の既定・削除を足す"
```

---

### Task 6: 使っているアイテムの件数

**Files:**
- Create: `internal/app/workspaces/usage.go`
- Test: `internal/app/workspaces/usage_test.go`

**Interfaces:**
- Consumes: `workspaceItems`(Task 4)
- Produces:
  - `type Usage struct{ Items int; FirstNumbers []int; Values map[change.ID]int; Properties map[change.ID]int }`
  - `func (s Service) Usage(ctx, store port.StoreID, ws workspace.ID) (Usage, error)`

`Items` は接頭辞の確認の N、`FirstNumbers` は番号の小さい方から 3 つ(03「接頭辞」の ID の例)、`Values` は値の内部 ID ごとの件数(ステータス・優先度・関連システム・選択肢)、`Properties` は追加プロパティごとの値の入っている件数(03「プロパティ」の削除の N)。ワークスペースがなければ `port.ErrNotFound`。

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestUsage(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	env, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "環境", Key: "env", Type: workspace.TypeSingle})
	prod, _ := s.AddValue(ctx, store, w.ID, workspace.ParseList(string(env.ID)), "本番")
	putItem(t, s, w.ID, change.NewSet("status", nil, st.Statuses[0].ID), change.NewAdd("systems", st.Systems[0].ID),
		change.NewSet(item.FieldKey(env.ID), nil, prod.ID))
	u, err := s.Usage(ctx, store, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Items != 1 || !slices.Equal(u.FirstNumbers, []int{1}) || u.Values[st.Statuses[0].ID] != 1 ||
		u.Values[st.Systems[0].ID] != 1 || u.Values[prod.ID] != 1 || u.Properties[env.ID] != 1 || u.Values[st.Statuses[1].ID] != 0 {
		t.Errorf("usage = %+v", u)
	}
	if _, err := s.Usage(ctx, store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}
```

アイテムが 4 つ以上あるときに 3 つだけ返すことも、`putItem` を番号を変えて呼べる形に直して確かめる(`putItem` の番号を引数にするか、同じプロジェクトに 4 つ作る小さな関数を足す)。

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/workspaces/ -run TestUsage`、Expected: FAIL

- [ ] **Step 3: 実装する**

`Stores.Read` の中で、`r.Workspaces().Get(ws)`(なければ `ErrNotFound`)、`workspaceItems` を読み、件数を数える。`FirstNumbers` は番号を集めて `slices.Sort` し、先頭の 3 つ。マップは空でも nil にしない。

- [ ] **Step 4: 通るのを確かめる** — Run: `go test ./internal/app/workspaces/`、Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/workspaces
git commit -m "feat(app): ワークスペースのアイテムの件数と、値ごとの件数を返す"
```

---

### Task 7: ワークスペースの並び

**Files:**
- Create: `internal/domain/workspace/order.go`、Test: `order_test.go`
- Create: `internal/app/workspaces/order.go`、Test: `order_test.go`
- Modify: `internal/app/workspaces/create.go`(作成で `rank` を書く)、`list.go`(並び)

**Interfaces:**
- Produces:
  - `func workspace.RankOf(log []change.Record) string`(ログの `rank` のいまの値。なければ空)
  - `func (s Service) MoveWorkspace(ctx, store port.StoreID, ws workspace.ID, up bool) error`
  - `Service.List` は `rank` の順、同じなら内部 ID の順で返す(B2-7)

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/workspaces/order_test.go`:

```go
func TestWorkspaceOrder(t *testing.T) {
	s := setup(t)
	rec := &recorder{Stores: s.Stores}
	s.Stores = rec
	ctx := t.Context()
	a, _ := s.Create(ctx, store, "Web", "HH")
	b, _ := s.Create(ctx, store, "API", "API")
	c, _ := s.Create(ctx, store, "Docs", "DOC")
	if err := s.MoveWorkspace(ctx, store, c.ID, true); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, w := range list {
		names = append(names, w.Name)
	}
	if !slices.Equal(names, []string{"Web", "Docs", "API"}) {
		t.Errorf("order = %v", names)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "Docs の並びを変更" {
		t.Errorf("summary = %q", last)
	}
	// 先頭で上へ・末尾で下へは、何も保存しない。
	n := len(rec.summaries)
	_ = s.MoveWorkspace(ctx, store, a.ID, true)
	_ = s.MoveWorkspace(ctx, store, b.ID, false)
	_, log := stored(t, s, a.ID)
	if len(log) != 2 { // 作成と rank だけ
		t.Errorf("log = %+v (%d summaries since)", log, len(rec.summaries)-n)
	}
}
```

`recorder` は `Write` のたびに文を控えるので、行のない Write も数える。上のテストは、ログの行の数で「保存しない」を確かめる。

`list.go` の一覧の戻り値の型はいまのまま(`[]workspace.Workspace` など)。テストの `w.Name` を、いまの型に合わせて直す。

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/workspaces/ -run TestWorkspaceOrder`、Expected: FAIL

- [ ] **Step 3: 実装する**

- `domain/workspace/order.go` の `RankOf` は `change.Resolve(log, opts).Values["rank"]` を文字列で読む。
- `create.go`: 書く Tx の中で、ほかのワークスペースのログから `RankOf` を集め、`workspace.NextRank(...)` を `change.NewSet("rank", nil, r)` として、作成の行と同じ `Append` に足す。
- `list.go`: 一覧を読んだあと、各ワークスペースのログ(`r.Changes().Of`)から `RankOf` を引き、`rank`、同じなら内部 ID の順に並べる。
- `order.go` の `MoveWorkspace`: 文は「{名前} の並びを変更」(04「並び順と並べ替え」)。名前は `oneLine`。並べた一覧で隣と `workspace.SwapRanks("rank", ra, "rank", rb)` の 2 行を、それぞれのワークスペースのログに 1 行ずつ書く(`SwapRanks` の 1 行目を自分、2 行目を隣へ)。端なら行なし。

- [ ] **Step 4: 通るのを確かめる** — Run: `go test ./internal/app/workspaces/ ./internal/domain/workspace/`、Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace internal/app/workspaces
git commit -m "feat(app): ワークスペースの並びを持ち、上へ・下へ移動を足す"
```

---

### Task 8: workspace.yaml の設定と、item.yaml の注釈

**Files:**
- Create: `internal/infra/gitstore/yaml_settings.go`
- Modify: `internal/infra/gitstore/yaml.go`、`internal/infra/gitstore/commit.go`
- Test: `internal/infra/gitstore/yaml_settings_test.go`

**Interfaces:**
- Consumes: `workspace.SettingsFromLog`・`workspace.RankOf`・`item.MultiChoice`
- Produces: 読む用の YAML だけ(口は変えない)

`workspace.yaml` の形(保存形式「ファイル構成」。キーの並びはこの順):

```yaml
id: 01JD0R5P7K…
name: Web
prefix: HH
rank: V
properties:
  - {id: status, key: status, name: ステータス, type: status, builtin: true, kinds: [Epic, Story, Task, Bug, Step], hidden: false, rank: V}
  - id: 01JD…F1
    key: customer
    name: 顧客
    type: single
    builtin: false
    kinds: [Story, Task]
    hidden: false
    rank: c
    choices:
      - {id: 01JD…F1V1, name: A社, color: gray, rank: V}
statuses:
  - {id: 01JD…S1, name: Todo, color: gray, category: todo, rank: V}
priorities:
  - {id: 01JD…P1, name: Highest, color: red, rank: V}
systems:
  - {id: 01JD…Y1, name: Web, color: blue, rank: V}
initial_status: 01JD…S1    # Todo
initial_priority: 01JD…P3  # Medium
category_defaults: {todo: 01JD…S1, doing: 01JD…S2, done: 01JD…S3, canceled: 01JD…S5}
```

選択肢のないプロパティは flow の 1 行、選択肢のあるプロパティはブロックで書く(`choices` が空なら出さない)。

`item.yaml` の注釈(B2-19):

```yaml
status: 01JD…S3       # InProgress
priority: 01JD…P2     # Medium
systems: [01JD…Y1, 01JD…Y2]   # Web, API
fields:
  01JD…F1: 01JD…F1V2  # 環境: 本番
  01JD…F2: [01JD…F2V1, 01JD…F2V3]  # タグ: A, C
```

名前の分からない値(消した値など)は注釈を付けない。注釈は `yaml.Node` の `LineComment` で付ける。

- [ ] **Step 1: 失敗するテストを書く**

`yaml_settings_test.go` で、gitstore の Store にワークスペース(`workspace.CreateWithSettings` に `DefaultSettings`)と、選択肢を持つプロパティ、値を持つアイテムを書き、ファイルを読んで次を確かめる:

- `workspace.yaml` に `statuses:`・`initial_status: <Todo の ID>  # Todo`・`category_defaults:` があり、プロパティの順が設定の順であること
- `item.yaml` の `status:` の行末に `# Todo`、`fields:` の行に `# 環境: 本番` があること
- 値の名前を変える Write(`RenameValue` と同じ行 `statuses/<ID>/name` をワークスペースのログに書くだけの Write)で、アイテムのログに行を書かなくても、そのアイテムの `item.yaml` の注釈が新しい名前になること(同じコミットで作り直す)

既存の gitstore のテストの準備の関数(Store を作って `Write` する関数)を使う。

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/ -run YAML`、Expected: FAIL

- [ ] **Step 3: 実装する**

- `renderYAML` の `change.KindWorkspace` の枝で、`workspace.SettingsFromLog(log)` と `workspace.RankOf(log)` から上の形を作る。`yaml_settings.go` に `workspaceToYAML(w, rank, settings) (*yaml.Node, error)` を置く。
- `renderYAML` の `change.KindItem` の枝は、アイテムのワークスペースの設定が要る。`renderYAML` に `settings func(workspace.ID) (workspace.Settings, bool)` を渡す形に変え、`commit.go` の `writeEntity` から、書いている Tx の読み取り(書いた行を含む)で設定を引く関数を渡す。
- `commit.go`: Tx がワークスペースのログに行を書いたら、そのワークスペースのすべてのアイテム(`Projects().List` → `Items().List`)の `item.yaml` も作り直して、同じコミットに入れる。アイテムのログは書かない。
- ファイルが 300 行、関数が 60 行を超えないように分ける。

- [ ] **Step 4: 通るのを確かめる** — Run: `go test ./internal/infra/gitstore/`、Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): workspace.yaml に設定を書き出し、item.yaml の値に名前の注釈を付ける"
```

---

### Task 9: プロパティ・値・件数・並びの API

**Files:**
- Create: `internal/adapter/httpapi/workspaces/properties.go`、`values.go`、`usage.go`
- Modify: `internal/adapter/httpapi/workspaces/workspaces.go`(`Register` から呼ぶ)、`dto.go`
- Test: `internal/adapter/httpapi/workspaces/settings_test.go`
- 生成物: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: Task 4〜7 の Service の操作
- Produces(パスはどれも `/api/stores/{store}/workspaces/{workspace}` の下。成功は本文なしの 204、作成は 201):

| OperationID | Method と Path | 本文・クエリ | 返すもの |
|---|---|---|---|
| `add-workspace-property` | POST `/properties` | `AddWorkspaceProperty{name, key, type(8 型の enum), kinds[]}` | `WorkspaceProperty` |
| `rename-workspace-property` | PUT `/properties/{property}/name` | `RenameWorkspaceProperty{name}` | — |
| `set-workspace-property-kinds` | PUT `/properties/{property}/kinds` | `SetWorkspacePropertyKinds{kinds[]}` | — |
| `set-workspace-property-hidden` | PUT `/properties/{property}/hidden` | `SetWorkspacePropertyHidden{hidden}` | — |
| `move-workspace-property` | POST `/properties/{property}/move` | `MoveWorkspaceItem{direction: up\|down}` | — |
| `delete-workspace-property` | DELETE `/properties/{property}` | — | — |
| `add-workspace-value` | POST `/lists/{list}/values` | `AddWorkspaceValue{name}` | `WorkspaceValue` |
| `rename-workspace-value` | PUT `/lists/{list}/values/{value}/name` | `RenameWorkspaceValue{name}` | — |
| `set-workspace-value-color` | PUT `/lists/{list}/values/{value}/color` | `SetWorkspaceValueColor{color(18 色の enum)}` | — |
| `move-workspace-value` | POST `/lists/{list}/values/{value}/move` | `MoveWorkspaceItem` | — |
| `delete-workspace-value` | DELETE `/lists/{list}/values/{value}` | クエリ `replacement`(省略可) | — |
| `set-workspace-status-category` | PUT `/statuses/{value}/category` | `SetWorkspaceStatusCategory{category(4 つの enum)}` | — |
| `set-workspace-initial-status` | PUT `/initial-status` | `SetWorkspaceValueRef{value}` | — |
| `set-workspace-initial-priority` | PUT `/initial-priority` | `SetWorkspaceValueRef{value}` | — |
| `set-workspace-category-default` | PUT `/category-defaults/{category}` | `SetWorkspaceValueRef{value}` | — |
| `get-workspace-usage` | GET `/usage` | — | `WorkspaceUsage{items, firstNumbers[], values{}, properties{}}` |
| `move-workspace` | POST `/move` | `MoveWorkspaceItem` | — |

`{list}` は `statuses`・`priorities`・`systems`、または選択肢のプロパティの ID(`workspace.ParseList`)。`WorkspaceProperty` に `choices: WorkspaceValue[]`(nullable でない、空なら `[]`)を足す。名前の本文は B1 と同じく `pattern:"^[^\\r\\n]*$"`。断りは `apierr.From` で 422、ないものは 404。

- [ ] **Step 1: 失敗するテストを書く**

`settings_test.go` は、B1 の `workspaces_test.go` と同じ準備(偽物の Stores と `humatest`)で、次を確かめる:

```go
// プロパティを足して選択肢を足し、詳細に出る。断りは 422 と文言。
func TestPropertyAndChoicesAPI(t *testing.T) {
	api, base := newAPI(t) // B1 のテストの準備の関数。名前が違えば合わせる
	ws := createWorkspace(t, api, base, "Web", "HH")
	res := api.Post(base+"/"+ws+"/properties", map[string]any{"name": "顧客", "key": "customer", "type": "single", "kinds": []string{}})
	if res.Code != http.StatusCreated {
		t.Fatalf("add property: %d %s", res.Code, res.Body)
	}
	var p struct{ ID string `json:"id"` }
	_ = json.Unmarshal(res.Body.Bytes(), &p)
	if res := api.Post(base+"/"+ws+"/lists/"+p.ID+"/values", map[string]any{"name": "A社"}); res.Code != http.StatusCreated {
		t.Fatalf("add choice: %d %s", res.Code, res.Body)
	}
	res = api.Post(base+"/"+ws+"/properties", map[string]any{"name": "顧客", "key": "c2", "type": "text", "kinds": []string{}})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "同じ名前のプロパティがあります") {
		t.Errorf("duplicate: %d %s", res.Code, res.Body)
	}
	res = api.Get(base + "/" + ws)
	if !strings.Contains(res.Body.String(), `"choices":[{"id"`) || !strings.Contains(res.Body.String(), `"name":"A社"`) {
		t.Errorf("detail: %s", res.Body)
	}
}

// ステータスを足して消すと、件数と詳細が変わる。初期ステータスは消せない。
func TestValuesAPI(t *testing.T) {
	api, base := newAPI(t)
	ws := createWorkspace(t, api, base, "Web", "HH")
	res := api.Post(base+"/"+ws+"/lists/statuses/values", map[string]any{"name": "Review"})
	var v struct{ ID string `json:"id"` }
	_ = json.Unmarshal(res.Body.Bytes(), &v)
	for _, r := range []*httptest.ResponseRecorder{
		api.Put(base+"/"+ws+"/lists/statuses/values/"+v.ID+"/color", map[string]any{"color": "purple"}),
		api.Put(base+"/"+ws+"/statuses/"+v.ID+"/category", map[string]any{"category": "doing"}),
		api.Post(base+"/"+ws+"/lists/statuses/values/"+v.ID+"/move", map[string]any{"direction": "up"}),
		api.Get(base + "/" + ws + "/usage"),
		api.Delete(base + "/" + ws + "/lists/statuses/values/" + v.ID),
	} {
		if r.Code >= 300 {
			t.Errorf("%d %s", r.Code, r.Body)
		}
	}
	detail := getDetail(t, api, base, ws) // 詳細を読んで、初期ステータスの ID を返す小さな関数を置く
	res = api.Delete(base + "/" + ws + "/lists/statuses/values/" + detail.Settings.InitialStatus)
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "初期ステータスで、未着手の既定のステータスでもあります") {
		t.Errorf("initial: %d %s", res.Code, res.Body)
	}
	if res := api.Put(base+"/"+ws+"/lists/statuses/values/"+v.ID+"/color", map[string]any{"color": "black"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("enum: %d", res.Code) // huma の enum の検査
	}
}
```

並べ替え(`/move`)と使う種類・非表示・初期優先度・分類の既定も、1 回ずつ 204 になることを確かめる。

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/adapter/httpapi/workspaces/`、Expected: FAIL(404)

- [ ] **Step 3: 実装する**

B1 の `workspaces.go` と同じ書き方で、`registerProperties(api, svc)`・`registerValues(api, svc)`・`registerUsage(api, svc)` を足し、`Register` から呼ぶ。入力の型は、パスの項目を埋め込まずに直接持つ(B1 のコメントのとおり)。`direction` は `enum:"up,down"`。`replacement` は `query:"replacement"`。`toSettings` で `Choices` も出す(`toValues` を使う)。

- [ ] **Step 4: 生成物を作り直し、通るのを確かめる**

Run: `make generate && (cd web && pnpm gen) && go test ./internal/adapter/... && make check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/adapter api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(adapter): プロパティ・値・件数・ワークスペースの並びの API を足す"
```

---

## 画面のタスクに共通すること

- ロジック(行の作り、文言、無効の理由、操作のあとの読み直しとトースト)は `core` に置き、Vitest で確かめる。Svelte は `core` の状態を出して操作を呼ぶだけにする(設計書 6 節。前回困ったこと d)。
- 断り(`Refused`)は、ダイアログの中ならダイアログに赤字で、ダイアログの外(インラインの編集、メニュー、ボタン)ならエラーのトーストで出し、設定を読み直して入力を元に戻す(03「名前」「接頭辞」「プロパティ」)。
- 成功のトーストは 03 冒頭の表のとおりだけ出す: 「接頭辞を変えました」「プロパティを追加しました」「プロパティを削除しました」「「X」を削除しました」(値の削除)。ほかの操作は出さない。
- 閲覧のみの Store では、設定の Store に書く入力欄とボタンをすべて無効にし、`readOnlyNote`(「閲覧のみの Store です」)を `title` に添える。サブタブ、「選択肢 N」、「使う種類」のメニューは開けるが、中の入力欄・チェック・追加・並べ替え・保存は無効で、「閉じる」だけ使える(03「範囲と画面の構成」)。
- 色は `--c-<色>-*` のトークン、分類は `--cat-*` のトークン(`StatusDot`)を使う。部品は `@hedgehug/ui` の `Button`・`IconButton`・`Input`・`Select`・`Checkbox`・`Dialog`・`Menu`・`Tabs`・`KindIcon`・`StatusDot`・`ReadOnlyBand` を使い、足りない見た目は `screens/workspace-settings` の中に作る(ui に新しい部品は足さない)。
- Svelte 200 行、TS 300 行を超えたら分ける。

### Task 10: ワークスペースのタブ、設定の口と状態

**Files:**
- Modify: `web/packages/core/src/features/router/route.ts`・`resolve.ts`(と各テスト)
- Modify: `web/packages/core/src/model/workspace.ts`
- Create: `web/packages/core/src/port/workspaces.ts`
- Create: `web/packages/core/src/features/workspaces/settings.ts`、`settings.test.ts`、`fixtures.test-util.ts`
- Create: `web/packages/api/src/workspaces.ts`、Test: `web/packages/api/src/ports.test.ts`(既存の口のテストに足す)
- Modify: `web/packages/core/src/index.ts`・`web/packages/api/src/index.ts`(公開)、`web/app/src/wire.ts`

**Interfaces:**
- Produces:

```ts
// route.ts
export type WorkspaceTab = "overview" | "settings";
// Route の workspace: { name: "workspace"; store: string; prefix: string; tab: WorkspaceTab }
// /s/{store}/w/{prefix} と /s/{store}/w/{prefix}/overview は概要、/settings は設定。形にするときは常にタブを付ける。
// resolve.ts の解決結果の workspace にも tab を持たせ、routeOfWorkspace(stores, storeId, workspaceId, tab = "overview")

// model/workspace.ts(既存の Workspace に足す)
export type Category = "todo" | "doing" | "done" | "canceled";
export type PropertyType = "status" | "user" | "priority" | "date" | "number" | "duration" | "systems"
  | "text" | "single" | "multi" | "check" | "url";
export interface WorkspaceValue { id: string; name: string; color: string; category?: Category }
export interface WorkspaceProperty {
  id: string; key: string; name: string; type: PropertyType; builtin: boolean;
  kinds: string[]; hidden: boolean; choices: WorkspaceValue[];
}
export interface WorkspaceSettings {
  properties: WorkspaceProperty[]; statuses: WorkspaceValue[]; priorities: WorkspaceValue[];
  systems: WorkspaceValue[]; initialStatus: string; initialPriority: string;
  categoryDefaults: Partial<Record<Category, string>>;
}
export interface WorkspaceDetail extends Workspace { oldPrefixes: string[]; settings: WorkspaceSettings }
export interface WorkspaceUsage {
  items: number; firstNumbers: number[]; values: Record<string, number>; properties: Record<string, number>;
}
// ValueList は、"statuses"・"priorities"・"systems" か、選択肢のプロパティの ID。
export type ValueList = string;
export interface NewProperty { name: string; key: string; type: PropertyType; kinds: string[] }

// port/workspaces.ts
export type Direction = "up" | "down";
export interface WorkspacesPort {
  get(storeId: string, workspaceId: string): Promise<WorkspaceDetail>;
  usage(storeId: string, workspaceId: string): Promise<WorkspaceUsage>;
  create(storeId: string, name: string, prefix: string): Promise<Workspace>;
  rename(storeId: string, workspaceId: string, name: string): Promise<void>;
  changePrefix(storeId: string, workspaceId: string, prefix: string): Promise<void>;
  move(storeId: string, workspaceId: string, direction: Direction): Promise<void>;
  addProperty(storeId: string, workspaceId: string, input: NewProperty): Promise<WorkspaceProperty>;
  renameProperty(storeId: string, workspaceId: string, propertyId: string, name: string): Promise<void>;
  setPropertyKinds(storeId: string, workspaceId: string, propertyId: string, kinds: string[]): Promise<void>;
  setPropertyHidden(storeId: string, workspaceId: string, propertyId: string, hidden: boolean): Promise<void>;
  moveProperty(storeId: string, workspaceId: string, propertyId: string, direction: Direction): Promise<void>;
  deleteProperty(storeId: string, workspaceId: string, propertyId: string): Promise<void>;
  addValue(storeId: string, workspaceId: string, list: ValueList, name: string): Promise<WorkspaceValue>;
  renameValue(storeId: string, workspaceId: string, list: ValueList, valueId: string, name: string): Promise<void>;
  setValueColor(storeId: string, workspaceId: string, list: ValueList, valueId: string, color: string): Promise<void>;
  setStatusCategory(storeId: string, workspaceId: string, valueId: string, category: Category): Promise<void>;
  moveValue(storeId: string, workspaceId: string, list: ValueList, valueId: string, direction: Direction): Promise<void>;
  // replacement が空なら、ステータス・優先度は初期値、単一選択は「なし」。
  deleteValue(storeId: string, workspaceId: string, list: ValueList, valueId: string, replacement: string): Promise<void>;
  setInitialStatus(storeId: string, workspaceId: string, valueId: string): Promise<void>;
  setInitialPriority(storeId: string, workspaceId: string, valueId: string): Promise<void>;
  setCategoryDefault(storeId: string, workspaceId: string, category: Category, valueId: string): Promise<void>;
}

// features/workspaces/settings.ts
export type SettingsTab = "general" | "properties" | "statuses" | "priorities" | "systems" | "workflow" | "templates" | "automation";
export const settingsTabs: { id: SettingsTab; label: string }[]; // 03 の 8 つ、この順
export interface SettingsState {
  storeId: string; readOnly: boolean; tab: SettingsTab;
  detail: WorkspaceDetail; usage: WorkspaceUsage;
}
export interface WorkspaceSettingsScreen {
  state: Readable<SettingsState | null>;
  open(storeId: string, workspaceId: string): Promise<void>; // 同じワークスペースならサブタブを保つ。別ならサブタブは一般
  selectTab(tab: SettingsTab): void;                        // 履歴に積まない(01)
  reload(): Promise<void>;
  // run は操作を 1 つ行い、読み直す。断りはエラーのトーストにして false を返す(読み直して入力を戻す)。
  // ok を渡したら、成功のときにトーストを出す。
  run(op: (port: WorkspacesPort, storeId: string, workspaceId: string) => Promise<unknown>, ok?: string): Promise<boolean>;
}
export function createWorkspaceSettings(deps: {
  port: WorkspacesPort; catalog: Catalog; toasts: Toasts;
}): WorkspaceSettingsScreen;
```

- [ ] **Step 1: 失敗するテストを書く**

`route.test.ts` に足す:

```ts
it.each([
  ["/s/work/w/HH", { name: "workspace", store: "work", prefix: "HH", tab: "overview" }],
  ["/s/work/w/HH/overview", { name: "workspace", store: "work", prefix: "HH", tab: "overview" }],
  ["/s/work/w/HH/settings", { name: "workspace", store: "work", prefix: "HH", tab: "settings" }],
])("parses the workspace tab of %s", (path, route) => {
  expect(parseRoute(path)).toEqual(route);
});

it("formats the workspace address with its tab", () => {
  expect(formatRoute({ name: "workspace", store: "work", prefix: "HH", tab: "settings" })).toBe("/s/work/w/HH/settings");
});
```

関数名(`parseRoute`・`formatRoute`)は、いまの `route.ts` の公開名に合わせる。知らないタブ(`/s/work/w/HH/kanban`)は、プロジェクトの知らないタブと同じ扱い(いまの `route.ts` のとおり)にする。

`settings.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeWorkspacesPort } from "./fixtures.test-util";
import { createWorkspaceSettings, settingsTabs } from "./settings";

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const port = fakeWorkspacesPort();
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const screen = createWorkspaceSettings({ port, catalog, toasts });
  return { screen, port, toasts };
}

describe("workspace settings", () => {
  it("has the eight sub tabs in the order of spec 03", () => {
    expect(settingsTabs.map((t) => t.label)).toEqual([
      "一般", "プロパティ", "ステータス", "優先度", "関連システム", "ワークフロー", "テンプレート", "自動化",
    ]);
  });

  it("opens on 一般 and keeps the sub tab while the workspace stays", async () => {
    const { screen } = await setup();
    await screen.open("S1", "W1");
    screen.selectTab("statuses");
    await screen.open("S1", "W1");
    expect(get(screen.state)?.tab).toBe("statuses");
    await screen.open("S1", "W2");
    expect(get(screen.state)?.tab).toBe("general");
  });

  it("shows a refusal as an error toast and reloads", async () => {
    const { screen, port, toasts } = await setup();
    await screen.open("S1", "W1");
    port.refuseNext("名前は空にできません");
    const ok = await screen.run((p, s, w) => p.rename(s, w, ""));
    expect(ok).toBe(false);
    expect(get(toasts.items).map((t) => [t.text, t.error])).toEqual([["名前は空にできません", true]]);
    expect(port.calls.filter((c) => c === "get")).toHaveLength(2); // 開いたときと、断りのあと
  });

  it("shows the success toast only when given", async () => {
    const { screen, toasts } = await setup();
    await screen.open("S1", "W1");
    await screen.run((p, s, w) => p.rename(s, w, "フロント"));
    await screen.run((p, s, w) => p.changePrefix(s, w, "WEB"), "接頭辞を変えました");
    expect(get(toasts.items).map((t) => t.text)).toEqual(["接頭辞を変えました"]);
  });

  it("is read only on a read-only store", async () => {
    const { screen } = await setup();
    await screen.open("S2", "W9"); // fakeCatalogPort の閲覧のみの Store。ID は fixtures に合わせる
    expect(get(screen.state)?.readOnly).toBe(true);
  });
});
```

`fixtures.test-util.ts` の `fakeWorkspacesPort()` は、`W1`・`W2`・`W9` の `WorkspaceDetail`(既定の 8 プロパティ・5 ステータス・5 優先度・6 関連システム)をメモリに持ち、呼ばれた操作の名前を `calls` に積み、`refuseNext(message)` の次の書き込みを `new Refused(message)` で断る。書き込みはメモリの設定を変える(画面のタスクのテストで使うため、`addValue`・`deleteValue`・`renameValue` ほか全部の操作を素直に実装する)。`toasts.items` の名前は、いまの `createToasts` の公開名に合わせる。

`ports.test.ts` に、`createWorkspacesPort()` の各関数が生成された関数を正しい引数で呼ぶことを、既存の口のテストと同じ方法で確かめるケースを足す。

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm --filter @hedgehug/api test`
Expected: FAIL

- [ ] **Step 3: 実装する**

- `route.ts`・`resolve.ts`: 上の Interfaces のとおり。`routeOfWorkspace` を呼んでいる箇所(サイドバーのリンクなど)は概要のまま。
- `settings.ts`: `open` は `port.get` と `port.usage` を並べて読み、`catalog` の Store の `readOnly` を持つ。`run` は `try { await op(...); if (ok) toasts.show(ok); } catch (e) { if (!(e instanceof Refused)) throw e; toasts.show(e.message, { error: true }); } finally { await reload(); }` の形。名前・接頭辞・並びの変更のあとは `catalog.load()` も呼ぶ(サイドバーの名前と並び)。
- `api/src/workspaces.ts`: 生成された関数(`getWorkspace`・`getWorkspaceUsage`・`addWorkspaceProperty` ほか、Task 9 の OperationID から orval が作る名前)を `call(...)` で包み、API の型を core の型に直す。
- `wire.ts`: `export const workspaceSettings = createWorkspaceSettings({ port: workspacesPort, catalog, toasts })`。

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm check && pnpm test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages
git add web/app/src/wire.ts
git commit -m "feat(web): ワークスペースのタブのアドレスと、設定の口と状態を足す"
```

---

### Task 11: 設定タブの枠と「一般」

**Files:**
- Modify: `web/app/src/screens/workspace/WorkspaceScreen.svelte`(タブ「概要」「設定」)
- Modify: `web/app/src/shell/Screen.svelte`(タブを渡す)
- Create: `web/app/src/screens/workspace-settings/SettingsTab.svelte`(サブタブと中身の切り替え)
- Create: `web/app/src/screens/workspace-settings/GeneralPanel.svelte`、`PrefixDialog.svelte`
- Create: `web/packages/core/src/features/workspaces/general.ts`、`general.test.ts`

**Interfaces:**
- Consumes: Task 10
- Produces(`general.ts`):

```ts
// prefixExamples は、接頭辞の欄の例(03「接頭辞」)。アイテムがなければ「アイテムはまだありません」。
export function prefixExamples(prefix: string, firstNumbers: number[]): string;
// prefixConfirm は、確認ダイアログ「接頭辞を変更」の本文。N が 0 のときは別の文。
export function prefixConfirm(oldPrefix: string, newPrefix: string, usage: WorkspaceUsage): string;
export const prefixHelp: string; // 「Epic・Story・Task・Bug・Step の ID の頭に付きます。…(番号はそのままです)。」
```

- [ ] **Step 1: 失敗するテストを書く**

`general.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { prefixConfirm, prefixExamples, prefixHelp } from "./general";

const usage = (items: number, firstNumbers: number[]) => ({ items, firstNumbers, values: {}, properties: {} });

describe("general", () => {
  it("shows up to three ids as examples", () => {
    expect(prefixExamples("HH", [1, 2, 5])).toBe("HH-1, HH-2, HH-5");
    expect(prefixExamples("HH", [])).toBe("アイテムはまだありません");
  });

  it("writes the confirm text of spec 03", () => {
    expect(prefixConfirm("HH", "WEB", usage(12, [1, 2, 3]))).toBe(
      "12 件の ID が HH-1 → WEB-1 のように変わります。古い ID で書かれたコメントやリンクは、そのままの文字で残ります。旧い接頭辞でも検索できます。",
    );
    expect(prefixConfirm("HH", "WEB", usage(0, []))).toBe(
      "接頭辞を HH から WEB に変更します。ID が変わるアイテムはありません。旧い接頭辞でも検索できます。",
    );
  });

  it("has the help text of spec 03", () => {
    expect(prefixHelp).toBe(
      "Epic・Story・Task・Bug・Step の ID の頭に付きます。変更すると、すべての ID が新しい接頭辞で表示されます(番号はそのままです)。",
    );
  });
});
```

例の区切り(「, 」)は仕様にないので、モック版 19 の一般のタブの表示を確かめて合わせる(違えばテストもそれに直す)。確認の例の「HH-1」は、いちばん小さい番号(`firstNumbers[0]`)で作る。

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test -- general`、Expected: FAIL

- [ ] **Step 3: 実装する**

- `general.ts`: 上のとおり。
- `WorkspaceScreen.svelte`: 見出しの下に `Tabs`(概要・設定)。タブを押すと `navigation.go({ name: "workspace", store, prefix, tab })`(履歴に積む。01)。設定のタブなら `SettingsTab` を出す。概要の中身はまだない(B2-15)。
- `SettingsTab.svelte`: `workspaceSettings.open(storeId, workspaceId)` を、ワークスペースが変わるたびに呼ぶ。`Tabs` で 8 つのサブタブ(`selectTab`。アドレスは変えない)。閲覧のみなら `ReadOnlyBand`。中身は、一般は `GeneralPanel`、プロパティは Task 13、ステータス・優先度・関連システムは Task 12、ワークフロー・テンプレート・自動化は空(B2-16)。
- `GeneralPanel.svelte`:
  - 名前: `Input`。Enter かフォーカスが外れたときに、いまの名前と違えば `run((p, s, w) => p.rename(s, w, value))`。断りなら `run` が読み直すので、入力は元の名前に戻る。
  - 接頭辞: `Input`(入力と同時に大文字に)、「変更」ボタンか Enter で、まず `prefixConfirm` の確認ダイアログ「接頭辞を変更」(ボタン「変更」)を出し、押したら `run((p, s, w) => p.changePrefix(s, w, value), "接頭辞を変えました")`。検査の断りはエラーのトースト(03「接頭辞」)なので、確認ダイアログを閉じてから `run` する。いまの接頭辞と同じなら何もしない。
  - 例の欄に `prefixExamples`、説明に `prefixHelp`。
  - 「別の Store へ移動」「削除」は B5。

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm check && pnpm test`、Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): ワークスペースの設定タブと、一般の名前・接頭辞を足す"
```

---

### Task 12: 値の編集(ステータス・優先度・関連システム)

**Files:**
- Create: `web/packages/core/src/features/workspaces/values.ts`、`values.test.ts`
- Create: `web/app/src/screens/workspace-settings/ValuesPanel.svelte`、`ValueRow.svelte`、`ColorMenu.svelte`、`DeleteValueDialog.svelte`、`CategoryDefaults.svelte`

**Interfaces:**
- Consumes: Task 10・11
- Produces(`values.ts`):

```ts
export const colors: { id: string; label: string }[]; // 03「色」の 18 色(id はトークンの名前、label は 03 の名前)
export const categoryLabels: Record<Category, string>; // 未着手・進行中・完了・中止
export const valuesHelp: string;  // 「名前・色・順番を変更できます。このワークスペースだけの値です。」
export const statusHelp: string;  // 「分類は、ボードの列・…見分けるのに使います。このワークスペースだけの値です。」
export interface ValueRow {
  id: string; name: string; color: string; category?: Category;
  count: number;          // 使っているアイテムの件数
  initial: boolean;       // 「初期」の印(ステータス・優先度)
  initialDisabled?: string; // 「初期ステータスにする」を無効にする理由(完了・中止のステータス)
  first: boolean; last: boolean; // 上へ・下への無効
}
export function valueRows(settings: WorkspaceSettings, list: ValueList, usage: WorkspaceUsage): ValueRow[];
export function valueListLabel(settings: WorkspaceSettings, list: ValueList): string; // 「ステータス」「「顧客」の選択肢」
// deleteDialog は、確認ダイアログ「「X」を削除」の中身(03「削除」の表)。
export interface DeleteDialogModel {
  title: string;               // 「「X」を削除」
  body: string;                // 表の本文
  replacements?: { value: string; label: string }[]; // 単一の値で使っているときだけ
  initialReplacement?: string;
  replaceNote?: (replacementName: string) => string; // ステータスで使っているとき「N 件を「{代わり}」に置き換えます。」
  statusNote?: string;         // ステータスのとき「ワークフローの制約と自動化(プロジェクト別のルールを含む)からも削除されます。」
}
export function deleteDialog(settings: WorkspaceSettings, list: ValueList, valueId: string, usage: WorkspaceUsage): DeleteDialogModel;
```

- [ ] **Step 1: 失敗するテストを書く**

`values.test.ts`(設定は `fixtures.test-util.ts` の既定の設定を使う。`Review` を足した設定や件数は、その場で作る):

```ts
describe("value rows", () => {
  it("marks the initial status and disables it for done and canceled", () => {
    const rows = valueRows(settings(), "statuses", usage({ [ids.todo]: 3 }));
    expect(rows.map((r) => [r.name, r.initial, r.count, r.initialDisabled ?? ""])).toEqual([
      ["Todo", true, 3, ""],
      ["InProgress", false, 0, ""],
      ["Done", false, 0, "完了・中止のステータスは初期ステータスにできません"],
      ["Pending", false, 0, ""],
      ["Cancel", false, 0, "完了・中止のステータスは初期ステータスにできません"],
    ]);
    expect(rows[0]?.first && rows[4]?.last).toBe(true);
  });

  it("has the 18 colors of spec 03 in order", () => {
    expect(colors.map((c) => c.label).join("、")).toBe(
      "グレー、青灰、茶、赤、ローズ、桃、赤紫、紫、藍、青、空色、水色、青緑、緑、オリーブ、山吹、黄、橙",
    );
  });
});

describe("delete dialog", () => {
  it("says nobody uses it", () => {
    const d = deleteDialog(settings(), "systems", ids.web, usage({}));
    expect([d.title, d.body, d.replacements]).toEqual(["「Web」を削除", "使っているアイテムはありません。", undefined]);
  });

  it("asks for a replacement of a single value and defaults to the initial status", () => {
    const d = deleteDialog(settings(), "statuses", ids.pending, usage({ [ids.pending]: 4 }));
    expect(d.body).toBe("4 件がこの値を使っています。代わりの値を選んでください。");
    expect(d.initialReplacement).toBe(ids.todo);
    expect(d.replacements?.map((r) => r.label)).toEqual(["Todo", "InProgress", "Done", "Cancel"]);
    expect(d.replaceNote?.("Todo")).toBe("4 件を「Todo」に置き換えます。");
    expect(d.statusNote).toBe("ワークフローの制約と自動化(プロジェクト別のルールを含む)からも削除されます。");
  });

  it("removes a multiple value", () => {
    const d = deleteDialog(settings(), "systems", ids.web, usage({ [ids.web]: 2 }));
    expect(d.body).toBe("2 件がこの値を使っています。それらからこの値が削除されます。");
  });

  it("offers なし for a single choice", () => {
    const s = withSingleChoice(settings()); // 「環境」(単一選択)に「本番」「検証」を足した設定
    const d = deleteDialog(s, ids.env, ids.prod, usage({ [ids.prod]: 1 }));
    expect(d.replacements?.map((r) => r.label)).toEqual(["なし", "検証"]);
    expect(d.initialReplacement).toBe("");
    expect(d.statusNote).toBeUndefined();
  });
});
```

ステータスの説明 `statusHelp` が 03 の文(「分類は、ボードの列・バックログ・バーンダウン・マイタスクで「未着手・進行中・完了・中止」を見分けるのに使います。このワークスペースだけの値です。」)であることも確かめる。

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test -- values`、Expected: FAIL

- [ ] **Step 3: 実装する**

- `values.ts`: 上のとおり。
- `ValuesPanel.svelte`(props: `list`): 説明、ステータスなら一覧の上に `CategoryDefaults`(「未着手の既定」「進行中の既定」「完了の既定」「中止の既定」の 4 つの `Select`。選択肢はその分類のステータス。選ぶと `setCategoryDefault`)、行の一覧、末尾に新しい値の名前の `Input` と「追加」(`addValue`。成功したら入力を空に)。
- `ValueRow.svelte`: 色のボタン(`ColorMenu` で 18 色)、名前の `Input`(Enter かフォーカスが外れたら `renameValue`)、ステータスなら分類の `Select`(`setStatusCategory`)、ステータスと優先度なら「初期」の印か「初期ステータスにする」(「初期優先度にする」)のボタン(無効の理由を `title` に)、件数、上へ・下へ(`moveValue`)、ごみ箱。
- ごみ箱: まず `deleteDialog` を作る。本文は、ダイアログを開く前にサーバーへ聞かずに出す。削除の断り(初期・既定)は、「削除」を押したときにサーバーが返す文をエラーのトーストで出す(03「初期ステータス」の「エラーのトーストで出す」)。`DeleteValueDialog.svelte` はタイトル `d.title`、危険操作の見た目のボタン「削除」、`d.body`、代わりの値の `Select`(`d.replacements` があるとき)、`d.replaceNote(選んだ名前)`、`d.statusNote`。「削除」で `run((p, s, w) => p.deleteValue(s, w, list, id, replacement), \`「${name}」を削除しました\`)`。
- 作ったばかりのワークスペースで「Todo」のごみ箱を押して「削除」を押すと、サーバーの断り「「Todo」は初期ステータスで、未着手の既定のステータスでもあります。…」がエラーのトーストに出る。

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm check && pnpm test`、Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): ステータス・優先度・関連システムの値の編集画面を足す"
```

---

### Task 13: プロパティのタブと選択肢

**Files:**
- Create: `web/packages/core/src/features/workspaces/properties.ts`、`properties.test.ts`
- Create: `web/app/src/screens/workspace-settings/PropertiesPanel.svelte`、`PropertyRow.svelte`、`KindsMenu.svelte`、`AddPropertyDialog.svelte`、`ChoicesDialog.svelte`、`DeletePropertyDialog.svelte`

**Interfaces:**
- Consumes: Task 10〜12(`ChoicesDialog` は `ValuesPanel` の行と同じ `ValueRow` を、`list` にプロパティの ID を渡して使う)
- Produces(`properties.ts`):

```ts
export const typeLabels: Record<PropertyType, string>;
// status ステータス, user ユーザー, priority 優先度, date 日付, number 数値, duration 時間, systems 関連システム,
// text テキスト, single 単一選択, multi 複数選択, check チェック, url URL
export const addableTypes: PropertyType[]; // 03 の表の順: text, number, date, single, multi, user, check, url
export const propertiesHelp: string; // 「組み込みのプロパティは名前の変更や、…(例: customer:サンプル)。」
export const keyHint: string;        // 「英小文字・数字・_。絞り込みで使います」
export interface PropertyRow {
  id: string; name: string; key: string; builtin: boolean; type: string; // type は typeLabels の名前
  choices?: number;      // 単一・複数選択だけ。「選択肢 N」
  kinds: string[] | "all"; // すべての種類なら "all"(「すべて」と出す)
  kindsFixed: boolean;   // ステータス(使う種類を変えられない)
  hide?: "非表示" | "表示"; // 組み込み(ステータス以外)だけ
  deletable: boolean;    // 追加プロパティだけ
  hidden: boolean;       // 行を薄く出す
  first: boolean; last: boolean;
}
export function propertyRows(settings: WorkspaceSettings): PropertyRow[];
// deletePropertyBody は、確認ダイアログ「プロパティを削除」の本文。
export function deletePropertyBody(name: string, count: number): string;
```

- [ ] **Step 1: 失敗するテストを書く**

```ts
describe("property rows", () => {
  it("lists the eight built-ins with their controls", () => {
    const rows = propertyRows(settings());
    expect(rows.map((r) => [r.key, r.type, r.hide ?? "", r.deletable, r.kindsFixed, r.kinds])).toEqual([
      ["status", "ステータス", "", false, true, "all"],
      ["assignee", "ユーザー", "非表示", false, false, "all"],
      ["priority", "優先度", "非表示", false, false, "all"],
      ["start", "日付", "非表示", false, false, "all"],
      ["end", "日付", "非表示", false, false, "all"],
      ["points", "数値", "非表示", false, false, "all"],
      ["estimate", "時間", "非表示", false, false, "all"],
      ["systems", "関連システム", "非表示", false, false, "all"],
    ]);
  });

  it("shows choices, kinds and a trash for an added property", () => {
    const s = withSingleChoice(settings()); // 「環境」(単一選択、Story と Task、選択肢 2 つ)
    const row = propertyRows(s).at(-1);
    expect([row?.type, row?.choices, row?.kinds, row?.deletable]).toEqual(["単一選択", 2, ["Story", "Task"], true]);
  });

  it("writes the delete body of spec 03", () => {
    expect(deletePropertyBody("顧客", 0)).toBe("「顧客」を削除します。");
    expect(deletePropertyBody("顧客", 3)).toBe("「顧客」を削除します。値の入っている 3 件から値が消えます。");
  });
});
```

`propertiesHelp`・`keyHint` が 03 の文字どおりであることも確かめる。

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test -- properties`、Expected: FAIL

- [ ] **Step 3: 実装する**

- `PropertiesPanel.svelte`: 説明、表(名前・キー・型・使う種類・操作の順)、「プロパティを追加」ボタン。
- `PropertyRow.svelte`: 名前の `Input`(Enter かフォーカスが外れたら `renameProperty`)、キーをコードの書体で(組み込みは「組み込み」を添える)、型(選択式なら「選択肢 N」のボタンで `ChoicesDialog`)、使う種類(種類のアイコンを並べる。すべてなら「すべて」。押すと `KindsMenu`)、上へ・下へ、「非表示」「表示」か、ごみ箱。非表示の行は薄く。
- `KindsMenu.svelte`: 5 つの種類のチェック。チェックのたびに `setPropertyKinds`(メニューは開いたまま)。ステータスでは開かない(`kindsFixed`)。断り「少なくとも 1 つの種類で使ってください」はエラーのトースト。
- `AddPropertyDialog.svelte`: 名前、キー(`keyHint` を添える)、型(`addableTypes`)、使う種類(チェック。1 つも選ばないとすべて)。断りはダイアログの中に赤字。成功したら「プロパティを追加しました」のトーストを出してダイアログを閉じ、型が単一・複数選択なら続けて `ChoicesDialog` を開く。
- `ChoicesDialog.svelte`: タイトル「「X」の選択肢」。説明は `valuesHelp`。中身は Task 12 の行と追加の欄(初期値の印と分類はない)。閉じるボタン「閉じる」。閲覧のみなら中を無効にする。
- `DeletePropertyDialog.svelte`: タイトル「プロパティを削除」、危険操作の見た目、本文 `deletePropertyBody(name, usage.properties[id] ?? 0)`、ボタン「削除」。成功したら「プロパティを削除しました」。保存したビューの添え書きは E(B2-20)。

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm check && pnpm test`、Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): プロパティのタブと、選択肢・使う種類・追加・削除のダイアログを足す"
```

---

### Task 14: ワークスペースの作成ダイアログと、サイドバーのメニュー

**Files:**
- Create: `web/packages/core/src/features/workspaces/createDialog.ts`、`createDialog.test.ts`
- Create: `web/app/src/screens/workspace-settings/CreateWorkspaceDialog.svelte`
- Create: `web/app/src/shell/StoreMenu.svelte`
- Modify: `web/app/src/shell/SidebarStore.svelte`(Store のメニューを置く)、`WorkspaceMenu.svelte`、`web/app/src/App.svelte`、`web/app/src/wire.ts`

**Interfaces:**
- Produces:

```ts
export interface WorkspaceForm { storeId: string; name: string; prefix: string }
export interface WorkspaceDialogState {
  form: WorkspaceForm;
  stores: { value: string; label: string }[]; // 書き込める Store(閲覧のみを除く)
  blocked: string; // Store がない・書ける Store がないときの文。空なら入力を出す
  error: string;
}
export const prefixPreviewHelp: string; // 「Epic・Story・Task・Bug・Step の ID が WEB-1 のようになります。」
export interface WorkspaceDialog {
  state: Readable<WorkspaceDialogState | null>;
  open(storeId?: string): void;
  update(patch: Partial<WorkspaceForm>): void; // prefix は大文字にして持つ
  submit(): Promise<void>;
  close(): void;
}
export function createWorkspaceDialog(deps: {
  catalog: Catalog; port: WorkspacesPort; onCreated(storeId: string, workspace: Workspace): void;
}): WorkspaceDialog;
```

- [ ] **Step 1: 失敗するテストを書く**

```ts
describe("workspace dialog", () => {
  it("opens on the store of the menu and upper-cases the prefix", async () => {
    const { dialog } = await setup();
    dialog.open("S1");
    dialog.update({ prefix: "web" });
    expect(get(dialog.state)?.form).toEqual({ storeId: "S1", name: "", prefix: "WEB" });
    expect(get(dialog.state)?.stores.map((s) => s.value)).not.toContain("S2"); // 閲覧のみ
  });

  it("shows the refusal inside the dialog", async () => {
    const { dialog, port } = await setup();
    dialog.open("S1");
    port.refuseNext("この Store には同じ接頭辞のワークスペースがあります");
    await dialog.submit();
    expect(get(dialog.state)?.error).toBe("この Store には同じ接頭辞のワークスペースがあります");
  });

  it("reloads the sidebar and opens the overview after creating", async () => {
    const { dialog, created } = await setup();
    dialog.open("S1");
    dialog.update({ name: "Docs", prefix: "DOC" });
    await dialog.submit();
    expect(get(dialog.state)).toBeNull();
    expect(created).toEqual([["S1", "Docs"]]);
  });

  it("tells why it cannot add a workspace", async () => {
    const none = await setup({ stores: [] });
    none.dialog.open();
    expect(get(none.dialog.state)?.blocked).toBe("先に Store を追加してください");
    const readOnly = await setup({ stores: "read-only only" });
    readOnly.dialog.open();
    expect(get(readOnly.dialog.state)?.blocked).toBe("ワークスペースを追加できる Store がありません");
  });
});
```

`setup` の Store の組み替えは、`fakeCatalogPort` の引数でできる形に合わせる(できなければ、このテストの中で `CatalogPort` の偽物を作る)。

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test -- createDialog`、Expected: FAIL

- [ ] **Step 3: 実装する**

- `createDialog.ts`: プロジェクトの作成ダイアログ(`features/projects/createDialog.ts`)と同じ形。作ったら `catalog.load()` のあと `onCreated`。
- `wire.ts`: `onCreated` で `navigation.go(routeOfWorkspace(get(catalog.stores), storeId, workspace.id, "overview"))`(03「作ると、そのワークスペースの概要を開く」。トーストは出さない)。
- `CreateWorkspaceDialog.svelte`: タイトル「ワークスペースを追加」、Store・名前・接頭辞(`prefixPreviewHelp` を説明に)、`blocked` があれば入力の代わりに文を出す。断りは赤字。
- `StoreMenu.svelte`: 「ワークスペースを追加」だけ(B2-17)。閲覧のみの Store では無効にし `readOnlyNote` を添える。
- `WorkspaceMenu.svelte`: 「プロジェクトを作成」に続けて「設定を開く」(`navigation.go` で設定のタブ)、「上へ移動」(先頭で無効)、「下へ移動」(末尾で無効)。上へ・下へは閲覧のみの Store で無効。押したら `workspacesPort.move(...)` のあと `catalog.load()`。断りはエラーのトースト。

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm check && pnpm test`、Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src
git commit -m "feat(web): ワークスペースの作成ダイアログと、Store・ワークスペースのメニューの項目を足す"
```

---

### Task 15: e2e(3 幅)

**Files:**
- Create: `web/e2e/settings.spec.ts`

e2e は、既存の `web/e2e/*.spec.ts` と同じ準備(`helpers.ts` の `tokenUrl`・`sidebar`・`unique`)を使い、Playwright の設定にある 3 幅で通す。e2e のサーバーの Store(書ける Store と閲覧のみの Store)は、既存の spec が使っているものを使う。名前は `unique(info, …)` で毎回変える。

- [ ] **Step 1: テストを書く**

次の流れを、それぞれ 1 つの `test` にする:

1. **作成**: Store のメニュー →「ワークスペースを追加」→ 名前と接頭辞(小文字で入れて大文字になる)→ 作る → そのワークスペースの概要が開き、サイドバーに出る。同じ接頭辞でもう一度作ると、ダイアログに「この Store には同じ接頭辞のワークスペースがあります」。
2. **メニュー**: ワークスペースのメニュー →「設定を開く」で設定のタブが開き、サブタブが 8 つ並ぶ。「下へ移動」で、サイドバーの並びが変わる。
3. **一般**: 名前を空にしてフォーカスを外すと「名前は空にできません」のトーストが出て、元の名前に戻る。接頭辞を変えると、確認ダイアログ(アイテムがないので「接頭辞を {旧} から {新} に変更します。…」)→「変更」→ トースト「接頭辞を変えました」、見出しの接頭辞が変わる。
4. **ステータス**: 「Review」を足す → 色を紫に → 分類を進行中に → 上へ → 初期ステータスにする(行に「初期」)→ 「Todo」のごみ箱 → 「削除」→ 「Todo」は未着手の既定なので断りのトースト → 未着手の既定を「Pending」に → もう一度消すと「「Todo」を削除しました」。
5. **プロパティ**: 「プロパティを追加」→ 名前「顧客」、キー `Customer` で断り(ダイアログの中)→ キー `customer`、型「単一選択」→ トースト「プロパティを追加しました」と選択肢のダイアログ → 「A社」を足して閉じる → 行に「選択肢 1」→ 使う種類のメニューで Epic を外す → 担当者を非表示(行が薄く、ボタンが「表示」)→ 「顧客」のごみ箱 →「プロパティを削除」→「削除」→ トースト「プロパティを削除しました」。
6. **閲覧のみ**: 閲覧のみの Store のワークスペースの設定を開くと、サブタブは押せるが、名前の入力欄と「追加」のボタンが無効。

- [ ] **Step 2: 通るのを確かめる**

Run: `cd web && pnpm e2e -- settings.spec.ts`(既存の e2e の実行方法に合わせる)
Expected: 3 幅すべて PASS。落ちたら、画面の作り(Task 11〜14)の誤りを直す。テストの期待を仕様から外さない。

- [ ] **Step 3: 全体を確かめる**

Run: `make check && (cd web && pnpm check && pnpm test && pnpm e2e)`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add web/e2e
git commit -m "test(e2e): ワークスペースの設定(作成・一般・ステータス・プロパティ・閲覧のみ)を 3 幅で確かめる"
```

---

## 最後に

- 全タスクのあと、`superpowers:requesting-code-review` で B2 全体をレビューし、指摘を直す。
- B2 の PR(develop 向け、ブランチ `feat/track-b2`)の本文に、決めたこと B2-1〜B2-21 と、ほかのトラックへの申し送り(漏れの表の「A3・A4・C・E・G」の行)を書く。
- A3 の計画を書く人へ: アイテムの値は Task 1 で入っている(B2-1)。作成時の初期値(初期ステータス・初期優先度)と、変更の操作を A3 で足す。
