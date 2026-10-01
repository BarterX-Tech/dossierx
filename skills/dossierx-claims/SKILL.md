---
name: dossierx-claims
description: >-
  Authoring, inspecting and moving claims through their lifecycle in a DossierX
  project. Use this WHENEVER you are about to create a claim, edit one, find
  the claim a human described in words, run dossierx check, or lock, unlock,
  flag or reaudit anything under a project's claims/ directory. Covers what is
  worth a claim and what is a brief instead (design, voice, rationale,
  research), how to write one (one fact, a summary that stands alone, contract
  vs internals, choosing rests_on), the claim schema and id grammar, dossierx
  claim new, the read-only authoring loop (dossierx check --validate), claim
  show and list, citing evidence with sources and [n] markers, the three
  review_pending triggers, and the rule a locked claim hangs off — draft claims
  are free; body-only meaning drift is claim flag, every other edit is unlock,
  fix, lock. Load the DossierX router skill first; it carries the envelope, the
  exit codes and the error-code recovery table this skill assumes.
---

# DossierX claims — authoring and lifecycle

Read **[`dossierx`](../dossierx/SKILL.md)** first: the envelope, the exit codes, the `error.code`
recovery table and the five rules are there and are not repeated here.

## The contract, in one table

| you want to | run |
|---|---|
| author a claim | `dossierx claim new <id> --summary "..." --body "..." --rests-on <ids>` (or `--rests-on-none-reason "..."`) |
| check your work, writing nothing | `dossierx check --validate` |
| build everything (catalog, viewer, code-link scan, ledger gate) | `dossierx check` |
| know everything about one claim | `dossierx claim show <id>` |
| find the claim a human described | `dossierx claim list --match "<their words>"` |
| what is pending / drifted | `dossierx claim list --review-pending` · `--drifted` |
| freeze a claim, on the human's word | `dossierx claim lock <id> --dry-run`, then `--proposal "<snapshot>" --reason "<their words>"` |
| change a locked claim | `dossierx claim unlock <id> --reason "..."` → edit → `claim lock <id> --dry-run` → `--proposal "<snapshot>" --reason "..."` |
| a locked claim is `review_pending` from drift or a flag | `dossierx claim reaudit <id>` (preview), then `--confirm --reason "..."` |

## Is this worth a claim?

**First, what kind of statement is it?** How the product should look, sound or feel, why a choice
was made, or what was learned — design, voice, rationale, research — is not a claim: write a brief
(**[`dossierx-briefs`](../dossierx-briefs/SKILL.md)**). What the product *does* — a guarantee about
UI behaviour included — goes to three questions.

Write a claim only when all three are **yes**: (1) would a competent engineer reading the code be
surprised by it? (2) if it were wrong, would another module or a locked promise break? (3) is it
invisible from any single file? A hard UI guarantee that lives in one component file fails (3) and
becomes a brief; that cost is accepted.

Never a claim: language semantics, framework defaults, restating what the code says, step-by-step
narration of one function, anything the constitution already states, statements about the module
or its own claims, anything that needs an image to make sense. Red flags: a body opening "This
module…", something nobody can falsify, a list of examples, something that changes weekly, a
restatement of another claim.

| the content is… | its home |
|---|---|
| a boundary guarantee another module may rely on: a data shape, an error it will see | a `contract` claim |
| how this module keeps its promises | an `internals` claim |
| law every module builds toward; a project-wide fact with no module | the constitution, or a `project.<slug>` claim (`dossierx-constitution`) |
| how it looks, sounds or feels; why; what was learned; what a feature is for the user | a brief — a feature is a brief in `briefs/features/` (`dossierx-briefs`) |
| what a module is for, where to start in it | its `manifest.yaml` summary (`dossierx-modules`) |
| a question, or a disagreement with a claim's wording | a comment on that claim (`dossierx-comments`) |
| a token value: a colour, a spacing step, a font size | code, never a claim or a brief |

