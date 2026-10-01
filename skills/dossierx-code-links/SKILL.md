---
name: dossierx-code-links
description: >-
  Implementing code from a locked DossierX module, grounding it in the claims it
  implements, and what to do when a later code change means a locked claim is no
  longer true. Use this WHENEVER you are about to implement code from locked claims,
  WHENEVER you finish implementing or modifying code against a locked claim,
  whenever you add a "dossierx-claim: <id>" or "dossierx-step: <id> #<n> <hash>"
  comment to source, whenever dossierx check reports a drifted or unlinked claim
  or a conformance result, whenever you declare a claim's embodiment, and whenever
  a maintenance change makes a claim's stated behavior stop matching reality.
  Covers the reading order for implementation, the dossierx-claim and
  dossierx-step tags scanned by dossierx check, dossierx claim link for the cases
  scanning cannot reach, the code-link gate, embodiment and conformance checks,
  the comment / flag / unlock / re-tag decision, and dossierx claim flag. Load the
  DossierX router skill first, and dossierx-claims for the lock basics.
---

# DossierX code links — grounding claims in real code

Read **[`dossierx`](../dossierx/SKILL.md)** for the envelope and error codes, and
**[`dossierx-claims`](../dossierx-claims/SKILL.md)** for the lock lifecycle.

Two channels close the loop from spec back to code, and they never mix:

| | Grounding correct code | The spec is wrong |
|---|---|---|
| About | where a still-correct claim lives in code | the locked claim itself needs revisiting |
| Gate | **yours alone**, no human step | the human, via `claim flag` → reaudit, or unlock → fix → lock |
| Trigger | code finished, or a linked file moved with identical meaning | the code's *meaning* changed relative to what the claim states |

## Implement from a locked module, then tag

1. **Read the module, not the tree.** `dossierx manifest show <module> --isolation` gives the
   constitution, the project claims index, the manifest and every claim's summary. Implement from
   **locked** claims; a draft can still change under you.
2. **Follow the edges.** The manifest's `depends_on` names the neighbor contracts this module
   consumes (`manifest show <module> --integration` gives their summaries). A claim's `rests_on`
   (`dossierx claim show <id>`) names what must already hold for it to be true: build or confirm
   those first. Open a body with `claim show` only when the summary is not enough.
3. **Tag as you finish each claim** (below), then run plain `dossierx check`.
4. **When the code cannot honor a claim**, stop: that is a comment or a flag for the human, never a
   quiet deviation and never a tag on code that does something else.

## Tag it — `dossierx check` does the rest

1. Right after finishing a claim's code (or, for a claim a test proves, that test), put a comment
   next to the function or type. Any comment syntax works — the scan looks for the literal marker:

   ```python
   # dossierx-claim: widget.internals.queue-saturation-policy
   def _drop_for_saturation(self):
       ...
   ```

   For a claim that carries `steps:`, pin each implemented step with its **1-based** index and the
   sha256-hex of **that step's YAML string** (not the source file):

   ```python
   # dossierx-step: widget.contract.walkthrough #2 <64-char-sha256>
   def _step_two():
       ...
   ```

   A bare `dossierx-step: <id>` with no `#n` and hash is a scan error. Both markers may appear on
   the same claim; each adds a link. A claim may have any number of tagged files, and a file may
   carry tags for any number of claims.

2. Run `dossierx check`. When `project.config.yaml` sets `source_dirs`, the scan finds the tag and
   links it — no separate command.
3. An invalid tag is a **hard failure**: an unknown id, a claim that is not locked, `#n` out of range,
   a claim with no `steps`, or a hash that does not match makes `dossierx check` exit 1
   (`implink_refused`, `stopped_at: scan`) and name the file and line in `data.scan_errors[]`. **One
   of those is not a bad tag.** While you hold a claim open — `claim unlock` … fix … `claim lock` —
   its correct tags name a `draft` claim, so every `check` in between fails `claim is not locked
   (status "draft")`. That is the unlock you asked for, mid-flight: finish the re-lock and the same
   `check` goes green. Never delete or retarget the tag, and never leave the claim unlocked to keep
   `check` quiet — `--validate` and `--staged` scan no source, so they stay green while the rebuild
   you need is the thing failing.
4. The `#symbol` a reader sees later is a best-effort guess from the declaration below the tag; the
   file-level link is what counts.

When there is no `source_dirs`, or the file cannot carry a comment (a generated artifact), link it
explicitly — same validation, same record:

```
dossierx claim link --module <name> --claim <id> --file <project-relative-path> [--symbol <name>]
```

