# フェーズ 4e: 残りの画面（概要）

> 4e は4回に分ける（2026-11-03 に利用者が決定）。この文書は4回に共通の決定事項・細部・回の分け方。Task の本文は各回の plan に書く: `2026-11-03-phase4e1-go.md`（Go の不足）、`2026-11-03-phase4e2-workspace.md`（ルートとタブ・サイドバー・ワークスペース・設定の骨組み・ダイアログ・パレット）、`2026-11-03-phase4e3-views.md`（カンバン・テーブル・保存済みビュー・マイタスク）、`2026-11-03-phase4e4-settings.md`（通知・アクティビティ・プロジェクトの設定・スキーマ・同期と保守・残りの E2E）。各回は前の回のマージのあとに main から切る。

**Goal:** spec の「画面と範囲」のうち 4d までに作っていない画面をすべて作り、「画面と範囲」の操作がすべて GUI で動くようにする。

**Architecture:** 4d の `web/core`（API クライアント・SSE・正規化キャッシュ・保存の制御・CLI の文字列・エラーの文言）と `web/app`（Svelte 5）の上に画面を足す。足りない API・ops・CLI は 4e-1 で先に Go に入れる（4d-1 と同じ形）。

**Tech Stack:** 4d と同じ。足すのは `@tanstack/svelte-table`（9 系の正式版。版は exact で固定し、公開から 7 日以上のもの）だけ。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「画面と範囲」「4d・4e フロントエンド」）。画面は Hedgehug GUI デザインのキャンバス（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）の Foundations と各アートボードを正とする。4d の決定と細部は `2026-11-02-phase4d-web.md` のとおりで、4e でもそのまま使う。

## Global Constraints

4d の概要 plan の Global Constraints をすべて引き継ぐ（日本語の文言、トークンの CSS 変数、ステータスは形で区別、行は ID だけ、外部に通信しない、キーボード、2 秒または blur で `PATCH`、トーストの秒数、足元の `hh` コマンド、`svelte-check` 警告ゼロと `biome ci` 指摘ゼロ、spec と違う挙動は止めて報告、Go のテストは `-race`、Go のジョブは `web/dist` なしで通る）。加えて:

- まだない画面への入口は出さない（4e の回の中でも、その画面を作る回まで）。後のフェーズの画面（ガント・バックログ・スプリント・リリース・レポート、アカウント・Store・自動化の設定）のタブと項目は出さない
- キャンバスの CLI の表記が実際のコマンドと違うときは実際のコマンドに合わせる（例: `hh workspace prefix` → `hh workspace set-prefix`、`hh purge --moved` は存在しない）

## 利用者の決定（2026-11-03、spec に反映する）

1. ProjectSettings は scoped の `statuses`（表示するステータス）と `workflow`（遷移の制約と必須の項目の追加）を閲覧・編集する。ops・API に加えて CLI も足す（CLI と GUI の対称を保つ）。spec の「表示するステータス、種類、プロパティ」は、scoped のデータ構造に種類とプロパティがないため「表示するステータスとワークフローの制約」に直す。`templates` はフェーズ6、自動化はフェーズ5
2. SettingsGeneral の同期の間隔は表示だけにする。`GET /api/sync` に `interval`・`push_interval` を足して出し、変えるには config.yaml を編集する旨と場所を案内する。`hh config` は作らない。キャンバスのプリセット（標準・控えめ・カスタム）と、非表示時の間隔・GitHub の間隔は作らない
3. 4e を4回に分ける: 4e-1 Go の不足、4e-2 ルートとタブ・サイドバーの入口・ワークスペースの概要と設定・設定のナビ・ダイアログ・パレット、4e-3 カンバン・テーブル・保存済みビュー・マイタスク、4e-4 通知・アクティビティ・プロジェクトの設定・スキーマ・同期と保守・残りの E2E

## 回の分け方と中身

