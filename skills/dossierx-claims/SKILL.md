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
  list/show/status, how to write a claim worth keeping (one fact, a summary
  that stands alone, contract vs internals, choosing rests_on), the three
  review_pending triggers, and the rule a locked claim hangs off — draft claims
  are free; body-only meaning drift is claim flag, and every other edit is unlock, fix, lock.
  Load the DossierX router skill first; it carries the envelope, the exit
  codes and the error-code recovery table this skill assumes.
---

# DossierX claims — authoring and lifecycle

Read **[`dossierx`](../dossierx/SKILL.md)** first: the envelope, the exit codes, the `error.code` recovery table and
the five rules are there and are not repeated here.

## The contract, in one table

| you want to | run |
|---|---|
| author a claim | `dossierx claim new <id> --summary "..." --body "..." --rests-on-none-reason "..."` |
| check your work, writing nothing | `dossierx check --validate` |
| build everything (catalog, viewer, code-link scan, ledger gate) | `dossierx check` |
| know everything about one claim | `dossierx claim show <id>` |
| find the claim a human described | `dossierx claim list --match "<their words>"` |
| what is pending / migrated / drifted | `dossierx claim list --review-pending` · `--migrated` · `--drifted` |
| freeze a claim, on the human's word | `dossierx claim lock <id> --dry-run`, then `--proposal "<snapshot>" --reason "<their words>"` |
| change a locked claim | `dossierx claim unlock <id> --reason "..."` → edit → `dossierx claim lock <id> --dry-run`, then `--proposal "<snapshot>" --reason "..."` |
| a locked claim drifted from a changed dependency | `dossierx claim reaudit <id>` (preview) then `--confirm --reason "..."` |
| read the feature axis (read-only; never a gate, never a build sequence) | `dossierx track list` · `show <id>` · `status <id>` — see Feature tracks below |

## Is this worth a claim? — and how to write one

Write a claim only when all three are **yes**: (1) would a competent engineer reading the code be
surprised by it? (2) if it were wrong, would another module or a locked promise break? (3) is it
invisible from any single file? Never: language semantics, framework defaults, restating what the
code says, step-by-step narration of one function, or anything the constitution already states. The
caps (10 claims, 200-character summary, 2,000-character body) are the backstop; this rubric is what
makes a module converge on ten claims rather than ten longer ones. A cap finding is never a reason to
pad or merge — see **[`dossierx-modules`](../dossierx-modules/SKILL.md)** for what to do instead.

**One fact per claim.** The human approves or rejects a claim as a unit, and a drifted dependency
flags all of it. If the summary needs "and" to be true, it is two claims — or one of them is not worth
writing. Never merge unrelated facts to stay under the module cap.

**The summary stands alone.** In `dossierx manifest show <module> --isolation`, in a neighbor's
`--integration` and in `claim list`, the summary is all anyone reads; the body is opened only with
`claim show`. Write the assertion itself, not a title: "Retries stop after 3 attempts, doubling from
1s and capped at 30s", not "Retry policy" or "How retries work". Name the subject, the rule and the
number that bounds it. No "see below", no "this claim", no `[n]` marker, no markdown — one plain line.

**The body carries what the summary could not:** the conditions, the exceptions, what happens on
failure, and why. Cite the evidence with `sources`. It is not a walkthrough of the code.

**`contract` or `internals`?** `contract` is what another module may rely on — a boundary guarantee,
a data shape, an error it will see. Other modules may cite it and your manifest may export it.
`internals` is how this module keeps its promises; only this module cites it, and it is never
exported. Ask: if this changed, would another module's code or a locked promise have to change? Yes
is `contract`. Decide before locking: the facet is part of the id, so moving a claim later is a new id,
a new lock, and every `rests_on` naming the old id rewritten.

