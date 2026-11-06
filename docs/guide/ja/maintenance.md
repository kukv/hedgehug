# メンテナンス

hedgehug が代わりにやってくれないことが2つあります。remote に使う bare リポジトリで `git gc` を実行することと、内容を消す必要があるときに履歴を書き換えることです。このガイドでは、`hh doctor` とシェルの補完も扱います。

## Store を点検する: `hh doctor`

`hh doctor` は、すべての Store のファイル、作業ツリー、git の状態を点検し、見つけた問題を表示します。

```console
$ hh doctor
WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP): moved to a store that is not mounted (ULID 01M6QB1AX3DCHF1FBZNKZ95D4C) (info)
1 problem
```

`hh doctor --fix` は、同期が直すものを1コミットで直します。親の循環、削除済みタスクの子、親と別のワークスペースにあるタスク、ID の衝突です。remote の履歴が書き換えられたあとに、Store を作り直す役目もあります(次の節)。直せなかったものは、そう知らせます。

## bare リポジトリを remote に使うとき

remote に、自分のマシンやサーバーにある bare リポジトリ(GitHub ではないもの)を使うときは、ときどきその中で `git gc` を実行してください。hedgehug は、手元のコピーのオブジェクトは同期のときにまとめますが、remote のオブジェクトには触れません。remote は、バラバラのオブジェクトが増え続けます。

```bash
git -C /srv/remotes/team.git gc
```

GitHub では、GitHub が自分でやります。

## 履歴から内容を消す

hedgehug には、履歴から内容を消すコマンドがありません。Store は git リポジトリで、全員が履歴をすべて持っているからです。タスクを削除(`hh rm`、`hh purge`)しても、別の Store へ移しても、古い内容は履歴に残ります。どうしても消す必要があるとき(たとえば秘密情報を commit したとき)は、履歴を手で書き換え、そのあとで全員の手元を揃えます。漏れた秘密情報は、失効させてください。履歴を書き換えても、すでに clone した人の手元からは消えません。

1. remote を新しく clone し、その中で [`git filter-repo`](https://github.com/newren/git-filter-repo) を使って書き換えます。

   ```bash
   git clone <remote-url> rewrite && cd rewrite
   git filter-repo --path tasks/<a>/<b>/<ULID>.md --invert-paths
   ```

   ファイルごと消すのでなく、文字列だけを伏せるなら `--replace-text` を使います。ミラーがある Store は、ミラーも同じように書き換えます。

2. force push します。`git filter-repo` は `origin` を外すので、足し直します。

   ```bash
   git remote add origin <remote-url>
   git push --force origin main
   ```

3. ほかの人と、あなた自身の Store のディレクトリは、次の同期で書き換えに気づきます。同期は、origin の履歴が自分の追っている履歴を含まなくなったことを見つけて、止まります。

   ```console
   $ hh sync
   hedgehug: sync team: the history of origin differs from this store's; check origin,
   then run hedgehug doctor --fix to rebuild the store on it
   ```

   origin を確かめてから、次を実行します。

   ```console
   $ hh doctor --fix
   == team
   Rebuilt team on the history of origin; put back 1 unpushed commit
   No problems
   ```

   `hh doctor --fix` は origin を clone し直し、push していなかったコミットをその上に戻し、古い履歴を手元のディスクから消します。戻せなかった変更があるときは、古いコピーを残し、そのパスを表示します。

同期が自動で直さないのは、誤った force push があったときに、全員がそれに揃ってしまわないようにするためです。

## シェルの補完

`hh completion bash|zsh|fish` は、コマンド、フラグ、値を補完するスクリプトを出力します。スクリプトは、実行した名前用に作られます。`hh completion zsh` は `hh` を補完し、`hedgehug completion zsh` は `hedgehug` を補完します。使う名前ごとに入れてください。

```bash
hh completion bash > ~/.local/share/bash-completion/completions/hh
hh completion zsh > "${fpath[1]}/_hh"
hh completion fish > ~/.config/fish/completions/hh.fish
```

入れたあとは、新しいシェルを開きます。bash のスクリプトには `bash-completion` パッケージが要ります。入っていないと、補完は動きません(macOS のシステムの bash は古すぎます。Homebrew で `bash` と `bash-completion@2` を入れてください)。

値は、手元の Store から読みます。Tab を押しても、書き込みも同期もしません。読めなかったときは、候補を出しません。

- タスクを取るコマンドと、`--parent`、`--after`、`--before`、`--blocked-by` には、タスク ID(タイトル付き)
- `hh status <id> <TAB>` には、ステータス
- `hh set` には `key=` の形のフィールド名。`assignees=` と `hh ls` の `assignee:` には担当者
- Store(`--store`)、ワークスペース、スプリント、リリース、保存したビュー、テンプレート
