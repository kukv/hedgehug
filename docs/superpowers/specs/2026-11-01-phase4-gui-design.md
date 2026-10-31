# フェーズ 4: GUI 設計

- 日付: 2026-11-01
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」4. GUI、「GUI 仕様」、「アーキテクチャと技術選定」、「CLI 仕様」
- 画面のデザイン: Hedgehug GUI デザイン（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）。色・文字・部品・ふるまいは「実装の基準」（Foundations）のアートボードを正とする
- 前段: `docs/superpowers/specs/2026-10-31-phase2-sync-merge-design.md`（ほか 1a〜1.5 の spec）

## 位置づけ

フェーズ4は1つの spec と、小フェーズごとの plan・ブランチ・PR で作る。小フェーズはどれもその中で完結させ、持ち越さない。

| 小フェーズ | 内容 | 完了条件 |
|---|---|---|
| 4a 操作の層を移す | `cmd/hedgehug` の操作を `internal/ops` に移す。挙動は変えない | 既存の CLI のゴールデンテストとシナリオテストが、期待値を変えずに通る |
| 4b 保存済みビューと進捗 | `views/` の形式、マージ、doctor の検査、`hh view`。チェックリストとプロジェクトの進捗を ops の結果と CLI に入れる | CLI でビューの保存・一覧・表示・削除・名前の変更ができ、同期でマージされる。`hh ls --tree`・`hh show` に進捗が出る |
| 4c サーバー | REST API、SSE、同期ループ、`hh ui`、起動とセキュリティ、埋め込み、OpenAPI | Go のテストで全エンドポイントと SSE を確かめ、CLI の変更が SSE で届く |
| 4d フロントエンドの土台と中心の画面 | ツールチェーンと CI、`web/core`、見た目の基準、サイドバー、ツリー、タスク詳細、作成、コマンドパレット、異常系と空の状態 | ブラウザでツリーと詳細からタスクを作成・編集でき、CLI の変更が即時に反映される（E2E） |
| 4e 残りの画面 | 「画面と範囲」の残りすべて | 「画面と範囲」がすべて GUI で動く |
| 4f フェーズ3の画面 | フェーズ3のマージ後に、フェーズ3の機能の画面 | フェーズ3の機能が GUI でもできる |

4a は、並行して進んでいるフェーズ3より先に main に入れ、フェーズ3のブランチを rebase してもらう。キャンバスの修正は 4d の前に、親設計の修正は本 spec の承認と同時に行う。

フェーズ4全体の完了条件は親設計のとおり、「CLI でできる操作がすべて GUI でもでき、CLI での変更が GUI に即時反映される」。ただし対象は「画面と範囲」に挙げた操作で、ほかのフェーズの機能の画面は、そのフェーズの完了条件に含める（「親設計の修正」）。

### 用語

フェーズ1.5の呼び方に合わせる。人が読める識別子（`WEB-42`）は **ID**、内部の識別子は **ULID**。デザインの「キー」は ID のこと。

## 利用者の決定（2026-11-01）

- 成果物は動く GUI まで。デザインは既存のキャンバスを使い、見た目は決め直さない
- キャンバスのうち、フェーズ1.5より前の前提で描いた箇所は、spec に合わせてキャンバスも直す
- 画面は、その機能を作るフェーズで作る。フェーズ3の画面は、フェーズ3のマージ後にフェーズ4の最後（4f）で足す
- 書き込み操作を共通の internal パッケージに移す作業を、フェーズ3より先に入れる
- SPA のビルド結果はコミットせず、CI・`make build`・goreleaser でビルドする
- 変更の検知は、HEAD を短い間隔で確かめる方法にする（fsnotify は使わない）
- API は REST にし、フロントエンドとバックエンドをはっきり分ける
- OpenAPI は Go のサーバーから huma v2 で生成する。親設計の技術選定（HTTP は標準 net/http）を書き換える
- ルーターは標準の ServeMux のままにし、ブラウザを開く処理に `github.com/pkg/browser` を足す
- 保存済みビューの形式と `hh view` は本 spec のとおり。`hh view rm` / `hh view rename` を足す
- フロントエンドのフォーマットと lint は Biome にする（ESLint と Prettier は使わない）
- フェーズ4を 4a〜4f に分ける

