---
name: dossierx-build-order
description: >-
  Deriving and following a DossierX module's Build Order — the dependency-ordered
  sequence (orientation, schema, behavior, api, verification) an implementing
  agent should actually build a fully-locked module in, as distinct from the
  human reading order the viewer displays claims in. Use this WHENEVER a module
  has just become fully locked and you are about to implement code from it,
  whenever you are unsure which claim to build next, and whenever a locked build
  order reports stale. Covers the completeness gate, the topological sort inside
  behavior and api, the propose then human-approved lock step, and what to do
  when the order goes stale. Load the DossierX router skill first, and the
  dossierx-claims skill for the lock basics this assumes.
---

# DossierX build order — implementation sequencing

Read **[`dossierx`](../dossierx/SKILL.md)** for the envelope and error codes, and **[`dossierx-claims`](../dossierx-claims/SKILL.md)** for
`build_role` and the lock lifecycle.

Once every claim in a module is `locked`, Build Order derives the sequence to **build** it in.
That is not the viewer's reading order (`section`/`order`), which exists for a human browsing the
spec and has no bearing on what to write first.

## The contract

```
dossierx build-order propose --module <name>                      # derive it; writes .build-order.<name>.json
dossierx build-order status  --module <name>                      # proposed | locked | stale, + N of M covered
dossierx build-order lock    --module <name> --reason "<their words>"
```

`propose` and `lock` both take `--dry-run`. `lock` requires `--reason` — it is a lifecycle action,
so it follows the same rule as `claim lock`: preview, show the human, get a yes, then run it.

| refusal | `error.code` | recovery |
|---|---|---|
| a module claim is still draft | `build_order_refused` | lock the remaining claims (each with the human's approval) |
| a module claim has an open thread | `build_order_refused` | reply; the human resolves in the viewer |
| a locked claim has no `build_role` | `build_order_refused` | unlock the claims it names (their yes, `--reason` with their words), set `build_role` on each, lock them again, then re-propose — no verb sets `build_role` on a locked claim, and setting it by editing the locked file is `lock-content-drift` on the next `dossierx check` |
| a same-phase `rests_on` cycle | `build_order_refused` | a real modelling error, never silently dropped — the claims on the loop are locked, so break it through unlock → fix → lock, each with the human's yes, then re-propose |
| the artifact was edited by hand | `build_order_hand_edited` | the claims are fine and the artifact is not — re-`propose` to discard the edit, then `lock` what the engine derived |
| nothing proposed yet | `not_proposed` | run `propose` |
| the locked order is stale | `build_order_stale` | re-`propose`, then re-`lock` |
| unknown `--module` | `unknown_module` | you typo'd it; a wrong module must not answer with an empty report |

## The five phases

Never reordered, never interleaved:

1. **orientation** — context and process claims. Read for background; never build code directly
   from one.
2. **schema** — data shapes. Build first; everything below assumes these types exist.
3. **behavior** — workflow and logic, the bulk of the work. Ordered *within* the phase by a real
   topological sort over `rests_on`, so a claim resting on another behavior claim is built strictly
   after it.
4. **api** — public entry points, after the behavior they wrap.
5. **verification** — test-checklist claims, read last so tests are written against everything
   already built.

`out-of-scope` claims are never placed in the sequence, but are always still reported as
`excluded` — never silently dropped from view.

`build_role` and `kind` are different axes. `build_role` says *where in the build sequence* a claim
sits; `kind: orientation-note` says *what the claim is* — reading guidance rather than a fact about
the system. They are orthogonal, but an orientation-note claim is **not** invisible to Build Order
and still needs a `build_role` (naturally `orientation`) before it can lock or be proposed.

## The completeness gate — why it refuses so much

`propose` refuses outright, naming every offending claim, unless **100% of the module's claims are
locked and none carries an open thread**. That is the point: a build order only exists once a
module's spec is genuinely finished, so it covers every claim by construction and nothing is
skipped. Do not work around it by excluding claims — a module that is not done is not ready to be
built from.

## Following the sequence

Read `.build-order.<module>.json` and build strictly phase by phase, in the order each phase
lists. Paths in it are project-relative by design — this artifact is meant to be shared, not to
record one machine's directory layout. Within `behavior` and `api`, the listed order already
accounts for every `rests_on` edge; do not re-derive your own order from the claim bodies.

Read it; never edit it. A **locked** artifact is an approved one: `build-order lock` records its
signature in the same ledger a `claim lock` writes to, and `dossierx check` reports a hand edit as
`build-order-content-drift` — reordering phases, moving a claim into `excluded`, or refreshing the
frozen `hashes` so it stops reporting `stale` are all the same act as editing a locked claim.
Commit the locked artifact along with the ledger; the sequence a human approved has to travel with
the repository for CI to check it.

As each claim's code lands, ground it — that is **[`dossierx-code-links`](../dossierx-code-links/SKILL.md)**, the natural next step
after finishing a claim from this sequence.

## When it goes stale

`dossierx build-order status --module <name>` reports `stale: true` when, since the order was
locked, a covered claim's content or `build_role` changed, a covered or excluded claim was deleted,
or a new claim was locked into the module.

`lock` refuses a stale artifact (`build_order_stale`) rather than refreshing it: a bare relock
would update hashes without recomputing the sequence, freezing an order that is now wrong — for
instance after a `rests_on` edit. Re-run `propose` to recompute against the current claim set,
show the human the new sequence, then `lock` again with their `--reason`. Never treat a stale build
order as authoritative.

## Portability

Nothing here is project-specific. The phase sequence, the completeness gate and the topological
sort operate purely on `build_role`, `rests_on` and `status` — fields every DossierX project
already has. No configuration beyond what **[`dossierx-claims`](../dossierx-claims/SKILL.md)** requires.
