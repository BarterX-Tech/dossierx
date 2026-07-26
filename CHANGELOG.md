# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] — v0.3.0

The agent-first restructure. DossierX has two users with opposite needs: an **agent** that
operates it, and a **human** who reviews what the agent did. Until now both were half-served by
one command line. v0.3.0 gives each its own surface and takes the other away — the agent gets a
19-command machine-readable CLI, the human gets the viewer and one command (`dossierx serve`).

Alongside the split, this release closes the gap that made the split worth making: until now a
locked claim could be hand-edited and **nothing would notice**. The new lock ledger records what
was approved, when, by whom, and on whose words, and a gate compares the claims against it in
`dossierx check`, in a pre-commit hook, and in CI.

**This release is not backward compatible at the CLI.** Ten commands were removed and four were
moved. The migration table below maps every one of them.

### Migration — every retired command and its replacement

| Retired | Replacement | Notes |
| --- | --- | --- |
| `dossierx lint` | `dossierx check --validate` | `--validate` is a **read-only** run — no claim files, no lock store, no `.catalog.json`, no viewer. Plain `check` writes all four. |
| `dossierx lint --json` | `dossierx check --validate` (JSON is the default format) | Findings are `data.lint_findings[]`, in snake_case: `lint`, `claim_id`, `severity`, `message` (the old bare array used Go field names). |
| `dossierx catalog` | `dossierx check` | It was a stage of `check`, exposed as a verb only because the extraction had no Go API. |
| `dossierx render` | `dossierx check` | Same. |
| `dossierx deps <id>` | `dossierx claim show <id>` | Reports both edge directions as before, **plus** lock state, `review_pending` and its trigger, code links with drift, comment counts, and `next_actions`. |
| `dossierx stale` | `dossierx claim list --review-pending` | Names the claims and reports the count, as before. The bespoke "nothing locked" message is gone; an empty project is an empty result. |
| `dossierx coverage` | `dossierx claim list --migrated` | Reports the same ratio (`count`, `total`, `percent_of_total`) **and** names the claims. |
| `dossierx implink set` | `dossierx claim link` | Identical flags (`--module --claim --file --symbol`) and identical behavior. |
| `dossierx implink status` | `dossierx claim show <id>` | Per-claim `implemented_in[]` with a `drifted` verdict on each file. `dossierx check` still reports module-wide impl-link status. |
| `dossierx lock <id>` | `dossierx claim lock <id>` | `--reason` is required (see below). |
| `dossierx unlock <id>` | `dossierx claim unlock <id>` | `--reason` is required. |
| `dossierx flag <id>` | `dossierx claim flag <id>` | Unchanged otherwise. |
| `dossierx reaudit <id>` | `dossierx claim reaudit <id>` | `--reason` is required with `--confirm`. |
| `dossierx comment edit` | the viewer | A review history the agent can rewrite is not a review history. Still fully available over `dossierx serve`'s HTTP API. |
| `dossierx comment delete` | the viewer | Same. |
| `dossierx comment resolve` | the viewer | Advisory rights already forbade an agent acting on a human-authored thread, and every viewer thread is human-authored — so on the CLI this could only ever act on the agent's own threads. The human's **Resolve click is the approval** the lock gate waits for. |
| `dossierx comment reopen` | the viewer | Same. |
| `dossierx comment list --json` | `dossierx comment list` (JSON is the default format) | Threads are `data.threads[]` inside the standard envelope, rather than a bare array. |

Nothing was removed from the **product**: `internal/comments` still implements all six comment
operations, and `dossierx serve`'s HTTP API — which is what the viewer drives — still exposes
every one of them. Only the CLI surface shrank.

### Added
- **`dossierx claim` — one noun for everything you do to a claim**: `show`, `list`, `new`,
  `lock`, `unlock`, `flag`, `reaudit`, `link`.
