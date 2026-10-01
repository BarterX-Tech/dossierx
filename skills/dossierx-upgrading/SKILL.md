---
name: dossierx-upgrading
description: >-
  Carrying a DossierX project from v0.7.20 or v0.7.21 onto this release. Use
  this WHENEVER you have upgraded the DossierX binary, WHENEVER a corpus you
  did not touch refuses — a claim file carrying governed_by, build_role,
  mirrors, migrated_from or tracks fails to load, a config setting
  doctrine_facet or tracks fails, every locked claim reports
  lock-content-drift right after an upgrade with no edit, or an old corpus
  lacks summaries, manifests or code links — and whenever you re-export the
  skills. Covers the one ordered pass (what to settle before upgrading, every
  binary first, re-export, make the corpus load, one unlock and one re-lock
  per claim), each fold (the doctrine hub into the constitution, governed_by,
  build_role, mirrors, migrated_from into sources, tracks into feature
  briefs), the new requirements a v0.7.20 corpus does not meet, the advisory
  claim-fit triage into claims and briefs, and an existing briefs/ directory.
  Every re-lock is the human's approval, claim by claim. Load the DossierX
  router skill first.
---

# DossierX upgrading — onto this release from v0.7.20 or v0.7.21

Read **[`dossierx`](../dossierx/SKILL.md)** first for the envelope and the error codes.

**A corpus that passed `dossierx check` before an upgrade can refuse after it with no edit on your
side.** You did not break it. Find the step below that matches the refusal, show the human what it
will change, and work it once. There is no `dossierx migrate`, no alias and no automatic adoption:
every fold is by hand, and every re-lock carries the human's `--reason`.

This skill covers a project on **v0.7.20 or v0.7.21**. A project older than v0.7.20 upgrades to
v0.7.20 first, with that release's binary and the skills it exports, then comes here.

## The pass, in this order

Every locked claim's lock hash moves in this upgrade (`migrated_from` was signed even when empty;
from v0.7.20, `governed_by` and `build_role` too), so every locked claim has to be unlocked and
re-locked once. Plan the whole pass with the human before the first unlock. Steps marked
**(v0.7.20)** are already done on a v0.7.21 project.

**Already on the new binary?** Steps 1–3 need the old one: on this release every locked claim reads
`review_pending` (cause `approval_content_drift`), which hides the claims that really wait on a
review, `claim reaudit` refuses with `integrity_failed`, and a claim file still carrying a retired
key does not load, so `claim list` and `claim unlock` refuse too. Tell the human, and put the old
binary back for steps 1–3 only (`go install` the v0.7.21 tag, or its release download), then return to this release for step 4 on. Do not unlock and re-lock
everything on the new binary instead: that approves every pending upstream change unreviewed.

**Before upgrading the binary — on the old one:**

1. **Settle two kinds of locked claim** — the bulk re-lock would otherwise erase them silently. List
   them with `dossierx claim list` (`review_pending`, `open_threads`):
   - **`review_pending: true`**: a lock refreshes the baselines and clears the flag, approving an
     upstream change nobody reviewed. Show the human each cause (`claim show <id>`) and take it
     through `claim reaudit` or a real review, not the batch. Reaudit only the claims whose
     `review_causes[]` has an entry with `direct: true`: an inherited cause
     (`upstream_dependency_review`) clears when its root claim clears or its upstream thread is
     resolved, and `claim reaudit` refuses it with `not_review_pending` — that refusal does not
     mean unlock it.
   - **an open thread**: `claim lock` refuses it later (`unresolved_comments`), so it stays draft and
     its dependents show `dependency_unapproved`. The human resolves the thread in the viewer first.
2. **Recover old approval wording if the human wants it.** `dossierx claim
   recover-approved-content` finds the approved wording of an approval recorded before the ledger
   kept text, by hashing git history against the recorded hash. After the upgrade the hash differs
   and it recovers nothing, so run it now: `--dry-run`, show the human, then `--reason "…"`.
   `git_unavailable` means nothing was looked for, not that nothing was found.
