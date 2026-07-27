---
name: dossierx
description: >-
  Router and machine contract for DossierX — the CLI that turns a project's
  atomic YAML "claims" into a reviewable HTML viewer, and that an agent
  OPERATES while a human REVIEWS. Load this FIRST and ALWAYS in any repo that
  has a project.config.yaml plus a claims/ directory, before running any
  DossierX command. It is short on purpose: the six nouns, the JSON envelope,
  the exit codes, the error.code to recovery table, the dry-run rule, the five
  rules that never bend, and which companion skill to load for the work in
  front of you. Load a companion skill only when this one sends you there.
---

# DossierX — router and machine contract

DossierX turns a project's `claims/` directory — one atomic, reviewable YAML fact per file —
into a linted, dependency-checked HTML viewer. A claim starts `draft` (freely editable) and
is promoted to `locked` (frozen; changes require a recorded human approval). Everything the
tool knows lives in `project.config.yaml` plus `claims/`; DossierX itself hardcodes nothing
about any project.

**You are the operator. The human is the reviewer.** They read the viewer, comment on cards,
click Resolve, and tell you what to do. You run every command. They run exactly one.

| | Agent (you) | Human |
|---|---|---|
| Surface | the CLI — all 19 commands | the viewer, via `dossierx serve` |
| Freely | author, edit, restructure, delete **draft** claims; reply to any thread; run `dossierx check` as often as you like | read anything; comment on any card; resolve/reopen/edit/delete their own messages |
| Never | change a **locked** claim without their recorded approval; lock/unlock/flag/reaudit unasked; resolve or reopen a thread a human opened; edit or delete a comment | — |

## The six nouns, nineteen leaves

```
dossierx check                             # the whole pipeline; --validate = read-only, --staged = judge the git index
dossierx claim  show list new lock unlock flag reaudit link
dossierx comment inbox list add reply
dossierx build-order propose status lock
dossierx serve                             # the human's one command
dossierx skills export [dir]
dossierx version
```

There is no `lint`, `catalog`, `render`, `deps`, `stale`, `coverage`, `implink`, or
`comment resolve|reopen|edit|delete`. If you remember one of those, you are remembering a
version before v0.3.0 — see the table at the bottom of this skill.

## The envelope — every command, every run

`--format json` is the **default**. One envelope per invocation, always on stdout, on failure
as well as success. `--format text` is the human prose you paste into chat for a review.

```json
{"ok": true,  "command": "claim show", "data": { }, "warnings": ["[warning] orphan: ..."]}
{"ok": false, "command": "claim lock", "error": {"code": "unresolved_comments",
  "message": "...", "hint": "...", "details": { }}, "stopped_at": "lint"}
```

Branch on `error.code` and on fields inside `data`. **Never** regex `message` or `hint`:
`code` is a promise, prose is not. `stopped_at` names the pipeline step a partial run reached
(`config`, `load`, `reconcile`, `lint`, `catalog`, `render`, `scan`, `ledger`), and `data` still
carries whatever a failed run managed to produce. `ledger` is the one worth reading closely: it
means the catalog and the viewer WERE regenerated and only the commit is refused, so the
documentation the human is reading is current — a gate, not an outage.

A **noun with no leaf** (`dossierx claim` on its own) is an ordinary failed invocation — one
envelope, `usage`, exit 1 — not help text at exit 0. Ask for the version with `dossierx version`,
the verb that answers in an envelope.

Exit status is one of three, unchanged since v0.1: `0` success · `1` failure (a lint error, a
refused gate, a write error) · `2` not found, or not in the state the command requires.

## error.code → what you actually do about it

