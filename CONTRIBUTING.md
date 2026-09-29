# Contributing to Consigna

Thanks for helping! Bug reports, ideas and pull requests are all welcome.

## Getting set up

You need Go 1.24+, Node.js 20.19+ and `make`. For linting Go, install [golangci-lint](https://golangci-lint.run/) v2.

```sh
make dev     # Go server on :7431 + Vite dev server with hot reload on http://localhost:5173
make check   # lint, unit tests and end-to-end tests: what CI runs
```

The end-to-end tests need Chromium: `npx --prefix web playwright install chromium`. They join a second browser through your machine's LAN address, the way a phone would, so they are skipped on a machine without one.

To try the UI on a real phone during development, use `make build && ./bin/consigna` and open the printed link. The Vite dev server only listens on localhost on purpose (see `web/vite.config.ts`).

## Where things are

See [docs/architecture.md](docs/architecture.md) for the full picture. In short: the Go server is in `internal/`, the entry point in `cmd/consigna`, the web UI in `web/src`, and the API contract in `internal/server/views.go` and `web/src/lib/types.ts` (keep both in sync).

The UI uses atomic design: add small, reusable pieces as atoms and molecules, compose them in organisms, and keep pages thin. Styles are CSS Modules on the tokens in `web/src/styles/tokens.css`; use tokens, not new hard-coded colours.

## Branches and releases

```text
feature/…, fix/…  ──PR──▶  dev  ──PR──▶  main  ──tag vX.Y.Z──▶  release
```

- **`main`** is production: what is released. It only receives pull requests from `dev`.
- **`dev`** is where finished work is integrated and tested together before it goes to `main`.
- **Work branches** start from `dev` and are named after what they do: `feature/https-on-lan`, `fix/upload-resume-ios`, `docs/…`, `chore/…`. Open the pull request against `dev`.

The usual loop:

```sh
git switch dev && git pull
git switch -c feature/my-change
# …commit…
git push -u origin feature/my-change   # open a PR into dev; CI must pass
```

When `dev` has been tried and is good, open a pull request from `dev` into `main`. A release is a tag on `main` (`git tag v0.2.0 && git push --tags`), which builds the binaries.

## Conventions

- **Tests with every change.** Go: table-driven tests next to the code, run with `-race`. Web: Vitest and Testing Library, querying by role and label like a user would. New user flows get an end-to-end test in `web/e2e`.
- **Accessibility is not optional.** Real buttons and links, labels on every control, 44px touch targets, text contrast of at least 4.5:1, and nothing that only colour or motion explains.
- **Security.** Never use client-supplied names as paths, never serve uploads inline, and keep new state-changing endpoints behind the device and origin checks. When unsure, ask in the pull request.
- **Commits** follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `test:`, `build:`, `chore:`).
- **Formatting** is automatic: `make fmt`.

## Pull requests

Keep them focused, describe what and why, and include screenshots (phone at 390px and desktop at 1280px) for UI changes. CI must be green.

By contributing you agree that your work is released under the [MIT License](LICENSE), and to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
