---
name: dossierx-upgrading
description: >-
  Carrying a DossierX project across a binary upgrade. Use this WHENEVER you
  have upgraded the DossierX binary, WHENEVER a corpus you did not touch
  refuses — a claim file carrying governed_by, build_role or mirrors fails to
  load, a config setting doctrine_facet fails, every verb refuses
  layout_legacy, check reports lock-ledger-pre-ledger / pre_ledger_unadopted,
  or an old corpus lacks summaries, manifests or code links — and whenever
  you run dossierx claim recover-approved-content or re-export the skills.
  Covers the v0.7.20 → v0.7.21 pass in order, re-exporting skills (retired
  bundles are pruned), the layout_legacy moves, the pre-ledger crossing,
  recovering approved wording from git, the lock-store diff when lock policy
  0 is carried over to v1, the hand folds for governed_by, the doctrine hub,
  build_role and mirrors, and the new requirements (summary, manifest.yaml,
  facets, caps, code links). Every fold that
  re-locks is the human's approval, claim by claim. Load the DossierX router
  skill first.
---

# DossierX upgrading — folding a corpus onto a new release

Read **[`dossierx`](../dossierx/SKILL.md)** first for the envelope and the error codes.

**A corpus that passed `dossierx check` before an upgrade can refuse after it with no edit on your
side.** That is the case agents handle worst: the instinct is to hunt for what you broke, find
nothing, and loop. You did not break it. Find the fold below that matches the refusal, show the human
what it will change, and work it once. There is no `dossierx migrate`, no alias and no automatic
adoption: every fold is by hand, and every re-lock carries the human's `--reason`.

## First: re-export the skills

The skills on disk came from the old binary. Run `dossierx skills export` (it writes every
`.claude/skills` and `.agents/skills` the repo already has; name the directory when the harness reads
another one), then `dossierx skills export --check`. The export **removes retired bundles** — a
`dossierx-*` directory the tree's previous `dossierx-skills.lock` listed that this release no longer
ships, such as `dossierx-build-order`. `--check` refuses `skills_drift` with `data.retired[]` for
any `dossierx-*` directory still there that the lock never listed: ask the human, then delete it. It
never touches a directory whose name does not start with `dossierx`.

## Coming from v0.7.20: one pass, in this order

v0.7.21 retired three claim fields and made five things required. Plan the whole pass with the
human before the first unlock, so each locked claim is unlocked and re-locked **once**:

1. Re-export the skills (above).
2. Make the corpus load: delete `build_role:`, `governed_by:` and `mirrors:` from every claim file,
   set `facets: [contract, internals]`, and drop `doctrine_facet:` by folding the doctrine hub
   (sections below). Every locked claim's hash moved, so each now reports `lock-content-drift`.
3. Write `claims/<module>/manifest.yaml` for every module, and `constitution.yaml` beside the config
   if the project has none; the human locks the constitution before any claim can lock.
4. With the human's yes, `claim unlock <id> --reason "…"` each locked claim. It is re-locked once,
   at step 7, whatever else changes.
5. Give every claim a `summary`, move claims off any other facet, and fit the caps.
6. `dossierx check --validate` until it is clean.
7. Re-lock what the human still stands behind: `claim lock --dry-run` →
   `claim lock --reason "…" --proposal "<snapshot>"`, their approval claim by claim.
8. Plain `dossierx check`: link code for every locked module claim.

Steps 3, 5 and 8 are under "New requirements" below.

## `layout_legacy` — generated files at the project root

Every verb refuses, `check --validate` and `--staged` included, until the printed `git mv` / `mv`
block has run: paste it verbatim (`error.details.moves[]` carries the same list), then
`dossierx check --validate`. Nothing needs re-locking: signatures hash bytes, not paths.

## The pre-ledger crossing

A project whose lock store predates the lock ledger **and** still holds a locked claim does not
`check` clean: `integrity_failed` carrying **`lock-ledger-pre-ledger`** — project-scoped, said once,
deliberately *not* one `lock-ledger-missing` per claim, whose "set it back to draft and re-lock"
recovery would be destructive here. Read the rule, not the count. The write path refuses in the same
state with `pre_ledger_unadopted` (`claim lock`, `claim reaudit --confirm`). It is **silent** on a
pre-ledger project holding nothing locked: that project's next `claim lock` crosses it.

**The crossing, in this order and no other:** (1) `dossierx claim unlock <id> --reason "…"` for
every locked claim; (2) re-lock only what the human still stands behind —
`dossierx claim lock <id> --dry-run`, then `--reason "…" --proposal "<snapshot>"`. The first lock in a project holding nothing
locked crosses the store. Nothing is grandfathered, because nothing can attest to content no ledger
ever recorded. **Not your call:** unlocking everything discards every standing approval, so show the
human the finding, say what it discards, and get a yes. Commit the lock store and the comment digest
store the crossing writes. The **store file** tells this benign case from its opposite: pre-ledger =
store **present** on the old schema (cross it); `lock-ledger-absent` = store **gone** while locked
claims remain (tampering; restore it from git).

## Lock policy 0 is retired — the store says so on its next write

Local approval v1 is the only lock policy. A lock store that records `policy_version: 0`, or has no
`policy_version` at all, reads as v1 with no command from you. The next command that writes the store
(for example `claim lock`, `unlock` or `constitution lock`) adds `policy_version: 1`,
`policy_migrated_at` and `policy_migration_reason` to `build/ledger/lock-store.json`. That diff is the
carry-over, not tampering: commit it with the write that produced it. No approval, baseline or
review cause changes.

