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
28. **SettingsWorkspaces**: Store ごとのワークスペースの一覧（名前・接頭辞・プロジェクト数・タスク数）、「ワークスペースを追加」（Dialogs）、各行から設定タブへ。プロジェクト数は kind が `project` のタスクの数（`WorkspaceEntry.projects` の件数。サイドバー・概要と同じ）
29. 設定の画面は全体を1つのレイアウト（左に SettingsNav、右に本文）にする
30. **Dialogs**: ワークスペースの追加（名前・接頭辞・Store。Store は2つ以上のときだけ選ぶ）、接頭辞の変更の確認（変わる件数と古い ID でも引けること）、ワークスペースの削除の確認、ビューを保存（4e-3）、別のワークスペースへの移動
31. **別のワークスペースへの移動**: タスク詳細の見出しの「…」（その他の操作）とツリーの行の「…」を新しく置き（2026-11-03 に利用者が決定）、パレットの選んだタスクのコマンドにも足す。移り先は同じ Store のワークスペースだけ（別の Store への移動は 4f の StoreMove）。説明は「スプリント・リリースが移り先のものでなければ外れます」と静的に出し、移したあとの応答の `detached` を名前でトーストに出す（事前の一覧はフェーズ6まで常に空なので dry-run の API は足さない）。足元に `hh mv <ID> --workspace <名前>`
32. キャンバスの「完全削除の確認」は作らない（2026-11-02 の決定。purge は設定の同期と保守）
33. ダイアログはどれも 4d の `ui/Dialog.svelte`（背後を押しても閉じない、Esc と閉じる）
34. **パレットのコマンド**: 設定を開く（同期と保守・スキーマ・ワークスペース）、マイタスク・通知・アクティビティを開く、保存済みビューを開く、ワークスペースを追加、直前の操作を取り消す（undo）、doctor を実行（同期と保守を開く）。「このクエリで: テーブルで開く・ビューとして保存」は 4e-3
35. **undo**: パレットの「直前の操作を取り消す」で `POST /api/undo` を呼び、結果をトーストで出す（確認しない。spec の決定）。⌘Z は割り当てない（spec のキーボードにないため、入力欄の取り消しとぶつかる）

### 4e-3 カンバン・テーブル・ビュー・マイタスク

