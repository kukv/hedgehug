# はじめに

インストールからチームのタスクを見るまでは、5分で済みます。インストールし、GitHub のアカウントを登録し、チームの Store を見つけて、タスクを読みます。このガイドは、最初のタスクを作るところまでを扱います。

## 1. インストールする

```bash
mise use -g github:kukv/hedgehug@latest
hh --version
```

リリースのアーカイブを使う方法とソースからビルドする方法は、[README](../../../README.ja.md#インストール)にあります。

## 2. アカウントを登録する

アカウントは、あなたが誰か、GitHub のトークンをどこから取るかを hedgehug に教えるものです。`gh` CLI にサインイン済みなら、これがいちばん簡単です。

```bash
hh auth add me --login <GitHub のログイン名> --token gh
hh auth status
```

`--token` には `gh`、`env:<変数名>`(トークンを入れた環境変数)、`keychain` を指定できます。`keychain` を選んだ場合は、あとで `hh auth login me --with-token` を実行し、標準入力からトークンを渡します。`--token` を省くと、そのアカウントはトークンを使いません。remote がローカルのパスなら、それで足ります。

`--with-token` なしの `hh auth login` は、GitHub の Device Flow を使います。`config.yaml` の `hosts[].client_id` に GitHub App の client ID が要ります。ない場合は、`gh`、`env`、`--with-token` のどれかを使ってください。

`config.yaml` は `~/.config/hedgehug/` にあります(macOS では `~/Library/Application Support/hedgehug/`)。

## 3. チームの Store を見つける

```bash
hh store discover
```

アカウントが読める GitHub リポジトリのうち、topic `hedgehug-store` が付いたものを探します。すでに設定済みの Store は除きます。端末では、どれを追加するかを聞かれます。スクリプトの中では一覧を出すだけで、`--all` を付けると全部を追加します。

GitHub なしで試すには、bare リポジトリを置いたディレクトリを `--from` に渡します。

```console
$ hh store discover --from ~/remotes
#  NAME  URL                         ACCOUNT  PERMISSION
1  team  /home/bob/remotes/team.git  bob      write
$ hh store discover --from ~/remotes --all
Added store team
```

## 4. タスクを読み、変更する

```console
$ hh ls
ID     ULID                        STATUS  KIND     ASSIGNEES         LABELS  TITLE
WEB-1  01M6QAZMHYRXKQJSTS7KTHA2GC  todo    task     github.com/alice  ui      Write the login page
WEB-2  01M6QAZMJND1BXZNFVP2PXHFHP  done    bug                                Fix the logout bug
WEB-6  01M6QAZMK2R3Z8Y1W9E4T7H5BD  todo    project                            Website
```

チームの Store にはこれより多くのタスクがあります。例では 3 行だけを残しています（WEB-3 から WEB-5 は省いています）。

`hh show WEB-1` は、1つのタスクを履歴つきで表示します。`hh ls status:todo` で絞り込み、`hh ls --tree` で親の下に並べます。

```bash
hh add "ログインページをレビューする" --parent WEB-6   # 既定の Store の、プロジェクト WEB-6 の下の task
hh status WEB-1 in_progress
hh set WEB-1 assignees=github.com/me labels+=ui
hh done WEB-1
hh sync                                 # 裏の同期を待たず、今すぐ同期する
```

どのコマンドも、まず手元のコピーに書くので、オフラインでも動きます。remote のある Store は、書き込みの約30秒後に裏で同期します。使えるステータスは Store のスキーマで決まります。既定のスキーマには `backlog`、`todo`、`on_hold`、`in_progress`、`waiting`、`done`、`canceled` があります。

## 5. 自分の Store を作る

```bash
hh store init me --prefix ME
hh add "家"                      # 親がなければ project（ME-1）
hh add "牛乳を買う" --parent ME-1  # その下の task
```

ワークスペースの直下に置けるのは project だけなので、task は project の下に置きます（[計画](planning.md#種類)を参照）。

`--remote` を付けない Store は、このマシンの中だけにあります。共有するには、空のリポジトリを用意し、`hh store init` に `--remote <url>` で渡します。GitHub の URL なら、hedgehug がリポジトリを作り、`hh store discover` が探す topic `hedgehug-store` を付けます。

`hh ui` を実行すると、同じタスクをブラウザで見られます。

次は[同期](sync.md)と [Store](stores.md)です。
