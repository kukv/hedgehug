# hedgehug トラック E3a(レイアウトの切り替えと検索のツリー)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 検索の画面にレイアウトの切り替え(ツリー・テーブル)を置き、レイアウトごとの設定を画面の状態に持つ。ツリー・ガントでの `sort:`・`group:` の注記と、削除済みの案内を出す。検索の画面のツリーを作る。ツリーは Store をまたいでワークスペースの見出し・プロジェクトの行・アイテムの行を入れ子に出し、一致しない祖先の行は薄く出す。プロジェクトの行のメニュー・キー操作・「名前を変更…」のダイアログ・プロジェクトの並べ替えも作る。保存したビューは E3b、カンバンは E5、ガントは E6 で作る。

**Architecture:** 一致するかどうかは、これまでどおりサーバーの `POST /api/search` だけが決める(E1-1)。ツリーの形は画面が作る。木に入れるプロジェクトのアイテム・ワークスペースの設定・Store のメンバーを、プロジェクトの画面と同じ口(`itemsPort.list`・`workspacesPort.get`・`listMembers`)で読み、一致した行の ID の集まりで木を刈り込む(E3a-3)。こうすると、祖先の行も、進捗やブロック中のように一致しない子も数える値も、`Item` の形のままそろう。そのため、既存の行の部品・フォーカス・書き込み・アイテムのメニューをそのまま使える。フォーカスと開閉の core の部品は、行の型を引数にとる形にする。Store をまたぐので、行の鍵は `{storeId}/{内部 ID}` にする(E3a-5)。ツリーの部品は E7 も使うので、`web/app/src/screens/tree/` に置く。

