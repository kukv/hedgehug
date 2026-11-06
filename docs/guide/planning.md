# Planning

Sprints, releases and templates belong to a workspace. When a store has several
workspaces, give `--workspace <name>` to `hh sprint`, `hh release` and `hh report`, or
choose a project with `hh use`.

## Sprints

A sprint is `planned`, then `active`, then `closed`.

```console
$ hh sprint add "Sprint 2" --start 2026-11-16 --end 2026-11-27 --goal "Fix crashes"
Added sprint Sprint 2
$ hh set WEB-8 "sprint=Sprint 2"
Updated WEB-8 (01M6QB329R52TP0EREQD1HDFAT) Crash on save
$ hh sprint start "Sprint 2"
Started sprint Sprint 2 (2026-11-16 to 2026-11-27)
$ hh ls sprint:current
```

Put a task in a sprint when you create it with `hh add --sprint`, or later with
`hh set <id> sprint=<name>`. `sprint:current` is the active sprint of each workspace.
`hh sprint set <name> goal=...` changes the name, dates or goal; `hh sprint ls` lists the sprints.

At the end, close the sprint and say where the unfinished tasks go: `next`, `backlog`
or the name of a sprint.

```console
$ hh sprint close "Sprint 1" --to next
Closed sprint Sprint 1
$ hh sprint ls
NAME      WORKSPACE  STATE   START       END         DONE  POINTS
Sprint 1  team       closed  2026-11-02  2026-11-13  1/1   0
```

## Releases

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

`hh release notes` prints the finished tasks of the release in Markdown (its section
headings are in Japanese). `hh release set v1.0 state=released` changes the state when you ship.

## Reports

Reports are read from the history, so they need no setup. Give a task points with
`hh set <id> points=3` and the burndown counts points; without points it counts tasks.

```console
$ hh report burndown "Sprint 2"
Sprint 2 (active), by points
DATE        SCOPE  DONE  LEFT  ADDED
2026-11-06  8      0     8     0
$ hh report velocity
SPRINT    POINTS  TASKS  NO POINTS
Sprint 1  0       1      1
```

- `burndown [sprint]`: what a sprint holds and has left, day by day. `--by count` counts tasks.
- `velocity`: what the closed sprints finished (`--last <n>` limits how many).
- `cycle`: how long finished tasks took, as median and P85.
- `cfd`: how many tasks each category holds, day by day.
- `estimate`: the estimates against the time logged with `hh worklog`.

## Templates

A template is a task file in `templates/` of the store: default properties and a body,
and optionally child tasks. Write one in your `$EDITOR`:

```console
$ hh template edit bug
Saved template bug
$ hh template ls
NAME  KIND  TITLE  CHILDREN
bug   bug   Bug:   0
```

The file looks like this:

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

Without `--template`, `hh add` expands the default template of the task's kind, if the
schema names one; `--no-template` turns that off. `hh template show <name>` prints a file,
and `hh template rm <name>` deletes one that the schema does not name.
