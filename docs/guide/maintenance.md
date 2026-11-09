# Maintenance

Two things are your job, because hedgehug does not do them for you: running `git gc` on a
bare repository you use as a remote, and rewriting history when content must be removed.
This guide also covers `hh doctor` and shell completion.

## Check a store: `hh doctor`

`hh doctor` checks the files, the working tree and the git state of every store and
prints what it finds.

```console
$ hh doctor
WEB-2 (01M6QAZMJND1BXZNFVP2PXHFHP): moved to a store that is not mounted (ULID 01M6QB1AX3DCHF1FBZNKZ95D4C) (info)
1 problem
```

`hh doctor --fix` repairs, in one commit, what a sync would repair: parent cycles,
children of deleted tasks, tasks in a different workspace than their parent, and IDs that
clash. A child whose parent is gone moves to the nearest ancestor that can hold it, or else
under a project called 「未分類」 (made if it is not there; a step becomes a task). It also rebuilds a store after the history of origin was rewritten (below).
Whatever it cannot repair, it tells you.

An older hedgehug that pushes to a store in the new format leaves values under `props:` that are
fields of the task now: a relation (`duplicates`, `causes`, `relates` or `clones`; fields since format 3)
or a built-in property (`priority`, `start`, `due`, `estimate` or `points`; fields since format 4).
`hh doctor` reports each as `task.legacy_field`, and `hh doctor --fix` moves it to the field
of the task, in a commit of its own:

```console
$ hh doctor
ME-3 (01M6ZCTTTJ0TC1RY48125GV8SB) Data loss: props.duplicates: is a relation of the task in this format, not a property; an older hedgehug wrote it
  hint: hedgehug doctor --fix moves it to the field of the task
1 problem
$ hh doctor --fix
repaired: ME-3: legacy_field props.duplicates moved to the fields of the task
No problems
```

A built-in property is moved the same way. A datetime written under `props:` (`due: 2026-11-10T09:30:00+09:00`) becomes a
date, and the time is dropped; `--fix` lists each dropped time with the task and the value it was:

```console
$ hh doctor --fix
repaired: ME-3: legacy_field props.priority, props.due moved to the fields of the task; the time of props.due was left out: 2026-11-10T09:30:00+09:00
No problems
```

A task that already has a value in the field has nothing moved, because it is not known which
of the two is newer. The finding stays: open the task with `hh edit <id>`, keep the value you
want in the field, and delete the one under `props:`. A value under `props:` that is not the
ULID of a task (a relation), or that cannot be read as the type of the field (a built-in property, such as an
`estimate` that is not a duration), also stays, and `hh edit` is how you fix it.

A store in an older format is reported once, with `hh store convert` as the way out, and its
tasks are not checked against the schema until it is converted
(see [Stores](stores.md#the-store-format-and-hh-store-convert)):

```console
$ hh doctor
.hedgehug/store.yaml: the store is in format 3, and this version uses format 4
  hint: hh store convert
1 problem
```

## A bare repository as your remote

If your remote is a bare repository on your own machine or server (not GitHub), run `git gc`
in it from time to time. hedgehug packs the objects of your local copy during sync, but it
never touches the remote's, which keeps growing as loose objects.

```bash
git -C /srv/remotes/team.git gc
```

GitHub does this on its own.

## Removing content from history

hedgehug has no command that removes content from the history, because a store is a git
repository and everyone has the full history. Deleting a task (`hh rm`, `hh purge`) or
moving it to another store leaves the old content in the history. If something must go (a
leaked secret, for example), rewrite the history by hand, then bring everyone's copy in line.
Revoke a leaked secret as well; a rewrite does not take it back from those who already cloned.

1. Rewrite in a fresh clone of the remote with [`git filter-repo`](https://github.com/newren/git-filter-repo):

   ```bash
   git clone <remote-url> rewrite && cd rewrite
   git filter-repo --path tasks/<a>/<b>/<ULID>.md --invert-paths
   ```

   Use `--replace-text` to blank a string instead of dropping a file. A store with
   mirrors has to be rewritten the same way, mirror by mirror.

2. Force-push it. `git filter-repo` removes the `origin` remote, so add it back:

   ```bash
   git remote add origin <remote-url>
   git push --force origin main
   ```

3. Everyone else, and your own store directory, picks it up at the next sync. The sync
   notices that the history of origin no longer contains the history it follows, and stops:

   ```console
   $ hh sync
   hedgehug: sync team: the history of origin differs from this store's; check origin,
   then run hedgehug doctor --fix to rebuild the store on it
   ```

   Look at origin, then run:

   ```console
   $ hh doctor --fix
   == team
   Rebuilt team on the history of origin; put back 1 unpushed commit
   No problems
   ```

   `hh doctor --fix` clones origin again, puts the commits you had not pushed back on top,
   and drops the old history from your disk. If some of your changes cannot be put back, it keeps the
   old copy and prints its path.

Sync does not fix this itself, so that a force-push by mistake does not pull everyone along.

## Shell completion

`hh completion bash|zsh|fish` prints a script that completes commands, flags and values.
It is written for the name you ran it by: `hh completion zsh` completes `hh`, and
`hedgehug completion zsh` completes `hedgehug`. Install it for each name you use.

```bash
hh completion bash > ~/.local/share/bash-completion/completions/hh
hh completion zsh > "${fpath[1]}/_hh"
hh completion fish > ~/.config/fish/completions/hh.fish
```

Start a new shell afterwards. The bash script needs the `bash-completion` package; without
it, completion does not work. (On macOS, the bash that ships with the system is too old;
install `bash` and `bash-completion@2` from Homebrew.)

Values come from your stores, read locally; nothing is written or synced when you press
Tab, and a failure gives no candidates:

- task IDs (with the title) for commands that take a task, and for `--parent`, `--after`,
  `--before` and the relations of `hh dep` (`--blocked-by`, `--causes`, `--relates`, `--duplicates`, `--clones`)
- statuses for `hh status <id> <TAB>`
- `key=` names for `hh set`, and assignees for `assignees=` and for `assignee:` in `hh ls`
- stores (`--store`), workspaces, sprints, releases, saved views and templates