**Tech Stack:** Go 1.27.3、huma v2。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`: 「並び順と並べ替え」(77〜86 行。プロジェクトの「上へ移動」「下へ移動」)、「削除済みを探す」のツリーの案内(314 行)、「検索」(354〜360 行)、「4 つのレイアウトに共通すること」の全節(434〜500 行)、「ツリー」の「行」「絞り込んだとき」「空のとき」「ツリーのキー操作」(502〜525・553〜578 行。ドラッグの 527〜551 行は E7)、「この文書の操作が残す記録」の並べ替えの行(959 行)、「モックからの変更点」の 1043・1045・1061・1065・1069・1088 行
- 02: 「名前の変更・移動・削除」(587 行。プロジェクトのメニューと「名前を変更…」)、コマンドパレットの「検索を開く」
- 01: 「画面のアドレス」(163・173〜178 行。レイアウトの切り替えと折りたたみは履歴に積まない、画面の状態はこのマシンに残す)
- `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`(トラック E の分け方)、`2026-11-15-hedgehug-track-e1.md` の末尾「E3(保存したビュー)」、`2026-11-13-hedgehug-track-a4b.md` の末尾の E への申し送り(プロジェクトの行・プロジェクトのメニュー・Epic の Alt+Shift+↑↓)

## 始める前に

- E3 は 2 本に分けた(ユーザーの決定、2026-11-15)。**E3a = この計画**、E3b = 保存したビュー(state.db の表と API、別のブラウザのタブへの反映、ダイアログ「ビューとして保存」、ビューの画面と「未保存の変更」、ビューのメニュー、サイドバーの「保存したビュー」、パレットのビューの群と「ビューとして保存…」、03 の 265 行のプロパティの削除の確認の添え書き)。E3a のあと、E3b と E5〜E7 を並行して進める。ブランチは `feat/track-e3`(develop から)。
- push は `git push -u origin feat/track-e3`(ブランチの上流が `origin/develop` になっているため)。

## Global Constraints

E1・E2a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。いまの数: `internal/app/projects` 12 個、`internal/app/search` 13 個、`internal/adapter/httpapi/projects/edit.go` 117 行、`internal/adapter/httpapi/search/search.go` 168 行、`web/app/src/screens/search/SearchScreen.svelte` **194 行**(Task 8 で分ける)、`web/app/src/search.ts` 179 行、`web/app/src/wire.ts` **298 行**(足さない)、`web/app/src/wireActions.ts` 179 行、`web/app/src/App.svelte` 142 行、`web/app/src/shell/Sidebar.svelte` 156 行、`web/app/src/screens/project/treeKeyboard.ts` 249 行、`TreeRow.svelte` 185 行、`web/packages/core/src/features/search/search.ts` **193 行**、`features/items/treeFocus.ts` 136 行、`treeWrites.ts` 106 行、`web/packages/core/src/index.ts` 184 行、`features/palette/actions.ts` 240 行。
- 画面のパッケージの依存の向き: ui は core を import しない。app の画面どうし(`screens/project` と `screens/search`)は、共通の部品を `screens/tree/` に置いて使う。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- 文言は仕様の文のとおりに書く。閲覧のみは core の `readOnlyNote`、人は `ownerName`(「なし」「不明なユーザー」)。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする。
- 既存の通しのテストは `getByRole("list", { name: "ツリー" })` でツリーを探している(16 か所)。プロジェクトの画面のツリーの role と名前は変えない。検索の画面のツリーも `<ul aria-label="ツリー">` にする(別の画面なので重ならない)。
- e2e のビルドも CI で lint する。手元の検証は 1 回で足りる。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| E3a-1 | 切り替えに出すのは、作ってあるレイアウト(ツリー・テーブル)だけにし、この順で並べる。カンバンは E5、ガントは E6 が、04 の並び(ツリー・カンバン・テーブル・ガント)の位置に足す。中身のないボタンは置かない | ユーザーの決定(2026-11-15) |
| E3a-2 | 画面の状態 `search` の中身を `{query, layout, table, tree}` にする。`layout` は `"tree"`・`"table"`(E5・E6 が値を足す)、`tree` は `{collapsed: string[]}`(閉じている行の鍵)。初めは `layout` が `"table"`、`collapsed` が空。形の合わない項目だけ初めの値にする(E2a-13 と同じ) | 04 の 461〜477 行。01 の 178 行(折りたたみも画面の状態で、このマシンに残す) |
| E3a-3 | ツリーの形は画面が作る。一致の判定はサーバーの応答の行だけを使い、画面で照合しない。木に入れるプロジェクトは、絞り込み欄が空(前後の空白を除く)なら表示中の Store のすべてのプロジェクト、そうでなければ一致した行があるプロジェクトだけにする。そのプロジェクトについて、アイテム(`itemsPort.list`)・ワークスペースの設定(`workspacesPort.get`)・Store のメンバー(`listMembers`)を読み、`buildTree` で木にする。そのうえで、一致した行と、その祖先だけを残す。一致しない祖先は薄く出す。読んだものは鍵(プロジェクト・ワークスペース・Store)ごとに取っておき、Store が変わった知らせ(`reloads`)で捨てて読み直す | 04 の 514 行(一致したアイテムとその祖先、一致を含まないプロジェクトを隠す)。E1-1(照合は 1 か所)。進捗・ブロック中・Step の「完了/対象」は一致しない子も数えるので、プロジェクトのアイテムをすべて読む |
| E3a-4 | 並び: Store はサイドバーの並び(`catalog.stores`)、ワークスペース・プロジェクトはサイドバーと同じ並び(catalog の並び)、アイテムは兄弟の並び(`compareSiblings`)。`sort:` と列見出しの並べ替えは使わない。ワークスペースの見出しは、プロジェクトの行が 1 つ以上出るワークスペースにだけ出す(見出しだけの空の段を作らない) | 04 の 506 行、452 行 |
| E3a-5 | 行の鍵は `{storeId}/{内部 ID}`(プロジェクトの行もアイテムの行も)。フォーカス・開閉・`data-row` はこの鍵を使う。プロジェクトの行の親は null、Epic の行の親はプロジェクトの行の鍵、ほかは親のアイテムの行の鍵 | 04 の 457〜459 行(Store をまたぐと同じ ID が並ぶ)。内部 ID は Store の中でしか一意と言えないため |
| E3a-6 | 件数「{N} 件」は、木に描いた行のうち一致したアイテムの行の数(薄い祖先・プロジェクトの行は数えない。親が見つからず木に入らない行も数えない) | 04 の 455 行 |
| E3a-7 | ツリーでは、ツールバーにレイアウトの設定を出さない(ツリーは設定を持たない)。`is:deleted` の検索では、本体の代わりに「削除済みのアイテムはテーブルで表示します。」とボタン「テーブルで開く」を出し、件数は 0 件にする。押すと、絞り込みを残してテーブルに切り替える | 04 の 314・467 行 |
| E3a-8 | 注記: ツリーのとき、応答の `hasSort`(`sort:` の語があるか。サーバーが返す)と `group`(空でなければ `group:` あり)から、「ツリー・ガントでは sort: は使われず、並び順のままです」「ツリー・ガントでは group: は使われず、並び順のままです」「ツリー・ガントでは sort: と group: は使われず、並び順のままです」のどれかを、絞り込み欄の下に灰色で出す。エラーとは別に出す。カンバンの注記は E5 | 04 の 452 行。E1-1(画面は解析しない) |
| E3a-9 | テーブルで開く入口: サイドバーの「検索」、パレットのコマンド「検索を開く」、パレットの「このクエリで」の「検索で開く」と「{N} 件をまとめて変更…」は、レイアウトを `table` にしてから開く。すでに検索の画面にいても、テーブルに切り替える。再読み込みと戻る・進むで開いたときは、画面の状態のレイアウトで開く | 04 の 359 行。01 の 173 行(戻る・進むは画面の状態で復元する) |
| E3a-10 | レイアウトを切り替えると、テーブルの選択を解除する(ツリーに選択はない) | 04 の 818・823 行(選択はテーブルとバックログのもの。一覧から消えたアイテムは選択から外す) |
| E3a-11 | 開閉は画面の状態の `tree.collapsed` に書く。行がなくなった鍵も残す(絞り込みを戻すとまた閉じて出る)。フォーカスは書かない | 04 の 476・557 行 |
| E3a-12 | プロジェクトの行: 開閉の矢印、Store の点、プロジェクトのアイコン、タイトル(押すとプロジェクトの概要。Ctrl+クリックは新しいタブ)、ステータス(点と名前。表示だけ)、オーナー(`ownerName`)、進捗(棒と「N%」。数えるのは、プロジェクトの下の Epic と Step を除くアイテムのうち中止を除くもの)、「Epic を追加」の「+」、メニューのボタン(読み上げの名前「{タイトル} のメニュー」) | 04 の 25・509 行、02 の 587 行 |
| E3a-13 | プロジェクトのメニュー: 「開く」「Epic を追加」「上へ移動」「下へ移動」「名前を変更…」「削除…」の順。閲覧のみの Store では、「開く」以外を無効にし、「閲覧のみの Store です」と添える。「上へ移動」「下へ移動」は、サイドバーの並び(そのワークスペースの削除されていないすべてのプロジェクト。絞り込みで隠れたものも含む)の先頭・末尾で無効にする | 04 の 512 行(並びはサイドバーと共通)、86 行。02 の 587 行の並び |
| E3a-14 | 「名前を変更…」のダイアログ: 見出し「プロジェクトの名前を変更」、入力欄の名前「タイトル」(初めは今のタイトル)、ボタン「保存」「キャンセル」、Enter で保存。検査と断りの文は `projectEditor.rename`(概要でのタイトルの変更と同じ)。保存できたら閉じる | 02 の 587 行(タイトルを入れるダイアログ、概要と同じ検査)。ビューの「名前を変更…」(04 の 426 行)と同じ組み立て |
| E3a-15 | プロジェクトの並べ替えの API: `POST /api/stores/{store}/projects/{project}/move`(本文 `{direction: "up"\|"down"}`、`MoveWorkspace` と同じ形)。端なら何も書かない。アクティビティは「{タイトル} の並びを変更」。トーストは出さない。動かしたあとは catalog を読み直す(サイドバーにも出る) | 04 の 79〜86・959 行 |
| E3a-16 | プロジェクトの行のキー: ↑↓→←、Enter(概要を開く)、Ctrl+Enter(概要を新しいタブ)、C(その下に Epic を作る作成ダイアログ)、Alt+↑↓(プロジェクトのメニューの「上へ移動」「下へ移動」と同じ)、メニューキー・Shift+F10。Space・S・K・Alt+Shift+↑↓ は何もしない。閲覧のみの Store では C・Alt+↑↓ は何もしない | 04 の 559〜574 行の「プロジェクトの行」の列 |
| E3a-17 | Epic の Alt+Shift+↑↓: 「移動…」の候補(プロジェクト)のうち、いま見えているプロジェクトの行を、アイテムと同じ規則でたどって新しい親にする。たどるのは、フォーカスのある行と同じ Store・同じワークスペースの、見えている行だけ。プロジェクトの画面では、プロジェクトの行がないので何もしない(いまのまま) | 04 の 576 行、A4b の申し送り |
| E3a-18 | アイテムの行は、プロジェクトの画面の行の部品(`TreeRow`)を使い、Store の点を ID の前に足す。薄い行は文字を薄くするだけで、操作は一致した行と同じにする | 04 の 459・514 行(薄く出す) |
| E3a-19 | 空のとき: 表示中の Store にプロジェクトが 1 つもなければ「プロジェクトはまだありません」と「ワークスペースにプロジェクトを作ると、ここに Epic・Story・Task・Bug・Step の木が並びます。」。行が 0 件なら「一致するものはありません」と「絞り込みを変えてみてください。」(書き方に誤りがあるときも同じ)。検索の画面には「作成」のボタンを置かない | 04 の 520〜523 行(「プロジェクトを作成」はワークスペースの画面だけ、「Epic を作成」はプロジェクトの画面だけ)、451 行 |
| E3a-20 | パレットの「選んでいるアイテム」: ツリーにフォーカスがあり、フォーカスのある行がアイテムの行なら、そのアイテム(rank 1、`source: "tree"`。プロジェクトの画面のツリーと同じ)。ステータスの変更は core の `setRowStatus` で、検索を読み直す | A5c-9、E2a-18 |
| E3a-21 | プロジェクトの画面とワークスペースの画面のツリーのタブの開閉を画面の状態に残すことは、そのタブの画面の状態の鍵を決める E2c・E7 のうち E7 が行う(トラック E の分け方に書く) | 04 の 474 行。タブの画面の状態の鍵は E2c で決める |
| E3a-22 | 既存の不具合の直し: 新しいブラウザのタブで、パレットの「検索で開く」「{N} 件をまとめて変更…」から検索の画面を開くと、保存した列などの設定が初めの値で上書きされていた。`prepareQuery` を、画面の状態を読んでから絞り込みだけを置き換える形にする(Task 3) | ユーザーの決定(2026-11-15)。04 の 359 行(レイアウトの設定と列は変えない) |

## Review Focus

1. **同じ ID のアイテムが 2 つの Store にあると、片方の開閉やフォーカスがもう片方に効く。** Store「work」と Store「team」のどちらにも HH-3 があるとき、Store「work」の HH-3 を閉じても Store「team」の HH-3 は開いたまま、↓ で 2 つは別の行として順に移る(E3a-5)。Task 4 の `keys are per store`、Task 8 の e2e に入れる。
2. **絞り込み中の「上へ移動」が、見えていない兄弟を飛び越えない・または動かす。** 兄弟が HH-3・HH-4・HH-5・HH-7・HH-9 で HH-4・HH-7 が一致しないとき、HH-5 の「上へ移動」で並びは HH-5・HH-3・HH-4・HH-7・HH-9 になる(04 の 512 行)。見えている行だけを `rows` に渡せば既存の `visibleNeighbors` がそうする。Task 4 の `rowsFor gives only visible siblings` と Task 8 の e2e に入れる。
3. **閲覧のみの Store の行と書ける Store の行が同じツリーに並ぶと、片方の判定がもう片方に移る。** 閲覧のみの Store の Epic の C・Space は何も起こさず、すぐ下の書ける Store の Epic の C はダイアログを開く(書き込みの画面はフォーカスのある行の Store で決める)。Task 4 の `screenFor uses the row's store` と Task 8 の e2e に入れる。
4. **一致しない子を数えずに、進捗やブロック中がずれる。** `kind:task` で Story の下の Task だけが一致するとき、薄い Story の行の Step の「完了/対象」とプロジェクトの行の進捗は、絞り込まないときと同じ数になる(E3a-3)。Task 4 の `progress counts unmatched children` に入れる。
5. **入口によってはテーブルに戻らない。** ツリーにしたあと、サイドバーの「検索」を押すとテーブルに戻り、再読み込みではツリーのまま開く(E3a-9)。Task 3 の `showTable switches to table` と Task 9 の e2e に入れる。

