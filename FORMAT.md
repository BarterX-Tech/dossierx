# DossierX claim format

This document describes the on-disk claim schema and project config schema
this engine reads. It is generic by design — nothing here names any
specific project, module, or facet. All project-specific vocabulary
(which facets exist, which modules exist, where claims live) comes from
`project.config.yaml`, never from the engine itself.

## Claim

A claim is one atomic YAML fact, one claim per file, under the project's
configured `claims_dir`. This is enforced: a claim file must contain exactly
one YAML document. Stacking a second `---`-separated document into the same
file is a hard load error (the engine rewrites a claim's file as a single
document when it locks or reaudits it, so a second document in that file
would be silently clobbered). Split multiple claims into separate files.

```yaml
id: module.facet.slug          # e.g. widget.contract.overview
facet: string                  # must be in project.config.yaml's facets[]
module: string                 # must be in project.config.yaml's modules[]
status: draft | locked
layout: card | table | list | steps | tree | banner | mockup  # optional
build_role: orientation | schema | behavior | api | verification | out-of-scope  # optional (see below)
body: markdown string          # optional, illustrative prose
rows: [ { ... } ]              # optional, table rows; each cell must be a string
steps: [ string ]              # optional, ordered steps
raw_html: string               # optional, layout: mockup only (review-gated)
raw_html_reviewed: bool        # optional, human-set gate for raw_html
section: string                # optional, in-content section heading (see below)
mirrors: [ id, ... ]
rests_on: [ id, ... ]
governed_by:
  type: none | doctrine_id
  reason: string                # required when type is "none"
migrated_from: string           # optional provenance note
order: int                      # optional, viewer-only display sequencing (see below)
comments:                       # optional, engine-managed review threads — authored via `dossierx comment`, not by hand (see below)
  - id: c-8f3a2b                # engine-generated: "c-" + 6 lowercase hex, unique within the file
    status: open | resolved
    author: human | agent       # role, not identity
    created: 2026-07-24T10:12:00Z   # RFC 3339 UTC
    body: markdown string
    edited: bool                # true once the thread root has been edited
    replies:                    # optional follow-ups
      - id: r-4c9e11            # engine-generated: "r-" + 6 lowercase hex
        author: human | agent
        created: 2026-07-24T10:40:00Z
        body: markdown string
        edited: bool
    resolved_by: human | agent  # optional, set when the thread is resolved
    resolved_at: 2026-07-24T11:02:00Z   # optional, RFC 3339 UTC
    reopened_by: human | agent  # optional, set when the thread is reopened
    reopened_at: 2026-07-24T11:10:00Z   # optional, RFC 3339 UTC
```

### `id` grammar

`id` is three dot-separated segments: `module.facet.slug`.

- `module` — one of the project's configured `modules[]`.
- `facet` — one of the project's configured `facets[]`.
- `slug` — a free-form, kebab-case identifier unique within that
  `module.facet` pair.

### `layout` inference

When `layout` is omitted, it is inferred from the claim's shape:

1. `rows` is non-empty → `table`.
2. Otherwise, `steps` is non-empty → `steps`.
3. Otherwise → `card`.

`list`, `tree`, `banner`, and `mockup` are never inferred; a claim must set
them explicitly. `mockup` renders a project-authored `raw_html` blob instead
of markdown/rows/steps and carries its own human review gate — see the
`raw-html-scope` lint for the full constraints.

### `body` and the markdown ceiling

`body` is markdown, but the engine's renderer is a small owned subset rather
than a general parser. It supports paragraphs, fenced code blocks, inline
`` `code` `` spans, `[text](url)` links whose scheme is allowlisted, and
unordered/ordered lists nested one level deep. Everything else — bold, italic,
headings, blockquotes, markdown tables, deeper nesting — stays literal text.

This is a documented ceiling, not a silent gap: the same subset renders `body`,
every `rows` cell, every `steps` entry, and every comment body, so what a claim
author sees in one place is what they get in all of them. A future release
widens it.

### `rows` cells

Every value in a `rows` cell must be an authored **string**. A non-string
cell — a number, bool, list, or map — is a `rows-shape` lint error:
`table.html` renders each cell as-is, so an unquoted `1.0` would silently
become `"1"` and a list/map would render as Go-native junk. Quote such values
in the YAML. Cells flow through the same inline renderer as `body` prose, so a
cell's `code` spans and `[text](url)` links render as HTML (URL schemes are
allowlisted — http, https, mailto, `#`-fragment, and relative only; others are
neutralized to literal text); all other markdown in a cell stays literal.

### `order` and viewer sequencing

`order` is optional and purely a viewer concern — it has no effect on
`.catalog.json` or lint output.

- Unset (or `0`) means "no explicit order": the claim keeps whatever stable
  fallback position it would otherwise get (currently source-file order).
- Set to a positive int to pin the claim ahead of every unordered claim in
  its module/facet group, ascending by `order` among claims that set it.

This is deliberately separate from `internal/catalog.Document`'s claim
order, which is always alphabetical by `id` — that ordering exists solely
to keep `.catalog.json` and lint diffs byte-deterministic across builds and
must never be repurposed for display sequencing. `order` only reorders how
a module/facet group's claims are laid out in the rendered viewer
(`internal/render.orderClaims`); it does not exist in `.catalog.json`.

### `comments`

`comments` is optional, engine-managed review discussion attached to a claim —
the threaded, Google-Docs-style "comments on claims" surface. Like
`review_pending` and `audit_notes`, it is engine bookkeeping rather than
authored claim content:

- It is **excluded from a claim's content hash**, so commenting on a claim
  never flips its dependents to `review_pending`.
- The field is `omitempty`: a claim that has never been commented on is written
  byte-for-byte as it was before this field existed.

Do **not** hand-edit `comments`; author it through the engine, which takes the
project-wide claims lock, re-reads the claim inside it, and writes it back
safely. Two surfaces reach the same operations: the CLI (`dossierx comment
add` / `reply` / `list` / `inbox`) and `dossierx serve`'s HTTP API, which is
what the viewer drives. Resolve, reopen, edit and delete exist only on the
second one.

