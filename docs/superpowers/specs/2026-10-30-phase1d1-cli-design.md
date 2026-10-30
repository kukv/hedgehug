# フェーズ 1d-1: CLI の基盤とタスク操作 設計

- 日付: 2026-10-30
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」1. Core とファイル形式、「CLI 仕様」「ローカル設定」「開発環境」
- 前段: `docs/superpowers/specs/2026-10-30-phase1a-model-format-design.md`、`docs/superpowers/specs/2026-10-30-phase1b-store-git-design.md`、`docs/superpowers/specs/2026-10-30-phase1c-index-query-design.md`

## 位置づけ

| 段階 | 内容 |
| --- | --- |
| 1a（完了） | 型・ファイル形式の codec・ULID・rank・パス・スキーマ検証 |
| 1b（完了） | Store のディスク読み書き、原子的な書き込み、ロック、go-git での1操作1コミット |
| 1c（完了） | インデックス、短縮 ID、クエリの基本、循環と Store 外参照の検出 |
| 1d-1（本書） | 設定と場所（`HEDGEHUG_HOME`）、Store の選び方、`store init`、タスクの CRUD とツリー表示、出力形式 |
| 1d-2 | `edit` / `comment` / `log` / `schema check` / `doctor`、`show` の直近の履歴、devenv |

### 1d-1 で扱うもの

- `internal/config`: 設定とデータの場所、`config.yaml` の読み書き、Store の選び方、git の作者の読み出し
- `internal/store`: `Create` の修正（スキーマを渡せる、失敗時の後始末、`meta` を書き換えない）
- `cmd/hedgehug`: cobra によるコマンド定義と出力。`store init`、`add`、`ls`、`show`、`set`、`status`、`done`、`rm`、`restore`、`purge`、`mv`、`dep`
- goreleaser: `hh` の別名ビルド

### 1d-1 で扱わないもの

- `edit`、`comment`、`log`、`schema check`、`doctor`、`show` の直近の履歴、devenv: 1d-2
- `restore --at`、`undo`、`.gitattributes` と merge driver: フェーズ2
- `mv --store`、`purge --moved` / `--rewrite-history`、`store init --remote`、`store add/remove/list/discover`、`use`、コンテキスト解決、アカウントと `@me`: フェーズ3
- `set --query`、`add --template`、種類に紐づくテンプレートの展開: フェーズ6
- `completion`、性能目標: フェーズ7

## パッケージ構成

```
internal/config/
  paths.go         HEDGEHUG_HOME と XDG からの場所の決定
  config.go        config.yaml の読み書き（yaml.Node で未知のキーとコメントを残す）
  author.go        git のグローバル設定から作者を読む
internal/store/
  store.go         Create の修正
cmd/hedgehug/
  main.go          main と run
  app.go           ルートコマンド、全体フラグ、使い方の誤り、Store と index を開く共通処理、エラーの出し方
  output.go        表・JSON・テンプレートの出力、色
  tree.go          ls --tree
  assign.go        k=v / k+=v / k-=v / k= の解釈（add の -p と set で共用）
  store_init.go    store init
  add.go ls.go show.go set.go status.go rm.go mv.go dep.go
```

依存の向きは `cmd/hedgehug → query → index → store → schema → model`、`cmd/hedgehug → config`。`config` は `store` を知らない。

新しい依存: `github.com/spf13/cobra`、`golang.org/x/term`。

## 場所

`internal/config` の `Paths` が、設定ファイルと Store を置くディレクトリを決める。環境変数は引数で受け取り（`func(string) string`）、テストから差し替える。

| 条件 | 設定ファイル | Store の置き場所 |
| --- | --- | --- |
| `HEDGEHUG_HOME` あり | `$HEDGEHUG_HOME/config.yaml` | `$HEDGEHUG_HOME/stores/<name>` |
| なし、XDG あり | `$XDG_CONFIG_HOME/hedgehug/config.yaml` | `$XDG_DATA_HOME/hedgehug/stores/<name>` |
| どちらもなし | OS の既定の設定ディレクトリ（下記）の `hedgehug/config.yaml` | Linux / macOS は `~/.local/share/hedgehug/stores/<name>`、Windows は `%LOCALAPPDATA%\hedgehug\stores\<name>` |

