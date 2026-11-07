# フェーズ 6: 計画とレポートの設計

- 日付: 2026-11-05
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」6. 計画とレポート、「計画とアジャイル」、「データ形式」の「テンプレート」「作業時間の記録」「プロジェクトごとの上書き」
- 画面のデザイン: Hedgehug GUI デザイン（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）
- 前段: `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`、`docs/superpowers/specs/2026-11-05-phase5-github-design.md`

## 位置づけ

フェーズ6は1つの spec と、小フェーズごとの plan・ブランチ・PR で作る（2026-11-05 に利用者が決定）。

| 小フェーズ | 内容 | 完了条件 |
|---|---|---|
| 6a 計画の土台 | `hh sprint`・`hh release`・`hh worklog`、`sprint:`・`release:`、`hh set` の `sprint`・`releases`、`TaskView` の項目、API | CLI だけでスプリントを作り、タスクを入れ、開始して閉じられる。API から同じ操作ができる |
| 6b テンプレートと一括更新 | `hh add --template`、種類の既定テンプレート、`hh template`、scoped の `templates`、`hh set --query`、API | 子タスクを含むテンプレートを CLI と API で展開できる。一致するタスクを1コミットで更新できる |
| 6c レポート | `hh report`、集計とキャッシュ、API | 閉じたスプリントのバーンダウンとベロシティが git 履歴から正しく出る（シナリオテスト） |
| 6d 計画の画面 | バックログ、スプリントボード、リリース、スプリントの開始と終了、作業時間、テンプレートの選択と設定、テーブルとパレットの一括編集、WIP 上限 | 下の「画面」の 6d の操作がブラウザで動く（E2E） |
| 6e ガントとグラフ | `layout: gantt`、ガントとロードマップ、レポートの画面 | 下の「画面」の 6e の操作がブラウザで動き、フェーズ6の CLI の操作が GUI でもできる（E2E） |

フェーズ6全体の完了条件は親設計のとおり、「スプリントの開始から終了までを Hedgehug だけで回せ、閉じたスプリントのバーンダウンとベロシティが git 履歴から正しく出る」こと。

親設計がフェーズ6に挙げる「ワークフローの制約」は、4e の `hh schema scope` と書き込みの検査、カンバンの `canTransition` で作った。6d の plan で、カンバンに足りない遷移の表示がないかを確かめ、あればそこで作る。

## 今あるもの

| 領域 | ある | ない |
|---|---|---|
| スプリント・リリース | `model.Sprint`・`model.Release`、`store` の読み書き、`sprint_refs` の検査、ワークスペースの移動で外す処理、マージ、`hh doctor` の検査 | usecase、CLI、API、クエリ、画面 |
| 作業時間 | `model.Worklog`、`store` の読み書き、index、履歴、inbox のパス判定 | usecase、CLI、API、画面 |
| テンプレート | `model.Template`、`store` の読み取り、`Effective.Templates()`、`schema_findings` の検査 | 書き込み、展開、CLI、API、画面 |
| ビュー | `tree`・`kanban`・`table` | `gantt`、WIP 上限 |
| 履歴 | `history` の索引（コミットごとの変更ファイルと trailer） | タスクの版ごとの状態の集計 |

## パッケージ

構成見直しの配置に足す。cli と server は usecase だけを通す。

| 場所 | 置くもの |
|---|---|
| `domain/report`（新規） | タスクの版の列から日ごとの値を求める計算。入出力は値だけで、I/O を持たない |
| `storage/reportcache`（新規） | 版ごとの状態のキャッシュ。`.hedgehug/local/report` に置く |
| `usecase/planning`（新規） | スプリント、リリース、`sprint:`・`release:` の解決に使う名前の引き当て |
| `usecase/worklogs`（新規） | 作業時間の記録と削除 |
| `usecase/templates`（新規） | テンプレートの一覧、読み書き、展開 |
| `usecase/reports`（新規） | 履歴の読み出しとレポートの組み立て |
| `usecase/tasks` | `hh set` の `sprint`・`releases`、一括更新 |

4つの新しい領域を足すので、構成見直しの spec の 2.3 の領域の数を 14 から 18 に直す。

## 6a 計画の土台

### スプリントのコマンド

