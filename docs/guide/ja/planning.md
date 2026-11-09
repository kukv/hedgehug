# 計画

スプリント、リリース、テンプレートは、ワークスペースに属します。Store にワークスペースが複数あるときは、`hh sprint`、`hh release`、`hh report` に `--workspace <name>` を渡すか、`hh use` でプロジェクトを選びます。

## 種類

タスクの種類は固定で、それぞれ置ける場所が決まっています。種類を足したり名前を変えたりはできません（スキーマに `kinds` はありません）。

| 種類 | 役割 | 置ける場所 | 子に置ける種類 |
|---|---|---|---|
| `project` | グループ | ワークスペースの直下 | epic、story、task、bug |
| `epic` | グループ | project の下 | story、task、bug |
| `story` | アイテム | project か epic の下 | step |
| `task` | アイテム | project か epic の下 | step |
| `bug` | アイテム | project か epic の下 | step |
| `step` | アイテム | story、task、bug の下 | なし |

story、task、bug は同じ階層の兄弟です。story の下に task は置けません。ワークスペースの直下に置けるのは project だけで、project の下に project は置けません。`hh add`、`hh mv`、画面は、表で許されない場所を断ります。

`--kind` を省くと、`hh add` は親から種類を決めます。親がなければ project、project か epic の下なら task、story、task、bug の下なら step です。たとえば `hh add "Website"` は project を、`hh add "Login page" --parent WEB-1` はその中の task を作ります。テンプレートの子が種類を書いていないときも、同じ決まりです。

進捗は、project や epic の下の story、task、bug を数えます。step は数えず、親の行に完了した数を `2/3` のように出します（画面と `hh ls --tree` の行、`hh show` の `steps` の行）。

## 関連

タスクは、同じ Store のほかのタスクを、5 つの関連で指せます。関連も種類と同じく固定で、足したり、名前を変えたり、消したりはできません。どの関連にも向きがあります。付けた側のタスクには関連の名前が、付けられた側のタスクには逆方向の名前が出ます。

| 関連 | 逆方向 | 使いどころ |
|---|---|---|
| `blocked_by` | `blocks` | 作業の順番。これは、あれが終わるまで待つ |
| `causes` | `caused_by` | 問題とその結果。すでに起きたことについて使う |
| `relates` | `related_by` | 2 つのタスクに関係がある |
| `duplicates` | `duplicated_by` | 同じ内容の重複。片方を閉じるときに使う |
| `clones` | `cloned_by` | 別のタスクの複製として作ったタスク |

`hh dep add` は、どの関連も受け取ります。1 つのフラグに複数のタスクを渡せます（フラグを繰り返すか、コンマで区切ります）。全部が 1 コミットで書かれます。`hh dep rm` も同じフラグを取ります。

```console
$ hh dep add WEB-2 --causes WEB-3 --clones WEB-1
Updated WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP) Fix the logout bug
$ hh dep rm WEB-2 --clones WEB-1
```

`hh show WEB-3` は、そのタスク自身の関連を出し、`Related:` の下に、そのタスクを指している関連を `Caused by` のように出します。ブラウザの詳細も、関連の節で同じものを出し、そこで追加と削除ができます。自分自身は指せません（`ref.self`）。`blocked_by` は、これまでどおり循環も断ります。

クエリのキーは、関連の名前か、逆方向の名前です。`hh ls causes:WEB-3` は `WEB-3` を引き起こしたタスク（自分の `causes` に `WEB-3` を持つタスク）を、`hh ls caused_by:WEB-2` は `WEB-2` が引き起こしたタスクを探します。これまでの `causes.inverse:` はなくなり、このキーを書くと、エラーが `caused_by:` を案内します。`relates` にも向きがあるので、`relates:WEB-2` は `WEB-2` を指しているタスクだけを探します。`WEB-2` に関係のあるものをすべて探すときは、`hh ls relates:WEB-2` と `hh ls related_by:WEB-2` の 2 回で求めます。クエリに OR はありません。関連は、ビューの列にも、並べ替えにも、グループにもなりません。

## スプリント

スプリントの状態は、`planned`、`active`、`closed` の順に進みます。

```console
$ hh sprint add "Sprint 2" --start 2026-11-16 --end 2026-11-27 --goal "Fix crashes"
Added sprint Sprint 2
$ hh set WEB-8 "sprint=Sprint 2"
Updated WEB-8 (01M6QB329R52TP0EREQD1HDFAT) Crash on save
$ hh sprint start "Sprint 2"
Started sprint Sprint 2 (2026-11-16 to 2026-11-27)
$ hh ls sprint:current
```

