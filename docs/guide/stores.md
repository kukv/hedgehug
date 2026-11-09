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
$ hh add "Buy milk" --parent ME-1
Added ME-2 (...) Buy milk
→ added to personal / Home
$ hh add "Plan the demo" --store team --parent WEB-6
Added WEB-8 (...) Plan the demo
→ added to team / Website
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

## The store format and `hh store convert`

A store records its format in `.hedgehug/store.yaml`. This version of hedgehug uses
format 4. In format 2 the kinds of task are fixed (see [Planning](planning.md#kinds)), in
format 3 the relations are too (see [Planning](planning.md#relations)), and in format 4 the
built-in properties are: priority, start, due, estimate and points are fields of the task, and
the values of priority and labels are lists in `schema.yaml`.

- A store in an **older** format does not open. Every command except `hh store convert`
  and `hh doctor` stops with `hh store convert で新しい形式にしてください`, and the browser shows the same
  sentence on the store's card in the sidebar and in the settings. `hh doctor` reports
  the format and names the command.
- A store in a **newer** format does not open either: update hedgehug.

`hh store convert` brings the store to the newest format, one step after the other, and writes
all of it as one commit, and a store with a remote has the result pushed. If the remote is in the
same old format and the two differ (you have unpushed commits, or the remote has commits you
lack), the command first syncs the store in the old format (fetch, merge, push), then converts
and pushes, all in one run. When that sync fails (a push is refused, a merge stops) it stops
with `store.convert_unsynced` (HTTP 409), and the message tells the cause. `--dry-run` neither syncs nor writes; it says that the sync would come first.

```console
$ hh store convert --dry-run
Format 1 -> 2
  ワークスペース me の直下のタスクを「未分類」プロジェクトの下へ移した: 1 件（「未分類」を作った: ME-3）
  schema.yaml の kinds を消した
  store.yaml の format を 1 から 2 にした
Nothing was written (--dry-run).
$ hh store convert
```

A store in format 2 converts like this. The lines are in Japanese, as the command prints them:

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

A store in format 3 converts like this (`ME-3` has a `due` with a time and an `estimate` that is
not a duration; `wip` is not in `labels:` of `schema.yaml`):

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

The first command stops with `convert.bad_value` and exit code 1 (a dry run stops the same way, so you see
what is wrong before you write anything).

A store that synced labels with GitHub also gets these lines. Here `ME-2` has an issue link whose only synced field was
`labels`, so the link now syncs nothing (`sync: link`):

```console
$ hh store convert --dry-run
Format 3 -> 4
  GitHub のラベルの同期の跡（gh.issues[].synced と links の fields の labels）を外した: 1 件
  ラベルだけを同期していたリンクの sync を link（同期しない）にした: ME-2 Login: github.com/acme/web#12
  automations.yaml の issue_fields の push・mirror から labels を外した（GitHub のラベルは同期しない）
  store.yaml の format を 3 から 4 にした
Nothing was written (--dry-run).
```

`--dry-run` lists what each step would change and writes nothing; `--json` prints the
same as JSON. Running it again changes nothing.

The conversion is a commit of its own (`op convert`), and `hh undo` does not take it back: right after
a conversion, `hh undo` undoes the operation before it.

- A kind of your own becomes a project when it groups work at the root of a workspace, an
  epic when it groups work below, and a task when it is a work item. `--kind <old>=<new>`
  (repeatable) moves the tasks of one kind to one of the fixed kinds instead, such as
  `--kind sprint=epic`.
- A task that sat at the root of a workspace and is not a project moves under a project
  named 「未分類」, made if it is not there.
- A story, a task or a bug under a story, a task or a bug becomes a step.
- Relations (format 2 to 3): the values of `duplicates`, `causes`, `relates` and `clones` move to the
  field of the same name of the task. A `task_ref` property named `duplicates`, `causes`, `relates` or
  `clones` moves to the field of that name; a `task_ref` property with any other name has its values moved to
  `relates` (the report has one line per property). Both are removed from `schema.yaml`; the type
  no longer exists.
- What changes in meaning, and what is lost:
  - `relates` has a direction now. A pair stored on both tasks (A has B, B has A) stays as two
    `relates`; the report counts them as `相互の組`. Before, one `relates` was found from both
    tasks; now `relates:A` finds only the tasks that point at A, and `related_by:A` the tasks A points at.
    A view whose query has `relates:` or `relates.inverse:` is not rewritten (the report lists it); add `related_by:` if you want both ways.
    The same goes for a `task_ref` of your own that had `symmetric: true`: its key and its `.inverse:` matched both ways, so they are listed, not rewritten.
  - `<name>.inverse:` in a view's query becomes the name of the other way (`causes.inverse:` becomes
    `caused_by:`, `blocked_by.inverse:` becomes `blocks:`); the `.inverse:` of a `task_ref` of your own (one that was not symmetric) becomes `related_by:`, and its key becomes `relates:`.
  - A relation is not a column, a sort or a group of a view, so the report lists what was taken out of
    `table.columns`, `sort` and `group`.
  - A value that is not the ULID of a task stops the conversion with `convert.bad_value`, listing the
    tasks and values. Fix them in the files, or give `--drop-bad-values` to drop them; each one is then
    listed in the report. A value that points at a task that does not exist or is deleted, or at the task itself, moves as it is.
  - A property of your own that is not a `task_ref` and is named `duplicates`, `causes`, `relates` or
    `clones` stops the conversion with `convert.prop_name_taken`, which lists the names and types. See below.
- Built-in properties and labels (format 3 to 4):
  - The values of `priority`, `start`, `due`, `estimate` and `points` under `props:` move to the
    field of the task of the same name (the templates too), and the definitions of those names
    leave `schema.yaml`. `priorities:` is the list the store already has, or else the values of
    the `priority` definition, or else `low`, `mid`, `high`, `urgent`; a value is matched without regard to
    case and stored as the list spells it.
  - A label that is not in the list is **removed** from the tasks and templates. The list is the
    top-level `labels:` of `schema.yaml` as it is when you convert; a store with none loses all its
    labels. The dry run lists each label it removes with the number of tasks. See
    [Keep your labels](#keep-your-labels-through-the-conversion) to keep them.
  - A list in which two values differ only by case (`[High, high]`) is folded to the one written first, and an empty
    `priorities: []` falls through to the next source; each is reported, and neither stops the conversion.
  - A value that can be read as the type of the field is fixed: `"3"` becomes `3`, a datetime
    becomes a date (the offset it was written with decides the day), a one-item array becomes
    its item, an empty string is removed. A datetime whose time is not 00:00 loses the time; the
    report lists each such task and its value.
  - A value that cannot be read (an `estimate` of `3`, a `due` of `next week`), or a field that is both at
    the top level and under `props:`, stops the conversion with `convert.bad_value`, which lists the tasks
    and values, as above. Fix them in the files, or give `--drop-bad-values`: an unreadable value is dropped,
    and for a field that is twice, the value of `props:` is kept and the one at the top level is dropped; each is
    listed in the report.
  - GitHub no longer syncs labels. `labels` leaves the fields of `automations.yaml`, `labels` leaves the
    recorded state of each linked issue, and a link that synced only labels becomes `sync: link` (no sync);
    the report lists those links. `hh gh import --label` stays: it picks the issues by their GitHub label.
- `--store <name>` picks the store.

When another clone has already converted the remote, `hh store convert` on a clone with
no unpushed commits fast-forwards it to the remote, and does not convert anything itself. A clone
that still has unpushed commits has its own changes converted first; see
[Sync](sync.md#when-the-remote-is-in-a-newer-format). When the remote is at an intermediate
format, the clone is converted to that format, synced, and then given the remaining steps.

### A property of your own named like a relation

A store in format 2 cannot be edited with `hh` (every command except `hh store convert` and `hh doctor`
stops), so fix it by hand. In the store's folder (`hh store list` shows where it is):

1. In `schema.yaml`, rename the property under `props:` (`causes` to `crash_cause`, say).
2. In the files under `tasks/`, rename the same key under `props:` of each task that has it.
3. Commit (`git add -A && git commit -m "rename properties"`), then run `hh store convert` again.

A property of your own named like the other way (`blocks`, `caused_by`, `related_by`, `duplicated_by` or
`cloned_by`) does not stop the conversion. It still works, but `hh doctor` warns (`prop_name.query_key`), because the key of a
query would mean the relation: rename it the same way, or with `hh schema migrate prop` once the store is converted.

### Keep your labels through the conversion

A store in format 3 cannot be edited with `hh`, so the list of labels you want to keep has to be
written by hand before the conversion; with no `labels:` in `schema.yaml`, every label is removed.
In the store's folder (`hh store list` shows where it is):

1. Find the labels the tasks hold. `hh ls` stops on a store in format 3, so ask the conversion: with no `labels:` in
   `schema.yaml`, `hh store convert --dry-run` lists every label, with the number of tasks that hold it (it writes nothing).
   Looking in the files works too (`grep -rh "^labels" tasks/`).

```console
$ hh store convert --dry-run
Format 3 -> 4
  ラベル bug を外した（schema.yaml の labels: にない）: タスク 2 件・テンプレート 0 件
  ラベル ui を外した（schema.yaml の labels: にない）: タスク 1 件・テンプレート 0 件
  ラベル wip を外した（schema.yaml の labels: にない）: タスク 1 件・テンプレート 0 件
  store.yaml の format を 3 から 4 にした
Nothing was written (--dry-run).
```

2. In `schema.yaml`, add the top-level key `labels:` with the ones to keep (`labels: [bug, ui]`). Case does not matter when the conversion matches them, and the list's spelling is what is stored.
3. Commit (`git add -A && git commit -m "labels"`), then run `hh store convert --dry-run` again: it lists only the labels it would still remove (`wip` below). Run `hh store convert` when that is what you want.

```console
$ hh store convert --dry-run
Format 3 -> 4
  ラベル wip を外した（schema.yaml の labels: にない）: タスク 1 件・テンプレート 0 件
  store.yaml の format を 3 から 4 にした
Nothing was written (--dry-run).
```

After the conversion, `hh schema labels add <name>…` adds a label to the list and `hh schema labels rm <name>…`
removes one that no task holds; `hh schema migrate label <old> <new>` renames or merges one (see
[Getting started](getting-started.md#4-read-and-change-tasks)).

### When the remote has changes in an older format

Another clone can push a commit in the old format after this clone has converted but before it
has pushed. This clone is then in the newest format and the remote is in an older one, with
commits this clone lacks. `hh sync` stops with `sync.format_diverged` (HTTP 409), and its
message says that the remote has changes in an older format and that `hh store convert` converts them.
Run `hh store convert`: it applies the conversion steps to the files of the remote's latest commit,
commits the result on top of that commit, pushes it (the remote moves forward, nothing is
rewritten), and then syncs as usual, so that your history and the converted changes are merged
and pushed. Other clones then fast-forward to it with `hh store convert`, or sync as usual if they
are converted. `--dry-run` lists what would be converted and writes nothing. If yet another clone
pushes meanwhile, the command stops with `store.convert_unsynced`; run it again. If the sync that merges fails after the push (for example a file it would write has uncommitted changes), the converted changes of the remote are already pushed and `hh store convert` has nothing left to do: fix the cause and run `hh sync`.

### Before you convert

Update hedgehug on **every clone** that uses the store before you run `hh store convert`.
A hedgehug from before the format check does not look at the format: a clone of it that has
unpushed commits syncs with the converted remote and pushes tasks of a kind of its own, or
tasks at the root of a workspace, into a store marked format 2. `hh store convert` then has
nothing to convert (the store is already in format 2), and `hh doctor --fix` does not repair
them. If it happened, find them with `hh doctor`: the findings with code `task.unknown_kind`
(`kind: unknown kind "sprint"`) and `task.placement` (`parent: a task cannot be placed under root`)
name the task. Open each with `hh edit <id>` and change `kind:` to one of the fixed kinds, or
`parent:` to a task that can hold it (a project at the root, for example), then run `hh doctor` again.

The same goes for the relations and the built-in properties. A clone from before format 3 keeps writing
`duplicates`, `causes`, `relates` and `clones` under `props:`, and one from before format 4 keeps writing
`priority`, `start`, `due`, `estimate` and `points` there; those are fields of the task in the new
format. `hh doctor` reports them as `task.legacy_field`, and `hh doctor --fix` moves them to the fields of the task
(see [Maintenance](maintenance.md#check-a-store-hh-doctor)).
A `task_ref` of your own is not one of the four names: `hh doctor` does not report it and `--fix` does
not move it. Open the task with `hh edit <id>` and move the values to `relates:`.

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