36. **カンバンの列**: `status`（スキーマの順。scoped の `statuses` があるプロジェクトの画面ではそれと、タスクのいる表示外のステータス（2026-11-03 に利用者が決定。タスクを隠さない））と `category`（todo → doing → done → canceled。表示名は「未着手」「進行中」「完了」「中止」）。既定のフィルタは 4d と同じ `-status:canceled` で、中止の列は条件に合うタスクがあるときだけ出す
37. **スイムレーン**: なし・担当者（複数担当のカードは各担当者の行に出す。未割り当ての行は最後）・プロジェクト（細部 2 の `project`。プロジェクトの外は最後）
38. **ドラッグ**: 列の間の移動でステータスを変える（`PATCH`。カテゴリ列ではそのカテゴリの既定のステータス）。列の中の順は rank のままで並べ替えはしない。スイムレーンをまたぐドロップは担当者・プロジェクトを変えるので拒否する（ツリーと同じく赤枠と理由のトースト）。遷移の制約に合わない列はドラッグ中に赤枠
39. **必須の項目**: 移る先のステータスに必須の項目（Store と scoped の和）が空なら、ダイアログ（「<ID> を<ステータス>にするには <項目> が必要です」）で入力させ、ステータスと項目を1回の `PATCH`（1コミット）で送る。ダイアログで入力できない項目（親・ブロッカー・ウォッチャー・リポジトリ・タスク参照・スプリント・リリース・リンク）が欠けていれば、それも並べて「詳細で入力」へ案内し、移動を無効にする（サーバーの 422 を起こさない。2026-11-03 に利用者が決定）
40. カードの「points 未入力」などの警告は、`problems` があるときだけ出す（フロントで必須を判定しない。サーバーの検査と食い違わないため）
41. 「完了」列の「ほか n 件」の打ち切りは作らない（全件出す）。カードの PR はフェーズ5
42. 列の「＋」は、その列のステータスで作成ダイアログを開く（`POST /api/tasks` の `status`）
43. **テーブル**: TanStack Table（`@tanstack/svelte-table` 9 系の正式版）で、行は 4d と同じ TanStack Virtual で仮想化する（グループの見出しも行として平らにする）
44. **列の候補**: 組み込みの9個、続けてスキーマのプロパティを名前順。表示の順は保存した `table.columns` の順。複数の Store を表示しているときは全 Store のプロパティの和で、ある Store のスキーマにない列はそのタスクだけ「（未定義）」の斜体。キャンバスの「ブロックされている」（`blocked_by`）列は組み込みの列にないので作らない
45. **セルの編集**: 4d の `detail/FieldInput.svelte` をセルで使い、2 秒または blur で `PATCH`。ID・作成・更新・親は読み取りだけ
46. **並び替えとグループ化はフロントで行う**: 列の見出しのクリックの並び替えは保存しない画面の状態。保存したい並びはフィルタ欄の `sort:`（`hh view save --query` と同じ）。グループは `none`・`parent`・`status`・`assignee`・`project`。グループの見出しは親なら進捗（`done/total`）を出す。キャンバスの「14 pt」（子孫の合計）は作らない
47. 一括編集（複数選択）はフェーズ6
48. 列の選択はメニューのチェックボックス。保存済みビューを開いているときは変更で「未保存の変更」の印を出し、「ビューを更新」で `PATCH /api/views/{name}`（あるビューの設定を変えるだけで、無いビューは作らない。404 `view.not_found` なら「見つかりません」の画面。利用者の決定、2026-11-03）
49. テーブル・カンバンの足元は、保存済みビューなら `hh view save <名前> --layout … --store <Store>`（`--query` は付けない。CLI は保存したクエリを残す。コピーできる）、ビューでないときは `hh view save <名前> --layout … --query …`（名前が決まるまでコピーできない）。どちらも `--query` の有無以外は同じ形
50. **保存済みビュー**: `/views/<store>/<名前>` で、保存した layout で表示し、画面でツリー・カンバン・テーブルを切り替えられる。切り替えは `PATCH /api/views/{name}` に `layout` だけを送る（`hh view save <名前> --layout` と同じ1コミット。`PUT` は無いビューを作るため使わない。利用者の決定、2026-11-03）。上部に名前・保存先・`hh view show` の行
51. **ビューを保存のダイアログ**: 名前（`view.*` の検証の文言）、保存先の Store（表示中の Store から選ぶ。既定は今の画面の Store、全 Store にまたがる画面は `GET /api/stores` の `default` の Store。選ぶ欄は2つ以上のときだけ）、今の設定の要約、足元の `hh view save`。今の並び替えの状態は保存しない（細部 46）
52. ビューの改名と削除を GUI に置く（ビューの画面の「…」から。削除は確認ダイアログと `hh view rm`）。API が揃っており、CLI の操作は GUI でもできるようにするため
53. 同じ名前のビューが別の Store にあってよい（URL に Store が入る）
54. tree の layout のビューでフィルタ欄に `sort:`・`group:` を入れたら、保存の前に理由を出して保存を無効にする（サーバーの拒否と同じ）
55. **マイタスク**: 既定の問い合わせは `assignee:@me -status:done -status:canceled`（フィルタ欄で変えられる）。`@me` は config の全アカウント（サーバーの今の扱い）
56. 「自分 = …」は `GET /api/stores` の `account` の login を並べる（個人・仕事の別名は 4f）
57. まとめ方は 期限（期限切れ・今日・今週・来週以降・期限なし。週は月曜はじまり、ブラウザのローカルの日付で比べる）とプロジェクト（細部 2）の切り替え。ID は Store の修飾なしで、Store のバッジで区別（修飾は 4f）
58. カンバン・テーブル・マイタスクの読み込み中と空の状態は 4d のツリーと `states/Empty.svelte` に揃える。キャンバスの States の「もしかして」（クエリの誤りの候補）は、フロントがスキーマの名前から編集距離 2 以内のもの、なければ打った語（3 文字以上）を含む名前を出す（Go は変えない。含む名前は 2026-11-03 に利用者が決定。キャンバスの `progres` → `in_progress` のため）

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

