---
name: dossierx
description: >-
  Router and machine contract for DossierX — the CLI that turns a project's atomic YAML "claims"
  into a reviewable HTML viewer, and that an agent OPERATES while a human REVIEWS. Load this FIRST
  and ALWAYS in any repo that has a project.config.yaml plus a claims/ directory, before running
  any DossierX command. It is short on purpose: the eight nouns, JSON envelope, exit codes, error.code
  recovery table, dry-run rule, five rules that never bend, which command (flag vs unlock vs reaudit,
  track vs module depends_on), the governed_by-is-gone migration, and companion skill routing. Load a companion skill only when this one sends you there.
---

DossierX turns a project's `claims/` directory — one atomic, reviewable YAML fact per file — into
a linted, dependency-checked HTML viewer. A claim starts `draft` (freely editable) and is promoted
to `locked` (frozen; changes require a recorded human approval). Everything the tool knows lives in
`project.config.yaml` plus `claims/`. **You are the operator, the human is the reviewer:** they read the
viewer, comment, click Resolve and tell you what to do; you run every command, they run one.

| | Agent (you) | Human |
|---|---|---|
| Surface | the CLI — all 22 commands | the viewer, via `dossierx serve` — including its **Constitution** pin above Modules (the project roof, not a module: The file \| Project claims), its **Tracks** group and per-track pages, its **claims graph**, the pane that draws `rests_on`, filters by track, and overlays isolated claims, dependency cycles, review-pending and open threads |
| Freely | author, edit, restructure, delete **draft** claims; reply to any thread; run `dossierx check` as often as you like | read anything; comment on any card; resolve/reopen/edit/delete their own messages |
| Never | change a **locked** claim without their recorded approval; lock/unlock/flag/reaudit unasked; resolve or reopen a thread a human opened; edit or delete a comment | — |

## The eight nouns, twenty-two leaves
```
dossierx check                             # the whole pipeline; --validate = read-only, --staged = judge the git index, write nothing — neither proves code links
dossierx claim  show list new lock unlock flag reaudit link recover-approved-content
dossierx comment inbox list add reply
dossierx constitution show lock            # the project-root constitution.yaml — the roof, not a module: show prints the words, lock records the hash
dossierx track list show status            # read-only: the cross-cutting feature axis
dossierx serve                             # the human's one command
dossierx skills export [dir]
dossierx version
```

A claim joins a track via `tracks:` in its own YAML, so changing membership on a **locked** claim is
`unlock → fix → lock`. `track` never edits anything and `track status` never gates a lock.

`claim recover-approved-content` is a ONE-TIME migration for a project that locked before the ledger
kept the approved WORDING — those records signed a hash, not the text, so a claim edited after one shows
"the approved wording was not kept on the record" and no comparison. It finds each approval's own revision
in the project's git history by hashing it against the hash the record already signed. It creates no
approval: a non-matching revision is refused and an unfindable claim is named and left alone. `--dry-run`
previews, the write takes `--reason`; show the human the dry run first — it edits their approval file. If git cannot answer, it refuses with `git_unavailable` rather than an empty recovery.

There is no `lint`, `catalog`, `render`, `deps`, `stale`, `coverage`, `implink`, `migrate`, or
`comment resolve|reopen|edit|delete`; the table at the bottom maps each to its replacement.

`--format json` is the **default**: one envelope per invocation, on stdout, on failure as well as
success. `--format text` is the prose you paste into chat for a human.

```json
{"ok": true,  "command": "claim show", "data": { }, "warnings": ["[warning] orphan: ..."]}
{"ok": false, "command": "claim lock", "error": {"code": "unresolved_comments",
  "message": "...", "hint": "...", "details": { }}, "stopped_at": "lint"}
```