- OS の既定の設定ディレクトリは、`os.UserConfigDir()` を呼ばず、注入された環境変数から同じ場所を求める。Windows は `%APPDATA%`、macOS は `~/Library/Application Support`、それ以外は `~/.config`。
- `XDG_CONFIG_HOME` と `XDG_DATA_HOME` はそれぞれ独立に見る。片方だけあればその片方だけを使う。
- 環境変数の値が相対パスなら無視する（XDG の仕様どおり）。`HEDGEHUG_HOME` も同じ。
- `HEDGEHUG_HOME` を1つのディレクトリにまとめるのは、devenv の `users/alice` をそのまま `HEDGEHUG_HOME` にするため。

## config.yaml

形は親設計の「ローカル設定」と同じにする。1d-1 が読み書きするのは `stores[].name` と `stores[].path` だけ。

```yaml
stores:
  - name: me
    path: ~/.local/share/hedgehug/stores/me
```

- ファイルがなければ、Store が1つもない設定として扱う。
- 読み込みは `yaml.Node` で行い、書き込みでは `stores` の列に要素を足すだけにする。ほかのキー、要素の未知のキー、コメントは残す。
- `path` の先頭の `~/` はホームディレクトリに展開する。書くときは、ホームの下なら `~/` で書く。
- 書き込みは一時ファイルからの rename で行う。親ディレクトリがなければ作る。
- 名前の重複、`name` か `path` のない要素は、読み込みのエラーにする。エラーにはファイルのパスと行を入れる。

## Store の選び方

フェーズ3のコンテキスト解決ができるまでのつなぎ。

1. 全体フラグ `--store <name>` があれば、その名前の Store を使う。なければエラー（登録済みの名前を並べる）。
2. なければ、登録が1つならそれを使う。
3. 0 なら `hh store init <name>` を案内するエラー、2つ以上なら `--store` を求めて名前を並べるエラーにする。

## 作者

- `Op.Author` には git のグローバル設定の `user.name` と `user.email` を使う。go-git の `LoadConfig` はプロセスの環境変数を読み、テストから注入できない。そこで `getenv` から決めたパスの gitconfig を読む。
- 読むファイルは、`GIT_CONFIG_GLOBAL` があればそのファイルだけ。なければ `$XDG_CONFIG_HOME/git/config`（なければ `~/.config/git/config`）、`~/.gitconfig` の順に読み、後のファイルが勝つ（git と同じ）。
- どちらかが空なら、`git config --global user.name` / `user.email` での設定を案内するエラーにする。読み出しは書き込みをするコマンドでだけ行う（読むだけのコマンドは git の設定がなくても動く）。
- アカウントによる作者の決定はフェーズ3。それまで `query.Options.Me` は空にする（1c で決めたとおり）。
- `created_by` の値にはアカウントが要るので、フェーズ3までは空のままにする。`show` は `by -` と出す。

## store.Create の修正

```go
func Create(dir string, meta *model.StoreMeta, sc *schema.Schema, author gitrepo.Signature) (*Store, error)
```

- `sc` が nil なら `schema.Default()` を使う。`--template dev` で `schema.Dev()` を渡すため。
- 呼び出し側の `meta` を書き換えない。コピーに `Format` を入れて使う（1b の持ち越し）。
- `ensureEmpty` を通ったあとで失敗したら、作ったものを消す。`dir` を新しく作ったならディレクトリごと、もとから空だったなら中身だけを消す。消すのにも失敗したら `errors.Join` で返す（1b の持ち越し）。

## コマンド

### 共通

- ID を受け取る引数は `index.Resolve`（キー、古いキー、短縮 ID の末尾一致）で解決する。削除済みのタスクも解決できる。
- 書き込みは1コマンドで `Store.Update` を1回呼び、1コミットにする。`Op.Name` は下の表のとおり。`Op.Task` は主になるタスク。
- 読むだけのコマンドは `index.Open` で開く。書き込むコマンドは、参照の解決のために `index.Open` してから `Store.Update` する。CLI は毎回 `index.Open` する（1c で決めたとおり）。
- 全体フラグ: `--store <name>`、`--json`、`--no-color`。
- 引数なしのルート `hedgehug` は使い方を出して終了コード 2。`hedgehug store` と `hedgehug dep` はサブコマンドなしならヘルプを出して 0。
- cobra の既定の `completion` コマンドは、フェーズ 7 まで無効にする。
- `add` / `mv` は、rank を Store のロックを取る前に index から計算する。同じ親の下で同時に2つ `add` すると rank が等しくなりうる。その並びは ID の順になる。

| コマンド | Op |
| --- | --- |
| `add` | `add` |
| `set` | `set` |
| `status` / `done` | `status` |
| `rm` | `rm` |
| `restore` | `restore` |
| `purge` | `purge` |
| `mv` | `mv` |
| `dep add` / `dep rm` | `dep` |
| `store init` | `init`（`Create` のコミット） |

