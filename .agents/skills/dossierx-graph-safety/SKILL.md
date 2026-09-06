---
name: dossierx-graph-safety
description: Audit and harden changes to DossierX graph behavior against semantic degradation and combinatorial blowups. Use for dependency readiness, claim locking, review propagation, build-order graphs, catalog or viewer graph projections, and traversal behavior; documentation-only typo or formatting edits do not require runtime graph proofs.
---

# DossierX Graph Safety

Protect graph meaning and graph cost together. A correct answer is not releasable
if producing it exhausts resources on a supported corpus.

This is a maintainer skill for this repository. It is not one of the consumer
skills embedded from `skills/` and must not be added to the exported bundle.
`docs/RELEASING.md` remains the only release procedure. The concrete failure that
motivated this skill is recorded in
[the graph path-growth lesson](../../../docs/lessons/2026-09-06-graph-path-growth.md).
The lesson explains the obligation; current source and contracts determine the
candidate's behavior.

Apply runtime proofs to changes affecting graph behavior, including shared inputs
and consumers, not merely files containing graph terminology. Documentation-only
typos or formatting need ordinary review. A contract proposal needs semantic and
complexity design review, but cannot earn a runtime `PASS` without an implemented
candidate. Editing this skill alone does not prove the engine graph-safe.

## Start from current evidence

Confirm the repository root, branch, candidate commit, upstream state, and dirty
files. For release work also resolve the target tag, if it exists, and the actual
previous release tag to full commits. Record the comparison range and inspect its
whole diff; the last commit alone does not describe a release. Do not use the
candidate itself as its release comparison baseline. Distinguish a compatibility
baseline from a historical faulty implementation used for a regression test.

Read the current contract, implementation, and tests from the exact candidate.
Start with `docs/approval-policy.md`, `internal/readiness/`, `internal/lock/`, and
the affected caller paths; include `internal/buildorder/`, catalog, render, and
serve code when relevant. Inspect other traversal contracts when those change.
Do not infer safety from an older checkout, generated artifact, or prior green
run. Identify the tree actually tested, including any uncommitted patch; a result
for that tree is not a result for the named commit alone.

Before implementation, state:

- the graph facts that determine truth and the applicable policy version;
- which facts are independently clearable or independently blocking;
- the identity used to deduplicate each kind of derived record;
- the affected computations and consumers, with reasons for exclusions;
- the worst-case time, memory, record-count, and serialized-size bounds for one
  query and for all claims and their actual catalog/viewer output.

If any bound depends on the number of possible paths, stop. Redesign before
implementation. This rejects exhaustive route enumeration, including intermediate
allocation; it does not prohibit bounded witnesses whose length depends on graph
depth. Account for those witness bytes explicitly.

## Preserve the approval contract

For changes touching approval or readiness, preserve these current boundaries:

- Local approval, dependency readiness, and integrated proof are different.
- Under policy v1, a readable draft prerequisite can support local approval;
  that does not make the dependent ready. Legacy stores retain their policy
  until explicit migration. Do not silently apply v1 rules to legacy evidence.
- A claim can retain `status: locked` and local approval while live readiness
  reports `review_pending: true` or `dependency_ready: false`. Standing ledger
  integrity and live causes, not the saved review bit alone, determine truth.
- A dependent-to-dependency ledger baseline is its own review boundary.
- A parent change must propagate through unchanged intermediate claims.
- Clearing one boundary must not clear another independent boundary.
- Readiness computation must not refresh baselines, rewrite approvals, or mutate
  claims, receipts, flags, or graph edges.
- `governed_by` and `mirrors` are drift inputs, not lock prerequisites solely
  because their target is unapproved. Existing integrity and lint gates remain.
- Singleton and batch locking use the same evaluator and snapshot rules. Refused
  batches write nothing; write failures follow the contract's rollback or
  explicit recovery behavior and never look like successful partial approval.

A skill does not approve a product-contract change. Separate invariants preserved
from intentionally changed behavior, cite the decision authorizing the latter,
and update the contract and tests together. The current implementation emits
path-based records; a bounded fact representation and representative-path policy
must be specified and verified as a candidate change, not described as already
shipped behavior.

For such a representation, do not deduplicate review causes only by terminal
dependency. Preserve the claim owning the local cause, original cause kind,
dependency for edge-owned causes, and any specific source identity. Define
condition identities separately, including edge-owned unknown baselines and
cycles. Representative paths are diagnostic witnesses, not clearance authority.
Diagnostic caps must disclose omissions and must not affect readiness truth.

## Prove semantics and scale

Read [references/graph-proof-matrix.md](references/graph-proof-matrix.md) whenever
designing, reviewing, or releasing a graph change. Use its applicable shapes and
proofs; explain exclusions before execution rather than silently dropping cases.

Require both:

1. Contract assertions on small graphs and differential comparisons for behavior
   that should remain equivalent. Name known baseline defects and assert their
   corrected outcomes independently; copying an old defect is not equivalence.
2. Adversarial scale evidence for the actual candidate, with asserted structural
   work/record and output-byte bounds, measured runtime and allocations, and
   explicit resource budgets. Polynomial notation alone does not prove usability.

Deduplicate during bounded traversal. Never enumerate all paths and deduplicate
after allocation. Verify invalid and cyclic input terminates safely. Cover every
affected consumer, including disk serialization and live rendering when shared
projections change. Read-only computation proofs and lock/write proofs are
separate: run mutation cases on disposable fixtures, and require them when the
change reaches their evaluator, snapshot, or write path.

## Report the graph gate honestly

- `PASS`: every applicable obligation ran on the identified candidate and met
  its declared expectations and budgets; no required evidence is missing.
- `FAIL`: an assertion, bound, consumer behavior, or contract is violated. A
  skipped check or selector matching no tests is failed coverage, never a pass.
  Missing prerequisites or required evidence also make the gate `FAIL` (the
  repository's `FAILED` outcome). Use "blocked" only as a reason describing the
  missing prerequisite and checks, never as a third, potentially passing verdict.
  List demonstrated defects separately from checks blocked from execution.

Return the verdict with:

- candidate commit/tree identity, comparison range, and oracle revisions;
- preserved invariants, approved changes, and explicit baseline defect exceptions;
- scoped proof matrix with every exclusion justified;
- worst-case bounds and declared acceptance budgets;
- fixture/generator paths, graph parameters, deterministic seeds, exact commands,
  relevant runtime/tool versions and platform, executed case counts, skips, and
  durable result paths with measured counts, bytes, time, and allocations;
- consumer results and unresolved risks, including failures already in baseline.

A missing tool is a blocker to acquire or report, not permission to reduce
coverage. State limitations of environmental measurements. A graph `PASS` is
only this obligation, not release approval or completion. Do not call a change
non-degrading until an actual patch passes the proofs. A proposed algorithm is
design evidence. A later change to the candidate invalidates the old exact-commit
result; re-establish applicable evidence for the final candidate through the sole
release procedure.
