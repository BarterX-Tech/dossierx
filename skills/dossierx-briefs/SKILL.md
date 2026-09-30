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
  -duplicate, brief-content-drift, brief-unrecorded, brief-dependency-drift.
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
| change a locked brief | `dossierx brief unlock <path> --reason "…"` → edit → `dossierx brief lock <path> --dry-run`, then `--reason "…"` |
| a locked brief is `review_pending` because a `rests_on` claim changed | `dossierx brief reaudit <path>` (preview) → update the brief → `--confirm --reason "…"` |
| answer the human's thread on a brief | `dossierx comment inbox`, then reply; never resolve |

`brief show` takes the path `brief list` prints (`briefs/checkout/flow.md`) or the id
(`checkout.flow`); anything else is `brief_not_found` (exit 2) — list, then pass what it printed.

## Brief, claim, manifest line or comment?

Run the three questions in **[`dossierx-claims`](../dossierx-claims/SKILL.md)** first. Three yeses
is a claim, and no brief replaces it. What fails them and still has to be written down is a brief:

| you have… | it is |
|---|---|
| one fact another module or a locked promise would break on | a claim — never a brief |
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
it between folders, no ceremony. Locking is the human's act of approving what it says.

1. When the brief is ready for them: `dossierx brief lock <path> --dry-run`. Show the preview and
   point them at the brief in the viewer; they read it there, not in your chat.
2. On their yes: `dossierx brief lock <path> --reason "<their words>"`. It writes `status: locked`
   and records the content hash, their reason and the approved markdown under the `briefs` map
   in `build/ledger/lock-store.json`. Commit both files.
3. To change a locked brief: `dossierx brief unlock <path> --reason "<their words>"` → edit →
   step 1 again → step 2 on their yes.

**Never lock, unlock or re-lock a brief unasked.** `--reason` carries their words, never yours;
a `--dry-run` that reports `blocked: true` is a successful answer (the router's dry-run rule), and
`side_effects` is the part to show them. `brief lock` refuses `already_locked` on a locked,
unchanged brief — a second lock would sign nothing — and `comment_open` while a thread on the
brief is open: reply, and the human resolves it.

**Drift is not yours to clear.** `brief-content-drift` (locked, and the hash moved) and
`brief-unrecorded` (`status: locked` in the frontmatter, no record) mean a locked brief was edited
outside that path — by hand, or by a merge. Both surface through `integrity_failed` in
`data.ledger_findings`, on `check`, `--validate` and `--staged` alike. The recovery is restoring
the file from git, or unlock → fix → lock with the human's words. **Never re-lock to make the
finding go away**: that signs the edit nobody approved. Never write `status:` by hand either way.

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

1. `dossierx brief reaudit <path>` — the preview: one diff per changed claim, from the wording
   the brief was approved against to today's. Read it.
2. Update the brief so it describes the claim as it now reads (unlock → edit → lock if the
   wording changes; the reaudit alone refreshes only the baselines).
3. Show the human what changed and wait. On their yes:
   `dossierx brief reaudit <path> --confirm --reason "<their words>"`.

`reaudit` refuses a brief that is not `review_pending` (the edit path is unlock → fix → lock)
and refuses `comment_open` while a thread is open. A draft has no baselines and is never
pending: `brief list --review-pending` lists locked briefs only. Nothing flows back — a brief's
state never sets `review_pending` on a claim.

## Writing a feature brief

A feature is an ordinary brief in `briefs/features/`. The body says **what the feature is and
how it works for the user**, in the user's order, and links the voice or design briefs it relies
on by name rather than restating them. `rests_on` lists the claims the feature is made of —
across modules, `contract` claims and `project.*` alike. That list is **composition**: it says
which promises the feature is built from. It is never a gate, never a build sequence, and never
a status.

- **No owner claim.** A claim that only says "the checkout feature exists" or narrates the
  feature fails the three questions and is deleted. The brief is the feature's home.
- **Two features with the same `rests_on` set** are `brief-rests-on-duplicate` (WARNING): they
  are one feature. Merge them, or make one of them describe something the other does not.
- **Nothing reports a feature as specified or built.** `brief show` derives nothing from the
  claims a brief rests on, `check` reports nothing of the kind, and neither do you: every claim
  in `rests_on` locked and linked is evidence about the claims, not about the feature. **Never
  say a feature is built** — say which of its claims are locked, which are linked, and stop.

## Citing a brief from a claim

A claim may cite a brief in `sources` as an `internal` entry (`path` and `sha256`; see
**[`dossierx-claims`](../dossierx-claims/SKILL.md)**) **only when the brief is evidence** — a
research finding, a recorded decision. Never cite a guidance brief (voice, design system, a
feature walkthrough): guidance is not what makes a claim true.

The citation pins the brief's bytes. When the brief is re-locked, every citing claim reports
`source-internal-drift` (ERROR) on the next `check`: expect it, and batch it — take the citing
claims through unlock → refresh the `sha256` → lock in the **same review** as the brief, with the
human's words, rather than meeting them one at a time later. A brief that claims cite is a brief
to edit rarely and deliberately.

## Comments on a brief

The human can open a thread on a brief in the viewer, exactly as on a claim. It arrives in
`dossierx comment inbox` with the rest, under the same cursor; reply to it with
`dossierx comment reply` as the row names it, `--as agent`, and **never resolve it** — their
Resolve click is the approval. An open thread blocks `brief lock` and `brief reaudit --confirm`
(`comment_open`); an open thread never blocks a claim. Threads live in the brief's frontmatter
and are excluded from its content hash, so a comment never drifts a locked brief.

## Portability

Briefs add `briefs_dir` and the five `max_brief…` values to `project.config.yaml`, nothing else.
A project with no `briefs/` sees no finding, no field and no byte of change; never create the
directory unasked.
