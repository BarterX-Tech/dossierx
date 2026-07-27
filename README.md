# DossierX

DossierX turns a directory of YAML "claims" — atomic, reviewable facts about a system — into a linted, cross-checked HTML site a human can actually read and argue with. Each claim can be **locked** once a human has approved it, after which it never changes silently: a drifted dependency, a code change that contradicts it, or an open comment thread flags it for review instead. It is built to be operated by an **agent** and reviewed by a **human**, and this release gives each of them their own surface.

[![CI](https://img.shields.io/github/actions/workflow/status/BarterX-Tech/dossierx/ci.yml?branch=main&label=CI)](https://github.com/BarterX-Tech/dossierx/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/BarterX-Tech/dossierx)](https://github.com/BarterX-Tech/dossierx/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/BarterX-Tech/dossierx)](https://github.com/BarterX-Tech/dossierx/releases)

## Two roles

|  | **Agent** — the operator | **Human** — the reviewer |
|---|---|---|
| **Surface** | the CLI: 19 commands under 6 nouns, JSON by default | the viewer: `dossierx serve`, plus chat with the agent |
| **Does** | writes and restructures draft claims, links code, replies on threads, runs `check`, executes lifecycle actions you approved | reads claims, comments on any card, resolves and reopens threads, says "lock it" |
| **Cannot** | change a **locked** claim without an approval on the record; resolve or reopen your threads; edit or delete comments | (nothing is *prevented* — you are the approver; you simply shouldn't need to type a DossierX command other than `serve`) |

The gate is narrower than "the agent may not touch claims", which would defeat the point. Draft claims are the agent's workshop and stay unfrictioned. The invariant is: **nothing already locked changes without your approval on the record** — see [the lock ledger](#integrity-the-lock-ledger).

## Start here — paste this to your agent

Paste this into Claude Code, Codex, or any other coding agent working in the repository you want documented. It is idempotent: running it again on a project that is already set up changes nothing and reports what it found.

```text
Set up DossierX in this repository.

1. If the `dossierx` binary is missing, install it with
   `go install github.com/BarterX-Tech/dossierx/cmd/dossierx@v0.3.0`,
   then run `dossierx version` and show me the output.
2. Run `dossierx skills export .claude/skills` — or point it at whichever
   skills/instructions directory this harness actually reads. Load what it
   wrote and follow it: those guides, not this message, are the contract.
3. If `project.config.yaml` and the claims directory do not exist yet,
   propose a title, the facets, and the modules, and WAIT for me to confirm
   before writing anything.
4. ASK ME before installing the git pre-commit hook. If I say yes, fetch
   https://raw.githubusercontent.com/BarterX-Tech/dossierx/v0.3.0/scripts/install-git-hook.sh
   to a file, show me what it does, then run `sh install-git-hook.sh --yes`.
   If I say no, add the CI workflow instead and tell me so — CI is the
   authority either way.
5. Run `dossierx check --format text` and show me the output. Do not tell me
   it works; show me it exiting 0.
6. Tell me to commit `.dossierx-lock-store.json` — and
   `.dossierx-comment-digest.json` and `.dossierx-flag-store.json` once they
   appear — together with the claim files. All three are tracked artifacts:
   the ledger is what CI compares the claims against, the gate is vacuous
   without it, and a flag that does not travel with its claim reaudits to an
   empty proposal on the next machine. Never add them to .gitignore.
7. Tell me to run `dossierx serve`. That is the only DossierX command I run.

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

Closing a thread is yours alone, and in v0.3.0 that is structural rather than polite: `dossierx comment` is `inbox · list · add · reply` and nothing else — resolve, reopen, edit and delete were removed from the CLI in this release and are reachable only from the viewer and `dossierx serve`'s HTTP API, which is to say, only from your click.

Underneath that, the older guarantee still holds and is enforced in the engine rather than by convention: an agent may only act on its own messages, so even over the HTTP API it cannot touch a thread or a message you authored. Everything you wrote is yours to close.

A static `file://` export of the viewer is read-only by design — comments need `dossierx serve`.

## The CLI surface

Nineteen leaf commands under six nouns. This is a *machine* surface: a human is not expected to run any of it. Use `dossierx <noun> --help` for flags, and `--format text` when you want prose.

```text
check                    lint, catalog, render and the lock-ledger gate in one shot
                         --validate  read-only: lint + ledger gate, writes nothing
                         --staged    judge the git index instead of the working tree

claim        show · list · new · lock · unlock · flag · reaudit · link
comment      inbox · list · add · reply
build-order  propose · status · lock

serve                    the human's viewer + comment API
skills export [dir]      write the embedded agent skills into a project
version                  version, commit, build date (also --version)
```

Every subcommand takes the global `--config` (a path to `project.config.yaml`; when omitted, DossierX searches upward from the current directory the way `git` finds `.git`) and `--format json|text`.

Upgrading from v0.2.x? Ten commands were removed and four moved — [the CHANGELOG has the full migration table](CHANGELOG.md).

### The machine contract

`--format json` is the default, and every run emits exactly one envelope:

```json
{"ok": false, "command": "claim lock", "stopped_at": "lint",
 "error": {"code": "lint_failed", "message": "...", "hint": "run: dossierx check"}}
```

`error.code` is a stable snake_case token — `lint_failed`, `claim_not_found`, `rights_denied`, `integrity_failed`, `unresolved_comments`, and so on. Branch on it. `message` and `hint` are prose and will be reworded; `code` is the promise. Successful runs carry their payload in `data`, non-blocking findings in `warnings`, and `check` reports how far it got in `stopped_at`.

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

Every legitimate approval — `claim lock`, a confirmed `claim reaudit`, `build-order lock` — writes a record into the **lock ledger**: the hash of exactly what was approved, when, by which account, and the human's own `--reason` words. Unlocking marks the record released rather than deleting it, so the evidence that a claim was ever locked survives. Comment history gets the same treatment in its own digest store, which is why `serve` never needs write access to the lock store.

Three files hold the review state, at the project root, next to `project.config.yaml`:

| File | Holds |
|---|---|
| `.dossierx-lock-store.json` | the lock ledger — per locked claim and locked build order: `{hash, at, actor, reason}` |
| `.dossierx-comment-digest.json` | the review history's fingerprint |
| `.dossierx-flag-store.json` | the pending `claim flag` triggers: each flagged claim's `{claim_says, now_does, reason, flagged_at}`, parked until a confirmed `claim reaudit` consumes it |

**All three are tracked artifacts. Commit them; never `.gitignore` them** — the lock store the moment anything is locked, the digest once anyone comments, the flag store the moment anything is flagged. A claim and its approval have to travel in the same commit for CI to be able to check either one: CI compares the claims against the ledger, so without the ledger in the repository the gate has nothing to compare against and is theatre. DossierX says so out loud rather than passing quietly: locked claims with no ledger is a hard error (`lock-ledger-absent`), and a ledger that exists but will not parse is `lock-ledger-unreadable`, never a silent skip.

The flag store is not part of the gate — nothing compares it to anything — but it is not optional either. `claim flag` writes the before/after there and sets `review_pending` on the claim; `claim reaudit` reads it back to produce the diff it asks you to confirm. If the claim travels to another machine and the flag store does not, that claim arrives `review_pending` with nothing to propose, and confirming the empty proposal clears the flag having changed nothing.

That makes it the one store with **no integrity coverage in either direction**: deleting it, `.gitignore`-ing it, or emptying its map is silent — `check` still exits 0, and your recorded "the claim says X, the code does Y" is gone with nothing in the report to say so. It is a bounded hole (a flag is a request for review, not an approval, so erasing one cannot make a locked claim change or an unapproved claim read as approved) but it is a real one, and the mitigation is procedural until a rule covers it: commit the store with the claim it describes, and read an *empty* `reaudit` proposal on a `review_pending` claim as a missing flag entry rather than as "nothing to change". [FORMAT.md](FORMAT.md#the-project-root-stores-are-tracked-artifacts) states the same thing next to the findings that do exist.

These are the exception, not the rule. `.catalog.json` and `viewer/` are *generated* — regenerated in full by every `dossierx check` — and are safe to `.gitignore`. `.build-order.<module>.json` starts out in that generated category and leaves it the moment you lock one: a **locked** build order is an approved artifact the gate compares against its record, so commit it like the stores above.

The gate names each disagreement:

| Finding | What it caught |
|---|---|
| `lock-ledger-missing` | a claim is `locked` with no approval record — e.g. `status: draft` flipped to `locked` by hand, walking past the lint, hub-gating and unresolved-comment gates |
| `lock-ledger-downgraded` | the lock store **says it predates the ledger** while the project proves it does not. Projects that locked claims before this release are grandfathered in, and that grandfathering keys on the store's own `version` field — so setting it back to `1` and deleting the `ledger` key re-ran adoption and recorded the claims *as they are now* as approved. One edit, inside the audited file, and the gate said ok. A downgrade must now survive evidence the store does not own: the sibling `.dossierx-comment-digest.json` this build writes the moment a project becomes ledger-covered, or records still sitting in a store that claims to predate records. Nothing is grandfathered when this fires. Restore the store from version control — do **not** re-lock |
| `lock-ledger-released` | a claim is `locked` on a record an `unlock` already **released** — lock, unlock, then hand-edit `status:` back. A released record is a withdrawn approval, not a standing one, and the content hash cannot see it because the hash excludes `status` |
| `lock-content-drift` | a locked claim's content no longer matches what was approved — including fields the dependency-drift hash never covered, such as `raw_html`, `build_role`, `section` and `order` |
| `lock-ledger-orphan` | a `draft` claim still holding an *unreleased* record — `locked` flipped back to `draft` to dodge review |
| `lock-ledger-abandoned` | a locked claim's **file was deleted** while its approval record still stands. There is no `claim delete` verb, so `rm` was the one change to a locked claim that no rule saw: every other finding starts from the claims that exist. Unlock first, then delete |
| `comment-ledger-drift` | a review thread edited or deleted outside the engine |
| `comment-digest-absent` | `.dossierx-comment-digest.json` is **gone** from a ledger-covered project, so the rule above is checking nothing at all. Deleting the store was how an edited-away thread stopped being reported: a claim the store has never seen is *unknown*, never *drifted*. Restore the file from version control — re-creating it records whatever the claims say now — or `git add` it if this commit is the one that created it. Coverage is the only trigger, so deleting a claim's last thread *and* the store together no longer buys silence, and an upgrade still never trips it |
| `comment-digest-missing` | the store is **there** but a claim holding a *standing* approval has no entry in it — the map was emptied rather than the file deleted, which is cheaper to miss in a diff. Every approval records the claim's comment digest in the same act it records the approval, so a standing record with no entry is a statement about the store. Restore the file from version control, or `git add` it if this is the commit that updated it — do **not** run a comment op to re-create the entry, which records whatever the claim says now |
| `comment-digest-abandoned` | a digest entry that recorded review history whose **claim id is no longer in the project** — the rename launder: delete a claim's `comments:` block *and* change its `id:` in one edit and every rule that starts from the claim went quiet, because the old entry is the only thing the tamper could not reach. Silent for an entry that recorded no threads, and for a claim whose record an honest `unlock` released |
| `build-order-content-drift` | a locked `.build-order.<module>.json` no longer matches the sequence that was approved — phases reordered by hand, a claim moved into `excluded`, or the frozen `hashes` baseline spliced so the order stops reporting `stale` |
| `build-order-ledger-missing` | a build-order artifact says `locked: true` with no approval record behind it |
| `build-order-ledger-orphan` | an approved build order with its own `locked` flag cleared to `false` while its ledger record still **stands**. The two rules above skip an unlocked artifact — correctly, since an unlocked one is a proposal nobody approved — so one boolean removed the file from every rule at once while the approved sequence stayed on disk for an agent to follow. Told apart from an honest re-propose by the *release*: `build-order propose` releases the record as it overwrites the artifact, so a standing record here means nothing released it — and a flag flip made together with a content edit is caught too |
| `build-order-ledger-abandoned` | a locked build order's **artifact is gone** while its approval record still stands — the `.build-order.<module>.json` was deleted, or the module was dropped from `modules:` so nothing audits it any more. The rules above all start from the file, so deleting it was quieter than editing it. Release the build order first, then remove it |
| `build-order-unreadable` | a `.build-order.<module>.json` that **is there and will not decode** — truncated or corrupted rather than deleted. It counted as neither present nor absent, so the rules above and the deletion sweep both skipped it and `check` exited 0 over a destroyed sequence. Restore the file from version control; do **not** re-propose, which would record the order the claims imply *now* as the approved one |

The gate runs as the **last** step of `check`, after the catalog and viewer have been written: a tampered project still regenerates its documentation, it just does not exit 0. It is deliberately not a lint — one tampered claim must not be able to freeze locking project-wide.

### Where the gate runs

- **`dossierx check --staged`** judges the git index — what the commit will actually contain — reading content with `git show`, never the worktree, and writing nothing. This is what the **pre-commit hook** runs: [`scripts/install-git-hook.sh`](scripts/install-git-hook.sh) (with [`install-git-hook.ps1`](scripts/install-git-hook.ps1) for PowerShell users). It asks before writing anything, resolves `core.hooksPath` instead of assuming `.git/hooks`, handles linked worktrees, never replaces a foreign hook without `--force`, and re-running it is a no-op.
- **CI is the authority.** git does not run `pre-commit` for a clean merge, a rebase, a cherry-pick or a revert, `--no-verify` is one keystroke away, and most contributors never installed the hook at all. Copy [`scripts/ci/dossierx-check.yml`](scripts/ci/dossierx-check.yml) into your repository's `.github/workflows/`. If you adopt only one of the two, adopt this one.

The ledger is not authentication. `actor` is provenance, not identity, and anyone who can edit a claim can edit the ledger. What it buys is that tampering now requires editing **two** tracked files consistently instead of one — and the second is a file whose entire purpose is to be read in the diff.

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

Every claim has an `id` (`module.facet.slug`), a `facet`, a `module`, a `status`, a `layout` (`card`, `table`, `list`, `steps`, `tree`, `banner`, `mockup`), and a `governed_by` block naming what backs its truth (a doctrine claim, or `none` with a reason). Claims name other claims they `rests_on` or `mirrors`, forming a graph the engine walks and validates. The full schema is in [FORMAT.md](FORMAT.md).

**`check` is the pipeline.** One command runs lint → catalog → render → the ledger gate and stops at the first failure. `--validate` is the read-only form for the authoring loop: it runs the lint gate and the ledger gate in memory and writes nothing — no claim files, no lock store, no `.catalog.json`, no viewer. It also does not reconcile `review_pending`, rebuild the catalog or the viewer, or scan source for code links; run plain `check` before trusting what the viewer shows.

**The lock lifecycle.** A `draft` claim is freely editable. `dossierx claim lock <id>` promotes it to `locked` — refused if lint has any error, if doctrine hub-gating blocks it, or if the claim still carries an unresolved comment thread. A locked claim never silently changes: it is flagged `review_pending` on any of three independent triggers — a dependency it `rests_on`/`mirrors` drifted, a `dossierx claim flag` recorded that its stated behavior no longer matches reality, or an open comment thread was added — rather than being auto-updated. `review_pending` is set automatically and never cleared automatically; it clears only once every trigger is gone, via one of three matching clearers: a confirmed `dossierx claim reaudit <id> --confirm --reason "..."` (drift/flag), `dossierx claim unlock`, or the human resolving the last open thread in the viewer.

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
| `mockup_modules` | []string | no | The allowlist of modules permitted to author `layout: mockup` claims. Every entry must also appear in `modules`. Unset/empty means no module may. |
| `viewer.template_overrides` | string | no | A directory of partial-template overrides, resolved relative to the config file. Missing individual partials fall back to engine defaults; a configured-but-missing directory is a hard error. |
| `viewer.theme` | map[string]string | no | CSS custom-property overrides. Keys must be drawn from the fixed allowlist below (without the leading `--`); values are validated defensively before being injected into a generated stylesheet. |

`viewer.theme`'s 14 allowed keys: `accent`, `accent-bg`, `ink`, `muted`, `faint`, `paper`, `card-bg`, `border`, `link`, `warn`, `warn-bg`, `font-sans`, `font-mono`, `radius`.

Config loading is strict: an unknown top-level or `viewer.theme` field is a hard error, not silently ignored.

## The skills

DossierX ships embedded [Claude Code](https://claude.com/claude-code) skills that teach an agent working in a *consuming* project how to operate it. `dossierx` is the router, loaded first and always: the six nouns, the envelope, the exit codes, the error-code-to-recovery table, and which companion to load next. The companions are `dossierx-claims` (author, find, and move claims through their lifecycle), `dossierx-build-order` (derive a locked module's implementation order), `dossierx-code-links` (ground finished code in the claims it implements), and `dossierx-comments` (run review threads, and when to comment versus `flag`). See [`skills/`](skills/) for what each covers.

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