| code | exit | recovery |
|---|---|---|
| `config_not_found` | 2 | not a DossierX project (yet). Do not create one unasked — see Bootstrap below. |
| `claim_not_found` | 2 | you guessed an id. Run `dossierx claim list --match "<what the human said>"` and confirm the id back to them. |
| `lint_failed` | 1 | read `data.lint_findings`, fix the claims, re-run `dossierx check --validate`. |
| `integrity_failed` | 1 | a locked claim moved outside the approval path. Read `data.ledger_findings`. **Do not re-lock to make it go away** — restore the file from git, or unlock → fix → lock. |
| `unresolved_comments` | 1 | the claim has an open thread. Reply on it; the **human** clicks Resolve in the viewer. That click is the approval this gate waits for. |
| `dependency_not_locked` | 1 | a doctrine dependency is still draft. Lock it first (with approval), then retry. |
| `not_review_pending` | 2 | you reached for `claim reaudit` on a claim that is not drifting. The general edit path is unlock → fix → lock. |
| `review_pending` | 2 | the claim IS pending, and that is what blocks you. `dossierx claim show <id>` names the trigger. |
| `already_locked` | 1 | the claim (or build order) is **already** locked, and `lock` refuses rather than re-signing it — a second lock would stamp a fresh approval over content nobody approved and clear `review_pending` with no diff. To change it: `unlock` → fix → `lock`. If a gate reported drift on it, restore the file from git instead. |
| `comment_digest_drift` | 1 | the claim's `comments:` block was changed outside the engine, so this write is refused rather than silently re-recording the tampered block as the truth. Restore the claim file from version control, then retry. |
| `comment_digest_unavailable` | 1 | the comment digest store could not be opened, so the write was refused **before anything changed**. Nothing was written, so a retry is safe — but it will keep failing identically until `.dossierx-comment-digest.json` is restored from version control (or a stale `.dossierx-comment-digest.json.lock` left by a crash is removed). Tell the human; do not loop on it. |
| `build_order_hand_edited` | 1 | `.build-order.<module>.json` is not what a fresh `propose` computes — a phase sequence, a claim's placement, or the `excluded` set was edited by hand. The **claims are fine**; the artifact is not, so none of `build_order_refused`'s recoveries apply. Re-run `build-order propose --module <m>` to discard the edit, then `lock` what the engine derived. |
| `not_locked` | 2 | flagging and linking need a locked claim. |
| `implink_refused` | 1 | `claim link` could not record the link, or `check`'s source scan rejected a `dossierx-claim:` tag — a file that does not exist, a claim outside `--module`, a path that is absolute or escapes the project, a tag naming an unknown id, or **a tag on a claim you deliberately unlocked**, which is still `draft` in the middle of `unlock → fix → lock` while the scan wants it locked. That last case is the one where the tag is already right: finish `dossierx claim lock <id> --reason "…"` and re-run — **do not remove or edit the tag**, and do not treat the exit 1 as a verdict on your source. `data.scan_errors[]` gives the file, line and `claim_id` of every rejected tag; `dossierx claim show <id>` settles which case you are in without reading a word of prose. Every other case is your invocation or your tag, not a gate: fix it and re-run, and show the human the message. |
| `structured_layout` | 1 | `claim flag` rewrites `body` only; this claim renders from `rows`/`steps`/`raw_html`. Use unlock → fix → lock. |
| `rights_denied` | 1 | the advisory-rights rule. You tried to act on a human's message. **Do not retry as another role.** Reply instead. |
| `missing_flag` | 1 | a required `--reason`/`--as`/`--module` was omitted. `--reason` carries the human's approving words; do not invent them. |
| `unknown_module` / `unsupported_format` / `usage` | 1 | fix your own invocation. |
| `write_conflict` | 1 | another process (often `dossierx serve`) holds the lock. Retry. If the retry stalls the same ~10s and fails identically, nobody is holding it: a process died inside the critical section and left the sentinel file behind, and no timeout clears it — the acquire timeout only makes each failure arrive faster. The message names the file (`.dossierx-claims.lock`, or the `.lock` sitting beside whichever store it names); delete that file and retry. Do not loop on it. |
| `claim_file_changed` | 1 | someone wrote while you were deciding. Re-read the claim and redo the decision — do **not** retry blindly. |
| `banner_claim` / `empty_body` / `unsafe_body` | 1 | the comment you tried to write cannot be stored. Fix the body; `claim_not_serializable` instead means the claim **on disk** is already broken. |

