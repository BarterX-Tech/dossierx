# DossierX graph proof matrix

Use the smallest graph that isolates each property, then vary graph size and
shape to test growth. Apply approval rows when approval/readiness is affected;
for other traversals add their own ordering, membership, edge, and diagnostic
invariants. For example, build order must preserve phase ordering and explicit
exclusions; an approval boolean alone cannot test it. Record applicability and
justify exclusions using the call graph before running proofs.

## Required graph shapes

| Shape | What it must prove |
| --- | --- |
| Single edge | Under v1, a child can be locally approved against a readable draft parent while remaining unready; legacy policy is not silently migrated. |
| Chain, including a deep chain | Parent drift and review propagate through unchanged intermediates; depth does not cause unsafe stack or witness growth. |
| Diamond | Two dependent-to-dependency baselines remain independently clearable. |
| Reconverging diamond | A key based only on first hop or terminal node cannot merge distinct causes. |
| Multiple routes to one source | One source stays active while any route remains, without one record per route; removing the chosen witness route retains the fact and yields another valid witness. |
| Wide fan-in and fan-out | Growth follows edges and independent facts, not path count. |
| Layered dense DAG | Exponentially many routes produce bounded work and output, both for one query and the whole corpus. |
| Self-cycle, two-node and longer cycles, with incoming/outgoing tails | Traversal terminates, keeps all affected readiness false, and emits valid bounded witnesses; cycles do not hide other independent obstacles. |
| Missing or invalid node | Readiness stays false, diagnostics identify the obstacle, and readers fail visibly when stores or source cannot be loaded. |

For readiness scale cases include all-draft and approved/mixed graphs. The
all-draft case exercises conditions even with no review causes; approved graphs
with drift, flags, and threads exercise cause propagation. Vary depth, width,
density, and independent fact count rather than only repeating one shape.

## Semantic matrix

For every applicable case, assert separately:

- `local_approved`, `dependency_ready`, `review_pending`, and final `ready`;
- standing approval and claim status, including invalid approval records and
  stale persisted review bits in both directions;
- surviving cause and condition identities;
- representative path validity and deterministic selection under reordered input;
- selective clearing after one baseline or own source is resolved, and remaining
  truth after a witness route disappears;
- no input or ledger mutation by the computation.

Cover own thread, own flag, direct dependency drift, approval-integrity failure,
unapproved dependency, missing dependency, retired dependency, unreadable
dependency, unknown historical baseline, cycles, and legacy/v1 stores. Include
multiple independent causes at one claim and independent baselines to one target.
Governance/mirror drift must still propagate as specified without accidentally
becoming required approval edges. Define an inherited witness as a required-edge
route to its owner plus any documented terminal drift edge; not every diagnostic
hop is necessarily `rests_on`.

Check every witness edge against the input. A cycle witness must contain an
actual closed cycle; any prefix must reach it through real edges. Do not accept
repeated non-edges just because the previous implementation emitted them.

For affected approval mutation paths, additionally prove singleton/batch verdict
parity on the same candidate snapshot, unrelated-member invariance, explicit
semantic-conflict refusal, malformed/stale/wrong-set token refusal, zero writes
on refused batches, and failure/recovery behavior on write errors. Snapshot
claims, ledger, receipts, baselines, and flags around these operations in a
disposable fixture. Successful writes have their own expected mutations; the
readiness computation's no-mutation assertion does not prohibit authorized writes.

## Complexity contract

Write expected upper bounds and practical acceptance budgets before executing.
Define variables rather than hiding cost in a vague "fact" count:

- `V`: claims in scope; distinguish one query's reachable graph from all claims;
- `E`: edges examined, counting required and drift edges separately if needed;
- `F`: independent local facts, with identity and maximum multiplicity specified;
- witness length and source string/input bytes, including identifiers and details.

Bound traversal work, transient and retained memory, records, and serialized
bytes separately. State the aggregate bound when every claim receives an
assessment. A per-query O(V + E) claim does not make V queries linear. One
O(V)-length witness per record multiplies output size; count duplicated alias
fields and copies in actual catalog and viewer serialization, not only a compact
internal map. Account for sorting, copying, repeated hashing, and serialization.

Reject exhaustive path enumeration even if the final output is deduplicated.
Assert structural operation/record limits and actual serialized byte budgets
across multiple sizes. Measure elapsed time and allocations with the declared
runtime/toolchain, repetitions, platform, and budget; use reproducible allocation
or operation limits for regression assertions where timing is noisy. Timeouts
are containment and practical budget checks, not complexity proofs. Explain
measurement limitations; do not silently replace required evidence with a skip.

Do not run the historical exhaustive algorithm on an unbounded dense production
corpus or deliberately exhaust the host. For a path-growth repair, demonstrate
the old defect with a safely bounded small counterexample, structural work
instrumentation, or a documented analytical bound, and run the large regression
only on the candidate under explicit resource limits. A baseline killed by a
limit proves that limit was exceeded; it is not an exact timing/allocation result.
Later candidates may have an already-bounded baseline: require continued
compliance and relevant regression sensitivity, not that the previous version
must fail. Document how the test catches the prohibited growth without requiring
an unsafe historical execution.

## Differential proof

Use generated small acyclic and cyclic graphs with recorded seeds and parameters.
Pin the oracle revision and any normalization code. Compare unchanged readiness
booleans and normalized independent fact identities against a trusted baseline;
validate witnesses independently and allow changes only under the documented
candidate representation rule. The normalizer must not use the candidate's
potentially faulty deduplication key as its only definition of expected identity.

Known baseline bugs and intentional contract changes require named exceptions
with concrete contract-based expected results and regression tests. Do not
blanket-ignore all cycles or all paths because one differs. For example, the
exhaustive `collect`/`cyclePath` in the original skill candidate can repeat a
non-edge on cycles longer than two nodes; output equality is not evidence that
such a witness is valid. Recheck source before assuming that defect still exists.

Preserve selective-clearing coverage. A documented representation change may
require changing path/count assertions; replace them with checks for every
independently clearable identity, surviving cause, and valid witness rather than
removing the semantic protection. Keep unaffected assertions intact.

## Consumer proof

Map changed computations to callers. Run shared cases through every affected
surface, with expected differences recorded rather than assuming byte equality:

- claim show and list projections, when both consume the changed assessment;
- write-mode `check` and the actual emitted catalog;
- generated viewer data and relevant browser-visible behavior;
- live `serve` initial rendering and refresh after an upstream change;
- singleton and batch preview/write paths when their policy or inputs are affected.

Use disposable corpora for writer and server tests. `check --validate` alone
cannot substitute for serialization/rendering. `check` may correctly reject a
malformed corpus that `serve` must still display safely: assert each surface's
contract and shared readiness facts, not identical exit codes. Exercise both the
small semantic cases and the scale-critical end-to-end paths. Include corrupted
store/load errors where those paths are affected so missing assessments cannot
look ready. Prove checks ran and assertions matched; inherited or cached success
is not current execution evidence.

When this belongs to a release candidate, attach durable evidence to the
graph-safety item in `docs/RELEASING.md`. Only that file governs release execution.
A green unit suite without the applicable graph proofs cannot clear that item.
