# Contributing to DossierX

Thanks for your interest in contributing. This document covers the mechanics of getting a
change merged; for design questions or larger proposals, open a GitHub Discussion first.

## Requirements

- Go **1.26** or newer.
- [`golangci-lint`](https://golangci-lint.run/) for linting (config in `.golangci.yml`).

## Running tests

```sh
go test ./...
```

Race-detector runs (matching CI) are strongly encouraged before opening a PR:

```sh
go test -race ./...
```

### The two suites `go test ./...` does not reach

`go test ./...` covers the root module and nothing else. Two suites live outside it on
purpose, and CI runs both — if you touch what they cover, run them too.

```sh
make viewer-test   # viewer-tests/, a separate module
make hook-test     # scripts/hook-smoke-test.sh
```

**`make viewer-test`** is no longer only the browser suite, and it no longer skips its way to
green. It runs the whole `viewer-tests/` module, which is now three things: the viewer's inline
JavaScript driven through a real headless browser (the comment panel, the comment chip, the edge
labels), the marketing site read as **rendered DOM** from a real `npm` build, and a GoReleaser
dry run that builds the release archives and reads the version back out of the binary. It is a
separate Go module because it needs `chromedp`, and the engine's `go.mod` stays cobra + yaml.v3 —
`go test ./...` therefore cannot descend into it. The root module tests the viewer's *markup*
(`internal/render`); this is the only thing that tests its *behaviour*, the site's, or the release
build's.

**It fails, rather than skips, when it cannot run** — a skipped check is indistinguishable from a
pass over zero assertions, so "we did not look" must not exit 0. On a machine that has all four
prerequisites it is a normal test run; on one that has none it fails immediately and tells you
which is missing. You need:

| Prerequisite | How it is supplied | What it is for |
| --- | --- | --- |
| Chrome/Chromium | `DOSSIERX_TEST_BROWSER=/path/to/chrome` | the viewer suite and the site's rendered DOM. The viewer suite still falls back to the usual install locations and only skips when nothing is found; the site extraction **requires** the variable and fails without it |
| `node` | on `PATH` | building the site the way the publish workflow builds it |
| `npm` | on `PATH`, with network access | same — the build runs `npm ci`, which reaches the registry |
| `goreleaser` | `DOSSIERX_TEST_GORELEASER=/path/to/goreleaser` | the release dry run. `go install github.com/goreleaser/goreleaser/v2@latest` puts one in `$(go env GOPATH)/bin`; nothing here downloads it for you, on purpose |

CI supplies all four as pinned job dependencies. Locally, the shape of an invocation that runs
everything is:

```sh
DOSSIERX_TEST_BROWSER="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
DOSSIERX_TEST_GORELEASER="$(go env GOPATH)/bin/goreleaser" \
make viewer-test
```

**`make hook-test`** is the pre-commit gate's suite. The gate is shell driving a real binary
against a real git repository, so no Go test can cover it; CI runs this on Linux, macOS and
Windows, because the hook body executes under git's own bundled `sh`.

`tests/nested_module_coverage_test.go` fails the build if a nested module is ever added
without a CI job *and* a Makefile target, so this list cannot quietly go stale.

## Linting

```sh
golangci-lint run
```

CI runs this with the pinned version in `.github/workflows/ci.yml`; install the same version
locally to avoid surprises.

## Running the CLI locally

```sh
go run ./cmd/dossierx <command>
```

For example, against one of the fixtures under `testdata/`:

```sh
go run ./cmd/dossierx check --config testdata/fixture-basic/project.config.yaml
```

## Commit and PR conventions

- **Conventional Commits.** Commit subjects must follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, ...) — `.goreleaser.yaml`'s changelog grouping and any future release tooling depend on this.
- **DCO sign-off.** Every commit must be signed off: `git commit -s`. This adds a `Signed-off-by:` trailer certifying you have the right to submit the change under the project's license.
- **One logical change per PR.** Keep PRs scoped to a single change — a feature, a fix, a refactor — even if that means splitting up a larger piece of work. Smaller PRs review faster and bisect cleaner.

## Cutting a release

Maintainers only: [docs/RELEASING.md](docs/RELEASING.md). Tagging is automated;
the checklist covers the parts that are not — the version numbers copied into
prose, and verifying the published artifact rather than the source you edited.

## Package boundaries

`internal/` is organized as a dependency graph with a small number of hard rules, enforced by
`.golangci.yml`'s `depguard` configuration rather than left to convention alone (see "Why
loosely coupled" below for why that matters here). The current packages are:

`buildorder`, `catalog`, `config`, `implink`, `lint`, `loader`, `lock`, `model`, `reaudit`,
`render`.

The rules:

- **`model` and `config` are dependency-free leaves.** Neither may import any other package
  under `internal/`. Everything else in the engine depends on the claim/config shapes these
  two packages define, so they must not depend back on anything.
- **`render` is a top-of-stack sink.** It may import other `internal/` packages, but nothing
  under `internal/` may import `render` in return — not `lint`, not `catalog`, not `lock`,
  not `loader`, not `implink`. Rendering is the last stage of the pipeline; nothing upstream
  of it should need to know how output gets drawn.
- **`lint` must never import `lock`.** The dependency runs the other way: `lock` imports
  `lint`, because locking a claim is gated on that claim passing a clean lint run. A `lint`
  package that imported `lock` back would create a cycle and would also be backwards from
  what the lock operation actually needs — it needs lint's answer, not the other way around.

If you're adding a new package, place it according to which of these three groups it
logically extends (or ask in the PR description if it's genuinely new territory), and run
`golangci-lint run` before pushing — the depguard rules will fail the build on a violation
rather than let it land silently.

### Why loosely coupled

DossierX has many independent downstream consumers — separate projects, on separate release
trains, upgrading on their own schedule. None of them share a deploy or a version bump with
any other. That means package layering inside this engine can't rely on "everyone remembers
the convention" the way a single-consumer internal tool might get away with; it has to be
enforced by tooling (lint, CI) so that a dependency-direction mistake fails fast in this repo
rather than surfacing as a confusing break in someone else's project months later.

### Patterns to follow

When you need to add an optional, render-time-only extension to the viewer — a computed view
that augments what's already there without changing what a claim's authored YAML means —
follow the model in `internal/render/depended_by_view.go`'s `attachEdgesOverride`: inject the
computed view at the render boundary via a template-func override, and compute it fresh every
render from the catalog. Do not store a derived or reverse-index fact (e.g. "which claims
depend on this one") as a second field on the claim itself — a hand-maintained copy of a
derived fact is a copy that can drift out of sync with its source the moment the source
changes without the copy being updated to match. Recomputing it on every render is the only
way to show it without ever risking that drift.
