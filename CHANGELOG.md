# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2026-07-25

Consolidated audit-fix release: a deep audit against a real 202-claim consumer project
surfaced 25 confirmed defects, fixed together here rather than as a stream of point
releases. Despite adding user-facing capabilities this is a patch bump — `internal/` is not
importable, there are no breaking CLI changes, and the lock-store migrates automatically.

### Added
- `dossierx version` subcommand and a `--version` flag (previously the binary could not
  report its own version, and the release-time `-X` ldflags targeted variables that did not
  exist).
- Markdown `[text](url)` links now render as anchors in claim bodies **and** in `table`
  cells; backtick code spans now render inside table cells too. Link URL schemes are
  allowlisted (`http`, `https`, `mailto`, relative, `#`-fragment); `javascript:`, `data:`,
  and `vbscript:` are neutralized to inert text. Bare URLs are not autolinked.
- New `status-shape` lint: `status` must be exactly `draft` or `locked`.
- `rows-shape` now flags any non-string table cell (number/bool/list/map) instead of letting
  it render as Go-native text (e.g. an unquoted `1.0` silently becoming `1`).

### Fixed
- A YAML file containing a second `---`-separated document silently dropped all but the first
  claim; it is now a hard load error (one claim per file is enforced).
- `lint --json` printed `null` instead of `[]` when there were no findings, and
  error-severity findings serialized with an empty severity; both now emit correct JSON.
- `lock` / `unlock` / `flag` returned exit code 1 for an unknown claim id; they now return
  exit 2, matching the documented exit-code contract (as `deps` / `reaudit` already did).
- `build-order status` and `implink status` accepted an unknown `--module` and exited 0; they
  now reject it.
- The invalid-`layout` lint message omitted `mockup`; it now lists all seven layouts.
- Dependency-hash baselines were keyed by dependency id alone and shared across dependents,
  so locking or reauditing one claim erased another's drift baseline and that claim would
  never flip to `review_pending` when the shared dependency changed. Baselines are now keyed
  per-dependent; the on-disk lock-store is versioned and migrates automatically, re-arming
  baselines for every currently-locked claim from current content on first run so drift
  detection is live immediately after upgrade without a manual re-lock.
- `unlock` left a claim's pending flag in the flag-store, so a later dependency-drift reaudit
  could silently re-apply stale pre-unlock content; `unlock` now clears the flag entry.
- `unlock` hard-failed when the flag-store file was missing or corrupt; flag-clearing is now
  best-effort — a missing store is silent, an unreadable one warns and still returns the claim
  to draft — so the recovery escape hatch stays reliable.
- `flag` on a `table` / `steps` / `mockup` claim rewrote only the body, leaving the rendered
  rows/steps/raw HTML stale while clearing `review_pending`; `flag` is now refused on those
  structured layouts (use unlock → edit → relock).
- Build-order staleness now flags an artifact stale on every order-affecting change, not just
  content edits, in-phase deletions, and additions: a covered claim's `build_role` change
  (which reorders its phase), an excluded out-of-scope claim being deleted, and an excluded
  claim being *promoted* into a real build phase (or edited to an empty/invalid role, mirroring
  what `propose` would now reject). Staleness also runs for a locked module that covers only
  out-of-scope claims, which previously escaped every check and could not be relocked.
- Build-order staleness ignored newly-added claims (an artifact could silently omit a claim);
  additions now flag the artifact stale, symmetric with deletions.
- `build-order lock` re-blessed a stale artifact without recomputing its order; it now refuses
  a stale artifact and directs you to re-propose first.
- The Build Order section was emitted without an id and hidden by the facet-tab logic on every
  view, making the feature unreachable; it now renders visibly and its cards are deep-linkable.
- A module overview/router claim was injected into every facet with the same id, producing
  duplicate ids (invalid HTML) and broken deep-links; the canonical id is now stamped on a
  single copy while the overview stays visible in every facet.
- The offline-guarantee test walked the whole repo including built site bundles, so it went
  red locally after a site build while passing on a clean CI checkout; it is now scoped to the
  engine directories with a positive control.

### Security
- The `raw_html` mockup allowlist only inspected double-quoted attributes, so single-quoted,
  unquoted, and valueless event handlers, styles, and external `src` bypassed it. It is
  replaced with a default-deny parser covering every quote form, and an `img` `src` is now
  HTML-entity-decoded and stripped of ASCII control bytes before the relative-only check, so
  neither an entity-encoded (`&#47;&#47;host`) nor a control-char-obfuscated (`ht&#9;tp://host`)
  absolute/external URL can slip past it.
- `render` and `catalog` never ran the `raw_html` gate (only `lock` did), so they could
  publish unreviewed or non-allowlisted mockup HTML into the viewer; both now enforce the gate
  and fail on a violation.

### Docs
- Corrected the build-order skill (orientation-note/overview claims do carry a `build_role`
  and render in the orientation phase). Updated the claims and code-links skills, `FORMAT.md`,
  and the marketing site to reflect the behavior above.

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

[Unreleased]: https://github.com/BarterX-Tech/dossierx/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.2
[0.1.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.1
[0.1.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.0
[0.0.3]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.3
[0.0.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.2
[0.0.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.1