**Choosing `rests_on`.** Name a target only when your claim stops being true if the target
changes — every edge is a future `review_pending` on your claim when the target moves. "Related" is
not an edge; mention it in prose. Too many edges bury the human in reviews; a missing one lets a claim
go silently false. Pick a neighbor's target from its provided summaries in `manifest show <module>
--integration`, not by opening its files. A claim that genuinely depends on no other claim says so:
`{none: true, reason: "..."}` with a reason a reviewer can check, not "n/a".

## A claim

One YAML document per file. A second `---` document in the same file is a hard load error — split
it out.

- `id: module.FACET.slug` — **exactly three** non-empty dot-separated segments. `module` must be
  one the project declares; `FACET` is **engine-fixed**: only `contract` or `internals`. `slug` must be kebab-case
  (lowercase alphanumerics, single hyphens). The viewer's card title is derived from the slug, so
  `retry-policy` renders as "Retry Policy" — you never write a title.
- `status: draft | locked` — **only** `dossierx claim lock` / `unlock` may change this. Editing it
  by hand walks past the lint gate, the roof gate and the open-thread gate as though all three
  had passed, and the lock ledger will report it as `integrity_failed` on the next check.
- `summary` — **required** on every claim, project claims too: one plain-text line ≤ `max_claim_summary_chars` (200), else `summary-required`/`-oversize`.
- `body` (prose) and/or `rows` (a table; every cell an authored **string** — quote numbers and
  booleans). A claim needs at least one. `body` takes block markdown (headings `###`–`######` only;
  images only from the claim's own `assets/`); a `rows` cell is inline-only. FORMAT.md's "`body` and
  the markdown ceiling" has the full account. `body`+`steps`+`rows` share `max_claim_body_chars`
  (2000 code points; `raw_html` exempt); over is `body-oversize`.
- `layout: card | table | list | steps | tree | banner | mockup` — inferred from shape if omitted.
  Be explicit once a claim is non-trivial.
