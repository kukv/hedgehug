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
ID     ULID                        STATUS  KIND  ASSIGNEES         LABELS  TITLE
WEB-1  01M6QAZMHYRXKQJSTS7KTHA2GC  todo    task  github.com/alice  ui      Write the login page
WEB-2  01M6QAZMJND1BXZNFVP2PXHFHP  done    bug                             Fix the logout bug
```

`hh show WEB-1` shows one task with its history. `hh ls status:todo` filters, and
`hh ls --tree` shows the tasks under their parents.

```bash
hh add "Review the login page"       # goes to the default store
hh status WEB-1 in_progress
hh set WEB-1 assignees=github.com/me labels+=ui
hh done WEB-1
hh sync                              # send your changes, get the others'
```

Every command writes to your local copy only. Nothing reaches your team until `hh sync`.
The statuses depend on the store's schema; the default one has `backlog`, `todo`,
`on_hold`, `in_progress`, `waiting`, `done` and `canceled`.

## 5. A store of your own

```bash
hh store init me --prefix ME
hh add "Buy milk"
```

A store with no `--remote` stays on this machine. To share it, create an empty
repository and pass it with `--remote <url>` when you run `hh store init`. For a GitHub
URL, hedgehug creates the repository and adds the `hedgehug-store` topic, which is what
`hh store discover` looks for.

`hh ui` opens the same tasks in the browser.

Next: [Sync](sync.md), [Stores](stores.md).
