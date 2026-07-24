# DossierX claim format

This document describes the on-disk claim schema and project config schema
this engine reads. It is generic by design — nothing here names any
specific project, module, or facet. All project-specific vocabulary
(which facets exist, which modules exist, where claims live) comes from
`project.config.yaml`, never from the engine itself.

## Claim

A claim is one atomic YAML fact, one claim per file, under the project's
configured `claims_dir`. This is enforced: a claim file must contain exactly
one YAML document. Stacking a second `---`-separated document into the same
file is a hard load error (the engine rewrites a claim's file as a single
document when it locks or reaudits it, so a second document in that file
would be silently clobbered). Split multiple claims into separate files.

```yaml
id: module.facet.slug          # e.g. widget.contract.overview
facet: string                  # must be in project.config.yaml's facets[]
module: string                 # must be in project.config.yaml's modules[]
status: draft | locked
layout: card | table | list | steps | tree | banner   # optional
build_role: orientation | schema | behavior | api | verification | out-of-scope  # optional (see below)
body: markdown string          # optional, illustrative prose
rows: [ { ... } ]              # optional, structured data
steps: [ string ]              # optional, ordered steps
section: string                # optional, in-content section heading (see below)
mirrors: [ id, ... ]
rests_on: [ id, ... ]
governed_by:
  type: none | doctrine_id
  reason: string                # required when type is "none"
migrated_from: string           # optional provenance note
order: int                      # optional, viewer-only display sequencing (see below)
```

### `id` grammar

`id` is three dot-separated segments: `module.facet.slug`.

- `module` — one of the project's configured `modules[]`.
- `facet` — one of the project's configured `facets[]`.
- `slug` — a free-form, kebab-case identifier unique within that
  `module.facet` pair.

### `layout` inference

When `layout` is omitted, it is inferred from the claim's shape:

1. `rows` is non-empty → `table`.
2. Otherwise, `steps` is non-empty → `steps`.
3. Otherwise → `card`.

`list`, `tree`, and `banner` are never inferred; a claim must set them
explicitly.

### `order` and viewer sequencing

`order` is optional and purely a viewer concern — it has no effect on
`.catalog.json` or lint output.

- Unset (or `0`) means "no explicit order": the claim keeps whatever stable
  fallback position it would otherwise get (currently source-file order).
- Set to a positive int to pin the claim ahead of every unordered claim in
  its module/facet group, ascending by `order` among claims that set it.

This is deliberately separate from `internal/catalog.Document`'s claim
order, which is always alphabetical by `id` — that ordering exists solely
to keep `.catalog.json` and lint diffs byte-deterministic across builds and
must never be repurposed for display sequencing. `order` only reorders how
a module/facet group's claims are laid out in the rendered viewer
(`internal/render.orderClaims`); it does not exist in `.catalog.json`.

### `build_role` and the build/implementation order

`build_role` is optional and orthogonal to `order`/`section` above: those
two are viewer-only reading-order concerns, while `build_role` drives a
different, additional ordering concept — a module's build (implementation)
order, computed by the engine's `internal/buildorder` package once every
claim in that module is locked.

- Unset (`""`) is allowed while a claim is `draft` — a human may not have
  decided yet where a claim sits in its module's build sequence.
- Once a claim locks, `build_role` becomes required for that claim's
  module, but only once that module has set `build_role` on at least one
  other claim — a module that has never used `build_role` at all sees no
  change in its lock-time behavior. This is enforced by the
  `build-role-required-for-locked` lint, not by the schema itself.
- The six values, in the fixed sequence a build order is computed in
  (`out-of-scope` is never part of the sequence — see below):
  1. `orientation` — context/process claims read for background but never
     themselves acted on during implementation.
  2. `schema` — data-shape claims (types, fields, storage layout); built
     first among the "real work" phases.
  3. `behavior` — workflow/logic claims, the bulk of the real
     implementation work; ordered within this phase by `rests_on` edges to
     other `behavior` claims in the same module.
  4. `api` — public-function/entry-point claims, built after the behavior
     they call into.
  5. `verification` — test-checklist/acceptance-criteria claims, read last
     so tests can be written against everything else already built.
  6. `out-of-scope` — deferred/future-scope claims. Never placed in a
     module's build order, but still reported (as excluded) by
     `internal/buildorder`, so nothing silently vanishes from view.
