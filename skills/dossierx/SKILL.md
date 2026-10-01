---
name: dossierx
description: >-
  Router and machine contract for DossierX — the CLI that turns a project's atomic YAML "claims"
  and markdown "briefs" into a reviewable HTML viewer, and that an agent OPERATES while a human
  REVIEWS. Load this FIRST and ALWAYS in any repo that has a project.config.yaml plus a claims/
  directory, before running any DossierX command. It carries the nine nouns, the JSON envelope,
  exit codes, the error.code recovery table, the dry-run rule, five rules that never bend, which
  command (flag vs unlock vs reaudit), bootstrap, and which companion skill to load (claims,
  modules, constitution, briefs, comments, code-links, upgrading). Load a companion skill only
  when this one sends you there.
---

DossierX keeps a project's guarantees as **claims** — one atomic, reviewable YAML fact per file
under `claims/` — and everything else worth writing down (design, voice, rationale, research,
features) as **briefs**, markdown documents under `briefs/`. `dossierx check` lints both and builds
a viewer. A claim or brief starts `draft` (freely editable) and is promoted to `locked` (frozen;
changes need a recorded human approval). **You are the operator, the human is the reviewer:** they
read the viewer, comment, click Resolve and tell you what to do; you run every command, they run one.

| | Agent (you) | Human |
|---|---|---|
| Surface | the CLI — all 26 commands | the viewer, via `dossierx serve`: Home (what waits on them), the Constitution, Modules and their claims, Briefs and Features, the claims graph, and comment threads |
| Freely | author, edit, restructure, delete **draft** claims and briefs; reply to any thread; run `dossierx check` as often as you like | read anything; comment on any card or brief; resolve/reopen/edit/delete their own messages |
| Never | change a **locked** claim or brief without their recorded approval; lock/unlock/flag/reaudit unasked; resolve or reopen a thread a human opened; edit or delete a comment | — |

## The nine nouns, twenty-six leaves
```
dossierx check                             # the whole pipeline; --validate = read-only, --staged = judge the git index, write nothing — neither proves code links
dossierx claim  show list new lock unlock flag reaudit link recover-approved-content
dossierx comment inbox list add reply
dossierx constitution show lock            # the project-root constitution.yaml — the roof, not a module: show prints the words, lock records the hash
dossierx brief  list show lock unlock reaudit # the briefs beside the claims; lock/unlock/reaudit take --reason, like claims
dossierx manifest show list                # module harness: --isolation = constitution + project claims index + manifest + claim summaries; --integration = neighbors; list = catalog
dossierx serve                             # the human's one command
dossierx skills export [dir]
dossierx version
```

**Work one module at a time:** `manifest show <module> --isolation` → `--integration` for neighbors.
Never walk or read the whole claims tree; `dossierx-modules` has the reading order and the caps.
**Claims hold guarantees; briefs hold the rest** — `dossierx-claims` decides which a statement is,
`dossierx-briefs` writes the brief. Find briefs with `dossierx brief list`; they never appear in
`manifest show`. `claim recover-approved-content` recovers an old approval's wording from git and
belongs to an upgrade (`dossierx-upgrading`). A command an older release had and this one does not
answers `usage` with the replacement in its `hint`.

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
it produced. At `ledger`, `constitution` and `links` the catalog and viewer WERE regenerated and only
the exit status is withheld — a gate, not an outage. A noun with no leaf (`dossierx claim` alone)
is a failed invocation: one envelope, `usage`, exit 1.

Exit status is one of three: `0` success · `1` failure (a lint error, a refused gate, a write
error) · `2` not found, or not in the state the command requires.

**Proof or stop.** Evidence is what the tool emits — an exit code, a finding, a conformance result,
`data.code_links` — or what the human does — Resolve, `--reason`. Your saying "it matches" or "it is
synced" in chat is neither and is never a certificate; when the evidence is missing, stop and say what is missing.

## error.code → what you actually do about it

