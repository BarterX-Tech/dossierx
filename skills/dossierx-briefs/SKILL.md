---
name: dossierx-briefs
description: >-
  Writing, editing, locking and keeping briefs — the markdown documents beside
  a DossierX project's claims (briefs/<folder>/<slug>.md) for the prose that is
  not a claim: research, voice, a design system, a recorded decision, a feature.
  Use this WHENEVER you are about to write or edit a file under briefs/, run
  dossierx brief list, show, lock, unlock or reaudit, decide whether something
  is a brief or a claim, write a feature brief, choose a brief's rests_on, cite
  a brief from a claim's sources, or meet any brief-* finding — brief-shape,
  brief-frontmatter, the four caps, brief-rests-on-unknown / -missing /
  -duplicate, brief-content-drift, brief-unrecorded, brief-orphan,
  brief-abandoned, brief-dependency-drift.
  Covers the shape and the folder names, writing short, the human-gated lock
  loop, review_pending through rests_on, feature briefs (composition, never a
  gate), and comment threads on a brief. Load the DossierX router skill first.
---

# DossierX briefs — how to write a brief

Read **[`dossierx`](../dossierx/SKILL.md)** first for the envelope, the exit codes and the
`error.code` recovery table, and **[`dossierx-claims`](../dossierx-claims/SKILL.md)** for what a
claim is. A brief is a **document**, not a claim: the human reads it in one sitting, it may rest
on claims, and it gates no claim work in any state.

## The contract, in one table

| you want to | run |
|---|---|
| see every brief: path, id, summary, status, review state | `dossierx brief list` · `--review-pending` |
| read one brief: content, digest, status, `rests_on` | `dossierx brief show <path or folder.slug>` |
| check the tree's shape, frontmatter and caps, writing nothing | `dossierx check --validate` (`brief-*` findings in `data.lint_findings`, `claim_id` is a path) |
| freeze a brief, on the human's word | `dossierx brief lock <path> --dry-run`, then `--reason "<their words>"` |
| change a locked brief | `dossierx brief unlock <path> --reason "…"` → edit → `dossierx brief lock <path> --dry-run`, then `--reason "…"` (a pending review survives it) |
| a locked brief is `review_pending` because a `rests_on` claim changed | `dossierx brief reaudit <path>` (preview, read-only) → update the brief, on their yes → `dossierx brief reaudit <path> --confirm --reason "…"`, on their yes |
| answer the human's thread on a brief | `dossierx comment inbox`, then `dossierx comment reply <brief-path> <thread-id> --as agent --body "…"`; never resolve |

Every `brief` leaf takes the path `brief list` prints (`briefs/checkout/flow.md`) or the id
(`checkout.flow`); anything else is `brief_not_found` (exit 2) — list, then pass what it printed.
The `comment` verbs take the **path only** (below).

## Brief, claim, manifest line or comment?

Run the sorting step in **[`dossierx-claims`](../dossierx-claims/SKILL.md)** first: what the product
does — a guarantee about UI behaviour included — goes to the three questions there, and three yeses
is a claim no brief replaces. What fails them and still has to be written down is a brief:

