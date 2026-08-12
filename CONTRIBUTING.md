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
| `goreleaser` | `DOSSIERX_TEST_GORELEASER=/path/to/goreleaser` | the release dry run. **Install the PINNED version, not `@latest`:** `go install github.com/goreleaser/goreleaser/v2@v2.17.1` puts one in `$(go env GOPATH)/bin`. `.github/workflows/ci.yml` holds that version in one place, `DOSSIERX_GORELEASER_VERSION`, and says why: two suites reimplement this tool's changelog and archive behaviour against that named version, one of them citing its `changelog.go` by line. On `@latest` you get a red `make viewer-test` that has nothing to do with your change. Nothing here downloads it for you, on purpose |

CI supplies three of the four as pinned job dependencies — Node via `actions/setup-node`,
`goreleaser` via the `DOSSIERX_GORELEASER_VERSION` pin, and `npm` with the Node setup. **The browser
is not pinned:** the viewer job probes the runner image for a preinstalled Chrome and fails if it
finds none, so its version moves with the image and nobody reviews that move. Locally, the shape of
an invocation that runs everything is:

```sh
DOSSIERX_TEST_BROWSER="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
DOSSIERX_TEST_GORELEASER="$(go env GOPATH)/bin/goreleaser" \
make viewer-test
```

**`make hook-test`** is the pre-commit gate's suite. The gate is shell driving a real binary
against a real git repository, so no Go test can cover it; CI runs this on Linux, macOS and
Windows, because the hook body executes under git's own bundled `sh`.

### Two places a local green proves less than it looks like

Neither of these is a reason to skip the local run. Both are reasons not to report one as the
result.

**The CI `git hook` matrix outranks a local `make hook-test`.** The hook body executes under git's
own bundled `sh`, which is a different shell on each of the three platforms — so a local PASS
covers one of three legs. Treat it as fast feedback; the authoritative answer is the matrix.

**A green against a Chromium fork is not a green against Chrome.** `resolveBrowser` falls back to
Comet when no Chrome or Chromium is installed, and since v0.5.0 the suite logs which browser it
actually drove — read that line before quoting a result. The difference is not hypothetical: Comet
serves its own `chrome://` UI and posts telemetry through a path containing `/api/`, and
`graph_offline_test.go` was reading that traffic as the page's own — 119 request events on the tab,
118 of them the browser's. the misread was fixed by attributing requests via CDP's
`DocumentURL`, but the class remains: a fork is a different browser, and the suite cannot tell you
what Chrome would have done.

Comet deliberately stays in the candidate list. On a machine with no other browser, removing it
means the suite cannot run locally at all, and a suite nobody can run is worse than one whose
evidence needs a caveat. `DOSSIERX_TEST_BROWSER` is how you remove the caveat: it pins which
browser is driven, and turns "no browser found" from a skip into a failure.

`tests/nested_module_coverage_test.go` keeps this list from quietly going stale, and it treats
RUNNING a module and LINTING it differently. For running the suite it is a **conjunction**: the
module needs a CI step with `working-directory: <mod>` AND a Makefile line carrying both
`cd <mod>` and `go test`, so deleting either one alone reds the build. Only the lint check is a
**disjunction** — a CI lint step or a Makefile lint target satisfies it, and deleting both is what
reds it.

### Adding a file is two steps: `surfaces.yaml` has to claim it

`go test ./...` also fails when a tracked file matches **no** entry in `surfaces.yaml`, so a
green tree needs that file edited in the same commit that adds the file. This is new in v0.5.1
and it applies to every kind of file — a script, a doc, a workflow, a package — not just to Go.

`surfaces.yaml` lists every client-facing surface this project has (the README, the changelog,
the skills, the install scripts, the site, the binary) and, beside them, the paths that are
deliberately out of scope with the reason each is. `tests/surfaces_manifest_test.go` requires
every tracked file to be claimed by **exactly one** of the two — exactly one, not at least one,
because an out-of-scope entry that quietly swallowed a path a surface also claims would shrink
what the release gate reads without anyone deciding to. Both entries get named and the build
goes red.

So when you add a file, say which it is:

- **It extends a surface that already exists.** `binary-and-viewer` claims `cmd/dossierx/` and
  `internal/` by directory (its `not:` list hands the `_test.go` files and the golden generator
  to `tests-and-fixtures`), so a new engine package needs nothing here. A new file under
  `scripts/`, under `docs/`, or at the repository root almost certainly does: those are claimed
  path by path, not by directory.
- **It is not client-facing at all** — a test, a fixture, a build or CI file. Add it to the
  matching `out_of_scope` entry (`tests-and-fixtures`, `repository-automation`,
  `toolchain-config`, …) and write the reason it makes no claim a release could falsify.
- **It is a surface nobody has declared yet.** Write a new entry with `what` and `reach` filled
  in. Those two fields are read by a human at release time, when one agent per surface reads
  that surface's prose against this repository's code, so "who is hurt if this goes stale"
  belongs in `reach` in plain words.

