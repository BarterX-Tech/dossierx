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