| you have… | it is |
|---|---|
| one fact that passes all three questions in [Is this worth a claim?](../dossierx-claims/SKILL.md#is-this-worth-a-claim--and-how-to-write-one) | a claim — never a brief |
| why a module exists, where to start in it | its `manifest.yaml` summary (**[`dossierx-modules`](../dossierx-modules/SKILL.md)**) |
| a question, or a disagreement with a claim's wording | a comment on that claim (**[`dossierx-comments`](../dossierx-comments/SKILL.md)**) |
| law every module builds toward | the constitution (**[`dossierx-constitution`](../dossierx-constitution/SKILL.md)**) |
| a research finding, how a flow reads end to end, what a design is for, a decision and its date, the voice to write in, what a feature is for the user | a **brief** |

A brief is never a way around a cap: ten facts that will not fit a module are ten facts to cut,
not a document. And a brief never carries a fact a claim should — a reader trusts a locked claim
because the engine watches it; nothing watches a sentence in a brief.

## Shape

```
briefs/                      # briefs_dir; absent is a project with no briefs
  research/                  # what was found out: a user interview, a benchmark, a survey
  voice/                     # how the product speaks: tone, words to use and avoid
  design-system/             # what the visual system is for, and its rules
  decisions/                 # one recorded decision per file, with its date
  features/                  # what a feature is and how it works for the user
    checkout-flow.md         # id features.checkout-flow
    checkout-diagram.svg     # an image checkout-flow.md references
```

Folder names are a suggestion, not a schema: any `[a-z0-9-]` folder works, one level deep, and a
file directly under `briefs/` or anything deeper is `brief-shape`. Names are `[a-z0-9-]`, extension
lowercase; the id is `<folder>.<slug>`. A folder holds `.md` briefs and the images they reference
by bare name (`![Flow](checkout-diagram.svg)`) and nothing else; a dot-file is not read.

```markdown
---
summary: How checkout reads end to end, from cart to receipt.
status: draft
rests_on:
  - checkout.contract.cart-total
  - project.currency
---
# Checkout flow
```

- `summary` — required, one plain line, at most 200 characters. It is the line `brief list`
  prints and the only line most readers see: say what the brief settles, not what it is about.
- `status` — `draft` or `locked`; only `brief lock` / `brief unlock` change it (see below).
- `rests_on` — optional, claim ids, none repeated; an id no claim carries is
  `brief-rests-on-unknown`. Omit it unless the section below says to list one.
- `comments` — engine-managed: the comment verbs and the viewer write it, you never do.
- No other key. `#` and `##` are headings in a brief (they are literal text in a claim body). The
  first `#` heading is the title; otherwise the file name, title-cased.

**One topic per file.** If the summary needs "and", it is two briefs. FORMAT.md's "Briefs" section
is the full account of the shape.

## Writing short

State the decision or the finding, not how you got there. "Checkout keeps the cart total in the
buyer's currency until the receipt" is a brief; the transcript of the meeting that decided it is
not. Put the outcome in the first paragraph; the reasons follow only where a reader would
otherwise reopen the question.

- **Link rather than copy.** A claim's wording lives in the claim: name its id and let the reader
  run `claim show`. A source document lives at its path or URL. A sentence copied into a brief
  is a sentence that goes stale with nothing to say so.
- **Edit an existing brief before adding one.** Run `dossierx brief list` and read the summaries;
  a second brief on the same topic is the one that will contradict the first. Extend, rename, or
  split the existing file rather than starting beside it.
- **An image only when the picture carries the point.** A flow diagram that the prose cannot
  replace, yes; a screenshot of what the prose already says, no. Three images per brief, one MiB
  each, exported small.
- No `[n]` citation markers: a brief has no `sources`, and a marker renders as literal text.

## Caps — trim or merge, never raise

| cap | finding | your recovery |
|---|---|---|
| 2,000 words per brief | `brief-word-cap` | cut the how-you-got-there; if two topics remain, two briefs |
| 3 images per brief, 1 MiB each | `brief-image-cap` | keep the figure that carries the argument; export smaller |
| 12 briefs per folder | `brief-folder-cap` | merge briefs on one topic, retire stale ones, or split the folder |
| 60 briefs in the project | `brief-total-cap` | retire or merge; images count toward neither count |

Every cap is an ERROR and `check` refuses the project until the brief is split or trimmed. Each
finding names a `max_brief…` config value: **raising it is the human's decision, never yours.**
Say what it costs (a longer read for every reviewer, or a tree nobody can hold in their head),
ask, and wait for an explicit yes. Never raise a value in the same change that hit the cap.

## The lock loop — the human's approval

A brief starts `status: draft` and stays freely editable: rewrite it, rename it, delete it, move
it between folders, no ceremony — unless it carries threads (a rename strands them: see Comments)
or a claim cites it (the pin breaks: see Citing). Locking is the human's act of approving what it
says.

1. When the brief is ready for them: `dossierx brief lock <path> --dry-run`. Show the preview and
   point them at the brief in the viewer; they read it there, not in your chat.
2. On their yes: `dossierx brief lock <path> --reason "<their words>"`. It writes `status: locked`
   and records, under `briefs` in `build/ledger/lock-store.json`, the hash of the summary,
   `rests_on` and body, the sha256 of every image the brief references, the approved text, their
   reason, and one baseline per `rests_on` claim. Commit both files.
3. To change a locked brief: `dossierx brief unlock <path> --reason "<their words>"` (the record
   is kept and stamped released; `not_locked` on a draft) → edit → step 1 again → step 2 on
   their yes.

**Never lock, unlock or re-lock a brief unasked.** `--reason` carries their words, never yours.
The dry run always answers `ok: true`: read `data.blocked` and `data.missing[]`, not the exit
status, and show `side_effects`. The real `brief lock` refuses `missing_flag` (no `--reason`),
`lint_failed` (an error finding on the brief — fix the shape or the cap first), `comment_open`
(a thread is open: reply, and the human resolves it in the viewer), `already_locked` (locked and
unchanged, images included — a second lock would sign nothing), `store_gitignored`,
`pre_ledger_unadopted`, `store_too_new` (a newer binary wrote the store: upgrade, never edit the
store) and `write_conflict`, each per the router's table. A lock over a standing **or released**
record signs **only the edit**: it carries the baseline of every `rests_on` claim still listed
(`carried_baselines`) — through unlock → edit → lock and through an in-place re-lock alike — so a
claim that moved under the brief stays `review_pending` until `brief reaudit --confirm` has shown
the human the change. No lock path accepts a changed claim unseen.

**Drift is not yours to clear.** Four integrity findings ride in `data.ledger_findings` and fail
`check`, `--validate` and `--staged` alike with `integrity_failed`:

| finding | what happened | recovery |
|---|---|---|
| `brief-content-drift` | locked, and the summary, `rests_on`, body or a referenced image's bytes moved since approval (the message names the image) | restore the file (and image) from git, or unlock → fix → lock on the human's yes |
| `brief-unrecorded` | `status: locked` typed by hand, or a record an unlock released | restore the file from git, or `brief unlock` (it writes `status: draft` and releases nothing); the human's yes then goes through `brief lock` |
| `brief-unrecorded` naming a store with **no `briefs` map** | an older binary rewrote `build/ledger/lock-store.json` and dropped every brief's approval | **restore the store from git** and upgrade that binary; re-locking would discard the baselines and any pending review |
| `brief-orphan` | `status: draft` with a standing record — flipped by hand (`brief unlock` refuses it `not_locked`) | restore the file from git |
| `brief-abandoned` | a locked brief deleted or renamed with its record standing | restore it, or unlock first and then delete |

**Never re-lock to make the finding go away**: that signs the edit nobody approved. Never write
`status:` by hand in either direction.

**A brief in any state never blocks claim work.** `claim lock` does not read a brief finding, a
brief never enters `manifest show`, the catalog or the claims graph, and no brief byte enters a
claim's hash. If you are holding a claim back for a brief, you have the direction wrong.

## `rests_on` — what the brief describes, not what it mentions

List a claim when the brief **describes what that claim guarantees**: a feature walkthrough that
narrates the retry policy, a screen explanation that says what the total shows. Do not list a
claim the brief merely mentions in passing, and do not list one to show your working. Every
edge is a future review for the human; a brief with no `rests_on` never goes `review_pending`,
which is right for research, voice and a design system.

When a listed claim's content changes after the brief was locked, the brief is `review_pending`
and `check` reports `brief-dependency-drift` (a WARNING, the drifted-claim tier). A listed claim
that no longer exists is `brief-rests-on-missing` (ERROR): unlock, remove or replace the id, lock.

1. `dossierx brief reaudit <path>` — the preview, read-only and never refused by an open thread:
   `data.changed_claims[]` carries each moved claim's wording at the baseline and now (a baseline
   no snapshot matches reads "earlier wording not available"); empty when nothing is pending.
