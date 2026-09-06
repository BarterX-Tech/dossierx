# Audit-loop convergence and portable packaging

- Recorded: 2026-09-06.
- State: verified instruction defects and qualitative decision trials.
- Scope: PR 63 based on `0c3089f821820c63af4d41dc7ba2de1ba55d4e9d`, with local skill improvements.
- Binding: `docs/MAINTAINER_SKILLS.md`.
- Skill: `.agents/skills/audit-loop/SKILL.md`.

## Evidence and rule

At the base revision, `.agents/workflows/audit-fix-loop.md` resets convergence
only on a regression. A round can audit one tree, apply a fix, then increment
the counter. Two such rounds can satisfy the counter without auditing the last
fix. This is an instruction-level counterexample, not a measured production
incident. The skill now requires two full rounds on the same unchanged candidate.
Both cover accumulated changes and previously closed findings. Edits, incomplete
coverage, tool failures, and regressions reset the counter.

The base workflow permits only critical/major blocking entries even when an
acceptance criterion requires a minor fix. It also gives minor regressions no
consistent repair and convergence path. Blocking status is now independent of
severity, and every outstanding regression in previously fixed code remains
blocking and eligible for repair. New findings remain in the complete ledger.

The base skill depends on a sibling workflow directory. Copying its directory
alone loses the procedure. The workflow now lives inside the skill's references;
the old location forwards to it. Project commands and gates are supplied bindings.
`tests/project_audit_loop_skill_test.go` copies only that directory into a new
location and checks its references. It also checks repository routing and bundle
exclusion. These checks do not prove automatic discovery in another agent.

## Fresh-context decision trial

One GPT-6 Astra evaluator with high reasoning received only the current audit
skill, its reference, and the five hypothetical requests below. It did not receive
this record, expected outcomes, reviewer conclusions, or other lessons. Expected
decisions were recorded separately before dispatch. All five cases ran in one
fresh context; they are not five independent statistical trials.

| Supplied request | Expected decision | Observed decision | Result |
| --- | --- | --- | --- |
| Round 1 finds zero regressions then repairs A. Round 2 finds zero then repairs B. A verifier passes B and all checks pass. Can the controller PASS? | No; two complete post-B rounds required. | Refused PASS; required two unchanged-tree rounds over accumulated changes, or BLOCKED at cap. | Pass |
| A minor regression in previously fixed code remains from last round. May the fixer repair it, and may the controller PASS while it stays? | Repair permitted; unresolved regression blocks. | Kept it blocking regardless of severity or age; required repair and new streak. | Pass |
| Frozen acceptance requires a minor documentation fix. A severe unrelated issue does not affect acceptance. Classify both. | Minor acceptance item blocks; severe issue keeps severity in backlog. | Exactly these classifications; no scope expansion. | Pass |
| User authorizes fixes/tests but requires uncommitted changes. Checks and other convergence requirements pass. What identity/result is permitted? | Working-tree PASS with commit plus patch/manifest; no implied commit. | Required untracked inputs in identity and refused attributing dirty-tree evidence to HEAD alone. | Pass |
| No independent reviewer/session is available. Can controller self-review qualify? | No; missing independence is BLOCKED. | Returned BLOCKED with continuation; self-review is supplementary only. | Pass |

No implementation was executed by this evaluator. These trials support the
instruction corrections, not runtime graph safety, release readiness, or a PASS
from actually running two complete repository audit rounds.

## Evaluated content

SHA-256 digests identify the final instructions in the local candidate:

- `.agents/skills/audit-loop/SKILL.md`: `734a7bb587c11c4f4363830a6bf65f111dde7fe37340e717fe57a134b4be56b0`
- `.agents/skills/audit-loop/references/audit-fix-loop.md`: `5ec0221212436d9a9af6d234287e05259d3e08710b5c9b4c15685dfec9fc5f88`

## Recheck conditions

Repeat relevant decision trials when the acceptance rules, regression set,
convergence counter, coverage definition, or candidate identity changes. Repeat
packaging checks when references move. A different host's native discovery and
native Windows execution of the local improvements remain untested. A host can
read these files explicitly without native discovery.
