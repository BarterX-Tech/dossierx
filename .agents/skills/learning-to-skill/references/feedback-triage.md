# Diagnose instruction feedback

Use for failures, repeated friction, and proposed instruction changes. Supply the
project's canonical rules, skill index, relevant bodies/references, and bounded
evidence sources. This reference needs no particular transcript store or tool.

## Establish what happened

Check the source against the actual observation. Verify quoted wording in the
source before labeling it a quotation; if unavailable, keep it as unverified
reported text. A quotation proves that someone
said something; an agent's claim that a test passed does not establish execution.
Prefer a reproducible fixture, command result, or source artifact at a named
revision for behavioral claims. Preserve reported or unavailable evidence as such.

Record which instruction revision was available during the event and whether the
relevant guidance was discoverable or actually loaded, if the evidence shows it.
If that history is missing, say unknown. A rule added later did not prevent an
earlier incident; current coverage alone is not evidence of a failed trigger.

Classify the gap before proposing a remedy:

- **Missing guidance:** no applicable rule covered the demonstrated decision.
  Add a narrow rule only when it would change that decision and has a clear trigger.
- **Discovery failure:** a relevant skill existed but the host did not expose it.
  Correct routing or an adapter; do not duplicate the procedure in global rules.
- **Trigger failure:** the skill was exposed but not selected for a fitting task.
  Test a clearer description with both matching and nearby non-matching requests.
- **Ignored guidance:** evidence shows the applicable rule was loaded but not
  followed. Investigate conflicting instructions, unclear steps, or missing
  enforcement. Do not remove a valid safeguard merely because it was ignored.
- **Harmful guidance:** following the rule caused the demonstrated failure.
  Correct, narrow, or remove it with a counterexample and checks for protections
  that still matter. Distinguish correlation from the causal mechanism.
- **Superseded or misplaced guidance:** a changed contract invalidates the rule,
  or a narrow procedure burdens unrelated work. Update or move it while retaining
  required routing; lack of recent use alone does not invalidate a rare safeguard.

When exposure or causality is unknown, keep competing explanations and name the
smallest check that would distinguish them. Do not force a confident category.
An external runner failure usually belongs in that runner's guidance; a project
may still need a narrow rule for reporting the resulting missing evidence.

## Weigh evidence without teaching noise

Count independent underlying incidents, not repeated summaries, copied logs,
agent votes, or reruns of the same attempt. Record relationships between sources.
Separate human-directed work from automated runs when summarizing recurrence.
State the collection window, selection rules, exclusions, and missing sources;
a sampled set cannot establish a repository-wide failure rate.

For a broad new convention, seek corroboration from independent incidents before
generalizing. A single reproducible correctness defect, authoritative contract,
or explicit user decision can justify a narrow correction now; do not wait for
another failure. Label that evidence basis instead of inventing recurrence.
Likewise, remove obsolete rules on current contract evidence without requiring
two new harmful incidents.

## Keep the instruction library small

Prefer, in order of fit: fix tooling or a test, correct discovery, sharpen the
trigger, clarify the existing rule, move a conditional procedure, or create a
new skill with a distinct use. Preserve rare safety requirements and semantic
protections when shortening text. A word-count reduction is not a correctness
measure. Keep both always-loaded guidance and triggered references concise;
triggered content still consumes context when used.

Set a small, task-appropriate change scope before editing. Separate each proposed
change's evidence, expected decision, affected files, and verification. Record
rejected proposals and why in the project's evidence record; revisit only when
new evidence or changed requirements address that reason. Do not infer approval
for unrelated changes from authorization to improve one skill.

Validate the resulting decisions using the parent skill's fresh-context method.
Keep at least the original failure temptation and a non-applicable task. Recheck
bodies and references, not only descriptions, when deciding whether a lesson is
already covered or a previous evaluation is stale.
