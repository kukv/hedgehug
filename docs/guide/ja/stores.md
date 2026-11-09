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

Store は、形式の番号を `.hedgehug/store.yaml` に持ちます。このバージョンの hedgehug は形式 4 を使います。形式 2 でタスクの種類が固定になり（[計画](planning.md#種類)を参照）、形式 3 で関連も固定になり（[計画](planning.md#関連)を参照）、形式 4 で組み込みのプロパティも固定になりました。priority・start・due・estimate・points はタスクの項目になり、priority と labels の値は `schema.yaml` のリストで決めます。

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

形式 2 の Store は、次のように変換されます。

```console
$ hh store convert --dry-run
Format 2 -> 3
  props の duplicates・causes・relates・clones の値を、タスクの同名の項目へ移した（ULID の関連）: 2 件
  culprit: 1 件を relates へ
  相互の組 1 件（A に B・B に A の relates を、どちらも向きありの relates として残した）
  ビュー bugs の query の relates:ME-2 は、これまで両方向に当たっていたが、これからは付けた側にだけ当たる（書き換えていない。逆方向も含めるなら related_by: も使う）
  ビュー bugs の query を書き換えた: relates:ME-2 culprit.inverse:ME-3 → relates:ME-2 related_by:ME-3
  schema.yaml の task_ref のプロパティを消した（値は関連の項目へ移した）: culprit
  store.yaml の format を 2 から 3 にした
Nothing was written (--dry-run).
```

形式 3 の Store は、次のように変換されます（`ME-3` は時刻つきの `due` と、期間でない `estimate` を持ち、`wip` は `schema.yaml` の `labels:` にありません）。

```console
$ hh store convert --dry-run
hedgehug: format 3 to 4: these values cannot be the built-in priority, start, due, estimate or points: they are not of the type of the field, or the task has the field at the top level too; fix them in the files, or drop them with --drop-bad-values (the value of props is kept for a field that is twice, and the one at the top level is dropped):
  ME-3 Logout: props.estimate: 3
$ hh store convert --dry-run --drop-bad-values
Format 3 -> 4
  props の priority・start・due・estimate・points の値を、タスクの同名の項目へ移した: 3 件
  日付にして時刻を落とした: ME-3 Logout: props.due: 2026-11-08T15:30:00+09:00
  ラベル wip を外した（schema.yaml の labels: にない）: タスク 1 件・テンプレート 0 件
  読めない値を外した: ME-3 Logout: props.estimate: 3
  schema.yaml の組み込みと同じ名前のプロパティを消した（値は同名の項目へ移した）: points, due
  store.yaml の format を 3 から 4 にした
Nothing was written (--dry-run).
```

1 つ目のコマンドは `convert.bad_value` で止まり、終了コードは 1 です（`--dry-run` も同じように止まるので、書き込む前に原因が分かります）。

ラベルを GitHub と同期していた Store には、次の行も出ます。この例の `ME-2` は、同期する項目が `labels` だけの Issue リンクを持っていたので、リンクは何も同期しなくなります（`sync: link`）。

```console
$ hh store convert --dry-run
Format 3 -> 4
  GitHub のラベルの同期の跡（gh.issues[].synced と links の fields の labels）を外した: 1 件
  ラベルだけを同期していたリンクの sync を link（同期しない）にした: ME-2 Login: github.com/acme/web#12
  automations.yaml の issue_fields の push・mirror から labels を外した（GitHub のラベルは同期しない）
  store.yaml の format を 3 から 4 にした
Nothing was written (--dry-run).
```

`--dry-run` は、段ごとに変わる内容を一覧するだけで、何も書きません。`--json` は同じ内容を JSON で出します。もう一度実行しても、何も変わりません。

変換は専用のコミット（`op convert`）で、`hh undo` では取り消せません。変換の直後に `hh undo` を実行すると、変換より前の操作が取り消されます。

- 独自の種類は、ワークスペースの直下でまとまりを作っていれば project、その下でまとまりを作っていれば epic、作業の単位なら task になります。`--kind <今の種類>=<新しい種類>`（繰り返せます）は、その種類のタスクを、決まった種類のどれかへ移します。たとえば `--kind sprint=epic` です。
- ワークスペースの直下にあった project 以外のタスクは、「未分類」という project の下へ移ります。なければ作ります。
- story、task、bug の下の story、task、bug は step になります。
- 関連（形式 2 から 3）: `duplicates`、`causes`、`relates`、`clones` の値は、タスクの同名の項目へ移ります。型が `task_ref` のプロパティは、名前が `duplicates`、`causes`、`relates`、`clones` のどれかなら同名の項目へ、それ以外の名前なら値が `relates` へ移り（報告にプロパティごとに 1 行出ます）、どちらも `schema.yaml` から消えます。この型はなくなりました。
- 意味が変わるものと、失うもの:
  - `relates` に向きができました。A に B、B に A の両方に保存されていた組は、2 本の `relates` のまま残り、報告に `相互の組` として数えます。これまでは 1 本の `relates` がどちらのタスクからも見つかりましたが、これからは `relates:A` は A を指しているタスクだけを、`related_by:A` は A が指しているタスクを探します。クエリに `relates:` や `relates.inverse:` があるビューは書き換えず、報告に一覧します。`symmetric: true` を付けていた独自の `task_ref` も同じで、キーも `.inverse:` も両方向に当たっていたので、書き換えずに一覧します。両方向を探したいときは `related_by:` を足してください。
  - ビューのクエリの `<名前>.inverse:` は、逆方向の名前になります（`causes.inverse:` は `caused_by:`、`blocked_by.inverse:` は `blocks:`）。独自の `task_ref`（`symmetric: true` でないもの）の `.inverse:` は `related_by:` に、キーは `relates:` になります。
  - 関連は、ビューの列にも、並べ替えにも、グループにもなりません。`table.columns`、`sort`、`group` から外したものは、報告に一覧します。
  - タスクの ULID の形でない値があると、変換は `convert.bad_value` で止まり、タスクと値を一覧します。ファイルを直すか、`--drop-bad-values` を付けて外してください。外した値は、報告にすべて出ます。存在しない、または削除済みのタスクや、タスク自身を指す値は、そのまま移ります。
  - `task_ref` ではない独自のプロパティが `duplicates`、`causes`、`relates`、`clones` という名前を持っていると、変換は `convert.prop_name_taken` で止まり、名前と型を一覧します。次の節を参照してください。
- 組み込みのプロパティとラベル（形式 3 から 4）:
  - `props:` の下の `priority`、`start`、`due`、`estimate`、`points` の値は、タスクの同名の項目へ移ります（テンプレートも同じです）。この名前の定義は `schema.yaml` から消えます。`priorities:` は、Store にすでにあるリスト、なければ `priority` の定義の値、それもなければ `low`、`mid`、`high`、`urgent` になります。値は大文字小文字を区別せずに照合し、リストの綴りで保存します。
  - リストにないラベルは、タスクとテンプレートから**外れます**。リストは、変換するときの `schema.yaml` のトップレベルの `labels:` です。ない Store は、すべてのラベルが外れます。dry run は、外れるラベルを、タスクの件数つきで一覧します。残すには、[ラベルを残して変換する](#ラベルを残して変換する)を参照してください。
  - 大文字小文字だけが違う値が 2 つあるリスト（`[High, high]`）は、先に書かれた綴りの 1 つに畳みます。空の `priorities: []` は、次の出どころに回します。どちらも報告に出し、変換は止めません。
  - 項目の型として読める値は、直して移します。`"3"` は `3` に、datetime は日付に（書かれていたオフセットのままで日が決まります）、1 件だけの配列はその値に、空文字は外します。時刻が 00:00 でない datetime は時刻を失うので、そのタスクと元の値を報告に一覧します。
  - 読めない値（`3` の `estimate`、`next week` の `due`）や、同じ項目がトップレベルと `props:` の両方にあるものは、`convert.bad_value` で変換を止め、上の例のようにタスクと値を一覧します。ファイルを直すか、`--drop-bad-values` を付けてください。読めない値は外し、項目が両方にあるものは `props:` の値を残してトップレベルの値を外します。外した値は、報告にすべて出ます。
  - GitHub とはラベルを同期しなくなりました。`automations.yaml` の同期する項目から `labels` が消え、リンクした Issue の記録からも `labels` が消えます。ラベルだけを同期していたリンクは `sync: link`（同期しない）になり、報告にそのリンクを一覧します。`hh gh import --label` は残ります。GitHub のラベルで取り込む Issue を選ぶ絞り込みです。
- `--store <name>` で Store を選びます。

ほかのクローンがすでに remote を変換していたとき、push していないコミットのないクローンでは、`hh store convert` は自分では変換せず、remote へ fast-forward します。push していないコミットが残っているクローンは、まず自分の変更を変換します。[同期](sync.md#remote-が新しい形式になったとき)を参照してください。remote が途中の形式なら、クローンをその形式まで変換して同期し、残りの段を当ててから push します。

### 関連と同じ名前の独自のプロパティ

形式 2 の Store は `hh` で編集できません（`hh store convert` と `hh doctor` 以外のコマンドが止まります）。手で直してください。Store のフォルダ（場所は `hh store list` で分かります）で、次のようにします。

1. `schema.yaml` の `props:` の下で、プロパティの名前を変えます（たとえば `causes` を `crash_cause` に）。
2. `tasks/` の下のファイルで、そのキーを持つタスクの `props:` の同じキーの名前を変えます。
3. コミットして（`git add -A && git commit -m "rename properties"`）、もう一度 `hh store convert` を実行します。

逆方向の名前（`blocks`、`caused_by`、`related_by`、`duplicated_by`、`cloned_by`）と同じ名前の独自のプロパティは、変換を止めません。そのまま動きますが、クエリのキーが関連の意味になるので、`hh doctor` が警告します（`prop_name.query_key`）。同じ手順か、変換のあとに `hh schema migrate prop` で名前を変えてください。

### ラベルを残して変換する

形式 3 の Store は `hh` で編集できないので、残したいラベルのリストは、変換の前に手で書きます。`schema.yaml` に `labels:` がないと、すべてのラベルが外れます。Store のフォルダ（場所は `hh store list` で分かります）で、次のようにします。

1. タスクが持っているラベルを調べます。形式 3 の Store では `hh ls` が止まるので、変換に聞きます。`schema.yaml` に `labels:` がない状態で `hh store convert --dry-run` を実行すると、すべてのラベルが、持っているタスクの件数つきで一覧されます（何も書きません）。読めない値がある Store では、`convert.bad_value`（上を参照）で止まってラベルの一覧が出ません。そのときは `--drop-bad-values` を付けてください（dry run なので、値を外したり書いたりはしません）。ファイルを見てもかまいません（`grep -rh "labels:" tasks/ templates/`。テンプレートの子のタスクのラベルはインデントされているので、行頭を指定しません）。

```console
$ hh store convert --dry-run
Format 3 -> 4
  ラベル bug を外した（schema.yaml の labels: にない）: タスク 2 件・テンプレート 0 件
  ラベル ui を外した（schema.yaml の labels: にない）: タスク 1 件・テンプレート 0 件
  ラベル wip を外した（schema.yaml の labels: にない）: タスク 1 件・テンプレート 0 件
  store.yaml の format を 3 から 4 にした
Nothing was written (--dry-run).
```

2. `schema.yaml` のトップレベルに、残すラベルを `labels:` として書きます（`labels: [bug, ui]`）。変換は大文字小文字を区別せずに照合し、リストの綴りで保存します。
3. コミットして（`git add -A && git commit -m "labels"`）、もう一度 `hh store convert --dry-run` を実行します。まだ外れるラベル（下の例では `wip`）だけが一覧されます。一覧が思ったとおりなら、`hh store convert` を実行してください。

```console
$ hh store convert --dry-run
Format 3 -> 4
  ラベル wip を外した（schema.yaml の labels: にない）: タスク 1 件・テンプレート 0 件
  store.yaml の format を 3 から 4 にした
Nothing was written (--dry-run).
```

変換のあとは、`hh schema labels add <名前>…` でリストにラベルを足し、`hh schema labels rm <名前>…` でどのタスクも持っていないラベルを消し、`hh schema migrate label <旧> <新>` で名前を変える、またはほかのラベルへマージします（[はじめに](getting-started.md#4-タスクを読み変更する)を参照）。

### remote に古い形式の変更があるとき

あるクローンが変換したあと、push するまでの間に、別のクローンが古い形式のコミットを push することがあります。変換したクローンは最新の形式で、remote はそれより古い形式のまま、手元にないコミットを持つ状態です。`hh sync` は `sync.format_diverged`（HTTP 409）で止まり、メッセージは「remote には古い形式の変更があるので、`hh store convert` で変換してください」と案内します。`hh store convert` を実行してください。remote の最新のコミットのファイルに変換の段を当て、その結果を、そのコミットの上のコミットとして push します（remote は前に進むだけで、書き換えません）。そのあと通常どおり同期して、手元の履歴と変換した変更を 1 つにマージし、push します。ほかのクローンは、変換していなければ `hh store convert` で fast-forward し、変換済みなら通常の同期で追いつきます。`--dry-run` は、変換する内容を一覧するだけで、何も書きません。その間にまた別のクローンが push したときは `store.convert_unsynced` で止まるので、もう一度実行してください。push のあとの、マージのための同期が失敗したとき（書き込む予定のファイルにコミットしていない変更がある、など）は、remote の変更の変換はもう push 済みで、`hh store convert` にすることは残っていません。原因を直してから `hh sync` を実行してください。

### 変換の前に

`hh store convert` を実行する前に、その Store を使う**すべてのクローン**で hedgehug を更新してください。形式の検査を持たない古い hedgehug は、形式を見ません。push していないコミットのある古いクローンが、変換済みの remote と同期すると、独自の種類のタスクや、ワークスペース直下のタスクを、形式 2 の Store へ push してしまいます。そうなると、`hh store convert` は変換するものがなく（すでに形式 2 です）、`hh doctor --fix` でも直せません。起きたときは、`hh doctor` で探します。コード `task.unknown_kind`（`kind: unknown kind "sprint"`）と `task.placement`（`parent: a task cannot be placed under root`）の指摘が、そのタスクを示します。それぞれを `hh edit <id>` で開き、`kind:` を決まった種類のどれかに、または `parent:` を置けるタスク（直下ならプロジェクトなど）に直して、もう一度 `hh doctor` を実行してください。

関連と組み込みのプロパティも同じです。形式 3 より前のクローンは `duplicates`、`causes`、`relates`、`clones` を、形式 4 より前のクローンは `priority`、`start`、`due`、`estimate`、`points` を、`props:` の下に書き続けます。新しい形式では、これらはタスクの項目です。`hh doctor` が `task.legacy_field` として指摘し、`hh doctor --fix` がタスクの項目へ移します（[メンテナンス](maintenance.md#store-を点検する-hh-doctor)を参照）。独自の `task_ref` はこの 4 つの名前ではないので、`hh doctor` は指摘せず、`--fix` でも移りません。`hh edit <id>` でタスクを開き、値を `relates:` に移してください。

## タスクを別の Store へ移す

```console
$ hh mv WEB-2 --store personal --yes
Moved WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP) → personal ME-2
```

タスクの ULID は変わらず、移動先で新しい ID が振られます。`--yes` を付けないと、確認を求められます。古い ID も使えます。`hh show WEB-2` は、そのタスクを `ME-2` として表示します。**古い内容は、移動元の Store の履歴に残り**、移動元を読める人は今でも見られます。消したいときは、[メンテナンス](maintenance.md#履歴から内容を消す)を参照してください。

`--workspace <ws>` は、タスクを子孫ごと別のワークスペースへ移し、番号を振り直します。`--store` なしなら、`--parent`、`--root`、`--before`、`--after` で、親と並び順も変えられます。
