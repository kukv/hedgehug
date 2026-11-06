# Sync

`hh sync` fetches, merges, repairs and pushes. You rarely need it: a store with a remote
syncs in the background. After a write, hedgehug pushes about 30 seconds later
(`sync.push_interval`), and any command starts a sync when the last one is more than
2 minutes old (`sync.interval`). Both settings are in `config.yaml`. Run `hh sync` when you
want it done now or want to see the result. `HEDGEHUG_AUTO_SYNC=0` turns the background
sync off.

```console
$ hh sync --status
team
  last attempt:  2026-11-06T11:28:54+09:00
  last success:  2026-11-06T11:28:54+09:00
  pending since: 2026-11-06T11:29:12+09:00
  2 commits not pushed
$ hh sync
Synced team: 1 merge, pushed to origin
```

`hh sync` syncs every store that has a remote; `--store <name>` picks one. `--status`
shows how the last syncs went and syncs nothing. While the GUI is open, it syncs on its own.

## How merges work

Two people can edit the same store offline. At the next sync, hedgehug merges the files
field by field, so editing different fields of one task never conflicts, and each new
comment is a new file. Two cases need a rule:

- **The same field changed on both sides.** The later change wins. The earlier value
  is not lost; it is recorded in the merge commit.
- **The same ID given twice.** If you and a colleague both add `WEB-3` offline, one
  keeps the ID and the other is renumbered. The ULID, the task's permanent name, does
  not change.

The sync prints both, and `hh inbox` keeps them until you read them:

```console
$ hh sync
Synced team: 1 merge, pushed to origin
  WEB-3 (01M6QB0KC5488AWY7AS44Y67JY) → WEB-4
  overwritten: WEB-1 status (github.com/alice's value was replaced)
```

## When your value was overwritten

The person who lost sees this in their next `hh inbox`:

```console
$ hh inbox
01M6QB0TGFH3B29VE6C5CN04ME  overwritten  WEB-1 (...) Write the login page  status: "waiting" was replaced
```

To put your value back, pass the ID in the first column to `hh inbox revert`:

```console
$ hh inbox revert 01M6QB0TGFH3B29VE6C5CN04ME
Reverted WEB-1 (01M6QAZMHYRXKQJSTS7KTHA2GC) Write the login page
```

`hh inbox read <ULID>...` marks items as read, and `hh inbox --all` shows the read ones too.

## Undo your last operation

```console
$ hh undo
Undid fd95324 status WEB-2 Fix the logout bug
```

`hh undo` reverts your last operation and keeps what others changed after it. It writes
a new commit; it does not rewrite history.

## Go back to an earlier state of one task

`hh restore <id> --at <time or commit>` puts one task back as it was. The time is an
RFC 3339 timestamp or a date (which means the end of that day); a commit comes from
`hh log <id>`.

```console
$ hh log WEB-1
a5bba5a  2026-11-06 11:29  alice  hedgehug: set WEB-1 (...) Write the login page
31f15d5  2026-11-06 11:28  alice  hedgehug: set WEB-1 (...) Write the login page
09431e7  2026-11-06 11:28  alice  hedgehug: add WEB-1 (...) Write the login page
$ hh restore WEB-1 --at 31f15d5
Restored WEB-1 (01M6QAZMHYRXKQJSTS7KTHA2GC) Write the login page
```

The ID and workspace of the task stay as they are now. Without `--at`, `hh restore <id>`
brings back a task deleted with `hh rm`, together with what the same `rm` deleted under it.

## If the history of origin changes

If someone rewrites the history of the remote, `hh sync` stops and does not push. See
[Maintenance](maintenance.md#removing-content-from-history).