- A `rests_on` edge from one claim to another claim in the SAME module
  whose `build_role` is a later phase in the sequence above is a
  phase-order violation — a modeling error the dependency graph doesn't
  respect the fixed phase sequence — and is refused, by name, when a
  build order is proposed. A `rests_on` edge to a claim in a DIFFERENT
  module is informational only and never checked this way: cross-module
  dependencies are out of scope for one module's own build sequence.

See `internal/buildorder`'s package doc comment for the full propose /
status / lock lifecycle (`dossierx build-order propose|status|lock`), which
mirrors `internal/lock`'s own draft→locked→stale lifecycle for claims.

### `section` and in-content headings

`section` is optional, free-form, human-readable text (e.g.
`"5 - workflows / lifecycle"`) that a claim author may set to get an
in-content section heading rendered in the content area, grouping that
claim under a visible label as the viewer scrolls.

`section` is the only supported way to get this — the engine never derives
a heading (or any other structure) from `claims_dir`'s directory layout.
See "Directory layout" under Project config below for why: directory
naming is cosmetic only, so section identity has to come from a real
schema field instead of a path convention.

### `status` and the lock lifecycle

- `draft` — freely editable, not yet reviewed.
- `locked` — has passed human review via `dossierx lock`; also carries an
  engine-managed `review_pending` bool, which is `true` only while a
  dependency's content has drifted since the claim was last locked or
  reaudited, and is otherwise `false`. A locked claim's `status` never
  reverts to `draft` on its own — `review_pending` is the only automatic
  transition, and only a human-confirmed `dossierx reaudit --confirm` clears
  it. See the engine's `internal/lock` and `internal/reaudit` packages for
  the full lifecycle.

## Edge types

A claim may reference other claims by `id` via three distinct kinds of
edge, each with a different meaning:

- **`mirrors`** — a deterministic equality edge. The target claims'
  comparable content must match this claim's exactly; if they diverge,
  that is a lint failure (`mirror-mismatch`), not merely staleness.
- **`rests_on`** — a semantic-consequence edge. This claim depends on the
  target claim remaining true, but is not required to be textually
  identical to it. When a `rests_on` target's content changes underneath a
  locked claim, the locked claim is flagged `review_pending` rather than
  invalidated outright.
- **`governed_by`** — names the doctrine claim (by id) that backs this
  claim's authority, or explicitly declares `type: none` with a required
  `reason` when no such doctrine claim exists. When the project config
  sets `doctrine_facet`, a claim naming a claim in that facet as a
  dependency cannot be locked until the doctrine claim itself is locked
  (hub-gating). If `doctrine_facet` is unset, hub-gating does not run at
  all.

## Project config (`project.config.yaml`)

```yaml
schema_version: 1              # engine refuses to run on an unknown version
title: string                    # optional; viewer <title>, header, and
                                   # sidebar heading. Falls back to a generic
                                   # "dossierx viewer" default when unset.
eyebrow: string                  # optional one-line subtitle rendered under
                                   # the sidebar heading (e.g. "user-intelligence
                                   # service"). No fallback — unset renders no
                                   # eyebrow element at all.
facets: [string, ...]           # non-empty, no duplicates
modules: [string, ...]          # non-empty, no duplicates
claims_dir: path                 # resolved relative to this file's own directory
                                  # (directory layout inside it is not part of
                                  # this spec — see "Directory layout" below)
doctrine_facet: string           # optional; omitted disables hub-gating entirely
viewer:
  template_overrides: path        # optional override dir; resolved relative
                                    # to this file's own directory. Eligible
                                    # for override, by filename, inside it:
                                    # the 6 per-layout component partials
                                    # (card.html, table.html, list.html,
                                    # steps.html, tree.html, banner.html)
                                    # plus the outer shell (shell.html) and
                                    # base stylesheet (style.css). Missing
                                    # individual files inside it fall back
                                    # to engine defaults per-file; a
                                    # configured-and-missing directory itself
                                    # is a hard load-time error.
  theme:                          # optional CSS token overrides; see below.
    accent: "#3fb950"
```

All paths in this file are resolved relative to the config file's own
location, never the process's current working directory — this is what
lets the same engine binary be pointed at a config file from anywhere.

### Directory layout is not part of this spec