---

### Task 1: プロジェクトの並べ替え(サーバーと口)

**Files:**
- Create: `internal/app/projects/move.go`、`internal/app/projects/move_test.go`
- Modify: `internal/adapter/httpapi/projects/edit.go`(`move-project` を登録。本文は `workspaces.MoveWorkspaceItem` と同じ形の型をこのパッケージに置く)、`internal/adapter/httpapi/projects/edit_test.go`、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(生成)、`web/packages/api/src/projects.ts`、`web/packages/api/src/projects.ports.test.ts`、`web/packages/core/src/port/projects.ts`

**Interfaces:**
- Produces:
  - Go: `func (s Service) MoveProject(ctx context.Context, store port.StoreID, id project.ID, up bool) error`
  - API: `POST /api/stores/{store}/projects/{project}/move`、本文 `{direction: "up"|"down"}`、応答なし(204)
  - core: `ProjectsPort.move(storeId: string, projectId: string, direction: Direction): Promise<void>`(`Direction` は `port/workspaces.ts` のもの)

- [ ] **Step 1: 失敗するテストを書く**

`move_test.go`(`apptest` の Store に同じワークスペースのプロジェクト A・B・C を作る):
- `TestMoveProjectUp`: B を上へ → `List` の並びが B・A・C。ログの最後の操作の要約が「B の並びを変更」。
- `TestMoveProjectEdgesWriteNothing`: A を上へ、C を下へ → どちらも err なし、A・C のログの行が増えない(`app/workspaces/order_test.go` の 46〜53 行の端のテストを写す)。
- `TestMoveProjectOnlyMovedRankChanges`: B を下へ → A・C の `Rank` は前と同じ。
- `TestMoveProjectWithoutRanks`: rank のないプロジェクト(`apptest.Sample` のもの)を動かすと、同じ保存で表示の順に rank を振ってから動かす(`TestWorkspaceOrderWithoutRanks` と同じ)。
- `TestMoveProjectOtherWorkspaceUntouched`: 別のワークスペースのプロジェクトの並びは変わらない。
- `TestMoveProjectDeleted`: 削除済みのプロジェクトの扱いは `Rename` と同じ(`s.editable` と `saveAs` の流れを通す)。削除の根でないもの(ワークスペースごと削除されたもの)は `port.ErrNotFound`。
- 履歴: プロジェクトの履歴は `rank` の行を出さない(`internal/domain/history/project_test.go` の O10 の行がすでに確かめている)。足すテストはない。

`edit_test.go` に `TestMoveProjectRoute`: `POST …/move` `{"direction":"down"}` → 204、端で `{"direction":"up"}` → 204(何も書かない)、`{"direction":"left"}` → 422。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/projects/ ./internal/adapter/httpapi/projects/ -run 'Move'`
Expected: FAIL(`MoveProject undefined`)

- [ ] **Step 3: `MoveProject` と API を書く**

並びの決め方は `MoveWorkspace`(`internal/app/workspaces/order.go`)と同じにし、削除済みの扱いと保存の流れは `Rename`(`rename.go`)に合わせる: 生きているプロジェクトを `Rank` の順(同じなら作成の保存が早い順、さらに内部 ID の順。`port.LiveProjects` の並び)で並べる。動かすのは 1 件だけで、その rank を新しい隣どうしのあいだの値(`rank.Between`)にする。rank のないものがあれば先に埋める。端なら何も書かない。プロジェクトの rank の変更の行の書き方は、作成のときに rank を入れている `create.go` に合わせる。`make generate` のあと、api の `projects.ts` に `move` を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/projects/ ./internal/adapter/httpapi/projects/ && pnpm -C web test -- projects.ports`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal api web/packages/api web/packages/core/src/port/projects.ts
git commit -m "feat(projects): プロジェクトの「上へ移動」「下へ移動」の API を足す(E3a Task 1)"
```

---

### Task 2: ツリーの部品を行の型で使えるようにし、Epic の Alt+Shift+↑↓ をプロジェクトの行に効かせる

**Files:**
- Modify: `web/packages/core/src/features/items/tree.ts`、`treeFocus.ts`、`siblings.ts`、`treeMove.ts`、`treeWrites.ts`、`treeKeys.ts`、`progress.ts`、`web/packages/core/src/index.ts`
- Test: `treeFocus.test.ts`、`treeMove.test.ts`、`treeWrites.test.ts`、`treeKeys.test.ts`、`progress.test.ts`

**Interfaces:**
- Produces:
  - `interface TreeRowLike { id: string; parentId: string | null }`
  - `interface TreeNodeOf<T extends TreeRowLike> { item: T; children: TreeNodeOf<T>[] }`、`type TreeNode = TreeNodeOf<Item>`(いまの名前のまま)
  - `createTreeFocus<T extends TreeRowLike = Item>(collapsed?: Iterable<string>): TreeFocus<T>`(`TreeFocusState<T>.rows: readonly T[]`。引数は初めに閉じている行)
  - `visibleRows<T extends TreeRowLike>(tree: readonly TreeNodeOf<T>[], collapsed): T[]`
  - `reparentTarget(rows: readonly TreeRowLike[], id, dir, candidates)` — 候補はアイテムとプロジェクトの両方を、行の `id` で照らす
  - `TreeScreen` に `order?: readonly TreeRowLike[]`(Alt+Shift+↑↓ でたどる見えている行。なければ `rows`)
  - `TreeWritesDeps.setStatus(item: Item, statusId: string): Promise<void>`(行のアイテムを渡す。Store は `item.storeId`)
  - `projectKeyAction(event: KeyInput): TreeKeyAction | null`(E3a-16 の分だけ。C は `"create-sibling"` を返し、実行する側がプロジェクトの行では「その下に Epic」と読む)
  - `projectProgress(items: Item[], settings: WorkspaceSettings): { done: number; total: number }`(E3a-12)

- [ ] **Step 1: 失敗するテストを書く**

- `treeFocus.test.ts` `works with any row type`: `{id, parentId, label}` の行の木で ↓・→・← がアイテムと同じに動く。`starts with given collapsed`: `createTreeFocus(["a"])` で a の子が `rows` にない。
- `treeMove.test.ts` `finds a project row for an Epic`: 見えている行が `[P1, E1, P2, E2]`(P はプロジェクトの行で `parentId: null`)、E1 の下へで、候補 `{kind: "project", id: "P2"}` → P2。`project candidates without project rows find nothing`: 行が Item だけなら null(プロジェクトの画面のいまの動き)。
- `treeWrites.test.ts` `changeParent moves an Epic under a visible project`: `order` に P2 を含む画面で Epic の `changeParent(…, "down")` → `moveTo(epic, P2 の候補)`。`uses order when given`: `rows` と違う `order` を渡すとそちらをたどる。既存の「Epic は何もしない」テストは「プロジェクトの行がなければ何もしない」に書き換える。`setStatus` を呼ぶテストは、第 1 引数が行のアイテムになる。
- `treeKeys.test.ts` `projectKeyAction`: ↑↓→←・Enter・Ctrl+Enter・C・Alt+↑↓・ContextMenu・Shift+F10 はアイテムと同じ値、Space・S・K・Alt+Shift+↑ は null。
- `progress.test.ts` `projectProgress`: Epic・Step・中止を除き、完了 1・未着手 1・中止 1・Step 完了 1 → `{done: 1, total: 2}`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- packages/core/src/features/items`
Expected: FAIL(`projectKeyAction` などがない)