- `section` / `order` — optional; the human's reading order in the viewer, nothing more.
- Edges: required `rests_on` — either a list of claim ids or `{none: true, reason: "..."}`. Targets
  are claim ids only: `project.<slug>`, any module's `*.contract.*`, and this module's own
  `*.internals.*`; a foreign module's `*.internals.*` is refused (`rests-on-target`). Never the
  constitution — it is not a target, not a node, not a ref grammar. Targets are **drift** edges (a
  target's content changing under a locked claim flags `review_pending`). A claim file carrying a
  retired key (`governed_by`, `build_role`, `mirrors`) fails to load — **[`dossierx-upgrading`](../dossierx-upgrading/SKILL.md)**. **A `rests_on` loop is refused** at ERROR (`cycle`), and so is naming yourself (`self-edge`).
- **Facets are hard law:** exactly `contract` and `internals`. Another module may cite only `contract`;
  foreign `internals` is refused (`rests-on-target`) and never exported (catalog, integration).
- **Module context is `claims/<module>/manifest.yaml`** — required YAML, not a claim, which **you
  draft**; `claim new` leaves a stub that fails until you do. How, and every cap: **[`dossierx-modules`](../dossierx-modules/SKILL.md)**.
- `kind` — optional; omit it or set `fact`. Any other value is refused (`kind-shape`).
- `sources` — optional, the evidence behind the claim, cited from `body` as `[1]`, `[2]`. See
  **Citing your evidence** below.
- `tracks` — optional, cross-cutting feature membership: `- {id: checkout, role: owns|cites}`,
  role defaulting to `cites`. See **Feature tracks** below.

## Authoring — `dossierx claim new`, not a text editor

Hand-writing claim YAML is the thing this design gates. Author through the command: it enforces
the id grammar, the body and `--summary` requirements and the required `rests_on` rule **before**
it writes, then lints the project with the new claim in it — an `orphan` warning on a claim with no
edges yet is a warning, not a refusal.

`--rests-on` / `--rests-on-none-reason` / `--section` / `--layout` are all
available at creation time; `--file` may only name a path **inside** `claims_dir` (the loader walks
nothing else, so a claim written outside it reports success and is then invisible). After creation
the claim is a **draft** — edit its file freely.

The loop while authoring is `dossierx check --validate`: the same lint gate `check` drives, at the
same severity, writing **nothing** — no claim files, no lock store, no `build/catalog/catalog.json`, no viewer.
Run the full `dossierx check` when you want the viewer rebuilt and code links scanned; only that
run proves a locked claim is linked to code (`--validate` reports `code_links` but never gates on it).
Report the envelope, never your belief: "it is synced" in chat is not a certificate, and an exit code
you did not see is one you do not have — stop and say what is missing.

## Project claims, the constitution, and the reading order

A `project.<slug>` claim (`project-claims/<slug>.yaml`) is an ordinary claim — this skill's rules
apply — with no module, no facet and no cap. The constitution is not a claim at all. When to use
which, and how the roof is drafted and locked: **[`dossierx-constitution`](../dossierx-constitution/SKILL.md)**.

Orient one module at a time through `dossierx manifest show <module> --isolation`; never walk the
claims tree or open claim files to orient yourself. The full reading order is in
**[`dossierx-modules`](../dossierx-modules/SKILL.md)**. Other modules read `contract` only.

## Citing your evidence — `sources`

`sources` records **what makes this claim true**, in the claim, where `check`, the viewer and the
lock ledger can all see it (`migrated_from` instead answers *what this claim replaced*). Never keep
evidence in a sidecar file: nothing checks it, so it could be rewritten after the human approved.

```yaml
sources:
  - {ref: 1, kind: external, title: SCShareableContent, url: https://developer.apple.com/documentation/screencapturekit/scshareablecontent, accessed_on: 2026-08-15}
  - {ref: 2, kind: internal, title: Product requirement PVR-010, path: migration/product-requirement-map.jsonl, record_id: PVR-010, sha256: 8afd3c9a...}
```

An `external` source needs `url` **and** `accessed_on` — the date records what the page said on the
day it was read. An `internal` source needs `path` (relative to `project.config.yaml`) **and**
`sha256`, and may set `record_id` to pin one JSONL record (matched on its top-level `"id"`) rather
than the whole file, which churns for reasons unrelated to your claim.

Cite from `body` with `[n]`: *"frames arrive only while the stream is running [1]."* Markers are
read **in prose only** — never inside a fenced block or an inline `` `code` `` span — and **only on
a claim that declares `sources`**, so a source-less claim writing `array[0]` is unaffected.

Lints: `source-shape`, `source-ref-undefined` (a `[n]` no entry declares), `source-external-unanchored`
and `source-internal-drift` (hash missing, unreadable or moved — **a check that cannot run is a
failure, not a pass**) are ERROR; `source-ref-unused` (an entry nothing cites) is a WARNING.

**`sources` is signed by the lock ledger and is not part of the dependency-drift hash.** Editing a
citation under a locked claim is `lock-content-drift`, exactly like editing the body — so the path
is `unlock → fix → lock`. But adding or correcting a citation never flips a dependent to
`review_pending`: provenance is not contract.

## Feature tracks — the second ownership axis

`module` says **who guarantees this**; a track says **what the user gets, and whether it is
finished**. Declare the vocabulary in `project.config.yaml` (`tracks: [{id, title, summary}]`),
then name them from a claim's own `tracks:` list, one `{id, role}` entry each.

**One owner per axis.** Exactly one `module`, and **at most one** claim owning a track (`owns`);
every other membership is `cites` — a reference, never a copy. Two owners is `track-multi-owner`
(ERROR). Membership is **not an edge**. Other lints: `track-shape` and `track-unknown` (ERROR),
`track-empty` and `track-unowned` (WARNING).

**Three verbs, all read-only.** `dossierx track list` names the tracks the project declares;
`dossierx track show <id>` reads a feature end to end, assembled across modules; `dossierx track
status <id>` REPORTS whether it is finished: COMPLETE when every claim the track owns and every claim
it cites is locked. **Never treat a track as a gate.** It does not block anything, and track
membership never gates `dossierx claim lock` — a claim locks on its own merits, and the way to
change a locked claim's `tracks` is `unlock → fix → lock`, since `tracks` is signed by the ledger
like every other field. "Is the feature done?" is `track status`. What to implement next is
locked claims, module `depends_on`, and claim `rests_on`.

## Finding the claim the human meant

They will say "the retry card in contract". That is not an id, and guessing costs a `claim_not_found` — or worse, acts on the wrong claim. Run `dossierx claim list --match "retry" [--facet contract] [--module widget]`.

Each row carries `claim_id`, `title`, `status`, `review_pending`, `drifted`, `open_threads` and a `score` — a ranked ladder over the id and derived title, so a confident hit sits well above a tie. **Name the winner and its title back to the human and wait** before running anything that writes.

## `dossierx claim show` — one call, the whole picture

Prefer it over reading the YAML. It reports status, lock state and `locked_at`, `review_pending` plus **which** trigger caused it, both edge directions (`rests_on` outgoing, `depended_on_by` incoming), `implemented_in[]` with per-file drift, comment counts with the open thread ids, and `next_actions` — computed from the *same* gate evaluation the write path uses, so it can never disagree with what the command would do. Read it rather than re-deriving the lifecycle yourself.

## Locked means locked

A draft claim is yours. A locked claim is the human's. Body-only meaning drift is `claim flag` (see the table under `review_pending`), not unlock. Every other change is `dossierx claim unlock <id> --reason "<their words>"` → edit the file → `dossierx claim lock <id> --dry-run`, then `--proposal "<snapshot>" --reason "<their words>"`.

Both ends require `--reason` and take `--dry-run`. Preview, show the human the `side_effects` (locking records a content baseline; unlocking releases it and can flip dependents), get a yes, then run it. `--reason` carries their approval into the record — never fabricate one.

The window between the two ends is not a steady state. If any source file carries a
`dossierx-claim:` or `dossierx-step:` tag for that id, a plain `dossierx check` mid-edit fails with `implink_refused`
and `claim is not locked (status "draft")` — the tag is fine, the claim is mid-edit. Finish the
relock; never touch the tag or leave the claim unlocked to silence it.

`dossierx claim lock` refuses on `lint_failed` (fix the findings), `unresolved_comments` (reply;
the human clicks Resolve) and `already_locked` — re-locking would sign whatever the file now says and
clear `review_pending` with no diff shown. `unlock` → fix → `lock`, or restore the file from git.

## `review_pending` — and why `reaudit` is not the edit tool

A locked claim's status **never** silently drops to `draft`. `review_pending` is true while any of
three independent triggers stands:

| trigger | set by | cleared by |
|---|---|---|
| a baselined `rests_on` dependency's content changed underneath it | `dossierx check`, from a stored hash | `dossierx claim reaudit <id> --confirm --reason "..."` |
| shipped code no longer matches the claim | `dossierx claim flag` — body-only claims; one that renders from `rows`/`steps`/`raw_html`/`mockup` is refused (`structured_layout`) and goes through unlock → fix → lock instead (see **[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**) | the same confirmed reaudit |
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

Every approval records a hash of what was approved. `dossierx check` (and `--validate`, and
`--staged`, which the pre-commit hook runs) fails with `integrity_failed` when a locked claim's
content, record, status or **file** moved outside the approval path (no `claim delete` verb exists —
`unlock` first), or a comment block changed outside the engine. **Branch on `rule` inside
`data.ledger_findings`** — the router's `integrity_failed` row says which rule means what. The
recovery is never "re-lock it so the hashes match" — that launders the edit. Restore from version
control, or go unlock → fix → lock. CI is the authority; the hook is only fast feedback.

The stores under `build/ledger/` are committed, never `.gitignore`d. `flag-store.json` has **no gate
rule behind it**: lost, the claim still arrives `review_pending` with an empty `reaudit` diff, and
confirming it clears the human's flag having applied nothing. Treat an empty diff on a flagged claim
as a missing entry: **stop and say so**, do not confirm.

## Portability

Modules, claims dir, source dirs and template overrides come from `project.config.yaml` — never patch the engine. Facets are engine-fixed (`contract`, `internals`); never invent a third.
