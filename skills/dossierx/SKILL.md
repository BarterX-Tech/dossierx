---
name: dossierx
description: >-
  Router and machine contract for DossierX — the CLI that turns a project's
  atomic YAML "claims" into a reviewable HTML viewer, and that an agent
  OPERATES while a human REVIEWS. Load this FIRST and ALWAYS in any repo that
  has a project.config.yaml plus a claims/ directory, before running any
  DossierX command. It is short on purpose: the seven nouns, the JSON envelope,
  the exit codes, the error.code to recovery table, the dry-run rule, the five
  rules that never bend, the one-time migrate --adopt every pre-v0.3.0 project
  must run, and which companion skill to load for the work in front of you.
  Load a companion skill only when this one sends you there.
---

# DossierX — router and machine contract

DossierX turns a project's `claims/` directory — one atomic, reviewable YAML fact per file — into
a linted, dependency-checked HTML viewer. A claim starts `draft` (freely editable) and is promoted
to `locked` (frozen; changes require a recorded human approval). Everything the tool knows lives in
`project.config.yaml` plus `claims/`. **You are the operator, the human is the reviewer:** they read the
viewer, comment, click Resolve and tell you what to do; you run every command, they run one.

| | Agent (you) | Human |
|---|---|---|
| Surface | the CLI — all 20 commands | the viewer, via `dossierx serve` |
| Freely | author, edit, restructure, delete **draft** claims; reply to any thread; run `dossierx check` as often as you like | read anything; comment on any card; resolve/reopen/edit/delete their own messages |
| Never | change a **locked** claim without their recorded approval; lock/unlock/flag/reaudit unasked; resolve or reopen a thread a human opened; edit or delete a comment | — |

## The seven nouns, twenty leaves

```
dossierx check                             # the whole pipeline; --validate = read-only, --staged = judge the git index, write nothing
dossierx claim  show list new lock unlock flag reaudit link
dossierx comment inbox list add reply
dossierx build-order propose status lock
dossierx migrate --adopt                   # ONE TIME, per pre-v0.3.0 project; see Adoption below
dossierx serve                             # the human's one command
dossierx skills export [dir]
dossierx version
```

There is no `lint`, `catalog`, `render`, `deps`, `stale`, `coverage`, `implink`, or
`comment resolve|reopen|edit|delete`; the table at the bottom maps each to its replacement.

## The envelope — every command, every run

`--format json` is the **default**: one envelope per invocation, on stdout, on failure as well as
success. `--format text` is the prose you paste into chat for a human.

```json
{"ok": true,  "command": "claim show", "data": { }, "warnings": ["[warning] orphan: ..."]}
{"ok": false, "command": "claim lock", "error": {"code": "unresolved_comments",
  "message": "...", "hint": "...", "details": { }}, "stopped_at": "lint"}
```

Branch on `error.code` and on fields inside `data`. **Never** regex `message` or `hint`: `code` is
a promise, prose is not. `stopped_at` names the pipeline step a partial run reached (`config`,
`load`, `reconcile`, `lint`, `catalog`, `render`, `scan`, `ledger`), and `data` still carries what
it produced. `ledger` is the one to read closely: the catalog and viewer WERE regenerated and only
the commit is refused — a gate, not an outage. A **noun with no leaf** (`dossierx claim` alone) is
an ordinary failed invocation — one envelope, `usage`, exit 1 — not help text at exit 0.

Exit status is one of three, unchanged since v0.1: `0` success · `1` failure (a lint error, a
refused gate, a write error) · `2` not found, or not in the state the command requires.

## error.code → what you actually do about it

