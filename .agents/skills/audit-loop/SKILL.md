---
name: audit-loop
description: Design or run a bounded DossierX audit-and-fix loop with independent review, regression-pinning fixes, explicit evidence, and a reachable exit condition. Use when an agent is asked to iterate between auditors and fixers; do not use for an ordinary one-pass review.
---

# DossierX audit loop

Use an audit loop to close a fixed acceptance scope. Do not use it as an
open-ended search for everything that could ever be improved.

Before starting, read the agent-neutral
[audit-and-fix workflow](../../workflows/audit-fix-loop.md). Record the exact
candidate, baseline, acceptance criteria, verification commands, allowed change
scope, and round cap before dispatching an auditor or fixer.

## Lock the audit scope

Every blocking audit finding must be one of:

- an acceptance item that is not verifiably satisfied;
- a regression in code changed by this loop;
- drift between a normative or pinned document and the implementation.

Future work, portability ideas, latent risks outside the changed surface, and
taste belong in the backlog. They must not silently widen the blocking scope.

When the candidate changes graph behavior, the fixed acceptance scope must
include the `dossierx-graph-safety` proof obligations. An audit-loop result does
not replace the graph-safety result.

## Make green reachable

Green is a severity and evidence gate, not an empty findings list. A round may
report minor or non-blocking backlog items and still be green.

The controller, not an auditor, decides convergence. Stop successfully only
when:

- every original blocking finding is closed with direct evidence;
- every required check ran and passed; a skipped or unmatched check is a
  failure;
- no critical or major regression remains in code changed by the loop;
- pinned documents match the implementation;
- two consecutive audit rounds find zero regressions in previously fixed code;
- any separately required project gate, including graph safety, passed for the
  exact final commit.

A round cap is a cost guard, not a success condition. Reaching it produces
`BLOCKED` with the remaining findings and the reason convergence failed.

## Keep roles independent

The controller owns scope, state, convergence, and the final evidence. Auditors
read the implementation and run relevant checks; they do not trust fixer
summaries. Fixers address only active blocking findings and do not declare their
own work accepted.

Every fix must include a test or probe that would have caught the defect. Keep
the diff narrow. Route observations outside the frozen scope to the backlog
instead of fixing them opportunistically.

Do not treat agent failure, missing output, a skipped check, or an empty result
from a failed auditor as a green round.

## Deliver the evidence

Return `PASS`, `FAIL`, or `BLOCKED` with:

- baseline and final candidate commits;
- original findings and their closure evidence;
- required commands and exact outcomes;
- per-round new-regression counts;
- the two rounds that satisfied the convergence rule, if passed;
- the non-blocking backlog;
- remaining findings and the reason, if failed or blocked.

The loop may edit files only within the authority of the task that invoked it.
It does not grant permission to commit, push, merge, publish, or release.
