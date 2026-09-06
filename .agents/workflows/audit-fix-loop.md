# DossierX audit-and-fix workflow

This is an agent-neutral workflow. Translate its roles into the current coding
agent's delegation mechanism. Do not copy a vendor-specific workflow script or
hardcode model names.

## 1. Freeze the run contract

Before round 1, the controller records:

- repository root, branch, baseline commit, and candidate commit;
- dirty files that belong to the user and must be preserved;
- acceptance criteria and the original finding ledger;
- allowed implementation and test scope;
- required verification commands and how to prove they exercised real work;
- project gates that apply, including graph safety when relevant;
- the maximum number of rounds and the cost or risk reason for that cap.

Assign stable finding IDs. Classify each finding as `critical`, `major`,
`minor`, or `note`. Only unresolved critical and major findings block by
default, unless the frozen acceptance criteria explicitly say otherwise.

## 2. Verify the starting state

Run the required checks before fixes when safe and useful. Record failures and
prove each selected test or check matched its intended target. A command that
skips, selects zero tests, or cannot use its required browser or tool is a
failure, not a pass.

## 3. Audit independently

Give each auditor the frozen contract, exact candidate, and one bounded lens.
Every auditor must:

1. verify the original findings directly in code and behavior;
2. inspect changes made since the previous round for regressions;
3. check normative or pinned documentation for drift;
4. return observations outside that scope as non-blocking backlog items;
5. report tool failures or incomplete coverage explicitly.

Use this result shape even if the current agent platform does not enforce a
schema:

```text
round: <number>
coverage_complete: <true|false>
blocking_findings:
  - id: <stable id>
    severity: <critical|major>
    evidence: <file, behavior, or command result>
    proposed_fix: <bounded correction>
new_regressions: <count>
backlog:
  - severity: <minor|note>
    observation: <what was found>
tool_failures: []
```

Missing auditor output or `coverage_complete: false` makes the round non-green.

## 4. Fix only blocking findings

The controller consolidates duplicate findings by underlying defect, not by
wording. A fixer receives only the active blocking set and its allowed files.

For each fix:

- make the smallest correct change;
- add or strengthen a test or probe that fails for the defect;
- rerun the affected focused checks;
- update normative documentation in the same change when a contract moved;
- leave unrelated backlog items untouched.

The fixer reports changes and evidence but cannot close its own finding.

## 5. Reverify and advance state

An independent verifier reruns the affected checks and reads the fix. The
controller then updates each original finding to `open`, `fixed-unverified`, or
`verified-closed` and records the round's new-regression count.

Reset the consecutive-zero counter whenever a regression is found. Increment it
only after a complete audit round reports zero regressions in previously fixed
code.

## 6. Exit

Return `PASS` after all blocking findings are verified closed, every required
check and project gate passes on the final candidate, and two consecutive
complete audit rounds report zero new regressions.

Return `FAIL` when evidence proves an acceptance criterion cannot be met by the
current approach and another round would repeat the same result.

Return `BLOCKED` when the round cap is reached, required coverage cannot run, or
the task needs authority or a product decision that the controller does not
have. Preserve the exact continuation point.

## 7. Produce the handoff

The final result contains:

1. baseline and final candidate commits;
2. the original finding ledger with closure evidence;
3. all required commands and exact outcomes;
4. a round table with new-regression counts;
5. the non-blocking backlog;
6. remaining findings and the continuation point when not passed.

Do not commit, push, merge, publish, or release unless the invoking task
separately authorized that action.