## 範囲

### 画面と範囲

デザインのアートボードと、フェーズ4で作る範囲。「後のフェーズ」の欄は、そのフェーズで同じ画面に足す。

| アートボード | フェーズ4で作るもの | 後のフェーズ |
|---|---|---|
| Sidebar | Store の一覧（今は1つ）と表示の切り替え、保存済みビュー、プロジェクトと進捗、同期の状態と「今すぐ同期」、通知の未読数 | Store の下の計画ビュー（6）、複数 Store と閲覧のみ（3） |
| Main（ツリー） | アウトライナー、キーボードでの追加・字下げ・並べ替え・完了、ドラッグでの並べ替えと親の付け替え、フィルタ欄、ビューを保存 | — |
| Project | 子タスクの進捗（done 数 / canceled を除いた全数）と一覧 | 紐付く repo とリンク中の PR（5） |
| Kanban | ステータスまたはカテゴリを列にする、スイムレーン（担当者・プロジェクト）、ドラッグでのステータス変更 | WIP 上限と遷移の制約の表示（6） |
| Table | セルの直接編集、並び替え、グループ化、列の選択 | 複数選択での一括編集（6、`hh set --query` と一緒に作る） |
| MyTasks | `assignee:@me` のタスクを期限・プロジェクトでまとめる | Store 別のまとめと、非表示の Store の案内（3） |
| TaskDetail | プロパティ、本文（Markdown エディタ）、コメント、関連と逆方向・バックリンク、チェックリストの進捗、ウォッチ、履歴と restore --at、削除と完全削除 | PR・Issue（5）、作業時間の記録（6） |
| Inbox | 一覧、既読、上書きされた値を自分の値に戻す | 自動化の結果（5） |
| Activity | 変更のフィード、人と同期での絞り込み | 自動化での絞り込み（5） |
| ProjectSettings | `scoped` の上書き（表示するステータス、種類、プロパティ）の閲覧と編集。Store の値と並べて表示 | テンプレートと自動化の欄（6・5） |
| CreateTask / CreateProject | add、add --kind project | テンプレートの選択（6） |
| CreateStore | store init（名前・接頭辞・最初のワークスペース・remote） | — |
| Dialogs | 削除・完全削除の確認、ビューを保存、ワークスペースの追加・名前の変更・接頭辞の変更・削除、undo の確認 | — |
| CommandPalette | ID・タイトルでの検索、画面の移動、操作の実行 | — |
| States | 空の状態、読み込み中、サーバーに接続できない、同期のエラー、壊れたファイル（doctor への案内） | — |
| Onboarding | Store が1つもないときの初回起動（store init へ） | Store の発見とマウント（3） |
| SettingsSchema | スキーマの閲覧、schema check、migrate | テンプレート（6） |
| SettingsGeneral | 同期の状態と間隔、doctor と修復 | — |
| SettingsNav | 上の設定画面への移動 | アカウント・Store（3）、自動化（5） |

フェーズ4で作らないアートボード: Backlog、SprintBoard、Gantt、Release、Reports、SprintDialogs（6）、StoreMove、SettingsAccounts、SettingsStores（3。4f で作る）、SettingsAutomations（5）。

### 親設計にない追加

- 保存済みビューのファイル形式と `hh view rm` / `hh view rename`
- `ui.json` による起動中のサーバーの再利用、`Host` の検査
- views/ のマージ規則と doctor の検査

## パッケージ構成