`claims_dir`'s internal structure — subdirectory names, nesting depth,
how files are grouped on disk — carries no meaning to the engine and is
entirely the claim author's choice. `internal/loader.LoadClaims` walks
`claims_dir` recursively and loads every `*.yaml`/`*.yml` file it finds,
matched purely by file extension; it does no filename or path-segment
parsing of any kind. A claim's `module` and `facet` come only from that
claim's own YAML fields (`module:`, `facet:`), never from where the file
happens to live on disk. This means a project can reorganize its
`claims_dir` freely — flatten it, rename subdirectories, move files
between them — without touching claim content or breaking anything the
engine reads.

#### Recommended authoring convention (non-enforced)

Although the engine attaches no meaning to directory structure, a
consistent layout still helps humans navigate a large claims tree. This
convention is a suggestion, not a rule — the engine does not check for
it, and a project is free to organize `claims_dir` differently:

```
claims_dir/
  <module>/
    <facet>/
      <topic-slug>.yaml
```

e.g. `claims/widget/contract/severity-policy.yaml`. Grouping by
`module` then `facet` mirrors the two-level grouping the viewer already
renders claims under, so the on-disk layout roughly matches what a
reader sees — but this is purely for the humans editing the claims
tree. If a project wants in-content section headings (as opposed to
just a tidy directory tree), use the `section` field described above;
the engine does not infer one from directory naming.

### `viewer.theme`

`viewer.theme` maps a fixed set of CSS custom-property token names (without
the leading `--`) to their values, letting a project restyle the shipped
viewer's colors, fonts, and corner radius without writing any CSS or
touching `viewer.template_overrides`.

The allowlist is intentionally fixed and engine-owned — an unrecognized key
is a load-time error (typo protection), not a silently-ignored field:

```
accent, accent-bg, ink, muted, faint, paper, card-bg, border, link,
warn, warn-bg, font-sans, font-mono, radius
```

Rules:

- Every present key's value must be non-empty.
- No value may contain `;`, `{`, `}`, `<`, or `>` — theme values are
  interpolated verbatim into a generated `<style>` block, so this is
  rejected outright as the actual injection-safety concern, independent of
  whether the value is otherwise a valid color or font-family.
- The color-shaped keys (every key above except `font-sans`, `font-mono`,
  and `radius`) additionally get a light format sanity check: the value
  must look like `#hex` (3/4/6/8 digits), `rgb(...)`/`rgba(...)`, or a bare
  CSS named color (e.g. `forestgreen`).
- A project may set none, some, or all of the allowlisted keys. Any key
  left unset keeps the engine's own default for that token (see
  `internal/render/viewer/template/style.css`'s `:root` block) — this is a
  per-token fallback, not an all-or-nothing swap, and it holds in both the
  light and dark `prefers-color-scheme` variants of the default stylesheet.

`viewer.theme` and `viewer.template_overrides` are orthogonal: a project
can use either, both, or neither. Theme tokens are plain CSS custom
properties, so they cascade into markup produced by any template —
including markup from a fully custom `template_overrides` partial, shell,
or stylesheet — with no extra wiring required on the override's part.

#### Mode-invariant vs. mode-varying tokens

The allowlisted tokens split into two groups, and the two groups should
usually be treated differently by a project's config:

- **Mode-invariant** — `accent`, `accent-bg`, `link`, `warn`, `warn-bg`,
  `font-sans`, `font-mono`, `radius`. These don't change between light and
  dark mode in the shipped default stylesheet (a brand color, a link color,
  and typography/radius choices read the same regardless of OS theme), so
  it's safe for a project to always set these — there is no light/dark
  variant to accidentally clobber.
- **Mode-varying** — `ink`, `muted`, `faint`, `paper`, `card-bg`, `border`.
  These are the ones `style.css`'s `@media (prefers-color-scheme: light)`
  block flips to different values than its dark defaults. The engine emits
  every configured token into one unconditional `<style>:root{...}</style>`
  block, and an unconditional rule always wins over a `@media`-scoped one —
  so setting any of these six pins the viewer to a single mode for every
  visitor, permanently overriding their OS/browser preference, regardless
  of which value you picked.

A project that deliberately wants to force one mode (e.g. a dark-only
brand site) can still set some or all of the six mode-varying tokens — that
is a supported, intentional use of the allowlist. But it should be a
deliberate choice: leave them unset by default and let the engine's own
`@media` defaults handle both OS themes, and only set them when forcing a
single mode is actually the goal. Setting all fourteen tokens out of habit
(e.g. by copying a fixed dark palette wholesale into `viewer.theme`) is the
most common way this trap gets hit by accident.