- **`dossierx claim show <id>`** — one call returning a claim's whole state: lifecycle status,
  lock state and timestamp, `review_pending` **and which of the three triggers caused it**, both
  edge directions (outgoing `mirrors`/`rests_on`/`governed_by`, and the derived incoming
  `mirrored_by`/`depended_on_by`), `implemented_in` with a per-file drift verdict, comment
  counts with the open thread ids, and `next_actions` — the legal next steps, computed from the
  same gates the write paths enforce, so the advice can never disagree with what the command
  would actually do.
- **`dossierx claim list`** with `--review-pending`, `--migrated`, `--drifted`, `--facet`,
  `--module`, and `--match`. `--match` is a fuzzy, ranked search over each claim's id and its
  derived title, so a human's "the retry-policy card in the contract facet" resolves to an id in
  one call; each result carries its `score` so an agent can tell a confident hit from a tie it
  should hand back.
- **`dossierx claim new <id>`** — the sanctioned way for an agent to author a claim. Since the
  release gates hand-editing claim YAML, an agent needs a way to write one at all; this writes
  `<claims_dir>/<id>.yaml` shaped to pass the lint suite immediately, validates the project with
  the new claim in it, and reports the verdict. The id grammar (`module.facet.slug`, kebab-case
  slug, module and facet the project actually declares) is enforced at the door rather than
  after the write. Draft authoring is deliberately unfrictioned: no `--reason`, no confirmation.
