---
name: dossierx-code-links
description: >-
  Workflow for grounding finished code in the docs-v1 claims it implements,
  and for what to do when a later code change means a locked claim is no
  longer true. Use this WHENEVER you finish implementing or modifying code
  against a locked docs-v1 claim, whenever you add a "docs-claim: <id>"
  comment to source, or whenever a maintenance change makes a claim's
  stated behavior stop matching reality. Covers the docs-claim tag
  convention (scanned automatically via source_dirs + "docs check"), the
  internal/implink manual-link commands for cases scanning can't reach, the
  agent-autonomous vs. human-gated decision rule, and "docs flag" for
  reporting a spec mismatch. Requires the dossierx-claims skill's claim/lock
  basics first.
---

# docs-v1 Code Links — grounding claims in real code

Two independent, deliberately separate channels close the loop from spec back to code.
Read **[[dossierx-claims]]** first — this skill assumes you already know the claim schema and
lock lifecycle.

| | Channel A — spec is wrong | Channel B — grounding correct code |
|---|---|---|
| About | Truth: the locked claim itself needs revisiting | Fact-recording: where the (still-correct) claim lives in code |
| Owner | Human, via `docs flag` → `docs reaudit` (see dossierx-claims) | Fully agent-autonomous, no human gate |
| Trigger | Meaning changed vs. what's locked | Code finished, or a linked file's tag still matches |

## Channel B — the everyday case: tag it, `docs check` does the rest

The default, and the only thing most implementation work needs:

1. Immediately after finishing a claim's code (or, for a `verification`-role claim, its
   test), add a comment next to the relevant function/type — any comment syntax works, the
   engine only searches for the literal marker string:

   ```python
   # docs-claim: widget.internals.queue-saturation-policy
   def _drop_for_saturation(self):
       ...
   ```

2. Run `docs check`. If the project's `project.config.yaml` has `source_dirs` set, this
   scans those directories, finds the tag, and automatically links it — no separate command.
   A claim can have any number of tagged files; a file can carry tags for any number of
   claims.
3. An invalid tag (unknown claim id — check for a typo — or a claim that isn't locked yet)
   is a **hard failure**: `docs check` exits non-zero and names exactly what's wrong. This is
   deliberate — an unbacked or stale tag must never sit silently wrong in the codebase.
4. Symbol capture (the `#function_name` a human sees later) is a best-effort text heuristic
   recognizing common declaration shapes immediately below the tag line — not a real parser.
   File-level linking is always reliable regardless of whether it guesses the symbol right.

If the project has no `source_dirs` configured, or the file genuinely can't carry a comment
(a generated artifact, a migration file with no room for one), fall back to the manual
command — same validation, same artifact, it's simply not tag-triggered:

```
docs implink set --module <name> --claim <id> --file <path> [--symbol <name>]
```

Both paths write to the same generated `.implementation.<module>.json` — never hand-edit it.
`docs check` also reports drift (a linked file changed since it was tagged/linked) and an
"unlinked" count (locked `schema`/`behavior`/`api`/`verification` claims with zero linked
files) as a non-blocking part of its normal output — never a separate report to remember to
check.

## Channel A — when a code change reveals the spec is wrong

The scenario this exists for: months after lock, you're asked to change code for a new
requirement, and the change means the locked claim's stated behavior no longer matches
reality. This is not something Channel B's silent auto-linking should paper over.

**The rule for which channel a code change goes through**: did the code's *meaning* change
relative to what the claim states, or did it just move/get renamed/refactored with identical
behavior?

- Same meaning → Channel B. Just re-tag (or re-run `implink set`) with the new
  file/location. Nothing else.
- Meaning changed → Channel A:

  ```
  docs flag <id> \
    --claim-says "what the claim currently states" \
    --now-does   "what the code now actually does" \
    --reason     "why the code changed"
  ```

  This flips the claim to `locked, review_pending` — go to **[[dossierx-claims]]**'s reaudit
  section from here. `--claim-says`/`--now-does` render as a real diff in `docs reaudit`'s
  output (the same red-strike/green-add convention as any other claim edit), so the human
  reviewing it sees an actual before/after instead of reverse-engineered prose. This is the
  *only* place a human re-enters this otherwise fully autonomous workflow — flagging a
  genuine mismatch, never routine linking.

## Portability note

`source_dirs` is the one new, opt-in `project.config.yaml` field this skill depends on for
automatic scanning — unset, a project sees zero behavior change from before this feature
existed. Everything else (the tag marker string, the link artifact, the flag/reaudit
dispatch) is already covered by the same zero-hardcoded-assumptions guarantee described in
dossierx-claims, and has been verified directly against an unrelated synthetic project.
