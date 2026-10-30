# フェーズ 1a: model とファイル形式 設計

- 日付: 2026-10-30
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」1. Core とファイル形式
- GUI デザイン: https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L

## 位置づけ

フェーズ1を次の4つに分け、本書は 1a だけを扱う。

| 段階 | 内容 |
| --- | --- |
| 1a（本書） | 型・ファイル形式の codec・ULID・rank・パス・スキーマ検証 |
| 1b | Store のディスク読み書き、原子的な書き込み、ロック、go-git での1操作1コミット |
| 1c | インデックス、短縮 ID、クエリの基本、循環と Store 外参照の検出 |
| 1d | CLI コマンド群、devenv、`HEDGEHUG_HOME` |

1a は純粋なコードだけで構成する。ディスク I/O、ロック、git には触れない。すべて単体テストとゴールデンテストで検証できることを目標にする。

## パッケージ構成

```
internal/model/
  id.go           ID（ULID のラッパー）
  rank.go         Rank（fractional index）
  path.go         ID と Store 内の相対パスの相互変換
  frontmatter.go  "---" による frontmatter と本文の分割・結合、CRLF→LF
  node.go         yaml.Node の保持と書き戻し（既知キーの並べ替え、値 Node の差し替え）
  task.go         Task
  comment.go      Comment
  worklog.go      Worklog
  sprint.go       Sprint
  release.go      Release
  template.go     Template
  storemeta.go    StoreMeta（.hedgehug/store.yaml）
  duration.go     Duration
  errors.go       DecodeError
internal/schema/
  schema.go       Schema と codec、Default / Dev
  check.go        スキーマ自体の検査
  effective.go    プロジェクトの実効スキーマ
  validate.go     タスクの検査
```

依存ライブラリは `go.yaml.in/yaml/v4` と `github.com/oklog/ulid/v2` だけ。`model` は `schema` に依存しない。

`go.yaml.in/yaml/v4` は 2026-10-30 時点で `v4.0.0-rc.6` までで、正式版が出ていない。実装の最初に、`yaml.Node` の API が案A の前提（コメントとキー順の保持、値 Node の差し替え）を満たすことを確かめる。

## YAML の往復（案A: 型付き struct ＋元の Node を保持）

- 各ファイル型は `Decode([]byte) (*T, error)` と `(*T) Encode() ([]byte, error)` を持つ。
- `Decode` は frontmatter を `yaml.Node` として保持したまま、既知のフィールドを struct に写す。Node は非公開フィールドに持ち、呼び出し側からは見えない。
- `Encode` は struct の値を保持中の Node に反映する。
  - 既知キーは値の Node だけを差し替える。キーに付いたコメントは残る。
  - 既知キーは型ごとに決めた順に並べ、未知のキーは元の順のまま既知キーの後ろに置く。
  - 元の Node にないキーは、値がゼロ値でなければ追加する。元の Node にあったキーは、値が空になっても残す（`null` や `[]` を書く）。
  - 新規作成した値（Node を持たない）は空の Node から始める。
- 未知のキーは `Extra`（キーの順序付きで値 Node を持つ）として公開する。フェーズ2のマージが未知のキーも片側の変更を取り込めるようにするため。値 Node どうしを比べる補助関数（`NodeEqual`）もあわせて持つ。
- `Decode` が必須とするのは `id` だけ（テンプレートと `store.yaml` を除く）。`title` などの欠けは検証の層で扱う。フェーズ3の転送ファイル（`id` と `moved_to` だけを持つタスクファイル）を読めるようにするため。
- 空行など Node で保持しきれない部分は、ゴールデンテストで出力を固定する。
- 本文は Markdown のまま `Body string` に持つ。読み込み時に CRLF を LF にそろえる。

## ファイル形式

### 共通

- 日時は RFC 3339 でオフセット付き（`2026-10-29T10:00:00+09:00`）。日付は `YYYY-MM-DD`。
- `tasks/`・`comments/`・`worklogs/`・`sprints/`・`releases/` の Markdown ファイルはすべて `id`・`created`・`updated` を持つ。テンプレートは `id` を持たず（名前がファイル名で決まる）、`created`・`updated` を持つ。`updated` は親設計のマージ規則（テンプレートはフィールドごとに LWW）に使う。`store.yaml` はどれも持たない。`updated` はフェーズ2の LWW に使う。
- 論理削除できるものは `deleted_at` を持つ。

### タスク `tasks/<a>/<b>/<ULID>.md`

既知キーとその書き出し順:

```
id, key, former_keys, key_prefix, kind, title, parent, rank, status,
assignees, watchers, labels, repos, blocked_by, sprint, releases, links,
props, gh, created, created_by, updated, deleted_at
```