| code | exit | recovery |
|---|---|---|
| `config_not_found` | 2 | not a DossierX project (yet). Do not create one unasked — see Bootstrap below. |
| `claim_not_found` | 2 | you guessed an id. Run `dossierx claim list --match "<what the human said>"` and confirm the id back to them. |
| `lint_failed` | 1 | findings are in **`data.lint_findings`** — on `check` and on `claim lock` alike (`claim lock` keeps a second copy under `error.details.lint_findings`). Fix the claims, then re-check **with the command that refused you**: `dossierx check --validate` after a `check` failure, `dossierx claim lock <id> --dry-run` after a `claim lock` failure. Re-running `check --validate` after a lock refusal is a **loop, not a recovery**: it does not re-attempt the lock, and it reports *zero* findings for every rule that keys off a claim's own status (`build-role-required-for-locked`, `rest-on-locked`, `roll-up`) because the claim is still `draft` on disk. The dry run lints the about-to-be-locked form, which is the only form that answers. |
| `integrity_failed` | 1 | **read `data.ledger_findings` and branch on `rule`** — one code, several causes, and one of them does NOT mean tampering: `lock-ledger-adoption-required` is a pre-v0.3.0 project nobody has adopted (see Adoption). Everything else — `lock-ledger-missing`, `lock-ledger-deleted`, `lock-content-drift`, `lock-ledger-released`, `lock-ledger-orphan`, `lock-ledger-abandoned`, `lock-ledger-downgraded`, `comment-ledger-drift`, the `comment-digest-*` and `build-order-*` families — is a locked artifact moved outside the approval path: **do not re-lock to make it go away**, restore the file from git or unlock → fix → lock. Two of them are now refusals on the WRITE path too, so you will meet them as a failed `claim lock` and not only as a finding: `lock-ledger-deleted` and `comment-digest-unrecorded`. Re-locking was the step that erased each of them, so there is no command that clears either — the recovery is restoring the named store file, and `unlock → fix → lock` is **wrong** here because it signs the edit. |
| `unresolved_comments` | 1 | the claim has an open thread. Reply on it; the **human** clicks Resolve in the viewer. That click is the approval this gate waits for. |
| `dependency_not_locked` | 1 | a doctrine dependency is still draft. Lock it first (with approval), then retry. |
| `not_review_pending` | 2 | you reached for `claim reaudit` on a claim that is not drifting. The general edit path is unlock → fix → lock. |
| `review_pending` | 2 | the claim IS pending, and that is what blocks you. `dossierx claim show <id>` names the trigger. |
| `already_locked` | 1 | the claim (or build order) is **already** locked, and `lock` refuses rather than re-signing it — a second lock would stamp a fresh approval over content nobody approved and clear `review_pending` with no diff. To change it: `unlock` → fix → `lock`. If a gate reported drift on it, restore the file from git instead. |
| `already_migrated` | 1 | `dossierx migrate --adopt` refused; `data.mode` says which case, and they differ. **`already_covered`** — this project has been through a ledger-aware build; adoption is one-time, and a second run would record whatever is on disk *now* as approved. A locked claim missing its record is `lock-ledger-missing`/`lock-ledger-deleted`, and that recovery is version control, never another migration. **`nothing_to_adopt`** — no store and nothing locked, so there is no pre-ledger state; the ledger is created by the first real `claim lock`. Either way: nothing to migrate, do not loop. |
| `comment_digest_drift` | 1 | the claim's `comments:` block and `.dossierx-comment-digest.json` disagree, so this write is refused rather than silently re-recording the block as the truth. **No command clears it** — the recovery is version control, and which file you restore depends on which side moved: the claim file if its block was hand-edited, the digest store if a commit carried the claim file without it, both from the same commit if you cannot tell. The engine writes the two as a pair and they only agree as a pair. **Never delete the digest store to clear this** — that is the laundering the store exists to catch, and `check` then reports `comment-digest-absent`. Tell the human; do not loop on it. |
| `comment_digest_unavailable` | 1 | the comment digest store could not be opened, so the write was refused **before anything changed**. Nothing was written, so a retry is safe — but it will keep failing identically until `.dossierx-comment-digest.json` is restored from version control (or a stale `.dossierx-comment-digest.json.lock` left by a crash is removed). Tell the human; do not loop on it. |
| `build_order_hand_edited` | 1 | `.build-order.<module>.json` is not what a fresh `propose` computes — a phase sequence, a claim's placement, or the `excluded` set was edited by hand. The **claims are fine**; the artifact is not, so none of `build_order_refused`'s recoveries apply. Re-run `build-order propose --module <m>` to discard the edit, then `lock` what the engine derived. |
| `untracked_config` | 1 | `check --staged` was asked to judge a commit whose `project.config.yaml` is not tracked. It reads the claims, the ledger and the digest store from the **index**, and an untracked config can be edited without staging anything — so honouring the worktree copy would let a one-line `claims_dir:` edit point the gate at a clean decoy while the commit carries a tampered locked claim. Run `git add project.config.yaml` (or the path you passed to `--config`) and commit again. Nothing was judged, so this is not a verdict on your claims. |
| `not_locked` | 2 | flagging and linking need a locked claim. |
| `implink_refused` | 1 | `claim link` could not record the link, or `check`'s source scan rejected a `dossierx-claim:` tag — a file that does not exist, a claim outside `--module`, a path that is absolute or escapes the project, a tag naming an unknown id, or **a tag on a claim you deliberately unlocked**, which is still `draft` in the middle of `unlock → fix → lock` while the scan wants it locked. That last case is the one where the tag is already right: finish `dossierx claim lock <id> --reason "…"` and re-run — **do not remove or edit the tag**, and do not treat the exit 1 as a verdict on your source. `data.scan_errors[]` gives the file, line and `claim_id` of every rejected tag; `dossierx claim show <id>` settles which case you are in without reading a word of prose. Every other case is your invocation or your tag, not a gate: fix it and re-run, and show the human the message. |
| `structured_layout` | 1 | `claim flag` rewrites `body` only; this claim renders from `rows`/`steps`/`raw_html`. Use unlock → fix → lock. |
| `rights_denied` | 1 | the advisory-rights rule, enforced against the `--as` you passed. You tried to act on a human's message. **Do not retry as another role, and do not retry over `dossierx serve`'s HTTP API** — that surface does not enforce this and would let the write through. Reply instead. |
| `missing_flag` | 1 | a required `--reason`/`--as`/`--module`/`--adopt` was omitted. `--reason` carries the human's approving words; do not invent them. |
| `unknown_module` / `unsupported_format` / `usage` | 1 | fix your own invocation. |
| `write_failed` | 1 | a write did not land: a permission, a missing directory, a full disk — or, from `skills export`, "no directory given and no `project.config.yaml` found", which is your invocation and not the filesystem. Give the export an explicit directory (`dossierx skills export .claude/skills`). Show the human anything else; retrying an unwritable path just fails again. |
| `write_conflict` | 1 | another process (often `dossierx serve`) holds the lock. Retry. If the retry stalls the same ~10s and fails identically, nobody is holding it: a process died inside the critical section and left the sentinel file behind, and no timeout clears it — the acquire timeout only makes each failure arrive faster. The message names the file (`.dossierx-claims.lock`, or the `.lock` sitting beside whichever store it names); delete that file and retry. Do not loop on it. |
| `claim_file_changed` | 1 | someone wrote while you were deciding. Re-read the claim and redo the decision — do **not** retry blindly. |
| `banner_claim` / `empty_body` / `unsafe_body` | 1 | the comment you tried to write cannot be stored. Fix the body; `claim_not_serializable` instead means the claim **on disk** is already broken. |

