# フェーズ 1d-2: 編集・コメント・履歴・検査と devenv 設計

- 日付: 2026-10-30
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」1. Core とファイル形式、「CLI 仕様」「データ形式」の「コメント」、「同期とマージ」の「消失への耐性」、「開発環境」
- 前段: `docs/superpowers/specs/2026-10-30-phase1d1-cli-design.md`（ほか 1a〜1c の spec）

## 位置づけ

| 段階 | 内容 |
| --- | --- |
| 1a〜1c（完了） | 型とファイル形式、Store と git、インデックスとクエリ |
| 1d-1（完了） | 設定と場所、`store init`、タスクの CRUD とツリー表示、出力形式 |
| 1d-2（本書） | `edit`、`comment`（追加・編集・削除）、`log`、`schema check`、`schema migrate`、`doctor`、`show` の直近の履歴、devenv、1a〜1d-1 の持ち越しの回収 |

1d-2 でフェーズ1を閉じる。1a〜1d-1 の持ち越しは、後のフェーズの機能がないと決められないものを除いて、すべてここで扱う。

### 1d-2 で扱わないもの

後のフェーズの機能が要るものだけ。

- `doctor` による修復: 検査だけにする（利用者の決定）。直し方は各問題に添える。`undo` と `restore --at` はフェーズ2
- merge driver の登録の確認: フェーズ2（merge driver と一緒に作る）
- go-git が `index.lock` の手順を使わない件（1b）: Store への書き込みは Store のロックで直列になるので、フェーズ1では問題にならない。手で git を操作するときとの共存を設計するフェーズ2で、要るかを決める
- schema.yaml / automations.yaml の「キーごとに LWW」で比べる時刻（1a）: フェーズ2のマージの設計で決める
- purge したタスクの `log`: index から ID を引けないため扱わない
- コメントの author と、コメントの編集・削除の本人確認: アカウントが要るのでフェーズ3
- devenv の bare リポジトリ、URL の書き換え、fakegh: フェーズ2・3・5
- 性能（1c の全件再構築の重さ）: フェーズ7の目標と一緒に測る

## パッケージ構成

```
internal/gitrepo/
  log.go       Log: HEAD から新しい順のコミット。パスの接頭辞で絞る
  status.go    Status: 作業ツリーと index の未コミットの変更
  verify.go    Verify: HEAD から届くオブジェクトがすべて読めるか
internal/store/
  read.go      TaskBytes、CommentIDs を足す
internal/doctor/
  finding.go   Finding
  schema.go    schema check の検査
  doctor.go    doctor の検査（store.Open に頼らない）
internal/schema/
  migrate.go   定義の改名と統合（Schema の値を変えるだけ。タスクは cmd が書き換える）
cmd/hedgehug/
  editor.go    $EDITOR で一時ファイルを開く共通処理と、再編集の確認
  edit.go comment.go log.go schema.go migrate.go doctor.go
tools/devenv/
  main.go      init / reset / env
```

依存の向きは `cmd/hedgehug → doctor → index → store → gitrepo`、`tools/devenv → config, store`。`doctor` は `cmd/hedgehug` を知らない。

## gitrepo

### Log

```go
type LogEntry struct {
    Hash     plumbing.Hash
    Author   Signature
    When     time.Time
    Subject  string
    Trailers []Trailer
}

func (r *Repo) Log(prefixes []string, limit int) ([]LogEntry, error)
```

- HEAD から親をたどり、新しい順に返す。`limit` が 0 なら全件。
- `prefixes` が空でなければ、いずれかの接頭辞で始まるパスを変えたコミットだけを返す（go-git の `LogOptions.PathFilter`）。接頭辞は、ファイルならそのパス、ディレクトリなら末尾に `/` を付けて渡す。
- Subject はメッセージの1行目。Trailers はメッセージの最後の段落の `Key: Value` の行。
- コミットがまだない（HEAD がない）なら空を返す。

### Status

```go
type FileStatus struct {
    Path     string
    Staged   bool // index が HEAD と違う
    Unstaged bool // 作業ツリーが index と違う、または未追跡
}

func (r *Repo) Status() ([]FileStatus, error)
```

