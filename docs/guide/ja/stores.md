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
$ hh add "Buy milk" --parent ME-1
Added ME-2 (...) Buy milk
→ added to personal / Home
$ hh add "Plan the demo" --store team --parent WEB-6
Added WEB-8 (...) Plan the demo
→ added to team / Website
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

## Store の形式と `hh store convert`

Store は、形式の番号を `.hedgehug/store.yaml` に持ちます。このバージョンの hedgehug は形式 2 を使います。形式 2 では、タスクの種類が固定です（[計画](planning.md#種類)を参照）。

- **古い**形式の Store は開きません。`hh store convert` と `hh doctor` 以外のコマンドは、`hh store convert で新しい形式にしてください` と言って止まります。ブラウザも、サイドバーの Store のカードと設定で同じ文を出します。`hh doctor` は、形式の番号を知らせ、このコマンドを案内します。
- **新しい**形式の Store も開きません。hedgehug を更新してください。

`hh store convert` は、Store を最新の形式に、1 段ずつ変換し、全部を 1 コミットで書きます。remote のある Store は、結果を push します。remote が同じ古い形式で、手元と違いがあるとき（push していないコミットがある、または手元にない remote のコミットがある）は、コマンドがまず古い形式のまま同期（取り込み・マージ・push）してから変換して push します。1 回の実行で済みます。この同期が失敗したとき（push が拒否された、マージで止まったなど）は、`store.convert_unsynced`（HTTP 409）で止まり、メッセージが原因を示します。`--dry-run` は同期も書き込みもせず、先に同期する旨を出します。

```console
$ hh store convert --dry-run
Format 1 -> 2
  ワークスペース me の直下のタスクを「未分類」プロジェクトの下へ移した: 1 件（「未分類」を作った: ME-3）
  schema.yaml の kinds を消した
  store.yaml の format を 1 から 2 にした
Nothing was written (--dry-run).
$ hh store convert
```

`--dry-run` は、段ごとに変わる内容を一覧するだけで、何も書きません。`--json` は同じ内容を JSON で出します。もう一度実行しても、何も変わりません。

- 独自の種類は、ワークスペースの直下でまとまりを作っていれば project、その下でまとまりを作っていれば epic、作業の単位なら task になります。`--kind <今の種類>=<新しい種類>`（繰り返せます）は、その種類のタスクを、決まった種類のどれかへ移します。たとえば `--kind sprint=epic` です。
- ワークスペースの直下にあった project 以外のタスクは、「未分類」という project の下へ移ります。なければ作ります。
- story、task、bug の下の story、task、bug は step になります。
- `--store <name>` で Store を選びます。

ほかのクローンがすでに remote を変換していたとき、push していないコミットのないクローンでは、`hh store convert` は自分では変換せず、remote へ fast-forward します。push していないコミットが残っているクローンは、まず自分の変更を変換します。[同期](sync.md#remote-が新しい形式になったとき)を参照してください。remote が途中の形式なら、クローンをその形式まで変換して同期し、残りの段を当ててから push します。

### remote に古い形式の変更があるとき

あるクローンが変換したあと、push するまでの間に、別のクローンが古い形式のコミットを push することがあります。変換したクローンは最新の形式で、remote はそれより古い形式のまま、手元にないコミットを持つ状態です。`hh sync` は `sync.format_diverged`（HTTP 409）で止まり、メッセージは「remote には古い形式の変更があるので、`hh store convert` で変換してください」と案内します。`hh store convert` を実行してください。remote の最新のコミットのファイルに変換の段を当て、その結果を、そのコミットの上のコミットとして push します（remote は前に進むだけで、書き換えません）。そのあと通常どおり同期して、手元の履歴と変換した変更を 1 つにマージし、push します。ほかのクローンは、変換していなければ `hh store convert` で fast-forward し、変換済みなら通常の同期で追いつきます。`--dry-run` は、変換する内容を一覧するだけで、何も書きません。その間にまた別のクローンが push したときは `store.convert_unsynced` で止まるので、もう一度実行してください。push のあとの、マージのための同期が失敗したとき（書き込む予定のファイルにコミットしていない変更がある、など）は、remote の変更の変換はもう push 済みで、`hh store convert` にすることは残っていません。原因を直してから `hh sync` を実行してください。

### 変換の前に

`hh store convert` を実行する前に、その Store を使う**すべてのクローン**で hedgehug を更新してください。形式の検査を持たない古い hedgehug は、形式を見ません。push していないコミットのある古いクローンが、変換済みの remote と同期すると、独自の種類のタスクや、ワークスペース直下のタスクを、形式 2 の Store へ push してしまいます。そうなると、`hh store convert` は変換するものがなく（すでに形式 2 です）、`hh doctor --fix` でも直せません。起きたときは、`hh doctor` で探します。コード `task.unknown_kind`（`kind: unknown kind "sprint"`）と `task.placement`（`parent: a task cannot be placed under root`）の指摘が、そのタスクを示します。それぞれを `hh edit <id>` で開き、`kind:` を決まった種類のどれかに、または `parent:` を置けるタスク（直下ならプロジェクトなど）に直して、もう一度 `hh doctor` を実行してください。

## タスクを別の Store へ移す

```console
$ hh mv WEB-2 --store personal --yes
Moved WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP) → personal ME-2
```

タスクの ULID は変わらず、移動先で新しい ID が振られます。`--yes` を付けないと、確認を求められます。古い ID も使えます。`hh show WEB-2` は、そのタスクを `ME-2` として表示します。**古い内容は、移動元の Store の履歴に残り**、移動元を読める人は今でも見られます。消したいときは、[メンテナンス](maintenance.md#履歴から内容を消す)を参照してください。

`--workspace <ws>` は、タスクを子孫ごと別のワークスペースへ移し、番号を振り直します。`--store` なしなら、`--parent`、`--root`、`--before`、`--after` で、親と並び順も変えられます。
