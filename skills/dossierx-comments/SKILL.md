---
name: dossierx-comments
description: >-
  The review-discussion loop between a human reading the DossierX viewer and
  the agent operating the CLI. Use this WHENEVER a human says they left
  comments, whenever you run dossierx comment inbox, whenever you are about to
  reply to a thread on a claim, and whenever you are deciding between opening a
  comment (a question, no proposed wording) and dossierx claim flag (a concrete
  before/after on a locked claim). Covers the inbox contract and its cursor,
  the advisory-rights rule an agent may never work around (you reply, the human
  resolves), how an open thread blocks locking, and why reaudit refuses a
  comment-only claim. Load the DossierX router skill first.
---

# DossierX comments — the human's half of the loop

Read **[[dossierx]]** first for the envelope, exit codes and error codes.

A comment is a threaded conversation attached to a claim. It is **dialogue about** a claim, never
a change to one. It is also the only channel in this design that runs from the human to you, so
treat an unanswered thread as a blocked task, not as background noise.

## The whole loop

| step | who | what |
|---|---|---|
| 1 | human | reads a card in the viewer, disagrees, opens a thread on it |
| 2 | human | tells you, in chat, "I left comments" |
| 3 | **you** | `dossierx comment inbox` — every open thread in the project, one call |
| 4 | **you** | fix the claim if it is draft (or take a locked one through unlock → fix → lock, with their yes), then `dossierx comment reply <claim-id> <thread-id> --as agent --body "..."` |
| 5 | human | clicks **Resolve** in the viewer. That click is their approval, and it is what unblocks locking |
| 6 | human | "good, lock it" |
| 7 | **you** | resolve their words to an id, `--dry-run`, show it, get a yes, `dossierx claim lock <id> --reason "<their words>"` |

Step 5 is theirs and only theirs. There is no CLI verb for it — `comment resolve`, `reopen`,
`edit` and `delete` are viewer-only in v0.3.0. If your plan contains "then I resolve the thread",
the plan is wrong.

## The verbs

```
dossierx comment inbox [--since <RFC3339>]              # project-wide open threads, oldest activity first
dossierx comment list <claim-id> [--open]               # one claim's threads
dossierx comment add   <claim-id> --as human|agent --body "..."
dossierx comment reply <claim-id> <thread-id> --as human|agent --body "..."
```

`--as` is required on every mutating verb and records a **role**, not an identity. Never pass
`--as human` for something you decided; the rights rule below keys off it, and mislabelling
yourself is how an agent ends up approving its own work.

Never hand-edit the `comments:` block in a claim file. The verbs and the viewer's API are the same
code path behind the same project-wide lock, so a CLI write and a browser write cannot clobber
each other. A raw text edit bypasses the lock and can destroy a comment the human just posted —
and the ledger reports it as `integrity_failed`.

## `dossierx comment inbox`

```json
{"ok": true, "command": "comment inbox", "data": {
  "cursor": "2026-07-26T21:15:37Z", "count": 1, "claims": 1,
  "threads": [{
    "claim_id": "widget.contract.retry-policy", "claim_title": "Retry Policy",
    "module": "widget", "facet": "contract", "claim_status": "draft",
    "thread_id": "c-b98f8b", "author": "human", "created": "2026-07-26T21:15:37Z",
    "body": "Is three retries right?", "replies": 0,
    "last_activity": "2026-07-26T21:15:37Z", "last_author": "human",
    "agent_can_resolve": false, "agent_has_replied": false}]}}
```

Three fields do the work:

- **`agent_can_resolve`** — the rights rule, as data. `false` means a human owns this thread and
  you may only reply. Read the field; do not try to remember who authored what.
- **`agent_has_replied`** — `false` on a human-authored thread is your queue. That is the work.
- **`cursor`** — echo it back as the next call's `--since`. It is **inclusive** of its own second:
  comment timestamps have one-second resolution, so an exclusive cursor would silently drop
  anything landing in the boundary second. Re-reporting a thread you have already seen costs you
  nothing; missing the human's comment breaks the loop.

## Advisory rights — you reply, you never resolve

Rights are advisory (`--as` is asserted, not authenticated) and enforced in code as
`rights_denied`. Honor them strictly, and never "retry as human" to get past one.

| actor | may act on |
|---|---|
| human | anything |
| agent | only agent-authored messages |
| anyone | **reply** to any open thread — this is your tool |

When a human opens a thread and you believe you have addressed it: reply — "addressed in
`internal/foo/bar.go`, please confirm" — and **wait**. Resolving it yourself would declare their
concern closed on their behalf, which is exactly the judgment the comment layer exists to keep
with them. It would also forge the approval that the lock gate is waiting for.

## Comment or `dossierx claim flag`? One question

**Can you state a specific before/after for the claim's wording?**

- **Yes → `dossierx claim flag`** (see **[[dossierx-code-links]]**). Only for a **locked** claim
  whose stated meaning has drifted from what the code now does. It carries `--claim-says` /
  `--now-does` / `--reason`, sets `review_pending`, and feeds the reaudit diff.
- **No → a comment.** "Is this still true?", "why was it done this way?", "I think this rests_on
  the wrong module." The thread is the deliverable; no claim text changes when it resolves.

If you find yourself unable to fill in `--now-does`, you have a question, not a flag. Conversely,
do not bury a concrete "this line should say X instead of Y" in a thread where it cannot feed a
reviewable diff.

## How an open thread gates the lifecycle

- **Lock gate.** A claim cannot be locked while it carries an open thread — `dossierx claim lock`
  refuses with `unresolved_comments` and names the blocking thread ids. `dossierx build-order
  propose` enforces the same gate across a whole module.
- **Third `review_pending` trigger.** Adding a thread to an already-locked claim sets
  `review_pending`. It clears when the last open thread is resolved — but only if no *other*
  trigger (dependency drift, a flag) still stands.
- **Reaudit refuses a comment-only claim.** There is no content diff to confirm, so it exits
  non-zero and tells you to clear the conversation instead. Do not route a comment through
  reaudit, and never clear `review_pending` by hand.

The loop before locking anything is therefore: `dossierx check` → work the open threads (reply,
never resolve) → the human resolves → re-run `dossierx check` until nothing is open → then lock.
`dossierx check` reports `open comments: module "<name>": N` per module and names the blocked
claims in its next-steps block, and `dossierx claim show <id>` reports the open thread ids for one
claim.

## Portability

Comments add no configuration. The `comments:` field is engine-managed bookkeeping on every claim,
`omitempty` and excluded from a claim's content hash — so commenting never rewrites an
uncommented claim or flips its dependents to `review_pending` by accident.
