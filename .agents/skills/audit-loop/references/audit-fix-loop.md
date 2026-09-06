# Audit-and-fix workflow

This is an agent-neutral workflow. Translate its roles into the current coding
agent's delegation mechanism. Do not copy a vendor-specific workflow script or
hardcode model names. If independent review is unavailable, record that required
coverage as blocked; do not claim that self-review supplies independence.

## 1. Freeze the run contract

Before round 1, the controller records:

- repository root, branch, baseline commit, and candidate commit; for a dirty
  candidate also record a reproducible patch or content manifest including
  untracked inputs, without creating a commit solely to label the evidence;
- dirty files that belong to the user and must be preserved;
- acceptance criteria and the original finding ledger;
- allowed implementation and test scope;
- required verification commands and how to prove they exercised real work;
- the tracked project guidance and all project gates that apply;
- the maximum number of rounds and the cost or risk reason for that cap.

Assign stable finding IDs. Classify impact as `critical`, `major`,
`minor`, or `note`, and state the concrete harm. Track blocking status separately:
an unmet acceptance criterion, required check, or project gate always blocks.
In-scope critical and major defects also block. Any regression in previously
fixed code blocks regardless of severity and belongs in the fixer's active set.
Out-of-scope observations go to the backlog without lowering their severity. If such an observation prevents
meeting the contract, report BLOCKED and the needed scope decision; do not hide
it in the backlog or silently expand the task.

## 2. Verify the starting state

Run the required checks before fixes when safe and useful. Record failures and
prove each selected test or check matched its intended target. A command that
skips, selects zero tests, or cannot use its required browser or tool is a
failure, not a pass.

## 3. Audit independently

Give each auditor the frozen contract, exact candidate, and one bounded lens.
Every auditor must:

1. verify every acceptance item and all active findings directly in code and behavior;
2. inspect changes made since the previous round for regressions;
3. check normative or pinned documentation for drift;
4. return observations outside that scope as non-blocking backlog items;
5. report tool failures or incomplete coverage explicitly.

Use this result shape even if the current agent platform does not enforce a
schema:

```text
round: <number>
candidate: <commit plus patch or content-manifest identity>
coverage_complete: <true|false>
blocking_findings:
  - id: <stable id>
    severity: <critical|major|minor|note>
    blocks_because: <acceptance item, required gate, or in-scope defect>
    evidence: <file, behavior, or command result>
    proposed_fix: <bounded correction>
new_regressions: <newly found count>
outstanding_regressions_in_fixed_code: <count of unresolved, new or old>
backlog:
  - severity: <critical|major|minor|note>
    observation: <what was found>
tool_failures: []
```

For each of the two qualifying convergence rounds, auditors inspect the full
accumulated change from the baseline and previously fixed code, including
verified-closed findings. Reviewing only the delta since the last round is not
complete coverage when that delta is empty.

Missing auditor output, a nonempty `tool_failures`, or `coverage_complete: false`
makes the round non-green and resets the consecutive-round counter.

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
controller then updates the complete ledger, including newly discovered findings,
to `open`, `fixed-unverified`, or `verified-closed` and records the round's
new-regression count and tested tree identity.

Reset the consecutive-zero counter after any candidate edit, regression, or
non-green round. Increment it only after a complete audit round reports zero
new or outstanding regressions in previously fixed code on the unchanged
candidate. Fix verification alone is not an audit round. If fixes follow a round, that round cannot count
toward the new tree's streak.

## 6. Exit

Return `PASS` after every acceptance item is evidenced, all blocking findings
are verified closed, every required check and project gate passes on the final
candidate, and two consecutive complete audit rounds on that same unchanged
candidate report zero new or outstanding regressions in previously fixed code.
No edits may follow those rounds without resetting the streak and refreshing affected evidence.

Return `FAIL` when evidence proves an acceptance criterion cannot be met by the
current approach and another round would repeat the same result.

Return `BLOCKED` when the round cap is reached, required coverage cannot run, or
the task needs authority or a product decision that the controller does not
have. Preserve the exact continuation point.

## 7. Produce the handoff

The final result contains:

1. baseline and final candidate commits/tree identities;
2. the complete finding ledger with closure evidence;
3. all required commands and exact outcomes;
4. a round table with newly found and outstanding regression counts;
5. the non-blocking backlog;
6. remaining findings and the continuation point when not passed.

Do not commit, push, merge, publish, or release unless the invoking task
separately authorized that action.
