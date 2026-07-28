# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] — v0.3.0

The agent-first restructure. DossierX has two users with opposite needs: an **agent** that
operates it, and a **human** who reviews what the agent did. Until now both were half-served by
one command line. v0.3.0 gives each its own surface and takes the other away — the agent gets a
20-command machine-readable CLI, the human gets the viewer and one command (`dossierx serve`).

Alongside the split, this release closes the gap that made the split worth making: until now a
locked claim could be hand-edited and **nothing would notice**. The new lock ledger records what
was approved, when, by whom, and on whose words, and a gate compares the claims against it in
`dossierx check`, in a pre-commit hook, and in CI.

**This release is not backward compatible at the CLI.** Ten commands were removed and four were
moved. The migration table below maps every one of them.

### BREAKING — every existing project must run `dossierx migrate --adopt` once

**This is the one change that breaks a project rather than a script, and it breaks on the first
`dossierx check` after the upgrade.** If your project has ever locked a claim, run this once,
before you lean on the hook or CI:

```sh
dossierx migrate --adopt --dry-run   # look first: it names every artifact it would adopt
dossierx migrate --adopt
```

then commit the rewritten `.dossierx-lock-store.json` — and the `.dossierx-comment-digest.json` the
same run creates — in the same commit as the claims they now cover. `--adopt` is required, so a bare
`dossierx migrate` refuses with `missing_flag` rather than guessing at a migration you did not name.
`--dry-run` lists every claim and build order it would adopt and writes nothing.

**There is deliberately no `--reason`,** and that is the one place this command breaks the pattern
every other record-writing verb follows. They take the human's words because a human approved
something. Nobody approved this. Every record the migration writes carries a fixed reason saying so
— *"grandfathered by `dossierx migrate --adopt`: locked before this project had a lock ledger;
content adopted as-found on migration day, never approved by anyone"* — and `grandfathered: true`,
permanently. A human-supplied reason would make an adoption read like an approval in a ledger diff,
which is exactly the confusion the fail-closed decision exists to remove.

**Why this is not automatic any more.** Earlier in this release cycle it was: a pre-ledger project
was grandfathered in on its first plain `check`, which observed whatever the claims said at that
moment and recorded them as approved. It was convenient and it was unsound, because *adoption is
the one operation in the design that manufactures approval out of nothing*. A gate that performs it
on sight rewards deleting the ledger with a clean bill of health over content nobody reviewed, and
turns "arrive with no ledger" into a universal bypass. Review rounds then tried to distinguish an
honest v0.2.x store from a deliberately downgraded one using evidence inside the project, and could
not: `locked_at` shipped in v0.2.0 (verifiable with `git show v0.2.0:internal/lock/lock.go`), so no
field, no timestamp and no sibling file tells the two apart. When no predicate can be trusted the
answer is not a cleverer predicate — it is to stop guessing and make a human decide, once.

So **adoption now fails closed, in every run**: a missing or unreadable ledger never grandfathers
anything on plain `check`, on `--validate`, or on `--staged`. The only code path that writes an
adopted record is the one a human invokes deliberately.

What the migration does and does not do: it hashes each currently-locked claim and each locked
build order **exactly as they sit on disk** and records them as the baseline, marked as adopted
rather than approved, permanently — an adopted hash is content that was *observed*, not reviewed.
It changes no claim's `status`, resolves no thread, and clears no `review_pending`. Read the claims
before you run it; nothing in the command can check them for you. It is an upgrade step and never a
recovery tool: on a project that already has ledger coverage it refuses, and reaching for it to
silence a gate on a project that *has* a ledger would record tampered bytes as approved, which is
precisely what the fail-closed rule exists to prevent.

Skipping it is loud rather than silent. `dossierx check` fails on the lock-ledger gate with the new
project-scoped finding **`lock-ledger-adoption-required`**, under the `integrity_failed` code the
gate already uses — **no new `error.code` was added for it**. It is one finding naming the
migration, deliberately in place of one `lock-ledger-missing` per claim: repeating "this claim is
locked with no record" N times would attach a recovery (set it back to draft and re-lock) that is
actively destructive advice at a project which has done nothing wrong. It is also genuinely
distinct from its neighbour, and the **lock store itself** is what tells the two apart, with no git
history required: `lock-ledger-adoption-required` fires when the store is **present** and still on
the pre-ledger schema (benign; recovery is the migration), `lock-ledger-absent` when the store
**file is gone** while locked claims remain (tampering; recovery is version control).

Running it twice is refused, with the new `error.code` **`already_migrated`** and a `data.mode` of
`already_covered` or `nothing_to_adopt` so a caller can tell the two apart — a migration that can be
re-run is a laundering command, since deleting one record and re-migrating would re-sign the edit it
covered as approved. A pre-commit hook and a CI run fail the same way as `check` — the run that
would previously have blessed a project quietly now refuses it.

### `check --staged` judges the git index, and only the git index

`--staged` reads the **git index** — what the commit will actually contain — with `git show`
instead of the worktree, and writes nothing. That is what makes it meaningful in a pre-commit hook,
and it is unchanged. What it does **not** do is read git history: it judges one tree, and its
verdict is identical in every clone, at every depth, on every branch.

