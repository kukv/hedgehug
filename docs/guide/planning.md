# Planning

Sprints, releases and templates belong to a workspace. When a store has several
workspaces, give `--workspace <name>` to `hh sprint`, `hh release` and `hh report`, or
choose a project with `hh use`.

## Kinds

The kinds of task are fixed, and each one sits in a fixed place. You cannot add or rename
a kind (the schema has no `kinds` any more).

| Kind | Role | Goes under | Holds |
|---|---|---|---|
| `project` | group | the workspace (the root) | epic, story, task, bug |
| `epic` | group | a project | story, task, bug |
| `story` | item | a project or an epic | step |
| `task` | item | a project or an epic | step |
| `bug` | item | a project or an epic | step |
| `step` | item | a story, a task or a bug | nothing |

Stories, tasks and bugs are siblings: a task does not go under a story. The root of a
workspace holds only projects, and a project does not go under a project. `hh add`,
`hh mv` and the screens refuse a place the table does not allow.

When you give no `--kind`, `hh add` decides it from the parent: a project with no parent,
a task under a project or an epic, a step under a story, a task or a bug. So
`hh add "Website"` makes a project, and `hh add "Login page" --parent WEB-1` makes a task
in it. A template whose child names no kind follows the same rule.

Progress counts the stories, tasks and bugs under a project or an epic. Steps are not
counted; the row of their parent shows how many are done, such as `2/3` (in the browser and in `hh ls --tree`; `hh show` has a `steps` line).

## Relations

A task can point at other tasks of the same store through five relations. They are fixed
like the kinds: you cannot add, rename or remove one. Each has a direction. The task that
sets it shows the name, and the task it points at shows the name of the other way.

| Relation | Other way | Use it for |
|---|---|---|
| `blocked_by` | `blocks` | the order of the work: this one waits for that one |
| `causes` | `caused_by` | a problem and its effect, once it has happened |
| `relates` | `related_by` | the two have something to do with each other |
| `duplicates` | `duplicated_by` | the same content twice; use it when you close one of them |
| `clones` | `cloned_by` | a task made as a copy of another |

`hh dep add` takes any of them, and several tasks per flag (repeat the flag or separate
with commas). Everything is written in one commit. `hh dep rm` takes the same flags.

```console
$ hh dep add WEB-2 --causes WEB-3 --clones WEB-1
Updated WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP) Fix the logout bug
$ hh dep rm WEB-2 --clones WEB-1
```

`hh show WEB-3` lists the task's own relations, and under `Related:` the ones that point at
it, such as `Caused by`. The detail in the browser shows the same in its Relations
section, where you can add and remove them. A task cannot point at itself (`ref.self`),
and `blocked_by` still refuses a cycle.

In a query, the key is the name of the relation or the name of the other way:
`hh ls causes:WEB-3` finds the tasks that cause `WEB-3` (the ones that have it in their
`causes`), and `hh ls caused_by:WEB-2` the tasks `WEB-2` causes. The old `causes.inverse:` is
gone; the error for it names `caused_by:`. `relates` has a direction now, so `relates:WEB-2`
finds only the tasks that point at `WEB-2`. To find everything that is related to it, run
`hh ls relates:WEB-2` and `hh ls related_by:WEB-2`: there is no OR in a query. Relations are
not a column, a sort or a group of a view.

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
`hh set <id> points=3` (`points` is a field of the task, like `priority`, `start`, `due` and `estimate`) and the burndown
counts points, also from the commits made before the store was converted to format 4; without points it counts tasks.

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
and optionally child tasks. The built-in properties (`priority`, `start`, `due`, `estimate`, `points`, `labels`) are keys at the top of
the file, as in a task; a label that is not in the list of the store refuses the expansion (`the change breaks the schema`), `hh doctor` reports the template (`template.bad_label`, with the path of the label inside it), and `hh template edit` fixes it. Write one in your `$EDITOR`:

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
priority: high
---
## Steps to reproduce

## Expected

## Actual
```

```console
$ hh add "Crash on save" --template bug
```

If a template names a label that is not in the list of the store, `hh add --template` refuses with `the change breaks the schema`, and `hh doctor` reports it:

```console
$ hh doctor
templates/lab.md: children[0].labels: label "docs" is not in the list of labels of the schema
  hint: add the label with `hedgehug schema labels add`, or fix the template with `hedgehug template edit lab`
templates/lab.md: labels: label "zzz" is not in the list of labels of the schema
  hint: add the label with `hedgehug schema labels add`, or fix the template with `hedgehug template edit lab`
2 problems
```

Without `--template`, `hh add` expands the default template of the task's kind, if the
schema names one in `templates.default` (a kind and the template it uses); `--no-template`
turns that off. In the browser, choose them on Settings, Templates: each kind has a
"<kind> の既定" menu there. A project can override them in its own settings.
`hh template show <name>` prints a file, and `hh template rm <name>` deletes one that the
schema does not name.
