# Lesson record template

Copy the structure below into the project's tracked lesson location. Replace
the prompts with evidence; omit inapplicable optional details. This is a record
of what was learned, not a new release procedure or a claim of current safety.

---

# <Short name for the observed problem or practice>

- Recorded: <date>
- State: <verified / proposal / superseded; mixed records label claims below>
- Scope: <project, affected behavior, relevant versions>
- Project binding: <repository-relative guide path>
- Related skill: <repository-relative SKILL.md path, or none yet>

## Observation and evidence

<Who did what, what happened, and the concrete consequence.>

| Claim and state | Evidence | Limits |
| --- | --- | --- |
| <Verified fact, proposal, or superseded conclusion> | <Repo-relative path plus immutable commit; or durable URL plus version/date> | <What this establishes and what it does not> |

<For measurements, include fixture shape, command, tool/runtime version, and
relevant environment. Record unavailable sources explicitly. Keep durable raw
evidence or reproducible inputs where practical; a session identifier alone is
not sufficient.>

## Guidance diagnosis

- Guidance available at the event: <Revision and exposure/loading evidence, or unknown>
- Current coverage and gap type: <Missing, discovery, trigger, ignored, harmful, superseded, tooling, or uncertain>
- Evidence basis: <Independent incidents and their relationships, reproducible defect, contract, or explicit decision>
- Smallest useful correction: <Target and changed decision; or why no skill change is justified>
- Deferred/rejected alternatives: <Reason and what evidence would reopen them; omit if none>

## Reusable lesson

- Decision rule: <What a future worker should do differently and why>
- Applies when: <Concrete trigger and required assumptions>
- Does not apply when: <Boundary or counterexample>
- Project-specific bindings: <Contracts, paths, commands, or existing guide links>
- Proposed remedy: <Only if still untested; distinguish it from the observation>

## Behavioral evaluation

Keep these expected outcomes out of the evaluator's starting prompt. Give the
evaluator the actual request and raw artifacts, not the incident's diagnosis.

| Case | Request and supplied artifacts | Expected observable behavior | Observed behavior and evidence | Result |
| --- | --- | --- | --- | --- |
| Applies | <Realistic task> | <Decision or artifact> | <Actual result or not run> | <pass / fail / pending> |
| Original failure temptation | <Plausible shortcut or stale evidence> | <Reject shortcut; preserve required behavior> | <Actual result or not run> | <pass / fail / pending> |
| Does not apply | <Nearby task outside the trigger> | <Complete task without imposing unrelated work> | <Actual result or not run> | <pass / fail / pending> |

- Evaluated skill revision/digest: <Immutable revision or content digest>
- Evaluation setup: <Fresh context, available tools, input artifact revisions>
- Structural checks: <What ran and result; separate from behavioral evaluation>
- Remaining uncertainty: <What has not been established>

## Revalidation and supersession

- Recheck when: <Relevant contract, code, fixture, dependency, or binding changes>
- Review date: <Only if time-sensitive; otherwise omit>
- Supersedes / superseded by: <Record or revision, and reason; otherwise omit>
- Next verification: <Smallest check needed for an unresolved claim or proposal>