**Before writing UI, copy, design or rationale, run `dossierx brief list`** and read what applies.
The caps (10 claims per module, 200-character summary, 2,000-character body) are the backstop;
this rubric is what makes a module converge on ten claims rather than ten longer ones. A cap finding
is never a reason to pad or merge — **[`dossierx-modules`](../dossierx-modules/SKILL.md)**.

## How to write one

**One fact per claim.** The human approves or rejects a claim as a unit, and a drifted dependency
flags all of it. If the summary needs "and" to be true, it is two claims — or one of them is not
worth writing.

**The summary stands alone.** In `manifest show --isolation`, a neighbor's `--integration` and
`claim list`, the summary is all anyone reads. Write the assertion, not a title: "Retries stop after
3 attempts, doubling from 1s and capped at 30s", not "Retry policy". Name the subject, the rule and
the number that bounds it. One plain line: no markdown, no `[n]` marker, no "see below".

**The body carries what the summary could not:** conditions, exceptions, what happens on failure,
and why. Cite evidence with `sources`. It is not a walkthrough of the code.

**`contract` or `internals`?** If this changed, would another module's code or a locked promise have
to change? Yes is `contract`: other modules may cite it and your manifest may export it. Otherwise it
is `internals`: only this module cites it, and it is never exported. Decide before locking — the
facet is part of the id, so moving it later is a new id, a new lock, and every `rests_on` naming the
old id rewritten.

**Choosing `rests_on`.** Name a target only when your claim stops being true if the target changes:
every edge is a future `review_pending` on your claim. "Related" is not an edge: mention it by its title in prose. Writing another claim's **id**
in prose draws the `body-edge-hint` warning (name it in `rests_on`, or use the title), and an id in a
fenced code block that resolves to no claim is `code-orphan`, an error.
Pick a neighbor's target from `manifest show <module> --integration`, not by opening its files. A
claim that rests on nothing says so — `{none: true, reason: "..."}` — with a reason a reviewer can
check, not "n/a".

## The schema

One YAML document per file, under `claims_dir`; a second `---` document in the file is a load error.
A key the schema does not have fails to load (`invalid_claim`). FORMAT.md's "Claim" section is the
full account.

- `id: module.FACET.slug` — exactly three segments. `module` is one the config declares; `FACET` is
  `contract` or `internals`, nothing else; `slug` is kebab-case. The card title is derived from the
  slug (`retry-policy` → "Retry Policy"); you never write a title.
- `status: draft | locked` — **only** `claim lock` / `unlock` change it. A hand edit walks past every
  lock gate and the ledger reports it as `integrity_failed`.
