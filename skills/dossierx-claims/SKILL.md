---
name: dossierx-claims
description: >-
  Workflow for authoring and reviewing "claims" (atomic YAML facts) rendered by
  DossierX into an HTML viewer. Use this WHENEVER creating, editing, linting,
  locking, or reauditing a claim under any project's claims/ directory that
  DossierX consumes. Covers the claim schema (including build_role), the
  lint → catalog → render → check pipeline, and the lock/review_pending/reaudit
  lifecycle (dependency changes never silently auto-unlock — they flag
  review_pending and go through a confirm-before-write diff review). For
  implementing code from a fully-locked module, see the dossierx-build-order
  skill; for linking finished code back to its claims, see dossierx-code-links.
---

# DossierX claims — claim authoring & review

A disciplined workflow for any project that consumes DossierX (a config-driven CLI that
turns YAML "claims" into a linted, validated HTML viewer). DossierX is generic — nothing
here is specific to any one project; it applies to any repo that has a
`project.config.yaml` pointing at a `claims/` directory and the dossierx CLI available
(installed via `go install github.com/BarterX-Tech/dossierx/cmd/dossierx@<tag>`).

This is the base DossierX skill, covering claims themselves. Three companion skills build on
it:

- **[[dossierx-build-order]]** — deriving and following the dependency-ordered sequence an
  implementing agent should build a locked module in.
- **[[dossierx-code-links]]** — grounding finished code in the claims it implements, and
  what to do when a later code change means a locked claim is no longer true.
- **[[dossierx-comments]]** — the threaded review-discussion layer: when to open a comment
  (a remark needing human dialogue) versus a `dossierx flag` (a proposed content edit), the
  advisory-rights rule, and how an open thread gates locking.

## When to use this

- Creating a new claim (a fact that belongs in the claims system) for any module/facet.
- Editing an existing claim's `body`, `rows`, `layout`, `build_role`, or edges.
- Running `dossierx lint` / `dossierx catalog` / `dossierx render` / `dossierx check` to validate and
  rebuild the viewer.
- Locking a claim, or handling a claim that `dossierx stale` reports as `review_pending`.
- Running `dossierx reaudit <id>` after an upstream dependency changed, or after `dossierx flag`
  (see dossierx-code-links) marked a claim's own meaning as drifted.

## Claim basics

A claim is one YAML entry, one claim per file (a file with a second `---`-separated YAML
document is a hard load error, not a silent drop — split extra claims into their own files),
with:

- `id: module.FACET.slug` — FACET must be one of the project's configured facets.
- `status: draft | locked` — set by hand only via `dossierx lock`/`dossierx unlock`, never edited
  directly in the YAML.
- `layout: card | table | list | steps | tree | banner | mockup` — optional; if omitted, the
  engine infers it from shape (`rows` present → `table`, `steps` array → `steps`, else →
  `card`). Prefer being explicit once a claim is non-trivial — inference is a fallback, not
  the primary path.
- `build_role: orientation | schema | behavior | api | verification | out-of-scope` —
  **required once a claim is locked** (a lint hard-fails a locked claim with no build_role
  set). Classifies the claim for Build Order (see that skill) — it has nothing to do with
  this claim's reading position in the viewer (that's `section`/`order`).
- `body` (prose, not lint-checked as data) and/or `rows` (structured, lint-checked for
  consistent columns *and* for every cell being an authored string — a number/bool/list/map
  cell is a `rows-shape` error, so quote it) — a claim needs at least one. In both `body`
  and `rows` cells, backtick code spans and `[text](url)` links now render as `<code>`/`<a>`
  (link URLs are limited to `http`/`https`/`mailto`/relative/`#`; `javascript:`/`data:` are
  neutralized to inert text). Bare URLs are not autolinked — write `[text](url)` explicitly.
- Edges: `mirrors` (deterministic value equality — both sides must declare it), `rests_on`
  (semantic dependency — target must exist, never point at an unmigrated module; use prose
  instead until that module has real claims), `governed_by` (`{type, reason}` — `reason` is
  required when `type: none`).

Run `dossierx lint` after writing or editing a claim, before moving to the next one — don't
batch several claims and lint at the end. A failing lint on claim A is easiest to fix while
claim A is still the thing you're looking at.

## Orientation-note claims — read these first

Some claims are not facts about the system — they are agent/reviewer-facing
guidance about how to read the *rest* of a module's claims (e.g. "if you
only call this module from elsewhere, read Contract, never Internals").
These carry `kind: orientation-note` (or live under the reserved
`module.overview.*` facet, which implies it automatically — see
`model.Claim.EffectiveKind`) and always render as a `layout: banner` card:
a colored, non-bold callout that visually sets the note apart from
ordinary fact claims on the same tab, so it can't be skimmed past by
accident.