タスクをスプリントに入れるには、作るときに `hh add --sprint`、あとからなら `hh set <id> sprint=<name>` を使います。`sprint:current` は、各ワークスペースの active なスプリントを指します。`hh sprint set <name> goal=...` で名前、期間、ゴールを変え、`hh sprint ls` でスプリントを一覧します。

スプリントの終わりには、スプリントを閉じ、終わらなかったタスクの行き先(`next`、`backlog`、またはスプリントの名前)を指定します。

```console
$ hh sprint close "Sprint 1" --to next
Closed sprint Sprint 1
$ hh sprint ls
NAME      WORKSPACE  STATE   START       END         DONE  POINTS
Sprint 1  team       closed  2026-11-02  2026-11-13  1/1   0
```

## リリース

```console
$ hh release add v1.0 --date 2026-12-01
Added release v1.0
$ hh set WEB-3 releases+=v1.0
$ hh release ls
NAME  WORKSPACE  STATE       DATE        DONE
v1.0  team       unreleased  2026-12-01  0/1
$ hh done WEB-3
$ hh release notes v1.0
# v1.0

## タスク

- WEB-3 Alice only task
```

`hh release notes` は、リリースに入った完了済みのタスクを Markdown で出力します。`hh release set v1.0 state=released` で、出荷したときに状態を変えます。

## レポート

レポートは履歴から作るので、事前の設定は要りません。タスクに `hh set <id> points=3` でポイントを付けると（`points` は、`priority`、`start`、`due`、`estimate` と同じくタスクの項目です）、バーンダウンはポイントで数えます。形式 4 に変換する前のコミットのポイントも、履歴から読みます。ポイントがなければ、タスクの数で数えます。

```console
$ hh report burndown "Sprint 2"
Sprint 2 (active), by points
DATE        SCOPE  DONE  LEFT  ADDED
2026-11-06  8      0     8     0
$ hh report velocity
SPRINT    POINTS  TASKS  NO POINTS
Sprint 1  0       1      1
```

- `burndown [sprint]`: スプリントが抱えているものと残りを、日ごとに出します。`--by count` でタスクの数で数えます。
- `velocity`: 閉じたスプリントが終わらせた量です。`--last <n>` で件数を絞ります。
- `cycle`: 完了したタスクにかかった時間を、中央値と P85 で出します。
- `cfd`: カテゴリごとのタスクの数を、日ごとに出します。
- `estimate`: 見積もりと、`hh worklog` で記録した時間を比べます。

## テンプレート

テンプレートは、Store の `templates/` にあるタスクのファイルです。既定のプロパティと本文を持ち、子タスクを持たせることもできます。`$EDITOR` で書きます。組み込みのプロパティ（`priority`、`start`、`due`、`estimate`、`points`、`labels`）は、タスクと同じく、ファイルの先頭のキーに書きます。Store のリストにないラベルがあると、展開は `the change breaks the schema` で拒否されます。`hh doctor` は、そのテンプレートを `template.bad_label` として、テンプレートの中のラベルの場所つきで指摘します。直すには `hh template edit` を使います。

```console
$ hh template edit bug
Saved template bug
$ hh template ls
NAME  KIND  TITLE  CHILDREN
bug   bug   Bug:   0
```

ファイルは次の形です。

```markdown
---
kind: bug
title: "Bug: "
priority: high
---
## Steps to reproduce

## Expected

## Actual
```

```console
$ hh add "Crash on save" --template bug
```

テンプレートがリストにないラベルを持つと、`hh add --template` は `the change breaks the schema` で断り、`hh doctor` が指摘します。

```console
$ hh doctor
templates/lab.md: children[0].labels: label "docs" is not in the list of labels of the schema
  hint: add the label with `hedgehug schema labels add`, or fix the template with `hedgehug template edit lab`
templates/lab.md: labels: label "zzz" is not in the list of labels of the schema
  hint: add the label with `hedgehug schema labels add`, or fix the template with `hedgehug template edit lab`
2 problems
```

`--template` を省くと、`hh add` は、そのタスクの種類の既定のテンプレートがスキーマの `templates.default`（種類と、使うテンプレートの組）に指定されていれば、それを展開します。`--no-template` で止められます。ブラウザでは、設定のテンプレート画面で選びます。種類ごとに「<種類> の既定」のメニューがあります。プロジェクトは、自分の設定で上書きできます。`hh template show <name>` はファイルを表示し、`hh template rm <name>` は、スキーマが名前を挙げていないテンプレートを削除します。
