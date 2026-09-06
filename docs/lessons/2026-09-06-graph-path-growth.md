# Graph path growth after policy v1

- Status: verified source mechanism; incident measurements are reporter evidence.
- Recorded: 2026-09-06.
- Scope: DossierX readiness projections and their catalog/viewer consumers.
- Verified source revision: `a7eb26a8d8aa37c985b4a225890d90e848fad1d1`.
- Incident: [issue 62](https://github.com/BarterX-Tech/dossierx/issues/62).
- Skill introduction: [PR 63](https://github.com/BarterX-Tech/dossierx/pull/63).

## Failure and evidence

Under lock policy v1, a dense corpus with unapproved dependencies can emit one
readiness condition per dependency path. Issue 62 reports a 7.3 GB catalog from
825 claims after releasing the claims to draft. Those numbers describe that
reported corpus; they are not a benchmark rerun or a universal threshold.

At the verified revision, `internal/readiness/readiness.go`'s `collect` recursively
visits dependencies, prefixes child paths, and accumulates records before
path-sensitive deduplication. `internal/catalog/catalog.go` includes the
assessments in the catalog. This source inspection supports the growth mechanism.
`check --validate` does not establish that write-mode catalog generation or live
`serve` can finish within resource limits.

## Reusable rule and limits

Prove graph meaning and graph cost separately. Count independent blocking facts
and review boundaries; do not use every route as a distinct fact. Bound traversal,
record count, path storage, and whole-output size before claiming safety. A small
semantically correct fixture cannot establish scale safety.

Do not generalize the issue's proposed terminal-dependency deduplication to all
review causes. Different ledger boundaries may refer to the same dependency and
must remain independently clearable. The existing approval contract and tests
are the authority for those distinctions.

The graph skill and its proof matrix define the required future evidence. Their
existence does not repair the algorithm, demonstrate a new bound, or clear a
release. This lesson introduces no release procedure or release authorization.

## Guard and recheck conditions

Use [dossierx-graph-safety](../../.agents/skills/dossierx-graph-safety/SKILL.md).
Check a small semantic oracle, selective clearing, a budgeted adversarial graph,
and affected consumers. Record actual test cases and results with the candidate
revision when a repair exists. Do not reproduce the multi-gigabyte incident on
an unbounded workload merely to reconfirm it.

Revisit this record when the readiness identity, approval policy, diagnostic
contract, or projection implementation changes. Compare the old and current
contract; mark superseded rules explicitly. An old incident is evidence to
investigate, not authority to reject an intentional reviewed product change.