- `key_prefix` は container 役割のタスクが持つ接頭辞。親設計の例には出てこないため、`former_keys` の直後に置く。
- `props` は型を決めずに、キーの順序付きで値 Node を持つ（`Props`）。型の解釈は `schema` が行う。
- `gh` は読み取り専用の名前空間。1a では Node のまま往復させるだけで、型はフェーズ5で付ける。
- `links` の要素は `{type, ref, sync}`。
- 転送ファイル用の `moved_to` は Store 間の移動（フェーズ3）で決める。それまでは未知キーとして保持される。

### コメント `comments/<a>/<b>/<タスクULID>/<ULID>.md`

frontmatter は `id, author, created, updated, deleted_at`。本文がコメントの内容。どのタスクのコメントかはパスで決まるので、タスク ID は持たない。

### 作業時間 `worklogs/<a>/<b>/<タスクULID>/<ULID>.md`

frontmatter は `id, author, date, duration, created, updated, deleted_at`。本文がメモ。

### duration 型

単位は `h` と `m` だけ（`6h`、`1h30m`、`45m`）。`d` や `w` は8時間か24時間かが曖昧になるので受け付けない。書き出しは `1h30m` の形に正規化する（`90m` は `1h30m` になる）。

### スプリント `sprints/<ULID>.md`

`id, name, start, end, state, goal, created, updated`。`state` は `planned / active / closed`。本文はメモ。

### リリース `releases/<ULID>.md`

`id, name, date, state, created, updated`。`state` は `unreleased / released / archived`。本文はメモ。

### テンプレート `templates/<name>.md`

タスクの frontmatter から `id`・`key`・`former_keys`・`parent`・`rank`・`created_by`・`deleted_at`・`gh` を除き（親と並び順は展開時に決まる）、`children` を足した形。`created`・`updated` はテンプレート自体の日時として持つ（`children` の要素は持たない）。

```markdown
---
kind: task
labels: [release]
props: {priority: high}
children:
  - {title: 切り替え前日の確認, assignees: inherit}
  - {title: 切り替え作業, assignees: inherit, props: {points: 2}}
  - {title: 切り替え後の監視（24 時間）}
---
## 手順
```

- `children` の要素はテンプレートと同じ形で、入れ子にできる。要素の `body` に本文を書ける。
- `assignees: inherit` は、展開時に親の担当者を引き継ぐ指定。
- テンプレート名はファイル名で決まる。

### `.hedgehug/store.yaml`

`id, name, format, key_prefix, audience`。

- `format` は整数で、1a では `1`。自分の対応より新しい値なら、`Decode` の時点でエラーにする（「Hedgehug を更新してください」）。
- `audience` は表示用の自由記述（例: 「backend チーム」）。GUI の Store 一覧に出す。

### 1a で扱わないファイル

- `views/<name>.yaml`: フェーズ4で決める。
- `automations.yaml`: フェーズ5で決める。

## ID・パス・rank

### ID

- `oklog/ulid/v2` の `ulid.Make()` で生成する（プロセス内で単調増加、乱数は crypto/rand）。
- 文字列は大文字の Crockford base32（26文字）。`Parse` は小文字も受け付ける。
- 短縮 ID は 1c で扱う。

### パス

- `TaskPath(id)` は `tasks/<末尾1文字>/<末尾から2文字目>/<ULID>.md`。コメント・作業時間も同じ規則でタスク ULID から分ける。
- `SprintPath`・`ReleasePath` は `sprints/<ULID>.md`・`releases/<ULID>.md`、`TemplatePath(name)` は `templates/<name>.md`。
- ディレクトリ名は ULID の大文字をそのまま使う。大文字小文字を区別しない FS でも衝突しない。
- 逆変換の `ParseTaskPath(path) (ID, error)` などを持つ。1b の走査とフェーズ2の merge driver が使う。

### rank

- rocicorp の fractional-indexing 形式を採用する（先頭1文字が整数部の長さを表し、base62 の小数部が続く。初期値は `a0`）。
- 文字は `0-9A-Za-z` で ASCII 順。文字列のバイト比較が並び順になる。
- API は `Between(a, b Rank) (Rank, error)`（空は端を表す）と `NBetween(a, b Rank, n int) ([]Rank, error)`。
- 自前で移植し、元実装のテストベクタで互換性を確かめる。
- 同じ rank の扱い（ID 順で並べて次の書き込みで振り直す）はフェーズ2で作る。

## スキーマ

### codec と既定値

- `schema.yaml` も案A の方式で読み書きする。対象は `statuses`・`kinds`・`props`・`workflow`・`scoped`。
- 各要素が持つフィールド:
  - ステータス: `name`・`label`・`category`・`color`・`default`。並び順は配列の順。
  - 種類: `name`・`label`・`role`・`parents`・`icon`・`template`・`default`。`parents` を省略した種類はどこにでも置ける。
  - props: `type`・`values`・`multi`・`label`・`inverse`・`symmetric`。
  - `workflow`: `transitions`（ステータス → 移れるステータスの列）と `required`（ステータス → 必須の項目の列）。`required` には組み込みのフィールド（`assignees` など）と props のキーの両方を書ける。
  - `scoped`: `where`・`statuses`・`workflow`・`templates`（`only` と、種類 → テンプレートの `default`）。
