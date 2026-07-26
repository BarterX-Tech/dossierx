---
name: dossierx-comments
description: >-
  Workflow for the threaded review discussion DossierX attaches to a claim —
  "comments on claims", the Google-Docs-style dialogue layer that sits beside
  the lock/reaudit lifecycle. Use this WHENEVER you are about to comment on a
  claim, reply to or resolve a thread, or decide whether a concern is a
  comment (discussion, no proposed edit) or a dossierx flag (a proposed
  content edit that feeds reaudit). Covers the comment-vs-flag discriminator,
  the advisory-rights rule (an agent never resolves a human-opened thread),
  how an open thread gates locking and sets review_pending, why reaudit
  refuses a comment-only claim, and the check-driven loop for clearing
  threads before locking. Requires the dossierx-claims skill's claim/lock
  basics first.
---

# dossierx-v1 Comments — review discussion on claims

`dossierx comment` attaches threaded, Google-Docs-style discussion to a claim so a human
and an agent can talk *about* a claim without editing it. A comment is dialogue; it is not
a change to the claim. This skill is about when to reach for it (versus `dossierx flag`),
who is allowed to resolve what, and how an open thread interacts with the lock lifecycle.

Read **[[dossierx-claims]]** first — this skill assumes you already know the claim schema
and the lock/`review_pending`/reaudit lifecycle. Comments plug a third trigger into that
same lifecycle.

## When to use this

- You have a question, concern, or observation about a claim that needs a human's judgment
  but is **not** a specific proposed rewording of the claim.
- A human left a comment thread on a claim you're working from and you've addressed it in
  code or in discussion.
- `dossierx check`'s next-steps block, or `dossierx comment list --open`, shows open threads
  standing between a module and being lockable/build-order-ready.
- You're deciding between opening a comment and running `dossierx flag` (see the next
  section — this is the most common reason to open this skill).

## Comment vs. flag — the one discriminator

Both a comment and a `dossierx flag` say "a human needs to look at this claim," so it is easy
to reach for the wrong one. The boundary is crisp:

**Is there a specific proposed wording change to the claim's body?**

- **Yes → `dossierx flag`** (see **[[dossierx-code-links]]**). A flag is for exactly one
  situation: a *locked* claim whose stated meaning has drifted from what the code now does.
  It carries a proposed content edit (`--claim-says` / `--now-does`), flips the claim to
  `review_pending`, and feeds the reaudit diff — a human confirms the rewrite, and the claim
  text actually changes. Use it only when you can state the before/after.
- **No → `dossierx comment`.** A comment is any discussion or remark that needs human
  dialogue but carries **no** content edit: "is this still true?", "why was it done this
  way?", "I think this rests_on the wrong module — thoughts?", "reviewed, looks right." The
  thread is the deliverable; the claim body is never touched by resolving it.

If you find yourself writing a flag whose `--now-does` you can't actually fill in — you only
have a *question* about whether the claim is right — that's a comment, not a flag. Conversely,
don't bury a concrete "this line should say X instead of Y" in a comment thread where it
can't feed reaudit; flag it so the fix is reviewable as a diff.

## The verbs — and always go through the CLI

```
dossierx comment add    <claim-id>            --as human|agent --body "..."   # open a thread
dossierx comment reply  <claim-id> <thread-id> --as human|agent --body "..."  # reply to an open thread
dossierx comment resolve <claim-id> <thread-id> --as human|agent              # mark a thread resolved
dossierx comment reopen  <claim-id> <thread-id> --as human|agent              # reopen a resolved thread
dossierx comment edit    <claim-id> <thread-id> --as human|agent --body "..." [--reply <reply-id>]
dossierx comment delete  <claim-id> <thread-id> --as human|agent            [--reply <reply-id>]
dossierx comment list    <claim-id> [--open] [--json]                         # read threads
```

`--as human|agent` is required on every mutating verb: it records the actor's **role** (not
an identity) on the message and is what the advisory-rights rule below keys off. Each verb
echoes the minted/affected id so you can chain the next one, and reminds you to run
`dossierx check` or `dossierx serve` to see the change rendered.

`dossierx comment list` (without `--json`) prints one pinned line per thread:

```
<thread-id> <status> <author> <created> replies=<N>: <body-first-line>
```

