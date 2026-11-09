# Getting started

You can go from install to your team's tasks in five minutes: install, register your
GitHub account, find the team's store, read the tasks. This guide ends with your first
task.

## 1. Install

```bash
mise use -g github:kukv/hedgehug@latest
hh --version
```

For the other ways to install (a release archive, or building from source), see the
[README](../../README.md#install).

## 2. Register your account

An account tells hedgehug who you are and where to get a GitHub token. The
simplest source is the `gh` CLI, if you have it signed in:

```bash
hh auth add me --login <your-github-login> --token gh
hh auth status
```

`--token` takes `gh`, `env:<VAR>` (a variable that holds the token) or `keychain`. With
`keychain`, store the token afterwards with `hh auth login me --with-token` (the token
comes from stdin). Without `--token`, the account uses no token, which is enough for
remotes that are local paths.

`hh auth login` without `--with-token` runs GitHub's device flow. It needs a GitHub App's
client ID in `hosts[].client_id` of `config.yaml`; without one, use `gh`, `env` or `--with-token`.

`config.yaml` is in `~/.config/hedgehug/` (on macOS, `~/Library/Application Support/hedgehug/`).

## 3. Find your team's store

```bash
hh store discover
```

It lists the GitHub repositories with the `hedgehug-store` topic that your accounts can
read, leaving out the ones you have set up. On a terminal it asks which to add. In a
script it only lists them; add them all with `--all`.

To try it without GitHub, point it at a directory of bare repositories:

```console
$ hh store discover --from ~/remotes
#  NAME  URL                         ACCOUNT  PERMISSION
1  team  /home/bob/remotes/team.git  bob      write
$ hh store discover --from ~/remotes --all
Added store team
```

## 4. Read and change tasks

```console
$ hh ls
ID     ULID                        STATUS  KIND     ASSIGNEES         LABELS  TITLE
WEB-1  01M6QAZMHYRXKQJSTS7KTHA2GC  todo    task     github.com/alice  ui      Write the login page
WEB-2  01M6QAZMJND1BXZNFVP2PXHFHP  done    bug                                Fix the logout bug
WEB-6  01M6QAZMK2R3Z8Y1W9E4T7H5BD  todo    project                            Website
```

The team's store already holds more tasks than this; the example keeps three rows (WEB-3 to
WEB-5 are left out).

`hh show WEB-1` shows one task with its history. `hh ls status:todo` filters, and
`hh ls --tree` shows the tasks under their parents.

```bash
hh add "Review the login page" --parent WEB-6   # a task under the project WEB-6, in the default store
hh status WEB-1 in_progress
hh schema labels add ui              # a label has to be in the store's list before a task can hold it
hh set WEB-1 assignees=github.com/me labels+=ui
hh dep add WEB-2 --blocked-by WEB-1  # WEB-2 waits for WEB-1; see Relations in Planning
hh done WEB-1
hh sync                              # sync now, without waiting for the background sync
```

A task can hold only the labels in the list of the store. Add them with `hh schema labels add <name>…`
(in the browser: Settings, Schema, the tab "ラベル"). A label matches without regard to case and is stored as the list
spells it: with `bug` in the list, `labels+=Bug` stores `bug`. A name that is not in the list stops with
`the change breaks the schema` (`labels: "nope" is not one of [bug ui docs]`).

To change the list later, `hh schema labels rm <name>…` removes labels no task holds, and
`hh schema migrate label <old> <new>` renames a label, or merges it into another, on the tasks that hold it.
`hh schema migrate priority <old> <new>` does the same for a priority. Neither rewrites the `query` of a saved view.

Every command writes to your local copy first, so it works offline. A store with a remote then
syncs in the background, about 30 seconds after a write.
The statuses depend on the store's schema; the default one has `backlog`, `todo`,
`on_hold`, `in_progress`, `waiting`, `done` and `canceled`.

## 5. A store of your own

```bash
hh store init me --prefix ME
hh add "Home"                    # with no parent, a project: ME-1
hh add "Buy milk" --parent ME-1  # a task under it
```

The root of a workspace holds only projects, so a task goes under a project (see
[Planning](planning.md#kinds)).

A store with no `--remote` stays on this machine. To share it, create an empty
repository and pass it with `--remote <url>` when you run `hh store init`. For a GitHub
URL, hedgehug creates the repository and adds the `hedgehug-store` topic, which is what
`hh store discover` looks for.

`hh ui` opens the same tasks in the browser.

Next: [Sync](sync.md), [Stores](stores.md).
