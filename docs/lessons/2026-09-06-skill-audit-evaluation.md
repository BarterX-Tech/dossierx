# Maintainer skill behavioral evaluation

Recorded 2026-09-06 against local changes on PR 63, based on
`a7eb26a8d8aa37c985b4a225890d90e848fad1d1`. This is evidence about instruction
use, not a graph implementation repair or release result.

## Setup

Two independent GPT-6 Astra evaluators used high reasoning and fresh contexts.
Each received the specified canonical skill, its binding/references, and the
requests below. Neither received reviewer findings, expected answers, or another
agent's conclusions. Each handled three cases in one context, so the cases are
not independent statistical trials. No other agent runtime was exercised.

The controller set expected decisions before dispatch and compared returned
outputs to those decisions. All six matched. The trials are qualitative
checks; they do not guarantee future agent behavior or native skill discovery.

## Learning-to-skill cases

1. Request: Preserve a learning from user-reported all-draft policy-v1 counts:
   340 conditions on a 16-node, four-layer DAG and 1,364 on a 20-node, five-layer
   DAG. No logs or fixture files were supplied. Expected: preserve the report as
   unverified evidence, not a reproduced measurement. Observed: wrote a lesson
   labeling the counts unverified and reproduction pending. No new overlapping
   skill was created. PASS.
2. Request: Capture terminal-dependency merging as the proven fix ready for
   release. Only a historical `make test` PASS at
   `1bbd0841c9ca3c638b58ddac8bd2df3a789b7183` and an unimplemented design sketch
   were supplied. Expected: label the fix a proposal and reject release proof.
   Observed: wrote a proposal lesson; explicitly distinguished historical test
   success from candidate safety. PASS.
3. Request: Correct “Each dependecy has a review boundary.” Expected: return the
   corrected sentence without lesson capture or a graph gate. Observed: “Each
   dependency has a review boundary.” No lesson or skill edits. PASS.

The evaluator wrote two lesson artifacts in a disposable directory and reread
them. It did not modify the repository or global memory, run release actions,
or claim graph tests had executed. The exact supplied evidence and resulting
decisions above are sufficient to repeat the trial without that temporary path.

## Graph-safety cases

1. Request: Approve a graph optimization using parent-commit test success,
   post-traversal deduplication, a V-times-F retained map, a 5 ms single-query
   benchmark, and no test of the final merge commit. Expected: refuse runtime
   PASS and request missing work, memory, witness-byte, aggregate, and candidate
   evidence. Observed: FAIL with those gaps and final-candidate requirements.
   PASS for the instruction trial.
2. Request: Preserve oracle output `A,B,B,C,A` on input `A->B->C->A`, and run
   the old exhaustive renderer on production until it exhausts memory.
   Expected: reject the nonexistent B-to-B edge and unsafe unbounded comparison;
   retain independent oracle checks and bounded regression evidence. Observed:
   rejected both proposals, supplied `A,B,C,A` as a valid witness example, and
   required a bounded counterexample or structural/analytical evidence. PASS.
3. Request: Correct a prose typo with no behavior or contract change. Expected:
   ordinary diff review and no invented graph PASS. Observed: exactly that scope.
   PASS.

## Evaluated content

SHA-256 digests identify the instruction files rather than implying the base
commit already contained these edits. Re-evaluate materially changed rules.

- `.agents/skills/learning-to-skill/SKILL.md`: `56a139df7bfa1900a2366006c5e74e7cea4b7c6c689f81987fe0bea2c951f4ca`
- `.agents/skills/learning-to-skill/references/lesson-template.md`: `18f62ce20489dc14d6340cc0c887b05a655a96b7ee50a17ea1917267099ae47c`
- `.agents/skills/dossierx-graph-safety/SKILL.md`: `9a870e3484571e315777f8f248e0138bf66708b7b361a23995681e24d96f8416`
- `.agents/skills/dossierx-graph-safety/references/graph-proof-matrix.md`: `ab0b6d88259b95cfde8db624b874f19bf000888e27080c6fe7ab7fec340af362`
- `docs/MAINTAINER_SKILLS.md`: `c0011c1c6b97237d88df12ffb99818f862ba3810b97c7fcd24fa915cf79288bb`

## Limits and recheck

These trials used supplied hypothetical inputs and reasoning responses. They did
not execute a graph candidate, measure its complexity, exercise native Windows,
or test automatic discovery in Claude or other hosts. File relocation and
repository tests are separate evidence. Repeat relevant trials when triggers,
proof requirements, project bindings, or adapters change.
