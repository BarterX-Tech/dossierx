---
name: dossierx-modules
description: >-
  Working a DossierX project one module at a time, and every size cap the engine
  enforces. Use this WHENEVER you orient in a module, run dossierx manifest show
  (--isolation or --integration) or dossierx manifest list, draft or fix a
  module's claims/<module>/manifest.yaml (summary, provides, depends_on), pick a
  neighbor's contract claim to rest on, or meet a cap — module-claim-cap,
  summary-required, summary-oversize, body-oversize, module-manifest,
  shared-context-budget, view_too_large, CONSTITUTION_OVER_CAP. Covers the reading
  order, the isolation and integration views and their budgets, manifest drafting,
  and the loud recovery for each cap: split or trim by default, and raise a config
  value only on the human's explicit yes. Load the DossierX router skill first.
---

# DossierX modules — the harness, the manifest, and the caps

Read **[`dossierx`](../dossierx/SKILL.md)** first for the envelope and the error codes, and
**[`dossierx-claims`](../dossierx-claims/SKILL.md)** for what a claim is and how to write one.

A module is the unit an agent works in. The caps exist so that one module's whole context — the
roof, the project claims index, the manifest and every claim summary — fits in one bounded read.

## The reading order

**Read in this order and stop as early as you can:**

1. the constitution, 2. the project claims index, 3. this module's manifest, 4. its claim
   summaries — all four are one `dossierx manifest show <module> --isolation`;
5. neighbors: `dossierx manifest show <module> --integration`; the module catalog is
   `dossierx manifest list`;
6. `dossierx claim show <id>` — one body — only when a summary is not enough.

A summary is always the claim's authored `summary` field; no view prints bodies. **Never walk the
claims tree or open claim files to orient yourself.** Other modules read `contract` only: a foreign
module's `*.internals.*` is never read, cited or exported — hard law, not style.

## The three views

| command | carries | bound |
|---|---|---|
| `manifest show <module> --isolation` | `shared.constitution_text`, `shared.project_claims` (id, summary, status), `manifest`, `claims` (id, title, facet, status, summary), `draft_hints.suggested_provides` | 10240 bytes shared + 6144 bytes module, reported in `data.isolation_budget` |
| `manifest show <module> --integration` | for each module your `depends_on` names: its manifest summary, its `provides` ids with each claim's summary; the `depends_on` edges; the project claims index | one hop, no byte cap: your `depends_on` bounds it |
| `manifest list` | every configured module: summary, `provides` / `depends_on` counts, claim and locked counts, findings | summaries only |

Every `manifest show` also carries `constitution_digest` (path, words, hash, lock state).
`--integration` never follows a neighbor's own `depends_on` and never shows internals.

## `manifest.yaml` — you draft it

`claims/<module>/manifest.yaml` is required, one per module, and is **not a claim**. The CLI never
drafts it: `dossierx claim new` writes a stub whose empty `summary` fails until you write one. For a
module with no file at all, `manifest show <module> --isolation` exits 1 (`lint_failed`) and still
returns `data.isolation` — the claim summaries and `draft_hints` — to draft from.

```yaml
summary: widget is the public boundary other modules call; start with retry-policy.
provides:
  - widget.contract.retry-policy
depends_on:
  - lock.contract.store
```

- `summary` — the module's why / start here, plus a short note on how neighbors or the product use
  it. At most **280 characters**. Never paste claim bodies.
- `provides` — this module's own `contract` ids that other modules may pin. Start from
  `draft_hints.suggested_provides` and drop any a neighbor should not build on. A contract claim
  left out is still readable and citable; it is only not pinnable.
- `depends_on` — **other** modules' contract ids this module consumes, each listed in that
  provider's `provides`. Never your own ids, never `project.*`, never the constitution.
- One YAML document, no unknown keys, at most **4096 bytes**, only at that exact path.

Then `dossierx check --validate`. Any defect is `module-manifest`: `check`, `manifest show` and
`claim lock` of **every claim in that module** refuse until it is fixed. The lists are not `rests_on`
edges: module cycles are legal, nothing sets `review_pending` on a manifest, and the manifest has no
lock of its own — the human reviews it in the viewer's Manifest tab and approves the module through
its claim locks. Redraft it when a neighbor needs a contract id you have not exported yet, or when
your claims change what the module is for.

## Caps are loud, and raising one is the human's call

Every cap refuses; none truncates. **When one fires, report the finding to the human in its own
words and take the default recovery below.** Some lint messages end "or raise `max_…` in
project.config.yaml": that option exists, and it is **the human's decision, never your default**. If
you think a raise is right, ask, say what it costs (a bigger module no longer fits its 6144-byte
isolation budget, and every reader pays for the extra context), and **wait for an explicit yes**.
Never raise a value in the same change that hits the cap, and never pad, merge or cram to fit.

| cap | refusal | default recovery | config value |
|---|---|---|---|
| 10 claims per module, drafts included | `module-claim-cap` on every claim of the module | retire a claim the rubric in `dossierx-claims` would not keep, or split the module | `max_claims_per_module` |
| summary: one plain line, 200 characters | `summary-required` / `summary-oversize` | rewrite it as one standalone assertion; if it will not fit, it is two claims | `max_claim_summary_chars` |
| `body` + `steps` + `rows` cells: 2000 characters (`raw_html` exempt) | `body-oversize` | cut the walkthrough, move evidence to `sources`, or split into two facts | `max_claim_body_chars` |
| manifest: 4096 bytes, summary 280 | `module-manifest` | trim the summary to the why; drop ids nobody pins | none |
| isolation, module part: 6144 bytes | `view_too_large` from `manifest show --isolation` | shorten claim summaries or the manifest, or split the module | none |
| isolation, shared part: 10240 bytes | `shared-context-budget` on the project claim that crosses it (project-wide if the constitution alone does) | shorten project claim summaries, retire project claims, or trim the constitution | none |
| constitution: 800 words (warning from 720) | `CONSTITUTION_OVER_CAP` / `constitution-near-cap` | trim it with the human — `dossierx-constitution` | none |

Characters are Unicode code points. The lint caps block `check` and `claim lock` like any ERROR;
`view_too_large` refuses only the view. Caps apply to drafts and locked claims alike, so a locked
claim can start failing when a sibling draft pushes the module over.

**Splitting a module** is a structural change the human should agree to: a new entry in
`modules:` in project.config.yaml, a new `manifest.yaml`, and claims re-authored under the new
module's ids. Drafts move freely. A **locked** claim's id changes when it moves, so it is
`claim unlock` → re-author under the new id → `claim lock` with the human's approval, the old file deleted
only once unlocked, and every `rests_on` and manifest id naming the old id updated in the same pass.
**Retiring** a draft is deleting its file; a locked one is unlocked first, with the human's yes.