| コマンド | 動き |
|---|---|
| `hh sprint add <name> [--start <日付>] [--end <日付>] [--goal <文>] [--workspace <w>]` | `planned` で作る |
| `hh sprint ls [--state <状態>] [--workspace <w>]` | 既定はコンテキストのワークスペース。`--workspace` なしでコンテキストがなければ全ワークスペース |
| `hh sprint set <name> key=value...` | `name`・`start`・`end`・`goal` を変える。`state` は変えられない |
| `hh sprint start <name>` | `planned` を `active` にする。`start`・`end` がなければエラー |
| `hh sprint close <name> [--to next\|backlog\|<名前>]` | `active` を `closed` にする |
| `hh sprint rm <name>` | 参照しているタスクから外して消す。1コミット |

- 状態の遷移は `planned → active → closed` だけ。閉じ間違いは `hh undo` で戻す
- 閉じるとき、done・canceled カテゴリでないタスクを移し先へ移す。`next` はそのワークスペースで `start` が最も早い `planned` のスプリント（`start` が同じなら ULID の順）。`backlog` は `sprint` を外す
- `--to` がないとき、端末なら移し先を聞き、端末でなければエラーにする。未完了のタスクがなければ聞かない
- `--to next` で次のスプリントがなければエラーにして、何も変えない
- 閉じる処理は全部で1コミット。結果に移したタスクの件数と移し先を出す
- `end < start` の値は書き込みで拒否する
- 名前はワークスペースの中で一意（親設計どおり）。`hh doctor` の検査に合わせ、大文字小文字だけが違う名前も同じ名前として拒否する
- 名前の引数は `--workspace`、コンテキストの順で探す。どちらでも決まらず複数のワークスペースに同じ名前があれば、候補を挙げてエラーにする

### リリースのコマンド

| コマンド | 動き |
|---|---|
| `hh release add <name> [--date <日付>] [--workspace <w>]` | `unreleased` で作る |
| `hh release ls [--state <状態>] [--workspace <w>]` | 一覧。件数と done の件数を出す |
| `hh release set <name> key=value...` | `name`・`date`・`state` を変える |
| `hh release notes <name>` | 親設計どおり。done のタスクを種類ごとにまとめた Markdown を標準出力に出す |
| `hh release rm <name>` | 参照しているタスクから外して消す。1コミット |

リリースの状態はどの方向にも変えられる（間違えて `released` にしたときに戻すため）。

### 作業時間のコマンド

- `hh worklog <id> <時間> [メモ] [--date <日付>]`: `--date` の既定は今日、`author` は「自分」
- `hh worklog ls <id>`: 記録の一覧。削除済みは出さない
- `hh worklog rm <id> <記録の ULID>`: 論理削除（`deleted_at`）。コメントと同じく自分の記録だけ消せる

編集は作らない。消して入れ直す。`hh show` と `show --json` に合計と件数を足す。

### タスクの項目

- `hh set <id> sprint=<名前>`、`sprint=`、`releases=<名前>`、`releases+=<名前>`、`releases-=<名前>` を足す。名前はタスクのワークスペースの中で引く
- `hh add` に `--sprint`・`--release` を足す
- `TaskView` に `sprint`（`{ulid, name, state}`）と `releases`（`[{ulid, name, state}]`）を足す
- `PATCH /api/tasks/{id}` に `sprint`・`releases` を足す。値は ULID
- ワークスペースの移動で外れたときの `detached` は今の実装のまま使う。名前が引けるようになるので、事前の一覧が空でなくなる

### クエリ

`sprint:current`・`sprint:next`・`sprint:none`・`sprint:<名前>`・`release:<名前>` を足す。

- `sprint:current` は、各ワークスペースの `active` なスプリントのどれかに入っているタスクに当たる
- `sprint:next` は、各ワークスペースの上の `next` に入っているタスクに当たる
- `sprint:<名前>` と `release:<名前>` は、タスクのワークスペースのスプリントとリリースの名前で比べる。どのワークスペースにもない名前は、今の `label:` と同じく一致なしとして扱う
- index にスプリントとリリースの名前と状態を持たせ、評価でファイルを読まない

### API