```
internal/ops/          CLI とサーバーが共通に呼ぶ操作（4a で移す）
internal/view/         保存済みビューの形式、読み書き、検証（4b）
internal/server/       huma v2 の REST API、SSE、同期ループ、埋め込みの配信（4c）
cmd/hedgehug/          cobra、フラグ、テキストと JSON の出力、確認、$EDITOR だけ
api/openapi.yaml       サーバーから生成した OpenAPI 3.1（コミットする）
web/
  package.json         pnpm、Vite、Biome、Vitest、Playwright
  core/                フレームワークに依存しない TypeScript
  app/                 Svelte 5 のコンポーネントとビュー
  e2e/                 Playwright
  embed.go             //go:embed all:dist
  dist/                ビルド結果（gitignore）
```

## 4a 操作の層（internal/ops）

### 移すもの

`cmd/hedgehug` の次の処理を、表示から切り離して `internal/ops` に移す。

- タスク: add、set（`assign` の解釈と適用を含む）、status / done、dep、mv、rm、restore、restore --at、purge、edit の検査（`checkEdit`）、watch / unwatch
- コメント: add、edit、rm
- 読み取り: ls、ls --tree（`treeOf`）、show（コメント・関連・逆方向・バックリンク・履歴）、log
- undo、inbox（一覧・既読・revert）、sync と sync status、workspace（add・ls・rename・prefix・rm）
- schema check、migrate、doctor と修復、store init / add / list / remove
- `app` の共通処理: `config()`・`open()`・`writer()`・`accountID()`・`update()`・`resolve()`・`resolveULID()`・`queryOptions()`・`syncOptions()`

`cmd/hedgehug` に残すのは、cobra の定義、フラグの解釈、テキストと JSON の出力、確認のプロンプト、`$EDITOR` の起動だけ。

### 形

- `ops.Session` が、設定・パス・書き手・開いた Store とインデックスを持つ。CLI は1回の実行ごとに、サーバーは起動中ずっと1つ持つ
- 操作は型付きの結果を返し、何も印字しない。今の `taskJSON`・`showOf`・`treeNode`・`syncJSON` などの組み立ては、この結果の型に寄せる。CLI の JSON 出力は結果の型から作り、出力のバイト列は変えない
- 書き込み後の同期の起動（今の `afterWrite`）は、`Session` に渡す関数にする。CLI は今どおり子プロセスを起動し、サーバーは自分の同期ループに知らせる
- エラーは型で返す

| 型 | 意味 | CLI | HTTP |
|---|---|---|---|
| `NotFound` | ID・ULID・名前が見つからない | 終了コード 1 | 404 |
| `Ambiguous` | ID の指定が複数に当たる | 1 | 409 |
| `Invalid` | 検証・スキーマ違反、不正な値 | 1 | 422 |
| `Conflict` | 削除済み、親が削除済み、同期中などで今は実行できない | 1 | 409 |
| `Usage` | 引数の組み合わせの誤り | 2 | 400 |

エラーは、画面の文言を選ぶための `Code`（`task.not_found` など）と引数を持つ。CLI のメッセージは今の英文のまま変えない。

### 完了条件

- `cmd/hedgehug` の既存のテスト（ゴールデン、シナリオ、e2e）が、期待値を変えずに通る
- `internal/ops` に、移した操作ごとの単体テストを置く（今 `cmd/hedgehug` のテストでしか確かめていない分岐を、ops の層で確かめる）
- `make check` が通る

## 4b 保存済みビュー

### 形式

```yaml
# views/今週のリリース作業.yaml（ファイル名から .yaml を除いたものがビューの名前）
layout: table            # tree | kanban | table
query: "project:SH-3 -status:canceled sort:due"
table:
  columns: [id, title, status, assignee, points, due]
  group_by: parent       # none | parent | status | assignee | project
kanban:
  columns: status        # status | category
  swimlane: none         # none | assignee | project
updated: 2026-11-01T10:00:00Z
updated_by: koki
```