## --dry-run: "blocked" is a successful answer

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
2. **A locked claim never changes outside the approval path, and the path is unlock → fix →
   lock.** Not `reaudit`: it refuses any claim not already `review_pending`, its dependency-drift
   proposer is a no-change stub, and it rewrites only `body`. It is the
   **drift** tool, not the edit tool. Never hand-edit locked YAML — the ledger sees it.
3. **Resolve the human's words to an id, and say the id back before acting.** "the retry card in
   contract" is not an id. `dossierx claim list --match "retry"` ranks candidates with a `score`;
   name the winner and its title to the human and wait.
4. **Preview, then ask, then act.** Every lifecycle action (`claim lock`, `unlock`, `flag`,
   `reaudit --confirm`, `claim link`, `build-order lock`, `migrate --adopt`) gets a `--dry-run`
   first, shown to the human, and a real yes. `--reason` carries *their* words.
5. **You reply; you never resolve a human's thread.** Their Resolve click is the approval that
   unblocks locking. Advisory rights are **enforced for the CLI actor** — `--as` is required and
   `--as agent` on a human's thread fails with `rights_denied` — but **not on `dossierx serve`'s
   HTTP API**, which reads the actor from the request body and treats a missing one as `human`, so
   any local caller gets full human rights: the same trust level as write access to the claim YAML.
   Nothing stops you curling the resolve endpoint. It is simply forgery, and it leaves a record
   positively attesting that a human resolved it.