**Always mutate comments through these verbs — never hand-edit the `comments:` block in a
claim file.** The verbs take the project-wide claims lock, re-read the claim fresh inside it,
and write exactly one claim back. This matters especially because a human may be reviewing
live in `dossierx serve` at the same moment: the serve HTTP API and these CLI verbs are the
*same* code path and the *same* lock, so a CLI mutation and a browser mutation can't clobber
each other. A raw text edit bypasses the lock and can lose a comment a human just posted.

## Advisory rights — an agent never resolves a human's thread

Rights are **advisory** (a coordination convention for a local single-user tool, not
authentication — `--as` is asserted, not verified), but honor them strictly:

- **A human may act on anything.**
- **An agent may resolve / reopen / edit / delete only its own (agent-authored) messages.**
  Equivalently: a human-opened thread — or a human-authored reply — can be resolved, reopened,
  edited, or deleted **only by a human**; an agent-opened one by either.
- **Replying is ungated** — anyone may reply to any open thread. That is the agent's tool.

So when a **human** opens a thread and you (as `--as agent`) believe you've addressed it:
**do not resolve it.** Reply on the thread — "addressed in <where>, please confirm" — and
**wait** for the human to resolve it themselves. Resolving a human's thread yourself
silently declares their concern closed on their behalf, which is exactly the judgment the
comment layer exists to keep with the human. Only resolve threads you opened as the agent.

## How an open thread gates the lock lifecycle

An open comment thread is the **third** `review_pending` trigger, alongside dependency drift
and `dossierx flag` (see **[[dossierx-claims]]**). Two rules follow:

- **Lock gate.** A claim **cannot be locked while it has an unresolved (open) comment
  thread** — `dossierx lock` refuses it and names the blocking thread ids. Resolve the
  thread(s) first, then lock. (`dossierx build-order propose` enforces the same gate for a
  whole module: it refuses while any module claim carries an open thread — see
  **[[dossierx-build-order]]**.)
- **Pending on a locked claim.** Adding a thread to an already-`locked` claim sets its
  `review_pending`. It clears when the last open thread is resolved/deleted — but only if no
  *other* trigger (drift or flag) still stands. If drift or a flag is also present, resolving
  the thread leaves `review_pending` set until that trigger is cleared its own way.

**`dossierx reaudit` refuses a comment-only `review_pending` claim.** Reaudit exists to
review a proposed *content diff*; an open thread carries no diff, so there is nothing to
confirm. Reaudit exits non-zero and tells you to resolve the thread instead:

```
reaudit: claim "<id>" is review_pending only because of N open comment thread(s);
resolve them with "dossierx comment resolve <id> <thread-id>" — nothing to reaudit
```

Don't try to route a comment through reaudit or clear `review_pending` by hand — resolve the
thread through the CLI and the flag clears itself.

## The loop — let `dossierx check` drive which threads to clear

`dossierx check` is the one command to run routinely (see **[[dossierx-claims]]**). Its
non-blocking output already tells you exactly which threads stand between you and a lock:

- A per-module summary line — `open comments: module "<name>": N` — for every module that
  has any open threads.
- A next-steps hint naming the work and the exact command to do it:

  ```
  N claim(s) with open comment thread(s) -> dossierx comment resolve <id> <thread-id> (e.g. <id> <thread-id>)
  ```

  This hint is partitioned by trigger: a claim that is `review_pending` from a comment gets
  this comment hint (not the reaudit hint), and a claim pending on *both* a comment and
  drift/flag is told to address the comment first.

So the loop before locking a module is: `dossierx check` → work the open-comment next-steps
(reply/resolve, honoring the rights rule above) → re-run `dossierx check` until no open-thread
steps remain → then `dossierx lock` / `dossierx build-order propose`. Never lock "around" an
open thread; the gate is there so a claim is never frozen mid-conversation.

## Portability note

Comments add no project-specific configuration. The `comments:` field is engine-managed
bookkeeping on every claim (excluded from the content hash, `omitempty` so a never-commented
claim is byte-identical to before the feature existed), and the verbs, the advisory-rights
rule, and the lock gate operate purely on that field plus the `--as` role — nothing read from
`project.config.yaml`. Like the other three skills, this has been verified against an
unrelated synthetic project with different facets and modules, needing zero code changes.