### `store init <name> [--template dev]`

- `name` は小文字の英数字、`-`、`_` からなり、英数字で始まる。登録済みの名前と重なればエラー。
- 置き場所は「場所」の表の Store のディレクトリ。`StoreMeta` は新しい ID と `Name` だけを入れる。
- `store.Create` が成功してから config.yaml に足す。config.yaml の書き込みに失敗したら、作った Store のディレクトリを消してエラーを返す。
- 出力は Store の名前とパス。

### `add <title> [--parent <id>] [--kind <kind>] [--status <status>] [-p k=v]...`

- `kind` の既定は `schema.DefaultKind()`。`status` の既定は `Schema.DefaultStatus(todo)` で引く。プロジェクトの上書きでカテゴリの既定は外せないので、置き場所によらず同じになる。
- `--parent` が削除済みのタスクならエラーにして、先に戻すよう案内する（`restore` と同じ文言）。
- `rank` は兄弟（同じ親の、削除されていないタスク）の最後の rank と空の間に作る（`model.Between(last, "")`）。兄弟がなければ `Between("", "")`。
- `-p` は `set` と同じ書き方（下の `set`）。
- 種類に紐づくテンプレートは展開しない（フェーズ6）。
- 出力は、作ったタスクの短縮 ID とタイトル。

### `ls [query] [--tree] [--ids-only] [--format <template>]`

- 引数は1つのクエリ文字列として `query.Parse` に渡す。複数の引数は空白でつないでから渡す。
- 既定の表の列は、短縮 ID、ステータス、種類、担当者、ラベル、タイトルの順。幅の広い文字を含むタイトルで列がずれないよう、タイトルを最後にした。`group:` があれば、グループごとに見出しを付ける。
- `--ids-only` は短縮 ID を1行に1つ出す。`--format` は Go の `text/template` で、1タスクごとに JSON と同じ形の値を渡す。`--json` / `--ids-only` / `--format` は同時に指定できない。
- `--tree`:
  - クエリに一致したタスクと、それを根からつなぐのに要る祖先を出す。一致しなかった祖先は薄く表示する（色なしのときは括弧で囲む）。
  - 兄弟は常に rank の順（同じ rank なら ID の順）に並べる。`sort:` と `group:` はどちらも `--tree` と併用するとエラー（使い方の誤り、終了コード 2）。
  - parent の循環に入っていて `Roots` から届かないタスクは、末尾の `== in a parent cycle` の見出しの下に1段で出す（1c の持ち越し）。見出しは英語の出力にした。
  - `--json` と併用したときは、子を `children` に入れた入れ子の JSON を出す。各タスクは `children`、`context`（つなぐために出した祖先）、`in_cycle`（循環のタスク）を持つ。
  - `--ids-only` は、クエリに一致したタスクだけを描く順に出す（循環の中で一致したものも含む）。祖先は出さない（`ls --tree --ids-only status:todo | xargs hh done` で祖先まで閉じないため）。`--format` は描くタスクすべてに渡し、その値には `context`（bool）と `in_cycle`（bool）のキーが加わる。

### `show <id>`

- 出すもの: フィールド（短縮 ID、キー、種類、タイトル、ステータス、親、担当者、ウォッチャー、ラベル、repos、blocked_by、props、作成・更新・削除の日時と作成者）、本文、コメント（`Store.Comments`）、問題（`Index.Problems`）、逆方向の関連（`blocks` と、task_ref の `inverse`）。
- ファイルが読めない（decode できない）壊れたタスクは、本文とコメントの代わりに index の値と `broken: <理由>` の行を出す。コメントは読まない。
- 参照先のタスクは短縮 ID とタイトルで出す。参照先がなければ ID のまま出して「見つからない」と添える。
- 直近の履歴は 1d-2 で足す。

### `set <id> <assign>...`

書き方:

| 形 | 意味 |
| --- | --- |
| `k=v` | 設定する。複数値のキーでは、値の列を `v` だけにする |
| `k=` | 消す |
| `k+=v` | 複数値のキーに足す（すでにあれば何もしない） |
| `k-=v` | 複数値のキーから外す（なければ何もしない） |

キー:

- 組み込み: `title`、`kind`、`status`、`assignees`、`watchers`、`labels`、`repos`、`blocked_by`（後ろの5つが複数値）。`blocked_by` の値は ID として解決する。
- `parent` と `rank` は受け付けず、`mv` を案内する。`id`、`created`、`updated`、`deleted_at`、`key` などの書き換えできないキーもエラー。
- それ以外はスキーマの props として扱う。スキーマにないキーはエラー。値は型で解釈する: `date` は `model.ParseDate`、`duration` は `model.ParseDuration`、`number` は数値、`enum` はそのまま（値の検査は Store の検査に任せる）、`task_ref` は ID として解決して ULID で書く。`multi` の props は複数値として扱う。
- 1回の `set` で同じキーを複数回書いたら、左から順に当てる。