- go-git の `Worktree.Status` を使い、変更のあるパスだけをパスの順に返す。`.gitignore` で無視されるもの（`.hedgehug/local/`）は含めない。
- 作業ツリー全体を見るので遅いが、`doctor` でしか使わない。

### Verify

```go
func (r *Repo) Verify() error
```

- HEAD から届くコミット、ツリー、blob をすべて読む。同じハッシュは1度だけ読む。最初に読めなかったオブジェクトのハッシュと種類を入れたエラーを返す。

## store

```go
// TaskBytes returns the task file of id as it is on disk.
func (s *Store) TaskBytes(id model.ID) ([]byte, error)

// CommentRef is one comment file.
type CommentRef struct{ Task, Comment model.ID }

// CommentIDs lists every comment file of the store by its path, without
// reading the files.
func (s *Store) CommentIDs() ([]CommentRef, error)
```

- `TaskBytes` はデコードしない。壊れたタスクも読める。ファイルがなければ `ErrNotFound`。
- `CommentIDs` は `comments/` の下のファイル名を `model.ParseCommentPath` で読む。読めない名前は飛ばす（`doctor` が別に見る）。

## コマンド

### 共通: エディタ

- `$VISUAL`、`$EDITOR` の順に見て、どちらもなければ `vi`（Windows は `notepad`）。値は空白で区切ってコマンドと引数にし、最後に一時ファイルのパスを足して実行する。stdin・stdout・stderr はそのまま渡す。
- 一時ファイルは `os.CreateTemp("", "hedgehug-<名前>-*.md")`。成功したら消す。失敗で終わるときは残してパスを出す。
- エディタが 0 以外で終わったら、編集を中止してエラー（一時ファイルは残す）。
- 再編集の確認は、stdin が TTY のときだけ「もう一度開く? [Y/n]」と stderr に出す。空か `y` で開き直し、それ以外で中止。TTY の判定は `purge` の確認と同じものを使う。

### `edit <id>`

1. index でタスクを引く。削除済みならエラーにして `restore` を案内する（`add --parent` と同じ文言）。
2. `TaskBytes` の内容を一時ファイルに書いてエディタで開く。デコードできない壊れたタスクも、生のバイトのまま開ける。
3. 保存後のバイトが開いたときと同じなら `No changes` と出して終了 0。コミットしない。
4. デコードし、次を拒否する:
   - `id`、`key`、`created`、`updated`、`deleted_at` の変更
   - `parent` と `rank` の変更（`set` と同じく `mv` を案内する）
5. `Store.Update` の中で、ファイルが開いたときのバイトと同じかを確かめ、同じならデコードした値を `PutTask` する。違えば「編集中にほかから変更された」でエラーにする（再編集の確認はせず、一時ファイルを残す）。
6. デコードの失敗、4 の拒否、5 の Store の検査の違反（`ValidationError`）のときは、理由を出して再編集を確かめる。開き直すときは前回の編集内容のまま開く。中止したら一時ファイルのパスを出して終了 1。
7. Op は `edit`。出力は `set` と同じ（`--json` は書いた後のタスク）。

### `comment <id> [text...]`

- text は空白でつなぐ。省略したら空の一時ファイルをエディタで開く。
- 空白だけの本文は「空のコメント」でエラー、コミットしない。
- 削除済みのタスクにはエラーにして `restore` を案内する。壊れたタスクには付けられる（index に Entry があればよい）。
- 新しい ULID でコメントを作り、author は空にする（フェーズ3まで）。Op は `comment`、Task はコメントのタスク。
- 出力はコメントの短縮 ID。`--json` はコメント（`id`、`short`、`task`、`author`、`created`、`updated`、`deleted_at`、`body`）。

### `comment edit <comment-id>` / `comment rm <comment-id>`

- `edit` は本文だけを一時ファイルに書いて開く（frontmatter は見せない）。変わっていなければ `No changes` で 0。空白だけにしたらエラーにして `comment rm` を案内する（再編集の確認はしない）。
- `rm` は `deleted_at` を付ける。すでに削除済みならエラー。
- 削除済みのコメントは `edit` できない。
- Op は `comment`。
- `comment` のサブコマンドと、タスクの ID の引数は重ならない。サブコマンド名には Crockford の base32 にない文字（`i`）があるか、短縮 ID の最短の4文字より短い。

