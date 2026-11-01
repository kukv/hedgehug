# Contributing

Thanks for taking the time to improve hedgehug.

## Getting started

hedgehug is a single Go binary for Linux, macOS and Windows. You need:

- the Go version in `go.mod`
- [golangci-lint](https://golangci-lint.run/) v2 (the version CI uses is pinned in
  `.github/workflows/ci.yaml`)
- [gotestsum](https://github.com/gotestyourself/gotestsum) for `make test`
- [GoReleaser](https://goreleaser.com/) v2 for `make release-check`

```bash
make check          # everything CI checks: tidy, lint, fmt, test
make test           # tests with the race detector
make cover          # tests with the race detector and coverage
make fmt            # format with gofumpt + goimports
make release-check  # goreleaser config and cross-compiling
make build          # bin/hedgehug
make clean-cache    # empty the Go build cache when it grows too large

go run ./cmd/hedgehug --version
```

Do not commit while `make check` fails. CI does not check the release configuration, so
run `make release-check` too when you change `.goreleaser.yaml`.

## Before you change code

Changes to `internal/` and `cmd/` follow a design and an implementation plan:

- Designs: `docs/superpowers/specs/`
- Plans: `docs/superpowers/plans/`

Both are written with the [superpowers](https://github.com/obra/superpowers) plugin for
Claude Code: its `brainstorming` skill produces the design and its `writing-plans` skill
the plan. You do not need the plugin to contribute — the documents are plain Markdown —
but new ones should follow the same format and locations.

Read the ones that cover your change first. If none does, open an issue to agree on
the design before writing code — a pull request that restructures packages without one
is closed.

The direction of dependencies between packages is enforced by `depguard` in
`.golangci.yaml`. When you add a package, add its rules there in the same pull request.
If a rule gets in the way, propose changing the rule rather than working around it.

## Pull requests

- CI runs on every pull request: `ci.yaml` (tidy, lint, format, and the tests on Linux,
  macOS and Windows) and `security.yaml` (hidden-content scan, gitleaks, dependency
  scanning, zizmor, actionlint). All of them must pass.
- Pin any GitHub Action you add to a full commit SHA with a `# vX.Y.Z` comment. Renovate
  follows the pins.
- Commit messages use a `feat:` / `fix:` / `docs:` / `refactor:` / `test:` / `chore:` /
  `ci:` prefix.
- Link the issue the pull request resolves (`Closes #123`).
- Label the pull request so it lands in the right section of the release notes and the
  right version bump — see [Releases](#releases).
- Review by the maintainer (`.github/CODEOWNERS`) is required before merge.

## Releases

The labels on the merged pull requests decide the version and the release notes:

| Label                | Bump                                   | Release notes section |
| -------------------- | -------------------------------------- | --------------------- |
| `Impact: Breaking`   | major (minor while the version is 0.x) | Breaking Changes      |
| `Kind: Feature`      | minor                                  | New Features          |
| `Kind: Enhancement`  | minor                                  | Enhancement Updates   |
| `Kind: Bug Fix`      | patch                                  | Bug Fix               |
| `Kind: Dependencies` | patch                                  | dependency updates    |
| anything else        | patch                                  | Other Changes         |

The highest bump among the pull requests since the last release wins.
`Meta: Release note ignored` keeps a pull request out of the notes.

To release, tag the head of `main` and push the tag:

```bash
git switch main && git pull
git tag v0.1.0 && git push origin v0.1.0
```

The tag push starts `.github/workflows/release.yaml`, which runs GoReleaser: it builds
the binaries for Linux, macOS and Windows (amd64 and arm64), stamps the version that
`hedgehug --version` prints, and creates the GitHub Release. Its notes are generated
from the pull request labels, following `.github/release.yaml`.

A version with a suffix (`-beta.1`, `-rc.1`) is published as a pre-release, so it does
not become the latest release.

Tags are never moved or reused once pushed: `go install` and mise resolve versions from
them. Going to v2 or later also means changing the module path to
`github.com/kukv/hedgehug/v2`, as Go modules require.

## Use of AI

AI assistance is fine. Submitting what an AI produced without understanding it is not.

Generating a submission takes seconds; verifying one takes a person's time. Sending
unverified output moves that cost onto the maintainer and takes time away from the review
this project actually needs.

Before you open an issue or a pull request, you are expected to have read the output,
verified it against this repository, and be able to explain and defend it. You are the
author of what you submit, whatever tool helped you write it.

Issues and pull requests that appear to be unreviewed AI output — invented options,
commands or files that do not exist, a diff that does not follow from the description,
boilerplate that does not engage with this project — are **closed without notice and
without individual explanation**. That judgment is the maintainer's, and there is no
appeal process; you are welcome to open a new issue or pull request that shows your own
reasoning.

Closing one does not mean the underlying point was worthless. If a closed issue or pull
request contains something useful, the maintainer may take it up — as an issue raised by
the maintainer, or by merging or rewriting the change — without notice and without credit
to the original submitter. Anything you submit is licensed under the
[MIT License](LICENSE), and opening an issue or pull request here means you accept this
handling.

## Reporting problems

- A security issue: see [SECURITY.md](SECURITY.md).
- A bug or a feature request: open an issue from the templates.

By contributing you agree that your contributions are licensed under the
[MIT License](LICENSE). Everyone taking part is expected to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).
