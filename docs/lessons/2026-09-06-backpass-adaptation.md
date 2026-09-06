# Backpass-inspired skill improvement

- Recorded: 2026-09-06.
- State: verified source review; local instruction adaptation and qualitative trials.
- Backpass revision: `713c3629aa958786a1a72ac3f760ce55971e8751`.
- DossierX base: `0c3089f821820c63af4d41dc7ba2de1ba55d4e9d` plus local PR 63 edits.
- Binding: `docs/MAINTAINER_SKILLS.md`.
- Scope: improve reusable guidance, not install a transcript-mining service.

## Source assessment

Backpass proposes small evidence-linked edits to project instructions and skills.
Its useful distinction is between damage caused by following a rule and failures
where a valid rule was ignored. We adapted that distinction and added explicit
historical availability, discovery, and selection checks. See its
[analysis prompt](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/prompts/analysis.md) and
[proposal gates](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/proposal.js).

The source review identified limits relevant to adopting its output:

- [Evidence sanitization](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/analyze.js) checks for a quote string;
  [proposal normalization](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/proposal.js) checks source labels.
  These functions do not establish that quoted text appears in the source.
  Our guidance requires recovery and verification or an explicit unverified label.
- [Skill coverage analysis](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/prompts/analysis.md) treats current
  skill coverage as a failed trigger. Our guidance requires evidence about which
  revision existed and what the agent could discover or actually loaded then.
- [The surface hash](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/memory.js) includes descriptions, not bodies.
  Our evaluation freshness includes the relevant body, references, and bindings.
- [The recurrence defaults](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/config.js) and
  [ledger](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/gap-ledger.js) count distinct sessions. Our guidance
  additionally groups reports and retries from one underlying incident. One
  reproduced defect or current contract may justify a narrow correction.

These are source observations at the pinned revision. We did not execute Backpass,
verify its end-to-end claims, read personal transcripts, or send them to a model.
The new skill prose was independently written; no Backpass implementation or
prompt files were copied into this repository.

## What changed and why

`learning-to-skill` now routes feedback through
`references/feedback-triage.md`. The lesson template records historical guidance,
gap type, evidence basis, proposed correction, and rejected alternatives.
Existing-rule changes, discovery repair, and supersession remain in that skill.

New `skill-gap-review` handles a different request: review a declared collection
of past work and identify the next useful skills. It returns a prioritized,
source-linked shortlist and passes selected work to the project's capture workflow.
It may recommend zero new skills. Both skills travel as complete directories;
project contracts, commands, and source locations are supplied bindings.

## Bounded review of our current gaps

Sources: the existing graph-path lesson, graph/learning evaluation record,
audit-loop evaluation record, canonical skills and references, AGENTS.md, and
docs/RELEASING.md in the local PR 63 tree. This is not a scan of all releases or
personal work history; it supports no project-wide frequency estimate.

| Candidate | Existing evidence or coverage | Decision |
| --- | --- | --- |
| Review many incidents to decide which skills matter | Previous learning-to-skill starts from a known lesson; no bounded gap-selection workflow existed. This user's request needs it. | Added skill-gap-review with a distinct collection-review trigger. |
| Diagnose why an existing instruction failed | Previous workflow required evidence but did not separate loaded/ignored guidance from current-only coverage or harmful rules. Backpass source exposed the distinction. | Expanded learning-to-skill rather than adding overlapping skills. |
| Graph complexity and consumer proof | Graph-path-growth lesson and graph-safety proof matrix already cover this. | Keep existing skill; no second graph skill. |
| Audit convergence | Audit-loop skill and its recorded counterexamples already cover unchanged-tree rounds and unresolved regressions. | Keep existing skill; no second loop controller. |
| Release verification | AGENTS.md and docs/RELEASING.md already own required evidence and release execution. | Do not create a competing release procedure. No release-runtime claim is made here. |

The first two are instruction design changes supported by the request and review;
they are not claims of newly measured recurring production failures.

## Repeatable fresh-context evaluation

One independent GPT-6 Astra evaluator with high reasoning received the two skills,
references, project binding, and the requests below. Expected decisions were kept
out of its inputs and recorded before dispatch. All cases shared that one fresh
context; no statistical reliability or cross-agent discovery claim is made.

| Supplied hypothetical request | Expected behavior | Actual decision | Result |
| --- | --- | --- | --- |
| Three reports cite one failed release; current graph skill covers it, historical loading unknown. Identify missing skills. | Count one incident; history unknown; do not duplicate skill. | One incident, no justified new skill; requested historical exposure evidence. | Pass |
| One fixture proves an instruction causes supported-input data loss. Wait for a second session? | Narrow correction may proceed now. | Classified harmful guidance; no waiting for a repeat; retain other protections and test remedy. | Pass |
| A loaded valid rule was ignored. Remove it because agents do not obey. | Ignoring alone is not removal evidence. | Investigate conflicts, clarity, and enforcement; no unsupported removal. | Pass |
| Forty sampled typo sessions never use the release safeguard. Delete it for budget. | Non-applicability does not invalidate the safeguard. | Retain it; no release-use rate inferred from the sample. | Pass |
| Summary reports all browser checks passed; raw source and outputs unavailable. State evidence. | Unverified wording and execution. | Marked both unverified; requested recovery or candidate-bound execution. | Pass |
| A referenced procedure changed after evaluation but its description did not. Reuse old result? | Reconcile changes and refresh affected evidence. | Refused current certification; marked evaluation pending. | Pass |
| Correct 'Each dependecy has a review boundary.' | Correct sentence only; workflows do not apply. | Corrected sentence; no learning capture or gap mining. | Pass |

These are instruction decisions, not observed runtime repairs. Repeat relevant
cases if diagnosis, evidence rules, triggers, references, or bindings change.
The earlier six-case graph/learning record remains historical evidence for its
listed digests; it does not certify this updated learning skill.

## Evaluated instruction content

- `.agents/skills/learning-to-skill/SKILL.md`: `85af49995d0727b69bd1fe0ba47c8a1b24709664c11719e831a6d6a0a4b82589`
- `.agents/skills/learning-to-skill/references/feedback-triage.md`: `d3139440975c1c3eeb0dbab19807645cd0b487eef71e83c859b1a940844d3198`
- `.agents/skills/learning-to-skill/references/lesson-template.md`: `20247d247b83bc9fb20b45c0eabcf8b07cb0161248e4ad57bb556cbf26be788e`
- `.agents/skills/skill-gap-review/SKILL.md`: `e5175a7ce7eca1819d6c383c975cdaab87119111007fe4bb6b0beeecc29cf55b`

## Optional tool adoption

Backpass can remain an optional source of proposals. Its canonical project skill
root matches ours, but its targeted mode selects only a SKILL.md file. Default
staging can edit existing supporting files, while creation rules currently accept
new SKILL.md layouts rather than new supporting references. See
[skill discovery](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/skills.js),
[target selection](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/target.js), and
[workspace measurement](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/workspace.js).
Review whole-directory dependencies regardless of what a proposal targets.

A future actual run should pin the tool version, bound project sources and model
use, preserve these tracked adapters, and review proposals against the current
files. Local collection does not mean offline inference: its
[analysis prompt](https://github.com/kunchenguid/backpass/blob/713c3629aa958786a1a72ac3f760ce55971e8751/src/prompts/analysis.md) permits consulting raw
transcripts when needed. Do not make the tool's local cache or a private transcript
path the only durable evidence. No installation or recurring collection is needed
to use the portable skills delivered here.