| メソッドとパス | 内容 |
|---|---|
| `GET /api/sprints`、`POST /api/sprints` | 一覧（`store`・`workspace`・`state`）、作成 |
| `PATCH /api/sprints/{id}`、`DELETE /api/sprints/{id}` | 変更、削除 |
| `POST /api/sprints/{id}/start`、`POST /api/sprints/{id}/close` | 開始、終了（本文は `to`） |
| `GET /api/releases`、`POST /api/releases`、`PATCH /api/releases/{id}`、`DELETE /api/releases/{id}` | リリースの一覧、作成、変更、削除 |
| `GET /api/releases/{id}/notes` | リリースノートの Markdown |
| `GET /api/tasks/{id}/worklogs`、`POST /api/tasks/{id}/worklogs`、`DELETE /api/tasks/{id}/worklogs/{log}` | 作業時間 |

`GET /api/workspaces` の件数に `sprints`・`releases` を足す。削除の無効の理由は 6d で画面に出す。

### Hedgehug-Op

`sprint-add`・`sprint-set`・`sprint-start`・`sprint-close`・`sprint-rm`・`release-add`・`release-set`・`release-rm`・`worklog-add`・`worklog-rm` を足す。`hh undo` がこれらのコミットを戻せることを確かめ、足りなければ 6a で直す。

## 6b テンプレートと一括更新

### 展開

- `hh add --template <name> [タイトル]`: テンプレートの項目と子タスクを作る。全部で1コミット
- タイトルは引数、テンプレートの `title` の順。どちらもなければエラー
- `hh add` で種類に既定のテンプレート（`Effective.Templates()` の `defaults`）があれば展開する。`--no-template` で外す
- プロジェクトの `only` にないテンプレートは、そのプロジェクトの配下には展開できない
- `assignees: inherit` は親の担当者を受け継ぐ。親がなければ空
- `sprint`・`releases` が展開先のワークスペースのものでなければ外し、結果に名前で出す
- `blocked_by` が Store にないタスクを指していれば、書き込みの検査で拒否する
- 親子の入れ子の順はテンプレートの `children` の順

### テンプレートの編集

- `hh template ls`・`hh template show <name>`・`hh template edit <name>`・`hh template rm <name>`
- `edit` は `$EDITOR` でファイルの形のまま編集する。なければ新しく作る。検査に失敗すれば編集に戻す（`hh edit` と同じ）
- 消すときに `kinds` の `template` や scoped の `templates` が指していれば拒否する
- `hh schema scope set` に `--templates <名前,...>`・`--template-default <種類>=<名前>` を足す（`--statuses` などと同じく、指定した部分だけ置き換える）
- Op は `template-set`・`template-rm`

### 一括更新

- `hh set --query '<クエリ>' key=value... [--dry-run] [--yes]`
- 全件を先に検査し、1件でも違反があれば何も書かず、違反を一覧する
- 実行の前に件数を出して確認する。端末でなければ `--yes` を必須にする。`--dry-run` は一覧だけを出す
- Store ごとに1コミット。閲覧のみの Store のタスクが含まれていれば、書き込む前に拒否する
- 一致が0件なら何もしない。Op は `bulk-set`、件名は `hedgehug: bulk-set <件数> tasks <key=value>`
- タスクごとの `Hedgehug-Task` を trailer に並べ、inbox と履歴が各タスクの変更として読めるようにする

### API

| メソッドとパス | 内容 |
|---|---|
| `GET /api/templates`、`GET`・`PUT`・`DELETE /api/templates/{name}` | テンプレートの一覧と読み書き。`PUT` の本文はファイルの内容 |
| `POST /api/tasks` の `template` | テンプレートの展開 |
| `POST /api/tasks/bulk` | 本文は `ulids` か `query` のどちらかと、`PATCH /api/tasks/{id}` と同じ項目、`dry_run`。応答は対象、違反、結果 |

## 6c レポート

### 版の読み出し

trailer にはステータスもスプリントも入っていないので、trailer だけでは集計できない。タスクのファイルの各版を読む。

- `history` の索引で `tasks/` のパスを列挙し、パスごとのコミットを取る
- 各コミットのファイルを読み、次の値だけを取り出す: `status`・`sprint`・`points`・`estimate`・`deleted_at`・`workspace`・`created`・`kind`。プロパティ名の `points`・`estimate` は既定のスキーマの名前で固定する
- 転送ファイルの版は「移動した」印にする。読めない版は飛ばし、結果に警告を出す
- 取り出した値を `.hedgehug/local/report` にコミットとパスを鍵で残し、次からは新しいコミットの分だけ読む
- キャッシュの形式が違う、または HEAD の履歴にないコミットの行があれば、捨てて作り直す（`history` の索引と同じ扱い）
- 1タスクの版の順は、`history` の並び（author time、世代、ハッシュ）の古い順

