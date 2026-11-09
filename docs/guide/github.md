# GitHub

A task can point at pull requests and issues. During `hh sync`, hedgehug reads their
state from GitHub (the open/draft/merged state, review and checks of a pull request; the
state of an issue), shows it on the task, and applies the automation rules. The account
of the store needs a token that can read the repository ([Getting started](getting-started.md)).

## Link a task

```console
$ hh link WEB-1 --issue acme/web#12
Linked WEB-1 (01M6QAZMHYRXKQJSTS7KTHA2GC) Write the login page
$ hh show WEB-1
...
Links:
  issue  acme/web#12  not read yet
```

`hh link <id> --pr owner/repo#n` links a pull request. `hh unlink` stops following a
link and keeps it in the file, turned off, so that a pull request whose text still
carries the task's mark is not linked again.

Hedgehug also links on its own, at sync, when it finds:

- a branch named `hh/<ID>-<slug>`,
- `hedgehug:<ULID>` in the title, body or comment of a pull request or issue.

A task can have several links, and one pull request can serve several tasks.

## Start work from a task

Run these inside the repository of the code:

```bash
hh start WEB-1      # makes the branch hh/WEB-1-<slug>, assigns you, moves the task to doing
hh pr WEB-1 --draft # pushes the branch, opens the pull request, links it
hh issue WEB-1      # opens an issue for the task and links it in push mode
```

## Automations

The rules live in `automations.yaml` of the store, and each acts on an event. A store made
with `hh store init --template dev` starts with these:

```console
$ hh automation list
ID            ON                   WHEN                   SET                 SCOPE  STATE
pr-opened     pr.opened            -                      status=in_progress  store  on
pr-review     pr.review_requested  -                      status=review       store  on
pr-merged     pr.merged            all_linked_prs_merged  category=done       store  on
issue-closed  issue.closed         -                      category=done       store  on
```

```bash
hh automation add --id wip --on pr.opened --status in_progress
hh automation disable pr-review                    # off for the store
hh automation disable pr-review --project WEB-6    # off in one project only
hh automation enable pr-review
```

`--on` takes `pr.opened`, `pr.review_requested`, `pr.merged` or `issue.closed`. Use
`--category done` in place of `--status` to set the default status of a category.
`--when all_linked_prs_merged` waits until every followed pull request of the task is merged.
Changes made by a rule are committed under the author `hedgehug-automation`, so the history
shows them. A rule that breaks the workflow of the schema is refused, and the refusal goes to `hh inbox`.

## Sync issues

Each issue link has a mode:

| Mode | Source of truth | Effect |
| --- | --- | --- |
| `link` | none | Follow the issue's state only. This is the default. |
| `mirror` | GitHub | Title, state and assignees of the issue go to the task. |
| `push` | hedgehug | Fields of the task go to the issue. |

```bash
hh link WEB-1 --issue acme/web#12 --sync push
hh gh import acme/web --label hedgehug --dry-run   # how many issues it would take
hh gh import acme/web --label hedgehug             # one mirror-mode task per open issue
```

`hh automation fields` shows and changes which fields each mode carries, and the default mode:

```console
$ hh automation fields --default-sync push
default_sync: push
push: title, body, status, assignees
mirror: title, status, assignees
props: -
```

Labels are not a field to sync: how labels are used differs from one repository and organization to the next, and
a list of labels of the store does not map onto them. No mode sends the labels of a task to its issue, creates an issue with them, or
takes the labels of an issue; `hh automation fields` has no `labels`, and the store format 4 conversion removes
it from an existing rule (see [Stores](stores.md#the-store-format-and-hh-store-convert)). `hh gh import --label` is a
filter, not a sync: it picks the issues to import by their GitHub label.

In `push` mode, a value that someone edited by hand on GitHub is overwritten at the next
push, and the overwrite is reported in `hh inbox`.

### Sync comments

`comments` is a field of the sync that neither mode carries by default; add it to the mode you want:

```bash
hh automation fields --push title,body,status,assignees,comments
hh automation fields --mirror title,status,assignees,comments
```

- **Update every clone first.** Before you add `comments`, update hedgehug in every clone that uses the store.
  An older hedgehug cannot read an `automations.yaml` that names `comments`, so in that clone the rules and the
  issue sync stop. The store format does not change, so nothing else warns you.
- **`push`** creates the comments of the task as comments of the issue, and carries edits and deletions over. It sends
  only your own comments (a comment with no author, and a deleted comment that was never sent, are not
  sent), and each clone sends its own. On GitHub the author of the comment is the person who ran the sync. A
  comment that someone edited by hand on GitHub is overwritten at the next push, and the overwrite is reported
  in `hh inbox` as "comment (...)". The first push after you add `comments` creates 30 comments at most in one
  sync and leaves the rest for the next one. The body is sent as written, so `@login` is a real mention and
  notifies that person; `@host/login` (a person of another host) can notify `@host` on GitHub. Each comment
  ends with an invisible marker `<!-- hedgehug:<ULID> -->`, which lets a later sync find the comment again
  even if the record of its GitHub ID was lost.
- **`mirror`** takes the comments of the issue as comments of the task, with the author `github:<login>`, and
  carries edits and deletions over. Two clones that take the same comment make the same file, so the merge keeps
  one. A comment taken from GitHub cannot be edited or deleted in hedgehug, now or after you unlink the issue or change the
  mode (the error is `comment.mirrored`); change it on GitHub. Only the first mirror link of a task is read.
  Taking comments is not limited and does not notify, and it adds nobody to the watchers.
- `hh undo` refuses to undo the adding of a comment that was already sent to GitHub. Use `hh comment rm`, which
  removes it from GitHub too.
- Only comments on issues are synced, not comments on pull requests.
