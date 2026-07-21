---
name: dossierx-build-order
description: >-
  Workflow for deriving and following a docs-v1 module's Build Order — the
  dependency-ordered sequence (orientation → schema → behavior → api →
  verification) an implementing agent should actually build a fully-locked
  module in, as opposed to the unrelated human "reading order" the viewer
  displays claims in. Use this WHENEVER a docs-v1 module has just become
  fully locked and you are about to implement code from it, or whenever you
  are about to write code against docs-v1 claims and want to know what
  order to build them in. Covers the completeness gate, the topological
  sort within behavior/api, the propose→lock review step, and what to do
  when the build order goes stale. Requires the dossierx-claims skill's
  claim/lock basics first.
---

# docs-v1 Build Order — implementation sequencing

Once every claim in a docs-v1 module is `status: locked`, Build Order derives the sequence
an implementing agent should actually follow — not the same thing as the viewer's reading
order (`section`/`order`), which exists purely for a human browsing the spec top to bottom
and has no bearing on what to build first.

Read **[[dossierx-claims]]** first if you haven't — this skill assumes you already know the
claim schema and lock lifecycle, in particular `build_role`.

## When to use this

- A module you're about to implement has just had its last claim locked.
- You're mid-implementation and unsure which claim to build next.
- `docs check`'s "next steps" block says a module is fully locked with no build order yet.
- A locked build order started reporting `stale` (a covered claim changed or was deleted).

## The five build_role phases

Every locked claim's `build_role` places it in a fixed sequence — phases are never
reordered or interleaved:

1. **orientation** — context/process claims. Read for background; never build code from
   these directly.
2. **schema** — data shapes. Build first: everything below assumes these types exist.
3. **behavior** — workflow/logic claims, the bulk of the real work. Ordered *within this
   phase* by a real topological sort over `rests_on` (Kahn's algorithm, layered) — a claim
   resting on another behavior claim is built strictly after it. A same-phase `rests_on`
   cycle is a hard error at propose time, never silently dropped from the sequence.
4. **api** — public entry points, built after the behavior they wrap.
5. **verification** — test-checklist claims, read last so tests get written against
   everything already built.

`out-of-scope` claims (deferred/future-scope) are never placed in the sequence, but are
always still reported as `excluded` — never silently dropped from view.

**Don't confuse `build_role: orientation` with `kind: orientation-note`.** They're
unrelated axes that happen to share a word. `build_role: orientation` is this skill's
phase 1 — a context/process *fact* claim, still ordered and built like any other claim,
just first. `kind: orientation-note` (see [[dossierx-claims]]) is a claim that *isn't* a
fact at all — reading guidance rendered as a pinned banner, invisible to Build Order
entirely (it's reachable only via the reserved `overview` facet or a regular facet with
`kind` set, never a `build_role`). A claim can be `build_role: orientation` without being
`kind: orientation-note`, and vice versa.

## Commands

```
docs build-order propose --module <name>   # derive the sequence, write .build-order.<module>.json
docs build-order status  --module <name>   # proposed / locked / stale + N of M claims covered
docs build-order lock    --module <name>   # freeze the proposed sequence (human confirms)
```

`propose` is a completeness gate: it refuses outright, listing every non-locked claim, unless
100% of the module's claims are locked. This is deliberate — the whole point is that a build
order only ever gets generated once a module's docs are genuinely done, so it covers every
claim by construction, nothing skipped.

## The review step

1. `docs build-order propose --module <name>` — writes the proposed sequence. Writes
   nothing if the module isn't fully locked, or if a same-phase cycle makes the sequence
   invalid (both cases print exactly what's blocking it).
2. **Present the proposed sequence and wait for explicit confirmation** before locking —
   same review discipline as any other docs-v1 gate.
3. `docs build-order lock --module <name>` — freezes it. Refuses if nothing was proposed
   yet, or if it's already locked and not stale (nothing to relock).

## Following the sequence as an implementing agent

Read `.build-order.<module>.json` (project-relative paths only, never absolute — this
artifact is meant to be shared, not a dump of one machine's directory layout) and build
strictly phase by phase, in the order each phase lists. Within `behavior`/`api`, the listed
order already accounts for every `rests_on` dependency — don't re-derive your own order from
the claim bodies.

Once each claim's code exists, ground it in the codebase — that's the
**[[dossierx-code-links]]** skill, the natural next step after finishing a claim from this
sequence.

## When the build order goes stale

`docs build-order status --module <name>` reports `stale: true` if a covered claim's content
changed or was deleted since the sequence was locked. Re-run `propose` (produces a fresh
diff against the current claim set) then `lock` again — same review-before-lock discipline
as the first time. Never treat a stale build order as still authoritative; re-derive it.

## Portability note

Nothing about Build Order is project-specific — the phase sequence, the completeness gate,
and the topological sort all operate purely on `build_role`/`rests_on`/`status`, fields every
docs-v1 project already has via `project.config.yaml`. No new config surface is needed for
this skill beyond what dossierx-claims already requires.