### コメントの ID

- `CommentIDs` の全件を対象に、タスクと同じ規則で解決する。完全な ULID か、末尾一致（大文字小文字は区別しない）。複数が一致したら、候補の短縮 ID とタスクのタイトルを並べたエラー。
- 短縮形は、全コメントの中で一意になる最短の末尾（最低4文字）。削除済みのコメントも数に入れる。
- `show` のコメントと `show --json` の `comments[].short` にこの短縮形を出す。author が空なら `-` と出す。

### `log [id] [-n N]`

- id なしは Store 全体。ありはそのタスクの `model.TaskPath` と、`comments/` と `worklogs/` の下のそのタスクのディレクトリに触れたコミット。
- `-n` は件数の上限。既定は 0（無制限）。負の値は使い方の誤り。
- 1行の形: `<hash の先頭7文字>  <日時 YYYY-MM-DD HH:MM（ローカル時刻）>  <作者名>  <subject>`
- `--json` は配列。各要素は `hash`（40文字）、`date`（RFC 3339）、`author`（`name`、`email`）、`op`（`Hedgehug-Op` の値、なければ null）、`task`（`Hedgehug-Task` の値、なければ null）、`subject`。

### `show` の直近の履歴

- `log <id> -n 5` と同じものを、本文とコメントの後の `History` の節に出す。
- `--json` は `history` に `log --json` と同じ形の配列を入れる。

### 共通: Finding

`schema check` と `doctor` が出す問題。

```go
type Finding struct {
    Check string // 検査の種類（下の表）
    Where string // ファイルとその中の場所、またはタスクの短縮 ID
    Msg   string
    Hint  string // 直し方。なければ空
}
```

| Check | 内容 |
| --- | --- |
| `schema` | schema.yaml が読めない、`Schema.Check` の問題 |
| `template` | 参照されたテンプレートがない |
| `prop_name` | クエリの組み込みキーと同じ名前の props |
| `override` | 上書きが存在しない、または削除済みのプロジェクトを指す |
| `task` | index の問題（スキーマの違反、参照先なし、循環、壊れたファイル） |
| `file` | デコードできないファイル、id とパスの食い違い、タスクのないコメント・作業時間の記録 |
| `worktree` | 未コミットの変更 |
| `git` | 読めない git オブジェクト、index が開けない |

- テキストは1件1行 `<where>: <msg>`。Hint があれば次の行に `  hint: <hint>`。最後に `N problems`。0 件なら `No problems`。
- `--json` は Finding の配列（キーは小文字。Hint が空なら省く）。
- 1件でもあれば終了コード 1、なければ 0。
- 並びは、検査の順（下の番号）、その中では Where の順。

### `schema check`

1. schema.yaml がデコードできなければ、それだけを出して終わる。Hint は「`git restore schema.yaml` で戻すか、手で直してコミットする」。
2. `Schema.Check()` の問題（Where は `schema.yaml: <Problem.Path>`）。
3. テンプレートの実在: `kinds[].template`、`scoped[].templates.only`、`scoped[].templates.default` の名前が `templates/<name>.md` にあるか（1a の持ち越し）。
4. クエリの組み込みキー（`project`、`is`、`sort`、`group`、`blocks`、`label`、`assignee`）と同じ名前の props。Msg は「クエリでこの名前の props を引けない」（1c の持ち越し）。
5. 既存タスクとの不整合: index の `Problems` のうち、スキーマ由来のもの（`unknown_kind`、`unknown_status`、`missing_title`、`bad_value`、`placement`、`required`）。index はプロジェクトの上書きを `Effective` で見ているので、上書きの分も入る。Where はタスクの短縮 ID とタイトル。
6. `scoped[].where` が、存在しないか削除済みのプロジェクトを指す。

`schema check` は `store.Open` と `index.Open` で開く。schema.yaml が壊れていて開けないときは、1 だけを出す（schema.yaml を自分で読んで判定する）。