- 名前は 1〜64 文字。`/ \ : * ? " < > |`、制御文字、先頭と末尾の空白とドットは拒否する。大文字小文字だけが違う名前は同じ名前として拒否する（大文字小文字を区別しないファイルシステムのため）
- `table` と `kanban` は省略でき、省略時は既定値（table: `id, title, status, assignee, due` と `group_by: none`、kanban: `columns: status` と `swimlane: none`）。`layout` に関係しない節も保存してよい（レイアウトを切り替えても設定が残る）
- `columns` に書けるのは、組み込みのフィールドとスキーマのプロパティのキー。スキーマから消えた列は表示で飛ばし、doctor が警告する
- `updated` と `updated_by` は書き込むたびに更新する（マージの勝ち負けに使う）
- 未知のキーは読み飛ばし、書き戻すときに残す（ほかのファイルと同じく `yaml.Node` で扱う）
- `layout: gantt` はフェーズ6で足す

### 書き込み

1回の保存・削除・名前の変更が1コミット。件名は `hedgehug: view save <名前>` / `view rm <名前>` / `view rename <旧> <新>`、trailer は `Hedgehug-Op: view`。

### マージ

`views/` を、schema.yaml と同じキー単位のマージ（`kindYAML`）に入れる。両方の側で同じキーを違う値に変えたときは既存の LWW（`updated` → 書き手 → 値）で決め、負けた値は `Hedgehug-Overwritten` に記録する。片側が削除し、もう片側が変更したときは、変更を残す（タスクと同じ）。

### doctor

形式の誤り（未知の layout、`group_by`・`columns`・`swimlane` の不正な値、名前の規則に合わないファイル名）と、スキーマにない列を検出する。修復はしない（どの値に直すべきか決まらないため）。

### CLI

| コマンド | 内容 |
|---|---|
| `hh view save <name> [--layout] [--query] [--columns] [--group-by] [--kanban-columns] [--swimlane]` | 作成・上書き。上書きでは、指定したフラグの項目だけを変える |
| `hh view list` | 名前・レイアウト・クエリ |
| `hh view show <name>` | そのクエリで `hh ls` と同じ一覧を出す。layout が tree なら `hh ls --tree` と同じ表示 |
| `hh view rm <name>` | 削除 |
| `hh view rename <old> <new>` | 名前の変更（ファイルの移動） |

どれも `--store` と `--json` に対応する。

### 進捗

親設計のフェーズ4の項目どおり、GUI と CLI の両方に入れる。計算は ops の結果の型に置き、CLI と API で共有する。

- チェックリストの進捗: 本文の `- [ ]` と `- [x]`（`X` も可）の数。リストの項目の行だけを数え、コードブロックの中は数えない。項目が1つもなければ出さない
- プロジェクトの進捗: 子孫のうち、カテゴリが done の数 / カテゴリが canceled のものを除いた数。削除済みは数えない。プロジェクト（`kind` の role が container）だけに出す
- CLI: `hh ls --tree` の行末と `hh show` の見出しの下に `☑ 2/5`・`3/14` の形で出す。`--json` には `checklist: {done, total}` と `progress: {done, total}` を足す（該当しなければ省く）
- ゴールデンテストの期待値は、この変更の分だけ更新する

## 4c サーバー（internal/server）

### API

huma v2 を標準の ServeMux の上（humago）で使う。パスはすべて `/api` の下。