| 回 | 中身 | 完了条件 |
|---|---|---|
| 4e-1 Go | 細部 1〜12 の API・ops・CLI の追加。`api/openapi.yaml` と `web/core/api/schema.d.ts` の再生成 | Go のテスト（ops・server・CLI のゴールデン）と OpenAPI の照合が通る |
| 4e-2 ワークスペース | ルートとタブ（細部 13〜15）、サイドバーの入口（16〜19）、Workspace（20〜22）、WorkspaceSettings（23〜25）、Project の見出しとタブ（26）、SettingsNav と SettingsWorkspaces（27〜29）、Dialogs（30〜33）、パレットのコマンド（34）、undo（35） | ワークスペースの概要・設定、ワークスペースの追加・改名・接頭辞の変更・削除、別のワークスペースへの移動、undo が GUI で動く（E2E） |
| 4e-3 ビュー | カンバン（36〜42）、テーブル（43〜49）、保存済みビュー（50〜54）、マイタスク（55〜57）、読み込み中と空の状態（58） | カンバンの移動、テーブルのセル編集、ビューの保存・開く・改名・削除、マイタスクが GUI で動く（E2E） |
| 4e-4 設定 | 通知（59〜63）、アクティビティ（64〜67）、ProjectSettings（68〜71）、SettingsSchema（72〜74）、SettingsGeneral（75〜80）、spec に書いた残りの E2E | 通知の既読と revert、アクティビティの絞り込み、scoped の編集、schema check と migrate、doctor と修復、purge が GUI で動く（E2E）。「画面と範囲」がすべて GUI で動く |

## この plan で埋める spec にない細部（利用者の承認待ち）

### 4e-1 Go の不足

1. **purge の API**: `GET /api/purge?older_than=<期間>&store=`（予定。ops の `PurgeList`。削除済みタスクと転送ファイルの一覧）と `POST /api/purge?store=`（本文 `{older_than, confirmed}`。Store はほかの API と同じくクエリ。`confirmed` は見せた予定で必須。ops の `PurgeConfirmed`。見せた予定と違えば 409 `purge.changed`）。期間は `hh purge --older-than` と同じ書式（`90d`、`0d` はすべて）。CLI は変えない。`web/core/messages.ts` に `purge.changed`・`purge.bad_period` などの文言を戻す
2. **タスクのプロジェクト**: `TaskView` に `project`（最も近い祖先のうち kind が `project` のもの。サイドバーのプロジェクト（細部 18 の `kind:project`）と一致させる。2026-11-03 に利用者が決定。`{ulid, id, title}`。なければ省く。自分自身は含めない）を足す。カンバンのスイムレーン、テーブルの `project` のグループ、マイタスクのプロジェクト別に使う。`hh ls --json` などの JSON にも同じ項目が出る（ゴールデンを更新）
3. **scoped の書き込み**（決定 1）: ops に `ScopedSet`（あるプロジェクトの上書きの `statuses`・`transitions`・`required` を、渡したものだけ置き換える。空の指定はその部分を Store の値に戻す）と `ScopedRemove`（そのプロジェクトの上書きを外す。`templates` も含めて消える）。書き込みは `schema check` と同じ検査（既定のステータスは外せない、未知のステータス・項目は拒否）を通してから1コミット（op `schema`）。`templates` などほかのキーは残す。CLI は `hh schema scope show <プロジェクト>`、`hh schema scope set <プロジェクト> [--statuses a,b] [--transition from=to1,to2]… [--required status=f1,f2]…`（指定した部分だけ置き換え。`--statuses ""` は Store と同じに戻す）、`hh schema scope rm <プロジェクト>`。API は `PUT /api/schema/scoped/{project}`（本文は `statuses`・`transitions`・`required`。省いたものは変えない）と `DELETE /api/schema/scoped/{project}`
4. **通知の一括既読**: `POST /api/notifications/read`（`{all: true}` または `{ids: [...]}`。ops の `InboxReadOf`。`hh inbox read --all` と同じ。応答は既読にした ULID の配列 `{read: [...]}` で `hh inbox read --json` と同じ）
5. **アクティビティの trailer**: `LogView` に `trailers`（キーと値の配列。`Hedgehug-Overwritten`・`Hedgehug-Moved-From` など、コミットにあるもの全部）を足す。画面は trailer をそのまま `sha · trailer` の行に出す
6. **doctor の finding のタスク**: `Finding` に `task`（ULID。タスクの問題のときだけ）を足し、画面の「<ID> を開く」に使う。文字列からは切り出さない
7. **ワークスペースの件数**: `WorkspaceView` に `deleted`（削除済みの件数）を足す。`workspace.not_empty` の args に `tasks`（削除済みを含む件数）を足す。WorkspaceSettings の「配下の n 件（削除済みを含む）」と削除の無効の理由に使う
8. **同期の間隔**（決定 2）: `SyncStatus` に `interval`・`push_interval`（秒）を足す。値は config.yaml の `sync.interval`・`sync.push_interval` そのもの（非表示時の 10 分は画面の説明文で出す）
9. **GET /api/activity の `store` の説明**を実装（省略は全 Store）に合わせて直す
10. 1〜8 の OpenAPI を再生成し、`web/core/api/schema.d.ts` を作り直す（`pnpm gen`）
11. 親設計の「コマンド一覧」に `hh schema scope show / set / rm` を足す（決定 1）
12. spec の ProjectSettings・SettingsGeneral の行を決定 1・2 に合わせて直す

