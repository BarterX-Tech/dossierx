# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-07-24

### Fixed
- `layout: steps` claims rendered a numbered circle (`.snum`) that sat visibly higher than the
  first line of step text. Step text is routed through the shared markdown renderer, which wraps
  it in a `<p>`; the `<p>`'s default browser top margin pushed the text down inside the
  `display:flex` step row while the fixed-height number circle stayed flush at the top. The
  default viewer stylesheet now resets step-body block margins (first-child top / last-child
  bottom) so the number and first line align. Affects every `layout: steps` claim in any project
  using the default viewer theme; a project overriding `style.css` is unaffected.

## [0.1.0] - 2026-07-23

### Changed
- Renamed every generic "docs" placeholder to the tool's actual name, `dossierx`: CLI-invocation
  examples across comments, tests, README/ROADMAP/FORMAT, and the website; the `docs-claim:`
  source tag (including the real Go regex in `internal/implink/scan.go`); `docs-v1` naming in
  the skill docs; and the default viewer title (`"docs viewer"` → `"dossierx viewer"`).

### Breaking
- `.docs-lock-store.json` and `.docs-flag-store.json` are renamed to `.dossierx-lock-store.json`
  and `.dossierx-flag-store.json`, with no migration. An existing project's lock/flag store will
  not be found after upgrading past this release — hence the minor version bump rather than a
  patch, under pre-1.0 semver.

## [0.0.3] - 2026-07-22

### Added
- The rendered viewer's sidebar now shows a "Generated <timestamp>" footer,
  the same render-time timestamp already stamped into the leading
  generated-by HTML comment, so a reviewer can tell how fresh the page is
  without needing to view source.

## [0.0.2] - 2026-07-22

First real CI run (Linux/Windows/macOS matrix, `-race`, gofmt, golangci-lint) surfaced gaps
that only local macOS testing had missed:

### Fixed
- Two files had minor gofmt drift.
- The CLI-integration test harness built the `dossierx` test binary without a `.exe` suffix on
  Windows, so `os/exec` couldn't launch it.
- Two POSIX-permission-based tests (unreadable file, read-only directory) don't apply under
  Windows's ACL model; skipped there.
- A concurrency test's non-trimpath "negative control" assertion is inconclusive on GitHub's
  windows-latest image (trimpath-equivalent by default); skipped there, the actual positive
  guarantee (a `-trimpath` build doesn't leak paths) is unaffected and still runs everywhere.
- **Real bug:** running many `dossierx lock` invocations concurrently against the same
  `claims_dir` could fail on Windows with a transient "being used by another process" error —
  Windows's mandatory file locking can collide a concurrent atomic rename with a concurrent
  read of the same claim file, unlike POSIX's atomic rename semantics. Both the read and write
  paths in `internal/loader` now retry a few times with a short backoff, Windows-only.
- `golangci-lint` config/version pinning tightened so CI's linter binary matches this module's
  actual `go 1.26` floor.

## [0.0.1] - 2026-07-21

DossierX is a config-driven CLI that turns YAML "claims" — atomic, reviewable facts about a
system — into a linted, validated, human-reviewable HTML documentation site, with a built-in
audit trail via a lock/lint/reaudit lifecycle: a claim is freely editable while in `draft`,
gets promoted to `locked` only once it passes lint, and any subsequent drift (a changed
dependency, code that no longer matches) is surfaced as `review_pending` and resolved through
an explicit, confirm-before-write reaudit rather than a silent auto-update.

The engine originated as an internal documentation tool built and hardened against real,
multi-module projects — proving out the claim schema, the lint → catalog → render → check
pipeline, the lock lifecycle, per-module build ordering, and claim-to-code linking against
production use before anything here was written with an external audience in mind. This
release extracts that engine into its own repository and genericizes it: every project-specific
name, facet, and module that had leaked into the original code has been removed, so the only
project-specific input the CLI now reads is a project's own `project.config.yaml`.

This is DossierX's first public release. It ships the `dossierx` CLI (`lint`, `catalog`,
`render`, `check`, `deps`, `coverage`, `stale`, `lock`/`unlock`, `reaudit`, `build-order`,
`flag`, `implink`), documented in [README.md](README.md), along with three Claude Code skills
in `skills/` for projects that consume DossierX to author claims, derive build order, and link
code back to claims from within an agentic workflow.

[Unreleased]: https://github.com/BarterX-Tech/dossierx/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.1
[0.1.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.0
[0.0.3]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.3
[0.0.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.2
[0.0.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.1
