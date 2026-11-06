# 計画

スプリント、リリース、テンプレートは、ワークスペースに属します。Store にワークスペースが複数あるときは、`hh sprint`、`hh release`、`hh report` に `--workspace <name>` を渡すか、`hh use` でプロジェクトを選びます。

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

レポートは履歴から作るので、事前の設定は要りません。タスクに `hh set <id> points=3` でポイントを付けると、バーンダウンはポイントで数えます。ポイントがなければ、タスクの数で数えます。

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

テンプレートは、Store の `templates/` にあるタスクのファイルです。既定のプロパティと本文を持ち、子タスクを持たせることもできます。`$EDITOR` で書きます。

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
props:
  priority: high
---
## Steps to reproduce

## Expected

## Actual
```

```console
$ hh add "Crash on save" --template bug
```

`--template` を省くと、`hh add` は、そのタスクの種類の既定のテンプレートがスキーマに指定されていれば、それを展開します。`--no-template` で止められます。`hh template show <name>` はファイルを表示し、`hh template rm <name>` は、スキーマが名前を挙げていないテンプレートを削除します。