- [ ] **Step 3: 型を広げ、`treeWrites` の Epic の早期の return をなくす**

`reparentTarget` は `inside` の親のたどりも `TreeRowLike` で行う。`treeWrites.parentStep` は `s.order ?? s.rows` をたどる。`setStatus` の呼び出し元(`app/src/screens/project/treeKeyboard.ts`)は `(item, status) => projectScreen.setStatus(item.id, status)` に合わせる。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core web/app/src/screens/project/treeKeyboard.ts
git commit -m "refactor(core): ツリーの部品を行の型で使えるようにし、Epic の Alt+Shift+↑↓ をプロジェクトの行に効かせる(E3a Task 2)"
```

---

### Task 3: 検索の画面の状態にレイアウトとツリーの開閉を持つ

**Files:**
- Create: `web/packages/core/src/features/search/layout.ts`、`layout.test.ts`
- Modify: `web/packages/core/src/features/search/search.ts`(状態の型と読み書きを `layout.ts` の関数に寄せ、行数を 200 行未満に保つ)、`search.test.ts`、`web/packages/core/src/index.ts`

**Interfaces:**
- Produces(`layout.ts`):
  - `type SearchLayout = "tree" | "table"`
  - `const layoutChoices: { id: SearchLayout; label: string }[]` — `[{tree, "ツリー"}, {table, "テーブル"}]`(E3a-1)
  - `interface TreeSettings { collapsed: string[] }`
  - `function readSearchState(raw: unknown): { query: string; layout: SearchLayout; table: TableSettings; tree: TreeSettings }`(E3a-2)
- Produces(`Search` に足す):
  - `SearchState` に `layout: SearchLayout; tree: TreeSettings`
  - `setLayout(layout: SearchLayout): void` — 書くだけで検索しない。テーブル以外にするときは `deps.selection?.clear()`(E3a-10。`SearchDeps.selection` の型を `Pick<RowSelection, "prune" | "clear">` にする)
  - `showTable(): void` — E3a-9 の入口が呼ぶ。そのブラウザのタブでまだ `open` が画面の状態を読んでいなければ、書かずに印だけを付け、`open` が読んだあとで `layout` を `table` にして書く(読む前に書くと、保存した絞り込み・列・開閉が初めの値で上書きされる)。読んだあとなら `setLayout("table")`
  - `setCollapsed(collapsed: string[]): void` — 書くだけ
  - `replaceQuery`・`prepareQuery` は、絞り込みを入れ替える前に `showTable()` を行う(パレットの「検索で開く」「{N} 件をまとめて変更…」の入口)
  - E3a-22(既存の不具合の直し。ユーザーの決定、2026-11-15): `prepareQuery` も、そのタブでまだ画面の状態を読んでいなければ書かない。絞り込みを印として持ち、`open` が読んだあとで、読んだ値の `query` だけを置き換え、`layout` を `table` にして書く(列・グループ・並べ替え・開閉は読んだ値のまま。04 の 359 行「レイアウトの設定と列は変えない」)。`open` が読んでいるあいだに `prepareQuery` が来たときも同じ(読み終えたところで置き換える)。`settled` と `cancelPrepare` の働きは変えない

- [ ] **Step 1: 失敗するテストを書く**

`layout.test.ts` `readSearchState`: `{query: "a"}` → layout `table`・collapsed `[]`。`{layout: "kanban"}` → `table`。`{tree: {collapsed: ["s/1", 3]}}` → `[]`(形が合わない)。`{layout: "tree", tree: {collapsed: ["s/1"]}}` → そのまま。

`search.test.ts`:
- `setLayout writes the state without searching`: `setLayout("tree")` → 書いた値が `{query, layout: "tree", table, tree}`、`port.search` の呼び出しが増えない。
- `setLayout away from table clears the selection`。
- `showTable switches to table`(Review Focus 5): `open` で `layout: "tree"` を読んだあと `showTable()` → `layout` が `table`。
- `showTable before the first open keeps the saved state`: 保存が `{query: "kind:bug", layout: "tree", table: {columns: ["id","title"], …}, tree: {collapsed: ["s/1"]}}` で、`open` の前に `showTable()` → `open` のあとの状態と書いた値が `query: "kind:bug"`・`layout: "table"`・その `columns`・その `collapsed`。`open` の前には `screenStates.write` が呼ばれない。
- `replaceQuery opens in table`: `layout: "tree"` の状態で `replaceQuery("kind:bug")` → `table`。
- `prepareQuery before the first open keeps the saved table`(E3a-22): 保存が `{query: "a", layout: "tree", table: {columns: ["id","title","points"], group: "kind", sort: {key: "id", desc: true}}, tree: {collapsed: ["s/1"]}}` で、`open` の前に `prepareQuery("kind:bug")` → `open` のあとの状態と書いた値が `query: "kind:bug"`・`layout: "table"`・その `table`・その `collapsed`。`open` の前には `screenStates.write` が呼ばれない。検索は 1 回だけで、`kind:bug` で行う。
- `open keeps the saved layout`: 保存が `tree` なら `open` のあと `tree`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/search`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 検索の画面の状態にレイアウトとツリーの開閉を持つ(E3a Task 3)"
```

---

### Task 4: 検索のツリーを組み立てる(core)

**Files:**
- Create: `web/packages/core/src/features/search/searchTree.ts`、`searchTree.test.ts`、`searchTreeData.ts`、`searchTreeData.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 2 の `TreeNodeOf`・`TreeRowLike`・`TreeScreen`・`projectProgress`、`buildTree`、`catalog` の `StoreNode`、`SearchRow`
- Produces(`searchTree.ts`):
  - `type SearchTreeRow = { id: string /* 鍵 */; parentId: string | null; kind: "project"; store: StoreNode; workspace: WorkspaceNode; project: ProjectNode } | { id: string; parentId: string | null; kind: "item"; store: StoreNode; workspace: WorkspaceNode; item: Item; dim: boolean }`
  - `interface ProjectData { items: Item[]; settings: WorkspaceSettings; members: Member[] }`
  - `interface SearchTreeSection { store: StoreNode; workspace: WorkspaceNode; projects: { project: ProjectNode; data: ProjectData; node: TreeNodeOf<SearchTreeRow> /* プロジェクトの行と、その下 */ }[] }`
  - `interface SearchTreeView { sections: SearchTreeSection[]; nodes: TreeNodeOf<SearchTreeRow>[] /* フォーカスに渡す。プロジェクトの行が根 */; matched: number /* E3a-6 */; hasProjects: boolean }`
  - `const rowKey = (storeId: string, id: string) => \`${storeId}/${id}\``
  - `function treeScope(stores: StoreNode[], rows: SearchRow[], filtered: boolean): { storeId: string; projectId: string }[]`(E3a-3。並びは E3a-4)
  - `function buildSearchTree(stores: StoreNode[], rows: SearchRow[], filtered: boolean, data: (storeId: string, projectId: string) => ProjectData | undefined): SearchTreeView`(データのないプロジェクトは出さない)
  - `function screenFor(rows: readonly SearchTreeRow[], key: string, data: (storeId: string, projectId: string) => ProjectData | undefined): TreeScreen | null` — `rows` は見えている行。フォーカスのある行と同じ Store・ワークスペースの見えている行から、`rows` はアイテムの行の `Item`、`order` はプロジェクトの行を `{id: projectId, parentId: null}`、アイテムの行を `{id: item.id, parentId: item.parentId}` にしたもの。`readOnly` は Store、`settings` は行のプロジェクトのデータ