## Adoption and the staged gate — what v0.3.0 changed under you

**1. A project that locked claims before v0.3.0 does not `check` clean until someone adopts it.**
You meet it as `integrity_failed` carrying **`lock-ledger-adoption-required`** — project-scoped,
said once, deliberately *not* one `lock-ledger-missing` per claim, whose "set it back to draft and
re-lock" recovery would be actively destructive here. Read the rule, not the count. The **store
file** tells it from its opposite neighbour, no git history needed: adoption-required = store
**present** on the pre-ledger schema (benign; run the migration); `lock-ledger-absent` = store
**file gone** (tampering; restore from git).

```
dossierx migrate --adopt --dry-run     # names every artifact it would grandfather, writes nothing
dossierx migrate --adopt
```

**Adoption fails closed in every run** — `check`, `--validate`, `--staged` alike. Nothing
grandfathers itself: adoption is the one operation that manufactures approval out of nothing, so a
gate doing it on sight would reward deleting the ledger — and no evidence inside a directory tells
an honest v0.2.x store from a downgraded one (`locked_at` shipped in v0.2.0).

| | |
|---|---|
| **Not your call** | it hashes every locked claim and build order **as they sit on disk now** and records that as the baseline — an approval nobody read. Show the human the finding, say what it will bless, and wait for a yes, exactly as for `claim lock`. |
| **No `--reason`, deliberately** | other record-writing verbs carry the human's words because a human approved something. Nobody approved this. Each record carries a fixed reason saying so, and `grandfathered: true`, permanently. |
| **Upgrade step, never a repair** | never run it to quiet a gate on a project that already has a ledger — it would record tampered bytes as approved, and a second run refuses with `already_migrated`. It changes no claim; commit the rewritten stores with the claims they cover. |

**2. `check --staged` judges the GIT INDEX** — what the commit will contain — with `git show` instead
of the worktree, **writing nothing**; that is what makes a pre-commit hook meaningful. It judges **one
tree**: no git history, no parent comparison, same verdict in every clone.
**DossierX detects; the forge enforces** — every rule is evidence production, not prevention, so
branch protection plus a required CI check is what makes anyone obey it, and is the answer when a
human asks you to set integrity up.

**The three things it does not detect**, so you never report them as safe (FORMAT.md has all three). Each
erases *every side* of a disagreement at once, so `check` returns `ok: true`; each half alone is refused.
(1) Repoint `claims_dir:` **and** delete the lock store — alone, `lock-ledger-abandoned` / `lock-ledger-absent`.
(2) The cheapest — delete one claim's `ledger` record, `locked_at` stamp **and** `hashes` baselines, flip it
to `draft`, rewrite the body; alone, a surviving key gives `lock-ledger-deleted`, still `locked` gives
`lock-ledger-missing`, a standing record `lock-ledger-orphan`. (3) Delete a **draft** claim's `comments:`
block **and** its digest-store key — alone, `comment-ledger-drift` / `comment-digest-unrecorded`; it erases
the thread blocking `claim lock`, so the claim locks over a review nobody had. **Never propose any of these.**

