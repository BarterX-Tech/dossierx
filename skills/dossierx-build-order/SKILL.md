---
name: dossierx-build-order
description: >-
  Workflow for deriving and following a dossierx-v1 module's Build Order — the
  dependency-ordered sequence (orientation → schema → behavior → api →
  verification) an implementing agent should actually build a fully-locked
  module in, as opposed to the unrelated human "reading order" the viewer
  displays claims in. Use this WHENEVER a dossierx-v1 module has just become
  fully locked and you are about to implement code from it, or whenever you
  are about to write code against dossierx-v1 claims and want to know what
  order to build them in. Covers the completeness gate, the topological
  sort within behavior/api, the propose→lock review step, and what to do
  when the build order goes stale. Requires the dossierx-claims skill's
  claim/lock basics first.
---

# dossierx-v1 Build Order — implementation sequencing

Once every claim in a dossierx-v1 module is `status: locked`, Build Order derives the sequence
an implementing agent should actually follow — not the same thing as the viewer's reading
order (`section`/`order`), which exists purely for a human browsing the spec top to bottom
and has no bearing on what to build first.

Read **[[dossierx-claims]]** first if you haven't — this skill assumes you already know the
claim schema and lock lifecycle, in particular `build_role`.

## When to use this

- A module you're about to implement has just had its last claim locked.
- You're mid-implementation and unsure which claim to build next.
- `dossierx check`'s "next steps" block says a module is fully locked with no build order yet.
- A locked build order started reporting `stale` (a covered claim changed, was deleted, or a
  new claim was locked into the module).

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

**`build_role: orientation` and `kind: orientation-note` are different axes — but both
participate in Build Order.** `build_role` classifies *where in the build sequence* a claim
sits (this section's five phases); `kind` classifies *what a claim is* — a fact about the
system (the default) versus reading guidance about the rest of the module
(`kind: orientation-note`, or the reserved `overview` facet, which implies it — see
[[dossierx-claims]]). They are orthogonal, but a `kind: orientation-note` claim is **not**
invisible to Build Order and **does** carry a `build_role`:

- Every claim in a module must be locked before `propose` runs (the completeness gate), and
  once a module adopts `build_role` the `build-role-required-for-locked` lint hard-fails
  locking *any* claim — orientation-note claims included — without a valid `build_role`.
- `propose` likewise rejects a locked claim with no `build_role` set, so an orientation-note
  claim reaches the build order carrying one.
- An orientation-note claim's natural `build_role` is **`orientation`** (phase 1): it is
  read for background — "read Contract, never Internals" — and never itself built from, so
  it renders in the orientation phase alongside `build_role: orientation` fact claims.

So a claim can be `kind: orientation-note` **and** `build_role: orientation` at once (the
common case), or either one on its own — but neither one lets a locked claim skip having a
`build_role` once its module uses the feature.

## Commands

```
dossierx build-order propose --module <name>   # derive the sequence, write .build-order.<module>.json
dossierx build-order status  --module <name>   # proposed / locked / stale + N of M claims covered
dossierx build-order lock    --module <name>   # freeze the proposed sequence (human confirms)
```

`propose` is a completeness gate: it refuses outright, listing every non-locked claim, unless
100% of the module's claims are locked. This is deliberate — the whole point is that a build
order only ever gets generated once a module's docs are genuinely done, so it covers every
claim by construction, nothing skipped.

## The review step

1. `dossierx build-order propose --module <name>` — writes the proposed sequence. Writes
   nothing if the module isn't fully locked, or if a same-phase cycle makes the sequence
   invalid (both cases print exactly what's blocking it).
2. **Present the proposed sequence and wait for explicit confirmation** before locking —
   same review discipline as any other dossierx-v1 gate.
3. `dossierx build-order lock --module <name>` — freezes it. Refuses if nothing was proposed
   yet, if the order is **stale** (a bare relock would freeze an outdated sequence — re-run
   `propose` first, see below), or if it's already locked and not stale (nothing to relock).

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

`dossierx build-order status --module <name>` reports `stale: true` if, since the sequence was
locked, a covered claim's content changed, a covered claim was deleted, or a new claim was
locked into the module (coverage the frozen order silently omits). `lock` refuses a stale
artifact outright — a bare relock would only refresh hashes/flags without recomputing the
order, freezing an outdated sequence (e.g. after a `rests_on` edit). Re-run `propose`
(recomputes the order against the current claim set) then `lock` again — same
review-before-lock discipline as the first time. Never treat a stale build order as still
authoritative; re-derive it.

## Portability note

Nothing about Build Order is project-specific — the phase sequence, the completeness gate,
and the topological sort all operate purely on `build_role`/`rests_on`/`status`, fields every
dossierx-v1 project already has via `project.config.yaml`. No new config surface is needed for
this skill beyond what dossierx-claims already requires.
