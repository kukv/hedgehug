# Security Policy

## Supported versions

Only the latest release is supported. Fixes are not backported to older versions;
update to the newest release from the
[releases page](https://github.com/kukv/hedgehug/releases), mise or `go install`.

## Reporting a vulnerability

Report privately through GitHub: open the **Security** tab of this repository and
choose **Report a vulnerability**. Please do not open a public issue.

Include what `hedgehug --version` prints, your operating system, whether it happened in
the CLI or the GUI, the command line or the steps you took, and the content that
reproduces the problem. Replace private tasks, remote URLs and anything secret with
placeholders.

## What counts as a security issue

hedgehug keeps tasks in a git repository and syncs them with its remotes, so the content
it reads may have been written by someone else. These count:

- **Credentials leaking.** A token or password for a git remote appearing in hedgehug's
  output, in a log, or in a commit hedgehug makes.
- **Content affecting the machine.** A task, or anything pulled from a remote, that makes
  hedgehug run a command, write outside its own files and the task repository, or inject
  escape sequences that the terminal acts on.
- **A released binary that is not what this repository builds.**

Not a security issue:

- A sync that fails, a conflict resolved badly or a task shown wrongly, without the
  effects above. Open an ordinary issue.
- What your own git configuration asks for, such as a hook or a credential helper you
  installed.

## Handling

Reports are acknowledged and triaged by the maintainer. Once a fix is released, the
advisory is published with credit to the reporter unless anonymity is requested.