3. **Unlock every locked claim, once, on the human's yes**: `dossierx claim unlock <id> --reason
   "…"`. Do it now: the new binary loads nothing until the retired keys are gone, and a locked
   claim the pass deletes (a doctrine claim, a track's owner) must be released before its file goes.
   Commit the lock store.

**Then, on this release:**

4. **Upgrade every binary that judges the project** before the first re-lock or `brief lock` lands:
   the pre-commit hook's, CI's pin and each collaborator's. An older binary reports every re-locked
   claim as `lock-content-drift`, and re-locking with it signs the old hash back. The first `brief
   lock` moves `build/ledger/lock-store.json` to version 4; an older binary writing that store
   afterwards silently drops every brief's approval (this release then reports `brief-unrecorded`:
   restore the store from git, never re-lock), and an older binary reading it judges no brief at
   all. A store written by a newer binary is refused `store_too_new`: upgrade, never edit the store.
5. **Re-export the skills** (below).
6. **Make the corpus load.** Copy aside what the folds need first: the config's `tracks:` list, each
   track's owner claim, every doctrine claim and every claim's `governed_by` list **(v0.7.20)**.
   Then, in `project.config.yaml`: set `facets: [contract, internals]` **(v0.7.20)**, drop
   `doctrine_facet:` and the hub module **(v0.7.20)**, and delete `tracks:`. In every claim file:
   delete `governed_by:`, `build_role:`, `mirrors:` **(v0.7.20)** and `tracks:`, and fold
   `migrated_from:` into `sources`. Check for an existing `briefs/` directory. Each fold is a
   section below, named as the binary's refusal names it.
7. **(v0.7.20)** Write `constitution.yaml` if the project has none, and
   `claims/<module>/manifest.yaml` for every module ("New requirements", below). The human locks the
   constitution (`dossierx constitution lock`) before any claim can lock.
8. **Fix the content while everything is draft:** every claim gets a `summary` and a `rests_on`
   **(v0.7.20)**; sort every claim (claim fit, below) and fold each old track into a feature brief;
   move claims off any other facet and fit the caps **(v0.7.20)**.
9. `dossierx check --validate` until it is clean.
10. **Re-lock what the human still stands behind**, claim by claim: `dossierx claim lock <id>
    --dry-run` → `claim lock <id> --proposal "<snapshot>" --reason "…"`. Lock each new brief the
    same way (`brief lock <path> --dry-run`, then `--reason`), in the same pass.
11. **Plain `dossierx check`.** With `source_dirs` set, link code for every locked module claim
    ("New requirements").
12. **Commit** the claims, briefs, config, `build/ledger/` stores and `build/code-links/` together.

## Re-export the skills

The skills on disk came from the old binary. Run `dossierx skills export` (it writes every
`.claude/skills` when the repo has `.claude/`, `.agents/skills` when it has `.agents/`; name the
directory when the harness reads another), then `dossierx skills export --check`. The export removes a `dossierx-*` bundle the
previous `dossierx-skills.lock` listed that this release no longer ships, and the guide file older
releases wrote under `docs/`. `--check` refuses `skills_drift` with `data.retired[]` for one it could
not prove an export wrote: ask the human, then delete it. Commit the deletions and the refreshed
`AGENTS.md` section.

## An existing `briefs/` directory

This release reads `briefs/` beside `project.config.yaml` as the briefs tree. If the project already
keeps something unrelated there, `check` fails `lint_failed` with `brief-shape` (a stray file) or
`brief-frontmatter` (a `.md` file one folder down without a brief's frontmatter). Move it, or set
`briefs_dir:` to a directory that does not exist yet. A `claims_dir`, `project_claims_dir` or
`build_dir` that is, contains or sits inside the briefs directory fails config load
(`invalid_config`): set `briefs_dir` to another name.

## governed_by and the doctrine hub are gone

v0.7.20 corpora. A claim carrying `governed_by:` or a config setting `doctrine_facet:` fails to load,
and so does every other command until both are gone — `claim new` included. Restoring from git
brings back the same file. The constitution replaces the hub, and it is never cited.

1. **Copy aside, then make it load.** Keep a copy of every doctrine claim and of every claim's
   `governed_by` list. Then delete the hub module (its claim files and its `modules:` entry), drop
   `doctrine_facet:`, and delete `governed_by:` from every claim file.
2. Write `constitution.yaml` from the critical doctrine claims — system law every module builds
   toward, plain text, under 800 words (**[`dossierx-constitution`](../dossierx-constitution/SKILL.md)**).
3. Every other doctrine claim that something rests on becomes a project claim: `dossierx claim new
   project.<slug> --summary "…" --body "…" --rests-on …` (or `--rests-on-none-reason`), carrying the
   old body across from your copy. The rest is not carried over.
4. From the copied `governed_by` lists, write each claim's `rests_on`: `project.<slug>` where the
   governor became a project claim, nothing where it became a constitution entry, `{none: true,
   reason: "…"}` otherwise. Fix every `rests-on-required` / `rests-on-target` finding.

## build_role is gone

v0.7.20 corpora. Delete `build_role:` from every claim file. Nothing replaces it: what to implement
next is a locked module's claims and edges (**[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**).
Every claim that used to be exempt from code links by its role is now covered by the code-link gate
("New requirements").

## mirrors is gone

v0.7.20 corpora. Delete the `mirrors:` block from every claim file. If a mirrored fact is one the
claim depends on, name it in `rests_on`.

## migrated_from is gone

What a claim replaced is git history; what backs it is `sources`. For each note:

- **It names a file that still exists** (relative to the config's directory): replace it with an
  internal source — `{ref: <next free>, kind: internal, title: "…", path: <that path>, sha256:
  <sha256 of the file>}` — and cite `[ref]` in the body where the claim leans on it
  (`source-ref-unused` warns until it is cited).
- **Anything else** (free text, a deleted file, an old claim id): delete the line.

## tracks are gone

A feature is now a brief in `briefs/features/` whose `rests_on` lists the claims it is made of
(**[`dossierx-briefs`](../dossierx-briefs/SKILL.md)**). A claim or config still carrying `tracks:`
refuses with a hint starting `tracks-retired`. From the copies you kept at step 6:

1. For each track write `briefs/features/<slug>.md`, the track id slugged to `[a-z0-9-]` (`Checkout
   Flow v2` → `checkout-flow-v2`): the body from the owner claim's summary and body, minus its `[n]` source markers (a brief renders
   them as literal text; link the source instead) — with no owner,
   from the track's title and summary, filled out from the cited claims — and every cited claim id
   in `rests_on`. Two tracks with the same members become one brief (`brief-rests-on-duplicate`
   says so). The folder holds 12 briefs, the feature briefs already there included; over it, say
   so — raising `max_briefs_per_folder` is the human's call.
2. Run each owner claim through the three questions in `dossierx-claims`. It usually fails and is
   deleted; if it survives, it stays a claim and joins the brief's `rests_on`.
3. Propose the batch through the claim-fit thread below. Lock each feature brief in the step-10 pass.

## Claim fit — sorting an old corpus into claims and briefs

An old corpus was written before briefs existed, so it holds claims that are not claims: design,
voice, rationale, research, "this module owns…". Nothing refuses a corpus for that — this is
advisory — but the unlock/re-lock pass is the time to sort it. Run every claim through "Is this worth
a claim?" in **[`dossierx-claims`](../dossierx-claims/SKILL.md)** and put it in one of three piles:
**keep**, **move to a brief**, **delete** ("this module owns…" is deleted, its useful part going to
the manifest `summary`). Each `--reason` below is `<thread id>: ` followed by the human's words, or
`resolved` when their Resolve click was their only word; never invent words.

1. Propose the batch in **one thread per module**, anchored on a claim that stays: `dossierx comment
   add <keep-pile claim id> --as agent --body "…"`, listing every claim id and its pile, the brief
   path for each move, and every claim or brief resting on a moved or deleted claim with the
   `rests_on` edit it needs. If nothing in the module stays, write the destination brief as a draft
   first and anchor the thread on its path (its open thread holds `brief lock` until the human
   resolves it — the order you want). A batch that only deletes anchors on a project claim. Nothing
   moves until the human resolves the thread in the viewer.
2. On their Resolve: write each brief as a draft; delete each moved or deleted claim's file (it is
   already unlocked at step 3); edit each dependent's `rests_on` in the same change, or `check` fails
   `dangling`. Deleting a claim that was **never locked** and carries any thread, open or resolved,
   is refused `comment-digest-abandoned`: the human deletes those threads in the viewer first. Never
   delete the batch thread. A module left with no claims is removed: its `manifest.yaml`, its
   `modules:` entry and every `depends_on` naming it, in the same change.
3. Product knowledge kept elsewhere in the repository — a design note, a decision, a research
   write-up — can become a brief in the same pass, on the human's yes.

## New requirements a v0.7.20 corpus does not meet

None of these is a key to delete; each is content to write, and each is loud until written.

- **`summary` on every claim** (`summary-required`) — one standalone line (`dossierx-claims`). It is
  signed content, so a locked claim gains it between unlock and lock.
- **`rests_on` on every claim** (`rests-on-required`) — claim ids, or `{none: true, reason: "…"}`.
- **One `claims/<module>/manifest.yaml` per module** (`module-manifest`) — draft it from
  `dossierx manifest show <module> --isolation` (**[`dossierx-modules`](../dossierx-modules/SKILL.md)**).
- **A locked `constitution.yaml`** — no claim locks until the human locks it (`CONSTITUTION_NOT_LOCKED`).
- **Facets are exactly `contract` and `internals`.** A claim on another facet no longer fits the id
  grammar: re-author it under a `contract` or `internals` id, update every `rests_on` naming the old
  id, and delete the old file.
- **The caps** — 10 claims per module, 200-character summaries, 2,000-character bodies, a 6,144-byte
  module isolation budget. Sort first (claim fit), then split or trim (`dossierx-modules`). For a
  corpus far over the caps, the human may choose to set `max_claims_per_module`,
  `max_claim_summary_chars` or `max_claim_body_chars` in the config so `check` keeps working, then
  lower them module by module. That is their decision, never your default.
- **Code links for every locked module claim.** With `source_dirs` set, plain `check` refuses
  `unlinked_claims` for any locked module claim with no link; project claims are exempt. Tag the
  real code (`dossierx-code-links`). A claim with genuinely no code behind it declares
  `embodiment: {mode: none, reason: "…"}` in its re-lock, on the human's yes, or becomes a project claim.
- **A draft dependency no longer blocks a lock.** A locked claim resting on a draft is reported as
  `dependency_unapproved` (not dependency-ready) in `claim show`, not refused. The first write to
  the lock store adds `policy_version: 1` and two `policy_migrat…` fields to it: commit that diff with
  the write that produced it.