- Produces(`searchTreeData.ts`):
  - `interface SearchTreeData { get(storeId: string, projectId: string): ProjectData | undefined; load(scope: { storeId: string; projectId: string }[]): Promise<void> /* まだないものだけ読む */; reset(): void }`
  - `function createSearchTreeData(deps: { catalog: Catalog; items: Pick<ItemsPort, "list">; workspaces: Pick<WorkspacesPort, "get">; members(storeId: string): Promise<Member[]> }): SearchTreeData` — ワークスペースの設定はワークスペースごと、メンバーは Store ごとに 1 回だけ読む。`reset` のあとに飛んでいた読み込みの結果は捨てる。読めなかったプロジェクトは入れない(次の `load` で読み直す)

- [ ] **Step 1: 失敗するテストを書く**

`searchTree.test.ts`(材料は `features/search/fixtures.test-util.ts` と `features/items/fixtures.test-util.ts` を使い、足りない助けはこのテストに書く):
- `unfiltered shows every project in sidebar order`: 2 つの Store・各 2 プロジェクト(1 つは空)で `filtered: false` → プロジェクトの行が 4 つ、Store・ワークスペース・プロジェクトの並びが catalog の順、空のプロジェクトの行もある、薄い行がない。
- `filtered keeps matches and their ancestors`: Epic E・Story S・Task T(S の子)・Task U(E の子)で T だけ一致 → 行は P・E(薄い)・S(薄い)・T、U は出ない。
- `filtered hides projects without matches` と `heading only for workspaces with projects`(E3a-4)。
- `ignores sort order`: 応答の行の順が逆でも、木は兄弟の並び。
- `keys are per store`(Review Focus 1): 2 つの Store に同じ内部 ID のアイテム → 鍵が `work/x`・`team/x` で別の行。
- `matched counts only drawn matches`: 親がデータにない一致した行は数えない、薄い行は数えない。
- `progress counts unmatched children`(Review Focus 4): プロジェクトの行の `data.items` は絞り込まない全件(`projectProgress` が絞り込まないときと同じ値)。
- `screenFor uses the row's store`(Review Focus 3): 閲覧のみの Store の行の鍵 → `readOnly: true`、書ける Store の行の鍵 → `false`、`settings` はその行のワークスペースのもの。
- `rowsFor gives only visible siblings`(Review Focus 2): HH-3・HH-5・HH-9 だけが見えているとき、`screenFor(…).rows` に HH-4・HH-7 はない。
- `order includes project rows of the same workspace only`。
- `treeScope` の 2 つ(filtered なら一致したプロジェクトだけ、そうでなければすべて)。

`searchTreeData.test.ts`: `loads each project once`、`settings once per workspace, members once per store`、`reset drops in-flight results`、`failed project is retried on next load`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/search/searchTree`
Expected: FAIL

- [ ] **Step 3: 書く**

`filtered` は、応答の元になった絞り込み(`SearchState.resultQuery`)が空でないかで決める(入力中の `query` で決めると、刈り込む行と食い違う)。木は `buildTree(data.items)` を刈り込む。残すのは、一致したアイテムか、一致したアイテムを子孫に持つアイテム。一致の集まりは `rows` の `storeId`・`itemId`(削除済みの行は入れない)。`filtered: false` なら刈り込まない。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 検索のツリーを、一致した行と祖先で組み立てる(E3a Task 4)"
```

---

### Task 5: 検索の応答に `hasSort` を足し、注記の文を作る

**Files:**
- Modify: `internal/app/search/search.go`(`Result.HasSort bool`)、`internal/app/search/search_test.go`、`internal/adapter/httpapi/search/search.go`(応答に `hasSort`)、`internal/adapter/httpapi/search/search_test.go`、生成物、`web/packages/api/src/search.ts`、`web/packages/core/src/features/search/types.ts`(`SearchResult.hasSort: boolean`)、`web/packages/core/src/features/search/layout.ts`、`layout.test.ts`、`fixtures.test-util.ts`

**Interfaces:**
- Produces: `function layoutNote(layout: SearchLayout, result: SearchResult | null): string | null`(E3a-8。ツリーのときだけ文を返す)、`const deletedTreeText = "削除済みのアイテムはテーブルで表示します。"`

- [ ] **Step 1: 失敗するテストを書く**

Go: `TestSearchHasSort`: `sort:id` → 真、`kind:bug` → 偽、`-sort:id` → 真(`Sort` が入る語)、エラーのあるクエリでも `sort:` があれば真。
TS `layoutNote`: ツリーで `hasSort` だけ → 「ツリー・ガントでは sort: は使われず、並び順のままです」、`group: "kind"` だけ →「ツリー・ガントでは group: は使われず、並び順のままです」、両方 →「ツリー・ガントでは sort: と group: は使われず、並び順のままです」、テーブルなら null、`result` が null なら null。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/app/search/ ./internal/adapter/httpapi/search/ -run HasSort; pnpm -C web test -- features/search/layout`
Expected: FAIL

- [ ] **Step 3: 書く(`HasSort` は `q.Sort != nil`。エラーのときも解析の結果から入れる)し、`make generate`**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/... && pnpm -C web test && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal api web/packages
git commit -m "feat(search): 応答に sort: の有無を足し、ツリーの注記の文を作る(E3a Task 5)"
```