### 4e-3 の plan で足した細部（2026-11-03）

4e-3 の plan（`2026-11-03-phase4e3-views.md`）を組むときに決めたもの。81・82 は利用者の決定、ほかは親設計（「ビューは常に統合する」「Store をまたぐ統合ビューはカテゴリで揃える」「sort:status はカテゴリの順で Store をまたいで比べる」「複数のプロジェクトのタスクが並ぶビューは Store の定義で表示する」）から導いたもの。細部は 4e-3 の plan 末尾「決めたこと」が詳しい。

81. **保存していないクエリの画面**: `/query/<layout>?q=<クエリ>`（tree・kanban・table）。全 Store で評価し、レイアウトはタブで切り替えるだけで保存しない。「ビューを保存」を置く。router はこのルートだけ `?q=` を残す。パレットの「このクエリで: テーブルで開く」はここ（細部 13・34 に足す）
82. **`hh view show` の評価範囲**: `--store` はビューを読む場所としてだけ使い、クエリは常に全マウント Store で評価する（`store:` で絞る）。GUI のビューも全 Store で評価する。親設計の「保存済みビュー」に足す
83. scoped の `statuses` を使うのはプロジェクトの画面だけ。ビュー・クエリ・マイタスク・ワークスペースは Store の定義
84. Store をまたぐカンバンの status 列は、表示中のタスクの Store のステータスの和を sort:status の順（カテゴリ → スキーマでの位置 → Store の順）に並べ、同じ名前は1列。テーブルの sort:status とグループも同じ順
85. Store をまたぐツリー（ビュー・クエリの tree）は根を Store の順に並べ、Store が2つ以上なら行に Store のバッジ。規則とキー操作は行の Store、作成は選んだ行の Store とワークスペース
86. ビューの画面では、クエリ・カンバン・テーブルの設定のどれかが保存した値と違えば「未保存の変更」と「ビューを更新」（細部 48 をカンバンとクエリに広げる）
87. ビューでない画面の列・スイムレーン・テーブルの列・グループは開いているあいだだけ（保存しない）。初期値はビューの既定
88. ワークスペース・プロジェクトから保存するクエリは固定の部分（`workspace:<名前>`・`project:<ID>`）＋フィルタ欄で、Store が2つ以上なら先頭に `store:<Store>`。保存したらそのビューを開く。同じ Store に同じ名前があれば保存を無効にして理由を出す
89. ビューの URL は `/views/<store>/<名前>`。足元は `hh view show <名前> --store <Store>`
90. プロジェクトの見出し（パンくずとタブ）は `ProjectHeader` にしてツリー・カンバン・テーブルのタブで共有する。カンバンのカードのキーボード操作は作らない

### 4e-4 の plan で足した細部（2026-11-03）

4e-4 の plan（`2026-11-03-phase4e4-settings.md`）を組むときに決めたもの（91〜105）と、組み上げの途中で決めたもの（106〜）。どれも API の形（Store ごとの API）と既存の画面の決まりから導いた。実装で plan から変わった点は、4e-4 の plan 末尾の「決めたこと」と「組み上げで直した点」が詳しい。

