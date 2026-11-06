# Stores

A store is a git repository of tasks. Split stores by who can read them: one for
yourself, one per team. Inside a store, workspaces (each with an ID prefix such as `WEB`)
and projects group the tasks. `hh ls` shows every store you have set up, in one list.

## Set up several stores

```console
$ hh store init personal --prefix ME
Created store personal at ~/.local/share/hedgehug/stores/personal with workspace personal (ME)
$ hh store list
NAME      PATH                                     REMOTES                  ACCOUNT           LAST SYNC
team      ~/.local/share/hedgehug/stores/team      /srv/remotes/team.git    github.com/alice  2026-11-06T11:29:19+09:00
personal  ~/.local/share/hedgehug/stores/personal  -                        github.com/alice  never
```

- `hh store init <name>` creates a store. `--remote <url>` pushes it to an empty
  remote; repeat the flag for mirrors (the first is `origin`). `--workspace` and `--prefix`
  name the first workspace and its ID prefix. `--template dev` adds a `review` status and
  rules that move a task with its pull request (see [GitHub](github.md)).
- `hh store add <name> --remote <url>` clones a store someone else made.
- `hh store discover` finds the stores you can read ([Getting started](getting-started.md)).
- `hh store remove <name>` deletes the local copy. It refuses when something is not
  pushed or the store has no remote; `--keep` only forgets the store.
- `hh store set <name> --color teal --hidden` changes how the GUI shows a store, on this
  machine only.

## Accounts

An account is a GitHub login on a host, plus where its token comes from. Each store uses
one account (`--account` on `store init` and `store add`). With one account, every store uses it.

```bash
hh auth add work --login alice-corp --token env:WORK_GH_TOKEN
hh auth add me   --login alice      --token gh
hh auth status
```

When you add a second account, stores that named no account are given the first, so they keep
using it. Your accounts together are "you": `assignee:@me` finds tasks assigned to any of them.

## Where a new task goes

`hh add` picks the store in this order: `--store`, then the project from `hh use`,
then the store you set as the default.

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

`hh use <project-id>` makes a project (a task of kind `project`) the place new tasks go
under; `hh use` shows it and `hh use --clear` stops.

```console
$ hh use WEB-6
→ team / Website is the default for new tasks
$ hh add "Child of website"
Added WEB-7 (...) Child of website
→ added to team / Website
```

Most commands take `--store <name>`. When several stores are set up and a command needs
one (`hh workspace ls`, for example), it asks for `--store`.

## Move a task to another store

```console
$ hh mv WEB-2 --store personal --yes
Moved WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP) → personal ME-2
```

The task keeps its ULID and gets a new ID in the other store. Without `--yes`, the
command asks you to confirm. The old ID still works: `hh show WEB-2` shows
the task as `ME-2`. **The old content stays in the history of the old store**, and
anyone who can read that store can still see it. To remove it, see
[Maintenance](maintenance.md#removing-content-from-history).

`--workspace <ws>` moves a task with its subtree to another workspace and renumbers it; with
`--parent`, `--root`, `--before` and `--after`, `hh mv` also changes the parent and the order
(without `--store`).
