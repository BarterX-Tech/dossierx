# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Bound readiness evaluation and catalog emission under lock policy v1 (issue #62).
  Readiness now collects independent causes and conditions via bounded BFS traversal
  with a deterministic representative witness path (shortest path with a lexicographical
  tie-break, and global cycle witness minimization) rather than enumerating every
  dependency route. This eliminates exponential route proliferation ($O(k^d)$) on dense
  dependency graphs, bounding traversal edge exploration to $O(V + E_r)$ per claim,
  path-construction work to $O(V \cdot D)$, and catalog output to $O(V \cdot R \cdot D)$ bytes.

## [0.7.8] - 2026-09-06

### Changed

- Lock policy v1 separates local approval from dependency readiness. A readable
  draft `rests_on` input can receive a conditional local approval with a visible
  `dependency_unapproved` condition; missing, retired, unreadable, cyclic, and
  doctrine-gated required inputs refuse a new approval. Existing projects stay
  on their recorded policy until `dossierx claim migrate-lock-policy --reason
  "..."` records the human adoption without rewriting standing approvals.
- `claim lock --dry-run` now returns the reviewed requested set, dependency
  conditions, and a request-bound proposal token. Every singleton or group
  write needs its matching `--proposal`; omitted, malformed, stale, or
  wrong-request tokens refuse before an approval, receipt, baseline, or claim
  write. The generated viewer and API show local approval, readiness conditions
  and review-cause paths on each claim.
- Inline emphasis and strikethrough now work in the inline-only markdown
  surfaces used by rows and table cells, with a dedicated golden case so the
  construct remains covered in the exported surface inventory.
- v0.7.7's `build/` artifact layout, Build order tab, Mermaid renderer and
  `build-order show` remain part of the release. The v0.7.7 comparison records
  ten intentional policy-v1 render changes: all five fixture catalogs gain
  readiness records and all five viewers gain readiness-card markup. Viewer
  bytes move as follows: `fixture-basic` 462,735 -> 468,304,
  `fixture-graph-demo` 589,713 -> 618,856, `fixture-portability` 464,414 ->
  470,693, `fixture-theme-flat` 4,084,957 -> 4,093,831, and
  `fixture-theme-preset` 466,204 -> 471,773. These are the otherwise-silent
  changes recorded in `testdata/render-across-releases.golden.txt` against
  v0.7.7; they make policy readiness visible in generated artifacts.

### Fixed

- A live viewer can insert each readiness card below a claim's links without
  using the section as an invalid DOM insertion parent. A rendering failure in
  the status refresh is no longer folded into the fetch failure path.
- Local approval previews and writes now enforce the `stores_are_tracked`
  repository guard for the policy-enabled lock path, so an ignored or
  unanswerable store is disclosed before review and refused before any write.
- A locked Build order artifact keeps rendering when its catalog has lost one
  of the artifact's claims. The approved node and Mermaid scripts remain
  visible, while the claim payload leaves the missing entry absent so a click
  reports an honest catalog miss.
- The v0.7.7 render comparison now marks whitespace-only context and real
  trailing-whitespace changes visibly, keeping report formatting clean without
  hiding byte-level differences.
- Generated viewers now keep the engine runtime as a dedicated JavaScript
  asset, preserving authored whitespace while retaining runtime comments and
  executing the script once on both full pages and live fragments.
- Build order now places the visible `Claim not found` status before diagrams
  when a locked artifact outlives a catalog claim; its graph and missing-node
  click path remain available without fabricating a claim card.
- Release comparisons share a changelog and canonical semver tag resolver
  across CI and render reports, with explicit baseline tags validated against
  the automatically selected immediate predecessor.

## [0.7.7] - 2026-09-04

### BREAKING — every dossierx artifact moved under build/

