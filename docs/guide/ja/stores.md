# Store

Store は、タスクを入れる git リポジトリです。読める人の範囲ごとに分けます。自分用に1つ、チームごとに1つ、といった形です。Store の中では、ワークスペース(`WEB` のような ID の接頭辞を持ちます)とプロジェクトがタスクをまとめます。`hh ls` は、設定済みのすべての Store を1つの一覧にして表示します。

## 複数の Store を設定する

```console
$ hh store init personal --prefix ME
Created store personal at ~/.local/share/hedgehug/stores/personal with workspace personal (ME)
$ hh store list
NAME      PATH                                     REMOTES                  ACCOUNT           LAST SYNC
team      ~/.local/share/hedgehug/stores/team      /srv/remotes/team.git    github.com/alice  2026-11-06T11:29:19+09:00
personal  ~/.local/share/hedgehug/stores/personal  -                        github.com/alice  never
```

- `hh store init <name>` は Store を作ります。`--remote <url>` で、空の remote に push します。ミラーを足すには、このフラグを繰り返します(最初のものが `origin` です)。`--workspace` と `--prefix` は、最初のワークスペースの名前と ID の接頭辞です。`--template dev` を付けると、`review` ステータスと、タスクを pull request に合わせて動かすルールが入ります([GitHub](github.md)を参照)。
- `hh store add <name> --remote <url>` は、他の人が作った Store を clone します。
- `hh store discover` は、読める Store を探します([はじめに](getting-started.md))。
- `hh store remove <name>` は、手元のコピーを削除します。push していない変更があるときと、remote がない Store では拒否します。`--keep` なら、Store を設定から外すだけです。
- `hh store set <name> --color teal --hidden` は、GUI での表示を変えます。このマシンだけに効きます。

## アカウント

アカウントは、あるホスト上の GitHub のログイン名と、トークンの取得元の組です。Store ごとに1つのアカウントを使います(`store init` と `store add` の `--account`)。アカウントが1つなら、すべての Store がそれを使います。

```bash
hh auth add work --login alice-corp --token env:WORK_GH_TOKEN
hh auth add me   --login alice      --token gh
hh auth status
```

2つ目のアカウントを足すと、アカウントを指定していなかった Store には最初のアカウントが書き込まれ、使うアカウントは変わりません。登録したアカウントをまとめて「自分」とみなすので、`assignee:@me` はどのアカウントが担当のタスクにも一致します。

## 新しいタスクの行き先

`hh add` は、次の順で行き先の Store を決めます。`--store`、`hh use` で選んだプロジェクト、既定の Store の順です。

```console
$ hh store default personal
personal
$ hh add "Buy milk"
Added ME-1 (...) Buy milk
→ added to personal / personal
$ hh add "Plan the demo" --store team
Added WEB-5 (...) Plan the demo
→ added to team / team
```

`hh use <project-id>` は、プロジェクト(種類が `project` のタスク)を、新しいタスクの親にします。`hh use` で今の設定を表示し、`hh use --clear` で解除します。

```console
$ hh use WEB-6
→ team / Website is the default for new tasks
$ hh add "Child of website"
Added WEB-7 (...) Child of website
→ added to team / Website
```

たいていのコマンドは `--store <name>` を受け取ります。Store が複数あり、コマンドが1つに決められないとき(たとえば `hh workspace ls`)は、`--store` を求められます。

## タスクを別の Store へ移す

```console
$ hh mv WEB-2 --store personal --yes
Moved WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP) → personal ME-2
```

タスクの ULID は変わらず、移動先で新しい ID が振られます。`--yes` を付けないと、確認を求められます。古い ID も使えます。`hh show WEB-2` は、そのタスクを `ME-2` として表示します。**古い内容は、移動元の Store の履歴に残り**、移動元を読める人は今でも見られます。消したいときは、[メンテナンス](maintenance.md#履歴から内容を消す)を参照してください。

`--workspace <ws>` は、タスクを子孫ごと別のワークスペースへ移し、番号を振り直します。`--store` なしなら、`--parent`、`--root`、`--before`、`--after` で、親と並び順も変えられます。