| 資源 | エンドポイント | 呼ぶ ops |
|---|---|---|
| タスク | `GET /tasks?q=&tree=`、`GET /tasks/{id}`、`POST /tasks`、`PATCH /tasks/{id}`、`DELETE /tasks/{id}` | ls / ls --tree、show、add、set・status・dep・mv・restore・本文の更新、rm |
| 完全削除 | `DELETE /tasks/{id}?purge=true` | purge |
| 履歴 | `GET /tasks/{id}/history`、`POST /tasks/{id}/history/{commit}/restore` | log、restore --at |
| コメント | `POST /tasks/{id}/comments`、`PATCH /tasks/{id}/comments/{cid}`、`DELETE /tasks/{id}/comments/{cid}` | comment add / edit / rm |
| ウォッチ | `PUT /tasks/{id}/watchers/me`、`DELETE /tasks/{id}/watchers/me` | watch / unwatch |
| 取り消し | `POST /undo` | undo（自分の直前の操作） |
| 通知 | `GET /notifications`、`PATCH /notifications/{id}`、`POST /notifications/{id}/revert` | inbox、read、revert |
| アクティビティ | `GET /activity` | log |
| ワークスペース | `GET /workspaces`、`POST /workspaces`、`PATCH /workspaces/{id}`、`DELETE /workspaces/{id}` | workspace ls / add / rename・prefix / rm |
| スキーマ | `GET /schema`、`GET /schema/check`、`POST /schema/migrate` | schema check、migrate |
| Store | `GET /stores`、`POST /stores` | store list、store init |
| 保守 | `GET /doctor`、`POST /doctor/fix` | doctor、doctor --fix |
| 同期 | `GET /sync`、`POST /sync` | sync status、sync |
| 保存済みビュー | `GET /views`、`GET /views/{name}`、`PUT /views/{name}`、`PATCH /views/{name}`（名前の変更）、`DELETE /views/{name}` | view list / show / save / rename / rm |
| 表示の状態 | `PUT /presence` | なし（同期ループの間隔を変える） |
| 状態の確認 | `GET /health` | なし |

- `{id}` は ID・以前の ID・ULID のどれでもよい（CLI と同じ解決）
- 1回の `PATCH` は1コミット。送ったフィールドだけを変える。同じフィールドを CLI と同時に変えたときは後勝ち
- 書き込みの応答は、変わったタスクと、作ったコミットのハッシュを返す
- タスクの応答には、4b の `checklist` と `progress` を含める
- エラーは problem+json。`type` に ops のエラーの `Code`、拡張メンバーに引数を入れる。表示の文言はフロントエンドが持つ
- OpenAPI 3.1 を `api/openapi.yaml` に生成してコミットする。`go generate` で作り直し、CI で差分がないことを確かめる

### SSE

`GET /api/events`。

- サーバーは約 300ms ごとに、Store ごとの HEAD・同期の状態・inbox の未読数を読む
- HEAD が動いたらインデックスを Refresh し、変わったタスクの ULID と消えたタスクの ULID を送る

| イベント | 中身 |
|---|---|
| `tasks.changed` | `store`、`ulids`、`removed`、`commit`、`op`、`author` |
| `sync.state` | `store`、最終同期時刻、未 push のコミット数、エラー、次の fetch の時刻 |
| `notifications.unread` | 未読数 |
| `schema.changed` | `store` |
| `views.changed` | `store` |

- 接続のたびに、最初に今の状態（各 Store の HEAD と同期の状態）を送る。クライアントは、切断中に HEAD が動いていたら表示中のデータを取り直す

### 同期ループ

- 親設計の既定のとおり、fetch はブラウザで表示中なら `sync.interval`（既定 2 分）、非表示なら 10 分（`sync.interval` が 10 分より長ければその値）ごとに行う。`hh ui` の起動時にも1回同期する
- 表示中かどうかは、SPA が Page Visibility API の変化を `PUT /api/presence` で知らせる。どのタブも表示中でなければ非表示とみなす
- サーバーの書き込みの後は、CLI のバックグラウンド同期と同じ規則（最初の書き込みから `sync.push_interval` 後に push）で同期する
- CLI の書き込みは、今どおり CLI の子プロセスが同期する。同時に動いても既存のロックで直列になる

### 起動とセキュリティ