### `status <id> <status>` / `done <id>`

- `status` は名前で指定する。スキーマにない名前はエラー（カテゴリ名への読み替えはしない）。
- `done` は `Schema.DefaultStatus(done)` で引いたステータスにする。置き場所によらず同じになる。
- 遷移と必須項目の検査は Store の書き込みの検査に任せ、`ValidationError` をそのまま出す。

### `rm <id>`

- 本人と、まだ削除されていない子孫すべてに、同じ `deleted_at`（コマンドを実行した時刻、秒に切り捨て）を付けて1コミットにする。
- すでに削除済みならエラー。

### `restore <id>`

- 本人と、`deleted_at` が本人と同じ子孫の `deleted_at` を外す。同じ `rm` で消えたものだけが戻り、それ以前に別に消したものは戻らない。
- `deleted_at` は秒の精度。同じ秒のうちに子の `rm` と親の `rm` をすると、まとめて戻る。
- 親が削除済みならエラーにして、先に親を戻すよう案内する。
- 削除されていなければエラー。

### `purge <id> [--yes]`

- 削除済みのタスクだけを対象にする。子孫もすべて削除済みでなければエラー。
- 本人と子孫のファイルを `Tx.RemoveTask` で消して1コミットにする。ほかのタスクからの参照はそのまま残る（参照先なしは `doctor` と `is:invalid` で分かる）。
- 端末なら、消すタスクの数と一覧を示して `y` の入力で確認する。端末でなければ `--yes` を必須にする。

### `mv <id> [--parent <id> | --root] [--before <id> | --after <id>]`

- `--parent` / `--root` がなければ親は変えない。
- `--before` / `--after` の相手は、移動先の兄弟でなければエラー。指定がなければ末尾に置く。rank は前後の兄弟の rank の間に作る（`model.Between`）。
- 親も位置も指定がなければエラー。
- 削除済みのタスクを `--parent` に指定したらエラーにして、先に戻すよう案内する（`restore` と同じ文言）。
- 循環と置き場所の検査は Store の書き込みに任せる。

### `dep add|rm <id> --blocked-by <id>`

- `add` は `blocked_by` に足す（すでにあればエラー）。`rm` は外す（なければエラー）。
- 循環は Store の書き込みが拒否する。

## 出力

- 既定は人が読む表とツリー。色は stdout が端末（`golang.org/x/term`）で、`NO_COLOR` がなく、`--no-color` がないときだけ付ける。
- `--json` はすべてのコマンドで使える。書き込み系は、書いた後のタスク（`rm` / `restore` / `purge` は対象のタスクの列、`store init` は名前とパス）を出す。
- タスクの JSON:

```json
{
  "id": "01JC...", "short": "7k2q", "key": "", "kind": "task", "title": "...",
  "parent": null, "rank": "U", "status": "todo",
  "assignees": [], "labels": [], "blocked_by": [],
  "props": {"due": "2026-11-01"},
  "created": "2026-10-30T10:00:00Z", "updated": "2026-10-30T10:00:00Z", "deleted_at": null,
  "problems": [{"code": "missing_ref", "field": "blocked_by", "message": "..."}]
}
```

- `props` の値は、単一値なら文字列、複数値なら文字列の配列にする。
- `show --json` はこれに `body`、`watchers`、`repos`、`comments`、`reverse`（関連名ごとの短縮 ID とタイトルの列）を足す。
- `ls --tree --json` の各タスクは、これに `children`、`context`、`in_cycle` を足す。3つとも、空でないとき、または true のときだけ現れる。
- 壊れたファイル（`Entry.Broken`）のタスクは、表では種類とステータスを `?` にし、JSON では `broken` に理由を入れる。

## エラー

- stderr に `hedgehug: <理由>` を出して、終了コード 1 で終わる。フラグや引数の誤りは 2。`-h` は 0。
- 書き込みを拒むエラーは、Store の違反のテキストをそのまま出す（循環なら「... leads back to this task」）。
- `ValidationError` は違反を1行に1つずつ（フィールドとメッセージ）出す。`AmbiguousError` は候補の短縮 ID とタイトルを並べる。`ErrLocked` は、ほかのプロセスが書き込み中である旨を出す。
- `--json` のときもエラーは stderr のテキストで出す。

