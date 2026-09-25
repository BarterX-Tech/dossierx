---
name: dossierx-upgrading
description: >-
  Carrying a DossierX project across a binary upgrade. Use this WHENEVER you
  have upgraded the DossierX binary, WHENEVER a corpus you did not touch
  refuses — a claim file carrying governed_by or build_role fails to load, a
  config setting doctrine_facet fails, every verb refuses layout_legacy, or
  check reports lock-ledger-pre-ledger / pre_ledger_unadopted — and whenever
  you run dossierx claim recover-approved-content or re-export the skills.
  Covers re-exporting skills (retired bundles are pruned), the layout_legacy
  moves, the pre-ledger crossing, recovering approved wording from git, and the
  hand folds for governed_by, the doctrine hub and build_role. Every fold that
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
5. Re-lock, per module and with the human: every locked claim that carried `governed_by` — before
   v0.7.21 that was normally all of them — has a moved hash (`lock-content-drift` until it re-locks):
   `claim unlock` → `claim lock --dry-run` → `claim lock --reason --proposal`.

## `build_role` is gone

A claim file still carrying `build_role:` fails strict decode at load, and `claim new --build-role`
no longer exists. There is no build-order sequencer; what to implement next is a locked module read
through `manifest show`, its `depends_on`, and its claims' `rests_on`
(**[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**).

1. Delete the `build_role:` line from every claim file, draft and locked alike.
2. `dossierx check --validate` — the corpus loads again. The locked claims you edited report
   `lock-content-drift` until step 3: that is the fold in progress, not tampering.
3. Each **locked** claim that carried it has a moved hash: `claim unlock` → `claim lock --dry-run` →
   `claim lock --reason "…" --proposal "<snapshot>"`, each re-lock approved by the human. Do this in
   the same pass as any `governed_by` fold, so each claim re-locks once.
4. Plain `dossierx check`. With `source_dirs` set, the code-link gate now covers **every** locked
   module claim (project claims are exempt), including the ones that used to be `orientation` or
   `out-of-scope`, so expect `unlinked_claims` on some. Tag the real code that implements each one.
   A locked claim that genuinely produces no code is a design question for the human — does it
   belong as a claim, or as a project claim? — not a field to set and not a file to tag around.