---

### Task 6: ツリーのキー操作をツリーの部品にし、行の型で使えるようにする

**Files:**
- Create: `web/app/src/screens/tree/treeKeyboard.ts`(`screens/project/treeKeyboard.ts` から移す)、`web/app/src/screens/tree/treeTabStops.ts`(移す)
- Delete: `web/app/src/screens/project/treeKeyboard.ts`、`web/app/src/screens/project/treeTabStops.ts`
- Modify: `web/app/src/screens/project/ProjectTree.svelte`

**Interfaces:**
- Consumes: Task 2 の `TreeFocus<T>`・`projectKeyAction`・`TreeScreen`
- Produces:
  - `createTreeKeyboard<T extends TreeRowLike>(deps: TreeKeyboardDeps<T>): TreeKeyboard<T>`(`TreeKeyboard` の中身はいまのまま。`focus: TreeFocus<T>`)
  - `interface TreeKeyboardDeps<T> { focus: TreeFocus<T>; list(): HTMLElement | undefined; itemOf(row: T): Item | null /* プロジェクトの行は null */; screen(): TreeScreen | null /* フォーカスのある行で書く画面 */; readOnly(row: T): boolean; open(row: T): void /* Enter */; openTab(row: T): void /* Ctrl+Enter */; openMenu(row: T, rect: { left: number; bottom: number }): void; create(row: T): void /* C */; setStatus(item: Item, statusId: string): Promise<void>; moveProject?(row: T, dir: "up" | "down"): void /* プロジェクトの行の Alt+↑↓ */; afterChange(): Promise<void> }`
  - アイテムの行は `treeKeyAction(event, item)`、プロジェクトの行は `projectKeyAction(event)` でキーを読む。プロジェクトの行の Alt+↑↓ は順番待ちに積まず `moveProject` を呼ぶ。閲覧のみの行の C と Alt+↑↓ は何もしない(E3a-16)。先頭・末尾のプロジェクトの Alt+↑↓ も何もしない(呼ぶ側の `moveProject` が `canMoveProject` で確かめる)
  - 鍵と内部 ID の境目: `createTreeWrites` には内部 ID を渡す(`toggleDone(itemOf(row).id)`・`moveSibling`・`changeParent` も `itemOf(row).id`)。`TreeScreen.rows` は `Item` なので、書き込みの側は内部 ID で行を探す。`TreeWritesDeps.expand(id)` に来るのは候補の内部 ID なので、`expand: (id) => focus.expand(deps.keyOf(id))` で鍵に戻す。そのため deps に `keyOf(internalId: string): string` を足す(プロジェクトの画面は `(id) => id`、検索のツリーはフォーカスのある行の Store で `rowKey(storeId, id)`)

- [ ] **Step 1: いまのテストで守られていることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/tree-keys.spec.ts e2e/tree-readonly.spec.ts e2e/project.spec.ts`
Expected: PASS(移す前の動き)

- [ ] **Step 2: 移して、`ProjectTree.svelte` を新しい形でつなぐ**

`ProjectTree.svelte` の `deps`: `focus = createTreeFocus<Item>()`、`itemOf: (it) => it`、`screen: () => ({storeId: view.store.id, readOnly: view.store.readOnly, settings: view.settings, rows: get(focus.state).rows})`、`readOnly: () => view.store.readOnly`、`open: (it) => detail.open(view.store.id, it.id)`、`openTab: (it) => links.openTab(route(it))`、`create`: いまの `create-sibling` と同じ、`setStatus: (it, s) => projectScreen.setStatus(it.id, s)`、`afterChange: () => itemRefresh.afterChange()`。`moveProject` は渡さない。

- [ ] **Step 3: 流して確かめる**

Run: `pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/tree-keys.spec.ts e2e/tree-readonly.spec.ts e2e/project.spec.ts`
Expected: PASS(Step 1 と同じ spec)

- [ ] **Step 4: コミット**

```bash
git add web/app
git commit -m "refactor(web): ツリーのキー操作をツリーの部品に移し、行の型で使えるようにする(E3a Task 6)"
```

---

### Task 7: プロジェクトの行・プロジェクトのメニュー・「名前を変更…」のダイアログ

**Files:**
- Create: `web/packages/core/src/features/projects/projectMenu.ts`、`projectMenu.test.ts`、`web/app/src/screens/tree/ProjectRow.svelte`、`web/app/src/screens/tree/ProjectMenu.svelte`、`web/app/src/screens/tree/RenameProjectDialog.svelte`、`web/app/src/projectRows.ts`(組み立て。`wire.ts` は満杯)
- Modify: `web/app/src/App.svelte`(`RenameProjectDialog` を置く)、`web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: Task 1 の `ProjectsPort.move`、Task 2 の `projectProgress`、`projectEditor.rename`、`projectDelete.open`、`itemDialog.open`
- Produces(core `projectMenu.ts`):
  - `type ProjectMenuId = "open" | "add-epic" | "up" | "down" | "rename" | "delete"`
  - `function projectMenu(store: StoreNode, workspace: WorkspaceNode, projectId: string): { id: ProjectMenuId; label: string; disabled: boolean; hint?: string }[]`(E3a-13。先頭・末尾の無効の理由の文は付けない。サイドバーのワークスペースのメニューと同じ)
  - `function canMoveProject(workspace: WorkspaceNode, projectId: string, dir: Direction): boolean`
  - `interface ProjectMover { move(storeId: string, projectId: string, dir: Direction): Promise<void> }`、`createProjectMover({ catalog, port: Pick<ProjectsPort, "move">, toasts })` — 断りはエラーのトースト、そのあと `catalog.load()`(`createWorkspaceMover` と同じ)
  - `interface RenameProjectDialog { state: Readable<{ project: Project; error: string | null } | null>; open(project: Project): void; save(title: string): Promise<void>; close(): void }`(E3a-14)
- Produces(app): `ProjectRow.svelte` の props `{ project: ProjectNode; store: StoreNode; progress: { done: number; total: number }; members: Member[]; open: boolean; hasChildren: boolean; focused: boolean; rowKey: string; ontoggle(): void; onopen(e: MouseEvent): void; href: string; onadd(): void; onmenu(e: MouseEvent): void }`、`projectRows.ts` の `projectMover`・`renameProject`・`openProjectMenu(store, workspace, project, rect)`

- [ ] **Step 1: 失敗するテストを書く**

`projectMenu.test.ts`: 項目の並びと文言(E3a-13)、閲覧のみなら「開く」以外が無効で `hint` が `readOnlyNote`、3 件の先頭は「上へ移動」が無効、末尾は「下へ移動」が無効、1 件だけなら両方無効。`ProjectMover` は断り(Refused)をエラーのトーストにして `catalog.load` を呼ぶ。`RenameProjectDialog`: 断られたら `error` にその文が入り開いたまま、通れば閉じる、`close` で閉じる。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/projects/projectMenu`
Expected: FAIL

- [ ] **Step 3: core と部品を書く**

`ProjectRow.svelte` は `TreeRow.svelte` と同じ見た目の段(開閉・Store の点・アイコン・タイトル・ステータス・オーナー・進捗・「+」・「⋯」)にする。`data-row={rowKey}`、`aria-current`、「+」の読み上げの名前は「Epic を追加」、閲覧のみでは「+」を無効にして `readOnlyNote` を添える。ステータスの点は `projectCategory`(サイドバーのプロジェクトの点と同じ)。進捗は ui の `ProgressBar`。メニューの「開く」は概要へ移る、「Epic を追加」は `itemDialog.open({workspaceId, projectId, kind: "Epic", parentId: null})`、「名前を変更…」は `renameProject.open`、「削除…」は `projectDelete.open`、「上へ移動」「下へ移動」は `projectMover.move`。ダイアログは ui の `Dialog`・`Input`・`Button`。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): ツリーのプロジェクトの行・プロジェクトのメニュー・名前の変更のダイアログを作る(E3a Task 7)"
```

