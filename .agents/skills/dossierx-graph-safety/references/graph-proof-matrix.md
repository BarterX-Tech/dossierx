# DossierX graph proof matrix

Use the smallest graph that isolates each property. Then use a layered dense
graph to test growth.

## Required graph shapes

| Shape | What it must prove |
| --- | --- |
| Single edge | A child can be locally approved against a readable draft parent. |
| Chain | Parent drift and review propagate through unchanged intermediates. |
| Diamond | Two dependent-to-dependency baselines remain independently clearable. |
| Reconverging diamond | A key based only on first hop or terminal node cannot merge causes. |
| Multiple routes to one source | One source stays active while any route remains, without one record per route. |
| Wide fan-in and fan-out | Record growth follows independent facts, not path count. |
| Layered dense DAG | Exponentially many routes produce polynomial work and bounded output. |
| Self-cycle and multi-node cycle | Traversal terminates and emits a valid bounded cycle witness. |
| Missing or invalid node | Readiness remains false and the diagnostic identifies the obstacle. |

## Semantic matrix

For every relevant graph shape, assert separately:

- `local_approved`;
- `dependency_ready`;
- `review_pending`;
- final `ready`;
- standing approval and claim status;
- surviving cause or condition identities;
- representative path validity;
- selective clearing after one baseline is refreshed;
- no input or ledger mutation.

Cover own thread, own flag, direct dependency drift, approval-integrity failure,
unapproved dependency, missing dependency, retired dependency, unreadable
dependency, unknown historical baseline, and cycles.

## Complexity contract

Write the expected upper bound before running the test. Prefer a bound based on:

- `V`: reachable claims;
- `E`: required dependency edges;
- `F`: independently clearable local facts.

Reject an implementation whose work or output is proportional to the number of
distinct paths. Reject a test that only uses claim count as its scale variable.

The dense test must fail the old exhaustive implementation for the intended
reason and pass the candidate without relying on a machine-specific generous
timeout. Assert structural counts and serialized bytes. Record allocations when
the language runtime supports a stable measurement.

## Differential proof

On generated small acyclic and cyclic graphs, compare the candidate with the
previous exhaustive implementation. Compare readiness booleans and normalized
independent fact identities. Paths may differ only under the documented
representative-path rule.

Keep any existing selective-clearing tests unchanged. Add tests rather than
weakening assertions that protect ledger semantics.

## Consumer proof

Run the same cases through:

- claim show or list projections;
- write-mode `check` and its catalog;
- generated viewer data;
- live `serve` rendering;
- singleton and batch preview and write paths.

When this evidence belongs to a release candidate, record it at the graph-safety
item in `docs/RELEASING.md` and follow that file for the release itself. A green
unit suite without the dense graph proof cannot clear the graph-safety item.