91. **通知は全 Store**: `GET /api/notifications?all=true` を `store` なしで1回呼ぶ（全 Store が返る）。未読を上、その中は時刻の新しい順に並べる。Store が2つ以上なら行に Store のバッジ
92. 通知のタブの数は、そのタブの未読の数。0 なら数を出さない。「割り当て」「ウォッチ」のタブは作らず「すべて」に入る
93. 「すべて既読にする」は `store` なしで `POST /api/notifications/read {all: true}` を1回。未読が 0 なら無効
94. 上書きの行のボタンは「自分の値に戻す」と「このままにする」（= 既読にする）。`prefix_duplicate` などワークスペースの通知（`task` がない）は ID のリンクを出さない。ほかの未読の行は「既読にする」。行の ID のリンク（タスク詳細を開く）を押すと、その通知を既読にしてから開く
95. 通知の一覧は、未読数（`c.conn.unread`）が変わったとき・`resync`・`tasks.changed` で取り直す
96. **アクティビティの範囲**: `GET /api/activity` を `store` なしで呼ぶ（全 Store）。ワークスペースの絞り込みは選んだワークスペースの ULID をカンマで `workspace=` に渡す。URL の `?workspace=` も ULID のカンマ区切り。人・同期の切り替えは URL の `?sync=exclude|only`（なしはすべて）。足元は `hh log --workspace <名前,…>`（選んだワークスペースの名前が別の Store の名前と重なるときだけ、その分を ULID で書く）
97. アクティビティの行の時刻は author の時刻をブラウザのローカルの時刻で `HH:MM`、日付の見出しは「今日 · 11/04（水）」「昨日 · 11/03（火）」、それより前は「11/02（月）」（年が違えば `2025/11/02（日）`）。行の ID はタスク詳細へのリンク。`tasks.changed` で取り直す
98. **プロジェクトの設定のルート**: `/projects/<ULID>/settings`。`ProjectHeader` のタブの右端に「設定」
99. ProjectSettings の「保存」は、`statuses`・`transitions`・`required` の3つのキーのうち画面で変えたキーだけを `PUT` の本文に入れる。入れたキーは差分ではなく、そのプロジェクトの上書きの全体を持つ。Store と同じに戻すときは `statuses: []`、`transitions: {}`・`required: {}`。変えていなければ「保存」は無効。保存したら `schema.changed` で画面と `SchemaCache` を取り直す
100. **設定のナビ**: 「このマシン」の段に「同期と保守」（`/settings/general`）、「Store ごとの設定」の段に「ワークスペース」「スキーマ」（`/settings/schema/<store>`）。キャンバスの「スキーマとテンプレート」は「スキーマ」とする。`/settings` は `/settings/general` に置き換える。サイドバーの歯車は `/settings/general`
101. **同期と保守の範囲**: 同期の状態・doctor は全 Store を Store ごとの段で並べる。purge は Store を選んで行う（Store が2つ以上のときだけ選択欄）。同期の間隔は Store によらず config.yaml の値なので1つだけ出す（最初の Store の `SyncStatus.interval`・`push_interval`）
102. doctor は画面を開いたときに各 Store へ `GET /api/doctor?store=` を1回送る。「もう一度実行」で取り直す。`tasks.changed` では取り直さない（重い検査のため）。修復の後は、修復の応答の findings をそのまま出す（二度目の GET は送らない。細部 120）
103. **スキーマの範囲**: 選んだ Store（URL）の `GET /api/schema?store=` と `GET /api/schema/check?store=`。件数は `GET /api/tasks?store=<S>`（削除済みを除く全タスク）をフロントで数える。`schema.changed` では check だけを取り直す（細部 129）。タスクの件数は `tasks.changed`（その Store）で取り直す
104. migrate のダイアログは新しい名前の入力欄と、入力が止まってから送る `dry_run: true` の結果（変わる件数と、`skipped` の件数）を出し、「名前を変える」（新しい名前が今の定義にないとき）か「併合する」（あるとき）で実行する。行のボタンは「名前を変える…」と「併合する…」の2つで、併合は今の定義から相手を選ぶ。値（`value`）の migrate は enum・multi_enum のプロパティの値の行に出す
105. パレットのコマンドは、その画面を作った Task で足した: 「通知を開く」、「アクティビティを開く」、「プロジェクトの設定を開く」（プロジェクトの画面とそのタスクの詳細を開いているときだけ）、「設定の同期と保守を開く」、「doctor を実行」（同期と保守を開く）、「設定のスキーマを開く」（Store ごと）