Before reading any other claim in a module: read that module's
`module.overview.*` claims first (they render on every one of that
module's facet tabs, not just one), then the `kind: orientation-note`
claims pinned at the top of whichever facet tab you're actually there
for. `dossierx lint`'s `orientation-note-order` rule guarantees these always
sort ahead of every fact claim in their facet, so "read top to bottom" is
always equivalent to "read orientation notes first" — you never have to
hunt for them.

The banner component stacks `id` above `body` (not side by side) with
plain, non-bold text at natural weight — the banner's own red/warn tint
is the visual signal, so don't lean on bold or a long inline id to make a
banner stand out further; keep `body` prose readable on its own.

`dossierx check` reports a non-blocking `orientation notes: module "...": N
(...)` line per module that has any, so their existence is visible
without a separate command.

## The pipeline

```
claim.yaml  →  dossierx catalog  →  .catalog.json  →  dossierx render  →  viewer/index.html
```

`dossierx check` runs lint → catalog → render in one shot, then (non-blocking) reports Code
Links status and always ends with a **next steps** block — the derived list of exactly what
to run next (which claims to lock, which to reaudit, which module is ready for a build
order, which code links have drifted). Treat `dossierx check` as the one command you run
routinely; every other command below is a one-time decision gate you reach for only at the
specific moment that decision applies — never something to remember on a schedule.

`dossierx render` always overwrites `viewer/index.html` (it carries a "generated — do not edit"
header); never hand-edit the generated viewer file.

## Lock lifecycle — what to do with a `review_pending` claim

A locked claim's status **never** silently drops back to `draft`. `review_pending` is `true`
while ANY of three independent triggers stands:

1. A dependency's content changed underneath it (`dossierx check` detects this automatically via
   a stored content hash).
2. An agent explicitly ran `dossierx flag <id> --claim-says --now-does --reason` because
   implementing or maintaining the code revealed the claim's own stated meaning no longer
   matches reality (see **[[dossierx-code-links]]** — this is the only place a human comes
   back into that skill's otherwise-autonomous flow).
3. An open comment thread was added to the locked claim — a review remark that needs human
   dialogue and carries no proposed edit (see **[[dossierx-comments]]**).

`review_pending` is set automatically but never cleared automatically; it clears only once
EVERY trigger is gone, via one of three matching clearers: a human-confirmed
`dossierx reaudit <id> --confirm` (triggers 1 and 2), `dossierx unlock`, or resolving/deleting
the last open comment thread with `dossierx comment resolve` (trigger 3, when no drift or flag
still stands). Relatedly, a claim **cannot be locked at all while it carries an unresolved
comment thread** — `dossierx lock` refuses it and names the blocking thread; resolve, then lock.

Do not hand-edit a `review_pending` claim's YAML directly as the default move. For a drift- or
flag-sourced trigger (1 or 2), go through the reaudit flow so the change is visible and
reviewable first; for an open-thread trigger (3), resolve the thread instead of reauditing (see
the note after the steps):

1. `dossierx stale` — lists every claim currently `review_pending`.
2. `dossierx reaudit <id>` — proposes a diff, rendered with git-diff-style `<mark>`
   highlighting: green add (`<mark style="background:#b7ebb0">…</mark>`), red strikethrough
   remove (`<mark style="background:#f7c2c2;text-decoration:line-through">…</mark>`). For a
   dependency-drift trigger this is a stubbed no-change proposal (no live LLM backend wired
   in yet — treat any content diff there as illustrative). For a `dossierx flag`-sourced trigger
   this is real and precise: the flag's `--claim-says` renders as the removal, `--now-does`
   as the addition. Either way, this step writes nothing to disk by itself.
3. **Present the proposed diff and wait for explicit confirmation** — same review-before-
   apply discipline used throughout this workflow. Do not auto-apply.
4. On confirmation, run `dossierx reaudit <id> --confirm` — this strips the markup, applies the
   edit, refreshes the lock timestamp + dependency hash, clears the drift/flag trigger, and
   (for a flag-sourced proposal) consumes the pending flag so it never re-fires. If an open
   comment thread is *also* standing on the claim, `review_pending` stays `true` until that
   thread is resolved too (reaudit cleared triggers 1/2, not trigger 3). The claim stays
   `locked` throughout; it is never demoted to `draft` by this flow.
5. On rejection, do nothing further via the CLI — the claim stays `locked, review_pending`
   until either hand-edited directly or re-reaudited later. Never force-clear the flag
   without either path.

**Reaudit refuses a comment-only `review_pending` claim.** If the *only* reason a claim is
`review_pending` is an open comment thread — no drift, no flag — `dossierx reaudit` exits
non-zero (there is no content diff to confirm) and tells you to resolve the thread instead.
Reaudit is for content changes; a comment is dialogue. See **[[dossierx-comments]]** for the
comment verbs and the advisory-rights rule (an agent never resolves a human-opened thread).

## Portability note

DossierX itself takes zero project-specific behavior from its own source — everything
project-shaped (facet list, module list, claims dir, source dirs, doctrine facet, template
overrides) comes from that project's `project.config.yaml`. If you're introducing DossierX
to a new project, that config file plus a `claims/` directory is the entire integration
surface; never patch DossierX's own source to special-case a project. This has been
verified directly, more than once, by running the full feature set against an unrelated
synthetic project with different facets/modules/vocabulary and confirming zero code changes
were needed.