The pattern grammar (`dir/`, `**/`, `*`, exact paths, and per-entry `not:` exceptions) is
documented in the comment at the top of `surfaces.yaml`. A file matching nothing is a build
failure on purpose: the next undeclared surface should cost a compile, not an audit.

**There is a third kind of entry, and it can fail your build on a file you did not touch.** New
in v0.5.2, a surface may carry a `reads:` list naming exact paths it borrows but does not own —
the documents its reviewing agent needs in order to judge its own prose. `reads:` is not a second
claim: ownership is decided by `paths:` alone and the exactly-one rule is about `paths:` alone.
What it adds is a second way to go red. `gateSurfaceReferences` in
`cmd/dossierx/gate_fingerprint_test.go` requires every `reads:` path to be an exact tracked file
that the declaring surface does not already claim, and a path that has **moved refuses the whole
fan-out** rather than producing a shorter bundle. So moving, renaming or deleting a tracked file
that some other surface declares in `reads:` is a red build even though your file is still
claimed exactly once — and the failure names a surface you may have had no reason to read. Fix it
by updating the `reads:` entry, not by widening it to a pattern: patterns are refused, precisely
so a borrowed set cannot grow without someone deciding.

## Linting

```sh
golangci-lint run    # the root module
make viewer-lint     # viewer-tests/, which the line above does not read
```

CI runs both, as two steps of the lint job, with the pinned version in
`.github/workflows/ci.yml`; install the same version locally to avoid surprises.

**`golangci-lint run ./...` at the root has the same blind spot `go test ./...` has.**
`viewer-tests/` is a separate module, so a root run does not read a line of it. That is the same
boundary the two-suites section above describes for tests, and it needs the same second command.

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

Maintainers only: [docs/RELEASING.md](docs/RELEASING.md). Releasing is a driver,
`make release-publish`, not a list somebody works through: it refuses to tag unless
the tree already declares the release being cut, and it verifies the published
archives itself before `main` moves. What that document is for is the half no
program does — reading the release gate's findings and ruling on them, and the
three post-publish checks (the deployed site, the workflow run, the CDN) that the
driver hands back to a person because they leave this repository.

## Package boundaries

`internal/` is organized as a dependency graph with a small number of hard rules, enforced by
`.golangci.yml`'s `depguard` configuration rather than left to convention alone (see "Why
loosely coupled" below for why that matters here). The current packages, which is what
`go list ./internal/...` prints today rather than what this list used to say:

`buildorder`, `catalog`, `check`, `cliout`, `comments`, `config`, `digest`, `graph`,
`implink`, `lint`, `loader`, `lock`, `model`, `reaudit`, `render`, `render/components`,
`render/markdown`, `render/markdown/generate-goldens`, `serve`, `urlsafe`.

The rules:

- **`model` and `config` are dependency-free leaves.** Neither may import any other package
  under `internal/`. Everything else in the engine depends on the claim/config shapes these
  two packages define, so they must not depend back on anything.
- **`graph` audits claims, not code.** `internal/graph` may not import `internal/implink`,
  `internal/lint` or `internal/render` — the last as a prefix, so `internal/render/components`
  is denied with it. That is `.golangci.yml`'s `graph-audits-claims-not-code` rule.
- **`render` is a top-of-stack sink for everything upstream of it.** It may import other
  `internal/` packages, and the five that sit upstream in the pipeline may not import it back:
  `lint`, `catalog`, `lock`, `loader` and `implink`. That is the exact set `.golangci.yml`'s
  `render-is-top-of-stack` rule denies, and the rule is deliberately not "nothing under
  `internal/`" — `check` and `serve` both import `render` and are supposed to. They are the two
  packages that DRIVE the pipeline rather than sitting inside it: `check` runs lint → catalog →
  render → ledger → scan as its five stages, and `serve` renders the viewer from memory on each
  request. The rule to keep in your head is direction, not a blanket ban: nothing that produces
  input for the renderer may depend on how output gets drawn.
- **`lint` must never import `lock` — nor `render`, `reaudit` or `buildorder`.** The `lock`
  dependency runs the other way: `lock` imports `lint`, because locking a claim is gated on that
  claim passing a clean lint run. A `lint` package that imported `lock` back would create a cycle
  and would also be backwards from what the lock operation actually needs — it needs lint's
  answer, not the other way around. The other three are denied by the same
  `lint-must-not-import-lock-and-friends` rule for a simpler reason: lint has no business
  depending on any of them.

If you're adding a new package, place it according to which of these four groups it
logically extends (or ask in the PR description if it's genuinely new territory), and run
`golangci-lint run` before pushing — the depguard rules will fail the build on a violation
rather than let it land silently.

**Wrap every propagated external error with `%w`.** `.golangci.yml` keeps `wrapcheck` disabled and
cites this document as where the convention lives, so it is a convention a reader enforces rather
than a linter: an error crossing out of this codebase's own packages carries `%w` and a sentence
saying what was being attempted, so `errors.Is` still works at the top of the stack and the message
names the operation rather than only its cause.

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
