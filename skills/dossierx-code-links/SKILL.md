---
name: dossierx-code-links
description: >-
  Grounding finished code in the DossierX claims it implements, and what to do
  when a later code change means a locked claim is no longer true. Use this
  WHENEVER you finish implementing or modifying code against a locked claim,
  whenever you add a "dossierx-claim: <id>" comment to source, whenever
  dossierx check reports a drifted or unlinked claim, and whenever a maintenance
  change makes a claim's stated behavior stop matching reality. Covers the
  dossierx-claim tag convention scanned by dossierx check, dossierx claim link
  for the cases scanning cannot reach, the fully-autonomous vs. human-gated
  decision rule, and dossierx claim flag for reporting a spec mismatch. Load
  the DossierX router skill first, and dossierx-claims for the lock basics.
---

# DossierX code links — grounding claims in real code

Read **[[dossierx]]** for the envelope and error codes, and **[[dossierx-claims]]** for the lock
lifecycle.

Two deliberately separate channels close the loop from spec back to code:

| | Channel B — grounding correct code | Channel A — the spec is wrong |
|---|---|---|
| About | where a still-correct claim lives in code | the locked claim itself needs revisiting |
| Gate | **fully yours**, no human gate | human, via `dossierx claim flag` → reaudit |
| Trigger | code finished, or a linked file moved with identical meaning | the code's *meaning* changed relative to what the claim states |

## Channel B — tag it, `dossierx check` does the rest

The everyday case, and the only thing most implementation work needs.

1. Immediately after finishing a claim's code (or, for a `verification` claim, its test), put a
   comment next to the relevant function or type. Any comment syntax works — the engine searches
   for the literal marker string:

   ```python
   # dossierx-claim: widget.internals.queue-saturation-policy
   def _drop_for_saturation(self):
       ...
   ```

2. Run `dossierx check`. If the project's `project.config.yaml` sets `source_dirs`, the scan finds
   the tag and links it — no separate command. A claim may have any number of tagged files; a file
   may carry tags for any number of claims.
3. An invalid tag is a **hard failure**: an unknown claim id (check for a typo) or a claim that is
   not locked yet makes `dossierx check` exit non-zero and name exactly what is wrong. Deliberate —
   an unbacked or stale tag must never sit silently wrong in the codebase.
4. Symbol capture (the `#function_name` a reader sees later) is a best-effort text heuristic over
   common declaration shapes below the tag line, not a real parser. File-level linking is reliable
   regardless.

When there is no `source_dirs`, or the file genuinely cannot carry a comment (a generated artifact,
a migration with nowhere to put one), link it explicitly — same validation, same artifact, simply
not tag-triggered:

```
dossierx claim link --module <name> --claim <id> --file <project-relative-path> [--symbol <name>]
```

It takes `--dry-run`, needs no `--reason` (it records a fact, it does not change what is approved),
and refuses a claim that is not locked (`not_locked`, exit 2). Both paths write the same generated
`.implementation.<module>.json` — never hand-edit it.

`dossierx check` also reports, non-blocking, the drift count (a linked file changed since it was
linked) and the unlinked count (locked `schema`/`behavior`/`api`/`verification` claims with zero
linked files). `dossierx claim show <id>` gives the same thing for one claim, per file:

```json
"implemented_in": [{"file": "internal/widget/queue.go", "symbol": "dropForSaturation", "drifted": true}]
```

## Channel A — when a code change reveals the spec is wrong

The scenario: months after lock, a new requirement changes the code, and the change means the
locked claim's stated behavior is no longer true. Channel B must not paper over that.

**First, is it actually a flag?** The discriminator is one question — *can you state a specific
before/after for the claim's wording?* If you only have a question or a doubt, that is a **comment**
(see **[[dossierx-comments]]**), not a flag.

**Then, which channel?** Did the code's *meaning* change relative to what the claim states, or did
it just move, get renamed, or get refactored with identical behavior?

- Same meaning → Channel B. Re-tag (or re-run `dossierx claim link`) with the new location.
  Nothing else, no approval needed. This is the common case and it is entirely yours.
- Meaning changed → Channel A:

  ```
  dossierx claim flag <id> \
    --claim-says "what the claim currently states" \
    --now-does   "what the code now actually does" \
    --reason     "why the code changed"
  ```

  All three are required, and `--dry-run` previews it. This sets `locked, review_pending` and hands
  the claim to the human: `--claim-says` renders as the removal and `--now-does` as the addition in
  `dossierx claim reaudit`'s diff, so they review a real before/after instead of reverse-engineered
  prose. Continue from **[[dossierx-claims]]**'s reaudit section. This is the **only** place a human
  re-enters this otherwise fully autonomous workflow — a genuine mismatch, never routine linking.

  `dossierx claim flag` works only on **body-rendered** claims (`card`, `banner`, `list`, `tree`).
  On a `table`, `steps` or `mockup` claim it is refused with `structured_layout`: a flag-sourced
  reaudit rewrites `body` and nothing else, so accepting it would clear `review_pending` while
  leaving the actually-rendered `rows`/`steps`/`raw_html` stale. For those, take the claim through
  **unlock → fix → lock** with the human's approval instead.

## Portability

`source_dirs` is the one opt-in `project.config.yaml` field this skill depends on; unset, a project
sees no behavior change. The tag marker string, the link artifact and the flag/reaudit dispatch are
all covered by the same zero-hardcoded-assumptions guarantee as everything else in DossierX.
