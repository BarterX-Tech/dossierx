---
name: dossierx-claims
description: >-
  Authoring, inspecting and moving claims through their lifecycle in a DossierX
  project. Use this WHENEVER you are about to create a claim, edit one, find
  the claim a human described in words, run dossierx check, or lock, unlock,
  flag or reaudit anything under a project's claims/ directory. Covers the
  claim schema and id grammar, dossierx claim new, the read-only authoring loop
  (dossierx check --validate), dossierx claim show and list, citing evidence with
  sources and [n] markers, the cross-cutting track axis and dossierx track
  list/show/status, the three
  review_pending triggers, and the one rule everything else hangs off — draft
  claims are free, a locked claim only ever changes via unlock, fix, lock.
  Load the DossierX router skill first; it carries the envelope, the exit
  codes and the error-code recovery table this skill assumes.
---

# DossierX claims — authoring and lifecycle

Read **[`dossierx`](../dossierx/SKILL.md)** first: the envelope, the exit codes, the `error.code` recovery table and
the five rules are there and are not repeated here.

## The contract, in one table

| you want to | run |
|---|---|
| author a claim | `dossierx claim new <id> --body "..." --governed-reason "..."` |
| check your work, writing nothing | `dossierx check --validate` |
| build everything (catalog, viewer, code-link scan, ledger gate) | `dossierx check` |
| know everything about one claim | `dossierx claim show <id>` |
| find the claim a human described | `dossierx claim list --match "<their words>"` |
| what is pending / migrated / drifted | `dossierx claim list --review-pending` · `--migrated` · `--drifted` |
| freeze a claim, on the human's word | `dossierx claim lock <id> --dry-run`, then `--proposal "<snapshot>" --reason "<their words>"` |
| change a locked claim | `dossierx claim unlock <id> --reason "..."` → edit → `dossierx claim lock <id> --dry-run`, then `--proposal "<snapshot>" --reason "..."` |
| a locked claim drifted from a changed dependency | `dossierx claim reaudit <id>` (preview) then `--confirm --reason "..."` |
| what feature tracks this project declares | `dossierx track list` |
| read a feature end to end, assembled across modules | `dossierx track show <id>` |
| is a feature finished? | `dossierx track status <id>` — COMPLETE when every claim it owns and every claim it cites is locked |

## A claim

One YAML document per file. A second `---` document in the same file is a hard load error — split
it out.

- `id: module.FACET.slug` — **exactly three** non-empty dot-separated segments. `module` and
  `FACET` must be ones the project declares in `project.config.yaml`; `slug` must be kebab-case
  (lowercase alphanumerics, single hyphens). The viewer's card title is derived from the slug, so
  `retry-policy` renders as "Retry Policy" — you never write a title.
- `status: draft | locked` — **only** `dossierx claim lock` / `unlock` may change this. Editing it
  by hand walks past the lint gate, the doctrine gate and the open-thread gate as though all three
  had passed, and the lock ledger will report it as `integrity_failed` on the next check.
