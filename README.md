# DossierX

DossierX turns a directory of YAML "claims" — atomic, reviewable facts about a system — into a linted, cross-checked HTML site a human can actually read and argue with. Each claim can be **locked** once a human has approved it, after which it never changes silently: a drifted dependency, a code change that contradicts it, or an open comment thread flags it for review instead. It is built to be operated by an **agent** and reviewed by a **human**, and since v0.3.0 each of them has their own surface.

[![CI](https://img.shields.io/github/actions/workflow/status/BarterX-Tech/dossierx/ci.yml?branch=main&label=CI)](https://github.com/BarterX-Tech/dossierx/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/BarterX-Tech/dossierx)](https://github.com/BarterX-Tech/dossierx/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/BarterX-Tech/dossierx)](https://github.com/BarterX-Tech/dossierx/releases)

## Two roles

|  | **Agent** — the operator | **Human** — the reviewer |
|---|---|---|
| **Surface** | the CLI: 19 commands under 7 nouns, JSON by default | the viewer: `dossierx serve`, plus chat with the agent |
| **Does** | writes and restructures draft claims, links code, replies on threads, runs `check`, executes lifecycle actions you approved | reads claims, comments on any card, resolves and reopens threads, says "lock it" |
| **Cannot** | change a **locked** claim without an approval on the record; resolve or reopen your threads; edit or delete comments — the last three refused outright on the CLI, and [rules rather than walls on the viewer's localhost API](#the-humans-one-command) | (nothing is *prevented* — you are the approver; you simply shouldn't need to type a DossierX command other than `serve`) |

The gate is narrower than "the agent may not touch claims", which would defeat the point. Draft claims are the agent's workshop and stay unfrictioned. The invariant is: **nothing already locked changes without your approval on the record** — see [the lock ledger](#integrity-the-lock-ledger).

## Start here — paste this to your agent

Paste this into Claude Code, Codex, or any other coding agent working in the repository you want documented. It is idempotent: running it again on a project that is already set up changes nothing and reports what it found.

```text
Set up DossierX in this repository.

1. If the `dossierx` binary is missing, install it with
   `go install github.com/BarterX-Tech/dossierx/cmd/dossierx@v0.6.0`,
   then run `dossierx version` and show me the output.
2. Run `dossierx skills export .claude/skills` — or point it at whichever
   skills/instructions directory this harness actually reads. Load what it
   wrote and follow it: those guides, not this message, are the contract.
3. If `project.config.yaml` and the claims directory do not exist yet,
   propose a title, the facets, and the modules, and WAIT for me to confirm
   before writing anything.
4. ASK ME before installing the git pre-commit hook. My answer decides the
   hook alone, never CI — CI is the authority either way. If I say yes, fetch
   https://raw.githubusercontent.com/BarterX-Tech/dossierx/v0.6.0/scripts/install-git-hook.sh
   to a file, show me what it does, run `sh install-git-hook.sh --yes`, then
   add the CI workflow as well. If I say no, skip the hook and
   add the CI workflow alone, and tell me so. Either answer ends with the
   workflow installed; the hook is only fast local feedback on top of it.
5. ONLY if `dossierx check` reports `lock-ledger-pre-ledger`, or a lock,
   reaudit or build-order lock refuses with `pre_ledger_unadopted`: this
   project locked claims before it had a lock ledger. There is NO migration
   command. Crossing means re-proposing every locked build order, unlocking
   every locked claim, then re-locking only what I still stand behind. Show
   me the plan and WAIT for my yes before unlocking anything. On a project
   you created at step 3 this never fires — say you skipped it.
6. Run `dossierx check --format text` and show me the output. Do not tell me
   it works; show me it exiting 0.
7. Tell me to commit `.dossierx-lock-store.json` — and
   `.dossierx-comment-digest.json` and `.dossierx-flag-store.json` once they
   appear — together with the claim files. All three are tracked artifacts:
   the ledger is what CI compares the claims against, the gate is vacuous
   without it, and a flag that does not travel with its claim reaudits to an
   empty proposal on the next machine. Never add them to .gitignore.
8. Tell me to run `dossierx serve`. That is the only DossierX command I run.

Draft claims are yours: write, restructure and delete them freely, that is
the work. A LOCKED claim never changes without asking me first, and the path
is unlock -> fix -> lock:

    dossierx claim unlock <id> --reason "<my words>"
    ...make the edit...
    dossierx claim lock   <id> --reason "<my words>"

Never hand-edit a locked claim's YAML, and never lock, unlock, flag or
reaudit anything without my explicit yes. `reaudit` is the drift tool, not
the general edit tool.
```

Installing the binary needs a Go 1.26+ toolchain; prebuilt binaries for common platforms are attached to each [GitHub release](https://github.com/BarterX-Tech/dossierx/releases). A DossierX project is a `project.config.yaml` plus a directory of claim YAML files — [the config schema is below](#config-schema-projectconfigyaml), and the agent will write both for you.

## The human's one command

```sh
dossierx serve
```

Open the URL it prints. That is a local viewer of every claim, facet, build order and code link, plus a live comment API: click 💬 on any card — including cards nobody has commented on yet — write what you doubt, and the page re-renders as claim files change on disk. It binds a random high port unless you pass `--port`, is localhost-only, and never writes `viewer/index.html` or `.catalog.json` on a page load.

**The review loop**, end to end:

1. You comment on a card in the viewer. The comment is written into that claim's YAML.
2. You tell the agent "I left comments."
3. The agent runs `dossierx comment inbox` — every open thread in the project, one call.
4. The agent fixes the claim and replies on the thread. Replies are deliberately **ungated**: an agent may reply to a human-opened thread, because that is the entire workflow.
5. You click **Resolve**. That click *is* the approval — and it is load-bearing, because a claim cannot be locked while it carries an unresolved thread.
6. You say "good, lock it." The agent resolves which card you meant, previews with `--dry-run`, waits for your yes, and runs `dossierx claim lock <id> --reason "<your words>"`.

Closing a thread is yours alone **on the CLI**, and in v0.3.0 that is structural rather than polite: `dossierx comment` is `inbox · list · add · reply` and nothing else. Resolve, reopen, edit and delete were removed from the CLI in v0.3.0 and live only in the viewer and in `dossierx serve`'s HTTP API.

**How far that enforcement actually goes.** Advisory rights — an actor may act only on its own messages — are enforced in the engine, and on the CLI the actor is whatever you passed as `--as`. On the CLI that is now structural rather than enforced: `dossierx comment` is `inbox · list · add · reply`, and none of those can close a thread at all — the verbs that could were removed in v0.3.0. An agent following its skills also never asserts `--as human` for something it decided. **The viewer's write API is a different trust boundary.** It takes the actor from the request body and treats a request that omits `as` as `human`, so any local caller that can reach `dossierx serve` gets full human rights: it can resolve, reopen, edit or delete your thread, and the record it leaves positively attests `human`. That is a deliberate choice, not an oversight — the server binds `127.0.0.1` and refuses cross-origin requests, but anything that can curl it can already open the claim's YAML in an editor, so a token on the API would move the lock rather than add one. Read the rule as: **enforced for the CLI actor, and the same trust level as filesystem access for the viewer API.** It is the operating rule of the review loop either way, and an agent that goes around it has forged the only approval signal in the design.

A static `file://` export of the viewer is read-only by design — comments need `dossierx serve`.

## The CLI surface

Nineteen leaf commands under seven nouns. This is a *machine* surface: a human is not expected to run any of it. Use `dossierx <noun> --help` for flags, and `--format text` when you want prose.

```text
check                    lint, catalog, render and the lock-ledger gate in one shot
                         --validate  read-only: lint + ledger gate, writes nothing
                         --staged    judge the git index — what the commit will actually
                                     contain — instead of the worktree, writes nothing

claim        show · list · new · lock · unlock · flag · reaudit · link
comment      inbox · list · add · reply
build-order  propose · status · lock

serve                    the human's viewer + comment API
skills export [dir]      write the embedded agent skills into a project
version                  version, commit, build date (also --version)
```

Every subcommand takes the global `--config` (a path to `project.config.yaml`; when omitted, DossierX searches upward from the current directory the way `git` finds `.git`) and `--format json|text`.

Upgrading from v0.2.x? Twelve commands were removed and four moved in v0.3.0, and v0.4.0 removed `migrate` outright — a project whose lock store predates the ledger now crosses onto it by holding nothing locked; see [Upgrading a pre-ledger project](#upgrading-a-pre-ledger-project) below and [the CHANGELOG's full migration table](CHANGELOG.md).

### The machine contract

`--format json` is the default, and every run emits exactly one envelope:

```json
{"ok": false, "command": "claim lock", "stopped_at": "lint",
 "error": {"code": "lint_failed", "message": "...", "hint": "run: dossierx check"}}
```

`error.code` is a stable snake_case token — `lint_failed`, `claim_not_found`, `rights_denied`, `integrity_failed`, `unresolved_comments`, `pre_ledger_unadopted`, and so on. Branch on it. `message` and `hint` are prose and will be reworded; `code` is the promise. Successful runs carry their payload in `data`, non-blocking findings in `warnings`, and `check` reports how far it got in `stopped_at`.

Mutating commands take `--dry-run`, which reports what *would* change and writes nothing. A dry run fails only when it cannot compute the preview: a refusal — including a missing required flag — is a *successful* blocked report (exit 0, `ok: true`, `data.blocked: true`).

`--reason` is required on `claim lock`, `claim unlock`, `claim reaudit --confirm`, `claim flag`, and `build-order lock`. Under the two-role split the human never types these, so `--reason` is where their approving words enter the record.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Failure — a lint error, an integrity finding, a validation failure, a malformed claim, a failed write. |
| `2` | Not found / not in the right state — e.g. no `project.config.yaml` found, a claim id that doesn't exist, or a claim that isn't `review_pending` when a command requires it to be. |

There is no fourth status and the three have not been renumbered. The fine-grained signal is `error.code`, not the exit status.

## Integrity: the lock ledger

Claims are YAML in git, so nothing can *prevent* an edit. The goal is that no out-of-band edit of a **locked** claim is *silent*.

**DossierX detects; the forge enforces.** Keep that division in view for everything below. The ledger's job is to turn a silent edit into a **named, recoverable finding** — a stable rule string, the claim it is about, and the command that puts things back. What makes anyone *obey* that finding is branch protection with a required CI check, and that is exactly the point: the ledger is what makes a red check mean "`widget.contract.overview` changed without an approval, restore it from version control" instead of "something is off somewhere". A gate nobody can name the failure of is a gate people learn to re-run.

Every legitimate approval — `claim lock`, a confirmed `claim reaudit`, `build-order lock` — writes a record into the **lock ledger**: the hash of exactly what was approved, when, by which account, and the human's own `--reason` words. Unlocking marks the record released rather than deleting it, so the evidence that a claim was ever locked survives. Comment history gets the same treatment in its own digest store, which is why `serve` never needs write access to the lock store.

Three files hold the review state, at the project root, next to `project.config.yaml`:

| File | Holds |
|---|---|
| `.dossierx-lock-store.json` | the lock ledger — per locked claim and locked build order: `{hash, at, actor, reason}` |
| `.dossierx-comment-digest.json` | the review history's fingerprint |
| `.dossierx-flag-store.json` | the pending `claim flag` triggers: each flagged claim's `{claim_says, now_does, reason, flagged_at}`, parked until a confirmed `claim reaudit` consumes it |

**All three are tracked artifacts. Commit them; never `.gitignore` them** — the lock store and the digest the moment anything is locked (the first lock creates `.dossierx-comment-digest.json` in the same act, before anyone has commented — stage them together or `check --staged` reports `comment-digest-absent`), the flag store the moment anything is flagged. A claim and its approval have to travel in the same commit for CI to be able to check either one: CI compares the claims against the ledger, so without the ledger in the repository the gate has nothing to compare against and is theatre. DossierX says so out loud rather than passing quietly: locked claims with no ledger is a hard error (`lock-ledger-absent`), and a ledger that exists but will not parse is `lock-ledger-unreadable`, never a silent skip. Neither one ever grandfathers its way to a pass, and since v0.4.0 there is no adoption path to reach for at all — restore the store from version control. [A store that genuinely predates the ledger is a different state with a different recovery](#upgrading-a-pre-ledger-project).

The flag store is not part of the gate — nothing compares it to anything — but it is not optional either. `claim flag` writes the before/after there and sets `review_pending` on the claim; `claim reaudit` reads it back to produce the diff it asks you to confirm. If the claim travels to another machine and the flag store does not, that claim arrives `review_pending` with nothing to propose, and confirming the empty proposal clears the flag having changed nothing.

That makes it the one store with **no integrity coverage in either direction**: deleting it, `.gitignore`-ing it, or emptying its map is silent — `check` still exits 0, and your recorded "the claim says X, the code does Y" is gone with nothing in the report to say so. It is a bounded hole (a flag is a request for review, not an approval, so erasing one cannot make a locked claim change or an unapproved claim read as approved) but it is a real one, and the mitigation is procedural until a rule covers it: commit the store with the claim it describes, and read an *empty* `reaudit` proposal on a `review_pending` claim as a missing flag entry rather than as "nothing to change". [FORMAT.md](FORMAT.md#the-project-root-stores-are-tracked-artifacts) states the same thing next to the findings that do exist.

These are the exception, not the rule. `.catalog.json` and `viewer/` are *generated* — regenerated in full by every `dossierx check` — and are safe to `.gitignore`. `.build-order.<module>.json` starts out in that generated category and leaves it the moment you lock one: a **locked** build order is an approved artifact the gate compares against its record, so commit it like the stores above.

The gate names each disagreement:

| Finding | What it caught |
|---|---|
| `lock-ledger-pre-ledger` | project-scoped, and still the **one benign entry in this table**: this project's lock store predates the lock ledger, so nothing locked here has an approval record and there is nothing to judge against yet. Said once, with the crossing that clears it, rather than as one `lock-ledger-missing` per claim — whose recovery ("set it back to draft and re-lock") would be actively destructive advice here. **New in v0.4.0, and the part not to miss: it is CONDITIONAL** — silent unless the project actually holds a locked claim or a locked build order, because a pre-ledger project with nothing locked is correct state and crosses onto the ledger on its next lock, and a finding on correct state is how gates get switched off. Its write-path twin is `error.code: pre_ledger_unadopted`, which `claim lock`, `claim reaudit --confirm` and `build-order lock` refuse with in the same state. The recovery is [the crossing](#upgrading-a-pre-ledger-project), not a migration — v0.4.0 removed `dossierx migrate` entirely. Told apart from `lock-ledger-absent` by the store file itself, with no history needed: pre-ledger means the store is **there** and still on the pre-ledger schema, absent means the file is **gone** |
| `lock-ledger-missing` | a claim is `locked` with no approval record — e.g. `status: draft` flipped to `locked` by hand, walking past the lint, hub-gating and unresolved-comment gates |
| `lock-ledger-deleted` | `lock-ledger-missing`'s sharper twin: a claim **this engine locked**, whose record is gone. Every other rule keys on a record *existing*, so deleting one took the claim out of the switch entirely — delete its `ledger` entry, flip `status: locked` back to `draft`, and it is an ordinary draft again, freely editable and re-lockable afterwards with an agent-supplied `--reason` that produces a record indistinguishable from a human's. `check --validate` reported `ok: true` with zero findings. The evidence the deletion does not reach sits one key away in the same file: `locked_at`, which every lock stamps and which nothing removes (`unlock` keeps the record and stamps `released_at`), plus the claim's dependency baselines under `hashes`. A record that is *absent* rather than *released* was deleted by hand. **`claim lock` refuses this state outright** (`integrity_failed`) — otherwise the last step of the bypass is the tool's own command: re-locking writes a fresh record over the rewritten content and the finding disappears for good |
| `lock-ledger-downgraded` | the lock store **says it predates the ledger** while the project proves it does not — its own `version` field set back to `1` and the `ledger` key deleted, one edit inside the audited file. This used to be the highest-value edit in the design, because adoption ran automatically and a store that claimed to predate the ledger was re-adopted on sight: the claims *as they are now* became the approved baseline. Adoption no longer exists at all ([see the upgrade section](#upgrading-a-pre-ledger-project)), so the edit no longer buys approval — but a store lying about its own version is still a tampered store, and it is still reported. Restore it from version control; do **not** re-lock, and do **not** try to force the pre-ledger crossing by unlocking everything — both record the current bytes as the baseline, which is exactly what the downgrade was trying to achieve |
| `lock-ledger-released` | a claim is `locked` on a record an `unlock` already **released** — lock, unlock, then hand-edit `status:` back. A released record is a withdrawn approval, not a standing one, and the content hash cannot see it because the hash excludes `status` |
| `lock-content-drift` | a locked claim's content no longer matches what was approved — including fields the dependency-drift hash does not cover, such as `raw_html_reviewed`, `build_role`, `section` and `order`. (`raw_html` joined the dependency-drift hash in v0.4.1 so an edited attachment marks dependents stale; the ledger signs it either way) |
| `lock-ledger-orphan` | a `draft` claim still holding an *unreleased* record — `locked` flipped back to `draft` to dodge review |
| `lock-ledger-abandoned` | a locked claim's **file was deleted** while its approval record still stands. There is no `claim delete` verb, so `rm` was the one change to a locked claim that no rule saw: every other finding starts from the claims that exist. Unlock first, then delete |
| `comment-ledger-drift` | a review thread edited or deleted outside the engine |
| `comment-digest-absent` | `.dossierx-comment-digest.json` is **gone** from a ledger-covered project, so the rule above is checking nothing at all. Deleting the store was how an edited-away thread stopped being reported: a claim the store has never seen is *unknown*, never *drifted*. Restore the file from version control — re-creating it records whatever the claims say now — or `git add` it if this commit is the one that created it. Coverage is the only trigger, so deleting a claim's last thread *and* the store together no longer buys silence, and an upgrade still never trips it |
| `comment-digest-missing` | the store is **there** but a claim holding a *standing* approval has no entry in it — the map was emptied rather than the file deleted, which is cheaper to miss in a diff. Every approval records the claim's comment digest in the same act it records the approval, so a standing record with no entry is a statement about the store. Restore the file from version control, or `git add` it if this is the commit that updated it — do **not** run a comment op to re-create the entry, which records whatever the claim says now |
| `comment-digest-unrecorded` | in a ledger-covered project, a claim **holding comment threads** with no entry beside them in the digest store. The map was protected against being emptied wholesale, not against losing one key: hand-forge a thread as `resolved`, then drop that claim's key, and `comment-ledger-drift` had nothing to compare against — the claim locked, and the next ordinary command re-adopted the forged block as truth. An edit smaller than the one it was catching cleared the gate the whole review loop rests on. The predicate is the threads themselves, which is what survives the tamper: the single code path that writes a thread records the claim's digest in the same act, so threads with no entry means either the entry was removed or the threads were never written by the engine. Deliberately silent where evidence is honestly absent — an uncovered project, an absent store (`comment-digest-absent` says that once), and a claim with no threads at all. **`claim lock` and every comment op refuse this state** (`integrity_failed` / `comment_digest_drift`): an approval records the claim's comment digest in the same act, so locking here would manufacture the very evidence whose absence is the finding |
| `comment-digest-abandoned` | a digest entry that recorded review history whose **claim id is no longer in the project** — the rename launder: delete a claim's `comments:` block *and* change its `id:` in one edit and every rule that starts from the claim went quiet, because the old entry is the only thing the tamper could not reach. Silent for an entry that recorded no threads, and for a claim whose record an honest `unlock` released |
| `build-order-content-drift` | a locked `.build-order.<module>.json` no longer matches the sequence that was approved — phases reordered by hand, a claim moved into `excluded`, or the frozen `hashes` baseline spliced so the order stops reporting `stale` |
| `build-order-ledger-missing` | a build-order artifact says `locked: true` with no approval record behind it |
| `build-order-ledger-orphan` | an approved build order with its own `locked` flag cleared to `false` while its ledger record still **stands**. The two rules above skip an unlocked artifact — correctly, since an unlocked one is a proposal nobody approved — so one boolean removed the file from every rule at once while the approved sequence stayed on disk for an agent to follow. Told apart from an honest re-propose by the *release*: `build-order propose` releases the record as it overwrites the artifact, so a standing record here means nothing released it — and a flag flip made together with a content edit is caught too |
| `build-order-ledger-abandoned` | a locked build order's **artifact is gone** while its approval record still stands — the `.build-order.<module>.json` was deleted, or the module was dropped from `modules:` so nothing audits it any more. The rules above all start from the file, so deleting it was quieter than editing it. Release the build order first, then remove it |
| `build-order-unreadable` | a `.build-order.<module>.json` that **is there and will not decode** — truncated or corrupted rather than deleted. It counted as neither present nor absent, so the rules above and the deletion sweep both skipped it and `check` exited 0 over a destroyed sequence. Restore the file from version control; do **not** re-propose, which would record the order the claims imply *now* as the approved one |

The gate runs as the **last** step of `check`, after the catalog and viewer have been written: a tampered project still regenerates its documentation, it just does not exit 0. It is deliberately not a lint — one tampered claim must not be able to freeze locking project-wide.

### Where the gate runs

- **`dossierx check --staged`** judges the git index — what the commit will actually contain — reading content with `git show`, never the worktree, and writing nothing. This is what the **pre-commit hook** runs: [`scripts/install-git-hook.sh`](scripts/install-git-hook.sh) (with [`install-git-hook.ps1`](scripts/install-git-hook.ps1) for PowerShell users). It asks before writing anything, resolves `core.hooksPath` instead of assuming `.git/hooks`, handles linked worktrees, never replaces a foreign hook without `--force`, and re-running it is a no-op.
- **CI is the authority.** git does not run `pre-commit` for a clean merge, a rebase, a cherry-pick or a revert, `--no-verify` is one keystroke away, and most contributors never installed the hook at all. Copy [`scripts/ci/dossierx-check.yml`](scripts/ci/dossierx-check.yml) into your repository's `.github/workflows/`. If you adopt only one of the two, adopt this one — and make it a **required status check on a protected branch**, which is the step that turns a finding into something a merge has to satisfy.
- **The CI template runs plain `dossierx check`, one step, and pins no `fetch-depth`.** Every rule is evaluated against one tree, and a shallow checkout *is* a complete tree — so `actions/checkout`'s default is all the job needs. `--staged` is not wrong in CI, it is redundant: on a fresh checkout the index, the worktree and `HEAD` are three names for the same tree, so it re-runs the same rules over the same bytes. Its home is the hook, where the index and the worktree genuinely differ.

### What the gate does not detect, and where that is caught

Every rule above judges **one tree**: these claim files, this ledger, this digest store, these build-order artifacts, exactly as they are. Within that evidence base, a change to a locked claim's **approved content** leaves a surviving artifact **disagreeing** with the one that moved, and that disagreement is caught. An agent that edits a locked claim, a careless hand-edit, a bad merge, a status flipped by hand in either direction, a deleted approval record, an erased comment thread — each one is a named finding (`lock-content-drift`, `lock-ledger-missing`, `lock-ledger-orphan`, `lock-ledger-deleted`, `comment-ledger-drift`, and the rest of the table above).

One field sits outside that: `review_pending` is engine-managed bookkeeping rather than approved content, so it is excluded from the locked-claim hash and hand-deleting it clears a review flag silently. [FORMAT.md](FORMAT.md) says where that line falls and why.

What it cannot see is the change that writes **both sides at once**, and the reason is one sentence rather than a list of cases:

> **An in-repo ledger cannot attest anything against the person who can write it.**

A claim and its approval record are both tracked files, and both belong to whoever is committing. Edited together, in one commit, they agree — and nothing is left in the tree to disagree with. The sharpest form of it is not a deletion at all. Unlock a locked claim, rewrite its body, re-lock it — which mints a fresh record whose hash correctly covers the new content — then hand-edit that record's `reason`, `at` and `actor` back to the original approval's values. `dossierx check` and `dossierx check --staged` both report `ok: true`, over a ledger that now credits a human with approving something they never read. Nothing was erased; the ledger is complete, consistent and false. That is **one illustration of the principle, not an inventory of the ways to reach it**.

**No in-repo mechanism closes this**, because any evidence the tool consults lives in the repository and the repository is writable by the person being gated. An earlier build of v0.3.0 compared the staged tree against its parent commit; it was removed before release, because the parent commit is outside the *commit* but not outside the *committer* — a rebase, `--orphan` or a second config file switches such a comparison off without looking unusual — and because it refused ordinary `git revert` of a lock commit and audited a genuinely new monorepo project against a retired one's ledger. A control a rebase disables and a revert trips is not a control.

**Where it is caught: the forge.** DossierX detects; branch protection enforces. Put `dossierx check` behind a required status check on a protected branch and every rule above runs on the merge result. Then read the diff, because the change still has to arrive as one: a hand edit of a tracked JSON store whose entire purpose is to be read in a diff, sitting next to the claim change it was made to permit. In the example above the store half is a single line, and it has a signature worth knowing — **the hash moved and the approval did not**. Every honest lock writes `hash`, `at`, `actor` and `reason` in the same act, so a record whose content changed while its timestamp, actor and reason stood still did not come from a lock this engine performed. `CODEOWNERS` on the stores and the config makes that reading someone's job.

This is a designed boundary, not a to-do. **[FORMAT.md states it in full](FORMAT.md#what-the-gate-detects-what-it-does-not-and-where-the-rest-is-caught)** — what the tree can prove, what it cannot, why no file in the repository fixes that, and the one direction that would: signing the ledger with a key held outside the repository, of which git's own commit signing is the cheapest form.

#### Moving `claims_dir`

No ceremony, and no flag, environment variable or config marker that exempts anything (an escape hatch on an integrity gate is itself the attack). Move the claim files, edit `claims_dir:`, stage the claims, the config and the unchanged stores together:

```sh
git mv claims docs/claims            # take the claims with you
$EDITOR project.config.yaml          # claims_dir: docs/claims
git add -A                           # the claims, the config AND the unchanged stores
dossierx check --staged              # verify before committing
git commit
```

It passes because every locked claim is still reachable and still hashes to its existing record — the same thing the rules were reading all along. A move that **strands** locked claims fails from state alone, as `lock-ledger-abandoned`, once per claim.

The ledger is not authentication. `actor` is provenance, not identity, and anyone who can edit a claim can edit the ledger. What it buys is that tampering requires editing **two** tracked files consistently instead of one — and the second is a file whose entire purpose is to be read in the diff.

## Upgrading a pre-ledger project

**There is no migration command.** v0.4.0 removed `dossierx migrate --adopt` outright, and nothing is grandfathered by any run, in any mode. A project whose locks predate the lock ledger crosses onto it by emptying itself of everything that predates it.

**The state.** A lock store written before the ledger existed — schema `version: 1`, no `ledger` key — fails `check` with the project-scoped `lock-ledger-pre-ledger` finding, and the three approval-recording commands (`claim lock`, `claim reaudit --confirm`, `build-order lock`) refuse with `error.code: pre_ledger_unadopted`. Both happen **only while the project still holds a locked claim or a locked build order.** Hold nothing locked and both are silent.

**The crossing.** The order is load-bearing, not cosmetic: `build-order propose` requires the module still fully locked, so re-proposing has to happen *before* any claim is unlocked — unlock first and propose then refuses, leaving the locked order with no way to be released. One decision belongs before the first command, per module: will you re-lock *every* claim in it at step 3? A build order exists only over a fully locked module, and step 1 releases the approved sequence — so a module you re-lock only partially finishes the crossing gate-green but without a locked build order, and its step 4 waits until the day its last claim locks.

```sh
# 1. FIRST, for every module whose build order is locked:
dossierx build-order propose --module <m>

# 2. then every locked claim — unlock is gateless and always has been:
dossierx claim unlock <id> --reason "<your words>"

# 3. then re-lock only what you still stand behind:
dossierx claim lock <id> --reason "<your words>"

# 4. then the build orders again, for every module that is fully locked
#    again. A module you re-locked only partially has nothing to propose
#    yet — run this pair for it on the day its last claim locks:
dossierx build-order propose --module <m>
dossierx build-order lock --module <m> --reason "<your words>"
```

**The first `claim lock` in a project holding nothing locked is what crosses it.** That lock stamps the store onto the ledger schema and creates `.dossierx-comment-digest.json` in the same act — and it records a real approval, with your own words in `--reason`. That is the whole difference from the adoption path this replaces. Commit the rewritten `.dossierx-lock-store.json` and the new `.dossierx-comment-digest.json` together with the re-locks.

**Why there is no command any more.** v0.3.0 answered this state with `dossierx migrate --adopt`, and even then the reasoning was uncomfortable: *adoption is the one operation that manufactures approval out of nothing*. No predicate inside a project can tell an honest v0.2.x store from a downgraded one — `locked_at` shipped in v0.2.0 (`git show v0.2.0:internal/lock/lock.go`), so there is no field, no timestamp and no sibling file whose presence or absence distinguishes the two. And a migration you can re-run is a laundering command: delete one record, migrate again, and the edit it covered is re-signed as approved. v0.3.0 answered that with an `already_migrated` refusal; v0.4.0 answers it by having no command to re-run. The crossing above is safe from an ordinary write path for exactly that reason — an attacker who empties the project of every locked artifact to trigger it has destroyed the approvals they were trying to launder.

**`dossierx migrate` survives as a hidden retired stub** that fails naming this path. It has to exist as a command at all because cobra parses flags before any unknown-command handler runs: without it, `dossierx migrate --adopt` would fail as `unknown flag: --adopt` for every agent that still remembers the old instruction, rather than as an explanation.

**A near neighbour worth telling apart**, and the lock store itself is what distinguishes them — no git history required. **`lock-ledger-pre-ledger`** means the store is **present**, still on the pre-ledger schema, and something is still locked: benign, and the recovery is the crossing. **`lock-ledger-absent`** means the store **file is gone** while locked claims remain: tampering, and the recovery is version control.

**Do not "fix" a ledger complaint by deleting the store or by re-locking.** Both record the current bytes as approved with no diff shown to anyone.

## Concepts

**Claims.** A claim is one atomic, YAML-authored fact about a system: a field table, a sequence of steps, a paragraph of prose, a piece of hand-authored mockup HTML. One claim per file, under the project's `claims_dir`.

```yaml
id: widget.contract.overview     # module.facet.slug
facet: contract                  # the tab it renders under
module: widget
status: draft                    # draft | locked
layout: card
body: |
  A widget is the smallest unit this project documents.
rests_on:
  - widget.internals.storage     # this claim is true only while that one is
governed_by:
  type: none
  reason: no doctrine facet configured yet
```

Every claim has an `id` (`module.facet.slug`), a `facet`, a `module`, a `status`, a `layout` (`card`, `table`, `list`, `steps`, `tree`, `banner`, `mockup`), and a `governed_by` block naming what backs its truth (a doctrine claim, or `none` with a reason). Claims name other claims they `rests_on` or `mirrors` — and a claim-valued `governed_by.type` is the same kind of edge for drift purposes — forming a graph the engine walks and validates. The full schema is in [FORMAT.md](FORMAT.md).

**`check` is the pipeline.** One command runs lint → catalog → render → the ledger gate and stops at the first failure. `--validate` is the read-only form for the authoring loop: it runs the lint gate and the ledger gate in memory and writes nothing — no claim files, no lock store, no `.catalog.json`, no viewer. It also does not reconcile `review_pending`, rebuild the catalog or the viewer, or scan source for code links; run plain `check` before trusting what the viewer shows.

**The lock lifecycle.** A `draft` claim is freely editable. `dossierx claim lock <id>` promotes it to `locked` — refused if lint has any error, if doctrine hub-gating blocks it, or if the claim still carries an unresolved comment thread. A locked claim never silently changes: it is flagged `review_pending` on any of three independent triggers — a dependency it `rests_on`, `mirrors` or is `governed_by` drifted, a `dossierx claim flag` recorded that its stated behavior no longer matches reality, or an open comment thread was added — rather than being auto-updated. `governed_by` joined the drift set in v0.4.0 as a **drift** edge only: a claim-valued governor whose content changes flags its dependants `review_pending`, but hub gating still walks `rests_on`/`mirrors` alone, so an unlocked governor named only by `governed_by` still never refuses a lock. There is no backfill — a claim locked before v0.4.0 carries no governance baseline until its next `claim lock` or confirmed `claim reaudit`, so the first governor edit after upgrading does not flag it. `review_pending` is set automatically and never cleared automatically; it clears only once every trigger is gone, via one of three matching clearers: a confirmed `dossierx claim reaudit <id> --confirm --reason "..."` (drift/flag), `dossierx claim unlock`, or the human resolving the last open thread in the viewer.

**`reaudit` is the drift tool, not the general edit tool.** It refuses a claim that is not already `review_pending`, it rewrites only `body`, and it refuses a claim whose only trigger is an open thread (there is no diff to confirm — resolve the thread instead). To change anything else about a locked claim, the path is `unlock → fix → lock`.

## Config schema (`project.config.yaml`)

| Field | Type | Required | Description |
|---|---|---|---|
| `schema_version` | int | yes | Must equal the version this DossierX build understands (currently `1`). |
| `facets` | []string | yes | The non-empty, deduplicated list of facet names (tabs) this project uses, e.g. `[contract, internals]`. |
| `modules` | []string | yes | The non-empty, deduplicated list of module names this project documents. |
| `claims_dir` | string | yes | Directory of claim YAML files, resolved relative to `project.config.yaml`'s own directory (never the process's current working directory). |
| `title` | string | no | The viewer's display name — used in `<title>`, the header, and the sidebar heading. Defaults to a generic fallback when unset. |
| `eyebrow` | string | no | A one-line subtitle rendered under the title in the sidebar header. No line is rendered when unset. |
| `doctrine_facet` | string | no | Names one of `facets` as the project's doctrine facet, enabling hub-gating. Must be a facet the project actually declares. |
| `source_dirs` | []string | no | Directories (relative to the config file) scanned for `dossierx-claim: <id>` source comments — the code side of claim-to-code linking. Unset means "do not scan." |
| `mockup_modules` | []string | no | The allowlist of modules permitted to author `raw_html` (on any layout, including `layout: mockup`). Every entry must also appear in `modules`. Unset/empty means no module may. |
| `viewer.template_overrides` | string | no | A directory of partial-template overrides, resolved relative to the config file. Missing individual partials fall back to engine defaults; a configured-but-missing directory is a hard error. |
| `viewer.theme` | map[string]string | no | CSS custom-property overrides. Keys must be drawn from the fixed allowlist below (without the leading `--`); values are validated defensively before being injected into a generated stylesheet. |

`viewer.theme`'s 14 allowed keys: `accent`, `accent-bg`, `ink`, `muted`, `faint`, `paper`, `card-bg`, `border`, `link`, `warn`, `warn-bg`, `font-sans`, `font-mono`, `radius`.

Config loading is strict: an unknown top-level or `viewer.theme` field is a hard error, not silently ignored.

## The skills

DossierX ships embedded [Claude Code](https://claude.com/claude-code) skills that teach an agent working in a *consuming* project how to operate it. `dossierx` is the router, loaded first and always: the seven nouns, the envelope, the exit codes, the error-code-to-recovery table, and which companion to load next. The companions are `dossierx-claims` (author, find, and move claims through their lifecycle), `dossierx-build-order` (derive a locked module's implementation order), `dossierx-code-links` (ground finished code in the claims it implements), and `dossierx-comments` (run review threads, and when to comment versus `flag`). See [`skills/`](skills/) for what each covers.

`dossierx skills export [dir]` writes them into a project, creating parent directories and overwriting in place, so re-running it is how you pick up a new release's guidance. Step 2 of the paste block above does this. `[dir]` is optional only *inside* an existing project — with neither a directory nor a `project.config.yaml` to root the write in there is nowhere to install to, and the command refuses with `write_failed`. That is why step 2 names `.claude/skills` explicitly: it runs before the config exists, so the guides are in place to be followed while the project is set up. Add a project-specific overlay skill alongside them for anything local to your repo — house style, module conventions — that the generic skills cannot know.

The skills are one source written in three forms, because no two agent harnesses read the same file:

| Form | Where | Notes |
| --- | --- | --- |
| `SKILL.md` tree | the `[dir]` you name, else `.claude/skills` if `.claude` already exists | verbatim bundles, frontmatter intact |
| `AGENTS.md` section | an existing `AGENTS.md` only — never created | marker-delimited and idempotent; carries the router only, since this text is resident on every turn |
| `dossierx-agent-guide.md` | `docs/` under the project root; `[dir]` itself when there is no project to root it in | always written — all five bundles inline, self-contained, no loader or plugin needed |

Both derived forms are regenerated by re-running the export, so they are committed artifacts like the ledger: re-export to pick up a new release, and commit the result.

## Scope

DossierX is a CLI only — there is no public Go package API. Everything here is invoked through the `dossierx` binary; `internal/` is not importable by other modules and its structure is free to change between releases without that counting as a breaking change. The runtime dependency set is deliberately two packages (cobra and yaml.v3) and adding a third is a decision, not a convenience.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, package boundary rules, and how to run tests and lint locally.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
