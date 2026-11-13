# トラック A4 後半(アイテムとプロジェクトの操作) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** アイテムの削除・同じワークスペースの中での移動・種類の変更・複製・上へ移動と下へ移動、詳細パネルとツリーの行のメニュー、ツリーのキー操作、プロジェクトの設定タブ(名前・削除)を、仕様どおりに保存し表示する。削除済みのものを一覧・件数・候補から除く。

**Architecture:** 削除は論理削除で、アイテム(プロジェクト)のログに `t: "delete"`・`f: "deleted"` の行を、下にあるものにも同じ `op` で書く。保存の口は削除済みも返し(`Deleted` の印つき)、除くのは app の側で `item.Live`・`project.Live` を通して行う(番号の採番と、プロジェクトが変わる移動は削除済みも数える・移すため)。新しい操作は `app/items`(14 ファイル)に 1 ファイル 1 操作で足し、プロジェクトの削除は `app/projects` に足す。画面は、メニューの項目と無効の理由・見えている兄弟・ツリーのフォーカスとキーの割り当てを core に置き、ダイアログと結び付けを app に置く。

**Tech Stack:** Go(huma・domain/app/infra/adapter の層)、TypeScript(core・api の orval)、Svelte 5(ui・app)、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(「アイテムの種類と親子」の断りの表、「複製」「同じワークスペースの中での移動」「種類の変更」「削除」、「詳細パネル」の冒頭・「上から順の構成」のメニューと上へ移動・下へ移動・「ブロック中」、「子と Step」の「上へ」と「Step の削除」、「プロジェクト」の「画面の見出しと構成」の設定タブの「名前」「削除」、「名前の変更・移動・削除」の削除、「操作ごとの記録と通知」)、`04-search-layouts.md`(「ツリー」の「行」のメニュー、「ツリーのキー操作」、「ドラッグの代わりの経路」)、`06-sync-notifications.md`(「同時に編集したとき」の場所と順位)、`03-workspace-settings.md`(名前の重複・接頭辞の件数で削除済みを除くこと)。保存形式は `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「変更記録」の `delete`・`root`、「削除・復元・完全に削除」、「移動・番号・旧 ID」の同じワークスペースの中)。

**前提として読むもの:** トラック A の計画 `2026-11-12-hedgehug-track-a.md` の A4 の行、A4 前半の計画 `2026-11-13-hedgehug-track-a4a.md` の「入れないもの」と末尾の「A4b・C4 への申し送り」、B2 の計画の B2-11、A2 の計画の `MoveUp`。

## Global Constraints

- 依存は追加しない(足すときは版を明記し、公開から 7 日以上の版だけ)。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル(テストを除いて数える)、Svelte 200 行、TS 300 行。超えたら分ける。上限の 20 にいる `internal/infra/gitstore`・`internal/app/workspaces`・`internal/app/apptest` には新しいファイルを足さず、既存のファイルに足す。`internal/app/items` は 14、`internal/app/projects` は 11。`web/app/src/wire.ts`・`web/packages/core/src/index.ts` は 300 行、`web/app/src/screens/project/TreeRow.svelte` は 199 行、`web/packages/core/src/features/items/createDialog.ts` は 299 行で、どれも 1 行も足せない(Task 9 が先に分ける)。
- 向き: adapter→app→domain、infra→app/port。app の機能どうしは import しない(テストも)。
- 画面の文言は仕様の文字どおり。モックの動きは写さない(見た目だけ参考: モック版 19)。
- 通知は作らない(複製の担当者・メンションの通知は C4 が行から作る)。A4b は行を正しく書くだけ。
- 閲覧のみの Store では、書く操作(メニューの「上へ移動」「下へ移動」「移動…」「種類を変更…」「複製」「テンプレートとして保存…」「削除…」、Step の「×」、設定タブの「名前」の入力と「プロジェクトを削除」)を無効にし「閲覧のみの Store です」を添える。「ID をコピー」は使える。キーの書き込みは何もしない。
- 変更の応答の `blocked` はいつも `false`(A4a の申し送り)。削除・移動・種類の変更・複製のあとは、応答の `Item` を画面に使わず、ツリーと詳細パネルを読み直す。
- 各タスクは TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクごとにレビューする。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR はトラックで 1 本(develop 向け、ブランチ `feat/track-a4b`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。
- 計画やテストにトークンらしい値(ghp_ など)を書かない。
- 確認のコマンド: Go は `go test ./internal/...`、全体は `make check`、画面は `pnpm -C web test`・`pnpm -C web check`、API の型は `make generate` のあと `pnpm -C web gen`、e2e は `pnpm -C web e2e`。e2e のポートは 4173 を他の worktree と取り合うので、流すときは別の番号にする(Task 20 に手順)。

---

## 持ち場と境目

| 入れるもの | 仕様 |
|---|---|
| アイテムの削除(確認ダイアログ・下にあるものと一緒の 1 回の削除・保持日数の補足・トースト・パネルを閉じる)、Step の一覧の「×」 | 02「削除」「子と Step」 |
| プロジェクトの削除(設定タブの「削除」の区画と確認ダイアログ・ワークスペースの概要へ移る) | 02「名前の変更・移動・削除」の削除、「画面の見出しと構成」 |
| プロジェクトの設定タブ(「名前」「削除」の 2 区画) | 02「画面の見出しと構成」 |
| 削除済みを一覧・件数・候補・関連・ブロック中・名前の重複から除く | 02「削除」、03「名前の重複」「接頭辞」「値の編集」、B2-11 |
| 同じワークスペースの中での移動(「移動…」の候補と検索・断りの文言・アクティビティ・トースト。プロジェクトが変わるときは下のものを削除済みも含めて移す) | 02「同じワークスペースの中での移動」「アイテムの種類と親子」 |
| 種類の変更(選べる種類・確認ダイアログ・Step からの昇格) | 02「種類の変更」 |
| 複製 | 02「複製」 |
| 上へ移動・下へ移動(動かした 1 件だけを書く。見えている兄弟の間)、Step の「上へ」 | 02「上から順の構成」「子と Step」、04「ツリー」の「行」 |
| 詳細パネルとツリーの行のアイテムのメニュー(無効の理由を添える) | 02「上から順の構成」、04「行」 |
| ツリーのキー操作(アイテムの行) | 04「ツリーのキー操作」 |
| 開いている詳細パネル・プロジェクトの画面のものが削除されたら閉じる・移る(同期や別のタブによるものも) | 02「詳細パネル」の冒頭、「名前の変更・移動・削除」の削除 |

| 入れないもの | 回す先 | 理由 |
|---|---|---|
| 移動の確認ダイアログの「ステータスの置き換え」 | B5 | 「プロジェクトが使わないステータス」はプロジェクト別のルール(B5)が入るまで起きない。B5 の計画は「プロジェクトの移動での引き継ぎは、トラック A の移動と同じ保存に入れる」としているので、B5 が移動の入力に置き換えを足す |
| 複製のスプリント・リリース | F | アイテムにスプリント・リリースの項目がまだない。F が項目を足すときに複製の規則(完了済み・アーカイブ済みは外す)も足す |
| メニューの「別のワークスペースへ移動…」 | A5 | A4a の決めごと「知らない種類の行は出さない」と同じく、まだない操作の項目は出さない。A5 が項目ごと足す |
| プロジェクトのメニュー(開く・Epic を追加・上へ移動・下へ移動・名前を変更…・削除…)と「名前を変更…」のダイアログ、プロジェクトの行のキー操作 | E | プロジェクトのメニューを出す行(ワークスペースのツリーのプロジェクトの行、概要の一覧の行)がまだない。プロジェクトの画面のツリーはプロジェクトの行を出さない(04)。名前の変更は、いつでも使える入口として設定タブの「名前」の区画に入れる |
| ツリーのドラッグ | E | トラック A の計画の決定 4 |
| ツリー・テーブルの絞り込み | E | A4b の「見えている兄弟」は、ツリーに見えている行の並びを受け取る関数にしておく。絞り込みがない今は、見えている兄弟 = 削除されていない全兄弟 |
| 設定の「削除済み」の一覧・復元・完全に削除、保持日数の変更、`restore` の行 | G | 08 の持ち場。A4b は保持日数を読むだけ |
| 別の画面(タブ)で削除されたアイテムを変更したときに削除を取り消す規則(06「このブラウザのタブの操作」の「…削除を取り消しました」)、同期で削除と変更が重なったときに変更を残す規則、削除済みの親の下の子を削除済みに入れる修復 | C | 同期と合わせ方の持ち場。A4b の読み方は「`delete` の行があれば削除済み」だけ |
| プロジェクトの設定タブの「使うステータス」「ワークフローの制約」「テンプレート」「自動化」の区画と見出しの下の 1 行 | B5 | プロジェクト別のルール |
| プロジェクトの設定タブの「別のワークスペースへ移動」の区画 | A5 | |
| 履歴の「スプリントを変更」「リリースを変更」「復元」「{旧 ID} から移動」 | F・G・A5 | A4a と同じ。A4b が書く `delete` の行は履歴に出さない(A4a-15 で除外済み) |

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| A4b-1 | 削除の行は `{"t":"delete","f":"deleted","root":true\|false}`(`from`・`to` なし)。利用者が選んだものに `root: true`、下にあるものに同じ `op` で `root: false`。作るのは `change.NewDeleteEntity(root bool) Draft`。すでに削除済みの下のもの(別の操作で消したもの)には書かない | 保存形式「変更記録」の `root`、「削除・復元・完全に削除」。`f` は `item.yaml` の `deleted` と同じ名前 |
| A4b-2 | 削除済みかは、ログに `t: delete`・`f: deleted` の行があるかで決める。`change.DeletionOf(log []Record) *Deletion`(`type Deletion struct { Op ID; At time.Time; Root bool }`。複数あれば `at` があとのもの)。`item.Item`・`project.Project` に `Deleted *change.Deletion` を足し、gitstore と偽物が埋める。`restore` の行の扱いは G が `DeletionOf` に足す | 保存形式「削除・復元・完全に削除」。復元は G |
| A4b-3 | 保存の口は削除済みも返す(`Get`・`List` とも)。除くのは app の側で、`item.Live(its []Item) []Item`・`project.Live(ps []Project) []Project` を通す。番号の採番(ワークスペースの `number` の行)はもとから削除済みも数える。プロジェクトが変わる移動は、削除済みの下のものも移す | 02「同じワークスペースの中での移動」(下のものと削除済みのものを含む)、「作成したあと」(番号は再利用しない)。口の既定で隠すと、この 2 つが書けない |
| A4b-4 | app の読み取り(`items.Service` の `Get`・`List`・`FindByNumber`、プロジェクトの `Get`・`List`)は削除済みを返さない。`Get` で削除済みなら `port.ErrNotFound`(404)。画面は 404 で詳細パネルを閉じ、プロジェクトの画面ならワークスペースの概要へ移る | 02「詳細パネル」の冒頭(削除されたらパネルを閉じる)、「名前の変更・移動・削除」(ワークスペースの概要に移る) |
| A4b-5 | 削除済みを除く場所(Task 2 が全呼び出し元を見て決める): ツリーと概要の一覧・Epic の進捗・子の完了の数、プロジェクトの名前の重複、使っているアイテムの件数と接頭辞の例(B2-11)、関連の一覧・候補・循環の検査・ブロック中(相手が削除済みなら一覧から隠し、ブロック中に数えない)、テンプレートとして保存の子、自動化の対象、兄弟の並びの末尾の決め方(削除済みの順位は比べない。06)。除かないもの: 番号、プロジェクトが変わる移動の下のもの | 02「削除」(関連は消さず一覧から隠す)、03、06「同時に編集したとき」、B2-11 |
| A4b-6 | 保持日数は、口 `port.Reader` に `RetentionDays() (int, error)` を足して `.hedgehug/store.yaml` の `retention_days` を読む(ないときは 30)。変えるのは G | 08「保持日数」。土台の `defaultRetentionDays = 30` |
| A4b-7 | アイテムの削除の確認ダイアログの中身は、サーバーの `GET items/{item}/delete-preview` が返す(下にある削除されていないものの件数・種類・保持日数)。{種類名の一覧} は、下にあるものの種類を Epic・Story・Task・Bug・Step の順で重ねずに「・」でつなぐ(例: 「下にある 3 件(Task・Step)も一緒に削除されます。」)。プロジェクトの削除も同じく `GET projects/{project}/delete-preview`(削除されていないアイテムの件数・保持日数) | 02「削除」。詳細パネルは下の孫まで読んでいない |
| A4b-8 | 削除のアクティビティは「{ID} {タイトル} を削除」、下にあるものがあれば「{ID} {タイトル} ほか N 件を削除」(N は同じ操作で `root: false` を書いた件数)。タイトルは空白と改行を 1 つの空白にまとめる(複数行の Summary は断られる)。プロジェクトは「プロジェクト「{タイトル}」を削除」 | 02「削除」「名前の変更・移動・削除」 |
| A4b-9 | 並べ替えは `items.Service.Reorder(ctx, store, id, anchor item.ID, after bool) error` に 1 本化し、`anchor` の直前(`after` なら直後)に置く。書くのは動かした 1 件の `set rank` だけで、値は `rank.Between`(端は `rank.After` と前の値)で作る。いまの `MoveUp`(2 件の値を入れ替える)と API `move-item-up` は消し、Step の「上へ」も「直前の Step の直前に置く」で呼ぶ(並びの結果は入れ替えと同じ)。`anchor` が同じ親の削除されていない兄弟でなければ `port.ErrNotFound`。すでにその位置なら何も保存しない。隣どうしの順位が同じで間の値が作れないときも何も保存しない(重なりの修復はトラック C) | 02・04「動くのは動かした 1 件だけ」と 04 の例(HH-5・HH-3・HH-4・HH-7・HH-9)。見えていない兄弟を動かさないためには、見えている兄弟を基準に置く形が要る |
| A4b-10 | 「上へ移動」「下へ移動」の基準(`anchor`)は画面が決める。core の `visibleNeighbors(rows, id)` が、見えている行の並びから同じ親の直前・直後の兄弟を返す。詳細パネルのメニューは、削除されていない全兄弟を見えている兄弟として渡す | 02「上から順の構成」、04「行」 |
| A4b-11 | 移動は `items.Service.Move(ctx, store, id, MoveInput) error`。`MoveInput{ Parent *item.ID; Project *project.ID }` のどちらか一方。Epic は `Project`(同じワークスペースのほかのプロジェクト)、ほかは `Parent`。書くのは、移すアイテムの `set place`(新しい親の子の末尾の `set rank` と同じ操作)と、プロジェクトが変わるときの下のものすべて(削除済みを含む)の `set place`(プロジェクトだけを変え、親と番号はそのまま)。断りは 02「アイテムの種類と親子」の文言(`refusal`)。自分自身の下は何もしない。いまの親と同じなら何もしない | 02「同じワークスペースの中での移動」、保存形式「移動・番号・旧 ID」 |
| A4b-12 | 移動の候補は `GET items/{item}/move-candidates` が全部返す(Epic にはほかのプロジェクト、Step には Story・Task・Bug、ほかには Epic。ワークスペース全体、自分と子孫といまの親を除く、削除済みを除く)。並びはプロジェクトの並び → ツリーの並び。検索は core が ID の前方一致(大文字小文字を区別しない)かタイトルの部分一致で絞る。件数の上限は設けない(メニューの中でスクロール) | 02「同じワークスペースの中での移動」。関連の候補(A4a-12)と同じ絞り方 |
| A4b-13 | 移動のアクティビティは、プロジェクトが変わるときは「{ID} を {プロジェクト名} に移動」、変わらないときは「{ID} を {ID} の下に移動」。トーストは「{プロジェクト名} に移動しました」「{ID} の下に移動しました」。Epic を別のプロジェクトへ、またはプロジェクトが変わる親への移動はプロジェクト名の文 | 02「同じワークスペースの中での移動」。プロジェクトが変わるときの文を優先する(両方が当てはまる文は仕様にない) |
| A4b-14 | 種類の変更は `items.Service.ChangeKind(ctx, store, id, kind item.Kind) error`。Story・Task・Bug は自分以外の Story・Task・Bug へ、Step は Story・Task・Bug へ(親の親の Epic の子の末尾へ `set place`・`set rank` も同じ操作で書く)。Epic と、Step への変更は `port.ErrNotFound`(画面から送らない)。アクティビティは「{ID} の種類を {種類} に変更」 | 02「種類の変更」 |
| A4b-15 | 複製は `items.Service.Duplicate(ctx, store, id) (item.ID, error)`。同じ親の子の末尾に作る。タイトルに「 のコピー」、種類・親・説明・プロパティ(優先度・担当者・関連システム・日付・ポイント・見積もり時間・追加プロパティ)は同じ、ステータスはワークスペースの初期ステータス、ウォッチ中の人はあなただけ。作成の行のあと、同じ操作で新しいアイテムのログに `relations` の `add {type: clones, to: 複製元}` を書く(履歴に出ない種類)。アクティビティは「{複製元の ID} を複製」 | 02「複製」。{ID} は複製元(「HH-3 を複製」と読める)。作成と同じく、説明は作成と同じ保存で書く(A4a-4) |
| A4b-16 | メニューの項目と順は「上へ移動」「下へ移動」「移動…」「種類を変更…」「複製」「テンプレートとして保存…」「ID をコピー」「削除…」。無効にするときは理由を添える: 閲覧のみは「閲覧のみの Store です」、上へ・下への端は「先頭です」「末尾です」、Epic の「種類を変更…」は「Epic の種類は変更できません」、Epic でほかのプロジェクトがない「移動…」は「移せるプロジェクトがありません」 | 02「上から順の構成」(使えない項目は無効にして理由を添える)。端と Epic の理由の文言は仕様にないので、項目の働きから短く決めた |
| A4b-17 | 「ID をコピー」の結果の表示は、種類の行の ID のコピーと同じ(「コピーしました: {ID}」・「コピーできませんでした。{ID} を選択してコピーしてください」) | 02「上から順の構成」の種類の行 |
| A4b-18 | 種類の変更は、メニューで「種類を変更…」を選ぶと選べる種類のメニュー(下位のメニュー)を出し、選ぶと確認ダイアログを出す | 02「種類の変更」(変える種類を選ぶと、実行の前に確認ダイアログ) |
| A4b-19 | プロジェクトの設定タブは、プロジェクトの画面のタブ「設定」(画面のアドレスの `tab: "settings"`)。A4b は「名前」「削除」の 2 区画だけを出す。「名前」は A4a の `projectEditor.rename`(`rename-project`)をそのまま使い、概要のタイトルと同じ保存のタイミングと断り方。「削除」は危険な操作の区画で、説明文の N は削除されていないアイテムの件数 | 02「画面の見出しと構成」、A4a の申し送り |
| A4b-20 | ツリーのフォーカスは core の `createTreeFocus` が持つ(見えている行の一覧・いまの行・開閉)。キーの割り当ては core の `treeKeyAction(event, row) → TreeKeyAction` が決め、app が実行する。行が見えなくなったら次、なければ前の見えている行へ移す | 04「ツリーのキー操作」 |
| A4b-21 | Alt+↑↓・Alt+Shift+↑↓ を続けて押したときは、core の順番待ち(`createKeyQueue`)が 1 回ずつ順に実行し、断られたとき・ほかのキーを押したとき・画面を移ったときに待っている分を捨てる | 04「ツリーのキー操作」 |
| A4b-22 | 開いている詳細パネルのアイテムが、同期・別のタブ・別のプログラムで削除されたときは、読み直しで 404 になったらそのまま閉じる(確認を出さない)。プロジェクトの画面のプロジェクトが削除されたら、ワークスペースの概要へ移る | 02「詳細パネル」の冒頭、A4b-4 |

---

## ファイルの地図

| 場所 | 新規・変更 | 役割 |
|---|---|---|
| `internal/domain/change/` | 変更 | `NewDeleteEntity`・`Deletion`・`DeletionOf` |
| `internal/domain/item/` | 変更 | `Deleted`、`Live`、`Descendants`、移動・種類の変更の検査(`CanPlace` の既存を使う) |
| `internal/domain/project/` | 変更 | `Deleted`、`Live` |
| `internal/app/port/stores.go`・`internal/app/apptest/`・`internal/infra/gitstore/` | 変更 | `Deleted` を埋める、`RetentionDays`、照合テスト |
| `internal/app/items/` | 変更・新規 | `delete.go`・`reorder.go`(`moveup.go` を置き換え)・`move.go`・`movecandidates.go`・`changekind.go`・`duplicate.go`、削除済みを除く |
| `internal/app/projects/` | 変更・新規 | `delete.go`、削除済みを除く |
| ほかの app の機能(`relations`・`workspaces`・`automations`・`history` など) | 変更 | 削除済みを除く(Task 2) |
| `internal/adapter/httpapi/items`・`projects` | 変更 | API |
| `web/packages/core/src/features/items/` | 変更・新規 | `menu.ts`(項目と無効の理由)、`siblings.ts`(見えている兄弟)、`moveCandidates.ts`(検索)、`treeFocus.ts`・`treeKeys.ts`・`keyQueue.ts` |
| `web/packages/core/src/actionsIndex.ts` | 新規 | A4b の core の export(`index.ts` は 300 行) |
| `web/app/src/wireActions.ts` | 新規 | A4b の配線(`wire.ts` は 300 行) |
| `web/app/src/screens/item/` | 変更・新規 | `ItemMenu.svelte` を全項目に、`DeleteItemDialog.svelte`・`MoveMenu.svelte`・`ChangeKindDialog.svelte` |
| `web/app/src/screens/project/` | 変更・新規 | `ProjectSettings.svelte`(名前・削除)、`DeleteProjectDialog.svelte`、`TreeRow.svelte` の分割とメニュー、`ProjectTree.svelte` のキー操作 |

## タスクの一覧

| # | タスク | 待つもの |
|---|---|---|
| 1 | 削除の印(`change.NewDeleteEntity`・`Deletion`・`DeletionOf`、`item`・`project` の `Deleted`、gitstore と偽物、照合テスト)と `Live`・`Descendants` | — |
| 2 | 削除済みを除く(app のすべての読み取りの呼び出し元を見て A4b-5 のとおりに) | 1 |
| 3 | 保持日数の口とアイテムの削除(予告・操作・API) | 1 |
| 4 | プロジェクトの削除(予告・操作・API) | 1・3 |
| 5 | 並べ替え `Reorder`(`MoveUp` を置き換え)と API。画面の Step の「上へ」の呼び出しを付け替える | 1 |
| 6 | 同じワークスペースの中での移動(候補・操作・断り・API) | 1 |
| 7 | 種類の変更(操作・API) | 1 |
| 8 | 複製(操作・API) | 1 |
| 9 | 画面の下ごしらえ: `wire.ts`・`index.ts`・`TreeRow.svelte`・`createDialog.ts` を分け、A4b の API を口(`ItemActionsPort`)と api の実装・偽物に足す | 3〜8 |
| 10 | core のメニューのモデル(項目・無効の理由・見えている兄弟)と移動の候補の検索 | 9 |
| 11 | 削除の確認ダイアログ(アイテム)と Step の「×」、削除のあとのパネルとツリー | 10 |
| 12 | 「移動…」の候補のメニューとトースト | 10 |
| 13 | 「種類を変更…」の種類のメニューと確認ダイアログ | 10 |
| 14 | 「複製」と「ID をコピー」 | 10 |
| 15 | 詳細パネルとツリーの行のメニューを全項目にする。上へ移動・下へ移動 | 11〜14 |
| 16 | プロジェクトの設定タブ(名前・削除)と削除のあとの移り先 | 4・9 |
| 17 | 別の経路の削除で、開いている詳細パネルを閉じ、プロジェクトの画面から移る | 11・16 |
| 18 | ツリーのフォーカスと、読むだけのキー(↑↓←→・Enter・Ctrl+Enter・メニューキー・Shift+F10) | 15 |
| 19 | ツリーの書き込みのキー(Space・S・K・C・Alt+↑↓・Alt+Shift+↑↓)と続けて押したときの順番待ち | 18 |
| 20 | 通しのテスト(e2e)と PR | すべて |

## タスク間の名前

各タスクの実装担当は自分のタスクしか読まないので、タスクをまたいで使う名前をここで決める。

### Go

| 名前 | 置き場所 | 形 |
|---|---|---|
| `change.DeletedField`・`change.NewDeleteEntity` | `internal/domain/change/draft.go` | `const DeletedField = "deleted"`、`func NewDeleteEntity(root bool) Draft`(`T: Delete, F: DeletedField, Root: &root`) |
| `change.Deletion`・`change.DeletionOf` | `internal/domain/change/` | `type Deletion struct { Op ID; At time.Time; Root bool }`、`func DeletionOf(log []Record) *Deletion`(なければ nil) |
| `item.Item.Deleted`・`project.Project.Deleted` | `internal/domain/item/item.go`・`project/project.go` | `Deleted *change.Deletion`。保存の口(gitstore・偽物)が埋める |
| `item.Live`・`project.Live` | 同上 | `func Live(its []Item) []Item`(削除済みを除く。順は保つ) |
| `item.Descendants` | `internal/domain/item/` | `func Descendants(id ID, all []Item) []Item`(子・孫を深さ優先、兄弟は `SortSiblings` の順。削除済みも含む。呼ぶ側が `Live` を通す) |
| `port.Reader.RetentionDays` | `internal/app/port/stores.go` | `RetentionDays() (int, error)` |
| `items.DeletePreview`・`(Service) DeletePreview`・`(Service) Delete` | `internal/app/items/delete.go` | `type DeletePreview struct { Count int; Kinds []item.Kind; RetentionDays int }`、`DeletePreview(ctx, store, id) (DeletePreview, error)`、`Delete(ctx, store, id) error` |
| `projects.DeletePreview`・`(Service) DeletePreview`・`(Service) Delete` | `internal/app/projects/delete.go` | `type DeletePreview struct { Items int; RetentionDays int }` |
| `(items.Service) Reorder` | `internal/app/items/reorder.go` | `Reorder(ctx, store, id, anchor item.ID, after bool) error` |
| `items.MoveInput`・`(Service) Move`・`items.MoveCandidate`・`(Service) MoveCandidates` | `internal/app/items/move.go`・`movecandidates.go` | `type MoveInput struct { Parent *item.ID; Project *project.ID }`、`Move(ctx, store, id, in MoveInput) error`、`type MoveCandidate struct { Item *item.Item; Project *project.Project; ProjectTitle string; Display string }`、`MoveCandidates(ctx, store, id) ([]MoveCandidate, error)` |
| `(items.Service) ChangeKind` | `internal/app/items/changekind.go` | `ChangeKind(ctx, store, id, kind item.Kind) error` |
| `(items.Service) Duplicate` | `internal/app/items/duplicate.go` | `Duplicate(ctx, store, id) (item.ID, error)` |

### HTTP API(`/api/stores/{store}/…`)

| operationId | メソッドとパス | 本文 → 応答 | タスク |
|---|---|---|---|
| `get-item-delete-preview` | GET `items/{item}/delete-preview` | → `{count, kinds: string[], retentionDays}` | 3 |
| `delete-item` | DELETE `items/{item}` | → 204 | 3 |
| `get-project-delete-preview` | GET `projects/{project}/delete-preview` | → `{items, retentionDays}` | 4 |
| `delete-project` | DELETE `projects/{project}` | → 204 | 4 |
| `reorder-item` | POST `items/{item}/reorder` | `{anchor, after}` → 204。`move-item-up` は消す | 5 |
| `list-move-candidates` | GET `items/{item}/move-candidates` | → `MoveCandidate{kind: "item"\|"project", id, display, title, projectTitle}[]` | 6 |
| `move-item` | POST `items/{item}/move` | `{parent?: string, project?: string}` → 204 | 6 |
| `change-item-kind` | PUT `items/{item}/kind` | `{kind}` → 204 | 7 |
| `duplicate-item` | POST `items/{item}/duplicate` | → `{id}`(新しいアイテムの内部 ID) | 8 |

### 画面

| 名前 | 置き場所 | 形 |
|---|---|---|
| `ItemActionsPort` | `web/packages/core/src/port/itemActions.ts` | 上の API を 1 メソッドずつ持つ口(`deletePreview`・`deleteItem`・`projectDeletePreview`・`deleteProject`・`reorder`・`moveCandidates`・`move`・`changeKind`・`duplicate`)。api の実装は `web/packages/api/src/itemActions.ts`、偽物は `web/packages/core/src/features/items/actions.test-util.ts`。Task 9 が全部を作る |
| `itemMenu` | `web/packages/core/src/features/items/menu.ts` | `itemMenu(ctx: ItemMenuContext): MenuEntry[]`。`MenuEntry = { id: ItemMenuId; label: string; disabled: boolean; hint?: string }`、`ItemMenuId = "move-up" \| "move-down" \| "move" \| "change-kind" \| "duplicate" \| "save-as-template" \| "copy-id" \| "delete"` |
| `visibleNeighbors` | `web/packages/core/src/features/items/siblings.ts` | `visibleNeighbors(rows: readonly Item[], id: string): { prev?: Item; next?: Item }`(`rows` は見えている順の行。同じ親で削除されていないものだけを比べる) |
| `filterMoveCandidates` | `web/packages/core/src/features/items/moveCandidates.ts` | `filterMoveCandidates(all: MoveCandidate[], q: string): MoveCandidate[]` |
| `createTreeFocus`・`treeKeyAction`・`createKeyQueue` | `web/packages/core/src/features/items/treeFocus.ts`・`treeKeys.ts`・`keyQueue.ts` | Task 18・19 の本文で決める |
| `itemActions` | `web/app/src/wireActions.ts` | `export const itemActions = createItemActionsPort();` と、ダイアログの開閉の状態 |
| `DeleteItemDialog.svelte`・`MoveMenu.svelte`・`ChangeKindDialog.svelte` | `web/app/src/screens/item/` | 詳細パネルとツリーの行の両方から開く。props は `{ storeId, item, onclose }` を基本に各タスクが決める |
| `ProjectSettings.svelte`・`DeleteProjectDialog.svelte` | `web/app/src/screens/project/` | |

---