| code | exit | recovery |
|---|---|---|
| `config_not_found` | 2 | not a DossierX project (yet). Do not create one unasked — see Bootstrap. |
| `invalid_config` | 1 | `project.config.yaml` does not load or validate. Fix the field the message names. On a config you did not touch, right after an upgrade: `dossierx-upgrading`. |
| `invalid_claim` | 1 | the claims do not load (`stopped_at: load`): `claims_dir` does not exist, bad YAML, a second YAML document, or a key the schema does not have. On a corpus you did not touch, right after an upgrade, the key was retired: `dossierx-upgrading` folds it. Restoring the file from git brings the same key back. |
| `claim_not_found` | 2 | you guessed an id. `dossierx claim list --match "<what the human said>"`, and confirm the id back to them. |
| `brief_not_found` | 2 | a `brief` verb got neither a brief's path nor its `<folder>.<slug>` id. `dossierx brief list`, then pass the path it prints. |
| `lint_failed` | 1 | findings are in **`data.lint_findings`** (`lint` names the rule, `claim_id` the claim or brief path). Fix them, then re-check **with the command that refused you**: `check --validate` after `check`, `claim lock <id> --dry-run` after `claim lock`. `check --validate` cannot answer a lock refusal — the claim is still `draft` on disk, so lock-time rules report nothing. A `constitution-not-locked` finding is the human's to clear (`CONSTITUTION_NOT_LOCKED`). A cap finding is loud by design: split or trim (`dossierx-modules`, `dossierx-briefs`); raising a cap is the human's call. |
| `integrity_failed` | 1 | **branch on `rule` in `data.ledger_findings`.** `store-gitignored`: an engine store is `.gitignore`d and untracked, so approvals never reach the repository — replace the pattern with the block under "The stores" below, or point `build_dir` elsewhere; neither restoring nor re-locking touches it. `lock-ledger-pre-ledger`: the lock store predates the lock ledger and still holds locked claims — the crossing is the human's call: unlock every locked claim, then re-lock only what they still stand behind (the first lock crosses the store). **Every locked claim** reporting `lock-content-drift` right after an upgrade, with no edit, is a lock-hash change: `dossierx-upgrading`. Everything else — the other `lock-ledger-*` rules, `lock-content-drift`, `comment-ledger-drift`, the `comment-digest-*` family, and a brief's `brief-content-drift` / `brief-unrecorded` / `brief-orphan` / `brief-abandoned` — is a locked artifact moved outside the approval path: **never re-lock to make it go away**; restore the file from git, or unlock → fix → lock on the human's yes. `lock-ledger-deleted` and `comment-digest-unrecorded` also refuse `claim lock`, and no command clears them: restore the named store file. |
| `comment_open` | 1 | `brief lock` / `brief reaudit --confirm` on a brief with an open thread. Reply (`comment reply <brief-path> <thread-id>`); the human resolves it in the viewer. |
| `store_too_new` | 1 | a lock or comment digest store written by a newer dossierx. Upgrade the binary; never edit or restore the store so an older one can read it. |
| `unresolved_comments` | 1 | the claim has an open thread. Reply on it; the **human** clicks Resolve in the viewer. That click is the approval this gate waits for. |
| `not_review_pending` | 2 | `reaudit` on a claim or brief that is not drifting. The edit path is unlock → fix → lock. |
| `review_pending` | 2 | the claim IS pending, and that blocks you; `claim show <id>` names the trigger. One case `claim show` cannot explain: `claim lock --semantic-conflict <claim-id>=<dependency-id>=<reason>` (repeatable; dependency id may be empty) refuses that claim in that one call and is recorded nowhere; the dry run shows it as `blocked: true`. Pass the flag only when you see a locked dependency that contradicts the claim you are about to lock. Put `error.details.semantic_conflicts` to the human, and re-run without the flag only after they resolve it. |
| `already_locked` | 1 | already locked, and a second lock would sign whatever the file now says with no diff. To change it: unlock → fix → lock. If a gate reported drift on it, restore the file from git instead. |
| `CONSTITUTION_NOT_LOCKED` | 1 | **the roof gate**: `claim lock`, `claim reaudit --confirm` and every `check`, `--validate` and `--staged` included (`stopped_at: constitution`), refuse until `constitution.yaml` is locked and unchanged; `error.details.state` says why. Show the human; **they** approve the lock. Do not loop on `claim lock`. `dossierx-constitution`. |
| `CONSTITUTION_OVER_CAP` | 1 | the constitution is over 800 words, a fixed cap. Trim it with the human — `dossierx-constitution`. |
| `pre_ledger_unadopted` | 1 | `claim lock` / `claim reaudit --confirm` on a project whose lock store predates the lock ledger and still holds locked claims. The crossing discards every standing approval, so it is the human's call: see `lock-ledger-pre-ledger` above. |
| `comment_digest_drift` | 1 | a claim's `comments:` block and `build/ledger/comment-digest.json` disagree. **No command clears it**: restore the claim file if its block was hand-edited, the digest store if a commit carried the claim without it, both from one commit if you cannot tell. **Never delete the digest store.** Tell the human; do not loop. |
| `comment_digest_unavailable` | 1 | the digest store could not be opened; nothing was written. Restore `build/ledger/comment-digest.json` from git, or remove a stale `comment-digest.json.lock` a crash left behind. Tell the human; do not loop. |
| `untracked_config` | 1 | `check --staged` on a repository whose `project.config.yaml` is not tracked — an untracked config could point the gate at a decoy. `git add project.config.yaml` and commit again. Nothing was judged. |
| `not_locked` | 2 | flagging, linking and unlocking need a locked claim or brief. |
| `implink_refused` | 1 | `claim link`, or `check`'s source scan, rejected a `dossierx-claim:` / `dossierx-step:` tag or link: a missing file, a claim outside `--module`, a path that escapes the project, an unknown id, a bad step `#n` or hash — or **a tag on a claim you deliberately unlocked**, still `draft` mid unlock → fix → lock. In that case the tag is right: finish `dossierx claim lock <id>` with the human's `--reason` and re-run — **do not remove or edit the tag**. `data.scan_errors[]` gives file, line and `claim_id`; `claim show <id>` tells you which case you are in. Otherwise fix your tag or invocation. |
| `unlinked_claims` | 1 | `check`'s code-link gate: `source_dirs` is set and a locked module claim has no code link, or a stepped claim is not tagged on every step (`data.code_links.modules[]`). Tag the real code or `claim link`, then plain `check`. **Never tag an unrelated file.** A claim with no code behind it is the human's call — `dossierx-code-links`. |
| `structured_layout` | 1 | `claim flag` rewrites `body` only, and this claim carries `rows`, `steps`, `raw_html` or `layout: mockup`. Use unlock → fix → lock. |
| `rights_denied` | 1 | the advisory-rights rule refused an actor acting on another author's message (from `serve`'s API). **Never retry as another role.** Reply instead. |
| `missing_flag` | 1 | a required `--reason`/`--proposal`/`--as`/`--module` was omitted. `--reason` carries the human's words — never invent them. `--proposal` is the `snapshot` from `claim lock <ids> --dry-run` for the same ids. |
| `invalid_actor` | 1 | `--as` must be `human` or `agent`. You are `agent`. |
| `bad_request` | 1 | a malformed argument: an `--since` that is not RFC 3339, a `--semantic-conflict` missing its parts. Fix the invocation. |
| `layout_legacy` | 1 | an old release's files still sit at the project root, and every verb refuses. Paste the printed `git mv`/`mv` block (`error.details.moves[]`), then `check --validate`. Nothing re-locks. |
| `store_gitignored` | 1 | `claim lock`, `claim flag` or `claim reaudit --confirm` refused. Either the store is **ignored and untracked** (same recovery as `store-gitignored` above; a retry fails identically) or **git could not be consulted** (not installed, a broken repository) — install or repair git and run again. |
| `git_unavailable` | 1 | `claim recover-approved-content` cannot read git history. Nothing was looked for — not an empty recovery. |
| `unknown_module` / `unsupported_format` / `usage` | 1 | fix your own invocation. |
| `skills_drift` | 1 | `skills export --check`: the exported skills differ from this binary's bundle. `data.hand_edited[]` was rewritten by hand (restore or re-export it, and say so); `data.stale[]` came from an older release and `data.missing[]` was never exported (re-run `dossierx skills export`); `data.unverified[]` sits in a tree with no lock; `data.forbidden[]` is a line teaching a whole-corpus read (remove it); `data.retired[]` is a bundle this release no longer ships (the export removes it when it can prove it wrote it; otherwise ask the human, then delete it). |
| `write_failed` | 1 | a write did not land (permissions, a missing directory, a full disk) — or `skills export` found no `project.config.yaml` and had no directory: pass one (`dossierx skills export .claude/skills`). |
| `view_too_large` | 1 | `manifest show --isolation`: this module's part of the view is over its 6144-byte budget. Shorten summaries or the manifest, or split the module — `dossierx-modules`. |
| `conformance_capacity_exceeded` | 1 | a generated output would exceed its bounded budget; nothing was replaced. At `conformance` reduce declared checks, at `catalog` the projected volume, at `render` the viewer content; then re-run. |
| `conformance_failed` | 1 | `conformance.blocking: true` and a declared embodiment check is owed, mismatched or uncheckable (`data.conformance`). Fix the code or produce the observation and re-run. Never unlock or re-lock for it — `dossierx-code-links`. |
| `write_conflict` | 1 | another process (often `dossierx serve`) holds the write lock. Retry. If every retry stalls ~10s and fails the same way, a process died holding it and left the sentinel behind: delete the `.lock` file the message names (e.g. `build/ledger/claims.lock`) and retry. Do not loop. |
| `claim_file_changed` | 1 | someone wrote while you were deciding — or your `--proposal` no longer matches the claims (`details.reason: "invalid"`). Re-read, re-run the `--dry-run`, show the human again — do **not** retry blindly. |
| `wrong_state` | 2 | not in the state the command needs (for `brief reaudit --confirm`: a `rests_on` claim is gone). `claim show` / `brief show`, then pick the command for that state. |
| `no_artifact` | 1 | a code-link operation found no link file for the module. Link the first claim with a tag or `claim link`, then plain `check`. |
| `thread_not_found` / `reply_not_found` | 2 | the id is not on that claim or thread. Re-run `dossierx comment inbox`; do not guess. |
| `thread_resolved` | 1 | the human resolved the thread while you worked — the approval you were waiting for. Drop the reply, move on. |
| `thread_open` | 1 | a reopen of a thread that is already open; reopening is the human's, in the viewer. |
| `read_only` | 1 | a comment write to `serve` whose custom `shell.html` cannot host comments; the CLI never returns it. Use the CLI. |
| `internal` | 1 | an unclassified failure. From `check --staged` with `stopped_at: load`, nothing was judged: git itself failed, or — mid-upgrade — the staged config still carries a retired key (`dossierx-upgrading`). Show the human the message; do not retry in a loop. |
| `banner_claim` / `empty_body` / `unsafe_body` | 1 | the comment cannot be stored: fix its body (`unsafe_body`: a first content line led by a TAB). |
| `claim_not_serializable` | 1 | the claim **on disk** is already broken; fix the file, then retry. |

