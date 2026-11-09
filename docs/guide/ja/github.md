# GitHub

タスクは、pull request や Issue を指せます。`hh sync` のときに、hedgehug が GitHub からその状態(pull request の open / draft / merged、レビュー、チェック、Issue の状態)を読み、タスクに表示し、自動化のルールを適用します。Store のアカウントには、リポジトリを読めるトークンが要ります([はじめに](getting-started.md))。

## タスクをリンクする

```console
$ hh link WEB-1 --issue acme/web#12
Linked WEB-1 (01M6QAZMHYRXKQJSTS7KTHA2GC) Write the login page
$ hh show WEB-1
...
Links:
  issue  acme/web#12  not read yet
```

pull request をリンクするには `hh link <id> --pr owner/repo#n` を使います。`hh unlink` はリンクを追うのをやめ、ファイルには無効にして残します。本文にタスクの印が残っている pull request が、次の同期でまたリンクされるのを防ぐためです。

次のものを見つけたときは、同期のときに hedgehug が自動でリンクします。

- `hh/<ID>-<slug>` という名前のブランチ
- pull request や Issue のタイトル、本文、コメントにある `hedgehug:<ULID>`

1つのタスクに複数のリンクを付けられ、1つの pull request を複数のタスクに付けることもできます。

## タスクから開発を始める

コードのリポジトリの中で実行します。

```bash
hh start WEB-1      # ブランチ hh/WEB-1-<slug> を作り、自分に割り当て、タスクを doing にする
hh pr WEB-1 --draft # ブランチを push し、pull request を作り、リンクする
hh issue WEB-1      # タスクから Issue を作り、push モードでリンクする
```

## 自動化

ルールは Store の `automations.yaml` にあり、イベントごとに動きます。`hh store init --template dev` で作った Store には、最初から次のルールがあります。

```console
$ hh automation list
ID            ON                   WHEN                   SET                 SCOPE  STATE
pr-opened     pr.opened            -                      status=in_progress  store  on
pr-review     pr.review_requested  -                      status=review       store  on
pr-merged     pr.merged            all_linked_prs_merged  category=done       store  on
issue-closed  issue.closed         -                      category=done       store  on
```

```bash
hh automation add --id wip --on pr.opened --status in_progress
hh automation disable pr-review                    # Store 全体で止める
hh automation disable pr-review --project WEB-6    # 1つのプロジェクトだけで止める
hh automation enable pr-review
```

`--on` には `pr.opened`、`pr.review_requested`、`pr.merged`、`issue.closed` を指定できます。`--status` の代わりに `--category done` を使うと、そのカテゴリの既定のステータスになります。`--when all_linked_prs_merged` は、タスクが追っているすべての pull request がマージされるまで待ちます。ルールによる変更は、作者 `hedgehug-automation` でコミットされるので、履歴で区別できます。スキーマのワークフローに反するルールは拒否され、拒否は `hh inbox` に出ます。

## Issue を同期する

Issue のリンクには、モードがあります。

| モード | 正本 | 動き |
| --- | --- | --- |
| `link` | なし | Issue の状態を追うだけです。既定です。 |
| `mirror` | GitHub | Issue のタイトル、状態、担当者をタスクに反映します。 |
| `push` | hedgehug | タスクのフィールドを Issue に反映します。 |

```bash
hh link WEB-1 --issue acme/web#12 --sync push
hh gh import acme/web --label hedgehug --dry-run   # 取り込む件数だけを調べる
hh gh import acme/web --label hedgehug             # open な Issue ごとに mirror モードのタスクを作る
```

`hh automation fields` は、モードごとに反映するフィールドと、既定のモードを表示・変更します。

```console
$ hh automation fields --default-sync push
default_sync: push
push: title, body, status, assignees
mirror: title, status, assignees
props: -
```

ラベルは同期する項目ではありません。ラベルの使い方はリポジトリごと、Organization ごとに違い、Store のラベルのリストとそのまま対応づけられないためです。どのモードも、タスクのラベルを Issue へ送らず、ラベルつきで Issue を作らず、Issue のラベルを取り込みません。`hh automation fields` に `labels` はなく、形式 4 への変換が、既存のルールからも取り除きます（[Store](stores.md#store-の形式と-hh-store-convert)を参照）。`hh gh import --label` は同期ではなく絞り込みで、GitHub のラベルで取り込む Issue を選びます。

`push` モードでは、GitHub 側で手で編集された値は次の push で上書きされ、上書きしたことが `hh inbox` に出ます。

### コメントを同期する

`comments` は同期する項目のひとつです。どのモードにも既定では入っていないので、使いたいモードに足します。

```bash
hh automation fields --push title,body,status,assignees,comments
hh automation fields --mirror title,status,assignees,comments
```

- **先にすべてのクローンを更新します。** `comments` を足す前に、その Store を使うすべてのクローンで hedgehug を更新してください。古い hedgehug は `comments` を含む `automations.yaml` を読めず、そのクローンではルールと Issue の同期が止まります。Store の形式は変わらないので、ほかに警告は出ません。
- **`push`** は、タスクのコメントを Issue のコメントとして作り、編集と削除も反映します。送るのは自分のコメントだけで、作者のないコメントと、一度も送っていない削除済みのコメントは送りません。送るのは各クローンです。GitHub 上の作者は、同期を実行した人になります。GitHub 側で手で編集されたコメントは次の push で上書きされ、上書きしたことが `hh inbox` に「コメント（…）」として出ます。1回の push の実行で作るコメントは、その Store の全 Issue を合わせて30件までで、残りは次の同期へ回します（`comments` を足した直後など、溜まったものが多いときに効きます）。本文はそのまま送るので、`@login` は GitHub でも本物のメンションとして通知になります。`@host/login`（別のホストの人）は、GitHub では `@host` への通知になりうるので注意してください。各コメントの末尾には見えないマーカー `<!-- hedgehug:<ULID> -->` が入り、GitHub の ID の記録が失われても、あとの同期が同じコメントを見つけられます。同じアカウントの2つのクローンが同時に同じコメントを作り、Issue に同じマーカーのコメントが2件できたときは、次の push が記録の指す1件（記録がなければ先頭の1件）を残し、余分を削除します。
- **`mirror`** は、Issue のコメントをタスクのコメントとして取り込み（作者は `github:<login>`）、編集と削除も反映します。2つのクローンが同じコメントを取り込んでも同じファイルになるので、マージで1件に畳まれます。取り込んだコメントは hedgehug で編集も削除もできません。リンクを外しても、モードを変えても同じです（エラーは `comment.mirrored`）。GitHub で変えてください。読むのはタスクの先頭の mirror のリンクだけです。取り込みに件数の上限はなく、通知も抑えません（新しいコメントやメンションは、同期を実行した人にも `hh inbox` に出ます）が、ウォッチする人は増やしません。記録が GitHub のコメントと結びつけている自分のコメント（`push` モードで送ったあとモードを変えたものなど）は、GitHub の本文で書き直されます。前回の同期のあとに hedgehug で編集していたときは、失われた本文が `hh inbox` に出ます。
- GitHub に送ったコメントの追加は、`hh undo` では取り消せません。`hh comment rm` を使ってください。リンクが `push` モードで、同期する項目に `comments` があるあいだは、次の同期で GitHub からも消えます（そうでなければ GitHub には残ります）。
- 対象は Issue のコメントだけで、pull request のコメントは同期しません。