- Every file the engine generates at runtime now lives under one configurable directory,
  `build_dir` (default `build`, resolved against the config file's directory like `claims_dir`),
  one subdirectory per kind: `build/build-order/<module>.json`, `build/code-links/<module>.json`,
  `build/ledger/lock-store.json`, `build/ledger/comment-digest.json`,
  `build/ledger/flag-store.json`, `build/catalog/catalog.json`, `build/viewer/index.html`, the
  claim-file write sentinel `build/ledger/claims.lock`, and a `build/.gitignore` that `check`
  writes once. Nothing is written to the project root any more. A project that still keeps any of
  the seven legacy root files (`.dossierx-lock-store.json`, `.dossierx-comment-digest.json`,
  `.dossierx-flag-store.json`, `.build-order.<module>.json`, `.implementation.<module>.json`,
  `.catalog.json`, `viewer/index.html`) is refused on EVERY verb — `check --validate` and
  `check --staged` included — with `error.code: layout_legacy` and the exact block to paste: one
  `mkdir -p` for the destination directories, `git mv` for each file git tracks, `mv` for each it
  does not, and a removal for the catalog and the viewer; `--format json` carries the same list as
  `error.details.moves[]` (`{from, to, tracked}`). Signatures hash bytes, not paths, so nothing is
  re-locked. `check --staged` refuses the same way when the INDEX still carries a legacy file.
- `build_order.html` is no longer an override point: the tab it used to render is not a component
  partial any more, so a project whose `viewer.template_overrides` directory still holds that file
  is refused by name — `check` (and every render) stops working for it until the file is removed
  (see Removed below).
- `claims_dir` may no longer contain the build directory, sit inside it, or equal it, and
  `build_dir` may not be the config directory itself; a project with `claims_dir: .` — which only
  `serve` refused before — now fails to load on every verb until the claims move under a
  subdirectory or `build_dir` points outside them.
- `claim lock` (single and batch), `claim flag`, `claim reaudit --confirm` and `build-order lock`
  now require `git` on PATH when the project is inside a git work tree, and refuse with
  `error.code: store_gitignored` when git cannot answer there (no binary, a bare or corrupt
  repository) or when any path the engine writes under `build/ledger`, `build/build-order` or
  `build/code-links` is matched by `.gitignore` and not in the index. `check`, `check --validate`
  and `check --staged` do not refuse on an unanswerable git: they report
  `data.gitignore_check: "git not available"` (or `"not a work tree"`, `"outside the work tree"`)
  instead of a verdict and exit 0. A `golang:alpine`-style CI image with git stripped sees the
  first half as a new refusal.

### Changed

- `check`'s envelope gains `data.gitignore_check`, present only when the store-gitignored guard
  gave no verdict, and `warnings[]` carries one line per path that is ignored but already tracked
  (force-added, or committed before the pattern) — that ledger does reach collaborators, but
  nothing will stage the next new artifact under `build/`.
- Every finding and refusal that names a store prints its `build/ledger/...` path.
- The viewer's per-module Build Order sub-tab and its flat per-claim list are replaced by a
  top-level **Build order** tab that renders the same locked artifact as a six-phase mermaid
  diagram — nodes for each claim, ghost nodes for a `rests_on` target the artifact placed in an
  earlier phase of the SAME module, and (in the tab's own list, not the diagram) a cross-module
  dependency list per phase. A viewer for a project with a locked build order now ships the
  vendored mermaid renderer (`mermaid.min.js` 11.17.2, 3,572,661 bytes): measured on the
  regenerated fixture viewers (`wc -c` on `testdata/<fixture>/build/viewer/index.html`, before ->
  after, against the v0.7.6 baseline), `fixture-theme-flat`, the one fixture with a locked order,
  grows from 490,505 to 4,084,957 bytes (+3,594,452), and the four fixtures with no locked order
  grow by under 1 KB each (`fixture-basic` 461,835 -> 462,735, `fixture-graph-demo`
  588,865 -> 589,713, `fixture-portability` 463,514 -> 464,414, `fixture-theme-preset`
  465,304 -> 466,204). A consuming project that commits its viewer should expect the same
  ~3.6 MB step the first time it locks a build order. The tab's own section and per-module
  groups carry the ids
  `dossierx-build-order` and `dossierx-build-order-<module>`, outside the space a module's name
  is slugged into, so a module named `build-order` keeps its own `#build-order` section and the
  tab its diagrams; a module whose name slugs to one of the tab's ids (`dossierx build order`) is
  refused at render by name. Two claim ids that sanitise to one mermaid node id (a module or
  facet name differing only in `-` vs `_`) are drawn as two nodes — the second carries a short,
  stable hash suffix — and a locked artifact the tab cannot draw (a hand-edited phase name or
  edge) costs that module its tab entry and one `warnings[]` line, never the viewer.
- `internal/check`'s git runner carries git's exit status (`runStatus`), so a `check-ignore` that
  exits 128 is reported as "git could not answer" rather than read as "not ignored".
- The lock, digest and flag stores are decoded strictly from bytes (`lock.DecodeStore`,
  `digest.DecodeStore`, `reaudit.DecodeFlagStore`), which is what lets `check --staged` tell the
  engine's `lock-store.json` from an unrelated repository file of the same name; the flag store
  gains a `version` field (an older store without one still loads).
- Rendered output across releases (`testdata/render-across-releases.golden.txt`, regenerated
  against v0.7.6): 152 artifacts compared, 0 added, 0 removed, 0 silent, and 5 viewers
  rendered differently after their own inputs changed — every fixture viewer, because this
  release adds tracked inputs under each fixture's `build/` (`build/.gitignore`, the digest). The
  four fixtures with no locked build order (`fixture-basic`, `fixture-graph-demo`,
  `fixture-portability`, `fixture-theme-preset`) each move by +266/-196 (graph-demo +266/-200)
  lines, and every hunk is the Build order tab: the `.bo-*` stylesheet block replacing the
  `.build-order-module` rules, the tab's print rules, the removed `buildOrderToModule` deep-link
  map and `prepareBuildOrder`, and the `:not(.build-order-section)` guards on the active-section
  queries. `fixture-theme-flat`, the one fixture with a locked order, is rewritten (490,505 ->
  4,084,957 bytes) because its viewer now carries the vendored mermaid renderer. The v0.7.6
  baseline already holds the theme work and the fenced-block seam fix, so neither appears here.

### Added

- `build_dir` in `project.config.yaml` (optional, default `build`).
- The `store-gitignored` ledger finding and the `store_gitignored` / `layout_legacy` error codes;
  `stores_are_tracked` as a `--dry-run` precondition on `claim lock`, `claim flag`,
  `claim reaudit --confirm` and `build-order lock`.
- README's "Where DossierX writes" subsection with the layout table and the `.gitignore`
  replacement block a bare `build/` pattern needs (`build/*` plus a slash-less negation and a `/*`
  re-include per tracked kind).
- `dossierx build-order show --module <name> --format json|text|mermaid`, the read-only leaf that
  renders a module's stored build-order artifact, proposed or locked (`data.locked` says which) —
  the one leaf besides the global `--format json|text` that also accepts `--format mermaid`, for
  pasting the diagram into a PR. It never recomputes the sequence; it reads exactly what
  `propose`/`lock` last wrote.

### Removed

- The per-module Build Order sub-tab and its flat per-claim list (`internal/render/components/build_order.html`,
  the `buildOrderToModule` deep-link map) — replaced by the mermaid **Build order** tab above.
- `build_order.html` as a `viewer.template_overrides` override point: the tab it used to render
  is no longer a component partial, so a project's override directory naming it is refused rather
  than silently ignored.
- `TestClaimEdgeListHTML_MatchesTheSharedFooterMarkup`, decision C5's shared-markup pin — it
  existed to hold the removed sub-tab's list markup to the claim-edge-list's shared footer, and
  goes with the list it was pinning.
## [0.7.6] - 2026-09-02

### Added

- The viewer can now be restyled with a project's own colours and fonts through `viewer.theme`:
  an optional built-in `preset` (`claude` today), an optional theme file (`extends`) a project can
  layer on top of a preset, per-colour-scheme overrides (`light:`/`dark:` alongside flat keys that
  apply to both), and the project's own local font files inlined as base64 `data:` URLs. The token
  vocabulary grows from 14 to 28 — 14 new tokens (`code-inline-bg`, `code-bg`, `table-head-bg`,
  `image-bg`, `hover-bg`, `border-strong`, `shadow`, `shadow-strong`, `shadow-cast`, `scrim`,
  `selection-bg`, `status-draft`, `status-draft-bg`, `mockup-bg`) cover consumers the original 14
  never reached. See [`docs/theming.md`](docs/theming.md), [FORMAT.md](FORMAT.md#viewertheme), and
  the embedded `dossierx-theme` skill.
- `dossierx theme list` reports every built-in preset and the token names it sets; `dossierx theme
  export <preset> [path]` writes one out as an editable theme file (or returns its YAML in the
  envelope when no path is given), refusing to overwrite an existing file unless `--force` is
  passed.
- A new error code, `unknown_preset`, for a `viewer.theme.preset` name the binary does not carry.
- `dossierx check`'s envelope data gains `theme_font_count` and `theme_font_bytes`, reporting how
  many project-supplied font files a theme inlines and their combined raw byte size — what a
  reader downloads before the page renders anything. Both are `omitempty`: they are absent from
  `data` (not present as `0`) when the theme declares no fonts, and when a render-phase theme
  failure sets `data.theme_error` instead (nothing was accepted). A config-phase theme failure —
  an unknown token, a bad value, a duplicate key, an `extends` outside the project — carries no
  `data` at all; only `error.message` names it.
- Under `dossierx serve`, the Content-Security-Policy now includes `font-src data:`, so a themed
  project's inlined fonts render under `serve` the same way they do in a statically rendered
  viewer. No other CSP directive changed.
- Built with a per-mode/preset/font `viewer.theme` and opened with a DossierX binary older than
  this release, a project's config fails to load rather than being partially understood. The
  error is verbatim one of two shapes: `viewer.theme: unknown theme token "preset" (must be one of
  accent, accent-bg, ink, muted, faint, paper, card-bg, border, link, warn, warn-bg, font-sans,
  font-mono, radius)` for a scalar key (`preset`, `extends`), or a YAML-level `... cannot unmarshal
  !!map into string` (`!!seq` for `fonts:`) for `light:`, `dark:`, and `fonts:`. Failing closed is
  deliberate: a viewer rendered with half a theme applied is worse than one that refuses to build.
- Built-in preset values may change between minor releases, since they track a palette this
  project does not own; every such change from now on gets its own **Changed** entry here. A
  project that needs a value frozen writes it inline, or exports the preset to a file it then
  owns.

### Fixed

- A faint 1px horizontal seam was drawn between every pair of lines of a multi-line fenced code
  block in a claim body, in both colour schemes. The reader saw a hairline in `--border` splitting
  the code block into per-line boxes. The cause: the stylesheet's inline-code pill rule gives every
  `code` element a `1px solid var(--border)` border, and the `.claim-body pre code` reset cancelled
  the pill's background, padding and radius but not that border — and a fenced block's `<code>` is
  a single *inline* box, so it fragments across line boxes and the initial `box-decoration-break:
  slice` repaints the border's top and bottom edge on every fragment. The reset now includes
  `border: 0`. A single-line fenced block was unaffected (one fragment, one ring, drawn flush
  inside the block's own border); a block of *n* lines showed *n*-1 seams.
- A fenced code block written in a step body, a comment, or a list item is now painted as a code
  block, the same as one written in a claim body. Those three bodies were never in the
  fenced-block rule's selector list, so their `<pre>` had no background, no border and no padding
  at all, and the block reached the reader as a run of inline-code *pills*: measured in a browser,
  `pre` computed `background rgba(0,0,0,0)`, `border 0`, `padding 0`, while its `<code>` kept the
  pill's `1px 5px` padding, `--code-inline-bg` background and the sliced border above. They now
  get `--code-bg`, the `1px solid var(--border)` block border and the `13px 14px` padding a claim
  body's fenced block has always had, with the pill reset applied inside. This changes pixels for
  any project that wrote a fence in one of those three places; it is a deliberate consistency fix,
  not a no-op. `.claim-tree-body` is unchanged — the template writes that `<pre>` directly, with no
  inner `<code>`.
- The viewer's `:root` now declares `color-scheme: light dark` instead of only `light`, so a
  reader in OS dark mode gets UA-native dark rendering for the surfaces the stylesheet never drew
  itself. Concretely, in dark mode: native form controls — every `<button>`, `<details>`/
  `<summary>` disclosure markers, and task-list checkboxes — now paint dark instead of a bright
  light-mode chrome showing through; the mobile facet table-of-contents `<select>` and the native
  popup it opens render dark; the comment composer's input and its placeholder text render dark;
  the scrollbar on an overflowing code block tracks the OS theme instead of staying light; and the
  claims graph canvas's backdrop and overscroll area render dark instead of white. A project
  supplying its own stylesheet through `viewer.template_overrides` is unaffected unless that
  stylesheet also sets `color-scheme`.
- Printing the viewer now always uses the light palette, regardless of the reader's OS colour
  scheme at print time and regardless of any `dark:` theme override the project configured. A
  `dark:`-only token — one with no `light:`/flat counterpart at all — previously had no defined
  print behaviour; it now resolves to the engine's light default under print, the same as every
  other token.
- FORMAT.md's "Mode-invariant vs. mode-varying tokens" table was wrong in two ways: it listed
  `accent`, `accent-bg`, `link`, `warn`, and `warn-bg` as mode-invariant, but the shipped
  stylesheet's dark `@media` block has always re-pointed all five; and it had no third category for
  `code-inline-bg`/`code-bg`, whose default is a `color-mix()` of `paper` and `card-bg` rather than
  a fixed colour, so they compute a different value per scheme without being re-declared in the
  dark block at all. The table is corrected to three groups: 14 mode-varying tokens re-declared in
  the dark block (the eleven original colour tokens `ink`, `muted`, `faint`, `paper`, `card-bg`,
  `border`, `accent`, `accent-bg`, `link`, `warn`, `warn-bg` plus `shadow`, `shadow-strong`, and
  `scrim`), 2 derived tokens (`code-inline-bg`, `code-bg`), and 12 mode-invariant tokens, including
  `font-sans`, `font-mono`, and `radius`.

### Changed

- Every colour, shadow, and shape value the generated stylesheet emits is now themeable through a
  `viewer.theme` token — this is the mechanism that makes the theming feature above take effect. A
  project with no theme configured sees identical rendered output; a project that reached into
  `viewer.template_overrides` to restyle a value this list now covers can drop that override in
  favour of a token. Run `dossierx check` to regenerate an existing viewer regardless: the
  generated `viewer/index.html` output changes shape (a `var()` call in place of a literal) even
  where it renders identically.
- This release silently changes rendered output for some existing projects: three fixtures with
  byte-identical inputs to a v0.7.5 baseline — `fixture-basic`, `fixture-graph-demo`, and
  `fixture-portability` — render differently, from the stylesheet's `var()`-token conversion and
  the print `color-scheme: light` fix above. Two new theme fixtures
  (`testdata/fixture-theme-flat`, `testdata/fixture-theme-preset`) are new to this release's
  cross-release golden and are therefore uncompared this time, not silently passing a comparison
  that ran. The `.status-strip` relocation that showed up here against older baselines is a
  v0.7.5-era change already folded into that baseline and no longer appears.
- After upgrading, re-run `dossierx skills export` to refresh `AGENTS.md`/the embedded agent guide
  with the updated `dossierx-theme` skill.

## [0.7.5] - 2026-08-30

### Added

- The generated System Record viewer can collapse its desktop module navigation and active-facet
  table of contents into narrow recovery rails. Each facet also has one control to collapse or
  expand all of its claims, while individual claim disclosures remain available and keep the
  bulk control in sync.

### Changed

- Live lint and lock-ledger issues now appear inline below the affected module header instead of
  floating over the viewer. The notice follows the System Record typography and disclosure
  patterns, lists only findings for the active facet, hides in every unrelated facet or module,
  and restores the same findings when the reader returns. Static generated viewers remain free of
  live status data. These viewer changes alter rendered output for unchanged claims; run
  `dossierx check` after upgrading to regenerate an existing viewer.

## [0.7.4] - 2026-08-29

### Fixed

- The generated System Record viewer now respects an explicit collapse of the active Modules or
  Tracks navigation group. Navigation still opens the group that contains a newly selected item,
  but it no longer reopens the same group immediately after the reader closes it. Large projects
  can therefore collapse Modules and reach Tracks without scrolling through the full module list.
  This changes rendered output for unchanged claims; run `dossierx check` after upgrading to
  regenerate an existing viewer.

## [0.7.3] - 2026-08-29

### Added

- Individual claims in the generated System Record viewer can now be collapsed from their
  title row. The title, status, and comment control remain visible; direct links reopen the
  target claim, and print output always includes the full content. The disclosure arrow stays
  pinned to the far right edge at every viewport, after the comment control. This intentionally
  changes rendered output for unchanged claims; run `dossierx check` after upgrading to
  regenerate an existing viewer.

## [0.7.2] - 2026-08-29

### Fixed

- Claim bodies in the generated System Record viewer now use the full available card width
  instead of wrapping inside a fixed 75-character column on wide screens.

## [0.7.1] - 2026-08-28

### Changed

- **The generated viewer now reads as a responsive System Record instead of a raw claim dump.**
  Navigation is grouped into collapsible Modules and Tracks, the desktop drawer can be resized
  from 220–420px (with keyboard controls and a reset gesture), generation time is shown in the
  reader's local format, and every facet gains an always-available claim table of contents.
  Claim cards use the same typography, spacing, color, and focus system throughout; their
  evidence footer now presents full-width vertical records with human labels such as **Governed
  By**, **Rests On**, **Depended On By**, and **Sources**, removing the old snake-case rows and
  marker-alignment failures. Build Order and Orientation are peer tabs rather than content mixed
  into the claims list. The claims graph uses the same visual system, removes the unused gaps
  rail, gives module groups an intentional blue treatment, and opens with every claim visible;
  module and facet aggregation remain explicit choices in the Granularity control.
- **The website's viewer gallery now shows the shipped design with synthetic public data.** The
  four legacy Curtainly captures are replaced by fresh overview, claim-detail, expanded-evidence,
  and all-claims graph screenshots generated from a 12-claim public example. No customer name,
  private wording, repository path, internal source, or review text is present in the images, and
  the surrounding copy now says exactly that instead of describing blurred private material.

## [0.7.0] - 2026-08-23

### Added

- **A claim can carry the sources behind it, and locking a claim now signs its evidence too**
  (`sources`, issue #49). Each entry is `{ref, kind, title, …}` and a `body` cites it from prose as
  `[1]`, `[2]` — the Perplexity/Wikipedia convention — recognized in prose only, never inside a
  fenced block or an inline code span, and only on a claim that declares `sources`, so a
  source-less claim writing `array[0]` renders exactly as it did before. The two kinds require
  different anchors because they are falsifiable in different ways: `kind: external` requires `url`
  and `accessed_on`, because a page can be rewritten under you and the honest record is what it
  said on the day it was read; `kind: internal` requires `path` and `sha256`, because the engine
  can open the file, so drift is detectable rather than merely possible. An optional `record_id`
  pins one JSONL record by its top-level `"id"` instead of a shared registry's whole file — a
  registry churns for reasons unrelated to any one claim, and whole-file drift findings train
  readers to wave them through. **What this replaces:** a claim could record *which* sources it
  came from (`migrated_from`, one free-text string) but not *what they were*, so a reader had to
  already know which external registry to open before they could check one sentence — and the
  sidecar file that was the workaround is invisible to `check`, to the viewer and to
  `lock-content-drift`, which left the evidence behind a **locked** claim freely rewritable after
  a human approved it. Five lints: `source-shape`, `source-ref-undefined`,
  `source-external-unanchored` and `source-internal-drift` at ERROR — an internal source whose
  hash is missing, whose file cannot be read, or whose content moved is a failure, never a silent
  pass — plus `source-ref-unused` at WARNING, since an uncited entry is clutter and not falsehood.
  `sources` **is** signed by the lock ledger's `LockedClaimHash`, so editing a citation under a
  locked claim is `lock-content-drift` exactly like editing the body; it is deliberately **not**
  part of the dependency-drift `ContentHash`, because correcting a citation does not change what a
  claim promises and must not flip every dependent to `review_pending` — provenance is not
  contract. A claim carrying no `sources` serializes byte-for-byte as it did before the field
  existed, so upgrading an existing project reports drift on nothing. `migrated_from` is unchanged
  and **not** deprecated: it answers what a claim replaced, `sources` answers what backs it.
  In the viewer, a `supports`/`does_not_support` line longer than **three lines** is clamped to
  three with a `show more`/`show less` control — but only where it actually runs past them, which
  is a fact about the rendered box and not about the string, so the page ships every note WHOLE
  with the control hidden and the viewer's script measures and reveals. That direction is the
  guarantee worth knowing: with no script — a printout, a text browser, a reader who blocks it —
  a citation's stated limit is shown in full rather than cut off behind a button that cannot run.
- **`tracks` — a cross-cutting feature axis, and the `track` noun that reads it** (issue #50).
  `module` answers "who guarantees this?" and takes exactly one value per claim, which is the right
  partition for writing and reviewing contracts. It cannot answer "what does the user get, and is
  it finished?": a user-facing feature is assembled from claims across many modules and a module
  serves many features, so that relationship is many-to-many where the schema allowed one. The
  workaround was generating feature documents outside the tool — true by regeneration, but unable
  to reach the lock ledger, `check`, review threads or the claims graph, and a second copy of the
  corpus by construction. A project declares the vocabulary in `project.config.yaml`
  (`tracks: [{id, title, summary}]`) and a claim names tracks as `- {id, role}` with `role` one of
  `owns`/`cites`, defaulting to `cites`. **The invariant that keeps this from being tagging: every
  claim has exactly one owner on each axis** — one `module`, and at most one track it `owns`.
  Everything else is a citation: a reference, never a copy. Owning is what lets a feature's
  trigger, failure behaviour and acceptance criteria — statements belonging to no single module —
  live in the corpus as a lockable claim. Track membership is **not an edge**: `rests_on`,
  `mirrors` and `governed_by` are semantic dependencies and carry cycle lints, and a set has no
  direction to run in a circle, so track membership joins no cycle walk. Five lints: `track-shape`,
  `track-unknown` and `track-multi-owner` at ERROR, `track-empty` and `track-unowned` at WARNING.
  **An explicit non-goal, stated so nobody adds it later by accident:** track membership never
  gates `dossierx claim lock`, and `dossierx track status` only reports — a track is COMPLETE when
  every claim it owns and every claim it cites is locked, and a claim still locks on its own merits
  through `unlock → fix → lock`. In the viewer, tracks are a sidebar group with a page per track
  rendering the assembled document, and the claims graph gains a track filter; a **cited** claim
  renders there as a reference carrying its owning module and lock state, never as an inlined
  duplicate body. `tracks` is `omitempty` and, like `sources`, outside `ContentHash`: adding a claim
  to a track flips no dependent to `review_pending`.
- **The CLI surface is twenty-two leaves under eight nouns**, up from nineteen under seven. The new
  noun is `track`, and it is the only one whose every leaf is read-only: `track list`, `track show
  <id>`, `track status <id>` write no claim, no store and no artifact. A claim joins a track by
  carrying `tracks:` in its own YAML, so membership on a locked claim changes through the ordinary
  approval path and never through a `track` command.

### Changed

- **The website is a two-page memo, and the twelve-section application that was there is
  deleted.** `site/` was a Vite + React + TypeScript build — twelve sections, fifteen components,
  and a 1,509-line content spec that restated the command surface, the error-code reference, the
  claim lifecycle and a terminal transcript for each. All of it was prose about a pre-1.0 tool
  whose direction is not settled, and all of it had to be re-verified against the binary at every
  release. It went stale in front of users anyway: the meta description advertised a 20-command
  CLI from v0.3.0 until v0.5.0 found it — two minor releases after the surface changed — and four
  separate version strings drifted before they were made to derive from one literal. What ships
  now is `site/index.html`, a memo on why the project exists, and `site/releases.html`, the
  release ledger. **Nothing on the memo is counted and nothing on it names a version**, so it has
  no way to go stale; the counts that must be right live in `README.md`, next to the binary that
  settles them. **This is a narrowing of what the site tells a reader, chosen deliberately** —
  `README.md` is the client-facing account now, and the site makes the argument for the project
  and stops.
- **There is no site build, and the publish workflow uploads the tree.** `.github/workflows/deploy-site.yml`
  no longer sets up Node, runs `npm ci` or builds anything; it uploads `site/` to GitHub Pages as
  it stands, and `package.json`, `package-lock.json`, `vite.config.ts` and the three `tsconfig`
  files are gone. **What this buys is not a faster deploy, it is that the reviewed bytes and the
  served bytes are the same bytes.** Every check in this repository — the release gate's site
  agent included — reads files in the worktree, and that reading was evidence about the deployed
  page only for as long as the toolchain that produced the page was the toolchain that published
  it. `TestThePublishWorkflowUploadsTheTreeWithoutBuildingIt` refuses the reintroduction of a
  build step, because a bundler that rewrote, inlined or dropped something would publish a page
  nobody in this repository has ever looked at while every assertion over `site/` stayed green.
- **The release stamp moved, and the current release is now MARKED rather than positioned.** The
  ledger the release driver checks the tag against was the last element of an oldest-first
  TypeScript array; it is now the entry in `site/releases.html` carrying `data-current="true"`.
  The old arrangement needed three expressions in two files to agree about what "last" meant, and
  a prepended entry silently demoted the new release and promoted the previous one on a page that
  went on rendering perfectly. Exactly one entry may be marked — nought is a page that names no
  current release, two is a page that names two — and re-sorting the page cannot change which
  release it names. `CHANGELOG.md`'s newest heading must still agree with it, and the driver
  still refuses to publish while they disagree.
- **The `site` surface is read as files, and `gate/site-text.json` is gone with the build that
  produced it.** `surfaces.yaml` defined the surface as "the RENDERED DOM of a real build ... not
  the component source", so the gate had to be handed an extraction produced by a headless browser
  — the sixth staged artifact, and the one whose freshness could not be recomputed at record time.
  With no build, the file IS the page: the surface is `site/`'s tracked files, the release
  procedure stages five artifacts rather than six, and "verify the thing the user sees" is
  satisfied directly instead of by proxy. Two consequences worth knowing: the site's
  `document_basis` in `gate-cost-model.yaml` is **decided** (`manifest.tracked_files`) where it had
  been undecided because the right bytes were not extractable from the tree at all; and the
  stale-capture guard the extraction's own failure bought is re-aimed at `gate/render-diff.json`,
  the one stamped capture left, rather than deleted with its old subject.
- **Coverage narrowed in two places, recorded rather than absorbed.** The site depicted a
  `reaudit` transcript and an error-code reference, and two audits pinned them against the binary:
  the reaudit success line must carry a claim id, and `write_conflict` must not be described as
  `claim_file_changed`'s meaning. Both were real shipped defects and both assertions now have no
  subject, because no document in this repository depicts either any more. The skills' half of the
  `write_conflict` rule — the one an agent branches on — is still pinned. `tests/docs_site_audit_test.go`
  carries the note saying what stopped being checked and what would restore it.

### Removed

- **The release gate pipeline is gone, and a release is a maintainer's again.** DossierX is alpha,
  and the multi-agent gate cost more to operate than the releases it was guarding were worth. What
  went: `surfaces.yaml` and the thirteen per-surface reading agents; `gate/` in full (the prompts,
  `method.yaml`, `adjudications.json`); the stage-2 and stage-3 fan-out, bundle, fingerprint,
  receipt, evidence and cost-ledger machinery in `cmd/dossierx/gate_*_test.go`;
  `surface.baseline.json` and `gate-cost-model.yaml`; `scripts/gate-agent/` and
  `scripts/gate-stage2/`; and the two Makefile targets the pipeline ran through, `make ci-evidence`
  and `make release-publish`. The release driver went with them: with no gate receipt to check, its
  first clause could never be satisfied, so it was a program that could only refuse.
  `docs/RELEASING.md` is now a two-page procedure a person works through, and it is still the only
  description of how this project releases.
- **What did NOT go, because none of it depended on the pipeline.** `make test`, `make hook-test`,
  `make viewer-test` and `make viewer-lint` are unchanged and are what a release is now read
  against. `surface.json` is still emitted and still goes red when it is stale, so a command, flag
  or exit code that moved without a CHANGELOG entry is still a failed build. The pre-commit hook
  and the lock-ledger gate are product features and are untouched.
- **`.github/workflows/release.yml` no longer refuses anything.** Its `gate` job checked that the
  tagged commit was a merge and that the tree stamped the version being tagged; both went with the
  rest of the pipeline. A `v*` tag now runs GoReleaser and GoReleaser publishes, which is what this
  file did before the gate existed. The three mistakes that leaves uncaught — tagging the release
  branch instead of the merge, tagging a tree whose site announces a different release, and tagging
  the wrong commit — are named in the workflow's own header and asked for by hand in
  `docs/RELEASING.md`. One guard survives as a test rather than as a job:
  `tests/ci_workflow_test.go` still refuses a `merge-base --is-ancestor` check in that workflow by
  name, because the tag is pushed before `main` and such a check deadlocks against that order — it
  stopped v0.5.1 with a public tag and no archives.

### Fixed

- **`dossierx check` and the three `build-order` leaves refuse a positional argument** instead of
  discarding it (#47). Without an `Args` declaration cobra falls back to `legacyArgs`, which accepts
  any positional on a leaf command and throws it away, so `dossierx check --validate <claim-id>`
  linted the whole project and reported an unrelated module's lint error as though it were about the
  claim that was named. There is no per-claim check to narrow to, so the honest answer is a usage
  error. This moves those invocations from exit 0 to exit 1; the repository-internal sweep is clean,
  so the exposure is to callers outside this tree.
- **`docs/RELEASING.md` pushes with fully qualified refspecs** (#40). A release developed on a
  branch named `vX.Y.Z` gives that name two referents, and the short forms resolve it by search
  order rather than by intent: `git push origin vX.Y.Z` refuses outright, and `git rev-parse
  --short vX.Y.Z` warns and then answers from `refs/tags` by a convention nothing enforces. The
  procedure now writes `refs/tags/…` and `refs/heads/…`, which have exactly one referent each.

## [0.6.0] - 2026-08-20

**SILENT: three shipped skill guides changed, and the skills are embedded in the binary — anyone
who has run `dossierx skills export` must re-run it after upgrading, or their agents keep
following procedures that do not work:** a review loop that wedges on a refusal whose recovery is
the step scheduled after it, a bootstrap that ends silently uninstructed, a bootstrap whose
yes-to-the-hook answer ends with no CI gate at all, and a build-order recovery whose advice fails
every claim it touches at the next `check`. In all four the engine was right and the guide was
wrong — no command, flag, `error.code`, lint rule or gate behaves differently. The corrections
are the first four items under **Changed**.

### Added

- **The release driver requires the forge to restrict who can create release tags.** The gate is
  enforced by files inside the repository being gated, so anyone with push rights could weaken it
  and tag that commit — a residual `docs/RELEASING.md` could previously only record. The driver
  now reads the rulesets off GitHub's API and refuses the release unless an ACTIVE tag ruleset
  covers the exact tag and restricts both `creation` and `update`. A check that cannot run — no
  `gh`, no token, a refused token, an uninterpretable ruleset pattern — is a refusal naming the
  problem, never a skip; its stated limits and the required configuration are in `docs/RELEASING.md`.
- **Every published release page points at the CHANGELOG**: two hand-written literal blocks in
  `.goreleaser.yaml` — a header saying changes your own tooling cannot detect are called out at
  the top of the release's CHANGELOG entry, a footer linking `CHANGELOG.md` on `main` — bracket
  the generated commit-subject bullets, where a breaking `feat` reads like any other. Both are
  deliberately template-free — a template there first renders after the tag is public — and the
  release-notes predictor refuses the templated form of either.
- A markdown case pinning `mailto:`, the only allowlisted non-http(s) scheme
  (`testdata/markdown-cases/link-mailto.{yaml,golden.html}`) — what notices if `mailto:` ever
  silently leaves the renderer's allowlist.

### Changed

The four skill-guide corrections behind the callout above:

- **The review loop no longer schedules a lock the lock gate refuses**
  (`skills/dossierx-comments/SKILL.md`). Step 4 sent a locked claim through unlock → fix → lock
  before replying — but the human's thread is still open then, exactly what `claim lock` refuses
  (`unresolved_comments`). Step 4 now ends in the reply; the lock lands at step 7, after Resolve.
- **The bootstrap creates the project before exporting the skills** (`skills/dossierx/SKILL.md`).
  The export resolves its root from `project.config.yaml`, which the old ordering had not written
  yet — rootless, it exits 0, maintains no `AGENTS.md` section, drops the agent guide in the
  wrong place, and nothing later exported again. Steps 2 and 3 are swapped.
- **The bootstrap installs the CI workflow on both answers to the hook question**
  (`skills/dossierx/SKILL.md`). Step 4 fetched `scripts/ci/dossierx-check.yml` only when the human
  *declined* the pre-commit hook — the identical defect this release fixes in README's paste
  block (under **Fixed** below), standing uncorrected in the procedure's other home, so the
  nudged answer — yes — ended the bootstrap with only the local gate git skips on merges,
  rebases, cherry-picks and reverts, and that `--no-verify` bypasses. The hook question now
  decides the hook alone, both answers end with the workflow installed, and "CI is the authority
  either way" governs both branches from the step's shared preface.
- **The build-order recovery for a missing `build_role` routes through the approval path**
  (`skills/dossierx-build-order/SKILL.md`). "Set it, then re-propose" could only mean hand-editing
  a locked file — no verb sets `build_role` after creation — and the next `check` failed
  `integrity_failed` / `lock-content-drift` on every claim so edited. The row now reads unlock
  (with the human's `--reason`) → set while draft → lock → re-propose; the same-phase `rests_on`
  cycle row routes the same way.
- `docs/RELEASING.md`'s evidence staging matches the stage-2 gate: site-text extraction requires
  `DOSSIERX_SITE_TEXT_TREE` beside `DOSSIERX_SITE_TEXT_OUT` (a stale capture is refused, not
  hashed cleanly into a key); `delta` is documented without `--tree` (still accepted, unused);
  and the SHARED evidence files are three, not four — `gate/site-text.json` is the `site`
  surface's own capture, so a site change re-keys one surface, not thirteen. The pin paragraph's
  counts are now derived from `surface.json` and test-pinned; the hand-list had gone stale twice.
- **The stage-2 baseline is derived from the previous release itself, never handed in as a
  file.** `scripts/gate-stage2/run.sh delta` now reads the baseline inventory out of
  `--baseline-commit`'s own tree (`git show <commit>:surface.json`; the frozen v0.5.0 commit —
  the one release with no `surface.json` of its own — resolves to the committed
  `surface.baseline.json`, chosen by identity and never as a fallback for a failed read) and
  refuses the retired `--baseline-file` flag by name; `record` derives the same bytes again and
  refuses a `gate/baseline.json` holding anything else. What forced it: the first v0.6.0 gate
  run computed its delta against v0.5.0's frozen inventory while recording baseline ref v0.5.1 —
  thirteen reading agents were handed a two-release comparison as this release's, and every
  digest in that run's manifest was honest — because `docs/RELEASING.md`'s staging block
  hard-coded `--baseline-file "$ROOT/surface.baseline.json"`, an invocation that was right for
  exactly one release. The staging block now passes only the ref and the commit.
- `docs/RELEASING.md`'s opening describes the forge gate that actually ships: the tagged commit
  must be a merge and its tree must carry the release stamp. The old text still promised the
  `origin/main` reachability check that was replaced after it deadlocked the v0.5.1 release — a
  maintainer reading it would conclude a locally created, never-pushed merge cannot be
  published, and take no care over exactly the case nothing refuses.
- The viewer templates say the truth about themselves: `graph.css` no longer describes a backdrop
  dim and drop shadow the opaque pane does not have, z-index band 80 is named as the pane root it
  is, the zero-thread comment chip is dated v0.3.0 (not v0.2.1), and `comments.html` names
  `components.CommentChipHTML` rather than a hand-built chip gone since v0.4.1. The fixture
  viewers are regenerated; nothing a reader sees changes.

### Fixed

In the documents client teams follow, each a procedure that failed a reader following it as written:

- **README's setup paste block installs the CI workflow on both answers to the hook question.**
  It fetched the workflow only when the human *declined* the pre-commit hook, so the nudged
  answer — yes — ended setup with only the local, skippable gate.
- **README's setup paste block creates the project before exporting the skills.** Step 2 ran
  `dossierx skills export .claude/skills` before step 3 had written `project.config.yaml` — the
  identical defect this release fixes in the router skill's bootstrap (under **Changed** above),
  standing uncorrected in the procedure's other home. Rootless, the export exits 0, maintains no
  section in an existing `AGENTS.md`, drops the agent guide beside the bundles instead of at
  `docs/`, and nothing later in the block exports again — a harness that reads `AGENTS.md` was
  never taught DossierX at all. Steps 2 and 3 are swapped, and the two documents now give the
  bootstrap in the same order.
- **README says to commit the comment digest store with the first lock, not "once anyone
  comments".** The engine creates `.dossierx-comment-digest.json` empty at the first lock; a
  reader who waited staged the lock store without it and was stopped by the hook's own
  `check --staged` with `comment-digest-absent`.
- **The pre-ledger crossing's step 4 is conditional on the module being fully locked again** — in
  README, FORMAT.md, the CI template's recovery, and the `pre_ledger_unadopted` hint. The recipe
  licensed re-locking "only what you still stand behind", then handed out an unconditional
  `build-order propose` + `lock` that refuses the moment that license is exercised. A partially
  re-locked module now finishes the crossing gate-green and runs the pair when fully locked.

In the pre-commit hook and its installer — the hook body moves to v8, so re-run the installer to
replace an installed v7:

- **The hook no longer refuses every commit on a path containing `"`, `\` or a tab.** Discovery
  trusted `-c core.quotepath=false` for raw paths, but git C-quotes those characters
  unconditionally: the quoted string named no file on disk, and a discovered config the hook
  cannot open is a refusal — every commit under such a path was refused until somebody
  uninstalled the hook. Discovery now uses `-z`, the one output mode git never quotes (a newline
  in a path still fails closed, and says so), and the foreign-hook refusal's remediation lines go
  through `sh_quote` so they re-parse when executed.
- **Printed recoveries name the invocation the reader actually used.** Every "run it again" line
  said `scripts/install-git-hook.sh` — a path only this repository has, offered to readers who
  curl'd one file into their own project. The script now prints its own `$0`, and the PowerShell
  wrapper passes its name through `DOSSIERX_HOOK_INVOCATION` so its readers get a PowerShell line.
- **A machine-wide hook install is said out loud.** A `core.hooksPath` from the global or system
  git config makes the hook fire on every commit on the whole machine; the installer used to name
  only the setting, not its reach. It now asks git for the setting's origin (submodules and
  linked worktrees classify as the repository's own), states the reach, names the config file,
  and prints the matching uninstall line. The disclosure accompanies the install, never vetoes it.
- **That disclosure names the config file by its real path, not git's C-quoted rendering of it.**
  `git config --show-origin` C-quotes an origin containing `"` or `\` unconditionally — on
  Windows, where the origin is a native absolute path, that is every origin — so the disclosure
  named `"C:\\Users\\...\\gitconfig"`, a string that exists nowhere on disk, as the one fact meant
  to let the reader verify or undo the setting. The origin is re-read with `--null`, the output
  mode git never quotes, converted outside command substitution (which cannot carry NUL and would
  glue the value onto the path); a newline in the config's own path still classifies loudly as
  machine-wide rather than silently as anything.
- **The PowerShell wrapper's `Find-Bash` rejects WSL's launcher.** A `bash` under
  `%SystemRoot%\System32` resolves a `C:\` script path inside the Linux filesystem: the install
  died "No such file or directory", and because a bash HAD been found, neither remedy printed.
  The wrapper falls through to the Git for Windows candidates, and now runs under Pester on
  `windows-latest` — it had shipped for releases while no CI job ever started pwsh.
- **The wrapper's no-bash remedy hands WSL a path WSL can open.** The message whose first
  sentence explains that WSL's bash cannot run a script on a `C:\` path went on to offer exactly
  that command — `bash "C:\...\install-git-hook.sh" --yes`, the path resolved on the Windows
  side. The WSL line now translates it with `wslpath` inside the WSL invocation, where the
  distro's mounts are known; whether a distro mounts that drive at all is more than the wrapper
  can check from Windows, and the message says so instead of implying the line always works.
- **`--help` no longer stops mid-sentence.** A sed range ends AT its closing match, so the last
  thing a reader saw was `1 declined, refused,`. Extraction now closes on an explicit
  `# END USAGE` sentinel, and the usage line swaps in the reader's own invocation by literal
  index — never sed replacement or awk `-v`, which mangle exactly the `C:\...` paths at stake.

In the engine — **the claims file lock could refuse the one case it exists for, on Windows** —
the fix to have if two `dossierx` invocations can touch one project at once. `AcquireFileLock`
retried its `O_EXCL` sentinel only on "already exists", but Windows keeps a deleted file's
directory entry in a delete-pending state that fails opens with `ERROR_ACCESS_DENIED` — beginning
at the exact instant the holder releases, which is the instant a waiter polls — so the waiter
returned `Access is denied.` at the one moment it should have polled once more. The
classification is pinned per-platform (POSIX `unlink` is atomic, so `EACCES` there fails fast as
a real permission problem), and the two contended timeouts now report apart.

In the release pipeline:

- **The release workflow's tag gate no longer deadlocks against the release driver.** The gate
  required the tagged commit to be reachable from `origin/main`, but the driver pushes the tag
  first and `main` last — deliberately, so the site never announces archives that do not exist
  yet — so the gate refused at the tag push and the driver waited for archives that could never
  exist; v0.5.1 was finished by hand this way. The gate now asks what the tag can answer alone:
  the tagged commit is a merge (the driver always tags its `--no-ff` merge) whose tree stamps
  exactly this release; the trade is stated in `release.yml`'s header, pinned by test.
- **Both install paths print one version string, the tag as tagged** (#38). The archive stamped
  `{{.Version}}` — the tag minus its leading `v` — while `go install` falls back to
  `debug.ReadBuildInfo`: one release, two answers (`0.5.1` and `v0.5.1`), so a scripted
  version-against-tag comparison succeeded one way and failed the other. The stamp is now
  `{{.Tag}}`, and the site's second constant (`latestBinaryVersion`) is deleted, any
  strip-the-`v` successor refused by test.

And the smaller corrections:

- **`dossierx lock|unlock|flag|reaudit` answer with their replacement, not a bare usage error.**
  The four verbs most likely typed from pre-v0.3.0 memory — each lives on at
  `dossierx claim <verb>` — answered `unknown command "lock"`, because cobra's `legacyArgs`
  rejects unknown commands before the root's hint-bearing branch runs. Each now refuses with
  `error.code` `usage` and a `run:` hint that actually runs — `run: dossierx claim lock <id>
  --reason "..."` — and the site's migration table gains the row naming all four.
- **A comment sweep: what the code says about itself matches what it does** — no behaviour
  changes. Nine files said `dossierx flag` for what has been `dossierx claim flag` since v0.3.0;
  `MockupModules`' doc (and `structured_layout`'s) kept v0.4.0's mockup-only framing after v0.4.1
  widened the `raw_html` gate to any layout; `already_locked`'s doc gains its second state (a
  draft claim with a standing approval is tampered content — restore it first); `claim new
  --help` no longer says every claim is a card; and two stale comments — a roll-up gate that
  moved to `evaluateLockGates`, and a `prepareStore` grandfathering claim that read literally
  meant a reaudit can bless content nobody approved — now say what the code does.
- **Two test guards advertised more than they checked** (#29, #28). The summary dash guard
  promised to refuse both the en and the em dash and checked one; it is now a check on the test
  data, run before the exact match it protects, naming both. The `</details>` ordering probe
  proved nothing the exact-adjacency match beside it does not — deleted rather than anchored.
- **A serve test raced the server it started, and only Windows noticed.** The watcher's re-walk
  holds a directory handle Windows refuses to unlink under, reddening
  `TestClaimAsset_SymlinkedDirectoryIsRefused` and, through it, CI on `main` — a refused release.
  The removal now retries in a bounded window and then **fails**: giving up quietly would leave
  the assertion passing against an ordinary missing file. Mutation-checked.

## [0.5.1] - 2026-08-10

**SILENT: the embedded agent skills changed, and nothing on your side reports it.
Re-run `dossierx skills export` after upgrading.** Those bundles are written into a project as
committed artifacts, and nothing in `dossierx check` compares an exported copy against the binary's
— v0.5.0's entry below says so in as many words. A project that skips the re-export keeps v0.5.0's
guidance, including an install line that fetches `scripts/install-git-hook.sh` from the v0.5.0 raw
path and a stale account of when `dossierx claim flag` is refused.

**Nothing a consumer runs behaves differently.** There is no new or changed command, flag,
`error.code`, lint rule, schema field or rendered-viewer byte, and no engine behaviour moves:
`internal/` is untouched end to end and there is nothing to re-render. What moved outside the
release machinery is the four install pins, now `v0.5.1` — one of them inside
`skills/dossierx/SKILL.md`, which is the byte that makes the exported bundle a different bundle —
and seven wrong strings in what a consumer's own tooling prints or ships: three from the binary,
two carried by the exported skill bundles, and two from the pre-commit hook installer and the hook
it writes.

The three the binary printed each named an invocation it rejects. The retired `implink` stub's
replacement command omitted `--module` and `--claim` and passed the id positionally at a command
declared `cobra.NoArgs`. The missing-`--reason` refusal printed the verb without the id or
`--module` the verb also requires. And `dossierx claim show`'s next action for a drifted
implementation link offered a bare `dossierx claim flag <id>` at a verb that requires
`--claim-says`, `--now-does` and `--reason`, all three, before it does anything at all. All three
now print the whole invocation; `claim show`'s is the one whose own doc comment already promised
"the advice can never disagree with what the command would do", which the other two are only held
to by `internal/cliout`'s definition of a hint.

The two in the bundles: the cross-references between bundles were written as `[[wikilink]]`, which
the two derived export forms rewrote into an anchor and the `SKILL.md` tree — the form Claude Code
actually loads — shipped to a client's agent as the literal characters `[[` and `]]`. Each is now
an ordinary relative link to the sibling bundle, which resolves as written in the exported tree and
is still retargeted to an anchor in the guide and in the `AGENTS.md` section. And the code-links
bundle's account of `dossierx claim flag` still stated v0.4.0's layout rule after v0.4.1 made the
refusal key on content.

The two in the hook: the installer's note used to open "this repository sets
core.hooksPath", when the value is read with a plain `git config --get`, which resolves across every
scope: a `git config --global core.hooksPath ~/.githooks` is an ordinary setup, and that reader was
being sent to look for the setting in a `.git/config` that never mentions it. The note now states
the value, says the setting may be the repository's or the global one, and hands over
`git config --show-origin --get core.hooksPath`, which answers it. Separately, the hook body's
"remove the hook" recovery — printed on two different refusal paths — named
`scripts/install-git-hook.sh --uninstall`, a path that exists in this repository and in no
consumer's: the installer is deliberately one file with the hook embedded so it can be fetched into
a project that has the binary and not this repository, which is the ordinary case and precisely the
reader being refused. It now names the hook where git will actually look for it, resolved by git at
the moment the line is run, so it is right under `core.hooksPath` and in a linked worktree too. The
hook body's version marker moves to v7 with it: re-running the installer replaces an installed v6
rather than reporting it current.

Four non-test Go files move for the binary's three and the bundles' two —
`cmd/dossierx/retired.go`, `cmd/dossierx/output.go`, `cmd/dossierx/claim.go` and
`cmd/dossierx/skills_embed.go` — and none of them changes anything but the text a reader is handed.
The hook's two are shell, in `scripts/install-git-hook.sh`.

What this release is otherwise is the machinery that publishes the next one. Everything that has ever
gone wrong with a DossierX release has been in the half a maintainer performs by hand — a version
string copied into prose and left behind, a verification step that read the source instead of the
artifact, a `commit` field that named the wrong sha for two releases running — and every one of
those was a promise to look rather than something that fails when it is not true. Each item in
`docs/RELEASING.md` now has a check behind it, the release itself is performed by a program rather
than by a person following a list, and the parts that genuinely cannot be checked from inside this
repository are named as such instead of being quietly assumed.

### Added — `surfaces.yaml`, and one reading agent per surface

`surfaces.yaml` declares every client-facing surface this project has — thirteen of them, from
`README.md` to the compiled binary — and, beside them, seven declarations of what is deliberately
out of scope and why. `tests/surfaces_manifest_test.go` requires every tracked file to be claimed by
**exactly one** entry: a file matching nothing fails the build, and an out-of-scope entry cannot
quietly swallow a path a surface also claims, because the test names both and fails. Before this,
the list of things to review lived in a scope document, so a new client-facing file could appear
with nothing to notice it and the only way to find the gap was an audit.

At release time each surface is read by its own agent against a bundle assembled for it — the
prompt (`gate/prompts/<surface>.md`), the surface's own files, and the extracted evidence its
questions need. The bundle is fingerprinted, and the cache key is the digest of what the agent was
**actually handed**, not the surface's name: change a byte in the evidence and that surface is
re-read rather than carried forward. `gate/method.yaml` grants the agent exactly two tools, both
report-only (`SurfaceFinding`, `SurfaceVerdict`), as an exclusive allow-list rather than a deny
list — there is no file, shell, search, network or subagent tool, because "the bundle is the whole
evidence set" is the property every key in the system rests on. What that file cannot promise, and
says so, is that the harness outside this repository honoured the request.

Findings are never filtered, deduplicated or ranked away on their route to the report, and a receipt
carrying any finding at all evaluates to FAILED. A finding's `severity` is free text the reporting
agent wrote about its own work: one sort comparator reads it so that a re-run over an unchanged tree
produces an identical document, and no verdict, filter or threshold consults it anywhere.

### Added — `make release-publish`, the only thing in this repository that tags

The irreversible half of a release is now a nine-step driver rather than a sequence of commands a
person types. It is authorized by the version typed twice —
`make release-publish DOSSIERX_RELEASE_VERSION=vX.Y.Z DOSSIERX_RELEASE_AUTHORIZE=vX.Y.Z` — and
deliberately not by a boolean, because a `=1` left in a shell profile or a CI secret authorizes
every release forever, including the next one somebody triggers by accident.

Before it touches git it establishes, in this order: that no part of this release is already
published; that **the tree declares the release being tagged** — `CHANGELOG.md`'s newest heading and
`site/src/content.ts`'s last `releases[]` entry must agree with each other and with what the human
typed, which is the one question content-matching cannot answer, since a self-consistent tree tagged
as some other release passes every other check; that the gate is green, **recomputed in this
process** rather than read out of a record, because "no findings" cannot stand in for six separate
refusals about coverage; and that the CI-run evidence for this exact commit exists. Then it merges
`--no-ff`, reads the merge commit once and uses that value everywhere after, tags the named object,
reads the tag back through its ref and re-checks the tree it points at, pushes the tag **by value**,
verifies the published archives, and only then pushes `main`.

A run that stops leaves a state a human can read: the step it stopped at, what had already been
published, and what had not. It never proposes a retry it cannot perform.

### Added — the published archives are verified before `main` moves

Between the two irreversible acts, the driver reads the artifacts the way somebody downloading them
does. It polls the forge until the Release workflow's assets exist — the ordinary state one second
after a tag push is "the tag is there and the assets are not", so waiting happens inside the step
rather than as advice to run the command again, which there is no way to do once the tag is public.
Then: a missing `checksums.txt` is UNCHECKABLE and never "no mismatches found"; the expected archive
names are derived from `.goreleaser.yaml` at the released commit — the build matrix, the
`name_template` and the format overrides — so the day a seventh target is added this check grows
with it instead of counting six and passing; every archive's sha256 is compared against its line
**and** every line against an archive that was actually read; and the host platform's archive is
extracted and its binary **run**, because an archive can carry a correct name, a correct checksum
and a stale binary while every metadata check passes over it.

### Added — the forge refuses an ungated tag

`.github/workflows/release.yml` used to be `on: push: tags: ['v*']` and, one job later, six archives
and a GitHub release, with no condition of any kind between the two. A new `gate` job now runs first
and the publishing job `needs:` it, so a tag that does not get past it produces no archives at all.
Two facts have to hold, both about the tagged tree rather than about whoever pushed: the tagged
commit is reachable from `origin/main`, and the tree at that commit carries the release stamp for
exactly this version. Every exit path that is not a pass is a refusal — there is deliberately no
branch that reports "could not check" and exits 0.

One residual is recorded in that file rather than described as fixed: the workflow GitHub runs for a
tag is the one in the tagged tree, so anyone with push rights can weaken this job and tag that
commit. Nothing inside this repository closes that — a check cannot be its own enforcement — and
only a forge-side tag protection rule can.

### Added — `make ci-evidence`: the run's own account, not the run's conclusion

A green badge is not the check, and neither is a green check run: a conclusion is `success` over
zero tests, so a suite emptied by a `-run` selector prints `ok [no tests to run]` for every package
and the step, the job and the check run all conclude success over it. `make ci-evidence` fetches the
CI run for a named merge commit and adjudicates the `go test -json` account the test binary itself
emitted — per package, per test, per matrix cell — against the job set derived from `ci.yml`. No
conclusion is read as evidence anywhere; conclusions are recorded in the verdict record for a human
to look at and are adjudicated by nothing. The record is required to exist and to name the commit
being released: a release nobody ran this for is refused, not assumed.

### Added — `surface.json`, and v0.5.0's inventory frozen as the baseline

`surface.json` is the machine-readable inventory of what this tree exposes — 19 commands under 7
nouns, 3 root flags, 12 retired spellings, 28 lint rules, 44 error codes, 5 skills, 14 HTTP routes,
129 markdown constructs, a render fingerprint, a per-package behaviour fingerprint, the JSON
envelope's keys and exit codes, and the version pins — extracted from the tree by a test and
regenerated, never written by hand. It is what prose gets judged against, which is what turns "the
README says twenty commands" from a reviewer's memory into a comparison.

`surface.baseline.json` freezes v0.5.0's inventory, because v0.5.0 shipped before the emitter existed
and carries no `surface.json` of its own. It is the only record of what that release's surface was,
so the first gated release has a real predecessor to be diffed against.
`testdata/render-across-releases.golden.txt` does the same job for rendered output: the class of
change a consumer's own gate cannot detect for them is exactly the class this project has shipped
three times, and it is now compared release to release rather than noticed.

### Changed — the published release notes no longer carry the merge commit's subject

`.goreleaser.yaml`'s `changelog.filters.exclude` gains `^Merge `. The GitHub release body is
generated from Conventional Commit subjects at tag time, and a `--no-ff` merge's own subject matches
neither `^chore:` nor `^docs:`, so the catch-all "Other changes" group swallowed it — v0.5.0's range
carries exactly one such subject, `eab3a63`, "Merge pull request #32 — v0.5.0, a claims graph in the
viewer". It also made the notes unpredictable by construction: the pre-merge prediction runs before
the merge commit exists, so it could never have seen the line the published page would carry. Both
are closed by the one exclude, proved against a real `git merge --no-ff` in a from-scratch
repository rather than against this project's own history, with the pre-fix config kept as the
negative control so the scenario is shown to be real and not hypothetical.

The release notes are a declared surface in their own right, and `.goreleaser.yaml` is its only
path: the notes themselves do not exist until the tag, so the rules that decide what they say are
what gets reviewed. The reason that surface exists is a shape nothing audited before — a
user-visible change landing under a `docs:` or `chore:` subject is dropped by the filters and is
invisible on the release page while being fully described in this file.

### Changed — one release procedure, and the encoded second one is retired

`.claude/workflows/release-checklist.js` — 447 lines that offered themselves to every agent in this
repository as a runnable release procedure under their own name — is deleted, and the deletion is
pinned. That distinction is the whole of it: restoring the file verbatim used to leave every test in
this repository green, because nothing had ever read that directory, so the deletion was a fact
about one commit rather than an invariant about the tree. `tests/ci_run_evidence_test.go` now parses
every workflow declaration under `.claude/workflows/` and refuses any that declares itself a release
procedure; a file it cannot parse is a failure and not a pass, because an unexamined corner of the
directory the harness loads is exactly where a second procedure would sit unnoticed.

`docs/RELEASING.md` is the single description of how this project releases, and it is now read by
tests as well as by people — its pin sweep, the ordering of its tagging steps, and the three checks
it keeps a person's are all held against the driver that performs them.

### Changed — CI reports what it ran

The test job checks out at `fetch-depth: 0`, because a checkout with no tags cannot resolve a
release baseline and would otherwise take the "no tag yet" branch of every date and cross-release
comparison and pass. `go test -race ./...` becomes `go test -race -json ./...`, which is what makes
the run's per-test account exist at all — and it stays ONE command, spelled entirely out of the
closed vocabulary that keeps `|| true`, `set +e` and `| tee` out of a suite step. The viewer job
installs a pinned GoReleaser (v2.17.1) and fails rather than skips when the binary is not there, so
the release build is watched doing its job instead of being read for what it was told to do; the
site is built with a real toolchain so the browser suite reads rendered DOM rather than source.

### Fixed — the site advertised a twenty-command CLI

`site/index.html`'s `<meta name="description">` had claimed "a 20-command JSON CLI" since v0.3.0.
Twenty was the v0.3.0 surface; v0.4.0 cut it to seven nouns and nineteen leaves, and the tag stayed
wrong through two minor releases. It survived because it is the one count on the site that nothing
derives — every other version and count comes from `latestRelease` / `latestVersion` /
`commandCount`, deliberately, after three of them went stale once before — and `index.html` is
static HTML that cannot interpolate. It is also the string search engines and link previews quote,
which is the least likely place for anyone editing the site to look. The guard walks the real
command tree rather than pinning a second literal, so changing the surface fails the build until the
site follows; a phrasing that carries no count at all is not held to one, since a sentence without a
number cannot go stale.

### Fixed — the `dossierx version` transcript, and the hand-stamped release sha

The site depicted `dossierx v0.5.0` where the published binary prints `dossierx version 0.5.0` —
two errors in one short line, and nothing compared it to real output. GoReleaser's `{{.Version}}` is
the tag with its leading `v` stripped, so the release spelling and the transcript spelling are
different strings for good reasons; the transcript now derives from the release entry with the `v`
removed, and it is checked against a binary linked the way a release links one, read out of the
**rendered page** rather than out of the source.

The `commit` field is deleted from every release entry, along with the step that wrote it, the
fallback that rendered it and its type declaration. It could not converge — writing the sha is
itself a commit, so the value was stale the moment it landed — and it named the wrong sha twice
running: v0.4.1 shipped naming `5327923` while `refs/tags/v0.4.1` points at `206b4a4`. It also
disagreed with the binary by construction, seven characters against the forty GoReleaser stamps into
`main.commit`. The optional type declaration outlived the data, the reader and the release step,
which is what would have let the field come back silently: `commit: "abc1234"` on a new entry would
have type-checked, and the compiler was the only thing that would have objected.

**Not in this release, and stated so deliberately.** Nothing derives a finding's classification from
the evidence behind it, and there is no override field on a receipt — a finding a human has judged
non-blocking can be cleared only by fixing the tree or by deleting the finding by hand, and deleting
it leaves an adjudicated finding indistinguishable from one nobody raised. Why neither was built, and
what each would need first, is recorded in the tests rather than left to be rediscovered. Nor does
anything here verify the deployed site, the workflow run or the CDN: those are the three checks the
driver hands to a person at the end, and it says in those words that it examined none of them.

## [0.5.0] - 2026-08-07

**BREAKING: `dossierx check` now fails on a dependency loop that alternates `rests_on` and
`governed_by`, and no migration path accompanies it.** The new `mixed-cycle` lint runs at ERROR
severity, taking the registered rule count from 27 to 28. It walks the union of the `rests_on`
and `governed_by` graphs with the edge kind carried on every hop, and reports a cycle whose hops
include at least one of each — "A rests_on B, B governed_by A". Neither existing cycle rule can
see that shape: `cycle` walks `rests_on` alone and `governed-cycle` walks `governed_by` alone, so
a mixed loop presents no back edge to either walk and passed the entire registry. A project
carrying one passed `dossierx check` before this release and exits 1 after it, with no edit on
its side, no content-hash move and nothing in `.dossierx-lock-store.json` to explain the change.

**Re-run `dossierx skills export` after upgrading.** Upgrading the binary does not touch skills
already exported into a project — they are plain files in your repo, and nothing in `dossierx check`
reports that they are a version behind. A project that upgrades without re-exporting keeps the
v0.4.1 router, which has no `mixed-cycle` section, so its agent meets this refusal on a corpus it did
not touch, hunts for what it broke, finds nothing, and loops. That is the exact failure the section
below exists to prevent.

There is deliberately no migration command and no migration document: a corpus containing this
shape was always malformed, the engine simply could not see it. The recovery is to break the loop
— the finding names every claim on it — and re-run `dossierx check`. Where those claims are
locked, that is `dossierx claim unlock`, the edit, then `dossierx claim lock`, the same as any
other correction to a locked claim. `mirrors` is not part of the union graph and never trips this
rule; a pure `rests_on` or pure `governed_by` loop still reports as `cycle` or `governed-cycle`
and this rule stays silent on it.

**Every project's rendered viewer changes on its next render, and roughly triples in size.** The
basic fixture goes from 108,577 to 348,496 bytes for the same `dossierx check`. The pane is rendered
unconditionally and there is no config opt-out. Your own gate cannot tell you this: no claim's
content hash moves, no `review_pending` flips, and `dossierx check` reports exactly what it reported
before — the artifact you ship is simply a different, larger file. No claim's own markup changed, so
this is additive chrome rather than v0.4.1's shape where locked bodies re-rendered. Re-run the render
to pick it up, and see the graph-pane section below.

### Added — a claims graph pane in the viewer

The rendered viewer now carries a "Claims graph" pane: a canvas view of the corpus's `rests_on`,
`governed_by` and `mirrors` edges, with selectable overlays (isolated & weakly linked, dependency
cycles, governance, review pending, open comment threads, draft vs locked), a per-claim detail
panel that includes an `in a cycle` row, granularity collapse to modules or facets, zoom, pan and
drag. Above 300 claims the pane opens at module granularity rather than drawing every claim.

It is built by a new `internal/graph` package as a JSON payload inlined into the single
self-contained `index.html`, alongside three new embedded client files (`graph-core.js`,
`graph-ui.js`, `graph.css`). No external assets, so it works over `file://`. There is no new CLI
noun and no new schema field.

The viewer's size and re-render consequence are stated in the preamble above, since a consumer's own
gate cannot report either. The pane is a third full-viewport overlay with its own body scroll
lock (`body.dxg-open`) that is additive with the sidebar drawer's and the comment panel's rather
than mutually exclusive with them.

### Added — `GET /api/graph` under `dossierx serve`

Backs a refresh button in the pane, rebuilding and re-stamping the payload from the current
catalog at request time rather than reusing the render's. The button is absent over `file://`,
where there is nothing to refresh from.

### Added — `testdata/fixture-graph-demo`, a third committed sample viewer

A 58-claim fixture project with a tracked `viewer/index.html`. It is the first fixture in this
repo that is itself a ledger-covered dossierx project, so its `.dossierx-comment-digest.json` is
a tracked input (`.gitignore` gained one negation line for exactly that path) alongside its
`.dossierx-lock-store.json`. Without the digest, a fresh clone or CI checkout would fail the
fixture on `comment-digest-absent`. `docs/RELEASING.md` now names three sample viewers to
regenerate, not two.

### Added — `tests/fixture_staleness_test.go`

Fails the build when a committed sample viewer no longer matches what the current renderer
produces, instead of leaving a stale generated artifact to be noticed at release time.

### Changed — the offline scan strips comments before matching

`tests/portability_test.go`'s check that no shipped `.js` reaches the network now strips `//` and
`/* */` comments from the file before matching, so a comment that names an endpoint — `graph-ui.js`
documents its single `/api/graph` call — no longer reads as a network call.

### Changed — the embedded skills teach `mixed-cycle`, and the release gate checks them

The router skill (`skills/dossierx/SKILL.md`) gains a `mixed-cycle` section, because the router is
the one file an agent is guaranteed to have read before it meets the finding, and this is a refusal
that fires on a corpus the agent did not touch. An agent meeting one otherwise hunts for what it
broke, finds nothing, and loops. The section says three things: you did not cause it, there is no
migration, and the claims on the loop are usually locked so the recovery is `unlock → fix → lock`
with the human's approval. `dossierx-claims` gains one bullet listing all three loop shapes beside
the edge schema, and the router's surface table now names the claims graph as part of what the human
reads.

The skill line budget moves 230 → 255 to fit it. That is a deliberate resize on the same reasoning
that moved it 200 → 230 for v0.3.0's adoption section, not a per-release ratchet: the router was
already at 229 of 230, so the choice was to cover the breaking change or to cut something else
load-bearing. The four companion skills are unaffected and still sit under 235.

`docs/RELEASING.md` gains a matching pre-merge item. These skills are `go:embed`-ed into the binary
and installed into *other people's* repositories by `dossierx skills export`, where a stale rule does
not render a wrong page — it teaches an agent the wrong recovery on somebody else's locked claims,
and it ships inside the binary, so a fix after the tag never reaches anyone who already installed.
The gate asks the falsification question ("did this release make that assertion FALSE?") rather than
the mention question, and singles out new refusals that can fire on an unchanged corpus.

### Fixed — the browser suite is linted, and says which browser it drove

`viewer-tests/` is a separate module, so `golangci-lint run ./...` at the repository root never read
a line of it — the same blind spot `go test ./...` has, and the reason a `viewer-test` target already
existed. CI's lint job gains a second step with `working-directory: viewer-tests`, and the Makefile
gains `viewer-lint`. The first run found ten findings in code no linter had ever read: five
unchecked errors around the `serve` subprocess teardown, a non-wrapping `%v` that should be `%w`,
and four gocritic style findings. All ten are fixed rather than silenced.

`tests/nested_module_coverage_test.go` — which already refused to let a nested module exist without
a CI test job and a Makefile target — now refuses to let one exist without a lint job either, so the
next nested module cannot repeat this. It checks per STEP rather than per file: the first version
asked whether `ci.yml` contained `working-directory: viewer-tests` and `golangci-lint` anywhere,
and passed immediately against a workflow that linted nothing but the root, because those two true
facts belonged to different jobs.

The suite also now logs which browser it resolved. A green run against the Comet fallback is not the
same evidence as a green run against Chrome — Comet is a Chromium fork that serves its own
`chrome://` UI, and its traffic is what the offline test was misreading as the page's own.

### Fixed — the offline viewer stops probing for a comment backend

Opened over `file://`, the viewer no longer issues the relative fetch that backs its comment
probe, so it no longer logs a `net::ERR_FILE_NOT_FOUND` to the console on load. What the reader
sees is unchanged — the panel was, and remains, read-only offline — and the probe still runs
normally over `http://` and `https://`.

**Not in this release, and stated so deliberately:** any code-grounding signal. The graph audits
claims, not code. There is no `has_code_link` field, no "locked, ungrounded" rule and no
`implink` argument.

## [0.4.1] - 2026-08-04

**Two things change for already-locked claims that `dossierx check` cannot tell you about.**
Neither is an edit a user made, and neither produces a ledger event, so the gate reports exactly
what it reported before the upgrade. They are listed here together because that is the only place
a consumer can see both; each has its own section below.

**1. An already-locked, byte-identical claim renders differently in the viewer.** The shared edges
footer collapses into a `<details>` disclosure and the comment chip moves out of that footer into
the claim's head, so every layout that carries a chip or a footer emits different HTML from the
same locked bytes — no edit, no content-hash move, no `review_pending` flip, nothing in
`.dossierx-lock-store.json`, and nothing for `dossierx check` to report. This is the same shape as
v0.4.0's table fix and v0.3.1's renderer expansion, and the same tool applies: re-run the render
to pick it up, and use `dossierx claim unlock` when you want to revisit the claim's rendered
output on a human's own review. See the edges-footer section below.

**2. A locked claim that already carries `raw_html` re-hashes once against its dependents on the
first check after this upgrade, with no edit and no ledger event.** `ContentHash` — the
`rests_on`/`mirrors`/`governed_by` drift baseline — now folds in `raw_html` when it is non-empty,
so that editing the attachment this release newly allows on a rule-bearing claim is no longer
invisible to that claim's dependents. The change is gated, not additive: a claim with no
`raw_html` — every claim before this release, and most after it — feeds `ContentHash`
byte-identical input to before and re-hashes nothing. The one case that does move is a claim that
already had `raw_html` set (only possible pre-upgrade on an allowlisted, reviewed `layout: mockup`
claim) and is named by another locked claim's `rests_on`, `mirrors`, or `governed_by`: that
dependent's recorded baseline no longer matches the recomputed hash, and it flips to
`review_pending` once, the same no-edit, no-ledger-event shape as this file's last two releases.
See the `raw_html` section below.

**No migration is required** for either — there is no schema change, no new store field, and no
command to run; a flip caused only by the `ContentHash` widening clears the ordinary way,
`dossierx claim reaudit --confirm` or `unlock` then `lock`.

### Changed — the edges footer collapses into a native `<details>`, and the comment chip moves into the claim head

The shared edges footer (`rests_on`, `mirrors`, `governed_by`, `depended on by`, `migrated_from`,
`implemented in`, `review_pending`) now sits inside `<details class="claim-links">` with a
`<summary>` giving a pluralized digest — `"1 link - 2 files"`, `"4 links - 2 files - 1 drifted"` —
where the drifted segment appears only when it is non-zero. `governed_by: none` no longer counts
toward the link total, since it states an absence rather than a followable edge; a claim with no
links and no linked files now emits no `<details>` at all rather than an empty disclosure. The
footer opens automatically, server-side, when a linked file has drifted or the claim is locked and
`review_pending`; a deep link to the claim (`:target`) and print output also force it open, both
via CSS only, since neither signal is knowable at render time.

The comment chip is no longer an `<li>` inside that footer — it moved into each claim's head, as
the head's trailing child, beside the new `<span class="label">` that holds the id/title and
status pill together. A claim with no comment threads reveals its chip on card hover or keyboard
focus (never `display:none`, so it stays in the tab order), rather than being hidden until the
footer was opened. A toolbar toggle expands or collapses every footer on the page at once, as a
client-side DOM change only, with no persistence across reloads. `layout: banner` is the one
partial that renders neither a chip nor an edges footer, and neither half of this touches it.

This changes rendered viewer output on every layout that carries a chip or a footer, so re-run the
render to pick it up.

### Fixed — `raw_html` is an attachment legal on any layout, not a layout a claim must adopt (closes issue #25)

`raw_html` used to be legal only on `layout: mockup`; a claim that was genuinely a table or a list
of steps could not also carry a diagram or a small rendered mockup alongside its own content.
`checkMockupGate`'s layout leg (`internal/lint/raw_html_scope.go`) is removed: `raw_html` may now
sit alongside `body`/`rows`/`steps` on any of the seven layouts. `layout: mockup` stays a valid
layout in its own right — it is the one that also swaps in a "No mockup content." empty state.

This changes **where** `raw_html` may sit, never **who** may author it or **what** reaches the
viewer unescaped — every other leg of the gate is untouched and still fires on every
`raw_html`-bearing claim regardless of layout: the `mockup_modules` allowlist, the
tag/attribute/class markup allowlist, the `raw_html_reviewed` human review flag, and the
lock-lifecycle check. `components.MockupHTML`'s render-time escaping gate is byte-identical. All
seven layout partials (`card`, `table`, `list`, `steps`, `tree`, `banner`, `mockup`) now render
the attachment — reusing the existing `claim-mockup-body` class — after the claim's own
body/rows/steps content and before the edges footer.

A related gap this same widening opened is closed alongside it: `dossierx claim flag`'s body-only
classifier used to key "safe to flag" purely on layout (card/banner/list/tree), which was only
sound while those layouts could not carry `raw_html`. It now keys on whether the claim actually
carries `raw_html`, regardless of layout.

## [0.4.0] - 2026-08-03

**Three things change for already-locked claims that `dossierx check` cannot tell you about.**
None involves an edit, a content-hash change or a ledger event, so the gate reports exactly what
it reported before the upgrade. They are listed here together because that is the only place a
consumer can see the whole set; each has its own section below.

**1. A locked `layout: table` claim renders differently, with no edit and no ledger event.** The
lock ledger signs a claim's `rows` bytes, not the HTML those bytes produce — so the table fix in
this release changes what an already-locked, byte-identical claim looks like in the viewer.
Ordinary table content redistributes by roughly 12% on a two-column 520px table, and a long
identifier that used to force its column wide now wraps. Nothing flips `review_pending`, and
nothing appears in `.dossierx-lock-store.json`. This is the same shape as v0.3.1's renderer
expansion, and the same tool applies: re-run the render to pick it up, and use `dossierx claim
unlock` when you want to revisit the claim's rendered output on a human's own review.

**2. The first write to a locked claim after upgrading may reindent its block scalars.** Claim
writes now merge only the changed top-level keys onto the existing document, but the merged tree
is re-emitted at two-space indent — which is what makes the bytes safe to hand to the round-trip
guard. A `body: |` authored at four spaces comes back at two. This changes bytes, not values: no
content hash moves, no ledger record is affected, and no locked claim is flagged by it. The
place a reviewer will see it is the `git diff`, once.

**3. A claim locked before this upgrade has no governance baseline, and the first edit to its
governor after upgrading does not flag it.** `governed_by` becomes a drift edge in this release,
and a baseline is the governor's content hash recorded at lock time — so a claim locked by v0.3.x
simply has no such entry in `.dossierx-lock-store.json`. There is deliberately **no backfill**, no
adoption event and no announcement: with the migration path removed in this same release there is
no adoption vocabulary left to reuse, and manufacturing a baseline out of content nobody approved
is precisely what v0.4.0 removes. A locked claim gains its baseline the next time it is locked or
re-audited, and only from then on does a governor edit flag it `review_pending`. The deliberate
tools are the ordinary ones — `dossierx claim reaudit --confirm` refreshes the baseline, and
`dossierx claim unlock` followed by `dossierx claim lock` mints one.

### BREAKING — migration is removed; a pre-ledger project crosses by holding nothing locked (closes issue #18)

**`dossierx migrate` — and `dossierx migrate --adopt` — is gone.** Adoption is removed rather
than carried forward: nothing can attest to content no ledger ever recorded, and a command that
manufactures approval out of observed bytes is the one operation v0.3.0 already called unsound.

The replacement is not a command but an **ordered sequence**, quoted here in the exact words the
binary emits so the CHANGELOG, the skills' retired-command row and the retired stub's own hint
cannot drift apart:

> re-propose any locked build order (`dossierx build-order propose --module <m>`), unlock every
> locked claim (`dossierx claim unlock <id> --reason "..."`), then lock only what you still stand
> behind — the first lock in a project with nothing locked crosses the store onto the ledger

The order matters. `build-order propose` requires the module still fully locked, so re-proposing
has to happen *before* any claim is unlocked; the other order strands the locked order with no
way to release it. And what the crossing records matters just as much: the first lock in a
project holding nothing locked stamps the store onto the ledger schema and records a **real
approval**, with nothing grandfathered. That is the whole difference from the removed adoption
path. Commit the updated `.dossierx-lock-store.json` and `.dossierx-comment-digest.json`
alongside the re-locks.

`dossierx migrate` survives as a **hidden retired stub** that fails naming the new path. It has
to exist as a command at all because flag parsing runs before any unknown-command handler — so
without it, the invocation a whole release told agents to type would fail as `unknown flag:
--adopt` rather than as an explanation.

The wire changes, each stated as the rename or removal a consumer's parser will see:

- `error.code` **`adoption_required` → `pre_ledger_unadopted`**. Same three emitters — `claim
  lock`, `claim reaudit --confirm`, `build-order lock` — but the condition is narrower and now
  literal: the project's lock store predates the ledger **and** it still holds locked artifacts.
  On a project holding nothing locked those commands no longer refuse at all; they perform the
  crossing. The recovery is no longer one command but the ordered sequence above, which is why
  the code was renamed rather than left in place.
- `error.code` **`already_migrated` is REMOVED**, together with its `data.mode` discriminator
  (`already_covered` / `nothing_to_adopt`). There is no command left that can be run twice.
- Ledger finding rule **`lock-ledger-adoption-required` → `lock-ledger-pre-ledger`**, and it is
  now **conditional**: silent unless the project holds at least one locked claim or at least one
  locked build order. A pre-ledger project with nothing locked is not in a bad state — it is one
  lock away from being on the ledger — and reporting a finding there told a clean project to run
  a recovery it did not need. The message now names the on-disk schema version, the count of
  locked claims and the count of locked build orders, and carries the ordered crossing steps.
- **`dossierx check`'s `data.ledger_adopted` field is REMOVED.** With no adoption path it could
  only ever be empty, and it was dropped rather than left as a permanently-absent key a consumer
  might wait for. `data.comment_digests_adopted` is unaffected and still reported.
- `dossierx claim show`'s `data.ledger.grandfathered` key **stays**, without `omitempty`, and is
  **always `false`** for any record this build mints. It is true only for records surviving from
  a project that ran the removed adoption path, and the key is kept precisely so a consumer can
  still tell the two apart.

The surface shrinks with it: **eight nouns and twenty leaves become seven nouns and nineteen
leaves** — `check`, `claim`, `comment`, `build-order`, `serve`, `skills`, `version`. The removed
noun is `migrate`, the same one that took the surface from nineteen-under-six to twenty-under-
seven when v0.3.0 added it, so this returns the leaf count to nineteen. The hidden retired stub
is deliberately not counted as surface: it is excluded by mark rather than by hidden-ness, so a
real leaf can never be smuggled past the count by hiding it.

**Two further machine-contract changes fall out of the same removal.** Neither is in the wire
list above, and both would be found the hard way by an agent that branches on them:

- **The dry-run precondition `project_migrated` is renamed `pre_ledger`.** It surfaces in every
  `--dry-run` envelope as `data.preconditions[].name` and, when it blocks, in `data.blocked[]`.
  The eight migrate-only preconditions (`adopt_flag_given`, `history_confirms_pre_ledger`,
  `lock_store_exists`, `locked_claims_match_version_control`, `not_already_migrated`,
  `pre_ledger_claim_not_contradicted`, `something_to_adopt`) go with the command; `pre_ledger` is
  the one replacement, and it is the only one of the set that ever applied to a surviving command.
- **An approval-recording command that REFUSES can still cross the store.** The crossing runs
  before the operation's own preconditions, so a `build-order lock` that then fails — on a
  hand-edited order, say — leaves the store stamped onto the ledger schema even though it
  reported `ok: false`. This is safe rather than merely tolerated: the crossing only runs at all
  when the project holds nothing locked, so it discards no approval, and the post-state passes
  `dossierx check` cleanly. It is recorded here because a durable write by a command that
  reported failure is genuinely surprising, and no other bullet would tell you. `--dry-run`
  writes nothing, and no read-only command crosses.

### Changed — `governed_by` is a drift dependency (closes issue #21)

A claim-valued `governed_by.type` now joins `mirrors` and `rests_on` as a **drift edge**. When
the governing claim's comparable content changes underneath a locked claim, that locked claim is
flagged `review_pending`, `claim show` reports `review_trigger: drift`, and `check`'s
`next_steps` lists it under the drift/reaudit step.

Four boundaries, each of them a decision rather than an accident:

- It is **not a gating edge.** Hub gating is byte-for-byte unchanged — a claim naming an
  unlocked doctrine-facet claim only through `governed_by.type` still locks.
- `governed_by.type: none` creates no baseline and never triggers drift.
- Propagation is **staged**, matching the existing drift edges: flagging a direct dependent does
  not itself flag claims downstream of it.
- A claim reaching the same target through two edge types (`rests_on: X` and
  `governed_by.type: X`) produces exactly **one** baseline entry, deterministically.

For what this means on claims locked before the upgrade, see the note at the top of this entry.
The internal correction that made it possible is worth naming: the drift set had three
independent implementations — `internal/lock`, the pending walk in `internal/comments`, and an
inline copy in `main.go` — which is why a reader who tested drift on v0.3.x may have seen
different answers depending on which command they asked. All three now call one function.

### Changed — comment bodies with a space-indented first content line are now storable (closes issue #24)

A comment or reply body whose first content line begins with **space** indentation used to be
refused as unstorable and now stores fine. The two shapes, as the tests spell them:
`"    func main(){}\n    return"` (space-indented first content line) and `"  a\n  b"` (a
two-space-indented multiline body).

The cause of the loosening is mechanical rather than a relaxed rule: the loader now emits claim
YAML at a two-space indent through one shared encoder, and at that indent a block scalar carries
those bodies back byte-exact — so the round-trip guard, which is the actual gate, stops refusing
them. This is a **widening**, and it lands at every layer that pinned the old behaviour
together, because they are matched by construction: `comments.validateBody`, the `comment add
--dry-run` `body_is_storable` precondition, the CLI's `unsafe_body` refusal, and `dossierx
serve`'s 400 on `POST /api/comments`.

The limit, stated as sharply as the widening: a **tab-led** first content line is still refused,
at any indent width. The store-bricking class survives; it is now tab-led only. (`ErrUnsafeBody`'s
own message still advises "de-indent the first line", which is now broader than the surviving
rule.)

### Fixed — a claim write touches only the keys that changed (closes issue #24)

Adding one comment used to land as a whole-file rewrite — a 117-line diff in which exactly one
key was new — because `SaveClaim` re-serialised the claim from the struct. It now merges the
changed top-level keys onto the existing document's node tree, so an unchanged key keeps its
authored quoting, block-scalar form, key order and YAML comments.

Two modes, because `SaveClaim` is also the file-**create** path: create emits the fresh document
wholesale, mutate merges. Both still end at `verifyRoundTrip` and an atomic write — the
round-trip guard is a gate, not a nicety — and the merge falls back to the fresh whole-document
bytes whenever it cannot be done faithfully, so no write is ever less safe than before.

One deliberate exception, stated as such: **block-scalar indent width is not preserved.** The
merged tree is re-emitted at the loader's two-space indent, which is exactly what makes the
merged bytes safe to hand to the round-trip guard. This changes bytes, not values — a claim's
content hash is computed over decoded field values, which a block scalar's indent width does not
change — so no ledger record is affected and no locked claim is flagged by it.

### Fixed — wide `layout: table` claims scroll instead of running under the viewer chrome (closes issues #22, #23)

A `layout: table` claim wider than its column used to overflow with no way to reach the content.
The table now sits in a `.claim-table-scroll` container that scrolls on its own axis, so the page
body never scrolls sideways, and cells wrap rather than forcing the track wider, with a 5rem
per-cell floor so a column of short values does not collapse to unreadable slivers.

Two consequences a reader will notice:

- **Ordinary table content redistributes slightly — about 12% on a two-column 520px table.**
  This is the knowing price of `overflow-wrap: anywhere` rather than `break-word`: only
  `anywhere` contributes its soft-wrap opportunities to min-content, which is what stops a single
  65-character identifier taking ~80% of the table under auto table layout. `break-word` leaves
  proportions byte-identical and does not fix the bug at all.
- **Markdown tables inside a claim `body` are deliberately not changed.** Both new rules are
  scoped to `.claim-table-scroll`, which a `.md-table` is never inside; a body pipe table is
  already contained by `width: max-content` plus `overflow-x: auto`, so a long identifier makes a
  body table *scroll* rather than wrap. Body tables therefore keep scrolling rather than wrapping
  — a deliberate deferral, not an oversight.

This changes rendered viewer output, so re-run the render to pick it up.

### Fixed — `markdown-sanity` no longer fires on punctuation after a closing delimiter run (closes issue #20)

The `markdown-sanity` lint rule reported an unmatched delimiter run whenever a correctly-closed
emphasis or strikethrough run was followed by punctuation. The measured cases are the clearest
statement of the bug: `"Only ~~strike~~, comma after."` produced one finding while `"Only
~~strike~~ no comma."` produced none, and `"Has **bold**, and more."` produced one with no `~`
anywhere in the string.

The issue report named the wrong cause, and the record is worth correcting: it was not a shared
cursor requiring `**`, `*` and `~~` together — it was flanking being decided on whitespace
alone, so any closing run followed by punctuation looked like an unclosed opener. The scanner now
applies CommonMark's punctuation clause **per rune** rather than per byte, which is what the spec
specifies and what multi-byte punctuation requires.

Widening right-flanking admits every run with a word character on one side and punctuation on the
other, so a carve-out holds the noise budget. It covers the bracket family — `(*Store)`, `a[*p]`,
`Topic_(disambiguation)` — and the **path separator**, without which every underscore-prefixed
path segment (`internal/_generated`, `docs/_partials`, `testdata/_fixtures`) becomes an unmatched
`_` warning. Those were silent before this release; admitting them would have traded the false
positive #20 removed for one this project's own claim bodies hit more often.

The finding's message was also wrong about the cause and no longer claims emphasis is outside the
renderer's subset — it has not been since v0.3.1 — and now names the delimiter it found and where
it opened. These are warning-severity craft findings, so the false positive was noise rather than
a blocked `claim lock`.

## [0.3.1] - 2026-07-30

**Locked claim bodies may render differently after this upgrade, with no edit and no ledger
event.** The lock ledger signs a claim's `body` bytes, not the HTML those bytes produce — so
widening the renderer changes what an already-locked, byte-identical body looks like in the
viewer without touching the field the ledger hashes, without flipping `review_pending`, and
without leaving any entry in `.dossierx-lock-store.json` naming the change. `dossierx check`
will report exactly what it reported before the upgrade: nothing. If a body relied on a
construct that used to fall through to literal text — a line that happened to start with a
dash, a stray backslash, an indented block that used to escape its enclosing list — it may now
render as a live construct instead. There is no automatic re-review step for this; the tool to
revisit a claim's rendered output deliberately, on the human's own review, is `dossierx claim
unlock` (or, for genuine drift, `dossierx claim reaudit`) — see FORMAT.md's markdown-ceiling
section for what changed.

### Changed — renderer expansion

The claim-body markdown renderer grew substantially beyond the previous release's ceiling —
this is the largest single change in v0.3.1, and it lands on every layout that renders `body`
or a `steps` entry (card, banner, list, steps, table, mockup), on comment bodies, and on `rows`
table cells, in each case exactly as far as that surface's ceiling reaches. See FORMAT.md's
markdown-ceiling section for the authoritative construct-by-surface table; summarized here:

- **Block structure.** Fenced code blocks are now recognized by the line scanner itself rather
  than a whole-body pre-pass, so a fence nested inside an open list item or an open blockquote
  no longer splits the container around it; a fence's info string now contributes
  `class="language-x"` to the rendered `<code>` element. One-level blockquotes recurse into the
  same block scanner, so paragraphs, lists, task items, headings, thematic breaks and fenced
  code all render inside a quote. ATX headings at levels 3–6 (`#`/`##` stay reserved for the
  viewer's own chrome and render as literal text). Thematic breaks. Unordered/ordered lists
  nest to unbounded depth via an indent-keyed stack, with GFM task-item checkboxes, CommonMark
  list looseness (a property of the list, not the item — a tight nested list stays tight inside
  a loose parent), and `<ol start="n">` for a list that does not begin at 1. Hard line breaks (a
  trailing backslash or two trailing spaces) become `<br>`.
- **Inline constructs.** Backslash escapes over a closed 15-character set (`<` and `&` are
  deliberately outside it, so `\<` still shows its backslash); double-backtick code spans;
  `**bold**`, `*italic*`/`_italic_` under strict CommonMark flanking (an intraword underscore
  can never open or close, so `governed_by`-shaped tokens never italicize), and `~~strike~~`;
  angle-bracket and bare-URL autolinks. The inline-only ceiling used by `rows` table cells and
  by a GFM pipe-table cell (`markdown.RenderInline`) gained the same emphasis, strikethrough and
  autolink constructs as the block ceiling, on top of the escapes/code-spans/links it already
  had — **every `rows` table cell renders differently after this upgrade** if its text happens
  to contain `*`, `_`, `~~`, or a bare URL. A third field joins that ceiling: a `governed_by:
  none` claim's `reason`, which the edges footer used to write through `html.EscapeString`, now
  renders through `markdown.RenderInline` — so a reason that names a claim id in backticks or
  carries a link, an asterisk or an underscore-flanked token renders differently after this
  upgrade too, on the same no-edit, no-ledger-event terms as a body.
- **GFM pipe tables**, new in this release: a header row, a required delimiter row (which must
  itself carry a pipe — a bare `---` stays a thematic break), and zero or more body rows. A
  well-formed table is always rendered as a table, at any size or shape: a body row with fewer
  cells than the header renders **short** rather than padded, and a longer row has its extra
  cells dropped. Row splitting happens before inline parsing, so a pipe inside a cell's code
  span still splits the cell unless escaped (`` `a\|b` `` is the working spelling for a literal
  pipe inside inline code).
- **Images**, new in this release, and the one construct that is not available everywhere the
  rest of this list is: `![alt](src)` renders as a real `<img>` in claim-authored `body` and
  `steps` text and in a table cell embedded in that text, and **never** in a comment body. `src`
  must resolve under the claim's own `assets/` directory (a fixed, non-configurable name) and
  end in one of six extensions (`.png .jpg .jpeg .gif .webp .svg`); anything else renders the
  whole `![alt](src)` as escaped literal text rather than as a broken image. Two new lint rules
  ship with it: `markdown-sanity` (mostly warning-severity craft findings — a malformed table, an
  unclosed fence — but error-severity on the security-relevant ones, such as an off-origin image
  or link `src`) and `asset-scope` (error-severity throughout — it can refuse `dossierx claim
  lock` on an image `src` that resolves outside `assets/` or carries an unlisted extension, on a
  corpus that had no such check before this upgrade).

### Changed — actionable status pills on claim-edge references (closes issue #11)

v0.3.0 shipped readable edge labels but left the last piece of issue #11 unbuilt: a claim-edge
reference said nothing about the state of the claim it pointed at. A `governed_by`, `mirrors`,
`rests_on` or "depended on by" target now carries a small status pill — `draft`, or
`review_pending` for a locked target flagged for re-review — reusing the same three-way
status-to-class mapping every claim-head pill already uses. The pill is **actionable-only**: a
healthy locked target gets nothing, so the footer stays quiet and lights up exactly on the case
it exists to explain, a claim gated on an edge that is not ready yet. It requires the whole
catalog to resolve a target's status, so it rides the catalog-aware edges binding
(`internal/render`'s `attachEdgesOverride`); the parse-time `edges`/`claimEdgeList` funcMap
bindings have no catalog and render every target with no pill, exactly as before — which is why
`build_order.html`'s `rests_on` list, which lists only locked claims anyway, is unchanged.

### Changed — new HTTP surface in `dossierx serve`

`dossierx serve` mounts a second, non-API route: `GET /claim-assets/<claim-id>/<path>`, which
serves exactly the images the loaded claims reference, answered from an allowlist computed from
those claims rather than by walking the filesystem — a percent-encoded path, a path outside the
extension allowlist, a symlink that resolves outside the claims directory, or anything that is
not a regular file is a bare 404, with no distinction from "does not exist". The viewer's
`Content-Security-Policy` on `GET /` widens accordingly, from `default-src 'none'; style-src ...`
to `default-src 'none'; img-src 'self'; style-src ...` — the one relaxation this release makes to
the CSP, scoped to same-origin images only.

### Fixed — a denial-of-service path in `parseLink`, live in the currently-shipped binary

This bug **predates** v0.3.1 and is live in the v0.3.0 binary you are upgrading from; the fix
ships in this release. `parseLink`'s bracket-matching had several quadratic-ish rescans —
repeatedly walking forward to a `]` or `)` from each `[` in the remaining text instead of
indexing them once — that produced no wrong byte of output but cost seconds to minutes of CPU on
an adversarial input. This is the smallest of **four** quadratic paths this release bounds,
each measured against a 1 MiB reviewer-authored body (the practical ceiling on a comment or
claim `body`) at the same commit:

| path | before | after |
|---|---|---|
| list continuation accumulator | 10.6s CPU, a 65 GB allocation | 23ms |
| fences indented under a list item | 8.55s CPU | 21ms |
| `parseLink` bracket rescan | 5.8s CPU | 12ms |
| fence rescan | 299ms @ 8 KiB | 5ms @ 16 KiB |

The `parseLink` case matters more on the comment surface than the CPU number alone suggests,
because `handleListComments` (`internal/serve/handlers.go`) re-renders **every** stored comment
on **every** `GET /api/comments` call — so one stored hostile body is not a one-time cost, it is
amplified across every later read of the comment panel for as long as that comment exists. The
65 GB allocation in the list-continuation path is reachable from a 1 MiB comment body with no
special privilege — any reviewer who can leave a comment could trigger it before this fix. All
four paths are bounded with an index built once per scan and guarded by a 16-shape growth sweep
plus a 1 MiB absolute-budget test in `internal/render/markdown/markdown_cost_test.go`.

### Security — an off-origin `<img>` could pass the mockup review gate, live in the currently-shipped binary

This hole **predates** v0.3.1 and is live in the v0.3.0 binary you are upgrading from. A
`layout: mockup` claim's `raw_html` is the one field DossierX renders unescaped, and it is
allowed only for an allowlisted module, only with `raw_html_reviewed: true`, and only through a
tag/attribute allowlist in which `<img>` may carry a **relative** `src`. That relative-only test
was a regular expression that treated the literal bytes `//` as the only authority prefix.
Browsers normalise `\` to `/` in the authority position of an `http`/`https` URL, so
`src="\\evil.example/p.png"`, `src="/\evil.example/p.png"` and `src="\/evil.example/p.png"` all
resolve off-origin — and all three passed the gate, meaning a **reviewed, locked** mockup claim
could load a third-party image (and leak the viewer's IP, User-Agent and referrer) from a page a
human had signed off on. `src="//evil.example/p.png"`, the one spelling the regex knew, was
correctly refused, which is why the gap was invisible in review.

The fix is not a stronger regex. The same rule was written **four** independent times in this
repository — the mockup gate, `internal/lint`'s markdown scanner, and two places in
`internal/render/markdown` — and the other three already knew that a backslash counts as a
slash; the weakest copy had simply never been brought up to the others. All four now call one
leaf package, **`internal/urlsafe`**, which imports nothing else in the module (so both the
renderer and the linter can depend on it) and exports a single by-construction gate:
`IsOffOrigin` refuses any explicit scheme, all four authority spellings, a root-relative path, a
query or a fragment, after HTML-entity-decoding and after stripping every ASCII control byte and
space — so `&#47;&#47;host`, `ht&#9;tp://host` and `\x01//host` are refused on the same terms as
the bytes they decode to. The four local copies are deleted rather than left delegating.

Four behaviour changes follow, all narrowing what is accepted, all confined to the mockup
`<img src>` gate. A **root-relative** `src` (`/foo.png`), an **empty** `src`, a `src` carrying a
**query** (`x.png?v=2`) and one carrying a **fragment** (`x.png#frag`) are now refused where they
previously passed. The last two are the ones most likely to surprise: a cache-busting query on an
otherwise relative path is same-origin, and the finding still describes it as a non-relative URL.
The gate implements the rule as written — a relative path with no scheme, no authority prefix, no
leading `/`, no `..`, no `#` and no `?` — and is deliberately stricter than the security argument
alone requires. No fixture or shipped mockup uses either form. Relative forms are unaffected:
`../diagrams/x.svg`, `./x.png` and `x.png` still pass, including the `../` form the shipped Google
Cloud Console mockups use.
Nothing on the claim-body image path or the markdown link-scheme allowlist changed: those were
already on the strong rule, and their accept/reject decisions are byte-identical across this
change.

## [0.3.0] - 2026-07-28

The agent-first restructure. DossierX has two users with opposite needs: an **agent** that
operates it, and a **human** who reviews what the agent did. Until now both were half-served by
one command line. v0.3.0 gives each its own surface and takes the other away — the agent gets a
20-command machine-readable CLI, the human gets the viewer and one command (`dossierx serve`).

Alongside the split, this release closes the gap that made the split worth making: until now a
locked claim could be hand-edited and **nothing would notice**. The new lock ledger records what
was approved, when, by whom, and on whose words, and a gate compares the claims against it in
`dossierx check`, in a pre-commit hook, and in CI.

**This release is not backward compatible at the CLI.** Twelve commands were removed and four were
moved. The migration table below maps every one of them.

### BREAKING — every existing project must run `dossierx migrate --adopt` once

**This is the one change that breaks a project rather than a script, and it breaks on the first
`dossierx check` after the upgrade.** If your project has ever locked a claim, run this once,
before you lean on the hook or CI:

```sh
dossierx migrate --adopt --dry-run   # look first: it names every artifact it would adopt
dossierx migrate --adopt
```

then commit the rewritten `.dossierx-lock-store.json` — and the `.dossierx-comment-digest.json` the
same run creates — in the same commit as the claims they now cover. `--adopt` is required, so a bare
`dossierx migrate` refuses with `missing_flag` rather than guessing at a migration you did not name.
`--dry-run` lists every claim and build order it would adopt and writes nothing.

**There is deliberately no `--reason`,** and that is the one place this command breaks the pattern
every other record-writing verb follows. They take the human's words because a human approved
something. Nobody approved this. Every record the migration writes carries a fixed reason saying so
— *"grandfathered by `dossierx migrate --adopt`: locked before this project had a lock ledger;
content adopted as-found on migration day, never approved by anyone"* — and `grandfathered: true`,
permanently. A human-supplied reason would make an adoption read like an approval in a ledger diff,
which is exactly the confusion the fail-closed decision exists to remove.

**Why this is not automatic any more.** Earlier in this release cycle it was: a pre-ledger project
was grandfathered in on its first plain `check`, which observed whatever the claims said at that
moment and recorded them as approved. It was convenient and it was unsound, because *adoption is
the one operation in the design that manufactures approval out of nothing*. A gate that performs it
on sight rewards deleting the ledger with a clean bill of health over content nobody reviewed, and
turns "arrive with no ledger" into a universal bypass. Review rounds then tried to distinguish an
honest v0.2.x store from a deliberately downgraded one using evidence inside the project, and could
not: `locked_at` shipped in v0.2.0 (verifiable with `git show v0.2.0:internal/lock/lock.go`), so no
field, no timestamp and no sibling file tells the two apart. When no predicate can be trusted the
answer is not a cleverer predicate — it is to stop guessing and make a human decide, once.

So **adoption now fails closed, in every run**: a missing or unreadable ledger never grandfathers
anything on plain `check`, on `--validate`, or on `--staged`. The only code path that writes an
adopted record is the one a human invokes deliberately.

What the migration does and does not do: it hashes each currently-locked claim and each locked
build order **exactly as they sit on disk** and records them as the baseline, marked as adopted
rather than approved, permanently — an adopted hash is content that was *observed*, not reviewed.
It changes no claim's `status`, resolves no thread, and clears no `review_pending`. Read the claims
before you run it; nothing in the command can check them for you. It is an upgrade step and never a
recovery tool: on a project that already has ledger coverage it refuses, and reaching for it to
silence a gate on a project that *has* a ledger would record tampered bytes as approved, which is
precisely what the fail-closed rule exists to prevent.

Skipping it is loud rather than silent. `dossierx check` fails on the lock-ledger gate with the new
project-scoped finding **`lock-ledger-adoption-required`**, under the `integrity_failed` code the
gate already uses — **no new `error.code` was added for it**. It is one finding naming the
migration, deliberately in place of one `lock-ledger-missing` per claim: repeating "this claim is
locked with no record" N times would attach a recovery (set it back to draft and re-lock) that is
actively destructive advice at a project which has done nothing wrong. It is also genuinely
distinct from its neighbour, and the **lock store itself** is what tells the two apart, with no git
history required: `lock-ledger-adoption-required` fires when the store is **present** and still on
the pre-ledger schema (benign; recovery is the migration), `lock-ledger-absent` when the store
**file is gone** while locked claims remain (tampering; recovery is version control).

Running it twice is refused, with the new `error.code` **`already_migrated`** and a `data.mode` of
`already_covered` or `nothing_to_adopt` so a caller can tell the two apart — a migration that can be
re-run is a laundering command, since deleting one record and re-migrating would re-sign the edit it
covered as approved. A pre-commit hook and a CI run fail the same way as `check` — the run that
would previously have blessed a project quietly now refuses it.

### `check --staged` judges the git index, and only the git index

`--staged` reads the **git index** — what the commit will actually contain — with `git show`
instead of the worktree, and writes nothing. That is what makes it meaningful in a pre-commit hook,
and it is unchanged. What it does **not** do is read git history: it judges one tree, and its
verdict is identical in every clone, at every depth, on every branch.

**Removed before release: the parent-commit comparison.** An earlier build on this branch had
`--staged` resolve the parent of the commit under judgement and compare the two, reporting a removed
lock ledger or a repointed `claims_dir` as **`integrity-store-removed`** and
**`claims-scope-narrowed`**. Both rules are gone, along with the shallow-clone advisory in
`data.next_steps` that told you to set `fetch-depth: 0`. **The shipped CI template changed with
them**: it is now a single `dossierx check` step and pins no `fetch-depth`, because a shallow
checkout is a complete tree and one tree is the whole evidence base. The second `check --staged`
step is gone from CI too — on a fresh checkout the index, the worktree and `HEAD` are three names
for one tree, so it re-ran the same rules over the same bytes. `--staged` itself is **not**
deprecated; its home is the pre-commit hook, where the index and the worktree genuinely differ.
The diagnosis behind the removed rules was right — a change that takes a claim's
evidence together with whatever was left to judge it against leaves the gate nothing to disagree
with, so it reports `ok: true` — but the fix was in the wrong layer. The parent commit is outside the
*commit* and not outside the *committer*: git history is written by exactly the party the gate
constrains, so `--orphan`, a rebase or a second config file switched the comparison off without
looking unusual in a log, and every other in-repo source of that evidence has the same property.
It also could not see intent the tree does not record, and charged two ordinary git operations for
that: a legitimate **`git revert`** of a commit containing a claim lock was refused (the revert
removes that lock's records, byte-identical to erasing them) and, because git does not run
`pre-commit` for `revert`, it landed locally at exit 0 and only CI objected; and a project that was
**new in a commit** was audited against an unrelated retired project's ledger in a monorepo and
refused with findings naming another project's claim ids. A control that a rebase disables and a
revert trips is not a control, so it was removed rather than patched.

**What this costs, and the boundary it leaves.** Removing the comparison gives up the detections
that needed a second tree: those where one change writes a claim's evidence **and** whatever was
left to judge it against, so that the single-tree gate has nothing surviving to disagree with.
Earlier revisions of this entry tried to count those cases and publish the list. The count rose
every time someone looked harder, and two of the published statements turned out to be false, so
the list is gone and the boundary is stated as what it always was:

> **An in-repo ledger cannot attest anything against the person who can write it.**

The gate catches every change that leaves a surviving file **disagreeing** with the one that moved,
which is what drift and tampering usually look like: an agent editing a locked claim, a careless
hand-edit, a bad merge, a status flipped by hand, a deleted approval record, an erased comment
thread. Each leaves the files nobody touched disagreeing with the one that was, and that
disagreement is a named finding. What has nothing to disagree with it is not caught — a record's
`reason`, `at` and `actor` are testimony, not signature, and no rule compares them to anything. Nor
is **coordinated** change — a claim and its ledger record rewritten together, in one commit, by
someone entitled to write both. The sharpest form is not a deletion at all: unlock a locked claim, rewrite its body,
re-lock it (minting a fresh record whose hash correctly covers the new content), then hand-edit
that record's `reason`, `at` and `actor` back to the original approval's values. `check` and
`check --staged` both report `ok: true`, over a ledger crediting a human who approved nothing. That
is an illustration, not an inventory.

No in-repo mechanism closes it, which is the general form of why the parent comparison failed: any
evidence the tool consults lives in the repository, and the repository is writable by the person
being gated. It is caught where a control the committer cannot rewrite actually lives — **branch
protection with a required CI check, plus review of a diff in which such a change is loud**, since
it is a hand edit of a tracked JSON store whose whole purpose is to be read in a diff, sitting
beside the claim change it was made to permit. **DossierX detects; the forge enforces.** The
boundary is pinned rather than asserted, in `internal/check/staged_no_parent_test.go` end to end
and `internal/lock/audit_boundary_test.go` at the audit layer, each beside its "the uncoordinated
half is still refused" assertions.
[FORMAT.md](FORMAT.md#what-the-gate-detects-what-it-does-not-and-where-the-rest-is-caught) states
the principle in full, including the one direction that would move it: signing the ledger with a key
held outside the repository.

Everything else in the gate is untouched and still single-tree: `lock-content-drift`,
`lock-ledger-missing`, `lock-ledger-deleted`, `lock-ledger-released`, `lock-ledger-orphan`,
`lock-ledger-abandoned`, `lock-ledger-absent`, `lock-ledger-unreadable`, `lock-ledger-downgraded`,
`lock-ledger-adoption-required`, `comment-ledger-drift`, the `comment-digest-*` family and the
`build-order-*` family, plus fail-closed adoption and `dossierx migrate --adopt`.

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

### Added — two integrity holes closed at the command that used to launder them

Both were found by reproducing them against the shipped binary, and in both the *gate* was already
correct: `check` named the tampered state at every step. What was missing is that naming it is not
refusing it, and in each case the next ordinary command wrote the evidence whose absence was the
finding — so the sequence ended green, permanently.

- **`lock-ledger-deleted` is now a refusal on `claim lock`, not only a finding.** Delete one
  claim's entry from the `ledger` map, flip `status: locked` to `draft`, rewrite the body, and
  `dossierx claim lock` used to succeed — `RecordApproval` wrote a *fresh* record over the
  rewritten content, and every check from then on exited 0 with zero findings. The claim's own
  `locked_at` stamp and dependency baselines survive the deletion (nothing removes them; `unlock`
  *releases* a record rather than deleting it), so "this engine locked it and the record is gone"
  is answerable, and locking is now refused with `integrity_failed`. The recovery is restoring the
  lock store — **not** `unlock → fix → lock`, which signs the attacker's edit. A claim this engine
  never locked is untouched by the gate, so a first lock still works normally.
- **`comment-digest-unrecorded` is now a refusal on `claim lock` and on every comment op.** An
  approval records the claim's comment digest in the same act, unconditionally — so on a claim
  whose digest key had been dropped, locking *manufactured* an entry from whatever the comments
  block said at that moment. Measured on a covered project: a human's open thread blocks the lock;
  forge `status: resolved` and drop that one key; `check` correctly reports
  `comment-digest-unrecorded`; `claim lock` then exits 0 and certifies the forged block; every
  later check exits 0. The human's objection is gone and the record says the review was clean.
  `dossierx comment add`/`reply` closed the same door from the other side — an *unknown* digest on
  a covered claim that carries threads is no longer treated as "cannot have drifted". Silent, in
  both, where evidence is honestly absent: an uncovered project, an absent digest store
  (`comment-digest-absent` is that cause, said once), and a claim with no threads at all.

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
- **`dossierx migrate --adopt`** — the seventh noun, and the only command in the surface a
  *human* is expected to run other than `serve`. It exists because adoption stopped being
  something a `check` does on its own; see the BREAKING section above. `--adopt` is required and
  `--dry-run` previews; there is deliberately no `--reason` (see the BREAKING section).
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
- **`.dossierx-lock-store.json`, `.dossierx-comment-digest.json` and `.dossierx-flag-store.json`
  are TRACKED ARTIFACTS.** Commit them; never `.gitignore` them. CI compares the claims against
  the ledger, so a project that does not track it has no gate; and a `review_pending` claim whose
  flag-store entry did not travel with it reaudits to an *empty* proposal, whose `--confirm`
  clears the human's flag having applied nothing. Documented in README, FORMAT.md, the skills,
  the hook installer's own output, and the CI workflow template.
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
- **Ten named findings**, stable strings the hook, CI and the skills branch on:
  `lock-ledger-absent` (locked claims but no ledger file — a hard error, never a silent pass,
  because "no ledger means bless everything" would make `rm` the universal bypass),
  `lock-ledger-missing`, `lock-ledger-released` (a `locked` claim whose only record was released
  by an unlock — a released record is not a standing approval, and the hash still matches because
  the hash excludes `status`, so `lock` → `unlock` → hand-edit `status:` back was otherwise a
  complete bypass that fired no rule at all), `lock-content-drift`, `lock-ledger-orphan`,
  `lock-ledger-abandoned` (an unreleased record whose claim FILE is gone — every other per-claim
  rule is driven by the claims that exist, so deleting one walked past all of them at once and
  there is no `claim delete` verb to have made it deliberate), `comment-ledger-drift`,
  `build-order-content-drift` and `build-order-ledger-missing` (a locked
  `.build-order.<module>.json` is what an implementing agent actually builds from; its approval
  record was being written and never read, so reordering the phases or splicing the frozen
  `hashes` baseline changed the plan with no finding anywhere), and
  `lock-ledger-unreadable` (a ledger that exists but will not parse fails closed and loudly).
  The gate is deliberately **not** a lint: registering these in the lint registry would let one
  tampered file freeze locking project-wide and stop the viewer regenerating. It runs as
  `check`'s last step, after the catalog and viewer are written — a disputed project still
  regenerates its documentation, it just does not exit 0.
- **`dossierx check --staged`** judges the **git index** — what the commit will actually
  contain — and writes nothing at all. `project.config.yaml`, every claim, the lock ledger, the
  comment digest and every locked build order all come from that one snapshot, so no unstaged
  edit can change the verdict on a commit that does not carry it. Claim content is read through
  a single `git cat-file --batch` over the index's own object ids, never conditionally off disk:
  git's stat cache and its `assume-unchanged` / `skip-worktree` bits are attacker-writable, and
  a gate whose evidence source they can choose is not a gate. Outside a work tree it warns and
  exits 0.
- **A pre-commit hook installer**, `scripts/install-git-hook.sh` (plus `install-git-hook.ps1`
  for PowerShell). It **asks before writing anything**, resolves `core.hooksPath` instead of
  assuming `.git/hooks` (so a repo using husky/lefthook is installed into *its* hook directory,
  never hijacked), handles linked worktrees, refuses to replace a foreign hook without
  `--force`, and is a no-op when re-run. The hook body is embedded in the one file so an agent
  can drop it into a project that has the binary but not this repository. In a repository holding
  **more than one** DossierX project the hook checks **every** one of them, in index order;
  `DOSSIERX_CONFIG` narrows it to a single project but is never required. It also refuses the
  commit — rather than reporting "no project here, skipping" — when it located a config that
  `dossierx` then returned `config_not_found` for, since a gate that cannot run must not pass.
- **A CI workflow template**, `scripts/ci/dossierx-check.yml`, to copy into a consuming
  project. **CI is the authority, not the hook**: git does not run `pre-commit` for a clean
  merge, a rebase, a cherry-pick or a revert, `--no-verify` is one keystroke away, and most
  contributors never installed the hook. If you adopt one of the two, adopt CI.
- **Adoption**, covered in full by the BREAKING section at the top of this release. In brief: a
  pre-ledger project is no longer grandfathered by any `check` and is adopted only by
  `dossierx migrate --adopt`, with claims and already-locked build orders adopted in the same act
  (splitting the two halves across the ledger line would leave a project half-covered).
- **`check --staged` judges the git index and nothing else** — no parent-commit comparison, no git
  history, the same verdict in every clone. See the `--staged` section above for what that does and
  does not detect, and where the remainder is caught.
- **Moving `claims_dir` needs no flag, and there is deliberately no escape hatch.** An exemption
  switch on an integrity gate is the attack, since the party who reliably remembers to set it is
  the one who read the source looking for it. None is needed either: move the claims and the
  stores in the **same commit**, keeping the claim files byte-identical, and it passes because
  every locked claim is still reachable and still hashes to its existing record. A repoint that
  **strands** locked claims fails from state alone, as `lock-ledger-abandoned` once per claim —
  their records are left naming claims the project can no longer see.

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

- **`dossierx build-order propose` now releases the approval it discards, and a hand-cleared
  `"locked": false` is a finding no matter what else was edited beside it.** The build-order
  orphan rule could only identify a *lone* flag flip: it re-signed the artifact as if the flag
  were still true and required that hash to match the record, because without that test it could
  not tell a hand edit from the honest window between a re-propose and the lock that follows.
  A content edit re-signs to something else, so flipping the flag **and** gutting the phases in
  the same write was strictly quieter than flipping the flag alone — `check --validate` reported
  `OK`, exit 0, and offered `dossierx build-order lock` as a next step over a sequence nobody
  approved. `propose` now writes the truth instead of leaving the gate to guess it: it releases
  the module's ledger record as it overwrites the artifact (under the lock-store sentinel, held
  across both writes, so contention refuses with nothing written). The honest window is therefore
  the only unlocked artifact whose record is *released*, and the rule needs no exception —
  any unlocked artifact under a **standing** record is `build-order-ledger-orphan`. The
  `--dry-run` discloses the release as a side effect.
- **A run that adopts a comment digest says so.** `lock.PrepareStore` left the adopted ids on the
  store and `cmd/dossierx` dropped them, so `dossierx check` printed `ok:true`, zero findings,
  exit 0 on the very run that recorded a hand-added comment block as truth. The ids now ride the
  same channel as the adopted claim records, reaching `data.comment_digests_adopted` and the
  envelope warnings — with the recovery that actually applies, which is **not** a re-lock: no verb
  in this binary clears a recorded comment digest, so the only way back is version control.
  Deliberately silent when the digest store is being *created* (a new project, or the one-time
  `migrate --adopt` crossing), where every block is adopted by definition and nothing has been
  laundered.
- **`claim lock --dry-run` no longer previews a lock the real run refuses.** Its
  `claim_is_draft` precondition read the claim file's own `status:` line — the exact line a hand
  edit rewrites — so a claim flipped out of locked without an unlock, still carrying a standing
  approval, previewed as lockable and was then refused `already_locked`. The preview now asks the
  question the real run asks, as a `no_standing_ledger_record` precondition. The refusal's
  `error.hint` also splits by state: a claim whose file says `locked` is pointed at `unlock`,
  while one that says `draft` under a standing approval is told to **restore from version control
  first**, since unlocking there accepts the edit that caused it.
- **A comment op refused because the digest store is unreadable now reports
  `comment_digest_unavailable`, not `internal`.** `internal` is defined as an unclassified
  failure — "a bug report, not a branch target" — and the reflex it invites is a retry. This
  refusal is deterministic and keeps failing identically until `.dossierx-comment-digest.json`
  is restored, so classifying it as internal sent a caller into a retry loop over a write. The
  code carries the fact that makes it actionable: **nothing was written**.
- **`build-order lock` refusing a hand-edited artifact now reports `build_order_hand_edited`,
  not `build_order_refused`.** Every recovery documented for `build_order_refused` is a repair
  to the *claims* (lock the remaining ones, resolve a thread, set a missing `build_role`, break
  a `rests_on` cycle). In this refusal the claims are correct and the *artifact* is not, so an
  agent following any of them inspected correct claims, found nothing to fix, and looped. The
  recovery for the new code is the one that works: re-`propose`, then `lock`.
- **A fresh project now acquires its comment digest store at the moment its lock store is
  created**, not only when an older project migrates across. A project that reached
  ledger-covered through its first `claim lock` never ran a migration, so it ended up
  ledger-covered with no digest store — on disk, indistinguishable from a project whose digest
  store had been **deleted**. Deleting the store from an already-covered project is still never
  silently re-created, so the deletion stays visible to the gate.
- **`FORMAT.md` no longer states that `governed_by` is hub-gated.** Hub gating walks `mirrors`
  and `rests_on` only, so a doctrine claim named *only* by `governed_by` is not gated — a reader
  who believed otherwise would drop the redundant `rests_on` edge and lock against an unapproved
  doctrine claim. `FORMAT.md` also no longer claims there is deliberately no comment-digest
  absence rule; `comment-digest-absent` ships, and its real boundary is now documented.
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
- **`claim lock` refuses a claim that is already `locked`** (`already_locked`), instead of
  re-signing the ledger over whatever the file currently says. Re-locking was the single command
  that laundered every gate this release adds: it re-stamped the approval over drifted content,
  re-snapshotted the dependency baselines, cleared `review_pending` with no diff shown, and left
  the human's `claim flag` entry stranded where `reaudit` could no longer reach it — all at
  exit 0, on the verb the drift finding itself names. `lock --dry-run` had reported
  `blocked: true` for this case all along; the write path now agrees with its own preview.
- **A comment write no longer re-blesses a tampered `comments:` block.** Every comment operation
  recorded the digest unconditionally, so a single ordinary `comment reply` — on an unrelated
  thread, on the same claim — erased a standing `comment-ledger-drift` finding and made a
  forged `resolved` the recorded truth. The digest is now compared against the claim as it was
  read, and a disagreement refuses the write (`comment_digest_drift`) rather than overwriting the
  record. Adoption on a never-seen store is unchanged.
- **The pre-commit hook no longer refuses every commit in a repository whose project lives
  under a non-ASCII path.** git's `core.quotepath` defaults to *true*, so the hook's
  `git ls-files` discovery query got a project at `café/project.config.yaml` back as the
  C-quoted string `"caf\303\251/project.config.yaml"` — surrounding double quotes and all.
  Handed to `--config`, that names no file, `dossierx` answers `config_not_found`, and the
  hook's (correct) rule that a config it discovered but cannot open is a refusal rather than a
  skip did the rest: **every** commit refused, on every branch, for every developer, including
  commits touching no claim at all, until somebody uninstalled the gate. Discovery now passes
  `-c core.quotepath=false`, the same override `check --staged`'s git runner has always used.
  Pinned by a `scripts/hook-smoke-test.sh` case that asserts both halves — an honest commit
  still passes, and a tampered locked claim is still refused — because "still refuses" alone is
  satisfied by a hook that refuses unconditionally, which was the bug. The hook body's marker is
  now `pre-commit v4` (see the two entries below for what v4 added); re-run the installer to
  pick it up. A v3 install is classified `outdated` and replaced cleanly by re-running the
  installer — no `--force` needed.
- **`check --staged` no longer disarms itself when `claims_dir` points outside the config's own
  directory.** Every git pathspec was built relative to the *config file's* directory, so the
  ordinary monorepo layout — `docs/project.config.yaml` with `claims_dir: ../claims` — produced
  the spec `../claims`, which mapped to the deliberate "no index here" escape hatch and exited 0
  having evaluated nothing. A tampered locked claim committed with **no hook output at all**.
  The git runner now anchors itself at `git rev-parse --show-toplevel` and takes the project's
  position from `--show-prefix` (asked of git rather than derived by string arithmetic, so
  macOS's `/var` vs `/private/var` symlink cannot desync it); the escape hatch is now reached
  only when a path is genuinely outside the work tree. `data.staged_files` consequently carries
  repository-relative paths — identical in the layout where the config sits at the repo root,
  and only different for the layout that used to be broken.
- **A skipped `check --staged` is no longer indistinguishable from a pass.** `--format json`
  never printed the skip warning and the hook branched on the exit code alone, so a gate that
  evaluated nothing looked byte-identical to a clean run. The hook (`pre-commit v4`) now matches
  `data.skipped`, re-runs in text mode so the reason reaches the screen, and **refuses** the
  commit, naming the likely cause and the `--no-verify` hatch.
- **An untracked `project.config.yaml` no longer judges tracked content.** `check --staged` fell
  back to the *worktree* config whenever the config was merely untracked — and an untracked
  config can be edited without staging anything, so pointing it at a pristine decoy directory
  made the gate report `OK`, exit 0, over an index carrying a tampered locked claim. It now
  refuses with a distinct, non-skippable error unless the index genuinely holds nothing to judge
  (no claim blob, no lock ledger, no comment digest store), which keeps the first-commit case the
  fallback exists for working. Ordinary repository YAML — workflows, chart values — does not
  decode as a claim, so a repo full of unrelated YAML is not turned into a refusal.
- **The viewer's browser suite is actually run.** `viewer-tests/` is a separate Go module (it
  needs `chromedp`; the engine's `go.mod` stays cobra + yaml.v3), which means the root
  `go test ./...` cannot descend into it — and until now nothing else did either: no CI job, no
  Makefile target. Its assertions against the viewer's inline JavaScript, including this
  release's comment-chip suite, had only ever executed on a maintainer's laptop while CI was
  green on three platforms. There is now a `viewer` CI job running it against the runner's
  headless Chrome, and a `make viewer-test` target (plus `make hook-test`) so both
  outside-the-root-module suites are reachable locally. The job sets `DOSSIERX_TEST_BROWSER`
  explicitly, because the suite *skips* when it cannot find a browser and a skip in CI is
  indistinguishable from a pass. `tests/nested_module_coverage_test.go` fails the build if a
  nested module is ever added without both.
- **`check --staged` reads `project.config.yaml` from the index as well.** It read the claims,
  the lock ledger and the comment digest from the git index and the config from the working tree,
  so an UNSTAGED one-line `claims_dir:` edit pointing at an empty directory enumerated zero
  claims, audited zero claims and passed every commit that followed. The gate now judges one
  consistent snapshot.
- **`check --staged` no longer trusts `git diff` to decide which files it may read from the
  worktree.** git deliberately omits paths carrying the assume-unchanged or skip-worktree bit, so
  those were precisely the paths whose worktree copy was read in place of the index blob —
  the substitution `--staged` exists to prevent. Both cases are pinned end to end in
  `scripts/hook-smoke-test.sh`.
- **`review_pending` reconciliation consults the flag store.** `check` re-derived only two of the
  three triggers, so a `review_pending: true` line deleted by hand (or by a bad merge) on a
  *flagged* claim was never restored and never reported: the claim vanished from
  `claim list --review-pending`, `reaudit` refused it, and the recorded doc/code mismatch became
  unreachable. It now uses the same shared predicate the comment ops and `reaudit` use.
- **`comment inbox` no longer drops a thread the human REOPENED.** `last_activity` was the newest
  reply's timestamp, or the thread's own creation time — never the resolve or reopen — so a
  reopened thread's activity sorted *before* any cursor the agent already held and disappeared
  from every incremental `--since` poll, which is exactly the message the inbox exists to deliver.
- **`comment inbox --since` validates its argument.** A malformed value answered `ok: true` with
  an empty inbox and echoed the bad value back as the next cursor, so the failure was
  self-perpetuating and byte-indistinguishable from "the human left you nothing new". It is now
  refused (`bad_request`) and normalized to UTC before comparison.
- **`build-order lock` on a stale order returns `build_order_stale`**, the code the skill's
  refusal table has always documented, instead of the generic `build_order_refused` whose three
  documented recoveries do not apply — leaving an agent that branches on `error.code` (as it is
  told to) with no reachable path to "re-propose, then re-lock".
- **`check`'s next steps only name a claim that would actually lock.** It named the first draft
  claim in load order without evaluating the gates, so on a module drafted alongside its own
  dependencies — including this repository's own shipped fixture — the one command an agent is
  told to trust exited 1 with `lint_failed`. The example is now chosen through the same gate
  evaluation `claim show` reports, and when nothing is lockable yet it says so.
- **A noun with no subcommand emits an envelope.** `dossierx claim`, `comment`, `build-order` and
  `skills` printed help prose on stdout and exited 0, so an agent that dropped a subcommand got
  the success signal plus output its JSON parse throws on — and no way to tell that from a
  command that genuinely had nothing to report. They now behave like any other bad invocation:
  one envelope, `usage`, exit 1, with `error.hint` naming the available leaves. An unknown leaf
  (`dossierx claim nonsense`) lands in the same place and names what was typed.
  `dossierx version` is the verb that reports the version; `--version` is unchanged.

### Changed
- The CLI is **20 leaf commands under 7 nouns**, down from 26. A test pins the exact set, so
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

[Unreleased]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.8...HEAD
[0.7.8]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.7...v0.7.8
[0.7.7]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.6...v0.7.7
[0.7.6]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.5...v0.7.6
[0.7.5]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.4...v0.7.5
[0.7.4]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.3...v0.7.4
[0.7.3]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/BarterX-Tech/dossierx/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/BarterX-Tech/dossierx/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/BarterX-Tech/dossierx/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/BarterX-Tech/dossierx/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/BarterX-Tech/dossierx/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.2.0
[0.1.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.2
[0.1.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.1
[0.1.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.0
[0.0.3]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.3
[0.0.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.2
[0.0.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.1