### 4e-2 ワークスペースと骨組み

13. **ルート**: `/workspaces/<store>/<ULID>`（概要）と `/workspaces/<store>/<ULID>/{tree,kanban,table,settings}`、`/projects/<ULID>`（ツリー。プロジェクトの既定の画面は 4d と同じツリー）と `/projects/<ULID>/{kanban,table,settings}`、`/views/<store>/<名前>`、`/my`、`/inbox`、`/activity`、`/settings/general`・`/settings/schema/<store>`・`/settings/workspaces/<store>`。`/` は最初のワークスペースの概要。ブラウザの戻る・リンクのコピー・⌘↵ の新しいタブが効くよう、タブも URL に持つ。既存の E2E のツリーの URL は `/tree` 付きに直す
14. **タブ**: ワークスペースは 概要・ツリー・カンバン・テーブル・設定、プロジェクトは ツリー・カンバン・テーブル・設定（ガント・バックログ・スプリント・リリース・レポートはフェーズ6まで出さない）
15. 設定の画面の Store の選択欄は、Store が2つ以上のときだけ出す。選んだ Store は URL に持つ。閲覧のみの Store の印と編集の無効は 4f（3 の機能）で足し、4e では書き込みの拒否をサーバーのエラーの文言で出す
16. **サイドバー**: マイタスク（件数は細部 55 と同じ問い合わせの数）、通知（未読数。SSE の `notifications.unread`）、アクティビティ、保存済みビュー（全 Store のビューを並べ、保存先の Store の色の点。同じ名前は Store 名をツールチップで区別）、ワークスペースの行は概要を開く、「…」は「設定を開く」「削除…」の2つ（接頭辞の変更は WorkspaceSettings だけ。spec のとおり。キャンバスの Dialogs の見出しは直す）
17. 保存済みビューの一覧は、`GET /api/stores` の各 Store に `GET /api/views?store=` を送って集め、`views.changed {store}` でその Store だけ取り直す（API は Store ごとのまま）
18. サイドバーの右上の歯車は `/settings` を開く（最初の Store のワークスペースの設定へ置き換える。4e-4 で同期と保守ができたら `/settings/general`。アカウントの設定は 4f）。「ワークスペース」見出しの右の「＋」はキャンバスどおりプロジェクトの追加（2026-11-03 に利用者が決定）
19. 「今すぐ同期」・同期のカードは 4d のまま
20. **Workspace の概要**: 見出し（Store のバッジ・名前・接頭辞・「タスク」「プロジェクト」の作成ボタン。作成は入れ先をこのワークスペースに固定）、件数（削除済みを除く全タスク。プロジェクトも含む。`WorkspaceView.tasks` と同じ値で「件 · プロジェクト m」の m はその内数）とカテゴリ別の帯（`GET /api/tasks?q=workspace:<名前>&store=` の結果をフロントで数える）、足元の `hh ls --tree workspace:<名前> --store <Store>`
21. プロジェクトの一覧（ID・タイトル・進捗のバーと `done/total`・期限・担当）と、プロジェクトの外のルートのタスク（rank 順。ステータス・種類・ID・タイトル・期限・担当）。「ツリーで開く」は `/tree`
22. 件数・一覧は `tasks.changed` で取り直す（4d のキャッシュの問い合わせとして持つ）
23. **WorkspaceSettings**: 名前はその場で編集し「保存」で `PATCH /api/workspaces/{id}`（同じ Store の名前と重なれば赤字で理由、保存を無効）。足元に `hh workspace rename`
24. 接頭辞は今の値と「接頭辞を変更…」（確認ダイアログ。配下の件数は `tasks + deleted`）。キャンバスの「最後に変えたのは 10/29」も `WorkspaceView.prefix_set` から出す（2026-11-03 に利用者が決定。当初「API にない」としたのは誤り）。設定のワークスペース一覧にも同じ日付
25. 削除は、配下にタスクが残っていれば（`tasks + deleted > 0`）無効にして理由（「タスク n 件（削除済み m 件を含む）が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます」）を出す。スプリント・リリースの件数はフェーズ6で足す。サーバーの 409 `workspace.not_empty` は競合したときの保険
26. **Project**: 見出しにワークスペース › プロジェクトと Store のバッジ、ワークスペースへ戻るリンク、進捗（4d のまま）。キャンバスの「気にかけること」・ブロック中／制約違反／今日が期限の件数・ウォッチ数・計画の欄は spec にないので作らない
27. **SettingsNav**: 「同期と保守」「スキーマ」「ワークスペース」の3つ（アカウント・Store・自動化は出さない）。「スキーマとテンプレート」のバッジは意味が決まらないので出さない。脚注は版（`GET /api/health` の版）だけで、config.yaml のパスは出さない
28. **SettingsWorkspaces**: Store ごとのワークスペースの一覧（名前・接頭辞・プロジェクト数・タスク数）、「ワークスペースを追加」（Dialogs）、各行から設定タブへ。プロジェクト数は `kind` の role が container のタスクの数をフロントで数える
29. 設定の画面は全体を1つのレイアウト（左に SettingsNav、右に本文）にする
30. **Dialogs**: ワークスペースの追加（名前・接頭辞・Store。Store は2つ以上のときだけ選ぶ）、接頭辞の変更の確認（変わる件数と古い ID でも引けること）、ワークスペースの削除の確認、ビューを保存（4e-3）、別のワークスペースへの移動
31. **別のワークスペースへの移動**: タスク詳細の見出しの「…」（その他の操作）とツリーの行の「…」を新しく置き（2026-11-03 に利用者が決定）、パレットの選んだタスクのコマンドにも足す。移り先は同じ Store のワークスペースだけ（別の Store への移動は 4f の StoreMove）。説明は「スプリント・リリースが移り先のものでなければ外れます」と静的に出し、移したあとの応答の `detached` を名前でトーストに出す（事前の一覧はフェーズ6まで常に空なので dry-run の API は足さない）。足元に `hh mv <ID> --workspace <名前>`
32. キャンバスの「完全削除の確認」は作らない（2026-11-02 の決定。purge は設定の同期と保守）
33. ダイアログはどれも 4d の `ui/Dialog.svelte`（背後を押しても閉じない、Esc と閉じる）
34. **パレットのコマンド**: 設定を開く（同期と保守・スキーマ・ワークスペース）、マイタスク・通知・アクティビティを開く、保存済みビューを開く、ワークスペースを追加、直前の操作を取り消す（undo）、doctor を実行（同期と保守を開く）。「このクエリで: テーブルで開く・ビューとして保存」は 4e-3
35. **undo**: パレットの「直前の操作を取り消す」で `POST /api/undo` を呼び、結果をトーストで出す（確認しない。spec の決定）。⌘Z は割り当てない（spec のキーボードにないため、入力欄の取り消しとぶつかる）

