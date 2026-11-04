# hedgehug

[日本語](README.ja.md)

A local-first task manager that uses git as its storage and sync layer, with a CLI and
a GUI in a single binary.

It runs on Linux and macOS. On Windows, use it in WSL.

> **Status: setting up.** The repository has its CI, lint and release configuration, but
> no features yet. Nothing here is ready to use.

## Development

The GUI is in `web/`: a Svelte app built with Vite, embedded in the binary. You need
Node and pnpm; `mise install` installs the versions pinned in `mise.toml`.

```bash
mise install
make build          # builds web/dist, then bin/hedgehug with the GUI inside
bin/hedgehug ui     # serves the GUI on 127.0.0.1 and opens the browser
```

To work on the GUI with reload, start the server and the Vite dev server side by side.
The dev server proxies `/api` to the server and opens the browser with its token:

```bash
bin/hedgehug ui --no-open       # prints: Hedgehug UI: http://127.0.0.1:<port>/#token=<token>
cd web
HEDGEHUG_UI='http://127.0.0.1:<port>/#token=<token>' pnpm dev
```

```bash
make web-check      # Biome, svelte-check, the API types and Vitest (also part of make check)
make e2e            # Playwright against bin/hedgehug-e2e (built with -tags e2e: a file keyring, not the OS keychain) and bin/fakegh (installs Chromium if it is missing)
make e2e-bin        # only those two binaries
```

A binary built without `make web` (`go build`, `go install`) has no GUI: `hh ui` serves a
page that says to build it.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Security issues: [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