- `hh ui [--port] [--no-open]`。`127.0.0.1` で待ち受ける。`--port` がなければ空いているポートを使う
- 起動ごとに 32 バイトのランダムなトークンを作り、`http://127.0.0.1:<port>/#token=<token>` を `pkg/browser` で開く。SPA はトークンを sessionStorage に移し、`history.replaceState` で URL から消す
- `/api` はすべて `Authorization: Bearer <token>` を必須にする（CSRF 対策を兼ねる）。`Host` が `127.0.0.1:<port>` と `localhost:<port>` 以外の要求は、静的ファイルも含めて拒否する（DNS リバインディング対策）
- 起動情報（ポート・トークン・pid・バージョン）を、データディレクトリの `ui.json`（権限 0600）に書き、終了時に消す。2回目の `hh ui` は、`ui.json` の情報で `GET /api/health` が通れば既存のサーバーを開いて終わり、通らなければ新しく起動して `ui.json` を書き直す
- サーバーは前面で動き、Ctrl-C（Windows では Ctrl-Break も）で止まる。止まるときは同期ループの push を待つ（最大 10 秒）

### 埋め込み

- `web/embed.go` が `//go:embed all:dist` でビルド結果を埋め込み、`http.FileServerFS` で配る。`/api` 以外の未知のパスには `index.html` を返す
- dist が空のビルド（Node なしの `go build`・`go test`）では、`/` に「GUI がビルドされていません。make web を実行してください」のページを返す。API はそのまま動く

### テスト

- 全エンドポイントを `httptest` で確かめる（正常系、エラーの型と HTTP ステータス、トークンと Host の拒否）
- 応答が `api/openapi.yaml` のスキーマに合うことを確かめる
- SSE: CLI 相当の書き込み（ops の呼び出しと、別プロセスの `hh` の実行）で `tasks.changed` が届くこと、切断と再接続
- 同期ループ: 表示・非表示での間隔、書き込み後の push、停止時の push 待ち（時計は差し替えて確かめる）
- `ui.json`: 再利用、古い `ui.json` の上書き、権限

## 4d・4e フロントエンド（web/）

### web/core

フレームワークに依存しない TypeScript の層。

- API クライアント: `openapi-fetch`。型は `api/openapi.yaml` から `openapi-typescript` で生成する
- SSE: fetch でストリームを読み、トークンをヘッダで送る。切れたら間隔を延ばしながら再接続し、再接続後は表示中のデータを取り直す
- タスクのキャッシュ: ULID をキーにした正規化キャッシュ。`tasks.changed` で該当の ULID だけを取り直す。自分の書き込みのコミットは、応答のハッシュと照合して見分け、それ以外の変更は10秒間「CLI で更新 · 今」の印を付ける
- 保存の制御: 入力が 2 秒止まったとき、またはフォーカスが外れたときに `PATCH` する。保存中・保存済み・push 待ちの状態を持つ
- `hh` コマンドの組み立て: フィルタ欄とダイアログの足元に出す CLI の文字列
- エラーの文言: problem+json の `type` と引数から、日本語の文言を作る
- 表示の状態: Page Visibility API の変化を `PUT /api/presence` で知らせる

### web/app

- Svelte 5（runes）、Vite でビルドする。SvelteKit は使わない。web/core を購読して描画するだけにする
- ルーティング: History API の上に自前で書く（画面は 15 前後で、入れ子のレイアウトがないため）
- テーブルは TanStack Table（Svelte 5 アダプタ）、長い一覧は TanStack Virtual、本文とコメントのエディタは CodeMirror 6、ドラッグは Pragmatic drag and drop
- uPlot はレポート（フェーズ6）まで入れない
- TanStack Table の Svelte 5 アダプタ（v9）が安定版でないとき、Biome の `.svelte` のテンプレートの扱いが足りないときは、4d の plan を書く時点で利用者に相談する

### 見た目