**Moving `claims_dir` needs no ceremony and no flag exempts it.** `git mv claims docs/claims`, edit
`claims_dir:`, stage claims, config and the unchanged stores, commit together — it passes because every
locked claim still resolves to its record. A move that **strands** them fails as
`lock-ledger-abandoned`, once per claim; never "fix" that by deleting more or re-locking.

## Which skill to load

| You are about to… | Load |
|---|---|
| write, edit, inspect, lock, unlock or reaudit a claim; run `dossierx check`; find the claim the human meant | `dossierx-claims` |
| read `dossierx comment inbox`, reply to a review thread, or decide comment vs. `claim flag` | `dossierx-comments` |
| implement code from a module whose claims are all locked | `dossierx-build-order` |
| tag finished code with its claim, or report that shipped code no longer matches a locked claim | `dossierx-code-links` |

Load one, not all four.

## Bootstrap — setting DossierX up in a repo

Only when the human asks, and **in this order** — steps 2 and 3 are not interchangeable.

1. Install the binary if absent, then `dossierx version`.
2. `dossierx skills export .claude/skills` — or whichever directory this harness reads. **Name it
   explicitly:** the argument is optional only *inside* an existing project, and with no directory
   and no `project.config.yaml` the command fails with `write_failed`. That is where you are at
   step 2, because the config does not exist until step 3.
3. Propose `project.config.yaml` (title, facets, modules) and `claims/`, and **ask them to confirm
   the facet list** before writing it.
4. Ask before installing the git pre-commit hook. If yes, fetch
   `https://raw.githubusercontent.com/BarterX-Tech/dossierx/v0.3.0/scripts/install-git-hook.sh`,
   show them what it does, then run `sh install-git-hook.sh --yes`. Neither it nor
   `scripts/ci/dossierx-check.yml` exists in *their* repo — both ship with DossierX, so fetch from
   the same release path. If no, add the CI workflow instead and say so: **CI is the authority.**
5. **Only if the project already existed under v0.2.x with locked claims**, it has no ledger and
   `check` will refuse it: take it through Adoption above — preview, get their yes, run it once,
   report what it adopted. Skip it on a project you created at step 3, and say you skipped it.
6. Run `dossierx check --format text` and show them the output exiting 0. Do not assert it works.
7. Tell them to commit `.dossierx-lock-store.json`, and `.dossierx-comment-digest.json` and
   `.dossierx-flag-store.json` once those appear — tracked artifacts, never `.gitignore`d.
8. Tell them to run `dossierx serve`; that is the only DossierX command they ever run.

**The three project-root stores travel with the claims.** `.dossierx-lock-store.json` is what CI
compares the claims against — without it the gate is theatre, and dropping it while locked claims
remain is `lock-ledger-absent`. `.dossierx-comment-digest.json` is the same for review history
(`comment-digest-absent`). `.dossierx-flag-store.json` has **no gate rule behind it at
all**: if it does not travel, its claim arrives `review_pending` with nothing to propose and a
confirmed `claim reaudit` clears the human's flag having changed nothing — silently.

## If you remember an older command

| gone | now |
|---|---|
| `lint`, `catalog`, `render` | `dossierx check` (`--validate` for a read-only lint) |
| `deps`, `implink status` | `dossierx claim show <id>` |
| `stale`, `coverage` | `dossierx claim list --review-pending` / `--migrated` |
| `implink set` | `dossierx claim link` |
| `lock`, `unlock`, `flag`, `reaudit` | `dossierx claim lock` / `unlock` / `flag` / `reaudit` |
| `comment resolve`, `reopen`, `edit`, `delete` | viewer only — the human does these |
