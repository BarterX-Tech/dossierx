---
name: dossierx-graph-safety
description: Audit and harden DossierX graph algorithms and graph-changing release candidates against semantic degradation and combinatorial blowups. Use when changing dependency readiness, claim locking, review propagation, build-order graphs, catalog or viewer graph projections, or a release that changes graph traversal.
---

# DossierX Graph Safety

Protect graph meaning and graph cost together. A correct answer is not releasable
if producing it is unbounded for a valid corpus.

This is a maintainer skill for this repository. It is not one of the consumer
skills embedded from `skills/` and must not be added to the exported bundle.
`docs/RELEASING.md` remains the only release procedure.

## Start from current evidence

Confirm the checkout, branch, candidate commit, release tag, upstream state, and
dirty files. Read the current graph contract, implementation, and tests from the
exact candidate. Do not infer release safety from an older checkout, generated
artifact, or prior green run.

Before implementation, state:

- the graph facts that determine truth;
- which facts are independently clearable or independently blocking;
- the identity used to deduplicate derived records;
- the worst-case time, memory, record-count, and serialized-size bounds in terms
  of nodes, edges, and independent facts.

If any bound depends on the number of possible paths, stop. Redesign before
implementation.

## Preserve the approval contract

Treat these as non-negotiable:

- Local approval, dependency readiness, and integrated proof are different.
- A claim can retain `status: locked` and local approval while live readiness
  reports `review_pending: true` or `dependency_ready: false`.
- A dependent-to-dependency ledger baseline is its own review boundary.
- A parent change must propagate through unchanged intermediate claims.
- Clearing one boundary must not clear another independent boundary.
- Readiness computation must not refresh baselines, rewrite approvals, or mutate
  claims, receipts, flags, or graph edges.
- `governed_by` and `mirrors` remain drift edges unless the product contract is
  explicitly changed. They do not silently become lock prerequisites.
- Singleton and batch locking use the same evaluator, snapshot rules, and
  failure-atomic write behavior.

Do not deduplicate review causes only by their terminal dependency. Use an
identity that preserves the claim which owns the local cause, its original cause
kind, the dependency for edge-owned causes, and any specific source identity.
Representative paths are diagnostic witnesses, not clearance authority.

## Prove semantics and scale

Read [references/graph-proof-matrix.md](references/graph-proof-matrix.md) whenever
designing, reviewing, or releasing a graph change. Use its graph shapes and proof
matrix.

Require both:

1. Semantic equivalence on small graphs, preferably against the previous
   implementation as a differential oracle.
2. An adversarial scale test that asserts bounded record count, output size,
   runtime, and allocations before the release candidate can pass.

Deduplicate during bounded traversal. Never enumerate all paths and deduplicate
after allocation. A truncation cap may limit diagnostics, but readiness truth
must be computed independently of what is displayed.

Verify every consumer that shares the projection: claim commands, catalog,
viewer, `check`, and `serve`. Verify invalid and cyclic input terminates safely.

## Report the graph gate honestly

Return `PASS`, `FAIL`, or `BLOCKED` with:

- candidate commit and comparison baseline;
- preserved semantic invariants;
- stated complexity bounds;
- adversarial fixture shape and measured results;
- consumer-parity results;
- unresolved risks.

Do not call the change non-degrading until an actual patch passes the semantic
and scale proofs. A proposed algorithm is design evidence, not release evidence.