- `web/app/styles/tokens.css`: Foundations の色・文字・間隔・角丸を CSS 変数にする。コンポーネントは 16 進数を直接書かない
- ステータスと種類のアイコンは Foundations の形を Svelte のコンポーネントにする。ステータスは色だけでなく形で区別する
- フォント（IBM Plex Sans JP、JetBrains Mono）は `@fontsource` で同梱する。外部に通信しない
- ダークモードは作らない（デザインにない）
- キーボード: ⌘K コマンドパレット、C タスクを作成、⇧⌘S 今すぐ同期、Enter 下に追加（ツリー）、Tab / ⇧Tab 字下げ・戻す、⌥↑↓ 並べ替え、Space 完了、S ステータス、K 種類、⌘↵ 詳細を開く・作成。Windows と Linux では ⌘ を Ctrl にする
- ふるまいは Foundations のとおり（拒否はドラッグ中に赤枠、離したらトースト。警告は黄色で隣に直す操作。トーストは左下に 6 秒、取り消し付きは 10 秒。取り消しにくい操作だけ確認ダイアログ）

### ID の表示

デザインのうち、フェーズ1.5より前の前提の表示は使わない。ID は作成時に振られているので、「キー未確定」のチップ、短縮 ID とその点線の下線、「次の同期で確定」は作らない。ID の先頭表示はデザインのとおりで、ULID はタスク詳細と、ID にカーソルを合わせたときに出す。

### ツールチェーンと CI

- pnpm、Vite、Biome（フォーマットと lint。`biome ci` を CI で実行）、`svelte-check`（警告ゼロ）、Vitest（core の単体テストと `@testing-library/svelte` のコンポーネントのテスト）、Playwright
- `make web` で SPA をビルドする。`make build` と goreleaser は先に `make web` を実行する。`make check` に web の検査を足す
- CI に web のジョブを足す（install → biome ci → svelte-check → vitest → build）。Go のジョブは dist なしで通る
- E2E は Linux のジョブで、web をビルドしたバイナリを使う。一時 Store を作って `hh ui --no-open` を起動し、主な流れを確かめる: 作成、編集と自動保存、ツリーのドラッグ、カンバンの移動、CLI の変更が SSE で反映されること、通知の既読と revert、保存済みビュー、undo、削除と復元

## キャンバスの修正

4d の前に、Hedgehug GUI デザインのキャンバスを直す。

- 「キー未確定」のチップ、短縮 ID（`01JR` など）と点線の下線、「キーは次の同期で確定します」「次の同期でキーが確定します」を消す（Sidebar、MyTasks、Main、Kanban、Table、Backlog、CommandPalette、States、StoreMove、Foundations）
- Activity の「キーを確定 · Hedgehug-Op: number」を、ID の振り直しの実際の記録に直す
- 呼び方の「キー」を「ID」にする（Sidebar の「キー・検索・コマンド」など）。スキーマのキーなど、ID でない「キー」はそのまま
- `former_keys` を `former_ids` にする

## 親設計の修正

本 spec の承認と同時に、Hedgehug 設計ドキュメントを直す。

- 「パッケージ構成」に `internal/ops`・`internal/view` を足し、`internal/server` と `web/` の配置を本 spec に合わせる
- 「技術選定」の HTTP を「huma v2（標準 net/http の ServeMux の上）。OpenAPI 3.1 を生成する」にし、`pkg/browser`、Biome、openapi-typescript を足す
- 「データ形式」に `views/<name>.yaml` の形式を足し、「フィールドごとのマージ規則」に views を足す
- 「コマンド一覧」の `hh view` に `rm` と `rename` を足す
- 「実装フェーズ」: フェーズ4の「全ビュー」を本 spec の「画面と範囲」にし、4a〜4f を書く。フェーズ3・5・6の項目に、それぞれの画面（本 spec の「後のフェーズ」）を足す
- 「GUI 仕様」: 起動情報の `ui.json`、`Host` の検査、HEAD のポーリングによる変更の検知、SSE のイベント、表示の状態による同期の間隔を書く