### 4e-3 カンバン・テーブル・ビュー・マイタスク

36. **カンバンの列**: `status`（スキーマの順。scoped の `statuses` があるプロジェクトの画面ではそれだけ）と `category`（todo → doing → done → canceled。表示名は「未着手」「進行中」「完了」「中止」）。既定のフィルタは 4d と同じ `-status:canceled` で、中止の列は条件に合うタスクがあるときだけ出す
37. **スイムレーン**: なし・担当者（複数担当のカードは各担当者の行に出す。未割り当ての行は最後）・プロジェクト（細部 2 の `project`。プロジェクトの外は最後）
38. **ドラッグ**: 列の間の移動でステータスを変える（`PATCH`。カテゴリ列ではそのカテゴリの既定のステータス）。列の中の順は rank のままで並べ替えはしない。スイムレーンをまたぐドロップは担当者・プロジェクトを変えるので拒否する（ツリーと同じく赤枠と理由のトースト）。遷移の制約に合わない列はドラッグ中に赤枠
39. **必須の項目**: 移る先のステータスに必須の項目（Store と scoped の和）が空なら、ダイアログ（「<ID> を<ステータス>にするには <項目> が必要です」）で入力させ、ステータスと項目を1回の `PATCH`（1コミット）で送る
40. カードの「points 未入力」などの警告は、`problems` があるときだけ出す（フロントで必須を判定しない。サーバーの検査と食い違わないため）
41. 「完了」列の「ほか n 件」の打ち切りは作らない（全件出す）。カードの PR はフェーズ5
42. 列の「＋」は、その列のステータスで作成ダイアログを開く（`POST /api/tasks` の `status`）
43. **テーブル**: TanStack Table（`@tanstack/svelte-table` 9 系の正式版）で、行は 4d と同じ TanStack Virtual で仮想化する（グループの見出しも行として平らにする）
44. **列の候補**: 組み込みの9個、続けてスキーマのプロパティを名前順。表示の順は保存した `table.columns` の順。複数の Store を表示しているときは全 Store のプロパティの和で、ある Store のスキーマにない列はそのタスクだけ「（未定義）」の斜体。キャンバスの「ブロックされている」（`blocked_by`）列は組み込みの列にないので作らない
45. **セルの編集**: 4d の `detail/FieldInput.svelte` をセルで使い、2 秒または blur で `PATCH`。ID・作成・更新・親は読み取りだけ
46. **並び替えとグループ化はフロントで行う**: 列の見出しのクリックの並び替えは保存しない画面の状態。保存したい並びはフィルタ欄の `sort:`（`hh view save --query` と同じ）。グループは `none`・`parent`・`status`・`assignee`・`project`。グループの見出しは親なら進捗（`done/total`）を出す。キャンバスの「14 pt」（子孫の合計）は作らない
47. 一括編集（複数選択）はフェーズ6
48. 列の選択はメニューのチェックボックス。保存済みビューを開いているときは変更で「未保存の変更」の印を出し、「ビューを更新」で `PUT`
49. テーブル・カンバンの足元は `hh view save <名前> --layout table --query …` の形、ビューでないときは `hh ls <クエリ>`
50. **保存済みビュー**: `/views/<store>/<名前>` で、保存した layout で表示し、画面でツリー・カンバン・テーブルを切り替えられる。切り替えは `PATCH` ではなく `PUT`（`hh view save <名前> --layout` と同じ1コミット）。上部に名前・保存先・`hh view show` の行
51. **ビューを保存のダイアログ**: 名前（`view.*` の検証の文言）、保存先の Store（表示中の Store から選ぶ。既定は今の画面の Store、全 Store にまたがる画面は `GET /api/stores` の `default` の Store。選ぶ欄は2つ以上のときだけ）、今の設定の要約、足元の `hh view save`。今の並び替えの状態は保存しない（細部 46）
52. ビューの改名と削除を GUI に置く（ビューの画面の「…」から。削除は確認ダイアログと `hh view rm`）。API が揃っており、CLI の操作は GUI でもできるようにするため
53. 同じ名前のビューが別の Store にあってよい（URL に Store が入る）
54. tree の layout のビューでフィルタ欄に `sort:`・`group:` を入れたら、保存の前に理由を出して保存を無効にする（サーバーの拒否と同じ）
55. **マイタスク**: 既定の問い合わせは `assignee:@me -status:done -status:canceled`（フィルタ欄で変えられる）。`@me` は config の全アカウント（サーバーの今の扱い）
56. 「自分 = …」は `GET /api/stores` の `account` の login を並べる（個人・仕事の別名は 4f）
57. まとめ方は 期限（期限切れ・今日・今週・来週以降・期限なし。週は月曜はじまり、ブラウザのローカルの日付で比べる）とプロジェクト（細部 2）の切り替え。ID は Store の修飾なしで、Store のバッジで区別（修飾は 4f）
58. カンバン・テーブル・マイタスクの読み込み中と空の状態は 4d のツリーと `states/Empty.svelte` に揃える。キャンバスの States の「もしかして」（クエリの誤りの候補）は、フロントがスキーマの名前から編集距離 2 以内のものを出す（Go は変えない）

