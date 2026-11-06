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
push: title, body, status, assignees, labels
mirror: title, status, assignees
props: -
```

`push` モードでは、GitHub 側で手で編集された値は次の push で上書きされ、上書きしたことが `hh inbox` に出ます。