### `schema migrate`

```
hh schema migrate status <old> <new> [--dry-run]
hh schema migrate kind <old> <new> [--dry-run]
hh schema migrate prop <old> <new> [--dry-run]
hh schema migrate value <prop> <old> <new> [--dry-run]
```

定義の名前（`value` は enum の値）を変え、それを使っているタスクも書き換えて、schema.yaml とタスクを1コミットにする。Op は `migrate`、Task はなし。

- `<old>` が定義されていなければエラー。`<old>` と `<new>` が同じならエラー。
- `<new>` が定義されていなければ改名（定義の名前だけを変え、ほかの項目はそのまま）。定義されていれば統合（`<old>` の定義を消し、使っている箇所を `<new>` に寄せる）。
- 書き換えるもの:

| 対象 | タスク | schema.yaml |
| --- | --- | --- |
| `status` | `status` | `workflow.transitions` のキーと値、`workflow.required` のキー、`scoped[].statuses`、`scoped[].workflow` の同じ箇所 |
| `kind` | `kind` | `kinds[].parents`、`scoped[].templates.default` のキー |
| `prop` | `props` のキー | `props` のキー、`workflow.required` の値（`scoped[]` も） |
| `value` | その props の値（複数値なら要素） | その props の `values` |

- 統合で同じ値が列に2つになったら、1つにまとめる（先に出てきた位置に残す）。
- `status` の統合で `<old>` がカテゴリの既定だったとき、`<new>` が同じカテゴリならその既定を `<new>` に移す。違うカテゴリなら、`Schema.Check` が「既定がない」で拒否する（そのまま拒否のエラーを出す）。
- `kind` の統合も同じで、`<old>` が既定のとき、`<new>` が同じ `role` ならその既定を `<new>` に移す。違う `role` なら `Schema.Check` が拒否する。
- `prop` の統合は、型（`type` と `multi`）が同じでなければエラー。`<old>` と `<new>` の両方に値を持つタスクがあればエラーにして、そのタスクを並べる。
- `value` は `enum` か `multi_enum` の props だけ。ほかの型はエラー。
- 削除済みのタスクも書き換える。デコードできない壊れたタスクは書き換えずに、短縮 ID を並べた警告を出す（`doctor` でも分かる）。
- 移行は遷移ではないので、`workflow.transitions` の検査はしない（Store の `Op` に遷移の検査を飛ばす指定を足す）。ほかの検査（必須項目、置き場所、値）はふだんどおりで、新しく違反になるタスクがあれば全体を拒否する。
- 出力は、書き換えたタスクの数と短縮 ID・タイトルの一覧。`--dry-run` は同じ一覧を出して何も書かない（schema の `Check` はするが、Store がするタスクの検査はしないので、本番では拒否されることがある）。`--json` は `{"changed": [タスク...], "skipped": [短縮 ID...]}`。

### `doctor`

config から決めた Store のディレクトリを直接見る。`store.Open` は schema.yaml が壊れていると失敗するため、最初は使わない。

1. store.yaml と schema.yaml がデコードできるか、`Schema.Check()` の問題。schema.yaml が壊れていたら、以降の 5 を飛ばした旨の Finding を足す（1b の持ち越し）。
2. 作業ツリーのファイル: `tasks/`、`comments/`、`worklogs/`、`sprints/`、`releases/`、`templates/` の下の全ファイル。
   - デコードできない
   - 中の id とパスの id が違う（1c の持ち越し。index では broken、store では存在扱いになる食い違い）
   - パスの規則に合わない名前
   - タスクファイルのないコメント・作業時間の記録
3. 未コミットの変更（`gitrepo.Status`）。Staged なら「次の操作のコミットに混ざる」と出し、Hint は `git restore --staged <path>`。Unstaged なら Hint は `git restore <path>`（未追跡なら削除）（1b の持ち越し）。
4. git オブジェクト（`gitrepo.Verify`）。
5. schema.yaml が読めれば `store.Open` と `index.Open` をして、`schema check` の 2〜6 と、index の `missing_ref`、`cycle`、`broken`。`index.Open` が失敗したら（HEAD の schema.yaml が壊れているなど）、その理由を `git` の Finding にする。