It takes `--dry-run`, needs no `--reason` (it records a fact, it approves nothing), and refuses a
claim that is not locked (`not_locked`). Both paths write `build/code-links/<module>.json`, a
committed artifact — never hand-edit it.

## The code-link gate

With `source_dirs` set, plain `dossierx check` refuses `unlinked_claims` (`stopped_at: links`) while
any locked module claim has no linked file, or a stepped claim is not tagged on every step (a
`dossierx-claim:` tag on a stepped claim counts as 0 of N). Project claims are exempt. The catalog and
viewer are rebuilt before the refusal; `data.code_links.modules[]` names each `unlinked` and
`partial` claim. `--validate` and `--staged` report `code_links` with `gated: false` and never refuse,
so a green there is not a linked green.

Recover by tagging the real code. **Never tag an unrelated file to clear it.** A locked claim with
genuinely no code behind it is the human's call: on their yes, unlock → add
`embodiment: {mode: none, reason: "…"}` → lock with their `--reason` (or it becomes a project claim).

Linked is not followed: the gate proves a pointer exists, never that the code still means what the
claim says. Nor is it your saying so: "the code matches the claim" in chat is not a certificate and closes nothing. `check` also counts drift (a linked
file changed since it was linked), and `claim show <id>` reports it per file:

```json
"implemented_in": [{"file": "internal/widget/queue.go", "symbol": "dropForSaturation", "drifted": true, "step": 2, "step_hash": "..."}]
```

## Embodiment and conformance — checks the project's own tools observe

A claim may declare what its implementation must show, so the project's own tooling can compare it:

```yaml
embodiment:
  mode: compare
  checks:
    - id: public-states                # unique within the claim
      adapter: source-symbols/v1       # opaque to DossierX: names the project's tool
      target: source://widget/state    # opaque: what that tool looks at
      expectation: {shape: set, value: [blocked, ready, waiting]}
    - id: schema-version
      adapter: schema-metadata/v1
      target: schema://widget/record
      expectation: {shape: scalar, value: "3"}
```

Every value is a string; DossierX never interprets versions, units or numbers. A project-owned
adapter writes one observation file (the path in `conformance.observations`); **DossierX never runs
the adapter** — producing that file is the project's job. Each check comes out `matched`, `owed`
(no observation for that adapter and target), `mismatch` (with sorted `missing` / `extra`, or
`expected` / `observed`) or `uncheckable` (bad input or an adapter error). Results are in
`data.conformance` and `build/conformance/status.json`, and on the claim's card.

With `conformance.blocking: true`, any non-matched check fails `check` with `conformance_failed`.
Fix the code or produce the observation, then re-run. **Never unlock or re-lock to clear it**: this
gate does not touch approval. The `embodiment` block is signed content, so adding or changing it on
a locked claim is unlock → fix → lock with the human. `mode: none` is the declaration that no code
embodies the claim, and it exempts the claim from the code-link gate.

## When a code change means the claim is wrong

Months after a lock, a new requirement changes the code. Ask one question: **can you state a
specific before/after for the claim's wording?**

- **Yes, and the claim renders from `body` only → `dossierx claim flag`** (below).
- **Yes, but the claim carries `rows`, `steps` or `raw_html`, or uses `layout: mockup` → unlock →
  fix → lock** with the human's `--reason`. `claim flag` refuses these with `structured_layout`: a
  flag's reaudit rewrites `body` only, and would clear `review_pending` while the content a reader
  sees stayed stale. Put the before/after in your message to the human instead.
- **Yes, but only the code moved — same meaning, new file or name → re-tag** (or re-run `claim
  link`). Nothing to approve; never flag a refactor.
- **No → a comment** (`dossierx-comments`). A question or a doubt has no `--now-does`.

```
dossierx claim flag <id> \
  --claim-says "what the claim currently states" \
  --now-does   "what the code now actually does" \
  --reason     "why the code changed"
```

All three are required; `--dry-run` previews it. It sets `locked, review_pending` and hands the claim
to the human: `--claim-says` renders as the removal and `--now-does` as the addition in `claim
reaudit`'s diff. Continue from the reaudit section of
**[`dossierx-claims`](../dossierx-claims/SKILL.md)**.

**The before/after lives in `build/ledger/flag-store.json`.** Commit it in the same commit as the
flagged claim and never `.gitignore` it. No integrity rule covers it, so a flag that does not travel
is lost in silence: the claim arrives elsewhere `review_pending`, `reaudit` proposes an empty diff,
and confirming that empty diff clears the flag having changed nothing. After flagging, tell the human
the store needs committing.

`source_dirs` and `conformance` are the only config fields this skill depends on; unset, a project
sees no change in behavior.