## ビルド

- `.goreleaser.yaml` に、同じ `main` から `hh` を出す build を足す（親設計の「名前と識別子」のとおり）。`make build` は `hedgehug` だけを作る。

## テスト

- **config**: 環境変数の組み合わせごとの場所（`HEDGEHUG_HOME`、XDG の片方だけ、相対パス、どちらもなし）。config.yaml の読み書きで、未知のキー、要素の未知のキー、コメントが残ること。`~/` の展開。重複した名前と欠けたキーのエラー。Store の選び方の3通り。作者の読み出しとその欠け。
- **store.Create**: スキーマを渡せる。呼び出し側の `meta` が変わらない。途中で失敗したとき、作ったディレクトリまたは中身が残らず、同じ場所に作り直せる。
- **cmd/hedgehug**: `t.TempDir()` を `HEDGEHUG_HOME` と HOME（`.gitconfig` を置く）にして `run` を呼ぶ。コマンドごとに次を確かめる。
  - 出力（表、ツリー、JSON、`--ids-only`、`--format`）
  - 1コマンドが1コミットになり、`Hedgehug-Op` と `Hedgehug-Task` の trailer が付くこと
  - エラーの終了コードとメッセージ（曖昧な ID、存在しない ID、検査の違反、使い方の誤り）
  - `rm` / `restore` の子孫の扱い（同じ rm で消えたものだけ戻る）、`purge` の確認と `--yes`
  - `ls --tree` の祖先の補完と循環の見出し
  - `set` の書き方の各形と、型ごとの値の解釈
- **シナリオ**: フェーズ1の完了条件のうち CLI の分を通しで確かめる1本。`store init` → `add`（親子と孫）→ `ls --tree` → `set` / `status` / `done` / `mv` / `dep` → `rm` → `restore` → `rm` → `purge` の順に実行し、各操作が git 履歴に1コミットずつ残ることを確かめる。

## 持ち越しの回収

1d に回されていた持ち越しのうち、1d-1 で扱うものと 1d-2 に回すもの。

| 持ち越し | 出どころ | 扱い |
| --- | --- | --- |
| `store.Create` が途中で失敗すると半端なディレクトリを残し、再試行が "not empty" で失敗する | 1b | 1d-1（store.Create の修正） |
| `store.Create` が `meta.Format` を呼び出し側の値ごと書き換える | 1b | 1d-1（store.Create の修正） |
| `Roots` は parent の循環に入ったタスクとその配下を返さない。tree 表示で消えないようにする | 1c | 1d-1（`ls --tree` の「循環」の見出し） |
| テンプレート名の存在確認、`hh schema check` でのテンプレート実在チェック | 1a | 1d-2 |
| schema.yaml が壊れると、それを直す PutSchema も含めて全 Update が失敗する | 1b | 1d-2（`doctor`） |
| 作業ツリーの未コミット変更の検出。手で stage した別の変更は次のコミットに混ざる | 1b | 1d-2（`doctor`） |
| schema.yaml が HEAD で壊れていると `index.Open` が失敗する。id とパスが食い違うファイルは index では broken、store では存在扱い | 1c | 1d-2（`doctor` は index に頼らない） |
| `status:` / `kind:` の名前が大文字小文字を区別する。props 名が組み込みキー（`project`、`is`、`sort`、`group`、`blocks`、`label`、`assignee`）だと隠れる | 1c | 1d-2（`schema check` で組み込みキーと重なる props 名を知らせる）。大文字小文字はそのまま |
| `status:todo` はカテゴリ扱い、`project:none` はエラー、`is:blocked` は削除済み・壊れたブロッカーを数えない | 1c | 変えない（1c で決めたとおり） |

## 後続への申し送り

- 1d-2: `show` に直近の履歴を足すため、gitrepo にコミットの履歴（パスで絞る）を読む口を作る。`log` も同じ口を使う。
- 1d-2: `edit` は作業ツリーのファイルを直接開かない。エンコードした内容を一時ファイルに書いて `$EDITOR` で開き、デコードして `PutTask` する。未コミットの変更を作らないため。
- 1d-2: `doctor` は `index.Open` が失敗しても動くよう、store と gitrepo から直接読む。
- フェーズ3: `config.yaml` の `accounts` などを読むようにし、Store の選び方をコンテキスト解決に置き換える。`--store` はそのまま残す。

## 完了条件

- 1d-1 の全コマンドが動き、テストが通る（`make check`）。
- シナリオテストで、1つの Store で CRUD とツリー表示ができ、全操作が git 履歴に1コミットずつ残る。
