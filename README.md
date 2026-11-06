# hedgehug

[日本語](README.ja.md)

A local-first task manager that uses git as its storage and sync layer, with a CLI and
a GUI in a single binary. The command is `hedgehug`; `hh` is the short form.

- Every task is a Markdown file in a git repository (a store). The history of the
  store is the history of your tasks.
- Everything works offline. A store with a remote syncs in the background, and `hh sync` syncs now.
- Concurrent edits merge field by field. When two people change the same field, the
  later change wins and the loser sees it in `hh inbox`.
- Stores map to who you share with: one for yourself, one per team. `hh ls` shows them all.
- Tasks link to GitHub pull requests and issues.

It runs on Linux and macOS. On Windows, use it in WSL.

## Install

The archive and `mise` give you both `hedgehug` and `hh`. `hh` shares its name with an
old version of HSTR (a shell history tool); if you have both, use `hedgehug`.

1. With [mise](https://mise.jdx.dev/):

   ```bash
   mise use -g github:kukv/hedgehug@latest
   ```

2. By hand: download the archive for your OS and CPU and `checksums.txt` from
   [Releases](https://github.com/kukv/hedgehug/releases), then verify and unpack it:

   ```bash
   sha256sum -c checksums.txt --ignore-missing
   tar xzf <archive>.tar.gz
   install hedgehug hh ~/.local/bin
   ```

3. From source, with Go, Node and pnpm (`mise install` installs the pinned versions of
   Node and pnpm):

   ```bash
   git clone https://github.com/kukv/hedgehug && cd hedgehug
   make build          # bin/hedgehug, with the GUI inside
   ```

`go install` is not supported: a binary built without `make build` has no GUI.

Check it: `hh --version`.

## Quick start

```bash
hh auth add me --login <your-github-login> --token gh   # uses the token of the gh CLI
hh store init me --prefix ME                            # a store only you use
hh add "Write the README"
hh ls
hh done ME-1
hh ui                                                   # the same tasks in the browser
```

To see your team's tasks, run `hh store discover`. The whole walk-through, from install to
the team's tasks, is in [Getting started](docs/guide/getting-started.md).

## Guide

| Guide | What it covers |
| --- | --- |
| [Getting started](docs/guide/getting-started.md) | Install, sign in, your first task, your team's tasks |
| [Sync](docs/guide/sync.md) | Sync, merges, overwritten values, `hh undo`, `hh restore --at` |
| [Stores](docs/guide/stores.md) | Several stores, accounts, the project to add under, moving tasks between stores |
| [GitHub](docs/guide/github.md) | Links, automations, issue sync |
| [Planning](docs/guide/planning.md) | Sprints, releases, reports, templates |
| [Maintenance](docs/guide/maintenance.md) | `hh doctor`, bare repositories, removing content from history, shell completion |

`hh <command> --help` lists every option of a command. The guides are also in
[Japanese](docs/guide/ja/getting-started.md).

## Contributing

Building from source, the GUI dev server and the checks are in
[CONTRIBUTING.md](CONTRIBUTING.md). Security issues: [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
