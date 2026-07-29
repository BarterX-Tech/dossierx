---
name: dossierx-claims
description: >-
  Authoring, inspecting and moving claims through their lifecycle in a DossierX
  project. Use this WHENEVER you are about to create a claim, edit one, find
  the claim a human described in words, run dossierx check, or lock, unlock,
  flag or reaudit anything under a project's claims/ directory. Covers the
  claim schema and id grammar, dossierx claim new, the read-only authoring loop
  (dossierx check --validate), dossierx claim show and list, the three
  review_pending triggers, and the one rule everything else hangs off — draft
  claims are free, a locked claim only ever changes via unlock, fix, lock.
  Load the DossierX router skill first; it carries the envelope, the exit
  codes and the error-code recovery table this skill assumes.
---

# DossierX claims — authoring and lifecycle

Read **[[dossierx]]** first: the envelope, the exit codes, the `error.code` recovery table and
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
| freeze a claim, on the human's word | `dossierx claim lock <id> --reason "<their words>"` |
| change a locked claim | `dossierx claim unlock <id> --reason "..."` → edit → `dossierx claim lock <id> --reason "..."` |
| a locked claim drifted from a changed dependency | `dossierx claim reaudit <id>` (preview) then `--confirm --reason "..."` |

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
  paragraphs, fenced code, backslash escapes, code spans, links, lists (with task items), thematic
  breaks, headings (`###`–`######` only — `#`/`##` render as literal text), and one level of
  blockquote. Every `rows` cell gets the narrower **inline-only** ceiling — code spans and links,
  no block constructs. In both, `[text](url)` links render with an allowlisted scheme; bare URLs
  are not autolinked; `javascript:`/`data:` URLs are neutralized. See FORMAT.md's markdown-ceiling
  section for the full two-ceiling table — do not reach past either one.
- `layout: card | table | list | steps | tree | banner | mockup` — inferred from shape if omitted.
  Be explicit once a claim is non-trivial.
- `build_role: orientation | schema | behavior | api | verification | out-of-scope` — **required
  before a claim can lock** once a module uses the feature. It orders implementation (see
  **[[dossierx-build-order]]**) and has nothing to do with `section`/`order`, which are the
  human's reading order in the viewer.
- Edges: `mirrors` (value equality; both sides must declare it), `rests_on` (semantic dependency;
  the target must exist and must not be an unmigrated module), `governed_by: {type, reason}` —
  `reason` is required when `type: none`.
- `kind: orientation-note` (implied by the reserved `overview` facet) marks a claim that tells a
  reader how to read the *rest* of the module. It renders as a banner and sorts ahead of fact
  claims, so "read top to bottom" already means "read the orientation notes first".

## Authoring — `dossierx claim new`, not a text editor

Hand-writing claim YAML is the thing this design gates. Author through the command: it enforces
the id grammar, the body requirement and the governed-reason rule **before** it writes, then lints
the project with the new claim in it.

```json
{"ok": true, "command": "claim new", "data": {
  "claim_id": "widget.contract.retry-policy", "path": ".../claims/widget.contract.retry-policy.yaml",
  "facet": "contract", "module": "widget", "status": "draft", "layout": "card",
  "lint_error_count": 0, "lint_warning_count": 1},
 "warnings": ["[warning] orphan: widget.contract.retry-policy: claim has no mirrors/rests_on edges in either direction"]}
```

`--rests-on` / `--mirrors` / `--governed-by` / `--build-role` / `--section` / `--layout` are all
available at creation time; `--file` may only name a path **inside** `claims_dir` (the loader walks
nothing else, so a claim written outside it reports success and is then invisible). After creation
the claim is a **draft** — edit its file freely.

The loop while authoring is `dossierx check --validate`: the same lint gate `check` drives, at the
same severity, writing **nothing** — no claim files, no lock store, no `.catalog.json`, no viewer.
Run the full `dossierx check` when you want the viewer rebuilt and code links scanned.

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
path uses, so it can never disagree with what the command would do. Read it rather than
re-deriving the lifecycle:

```json
"next_actions": ["1 open comment thread(s) block locking -> the human resolves them in the viewer; that click is the approval"]
```

## Locked means locked

A draft claim is yours. A locked claim is the human's, and the **only** path through it is:

```
dossierx claim unlock <id> --reason "<their words>"   →   edit the file   →   dossierx claim lock <id> --reason "<their words>"
```

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
that is *already* `locked` is not re-locked. That last one matters most when a gate has just
complained: re-locking a drifted or flagged claim would sign whatever the file now says, clear
`review_pending` with no diff shown, and strand the human's flag where `reaudit` can no longer
reach it. `unlock` → fix → `lock`, or restore the file from git.