### 4e-4 通知・アクティビティ・設定

59. **通知**: 一覧（未読を上）、タブは「すべて」「上書き」「メンション」「同期の修復」（自動化はフェーズ5なので出さない）。repaired・revived・renumbered・prefix_duplicate・rewrite_applied・purged_change は「同期の修復」に入れる
60. 通知の文は kind と detail からフロントで日本語にする（spec の「表示の文言はフロントエンドが持つ」）。上書きの相手の名前は API にない（inbox が持たない）ので出さず「同時に編集した値で上書きされました」とする
61. 既読は行を開いたときと「既読にする」、「すべて既読にする」は細部 4 の API
62. 「自分の値に戻す」（revert）はボタンを出し、戻せないとき（移動済み・削除済みなど）はサーバーのエラーをトーストで出す（事前に判定しない）
63. 未読数はサイドバーと同じ SSE の値
64. **アクティビティ**: 変更のフィード（新しい順。日付の見出し、作者、op の日本語名、件名、ID、`sha · trailer`）。人と同期の絞り込みは `sync=exclude`（人）・`only`（同期）・なし（すべて）。フェーズ5で自動化を分ける
65. ワークスペースの絞り込みは spec のとおり検索つきのチェックボックスのリスト（Store ごとの見出し、未選択はすべて、`?workspace=`）
66. 作者の表示は、op が merge・repair なら「同期（<作者名>）」、それ以外は git の作者名。trailer の日本語の説明文は作らない（trailer をそのまま出す）
67. 件数は `limit`（既定 100）で「さらに読み込む」で増やす
68. **ProjectSettings**: 表示するステータス（チェックボックス。カテゴリの既定は外せない印。「Store と同じ」に戻す）と、ワークフローの制約（Store の制約と並べ、このプロジェクトで足したものだけ外せる。遷移の制約の追加、必須の項目の追加）。保存は「保存」ボタンで1回の `PUT /api/schema/scoped/{project}`（1コミット）。足元に `hh schema scope set …`
69. 「すべて Store の値に戻す…」は確認ダイアログで `DELETE`（`templates` も外れることを出す）
70. いまの違反は `is:invalid project:<ULID>` の件数と ID を出す
71. キャンバスのテンプレート（6）・自動化（5）・「最終更新 11/03 koki」は作らない
72. **SettingsSchema**: Store ごとに、ステータス・種類・プロパティ・ワークフローの閲覧と、各定義を使っているタスクの件数（全タスクをフロントで数える）。上書きのあるプロジェクトの一覧（親設計の「Store の設定画面には、上書きのあるプロジェクトの一覧を出す」）
73. `schema check` の結果（`GET /api/schema/check`）と、migrate（各行の「名前を変える…」「併合する…」。`dry_run` の結果で変わる件数を確認ダイアログに出してから実行）。キャンバスの追加・削除・色・並び替え・未保存の変更の帯・「YAML を開く」は作らない（spec の範囲は閲覧・check・migrate）
74. 足元に `hh schema check`・`hh migrate …`
75. **SettingsGeneral**: 同期の状態（4d の同期のカードと同じ値を Store ごと）と間隔（決定 2）
76. doctor は `GET /api/doctor` の findings を `check` ごとにまとめて出し、なければ「問題は見つかりませんでした」。「もう一度実行」、各 finding のタスクは「<ID> を開く」（細部 6）
77. 修復は確認ダイアログを挟んで `POST /api/doctor/fix`、結果の `repaired` と残りを出す
78. **purge**: 「N 日より古いもの」の入力（既定 90、0 で全部）と「確かめる」→ 予定の一覧（ID・タイトル・転送ファイルの件数）→ 確認ダイアログ（取り消せないこと）→ 実行 → コミットをトースト。足元に `hh purge --older-than <N>d`
79. キャンバスの GUI サーバーの欄（待ち受け・トークン・プロセス・トークンの作り直し）、merge driver の「登録する」、インデックスの作り直し、repack は作らない（spec にない）。merge driver の未登録は doctor の finding と CLI の行で案内する
80. **E2E**（spec の 4e 分）: カンバンの移動（必須の項目のダイアログを含む）、通知の既読と revert、保存済みビュー（保存・開く・レイアウトの切り替え）、undo。4e-2・4e-3 の回ではそれぞれの画面の E2E も足す（ワークスペースの追加・改名・接頭辞・削除、移動、テーブルのセル編集、マイタスク）