## --dry-run: "blocked" is a successful answer

Every mutating `claim`, `brief`, `comment` and `constitution` verb takes `--dry-run`. It writes
nothing and **always exits 0 with `ok: true`**, even when the real run would refuse — including when
you forgot a required flag. `check --validate` is the write-free `check`; `skills export --check`
compares and writes nothing, but exits 1 with `skills_drift` when they differ.

```json
{"ok": true, "command": "claim lock", "data": {"would": "lock widget.contract.retry-policy",
  "from": "draft", "to": "locked", "blocked": true, "missing": ["--reason"],
  "snapshot": "v2:widget.contract.retry-policy:5e8f…",
  "evaluation": {"verdicts": [{"claim_id": "widget.contract.retry-policy", "local_admissible": false,
    "refusals": ["unresolved_comments"], "open_threads": ["c-a53344"], "dependency_conditions": null}],
    "unrelated_findings": []},
  "preconditions": [{"name": "lint_clean", "ok": true}, {"name": "constitution_locked", "ok": true}, "…"],
  "side_effects": ["write requested claim approvals and lock ledger records"]}}
```

Read `data.blocked`, not the exit status. **Why** it is blocked is per claim in
`data.evaluation.verdicts[]`: `refusals` (error codes such as `unresolved_comments` or
`lint_failed`), `lint_findings`, `open_threads` and `dependency_conditions`; findings on other claims
are in `unrelated_findings`. The `claim_is_draft` and `no_open_comment_threads` preconditions only
echo the overall verdict, so do not diagnose from them. Always show the human `side_effects`.