- 既定を引く API として `DefaultStatus(category)` と `DefaultKind()`（work 役割の既定）を持つ。`hh start`、自動化の `set: {category: …}`、Store 間の移動で使う。
- `schema.Default()` は親設計の既定（ステータス7つ、種類5つ、props の `priority`・`start`・`due`・`estimate`・`points`・`duplicates`・`causes`・`relates`）を返す。
- `kinds` は親設計の既定どおりにする。`project` は `parents` を省略しているので、どこにでも置ける。
- `schema.Dev()` は `Default()` に `review` ステータス（doing カテゴリ）を足したもの。

### スキーマ自体の検査 `(*Schema).Check() []Problem`

- ステータス: 名前が一意、カテゴリが `todo / doing / done / canceled` のどれか、各カテゴリに `default: true` がちょうど1つ
- 種類: 名前が一意、`role` が `container / work` のどれか、`parents` が既存の種類か `root`、work 役割に `default: true` がちょうど1つ
- props: 型が既知、enum と multi_enum の `values` が空でない、`inverse` と `symmetric` を同時に指定しない、組み込みのフィールド名と重ならない
- workflow: `transitions` に出てくるステータスが存在する。`required` の項目が組み込みのフィールドか定義済みの props である
- scoped: `where` が `project:<ULID>` の形、`where` が重複しない、`statuses` が各カテゴリの既定を含む

### プロジェクトの実効スキーマ `(*Schema).For(ancestors []ID) *Effective`

- `ancestors` はタスクの祖先の ID を近い順に並べたもの（タスク自身が container ならそれを先頭に含める）。祖先を引くのは呼び出し側。
- `scoped` の要素を持つ祖先のうち、最も近いものを適用した結果を返す。該当がなければ Store の定義そのもの。エピックなど途中の container に `scoped` がなくても、その上のプロジェクトの上書きが効く（親設計の「そのプロジェクトの子孫すべてに効く」）。
- 複数の祖先がそれぞれ `scoped` を持つとき、それらを合成することは 1a では扱わない（最も近いものだけを使う）。
- 遷移は Store とプロジェクトの両方で許されたものだけ。必須プロパティは両方の和。`statuses` は表示・選択の絞り込み。
- `templates` の絞り込みと種類ごとの既定も持つ。

### タスクの検査

違反はコード付きの `[]Violation` で返し、拒否か警告かは呼び出し側が決める（書き込みでは拒否、同期後は警告）。

- `ValidateTask(task, eff)`: 種類とステータスが定義済みか、props の値が型に合うか。スキーマにない props は違反ではなく「未定義」として返す。`task_ref` は ULID として正しい形か、`user` は `<host>/<login>` の形かだけを見る。参照先が存在するかは 1c で扱う。
- `ValidatePlacement(kind, parentKind)`: `parents` に照らした配置の検査。親がないときは `root` として扱う。親タスクを引くのは呼び出し側。
- `ValidateTransition(from, to, task, eff)`: 遷移が許されるかと、入る先のステータスで必須のプロパティがそろっているか。同じステータスへの遷移は常に許す（親設計に記載がないため 1a で決めたこと）。

### 1a で扱わない検査

- 親と依存の循環、Store をまたぐ参照: 1c（インデックス）で扱う。

## エラー

- デコードのエラーは `*DecodeError{Field, Line, Err}`。行番号は `yaml.Node` から取る。ファイルパスは呼び出し側で wrap する。
- 未知のキーとスキーマにない props はエラーにせず保持する。

## テスト

- **ゴールデンテスト**: `testdata/` の入力をデコードしてエンコードし、期待ファイルと比べる。コメントの保持、未知キーの順序の保持、既知キーの並べ替え、CRLF→LF を確かめる。期待ファイルは `-update` フラグで更新する。
- **冪等性**: 全ファイル型で `Encode(Decode(Encode(x))) == Encode(x)`。
- **fuzz テスト**: frontmatter の分割、`Rank.Between` の性質（`a < 結果 < b`）。
- **rank の互換性**: rocicorp のテストベクタ。
- **スキーマ検査**: 検査項目ごとのテーブル駆動テスト。

## 後続への申し送り

- フェーズ2: `schema.yaml` と `automations.yaml` の「キーごとに LWW」で比べる時刻が、親設計では決まっていない（ファイルに `updated` がない）。
- 親設計ドキュメント: 「コメント」の節のパス（`comments/<タスクULID>/…`）が「ファイルレイアウト」の節（`comments/<a>/<b>/<タスクULID>/…`）と食い違う。本書は後者に合わせた。

## 完了条件

- 全ファイル型の往復で、コメントと未知のキーが失われない。
- `make check` が通る。