**Removed before release: the parent-commit comparison.** An earlier build on this branch had
`--staged` resolve the parent of the commit under judgement and compare the two, reporting a removed
lock ledger or a repointed `claims_dir` as **`integrity-store-removed`** and
**`claims-scope-narrowed`**. Both rules are gone, along with the shallow-clone advisory in
`data.next_steps` that told you to set `fetch-depth: 0`. **The shipped CI template changed with
them**: it is now a single `dossierx check` step and pins no `fetch-depth`, because a shallow
checkout is a complete tree and one tree is the whole evidence base. The second `check --staged`
step is gone from CI too — on a fresh checkout the index, the worktree and `HEAD` are three names
for one tree, so it re-ran the same rules over the same bytes. `--staged` itself is **not**
deprecated; its home is the pre-commit hook, where the index and the worktree genuinely differ.
The diagnosis behind the removed rules was right — a
change that repoints `claims_dir` *and* deletes the ledger leaves nothing in scope to judge, so the
gate reports `ok: true` — but the fix was in the wrong layer. The parent commit is outside the
*commit* and not outside the *committer*: git history is written by exactly the party the gate
constrains, so `--orphan`, a rebase or a second config file switched the comparison off without
looking unusual in a log, and every other in-repo source of that evidence has the same property.
It also could not see intent the tree does not record, and charged two ordinary git operations for
that: a legitimate **`git revert`** of a commit containing a claim lock was refused (the revert
removes that lock's records, byte-identical to erasing them) and, because git does not run
`pre-commit` for `revert`, it landed locally at exit 0 and only CI objected; and a project that was
**new in a commit** was audited against an unrelated retired project's ledger in a monorepo and
refused with findings naming another project's claim ids. A control that a rebase disables and a
revert trips is not a control, so it was removed rather than patched.

**What this costs, stated exactly.** Removing it loses **three** detections — not one, and not two.
Two earlier statements of this cost in this branch were wrong and are corrected here: "exactly one
detection" counted only the scope comparison, and the revision to "two" still missed
`lock.AuditAgainstParent`, the per-claim half that read the parent's stores. The true number is
three. All three are changes that erase *every side* of a disagreement at once so that no surviving
evidence can name it, and each is a conjunction: every half alone is still caught from state.

1. **The collapsed scope** — `claims_dir` repointed **and** the lock ledger removed **in the same
   change**. Repoint only gives `lock-ledger-abandoned` (once per locked claim whose standing record
   now names a claim the project cannot see); deleting the ledger only gives project-scoped
   `lock-ledger-absent` plus `lock-ledger-missing` per claim.
2. **The disowned claim** — one claim's `ledger` record, its `locked_at` stamp **and** its `hashes`
   baselines deleted from the lock store, its YAML flipped `locked` → `draft`, and its body
   rewritten, **in the same change**. It is the **cheapest of the three**: no `claims_dir` edit, no
   store deleted, no other claim touched — three keys out of one JSON file and one word in one YAML
   file, after which the claim is a freely editable draft that re-locks with an agent-supplied
   `--reason` reading exactly like a human's approval. The parent comparison reported it as
   `lock-ledger-deleted`. Single-tree, that rule still fires whenever `locked_at` or a baseline
   survives the deletion; leaving `status: locked` instead gives `lock-ledger-missing`; leaving the
   record standing while flipping to draft gives `lock-ledger-orphan`; and the variant that moves
   the claim file out of `claims_dir` rather than flipping its status gives `lock-ledger-abandoned`
   unless the same three keys go with it. `claim lock` refuses it on the **write** path too, on the
   same surviving evidence.
3. **The erased review** — a **draft** claim's `comments:` block deleted **and** that claim's key
   dropped from the digest store **in the same change**. Erasing the block only gives
   `comment-ledger-drift`; dropping the key only gives `comment-digest-unrecorded`. This one is
   sharper than its size suggests: an open thread is exactly what blocks `claim lock`
   (`unresolved_comments`), so the erasure buys the lock, and the claim then locks cleanly over a
   review that was deleted. It is **confined to draft claims**, because `comment-digest-missing`
   keys on a STANDING lock-ledger record with no digest entry: a locked claim has such a record and
   is reported, a draft claim has none and is never asked. Measured on a locked claim: erasing the
   block alone gives `comment-ledger-drift`, erasing it with its digest key gives
   `comment-digest-missing`. `lock-content-drift` is NOT involved — `comments:` is excluded from the
   locked-claim hash by design (`internal/lock/lockedhash.go`), since `dossierx serve` writes
   comments without write authority over the lock store.

There is no cheap single-tree replacement for any of them, because each resulting tree is
indistinguishable from an innocent one: "no claims in scope and no ledger" is exactly what a
brand-new project looks like, "a draft with no record, no `locked_at` and no baselines" is exactly
what every draft looks like on the day it is written, and "a draft with no threads and no digest
entry" is exactly what most drafts look like. A gate that refuses every new project on its first
day, every new draft on the day it is authored, or every uncommented draft, gets deleted. All three
are caught where a control the committer cannot rewrite actually lives: **branch protection with a
required CI check, plus review of a loud diff** — each of the three is a hand edit of a tracked JSON
store whose whole purpose is to be read in a diff, sitting beside the claim change it was made to
permit. **DossierX detects; the forge enforces.** None of the three is folklore: all three are pinned
as passing tests in `internal/check/staged_no_parent_test.go`, with the same boundary pinned at the
audit layer in `internal/lock/audit_boundary_test.go`, each beside its "either half alone is still
refused" assertions.
[FORMAT.md](FORMAT.md#what-the-gate-detects-what-it-does-not-and-where-the-rest-is-caught) carries
the full boundary, shape by shape.

Everything else in the gate is untouched and still single-tree: `lock-content-drift`,
`lock-ledger-missing`, `lock-ledger-deleted`, `lock-ledger-released`, `lock-ledger-orphan`,
`lock-ledger-abandoned`, `lock-ledger-absent`, `lock-ledger-unreadable`, `lock-ledger-downgraded`,
`lock-ledger-adoption-required`, `comment-ledger-drift`, the `comment-digest-*` family and the
`build-order-*` family, plus fail-closed adoption and `dossierx migrate --adopt`.

### Migration — every retired command and its replacement

| Retired | Replacement | Notes |
| --- | --- | --- |
| `dossierx lint` | `dossierx check --validate` | `--validate` is a **read-only** run — no claim files, no lock store, no `.catalog.json`, no viewer. Plain `check` writes all four. |
| `dossierx lint --json` | `dossierx check --validate` (JSON is the default format) | Findings are `data.lint_findings[]`, in snake_case: `lint`, `claim_id`, `severity`, `message` (the old bare array used Go field names). |
| `dossierx catalog` | `dossierx check` | It was a stage of `check`, exposed as a verb only because the extraction had no Go API. |
| `dossierx render` | `dossierx check` | Same. |
| `dossierx deps <id>` | `dossierx claim show <id>` | Reports both edge directions as before, **plus** lock state, `review_pending` and its trigger, code links with drift, comment counts, and `next_actions`. |
| `dossierx stale` | `dossierx claim list --review-pending` | Names the claims and reports the count, as before. The bespoke "nothing locked" message is gone; an empty project is an empty result. |
| `dossierx coverage` | `dossierx claim list --migrated` | Reports the same ratio (`count`, `total`, `percent_of_total`) **and** names the claims. |
| `dossierx implink set` | `dossierx claim link` | Identical flags (`--module --claim --file --symbol`) and identical behavior. |
| `dossierx implink status` | `dossierx claim show <id>` | Per-claim `implemented_in[]` with a `drifted` verdict on each file. `dossierx check` still reports module-wide impl-link status. |
| `dossierx lock <id>` | `dossierx claim lock <id>` | `--reason` is required (see below). |
| `dossierx unlock <id>` | `dossierx claim unlock <id>` | `--reason` is required. |
| `dossierx flag <id>` | `dossierx claim flag <id>` | Unchanged otherwise. |
| `dossierx reaudit <id>` | `dossierx claim reaudit <id>` | `--reason` is required with `--confirm`. |
| `dossierx comment edit` | the viewer | A review history the agent can rewrite is not a review history. Still fully available over `dossierx serve`'s HTTP API. |
| `dossierx comment delete` | the viewer | Same. |
| `dossierx comment resolve` | the viewer | Advisory rights already forbade an agent acting on a human-authored thread, and every viewer thread is human-authored — so on the CLI this could only ever act on the agent's own threads. The human's **Resolve click is the approval** the lock gate waits for. |
| `dossierx comment reopen` | the viewer | Same. |
| `dossierx comment list --json` | `dossierx comment list` (JSON is the default format) | Threads are `data.threads[]` inside the standard envelope, rather than a bare array. |

Nothing was removed from the **product**: `internal/comments` still implements all six comment
operations, and `dossierx serve`'s HTTP API — which is what the viewer drives — still exposes
every one of them. Only the CLI surface shrank.

### Added — two integrity holes closed at the command that used to launder them

Both were found by reproducing them against the shipped binary, and in both the *gate* was already
correct: `check` named the tampered state at every step. What was missing is that naming it is not
refusing it, and in each case the next ordinary command wrote the evidence whose absence was the
finding — so the sequence ended green, permanently.

- **`lock-ledger-deleted` is now a refusal on `claim lock`, not only a finding.** Delete one
  claim's entry from the `ledger` map, flip `status: locked` to `draft`, rewrite the body, and
  `dossierx claim lock` used to succeed — `RecordApproval` wrote a *fresh* record over the
  rewritten content, and every check from then on exited 0 with zero findings. The claim's own
  `locked_at` stamp and dependency baselines survive the deletion (nothing removes them; `unlock`
  *releases* a record rather than deleting it), so "this engine locked it and the record is gone"
  is answerable, and locking is now refused with `integrity_failed`. The recovery is restoring the
  lock store — **not** `unlock → fix → lock`, which signs the attacker's edit. A claim this engine
  never locked is untouched by the gate, so a first lock still works normally.
- **`comment-digest-unrecorded` is now a refusal on `claim lock` and on every comment op.** An
  approval records the claim's comment digest in the same act, unconditionally — so on a claim
  whose digest key had been dropped, locking *manufactured* an entry from whatever the comments
  block said at that moment. Measured on a covered project: a human's open thread blocks the lock;
  forge `status: resolved` and drop that one key; `check` correctly reports
  `comment-digest-unrecorded`; `claim lock` then exits 0 and certifies the forged block; every
  later check exits 0. The human's objection is gone and the record says the review was clean.
  `dossierx comment add`/`reply` closed the same door from the other side — an *unknown* digest on
  a covered claim that carries threads is no longer treated as "cannot have drifted". Silent, in
  both, where evidence is honestly absent: an uncovered project, an absent digest store
  (`comment-digest-absent` is that cause, said once), and a claim with no threads at all.

### Added
- **`dossierx claim` — one noun for everything you do to a claim**: `show`, `list`, `new`,
  `lock`, `unlock`, `flag`, `reaudit`, `link`.
- **`dossierx claim show <id>`** — one call returning a claim's whole state: lifecycle status,
  lock state and timestamp, `review_pending` **and which of the three triggers caused it**, both
  edge directions (outgoing `mirrors`/`rests_on`/`governed_by`, and the derived incoming
  `mirrored_by`/`depended_on_by`), `implemented_in` with a per-file drift verdict, comment
  counts with the open thread ids, and `next_actions` — the legal next steps, computed from the
  same gates the write paths enforce, so the advice can never disagree with what the command
  would actually do.
- **`dossierx claim list`** with `--review-pending`, `--migrated`, `--drifted`, `--facet`,
  `--module`, and `--match`. `--match` is a fuzzy, ranked search over each claim's id and its
  derived title, so a human's "the retry-policy card in the contract facet" resolves to an id in
  one call; each result carries its `score` so an agent can tell a confident hit from a tie it
  should hand back.
- **`dossierx claim new <id>`** — the sanctioned way for an agent to author a claim. Since the
  release gates hand-editing claim YAML, an agent needs a way to write one at all; this writes
  `<claims_dir>/<id>.yaml` shaped to pass the lint suite immediately, validates the project with
  the new claim in it, and reports the verdict. The id grammar (`module.facet.slug`, kebab-case
  slug, module and facet the project actually declares) is enforced at the door rather than
  after the write. Draft authoring is deliberately unfrictioned: no `--reason`, no confirmation.
- **`dossierx migrate --adopt`** — the seventh noun, and the only command in the surface a
  *human* is expected to run other than `serve`. It exists because adoption stopped being
  something a `check` does on its own; see the BREAKING section above. `--adopt` and `--reason`
  are required, `--dry-run` previews.
- **`dossierx check --validate`** — a read-only run over `internal/check`'s existing non-writing
  seam (the same one `serve`'s status endpoint uses). It exists because cutting `lint` would
  otherwise have turned the per-claim authoring loop into a writer.
- **`dossierx comment inbox`** — every open thread in the project in one call, oldest activity
  first, with `--since <RFC3339>` and an echoed `cursor` to poll with. Each thread carries
  **`agent_can_resolve`**, so an agent never spends a call earning `rights_denied` on a thread
  it was never allowed to close. `--since` is inclusive of its own second: comment timestamps
  have one-second resolution, and re-reporting a thread costs nothing while missing the human's
  comment breaks the entire loop.
- **A machine contract on every command.** `--format json|text` is global and **JSON is the
  default**; every run emits exactly one envelope — `{ok, command, data, warnings, error,
  stopped_at}` — and every failure carries a stable snake_case `error.code` (`lint_failed`,
  `claim_not_found`, `rights_denied`, `integrity_failed`, `unresolved_comments`, …) so a skill
  branches on a token instead of regexing an English sentence. `message` and `hint` are prose
  and will be reworded; `code` is the promise.
- **`--dry-run` on every mutating command**, reporting what would change, what is missing, and
  what else it affects. A dry run fails *only* when it cannot compute the preview: a refusal —
  including a missing required flag — is a **successful** blocked report (exit 0, `ok: true`,
  `data.blocked: true`), because "would this be allowed?" is a question, and answering "no" is
  not an error. `claim reaudit` keeps `--confirm` as its apply gate; `--dry-run` always wins
  over it.

### Added — integrity: the lock ledger

Claims are YAML in git, so nothing can *prevent* an edit. The goal is that no out-of-band edit
of a **locked** claim is *silent*. Before this release, every one of these was invisible: a
`status: draft` flipped to `locked` by hand (walking past the lint gate, hub-gating and the
unresolved-comment gate as though all three had passed); an edited locked body with no locked
dependents; a swapped `raw_html` on a locked, reviewed, allowlisted mockup — which the viewer
renders **unescaped**; a flipped `build_role`/`section`/`order`/`emphasis`; a comment thread
deleted straight out of the YAML; a `locked` flipped back to `draft` to dodge review.

- **The lock ledger.** Every legitimate approval — `claim lock`, a confirmed `claim reaudit`,
  `build-order lock` — now records `{hash, at, actor, reason}` for the artifact it approved, in
  `.dossierx-lock-store.json`. `reason` is the human's own approving words, carried in from
  `--reason`: the one part of the record a machine cannot generate for itself. `claim unlock`
  **releases** a record rather than deleting it, so the evidence that a claim was ever locked
  survives the window in which it matters.
- **`.dossierx-lock-store.json`, `.dossierx-comment-digest.json` and `.dossierx-flag-store.json`
  are TRACKED ARTIFACTS.** Commit them; never `.gitignore` them. CI compares the claims against
  the ledger, so a project that does not track it has no gate; and a `review_pending` claim whose
  flag-store entry did not travel with it reaudits to an *empty* proposal, whose `--confirm`
  clears the human's flag having applied nothing. Documented in README, FORMAT.md, the skills,
  the hook installer's own output, and the CI workflow template.
- **A new hash, `LockedClaimHash`, separate from `ContentHash`.** It is a **deny-list** over
  every persisted claim field except `status`, `review_pending` and `comments` (each excluded
  because the engine rewrites it as ordinary bookkeeping), so a field added to the schema
  tomorrow is signed by default. `ContentHash` — the dependency-drift baseline — is
  **byte-identical to v0.2.0**: widening it would have flipped every locked claim in every
  existing project to `review_pending` on upgrade day. It covers ten of the schema's fields;
  the nine it cannot see include `raw_html`, the only path in the engine that renders author
  bytes unescaped, which is why the ledger could not reuse it.
- **The comment digest lives in its own store** (`internal/digest`, `.dossierx-comment-digest.json`),
  refreshed on every legitimate comment write. Putting it in the lock store would have made
  `dossierx serve` a lock-store writer and falsified this release's own headline guarantee.
- **Ten named findings**, stable strings the hook, CI and the skills branch on:
  `lock-ledger-absent` (locked claims but no ledger file — a hard error, never a silent pass,
  because "no ledger means bless everything" would make `rm` the universal bypass),
  `lock-ledger-missing`, `lock-ledger-released` (a `locked` claim whose only record was released
  by an unlock — a released record is not a standing approval, and the hash still matches because
  the hash excludes `status`, so `lock` → `unlock` → hand-edit `status:` back was otherwise a
  complete bypass that fired no rule at all), `lock-content-drift`, `lock-ledger-orphan`,
  `lock-ledger-abandoned` (an unreleased record whose claim FILE is gone — every other per-claim
  rule is driven by the claims that exist, so deleting one walked past all of them at once and
  there is no `claim delete` verb to have made it deliberate), `comment-ledger-drift`,
  `build-order-content-drift` and `build-order-ledger-missing` (a locked
  `.build-order.<module>.json` is what an implementing agent actually builds from; its approval
  record was being written and never read, so reordering the phases or splicing the frozen
  `hashes` baseline changed the plan with no finding anywhere), and
  `lock-ledger-unreadable` (a ledger that exists but will not parse fails closed and loudly).
  The gate is deliberately **not** a lint: registering these in the lint registry would let one
  tampered file freeze locking project-wide and stop the viewer regenerating. It runs as
  `check`'s last step, after the catalog and viewer are written — a disputed project still
  regenerates its documentation, it just does not exit 0.
- **`dossierx check --staged`** judges the **git index** — what the commit will actually
  contain — and writes nothing at all. `project.config.yaml`, every claim, the lock ledger, the
  comment digest and every locked build order all come from that one snapshot, so no unstaged
  edit can change the verdict on a commit that does not carry it. Claim content is read through
  a single `git cat-file --batch` over the index's own object ids, never conditionally off disk:
  git's stat cache and its `assume-unchanged` / `skip-worktree` bits are attacker-writable, and
  a gate whose evidence source they can choose is not a gate. Outside a work tree it warns and
  exits 0.
- **A pre-commit hook installer**, `scripts/install-git-hook.sh` (plus `install-git-hook.ps1`
  for PowerShell). It **asks before writing anything**, resolves `core.hooksPath` instead of
  assuming `.git/hooks` (so a repo using husky/lefthook is installed into *its* hook directory,
  never hijacked), handles linked worktrees, refuses to replace a foreign hook without
  `--force`, and is a no-op when re-run. The hook body is embedded in the one file so an agent
  can drop it into a project that has the binary but not this repository. In a repository holding
  **more than one** DossierX project the hook checks **every** one of them, in index order;
  `DOSSIERX_CONFIG` narrows it to a single project but is never required. It also refuses the
  commit — rather than reporting "no project here, skipping" — when it located a config that
  `dossierx` then returned `config_not_found` for, since a gate that cannot run must not pass.
- **A CI workflow template**, `scripts/ci/dossierx-check.yml`, to copy into a consuming
  project. **CI is the authority, not the hook**: git does not run `pre-commit` for a clean
  merge, a rebase, a cherry-pick or a revert, `--no-verify` is one keystroke away, and most
  contributors never installed the hook. If you adopt one of the two, adopt CI.
- **Adoption**, covered in full by the BREAKING section at the top of this release. In brief: a
  pre-ledger project is no longer grandfathered by any `check` and is adopted only by
  `dossierx migrate --adopt`, with claims and already-locked build orders adopted in the same act
  (splitting the two halves across the ledger line would leave a project half-covered).
- **`check --staged` judges the git index and nothing else** — no parent-commit comparison, no git
  history, the same verdict in every clone. See the `--staged` section above for what that does and
  does not detect, and where the remainder is caught.
- **Moving `claims_dir` needs no flag, and there is deliberately no escape hatch.** An exemption
  switch on an integrity gate is the attack, since the party who reliably remembers to set it is
  the one who read the source looking for it. None is needed either: move the claims and the
  stores in the **same commit**, keeping the claim files byte-identical, and it passes because
  every locked claim is still reachable and still hashes to its existing record. A repoint that
  **strands** locked claims fails from state alone, as `lock-ledger-abandoned` once per claim —
  their records are left naming claims the project can no longer see.

### Added — graph integrity and readability

- **New `self-edge` lint** (error): a claim may not name its own id in `rests_on`, `mirrors`, or
  `governed_by`. A self-edge is trivially satisfied by every content rule — a claim always
  equals itself, always mirrors itself back, always resolves — so it asserted nothing while
  looking like a well-formed edge.
- **New `governed-cycle` lint** (error): `governed_by` is now cycle-checked, with its own
  message distinct from `cycle`'s. Following `governed_by` must reach `type: none` in finitely
  many steps; a cycle means a set of claims whose authority rests only on each other, which is
  to say on nothing.
- The `cycle` lint's depth-first search is now an **explicit stack** rather than recursion. Its
  depth was the longest authored edge chain in the project, with no engine-imposed bound.
- **Readable edge labels in the viewer** (issue #11). A claim-to-claim edge used to render as
  its raw id. It now renders as a derived label with prefix elision keyed on how far the target
  is from the claim doing the pointing — bare within the same facet, `facet › Label` across
  facets, `module · facet › Label` across modules. The machine id stays reachable via
  `data-claim-id` and a tooltip, and an id that is not exactly three non-empty segments renders
  as the **raw id, verbatim** — never a partial label — because rendering does not run the lint
  suite.

### Fixed

- **`dossierx build-order propose` now releases the approval it discards, and a hand-cleared
  `"locked": false` is a finding no matter what else was edited beside it.** The build-order
  orphan rule could only identify a *lone* flag flip: it re-signed the artifact as if the flag
  were still true and required that hash to match the record, because without that test it could
  not tell a hand edit from the honest window between a re-propose and the lock that follows.
  A content edit re-signs to something else, so flipping the flag **and** gutting the phases in
  the same write was strictly quieter than flipping the flag alone — `check --validate` reported
  `OK`, exit 0, and offered `dossierx build-order lock` as a next step over a sequence nobody
  approved. `propose` now writes the truth instead of leaving the gate to guess it: it releases
  the module's ledger record as it overwrites the artifact (under the lock-store sentinel, held
  across both writes, so contention refuses with nothing written). The honest window is therefore
  the only unlocked artifact whose record is *released*, and the rule needs no exception —
  any unlocked artifact under a **standing** record is `build-order-ledger-orphan`. The
  `--dry-run` discloses the release as a side effect.
- **A run that adopts a comment digest says so.** `lock.PrepareStore` left the adopted ids on the
  store and `cmd/dossierx` dropped them, so `dossierx check` printed `ok:true`, zero findings,
  exit 0 on the very run that recorded a hand-added comment block as truth. The ids now ride the
  same channel as the adopted claim records, reaching `data.comment_digests_adopted` and the
  envelope warnings — with the recovery that actually applies, which is **not** a re-lock: no verb
  in this binary clears a recorded comment digest, so the only way back is version control.
  Deliberately silent when the digest store is being *created* (a new project, or the one-time
  `migrate --adopt` crossing), where every block is adopted by definition and nothing has been
  laundered.
- **`claim lock --dry-run` no longer previews a lock the real run refuses.** Its
  `claim_is_draft` precondition read the claim file's own `status:` line — the exact line a hand
  edit rewrites — so a claim flipped out of locked without an unlock, still carrying a standing
  approval, previewed as lockable and was then refused `already_locked`. The preview now asks the
  question the real run asks, as a `no_standing_ledger_record` precondition. The refusal's
  `error.hint` also splits by state: a claim whose file says `locked` is pointed at `unlock`,
  while one that says `draft` under a standing approval is told to **restore from version control
  first**, since unlocking there accepts the edit that caused it.
- **A comment op refused because the digest store is unreadable now reports
  `comment_digest_unavailable`, not `internal`.** `internal` is defined as an unclassified
  failure — "a bug report, not a branch target" — and the reflex it invites is a retry. This
  refusal is deterministic and keeps failing identically until `.dossierx-comment-digest.json`
  is restored, so classifying it as internal sent a caller into a retry loop over a write. The
  code carries the fact that makes it actionable: **nothing was written**.
- **`build-order lock` refusing a hand-edited artifact now reports `build_order_hand_edited`,
  not `build_order_refused`.** Every recovery documented for `build_order_refused` is a repair
  to the *claims* (lock the remaining ones, resolve a thread, set a missing `build_role`, break
  a `rests_on` cycle). In this refusal the claims are correct and the *artifact* is not, so an
  agent following any of them inspected correct claims, found nothing to fix, and looped. The
  recovery for the new code is the one that works: re-`propose`, then `lock`.
- **A fresh project now acquires its comment digest store at the moment its lock store is
  created**, not only when an older project migrates across. A project that reached
  ledger-covered through its first `claim lock` never ran a migration, so it ended up
  ledger-covered with no digest store — on disk, indistinguishable from a project whose digest
  store had been **deleted**. Deleting the store from an already-covered project is still never
  silently re-created, so the deletion stays visible to the gate.
- **`FORMAT.md` no longer states that `governed_by` is hub-gated.** Hub gating walks `mirrors`
  and `rests_on` only, so a doctrine claim named *only* by `governed_by` is not gated — a reader
  who believed otherwise would drop the redundant `rests_on` edge and lock against an unapproved
  doctrine claim. `FORMAT.md` also no longer claims there is deliberately no comment-digest
  absence rule; `comment-digest-absent` ships, and its real boundary is now documented.
- **The viewer's 💬 chip now appears on every card, not only on cards that already have a
  thread** — so the first comment on a claim can actually be opened, which was the whole
  premise of the human review loop. Two gates were involved and both had to move together: the
  server emitted no chip for a zero-thread claim, and the client hid any chip reading `0`, so
  an empty chip would have vanished the moment it was clicked. Empty chips are now hidden only
  when no live comment API answered — the static `file://` export, where there would be nothing
  to open — and the three chip states (`--open`, `--resolved`, `--empty`) are mutually
  exclusive, so "no comments" no longer reads as "everything raised was settled".
- **Every command the engine advises you to run is a command that exists** — and, where the
  verb requires it, one that would succeed as printed. `check`'s next steps named five retired
  invocations (`dossierx lock`, `dossierx reaudit`, `dossierx implink set`, and a
  `dossierx comment resolve` that this release deliberately removed from the CLI); they now
  name `claim lock … --reason`, `claim reaudit`, `claim link`, and — for an open thread — the
  human's viewer rather than any agent-runnable command, because resolving a thread is the
  approval itself. `lock`'s and `build-order propose`'s comment refusals and `build-order`'s
  cycle diagnostic were stale in the same way and were corrected alongside. This matters more
  than the wording suggests: the v0.3.0 skills tell an agent to read `next_actions` and
  `error.hint` instead of re-deriving the lifecycle, so a stale hint is advice an agent acts on.
- **Generated viewers no longer advertise a deleted command.** Every `viewer/index.html`
  carried `generated by dossierx render … re-run "dossierx render"`; the banner now names
  `dossierx check`, which is the command that actually regenerates it.
- **`claim lock` refuses a claim that is already `locked`** (`already_locked`), instead of
  re-signing the ledger over whatever the file currently says. Re-locking was the single command
  that laundered every gate this release adds: it re-stamped the approval over drifted content,
  re-snapshotted the dependency baselines, cleared `review_pending` with no diff shown, and left
  the human's `claim flag` entry stranded where `reaudit` could no longer reach it — all at
  exit 0, on the verb the drift finding itself names. `lock --dry-run` had reported
  `blocked: true` for this case all along; the write path now agrees with its own preview.
- **A comment write no longer re-blesses a tampered `comments:` block.** Every comment operation
  recorded the digest unconditionally, so a single ordinary `comment reply` — on an unrelated
  thread, on the same claim — erased a standing `comment-ledger-drift` finding and made a
  forged `resolved` the recorded truth. The digest is now compared against the claim as it was
  read, and a disagreement refuses the write (`comment_digest_drift`) rather than overwriting the
  record. Adoption on a never-seen store is unchanged.
- **The pre-commit hook no longer refuses every commit in a repository whose project lives
  under a non-ASCII path.** git's `core.quotepath` defaults to *true*, so the hook's
  `git ls-files` discovery query got a project at `café/project.config.yaml` back as the
  C-quoted string `"caf\303\251/project.config.yaml"` — surrounding double quotes and all.
  Handed to `--config`, that names no file, `dossierx` answers `config_not_found`, and the
  hook's (correct) rule that a config it discovered but cannot open is a refusal rather than a
  skip did the rest: **every** commit refused, on every branch, for every developer, including
  commits touching no claim at all, until somebody uninstalled the gate. Discovery now passes
  `-c core.quotepath=false`, the same override `check --staged`'s git runner has always used.
  Pinned by a `scripts/hook-smoke-test.sh` case that asserts both halves — an honest commit
  still passes, and a tampered locked claim is still refused — because "still refuses" alone is
  satisfied by a hook that refuses unconditionally, which was the bug. The hook body's marker is
  now `pre-commit v4` (see the two entries below for what v4 added); re-run the installer to
  pick it up. A v3 install is classified `outdated` and replaced cleanly by re-running the
  installer — no `--force` needed.
- **`check --staged` no longer disarms itself when `claims_dir` points outside the config's own
  directory.** Every git pathspec was built relative to the *config file's* directory, so the
  ordinary monorepo layout — `docs/project.config.yaml` with `claims_dir: ../claims` — produced
  the spec `../claims`, which mapped to the deliberate "no index here" escape hatch and exited 0
  having evaluated nothing. A tampered locked claim committed with **no hook output at all**.
  The git runner now anchors itself at `git rev-parse --show-toplevel` and takes the project's
  position from `--show-prefix` (asked of git rather than derived by string arithmetic, so
  macOS's `/var` vs `/private/var` symlink cannot desync it); the escape hatch is now reached
  only when a path is genuinely outside the work tree. `data.staged_files` consequently carries
  repository-relative paths — identical in the layout where the config sits at the repo root,
  and only different for the layout that used to be broken.
- **A skipped `check --staged` is no longer indistinguishable from a pass.** `--format json`
  never printed the skip warning and the hook branched on the exit code alone, so a gate that
  evaluated nothing looked byte-identical to a clean run. The hook (`pre-commit v4`) now matches
  `data.skipped`, re-runs in text mode so the reason reaches the screen, and **refuses** the
  commit, naming the likely cause and the `--no-verify` hatch.
- **An untracked `project.config.yaml` no longer judges tracked content.** `check --staged` fell
  back to the *worktree* config whenever the config was merely untracked — and an untracked
  config can be edited without staging anything, so pointing it at a pristine decoy directory
  made the gate report `OK`, exit 0, over an index carrying a tampered locked claim. It now
  refuses with a distinct, non-skippable error unless the index genuinely holds nothing to judge
  (no claim blob, no lock ledger, no comment digest store), which keeps the first-commit case the
  fallback exists for working. Ordinary repository YAML — workflows, chart values — does not
  decode as a claim, so a repo full of unrelated YAML is not turned into a refusal.
- **The viewer's browser suite is actually run.** `viewer-tests/` is a separate Go module (it
  needs `chromedp`; the engine's `go.mod` stays cobra + yaml.v3), which means the root
  `go test ./...` cannot descend into it — and until now nothing else did either: no CI job, no
  Makefile target. Its assertions against the viewer's inline JavaScript, including this
  release's comment-chip suite, had only ever executed on a maintainer's laptop while CI was
  green on three platforms. There is now a `viewer` CI job running it against the runner's
  headless Chrome, and a `make viewer-test` target (plus `make hook-test`) so both
  outside-the-root-module suites are reachable locally. The job sets `DOSSIERX_TEST_BROWSER`
  explicitly, because the suite *skips* when it cannot find a browser and a skip in CI is
  indistinguishable from a pass. `tests/nested_module_coverage_test.go` fails the build if a
  nested module is ever added without both.
- **`check --staged` reads `project.config.yaml` from the index as well.** It read the claims,
  the lock ledger and the comment digest from the git index and the config from the working tree,
  so an UNSTAGED one-line `claims_dir:` edit pointing at an empty directory enumerated zero
  claims, audited zero claims and passed every commit that followed. The gate now judges one
  consistent snapshot.
- **`check --staged` no longer trusts `git diff` to decide which files it may read from the
  worktree.** git deliberately omits paths carrying the assume-unchanged or skip-worktree bit, so
  those were precisely the paths whose worktree copy was read in place of the index blob —
  the substitution `--staged` exists to prevent. Both cases are pinned end to end in
  `scripts/hook-smoke-test.sh`.
- **`review_pending` reconciliation consults the flag store.** `check` re-derived only two of the
  three triggers, so a `review_pending: true` line deleted by hand (or by a bad merge) on a
  *flagged* claim was never restored and never reported: the claim vanished from
  `claim list --review-pending`, `reaudit` refused it, and the recorded doc/code mismatch became
  unreachable. It now uses the same shared predicate the comment ops and `reaudit` use.
- **`comment inbox` no longer drops a thread the human REOPENED.** `last_activity` was the newest
  reply's timestamp, or the thread's own creation time — never the resolve or reopen — so a
  reopened thread's activity sorted *before* any cursor the agent already held and disappeared
  from every incremental `--since` poll, which is exactly the message the inbox exists to deliver.
- **`comment inbox --since` validates its argument.** A malformed value answered `ok: true` with
  an empty inbox and echoed the bad value back as the next cursor, so the failure was
  self-perpetuating and byte-indistinguishable from "the human left you nothing new". It is now
  refused (`bad_request`) and normalized to UTC before comparison.
- **`build-order lock` on a stale order returns `build_order_stale`**, the code the skill's
  refusal table has always documented, instead of the generic `build_order_refused` whose three
  documented recoveries do not apply — leaving an agent that branches on `error.code` (as it is
  told to) with no reachable path to "re-propose, then re-lock".
- **`check`'s next steps only name a claim that would actually lock.** It named the first draft
  claim in load order without evaluating the gates, so on a module drafted alongside its own
  dependencies — including this repository's own shipped fixture — the one command an agent is
  told to trust exited 1 with `lint_failed`. The example is now chosen through the same gate
  evaluation `claim show` reports, and when nothing is lockable yet it says so.
- **A noun with no subcommand emits an envelope.** `dossierx claim`, `comment`, `build-order` and
  `skills` printed help prose on stdout and exited 0, so an agent that dropped a subcommand got
  the success signal plus output its JSON parse throws on — and no way to tell that from a
  command that genuinely had nothing to report. They now behave like any other bad invocation:
  one envelope, `usage`, exit 1, with `error.hint` naming the available leaves. An unknown leaf
  (`dossierx claim nonsense`) lands in the same place and names what was typed.
  `dossierx version` is the verb that reports the version; `--version` is unchanged.

### Changed
- The CLI is **20 leaf commands under 7 nouns**, down from 26. A test pins the exact set, so
  adding to the surface is a decision someone makes on purpose.
- `--reason` is **required** on `claim lock`, `claim unlock`, `claim reaudit --confirm`, and
  `build-order lock`. Under the new split the human never types these — they say "good, lock it"
  in chat and the agent executes — so `--reason` is where the human's own approving words enter
  the record.
- Exit codes are **unchanged**: still 0 / 1 / 2 with the meanings the README documents. The
  fine-grained signal is `error.code` in the envelope, not a new status.
- `dossierx check` now runs **four** stages, not three: lint, catalog, render, and the
  lock-ledger gate. `--validate` is the read-only form — it runs the lint gate and the ledger
  gate in memory and writes nothing, and is honest about what it therefore does not do (no
  `review_pending` reconcile, no catalog, no viewer, no source scan for code links).
- `dossierx skills export <dir>` now writes **five** skill bundles. A project that exported the
  skills before must **re-export** to pick up the new router and the rewritten companions; the
  export overwrites in place, so re-running the same command is all that is needed.

### Docs
- **README rewritten around the two roles.** It now opens on who does what, carries a
  copy-paste bootstrap block a human hands to their agent (install, export the skills, propose
  the config, *ask* before installing the git hook, prove itself with `check`, commit the
  ledger, then hand `dossierx serve` back to the human), and documents `dossierx serve` as the
  human's one command. The lint → catalog → render walkthrough and the per-verb command table
  are gone: a human is not expected to run any of it, and the CLI is now documented as a
  machine contract — the envelope, `error.code`, `--dry-run`, and the unchanged exit codes.
- **FORMAT.md gained an "Integrity invariants" section**: the two tracked ledger files, what
  `LockedClaimHash` signs and the three fields it deliberately does not, all six findings and
  the invariant each one enforces, and how one-time adoption works. It also gained the three
  **graph invariants** (`rests_on` acyclic, `mirrors` a reciprocal 2-cycle, `governed_by`
  terminating, and no self-edges in any of the three), and a short, quotable statement of the
  **markdown ceiling** — the subset `body`, `rows` cells, `steps` and comment bodies all render
  through, with everything outside it staying literal text.

## [0.2.0] - 2026-07-26

### Added
- **Comments on claims** — threaded, Google-Docs-style review discussion attached to any claim,
  so a human and an agent can talk *about* a claim without editing it.
  - New `dossierx comment` command group: `add`, `reply`, `resolve`, `reopen`, `edit`, `delete`,
    and `list` (with `--open` and `--json`). Every mutating verb takes `--as human|agent`
    (recording the actor's role, which the advisory-rights rule keys off) and takes the
    project-wide claims lock, so concurrent CLI and browser edits can't clobber each other.
    Threads and replies carry engine-minted ids; the new `comments:` claim field is `omitempty`
    and **excluded from a claim's content hash**, so commenting on a claim never rewrites an
    uncommented claim's bytes and never flips its dependents to `review_pending`.
  - New `dossierx serve`: a localhost-only HTTP server that renders the claims viewer from
    memory and exposes the same comment operations to the browser, with an interactive thread
    panel and composer, a same-origin admission layer (Host/Origin checks, no CORS), and live
    reload that pushes changes over server-sent events as claim files change on disk. Binds a
    random high port by default (`--port` to override) and never writes `viewer/index.html` or
    `.catalog.json` on a page load. **Adds no new runtime dependency** — the file watcher is a
    standard-library modification-time poll, so the runtime stays cobra + yaml.v3 only.
  - An open comment thread is now a third `review_pending` trigger on a locked claim, alongside
    dependency drift and `dossierx flag`. A claim **cannot be locked while it has an unresolved
    comment thread** (and `dossierx build-order propose` refuses a module with any open thread);
    `review_pending` clears when the last open thread is resolved, unless drift or a flag also
    stands. `dossierx reaudit` refuses a claim that is `review_pending` only because of an open
    thread — there is no content diff to confirm, so it directs you to resolve the thread
    instead. `dossierx check` reports open-comment counts per module and points its next-steps
    at the exact `dossierx comment resolve` command.
  - New `comments-unresolved` lint (warning severity): surfaces claims that still carry an open
    comment thread.
- A fourth embedded Claude Code skill, **`dossierx-comments`**, teaching an agent when to
  comment versus `flag` (the discriminator is "is there a specific proposed wording change?"),
  the advisory-rights rule (an agent never resolves a human-opened thread), and how an open
  thread gates locking. The three existing skills were updated for the new three-trigger
  lifecycle and cross-linked to it.

### Changed
- `dossierx skills export` now writes **four** skill bundles instead of three. Projects that
  previously exported the skills (e.g. into `.claude/skills/`) with the old three bundles must
  **re-export** to pick up `dossierx-comments`; the export overwrites in place, so re-running
  the same `dossierx skills export <dir>` is all that's needed.

### Docs
- Documented the `comments:` claim field and rewrote the lock lifecycle in `FORMAT.md`,
  `README.md`, and the skills to the three-trigger (dependency drift / `dossierx flag` / open
  comment thread) model with its three matching clearers.

## [0.1.2] - 2026-07-25

Consolidated audit-fix release: a deep audit against a real 202-claim consumer project
surfaced 25 confirmed defects, fixed together here rather than as a stream of point
releases. Despite adding user-facing capabilities this is a patch bump — `internal/` is not
importable, there are no breaking CLI changes, and the lock-store migrates automatically.

### Added
- `dossierx version` subcommand and a `--version` flag (previously the binary could not
  report its own version, and the release-time `-X` ldflags targeted variables that did not
  exist).
- Markdown `[text](url)` links now render as anchors in claim bodies **and** in `table`
  cells; backtick code spans now render inside table cells too. Link URL schemes are
  allowlisted (`http`, `https`, `mailto`, relative, `#`-fragment); `javascript:`, `data:`,
  and `vbscript:` are neutralized to inert text. Bare URLs are not autolinked.
- New `status-shape` lint: `status` must be exactly `draft` or `locked`.
- `rows-shape` now flags any non-string table cell (number/bool/list/map) instead of letting
  it render as Go-native text (e.g. an unquoted `1.0` silently becoming `1`).

### Fixed
- A YAML file containing a second `---`-separated document silently dropped all but the first
  claim; it is now a hard load error (one claim per file is enforced).
- `lint --json` printed `null` instead of `[]` when there were no findings, and
  error-severity findings serialized with an empty severity; both now emit correct JSON.
- `lock` / `unlock` / `flag` returned exit code 1 for an unknown claim id; they now return
  exit 2, matching the documented exit-code contract (as `deps` / `reaudit` already did).
- `build-order status` and `implink status` accepted an unknown `--module` and exited 0; they
  now reject it.
- The invalid-`layout` lint message omitted `mockup`; it now lists all seven layouts.
- Dependency-hash baselines were keyed by dependency id alone and shared across dependents,
  so locking or reauditing one claim erased another's drift baseline and that claim would
  never flip to `review_pending` when the shared dependency changed. Baselines are now keyed
  per-dependent; the on-disk lock-store is versioned and migrates automatically, re-arming
  baselines for every currently-locked claim from current content on first run so drift
  detection is live immediately after upgrade without a manual re-lock.
- `unlock` left a claim's pending flag in the flag-store, so a later dependency-drift reaudit
  could silently re-apply stale pre-unlock content; `unlock` now clears the flag entry.
- `unlock` hard-failed when the flag-store file was missing or corrupt; flag-clearing is now
  best-effort — a missing store is silent, an unreadable one warns and still returns the claim
  to draft — so the recovery escape hatch stays reliable.
- `flag` on a `table` / `steps` / `mockup` claim rewrote only the body, leaving the rendered
  rows/steps/raw HTML stale while clearing `review_pending`; `flag` is now refused on those
  structured layouts (use unlock → edit → relock).
- Build-order staleness is now computed structurally: `status` re-derives the order a fresh
  `propose` would produce from the current claims and flags the locked artifact stale whenever
  they differ. This covers every order-affecting change in one check — a covered claim's
  `build_role` or `order:` edit, a source-file rename, `rests_on` reordering, additions,
  deletions, and an excluded claim promoted into a phase (or edited to an empty/invalid role) —
  plus content edits via the existing per-claim hash. It also runs for a locked module that
  covers only out-of-scope claims, which previously escaped every check and could not be relocked.
- Build-order staleness ignored newly-added claims (an artifact could silently omit a claim);
  additions now flag the artifact stale, symmetric with deletions.
- `build-order lock` re-blessed a stale artifact without recomputing its order; it now refuses
  a stale artifact and directs you to re-propose first.
- The Build Order section was emitted without an id and hidden by the facet-tab logic on every
  view, making the feature unreachable; it now renders visibly and its cards are deep-linkable.
- A module overview/router claim was injected into every facet with the same id, producing
  duplicate ids (invalid HTML) and broken deep-links; the canonical id is now stamped on a
  single copy while the overview stays visible in every facet.
- The offline-guarantee test walked the whole repo including built site bundles, so it went
  red locally after a site build while passing on a clean CI checkout; it is now scoped to the
  engine directories with a positive control.

### Security
- The `raw_html` mockup allowlist only inspected double-quoted attributes, so single-quoted,
  unquoted, and valueless event handlers, styles, and external `src` bypassed it. It is
  replaced with a default-deny parser covering every quote form, and an `img` `src` is now
  HTML-entity-decoded and stripped of ASCII control bytes before the relative-only check, so
  neither an entity-encoded (`&#47;&#47;host`) nor a control-char-obfuscated (`ht&#9;tp://host`)
  absolute/external URL can slip past it.
- `render` and `catalog` never ran the `raw_html` gate (only `lock` did), so they could
  publish unreviewed or non-allowlisted mockup HTML into the viewer; both now enforce the gate
  and fail on a violation.

### Docs
- Corrected the build-order skill (orientation-note/overview claims do carry a `build_role`
  and render in the orientation phase). Updated the claims and code-links skills, `FORMAT.md`,
  and the marketing site to reflect the behavior above.

## [0.1.1] - 2026-07-24

### Fixed
- `layout: steps` claims rendered a numbered circle (`.snum`) that sat visibly higher than the
  first line of step text. Step text is routed through the shared markdown renderer, which wraps
  it in a `<p>`; the `<p>`'s default browser top margin pushed the text down inside the
  `display:flex` step row while the fixed-height number circle stayed flush at the top. The
  default viewer stylesheet now resets step-body block margins (first-child top / last-child
  bottom) so the number and first line align. Affects every `layout: steps` claim in any project
  using the default viewer theme; a project overriding `style.css` is unaffected.

## [0.1.0] - 2026-07-23

### Changed
- Renamed every generic "docs" placeholder to the tool's actual name, `dossierx`: CLI-invocation
  examples across comments, tests, README/ROADMAP/FORMAT, and the website; the `docs-claim:`
  source tag (including the real Go regex in `internal/implink/scan.go`); `docs-v1` naming in
  the skill docs; and the default viewer title (`"docs viewer"` → `"dossierx viewer"`).

### Breaking
- `.docs-lock-store.json` and `.docs-flag-store.json` are renamed to `.dossierx-lock-store.json`
  and `.dossierx-flag-store.json`, with no migration. An existing project's lock/flag store will
  not be found after upgrading past this release — hence the minor version bump rather than a
  patch, under pre-1.0 semver.

## [0.0.3] - 2026-07-22

### Added
- The rendered viewer's sidebar now shows a "Generated <timestamp>" footer,
  the same render-time timestamp already stamped into the leading
  generated-by HTML comment, so a reviewer can tell how fresh the page is
  without needing to view source.

## [0.0.2] - 2026-07-22

First real CI run (Linux/Windows/macOS matrix, `-race`, gofmt, golangci-lint) surfaced gaps
that only local macOS testing had missed:

### Fixed
- Two files had minor gofmt drift.
- The CLI-integration test harness built the `dossierx` test binary without a `.exe` suffix on
  Windows, so `os/exec` couldn't launch it.
- Two POSIX-permission-based tests (unreadable file, read-only directory) don't apply under
  Windows's ACL model; skipped there.
- A concurrency test's non-trimpath "negative control" assertion is inconclusive on GitHub's
  windows-latest image (trimpath-equivalent by default); skipped there, the actual positive
  guarantee (a `-trimpath` build doesn't leak paths) is unaffected and still runs everywhere.
- **Real bug:** running many `dossierx lock` invocations concurrently against the same
  `claims_dir` could fail on Windows with a transient "being used by another process" error —
  Windows's mandatory file locking can collide a concurrent atomic rename with a concurrent
  read of the same claim file, unlike POSIX's atomic rename semantics. Both the read and write
  paths in `internal/loader` now retry a few times with a short backoff, Windows-only.
- `golangci-lint` config/version pinning tightened so CI's linter binary matches this module's
  actual `go 1.26` floor.

## [0.0.1] - 2026-07-21

DossierX is a config-driven CLI that turns YAML "claims" — atomic, reviewable facts about a
system — into a linted, validated, human-reviewable HTML documentation site, with a built-in
audit trail via a lock/lint/reaudit lifecycle: a claim is freely editable while in `draft`,
gets promoted to `locked` only once it passes lint, and any subsequent drift (a changed
dependency, code that no longer matches) is surfaced as `review_pending` and resolved through
an explicit, confirm-before-write reaudit rather than a silent auto-update.

The engine originated as an internal documentation tool built and hardened against real,
multi-module projects — proving out the claim schema, the lint → catalog → render → check
pipeline, the lock lifecycle, per-module build ordering, and claim-to-code linking against
production use before anything here was written with an external audience in mind. This
release extracts that engine into its own repository and genericizes it: every project-specific
name, facet, and module that had leaked into the original code has been removed, so the only
project-specific input the CLI now reads is a project's own `project.config.yaml`.

This is DossierX's first public release. It ships the `dossierx` CLI (`lint`, `catalog`,
`render`, `check`, `deps`, `coverage`, `stale`, `lock`/`unlock`, `reaudit`, `build-order`,
`flag`, `implink`), documented in [README.md](README.md), along with three Claude Code skills
in `skills/` for projects that consume DossierX to author claims, derive build order, and link
code back to claims from within an agentic workflow.

[Unreleased]: https://github.com/BarterX-Tech/dossierx/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.2.0
[0.1.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.2
[0.1.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.1
[0.1.0]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.1.0
[0.0.3]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.3
[0.0.2]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.2
[0.0.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.1