### スプリントの範囲

スプリントのファイルの履歴から、`state` が `active` になった最初のコミットの時刻を開始の時刻、`closed` になった最後のコミットの時刻を終了の時刻とする。

- **初期の範囲**: 開始の時刻にそのスプリントにいたタスク
- **途中で増えた範囲**: 開始の後に入ったタスク。日ごとに分けて出す
- **外れたタスク**: 終了の前に外れたタスクは、外れた日から範囲に数えない。閉じるときに移されたタスクは終了の時点まで範囲に数える
- **完了**: 最後に done カテゴリに入った時点。開き直せば完了から戻す
- **canceled**: canceled カテゴリに入った時点で範囲から除く
- **削除**: `deleted_at` が入った時点で範囲から除く
- **ベロシティ**: 終了の時刻に done カテゴリのタスクの `points` の合計。`points` のないタスクは0として、件数を併記する。閉じていないスプリントは出さない

開始の時刻がない（ファイルを手で `closed` にした）スプリントは `start` の日の 00:00 を開始、`end` の日の 24:00 を終了として数える。日の区切りは表示する人のローカルのタイムゾーン。

### カテゴリの判定

ステータスのカテゴリは今のスキーマで判定する。スキーマから消えたステータスは `unknown` にする。バーンダウンでは未完了に数え、累積フロー図では `unknown` の列に出す。

### Store 間の移動

マウント中の Store の転送ファイルから、同じ ULID の移動元の版の列をつなぐ。サイクルタイムとリードタイムはつないだ列で計算する。移動元がマウントされていなければ、手元の列だけで計算し、結果の `warnings` に出す。

スプリントはワークスペースに属し、Store 間の移動で外れるので、バーンダウンとベロシティは移動元の Store の中で閉じる。

### コマンド

| コマンド | 内容 |
|---|---|
| `hh report burndown [スプリント] [--by points\|count]` | 既定はコンテキストの `active` なスプリント、1つに決まらなければエラー。日ごとの残り、完了、増えた範囲 |
| `hh report velocity [--last <n>] [--workspace <w>]` | 閉じたスプリントごとの完了の `points` と件数。既定は最新の 10 件 |
| `hh report cfd [--since <日付>] [--until <日付>] [--by category\|status] [--workspace <w>]` | 日ごとの件数。既定は過去 30 日のカテゴリ別 |
| `hh report cycle [--since <日付>] [--until <日付>] [--workspace <w>]` | 期間に完了したタスクのサイクルタイムとリードタイムの中央値、p85、件数 |
| `hh report estimate [--workspace <w>] [--sprint <名前>]` | `estimate` と作業時間の合計の比較 |

- 既定の出力は表に文字のバーを添える（2026-11-05 利用者が決定）。パイプ先では色を付けない
- `--json` で全コマンドの値を出す
- サイクルタイムは最初に doing カテゴリに入ってから最後に done に入るまで、リードタイムは `created` から最後に done に入るまで
- `--by` の既定は `points`。`points` を持つタスクが範囲に1つもなければ、エラーにせず件数で出して注記する

### API

`GET /api/reports/burndown`・`/velocity`・`/cfd`・`/cycle`・`/estimate`。クエリの引数は CLI と同じ。`--json` と同じ型を返す。キャッシュの作り直しは要求の中で行い、画面は読み込み中の表示で待つ。

## 画面

計画の画面はワークスペースのタブに置く。

**タブの出し方**（2026-11-05 利用者が決定、2026-11-08 に変更）: バックログ、スプリント、リリースのタブは常に出す。レポート、ガントのタブは、そのワークスペースにスプリントかリリースが1件以上あるときだけ出す。最初の1件は、スプリント・リリースのタブの見出しのボタンとコマンドパレットの「スプリントを作る」「リリースを作る」から作る（2026-11-08 までの、ワークスペースの設定の「計画」の節は、設定に置くとわかりにくいためなくした）。

### 6d