- `body` (prose) and/or `rows` (a table; every cell must be an authored **string**, so quote
  numbers and booleans). A claim needs at least one. `body` gets the wider **block** ceiling —
  paragraphs, fenced code, lists, GFM pipe tables, images (claim `body`/`steps` only, `src`
  confined to that claim's own `assets/`), headings `###`–`######` only, one level of blockquote,
  and every inline construct. Every `rows` cell gets the narrower **inline-only** ceiling: no block
  construct and no image. See FORMAT.md's "`body` and the markdown ceiling" section for the full
  construct-by-surface account — do not reach past either ceiling.
- `layout: card | table | list | steps | tree | banner | mockup` — inferred from shape if omitted.
  Be explicit once a claim is non-trivial.
- `build_role: orientation | schema | behavior | api | verification | out-of-scope` — **required
  before a claim can lock** once a module uses the feature. It orders implementation (see
  **[`dossierx-build-order`](../dossierx-build-order/SKILL.md)**) and has nothing to do with `section`/`order`, which are the
  human's reading order in the viewer.
- Edges: `mirrors` (value equality; both sides must declare it), `rests_on` (semantic dependency;
  the target must exist), `governed_by: {type, reason}` —
  `reason` is required when `type: none`; a claim-valued `type` is a **drift** edge (its content
  changing under a locked claim flags `review_pending`) but never a gating one, so it cannot block
  a lock.
- **Loops are refused in all three shapes**, at ERROR: `rests_on` → `cycle`, `governed_by` →
  `governed-cycle`, and as of v0.5.0 one *alternating* the two → `mixed-cycle`. "B is governed by A"
  buys no free back edge when A already rests on B; `mirrors` is exempt. The router's `mixed-cycle`
  section covers why an untouched corpus can start failing this.
- `kind: orientation-note` (implied by the reserved `overview` facet) marks a claim that tells a
  reader how to read the *rest* of the module. It renders as a banner and sorts ahead of fact claims.
- `sources` — optional, the evidence behind the claim, cited from `body` as `[1]`, `[2]`. See
  **Citing your evidence** below.
- `tracks` — optional, cross-cutting feature membership: `- {id: checkout, role: owns|cites}`,
  role defaulting to `cites`. See **Feature tracks** below.

## Authoring — `dossierx claim new`, not a text editor

Hand-writing claim YAML is the thing this design gates. Author through the command: it enforces
the id grammar, the body requirement and the governed-reason rule **before** it writes, then lints
the project with the new claim in it — an `orphan` warning on a claim with no edges yet is a
warning, not a refusal.

`--rests-on` / `--mirrors` / `--governed-by` / `--build-role` / `--section` / `--layout` are all
available at creation time; `--file` may only name a path **inside** `claims_dir` (the loader walks
nothing else, so a claim written outside it reports success and is then invisible). After creation
the claim is a **draft** — edit its file freely.

The loop while authoring is `dossierx check --validate`: the same lint gate `check` drives, at the
same severity, writing **nothing** — no claim files, no lock store, no `.catalog.json`, no viewer.
Run the full `dossierx check` when you want the viewer rebuilt and code links scanned.

## Citing your evidence — `sources`

`sources` records **what makes this claim true**, in the claim, where `check`, the viewer and the
lock ledger can all see it. It is a different field from `migrated_from`, which answers *what this
claim replaced*. Never keep the evidence in a sidecar file instead: nothing checks a sidecar, so
the evidence behind a locked claim could be rewritten after the human approved it and nothing
would say so.

```yaml
sources:
  - ref: 1
    kind: external
    title: SCShareableContent
    url: https://developer.apple.com/documentation/screencapturekit/scshareablecontent
    accessed_on: 2026-08-15
    supports: "the enumeration API returns windows only while the user has granted permission"
  - ref: 2
    kind: internal
    title: Product requirement PVR-010
    path: migration/product-requirement-map.jsonl
    record_id: PVR-010
    sha256: 8afd3c9a...
```

An `external` source needs `url` **and** `accessed_on` — the date records what the page said on the
day it was read. An `internal` source needs `path` (relative to `project.config.yaml`) **and**
`sha256`, and may set `record_id` to pin one JSONL record (matched on its top-level `"id"`) rather
than the whole file, which churns for reasons unrelated to your claim.

Cite from `body` with `[n]`: *"frames arrive only while the stream is running [1]."* Markers are
read **in prose only** — never inside a fenced block or an inline `` `code` `` span — and **only on
a claim that declares `sources`**, so a source-less claim writing `array[0]` is unaffected.

Five lints: `source-shape` (ERROR — `ref` positive and unique, `kind` known, `title` present, no
field used across kinds, `internal` names a `path`), `source-ref-undefined` (ERROR — the body cites
`[n]` that no entry declares), `source-external-unanchored` (ERROR — no `url` or no `accessed_on`),
`source-internal-drift` (ERROR — the hash is missing, the file or record cannot be read, or the
content no longer matches; **a check that cannot run is a failure, not a pass**), and
`source-ref-unused` (WARNING — an entry nothing cites).

**`sources` is signed by the lock ledger and is not part of the dependency-drift hash.** Editing a
citation under a locked claim is `lock-content-drift`, exactly like editing the body — so the path
is `unlock → fix → lock`. But adding or correcting a citation never flips a dependent to
`review_pending`: provenance is not contract.

## Feature tracks — the second ownership axis

`module` says **who guarantees this**; a track says **what the user gets, and whether it is
finished**. Declare the vocabulary in `project.config.yaml` (`tracks: [{id, title, summary}]`),
then name them from a claim's own `tracks:` list, one `{id, role}` entry each.

**One owner per axis.** Exactly one `module`, and **at most one** track whose role is `owns`; every
other membership is `cites` — a reference, never a copy. Owning is what lets a feature's trigger,
failure behaviour and acceptance criteria live in the corpus as a real, lockable claim. Two claims
owning the same track is `track-multi-owner` (ERROR).

Track membership is **not an edge**: it carries no cycle lint and joins no cycle walk. The other
lints are `track-shape` (ERROR), `track-unknown` (ERROR — the claim names a track config does not
declare), `track-empty` (WARNING — a declared track nothing references) and `track-unowned`
(WARNING — citations but no owner).

**Never treat a track as a gate.** `dossierx track status <id>` REPORTS: COMPLETE when every claim
the track owns and every claim it cites is locked. It does not block anything, and track membership
never gates `dossierx claim lock` — a claim locks on its own merits, and the way to change a locked
claim's `tracks` is `unlock → fix → lock`, since `tracks` is signed by the ledger like every other
field.

## Finding the claim the human meant

They will say "the retry card in contract". That is not an id, and guessing costs a
`claim_not_found` — or worse, acts on the wrong claim. Run
`dossierx claim list --match "retry" [--facet contract] [--module widget]`.

Each row carries `claim_id`, `title`, `status`, `review_pending`, `drifted`, `open_threads` and a
`score` — a ranked ladder over the id and derived title, so a confident hit sits well above a tie.
**Name the winner and its title back to the human and wait** before running anything that writes.

## `dossierx claim show` — one call, the whole picture

Prefer it over reading the YAML. It reports status, lock state and `locked_at`, `review_pending`
plus **which** trigger caused it, both edge directions (`rests_on`/`mirrors` outgoing,
`depended_on_by`/`mirrored_by` incoming), `implemented_in[]` with per-file drift, comment counts
with the open thread ids, and `next_actions` — computed from the *same* gate evaluation the write
path uses, so it can never disagree with what the command would do. Read it rather than re-deriving
the lifecycle yourself.

## Locked means locked

A draft claim is yours. A locked claim is the human's, and the **only** path through it is
`dossierx claim unlock <id> --reason "<their words>"` → edit the file →
`dossierx claim lock <id> --dry-run`, then `--proposal "<snapshot>" --reason "<their words>"`.

Both ends require `--reason` and take `--dry-run`. Preview, show the human the `side_effects`
(locking records a content baseline; unlocking releases it and can flip dependents), get a yes,
then run it. `--reason` carries their approval into the record — never fabricate one.

The window between the two ends is not a steady state. If any source file carries a
`dossierx-claim:` tag for that id, a plain `dossierx check` mid-edit fails with `implink_refused`
and `claim is not locked (status "draft")` — the tag is fine, the claim is mid-edit. Finish the
relock; never touch the tag or leave the claim unlocked to silence it.

`dossierx claim lock` refuses on four gates, each with its own `error.code`: `lint_failed` (fix
the findings), `unresolved_comments` (reply, and let the human click Resolve),
`dependency_not_locked` (a doctrine dependency is still draft), and `already_locked` — a claim
that is *already* `locked` is not re-locked, because re-locking a drifted or flagged claim would
sign whatever the file now says and clear `review_pending` with no diff shown. `unlock` → fix →
`lock`, or restore the file from git.

## `review_pending` — and why `reaudit` is not the edit tool

A locked claim's status **never** silently drops to `draft`. `review_pending` is true while any of
three independent triggers stands:

| trigger | set by | cleared by |
|---|---|---|
| a baselined dependency's content changed underneath it — `mirrors`, `rests_on`, or a claim-valued `governed_by.type` | `dossierx check`, from a stored hash | `dossierx claim reaudit <id> --confirm --reason "..."` |
| shipped code no longer matches the claim | `dossierx claim flag` (see **[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**) | the same confirmed reaudit |
| an open comment thread on the claim | anyone commenting (see **[`dossierx-comments`](../dossierx-comments/SKILL.md)**) | the **human** resolving it in the viewer |

It is set automatically and never cleared automatically: it clears only once *every* standing
trigger is gone. `unlock` also clears it, by leaving the locked state entirely.

**`dossierx claim reaudit` is the drift tool, not the general edit tool.** It refuses any claim
that is not already locked **and** `review_pending` (`not_review_pending`, exit 2), its
dependency-drift proposal is a no-change stub today (treat any content diff there as
illustrative), and it rewrites `body` and nothing else. Any other change to a locked claim — new
information, better wording, a `rows` fix, a structural change — is unlock → fix → lock.

When reaudit *is* right: run it bare first (a preview; writes nothing, renders the before/after as
a diff), **show the human the diff and wait**, then `--confirm --reason "<their words>"`. On
rejection do nothing — the claim stays `locked, review_pending`, and you never clear a flag by
hand. Reaudit refuses a claim whose *only* trigger is an open thread: no diff to confirm, so
resolve the conversation instead.

## Integrity — the ledger sees hand edits

Every legitimate approval records a hash of what was approved. `dossierx check` (and `--validate`,
and `--staged`, which the pre-commit hook runs) compares the world against that ledger and fails
with `integrity_failed` on: a locked claim with no record or with a **deleted** one
(`lock-ledger-deleted`), a locked claim whose content moved, a draft claim still holding a record
(`locked` → `draft` to dodge review), a locked claim whose **file** was deleted while its record
stands (no `claim delete` verb exists — `unlock` first), or a comment block changed outside the
engine. **Branch on `rule` inside `data.ledger_findings`, not on the code** — the router's
`integrity_failed` row says which rule means what, and one of them is not tampering.

The recovery is never "re-lock it so the hashes match" — that launders the edit. Restore from
version control, or go unlock → fix → lock. CI is the authority; the hook is only fast feedback.

The three project-root stores are tracked, committed artifacts, never `.gitignore`d, and
`.dossierx-flag-store.json` is the one to watch: **no gate rule reads it at all**, so losing it is
silent. The claim still arrives `review_pending`, but `reaudit` has no before/after to propose, and
confirming that empty proposal clears the human's flag having applied nothing. Commit it with its
claim, and treat an empty `reaudit` diff on a flagged claim as a missing entry: **stop and say
so**, do not confirm.

## Portability

Facets, modules, claims dir, source dirs, doctrine facet and template overrides all come from
`project.config.yaml` — never patch the engine.