2. Update the brief so it describes the claim as it now reads: unlock → edit → lock on the
   human's yes, or an in-place re-lock when that is how they approved the edit. Both paths keep
   the pending review (the baselines are carried); the reaudit alone refreshes only the baselines.
3. Show the human the diff from step 1 and wait. On their yes:
   `dossierx brief reaudit <path> --confirm --reason "<their words>"`.

`--confirm` refuses `not_locked` (exit 2), `not_review_pending` (exit 2; the edit path is unlock
→ fix → lock), `wrong_state` (exit 2; a `rests_on` claim is gone — fix the list first),
`integrity_failed` (the brief itself was edited since approval — settle that first, above),
`comment_open` and `missing_flag`. A draft has no baselines and is never pending:
`brief list --review-pending` lists locked briefs only. Nothing flows back — a brief's state
never sets `review_pending` on a claim.

## Writing a feature brief

A feature is an ordinary brief in `briefs/features/`. The body says **what the feature is and
how it works for the user**, in the user's order, and links the voice or design briefs it relies
on by name rather than restating them. `rests_on` lists the claims the feature is made of —
across modules, `contract` claims and `project.*` alike. That list is **composition**: it says
which promises the feature is built from. It is never a gate, never a build sequence, and never
a status.

- **No owner claim.** A claim that only says "the checkout feature exists" or narrates the
  feature fails the three questions and is not written; one already locked is unlocked, then
  deleted, on the human's yes. The brief is the feature's home.