**`claim lock` needs the preview's `snapshot`:** pass it back as `--proposal` with the human's
`--reason`, for the same set of ids. A stale or different set is refused (`claim_file_changed`), so a
lock always signs exactly what the human was shown. A draft dependency does not block a lock; it
shows as `dependency_unapproved` in `claim show`'s `readiness`.

## Five rules that never bend

1. **Draft is your workshop.** Create, rewrite, restructure and delete draft claims and briefs
   freely — no approval, no ceremony.
2. **A locked claim never changes without a recorded approval.** Body-only meaning drift is
   `claim flag`, then the human, then `reaudit` — not unlock — when the summary still holds: a
   confirmed flag replaces the whole `body` with `--now-does` and leaves `summary` as it was. Every other edit is unlock → fix →
   lock. `reaudit` refuses a claim not already `review_pending`, its dependency-drift proposer is
   a no-change stub, and it rewrites only `body`. It is the **drift** tool, not the edit tool.
   Never hand-edit locked YAML or a locked brief — the ledger sees it.
3. **Resolve the human's words to an id, and say the id back before acting.** `dossierx claim list
   --match "retry"` ranks candidates with a `score`; name the winner and its title, and wait.
4. **Preview, then ask, then act.** Every lifecycle action (`claim lock`, `unlock`, `flag`,
   `reaudit --confirm`, every `brief` and `constitution` lock) gets a `--dry-run` first, shown to the
   human, and a real yes. (`claim link` and source tags record where code lives and approve
   nothing: they are yours.) `--reason` carries *their* words.
5. **You reply; you never resolve a human's thread.** Their Resolve click is the approval that
   unblocks locking. The CLI cannot breach this — its only comment writes are `add` and `reply` —
   but `dossierx serve`'s HTTP API can: it treats a request with no actor as `human`. Calling it to
   resolve, edit or delete a thread is forgery, and it leaves a record saying a human did it.

## Which command — the calls agents get wrong

| a locked claim… | run | never |
|---|---|---|
| is `review_pending` from a flag or a dependency's drift | `dossierx claim reaudit <id>` (preview), then `--confirm --reason "…"` | reaudit a claim that is not `review_pending` |
| means something else now, renders from `body` only, the summary still holds, and you can write the whole corrected body | `dossierx claim flag <id> --claim-says --now-does --reason` → the human → reaudit | fold this into unlock: the before/after is the human's diff to confirm |
| means something else now but carries `rows`/`steps`/`raw_html` or `layout: mockup` | unlock → fix → lock `--reason "…"` (`claim flag` refuses: `structured_layout`) | hand-edit the YAML |
| needs any other edit while locked | unlock → fix → lock | reaudit it |
| shows a `reaudit` preview with `trigger: "none"`, `no_change: true` after a flag | stop: `build/ledger/flag-store.json` did not travel — recover it | confirm the empty diff |
| was refused by `claim lock` | `dossierx claim lock <id> --dry-run` and read the findings | loop `check --validate` |
| has code that only moved or was renamed, same meaning | re-tag it, or `dossierx claim link` | flag it |
| raises a question you cannot phrase as before/after | a comment (`dossierx comment add`) | flag it |

What to implement next is a locked module: `manifest show --isolation`, its `depends_on`, its
claims' `rests_on` (`dossierx-code-links`).

## `check --staged`, and what the ledger cannot see

**`check --staged` judges the GIT INDEX** — what the commit will contain — writing nothing; that is
what the pre-commit hook runs. It judges one tree, with no history. **DossierX detects; the forge
enforces**: branch protection plus a required CI check is what makes anyone obey it, and is the
answer when a human asks you to set integrity up.

**An in-repo ledger cannot attest anything against the person who can write it.** The gate catches
an edit to a locked claim's approved content that leaves a surviving file disagreeing. It cannot
catch a claim and its ledger record rewritten **together** in one commit: `check` returns `ok: true`
over a ledger crediting a human who approved nothing. **Never propose editing a locked claim and its
record in the same breath; never report `ok: true` as proof nobody did.**

**Moving `claims_dir`** needs no ceremony: `git mv claims docs/claims`, edit `claims_dir:`, commit
claims, config and the unchanged stores together. A move that strands them fails as
`lock-ledger-abandoned`; never "fix" that by deleting more or re-locking.

## The stores — committed, never `.gitignore`d

`build/ledger/lock-store.json` (every claim, brief and constitution approval),
`build/ledger/comment-digest.json` (the review history's fingerprint),
`build/ledger/flag-store.json` (pending flags) and `build/code-links/<module>.json` travel with the
claims in the same commit. Without the lock store, CI has nothing to compare against
(`lock-ledger-absent`). The flag store has **no gate rule behind it**: if it does not travel, its
claim arrives `review_pending` with nothing to propose, and a confirmed reaudit clears the human's
flag having changed nothing. `build/catalog/`, `build/viewer/` and `build/conformance/` are
regenerated and safe to ignore. A repository `.gitignore` with a bare `build/` pattern hides the
stores (`store-gitignored`); replace it with:

```
build/*
!build/.gitignore
!build/ledger
!build/ledger/*
!build/code-links
!build/code-links/*
```

or set `build_dir` to a directory the pattern does not match.

## Which skill to load

| You are about to… | Load |
|---|---|
| decide whether something is worth a claim; write, edit, inspect, lock, unlock or reaudit a claim; cite evidence (`sources`); run `dossierx check`; find the claim the human meant | `dossierx-claims` |
| orient in a module, draft `manifest.yaml`, read neighbors, or meet any cap | `dossierx-modules` |
| draft, show, lock or re-lock the constitution; author a project claim (`project.<slug>`) | `dossierx-constitution` |
| write, edit, lock, unlock or reaudit a brief, write a feature brief, choose a brief's `rests_on`, cite a brief from a claim, or meet a `brief-*` finding | `dossierx-briefs` |
| read `dossierx comment inbox`, reply to a thread, or decide comment vs. `claim flag` | `dossierx-comments` |
| implement code from a locked module, tag it, declare an embodiment, or report that shipped code no longer matches a locked claim | `dossierx-code-links` |
| upgrade the binary, or meet a refusal on a corpus you did not touch right after one | `dossierx-upgrading` |

Load one at a time, not all eight.

## Bootstrap — setting DossierX up in a repo

Only when the human asks, and **in this order** — steps 2 and 3 are not interchangeable.

1. Install the binary if absent, then `dossierx version`.
2. Propose `project.config.yaml` (`schema_version: 1`, `title`, `modules`, `claims_dir: claims`, and
   the engine-fixed `facets: [contract, internals]`; add `source_dirs` to gate code links), create
   the `claims/` directory, and one `claims/<module>/manifest.yaml` per module (a `summary`,
   `provides: []`, `depends_on: []` — `dossierx-modules`). Propose `constitution.yaml` beside the
   config (`dossierx-constitution`); once read, **they** approve its lock —
   `dossierx constitution lock --reason "<their words>"`. Step 6 cannot pass without both.
3. `dossierx skills export .claude/skills` — or whichever directory this harness reads. **Name it
   explicitly** (the harness, not DossierX, decides where skills are read from; with no argument the
   export writes `.claude/skills` when the repo has a `.claude/` directory and `.agents/skills` when
   it has `.agents/`, and a
   `dossierx-skills.lock` beside them — `dossierx skills export --check` refuses `skills_drift` when
   a skill on disk was edited by hand or came from an older release), and run it **after
   step 2, never before**: the export finds the project root through `project.config.yaml`, and
   only a rooted export maintains its section in an `AGENTS.md` that already exists, linking each
   companion to the bundle it wrote. Run before the config exists it still exits 0 — it installs
   the bundles, no `AGENTS.md` touched — and nothing later in this sequence exports again.
4. Ask before installing the git pre-commit hook. Their answer decides the hook alone, never CI:
   **CI is the authority either way**, and both answers end with the workflow installed — the hook
   is only fast local feedback on top of it, which git skips on merges, rebases, cherry-picks and
   reverts, and which `--no-verify` bypasses. Neither the hook installer nor
   `scripts/ci/dossierx-check.yml` exists in *their* repo — both ship with DossierX, so fetch each
   from the same release path (the workflow is
   `https://raw.githubusercontent.com/BarterX-Tech/dossierx/v0.7.22/scripts/ci/dossierx-check.yml`,
   copied into `.github/workflows/`). If yes, fetch
   `https://raw.githubusercontent.com/BarterX-Tech/dossierx/v0.7.22/scripts/install-git-hook.sh`,
   show them what it does, run `sh install-git-hook.sh --yes`, then add the CI workflow as well.
   If no, skip the hook, add the CI workflow alone, and say so.
5. If the repository `.gitignore` has a bare `build/` pattern, replace it with the block under
   "The stores" before the first lock.
6. Run `dossierx check --format text` and show them the output exiting 0. Do not assert it works.
7. Tell them to commit the stores under `build/ledger/` and `build/code-links/` as they appear —
   tracked artifacts, never `.gitignore`d.
8. Tell them to run `dossierx serve`; that is the only DossierX command they ever run.