index は HEAD のツリーを読み、2 は作業ツリーを読む。未コミットの変更があると食い違うことがあるが、3 で知らせる。

## devenv

`go run ./tools/devenv <command>`。開発者が使うだけで、配布物には入らない。

### `init [--dir <path>]`

- 既定の dir はリポジトリ直下の `.devenv/`。`.gitignore` に `/.devenv/` を足す。
- dir がすでにあればエラーにして `reset` を案内する。
- 目印のファイル `<dir>/.hedgehug-devenv` を最初に作る。
- `users/alice` と `users/bob` を作る。それぞれについて:
  - `users/<u>/.gitconfig` に `user.name = <u>`、`user.email = <u>@example.com` を書く。
  - `users/<u>` を `HEDGEHUG_HOME` とした `config.Paths` で、Store `me` を `store.Create` で作り、config.yaml に足す。CLI のコマンドは通さず、`internal/config` と `internal/store` を直接呼ぶ。
- 最後に `env` の使い方を出す。

### `reset [--dir <path>]`

- 目印のファイルがあるときだけ dir を消して `init` し直す。目印がなければ消さずにエラー。dir がなければそのまま `init`。

### `env <user> [--dir <path>]`

- `export HEDGEHUG_HOME=<dir>/users/<u>` と `export GIT_CONFIG_GLOBAL=<dir>/users/<u>/.gitconfig` を出す。`eval "$(go run ./tools/devenv env alice)"` で使う。HOME は変えない（作者の読み出しは `GIT_CONFIG_GLOBAL` を優先するので足りる）。
- パスは絶対パスで出す。存在しない利用者はエラー。

## エラー

- 1d-1 の決まり（stderr に `hedgehug: <理由>`、終了コード 1、使い方の誤りは 2）に従う。
- `schema check` と `doctor` の問題の一覧は stdout に出す（エラーではなく結果）。終了コードは 1。
- エディタが見つからない、起動できないときは、`$EDITOR` の設定を案内するエラー。

## テスト

- **gitrepo**: `Log` の順序、件数の上限、接頭辞での絞り込み（ファイルとディレクトリ）、trailer の読み出し、コミットなし。`Status` の変更・stage・未追跡・無視されるファイル。`Verify` はオブジェクトのファイルを壊して検出すること。
- **store**: `TaskBytes` が壊れたファイルをそのまま返す。`CommentIDs` の列挙。
- **doctor**: 壊れた schema.yaml（以降の検査を飛ばす）、id とパスの食い違い、規則に合わない名前、タスクのないコメント、未コミットの変更（stage あり・なし）、組み込みキーと同じ名前の props、存在しないテンプレート、削除済みのプロジェクトを指す上書き、index の問題、問題なし。
- **cmd/hedgehug**: エディタは `$EDITOR` にテスト用のプログラム（テストのバイナリを `TestMain` で別の役として起動し、決めた内容でファイルを書き換える）を指定する。
  - `edit`: 変更なし、成功（1コミット、`Hedgehug-Op: edit`）、id や parent の変更の拒否、検査の違反、TTY でないときの中止と一時ファイルの残り、壊れたタスクの修復、編集中の変更の検出、削除済みのタスク。
  - `comment`: 引数での追加、エディタでの追加、空の本文、`edit`、`rm`、短縮 ID の解決と曖昧さ、削除済みのタスク。
  - `log`: Store 全体と1タスク、`-n`、`--json`。`show` の `History` と `history`。
  - `schema check` / `doctor`: テキストと JSON の形、終了コード。
  - `schema migrate`: 4つの対象それぞれの改名と統合、schema.yaml の参照の書き換え、既定のステータスの引き継ぎと拒否、prop の型の違いと両方に値があるタスク、削除済みと壊れたタスク、遷移の検査を飛ばすこと、新しい違反での拒否、`--dry-run`、schema.yaml の未知のキーとコメントが残ること、1コミット。