---

### Task 8: 検索の画面のツリー

**Files:**
- Create: `web/app/src/screens/tree/SearchTree.svelte`、`web/app/src/screens/tree/SearchTreeBody.svelte`(ワークスペースの見出しとプロジェクトの段を描く)、`web/app/src/screens/search/SearchBody.svelte`(テーブル・ツリー・空のどれを出すか。`SearchScreen.svelte` から本体を移す)、`web/app/src/searchTree.ts`(`createSearchTreeData` の組み立て)、`web/e2e/search-tree.spec.ts`
- Modify: `web/app/src/screens/search/SearchScreen.svelte`(200 行未満にする)、`web/app/src/screens/project/TreeRow.svelte`(`rowKey?: (it: Item) => string`(初めは `it.id`)・`dim?: ReadonlySet<string>`・`storeColor?: string` を足す)、`web/app/src/search.ts`(Store が変わった知らせの処理で、`searchTreeData.reset()` をしてから `search.reload()` する。1 つの処理の中でこの順にする)、`web/e2e/helpers.ts`(`resetSearchScreen` が `layout`・`tree` も初めに戻す。このタスクの e2e は画面の状態に `layout: "tree"` を書いてから開く)

**Interfaces:**
- Consumes: Task 3 の `setCollapsed`、Task 4 の `buildSearchTree`・`treeScope`・`screenFor`・`rowKey`、Task 6 の `createTreeKeyboard`、Task 7 の `ProjectRow`・`openProjectMenu`・`projectMover`・`canMoveProject`

- [ ] **Step 1: 失敗するテストを書く**

`search-tree.spec.ts`(各テストの前に `resetSearchScreen`、そのあと画面の状態を `layout: "tree"` にする。サンプルデータに足りない材料(2 つ目の Store の同じ ID、閲覧のみの Store)は、既存の e2e の助け(`tree-readonly.spec.ts`・`sync.spec.ts` の Store の作り方)を使う):
- `draws matches with dim ancestors`: `kind:task` → 一致しない祖先の行が薄い、一致しないプロジェクトの行がない、件数が一致した行の数。
- `collapse survives reload`: Epic を閉じて再読み込み → 閉じたまま。
- `same id in two stores`(Review Focus 1)。
- `move up among visible siblings`(Review Focus 2)。
- `space and alt-up on item rows`: アイテムの行で Space → 完了の既定のステータスになる、Alt+↑ → 見えている兄弟の前へ動き、フォーカスがその行に残る。
- `read-only row next to writable row`(Review Focus 3): 閲覧のみの Store の Epic で C を押してもダイアログが開かず、書ける Store の Epic の C で開く。
- `project row keys and menu`: プロジェクトの行で Enter → 概要、Alt+↓ → サイドバーの並びも変わる、末尾のプロジェクトの Alt+↓ は何もしない、メニューの「名前を変更…」でタイトルが変わる、Epic の Alt+Shift+↓ で次のプロジェクトへ移る。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/search-tree.spec.ts`
Expected: FAIL(ツリーがない)

- [ ] **Step 3: ツリーを組む**

- `SearchBody.svelte`: レイアウトが `tree` なら `SearchTree`、空なら E3a-19 の文。そうでなければ今のテーブルと空の文。
- `SearchTree.svelte`: 結果が変わるたびに `treeScope` で `searchTreeData.load` を呼び、読めたら `buildSearchTree` を作る(`filtered` は Task 4 のとおり `resultQuery` で決める)。`createTreeFocus<SearchTreeRow>(state の collapsed)` を作る。`focus.state` の `collapsed` が変わったら `search.setCollapsed([...collapsed])`。`createTreeKeyboard` の deps は次のとおり。
  - `itemOf`: アイテムの行なら `item`。
  - `screen`: `screenFor(rows, currentId, data.get)`。
  - `keyOf`: フォーカスのある行の Store で `rowKey`。
  - `readOnly`: 行の Store が閲覧のみか。
  - `open`: アイテムの行は `detail.open(storeId, item.id)`、プロジェクトの行は概要へ移る。
  - `openTab`: 新しいタブで開く。
  - `openMenu`: アイテムの行は `itemMenuHost.open(item, {rows: screenFor(...).rows, children, row: true}, x, y)`、プロジェクトの行は `openProjectMenu`。
  - `create`: アイテムの行は同じ親の兄弟、プロジェクトの行は Epic。
  - `setStatus`: `setRowStatus` で、あと `search.reload()`。
  - `moveProject`: `canMoveProject` が真のときだけ `projectMover.move`。
  - `afterChange`: `search.reload()` と `itemRefresh.afterChange()`。
  
  `selection.provide(1, …)` は E3a-20。`<ul aria-label="ツリー">`。
- `SearchTreeBody.svelte`: ワークスペースの見出し(ui の `StoreBadge` とワークスペース名。フォーカスの対象にしない)、`ProjectRow`、その下に `TreeRow`(`rowKey`・`dim`・`storeColor`。`prefix`・`settings`・`members`・`items` はそのプロジェクトのデータ)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/search-tree.spec.ts e2e/search.spec.ts e2e/search-table.spec.ts e2e/search-table-select.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 検索の画面のツリーを作る(E3a Task 8)"
```

---

### Task 9: レイアウトの切り替え・注記・削除済みの案内・テーブルで開く入口

**Files:**
- Create: `web/packages/ui/src/patterns/LayoutSwitch.svelte`(と `patterns.test.ts` へのテスト)、`web/app/src/screens/search/SearchToolbar.svelte`、`web/e2e/search-layout.spec.ts`
- Modify: `web/app/src/screens/search/SearchScreen.svelte`、`web/app/src/screens/search/SearchBody.svelte`(削除済みの案内)、`web/app/src/screens/table/TableToolbar.svelte`(左端に部品を置く `lead?: Snippet` を足す)、`web/app/src/shell/Sidebar.svelte`(「検索」を押したら `search.showTable()`)、`web/packages/core/src/features/palette/actions.ts`(コマンド「検索を開く」の前に `deps.showSearchTable()`)、`web/app/src/palette.ts`(`showSearchTable: () => search.showTable()` を渡す)、パレットの deps の型と `palette.test-util.ts`

**Interfaces:**
- Consumes: Task 3 の `layoutChoices`・`setLayout`・`showTable`、Task 5 の `layoutNote`・`deletedTreeText`
- Produces: ui `LayoutSwitch.svelte` の props `{ choices: { id: string; label: string }[]; value: string; onchange(id: string): void }`(`role="group"`・`aria-label="レイアウト"`、ボタンは `aria-pressed`)

- [ ] **Step 1: 失敗するテストを書く**