What changes is what a draft dependency means. `rest-on-locked` no longer exists: a locked claim
that rests on a draft is no longer a lint error; `claim show` and `readiness` report it as
`dependency_unapproved`, and the claim is not dependency-ready. A draft whose dependency is still
draft can now be locked (preview first, then the human's `--reason` and the `--proposal` snapshot).

## `claim recover-approved-content` — once, for old approvals

Approvals recorded before the ledger kept the approved WORDING signed a hash, not the text, so a
claim edited since shows "the approved wording was not kept on the record" and no comparison. This
verb finds each approval's own revision in the project's git history by hashing it against the hash
the record already signed, and stores that wording on the record. It creates no approval: a revision
that does not hash equal is refused, and an unfindable claim is named and left alone. `--dry-run`
first and show the human — it edits their approval file — then run it with `--reason`. If git cannot
answer it refuses `git_unavailable`, which is not an empty recovery: nothing was looked for.

## `governed_by` and the doctrine hub are gone

A claim still carrying `governed_by:` or a config still setting `doctrine_facet:` **fails strict
decode** (`stopped_at: load`), and "restore from git" restores the file that no longer loads. The
constitution is the roof and is never cited. **Fold by hand, one pass, in this order:**

1. Write `constitution.yaml` from the critical former doctrine claims — plain text, under 800 words
   (**[`dossierx-constitution`](../dossierx-constitution/SKILL.md)** says what belongs). The **human**
   approves its lock; nothing else locks until then (`CONSTITUTION_NOT_LOCKED`).
2. Every other doctrine claim becomes `project.<slug>` in `project-claims/<slug>.yaml`:
   `dossierx claim new project.<slug>` with `--summary`, `--body` and `--rests-on` or
   `--rests-on-none-reason`, then carry the old body across. Delete the hub module, its `doctrine` facet and its claim files; a
   module left over is a normal module, never the roof. Drop `doctrine_facet:` from the config.
3. In every remaining claim delete `governed_by:` and write `rests_on`: `project.<slug>` where the
   governor became a project claim, nothing where it became a constitution entry, `{none: true,
   reason: "…"}` otherwise. Every `<hub>.doctrine.<slug>` reference becomes `project.<slug>` or goes.
4. `dossierx check --validate`; fix every `rests-on-required` / `rests-on-target` finding.
5. Re-lock, per module and with the human. In v0.7.21 **every** locked claim's hash moves once —
   `governed_by` and `build_role` both left the signed schema — so each reports
   `lock-content-drift` until it re-locks:
   `claim unlock` → `claim lock --dry-run` → `claim lock --reason --proposal`.

## `build_role` is gone

A claim file still carrying `build_role:` fails strict decode at load, and `claim new --build-role`
no longer exists. There is no build-order sequencer; what to implement next is a locked module read
through `manifest show`, its `depends_on`, and its claims' `rests_on`
(**[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**).

1. Delete the `build_role:` line from every claim file, draft and locked alike.
2. `dossierx check --validate` — the corpus loads again. Every locked claim reports
   `lock-content-drift` until step 3, whether or not it carried the key: the field left the signed
   schema, so every hash moved. That is the fold in progress, not tampering.
3. Re-lock each **locked** claim: `claim unlock` → `claim lock --dry-run` →
   `claim lock --reason "…" --proposal "<snapshot>"`, each re-lock approved by the human. Do this in
   the same pass as any `governed_by` fold, so each claim re-locks once.
4. Plain `dossierx check`. With `source_dirs` set, the code-link gate now covers **every** locked
   module claim (project claims are exempt), including the ones that used to be `orientation` or
   `out-of-scope`, so expect `unlinked_claims` on some. Tag the real code that implements each one.
   A locked claim that genuinely has no code behind it is the human's call, not a file to tag
   around: on their yes it declares `embodiment: {mode: none, reason: "…"}` — best folded into the
   same re-lock as step 3 — or it becomes a project claim.

## `mirrors` is gone

A claim file still carrying `mirrors:` fails strict decode at load (`invalid_claim`,
`stopped_at: load`), and `claim new --mirrors` no longer exists. The engine never drew the edge in
this release; now the key itself is refused. Delete the `mirrors:` block from every claim file,
draft and locked alike. If the mirrored fact is one this claim depends on, say so with `rests_on`
instead. A locked claim that carried the key re-locks after the edit, and that re-lock is the
human's approval, in the same pass as the other folds.

## New requirements a v0.7.20 corpus does not meet

None of these is a key to delete; each is content to write, and each is loud until it is written.

- **`summary` on every claim** (`summary-required`, ERROR on every status). One plain line that
  stands alone in `manifest show --isolation`; `dossierx-claims` says how to write one. A locked
  claim gains its summary between unlock and lock: the summary is signed content.
- **One `claims/<module>/manifest.yaml` per module** (`module-manifest`). Draft it from
  `dossierx manifest show <module> --isolation` — **[`dossierx-modules`](../dossierx-modules/SKILL.md)**.
- **Facets are exactly `contract` and `internals`** (`invalid_config` at load for any other list).
  A claim on another facet no longer fits the id grammar: re-author it under a `contract` or
  `internals` id, update every `rests_on` naming the old id, and delete the old file. A locked one
  is unlocked first.
- **The caps** — ten claims per module, 200-character summaries, 2000-character bodies, the
  module's 6144-byte isolation budget. Split or trim per `dossierx-modules`; raising a cap is the
  human's call.
- **Code links for every locked module claim.** With `source_dirs` set, plain `check` refuses
  `unlinked_claims` for any locked module claim with no link, whatever it used to be; project
  claims are exempt. Tag the real code (**[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**).
  A locked claim with genuinely no code behind it is the human's call: on their yes it declares
  `embodiment: {mode: none, reason: "…"}` in the same re-lock, or becomes a project claim.