## --dry-run: "blocked" is a successful answer

Every mutating verb takes `--dry-run`. It writes nothing and **always exits 0 with `ok: true`**,
even when the real run would refuse — including when you forgot a required flag.

```json
{"ok": true, "command": "claim lock", "data": {
  "would": "lock claim widget.contract.retry-policy", "from": "draft", "to": "locked",
  "preconditions": [{"name": "lint_clean", "ok": true, "detail": "0 error-level lint finding(s)"},
                    {"name": "no_open_comment_threads", "ok": false, "detail": "1 unresolved thread(s) [c-b98f8b]"}],
  "side_effects": ["rewrites claims/retry-policy.yaml", "the claim becomes locked: every later change must go through unlock -> fix -> lock"],
  "missing": ["--reason", "no_open_comment_threads"], "blocked": true}}
```

Read `data.blocked`, not the exit status. A non-zero exit from a dry run means the *preview*
broke, not that the action would be refused. `side_effects` is the part a human cannot infer —
always show it when you ask for a yes.

## Five rules that never bend

1. **Draft is your workshop.** Create, rewrite, restructure and delete draft claims freely. No
   approval, no ceremony. This is the work, and nothing here gates it.
2. **A locked claim never changes outside the approval path, and the path is unlock → fix →
   lock.** Not `reaudit`: `reaudit` refuses any claim that is not already `review_pending`, its
   dependency-drift proposer is a no-change stub, and it can only rewrite `body`. It is the
   **drift** tool, not the edit tool. Never hand-edit a locked claim's YAML — the lock ledger
   sees it and `dossierx check` fails with `integrity_failed`.
3. **Resolve the human's words to an id, and say the id back before acting.** "the retry card in
   contract" is not an id. `dossierx claim list --match "retry"` returns candidates with a
   `score`; name the winner and its title to the human and wait.
4. **Preview, then ask, then act.** Every lifecycle action (`claim lock`, `unlock`, `flag`,
   `reaudit --confirm`, `claim link`, `build-order lock`) gets a `--dry-run` first, shown to the
   human, and a real yes before the real run. `--reason` carries *their* words, not yours.
5. **You reply; you never resolve a human's thread.** Advisory rights are enforced in code
   (`rights_denied`), and the human's Resolve click is the approval that unblocks locking. Taking
   it for them destroys the only approval signal in the design.

## Which skill to load

| You are about to… | Load |
|---|---|
| write, edit, inspect, lock, unlock or reaudit a claim; run `dossierx check`; find the claim the human meant | `dossierx-claims` |
| read `dossierx comment inbox`, reply to a review thread, or decide comment vs. `claim flag` | `dossierx-comments` |
| implement code from a module whose claims are all locked | `dossierx-build-order` |
| tag finished code with its claim, or report that shipped code no longer matches a locked claim | `dossierx-code-links` |

Load one, not all four. Each states the contract it needs at the top.

## Bootstrap — setting DossierX up in a repo

Only when the human asks. 1) install the binary if absent, then `dossierx version`. 2)
`dossierx skills export` — it writes the guide in whatever form this repo uses. 3) propose
`project.config.yaml` (project name, facets, modules) and `claims/`, and **ask them to confirm
the facet list** before writing it. 4) ask before installing the git pre-commit hook
(`scripts/install-git-hook.sh`), then run `dossierx check` and show them the result. Commit the
lock ledger file — CI depends on it. 5) tell them to run `dossierx serve`; that is the only
DossierX command they ever run.

## If you remember an older command

| gone | now |
|---|---|
| `lint`, `catalog`, `render` | `dossierx check` (`--validate` for a read-only lint) |
| `deps`, `implink status` | `dossierx claim show <id>` |
| `stale`, `coverage` | `dossierx claim list --review-pending` / `--migrated` |
| `implink set` | `dossierx claim link` |
| `lock`, `unlock`, `flag`, `reaudit` | `dossierx claim lock` / `unlock` / `flag` / `reaudit` |
| `comment resolve`, `reopen`, `edit`, `delete` | viewer only — the human does these |