- **devenv**: `init` の中身（2人分の .gitconfig と Store と config.yaml）、既存の dir での `init` のエラー、`reset`、目印のない dir を `reset` が消さないこと、`env` の出力。
- **シナリオ**: 1d-1 のシナリオに、`edit`、`comment`（追加・編集・削除）、`log`、`schema check`、`doctor` を足す。全操作が git 履歴に1コミットずつ残り、最後に `doctor` が問題なしで終わることを確かめる。

## 持ち越しの直し

1a〜1d-1 で、レビューの指摘として記録だけして直さなかったもの。どれも後のフェーズの機能を要しない。

### 形式（model / schema）

- **schema.yaml の書き込みで未知のキーとコメントを残す**（1a）。今は `props` や `workflow` の1項目を変えると、その値全体が作り直され、ほかの項目の未知のキーとコメントが消える。`schema migrate` が schema.yaml を書くので、ここで直す。
  - 列（`statuses`、`kinds`、`scoped`）の要素は名前（`scoped` は `where`）で、mapping（`props`、`workflow.transitions`、`workflow.required`）の要素はキーで、元のノードと対応させる。名前で対応しない要素は、残った要素どうしを順番に対応させる（改名した要素も元のノードを引き継ぐ）。
  - 対応した要素は、値の変わらないキーの元のノード（コメント付き）を使い、型が知らないキー（構造体の yaml タグにないキー）を残す。変わったキーだけを新しいノードにする。
  - 対応しない新しい要素は、今と同じく新しいノードにする。消えた要素は消す。
  - これは `model.Encoder` の一般の仕組みとして作り、タスクの `props` などほかの mapping にも同じ規則で効かせる。
- **明示タグを拒否する**（1a）。`!!binary` などのタグを付けた値は、書き換えても変更が検出されないことがある。`ParseDoc` の `checkNode` で、アンカーやエイリアスと同じく、明示タグ（`!!str` などの標準のものも含め、書き手が付けたタグ）を `DecodeError` にする。
- **number の NaN と Inf を拒否する**（1d-1）。`schema.ValidateTask` の number の検査で `bad_value` にする。`set` の値の解釈も同じく拒否する。

### store / gitrepo

- **空のコミット**（1b）。`gitrepo.Commit` は go-git の `ErrEmptyCommit` を `ErrNothingToCommit` に寄せる。`Store.Update` は、ファイルは変わったが HEAD と同じになった（作業ツリーが HEAD からずれていた）ときに `ErrNothingToCommit` を受けたら、コミットなしの成功として扱う。
- **`RemoveTask` と同じ Update で積んだコメント・作業時間**（1b）。`RemoveTask` は、そのタスクのコメント・作業時間のディレクトリの下に積まれた書き込みも、消す指定に変える。`RemoveTask` の後に同じタスクへ `PutComment` / `PutWorklog` したらエラー。
- **コミットしたかを返す**（1d-1。変更のない `set` / `status` が「Updated」と出る件）。`Store.Update` は、コミットしたかどうかを呼び出し側に返す。CLI は、コミットしなかったら `No changes` と出す（`--json` は今と同じくタスクを出す）。
- **遷移の検査を飛ばす指定**（`schema migrate` のため）。`Op` に足す。

### config

- **再インデントしない**（1d-1）。`Save` が `  - name` を `- name` に書き換える。読み込んだファイルの列の字下げを調べ、同じ字下げで書く（`yaml.WithIndent` と列の字下げの指定）。ファイルがなかったときは、親設計の例と同じ `  - name` にする。
- **symlink を壊さない**（1d-1）。config.yaml が symlink なら、リンク先のファイルを一時ファイルからの rename で置き換える。
- **コメントだけの config.yaml**（1d-1）。`AddStore` でコメントが消える。中身のないドキュメントでも、コメントを残したまま `stores` を足す。
- **相対の `path:`**（1d-1）。config.yaml のあるディレクトリからの相対として読む（利用者の決定）。書くときは今と同じ（ホームの下なら `~/`、ほかは絶対パス）。
- **git の `[include]`**（1d-1）。作者の読み出しで `[include] path = ...` をたどる。相対のパスは include を書いたファイルのディレクトリから、`~/` はホームに展開する。たどるのは深さ 10 まで（git と同じ）。`[includeIf]` は読まない。条件の `gitdir:` は Store のリポジトリでは一致しないことが多く、`onbranch:` などは作者の決定に向かないため。