- **`dossierx check --validate`** — a read-only run over `internal/check`'s existing non-writing
  seam (the same one `serve`'s status endpoint uses). It exists because cutting `lint` would
  otherwise have turned the per-claim authoring loop into a writer.
- **`dossierx comment inbox`** — every open thread in the project in one call, oldest activity
  first, with `--since <RFC3339>` and an echoed `cursor` to poll with. Each thread carries
  **`agent_can_resolve`**, so an agent never spends a call earning `rights_denied` on a thread
  it was never allowed to close. `--since` is inclusive of its own second: comment timestamps
  have one-second resolution, and re-reporting a thread costs nothing while missing the human's
  comment breaks the entire loop.
- **A machine contract on every command.** `--format json|text` is global and **JSON is the
  default**; every run emits exactly one envelope — `{ok, command, data, warnings, error,
  stopped_at}` — and every failure carries a stable snake_case `error.code` (`lint_failed`,
  `claim_not_found`, `rights_denied`, `integrity_failed`, `unresolved_comments`, …) so a skill
  branches on a token instead of regexing an English sentence. `message` and `hint` are prose
  and will be reworded; `code` is the promise.
- **`--dry-run` on every mutating command**, reporting what would change, what is missing, and
  what else it affects. A dry run fails *only* when it cannot compute the preview: a refusal —
  including a missing required flag — is a **successful** blocked report (exit 0, `ok: true`,
  `data.blocked: true`), because "would this be allowed?" is a question, and answering "no" is
  not an error. `claim reaudit` keeps `--confirm` as its apply gate; `--dry-run` always wins
  over it.

### Added — integrity: the lock ledger

Claims are YAML in git, so nothing can *prevent* an edit. The goal is that no out-of-band edit
of a **locked** claim is *silent*. Before this release, every one of these was invisible: a
`status: draft` flipped to `locked` by hand (walking past the lint gate, hub-gating and the
unresolved-comment gate as though all three had passed); an edited locked body with no locked
dependents; a swapped `raw_html` on a locked, reviewed, allowlisted mockup — which the viewer
renders **unescaped**; a flipped `build_role`/`section`/`order`/`emphasis`; a comment thread
deleted straight out of the YAML; a `locked` flipped back to `draft` to dodge review.

- **The lock ledger.** Every legitimate approval — `claim lock`, a confirmed `claim reaudit`,
  `build-order lock` — now records `{hash, at, actor, reason}` for the artifact it approved, in
  `.dossierx-lock-store.json`. `reason` is the human's own approving words, carried in from
  `--reason`: the one part of the record a machine cannot generate for itself. `claim unlock`
  **releases** a record rather than deleting it, so the evidence that a claim was ever locked
  survives the window in which it matters.
- **`.dossierx-lock-store.json` and `.dossierx-comment-digest.json` are TRACKED ARTIFACTS.**
  Commit them; never `.gitignore` them. CI compares the claims against the ledger, so a project
  that does not track it has no gate. Documented in README, FORMAT.md, the skills, the hook
  installer's own output, and the CI workflow template.
- **A new hash, `LockedClaimHash`, separate from `ContentHash`.** It is a **deny-list** over
  every persisted claim field except `status`, `review_pending` and `comments` (each excluded
  because the engine rewrites it as ordinary bookkeeping), so a field added to the schema
  tomorrow is signed by default. `ContentHash` — the dependency-drift baseline — is
  **byte-identical to v0.2.0**: widening it would have flipped every locked claim in every
  existing project to `review_pending` on upgrade day. It covers ten of the schema's fields;
  the nine it cannot see include `raw_html`, the only path in the engine that renders author
  bytes unescaped, which is why the ledger could not reuse it.
- **The comment digest lives in its own store** (`internal/digest`, `.dossierx-comment-digest.json`),
  refreshed on every legitimate comment write. Putting it in the lock store would have made
  `dossierx serve` a lock-store writer and falsified this release's own headline guarantee.
- **Six named findings**, stable strings the hook, CI and the skills branch on:
  `lock-ledger-absent` (locked claims but no ledger file — a hard error, never a silent pass,
  because "no ledger means bless everything" would make `rm` the universal bypass),
  `lock-ledger-missing`, `lock-content-drift`, `lock-ledger-orphan`, `comment-ledger-drift`, and
  `lock-ledger-unreadable` (a ledger that exists but will not parse fails closed and loudly).
  The gate is deliberately **not** a lint: registering these in the lint registry would let one
  tampered file freeze locking project-wide and stop the viewer regenerating. It runs as
  `check`'s last step, after the catalog and viewer are written — a disputed project still
  regenerates its documentation, it just does not exit 0.
- **`dossierx check --staged`** judges the **git index** — what the commit will actually
  contain — reading content with `git show :<path>` rather than the worktree, and writing
  nothing at all. Outside a work tree it warns and exits 0.
- **A pre-commit hook installer**, `scripts/install-git-hook.sh` (plus `install-git-hook.ps1`
  for PowerShell). It **asks before writing anything**, resolves `core.hooksPath` instead of
  assuming `.git/hooks` (so a repo using husky/lefthook is installed into *its* hook directory,
  never hijacked), handles linked worktrees, refuses to replace a foreign hook without
  `--force`, and is a no-op when re-run. The hook body is embedded in the one file so an agent
  can drop it into a project that has the binary but not this repository.
- **A CI workflow template**, `scripts/ci/dossierx-check.yml`, to copy into a consuming
  project. **CI is the authority, not the hook**: git does not run `pre-commit` for a clean
  merge, a rebase, a cherry-pick or a revert, `--no-verify` is one keystroke away, and most
  contributors never installed the hook. If you adopt one of the two, adopt CI.
- **Grandfathering, once and loudly.** A project that locked claims before the ledger existed
  adopts them on first run of a build that has it, each record marked `grandfathered` — an
  adopted hash is content that was *observed*, not approved, and the flag stays on the record
  permanently so nobody mistakes the two. Adoption triggers only on an older store file being
  *present*; an absent store never adopts.

### Added — graph integrity and readability

- **New `self-edge` lint** (error): a claim may not name its own id in `rests_on`, `mirrors`, or
  `governed_by`. A self-edge is trivially satisfied by every content rule — a claim always
  equals itself, always mirrors itself back, always resolves — so it asserted nothing while
  looking like a well-formed edge.
- **New `governed-cycle` lint** (error): `governed_by` is now cycle-checked, with its own
  message distinct from `cycle`'s. Following `governed_by` must reach `type: none` in finitely
  many steps; a cycle means a set of claims whose authority rests only on each other, which is
  to say on nothing.
- The `cycle` lint's depth-first search is now an **explicit stack** rather than recursion. Its
  depth was the longest authored edge chain in the project, with no engine-imposed bound.
- **Readable edge labels in the viewer** (issue #11). A claim-to-claim edge used to render as
  its raw id. It now renders as a derived label with prefix elision keyed on how far the target
  is from the claim doing the pointing — bare within the same facet, `facet › Label` across
  facets, `module · facet › Label` across modules. The machine id stays reachable via
  `data-claim-id` and a tooltip, and an id that is not exactly three non-empty segments renders
  as the **raw id, verbatim** — never a partial label — because rendering does not run the lint
  suite.

### Fixed

- **The viewer's 💬 chip now appears on every card, not only on cards that already have a
  thread** — so the first comment on a claim can actually be opened, which was the whole
  premise of the human review loop. Two gates were involved and both had to move together: the
  server emitted no chip for a zero-thread claim, and the client hid any chip reading `0`, so
  an empty chip would have vanished the moment it was clicked. Empty chips are now hidden only
  when no live comment API answered — the static `file://` export, where there would be nothing
  to open — and the three chip states (`--open`, `--resolved`, `--empty`) are mutually
  exclusive, so "no comments" no longer reads as "everything raised was settled".
- **Every command the engine advises you to run is a command that exists** — and, where the
  verb requires it, one that would succeed as printed. `check`'s next steps named five retired
  invocations (`dossierx lock`, `dossierx reaudit`, `dossierx implink set`, and a
  `dossierx comment resolve` that this release deliberately removed from the CLI); they now
  name `claim lock … --reason`, `claim reaudit`, `claim link`, and — for an open thread — the
  human's viewer rather than any agent-runnable command, because resolving a thread is the
  approval itself. `lock`'s and `build-order propose`'s comment refusals and `build-order`'s
  cycle diagnostic were stale in the same way and were corrected alongside. This matters more
  than the wording suggests: the v0.3.0 skills tell an agent to read `next_actions` and
  `error.hint` instead of re-deriving the lifecycle, so a stale hint is advice an agent acts on.
- **Generated viewers no longer advertise a deleted command.** Every `viewer/index.html`
  carried `generated by dossierx render … re-run "dossierx render"`; the banner now names
  `dossierx check`, which is the command that actually regenerates it.

### Changed
- The CLI is **19 leaf commands under 6 nouns**, down from 26. A test pins the exact set, so
  adding to the surface is a decision someone makes on purpose.
- `--reason` is **required** on `claim lock`, `claim unlock`, `claim reaudit --confirm`, and
  `build-order lock`. Under the new split the human never types these — they say "good, lock it"
  in chat and the agent executes — so `--reason` is where the human's own approving words enter
  the record.
- Exit codes are **unchanged**: still 0 / 1 / 2 with the meanings the README documents. The
  fine-grained signal is `error.code` in the envelope, not a new status.
- `dossierx check` now runs **four** stages, not three: lint, catalog, render, and the
  lock-ledger gate. `--validate` is the read-only form — it runs the lint gate and the ledger
  gate in memory and writes nothing, and is honest about what it therefore does not do (no
  `review_pending` reconcile, no catalog, no viewer, no source scan for code links).
- `dossierx skills export <dir>` now writes **five** skill bundles. A project that exported the
  skills before must **re-export** to pick up the new router and the rewritten companions; the
  export overwrites in place, so re-running the same command is all that is needed.

### Docs
- **README rewritten around the two roles.** It now opens on who does what, carries a
  copy-paste bootstrap block a human hands to their agent (install, export the skills, propose
  the config, *ask* before installing the git hook, prove itself with `check`, commit the
  ledger, then hand `dossierx serve` back to the human), and documents `dossierx serve` as the
  human's one command. The lint → catalog → render walkthrough and the per-verb command table
  are gone: a human is not expected to run any of it, and the CLI is now documented as a
  machine contract — the envelope, `error.code`, `--dry-run`, and the unchanged exit codes.
- **FORMAT.md gained an "Integrity invariants" section**: the two tracked ledger files, what
  `LockedClaimHash` signs and the three fields it deliberately does not, all six findings and
  the invariant each one enforces, and how one-time adoption works. It also gained the three
  **graph invariants** (`rests_on` acyclic, `mirrors` a reciprocal 2-cycle, `governed_by`
  terminating, and no self-edges in any of the three), and a short, quotable statement of the
  **markdown ceiling** — the subset `body`, `rows` cells, `steps` and comment bodies all render
  through, with everything outside it staying literal text.

## [0.2.0] - 2026-07-26

### Added
- **Comments on claims** — threaded, Google-Docs-style review discussion attached to any claim,
  so a human and an agent can talk *about* a claim without editing it.
  - New `dossierx comment` command group: `add`, `reply`, `resolve`, `reopen`, `edit`, `delete`,
    and `list` (with `--open` and `--json`). Every mutating verb takes `--as human|agent`
    (recording the actor's role, which the advisory-rights rule keys off) and takes the
    project-wide claims lock, so concurrent CLI and browser edits can't clobber each other.
    Threads and replies carry engine-minted ids; the new `comments:` claim field is `omitempty`
    and **excluded from a claim's content hash**, so commenting on a claim never rewrites an
    uncommented claim's bytes and never flips its dependents to `review_pending`.
  - New `dossierx serve`: a localhost-only HTTP server that renders the claims viewer from
    memory and exposes the same comment operations to the browser, with an interactive thread
    panel and composer, a same-origin admission layer (Host/Origin checks, no CORS), and live
    reload that pushes changes over server-sent events as claim files change on disk. Binds a
    random high port by default (`--port` to override) and never writes `viewer/index.html` or
    `.catalog.json` on a page load. **Adds no new runtime dependency** — the file watcher is a
    standard-library modification-time poll, so the runtime stays cobra + yaml.v3 only.
  - An open comment thread is now a third `review_pending` trigger on a locked claim, alongside
    dependency drift and `dossierx flag`. A claim **cannot be locked while it has an unresolved
    comment thread** (and `dossierx build-order propose` refuses a module with any open thread);
    `review_pending` clears when the last open thread is resolved, unless drift or a flag also
    stands. `dossierx reaudit` refuses a claim that is `review_pending` only because of an open
    thread — there is no content diff to confirm, so it directs you to resolve the thread
    instead. `dossierx check` reports open-comment counts per module and points its next-steps
    at the exact `dossierx comment resolve` command.
  - New `comments-unresolved` lint (warning severity): surfaces claims that still carry an open
    comment thread.
- A fourth embedded Claude Code skill, **`dossierx-comments`**, teaching an agent when to
  comment versus `flag` (the discriminator is "is there a specific proposed wording change?"),
  the advisory-rights rule (an agent never resolves a human-opened thread), and how an open
  thread gates locking. The three existing skills were updated for the new three-trigger
  lifecycle and cross-linked to it.

### Changed
- `dossierx skills export` now writes **four** skill bundles instead of three. Projects that
  previously exported the skills (e.g. into `.claude/skills/`) with the old three bundles must
  **re-export** to pick up `dossierx-comments`; the export overwrites in place, so re-running
  the same `dossierx skills export <dir>` is all that's needed.

### Docs
- Documented the `comments:` claim field and rewrote the lock lifecycle in `FORMAT.md`,
  `README.md`, and the skills to the three-trigger (dependency drift / `dossierx flag` / open
  comment thread) model with its three matching clearers.

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
- Build-order staleness is now computed structurally: `status` re-derives the order a fresh
  `propose` would produce from the current claims and flags the locked artifact stale whenever
  they differ. This covers every order-affecting change in one check — a covered claim's
  `build_role` or `order:` edit, a source-file rename, `rests_on` reordering, additions,
  deletions, and an excluded claim promoted into a phase (or edited to an empty/invalid role) —
  plus content edits via the existing per-claim hash. It also runs for a locked module that
  covers only out-of-scope claims, which previously escaped every check and could not be relocked.
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

[Unreleased]: https://github.com/BarterX-Tech/dossierx/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.2.0
[0.1.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.2
[0.1.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.1
[0.1.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.0
[0.0.3]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.3
[0.0.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.2
[0.0.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.1