**That split is a rule, not a wall, and the two surfaces do not enforce it
equally.** Advisory rights — an actor acts only on its own messages — are
enforced in `internal/comments` against the actor it is handed, and on the CLI
that actor is the required `--as`, so `dossierx comment reply --as agent`
cannot close a human's thread and fails with `rights_denied`. The viewer's API
takes the actor from the request body and treats a request that omits `as` as
`human` (`internal/serve.actorFromString`), so **any local caller reaching
`dossierx serve` has full human rights** and can resolve, reopen, edit or delete
a human's thread — leaving a record that positively attests `human`. The server
binds `127.0.0.1` and applies Host/Origin admission checks, but it does not
authenticate; the honest statement is that its write API is the same trust level
as write access to the claim files, which any such caller already has. Do not
read "viewer-only" as "the party being reviewed cannot reach it". A hand-edited
thread is detected rather than accepted: see `comment-ledger-drift` under
Integrity invariants below. Each thread and reply `id` is engine-generated (`c-`/`r-` followed
by 6 lowercase hex, unique within the claim file); a hand-authored or legacy
entry that omits its `id` is assigned one on the next engine write, so strict
decoding never rejects it.

`author`, `resolved_by`, and `reopened_by` record a **role** (`human` or
`agent`), not an identity — the same axis as the CLI's `--as` flag. A banner
(`layout: banner`) claim is decorative and cannot carry comment threads.

### `build_role` and the build/implementation order

`build_role` is optional and orthogonal to `order`/`section` above: those
two are viewer-only reading-order concerns, while `build_role` drives a
different, additional ordering concept — a module's build (implementation)
order, computed by the engine's `internal/buildorder` package once every
claim in that module is locked.

- Unset (`""`) is allowed while a claim is `draft` — a human may not have
  decided yet where a claim sits in its module's build sequence.
- Once a claim locks, `build_role` becomes required for that claim's
  module, but only once that module has set `build_role` on at least one
  other claim — a module that has never used `build_role` at all sees no
  change in its lock-time behavior. This is enforced by the
  `build-role-required-for-locked` lint, not by the schema itself.
- The six values, in the fixed sequence a build order is computed in
  (`out-of-scope` is never part of the sequence — see below):
  1. `orientation` — context/process claims read for background but never
     themselves acted on during implementation.
  2. `schema` — data-shape claims (types, fields, storage layout); built
     first among the "real work" phases.
  3. `behavior` — workflow/logic claims, the bulk of the real
     implementation work; ordered within this phase by `rests_on` edges to
     other `behavior` claims in the same module.
  4. `api` — public-function/entry-point claims, built after the behavior
     they call into.
  5. `verification` — test-checklist/acceptance-criteria claims, read last
     so tests can be written against everything else already built.
  6. `out-of-scope` — deferred/future-scope claims. Never placed in a
     module's build order, but still reported (as excluded) by
     `internal/buildorder`, so nothing silently vanishes from view.
- A `rests_on` edge from one claim to another claim in the SAME module
  whose `build_role` is a later phase in the sequence above is a
  phase-order violation — a modeling error the dependency graph doesn't
  respect the fixed phase sequence — and is refused, by name, when a
  build order is proposed. A `rests_on` edge to a claim in a DIFFERENT
  module is informational only and never checked this way: cross-module
  dependencies are out of scope for one module's own build sequence.

See `internal/buildorder`'s package doc comment for the full propose /
status / lock lifecycle (`dossierx build-order propose|status|lock`), which
mirrors `internal/lock`'s own draft→locked→stale lifecycle for claims.

### `section` and in-content headings

`section` is optional, free-form, human-readable text (e.g.
`"5 - workflows / lifecycle"`) that a claim author may set to get an
in-content section heading rendered in the content area, grouping that
claim under a visible label as the viewer scrolls.

`section` is the only supported way to get this — the engine never derives
a heading (or any other structure) from `claims_dir`'s directory layout.
See "Directory layout" under Project config below for why: directory
naming is cosmetic only, so section identity has to come from a real
schema field instead of a path convention.

### `status` and the lock lifecycle

- `draft` — freely editable, not yet reviewed.
- `locked` — has passed human review via `dossierx claim lock` (refused if lint
  has any error-level finding, if doctrine hub-gating blocks it, or if the claim
  still carries an unresolved comment thread); also carries an engine-managed
  `review_pending` bool. `review_pending` is `true` while ANY of three
  independent triggers stands: a dependency's content has drifted since the
  claim was last locked or reaudited; a `dossierx claim flag` has recorded a
  spec mismatch; or the claim carries an unresolved (`status: open`) comment
  thread. It is set automatically but never cleared automatically — a locked
  claim's `status` never reverts to `draft` on its own, and `review_pending`
  clears only once EVERY trigger is gone, via one of three matching clearers: a
  human-confirmed `dossierx claim reaudit --confirm` (drift/flag), `dossierx
  claim unlock`, or resolving/deleting the last open comment thread in the
  viewer (while no drift or flag still stands). A claim cannot lock while it
  has an unresolved comment thread, and `reaudit` refuses a claim that is
  `review_pending` only because of an open thread (there is no content diff to
  confirm — resolve the thread instead). `reaudit` is the DRIFT tool: it
  rewrites only `body` and refuses a claim that is not already
  `review_pending`. Every other change to a locked claim goes through
  `unlock → fix → lock`. See the engine's `internal/lock`, `internal/reaudit`,
  and `internal/comments` packages for the full lifecycle.

## Edge types

A claim may reference other claims by `id` via three distinct kinds of
edge, each with a different meaning:

- **`mirrors`** — a deterministic equality edge. The target claims'
  comparable content must match this claim's exactly; if they diverge,
  that is a lint failure (`mirror-mismatch`), not merely staleness.
- **`rests_on`** — a semantic-consequence edge. This claim depends on the
  target claim remaining true, but is not required to be textually
  identical to it. When a `rests_on` target's content changes underneath a
  locked claim, the locked claim is flagged `review_pending` rather than
  invalidated outright.
- **`governed_by`** — names the doctrine claim (by id) that backs this
  claim's authority, or explicitly declares `type: none` with a required
  `reason` when no such doctrine claim exists. Note that `governed_by` is
  **not** itself a gated edge: when the project config sets
  `doctrine_facet`, hub-gating refuses to lock a claim whose **`mirrors`
  or `rests_on`** names an unlocked claim in that facet — those two lists
  are the whole of what it walks. A doctrine claim named *only* by
  `governed_by` is not gated, so to have hub-gating cover it, name it as a
  `rests_on` dependency as well. If `doctrine_facet` is unset, hub-gating
  does not run at all. What `governed_by` is checked for is that the
  authority chain terminates — see `governed-cycle` below.

### Graph invariants

Each edge kind is not just a per-claim field but a directed graph over the
whole claim set, and each of those three graphs has a shape it must hold to.
These are enforced by the lint suite, so a violation blocks `dossierx claim lock`
the same way any other error-severity finding does:

1. **`rests_on` must be acyclic.** It is a dependency edge, so a cycle means
   a set of claims each of which is true only if the others are — no claim
   in the loop can be reviewed on its own terms, and the drift propagation
   that flips dependents to `review_pending` has no order to run in. Every
   claim in the loop is reported by the `cycle` lint, with the cycle path in
   the message.
2. **`mirrors` must be a reciprocal 2-cycle.** Equality is symmetric, so if
   `A` mirrors `B` then `B` must mirror `A` back (`mirror-reciprocal`), the
   target must exist (`mirror-unanchored`), and the two claims' comparable
   content — `layout`, `body`, `rows`, `steps` — must actually match
   (`mirror-mismatch`). A one-directional `mirrors` edge is not a weaker
   equality claim; it is an unfinished one.
3. **`governed_by` must terminate.** Following `governed_by` from any claim
   has to reach `type: none` (with its required `reason`) in finitely many
   steps — that sentinel is the only grounded end state. A cycle in this
   graph means a set of claims whose authority rests only on each other,
   which is to say on nothing, and is reported by the `governed-cycle` lint.

Across all three, a claim may never name **its own id** in any edge
(`self-edge`). A self-edge is trivially satisfied by every content rule —
a claim always equals itself, always mirrors itself back, and always
resolves — so it asserts nothing while looking like a well-formed edge. An
edge is a statement about a *different* claim.

## Integrity invariants

Claim files are YAML in git, so nothing in this format can *prevent* a hand
edit. The invariants below are about something narrower and achievable: no
out-of-band edit of a **locked** claim is silent.