- `summary` — required, one plain line, at most 200 characters (`summary-required` / `summary-oversize`).
- Content — **at least one** of `body` (markdown; headings `###`–`######` only, images only from the
  claim's own `assets/`, else `asset-scope`), `rows` (a table; every cell a quoted **string**, inline markdown only),
  `steps` (an ordered list of strings) or `raw_html`. None is `layout-shape-mismatch`. `body`,
  `steps` and `rows` cells share the 2,000-character cap (`body-oversize`); `raw_html` is exempt.
- `raw_html` — markup rendered beside the layout's own content, gated by `raw-html-scope`: only a
  module listed in the config's `mockup_modules` may carry it; only `div`, `span`, `b`, `br` and
  `img` tags, with a `class` attribute whose every token starts `gcp-` or `mockup-` (an `img` adds a
  same-origin `src` and an `alt`); and the claim cannot lock until `raw_html_reviewed: true` — the
  **human's** sign-off on that markup. Ask them to set it; never set it yourself.
- `layout: card | table | list | steps | tree | banner | mockup` — `rows` infers `table`, `steps`
  infers `steps`, anything else `card`; the other four are never inferred. Be explicit once a claim
  is non-trivial.
- `rests_on` — required: a list of ids, or `{none: true, reason: "..."}`. Targets are `project.<slug>`,
  any module's `*.contract.*`, and this module's own `*.internals.*`; a foreign module's internals is
  `rests-on-target`, a loop is `cycle`, naming yourself is `self-edge`. Never the constitution: it
  is not a target. Every edge is a **drift** edge.
- `sources` — optional evidence, cited from `body` as `[1]`, `[2]` (below).
- `embodiment` — optional: what the code must show (`mode: compare`) or that no code embodies the
  claim (`mode: none` with a `reason`). **[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**.
- `section` / `order` — optional reading order in the viewer, nothing more. `emphasis: true` renders
  a hard-boundary card. `kind` is omitted or `fact` (`kind-shape`).
- `comments`, `review_pending`, `audit_notes` — engine-managed: the comment verbs, the viewer and a
  confirmed reaudit write them, never you.

Module context is `claims/<module>/manifest.yaml` — not a claim, and **you draft it**
(**[`dossierx-modules`](../dossierx-modules/SKILL.md)**). A `project.<slug>` claim lives in
`project-claims/` with no module, facet or cap (**[`dossierx-constitution`](../dossierx-constitution/SKILL.md)**).

## Authoring — `dossierx claim new`, not a text editor

Author through the command: it enforces the id grammar, `--summary`, `--body` and the `rests_on`
rule before it writes, then lints the project with the new claim in it. **`ok: true` means written, not clean**: read
`data.lint_error_count` (a `rests_on` naming a foreign module's internals is written, then fails
lint) and run `check --validate`. An `orphan` warning on a claim with no edges yet is only a warning. The command needs `claims_dir` to exist, and
writes a stub `manifest.yaml` (empty summary, refused until you draft it) for a module that has none.

`--layout` takes `card`, `list`, `tree` or `banner`; a `table`, `steps` or `mockup` claim gets its
`rows`, `steps` or `raw_html` by editing the draft file afterwards. `--section` sets the in-content
heading it sits under, and `--file` is a path relative to `claims_dir`. Once created, the claim is a draft: edit its file freely.

The loop while authoring is `dossierx check --validate`: the same lint gate at the same severity,
writing **nothing**. Until the human has locked the constitution it exits 1
`CONSTITUTION_NOT_LOCKED` even when every claim is clean — that one is theirs to clear, not a defect
in your claims. Run plain `dossierx check` when you want the viewer rebuilt and code links
scanned; only that run proves a locked claim is linked to code. Report the envelope, never your belief: an exit code
you did not see is one you do not have.

## Citing your evidence — `sources`

`sources` records what makes the claim true, in the claim, where `check` and the lock ledger can see
it. Never keep evidence in a sidecar file: nothing checks it.

```yaml
sources:
  - {ref: 1, kind: external, title: "HTTP Semantics, section 15.6.4", url: https://www.rfc-editor.org/rfc/rfc9110#section-15.6.4, accessed_on: 2026-08-15}
  - {ref: 2, kind: internal, title: Load test results, path: research/load-test.jsonl, record_id: run-42, sha256: 8afd3c9a...}
```

`external` needs `url` and `accessed_on` (the date pins what the page said). `internal` needs `path`
(relative to `project.config.yaml`) and `sha256`: the hex sha256 of the whole file's bytes
(`shasum -a 256 <path>`). `record_id` narrows the pin to the one JSONL line whose top-level `"id"`
matches; the hash is then of that raw line, without its line terminator. Both kinds may add `supports` / `does_not_support` to say what the source
does and does not establish. An internal source on a **brief** pins the brief's content hash — the
`content:` line of `dossierx brief show <path>` — not the file's bytes.

Cite from `body` with `[n]` — "a 503 carries a Retry-After header [1]". Markers are read in prose
only, never in code, and only on a claim that declares `sources`. `source-shape`,
`source-ref-undefined`, `source-external-unanchored` and `source-internal-drift` (hash missing,
unreadable or moved — a check that cannot run is a failure) are errors; `source-ref-unused` is a
warning.

`sources` is signed by the lock ledger, so editing a citation under a locked claim is
`lock-content-drift` — unlock → fix → lock. It is not part of the dependency-drift hash, so it never
flips a dependent to `review_pending`.

## Finding the claim the human meant

"The retry card in contract" is not an id. Run `dossierx claim list --match "retry"` (add
`--facet` / `--module` to narrow). Rows carry `claim_id`, `title`, `status`, `review_pending`,
`drifted`, `open_threads` and a ranked `score`. **Name the winner and its title back to the human
and wait** before running anything that writes.

## `dossierx claim show` — one call, the whole picture

Prefer it to reading YAML: status, `locked_at`, `review_pending` and which trigger set it, both edge
directions (`edges.rests_on`, `edges.depended_on_by`), `implemented_in[]` with per-file drift, open
thread ids, `readiness`, and `next_actions` — computed from the same gate the write path uses, so it
cannot disagree with what a command would do.

## Locked means locked

A draft claim is yours; a locked claim is the human's. Body-only meaning drift is `claim flag`
— only when the summary still holds, because a confirmed flag replaces the **whole** body with
`--now-does` and never touches `summary` (**[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**).
Every other change is unlock → fix → lock: `claim unlock <id> --reason "<their words>"` → edit → `claim lock <id> --dry-run` → `claim lock
<id> --proposal "<snapshot>" --reason "<their words>"`. Preview each end, show the human the
`side_effects` (unlocking releases the approval, and claims resting on it show
`dependency_unapproved` until it re-locks), and get a yes.

Between the two ends, plain `dossierx check` fails `implink_refused` (`claim is not locked (status
"draft")`) on every `dossierx-claim:` / `dossierx-step:` tag for that id. The tag is right; the claim
is mid-edit. Finish the re-lock — never touch the tag, and never leave the claim unlocked to quiet it.

`claim lock` refuses on `lint_failed` (re-check with `claim lock <id> --dry-run`, not `check
--validate`), `unresolved_comments` (reply; the human resolves) and `already_locked` (a second
lock would sign whatever the file now says with no diff shown: unlock → fix → lock, or restore the
file from git if a gate reported drift).

**Deleting or renaming a claim.** There is no delete verb. A draft is deleted by removing its file —
unless it carries a comment thread, open or resolved: that is `comment-digest-abandoned`, and the
human deletes those threads in the viewer first. A locked claim is unlocked first, on the human's
yes. Renaming changes the id and counts as a delete plus a new claim. In the same change, fix every
`rests_on` that named it (`dangling`), every manifest `provides` / `depends_on`, and every brief's
`rests_on` (`brief-rests-on-missing`).

## `review_pending` — and why `reaudit` is not the edit tool

A locked claim never silently drops to `draft`. `review_pending` is true while any of three
triggers stands, and clears only when all are gone (or on `unlock`):

| trigger | set by | cleared by |
|---|---|---|
| a `rests_on` target's content changed since the lock | `dossierx check` | `claim reaudit <id> --confirm --reason "..."` |
| shipped code no longer matches the claim | `dossierx claim flag` (body-only claims) | the same confirmed reaudit |
| an open comment thread on the claim | anyone commenting (`dossierx-comments`) | the **human** resolving it in the viewer |

`claim reaudit` refuses a claim that is not locked and `review_pending` (`not_review_pending`), and
one whose only trigger is an open thread (`review_pending`, exit 2) — there is no diff to confirm. Its dependency-drift
proposal is a no-change stub, and it rewrites `body` and nothing else. Any other change — new
information, better wording, a `rows` fix — is unlock → fix → lock.

When it is right: run it bare (a preview that renders the before/after), **show the human the diff
and wait**, then `--confirm --reason "<their words>"`. On rejection do nothing. A preview of a flagged claim
showing `data.trigger: "none"` and `no_change: true` means `build/ledger/flag-store.json` did not
travel with the claim — stop and say so; confirming it clears the human's flag having changed nothing.

## Integrity

Every approval records a hash of what was approved. `check` (and `--validate`, and `--staged`)
fails `integrity_failed` when a locked claim's content, record, status or file moved outside the
approval path, or a comment block changed outside the engine. Branch on `rule` in
`data.ledger_findings` (the router's row says which rule means what). There is no `claim delete`:
unlock first. The recovery is never "re-lock so the hashes match" — restore from git, or unlock →
fix → lock with the human. The stores under `build/ledger/` are committed, never `.gitignore`d.

Modules, `claims_dir`, `source_dirs` and template overrides come from `project.config.yaml`; never
patch the engine, and never invent a third facet.