ui `patterns.test.ts` `LayoutSwitch`: 2 つのボタン、選んでいるものが `aria-pressed="true"`、押すと `onchange` に id。パレットの `actions.test.ts` `search command shows table first`: 「検索を開く」を選ぶと `showSearchTable` のあとに移る。

`search-layout.spec.ts`(各テストの前に `resetSearchScreen`):
- `switches layouts and keeps the filter`: `kind:task` でツリーに切り替え → 欄は `kind:task` のまま。テーブルに戻すと列の設定が前のまま。テーブルで選んでいた行の選択バーが、ツリーに切り替えると消える。
- `reload keeps tree, sidebar opens table`(Review Focus 5)。新しいブラウザのタブでサイドバーの「検索」を押しても、保存した絞り込みと列が残る。
- `notes for sort and group`: ツリーで `sort:id group:kind` → 「ツリー・ガントでは sort: と group: は使われず、並び順のままです」。テーブルでは出ない。
- `deleted shows guidance`: `is:deleted` でツリー → 案内と「テーブルで開く」、件数「0 件」。押すとテーブルになり、欄は `is:deleted` のまま。
- `palette opens in table`: ツリーにしてからパレットの「このクエリで」の「検索で開く」→ テーブル。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- LayoutSwitch actions && make e2e-bin && pnpm -C web exec playwright test e2e/search-layout.spec.ts`
Expected: FAIL

- [ ] **Step 3: 画面を組む**

- ツールバー(`SearchToolbar.svelte`): `LayoutSwitch` を左端に置く。テーブルなら `TableToolbar`(その `lead` に置く)、ツリーなら件数「{N} 件」(`matched`。削除済みの案内のときは 0)だけ。
- 絞り込み欄の下: エラー(いまのまま)と、`layoutNote` の灰色の文。
- 本体(`SearchBody.svelte`): ツリーで `deleted` のときは `deletedTreeText` とボタン「テーブルで開く」(`search.setLayout("table")`)。
- サイドバーの「検索」の `onclick` は、`links.click` の前に `search.showTable()` を呼ぶ。新しいタブで開くクリックでは呼ばない(修飾キーなしの主ボタンのときだけ)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web lint && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/search-layout.spec.ts e2e/search-tree.spec.ts e2e/search.spec.ts e2e/search-table.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 検索の画面にレイアウトの切り替え・注記・削除済みの案内を足す(E3a Task 9)"
```

---

### Task 10: 文書と、すべての確かめ

**Files:**
- Modify: `docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`、`docs/superpowers/plans/2026-11-15-hedgehug-track-e1.md`(末尾「E3(保存したビュー)」の各行に「E3a で足した」「E3b に回した」を添える)、`docs/superpowers/plans/2026-11-13-hedgehug-track-a4b.md`(末尾の E への申し送りの各行に「E3a で足した」「E7 に回した」を添える)、この計画の末尾「E3b・E4〜E7 に渡すもの」

- [ ] **Step 1: 文書を直す**

- トラック E の分け方: 決めたことの表に「E-7 E3 は 2 本に分ける(E3a・E3b)。切り替えには作ってあるレイアウトだけを出し、カンバン・ガントのボタンは E5・E6 が足す。ユーザーの決定(2026-11-15)」を足す。小計画の一覧の E3 の行を E3a(この計画の Goal の中身)と E3b(「始める前に」の E3b の中身、待つものは E3a)の 2 行にし、E4〜E7 の待つものを「E3a」にする(E3b は E4〜E7 をブロックしない)。E7 の行の中身から、E3a で作ったもの(プロジェクトの行・プロジェクトのメニュー・プロジェクトの行のキー・Epic の Alt+Shift+↑↓・絞り込んだときの刈り込みの部品)を外し、「ワークスペース・プロジェクトのツリーのタブの絞り込み欄(E3a の `buildSearchTree` の刈り込みと同じ規則)、ワークスペースの画面の「ツリー」タブ(E3a の `ProjectRow` を使う)、ツリーのタブの開閉を画面の状態に残すこと(E3a-21)、ドラッグ」にする。E4 の行の中身に「ワークスペースの概要のプロジェクトの一覧の行のプロジェクトのメニュー(04 の 84 行。E3a の `openProjectMenu` を使う)」を足す。「ほかのトラックから回された項目の行き先」の A5c の「ビューとして保存」「ビューの群」と B5b の「保存したビュー」を E3b にする。
- この計画の末尾「E3b・E4〜E7 に渡すもの」を、実装で変えたことに合わせて直す。

- [ ] **Step 2: すべてを確かめる**

Run: `make check && make e2e-bin && pnpm -C web exec playwright test e2e/search-tree.spec.ts e2e/search-layout.spec.ts e2e/search.spec.ts e2e/search-table.spec.ts e2e/search-table-select.spec.ts`
Expected: PASS

- [ ] **Step 3: コミット**

```bash
git add docs
git commit -m "docs: トラック E の分け方を E3a・E3b に書き換え、E7 との境目を書く(E3a Task 10)"
```

## E3b・E4〜E7 に渡すもの

### E3b(保存したビュー)

- 画面の状態は `{query, layout, table, tree}`(E3a-2)。ビューが持つもの(04 の 366〜375 行)は、このうち `query`・`layout`・`table` と、E5・E6 が足すカンバン・ガントの設定。`tree.collapsed` は「未保存の変更」の比べる対象に入れない(04 の 418 行)。
- ビューの画面は、検索の画面の部品(`SearchToolbar`・`SearchBody`・`LayoutSwitch`・`SearchTree`)を使う。検索の流れ(core の `createSearch`)は画面の状態の鍵 `search` に決め打ちなので、鍵を引数にとる形にする。
- 「このクエリで」の群(`queryGroup.ts`)の「検索で開く」と「{N} 件をまとめて変更…」のあいだに「ビューとして保存…」を足す。テーブルで開く入口なので `search.showTable()` を通る(E3a-9)。

### E4(マイタスクと概要)

- ワークスペースの概要のプロジェクトの一覧の行にも、プロジェクトのメニューを付ける(04 の 84 行)。`projectRows.ts` の `openProjectMenu` と、core の `projectMenu`・`canMoveProject` を使う。

### E5(カンバン)・E6(ガント)

- `layoutChoices` と `SearchLayout` に値を足し、04 の並び(ツリー・カンバン・テーブル・ガント)の位置に置く。画面の状態の読み方(`readSearchState`)に、そのレイアウトの設定の読み方を足す。
- `is:deleted` の案内(`deletedTreeText`・「テーブルで開く」)は、カンバン・ガントでも同じものを出す(04 の 314 行)。
- 注記は `layoutNote`。ガントはツリーと同じ文、カンバンは 04 の 452 行の文を足す。サーバーの応答の `hasSort`・`group` を使い、画面で解析しない。

### E7(ドラッグとツリーのタブ)

- ワークスペースの画面の「ツリー」タブは `screens/tree/` の `ProjectRow`・`createTreeKeyboard`(`moveProject` を渡す)を使う。プロジェクトのメニューは `projectRows.ts` の `openProjectMenu`。
- ツリーのタブの絞り込みは、E3a の刈り込み(一致したアイテムと祖先、一致しない祖先は薄く、クエリがあるときは一致のないプロジェクトを隠す)と同じ規則にする。
- ツリーのタブの開閉を画面の状態に残す(E3a-21)。鍵は E2c が決めたものを使う。