- **バックログ**: `sprint:none` で todo・doing カテゴリのタスクを rank 順に並べる。右にスプリントの一覧と `points` の合計を出し、ドラッグでスプリントに入れる
- **バックログの並べ替え**（2026-11-05 利用者が決定）: rank は兄弟の間の順序だが、バックログでは親をまたいで並べる。ドラッグでは親を変えず、隣のタスクの rank の間に置く。そのためツリーの順序も変わることがある
- **スプリント**: 一覧、作成、編集、開始、終了。終了のダイアログで未完了の件数と移し先を選ぶ。active なスプリントのカンバンは `sprint:current` で絞る。上部に残り日数を出す（バーンダウンのグラフは 6e）
- **リリース**: 一覧、進捗、含まれるタスク、状態の変更、リリースノートの表示とコピー
- **タスク詳細**: スプリントとリリースの欄、作業時間の欄（一覧、追加、削除、`estimate` との比較）
- **必須の項目**: `RequiredDialog` でスプリント・リリース・リンクを「常に欠けている」扱いにしていたのを、API の値で判定する
- **テーブルの一括編集**: 複数選択と、選んだタスクへの1つの項目の変更。`POST /api/tasks/bulk`
- **パレット**: 「一致する n 件をまとめて更新…」。ダイアログで項目と値を選び、件数を出して実行する
- **カンバンの WIP 上限**: ビューの `kanban.wip`（列の名前 → 上限）。列の見出しに件数と上限を出し、超えたら警告色にする。移動は拒否しない
- **テンプレート**: 作成ダイアログで選ぶ。設定にテンプレートの一覧と編集（ファイルの形のまま編集し、保存の検査の結果を出す）を足す。スキーマの種類に既定のテンプレート、プロジェクトの設定に使うテンプレートの絞り込みを足す
- **ワークスペースの削除**: スプリントとリリースの件数を無効の理由に足す。文言は「タスク n 件（削除済み m 件を含む）、スプリント a 件、リリース b 件が残っているため削除できません」

### 6e

- **ガント**: 親設計のとおり SVG で自前で描く。左にツリー、右にバー、依存の矢印、スプリントの帯、リリースの縦線。バーのドラッグはドロップで1コミット。プロジェクトのタブにも置く（出す条件はワークスペースと同じ）
- **ビュー**: `layout: gantt` と `gantt:` の節（`start`・`end` のプロパティ名、`roadmap: true` で container 役割だけに絞る）を足す
- **レポート**: バーンダウン、ベロシティ、累積フロー図は uPlot で描く。サイクルタイムは数値のカード、見積もりと実績は行で出す。スプリントボードの上部にバーンダウンを足す

## テスト

- **`domain/report`**: 版の列を手で組み、開始後の追加、外れたタスク、開き直し、canceled、削除、手で閉じたスプリントを表形式で確かめる。日付の境界はタイムゾーンを固定して確かめる
- **`storage/reportcache`**: 壊れたキャッシュ、形式の違い、履歴の書き換えで作り直すこと、新しいコミットの分だけ読むこと
- **usecase と CLI**: 新しいコマンドのゴールデンテスト。`cli/scenario` に、スプリントの作成から開始、タスクの完了、終了、次のスプリントへの持ち越し、バーンダウンとベロシティの確認までの通しのテストを足す。PATH から git を外した CI ジョブでも通す
- **マージ**: スプリントの閉じる操作とタスクの編集の同時更新で、今のプロパティテストの不変条件が崩れないことを確かめる
- **性能**: 既存の10万コミットの生成器で、キャッシュの初回と2回目の `hh report velocity` の時間を測る。目標は2回目が1秒以内
- **web**: vitest と Playwright の E2E

## 親設計の修正

本 spec の承認と同時に、Hedgehug 設計ドキュメントを直す。

- 「スプリント」に状態の遷移、`--to`、`next` の定義を足す
- 「リリース」に状態の変更と削除を足す
- 「コマンド一覧」に `hh sprint set/rm`、`hh release set/rm`、`hh worklog ls/rm`、`hh template`、`hh report estimate`、`hh set --query` の `--dry-run`・`--yes`、`hh add --no-template`・`--sprint`・`--release`、`hh schema scope set` の `--templates`・`--template-default` を足す
- 「レポート」に版の読み出し、範囲の定義、カテゴリの判定を足す
- 「ボードと一括操作」に `kanban.wip` と、一括更新の検査と Store ごとのコミットを足す
- 「ガントとロードマップ」に `gantt:` の節を足す
- 「GUI 仕様」に、計画のタブを出す条件を足す
- 「実装フェーズ」の 6 に小フェーズと、ワークフローの制約が 4e で済んだことを足す