**DossierX detects; the forge enforces.** That division is the whole design, not
a caveat on it. Every rule below produces a *named finding* — a stable rule
string, the claim it is about, and the command that puts things back. Turning
that finding into something anyone has to obey is the job of branch protection
and a required CI check, which is why the ledger is worth having at all: it is
what makes a red check mean "this specific locked claim changed without an
approval" instead of "something, somewhere, is off". See
[What the gate detects, what it does not, and where the rest is caught](#what-the-gate-detects-what-it-does-not-and-where-the-rest-is-caught).

They are enforced by the **lock-ledger gate**, which is not part of the lint
suite. A lint failure is a statement about a claim's content; a ledger finding
is a statement about whether a human ever approved it, and the two must not
share a severity ladder — registering these as lints would let one tampered
file freeze locking project-wide and stop the viewer regenerating.

### The project-root stores are tracked artifacts

| File | Holds |
|---|---|
| `.dossierx-lock-store.json` | the lock ledger: per locked claim and per locked build-order artifact, `{hash, at, actor, reason}`, plus the dependency-drift baselines |
| `.dossierx-comment-digest.json` | a digest of each claim's comment block, as of the engine's last comment write |
| `.dossierx-flag-store.json` | each flagged claim's pending `claim flag` trigger: `{claim_says, now_does, reason, flagged_at}`, consumed and deleted by a confirmed `claim reaudit` |

All three live beside `project.config.yaml`. **Commit them; never `.gitignore`
them.** The gate compares the claims on disk against the first two, so a
project that does not track them has no gate — a claim and its approval have to
travel in the same commit for CI to be able to check either one. All three are
engine-written: hand-editing them is the same act as hand-editing a locked
claim, and it is visible in exactly the same way (the diff).

The flag store is not evidence and no finding reads it, which is why it is
listed here rather than under the findings below. It is still required to
travel with the claims: `claim flag` writes the before/after there and sets
`review_pending` on the claim itself, and `claim reaudit` reads it back to build
the diff it asks a human to confirm. A `review_pending` claim that arrives on a
machine without its flag entry reaudits to an EMPTY proposal, and confirming
that empty proposal clears `review_pending` having applied nothing — the one
state in which a trigger disappears with no edit and no record.

**It has no integrity coverage at all, and that gap is stated here rather than
left to be discovered.** The other two stores are each guarded from both sides —
the lock ledger by `lock-ledger-missing` / `-orphan` / `-released` / `-abandoned`
and `lock-content-drift`, the digest store by `comment-digest-absent` /
`-missing` / `-abandoned` and `comment-ledger-drift`. Nothing in the gate reads
`.dossierx-flag-store.json`, so **deleting it, `.gitignore`-ing it, or emptying
its map is undetectable**: `dossierx check` exits 0 over a project whose flag
entries are gone, and the human's recorded "the claim says X, the code does Y"
is erased with no finding, no warning, and nothing in any diff but the absence
of a file nobody is looking for. That is a smaller hole than it looks — a flag
is a request for review, not an approval, and erasing one cannot make a locked
claim change or make an unapproved claim read as approved — but it is a real
one. Until a rule covers it, the mitigation is procedural and belongs in review:
commit the store in the same commit as the claim it describes, and treat an
empty `reaudit` proposal on a `review_pending` claim as a missing flag entry
rather than as a claim that needs no change.

### What is signed, and what is not

A ledger record stores `LockedClaimHash` of the claim as approved. That hash is
a **deny-list over every persisted field** — it signs everything a claim
persists except three engine-managed fields:

- `status` — the gate notices a status flip by the presence or absence of a
  record, not by hashing the field. Hashing it would make a legitimate unlock
  read as tampering.
- `review_pending` — set automatically by the three triggers with no human in
  the loop; signing it would report drift every time the engine did its job.
- `comments` — written on every comment operation, including from `dossierx
  serve`, which deliberately has no write authority over the lock store.
  Comment integrity is covered by the separate digest file instead.

Everything else is signed, **including any field added to the schema later**.
This is deliberately not the same hash as the dependency-drift `ContentHash`,
which covers a hand-picked ten fields and must stay byte-identical forever:
`raw_html`, `raw_html_reviewed`, `build_role`, `kind`, `section`, `order`,
`emphasis`, `migrated_from`, and `audit_notes` are invisible to it — and
`raw_html` on a locked, reviewed, allowlisted mockup is the only path in the
engine that renders author bytes unescaped. A ledger built on `ContentHash`
would certify the one edit that most needs a signature.

### The findings

| Finding | The invariant it enforces |
|---|---|
| `lock-ledger-absent` | Locked claims exist, so the ledger file must exist. Deleting it is not a way to re-bless a project; it is a project-scoped refusal you fix by restoring the file from version control. |
| `lock-ledger-downgraded` | The lock store says it predates the ledger while the project around it proves otherwise — its `version` set back from `2` to `1` and the `ledger` key deleted, one hand edit to the audited file. **Read this as tamper evidence, not as a grandfathering guard.** It was written as the latter: adoption used to key on the store's own `version`, so this edit re-ran adoption and recorded whatever the claims said at that moment as approved, and the rule's job was to catch that with evidence the store does not own (a sibling `.dossierx-comment-digest.json`, or ledger records still sitting in a store claiming to predate records). Adoption is no longer automatic — see *Adoption, once* above — so the edit no longer buys approval and this rule is no longer load-bearing for that. It still fires, because a store lying about its own schema version is still a store somebody edited by hand, and the per-claim findings under it still stand. Restore the store from version control. Do **not** re-lock, and do **not** reach for `dossierx migrate --adopt`: on a covered project it refuses, and on an uncovered one it would record the current bytes as the baseline, which is what the downgrade was trying to achieve. |
| `lock-ledger-adoption-required` | A project that predates the lock ledger must be adopted deliberately before the gate will pass. This is **not** tampering and there is nothing wrong with the claims: the ledger simply does not exist yet, and adoption fails closed rather than manufacturing itself. One project-scoped finding naming the migration, deliberately in place of one `lock-ledger-missing` per claim — repeating "locked with no record" N times would attach a recovery (set it back to draft and re-lock) that is destructive advice at a project that has done nothing wrong. See *Adoption, once* below. Tell it apart from `lock-ledger-absent`, which means the project **had** a ledger and no longer does. |
| `lock-ledger-missing` | Every `locked` claim has an approval record. A `status:` flipped to `locked` by hand walks past the lint gate, hub-gating and the unresolved-comment gate as though all three had passed. |
| `lock-ledger-deleted` | A claim **this engine locked** still has its record. `lock-ledger-missing`'s sharper twin, and it exists because every other rule keyed on a record *existing*, so deleting one removed the claim from the switch entirely: drop its entry from the `ledger` map, flip `status: locked` to `draft`, and it is an ordinary draft — freely editable, and re-lockable afterwards with an agent-supplied `--reason` that produces a record indistinguishable from a human's. The evidence the deletion does not reach is one key away in the same file: `locked_at`, stamped by every lock and confirmed reaudit and removed by nothing in this build, plus the claim's dependency baselines under `hashes`. The only path that legitimately ends an approval is `unlock`, which **keeps** the record and stamps `ReleasedAt` — so a record that is absent rather than released was deleted by hand. Stated plainly: deleting `locked_at` and the baselines in the same edit leaves nothing to notice, which is three keys in a tracked file instead of one, in a diff whose purpose is to be read. |
| `lock-ledger-released` | A `locked` claim's record is a *standing* approval. Unlocking marks the record released rather than deleting it, so flipping `status:` back to `locked` by hand leaves a released record in place — which satisfies "a record exists" while recording the opposite of an approval, and passes the hash check because the hash deliberately excludes `status`. |
| `lock-content-drift` | A locked claim's content still hashes to what was approved. Covers every field above, including the ones `ContentHash` cannot see. |
| `lock-ledger-orphan` | A `draft` claim holds no *unreleased* record. Unlocking releases a record and keeps it; flipping `locked` back to `draft` by hand does not, and that is the cheapest way to dodge review. |
| `lock-ledger-abandoned` | An unreleased record still has the claim it approved. Deleting a locked claim's *file* removes the node from every per-claim rule below at once — they are all driven by the claims that exist — so removal was the one change to a locked claim that produced no finding at all. There is no `claim delete` verb: `unlock` first, then delete, so the withdrawal is on the record. |
| `comment-ledger-drift` | A claim's comment block matches the digest recorded at the last engine write. Deleting an unresolved thread by hand is how a claim would otherwise slip past the lock gate with a review still open. |
| `comment-digest-absent` | A ledger-covered project has the digest store the rule above compares against. A claim the store has never seen is *unknown*, never *drifted* — correct, since a gate must not manufacture a finding out of missing evidence, but it made the file a delete-to-clear switch: hand-delete an unresolved thread, delete `.dossierx-comment-digest.json` in the same commit, and the finding that named the edit was gone before any command ran. Project-scoped, and gated **only** on the project already being ledger-covered, so a project upgrading into the feature never sees it. |
| `comment-digest-unrecorded` | In a ledger-covered project, a claim **holding threads** has a digest entry beside them. The predicate is the threads themselves, which is what makes it survive the tamper: comments are engine-managed and the single path that writes a thread into a claim file records the claim's digest in the same act, so threads with no entry have exactly two explanations — the entry was removed, or the threads were never written by the engine — and both are the finding. Deliberately silent where the evidence is honestly absent: an uncovered project, an absent store (`comment-digest-absent` is that cause, said once), a claim with no threads, and a claim holding a *standing* approval (that one is `comment-digest-missing`, built on the ledger record instead — reporting both would name one state twice). |
| `comment-digest-missing` | The digest store is there, and a claim holding a **standing** approval record has no entry in it. The store was protected against deletion and not against being *emptied*, and overwriting it with `{"version":1,"digests":{}}` is strictly cheaper to hide in a review diff than the `rm` the rule above catches: hand-delete an unresolved `comments:` block and empty the map in one edit, and `claim lock` accepted the claim with a real, non-grandfathered record while `check --validate` reported ok. Coverage, not file presence, is the trigger, and the predicate is built only out of the ledger record — every approval writes the claim's comment digest in the same act that writes the record (`lock.RecordApproval`), so a standing record with no entry is a statement about the store, not about the claim. Silent where it should be: a project with no ledger coverage is not asked, an uncommented draft holds no record, and a released record describes a claim that has left the approval path. Suppressed entirely when the whole file is gone, so `comment-digest-absent` stays the single project-scoped cause. |
| `comment-digest-abandoned` | A digest entry that recorded review history still has the claim it recorded it for. This is the comment half's reverse sweep, symmetric with `lock-ledger-abandoned`, and it is what makes the **rename** launder visible: deleting a claim's `comments:` block alone fires `comment-ledger-drift`, but deleting the block *and* changing `id:` in the same edit went completely quiet — the claim the store knows no longer exists, the claim that exists is one the store has never seen, and `claim lock <new id>` then succeeded on a claim whose human review had been erased. The old id's entry survives that edit precisely because it is not reachable from the file the tamper rewrote. It does not fire on the two accounted-for departures — an entry that recorded no threads, and a claim whose record an honest `unlock` released — and `lock.SweepCommentDigests` drops those entries so they never accumulate. `lock.AbandonedCommentDigests` owns the predicate for both the rule and the sweep, so the gate and the sweep cannot disagree. |
| `build-order-content-drift` | A locked `.build-order.<module>.json` still matches the artifact that was approved. The sequence is what an implementing agent builds from, so reordering two phases by hand, moving a claim into `excluded`, or splicing the frozen `hashes` baseline so the order never reports `stale` again all change what gets built without changing any claim. |
| `build-order-ledger-missing` | A build-order artifact carrying `locked: true` has an approval record. `locked` in that file is a claim about a human's `--reason`, and a hand-set one is the same act as a hand-set `status: locked` on a claim. |
| `build-order-ledger-orphan` | An unlocked build-order artifact whose ledger record still **stands**, unreleased. This was the cheapest bypass in the gate: both build-order rules above skip an artifact carrying `locked: false` — correctly, since an unlocked artifact is a proposal nobody approved — so writing `false` removed the file from every rule's evidence at once while the approved sequence sat there for an agent to follow and the record still said a human approved it. The honest re-propose window is separated by the *release*, not by a guess: `build-order propose` releases the module's record as it overwrites the artifact, so an unlocked artifact under a released record is the documented flow and one under a standing record is not. The predicate therefore has no exception, and it catches a flag flip made together with a content edit — which the earlier, exact predicate (re-sign the artifact as if the flag were still `true`) could not, since a content edit re-signs to something else. |
| `build-order-ledger-abandoned` | An unreleased build-order record still has the artifact it approved. The two build-order rules above are both driven by the artifacts that exist, so deleting `.build-order.<module>.json` — or dropping the module from `modules:`, which stops anything auditing it — silenced them both at once and made removal strictly quieter than editing. It fires on *unreleased* records only, so a build order a human deliberately released stays silent. This is the build-order twin of `lock-ledger-abandoned`, and exists for the same reason. |
| `build-order-unreadable` | A build-order artifact that is *there* is legible. This is the build-order twin of `lock-ledger-unreadable`, and it closed the gap where corrupting the approved sequence was quieter than deleting it: deletion is caught by `build-order-ledger-abandoned`, but truncating the same file mid-token left it neither present (so the forward rules skipped it) nor absent (so the reverse sweep skipped it), and `check` exited 0 over a destroyed sequence. Its own rule because it is neither of the two: the artifact was not deleted, and its bytes cannot be compared to anything. Restore the file from version control — never re-propose, which records whatever the claims say **now** as the approved order. |
| `lock-ledger-unreadable` | The evidence itself is legible. A ledger that exists but does not parse fails closed and loudly, never quieter than a deleted one. |

`comment-digest-absent` is the comment half's answer to `lock-ledger-absent`,
and it is **narrower on purpose**. The lock ledger guards the trust boundary —
approved content, including the only unescaped-HTML render path in the engine —
so its file's absence is a flat refusal. The comment digest guards a
review-workflow gate, where a flat refusal would reject every project that
carries comments but has never written one through a build that had this store:
the read-only paths the hook and CI run (`check --staged`, `check --validate`)
never adopt, so those projects could not commit at all until someone commented.

So the trigger carries exactly one qualifier, and it is the one an attacker
cannot edit their way into:

- **The project must already be ledger-covered.** The digest store's absence
  cannot be keyed on the digest store's own history — the file whose absence is
  the question cannot also be the evidence — so it is keyed on the lock store's
  schema version (`lock.Store.LedgerCovered`). A project still on an older lock
  store is mid-upgrade and exempt. `lock.PrepareStore` **creates** the digest
  store, adopting every claim's current threads, at the same moment it stamps
  that version, so an upgrading project crosses both lines together and never
  sees the finding. That adoption never overwrites an existing store and is
  best-effort: a migration must not fail on it.

There used to be a second qualifier — *some claim must actually carry threads* —
and removing it is the point. It was computed from the very state the attacker
controls, which made the **total** launder free: delete a claim's only thread
*and* the digest store in one commit, and the thread count is zero, so the rule
whose whole job is to report the deleted store stayed silent about the deletion
that hid the deleted thread. A gate whose trigger is derived from the tampered
evidence is not a gate. The rule now fires on coverage alone.

That widening has a stated cost, and it is not a surprise: `check --staged`
reads both stores out of the git *index*, so a commit whose index carries a lock
ledger but no digest store beside it is now refused — a project that never
`git add`ed the file the engine already wrote. The finding names that recovery
explicitly (`git add` it, or restore it from version control), and it is one
command. An ordinary fully-committed project is unaffected.

The recovery is version control, not a re-run. Re-creating the store by running
a comment op would record whatever the claims say *now*, which is exactly what
the deletion was for — so `internal/comments` refuses to adopt wholesale in a
covered project. A comment write into a covered project whose store is gone
records only the claim it touched; every other claim stays *unknown* — never
blessed, never accused. `comment-ledger-drift` continues to cover the edit
itself whenever the digest is present.

Several of the rules above are not about a claim's *content*, and each exists
because "nothing already locked changes without an approval on the record" was
otherwise satisfiable by changing something other than a claim body.
`lock-ledger-abandoned` covers the node disappearing: every other per-claim rule
starts from the claims that exist, so `rm claims/foo.yaml` walked past all of
them at once and left an unreleased approval pointing at nothing.
`lock-ledger-downgraded` covers the *ledger* being edited instead of the claims,
which was the one bypass that lived entirely inside the file doing the checking.
The `build-order-*` rules cover the artifact an implementing agent actually
reads: `.build-order.<module>.json` is generated, but a **locked** one is
generated, approved and then frozen, and its `hashes` baseline is the only thing
that makes `stale` mean anything. A locked build order is checked against its
record for the same reason a locked claim is — the record is written by
`build-order lock`, and a record nothing ever reads is not a gate. Note that the
evidence set has to be closed from both ends: a rule keyed on `locked: true`
is disarmed by writing `false`, which is why `build-order-ledger-orphan` audits
the artifacts the forward rules skip.

Commit `.build-order.<module>.json` once it is locked, for the same reason you
commit the ledger: those rules read the artifact off disk, so an approved order
that never travels with the repository is an approval CI has nothing to compare
against. While it is still `locked: false` it is ordinary generated output that
`propose` rewrites in full.

### What the gate detects, what it does not, and where the rest is caught

**DossierX detects. The forge enforces.** The ledger's whole job is to turn a
silent edit into a **named, recoverable finding** — a rule string, the claim it
is about, and the command that puts things back. Nothing in this repository can
stop a commit from being written; a `chmod` and a text editor beat any local
tool. What a finding buys is that the edit cannot be *quiet*. Wire that finding
to a required status check on a protected branch and the quietness is gone for
good, because now the edit has to survive a human reading the diff that carries
it. Read every rule below as evidence production, not as enforcement.

That framing is what makes the boundary in this section a design decision rather
than a hole.

#### What IS detected

Every rule in this document judges **one tree** — these claim files, this lock
store, this digest store, these build-order artifacts, exactly as they are. That
is the whole evidence base. Within it, every edit that puts one artifact at odds
with another is a named finding — every edit to the **approved content** of a
locked claim, and every removal of any piece of the evidence around it. The
table below is that list, and it is meant to be read as a complete one. The next
section is deliberately not a list at all, for reasons it gives.

One field is deliberately outside that sentence. `review_pending` is engine-
managed bookkeeping, not approved content, so it sits in the locked-claim hash's
deny-list alongside `status` and `comments` (see `internal/lock/lockedhash.go`).
Deleting a `review_pending: true` line by hand therefore clears a standing review
flag without a finding. That is the flag store's business rather than the
ledger's, and the flag store has no integrity coverage — worth knowing before you
read the sentence above as covering the file byte for byte.

| The tampering | Named by |
|---|---|
| a locked claim's content edited — including `raw_html`, `build_role`, `section`, `order` | `lock-content-drift` |
| `status: draft` flipped to `locked` by hand, with no approval record | `lock-ledger-missing` |
| a record deleted from a claim this engine locked | `lock-ledger-deleted` |
| `status:` edited back to `locked` over a record `unlock` already released | `lock-ledger-released` |
| `locked` flipped back to `draft` while its record still stands | `lock-ledger-orphan` |
| a locked claim's **file deleted** while its record still stands | `lock-ledger-abandoned` |
| the whole lock store removed while locked claims remain | `lock-ledger-absent` |
| the lock store present but unparseable | `lock-ledger-unreadable` |
| the lock store's own `version` set back to pre-ledger | `lock-ledger-downgraded` |
| a pre-v0.3.0 project that never ran the migration | `lock-ledger-adoption-required` |
| a review thread edited or deleted outside the engine | `comment-ledger-drift` |
| the digest store removed from a covered project | `comment-digest-absent` |
| a standing approval whose digest entry was dropped from the map | `comment-digest-missing` |
| threads present on a claim with no digest entry beside them | `comment-digest-unrecorded` |
| a digest entry whose claim id was renamed out from under it | `comment-digest-abandoned` |
| a locked build order's sequence or frozen `hashes` edited | `build-order-content-drift` |
| a build order claiming `locked: true` with no record | `build-order-ledger-missing` |
| a locked build order's flag cleared to `false` while its record stands | `build-order-ledger-orphan` |
| a locked build order's artifact deleted, or its module dropped from `modules:` | `build-order-ledger-abandoned` |
| a build-order artifact present and undecodable | `build-order-unreadable` |

Note the shape of that table. Every artifact in the design is watched by rules
built out of the *other* artifacts, so removing any single piece of the evidence
is itself reported by a rule made of the pieces that remain — delete a claim and
its record accuses you, delete the record and the claim's `locked_at` and
baselines do, delete the whole store and the locked claims do. That mutual
defence is what the coverage above rests on, and it is worth being exact about
its predicate: a change is caught when it leaves a surviving artifact that
**disagrees** with the one that moved — not merely when it leaves some file
untouched. A record's `reason`, `at` and `actor` have nothing anywhere to
disagree with them, so editing those alone changes what the ledger says a human
approved and is reported by nothing, even though only one file was touched. That
is not a separate hole; it is the next section's principle showing through the
table, and the next section says once what the whole of it is.

#### What is NOT detected, and why nothing in the repository would fix it

**An in-repo ledger cannot attest anything against the person who can write it.**

That sentence is the boundary. Everything above is one tree judging itself: the
claim files, the lock store, the digest store and the build-order artifacts are
the entire evidence base, and every one of them is a tracked file in the
repository the committer is editing.

So the line does not fall between clever tampering and clumsy tampering. It falls
between **uncoordinated** and **coordinated** change:

- **Uncoordinated** — one file edited, one record removed, one status flipped,
  one artifact deleted, one thread erased. The files that were not touched
  disagree with the one that was, and the disagreement is the finding. This is
  what a drifting agent produces, what a careless hand-edit produces, and what a
  bad merge produces, which is why the table above is worth its length.
- **Coordinated** — a claim and its ledger record written *together*, in one
  commit, by someone entitled to write both. Nothing survives to disagree,
  because nothing was left out. `check` exits 0, and it is right to: there is no
  evidence in the tree that anything is wrong.

##### The worked example: the re-signed approval

One illustration, run end to end against a binary built from this tree. A locked
claim goes through the ordinary approval path — no deletions, no scope trick, no
missing file — and then one hand-edit falsifies what the ledger says about it:

```sh
dossierx claim unlock widget.contract.overview --reason "editing"
$EDITOR claims/widget-overview.yaml           # rewrite the body to say something else
dossierx claim lock   widget.contract.overview --reason "re-lock"
$EDITOR .dossierx-lock-store.json             # put the ORIGINAL reason, at and actor back
```

`dossierx check` → `ok: true`, zero findings. `dossierx check --staged` →
`ok: true`. The ledger now reads:

```json
"widget.contract.overview": {
  "subject": "claim",
  "hash": "fa0c1529…",
  "at": "2026-07-28T13:41:17.384943Z",
  "actor": "nitinkhanna",
  "reason": "Reviewed on the 2026-07-20 call; Nitin approved the wording."
}
```

— the original approval, at the original timestamp, standing over a body nobody
approved.

It passes because the re-lock did the hard part. `hash` is what the gate compares
against the claim, and after the re-lock it is the *correct* hash of the new
body. `reason`, `at` and `actor` are compared against nothing at all — they are
the record's **testimony, not its signature**, prose the engine records
faithfully and can never check, because there is nothing in the tree to check it
against. Nothing was erased, so none of the deletion rules above has anything to
fire on. The ledger is complete, internally consistent, and false.

**This is an example, not an enumeration.** Earlier drafts of this document tried
to list the arrangements that get past the gate; the list grew every time someone
looked harder, and two of its entries turned out to be wrong. Any change that
leaves the claim and its record agreeing has the property described above, and
the routes to that state are not worth counting. Read the principle, not a
catalogue.

##### Why no in-repo mechanism closes it

Because any evidence the tool consults lives in the repository, and the
repository is writable by the person being gated.

An earlier build of v0.3.0 tried exactly one thing outside the commit: `check
--staged` resolved `HEAD` and compared the staged tree against its parent. It was
removed before release, and the reasoning generalises to every replacement anyone
would reach for next.

**The parent commit is outside the *commit*, but it is not outside the
*committer*.** Git history is written by the party the gate exists to constrain,
so the comparison was not a check on them — it was a check they could also edit.
`--orphan`, a rebase, a second config file, a fresh repository: any one of them
switches such a comparison off, and none of them looks unusual in a log.

It also charged honest projects for a guarantee it could not deliver:

- **`git revert` of a commit that locked a claim was refused.** A legitimate
  revert removes that lock's records, and the resulting tree is byte-identical to
  the one an erasure produces. Git does not run `pre-commit` for `revert`, so the
  revert landed locally at rc 0 and only CI objected — the worst possible place to
  find out.
- **A project that was new in a commit** got audited against an unrelated retired
  project's ledger in a monorepo, and refused with findings naming another
  project's claim ids.

A control that a rebase switches off and a revert trips is not a control. It is a
coin flip with a rule name attached, and a rule name people learn to route around
is worse than no rule, because the next reader believes it.

The boundary is pinned rather than assumed:
[`internal/lock/audit_boundary_test.go`](internal/lock/audit_boundary_test.go)
holds it at the audit layer and
[`internal/check/staged_no_parent_test.go`](internal/check/staged_no_parent_test.go)
holds it end to end through `check --staged`, together with the revert and
monorepo cases above. Anything that later moves the boundary fails there loudly
and on purpose.

#### Where it IS caught

On the forge, which is where it always belonged. **DossierX detects; the forge
enforces.**

- **Branch protection with a required status check.** `dossierx check` on the
  merge result decides whether the pull request may merge. Every rule above runs
  there, on a tree nobody can rewrite after the fact. The shipped template
  ([`scripts/ci/dossierx-check.yml`](scripts/ci/dossierx-check.yml)) is one
  `dossierx check` step and pins no `fetch-depth`: a shallow checkout is a
  complete tree, which is the whole evidence base.
- **Review.** A coordinated change still has to arrive as a diff, and it arrives
  in a tracked JSON store whose entire stated purpose is to be read in one,
  sitting on the same page as the claim change it was made to permit. The claim
  half is as loud as any other rewrite of an approved fact. The store half is one
  line, and it has a signature worth knowing, because in the worked example above
  the whole diff is this:

  ```diff
  -      "hash": "ba4835bc…",
  +      "hash": "e9bdf091…",
         "at": "2026-07-28T13:51:55.599082Z",
         "actor": "nitinkhanna",
         "reason": "Reviewed on the 2026-07-20 call; Nitin approved the wording.",
  ```

  **The hash moved and the approval did not.** Every honest lock writes `hash`,
  `at`, `actor` and `reason` in the same act, so a record whose content changed
  while its timestamp, actor and reason stood still did not come from a lock this
  engine performed. That is a rule a reviewer can apply without knowing anything
  else about the change.
- **`CODEOWNERS` on the stores and the config**, if you want that reading to be a
  specific pair of eyes rather than whoever is on rotation.

The trade is honest and it is the right way round. DossierX names every tampering
a tree can prove, in a form CI can fail on, identically in every clone — and it
declines to adjudicate the history the committer writes. A red check means
something precise because the finding under it is specific, reproducible from the
committed tree, and true wherever that tree is checked out.

#### If you need more than a repository can prove

Sign the ledger with a key held **outside** the repository. That is the one move
that changes the principle rather than working around it: an attestation the
committer cannot mint is an attestation that survives the committer. Git's own
commit signing is the cheapest form of it available today, and pointing branch
protection at signed commits costs nothing to adopt.

This is noted as the direction that would actually move the boundary. It is not a
promise, and nothing in the format reserves space for it.

#### Moving `claims_dir`

Because there is no scope rule any more, there is no scope ceremony either. Move
the claim files, edit `claims_dir:`, stage the claims, the config and the
unchanged stores together, and commit. A legitimate move passes because every
locked claim is still reachable and still hashes to its existing record — which
is what the rules were reading all along.

A move that **strands** locked claims still fails, from state alone: their
records are left pointing at claims the project can no longer see, which is
`lock-ledger-abandoned`, once per claim. What is gone is only the ability to
catch that stranding when the ledger holding those records was deleted in the
same change.

There is still **no flag, environment variable or config marker** that exempts
anything here, and there never will be: an escape hatch on an integrity gate is
the attack, because the party who reliably remembers to set it is the one who
went looking for it in the source.

### Adoption, once, and only when a human asks for it

A project that locked claims before the ledger existed is **not** adopted by any
`check`. It is adopted by one explicit command:

```sh
dossierx migrate --adopt --dry-run             # names every artifact it would adopt
dossierx migrate --adopt
```

`--adopt` is required, so a bare `dossierx migrate` refuses with `missing_flag`
rather than guessing at a migration nobody named. There is deliberately **no
`--reason`**: every other record-writing verb takes one because a human approved
something, and nobody approved this. Each record the migration writes carries a
fixed reason saying so, plus `grandfathered: true`, permanently — a
human-supplied reason would make an adoption read like an approval in a ledger
diff, which is the single thing this command must not do.

Then **commit the rewritten `.dossierx-lock-store.json`** with the claims it now
covers. Until that commit lands, every CI run and every hook run starts from the
un-adopted store again.

**Adoption fails closed.** A missing or unreadable ledger never adopts itself, in
any run — not plain `check`, not `--validate`, not `--staged`. This is a
deliberate breaking change for every v0.2.x project, and the reasoning is worth
keeping next to the format it constrains.

Adoption is the one operation in this design that *manufactures approval out of
nothing*: it takes bytes nobody reviewed and writes a record saying they are the
approved baseline. Any rule that performs it automatically therefore hands an
attacker the same primitive the gate exists to deny — "arrive with no ledger"
becomes a universal bypass, and `rm .dossierx-lock-store.json` is rewarded with a
clean report. The obvious repair is a predicate that tells an honest pre-ledger
store from a downgraded one, and that repair does not exist: `locked_at` shipped
in v0.2.0 (`git show v0.2.0:internal/lock/lock.go`), so no field, no timestamp
and no sibling file distinguishes the two from inside the directory. Earlier
builds tried anyway and produced `lock-ledger-downgraded`, which narrowed the
window without closing it. When no predicate can be trusted, the answer is not a
cleverer predicate — it is to stop guessing and require a human decision.

Each adopted record is marked as adopted **permanently**, because an adopted hash
is content that was *observed*, not content anyone approved; re-lock those claims
deliberately when you get to them. Claims and already-locked build orders are
adopted in the same act, since a project half-across the ledger line is a state
with no honest reading. The migration changes no claim's `status`, resolves no
thread and clears no `review_pending`.

It is an upgrade step and never a recovery tool. On a project that already has
ledger coverage it refuses, and reaching for it to silence a gate on a covered
project would record tampered bytes as approved — exactly the outcome fail-closed
exists to prevent.

## Project config (`project.config.yaml`)

```yaml
schema_version: 1              # engine refuses to run on an unknown version
title: string                    # optional; viewer <title>, header, and
                                   # sidebar heading. Falls back to a generic
                                   # "dossierx viewer" default when unset.
eyebrow: string                  # optional one-line subtitle rendered under
                                   # the sidebar heading (e.g. "user-intelligence
                                   # service"). No fallback — unset renders no
                                   # eyebrow element at all.
facets: [string, ...]           # non-empty, no duplicates
modules: [string, ...]          # non-empty, no duplicates
claims_dir: path                 # resolved relative to this file's own directory
                                  # (directory layout inside it is not part of
                                  # this spec — see "Directory layout" below)
doctrine_facet: string           # optional; omitted disables hub-gating entirely
viewer:
  template_overrides: path        # optional override dir; resolved relative
                                    # to this file's own directory. Eligible
                                    # for override, by filename, inside it:
                                    # the 7 per-layout component partials
                                    # (card.html, table.html, list.html,
                                    # steps.html, tree.html, banner.html,
                                    # mockup.html), the Build Order partial
                                    # (build_order.html), plus the outer shell
                                    # (shell.html) and base stylesheet
                                    # (style.css). Missing
                                    # individual files inside it fall back
                                    # to engine defaults per-file; a
                                    # configured-and-missing directory itself
                                    # is a hard load-time error.
  theme:                          # optional CSS token overrides; see below.
    accent: "#3fb950"
```

All paths in this file are resolved relative to the config file's own
location, never the process's current working directory — this is what
lets the same engine binary be pointed at a config file from anywhere.

### Directory layout is not part of this spec

`claims_dir`'s internal structure — subdirectory names, nesting depth,
how files are grouped on disk — carries no meaning to the engine and is
entirely the claim author's choice. `internal/loader.LoadClaims` walks
`claims_dir` recursively and loads every `*.yaml`/`*.yml` file it finds,
matched purely by file extension; it does no filename or path-segment
parsing of any kind. A claim's `module` and `facet` come only from that
claim's own YAML fields (`module:`, `facet:`), never from where the file
happens to live on disk. This means a project can reorganize its
`claims_dir` freely — flatten it, rename subdirectories, move files
between them — without touching claim content or breaking anything the
engine reads.

#### Recommended authoring convention (non-enforced)

Although the engine attaches no meaning to directory structure, a
consistent layout still helps humans navigate a large claims tree. This
convention is a suggestion, not a rule — the engine does not check for
it, and a project is free to organize `claims_dir` differently:

```
claims_dir/
  <module>/
    <facet>/
      <topic-slug>.yaml
```

e.g. `claims/widget/contract/severity-policy.yaml`. Grouping by
`module` then `facet` mirrors the two-level grouping the viewer already
renders claims under, so the on-disk layout roughly matches what a
reader sees — but this is purely for the humans editing the claims
tree. If a project wants in-content section headings (as opposed to
just a tidy directory tree), use the `section` field described above;
the engine does not infer one from directory naming.

### `viewer.theme`

`viewer.theme` maps a fixed set of CSS custom-property token names (without
the leading `--`) to their values, letting a project restyle the shipped
viewer's colors, fonts, and corner radius without writing any CSS or
touching `viewer.template_overrides`.

The allowlist is intentionally fixed and engine-owned — an unrecognized key
is a load-time error (typo protection), not a silently-ignored field:

```
accent, accent-bg, ink, muted, faint, paper, card-bg, border, link,
warn, warn-bg, font-sans, font-mono, radius
```

Rules:

- Every present key's value must be non-empty.
- No value may contain `;`, `{`, `}`, `<`, or `>` — theme values are
  interpolated verbatim into a generated `<style>` block, so this is
  rejected outright as the actual injection-safety concern, independent of
  whether the value is otherwise a valid color or font-family.
- The color-shaped keys (every key above except `font-sans`, `font-mono`,
  and `radius`) additionally get a light format sanity check: the value
  must look like `#hex` (3/4/6/8 digits), `rgb(...)`/`rgba(...)`, or a bare
  CSS named color (e.g. `forestgreen`).
- A project may set none, some, or all of the allowlisted keys. Any key
  left unset keeps the engine's own default for that token (see
  `internal/render/viewer/template/style.css`'s `:root` block) — this is a
  per-token fallback, not an all-or-nothing swap, and it holds in both the
  light and dark `prefers-color-scheme` variants of the default stylesheet.

`viewer.theme` and `viewer.template_overrides` are orthogonal: a project
can use either, both, or neither. Theme tokens are plain CSS custom
properties, so they cascade into markup produced by any template —
including markup from a fully custom `template_overrides` partial, shell,
or stylesheet — with no extra wiring required on the override's part.

#### Mode-invariant vs. mode-varying tokens

The allowlisted tokens split into two groups, and the two groups should
usually be treated differently by a project's config:

- **Mode-invariant** — `accent`, `accent-bg`, `link`, `warn`, `warn-bg`,
  `font-sans`, `font-mono`, `radius`. These don't change between light and
  dark mode in the shipped default stylesheet (a brand color, a link color,
  and typography/radius choices read the same regardless of OS theme), so
  it's safe for a project to always set these — there is no light/dark
  variant to accidentally clobber.
- **Mode-varying** — `ink`, `muted`, `faint`, `paper`, `card-bg`, `border`.
  These are the ones `style.css`'s `@media (prefers-color-scheme: light)`
  block flips to different values than its dark defaults. The engine emits
  every configured token into one unconditional `<style>:root{...}</style>`
  block, and an unconditional rule always wins over a `@media`-scoped one —
  so setting any of these six pins the viewer to a single mode for every
  visitor, permanently overriding their OS/browser preference, regardless
  of which value you picked.

A project that deliberately wants to force one mode (e.g. a dark-only
brand site) can still set some or all of the six mode-varying tokens — that
is a supported, intentional use of the allowlist. But it should be a
deliberate choice: leave them unset by default and let the engine's own
`@media` defaults handle both OS themes, and only set them when forcing a
single mode is actually the goal. Setting all fourteen tokens out of habit
(e.g. by copying a fixed dark palette wholesale into `viewer.theme`) is the
most common way this trap gets hit by accident.
