---
name: audit-loop
description: Design or run a bounded audit-and-fix loop with independent review, regression-pinning fixes, explicit evidence, and a reachable exit condition. Use when an agent is asked to iterate between auditors and fixers; do not use for an ordinary one-pass review.
---

# Audit loop

Use an audit loop to close a fixed acceptance scope. Do not use it as an
open-ended search for everything that could ever be improved.

Before starting, read the agent-neutral
[audit-and-fix workflow](references/audit-fix-loop.md). Record the exact
candidate, baseline, acceptance criteria, verification commands, allowed change
scope, and round cap before dispatching an auditor or fixer. Read the project's
tracked guidance for required gates, commands, and release authority. Supply
these bindings in the run contract; do not assume a particular repository,
agent platform, model, or machine. This directory contains the shared procedure.

## Lock the audit scope

Every blocking audit finding must be one of:

- an acceptance item that is not verifiably satisfied;
- a regression in code changed by this loop;
- drift between a normative or pinned document and the implementation.

Future work, portability ideas, latent risks outside the changed surface, and
taste belong in the backlog. They must not silently widen the blocking scope.

Include every applicable project gate from those bindings in the fixed scope.
An audit-loop result does not replace a separately required project result.

## Make green reachable

Green is an acceptance and evidence gate, not an empty findings list. A round
may report non-blocking backlog items and still be green. A regression in
previously fixed code blocks regardless of severity, so fixers may repair it.
Count outstanding regressions as well as newly found ones; rediscovery is not
required to keep an unresolved regression blocking.

The controller, not an auditor, decides convergence. Stop successfully only
when:

- every acceptance item and every blocking finding, including findings discovered
  during the loop, is satisfied or closed with direct evidence;
- every required check ran and passed; a skipped or unmatched check is a
  failure;
- no critical or major regression remains in code changed by the loop, and no
  regression of any severity remains in previously fixed code;
- pinned documents match the implementation;
- two consecutive audit rounds find zero regressions in previously fixed code
  on the same unchanged candidate, with complete required coverage in both;
- every separately required project gate passed for the exact final tested tree. Record the commit plus any uncommitted patch identity;
  a working-tree result is not a result for the commit alone.

Any candidate edit, incomplete round, tool failure, or regression resets the
consecutive-round counter. A verifier accepting a fix does not count as a full
audit round. Do not count audits of an earlier tree toward the final tree.

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

- baseline and final candidate commits/tree identities;
- the complete finding ledger and closure evidence;
- required commands and exact outcomes;
- per-round newly found and outstanding regression counts;
- the two rounds that satisfied the convergence rule, if passed;
- the non-blocking backlog;
- remaining findings and the reason, if failed or blocked.

The loop may edit files only within the authority of the task that invoked it.
It does not grant permission to commit, push, merge, publish, or release.