### CLI

- **使い方の誤りの終了コードを揃える**（1d-1、利用者の決定）。引数の誤りはすべて 2 にする。`hedgehug store` と `hedgehug dep` を単体で実行したら、使い方を stderr に出して 2（ルートと同じ）。`store init` の名前の形の誤りも 2。登録済みの名前と重なるのは実行時のエラーとして 1 のまま。
- **`--format` の未知のキー**（1d-1）。今は `<no value>` と出る。`missingkey=error` で実行し、テンプレートの誤り（解析と実行）を使い方の誤り（2）にする。
- **`show` のラベルの幅**（1d-1）。ルーン数でなく表示幅で揃える。East Asian Width が W と F の文字を幅 2 として数える（`golang.org/x/text/width`）。
- **`ls --tree` の循環の見出し**（1d-1、利用者の決定）。`== in a parent cycle` の下に、parent の循環に入っているタスクを ID の順に並べ、それぞれの配下（循環に入っていない子孫）をふつうのツリーとして子に出す。`in_cycle` は循環に入っているタスクだけに付ける。`--json` も同じ入れ子にする。

### テスト

挙げたまま書かなかったもの。

- 1c: parent の3つ組の循環、削除済みを含む短縮 ID の一意性、全件再構築とキャッシュからの読み込みで `Entry` が全部一致すること、循環を含む Store の `Open`、`update` が途中で失敗した後に `Open` し直すと正しい状態になること。
- 1d-1: `store init` で `Create` の後の config.yaml の書き込みに失敗したときの巻き戻し、`status` と `dep` が拒否されたときにコミットしないこと、`mv` で自分自身を `--before` に指定、削除済みのタスクの `mv`、自分自身を `--blocked-by` に指定。

### 親設計ドキュメント

- パッケージ構成の表に `internal/gitrepo`、`internal/index`、`internal/config`、`internal/doctor` を足す（1b・1c で分けた分）。
- 「コメント」の節のパスを、ファイルレイアウトと実装に合わせて `comments/<a>/<b>/<タスクULID>/<コメントULID>.md` に直す。

### 直さないもの

- 1a〜1d-1 で意図して決めたもの（restore の秒精度、rank をロックの外で計算すること、`status:todo` をカテゴリとして扱うことなど）は、それぞれの spec のとおりにする。
- コミットの Co-Authored-By の混在は、履歴を書き換えないと直らないのでそのままにする。

## 持ち越しの回収（新しい機能で扱うもの）

| 持ち越し | 出どころ | 扱い |
| --- | --- | --- |
| テンプレート名の存在確認 | 1a | `schema check` の 3 |
| schema.yaml が壊れると、それを直す PutSchema も含めて全 Update が失敗する | 1b | `doctor` と `schema check` が知らせ、`git restore` を案内する（修復はしない） |
| 作業ツリーの未コミット変更。手で stage した別の変更は次のコミットに混ざる | 1b | `doctor` の 3 |
| schema.yaml が HEAD で壊れていると `index.Open` が失敗する。id とパスが食い違うファイル | 1c | `doctor` の 1・2・5 |
| props 名が組み込みキーと重なると隠れる | 1c | `schema check` の 4 |
| `show` の直近の履歴 | 1d-1 | `show` の `History` |
| `edit` は作業ツリーのファイルを直接開かない | 1d-1 | 一時ファイルで開く。生のバイトを使うのは壊れたタスクを直すため |

1b の「親が purge されたタスクが壊れていると、修復の書き込みが ErrNotFound になる」と、1a の「Rank を検証していない」は 1c で直っている。

## 完了条件

- 1d-2 の全コマンドと devenv が動き、テストが通る（`make check`）。
- 「持ち越しの直し」がすべて入り、それぞれにテストがある。
- シナリオテストで、フェーズ1の CLI（add / ls / show / edit / set / mv / status / done / rm / restore / purge / dep / comment / log / schema check / schema migrate / doctor）がすべて動き、全操作が git 履歴に残る。