以下は組み上げの途中で決めたもの。

106. **通知の足元**: `hh inbox --all`（既読を含めて一覧する）。既読にした上書きの行はボタンを出さない
107. 同期の修復の通知の文は、detail の種類（repaired・revived・renumbered・prefix_duplicate・rewrite_applied・purged_change）ごとに分けて書く
108. **タブはアドレスに持たせる**: 通知は `/inbox?tab=overwritten|mention|repair`（なしはすべて）、スキーマは `/settings/schema/<store>?tab=kinds|props|workflow`（なしはステータス）。タブの一覧は1か所（router）に置き、ルートの解釈・リンク・画面が同じものを使う
109. アクティビティの行は、trailer をすべて出す。タスクはあるが ID がないログ（削除済みなど）は ULID を出す
110. アクティビティの `limit` の上限は 1000（サーバーの最大）。「さらに読み込む」はそこまで増やす
111. 日時を読めないコミットは日付の見出し「日時不明」にまとめる
112. **プロジェクトの上書きの読み取り**: scoped を1件だけ返す GET の API はない。上書きは `SchemaView.scoped`（`GET /api/schema`）から読む（plan の `GET /api/schema/scoped/{project}` は実在しなかった）
113. `PUT`・`DELETE /api/schema/scoped/{project}` のあとは `SchemaCache.reload(store)` を呼び、`schema.changed` を待たずに画面を新しくする
114. すでにエントリのあるステータスへ遷移の制約・必須の項目を足すと、そのエントリに併合する（同じステータスの行を2つ作らない）
115. 必須にできる項目は `internal/schema/check.go` の規則から取る（サーバーが 422 にしないものだけを選択肢に出す）
116. プロジェクトの設定は `ConstraintEditor`（遷移の制約と必須の項目の編集）と `ProjectViolations`（いまの違反の一覧）に分ける
117. **同期のカード**: `SyncCard` は任意の `store` を受け、設定の画面では Store ごとの段に置く。窓が隠れているあいだの fetch は `max(interval, 10 分)`（サーバーの `loop.go` と同じ値）で、画面の説明文にこの値を出す
118. 設定の画面の Store の選択欄は、ナビを作り直さずに残し、選んだ Store を覚える
119. **doctor の API の形**: 応答は `DoctorReport[]`（Store ごと）。plan の `FindingsOutputBody` は実際の形ではなかった
120. 修復の応答の findings をそのまま出し、修復のあとの二度目の GET は送らない
121. finding のタスクを引く ID は `GET /api/tasks/{ulid}` を通す。severity が info の finding も扱う（警告・誤りとは別の見た目）
122. E2E の `Hedgehug.env()` は `hedgehug` を PATH に載せる（merge driver が `hedgehug` を探すため）。`HEDGEHUG_BIN` は絶対パスにする
123. **purge の日数**: テキスト欄で、全角の数字は NFKC で半角にし、0 から 36500 までの整数だけを受ける
124. **スキーマの修復後の表示**: migrate を実行した行が消えたときは、フォーカスを見出し（h1）へ移す
125. 「併合する…」は、同じ種類の定義が1つしかないときは出さない（併合先がないため）
126. ワークフローは、遷移の制約と必須の項目を「X から移せるのは A、B だけ」「X にするには P、Q が必要」の文の一覧で見せる
127. schema check の欄は `SchemaCheck.svelte` に分ける
128. CLI の表記: 概要の細部 74 の「`hh migrate …`」は実際には `hh schema migrate …`。使わない `cliInboxRead`・`cliInboxRevert`・`cliScopeShow` は作らずに外した
129. `schema.changed` が届いたら、スキーマの画面は schema check だけを取り直す（定義と件数は自分の操作のあとに取り直しているため）