Branch on `error.code` and on fields inside `data`. **Never** regex `message` or `hint`: `code` is
a promise, prose is not. `stopped_at` names the pipeline step a partial run reached (`config`,
`load`, `reconcile`, `lint`, `catalog`, `conformance`, `render`, `scan`, `ledger`, `constitution`, `links`), and `data` still carries what
it produced. `ledger`, `constitution` and `links` are the ones to read closely: the catalog and viewer WERE regenerated and only
the commit is refused — a gate, not an outage (`constitution` is the roof gate: lock or re-lock `constitution.yaml`, nothing else moved). A **noun with no leaf** (`dossierx claim` alone) is
an ordinary failed invocation — one envelope, `usage`, exit 1 — not help text at exit 0.

Exit status is one of three, unchanged since v0.1: `0` success · `1` failure (a lint error, a
refused gate, a write error) · `2` not found, or not in the state the command requires.

**Proof or stop.** Evidence is what the tool emits — an exit code, a finding, a conformance result,
`data.code_links` — or what the human does — Resolve, `--reason`. Your saying "it matches" or "it is
synced" in chat is neither and is never a certificate; when the evidence is missing, stop and say what is missing.

## error.code → what you actually do about it

| code | exit | recovery |
|---|---|---|
| `config_not_found` | 2 | not a DossierX project (yet). Do not create one unasked — see Bootstrap below. |
| `invalid_config` | 1 | `project.config.yaml` exists but does not load or validate. Fix the field the message names and re-run. |
| `claim_not_found` | 2 | you guessed an id. Run `dossierx claim list --match "<what the human said>"` and confirm the id back to them. |
| `lint_failed` | 1 | findings are in **`data.lint_findings`** — on `check` and on `claim lock` alike (`claim lock` keeps a second copy under `error.details.lint_findings`). Fix the claims, then re-check **with the command that refused you**: `dossierx check --validate` after a `check` failure, `dossierx claim lock <id> --dry-run` after a `claim lock` failure. Re-running `check --validate` after a lock refusal is a **loop, not a recovery**: it does not re-attempt the lock, and it reports *zero* findings for every rule that keys off a claim's own status (`rest-on-locked`, `roll-up`) because the claim is still `draft` on disk. The dry run lints the about-to-be-locked form, which is the only form that answers. **If the run stopped at `load` on a `governed_by` key, read the "governed_by is gone" section below before anything else: you did not cause it, and "fix the claims" is not where you start.** (Lint findings key on `lint`; the ledger findings two rows down key on `rule`.) |
| `integrity_failed` | 1 | **read `data.ledger_findings` and branch on `rule`** — one code, three kinds of arm. `lock-ledger-pre-ledger` is a project whose lock store predates the lock ledger and that still holds something locked (see The pre-ledger crossing below) — it is SILENT on a pre-ledger project holding nothing locked, which crosses correctly on its next lock. `store-gitignored` is not tampering either: a store the engine writes under `build/ledger` or `build/code-links` is matched by `.gitignore` and untracked, so the approval never reaches the repository — recover by replacing the pattern with the `RecommendedGitignore` block (README's "Where DossierX writes") or pointing `build_dir` elsewhere; `restore from git` and `unlock → fix → lock` are both wrong for it, an agent looping either one is chasing an unchanged finding about *where the store sits*, not its content. Everything else — `lock-ledger-missing`, `lock-ledger-deleted`, `lock-content-drift`, `lock-ledger-released`, `lock-ledger-orphan`, `lock-ledger-abandoned`, `lock-ledger-absent`, `lock-ledger-downgraded`, `comment-ledger-drift`, and the `comment-digest-*` family — is a locked artifact moved outside the approval path: **do not re-lock to make it go away**, restore the file from git or unlock → fix → lock. Two of them are now refusals on the WRITE path too, so you will meet them as a failed `claim lock` and not only as a finding: `lock-ledger-deleted` and `comment-digest-unrecorded`. Re-locking was the step that erased each of them, so there is no command that clears either — the recovery is restoring the named store file, and `unlock → fix → lock` is **wrong** here because it signs the edit. |
| `unresolved_comments` | 1 | the claim has an open thread. Reply on it; the **human** clicks Resolve in the viewer. That click is the approval this gate waits for. |
| `not_review_pending` | 2 | you reached for `claim reaudit` on a claim that is not drifting. The general edit path is unlock → fix → lock. |
| `review_pending` | 2 | the claim IS pending, and that is what blocks you. `dossierx claim show <id>` names the trigger. |
| `already_locked` | 1 | the claim is **already** locked, and `lock` refuses rather than re-signing it — a second lock would stamp a fresh approval over content nobody approved and clear `review_pending` with no diff. To change it: `unlock` → fix → `lock`. If a gate reported drift on it, restore the file from git instead. |
| `CONSTITUTION_NOT_LOCKED` | 1 | **the roof gate.** `claim lock`, `claim reaudit --confirm` and plain `check` refuse while `constitution.yaml` is missing, `status: draft`, locked-by-hand with no record, or **edited after its lock** (the edited file is what every module now reads; nothing from the old version stays in force). `error.details.state` says which. Plain `check` already regenerated the catalog and viewer (`stopped_at: constitution`, a gate not an outage); `check --validate` / `--staged` carry it as the `constitution-not-locked` finding. Recovery is never yours alone: `dossierx constitution show` prints the words and the lock state, you draft or show the diff, the **human** locks — `dossierx constitution lock --dry-run`, then `--reason "<their words>"`. `claim new`, `claim show`, `claim list`, `serve` and the bare `claim reaudit <id>` preview keep working meanwhile. Do not loop on `claim lock` or `claim reaudit --confirm`. |
| `CONSTITUTION_OVER_CAP` | 1 | the roof is over **800 words** (titles and bodies; slugs do not count). `check` and `constitution lock` refuse it; `constitution-near-cap` warned from 720. Trim it — it is the critical brief, not a design document — then lock again. |
| `pre_ledger_unadopted` | 1 | an approval-recording command — `claim lock`, `claim reaudit --confirm` — refused because this project's lock store predates the lock ledger and it still holds locked claims. It is the write-path twin of the `lock-ledger-pre-ledger` finding. Legacy ledger adoption has no automatic `dossierx migrate`. Unlock every locked claim (`dossierx claim unlock <id> --reason "…"`); then preview and write each lock with its matching token (`dossierx claim lock <id> --dry-run`, then `dossierx claim lock <id> --reason "…" --proposal "<snapshot>"`). The crossing is stamped by that first **lock**, not by the unlock. It discards every standing approval, so it is the human's call — show them and wait. |
| `comment_digest_drift` | 1 | the claim's `comments:` block and `build/ledger/comment-digest.json` disagree, so this write is refused rather than silently re-recording the block as the truth. **No command clears it** — the recovery is version control, and which file you restore depends on which side moved: the claim file if its block was hand-edited, the digest store if a commit carried the claim file without it, both from the same commit if you cannot tell. The engine writes the two as a pair and they only agree as a pair. **Never delete the digest store to clear this** — that is the laundering the store exists to catch, and `check` then reports `comment-digest-absent`. Tell the human; do not loop on it. |
| `comment_digest_unavailable` | 1 | the comment digest store could not be opened, so the write was refused **before anything changed**. Nothing was written, so a retry is safe — but it will keep failing identically until `build/ledger/comment-digest.json` is restored from version control (or a stale `build/ledger/comment-digest.json.lock` left by a crash is removed). Tell the human; do not loop on it. |
| `untracked_config` | 1 | `check --staged` was asked to judge a commit whose `project.config.yaml` is not tracked. It reads the claims, the ledger and the digest store from the **index**, and an untracked config can be edited without staging anything — so honouring the worktree copy would let a one-line `claims_dir:` edit point the gate at a clean decoy while the commit carries a tampered locked claim. Run `git add project.config.yaml` (or the path you passed to `--config`) and commit again. Nothing was judged, so this is not a verdict on your claims. |
| `not_locked` | 2 | flagging and linking need a locked claim. |
| `implink_refused` | 1 | `claim link` could not record the link, or `check`'s source scan rejected a `dossierx-claim:` or `dossierx-step:` tag — a file that does not exist, a claim outside `--module`, a path that is absolute or escapes the project, a tag naming an unknown id, a `dossierx-step` whose `#n`/hash is illegal or does not match the YAML step text, or **a tag on a claim you deliberately unlocked**, which is still `draft` in the middle of `unlock → fix → lock` while the scan wants it locked. That last case is the one where the tag is already right: finish `dossierx claim lock <id> --reason "…"` and re-run — **do not remove or edit the tag**, and do not treat the exit 1 as a verdict on your source. `data.scan_errors[]` gives the file, line and `claim_id` of every rejected tag; `dossierx claim show <id>` settles which case you are in without reading a word of prose. Every other case is your invocation or your tag, not a gate: fix it and re-run, and show the human the message. |
| `unlinked_claims` | 1 | `check`'s code-link gate: the project sets `source_dirs`, and a locked `schema`/`behavior`/`api`/`verification` claim has no code link — or, carrying `steps:`, is not tagged on every step. `data.code_links.modules[].unlinked` and `.partial` (with `missing` step indexes) name each one; the viewer shows the same on the claim's card. Recover by tagging the real code (`dossierx-claim:` for the whole claim, `dossierx-step: <id> #<n> <hash>` for each step) or `dossierx claim link`, then re-run plain `check` — `--validate` and `--staged` report `code_links` with `gated: false` and never refuse, so a green there is not a linked green. **Do not tag an unrelated file to clear it.** If the claim genuinely produces no code, it is mis-roled: unlock → set `build_role` to `orientation` or `out-of-scope` → lock. |
| `structured_layout` | 1 | `claim flag` rewrites `body` only; this claim renders from `rows`/`steps`/`raw_html`. Use unlock → fix → lock. |
| `rights_denied` | 1 | the advisory-rights rule, enforced against the `--as` you passed. You tried to act on a human's message. **Do not retry as another role, and do not retry over `dossierx serve`'s HTTP API** — that surface does not enforce this and would let the write through. Reply instead. |
| `missing_flag` | 1 | a required `--reason`/`--as`/`--module` was omitted. `--reason` carries the human's approving words; do not invent them. |
| `layout_legacy` | 1 | this project still keeps DossierX's generated files at the project root, which this release does not read. Every verb refuses until the printed `git mv`/`mv` block is run — paste it verbatim, then `dossierx check --validate`. Nothing needs re-locking: signatures hash bytes, not paths. |
| `store_gitignored` | 1 | `claim lock`, `claim flag` or `claim reaudit --confirm` refused, one of two arms, and the recovery differs: **ignored and untracked** — same recovery as the `store-gitignored` finding above, replace the `.gitignore` pattern (or repoint `build_dir`); do not retry the same command unchanged, it fails identically. **git could not be consulted** (missing from PATH, a bare or corrupt repository, a `.git` file whose gitdir is missing) — no pattern or `build_dir` edit touches this; install git, or fix the repository, and run again. |
| `git_unavailable` | 1 | `claim recover-approved-content` refused because it needs to READ git history and cannot: git is not installed, or the project is not inside a work tree. That is not an empty recovery — nothing was looked for. Install git or run from a work tree; do not treat the refusal as "no approval could be recovered". Distinct from `store_gitignored`, which is a write-path answer from git. |
| `unknown_module` / `unknown_track` / `unsupported_format` / `usage` | 1 | fix your own invocation. A typo'd track id is the one worth naming: without this code it would answer exactly like a real track nobody has joined yet. |
| `skills_drift` | 1 | `skills export --check`: a skill file on disk differs from this binary's bundle. `data.hand_edited[]` names files that also differ from `dossierx-skills.lock` (someone rewrote the skill — a rewritten skill can teach a false sync story, so restore or re-export it and say so); `data.stale[]` names files that match the lock but not the binary (exported by an older release — re-run `dossierx skills export`); `data.missing[]` was never exported; `data.unverified[]` differs in a tree with no lock (an export older than the lock, or assembled by hand — the check does not guess). Nothing was written; `dossierx skills export` settles all but the hand edit. |
| `write_failed` | 1 | a write did not land: a permission, a missing directory, a full disk — or, from `skills export`, "no directory given and no `project.config.yaml` found", which is your invocation and not the filesystem. Give the export an explicit directory (`dossierx skills export .claude/skills`). Show the human anything else; retrying an unwritable path just fails again. |
| `conformance_capacity_exceeded` | 1 | a status, whole catalog (including readiness), or viewer projection cannot fit DossierX's bounded output budget. No generated artifact was replaced. Read `stopped_at`: reduce total declared checks or check-ID/value bytes at `conformance`, projected catalog/readiness/conformance volume at `catalog`, or viewer content/facet/track duplication at `render`, then run the same check again. The matching detail remains in `data.conformance_error`, `data.catalog_error`, or `data.render_error`; do not treat a catalog/render refusal as an observation failure. |
| `conformance_failed` | 1 | `conformance.blocking: true` found at least one owed, mismatch, or uncheckable named check. Read the grouped results in `data.conformance`: each check carries its stable id and exact state; set mismatches carry missing/extra members and scalar mismatches carry expected/observed strings. A plain `check` has already refreshed the inspectable status, catalog, and viewer; `--validate` and `--staged` wrote nothing. Fix or produce the project-owned observation and rerun the same command. Do not unlock or relock claims: this gate does not change approval. |
| `write_conflict` | 1 | another process (often `dossierx serve`) holds the lock. Retry. If the retry stalls the same ~10s and fails identically, nobody is holding it: a process died inside the critical section and left the sentinel file behind, and no timeout clears it — the acquire timeout only makes each failure arrive faster. The message names the file (`build/ledger/claims.lock`, or the `.lock` sitting beside whichever store it names); delete that file and retry. Do not loop on it. |
| `claim_file_changed` | 1 | someone wrote while you were deciding. Re-read the claim and redo the decision — do **not** retry blindly. |
| `banner_claim` / `empty_body` / `unsafe_body` | 1 | the comment you tried to write cannot be stored. Fix the body (`unsafe_body` is now narrow: a first content line led by a TAB. Space-indented first lines store fine as of v0.4.0); `claim_not_serializable` instead means the claim **on disk** is already broken. |

## --dry-run: "blocked" is a successful answer

### Local-approval policy v1

New projects use one set evaluator for single/group lock preview and write.
`--dry-run` returns verdicts, conditions and a `snapshot`; `--proposal` is
required on every lock write, rejecting missing, invalid, stale and wrong-set review. Draft dependencies yield visible
`dependency_unapproved`, never readiness. Read `claim show` or API `readiness`
for causes/paths. Existing stores stay on their recorded policy; a newer
binary does not reinterpret old approvals.

Every mutating verb takes `--dry-run`. It writes nothing and **always exits 0 with `ok: true`**,
even when the real run would refuse — including when you forgot a required flag.

```json
{"ok": true, "command": "claim lock", "data": {"would": "lock claim widget.contract.retry-policy",
  "from": "draft", "to": "locked", "blocked": true, "missing": ["--reason"],
  "preconditions": [{"name": "no_open_comment_threads", "ok": false, "detail": "1 thread [c-b98f8b]"}],
  "side_effects": ["the claim becomes locked: every later change goes through unlock -> fix -> lock"]}}
```

Read `data.blocked`, not the exit status: a non-zero exit means the *preview* broke, not that the
action would be refused. `side_effects` is the part a human cannot infer — always show it.

## Five rules that never bend

1. **Draft is your workshop.** Create, rewrite, restructure and delete draft claims freely — no
   approval, no ceremony. This is the work, and nothing here gates it.
2. **A locked claim never changes without a recorded approval.** Body-only meaning drift is
   `claim flag`, then the human, then `reaudit` — not unlock. Every other edit is unlock → fix →
   lock. `reaudit` refuses a claim not already `review_pending`, its dependency-drift proposer is
   a no-change stub, and it rewrites only `body`. It is the **drift** tool, not the edit tool.
   Never hand-edit locked YAML — the ledger sees it.
3. **Resolve the human's words to an id, and say the id back before acting.** "the retry card in
   contract" is not an id. `dossierx claim list --match "retry"` ranks candidates with a `score`;
   name the winner and its title to the human and wait.
4. **Preview, then ask, then act.** Every lifecycle action (`claim lock`, `unlock`, `flag`,
   `reaudit --confirm`, `claim link`) gets a `--dry-run`
   first, shown to the human, and a real yes. `--reason` carries *their* words.
5. **You reply; you never resolve a human's thread.** Their Resolve click is the approval that
   unblocks locking. Advisory rights are **enforced for the CLI actor** — `--as` is required and
   `--as agent` on a human's thread fails with `rights_denied` — but **not on `dossierx serve`'s
   HTTP API**, which reads the actor from the request body and treats a missing one as `human`, so
   any local caller gets full human rights: the same trust level as write access to the claim YAML.
   Nothing stops you curling the resolve endpoint. It is simply forgery, and it leaves a record
   positively attesting that a human resolved it.

## Which command — the calls agents get wrong

| a locked claim… | run | never |
|---|---|---|
| is `review_pending` from a flag or a dependency's drift | `dossierx claim reaudit <id>` (preview), then `--confirm --reason "…"` | reaudit a claim that is not `review_pending` (`not_review_pending`) |
| means something else now, renders from `body` only, and you can write before/after | `dossierx claim flag <id> --claim-says --now-does --reason` → the human → reaudit | fold this into unlock: the before/after is the human's diff to confirm |
| means something else now but renders from `rows`/`steps`/`raw_html`/`mockup` | `unlock → fix → lock --reason "…"` (`claim flag` refuses: `structured_layout`) | hand-edit the YAML |
| needs any other edit while locked | `unlock → fix → lock` | reaudit it |
| shows an EMPTY `reaudit` preview after a flag | stop: `build/ledger/flag-store.json` did not travel — recover it | confirm the empty diff, or unlock to clear the noise |
| refused `claim lock` | `dossierx claim lock <id> --dry-run` and read the findings | loop `check --validate`: the claim is still `draft` on disk, so the lock-time rules report nothing |
| has code that only moved or was renamed, same meaning | re-tag it, or `dossierx claim link` | flag it |
| raises a question you cannot phrase as before/after | a comment (`dossierx comment add`) | flag it |

| "are we done?" | run | what it is |
|---|---|---|
| a user feature, across modules | `dossierx track status <id>` | read-only: COMPLETE when every claim it owns or cites is locked. It gates nothing and orders nothing. |
| what to implement next | locked claims, module depends_on, and claim rests_on / build_role | follow those edges; there is no sequencer |

## `governed_by` is gone — what v0.7.21 changed under you

**A corpus that passed `dossierx check` before v0.7.21 can refuse to LOAD after it with no edit on
your side.** `governed_by`, its four lints, its graph edge and its drift baseline are gone (NIT-29):
the constitution replaces the roof and is never cited, with no alias and no migrate command. (1) A claim file
still carrying `governed_by:` **fails strict decode** — `check` stops at `stopped_at: load`, and
"restore from git" is no recovery: that file is the one that no longer loads. (2) **Every locked
claim's hash moved** (the field left `ContentHash` and `LockedClaimHash`), so `check` then reports
`lock-content-drift` on every locked claim; recover by `unlock → lock`, never by editing the store.

**Stage A, this release, before the first `check`:** delete every `governed_by:` block; where its
`type` named a claim the dependent relies on, add that id to `rests_on` (`type: none` gets nothing
until NIT-24); leave `doctrine_facet` and the doctrine claims alone; then `check --validate` and
re-lock each listed claim: `claim unlock` → `claim lock --dry-run` → `claim lock --reason --proposal`.

**Stage B, same release (NIT-6 / NIT-25 / NIT-23):** each former doctrine claim goes one way — rested
on → a project claim (`project.<slug>`, `rests_on` retargeted); system law → a constitution entry; else
delete. `doctrine_facet` is gone (a config that sets it fails to load). Then the human locks the roof.

## The pre-ledger crossing and the staged gate — what v0.4.0 changed under you

**1. A project whose lock store predates the lock ledger, AND that still holds a locked claim,
does not `check` clean.** You meet it as `integrity_failed` carrying
**`lock-ledger-pre-ledger`** — project-scoped, said once, deliberately *not* one
`lock-ledger-missing` per claim, whose "set it back to draft and re-lock" recovery would be actively
destructive here. Read the rule, not the count. It is **silent** on a pre-ledger project holding
nothing locked: that project is not broken, and its next `claim lock` stamps the store onto the
ledger schema and records a real approval. The write path refuses in the same state with
`pre_ledger_unadopted` — `claim lock`, `claim reaudit --confirm`.

**The crossing, in this order and no other:** (1)
`dossierx claim unlock <id> --reason "…"` for every locked claim; (2) lock only what the human still
stands behind. The first lock in a project holding nothing locked is what crosses the store. Nothing
is grandfathered: there is no `dossierx migrate` and no automatic adoption, because nothing can
attest to content no ledger ever recorded. **Not your call** — unlocking everything discards every
standing approval, so show the human the finding, say what it will discard, and get a yes, exactly
as for `claim lock`. Commit the lock store and the comment digest store the crossing writes. The
**store file** tells the benign case from its opposite neighbour, no git history needed: pre-ledger
= store **present** on the pre-ledger schema (cross it); `lock-ledger-absent` = store **file gone**
while locked claims remain (tampering; restore from git).

**2. `check --staged` judges the GIT INDEX** — what the commit will contain — with `git show` instead
of the worktree, **writing nothing**; that is what makes a pre-commit hook meaningful. It judges **one
tree**: no git history, no parent comparison, same verdict in every clone.
**DossierX detects; the forge enforces** — every rule is evidence production, not prevention, so
branch protection plus a required CI check is what makes anyone obey it, and is the answer when a
human asks you to set integrity up.

**The boundary: an in-repo ledger cannot attest anything against the person who can write it.** The gate catches
edits to a locked claim's *approved content* that leave a surviving file *disagreeing* — a claim edited, a
record deleted, a status flipped, a thread erased. It cannot catch what nothing disagrees with: a record's
`reason`, `at` and `actor` are prose no rule checks, and a claim and its record written **together** in one
commit leave nothing over to object — `check` and `check --staged` both return `ok: true` over a ledger
crediting a human who approved nothing. **Never propose editing a locked claim and its record in the same
breath; never report `ok: true` as proof nobody did.**

**Moving `claims_dir` needs no ceremony and no flag exempts it.** `git mv claims docs/claims`, edit
`claims_dir:`, stage claims, config and the unchanged stores, commit together — it passes because every
locked claim still resolves to its record. A move that **strands** them fails as
`lock-ledger-abandoned`, once per claim; never "fix" that by deleting more or re-locking.

## Which skill to load

| You are about to… | Load |
|---|---|
| write, edit, inspect, lock, unlock or reaudit a claim; run `dossierx check`; find the claim the human meant | `dossierx-claims` |
| draft, show, lock or re-lock the constitution; author a project claim (`project.<slug>`); pick a `rests_on` target | `dossierx-claims` |
| cite the evidence behind a claim (`sources`, `[n]` markers), put a claim on a feature track, or read `dossierx track list/show/status` | `dossierx-claims` |
| tag finished code with `dossierx-claim:` or `dossierx-step:`, or report that shipped code no longer matches a locked claim | `dossierx-code-links` |
| read `dossierx comment inbox`, reply to a review thread, or decide comment vs. `claim flag` | `dossierx-comments` |
| implement code from locked claims | `dossierx-claims` — follow module depends_on and claim rests_on / build_role |

Load one, not all four.

## Bootstrap — setting DossierX up in a repo

Only when the human asks, and **in this order** — steps 2 and 3 are not interchangeable.

1. Install the binary if absent, then `dossierx version`.
2. Propose `project.config.yaml` (title, facets, modules) and `claims/`, and **ask them to confirm
   the facet list** before writing it. Propose `constitution.yaml` beside it (the critical brief, under
   800 words); once read, **they** lock it — `dossierx constitution lock --reason "<their words>"`.
3. `dossierx skills export .claude/skills` — or whichever directory this harness reads. **Name it
   explicitly** (the harness, not DossierX, decides where skills are read from; with no argument the
   export writes into every `.claude/skills` and `.agents/skills` the repo already has, and a
   `dossierx-skills.lock` beside them — `dossierx skills export --check` refuses `skills_drift` when
   a skill on disk was edited by hand or came from an older release), and run it **after
   step 2, never before**: the export finds the project root through `project.config.yaml`, and
   only a rooted export maintains its section in an `AGENTS.md` that already exists and writes
   `docs/dossierx-agent-guide.md` under the root, where that section links it. Run before the
   config exists it still exits 0 — it installs the bundles and drops the guide beside them, no
   `AGENTS.md` touched — and nothing later in this sequence exports again.
4. Ask before installing the git pre-commit hook. Their answer decides the hook alone, never CI:
   **CI is the authority either way**, and both answers end with the workflow installed — the hook
   is only fast local feedback on top of it, which git skips on merges, rebases, cherry-picks and
   reverts, and which `--no-verify` bypasses. Neither the hook installer nor
   `scripts/ci/dossierx-check.yml` exists in *their* repo — both ship with DossierX, so fetch each
   from the same release path. If yes, fetch
   `https://raw.githubusercontent.com/BarterX-Tech/dossierx/v0.7.20/scripts/install-git-hook.sh`,
   show them what it does, run `sh install-git-hook.sh --yes`, then add the CI workflow as well.
   If no, skip the hook, add the CI workflow alone, and say so.
5. **Only if the project predates the lock ledger AND still holds locked claims**, `check` will
   refuse it: take it through The pre-ledger crossing above — show the human
   what unlocking everything discards, get their yes, then unlock and re-lock. Skip it on
   a project you created at step 2, and say you skipped it.
6. Run `dossierx check --format text` and show them the output exiting 0. Do not assert it works.
7. Tell them to commit `build/ledger/lock-store.json`, and `build/ledger/comment-digest.json` and
   `build/ledger/flag-store.json` once those appear — tracked artifacts, never `.gitignore`d.
8. Tell them to run `dossierx serve`; that is the only DossierX command they ever run.

**The three stores under build/ledger/ travel with the claims.** `build/ledger/lock-store.json` is what CI
compares the claims against — without it the gate is theatre, and dropping it while locked claims
remain is `lock-ledger-absent`. `build/ledger/comment-digest.json` is the same for review history
(`comment-digest-absent`). `build/ledger/flag-store.json` has **no gate rule behind it at
all**: if it does not travel, its claim arrives `review_pending` with nothing to propose and a
confirmed `claim reaudit` clears the human's flag having changed nothing — silently.

## If you remember an older command

| gone | now |
|---|---|
| `lint`, `catalog`, `render` → `dossierx check` · `deps`, `implink status` → `dossierx claim show <id>` · `stale`, `coverage` → `dossierx claim list --review-pending` / `--migrated` · `implink set` → `dossierx claim link` · bare `lock`/`unlock`/`flag`/`reaudit` → `dossierx claim lock` / `unlock` / `flag` / `reaudit` · `comment resolve|reopen|edit|delete` → viewer only, the human does these | |
| `migrate --adopt` | legacy ledger adoption has no `dossierx migrate`. For an unadopted project, unlock every locked claim (`dossierx claim unlock <id> --reason "…"`), then preview and write each lock with its matching token — `dossierx claim lock <id> --dry-run`, then `dossierx claim lock <id> --reason "…" --proposal "<snapshot>"` — so the first lock in a project with nothing locked crosses the store onto the ledger |