- **Two briefs with the same `rests_on` set** — features or not — are `brief-rests-on-duplicate`
  (WARNING): two features made of the same claims are one feature. Merge them, or make one of
  them describe something the other does not.
- **Nothing reports a feature as specified or built.** `brief show` derives nothing from the
  claims a brief rests on, `check` reports nothing of the kind, and neither do you: every claim
  in `rests_on` locked and linked is evidence about the claims, not about the feature.
  **Never say a feature is built** — say which of its claims are locked, which are linked, and stop.

## Citing a brief from a claim

A claim may cite a brief in `sources` as an `internal` entry (`path` and `sha256`; see
**[`dossierx-claims`](../dossierx-claims/SKILL.md)**) **only when the brief is evidence** — a
research finding, a recorded decision. Never cite a guidance brief (voice, design system, a
feature walkthrough): guidance is not what makes a claim true.

The citation pins the **whole file's bytes** — not the lock hash. A lock or unlock (the `status:`
write), any edit, and every comment thread opened or answered on the brief moves them, and every
citing claim then reports `source-internal-drift` (ERROR) on the next `check`. Expect it, and
batch it: refresh the citing claims' `sha256` **after the brief's final lock**, taking them
through unlock → refresh → lock in the **same review** as the brief, with the human's words,
rather than meeting them one at a time later. A thread on a cited brief carries that cost too,
so a brief that claims cite is a brief to edit, and to discuss, rarely and deliberately.

## Comments on a brief

The human can open a thread on a brief in the viewer, exactly as on a claim. It arrives in
`dossierx comment inbox` with the rest, under the same cursor, as a row with `kind: "brief"` and
the brief's **path** as `claim_id`; `check` counts it as `open comments: brief "<path>": N`. The
`comment` verbs take that path, never the id (`comment list <brief-path>`,
`dossierx comment reply briefs/checkout/flow.md <thread-id> --as agent --body "…"`; an id is
refused `claim_not_found`). Reply and **never resolve** — the human resolves it in the viewer,
and their Resolve is the approval. An open thread blocks `brief lock` and
`brief reaudit --confirm` (`comment_open`) until they do, so **do not open a thread on a brief
you are about to lock unless the human asked for one**: a question about a draft goes in chat.
An open thread never blocks a claim. Threads live in the brief's frontmatter, excluded from its
lock hash, so a comment never drifts a locked brief — it does move a citing claim's pin (above).
A brief renamed with its threads left behind is `comment-digest-abandoned`.

## Portability

Briefs add `briefs_dir` and the five `max_brief…` values to `project.config.yaml`, nothing else.
A project with no `briefs/` sees no finding, no field and no byte of change; never create the
directory unasked.