## `review_pending` — and why `reaudit` is not the edit tool

A locked claim's status **never** silently drops to `draft`. `review_pending` is true while any of
three independent triggers stands:

| trigger | set by | cleared by |
|---|---|---|
| a dependency's content changed underneath it | `dossierx check`, from a stored hash | `dossierx claim reaudit <id> --confirm --reason "..."` |
| shipped code no longer matches the claim | `dossierx claim flag` (see **[[dossierx-code-links]]**) | the same confirmed reaudit |
| an open comment thread on the claim | anyone commenting (see **[[dossierx-comments]]**) | the **human** resolving it in the viewer |

It is set automatically and never cleared automatically: it clears only once *every* standing
trigger is gone. `unlock` also clears it, by leaving the locked state entirely.

**`dossierx claim reaudit` is the drift tool, not the general edit tool.** It refuses any claim
that is not already locked **and** `review_pending` (`not_review_pending`, exit 2), its
dependency-drift proposal is a no-change stub today (treat any content diff there as
illustrative), and it rewrites `body` and nothing else. Any other change to a locked claim — new
information, better wording, a `rows` fix, a structural change — is unlock → fix → lock, and the
refusal's own `error.hint` spells out both commands with the id substituted.

When reaudit *is* right: run it bare first (a preview; writes nothing, renders the before/after as
a diff), **show the human the diff and wait**, then `--confirm --reason "<their words>"`. On
rejection do nothing — the claim stays `locked, review_pending`, and you never clear a flag by
hand. Reaudit refuses a claim whose *only* trigger is an open thread: no diff to confirm, so
resolve the conversation instead.

## Integrity — the ledger sees hand edits

**DossierX detects; the forge enforces.** The ledger turns a silent edit into a named finding you
can act on; branch protection and a required CI check are what make anyone obey it. It judges the
tree in front of it — no git history, same verdict in every clone.

Every legitimate approval records a hash of what was approved. `dossierx check` (and `--validate`,
and `--staged`, which the pre-commit hook runs) compares the world against that ledger and fails
with `integrity_failed` on: a locked claim with no record or with a **deleted** one
(`lock-ledger-deleted`), a locked claim whose content moved, a draft claim still holding a record
(`locked` → `draft` to dodge review), a locked claim whose **file** was deleted while its record
stands (no `claim delete` verb exists — `unlock` first), or a comment block changed outside the
engine.

**Branch on `rule` inside `data.ledger_findings`, not on the code** — one is not tampering.
`lock-ledger-adoption-required` means the project predates the ledger and was never adopted:
adoption fails closed, and the fix is a human running `dossierx migrate --adopt` once (see
**[[dossierx]]**). Do not confuse it with `lock-ledger-absent`, which means the ledger file is
**gone** while locked claims remain — the two are told apart by the store itself, not by history.
To move `claims_dir` legitimately, move the claims and the stores in the **same** commit, claim
files byte-identical; that passes because every locked claim still resolves to its record.

Everywhere else the recovery is never "re-lock it so the hashes match" — that launders the edit.
Restore from version control, or go unlock → fix → lock. CI is the authority; the hook is only
fast feedback.

**And know what a clean `check` does and does not prove.** It proves nothing was changed *out of
step*: the untouched files still agree with the touched one. It cannot prove a claim and its ledger
record were not rewritten **together** — an in-repo ledger cannot attest anything against the person
who can write it, and a re-lock mints a record whose hash is correctly that of the new content, while
`reason`, `at` and `actor` are prose nothing checks. So never report `ok: true` as "nobody tampered",
and never propose an edit that touches a locked claim and its record in the same breath. FORMAT.md
states the principle; the diff and a required CI check are where it is caught.

**Three project-root files are tracked, committed artifacts**, beside `project.config.yaml` and
never `.gitignore`d: `.dossierx-lock-store.json` (the ledger; missing → `lock-ledger-absent`),
`.dossierx-comment-digest.json` (review history's fingerprint; missing → `comment-digest-absent`),
`.dossierx-flag-store.json` (each flagged claim's pending `{claim_says, now_does, reason,
flagged_at}`).

The flag store is the one to watch, because **no gate rule reads it at all**: losing it is silent.
The claim still arrives `review_pending`, but `reaudit` has no before/after to propose, and
confirming that empty proposal clears the human's flag having applied nothing. Commit it with its
claim, and treat an empty `reaudit` diff on a flagged claim as a missing entry: **stop and say
so**, do not confirm.

## Portability

DossierX takes zero project-specific behavior from its own source: facets, modules, claims dir,
source dirs, doctrine facet and template overrides all come from `project.config.yaml`. Adding
DossierX to a project is that file plus a `claims/` directory — never patch the engine.
